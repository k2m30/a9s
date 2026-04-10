// zzz_ct_events_all_related.go registers a CloudTrail Events RelatedDef for
// every resource type. The zzz_ prefix ensures this file's init() runs after
// all per-type init() functions (Go runs init in source file name order within
// a package), so AppendRelated appends to already-registered slices.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	for _, shortName := range resource.AllShortNames() {
		if shortName == "ct-events" {
			continue // don't add self-reference
		}
		sn := shortName // capture for closure
		checker := func(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
			filter := resource.BuildCloudTrailFilter(res, sn)
			if filter == nil {
				return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
			}
			return resource.RelatedCheckResult{
				TargetType:  "ct-events",
				Count:       -1,
				FetchFilter: filter,
			}
		}
		resource.AppendRelated(shortName, resource.RelatedDef{
			TargetType:       "ct-events",
			DisplayName:      "CloudTrail Events",
			Checker:          checker,
			NeedsTargetCache: false,
		})
	}
}
