// codepipeline_related.go contains CodePipeline related-resource checker functions.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkPipelineCB returns Count: 0 because CodePipeline PipelineSummary does not
// include stage details — the CodeBuild projects used in pipeline stages cannot
// be determined from cache alone.
func checkPipelineCB(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cb", Count: 0}
}

// checkPipelineRole returns Count: 0 because CodePipeline PipelineSummary does
// not include the execution role ARN — the relationship cannot be determined
// from cache alone.
func checkPipelineRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}
