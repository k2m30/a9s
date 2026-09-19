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

func TestApplyIntents_PatchRelatedCache_WritesSessionRelatedCache(t *testing.T) {
	c, core := newTestControllerAndCore(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchRelatedCache{
			ResourceType:   "ec2",
			SourceID:       "i-relcache001",
			DefDisplayName: "Security Groups",
			Result:         resource.KnownRelated("sg", []string{"sg-1", "sg-2", "sg-3"}, false),
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
	if cached[0].Result.Count() != 3 {
		t.Errorf("cached[0].Result.Count = %d, want 3", cached[0].Result.Count())
	}
}

func TestApplyIntents_PatchRelatedCache_SameDefTwice_ReplacesInPlace(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	key := runtime.RelatedCacheKey("ec2", "i-relcache002")

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchRelatedCache{
			ResourceType:   "ec2",
			SourceID:       "i-relcache002",
			DefDisplayName: "Security Groups",
			Result:         resource.KnownRelated("sg", []string{"sg-1", "sg-2", "sg-3"}, false),
		},
	})
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchRelatedCache{
			ResourceType:   "ec2",
			SourceID:       "i-relcache002",
			DefDisplayName: "Security Groups",
			Result:         resource.KnownRelated("sg", []string{"sg-1", "sg-2", "sg-3", "sg-4", "sg-5"}, false),
		},
	})

	cached, hit := core.RelatedCacheGet(key)
	if !hit {
		t.Fatal("Core.RelatedCacheGet after two PatchRelatedCache intents for the same def: hit=false")
	}
	if len(cached) != 1 {
		t.Fatalf("Core.RelatedCacheGet after two PatchRelatedCache intents for the SAME def: len=%d, want exactly 1 (idempotent replace, not append)", len(cached))
	}
	if cached[0].Result.Count() != 5 {
		t.Errorf("cached[0].Result.Count = %d, want 5 (the NEWER value)", cached[0].Result.Count())
	}
}

func TestApplyIntents_PatchRelatedCache_TwoDifferentDefs_Coexist(t *testing.T) {
	c, core := newTestControllerAndCore(t)
	key := runtime.RelatedCacheKey("ec2", "i-relcache003")

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchRelatedCache{
			ResourceType:   "ec2",
			SourceID:       "i-relcache003",
			DefDisplayName: "Security Groups",
			// KnownRelated derives Count from the unique ResourceIDs.
			Result: resource.KnownRelated("sg", []string{"sg-x1", "sg-x2", "sg-x3"}, false),
		},
	})
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchRelatedCache{
			ResourceType:   "ec2",
			SourceID:       "i-relcache003",
			DefDisplayName: "EBS Volumes",
			Result:         resource.KnownRelated("ebs", []string{"ebs-x1", "ebs-x2"}, false),
		},
	})

	cached, hit := core.RelatedCacheGet(key)
	if !hit {
		t.Fatal("Core.RelatedCacheGet after two PatchRelatedCache intents for DIFFERENT defs: hit=false")
	}
	if len(cached) != 2 {
		t.Fatalf("Core.RelatedCacheGet after two PatchRelatedCache intents for DIFFERENT defs: len=%d, want 2", len(cached))
	}
	byName := map[string]int{}
	for _, entry := range cached {
		byName[entry.DefDisplayName] = entry.Result.Count()
	}
	if byName["Security Groups"] != 3 {
		t.Errorf("Security Groups Count = %d, want 3", byName["Security Groups"])
	}
	if byName["EBS Volumes"] != 2 {
		t.Errorf("EBS Volumes Count = %d, want 2", byName["EBS Volumes"])
	}
}

func TestOpenSelectedListDetail_SecondOpen_CacheHit_NoRelatedCheckTask(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: noopChecker},
	})

	c := newTestController(t)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-reopen0001", Type: "ec2", Name: "reopen-test-instance"},
	}, nil, false)

	_, tasksFirst := c.Apply(app.Action{Kind: app.ActionSelect})
	if !hasTaskKind(tasksFirst, runtime.KindRelatedCheck) {
		t.Fatalf("first ActionSelect: expected a KindRelatedCheck task on cache miss, got %v", taskKindStrings(tasksFirst))
	}

	c.Handle(messages.RelatedCheckBatch{
		ResourceType:     "ec2",
		SourceResourceID: "i-reopen0001",
		Results: []messages.RelatedCheckResult{
			{
				ResourceType:     "ec2",
				SourceResourceID: "i-reopen0001",
				DefDisplayName:   "Security Groups",
				Result:           resource.KnownRelated("sg", []string{"sg-a", "sg-b"}, false),
			},
		},
		OperationID: 0, // AcceptZeroGen=true
	})

	c.Apply(app.Action{Kind: app.ActionBack})

	_, tasksSecond := c.Apply(app.Action{Kind: app.ActionSelect})
	if hasTaskKind(tasksSecond, runtime.KindRelatedCheck) {
		t.Errorf("second ActionSelect on the same resource: got a KindRelatedCheck task (%v) — "+
			"expected zero tasks of that kind because the related cache should have been populated "+
			"by the first RelatedCheckBatch via PatchRelatedCache", taskKindStrings(tasksSecond))
	}
}

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
					Result:           resource.KnownRelated("sg", []string{"sg-late"}, false),
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

// OperationID 0 cannot express staleness: RelatedCheckBatch.AcceptZeroGen()
// returns true, because real batches always carry the non-zero id of the
// DetailOperation that dispatched them.
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
				Result:           resource.KnownRelated("sg", []string{"sg-stale"}, false),
			},
		},
		OperationID: staleOp,
	})

	key := runtime.RelatedCacheKey("ec2", "i-stale0001")
	if _, hit := core.RelatedCacheGet(key); hit {
		t.Error("RelatedCacheGet hit after a stale-operation RelatedCheckBatch — " +
			"the stale batch must be dropped entirely, including the RelatedCache write")
	}
}

// newTestControllerAndCore returns the backing *runtime.Core alongside the
// Controller so tests can read session-owned cache state.
//
// A9S_CONFIG_FOLDER is redirected before t.Cleanup(c.Close) is registered so
// Close runs before the temp dir is removed.
func newTestControllerAndCore(t *testing.T) (*app.Controller, *runtime.Core) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	return c, core
}

func hasTaskKind(tasks []runtime.TaskRequest, kind runtime.TaskKind) bool {
	for _, task := range tasks {
		if task.Key.Kind == kind {
			return true
		}
	}
	return false
}
