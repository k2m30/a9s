// tui_viewstate_purity_list_test.go — the TUI list renderer is a pure consumer
// of app.ListBody.
//
// Contract under test: internal/tui/views/resourcelist.go's RenderList(body)
// derives every presentation decision from the ListBody fields it is given,
// and never re-derives one from body.EnrichmentFindings or any other side
// channel. Two decisions were re-derived when this file was written, and
// neither is any more:
//
//  1. The "!"/"~" glyph on the identity cell. There is no glyph: a row's
//     colour is the worst finding over both waves, so a row with anything to
//     say is already off-green and nothing annotates it
//     (docs/attention-signals.md §Visualization Surfaces). Case 2 below is
//     what keeps a re-derivation from growing back in its place.
//  2. The status cell. buildListBody bakes any status-column override into
//     row.Cells, and RenderList consumes body.StatusCol rather than
//     re-resolving the index from the type definition.
//
// Each test constructs a SYNTHETIC ListBody whose pre-resolved Cells
// deliberately DISAGREE with what a re-derivation from EnrichmentFindings
// would produce, so a renderer that went back to re-deriving fails here.
//
// Harness: construct a views.ResourceListModel via views.NewResourceList(td,
// nil, k), SetSize, then call m.RenderList(body) directly with a hand-built
// app.ListBody. No controller is involved, which is what lets the body's
// fields disagree with the findings map.
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

func TestViewStatePurity_List_NoGlyphIsDerivedFromFindings(t *testing.T) {
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
// app.ListBody.StatusCol exists, and is what RenderList consumes instead of
// re-resolving the status-column index from the type definition. This started
// as a deliberately non-compiling marker asking for the field; it compiles
// now, and stays as the pin that the field is not quietly removed.
// ---------------------------------------------------------------------------

func TestViewStatePurity_List_StatusColFieldExists(t *testing.T) {
	body := app.ListBody{StatusCol: 1}
	if body.StatusCol != 1 {
		t.Fatal("unreachable")
	}
}
