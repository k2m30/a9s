package unit_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/config"
)

func testdataDir(parts ...string) string {
	// tests/unit/ -> tests/testdata/
	return filepath.Join(append([]string{"..", "testdata"}, parts...)...)
}

func TestConfigYAMLParsing(t *testing.T) {
	dirs := []string{testdataDir("views", "valid")}
	cfg, err := config.LoadFromDirs(dirs)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	ec2, ok := cfg.Views["ec2"]
	if !ok {
		t.Fatal("missing ec2 view definition")
	}
	if len(ec2.List) != 3 {
		t.Fatalf("ec2: expected 3 list columns, got %d", len(ec2.List))
	}

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
		if ec2.Detail[i].String() != want {
			t.Errorf("ec2.Detail[%d] = %q, want %q", i, ec2.Detail[i].String(), want)
		}
	}

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

func TestConfigLookupChain(t *testing.T) {
	// LoadFromDirs processes directories in order; later dirs overlay earlier.
	// Each dir contains per-resource .yaml files (no "views:" wrapper).

	t.Run("cwd_wins", func(t *testing.T) {
		dir1 := filepath.Join(t.TempDir(), "views")
		dir2 := filepath.Join(t.TempDir(), "views")
		os.MkdirAll(dir1, 0755)
		os.MkdirAll(dir2, 0755)
		os.WriteFile(filepath.Join(dir1, "ec2.yaml"), []byte("list:\n  FromDir1:\n    path: dir1\n    width: 1\n"), 0644)
		os.WriteFile(filepath.Join(dir2, "ec2.yaml"), []byte("list:\n  FromDir2:\n    path: dir2\n    width: 2\n"), 0644)

		cfg, err := config.LoadFromDirs([]string{dir1, dir2})
		if err != nil {
			t.Fatalf("LoadFromDirs failed: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		ec2 := cfg.Views["ec2"]
		if len(ec2.List) != 1 || ec2.List[0].Title != "FromDir2" {
			t.Fatalf("expected column from Dir2 (overlay), got %+v", ec2.List)
		}
	})

	t.Run("configdir_when_no_cwd", func(t *testing.T) {
		dir1 := filepath.Join(t.TempDir(), "views")
		os.MkdirAll(dir1, 0755)
		os.WriteFile(filepath.Join(dir1, "ec2.yaml"), []byte("list:\n  FromConfigDir:\n    path: configdir\n    width: 2\n"), 0644)

		cfg, err := config.LoadFromDirs([]string{dir1, filepath.Join(t.TempDir(), "nonexistent")})
		if err != nil {
			t.Fatalf("LoadFromDirs failed: %v", err)
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

func TestConfigFallbackDefaults(t *testing.T) {
	dirs := []string{filepath.Join(t.TempDir(), "nonexistent")}
	cfg, err := config.LoadFromDirs(dirs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config when no file found")
	}

	ec2 := config.GetViewDef(nil, "ec2")
	expected := config.DefaultViewDef("ec2")
	if len(ec2.List) != len(expected.List) {
		t.Fatalf("expected %d default ec2 columns, got %d", len(expected.List), len(ec2.List))
	}

	for i, want := range expected.List {
		if ec2.List[i].Title != want.Title {
			t.Errorf("ec2 default column %d: got title %q, want %q", i, ec2.List[i].Title, want.Title)
		}
	}
}

func TestConfigPartialOverride(t *testing.T) {
	dirs := []string{testdataDir("views", "partial")}
	cfg, err := config.LoadFromDirs(dirs)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

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

	ec2 := config.GetViewDef(cfg, "ec2")
	expectedEC2 := config.DefaultViewDef("ec2")
	if len(ec2.List) != len(expectedEC2.List) {
		t.Fatalf("ec2: expected %d default columns, got %d", len(expectedEC2.List), len(ec2.List))
	}
}

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
		{"Size", "", 12},
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
	dirs := []string{testdataDir("views", "s3_objects")}
	cfg, err := config.LoadFromDirs(dirs)
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

func TestGetViewDef_S3Objects_NilConfig(t *testing.T) {
	vd := config.GetViewDef(nil, "s3_objects")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 default s3_objects columns with nil config, got %d", len(vd.List))
	}
	if vd.List[0].Title != "Key" {
		t.Errorf("expected first column title 'Key', got %q", vd.List[0].Title)
	}
}

func TestGetViewDef_S3Objects_FromConfig(t *testing.T) {
	dirs := []string{testdataDir("views", "s3_objects")}
	cfg, err := config.LoadFromDirs(dirs)
	if err != nil {
		t.Fatalf("LoadFromDirs failed: %v", err)
	}

	vd := config.GetViewDef(cfg, "s3_objects")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 s3_objects columns from config, got %d", len(vd.List))
	}
	if vd.List[0].Width != 60 {
		t.Errorf("expected Key width 60 from config override, got %d", vd.List[0].Width)
	}
}

func TestGetViewDef_S3Objects_PartialConfig_FallsBackToDefaults(t *testing.T) {
	dirs := []string{testdataDir("views", "partial")}
	cfg, err := config.LoadFromDirs(dirs)
	if err != nil {
		t.Fatalf("LoadFromDirs failed: %v", err)
	}

	vd := config.GetViewDef(cfg, "s3_objects")
	if len(vd.List) != 4 {
		t.Fatalf("expected 4 default s3_objects columns (not in partial config), got %d", len(vd.List))
	}
	if vd.List[0].Width != 36 {
		t.Errorf("expected default Key width 36, got %d", vd.List[0].Width)
	}
}

func TestViewsDir_RoundTrip_MatchesDefaults(t *testing.T) {
	viewsDir := filepath.Join("..", "..", ".a9s", "views")
	cfg, err := config.LoadFromDirs([]string{viewsDir})
	if err != nil {
		t.Fatalf("failed to load .a9s/views/: %v", err)
	}
	if cfg == nil {
		t.Fatalf(".a9s/views/ directory not found or returned nil config")
	}

	defaults := config.DefaultConfig()

	for name, defView := range defaults.Views {
		yamlView, ok := cfg.Views[name]
		if !ok {
			t.Errorf("views dir missing resource %q (present in defaults.go)", name)
			continue
		}

		if len(yamlView.List) != len(defView.List) {
			t.Errorf("%s: list column count mismatch: yaml=%d, default=%d",
				name, len(yamlView.List), len(defView.List))
			continue
		}

		for i, defCol := range defView.List {
			yamlCol := yamlView.List[i]
			if yamlCol.Title != defCol.Title {
				t.Errorf("%s.List[%d].Title: yaml=%q, default=%q", name, i, yamlCol.Title, defCol.Title)
			}
			if yamlCol.Key != defCol.Key {
				t.Errorf("%s.List[%d].Key: yaml=%q, default=%q", name, i, yamlCol.Key, defCol.Key)
			}
			if yamlCol.Path != defCol.Path {
				t.Errorf("%s.List[%d].Path: yaml=%q, default=%q", name, i, yamlCol.Path, defCol.Path)
			}
			if yamlCol.Width != defCol.Width {
				t.Errorf("%s.List[%d].Width: yaml=%d, default=%d", name, i, yamlCol.Width, defCol.Width)
			}
		}

		if len(yamlView.Detail) != len(defView.Detail) {
			t.Errorf("%s: detail path count mismatch: yaml=%d, default=%d",
				name, len(yamlView.Detail), len(defView.Detail))
			continue
		}

		for i, defPath := range defView.Detail {
			if yamlView.Detail[i].String() != defPath.String() {
				t.Errorf("%s.Detail[%d]: yaml=%q, default=%q", name, i, yamlView.Detail[i].String(), defPath.String())
			}
		}
	}

	for name := range cfg.Views {
		if _, ok := defaults.Views[name]; !ok {
			t.Errorf("views dir has extra resource %q not in defaults.go", name)
		}
	}
}

func TestConfigInvalidYAML(t *testing.T) {
	dirs := []string{testdataDir("views", "invalid")}
	cfg, err := config.LoadFromDirs(dirs)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
	if cfg != nil {
		t.Fatal("expected nil config on parse error")
	}

	ec2 := config.GetViewDef(nil, "ec2")
	expected := config.DefaultViewDef("ec2")
	if len(ec2.List) != len(expected.List) {
		t.Fatalf("expected %d default ec2 columns after error, got %d", len(expected.List), len(ec2.List))
	}
}

func TestConfigYAMLParsing_KeyField(t *testing.T) {
	yamlData := `list:
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
	vd, err := config.ParseSingle([]byte(yamlData))
	if err != nil {
		t.Fatalf("ParseSingle failed: %v", err)
	}

	sqs := *vd
	if len(sqs.List) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(sqs.List))
	}

	if sqs.List[0].Path != "QueueUrl" {
		t.Errorf("col 0 path: got %q, want %q", sqs.List[0].Path, "QueueUrl")
	}
	if sqs.List[0].Key != "" {
		t.Errorf("col 0 key: got %q, want empty", sqs.List[0].Key)
	}

	if sqs.List[1].Key != "approx_messages" {
		t.Errorf("col 1 key: got %q, want %q", sqs.List[1].Key, "approx_messages")
	}
	if sqs.List[1].Path != "" {
		t.Errorf("col 1 path: got %q, want empty", sqs.List[1].Path)
	}

	if sqs.List[2].Key != "approx_not_visible" {
		t.Errorf("col 2 key: got %q, want %q", sqs.List[2].Key, "approx_not_visible")
	}
}

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

	wantSuffix := ".a9s"
	if !strings.HasSuffix(dir, wantSuffix) {
		t.Errorf("ConfigDir() = %q, expected it to end with %q", dir, wantSuffix)
	}

	expected := filepath.Join(home, ".a9s")
	if dir != expected {
		t.Errorf("ConfigDir() = %q, want %q", dir, expected)
	}
}

func TestConfigDir_RespectsEnvVar(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", "/tmp/custom-a9s")

	dir := config.ConfigDir()

	if dir != "/tmp/custom-a9s" {
		t.Errorf("ConfigDir() = %q, want %q", dir, "/tmp/custom-a9s")
	}
}

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

	if runtime.GOOS != "windows" {
		perm := info.Mode().Perm()
		if perm != 0700 {
			t.Errorf("directory permissions = %o, want %o", perm, 0700)
		}
	}
}

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

func TestConfigFilePath(t *testing.T) {
	base := filepath.Join(os.TempDir(), "a9s-test")
	t.Setenv("A9S_CONFIG_FOLDER", base)

	got := config.ConfigFilePath("views.yaml")
	want := filepath.Join(base, "views.yaml")
	if got != want {
		t.Errorf("ConfigFilePath(\"views.yaml\") = %q, want %q", got, want)
	}

	got = config.ConfigFilePath("keybindings.yaml")
	want = filepath.Join(base, "keybindings.yaml")
	if got != want {
		t.Errorf("ConfigFilePath(\"keybindings.yaml\") = %q, want %q", got, want)
	}
}

func TestLookupPaths_EnvVarDoesNotFallThrough(t *testing.T) {
	emptyDir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", emptyDir)

	// HOME points at a directory that does have ~/.a9s/views/, which Load() must
	// not read.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	homeA9s := filepath.Join(fakeHome, ".a9s")
	homeViews := filepath.Join(homeA9s, "views")
	if err := os.MkdirAll(homeViews, 0755); err != nil {
		t.Fatalf("failed to create fake home .a9s/views dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeViews, "ec2.yaml"),
		[]byte("list:\n  ShouldNotLoad:\n    path: sneaky\n    width: 99\n"), 0644); err != nil {
		t.Fatalf("failed to write ec2.yaml: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg != nil {
		ec2, ok := cfg.Views["ec2"]
		if ok && len(ec2.List) > 0 && ec2.List[0].Title == "ShouldNotLoad" {
			t.Fatalf("Load() fell through to ~/.a9s/views/ despite A9S_CONFIG_FOLDER being set (to empty dir)")
		}
		t.Fatalf("Load() returned non-nil config; expected (nil, nil) when env var dir has no file")
	}
}
