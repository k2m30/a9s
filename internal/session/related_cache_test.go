package session_test

import (
	"fmt"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
)

// testRelatedResults returns a non-nil slice of RelatedCacheResult for use as
// a cache value.
func testRelatedResults(count int) []session.RelatedCacheResult {
	return []session.RelatedCacheResult{{
		DefDisplayName: "",
		Result:         resource.RelatedCheckResult{TargetType: "ec2", Count: count},
	}}
}

// TestRelatedCacheLRU_CapEnforced verifies that inserting cap+1 entries evicts
// the oldest entry so the cache never exceeds its capacity.
func TestRelatedCacheLRU_CapEnforced(t *testing.T) {
	t.Parallel()

	const cap = 500
	c := session.NewRelatedCacheLRU(cap)

	for i := 0; i <= cap; i++ {
		c.Set(fmt.Sprintf("key-%d", i), testRelatedResults(i))
	}

	got := c.Len()
	if got != cap {
		t.Errorf("expected len %d after inserting %d entries into cap=%d cache, got %d", cap, cap+1, cap, got)
	}
}

// TestRelatedCacheLRU_LRUEviction verifies that the least-recently-used entry
// is evicted when the cache is full, and that a recent Get() promotes an entry.
func TestRelatedCacheLRU_LRUEviction(t *testing.T) {
	t.Parallel()

	c := session.NewRelatedCacheLRU(3)

	c.Set("a", testRelatedResults(1))
	c.Set("b", testRelatedResults(2))
	c.Set("c", testRelatedResults(3))

	// Access "a" to promote it to most recently used.
	if _, ok := c.Get("a"); !ok {
		t.Fatal("expected 'a' to exist before eviction test")
	}

	// Insert "d" — should evict "b" (least recently used).
	c.Set("d", testRelatedResults(4))

	if _, ok := c.Get("a"); !ok {
		t.Error("expected 'a' to still exist after eviction")
	}
	if _, ok := c.Get("b"); ok {
		t.Error("expected 'b' to have been evicted (LRU)")
	}
	if _, ok := c.Get("c"); !ok {
		t.Error("expected 'c' to still exist after eviction")
	}
	if _, ok := c.Get("d"); !ok {
		t.Error("expected 'd' to exist after insertion")
	}
}

// TestRelatedCacheLRU_Clear verifies that Clear() removes all entries and
// resets the length to zero.
func TestRelatedCacheLRU_Clear(t *testing.T) {
	t.Parallel()

	c := session.NewRelatedCacheLRU(10)

	for i := range 5 {
		c.Set(fmt.Sprintf("k%d", i), testRelatedResults(i))
	}

	c.Clear()

	if got := c.Len(); got != 0 {
		t.Errorf("expected len 0 after Clear(), got %d", got)
	}
}

// TestRelatedCacheLRU_Delete verifies that Delete() removes a specific key
// while leaving other entries intact.
func TestRelatedCacheLRU_Delete(t *testing.T) {
	t.Parallel()

	c := session.NewRelatedCacheLRU(10)

	c.Set("x", testRelatedResults(1))
	c.Set("y", testRelatedResults(2))
	c.Set("z", testRelatedResults(3))

	c.Delete("y")

	if _, ok := c.Get("y"); ok {
		t.Error("expected 'y' to be deleted")
	}
	if _, ok := c.Get("x"); !ok {
		t.Error("expected 'x' to still exist after deleting 'y'")
	}
	if _, ok := c.Get("z"); !ok {
		t.Error("expected 'z' to still exist after deleting 'y'")
	}
	if got := c.Len(); got != 2 {
		t.Errorf("expected len 2 after deleting one of three entries, got %d", got)
	}
}

// TestRelatedCacheLRU_MaxRelatedCacheEntries verifies that MaxRelatedCacheEntries
// constant is defined and has a positive value (enforces the default cap exists).
func TestRelatedCacheLRU_MaxRelatedCacheEntries(t *testing.T) {
	t.Parallel()

	if session.MaxRelatedCacheEntries <= 0 {
		t.Errorf("MaxRelatedCacheEntries = %d, must be > 0", session.MaxRelatedCacheEntries)
	}
}

// TestRelatedCacheLRU_GetMiss verifies that Get on an absent key returns (nil, false).
func TestRelatedCacheLRU_GetMiss(t *testing.T) {
	t.Parallel()

	c := session.NewRelatedCacheLRU(10)

	got, ok := c.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for absent key, got true")
	}
	if got != nil {
		t.Errorf("expected nil result for absent key, got %v", got)
	}
}

// TestRelatedCacheLRU_SetUpdateExisting verifies that Set on an existing key
// updates the value without growing the cache.
func TestRelatedCacheLRU_SetUpdateExisting(t *testing.T) {
	t.Parallel()

	c := session.NewRelatedCacheLRU(10)

	c.Set("k", testRelatedResults(1))
	c.Set("k", testRelatedResults(99))

	got, ok := c.Get("k")
	if !ok {
		t.Fatal("expected 'k' to exist after second Set")
	}
	if len(got) == 0 || got[0].Result.Count != 99 {
		t.Errorf("expected updated count 99, got %v", got)
	}
	if c.Len() != 1 {
		t.Errorf("expected len 1 after updating existing key, got %d", c.Len())
	}
}

// TestRelatedCacheReplay_PreservesDefDisplayName pins the cache-replay
// contract used by handleNavigate / handleRelatedNavigate on detail
// re-entry: entries stored with distinct DefDisplayName values must reach
// the rightcolumn view as distinct messages so every per-row match resolves.
//
// This guards against regressing to a shape where the cache stores only
// resource.RelatedCheckResult — losing DefDisplayName left all four
// ct-events self-pivot rows stuck loading because the strict-match fallback
// refused to bind when TargetType matched multiple rows.
func TestRelatedCacheReplay_PreservesDefDisplayName(t *testing.T) {
	t.Parallel()

	c := session.NewRelatedCacheLRU(10)
	key := "ct-events:src-evt-0001"

	in := []session.RelatedCacheResult{
		{DefDisplayName: "CT events by AccessKeyId", Result: resource.RelatedCheckResult{TargetType: "ct-events", Count: 3, ResourceIDs: []string{"e1"}}},
		{DefDisplayName: "CT events by Username", Result: resource.RelatedCheckResult{TargetType: "ct-events", Count: 2, ResourceIDs: []string{"e2"}}},
		{DefDisplayName: "CT events by EventName", Result: resource.RelatedCheckResult{TargetType: "ct-events", Count: 1, ResourceIDs: []string{"e3"}}},
		{DefDisplayName: "CT events by SharedEventId", Result: resource.RelatedCheckResult{TargetType: "ct-events", Count: 4, ResourceIDs: []string{"e4"}}},
	}
	c.Set(key, in)

	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cached entry")
	}
	if len(got) != len(in) {
		t.Fatalf("len(cached) = %d, want %d", len(got), len(in))
	}

	for i, entry := range got {
		if entry.DefDisplayName != in[i].DefDisplayName {
			t.Errorf("got[%d].DefDisplayName = %q, want %q — losing this breaks self-pivot row binding",
				i, entry.DefDisplayName, in[i].DefDisplayName)
		}
		if entry.Result.Count != in[i].Result.Count {
			t.Errorf("got[%d].Result.Count = %d, want %d", i, entry.Result.Count, in[i].Result.Count)
		}
	}
}
