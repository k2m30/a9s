package tui

// probe_demo_guard_test.go — AS-658 / AS-648-h3 P2.
//
// Pins the contract that `Model.probeEnrichment` returns a nil tea.Cmd when the
// Model is in demo mode (`WithIsDemo(true)`), so registered Wave-2 enrichers
// are NOT invoked against synthetic fakes / missing AWS credentials during the
// `./a9s --demo` startup prefetch or list refresh paths.

import (
	"context"
	"sync/atomic"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// TestProbeEnrichment_DemoMode_ReturnsNilAndSkipsRegistry verifies that when
// `m.isDemo == true`, probeEnrichment short-circuits to nil BEFORE consulting
// the Wave 2 enricher accessor. A sentinel enricher counts invocations of its
// lookup hit + closure; both must remain zero in demo mode.
func TestProbeEnrichment_DemoMode_ReturnsNilAndSkipsRegistry(t *testing.T) {
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
	if cmd != nil {
		t.Fatalf("probeEnrichment returned non-nil cmd in demo mode; want nil to skip Wave-2 enrichment")
	}
	if got := atomic.LoadInt32(&fnCalls); got != 0 {
		t.Fatalf("enricher Fn was invoked %d time(s) in demo mode; want 0 (early return must precede registry lookup)", got)
	}
}

// TestProbeEnrichment_NonDemoMode_ReturnsCmd verifies that the demo guard does
// NOT affect production (non-demo) behavior: a registered enricher still
// produces a non-nil tea.Cmd whose closure invokes the enricher Fn.
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
		t.Fatalf("enricher Fn invocations in non-demo mode = %d; want 1 (demo guard must not affect production path)", got)
	}
}
