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

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// frameTitleTestFindings returns 3 distinct ! findings — drives issueCount=3.
func frameTitleTestFindings() map[string]resource.EnrichmentFinding {
	return map[string]resource.EnrichmentFinding{
		"id-1": {Severity: "!", Summary: "instance unhealthy"},
		"id-2": {Severity: "!", Summary: "instance unhealthy"},
		"id-3": {Severity: "!", Summary: "instance unhealthy"},
	}
}

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

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})
	m.SetShowIssueBadge(true)

	findings := frameTitleTestFindings()
	m.SetEnrichmentState(3, true, findings)

	title := m.FrameTitle()

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

	findings := frameTitleTestFindings()
	m.SetEnrichmentState(3, false, findings)

	title := m.FrameTitle()

	if strings.Contains(title, "+") {
		t.Errorf("FrameTitle with enrichmentTruncated=false must NOT contain '+' in issue badge; got %q", title)
	}
}
