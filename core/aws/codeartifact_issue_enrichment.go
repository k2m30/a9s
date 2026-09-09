// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// codeartifact_issue_enrichment.go — Wave 2 issue enrichment for the codeartifact resource type.
package aws

import (
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// codeartifact canonical FindingCodes.
const (
	codeartifactCodePublicAccessPolicy  domain.FindingCode = "codeartifact.public-access-policy"
	codeartifactCodeNoPermissionsPolicy domain.FindingCode = "codeartifact.no-permissions-policy"
)

// EnrichCodeArtifactRepository calls GetRepositoryPermissionsPolicy per repository (capped at
// EnrichmentCap) to surface IAM policy findings.
//
// Findings:
//   - ResourceNotFoundException → "~" severity, "no permissions policy" (default open within domain).
//   - the resource policy grants a wildcard principal with no restrictive
//     condition → "!" severity, "public access policy".
//
// Per-repo errors other than ResourceNotFoundException mark Truncated=true and are skipped.
// Skip when clients.CodeArtifact == nil.
func EnrichCodeArtifactRepository(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.CodeArtifact == nil {
		return result, nil
	}
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	truncated := false
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		// Support both "repo_name" (fetcher canonical) and "repository_name" (legacy/test alias).
		repoName := r.Fields["repo_name"]
		if repoName == "" {
			repoName = r.Fields["repository_name"]
		}
		if repoName == "" {
			repoName = r.ID
		}
		// Support both "domain_name" (fetcher canonical) and "domain" (legacy/test alias).
		domainName := r.Fields["domain_name"]
		if domainName == "" {
			domainName = r.Fields["domain"]
		}
		domainOwner := r.Fields["domain_owner"]
		if repoName == "" || domainName == "" {
			return
		}
		key := r.ID
		if key == "" {
			key = repoName
		}
		// Count packages in this repository (optional — only if the client supports ListPackages).
		// Walks all pages via NextToken so the count is exact, not first-page only —
		// up to PerParentPageCap: no packages-per-repository quota exists at all, and
		// upstream-connected repos materialize external packages as consumers pull
		// them, so an account with a heavily-used upstream repo can walk indefinitely.
		if listPkgAPI, ok := clients.CodeArtifact.(CodeArtifactListPackagesAPI); ok {
			total := 0
			pkgTruncated := false
			var nextToken *string
			for pages := 0; ; pages++ {
				pkgInput := &codeartifact.ListPackagesInput{
					Domain:     aws.String(domainName),
					Repository: aws.String(repoName),
					NextToken:  nextToken,
				}
				if domainOwner != "" {
					pkgInput.DomainOwner = aws.String(domainOwner)
				}
				pkgOut, pkgErr := listPkgAPI.ListPackages(ctx, pkgInput)
				if pkgErr != nil {
					total = -1 // signal partial
					break
				}
				total += len(pkgOut.Packages)
				if pkgOut.NextToken == nil || *pkgOut.NextToken == "" {
					break
				}
				if pages+1 >= PerParentPageCap {
					pkgTruncated = true
					break
				}
				nextToken = pkgOut.NextToken
			}
			if total >= 0 {
				count := resource.FormatExact(total)
				if pkgTruncated {
					// A page cap on an informational count, not a failed call:
					// the "+" is where the cap is reported. Marking the ID
					// truncated would make FoldWave2Rows skip the row, and the
					// permissions-policy verdict below — a separate call the
					// count says nothing about — would never reach it.
					count = resource.FormatTruncated(total)
				}
				mu.Lock()
				result.FieldUpdates[key] = map[string]string{"package_count": count}
				mu.Unlock()
			}
		}
		input := &codeartifact.GetRepositoryPermissionsPolicyInput{
			Domain:     aws.String(domainName),
			Repository: aws.String(repoName),
		}
		if domainOwner != "" {
			input.DomainOwner = aws.String(domainOwner)
		}
		out, err := clients.CodeArtifact.GetRepositoryPermissionsPolicy(ctx, input)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			if _, ok := errors.AsType[*codeartifacttypes.ResourceNotFoundException](err); ok {
				// No policy set — default open within the domain.
				setWave2Finding(&result, key, codeartifactCodeNoPermissionsPolicy, nil)
				return
			}
			truncated = true
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		if out.Policy == nil || out.Policy.Document == nil {
			return
		}
		parsed, perr := iampolicy.Parse(*out.Policy.Document)
		if perr != nil {
			truncated = true
			MarkSkipped(&result, r.ID, &failures, perr)
			return
		}
		if ex := iampolicy.Evaluate(parsed, ownAccount); ex.Public {
			setWave2Finding(&result, key, codeartifactCodePublicAccessPolicy, publicPolicyRows(ex))

		}
	})
	SetTruncated(&result, truncated)
	return result, AggregateFailures("GetRepositoryPermissionsPolicy", failures, n)
}
