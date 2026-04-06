// athena_related.go contains Athena related-resource checker functions.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkAthenaS3 returns Count: 0 because WorkGroupSummary does not include the
// output S3 location — the relationship cannot be determined from cache alone.
func checkAthenaS3(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
}

// checkAthenaKMS returns Count: 0 because WorkGroupSummary does not include
// encryption configuration — the relationship cannot be determined from cache alone.
func checkAthenaKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
}
