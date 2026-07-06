// tui_related_dim_parity_test.go — pins the user-visible defect where an
// approximate-zero related row (checker resolved Count=0, Approximate=true —
// e.g. resource.ApproximateZero() results like trail/glue/backup on an S3
// bucket) renders BRIGHT "(0)" in the live TUI detail RELATED panel while an
// exact-zero row renders dim, even though both are dead-end pivots per
// resource.IsRelatedActionable (any resolved zero, approximate or not, is
// never actionable — related.go:249-266).
//
// Root cause: renderDetailRelatedFromBody (internal/tui/views/detail_helpers.go,
// the LIVE renderer invoked by DetailModel.RenderDetail) carries an inline
// style switch with its own `case blk.Count == 0 && blk.Approximate` branch
// that assigns styles.RowNormal (bright), diverging from the controller's
// already-correct RelatedBlock.Actionable field (set via
// isActionableDetailRow -> resource.IsRelatedActionable by
// buildDetailRelatedBlocks, internal/app/detail_body.go:435-436). The fix
// under test makes the renderer derive rowStyle strictly from blk.Actionable
// and the badge text from blk.CountDisplay, so the two renderers (TUI +
// web template, which already reads .Actionable/.CountDisplay) cannot drift.
//
// These tests build a real views.DetailModel, size it to trigger the
// side-by-side related-panel layout, and call m.RenderDetail(body) with a
// hand-built app.DetailBody — the exact harness detail_render_parity_test.go
// uses for RenderDetail-level assertions. NO_COLOR is intentionally left
// unset so the SGR-wrapped output from styles.DimText / styles.RowNormal can
// be compared byte-for-byte, which is the only way to distinguish "bright"
// from "dim" in a rendered string.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Shared setup
// ---------------------------------------------------------------------------

// relatedDimParityTypes returns representative resource types for the sweep.
// "s3" matches the dispatch's real-world case (trail/glue/backup checkers on
// a bucket resolving ApproximateZero); "ec2" is a non-S3 control to prove the
// bug (and its fix) is renderer-generic, not S3-specific.
func relatedDimParityTypes() []struct {
	shortName string
	res       resource.Resource
} {
	return []struct {
		shortName string
		res       resource.Resource
	}{
		{"s3", resource.Resource{
			ID:   "acme-prod-assets",
			Name: "acme-prod-assets",
			Fields: map[string]string{
				"name":   "acme-prod-assets",
				"region": "us-east-1",
			},
		}},
		{"ec2", resource.Resource{
			ID:   "i-0abc123def456789a",
			Name: "prod-backend-01",
			Fields: map[string]string{
				"instance_id": "i-0abc123def456789a",
				"state":       "running",
			},
		}},
	}
}

// newRelatedDimParityDetail builds a sized DetailModel for RenderDetail-level
// assertions, matching detail_render_parity_test.go's pattern (wide width
// triggers the side-by-side related-panel layout).
func newRelatedDimParityDetail(shortName string, res resource.Resource) views.DetailModel {
	k := keys.Default()
	m := views.NewDetail(res, shortName, nil, k)
	m.SetSize(160, 30)
	return m
}

// relatedDimParityBody wraps the given blocks in a DetailBody with the
// RelatedVisible gate set, matching what buildDetailBody produces once the
// panel is showing.
func relatedDimParityBody(blocks []app.RelatedBlock) app.DetailBody {
	return app.DetailBody{
		Related:        blocks,
		RelatedVisible: true,
	}
}

// relatedColSep is the styled column separator RenderDetail places between
// the left field panel and the right RELATED panel (detail_helpers.go's
// "│" rendered via styles.ColSepDim/ColSepAccent). The related-panel content
// for a given line always starts immediately after the LAST such separator
// on that line, since the left panel's own content can never contain this
// exact styled byte sequence (it is a private box-drawing glyph wrapped in a
// column-separator-specific SGR code).
const relatedColSepGlyph = "│" // "│"

// extractRelatedLine returns the RIGHT-COLUMN portion (everything after the
// last "│" column separator) of the single rendered line containing needle,
// still ANSI-styled and NOT stripped, so the row's exact style can be
// compared. Fails the test if zero or multiple lines match, or if the
// matching line has no column separator (the related panel was not shown).
func extractRelatedLine(t *testing.T, rendered, needle string) string {
	t.Helper()
	var matches []string
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, needle) {
			matches = append(matches, line)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 rendered line containing %q, got %d; full render:\n%s", needle, len(matches), rendered)
	}
	line := matches[0]
	sepIdx := strings.LastIndex(line, relatedColSepGlyph)
	if sepIdx == -1 {
		t.Fatalf("rendered line containing %q has no %q column separator — related panel not shown; line:\n%q", needle, relatedColSepGlyph, line)
	}
	// Advance past the separator glyph itself plus its trailing reset/SGR
	// bytes up to (and including) the reset sequence "\x1b[m" that always
	// terminates ColSepDim/ColSepAccent's Render call, so what remains is
	// exactly the row's own styled text with no separator bytes mixed in.
	rest := line[sepIdx+len(relatedColSepGlyph):]
	const reset = "\x1b[m"
	if resetIdx := strings.Index(rest, reset); resetIdx != -1 {
		rest = rest[resetIdx+len(reset):]
	}
	return rest
}

// ---------------------------------------------------------------------------
// Pin 1: approximate-zero renders with the SAME dim style as exact-zero.
// ---------------------------------------------------------------------------

// TestRelatedDim_ApproximateZero_SameStyleAsExactZero is RED at HEAD: the live
// renderer's `case blk.Count == 0 && blk.Approximate` branch assigns
// styles.RowNormal (bright) instead of styles.DimText, so the approximate-zero
// row's styled prefix diverges from the exact-zero row's styled prefix even
// though both carry Actionable=false.
func TestRelatedDim_ApproximateZero_SameStyleAsExactZero(t *testing.T) {
	for _, tc := range relatedDimParityTypes() {
		tc := tc
		t.Run(tc.shortName, func(t *testing.T) {
			m := newRelatedDimParityDetail(tc.shortName, tc.res)

			approxBlock := app.RelatedBlock{
				Name:         "Trail Events",
				Count:        0,
				Approximate:  true,
				TargetType:   "ct-events",
				Actionable:   resource.IsRelatedActionable(0, true, false, false, false),
				CountDisplay: resource.FormatRelatedCount(0),
			}
			exactBlock := app.RelatedBlock{
				Name:         "Backup Plans",
				Count:        0,
				Approximate:  false,
				TargetType:   "backup",
				Actionable:   resource.IsRelatedActionable(0, false, false, false, false),
				CountDisplay: resource.FormatRelatedCount(0),
			}
			if approxBlock.Actionable {
				t.Fatal("test setup: approxBlock.Actionable must be false (resolved zero is never actionable)")
			}
			if exactBlock.Actionable {
				t.Fatal("test setup: exactBlock.Actionable must be false")
			}

			body := relatedDimParityBody([]app.RelatedBlock{approxBlock, exactBlock})
			rendered := m.RenderDetail(body)

			approxLine := extractRelatedLine(t, rendered, "Trail Events")
			exactLine := extractRelatedLine(t, rendered, "Backup Plans")

			wantApproxText := "  Trail Events (0)"
			wantExactText := "  Backup Plans (0)"
			wantApproxStyled := styles.DimText.Render(wantApproxText)
			wantExactStyled := styles.DimText.Render(wantExactText)

			if approxLine != wantApproxStyled {
				t.Errorf("[%s] approximate-zero row not rendered with DimText style.\n  got:  %q\n  want: %q", tc.shortName, approxLine, wantApproxStyled)
			}
			if exactLine != wantExactStyled {
				t.Errorf("[%s] exact-zero row not rendered with DimText style.\n  got:  %q\n  want: %q", tc.shortName, exactLine, wantExactStyled)
			}
			// Same SGR prefix on both rows (the style code up to the row text)
			// pins "same style" independent of the two rows' differing names —
			// a byte-for-byte full-line comparison would always fail because
			// the row text itself legitimately differs.
			approxSGR := approxLine[:strings.Index(approxLine, "m")+1]
			exactSGR := exactLine[:strings.Index(exactLine, "m")+1]
			if approxSGR != exactSGR {
				t.Errorf("[%s] approximate-zero and exact-zero rows render with DIFFERENT styles even though both are non-actionable dead ends.\n  approx SGR: %q (full: %q)\n  exact SGR:  %q (full: %q)", tc.shortName, approxSGR, approxLine, exactSGR, exactLine)
			}

			// Guard against the historical bug reappearing under a different
			// guise: the approximate-zero row must never carry RowNormal
			// (bright) styling.
			brightApprox := styles.RowNormal.Render(wantApproxText)
			if approxLine == brightApprox {
				t.Errorf("[%s] approximate-zero row rendered BRIGHT (styles.RowNormal) — this is the exact defect under test", tc.shortName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pin 2: property-style sweep — bright IFF resource.IsRelatedActionable.
// ---------------------------------------------------------------------------

// relatedDimParityCase is one block-state combination in the sweep.
type relatedDimParityCase struct {
	name           string
	block          app.RelatedBlock
	wantActionable bool
}

// relatedDimParitySweepCases builds one RelatedBlock per state combination
// named in the dispatch: loading / err / count -1 with+without filter /
// 0 exact / 0 approximate / N>0. Actionable and CountDisplay are computed via
// the same shared helpers the controller uses, so this table is itself a
// pin on resource.IsRelatedActionable / resource.FormatRelatedCount wiring,
// not just the renderer.
func relatedDimParitySweepCases() []relatedDimParityCase {
	mk := func(name string, count int, approximate, hasFilter, loading, hasErr bool) relatedDimParityCase {
		var filter map[string]string
		if hasFilter {
			filter = map[string]string{"instance-id": "i-0abc123def456789a"}
		}
		actionable := resource.IsRelatedActionable(count, approximate, hasFilter, loading, hasErr)
		blk := app.RelatedBlock{
			Name:        name,
			Count:       count,
			Approximate: approximate,
			FetchFilter: filter,
			Loading:     loading,
			Err:         hasErr,
			TargetType:  "sweep-" + name,
			Actionable:  actionable,
		}
		if !loading && !hasErr {
			blk.CountDisplay = resource.FormatRelatedCount(count)
		}
		return relatedDimParityCase{name: name, block: blk, wantActionable: actionable}
	}
	return []relatedDimParityCase{
		mk("LoadingRow", -1, false, false, true, false),
		mk("ErrRow", -1, false, false, false, true),
		mk("UnknownNoFilter", -1, false, false, false, false),
		mk("UnknownWithFilter", -1, false, true, false, false),
		mk("ExactZero", 0, false, false, false, false),
		mk("ApproxZero", 0, true, false, false, false),
		mk("PositiveCount", 7, false, false, false, false),
		mk("PositiveApprox", 7, true, false, false, false),
	}
}

// expectedRelatedRowText mirrors the renderer's row-text construction rules
// (prefix + name, "(N)" suffix for resolved counts, em-dash for errors, no
// suffix for loading/unknown) so this test does not merely re-run the
// production switch — it independently derives the text the contract implies
// and lets the assertion below catch any divergence in rowStyle.
func expectedRelatedRowText(c relatedDimParityCase) string {
	blk := c.block
	switch {
	case blk.Loading:
		return "  " + blk.Name
	case blk.Err:
		return "  " + blk.Name + "  —"
	case blk.Count == -1:
		return "  " + blk.Name
	default:
		display := resource.FormatRelatedCount(blk.Count)
		if display == "" {
			return "  " + blk.Name
		}
		return "  " + blk.Name + " " + display
	}
}

// TestRelatedDim_PropertySweep_BrightIffActionable drives one detail frame
// containing every block-state combination and asserts, per row, that the
// rendered style is styles.RowNormal (bright) exactly when
// resource.IsRelatedActionable(...) is true for that block, and
// styles.DimText otherwise. This is RED at HEAD for the ApproxZero and
// PositiveApprox cases only if the renderer's inline switch diverges from
// blk.Actionable; it is the general contract the coder's fix must satisfy
// for every state, not just the approximate-zero one named in the report.
func TestRelatedDim_PropertySweep_BrightIffActionable(t *testing.T) {
	for _, tc := range relatedDimParityTypes() {
		tc := tc
		t.Run(tc.shortName, func(t *testing.T) {
			cases := relatedDimParitySweepCases()
			blocks := make([]app.RelatedBlock, len(cases))
			for i, c := range cases {
				blocks[i] = c.block
			}

			m := newRelatedDimParityDetail(tc.shortName, tc.res)
			body := relatedDimParityBody(blocks)
			rendered := m.RenderDetail(body)

			for _, c := range cases {
				rowText := expectedRelatedRowText(c)
				line := extractRelatedLine(t, rendered, c.block.Name)

				wantStyled := styles.DimText.Render(rowText)
				if c.wantActionable {
					wantStyled = styles.RowNormal.Render(rowText)
				}

				if line != wantStyled {
					styleName := "DimText"
					if c.wantActionable {
						styleName = "RowNormal"
					}
					t.Errorf("[%s/%s] IsRelatedActionable=%v but rendered line does not match styles.%s.\n  got:  %q\n  want: %q",
						tc.shortName, c.name, c.wantActionable, styleName, line, wantStyled)
				}
			}
		})
	}
}

// TestRelatedDim_PropertySweep_TableDrivenSanity is a plain (no rendering)
// sanity check that the sweep table itself encodes the documented contract
// from related.go:249-266 correctly, so a future edit to
// relatedDimParitySweepCases cannot silently invert an expectation and make
// the rendering test above vacuously pass.
func TestRelatedDim_PropertySweep_TableDrivenSanity(t *testing.T) {
	want := map[string]bool{
		"LoadingRow":        false,
		"ErrRow":            false,
		"UnknownNoFilter":   false,
		"UnknownWithFilter": true,
		"ExactZero":         false,
		"ApproxZero":        false,
		"PositiveCount":     true,
		"PositiveApprox":    true,
	}
	for _, c := range relatedDimParitySweepCases() {
		w, ok := want[c.name]
		if !ok {
			t.Fatalf("sweep case %q has no expectation registered in this sanity table", c.name)
		}
		if c.wantActionable != w {
			t.Errorf("case %q: wantActionable=%v, want %v (contract mismatch — check resource.IsRelatedActionable wiring)", c.name, c.wantActionable, w)
		}
	}
}

// ---------------------------------------------------------------------------
// Pin 3: cursor parity — approximate-zero row is still skipped.
// ---------------------------------------------------------------------------
//
// This extends the existing app_related_cursor_skip_test.go harness
// (newRelatedSkipController / relatedCursorAndActionable, same package) with
// one additional case: an approximate-zero row must be skipped by cursor
// movement exactly like an exact-zero row, so a future fix to the renderer's
// brightness cannot silently diverge from the cursor's skip predicate (both
// already delegate to isActionableDetailRow / resource.IsRelatedActionable,
// so this is a regression guard tying the three surfaces — render style,
// cursor skip, Enter/click gating — to the single shared predicate).

// relatedRowApprox builds a DetailRelatedRow with an explicit Approximate
// flag, extending relatedRow (which always passes approximate=false) for the
// one case this suite adds.
func relatedRowApprox(targetType string, count int, approximate bool) app.DetailRelatedRow {
	return app.DetailRelatedRow{
		TargetType:  targetType,
		DisplayName: targetType,
		Count:       count,
		Approximate: approximate,
	}
}

// TestRelatedCursor_MoveDown_SkipsApproximateZeroRow verifies that moving
// down from a non-dim row past an APPROXIMATE-zero row (Count=0,
// Approximate=true — an ApproximateZero() result) lands on the next non-dim
// row, not on the approximate-zero one. This is the cursor-side half of the
// approximate-zero contract: the dispatch's defect was purely visual
// (renderer brightness), and this test confirms the cursor-skip and
// Actionable-flag paths were never affected — a real navigation must never
// stop on an approximate-zero row any more than an exact-zero one.
func TestRelatedCursor_MoveDown_SkipsApproximateZeroRow(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 3),                    // index 0: non-dim, cursor starts here
		relatedRowApprox("ct-events", 0, true), // index 1: approximate-zero, dimmed
		relatedRow("eni", 2),                   // index 2: non-dim
	}
	c := newRelatedSkipController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionMoveDown})

	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 2 {
		t.Errorf("RelatedCursor after MoveDown = %d, want 2 (should skip approximate-zero index 1)", cursor)
	}
	if !actionable {
		t.Errorf("row at RelatedCursor=%d is dimmed (Actionable=false), want a non-dim landing row", cursor)
	}
	// Sanity: the skipped row really is the approximate-zero one, and it is
	// indeed non-actionable per the shared predicate — otherwise this test
	// would pass for the wrong reason if the fixture were edited later.
	if got := resource.IsRelatedActionable(0, true, false, false, false); got {
		t.Fatalf("test setup: resource.IsRelatedActionable(0, approximate=true, ...) = %v, want false — the approximate-zero fixture in this test is no longer non-actionable per the shared contract", got)
	}
}
