// codeartifact_related.go contains CodeArtifact repository related-resource checker functions.
//
// CodeArtifact RepositorySummary carries only Name, AdministratorAccount,
// DomainName, DomainOwner, Arn, Description, CreatedTime. It does NOT expose
// KMS, Lambda triggers, CloudWatch Logs, ACM certs, Route 53 endpoints, WAF,
// IAM role bindings, or Kinesis feeds. Resolving these relationships would
// require DescribeRepository + DescribeDomain + GetRepositoryPermissionsPolicy
// calls (N+1 per repo). The KMS checker below uses DescribeDomain (Pattern C)
// to resolve the domain's encryption key.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	catypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkCodeartifactKMS resolves the KMS encryption key for this repository's
// domain via codeartifact:DescribeDomain (Pattern C: 1 API call).
// RepositorySummary.DomainName is used as the domain identifier.
// DomainDescription.EncryptionKey holds the KMS key ARN (bare UUID extracted
// from the last "/" segment).
func checkCodeartifactKMS(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	repo, ok := assertStruct[catypes.RepositorySummary](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if repo.DomainName == nil || *repo.DomainName == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.CodeArtifact == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	api, ok := c.CodeArtifact.(CodeArtifactDescribeDomainAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	input := &codeartifact.DescribeDomainInput{Domain: repo.DomainName}
	if repo.DomainOwner != nil && *repo.DomainOwner != "" {
		input.DomainOwner = repo.DomainOwner
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*codeartifact.DescribeDomainOutput, error) {
		return api.DescribeDomain(ctx, input)
	})
	if err != nil || out == nil || out.Domain == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if out.Domain.EncryptionKey == nil || *out.Domain.EncryptionKey == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := arnLastSegment(*out.Domain.EncryptionKey)
	return relatedResult("kms", []string{keyID})
}
