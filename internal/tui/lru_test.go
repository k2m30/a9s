package tui

import (
	"fmt"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// testResults returns a non-nil slice of relatedCacheResult for use as a cache value.
func testResults(count int) []relatedCacheResult {
	return []relatedCacheResult{{
		DefDisplayName: "",
		Result:         resource.RelatedCheckResult{TargetType: "ec2", Count: count},
	}}
}

// TestRelatedCacheLRU_CapEnforced verifies that inserting cap+1 entries evicts
// the oldest entry so the cache never exceeds its capacity.
func TestRelatedCacheLRU_CapEnforced(t *testing.T) {
	const cap = 500
	c := newRelatedCacheLRU(cap)

	for i := 0; i <= cap; i++ {
		c.set(fmt.Sprintf("key-%d", i), testResults(i))
	}

	got := c.len()
	if got != cap {
		t.Errorf("expected len %d after inserting %d entries into cap=%d cache, got %d", cap, cap+1, cap, got)
	}
}

// TestRelatedCacheLRU_LRUEviction verifies that the least-recently-used entry
// is evicted when the cache is full, and that a recent get() promotes an entry.
func TestRelatedCacheLRU_LRUEviction(t *testing.T) {
	c := newRelatedCacheLRU(3)

	c.set("a", testResults(1))
	c.set("b", testResults(2))
	c.set("c", testResults(3))

	// Access "a" to make it the most recently used.
	if _, ok := c.get("a"); !ok {
		t.Fatal("expected 'a' to exist before eviction test")
	}

	// Insert "d" — should evict "b" (least recently used).
	c.set("d", testResults(4))

	if _, ok := c.get("a"); !ok {
		t.Error("expected 'a' to still exist after eviction")
	}
	if _, ok := c.get("b"); ok {
		t.Error("expected 'b' to have been evicted (LRU)")
	}
	if _, ok := c.get("c"); !ok {
		t.Error("expected 'c' to still exist after eviction")
	}
	if _, ok := c.get("d"); !ok {
		t.Error("expected 'd' to exist after insertion")
	}
}

// TestRelatedCacheLRU_Clear verifies that clear() removes all entries and
// resets the length to zero.
func TestRelatedCacheLRU_Clear(t *testing.T) {
	c := newRelatedCacheLRU(10)

	for i := range 5 {
		c.set(fmt.Sprintf("k%d", i), testResults(i))
	}

	c.clear()

	if got := c.len(); got != 0 {
		t.Errorf("expected len 0 after clear(), got %d", got)
	}
}

// TestRelatedCacheReplay_PreservesDefDisplayName pins the cache-replay
// contract used by handleNavigate / handleRelatedNavigate on detail
// re-entry: entries stored with distinct DefDisplayName values must reach
// the rightcolumn view as distinct RelatedCheckResultMsg messages so the
// per-row match by DefDisplayName resolves every row.
//
// This guards against regressing back to a shape where the cache stores
// only `resource.RelatedCheckResult` — the case that left all four
// ct-events self-pivot rows ("by AccessKeyId" / "by Username" /
// "by EventName" / "by SharedEventId") stuck loading because the
// rightcolumn's strict-match fallback refused to bind when matches > 1
// for the shared TargetType="ct-events".
func TestRelatedCacheReplay_PreservesDefDisplayName(t *testing.T) {
	c := newRelatedCacheLRU(10)
	key := relatedCacheKey("ct-events", "src-evt-0001")

	in := []relatedCacheResult{
		{DefDisplayName: "CT events by AccessKeyId", Result: resource.RelatedCheckResult{TargetType: "ct-events", Count: 3, ResourceIDs: []string{"e1"}}},
		{DefDisplayName: "CT events by Username", Result: resource.RelatedCheckResult{TargetType: "ct-events", Count: 2, ResourceIDs: []string{"e2"}}},
		{DefDisplayName: "CT events by EventName", Result: resource.RelatedCheckResult{TargetType: "ct-events", Count: 1, ResourceIDs: []string{"e3"}}},
		{DefDisplayName: "CT events by SharedEventId", Result: resource.RelatedCheckResult{TargetType: "ct-events", Count: 4, ResourceIDs: []string{"e4"}}},
	}
	c.set(key, in)

	got, ok := c.get(key)
	if !ok {
		t.Fatal("expected cached entry")
	}
	if len(got) != len(in) {
		t.Fatalf("len(cached) = %d, want %d", len(got), len(in))
	}

	// relatedCacheReplay must project cached entries back into messages
	// carrying the original DefDisplayName so every row resolves.
	msgs := relatedCacheReplay("ct-events", got)
	if len(msgs) != len(in) {
		t.Fatalf("len(replay) = %d, want %d", len(msgs), len(in))
	}
	for i, m := range msgs {
		if m.ResourceType != "ct-events" {
			t.Errorf("replay[%d].ResourceType = %q, want ct-events", i, m.ResourceType)
		}
		if m.DefDisplayName != in[i].DefDisplayName {
			t.Errorf("replay[%d].DefDisplayName = %q, want %q — losing this breaks self-pivot row binding",
				i, m.DefDisplayName, in[i].DefDisplayName)
		}
		if m.Result.Count != in[i].Result.Count {
			t.Errorf("replay[%d].Result.Count = %d, want %d", i, m.Result.Count, in[i].Result.Count)
		}
	}
}

// TestRelatedCacheLRU_Delete verifies that delete() removes a specific key
// while leaving other entries intact.
func TestRelatedCacheLRU_Delete(t *testing.T) {
	c := newRelatedCacheLRU(10)

	c.set("x", testResults(1))
	c.set("y", testResults(2))
	c.set("z", testResults(3))

	c.delete("y")

	if _, ok := c.get("y"); ok {
		t.Error("expected 'y' to be deleted")
	}
	if _, ok := c.get("x"); !ok {
		t.Error("expected 'x' to still exist after deleting 'y'")
	}
	if _, ok := c.get("z"); !ok {
		t.Error("expected 'z' to still exist after deleting 'y'")
	}
	if got := c.len(); got != 2 {
		t.Errorf("expected len 2 after deleting one of three entries, got %d", got)
	}
}
