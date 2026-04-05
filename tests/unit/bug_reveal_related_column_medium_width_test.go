package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// Reveals bug from real UI: on medium terminal widths, EC2 detail renders
// without visible RELATED panel.
func TestBugReveal_EC2Detail_MediumWidth_MustStillShowRelated(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	if initCmd := m.Init(); initCmd != nil {
		if initMsg := initCmd(); initMsg != nil {
			m2, _ := rootApplyMsg(m, initMsg)
			m = m2
		}
	}

	// Medium width similar to user screenshot behavior.
	m2, _ := rootApplyMsg(m, tea.WindowSizeMsg{Width: 95, Height: 36})
	m = m2

	ec2, ok := demo.GetResources("ec2")
	if !ok || len(ec2) == 0 {
		t.Fatal("demo ec2 fixtures missing")
	}
	m2, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2[0],
	})
	m = m2

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail --") {
		t.Fatalf("precondition failed: expected detail view; got:\n%s", view)
	}
	if !strings.Contains(view, "RELATED") {
		t.Fatalf("BUG REVEALED: RELATED panel is not visible at medium width; got:\n%s", view)
	}
}
