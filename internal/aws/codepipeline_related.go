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

// checkPipelineKMS is a stub. The CodePipeline PipelineSummary list response does
// not include artifact store encryption key details — the KMS key is only on
// GetPipelineOutput, not the list summary RawStruct.
func checkPipelineKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
}

func checkPipelineCFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

func checkPipelineCodeArtifact(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "codeartifact", Count: 0}
}

func checkPipelineEbRule(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "eb-rule", Count: 0}
}

func checkPipelineECR(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ecr", Count: 0}
}

func checkPipelineECSSvc(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "ecs-svc", Count: 0}
}

func checkPipelineLambda(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
}

func checkPipelineLogs(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
}

func checkPipelineS3(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
}

func checkPipelineSNS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
}
