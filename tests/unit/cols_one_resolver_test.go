// cols_one_resolver_test.go — a resource type has exactly one answer to
// "what are my list columns", and exactly one column that names its rows.
//
// Two resolvers currently answer the first question — (*app.Controller).
// ResolveColumnsForType and resource.ResolveListColumnCascade — and they
// disagree about Path, Humanize and whether a child short name resolves at
// all. Whichever one a caller happens to reach decides what the user sees,
// so the two must be indistinguishable field by field, for every registered
// type, with and without a view config loaded from disk.
//
// The identity column is elected twice for the same reason: once over the
// set that is rendered, and again inside the cell extractor over the type's
// built-in set, which is not the same set once a view file is loaded. When
// those two elections disagree, a row with nothing but an ID and a name —
// a warm-cache replay, a degraded fetch, a related-panel stub — shows its
// name under a column the marker glyph is not on, or under no column at
// all. The election belongs on the resolved set, and every consumer of it
// (the cell extractor, the marker column, the sort comparator) must read
// that one election.
package unit_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// colsTypeDefFor resolves the typeDef the cascade must be handed for a short
// name: the catalog parent, else the registered child. The controller's own
// lookup consults only the parent half, which is the point of the gate.
func colsTypeDefFor(shortName string) *resource.ResourceTypeDef {
	if td := resource.FindResourceType(shortName); td != nil {
		return td
	}
	return resource.GetChildType(shortName)
}

// colsAllShortNames is every short name a caller may hand a column resolver:
// both list-openable parents and the child types reached through a parent's
// drill-down. internal/tui/app_stack.go and views/resourcelist.go both call
// ResolveColumnsForType with a typeDef ShortName that may be either.
func colsAllShortNames() []string {
	names := make([]string, 0, 256)
	for _, td := range resource.AllResourceTypes() {
		names = append(names, td.ShortName)
	}
	names = append(names, resource.AllChildShortNamesForTest()...)
	sort.Strings(names)
	return names
}

// colsCascade is the cascade's answer in the controller's own shape, so the
// two answers are comparable field by field. The mapping lives here and not
// in production precisely because there must be only one resolver to map
// from.
func colsCascade(vc *config.ViewsConfig, shortName string) []app.ColumnDef {
	lcs := resource.ResolveListColumnCascade(vc, shortName, colsTypeDefFor(shortName))
	cols := make([]app.ColumnDef, len(lcs))
	for i, lc := range lcs {
		cols[i] = app.ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path, Humanize: lc.Humanize}
	}
	return cols
}

// colsDescribe renders one column's five identifying fields for a failure
// message. Width is included: a column set that agrees on everything but
// width still lays the list out differently.
func colsDescribe(c app.ColumnDef) string {
	return fmt.Sprintf("{key=%q title=%q width=%d path=%q humanize=%v}", c.Key, c.Title, c.Width, c.Path, c.Humanize)
}

// colsCompare reports every field on which two answers for the same type
// differ. An empty result means the two resolvers are indistinguishable for
// that type.
func colsCompare(got, want []app.ColumnDef) []string {
	if len(got) != len(want) {
		return []string{fmt.Sprintf("column count: controller=%d cascade=%d", len(got), len(want))}
	}
	var diffs []string
	for i := range got {
		g, w := got[i], want[i]
		if g.Key != w.Key || g.Title != w.Title || g.Width != w.Width || g.Path != w.Path || g.Humanize != w.Humanize {
			diffs = append(diffs, fmt.Sprintf("col[%d]: controller=%s cascade=%s", i, colsDescribe(g), colsDescribe(w)))
		}
	}
	return diffs
}

// colsLoadedViewConfig loads the shipped per-resource view files the way a
// running app loads a user's own. A view file stores only what differs from
// the built-in default and keys a column by its title, so the loaded shape
// and the built-in shape are genuinely different inputs to the resolvers —
// both halves of the gate are needed.
func colsLoadedViewConfig(t *testing.T) *config.ViewsConfig {
	t.Helper()
	vc, err := config.LoadFromDirs([]string{"../../.a9s/views"})
	if err != nil {
		t.Fatalf("loading .a9s/views: %v", err)
	}
	if vc == nil || len(vc.Views) == 0 {
		t.Fatalf("loading .a9s/views: no view files found")
	}
	return vc
}

// TestCols_ControllerAnswerEqualsCascadeAnswer walks every registered type,
// parent and child, and asserts the controller's column set is the cascade's
// column set — with no view config and with the shipped view files loaded.
// Both must also be non-empty: a type that resolves no columns renders a list
// with no cells, which is a worse failure than disagreeing about one of them.
func TestCols_ControllerAnswerEqualsCascadeAnswer(t *testing.T) {
	loaded := colsLoadedViewConfig(t)

	for _, half := range []struct {
		name string
		vc   *config.ViewsConfig
	}{
		{"no_view_config", nil},
		{"shipped_view_files_loaded", loaded},
	} {
		t.Run(half.name, func(t *testing.T) {
			ctrl := newTestController(t)
			ctrl.SetViewConfig(half.vc)

			for _, name := range colsAllShortNames() {
				got := ctrl.ResolveColumnsForType(name)
				want := colsCascade(half.vc, name)

				if len(want) == 0 {
					t.Errorf("%s: the cascade resolves no columns at all — this type would render a list with no cells", name)
					continue
				}
				if len(got) == 0 {
					t.Errorf("%s: the controller resolves no columns where the cascade resolves %d — a caller reaching this resolver renders an empty list", name, len(want))
					continue
				}
				for _, d := range colsCompare(got, want) {
					t.Errorf("%s: the two resolvers disagree — %s", name, d)
				}
			}
		})
	}
}

// TestCols_ExactlyOneResolvedColumnIsTheIdentityColumn pins the shape the
// election travels on: whatever branch of the resolver answers, the set it
// returns carries the election, and it carries it exactly once. A set with no
// flagged column leaves every consumer to re-elect for itself, which is the
// second truth source; a set with two leaves them to disagree about which.
func TestCols_ExactlyOneResolvedColumnIsTheIdentityColumn(t *testing.T) {
	loaded := colsLoadedViewConfig(t)

	for _, half := range []struct {
		name string
		vc   *config.ViewsConfig
	}{
		{"no_view_config", nil},
		{"shipped_view_files_loaded", loaded},
	} {
		t.Run(half.name, func(t *testing.T) {
			ctrl := newTestController(t)
			ctrl.SetViewConfig(half.vc)

			for _, name := range colsAllShortNames() {
				cols := ctrl.ResolveColumnsForType(name)
				if len(cols) == 0 {
					continue // reported by the equality gate above
				}
				var flagged []string
				for _, c := range cols {
					if c.Identity {
						flagged = append(flagged, c.Title)
					}
				}
				if len(flagged) != 1 {
					t.Errorf("%s: %d columns flagged Identity (%v), want exactly 1", name, len(flagged), flagged)
				}
			}
		})
	}
}

// colsDegradedRows is the shape a row has when nothing but the disk cache or
// a related-panel stub produced it: an ID, a name, no Fields and no SDK
// struct. Every path-backed column has nothing to say about such a row, and
// the row's name belongs in the one column that names it — the column the
// marker glyph sits on.
func colsDegradedRows(shortName string) []resource.Resource {
	return []resource.Resource{
		{ID: shortName + "-alpha", Name: "acme-" + shortName + "-alpha", Type: shortName},
		{ID: shortName + "-bravo", Name: "acme-" + shortName + "-bravo", Type: shortName},
	}
}

// TestCols_MarkerColumnIsTheColumnThatShowsTheRowName drives the app's own
// render path: a list opened on the controller with the shipped view files
// loaded, rows delivered through the real ResourcesLoaded event, cells read
// off the ListBody the renderers consume. The column the body marks as the
// identity column (MarkerCol) must be the column whose cell carries the row's
// name, and no other column may carry it.
//
// ct-events and eb are the types where this fails today: their view file
// elects a different column than their built-in set does, and the extractor
// re-elects from the built-in set, so the name lands under a column the
// marker is not on — or, when no built-in title matches a view-file title,
// under no column at all and the first cell of every row goes blank.
func TestCols_MarkerColumnIsTheColumnThatShowsTheRowName(t *testing.T) {
	vc := colsLoadedViewConfig(t)

	for _, td := range resource.AllResourceTypes() {
		t.Run(td.ShortName, func(t *testing.T) {
			ctrl := newTestController(t)
			ctrl.SetViewConfig(vc)
			ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})

			rows := colsDegradedRows(td.ShortName)
			ctrl.Handle(messages.ResourcesLoaded{
				ResourceType: td.ShortName,
				Resources:    rows,
				Provenance:   messages.FetchProvenanceCanonicalList,
			})

			lb := ctrl.Snapshot().Body.List
			if lb == nil {
				t.Fatalf("%s: no list body after opening the list", td.ShortName)
			}
			if len(lb.Rows) != len(rows) {
				t.Fatalf("%s: %d rows rendered, want %d", td.ShortName, len(lb.Rows), len(rows))
			}
			if lb.MarkerCol < 0 || lb.MarkerCol >= len(lb.Columns) {
				t.Fatalf("%s: MarkerCol=%d out of range for %d columns", td.ShortName, lb.MarkerCol, len(lb.Columns))
			}

			for _, lr := range lb.Rows {
				want := ""
				for _, r := range rows {
					if r.ID == lr.ResourceID {
						want = r.Name
					}
				}
				if want == "" {
					t.Fatalf("%s: rendered row %q is not one of the rows delivered", td.ShortName, lr.ResourceID)
				}
				for i, cell := range lr.Cells {
					switch {
					case i == lb.MarkerCol && cell != want:
						t.Errorf("%s: MarkerCol is col[%d] %q, but its cell on row %q is %q, want the row's name %q",
							td.ShortName, i, lb.Columns[i].Title, lr.ResourceID, cell, want)
					case i != lb.MarkerCol && cell == want:
						t.Errorf("%s: col[%d] %q shows the row's own name %q on row %q, but the marker is on col[%d] %q",
							td.ShortName, i, lb.Columns[i].Title, want, lr.ResourceID, lb.MarkerCol, lb.Columns[lb.MarkerCol].Title)
					}
				}
			}
		})
	}
}

// TestCols_SortingByTheIdentityColumnOrdersDegradedRowsByName pins the third
// consumer of the election. The sort comparator builds its own ColumnDef from
// the view definition rather than from the resolved set, so on a row with no
// struct and no Fields it asks the extractor for a value the extractor will
// only produce for the identity column — and the comparator's hand-built
// column is not that column. Every row then compares equal and the list keeps
// its arrival order, so sorting by the one column that has anything to say
// about these rows does nothing at all.
//
// ct-events is driven here because its view file's identity column ("V") and
// its built-in one ("Time") are the pair that diverge; the rows are delivered
// in descending name order so a working sort has to move them.
func TestCols_SortingByTheIdentityColumnOrdersDegradedRowsByName(t *testing.T) {
	const shortName = "ct-events"
	vc := colsLoadedViewConfig(t)

	ctrl := newTestController(t)
	ctrl.SetViewConfig(vc)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})

	rows := colsDegradedRows(shortName)
	reversed := []resource.Resource{rows[1], rows[0]}
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: shortName,
		Resources:    reversed,
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	cols := ctrl.ResolveColumnsForType(shortName)
	identity := app.ColumnDef{}
	for _, c := range cols {
		if c.Identity {
			identity = c
		}
	}
	if identity.Key == "" {
		t.Fatalf("%s: the resolved set flags no identity column with a sortable key: %+v", shortName, cols)
	}

	ctrl.Apply(app.Action{Kind: app.ActionSort, Arg: identity.Key})

	lb := ctrl.Snapshot().Body.List
	if lb == nil || len(lb.Rows) != 2 {
		t.Fatalf("%s: want 2 rendered rows after sorting, got %v", shortName, lb)
	}
	if got := []string{lb.Rows[0].ResourceID, lb.Rows[1].ResourceID}; got[0] != rows[0].ID || got[1] != rows[1].ID {
		t.Errorf("%s: sorting ascending by the identity column %q left the rows in %v, want %v — the comparator read a column that carries no election, so every row compared equal",
			shortName, identity.Title, got, []string{rows[0].ID, rows[1].ID})
	}
}
