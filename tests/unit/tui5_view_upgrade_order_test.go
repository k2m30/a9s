// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

// tui5_view_upgrade_order_test.go — the view-file upgrade could add a column
// and re-source one, but never reorder, so a corrected default column order
// reached new installations only. These pin the rule that fixes it: a file
// whose every column is still the source and width its stamp's build
// generated has never been touched, and takes this build's order wholesale;
// a file the operator has edited keeps the order it has.

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
)

// tui5SeedOlderView writes dir/<name>.yaml holding cols in the given order,
// stamped one build older than this one, and returns the directory.
func tui5SeedOlderView(t *testing.T, name string, cols []config.ListColumn, detail []config.DetailField) string {
	t.Helper()
	dir := t.TempDir()
	body := string(config.GenerateViewYAML(config.ViewDef{List: cols, Detail: detail}))
	older := strings.Replace(body,
		"generated: "+strconv.Itoa(config.GeneratedViewsVersion),
		"generated: "+strconv.Itoa(config.GeneratedViewsVersion-1), 1)
	if older == body {
		t.Fatalf("could not age the stamp in the generated file:\n%s", body)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(older), 0600); err != nil {
		t.Fatalf("writing %s.yaml: %v", name, err)
	}
	return dir
}

// tui5ColumnTitles lists the column titles a view file on disk holds, in order.
func tui5ColumnTitles(t *testing.T, dir, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name+".yaml")) //nolint:gosec // dir is this test's own TempDir
	if err != nil {
		t.Fatalf("reading %s.yaml: %v", name, err)
	}
	vd, err := config.ParseSingle(data)
	if err != nil {
		t.Fatalf("parsing %s.yaml: %v\n%s", name, err, data)
	}
	titles := make([]string, len(vd.List))
	for i, c := range vd.List {
		titles[i] = c.Title
	}
	return titles
}

// tui5ReversedColumns returns cols in reverse order.
func tui5ReversedColumns(cols []config.ListColumn) []config.ListColumn {
	out := make([]config.ListColumn, len(cols))
	for i, c := range cols {
		out[len(cols)-1-i] = c
	}
	return out
}

// tui5DefaultView is the built-in view for a type whose columns no migration
// table entry has ever re-sourced, so nothing but the order rule is in play.
const tui5UpgradeView = "nat"

func tui5DefaultView(t *testing.T) config.ViewDef {
	t.Helper()
	vd, ok := config.DefaultConfig().Views[tui5UpgradeView]
	if !ok {
		t.Fatalf("built-in views have no %q entry", tui5UpgradeView)
	}
	return vd
}

// TestViewUpgrade_UneditedFileTakesTheBuildsOrder seeds a file that an older
// build generated — every column still the source and width that build wrote,
// only the order differs from what this build generates — and asserts the
// upgrade puts the columns in this build's order.
func TestViewUpgrade_UneditedFileTakesTheBuildsOrder(t *testing.T) {
	def := tui5DefaultView(t)
	dir := tui5SeedOlderView(t, tui5UpgradeView, tui5ReversedColumns(def.List), def.Detail)

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	want := make([]string, len(def.List))
	for i, c := range def.List {
		want[i] = c.Title
	}
	got := tui5ColumnTitles(t, dir, tui5UpgradeView)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("unedited file kept its own order after the upgrade\n got: %v\nwant: %v", got, want)
	}
}

// TestViewUpgrade_EditedFileKeepsItsOrder seeds the same older file with one
// operator edit — a width they set themselves — and asserts the upgrade
// leaves the order they have alone.
func TestViewUpgrade_EditedFileKeepsItsOrder(t *testing.T) {
	def := tui5DefaultView(t)
	onDisk := tui5ReversedColumns(def.List)
	onDisk[0].Width += 7 // the operator widened a column
	dir := tui5SeedOlderView(t, tui5UpgradeView, onDisk, def.Detail)

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	want := make([]string, len(onDisk))
	for i, c := range onDisk {
		want[i] = c.Title
	}
	got := tui5ColumnTitles(t, dir, tui5UpgradeView)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("an edited file was reordered by the upgrade\n got: %v\nwant: %v", got, want)
	}
}
