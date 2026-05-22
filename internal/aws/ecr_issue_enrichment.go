// ecr_issue_enrichment.go — Wave 2 issue enrichment for the ecr resource type.
package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/internal/resource"
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
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients == nil || clients.ECR == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	describeAPI, ok := clients.ECR.(ECRDescribeImagesAPI)
	if !ok {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}

	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		repoName := r.Name
		if repoName == "" {
			repoName = r.ID
		}
		if repoName == "" {
			continue
		}
		total++

		// ONE call per repo. Returns up to ECRImagesPerRepo most-recent images
		// with ImageScanFindingsSummary populated inline.
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.DescribeImagesOutput, error) {
			return describeAPI.DescribeImages(ctx, &ecr.DescribeImagesInput{
				RepositoryName: aws.String(repoName),
				MaxResults:     aws.Int32(int32(ECRImagesPerRepo)),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			truncatedIDs[r.ID] = true
			continue
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

		fieldUpdates[r.ID] = map[string]string{
			"critical_vulns": strconv.FormatInt(int64(criticalTotal), 10),
			"high_vulns":     strconv.FormatInt(int64(highTotal), 10),
			"images_scanned": strconv.Itoa(scannedCount),
		}

		if criticalTotal == 0 && highTotal == 0 {
			continue
		}

		var rows []resource.FindingRow
		tier := "~"
		if criticalTotal > 0 {
			tier = "!"
			rows = append(rows, resource.FindingRow{
				Label: "CRITICAL",
				Value: fmt.Sprintf("%d CRITICAL findings across %d image(s)", criticalTotal, scannedCount),
				Tier:  "!",
			})
		}
		if highTotal > 0 {
			rows = append(rows, resource.FindingRow{
				Label: "HIGH",
				Value: fmt.Sprintf("%d HIGH findings across %d image(s)", highTotal, scannedCount),
				Tier:  "~",
			})
		}
		findings[r.ID] = resource.EnrichmentFinding{
			Severity: tier,
			Summary:  rows[0].Value,
			Rows:     rows,
		}
	}

	issueCount := 0
	for _, f := range findings {
		if f.Severity == "!" {
			issueCount++
		}
	}
	return IssueEnricherResult{IssueCount: issueCount, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates},
		AggregateFailures("ecr-enrich: DescribeImages", failures, total)
}
