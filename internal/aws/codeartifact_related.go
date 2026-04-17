// codeartifact_related.go contains CodeArtifact repository related-resource checker functions.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// CodeArtifact RepositorySummary carries only Name, AdministratorAccount,
// DomainName, DomainOwner, Arn, Description, CreatedTime. It does NOT expose
// KMS, Lambda triggers, CloudWatch Logs, ACM certs, Route 53 endpoints, WAF,
// IAM role bindings, or Kinesis feeds. Resolving these relationships would
// require DescribeRepository + DescribeDomain + GetRepositoryPermissionsPolicy
// calls (N+1 per repo). Intentionally returned as Count: -1.
