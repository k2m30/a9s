package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
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

// GetDistributionConfig returns an empty config for demo mode.
// Wave 2 enrichment uses this to check viewer/origin protocol policies;
// returning an empty config produces no findings in demo mode.
func (f *CloudFrontFake) GetDistributionConfig(_ context.Context, _ *cloudfront.GetDistributionConfigInput, _ ...func(*cloudfront.Options)) (*cloudfront.GetDistributionConfigOutput, error) {
	return &cloudfront.GetDistributionConfigOutput{
		DistributionConfig: &cftypes.DistributionConfig{},
	}, nil
}
