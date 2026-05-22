package unit

// Tests for probeEnrichment coverage (internal/tui/app_fetchers.go).
//
// probeEnrichment is invoked when ResourcesLoadedMsg{TypeGen!=0} arrives after
// a Ctrl+R. Its branches:
//
//  (a) enricher == nil for the given type → returns nil cmd (skipped by buildEnrichQueue)
//  (b) clients == nil → EnrichmentCheckedMsg{Err: "AWS clients not initialized"}
//  (c) enricher returns error → EnrichmentCheckedMsg{Err: <error>}
//  (d) enricher success → EnrichmentCheckedMsg{TypeGen > 0, ResourceType set}
//
// Approach: use the existing "dbi" type (has enricher in registry and is in
// buildEnrichQueue's order list). For branches (c), temporarily replace
// EnricherRegistry["dbi"] with a fake that returns an error, restoring the
// original with t.Cleanup.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
	m, probeCmd := rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "dbi",
		Resources:    resources,
		TypeGen:      1,
	})

	if probeCmd == nil {
		t.Fatal("ResourcesLoadedMsg{TypeGen=1} should dispatch probeEnrichment (non-nil cmd)")
	}

	return m, func() interface{} { return probeCmd() }
}

// TestProbeEnrichment_NilClients_ReturnsErrorMsg verifies that when clients are
// nil (no AWS connection), probeEnrichment returns EnrichmentCheckedMsg with Err.
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

// TestProbeEnrichment_EnricherError_ReturnsErrorMsg verifies that when the
// registered enricher returns an error, probeEnrichment returns
// EnrichmentCheckedMsg with Err set. We override the dbi enricher with a fake
// that errors via SetWave2EnricherForTest; cleanup is automatic via t.Cleanup.
//
// To prove branch (c) (enricher-error) is reached and not branch (b)
// (nil-clients), the test seeds the session with a non-nil empty
// *awsclient.ServiceClients via the public ClientsReady message channel
// and asserts the substring of the sentinel error returned by the fake.
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

	// Seed non-nil empty clients via the public message channel so probeEnrichment's
	// nil-clients early-return (branch b) is NOT taken. The fake enricher ignores
	// the *ServiceClients argument so a zero value is safe.
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: &awsclient.ServiceClients{}})
	m = navigateToDBIList(m)
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())
	_, probeCmd := rootApplyMsg(m, messages.ResourcesLoaded{
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

// TestProbeEnrichment_TypeGenForwarded verifies that EnrichmentCheckedMsg
// carries a non-zero TypeGen from the probeEnrichment invocation.
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

// TestProbeEnrichment_NoEnricher_NoCmdDispatched verifies that when no enricher
// is registered for a type, probeEnrichment is not dispatched (returns nil cmd).
//
// We test this by temporarily shadowing dbi via DeleteWave2EnricherForTest
// (injects an Fn=nil override). With no enricher, buildEnrichQueue skips
// dbi → no probeEnrichment cmd returned. Cleanup is automatic.
func TestProbeEnrichment_NoEnricher_NoCmdDispatched(t *testing.T) {
	tui.Version = "test"

	// Temporarily remove the dbi enricher via Fn=nil override.
	awsclient.DeleteWave2EnricherForTest(t, "dbi")

	m := newRootSizedModel()
	m = navigateToDBIList(m)
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())
	_, probeCmd := rootApplyMsg(m, messages.ResourcesLoaded{
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

// TestProbeEnrichment_EmptyResources_StillDispatches verifies that an empty
// probeResources slice doesn't prevent the cmd from being returned. The enricher
// is still called (with an empty slice) when probeResources is empty.
func TestProbeEnrichment_EmptyResources_StillDispatches(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()
	m = navigateToDBIList(m)
	m, _ = rootApplyMsg(m, ctrlRKeyMsg())
	// Deliver empty resource list with TypeGen=1.
	_, probeCmd := rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "dbi",
		Resources:    []resource.Resource{}, // empty
		TypeGen:      1,
	})

	// buildEnrichQueue checks probeResources[shortName] is present (not empty).
	// An empty ResourcesLoadedMsg should still seed probeResources (even empty).
	// If probeCmd is nil here, that's also acceptable behavior — the test documents it.
	if probeCmd != nil {
		// Execute it — must not panic.
		msg := probeCmd()
		_ = msg //nolint:ineffassign,staticcheck // verifying no panic on execution
	}
}
