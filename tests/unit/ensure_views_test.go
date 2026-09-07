package unit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
)

func TestGenerateViewYAML_ListOnly(t *testing.T) {
	v := config.ViewDef{
		List: []config.ListColumn{
			{Title: "Name", Path: "InstanceId", Width: 20},
			{Title: "State", Key: "lifecycle", Width: 10},
		},
	}
	out := string(config.GenerateViewYAML(v))

	for _, want := range []string{
		"list:\n",
		"  Name:\n",
		"    path: InstanceId\n",
		"    width: 20\n",
		"  State:\n",
		"    key: lifecycle\n",
		"    width: 10\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
	if strings.Contains(out, "detail:") {
		t.Error("output should not contain detail: section")
	}
}

func TestGenerateViewYAML_DetailOnly(t *testing.T) {
	v := config.ViewDef{
		Detail: []config.DetailField{{Path: "InstanceId"}, {Path: "State.Name"}, {Path: "InstanceType"}},
	}
	out := string(config.GenerateViewYAML(v))

	if !strings.Contains(out, "detail:\n") {
		t.Error("output missing detail: header")
	}
	for _, field := range []string{"InstanceId", "State.Name", "InstanceType"} {
		if !strings.Contains(out, field) {
			t.Errorf("output missing detail field %q", field)
		}
	}
	if strings.Contains(out, "list:") {
		t.Error("output should not contain list: section")
	}
}

func TestGenerateViewYAML_ListAndDetail(t *testing.T) {
	v := config.ViewDef{
		List: []config.ListColumn{
			{Title: "Name", Path: "BucketName", Width: 30},
		},
		Detail: []config.DetailField{{Path: "BucketName"}, {Path: "CreationDate"}},
	}
	out := string(config.GenerateViewYAML(v))

	if !strings.Contains(out, "list:\n") {
		t.Error("output missing list: section")
	}
	if !strings.Contains(out, "\n\ndetail:\n") {
		t.Error("list and detail sections should be separated by a blank line")
	}
}

func TestGenerateViewYAML_SpecialCharKey(t *testing.T) {
	v := config.ViewDef{
		List: []config.ListColumn{
			{Title: "Public IP:Port", Path: "Address", Width: 20},
		},
	}
	out := string(config.GenerateViewYAML(v))

	if !strings.Contains(out, `"Public IP:Port"`) {
		t.Errorf("title with colon should be quoted, got:\n%s", out)
	}
}

func TestEnsureViewsDir_CreatesFiles(t *testing.T) {
	dir := t.TempDir()

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	ec2Path := filepath.Join(dir, "ec2.yaml")
	data, err := os.ReadFile(ec2Path)
	if err != nil {
		t.Fatalf("reading ec2.yaml: %v", err)
	}
	if !strings.Contains(string(data), "list:") {
		t.Error("ec2.yaml should contain list: section")
	}
}

// TestEnsureViewsDir_SkipsExisting used to assert that an existing file is
// returned byte for byte, which is why a column added after an operator's first
// launch never reached them. The sort task's stamp row replaced "never touch it"
// with "add only what is missing": an existing file is still never replaced by
// the built-in default, and what it says about a column it already has still
// wins. Do not restore the byte-equality assertion — see
// TestEnsureViewsDir_LeavesACurrentFileAlone for the half that is still exact.
func TestEnsureViewsDir_SkipsExisting(t *testing.T) {
	dir := t.TempDir()

	ec2Path := filepath.Join(dir, "ec2.yaml")
	edited := "list:\n  Status:\n    path: State.Name\n    width: 3\n"
	if err := os.WriteFile(ec2Path, []byte(edited), 0600); err != nil {
		t.Fatalf("writing ec2.yaml: %v", err)
	}

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	data, err := os.ReadFile(ec2Path)
	if err != nil {
		t.Fatalf("reading ec2.yaml: %v", err)
	}
	vd, err := config.ParseSingle(data)
	if err != nil {
		t.Fatalf("parsing ec2.yaml: %v", err)
	}
	for _, c := range vd.List {
		if c.Title == "Status" && c.Width != 3 {
			t.Errorf("the operator's Status width became %d, want the 3 they set", c.Width)
		}
	}
}

func TestEnsureViewsDir_CreatesAllResourceTypes(t *testing.T) {
	dir := t.TempDir()

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir: %v", err)
	}

	var yamlCount int
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") {
			yamlCount++
		}
	}

	expected := len(config.DefaultConfig().Views)
	if yamlCount != expected {
		t.Errorf("got %d yaml files, want %d (one per resource type)", yamlCount, expected)
	}
}

func TestEnsureViewsReference_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	if err := config.EnsureViewsReference(dir); err != nil {
		t.Fatalf("EnsureViewsReference: %v", err)
	}

	refPath := filepath.Join(dir, "views_reference.yaml")
	data, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("reading views_reference.yaml: %v", err)
	}
	if !strings.Contains(string(data), "ec2:") {
		t.Error("views_reference.yaml should contain ec2: section")
	}
}

func TestEnsureViewsReference_OverwritesOnUpgrade(t *testing.T) {
	dir := t.TempDir()

	refPath := filepath.Join(dir, "views_reference.yaml")
	if err := os.WriteFile(refPath, []byte("# old version"), 0600); err != nil {
		t.Fatalf("writing views_reference.yaml: %v", err)
	}

	if err := config.EnsureViewsReference(dir); err != nil {
		t.Fatalf("EnsureViewsReference: %v", err)
	}

	data, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("reading views_reference.yaml: %v", err)
	}
	if string(data) == "# old version" {
		t.Error("views_reference.yaml should be overwritten with embedded data on upgrade")
	}
	if !strings.Contains(string(data), "ec2:") {
		t.Error("views_reference.yaml should contain embedded reference data after overwrite")
	}
}

// TestEnsureViewsDir_MergesGeneratedColumnsIntoAnOlderFile pins the migration
// an operator gets for free: a view file written by an older build carries an
// older stamp, so the columns that build did not know about are added to it and
// the stamp is brought up to date, while everything they wrote themselves —
// a column of their own, a width they changed — is left exactly as it was.
func TestEnsureViewsDir_MergesGeneratedColumnsIntoAnOlderFile(t *testing.T) {
	dir := t.TempDir()
	defaults := config.GetViewDef(nil, "ec2").List
	if len(defaults) < 3 {
		t.Fatalf("ec2 has %d built-in columns, need at least 3", len(defaults))
	}

	// The old build's file: the first column at a width the operator changed,
	// one column of their own, and nothing else the current build ships.
	old := "list:\n" +
		"  " + defaults[0].Title + ":\n" +
		pathOrKeyLine(defaults[0]) +
		"    width: 7\n" +
		"  Owner Team:\n" +
		"    key: owner_team\n" +
		"    width: 12\n"
	path := filepath.Join(dir, "ec2.yaml")
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatalf("writing ec2.yaml: %v", err)
	}

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading ec2.yaml: %v", err)
	}
	vd, err := config.ParseSingle(data)
	if err != nil {
		t.Fatalf("parsing merged ec2.yaml: %v", err)
	}

	byTitle := map[string]config.ListColumn{}
	for _, c := range vd.List {
		byTitle[c.Title] = c
	}
	for _, def := range defaults {
		if _, ok := byTitle[def.Title]; !ok {
			t.Errorf("column %q ships with this build but is missing from the migrated file", def.Title)
		}
	}
	if got := byTitle[defaults[0].Title].Width; got != 7 {
		t.Errorf("column %q has width %d, want the 7 the operator set", defaults[0].Title, got)
	}
	if own, ok := byTitle["Owner Team"]; !ok || own.Key != "owner_team" || own.Width != 12 {
		t.Errorf("the operator's own column did not survive the migration: %+v (present=%v)", own, ok)
	}
	if vd.Generated != config.GeneratedViewsVersion {
		t.Errorf("migrated file carries stamp %d, want %d — the next launch would migrate it again",
			vd.Generated, config.GeneratedViewsVersion)
	}
}

// TestEnsureViewsDir_LeavesACurrentFileAlone pins the other half: a file
// already at this build's stamp is not rewritten, so nothing the operator did
// to it can be lost by a launch that had nothing to add.
func TestEnsureViewsDir_LeavesACurrentFileAlone(t *testing.T) {
	dir := t.TempDir()
	current := fmt.Sprintf("generated: %d\nlist:\n  Name:\n    key: name\n    width: 9\n", config.GeneratedViewsVersion)
	path := filepath.Join(dir, "ec2.yaml")
	if err := os.WriteFile(path, []byte(current), 0600); err != nil {
		t.Fatalf("writing ec2.yaml: %v", err)
	}

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading ec2.yaml: %v", err)
	}
	if string(data) != current {
		t.Errorf("a file already at this build's stamp was rewritten:\n%s", string(data))
	}
}

func pathOrKeyLine(c config.ListColumn) string {
	if c.Path != "" {
		return "    path: " + c.Path + "\n"
	}
	return "    key: " + c.Key + "\n"
}
