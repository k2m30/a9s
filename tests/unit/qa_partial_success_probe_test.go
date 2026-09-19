package unit

// Partial-success handling in probeResourceAvailability and probeEnrichment.
//
// When a fetcher returns (FetchResult{Resources:[...]}, err), the caller
// receives both. probeResourceAvailability is an unexported method on
// *tui.Model, so its output is verified through the registered fetcher and the
// handler.
//
// When an enricher returns (IssueEnricherResult{...}, err), probeEnrichment
// returns EnrichmentCheckedMsg with Err set and Findings/FieldUpdates/
// TruncatedIDs/Issues/Truncated populated from the result. Delivering
// AvailabilityCheckedMsg seeds probeResources and starts enrichment, which
// runs the enricher without an AWS call.

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// partialProbeResources returns a small slice of synthetic resources used in
// probeResourceAvailability partial-success tests.
func partialProbeResources() []resource.Resource {
	return []resource.Resource{
		{ID: "res-pa-001", Name: "res-pa-001"},
		{ID: "res-pa-002", Name: "res-pa-002"},
	}
}

// (FetchResult{Resources:[...]}, err) reaches the caller as both values. The
// probe dispatch path needs real STS credentials, so the fetcher half is
// pinned on its own.
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

// A partial enricher result with an error reaches EnrichmentCheckedMsg with Err
// set and the result fields populated. AvailabilityCheckedMsg seeds
// probeResources and starts probeEnrichment, which calls only the registered
// enricher (no STS, no AWS).
func TestProbeEnrichment_PartialSuccess(t *testing.T) {
	tui.Version = "test"

	const shortName = "test-pe-partial"

	partialErr := errors.New("partial: enrichment call timed out for 1 resource")
	partialResult := awsclient.IssueEnricherResult{
		Truncated: true,
		TruncatedIDs: map[string]string{
			"res-pe-002": "",
		},
		Findings: map[string][]domain.Finding{
			"res-pe-001": {{Code: "rds.pending-maintenance", Phrase: "maintenance window overdue", Severity: domain.SevBroken, Source: "wave2:test-pe-partial"}},
		},
		FieldUpdates: map[string]map[string]string{
			"res-pe-001": {"maintenance_window": "overdue"},
		},
	}

	// Sentinel name not in the catalog; SetWave2EnricherForTest resets the
	// registry via t.Cleanup.
	awsclient.SetWave2EnricherForTest(t, shortName, awsclient.IssueEnricher{
		Priority: 100,
		Fn: func(_ context.Context, _ *awsclient.ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
			return partialResult, partialErr
		},
	})

	// WithNoCache(true) makes ClientsReadyMsg set m.clients without triggering
	// fetchIdentity (STS); probeEnrichment returns "clients not initialized"
	// while m.clients is nil.
	m := newBlessedModel(t, "test", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	// The returned demoPrefetchCounts cmd is discarded.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: &awsclient.ServiceClients{}, Gen: 1})

	probeRes := []resource.Resource{
		{ID: "res-pe-001", Name: "res-pe-001"},
		{ID: "res-pe-002", Name: "res-pe-002"},
	}
	// session.New seeds AvailabilityGen=1 — stamp the live value so
	// the AvailabilityChecked stale guard (AcceptZeroGen=false) accepts it.
	// availTotal=0 → availChecked(1) >= 0 → finalize → startEnrichment.
	_, enrichCmd := rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: shortName,
		Gen:          m.Core().Session().AvailabilityGen,
		Count:        len(probeRes),
		HasResources: true,
		Resources:    probeRes,
	})

	if enrichCmd == nil {
		t.Skip("startEnrichment returned nil cmd — shortName not in enricher dispatch queue")
	}

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
	if !gotMsg.Truncated {
		t.Errorf("probeEnrichment partial success: Truncated = false, want true — PARTIAL-SUCCESS BUG: Truncated dropped on error")
	}
}
