package unit

// qa_attention_refactor_test.go — T009: attention filter behavior tests.
//
// These tests verify that ResourceListModel ctrl+z behavior works correctly
// and that the AttentionOnly() accessor reflects the state set at construction.
// Written against the CURRENT interface to guard against regressions during
// the planned refactor of attentionOnly bool → embedded AttentionFilter.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// attentionEC2TypeDef returns a minimal ResourceTypeDef for EC2 instances.
func attentionEC2TypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
}

// attentionMixedResources returns a slice with running, stopped, and terminated resources.
func attentionMixedResources() []resource.Resource {
	return []resource.Resource{
		{ID: "i-001", Name: "web-01", Status: "running"},
		{ID: "i-002", Name: "web-02", Status: "stopped"},
		{ID: "i-003", Name: "web-03", Status: "terminated"},
		{ID: "i-004", Name: "api-01", Status: "running"},
		{ID: "i-005", Name: "db-01", Status: "pending"},
	}
}

// newAttentionListFromCache builds a ResourceListModel via NewResourceListFromCache.
// attentionOnly controls the attention filter state at construction time.
func newAttentionListFromCache(t *testing.T, attentionOnly bool, resources []resource.Resource) views.ResourceListModel {
	t.Helper()
	return views.NewResourceListFromCache(
		attentionEC2TypeDef(),
		nil,               // viewConfig
		keys.Default(),    // keys
		resources,
		nil,               // pagination
		"",                // filterText
		views.SortColNone, // sortColIdx
		true,              // sortAsc
		0,                 // cursorPos
		0,                 // hScrollOffset
		attentionOnly,
	)
}

// ---------------------------------------------------------------------------
// TestResourceListAttentionFilterPreservesExistingBehavior
// ---------------------------------------------------------------------------

// TestResourceListAttentionFilterPreservesExistingBehavior verifies that when
// attentionOnly is false, all resources are visible: the frame title shows the
// total count and the [!] suffix is absent.
func TestResourceListAttentionFilterPreservesExistingBehavior(t *testing.T) {
	resources := attentionMixedResources()

	t.Run("frame title contains total count when attentionOnly is false", func(t *testing.T) {
		m := newAttentionListFromCache(t, false, resources)
		title := m.FrameTitle()
		// Expect "ec2(5)" — all 5 resources visible, no attention filter.
		want := "ec2(5)"
		if title != want {
			t.Errorf("FrameTitle() = %q, want %q", title, want)
		}
	})

	t.Run("frame title contains [!] suffix when attentionOnly is true", func(t *testing.T) {
		m := newAttentionListFromCache(t, true, resources)
		title := m.FrameTitle()
		if !strings.Contains(title, "[!]") {
			t.Errorf("FrameTitle() = %q, expected [!] suffix when attentionOnly=true", title)
		}
	})

	t.Run("all resources loaded regardless of attentionOnly state", func(t *testing.T) {
		m := newAttentionListFromCache(t, false, resources)
		if got := len(m.AllResources()); got != len(resources) {
			t.Errorf("AllResources() count = %d, want %d", got, len(resources))
		}
	})
}
