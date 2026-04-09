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
