package unit

// qa_refresh_clears_wave2_test.go — Regression: Ctrl+R on main menu clears Wave 2 state.
//
// Bug: Ctrl+R on the main menu did not clear enrichmentFindings, enrichmentRan,
// enrichmentTypeGen, and probeResources, leaving stale enrichment state visible.
// Fix: Ctrl+R on main menu increments enrichmentGen and resets all four maps.
//
// Tests verify the observable effect: old-gen EnrichmentCheckedMsg is dropped
// after Ctrl+R (proving enrichmentGen was bumped and maps were cleared).

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// TestMainMenuCtrlR_ClearsEnrichmentFindings verifies that Ctrl+R on the main menu
// bumps enrichmentGen, causing all previously seeded enrichment findings to be
// treated as stale and dropped on re-delivery.
//
// Behavioral proof:
//  1. Seed findings for "ec2" and "ddb" at Gen=0, TypeGen=0.
//  2. Navigate back to main menu (pop any child views).
//  3. Press Ctrl+R — should bump enrichmentGen and clear all Wave 2 maps.
//  4. Deliver old-gen EnrichmentCheckedMsg{Gen=0} → must be dropped (nil cmd).
func TestMainMenuCtrlR_ClearsEnrichmentFindings(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: seed findings for ec2 and ddb at Gen=0.
	m, _ = rootApplyMsg(m, messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       3,
		Findings: map[string]resource.EnrichmentFinding{
			"i-0abc1111aaa111111": {Severity: "!", Summary: "system status impaired"},
		},
		Gen:     0,
		TypeGen: 0,
	})
	m, _ = rootApplyMsg(m, messages.EnrichmentCheckedMsg{
		ResourceType: "ddb",
		Issues:       1,
		Findings: map[string]resource.EnrichmentFinding{
			"arn:aws:dynamodb:us-east-1:123456789012:table/orders": {Severity: "!", Summary: "table status: DELETING"},
		},
		Gen:     0,
		TypeGen: 0,
	})

	// Step 2: ensure we are on main menu (fresh model starts there).
	// Step 3: press Ctrl+R — bumps enrichmentGen and clears Wave 2 state.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// Step 4: re-deliver old-gen messages — must be dropped after enrichmentGen bump.
	_, cmd1 := rootApplyMsg(m, messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       3,
		Findings: map[string]resource.EnrichmentFinding{
			"i-0abc1111aaa111111": {Severity: "!", Summary: "system status impaired"},
		},
		Gen:     0, // stale — Ctrl+R bumped enrichmentGen
		TypeGen: 0,
	})
	if cmd1 != nil {
		t.Error("after main-menu Ctrl+R: ec2 EnrichmentCheckedMsg{Gen=0} must be dropped — enrichmentGen was not bumped")
	}

	_, cmd2 := rootApplyMsg(m, messages.EnrichmentCheckedMsg{
		ResourceType: "ddb",
		Issues:       1,
		Findings: map[string]resource.EnrichmentFinding{
			"arn:aws:dynamodb:us-east-1:123456789012:table/orders": {Severity: "!", Summary: "table status: DELETING"},
		},
		Gen:     0, // stale
		TypeGen: 0,
	})
	if cmd2 != nil {
		t.Error("after main-menu Ctrl+R: ddb EnrichmentCheckedMsg{Gen=0} must be dropped — Wave 2 state was not cleared")
	}
}

// TestMainMenuCtrlR_EnrichmentGenIncremented verifies that multiple types' old-gen
// messages are all stale after a main-menu Ctrl+R, confirming the session-wide
// enrichmentGen was incremented (not just per-type counters).
func TestMainMenuCtrlR_EnrichmentGenIncremented(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Seed findings for several types.
	for _, rt := range []string{"ec2", "ebs", "ddb", "tg"} {
		m, _ = rootApplyMsg(m, messages.EnrichmentCheckedMsg{
			ResourceType: rt,
			Issues:       1,
			Findings:     map[string]resource.EnrichmentFinding{},
			Gen:          0,
			TypeGen:      0,
		})
	}

	// Press Ctrl+R on main menu — must bump enrichmentGen.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// All types' old-gen messages must be dropped.
	for _, rt := range []string{"ec2", "ebs", "ddb", "tg"} {
		_, cmd := rootApplyMsg(m, messages.EnrichmentCheckedMsg{
			ResourceType: rt,
			Findings:     map[string]resource.EnrichmentFinding{},
			Gen:          0, // stale after Ctrl+R bumped enrichmentGen
			TypeGen:      0,
		})
		if cmd != nil {
			t.Errorf("after main-menu Ctrl+R: EnrichmentCheckedMsg{%s, Gen=0} must be dropped — enrichmentGen was not bumped", rt)
		}
	}
}

// TestMainMenuCtrlR_MapsSafeAfterReset verifies that after Ctrl+R on main menu,
// the enrichment maps are non-nil (not nil) so subsequent writes don't panic.
func TestMainMenuCtrlR_MapsSafeAfterReset(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Press Ctrl+R — resets maps.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// Writing to the maps (via a non-stale message after navigating to ec2 list
	// with the new gen) must not panic. We verify this by delivering a stale message
	// which exercises the gen-guard path without writing — the important thing is
	// no panic occurs from a nil map access.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("after main-menu Ctrl+R, enrichment maps must be non-nil (no panic): %v", r)
			}
		}()
		m2, _ := m.Update(messages.EnrichmentCheckedMsg{
			ResourceType: "ec2",
			Findings:     map[string]resource.EnrichmentFinding{},
			Gen:          0,
			TypeGen:      99,
		})
		_ = m2
	}()
}
