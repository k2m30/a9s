package unit

// In demo / --no-cache sessions the availability prefetch keeps only the first
// page of each paginated fetcher, so the seeded ResourceCache entry must carry
// the full PaginationMeta (NextToken etc.) for load-more to advance. Without a
// Pagination map, the seed takes IsTruncated from the Truncated map.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
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

	m, _ = rootApplyMsg(m, messages.AvailabilityPrefetched{
		Entries:        map[string]int{targetType: 2},
		Truncated:      map[string]bool{targetType: true},
		IssueCounts:    map[string]int{targetType: 0},
		IssueTruncated: map[string]bool{targetType: true},
		Resources:      map[string][]resource.Resource{targetType: {first, second}},
		Pagination:     map[string]*resource.PaginationMeta{targetType: wantMeta},
		// Stamp the live AvailabilityGen so the staleness guard accepts the
		// message (AcceptZeroGen=false).
		Gen: m.Core().Session().AvailabilityGen,
	})

	entry, ok := m.Core().ResourceCache(targetType)
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

// Without the Pagination field, the seed builds a PaginationMeta with
// IsTruncated taken from the Truncated map.
func TestPrefetchPaginationSeed_FallbackWhenPaginationOmitted(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	const targetType = "rds"
	r := resource.Resource{ID: "db-001", Name: "db-001"}

	m, _ = rootApplyMsg(m, messages.AvailabilityPrefetched{
		Entries:        map[string]int{targetType: 1},
		Truncated:      map[string]bool{targetType: true},
		IssueCounts:    map[string]int{targetType: 0},
		IssueTruncated: map[string]bool{targetType: true},
		Resources:      map[string][]resource.Resource{targetType: {r}},
		// Stamp the live AvailabilityGen so the staleness guard accepts the message
		// (AcceptZeroGen=false).
		Gen: m.Core().Session().AvailabilityGen,
	})

	entry, ok := m.Core().ResourceCache(targetType)
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
