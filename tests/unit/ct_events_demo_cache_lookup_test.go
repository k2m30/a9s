package unit_test

// ct_events_demo_cache_lookup_test.go — Bug E regression tests.
//
// Bug E: checkCtEventsUser, checkCtEventsRole, and other cache-reading checkers
// return Count=-1 in demo mode because ctEventsRelatedResources → FetchRelatedTarget
// short-circuits on nil clients when the cache is a miss.
//
// Exact bug flow:
//  1. In demo mode, def.Checker is called with nil clients and a partial cache
//     (m.resourceCache may not yet contain iam-user/role if those lists haven't
//     been loaded).
//  2. FetchRelatedTarget: cache miss → calls paginated fetcher with nil clients.
//  3. Paginated fetcher fails (nil clients) → error.
//  4. ctEventsRelatedResources sees the error, sees clients is not *ServiceClients,
//     returns nil, false, nil  ← the short-circuit.
//  5. Checker sees nil resourceList → returns Count=-1.
//
// Expected fix: when clients is nil (demo mode), the checker should
// populate the cache from demo.GetResources before falling through to the fetcher,
// OR FetchRelatedTarget should tolerate nil clients without erroring.
//
// Test approach:
//   - Pass an EMPTY cache (cache miss for the target type) and nil clients.
//   - The checker should return Count>=0 (0 if no match, >0 if matched) when the
//     target demos exist. Currently it returns Count=-1 (bug).
//
// Specifically:
//   - Case K (e-e1f2a3b4, AttachUserPolicy, alice.johnson): checkCtEventsUser
//     with empty cache + nil clients must NOT return Count=-1.
//   - AssumedRole events: checkCtEventsRole with empty cache + nil clients must
//     NOT return Count=-1.

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// TestCtEventsCheckersResolveFromDemoCache — Bug E regression
// ---------------------------------------------------------------------------

// TestCtEventsCheckersResolveFromDemoCache verifies that cache-backed checkers
// called with nil clients and an EMPTY cache do NOT return Count=-1.
//
// An empty cache simulates the real demo-mode scenario where ct-events detail is
// opened before iam-user/role resource lists have been loaded into m.resourceCache.
// The short-circuit in ctEventsRelatedResources returns nil on nil clients + error,
// which causes the checker to return Count=-1 (Bug E).
//
// The expected behavior is that the checker returns a definitive Count (0 or >0),
// not Count=-1, so the right column does not show the "unknown" state for a row
// that should have resolved.
func TestCtEventsCheckersResolveFromDemoCache(t *testing.T) {
	fixtures, ok := demo.GetResources("ct-events")
	if !ok || len(fixtures) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("resource.GetRelated(\"ct-events\") returned no defs — RegisterRelated not called?")
	}

	// Identify cache-backed target types (NeedsTargetCache == true, not self-pivot).
	// These are the types whose checkers call ctEventsRelatedResources and hit Bug E
	// when the cache is empty and clients is nil.
	cacheBackedTypes := make(map[string]bool)
	for _, def := range defs {
		if def.NeedsTargetCache && def.TargetType != "ct-events" {
			cacheBackedTypes[def.TargetType] = true
		}
	}

	if len(cacheBackedTypes) == 0 {
		t.Skip("no NeedsTargetCache defs registered for ct-events — Bug E test is vacuous")
	}

	// Use an EMPTY cache to simulate the demo scenario where target resource lists
	// haven't been loaded yet. This is the condition that triggers Bug E.
	emptyCache := make(resource.ResourceCache)

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			allResults := ctEventsRealCheckerResults(fixture, emptyCache)

			for _, result := range allResults {
				result := result
				if !cacheBackedTypes[result.TargetType] {
					continue
				}

				// Bug E: cache-backed checker with nil clients + EMPTY cache returns
				// Count=-1 (short-circuit fires). The fix should make it return 0
				// (no match) instead of -1 (unknown/error).
				//
				// We assert Count != -1 here. Currently this fails because the
				// short-circuit in ctEventsRelatedResources returns nil resourceList.
				if result.Count == -1 && len(result.FetchFilter) == 0 && result.Err == nil {
					t.Errorf("Bug E: event=%s targetType=%s: checker returned Count=-1 with nil clients"+
						" and empty cache — short-circuit ignores nil error from failed paginated fetcher."+
						" Expected Count=0 (no match) because nil-client fetcher should not be treated as unknown.",
						fixture.ID, result.TargetType)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCtEventsCheckersResolveFromDemoCache_CaseKUserChecker — pinned regression
// ---------------------------------------------------------------------------

// TestCtEventsCheckersResolveFromDemoCache_CaseKUserChecker pins the exact failure
// for Case K (e-e1f2a3b4, AttachUserPolicy, alice.johnson).
//
// With an empty cache and nil clients, checkCtEventsUser must NOT return Count=-1.
// Bug E: the short-circuit in ctEventsRelatedResources returns nil resourceList
// when clients is not *ServiceClients and FetchRelatedTarget errored.
func TestCtEventsCheckersResolveFromDemoCache_CaseKUserChecker(t *testing.T) {
	fixtures, ok := demo.GetResources("ct-events")
	if !ok || len(fixtures) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	var caseK resource.Resource
	var found bool
	for _, f := range fixtures {
		if f.ID == "e-e1f2a3b4" {
			caseK = f
			found = true
			break
		}
	}
	if !found {
		t.Skip("Case K fixture e-e1f2a3b4 not present in demo — skipping pinned regression")
	}

	if caseK.Fields["user"] != "alice.johnson" {
		t.Fatalf("Case K fixture user field=%q, want \"alice.johnson\"", caseK.Fields["user"])
	}

	// Empty cache — simulates the bug condition (iam-user not yet loaded).
	emptyCache := make(resource.ResourceCache)

	allResults := ctEventsRealCheckerResults(caseK, emptyCache)

	var iamUserResult *resource.RelatedCheckResult
	for i, r := range allResults {
		if r.TargetType == "iam-user" {
			cp := allResults[i]
			iamUserResult = &cp
			break
		}
	}
	if iamUserResult == nil {
		t.Fatal("no iam-user RelatedCheckResult returned for Case K — checker not registered?")
	}

	// Bug E: with nil clients + empty cache, the paginated fetcher fails, the
	// short-circuit fires, and the checker returns Count=-1. Expected: Count=0
	// (definitive "not found" because the fetcher should not report unknown on nil clients).
	if iamUserResult.Count == -1 && iamUserResult.Err == nil && len(iamUserResult.FetchFilter) == 0 {
		t.Errorf("Bug E pinned: event=e-e1f2a3b4 (AttachUserPolicy/alice.johnson):"+
			" checkCtEventsUser returned Count=-1 with nil clients and empty cache"+
			" — short-circuit in ctEventsRelatedResources discards nil error from failed fetcher."+
			" Expected Count=0 (no match, not unknown/error)."+
			" ResourceIDs=%v Err=%v FetchFilter=%v",
			iamUserResult.ResourceIDs, iamUserResult.Err, iamUserResult.FetchFilter)
	}
}

// ---------------------------------------------------------------------------
// TestCtEventsCheckersResolveFromDemoCache_RoleCheckerAssumedRoleEvents — pinned regression
// ---------------------------------------------------------------------------

// TestCtEventsCheckersResolveFromDemoCache_RoleCheckerAssumedRoleEvents verifies that
// checkCtEventsRole called with nil clients + empty cache does NOT return Count=-1
// for AssumedRole events.
//
// AssumedRole events are identified by a non-empty role_name field.
// Bug E: same short-circuit as checkCtEventsUser — nil-client fetcher error causes
// ctEventsRelatedResources to return nil, making checkCtEventsRole return Count=-1.
func TestCtEventsCheckersResolveFromDemoCache_RoleCheckerAssumedRoleEvents(t *testing.T) {
	fixtures, ok := demo.GetResources("ct-events")
	if !ok || len(fixtures) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// Empty cache — triggers the bug path.
	emptyCache := make(resource.ResourceCache)

	type bugECase struct {
		fixtureID string
		count     int
	}
	var bugECases []bugECase

	var hasAssumedRoleEvent bool
	for _, fixture := range fixtures {
		if fixture.Fields["role_name"] == "" {
			continue // not an AssumedRole event
		}
		hasAssumedRoleEvent = true

		allResults := ctEventsRealCheckerResults(fixture, emptyCache)
		for _, r := range allResults {
			if r.TargetType != "role" {
				continue
			}
			// Bug E: nil clients + empty cache → short-circuit → Count=-1.
			if r.Count == -1 && len(r.FetchFilter) == 0 && r.Err == nil {
				bugECases = append(bugECases, bugECase{
					fixtureID: fixture.ID,
					count:     r.Count,
				})
			}
		}
	}

	if !hasAssumedRoleEvent {
		t.Log("INFO: no AssumedRole fixtures (role_name field set) in ct-events demo data — test is vacuous")
	}

	for _, bc := range bugECases {
		bc := bc
		t.Run(bc.fixtureID, func(t *testing.T) {
			t.Errorf("Bug E: event=%s: checkCtEventsRole returned Count=-1 with nil clients"+
				" and empty cache — short-circuit in ctEventsRelatedResources ignores the nil"+
				" error from the failed paginated fetcher. Expected Count=0 (definitive no-match),"+
				" not Count=-1 (unknown).",
				bc.fixtureID)
		})
	}
}
