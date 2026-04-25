package unit

// ct_events_pivot_test.go — Regression pins for Issue 6 (P2):
// BuildCTEventsPivotChecker factory function does not exist yet.
//
// Bug / gap: every ct-events related checker (checkDbcCTEvents,
// checkDbcSnapCTEvents, checkIAMUserCtEvents, checkECSTaskCTEvents …)
// duplicates the same pattern:
//   1. Extract the resource ID.
//   2. Fetch ct-events from cache (or first page on cache miss).
//   3. Iterate events, matching via typed cloudtrailtypes.Event.Resources[]
//      (authoritative) with a Fields["resource_name"] text fallback.
//   4. Return Count=-1 when cache is truncated or errored; Count=N otherwise.
//
// The fix introduces a BuildCTEventsPivotChecker factory in internal/aws that
// parameterizes this pattern so future resource types can register a ct-events
// pivot checker without copy-paste.
//
// These tests COMPILE-FAIL today because awsclient.BuildCTEventsPivotChecker
// and awsclient.CTEventsPivotConfig do not exist yet.
//
// Test strategy: build a checker via the factory, supply minimal resource +
// cache objects, and assert on the returned RelatedCheckResult.Count.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ctPivotCacheWith returns a ResourceCache with a pre-populated ct-events entry.
// truncated controls whether the cache entry is IsTruncated.
func ctPivotCacheWith(events []resource.Resource, truncated bool) resource.ResourceCache {
	return resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{
			Resources:   events,
			IsTruncated: truncated,
		},
	}
}

// ctPivotBuildEvent builds a ct-events Resource whose typed RawStruct
// references the given resourceName.
func ctPivotBuildEvent(eventID, resourceName string) resource.Resource {
	return resource.Resource{
		ID:   eventID,
		Name: eventID,
		Fields: map[string]string{
			"resource_name": resourceName,
		},
		RawStruct: cloudtrailtypes.Event{
			EventId: aws.String(eventID),
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceName: aws.String(resourceName),
				},
			},
		},
	}
}

// ctPivotBuildTextEvent builds a ct-events Resource with NO typed RawStruct —
// only the Fields["resource_name"] text field for fallback matching.
func ctPivotBuildTextEvent(eventID, resourceName string) resource.Resource {
	return resource.Resource{
		ID:   eventID,
		Name: eventID,
		Fields: map[string]string{
			"resource_name": resourceName,
		},
		// RawStruct intentionally nil — exercises the text-fallback branch.
	}
}

// ctPivotSrcResource builds a source resource with the given ID.
func ctPivotSrcResource(id string) resource.Resource {
	return resource.Resource{ID: id, Name: id}
}

// ---------------------------------------------------------------------------
// Tests — COMPILE-FAIL today: BuildCTEventsPivotChecker does not exist.
// ---------------------------------------------------------------------------

// TestBuildCTEventsPivotChecker_EmptyID_ReturnsZero verifies that when the
// IDExtractor returns an empty string (resource has no usable identifier),
// the checker returns Count=0 immediately without scanning the cache.
//
// COMPILE-FAIL today: awsclient.BuildCTEventsPivotChecker undefined.
func TestBuildCTEventsPivotChecker_EmptyID_ReturnsZero(t *testing.T) {
	checker := awsclient.BuildCTEventsPivotChecker(awsclient.CTEventsPivotConfig{
		IDExtractor: func(_ resource.Resource) string { return "" },
	})

	res := ctPivotSrcResource("")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf(
			"BuildCTEventsPivotChecker: empty ID: Count = %d, want 0 — "+
				"CT-EVENTS-PIVOT: empty ID must short-circuit to zero without scanning",
			result.Count,
		)
	}
	if result.TargetType != "ct-events" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "ct-events")
	}
}

// TestBuildCTEventsPivotChecker_TypedEventMatches verifies that when the
// ct-events cache contains an event whose typed RawStruct.Resources[].ResourceName
// matches the extracted ID, that event is counted.
//
// COMPILE-FAIL today: awsclient.BuildCTEventsPivotChecker undefined.
func TestBuildCTEventsPivotChecker_TypedEventMatches(t *testing.T) {
	const snapID = "my-snap-001"

	checker := awsclient.BuildCTEventsPivotChecker(awsclient.CTEventsPivotConfig{
		IDExtractor: func(r resource.Resource) string { return r.ID },
	})

	matchEvent := ctPivotBuildEvent("event-aaa", snapID)
	noMatchEvent := ctPivotBuildEvent("event-bbb", "other-resource")

	cache := ctPivotCacheWith([]resource.Resource{matchEvent, noMatchEvent}, false)
	res := ctPivotSrcResource(snapID)

	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf(
			"BuildCTEventsPivotChecker: typed match: Count = %d, want 1 — "+
				"CT-EVENTS-PIVOT: typed event matching Resources[].ResourceName must count matching events",
			result.Count,
		)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "event-aaa" {
		t.Errorf("ResourceIDs = %v, want [event-aaa]", result.ResourceIDs)
	}
}

// TestBuildCTEventsPivotChecker_TextFallback_NoTypedStruct verifies that when
// an event resource has no typed RawStruct (only Fields["resource_name"]),
// the text-fallback branch correctly matches the event.
//
// COMPILE-FAIL today: awsclient.BuildCTEventsPivotChecker undefined.
func TestBuildCTEventsPivotChecker_TextFallback_NoTypedStruct(t *testing.T) {
	const snapID = "text-fallback-snap"

	checker := awsclient.BuildCTEventsPivotChecker(awsclient.CTEventsPivotConfig{
		IDExtractor: func(r resource.Resource) string { return r.ID },
	})

	textEvent := ctPivotBuildTextEvent("event-text-001", snapID)
	cache := ctPivotCacheWith([]resource.Resource{textEvent}, false)
	res := ctPivotSrcResource(snapID)

	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf(
			"BuildCTEventsPivotChecker: text fallback: Count = %d, want 1 — "+
				"CT-EVENTS-PIVOT: text-fallback via Fields[resource_name] must match "+
				"events without typed RawStruct",
			result.Count,
		)
	}
}

// TestBuildCTEventsPivotChecker_Truncated_ReturnsMinusOne verifies that when
// the ct-events cache is truncated (IsTruncated=true), the checker returns
// Count=-1 regardless of matching events in the visible window.
//
// COMPILE-FAIL today: awsclient.BuildCTEventsPivotChecker undefined.
func TestBuildCTEventsPivotChecker_Truncated_ReturnsMinusOne(t *testing.T) {
	const snapID = "trunc-snap"

	checker := awsclient.BuildCTEventsPivotChecker(awsclient.CTEventsPivotConfig{
		IDExtractor: func(r resource.Resource) string { return r.ID },
	})

	// Cache contains a matching event but is truncated.
	matchEvent := ctPivotBuildEvent("event-trunc-001", snapID)
	cache := ctPivotCacheWith([]resource.Resource{matchEvent}, true /* truncated */)
	res := ctPivotSrcResource(snapID)

	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf(
			"BuildCTEventsPivotChecker: truncated cache: Count = %d, want -1 — "+
				"CT-EVENTS-PIVOT: truncated cache means partial window; count is unknown, not positive",
			result.Count,
		)
	}
}

// TestBuildCTEventsPivotChecker_NilCacheList_ReturnsMinusOne verifies that
// when the ct-events type is absent from the cache entirely (nil list), the
// checker returns Count=-1 (cannot determine without ct-events loaded).
//
// COMPILE-FAIL today: awsclient.BuildCTEventsPivotChecker undefined.
func TestBuildCTEventsPivotChecker_NilCacheList_ReturnsMinusOne(t *testing.T) {
	checker := awsclient.BuildCTEventsPivotChecker(awsclient.CTEventsPivotConfig{
		IDExtractor: func(r resource.Resource) string { return r.ID },
	})

	res := ctPivotSrcResource("some-resource")
	// Cache has no ct-events entry at all.
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf(
			"BuildCTEventsPivotChecker: absent cache entry: Count = %d, want -1 — "+
				"CT-EVENTS-PIVOT: ct-events not loaded in cache; count is unknown",
			result.Count,
		)
	}
}

// TestBuildCTEventsPivotChecker_CacheError_ReturnsMinusOne verifies that when
// the registered ct-events fetcher returns an error (cache miss + fetch error),
// the checker returns Count=-1 with the error propagated.
//
// COMPILE-FAIL today: awsclient.BuildCTEventsPivotChecker undefined.
// Note: this test exercises the error path by passing a non-nil clients object
// that is NOT a *ServiceClients (wrong type), which FetchRelatedTarget treats
// as a non-AWS client error — the checker should surface Count=-1.
func TestBuildCTEventsPivotChecker_CacheError_ReturnsMinusOne(t *testing.T) {
	checker := awsclient.BuildCTEventsPivotChecker(awsclient.CTEventsPivotConfig{
		IDExtractor: func(r resource.Resource) string { return r.ID },
	})

	res := ctPivotSrcResource("err-resource")
	// Empty cache forces FetchRelatedTarget; passing a non-ServiceClients
	// clients value triggers the clients-type guard in FetchRelatedTarget,
	// which returns (nil, false, nil) — the checker then returns Count=-1.
	result := checker(context.Background(), struct{}{}, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf(
			"BuildCTEventsPivotChecker: cache miss + bad clients: Count = %d, want -1 — "+
				"CT-EVENTS-PIVOT: unavailable ct-events must yield unknown count, not positive",
			result.Count,
		)
	}
}

// TestBuildCTEventsPivotChecker_Wired_DbcSnap verifies that the factory-built
// checker produces the expected matching result for the dbc-snap use case:
// IDExtractor = res.ID (snapshot identifier), matching against event
// Resources[].ResourceName.
//
// This test exercises the full factory-to-behavior path for the dbc-snap
// scenario and acts as an integration pin for the factory's design contract.
//
// COMPILE-FAIL today: awsclient.BuildCTEventsPivotChecker undefined.
func TestBuildCTEventsPivotChecker_Wired_DbcSnap(t *testing.T) {
	const snapID = "rds:acme-prod-cluster-2026-04-01"

	checker := awsclient.BuildCTEventsPivotChecker(awsclient.CTEventsPivotConfig{
		IDExtractor: func(r resource.Resource) string { return r.ID },
	})

	matchEvent1 := ctPivotBuildEvent("ct-event-snap-001", snapID)
	matchEvent2 := ctPivotBuildEvent("ct-event-snap-002", snapID)
	noMatchEvent := ctPivotBuildEvent("ct-event-other-001", "rds:other-cluster-snap")

	cache := ctPivotCacheWith(
		[]resource.Resource{matchEvent1, noMatchEvent, matchEvent2},
		false, /* not truncated */
	)
	res := ctPivotSrcResource(snapID)

	result := checker(context.Background(), nil, res, cache)

	if result.Count != 2 {
		t.Errorf(
			"BuildCTEventsPivotChecker (dbc-snap scenario): Count = %d, want 2 — "+
				"CT-EVENTS-PIVOT (Wired_DbcSnap): factory must count all events "+
				"referencing the snapshot ID via Resources[].ResourceName",
			result.Count,
		)
	}
	if result.TargetType != "ct-events" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "ct-events")
	}
	// FetchFilter["ResourceName"] must be set so the caller can do a filtered re-fetch.
	if result.FetchFilter["ResourceName"] != snapID {
		t.Errorf(
			"FetchFilter[ResourceName] = %q, want %q — "+
				"CT-EVENTS-PIVOT: FetchFilter must be populated for filtered re-fetch",
			result.FetchFilter["ResourceName"], snapID,
		)
	}
}
