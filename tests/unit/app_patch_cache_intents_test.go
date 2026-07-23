// app_patch_cache_intents_test.go — RED-phase pins for the web detail-latency
// fix's remaining three contracts:
//
//  3. Patch*Cache intents (PatchRelatedCache / PatchResourceCache /
//     PatchLazyResourceCache) currently no-op in Controller.applyIntents
//     (core/app/intents.go:193-197, default case `_ = v`), so the headless
//     RelatedCache/ResourceCache/LazyResourceCache never fill. Reopening the
//     same detail after popping re-runs the full related-check fan-out
//     instead of hitting the cache-hit replay path at
//     core/app/controller.go:209-257 (openSelectedListDetail).
//
//  4. KindEnrichDetail is dispatched by internal/tui/runtime_adapter.go's
//     handleEnrichDetail but NEVER by the headless controller —
//     core/app/navigate.go's applyNavResult (NavigateKindPushDetail case,
//     lines 115-132) reads res.DispatchRelated but never res.DispatchEnrich,
//     even though runtime.HandleNavigate already sets DispatchEnrich=true
//     (core/runtime/handlers_navigate.go:195) whenever the resource type
//     has a registered detail enricher. Web details never get Wave-2
//     enrichment.
//
//  5. Staleness hazards around RelatedCheckBatch delivery (Controller.Handle,
//     core/app/handle.go:68-70 -> handleRelatedCheckBatch). These may
//     already be green; pinned here regardless per the fix task spec, with
//     each test's status noted in its doc comment.
//
// Harness: mirrors controller_detail_rightcol_test.go and
// runtime_executor_test.go's SetRelatedForTest/SetDetailEnricherForTest
// pattern — synthetic short names or a real catalog type (ec2, which has no
// production detail enricher registered) with t.Cleanup restoring the global
// registries.
package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// ─────────────────────────────────────────────────────────────────────────
// Contract 3: Patch*Cache intents must write through to session state.
// ─────────────────────────────────────────────────────────────────────────

// TestApplyIntents_PatchResourceCache_WritesSessionResourceCache verifies
// that applying a runtime.PatchResourceCache intent actually populates the
// session-owned ResourceCache (read back via Core.ResourceCache), instead of
// falling through to the intents.go default no-op case.
//
// Status: RED (behavior). intents.go's default case discards this intent.
func TestApplyIntents_PatchResourceCache_WritesSessionResourceCache(t *testing.T) {
	c, core := newTestControllerAndCore(t)

	entry := &domain.ListViewCacheEntry{
		Resources: []resource.Resource{{ID: "i-cache0001", Type: "ec2"}},
	}
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchResourceCache{ResourceType: "ec2", Entry: entry},
	})

	got, ok := core.ResourceCache("ec2")
	if !ok {
		t.Fatal("Core.ResourceCache(\"ec2\") ok=false after PatchResourceCache intent — intent did not write through")
	}
	if got == nil || len(got.Resources) != 1 || got.Resources[0].ID != "i-cache0001" {
		t.Errorf("Core.ResourceCache(\"ec2\") = %+v, want entry with Resources[0].ID=%q", got, "i-cache0001")
	}
}

// TestApplyIntents_PatchLazyResourceCache_WritesSessionLazyCache verifies
// that applying a runtime.PatchLazyResourceCache intent merges into the
// session-owned LazyResourceCache (read back via Core.LazyResourceCache).
//
// Status: RED (behavior). intents.go's default case discards this intent.
func TestApplyIntents_PatchLazyResourceCache_WritesSessionLazyCache(t *testing.T) {
	c, core := newTestControllerAndCore(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchLazyResourceCache{
			Adds: map[string][]resource.Resource{
				"kms": {{ID: "key-lazy0001", Type: "kms"}},
			},
		},
	})

	rows, ok := core.LazyResourceCache("kms")
	if !ok {
		t.Fatal("Core.LazyResourceCache(\"kms\") ok=false after PatchLazyResourceCache intent — intent did not write through")
	}
	if len(rows) != 1 || rows[0].ID != "key-lazy0001" {
		t.Errorf("Core.LazyResourceCache(\"kms\") = %+v, want [{ID: key-lazy0001}]", rows)
	}
}

// TestApplyIntents_PatchRelatedCache_WritesSessionRelatedCache verifies that
// applying a runtime.PatchRelatedCache intent populates the session-owned
// RelatedCache LRU (read back via Core.RelatedCacheGet using the same
// runtime.RelatedCacheKey the read paths use).
//
// Status: RED (behavior). intents.go's default case discards this intent —
// this is the direct cause of the reported bug: headless RelatedCache never
// fills, so reopening a detail always misses and re-runs the full fan-out.
func TestApplyIntents_PatchRelatedCache_WritesSessionRelatedCache(t *testing.T) {
	c, core := newTestControllerAndCore(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchRelatedCache{
			ResourceType:   "ec2",
			SourceID:       "i-relcache001",
			DefDisplayName: "Security Groups",
			Result: resource.RelatedCheckResult{
				TargetType:  "sg",
				Count:       3,
				ResourceIDs: []string{"sg-1", "sg-2", "sg-3"},
			},
		},
	})

	key := runtime.RelatedCacheKey("ec2", "i-relcache001")
	cached, hit := core.RelatedCacheGet(key)
	if !hit {
		t.Fatal("Core.RelatedCacheGet after PatchRelatedCache intent: hit=false — intent did not write through")
	}
	if len(cached) != 1 {
		t.Fatalf("Core.RelatedCacheGet after PatchRelatedCache intent: len=%d, want 1", len(cached))
	}
	if cached[0].DefDisplayName != "Security Groups" {
		t.Errorf("cached[0].DefDisplayName = %q, want %q", cached[0].DefDisplayName, "Security Groups")
	}
	if cached[0].Result.Count != 3 {
		t.Errorf("cached[0].Result.Count = %d, want 3", cached[0].Result.Count)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Contract 3 (integration pin): reopening the same detail must not re-dispatch
// KindRelatedCheck once the cache is populated via the fixed intent.
// ─────────────────────────────────────────────────────────────────────────

// TestOpenSelectedListDetail_SecondOpen_CacheHit_NoRelatedCheckTask drives
// the real cache-hit replay path at core/app/controller.go's
// openSelectedListDetail: open a detail (miss -> KindRelatedCheck dispatched),
// feed back a RelatedCheckBatch result (which — once contract 3 is fixed —
// writes through to session.RelatedCache via PatchRelatedCache), pop back to
// the list, then select the SAME row again. The second open must dispatch
// ZERO KindRelatedCheck tasks because openSelectedListDetail's RelatedCacheGet
// check (controller.go:233) now hits.
//
// Status: RED (behavior). Currently PatchRelatedCache is a no-op, so
// RelatedCacheGet always misses and every reopen re-dispatches the full
// related-check fan-out — the reported latency bug.
func TestOpenSelectedListDetail_SecondOpen_CacheHit_NoRelatedCheckTask(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: noopChecker},
	})

	c := newTestController(t)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-reopen0001", Type: "ec2", Name: "reopen-test-instance"},
	}, nil, false)

	// First open: cache miss, must dispatch KindRelatedCheck.
	_, tasksFirst := c.Apply(app.Action{Kind: app.ActionSelect})
	if !hasTaskKind(tasksFirst, runtime.KindRelatedCheck) {
		t.Fatalf("first ActionSelect: expected a KindRelatedCheck task on cache miss, got %v", taskKindStrings(tasksFirst))
	}

	// Feed back the checker result as the headless executor would via DrainSync.
	c.Handle(messages.RelatedCheckBatch{
		ResourceType:     "ec2",
		SourceResourceID: "i-reopen0001",
		Results: []messages.RelatedCheckResult{
			{
				ResourceType:     "ec2",
				SourceResourceID: "i-reopen0001",
				DefDisplayName:   "Security Groups",
				Result:           resource.RelatedCheckResult{TargetType: "sg", Count: 2, ResourceIDs: []string{"sg-a", "sg-b"}},
			},
		},
		OperationID: 0, // AcceptZeroGen=true
	})

	// Pop back to the list.
	c.Apply(app.Action{Kind: app.ActionBack})

	// Second open of the SAME row: must be a cache hit -> zero KindRelatedCheck tasks.
	_, tasksSecond := c.Apply(app.Action{Kind: app.ActionSelect})
	if hasTaskKind(tasksSecond, runtime.KindRelatedCheck) {
		t.Errorf("second ActionSelect on the same resource: got a KindRelatedCheck task (%v) — "+
			"expected zero tasks of that kind because the related cache should have been populated "+
			"by the first RelatedCheckBatch via PatchRelatedCache", taskKindStrings(tasksSecond))
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Contract 4: detail enrichment must be dispatched headless.
// ─────────────────────────────────────────────────────────────────────────

// TestOpenSelectedListDetail_TypeWithDetailEnricher_DispatchesEnrichDetailTask
// verifies that opening a detail for a resource type with a registered
// detail enricher includes a KindEnrichDetail task among the returned
// TaskRequests, and that the kind is classified background per
// app.IsBackgroundTaskKind.
//
// Status: RED (behavior). applyNavResult's NavigateKindPushDetail case never
// reads res.DispatchEnrich nor calls c.core.HandleEnrichDetail — the runtime
// signals DispatchEnrich=true (handlers_navigate.go:195) but the headless
// controller drops it on the floor. Web details never get Wave-2 enrichment.
func TestOpenSelectedListDetail_TypeWithDetailEnricher_DispatchesEnrichDetailTask(t *testing.T) {
	resource.SetDetailEnricherForTest("ec2", func(_ context.Context, _ any, res resource.Resource) (resource.Resource, error) {
		return res, nil
	})
	t.Cleanup(func() { resource.CleanupDetailEnricherForTest("ec2") })

	c := newTestController(t)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-enrich0001", Type: "ec2", Name: "enrich-test-instance"},
	}, nil, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})

	if !hasTaskKind(tasks, runtime.KindEnrichDetail) {
		t.Fatalf("ActionSelect on a resource type with a registered detail enricher: expected a "+
			"KindEnrichDetail task, got %v", taskKindStrings(tasks))
	}
	if !app.IsBackgroundTaskKind(runtime.KindEnrichDetail) {
		t.Error("app.IsBackgroundTaskKind(KindEnrichDetail) = false, want true — detail enrichment " +
			"must not block the initial detail render")
	}
}

// TestOpenSelectedListDetail_TypeWithoutDetailEnricher_NoEnrichDetailTask
// verifies the negative: a resource type with NO registered detail enricher
// must not dispatch KindEnrichDetail — mirrors resource.GetDetailEnricher's
// nil-guard in runtime.HandleEnrichDetail.
//
// Status: this may already be green (absence of a positive case is trivially
// satisfied while contract 4 is entirely unwired) but is pinned here so a
// fix that dispatches KindEnrichDetail unconditionally (ignoring the
// registered-enricher gate) is caught as a regression.
func TestOpenSelectedListDetail_TypeWithoutDetailEnricher_NoEnrichDetailTask(t *testing.T) {
	if resource.HasDetailEnricher("s3") {
		t.Skip("s3 unexpectedly has a registered detail enricher in this test binary — assumption broken")
	}

	c := newTestController(t)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	c.ApplyResourcesLoaded("s3", []resource.Resource{
		{ID: "bucket-no-enrich", Type: "s3", Name: "bucket-no-enrich"},
	}, nil, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})

	if hasTaskKind(tasks, runtime.KindEnrichDetail) {
		t.Errorf("ActionSelect on s3 (no registered detail enricher): got a KindEnrichDetail task (%v), want none",
			taskKindStrings(tasks))
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Contract 5: staleness hazards around RelatedCheckBatch delivery.
// ─────────────────────────────────────────────────────────────────────────

// TestHandle_RelatedCheckBatch_AfterDetailPopped_NoPanic_TopScreenUnchanged
// verifies that a RelatedCheckBatch arriving after its originating detail
// screen has been popped off the stack does not panic and does not corrupt
// the new top-of-stack screen (handleRelatedCheckBatch's target-detail scan
// finds no match and silently drops the merge).
//
// Status: expected GREEN already — handleRelatedCheckBatch (handle.go:204-221)
// scans the stack for a matching ScreenDetail by (ResourceType, ResourceID)
// and no-ops when none is found. Pinned per the fix task spec so a future
// change to that scan cannot silently reintroduce a panic or cross-screen
// corruption.
func TestHandle_RelatedCheckBatch_AfterDetailPopped_NoPanic_TopScreenUnchanged(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: noopChecker},
	})

	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-popped0001", Type: "ec2", Name: "popped-test-instance"},
	}, nil, false)
	c.Apply(app.Action{Kind: app.ActionSelect})

	// Pop the detail back to the list before the batch arrives.
	c.Apply(app.Action{Kind: app.ActionBack})
	snapBeforeBatch := c.Snapshot()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Handle(RelatedCheckBatch) after detail popped panicked: %v", r)
			}
		}()
		c.Handle(messages.RelatedCheckBatch{
			ResourceType:     "ec2",
			SourceResourceID: "i-popped0001",
			Results: []messages.RelatedCheckResult{
				{
					ResourceType:     "ec2",
					SourceResourceID: "i-popped0001",
					DefDisplayName:   "Security Groups",
					Result:           resource.RelatedCheckResult{TargetType: "sg", Count: 1, ResourceIDs: []string{"sg-late"}},
				},
			},
			OperationID: 0,
		})
	}()

	snapAfterBatch := c.Snapshot()
	if snapAfterBatch.Body.Kind != snapBeforeBatch.Body.Kind {
		t.Errorf("top screen Body.Kind changed after a late RelatedCheckBatch for a popped detail: "+
			"before=%q after=%q — the batch must not mutate the new top screen", snapBeforeBatch.Body.Kind, snapAfterBatch.Body.Kind)
	}
}

// TestHandle_RelatedCheckBatch_StaleOperation_Dropped verifies that a
// RelatedCheckBatch stamped with a stale (non-zero) OperationID — i.e. a
// batch dispatched under an earlier DetailOperation than the session's
// active one — is dropped by the messages.IsStale guard in Controller.Handle
// (handle.go:68) rather than merged into the active detail's RelatedRows.
//
// OperationID 0 is NOT usable to pin staleness here:
// messages.RelatedCheckBatch.AcceptZeroGen() returns true by design
// (core/runtime/messages/event.go:189), because real batches are always
// stamped with the non-zero id of the DetailOperation that dispatched them.
// This test captures the real dispatch-time operation id, begins a fresh
// DetailOperation (mirrors a Ctrl+R refresh) so that id is now stale, and
// delivers a batch stamped with it.
//
// Status: expected GREEN already — Handle explicitly gates
// handleRelatedCheckBatch behind !messages.IsStale(batch, c.core). Pinned per
// the fix task spec as a regression guard.
func TestHandle_RelatedCheckBatch_StaleOperation_Dropped(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: noopChecker},
	})

	c, core := newTestControllerAndCore(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-stale0001", Type: "ec2", Name: "stale-gen-test-instance"},
	}, nil, false)
	c.Apply(app.Action{Kind: app.ActionSelect})

	// Capture the operation ID this batch would have been dispatched under,
	// then begin a fresh DetailOperation (mirrors a Ctrl+R refresh) so that
	// captured id is now stale relative to the session's active operation.
	staleOp := core.ActiveDetailOp()
	core.BeginDetailOperation("ec2", resource.Resource{ID: "i-stale0001"}, true)

	c.Handle(messages.RelatedCheckBatch{
		ResourceType:     "ec2",
		SourceResourceID: "i-stale0001",
		Results: []messages.RelatedCheckResult{
			{
				ResourceType:     "ec2",
				SourceResourceID: "i-stale0001",
				DefDisplayName:   "Security Groups",
				Result:           resource.RelatedCheckResult{TargetType: "sg", Count: 9, ResourceIDs: []string{"sg-stale"}},
			},
		},
		OperationID: staleOp, // captured before the fresh BeginDetailOperation above — now stale
	})

	key := runtime.RelatedCacheKey("ec2", "i-stale0001")
	if _, hit := core.RelatedCacheGet(key); hit {
		t.Error("RelatedCacheGet hit after a stale-operation RelatedCheckBatch — " +
			"the stale batch must be dropped entirely, including the RelatedCache write")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// shared helpers
// ─────────────────────────────────────────────────────────────────────────

// newTestControllerAndCore builds a Controller and returns both it and the
// backing *runtime.Core so tests can assert on session-owned cache state via
// Core's public accessors (ResourceCache / LazyResourceCache / RelatedCacheGet
// / BumpRelatedGen) without the Controller exposing its unexported core field.
//
// Redirects A9S_CONFIG_FOLDER to a fresh t.TempDir() and registers
// t.Cleanup(c.Close) — in that order (see newTestController in
// app_controller_test.go, and app_availsave_tempdir_cleanup_race_test.go
// for the traced race this ordering closes).
func newTestControllerAndCore(t *testing.T) (*app.Controller, *runtime.Core) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	return c, core
}

// hasTaskKind reports whether tasks contains at least one TaskRequest of the
// given kind.
func hasTaskKind(tasks []runtime.TaskRequest, kind runtime.TaskKind) bool {
	for _, task := range tasks {
		if task.Key.Kind == kind {
			return true
		}
	}
	return false
}
