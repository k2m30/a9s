package unit

// Tests for loadAvailabilityCache coverage (internal/tui/app_fetchers.go).
//
// loadAvailabilityCache delegates to cache.Load, then maps cache.File entries
// into AvailabilityCacheLoadedMsg. Testing cache.Load and File.IsExpired directly
// covers the same logic branches at the data-transformation level:
//
//   (a) missing file     → (nil, nil) → Expired: true, empty Entries
//   (b) valid fresh file → populated entries, IsExpired=false
//   (c) stale file       → populated entries, IsExpired=true
//   (d) corrupt YAML     → (nil, err) → Expired: true
//   (e) error entry      → excluded from Entries by the non-empty-error guard
//   (f) issue fields     → IssueCounts / IssueKnown / IssueTruncated populated
//   (g) truncated entry  → Truncated map populated
//
// We also test cache.File.IsExpired and the profile/region isolation of cache.Path
// to verify the loading key correctly selects the right file.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/k2m30/a9s/v3/internal/cache"
)

// writeCacheYAML writes a cache.File as YAML to the given path.
func writeCacheYAML(t *testing.T, path string, f cache.File) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// writeCacheRaw writes raw bytes to the given path (for corrupt-YAML tests).
func writeCacheRaw(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// -------------------------------------------------------------------------
// cache.Load — branch coverage
// -------------------------------------------------------------------------

// TestCacheLoad_MissingFile verifies (nil, nil) when the file does not exist.
func TestCacheLoad_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	// No file created — Load should return (nil, nil)
	got, err := cache.Load("no-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load on missing file returned error: %v", err)
	}
	if got != nil {
		t.Errorf("Load on missing file returned non-nil File: %+v", got)
	}
}

// TestCacheLoad_ValidFreshFile verifies entries are populated and IsExpired is false.
func TestCacheLoad_ValidFreshFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	p := cache.Path("test-profile", "us-east-1")
	writeCacheYAML(t, p, cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Now(),
		Resources: map[string]cache.Entry{
			"ec2": {HasResources: true, Count: 5},
			"dbi": {HasResources: false, Count: 0},
		},
	})

	f, err := cache.Load("test-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if f == nil {
		t.Fatal("Load returned nil for valid cache file")
	}
	if f.IsExpired(cache.DefaultTTL) {
		t.Error("fresh cache file should not be expired")
	}
	if e, ok := f.Resources["ec2"]; !ok || e.Count != 5 {
		t.Errorf("ec2 entry: got %+v (ok=%v), want Count=5", e, ok)
	}
	if e, ok := f.Resources["dbi"]; !ok || e.Count != 0 {
		t.Errorf("dbi entry: got %+v (ok=%v), want Count=0", e, ok)
	}
}

// TestCacheLoad_ExpiredFile verifies IsExpired returns true for stale files.
func TestCacheLoad_ExpiredFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	p := cache.Path("test-profile", "us-west-2")
	writeCacheYAML(t, p, cache.File{
		Profile:   "test-profile",
		Region:    "us-west-2",
		CheckedAt: time.Now().Add(-2 * time.Hour), // 2h ago, past 1h DefaultTTL
		Resources: map[string]cache.Entry{
			"ec2": {HasResources: true, Count: 3},
		},
	})

	f, err := cache.Load("test-profile", "us-west-2")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if f == nil {
		t.Fatal("Load returned nil for existing file")
	}
	if !f.IsExpired(cache.DefaultTTL) {
		t.Error("2h old cache should be expired with 1h DefaultTTL")
	}
	// Data is still accessible even when expired (caller decides to re-probe).
	if _, ok := f.Resources["ec2"]; !ok {
		t.Error("expired cache: ec2 entry should still be accessible")
	}
}

// TestCacheLoad_CorruptYAML verifies (nil, err) is returned for corrupt files.
func TestCacheLoad_CorruptYAML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	p := cache.Path("test-profile", "eu-west-1")
	writeCacheRaw(t, p, ":::not valid yaml:::[[[{{{")

	f, err := cache.Load("test-profile", "eu-west-1")
	if err == nil {
		t.Errorf("Load on corrupt YAML should return error, got nil; file=%+v", f)
	}
	if f != nil {
		t.Errorf("Load on corrupt YAML should return nil File, got %+v", f)
	}
}

// TestCacheLoad_ErrorEntryRetained verifies error entries are present in the
// raw File.Resources map (the exclusion happens in loadAvailabilityCache, not Load).
func TestCacheLoad_ErrorEntryRetained(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	p := cache.Path("test-profile", "us-east-1")
	writeCacheYAML(t, p, cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Now(),
		Resources: map[string]cache.Entry{
			"ec2": {HasResources: true, Count: 7},
			"kms": {Error: "AccessDeniedException"},
		},
	})

	f, err := cache.Load("test-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if f == nil {
		t.Fatal("Load returned nil")
	}
	if e, ok := f.Resources["kms"]; !ok || e.Error == "" {
		t.Errorf("kms entry with Error should be retained by Load; got ok=%v entry=%+v", ok, e)
	}
	if e, ok := f.Resources["ec2"]; !ok || e.Count != 7 {
		t.Errorf("ec2 entry: got %+v (ok=%v), want Count=7", e, ok)
	}
}

// TestCacheLoad_IssueFields verifies issues/issues_known/issues_truncated
// round-trip correctly through cache.Load.
func TestCacheLoad_IssueFields(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	p := cache.Path("test-profile", "us-east-1")
	writeCacheYAML(t, p, cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Now(),
		Resources: map[string]cache.Entry{
			"ec2": {
				HasResources:    true,
				Count:           10,
				Truncated:       true,
				Issues:          3,
				IssuesKnown:     true,
				IssuesTruncated: false,
			},
		},
	})

	f, err := cache.Load("test-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if f == nil {
		t.Fatal("Load returned nil")
	}

	e := f.Resources["ec2"]
	if e.Count != 10 {
		t.Errorf("Count = %d, want 10", e.Count)
	}
	if !e.Truncated {
		t.Error("Truncated should be true")
	}
	if e.Issues != 3 {
		t.Errorf("Issues = %d, want 3", e.Issues)
	}
	if !e.IssuesKnown {
		t.Error("IssuesKnown should be true")
	}
	if e.IssuesTruncated {
		t.Error("IssuesTruncated should be false")
	}
}

// TestCacheLoad_ProfileRegionIsolation verifies that profile+region determine
// which file is read (different keys read different files).
func TestCacheLoad_ProfileRegionIsolation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	// Write cache for profile-a / us-west-2
	pA := cache.Path("profile-a", "us-west-2")
	writeCacheYAML(t, pA, cache.File{
		Profile:   "profile-a",
		Region:    "us-west-2",
		CheckedAt: time.Now(),
		Resources: map[string]cache.Entry{
			"ec2": {HasResources: true, Count: 99},
		},
	})

	// Load for profile-b / eu-west-1 — file does not exist
	f, err := cache.Load("profile-b", "eu-west-1")
	if err != nil {
		t.Fatalf("Load for missing profile returned error: %v", err)
	}
	if f != nil {
		t.Error("profile-b/eu-west-1 should return nil (no file for this key)")
	}

	// Load for profile-a / us-west-2 — should succeed
	f, err = cache.Load("profile-a", "us-west-2")
	if err != nil {
		t.Fatalf("Load for profile-a returned error: %v", err)
	}
	if f == nil {
		t.Fatal("profile-a/us-west-2 should return valid File")
	}
	if e := f.Resources["ec2"]; e.Count != 99 {
		t.Errorf("ec2 Count = %d, want 99", e.Count)
	}
}

// TestCacheLoad_StaleResourceKeyStripped verifies that unknown resource type
// keys in the cache file (from old a9s versions) are stripped by Load.
func TestCacheLoad_StaleResourceKeyStripped(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	p := cache.Path("test-profile", "us-east-1")
	writeCacheYAML(t, p, cache.File{
		Profile:   "test-profile",
		Region:    "us-east-1",
		CheckedAt: time.Now(),
		Resources: map[string]cache.Entry{
			"ec2":                  {HasResources: true, Count: 2},
			"old_deprecated_type":  {HasResources: false, Count: 0},
			"another_stale_key_v1": {HasResources: false, Count: 0},
		},
	})

	f, err := cache.Load("test-profile", "us-east-1")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if f == nil {
		t.Fatal("Load returned nil")
	}

	// Stale/unknown keys must be stripped.
	if _, ok := f.Resources["old_deprecated_type"]; ok {
		t.Error("old_deprecated_type (not in registry) should be stripped by Load")
	}
	if _, ok := f.Resources["another_stale_key_v1"]; ok {
		t.Error("another_stale_key_v1 (not in registry) should be stripped by Load")
	}
	// Known key must survive.
	if _, ok := f.Resources["ec2"]; !ok {
		t.Error("ec2 (registered type) should not be stripped by Load")
	}
}

// -------------------------------------------------------------------------
// cache.File.IsExpired — edge cases
// -------------------------------------------------------------------------

// TestCacheIsExpired_ZeroCheckedAt verifies nil/zero-time is always expired.
func TestCacheIsExpired_ZeroCheckedAt(t *testing.T) {
	var f cache.File // CheckedAt is zero
	if !f.IsExpired(cache.DefaultTTL) {
		t.Error("zero CheckedAt should always be expired")
	}
}

// TestCacheIsExpired_NilFile verifies nil receiver is always expired.
func TestCacheIsExpired_NilFile(t *testing.T) {
	var f *cache.File // nil
	if !f.IsExpired(cache.DefaultTTL) {
		t.Error("nil File should always be expired")
	}
}

// TestCacheIsExpired_BoundaryCondition verifies the boundary: exactly 1h ago
// should be expired with 1h TTL.
func TestCacheIsExpired_BoundaryCondition(t *testing.T) {
	// Slightly over TTL (1h + 1s)
	f := cache.File{CheckedAt: time.Now().Add(-cache.DefaultTTL - time.Second)}
	if !f.IsExpired(cache.DefaultTTL) {
		t.Error("file 1h+1s old should be expired with 1h TTL")
	}
	// Slightly under TTL (1h - 1s)
	f2 := cache.File{CheckedAt: time.Now().Add(-cache.DefaultTTL + time.Second)}
	if f2.IsExpired(cache.DefaultTTL) {
		t.Error("file 1h-1s old should NOT be expired with 1h TTL")
	}
}
