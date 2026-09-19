package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

func TestBugReveal_EC2Detail_AutoShowsRelatedAfterResizeToWide(t *testing.T) {
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	if initCmd := m.Init(); initCmd != nil {
		if initMsg := initCmd(); initMsg != nil {
			m2, _ := rootApplyMsg(m, initMsg)
			m = m2
		}
	}

	m2, _ := rootApplyMsg(m, tea.WindowSizeMsg{Width: 59, Height: 36})
	m = m2

	clients := demo.NewServiceClients()
	ec2, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), clients.EC2, token)
	})
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

	m2, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 140, Height: 36})
	m = m2
	wide := stripANSI(rootViewContent(m))
	if !strings.Contains(wide, "RELATED") {
		t.Fatalf("BUG REVEALED: RELATED column missing after resize to wide terminal; got:\n%s", wide)
	}
}

func TestBugReveal_EC2Detail_ResizeDoesNotOverrideExplicitHide(t *testing.T) {
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	if initCmd := m.Init(); initCmd != nil {
		if initMsg := initCmd(); initMsg != nil {
			m2, _ := rootApplyMsg(m, initMsg)
			m = m2
		}
	}

	m2, _ := rootApplyMsg(m, tea.WindowSizeMsg{Width: 140, Height: 36})
	m = m2

	clients2 := demo.NewServiceClients()
	ec2b, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), clients2.EC2, token)
	})
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

	m2, _ = rootApplyMsg(m, rootKeyPress("r"))
	m = m2
	hidden := stripANSI(rootViewContent(m))
	if strings.Contains(hidden, "RELATED") {
		t.Fatalf("expected RELATED to be hidden after explicit toggle; got:\n%s", hidden)
	}

	m2, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 70, Height: 36})
	m = m2
	m2, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 140, Height: 36})
	m = m2
	after := stripANSI(rootViewContent(m))
	if strings.Contains(after, "RELATED") {
		t.Fatalf("explicit hide should persist across resize; got:\n%s", after)
	}
}
