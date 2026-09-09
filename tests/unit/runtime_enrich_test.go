package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestCoreDetailOperationTasks_NoEnricher_ReturnsNilEnrichTask verifies that
// Core.BeginDetailOperation returns a nil enrich task for a resource type
// with no registered detail enricher. This pins the SSOT contract: the
// runtime is the single decision-maker for the dispatch gate, so the
// adapter does not need to re-check enricher existence.
func TestCoreDetailOperationTasks_NoEnricher_ReturnsNilEnrichTask(t *testing.T) {
	if resource.HasDetailEnricher("ec2") {
		t.Skip("ec2 now has a detail enricher — pick a different no-enricher type")
	}
	core := runtime.New(session.New(), resource.AllResourceTypes())
	_, tasks := core.BeginDetailOperation("ec2", resource.Resource{ID: "i-1234567890abcdef0", Name: "no-enricher"}, false)
	if enrichTask := findTaskKind(tasks, runtime.KindEnrichDetail); enrichTask != nil {
		t.Errorf("expected no KindEnrichDetail task, got %+v", enrichTask)
	}
}

// TestCoreDetailOperationTasks_WithEnricher_EmitsTaskRequest verifies the shape
// of the TaskRequest Core.BeginDetailOperation emits for a resource type
// with a registered detail enricher: kind, scope, cache policy, and the
// EnrichDetailPayload the adapter type-switches on — Op the constructed
// DetailOperation verbatim, DetailCtx.SkipCache mirroring op.Refresh,
// DetailCtx.OpID mirroring op.ID.
func TestCoreDetailOperationTasks_WithEnricher_EmitsTaskRequest(t *testing.T) {
	if !resource.HasDetailEnricher("role_policies") {
		t.Fatal("expected role_policies detail enricher to be registered")
	}
	res := resource.Resource{
		ID:   "arn:aws:iam::123456789012:policy/runtime-test",
		Name: "runtime-test",
	}
	core := runtime.New(session.New(), resource.AllResourceTypes())
	op, tasks := core.BeginDetailOperation("role_policies", res, false)
	enrichTask := findTaskKind(tasks, runtime.KindEnrichDetail)
	if enrichTask == nil {
		t.Fatal("expected a KindEnrichDetail task")
	}
	if enrichTask.Key.Kind != runtime.KindEnrichDetail {
		t.Errorf("Key.Kind = %q, want %q", enrichTask.Key.Kind, runtime.KindEnrichDetail)
	}
	if want := "role_policies/" + res.ID; enrichTask.Key.Scope != want {
		t.Errorf("Key.Scope = %q, want %q", enrichTask.Key.Scope, want)
	}
	if enrichTask.Cache != runtime.CacheNone {
		t.Errorf("Cache = %v, want CacheNone", enrichTask.Cache)
	}
	payload, ok := enrichTask.Payload.(runtime.EnrichDetailPayload)
	if !ok {
		t.Fatalf("Payload type = %T, want runtime.EnrichDetailPayload", enrichTask.Payload)
	}

	// Payload.Op must equal the constructed op verbatim (field-by-field,
	// not reflect.DeepEqual: resource.Resource carries map fields that
	// obscure which one differs on failure).
	if payload.Op.ID != op.ID {
		t.Errorf("Payload.Op.ID = %d, want %d", payload.Op.ID, op.ID)
	}
	if payload.Op.ResourceType != op.ResourceType {
		t.Errorf("Payload.Op.ResourceType = %q, want %q", payload.Op.ResourceType, op.ResourceType)
	}
	if payload.Op.Resource.ID != op.Resource.ID {
		t.Errorf("Payload.Op.Resource.ID = %q, want %q", payload.Op.Resource.ID, op.Resource.ID)
	}
	if payload.Op.Clients != op.Clients {
		t.Errorf("Payload.Op.Clients = %v, want %v", payload.Op.Clients, op.Clients)
	}
	if payload.Op.Refresh != op.Refresh {
		t.Errorf("Payload.Op.Refresh = %v, want %v", payload.Op.Refresh, op.Refresh)
	}

	if payload.DetailCtx == nil {
		t.Fatal("expected a non-nil DetailCtx (session.New() seeds PolicyDocCache/DetailDocCache)")
	}
	if payload.DetailCtx.SkipCache != op.Refresh {
		t.Errorf("DetailCtx.SkipCache = %v, want op.Refresh = %v", payload.DetailCtx.SkipCache, op.Refresh)
	}
	if payload.DetailCtx.OpID != op.ID {
		t.Errorf("DetailCtx.OpID = %d, want op.ID = %d", payload.DetailCtx.OpID, op.ID)
	}
	if payload.DetailCtx.Clients != op.Clients {
		t.Errorf("DetailCtx.Clients = %v, want op.Clients = %v", payload.DetailCtx.Clients, op.Clients)
	}
}
