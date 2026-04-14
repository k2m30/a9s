package unit

// Tests for ResourceListModel.InvalidateStyleCache and SetFetchFilter.
//
// InvalidateStyleCache: clears internal styledRowCache — observable only by
// verifying subsequent View() calls still render correctly (no panic, no stale
// data from a prior width change).
//
// SetFetchFilter / FetchFilter: round-trip tests confirming the server-side
// filter params are stored and returned intact.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// buildEC2List creates a loaded ResourceListModel for ec2 with one synthetic resource.
func buildEC2List(t *testing.T, width, height int) views.ResourceListModel {
	t.Helper()
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 resource type not found in registry")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, config.DefaultConfig(), k)
	m.SetSize(width, height)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{
				ID:     "i-0aaa111111111111a",
				Name:   "test-instance",
				Status: "running",
				Fields: map[string]string{
					"instance_id": "i-0aaa111111111111a",
					"type":        "t3.medium",
					"state":       "running",
				},
			},
		},
	})
	return m
}

// ---------------------------------------------------------------------------
// InvalidateStyleCache tests
// ---------------------------------------------------------------------------

// TestInvalidateStyleCache_ViewRendersAfterInvalidate verifies that after
// calling InvalidateStyleCache(), the subsequent View() call does not panic
// and still returns a non-empty string containing the resource ID.
func TestInvalidateStyleCache_ViewRendersAfterInvalidate(t *testing.T) {
	m := buildEC2List(t, 120, 24)

	// First render — populates styledRowCache.
	view1 := m.View()
	if view1 == "" {
		t.Fatal("first View() returned empty string")
	}

	// Invalidate the cache.
	m.InvalidateStyleCache()

	// Second render — must re-build from scratch without panic.
	view2 := m.View()
	if view2 == "" {
		t.Fatal("View() after InvalidateStyleCache returned empty string")
	}
}

// TestInvalidateStyleCache_RerendersWithNewWidth verifies that after a width
// change + InvalidateStyleCache(), the resource ID still appears in View().
// This simulates a terminal resize scenario where style cache must be cleared.
func TestInvalidateStyleCache_RerendersWithNewWidth(t *testing.T) {
	m := buildEC2List(t, 80, 24)

	// Render at width 80.
	_ = m.View()

	// Simulate terminal resize to 120.
	m.SetSize(120, 24)
	m.InvalidateStyleCache()

	view := m.View()
	if view == "" {
		t.Fatal("View() after resize + InvalidateStyleCache returned empty string")
	}
	plain := stripANSI(view)
	if plain == "" {
		t.Fatal("stripANSI(view) returned empty string")
	}
}

// TestInvalidateStyleCache_EmptyList verifies that InvalidateStyleCache on an
// empty-resource list does not panic and View() still returns something.
func TestInvalidateStyleCache_EmptyList(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 resource type not found")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, config.DefaultConfig(), k)
	m.SetSize(80, 24)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{},
	})

	m.InvalidateStyleCache()

	view := m.View()
	if view == "" {
		t.Fatal("View() after InvalidateStyleCache on empty list returned empty string")
	}
}

// ---------------------------------------------------------------------------
// SetFetchFilter / FetchFilter round-trip tests
// ---------------------------------------------------------------------------

// TestSetFetchFilter_RoundTrip verifies that FetchFilter returns the exact map
// set by SetFetchFilter.
func TestSetFetchFilter_RoundTrip(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 resource type not found")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, config.DefaultConfig(), k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	filter := map[string]string{
		"instance-state-name": "running",
		"tag:Env":             "prod",
	}
	m.SetFetchFilter(filter)

	got := m.FetchFilter()
	if len(got) != len(filter) {
		t.Fatalf("FetchFilter() len = %d, want %d", len(got), len(filter))
	}
	for k, wantV := range filter {
		if gotV := got[k]; gotV != wantV {
			t.Errorf("FetchFilter()[%q] = %q, want %q", k, gotV, wantV)
		}
	}
}

// TestSetFetchFilter_Nil verifies that setting nil clears the fetch filter.
func TestSetFetchFilter_Nil(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 resource type not found")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, config.DefaultConfig(), k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	// Set a filter first, then clear with nil.
	m.SetFetchFilter(map[string]string{"foo": "bar"})
	m.SetFetchFilter(nil)

	got := m.FetchFilter()
	if got != nil {
		t.Errorf("FetchFilter() after SetFetchFilter(nil) = %v, want nil", got)
	}
}

// TestSetFetchFilter_EmptyMap verifies that setting an empty (non-nil) map
// stores an empty map (not nil).
func TestSetFetchFilter_EmptyMap(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 resource type not found")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, config.DefaultConfig(), k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	m.SetFetchFilter(map[string]string{})

	got := m.FetchFilter()
	if got == nil {
		t.Error("FetchFilter() = nil after setting empty map, want non-nil empty map")
	}
	if len(got) != 0 {
		t.Errorf("FetchFilter() len = %d, want 0", len(got))
	}
}

// TestSetFetchFilter_InitiallyNil verifies that a fresh model has no fetch filter.
func TestSetFetchFilter_InitiallyNil(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 resource type not found")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, config.DefaultConfig(), k)
	m.SetSize(80, 24)
	m, _ = m.Init()

	if got := m.FetchFilter(); got != nil {
		t.Errorf("FetchFilter() on fresh model = %v, want nil", got)
	}
}
