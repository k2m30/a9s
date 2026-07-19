// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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

	// Read existing config to preserve other keys. A missing file is fine
	// (first run creates config.yaml). Any other read error, or a file that
	// exists but fails to parse, means we cannot safely preserve the
	// existing keys — return before touching the file rather than silently
	// discarding it.
	data := make(map[string]any)
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if unmarshalErr := yaml.Unmarshal(existing, &data); unmarshalErr != nil {
			return fmt.Errorf("parsing existing config %s: %w", path, unmarshalErr)
		}
	case !os.IsNotExist(err):
		return fmt.Errorf("reading existing config %s: %w", path, err)
	}
	data["theme"] = filename

	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Atomic replace: write to a temp file in the same directory, then
	// rename over the target, matching core/cache.Store.SaveType's pattern.
	tmpFile, err := os.CreateTemp(dir, "config.yaml.tmp.*")
	if err != nil {
		return fmt.Errorf("creating config temp file in %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(out); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing config %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing config %s: %w", tmpPath, err)
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("setting permissions on config %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming config %s: %w", path, err)
	}
	return nil
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
