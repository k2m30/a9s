// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecr_issue_enrichment.go — Wave 2 issue enrichment for the ecr resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	// ecrCodeHighVulnerabilities — highs but no critical. A separate code
	// because it is a separate severity, and a code declares one.
	ecrCodeHighVulnerabilities domain.FindingCode = "ecr.vulnerabilities-high"

	// ecrCodePublicPolicy — the repository's resource policy grants a
	// wildcard principal (GetRepositoryPolicy, evaluated by iampolicy).
	ecrCodePublicPolicy domain.FindingCode = "ecr.public-policy"

	// ecrCodeNoLifecyclePolicy — GetLifecyclePolicy reports no policy.
	ecrCodeNoLifecyclePolicy domain.FindingCode = "ecr.no-lifecycle-policy"
)

// ecrPageSize is the largest page DescribeImages and DescribeImageScanFindings
// accept (maxResults 1-1000,
// https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_DescribeImages.html,
// https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_DescribeImageScanFindings.html).
const ecrPageSize = 1000

// EnrichECRRepository reads every DescribeImages page of each repository,
// picks the newest image by imagePushedAt — DescribeImages documents no order
// and cannot sort — and reads that image's scan results
// (ecrReadScanResults) for its CRITICAL and HIGH counts.
//
// Findings:
//   - Any CRITICAL in the newest image → "!" severity.
//   - Any HIGH (no CRITICAL) → "~" severity.
//
// fieldUpdates keys: "critical_vulns", "high_vulns", "images_scanned".
// Per-repo errors aggregate into a composite returned error. A repository
// whose image list or scan results could not be read in full, or whose newest
// image's scan holds no findings to count (ecrScanUnread), gets no count and
// is marked not inspected: the newest image may be on a page nobody read, a
// scan may not have finished, and a partial count is neither the total nor a
// proven zero.
func EnrichECRRepository(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients == nil || clients.ECR == nil {
		return result, nil
	}
	describeAPI, ok := clients.ECR.(ECRDescribeImagesAPI)
	if !ok {
		return result, nil
	}
	scanAPI, _ := clients.ECR.(ECRDescribeImageScanFindingsAPI)
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())

	truncated := false
	var failures []Failure
	total := 0
	resources = capAtEnrichmentCap(&result, resources, nil, resourceIDsOf)
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
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

		images, imagesComplete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]ecrtypes.ImageDetail, *string, error) {
			out, err := describeAPI.DescribeImages(ctx, &ecr.DescribeImagesInput{
				RepositoryName: aws.String(repoName),
				MaxResults:     aws.Int32(ecrPageSize),
				NextToken:      token,
			})
			if err != nil {
				return nil, nil, err
			}
			return out.ImageDetails, out.NextToken, nil
		})
		if err != nil {
			mu.Lock()
			MarkSkipped(&result, r.ID, &failures, err)
			truncated = true
			mu.Unlock()
			return
		}
		// An image with no push time sorts oldest.
		var latest *ecrtypes.ImageDetail
		for j := range images {
			at := images[j].ImagePushedAt
			if latest == nil || at != nil && (latest.ImagePushedAt == nil || at.After(*latest.ImagePushedAt)) {
				latest = &images[j]
			}
		}
		scanComplete := true
		var scanErr error
		if imagesComplete && latest != nil {
			scanComplete, scanErr = ecrReadScanResults(ctx, scanAPI, repoName, latest)
		}

		// The two policy reads are issued before the lock is taken: holding it
		// across a network call would serialise the whole page behind one
		// repository and undo ForEachParallel.
		exposure, policyErr := ecrRepositoryExposure(ctx, clients.ECR, repoName, ownAccount)
		noLifecyclePolicy, lifecycleErr := ecrLifecyclePolicyMissing(ctx, clients.ECR, repoName)

		mu.Lock()
		defer mu.Unlock()

		switch {
		case policyErr != nil:
			// Unknown, not clean: the row renders "?" rather than claiming a
			// repository nobody could read is private.
			truncated = true
			MarkSkipped(&result, r.ID, &failures, policyErr)
		case exposure.Public:
			setWave2Finding(&result, r.ID, ecrCodePublicPolicy, publicPolicyRows(exposure))
		}
		switch {
		case lifecycleErr != nil:
			truncated = true
			MarkSkipped(&result, r.ID, &failures, lifecycleErr)
		case noLifecyclePolicy:
			setWave2Finding(&result, r.ID, ecrCodeNoLifecyclePolicy, nil)
		}

		switch {
		case scanErr != nil:
			truncated = true
			MarkSkipped(&result, r.ID, &failures, scanErr)
			return
		case !imagesComplete || !scanComplete:
			truncated = true
			markUninspected(&result, r.ID, CheckCap)
			return
		case latest != nil && latest.ImageScanFindingsSummary == nil && scanAPI == nil:
			// A client without DescribeImageScanFindings reads no scan: no
			// count, and no mark, as for any call a client predates.
			return
		case latest != nil && ecrScanUnread(*latest) != "":
			truncated = true
			markUninspected(&result, r.ID, ecrScanUnread(*latest))
			return
		}

		scannedCount := 0
		var criticalTotal, highTotal int32
		if latest != nil {
			scannedCount = 1
			counts := latest.ImageScanFindingsSummary.FindingSeverityCounts
			criticalTotal = counts[string(ecrtypes.FindingSeverityCritical)]
			highTotal = counts[string(ecrtypes.FindingSeverityHigh)]
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
		if criticalTotal > 0 {
			rows = append(rows, domain.DetailRow{
				Label: "Critical",
				Value: fmt.Sprintf("%d critical findings in the newest image", criticalTotal),
				Tier:  "!",
			})
		}
		if highTotal > 0 {
			rows = append(rows, domain.DetailRow{
				Label: "High",
				Value: fmt.Sprintf("%d high findings in the newest image", highTotal),
				Tier:  "~",
			})
		}
		// A repository with a critical is a different signal from one with
		// highs alone, and a code declares one severity.
		if criticalTotal > 0 {
			setWave2Finding(&result, r.ID, ecrCodeVulnerabilities, rows, strconv.Itoa(int(criticalTotal)), strconv.Itoa(int(highTotal)))
			return
		}
		setWave2Finding(&result, r.ID, ecrCodeHighVulnerabilities, rows, strconv.Itoa(int(highTotal)))
	})

	SetTruncated(&result, truncated)
	return result, errors.Join(loopErr, AggregateFailures("repository posture", failures, total))
}

// ecrRepositoryExposure evaluates a repository's resource policy through the
// shared policy engine. A repository with no policy at all is a definite
// answer — the default, and the safe one — so it reports no exposure and no
// failure. Anything else that stops the read leaves the repository unknown.
func ecrRepositoryExposure(ctx context.Context, api ECRAPI, repoName, ownAccount string) (iampolicy.Exposure, error) {
	policyAPI, ok := api.(ECRGetRepositoryPolicyAPI)
	if !ok {
		return iampolicy.Exposure{}, nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.GetRepositoryPolicyOutput, error) {
		return policyAPI.GetRepositoryPolicy(ctx, &ecr.GetRepositoryPolicyInput{RepositoryName: aws.String(repoName)})
	})
	if err != nil {
		if _, notFound := errors.AsType[*ecrtypes.RepositoryPolicyNotFoundException](err); notFound {
			return iampolicy.Exposure{}, nil
		}
		return iampolicy.Exposure{}, err
	}
	if out.PolicyText == nil {
		return iampolicy.Exposure{}, nil
	}
	doc, perr := iampolicy.Parse(*out.PolicyText)
	if perr != nil {
		return iampolicy.Exposure{}, perr
	}
	return iampolicy.Evaluate(doc, ownAccount), nil
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
func ecrLifecyclePolicyMissing(ctx context.Context, api ECRAPI, repoName string) (missing bool, err error) {
	lifecycleAPI, ok := api.(ECRGetLifecyclePolicyAPI)
	if !ok {
		return false, nil
	}
	_, err = RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.GetLifecyclePolicyOutput, error) {
		return lifecycleAPI.GetLifecyclePolicy(ctx, &ecr.GetLifecyclePolicyInput{RepositoryName: aws.String(repoName)})
	})
	if err == nil {
		return false, nil
	}
	if _, notFound := errors.AsType[*ecrtypes.LifecyclePolicyNotFoundException](err); notFound {
		return true, nil
	}
	return false, err
}
