package unit_test

import (
	"os"
	"path/filepath"
	"strings"
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
	// New lookup chain has 2 paths:
	//   1. .a9s/views.yaml (CWD-relative)
	//   2. ConfigDir()/views.yaml (resolved config dir — env var or ~/.a9s/)
	dirCWD := t.TempDir()
	dirConfigDir := t.TempDir()

	yamlCWD := []byte("views:\n  ec2:\n    list:\n      FromCWD:\n        path: cwd\n        width: 1\n")
	yamlConfigDir := []byte("views:\n  ec2:\n    list:\n      FromConfigDir:\n        path: configdir\n        width: 2\n")

	writeTmp := func(dir, filename string, data []byte) {
		t.Helper()
		full := filepath.Join(dir, filename)
		parent := filepath.Dir(full)
		if err := os.MkdirAll(parent, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Subtest 1: CWD .a9s/views.yaml wins over ConfigDir
	t.Run("cwd_wins", func(t *testing.T) {
		cwdA9sDir := filepath.Join(dirCWD, ".a9s")
		writeTmp(cwdA9sDir, "views.yaml", yamlCWD)
		writeTmp(dirConfigDir, "views.yaml", yamlConfigDir)

		paths := []string{
			filepath.Join(cwdA9sDir, "views.yaml"),
			filepath.Join(dirConfigDir, "views.yaml"),
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
			t.Fatalf("expected column from CWD .a9s/, got %+v", ec2.List)
		}
	})

	// Subtest 2: ConfigDir is used when CWD .a9s/ has no file
	t.Run("configdir_when_no_cwd", func(t *testing.T) {
		paths := []string{
			filepath.Join(t.TempDir(), ".a9s", "views.yaml"), // doesn't exist
			filepath.Join(dirConfigDir, "views.yaml"),
		}
		cfg, err := config.LoadFrom(paths)
		if err != nil {
			t.Fatalf("LoadFrom failed: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		ec2 := cfg.Views["ec2"]
		if len(ec2.List) != 1 || ec2.List[0].Title != "FromConfigDir" {
			t.Fatalf("expected column from ConfigDir, got %+v", ec2.List)
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
	expected := config.DefaultViewDef("ec2")
	if len(ec2.List) != len(expected.List) {
		t.Fatalf("expected %d default ec2 columns, got %d", len(expected.List), len(ec2.List))
	}

	// Verify the column titles match the built-in defaults
	for i, want := range expected.List {
		if ec2.List[i].Title != want.Title {
			t.Errorf("ec2 default column %d: got title %q, want %q", i, ec2.List[i].Title, want.Title)
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

	// EC2 not in partial config — should fall back to defaults
	ec2 := config.GetViewDef(cfg, "ec2")
	expectedEC2 := config.DefaultViewDef("ec2")
	if len(ec2.List) != len(expectedEC2.List) {
		t.Fatalf("ec2: expected %d default columns, got %d", len(expectedEC2.List), len(ec2.List))
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
	expected := config.DefaultViewDef("ec2")
	if len(ec2.List) != len(expected.List) {
		t.Fatalf("expected %d default ec2 columns after error, got %d", len(expected.List), len(ec2.List))
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

// ---------------------------------------------------------------------------
// Config directory management tests (ConfigDir, EnsureConfigDir, ConfigFilePath)
// ---------------------------------------------------------------------------

// TestConfigDir_DefaultsToHomeDir verifies that ConfigDir returns ~/.a9s/
// when no A9S_CONFIG_FOLDER env var is set.
func TestConfigDir_DefaultsToHomeDir(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", "")

	dir := config.ConfigDir()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	if !strings.HasPrefix(dir, home) {
		t.Errorf("ConfigDir() = %q, expected it to start with home dir %q", dir, home)
	}

	wantSuffix := filepath.Join(".a9s")
	if !strings.HasSuffix(dir, wantSuffix) {
		t.Errorf("ConfigDir() = %q, expected it to end with %q", dir, wantSuffix)
	}

	expected := filepath.Join(home, ".a9s")
	if dir != expected {
		t.Errorf("ConfigDir() = %q, want %q", dir, expected)
	}
}

// TestConfigDir_RespectsEnvVar verifies that ConfigDir returns the exact path
// from A9S_CONFIG_FOLDER when set.
func TestConfigDir_RespectsEnvVar(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", "/tmp/custom-a9s")

	dir := config.ConfigDir()

	if dir != "/tmp/custom-a9s" {
		t.Errorf("ConfigDir() = %q, want %q", dir, "/tmp/custom-a9s")
	}
}

// TestConfigDir_EnvVarOverridesHome verifies that when A9S_CONFIG_FOLDER is
// set, ConfigDir does NOT return a path under ~/.a9s/.
func TestConfigDir_EnvVarOverridesHome(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", "/tmp/override")

	dir := config.ConfigDir()

	if strings.Contains(dir, ".a9s") {
		t.Errorf("ConfigDir() = %q, should not contain '.a9s' when env var is set", dir)
	}
	if dir != "/tmp/override" {
		t.Errorf("ConfigDir() = %q, want %q", dir, "/tmp/override")
	}
}

// TestEnsureConfigDir_CreatesDirectory verifies that EnsureConfigDir creates
// the directory with 0700 permissions when it does not exist.
func TestEnsureConfigDir_CreatesDirectory(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "new-config")
	t.Setenv("A9S_CONFIG_FOLDER", target)

	dir, err := config.EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir() error: %v", err)
	}
	if dir != target {
		t.Errorf("EnsureConfigDir() = %q, want %q", dir, target)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("os.Stat(%q) error: %v", target, err)
	}
	if !info.IsDir() {
		t.Fatalf("%q is not a directory", target)
	}

	perm := info.Mode().Perm()
	if perm != 0700 {
		t.Errorf("directory permissions = %o, want %o", perm, 0700)
	}
}

// TestEnsureConfigDir_ExistingDirectory verifies that EnsureConfigDir succeeds
// without error when the directory already exists.
func TestEnsureConfigDir_ExistingDirectory(t *testing.T) {
	existing := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", existing)

	dir, err := config.EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir() error: %v", err)
	}
	if dir != existing {
		t.Errorf("EnsureConfigDir() = %q, want %q", dir, existing)
	}
}

// TestEnsureConfigDir_NoFilesCreated verifies that EnsureConfigDir creates
// only the directory itself, without populating it with any files.
func TestEnsureConfigDir_NoFilesCreated(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "empty-config")
	t.Setenv("A9S_CONFIG_FOLDER", target)

	_, err := config.EnsureConfigDir()
	if err != nil {
		t.Fatalf("EnsureConfigDir() error: %v", err)
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error: %v", target, err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected empty config directory, found %d entries: %v", len(entries), names)
	}
}

// TestConfigFilePath verifies that ConfigFilePath joins the config directory
// with the given filename correctly.
func TestConfigFilePath(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", "/tmp/a9s-test")

	got := config.ConfigFilePath("views.yaml")
	want := "/tmp/a9s-test/views.yaml"
	if got != want {
		t.Errorf("ConfigFilePath(\"views.yaml\") = %q, want %q", got, want)
	}

	got = config.ConfigFilePath("keybindings.yaml")
	want = "/tmp/a9s-test/keybindings.yaml"
	if got != want {
		t.Errorf("ConfigFilePath(\"keybindings.yaml\") = %q, want %q", got, want)
	}
}

// TestLookupPaths_EnvVarDoesNotFallThrough verifies that when
// A9S_CONFIG_FOLDER is set to a directory that has no views.yaml, Load()
// does NOT fall through to ~/.a9s/views.yaml. It should return (nil, nil).
func TestLookupPaths_EnvVarDoesNotFallThrough(t *testing.T) {
	// Point A9S_CONFIG_FOLDER to an empty temp dir (no views.yaml)
	emptyDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", emptyDir)

	// Also override HOME to a temp dir that DOES have ~/.a9s/views.yaml,
	// to prove Load() doesn't fall through to the home dir.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	homeA9s := filepath.Join(fakeHome, ".a9s")
	if err := os.MkdirAll(homeA9s, 0755); err != nil {
		t.Fatalf("failed to create fake home .a9s dir: %v", err)
	}
	yamlData := []byte("views:\n  ec2:\n    list:\n      ShouldNotLoad:\n        path: sneaky\n        width: 99\n")
	if err := os.WriteFile(filepath.Join(homeA9s, "views.yaml"), yamlData, 0644); err != nil {
		t.Fatalf("failed to write views.yaml: %v", err)
	}

	// Make sure the CWD .a9s/ path also doesn't exist
	// (we're in the test binary's working dir, which shouldn't have .a9s/)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg != nil {
		// If cfg is non-nil, it means Load() fell through to the home dir
		ec2, ok := cfg.Views["ec2"]
		if ok && len(ec2.List) > 0 && ec2.List[0].Title == "ShouldNotLoad" {
			t.Fatalf("Load() fell through to ~/.a9s/views.yaml despite A9S_CONFIG_FOLDER being set (to empty dir)")
		}
		t.Fatalf("Load() returned non-nil config; expected (nil, nil) when env var dir has no file")
	}
}
