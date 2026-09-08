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

// TestAnOperatorsOwnViewFileRendersTheSameCells is the bench half of row 15.
//
// ResolveListColumnCascade has two arms. With no view config it merges the
// catalog's Key onto the defaults' Path; with one loaded it returns that file's
// columns verbatim, Key-less where the file is Key-less. Every installation
// takes the second arm, because EnsureViewsDir writes those files on first
// start — so a rule that reads a column by Key works on the bench and not on
// anyone's machine.
//
// The comparison is between the two arms rather than against a list of
// expected words, so it needs no second opinion about which spelling is
// right: a column that renders one thing under the merged set and another
// under the shipped file is a fact with two answers, and one of them is
// reaching an operator.
func TestAnOperatorsOwnViewFileRendersTheSameCells(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)

	for _, td := range resource.AllResourceTypes() {
		if len(td.HumanizeFields) == 0 {
			continue
		}
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

			merged := newVisibilityListController(t, td.ShortName)
			merged.ApplyResourcesLoaded(td.ShortName, rows, nil, false)

			a, b := fromFile.Snapshot().Body.List, merged.Snapshot().Body.List
			if a == nil || b == nil {
				t.Fatalf("%s rendered no list body", td.ShortName)
			}
			fileHasColumn := map[string]bool{}
			for _, col := range a.Columns {
				fileHasColumn[col.Title] = true
			}
			for _, col := range b.Columns {
				// A column the shipped file does not declare at all is a
				// different question — the two declarations disagreeing about
				// the column SET, not about what one cell says — and it is not
				// what this test can see. lambda's Handler is that case.
				if !fileHasColumn[col.Title] {
					continue
				}
				want := aws6CellsByColumn(t, merged, col.Title)
				got := aws6CellsByColumn(t, fromFile, col.Title)
				for id, w := range want {
					if got[id] == w {
						continue
					}
					t.Errorf("%s row %q column %q: an operator's own view file renders %q where the "+
						"catalog-merged set renders %q — one fact, two answers, and the file is the one "+
						"every installation reads", td.ShortName, id, col.Title, got[id], w)
				}
			}
		})
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
