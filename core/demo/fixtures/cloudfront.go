// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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
			// Status=InProgress with Enabled=true → Warning (colorCF). The
			// existing InProgress distribution (E3C4D5E6F7G8H9) is also
			// Enabled=false, which colorCF checks first and resolves to Dim —
			// this entry isolates the InProgress-only signal.
			{
				Id:         aws.String("E8H9I0J1K2L3M4"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E8H9I0J1K2L3M4"),
				DomainName: aws.String("d888888hijklm5.cloudfront.net"),
				Status:     aws.String("InProgress"),
				Enabled:    aws.Bool(true),
				Aliases: &cftypes.Aliases{
					Quantity: aws.Int32(1),
					Items:    []string{"new-launch.acme-corp.com"},
				},
				Origins: &cftypes.Origins{
					Quantity: aws.Int32(1),
					Items: []cftypes.Origin{
						{
							Id:         aws.String("s3-new-launch"),
							DomainName: aws.String("acme-new-launch-assets.s3.amazonaws.com"),
						},
					},
				},
				PriceClass:       cftypes.PriceClassPriceClass100,
				Comment:          aws.String("New product launch distribution — propagating config changes"),
				LastModifiedTime: aws.Time(time.Date(2026, 4, 25, 15, 0, 0, 0, time.UTC)),
			},
			// A distribution serving a bucket in ANOTHER account, which is
			// how a shared asset bucket is normally fronted. Its name is
			// absent from this account's bucket list, so the origin check
			// must ask HeadBucket rather than read absence as deletion. It
			// carries no finding: that is the whole point of the witness.
			{
				Id:         aws.String("E9I0J1K2L3M4N5"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E9I0J1K2L3M4N5"),
				DomainName: aws.String("d999999ijklmn6.cloudfront.net"),
				Status:     aws.String("Deployed"),
				Enabled:    aws.Bool(true),
				Aliases: &cftypes.Aliases{
					Quantity: aws.Int32(1),
					Items:    []string{"partner.acme-corp.com"},
				},
				Origins: &cftypes.Origins{
					Quantity: aws.Int32(1),
					Items: []cftypes.Origin{
						{
							Id:         aws.String("s3-partner-shared"),
							DomainName: aws.String(CFCrossAccountOriginDomain),
						},
					},
				},
				PriceClass:       cftypes.PriceClassPriceClass100,
				Comment:          aws.String("Partner asset distribution — origin bucket lives in the partner account"),
				LastModifiedTime: aws.Time(time.Date(2026, 5, 2, 11, 0, 0, 0, time.UTC)),
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
		DistributionConfigs: cfDistributionConfigs(),
	}
})

func NewCloudFrontFixtures() *CloudFrontFixtures {
	return sharedCloudFrontFixtures()
}

// Witness distributions for the w6a Prowler batch. Each names the ONE demo
// distribution carrying its finding.
const (
	// CFOriginBucketMissing is the distribution whose S3 origin names a
	// bucket no s3 fixture holds.
	CFOriginBucketMissing = "E8H9I0J1K2L3M4"

	// CFOriginBucketMissingDomain is that origin's domain name, built on a
	// bucket absent from the s3 fixtures.
	CFOriginBucketMissingDomain = "acme-new-launch-assets.s3.amazonaws.com"

	// CFCrossAccountOrigin is the distribution whose S3 origin bucket exists
	// in another account. It must carry NO finding: this account's bucket
	// list cannot see it, and absence from that list is not deletion.
	CFCrossAccountOrigin = "E9I0J1K2L3M4N5"

	// CFCrossAccountOriginDomain is that origin's domain name. Its bucket is
	// absent from the s3 fixtures and present in CrossAccountBuckets, so
	// HeadBucket answers that it exists.
	CFCrossAccountOriginDomain = PartnerSharedBucketName + ".s3.us-east-1.amazonaws.com"

	// CFDeprecatedTLS is the distribution whose minimum protocol version is
	// below TLS 1.2.
	CFDeprecatedTLS = "E2B3C4D5E6F7G8"

	// CFLoggingOff is the distribution with access logging switched off.
	CFLoggingOff = "E5E6F7G8H9I0J1"

	// CFNoRootObject is the distribution with no default root object.
	CFNoRootObject = "E4D5E6F7G8H9I0"

	// CFS3OriginNoOAC is the distribution whose REST-endpoint S3 origin has
	// neither an origin access control nor a legacy origin access identity;
	// it is the no-root-object row as well.
	CFS3OriginNoOAC = CFNoRootObject

	// CFDefaultCert is the distribution serving custom aliases with the
	// default CloudFront certificate.
	CFDefaultCert = CFS3WebsiteOrigin

	// CFNoGeoRestriction is the distribution with no geographic restriction.
	CFNoGeoRestriction = "E6F7G8H9I0J1K2"

	// CFS3WebsiteOrigin is the distribution reaching an S3 static-website
	// endpoint over http-only. It must carry NO insecure-protocol finding:
	// that endpoint serves HTTP alone, so http-only is the only policy it
	// accepts and there is nothing for an operator to change. It is the
	// row with no origin access control that carries no finding for it,
	// because a website endpoint is exactly the origin that cannot have one.
	CFS3WebsiteOrigin = "E7G8H9I0J1K2L3"

	// CFS3WebsiteOriginDomain is that origin's domain: the regional website
	// endpoint of the same bucket, which is a different hostname from the
	// REST endpoint the other S3 origins use.
	CFS3WebsiteOriginDomain = "a9s-demo-nopab.s3-website-us-east-1.amazonaws.com"
)

// cfHealthyConfig is the baseline every demo distribution config starts from:
// access logging on, a default root object, a custom certificate at TLS 1.2,
// a geo restriction, and an S3 origin behind an origin access control. Each
// witness below switches off exactly the one setting it demonstrates, so the
// demo bench shows one row per cf finding.
func cfHealthyConfig(originID, originDomain, alias string) *cftypes.DistributionConfig {
	return &cftypes.DistributionConfig{
		Enabled:           aws.Bool(true),
		Comment:           aws.String("demo distribution"),
		DefaultRootObject: aws.String("index.html"),
		Aliases:           &cftypes.Aliases{Quantity: aws.Int32(1), Items: []string{alias}},
		Origins: &cftypes.Origins{
			Quantity: aws.Int32(1),
			Items: []cftypes.Origin{{
				Id:                    aws.String(originID),
				DomainName:            aws.String(originDomain),
				OriginAccessControlId: aws.String("EOAC1A2B3C4D5E"),
			}},
		},
		DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
			TargetOriginId:       aws.String(originID),
			ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
		},
		ViewerCertificate: &cftypes.ViewerCertificate{
			ACMCertificateArn:            aws.String(ProdACMCertARN1),
			CloudFrontDefaultCertificate: aws.Bool(false),
			MinimumProtocolVersion:       cftypes.MinimumProtocolVersionTLSv122021,
		},
		Logging: &cftypes.LoggingConfig{
			Enabled: aws.Bool(true),
			Bucket:  aws.String(LogsBucketName + ".s3.amazonaws.com"),
			Prefix:  aws.String("cf/"),
		},
		Restrictions: &cftypes.Restrictions{
			GeoRestriction: &cftypes.GeoRestriction{
				RestrictionType: cftypes.GeoRestrictionTypeWhitelist,
				Quantity:        aws.Int32(2),
				Items:           []string{"DE", "FR"},
			},
		},
	}
}

// cfDistributionConfigs returns one config per demo distribution. Every
// distribution needs one: an absent config reads as no logging and no default
// root object, which would colour every row instead of the named witnesses.
func cfDistributionConfigs() map[string]*cftypes.DistributionConfig {
	cfgs := map[string]*cftypes.DistributionConfig{
		// Healthy, and the carrier for the cf→lambda and cf→logs pivots.
		"E1A2B3C4D5E6F7": cfHealthyConfig("s3-static-assets", "webapp-assets-prod.s3.amazonaws.com", "www.acme-corp.com"),
		CFDeprecatedTLS:  cfHealthyConfig("alb-legacy-api", "legacy-api.acme-corp.com", "legacy.acme-corp.com"),
		// Enabled=false on the summary makes this the dim row; its config is
		// healthy so nothing else colours it.
		"E3C4D5E6F7G8H9":      cfHealthyConfig("s3-archive", "acme-logs-archive.s3.amazonaws.com", "archive.acme-corp.com"),
		CFNoRootObject:        cfHealthyConfig("s3-media", "ml-training-data.s3.amazonaws.com", "media.acme-corp.com"),
		CFLoggingOff:          cfHealthyConfig("alb-old-api", "old-api.acme-corp.com", "old.acme-corp.com"),
		CFNoGeoRestriction:    cfHealthyConfig("s3-demo-healthy", HealthyBucketName+".s3.us-east-1.amazonaws.com", "demo.acme-corp.com"),
		CFS3WebsiteOrigin:     cfHealthyConfig("s3-nopab", "a9s-demo-nopab.s3.amazonaws.com", "pab.acme-corp.com"),
		CFOriginBucketMissing: cfHealthyConfig("s3-new-launch", CFOriginBucketMissingDomain, "new-launch.acme-corp.com"),
		CFCrossAccountOrigin:  cfHealthyConfig("s3-partner-shared", CFCrossAccountOriginDomain, "partner.acme-corp.com"),
	}

	// CFLoggingOff reaches an ordinary custom origin over http-only as well
	// as allowing plain HTTP from viewers, so the origin arm of the
	// insecure-protocol check has a demo row that legitimately trips it. That
	// origin can serve TLS, which is what separates it from the website
	// endpoint above.
	cfgs[CFLoggingOff].Origins.Items[0].OriginAccessControlId = nil
	cfgs[CFLoggingOff].Origins.Items[0].CustomOriginConfig = &cftypes.CustomOriginConfig{
		HTTPPort:             aws.Int32(80),
		HTTPSPort:            aws.Int32(443),
		OriginProtocolPolicy: cftypes.OriginProtocolPolicyHttpOnly,
	}

	cfgs[CFDeprecatedTLS].ViewerCertificate.MinimumProtocolVersion = cftypes.MinimumProtocolVersionTLSv12016
	cfgs[CFNoRootObject].DefaultRootObject = aws.String("")
	cfgs[CFLoggingOff].Logging = nil
	// The distribution keeps its insecure viewer policy from the summary.
	cfgs[CFLoggingOff].DefaultCacheBehavior.ViewerProtocolPolicy = cftypes.ViewerProtocolPolicyAllowAll
	cfgs[CFNoGeoRestriction].Restrictions.GeoRestriction.RestrictionType = cftypes.GeoRestrictionTypeNone
	cfgs[CFS3OriginNoOAC].Origins.Items[0].OriginAccessControlId = aws.String("")
	cfgs[CFS3OriginNoOAC].Origins.Items[0].S3OriginConfig = &cftypes.S3OriginConfig{OriginAccessIdentity: aws.String("")}
	// The website-endpoint row has no access control either, which is the
	// only shape that endpoint can take, so it must carry no finding for it.
	// It also carries the default-certificate witness; the two conditions
	// are independent.
	cfgs[CFS3WebsiteOrigin].Origins.Items[0].OriginAccessControlId = aws.String("")
	cfgs[CFS3WebsiteOrigin].Origins.Items[0].S3OriginConfig = &cftypes.S3OriginConfig{OriginAccessIdentity: aws.String("")}
	// It reaches that bucket through the S3 website endpoint, which serves
	// HTTP alone, so http-only is the only origin protocol policy CloudFront
	// accepts for it and the insecure-protocol finding must not fire. The
	// ordinary custom origin on CFLoggingOff is the row that trips it.
	cfgs[CFS3WebsiteOrigin].Origins.Items[0].DomainName = aws.String(CFS3WebsiteOriginDomain)
	cfgs[CFS3WebsiteOrigin].Origins.Items[0].CustomOriginConfig = &cftypes.CustomOriginConfig{
		HTTPPort:             aws.Int32(80),
		HTTPSPort:            aws.Int32(443),
		OriginProtocolPolicy: cftypes.OriginProtocolPolicyHttpOnly,
	}
	cfgs[CFS3WebsiteOrigin].ViewerCertificate = &cftypes.ViewerCertificate{CloudFrontDefaultCertificate: aws.Bool(true)}

	// Lambda@Edge association on the healthy distribution, for checkCfLambda.
	cfgs["E1A2B3C4D5E6F7"].DefaultCacheBehavior.LambdaFunctionAssociations = &cftypes.LambdaFunctionAssociations{
		Quantity: aws.Int32(1),
		Items: []cftypes.LambdaFunctionAssociation{{
			EventType:         cftypes.EventTypeViewerRequest,
			LambdaFunctionARN: aws.String("arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer:1"),
		}},
	}
	return cfgs
}

func init() {
	Register(Pin{ShortName: "cf", Rows: 9, Issues: 1})
}
