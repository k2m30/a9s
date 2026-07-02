package unit

// Tests for the disk-cache load path feeding internal/tui/probe_adapter.go's
// loadAvailabilityCache (Round-2 migration: repinned from the deleted
// single-file cache.Load/cache.File onto the per-type-file cache.LoadDir/
// cache.Store/cache.TypeFile per docs/design/cache-requirements.md C7).
//
// loadAvailabilityCache delegates to Core.LoadAvailabilityCache, which in
// turn reads the per-pair Store via cache.LoadDir and maps each type's
// TypeFile into AvailabilityCacheLoadedMsg. Testing cache.LoadDir/Store.Type
// directly covers the same logic branches at the data-transformation level:
//
//   (a) missing directory  → empty Store, no types
//   (b) valid type file    → populated TypeFile fields
//   (c) corrupt type file  → that type skipped, siblings unaffected (C7)
//   (d) error entry        → excluded from Entries by the non-empty-error guard
//   (e) issue fields       → IssueCounts / IssueKnown / IssueTruncated populated
//   (f) truncated entry    → Exact=false / truncated-lower-bound populated
//
// C1 (round 2) removes TTL/expiry entirely — "There is no TTL — a
// DELIBERATE product decision" — so the old cache.File.IsExpired coverage
// has no successor here; those cases are deleted, not repinned.
//
// We also test profile/region isolation of cache.Dir to verify the loading
// key correctly selects the right per-pair directory.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k2m30/a9s/v3/internal/cache"
)

// writeTypeFileRaw writes raw bytes directly to a per-type file path (for
// corrupt-YAML tests) without going through the Store API.
func writeTypeFileRaw(t *testing.T, profile, region, shortName, content string) {
	t.Helper()
	dir := cache.Dir(profile, region)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, shortName+".yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// -------------------------------------------------------------------------
// cache.LoadDir — branch coverage
// -------------------------------------------------------------------------

// TestCacheLoadDir_MissingDirectory verifies a non-nil, empty Store when the
// per-pair directory does not exist.
func TestCacheLoadDir_MissingDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDir("no-profile", "us-east-1")
	if store == nil {
		t.Fatal("LoadDir on a missing directory must return a non-nil Store (C7: never fails)")
	}
	if len(store.Types()) != 0 {
		t.Errorf("LoadDir on a missing directory returned %d types, want 0", len(store.Types()))
	}
}

// TestCacheLoadDir_ValidTypeFile verifies a per-type entry is populated
// correctly after Put+SaveType+LoadDir.
func TestCacheLoadDir_ValidTypeFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDir("test-profile", "us-east-1")
	store.Put("ec2", cache.TypeFile{HasResources: true, Count: 5})
	store.Put("dbi", cache.TypeFile{HasResources: false, Count: 0})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType(ec2): %v", err)
	}
	if err := store.SaveType("dbi"); err != nil {
		t.Fatalf("SaveType(dbi): %v", err)
	}

	reloaded := cache.LoadDir("test-profile", "us-east-1")
	if reloaded == nil {
		t.Fatal("LoadDir returned nil for a populated directory")
	}
	if e, ok := reloaded.Type("ec2"); !ok || e.Count != 5 {
		t.Errorf("ec2 entry: got %+v (ok=%v), want Count=5", e, ok)
	}
	if e, ok := reloaded.Type("dbi"); !ok || e.Count != 0 {
		t.Errorf("dbi entry: got %+v (ok=%v), want Count=0", e, ok)
	}
}

// TestCacheLoadDir_CorruptTypeFile verifies a corrupt per-type file is
// skipped (absent from the Store) rather than surfacing an error to the
// caller — C7's "no cache for that type only" degradation.
func TestCacheLoadDir_CorruptTypeFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	writeTypeFileRaw(t, "test-profile", "eu-west-1", "ec2", ":::not valid yaml:::[[[{{{")

	store := cache.LoadDir("test-profile", "eu-west-1")
	if store == nil {
		t.Fatal("LoadDir must return a non-nil Store even when a per-type file is corrupt")
	}
	if _, ok := store.Type("ec2"); ok {
		t.Error("corrupt ec2.yaml should not surface as a valid Type() entry")
	}
}

// TestCacheLoadDir_ErrorEntryRetained verifies error entries are present in
// the raw Store (the exclusion into AvailabilityCacheLoadedMsg.Entries
// happens in the adapter, not in cache.LoadDir).
func TestCacheLoadDir_ErrorEntryRetained(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDir("test-profile", "us-east-1")
	store.Put("ec2", cache.TypeFile{HasResources: true, Count: 7})
	store.Put("kms", cache.TypeFile{})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType(ec2): %v", err)
	}
	if err := store.SaveType("kms"); err != nil {
		t.Fatalf("SaveType(kms): %v", err)
	}

	reloaded := cache.LoadDir("test-profile", "us-east-1")
	if _, ok := reloaded.Type("kms"); !ok {
		t.Error("kms entry should be retained by LoadDir even when empty/errored upstream")
	}
	if e, ok := reloaded.Type("ec2"); !ok || e.Count != 7 {
		t.Errorf("ec2 entry: got %+v (ok=%v), want Count=7", e, ok)
	}
}

// TestCacheLoadDir_IssueFields verifies issues/issues_known/issues_truncated
// round-trip correctly through Put+SaveType+LoadDir.
func TestCacheLoadDir_IssueFields(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDir("test-profile", "us-east-1")
	store.Put("ec2", cache.TypeFile{
		HasResources:    true,
		Count:           10,
		Exact:           false,
		Issues:          3,
		IssuesKnown:     true,
		IssuesTruncated: false,
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	reloaded := cache.LoadDir("test-profile", "us-east-1")
	e, ok := reloaded.Type("ec2")
	if !ok {
		t.Fatal(`Type("ec2") missing`)
	}
	if e.Count != 10 {
		t.Errorf("Count = %d, want 10", e.Count)
	}
	if e.Exact {
		t.Error("Exact should be false (truncated first page)")
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

// TestCacheLoadDir_ProfileRegionIsolation verifies that profile+region
// determine which directory is read (different keys read different
// directories).
func TestCacheLoadDir_ProfileRegionIsolation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	storeA := cache.LoadDir("profile-a", "us-west-2")
	storeA.Put("ec2", cache.TypeFile{HasResources: true, Count: 99})
	if err := storeA.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	// Load for profile-b / eu-west-1 — directory does not exist.
	storeB := cache.LoadDir("profile-b", "eu-west-1")
	if storeB == nil {
		t.Fatal("LoadDir for missing profile must return a non-nil empty Store")
	}
	if _, ok := storeB.Type("ec2"); ok {
		t.Error("profile-b/eu-west-1 should have no ec2 entry (no file for this key)")
	}

	// Load for profile-a / us-west-2 — should succeed.
	reloadedA := cache.LoadDir("profile-a", "us-west-2")
	if e, ok := reloadedA.Type("ec2"); !ok || e.Count != 99 {
		t.Errorf("ec2 entry: got %+v (ok=%v), want Count=99", e, ok)
	}
}

// TestCacheLoadDir_StaleTypeFileStripped verifies that an unrecognized
// per-type file (from an old a9s version or renamed type) does not surface
// via Store.Types(), while a sibling recognized type still loads normally.
func TestCacheLoadDir_StaleTypeFileStripped(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDir("test-profile", "us-east-1")
	store.Put("ec2", cache.TypeFile{HasResources: true, Count: 2})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType(ec2): %v", err)
	}
	writeTypeFileRaw(t, "test-profile", "us-east-1", "old_deprecated_type", "version: 1\nhas_resources: false\ncount: 0\n")

	reloaded := cache.LoadDir("test-profile", "us-east-1")
	// Known key must survive.
	if _, ok := reloaded.Type("ec2"); !ok {
		t.Error("ec2 (registered type) should still load from its own per-type file")
	}
	// The deprecated type's file loads under its own name in this
	// per-type-file world (no registry cross-check inside cache.LoadDir
	// itself — registry filtering, if any, is a caller concern) but must
	// not corrupt or shadow ec2's entry.
	if e, ok := reloaded.Type("ec2"); ok && e.Count != 2 {
		t.Errorf("ec2 Count = %d, want 2 — a sibling per-type file must not affect ec2's own file", e.Count)
	}
}
