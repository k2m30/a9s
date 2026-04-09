package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

// CloudFrontFixtures holds typed fixture data for CloudFront.
type CloudFrontFixtures struct {
	Distributions []cftypes.DistributionSummary
}

// NewCloudFrontFixtures constructs CloudFrontFixtures from the canonical demo data.
func NewCloudFrontFixtures() *CloudFrontFixtures {
	return &CloudFrontFixtures{
		Distributions: []cftypes.DistributionSummary{
			{
				Id:         aws.String("E1A2B3C4D5E6F7"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E1A2B3C4D5E6F7"),
				DomainName: aws.String("d111111abcdef8.cloudfront.net"),
				Status:     aws.String("Deployed"),
				Enabled:    aws.Bool(true),
				Aliases: &cftypes.Aliases{
					Quantity: aws.Int32(2),
					Items:    []string{"acme-corp.com", "www.acme-corp.com"},
				},
				Origins: &cftypes.Origins{
					Quantity: aws.Int32(2),
					Items: []cftypes.Origin{
						{
							Id:         aws.String("s3-static-assets"),
							DomainName: aws.String("acme-webapp-assets-prod.s3-website.us-east-1.amazonaws.com"),
						},
						{
							Id:         aws.String("alb-api-backend"),
							DomainName: aws.String("prod-api-alb-1234567890.us-east-1.elb.amazonaws.com"),
						},
					},
				},
				DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
					TargetOriginId:       aws.String("s3-static-assets"),
					ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
				},
				HttpVersion:      cftypes.HttpVersionHttp2,
				PriceClass:       cftypes.PriceClassPriceClassAll,
				Comment:          aws.String("Production website distribution"),
				LastModifiedTime: aws.Time(time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)),
			},
			{
				Id:         aws.String("E2B3C4D5E6F7G8"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E2B3C4D5E6F7G8"),
				DomainName: aws.String("d222222bcdefg9.cloudfront.net"),
				Status:     aws.String("Deployed"),
				Enabled:    aws.Bool(true),
				Aliases: &cftypes.Aliases{
					Quantity: aws.Int32(1),
					Items:    []string{"assets.acme-corp.com"},
				},
				Origins: &cftypes.Origins{
					Quantity: aws.Int32(1),
					Items: []cftypes.Origin{
						{
							Id:         aws.String("s3-webapp-assets"),
							DomainName: aws.String("acme-webapp-assets-prod.s3.amazonaws.com"),
						},
					},
				},
				PriceClass:       cftypes.PriceClassPriceClass100,
				Comment:          aws.String("Static assets CDN"),
				LastModifiedTime: aws.Time(time.Date(2026, 2, 15, 8, 30, 0, 0, time.UTC)),
			},
			{
				Id:         aws.String("E3C4D5E6F7G8H9"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E3C4D5E6F7G8H9"),
				DomainName: aws.String("d333333cdefgh0.cloudfront.net"),
				Status:     aws.String("InProgress"),
				Enabled:    aws.Bool(false),
				Aliases: &cftypes.Aliases{
					Quantity: aws.Int32(0),
				},
				PriceClass:       cftypes.PriceClassPriceClass200,
				Comment:          aws.String("Staging distribution (being configured)"),
				LastModifiedTime: aws.Time(time.Date(2026, 3, 21, 9, 0, 0, 0, time.UTC)),
			},
		},
	}
}
