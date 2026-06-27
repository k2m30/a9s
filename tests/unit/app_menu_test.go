// app_menu_test.go — PR-C slice 1a: controller-side MENU machinery.
//
// Covers the four behavioral areas introduced by PR-C:
//
//  2. Menu intents → MenuState: ApplyIntents with PatchMenuAvailability,
//     PatchMenu, PatchMenuIssueBatch, PatchMenuCheckProgress,
//     PatchMenuEnrichProgress, and MenuClearAvailabilityIntent apply their
//     payloads to MenuState, visible via Snapshot().Body.Menu.
//
//  3. Menu actions: ActionMoveUp/Down/Top/Bottom/PageUp/PageDown move
//     MenuBody.Selected; ActionToggleAttention flips AttentionOnly;
//     ActionSetFilter{Arg:"ec"} narrows visible entries; ActionSelect
//     navigates to a resource list (BodyKindList) or is blocked for
//     confirmed-empty types.
//
//  4. Snapshot MenuBody visibility parity: MenuBody.Entries reflect the
//     same visibility / badge logic as mainmenu.go's applyFilter +
//     isVisibleUnderIssueFilter — that file is the oracle.
//
// Oracle references:
//   - internal/tui/views/mainmenu.go: applyFilter, isVisibleUnderIssueFilter,
//     skipUnavailable, issueBadge, FrameTitle.
//
// All scenarios are hermetic (no AWS clients, no real config on disk).
// Tests use resource.AllResourceTypes() to compute expectations so they
// stay correct when new resource types are registered.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// newMenuController returns a Controller that starts with ScreenMenu as the
// root (PR-C contract). Profile/region are set to recognisable fake values.
func newMenuController(t *testing.T) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "eu-west-1"
	core := runtime.New(s, nil)
	return app.New(core)
}

// requireMenuBody extracts the MenuBody from the current Snapshot, failing
// the test immediately if the body kind is not BodyKindMenu or Menu is nil.
func requireMenuBody(t *testing.T, c *app.Controller) *app.MenuBody {
	t.Helper()
	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindMenu {
		t.Fatalf("expected BodyKindMenu, got %q — controller not on menu screen", vs.Body.Kind)
	}
	if vs.Body.Menu == nil {
		t.Fatal("Body.Menu is nil even though BodyKind == BodyKindMenu")
	}
	return vs.Body.Menu
}

// =============================================================================
// 2. Menu intents → MenuState
// =============================================================================

// TestMenuIntent_PatchMenuAvailability_SetsCountOnEntry verifies that
// ApplyIntents(PatchMenuAvailability{ResourceType:"ec2", Count:5}) causes the
// ec2 entry in Snapshot().Body.Menu to show Availability == 5.
func TestMenuIntent_PatchMenuAvailability_SetsCountOnEntry(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 5, Truncated: false},
	})

	menu := requireMenuBody(t, c)

	var found bool
	for _, e := range menu.Entries {
		if e.ShortName == "ec2" {
			found = true
			if e.Availability != 5 {
				t.Errorf("ec2 entry Availability: got %d want 5", e.Availability)
			}
		}
	}
	if !found {
		t.Error("ec2 entry not present in MenuBody.Entries after PatchMenuAvailability")
	}
}

// TestMenuIntent_PatchMenuAvailability_TruncatedCount verifies that
// Availability is set when Count is a truncated lower bound (Truncated=true).
func TestMenuIntent_PatchMenuAvailability_TruncatedCount(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "rds", Count: 3, Truncated: true},
	})

	menu := requireMenuBody(t, c)
	for _, e := range menu.Entries {
		if e.ShortName == "rds" {
			if e.Availability != 3 {
				t.Errorf("rds entry Availability: got %d want 3", e.Availability)
			}
			return
		}
	}
	t.Error("rds entry not present in MenuBody.Entries after PatchMenuAvailability")
}

// TestMenuIntent_PatchMenuAvailability_ZeroCount_ConfirmedEmpty verifies that
// a confirmed-empty type (Count=0, Truncated=false) shows Availability == 0.
func TestMenuIntent_PatchMenuAvailability_ZeroCount_ConfirmedEmpty(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "lambda", Count: 0, Truncated: false},
	})

	menu := requireMenuBody(t, c)
	for _, e := range menu.Entries {
		if e.ShortName == "lambda" {
			if e.Availability != 0 {
				t.Errorf("lambda entry Availability: got %d want 0 (confirmed empty)", e.Availability)
			}
			return
		}
	}
	t.Error("lambda entry not present in MenuBody.Entries after PatchMenuAvailability(0)")
}

// TestMenuIntent_PatchMenu_SetsIssueBadgeOnEntry verifies that
// ApplyIntents(PatchMenu{ResourceType:"s3", Issues:7}) causes the s3 entry
// to carry an IssueBadge with Count==7.
func TestMenuIntent_PatchMenu_SetsIssueBadgeOnEntry(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenu{ResourceType: "s3", Issues: 7, Truncated: false},
	})

	menu := requireMenuBody(t, c)
	for _, e := range menu.Entries {
		if e.ShortName == "s3" {
			if e.IssueBadge.Count != 7 {
				t.Errorf("s3 IssueBadge.Count: got %d want 7", e.IssueBadge.Count)
			}
			return
		}
	}
	t.Error("s3 entry not present in MenuBody.Entries after PatchMenu")
}

// TestMenuIntent_PatchMenuIssueBatch_PopulatesBadges verifies that
// PatchMenuIssueBatch atomically applies issue counts for multiple types.
func TestMenuIntent_PatchMenuIssueBatch_PopulatesBadges(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuIssueBatch{
			Counts: map[string]int{
				"ec2":    3,
				"rds":    0,
				"lambda": 12,
			},
			Truncated: map[string]bool{
				"lambda": true,
			},
			Known: map[string]bool{
				"ec2":    true,
				"rds":    true,
				"lambda": true,
			},
		},
	})

	menu := requireMenuBody(t, c)

	expectations := map[string]int{
		"ec2":    3,
		"rds":    0,
		"lambda": 12,
	}
	found := map[string]bool{}
	for _, e := range menu.Entries {
		if want, ok := expectations[e.ShortName]; ok {
			found[e.ShortName] = true
			if e.IssueBadge.Count != want {
				t.Errorf("%s IssueBadge.Count: got %d want %d", e.ShortName, e.IssueBadge.Count, want)
			}
		}
	}
	for name := range expectations {
		if !found[name] {
			t.Errorf("entry %q not found in MenuBody.Entries after PatchMenuIssueBatch", name)
		}
	}
}

// TestMenuIntent_MenuClearAvailabilityIntent_ClearsAllCounts verifies that
// MenuClearAvailabilityIntent resets all availability and issue state so the
// menu returns to the "no data yet" state — matching mainmenu.go ClearAvailability.
func TestMenuIntent_MenuClearAvailabilityIntent_ClearsAllCounts(t *testing.T) {
	c := newMenuController(t)

	// Seed some availability and issue data.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 5},
		runtime.PatchMenu{ResourceType: "ec2", Issues: 3},
	})

	// Clear everything via MenuClearAvailabilityIntent.
	c.ApplyIntents([]runtime.UIIntent{runtime.MenuClearAvailabilityIntent{}})

	menu := requireMenuBody(t, c)
	for _, e := range menu.Entries {
		if e.Availability != 0 {
			t.Errorf("after clear: %q Availability=%d want 0", e.ShortName, e.Availability)
		}
		if e.IssueBadge.Count != 0 {
			t.Errorf("after clear: %q IssueBadge.Count=%d want 0", e.ShortName, e.IssueBadge.Count)
		}
	}
}

// TestMenuIntent_PatchMenuCheckProgress_SurfacesInMenuBody verifies that
// PatchMenuCheckProgress results in MenuBody.Progress carrying a non-empty
// progress string while in-progress (Checked < Total).
func TestMenuIntent_PatchMenuCheckProgress_SurfacesInMenuBody(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuCheckProgress{Checked: 4, Total: 10},
	})

	menu := requireMenuBody(t, c)
	if menu.Progress == "" {
		t.Error("MenuBody.Progress: expected non-empty string while check is in progress (4/10), got empty")
	}
}

// TestMenuIntent_PatchMenuCheckProgress_ZeroTotalClearsProgress verifies that
// PatchMenuCheckProgress{Total:0} clears the progress indicator — matching
// mainmenu.go SetCheckProgress(0,0) semantics ("scan complete").
func TestMenuIntent_PatchMenuCheckProgress_ZeroTotalClearsProgress(t *testing.T) {
	c := newMenuController(t)

	// First set a progress state.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuCheckProgress{Checked: 4, Total: 10},
	})
	// Then signal completion.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuCheckProgress{Checked: 10, Total: 0},
	})

	menu := requireMenuBody(t, c)
	if menu.Progress != "" {
		t.Errorf("MenuBody.Progress: expected empty after Total=0 (scan complete), got %q", menu.Progress)
	}
}

// TestMenuIntent_PatchMenuEnrichProgress_SurfacesInMenuBody verifies that
// PatchMenuEnrichProgress results in MenuBody.Progress carrying enrichment info.
func TestMenuIntent_PatchMenuEnrichProgress_SurfacesInMenuBody(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuEnrichProgress{Checked: 2, Total: 8},
	})

	menu := requireMenuBody(t, c)
	if menu.Progress == "" {
		t.Error("MenuBody.Progress: expected non-empty string while enrichment in progress (2/8), got empty")
	}
}

// TestMenuIntent_PatchMenuEnrichProgress_ZeroTotalClearsProgress verifies that
// PatchMenuEnrichProgress{Total:0} clears the progress indicator.
func TestMenuIntent_PatchMenuEnrichProgress_ZeroTotalClearsProgress(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuEnrichProgress{Checked: 8, Total: 8},
	})
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuEnrichProgress{Checked: 8, Total: 0},
	})

	menu := requireMenuBody(t, c)
	if menu.Progress != "" {
		t.Errorf("MenuBody.Progress: expected empty after Total=0 (enrichment complete), got %q", menu.Progress)
	}
}

// =============================================================================
// 3. Menu actions
// =============================================================================

// TestMenuAction_MoveDown_AdvancesSelected verifies that ActionMoveDown
// increments MenuBody.Selected by one.
func TestMenuAction_MoveDown_AdvancesSelected(t *testing.T) {
	c := newMenuController(t)

	before := requireMenuBody(t, c)
	if before.Selected != 0 {
		t.Fatalf("precondition: Selected=%d want 0", before.Selected)
	}

	c.Apply(app.Action{Kind: app.ActionMoveDown})

	after := requireMenuBody(t, c)
	if after.Selected != 1 {
		t.Errorf("ActionMoveDown: Selected=%d want 1", after.Selected)
	}
}

// TestMenuAction_MoveUp_DecrementsSelected verifies that ActionMoveUp after
// a MoveDown returns to the original position.
func TestMenuAction_MoveUp_DecrementsSelected(t *testing.T) {
	c := newMenuController(t)
	c.Apply(app.Action{Kind: app.ActionMoveDown})

	before := requireMenuBody(t, c)
	c.Apply(app.Action{Kind: app.ActionMoveUp})

	after := requireMenuBody(t, c)
	if after.Selected != before.Selected-1 {
		t.Errorf("ActionMoveUp: Selected=%d want %d", after.Selected, before.Selected-1)
	}
}

// TestMenuAction_MoveDown_Then_MoveUp_ReturnToStart verifies the round-trip:
// down then up leaves the cursor at position 0.
func TestMenuAction_MoveDown_Then_MoveUp_ReturnToStart(t *testing.T) {
	c := newMenuController(t)

	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveUp})

	menu := requireMenuBody(t, c)
	if menu.Selected != 0 {
		t.Errorf("down+up round-trip: Selected=%d want 0", menu.Selected)
	}
}

// TestMenuAction_MoveTop_JumpsToFirstEntry verifies that ActionMoveTop
// sets Selected to 0 regardless of current position.
func TestMenuAction_MoveTop_JumpsToFirstEntry(t *testing.T) {
	c := newMenuController(t)

	// Move down a few positions first.
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	c.Apply(app.Action{Kind: app.ActionMoveDown})

	c.Apply(app.Action{Kind: app.ActionMoveTop})

	menu := requireMenuBody(t, c)
	if menu.Selected != 0 {
		t.Errorf("ActionMoveTop: Selected=%d want 0", menu.Selected)
	}
}

// TestMenuAction_MoveBottom_JumpsToLastEntry verifies that ActionMoveBottom
// sets Selected to the last visible entry index.
func TestMenuAction_MoveBottom_JumpsToLastEntry(t *testing.T) {
	c := newMenuController(t)

	c.Apply(app.Action{Kind: app.ActionMoveBottom})

	menu := requireMenuBody(t, c)
	lastIdx := len(menu.Entries) - 1
	if lastIdx < 0 {
		t.Fatal("MenuBody.Entries is empty — cannot test MoveBottom")
	}
	if menu.Selected != lastIdx {
		t.Errorf("ActionMoveBottom: Selected=%d want %d (last entry)", menu.Selected, lastIdx)
	}
}

// TestMenuAction_PageDown_AdvancesSelectedByPageSize verifies that
// ActionPageDown moves Selected forward by a page (> 1 position).
func TestMenuAction_PageDown_AdvancesSelectedByPageSize(t *testing.T) {
	c := newMenuController(t)

	before := requireMenuBody(t, c)
	c.Apply(app.Action{Kind: app.ActionPageDown})
	after := requireMenuBody(t, c)

	if len(after.Entries) < 2 {
		t.Skip("fewer than 2 entries — cannot test PageDown")
	}
	if after.Selected <= before.Selected {
		t.Errorf("ActionPageDown: Selected did not advance: before=%d after=%d", before.Selected, after.Selected)
	}
}

// TestMenuAction_PageUp_DecreasesSelectedOrClampsToZero verifies that
// ActionPageUp after PageDown moves Selected backward (or stays at 0 if
// already at top).
func TestMenuAction_PageUp_DecreasesSelectedOrClampsToZero(t *testing.T) {
	c := newMenuController(t)

	c.Apply(app.Action{Kind: app.ActionPageDown})
	midMenu := requireMenuBody(t, c)
	c.Apply(app.Action{Kind: app.ActionPageUp})
	after := requireMenuBody(t, c)

	if after.Selected > midMenu.Selected {
		t.Errorf("ActionPageUp: Selected increased from %d to %d", midMenu.Selected, after.Selected)
	}
}

// TestMenuAction_ToggleAttention_FlipsAttentionOnly verifies that
// ActionToggleAttention toggles MenuBody.AttentionOnly between false and true.
func TestMenuAction_ToggleAttention_FlipsAttentionOnly(t *testing.T) {
	c := newMenuController(t)

	before := requireMenuBody(t, c)
	if before.AttentionOnly {
		t.Fatal("precondition: AttentionOnly should be false on fresh controller")
	}

	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	after := requireMenuBody(t, c)
	if !after.AttentionOnly {
		t.Error("ActionToggleAttention: AttentionOnly should be true after first toggle")
	}

	// Second toggle reverts.
	c.Apply(app.Action{Kind: app.ActionToggleAttention})
	reverted := requireMenuBody(t, c)
	if reverted.AttentionOnly {
		t.Error("ActionToggleAttention: AttentionOnly should be false after second toggle")
	}
}

// TestMenuAction_SetFilter_NarrowsVisibleEntries verifies that
// ActionSetFilter{Arg:"ec"} narrows MenuBody.Entries to only types whose
// ShortName or display name contains "ec" (case-insensitive), mirroring the
// ≥2-char filter in mainmenu.go applyFilter.
func TestMenuAction_SetFilter_NarrowsVisibleEntries(t *testing.T) {
	c := newMenuController(t)

	unfiltered := requireMenuBody(t, c)
	allCount := len(unfiltered.Entries)
	if allCount == 0 {
		t.Fatal("MenuBody.Entries is empty before filter — catalog may not be loaded")
	}

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "ec"})

	filtered := requireMenuBody(t, c)
	if len(filtered.Entries) == 0 {
		t.Error("ActionSetFilter(ec): all entries hidden — expected at least ec2 to match")
	}
	if len(filtered.Entries) >= allCount {
		t.Errorf("ActionSetFilter(ec): entry count did not decrease (%d → %d)", allCount, len(filtered.Entries))
	}

	// Every remaining entry must contain "ec" in its ShortName or Display name.
	for _, e := range filtered.Entries {
		nameLC := strings.ToLower(e.Display)
		shortLC := strings.ToLower(e.ShortName)
		if !strings.Contains(nameLC, "ec") && !strings.Contains(shortLC, "ec") {
			t.Errorf("entry %q (%q) visible after filter %q but does not match", e.ShortName, e.Display, "ec")
		}
	}
}

// TestMenuAction_SetFilter_SingleChar_NoFilter verifies that a one-character
// filter is ignored (too ambiguous), mirroring mainmenu.go's < 2 char guard.
func TestMenuAction_SetFilter_SingleChar_NoFilter(t *testing.T) {
	c := newMenuController(t)

	unfiltered := requireMenuBody(t, c)
	allCount := len(unfiltered.Entries)

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "e"})

	after := requireMenuBody(t, c)
	if len(after.Entries) != allCount {
		t.Errorf("ActionSetFilter(1-char): entry count changed (%d → %d); single-char filter should be no-op", allCount, len(after.Entries))
	}
}

// TestMenuAction_SetFilter_StoresFilterInMenuBody verifies that the filter
// text appears in MenuBody.Filter so renderers can display it.
func TestMenuAction_SetFilter_StoresFilterInMenuBody(t *testing.T) {
	c := newMenuController(t)

	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "rds"})

	menu := requireMenuBody(t, c)
	if menu.Filter != "rds" {
		t.Errorf("MenuBody.Filter: got %q want %q", menu.Filter, "rds")
	}
}

// TestMenuAction_Select_NavigatesToResourceList verifies that ActionSelect on
// a visible, non-empty (or unknown) menu entry navigates to the resource list,
// changing the body kind to BodyKindList.
func TestMenuAction_Select_NavigatesToResourceList(t *testing.T) {
	c := newMenuController(t)

	// Ensure the first entry has unknown availability (not confirmed empty) so
	// navigation is permitted.
	menu := requireMenuBody(t, c)
	if len(menu.Entries) == 0 {
		t.Fatal("MenuBody.Entries is empty — cannot test ActionSelect")
	}

	vs, _ := c.Apply(app.Action{Kind: app.ActionSelect})
	if vs.Body.Kind != app.BodyKindList {
		t.Errorf("ActionSelect on available entry: got BodyKind %q want BodyKindList", vs.Body.Kind)
	}
}

// TestMenuAction_Select_BlockedForConfirmedEmpty verifies that ActionSelect is
// a no-op (stays on BodyKindMenu) when the selected entry is confirmed empty
// (Availability==0 and not truncated), mirroring mainmenu.go Enter semantics.
func TestMenuAction_Select_BlockedForConfirmedEmpty(t *testing.T) {
	c := newMenuController(t)

	// Find the first resource type and mark it confirmed empty.
	menu := requireMenuBody(t, c)
	if len(menu.Entries) == 0 {
		t.Fatal("MenuBody.Entries is empty")
	}
	target := menu.Entries[0].ShortName

	// Mark as confirmed empty: count=0, truncated=false.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: target, Count: 0, Truncated: false},
	})

	// Cursor is at 0 (the confirmed-empty entry). Select must be blocked.
	vs, _ := c.Apply(app.Action{Kind: app.ActionSelect})
	if vs.Body.Kind != app.BodyKindMenu {
		t.Errorf("ActionSelect on confirmed-empty entry: got BodyKind %q want BodyKindMenu (blocked)", vs.Body.Kind)
	}
}

// TestMenuAction_Select_AllowedForTruncatedZero verifies that ActionSelect is
// allowed when availability is 0 but truncated=true, mirroring mainmenu.go:
// "truncated-zero is not confirmed empty — more pages may exist."
func TestMenuAction_Select_AllowedForTruncatedZero(t *testing.T) {
	c := newMenuController(t)

	menu := requireMenuBody(t, c)
	if len(menu.Entries) == 0 {
		t.Fatal("MenuBody.Entries is empty")
	}
	target := menu.Entries[0].ShortName

	// Truncated zero — navigation must be allowed.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: target, Count: 0, Truncated: true},
	})

	vs, _ := c.Apply(app.Action{Kind: app.ActionSelect})
	if vs.Body.Kind != app.BodyKindList {
		t.Errorf("ActionSelect on truncated-zero entry: got BodyKind %q want BodyKindList", vs.Body.Kind)
	}
}

// =============================================================================
// 4. Snapshot MenuBody visibility parity
// =============================================================================

// TestMenuSnapshot_AllRegisteredTypesVisibleByDefault verifies that without
// any filter or attention toggle, all registered resource types appear in
// MenuBody.Entries — one entry per type.
func TestMenuSnapshot_AllRegisteredTypesVisibleByDefault(t *testing.T) {
	c := newMenuController(t)

	menu := requireMenuBody(t, c)
	allTypes := resource.AllResourceTypes()

	if len(menu.Entries) != len(allTypes) {
		t.Errorf("MenuBody.Entries count: got %d want %d (all resource types)", len(menu.Entries), len(allTypes))
	}

	// Build a set of expected short names.
	expectedSet := make(map[string]bool, len(allTypes))
	for _, rt := range allTypes {
		expectedSet[rt.ShortName] = true
	}
	for _, e := range menu.Entries {
		if !expectedSet[e.ShortName] {
			t.Errorf("unexpected entry %q in MenuBody.Entries", e.ShortName)
		}
	}
}

// TestMenuSnapshot_AttentionOnly_ColdStart_AllVisible verifies that when
// AttentionOnly is enabled but no type has been probed yet (issueKnown is
// empty), ALL types remain visible. This mirrors isVisibleUnderIssueFilter's
// cold-start behaviour: len(issueKnown)==0 → show everything.
func TestMenuSnapshot_AttentionOnly_ColdStart_AllVisible(t *testing.T) {
	c := newMenuController(t)
	allTypes := resource.AllResourceTypes()

	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	menu := requireMenuBody(t, c)
	if !menu.AttentionOnly {
		t.Fatal("AttentionOnly should be true after toggle")
	}

	// Cold-start: no issue counts seeded — all types visible.
	if len(menu.Entries) != len(allTypes) {
		t.Errorf("cold-start attention-only: got %d entries want %d (all types visible while none probed)", len(menu.Entries), len(allTypes))
	}
}

// TestMenuSnapshot_AttentionOnly_HidesNeutralTypes verifies that after
// at least one type has been probed and AttentionOnly is enabled, types
// with zero issues (confirmed no issues) are hidden.
func TestMenuSnapshot_AttentionOnly_HidesNeutralTypes(t *testing.T) {
	c := newMenuController(t)

	// Seed: ec2 has 3 issues, s3 has 0 issues (known). Once any type is known,
	// the cold-start path ends and confirmed-zero types are hidden.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuIssueBatch{
			Counts:    map[string]int{"ec2": 3, "s3": 0},
			Truncated: map[string]bool{},
			Known:     map[string]bool{"ec2": true, "s3": true},
		},
	})

	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	menu := requireMenuBody(t, c)
	if !menu.AttentionOnly {
		t.Fatal("AttentionOnly should be true after toggle")
	}

	// s3 with 0 issues (not truncated, known) must not appear.
	for _, e := range menu.Entries {
		if e.ShortName == "s3" {
			t.Error("s3 with confirmed-zero issues should be hidden under AttentionOnly filter")
		}
	}

	// ec2 with 3 issues must appear.
	var ec2Found bool
	for _, e := range menu.Entries {
		if e.ShortName == "ec2" {
			ec2Found = true
		}
	}
	if !ec2Found {
		t.Error("ec2 with 3 issues should be visible under AttentionOnly filter")
	}
}

// TestMenuSnapshot_AttentionOnly_TruncatedZeroIssues_Visible verifies that
// a type with truncated issue count zero is visible under AttentionOnly — more
// pages may carry issues. Mirrors isVisibleUnderIssueFilter truncated path.
func TestMenuSnapshot_AttentionOnly_TruncatedZeroIssues_Visible(t *testing.T) {
	c := newMenuController(t)

	// Seed: rds has 0 issues but truncated (lower bound).
	// Also seed one known non-truncated type so cold-start ends.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuIssueBatch{
			Counts:    map[string]int{"rds": 0, "ec2": 1},
			Truncated: map[string]bool{"rds": true},
			Known:     map[string]bool{"rds": true, "ec2": true},
		},
	})

	c.Apply(app.Action{Kind: app.ActionToggleAttention})

	menu := requireMenuBody(t, c)
	var rdsFound bool
	for _, e := range menu.Entries {
		if e.ShortName == "rds" {
			rdsFound = true
		}
	}
	if !rdsFound {
		t.Error("rds with truncated-zero issues should remain visible under AttentionOnly (lower bound)")
	}
}

// TestMenuSnapshot_ExcludeFromIssueBadge_HiddenUnderAttentionOnly verifies
// that types marked ExcludeFromIssueBadge are always hidden when AttentionOnly
// is active — they are never probed, so they have no issue signal.
func TestMenuSnapshot_ExcludeFromIssueBadge_HiddenUnderAttentionOnly(t *testing.T) {
	c := newMenuController(t)

	// Collect all ExcludeFromIssueBadge types.
	var excluded []string
	for _, rt := range resource.AllResourceTypes() {
		if rt.ExcludeFromIssueBadge {
			excluded = append(excluded, rt.ShortName)
		}
	}
	if len(excluded) == 0 {
		t.Skip("no ExcludeFromIssueBadge types registered — skip")
	}

	// Seed one non-excluded known type so cold-start ends.
	for _, rt := range resource.AllResourceTypes() {
		if !rt.ExcludeFromIssueBadge {
			c.ApplyIntents([]runtime.UIIntent{
				runtime.PatchMenu{ResourceType: rt.ShortName, Issues: 1},
			})
			// Also mark as known via PatchMenuIssueBatch so issueKnown is populated.
			c.ApplyIntents([]runtime.UIIntent{
				runtime.PatchMenuIssueBatch{
					Counts:    map[string]int{rt.ShortName: 1},
					Truncated: map[string]bool{},
					Known:     map[string]bool{rt.ShortName: true},
				},
			})
			break
		}
	}

	c.Apply(app.Action{Kind: app.ActionToggleAttention})
	menu := requireMenuBody(t, c)

	excludedSet := make(map[string]bool, len(excluded))
	for _, name := range excluded {
		excludedSet[name] = true
	}

	for _, e := range menu.Entries {
		if excludedSet[e.ShortName] {
			t.Errorf("ExcludeFromIssueBadge type %q must be hidden under AttentionOnly", e.ShortName)
		}
	}
}

// TestMenuSnapshot_IssueBadge_OnlyShownWhenKnownAndNonZero verifies that the
// issue badge appears only for types where the issue count is both known
// (issueKnown[type]==true) and non-zero — mirroring mainmenu.go issueBadge().
func TestMenuSnapshot_IssueBadge_OnlyShownWhenKnownAndNonZero(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuIssueBatch{
			Counts:    map[string]int{"ec2": 5, "rds": 0, "lambda": 2},
			Truncated: map[string]bool{},
			Known:     map[string]bool{"ec2": true, "rds": true, "lambda": true},
		},
	})

	menu := requireMenuBody(t, c)

	for _, e := range menu.Entries {
		switch e.ShortName {
		case "ec2":
			if e.IssueBadge.Count != 5 {
				t.Errorf("ec2 IssueBadge.Count: got %d want 5", e.IssueBadge.Count)
			}
		case "rds":
			// Known but zero — badge should be absent (count=0, matching issueBadge() which returns "" for count==0).
			if e.IssueBadge.Count != 0 {
				t.Errorf("rds IssueBadge.Count: got %d want 0 (badge hidden for zero)", e.IssueBadge.Count)
			}
		case "lambda":
			if e.IssueBadge.Count != 2 {
				t.Errorf("lambda IssueBadge.Count: got %d want 2", e.IssueBadge.Count)
			}
		}
	}
}

// TestMenuSnapshot_IssueBadge_UnknownType_NoBadge verifies that a type not
// yet probed (not in issueKnown) carries no issue badge — the badge is not
// shown for unknown types, only for confirmed-known ones.
func TestMenuSnapshot_IssueBadge_UnknownType_NoBadge(t *testing.T) {
	c := newMenuController(t)

	// Only set ec2 as known; s3 remains unknown.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuIssueBatch{
			Counts:    map[string]int{"ec2": 4},
			Truncated: map[string]bool{},
			Known:     map[string]bool{"ec2": true},
		},
	})

	menu := requireMenuBody(t, c)
	for _, e := range menu.Entries {
		if e.ShortName == "s3" {
			if e.IssueBadge.Count != 0 {
				t.Errorf("s3 (not probed): IssueBadge.Count=%d want 0 (no badge for unknown)", e.IssueBadge.Count)
			}
			return
		}
	}
	t.Error("s3 entry not found in MenuBody.Entries")
}

// TestMenuSnapshot_IssueBadgeTruncated_ReflectsInEntry verifies that
// IssueBadge.Truncated is set when the issue count is a lower bound.
func TestMenuSnapshot_IssueBadgeTruncated_ReflectsInEntry(t *testing.T) {
	c := newMenuController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuIssueBatch{
			Counts:    map[string]int{"ec2": 10},
			Truncated: map[string]bool{"ec2": true},
			Known:     map[string]bool{"ec2": true},
		},
	})

	menu := requireMenuBody(t, c)
	for _, e := range menu.Entries {
		if e.ShortName == "ec2" {
			if !e.IssueBadge.Truncated {
				t.Error("ec2 IssueBadge.Truncated: got false want true")
			}
			return
		}
	}
	t.Error("ec2 entry not found")
}

// TestMenuSnapshot_ConfirmedEmptyEntry_StillInEntries verifies that a
// confirmed-empty entry (availability known, count=0, not truncated) is still
// present in MenuBody.Entries — it just carries Availability==0 so the renderer
// can dim it. The entry must NOT be hidden from the list.
func TestMenuSnapshot_ConfirmedEmptyEntry_StillInEntries(t *testing.T) {
	c := newMenuController(t)

	// Mark ec2 as confirmed empty.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 0, Truncated: false},
	})

	menu := requireMenuBody(t, c)
	for _, e := range menu.Entries {
		if e.ShortName == "ec2" {
			if e.Availability != 0 {
				t.Errorf("confirmed-empty ec2: Availability=%d want 0", e.Availability)
			}
			return
		}
	}
	t.Error("confirmed-empty ec2 entry not found in MenuBody.Entries — must be present (renderer dims it)")
}

// TestMenuSnapshot_EntryDisplayNameMatchesCatalog verifies that each
// MenuEntry.Display matches the corresponding ResourceTypeDef.Name from the
// catalog. This pins the Display-vs-ShortName mapping contract.
func TestMenuSnapshot_EntryDisplayNameMatchesCatalog(t *testing.T) {
	c := newMenuController(t)

	menu := requireMenuBody(t, c)
	typesByShortName := make(map[string]resource.ResourceTypeDef)
	for _, rt := range resource.AllResourceTypes() {
		typesByShortName[rt.ShortName] = rt
	}

	for _, e := range menu.Entries {
		rt, ok := typesByShortName[e.ShortName]
		if !ok {
			t.Errorf("entry %q not found in AllResourceTypes()", e.ShortName)
			continue
		}
		if e.Display != rt.Name {
			t.Errorf("entry %q Display=%q want %q (from catalog)", e.ShortName, e.Display, rt.Name)
		}
	}
}

// TestMenuSnapshot_Filter_ClearedByEmptyString verifies that setting an empty
// filter after a non-empty one restores all entries. Mirroring applyFilter
// which treats len < 2 as "no filter".
func TestMenuSnapshot_Filter_ClearedByEmptyString(t *testing.T) {
	c := newMenuController(t)

	allTypes := resource.AllResourceTypes()
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "ec"})
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: ""})

	menu := requireMenuBody(t, c)
	if len(menu.Entries) != len(allTypes) {
		t.Errorf("after clearing filter: got %d entries want %d", len(menu.Entries), len(allTypes))
	}
	if menu.Filter != "" {
		t.Errorf("MenuBody.Filter after clear: got %q want %q", menu.Filter, "")
	}
}

// TestMenuSnapshot_Back_FromResourceList_ReturnsToMenu verifies the full
// navigation: ActionSelect navigates to BodyKindList, then ActionBack
// returns to BodyKindMenu.
func TestMenuSnapshot_Back_FromResourceList_ReturnsToMenu(t *testing.T) {
	c := newMenuController(t)

	// Navigate into the resource list.
	vs, _ := c.Apply(app.Action{Kind: app.ActionSelect})
	if vs.Body.Kind != app.BodyKindList {
		t.Skipf("ActionSelect did not navigate to list (got %q) — skipping back test", vs.Body.Kind)
	}

	// Navigate back to menu.
	vs, _ = c.Apply(app.Action{Kind: app.ActionBack})
	if vs.Body.Kind != app.BodyKindMenu {
		t.Errorf("ActionBack from resource list: got BodyKind %q want BodyKindMenu", vs.Body.Kind)
	}
}
