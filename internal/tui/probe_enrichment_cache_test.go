package tui

// probe_enrichment_cache_test.go — pin that probeEnrichment passes a cache
// snapshot containing m.probeResources to the enricher closure, even when
// m.resourceCache is empty.
//
// Codex review (2026-04-25): the cross-ref enricher (e.g. rds-snap orphan
// detection) cannot fire on the initial enrichment pass if the cache snapshot
// is built from m.resourceCache alone, because that map is empty until the
// user opens a list. probeResources holds the first-page rows retained by the
// availability probe — it MUST be merged in. The fix is to call
// m.buildResourceCacheSnapshot() (which merges all three sources) instead of
// rolling an inline view of m.resourceCache only.
//
// This test exercises the bug shape directly: register a capture-only enricher
// for a sentinel resource type, populate m.probeResources["dbi"] with a sibling
// row, leave m.resourceCache empty, dispatch probeEnrichment, and assert the
// enricher saw "dbi" in its cache argument.

import (
	"context"
	"sync"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestProbeEnrichment_CacheSnapshotMergesProbeResources pins the contract
// that probeEnrichment's cache snapshot includes m.probeResources, not just
// m.resourceCache. Pre-fix this test FAILS because the inline snapshot at
// app_probes.go:344-353 builds only from m.resourceCache.
func TestProbeEnrichment_CacheSnapshotMergesProbeResources(t *testing.T) {
	const sentinelType = "rds-snap-probe-cache-pin"

	// Capture the cache the enricher saw on its single invocation.
	var (
		mu          sync.Mutex
		seenCache   resource.ResourceCache
		invocations int
	)
	captureFn := func(_ context.Context, _ *awsclient.ServiceClients, _ []resource.Resource, cache resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		mu.Lock()
		seenCache = cache
		invocations++
		mu.Unlock()
		return awsclient.IssueEnricherResult{}, nil
	}

	// Stash and restore registry entry so the test doesn't pollute global state.
	prev, hadPrev := awsclient.IssueEnricherRegistry[sentinelType]
	awsclient.IssueEnricherRegistry[sentinelType] = awsclient.IssueEnricher{Fn: captureFn, Priority: 100}
	t.Cleanup(func() {
		if hadPrev {
			awsclient.IssueEnricherRegistry[sentinelType] = prev
		} else {
			delete(awsclient.IssueEnricherRegistry, sentinelType)
		}
	})

	// Construct a Model where probeResources has a sibling list ("dbi") but
	// resourceCache is empty — this models the initial-menu-enrichment state.
	// resourceCache / probeResources / enrichmentTypeGen live on the embedded
	// sessionRuntime; newSessionRuntime() initialises the required maps.
	m := &Model{
		sessionRuntime: newSessionRuntime(),
		appCtx:         context.Background(),
		clients:        &awsclient.ServiceClients{}, // non-nil so closure passes the nil-check
	}
	m.probeResources = map[string][]resource.Resource{
		"dbi": {
			{ID: "prod-dbi-1", Name: "prod-dbi-1"},
		},
		sentinelType: {
			{ID: "rds:test-snap"},
		},
	}

	cmd := m.probeEnrichment(sentinelType, 1)
	if cmd == nil {
		t.Fatalf("probeEnrichment returned nil cmd")
	}
	_ = cmd() // execute the closure synchronously; the captureFn writes seenCache.

	mu.Lock()
	defer mu.Unlock()

	if invocations != 1 {
		t.Fatalf("enricher invocations = %d, want 1", invocations)
	}
	if seenCache == nil {
		t.Fatalf("enricher saw nil cache; want a snapshot containing %q from probeResources", "dbi")
	}
	dbiEntry, ok := seenCache["dbi"]
	if !ok {
		t.Errorf("cache snapshot missing %q — enricher cannot run cross-ref signals against probeResources-only siblings.\n"+
			"This is the Codex P1 regression: probeEnrichment must merge m.probeResources into the cache snapshot, "+
			"not just m.resourceCache (which is empty until the user opens a list).", "dbi")
	}
	if len(dbiEntry.Resources) != 1 || dbiEntry.Resources[0].ID != "prod-dbi-1" {
		t.Errorf("cache[dbi].Resources = %v, want exactly the probeResources sibling row", dbiEntry.Resources)
	}
}
