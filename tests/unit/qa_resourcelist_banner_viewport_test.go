package unit

// qa_resourcelist_banner_viewport_test.go — P2.1 regression test.
//
// Bug: resourcelist.go View() writes the banner row AFTER visibleRows has
// already been committed to VisibleWindow(), so when a banner is shown the
// total rendered line count is height+1 instead of <= height.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// bannerViewportTypeDef returns a minimal ResourceTypeDef suitable for viewport
// overflow tests.  Using "name" as identity column so the marker cascade works.
func bannerViewportTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		ShortName: "ec2",
		Name:      "EC2 Instances",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 12},
		},
	}
}

// bannerViewportResources returns 10 resources — more than any tight viewport
// can show — so the scroll window never obscures the banner condition.
func bannerViewportResources() []resource.Resource {
	out := make([]resource.Resource, 10)
	for i := range 10 {
		id := fmt.Sprintf("i-%02d", i+1)
		name := fmt.Sprintf("instance-%d", i+1)
		out[i] = resource.Resource{
			ID:     id,
			Name:   name,
			Status: "running",
			Fields: map[string]string{"name": name, "state": "running"},
		}
	}
	return out
}

// buildBannerViewportModel constructs a ResourceListModel with the given height
// and 10 resources loaded.
func buildBannerViewportModel(t *testing.T, height int) views.ResourceListModel {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	td := bannerViewportTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, height)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    bannerViewportResources(),
	})
	return m
}

// TestResourceList_BannerReservesViewportRow verifies that when a banner is
// shown the total line count of View() is <= the configured height.
//
// Current bug: the banner is appended BEFORE the header but AFTER visibleRows
// is committed, so the output is height+1 lines when a banner appears.
//
// Setup: height=6 → visibleRows=5; inject 5 findings whose resource IDs are
// NOT in the visible window (IDs "i-off-01"…"i-off-05") so that hidden > 0
// and the banner fires on the existing "hidden > 0" branch.
func TestResourceList_BannerReservesViewportRow(t *testing.T) {
	const height = 6
	m := buildBannerViewportModel(t, height)

	// Inject findings for 5 IDs that are NOT in the resource list at all.
	// This guarantees hidden > 0 regardless of the scroll window, which is
	// the condition that makes findingsBanner() return a non-empty string.
	offViewportFindings := map[string]resource.EnrichmentFinding{
		"i-off-01": {Severity: "!", Summary: "off-viewport finding 1"},
		"i-off-02": {Severity: "!", Summary: "off-viewport finding 2"},
		"i-off-03": {Severity: "!", Summary: "off-viewport finding 3"},
		"i-off-04": {Severity: "!", Summary: "off-viewport finding 4"},
		"i-off-05": {Severity: "!", Summary: "off-viewport finding 5"},
	}
	m.SetEnrichmentState(5, false, offViewportFindings)

	out := m.View()
	// Count rendered lines.  A string with N newlines has N+1 lines.
	lines := strings.Count(out, "\n") + 1
	if lines > height {
		t.Errorf("View() rendered %d lines with height=%d; banner must reserve a viewport row so total stays <= height (P2.1 bug)", lines, height)
	}
}
