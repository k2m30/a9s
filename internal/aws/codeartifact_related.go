// codeartifact_related.go contains CodeArtifact related-resource checker functions.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkCodeArtifactCB returns Count: 0 because CodeBuild project environment
// configurations (including CodeArtifact repository references) are not available
// in the ListProjects/BatchGetProjects list response — the relationship cannot be
// determined from cache alone.
func checkCodeArtifactCB(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cb", Count: 0}
}

// checkCodeArtifactRole returns Count: 0 because CodeArtifact repository domain
// policies reference roles but this is not surfaced on the ListRepositories
// list response.
func checkCodeArtifactRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}

// checkCodeArtifactKMS is a stub. The CodeArtifact domain's KMS key is not
// surfaced on the ListRepositories list response — the relationship cannot be
// determined from cache alone.
func checkCodeArtifactKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
}

func checkCodeArtifactACM(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "acm", Count: 0}
}

func checkCodeArtifactKinesis(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kinesis", Count: 0}
}

func checkCodeArtifactLambda(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
}

func checkCodeArtifactLogs(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
}

func checkCodeArtifactR53(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "r53", Count: 0}
}

func checkCodeArtifactWAF(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "waf", Count: 0}
}
