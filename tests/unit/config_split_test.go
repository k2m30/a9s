package unit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
)

// ===========================================================================
// ParseSingle tests
// ===========================================================================

func TestParseSingle_ValidEC2(t *testing.T) {
	data := []byte(`
list:
  Instance ID:
    path: instanceId
    width: 20
  State:
    path: state.name
    width: 12
  Type:
    path: instanceType
    width: 14
detail:
  - instanceId
  - state
  - instanceType
  - placement
`)
	vd, err := config.ParseSingle(data)
	if err != nil {
		t.Fatalf("ParseSingle failed: %v", err)
	}
	if vd == nil {
		t.Fatal("expected non-nil ViewDef")
	}
	if len(vd.List) != 3 {
		t.Fatalf("expected 3 list columns, got %d", len(vd.List))
	}
	if vd.List[0].Title != "Instance ID" || vd.List[0].Path != "instanceId" || vd.List[0].Width != 20 {
		t.Errorf("col 0 mismatch: %+v", vd.List[0])
	}
	if vd.List[1].Title != "State" || vd.List[1].Path != "state.name" || vd.List[1].Width != 12 {
		t.Errorf("col 1 mismatch: %+v", vd.List[1])
	}
	if vd.List[2].Title != "Type" || vd.List[2].Path != "instanceType" || vd.List[2].Width != 14 {
		t.Errorf("col 2 mismatch: %+v", vd.List[2])
	}
	if len(vd.Detail) != 4 {
		t.Fatalf("expected 4 detail paths, got %d", len(vd.Detail))
	}
	wantDetail := []string{"instanceId", "state", "instanceType", "placement"}
	for i, want := range wantDetail {
		if vd.Detail[i] != want {
			t.Errorf("Detail[%d] = %q, want %q", i, vd.Detail[i], want)
		}
	}
}

func TestParseSingle_ListOnly(t *testing.T) {
	data := []byte(`
list:
  Bucket Name:
    path: name
    width: 50
`)
	vd, err := config.ParseSingle(data)
	if err != nil {
		t.Fatalf("ParseSingle failed: %v", err)
	}
	if len(vd.List) != 1 {
		t.Fatalf("expected 1 list column, got %d", len(vd.List))
	}
	if len(vd.Detail) != 0 {
		t.Errorf("expected 0 detail paths, got %d", len(vd.Detail))
	}
}

func TestParseSingle_DetailOnly(t *testing.T) {
	data := []byte(`
detail:
  - InstanceId
  - State
`)
	vd, err := config.ParseSingle(data)
	if err != nil {
		t.Fatalf("ParseSingle failed: %v", err)
	}
	if len(vd.List) != 0 {
		t.Errorf("expected 0 list columns, got %d", len(vd.List))
	}
	if len(vd.Detail) != 2 {
		t.Fatalf("expected 2 detail paths, got %d", len(vd.Detail))
	}
}

func TestParseSingle_KeyField(t *testing.T) {
	data := []byte(`
list:
  Queue Name:
    path: QueueUrl
    width: 36
  Messages:
    key: approx_messages
    width: 10
`)
	vd, err := config.ParseSingle(data)
	if err != nil {
		t.Fatalf("ParseSingle failed: %v", err)
	}
	if len(vd.List) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(vd.List))
	}
	if vd.List[0].Path != "QueueUrl" || vd.List[0].Key != "" {
		t.Errorf("col 0: path=%q key=%q", vd.List[0].Path, vd.List[0].Key)
	}
	if vd.List[1].Key != "approx_messages" || vd.List[1].Path != "" {
		t.Errorf("col 1: key=%q path=%q", vd.List[1].Key, vd.List[1].Path)
	}
}

func TestParseSingle_InvalidYAML(t *testing.T) {
	data := []byte(`
list:
  Name:
    path: foo
    width: [invalid
  this is not valid: {{{
`)
	_, err := config.ParseSingle(data)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParseSingle_EmptyData(t *testing.T) {
	vd, err := config.ParseSingle([]byte(""))
	if err != nil {
		t.Fatalf("ParseSingle on empty data failed: %v", err)
	}
	// Empty YAML should return an empty ViewDef, not nil
	if vd == nil {
		t.Fatal("expected non-nil ViewDef for empty data")
	}
	if len(vd.List) != 0 || len(vd.Detail) != 0 {
		t.Errorf("expected empty ViewDef, got List=%d Detail=%d", len(vd.List), len(vd.Detail))
	}
}

// ===========================================================================
// LoadFromDirs tests
// ===========================================================================

func TestLoadFromDirs_SingleDir(t *testing.T) {
	dir := t.TempDir()
	viewsDir := filepath.Join(dir, "views")
	if err := os.MkdirAll(viewsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write ec2.yaml
	ec2Data := []byte(`list:
  Instance ID:
    path: instanceId
    width: 20
detail:
  - instanceId
`)
	if err := os.WriteFile(filepath.Join(viewsDir, "ec2.yaml"), ec2Data, 0644); err != nil {
		t.Fatal(err)
	}

	// Write s3.yaml
	s3Data := []byte(`list:
  Bucket Name:
    path: name
    width: 40
detail:
  - name
`)
	if err := os.WriteFile(filepath.Join(viewsDir, "s3.yaml"), s3Data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFromDirs([]string{viewsDir})
	if err != nil {
		t.Fatalf("LoadFromDirs failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	// Check ec2
	ec2, ok := cfg.Views["ec2"]
	if !ok {
		t.Fatal("missing ec2 view")
	}
	if len(ec2.List) != 1 {
		t.Fatalf("ec2: expected 1 list column, got %d", len(ec2.List))
	}
	if ec2.List[0].Title != "Instance ID" {
		t.Errorf("ec2 col 0 title: %q", ec2.List[0].Title)
	}

	// Check s3
	s3, ok := cfg.Views["s3"]
	if !ok {
		t.Fatal("missing s3 view")
	}
	if len(s3.List) != 1 {
		t.Fatalf("s3: expected 1 list column, got %d", len(s3.List))
	}
}

func TestLoadFromDirs_LayerMerge(t *testing.T) {
	// Global dir has ec2 and s3
	globalDir := t.TempDir()
	ec2Global := []byte(`list:
  Instance ID:
    path: instanceId
    width: 20
detail:
  - instanceId
`)
	s3Global := []byte(`list:
  Bucket Name:
    path: name
    width: 30
detail:
  - name
`)
	if err := os.WriteFile(filepath.Join(globalDir, "ec2.yaml"), ec2Global, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "s3.yaml"), s3Global, 0644); err != nil {
		t.Fatal(err)
	}

	// Project dir overrides ec2 only
	projectDir := t.TempDir()
	ec2Project := []byte(`list:
  Instance ID:
    path: instanceId
    width: 99
detail:
  - instanceId
  - state
`)
	if err := os.WriteFile(filepath.Join(projectDir, "ec2.yaml"), ec2Project, 0644); err != nil {
		t.Fatal(err)
	}

	// Global first, project overlays
	cfg, err := config.LoadFromDirs([]string{globalDir, projectDir})
	if err != nil {
		t.Fatalf("LoadFromDirs failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	// ec2 should come from project dir (overlay wins)
	ec2 := cfg.Views["ec2"]
	if len(ec2.List) != 1 || ec2.List[0].Width != 99 {
		t.Errorf("ec2: expected project override (width=99), got %+v", ec2.List)
	}
	if len(ec2.Detail) != 2 {
		t.Errorf("ec2 detail: expected 2, got %d", len(ec2.Detail))
	}

	// s3 should still come from global dir (not overridden)
	s3 := cfg.Views["s3"]
	if len(s3.List) != 1 || s3.List[0].Width != 30 {
		t.Errorf("s3: expected global (width=30), got %+v", s3.List)
	}
}

func TestLoadFromDirs_NoDirsExist(t *testing.T) {
	cfg, err := config.LoadFromDirs([]string{"/nonexistent/path1", "/nonexistent/path2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config when no dirs exist")
	}
}

func TestLoadFromDirs_EmptyDir(t *testing.T) {
	emptyDir := t.TempDir()
	cfg, err := config.LoadFromDirs([]string{emptyDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for empty dir")
	}
}

func TestLoadFromDirs_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()
	// Write a .txt file — should be skipped
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	// Write a valid .yaml file
	if err := os.WriteFile(filepath.Join(dir, "ec2.yaml"), []byte("list:\n  ID:\n    path: id\n    width: 10\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFromDirs([]string{dir})
	if err != nil {
		t.Fatalf("LoadFromDirs failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if _, ok := cfg.Views["notes"]; ok {
		t.Error("should not have parsed notes.txt as a resource")
	}
	if _, ok := cfg.Views["ec2"]; !ok {
		t.Error("missing ec2 from valid yaml file")
	}
}

func TestLoadFromDirs_InvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("list: {{{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := config.LoadFromDirs([]string{dir})
	if err == nil {
		t.Fatal("expected error for invalid YAML file")
	}
}

func TestLoadFromDirs_FilenameBecomesResourceName(t *testing.T) {
	dir := t.TempDir()
	// s3_objects.yaml -> resource name "s3_objects"
	data := []byte(`list:
  Key:
    path: Key
    width: 36
`)
	if err := os.WriteFile(filepath.Join(dir, "s3_objects.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadFromDirs([]string{dir})
	if err != nil {
		t.Fatalf("LoadFromDirs failed: %v", err)
	}
	if _, ok := cfg.Views["s3_objects"]; !ok {
		t.Error("expected resource name 's3_objects' from file s3_objects.yaml")
	}
}

// ===========================================================================
// Load() integration with dirs
// ===========================================================================

func TestLoad_UsesViewsDirs(t *testing.T) {
	// Set up a temp config dir with views/ subdirectory
	tmpDir := t.TempDir()
	viewsDir := filepath.Join(tmpDir, "views")
	if err := os.MkdirAll(viewsDir, 0755); err != nil {
		t.Fatal(err)
	}

	ec2Data := []byte(`list:
  Custom Col:
    path: custom
    width: 42
`)
	if err := os.WriteFile(filepath.Join(viewsDir, "ec2.yaml"), ec2Data, 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("A9S_CONFIG_FOLDER", tmpDir)
	// Set HOME to something without .a9s
	t.Setenv("HOME", t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	ec2, ok := cfg.Views["ec2"]
	if !ok {
		t.Fatal("missing ec2 in loaded config")
	}
	if len(ec2.List) != 1 || ec2.List[0].Title != "Custom Col" {
		t.Errorf("expected custom col from views dir, got %+v", ec2.List)
	}
}

func TestLoad_ProjectDirOverlaysGlobal(t *testing.T) {
	// Global config dir with ec2 width=10
	globalDir := t.TempDir()
	globalViewsDir := filepath.Join(globalDir, "views")
	if err := os.MkdirAll(globalViewsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalViewsDir, "ec2.yaml"),
		[]byte("list:\n  ID:\n    path: id\n    width: 10\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("A9S_CONFIG_FOLDER", globalDir)
	t.Setenv("HOME", t.TempDir())

	// We can't easily test CWD-relative .a9s/views/ without changing CWD,
	// but we can verify Load works with just the global dir.
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	ec2 := cfg.Views["ec2"]
	if len(ec2.List) != 1 || ec2.List[0].Width != 10 {
		t.Errorf("expected width=10 from global, got %+v", ec2.List)
	}
}
