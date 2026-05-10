package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// TestCoreHandleEnrichDetail_NoEnricher_ReturnsNilNil verifies that the
// runtime returns no UIIntents and no TaskRequests for a resource type
// with no registered detail enricher. This pins the SSOT contract: the
// runtime is the single decision-maker for the dispatch gate, so the
// adapter does not need to re-check enricher existence.
func TestCoreHandleEnrichDetail_NoEnricher_ReturnsNilNil(t *testing.T) {
	if resource.HasDetailEnricher("ec2") {
		t.Skip("ec2 now has a detail enricher — pick a different no-enricher type")
	}
	core := runtime.New(session.New(), resource.AllResourceTypes())
	intents, tasks := core.HandleEnrichDetail(runtime.EnrichDetailEvent{
		ResourceType: "ec2",
		Resource:     resource.Resource{ID: "i-1234567890abcdef0", Name: "no-enricher"},
	})
	if intents != nil {
		t.Errorf("expected nil intents, got %d", len(intents))
	}
	if tasks != nil {
		t.Errorf("expected nil tasks, got %d", len(tasks))
	}
}

// TestCoreHandleEnrichDetail_WithEnricher_EmitsTaskRequest verifies the
// shape of the TaskRequest emitted for a resource type with a registered
// detail enricher: kind, scope, cache policy, and typed Payload all
// match the contract the adapter type-switches on.
func TestCoreHandleEnrichDetail_WithEnricher_EmitsTaskRequest(t *testing.T) {
	if !resource.HasDetailEnricher("role_policies") {
		t.Fatal("expected role_policies detail enricher to be registered")
	}
	res := resource.Resource{
		ID:   "arn:aws:iam::123456789012:policy/runtime-test",
		Name: "runtime-test",
	}
	core := runtime.New(session.New(), resource.AllResourceTypes())
	intents, tasks := core.HandleEnrichDetail(runtime.EnrichDetailEvent{
		ResourceType: "role_policies",
		Resource:     res,
	})
	if intents != nil {
		t.Errorf("expected nil intents (no UI patches at dispatch time), got %d", len(intents))
	}
	if got := len(tasks); got != 1 {
		t.Fatalf("expected exactly 1 task, got %d", got)
	}
	task := tasks[0]
	if task.Key.Kind != runtime.KindEnrichDetail {
		t.Errorf("Key.Kind = %q, want %q", task.Key.Kind, runtime.KindEnrichDetail)
	}
	if want := "role_policies/" + res.ID; task.Key.Scope != want {
		t.Errorf("Key.Scope = %q, want %q", task.Key.Scope, want)
	}
	if task.Cache != runtime.CacheNone {
		t.Errorf("Cache = %v, want CacheNone", task.Cache)
	}
	payload, ok := task.Payload.(runtime.EnrichDetailPayload)
	if !ok {
		t.Fatalf("Payload type = %T, want runtime.EnrichDetailPayload", task.Payload)
	}
	if payload.ResourceType != "role_policies" {
		t.Errorf("Payload.ResourceType = %q, want %q", payload.ResourceType, "role_policies")
	}
	if payload.Resource.ID != res.ID {
		t.Errorf("Payload.Resource.ID = %q, want %q", payload.Resource.ID, res.ID)
	}
}
