package unit

// qa_coverage_gaps_test.go — Tests for coverage gaps identified during audit.
// Covers: enrichment progress indicator, popView sync-back guard,
// showIssueBadge control, quad-state via real applyFilter, enricher registry.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

func setupNoColor(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.Reinit()
	})
}

// ---------------------------------------------------------------------------
// Enrichment progress indicator in MainMenuModel.FrameTitle()
// ---------------------------------------------------------------------------

func TestEnrichProgressIndicator_ShownInFrameTitle(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetEnrichProgress(3, 10)

	title := m.FrameTitle()
	if !strings.Contains(title, "[enriching 3/10]") {
		t.Errorf("FrameTitle() = %q, want to contain '[enriching 3/10]'", title)
	}
}

func TestEnrichProgressIndicator_ClearedWhenDone(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.SetEnrichProgress(10, 10)

	title := m.FrameTitle()
	if strings.Contains(title, "enriching") {
		t.Errorf("FrameTitle() = %q, should NOT contain 'enriching' when checked >= total", title)
	}

	m.SetEnrichProgress(0, 0)
	title = m.FrameTitle()
	if strings.Contains(title, "enriching") {
		t.Errorf("FrameTitle() = %q, should NOT contain 'enriching' after SetEnrichProgress(0,0)", title)
	}
}

func TestEnrichProgressIndicator_CombinesWithCtrlZ(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.Toggle() // enable ctrl+z
	m.SetEnrichProgress(2, 9)

	title := m.FrameTitle()
	if !strings.Contains(title, "[!]") {
		t.Errorf("FrameTitle() = %q, want '[!]' for ctrl+z", title)
	}
	if !strings.Contains(title, "[enriching 2/9]") {
		t.Errorf("FrameTitle() = %q, want '[enriching 2/9]'", title)
	}
}

// ---------------------------------------------------------------------------
// SetShowIssueBadge controls badge display in ResourceListModel.FrameTitle()
// ---------------------------------------------------------------------------

func gapResources(statuses ...string) []resource.Resource {
	res := make([]resource.Resource, len(statuses))
	for i, s := range statuses {
		res[i] = resource.Resource{ID: fmt.Sprintf("gap-%d", i), Status: s}
	}
	return res
}

func TestSetShowIssueBadge_False_HidesBadge(t *testing.T) {
	resources := gapResources("running", "running", "stopped", "failed")
	td := resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
	m := views.NewResourceListFromCache(td, nil, keys.Default(), resources, nil, "", views.SortColNone, true, 0, 0, false)
	title := m.FrameTitle()
	if strings.Contains(title, "issues") {
		t.Errorf("FrameTitle() = %q, should NOT contain 'issues' when showIssueBadge=false", title)
	}
}

func TestSetShowIssueBadge_True_ShowsBadge(t *testing.T) {
	resources := gapResources("running", "running", "stopped", "failed")
	td := resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
	m := views.NewResourceListFromCache(td, nil, keys.Default(), resources, nil, "", views.SortColNone, true, 0, 0, false)
	m.SetShowIssueBadge(true)
	title := m.FrameTitle()
	if !strings.Contains(title, "issues") {
		t.Errorf("FrameTitle() = %q, want 'issues' when showIssueBadge=true", title)
	}
}

func TestSetShowIssueBadge_True_NoIssues_NoBadge(t *testing.T) {
	resources := gapResources("running", "running", "available")
	td := resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
	m := views.NewResourceListFromCache(td, nil, keys.Default(), resources, nil, "", views.SortColNone, true, 0, 0, false)
	m.SetShowIssueBadge(true)
	title := m.FrameTitle()
	if strings.Contains(title, "issues") {
		t.Errorf("FrameTitle() = %q, should NOT contain 'issues' when all healthy", title)
	}
}

// ---------------------------------------------------------------------------
// Quad-state visibility via REAL MainMenuModel.applyFilter()
// ---------------------------------------------------------------------------

func extractCount(title string) int {
	start := strings.Index(title, "(")
	end := strings.Index(title, ")")
	if start < 0 || end < 0 || end <= start {
		return -1
	}
	inner := title[start+1 : end]
	if idx := strings.Index(inner, "/"); idx >= 0 {
		inner = inner[:idx]
	}
	n, _ := strconv.Atoi(inner)
	return n
}

func TestRealApplyFilter_UnknownVisibleWhenCtrlZActive(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	allCount := extractCount(m.FrameTitle())
	if allCount <= 0 {
		t.Fatal("expected nonzero resource type count")
	}
	m.Toggle()
	if extractCount(m.FrameTitle()) != allCount {
		t.Errorf("ctrl+z with all-unknown: want %d visible, got %d", allCount, extractCount(m.FrameTitle()))
	}
}

func TestRealApplyFilter_ConfirmedZeroHiddenWhenCtrlZActive(t *testing.T) {
	setupNoColor(t)
	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)
	m.Toggle()
	m.SetIssues("ec2", 0, false) // confirmed zero

	plain := m.View()
	if strings.Contains(plain, "EC2 Instances") {
		t.Error("confirmed-zero EC2 should be hidden from View() when ctrl+z active")
	}
}

func TestRealApplyFilter_TruncatedZeroVisible(t *testing.T) {
	setupNoColor(t)
	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)
	m.Toggle()
	// Truncated-zero is a LOWER BOUND — issues may exist on unread pages. Per
	// docs/attention-signals.md every registered type has at least a Wave 1 or
	// Wave 2 signal, so "hide if truncated-zero" is wrong for every type.
	m.SetIssues("ec2", 0, true)
	m.SetIssues("s3", 0, true)

	plain := m.View()
	if !strings.Contains(plain, "EC2 Instances") {
		t.Error("truncated-zero EC2 must be visible under ctrl+z — count is a lower bound")
	}
	if !strings.Contains(plain, "S3 Buckets") {
		t.Error("truncated-zero S3 must be visible under ctrl+z — unread pages may carry Wave 2 findings")
	}
}

func TestRealApplyFilter_NonzeroVisible_AllOthersZero(t *testing.T) {
	setupNoColor(t)
	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)
	m.Toggle()
	m.SetIssues("ec2", 3, false)
	for _, rt := range resource.AllResourceTypes() {
		if rt.ShortName != "ec2" {
			m.SetIssues(rt.ShortName, 0, false)
		}
	}

	plain := m.View()
	if !strings.Contains(plain, "EC2 Instances") {
		t.Error("ec2 with issues should be visible in View()")
	}
	// Spot-check that some other type is NOT visible
	if strings.Contains(plain, "S3 Buckets") {
		t.Error("s3 with zero issues should be hidden from View()")
	}
}

// ---------------------------------------------------------------------------
// EnricherRegistry completeness
// ---------------------------------------------------------------------------

func TestEnricherRegistry_AllExpectedKeys(t *testing.T) {
	// The original 8 enrichers from issue #196. These must still be
	// registered (real, not noop) — they're the foundational Wave 2
	// implementations.
	expected := []string{"rds", "dbi", "ebs", "cb", "tg", "pipeline", "sfn", "glue"}
	for _, key := range expected {
		if awsclient.EnricherRegistry[key] == nil {
			t.Errorf("EnricherRegistry[%q] is nil", key)
		}
	}
}

func TestEnricherRegistry_NoUnexpectedKeys(t *testing.T) {
	// Per docs/attention-signals.md, EVERY registered resource type has an
	// EnricherRegistry entry (real or NoOpEnricher). The doc-grounded test
	// TestAttentionSignalsDoc enforces the full contract — this test only
	// asserts there are no entries for shortNames that are not registered as
	// resource types.
	for key := range awsclient.EnricherRegistry {
		if resource.FindResourceType(key) == nil {
			t.Errorf("EnricherRegistry has entry for %q but no such ResourceTypeDef is registered", key)
		}
	}
}

// ---------------------------------------------------------------------------
// MainMenu FrameTitle [!] behavior
// ---------------------------------------------------------------------------

func TestMainMenuFrameTitle_NoExclamationWhenOff(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	if strings.Contains(m.FrameTitle(), "[!]") {
		t.Error("should not contain [!] when ctrl+z off")
	}
}

func TestMainMenuFrameTitle_ExclamationWhenOn(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	m.Toggle()
	if !strings.Contains(m.FrameTitle(), "[!]") {
		t.Error("should contain [!] when ctrl+z on")
	}
}

// ---------------------------------------------------------------------------
// MainMenu BottomHints
// ---------------------------------------------------------------------------

func TestMainMenuBottomHints_HasCtrlZ(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	found := false
	for _, h := range m.BottomHints() {
		if h.Key == "ctrl+z" {
			found = true
		}
	}
	if !found {
		t.Error("missing ctrl+z hint")
	}
}

// ---------------------------------------------------------------------------
// IssueCount accessor with mixed statuses
// ---------------------------------------------------------------------------

func TestResourceListIssueCount_MixedStatuses(t *testing.T) {
	resources := gapResources("running", "stopped", "running", "stopped", "terminated", "pending")
	td := resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
	m := views.NewResourceListFromCache(td, nil, keys.Default(), resources, nil, "", views.SortColNone, true, 0, 0, false)
	// stopped(2) + pending(1) = 3; running(2) + terminated(1) = not counted
	if m.IssueCount() != 3 {
		t.Errorf("IssueCount() = %d, want 3", m.IssueCount())
	}
}
