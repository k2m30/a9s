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

	"github.com/k2m30/a9s/v3/internal/resource"
)

// DefaultTTL is the default cache expiration duration.
const DefaultTTL = 1 * time.Hour

// Entry holds availability info for a single resource type.
type Entry struct {
	HasResources    bool   `yaml:"has_resources"`
	Count           int    `yaml:"count"`
	Truncated       bool   `yaml:"truncated,omitempty"`
	Error           string `yaml:"error,omitempty"`
	Issues          int    `yaml:"issues,omitempty"`           // issue-state resource count (red/yellow only)
	IssuesTruncated bool   `yaml:"issues_truncated,omitempty"` // true if issue count is lower bound
	IssuesKnown     bool   `yaml:"issues_known,omitempty"`     // true = probed (even if Issues=0); false = unknown
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
		s = strings.ReplaceAll(s, "\\", "_")
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
	// Strip resource keys that are not in the registry. This guards against
	// stale cache files from older versions containing unknown type keys.
	// Both ShortNames and Aliases are valid keys (aliases allow old cache files
	// written under a previous name to survive gracefully).
	if len(f.Resources) > 0 {
		known := make(map[string]bool)
		for _, rt := range resource.AllResourceTypes() {
			known[rt.ShortName] = true
			for _, alias := range rt.Aliases {
				known[alias] = true
			}
		}
		for key := range f.Resources {
			if !known[key] {
				delete(f.Resources, key)
			}
		}
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
	// Use CreateTemp so concurrent Save calls (two a9s processes, or two goroutines
	// targeting the same profile/region) don't race on a shared temp path.
	tmpFile, err := os.CreateTemp(dir, filepath.Base(p)+".tmp.*")
	if err != nil {
		return fmt.Errorf("creating cache temp file in %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing cache %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing cache %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, p); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming cache %s: %w", p, err)
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
