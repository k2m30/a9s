// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"fmt"
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// testResults returns a non-nil slice of session.RelatedCacheResult for use
// as a cache value. The result carries exactly count synthetic IDs so
// Result.Count() == count, matching this helper's old Count-literal behavior.
func testResults(count int) []session.RelatedCacheResult {
	ids := make([]string, count)
	for i := range ids {
		ids[i] = fmt.Sprintf("ec2-%d", i)
	}
	return []session.RelatedCacheResult{{
		DefDisplayName: "",
		Result:         resource.KnownRelated("ec2", ids, false),
	}}
}

// TestRelatedCacheLRU_CapEnforced verifies that inserting cap+1 entries evicts
// the oldest entry so the cache never exceeds its capacity.
func TestRelatedCacheLRU_CapEnforced(t *testing.T) {
	const cap = 500
	c := session.NewRelatedCacheLRU(cap)

	for i := 0; i <= cap; i++ {
		c.Set(fmt.Sprintf("key-%d", i), testResults(i))
	}

	got := c.Len()
	if got != cap {
		t.Errorf("expected len %d after inserting %d entries into cap=%d cache, got %d", cap, cap+1, cap, got)
	}
}

// TestRelatedCacheLRU_LRUEviction verifies that the least-recently-used entry
// is evicted when the cache is full, and that a recent Get() promotes an entry.
func TestRelatedCacheLRU_LRUEviction(t *testing.T) {
	c := session.NewRelatedCacheLRU(3)

	c.Set("a", testResults(1))
	c.Set("b", testResults(2))
	c.Set("c", testResults(3))

	// Access "a" to make it the most recently used.
	if _, ok := c.Get("a"); !ok {
		t.Fatal("expected 'a' to exist before eviction test")
	}

	// Insert "d" — should evict "b" (least recently used).
	c.Set("d", testResults(4))

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
	c := session.NewRelatedCacheLRU(10)

	for i := range 5 {
		c.Set(fmt.Sprintf("k%d", i), testResults(i))
	}

	c.Clear()

	if got := c.Len(); got != 0 {
		t.Errorf("expected len 0 after Clear(), got %d", got)
	}
}

// TestRelatedCacheLRU_Delete verifies that Delete() removes a specific key
// while leaving other entries intact.
func TestRelatedCacheLRU_Delete(t *testing.T) {
	c := session.NewRelatedCacheLRU(10)

	c.Set("x", testResults(1))
	c.Set("y", testResults(2))
	c.Set("z", testResults(3))

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
