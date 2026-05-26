// codeartifact_issue_enrichment.go — Wave 2 issue enrichment for the codeartifact resource type.
package aws

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// codeartifact canonical FindingCodes.
const (
	codeartifactCodePublicAccessPolicy domain.FindingCode = "codeartifact.public-access-policy"
	codeartifactCodeNoPermissionsPolicy domain.FindingCode = "codeartifact.no-permissions-policy"
)

// EnrichCodeArtifactRepository calls GetRepositoryPermissionsPolicy per repository (capped at
// EnrichmentCap) to surface IAM policy findings.
//
// Findings:
//   - ResourceNotFoundException → "~" severity, "no permissions policy" (default open within domain).
//   - Policy.Document contains `"Principal":"*"` → "!" severity, "public access policy".
//
// Per-repo errors other than ResourceNotFoundException mark Truncated=true and are skipped.
// Skip when clients.CodeArtifact == nil.
func EnrichCodeArtifactRepository(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.CodeArtifact == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	issueCount := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
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
			continue
		}
		key := r.ID
		if key == "" {
			key = repoName
		}
		// Count packages in this repository (optional — only if the client supports ListPackages).
		// Walks all pages via NextToken so the count is exact, not first-page only.
		if listPkgAPI, ok := clients.CodeArtifact.(CodeArtifactListPackagesAPI); ok {
			total := 0
			var nextToken *string
			for {
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
				nextToken = pkgOut.NextToken
			}
			if total >= 0 {
				result.FieldUpdates[key] = map[string]string{"package_count": resource.FormatExact(total)}
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
		if err != nil {
			if _, ok := errors.AsType[*codeartifacttypes.ResourceNotFoundException](err); ok {
				// No policy set — default open within the domain.
				setWave2Finding(&result, key, codeartifactCodeNoPermissionsPolicy, "no permissions policy", "~", "codeartifact", nil)
				// "~" does not contribute to IssueCount.
				continue
			}
			// Any other error — skip this repo but flag truncation.
			truncated = true
			result.TruncatedIDs[r.ID] = true
			continue
		}
		if out.Policy == nil || out.Policy.Document == nil {
			continue
		}
		doc := *out.Policy.Document
		if strings.Contains(doc, `"Principal":"*"`) || strings.Contains(doc, `"Principal": "*"`) {
			setWave2Finding(&result, key, codeartifactCodePublicAccessPolicy, "public access policy", "!", "codeartifact",
				[]domain.DetailRow{
					{Label: "Principal", Value: "*", Tier: "!"},
				})
			issueCount++
		}
	}
	result.IssueCount = issueCount
	result.Truncated = truncated
	return result, nil
}
