package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ═══════════════════════════════════════════════════════════════════════════
// HELP FROM MAIN MENU
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_MainMenu_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

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

	mustNotContain := []string{
		"detail",  // d key description
		"yaml",    // y key description
		"copy",    // c key description
		"reveal",  // x key description
		"wrap",    // w key description
		"refresh", // ctrl+r description

		"sort", // sort keys
	}
	for _, text := range mustNotContain {
		if strings.Contains(strings.ToLower(plain), text) {
			t.Errorf("HC-01: main menu help should NOT contain %q, got:\n%s", text, plain)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HELP FROM RESOURCE LIST
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_ResourceList_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	mustContain := []string{
		"up/down",  // j/k description
		"top",      // g/G description (top/bottom)
		"pgup",     // page up key
		"pgdn",     // page down key
		"scroll c", // h/l description (may truncate)
		"enter",    // open
		"detail",   // d key description
		"yaml",     // y key description
		"copy",     // c key description
		"filter",   // / key description
		"sort",     // sort keys description
		"refresh",  // ctrl+r description
		"esc",      // back
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

	m, _ = rootApplyMsg(m, messages.Navigate{
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
// HELP FROM SECRETS RESOURCE LIST INCLUDES REVEAL
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_SecretsResourceList_IncludesReveal(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
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
// HELP FROM NON-SECRETS RESOURCE LIST EXCLUDES REVEAL
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_EC2ResourceList_ExcludesReveal(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
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
// HELP FROM DETAIL VIEW
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_DetailView_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{ID: "i-abc123", Name: "test-instance"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	mustContain := []string{
		"up/down", // j/k description
		"top",     // g description
		"bottom",  // G description
		"yaml",    // y key description
		"copy",    // c key description
		"wrap",    // w key description
		"esc",     // back
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
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	mustNotContain := []string{
		"detail",  // d key - not in detail view
		"reveal",  // x key
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
// HELP FROM YAML VIEW
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_YAMLView_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{ID: "i-abc123", Name: "test-instance"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	mustContain := []string{
		"up/down", // j/k description
		"top",     // g description
		"bottom",  // G description
		"copy",    // c key description
		"wrap",    // w key description
		"esc",     // back
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
	m, _ = rootApplyMsg(m, messages.Navigate{
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
// HELP FROM PROFILE/REGION SELECTOR
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_RegionSelector_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	mustContain := []string{
		"up/down", // j/k description
		"top",     // g description
		"bottom",  // G description
		"enter",   // select
		"filter",  // / key description
		"esc",     // cancel
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

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

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
// HELP FROM REVEAL VIEW
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_RevealView_ShowsRelevantKeys(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceID: "my-secret",
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

	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceID: "my-secret",
		Value:      "super-secret-value",
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	mustNotContain := []string{
		"detail",  // d key
		"yaml",    // y key
		"reveal",  // x key
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

	// Reveal view supports wrap toggle (w key)
	if !strings.Contains(strings.ToLower(plain), "wrap") {
		t.Error("HC-08: reveal help should contain wrap toggle (w key)")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HELP TEXT MATCHES ACTUAL BEHAVIOR
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_DetailView_CopySaysValue(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{ID: "i-abc123", Name: "test-instance"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	// Detail view c key copies the active field value — help should say "copy value"
	if strings.Contains(plainLower, "copy id") {
		t.Errorf("HC-09: detail help should say 'copy value' not 'copy id', got:\n%s", plain)
	}
	if !strings.Contains(plainLower, "copy value") {
		t.Errorf("HC-09: detail help should contain 'copy value', got:\n%s", plain)
	}
}

func TestQA_HelpContext_ResourceList_SortLabelsAccurate(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	// Mnemonic sort labels must not appear.
	if strings.Contains(plainLower, "sort age") {
		t.Errorf("HC-09: old 'sort age' label must not appear; positional sort replaced mnemonic sort, got:\n%s", plain)
	}
	if strings.Contains(plainLower, "sort date") {
		t.Errorf("HC-09: old 'sort date' label must not appear; positional sort replaced mnemonic sort, got:\n%s", plain)
	}
	if strings.Contains(plainLower, "sort name") {
		t.Errorf("HC-09: old 'sort name' label must not appear; positional sort replaced mnemonic sort, got:\n%s", plain)
	}
	if strings.Contains(plainLower, "sort status") {
		t.Errorf("HC-09: old 'sort status' label must not appear; positional sort replaced mnemonic sort, got:\n%s", plain)
	}
	if strings.Contains(plainLower, "sort id") {
		t.Errorf("HC-09: old 'sort id' label must not appear; positional sort replaced mnemonic sort, got:\n%s", plain)
	}

	// Keys "1"-"0" sort by column position.
	if !strings.Contains(plainLower, "sort col") {
		t.Errorf("HC-09: resource list help should contain 'sort col' (positional sort labels), got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FRAME TITLE
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
// ANY KEY CLOSES HELP
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_AnyKeyCloses(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Fatal("should be on help view")
	}

	m, cmd := rootApplyMsg(m, rootKeyPress("a"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("HC-11: after closing help, should return to main menu, got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// NARROW TERMINAL
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_NarrowTerminal(t *testing.T) {
	tui.Version = "0.6.0"
	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 60, Height: 24})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "help") {
		t.Errorf("HC-13: help should render at 60 cols, got:\n%s", plain)
	}
	if !strings.Contains(plain, "esc") {
		t.Errorf("HC-13: key bindings should be readable at 60 cols, got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// ? ON HELP CLOSES HELP (NOT HELP-ON-HELP)
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_QuestionMarkOnHelpClosesHelp(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

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
// HELP PRESERVES VIEW CONTEXT
// ═══════════════════════════════════════════════════════════════════════════

func TestQA_HelpContext_PreservesViewContext(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	m, cmd := rootApplyMsg(m, rootKeyPress("a"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

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

	m1 := newRootSizedModel()
	m1, _ = rootApplyMsg(m1, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})
	m1, _ = rootApplyMsg(m1, rootKeyPress("?"))
	secretsHelp := strings.ToLower(stripANSI(rootViewContent(m1)))

	if !strings.Contains(secretsHelp, "reveal") {
		t.Errorf("secrets help should contain 'reveal', got:\n%s", secretsHelp)
	}

	m2 := newRootSizedModel()
	m2, _ = rootApplyMsg(m2, messages.Navigate{
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
	// "secrets" exercises the reveal branch; "ec2"/"s3"/"dbi" exercise the non-reveal branch.
	resourceTypes := []string{"ec2", "s3", "secrets", "dbi"}

	for _, rt := range resourceTypes {
		t.Run(rt, func(t *testing.T) {
			tui.Version = "0.6.0"
			m := newRootSizedModel()
			m, _ = rootApplyMsg(m, messages.Navigate{
				Target:       messages.TargetResourceList,
				ResourceType: rt,
			})
			m, _ = rootApplyMsg(m, rootKeyPress("?"))
			plain := strings.ToLower(stripANSI(rootViewContent(m)))

			for _, key := range []string{"detail", "copy", "sort", "refresh"} {
				if !strings.Contains(plain, key) {
					t.Errorf("HC-02: %s resource list help should contain %q", rt, key)
				}
			}

			// Secrets and SSM should show reveal (x key); all others should not.
			revealTypes := map[string]bool{"secrets": true, "ssm": true}
			if revealTypes[rt] {
				if !strings.Contains(plain, "reveal") {
					t.Errorf("HC-03: %s help should contain 'reveal'", rt)
				}
			} else {
				if strings.Contains(plain, "reveal") {
					t.Errorf("HC-04: %s help should NOT contain 'reveal'", rt)
				}
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// PAGINATED HELP CONTEXTS
//
// When a resource list has IsTruncated=true, help should use
// HelpFromResourceListPaginated (or HelpFromSecretsListPaginated for secrets)
// which includes the "M" / "load more" key binding.
// ═══════════════════════════════════════════════════════════════════════════

// TestQA_HelpContext_PaginatedResourceList_ShowsLoadMore verifies that the
// paginated help context includes "load more" and "M" key bindings.
func TestQA_HelpContext_PaginatedResourceList_ShowsLoadMore(t *testing.T) {
	tuitest.ForceColor(t)

	help := views.NewHelpWithResource(keys.Default(), views.HelpFromResourceListPaginated, "ec2")
	help.SetSize(120, 30)

	output := help.View()
	outputLower := strings.ToLower(output)

	if !strings.Contains(outputLower, "load more") {
		t.Errorf("paginated resource list help must contain 'load more', got:\n%s", output)
	}
	if !strings.Contains(output, "M") {
		t.Errorf("paginated resource list help must contain 'M' key, got:\n%s", output)
	}
}

// TestQA_HelpContext_PaginatedSecretsList_ShowsLoadMoreAndReveal verifies
// that the paginated secrets help context includes both "load more" and "reveal".
func TestQA_HelpContext_PaginatedSecretsList_ShowsLoadMoreAndReveal(t *testing.T) {
	tuitest.ForceColor(t)

	help := views.NewHelpWithResource(keys.Default(), views.HelpFromSecretsListPaginated, "ec2")
	help.SetSize(120, 30)

	output := help.View()
	outputLower := strings.ToLower(output)

	if !strings.Contains(outputLower, "load more") {
		t.Errorf("paginated secrets help must contain 'load more', got:\n%s", output)
	}
	if !strings.Contains(outputLower, "reveal") {
		t.Errorf("paginated secrets help must contain 'reveal' (x key), got:\n%s", output)
	}
}
