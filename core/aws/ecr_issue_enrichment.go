// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecr_issue_enrichment.go — Wave 2 issue enrichment for the ecr resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ecr canonical FindingCodes.
const (
	ecrCodeVulnerabilities domain.FindingCode = "ecr.vulnerabilities"

	// ecrCodePublicPolicy — the repository's resource policy grants a
	// wildcard principal (GetRepositoryPolicy, evaluated by iampolicy).
	ecrCodePublicPolicy domain.FindingCode = "ecr.public-policy"

	// ecrCodeNoLifecyclePolicy — GetLifecyclePolicy reports no policy.
	ecrCodeNoLifecyclePolicy domain.FindingCode = "ecr.no-lifecycle-policy"
)

// ECRImagesPerRepo caps how many recent images are inspected per repository.
// DescribeImages returns ImageScanFindingsSummary inline, so the enricher pays
// exactly one AWS call per repo regardless of how many images it samples —
// restoring the wave-2 N+1 budget (previously 11N: ListImages + ≤10
// DescribeImageScanFindings per repo). This value caps how many images are
// included in the single DescribeImages response; AWS returns the most
// recent images by default.
const ECRImagesPerRepo = 10

// EnrichECRRepository issues ONE DescribeImages call per repository with
// maxResults=ECRImagesPerRepo and aggregates CRITICAL / HIGH counts from
// ImageDetails[].ImageScanFindingsSummary.FindingSeverityCounts — which AWS
// populates inline when scan-on-push is enabled on the repo.
//
// Wave-2 budget: 1 call per repo (N), 0 ancillary per-image calls. This
// matches the N+1 design every other enricher follows. The previous
// implementation fanned out up to 11 calls per repo which blew the 10s
// enrichment context on any account with >10 repos.
//
// Findings:
//   - Any CRITICAL across scanned images → "!" severity (bumps S1 badge).
//   - Any HIGH (no CRITICAL) → "~" severity.
//
// fieldUpdates keys: "critical_vulns", "high_vulns", "images_scanned".
// Per-repo errors aggregate into a composite returned error (E1–E6 contract).
// Repositories without scan data (unscanned images) contribute zero counts
// silently — AWS returns a nil ImageScanFindingsSummary for those.
func EnrichECRRepository(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients == nil || clients.ECR == nil {
		return result, nil
	}
	describeAPI, ok := clients.ECR.(ECRDescribeImagesAPI)
	if !ok {
		return result, nil
	}
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())

	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		repoName := r.Name
		if repoName == "" {
			repoName = r.ID
		}
		if repoName == "" {
			return
		}
		mu.Lock()
		total++
		mu.Unlock()

		// ONE call per repo. Returns up to ECRImagesPerRepo most-recent images
		// with ImageScanFindingsSummary populated inline.
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.DescribeImagesOutput, error) {
			return describeAPI.DescribeImages(ctx, &ecr.DescribeImagesInput{
				RepositoryName: aws.String(repoName),
				MaxResults:     aws.Int32(int32(ECRImagesPerRepo)),
			})
		})
		if err != nil {
			mu.Lock()
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			result.TruncatedIDs[r.ID] = true
			mu.Unlock()
			return
		}

		// The two policy reads are issued before the lock is taken: holding it
		// across a network call would serialise the whole page behind one
		// repository and undo ForEachParallel.
		exposure, policyUnreadable := ecrRepositoryExposure(ctx, clients.ECR, repoName, ownAccount)
		noLifecyclePolicy, lifecycleUnreadable := ecrLifecyclePolicyMissing(ctx, clients.ECR, repoName)

		mu.Lock()
		defer mu.Unlock()

		switch {
		case policyUnreadable:
			// Unknown, not clean: the row renders "?" rather than claiming a
			// repository nobody could read is private.
			truncated = true
			result.TruncatedIDs[r.ID] = true
		case exposure.Public:
			setWave2Finding(&result, r.ID, ecrCodePublicPolicy, "repository policy open to anyone", "!", "ecr", publicPolicyRows(exposure))
		}
		switch {
		case lifecycleUnreadable:
			truncated = true
			result.TruncatedIDs[r.ID] = true
		case noLifecyclePolicy:
			setWave2Finding(&result, r.ID, ecrCodeNoLifecyclePolicy, "no lifecycle policy", "~", "ecr", nil)
		}

		scannedCount := 0
		var criticalTotal int32
		var highTotal int32
		for _, img := range out.ImageDetails {
			summary := img.ImageScanFindingsSummary
			if summary == nil {
				continue
			}
			scannedCount++
			for sev, n := range summary.FindingSeverityCounts {
				switch sev {
				case string(ecrtypes.FindingSeverityCritical):
					criticalTotal += n
				case string(ecrtypes.FindingSeverityHigh):
					highTotal += n
				}
			}
		}

		result.FieldUpdates[r.ID] = map[string]string{
			"critical_vulns": strconv.FormatInt(int64(criticalTotal), 10),
			"high_vulns":     strconv.FormatInt(int64(highTotal), 10),
			"images_scanned": strconv.Itoa(scannedCount),
		}

		if criticalTotal == 0 && highTotal == 0 {
			return
		}

		var rows []domain.DetailRow
		var parts []string
		tier := "~"
		if criticalTotal > 0 {
			tier = "!"
			parts = append(parts, fmt.Sprintf("%d critical", criticalTotal))
			rows = append(rows, domain.DetailRow{
				Label: "Critical",
				Value: fmt.Sprintf("%d critical findings across %d image(s)", criticalTotal, scannedCount),
				Tier:  "!",
			})
		}
		if highTotal > 0 {
			parts = append(parts, fmt.Sprintf("%d high", highTotal))
			rows = append(rows, domain.DetailRow{
				Label: "High",
				Value: fmt.Sprintf("%d high findings across %d image(s)", highTotal, scannedCount),
				Tier:  "~",
			})
		}
		summary := strings.Join(parts, ", ") + " vulnerabilities"
		setWave2Finding(&result, r.ID, ecrCodeVulnerabilities, summary, tier, "ecr", rows)
	})
	sort.Strings(failures)

	result.Truncated = truncated
	return result, AggregateFailures("ecr-enrich: DescribeImages", failures, total)
}

// ecrRepositoryExposure evaluates a repository's resource policy through the
// shared policy engine. A repository with no policy at all is a definite
// answer — the default, and the safe one — so it reports no exposure and no
// failure. Anything else that stops the read leaves the repository unknown.
func ecrRepositoryExposure(ctx context.Context, api ECRAPI, repoName, ownAccount string) (iampolicy.Exposure, bool) {
	policyAPI, ok := api.(ECRGetRepositoryPolicyAPI)
	if !ok {
		return iampolicy.Exposure{}, false
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.GetRepositoryPolicyOutput, error) {
		return policyAPI.GetRepositoryPolicy(ctx, &ecr.GetRepositoryPolicyInput{RepositoryName: aws.String(repoName)})
	})
	if err != nil {
		if _, notFound := errors.AsType[*ecrtypes.RepositoryPolicyNotFoundException](err); notFound {
			return iampolicy.Exposure{}, false
		}
		return iampolicy.Exposure{}, true
	}
	if out.PolicyText == nil {
		return iampolicy.Exposure{}, false
	}
	doc, perr := iampolicy.Parse(*out.PolicyText)
	if perr != nil {
		return iampolicy.Exposure{}, true
	}
	return iampolicy.Evaluate(doc, ownAccount), false
}

// ecrLifecyclePolicyMissing reports whether the repository keeps images
// forever. Here the NotFound answer IS the finding, which is why it is read
// through its own standalone interface rather than the aggregate: a client
// that predates the call degrades to "nothing to say" instead of reporting
// every repository as unpolicied.
//
// The read gives three answers and they stay apart: NotFound is the finding,
// success is the healthy case, and anything else — a denial, a transient
// failure — is a read that did not happen. The second return says so, and a
// repository nobody could read is not a repository with a policy.
func ecrLifecyclePolicyMissing(ctx context.Context, api ECRAPI, repoName string) (missing, unreadable bool) {
	lifecycleAPI, ok := api.(ECRGetLifecyclePolicyAPI)
	if !ok {
		return false, false
	}
	_, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.GetLifecyclePolicyOutput, error) {
		return lifecycleAPI.GetLifecyclePolicy(ctx, &ecr.GetLifecyclePolicyInput{RepositoryName: aws.String(repoName)})
	})
	if err == nil {
		return false, false
	}
	if _, notFound := errors.AsType[*ecrtypes.LifecyclePolicyNotFoundException](err); notFound {
		return true, false
	}
	return false, true
}

// S5 operator sentences for the two policy findings.
