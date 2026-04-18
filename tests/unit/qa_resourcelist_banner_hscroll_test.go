package unit

// qa_resourcelist_banner_hscroll_test.go — P2.2 regression test.
//
// Bug: when hScrollOffset is high enough that markerColIdx == -1 (identity
// column scrolled off-screen), renderDataRow skips the row marker. But
// findingsBanner() counts that resource as "visible-with-finding" because
// m.findingsByID[id] returns ok==true. So hidden == 0 and the banner is
// suppressed even though no visual signal reaches the user.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// hscrollMarkerTypeDef returns a ResourceTypeDef whose identity column is at
// index 0 (set via IdentityKey so resolveIdentityColumn's step 1 always finds
// it there, regardless of path inspection). ShortName "hscroll-test" has no
// default config so resolveColumns() always uses typeDef.Columns.
//
// Column widths are sized so that at width=60 the Name column + at least two
// more columns fit without scrolling, but pressing 'l' drops the rightmost
// column (canScroll=true), incrementing hScrollOffset to 1.
//
// With hScrollOffset=1, fullMarkerColIdx(0) < hScrollOffset(1) → markerColIdx=-1.
func hscrollMarkerTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		ShortName:   "hscroll-test",
		Name:        "HScroll Test",
		IdentityKey: "resource_name",
		Columns: []resource.Column{
			{Key: "resource_name", Title: "Resource Name", Width: 24},
			{Key: "state", Title: "State", Width: 14},
			{Key: "region", Title: "Region", Width: 14},
			{Key: "account", Title: "Account", Width: 20},
		},
	}
}

func hscrollMarkerResources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "i-hscroll-01", Name: "web-server-01", Status: "running",
			Fields: map[string]string{
				"resource_name": "web-server-01",
				"state":         "running",
				"region":        "us-east-1",
				"account":       "123456789012",
			},
		},
		{
			ID: "i-hscroll-02", Name: "api-gateway-02", Status: "running",
			Fields: map[string]string{
				"resource_name": "api-gateway-02",
				"state":         "running",
				"region":        "us-east-1",
				"account":       "123456789012",
			},
		},
		{
			ID: "i-hscroll-03", Name: "worker-node-03", Status: "running",
			Fields: map[string]string{
				"resource_name": "worker-node-03",
				"state":         "running",
				"region":        "us-east-1",
				"account":       "123456789012",
			},
		},
	}
}

// buildHscrollModel returns a ResourceListModel with the identity column
// scrolled off-screen.
//
// Column widths: resource_name(24+2=26) + state(14+2=16) + region(14+2=16) = 58,
// then account(20+2=22) → total=80 > 60. At width=60:
//   - 1(leading) + 26 + 16 = 43, then region: 43+16=59 <= 60, then
//     account: 59+22=81 > 60, remaining=60-59-2=-1 < 10 → dropped.
// So len(fitted)=3 < len(visible)=4 → canScroll=true → 'l' increments hScrollOffset.
// After scroll: fullMarkerColIdx(0) < hScrollOffset(1) → markerColIdx=-1.
func buildHscrollModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	td := hscrollMarkerTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(60, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "hscroll-test",
		Resources:    hscrollMarkerResources(),
	})
	// Press 'l' until hScrollOffset becomes 1.
	for range 5 {
		prev := m.HScrollOffset()
		m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
		if m.HScrollOffset() > prev {
			break
		}
	}
	return m
}

// TestResourceList_HscrollHiddenMarkerStillTriggersBanner verifies that when
// the identity column is scrolled off-screen (markerColIdx == -1), a finding
// on a visible row STILL produces a banner.
//
// Current bug: findingsBanner() counts that row as "visible-with-finding"
// regardless of whether the marker is actually rendered, so hidden stays 0
// and the banner is suppressed — leaving the user with no signal at all.
func TestResourceList_HscrollHiddenMarkerStillTriggersBanner(t *testing.T) {
	m := buildHscrollModel(t)

	if m.HScrollOffset() == 0 {
		t.Fatal("precondition: hScrollOffset must be > 0 for P2.2 test (column setup did not allow scrolling)")
	}

	// Place a finding on resource i-hscroll-01, which IS in the visible window
	// but whose identity column (resource_name at fullColIdx=0) is off-screen
	// because hScrollOffset=1.  The row marker (renderDataRow, markerColIdx=-1)
	// will NOT be rendered.
	findings := map[string]resource.EnrichmentFinding{
		"i-hscroll-01": {Severity: "!", Summary: "system status impaired"},
	}
	m.SetEnrichmentState(1, false, findings)

	out := m.View()
	plain := stripANSI(out)

	// Verify the identity column's cell is NOT in the rendered output
	// (it would show "web-server-01" which is the resource_name value).
	// If it IS visible, the column didn't scroll off and the precondition fails.
	if strings.Contains(plain, "web-server-01") {
		t.Skip("identity column value 'web-server-01' is still visible after hscroll — precondition not met")
	}

	// With the identity column off-screen, the row marker "! " must NOT appear
	// (markerColIdx==-1 suppresses it).
	if strings.Contains(plain, "! ") {
		// Marker visible — either (a) scrolling put the marker on a different column
		// (wrong-column marker bug) or (b) our identity column didn't scroll off.
		// In either case the P2.2 precondition (marker=hidden) is not met.
		t.Skipf("marker '! ' visible at hScrollOffset=%d — precondition not met", m.HScrollOffset())
	}

	// With no marker AND no banner, the finding is completely invisible.
	// A correct fix would show a banner.
	if !strings.Contains(plain, "finding") && !strings.Contains(plain, "background-check") {
		t.Errorf("View() shows neither row marker nor banner for a finding whose identity "+
			"column is scrolled off-screen (P2.2 bug: findingsBanner counts off-screen "+
			"rows as 'visible-with-finding', suppressing the banner)\n"+
			"hScrollOffset=%d\nRendered:\n%s", m.HScrollOffset(), plain)
	}
}
