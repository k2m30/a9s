package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
)

// CloudFrontListDistributionsAPI defines the interface for the CloudFront ListDistributions operation.
type CloudFrontListDistributionsAPI interface {
	ListDistributions(ctx context.Context, params *cloudfront.ListDistributionsInput, optFns ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsOutput, error)
}

// CloudFrontGetDistributionConfigAPI defines the interface for the CloudFront GetDistributionConfig operation.
// Used by Wave 2 enrichment to inspect per-distribution security configuration.
type CloudFrontGetDistributionConfigAPI interface {
	GetDistributionConfig(ctx context.Context, params *cloudfront.GetDistributionConfigInput, optFns ...func(*cloudfront.Options)) (*cloudfront.GetDistributionConfigOutput, error)
}

// CloudFrontListDistributionsByWebACLIdAPI is the forward "WebACL → CF distributions" lookup.
type CloudFrontListDistributionsByWebACLIdAPI interface {
	ListDistributionsByWebACLId(ctx context.Context, params *cloudfront.ListDistributionsByWebACLIdInput, optFns ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsByWebACLIdOutput, error)
}

// CloudFrontAPI is the aggregate interface covering all CloudFront operations used by a9s fetchers.
// *cloudfront.Client structurally satisfies this interface.
type CloudFrontAPI interface {
	CloudFrontListDistributionsAPI
	CloudFrontGetDistributionConfigAPI // Wave 2 enrichment
}
