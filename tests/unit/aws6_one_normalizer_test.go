package unit_test

// aws6_one_normalizer_test.go — the one normalizer answers every spelling the
// column cascade can produce, and the test harness resolves columns the way
// the app does.
//
// A fact is reached by four spellings: the RawStruct path a column declares
// ("SourceType"), the dotted path a nested one declares ("Source.Type"), the
// snake_case key a fetcher writes ("source_type"), and the label a detail row
// carries ("Source Type"). catalog.HumanizeFieldKey exists so ONE declaration
// on the type answers all four; a spelling it does not fold is a surface that
// silently keeps showing the SDK constant.
//
// The dotted path is not hypothetical. A column with no Key resolves by Path
// alone, and that is what an operator's own view file gives every column —
// ResolveListColumnCascade returns a loaded ViewDef verbatim, so the catalog's
// Key is not merged in on that arm. cb's Source Type is exactly that shape.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestOneNormalizerFoldsEverySpelling pins row 15's rule directly.
func TestOneNormalizerFoldsEverySpelling(t *testing.T) {
	groups := [][]string{
		{"source_type", "SourceType", "Source.Type", "Source Type", "sourcetype"},
		{"last_update_status", "LastUpdateStatus", "LastUpdate.Status", "Last Update Status"},
		{"stream_mode", "StreamMode", "Stream Mode"},
	}
	for _, g := range groups {
		want := catalog.HumanizeFieldKey(g[0])
		for _, spelling := range g[1:] {
			if got := catalog.HumanizeFieldKey(spelling); got != want {
				t.Errorf("HumanizeFieldKey(%q) = %q, want %q (from %q) — one declaration has to answer "+
					"every spelling the cascade produces, or the surface that uses the odd one keeps "+
					"rendering the SDK constant", spelling, got, want, g[0])
			}
		}
	}

	// The negative case: folding must not collapse two different facts. A
	// separator is noise, but a word is not.
	for _, pair := range [][2]string{
		{"source_type", "source_types"},
		{"stream_mode", "stream_status"},
		{"status", "last_update_status"},
		// A path whose leaf is the fact is NOT its leaf: folding an ancestor
		// away would make "role_name_arn" and "role" one key.
		{"stream_mode", "StreamModeDetails.StreamMode"},
	} {
		if catalog.HumanizeFieldKey(pair[0]) == catalog.HumanizeFieldKey(pair[1]) {
			t.Errorf("HumanizeFieldKey folds %q and %q to one key; they are different facts",
				pair[0], pair[1])
		}
	}
}

// TestAnOperatorsOwnViewFileRendersTheSameCells sweeps every type that ships a
// view file: the cell the bench renders and the cell an installation renders
// are the same cell.
//
// The demo bench, which is what acceptance looks at, and an installation
// with a loaded view file must read cells by one rule; otherwise a defect
// can be invisible on the bench and on screen for everyone.
//
// The comparison needs no second opinion about which spelling is right: a
// column that renders one thing under the built-in defaults and another under
// the file EnsureViewsDir writes is a fact with two answers, and one of them
// is reaching an operator.
func TestAnOperatorsOwnViewFileRendersTheSameCells(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)

	for _, td := range resource.AllResourceTypes() {
		rows := byType[td.ShortName]
		if len(rows) == 0 {
			continue
		}
		shipped := shippedViewConfigFor(td.ShortName)
		if shipped == nil {
			continue
		}

		t.Run(td.ShortName, func(t *testing.T) {
			fromFile := openListControllerWithConfig(t, td.ShortName, shipped)
			fromFile.ApplyResourcesLoaded(td.ShortName, rows, nil, false)

			bench := newVisibilityListController(t, td.ShortName)
			bench.ApplyResourcesLoaded(td.ShortName, rows, nil, false)

			a, b := fromFile.Snapshot().Body.List, bench.Snapshot().Body.List
			if a == nil || b == nil {
				t.Fatalf("%s rendered no list body", td.ShortName)
			}

			gotTitles := make([]string, 0, len(a.Columns))
			for _, col := range a.Columns {
				gotTitles = append(gotTitles, col.Title)
			}
			wantTitles := make([]string, 0, len(b.Columns))
			for _, col := range b.Columns {
				wantTitles = append(wantTitles, col.Title)
			}
			if strings.Join(gotTitles, "|") != strings.Join(wantTitles, "|") {
				t.Errorf("%s: an installation renders columns %v and the bench renders %v — "+
					"one of the two is a screen nobody reviews", td.ShortName, gotTitles, wantTitles)
			}

			for _, col := range b.Columns {
				want := aws6CellsByColumn(t, bench, col.Title)
				got := aws6CellsByColumn(t, fromFile, col.Title)
				for id, w := range want {
					if got[id] == w {
						continue
					}
					t.Errorf("%s row %q column %q: an installation renders %q where the bench renders %q — "+
						"one fact, two answers, and the bench is the one acceptance reads",
						td.ShortName, id, col.Title, got[id], w)
				}
			}
		})
	}
}

// TestLambdaListShowsItsHandler: the catalog declares a Handler column and the
// shipped view file must declare it too, since the view owns which columns
// there are — otherwise the column exists on the bench, which resolves its own
// set, and on no operator's screen.
//
// This pins only that the two agree for lambda, not whether a column list
// should have two owners at all.
func TestLambdaListShowsItsHandler(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	rows := byType["lambda"]
	if len(rows) == 0 {
		t.Fatal("no demo lambda rows")
	}

	c := openListControllerWithConfig(t, "lambda", shippedViewConfigFor("lambda"))
	c.ApplyResourcesLoaded("lambda", rows, nil, false)

	cells := aws6CellsByColumn(t, c, "Handler")
	if cells == nil {
		t.Fatal("the lambda list has no Handler column — the catalog declares one " +
			"(core/aws/catalog_compute.go) and the view file is what decides, so it has to declare it too")
	}
	shown := 0
	for _, v := range cells {
		if v != "" {
			shown++
		}
	}
	if shown == 0 {
		t.Errorf("every lambda row renders an empty Handler; a column that shows nothing for every row "+
			"is a header with no fact behind it (%d rows)", len(cells))
	}
}

// TestHarnessCellMatchesTheBenchCell is row 14. A harness that resolves its
// columns differently from the controller renders a different cell, and a test
// written against it pins something no operator ever sees.
func TestHarnessCellMatchesTheBenchCell(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	td := resource.FindResourceType("cb")
	if td == nil {
		t.Fatal("no cb resource type registered")
	}
	rows := byType["cb"]
	if len(rows) == 0 {
		t.Fatal("no demo cb rows")
	}

	harness := openListControllerWithConfig(t, "cb", configForType("cb"))
	harness.ApplyResourcesLoaded("cb", rows, nil, false)

	bench := newVisibilityListController(t, "cb")
	bench.ApplyResourcesLoaded("cb", rows, nil, false)

	got := aws6CellsByColumn(t, harness, "Source Type")
	want := aws6CellsByColumn(t, bench, "Source Type")
	if len(want) == 0 {
		t.Fatal("the bench rendered no Source Type cells, so there is nothing to compare against")
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("cb row %q: the harness renders Source Type as %q while the app renders %q — "+
				"the harness must resolve its columns through the production cascade, or a test written "+
				"against it pins a cell no operator sees", id, got[id], w)
		}
	}
}

// aws6CellsByColumn returns the rendered cell under the named column title,
// keyed by row id.
func aws6CellsByColumn(t *testing.T, c *app.Controller, title string) map[string]string {
	t.Helper()
	body := c.Snapshot().Body.List
	if body == nil {
		return nil
	}
	col := -1
	for i, h := range body.Columns {
		if h.Title == title {
			col = i
			break
		}
	}
	if col < 0 {
		return nil
	}
	out := map[string]string{}
	for _, row := range body.Rows {
		if col < len(row.Cells) {
			out[row.ResourceID] = row.Cells[col]
		}
	}
	return out
}

// shippedViewConfigFor returns the view definition an installation carries on
// disk for one type: what cmd/viewsgen writes, which is config.DefaultConfig's
// entry. It is handed to the controller as a LOADED config, so the cascade
// takes the arm that returns a file's columns verbatim.
func shippedViewConfigFor(typeName string) *config.ViewsConfig {
	full := config.DefaultConfig()
	vd, ok := full.Views[typeName]
	if !ok {
		return nil
	}
	return &config.ViewsConfig{Views: map[string]config.ViewDef{typeName: vd}}
}
