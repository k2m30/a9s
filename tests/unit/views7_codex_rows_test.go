// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// views7_codex_rows_test.go — the five ways a view file an operator edited is
// still not read the way they wrote it.
//
// Every one of them is the same mistake in a different place: a rule about
// what a person MEANT, inferred from what their file happens to differ from
// rather than from what the build before this one wrote. A key that differs is
// read as a key this build moved. A column renamed is read as a column that
// never was the status. A child type is not looked up at all. A file whose
// name is not exactly the short name is validated and then never read. A
// column whose value comes from a path is reported as one nothing fills.
package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// views7EC2Placement is the SDK shape a Path-backed custom column reads.
type views7EC2Placement struct {
	AvailabilityZone string
}

// views7EC2Raw carries the two paths the row 22 case names.
type views7EC2Raw struct {
	InstanceType string
	Placement    views7EC2Placement
}

// views7Migrate writes one view file, runs the migration over it, and returns
// the config the next start loads.
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

// views7MigratedFile is what the migration left on disk — the file the next
// start reads, before any merge the renderer performs on top of it.
func views7MigratedFile(t *testing.T, dir, shortName string) config.ViewDef {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, shortName+".yaml")) //nolint:gosec // a temp dir this test wrote
	if err != nil {
		t.Fatalf("reading the migrated %s.yaml: %v", shortName, err)
	}
	vd, err := config.ParseSingle(raw)
	if err != nil {
		t.Fatalf("parsing the migrated %s.yaml: %v", shortName, err)
	}
	return *vd
}

// views7CellOf renders one column of one row through the cascade every surface
// shares.
func views7CellOf(t *testing.T, shortName string, col config.ListColumn, r resource.Resource) string {
	t.Helper()
	return app.ExtractCellValue(
		app.ColumnDef{Key: col.Key, Title: col.Title, Width: col.Width, Path: col.Path, SortKey: col.SortKey},
		views7TypeOf(t, shortName), r)
}

// TestOperatorsExplicitKeySurvivesTheKeyMove pins row 18. The key move's
// bargain is that a column still holding what the previous build wrote takes
// this build's key. A column holding something ELSE is the operator's, and
// "differs from what this build declares" is not the same question.
func TestOperatorsExplicitKeySurvivesTheKeyMove(t *testing.T) {
	// The v5 alarm file with one edit: a key on the column the generator left
	// key-less, which is what an operator does when they want the cell to read
	// a field their own enricher writes.
	cfg, _ := views7Migrate(t, "alarm", `generated: 5
list:
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
		t.Errorf("the operator's key on Actions On resolved as %q, want %q — a column that does not "+
			"hold what the previous build wrote was edited, and the migration moves the key it moved, "+
			"not every key that differs from this build's", col.Key, "actions_count")
	}
	row := resource.Resource{ID: "acme-cpu-alarm", Fields: map[string]string{"actions_count": "3 actions"}}
	if got := views7CellOf(t, "alarm", col, row); got != "3 actions" {
		t.Errorf("the column shows %q, want %q — the key they wrote is the field the cell reads",
			got, "3 actions")
	}
}

// TestRenamedStatusColumnMigratesToTheLifecycleKey pins row 19. Until stamp 7
// a column was the status column if its TITLE said so, so an operator who
// renamed it kept a status column. This build asks the key, and the migration
// is what carries them across a rule it replaced.
func TestRenamedStatusColumnMigratesToTheLifecycleKey(t *testing.T) {
	// The v5 ec2 file with two renames: the status column, which the old rule
	// recognised by title, and an ordinary column, which it never did.
	cfg, _ := views7Migrate(t, "ec2", `generated: 5
list:
  Name:
    width: 24
  State:
    path: State.Name
    width: 12
  Size:
    path: InstanceType
    width: 14
  Instance ID:
    path: InstanceId
    width: 20

detail:
  - InstanceId
`)

	td := resource.FindResourceType("ec2")
	statusKey := views7LifecycleKey(td)

	renamed := views7ResolvedColumn(t, cfg, "ec2", "State")
	if renamed.Key != statusKey {
		t.Errorf("the renamed status column resolved with key %q, want %q — it was the status column "+
			"under the rule this build replaced, and a rule this build replaced is one it migrates",
			renamed.Key, statusKey)
	}
	flagged := resource.Resource{
		ID:     "i-0123456789abcdef0",
		Fields: map[string]string{"state": "running"},
		Findings: []domain.Finding{{
			Code: "ec2-public-ip", Phrase: "public ip", Severity: domain.SevWarn, Source: "wave1",
		}},
	}
	if got := views7CellOf(t, "ec2", renamed, flagged); got != "public ip" {
		t.Errorf("the renamed status column shows %q, want %q — a status column shows what the row is "+
			"flagged for", got, "public ip")
	}

	// The other half: a column the old rule never treated as the status is
	// left exactly as they wrote it.
	ordinary := views7ResolvedColumn(t, cfg, "ec2", "Size")
	if ordinary.Key != "" {
		t.Errorf("the renamed Type column gained key %q — only the column the replaced rule recognised "+
			"is migrated", ordinary.Key)
	}
	if ordinary.Path != "InstanceType" {
		t.Errorf("the renamed Type column resolved with path %q, want %q", ordinary.Path, "InstanceType")
	}
}

// TestChildViewFileMigratesLikeAnyOther pins row 20. A child type has a view
// file, a lifecycle key and an operator, and the migration looked its type up
// in the parent catalog only — so every child file is stamped as migrated and
// migrated by nothing.
func TestChildViewFileMigratesLikeAnyOther(t *testing.T) {
	// The status column is renamed, which is the half a title merge cannot
	// cover: the renderer folds the catalog's key onto a column whose title
	// still matches, so a file left unmigrated goes unnoticed until someone
	// renames one.
	cfg, dir := views7Migrate(t, "tg_health", `generated: 6
list:
  Target ID:
    key: target_id
    width: 24
  Check:
    key: status
    width: 14

detail:
  - target_id
`)

	child := resource.GetChildType("tg_health")
	if child == nil {
		t.Fatal("no tg_health child type registered")
	}

	onDisk := views7MigratedFile(t, dir, "tg_health")
	for _, col := range onDisk.List {
		if col.Title != "Check" {
			continue
		}
		if col.Key != child.StatusKey() {
			t.Errorf("the migrated child file carries key %q on its status column, want %q — the "+
				"migration looked the type up among the parents only, so every child file is stamped "+
				"as migrated and migrated by nothing", col.Key, child.StatusKey())
		}
	}

	col := views7ResolvedColumn(t, cfg, "tg_health", "Check")
	if col.Key != child.StatusKey() {
		t.Errorf("the child's status column resolved with key %q, want %q — a child type is a type, "+
			"and one lookup answers for both", col.Key, child.StatusKey())
	}
	flagged := resource.Resource{
		ID:     "i-0123456789abcdef0",
		Fields: map[string]string{"health": "unhealthy"},
		Findings: []domain.Finding{{
			Code: "tg-health-unhealthy", Phrase: "health checks failing",
			Severity: domain.SevBroken, Source: "wave1",
		}},
	}
	if got := views7CellOf(t, "tg_health", col, flagged); got != "health checks failing" {
		t.Errorf("the child's status column shows %q, want %q", got, "health checks failing")
	}
}

// TestViewFileNameResolvesOrIsReported pins row 21. A file is only in use when
// the name it is stored under is the name the runtime asks for, and a file
// that names no type is the same mistake as a key that names no field: silent
// today, and the operator believes their file is live.
func TestViewFileNameResolvesOrIsReported(t *testing.T) {
	dir := t.TempDir()
	views7WriteViewFile(t, dir, "EC2", `generated: 7
list:
  Instance ID:
    key: instance_id
    width: 99

detail:
  - InstanceId
`)
	views7WriteViewFile(t, dir, "ec22", `generated: 7
list:
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

// TestPathBackedCustomColumnIsAProducer pins row 22. The report asks whether
// anything can fill a column's key; a path IS what fills it, on both lanes —
// the cascade reads the struct live and the save lane writes that value under
// the same key for the next start.
func TestPathBackedCustomColumnIsAProducer(t *testing.T) {
	dir := t.TempDir()
	views7WriteViewFile(t, dir, "ec2", `generated: 7
list:
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

// TestTwoFilesForOneTypeAreReported pins row 24. Two files in one directory
// can name the same type — an operator who renamed a file and kept the old
// one, or a case-insensitive filesystem's two spellings — and the loader
// merges them in directory order with the last one winning. Whichever it
// picks, the operator has two files and one screen, and only the loader knows
// which one they are looking at.
func TestTwoFilesForOneTypeAreReported(t *testing.T) {
	dir := t.TempDir()
	views7WriteViewFile(t, dir, "EC2", `generated: 7
list:
  Instance ID:
    key: instance_id
    width: 99

detail:
  - InstanceId
`)
	views7WriteViewFile(t, dir, "ec2", `generated: 7
list:
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
		active = "EC2.yaml"
	}

	if err == nil {
		t.Fatalf("two files in one directory both name the ec2 type and %s is the one in use — the "+
			"other was read, discarded and never mentioned", active)
	}
	report := err.Error()
	for _, name := range []string{"EC2.yaml", "ec2.yaml"} {
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
// named file is the one in use. The sentence is dev's; what it has to carry is
// the file name and the fact that this is the one being read.
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
