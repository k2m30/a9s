// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"slices"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestSort_ColumnKeyRoundTripsForEveryRegisteredType walks every registered
// type's resolved column set and asserts that the key a sort names a column by
// finds that same column again. Leaving the list writes the key's column index
// into the list-view cache entry; re-entering reads the index back and asks for
// a key. When the two directions spell the key differently the sort the
// operator set is silently dropped on re-entry.
func TestSort_ColumnKeyRoundTripsForEveryRegisteredType(t *testing.T) {
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
			checked := 0
			for _, name := range colsAllShortNames() {
				cols := ctrl.ResolveColumnsForType(name)
				for i, c := range cols {
					key := c.SortColKey()
					if key == "" {
						t.Errorf("%s: column %d (%q) has no sort key at all — a sort on it can never be saved", name, i, c.Title)
						continue
					}
					if got := app.SortColIndex(cols, key); got != i {
						t.Errorf("%s: column %d (%q, key=%q path=%q) is named %q by the save direction, but the restore direction resolves that to column %d",
							name, i, c.Title, c.Key, c.Path, key, got)
					}
					checked++
				}
			}
			if checked == 0 {
				t.Fatal("no columns walked — the gate would pass vacuously")
			}
		})
	}
}

// TestSort_SavedSortSurvivesReEntry sorts a list on a path-only column,
// writes the column index the way leaving the list does, and rebuilds the
// list from the cache entry the way re-entering does. The sort the operator
// set must still be there.
func TestSort_SavedSortSurvivesReEntry(t *testing.T) {
	loaded := colsLoadedViewConfig(t)
	ctrl := newTestController(t)
	ctrl.SetViewConfig(loaded)

	shortName, colIdx := sortFirstPathOnlyColumn(t, ctrl)
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s: no registered type", shortName)
	}
	cols := ctrl.ResolveColumnsForType(shortName)
	key := cols[colIdx].SortColKey()

	sortOpenList(ctrl, shortName)
	ctrl.ApplyResourcesLoaded(shortName, sortTwoRows(), nil, false)
	ctrl.Apply(app.Action{Kind: app.ActionSort, Arg: key})

	// The save direction: leaving the list stores the sort column's index.
	sortCol, dir := ctrl.GetListSort()
	if sortCol != key {
		t.Fatalf("controller holds sort column %q, want %q", sortCol, key)
	}
	savedIdx := app.SortColIndex(cols, sortCol)
	if savedIdx != colIdx {
		t.Fatalf("leaving the list saved column index %d for key %q, want %d", savedIdx, key, colIdx)
	}

	// The restore direction: re-entering rebuilds the list from the entry.
	restored := newTestController(t)
	sortOpenList(restored, shortName)
	views.NewResourceListFromCache(
		*td, loaded, keys.Default(), sortTwoRows(), nil,
		"", savedIdx, dir != "desc", 0, 0, false, restored,
	)
	snap := restored.Snapshot()
	if snap.Body.List == nil {
		t.Fatal("re-entered list has no list body")
	}
	if snap.Body.List.Sort.Col != key {
		t.Errorf("re-entering %s restored sort column %q, want %q — the sort the operator set was dropped",
			shortName, snap.Body.List.Sort.Col, key)
	}
}

// sortFirstPathOnlyColumn returns a registered type and the index of one of
// its resolved columns that carries a Path and no Key.
func sortFirstPathOnlyColumn(t *testing.T, ctrl *app.Controller) (string, int) {
	t.Helper()
	for _, name := range colsAllShortNames() {
		if resource.FindResourceType(name) == nil {
			continue
		}
		for i, c := range ctrl.ResolveColumnsForType(name) {
			if c.Key == "" && c.Path != "" {
				return name, i
			}
		}
	}
	t.Fatal("no registered type has a path-only column — the gate would pass vacuously")
	return "", 0
}

// sortOpenList puts a top-level resource list for shortName on the controller's
// screen stack, the way navigating into one from the menu does.
func sortOpenList(c *app.Controller, shortName string) {
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenResourceList,
		Context: runtime.ScreenContext{ResourceType: shortName},
	}})
	c.EnsureListState()
}

func sortTwoRows() []resource.Resource {
	return []resource.Resource{
		{ID: "r-bbb", Name: "r-bbb", Fields: map[string]string{"name": "r-bbb"}},
		{ID: "r-aaa", Name: "r-aaa", Fields: map[string]string{"name": "r-aaa"}},
	}
}

// TestSort_SortKeyOverridesNameAFetcherField asserts every built-in column that
// declares a sort_key names a Fields key its own Wave 1 fetcher writes. A
// sort_key the fetcher never writes compares empty strings, so the list keeps
// whatever order it already had and the operator sees nothing happen.
func TestSort_SortKeyOverridesNameAFetcherField(t *testing.T) {
	checked := 0
	for _, td := range resource.AllResourceTypes() {
		for _, col := range config.GetViewDef(nil, td.ShortName).List {
			if col.SortKey == "" {
				continue
			}
			checked++
			if !slices.Contains(td.FieldKeys, col.SortKey) {
				t.Errorf("%s: column %q sorts on Fields key %q, which the fetcher does not declare in FieldKeys %v",
					td.ShortName, col.Title, col.SortKey, td.FieldKeys)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no built-in column declares a sort_key — the gate would pass vacuously")
	}
}
