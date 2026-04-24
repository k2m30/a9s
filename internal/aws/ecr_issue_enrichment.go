// ecr_issue_enrichment.go — Wave 2 issue enrichment for the ecr resource type.
package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("ecr", EnrichECRRepository, 100)
	resource.RegisterIssueEnricherFieldKeys("ecr", []string{"critical_vulns", "high_vulns", "images_scanned"})
}

// ECRImagesCapPerRepo is the maximum number of images scanned per repository
// during Wave 2 enrichment. Caps the per-repo ListImages + DescribeImageScanFindings
// fanout so that large registries do not cause excessive API call counts.
const ECRImagesCapPerRepo = 10

// EnrichECRRepository enumerates up to ECRImagesCapPerRepo images per repository
// using ListImages, then calls DescribeImageScanFindings for each image to surface
// CRITICAL and HIGH vulnerability findings.
//
// Findings:
//   - Any CRITICAL findings across scanned images → "!" severity.
//   - Any HIGH findings (no critical) → "~" severity.
//
// fieldUpdates keys: "critical_vulns", "high_vulns", "images_scanned".
// Repositories without scan data (unscanned images) are skipped silently.
// Per-image ScanNotFoundException is routine and not treated as an error.
// Skip when clients is nil or clients.ECR does not implement ECRListImagesAPI
// or ECRDescribeImageScanFindingsAPI.
func EnrichECRRepository(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients == nil || clients.ECR == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	listAPI, okL := clients.ECR.(ECRListImagesAPI)
	scanAPI, okS := clients.ECR.(ECRDescribeImageScanFindingsAPI)
	if !okL || !okS {
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
			// ID is usually an ARN; last segment after '/' is the repository name.
			if idx := strings.LastIndex(r.ID, "/"); idx >= 0 && idx < len(r.ID)-1 {
				repoName = r.ID[idx+1:]
			} else {
				repoName = r.ID
			}
		}
		if repoName == "" {
			continue
		}
		total++

		// Paginate ListImages, capped at ECRImagesCapPerRepo.
		var imageIDs []ecrtypes.ImageIdentifier
		var listToken *string
		listFailed := false
		for len(imageIDs) < ECRImagesCapPerRepo {
			listOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.ListImagesOutput, error) {
				return listAPI.ListImages(ctx, &ecr.ListImagesInput{
					RepositoryName: aws.String(repoName),
					NextToken:      listToken,
				})
			})
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
				truncated = true
				truncatedIDs[r.ID] = true
				listFailed = true
				break
			}
			for _, id := range listOut.ImageIds {
				if len(imageIDs) >= ECRImagesCapPerRepo {
					truncated = true
					truncatedIDs[r.ID] = true
					break
				}
				imageIDs = append(imageIDs, id)
			}
			if listOut.NextToken == nil {
				break
			}
			listToken = listOut.NextToken
		}

		if listFailed {
			continue
		}

		scannedCount := 0
		var criticalTotal int32
		var highTotal int32
		for _, img := range imageIDs {
			if img.ImageDigest == nil {
				continue
			}
			scanOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecr.DescribeImageScanFindingsOutput, error) {
				return scanAPI.DescribeImageScanFindings(ctx, &ecr.DescribeImageScanFindingsInput{
					RepositoryName: aws.String(repoName),
					ImageId:        &ecrtypes.ImageIdentifier{ImageDigest: img.ImageDigest},
				})
			})
			if err != nil {
				// ScanNotFoundException is routine for unscanned images — skip silently.
				continue
			}
			scannedCount++
			if scanOut.ImageScanFindings != nil {
				for sev, n := range scanOut.ImageScanFindings.FindingSeverityCounts {
					switch sev {
					case string(ecrtypes.FindingSeverityCritical):
						criticalTotal += n
					case string(ecrtypes.FindingSeverityHigh):
						highTotal += n
					}
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
		AggregateFailures("ecr-enrich: ListImages", failures, total)
}
