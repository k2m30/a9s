package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

func openFocusedRelatedDetailForRootFilterTest(t *testing.T) tui.Model {
	t.Helper()

	oldDefs := append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: resource.NoopChecker},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: resource.NoopChecker},
	})
	t.Cleanup(func() { resource.RegisterRelated("ec2", oldDefs) })

	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m = applyRootAndCmd(t, m, tea.WindowSizeMsg{Width: 120, Height: 36})

	res := resource.Resource{
		ID:   "i-root-related-filter",
		Name: "root-related-filter",
		Fields: map[string]string{
			"InstanceId": "i-root-related-filter",
			"ImageId":    "ami-root-related-filter",
			"VpcId":      "vpc-root-related-filter",
			"SubnetId":   "subnet-root-related-filter",
			"State":      "running",
		},
	}

	m = applyRootAndCmd(t, m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &res,
	})

	if !strings.Contains(stripANSI(rootViewContent(m)), "RELATED") {
		t.Fatalf("precondition failed: expected RELATED column on detail view")
	}

	// First r hides the auto-shown panel, second r re-opens it explicitly.
	m = applyRootAndCmd(t, m, rootKeyPress("r"))
	m = applyRootAndCmd(t, m, rootKeyPress("r"))
	m = applyRootAndCmd(t, m, rootSpecialKey(tea.KeyTab))
	return m
}

func TestBug_Root_RightColumnFilter_SlashShowsFilterStateInHeader(t *testing.T) {
	m := openFocusedRelatedDetailForRootFilterTest(t)

	m = applyRootAndCmd(t, m, rootKeyPress("/"))

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(strings.ToLower(plain), "? for help") {
		t.Fatalf("after / in focused related pane, header should leave help hint; got:\n%s", plain)
	}
	if !strings.Contains(plain, "/") {
		t.Fatalf("after / in focused related pane, header should show filter state; got:\n%s", plain)
	}
}

func TestBug_Root_RightColumnFilter_TypingFiltersRows(t *testing.T) {
	m := openFocusedRelatedDetailForRootFilterTest(t)

	m = applyRootAndCmd(t, m, rootKeyPress("/"))
	for _, ch := range "trail" {
		m = applyRootAndCmd(t, m, rootKeyPress(string(ch)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "CloudTrail Events") {
		t.Fatalf("typing in focused related-pane filter should keep matching row visible; got:\n%s", plain)
	}
	if strings.Contains(plain, "CloudWatch Alarms") {
		t.Fatalf("typing in focused related-pane filter should hide non-matching rows; got:\n%s", plain)
	}
	if !strings.Contains(plain, "/trail") {
		t.Fatalf("header should show related-pane filter text, got:\n%s", plain)
	}
}

func TestBug_Root_RightColumnFilter_EscClearsConfirmedFilter(t *testing.T) {
	m := openFocusedRelatedDetailForRootFilterTest(t)

	m = applyRootAndCmd(t, m, rootKeyPress("/"))
	for _, ch := range "trail" {
		m = applyRootAndCmd(t, m, rootKeyPress(string(ch)))
	}
	m = applyRootAndCmd(t, m, rootSpecialKey(tea.KeyEnter))

	confirmed := stripANSI(rootViewContent(m))
	if !strings.Contains(confirmed, "CloudTrail Events") || strings.Contains(confirmed, "CloudWatch Alarms") {
		t.Fatalf("precondition failed: confirmed related filter should keep only matching rows, got:\n%s", confirmed)
	}

	m = applyRootAndCmd(t, m, rootSpecialKey(tea.KeyEscape))

	cleared := stripANSI(rootViewContent(m))
	if !strings.Contains(cleared, "CloudWatch Alarms") {
		t.Fatalf("Esc after confirming right-column filter should clear the filter and restore hidden rows, got:\n%s", cleared)
	}
	if strings.Contains(cleared, "/trail") {
		t.Fatalf("Esc after confirming right-column filter should clear header filter text, got:\n%s", cleared)
	}
}
