// rds_snap_issue_enrichment.go — cross-ref enricher for dbi-snap.
//
// The enricher detects two signals from docs/resources/dbi-snap.md §3.1
// that require sibling-cache access (and therefore can't run inside the
// fetcher):
//
//  1. orphan: parent DBInstanceIdentifier is NOT found in the loaded dbi cache.
//     Phrase: "orphan: source DB deleted"
//
//  2. past-retention: automated snapshot is older than the parent's
//     BackupRetentionPeriod. Only checked when the parent IS in the dbi cache.
//     Phrase: "automated, <N>d past retention"
//
// Wave classification (zero AWS API calls) is unchanged: this enricher makes
// no SDK calls — it scans the in-memory dbi ResourceCache. The signals are
// emitted via the IssueEnricherResult.Findings channel (with Rows) plus
// FieldUpdates["status"], because Findings is the only enricher-output
// channel that reaches the detail view's Attention section AND survives
// repeated enrichment passes (Findings is overwritten per run, not appended).
// IssueAppends is intentionally NOT used here — appending to Resource.Issues
// is non-idempotent on re-runs and would duplicate Attention entries that
// already render via Findings.
//
// docs/resources/dbi-snap.md §4 surface mapping was updated in the same
// commit to acknowledge cross-ref signals reach S1+S3+S5.
package aws

import (
	"context"
	"fmt"
	"time"

	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("dbi-snap", enrichDBISnapCrossRef, 100)
}

// enrichDBISnapCrossRef is the cross-ref enricher for RDS snapshots. Zero
// API calls; nil clients are safe; idempotent on repeated runs (Findings
// and FieldUpdates both overwrite per resource ID).
func enrichDBISnapCrossRef(
	_ context.Context,
	_ *ServiceClients,
	resources []resource.Resource,
	cache resource.ResourceCache,
) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]resource.EnrichmentFinding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
		IssueCount:   0,
		Truncated:    false,
	}

	// If the dbi cache is not loaded, cross-ref rules cannot fire (orphan
	// and past-retention both require a loaded sibling list). Spec §3.1:
	// "Skip the rule when the dbi list has not been loaded in this session."
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
			// Orphan rule (P2): when the dbi cache is truncated AND the
			// parent isn't in the visible window, absence is not definitive
			// — the parent may be on a later page. Skip the orphan signal
			// in that case rather than emit a false positive. Same applies
			// to past-retention (the rule below is gated on parentFound).
			if dbiEntry.IsTruncated {
				continue
			}
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

		// FieldUpdates carries the merged §4 status phrase.
		// Idempotent: existingStatus reads res.Status (the FETCHER-emitted
		// value, not a previous merge), so re-runs converge.
		mergedStatus := computeMergedStatus(res.Status, res.Issues, newPhrases)
		result.FieldUpdates[res.ID] = map[string]string{"status": mergedStatus}

		// Findings emits the entries for the detail-view Attention section.
		// Idempotent: handleEnrichmentChecked overwrites m.enrichmentFindings
		// keyed by resource ID per run (no append).
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

// computeMergedStatus builds the §4 status phrase when the cross-ref enricher
// adds phrases to an existing fetcher-emitted set of Wave-1 phrases.
//
//   - existingStatus is the FETCHER's Resource.Status (the §4 top phrase plus
//     any (+N) suffix the fetcher already emitted).  Reading from this — not
//     from a previously-merged value — keeps the function idempotent: the
//     enricher can run repeatedly and the suffix never accumulates.
//   - existingIssues is the fetcher's Resource.Issues slice (count of fetcher
//     phrases). Used only for total-count math.
//   - newPhrases are the phrases this enricher contributes.
//
// Returned status = top phrase + " (+N-1)" where N is the total count, or the
// bare top phrase when N == 1.
func computeMergedStatus(existingStatus string, existingIssues []string, newPhrases []string) string {
	totalIssues := len(existingIssues) + len(newPhrases)

	if totalIssues == 0 {
		return ""
	}
	if totalIssues == 1 {
		// Exactly one phrase across both sides. If it came from the fetcher,
		// existingStatus is non-empty; if it came from us, fall back to the
		// new phrase.
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
		// First new phrase IS the top; remaining new phrases each bump.
		startBumps = len(newPhrases) - 1
	}
	// existingIssues beyond index 0 already contributed to the fetcher's
	// suffix encoded in existingStatus (e.g. "publicly accessible (+1)"), so
	// we only bump once per *new* phrase the enricher adds.
	status := top
	for i := 0; i < startBumps; i++ {
		status = resource.BumpFindingSuffix(status)
	}
	return status
}
