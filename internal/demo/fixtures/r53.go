package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// R53Fixtures holds typed fixture data for Route53.
type R53Fixtures struct {
	HostedZones []r53types.HostedZone
	// RecordSets maps hosted zone ID to its resource record sets.
	RecordSets map[string][]r53types.ResourceRecordSet
}

// NewR53Fixtures constructs R53Fixtures from the canonical demo data.
func NewR53Fixtures() *R53Fixtures {
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
			// alias_targets field is populated by the r53 fetcher in Phase 7.
			// The record's AliasTarget.DNSName contains the S3 website endpoint.
			{
				Id:                     aws.String("/hostedzone/Z4567890123ABCDEFGHIJ"),
				Name:                   aws.String("demo.acme-corp.com."),
				CallerReference:        aws.String("2025-10-01T00:00:00Z"),
				ResourceRecordSetCount: aws.Int64(3),
				Config: &r53types.HostedZoneConfig{
					Comment:     aws.String("Demo zone with S3-website alias for a9s-demo-healthy"),
					PrivateZone: false,
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
			// S3 healthy-bucket record set.
			// AliasTarget.DNSName contains the S3 website endpoint — used by checkS3R53
			// once Phase 7 populates alias_targets in the r53 fetcher.
			"/hostedzone/Z4567890123ABCDEFGHIJ": {
				{
					Name: aws.String("demo.acme-corp.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String("s3-website-us-east-1.amazonaws.com."),
						HostedZoneId:         aws.String("Z3AQBSTGFYJSTF"),
						EvaluateTargetHealth: false,
					},
				},
				{
					Name: aws.String(HealthyBucketName + ".demo.acme-corp.com."),
					Type: r53types.RRTypeA,
					AliasTarget: &r53types.AliasTarget{
						DNSName:              aws.String(HealthyBucketName + ".s3-website-us-east-1.amazonaws.com."),
						HostedZoneId:         aws.String("Z3AQBSTGFYJSTF"),
						EvaluateTargetHealth: false,
					},
				},
			},
		},
	}
}
