// qa_wave2_attention_keying_test.go — AttentionDetail rows are retrievable
// per FINDING, not per RESOURCE, so two independently-evaluated findings on
// the same resource can each carry their own supporting rows.
//
// IssueEnricherResult.AttentionDetails is
// map[string]map[domain.FindingCode]domain.AttentionDetail (Resource.ID, then
// FindingCode), and runtime.ApplyWave2ToRow threads that nested map — so
// setWave2Finding records each independently-evaluated condition's own rows
// under its own Code, and a later call for the same resource never overwrites
// them. One level down, domain.Resource.AttentionDetails is
// map[domain.FindingCode]domain.AttentionDetail and buildAttentionEntries
// reads attentionDetails[f.Code] per finding, so each finding surfaces its own
// Attention rows end to end.
//
// This test drives the REAL, exported runtime.ApplyWave2ToRow fold and
// asserts each finding's rows survive independently — the exact case
// issue_enrichment.go names as motivating per-Code keying (opensearch's
// forced-update + encryption-off).
package unit_test

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
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
	// setWave2Finding is unexported (core/aws/issue_enrichment.go:129) so
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
		AttentionDetails: map[string]map[domain.FindingCode]domain.AttentionDetail{
			resourceID: {
				codeUpdateForced:  {Rows: rowsUpdateForced},
				codeEncryptionOff: {Rows: rowsEncryptionOff},
			},
		},
	}

	// ACT: fold through the real production seam, exactly as
	// Core.applyEnrichment (core/runtime/helpers.go:36) does for every
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
			"row.AttentionDetails[%q] = %+v (present=%v), want Rows=%v — each independently-"+
				"evaluated finding must keep its OWN AttentionDetail: setWave2Finding keys "+
				"IssueEnricherResult.AttentionDetails by (Resource.ID, FindingCode), and "+
				"ApplyWave2ToRow folds that nested map so buildAttentionEntries reads "+
				"row.AttentionDetails[f.Code] per finding when rendering the Attention section.",
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
