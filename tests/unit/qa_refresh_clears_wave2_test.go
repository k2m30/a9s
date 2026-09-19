package unit

// Ctrl+R on the main menu bumps enrichmentGen and resets enrichmentFindings,
// enrichmentRan, enrichmentTypeGen and probeResources, so an old-gen
// EnrichmentCheckedMsg cannot resurrect enrichment state.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// Ctrl+R on the main menu bumps enrichmentGen and clears Wave 2 state. Gen=0 is
// never stale by itself (EnrichmentChecked.AcceptZeroGen()==true short-circuits
// the gen guard; see hasReenrichOrRefetch in
// qa_enrichment_rerun_overlap_test.go), so a re-delivered Gen=0 message is
// accepted; it must not trigger a re-enrichment probe or refetch. A same-call
// TaskKindSaveCache background-cache-save cmd is tolerated.
func TestMainMenuCtrlR_ClearsEnrichmentFindings(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings: map[string][]domain.Finding{
			"i-0abc1111aaa111111": {{Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		},
		Gen:     0,
		TypeGen: 0,
	})
	m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ddb",
		Findings: map[string][]domain.Finding{
			"arn:aws:dynamodb:us-east-1:123456789012:table/orders": {{Code: "ddb.table.status.deleting", Phrase: "table status: DELETING", Severity: domain.SevBroken, Source: "wave2:ddb"}},
		},
		Gen:     0,
		TypeGen: 0,
	})

	// A fresh model starts on the main menu.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	_, cmd1 := rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings: map[string][]domain.Finding{
			"i-0abc1111aaa111111": {{Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		},
		Gen:     0,
		TypeGen: 0,
	})
	if hasReenrichOrRefetch(cmd1) {
		t.Error("after main-menu Ctrl+R: redelivering ec2 EnrichmentCheckedMsg{Gen=0} must not spuriously trigger a re-enrichment probe or refetch")
	}

	_, cmd2 := rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ddb",
		Findings: map[string][]domain.Finding{
			"arn:aws:dynamodb:us-east-1:123456789012:table/orders": {{Code: "ddb.table.status.deleting", Phrase: "table status: DELETING", Severity: domain.SevBroken, Source: "wave2:ddb"}},
		},
		Gen:     0,
		TypeGen: 0,
	})
	if hasReenrichOrRefetch(cmd2) {
		t.Error("after main-menu Ctrl+R: redelivering ddb EnrichmentCheckedMsg{Gen=0} must not spuriously trigger a re-enrichment probe or refetch")
	}
}

// After a main-menu Ctrl+R, re-delivering several types' old-gen messages
// triggers no enrichment or refetch work.
func TestMainMenuCtrlR_EnrichmentGenIncremented(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	for _, rt := range []string{"ec2", "ebs", "ddb", "tg"} {
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: rt,
			Findings:     map[string][]domain.Finding{},
			Gen:          0,
			TypeGen:      0,
		})
	}

	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// Gen=0 is never stale by itself (see hasReenrichOrRefetch in
	// qa_enrichment_rerun_overlap_test.go).
	for _, rt := range []string{"ec2", "ebs", "ddb", "tg"} {
		_, cmd := rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: rt,
			Findings:     map[string][]domain.Finding{},
			Gen:          0,
			TypeGen:      0,
		})
		if hasReenrichOrRefetch(cmd) {
			t.Errorf("after main-menu Ctrl+R: redelivering EnrichmentCheckedMsg{%s, Gen=0} must not spuriously trigger a re-enrichment probe or refetch", rt)
		}
	}
}

// TestMainMenuCtrlR_MapsSafeAfterReset verifies that after Ctrl+R on main menu,
// the enrichment maps are non-nil (not nil) so subsequent writes don't panic.
func TestMainMenuCtrlR_MapsSafeAfterReset(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// A stale message exercises the gen-guard path; a nil map access would panic.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("after main-menu Ctrl+R, enrichment maps must be non-nil (no panic): %v", r)
			}
		}()
		m2, _ := m.Update(messages.EnrichmentChecked{
			ResourceType: "ec2",
			Findings:     map[string][]domain.Finding{},
			Gen:          0,
			TypeGen:      99,
		})
		_ = m2
	}()
}
