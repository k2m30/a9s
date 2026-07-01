// dbi_issue_enrichment.go — Wave 2 enrichment for dbi: DescribePendingMaintenanceActions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// dbi canonical FindingCodes.
const (
	dbiCodePendingMaintenance domain.FindingCode = "dbi.pending-maintenance"
)

// EnrichDBIMaintenance calls DescribePendingMaintenanceActions (account-wide, paginated)
// and emits one Finding per dbi instance with pending maintenance. Severity "~" —
// IssueCount is always 0 (Wave 2 ~ does not bump the S1 menu badge). The merged
// S4 status phrase (e.g. "maintenance scheduled" alone, or "stopped (+1)" stacked
// over a Wave-1 finding) is computed at render time from r.Findings via
// phraseFromFindings; this enricher only emits Findings.
func EnrichDBIMaintenance(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}

	if clients == nil || clients.RDS == nil {
		return result, nil
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
			return result, err
		}
		allActions = append(allActions, out.PendingMaintenanceActions...)
		if out.Marker == nil || *out.Marker == "" {
			break
		}
		marker = out.Marker
	}

	// Deterministic ARN-suffix matching via ordered probeIDs. There is no
	// parallel statusByID map: the merged S4 phrase (single-finding or
	// Wave-1+Wave-2 stacked) is computed at render time from r.Findings, so
	// the enricher does not read the fetcher's status overlay here.
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
		// the Attention section does not render duplicated content.
		var rows []domain.DetailRow
		for _, pa := range action.PendingMaintenanceActionDetails {
			if pa.Action != nil && *pa.Action != "" {
				rows = append(rows, domain.DetailRow{Label: "Action", Value: *pa.Action, Tier: "~"})
			}
			if pa.OptInStatus != nil && *pa.OptInStatus != "" {
				rows = append(rows, domain.DetailRow{Label: "Apply Method", Value: *pa.OptInStatus})
			}
			if pa.AutoAppliedAfterDate != nil {
				rows = append(rows, domain.DetailRow{Label: "Earliest Target", Value: formatDate(pa.AutoAppliedAfterDate), Tier: "~"})
			} else if pa.ForcedApplyDate != nil {
				rows = append(rows, domain.DetailRow{Label: "Earliest Target", Value: formatDate(pa.ForcedApplyDate), Tier: "~"})
			}
			if pa.Description != nil && *pa.Description != "" {
				rows = append(rows, domain.DetailRow{Label: "Description", Value: *pa.Description})
			}
		}

		setWave2Finding(&result, key, dbiCodePendingMaintenance, "pending maintenance", "~", "dbi", rows)
	}

	result.IssueCount = 0 // "~" findings never bump the S1 badge
	result.Truncated = truncated
	return result, nil
}

