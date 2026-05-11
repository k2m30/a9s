package unit

// qa_prefetch_pagination_seed_test.go — regression pin for the prefetch
// pagination-seed bug (P1 finding).
//
// Bug: In demo / --no-cache sessions, the synchronous availability prefetch
// retains only the first page of each paginated fetcher. Pre-fix, the
// AvailabilityPrefetchedMsg handler synthesized a PaginationMeta that kept
// only IsTruncated, discarding the original NextToken. That turned the
// seeded entry into an apparent full-cache hit on the next OpenList /
// load-more path, but no further pages could ever be fetched — pagination
// was dead.
//
// Fix: AvailabilityPrefetchedMsg now carries the full per-type
// PaginationMeta map, and handleAvailabilityPrefetched seeds the
// ResourceCache with that meta when present. Synthetic IsTruncated-only
// remains as a fallback for callers (tests / messages) that don't supply
// the new Pagination field.
//
// This test pins both:
//  1. Full pagination (NextToken etc.) survives the prefetch → cache seed.
//  2. The legacy fallback still works when Pagination is omitted.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// TestPrefetchPaginationSeed_PreservesNextToken — when the prefetched message
// carries a full PaginationMeta with NextToken, the seeded ResourceCache entry
// MUST keep that NextToken so a later load-more path can advance.
func TestPrefetchPaginationSeed_PreservesNextToken(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	const targetType = "ec2"
	first := resource.Resource{ID: "i-001", Name: "i-001"}
	second := resource.Resource{ID: "i-002", Name: "i-002"}

	wantMeta := &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "page-2-cursor",
		PageSize:    2,
		TotalHint:   -1,
	}

	m, _ = rootApplyMsg(m, messages.AvailabilityPrefetchedMsg{
		Entries:        map[string]int{targetType: 2},
		Truncated:      map[string]bool{targetType: true},
		IssueCounts:    map[string]int{targetType: 0},
		IssueTruncated: map[string]bool{targetType: true},
		Resources:      map[string][]resource.Resource{targetType: {first, second}},
		Pagination:     map[string]*resource.PaginationMeta{targetType: wantMeta},
		Gen:            0,
	})

	entry, ok := m.Session.ResourceCache[targetType]
	if !ok {
		t.Fatalf("expected ResourceCache entry for %q after prefetch seed; got nil", targetType)
	}
	if entry.Pagination == nil {
		t.Fatalf("seeded entry has nil Pagination; expected to be carried over from msg.Pagination")
	}
	if entry.Pagination.NextToken != wantMeta.NextToken {
		t.Errorf("NextToken lost in seed: got %q, want %q — pagination cannot advance past page 1",
			entry.Pagination.NextToken, wantMeta.NextToken)
	}
	if !entry.Pagination.IsTruncated {
		t.Errorf("IsTruncated lost in seed; want true")
	}
	if entry.Pagination.PageSize != wantMeta.PageSize {
		t.Errorf("PageSize lost: got %d, want %d", entry.Pagination.PageSize, wantMeta.PageSize)
	}
	if entry.Pagination.TotalHint != wantMeta.TotalHint {
		t.Errorf("TotalHint lost: got %d, want %d", entry.Pagination.TotalHint, wantMeta.TotalHint)
	}
}

// TestPrefetchPaginationSeed_FallbackWhenPaginationOmitted — when an old-style
// message arrives without the Pagination field, the seed still produces a
// minimum-viable PaginationMeta with IsTruncated set from the legacy
// Truncated map. This preserves backward compatibility for any test/message
// that doesn't construct Pagination.
func TestPrefetchPaginationSeed_FallbackWhenPaginationOmitted(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	const targetType = "rds"
	r := resource.Resource{ID: "db-001", Name: "db-001"}

	m, _ = rootApplyMsg(m, messages.AvailabilityPrefetchedMsg{
		Entries:        map[string]int{targetType: 1},
		Truncated:      map[string]bool{targetType: true},
		IssueCounts:    map[string]int{targetType: 0},
		IssueTruncated: map[string]bool{targetType: true},
		Resources:      map[string][]resource.Resource{targetType: {r}},
		// Pagination intentionally nil — pre-fix shape.
		Gen: 0,
	})

	entry, ok := m.Session.ResourceCache[targetType]
	if !ok {
		t.Fatalf("expected ResourceCache entry for %q after fallback prefetch seed; got nil", targetType)
	}
	if entry.Pagination == nil {
		t.Fatalf("fallback seed: expected synthetic PaginationMeta, got nil")
	}
	if !entry.Pagination.IsTruncated {
		t.Errorf("fallback seed dropped IsTruncated; expected true from msg.Truncated")
	}
	if entry.Pagination.NextToken != "" {
		t.Errorf("fallback seed leaked NextToken %q; expected empty", entry.Pagination.NextToken)
	}
}
