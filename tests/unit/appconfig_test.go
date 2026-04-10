package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
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
	origHome, homeSet := os.LookupEnv("HOME")
	os.Unsetenv("HOME")
	defer func() {
		if homeSet {
			os.Setenv("HOME", origHome)
		}
	}()
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
	origHome, homeSet := os.LookupEnv("HOME")
	os.Unsetenv("HOME")
	defer func() {
		if homeSet {
			os.Setenv("HOME", origHome)
		}
	}()
	t.Setenv("A9S_CONFIG_FOLDER", "")

	err := config.SaveTheme("dracula.yaml")
	if err == nil {
		t.Fatal("SaveTheme: expected error when ConfigDir is empty, got nil")
	}
}
