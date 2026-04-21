// rds_issue_enrichment.go — Wave 2 issue enrichment for the rds resource type.
package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

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
//
// Summary format:
//   - If the action has no scheduled dates (ForcedApplyDate and AutoAppliedAfterDate both nil):
//     "pending maintenance: <action1>, <action2>, ..." (always emitted).
//   - If the action has at least one date and it is in the past (overdue):
//     "Pending maintenance action overdue: <ActionType> (<Description>)."
//   - If the action has dates but all are in the future: no finding is emitted.
func EnrichRDSDocDBMaintenance(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	return enrichRDSDocDBMaintenanceAt(ctx, clients, resources, time.Now())
}

// enrichRDSDocDBMaintenanceAt is the inner testable form of EnrichRDSDocDBMaintenance.
// now is accepted as a parameter so tests can inject a fixed reference time.
func enrichRDSDocDBMaintenanceAt(ctx context.Context, clients *ServiceClients, resources []resource.Resource, now time.Time) (IssueEnricherResult, error) {
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

			summary, rows := rdsMaintenanceSummaryAndRows(action.PendingMaintenanceActionDetails, now)
			if summary == "" {
				// All actions have future dates — no finding.
				continue
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
				// for an instance; or page truncation evicted it).
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

// rdsMaintenanceSummaryAndRows builds the summary string and finding rows for a set
// of PendingMaintenanceAction details, applying the dual-format rule:
//
//   - Actions with no scheduled dates (both nil): always emit; summary uses
//     "pending maintenance: <action1>, <action2>, ..." format.
//   - Actions with at least one scheduled date: emit only if the date is in the past
//     (overdue); summary uses "Pending maintenance action overdue: <Action> (<Description>)."
//   - If ALL actions have future dates: returns ("", nil) — caller must skip the finding.
func rdsMaintenanceSummaryAndRows(details []rdstypes.PendingMaintenanceAction, now time.Time) (string, []resource.FindingRow) {
	// Separate actions into two buckets.
	type actionEntry struct {
		action      string
		description string
		applyMethod string
		earliestStr string
		overdue     bool // true = dated+past; false = undated (legacy)
	}

	// Empty details means the maintenance record exists but carries no action specifics.
	// Emit a generic "pending maintenance" finding (backward compat).
	if len(details) == 0 {
		return "pending maintenance", nil
	}

	var legacyActions []actionEntry // no dates
	var overdueActions []actionEntry

	for _, pa := range details {
		actionVal := ""
		if pa.Action != nil {
			actionVal = *pa.Action
		}
		desc := ""
		if pa.Description != nil {
			desc = *pa.Description
		}
		applyMethod := ""
		if pa.OptInStatus != nil {
			applyMethod = *pa.OptInStatus
		}
		earliestStr := ""
		if pa.AutoAppliedAfterDate != nil {
			earliestStr = formatDate(pa.AutoAppliedAfterDate)
		} else if pa.ForcedApplyDate != nil {
			earliestStr = formatDate(pa.ForcedApplyDate)
		}

		hasDates := pa.ForcedApplyDate != nil || pa.AutoAppliedAfterDate != nil
		if !hasDates {
			// Legacy: no date → always emit, old summary format.
			legacyActions = append(legacyActions, actionEntry{actionVal, desc, applyMethod, earliestStr, false})
			continue
		}
		// Has dates.
		if dbiActionOverdue(pa.ForcedApplyDate, pa.AutoAppliedAfterDate, now) {
			overdueActions = append(overdueActions, actionEntry{actionVal, desc, applyMethod, earliestStr, true})
		}
		// Future-dated: skip this action.
	}

	// No legacy and no overdue actions — all actions have future dates; skip finding.
	if len(legacyActions) == 0 && len(overdueActions) == 0 {
		return "", nil
	}

	// If we have overdue dated actions, use the new summary format (first action).
	// Prefer overdue actions over legacy when both are present.
	if len(overdueActions) > 0 {
		first := overdueActions[0]
		var summary string
		if first.description != "" {
			summary = fmt.Sprintf("Pending maintenance action overdue: %s (%s).", first.action, first.description)
		} else {
			summary = fmt.Sprintf("Pending maintenance action overdue: %s.", first.action)
		}
		var rows []resource.FindingRow
		for _, oa := range overdueActions {
			rows = appendMaintenanceRows(rows, oa.action, oa.applyMethod, oa.earliestStr, oa.description)
		}
		return summary, rows
	}

	// Legacy (undated) actions: old "pending maintenance: ..." format.
	var actionNames []string
	var rows []resource.FindingRow
	for _, la := range legacyActions {
		if la.action != "" {
			actionNames = append(actionNames, la.action)
		}
		rows = appendMaintenanceRows(rows, la.action, la.applyMethod, la.earliestStr, la.description)
	}
	summary := "pending maintenance"
	if len(actionNames) > 0 {
		summary = "pending maintenance: " + strings.Join(actionNames, ", ")
	}
	return summary, rows
}

// appendMaintenanceRows appends FindingRow entries for one maintenance action.
func appendMaintenanceRows(rows []resource.FindingRow, action, applyMethod, earliestStr, description string) []resource.FindingRow {
	if action != "" {
		rows = append(rows, resource.FindingRow{Label: "Action", Value: action, Tier: "~"})
	}
	if applyMethod != "" {
		rows = append(rows, resource.FindingRow{Label: "Apply Method", Value: applyMethod})
	}
	if earliestStr != "" {
		rows = append(rows, resource.FindingRow{Label: "Earliest Target", Value: earliestStr, Tier: "~"})
	}
	if description != "" {
		rows = append(rows, resource.FindingRow{Label: "Description", Value: description})
	}
	return rows
}
