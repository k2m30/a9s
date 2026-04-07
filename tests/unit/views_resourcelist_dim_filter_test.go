package unit

// Tests for §7 of docs/design/ct-event-list-v2.md: ctrl+z attention filter.
//
// Contract: pressing ctrl+z on a ResourceListModel toggles attentionOnly mode.
// When on, rows whose Status resolves to dim/neutral via styles.IsDimRowColor
// are hidden from the rendered view. Toggle off restores all rows.
//
// These tests WILL FAIL until the P4 coder ships:
//   - keys.Map.ToggleAttentionOnly binding (ctrl+z)
//   - ResourceListModel.attentionOnly bool field
//   - Update handler for ctrl+z (flip attentionOnly, call applySortAndFilter, reset cursor)
//   - applyFilter second pass that drops dim rows when attentionOnly==true
//   - [!] indicator in View()/FrameTitle()/BottomHints() when attentionOnly is active
//
// Key construction:
//   tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}
// This matches key.NewBinding(key.WithKeys("ctrl+z")) via bubbles/v2 key.Matches.

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ctrlZ constructs the ctrl+z key press message understood by bubbles/v2 key.Matches.
// Equivalent to key.NewBinding(key.WithKeys("ctrl+z")).
func ctrlZ() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}
}

// ctrlZModel builds a ResourceListModel for the given resource type, loads the
// provided resources, and returns the model ready for Update calls.
// viewConfig is passed so the model can call applySortAndFilter correctly.
func ctrlZModel(t *testing.T, shortName string, resources []resource.Resource) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("resource type %q not found in registry", shortName)
	}

	cfg := config.DefaultConfig()
	k := keys.Default()
	m := views.NewResourceList(*td, cfg, k)
	m.SetSize(200, 30)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: shortName,
		Resources:    resources,
	})
	return m
}

// ctEvents4 returns the canonical 4-resource ct-events seed used across §7 tests:
// 2 ct-info (dim), 1 ct-attention (yellow), 1 ct-danger (red).
func ctEvents4() []resource.Resource {
	return []resource.Resource{
		{ID: "evt-0001", Name: "read-1", Status: "ct-info"},
		{ID: "evt-0002", Name: "write-1", Status: "ct-attention"},
		{ID: "evt-0003", Name: "delete-1", Status: "ct-danger"},
		{ID: "evt-0004", Name: "read-2", Status: "ct-info"},
	}
}

// ===========================================================================
// TestCtrlZ_CTEvents_HidesDimRows (§7.1, §7.2)
//
// After one ctrl+z press, ct-info rows disappear from View() while the
// underlying AllResources() slice is unchanged.
// ===========================================================================

func TestCtrlZ_CTEvents_HidesDimRows(t *testing.T) {
	m := ctrlZModel(t, "ct-events", ctEvents4())

	// Sanity: all 4 resources loaded.
	if got := len(m.AllResources()); got != 4 {
		t.Fatalf("AllResources before toggle: got %d, want 4", got)
	}

	// Toggle on.
	m, _ = m.Update(ctrlZ())

	// Underlying data must not change.
	if got := len(m.AllResources()); got != 4 {
		t.Errorf("AllResources after toggle: got %d, want 4 (underlying data must be preserved)", got)
	}

	view := stripANSI(m.View())

	// Attention-worthy rows must be visible.
	if !strings.Contains(view, "write-1") {
		t.Errorf("ct-attention row 'write-1' missing from view after ctrl+z toggle ON\n  view:\n%s", view)
	}
	if !strings.Contains(view, "delete-1") {
		t.Errorf("ct-danger row 'delete-1' missing from view after ctrl+z toggle ON\n  view:\n%s", view)
	}

	// Dim rows must be hidden.
	if strings.Contains(view, "read-1") {
		t.Errorf("ct-info row 'read-1' should be hidden after ctrl+z toggle ON\n  view:\n%s", view)
	}
	if strings.Contains(view, "read-2") {
		t.Errorf("ct-info row 'read-2' should be hidden after ctrl+z toggle ON\n  view:\n%s", view)
	}
}

// ===========================================================================
// TestCtrlZ_Toggles_On_Off_Restores (§7.2)
//
// Two ctrl+z presses: on then off. All 4 rows visible after second press.
// ===========================================================================

func TestCtrlZ_Toggles_On_Off_Restores(t *testing.T) {
	m := ctrlZModel(t, "ct-events", ctEvents4())

	// First press: toggle on.
	m, _ = m.Update(ctrlZ())
	// Second press: toggle off.
	m, _ = m.Update(ctrlZ())

	view := stripANSI(m.View())

	for _, name := range []string{"read-1", "write-1", "delete-1", "read-2"} {
		if !strings.Contains(view, name) {
			t.Errorf("row %q missing from view after ctrl+z toggle OFF\n  view:\n%s", name, view)
		}
	}
}

// ===========================================================================
// TestCtrlZ_ResetsCursorToTop (§7.2)
//
// Move cursor to index 2, then toggle on. Cursor must reset to 0 (top of
// the remaining filtered rows).
// ===========================================================================

func TestCtrlZ_ResetsCursorToTop(t *testing.T) {
	m := ctrlZModel(t, "ct-events", ctEvents4())

	// Move cursor down twice (index 0 → 2).
	m, _ = m.Update(rlKeyPress("j"))
	m, _ = m.Update(rlKeyPress("j"))

	if got := m.CursorPosition(); got != 2 {
		// Keyboard nav may differ; just ensure cursor is non-zero before toggle.
		if got == 0 {
			t.Skip("cursor did not advance past 0 — skipping cursor-reset assertion")
		}
	}

	// Toggle on.
	m, _ = m.Update(ctrlZ())

	if got := m.CursorPosition(); got != 0 {
		t.Errorf("CursorPosition after ctrl+z toggle ON: got %d, want 0 (cursor must reset to top)", got)
	}
}

// ===========================================================================
// TestCtrlZ_EC2_HidesDimRowsNotRunningOrStopped (§7.4)
//
// ec2 case: running (green) and stopped (red) stay visible.
// terminated (dim) is hidden. stopped is NOT dim per §7.4.
// ===========================================================================

func TestCtrlZ_EC2_HidesDimRowsNotRunningOrStopped(t *testing.T) {
	ec2Resources := []resource.Resource{
		{ID: "i-0001", Name: "web-prod", Status: "running"},
		{ID: "i-0002", Name: "batch-job", Status: "stopped"},
		{ID: "i-0003", Name: "old-build", Status: "terminated"},
		{ID: "i-0004", Name: "api-prod", Status: "running"},
	}
	m := ctrlZModel(t, "ec2", ec2Resources)

	// Toggle on.
	m, _ = m.Update(ctrlZ())

	view := stripANSI(m.View())

	// running rows must be visible (green — not dim).
	if !strings.Contains(view, "web-prod") {
		t.Errorf("running row 'web-prod' must be visible after ctrl+z (green is not dim)\n  view:\n%s", view)
	}
	if !strings.Contains(view, "api-prod") {
		t.Errorf("running row 'api-prod' must be visible after ctrl+z (green is not dim)\n  view:\n%s", view)
	}

	// stopped must be visible (red — not dim per §7.4).
	if !strings.Contains(view, "batch-job") {
		t.Errorf("stopped row 'batch-job' must be visible after ctrl+z (red is not dim per §7.4)\n  view:\n%s", view)
	}

	// terminated must be hidden (dim).
	if strings.Contains(view, "old-build") {
		t.Errorf("terminated row 'old-build' must be hidden after ctrl+z (terminated is dim)\n  view:\n%s", view)
	}
}

// ===========================================================================
// TestCtrlZ_PerViewState_DoesNotBleed (§7.3)
//
// Toggle attentionOnly on the ct-events instance. The ec2 instance must
// still show all its rows (attentionOnly state does not bleed across views).
// ===========================================================================

func TestCtrlZ_PerViewState_DoesNotBleed(t *testing.T) {
	// Build two independent model instances.
	ctEventsResources := ctEvents4()
	ec2Resources := []resource.Resource{
		{ID: "i-0001", Name: "web-prod", Status: "running"},
		{ID: "i-0002", Name: "batch-job", Status: "stopped"},
		{ID: "i-0003", Name: "old-build", Status: "terminated"},
		{ID: "i-0004", Name: "api-prod", Status: "running"},
	}

	mCT := ctrlZModel(t, "ct-events", ctEventsResources)
	mEC2 := ctrlZModel(t, "ec2", ec2Resources)

	// Toggle ct-events on.
	mCT, _ = mCT.Update(ctrlZ())

	// ct-events: dim rows hidden (confirming toggle applied).
	ctView := stripANSI(mCT.View())
	if strings.Contains(ctView, "read-1") {
		t.Errorf("ct-events: ct-info row 'read-1' should be hidden after toggle ON\n  view:\n%s", ctView)
	}

	// ec2: all rows must still be visible (no bleed).
	ec2View := stripANSI(mEC2.View())
	for _, name := range []string{"web-prod", "batch-job", "old-build", "api-prod"} {
		if !strings.Contains(ec2View, name) {
			t.Errorf("ec2 view missing %q after ct-events toggle ON (attentionOnly bled across views)\n  ec2 view:\n%s", name, ec2View)
		}
	}
}

// ===========================================================================
// TestCtrlZ_PersistsAcrossCacheRoundTrip (§7.3 persistent state)
//
// attentionOnly must survive a cache eviction+restore cycle via
// NewResourceListFromCache. The coder adds attentionOnly bool as the 11th
// trailing parameter and adds an AttentionOnly() accessor.
// This test WILL COMPILE-ERROR until those two additions land.
// ===========================================================================

func TestCtrlZ_PersistsAcrossCacheRoundTrip(t *testing.T) {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := resource.FindResourceType("ct-events")
	if td == nil {
		t.Fatal("ct-events resource type not found")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, config.DefaultConfig(), k)
	m.SetSize(200, 20)
	m, _ = m.Init()

	// Load 4 seed resources: 2 ct-info (dim), 1 ct-attention, 1 ct-danger.
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ct-events",
		Resources:    ctEvents4(),
	})

	// Toggle attentionOnly ON.
	m, _ = m.Update(ctrlZ())
	if !m.AttentionOnly() {
		t.Fatal("attentionOnly should be true after ctrl+z")
	}

	// Capture non-attention row count before cache round-trip.
	visibleBefore := len(m.AllResources()) // underlying 4 rows preserved

	// Simulate cache eviction: rebuild via NewResourceListFromCache with the
	// attentionOnly flag forwarded as the 11th arg (added by coder).
	sortField, sortAsc := m.SortState()
	m2 := views.NewResourceListFromCache(
		*td, config.DefaultConfig(), k,
		m.AllResources(),
		m.PaginationState(),
		m.FilterText(),
		sortField,
		sortAsc,
		m.CursorPosition(),
		m.HScrollOffset(),
		m.AttentionOnly(), // 11th arg — NEW; coder adds this
	)
	m2.SetSize(200, 20)

	// attentionOnly must be restored.
	if !m2.AttentionOnly() {
		t.Fatal("attentionOnly should persist across cache round-trip via NewResourceListFromCache")
	}

	// Underlying AllResources() count must be unchanged.
	if got := len(m2.AllResources()); got != visibleBefore {
		t.Errorf("AllResources after cache round-trip: got %d, want %d (underlying data must be preserved)", got, visibleBefore)
	}

	// Dim rows must still be filtered from View().
	view2 := stripANSI(m2.View())
	if strings.Contains(view2, "read-1") {
		t.Errorf("ct-info row 'read-1' should be hidden after cache round-trip with attentionOnly=true\n  view:\n%s", view2)
	}
	if strings.Contains(view2, "read-2") {
		t.Errorf("ct-info row 'read-2' should be hidden after cache round-trip with attentionOnly=true\n  view:\n%s", view2)
	}

	// Attention/danger rows must be visible.
	if !strings.Contains(view2, "write-1") {
		t.Errorf("ct-attention row 'write-1' must be visible after cache round-trip\n  view:\n%s", view2)
	}
	if !strings.Contains(view2, "delete-1") {
		t.Errorf("ct-danger row 'delete-1' must be visible after cache round-trip\n  view:\n%s", view2)
	}
}

// ===========================================================================
// TestCtrlZ_StatusLineIndicator (§7.3)
//
// When attentionOnly is active, [!] must appear somewhere in the rendered
// output (View(), FrameTitle(), or BottomHints()).
// Location is up to the coder; we check all three surfaces.
// ===========================================================================

func TestCtrlZ_StatusLineIndicator(t *testing.T) {
	m := ctrlZModel(t, "ct-events", ctEvents4())

	// Sanity: [!] must NOT appear before toggle.
	beforeView := stripANSI(m.View())
	beforeTitle := stripANSI(m.FrameTitle())
	if strings.Contains(beforeView, "[!]") || strings.Contains(beforeTitle, "[!]") {
		t.Errorf("[!] indicator present before ctrl+z toggle — should only appear when attentionOnly is active")
	}

	// Toggle on.
	m, _ = m.Update(ctrlZ())

	afterView := stripANSI(m.View())
	afterTitle := stripANSI(m.FrameTitle())

	// Check all surfaces: View() or FrameTitle() must contain [!].
	// BottomHints() returns []layout.KeyHint — join the key/desc strings for the check.
	hintsStr := ""
	for _, h := range m.BottomHints() {
		hintsStr += h.Key + " " + h.Desc + " "
	}

	if !strings.Contains(afterView, "[!]") && !strings.Contains(afterTitle, "[!]") && !strings.Contains(hintsStr, "[!]") {
		t.Errorf(
			"[!] indicator not found in any rendered surface after ctrl+z toggle ON\n"+
				"  §7.3 requires '[!]' next to filter indicator when attentionOnly is active\n"+
				"  FrameTitle: %q\n"+
				"  BottomHints: %q\n"+
				"  View (first 200 chars): %.200s",
			afterTitle, hintsStr, afterView,
		)
	}
}
