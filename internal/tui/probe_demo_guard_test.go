package tui

// probe_demo_guard_test.go — AS-658 / AS-648-h3 P2.
//
// Pins the contract that `Model.probeEnrichment` treats demo mode exactly like
// live mode: demo clients are real *awsclient.ServiceClients backed by typed
// fakes (internal/demo.NewServiceClients), so Wave-2 enrichers dispatch
// against them the same way they dispatch against live AWS clients. There is
// no demo-mode skip, and dispatch stays lazy — the enricher Fn runs only when
// the returned tea.Cmd executes, never at arm time.

import (
	"context"
	"sync/atomic"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestProbeEnrichment_DemoMode_DispatchesLikeLive verifies that when
// `m.isDemo == true`, probeEnrichment for an enricher-bearing type returns a
// real non-nil tea.Cmd whose closure invokes the registered Wave-2 enricher
// Fn exactly once and yields an EnrichmentChecked message — identical to the
// live path. The Fn must not run before the cmd executes.
func TestProbeEnrichment_DemoMode_DispatchesLikeLive(t *testing.T) {
	const sentinelType = "dbi-snap-probe-demo-guard-pin"

	var fnCalls int32
	captureFn := func(_ context.Context, _ *awsclient.ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		atomic.AddInt32(&fnCalls, 1)
		return awsclient.IssueEnricherResult{}, nil
	}

	awsclient.SetWave2EnricherForTest(t, sentinelType, awsclient.IssueEnricher{Fn: captureFn, Priority: 100})

	sess := session.New()
	sess.Clients = &awsclient.ServiceClients{}
	m := &Model{
		core:   runtime.New(sess, catalog.All()),
		appCtx: context.Background(),
		isDemo: true,
	}

	cmd := m.probeEnrichment(sentinelType, 1)
	if cmd == nil {
		t.Fatalf("probeEnrichment returned nil cmd in demo mode with registered enricher; want a tea.Cmd (demo dispatches like live)")
	}
	if got := atomic.LoadInt32(&fnCalls); got != 0 {
		t.Fatalf("enricher Fn was invoked %d time(s) before the cmd executed; want 0 (dispatch must be lazy)", got)
	}

	msg := cmd()
	checked, ok := msg.(messages.EnrichmentChecked)
	if !ok {
		t.Fatalf("cmd() returned %T; want messages.EnrichmentChecked", msg)
	}
	if checked.ResourceType != sentinelType {
		t.Fatalf("EnrichmentChecked.ResourceType = %q; want %q", checked.ResourceType, sentinelType)
	}
	if got := atomic.LoadInt32(&fnCalls); got != 1 {
		t.Fatalf("enricher Fn invocations in demo mode = %d; want 1 (demo must dispatch through the registry like live)", got)
	}
}

// TestProbeEnrichment_NonDemoMode_ReturnsCmd verifies the live path: a
// registered enricher produces a non-nil tea.Cmd whose closure invokes the
// enricher Fn.
func TestProbeEnrichment_NonDemoMode_ReturnsCmd(t *testing.T) {
	const sentinelType = "dbi-snap-probe-demo-guard-prod-pin"

	var fnCalls int32
	captureFn := func(_ context.Context, _ *awsclient.ServiceClients, _ []resource.Resource, _ resource.ResourceCache) (awsclient.IssueEnricherResult, error) {
		atomic.AddInt32(&fnCalls, 1)
		return awsclient.IssueEnricherResult{}, nil
	}

	awsclient.SetWave2EnricherForTest(t, sentinelType, awsclient.IssueEnricher{Fn: captureFn, Priority: 100})

	sess := session.New()
	sess.Clients = &awsclient.ServiceClients{}
	m := &Model{
		core:   runtime.New(sess, catalog.All()),
		appCtx: context.Background(),
		isDemo: false,
	}

	cmd := m.probeEnrichment(sentinelType, 1)
	if cmd == nil {
		t.Fatalf("probeEnrichment returned nil cmd in non-demo mode with registered enricher; want a tea.Cmd")
	}
	_ = cmd()
	if got := atomic.LoadInt32(&fnCalls); got != 1 {
		t.Fatalf("enricher Fn invocations in non-demo mode = %d; want 1", got)
	}
}
