package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/core/config"
)

// ===========================================================================
// T023 — LoadAppConfig reads theme filename from config.yaml
// ===========================================================================

func TestLoadAppConfig_ReadsThemeFromConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)

	content := []byte("theme: \"dracula.yaml\"\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatalf("writing config.yaml: %v", err)
	}

	cfg, err := config.LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig: unexpected error: %v", err)
	}

	if cfg.Theme != "dracula.yaml" {
		t.Errorf("LoadAppConfig Theme: expected %q, got %q", "dracula.yaml", cfg.Theme)
	}
}

// ===========================================================================
// T024 — LoadAppConfig returns default when no config.yaml exists
// ===========================================================================

func TestLoadAppConfig_DefaultWhenNoConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)

	cfg, err := config.LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig: unexpected error when no config.yaml: %v", err)
	}

	if cfg.Theme != "" {
		t.Errorf("LoadAppConfig Theme: expected empty default, got %q", cfg.Theme)
	}
}

// ===========================================================================
// T025 — LoadAppConfig returns default when theme key is empty string
// ===========================================================================

func TestLoadAppConfig_EmptyThemeKeyReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)

	content := []byte("theme: \"\"\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), content, 0644); err != nil {
		t.Fatalf("writing config.yaml: %v", err)
	}

	cfg, err := config.LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig: unexpected error: %v", err)
	}

	if cfg.Theme != "" {
		t.Errorf("LoadAppConfig Theme: expected empty string, got %q", cfg.Theme)
	}
}

// ===========================================================================
// T026 — ThemePath rejects path traversal and absolute paths
// ===========================================================================

func TestThemePath_RejectsTraversalAndAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)

	errorCases := []string{
		"../etc/passwd",
		"/absolute/path.yaml",
		"foo/bar.yaml",
		"sub/dir/theme.yaml",
	}

	for _, input := range errorCases {
		_, err := config.ThemePath(input)
		if err == nil {
			t.Errorf("ThemePath(%q): expected error for unsafe path, got nil", input)
		}
	}
}

func TestThemePath_AcceptsSimpleFilename(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)

	got, err := config.ThemePath("dracula.yaml")
	if err != nil {
		t.Fatalf("ThemePath(\"dracula.yaml\"): unexpected error: %v", err)
	}

	expected := filepath.Join(dir, "themes", "dracula.yaml")
	if got != expected {
		t.Errorf("ThemePath(\"dracula.yaml\"): expected %q, got %q", expected, got)
	}
}

// ===========================================================================
// T039 — SaveTheme writes and overwrites config.yaml theme key
// ===========================================================================

func TestSaveTheme_WritesAndOverwritesConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)

	// First call: config.yaml must be created with the correct theme.
	if err := config.SaveTheme("dracula.yaml"); err != nil {
		t.Fatalf("SaveTheme(\"dracula.yaml\"): unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("reading config.yaml after first SaveTheme: %v", err)
	}
	if !strings.Contains(string(data), "dracula.yaml") {
		t.Errorf("config.yaml after SaveTheme(\"dracula.yaml\"): expected to contain %q, got:\n%s", "dracula.yaml", string(data))
	}

	// Second call with different theme: file must be updated in place, not appended.
	if saveErr := config.SaveTheme("nord.yaml"); saveErr != nil {
		t.Fatalf("SaveTheme(\"nord.yaml\"): unexpected error: %v", saveErr)
	}

	data, err = os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("reading config.yaml after second SaveTheme: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "nord.yaml") {
		t.Errorf("config.yaml after SaveTheme(\"nord.yaml\"): expected to contain %q, got:\n%s", "nord.yaml", content)
	}
	// Old theme must not remain in the file.
	if strings.Contains(content, "dracula.yaml") {
		t.Errorf("config.yaml after SaveTheme(\"nord.yaml\"): still contains old value %q:\n%s", "dracula.yaml", content)
	}
}

// ===========================================================================
// T058 — ThemePath returns error when ConfigDir is empty
// ===========================================================================

func TestThemePath_ErrorWhenConfigDirEmpty(t *testing.T) {
	// Force ConfigDir() to return "" by unsetting both HOME and A9S_CONFIG_FOLDER.
	// On Windows, os.UserHomeDir() uses USERPROFILE, not HOME.
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("A9S_CONFIG_FOLDER", "")

	_, err := config.ThemePath("dracula.yaml")
	if err == nil {
		t.Fatal("ThemePath: expected error when ConfigDir is empty, got nil")
	}
}

// ===========================================================================
// T059 — SaveTheme returns error when ConfigDir is empty
// ===========================================================================

func TestSaveTheme_ErrorWhenConfigDirEmpty(t *testing.T) {
	// Force ConfigDir() to return "" by unsetting both HOME and A9S_CONFIG_FOLDER.
	// On Windows, os.UserHomeDir() uses USERPROFILE, not HOME.
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Setenv("A9S_CONFIG_FOLDER", "")

	err := config.SaveTheme("dracula.yaml")
	if err == nil {
		t.Fatal("SaveTheme: expected error when ConfigDir is empty, got nil")
	}
}

// ===========================================================================
// Data-loss regression: SaveTheme on a corrupt existing config.yaml
// ===========================================================================

// TestSaveTheme_CorruptExistingConfig_ReturnsErrorAndDoesNotDestroyFile pins
// the fix for the swallowed-parse-error data-loss path in
// config.SaveTheme (core/config/appconfig.go): today, when config.yaml exists
// but is not valid YAML, `_ = yaml.Unmarshal(existing, &data)` discards the
// parse error, `data` stays nil, and the function rebuilds the map from
// scratch with ONLY the theme key before rewriting config.yaml — silently
// returning nil and destroying every other key the file had. This RED test
// asserts the corrected contract: a corrupt existing config.yaml must cause
// SaveTheme to return an error, and the file on disk must be left untouched.
func TestSaveTheme_CorruptExistingConfig_ReturnsErrorAndDoesNotDestroyFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)

	corrupt := []byte("[unterminated")
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatalf("writing corrupt config.yaml: %v", err)
	}

	err := config.SaveTheme("dracula.yaml")
	if err == nil {
		t.Error("SaveTheme on a corrupt existing config.yaml: expected error, got nil — the parse failure is being silently swallowed")
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading config.yaml after SaveTheme: %v", readErr)
	}
	if string(after) != string(corrupt) {
		t.Errorf("config.yaml was rewritten despite the parse error: before=%q after=%q — SaveTheme must not touch the file when it cannot safely preserve the existing keys", corrupt, after)
	}
}

// TestSaveTheme_ValidConfigWithExtraKeys_PreservesExtraKeys is a boundary pin,
// not a regression test for a live bug: the swallowed-error path in SaveTheme
// only misbehaves when the EXISTING file fails to parse (see
// TestSaveTheme_CorruptExistingConfig_ReturnsErrorAndDoesNotDestroyFile).
// When the existing config.yaml is valid YAML with keys beyond "theme",
// yaml.Unmarshal succeeds, data retains those keys, and only "theme" is
// overwritten — this already passes today. Kept as a pin so a future
// SaveTheme rewrite (e.g. switching AppConfig to a typed round-trip) cannot
// silently regress key preservation.
func TestSaveTheme_ValidConfigWithExtraKeys_PreservesExtraKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)

	initial := []byte("theme: dracula.yaml\nprofile: prod-readonly\nview_mode: table\n")
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, initial, 0600); err != nil {
		t.Fatalf("writing initial config.yaml: %v", err)
	}

	if err := config.SaveTheme("nord.yaml"); err != nil {
		t.Fatalf("SaveTheme(\"nord.yaml\"): unexpected error: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config.yaml after SaveTheme: %v", err)
	}

	var data map[string]any
	if err := yaml.Unmarshal(after, &data); err != nil {
		t.Fatalf("config.yaml after SaveTheme is not valid YAML: %v\ncontent:\n%s", err, after)
	}
	if data["theme"] != "nord.yaml" {
		t.Errorf("config.yaml theme key = %v, want %q", data["theme"], "nord.yaml")
	}
	if data["profile"] != "prod-readonly" {
		t.Errorf("config.yaml lost the pre-existing 'profile' key: got %v, want %q", data["profile"], "prod-readonly")
	}
	if data["view_mode"] != "table" {
		t.Errorf("config.yaml lost the pre-existing 'view_mode' key: got %v, want %q", data["view_mode"], "table")
	}
}

// ===========================================================================
// Atomicity: SaveTheme must not truncate-write config.yaml in place
// ===========================================================================

// statFile stats path and returns its os.FileInfo, for a later os.SameFile
// comparison. A write that goes through the standard atomic-replace pattern
// (write to a temp file, then rename over the target) always produces a
// distinct underlying file from the caller's perspective; an in-place
// os.WriteFile (open O_TRUNC, write, close) always keeps the same one.
// os.SameFile does this comparison portably (inode on Unix, volume+file-index
// on Windows), without any production-side test hooks.
func statFile(t *testing.T, path string) os.FileInfo {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%s): %v", path, err)
	}
	return fi
}

// TestSaveTheme_ExistingFile_WriteIsAtomicByRename pins the fix for
// SaveTheme's non-atomic write (core/config/appconfig.go: `return
// os.WriteFile(path, out, 0600)`). A direct os.WriteFile truncates and
// rewrites config.yaml in place, so a process crash or a concurrent reader
// mid-write can observe (or be left with) a half-written file. This RED test
// asserts the corrected contract: SaveTheme replaces config.yaml via a
// temp-file-then-rename, observable as an inode change on the destination
// path. Today the write is in place, so the inode is unchanged — RED.
func TestSaveTheme_ExistingFile_WriteIsAtomicByRename(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", dir)
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("theme: dracula.yaml\n"), 0600); err != nil {
		t.Fatalf("writing initial config.yaml: %v", err)
	}
	before := statFile(t, path)

	if err := config.SaveTheme("nord.yaml"); err != nil {
		t.Fatalf("SaveTheme(\"nord.yaml\"): unexpected error: %v", err)
	}
	after := statFile(t, path)

	if os.SameFile(before, after) {
		t.Errorf("config.yaml is still the same underlying file after SaveTheme — the write is not atomic (in-place truncate+write instead of temp-file+rename), so a crash mid-write can corrupt config.yaml")
	}
}
