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

// ---------------------------------------------------------------------------
// The migration table generalises. An entry names a title and the whole column
// the previous build generated for it; an on-disk column still matching that
// takes the current default column wholesale — source, width, and whatever a
// later build adds — so a correction reaches an installation that already
// exists rather than only a fresh one.
//
// INVERTED for aws6 rows 4-6 (one humanize owner). These two tests used to
// demonstrate the carry on ecs-task's Stop Code column, whose correction WAS
// the humanize flag. That flag is no longer a column field at all — it is the
// type's own declaration (ResourceTypeDef.HumanizeFields) — so there is
// nothing left on that column for the carry to deliver, and the assertions
// naming it are not to be restored. The rule they pin is unchanged and is now
// demonstrated on logs' Retention column, whose correction is live: an older
// build sourced it from the RawStruct path RetentionInDays at width 10, and
// this build reads Fields["retention"] at width 12.
// ---------------------------------------------------------------------------

// tui5ColumnTitled returns the on-disk column with the given title.
func tui5ColumnTitled(t *testing.T, dir, name, title string) config.ListColumn {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name+".yaml")) //nolint:gosec // dir is this test's own TempDir
	if err != nil {
		t.Fatalf("reading %s.yaml: %v", name, err)
	}
	vd, err := config.ParseSingle(data)
	if err != nil {
		t.Fatalf("parsing %s.yaml: %v\n%s", name, err, data)
	}
	for _, c := range vd.List {
		if c.Title == title {
			return c
		}
	}
	t.Fatalf("%s.yaml has no column titled %q", name, title)
	return config.ListColumn{}
}

// tui5PreviousBuildColumn is the logs Retention column exactly as the build
// before the source correction generated it: read off the RawStruct path at
// width 10, where this build reads the fetcher's key at width 12.
var tui5PreviousBuildColumn = config.ListColumn{Title: "Retention", Path: "RetentionInDays", Width: 10}

// TestViewUpgrade_StillGeneratedColumnTakesTheCorrection seeds a file an
// earlier build wrote and asserts the Retention column comes out of the
// upgrade carrying this build's source and width, so an operator who has run
// a9s before reads the corrected cell rather than the old one.
func TestViewUpgrade_StillGeneratedColumnTakesTheCorrection(t *testing.T) {
	def, ok := config.DefaultConfig().Views["logs"]
	if !ok {
		t.Fatal("built-in views have no logs entry")
	}
	// The operator has moved a column, so the file is theirs and the wholesale
	// order rule does not apply — only the per-column correction can carry it.
	onDisk := tui5WithColumn(def.List, tui5PreviousBuildColumn)
	onDisk[0], onDisk[1] = onDisk[1], onDisk[0]

	dir := tui5SeedOlderView(t, "logs", onDisk, def.Detail)
	if got := tui5ColumnTitled(t, dir, "logs", "Retention"); got.Key != "" {
		t.Fatalf("test sanity: the seeded file already carries the correction: %+v", got)
	}

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	got := tui5ColumnTitled(t, dir, "logs", "Retention")
	var want config.ListColumn
	for _, c := range def.List {
		if c.Title == "Retention" {
			want = c
		}
	}
	if got != want {
		t.Errorf("Retention after the upgrade = %+v, want this build's column %+v", got, want)
	}
}

// TestViewUpgrade_OperatorsOwnFieldSurvivesTheCorrection is the other side,
// and it is per field rather than per column: the operator widened Retention,
// so the width is theirs and stays, while every field they left alone — the
// source among them — still takes this build's value. Freezing the whole
// column on one edit would leave them reading the old source because they had
// once made a column wider.
func TestViewUpgrade_OperatorsOwnFieldSurvivesTheCorrection(t *testing.T) {
	def := config.DefaultConfig().Views["logs"]
	theirs := tui5PreviousBuildColumn
	theirs.Width = 44 // the operator widened it

	dir := tui5SeedOlderView(t, "logs", tui5WithColumn(def.List, theirs), def.Detail)
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	got := tui5ColumnTitled(t, dir, "logs", "Retention")
	if got.Width != theirs.Width {
		t.Errorf("Retention width = %d, want the operator's %d", got.Width, theirs.Width)
	}
	if got.Key != "retention" {
		t.Errorf("Retention = %+v — a width they set kept the correction off a field they never touched", got)
	}
}

// TestUpgradeLeavesAnOperatorsOwnSourceAlone covers the other distinguishable
// case, a field whose value the operator changed to something neither build
// wrote.

// tui5WithColumn returns cols with the entry sharing replacement's title
// swapped for it.
func tui5WithColumn(cols []config.ListColumn, replacement config.ListColumn) []config.ListColumn {
	out := make([]config.ListColumn, 0, len(cols))
	for _, c := range cols {
		if c.Title == replacement.Title {
			out = append(out, replacement)
			continue
		}
		out = append(out, c)
	}
	return out
}
