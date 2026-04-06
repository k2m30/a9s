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
