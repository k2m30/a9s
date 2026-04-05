package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

func applyWithCmd(m tui.Model, msg tea.Msg) tui.Model {
	next, cmd := rootApplyMsg(m, msg)
	if cmd == nil {
		return next
	}
	produced := cmd()
	if produced == nil {
		return next
	}
	next2, cmd2 := rootApplyMsg(next, produced)
	if cmd2 == nil {
		return next2
	}
	produced2 := cmd2()
	if produced2 == nil {
		return next2
	}
	next3, _ := rootApplyMsg(next2, produced2)
	return next3
}

// Reveals regression from real user journey:
// main menu -> filter "ec2" -> Enter list -> Enter detail.
// At wide width, EC2 detail MUST show RELATED column.
func TestBugReveal_MainMenuToEC2Detail_MustShowRelatedColumn(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))

	if initCmd := m.Init(); initCmd != nil {
		if initMsg := initCmd(); initMsg != nil {
			m, _ = rootApplyMsg(m, initMsg)
		}
	}

	m = applyWithCmd(m, tea.WindowSizeMsg{Width: 170, Height: 50})

	// Filter main menu to EC2 for deterministic selection.
	m = applyWithCmd(m, rootKeyPress("/"))
	m = applyWithCmd(m, rootKeyPress("e"))
	m = applyWithCmd(m, rootKeyPress("c"))
	m = applyWithCmd(m, rootKeyPress("2"))
	m = applyWithCmd(m, rootSpecialKey(tea.KeyEnter))

	// Enter on EC2 menu item -> resource list (with fetched resources).
	m = applyWithCmd(m, rootSpecialKey(tea.KeyEnter))

	// Stabilize list data deterministically for this journey test.
	ec2, ok := demo.GetResources("ec2")
	if !ok || len(ec2) == 0 {
		t.Fatal("demo ec2 fixtures missing")
	}
	m = applyWithCmd(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2,
	})

	viewList := stripANSI(rootViewContent(m))
	if !strings.Contains(strings.ToLower(viewList), "ec2") {
		t.Fatalf("precondition failed: expected ec2 list after first Enter; got:\n%s", viewList)
	}

	// Enter on first EC2 row -> detail.
	m = applyWithCmd(m, rootSpecialKey(tea.KeyEnter))

	viewDetail := stripANSI(rootViewContent(m))
	if !strings.Contains(viewDetail, "detail --") {
		t.Fatalf("expected detail view after second Enter; got:\n%s", viewDetail)
	}
	if !strings.Contains(viewDetail, "RELATED") {
		t.Fatalf("BUG REVEALED: EC2 detail opened from main-menu journey is missing RELATED column; got:\n%s", viewDetail)
	}
}

// Same journey as production invocation style:
// ./a9s -p test-profile --demo
func TestBugReveal_MainMenuToEC2Detail_MustShowRelatedColumn_ProfileFlagInDemo(t *testing.T) {
	m := tui.New("test-profile", "us-east-1", tui.WithDemo(true))

	if initCmd := m.Init(); initCmd != nil {
		if initMsg := initCmd(); initMsg != nil {
			m, _ = rootApplyMsg(m, initMsg)
		}
	}

	m = applyWithCmd(m, tea.WindowSizeMsg{Width: 170, Height: 50})
	m = applyWithCmd(m, rootKeyPress("/"))
	m = applyWithCmd(m, rootKeyPress("e"))
	m = applyWithCmd(m, rootKeyPress("c"))
	m = applyWithCmd(m, rootKeyPress("2"))
	m = applyWithCmd(m, rootSpecialKey(tea.KeyEnter))
	m = applyWithCmd(m, rootSpecialKey(tea.KeyEnter))

	ec2, ok := demo.GetResources("ec2")
	if !ok || len(ec2) == 0 {
		t.Fatal("demo ec2 fixtures missing")
	}
	m = applyWithCmd(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2,
	})

	m = applyWithCmd(m, rootSpecialKey(tea.KeyEnter))

	viewDetail := stripANSI(rootViewContent(m))
	if !strings.Contains(viewDetail, "detail --") {
		t.Fatalf("expected detail view after second Enter; got:\n%s", viewDetail)
	}
	if !strings.Contains(viewDetail, "RELATED") {
		t.Fatalf("BUG REVEALED: EC2 detail is missing RELATED column with -p test-profile --demo; got:\n%s", viewDetail)
	}
}
