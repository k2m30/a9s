// SPDX-License-Identifier: GPL-3.0-or-later

package tui

// probe_enrichment_cache_test.go — pin that probeEnrichment passes a cache
// snapshot containing m.core.Session().RowStore's retained rows to the
// enricher closure, even when m.core.Session().ResourceCache is empty.
//
// Codex review (2026-04-25): the cross-ref enricher (e.g. dbi-snap orphan
// detection) cannot fire on the initial enrichment pass if the cache snapshot
// is built from m.core.Session().ResourceCache alone, because that map is empty until the
// user opens a list. RowStore (task #17 wave 1 stage 2 — replaces the removed
// session.ProbeResources/ProbeTruncated maps) holds the first-page rows
// retained by the availability probe — it MUST be merged in. The fix is to
// call m.buildResourceCacheSnapshot() (which merges all three sources)
// instead of rolling an inline view of m.core.Session().ResourceCache only.
//
// This test exercises the bug shape directly: register a capture-only enricher
// for a sentinel resource type, populate m.core.Session().RowStore's "dbi" entry with a
// sibling row (OriginProbe), leave m.core.Session().ResourceCache empty, dispatch
// probeEnrichment, and assert the enricher saw "dbi" in its cache argument.

import (
	"context"
	"sync"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestProbeEnrichment_CacheSnapshotMergesProbeResources pins the contract
// that probeEnrichment's cache snapshot includes m.core.Session().RowStore's
// retained rows, not just m.core.Session().ResourceCache. Pre-fix this test
// FAILS because the inline snapshot at app_probes.go:344-353 builds only from
// m.core.Session().ResourceCache.
func TestProbeEnrichment_CacheSnapshotMergesProbeResources(t *testing.T) {
	const sentinelType = "dbi-snap-probe-cache-pin"

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

	awsclient.SetWave2EnricherForTest(t, sentinelType, awsclient.IssueEnricher{Fn: captureFn, Priority: 100})

	// Construct a Model where RowStore has a sibling list ("dbi") but
	// ResourceCache is empty — this models the initial-menu-enrichment state.
	// ResourceCache / RowStore / EnrichmentTypeGen live on the session
	// owned by core; session.New() initialises the required maps/store. The
	// runtime.Core is bound to the session so the probe_adapter delegate
	// (m.core.ProbeEnrichment) sees the seeded RowStore rows without a nil
	// receiver panic.
	sess := session.New()
	sess.Clients = &awsclient.ServiceClients{} // non-nil so closure passes the nil-check
	m := &Model{
		core:   runtime.New(sess, catalog.All()),
		appCtx: context.Background(),
	}
	m.core.Session().RowStore.Observe("dbi", []resource.Resource{
		{ID: "prod-dbi-1", Name: "prod-dbi-1"},
	}, nil, session.OriginProbe, false)
	m.core.Session().RowStore.Observe(sentinelType, []resource.Resource{
		{ID: "rds:test-snap"},
	}, nil, session.OriginProbe, false)

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
		t.Fatalf("enricher saw nil cache; want a snapshot containing %q from RowStore", "dbi")
	}
	dbiEntry, ok := seenCache["dbi"]
	if !ok {
		t.Errorf("cache snapshot missing %q — enricher cannot run cross-ref signals against RowStore-only siblings.\n"+
			"This is the Codex P1 regression: probeEnrichment must merge m.core.Session().RowStore's retained rows into the cache snapshot, "+
			"not just m.core.Session().ResourceCache (which is empty until the user opens a list).", "dbi")
	}
	if len(dbiEntry.Resources) != 1 || dbiEntry.Resources[0].ID != "prod-dbi-1" {
		t.Errorf("cache[dbi].Resources = %v, want exactly the RowStore sibling row", dbiEntry.Resources)
	}
}
