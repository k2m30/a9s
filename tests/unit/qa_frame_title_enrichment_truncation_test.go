package unit

// qa_frame_title_enrichment_truncation_test.go — Regression: FrameTitle shows "N+"
// when enrichment result is truncated.
//
// Bug: enrichmentTruncated was not factored into the issueStr computation, so
// the badge showed a definitive count even when Wave 2 was capped.
// Fix: issueStr gets "+" suffix when enrichmentTruncated == true AND ic > 0.
//
// Test fails if the fix is reverted: FrameTitle would show "5/3 issues" not "5/3+ issues".

import (
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestFrameTitle_EnrichmentTruncated_ShowsPlus verifies that when enrichmentTruncated
// is true and issueCount > 0, FrameTitle includes "+" in the issue count portion.
func TestFrameTitle_EnrichmentTruncated_ShowsPlus(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	// Load resources — stopped and pending resources will be ColorBroken/ColorWarning
	// via fallbackColor on r.Status, so issueCount > 0 after applyFilter.
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})
	m.SetShowIssueBadge(true)

	// Set enrichment state with truncated=true — this should trigger the "+" in issueStr.
	m.SetEnrichmentState(3, true, true, nil)

	title := m.FrameTitle()

	// The issue count suffix must contain "+".
	if !strings.Contains(title, "+") {
		t.Errorf("FrameTitle with enrichmentTruncated=true must contain '+' in issue badge; got %q — was the enrichmentTruncated check reverted?", title)
	}
}

// TestFrameTitle_EnrichmentNotTruncated_NoPlus verifies that when enrichmentTruncated
// is false and list pagination is nil, FrameTitle does NOT include "+" in issue count.
func TestFrameTitle_EnrichmentNotTruncated_NoPlus(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})
	m.SetShowIssueBadge(true)

	// enrichmentTruncated=false, ran=true — badge should be exact.
	m.SetEnrichmentState(3, false, true, nil)

	title := m.FrameTitle()

	// No "+" expected when truncated=false.
	if strings.Contains(title, "+") {
		t.Errorf("FrameTitle with enrichmentTruncated=false must NOT contain '+' in issue badge; got %q", title)
	}
}
