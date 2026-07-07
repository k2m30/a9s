// qa_wave2_attention_keying_test.go — pins the second architectural finish
// Codex required for the v3.47.0 multi-finding work: AttentionDetail rows
// must be retrievable per FINDING, not per RESOURCE, so two independently-
// evaluated findings on the same resource can each carry their own
// supporting rows.
//
// CURRENT SHAPE (verified by direct read):
//   - IssueEnricherResult.AttentionDetails is map[string]domain.AttentionDetail
//     — keyed by Resource.ID ONLY (internal/aws/issue_enrichment.go:203).
//   - setWave2Finding's own doc comment (issue_enrichment.go:117-124) already
//     documents the gap this test pins: "rows, when non-empty, become this
//     resourceID's single AttentionDetail entry — the first call for a given
//     resourceID that supplies non-empty rows wins that slot... The
//     AttentionDetail is keyed by resourceID only (not per-Code), so a second
//     independently-evaluated condition with its own rows on the same
//     resourceID is a case this helper does not disambiguate." The doc
//     comment names the exact real-world case this test drives:
//     "opensearch's update-forced + encryption-off" (issue_enrichment.go:107).
//   - The fold layer, runtime.ApplyWave2ToRow (internal/runtime/helpers.go:70),
//     inherits the same one-slot-per-resource limit: it looks up ONE ad,
//     hasAD := attentionDetails[r.ID] (helpers.go:98) before the per-finding
//     loop, then attaches it to only the FIRST finding it appends via the
//     `if hasAD && len(ad.Rows) > 0 && !adAssigned` guard (helpers.go:119) —
//     every subsequent finding for that resource gets no AttentionDetails
//     entry at all, regardless of whether the enricher intended it to have
//     its own rows.
//   - One level further down the pipe, domain.Resource.AttentionDetails is
//     ALREADY map[domain.FindingCode]domain.AttentionDetail (internal/domain/
//     finding.go:45), and internal/app/detail_body.go's buildAttentionEntries
//     reads exactly attentionDetails[f.Code] per finding — so the per-code
//     keying this test wants already exists at the ROW level. The gap is one
//     level up: IssueEnricherResult.AttentionDetails (the enricher-result
//     level) and ApplyWave2ToRow's attentionDetails parameter (the fold-level
//     bridge) are still keyed by Resource.ID only, so there is nowhere for a
//     second finding's distinct rows to live before the row level even sees
//     them.
//
// INTENDED FIX: IssueEnricherResult.AttentionDetails must become
// map[string]map[domain.FindingCode]domain.AttentionDetail (Resource.ID, then
// FindingCode), and ApplyWave2ToRow's attentionDetails parameter (plus
// applyEnrichment's threading of it, internal/runtime/helpers.go:39) must
// follow — so setWave2Finding can record each independently-evaluated
// condition's own rows without a later call silently losing them.
//
// TEST SHAPE — behaviorally RED, not compile-red (a deliberate deviation from
// the dispatch's "may be compile-red" option, documented here per this
// project's verify-before-pinning discipline): tests/unit is compiled as a
// single Go package (unit / unit_test) — a compile-red file here would break
// `go build`/`go vet`/`go test` for every OTHER file in the package,
// including the sibling qa57 regression tests this same dispatch explicitly
// scoped as untouchable and disjoint. Attempting to assign the intended
// map[string]map[domain.FindingCode]domain.AttentionDetail shape directly
// into IssueEnricherResult.AttentionDetails would be a genuine compile error
// today, but the resulting package-wide build break has a larger blast
// radius than this dispatch's own "disjoint files" contract anticipates. The
// test below instead drives the REAL, exported runtime.ApplyWave2ToRow fold
// function with TODAY's real types end to end and asserts on its output —
// compiles clean, fails at runtime for the exact mechanism traced above, and
// requires exactly the same production fix (promoting AttentionDetails to be
// keyed by FindingCode, at both the IssueEnricherResult and ApplyWave2ToRow
// layers) to turn green.
package unit_test

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// TestWave2AttentionKeying_TwoFindingsOneResource_EachFindingsAttentionRowsSurviveIndependently
// drives the exact scenario issue_enrichment.go's own doc comment names as
// the motivating case for per-Code AttentionDetail keying: an OpenSearch
// domain with two independently-evaluated Wave-2 conditions (forced service
// update, encryption-at-rest disabled), each carrying its own supporting
// Attention rows.
func TestWave2AttentionKeying_TwoFindingsOneResource_EachFindingsAttentionRowsSurviveIndependently(t *testing.T) {
	const resourceID = "opensearch-domain-search-prod"
	const shortName = "opensearch"

	codeUpdateForced := domain.FindingCode("opensearch.update-forced")
	codeEncryptionOff := domain.FindingCode("opensearch.encryption-off")

	rowsUpdateForced := []domain.DetailRow{{Label: "Update Status", Value: "REQUIRED", Tier: "!"}}
	rowsEncryptionOff := []domain.DetailRow{{Label: "Encryption At Rest", Value: "disabled", Tier: "~"}}

	findingUpdateForced := domain.Finding{
		Code:     codeUpdateForced,
		Phrase:   "service software update forced",
		Severity: domain.SevBroken,
		Source:   "wave2:" + shortName,
	}
	findingEncryptionOff := domain.Finding{
		Code:     codeEncryptionOff,
		Phrase:   "encryption at rest disabled",
		Severity: domain.SevWarn,
		Source:   "wave2:" + shortName,
	}

	// ARRANGE: replicate exactly what two setWave2Finding(&result, resourceID,
	// ...) calls — one per independently-evaluated condition, each with its
	// OWN non-empty rows — produce under TODAY's IssueEnricherResult shape.
	// setWave2Finding is unexported (internal/aws/issue_enrichment.go:129) so
	// it cannot be called from tests/unit directly; its documented behavior
	// (doc comment lines 117-124, and the `if _, ok :=
	// r.AttentionDetails[resourceID]; !ok` guard at line 152 that enforces
	// it) means the SECOND call's rows — rowsEncryptionOff — have no
	// representation anywhere in IssueEnricherResult.AttentionDetails: the
	// map holds exactly one AttentionDetail per resourceID, so this ARRANGE
	// step reflects the only value two real setWave2Finding calls could ever
	// produce here.
	enricherResult := awsclient.IssueEnricherResult{
		Findings: map[string][]domain.Finding{
			resourceID: {findingUpdateForced, findingEncryptionOff},
		},
		AttentionDetails: map[string]domain.AttentionDetail{
			resourceID: {Rows: rowsUpdateForced},
		},
	}

	// ACT: fold through the real production seam, exactly as
	// Core.applyEnrichment (internal/runtime/helpers.go:36) does for every
	// row of a cached resource type.
	row := &domain.Resource{ID: resourceID}
	td := resource.ResourceTypeDef{ShortName: shortName}
	runtime.ApplyWave2ToRow(row, td, enricherResult.Findings, enricherResult.AttentionDetails)

	// Sanity: both findings must reach the row's Findings slice — this is
	// Test A's target bug class (a dropped Finding), not this test's
	// concern, so a failure here would mean the fixture is wrong, not that
	// this pin fired for the right reason.
	if len(row.Findings) != 2 {
		t.Fatalf("row.Findings = %+v, want 2 findings (codeUpdateForced and codeEncryptionOff) after ApplyWave2ToRow", row.Findings)
	}

	adUpdateForced, hasUpdateForced := row.AttentionDetails[codeUpdateForced]
	if !hasUpdateForced || len(adUpdateForced.Rows) != 1 || adUpdateForced.Rows[0] != rowsUpdateForced[0] {
		t.Errorf(
			"row.AttentionDetails[%q] = %+v (present=%v), want Rows=%v — the first-appended finding's own rows should survive the fold",
			codeUpdateForced, adUpdateForced, hasUpdateForced, rowsUpdateForced,
		)
	}

	adEncryptionOff, hasEncryptionOff := row.AttentionDetails[codeEncryptionOff]
	if !hasEncryptionOff || len(adEncryptionOff.Rows) != 1 || adEncryptionOff.Rows[0] != rowsEncryptionOff[0] {
		t.Errorf(
			"row.AttentionDetails[%q] = %+v (present=%v), want Rows=%v — FAILS TODAY: runtime.ApplyWave2ToRow "+
				"(internal/runtime/helpers.go:70) reads exactly one shared AttentionDetail per resourceID "+
				"(`ad, hasAD := attentionDetails[r.ID]` at helpers.go:98) and attaches it to only the FIRST "+
				"finding it appends (the `!adAssigned` guard at helpers.go:119) — the second "+
				"independently-evaluated finding's own rows are unrepresentable under "+
				"IssueEnricherResult.AttentionDetails' current map[string]domain.AttentionDetail shape "+
				"(issue_enrichment.go:203) and never reach internal/app/detail_body.go's "+
				"buildAttentionEntries, which reads exactly this row.AttentionDetails[f.Code] map when "+
				"rendering the detail view's Attention section. Fix requires promoting "+
				"IssueEnricherResult.AttentionDetails to map[string]map[domain.FindingCode]domain.AttentionDetail "+
				"(keyed by Resource.ID then FindingCode) so setWave2Finding can record each finding's own "+
				"rows independently, and updating ApplyWave2ToRow's attentionDetails parameter and fold "+
				"logic to match.",
			codeEncryptionOff, adEncryptionOff, hasEncryptionOff, rowsEncryptionOff,
		)
	}

	// Sanity: the update-forced finding must not have silently acquired the
	// OTHER finding's rows (the "merged" failure mode the dispatch also
	// warns against, distinct from "attached-to-first").
	if hasUpdateForced && len(adUpdateForced.Rows) > 0 && adUpdateForced.Rows[0] == rowsEncryptionOff[0] {
		t.Errorf("row.AttentionDetails[%q] leaked the OTHER finding's row %+v — rows must never merge across findings", codeUpdateForced, adUpdateForced.Rows[0])
	}
}
