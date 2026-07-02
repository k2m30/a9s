// app_detail_attention_cursor_test.go — TDD red-phase pin for Finding A:
// injectAttentionSectionDetail (internal/app/detail_body.go) sorts Attention
// entries by tier ("!" before "~") before rendering, but attentionPrependCount
// (internal/app/detail_state.go ~line 71, called from applyFindingToState
// ~line 187) computes lastEntryBare from the ORIGINAL (unsorted) findings
// order. When a "~" (warning) finding WITH Detail text is followed by a bare
// "!" (broken) finding appended later:
//
//   - Unsorted order (what attentionPrependCount walks): [warn(detail), broken(bare)]
//     -> last iterated = broken(bare) -> lastEntryBare=true -> spacer OMITTED.
//   - Sorted order (what injectAttentionSectionDetail actually renders):
//     [broken(bare), warn(detail)] -> last rendered = warn(detail), NOT bare
//     -> spacer INCLUDED.
//
// This mismatch makes attentionPrependCount return a value 1 LOWER than the
// true prepend size, so applyFindingToState's cursor-delta adjustment
// (internal/app/detail_state.go ~line 231-247) shifts FieldCursor by the wrong
// amount after a wave-2 enrichment finding arrives, landing the cursor on the
// wrong logical field row.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// newAttentionCursorController builds a Controller with a ScreenDetail pushed
// for res/resourceType, ready to drive FieldCursor via ActionMoveBottom and
// inspect Snapshot().Body.Detail. Mirrors newDetailController in
// detail_render_parity_test.go (same package, kept local to avoid a
// cross-file rename dependency on that helper's visibility).
func newAttentionCursorController(t *testing.T, res resource.Resource, resourceType string) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenDetail},
	})
	c.EnsureDetailState(res, resourceType)
	return c
}

// fieldRowAt returns the FieldRow at idx in body.Fields, failing the test if
// idx is out of range.
func fieldRowAt(t *testing.T, body *app.DetailBody, idx int) app.FieldRow {
	t.Helper()
	if idx < 0 || idx >= len(body.Fields) {
		t.Fatalf("FieldCursor %d out of range for %d field rows", idx, len(body.Fields))
	}
	return body.Fields[idx]
}

// TestApplyDetailFinding_CursorStaysOnSameFieldAcrossMixedSeverityAttentionSort
// pins Finding A. Repro:
//
//  1. Seed a detail whose Attention has a "~" (warning) finding WITH Detail
//     text (wave-1, so it's present before any enrichment), plus a content
//     field after it.
//  2. Move FieldCursor to the bottom — lands on the last real content field
//     row (identified by its Key, not its index).
//  3. Apply a wave-2 "!" (broken) finding that is BARE (no Detail, no rows) —
//     this is the finding that gets appended LAST in unsorted order but
//     sorts FIRST ("!" before "~") when rendered.
//  4. Assert the cursor still points at the SAME logical field row (same Key)
//     after the enrichment lands — not shifted by the attentionPrependCount
//     off-by-one.
func TestApplyDetailFinding_CursorStaysOnSameFieldAcrossMixedSeverityAttentionSort(t *testing.T) {
	res := resource.Resource{
		ID:   "i-0aaa111111111111a",
		Name: "web-server",
		Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0aaa111111111111a",
			"state":       "running",
		},
		// Wave-1 "~" finding WITH Detail text (not bare) — seeded via
		// EnsureDetailState's res.Findings copy, so it is present in
		// ds.Findings BEFORE the wave-2 finding is appended.
		Findings: []domain.Finding{{
			Code:     "ec2.long-stopped",
			Phrase:   "instance stopped 42d ago",
			Detail:   "Instance stopped more than 30 days ago — review whether it is still needed.",
			Severity: domain.SevWarn,
			Source:   "wave1",
		}},
	}

	c := newAttentionCursorController(t, res, "ec2")

	// Move cursor to the bottom — lands on the last content field row
	// (skips section headers / spacers per ActionMoveBottom's detailFieldCount).
	c.Apply(app.Action{Kind: app.ActionMoveBottom})

	vs0 := c.Snapshot()
	if vs0.Body.Detail == nil {
		t.Fatal("precondition: Body.Detail is nil")
	}
	preCursor := vs0.Body.Detail.FieldCursor
	preRow := fieldRowAt(t, vs0.Body.Detail, preCursor)
	if preRow.IsSection || preRow.IsSpacer {
		t.Fatalf("precondition: cursor landed on a section/spacer row (Key=%q) — test fixture needs a real content field at the bottom", preRow.Key)
	}
	if preRow.Path == "Attention" {
		t.Fatalf("precondition: cursor landed inside the Attention block (Key=%q) — test fixture needs the cursor to start on a NON-attention content field", preRow.Key)
	}

	// Wave-2 "!" (broken) finding that is BARE: no Detail text, no
	// AttentionDetail rows. Appended AFTER the wave-1 "~" finding in
	// ds.Findings (unsorted order), but sorts BEFORE it ("!" first) in
	// injectAttentionSectionDetail's rendered order.
	broken := &domain.Finding{
		Code:     "ec2.instance-status-impaired",
		Phrase:   "impaired: system checks failing",
		Severity: domain.SevBroken,
		Source:   "wave2:ec2",
	}
	c.ApplyDetailFinding(broken, nil)

	vs1 := c.Snapshot()
	if vs1.Body.Detail == nil {
		t.Fatal("Body.Detail became nil after ApplyDetailFinding")
	}
	postCursor := vs1.Body.Detail.FieldCursor
	postRow := fieldRowAt(t, vs1.Body.Detail, postCursor)

	if postRow.Key != preRow.Key || postRow.Path != preRow.Path {
		t.Errorf(
			"cursor did not stay on the same logical field after mixed-severity Attention sort:\n"+
				"  before enrichment: FieldCursor=%d Key=%q Path=%q\n"+
				"  after  enrichment: FieldCursor=%d Key=%q Path=%q\n"+
				"attentionPrependCount (internal/app/detail_state.go) computed lastEntryBare from "+
				"the UNSORTED findings order (last appended = bare broken finding -> lastEntryBare=true "+
				"-> spacer omitted), but injectAttentionSectionDetail (internal/app/detail_body.go) "+
				"sorts \"!\" before \"~\" before rendering, so the ACTUAL last rendered entry is the "+
				"non-bare warning finding (spacer included). The prepend-count mismatch shifted "+
				"FieldCursor by the wrong delta.",
			preCursor, preRow.Key, preRow.Path,
			postCursor, postRow.Key, postRow.Path,
		)
	}
}
