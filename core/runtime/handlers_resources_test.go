// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// handlers_resources_test.go — Core-direct unit tests for the resource/detail
// Handle* methods, AllRegions and ResetRuleSets.
//
// Tests live in package runtime so they can exercise private fields
// (canonShortName, deriveFindingsForType internal helpers) without going
// through the adapter.
package runtime

import (
	"errors"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// findIntent returns the first intent of type T in xs, or the zero value
// and false.
func findIntent[T UIIntent](xs []UIIntent) (T, bool) {
	var zero T
	for _, x := range xs {
		if t, ok := x.(T); ok {
			return t, true
		}
	}
	return zero, false
}

// findIntents returns every intent of type T in xs.
func findIntents[T UIIntent](xs []UIIntent) []T {
	var out []T
	for _, x := range xs {
		if t, ok := x.(T); ok {
			out = append(out, t)
		}
	}
	return out
}

// hasTask returns true when xs contains a task whose Kind matches k.
func hasTask(xs []TaskRequest, k TaskKind, scope string) bool {
	for _, t := range xs {
		if t.Key.Kind == k && (scope == "" || t.Key.Scope == scope) {
			return true
		}
	}
	return false
}

func TestHandleResourcesLoaded_NotCachedYet_EmitsPatchResourceCache(t *testing.T) {
	sess := session.New()
	c := New(sess, catalog.All())
	rows := []resource.Resource{{ID: "i-001"}, {ID: "i-002"}}

	intents, _ := c.HandleResourcesLoaded(ResourcesLoadedEvent{
		ResourceType: "ec2",
		Resources:    rows,
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	patch, ok := findIntent[PatchResourceCache](intents)
	if !ok {
		t.Fatalf("expected PatchResourceCache intent, got %d intents", len(intents))
	}
	if patch.ResourceType != "ec2" {
		t.Errorf("PatchResourceCache.ResourceType = %q, want %q", patch.ResourceType, "ec2")
	}
	if patch.Entry == nil || len(patch.Entry.Resources) != 2 {
		t.Errorf("PatchResourceCache.Entry.Resources = %v, want 2 rows", patch.Entry)
	}
	if _, hasClear := findIntent[ClearFlash](intents); !hasClear {
		t.Errorf("expected ClearFlash intent, got %v", intents)
	}
}

// A later message (Append=false re-fetch) must not evict the richer entry
// the view-side cacheTopLevelResourceList wrote.
func TestHandleResourcesLoaded_AlreadyCached_SkipsPatch(t *testing.T) {
	sess := session.New()
	sess.RowStore.Observe("ec2", []resource.Resource{{ID: "pre-existing"}}, nil, session.OriginFetch, false)
	c := New(sess, catalog.All())

	intents, _ := c.HandleResourcesLoaded(ResourcesLoadedEvent{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-new"}},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if _, ok := findIntent[PatchResourceCache](intents); ok {
		t.Errorf("expected NO PatchResourceCache (alreadyCached guard), got intents=%v", intents)
	}
}

func TestHandleResourcesLoaded_PartialError_EmitsFlash(t *testing.T) {
	sess := session.New()
	c := New(sess, catalog.All())

	intents, _ := c.HandleResourcesLoaded(ResourcesLoadedEvent{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-001"}},
		Err:          errors.New("partial failure: one item timed out"),
	})

	flashes := findIntents[FlashIntent](intents)
	if len(flashes) == 0 {
		t.Fatal("expected at least one FlashIntent on partial-success Err")
	}
	if !flashes[0].IsError {
		t.Errorf("FlashIntent.IsError = false, want true")
	}
	if got, want := flashes[0].Text, "fetch ec2: partial failure: one item timed out"; got != want {
		t.Errorf("FlashIntent.Text = %q, want %q", got, want)
	}
}

func TestHandleResourcesLoaded_RerunTokenMatches_EmitsProbeTask(t *testing.T) {
	sess := session.New()
	sess.EnrichmentTypeGen["ec2"] = 7
	c := New(sess, catalog.All())
	rows := []resource.Resource{{ID: "i-001"}}

	_, tasks := c.HandleResourcesLoaded(ResourcesLoadedEvent{
		ResourceType: "ec2",
		Resources:    rows,
		Pagination:   &resource.PaginationMeta{IsTruncated: true},
		TypeGen:      7,
	})

	if !hasTask(tasks, TaskKindProbeEnrich, "ec2") {
		t.Fatalf("expected TaskKindProbeEnrich for ec2, got %d tasks", len(tasks))
	}
	tr := sess.RowStore.Snapshot("ec2")
	if len(tr.Rows) != 1 || tr.Rows[0].ID != "i-001" {
		t.Errorf("RowStore.Snapshot(\"ec2\").Rows = %v, want one row with ID i-001", tr.Rows)
	}
	if tr.Pagination == nil || !tr.Pagination.IsTruncated {
		t.Errorf("RowStore.Snapshot(\"ec2\").Pagination.IsTruncated = %v, want true", tr.Pagination)
	}
}

func TestHandleResourcesLoaded_RerunTokenStale_NoProbeTask(t *testing.T) {
	sess := session.New()
	sess.EnrichmentTypeGen["ec2"] = 7
	c := New(sess, catalog.All())

	_, tasks := c.HandleResourcesLoaded(ResourcesLoadedEvent{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-001"}},
		TypeGen:      3, // stale: doesn't match the current 7
	})

	if hasTask(tasks, TaskKindProbeEnrich, "ec2") {
		t.Errorf("expected NO probe task for stale TypeGen, got %d tasks", len(tasks))
	}
	if tr := sess.RowStore.Snapshot("ec2"); tr.Gen != 0 {
		t.Errorf("RowStore.Snapshot(\"ec2\") should not be observed on stale gen, got Gen=%d Rows=%v", tr.Gen, tr.Rows)
	}
}

// The adapter shim short-circuits on Err so the view-side
// derive + updateActiveView never sees a half-populated EnrichedRes.
func TestHandleEnrichDetailResult_Err_EmitsFlash(t *testing.T) {
	sess := session.New()
	c := New(sess, catalog.All())

	intents, tasks := c.HandleEnrichDetailResult(EnrichDetailResultEvent{
		ResourceType: "ec2",
		Err:          errors.New("AccessDenied: iam:GetPolicyVersion"),
	})

	if len(tasks) != 0 {
		t.Errorf("expected no tasks on Err, got %d", len(tasks))
	}
	flashes := findIntents[FlashIntent](intents)
	if len(flashes) != 1 {
		t.Fatalf("expected exactly 1 FlashIntent, got %d", len(flashes))
	}
	if !flashes[0].IsError {
		t.Errorf("FlashIntent.IsError = false, want true")
	}
	// The one formatter names what failed the way every other failed call
	// does, without a redundant "failed" — the cause already says it failed.
	if got, want := flashes[0].Text, "enrich ec2: AccessDenied: iam:GetPolicyVersion"; got != want {
		t.Errorf("FlashIntent.Text = %q, want %q", got, want)
	}
}

func TestHandleEnrichDetailResult_NoErr_NoIntents(t *testing.T) {
	sess := session.New()
	c := New(sess, catalog.All())

	intents, tasks := c.HandleEnrichDetailResult(EnrichDetailResultEvent{
		ResourceType: "ec2",
	})
	if len(intents) != 0 || len(tasks) != 0 {
		t.Errorf("expected (nil, nil) on success path, got intents=%v tasks=%v", intents, tasks)
	}
}

func TestHandleRelatedCheckResult_AppendsRelatedCache(t *testing.T) {
	sess := session.New()
	c := New(sess, catalog.All())

	intents, _ := c.HandleRelatedCheckResult(RelatedCheckResultEvent{
		ResourceType:     "ec2",
		SourceResourceID: "i-abc",
		DefDisplayName:   "SecurityGroups",
		Result:           resource.KnownRelated("sg", nil, false),
	})

	patch, ok := findIntent[PatchRelatedCache](intents)
	if !ok {
		t.Fatalf("expected PatchRelatedCache intent, got %d intents", len(intents))
	}
	if patch.ResourceType != "ec2" || patch.SourceID != "i-abc" {
		t.Errorf("PatchRelatedCache key fields = (%q, %q), want (ec2, i-abc)", patch.ResourceType, patch.SourceID)
	}
	if patch.DefDisplayName != "SecurityGroups" {
		t.Errorf("PatchRelatedCache.DefDisplayName = %q, want SecurityGroups", patch.DefDisplayName)
	}
}

func TestHandleRelatedCheckResult_LazyAddError_EmitsFlash(t *testing.T) {
	sess := session.New()
	c := New(sess, catalog.All())

	intents, _ := c.HandleRelatedCheckResult(RelatedCheckResultEvent{
		ResourceType:     "ec2",
		SourceResourceID: "i-abc",
		LazyAddError:     errors.New("FetchByIDs: throttled"),
	})

	flashes := findIntents[FlashIntent](intents)
	if len(flashes) == 0 {
		t.Fatal("expected at least one FlashIntent on LazyAddError")
	}
	if !flashes[0].IsError {
		t.Errorf("FlashIntent.IsError = false, want true")
	}
}

func TestHandleRelatedCheckResult_CachedPagesMerge(t *testing.T) {
	sess := session.New()
	// Pre-seed a Partial RowStore entry for "kms" so the matching CachedPages
	// entry is skipped (Gen!=0 is the "already has an entry" test).
	sess.RowStore.ObservePartial("kms", []resource.Resource{{ID: "pre-lazy"}})
	c := New(sess, catalog.All())

	intents, _ := c.HandleRelatedCheckResult(RelatedCheckResultEvent{
		ResourceType:     "ec2",
		SourceResourceID: "i-src",
		CachedPages: map[string]resource.ResourceCacheEntry{
			"sg":  {Resources: []resource.Resource{{ID: "sg-1"}}},
			"kms": {Resources: []resource.Resource{{ID: "key-1"}}}, // should skip
		},
	})

	caches := findIntents[PatchResourceCache](intents)
	if len(caches) != 1 {
		t.Fatalf("expected exactly 1 PatchResourceCache (sg only), got %d", len(caches))
	}
	if caches[0].ResourceType != "sg" {
		t.Errorf("PatchResourceCache.ResourceType = %q, want sg", caches[0].ResourceType)
	}
}

func TestHandleIdentityLoaded_EmitsSetIdentityAndHeaderInvalidate(t *testing.T) {
	sess := session.New()
	sess.IdentityFetching = true
	c := New(sess, catalog.All())

	awsID := &awsclient.CallerIdentity{
		AccountID:     "123456789012",
		AccountAlias:  "prod",
		Arn:           "arn:aws:iam::123456789012:user/alice",
		UserName:      "alice",
		IdentityName:  "alice",
		IsAssumedRole: false,
	}

	intents, _ := c.HandleIdentityLoaded(IdentityLoadedEvent{Identity: awsID})

	if sess.IdentityFetching {
		t.Errorf("IdentityFetching = true after HandleIdentityLoaded, want false")
	}
	if sess.Identity != awsID {
		t.Errorf("session.Identity not set; got %v", sess.Identity)
	}
	set, ok := findIntent[SetIdentityIntent](intents)
	if !ok {
		t.Fatalf("expected SetIdentityIntent intent")
	}
	if set.Identity == nil || set.Identity.AccountID != "123456789012" {
		t.Errorf("SetIdentityIntent.Identity.AccountID = %v, want 123456789012", set.Identity)
	}
	if set.Identity.AccountAlias != "prod" {
		t.Errorf("SetIdentityIntent.Identity.AccountAlias = %q, want prod", set.Identity.AccountAlias)
	}
	if _, ok := findIntent[HeaderInvalidateIntent](intents); !ok {
		t.Errorf("expected HeaderInvalidateIntent intent")
	}
}

func TestHandleIdentityLoaded_WrongType_OnlyClearsFetching(t *testing.T) {
	sess := session.New()
	sess.IdentityFetching = true
	c := New(sess, catalog.All())

	intents, tasks := c.HandleIdentityLoaded(IdentityLoadedEvent{Identity: "not-an-identity"})

	if sess.IdentityFetching {
		t.Errorf("IdentityFetching not cleared on wrong-typed payload")
	}
	if len(intents) != 0 || len(tasks) != 0 {
		t.Errorf("expected nil intents+tasks on wrong-typed payload, got %v %v", intents, tasks)
	}
	if sess.Identity != nil {
		t.Errorf("session.Identity should remain nil on wrong-typed payload, got %v", sess.Identity)
	}
}

func TestHandleIdentityError_ClearsFetching(t *testing.T) {
	sess := session.New()
	sess.IdentityFetching = true
	c := New(sess, catalog.All())

	intents, tasks := c.HandleIdentityError(IdentityErrorEvent{Err: "AccessDenied"})

	if sess.IdentityFetching {
		t.Errorf("IdentityFetching not cleared")
	}
	if len(intents) != 0 || len(tasks) != 0 {
		t.Errorf("expected nil intents+tasks, got %v %v", intents, tasks)
	}
}

func TestResetRuleSets_SwapsStoreAndRewiresClients(t *testing.T) {
	sess := session.New()
	oldStore := sess.RuleSets
	c := New(sess, catalog.All())

	c.ResetRuleSets()

	if sess.RuleSets == oldStore {
		t.Errorf("ResetRuleSets did not swap session.RuleSets")
	}
	if sess.RuleSets == nil {
		t.Errorf("ResetRuleSets left session.RuleSets nil")
	}
}

func TestAllRegions_ReturnsCommercialPartition(t *testing.T) {
	sess := session.New()
	c := New(sess, catalog.All())

	regions := c.AllRegions()
	if len(regions) == 0 {
		t.Fatal("AllRegions returned empty slice")
	}
	// Sanity: us-east-1 is in the commercial partition.
	var found bool
	for _, r := range regions {
		if r.Code == "us-east-1" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("us-east-1 not present in AllRegions output")
	}
}
