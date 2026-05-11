package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// Reveals bug: detail opened when terminal is narrow, then widened.
// RELATED column must appear automatically once width is sufficient.
func TestBugReveal_EC2Detail_AutoShowsRelatedAfterResizeToWide(t *testing.T) {
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	if initCmd := m.Init(); initCmd != nil {
		if initMsg := initCmd(); initMsg != nil {
			m2, _ := rootApplyMsg(m, initMsg)
			m = m2
		}
	}

	// Start narrow so detail initially has no room for related column.
	m2, _ := rootApplyMsg(m, tea.WindowSizeMsg{Width: 59, Height: 36})
	m = m2

	clients := demo.NewServiceClients()
	ec2, err := awsclient.FetchEC2Instances(context.Background(), clients.EC2)
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing: err=%v len=%d", err, len(ec2))
	}
	m2, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2[0],
	})
	m = m2

	narrow := stripANSI(rootViewContent(m))
	if strings.Contains(narrow, "RELATED") {
		t.Fatalf("precondition failed: RELATED should not render at width 59; got:\n%s", narrow)
	}

	// Resize to wide; RELATED should auto-appear without keypress.
	m2, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 140, Height: 36})
	m = m2
	wide := stripANSI(rootViewContent(m))
	if !strings.Contains(wide, "RELATED") {
		t.Fatalf("BUG REVEALED: RELATED column missing after resize to wide terminal; got:\n%s", wide)
	}
}

// User choice guard: once RELATED is explicitly hidden with 'r', resizing should
// not auto-show it again.
func TestBugReveal_EC2Detail_ResizeDoesNotOverrideExplicitHide(t *testing.T) {
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	if initCmd := m.Init(); initCmd != nil {
		if initMsg := initCmd(); initMsg != nil {
			m2, _ := rootApplyMsg(m, initMsg)
			m = m2
		}
	}

	m2, _ := rootApplyMsg(m, tea.WindowSizeMsg{Width: 140, Height: 36})
	m = m2

	clients2 := demo.NewServiceClients()
	ec2b, err := awsclient.FetchEC2Instances(context.Background(), clients2.EC2)
	if err != nil || len(ec2b) == 0 {
		t.Fatalf("demo ec2 fixtures missing: err=%v len=%d", err, len(ec2b))
	}
	m2, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2b[0],
	})
	m = m2

	before := stripANSI(rootViewContent(m))
	if !strings.Contains(before, "RELATED") {
		t.Fatalf("precondition failed: expected RELATED at wide width before explicit toggle; got:\n%s", before)
	}

	// First r hides the auto-shown column.
	m2, _ = rootApplyMsg(m, rootKeyPress("r"))
	m = m2
	hidden := stripANSI(rootViewContent(m))
	if strings.Contains(hidden, "RELATED") {
		t.Fatalf("expected RELATED to be hidden after explicit toggle; got:\n%s", hidden)
	}

	// Resize around breakpoints; explicit hide must be respected.
	m2, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 70, Height: 36})
	m = m2
	m2, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 140, Height: 36})
	m = m2
	after := stripANSI(rootViewContent(m))
	if strings.Contains(after, "RELATED") {
		t.Fatalf("explicit hide should persist across resize; got:\n%s", after)
	}
}
