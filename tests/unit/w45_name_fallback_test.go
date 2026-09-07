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
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/k2m30/a9s/v3/core/app"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/tests/unit"
)

// w45Columns resolves one type's list columns through the shared cascade
// core/app's resolveListColumnsForBuild and core/runtime's resolveSaveColumns
// both delegate to. A nil vc is the column set a fresh install renders; a
// loaded one is what a user with view files on disk sees.
//
// The election is marked here because core/app's resolver marks it: per the
// cols spec row 2, the identity column is elected once over the resolved set
// and travels on ColumnDef.Identity, and ExtractCellValue reads that flag
// instead of re-electing from the type's built-in set. A column set assembled
// without the election is a set no resolver produces, so asserting the
// extractor's behaviour on one would test nothing. The old assertion (the
// extractor finds the identity column from the typeDef alone) is not to be
// restored: it is what made a loaded view file's identity cell render blank.
func w45Columns(vc *config.ViewsConfig, td resource.ResourceTypeDef) []app.ColumnDef {
	lcs := resource.ResolveListColumnCascade(vc, td.ShortName, &td)
	if len(lcs) == 0 {
		return nil
	}
	cols := make([]app.ColumnDef, len(lcs))
	for i, lc := range lcs {
		cols[i] = app.ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path, Humanize: lc.Humanize}
	}
	cols[app.IdentityColumnIndex(cols, &td)].Identity = true
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
		cols := w45Columns(nil, td)
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
		cols := w45Columns(nil, td)
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
	for _, c := range w45Columns(nil, td) {
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
	codeartifactTD, codeartifactRow := w45DemoRow(t, "codeartifact", "acme-npm")
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
			// Path branch: "DomainName" is the CodeArtifact domain the
			// repository belongs to, not the repository.
			name: "path substring — codeartifact Domain",
			td:   codeartifactTD,
			col:  w45ColumnByTitle(t, *codeartifactTD, "Domain"),
			row:  codeartifactRow,
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

// w45LoadedViewConfig loads the shipped per-resource view files the way the
// app loads a user's own: through config.LoadFromDirs. The built-in defaults
// and a loaded view file are different shapes — the YAML keys a column by its
// title and stores only what differs from the default — so a rule that reads
// a column's identity off the ColumnDef must hold for both.
func w45LoadedViewConfig(t *testing.T) *config.ViewsConfig {
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

// TestW45_IdentityColumnSurvivesALoadedViewFile is the round-trip half of the
// fix. The extractor decides which column names the row from the built-in
// column set, but what it is handed at render time came through the session's
// view config; if the two disagree about the identity column, the first column
// of a warm-cache list goes blank — worse than the wrong value this task
// removes. Every shipped view file is loaded and compared against the built-in
// election for the same type, and the identity cell is then rendered on a real
// struct-less demo row.
func TestW45_IdentityColumnSurvivesALoadedViewFile(t *testing.T) {
	vc := w45LoadedViewConfig(t)
	clients := demo.NewServiceClients()

	for _, td := range resource.AllResourceTypes() {
		if _, ok := vc.Views[td.ShortName]; !ok {
			continue
		}
		loaded := w45Columns(vc, td)
		builtin := w45Columns(nil, td)
		if len(loaded) == 0 || len(builtin) == 0 {
			t.Errorf("%s: no columns resolved (loaded=%d builtin=%d)", td.ShortName, len(loaded), len(builtin))
			continue
		}
		li, bi := app.IdentityColumnIndex(loaded, &td), app.IdentityColumnIndex(builtin, &td)
		if loaded[li].Title != builtin[bi].Title {
			t.Logf("%s: identity column differs between a loaded view file (%q at %d) and the built-in set (%q at %d)",
				td.ShortName, loaded[li].Title, li, builtin[bi].Title, bi)
		}

		rows, ok := unit.DrainFixtures(t, td, clients)
		if !ok || len(rows) == 0 {
			continue
		}
		for _, raw := range rows {
			r := w45StructLess(raw)
			if r.Name == "" {
				continue
			}
			if got := app.ExtractCellValue(loaded[li], &td, r); got == "" {
				t.Errorf("%s: identity column %q renders blank on the struct-less row %q when the column came from a view file",
					td.ShortName, loaded[li].Title, r.ID)
			}
			for i, col := range loaded {
				if i == li {
					continue
				}
				if w45NameLeak(col, &td, r) {
					t.Errorf("%s: column %q (loaded view) renders the row's own name on the struct-less row %q", td.ShortName, col.Title, r.ID)
				}
			}
		}
	}
}

// TestW45_IdentityTitleIsUniqueWithinAType pins the assumption the extractor
// now rests on: it identifies the row-naming column by comparing titles, so a
// type with two columns under one title would have a second column inheriting
// the row's name, and a type whose identity column has no title at all would
// hand the name to every title-less column beside it.
func TestW45_IdentityTitleIsUniqueWithinAType(t *testing.T) {
	tds := append(resource.AllResourceTypes(), resource.AllChildTypesForTest()...)
	for _, td := range tds {
		cols := w45Columns(nil, td)
		if len(cols) == 0 {
			continue
		}
		identity := cols[app.IdentityColumnIndex(cols, &td)]
		if identity.Title == "" {
			t.Errorf("%s: the identity column has no title, so every title-less column beside it matches it", td.ShortName)
			continue
		}
		for i, c := range cols {
			if i != app.IdentityColumnIndex(cols, &td) && c.Title == identity.Title {
				t.Errorf("%s: column %d also carries the identity title %q", td.ShortName, i, identity.Title)
			}
		}
	}
}

// TestW45_LiveRowWithANilNameFieldRendersBlank covers the row the struct-less
// pins cannot reach: the struct is present, so the path branch runs, but the
// field it points at is nil. The extraction yields nothing and the cascade
// falls through to the same fallback — a live alarm whose MetricName the API
// omitted must not print the alarm's own name under Metric.
func TestW45_LiveRowWithANilNameFieldRendersBlank(t *testing.T) {
	td := resource.FindResourceType("alarm")
	if td == nil {
		t.Fatalf("alarm is not a registered type")
	}
	// The alarm fetcher stores the SDK value as RawStruct; MetricName is a
	// pointer and DescribeAlarms leaves it unset for a composite alarm.
	live := resource.Resource{
		ID:   "api-high-error-rate",
		Name: "api-high-error-rate",
		Type: "alarm",
		Fields: map[string]string{
			"alarm_name": "api-high-error-rate",
			"state":      "OK",
		},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:  aws.String("api-high-error-rate"),
			StateValue: cwtypes.StateValueOk,
		},
	}

	metric := w45ColumnByTitle(t, *td, "Metric")
	if got := app.ExtractCellValue(metric, td, live); got != "" {
		t.Errorf("Metric on a live alarm with no MetricName = %q, want %q", got, "")
	}
	alarmName := w45ColumnByTitle(t, *td, "Alarm Name")
	if got := app.ExtractCellValue(alarmName, td, live); got != "api-high-error-rate" {
		t.Errorf("Alarm Name on the same row = %q, want %q", got, "api-high-error-rate")
	}
}

// TestW45_RenderedListPutsTheNameInTheNameColumn drives the fix through the
// app's own list-body path rather than the extractor alone. The rendered
// ViewState is what both the TUI and the web draw from, and it is where the
// two user-visible consequences land: the marker glyph is prepended to the
// MarkerCol cell, and the cells are the values a reader sees. ec2's marker
// column moved from Status to Name with the election fix, and no golden covers
// that move because no row in the golden scenarios carries a Decorator.
func TestW45_RenderedListPutsTheNameInTheNameColumn(t *testing.T) {
	clients := demo.NewServiceClients()

	for _, tc := range []struct {
		shortName    string
		markerCol    int
		identityCell string
		blankColumn  string
	}{
		{"ec2", 0, "Name", ""},
		{"alarm", 0, "Alarm Name", "Metric"},
	} {
		t.Run(tc.shortName, func(t *testing.T) {
			td := resource.FindResourceType(tc.shortName)
			if td == nil {
				t.Fatalf("%s is not a registered type", tc.shortName)
			}
			rows, ok := unit.DrainFixtures(t, *td, clients)
			if !ok || len(rows) == 0 {
				t.Fatalf("%s: no demo rows", tc.shortName)
			}
			stripped := make([]resource.Resource, len(rows))
			for i, r := range rows {
				stripped[i] = w45StructLess(r)
			}

			c := newTestController(t)
			c.Apply(app.Action{Kind: app.ActionCommand, Arg: tc.shortName})
			c.ApplyResourcesLoaded(tc.shortName, stripped, nil, false)
			lb := c.Snapshot().Body.List
			if lb == nil {
				t.Fatalf("%s: nil list body", tc.shortName)
			}
			if lb.MarkerCol != tc.markerCol {
				t.Errorf("%s: MarkerCol = %d (%q), want %d (%q)",
					tc.shortName, lb.MarkerCol, lb.Columns[lb.MarkerCol].Title, tc.markerCol, tc.identityCell)
			}
			if lb.Columns[lb.MarkerCol].Title != tc.identityCell {
				t.Errorf("%s: marker column title = %q, want %q", tc.shortName, lb.Columns[lb.MarkerCol].Title, tc.identityCell)
			}
			if len(lb.Rows) == 0 {
				t.Fatalf("%s: no rendered rows", tc.shortName)
			}

			byTitle := map[string]int{}
			for i, col := range lb.Columns {
				byTitle[col.Title] = i
			}
			for i, row := range lb.Rows {
				name := stripped[i].Name
				if got := row.Cells[lb.MarkerCol]; got != name {
					t.Errorf("%s row %d: the column that names the row shows %q, want %q", tc.shortName, i, got, name)
				}
				if tc.blankColumn == "" {
					continue
				}
				j, ok := byTitle[tc.blankColumn]
				if !ok {
					t.Fatalf("%s: no rendered column %q", tc.shortName, tc.blankColumn)
				}
				if got := row.Cells[j]; got == name {
					t.Errorf("%s row %d: %q shows the row's own name %q", tc.shortName, i, tc.blankColumn, got)
				}
			}
		})
	}
}

// TestW45_ArchitectureDocDescribesTheLiveElection pins the identity-column
// paragraph of docs/architecture.md against the code it documents. The doc is
// what a reader consults before touching this cascade, so a step it lists that
// the code no longer runs, or a function name the tree no longer defines,
// sends the next reader to restore a defect.
func TestW45_ArchitectureDocDescribesTheLiveElection(t *testing.T) {
	data, err := os.ReadFile("../../docs/architecture.md")
	if err != nil {
		t.Fatalf("reading docs/architecture.md: %v", err)
	}
	doc := string(data)

	for _, stale := range []string{
		"resolveIdentityColumn",
		"`Path` contains `\"Name\"` or `\"Identifier\"`",
		"5-step cascade",
	} {
		if strings.Contains(doc, stale) {
			t.Errorf("docs/architecture.md still describes %q — the election has four steps and lives in app.IdentityColumnIndex", stale)
		}
	}
	if !strings.Contains(doc, "IdentityColumnIndex") {
		t.Errorf("docs/architecture.md never names IdentityColumnIndex, the function that elects the identity column")
	}
}
