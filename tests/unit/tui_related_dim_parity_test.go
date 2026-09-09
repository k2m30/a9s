// tui_related_dim_parity_test.go — pins the user-visible defect where an
// truncated-zero related row (checker resolved Count=0, Truncated=true —
// e.g. relatedResultTrunc() results like trail/glue/backup on an S3
// bucket) renders BRIGHT "(0)" in the live TUI detail RELATED panel while an
// exact-zero row renders dim, even though both are dead-end pivots per
// resource.IsRelatedActionable (any RelatedResolved zero, truncated or not,
// is never actionable — related.go:290-306).
//
// Root cause: renderDetailRelatedFromBody (internal/tui/views/detail_helpers.go,
// the LIVE renderer invoked by DetailModel.RenderDetail) carries an inline
// style switch with its own `case blk.Count == 0 && blk.Truncated` branch
// that assigns styles.RowNormal (bright), diverging from the controller's
// already-correct RelatedBlock.Actionable field (set via
// isActionableDetailRow -> resource.IsRelatedActionable by
// buildDetailRelatedBlocks, core/app/detail_body.go:435-436). The fix
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

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Shared setup
// ---------------------------------------------------------------------------

// relatedDimParityTypes returns representative resource types for the sweep.
// "s3" is the real-world case (trail/glue/backup checkers on a bucket
// resolving TruncatedResult); "ec2" is a non-S3 control to prove the
// contract is renderer-generic, not S3-specific.
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

// newRelatedDimParityDetail builds a DetailModel via the live
// NewTransientDetail seam for RenderDetail-level assertions, sized wide
// enough to trigger the side-by-side related-panel layout. shortName/res are
// unused by this constructor (RenderDetail reads everything from the body
// passed to it) but are kept in the signature so callers stay table-driven
// per relatedDimParityTypes().
func newRelatedDimParityDetail(_ string, _ resource.Resource) views.DetailModel {
	vp := viewport.New(viewport.WithWidth(160), viewport.WithHeight(30))
	return views.NewTransientDetail(160, 30, vp)
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
// Pin 1: truncated-zero renders with the SAME dim style as exact-zero.
// ---------------------------------------------------------------------------

// TestRelatedDim_TruncatedResult_BrightAndActionable pins the correct contract:
// an truncated lower bound ("0+", from a truncated target scan where more may
// exist on later pages) renders BRIGHT and actionable with a "(0+)" badge — the
// user can drill in — while only a PROVEN exact zero renders dim "(0)" as a
// dead end. The two must therefore render DIFFERENTLY.
func TestRelatedDim_TruncatedResult_BrightAndActionable(t *testing.T) {
	for _, tc := range relatedDimParityTypes() {
		tc := tc
		t.Run(tc.shortName, func(t *testing.T) {
			m := newRelatedDimParityDetail(tc.shortName, tc.res)

			truncatedBlock := app.RelatedBlock{
				Name:         "Trail Events",
				State:        domain.RelatedResolved,
				Count:        0,
				Truncated:    true,
				TargetType:   "ct-events",
				Actionable:   resource.IsRelatedActionable(domain.RelatedResolved, 0, true),
				CountDisplay: resource.FormatRelatedCount(domain.RelatedResolved, 0, true),
			}
			exactBlock := app.RelatedBlock{
				Name:         "Backup Plans",
				State:        domain.RelatedResolved,
				Count:        0,
				Truncated:    false,
				TargetType:   "backup",
				Actionable:   resource.IsRelatedActionable(domain.RelatedResolved, 0, false),
				CountDisplay: resource.FormatRelatedCount(domain.RelatedResolved, 0, false),
			}
			if !truncatedBlock.Actionable {
				t.Fatal("test setup: truncatedBlock.Actionable must be true (0+ lower bound is drillable)")
			}
			if exactBlock.Actionable {
				t.Fatal("test setup: exactBlock.Actionable must be false (proven zero is a dead end)")
			}

			body := relatedDimParityBody([]app.RelatedBlock{truncatedBlock, exactBlock})
			rendered := m.RenderDetail(body)

			truncatedLine := extractRelatedLine(t, rendered, "Trail Events")
			exactLine := extractRelatedLine(t, rendered, "Backup Plans")

			// Truncated row: BRIGHT with a "(0+)" badge.
			wantTruncatedStyled := styles.RowNormal.Render("  Trail Events (0+)")
			// Exact zero: DIM with a plain "(0)" badge.
			wantExactStyled := styles.DimText.Render("  Backup Plans (0)")

			if truncatedLine != wantTruncatedStyled {
				t.Errorf("[%s] truncated-zero row not rendered BRIGHT with \"(0+)\" badge.\n  got:  %q\n  want: %q", tc.shortName, truncatedLine, wantTruncatedStyled)
			}
			if exactLine != wantExactStyled {
				t.Errorf("[%s] exact-zero row not rendered DIM with \"(0)\" badge.\n  got:  %q\n  want: %q", tc.shortName, exactLine, wantExactStyled)
			}
			if truncatedLine == exactLine {
				t.Errorf("[%s] truncated-zero and exact-zero rows render identically — a drillable lower bound must be visually distinct from a proven dead-end zero", tc.shortName)
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
// named in the dispatch: loading / err / resolved-unknown with+without filter
// (deferred) / 0 exact / 0 truncated / N>0. Actionable and CountDisplay are
// computed via the same shared helpers the controller uses, so this table is
// itself a pin on resource.IsRelatedActionable / resource.FormatRelatedCount
// wiring, not just the renderer.
//
// mk's (count, hasFilter, loading, hasErr) parameters preserve each sweep
// case's pre-task-#58 identity; state is derived from them via the migration
// rule 2/3 mapping (loading->RelatedLoading, hasErr->RelatedError,
// count<0&&hasFilter->RelatedDeferred, count<0->RelatedUnknown,
// else->RelatedResolved), matching what the real checker constructors
// (LoadingRelated/ErrorRelated/DeferredRelated/UnknownRelated) now produce.
// Count is normalized to 0 for every non-Resolved state, mirroring those
// constructors.
func relatedDimParitySweepCases() []relatedDimParityCase {
	mk := func(name string, count int, truncated, hasFilter, loading, hasErr bool) relatedDimParityCase {
		var filter map[string]string
		if hasFilter {
			filter = map[string]string{"instance-id": "i-0abc123def456789a"}
		}
		state := domain.RelatedResolved
		switch {
		case loading:
			state = domain.RelatedLoading
		case hasErr:
			state = domain.RelatedError
		case count < 0 && hasFilter:
			state = domain.RelatedDeferred
		case count < 0:
			state = domain.RelatedUnknown
		}
		if state != domain.RelatedResolved {
			count = 0
		}
		actionable := resource.IsRelatedActionable(state, count, truncated)
		blk := app.RelatedBlock{
			Name:        name,
			State:       state,
			Count:       count,
			Truncated:   truncated,
			FetchFilter: filter,
			Loading:     loading,
			Err:         hasErr,
			TargetType:  "sweep-" + name,
			Actionable:  actionable,
		}
		if !loading && !hasErr {
			blk.CountDisplay = resource.FormatRelatedCount(state, count, truncated)
		}
		return relatedDimParityCase{name: name, block: blk, wantActionable: actionable}
	}
	return []relatedDimParityCase{
		mk("LoadingRow", -1, false, false, true, false),
		mk("ErrRow", -1, false, false, false, true),
		mk("UnknownNoFilter", -1, false, false, false, false),
		mk("UnknownWithFilter", -1, false, true, false, false),
		mk("ExactZero", 0, false, false, false, false),
		mk("TruncatedZero", 0, true, false, false, false),
		mk("PositiveCount", 7, false, false, false, false),
		mk("PositiveTruncated", 7, true, false, false, false),
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
	default:
		display := resource.FormatRelatedCount(blk.State, blk.Count, blk.Truncated)
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
// styles.DimText otherwise — the general contract for every state, not just
// the truncated-zero one, so a renderer switch that diverges from
// blk.Actionable fails here.
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
		"UnknownNoFilter":   true,
		"UnknownWithFilter": true,
		"ExactZero":         false,
		"TruncatedZero":     true,
		"PositiveCount":     true,
		"PositiveTruncated": true,
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
// Pin 3: cursor parity — truncated-zero row is still skipped.
// ---------------------------------------------------------------------------
//
// This extends the existing app_related_cursor_skip_test.go harness
// (newRelatedSkipController / relatedCursorAndActionable, same package) with
// one additional case: an truncated-zero row must be skipped by cursor
// movement exactly like an exact-zero row, so a future fix to the renderer's
// brightness cannot silently diverge from the cursor's skip predicate (both
// already delegate to isActionableDetailRow / resource.IsRelatedActionable,
// so this is a regression guard tying the three surfaces — render style,
// cursor skip, Enter/click gating — to the single shared predicate).

// relatedRowTruncated builds a DetailRelatedRow with an explicit Truncated
// flag, extending relatedRow (which always passes truncated=false) for the
// one case this suite adds.
func relatedRowTruncated(targetType string, count int, truncated bool) app.DetailRelatedRow {
	return app.DetailRelatedRow{
		TargetType:  targetType,
		DisplayName: targetType,
		Count:       count,
		Truncated:   truncated,
	}
}

// TestRelatedCursor_MoveDown_LandsOnTruncatedResultRow verifies that moving
// down from an actionable row LANDS ON an truncated-zero row (Count=0,
// Truncated=true — an TruncatedResult() result), because a "0+" lower bound
// is drillable (more may exist on later pages). The cursor skip predicate,
// render brightness, and Enter/click gating all delegate to the single shared
// resource.IsRelatedActionable, so an truncated-zero row is a valid landing
// row exactly like any other actionable row.
func TestRelatedCursor_MoveDown_LandsOnTruncatedResultRow(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 3),                       // index 0: actionable, cursor starts here
		relatedRowTruncated("ct-events", 0, true), // index 1: truncated-zero, actionable (0+ drillable)
		relatedRow("eni", 2),                      // index 2: actionable
	}
	c := newRelatedSkipController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionMoveDown})

	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 1 {
		t.Errorf("RelatedCursor after MoveDown = %d, want 1 (truncated-zero row is actionable and must NOT be skipped)", cursor)
	}
	if !actionable {
		t.Errorf("row at RelatedCursor=%d is dimmed (Actionable=false), want the actionable truncated-zero landing row", cursor)
	}
	// Sanity: an truncated-zero row is actionable per the shared predicate.
	if got := resource.IsRelatedActionable(domain.RelatedResolved, 0, true); !got {
		t.Fatalf("test setup: resource.IsRelatedActionable(RelatedResolved, 0, truncated=true) = %v, want true — a 0+ lower bound must be drillable", got)
	}
}
