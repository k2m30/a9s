package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
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

func TestEnsureViewsDir_SkipsExisting(t *testing.T) {
	dir := t.TempDir()

	ec2Path := filepath.Join(dir, "ec2.yaml")
	if err := os.WriteFile(ec2Path, []byte("# user edited"), 0600); err != nil {
		t.Fatalf("writing ec2.yaml: %v", err)
	}

	if err := config.EnsureViewsDir(dir); err != nil {
		t.Fatalf("EnsureViewsDir: %v", err)
	}

	data, err := os.ReadFile(ec2Path)
	if err != nil {
		t.Fatalf("reading ec2.yaml: %v", err)
	}
	if string(data) != "# user edited" {
		t.Errorf("ec2.yaml was overwritten, got: %s", string(data))
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
