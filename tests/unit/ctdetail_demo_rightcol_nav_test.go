package unit_test

// ctdetail_demo_rightcol_nav_test.go — right-column navigation dispatch tests.
//
// For each of the 9 demo ct-events fixtures (Cases A–I), this file tests that:
//   - pressing Tab focuses the right column (when actionable rows exist)
//   - pressing Enter on the correct row dispatches messages.RelatedNavigateMsg
//   - the RelatedNavigateMsg.TargetType matches the expected group
//   - each RelatedID in the message resolves to a real demo fixture
//
// Case H (no actionable rows): asserts that Enter on every row emits no RelatedNavigateMsg.
//
// Strategy: construct DetailModel at width 180 (auto-shows right column), inject
// demo checker results via ApplyRelatedResults, then focus with Tab and walk
// the cursor with 'j' to the target row before pressing Enter.
//
// Since rightColumnModel.moveCursor skips non-actionable rows, we compute the
// actionable order from the known demo results and navigate to the target by
// its ordinal position within that ordered list.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helper: assertRelatedIDsResolve
// ---------------------------------------------------------------------------

// assertRelatedIDsResolve verifies that every ID in nav.RelatedIDs exists in
// demo.GetResources(nav.TargetType). Uses t.Errorf so all bad IDs are reported.
// An empty fixture set is a hard failure (fixture missing).
func assertRelatedIDsResolve(t *testing.T, nav messages.RelatedNavigateMsg, subtestName string) {
	t.Helper()
	if len(nav.RelatedIDs) == 0 {
		t.Fatalf("%s: RelatedIDs is empty", subtestName)
	}
	targets, ok := demo.GetResources(nav.TargetType)
	if !ok || len(targets) == 0 {
		t.Fatalf("%s: demo.GetResources(%q) empty — fixture missing", subtestName, nav.TargetType)
	}
	fixtureIDs := make(map[string]bool, len(targets))
	for _, r := range targets {
		fixtureIDs[r.ID] = true
	}
	for _, rid := range nav.RelatedIDs {
		if !fixtureIDs[rid] {
			ids := make([]string, 0, len(targets))
			for id := range fixtureIDs {
				ids = append(ids, id)
			}
			t.Errorf("%s: RelatedID %q not in demo.GetResources(%q). Available: %v",
				subtestName, rid, nav.TargetType, ids)
		}
	}
}

// ---------------------------------------------------------------------------
// Helper: loadCTEventsFixtureByID
// ---------------------------------------------------------------------------

// loadCTEventsFixtureByID returns the demo resource with the given ID from
// the ct-events fixture set. Fatals if not found.
func loadCTEventsFixtureByID(t *testing.T, id string) resource.Resource {
	t.Helper()
	fixtures, ok := demo.GetResources("ct-events")
	if !ok || len(fixtures) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}
	for _, r := range fixtures {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("ct-events fixture %q not found", id)
	panic("unreachable")
}

// ---------------------------------------------------------------------------
// Helper: buildRightColModel
// ---------------------------------------------------------------------------

// buildRightColModel creates a DetailModel with the right column auto-shown
// and populated with demo checker results via ApplyRelatedResults.
// Width 180 triggers auto-show (>= 60). Returns the model ready for Tab+Enter.
func buildRightColModel(t *testing.T, res resource.Resource) views.DetailModel {
	t.Helper()

	// Get the demo checker for ct-events.
	demoFn := resource.GetRelatedDemo("ct-events")
	if demoFn == nil {
		t.Fatal("no demo checker registered for ct-events — RegisterRelatedDemo was not called")
	}

	cfg := configForType("ct-events")
	m := newDetailModel(res, "ct-events", cfg)
	// Width 180 → right column auto-shown.
	m.SetSize(180, 40)

	// Populate the right column with demo checker results.
	results := demoFn(res)
	m.ApplyRelatedResults(results)

	return m
}

// ---------------------------------------------------------------------------
// Helper: dispatchRightColumnEnter
// ---------------------------------------------------------------------------

// dispatchRightColumnEnter focuses the right column, navigates to the row with
// targetGroup as its TargetType, presses Enter, and returns the RelatedNavigateMsg.
//
// The function knows the canonical row order from resource.GetRelated("ct-events")
// (same order as RegisterRelated in ct_events.go). It uses that order plus the
// demo checker results to compute how many 'j' presses are needed to reach the
// target. moveCursor skips non-actionable rows, so we count only actionable rows
// up to the target index.
//
// A row is actionable when count > 0 OR (count == -1 AND fetchFilter is non-empty).
func dispatchRightColumnEnter(t *testing.T, m *views.DetailModel, targetGroup string) messages.RelatedNavigateMsg {
	t.Helper()

	demoFn := resource.GetRelatedDemo("ct-events")
	if demoFn == nil {
		t.Fatal("no demo checker registered for ct-events")
	}
	results := demoFn(m.SourceResource())

	// Build a map of targetType → RelatedCheckResult for quick lookup.
	resultByType := make(map[string]resource.RelatedCheckResult, len(results))
	for _, r := range results {
		resultByType[r.TargetType] = r
	}

	// Get canonical row order.
	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("resource.GetRelated(\"ct-events\") returned no defs")
	}

	// Find target index in canonical order.
	targetIdx := -1
	for i, def := range defs {
		if def.TargetType == targetGroup {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		t.Fatalf("targetGroup %q not found in ct-events RelatedDefs", targetGroup)
	}

	// isActionable mirrors rightColumnModel.isActionableRow logic.
	isActionable := func(r resource.RelatedCheckResult) bool {
		if r.Count == -1 {
			return len(r.FetchFilter) > 0
		}
		return r.Count > 0
	}

	// Count how many 'j' presses are needed:
	// Tab puts cursor at first actionable row (ordinal 0 within actionable rows).
	// Each 'j' moves to the next actionable row.
	// We need to know the ordinal of targetGroup within actionable rows.
	// Rows iterate in canonical def order; moveCursor skips non-actionable.
	actionableOrder := make([]string, 0, len(defs))
	for _, def := range defs {
		r, ok := resultByType[def.TargetType]
		if ok && isActionable(r) {
			actionableOrder = append(actionableOrder, def.TargetType)
		}
	}

	targetOrdinal := -1
	for i, tt := range actionableOrder {
		if tt == targetGroup {
			targetOrdinal = i
			break
		}
	}
	if targetOrdinal == -1 {
		t.Fatalf("targetGroup %q is not an actionable row for fixture %q (actionable: %v)",
			targetGroup, m.SourceResource().ID, actionableOrder)
	}

	tabKey := tea.KeyPressMsg{Code: tea.KeyTab}
	jKey := tea.KeyPressMsg{Code: -1, Text: "j"}
	enterKey := tea.KeyPressMsg{Code: tea.KeyEnter}

	// Focus the right column via Tab.
	updated, _ := m.Update(tabKey)
	*m = updated

	// Navigate to the target row (ordinal 0 = already there after Tab-focus).
	for i := 0; i < targetOrdinal; i++ {
		updated, _ = m.Update(jKey)
		*m = updated
	}

	// Press Enter and capture the command.
	_, cmd := m.Update(enterKey)
	if cmd == nil {
		t.Fatalf("Enter on right-column row %q returned nil cmd — row not actionable or focus not set",
			targetGroup)
	}

	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("Enter on right-column row %q dispatched %T, want messages.RelatedNavigateMsg",
			targetGroup, msg)
	}
	return nav
}

// ---------------------------------------------------------------------------
// Case A: e-a1b2c3d4 — Karpenter DescribeInstances (role only)
// ---------------------------------------------------------------------------

func TestCtEventsRightColumnNav_CaseA(t *testing.T) {
	ensureNoColor(t)

	res := loadCTEventsFixtureByID(t, "e-a1b2c3d4")
	m := buildRightColModel(t, res)

	for _, group := range []string{"role"} {
		group := group
		t.Run(group, func(t *testing.T) {
			mc := m
			nav := dispatchRightColumnEnter(t, &mc, group)
			if nav.TargetType != group {
				t.Errorf("TargetType = %q, want %q", nav.TargetType, group)
			}
			assertRelatedIDsResolve(t, nav, "CaseA/"+group)
		})
	}
}

// ---------------------------------------------------------------------------
// Case B: e-b2c3d4e5 — SSO TerminateInstances (role, ec2)
// ---------------------------------------------------------------------------

func TestCtEventsRightColumnNav_CaseB(t *testing.T) {
	ensureNoColor(t)

	res := loadCTEventsFixtureByID(t, "e-b2c3d4e5")
	m := buildRightColModel(t, res)

	for _, group := range []string{"role", "ec2"} {
		group := group
		t.Run(group, func(t *testing.T) {
			mc := m
			nav := dispatchRightColumnEnter(t, &mc, group)
			if nav.TargetType != group {
				t.Errorf("TargetType = %q, want %q", nav.TargetType, group)
			}
			assertRelatedIDsResolve(t, nav, "CaseB/"+group)
		})
	}
}

// ---------------------------------------------------------------------------
// Case C: e-c3d4e5f6 — IAMUser PutObject AccessDenied (iam-user, s3, s3_objects)
// ---------------------------------------------------------------------------

func TestCtEventsRightColumnNav_CaseC(t *testing.T) {
	ensureNoColor(t)

	res := loadCTEventsFixtureByID(t, "e-c3d4e5f6")
	m := buildRightColModel(t, res)

	for _, group := range []string{"iam-user", "s3", "s3_objects"} {
		group := group
		t.Run(group, func(t *testing.T) {
			mc := m
			nav := dispatchRightColumnEnter(t, &mc, group)
			if nav.TargetType != group {
				t.Errorf("TargetType = %q, want %q", nav.TargetType, group)
			}
			assertRelatedIDsResolve(t, nav, "CaseC/"+group)
		})
	}
}

// ---------------------------------------------------------------------------
// Case D: e-d4e5f6a7 — KMS RotateKey AwsServiceEvent (kms only)
// ---------------------------------------------------------------------------

func TestCtEventsRightColumnNav_CaseD(t *testing.T) {
	ensureNoColor(t)

	res := loadCTEventsFixtureByID(t, "e-d4e5f6a7")
	m := buildRightColModel(t, res)

	for _, group := range []string{"kms"} {
		group := group
		t.Run(group, func(t *testing.T) {
			mc := m
			nav := dispatchRightColumnEnter(t, &mc, group)
			if nav.TargetType != group {
				t.Errorf("TargetType = %q, want %q", nav.TargetType, group)
			}
			assertRelatedIDsResolve(t, nav, "CaseD/"+group)
		})
	}
}

// ---------------------------------------------------------------------------
// Case E: e-e5f6a7b8 — Root PutBucketPolicy (s3 only)
// ---------------------------------------------------------------------------

func TestCtEventsRightColumnNav_CaseE(t *testing.T) {
	ensureNoColor(t)

	res := loadCTEventsFixtureByID(t, "e-e5f6a7b8")
	m := buildRightColModel(t, res)

	for _, group := range []string{"s3"} {
		group := group
		t.Run(group, func(t *testing.T) {
			mc := m
			nav := dispatchRightColumnEnter(t, &mc, group)
			if nav.TargetType != group {
				t.Errorf("TargetType = %q, want %q", nav.TargetType, group)
			}
			assertRelatedIDsResolve(t, nav, "CaseE/"+group)
		})
	}
}

// ---------------------------------------------------------------------------
// Case F: e-f6a7b8c9 — IRSA GetObject (role, s3, s3_objects, vpce)
// ---------------------------------------------------------------------------

func TestCtEventsRightColumnNav_CaseF(t *testing.T) {
	ensureNoColor(t)

	res := loadCTEventsFixtureByID(t, "e-f6a7b8c9")
	m := buildRightColModel(t, res)

	for _, group := range []string{"role", "s3", "s3_objects", "vpce"} {
		group := group
		t.Run(group, func(t *testing.T) {
			mc := m
			nav := dispatchRightColumnEnter(t, &mc, group)
			if nav.TargetType != group {
				t.Errorf("TargetType = %q, want %q", nav.TargetType, group)
			}
			assertRelatedIDsResolve(t, nav, "CaseF/"+group)
		})
	}
}

// ---------------------------------------------------------------------------
// Case G: e-a7b8c9d0 — CrossAccount PutObject (role, s3, s3_objects)
// ---------------------------------------------------------------------------

func TestCtEventsRightColumnNav_CaseG(t *testing.T) {
	ensureNoColor(t)

	res := loadCTEventsFixtureByID(t, "e-a7b8c9d0")
	m := buildRightColModel(t, res)

	for _, group := range []string{"role", "s3", "s3_objects"} {
		group := group
		t.Run(group, func(t *testing.T) {
			mc := m
			nav := dispatchRightColumnEnter(t, &mc, group)
			if nav.TargetType != group {
				t.Errorf("TargetType = %q, want %q", nav.TargetType, group)
			}
			assertRelatedIDsResolve(t, nav, "CaseG/"+group)
		})
	}
}

// ---------------------------------------------------------------------------
// Case H: e-b8c9d0e1 — Insight RunInstances (no actionable groups)
// ---------------------------------------------------------------------------

// TestCtEventsRightColumnNav_CaseH asserts that the Insight fixture has no
// actionable right-column rows: the right column should not be focusable via
// Tab, and pressing Enter on any row must not emit a RelatedNavigateMsg.
func TestCtEventsRightColumnNav_CaseH(t *testing.T) {
	ensureNoColor(t)

	res := loadCTEventsFixtureByID(t, "e-b8c9d0e1")
	m := buildRightColModel(t, res)

	// Attempt to focus via Tab — should be a no-op because there are no actionable rows.
	tabKey := tea.KeyPressMsg{Code: tea.KeyTab}
	m, _ = m.Update(tabKey)

	// Walk every row position and press Enter — none should dispatch RelatedNavigateMsg.
	enterKey := tea.KeyPressMsg{Code: tea.KeyEnter}
	jKey := tea.KeyPressMsg{Code: -1, Text: "j"}

	defs := resource.GetRelated("ct-events")
	for pos := 0; pos < len(defs); pos++ {
		_, cmd := m.Update(enterKey)
		if cmd != nil {
			msg := cmd()
			if _, isNav := msg.(messages.RelatedNavigateMsg); isNav {
				t.Errorf("Case H cursor pos %d: Enter emitted RelatedNavigateMsg — "+
					"Insight fixture must have ZERO actionable right-column rows (design violation)", pos)
			}
		}
		m, _ = m.Update(jKey)
	}
}

// ---------------------------------------------------------------------------
// Case I: e-c9d0e1f2 — NetworkActivity VPCE deny (role, s3, s3_objects, vpce)
// ---------------------------------------------------------------------------

func TestCtEventsRightColumnNav_CaseI(t *testing.T) {
	ensureNoColor(t)

	res := loadCTEventsFixtureByID(t, "e-c9d0e1f2")
	m := buildRightColModel(t, res)

	for _, group := range []string{"role", "s3", "s3_objects", "vpce"} {
		group := group
		t.Run(group, func(t *testing.T) {
			mc := m
			nav := dispatchRightColumnEnter(t, &mc, group)
			if nav.TargetType != group {
				t.Errorf("TargetType = %q, want %q", nav.TargetType, group)
			}
			assertRelatedIDsResolve(t, nav, "CaseI/"+group)
		})
	}
}
