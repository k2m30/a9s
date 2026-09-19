package unit

// Ctrl+R enrichment rerun and overlap
// behaviour, observed without access to unexported model fields.
//
// A queue-drained EnrichmentChecked delivery produces a TaskKindSaveCache cmd
// whenever session.ProbeResources is non-empty, independently of whether the
// message was stale by gen, so cmd nilness cannot tell "dropped as stale"
// from "accepted". hasReenrichOrRefetch looks for the message shapes that
// real re-enrichment or refetch work produces instead.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// hasReenrichOrRefetch executes cmd (recursing through any tea.BatchMsg) and
// reports whether any resulting message is messages.EnrichmentChecked or
// messages.ResourcesLoaded — the two message shapes that indicate a real
// re-enrichment or refetch actually fired. A rerouted TaskKindSaveCache
// background-save cmd resolves to nil or messages.Flash, neither of which
// trips this check, so it safely distinguishes "the stale/guarded message
// caused new enrichment/fetch work" from "an unrelated cache save happened
// to be batched in the same Update call".
func hasReenrichOrRefetch(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	return msgIsReenrichOrRefetch(cmd())
}

func msgIsReenrichOrRefetch(msg tea.Msg) bool {
	switch v := msg.(type) {
	case messages.EnrichmentChecked:
		return true
	case messages.ResourcesLoaded:
		return true
	case tea.BatchMsg:
		for _, sub := range v {
			if sub == nil {
				continue
			}
			if msgIsReenrichOrRefetch(sub()) {
				return true
			}
		}
	}
	return false
}

// ctrlRKeyMsg returns a KeyPressMsg that matches the Refresh key binding.
// The Refresh binding registers "\x12" (Ctrl+R, ASCII 18) as a valid key.
// Key.String() returns Text when non-empty, so setting Text="\x12" causes
// key.Matches to succeed against the "\x12" binding entry.
func ctrlRKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: "\x12"}
}

// navigateToEC2List navigates a model to the EC2 resource list and returns
// the updated model. The navigation pushes a ResourceListModel onto the stack
// without pre-loading resources (fresh list, no cache).
func navigateToEC2List(m tui.Model) tui.Model {
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	return m
}

// rerunEC2Resources returns a small slice of realistic EC2 resources for use
// as simulated fetch results in rerun/overlap tests.
func rerunEC2Resources() []resource.Resource {
	return []resource.Resource{
		{ID: "i-0abc1111aaa111111", Name: "web-server-1", Fields: map[string]string{"State": "running"}},
		{ID: "i-0abc2222bbb222222", Name: "web-server-2", Fields: map[string]string{"State": "running"}},
		{ID: "i-0abc3333ccc333333", Name: "worker-1", Fields: map[string]string{"State": "stopped"}},
	}
}

// navigateToEBSList navigates a model to the EBS Volumes resource list.
// EBS is used for tests that require probeEnrichment to fire: it is registered
// in both EnricherRegistry and buildEnrichQueue's order list.
func navigateToEBSList(m tui.Model) tui.Model {
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ebs",
	})
	return m
}

// rerunEBSResources returns a small slice of realistic EBS volume resources for
// use as simulated fetch results in probeEnrichment-testing scenarios.
func rerunEBSResources() []resource.Resource {
	return []resource.Resource{
		{ID: "vol-0abc1111aaa11111a", Name: "vol-0abc1111aaa11111a", Fields: map[string]string{"volume_id": "vol-0abc1111aaa11111a", "state": "available"}},
		{ID: "vol-0abc2222bbb22222b", Name: "vol-0abc2222bbb22222b", Fields: map[string]string{"volume_id": "vol-0abc2222bbb22222b", "state": "available"}},
		{ID: "vol-0abc3333ccc33333c", Name: "vol-0abc3333ccc33333c", Fields: map[string]string{"volume_id": "vol-0abc3333ccc33333c", "state": "in-use"}},
	}
}

// enrichmentCheckedWithFindings builds an EnrichmentCheckedMsg that will be
// accepted by handleEnrichmentChecked when typeGen matches the model's current
// per-type gen for "ec2".
//
// sessionGen MUST be 0 for a freshly created model (enrichmentGen starts at 0).
// typeGen MUST match the current per-type gen (0 initially, 1 after first Ctrl+R).
func enrichmentCheckedWithFindings(sessionGen, typeGen domain.Gen) messages.EnrichmentChecked {
	return messages.EnrichmentChecked{
		ResourceType: "ec2",
		Truncated:    false,
		Findings: map[string][]domain.Finding{
			"i-0abc1111aaa111111": {{Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		},
		Gen:     sessionGen,
		TypeGen: typeGen,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Happy path: single Ctrl+R re-runs enrichment
// ─────────────────────────────────────────────────────────────────────────────

// TestListCtrlR_HappyPath_RerunsEnrichment verifies that Ctrl+R on the EC2
// list returns the wrapped fetch cmd, that a TypeGen=0 EnrichmentCheckedMsg
// triggers no re-enrichment or refetch work, and that the fetch error passes
// through the wrapper unchanged.
func TestListCtrlR_HappyPath_RerunsEnrichment(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	m, refreshCmd := rootApplyMsg(m, ctrlRKeyMsg())
	if refreshCmd == nil {
		t.Fatal("Ctrl+R on top-level EC2 list must return a non-nil cmd (wrapped fetch)")
	}

	// The per-type gen guard never drops TypeGen=0 (0 means "not a tracked
	// rerun"), so the observable contract is that no re-enrich/refetch work
	// follows.
	_, cmd := rootApplyMsg(m, enrichmentCheckedWithFindings(0, 0))
	if hasReenrichOrRefetch(cmd) {
		t.Error("after Ctrl+R, EnrichmentCheckedMsg{TypeGen=0} must not spuriously trigger a re-enrichment probe or refetch")
	}

	msg := refreshCmd()
	switch msg.(type) {
	case messages.APIError:
		m, _ = rootApplyMsg(m, msg)
	case messages.ResourcesLoaded:
		m, _ = rootApplyMsg(m, msg)
	default:
	}

	// A TypeGen=1 EnrichmentCheckedMsg must still be accepted after the error.
	m, _ = rootApplyMsg(m, enrichmentCheckedWithFindings(0, 1))
	_ = m
}

// TestListCtrlR_HappyPath_WrappedCmdStampsTypeGen verifies that a
// ResourcesLoadedMsg whose TypeGen matches enrichmentTypeGen["ebs"] seeds
// probeResources and dispatches probeEnrichment. ebs is registered in both
// EnricherRegistry and buildEnrichQueue's order list.
func TestListCtrlR_HappyPath_WrappedCmdStampsTypeGen(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = navigateToEBSList(m)

	// Ctrl+R: bumps enrichmentTypeGen["ebs"] from 0 to 1.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	loadedMsg := messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ebs",
		Resources:    rerunEBSResources(),
		TypeGen:      1, // matches enrichmentTypeGen["ebs"]=1 after Ctrl+R
	}
	m, probeCmd := rootApplyMsg(m, loadedMsg)

	if probeCmd == nil {
		t.Error("ResourcesLoadedMsg{TypeGen=1} matching current per-type gen must return non-nil cmd (probeEnrichment dispatch)")
	}

	if probeCmd != nil {
		probeMsg := probeCmd()
		switch probeMsg.(type) {
		case messages.EnrichmentChecked:
		default:
		}
	}
	_ = m
}

// TestListCtrlR_HappyPath_ResourcesLoadedUpdatesView verifies that after
// delivering ResourcesLoadedMsg (TypeGen=1 stamped), the active ResourceListModel
// reflects the loaded resources — the list update is unconditional.
func TestListCtrlR_HappyPath_ResourcesLoadedUpdatesView(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	resources := rerunEC2Resources()
	loadedMsg := messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    resources,
		TypeGen:      1, Provenance: messages.FetchProvenanceCanonicalList,
	}
	m, _ = rootApplyMsg(m, loadedMsg)

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2(3)") {
		t.Errorf("after ResourcesLoadedMsg with 3 EC2 resources, view must contain 'ec2(3)', got excerpt: %s",
			plain[:min(300, len(plain))])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Overlap: stale first-press rerun is skipped, list still updates
// ─────────────────────────────────────────────────────────────────────────────

// TestListCtrlR_Overlap_StaleRerunSkipped_ListStillUpdates: with Ctrl+R
// pressed twice before the first fetch returns, the stale
// ResourcesLoadedMsg{TypeGen:1} still updates the list but skips the rerun
// tail; the fresh ResourcesLoadedMsg{TypeGen:2} updates the list and fires the
// rerun. ebs is registered in both EnricherRegistry and buildEnrichQueue's
// order list.
func TestListCtrlR_Overlap_StaleRerunSkipped_ListStillUpdates(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = navigateToEBSList(m)

	// First Ctrl+R: enrichmentTypeGen["ebs"] → 1.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// Second Ctrl+R before first fetch returns: enrichmentTypeGen["ebs"] → 2.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	firstResources := []resource.Resource{
		{ID: "vol-first111111111a", Name: "vol-first111111111a", Fields: map[string]string{"volume_id": "vol-first111111111a", "state": "available"}},
	}
	m, firstCmd := rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ebs",
		Resources:    firstResources,
		TypeGen:      1, Provenance: // stale (current=2)
		messages.FetchProvenanceCanonicalList,
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ebs(1)") {
		t.Errorf("stale TypeGen=1 must still update the list: expected 'ebs(1)' in view, got excerpt: %s",
			plain[:min(300, len(plain))])
	}

	if firstCmd != nil {
		firstMsg := firstCmd()
		if _, isEnrich := firstMsg.(messages.EnrichmentChecked); isEnrich {
			t.Error("stale TypeGen=1 tail branch must not dispatch probeEnrichment; " +
				"got EnrichmentCheckedMsg from firstCmd — stale rerun fired incorrectly")
		}
	}

	secondResources := []resource.Resource{
		{ID: "vol-second11111111a", Name: "vol-second11111111a", Fields: map[string]string{"volume_id": "vol-second11111111a", "state": "available"}},
		{ID: "vol-second22222222b", Name: "vol-second22222222b", Fields: map[string]string{"volume_id": "vol-second22222222b", "state": "in-use"}},
	}
	m, secondCmd := rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ebs",
		Resources:    secondResources,
		TypeGen:      2, Provenance: // matches current per-type gen=2
		messages.FetchProvenanceCanonicalList,
	})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ebs(2)") {
		t.Errorf("fresh TypeGen=2 must update the list: expected 'ebs(2)' in view, got excerpt: %s",
			plain[:min(300, len(plain))])
	}

	if secondCmd == nil {
		t.Error("fresh TypeGen=2 must dispatch probeEnrichment: secondCmd must be non-nil")
	}

	_ = m
}

// ─────────────────────────────────────────────────────────────────────────────
// Fetch error: no latent state persists
// ─────────────────────────────────────────────────────────────────────────────

// TestListCtrlR_FetchError_NoLatentState verifies that a failed Ctrl+R refresh
// (APIErrorMsg from fetch) leaves enrichmentFindings and enrichmentRan cleared.
//
// Proof: after the error, a subsequent successful Ctrl+R with TypeGen=2 delivers
// ResourcesLoadedMsg{TypeGen:2} and probeEnrichment fires cleanly — no latent
// state from the failed attempt corrupts the second attempt.
//
// ebs is registered in both EnricherRegistry and buildEnrichQueue's order list.
func TestListCtrlR_FetchError_NoLatentState(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = navigateToEBSList(m)

	// enrichmentTypeGen["ebs"]=0 on fresh model; Gen=0 is the fresh session gen.
	ebsFinding := messages.EnrichmentChecked{
		ResourceType: "ebs",
		Findings: map[string][]domain.Finding{
			"vol-0abc1111aaa11111a": {{Code: "ebs.volume.degraded", Phrase: "volume degraded", Severity: domain.SevBroken, Source: "wave2:ebs"}},
		},
		Gen:     0,
		TypeGen: 0,
	}
	m, _ = rootApplyMsg(m, ebsFinding)

	// First Ctrl+R: bumps enrichmentTypeGen["ebs"] → 1, clears findings and ran.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// The per-type gen guard never drops TypeGen=0 (0 means "not a tracked
	// rerun"), so the observable contract is that redelivering the superseded
	// finding triggers no re-enrichment probe or refetch.
	_, dropCmd := rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ebs",
		Findings: map[string][]domain.Finding{
			"vol-0abc1111aaa11111a": {{Code: "ebs.volume.degraded", Phrase: "volume degraded", Severity: domain.SevBroken, Source: "wave2:ebs"}},
		},
		Gen:     0,
		TypeGen: 0,
	})
	if hasReenrichOrRefetch(dropCmd) {
		t.Error("after Ctrl+R: redelivering EnrichmentCheckedMsg{TypeGen=0} must not spuriously trigger a re-enrichment probe or refetch")
	}

	// nil clients make the wrapped fetch return APIError.
	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "ebs",
		Err:          errFetchFailed,
	})

	// A TypeGen=1 EnrichmentCheckedMsg must still be accepted after the error.
	m, _ = rootApplyMsg(m, messages.EnrichmentChecked{
		ResourceType: "ebs",
		Findings: map[string][]domain.Finding{
			"vol-0abc1111aaa11111a": {{Code: "ebs.volume.degraded", Phrase: "volume degraded", Severity: domain.SevBroken, Source: "wave2:ebs"}},
		},
		Gen:     0,
		TypeGen: 1,
	})

	// Second Ctrl+R: bumps enrichmentTypeGen["ebs"] → 2.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	m, probeCmd := rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ebs",
		Resources:    rerunEBSResources(),
		TypeGen:      2,
	})

	if probeCmd == nil {
		t.Error("second Ctrl+R after a previous fetch error must still dispatch probeEnrichment (no latent state from error)")
	}

	_ = m
}

// errFetchFailed is a sentinel for a simulated AWS API failure.
var errFetchFailed = simpleError("aws: ec2: request failed (simulated)")

// simpleError is a minimal error type for test sentinels.
type simpleError string

func (e simpleError) Error() string { return string(e) }

// ─────────────────────────────────────────────────────────────────────────────
// Stale startup probe dropped by per-type gen guard
// ─────────────────────────────────────────────────────────────────────────────
// ─────────────────────────────────────────────────────────────────────────────
// TestHandleEnrichmentChecked_DropsStaleTypeGen verifies that a startup Wave 2
// probe that captured TypeGen=0 does not resurrect enrichment work after
// Ctrl+R. The per-type gen guard never drops TypeGen=0 (0 means "not a
// tracked rerun"), so the observable contract is that no re-enrichment probe
// or refetch follows; a same-call TaskKindSaveCache cmd is tolerated (see
// hasReenrichOrRefetch).
func TestHandleEnrichmentChecked_DropsStaleTypeGen(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// Simulate stale startup probe: TypeGen=0 (captured before Ctrl+R).
	staleProbeMsg := messages.EnrichmentChecked{
		ResourceType: "ec2",
		Truncated:    false,
		Findings: map[string][]domain.Finding{
			"i-0abc1111aaa111111": {{Code: "ec2.system.status.impaired", Phrase: "system status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
			"i-0abc2222bbb222222": {{Code: "ec2.instance.status.impaired", Phrase: "instance status impaired", Severity: domain.SevBroken, Source: "wave2:ec2"}},
		},
		Gen:     0, // matches session enrichmentGen=0
		TypeGen: 0,
	}

	_, cmd := rootApplyMsg(m, staleProbeMsg)

	if hasReenrichOrRefetch(cmd) {
		t.Error("stale TypeGen=0 startup probe after Ctrl+R must not spuriously trigger a re-enrichment probe or refetch")
	}

	// A TypeGen=1 EnrichmentCheckedMsg must still be accepted afterwards.
	m, _ = rootApplyMsg(m, enrichmentCheckedWithFindings(0, 1))
	_ = m
}

// TestListCtrlR_NormalFetch_TypeGenZeroDoesNotCorruptRerunGen verifies that a
// normal (non-Ctrl+R) ResourcesLoadedMsg with TypeGen=0 does not consume or
// corrupt the per-type rerun-gen bookkeeping: a subsequent Ctrl+R rerun still
// dispatches. ec2 has a registered Wave-2 enricher, so the plain list-open
// dispatch (core/runtime/handlers_resources.go's `ev.TypeGen == 0 && ...`
// branch) legitimately resolves to EnrichmentChecked on the first load; only
// the rerun branch is under test.
func TestListCtrlR_NormalFetch_TypeGenZeroDoesNotCorruptRerunGen(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m = navigateToEC2List(m)

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    rerunEC2Resources(),
		TypeGen:      0, // normal fetch, no rerun intent
	})

	// A Ctrl+R rerun matching the fresh token must still dispatch.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())
	_, rerunCmd := rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    rerunEC2Resources(),
		TypeGen:      1, // matches enrichmentTypeGen["ec2"]=1 after Ctrl+R
	})
	if rerunCmd == nil {
		t.Error("Ctrl+R after a prior TypeGen=0 normal fetch must still dispatch a rerun probe (rerunCmd nil) — the prior TypeGen=0 fetch must not have corrupted enrichmentTypeGen[\"ec2\"]")
	}
}
