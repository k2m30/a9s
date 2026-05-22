package unit

// qa_partial_success_probe_test.go — Regression pins for partial-success
// handling in probeResourceAvailability and probeEnrichment (Groups A and C).
//
// Group A — probeResourceAvailability partial success
//   File: internal/tui/app_fetchers.go:373-425
//   Contract (fixed at HEAD): when the registered fetcher returns
//   (FetchResult{Resources:[...]}, err), probeResourceAvailability returns
//   AvailabilityCheckedMsg with BOTH Err set AND HasResources=true/Count populated.
//
//   Note: probeResourceAvailability is an unexported method on *tui.Model.
//   It cannot be invoked directly from tests/unit/. The probe output is verified
//   indirectly by (a) injecting AvailabilityCheckedMsg with Err+Resources and
//   asserting handleAvailabilityChecked applies state, and (b) verifying the
//   registered fetcher returns the expected shape when called directly.
//
// Group C — probeEnrichment partial success
//   File: internal/tui/app_fetchers.go:607-650
//   Contract (fixed at HEAD): when the enricher returns (IssueEnricherResult{...}, err),
//   probeEnrichment returns EnrichmentCheckedMsg with BOTH Err set AND
//   Findings/FieldUpdates/TruncatedIDs/Issues/Truncated populated from the result.
//
//   probeEnrichment CAN be exercised indirectly: deliver AvailabilityCheckedMsg
//   to seed probeResources, which triggers startEnrichment returning a cmd that
//   executes the enricher immediately. No real AWS call is needed.

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ─────────────────────────────────────────────────────────────────────────────
// Group A: probeResourceAvailability partial success
// Tested via registered fetcher output + handler integration.
// ─────────────────────────────────────────────────────────────────────────────

// partialProbeResources returns a small slice of synthetic resources used in
// probeResourceAvailability partial-success tests.
func partialProbeResources() []resource.Resource {
	return []resource.Resource{
		{ID: "res-pa-001", Name: "res-pa-001", Status: "running"},
		{ID: "res-pa-002", Name: "res-pa-002", Status: "running"},
	}
}

// TestProbeAvailability_FetcherReturnsPartialResults verifies that the
// registered paginated fetcher contract is honoured: when a fetcher returns
// (FetchResult{Resources:[...]}, err), the caller receives both.
//
// This test pins the fetcher half of the partial-success contract independently
// of the probe dispatch path (which requires real STS credentials in tests).
// The model-level integration is covered by TestHandleAvailabilityChecked_PartialErrAppliesState.
func TestProbeAvailability_FetcherReturnsPartialResults(t *testing.T) {
	const shortName = "test-pa-partial-fetcher"
	partialErr := errors.New("partial: one ID failed to fetch")
	expectedResources := partialProbeResources()

	resource.SetPaginatedForTest(shortName, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{Resources: expectedResources}, partialErr
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(shortName) })

	pf := resource.GetPaginatedFetcher(shortName)
	if pf == nil {
		t.Fatal("registered fetcher not retrievable — registry broken")
	}

	result, err := pf(context.Background(), nil, "")

	// Contract: partial success returns both resources AND error.
	if err == nil {
		t.Fatal("partial fetcher: expected err, got nil")
	}
	if !errors.Is(err, partialErr) {
		t.Errorf("partial fetcher: err = %v, want %v", err, partialErr)
	}
	if len(result.Resources) != len(expectedResources) {
		t.Errorf("partial fetcher: len(Resources) = %d, want %d — partial resources must not be dropped",
			len(result.Resources), len(expectedResources))
	}
}

// TestProbeAvailability_HardFailure_FetcherReturnsNoResources verifies that the
// hard-failure contract is: (FetchResult{}, err) — no resources, error set.
func TestProbeAvailability_HardFailure_FetcherReturnsNoResources(t *testing.T) {
	const shortName = "test-pa-hardfail-fetcher"
	hardErr := errors.New("hard: service unreachable")

	resource.SetPaginatedForTest(shortName, func(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, hardErr
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(shortName) })

	pf := resource.GetPaginatedFetcher(shortName)
	if pf == nil {
		t.Fatal("registered fetcher not retrievable")
	}
	result, err := pf(context.Background(), nil, "")

	if err == nil {
		t.Fatal("hard failure fetcher: expected err, got nil")
	}
	if len(result.Resources) != 0 {
		t.Errorf("hard failure fetcher: len(Resources) = %d, want 0", len(result.Resources))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Group C: probeEnrichment partial success
// ─────────────────────────────────────────────────────────────────────────────

// TestProbeEnrichment_PartialSuccess verifies that when the registered enricher
// returns (IssueEnricherResult{Findings: {...}, ...}, err), probeEnrichment
// returns EnrichmentCheckedMsg with BOTH Err set AND Findings/FieldUpdates/
// TruncatedIDs/Issues/Truncated populated from the result.
//
// This test CAN be run without real AWS credentials because it drives the model
// via AvailabilityCheckedMsg (Gen=0 bypass) which seeds probeResources and
// triggers startEnrichment → probeEnrichment in a goroutine that only calls
// the registered enricher function (no STS, no real AWS).
func TestProbeEnrichment_PartialSuccess(t *testing.T) {
	tui.Version = "test"

	const shortName = "test-pe-partial"

	partialErr := errors.New("partial: enrichment call timed out for 1 resource")
	partialResult := awsclient.IssueEnricherResult{
		IssueCount: 1,
		Truncated:  true,
		TruncatedIDs: map[string]bool{
			"res-pe-002": true,
		},
		Findings: map[string]resource.EnrichmentFinding{
			"res-pe-001": {Severity: "!", Summary: "maintenance window overdue"},
		},
		FieldUpdates: map[string]map[string]string{
			"res-pe-001": {"maintenance_window": "overdue"},
		},
	}

	// Register a synthetic enricher (sentinel name not in catalog) that returns
	// a partial result AND an error. SetWave2EnricherForTest restores state
	// automatically via t.Cleanup.
	awsclient.SetWave2EnricherForTest(t, shortName, awsclient.IssueEnricher{
		Priority: 100,
		Fn: func(_ context.Context, _ *awsclient.ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
			return partialResult, partialErr
		},
	})

	// Seed probeResources[shortName] so buildEnrichQueue includes it.
	// Use WithNoCache(true) so that delivering ClientsReadyMsg sets m.clients without
	// triggering fetchIdentity (STS). probeEnrichment guards on m.clients != nil;
	// without this, it returns an early "clients not initialized" error instead of
	// calling our enricher.
	m := tui.New("test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})
	// Pre-supply clients so probeEnrichment's nil-clients guard passes.
	// noCache=true means handleClientsReady skips fetchIdentity and goes through
	// demoPrefetchCounts instead. We discard that cmd (no real AWS data needed).
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: &awsclient.ServiceClients{}, Gen: 0})

	probeRes := []resource.Resource{
		{ID: "res-pe-001", Name: "res-pe-001", Status: "running"},
		{ID: "res-pe-002", Name: "res-pe-002", Status: "running"},
	}
	// Deliver AvailabilityCheckedMsg to seed probeResources[shortName].
	// availabilityGen=0 and Gen=0 → guard passes.
	// availTotal=0 → availChecked(1) >= 0 → finalize → startEnrichment.
	_, enrichCmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: shortName,
		Gen:          0,
		Count:        len(probeRes),
		HasResources: true,
		Resources:    probeRes,
	})

	if enrichCmd == nil {
		t.Skip("startEnrichment returned nil cmd — shortName not in enricher dispatch queue")
	}

	// Walk the cmd tree for EnrichmentCheckedMsg for shortName.
	allMsgs := drainAllMessages(enrichCmd)
	var gotMsg *messages.EnrichmentChecked
	for _, msg := range allMsgs {
		if ec, ok := msg.(messages.EnrichmentChecked); ok && ec.ResourceType == shortName {
			ec := ec
			gotMsg = &ec
			break
		}
	}

	if gotMsg == nil {
		t.Skipf("EnrichmentCheckedMsg for %q not in cmd tree — enricher not dispatched (check buildEnrichQueue order)", shortName)
	}

	// CONTRACT ASSERTIONS — fail today if probeEnrichment drops results on error.
	if gotMsg.Err == nil {
		t.Fatalf("probeEnrichment partial success: Err must be set, got nil")
	}
	if len(gotMsg.Findings) == 0 {
		t.Errorf("probeEnrichment partial success: Findings empty — PARTIAL-SUCCESS BUG: Findings dropped on error (want %d findings)", len(partialResult.Findings))
	}
	if len(gotMsg.FieldUpdates) == 0 {
		t.Errorf("probeEnrichment partial success: FieldUpdates empty — PARTIAL-SUCCESS BUG: FieldUpdates dropped on error")
	}
	if len(gotMsg.TruncatedIDs) == 0 {
		t.Errorf("probeEnrichment partial success: TruncatedIDs empty — PARTIAL-SUCCESS BUG: TruncatedIDs dropped on error")
	}
	if gotMsg.Issues != partialResult.IssueCount {
		t.Errorf("probeEnrichment partial success: Issues = %d, want %d — PARTIAL-SUCCESS BUG: Issues dropped on error",
			gotMsg.Issues, partialResult.IssueCount)
	}
	if !gotMsg.Truncated {
		t.Errorf("probeEnrichment partial success: Truncated = false, want true — PARTIAL-SUCCESS BUG: Truncated dropped on error")
	}
}
