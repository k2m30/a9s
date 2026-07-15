package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// CloudFrontFake implements aws.CloudFrontAPI against fixture data loaded at construction time.
type CloudFrontFake struct {
	fix *fixtures.CloudFrontFixtures
}

// NewCloudFront constructs a CloudFrontFake backed by fixture data from the fixtures package.
func NewCloudFront() *CloudFrontFake {
	return &CloudFrontFake{fix: fixtures.NewCloudFrontFixtures()}
}

func (f *CloudFrontFake) ListDistributions(_ context.Context, _ *cloudfront.ListDistributionsInput, _ ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsOutput, error) {
	return &cloudfront.ListDistributionsOutput{
		DistributionList: &cftypes.DistributionList{
			Items: f.fix.Distributions,
		},
	}, nil
}

// ListDistributionsByWebACLId returns distributions whose WebACLId exactly
// matches the requested ARN, backing the waf:cf related-panel pivot
// (checkWAFCF, which sends the Web ACL's ARN — cloudfront:ListDistributionsByWebACLId
// expects the ARN form, not the bare Web ACL ID).
func (f *CloudFrontFake) ListDistributionsByWebACLId(_ context.Context, input *cloudfront.ListDistributionsByWebACLIdInput, _ ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsByWebACLIdOutput, error) {
	var webACLID string
	if input != nil && input.WebACLId != nil {
		webACLID = *input.WebACLId
	}
	var items []cftypes.DistributionSummary
	for _, d := range f.fix.Distributions {
		if d.WebACLId != nil && *d.WebACLId == webACLID {
			items = append(items, d)
		}
	}
	return &cloudfront.ListDistributionsByWebACLIdOutput{
		DistributionList: &cftypes.DistributionList{Items: items},
	}, nil
}

// GetDistributionConfig returns a config carrying the Lambda@Edge
// associations and access-log destination for known demo distributions
// (backing checkCfLambda / checkCfLogs), and an empty config for everything
// else so Wave 2 enrichment (viewer/origin protocol policy checks) produces
// no findings for those distributions in demo mode.
func (f *CloudFrontFake) GetDistributionConfig(_ context.Context, input *cloudfront.GetDistributionConfigInput, _ ...func(*cloudfront.Options)) (*cloudfront.GetDistributionConfigOutput, error) {
	if input != nil && input.Id != nil {
		if cfg, ok := f.fix.DistributionConfigs[*input.Id]; ok {
			return &cloudfront.GetDistributionConfigOutput{DistributionConfig: cfg}, nil
		}
	}
	return &cloudfront.GetDistributionConfigOutput{
		DistributionConfig: &cftypes.DistributionConfig{},
	}, nil
}
