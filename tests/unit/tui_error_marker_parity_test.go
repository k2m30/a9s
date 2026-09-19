// The fetch-failure error marker on the TUI
// renderer. internal/tui.Model's ctrl field is the same app.Controller the
// headless lane uses, and its ClearActiveListLoadingIntent case calls
// m.ctrl.SetListFetchError(v.Err); the write reaches the screen only if the
// real Bubble Tea Update/View path surfaces ListBody.LastFetchError, so this
// drives that seam (tuitest.Step/Render) end to end.
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
	m := newBlessedModel(t, "errmarker-demo", "us-east-1",
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

// TestTUI_APIError_OverListWithRows_RendersErrorMarker_KeepsRows pins the
// error marker at the real Bubble Tea Update/View seam: a list screen
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
		Resources:    rows, Provenance: messages.FetchProvenanceCanonicalList,
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
