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
// IssueCount is always 0 (Wave 2 ~ does not bump the S1 menu badge). When the
// resource is Healthy (fetcher Status == ""), the enricher sets
// FieldUpdates[id]["status"] = "maintenance scheduled" so the S4 column shows
// the short cause. When Wave 1 already populated Status, the enricher bumps the
// (+N) suffix on the existing phrase (universal rule 7) so the operator sees
// there is more to open for.
func EnrichDBIMaintenance(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	truncatedIDs := make(map[string]bool)
	fieldUpdates := make(map[string]map[string]string)

	if clients == nil || clients.RDS == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs, FieldUpdates: fieldUpdates}, nil
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
			return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs, FieldUpdates: fieldUpdates}, err
		}
		allActions = append(allActions, out.PendingMaintenanceActions...)
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		marker = out.Marker
	}

	// Deterministic ARN-suffix matching via ordered probeIDs.
	probeIDs := make([]string, 0, len(resources))
	statusByID := make(map[string]string, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			probeIDs = append(probeIDs, r.ID)
			statusByID[r.ID] = r.Status
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

		// Emit S4 FieldUpdate: if Healthy, set "maintenance scheduled"; if Wave 1 already
		// populated Status, bump the (+N) suffix so the operator sees there's more to open for.
		existing := statusByID[key]
		var newStatus string
		if existing == "" {
			// Healthy + Wave 2 → sole finding, no suffix
			newStatus = "maintenance scheduled"
		} else {
			// Wave 1 + Wave 2 stack → bump (+N) suffix on existing phrase
			newStatus = resource.BumpFindingSuffix(existing)
		}
		fieldUpdates[key] = map[string]string{"status": newStatus}
	}

	return IssueEnricherResult{
		IssueCount:   0, // "~" findings never bump the S1 badge
		Truncated:    truncated,
		TruncatedIDs: truncatedIDs,
		Findings:     findings,
		FieldUpdates: fieldUpdates,
	}, nil
}

