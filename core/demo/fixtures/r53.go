package fixtures

import (
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// R53Fixtures holds typed fixture data for Route53.
type R53Fixtures struct {
	HostedZones []r53types.HostedZone
	// RecordSets maps hosted zone ID to its resource record sets.
	RecordSets map[string][]r53types.ResourceRecordSet
	// QueryLoggingConfigs maps hosted zone ID (canonical "/hostedzone/..."
	// form — the same form the r53 fetcher stores as Resource.ID) to its
	// query-logging configuration — required for the r53:logs related-panel
	// pivot (checkR53Logs).
	QueryLoggingConfigs map[string]r53types.QueryLoggingConfig
}

// PublicZoneQueryLogGroupARN is the CloudWatch Logs log group ARN receiving
// query-log traffic for the acme-corp.com public zone — matches the
// /app/custom/no-retention log group fixture (cwlogs.go), required for the
// r53:logs related-panel pivot (checkR53Logs).
const PublicZoneQueryLogGroupARN = "arn:aws:logs:us-east-1:123456789012:log-group:/app/custom/no-retention:*"

// NewR53Fixtures constructs R53Fixtures from the canonical demo data.
var sharedR53Fixtures = sync.OnceValue(func() *R53Fixtures {
	return &R53Fixtures{
		HostedZones: []r53types.HostedZone{
			{
				Id:                     aws.String("/hostedzone/Z0123456789ABCDEFGHIJ"),
				Name:                   aws.String("acme-corp.com."),
				CallerReference:        aws.String("2025-01-01T00:00:00Z"),
				ResourceRecordSetCount: aws.Int64(42),
				Config: &r53types.HostedZoneConfig{
					Comment:     aws.String("Primary public domain for Acme Corp"),
					PrivateZone: false,
				},
			},
			{
				Id:                     aws.String("/hostedzone/Z1234567890ABCDEFGHIJ"),
				Name:                   aws.String("internal.acme-corp.com."),
				CallerReference:        aws.String("2025-02-15T00:00:00Z"),
				ResourceRecordSetCount: aws.Int64(18),
				Config: &r53types.HostedZoneConfig{
					Comment:     aws.String("Private zone for internal service discovery"),
					PrivateZone: true,
				},
			},
			{
				Id:                     aws.String("/hostedzone/Z2345678901ABCDEFGHIJ"),
				Name:                   aws.String("staging.acme-corp.com."),
				CallerReference:        aws.String("2025-06-01T00:00:00Z"),
				ResourceRecordSetCount: aws.Int64(8),
				Config: &r53types.HostedZoneConfig{
					Comment:     aws.String("Staging environment DNS"),
					PrivateZone: false,
				},
			},
			// Issue: ResourceRecordSetCount=2 → Warning (only NS+SOA, no real records — likely unused zone)
			{
				Id:                     aws.String("/hostedzone/Z3456789012ABCDEFGHIJ"),
				Name:                   aws.String("unused-zone.example.com."),
				CallerReference:        aws.String("2025-09-15T00:00:00Z"),
				ResourceRecordSetCount: aws.Int64(2),
				Config: &r53types.HostedZoneConfig{
					Comment:     aws.String("Abandoned zone — only default NS and SOA records remain"),
					PrivateZone: false,
				},
			},
			// S3 healthy-bucket alias zone (checkS3R53 pivot).
			// alias_targets is emitted by the r53 fetcher.
			// The record's AliasTarget.DNSName contains the S3 website endpoint.
			{
				Id:                     aws.String("/hostedzone/Z4567890123ABCDEFGHIJ"),
				Name:                   aws.String("demo.acme-corp.com."),
				CallerReference:        aws.String("2025-10-01T00:00:00Z"),
				ResourceRecordSetCount: aws.Int64(4),
				Config: &r53types.HostedZoneConfig{
					Comment:     aws.String("Demo zone with S3-website alias for a9s-demo-healthy"),
					PrivateZone: false,
				},
			},
			// Private zone with an active VPC association — required for the
			// r53:vpc related-panel pivot (checkR53VPC).
			{
				Id:                     aws.String("/hostedzone/Z5678901234ABCDEFGHIJ"),
				Name:                   aws.String("vpc-private.acme-corp.com."),
				CallerReference:        aws.String("2025-11-01T00:00:00Z"),
				ResourceRecordSetCount: aws.Int64(1),
				Config: &r53types.HostedZoneConfig{
					Comment:     aws.String("Private zone associated with the production VPC"),
					PrivateZone: true,
				},
			},
		},
		RecordSets: map[string][]r53types.ResourceRecordSet{
			"/hostedzone/Z0123456789ABCDEFGHIJ": {
				{
					Name: aws.String("acme-corp.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("d111111abcdef8.cloudfront.net."),
						HostedZoneId:         aws.String("Z2FDTNDATAQYW2"),
						EvaluateTargetHealth: false,
					},
				},
				{
					Name: aws.String("acme-corp.com."),
					Type: r53types.RRTypeNs,
					TTL:  aws.Int64(172800),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("ns-111.awsdns-11.com.")},
						{Value: aws.String("ns-222.awsdns-22.net.")},
						{Value: aws.String("ns-333.awsdns-33.org.")},
						{Value: aws.String("ns-444.awsdns-44.co.uk.")},
					},
				},
				{
					Name: aws.String("acme-corp.com."),
					Type: r53types.RRTypeSoa,
					TTL:  aws.Int64(900),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("ns-111.awsdns-11.com. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400")},
					},
				},
				{
					Name: aws.String("api.acme-corp.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("prod-api-alb-1234567890.us-east-1.elb.amazonaws.com."),
						HostedZoneId:         aws.String("Z35SXDOTRQ7X7K"),
						EvaluateTargetHealth: true,
					},
				},
				{
					Name: aws.String("mail.acme-corp.com."),
					Type: r53types.RRTypeMx,
					TTL:  aws.Int64(300),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("10 inbound-smtp.us-east-1.amazonaws.com.")},
					},
				},
				// ACM DNS-validation CNAME — required for the r53:acm
				// related-panel pivot (checkR53ACM). Validation record name
				// starts with "_" and value ends with ".acm-validations.aws".
				{
					Name: aws.String("_a1b2c3d4e5f6.acme-corp.com."),
					Type: r53types.RRTypeCname,
					TTL:  aws.Int64(300),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("_x1y2z3.acm-validations.aws.")},
					},
				},
				// API Gateway custom-domain alias — required for the
				// r53:apigw related-panel pivot (checkR53APIGW). Matches
				// PublicAPIGWID (apigw.go).
				{
					Name: aws.String("api-v2.acme-corp.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("abc123def4.execute-api.us-east-1.amazonaws.com."),
						HostedZoneId:         aws.String("Z1UJRXOUMOOFQ8"),
						EvaluateTargetHealth: false,
					},
				},
			},
			"/hostedzone/Z1234567890ABCDEFGHIJ": {
				{
					Name: aws.String("internal.acme.local."),
					Type: r53types.RRTypeA,
					TTL:  aws.Int64(300),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("10.0.1.100")},
					},
				},
				{
					Name: aws.String("db.internal.acme.local."),
					Type: r53types.RRTypeCname,
					TTL:  aws.Int64(60),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("prod-api-primary.cluster-c9xyz123.us-east-1.rds.amazonaws.com.")},
					},
				},
				{
					Name: aws.String("redis.internal.acme.local."),
					Type: r53types.RRTypeCname,
					TTL:  aws.Int64(60),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("acme-prod-redis.abc123.0001.use1.cache.amazonaws.com.")},
					},
				},
			},
			"/hostedzone/Z2345678901ABCDEFGHIJ": {
				{
					Name: aws.String("staging.acme-corp.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("staging-web-alb-5555555555.us-east-1.elb.amazonaws.com."),
						HostedZoneId:         aws.String("Z35SXDOTRQ7X7K"),
						EvaluateTargetHealth: true,
					},
				},
				{
					Name: aws.String("staging.acme-corp.com."),
					Type: r53types.RRTypeNs,
					TTL:  aws.Int64(172800),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("ns-777.awsdns-33.net.")},
						{Value: aws.String("ns-888.awsdns-44.org.")},
					},
				},
			},
			"/hostedzone/Z3456789012ABCDEFGHIJ": {
				{
					Name: aws.String("unused-zone.example.com."),
					Type: r53types.RRTypeNs,
					TTL:  aws.Int64(172800),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("ns-999.awsdns-99.com.")},
						{Value: aws.String("ns-111.awsdns-11.net.")},
					},
				},
				{
					Name: aws.String("unused-zone.example.com."),
					Type: r53types.RRTypeSoa,
					TTL:  aws.Int64(900),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("ns-999.awsdns-99.com. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400")},
					},
				},
			},
			// S3 healthy-bucket alias record. AWS-realistic shape: record
			// NAME (FQDN) is exactly the bucket name (S3 website aliases
			// require bucket-name==FQDN), and AliasTarget.DNSName is the
			// regional s3-website endpoint WITHOUT any bucket segment.
			// checkS3R53 joins on the record name.
			"/hostedzone/Z4567890123ABCDEFGHIJ": {
				{
					Name: aws.String(HealthyBucketName + "."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("s3-website-us-east-1.amazonaws.com."),
						HostedZoneId:         aws.String("Z3AQBSTGFYJSTF"),
						EvaluateTargetHealth: false,
					},
				},
				// downloads.demo.acme-corp.com — required for the r53:s3
				// related-panel pivot (checkR53S3, the forward r53→s3
				// direction). Uses the bucket-prefixed DNS-name alias style
				// (record name != bucket name, so the S3-website endpoint
				// itself carries the bucket segment).
				{
					Name: aws.String("downloads.demo.acme-corp.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String(HealthyBucketName + ".s3-website-us-east-1.amazonaws.com."),
						HostedZoneId:         aws.String("Z3AQBSTGFYJSTF"),
						EvaluateTargetHealth: false,
					},
				},
			},
			// Private zone with an active VPC association — required for the
			// r53:vpc related-panel pivot (checkR53VPC). Kept distinct from
			// the intentionally-orphaned Z1234567890ABCDEFGHIJ private zone
			// (which demonstrates the Wave-2 "no VPC associations" finding).
			"/hostedzone/Z5678901234ABCDEFGHIJ": {
				{
					Name: aws.String("db.vpc-private.acme-corp.com."),
					Type: r53types.RRTypeCname,
					TTL:  aws.Int64(60),
					ResourceRecords: []r53types.ResourceRecord{
						{Value: aws.String("prod-api-primary.cluster-c9xyz123.us-east-1.rds.amazonaws.com.")},
					},
				},
			},
		},
		QueryLoggingConfigs: map[string]r53types.QueryLoggingConfig{
			"/hostedzone/Z0123456789ABCDEFGHIJ": {
				Id:                        aws.String("qlc-acme-corp-001"),
				HostedZoneId:              aws.String("Z0123456789ABCDEFGHIJ"),
				CloudWatchLogsLogGroupArn: aws.String(PublicZoneQueryLogGroupARN),
			},
		},
	}
})

func NewR53Fixtures() *R53Fixtures {
	return sharedR53Fixtures()
}
