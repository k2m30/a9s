// tui_viewstate_purity_list_test.go — TDD red-light tests pinning the TUI
// list renderer as a PURE consumer of app.ListBody.
//
// Contract under test: internal/tui/views/resourcelist.go's RenderList(body)
// must derive every presentation decision from the ListBody fields it is
// given — it must NOT re-derive them from body.EnrichmentFindings or any
// other side channel. Today RenderList re-derives three things instead of
// consuming the pre-resolved body fields:
//
//  1. renderListDataRow (~line 882 resourcelist.go): the "!"/"~" glyph is
//     re-derived from body.EnrichmentFindings[row.ResourceID] + row.Color ==
//     "healthy", instead of consuming row.Decorator (which buildListBody
//     already computed via resolveListDecoratorFull).
//  2. renderListDataRow (~line 874): the S4 status-cell override re-applies
//     findings[row.ResourceID].Phrase at statusColIdx, even though
//     buildListBody (list_body.go ~line 124) already bakes the phrase into
//     row.Cells[statusCol].
//  3. RenderList (~line 583-617): markerColIdx/statusColIdx are re-resolved
//     from cols/fullCols/typeDef instead of consuming body.MarkerCol (which
//     exists) and a to-be-added body.StatusCol (which does NOT exist yet —
//     see TestViewStatePurity_StatusCol_FieldExists below, legitimate
//     compile-red until the architect adds the field).
//
// Each test below constructs a SYNTHETIC ListBody whose pre-resolved fields
// (Decorator, Cells) deliberately DISAGREE with what re-derivation from
// EnrichmentFindings would produce. A pure renderer follows the body fields;
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
// Case 1 — Decorator="!" present, findings map does NOT contain the row's ID.
// A pure renderer consumes row.Decorator and MUST show the glyph. Today's
// renderer re-derives from findings[row.ResourceID] (absent) and shows none.
// ---------------------------------------------------------------------------

func TestViewStatePurity_List_GlyphFollowsDecoratorNotFindings_Present(t *testing.T) {
	ensureNoColor(t)
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	td := purityTypeDef("purity-glyph-present")
	m := newPurityListModel(td)

	row := app.ListRow{
		Cells:      []string{"demo-instance-1", "running"},
		Decorator:  app.DecoratorError, // "!" — pre-resolved by buildListBody
		ResourceID: "res-1",
		Color:      "healthy",
	}
	body := app.ListBody{
		Columns:   purityColumns(),
		Rows:      []app.ListRow{row},
		Selected:  0,
		MarkerCol: 0,
		StatusCol: 1, // "Status" column — matches purityColumns()[1]
		// Deliberately EMPTY — no finding for res-1. Re-derivation (which scans
		// EnrichmentFindings[row.ResourceID]) will find nothing and skip the
		// glyph. A pure consumer of row.Decorator must still show it.
		EnrichmentFindings: map[string][]domain.Finding{},
	}

	out := m.RenderList(body)
	if !strings.Contains(out, "! demo-instance-1") {
		t.Errorf("expected glyph prefix from row.Decorator (pure consumer) even though "+
			"EnrichmentFindings has no entry for res-1 — got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Case 2 — Decorator="" (no glyph), findings map DOES contain an issue
// finding for the row. A pure renderer consumes row.Decorator=="" and MUST
// NOT show a glyph. Today's renderer re-derives from findings and shows one.
// ---------------------------------------------------------------------------

func TestViewStatePurity_List_GlyphFollowsDecoratorNotFindings_Absent(t *testing.T) {
	ensureNoColor(t)
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	td := purityTypeDef("purity-glyph-absent")
	m := newPurityListModel(td)

	row := app.ListRow{
		Cells:      []string{"demo-instance-2", "running"},
		Decorator:  app.DecoratorNormal, // "" — pre-resolved: no glyph
		ResourceID: "res-2",
		Color:      "healthy",
	}
	body := app.ListBody{
		Columns:   purityColumns(),
		Rows:      []app.ListRow{row},
		Selected:  0,
		MarkerCol: 0,
		StatusCol: 1, // "Status" column — matches purityColumns()[1]
		// Deliberately carries an issue-severity finding for res-2. Re-derivation
		// will find it and prepend "! " even though Decorator says otherwise.
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
		t.Errorf("expected NO glyph prefix — row.Decorator is DecoratorNormal (pure consumer "+
			"contract), but EnrichmentFindings carries an issue finding for res-2 that a "+
			"re-deriving renderer would surface as a glyph — got:\n%s", out)
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
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	td := purityTypeDef("purity-status-cell")
	m := newPurityListModel(td)

	const bakedPhrase = "baked: disk pressure high"
	const findingsPhrase = "DECOY: should never render"

	row := app.ListRow{
		// Cells[1] is the "status" column — already carries the baked S4 phrase.
		Cells:      []string{"demo-instance-3", bakedPhrase},
		Decorator:  app.DecoratorNormal,
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
// Case 4 — body.MarkerCol points at column 1 (not 0), with a decorator
// present. Glyph must prefix column 1's cell exactly. This exercises the
// SAME markerColIdx-translation code path RenderList already uses for
// body.MarkerCol (unlike Case 1/2, this does not require touching the
// re-derivation branch) — kept as a pin either way; see the per-test comment
// on which side of the contract it verifies.
// ---------------------------------------------------------------------------

func TestViewStatePurity_List_MarkerColSelectsPrefixedColumn(t *testing.T) {
	ensureNoColor(t)
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	td := purityTypeDef("purity-markercol")
	m := newPurityListModel(td)

	row := app.ListRow{
		Cells:      []string{"demo-instance-4", "needs-attention"},
		Decorator:  app.DecoratorError,
		ResourceID: "res-4",
		Color:      "healthy",
	}
	body := app.ListBody{
		Columns:  purityColumns(),
		Rows:     []app.ListRow{row},
		Selected: 0,
		// Marker column is column 1 ("Status"), NOT the default column 0 ("Name").
		MarkerCol: 1,
		StatusCol: 1, // "Status" column — matches purityColumns()[1]
		EnrichmentFindings: map[string][]domain.Finding{
			"res-4": {{
				Code:     "PURITY-TEST",
				Phrase:   "needs-attention",
				Severity: domain.SevBroken,
			}},
		},
	}

	out := m.RenderList(body)
	// PIN: this assertion currently PASSES — RenderList already translates
	// body.MarkerCol (full-column-space) into the visible markerColIdx via the
	// fullMarkerColIdx cascade at resourcelist.go ~line 583-592, and
	// renderListDataRow prepends the glyph at i==markerColIdx regardless of
	// which column that is. Kept as a purity pin: if a future change makes
	// the glyph placement depend on re-derived identity-column logic instead
	// of body.MarkerCol, this test must catch the regression.
	if !strings.Contains(out, "! needs-attention") {
		t.Errorf("expected glyph to prefix column 1 (body.MarkerCol=1)'s cell "+
			"\"needs-attention\", not column 0 — got:\n%s", out)
	}
	if strings.Contains(out, "! demo-instance-4") {
		t.Errorf("glyph incorrectly prefixed column 0 instead of body.MarkerCol=1 — got:\n%s", out)
	}
}

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
