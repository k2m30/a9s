// tui_error_marker_parity_test.go — regression pin for the C4 fetch-failure
// error marker's renderer parity on the TUI side (branch feat/cache).
//
// The headless controller's C4 contract (a fetch failure over cached
// content clears Refreshing and sets an error marker, cached rows stay on
// screen) is already pinned end-to-end at the Controller.Handle seam in
// app_pilot_defects_test.go's TestAPIError_OverCachedList_ClearsRefreshing_
// SetsErrorMarker. internal/tui.Model's own ctrl field IS that same
// app.Controller (single source of truth for both renderers), and its
// runtime_adapter.go applyIntent's ClearActiveListLoadingIntent case already
// mirrors the headless intent by calling m.ctrl.SetListFetchError(v.Err) —
// but that write only reaches the SCREEN if the TUI's real Bubble Tea
// Update/View path actually surfaces ListBody.LastFetchError. This test
// drives the real renderer seam (tuitest.Step/Render, exactly as
// tui_savecache_routing_test.go does for its own save-cache routing pin) end to end: a
// list with cached rows on screen, then a real messages.APIError delivered
// through m.Update, asserting the rendered View() carries the
// "── error: ..." marker text (internal/tui/views/resourcelist.go) and the
// rows remain visible.
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// newErrorMarkerApp builds a tui.Model wired to demo clients (no real AWS
// calls), sized so View() renders real content. Mirrors newSaveCacheApp /
// newEnrichApp's construction pattern from sibling test files in this
// package.
func newErrorMarkerApp(t *testing.T) tui.Model {
	t.Helper()
	m := tui.New("errmarker-demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest("errmarker-demo"),
		tui.WithRegionForTest("us-east-1"))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// errTUIFetchFailed is a fixed sentinel error, kept local to this file so
// the test has no dependency on any specific AWS SDK error type — mirrors
// errPilotFetchFailed in app_pilot_defects_test.go (different package, not
// importable from here).
type errTUIFetchFailed struct{}

func (errTUIFetchFailed) Error() string { return "tui pilot: simulated fetch failure" }

// TestTUI_APIError_OverListWithRows_RendersErrorMarker_KeepsRows pins the C4
// error marker's renderer parity at the real Bubble Tea Update/View seam: a list screen
// with rows already landed (via a real messages.ResourcesLoaded through
// m.Update, exactly as the live runtime would deliver a completed fetch),
// then a real messages.APIError landing for the SAME type, must render the
// "── error: ..." marker (resourcelist.go) in View() while the rows stay
// on screen — nothing goes blank.
func TestTUI_APIError_OverListWithRows_RendersErrorMarker_KeepsRows(t *testing.T) {
	m := newErrorMarkerApp(t)

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	rows := []resource.Resource{
		{ID: "bucket-errmarker-1", Name: "errmarker-bucket-1", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
		{ID: "bucket-errmarker-2", Name: "errmarker-bucket-2", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    rows,
	})

	beforeContent := stripANSI(rootViewContent(m))
	if !strings.Contains(beforeContent, "errmarker-bucket-1") {
		t.Fatalf("test setup: rendered view does not show seeded row before the failure:\n%s", beforeContent)
	}

	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "s3",
		Err:          errTUIFetchFailed{},
	})

	afterContent := stripANSI(rootViewContent(m))
	if !strings.Contains(afterContent, "── error:") {
		t.Errorf("rendered view after messages.APIError does not contain the C4 fetch-error marker (\"── error: ...\"):\n%s", afterContent)
	}
	if !strings.Contains(afterContent, "errmarker-bucket-1") || !strings.Contains(afterContent, "errmarker-bucket-2") {
		t.Errorf("rendered view after messages.APIError is missing previously-loaded rows — C4: cached content must remain on screen, nothing goes blank on a fetch failure:\n%s", afterContent)
	}
}
