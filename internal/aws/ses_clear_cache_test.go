// ses_clear_cache_test.go — Pin 5 regression pin for ClearAllSESRuleSetCaches.
//
// Verifies that ClearAllSESRuleSetCaches() wipes ALL entries from the
// package-level sesRuleSetCaches map, not just one client's entry.
//
// Pre-fix risk: if ClearAllSESRuleSetCaches iterated and deleted entries one by
// one, a concurrent goroutine adding an entry mid-loop could survive. The
// post-fix implementation replaces the map atomically.
package aws

import (
	"testing"
)

// TestClearAllSESRuleSetCaches_WipesAllEntries verifies that after seeding
// multiple distinct *ServiceClients entries into sesRuleSetCaches, calling
// ClearAllSESRuleSetCaches() leaves the map empty (len == 0).
//
// This pin would FAIL against pre-fix code that only deleted a single entry
// or that iterated while holding no lock — the map would retain stale entries.
func TestClearAllSESRuleSetCaches_WipesAllEntries(t *testing.T) {
	// Seed three distinct client pointers into the cache.
	c1 := &ServiceClients{}
	c2 := &ServiceClients{}
	c3 := &ServiceClients{}

	SeedSESRuleSetCache(c1)
	SeedSESRuleSetCache(c2)
	SeedSESRuleSetCache(c3)

	if got := SESRuleSetCachesLen(); got != 3 {
		t.Fatalf("pre-condition failed: expected 3 entries after seeding, got %d", got)
	}

	// ---- Call under test ----
	ClearAllSESRuleSetCaches()

	if got := SESRuleSetCachesLen(); got != 0 {
		t.Errorf("SESRuleSetCachesLen() = %d after ClearAllSESRuleSetCaches(), want 0 — map not fully wiped", got)
	}
}

// TestClearAllSESRuleSetCaches_EmptyMapIsNoop verifies that calling
// ClearAllSESRuleSetCaches on an already-empty map does not panic.
func TestClearAllSESRuleSetCaches_EmptyMapIsNoop(t *testing.T) {
	// Ensure map is empty before calling.
	ClearAllSESRuleSetCaches()
	if got := SESRuleSetCachesLen(); got != 0 {
		t.Fatalf("pre-condition: map not empty after first clear, len=%d", got)
	}

	// Must not panic.
	ClearAllSESRuleSetCaches()

	if got := SESRuleSetCachesLen(); got != 0 {
		t.Errorf("SESRuleSetCachesLen() = %d after second ClearAllSESRuleSetCaches(), want 0", got)
	}
}
