package tui

import (
	"fmt"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// testResults returns a non-nil slice of RelatedCheckResult for use as a cache value.
func testResults(count int) []resource.RelatedCheckResult {
	return []resource.RelatedCheckResult{{TargetType: "ec2", Count: count}}
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

	for i := 0; i < 5; i++ {
		c.set(fmt.Sprintf("k%d", i), testResults(i))
	}

	c.clear()

	if got := c.len(); got != 0 {
		t.Errorf("expected len 0 after clear(), got %d", got)
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
