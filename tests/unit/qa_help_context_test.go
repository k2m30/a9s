package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui"
	"github.com/k2m30/a9s/internal/tui/messages"
)

// ═══════════════════════════════════════════════════════════════════════════
// HC-01: HELP FROM MAIN MENU
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_MainMenu_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Press ? to open help from main menu
	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))

	// Should show main menu keys (check case-sensitive since G vs g matters)
	mustContain := []string{
		"up/down",    // j/k description
		"top",        // g description
		"bottom",     // G description
		"enter",      // select
		"filter",     // / description
		"command",    // : description
		"quit",       // q description
		"help",       // ? description
		"ctrl+c",     // force quit
		"force quit", // ctrl+c description
		"pgup",       // page up
		"pgdn",       // page down
	}
	for _, key := range mustContain {
		if !strings.Contains(strings.ToLower(plain), key) {
			t.Errorf("HC-01: main menu help should contain %q, got:\n%s", key, plain)
		}
	}
}

func TestQA_HelpContext_MainMenu_ExcludesIrrelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	// Should NOT show keys only for other views
	mustNotContain := []string{
		"detail",  // d key description
		"yaml",    // y key description
		"copy",    // c key description
		"reveal",  // x key description
		"wrap",    // w key description
		"refresh", // ctrl+r description
		// pgup/pgdn are now shown in main menu help
		"sort",    // sort keys
	}
	for _, text := range mustNotContain {
		if strings.Contains(strings.ToLower(plain), text) {
			t.Errorf("HC-01: main menu help should NOT contain %q, got:\n%s", text, plain)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-02: HELP FROM RESOURCE LIST
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_ResourceList_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to ec2 resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Press ? to open help from resource list
	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	mustContain := []string{
		"up/down",     // j/k description
		"top",         // g/G description (top/bottom)
		"pgup",        // page up key
		"pgdn",        // page down key
		"scroll c",    // h/l description (may truncate)
		"enter",       // open
		"detail",      // d key description
		"yaml",        // y key description
		"copy",        // c key description
		"filter",      // / key description
		"sort",        // sort keys description
		"refresh",     // ctrl+r description
		"esc",         // back
	}
	for _, text := range mustContain {
		if !strings.Contains(plainLower, text) {
			t.Errorf("HC-02: resource list help should contain %q, got:\n%s", text, plain)
		}
	}
}

func TestQA_HelpContext_ResourceList_ExcludesIrrelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	// Resource list should NOT show wrap or reveal (ec2 is not secrets)
	mustNotContain := []string{
		"wrap",   // w key - detail/yaml only
		"reveal", // x key - secrets only
	}
	for _, text := range mustNotContain {
		if strings.Contains(strings.ToLower(plain), text) {
			t.Errorf("HC-02: ec2 resource list help should NOT contain %q, got:\n%s", text, plain)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-03: HELP FROM SECRETS RESOURCE LIST INCLUDES REVEAL
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_SecretsResourceList_IncludesReveal(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to secrets resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(strings.ToLower(plain), "reveal") {
		t.Errorf("HC-03: secrets resource list help should contain 'reveal', got:\n%s", plain)
	}
	if !strings.Contains(plain, "x") {
		t.Errorf("HC-03: secrets resource list help should contain 'x' key, got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-04: HELP FROM NON-SECRETS RESOURCE LIST EXCLUDES REVEAL
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_EC2ResourceList_ExcludesReveal(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to ec2 resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	if strings.Contains(strings.ToLower(plain), "reveal") {
		t.Errorf("HC-04: ec2 resource list help should NOT contain 'reveal', got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-05: HELP FROM DETAIL VIEW
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_DetailView_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{ID: "i-abc123", Name: "test-instance"}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	mustContain := []string{
		"up/down",  // j/k description
		"top",      // g description
		"bottom",   // G description
		"yaml",     // y key description
		"copy",     // c key description
		"wrap",     // w key description
		"esc",      // back
	}
	for _, text := range mustContain {
		if !strings.Contains(plainLower, text) {
			t.Errorf("HC-05: detail help should contain %q, got:\n%s", text, plain)
		}
	}
}

func TestQA_HelpContext_DetailView_ExcludesIrrelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{ID: "i-abc123", Name: "test-instance"}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	mustNotContain := []string{
		"detail",  // d key - not in detail view
		"reveal",  // x key
		"filter",  // / key
		"refresh", // ctrl+r
		"pgup",    // pagination
		"pgdn",    // pagination
		"sort",    // sort keys
	}
	for _, text := range mustNotContain {
		if strings.Contains(strings.ToLower(plain), text) {
			t.Errorf("HC-05: detail help should NOT contain %q, got:\n%s", text, plain)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-06: HELP FROM YAML VIEW
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_YAMLView_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{ID: "i-abc123", Name: "test-instance"}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	mustContain := []string{
		"up/down",  // j/k description
		"top",      // g description
		"bottom",   // G description
		"copy",     // c key description
		"wrap",     // w key description
		"esc",      // back
	}
	for _, text := range mustContain {
		if !strings.Contains(plainLower, text) {
			t.Errorf("HC-06: yaml help should contain %q, got:\n%s", text, plain)
		}
	}
}

func TestQA_HelpContext_YAMLView_ExcludesIrrelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{ID: "i-abc123", Name: "test-instance"}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	mustNotContain := []string{
		"detail",  // d key
		"reveal",  // x key
		"filter",  // / key
		"refresh", // ctrl+r
		"pgup",    // pagination
		"pgdn",    // pagination
		"sort",    // sort keys
		"enter",   // enter key (no action in yaml)
	}
	for _, text := range mustNotContain {
		if strings.Contains(strings.ToLower(plain), text) {
			t.Errorf("HC-06: yaml help should NOT contain %q, got:\n%s", text, plain)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-07: HELP FROM PROFILE/REGION SELECTOR
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_RegionSelector_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetRegion})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	mustContain := []string{
		"up/down",  // j/k description
		"top",      // g description
		"bottom",   // G description
		"enter",    // select
		"filter",   // / key description
		"esc",      // cancel
	}
	for _, text := range mustContain {
		if !strings.Contains(plainLower, text) {
			t.Errorf("HC-07: region selector help should contain %q, got:\n%s", text, plain)
		}
	}
}

func TestQA_HelpContext_RegionSelector_ExcludesIrrelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetRegion})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	mustNotContain := []string{
		"detail",  // d key
		"yaml",    // y key
		"copy",    // c key
		"reveal",  // x key
		"wrap",    // w key
		"refresh", // ctrl+r
		"pgup",    // pagination
		"pgdn",    // pagination
		"sort",    // sort keys
	}
	for _, text := range mustNotContain {
		if strings.Contains(strings.ToLower(plain), text) {
			t.Errorf("HC-07: region selector help should NOT contain %q, got:\n%s", text, plain)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-08: HELP FROM REVEAL VIEW
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_RevealView_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Push reveal view via SecretRevealedMsg
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "my-secret",
		Value:      "super-secret-value",
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	mustContain := []string{
		"copy", // c key description
		"esc",  // close
	}
	for _, text := range mustContain {
		if !strings.Contains(strings.ToLower(plain), text) {
			t.Errorf("HC-08: reveal help should contain %q, got:\n%s", text, plain)
		}
	}
}

func TestQA_HelpContext_RevealView_ExcludesIrrelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "my-secret",
		Value:      "super-secret-value",
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	mustNotContain := []string{
		"detail",  // d key
		"yaml",    // y key
		"reveal",  // x key
		"wrap",    // w key
		"refresh", // ctrl+r
		"pgup",    // pagination
		"pgdn",    // pagination
		"sort",    // sort keys
		"filter",  // / key
	}
	for _, text := range mustNotContain {
		if strings.Contains(strings.ToLower(plain), text) {
			t.Errorf("HC-08: reveal help should NOT contain %q, got:\n%s", text, plain)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-10: FRAME TITLE
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_FrameTitle(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "help") {
		t.Errorf("HC-10: help frame title should be 'help', got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-11: ANY KEY CLOSES HELP
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_AnyKeyCloses(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	// Verify we're on help
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Fatal("should be on help view")
	}

	// Press arbitrary key to close
	m, cmd := rootApplyMsg(m, rootKeyPress("a"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	// Should be back at main menu
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("HC-11: after closing help, should return to main menu, got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-13: NARROW TERMINAL
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_NarrowTerminal(t *testing.T) {
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 60, Height: 24})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	// Should still render help without crashing
	if !strings.Contains(plain, "help") {
		t.Errorf("HC-13: help should render at 60 cols, got:\n%s", plain)
	}
	// Key bindings should still be readable
	if !strings.Contains(plain, "esc") {
		t.Errorf("HC-13: key bindings should be readable at 60 cols, got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-14: ? ON HELP CLOSES HELP (NOT HELP-ON-HELP)
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_QuestionMarkOnHelpClosesHelp(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Open help
	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Fatal("should be on help view")
	}

	// Press ? on help -- should close help via PopViewMsg, not open another help
	m, cmd := rootApplyMsg(m, rootKeyPress("?"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("HC-14: pressing ? on help should close help, got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HC-15: HELP PRESERVES VIEW CONTEXT
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_PreservesViewContext(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Open and close help
	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	m, cmd := rootApplyMsg(m, rootKeyPress("a"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	// Should be back at ec2 resource list
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2") {
		t.Errorf("HC-15: after closing help, should return to ec2 list, got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SECRETS vs EC2: FULL COMPARISON
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_SecretsVsEC2_RevealKey(t *testing.T) {
	tui.Version = "0.6.0"

	// Test 1: Secrets should show reveal
	m1 := newRootSizedModel()
	m1, _ = rootApplyMsg(m1, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	m1, _ = rootApplyMsg(m1, rootKeyPress("?"))
	secretsHelp := strings.ToLower(stripANSI(rootViewContent(m1)))

	if !strings.Contains(secretsHelp, "reveal") {
		t.Errorf("secrets help should contain 'reveal', got:\n%s", secretsHelp)
	}

	// Test 2: EC2 should NOT show reveal
	m2 := newRootSizedModel()
	m2, _ = rootApplyMsg(m2, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m2, _ = rootApplyMsg(m2, rootKeyPress("?"))
	ec2Help := strings.ToLower(stripANSI(rootViewContent(m2)))

	if strings.Contains(ec2Help, "reveal") {
		t.Errorf("ec2 help should NOT contain 'reveal', got:\n%s", ec2Help)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// DIFFERENT RESOURCE TYPES ALL SHOW RESOURCE LIST KEYS
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_AllResourceTypes_ShowResourceListKeys(t *testing.T) {
	resourceTypes := resource.AllShortNames()

	for _, rt := range resourceTypes {
		t.Run(rt, func(t *testing.T) {
			tui.Version = "0.6.0"
			m := newRootSizedModel()
			m, _ = rootApplyMsg(m, messages.NavigateMsg{
				Target:       messages.TargetResourceList,
				ResourceType: rt,
			})
			m, _ = rootApplyMsg(m, rootKeyPress("?"))
			plain := strings.ToLower(stripANSI(rootViewContent(m)))

			// All resource lists should show these keys
			for _, key := range []string{"detail", "copy", "sort", "refresh"} {
				if !strings.Contains(plain, key) {
					t.Errorf("HC-02: %s resource list help should contain %q", rt, key)
				}
			}

			// Only secrets should show reveal
			if rt == "secrets" {
				if !strings.Contains(plain, "reveal") {
					t.Errorf("HC-03: secrets help should contain 'reveal'")
				}
			} else {
				if strings.Contains(plain, "reveal") {
					t.Errorf("HC-04: %s help should NOT contain 'reveal'", rt)
				}
			}
		})
	}
}
