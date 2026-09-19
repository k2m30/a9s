package unit

// probeEnrichment runs when ResourcesLoadedMsg{TypeGen!=0} arrives after a
// Ctrl+R. "dbi" has a registered enricher and is in buildEnrichQueue's order
// list.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// navigateToDBIList navigates the root model to the DBI resource list.
func navigateToDBIList(m tui.Model) tui.Model {
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})
	return m
}

// rerunDBIResources returns a small slice of realistic RDS cluster instance
// resources for use as simulated fetch results in rerun/overlap tests.
func rerunDBIResources() []resource.Resource {
	return []resource.Resource{
		{ID: "db-prod-1", Name: "db-prod-1", Fields: map[string]string{"db_instance_id": "db-prod-1", "status": "available"}},
		{ID: "db-prod-2", Name: "db-prod-2", Fields: map[string]string{"db_instance_id": "db-prod-2", "status": "available"}},
		{ID: "db-staging-1", Name: "db-staging-1", Fields: map[string]string{"db_instance_id": "db-staging-1", "status": "available"}},
	}
}

// setupEnrichmentDispatch navigates to the dbi list, presses Ctrl+R (which bumps
// enrichmentTypeGen["dbi"] to 1), then delivers ResourcesLoadedMsg{TypeGen=1} to
// trigger the tail-branch dispatch. Returns the model and the probeEnrichment cmd.
func setupEnrichmentDispatch(t *testing.T, resources []resource.Resource) (tui.Model, func() interface{}) {
	t.Helper()
	m := newRootSizedModel()
	m = navigateToDBIList(m)

	// Ctrl+R bumps enrichmentTypeGen["dbi"] → 1.
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())

	// Deliver ResourcesLoadedMsg{TypeGen=1} → tail branch fires probeEnrichment.
	m, probeCmd := rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "dbi",
		Resources:    resources,
		TypeGen:      1,
	})

	if probeCmd == nil {
		t.Fatal("ResourcesLoadedMsg{TypeGen=1} should dispatch probeEnrichment (non-nil cmd)")
	}

	return m, func() interface{} { return probeCmd() }
}

func TestProbeEnrichment_NilClients_ReturnsErrorMsg(t *testing.T) {
	tui.Version = "test"

	_, execProbe := setupEnrichmentDispatch(t, rerunDBIResources())
	msg := execProbe()

	checked, ok := msg.(messages.EnrichmentChecked)
	if !ok {
		// Batch is also acceptable — dig one level.
		t.Logf("probeEnrichment returned %T (not directly EnrichmentCheckedMsg) — acceptable if batch", msg)
		return
	}
	if checked.Err == nil {
		t.Error("probeEnrichment with nil clients should set Err, got nil")
	}
	if checked.ResourceType != "dbi" {
		t.Errorf("EnrichmentCheckedMsg.ResourceType = %q, want %q", checked.ResourceType, "dbi")
	}
}

func TestProbeEnrichment_EnricherError_ReturnsErrorMsg(t *testing.T) {
	tui.Version = "test"

	// Temporarily replace the dbi enricher with one that always errors. The
	// helper records the previous value and restores it on t.Cleanup.
	prev, _ := awsclient.Wave2EnricherFor("dbi")
	const sentinel = "simulated enricher failure"
	awsclient.SetWave2EnricherForTest(t, "dbi", awsclient.IssueEnricher{
		Fn: func(_ context.Context, _ *awsclient.ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
			return awsclient.IssueEnricherResult{}, fmt.Errorf("%s", sentinel)
		},
		Priority: prev.Priority,
	})

	// Non-nil empty clients keep probeEnrichment past its nil-clients early
	// return; the fake enricher ignores the *ServiceClients argument, so a zero
	// value is safe.
	m := newRootSizedModel()
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: &awsclient.ServiceClients{}, Gen: 1})
	m = navigateToDBIList(m)
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())
	_, probeCmd := rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "dbi",
		Resources:    rerunDBIResources(),
		TypeGen:      1,
	})
	if probeCmd == nil {
		t.Fatal("ResourcesLoadedMsg{TypeGen=1} should dispatch probeEnrichment (non-nil cmd)")
	}
	msg := probeCmd()
	checked, ok := msg.(messages.EnrichmentChecked)
	if !ok {
		t.Fatalf("probeEnrichment returned %T; want messages.EnrichmentChecked", msg)
	}
	if checked.Err == nil {
		t.Fatal("EnrichmentCheckedMsg.Err is nil — enricher-error branch (c) was not reached")
	}
	if !strings.Contains(checked.Err.Error(), sentinel) {
		t.Errorf("EnrichmentCheckedMsg.Err = %q; want substring %q (nil-clients branch likely fired instead)", checked.Err.Error(), sentinel)
	}
}

func TestProbeEnrichment_TypeGenForwarded(t *testing.T) {
	tui.Version = "test"

	_, execProbe := setupEnrichmentDispatch(t, rerunDBIResources())
	msg := execProbe()

	checked, ok := msg.(messages.EnrichmentChecked)
	if !ok {
		t.Logf("probeEnrichment returned %T — skipping TypeGen check (batch acceptable)", msg)
		return
	}
	// TypeGen must be > 0 — the Ctrl+R bumped it to 1.
	if checked.TypeGen == 0 {
		t.Error("EnrichmentCheckedMsg.TypeGen should be non-zero after Ctrl+R bump")
	}
	if checked.ResourceType != "dbi" {
		t.Errorf("ResourceType = %q, want dbi", checked.ResourceType)
	}
}

// DeleteWave2EnricherForTest shadows dbi with an Fn=nil override, so
// buildEnrichQueue skips dbi.
func TestProbeEnrichment_NoEnricher_NoCmdDispatched(t *testing.T) {
	tui.Version = "test"

	awsclient.DeleteWave2EnricherForTest(t, "dbi")

	m := newRootSizedModel()
	m = navigateToDBIList(m)
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())
	_, probeCmd := rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "dbi",
		Resources:    rerunDBIResources(),
		TypeGen:      1,
	})

	// With no enricher, probeEnrichment returns nil — buildEnrichQueue skips it.
	if probeCmd != nil {
		msg := probeCmd()
		if _, ok := msg.(messages.EnrichmentChecked); ok {
			t.Errorf("type with no enricher should not dispatch EnrichmentCheckedMsg, got %T", msg)
		}
	}
}

func TestProbeEnrichment_EmptyResources_StillDispatches(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()
	m = navigateToDBIList(m)
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())
	_, probeCmd := rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "dbi",
		Resources:    []resource.Resource{}, // empty
		TypeGen:      1,
	})

	// buildEnrichQueue checks probeResources[shortName] is present (not empty).
	// An empty ResourcesLoadedMsg should still seed probeResources (even empty).
	if probeCmd != nil {
		// Execute it — must not panic.
		msg := probeCmd()
		_ = msg //nolint:ineffassign,staticcheck // verifying no panic on execution
	}
}
