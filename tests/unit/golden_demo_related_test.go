package unit

// golden_demo_related_test.go — Golden tests for demo-mode related views.
//
// These tests verify real consistency between registration layers, NOT
// tautological "demo checker returns what demo checker returns" checks.
//
// What they catch:
//   - RelatedDef targets missing from demo checker (shows loading/unknown in demo)
//   - Demo checker ResourceIDs that don't exist in demo fixtures (navigation fails)
//   - Live checkers that are nil (demo works, live silently returns -1)
//   - Demo checker targets not in RegisterRelated (dead demo code)
//   - Navigation roundtrip failures (navigate to related, Esc back)

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// goldenDemoModel creates a demo-mode root model sized at 120×36.
func goldenDemoModel(t *testing.T) tui.Model {
	t.Helper()
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})
	return m
}

// goldenDrainBatch executes a cmd chain that may contain tea.BatchMsg at any
// level and collects all produced messages. Expands BatchMsg by executing
// every sub-cmd individually so that all async checker results are captured.
func goldenDrainBatch(t *testing.T, m tui.Model, cmd tea.Cmd, maxOps int) (tui.Model, []tea.Msg) {
	t.Helper()
	var allMsgs []tea.Msg
	queue := []tea.Cmd{cmd}
	for ops := 0; ops < maxOps && len(queue) > 0; ops++ {
		next := queue[0]
		queue = queue[1:]
		if next == nil {
			continue
		}
		msg := next()
		if msg == nil {
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub != nil {
					queue = append(queue, sub)
				}
			}
			continue
		}
		allMsgs = append(allMsgs, msg)
		var nextCmd tea.Cmd
		m, nextCmd = rootApplyMsg(m, msg)
		if nextCmd != nil {
			queue = append(queue, nextCmd)
		}
	}
	return m, allMsgs
}

// ---------------------------------------------------------------------------
// Test 1: Every RelatedDef target must have a matching demo checker entry.
//
// If RegisterRelated("ec2", [...{TargetType:"tg"}, {TargetType:"alarm"}...])
// but RegisterRelatedDemo("ec2") only returns results for "tg", then "alarm"
// shows as loading/unknown forever in demo mode.
// ---------------------------------------------------------------------------

func TestGolden_DemoRelatedCoverage(t *testing.T) {
	for _, shortName := range resource.AllShortNames() {
		defs := resource.GetRelated(shortName)
		if len(defs) == 0 {
			continue
		}

		demoFn := resource.GetRelatedDemo(shortName)
		if demoFn == nil {
			// ct-events intentionally has no RegisterRelatedDemo override: its real
			// checkers are pure field-readers that work correctly with in-memory demo
			// fixture data. app_related.go falls through to the real checkers when no
			// demo override is registered. See internal/demo/fixtures_related.go.
			if shortName == "ct-events" {
				continue
			}
			t.Errorf("%s: has %d RelatedDefs but no RegisterRelatedDemo — entire right column broken in demo", shortName, len(defs))
			continue
		}

		// Get first fixture to evaluate conditional demo checkers.
		fixtures, ok := demo.GetResources(shortName)
		if !ok || len(fixtures) == 0 {
			continue // no demo fixtures = can't evaluate
		}

		demoResults := demoFn(fixtures[0])
		demoTargets := make(map[string]bool, len(demoResults))
		for _, r := range demoResults {
			demoTargets[r.TargetType] = true
		}

		for _, def := range defs {
			t.Run(fmt.Sprintf("%s→%s", shortName, def.TargetType), func(t *testing.T) {
				if !demoTargets[def.TargetType] {
					t.Errorf("RelatedDef target %q registered for %s but missing from demo checker — shows loading/unknown in demo mode",
						def.TargetType, shortName)
				}
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Test 2: Every ResourceID returned by demo checkers must exist in demo fixtures.
//
// If demo checker says ec2→tg has IDs ["acme-web-tg"] but demo.GetResources("tg")
// has no resource with that ID, then navigating ec2→tg in demo mode fails.
// ---------------------------------------------------------------------------

func TestGolden_DemoRelatedIDsExist(t *testing.T) {
	// Build lookup of all demo fixture IDs per type.
	fixtureIDs := make(map[string]map[string]bool)
	for _, shortName := range resource.AllShortNames() {
		fixtures, ok := demo.GetResources(shortName)
		if !ok {
			continue
		}
		ids := make(map[string]bool, len(fixtures))
		for _, f := range fixtures {
			ids[f.ID] = true
		}
		fixtureIDs[shortName] = ids
	}

	for _, shortName := range resource.AllShortNames() {
		demoFn := resource.GetRelatedDemo(shortName)
		if demoFn == nil {
			continue
		}
		fixtures, ok := demo.GetResources(shortName)
		if !ok || len(fixtures) == 0 {
			continue
		}

		results := demoFn(fixtures[0])
		for _, r := range results {
			if r.Count <= 0 || len(r.ResourceIDs) == 0 {
				continue
			}

			targetFixtures, hasFixtures := fixtureIDs[r.TargetType]
			if !hasFixtures {
				t.Errorf("%s→%s: demo checker returns %d IDs but no demo fixtures exist for target type %q",
					shortName, r.TargetType, len(r.ResourceIDs), r.TargetType)
				continue
			}

			for _, id := range r.ResourceIDs {
				t.Run(fmt.Sprintf("%s→%s/%s", shortName, r.TargetType, id), func(t *testing.T) {
					if !targetFixtures[id] {
						t.Errorf("demo checker for %s returns ID %q for target %q, but that ID does not exist in demo.GetResources(%q) — navigation will fail",
							shortName, id, r.TargetType, r.TargetType)
					}
				})
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Test 3: Every demo checker target must have a matching RegisterRelated entry.
//
// If demo checker returns a result for target "foo" but there's no RelatedDef
// for "foo", the demo result is dead code — it never reaches the right column.
// ---------------------------------------------------------------------------

func TestGolden_DemoRelatedNoOrphans(t *testing.T) {
	for _, shortName := range resource.AllShortNames() {
		demoFn := resource.GetRelatedDemo(shortName)
		if demoFn == nil {
			continue
		}
		fixtures, ok := demo.GetResources(shortName)
		if !ok || len(fixtures) == 0 {
			continue
		}

		defs := resource.GetRelated(shortName)
		defTargets := make(map[string]bool, len(defs))
		for _, d := range defs {
			defTargets[d.TargetType] = true
		}

		results := demoFn(fixtures[0])
		for _, r := range results {
			t.Run(fmt.Sprintf("%s→%s", shortName, r.TargetType), func(t *testing.T) {
				if !defTargets[r.TargetType] {
					t.Errorf("demo checker for %s returns target %q but no RegisterRelated entry exists — dead demo code",
						shortName, r.TargetType)
				}
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Test 4: Every registered RelatedDef MUST have a live Checker.
//
// If Checker is nil the right column shows "—" in live mode while demo
// mode happily shows a count. That's a bug, not a TODO.
// See: https://github.com/k2m30/a9s/issues/243
// ---------------------------------------------------------------------------

func TestGolden_LiveCheckerCompleteness(t *testing.T) {
	for _, shortName := range resource.AllShortNames() {
		defs := resource.GetRelated(shortName)
		for _, def := range defs {
			t.Run(fmt.Sprintf("%s→%s", shortName, def.TargetType), func(t *testing.T) {
				if def.Checker == nil {
					t.Errorf("Checker is nil — shows dash in live mode, count in demo. Implement it. (#243)")
				}
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Test 5: Navigation roundtrip — navigate to related, Esc back.
//
// For each resource type with Count>0 related resources, verify:
//   1. Navigate to detail view
//   2. Trigger related checks
//   3. Navigate to first related resource with Count>0
//   4. Press Esc
//   5. Verify we're back on the original detail view
// ---------------------------------------------------------------------------

func TestGolden_DemoRelatedNavRoundTrip(t *testing.T) {
	for _, shortName := range resource.AllShortNames() {
		demoFn := resource.GetRelatedDemo(shortName)
		if demoFn == nil {
			continue
		}
		fixtures, ok := demo.GetResources(shortName)
		if !ok || len(fixtures) == 0 {
			continue
		}
		firstRes := fixtures[0]

		// Find the first related target with Count>0 and ResourceIDs.
		results := demoFn(firstRes)
		var navTarget *resource.RelatedCheckResult
		for i := range results {
			if results[i].Count > 0 && len(results[i].ResourceIDs) > 0 {
				navTarget = &results[i]
				break
			}
		}
		if navTarget == nil {
			continue // nothing to navigate to
		}

		t.Run(fmt.Sprintf("%s→%s", shortName, navTarget.TargetType), func(t *testing.T) {
			// Preload target fixtures so navigation can find the resource.
			targetFixtures, targetOk := demo.GetResources(navTarget.TargetType)
			if !targetOk || len(targetFixtures) == 0 {
				t.Skipf("no demo fixtures for target type %q", navTarget.TargetType)
				return
			}

			m := goldenDemoModel(t)

			// Navigate to detail view for source resource.
			m, _ = rootApplyMsg(m, messages.NavigateMsg{
				Target:       messages.TargetDetail,
				ResourceType: shortName,
				Resource:     &firstRes,
			})

			// Trigger related checks and drain.
			m, startCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
				ResourceType:   shortName,
				SourceResource: firstRes,
			})
			if startCmd != nil {
				m, _ = goldenDrainBatch(t, m, startCmd, 200)
			}

			// Preload target type resources into cache.
			m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
				ResourceType: navTarget.TargetType,
				Resources:    targetFixtures,
			})

			// Navigate to related resource.
			m, navCmd := rootApplyMsg(m, messages.RelatedNavigateMsg{
				TargetType:     navTarget.TargetType,
				RelatedIDs:     navTarget.ResourceIDs,
				SourceResource: firstRes,
				SourceType:     shortName,
			})
			if navCmd != nil {
				m, _ = goldenDrainBatch(t, m, navCmd, 200)
			}

			// Verify we navigated away from the source detail.
			viewAfterNav := stripANSI(rootViewContent(m))
			rt := resource.FindResourceType(navTarget.TargetType)
			if rt == nil {
				t.Fatalf("unknown target type %q", navTarget.TargetType)
			}
			// Check view shows target type content (short name in frame, display name, or resource ID).
			hasTarget := strings.Contains(viewAfterNav, navTarget.TargetType) ||
				strings.Contains(viewAfterNav, rt.Name) ||
				strings.Contains(viewAfterNav, navTarget.ResourceIDs[0])
			if !hasTarget {
				t.Errorf("after navigating %s→%s: view does not contain short name %q, display name %q, or ID %q\nview:\n%s",
					shortName, navTarget.TargetType, navTarget.TargetType, rt.Name, navTarget.ResourceIDs[0], viewAfterNav)
			}

			// Press Esc to go back.
			m, escCmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
			if escCmd != nil {
				m, _ = goldenDrainBatch(t, m, escCmd, 50)
			}

			// Verify we're back on the source detail.
			viewAfterEsc := stripANSI(rootViewContent(m))
			hasSource := strings.Contains(viewAfterEsc, firstRes.ID) ||
				strings.Contains(viewAfterEsc, firstRes.Name)
			if !hasSource {
				t.Errorf("after Esc from %s→%s: view does not contain source ID %q or name %q — back navigation failed\nview:\n%s",
					shortName, navTarget.TargetType, firstRes.ID, firstRes.Name, viewAfterEsc)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 6: Demo-mode related counter mismatch — announced count vs. navigable items.
//
// This is a hard assertion for CONCERNS.md #5 (Known Bugs).
//
// The bug: a demo checker returns Count > 0 but either:
//   (a) ResourceIDs is empty — right column shows "3" but navigation opens an empty list, OR
//   (b) Count != len(ResourceIDs) — right column shows "4" but only 1 ID is provided,
//       so the filtered list has at most 1 item.
//
// Both cases produce a detail-view counter that disagrees with what the user actually
// sees when they press Enter to navigate. The test deliberately fails on every known
// mismatch pair so the coder knows exactly what to fix.
//
// Known failing pairs at time of authoring (2026-04-07):
//   - iam-group → iam-user  : Count=3, ResourceIDs=[]
//   - iam-group → policy    : Count=2, ResourceIDs=[]
//   - policy → role         : Count=5, ResourceIDs=[]
//   - policy → iam-user     : Count=2, ResourceIDs=[]
//   - policy → iam-group    : Count=1, ResourceIDs=[]
//   - role (acme-lambda-execution) → lambda : Count=4, len(ResourceIDs)=1
//
// Fix: either populate ResourceIDs with the correct IDs, or lower Count to
// len(ResourceIDs). Do NOT suppress this test to make it pass; fix the data.
// ---------------------------------------------------------------------------

func TestGoldenDemoRelated_CountMatchesResourceIDs(t *testing.T) {
	for _, shortName := range resource.AllShortNames() {
		demoFn := resource.GetRelatedDemo(shortName)
		if demoFn == nil {
			continue
		}
		fixtures, ok := demo.GetResources(shortName)
		if !ok || len(fixtures) == 0 {
			continue
		}

		// Evaluate the checker against every fixture, not just fixtures[0], so
		// that per-resource-ID switch cases (e.g. "role") are also exercised.
		for _, res := range fixtures {
			results := demoFn(res)
			for _, r := range results {
				if r.Count <= 0 {
					// Count=0 or Count=-1 are fine: no items announced, no mismatch possible.
					continue
				}

				// Case (a): Count > 0 but no ResourceIDs — navigation opens an empty list.
				if len(r.ResourceIDs) == 0 {
					t.Run(fmt.Sprintf("%s/%s→%s/no-ids", shortName, res.ID, r.TargetType), func(t *testing.T) {
						t.Errorf(
							"%s (res=%q) → %s: demo checker announces Count=%d but ResourceIDs is empty — "+
								"right column counter will be %d but navigation opens an empty list. "+
								"Fix: populate ResourceIDs with the actual fixture IDs.",
							shortName, res.ID, r.TargetType, r.Count, r.Count,
						)
					})
					continue
				}

				// Case (b): Count != len(ResourceIDs) — counter disagrees with navigable items.
				if r.Count != len(r.ResourceIDs) {
					t.Run(fmt.Sprintf("%s/%s→%s/count-mismatch", shortName, res.ID, r.TargetType), func(t *testing.T) {
						t.Errorf(
							"%s (res=%q) → %s: demo checker announces Count=%d but len(ResourceIDs)=%d — "+
								"right column shows %d but navigation filters to at most %d items. "+
								"Fix: set Count=len(ResourceIDs) or add the missing IDs.",
							shortName, res.ID, r.TargetType,
							r.Count, len(r.ResourceIDs),
							r.Count, len(r.ResourceIDs),
						)
					})
				}
			}
		}
	}
}
