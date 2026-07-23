package unit

// detail_doc_cache_test.go — coverage for awsclient.DetailDocCache
// (core/aws/detail_doc_cache.go), the session-scoped cache backing the
// sfn/cfn on-demand detail enrichers (#261). Mirrors the PolicyDocumentCache
// coverage in app_enrich_test.go (TestPolicyDocCache_ZeroValueSafe et al.).
//
// Covers:
//   - zero-value cache: Get on an unset key returns nil, Set does not panic
//   - Set/Get roundtrip for a single key
//   - overwrite: a second Set for the same key replaces the first value
//   - distinct keys (sfn:/cfn: prefixes) do not collide

import (
	"fmt"
	"sync"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

func TestDetailDocCache_ZeroValueSafe(t *testing.T) {
	var cache awsclient.DetailDocCache

	if got := cache.Get("nonexistent"); got != nil {
		t.Fatalf("expected nil from zero-value cache, got %v", got)
	}

	// Set on a zero-value cache must not panic.
	cache.Set("sfn:arn:aws:states:us-east-1:123456789012:stateMachine:order-processing", "value")
	if got := cache.Get("sfn:arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"); got != "value" {
		t.Fatalf("expected %q after Set on zero-value cache, got %v", "value", got)
	}
}

func TestDetailDocCache_SetGetRoundtrip(t *testing.T) {
	var cache awsclient.DetailDocCache
	key := "sfn:arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	doc := map[string]any{"Comment": "Order processing workflow", "StartAt": "ValidateOrder"}

	cache.Set(key, doc)

	got := cache.Get(key)
	if got == nil {
		t.Fatal("expected cache hit after Set, got nil")
	}
	gotDoc, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("Get returned %T, want map[string]any", got)
	}
	if gotDoc["Comment"] != "Order processing workflow" {
		t.Errorf("gotDoc[Comment] = %v, want %q", gotDoc["Comment"], "Order processing workflow")
	}
}

func TestDetailDocCache_Overwrite_ReplacesPreviousValue(t *testing.T) {
	var cache awsclient.DetailDocCache
	key := "cfn:arn:aws:cloudformation:us-east-1:123456789012:stack/prod-vpc-network/abcd1234"

	cache.Set(key, "AWSTemplateFormatVersion: '2010-09-09'\n")
	cache.Set(key, map[string]any{"AWSTemplateFormatVersion": "2010-09-09"})

	got := cache.Get(key)
	if _, stillRaw := got.(string); stillRaw {
		t.Fatalf("Get returned the stale raw-string value %v, want the overwritten parsed value", got)
	}
	gotDoc, ok := got.(map[string]any)
	if !ok || gotDoc["AWSTemplateFormatVersion"] != "2010-09-09" {
		t.Errorf("Get after overwrite = %v, want the second Set's value", got)
	}
}

func TestDetailDocCache_DistinctKeys_DoNotCollide(t *testing.T) {
	var cache awsclient.DetailDocCache

	sfnKey := "sfn:arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	cfnKey := "cfn:prod-vpc-network"

	cache.Set(sfnKey, map[string]any{"kind": "sfn-definition"})
	cache.Set(cfnKey, "AWSTemplateFormatVersion: '2010-09-09'\n")

	sfnDoc, ok := cache.Get(sfnKey).(map[string]any)
	if !ok || sfnDoc["kind"] != "sfn-definition" {
		t.Errorf("cache.Get(sfnKey) = %v, want the sfn document", cache.Get(sfnKey))
	}
	cfnDoc, ok := cache.Get(cfnKey).(string)
	if !ok || cfnDoc != "AWSTemplateFormatVersion: '2010-09-09'\n" {
		t.Errorf("cache.Get(cfnKey) = %v, want the cfn template", cache.Get(cfnKey))
	}
}

func TestDetailDocCache_DifferentInstances_DoNotShareCache(t *testing.T) {
	var cache1, cache2 awsclient.DetailDocCache

	cache1.Set("sfn:shared-key", "from-instance-1")

	if got := cache2.Get("sfn:shared-key"); got != nil {
		t.Fatalf("different DetailDocCache instances must not share data, got %v", got)
	}
}

// TestDetailDocCache_ConcurrentAccess_NoRace hammers a single DetailDocCache
// from many goroutines doing overlapping Set/Get on both a distinct
// per-goroutine key and one key shared by every goroutine, joined via
// WaitGroup. Designed to always pass (values are always ints, so the type
// assertions below never fail on a torn write) while exercising the
// RWMutex under `-race` — a regression that narrowed or dropped the lock
// would show up as a race detector failure here, not as an assertion
// failure.
func TestDetailDocCache_ConcurrentAccess_NoRace(t *testing.T) {
	var cache awsclient.DetailDocCache
	const goroutines = 20
	const itersPerGoroutine = 50
	const sharedKey = "cfn:shared"

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			distinctKey := fmt.Sprintf("sfn:goroutine-%d", id)
			for i := 0; i < itersPerGoroutine; i++ {
				// Distinct key: only this goroutine ever writes it.
				cache.Set(distinctKey, i)
				cache.Get(distinctKey)

				// Shared key: every goroutine reads and writes it concurrently.
				cache.Set(sharedKey, id)
				cache.Get(sharedKey)
			}
		}(g)
	}
	wg.Wait()

	// Every goroutine's distinct key must hold ITS last-written value — no
	// cross-goroutine corruption of the underlying map.
	for g := 0; g < goroutines; g++ {
		distinctKey := fmt.Sprintf("sfn:goroutine-%d", g)
		got := cache.Get(distinctKey)
		if gotInt, ok := got.(int); !ok || gotInt != itersPerGoroutine-1 {
			t.Errorf("cache.Get(%q) = %v, want %d (this goroutine's last write)", distinctKey, got, itersPerGoroutine-1)
		}
	}

	// The shared key is last-writer-wins under concurrency — the assertion is
	// only that it holds a valid goroutine id (a clean, untorn int), not a
	// specific writer.
	shared := cache.Get(sharedKey)
	sharedInt, ok := shared.(int)
	if !ok || sharedInt < 0 || sharedInt >= goroutines {
		t.Errorf("cache.Get(sharedKey) = %v (type ok=%v), want a valid goroutine id in [0,%d)", shared, ok, goroutines)
	}
}
