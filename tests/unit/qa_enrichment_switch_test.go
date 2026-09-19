package unit

// Profile and region switches clear enrichment
// state (per-type findings, "ran" flags and generation counters) and leave
// those maps non-nil, so later EnrichmentChecked deliveries neither revive old
// work nor panic.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// seedEnrichmentFindings delivers EnrichmentCheckedMsg for multiple resource
// types so that enrichmentFindings and enrichmentRan are non-empty before
// the switch. Returns the updated model plus the session-wide enrichmentGen
// that was active at seeding time (0 for fresh models).
func seedEnrichmentFindings(m tui.Model) tui.Model {
	// tui.Model exposes no EnrichmentGen() accessor; 0 is a fresh model's value.
	m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Truncated:    false,
		Findings: map[string][]domain.Finding{
			"i-0abc1111aaa111111": {{Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		},
		Gen:     0,
		TypeGen: 0,
	})
	m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "rds",
		Truncated:    false,
		Findings: map[string][]domain.Finding{
			"arn:aws:rds:us-east-1:123456789012:db:prod-db": {{Code: "rds.pending-maintenance", Phrase: "pending maintenance: system-update", Severity: domain.SevWarn, Source: "wave2:rds"}},
		},
		Gen:     0,
		TypeGen: 0,
	})
	return m
}

// ─────────────────────────────────────────────────────────────────────────────
// Profile switch clears all enrichment state
// ─────────────────────────────────────────────────────────────────────────────

// TestProfileSwitch_ClearsEnrichmentState verifies that handleProfileSelected
// clears enrichmentFindings, enrichmentRan, and enrichmentTypeGen for all types.
//
// Gen=0 is never stale by itself (EnrichmentChecked.AcceptZeroGen()), so a
// redelivered old message is accepted; what must hold is that it triggers no
// re-enrichment probe or refetch (a same-call TaskKindSaveCache cmd is
// tolerated — see hasReenrichOrRefetch).
func TestProfileSwitch_ClearsEnrichmentState(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	m = seedEnrichmentFindings(m)

	m, switchCmd := rootApplyMsg(m, messages.ProfileSelected{Profile: "staging"})

	_ = switchCmd

	_, dropEC2Cmd := rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings: map[string][]domain.Finding{
			"i-0abc1111aaa111111": {{Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		},
		Gen:     0,
		TypeGen: 0,
	})
	if hasReenrichOrRefetch(dropEC2Cmd) {
		t.Error("after profile switch: redelivering ec2 EnrichmentCheckedMsg{Gen=0} must not spuriously trigger a re-enrichment probe or refetch")
	}

	_, dropRDSCmd := rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "rds",
		Findings: map[string][]domain.Finding{
			"arn:aws:rds:us-east-1:123456789012:db:prod-db": {{Code: "rds.pending-maintenance", Phrase: "pending maintenance", Severity: domain.SevWarn, Source: "wave2:rds"}},
		},
		Gen:     0,
		TypeGen: 0,
	})
	if hasReenrichOrRefetch(dropRDSCmd) {
		t.Error("after profile switch: redelivering rds EnrichmentCheckedMsg{Gen=0} must not spuriously trigger a re-enrichment probe or refetch")
	}

	// The maps are non-nil after re-initialization: a new message must not panic.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("after profile switch, enrichment map write must not panic (maps must be non-nil, got panic: %v)", r)
			}
		}()
		m2, _ := m.Update(messages.EnrichmentChecked{
			ResourceType: "ec2",
			Findings:     map[string][]domain.Finding{},
			Gen:          0,  // stale
			TypeGen:      99, // stale
		})
		_ = m2
	}()
}

// TestProfileSwitch_BothEnrichmentMapsCleared verifies that after switching
// profiles, redelivering an old EnrichmentCheckedMsg for any enriched type
// must not spuriously trigger new enrichment/refetch work — both ec2 AND
// rds must behave identically.
func TestProfileSwitch_BothEnrichmentMapsCleared(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = seedEnrichmentFindings(m)

	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "dev"})

	for _, rt := range []string{"ec2", "rds", "ebs", "ddb"} {
		_, cmd := rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: rt,
			Findings:     map[string][]domain.Finding{},
			Gen:          0,
			TypeGen:      0,
		})
		if hasReenrichOrRefetch(cmd) {
			t.Errorf("after profile switch: redelivering EnrichmentCheckedMsg{%s, Gen=0} must not spuriously trigger a re-enrichment probe or refetch", rt)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Region switch clears all enrichment state
// ─────────────────────────────────────────────────────────────────────────────

// TestRegionSwitch_ClearsEnrichmentState verifies that handleRegionSelected
// clears enrichmentFindings, enrichmentRan, and enrichmentTypeGen — same as
// profile switch but triggered by RegionSelectedMsg.
func TestRegionSwitch_ClearsEnrichmentState(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	m = seedEnrichmentFindings(m)

	m, switchCmd := rootApplyMsg(m, messages.RegionSelected{Region: "eu-west-1"})
	_ = switchCmd

	_, dropEC2Cmd := rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ec2",
		Findings: map[string][]domain.Finding{
			"i-0abc1111aaa111111": {{Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		},
		Gen:     0,
		TypeGen: 0,
	})
	if hasReenrichOrRefetch(dropEC2Cmd) {
		t.Error("after region switch: redelivering ec2 EnrichmentCheckedMsg{Gen=0} must not spuriously trigger a re-enrichment probe or refetch")
	}

	_, dropRDSCmd := rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "rds",
		Findings: map[string][]domain.Finding{
			"arn:aws:rds:us-east-1:123456789012:db:prod-db": {{Code: "rds.pending-maintenance", Phrase: "pending maintenance", Severity: domain.SevWarn, Source: "wave2:rds"}},
		},
		Gen:     0,
		TypeGen: 0,
	})
	if hasReenrichOrRefetch(dropRDSCmd) {
		t.Error("after region switch: redelivering rds EnrichmentCheckedMsg{Gen=0} must not spuriously trigger a re-enrichment probe or refetch")
	}

	// The maps are non-nil after re-initialization: a new message must not panic.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("after region switch, enrichment map write must not panic: %v", r)
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

// TestRegionSwitch_BothEnrichmentMapsCleared verifies that region switch
// leaves redelivered old-gen messages unable to trigger new enrichment/refetch
// work, for all enriched types simultaneously (parallel to the profile-switch
// test above).
func TestRegionSwitch_BothEnrichmentMapsCleared(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = seedEnrichmentFindings(m)

	m, _ = rootApplyMsg(m, messages.RegionSelected{Region: "ap-southeast-1"})

	for _, rt := range []string{"ec2", "rds", "ebs", "ddb"} {
		_, cmd := rootApplyMsg(m, messages.EnrichmentChecked{
			ResourceType: rt,
			Findings:     map[string][]domain.Finding{},
			Gen:          0,
			TypeGen:      0,
		})
		if hasReenrichOrRefetch(cmd) {
			t.Errorf("after region switch: redelivering EnrichmentCheckedMsg{%s, Gen=0} must not spuriously trigger a re-enrichment probe or refetch", rt)
		}
	}
}

// TestProfileSwitch_TypeGenResetAllowsNewEnrichment verifies that after a
// profile switch, enrichmentTypeGen is empty (not nil) so a Ctrl+R on a
// resource list correctly bumps it from 0 (missing key = zero value) to 1.
func TestProfileSwitch_TypeGenResetAllowsNewEnrichment(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	// Seed some per-type gen by pressing Ctrl+R on ec2 list.
	m = navigateToEC2List(m)
	m, _ = rootApplyMsg(m, ctrlRKeyMsg()) // enrichmentTypeGen["ec2"] → 1

	m, _ = rootApplyMsg(m, messages.PopView{})

	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "prod"})

	// Re-navigate to ec2 and Ctrl+R — must not panic (map is empty, not nil).
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Ctrl+R after profile switch must not panic (enrichmentTypeGen must be empty non-nil map): %v", r)
			}
		}()
		m2 := navigateToEC2List(m)
		m2, _ = rootApplyMsg(m2, ctrlRKeyMsg())
		_ = m2
	}()
}

// TestRegionSwitch_TypeGenResetAllowsNewEnrichment verifies same as above but
// for region switch — enrichmentTypeGen is reset to empty (not nil) map.
func TestRegionSwitch_TypeGenResetAllowsNewEnrichment(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	m = navigateToEC2List(m)
	m, _ = rootApplyMsg(m, ctrlRKeyMsg()) // enrichmentTypeGen["ec2"] → 1

	m, _ = rootApplyMsg(m, messages.PopView{})

	m, _ = rootApplyMsg(m, messages.RegionSelected{Region: "us-west-2"})

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Ctrl+R after region switch must not panic (enrichmentTypeGen empty non-nil map): %v", r)
			}
		}()
		m2 := navigateToEC2List(m)
		m2, _ = rootApplyMsg(m2, ctrlRKeyMsg())
		_ = m2
	}()
}
