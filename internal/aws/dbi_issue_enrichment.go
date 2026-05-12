// dbi_issue_enrichment.go — Wave 2 enrichment for dbi: DescribePendingMaintenanceActions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("dbi", EnrichDBIMaintenance, 10)
}

// EnrichDBIMaintenance calls DescribePendingMaintenanceActions (account-wide, paginated)
// and emits one Finding per dbi instance with pending maintenance. Severity "~" —
// IssueCount is always 0 (Wave 2 ~ does not bump the S1 menu badge). The merged
// S4 status phrase (e.g. "maintenance scheduled" alone, or "stopped (+1)" stacked
// over a Wave-1 finding) is computed at render time from r.Findings via
// phraseFromFindings; this enricher only emits Findings.
func EnrichDBIMaintenance(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)

	if clients == nil || clients.RDS == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs, FieldUpdates: make(map[string]map[string]string)}, nil
	}

	// Paginate with a cap.
	var allActions []rdstypes.ResourcePendingMaintenanceActions
	var marker *string
	truncated := false
	pages := 0
	for {
		if pages >= EnrichmentCap {
			truncated = true
			break
		}
		out, err := clients.RDS.DescribePendingMaintenanceActions(ctx, &rds.DescribePendingMaintenanceActionsInput{Marker: marker})
		pages++
		if err != nil {
			return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs, FieldUpdates: make(map[string]map[string]string)}, err
		}
		allActions = append(allActions, out.PendingMaintenanceActions...)
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		marker = out.Marker
	}

	// Deterministic ARN-suffix matching via ordered probeIDs. AS-140 removed
	// the parallel statusByID map: the merged S4 phrase (single-finding or
	// Wave-1+Wave-2 stacked) is computed at render time from r.Findings, so
	// the enricher no longer needs to read the fetcher's status overlay here.
	probeIDs := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			probeIDs = append(probeIDs, r.ID)
		}
	}

	for _, action := range allActions {
		if action.ResourceIdentifier == nil {
			continue
		}
		arn := *action.ResourceIdentifier
		if !isInstanceARN(arn) {
			continue // dbc / other RDS resources — not dbi
		}
		// Find the longest matching probeID (specificity wins over prefix).
		key := ""
		for _, id := range probeIDs {
			if strings.HasSuffix(arn, ":"+id) && len(id) > len(key) {
				key = id
			}
		}
		if key == "" {
			continue
		}

		// Summary is the short S5 phrase; every concrete fact (Action,
		// Description, Earliest Target, Apply Method) lives only in Rows so
		// the Attention section does not render duplicated content. See the
		// contract on resource.EnrichmentFinding.
		var rows []resource.FindingRow
		for _, pa := range action.PendingMaintenanceActionDetails {
			if pa.Action != nil && *pa.Action != "" {
				rows = append(rows, resource.FindingRow{Label: "Action", Value: *pa.Action, Tier: "~"})
			}
			if pa.OptInStatus != nil && *pa.OptInStatus != "" {
				rows = append(rows, resource.FindingRow{Label: "Apply Method", Value: *pa.OptInStatus})
			}
			if pa.AutoAppliedAfterDate != nil {
				rows = append(rows, resource.FindingRow{Label: "Earliest Target", Value: formatDate(pa.AutoAppliedAfterDate), Tier: "~"})
			} else if pa.ForcedApplyDate != nil {
				rows = append(rows, resource.FindingRow{Label: "Earliest Target", Value: formatDate(pa.ForcedApplyDate), Tier: "~"})
			}
			if pa.Description != nil && *pa.Description != "" {
				rows = append(rows, resource.FindingRow{Label: "Description", Value: *pa.Description})
			}
		}

		findings[key] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  "pending maintenance",
			Rows:     rows,
		}
	}

	return IssueEnricherResult{
		IssueCount:   0, // "~" findings never bump the S1 badge
		Truncated:    truncated,
		TruncatedIDs: truncatedIDs,
		Findings:     findings,
		FieldUpdates: make(map[string]map[string]string),
	}, nil
}

