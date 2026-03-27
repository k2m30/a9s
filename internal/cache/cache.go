// Package cache provides resource availability persistence for the a9s TUI.
// It stores which resource types have resources in a per-profile+region YAML file
// under ~/.a9s/cache/, enabling instant grey-out of empty resource types on startup.
package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultTTL is the default cache expiration duration.
const DefaultTTL = 1 * time.Hour

// Entry holds availability info for a single resource type.
type Entry struct {
	HasResources bool   `yaml:"has_resources"`
	Count        int    `yaml:"count"`
	Error        string `yaml:"error,omitempty"`
}

// File is the on-disk cache structure.
type File struct {
	Profile   string           `yaml:"profile"`
	Region    string           `yaml:"region"`
	CheckedAt time.Time        `yaml:"checked_at"`
	Resources map[string]Entry `yaml:"resources"`
}

// Dir returns the cache directory path (~/.a9s/cache/).
func Dir() string {
	if folder := os.Getenv("A9S_CONFIG_FOLDER"); folder != "" {
		return filepath.Join(folder, "cache")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".a9s", "cache")
	}
	return ""
}

// Path returns the full path to the cache file for a profile+region combination.
// Uses double-dash separator: <profile>--<region>.yaml
func Path(profile, region string) string {
	dir := Dir()
	if dir == "" {
		return ""
	}
	// Sanitize profile/region for filenames (replace / and spaces)
	safe := func(s string) string {
		s = strings.ReplaceAll(s, "/", "_")
		s = strings.ReplaceAll(s, " ", "_")
		return s
	}
	return filepath.Join(dir, safe(profile)+"--"+safe(region)+".yaml")
}

// Load reads and parses the cache file for the given profile+region.
// Returns (nil, nil) if the file does not exist.
// Returns (nil, err) if the file exists but cannot be parsed.
func Load(profile, region string) (*File, error) {
	p := Path(profile, region)
	if p == "" {
		return nil, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading cache %s: %w", p, err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parsing cache %s: %w", p, err)
	}
	return &f, nil
}

// Save writes the cache file to disk, creating the cache directory if needed.
func Save(f *File) error {
	if f == nil {
		return nil
	}
	p := Path(f.Profile, f.Region)
	if p == "" {
		return fmt.Errorf("cannot determine cache path")
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating cache directory %s: %w", dir, err)
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshaling cache: %w", err)
	}
	if err := os.WriteFile(p, data, 0600); err != nil {
		return fmt.Errorf("writing cache %s: %w", p, err)
	}
	return nil
}

// IsExpired returns true if the cache is older than the given TTL,
// or if CheckedAt is zero (never set).
func (f *File) IsExpired(ttl time.Duration) bool {
	if f == nil || f.CheckedAt.IsZero() {
		return true
	}
	return time.Since(f.CheckedAt) > ttl
}
