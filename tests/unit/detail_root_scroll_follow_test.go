// detail_root_scroll_follow_test.go — TDD red-phase pin for the LIVE app_stack.go
// wiring gap behind the detail-view scroll-follows-cursor bug. The controller-level
// fix (internal/app/detail_cursor.go's reconcileDetailScrollToCursor) is already
// covered by detail_controller_scroll_follow_test.go, but that test calls
// controller.Apply directly with Action.N pre-set — it never exercises the real
// key-dispatch path the running binary uses. A --demo smoke proved the fix does
// NOT work end-to-end: pressing Down on a long detail does not scroll the field
// viewport, because internal/tui/app_stack.go's left-column ActionMoveDown/
// ActionMoveUp/ActionMoveBottom handling (handleDetailKeyMsg) never supplies
// Action.N (the renderer-owned viewport height), unlike the ActionPageDown/
// ActionPageUp handling a few lines below it in the same switch. This test
// drives the ROOT tui.Model.Update the running binary uses, so it fails for the
// exact reason the smoke test failed.
package unit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// rootScrollProbeFieldCount is comfortably larger than the detail viewport
// height at the 200x24 terminal this test sizes (Model.innerSize yields
// rs.height = 24-3 = 21 for a no-related-panel detail screen), so a Down-key
// walk must scroll the field viewport to reach the tail of the list.
const rootScrollProbeFieldCount = 60

// rootScrollProbeDownPresses lands the field cursor at index 30 — comfortably
// past the initial visible window (indices 0..~20 at a 21-line viewport) —
// so the corresponding field label only appears in rendered output once the
// viewport has actually scrolled.
const rootScrollProbeDownPresses = 30

// rootScrollProbeResource returns an unregistered-type resource (Type == "",
// so no related-panel defs auto-show and the left-column key path in
// handleDetailKeyMsg is exercised directly) with rootScrollProbeFieldCount
// plain fields, keyed "field-00".."field-NN" so alphabetical field-item sort
// (the same unregistered-type fallback rendering path exercised by
// manyFieldsResource in detail_controller_scroll_follow_test.go) yields a
// deterministic, section-free, spacer-free field list of that length.
func rootScrollProbeResource() *resource.Resource {
	fields := make(map[string]string, rootScrollProbeFieldCount)
	for i := 0; i < rootScrollProbeFieldCount; i++ {
		key := fmt.Sprintf("field-%02d", i)
		fields[key] = fmt.Sprintf("value-%02d", i)
	}
	return &resource.Resource{
		ID:     "scroll-probe-root-001",
		Name:   "scroll-probe-root",
		Fields: fields,
	}
}

// TestRootDetail_DownKeys_ScrollFollowsCursor pins the end-to-end fix: repeatedly
// pressing Down on a long detail screen through the ROOT model's real key
// dispatcher must scroll the field viewport, the same way a user watching the
// terminal would observe it.
//
// tui.Model.ctrl is unexported (package tui), so DetailBody.ScrollY cannot be
// read directly from this external test package — the scroll is asserted the
// way a user (and the failing --demo smoke) observes it: via rendered View()
// content. Before the fix, ScrollY never advances on this path, so a field
// label deep in the list never becomes visible no matter how many times Down
// is pressed.
func TestRootDetail_DownKeys_ScrollFollowsCursor(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 200, Height: 24})

	res := rootScrollProbeResource()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	// Force a render pass so rs.viewport is populated with a real height —
	// renderDetail (internal/tui/renderer.go) only replaces the zero-value
	// viewport with one sized to rs.height on a View() call. Without this,
	// even the fixed app_stack.go code would read rs.viewport.Height()==0
	// and pass N=0, making reconcileDetailScrollToCursor a no-op regardless.
	before := stripANSI(rootViewContent(m))
	if !strings.Contains(before, "field-00") {
		t.Fatalf("precondition: initial detail render should show field-00 (top of list), got:\n%s", before)
	}
	deepLabel := fmt.Sprintf("field-%02d", rootScrollProbeDownPresses)
	if strings.Contains(before, deepLabel) {
		t.Fatalf("precondition: %s should not already be visible before any Down presses — viewport is taller than expected for this probe (widen rootScrollProbeFieldCount/rootScrollProbeDownPresses margin)", deepLabel)
	}

	for i := 0; i < rootScrollProbeDownPresses; i++ {
		m, _ = rootApplyMsg(m, rootKeyPress("j"))
	}

	after := stripANSI(rootViewContent(m))

	if !strings.Contains(after, deepLabel) {
		t.Errorf("after %d Down presses, the detail viewport did not scroll: %q never became visible in rendered output — internal/tui/app_stack.go's left-column ActionMoveDown handling (handleDetailKeyMsg) must pass Action.N: rs.viewport.Height() so the controller's scroll reconcile (reconcileDetailScrollToCursor) can run, mirroring the ActionPageDown/ActionPageUp handling a few lines below it\n--- rendered output after %d Down presses ---\n%s",
			rootScrollProbeDownPresses, deepLabel, rootScrollProbeDownPresses, after)
	}

	if strings.Contains(after, "field-00") {
		t.Errorf("after %d Down presses, field-00 (top of list) is still visible — the viewport did not scroll away from the top", rootScrollProbeDownPresses)
	}
}
