// cols_one_resolver_test.go — a resource type has exactly one answer to
// "what are my list columns", and exactly one column that names its rows.
//
// Two resolvers currently answer the first question — (*app.Controller).
// ResolveColumnsForType and resource.ResolveListColumnCascade — and they
// disagree about Path and whether a child short name resolves at
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
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/tests/unit"
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
//
// INVERTED for aws6 rows 4-6 (one humanize owner): the humanize opt-in moved
// off config.ListColumn and onto the TYPE (ResourceTypeDef.HumanizeFields),
// so the mirror reads it from the type the way production does. The old
// `lc.Humanize` read is not to be restored — a column-owned flag cannot reach
// a field no column shows, which is the defect that moved it.
func colsCascade(vc *config.ViewsConfig, shortName string) []app.ColumnDef {
	td := colsTypeDefFor(shortName)
	lcs := resource.ResolveListColumnCascade(vc, shortName, td)
	var humanized map[string]bool
	if td != nil {
		humanized = td.HumanizedFields()
	}
	cols := make([]app.ColumnDef, len(lcs))
	for i, lc := range lcs {
		cols[i] = app.ColumnDef{
			Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path,
			Humanize: catalog.Humanizes(humanized, lc.Key, lc.Path),
		}
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

// TestCols_IdentityColumnIsTheColumnThatShowsTheRowName drives the app's own
// render path: a list opened on the controller with the shipped view files
// loaded, rows delivered through the real ResourcesLoaded event, cells read
// off the ListBody the renderers consume. The column the body marks as the
// identity column (IdentityCol) must be the column whose cell carries the row's
// name, and no other column may carry it.
//
// ct-events and eb are the types where this fails today: their view file
// elects a different column than their built-in set does, and the extractor
// re-elects from the built-in set, so the name lands under a column the
// marker is not on — or, when no built-in title matches a view-file title,
// under no column at all and the first cell of every row goes blank.
func TestCols_IdentityColumnIsTheColumnThatShowsTheRowName(t *testing.T) {
	vc := colsLoadedViewConfig(t)

	for _, td := range resource.AllResourceTypes() {
		t.Run(td.ShortName, func(t *testing.T) {
			ctrl := newTestController(t)
			ctrl.SetViewConfig(vc)
			ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})

			rows := colsDegradedRows(td.ShortName)
			handlePage(ctrl, messages.ResourcesLoaded{
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
			if lb.IdentityCol < 0 || lb.IdentityCol >= len(lb.Columns) {
				t.Fatalf("%s: IdentityCol=%d out of range for %d columns", td.ShortName, lb.IdentityCol, len(lb.Columns))
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
					case i == lb.IdentityCol && cell != want:
						t.Errorf("%s: IdentityCol is col[%d] %q, but its cell on row %q is %q, want the row's name %q",
							td.ShortName, i, lb.Columns[i].Title, lr.ResourceID, cell, want)
					case i != lb.IdentityCol && cell == want:
						t.Errorf("%s: col[%d] %q shows the row's own name %q on row %q, but the marker is on col[%d] %q",
							td.ShortName, i, lb.Columns[i].Title, want, lr.ResourceID, lb.IdentityCol, lb.Columns[lb.IdentityCol].Title)
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
	handlePage(ctrl, messages.ResourcesLoaded{
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

// TestCols_SaveColumnsAreAProjectionOfTheResolvedSet pins the third lane. The
// cache-save resolver must not resolve anything: it must project the set the
// render path shows, so a save can never persist a column the list does not
// render, or miss one it does. Walked for every type with and without the
// shipped view files, because the two lanes previously duplicated the typeDef
// lookup and would diverge on whichever branch that lookup answered
// differently.
func TestCols_SaveColumnsAreAProjectionOfTheResolvedSet(t *testing.T) {
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
				rendered := ctrl.ResolveColumnsForType(name)
				saved := ctrl.SaveColumnsForType(name)
				if len(saved) != len(rendered) {
					t.Errorf("%s: the save lane resolves %d columns, the render lane %d", name, len(saved), len(rendered))
					continue
				}
				for i := range rendered {
					r, s := rendered[i], saved[i]
					if s.Key != r.Key || s.Title != r.Title || s.Width != r.Width || s.Path != r.Path {
						t.Errorf("%s: col[%d] saved as {key=%q title=%q width=%d path=%q} but rendered as %s",
							name, i, s.Key, s.Title, s.Width, s.Path, colsDescribe(r))
					}
				}
			}
		})
	}
}

// TestCols_SortOverridesSurviveOntoTheResolvedColumn pins the field that
// exists for one purpose: to let a column sort by something other than the text
// it shows. A size rendered "1.2 GB" and a timestamp rendered "Mar 28 14:30"
// sort wrongly as text, so the built-in view declares the raw Fields key to
// compare instead. The sort_path companion this test also covered was removed
// with the sort task: it read the AWS struct, which a warm-cache row does not
// have, so it ordered the cached frame differently from the live one. Do not
// restore that assertion. Those declarations have to reach the
// column the comparator reads, on the branch a real user's session takes —
// which is the one with the shipped view files loaded, not the built-in
// defaults.
func TestCols_SortOverridesSurviveOntoTheResolvedColumn(t *testing.T) {
	loaded := colsLoadedViewConfig(t)
	ctrl := newTestController(t)
	ctrl.SetViewConfig(loaded)

	checked := 0
	for _, name := range colsAllShortNames() {
		defaults := config.GetViewDef(nil, name).List
		resolved := ctrl.ResolveColumnsForType(name)
		byTitle := make(map[string]app.ColumnDef, len(resolved))
		for _, c := range resolved {
			byTitle[c.Title] = c
		}
		for _, def := range defaults {
			if def.SortKey == "" {
				continue
			}
			checked++
			got, ok := byTitle[def.Title]
			if !ok {
				t.Errorf("%s: the built-in view declares a sort override on %q but no resolved column carries that title", name, def.Title)
				continue
			}
			if got.SortKey != def.SortKey {
				t.Errorf("%s: column %q resolves SortKey %q, want %q — the comparator falls back to sorting the displayed text",
					name, def.Title, got.SortKey, def.SortKey)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no built-in column declares a sort override — the gate would pass vacuously")
	}
}

// TestCols_SortingCloudTrailByTimeIsChronological is the user-visible half of
// the rule above. A CloudTrail event's time cell reads "Mar 28 14:30:15", so
// sorting those cells as text puts April before March and every August before
// every February. The built-in view declares the RFC3339 field to compare
// instead; sorting the list ascending has to produce that order.
func TestCols_SortingCloudTrailByTimeIsChronological(t *testing.T) {
	const shortName = "ct-events"
	var td resource.ResourceTypeDef
	for _, x := range resource.AllResourceTypes() {
		if x.ShortName == shortName {
			td = x
		}
	}
	rows, ok := unit.DrainFixtures(t, td, demo.NewServiceClients())
	if !ok || len(rows) < 2 {
		t.Fatalf("%s: need at least 2 demo rows, got %d", shortName, len(rows))
	}

	ctrl := newTestController(t)
	ctrl.SetViewConfig(colsLoadedViewConfig(t))
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: shortName,
		Resources:    rows,
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	ctrl.Apply(app.Action{Kind: app.ActionSort, Arg: "time"})

	lb := ctrl.Snapshot().Body.List
	if lb == nil || len(lb.Rows) < 2 {
		t.Fatalf("%s: want at least 2 rendered rows after sorting, got %v", shortName, lb)
	}
	rawByID := make(map[string]string, len(rows))
	for _, r := range rows {
		rawByID[r.ID] = r.Fields["event_time"]
	}
	for i := 1; i < len(lb.Rows); i++ {
		prev, cur := rawByID[lb.Rows[i-1].ResourceID], rawByID[lb.Rows[i].ResourceID]
		if prev == "" || cur == "" {
			t.Fatalf("%s: fixture assumption broken — a rendered row carries no event_time", shortName)
		}
		if prev > cur {
			t.Fatalf("%s: sorting ascending by TIME put %s before %s — the comparator compared the displayed text, not the event time",
				shortName, prev, cur)
		}
	}
}
