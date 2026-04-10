package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AppConfig is the top-level ~/.a9s/config.yaml structure.
type AppConfig struct {
	Theme string `yaml:"theme"`
}

// DefaultAppConfig returns an AppConfig with built-in defaults.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		Theme: "",
	}
}

// LoadAppConfig reads ~/.a9s/config.yaml (or $A9S_CONFIG_FOLDER/config.yaml).
// Returns DefaultAppConfig if the file does not exist.
// Returns an error only if the file exists but cannot be parsed.
func LoadAppConfig() (AppConfig, error) {
	path := ConfigFilePath("config.yaml")
	if path == "" {
		return DefaultAppConfig(), nil
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultAppConfig(), nil
	}
	if err != nil {
		return DefaultAppConfig(), fmt.Errorf("reading app config %s: %w", path, err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultAppConfig(), fmt.Errorf("parsing app config %s: %w", path, err)
	}

	return cfg, nil
}

// SaveTheme persists the theme filename to the user-global config.yaml.
// Creates the file if it doesn't exist. Updates only the theme key,
// preserving other keys.
func SaveTheme(filename string) error {
	dir := ConfigDir()
	if dir == "" {
		return fmt.Errorf("cannot determine config directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "config.yaml")

	// Read existing config to preserve other keys.
	var data map[string]any
	if existing, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(existing, &data)
	}
	if data == nil {
		data = make(map[string]any)
	}
	data["theme"] = filename

	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, out, 0600)
}

// ThemePath resolves a theme filename to an absolute file path within the themes directory.
// The name must be a plain filename including extension (e.g. "dracula.yaml").
// Absolute paths, paths containing directory separators, and traversal attempts are rejected.
func ThemePath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("theme name must not be empty")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid theme filename %q: absolute paths not allowed", name)
	}
	if name != filepath.Base(name) {
		return "", fmt.Errorf("invalid theme filename %q: must be a plain filename with no path separators", name)
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid theme filename %q: must not contain \"..\"", name)
	}
	dir := ConfigDir()
	if dir == "" {
		return "", fmt.Errorf("cannot determine config directory")
	}
	return filepath.Join(dir, "themes", name), nil
}
