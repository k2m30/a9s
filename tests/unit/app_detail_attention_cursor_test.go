// app_detail_attention_cursor_test.go — the cursor must stay on the same
// logical field when the Attention block changes size under it.
//
// Both pins here are regressions of the same class: the FieldCursor delta in
// applyFindingToState (core/app/detail_state.go) needs the size of the block
// the user is actually looking at. It used to walk the entries a second time
// to derive that size, and a second walk can disagree with the render —
// Finding A because it walked them UNSORTED while the renderer sorts "!"
// before "~" (so it picked the wrong "last entry" and dropped the trailing
// spacer from its count), Finding B because it read session state the runtime
// had already moved. buildAttentionSectionDetail now reports the size it
// emitted, snapshot() records it with the identity of the row under the
// cursor, and the relocation reads that record — so the class is closed by
// construction: there is no second walk left to disagree.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
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
	t.Cleanup(c.Close)
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
//     after the enrichment lands — not shifted by a prepend-size off-by-one.
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
	// buildAttentionSectionDetail's rendered order.
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
				"the prepend size must be the one buildAttentionSectionDetail actually emitted: "+
				"it sorts \"!\" before \"~\", so the last RENDERED entry here is the non-bare warning "+
				"finding (spacer included), not the bare broken one appended last. A size derived "+
				"from the unsorted order is one lower and shifts FieldCursor by the wrong delta.",
			preCursor, preRow.Key, preRow.Path,
			postCursor, postRow.Key, postRow.Path,
		)
	}
}

// TestApplyDetailFinding_CursorStaysOnSameFieldWhenNotInspectedMarkArrivesToo
// pins Finding B. The Attention block's size stopped being a function of
// ds.Findings alone when the "not inspected" entry landed: it now also depends
// on the session truncated-ID set, which Core.handleEnrichmentChecked writes
// BEFORE the controller applies the intent. A prepend size recomputed inside
// applyFindingToState therefore describes a layout that was never rendered —
// it already includes the not-inspected entry while ds.Findings is still the
// pre-enrichment set — so the "was the cursor inside the old block?" test
// wrongly says yes and resets the cursor to the Attention header.
//
// Repro (from the round-2 probe): open a detail on a clean row, move the
// cursor onto a content field, then deliver ONE EnrichmentChecked carrying
// both a finding and a TruncatedIDs entry for that row. The cursor must still
// point at the same logical field.
func TestApplyDetailFinding_CursorStaysOnSameFieldWhenNotInspectedMarkArrivesToo(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	row := resource.Resource{
		ID: "i-0aaa111111111111a", Name: "web-server", Type: "ec2",
		Fields: map[string]string{
			"instance_id": "i-0aaa111111111111a",
			"name":        "web-server",
			"state":       "running",
		},
	}
	c.ApplyResourcesLoaded("ec2", []resource.Resource{row}, nil, false)
	core.ObserveRows("ec2", []resource.Resource{row}, nil, session.OriginFetch, false)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: row.ID},
	}})
	c.EnsureDetailState(row, "ec2")
	c.SetDetailViewportHeight(40)
	c.SetDetailViewportWidth(80)

	for range 3 {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	before := c.Snapshot().Body.Detail
	preCursor := before.FieldCursor
	preRow := fieldRowAt(t, before, preCursor)
	if preRow.Key == "" || preRow.IsSection {
		t.Fatalf("precondition: cursor should sit on a real content field, got %+v", preRow)
	}

	// ONE event, exactly as the sweep delivers it: the finding and the
	// truncation mark for the same row.
	intents, _ := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: "ec2",
		TruncatedIDs: map[string]bool{row.ID: true},
		Findings: map[string][]domain.Finding{row.ID: {{
			Code:     "ec2.impaired",
			Phrase:   "system check failed",
			Detail:   "The instance failed its system status check.",
			Severity: domain.SevBroken,
			Source:   "wave2:ec2",
		}}},
	})
	c.ApplyIntents(intents)

	after := c.Snapshot().Body.Detail
	postCursor := after.FieldCursor
	postRow := fieldRowAt(t, after, postCursor)

	if postRow.Key != preRow.Key || postRow.Path != preRow.Path {
		t.Errorf(
			"cursor did not stay on the same logical field when the not-inspected mark arrived with the finding:\n"+
				"  before enrichment: FieldCursor=%d Key=%q Path=%q\n"+
				"  after  enrichment: FieldCursor=%d Key=%q Path=%q\n"+
				"the old prepend size must be the one the LAST BUILD produced, not one "+
				"recomputed from a session set that has already moved.",
			preCursor, preRow.Key, preRow.Path,
			postCursor, postRow.Key, postRow.Path,
		)
	}
}

// wave1TwoWarnings is a row whose Attention block already has two entries
// before any enrichment runs, so a Wave-2 result rebuilds a block the cursor
// is sitting inside rather than creating one.
func wave1TwoWarnings(id, secondPhrase string) resource.Resource {
	return resource.Resource{
		ID: id, Name: "batch-runner", Type: "ec2",
		Fields: map[string]string{"instance_id": id, "name": "batch-runner", "state": "running"},
		Findings: []domain.Finding{
			{
				Code:     "ec2.long-stopped",
				Phrase:   "instance stopped 42d ago",
				Detail:   "Instance stopped more than 30 days ago — review whether it is still needed.",
				Severity: domain.SevWarn,
				Source:   "wave1",
			},
			{
				Code:     "ec2.open-ssh",
				Phrase:   secondPhrase,
				Detail:   "A security group attached to this instance allows SSH from the whole internet.",
				Severity: domain.SevWarn,
				Source:   "wave1",
			},
		},
	}
}

// openDetailWithSweptRow opens a detail on row through the same seams the
// sweep and the navigation use, at a real viewport size — the Wave-2 result
// below is delivered as the runtime event, so every input the relocation
// reads has to be the one the running app produces.
func openDetailWithSweptRow(t *testing.T, row resource.Resource) (*app.Controller, *runtime.Core) {
	t.Helper()
	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{row}, nil, false)
	core.ObserveRows("ec2", []resource.Resource{row}, nil, session.OriginFetch, false)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: "ec2", ResourceID: row.ID},
	}})
	c.EnsureDetailState(row, "ec2")
	c.SetDetailViewportHeight(40)
	c.SetDetailViewportWidth(80)
	return c, core
}

// seekAttentionEntry walks the cursor down until it lands on the Attention
// entry whose line reads phrase, and returns the index it stopped at.
func seekAttentionEntry(t *testing.T, c *app.Controller, phrase string) int {
	t.Helper()
	c.Apply(app.Action{Kind: app.ActionMoveTop})
	for step := 0; step < 40; step++ {
		body := c.Snapshot().Body.Detail
		if body == nil {
			t.Fatal("precondition: Body.Detail is nil")
		}
		if row := fieldRowAt(t, body, body.FieldCursor); row.Path == "Attention" && row.Key == phrase {
			return body.FieldCursor
		}
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	t.Fatalf("precondition: never reached the Attention entry %q with the cursor", phrase)
	return -1
}

// deliverEnrichment applies one Wave-2 sweep result for row through the
// runtime event path — the path a real enrichment takes, and the one the
// single-finding entry point cannot stand in for: that entry point strips the
// previous findings, so it can never rebuild a block with more than one
// Wave-2 entry in it.
func deliverEnrichment(t *testing.T, c *app.Controller, core *runtime.Core, id string, truncated bool, findings []domain.Finding) {
	t.Helper()
	ev := messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings:     map[string][]domain.Finding{id: findings},
	}
	if truncated {
		// The sweep delivers the truncation mark with the findings, and the
		// runtime writes the session set before the controller applies the
		// intent — so by then no rebuild can reproduce the block the cursor
		// was indexed against.
		ev.TruncatedIDs = map[string]bool{id: true}
	}
	intents, _ := core.HandleEvent(ev)
	c.ApplyIntents(intents)
}

// TestApplyDetailFinding_CursorFollowsItsAttentionEntry: a cursor INSIDE the
// Attention block used to be sent back to the section header whenever the
// block changed, so an operator reading the second finding lost their place
// the moment an enrichment result arrived. The entry the cursor is on is the
// thing to keep, and it is still there — it has only moved down to make room
// for a more severe one.
func TestApplyDetailFinding_CursorFollowsItsAttentionEntry(t *testing.T) {
	const secondPhrase = "public ingress on port 22 from 0.0.0.0/0"
	row := wave1TwoWarnings("i-0bbb222222222222b", secondPhrase)
	c, core := openDetailWithSweptRow(t, row)

	preCursor := seekAttentionEntry(t, c, secondPhrase)

	// A more severe finding lands and sorts above both warnings, pushing the
	// entry the cursor is on further down the block.
	deliverEnrichment(t, c, core, row.ID, true, []domain.Finding{{
		Code:     "ec2.instance-status-impaired",
		Phrase:   "impaired: system checks failing",
		Detail:   "The instance failed its system status check.",
		Severity: domain.SevBroken,
		Source:   "wave2:ec2",
	}})

	after := c.Snapshot().Body.Detail
	postRow := fieldRowAt(t, after, after.FieldCursor)
	if postRow.Path != "Attention" || postRow.Key != secondPhrase {
		t.Errorf(
			"the cursor left the entry the operator was reading when a new finding arrived above it:\n"+
				"  before: FieldCursor=%d Key=%q\n"+
				"  after:  FieldCursor=%d Key=%q Path=%q\n"+
				"want the cursor still on %q.",
			preCursor, secondPhrase,
			after.FieldCursor, postRow.Key, postRow.Path, secondPhrase,
		)
	}
}

// TestApplyDetailFinding_CursorFollowsItsEntryWhenTheBlockOnlyReorders is the
// same rule at the size that hides it: a Wave-2 result replaced by a more
// severe one of the same shape leaves the block exactly as long as it was and
// sorts to the top, so every entry below it moves down one row while the block
// size says nothing changed. A cursor kept by index reads a different finding
// than the one the operator was on.
func TestApplyDetailFinding_CursorFollowsItsEntryWhenTheBlockOnlyReorders(t *testing.T) {
	const secondPhrase = "public ingress on port 22 from 0.0.0.0/0"
	row := wave1TwoWarnings("i-0ccc333333333333c", secondPhrase)
	c, core := openDetailWithSweptRow(t, row)

	// A Wave-2 warning of the same shape as the broken finding that replaces
	// it below: one phrase line and one detail line, sorted after the warnings.
	deliverEnrichment(t, c, core, row.ID, false, []domain.Finding{{
		Code: "ec2.slow-disk", Phrase: "volume queue depth is high", Detail: "One sentence.",
		Severity: domain.SevWarn, Source: "wave2:ec2",
	}})
	preCursor := seekAttentionEntry(t, c, secondPhrase)
	beforeLen := len(c.Snapshot().Body.Detail.Fields)

	deliverEnrichment(t, c, core, row.ID, false, []domain.Finding{{
		Code: "ec2.instance-status-impaired", Phrase: "impaired: system checks failing", Detail: "One sentence.",
		Severity: domain.SevBroken, Source: "wave2:ec2",
	}})

	after := c.Snapshot().Body.Detail
	if len(after.Fields) != beforeLen {
		t.Fatalf("fixture no longer holds the block length constant (%d then %d) — the reorder-only case is what this pins", beforeLen, len(after.Fields))
	}
	postRow := fieldRowAt(t, after, after.FieldCursor)
	if postRow.Path != "Attention" || postRow.Key != secondPhrase {
		t.Errorf(
			"the block reordered without changing length and the cursor kept its index instead of its entry:\n"+
				"  before: FieldCursor=%d Key=%q\n"+
				"  after:  FieldCursor=%d Key=%q Path=%q\n"+
				"want the cursor still on %q.",
			preCursor, secondPhrase,
			after.FieldCursor, postRow.Key, postRow.Path, secondPhrase,
		)
	}
}

// TestApplyDetailFinding_CursorOnTheAttentionHeaderStaysThere is the boundary
// of the same rule. The header is inside the block but is not an entry: its
// line counts the findings, so it is a different string every time the count
// moves. A cursor there must stay on the header, never be carried into the
// block by the content-field arithmetic.
func TestApplyDetailFinding_CursorOnTheAttentionHeaderStaysThere(t *testing.T) {
	row := wave1TwoWarnings("i-0ddd444444444444d", "public ingress on port 22 from 0.0.0.0/0")
	c, core := openDetailWithSweptRow(t, row)

	c.Apply(app.Action{Kind: app.ActionMoveTop})
	before := c.Snapshot().Body.Detail
	beforeRow := fieldRowAt(t, before, before.FieldCursor)
	if !beforeRow.IsSection || beforeRow.Path != "Attention" {
		t.Fatalf("precondition: the top row is not the Attention header, got Key=%q Path=%q IsSection=%v", beforeRow.Key, beforeRow.Path, beforeRow.IsSection)
	}

	deliverEnrichment(t, c, core, row.ID, false, []domain.Finding{{
		Code: "ec2.instance-status-impaired", Phrase: "impaired: system checks failing",
		Detail: "The instance failed its system status check.", Severity: domain.SevBroken, Source: "wave2:ec2",
	}})

	after := c.Snapshot().Body.Detail
	afterRow := fieldRowAt(t, after, after.FieldCursor)
	if !afterRow.IsSection || afterRow.Path != "Attention" {
		t.Errorf(
			"the cursor was carried off the Attention header into the block:\n"+
				"  before: FieldCursor=%d Key=%q\n"+
				"  after:  FieldCursor=%d Key=%q Path=%q IsSection=%v\n"+
				"want the cursor still on the header.",
			before.FieldCursor, beforeRow.Key,
			after.FieldCursor, afterRow.Key, afterRow.Path, afterRow.IsSection,
		)
	}
}
