package unit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k2m30/a9s/internal/config"
)

// testdataPath returns the absolute path to a file inside tests/testdata/.
func testdataPath(name string) string {
	// tests/unit/ -> tests/testdata/
	return filepath.Join("..", "testdata", name)
}

// ---------------------------------------------------------------------------
// T015: Test YAML parsing — load views_valid.yaml
// ---------------------------------------------------------------------------

func TestConfigYAMLParsing(t *testing.T) {
	paths := []string{testdataPath("views_valid.yaml")}
	cfg, err := config.LoadFrom(paths)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	// --- EC2 ---
	ec2, ok := cfg.Views["ec2"]
	if !ok {
		t.Fatal("missing ec2 view definition")
	}
	if len(ec2.List) != 3 {
		t.Fatalf("ec2: expected 3 list columns, got %d", len(ec2.List))
	}

	// Verify order and values
	wantEC2Cols := []struct {
		title string
		path  string
		width int
	}{
		{"Instance ID", "instanceId", 20},
		{"State", "state.name", 12},
		{"Type", "instanceType", 14},
	}
	for i, want := range wantEC2Cols {
		got := ec2.List[i]
		if got.Title != want.title || got.Path != want.path || got.Width != want.width {
			t.Errorf("ec2.List[%d] = {%q, %q, %d}, want {%q, %q, %d}",
				i, got.Title, got.Path, got.Width,
				want.title, want.path, want.width)
		}
	}

	if len(ec2.Detail) != 4 {
		t.Fatalf("ec2: expected 4 detail paths, got %d", len(ec2.Detail))
	}
	wantDetail := []string{"instanceId", "state", "instanceType", "placement"}
	for i, want := range wantDetail {
		if ec2.Detail[i] != want {
			t.Errorf("ec2.Detail[%d] = %q, want %q", i, ec2.Detail[i], want)
		}
	}

	// --- S3 ---
	s3, ok := cfg.Views["s3"]
	if !ok {
		t.Fatal("missing s3 view definition")
	}
	if len(s3.List) != 2 {
		t.Fatalf("s3: expected 2 list columns, got %d", len(s3.List))
	}
	if len(s3.Detail) != 2 {
		t.Fatalf("s3: expected 2 detail paths, got %d", len(s3.Detail))
	}
}

// ---------------------------------------------------------------------------
// T016: Test lookup chain — priority of config file locations
// ---------------------------------------------------------------------------

func TestConfigLookupChain(t *testing.T) {
	// Create three temp dirs simulating: cwd, A9S_CONFIG_FOLDER, ~/.a9s/
	dirCWD := t.TempDir()
	dirEnv := t.TempDir()
	dirHome := t.TempDir()

	yamlCWD := []byte("views:\n  ec2:\n    list:\n      FromCWD:\n        path: cwd\n        width: 1\n")
	yamlEnv := []byte("views:\n  ec2:\n    list:\n      FromEnv:\n        path: env\n        width: 2\n")
	yamlHome := []byte("views:\n  ec2:\n    list:\n      FromHome:\n        path: home\n        width: 3\n")

	writeTmp := func(dir string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "views.yaml"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Subtest 1: CWD wins over all
	t.Run("cwd_wins", func(t *testing.T) {
		writeTmp(dirCWD, yamlCWD)
		writeTmp(dirEnv, yamlEnv)
		writeTmp(dirHome, yamlHome)

		paths := []string{
			filepath.Join(dirCWD, "views.yaml"),
			filepath.Join(dirEnv, "views.yaml"),
			filepath.Join(dirHome, "views.yaml"),
		}
		cfg, err := config.LoadFrom(paths)
		if err != nil {
			t.Fatalf("LoadFrom failed: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		ec2 := cfg.Views["ec2"]
		if len(ec2.List) != 1 || ec2.List[0].Title != "FromCWD" {
			t.Fatalf("expected column from CWD, got %+v", ec2.List)
		}
	})

	// Subtest 2: env wins when cwd doesn't exist
	t.Run("env_wins_over_home", func(t *testing.T) {
		paths := []string{
			filepath.Join(t.TempDir(), "views.yaml"), // doesn't exist
			filepath.Join(dirEnv, "views.yaml"),
			filepath.Join(dirHome, "views.yaml"),
		}
		cfg, err := config.LoadFrom(paths)
		if err != nil {
			t.Fatalf("LoadFrom failed: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		ec2 := cfg.Views["ec2"]
		if len(ec2.List) != 1 || ec2.List[0].Title != "FromEnv" {
			t.Fatalf("expected column from Env, got %+v", ec2.List)
		}
	})

	// Subtest 3: home is lowest priority
	t.Run("home_lowest_priority", func(t *testing.T) {
		paths := []string{
			filepath.Join(t.TempDir(), "views.yaml"), // doesn't exist
			filepath.Join(t.TempDir(), "views.yaml"), // doesn't exist
			filepath.Join(dirHome, "views.yaml"),
		}
		cfg, err := config.LoadFrom(paths)
		if err != nil {
			t.Fatalf("LoadFrom failed: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		ec2 := cfg.Views["ec2"]
		if len(ec2.List) != 1 || ec2.List[0].Title != "FromHome" {
			t.Fatalf("expected column from Home, got %+v", ec2.List)
		}
	})
}

// ---------------------------------------------------------------------------
// T017: Test fallback — no config file returns built-in defaults
// ---------------------------------------------------------------------------

func TestConfigFallbackDefaults(t *testing.T) {
	// No files exist
	paths := []string{
		filepath.Join(t.TempDir(), "views.yaml"),
	}
	cfg, err := config.LoadFrom(paths)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config when no file found")
	}

	// GetViewDef with nil cfg should return defaults
	ec2 := config.GetViewDef(nil, "ec2")
	if len(ec2.List) != 7 {
		t.Fatalf("expected 7 default ec2 columns, got %d", len(ec2.List))
	}

	// Verify the default column titles match the hardcoded values
	wantTitles := []string{
		"Name", "Instance ID", "State", "Type",
		"Private IP", "Public IP", "Launch Time",
	}
	for i, want := range wantTitles {
		if ec2.List[i].Title != want {
			t.Errorf("ec2 default column %d: got title %q, want %q", i, ec2.List[i].Title, want)
		}
	}
}

// ---------------------------------------------------------------------------
// T018: Test partial config — s3 from config, ec2 falls back to defaults
// ---------------------------------------------------------------------------

func TestConfigPartialOverride(t *testing.T) {
	paths := []string{testdataPath("views_partial.yaml")}
	cfg, err := config.LoadFrom(paths)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	// S3 should use config values
	s3 := config.GetViewDef(cfg, "s3")
	if len(s3.List) != 2 {
		t.Fatalf("s3: expected 2 columns from config, got %d", len(s3.List))
	}
	if s3.List[0].Title != "Bucket Name" || s3.List[0].Width != 50 {
		t.Errorf("s3 col 0: got {%q, %d}, want {\"Bucket Name\", 50}", s3.List[0].Title, s3.List[0].Width)
	}
	if s3.List[1].Title != "Created" || s3.List[1].Width != 20 {
		t.Errorf("s3 col 1: got {%q, %d}, want {\"Created\", 20}", s3.List[1].Title, s3.List[1].Width)
	}

	// EC2 not in partial config — should fall back to defaults (7 columns)
	ec2 := config.GetViewDef(cfg, "ec2")
	if len(ec2.List) != 7 {
		t.Fatalf("ec2: expected 7 default columns, got %d", len(ec2.List))
	}
}

// ---------------------------------------------------------------------------
// T041: Test config loading recognizes s3_objects — default and YAML
// ---------------------------------------------------------------------------

func TestConfigDefaultViewDef_S3Objects(t *testing.T) {
	vd := config.DefaultViewDef("s3_objects")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 list columns for s3_objects default, got %d", len(vd.List))
	}

	wantCols := []struct {
		title string
		path  string
		width int
	}{
		{"Key", "Key", 36},
		{"Size", "Size", 12},
		{"Storage Class", "StorageClass", 16},
		{"Last Modified", "LastModified", 22},
	}
	for i, want := range wantCols {
		got := vd.List[i]
		if got.Title != want.title || got.Path != want.path || got.Width != want.width {
			t.Errorf("s3_objects default List[%d] = {%q, %q, %d}, want {%q, %q, %d}",
				i, got.Title, got.Path, got.Width,
				want.title, want.path, want.width)
		}
	}
}

func TestConfigYAMLParsing_S3Objects(t *testing.T) {
	paths := []string{testdataPath("views_s3_objects.yaml")}
	cfg, err := config.LoadFrom(paths)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	s3obj, ok := cfg.Views["s3_objects"]
	if !ok {
		t.Fatal("missing s3_objects view definition in parsed YAML")
	}
	if len(s3obj.List) != 4 {
		t.Fatalf("s3_objects: expected 4 list columns, got %d", len(s3obj.List))
	}

	// The YAML file uses custom widths different from defaults
	wantCols := []struct {
		title string
		path  string
		width int
	}{
		{"Key", "key", 60},
		{"Size", "size", 14},
		{"Last Modified", "lastModified", 24},
		{"Storage Class", "storageClass", 18},
	}
	for i, want := range wantCols {
		got := s3obj.List[i]
		if got.Title != want.title || got.Path != want.path || got.Width != want.width {
			t.Errorf("s3_objects YAML List[%d] = {%q, %q, %d}, want {%q, %q, %d}",
				i, got.Title, got.Path, got.Width,
				want.title, want.path, want.width)
		}
	}
}

// ---------------------------------------------------------------------------
// T042: Test s3_objects config columns via GetViewDef (with and without config)
// ---------------------------------------------------------------------------

func TestGetViewDef_S3Objects_NilConfig(t *testing.T) {
	// With nil config, should fall back to defaults
	vd := config.GetViewDef(nil, "s3_objects")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 default s3_objects columns with nil config, got %d", len(vd.List))
	}
	if vd.List[0].Title != "Key" {
		t.Errorf("expected first column title 'Key', got %q", vd.List[0].Title)
	}
}

func TestGetViewDef_S3Objects_FromConfig(t *testing.T) {
	paths := []string{testdataPath("views_s3_objects.yaml")}
	cfg, err := config.LoadFrom(paths)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	vd := config.GetViewDef(cfg, "s3_objects")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 s3_objects columns from config, got %d", len(vd.List))
	}
	// Config has width 60 for Key, defaults have 50
	if vd.List[0].Width != 60 {
		t.Errorf("expected Key width 60 from config override, got %d", vd.List[0].Width)
	}
}

func TestGetViewDef_S3Objects_PartialConfig_FallsBackToDefaults(t *testing.T) {
	// Config has ec2 but not s3_objects — should fall back to defaults
	paths := []string{testdataPath("views_partial.yaml")}
	cfg, err := config.LoadFrom(paths)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	vd := config.GetViewDef(cfg, "s3_objects")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 default s3_objects columns (not in partial config), got %d", len(vd.List))
	}
	// Should be default widths
	if vd.List[0].Width != 36 {
		t.Errorf("expected default Key width 36, got %d", vd.List[0].Width)
	}
}

// ---------------------------------------------------------------------------
// T019: Test invalid YAML — returns error, GetViewDef still returns defaults
// ---------------------------------------------------------------------------

func TestConfigInvalidYAML(t *testing.T) {
	paths := []string{testdataPath("views_invalid.yaml")}
	cfg, err := config.LoadFrom(paths)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
	if cfg != nil {
		t.Fatal("expected nil config on parse error")
	}

	// Even after error, GetViewDef with nil config should give defaults
	ec2 := config.GetViewDef(nil, "ec2")
	if len(ec2.List) != 7 {
		t.Fatalf("expected 7 default ec2 columns after error, got %d", len(ec2.List))
	}
}

// ---------------------------------------------------------------------------
// Test: YAML key field parses into ListColumn.Key
// ---------------------------------------------------------------------------

func TestConfigYAMLParsing_KeyField(t *testing.T) {
	yamlData := `
views:
  sqs:
    list:
      Queue Name:
        path: QueueUrl
        width: 36
      Messages:
        key: approx_messages
        width: 10
      In Flight:
        key: approx_not_visible
        width: 10
    detail:
      - QueueUrl
`
	cfg, err := config.Parse([]byte(yamlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	sqs, ok := cfg.Views["sqs"]
	if !ok {
		t.Fatal("missing sqs view definition")
	}
	if len(sqs.List) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(sqs.List))
	}

	// First column: has path, no key
	if sqs.List[0].Path != "QueueUrl" {
		t.Errorf("col 0 path: got %q, want %q", sqs.List[0].Path, "QueueUrl")
	}
	if sqs.List[0].Key != "" {
		t.Errorf("col 0 key: got %q, want empty", sqs.List[0].Key)
	}

	// Second column: has key, no path
	if sqs.List[1].Key != "approx_messages" {
		t.Errorf("col 1 key: got %q, want %q", sqs.List[1].Key, "approx_messages")
	}
	if sqs.List[1].Path != "" {
		t.Errorf("col 1 path: got %q, want empty", sqs.List[1].Path)
	}

	// Third column: has key, no path
	if sqs.List[2].Key != "approx_not_visible" {
		t.Errorf("col 2 key: got %q, want %q", sqs.List[2].Key, "approx_not_visible")
	}
}
