// tui_viewstate_purity_detail_test.go — Class B duplication pin for the
// detail pipeline (detail_body.go vs detail_fields.go).
//
// Investigation finding (see internal/tui/views/detail_helpers.go
// RenderDetail ~line 616 and renderDetailFieldsFromBody ~line 792-829):
// the TUI's RenderDetail(body) is ALREADY a pure consumer of body.Fields —
// it converts []app.FieldRow to []fieldpath.FieldItem 1:1 and never re-runs
// domainItemToFieldItem/injectAttentionSection on raw resource.Resource data.
// There is no "Class A" (renderer re-derivation) duplication to pin here,
// unlike the list pipeline (see tui_viewstate_purity_list_test.go).
//
// What DOES duplicate the projector pipeline is Class B: two independent
// PRODUCTION implementations of the same "resource.Resource → field list"
// pipeline exist:
//   - internal/tui/views/detail_fields.go: buildFieldList/sectionsToFieldItems/
//     domainItemToFieldItem/injectAttentionSection (consumed by TUI's legacy
//     View() path, and by nothing else).
//   - internal/app/detail_body.go: buildDetailFieldItems/sectionsToFieldItemsDetail/
//     domainItemToFieldItemDetail/injectAttentionSectionDetail (consumed by
//     buildDetailBody, which both the TUI's RenderDetail(body) path and the
//     web renderer consume).
//
// Per the task's own fallback instruction: "if the TUI detail already
// consumes DetailBody verbatim... report that Class B duplication is
// app-internal only... and pin the CONSISTENCY instead: same inputs →
// byte-identical field lists from both paths." This file is that pin.
//
// tests/unit/detail_render_parity_test.go ALREADY exercises this exact
// contract (View() = legacy detail_fields.go path; RenderDetail(body) =
// buildDetailBody path; asserted byte-identical across 8 resource-type ×
// scenario combinations including S3/S4/S5/S6/S11/S15 Attention-block
// scenarios). This file adds one additional, narrowly-targeted sweep across
// EVERY catalog resource type (not just the 8-type fixture list in
// detail_render_parity_test.go) with an issue-severity finding + supporting
// detail rows, so a Class-B drift introduced for a resource type outside
// that fixture list is still caught.
package unit_test

import (
	"fmt"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// TestViewStatePurity_Detail_FieldListConsistencyAcrossAllTypes sweeps every
// registered resource type with an issue-severity finding + AttentionDetail
// rows (the highest-divergence-risk scenario, per detail_render_parity_test.go's
// own comments) and asserts View() (detail_fields.go pipeline) produces
// byte-identical output to RenderDetail(body) (detail_body.go pipeline).
//
// A mismatch here is a genuine Class-B drift: the two independent projector
// implementations disagreed on a resource type that the fixture-based sweep
// in detail_render_parity_test.go does not cover.
func TestViewStatePurity_Detail_FieldListConsistencyAcrossAllTypes(t *testing.T) {
	tuitest.NoColor(t)

	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Fatal("resource.AllResourceTypes() returned empty slice — catalog not registered")
	}

	k := keys.Default()
	const narrowW = 40 // right column never shows — clean left-panel signal only

	for _, td := range allTypes {
		td := td
		t.Run(td.ShortName, func(t *testing.T) {
			res := resource.Resource{
				ID:   fmt.Sprintf("%s-consistency-001", td.ShortName),
				Name: fmt.Sprintf("consistency-%s-01", td.ShortName),
				Fields: map[string]string{
					"name":   fmt.Sprintf("consistency-%s-01", td.ShortName),
					"status": "active",
				},
			}
			f := &domain.Finding{
				Code:     domain.FindingCode(td.ShortName + ".purity-test"),
				Phrase:   "purity consistency check finding",
				Severity: domain.SevBroken,
				Source:   "wave2:test",
			}
			ad := &domain.AttentionDetail{
				Rows: []domain.DetailRow{
					{Label: "Evidence", Value: "synthetic-evidence-value"},
				},
			}

			m := views.NewDetail(res, td.ShortName, nil, k)
			m.SetSize(narrowW, 30)
			m.SetEnrichmentFinding(f, ad)
			legacy := m.View()

			t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
			s := session.New()
			s.Profile = "test-profile"
			s.Region = "us-east-1"
			core := runtime.New(s, nil)
			c := app.New(core)
			c.ApplyIntents([]runtime.UIIntent{
				runtime.PushScreen{ID: runtime.ScreenDetail},
			})
			c.EnsureDetailState(res, td.ShortName)
			c.ApplyDetailFinding(f, ad)
			body := *c.Snapshot().Body.Detail

			got := m.RenderDetail(body)
			if got != legacy {
				t.Errorf("[%s] Class-B drift: View() (detail_fields.go pipeline) and "+
					"RenderDetail(body) (detail_body.go pipeline) disagree given the SAME "+
					"resource+finding input — the two production projector implementations "+
					"must stay byte-identical.\n--- View() ---\n%s\n--- RenderDetail(body) ---\n%s",
					td.ShortName, legacy, got)
			}
		})
	}
}
