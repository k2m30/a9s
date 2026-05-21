// zzz_ct_events_all_related.go registers a CloudTrail Events RelatedDef for
// every resource type whose Related list is populated at init() time (i.e.
// types not yet migrated to the catalog struct literal). The zzz_ prefix
// ensures this file's init() runs after all per-type init() functions (Go runs
// init in source file name order within a package), so AppendRelated appends
// to already-registered slices.
//
// Types migrated under AS-795b–m carry their ct-events RelatedDef directly in
// their catalog_<category>.go struct literal; this file is a no-op for them
// because AppendRelated dedupes on TargetType.
package aws

import (
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
		resource.AppendRelated(shortName, resource.RelatedDef{
			TargetType:       "ct-events",
			DisplayName:      "CloudTrail Events",
			Checker:          ctEventsCheckerFor(shortName),
			NeedsTargetCache: false,
		})
	}
}
