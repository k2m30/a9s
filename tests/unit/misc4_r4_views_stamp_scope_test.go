package unit

// misc4_r4_views_stamp_scope_test.go — misc4 row 4.
//
// GeneratedViewsVersion's own doc named removes and renames among the reasons
// to bump it, and the merge can do neither: it adds a built-in title the file
// on disk does not carry, and it re-sources a column still reading what an
// older build generated. Nothing deletes a column, and a renamed built-in
// reaches the merge as a title the file has never heard of — so the operator
// ends up with both.
//
// These pin the merge's actual reach, so the corrected doc stays true.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
)

// misc4WriteStaleView writes a view file stamped one version behind this
// build, carrying exactly the columns given.
func misc4WriteStaleView(t *testing.T, dir, name string, cols []config.ListColumn) string {
	t.Helper()
	body := config.GenerateViewYAML(config.ViewDef{List: cols})
	stale := strings.Replace(string(body),
		"generated: "+strconv.Itoa(config.GeneratedViewsVersion),
		"generated: "+strconv.Itoa(config.GeneratedViewsVersion-1), 1)
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(stale), 0600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// TestViewsMergeNeverRemovesAColumn pins that a column no built-in set
// declares survives the merge, so "remove" is not something bumping the stamp
// can deliver.
func TestViewsMergeNeverRemovesAColumn(t *testing.T) {
	dir := t.TempDir()
	path := misc4WriteStaleView(t, dir, "sns", []config.ListColumn{
		{Title: "Retired Column", Path: "SomethingGone", Width: 12},
	})
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // path is this test's temp dir
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), "Retired Column") {
		t.Errorf("the merge dropped a column the built-in set no longer declares:\n%s", got)
	}
}

// TestViewsMergeLeavesTheOldTitleBehindOnARename pins that renaming a built-in
// column reaches the merge as an addition: the operator keeps both titles, so
// "rename" is not something bumping the stamp can deliver either.
func TestViewsMergeLeavesTheOldTitleBehindOnARename(t *testing.T) {
	dir := t.TempDir()
	def, ok := config.DefaultConfig().Views["sns"]
	if !ok || len(def.List) == 0 {
		t.Fatal("sns has no built-in list columns")
	}
	first := def.List[0]
	path := misc4WriteStaleView(t, dir, "sns", []config.ListColumn{
		{Title: "Former " + first.Title, Path: first.Path, Width: first.Width},
	})
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}
	got, err := os.ReadFile(path) //nolint:gosec // path is this test's temp dir
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(got), "Former "+first.Title) {
		t.Errorf("the old title vanished; the merge is doing more than adding:\n%s", got)
	}
	if !strings.Contains(string(got), first.Title) {
		t.Errorf("the built-in title was not added:\n%s", got)
	}
}
