// rds_issue_enrichment.go — Wave 2 issue enrichment for the rds resource type.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("rds", EnrichRDSDocDBMaintenance, 100)
}

// EnrichRDSDocDBMaintenance calls DescribePendingMaintenanceActions (account-wide, paginated)
// and returns a Finding for every resource with pending maintenance.
// Severity is "~" (informational); IssueCount is always 0 (excluded from menu badge).
// The API returns maintenance actions for all RDS/DocDB resources (clusters AND instances).
// Pagination uses Marker; walks up to EnrichmentCap pages.
func EnrichRDSDocDBMaintenance(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	if clients.RDS == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	type maintenanceAction = rds.DescribePendingMaintenanceActionsOutput
	var allPages []*maintenanceAction
	var marker *string
	truncated := false
	pages := 0
	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := clients.RDS.DescribePendingMaintenanceActions(ctx, &rds.DescribePendingMaintenanceActionsInput{
			Marker: marker,
		})
		pages++
		if err != nil {
			return IssueEnricherResult{TruncatedIDs: truncatedIDs}, err
		}
		allPages = append(allPages, out)
		if out.Marker == nil {
			break
		}
		marker = out.Marker
	}
	// Collect probed resource IDs as an ordered slice for deterministic
	// suffix matching below. Using a map's random iteration order would
	// make key selection non-deterministic when two IDs both suffix-match
	// the same ARN (e.g. "foo-db" and "bar-foo-db").
	probeIDs := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			probeIDs = append(probeIDs, r.ID)
		}
	}
	// Emit a finding for every DB instance ARN that has pending maintenance.
	for _, page := range allPages {
		for _, action := range page.PendingMaintenanceActions {
			if action.ResourceIdentifier == nil {
				continue
			}
			arn := *action.ResourceIdentifier
			if !isInstanceARN(arn) {
				continue
			}
			// Collect action descriptions for the summary and rows.
			var actions []string
			var rows []resource.FindingRow
			for _, pa := range action.PendingMaintenanceActionDetails {
				if pa.Action != nil {
					actions = append(actions, *pa.Action)
				}
				// Emit a row per action detail.
				actionVal := ""
				if pa.Action != nil {
					actionVal = *pa.Action
				}
				applyMethod := ""
				if pa.OptInStatus != nil {
					applyMethod = *pa.OptInStatus
				}
				earliestTarget := ""
				if pa.AutoAppliedAfterDate != nil {
					earliestTarget = formatDate(pa.AutoAppliedAfterDate)
				} else if pa.ForcedApplyDate != nil {
					earliestTarget = formatDate(pa.ForcedApplyDate)
				}
				if actionVal != "" {
					rows = append(rows, resource.FindingRow{Label: "Action", Value: actionVal, Tier: "~"})
				}
				if applyMethod != "" {
					rows = append(rows, resource.FindingRow{Label: "Apply Method", Value: applyMethod})
				}
				if earliestTarget != "" {
					rows = append(rows, resource.FindingRow{Label: "Earliest Target", Value: earliestTarget, Tier: "~"})
				}
				if pa.Description != nil && *pa.Description != "" {
					rows = append(rows, resource.FindingRow{Label: "Description", Value: *pa.Description})
				}
			}
			summary := "pending maintenance"
			if len(actions) > 0 {
				summary = "pending maintenance: " + strings.Join(actions, ", ")
			}
			// Determine the key: prefer the longest matching probeID so that
			// when two IDs both suffix-match the same ARN (e.g. "foo-db" and
			// "bar-foo-db" for arn ":bar-foo-db"), the more specific one wins.
			// Iteration is over the ordered probeIDs slice — deterministic.
			key := ""
			for _, id := range probeIDs {
				if strings.HasSuffix(arn, ":"+id) && len(id) > len(key) {
					key = id
				}
			}
			if key == "" {
				// No probeID matched — this maintenance action targets a resource
				// not in the current input slice (e.g. dispatched for dbc, ARN is
				// for an instance; or page truncation evicted it). Append to
				continue
			}
			findings[key] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  summary,
				Rows:     rows,
			}
		}
	}
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings}, nil
}
