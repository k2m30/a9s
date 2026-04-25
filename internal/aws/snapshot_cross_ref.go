// snapshot_cross_ref.go — generic Wave-1 cross-ref enricher pattern shared
// across snapshot resource types (dbi-snap, dbc-snap, future ebs-snap).
//
// Two signals fire from sibling-cache scans (zero AWS API calls):
//
//   1. orphan: snapshot's parent identifier NOT found in the loaded parent
//      cache, AND the cache is not truncated.
//      Phrase: configurable per-type (e.g. "orphan: source DB deleted").
//
//   2. past-retention: automated snapshot older than the parent's
//      `BackupRetentionPeriod` (1.0× — no multiplier; the operator's
//      declared retention IS the policy). Only fires when the parent IS in
//      the cache, the snapshot is "automated", and the parent retention > 0.
//      Phrase: configurable per-type
//      (e.g. "automated, <N>d past retention").
//
// Wave classification stays Wave 1 (zero SDK calls) — the helper scans the
// in-memory ResourceCache only. Both signals route through
// `IssueEnricherResult.Findings` (which surfaces in S5 Attention) plus
// `FieldUpdates["status"]` (which merges into S4 via computeMergedStatus).
// Re-runs are idempotent: Findings and FieldUpdates are map-keyed so a second
// pass overwrites the first; computeMergedStatus reads the FETCHER's
// `Resource.Status`, never a previously-merged value, so suffixes never
// accumulate.
//
// Retention-rule-disabled mode: when a future consumer's parent type has no
// retention concept (e.g. ebs-snap on ec2.Volume), set
// `RetentionEnabled: false` and only the orphan rule fires.
package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// SnapshotCrossRefConfig parameterizes the cross-ref enricher pattern shared
// across snapshot resource types.
type SnapshotCrossRefConfig struct {
	// ParentShortName is the resource type whose ResourceCache entry the
	// helper scans for parents (e.g. "dbi", "dbc", "ec2").
	ParentShortName string

	// GetParentID extracts the parent identifier from a snapshot's RawStruct.
	// Returns ("", false) when the snapshot has no parent reference — the
	// helper skips the row in that case (no orphan, no retention).
	GetParentID func(snapRaw any) (string, bool)

	// GetCreatedAt extracts the snapshot creation time from RawStruct.
	// Returns (zero, false) if absent — past-retention rule skips this row.
	// Required when RetentionEnabled is true; may be nil otherwise.
	GetCreatedAt func(snapRaw any) (time.Time, bool)

	// GetSnapshotType extracts the snapshot type ("automated"/"manual"/etc).
	// Returns ("", false) if absent — past-retention rule skips this row.
	// Required when RetentionEnabled is true; may be nil otherwise.
	GetSnapshotType func(snapRaw any) (string, bool)

	// GetParentRetention extracts the BackupRetentionPeriod (in days) from
	// the parent's RawStruct. Returns (0, false) when absent or zero — the
	// past-retention rule will not fire on this row.
	// Required when RetentionEnabled is true; may be nil otherwise.
	GetParentRetention func(parentRaw any) (int32, bool)

	// OrphanPhrase is the §4 status phrase emitted when the parent is missing
	// from the loaded cache (and the cache is NOT truncated).
	OrphanPhrase string

	// ParentRowLabel is the FindingRow label used to cite the parent in the
	// detail-view Attention section (e.g. "Source DB" for dbi, "Source
	// Cluster" for dbc). It is reused for both the orphan citation row AND
	// the past-retention parent-cite row.
	ParentRowLabel string

	// RetentionPhrase formats the past-retention status phrase given days-over.
	// Example: func(d int) string { return fmt.Sprintf("automated, %dd past retention", d) }
	// Required when RetentionEnabled is true; may be nil otherwise.
	RetentionPhrase func(daysOver int) string

	// RetentionEnabled gates the past-retention rule. Set false for snapshot
	// types whose parent has no retention concept (e.g. future ebs-snap, where
	// ec2.Volume has no BackupRetentionPeriod). When false, only the orphan
	// rule fires; GetParentRetention/GetSnapshotType/GetCreatedAt/RetentionPhrase
	// may be nil.
	RetentionEnabled bool

	// Severity is the severity tier emitted on every FindingRow and on the
	// EnrichmentFinding.Severity for this enricher's output. "!" for the
	// existing snapshot consumers (orphan + past-retention are operator-
	// actionable). Future consumers may use "~" for informational signals.
	Severity string
}

// EnrichSnapshotCrossRef returns an IssueEnricherFunc that applies the
// orphan + past-retention pattern parameterized by cfg. The returned closure
// is registered via registerIssueEnricher in the per-resource enricher file.
//
// Contract: zero API calls, idempotent on repeated runs (Findings and
// FieldUpdates both overwrite per resource ID; reads res.Status which is the
// fetcher-emitted value, never a previously-merged one).
func EnrichSnapshotCrossRef(cfg SnapshotCrossRefConfig) IssueEnricherFunc {
	return func(_ context.Context, _ *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error) {
		// Default severity to "!" (operator-actionable) when callers omit the field.
		severity := cfg.Severity
		if severity == "" {
			severity = "!"
		}

		result := IssueEnricherResult{
			Findings:     make(map[string]resource.EnrichmentFinding),
			TruncatedIDs: make(map[string]bool),
			FieldUpdates: make(map[string]map[string]string),
		}

		// Skip rule per spec §3.1: the cross-ref enricher requires the parent
		// list to be loaded. If absent, both rules silently skip.
		parentEntry, parentLoaded := cache[cfg.ParentShortName]
		if !parentLoaded {
			return result, nil
		}

		// Build a lookup map: parent ID → RawStruct for O(1) access.
		parentByID := make(map[string]any, len(parentEntry.Resources))
		for _, p := range parentEntry.Resources {
			parentByID[p.ID] = p.RawStruct
		}

		for _, res := range resources {
			parentID, ok := cfg.GetParentID(res.RawStruct)
			if !ok || parentID == "" {
				continue
			}

			parentRaw, parentFound := parentByID[parentID]

			var newPhrases []string
			var rows []resource.FindingRow

			switch {
			case !parentFound:
				// Orphan rule: when the cache is truncated AND the parent
				// isn't in the visible window, absence is non-definitive —
				// skip rather than emit a false positive.
				if parentEntry.IsTruncated {
					continue
				}
				newPhrases = append(newPhrases, cfg.OrphanPhrase)
				rows = append(rows, resource.FindingRow{
					Label: cfg.ParentRowLabel,
					Value: parentID + " (not in loaded list)",
					Tier:  severity,
				})
			case cfg.RetentionEnabled:
				// Past-retention rule: only for "automated" snapshots whose
				// parent has a positive BackupRetentionPeriod.
				snapType, hasType := cfg.GetSnapshotType(res.RawStruct)
				createdAt, hasCreated := cfg.GetCreatedAt(res.RawStruct)
				retention, hasRetention := cfg.GetParentRetention(parentRaw)
				if !hasType || !hasCreated || !hasRetention {
					continue
				}
				if snapType != "automated" || retention <= 0 {
					continue
				}
				ageD := int(time.Since(createdAt).Hours() / 24)
				retentionD := int(retention)
				if ageD <= retentionD {
					continue
				}
				overD := ageD - retentionD
				newPhrases = append(newPhrases, cfg.RetentionPhrase(overD))
				rows = append(rows, resource.FindingRow{
					Label: cfg.ParentRowLabel,
					Value: parentID,
					Tier:  severity,
				})
				rows = append(rows, resource.FindingRow{
					Label: "Retention",
					Value: fmt.Sprintf("%d days", retention),
					Tier:  severity,
				})
				rows = append(rows, resource.FindingRow{
					Label: "Created",
					Value: createdAt.Format("2006-01-02"),
					Tier:  severity,
				})
			default:
				// Parent found but retention rule disabled — nothing to emit.
				continue
			}

			if len(newPhrases) == 0 {
				continue
			}

			// FieldUpdates carries the merged §4 status phrase. Idempotent:
			// reads res.Status (fetcher-emitted), never a previously-merged
			// value, so re-runs converge.
			result.FieldUpdates[res.ID] = map[string]string{
				"status": computeMergedStatus(res.Status, res.Issues, newPhrases),
			}

			// Findings emits the entries for the detail-view Attention section.
			result.Findings[res.ID] = resource.EnrichmentFinding{
				Severity: severity,
				Summary:  newPhrases[0],
				Rows:     rows,
			}
		}

		return result, nil
	}
}

// computeMergedStatus builds the §4 status phrase when the cross-ref enricher
// adds phrases to an existing fetcher-emitted set of Wave-1 phrases.
//
//   - existingStatus is the FETCHER's Resource.Status (the §4 top phrase plus
//     any (+N) suffix the fetcher already emitted). Reading from this — not
//     from a previously-merged value — keeps the function idempotent: the
//     enricher can run repeatedly and the suffix never accumulates.
//   - existingIssues is the fetcher's Resource.Issues slice (count of fetcher
//     phrases). Used only for total-count math.
//   - newPhrases are the phrases this enricher contributes.
//
// Returned status = top phrase + " (+N-1)" where N is the total count, or the
// bare top phrase when N == 1.
//
// Single-issue rule: when totalIssues == 1, reading existingStatus when
// non-empty preserves the fetcher's §4 phrase. The fetcher contract (U7f)
// requires Issues to be populated for every active Wave-1 phrase, so an
// existingStatus with empty Issues should not occur — if it does, that's a
// fetcher bug, not a helper concern.
func computeMergedStatus(existingStatus string, existingIssues []string, newPhrases []string) string {
	totalIssues := len(existingIssues) + len(newPhrases)

	if totalIssues == 0 {
		return ""
	}
	if totalIssues == 1 {
		if existingStatus != "" {
			return existingStatus
		}
		return newPhrases[0]
	}
	// N ≥ 2: pick the top phrase (fetcher's, if present; else the first new
	// phrase) and apply BumpFindingSuffix once per *additional* phrase beyond
	// it, so the suffix matches (totalIssues - 1).
	top := existingStatus
	startBumps := len(newPhrases)
	if top == "" {
		top = newPhrases[0]
		startBumps = len(newPhrases) - 1
	}
	// existingIssues beyond index 0 already contributed to the fetcher's
	// suffix encoded in existingStatus, so we only bump once per *new* phrase.
	status := top
	for i := 0; i < startBumps; i++ {
		status = resource.BumpFindingSuffix(status)
	}
	return status
}
