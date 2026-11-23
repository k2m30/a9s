// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Operator-edited view files are read the way they were written: an explicit
// key, a file whose name resolves to no type, and a column whose value comes
// from a path.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
)

// views7EC2Placement is the SDK shape a Path-backed custom column reads.
type views7EC2Placement struct {
	AvailabilityZone string
}

// views7EC2Raw carries the two paths TestPathBackedCustomColumnIsAProducer reads.
type views7EC2Raw struct {
	InstanceType string
	Placement    views7EC2Placement
}

func views7Migrate(t *testing.T, shortName, body string) (*config.ViewsConfig, string) {
	t.Helper()
	dir := t.TempDir()
	views7WriteViewFile(t, dir, shortName, body)
	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}
	// The load's report is a value here, not a failure: a file the migration
	// did not carry is exactly what these cases are about, and the report is
	// one of its symptoms rather than the subject.
	cfg, _ := config.LoadFromDirs([]string{dir}) //nolint:errcheck // asserted by the caller, on the column
	if cfg == nil {
		t.Fatal("LoadFromDirs returned no config for a directory holding one file")
	}
	return cfg, dir
}

// views7CellOf renders one column of one row through the cascade every surface
// shares.
func views7CellOf(t *testing.T, shortName string, col config.ListColumn, r resource.Resource) string {
	t.Helper()
	return app.ExtractCellValue(
		app.ColumnDef{Key: col.Key, Title: col.Title, Width: col.Width, Path: col.Path, SortKey: col.SortKey},
		views7TypeOf(t, shortName), r)
}

// TestOperatorsExplicitKeySurvivesTheStart: a file with no generation stamp
// is the operator's. A key they put on a column, one the built-in column does
// not carry, is what the cell reads after a start.
func TestOperatorsExplicitKeySurvivesTheStart(t *testing.T) {
	cfg, _ := views7Migrate(t, "alarm", `list:
  Alarm Name:
    path: AlarmName
    width: 36
  Status:
    path: StateValue
    width: 12
  Actions On:
    path: ActionsEnabled
    key: actions_count
    width: 10
  Metric:
    path: MetricName
    width: 24
  Namespace:
    path: Namespace
    width: 24
  Threshold:
    path: Threshold
    key: threshold
    width: 12

detail:
  - AlarmName
`)

	col := views7ResolvedColumn(t, cfg, "alarm", "Actions On")
	if col.Key != "actions_count" {
		t.Errorf("the operator's key on Actions On resolved as %q, want %q — an unstamped file is "+
			"never rewritten and overrides the defaults", col.Key, "actions_count")
	}
	row := resource.Resource{ID: "acme-cpu-alarm", Fields: map[string]string{"actions_count": "3 actions"}}
	if got := views7CellOf(t, "alarm", col, row); got != "3 actions" {
		t.Errorf("the column shows %q, want %q — the key they wrote is the field the cell reads",
			got, "3 actions")
	}
}

// TestViewFileNameResolvesOrIsReported: a file is only in use when the name
// it is stored under is the name the runtime asks for, and a file that names
// no type is the same mistake as a key that names no field: unless reported,
// the operator believes their file is live.
func TestViewFileNameResolvesOrIsReported(t *testing.T) {
	dir := t.TempDir()
	views7WriteViewFile(t, dir, "EC2", `list:
  Instance ID:
    key: instance_id
    width: 99

detail:
  - InstanceId
`)
	views7WriteViewFile(t, dir, "ec22", `list:
  Instance ID:
    key: instance_id
    width: 20

detail:
  - InstanceId
`)

	cfg, err := config.LoadFromDirs([]string{dir})
	if cfg == nil {
		t.Fatal("LoadFromDirs returned no config for a directory holding two files")
	}

	view := config.GetViewDef(cfg, "ec2")
	if len(view.List) != 1 || view.List[0].Width != 99 {
		t.Errorf("ec2 renders %d columns from the built-in defaults — EC2.yaml names the ec2 type and "+
			"validated at load, so the runtime reads it too", len(view.List))
	}

	if err == nil {
		t.Fatalf("a file named ec22.yaml, which resolves to no resource type, was loaded in silence — " +
			"it is the same mistake as a key that names nothing, and the operator is watching a screen " +
			"their file never reached")
	}
	report := err.Error()
	if !strings.Contains(report, "ec22.yaml") {
		t.Errorf("the load reported %q — it names the file whose name resolves to nothing", report)
	}
	if n := strings.Count(report, "ec22"); n != 1 {
		t.Errorf("the load names ec22 %d times in one report; once is what a person reads", n)
	}
	if strings.Contains(report, "EC2.yaml") {
		t.Errorf("the load reported %q — EC2.yaml names a type this build has", report)
	}
}

// TestPathBackedCustomColumnIsAProducer: the load report asks
// anything can fill a column's key; a path IS what fills it, on both lanes —
// the cascade reads the struct live and the save lane writes that value under
// the same key for the next start.
func TestPathBackedCustomColumnIsAProducer(t *testing.T) {
	dir := t.TempDir()
	views7WriteViewFile(t, dir, "ec2", `list:
  Instance ID:
    key: instance_id
    width: 20
  AZ Copy:
    key: my_az
    path: Placement.AvailabilityZone
    width: 16
  Nowhere:
    key: my_missing_field
    width: 12

detail:
  - InstanceId
`)

	cfg, err := config.LoadFromDirs([]string{dir})
	if cfg == nil {
		t.Fatal("LoadFromDirs returned no config")
	}
	if err == nil {
		t.Fatal("the keyless-source column Nowhere was not reported — a key nothing fills is still a key nothing fills")
	}
	if strings.Contains(err.Error(), "my_az") {
		t.Errorf("the load reported %q — the column names the path its value comes from, which is what "+
			"fills the key on both lanes", err.Error())
	}
	if !strings.Contains(err.Error(), "my_missing_field") {
		t.Errorf("the load reported %q — a column with neither a path nor a producer is the report's "+
			"whole subject", err.Error())
	}

	col := views7ResolvedColumn(t, cfg, "ec2", "AZ Copy")
	live := resource.Resource{
		ID:        "i-0123456789abcdef0",
		Fields:    map[string]string{"instance_id": "i-0123456789abcdef0"},
		RawStruct: views7EC2Raw{InstanceType: "t3.micro", Placement: views7EC2Placement{AvailabilityZone: "us-east-1a"}},
	}
	if got := views7CellOf(t, "ec2", col, live); got != "us-east-1a" {
		t.Errorf("the live row shows %q in the operator's column, want %q", got, "us-east-1a")
	}

	// The cached row: the same column after the value has been materialised
	// under its key and the struct is gone, which is every start after the
	// first.
	cols := []app.ColumnDef{{Key: col.Key, Title: col.Title, Width: col.Width, Path: col.Path}}
	stored := app.MaterializeListFields(live, cols)
	cached := resource.Resource{ID: live.ID, Fields: stored.Fields}
	if got := views7CellOf(t, "ec2", col, cached); got != "us-east-1a" {
		t.Errorf("the cached row shows %q in the operator's column, want %q — the value the path "+
			"produced is what the next start reads", got, "us-east-1a")
	}
}

// TestTwoFilesForOneTypeAreReported: two files in one directory
// can name the same type — an operator who renamed a file and kept the old
// one, or one file named for the type and another for its alias — and the
// loader merges them in directory order with the last one winning. Whichever
// it picks, the operator has two files and one screen, and only the loader
// knows which one they are looking at.
//
// The two names differ by more than case on purpose: this filesystem is
// case-insensitive, so ec2.yaml and EC2.yaml are one file and there is no
// collision to report. "instances" is ec2's own alias.
func TestTwoFilesForOneTypeAreReported(t *testing.T) {
	dir := t.TempDir()
	views7WriteViewFile(t, dir, "instances", `list:
  Instance ID:
    key: instance_id
    width: 99

detail:
  - InstanceId
`)
	views7WriteViewFile(t, dir, "ec2", `list:
  Instance ID:
    key: instance_id
    width: 20

detail:
  - InstanceId
`)

	cfg, err := config.LoadFromDirs([]string{dir})
	if cfg == nil {
		t.Fatal("LoadFromDirs returned no config for a directory holding two files")
	}

	// One of them is in use, whole — never a column from each.
	view := config.GetViewDef(cfg, "ec2")
	if len(view.List) != 1 {
		t.Fatalf("ec2 resolved to %d columns; each file declares one", len(view.List))
	}
	active := "ec2.yaml"
	if view.List[0].Width == 99 {
		active = "instances.yaml"
	}

	if err == nil {
		t.Fatalf("two files in one directory both name the ec2 type and %s is the one in use — the "+
			"other was read, discarded and never mentioned", active)
	}
	report := err.Error()
	for _, name := range []string{"instances.yaml", "ec2.yaml"} {
		if n := strings.Count(report, name); n != 1 {
			t.Errorf("the load names %s %d times in %q; the report names both files, once each",
				name, n, report)
		}
	}
	if !views7SaysWhichIsActive(report, active) {
		t.Errorf("the load reported %q — it says which of the two files the screen is showing, so the "+
			"operator edits that one", report)
	}
}

// views7SaysWhichIsActive reports whether a collision report states that the
// views7SaysWhichIsActive reports whether a collision report states that the
// named file is the one in use. The wording is free; what it has to carry is
func views7SaysWhichIsActive(report, active string) bool {
	i := strings.Index(report, active)
	if i < 0 {
		return false
	}
	tail := strings.ToLower(report[i:])
	for _, said := range []string{"active", "in use", "wins", "is read", "reading"} {
		if strings.Contains(tail, said) {
			return true
		}
	}
	return false
}
