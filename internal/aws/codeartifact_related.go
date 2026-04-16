// codeartifact_related.go contains CodeArtifact repository related-resource checker functions.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkCodeartifactCB attempts to reverse-look up CodeBuild projects that pull from
// this CodeArtifact repository. Returns Count: -1 (unknown) because the CodeBuild
// project buildspec (which would name the repo/domain) is not stored as a structured
// field on cbtypes.Project — it is an inline YAML string or an external file reference.
// Parsing buildspecs per-project at related-panel rendering time is intentionally avoided.
func checkCodeartifactCB(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cb", Count: -1}
}

// CodeArtifact RepositorySummary carries only Name, AdministratorAccount,
// DomainName, DomainOwner, Arn, Description, CreatedTime. It does NOT expose
// KMS, Lambda triggers, CloudWatch Logs, ACM certs, Route 53 endpoints, WAF,
// IAM role bindings, or Kinesis feeds. Resolving these relationships would
// require DescribeRepository + DescribeDomain + GetRepositoryPermissionsPolicy
// calls (N+1 per repo). Intentionally returned as Count: -1.

func checkCodeartifactAcm(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "acm", Count: -1}
}

func checkCodeartifactKinesis(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kinesis", Count: -1}
}

func checkCodeartifactKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
}

func checkCodeartifactLambda(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
}

func checkCodeartifactLogs(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
}

func checkCodeartifactR53(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "r53", Count: -1}
}

func checkCodeartifactRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: -1}
}

func checkCodeartifactWaf(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
}
