// The TUI list renderer is a pure consumer
// of app.ListBody.
//
// Contract under test: internal/tui/views/resourcelist.go's RenderList(body)
// derives every presentation decision from the ListBody fields it is given,
// not from body.EnrichmentFindings or any other side channel:
//
//  1. The identity cell carries no "!"/"~" glyph: a row's colour is the
//     worst finding over both waves, so a row with anything to say is
//     already off-green (docs/attention-signals.md).
//  2. The status cell: buildListBody bakes any status-column override into
//     row.Cells, and RenderList consumes body.StatusCol rather than
//     re-resolving the index from the type definition.
//
// Each test constructs a SYNTHETIC ListBody whose pre-resolved Cells
// disagree with what a re-derivation from EnrichmentFindings would produce.
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

// purityTypeDef builds a minimal ResourceTypeDef: column 0 = "Name" (the
// identity column by key=="name"), column 1 = "Status" (status/lifecycle
// column).
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

// The findings map carries an issue finding for the row, and the row renders
// with no glyph on any cell: a row's colour is the worst finding over both
// waves, so there is nothing for a marker to add.

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
		Columns:     purityColumns(),
		Rows:        []app.ListRow{row},
		Selected:    0,
		IdentityCol: 0,
		StatusCol:   1, // "Status" column — matches purityColumns()[1]
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

// Cells already carry the baked phrase and the findings map carries a
// DIFFERENT phrase for the same row; the renderer shows the Cells value
// verbatim.

func TestViewStatePurity_List_StatusCellFollowsCellsNotFindingsPhrase(t *testing.T) {
	ensureNoColor(t)
	styles.ReinitForTest()
	t.Cleanup(styles.ReinitForTest)

	td := purityTypeDef("purity-status-cell")
	m := newPurityListModel(td)

	const bakedPhrase = "baked: disk pressure high"
	const findingsPhrase = "DECOY: should never render"

	row := app.ListRow{
		// Cells[1] is the "status" column — already carries the baked phrase.
		Cells:      []string{"demo-instance-3", bakedPhrase},
		ResourceID: "res-3",
		Color:      "healthy",
	}
	// The status column is declared wide enough for the baked phrase: the width
	// a hand-assembled body publishes is the width the painter fills, like every
	// other column's.
	columns := purityColumns()
	// Wide enough for the DECOY, not just the baked phrase: a column that
	// truncates the decoy would take the string this case is looking for off
	// the screen and pass however the renderer behaved.
	columns[1].Width = max(len(bakedPhrase), len(findingsPhrase)) + 1

	body := app.ListBody{
		Columns:     columns,
		Rows:        []app.ListRow{row},
		Selected:    0,
		IdentityCol: 0,
		StatusCol:   1, // "Status" column — matches purityColumns()[1]
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
