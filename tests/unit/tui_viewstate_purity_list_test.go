// tui_viewstate_purity_list_test.go — TDD red-light tests pinning the TUI
// list renderer as a PURE consumer of app.ListBody.
//
// Contract under test: internal/tui/views/resourcelist.go's RenderList(body)
// must derive every presentation decision from the ListBody fields it is
// given — it must NOT re-derive them from body.EnrichmentFindings or any
// other side channel. Today RenderList re-derives three things instead of
// consuming the pre-resolved body fields:
//
//  1. renderListDataRow: no "!"/"~" glyph may be re-derived from
//     body.EnrichmentFindings[row.ResourceID] + row.Color == "healthy". A
//     row's colour already carries its worst finding, and the row-decorator
//     plumbing that once fed this was deleted (tui5 row 5).
//  2. renderListDataRow (~line 874): the S4 status-cell override re-applies
//     findings[row.ResourceID].Phrase at statusColIdx, even though
//     buildListBody (list_body.go ~line 124) already bakes the phrase into
//     row.Cells[statusCol].
//  3. RenderList: statusColIdx is re-resolved from cols/fullCols/typeDef
//     instead of consuming body.StatusCol.
//
// Each test below constructs a SYNTHETIC ListBody whose pre-resolved Cells
// deliberately DISAGREE with what re-derivation from EnrichmentFindings would
// produce. A pure renderer follows the body fields;
// today's renderer follows the re-derivation, so these tests fail red.
//
// Harness: mirrors tests/unit/resourcelist_render_parity_test.go — construct
// a views.ResourceListModel via views.NewResourceList(td, nil, k), SetSize,
// then call m.RenderList(body) directly with a hand-built app.ListBody. No
// controller involved: the body is constructed by hand so its fields can be
// set to values that intentionally disagree with the findings map.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// purityTypeDef builds a minimal ResourceTypeDef: column 0 = "Name" (marker
// column via Step 2 key=="name"), column 1 = "Status" (status/lifecycle
// column). Mirrors minimalTypeDef in phase03_view_reads_test.go.
func purityTypeDef(shortName string) resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      shortName,
		ShortName: shortName,
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 24},
			{Key: "status", Title: "Status", Width: 20},
		},
		Color: func(r resource.Resource) resource.Color {
			return resource.ColorHealthy
		},
	}
}

// purityColumns returns the two-column ColumnDef list matching purityTypeDef,
// for direct use in a synthetic app.ListBody.
func purityColumns() []app.ColumnDef {
	return []app.ColumnDef{
		{Key: "name", Title: "Name", Width: 24},
		{Key: "status", Title: "Status", Width: 20},
	}
}

// newPurityListModel builds a sized ResourceListModel for the given typeDef,
// ready to receive RenderList(body) calls.
func newPurityListModel(td resource.ResourceTypeDef) views.ResourceListModel {
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 30)
	return m
}

// ---------------------------------------------------------------------------
// Case 1 — deleted with the row-decorator plumbing (tui5 row 5). It pinned
// that the renderer prefers row.Decorator over a re-derivation from
// EnrichmentFindings; there is no row decorator any more, so the only half of
// that contract left to hold is Case 2's: no glyph is prepended to any cell,
// whatever the findings map says. Do not restore a Decorator-carrying pin.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Case 2 — the findings map DOES carry an issue finding for the row, and the
// row still renders with no glyph on any cell. A row's colour is the worst
// finding over both waves, so there is nothing for a marker to add; a renderer
// that re-derived one from EnrichmentFindings would be inventing a surface.
// ---------------------------------------------------------------------------

func TestViewStatePurity_List_GlyphFollowsDecoratorNotFindings_Absent(t *testing.T) {
	ensureNoColor(t)
	styles.ReinitForTest()
	t.Cleanup(styles.ReinitForTest)

	td := purityTypeDef("purity-glyph-absent")
	m := newPurityListModel(td)

	row := app.ListRow{
		Cells:      []string{"demo-instance-2", "running"},
		ResourceID: "res-2",
		Color:      "healthy",
	}
	body := app.ListBody{
		Columns:   purityColumns(),
		Rows:      []app.ListRow{row},
		Selected:  0,
		MarkerCol: 0,
		StatusCol: 1, // "Status" column — matches purityColumns()[1]
		// Deliberately carries an issue-severity finding for res-2, which a
		// re-deriving renderer would surface as a "! " prefix.
		EnrichmentFindings: map[string][]domain.Finding{
			"res-2": {{
				Code:     "PURITY-TEST",
				Phrase:   "should not surface as glyph",
				Severity: domain.SevBroken,
			}},
		},
	}

	out := m.RenderList(body)
	if strings.Contains(out, "! demo-instance-2") || strings.Contains(out, "~ demo-instance-2") {
		t.Errorf("expected NO glyph prefix on any cell — EnrichmentFindings carries an "+
			"issue finding for res-2 that a re-deriving renderer would surface as a "+
			"glyph — got:\n%s", out)
	}
	if !strings.Contains(out, "demo-instance-2") {
		t.Fatalf("expected row identity cell to render at all — got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Case 3 — Cells already carry the S4-baked phrase; findings map carries a
// DIFFERENT phrase for the same row. A pure renderer must show the Cells
// value verbatim. Today's renderer overrides the status cell with the
// findings phrase whenever i==statusColIdx and an issue finding exists.
// ---------------------------------------------------------------------------

func TestViewStatePurity_List_StatusCellFollowsCellsNotFindingsPhrase(t *testing.T) {
	ensureNoColor(t)
	styles.ReinitForTest()
	t.Cleanup(styles.ReinitForTest)

	td := purityTypeDef("purity-status-cell")
	m := newPurityListModel(td)

	const bakedPhrase = "baked: disk pressure high"
	const findingsPhrase = "DECOY: should never render"

	row := app.ListRow{
		// Cells[1] is the "status" column — already carries the baked S4 phrase.
		Cells:      []string{"demo-instance-3", bakedPhrase},
		ResourceID: "res-3",
		Color:      "healthy",
	}
	body := app.ListBody{
		Columns:   purityColumns(),
		Rows:      []app.ListRow{row},
		Selected:  0,
		MarkerCol: 0,
		StatusCol: 1, // "Status" column — matches purityColumns()[1]
		EnrichmentFindings: map[string][]domain.Finding{
			"res-3": {{
				Code:     "PURITY-TEST",
				Phrase:   findingsPhrase,
				Severity: domain.SevBroken,
			}},
		},
	}

	out := m.RenderList(body)
	if strings.Contains(out, findingsPhrase) {
		t.Errorf("status cell rendered the findings-map Phrase (%q) instead of the "+
			"pre-baked body.Rows[i].Cells value (%q) — RenderList must consume Cells "+
			"verbatim, not re-apply the S4 override from EnrichmentFindings — got:\n%s",
			findingsPhrase, bakedPhrase, out)
	}
	if !strings.Contains(out, bakedPhrase) {
		t.Errorf("expected the pre-baked status cell value %q to render verbatim — got:\n%s",
			bakedPhrase, out)
	}
}

// ---------------------------------------------------------------------------
// Case 4 — deleted with the row-decorator plumbing (tui5 row 5). It pinned
// that the glyph landed on body.MarkerCol's cell rather than on a re-derived
// identity column; no cell carries a glyph any more. body.MarkerCol survives
// for the widen pass, and Case 2 covers the "no glyph, whatever the findings
// say" half. Do not restore a glyph-placement pin.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Compile-red marker — body.StatusCol does not exist yet.
//
// Per task note: ViewState may need a StatusCol field (sibling of MarkerCol)
// so RenderList can consume the status-column index instead of re-resolving
// it via the lifecycleColumnKey/title-fallback cascade at resourcelist.go
// ~line 594-617. This test references body.StatusCol, which does not compile
// against the current app.ListBody struct (core/app/viewstate.go ~line
// 137-156 has no StatusCol field). This is INTENTIONAL compile-red: the
// architect must add the field before this file compiles.
//
// Uncomment once app.ListBody.StatusCol exists:

func TestViewStatePurity_List_StatusColFieldExists(t *testing.T) {
	body := app.ListBody{StatusCol: 1}
	if body.StatusCol != 1 {
		t.Fatal("unreachable")
	}
}
