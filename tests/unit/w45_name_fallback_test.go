// w45_name_fallback_test.go — the cell extractor's name fallback and the
// identity-column election must both key on the type's identity column, never
// on a substring of a key, a title or a path.
//
// A column whose path merely contains "name" (ng's "Cluster" -> Path
// "ClusterName", ecs's "Cluster" -> Path "ClusterArn"+, sfn's "State Machine"
// ...) is not the column that names the row. When the row has no RawStruct —
// a warm-cache replay, a related-panel stub, a row whose fetch degraded — the
// path branch cannot answer and the substring fallback hands back r.Name, so
// the cell confidently shows the row's own name under a foreign heading. A
// blank cell is the honest answer; the row's name belongs only in the row's
// identity column.
package unit_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/tests/unit"
)

// w45Columns resolves one type's list columns through the shared cascade
// core/app's resolveListColumnsForBuild and core/runtime's resolveSaveColumns
// both delegate to, with no session view config — the column set a fresh
// install renders.
func w45Columns(td resource.ResourceTypeDef) []app.ColumnDef {
	lcs := resource.ResolveListColumnCascade(nil, td.ShortName, &td)
	cols := make([]app.ColumnDef, len(lcs))
	for i, lc := range lcs {
		cols[i] = app.ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path, Humanize: lc.Humanize}
	}
	return cols
}

// w45NameLeak reports whether col renders r.Name *because of the name
// fallback* rather than because the row genuinely carries its own name under
// that column. Clearing r.Name is the discriminator: a cell sourced from
// Fields or from a path is unchanged by it, a cell sourced from the fallback
// collapses to "". Without this a fixture whose Fields value happens to equal
// the row name would be an un-fixable false positive.
func w45NameLeak(col app.ColumnDef, td *resource.ResourceTypeDef, r resource.Resource) bool {
	if r.Name == "" {
		return false
	}
	if app.ExtractCellValue(col, td, r) != r.Name {
		return false
	}
	noName := r
	noName.Name = ""
	return app.ExtractCellValue(col, td, noName) != r.Name
}

// w45StructLess is the shape the disk cache restores and a degraded fetch
// leaves behind: the row's ID, Name and Fields survive, the SDK struct does
// not.
func w45StructLess(r resource.Resource) resource.Resource {
	out := r
	out.RawStruct = nil
	return out
}

// TestW45_NoColumnButTheIdentityColumnShowsTheRowName walks every registered
// type, parent and child, and every column of its resolved list view against a
// struct-less row. Only the identity column may render the row's name; any
// other column doing so is showing a value it does not have.
//
// Parent rows are the real demo fixtures drained through each type's own
// Wave-1 fetcher with RawStruct stripped. No child type has a Wave-1 fetcher
// (they are reached through ChildFetcher with a parent ID), so a child is
// walked against the degraded shape its own stub rows have: ID and Name, no
// Fields, no struct.
func TestW45_NoColumnButTheIdentityColumnShowsTheRowName(t *testing.T) {
	clients := demo.NewServiceClients()
	var offenders []string
	seen := map[string]bool{}

	check := func(td resource.ResourceTypeDef, r resource.Resource) {
		cols := w45Columns(td)
		if len(cols) == 0 {
			return
		}
		identity := app.IdentityColumnIndex(cols, &td)
		for i, col := range cols {
			if i == identity {
				continue
			}
			if !w45NameLeak(col, &td, r) {
				continue
			}
			key := fmt.Sprintf("%s / col[%d] %q (key=%q path=%q)", td.ShortName, i, col.Title, col.Key, col.Path)
			if seen[key] {
				continue
			}
			seen[key] = true
			offenders = append(offenders, key)
		}
	}

	for _, td := range resource.AllResourceTypes() {
		rows, ok := unit.DrainFixtures(t, td, clients)
		if !ok || len(rows) == 0 {
			t.Errorf("%s: no demo rows drained — the gate cannot see this type", td.ShortName)
			continue
		}
		for _, r := range rows {
			check(td, w45StructLess(r))
		}
	}

	for _, td := range resource.AllChildTypesForTest() {
		check(td, resource.Resource{
			ID:   td.ShortName + "-1",
			Name: "acme-" + td.ShortName + "-1",
			Type: td.ShortName,
		})
	}

	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("column renders the row's own name on a struct-less row: %s", o)
	}
}

// TestW45_IdentityColumnElectionIgnoresPathSubstrings pins that no registered
// type's identity column is elected by a path containing "Name" or
// "Identifier". That step cannot tell "the column that names the row" from
// "a column that happens to point at a name-shaped field of something else",
// which is the same defect one level up from the cell fallback. Masking the
// substrings in a copy of the columns leaves every other election step
// untouched, so a differing index means that step, and only that step, decided.
func TestW45_IdentityColumnElectionIgnoresPathSubstrings(t *testing.T) {
	mask := func(cols []app.ColumnDef) []app.ColumnDef {
		out := make([]app.ColumnDef, len(cols))
		copy(out, cols)
		for i := range out {
			out[i].Path = strings.ReplaceAll(out[i].Path, "Name", "Nm")
			out[i].Path = strings.ReplaceAll(out[i].Path, "Identifier", "Ident")
		}
		return out
	}

	tds := append(resource.AllResourceTypes(), resource.AllChildTypesForTest()...)
	for _, td := range tds {
		cols := w45Columns(td)
		if len(cols) == 0 {
			continue
		}
		got := app.IdentityColumnIndex(cols, &td)
		want := app.IdentityColumnIndex(mask(cols), &td)
		if got != want {
			t.Errorf("%s: identity column elected by a path substring: index %d (%q path=%q) with the step, index %d (%q) without it",
				td.ShortName, got, cols[got].Title, cols[got].Path, want, cols[want].Title)
		}
	}
}

// w45DemoRow drains one type's demo fixtures and returns the row with the
// given ID, struct-less — the shape the disk cache restores.
func w45DemoRow(t *testing.T, shortName, id string) (*resource.ResourceTypeDef, resource.Resource) {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("%s is not a registered type", shortName)
	}
	rows, ok := unit.DrainFixtures(t, *td, demo.NewServiceClients())
	if !ok {
		t.Fatalf("%s has no Wave-1 fetcher", shortName)
	}
	for _, r := range rows {
		if r.ID == id {
			return td, w45StructLess(r)
		}
	}
	t.Fatalf("%s has no demo row %q", shortName, id)
	return nil, resource.Resource{}
}

// w45ColumnByTitle returns one resolved list column of a type by its heading.
func w45ColumnByTitle(t *testing.T, td resource.ResourceTypeDef, title string) app.ColumnDef {
	t.Helper()
	for _, c := range w45Columns(td) {
		if c.Title == title {
			return c
		}
	}
	t.Fatalf("%s has no column titled %q", td.ShortName, title)
	return app.ColumnDef{}
}

// TestW45_NameFallbackOnlyFillsTheIdentityColumn pins the cell extractor
// branch by branch. Each case is a column that the old cascade matched on a
// "name" substring of its key, its title or its path; on a struct-less row
// none of them can answer from a path, and none of them is the column that
// names the row, so the honest cell is empty. The identity column is the one
// place the row's name belongs, and it must still be filled — a blank first
// column would make a warm-cache list unreadable.
func TestW45_NameFallbackOnlyFillsTheIdentityColumn(t *testing.T) {
	alarmTD, alarmRow := w45DemoRow(t, "alarm", "api-high-error-rate")
	redshiftTD, redshiftRow := w45DemoRow(t, "redshift", "acme-warehouse")
	ngTD, ngRow := w45DemoRow(t, "ng", "general-pool")

	stagesTD := resource.GetChildType("pipeline_stages")
	if stagesTD == nil {
		t.Fatalf("pipeline_stages is not a registered child type")
	}
	// A CodePipeline stage row whose action name never arrived: the stage is
	// named, the action is not.
	stageRow := resource.Resource{
		ID:   "acme-delivery/Deploy",
		Name: "Deploy",
		Type: "pipeline_stages",
		Fields: map[string]string{
			"stage_name":   "Deploy",
			"stage_status": "Succeeded",
		},
	}

	cases := []struct {
		name string
		td   *resource.ResourceTypeDef
		col  app.ColumnDef
		row  resource.Resource
		want string
	}{
		{
			// Path branch: "MetricName" is the name of the metric the alarm
			// watches, never the alarm's own name.
			name: "path substring — alarm Metric",
			td:   alarmTD,
			col:  w45ColumnByTitle(t, *alarmTD, "Metric"),
			row:  alarmRow,
			want: "",
		},
		{
			// Path branch: "DBName" is the database inside the cluster.
			name: "path substring — redshift Database",
			td:   redshiftTD,
			col:  w45ColumnByTitle(t, *redshiftTD, "Database"),
			row:  redshiftRow,
			want: "",
		},
		{
			// Key branch: "action_name" is the pipeline action, and this row
			// carries no value for it.
			name: "key substring — pipeline_stages Action",
			td:   stagesTD,
			col:  w45ColumnByTitle(t, *stagesTD, "Action"),
			row:  stageRow,
			want: "",
		},
		{
			// Title branch. No shipped column carries "name" in its title
			// alone, so the branch is pinned against a column shaped like one
			// that could be added; the cascade must not have a third way back
			// to r.Name.
			name: "title substring — a name-titled column with no key and no path",
			td:   alarmTD,
			col:  app.ColumnDef{Title: "Alarm Display Name", Width: 24},
			row:  alarmRow,
			want: "",
		},
		{
			// The identity column of a type whose name lives nowhere but
			// r.Name once the struct is gone: the fallback exists for this.
			name: "identity column — alarm Alarm Name",
			td:   alarmTD,
			col:  w45ColumnByTitle(t, *alarmTD, "Alarm Name"),
			row:  alarmRow,
			want: "api-high-error-rate",
		},
		{
			// A key-substring column that the fetcher does write: keying the
			// fallback on the identity column must not blank a cell that has
			// a real value.
			name: "key substring with a value — ng Cluster",
			td:   ngTD,
			col:  w45ColumnByTitle(t, *ngTD, "Cluster"),
			row:  ngRow,
			want: "acme-prod",
		},
		{
			name: "identity column — ng Node Group",
			td:   ngTD,
			col:  w45ColumnByTitle(t, *ngTD, "Node Group"),
			row:  ngRow,
			want: "general-pool",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := app.ExtractCellValue(tc.col, tc.td, tc.row)
			if got != tc.want {
				t.Errorf("ExtractCellValue(%q, key=%q, path=%q) on struct-less row %q = %q, want %q",
					tc.col.Title, tc.col.Key, tc.col.Path, tc.row.ID, got, tc.want)
			}
		})
	}
}
