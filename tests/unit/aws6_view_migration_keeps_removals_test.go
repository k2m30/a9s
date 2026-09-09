package unit

// aws6_view_migration_keeps_removals_test.go — a migration inserts what its
// version introduced, and nothing else.
//
// A view file is the operator's. Deleting a column from it is a decision, and
// the only record of that decision is the column's absence. A migration that
// re-adds every built-in the file does not carry cannot tell "removed on
// purpose" from "introduced since", so it reverses the decision — and does it
// again on the next start, so the operator cannot win by deleting it twice.
//
// Both paths that write columns are pinned. The wholesale replacement fires
// when the file looks untouched, and the per-column insertion fires when some
// other edit stops it.

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
)

// lambdaViewName is the view file under test.
const lambdaViewName = "lambda"

// lambdaDefaultsWithout returns the built-in lambda columns with the named
// title removed, and the detail section unchanged.
func lambdaDefaultsWithout(t *testing.T, title string) ([]config.ListColumn, []config.DetailField) {
	t.Helper()
	def, ok := config.DefaultConfig().Views[lambdaViewName]
	if !ok {
		t.Fatalf("built-in views have no %q entry", lambdaViewName)
	}
	var kept []config.ListColumn
	removed := false
	for _, c := range def.List {
		if c.Title == title {
			removed = true
			continue
		}
		kept = append(kept, c)
	}
	if !removed {
		t.Fatalf("the built-in %s view has no %q column, so this scenario no longer describes it",
			lambdaViewName, title)
	}
	return kept, def.Detail
}

// TestMigrationKeepsARemovedColumnOnTheWholesalePath pins the wholesale path:
// every column that REMAINS matches what a build generated, so the file reads as
// untouched and is replaced with the current defaults, restoring the one the
// operator deleted.
func TestMigrationKeepsARemovedColumnOnTheWholesalePath(t *testing.T) {
	cols, detail := lambdaDefaultsWithout(t, "Runtime")
	dir := tui5SeedOlderView(t, lambdaViewName, cols, detail)

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	titles := tui5ColumnTitles(t, dir, lambdaViewName)
	if slices.Contains(titles, "Runtime") {
		t.Errorf("migrating a v%d %s view with Runtime deleted put it back (columns now %v); "+
			"the file is the operator's and an absent column is their decision, so a migration "+
			"inserts only what its own version introduced",
			config.GeneratedViewsVersion-1, lambdaViewName, titles)
	}
}

// TestMigrationKeepsARemovedColumnOnTheInsertionPath pins the other path. A
// second customization — a width the operator changed — stops the wholesale
// replacement, and the per-column insertion loop restores the deleted column
// anyway.
func TestMigrationKeepsARemovedColumnOnTheInsertionPath(t *testing.T) {
	cols, detail := lambdaDefaultsWithout(t, "Runtime")
	if len(cols) == 0 {
		t.Fatal("the lambda view has no other column to customize")
	}
	cols[0].Width += 7 // the operator's own width, on a column they kept

	dir := tui5SeedOlderView(t, lambdaViewName, cols, detail)
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	titles := tui5ColumnTitles(t, dir, lambdaViewName)
	if slices.Contains(titles, "Runtime") {
		t.Errorf("migrating a v%d %s view with Runtime deleted and another column's width changed "+
			"put Runtime back (columns now %v); the insertion path must add only columns introduced "+
			"after the file's stamp", config.GeneratedViewsVersion-1, lambdaViewName, titles)
	}

	// The operator's width is theirs and survives: a migration that kept the
	// removal by refusing to touch the file at all would pass the check above
	// while also stranding every real correction.
	got := tui5ColumnTitled(t, dir, lambdaViewName, cols[0].Title)
	if got.Width != cols[0].Width {
		t.Errorf("the operator's width on %q became %d, want %d — the removal is kept by inserting "+
			"less, not by leaving the file alone", cols[0].Title, got.Width, cols[0].Width)
	}
}

// TestMigrationKeepsARemovedColumnOnASecondStart pins persistence. A migration
// that restores the column once is an annoyance; one that restores it on every
// start is a file the operator cannot edit.
//
// Two runs, no edit between them: the first must keep the deletion and the
// second must leave the settled file alone. Deleting the column again between
// the runs would have made the pin passable only against the defect, since a
// correct first pass leaves nothing to delete.
func TestMigrationKeepsARemovedColumnOnASecondStart(t *testing.T) {
	cols, detail := lambdaDefaultsWithout(t, "Runtime")
	dir := tui5SeedOlderView(t, lambdaViewName, cols, detail)

	for pass := 1; pass <= 2; pass++ {
		if err := config.EnsureViewsDir(dir); err != nil {
			t.Fatalf("EnsureViewsDir pass %d: %v", pass, err)
		}
		titles := tui5ColumnTitles(t, dir, lambdaViewName)
		if slices.Contains(titles, "Runtime") {
			t.Fatalf("Runtime came back on start %d (columns now %v); an absent column is the "+
				"operator's decision and every later start reads it the same way", pass, titles)
		}
	}
}

// TestMigrationStillInsertsAColumnItsVersionIntroduced is the negative case.
// Keeping removals is only correct if the migration still delivers what it was
// written to deliver: a column this build added, which the operator has never
// seen and so cannot have decided against.
func TestMigrationStillInsertsAColumnItsVersionIntroduced(t *testing.T) {
	def, ok := config.DefaultConfig().Views[lambdaViewName]
	if !ok {
		t.Fatalf("built-in views have no %q entry", lambdaViewName)
	}
	// A file two versions old that predates the current default set entirely:
	// its columns are all still present, so nothing in it records a decision.
	dir := tui5SeedOlderView(t, lambdaViewName, def.List, def.Detail)
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	titles := tui5ColumnTitles(t, dir, lambdaViewName)
	for _, c := range def.List {
		if !slices.Contains(titles, c.Title) {
			t.Errorf("migration dropped the built-in column %q (columns now %v); keeping a REMOVAL "+
				"must not become keeping the file frozen", c.Title, titles)
		}
	}
}

// TestDefaultColumnsAreTheBaselinePlusTheRecordedAdditions is the conformance
// gate on core/config/ensure_views.go's per-version additions table.
//
// That table has to be extended by hand whenever a column joins the built-in
// views, and nothing today notices when it is forgotten. The cost is silent and
// lands on existing installations only: a column with no entry is one the
// migration cannot tell from a column the operator deleted, so it reaches new
// installations and never reaches anyone who has run a9s before.
//
// The check restates the invariant as a migration. Seed each view as the oldest
// migratable build wrote it, run the real EnsureViewsDir, and the file must come
// out carrying exactly today's built-in columns. A column added to the defaults
// without a table entry is not inserted, so it is missing here and named.
func TestDefaultColumnsAreTheBaselinePlusTheRecordedAdditions(t *testing.T) {
	for name, def := range config.DefaultConfig().Views {
		baseline, recorded := viewColumnsAtOldestMigratableVersion[name]
		if !recorded {
			if len(def.List) == 0 {
				continue
			}
			t.Errorf("the built-in %s view has columns and no baseline; add its titles to "+
				"viewColumnsAtOldestMigratableVersion", name)
			continue
		}

		t.Run(name, func(t *testing.T) {
			dir := seedViewAtOldestMigratableStamp(t, name, baseline, def)
			if err := config.EnsureViewsDir(dir); err != nil {
				t.Fatalf("EnsureViewsDir: %v", err)
			}

			got := tui5ColumnTitles(t, dir, name)
			want := make([]string, 0, len(def.List))
			for _, c := range def.List {
				want = append(want, c.Title)
			}
			if slices.Equal(got, want) {
				return
			}
			for _, w := range want {
				if !slices.Contains(got, w) {
					t.Errorf("the built-in column %q never reaches a file an older build wrote; "+
						"it is missing from viewColumnAdditions in core/config/ensure_views.go, so the "+
						"migration cannot tell it from a column the operator deleted and it lands on new "+
						"installations only", w)
				}
			}
			for _, g := range got {
				if !slices.Contains(want, g) {
					t.Errorf("the migrated file carries %q, which the built-in %s view no longer "+
						"declares; drop it from viewColumnsAtOldestMigratableVersion so the baseline "+
						"stays the set that build actually wrote", g, name)
				}
			}
			if len(t.Name()) > 0 && slices.Equal(sortedCopy(got), sortedCopy(want)) {
				t.Errorf("migrated order %v, want %v", got, want)
			}
		})
	}
}

// sortedCopy returns in sorted, without touching it.
func sortedCopy(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)
	return out
}

// seedViewAtOldestMigratableStamp writes name.yaml carrying the given column
// titles at stamp 1 — the oldest stamp a migration will still act on, since 0
// means "predates the table" and every built-in is new to such a file.
func seedViewAtOldestMigratableStamp(t *testing.T, name string, titles []string, def config.ViewDef) string {
	t.Helper()
	byTitle := make(map[string]config.ListColumn, len(def.List))
	for _, c := range def.List {
		byTitle[c.Title] = c
	}
	cols := make([]config.ListColumn, 0, len(titles))
	for _, title := range titles {
		c, ok := byTitle[title]
		if !ok {
			// A baseline title the defaults no longer declare: keep it, with
			// only the title the old build wrote, so the check below reports it.
			c = config.ListColumn{Title: title, Width: 12}
		}
		cols = append(cols, c)
	}

	dir := t.TempDir()
	body := string(config.GenerateViewYAML(config.ViewDef{List: cols, Detail: def.Detail}))
	older := strings.Replace(body,
		"generated: "+strconv.Itoa(config.GeneratedViewsVersion), "generated: 1", 1)
	if older == body {
		t.Fatalf("could not age the stamp in the generated %s file:\n%s", name, body)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(older), 0600); err != nil {
		t.Fatalf("writing %s.yaml: %v", name, err)
	}
	return dir
}
