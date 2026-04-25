// ct_events_pivot.go — factory for CloudTrail events pivot checkers.
// Eliminates the copy-paste pattern shared by checkDbcCTEvents,
// checkDbcSnapCTEvents, and checkDBISnapCTEvents.
package aws

import (
	"context"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// CTEventsPivotConfig parameterises BuildCTEventsPivotChecker.
// IDExtractor derives the resource identifier used to match CloudTrail event
// Resources[].ResourceName entries. It is called once per checker invocation.
type CTEventsPivotConfig struct {
	// IDExtractor returns the string used to match against CloudTrail event
	// Resources[].ResourceName (and the Fields["resource_name"] text fallback).
	// Return "" to short-circuit the checker with Count=0 immediately.
	IDExtractor func(res resource.Resource) string
}

// BuildCTEventsPivotChecker returns a resource.RelatedChecker that:
//  1. Extracts the resource ID via cfg.IDExtractor.
//  2. Returns Count=0 immediately when the extracted ID is empty.
//  3. Reads the "ct-events" entry from the cache (or fetches the first page via
//     FetchRelatedTarget when the entry is absent).
//  4. Returns Count=-1 when the event list cannot be loaded (error, nil list).
//  5. Returns Count=-1 when the cache is truncated (partial window — actual count
//     may exceed what is visible).
//  6. Counts events that reference the extracted ID via typed
//     cloudtrailtypes.Event.Resources[].ResourceName (authoritative path) or the
//     Fields["resource_name"] text fallback for resources without a typed RawStruct.
//  7. Sets FetchFilter["ResourceName"] on the result so callers can issue a
//     server-side filtered re-fetch on drill-in navigation.
func BuildCTEventsPivotChecker(cfg CTEventsPivotConfig) resource.RelatedChecker {
	return func(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
		id := cfg.IDExtractor(res)
		if id == "" {
			return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
		}

		fetchFilter := map[string]string{"ResourceName": id}

		eventList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "ct-events")
		if err != nil {
			return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, Err: err, FetchFilter: fetchFilter}
		}
		if eventList == nil {
			return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, FetchFilter: fetchFilter}
		}

		var ids []string
		for _, eventRes := range eventList {
			// When a typed cloudtrail Event is present, its Resources slice is
			// authoritative — the Fields["resource_name"] fallback below is only
			// for resources without a typed RawStruct (test helpers, demo shortcuts).
			// If the typed slice exists and contains no match for id, the event
			// genuinely doesn't reference this resource; don't second-guess via text fallback.
			if raw, ok := assertStruct[cloudtrailtypes.Event](eventRes.RawStruct); ok {
				for _, rr := range raw.Resources {
					if rr.ResourceName != nil && *rr.ResourceName == id {
						ids = append(ids, eventRes.ID)
						break
					}
				}
				continue
			}
			if eventRes.Fields["resource_name"] == id {
				ids = append(ids, eventRes.ID)
			}
		}

		if truncated {
			return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, FetchFilter: fetchFilter}
		}

		result := relatedResult("ct-events", ids)
		result.FetchFilter = fetchFilter
		return result
	}
}
