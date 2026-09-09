package unit

// runtime_probe_enrichment_test.go — live-seam replacement for the retired
// internal/tui/probe_demo_guard_test.go and internal/tui/probe_enrichment_cache_test.go.
//
// Model.probeEnrichment (internal/tui/probe_adapter.go) was a thin tea.Cmd
// wrapper with zero production callers; the only reachable enrichment
// dispatch path is Core.ProbeEnrichment (core/runtime/probes.go), which does
// not branch on demo vs. live mode at all — demo clients are real
// *awsclient.ServiceClients backed by typed fakes, so there is nothing left
// to distinguish once the TUI wrapper is gone. These tests pin
// Core.ProbeEnrichment directly.
//
// Assertions target Core.ProbeEnrichment's RETURNED ProbeEnrichmentResult
// rather than spying on the registered enricher's invocation count or the
// cache it was handed — the registered test enricher returns a sentinel
// IssueEnricherResult (conditionally, for the cache-merge test), and the
// test proves the seam by checking that sentinel surfaces unchanged through
// ProbeEnrichment's return value.

import (
	"context"
	"reflect"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestCoreProbeEnrichment_DispatchesRegisteredEnricher verifies that
// Core.ProbeEnrichment returns the registered Wave-2 enricher's result
// unchanged: every field of the returned ProbeEnrichmentResult must carry
// the exact sentinel values the test enricher produced.
func TestCoreProbeEnrichment_DispatchesRegisteredEnricher(t *testing.T) {
	const sentinelType = "dbi-snap-probe-demo-guard-pin"

	sentinelFindings := map[string][]domain.Finding{
		"sentinel-res-1": {{
			Code: "dbi-snap.sentinel-code", Phrase: "sentinel phrase", Detail: "sentinel detail",
			Severity: domain.SevBroken, Source: "wave2:" + sentinelType,
		}},
	}
	sentinelAttentionDetails := map[string]map[domain.FindingCode]domain.AttentionDetail{
		"sentinel-res-1": {
			"dbi-snap.sentinel-code": {Rows: []domain.DetailRow{{Label: "sentinel-label", Value: "sentinel-value"}}},
		},
	}
	sentinelFieldUpdates := map[string]map[string]string{
		"sentinel-res-1": {"sentinel-field": "sentinel-value"},
	}
	sentinelTruncatedIDs := map[string]string{"sentinel-res-1": ""}

	sentinelResult := awsclient.IssueEnricherResult{
		Truncated:        true,
		TruncatedIDs:     sentinelTruncatedIDs,
		Findings:         sentinelFindings,
		AttentionDetails: sentinelAttentionDetails,
		FieldUpdates:     sentinelFieldUpdates,
	}
	sentinelFn := func(_ context.Context, _ *awsclient.ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		return sentinelResult, nil
	}
	awsclient.SetWave2EnricherForTest(t, sentinelType, awsclient.IssueEnricher{Fn: sentinelFn, Priority: 100})

	sess := session.New()
	sess.Clients = &awsclient.ServiceClients{}
	core := runtime.New(sess, catalog.All())

	result := core.ProbeEnrichment(context.Background(), sess.Clients, sentinelType)

	if result.ResourceType != sentinelType {
		t.Errorf("ProbeEnrichment(...).ResourceType = %q; want %q", result.ResourceType, sentinelType)
	}
	if result.Err != nil {
		t.Fatalf("ProbeEnrichment(...).Err = %v; want nil", result.Err)
	}
	if result.Truncated != sentinelResult.Truncated {
		t.Errorf("ProbeEnrichment(...).Truncated = %v; want sentinel %v", result.Truncated, sentinelResult.Truncated)
	}
	if !reflect.DeepEqual(result.Findings, sentinelFindings) {
		t.Errorf("ProbeEnrichment(...).Findings = %#v; want sentinel %#v", result.Findings, sentinelFindings)
	}
	if !reflect.DeepEqual(result.AttentionDetails, sentinelAttentionDetails) {
		t.Errorf("ProbeEnrichment(...).AttentionDetails = %#v; want sentinel %#v", result.AttentionDetails, sentinelAttentionDetails)
	}
	if !reflect.DeepEqual(result.FieldUpdates, sentinelFieldUpdates) {
		t.Errorf("ProbeEnrichment(...).FieldUpdates = %#v; want sentinel %#v", result.FieldUpdates, sentinelFieldUpdates)
	}
	if !reflect.DeepEqual(result.TruncatedIDs, sentinelTruncatedIDs) {
		t.Errorf("ProbeEnrichment(...).TruncatedIDs = %#v; want sentinel %#v", result.TruncatedIDs, sentinelTruncatedIDs)
	}
}

// TestCoreProbeEnrichment_NilClients_ReturnsErr verifies the nil-clients
// guard: ProbeEnrichment must set Err and must not surface the registered
// enricher's sentinel result (the guard short-circuits before dispatch).
func TestCoreProbeEnrichment_NilClients_ReturnsErr(t *testing.T) {
	const sentinelType = "dbi-snap-probe-demo-guard-prod-pin"

	sentinelFn := func(_ context.Context, _ *awsclient.ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		return awsclient.IssueEnricherResult{
			Findings: map[string][]domain.Finding{
				"sentinel-res": {{
					Code: "guard-must-not-reach-enricher", Phrase: "sentinel leaked",
					Severity: domain.SevBroken, Source: "wave2:" + sentinelType,
				}},
			},
		}, nil
	}
	awsclient.SetWave2EnricherForTest(t, sentinelType, awsclient.IssueEnricher{Fn: sentinelFn, Priority: 100})

	core := runtime.New(session.New(), catalog.All())

	result := core.ProbeEnrichment(context.Background(), nil, sentinelType)
	if result.Err == nil {
		t.Fatalf("ProbeEnrichment(...) with nil clients should set Err, got nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("ProbeEnrichment(...).Findings with nil clients = %#v, want empty (guard must short-circuit before the sentinel enricher runs)", result.Findings)
	}
}

// TestCoreProbeEnrichment_CacheSnapshotMergesRowStore pins the contract that
// ProbeEnrichment's BuildResourceCacheSnapshot merges RowStore's retained
// sibling rows into the cache the enricher sees, even when no list has been
// opened yet (Codex P1 regression — see Core.ProbeEnrichment's doc comment).
//
// The registered test enricher returns one sentinel Finding Code when
// cache["dbi"] carries exactly the RowStore sibling row, and a different
// sentinel Code otherwise — so the assertion below (checking the RETURNED
// ProbeEnrichmentResult.Findings) can only pass if ProbeEnrichment actually
// built and passed the merged cache snapshot to the enricher.
func TestCoreProbeEnrichment_CacheSnapshotMergesRowStore(t *testing.T) {
	const sentinelType = "dbi-snap-probe-cache-pin"
	const cacheMergedCode = domain.FindingCode("cache-merged")
	const cacheMissingOrWrongCode = domain.FindingCode("cache-missing-or-wrong")

	sentinelFn := func(_ context.Context, _ *awsclient.ServiceClients, _ []resource.Resource, cache resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		dbiEntry, ok := cache["dbi"]
		code := cacheMissingOrWrongCode
		if ok && len(dbiEntry.Resources) == 1 && dbiEntry.Resources[0].ID == "prod-dbi-1" {
			code = cacheMergedCode
		}
		return awsclient.IssueEnricherResult{
			Findings: map[string][]domain.Finding{
				"sentinel-res": {{Code: code, Severity: domain.SevBroken, Source: "wave2:" + sentinelType}},
			},
		}, nil
	}
	awsclient.SetWave2EnricherForTest(t, sentinelType, awsclient.IssueEnricher{Fn: sentinelFn, Priority: 100})

	sess := session.New()
	sess.Clients = &awsclient.ServiceClients{}
	core := runtime.New(sess, catalog.All())
	core.Session().RowStore.Observe("dbi", []resource.Resource{
		{ID: "prod-dbi-1", Name: "prod-dbi-1"},
	}, nil, session.OriginProbe, false)
	core.Session().RowStore.Observe(sentinelType, []resource.Resource{
		{ID: "rds:test-snap"},
	}, nil, session.OriginProbe, false)

	result := core.ProbeEnrichment(context.Background(), sess.Clients, sentinelType)
	if result.Err != nil {
		t.Fatalf("ProbeEnrichment(...).Err = %v; want nil", result.Err)
	}
	fs, ok := result.Findings["sentinel-res"]
	if !ok || len(fs) == 0 {
		t.Fatalf("ProbeEnrichment(...).Findings missing entry for \"sentinel-res\"")
	}
	if fs[0].Code != cacheMergedCode {
		t.Errorf("ProbeEnrichment(...).Findings[sentinel-res][0].Code = %q, want %q — the sentinel enricher only returns %q when cache[dbi] carries the RowStore sibling row.\n"+
			"This is the Codex P1 regression: ProbeEnrichment must merge RowStore's retained rows into the cache snapshot, "+
			"not just ResourceCache (which is empty until the user opens a list).", fs[0].Code, cacheMergedCode, cacheMergedCode)
	}
}
