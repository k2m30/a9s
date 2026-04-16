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

// checkAthenaGlue returns Count: 0 because WorkGroupSummary does not include
// Glue catalog references — the relationship cannot be determined from cache alone.
func checkAthenaGlue(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "glue", Count: 0}
}

// checkAthenaLogs returns Count: 0 because WorkGroupSummary does not include
// CloudWatch Logs configuration — the relationship cannot be determined from cache alone.
func checkAthenaLogs(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
}

// checkAthenaRole returns Count: 0 because WorkGroupSummary does not include
// IAM role information — the relationship cannot be determined from cache alone.
func checkAthenaRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}
