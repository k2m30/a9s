// rds_snap_issue_enrichment.go — Wave-1 cross-ref enricher for rds-snap.
//
// The enricher is "Wave 2 = None" per docs/attention-signals.md (no background
// API calls). Instead it performs pure cross-ref logic against the dbi
// ResourceCache to detect two Wave-1 signals that require sibling-cache access:
//
//  1. orphan: parent DBInstanceIdentifier is NOT found in the loaded dbi cache.
//     Phrase: "orphan: source DB deleted"
//
//  2. past-retention: automated snapshot is older than the parent's
//     BackupRetentionPeriod. Only checked when the parent IS in the dbi cache.
//     Phrase: "automated, <N>d past retention"
//
// Both signals are emitted via IssueAppends (not Findings). FieldUpdates carries
// the merged §4 status phrase (BumpFindingSuffix applied when pre-existing issues exist).
// Findings is always empty; nil error always returned.
package aws

import (
	"context"
	"fmt"
	"time"

	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("rds-snap", enrichRDSSnapCrossRef, 100)
}

// enrichRDSSnapCrossRef is the Wave-1 cross-ref enricher for RDS snapshots.
// It detects orphan and past-retention signals via the dbi ResourceCache.
// Zero API calls; nil clients are safe.
func enrichRDSSnapCrossRef(
	_ context.Context,
	_ *ServiceClients,
	resources []resource.Resource,
	cache resource.ResourceCache,
) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]resource.EnrichmentFinding),
		TruncatedIDs: make(map[string]bool),
		IssueAppends: make(map[string][]string),
		FieldUpdates: make(map[string]map[string]string),
		IssueCount:   0,
		Truncated:    false,
	}

	// If the dbi cache is not loaded, cross-ref rules cannot fire (orphan
	// and past-retention both require a loaded sibling list). Return empty result.
	dbiEntry, dbiLoaded := cache["dbi"]
	if !dbiLoaded {
		return result, nil
	}

	// Build a lookup map: dbi ID → DBInstance struct for O(1) access.
	// Carry the full DBInstance so the EnrichmentFinding.Rows below can cite
	// concrete parent fields (BackupRetentionPeriod, etc.) without a second pass.
	dbiByID := make(map[string]rdstypes.DBInstance, len(dbiEntry.Resources))
	for _, dbiRes := range dbiEntry.Resources {
		db, ok := assertStruct[rdstypes.DBInstance](dbiRes.RawStruct)
		if !ok {
			continue
		}
		dbiByID[dbiRes.ID] = db
	}

	for _, res := range resources {
		snap, ok := assertStruct[rdstypes.DBSnapshot](res.RawStruct)
		if !ok {
			continue
		}

		// Extract parent instance identifier.
		parentID := ""
		if snap.DBInstanceIdentifier != nil {
			parentID = *snap.DBInstanceIdentifier
		}
		if parentID == "" {
			// No parent reference; skip cross-ref checks.
			continue
		}

		var newPhrases []string

		parent, parentFound := dbiByID[parentID]
		if !parentFound {
			// Orphan rule: parent not in the loaded dbi cache.
			newPhrases = append(newPhrases, "orphan: source DB deleted")
		} else {
			// Past-retention rule: only for automated snapshots with a parent
			// that has a positive BackupRetentionPeriod.
			snapType := ""
			if snap.SnapshotType != nil {
				snapType = *snap.SnapshotType
			}
			retention := int32(0)
			if parent.BackupRetentionPeriod != nil {
				retention = *parent.BackupRetentionPeriod
			}
			if snapType == "automated" && retention > 0 && snap.SnapshotCreateTime != nil {
				ageD := int(time.Since(*snap.SnapshotCreateTime).Hours() / 24)
				retentionD := int(retention)
				if ageD > retentionD {
					overD := ageD - retentionD
					phrase := fmt.Sprintf("automated, %dd past retention", overD)
					newPhrases = append(newPhrases, phrase)
				}
			}
		}

		if len(newPhrases) == 0 {
			continue
		}

		result.IssueAppends[res.ID] = newPhrases

		// Build merged §4 status phrase.
		// Pre-existing status from Wave-1 fetcher (res.Status) is the top phrase.
		// New phrases from this enricher are appended. BumpFindingSuffix is applied
		// once for each additional phrase beyond the first.
		mergedStatus := computeMergedStatus(res.Status, res.Issues, newPhrases)
		if result.FieldUpdates[res.ID] == nil {
			result.FieldUpdates[res.ID] = make(map[string]string)
		}
		result.FieldUpdates[res.ID]["status"] = mergedStatus

		// Emit a Wave-2 EnrichmentFinding for the detail-view's Attention
		// section. The cross-ref signals are conceptually Wave-1 (zero AWS
		// API calls — pure cache cross-ref), but the only path that reaches
		// the detail view from inside the enricher hook is via Findings —
		// the dispatcher wires `m.enrichmentFindings[type][id]` into
		// DetailModel at view-open time (app_handlers_navigate.go:110).
		// Severity "!" is used so unifiedIssueCount agrees with the Wave-1
		// ResolveColor path (both contribute the same instance, deduped).
		// Summary carries the §4 phrase verbatim — same string operators
		// see in the Status column. Rows expose the per-instance context
		// so the operator doesn't have to pivot to the dbi list to find
		// out which parent's retention window was overshot.
		summary := newPhrases[0]
		rows := []resource.FindingRow{}
		if !parentFound {
			rows = append(rows, resource.FindingRow{
				Label: "Source DB",
				Value: parentID + " (not in loaded dbi list)",
				Tier:  "!",
			})
		} else {
			rows = append(rows, resource.FindingRow{
				Label: "Source DB",
				Value: parentID,
				Tier:  "!",
			})
			if parent.BackupRetentionPeriod != nil {
				rows = append(rows, resource.FindingRow{
					Label: "Retention",
					Value: fmt.Sprintf("%d days", *parent.BackupRetentionPeriod),
					Tier:  "!",
				})
			}
			if snap.SnapshotCreateTime != nil {
				rows = append(rows, resource.FindingRow{
					Label: "Created",
					Value: snap.SnapshotCreateTime.Format("2006-01-02"),
					Tier:  "!",
				})
			}
		}
		result.Findings[res.ID] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  summary,
			Rows:     rows,
		}
	}

	return result, nil
}

// computeMergedStatus builds the final §4 status phrase when the enricher appends
// new phrases to an existing set of issues.
//
// Logic:
//   - If existingIssues is empty (no pre-existing Wave-1 issues), the first new phrase
//     becomes the status phrase. If there are more new phrases, BumpFindingSuffix is
//     applied. existingStatus is ignored in this case — for healthy snaps it is ""
//     (§4 phrase design: empty = healthy), and the cross-ref phrase replaces it.
//   - If existingIssues is non-empty, the existing top phrase (existingStatus) remains
//     the status but BumpFindingSuffix is applied once per new phrase added (since each
//     new phrase expands the hidden-count).
func computeMergedStatus(existingStatus string, existingIssues []string, newPhrases []string) string {
	totalIssues := len(existingIssues) + len(newPhrases)

	if len(existingIssues) == 0 {
		// No pre-existing Wave-1 issues — new phrases are the only issues.
		if len(newPhrases) == 0 {
			return ""
		}
		topPhrase := newPhrases[0]
		if totalIssues == 1 {
			return topPhrase
		}
		return resource.BumpFindingSuffix(topPhrase)
	}

	// Pre-existing Wave-1 issues exist: the existing top phrase is the status.
	// Apply BumpFindingSuffix for each new phrase appended.
	status := existingStatus
	for range newPhrases {
		status = resource.BumpFindingSuffix(status)
	}
	return status
}
