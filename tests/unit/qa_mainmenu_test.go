package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ---------------------------------------------------------------------------
// A. Resource Type Listing
// ---------------------------------------------------------------------------

func TestQA_MainMenu_AllSevenResourceTypesVisible(t *testing.T) {
	tui.Version = "1.0.2"
	// Use a tall terminal so all resource types are visible without scrolling
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 90})
	plain := stripANSI(rootViewContent(m))
	lines := strings.Split(plain, "\n")

	allTypes := resource.AllResourceTypes()
	for _, rt := range allTypes {
		// Menu renders Aliases[0] when present, else ShortName — match that logic.
		aliasKey := rt.ShortName
		if len(rt.Aliases) > 0 {
			aliasKey = rt.Aliases[0]
		}
		alias := ":" + aliasKey
		found := false
		for _, line := range lines {
			if strings.Contains(line, rt.Name) && strings.Contains(line, alias) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("main menu: no single line contains both %q and %q", rt.Name, alias)
		}
	}
}

func TestQA_MainMenu_EachRowShowsAlias(t *testing.T) {
	tui.Version = "1.0.2"
	// Use a tall terminal so all resource types are visible without scrolling
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 90})
	plain := stripANSI(rootViewContent(m))

	allTypes := resource.AllResourceTypes()
	aliases := make([]string, len(allTypes))
	for i, rt := range allTypes {
		// Menu renders Aliases[0] when present, else ShortName — match that logic.
		aliasKey := rt.ShortName
		if len(rt.Aliases) > 0 {
			aliasKey = rt.Aliases[0]
		}
		aliases[i] = ":" + aliasKey
	}
	for _, alias := range aliases {
		if !strings.Contains(plain, alias) {
			t.Errorf("main menu should contain alias %q, got:\n%s", alias, plain)
		}
	}
}

func TestQA_MainMenu_DisplayNameAndAliasOnSameRow(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()
	content := rootViewContent(m)
	lines := strings.Split(content, "\n")

	pairs := []struct {
		name  string
		alias string
	}{
		{"EC2 Instances", ":ec2"},
		{"ECS Services", ":ecs-svc"},
		{"ECS Clusters", ":ecs"},
		{"Lambda Functions", ":lambda"},
		{"EKS Clusters", ":eks"},
		{"EKS Node Groups", ":ng"},
		{"Load Balancers", ":elb"},
		{"Security Groups", ":sg"},
		{"VPCs", ":vpc"},
		{"S3 Buckets", ":s3"},
	}

	for _, pair := range pairs {
		found := false
		for _, line := range lines {
			plain := stripANSI(line)
			if strings.Contains(plain, pair.name) && strings.Contains(plain, pair.alias) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q and %q on the same line", pair.name, pair.alias)
		}
	}
}

func TestQA_MainMenu_FirstRowSelectedByDefault(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()
	content := rootViewContent(m)
	lines := strings.SplitSeq(content, "\n")

	// EC2 Instances is the first resource type in AllResourceTypes()
	// The selected row should have different rendering (ANSI sequences for bold/bg)
	// than non-selected rows. We verify by checking raw content (with ANSI) for
	// EC2 Instances line having background styling (escape codes present).
	for line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "EC2 Instances") {
			// Selected row should have ANSI styling that differs from raw text
			if line == plain {
				t.Error("first row (EC2 Instances) should have ANSI styling (selected highlight)")
			}
			return
		}
	}
	t.Error("EC2 Instances not found in rendered output")
}

func TestQA_MainMenu_ExactlySevenResourceRows(t *testing.T) {
	tui.Version = "1.0.2"
	// Use a tall terminal so all resource types are visible without scrolling
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 90})
	plain := stripANSI(rootViewContent(m))
	lines := strings.Split(plain, "\n")

	allTypes := resource.AllResourceTypes()
	resourceNames := make([]string, len(allTypes))
	for i, rt := range allTypes {
		resourceNames[i] = rt.Name
	}
	count := 0
	for _, line := range lines {
		for _, name := range resourceNames {
			if strings.Contains(line, name) {
				count++
				break
			}
		}
	}
	if count != len(allTypes) {
		t.Errorf("expected exactly %d resource type rows, got %d", len(allTypes), count)
	}
}

// ---------------------------------------------------------------------------
// B. Navigation
// ---------------------------------------------------------------------------

func TestQA_MainMenu_MoveDownWithJ(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Default cursor is on EC2 Instances (index 0). Press j to move to ECS Services (index 1).
	m, _ = rootApplyMsg(m, rootKeyPress("j"))

	// Now press Enter to confirm which item is selected.
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ecs-svc" {
		t.Errorf("after j, selected should be ecs-svc, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_MoveDownWithDownArrow(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyDown))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ecs-svc" {
		t.Errorf("after down arrow, selected should be ecs-svc, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_MoveUpWithK(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Move down first, then up
	m, _ = rootApplyMsg(m, rootKeyPress("j"))
	m, _ = rootApplyMsg(m, rootKeyPress("k"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ec2" {
		t.Errorf("after j then k, selected should be ec2, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_MoveUpWithUpArrow(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyDown))
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyUp))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ec2" {
		t.Errorf("after down then up, selected should be ec2, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_CursorStopsAtBottom(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Press G to go to bottom (SES Identities), then j should stay at bottom
	m, _ = rootApplyMsg(m, rootKeyPress("G"))
	m, _ = rootApplyMsg(m, rootKeyPress("j"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ses" {
		t.Errorf("j at bottom should stay on ses, got %q", nav.ResourceType)
	}
}

// ---------------------------------------------------------------------------
// B2. Page Up / Page Down
// ---------------------------------------------------------------------------

func TestQA_MainMenu_PageDownMovesMultipleItems(t *testing.T) {
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Press PageDown — cursor should jump past the first few items
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyPgDown))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	nav := msg.(messages.NavigateMsg)
	// Cursor should NOT still be on ec2 (index 0)
	if nav.ResourceType == "ec2" {
		t.Error("after PageDown, cursor should have moved past ec2")
	}
}

func TestQA_MainMenu_PageUpMovesMultipleItems(t *testing.T) {
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Go to bottom, then PageUp — cursor should NOT be on the last item
	m, _ = rootApplyMsg(m, rootKeyPress("G"))
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyPgUp))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	nav := msg.(messages.NavigateMsg)
	if nav.ResourceType == "ses" {
		t.Error("after PageUp from bottom, cursor should have moved up from ses")
	}
}

func TestQA_MainMenu_PageDownClampsAtBottom(t *testing.T) {
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Press PageDown many times — should clamp at last item
	for range 20 {
		m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyPgDown))
	}

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	nav := msg.(messages.NavigateMsg)
	if nav.ResourceType != "ses" {
		t.Errorf("repeated PageDown should end on ses, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_PageUpClampsAtTop(t *testing.T) {
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Move down a bit, then PageUp many times — should clamp at first item
	for range 5 {
		m, _ = rootApplyMsg(m, rootKeyPress("j"))
	}
	for range 20 {
		m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyPgUp))
	}

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	nav := msg.(messages.NavigateMsg)
	if nav.ResourceType != "ec2" {
		t.Errorf("repeated PageUp should end on ec2, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_CtrlD_PageDown(t *testing.T) {
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Ctrl+D should work as PageDown
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	nav := msg.(messages.NavigateMsg)
	if nav.ResourceType == "ec2" {
		t.Error("after Ctrl+D, cursor should have moved past ec2")
	}
}

func TestQA_MainMenu_CtrlU_PageUp(t *testing.T) {
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Go to bottom, then Ctrl+U should work as PageUp
	m, _ = rootApplyMsg(m, rootKeyPress("G"))
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	nav := msg.(messages.NavigateMsg)
	if nav.ResourceType == "ses" {
		t.Error("after Ctrl+U from bottom, cursor should have moved up from ses")
	}
}

func TestQA_MainMenu_CursorStopsAtTop(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Already at top, press k should stay at top
	m, _ = rootApplyMsg(m, rootKeyPress("k"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ec2" {
		t.Errorf("k at top should stay on ec2, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_JumpToTopWithG(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Move down 4 times, then press g
	for range 4 {
		m, _ = rootApplyMsg(m, rootKeyPress("j"))
	}
	m, _ = rootApplyMsg(m, rootKeyPress("g"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ec2" {
		t.Errorf("after g, should be at top (ec2), got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_JumpToBottomWithShiftG(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("G"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ses" {
		t.Errorf("after G, should be at bottom (ses), got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_GOnFirstRowIsNoop(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Already at first row, press g
	m, _ = rootApplyMsg(m, rootKeyPress("g"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ec2" {
		t.Errorf("g on first row should stay on ec2, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_ShiftGOnLastRowIsNoop(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("G"))
	m, _ = rootApplyMsg(m, rootKeyPress("G"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ses" {
		t.Errorf("G on last row should stay on ses, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_EnterOnEachResourceType(t *testing.T) {
	// Resource types in order as defined in AllResourceTypes()
	expectedTypes := resource.AllShortNames()

	for i, expected := range expectedTypes {
		t.Run(expected, func(t *testing.T) {
			tui.Version = "1.0.2"
			m := newRootSizedModel()

			// Navigate to position i
			for range i {
				m, _ = rootApplyMsg(m, rootKeyPress("j"))
			}

			// Press Enter
			_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
			if cmd == nil {
				t.Fatal("Enter should produce a navigate command")
			}
			msg := cmd()
			nav, ok := msg.(messages.NavigateMsg)
			if !ok {
				t.Fatalf("expected NavigateMsg, got %T", msg)
			}
			if nav.Target != messages.TargetResourceList {
				t.Errorf("expected TargetResourceList, got %d", nav.Target)
			}
			if nav.ResourceType != expected {
				t.Errorf("expected resource type %q, got %q", expected, nav.ResourceType)
			}
		})
	}
}

func TestQA_MainMenu_RapidJPresses(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Press j 5 times -> should land on index 5 (Auto Scaling Groups)
	for range 5 {
		m, _ = rootApplyMsg(m, rootKeyPress("j"))
	}

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "asg" {
		t.Errorf("after 5 j presses, should be on asg, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_MultipleJThenGReturnsToTop(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	for range 4 {
		m, _ = rootApplyMsg(m, rootKeyPress("j"))
	}
	m, _ = rootApplyMsg(m, rootKeyPress("g"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav := msg.(messages.NavigateMsg)
	if nav.ResourceType != "ec2" {
		t.Errorf("after j*4 then g, should be at ec2, got %q", nav.ResourceType)
	}
}

// ---------------------------------------------------------------------------
// K. Viewport Scrolling (small terminal)
// ---------------------------------------------------------------------------

func TestMainMenu_Viewport_CursorVisibleWhenScrolledDown(t *testing.T) {
	tui.Version = "test"
	// Height 10: innerSize.h = 7, so only 7 render lines visible
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Move cursor to item index 11 (EKS Node Groups, 0-indexed)
	for range 11 {
		m, _ = rootApplyMsg(m, rootKeyPress("j"))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "EKS Node Groups") {
		t.Errorf("item at cursor (index 11) should be visible after scrolling down, got:\n%s", plain)
	}
}

func TestMainMenu_Viewport_BottomKey_LastItemVisible(t *testing.T) {
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Press G to go to bottom
	m, _ = rootApplyMsg(m, rootKeyPress("G"))

	plain := stripANSI(rootViewContent(m))
	allTypes := resource.AllResourceTypes()
	lastName := allTypes[len(allTypes)-1].Name
	if !strings.Contains(plain, lastName) {
		t.Errorf("last item %q should be visible after G key, got:\n%s", lastName, plain)
	}
}

func TestMainMenu_Viewport_TopAfterScroll_FirstItemVisible(t *testing.T) {
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Scroll to bottom, then back to top
	m, _ = rootApplyMsg(m, rootKeyPress("G"))
	m, _ = rootApplyMsg(m, rootKeyPress("g"))

	plain := stripANSI(rootViewContent(m))
	firstName := resource.AllResourceTypes()[0].Name
	if !strings.Contains(plain, firstName) {
		t.Errorf("first item %q should be visible after g key, got:\n%s", firstName, plain)
	}
}

func TestMainMenu_Viewport_ScrolledDown_EnterSelectsCorrectItem(t *testing.T) {
	tui.Version = "test"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Move to last item
	m, _ = rootApplyMsg(m, rootKeyPress("G"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter on last item should produce a command")
	}
	msg := cmd()
	nav := msg.(messages.NavigateMsg)
	allTypes := resource.AllResourceTypes()
	expected := allTypes[len(allTypes)-1].ShortName
	if nav.ResourceType != expected {
		t.Errorf("expected %q after G+Enter, got %q", expected, nav.ResourceType)
	}
}

func TestMainMenu_Viewport_OnlyVisibleRowsRendered(t *testing.T) {
	tui.Version = "test"
	// Height 10: innerSize.h = 7, so 7 render lines visible.
	// With category headers, the first 7 lines are:
	//   COMPUTE (header), EC2, ECS Services, ECS Clusters, ECS Tasks, Lambda, ASG
	// So items 0-5 (6 items) are visible, item 6+ not visible.
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	plain := stripANSI(rootViewContent(m))
	allTypes := resource.AllResourceTypes()

	// Items 0-5 should be visible (EC2 through ASG)
	for i := range 6 {
		if !strings.Contains(plain, allTypes[i].Name) {
			t.Errorf("item %d (%s) should be visible at scroll offset 0", i, allTypes[i].Name)
		}
	}

	// Item 6+ should NOT be visible (they are below the viewport)
	for i := 7; i < len(allTypes); i++ {
		if strings.Contains(plain, allTypes[i].Name) {
			t.Errorf("item %d (%s) should NOT be visible at scroll offset 0 (viewport is 7 render lines)", i, allTypes[i].Name)
		}
	}
}

// ---------------------------------------------------------------------------
// L. Category Headers
// ---------------------------------------------------------------------------

func TestQA_MainMenu_CategoryHeadersVisible(t *testing.T) {
	tui.Version = "1.0.2"
	// Use a tall terminal so all items are visible
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 90})
	plain := stripANSI(rootViewContent(m))

	expectedCategories := []string{
		"COMPUTE",
		"CONTAINERS",
		"NETWORKING",
		"DATABASES & STORAGE",
		"MONITORING",
		"MESSAGING",
		"SECRETS & CONFIG",
		"DNS & CDN",
		"SECURITY & IAM",
		"CI/CD",
		"DATA & ANALYTICS",
		"BACKUP",
	}
	for _, cat := range expectedCategories {
		if !strings.Contains(plain, cat) {
			t.Errorf("main menu should contain category header %q", cat)
		}
	}
}

func TestQA_MainMenu_CategoryHeadersNotSelectable(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Press Enter on the first item — should be EC2 Instances, not COMPUTE header
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a navigate command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ec2" {
		t.Errorf("first selectable item should be ec2, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_CategoryHeaderAppearsBeforeFirstItem(t *testing.T) {
	tui.Version = "1.0.2"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 90})
	plain := stripANSI(rootViewContent(m))
	lines := strings.Split(plain, "\n")

	// COMPUTE header should appear before EC2 Instances
	computeLine := -1
	ec2Line := -1
	for i, line := range lines {
		if strings.Contains(line, "COMPUTE") && !strings.Contains(line, "EC2") {
			if computeLine == -1 {
				computeLine = i
			}
		}
		if strings.Contains(line, "EC2 Instances") {
			ec2Line = i
			break
		}
	}

	if computeLine == -1 {
		t.Fatal("COMPUTE header not found in output")
	}
	if ec2Line == -1 {
		t.Fatal("EC2 Instances not found in output")
	}
	if computeLine >= ec2Line {
		t.Errorf("COMPUTE header (line %d) should appear before EC2 Instances (line %d)", computeLine, ec2Line)
	}
}

func TestQA_MainMenu_AllResourceTypesHaveCategory(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	for _, rt := range allTypes {
		if rt.Category == "" {
			t.Errorf("resource type %q (%s) is missing a Category", rt.Name, rt.ShortName)
		}
	}
}

func TestQA_MainMenu_CategoryOrderMatchesSpec(t *testing.T) {
	allTypes := resource.AllResourceTypes()

	// Verify the category order matches the spec
	expectedOrder := []string{
		"COMPUTE",
		"CONTAINERS",
		"NETWORKING",
		"DATABASES & STORAGE",
		"MONITORING",
		"MESSAGING",
		"SECRETS & CONFIG",
		"DNS & CDN",
		"SECURITY & IAM",
		"CI/CD",
		"DATA & ANALYTICS",
		"BACKUP",
	}

	seen := make([]string, 0)
	lastCat := ""
	for _, rt := range allTypes {
		if rt.Category != lastCat {
			seen = append(seen, rt.Category)
			lastCat = rt.Category
		}
	}

	if len(seen) != len(expectedOrder) {
		t.Fatalf("expected %d categories, got %d: %v", len(expectedOrder), len(seen), seen)
	}
	for i, cat := range expectedOrder {
		if seen[i] != cat {
			t.Errorf("category at position %d should be %q, got %q", i, cat, seen[i])
		}
	}
}

func TestQA_MainMenu_FirstItemIsEC2(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if allTypes[0].ShortName != "ec2" {
		t.Errorf("first resource type should be ec2, got %q", allTypes[0].ShortName)
	}
}

func TestQA_MainMenu_LastItemIsSES(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	last := allTypes[len(allTypes)-1]
	if last.ShortName != "ses" {
		t.Errorf("last resource type should be ses, got %q", last.ShortName)
	}
}

func TestQA_MainMenu_FilterHidesCategoriesWithNoMatches(t *testing.T) {
	tui.Version = "1.0.2"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 90})

	// Enter filter mode and type "ec2" (matches only EC2 Instances)
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, ch := range "ec2" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	plain := stripANSI(rootViewContent(m))
	// COMPUTE should be visible (EC2 is in COMPUTE)
	if !strings.Contains(plain, "COMPUTE") {
		t.Error("COMPUTE category should be visible when EC2 matches filter")
	}
	// NETWORKING should NOT be visible (no networking items match "ec2")
	if strings.Contains(plain, "NETWORKING") {
		t.Error("NETWORKING category should be hidden when no items match filter 'ec2'")
	}
}

func TestQA_MainMenu_FirstHeaderVisibleAfterScrollDownAndBackUp(t *testing.T) {
	tui.Version = "test"
	// Small viewport so scrolling is required
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 10})

	// Scroll down past the first header
	for range 10 {
		m, _ = rootApplyMsg(m, rootKeyPress("j"))
	}

	// Now scroll all the way back up to the first item
	m, _ = rootApplyMsg(m, rootKeyPress("g"))

	plain := stripANSI(rootViewContent(m))
	// The COMPUTE header (render line 0) must be visible when cursor is on EC2 (item 0)
	if !strings.Contains(plain, "COMPUTE") {
		t.Errorf("COMPUTE header should be visible after scrolling back to top, got:\n%s", plain)
	}
	if !strings.Contains(plain, "EC2 Instances") {
		t.Errorf("EC2 Instances should be visible after scrolling back to top, got:\n%s", plain)
	}
}

func TestQA_MainMenu_ScrollAccountsForHeaders(t *testing.T) {
	tui.Version = "test"
	// Small viewport: only 5 content lines
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 8})

	// Navigate to item index 10 (EKS Clusters, first CONTAINERS item)
	for range 10 {
		m, _ = rootApplyMsg(m, rootKeyPress("j"))
	}

	plain := stripANSI(rootViewContent(m))
	// EKS Clusters should be visible
	if !strings.Contains(plain, "EKS Clusters") {
		t.Errorf("EKS Clusters should be visible after scrolling, got:\n%s", plain)
	}
	// CONTAINERS header should also be visible since it precedes EKS Clusters
	if !strings.Contains(plain, "CONTAINERS") {
		t.Errorf("CONTAINERS header should be visible when EKS Clusters is visible, got:\n%s", plain)
	}
}
