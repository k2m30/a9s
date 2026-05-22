// s3_objects.go — child-type registration for S3 Objects (the per-bucket
// drill-down). Split out of s3.go in AS-817 so s3.go can have its init() body
// removed (per the data-category init()→catalog migration). The child-type
// migration to catalog.SetChildTypes lands in AS-795n (the Wave 2 / child
// infrastructure sweep); until then this init() body keeps the s3_objects
// child type registered via the legacy internal/resource registries — same
// rationale as asg_activities.go / ecs_svc_logs.go / etc.
package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("s3_objects", []string{
		"key",
		"size",
		"last_modified",
		"storage_class",
	})

	resource.RegisterPaginatedChild("s3_objects", func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchS3Objects(ctx, c.S3, parentCtx["bucket"], parentCtx["prefix"], continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "s3_objects",
		Columns:   resource.S3ObjectColumns(),
		Children: []resource.ChildViewDef{{
			ChildType:      "s3_objects",
			Key:            "enter",
			ContextKeys:    map[string]string{"bucket": "@parent.bucket", "prefix": "ID"},
			DisplayNameKey: "bucket",
			DrillCondition: func(r resource.Resource) bool { return r.Status == "folder" },
		}},
		// RelatedContextFromIDs extracts the bucket name from related IDs encoded as
		// "bucket|key". Used when navigating to s3_objects from the related panel
		// (e.g., from a CloudTrail event detail view).
		RelatedContextFromIDs: func(relatedIDs []string) map[string]string {
			for _, id := range relatedIDs {
				parts := strings.SplitN(id, "|", 2)
				if len(parts) != 2 || parts[0] == "" {
					continue
				}
				bucket := parts[0]
				key := parts[1]
				// Derive the prefix (folder path) from the key so the child view
				// lands on the folder containing the object, not the bucket root.
				// Example: key="prod/config.json" → prefix="prod/"
				// Example: key="landing/2026/04/07/x.parquet" → prefix="landing/2026/04/07/"
				// Example: key="build-4821.tar.gz" → prefix=""
				prefix := ""
				if idx := strings.LastIndex(key, "/"); idx >= 0 {
					prefix = key[:idx+1]
				}
				return map[string]string{"bucket": bucket, "prefix": prefix}
			}
			return map[string]string{"bucket": "", "prefix": ""}
		},
	})
}
