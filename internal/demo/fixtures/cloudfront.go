package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

// CloudFrontFixtures holds typed fixture data for CloudFront.
type CloudFrontFixtures struct {
	Distributions []cftypes.DistributionSummary
	// DistributionConfigs maps a distribution ID to the config returned by
	// cloudfront:GetDistributionConfig. Backs the cf→lambda (Lambda@Edge
	// associations) and cf→logs (access-log S3 destination) related-panel
	// pivots (checkCfLambda / checkCfLogs), which call this API directly.
	DistributionConfigs map[string]*cftypes.DistributionConfig
}

// NewCloudFrontFixtures constructs CloudFrontFixtures from the canonical demo data.
var sharedCloudFrontFixtures = sync.OnceValue(func() *CloudFrontFixtures {
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
					Quantity: aws.Int32(3),
					Items: []cftypes.Origin{
						{
							Id:         aws.String("s3-static-assets"),
							DomainName: aws.String("acme-webapp-assets-prod.s3-website.us-east-1.amazonaws.com"),
						},
						// alb-api-backend origin — required for the cf:elb /
						// elb:cf related-panel pivot witness. DomainName must
						// match an existing ELB fixture's DNS name exactly
						// (checkCfELB / checkELBCF match on dns_name).
						{
							Id:         aws.String("alb-api-backend"),
							DomainName: aws.String(fixtProdELBDNS),
						},
						// acme-public-api origin — required for apigw:cf
						// related-panel pivot. checkApigwCF matches origin
						// DomainName containing "{apiID}.execute-api.".
						{
							Id:         aws.String("apigw-public-api"),
							DomainName: aws.String(PublicAPIGWID + ".execute-api.us-east-1.amazonaws.com"),
						},
					},
				},
				// LambdaFunctionAssociations — required for the lambda:cf
				// related-panel pivot witness (checkLambdaCF). Lambda@Edge
				// always references a published version; api-gateway-authorizer
				// is a real lambda.go fixture.
				DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
					TargetOriginId:       aws.String("s3-static-assets"),
					ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
					LambdaFunctionAssociations: &cftypes.LambdaFunctionAssociations{
						Quantity: aws.Int32(1),
						Items: []cftypes.LambdaFunctionAssociation{
							{
								EventType:         cftypes.EventTypeViewerRequest,
								LambdaFunctionARN: aws.String("arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer:1"),
							},
						},
					},
				},
				// WebACLId — required for the cf:waf related-panel pivot
				// witness (checkCfWAF). Matches the acme-cloudfront-waf ACL's
				// ARN in waf.go (ResourcesByWebACL already reverse-maps this
				// same distribution for the waf→cf direction).
				WebACLId: aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-cloudfront-waf/a1b2c3d4-5678-90ab-cdef-222222222222"),
				// ViewerCertificate.ACMCertificateArn — required for the
				// cf:acm related-panel pivot (checkCfACM). ProdACMCertARN1
				// (acm.go) covers acme-corp.com, this distribution's alias.
				ViewerCertificate: &cftypes.ViewerCertificate{
					ACMCertificateArn:      aws.String("arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111"),
					MinimumProtocolVersion: cftypes.MinimumProtocolVersionTLSv122021,
					SSLSupportMethod:       cftypes.SSLSupportMethodSniOnly,
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
			// Issue: Enabled=false → Dim (distribution deliberately disabled)
			{
				Id:         aws.String("E4D5E6F7G8H9I0"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E4D5E6F7G8H9I0"),
				DomainName: aws.String("d444444defghi1.cloudfront.net"),
				Status:     aws.String("Deployed"),
				Enabled:    aws.Bool(false),
				Aliases: &cftypes.Aliases{
					Quantity: aws.Int32(1),
					Items:    []string{"legacy-cdn.acme-corp.com"},
				},
				Origins: &cftypes.Origins{
					Quantity: aws.Int32(1),
					Items: []cftypes.Origin{
						{
							Id:         aws.String("s3-legacy"),
							DomainName: aws.String("acme-legacy-assets.s3.amazonaws.com"),
						},
					},
				},
				PriceClass:       cftypes.PriceClassPriceClass100,
				Comment:          aws.String("Legacy distribution — disabled pending decommission"),
				LastModifiedTime: aws.Time(time.Date(2025, 11, 1, 14, 0, 0, 0, time.UTC)),
			},
			// Issue: ViewerCertificate.MinimumProtocolVersion=TLSv1 → Warning (weak TLS)
			{
				Id:         aws.String("E5E6F7G8H9I0J1"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E5E6F7G8H9I0J1"),
				DomainName: aws.String("d555555efghij2.cloudfront.net"),
				Status:     aws.String("Deployed"),
				Enabled:    aws.Bool(true),
				Aliases: &cftypes.Aliases{
					Quantity: aws.Int32(1),
					Items:    []string{"old-api.acme-corp.com"},
				},
				Origins: &cftypes.Origins{
					Quantity: aws.Int32(1),
					Items: []cftypes.Origin{
						{
							Id:         aws.String("alb-old-api"),
							DomainName: aws.String("old-api-alb-9876543210.us-east-1.elb.amazonaws.com"),
						},
					},
				},
				DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
					TargetOriginId:       aws.String("alb-old-api"),
					ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyAllowAll,
				},
				ViewerCertificate: &cftypes.ViewerCertificate{
					// Weak minimum TLS version — should be TLSv1.2_2021
					MinimumProtocolVersion: cftypes.MinimumProtocolVersionTLSv1,
					SSLSupportMethod:       cftypes.SSLSupportMethodSniOnly,
				},
				PriceClass:       cftypes.PriceClassPriceClass100,
				Comment:          aws.String("Legacy API distribution with weak TLS configuration"),
				LastModifiedTime: aws.Time(time.Date(2025, 6, 10, 10, 0, 0, 0, time.UTC)),
			},
			// S3 healthy-bucket distribution (checkS3CF pivot).
			// checkS3CF checks strings.Contains(*origin.DomainName, bucketName+".s3").
			{
				Id:         aws.String("E6F7G8H9I0J1K2"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E6F7G8H9I0J1K2"),
				DomainName: aws.String("d666666fghijk3.cloudfront.net"),
				Status:     aws.String("Deployed"),
				Enabled:    aws.Bool(true),
				Aliases: &cftypes.Aliases{
					Quantity: aws.Int32(1),
					Items:    []string{"demo.acme-corp.com"},
				},
				Origins: &cftypes.Origins{
					Quantity: aws.Int32(1),
					Items: []cftypes.Origin{
						{
							Id: aws.String("s3-demo-healthy"),
							// DomainName must contain bucketName+".s3" for checkS3CF to match.
							DomainName: aws.String(HealthyBucketName + ".s3.us-east-1.amazonaws.com"),
						},
					},
				},
				DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
					TargetOriginId:       aws.String("s3-demo-healthy"),
					ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
				},
				PriceClass:       cftypes.PriceClassPriceClass100,
				Comment:          aws.String("Demo distribution backed by a9s-demo-healthy S3 bucket"),
				LastModifiedTime: aws.Time(time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)),
			},
			// Distribution fronting the PAB-issue buckets — realistic
			// scenario: a CDN points at an origin bucket whose access
			// policy is misconfigured. Operator pivoting from the `!`
			// row reaches the distribution via this row.
			{
				Id:         aws.String("E7G8H9I0J1K2L3"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E7G8H9I0J1K2L3"),
				DomainName: aws.String("d777777ghijkl4.cloudfront.net"),
				Status:     aws.String("Deployed"),
				Enabled:    aws.Bool(true),
				Origins: &cftypes.Origins{
					Quantity: aws.Int32(4),
					Items: []cftypes.Origin{
						{
							Id:         aws.String("s3-nopab"),
							DomainName: aws.String("a9s-demo-nopab.s3.us-east-1.amazonaws.com"),
						},
						{
							Id:         aws.String("s3-partial"),
							DomainName: aws.String("a9s-demo-partial-pab.s3.us-east-1.amazonaws.com"),
						},
						{
							Id:         aws.String("s3-multifail"),
							DomainName: aws.String("a9s-demo-multifail-pab.s3.us-east-1.amazonaws.com"),
						},
						{
							Id:         aws.String("s3-nilcfg"),
							DomainName: aws.String("a9s-demo-nilcfg.s3.us-east-1.amazonaws.com"),
						},
					},
				},
				DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
					TargetOriginId:       aws.String("s3-partial"),
					ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
				},
				PriceClass:       cftypes.PriceClassPriceClass100,
				Comment:          aws.String("CDN fronting misconfigured S3 buckets"),
				LastModifiedTime: aws.Time(time.Date(2026, 2, 20, 11, 0, 0, 0, time.UTC)),
			},
		},
		// DistributionConfigs — backs the cf→lambda and cf→logs related-panel
		// pivots (checkCfLambda / checkCfLogs), which call
		// cloudfront:GetDistributionConfig directly. E1A2B3C4D5E6F7 mirrors
		// the Lambda@Edge association already on its DistributionSummary and
		// adds an access-log destination pointing at the a9s-demo-logs bucket
		// (s3.go LogsBucketName).
		DistributionConfigs: map[string]*cftypes.DistributionConfig{
			"E1A2B3C4D5E6F7": {
				DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
					TargetOriginId:       aws.String("s3-static-assets"),
					ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
					LambdaFunctionAssociations: &cftypes.LambdaFunctionAssociations{
						Quantity: aws.Int32(1),
						Items: []cftypes.LambdaFunctionAssociation{
							{
								EventType:         cftypes.EventTypeViewerRequest,
								LambdaFunctionARN: aws.String("arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer:1"),
							},
						},
					},
				},
				Logging: &cftypes.LoggingConfig{
					Enabled: aws.Bool(true),
					Bucket:  aws.String(LogsBucketName + ".s3.amazonaws.com"),
				},
			},
			// E5E6F7G8H9I0J1 — required for the Wave-2 issue-coverage gate
			// (TestDemoIssueCoverage). Mirrors its DistributionSummary's
			// already-modeled insecure ViewerProtocolPolicyAllowAll so
			// EnrichCloudFrontDistribution's viewer-protocol check fires.
			"E5E6F7G8H9I0J1": {
				DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
					TargetOriginId:       aws.String("alb-old-api"),
					ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyAllowAll,
				},
			},
		},
	}
})

func NewCloudFrontFixtures() *CloudFrontFixtures {
	return sharedCloudFrontFixtures()
}
