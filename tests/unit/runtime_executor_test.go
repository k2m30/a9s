package unit

// runtime_executor_test.go — behavioral tests for Core.ExecuteTask.
//
// Every test builds a demo-mode Core (fake AWS clients, no real network) and
// calls ExecuteTask synchronously. Assertions are on the concrete messages.Event
// type returned and key result fields — not on internal runtime state.
//
// TaskKind → expected result table:
//
//	TaskKindProbeAvailability  → messages.AvailabilityChecked
//	TaskKindProbeEnrich        → nil,nil (isDemo=true); messages.EnrichmentChecked (isDemo=false, no enricher registered → nil,nil)
//	TaskKindSaveCache          → nil,nil (NoCache=true); nil,nil (empty cache)
//	TaskKindConnect            → messages.ClientsReady (error path — no real AWS creds)
//	TaskKindFetchIdentity      → messages.IdentityError (nil STS client)
//	TaskKindLoadAvailCache     → messages.AvailabilityCacheLoaded (no cache file → Expired:true)
//	TaskKindDemoPrefetchCounts → messages.AvailabilityPrefetched
//	TaskKindFetchChildResources → messages.APIError (nil clients path) / messages.ResourcesLoaded (demo)
//	KindFetchResources         → messages.ResourcesLoaded (demo, ec2)
//	KindFetchFiltered          → messages.APIError (no filtered fetcher for ec2)
//	KindFetchMore              → messages.ResourcesLoaded (demo, ec2)
//	KindFetchByIDDetail        → messages.Flash (no by-id fetcher registered for "unknown-type")
//	KindFetchReveal            → messages.ValueRevealed (nil clients → Err set)
//	KindRelatedCheck           → nil (no related defs for "ec2" in test isolation)
//	KindEnrichDetail           → error (nil payload)
//	Adapter-only kinds         → ErrAdapterOnlyTask (6 kinds)

import (
	"context"
	"errors"
	"testing"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// ────────────────────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────────────────────

// newExecutorCore builds a Core with demo fake clients installed into
// session.Clients via HandleClientsReady (the only public path). NoCache is
// set so the save-cache path is short-circuited to nil,nil by default.
func newExecutorCore(t *testing.T) *runtime.Core {
	t.Helper()
	c := runtime.Bootstrap(demo.DemoProfile, demo.DemoRegion, catalog.All())
	c.SetNoCache(true)
	c.SetIsDemo(true)
	fakeClients := demo.NewServiceClients()
	c.SetPreSuppliedClients(fakeClients)
	// Gen guard: ConnectGen starts at 0 (session.New does not seed it).
	// Pass Gen=0 to match; StackDepth=1 (main menu only, no -c command).
	c.HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // intentional — we only need side-effect
		Clients:    nil, // triggers PreSuppliedClients fallback
		Gen:        c.ConnectGen(),
		StackDepth: 1,
	})
	return c
}

// newExecutorCoreNonDemo builds a Core identical to newExecutorCore but with
// isDemo=false, so Wave-2 enrichment probe paths are exercised.
func newExecutorCoreNonDemo(t *testing.T) *runtime.Core {
	t.Helper()
	c := runtime.Bootstrap(demo.DemoProfile, demo.DemoRegion, catalog.All())
	c.SetNoCache(true)
	c.SetIsDemo(false)
	fakeClients := demo.NewServiceClients()
	c.SetPreSuppliedClients(fakeClients)
	c.HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // intentional — we only need side-effect
		Clients:    nil,
		Gen:        c.ConnectGen(),
		StackDepth: 1,
	})
	return c
}

// req builds a TaskRequest with no payload (for kinds that need none).
func req(kind runtime.TaskKind, scope string) runtime.TaskRequest {
	return runtime.TaskRequest{Key: runtime.TaskKey{Kind: kind, Scope: scope}}
}

// reqP builds a TaskRequest with a payload.
func reqP(kind runtime.TaskKind, scope string, payload runtime.TaskPayload) runtime.TaskRequest {
	return runtime.TaskRequest{Key: runtime.TaskKey{Kind: kind, Scope: scope}, Payload: payload}
}

// ────────────────────────────────────────────────────────────────────────────
// TaskKindProbeAvailability → messages.AvailabilityChecked
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_ProbeAvailability_ReturnsAvailabilityChecked(t *testing.T) {
	c := newExecutorCore(t)
	ev, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindProbeAvailability, "ec2"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.AvailabilityChecked)
	if !ok {
		t.Fatalf("expected messages.AvailabilityChecked, got %T", ev)
	}
	if got.ResourceType != "ec2" {
		t.Errorf("ResourceType = %q, want %q", got.ResourceType, "ec2")
	}
}

func TestExecuteTask_ProbeAvailability_AllDemoResourceTypes(t *testing.T) {
	c := newExecutorCore(t)
	ctx := context.Background()
	// Probe every registered resource type — all must return AvailabilityChecked.
	types := catalog.All()
	if len(types) == 0 {
		t.Fatal("catalog.All() is empty")
	}
	for _, td := range types {
		t.Run(td.ShortName, func(t *testing.T) {
			ev, err := c.ExecuteTask(ctx, req(runtime.TaskKindProbeAvailability, td.ShortName))
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", td.ShortName, err)
			}
			got, ok := ev.(messages.AvailabilityChecked)
			if !ok {
				t.Fatalf("%s: expected messages.AvailabilityChecked, got %T", td.ShortName, ev)
			}
			if got.ResourceType != td.ShortName {
				t.Errorf("%s: ResourceType = %q, want %q", td.ShortName, got.ResourceType, td.ShortName)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TaskKindProbeEnrich — isDemo gating
// ────────────────────────────────────────────────────────────────────────────

// When isDemo=true the executor must return nil,nil without calling any
// enricher (demo fixtures do not require real credentials).
func TestExecuteTask_ProbeEnrich_IsDemo_ReturnsNilNil(t *testing.T) {
	c := newExecutorCore(t) // isDemo=true
	ev, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindProbeEnrich, "ec2"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Errorf("expected nil event in demo mode, got %T", ev)
	}
}

// When isDemo=false AND the resource type has no issue enricher registered,
// the executor also returns nil,nil (HasIssueEnricher guard).
func TestExecuteTask_ProbeEnrich_NonDemo_NoEnricher_ReturnsNilNil(t *testing.T) {
	c := newExecutorCoreNonDemo(t) // isDemo=false

	// Find a resource type that has no Wave-2 enricher.
	var noEnricherType string
	for _, td := range catalog.All() {
		if !c.HasIssueEnricher(td.ShortName) {
			noEnricherType = td.ShortName
			break
		}
	}
	if noEnricherType == "" {
		t.Skip("all registered types have issue enrichers; cannot test no-enricher path")
	}

	ev, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindProbeEnrich, noEnricherType))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Errorf("expected nil event when no enricher registered, got %T", ev)
	}
}

// When isDemo=false AND the resource type HAS an issue enricher, the executor
// must return messages.EnrichmentChecked (not nil).
func TestExecuteTask_ProbeEnrich_NonDemo_WithEnricher_ReturnsEnrichmentChecked(t *testing.T) {
	c := newExecutorCoreNonDemo(t) // isDemo=false

	// Find a resource type that has a Wave-2 enricher.
	var enricherType string
	for _, td := range catalog.All() {
		if c.HasIssueEnricher(td.ShortName) {
			enricherType = td.ShortName
			break
		}
	}
	if enricherType == "" {
		t.Skip("no registered types have issue enrichers; cannot test enricher path")
	}

	ev, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindProbeEnrich, enricherType))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// If an enricher is registered the result must be EnrichmentChecked.
	got, ok := ev.(messages.EnrichmentChecked)
	if !ok {
		t.Fatalf("expected messages.EnrichmentChecked, got %T", ev)
	}
	if got.ResourceType != enricherType {
		t.Errorf("ResourceType = %q, want %q", got.ResourceType, enricherType)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TaskKindSaveCache — NoCache short-circuit and empty cache
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_SaveCache_NoCache_ReturnsNilNil(t *testing.T) {
	c := newExecutorCore(t) // NoCache=true
	ev, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindSaveCache, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Errorf("expected nil event with NoCache=true, got %T", ev)
	}
}

func TestExecuteTask_SaveCache_EmptyResourceCache_ReturnsNilNil(t *testing.T) {
	// Isolate writes from ~/.a9s/cache/ — any write lands in an empty temp dir.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	// Build core with caching enabled but no resources loaded → entries==nil path.
	c := runtime.Bootstrap(demo.DemoProfile, demo.DemoRegion, catalog.All())
	c.SetIsDemo(true)
	// NoCache=false so save-cache proceeds past the guard.
	c.SetNoCache(false)
	fakeClients := demo.NewServiceClients()
	c.SetPreSuppliedClients(fakeClients)
	c.HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // side-effect only
		Clients:    nil,
		Gen:        c.ConnectGen(),
		StackDepth: 1,
	})

	// ResourceCache is empty → availabilityFromResourceCache returns nil → nil,nil.
	ev, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindSaveCache, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Errorf("expected nil event with empty ResourceCache, got %T", ev)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TaskKindConnect → messages.ClientsReady (error path — no real AWS creds)
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_Connect_ReturnsClientsReady(t *testing.T) {
	c := newExecutorCore(t)
	p := runtime.ConnectPayload{Profile: "nonexistent-profile-000000000000", Region: "us-east-1"}
	ev, err := c.ExecuteTask(context.Background(), reqP(runtime.TaskKindConnect, "", p))
	// ConnectAWS may error (no real creds) but ExecuteTask itself must not error —
	// it wraps the error in ClientsReady.Err instead.
	if err != nil {
		t.Fatalf("unexpected error from ExecuteTask: %v", err)
	}
	got, ok := ev.(messages.ClientsReady)
	if !ok {
		t.Fatalf("expected messages.ClientsReady, got %T", ev)
	}
	// Gen must be forwarded from the payload.
	if got.Gen != p.Gen {
		t.Errorf("Gen = %v, want %v", got.Gen, p.Gen)
	}
}

func TestExecuteTask_Connect_MissingPayload_ReturnsError(t *testing.T) {
	c := newExecutorCore(t)
	_, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindConnect, ""))
	if err == nil {
		t.Fatal("expected error for missing ConnectPayload, got nil")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TaskKindFetchIdentity → messages.IdentityError (STS client is a fake that
// does not implement STS GetCallerIdentity)
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_FetchIdentity_ReturnsIdentityResult(t *testing.T) {
	c := newExecutorCore(t)
	ev, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindFetchIdentity, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Demo STS fake may succeed or fail; either IdentityLoaded or IdentityError
	// must be returned — never nil.
	switch ev.(type) {
	case messages.IdentityLoaded, messages.IdentityError:
		// correct
	default:
		t.Fatalf("expected IdentityLoaded or IdentityError, got %T", ev)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TaskKindLoadAvailCache → messages.AvailabilityCacheLoaded
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_LoadAvailCache_ReturnsAvailabilityCacheLoaded(t *testing.T) {
	// Isolate from ~/.a9s/cache/ so no pre-existing file can make Expired=false.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	c := newExecutorCore(t)
	ev, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindLoadAvailCache, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.AvailabilityCacheLoaded)
	if !ok {
		t.Fatalf("expected messages.AvailabilityCacheLoaded, got %T", ev)
	}
	// No cache file exists for demo profile → Entries must be non-nil map (possibly empty)
	// and Expired must be true.
	if got.Entries == nil {
		t.Error("Entries must not be nil")
	}
	if !got.Expired {
		t.Error("Expired must be true when no cache file exists")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TaskKindDemoPrefetchCounts → messages.AvailabilityPrefetched
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_DemoPrefetchCounts_ReturnsAvailabilityPrefetched(t *testing.T) {
	c := newExecutorCore(t)
	ev, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindDemoPrefetchCounts, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.AvailabilityPrefetched)
	if !ok {
		t.Fatalf("expected messages.AvailabilityPrefetched, got %T", ev)
	}
	if got.Entries == nil {
		t.Error("Entries must not be nil after demo prefetch")
	}
	if len(got.Entries) == 0 {
		t.Error("Entries must be non-empty after demo prefetch with fake clients")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// KindFetchResources → messages.ResourcesLoaded (demo ec2, s3, rds, lambda)
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_FetchResources_EC2_ReturnsResourcesLoaded(t *testing.T) {
	c := newExecutorCore(t)
	ev, err := c.ExecuteTask(context.Background(), req(runtime.KindFetchResources, "ec2"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected messages.ResourcesLoaded, got %T", ev)
	}
	if got.ResourceType != "ec2" {
		t.Errorf("ResourceType = %q, want %q", got.ResourceType, "ec2")
	}
}

func TestExecuteTask_FetchResources_DemoTypes_ReturnResourcesLoaded(t *testing.T) {
	// Spot-check a cross-section of demo resource types.
	demoTypes := []string{"ec2", "s3", "rds", "lambda", "ecs", "eks"}
	c := newExecutorCore(t)
	ctx := context.Background()
	for _, rt := range demoTypes {
		t.Run(rt, func(t *testing.T) {
			ev, err := c.ExecuteTask(ctx, req(runtime.KindFetchResources, rt))
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", rt, err)
			}
			got, ok := ev.(messages.ResourcesLoaded)
			if !ok {
				t.Fatalf("%s: expected messages.ResourcesLoaded, got %T", rt, ev)
			}
			if got.ResourceType != rt {
				t.Errorf("%s: ResourceType = %q, want %q", rt, got.ResourceType, rt)
			}
		})
	}
}

func TestExecuteTask_FetchResources_UnknownType_ReturnsAPIError(t *testing.T) {
	c := newExecutorCore(t)
	ev, err := c.ExecuteTask(context.Background(), req(runtime.KindFetchResources, "nonexistent-type-000"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, ok := ev.(messages.APIError)
	if !ok {
		t.Fatalf("expected messages.APIError for unknown type, got %T", ev)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// KindFetchFiltered → messages.APIError (ec2 has no filtered fetcher)
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_FetchFiltered_MissingPayload_ReturnsError(t *testing.T) {
	c := newExecutorCore(t)
	_, err := c.ExecuteTask(context.Background(), req(runtime.KindFetchFiltered, "ec2"))
	if err == nil {
		t.Fatal("expected error for missing fetchFilteredPayload")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// KindFetchMore → messages.ResourcesLoaded (demo ec2 with empty token)
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_FetchMore_EC2_ReturnsResourcesLoaded(t *testing.T) {
	c := newExecutorCore(t)
	p := runtime.FetchMorePayload{ContinuationToken: ""}
	ev, err := c.ExecuteTask(context.Background(), reqP(runtime.KindFetchMore, "ec2", p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected messages.ResourcesLoaded, got %T", ev)
	}
	if got.ResourceType != "ec2" {
		t.Errorf("ResourceType = %q, want %q", got.ResourceType, "ec2")
	}
	// FetchMore sets Append=true.
	if !got.Append {
		t.Error("Append must be true for FetchMore results")
	}
}

func TestExecuteTask_FetchMore_MissingPayload_ReturnsError(t *testing.T) {
	c := newExecutorCore(t)
	_, err := c.ExecuteTask(context.Background(), req(runtime.KindFetchMore, "ec2"))
	if err == nil {
		t.Fatal("expected error for missing FetchMorePayload")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// TaskKindFetchChildResources
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_FetchChildResources_MissingPayload_ReturnsError(t *testing.T) {
	c := newExecutorCore(t)
	_, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindFetchChildResources, ""))
	if err == nil {
		t.Fatal("expected error for missing FetchChildResourcesPayload")
	}
}

func TestExecuteTask_FetchChildResources_UnknownChildType_ReturnsAPIError(t *testing.T) {
	c := newExecutorCore(t)
	p := runtime.FetchChildResourcesPayload{
		ChildType:     "nonexistent-child-000",
		ParentContext: map[string]string{"parentID": "i-0000000000000000"},
	}
	ev, err := c.ExecuteTask(context.Background(), reqP(runtime.TaskKindFetchChildResources, "", p))
	if err != nil {
		t.Fatalf("unexpected error from ExecuteTask: %v", err)
	}
	_, ok := ev.(messages.APIError)
	if !ok {
		t.Fatalf("expected messages.APIError for unknown child type, got %T", ev)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// KindFetchReveal → messages.ValueRevealed
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_FetchReveal_MissingPayload_ReturnsError(t *testing.T) {
	c := newExecutorCore(t)
	_, err := c.ExecuteTask(context.Background(), req(runtime.KindFetchReveal, "secrets/my-secret"))
	if err == nil {
		t.Fatal("expected error for missing FetchRevealPayload")
	}
}

func TestExecuteTask_FetchReveal_ReturnsValueRevealed(t *testing.T) {
	c := newExecutorCore(t)
	p := runtime.FetchRevealPayload{ResourceType: "secrets", ResourceID: "my-secret-000000000000"}
	ev, err := c.ExecuteTask(context.Background(), reqP(runtime.KindFetchReveal, "secrets/my-secret-000000000000", p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.ValueRevealed)
	if !ok {
		t.Fatalf("expected messages.ValueRevealed, got %T", ev)
	}
	if got.ResourceType != "secrets" {
		t.Errorf("ResourceType = %q, want %q", got.ResourceType, "secrets")
	}
	if got.ResourceID != "my-secret-000000000000" {
		t.Errorf("ResourceID = %q, want %q", got.ResourceID, "my-secret-000000000000")
	}
	// Err may be set (fake reveal fetcher); that is an acceptable result.
}

// ────────────────────────────────────────────────────────────────────────────
// KindFetchByIDDetail → messages.Flash (unknown type has no by-id fetcher)
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_FetchByIDDetail_NoFetcher_ReturnsFlash(t *testing.T) {
	c := newExecutorCore(t)
	p := runtime.FetchByIDDetailPayload{TargetType: "nonexistent-type-000", ID: "some-id-000000000000"}
	ev, err := c.ExecuteTask(context.Background(), reqP(runtime.KindFetchByIDDetail, "nonexistent-type-000", p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.Flash)
	if !ok {
		t.Fatalf("expected messages.Flash for no-fetcher path, got %T", ev)
	}
	if !got.IsError {
		t.Error("Flash.IsError must be true when no by-id fetcher is registered")
	}
}

func TestExecuteTask_FetchByIDDetail_MissingPayload_ReturnsError(t *testing.T) {
	c := newExecutorCore(t)
	_, err := c.ExecuteTask(context.Background(), req(runtime.KindFetchByIDDetail, "ec2"))
	if err == nil {
		t.Fatal("expected error for missing FetchByIDDetailPayload")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// KindRelatedCheck — no TUI coupling; returns nil (no defs, or defs with
// nil checkers that are skipped)
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_RelatedCheck_NoTUICoupling_ReturnsNil(t *testing.T) {
	// Temporarily replace ec2 related defs with a single def whose checker
	// returns zero results. runRelatedCheckers requires a non-nil checker.
	// The executor's runRelatedCheckers skips calling the checker (no source
	// resource is available in the TaskKey.Scope), so any non-nil checker works.
	resource.SetRelatedForTest("ec2", []resource.RelatedDef{
		{TargetType: "s3", DisplayName: "S3 Buckets", Checker: noopChecker},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest("ec2") })

	c := newExecutorCore(t)
	ev, err := c.ExecuteTask(context.Background(), req(runtime.KindRelatedCheck, "ec2/i-0000000000000000"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// runRelatedCheckers with nil checker returns nil.
	if ev != nil {
		t.Errorf("expected nil event for nil-checker relatedCheck, got %T", ev)
	}
}

func TestExecuteTask_RelatedCheck_NoDefs_ReturnsNil(t *testing.T) {
	// Temporarily clear related defs for a resource type so the "no defs" guard
	// fires first.
	resource.SetRelatedForTest("ec2", []resource.RelatedDef{})
	t.Cleanup(func() { resource.CleanupRelatedForTest("ec2") })

	c := newExecutorCore(t)
	ev, err := c.ExecuteTask(context.Background(), req(runtime.KindRelatedCheck, "ec2/i-0000000000000000"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Errorf("expected nil event when no related defs, got %T", ev)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// KindEnrichDetail — error paths (missing payload, nil DetailCtx, no enricher)
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_EnrichDetail_MissingPayload_ReturnsError(t *testing.T) {
	c := newExecutorCore(t)
	_, err := c.ExecuteTask(context.Background(), req(runtime.KindEnrichDetail, "ec2/i-0000000000000000"))
	if err == nil {
		t.Fatal("expected error for missing EnrichDetailPayload")
	}
}

func TestExecuteTask_EnrichDetail_NoEnricher_ReturnsError(t *testing.T) {
	c := newExecutorCore(t)
	p := runtime.EnrichDetailPayload{
		ResourceType: "nonexistent-type-000",
		Resource:     resource.Resource{ID: "some-id"},
	}
	_, err := c.ExecuteTask(context.Background(), reqP(runtime.KindEnrichDetail, "nonexistent-type-000/some-id", p))
	if err == nil {
		t.Fatal("expected error when no detail enricher registered")
	}
}

func TestExecuteTask_EnrichDetail_NilDetailCtx_ReturnsError(t *testing.T) {
	c := newExecutorCore(t)
	// Find a resource type that has a detail enricher.
	var enricherType string
	for _, td := range catalog.All() {
		if resource.GetDetailEnricher(td.ShortName) != nil {
			enricherType = td.ShortName
			break
		}
	}
	if enricherType == "" {
		t.Skip("no detail enrichers registered; cannot test nil-DetailCtx path")
	}
	p := runtime.EnrichDetailPayload{
		ResourceType: enricherType,
		Resource:     resource.Resource{ID: "some-id-000000000000"},
		DetailCtx:    nil, // triggers nil-DetailCtx error
	}
	_, err := c.ExecuteTask(context.Background(), reqP(runtime.KindEnrichDetail, enricherType+"/some-id-000000000000", p))
	if err == nil {
		t.Fatal("expected error when DetailCtx is nil")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Adapter-only kinds — all must return ErrAdapterOnlyTask
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_AdapterOnlyKinds_ReturnErrAdapterOnlyTask(t *testing.T) {
	adapterOnlyKinds := []runtime.TaskKind{
		runtime.TaskKindFlashTick,
		runtime.TaskKindEmitNavigate,
		runtime.TaskKindEmitAPIError,
		runtime.TaskKindReadThemeFile,
		runtime.TaskKindSaveThemeConfig,
		runtime.KindFetchProfiles,
	}

	c := newExecutorCore(t)
	ctx := context.Background()
	for _, kind := range adapterOnlyKinds {
		t.Run(string(kind), func(t *testing.T) {
			ev, err := c.ExecuteTask(ctx, req(kind, ""))
			if ev != nil {
				t.Errorf("kind %q: expected nil event, got %T", kind, ev)
			}
			if !errors.Is(err, runtime.ErrAdapterOnlyTask) {
				t.Errorf("kind %q: expected ErrAdapterOnlyTask, got %v", kind, err)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Unknown kind — must return a descriptive error (not ErrAdapterOnlyTask)
// ────────────────────────────────────────────────────────────────────────────

func TestExecuteTask_UnknownKind_ReturnsError(t *testing.T) {
	c := newExecutorCore(t)
	_, err := c.ExecuteTask(context.Background(), req("unknown-kind-that-does-not-exist", ""))
	if err == nil {
		t.Fatal("expected error for unknown task kind")
	}
	if errors.Is(err, runtime.ErrAdapterOnlyTask) {
		t.Error("unknown kind must not return ErrAdapterOnlyTask")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// isDemo gating contrast — same kind, same core, toggled flag
// ────────────────────────────────────────────────────────────────────────────

// Confirms that isDemo=true → nil,nil and isDemo=false (with enricher) →
// non-nil event for TaskKindProbeEnrich, proving the flag is the only
// deciding variable.
func TestExecuteTask_ProbeEnrich_IsDemoContrast(t *testing.T) {
	// Find a type with an enricher; skip if none.
	var enricherType string
	cProbe := newExecutorCoreNonDemo(t)
	for _, td := range catalog.All() {
		if cProbe.HasIssueEnricher(td.ShortName) {
			enricherType = td.ShortName
			break
		}
	}
	if enricherType == "" {
		t.Skip("no enricher registered; cannot contrast isDemo flag")
	}

	// isDemo=true → nil,nil
	cDemo := newExecutorCore(t) // isDemo=true
	evDemo, errDemo := cDemo.ExecuteTask(context.Background(), req(runtime.TaskKindProbeEnrich, enricherType))
	if errDemo != nil {
		t.Fatalf("isDemo=true: unexpected error: %v", errDemo)
	}
	if evDemo != nil {
		t.Errorf("isDemo=true: expected nil event, got %T", evDemo)
	}

	// isDemo=false → EnrichmentChecked
	evLive, errLive := cProbe.ExecuteTask(context.Background(), req(runtime.TaskKindProbeEnrich, enricherType))
	if errLive != nil {
		t.Fatalf("isDemo=false: unexpected error: %v", errLive)
	}
	if _, ok := evLive.(messages.EnrichmentChecked); !ok {
		t.Errorf("isDemo=false: expected messages.EnrichmentChecked, got %T", evLive)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// No TUI coupling — save-cache and related-check read Core/session only
// ────────────────────────────────────────────────────────────────────────────

// Verifies that KindRelatedCheck executes without any TUI model in scope.
// The only state it reads is c.session.ResourceCache (via SnapshotCache) and
// resource.GetRelated (registry). Neither touches a view stack.
func TestExecuteTask_RelatedCheck_ReadsCoreNotTUI(t *testing.T) {
	resource.SetRelatedForTest("s3", []resource.RelatedDef{})
	t.Cleanup(func() { resource.CleanupRelatedForTest("s3") })

	c := newExecutorCore(t)
	// Seed ResourceCache for s3 so SnapshotCache returns a non-empty map.
	c.SetResourceCache("s3", &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "my-bucket-000000000000", Name: "my-bucket"}},
	})

	ev, err := c.ExecuteTask(context.Background(), req(runtime.KindRelatedCheck, "s3/my-bucket-000000000000"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No defs → nil.
	if ev != nil {
		t.Errorf("expected nil (no defs), got %T", ev)
	}
}

// Verifies that TaskKindSaveCache reads c.session.ResourceCache without
// touching any TUI model. When the cache has one entry the result is either
// nil (write error or successful write) — never a panic.
func TestExecuteTask_SaveCache_ReadsCoreNotTUI(t *testing.T) {
	// Isolate writes from ~/.a9s/cache/ — any write lands in an empty temp dir.
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	c := runtime.Bootstrap(demo.DemoProfile, demo.DemoRegion, catalog.All())
	c.SetIsDemo(true)
	c.SetNoCache(false) // let save-cache proceed
	fakeClients := demo.NewServiceClients()
	c.SetPreSuppliedClients(fakeClients)
	c.HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // side-effect only
		Clients:    nil,
		Gen:        c.ConnectGen(),
		StackDepth: 1,
	})

	// Seed one cache entry so availabilityFromResourceCache returns non-nil.
	c.SetResourceCache("ec2", &session.ResourceCacheEntry{
		Resources: []resource.Resource{{ID: "i-0000000000000001", Name: "test-instance"}},
	})

	ev, err := c.ExecuteTask(context.Background(), req(runtime.TaskKindSaveCache, ""))
	// May return Flash{IsError:true} if the cache directory doesn't exist in test env.
	// Must not panic or return a non-nil err.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ev is either nil (success write) or Flash{IsError:true} (write error).
	if ev != nil {
		f, ok := ev.(messages.Flash)
		if !ok {
			t.Fatalf("expected nil or messages.Flash, got %T", ev)
		}
		if !f.IsError {
			t.Error("if SaveCache emits Flash it must be an error flash")
		}
	}
}
