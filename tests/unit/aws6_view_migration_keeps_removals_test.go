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
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
)

// lambdaViewName is the view file the Codex scenario names.
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

// TestMigrationKeepsARemovedColumnOnTheWholesalePath pins the path Codex names:
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
func TestMigrationKeepsARemovedColumnOnASecondStart(t *testing.T) {
	cols, detail := lambdaDefaultsWithout(t, "Runtime")
	dir := tui5SeedOlderView(t, lambdaViewName, cols, detail)

	for pass := 1; pass <= 2; pass++ {
		if err := config.EnsureViewsDir(dir); err != nil {
			t.Fatalf("EnsureViewsDir pass %d: %v", pass, err)
		}
		// The operator deletes it again after the first pass, exactly as they
		// would; the second pass must leave the deletion alone.
		if pass == 1 {
			removeColumnFromViewFile(t, dir, lambdaViewName, "Runtime")
		}
	}

	titles := tui5ColumnTitles(t, dir, lambdaViewName)
	if slices.Contains(titles, "Runtime") {
		t.Errorf("Runtime came back on the second start (columns now %v); a file at the current stamp "+
			"with a column deleted is settled and later starts leave it alone", titles)
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

// removeColumnFromViewFile deletes one column from a view file on disk, the way
// an operator editing the YAML does.
func removeColumnFromViewFile(t *testing.T, dir, name, title string) {
	t.Helper()
	path := filepath.Join(dir, name+".yaml")
	data, err := os.ReadFile(path) //nolint:gosec // dir is this test's own TempDir
	if err != nil {
		t.Fatalf("reading %s.yaml: %v", name, err)
	}
	vd, err := config.ParseSingle(data)
	if err != nil {
		t.Fatalf("parsing %s.yaml: %v", name, err)
	}
	var kept []config.ListColumn
	for _, c := range vd.List {
		if c.Title != title {
			kept = append(kept, c)
		}
	}
	if len(kept) == len(vd.List) {
		t.Fatalf("%s.yaml has no %q column to delete", name, title)
	}
	vd.List = kept
	if err := os.WriteFile(path, config.GenerateViewYAML(*vd), 0600); err != nil {
		t.Fatalf("writing %s.yaml: %v", name, err)
	}
}
