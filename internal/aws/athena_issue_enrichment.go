// athena_issue_enrichment.go — Wave 2 issue enrichment for the athena resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("athena", EnrichAthenaWorkGroup, 100)
}

// EnrichAthenaWorkGroup calls GetWorkGroup per workgroup (capped at EnrichmentCap) to
// surface governance and security findings.
//
// Findings:
//   - WorkGroup.Configuration.EnforceWorkGroupConfiguration == false → "~" severity,
//     "EnforceWorkGroupConfiguration disabled (callers can bypass)".
//   - WorkGroup.Configuration.ResultConfiguration.EncryptionConfiguration == nil → "~" severity,
//     "result encryption not configured".
//
// Per-WG errors mark Truncated=true and are skipped.
// Skip when clients.Athena == nil.
func EnrichAthenaWorkGroup(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.Athena == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		wgName := r.Fields["workgroup_name"]
		if wgName == "" {
			wgName = r.ID
		}
		if wgName == "" {
			continue
		}
		out, err := clients.Athena.GetWorkGroup(ctx, &athena.GetWorkGroupInput{
			WorkGroup: aws.String(wgName),
		})
		if err != nil {
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}
		if out.WorkGroup == nil || out.WorkGroup.Configuration == nil {
			continue
		}
		cfg := out.WorkGroup.Configuration
		key := r.ID
		if key == "" {
			key = wgName
		}
		var rows []resource.FindingRow
		// EnforceWorkGroupConfiguration defaults to true; false means callers can bypass settings.
		if cfg.EnforceWorkGroupConfiguration != nil && !*cfg.EnforceWorkGroupConfiguration {
			rows = append(rows, resource.FindingRow{
				Label: "EnforceWorkGroupConfiguration",
				Value: "false",
				Tier:  "~",
			})
		}
		// Missing encryption on result configuration is a security concern.
		if cfg.ResultConfiguration == nil || cfg.ResultConfiguration.EncryptionConfiguration == nil {
			rows = append(rows, resource.FindingRow{
				Label: "ResultConfiguration.EncryptionConfiguration",
				Value: "nil",
				Tier:  "~",
			})
		}
		if len(rows) == 0 {
			continue
		}
		summary := rows[0].Label
		if len(rows) > 1 {
			summary = fmt.Sprintf("%s (%d findings)", rows[0].Label, len(rows))
		}
		findings[key] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  summary,
			Rows:     rows,
		}
		// "~" severity does not contribute to IssueCount.
	}
	return IssueEnricherResult{Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings}, nil
}
