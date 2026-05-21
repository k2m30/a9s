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
	// Iterate the package-local catalog data slices (initialized by Go before
	// init() runs). We cannot use resource.AllShortNames here because that
	// would call catalog.AllShortNames, which panics until aws.Install runs in
	// main() / TestMain — and main() runs after init(). The catalog data
	// already lives in this package post-AS-795a, so iterate it directly.
	for _, rt := range allTopLevelTypes() {
		shortName := rt.ShortName
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
