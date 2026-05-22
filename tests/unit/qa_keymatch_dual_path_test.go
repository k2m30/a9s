package unit_test

// qa_keymatch_dual_path_test.go — Issue #203: verify detail view and right column
// key handling works identically via tea.KeyPressMsg AND tea.KeyReleaseMsg paths.
//
// Background:
//   In BT v2, detail.Update() has two separate key-handling blocks:
//     case tea.KeyPressMsg:   (line ~357) — handles j, k, g, G, y, r, Tab, Enter, Esc, w
//     case tea.KeyMsg:        (line ~616) — interface implemented by BOTH KeyPressMsg
//                                           and KeyReleaseMsg; in the type switch it catches
//                                           KeyReleaseMsg (since KeyPressMsg already matched
//                                           the earlier case).
//
//   Issue #203: KeyReleaseMsg events (which match case tea.KeyMsg:) must trigger the
//   same navigation behaviour as KeyPressMsg events. All existing tests use KeyPressMsg
//   only, leaving the KeyReleaseMsg path untested.
//
// Test structure:
//   Each test runs two sub-tests:
//     "KeyPressMsg" — documents the currently-passing behaviour (case tea.KeyPressMsg:).
//     "KeyReleaseMsg" — asserts identical behaviour via case tea.KeyMsg: path.
//
//   KeyPressMsg sub-tests must PASS (existing behaviour).
//   KeyReleaseMsg sub-tests will FAIL until the coder adds the missing handling in
//   the case tea.KeyMsg: block for text keys (j, k, g, G, y, r, w) that are currently
//   only handled as msg.Text == "x" in the KeyPressMsg block.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers local to this file
// ---------------------------------------------------------------------------

// makeDetailDualPath creates a DetailModel for "ec2" with 5 plain fields.
// Width=80 keeps right column hidden; height=24.
func makeDetailDualPath(t *testing.T) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "i-dual-path-test",
		Name: "dual-path-instance",
		Fields: map[string]string{
			"InstanceId":   "i-dual-path-test",
			"State":        "running",
			"InstanceType": "t3.large",
			"VpcId":        "vpc-dual-path",
			"SubnetId":     "subnet-dual-path",
		},
	}
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: []config.DetailField{
					{Path: "InstanceId"}, {Path: "State"}, {Path: "InstanceType"}, {Path: "VpcId"}, {Path: "SubnetId"},
				},
			},
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", cfg, k)
	d.SetSize(80, 24)
	return d
}

// makeDetailDualPathWideEC2 creates a wide DetailModel with resourceType="ec2".
func makeDetailDualPathWideEC2(t *testing.T) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "i-dual-path-wide-ec2",
		Name: "dual-path-wide-ec2",
		Fields: map[string]string{
			"InstanceId": "i-dual-path-wide-ec2",
			"State":      "running",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(140, 30)
	return d
}

// makeDetailWithNavFieldDual creates a detail model with VpcId registered as navigable.
func makeDetailWithNavFieldDual(t *testing.T) views.DetailModel {
	t.Helper()
	resource.SetNavigableFieldsForTest("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})
	t.Cleanup(func() { resource.CleanupNavigableFieldsForTest("ec2") })

	res := resource.Resource{
		ID:   "i-navfield-dual",
		Name: "navfield-dual-instance",
		Fields: map[string]string{
			"VpcId": "vpc-navfield-dual",
			"State": "running",
		},
	}
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {Detail: []config.DetailField{{Path: "VpcId"}, {Path: "State"}}},
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", cfg, k)
	d.SetSize(80, 24)
	return d
}

// pressKeyRelease sends a tea.KeyReleaseMsg for a text key (e.g. "j", "k", "g").
func pressKeyRelease(text string) tea.KeyReleaseMsg {
	return tea.KeyReleaseMsg{Code: -1, Text: text}
}

// pressSpecialKeyRelease sends a tea.KeyReleaseMsg for a special key (e.g. tea.KeyTab).
func pressSpecialKeyRelease(code rune) tea.KeyReleaseMsg {
	return tea.KeyReleaseMsg{Code: code}
}

// makeExplicitlyVisibleDualPath transitions right column from auto-shown to explicitly-visible.
// Uses KeyPressMsg for setup so as not to affect the path under test.
func makeExplicitlyVisibleDualPath(d views.DetailModel) views.DetailModel {
	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "r"})
	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "r"})
	return d
}

// ---------------------------------------------------------------------------
// 1. j / Down — cursor moves down (left column, right col not focused)
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_JMovesDown_BothPaths verifies that pressing j moves
// the field cursor down by 1 regardless of KeyPressMsg vs KeyReleaseMsg.
func TestQA_DetailKeyMatch_JMovesDown_BothPaths(t *testing.T) {
	t.Run("KeyPressMsg", func(t *testing.T) {
		d := makeDetailDualPath(t)
		initialCursor := d.FieldCursor()

		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})

		if d.FieldCursor() != initialCursor+1 {
			t.Errorf("KeyPressMsg j: cursor must advance from %d to %d, got %d",
				initialCursor, initialCursor+1, d.FieldCursor())
		}
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		d := makeDetailDualPath(t)
		initialCursor := d.FieldCursor()

		d, _ = d.Update(pressKeyRelease("j"))

		if d.FieldCursor() != initialCursor+1 {
			t.Errorf("KeyReleaseMsg j: cursor must advance from %d to %d, got %d",
				initialCursor, initialCursor+1, d.FieldCursor())
		}
	})
}

// TestQA_DetailKeyMatch_DownArrowMovesDown_BothPaths verifies the Down arrow key.
func TestQA_DetailKeyMatch_DownArrowMovesDown_BothPaths(t *testing.T) {
	t.Run("KeyPressMsg", func(t *testing.T) {
		d := makeDetailDualPath(t)
		initialCursor := d.FieldCursor()

		d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyDown})

		if d.FieldCursor() != initialCursor+1 {
			t.Errorf("KeyPressMsg Down arrow: cursor must advance from %d to %d, got %d",
				initialCursor, initialCursor+1, d.FieldCursor())
		}
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		d := makeDetailDualPath(t)
		initialCursor := d.FieldCursor()

		d, _ = d.Update(pressSpecialKeyRelease(tea.KeyDown))

		if d.FieldCursor() != initialCursor+1 {
			t.Errorf("KeyReleaseMsg Down arrow: cursor must advance from %d to %d, got %d",
				initialCursor, initialCursor+1, d.FieldCursor())
		}
	})
}

// ---------------------------------------------------------------------------
// 2. k / Up — cursor moves up
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_KMovesUp_BothPaths verifies k moves the cursor up by 1.
func TestQA_DetailKeyMatch_KMovesUp_BothPaths(t *testing.T) {
	// Navigate to index 2 first using KeyPressMsg (setup only).
	setupAtIndex2 := func(t *testing.T) views.DetailModel {
		t.Helper()
		d := makeDetailDualPath(t)
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		if d.FieldCursor() != 2 {
			t.Fatalf("setup: expected cursor at 2, got %d", d.FieldCursor())
		}
		return d
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		d := setupAtIndex2(t)
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "k"})
		if d.FieldCursor() != 1 {
			t.Errorf("KeyPressMsg k: cursor must move from 2 to 1, got %d", d.FieldCursor())
		}
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		d := setupAtIndex2(t)
		d, _ = d.Update(pressKeyRelease("k"))
		if d.FieldCursor() != 1 {
			t.Errorf("KeyReleaseMsg k: cursor must move from 2 to 1, got %d", d.FieldCursor())
		}
	})
}

// ---------------------------------------------------------------------------
// 3. g — cursor moves to top (index 0)
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_GMovesToTop_BothPaths verifies g jumps to index 0.
func TestQA_DetailKeyMatch_GMovesToTop_BothPaths(t *testing.T) {
	setupAtIndex3 := func(t *testing.T) views.DetailModel {
		t.Helper()
		d := makeDetailDualPath(t)
		for range 3 {
			d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		}
		if d.FieldCursor() != 3 {
			t.Fatalf("setup: expected cursor at 3, got %d", d.FieldCursor())
		}
		return d
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		d := setupAtIndex3(t)
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "g"})
		if d.FieldCursor() != 0 {
			t.Errorf("KeyPressMsg g: cursor must jump to 0, got %d", d.FieldCursor())
		}
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		d := setupAtIndex3(t)
		d, _ = d.Update(pressKeyRelease("g"))
		if d.FieldCursor() != 0 {
			t.Errorf("KeyReleaseMsg g: cursor must jump to 0, got %d", d.FieldCursor())
		}
	})
}

// ---------------------------------------------------------------------------
// 4. G — cursor moves to bottom (last index)
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_GShiftMovesToBottom_BothPaths verifies G jumps to last index.
func TestQA_DetailKeyMatch_GShiftMovesToBottom_BothPaths(t *testing.T) {
	// With 5 fields (indices 0-4), G must land on index 4.
	const expectedLast = 4

	t.Run("KeyPressMsg", func(t *testing.T) {
		d := makeDetailDualPath(t)
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "G"})
		if d.FieldCursor() != expectedLast {
			t.Errorf("KeyPressMsg G: cursor must jump to %d (last), got %d", expectedLast, d.FieldCursor())
		}
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		d := makeDetailDualPath(t)
		d, _ = d.Update(pressKeyRelease("G"))
		if d.FieldCursor() != expectedLast {
			t.Errorf("KeyReleaseMsg G: cursor must jump to %d (last), got %d", expectedLast, d.FieldCursor())
		}
	})
}

// ---------------------------------------------------------------------------
// 5. y — emits NavigateMsg to YAML
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_YEmitsYAMLNavigate_BothPaths verifies y emits NavigateMsg{TargetYAML}.
func TestQA_DetailKeyMatch_YEmitsYAMLNavigate_BothPaths(t *testing.T) {
	assertYAMLCmd := func(t *testing.T, cmd tea.Cmd, path string) {
		t.Helper()
		if cmd == nil {
			t.Errorf("%s y: must return non-nil cmd", path)
			return
		}
		msg := cmd()
		nav, ok := msg.(messages.Navigate)
		if !ok {
			t.Errorf("%s y: cmd() must return NavigateMsg, got %T", path, msg)
			return
		}
		if nav.Target != messages.TargetYAML {
			t.Errorf("%s y: NavigateMsg.Target must be TargetYAML, got %v", path, nav.Target)
		}
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		d := makeDetailDualPath(t)
		_, cmd := d.Update(tea.KeyPressMsg{Code: -1, Text: "y"})
		assertYAMLCmd(t, cmd, "KeyPressMsg")
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		d := makeDetailDualPath(t)
		_, cmd := d.Update(pressKeyRelease("y"))
		assertYAMLCmd(t, cmd, "KeyReleaseMsg")
	})
}

// ---------------------------------------------------------------------------
// 6. r — toggles right column visibility
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_RTogglesRightCol_BothPaths verifies r toggles right
// column visibility at wide terminal width with related defs registered.
func TestQA_DetailKeyMatch_RTogglesRightCol_BothPaths(t *testing.T) {
	checkRToggle := func(t *testing.T, rMsg tea.Msg, path string) {
		t.Helper()
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		})

		d := makeDetailDualPathWideEC2(t)

		if !strings.Contains(d.View(), "RELATED") {
			t.Skip("right column not auto-shown at width=140; skipping r-toggle test")
		}

		viewBefore := d.View()

		// First r: hides the auto-shown column.
		d, _ = d.Update(rMsg)
		viewAfterFirstR := d.View()

		if viewBefore == viewAfterFirstR {
			t.Errorf("%s r: first r press should toggle right column visibility; view unchanged", path)
		}
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		checkRToggle(t, tea.KeyPressMsg{Code: -1, Text: "r"}, "KeyPressMsg")
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		checkRToggle(t, pressKeyRelease("r"), "KeyReleaseMsg")
	})
}

// ---------------------------------------------------------------------------
// 7. Enter on navigable field — emits RelatedNavigateMsg
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_EnterOnNavigable_BothPaths verifies Enter on a navigable
// field emits RelatedNavigateMsg with correct TargetType and TargetID.
func TestQA_DetailKeyMatch_EnterOnNavigable_BothPaths(t *testing.T) {
	assertEnterNav := func(t *testing.T, enterMsg tea.Msg, path string) {
		t.Helper()
		d := makeDetailWithNavFieldDual(t)

		// VpcId is at index 0 (registered navigable → "vpc").
		if d.FieldCursor() != 0 {
			t.Fatalf("setup: expected cursor at 0 (VpcId), got %d", d.FieldCursor())
		}

		_, cmd := d.Update(enterMsg)

		if cmd == nil {
			t.Errorf("%s Enter on navigable field: must return non-nil cmd", path)
			return
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigate)
		if !ok {
			t.Errorf("%s Enter: cmd() must return RelatedNavigateMsg, got %T", path, msg)
			return
		}
		if nav.TargetType != "vpc" {
			t.Errorf("%s Enter: RelatedNavigateMsg.TargetType must be %q, got %q", path, "vpc", nav.TargetType)
		}
		if nav.TargetID != "vpc-navfield-dual" {
			t.Errorf("%s Enter: RelatedNavigateMsg.TargetID must be %q, got %q",
				path, "vpc-navfield-dual", nav.TargetID)
		}
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		assertEnterNav(t, tea.KeyPressMsg{Code: tea.KeyEnter}, "KeyPressMsg")
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		assertEnterNav(t, pressSpecialKeyRelease(tea.KeyEnter), "KeyReleaseMsg")
	})
}

// ---------------------------------------------------------------------------
// 8. Esc — clears search if active; does NOT emit PopViewMsg
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_EscClearsSearch_BothPaths verifies Esc does not
// emit PopViewMsg regardless of key message type.
func TestQA_DetailKeyMatch_EscClearsSearch_BothPaths(t *testing.T) {
	// Activate search using KeyPressMsg (setup only).
	activateSearch := func(d views.DetailModel) views.DetailModel {
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
		return d
	}

	assertEscNoPopView := func(t *testing.T, escMsg tea.Msg, path string) {
		t.Helper()
		d := makeDetailDualPath(t)
		d = activateSearch(d)

		_, cmd := d.Update(escMsg)

		// Must NOT emit PopViewMsg.
		if cmd != nil {
			produced := cmd()
			if _, isPopView := produced.(messages.PopView); isPopView {
				t.Errorf("%s Esc: must NOT emit PopViewMsg; got PopViewMsg", path)
			}
		}
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		assertEscNoPopView(t, tea.KeyPressMsg{Code: tea.KeyEscape}, "KeyPressMsg")
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		assertEscNoPopView(t, pressSpecialKeyRelease(tea.KeyEscape), "KeyReleaseMsg")
	})
}

// ---------------------------------------------------------------------------
// 9. Tab — toggles focus to right column when right col is visible
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_TabFocusesRightCol_BothPaths verifies Tab focuses the
// right column (view changes) when right column is explicitly visible.
func TestQA_DetailKeyMatch_TabFocusesRightCol_BothPaths(t *testing.T) {
	checkTabFocus := func(t *testing.T, tabMsg tea.Msg, path string) {
		t.Helper()
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		})

		d := makeDetailDualPathWideEC2(t)

		if !strings.Contains(d.View(), "RELATED") {
			t.Skip("right column not auto-shown at width=140; skipping Tab focus test")
		}

		// Transition to explicitly visible using KeyPressMsg (setup only).
		d = makeExplicitlyVisible(d)
		viewBeforeTab := d.View()

		d, _ = d.Update(tabMsg)
		viewAfterTab := d.View()

		if viewBeforeTab == viewAfterTab {
			t.Errorf("%s Tab: should change View() output (focus highlight); views identical before/after", path)
		}
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		checkTabFocus(t, tea.KeyPressMsg{Code: tea.KeyTab}, "KeyPressMsg")
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		checkTabFocus(t, pressSpecialKeyRelease(tea.KeyTab), "KeyReleaseMsg")
	})
}

// ---------------------------------------------------------------------------
// 10. w — toggles wrap mode
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_WTogglesWrap_BothPaths verifies w toggles wrap mode
// without panicking, on both message paths.
func TestQA_DetailKeyMatch_WTogglesWrap_BothPaths(t *testing.T) {
	// Wrap toggling may not produce visible differences for short content,
	// so we just verify no panic and that two w presses are symmetric.
	checkWrapTwoPresses := func(t *testing.T, wMsg tea.Msg, path string) {
		t.Helper()
		d := makeDetailDualPath(t)
		viewBefore := d.View()

		d, _ = d.Update(wMsg)
		viewAfterFirst := d.View()

		d, _ = d.Update(wMsg)
		viewAfterSecond := d.View()

		// Two presses must restore to original state (wrap is a boolean toggle).
		if viewBefore != viewAfterSecond {
			t.Errorf("%s w+w: two w presses must restore original view; before len=%d after len=%d",
				path, len(viewBefore), len(viewAfterSecond))
		}
		// Suppress unused variable warnings.
		_ = viewAfterFirst
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		checkWrapTwoPresses(t, tea.KeyPressMsg{Code: -1, Text: "w"}, "KeyPressMsg")
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		checkWrapTwoPresses(t, pressKeyRelease("w"), "KeyReleaseMsg")
	})
}

// ---------------------------------------------------------------------------
// 11. Tab (right col focused) — unfocuses right column
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_TabUnfocusesRightCol_BothPaths verifies Tab while right
// column is focused removes focus (view changes back).
func TestQA_DetailKeyMatch_TabUnfocusesRightCol_BothPaths(t *testing.T) {
	checkTabUnfocus := func(t *testing.T, tabMsg tea.Msg, path string) {
		t.Helper()
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		})

		d := makeDetailDualPathWideEC2(t)

		if !strings.Contains(d.View(), "RELATED") {
			t.Skip("right column not auto-shown; skipping Tab-unfocus test")
		}

		// Reach focused state using KeyPressMsg (setup only).
		d = makeExplicitlyVisible(d)
		d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		viewFocused := d.View()

		// Now press Tab via the path under test — should unfocus.
		d, _ = d.Update(tabMsg)
		viewUnfocused := d.View()

		if viewFocused == viewUnfocused {
			t.Errorf("%s Tab (unfocus): view must change when Tab removes right-col focus; views identical", path)
		}
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		checkTabUnfocus(t, tea.KeyPressMsg{Code: tea.KeyTab}, "KeyPressMsg")
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		checkTabUnfocus(t, pressSpecialKeyRelease(tea.KeyTab), "KeyReleaseMsg")
	})
}

// ---------------------------------------------------------------------------
// 12. Esc (right col focused) — unfocuses right column; does NOT emit PopViewMsg
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_EscUnfocusesRightCol_BothPaths verifies Esc while right
// column is focused removes focus and does NOT emit PopViewMsg.
func TestQA_DetailKeyMatch_EscUnfocusesRightCol_BothPaths(t *testing.T) {
	checkEscUnfocus := func(t *testing.T, escMsg tea.Msg, path string) {
		t.Helper()
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		})

		d := makeDetailDualPathWideEC2(t)

		if !strings.Contains(d.View(), "RELATED") {
			t.Skip("right column not auto-shown; skipping Esc-unfocus test")
		}

		// Reach focused state using KeyPressMsg.
		d = makeExplicitlyVisible(d)
		d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		viewFocused := d.View()

		// Esc via the path under test.
		d, cmd := d.Update(escMsg)
		viewAfterEsc := d.View()

		// View must change (focus removed).
		if viewFocused == viewAfterEsc {
			t.Errorf("%s Esc (unfocus): view must change when Esc removes right-col focus; views identical", path)
		}

		// Must NOT emit PopViewMsg.
		if cmd != nil {
			produced := cmd()
			if _, isPopView := produced.(messages.PopView); isPopView {
				t.Errorf("%s Esc (unfocus): must NOT emit PopViewMsg; got PopViewMsg", path)
			}
		}
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		checkEscUnfocus(t, tea.KeyPressMsg{Code: tea.KeyEscape}, "KeyPressMsg")
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		checkEscUnfocus(t, pressSpecialKeyRelease(tea.KeyEscape), "KeyReleaseMsg")
	})
}

// ---------------------------------------------------------------------------
// 13. j / Down (right col focused) — cursor moves within right column
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_RightColFocused_JMovesDown_BothPaths verifies j moves
// the right column cursor when the right column is focused (view highlight changes).
func TestQA_DetailKeyMatch_RightColFocused_JMovesDown_BothPaths(t *testing.T) {
	setupFocusedRightColTwoActionableRows := func(t *testing.T) views.DetailModel {
		t.Helper()
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
			{TargetType: "vpc", DisplayName: "VPCs", Checker: noopChecker},
		})
		d := makeDetailDualPathWideEC2(t)
		if !strings.Contains(d.View(), "RELATED") {
			t.Skip("right column not auto-shown; skipping right-col j test")
		}
		d = makeExplicitlyVisibleDualPath(d)
		d, _ = d.Update(messages.RelatedCheckResult{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  "tg",
				Count:       3,
				ResourceIDs: []string{"tg-aaa"},
			},
		})
		d, _ = d.Update(messages.RelatedCheckResult{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  "vpc",
				Count:       1,
				ResourceIDs: []string{"vpc-bbb"},
			},
		})
		d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		return d
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
			{TargetType: "vpc", DisplayName: "VPCs", Checker: noopChecker},
		})

		d := setupFocusedRightColTwoActionableRows(t)
		viewBefore := d.View()

		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		viewAfterJ := d.View()

		if viewBefore == viewAfterJ {
			t.Errorf("KeyPressMsg j (right col focused): cursor must move highlight; view unchanged")
		}
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
			{TargetType: "vpc", DisplayName: "VPCs", Checker: noopChecker},
		})

		d := setupFocusedRightColTwoActionableRows(t)
		viewBefore := d.View()

		d, _ = d.Update(pressKeyRelease("j"))
		viewAfterJ := d.View()

		if viewBefore == viewAfterJ {
			t.Errorf("KeyReleaseMsg j (right col focused): cursor must move highlight; view unchanged")
		}
	})
}

// ---------------------------------------------------------------------------
// 14. k / Up (right col focused) — cursor moves within right column
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_RightColFocused_KMovesUp_BothPaths verifies k moves
// the right column cursor up when the column is focused.
func TestQA_DetailKeyMatch_RightColFocused_KMovesUp_BothPaths(t *testing.T) {
	setupFocusedRightColAtSecondRow := func(t *testing.T) views.DetailModel {
		t.Helper()
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
			{TargetType: "vpc", DisplayName: "VPCs", Checker: noopChecker},
		})
		d := makeDetailDualPathWideEC2(t)
		if !strings.Contains(d.View(), "RELATED") {
			t.Skip("right column not auto-shown; skipping right-col k test")
		}
		d = makeExplicitlyVisibleDualPath(d)
		d, _ = d.Update(messages.RelatedCheckResult{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  "tg",
				Count:       3,
				ResourceIDs: []string{"tg-aaa"},
			},
		})
		d, _ = d.Update(messages.RelatedCheckResult{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  "vpc",
				Count:       1,
				ResourceIDs: []string{"vpc-bbb"},
			},
		})
		// Focus and move to second row.
		d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		return d
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
			{TargetType: "vpc", DisplayName: "VPCs", Checker: noopChecker},
		})

		d := setupFocusedRightColAtSecondRow(t)
		viewAtSecond := d.View()

		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "k"})
		viewAfterK := d.View()

		if viewAtSecond == viewAfterK {
			t.Errorf("KeyPressMsg k (right col focused): cursor must move highlight up; view unchanged")
		}
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
			{TargetType: "vpc", DisplayName: "VPCs", Checker: noopChecker},
		})

		d := setupFocusedRightColAtSecondRow(t)
		viewAtSecond := d.View()

		d, _ = d.Update(pressKeyRelease("k"))
		viewAfterK := d.View()

		if viewAtSecond == viewAfterK {
			t.Errorf("KeyReleaseMsg k (right col focused): cursor must move highlight up; view unchanged")
		}
	})
}

// ---------------------------------------------------------------------------
// 15. / (right col focused) — activates filter in right column
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_RightColFocused_SlashActivatesFilter_BothPaths verifies
// that pressing / while the right column is focused activates filter mode.
// Filter activation is confirmed by sending a follow-up character that narrows
// results to "No matches".
func TestQA_DetailKeyMatch_RightColFocused_SlashActivatesFilter_BothPaths(t *testing.T) {
	setupFocusedRightColWithResults := func(t *testing.T) views.DetailModel {
		t.Helper()
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		})
		d := makeDetailDualPathWideEC2(t)
		if !strings.Contains(d.View(), "RELATED") {
			t.Skip("right column not auto-shown; skipping right-col / filter test")
		}
		d = makeExplicitlyVisibleDualPath(d)
		d, _ = d.Update(messages.RelatedCheckResult{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  "tg",
				Count:       2,
				ResourceIDs: []string{"tg-aaa", "tg-bbb"},
			},
		})
		d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		return d
	}

	checkSlashFilter := func(t *testing.T, slashMsg tea.Msg, path string) {
		t.Helper()
		d := setupFocusedRightColWithResults(t)

		// Press / via the path under test.
		d, _ = d.Update(slashMsg)

		// Type "x" via KeyPressMsg — if filter was activated, this enters the query.
		// "x" matches nothing in "Target Groups".
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "x"})
		viewAfterX := d.View()

		// If filter is active, "No matches" must appear.
		if !strings.Contains(viewAfterX, "No matches") {
			t.Errorf("%s /: after activating filter and typing 'x', expected 'No matches' in view; got:\n%s",
				path, viewAfterX)
		}
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		})
		checkSlashFilter(t, tea.KeyPressMsg{Code: -1, Text: "/"}, "KeyPressMsg")
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		})
		checkSlashFilter(t, pressKeyRelease("/"), "KeyReleaseMsg")
	})
}

// ---------------------------------------------------------------------------
// 16. Enter (right col focused) — emits RelatedNavigateMsg for selected row
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_RightColFocused_EnterEmitsNavigate_BothPaths verifies
// Enter when right column is focused emits RelatedNavigateMsg.
func TestQA_DetailKeyMatch_RightColFocused_EnterEmitsNavigate_BothPaths(t *testing.T) {
	setupFocusedRightColActionable := func(t *testing.T) views.DetailModel {
		t.Helper()
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		})
		d := makeDetailDualPathWideEC2(t)
		if !strings.Contains(d.View(), "RELATED") {
			t.Skip("right column not auto-shown; skipping right-col Enter test")
		}
		d = makeExplicitlyVisibleDualPath(d)
		d, _ = d.Update(messages.RelatedCheckResult{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  "tg",
				Count:       2,
				ResourceIDs: []string{"tg-ccc", "tg-ddd"},
			},
		})
		d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		return d
	}

	assertEnterOnRightCol := func(t *testing.T, enterMsg tea.Msg, path string) {
		t.Helper()
		d := setupFocusedRightColActionable(t)

		_, cmd := d.Update(enterMsg)

		if cmd == nil {
			t.Errorf("%s Enter (right col focused): must return non-nil cmd for actionable row", path)
			return
		}
		msg := cmd()
		nav, ok := msg.(messages.RelatedNavigate)
		if !ok {
			t.Errorf("%s Enter (right col focused): cmd() must return RelatedNavigateMsg, got %T", path, msg)
			return
		}
		if nav.TargetType != "tg" {
			t.Errorf("%s Enter (right col focused): RelatedNavigateMsg.TargetType must be %q, got %q",
				path, "tg", nav.TargetType)
		}
	}

	t.Run("KeyPressMsg", func(t *testing.T) {
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		})
		assertEnterOnRightCol(t, tea.KeyPressMsg{Code: tea.KeyEnter}, "KeyPressMsg")
	})

	t.Run("KeyReleaseMsg", func(t *testing.T) {
		replaceEC2Related(t, []resource.RelatedDef{
			{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		})
		assertEnterOnRightCol(t, pressSpecialKeyRelease(tea.KeyEnter), "KeyReleaseMsg")
	})
}

// ---------------------------------------------------------------------------
// 17. Right column standalone updateKeyMsg — j / k / /
// ---------------------------------------------------------------------------
// These tests exercise the rightColumnModel.updateKeyMsg path directly via
// the DetailModel delegation in case tea.KeyMsg:.

// TestQA_RightCol_UpdateKeyMsg_JMovesDown verifies j via KeyReleaseMsg moves
// the right column cursor when focused.
func TestQA_RightCol_UpdateKeyMsg_JMovesDown(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "aaa", DisplayName: "AAA Resources", Checker: noopChecker},
		{TargetType: "bbb", DisplayName: "BBB Resources", Checker: noopChecker},
	})

	d := makeDetailDualPathWideEC2(t)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown; skipping rightcol updateKeyMsg j test")
	}
	d = makeExplicitlyVisibleDualPath(d)
	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "aaa",
			Count:       1,
			ResourceIDs: []string{"aaa-001"},
		},
	})
	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "bbb",
			Count:       1,
			ResourceIDs: []string{"bbb-001"},
		},
	})

	// Focus right col.
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	viewAtFirst := d.View()

	// Press j via KeyReleaseMsg (exercises case tea.KeyMsg: path).
	d, _ = d.Update(pressKeyRelease("j"))
	viewAfterJ := d.View()

	if viewAtFirst == viewAfterJ {
		t.Errorf("rightcol updateKeyMsg j: must move cursor highlight; view unchanged")
	}
}

// TestQA_RightCol_UpdateKeyMsg_KMovesUp verifies k via KeyReleaseMsg in the right column.
func TestQA_RightCol_UpdateKeyMsg_KMovesUp(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "aaa", DisplayName: "AAA Resources", Checker: noopChecker},
		{TargetType: "bbb", DisplayName: "BBB Resources", Checker: noopChecker},
	})

	d := makeDetailDualPathWideEC2(t)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown; skipping rightcol updateKeyMsg k test")
	}
	d = makeExplicitlyVisibleDualPath(d)
	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "aaa",
			Count:       1,
			ResourceIDs: []string{"aaa-001"},
		},
	})
	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "bbb",
			Count:       1,
			ResourceIDs: []string{"bbb-001"},
		},
	})

	// Focus and move to second row via KeyPressMsg.
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	viewAtSecond := d.View()

	// Press k via KeyReleaseMsg.
	d, _ = d.Update(pressKeyRelease("k"))
	viewAfterK := d.View()

	if viewAtSecond == viewAfterK {
		t.Errorf("rightcol updateKeyMsg k: must move cursor highlight up; view unchanged")
	}
}

// TestQA_RightCol_UpdateKeyMsg_SlashActivatesFilter verifies / via KeyReleaseMsg
// activates the filter in the right column.
func TestQA_RightCol_UpdateKeyMsg_SlashActivatesFilter(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "aaa", DisplayName: "AAA Resources", Checker: noopChecker},
	})

	d := makeDetailDualPathWideEC2(t)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown; skipping rightcol updateKeyMsg / test")
	}
	d = makeExplicitlyVisibleDualPath(d)
	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "aaa",
			Count:       2,
			ResourceIDs: []string{"aaa-001", "aaa-002"},
		},
	})

	// Focus right col.
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})

	// Activate filter via KeyReleaseMsg /.
	d, _ = d.Update(pressKeyRelease("/"))

	// Type "x" via KeyPressMsg — if filter was activated, this enters the query.
	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "x"})
	viewAfterX := d.View()

	// If filter active, "No matches" must appear (since "aaa" doesn't contain "x").
	if !strings.Contains(viewAfterX, "No matches") {
		t.Errorf("rightcol updateKeyMsg /: after typing 'x', expected 'No matches' in view; got:\n%s", viewAfterX)
	}
}

// ---------------------------------------------------------------------------
// Strict parity assertions: KeyPressMsg and KeyReleaseMsg must produce
// IDENTICAL cursor positions and View() output for the same logical key.
// ---------------------------------------------------------------------------

// TestQA_DetailKeyMatch_JBehaviorIsIdentical_KeyPressVsKeyRelease verifies
// that j via KeyPressMsg and KeyReleaseMsg produce identical View() output.
func TestQA_DetailKeyMatch_JBehaviorIsIdentical_KeyPressVsKeyRelease(t *testing.T) {
	dPress := makeDetailDualPath(t)
	dRelease := makeDetailDualPath(t)

	if dPress.FieldCursor() != 0 || dRelease.FieldCursor() != 0 {
		t.Fatalf("setup: both models must start at cursor 0; got press=%d release=%d",
			dPress.FieldCursor(), dRelease.FieldCursor())
	}

	dPress, _ = dPress.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	dRelease, _ = dRelease.Update(pressKeyRelease("j"))

	if dPress.FieldCursor() != dRelease.FieldCursor() {
		t.Errorf("j parity: KeyPressMsg cursor=%d, KeyReleaseMsg cursor=%d — must be identical",
			dPress.FieldCursor(), dRelease.FieldCursor())
	}
	if dPress.View() != dRelease.View() {
		t.Errorf("j parity: View() differs between KeyPressMsg and KeyReleaseMsg paths:\nKeyPressMsg:\n%s\nKeyReleaseMsg:\n%s",
			dPress.View(), dRelease.View())
	}
}

// TestQA_DetailKeyMatch_GBehaviorIsIdentical_KeyPressVsKeyRelease verifies g parity.
func TestQA_DetailKeyMatch_GBehaviorIsIdentical_KeyPressVsKeyRelease(t *testing.T) {
	setup := func(t *testing.T) views.DetailModel {
		t.Helper()
		d := makeDetailDualPath(t)
		for range 4 {
			d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		}
		return d
	}

	dPress := setup(t)
	dRelease := setup(t)

	dPress, _ = dPress.Update(tea.KeyPressMsg{Code: -1, Text: "g"})
	dRelease, _ = dRelease.Update(pressKeyRelease("g"))

	if dPress.FieldCursor() != dRelease.FieldCursor() {
		t.Errorf("g parity: KeyPressMsg cursor=%d, KeyReleaseMsg cursor=%d — must be identical",
			dPress.FieldCursor(), dRelease.FieldCursor())
	}
	if dPress.View() != dRelease.View() {
		t.Errorf("g parity: View() differs between KeyPressMsg and KeyReleaseMsg paths")
	}
}

// TestQA_DetailKeyMatch_GShiftBehaviorIsIdentical_KeyPressVsKeyRelease verifies G parity.
func TestQA_DetailKeyMatch_GShiftBehaviorIsIdentical_KeyPressVsKeyRelease(t *testing.T) {
	dPress := makeDetailDualPath(t)
	dRelease := makeDetailDualPath(t)

	dPress, _ = dPress.Update(tea.KeyPressMsg{Code: -1, Text: "G"})
	dRelease, _ = dRelease.Update(pressKeyRelease("G"))

	if dPress.FieldCursor() != dRelease.FieldCursor() {
		t.Errorf("G parity: KeyPressMsg cursor=%d, KeyReleaseMsg cursor=%d — must be identical",
			dPress.FieldCursor(), dRelease.FieldCursor())
	}
	if dPress.View() != dRelease.View() {
		t.Errorf("G parity: View() differs between KeyPressMsg and KeyReleaseMsg paths")
	}
}

// TestQA_DetailKeyMatch_KBehaviorIsIdentical_KeyPressVsKeyRelease verifies k parity.
func TestQA_DetailKeyMatch_KBehaviorIsIdentical_KeyPressVsKeyRelease(t *testing.T) {
	setup := func(t *testing.T) views.DetailModel {
		t.Helper()
		d := makeDetailDualPath(t)
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		return d
	}

	dPress := setup(t)
	dRelease := setup(t)

	dPress, _ = dPress.Update(tea.KeyPressMsg{Code: -1, Text: "k"})
	dRelease, _ = dRelease.Update(pressKeyRelease("k"))

	if dPress.FieldCursor() != dRelease.FieldCursor() {
		t.Errorf("k parity: KeyPressMsg cursor=%d, KeyReleaseMsg cursor=%d — must be identical",
			dPress.FieldCursor(), dRelease.FieldCursor())
	}
	if dPress.View() != dRelease.View() {
		t.Errorf("k parity: View() differs between KeyPressMsg and KeyReleaseMsg paths")
	}
}
