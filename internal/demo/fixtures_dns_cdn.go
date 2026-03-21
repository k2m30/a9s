package demo

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	demoData["r53"] = route53Zones
	demoData["cf"] = cloudFrontDistributions
	demoData["acm"] = acmCertificates
	demoData["apigw"] = apiGateways

	// Register R53 record fixtures for sub-resource drill-down.
	r53RecordData["/hostedzone/Z0123456789ABCDEFGHIJ"] = r53RecordsAcmeCorp
}

// route53Zones returns demo Route53 hosted zone fixtures.
func route53Zones() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "/hostedzone/Z0123456789ABCDEFGHIJ",
			Name:   "acme-corp.com.",
			Status: "",
			Fields: map[string]string{
				"zone_id":      "/hostedzone/Z0123456789ABCDEFGHIJ",
				"name":         "acme-corp.com.",
				"record_count": "42",
				"private_zone": "false",
				"comment":      "Primary public domain for Acme Corp",
			},
			RawStruct: r53types.HostedZone{
				Id:                     aws.String("/hostedzone/Z0123456789ABCDEFGHIJ"),
				Name:                   aws.String("acme-corp.com."),
				CallerReference:        aws.String("2025-01-01T00:00:00Z"),
				ResourceRecordSetCount: aws.Int64(42),
				Config: &r53types.HostedZoneConfig{
					Comment:     aws.String("Primary public domain for Acme Corp"),
					PrivateZone: false,
				},
			},
		},
		{
			ID:     "/hostedzone/Z1234567890ABCDEFGHIJ",
			Name:   "internal.acme-corp.com.",
			Status: "",
			Fields: map[string]string{
				"zone_id":      "/hostedzone/Z1234567890ABCDEFGHIJ",
				"name":         "internal.acme-corp.com.",
				"record_count": "18",
				"private_zone": "true",
				"comment":      "Private zone for internal service discovery",
			},
			RawStruct: r53types.HostedZone{
				Id:                     aws.String("/hostedzone/Z1234567890ABCDEFGHIJ"),
				Name:                   aws.String("internal.acme-corp.com."),
				CallerReference:        aws.String("2025-02-15T00:00:00Z"),
				ResourceRecordSetCount: aws.Int64(18),
				Config: &r53types.HostedZoneConfig{
					Comment:     aws.String("Private zone for internal service discovery"),
					PrivateZone: true,
				},
			},
		},
		{
			ID:     "/hostedzone/Z2345678901ABCDEFGHIJ",
			Name:   "staging.acme-corp.com.",
			Status: "",
			Fields: map[string]string{
				"zone_id":      "/hostedzone/Z2345678901ABCDEFGHIJ",
				"name":         "staging.acme-corp.com.",
				"record_count": "8",
				"private_zone": "false",
				"comment":      "Staging environment DNS",
			},
			RawStruct: r53types.HostedZone{
				Id:                     aws.String("/hostedzone/Z2345678901ABCDEFGHIJ"),
				Name:                   aws.String("staging.acme-corp.com."),
				CallerReference:        aws.String("2025-06-01T00:00:00Z"),
				ResourceRecordSetCount: aws.Int64(8),
				Config: &r53types.HostedZoneConfig{
					Comment:     aws.String("Staging environment DNS"),
					PrivateZone: false,
				},
			},
		},
	}
}

// r53RecordsAcmeCorp returns demo DNS records for the acme-corp.com zone.
func r53RecordsAcmeCorp() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-corp.com.|A",
			Name:   "acme-corp.com.",
			Status: "A",
			Fields: map[string]string{
				"name":   "acme-corp.com.",
				"type":   "A",
				"ttl":    "",
				"values": "ALIAS: d111111abcdef8.cloudfront.net.",
			},
			RawStruct: r53types.ResourceRecordSet{
				Name: aws.String("acme-corp.com."),
				Type: r53types.RRTypeA,
				AliasTarget: &r53types.AliasTarget{
					DNSName:              aws.String("d111111abcdef8.cloudfront.net."),
					HostedZoneId:         aws.String("Z2FDTNDATAQYW2"),
					EvaluateTargetHealth: false,
				},
			},
		},
		{
			ID:     "acme-corp.com.|NS",
			Name:   "acme-corp.com.",
			Status: "NS",
			Fields: map[string]string{
				"name":   "acme-corp.com.",
				"type":   "NS",
				"ttl":    "172800",
				"values": "ns-111.awsdns-11.com., ns-222.awsdns-22.net., ns-333.awsdns-33.org., ns-444.awsdns-44.co.uk.",
			},
			RawStruct: r53types.ResourceRecordSet{
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
		},
		{
			ID:     "acme-corp.com.|SOA",
			Name:   "acme-corp.com.",
			Status: "SOA",
			Fields: map[string]string{
				"name":   "acme-corp.com.",
				"type":   "SOA",
				"ttl":    "900",
				"values": "ns-111.awsdns-11.com. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400",
			},
			RawStruct: r53types.ResourceRecordSet{
				Name: aws.String("acme-corp.com."),
				Type: r53types.RRTypeSoa,
				TTL:  aws.Int64(900),
				ResourceRecords: []r53types.ResourceRecord{
					{Value: aws.String("ns-111.awsdns-11.com. awsdns-hostmaster.amazon.com. 1 7200 900 1209600 86400")},
				},
			},
		},
		{
			ID:     "api.acme-corp.com.|A",
			Name:   "api.acme-corp.com.",
			Status: "A",
			Fields: map[string]string{
				"name":   "api.acme-corp.com.",
				"type":   "A",
				"ttl":    "",
				"values": "ALIAS: acme-prod-alb-123456789.us-east-1.elb.amazonaws.com.",
			},
			RawStruct: r53types.ResourceRecordSet{
				Name: aws.String("api.acme-corp.com."),
				Type: r53types.RRTypeA,
				AliasTarget: &r53types.AliasTarget{
					DNSName:              aws.String("acme-prod-alb-123456789.us-east-1.elb.amazonaws.com."),
					HostedZoneId:         aws.String("Z35SXDOTRQ7X7K"),
					EvaluateTargetHealth: true,
				},
			},
		},
		{
			ID:     "mail.acme-corp.com.|MX",
			Name:   "mail.acme-corp.com.",
			Status: "MX",
			Fields: map[string]string{
				"name":   "mail.acme-corp.com.",
				"type":   "MX",
				"ttl":    "300",
				"values": "10 inbound-smtp.us-east-1.amazonaws.com.",
			},
			RawStruct: r53types.ResourceRecordSet{
				Name: aws.String("mail.acme-corp.com."),
				Type: r53types.RRTypeMx,
				TTL:  aws.Int64(300),
				ResourceRecords: []r53types.ResourceRecord{
					{Value: aws.String("10 inbound-smtp.us-east-1.amazonaws.com.")},
				},
			},
		},
	}
}

// cloudFrontDistributions returns demo CloudFront distribution fixtures.
func cloudFrontDistributions() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "E1A2B3C4D5E6F7",
			Name:   "E1A2B3C4D5E6F7",
			Status: "Deployed",
			Fields: map[string]string{
				"distribution_id": "E1A2B3C4D5E6F7",
				"domain_name":     "d111111abcdef8.cloudfront.net",
				"status":          "Deployed",
				"enabled":         "true",
				"aliases":         "acme-corp.com, www.acme-corp.com",
				"price_class":     "PriceClass_All",
			},
			RawStruct: cftypes.DistributionSummary{
				Id:         aws.String("E1A2B3C4D5E6F7"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E1A2B3C4D5E6F7"),
				DomainName: aws.String("d111111abcdef8.cloudfront.net"),
				Status:     aws.String("Deployed"),
				Enabled:    aws.Bool(true),
				Aliases: &cftypes.Aliases{
					Quantity: aws.Int32(2),
					Items:    []string{"acme-corp.com", "www.acme-corp.com"},
				},
				PriceClass:       cftypes.PriceClassPriceClassAll,
				Comment:          aws.String("Production website distribution"),
				LastModifiedTime: aws.Time(time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "E2B3C4D5E6F7G8",
			Name:   "E2B3C4D5E6F7G8",
			Status: "Deployed",
			Fields: map[string]string{
				"distribution_id": "E2B3C4D5E6F7G8",
				"domain_name":     "d222222bcdefg9.cloudfront.net",
				"status":          "Deployed",
				"enabled":         "true",
				"aliases":         "assets.acme-corp.com",
				"price_class":     "PriceClass_100",
			},
			RawStruct: cftypes.DistributionSummary{
				Id:         aws.String("E2B3C4D5E6F7G8"),
				ARN:        aws.String("arn:aws:cloudfront::123456789012:distribution/E2B3C4D5E6F7G8"),
				DomainName: aws.String("d222222bcdefg9.cloudfront.net"),
				Status:     aws.String("Deployed"),
				Enabled:    aws.Bool(true),
				Aliases: &cftypes.Aliases{
					Quantity: aws.Int32(1),
					Items:    []string{"assets.acme-corp.com"},
				},
				PriceClass:       cftypes.PriceClassPriceClass100,
				Comment:          aws.String("Static assets CDN"),
				LastModifiedTime: aws.Time(time.Date(2026, 2, 15, 8, 30, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "E3C4D5E6F7G8H9",
			Name:   "E3C4D5E6F7G8H9",
			Status: "InProgress",
			Fields: map[string]string{
				"distribution_id": "E3C4D5E6F7G8H9",
				"domain_name":     "d333333cdefgh0.cloudfront.net",
				"status":          "InProgress",
				"enabled":         "false",
				"aliases":         "",
				"price_class":     "PriceClass_200",
			},
			RawStruct: cftypes.DistributionSummary{
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

// acmCertificates returns demo ACM certificate fixtures.
func acmCertificates() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-corp.com",
			Name:   "acme-corp.com",
			Status: "ISSUED",
			Fields: map[string]string{
				"domain_name": "acme-corp.com",
				"status":      "ISSUED",
				"type":        "AMAZON_ISSUED",
				"not_after":   "2027-04-15T23:59:59Z",
				"in_use":      "true",
			},
			RawStruct: acmtypes.CertificateSummary{
				DomainName:     aws.String("acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111"),
				Status:         acmtypes.CertificateStatusIssued,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				NotAfter:       aws.Time(mustParseTime("2027-04-15T23:59:59+00:00")),
				InUse:          aws.Bool(true),
				CreatedAt:      aws.Time(time.Date(2025, 4, 15, 10, 0, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "*.acme-corp.com",
			Name:   "*.acme-corp.com",
			Status: "ISSUED",
			Fields: map[string]string{
				"domain_name": "*.acme-corp.com",
				"status":      "ISSUED",
				"type":        "AMAZON_ISSUED",
				"not_after":   "2027-06-20T23:59:59Z",
				"in_use":      "true",
			},
			RawStruct: acmtypes.CertificateSummary{
				DomainName:     aws.String("*.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/b2c3d4e5-6789-01ab-cdef-222222222222"),
				Status:         acmtypes.CertificateStatusIssued,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				NotAfter:       aws.Time(mustParseTime("2027-06-20T23:59:59+00:00")),
				InUse:          aws.Bool(true),
				CreatedAt:      aws.Time(time.Date(2025, 6, 20, 14, 0, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "staging.acme-corp.com",
			Name:   "staging.acme-corp.com",
			Status: "PENDING_VALIDATION",
			Fields: map[string]string{
				"domain_name": "staging.acme-corp.com",
				"status":      "PENDING_VALIDATION",
				"type":        "AMAZON_ISSUED",
				"not_after":   "",
				"in_use":      "false",
			},
			RawStruct: acmtypes.CertificateSummary{
				DomainName:     aws.String("staging.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/c3d4e5f6-7890-12ab-cdef-333333333333"),
				Status:         acmtypes.CertificateStatusPendingValidation,
				Type:           acmtypes.CertificateTypeAmazonIssued,
				InUse:          aws.Bool(false),
				CreatedAt:      aws.Time(time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "legacy.acme-corp.com",
			Name:   "legacy.acme-corp.com",
			Status: "EXPIRED",
			Fields: map[string]string{
				"domain_name": "legacy.acme-corp.com",
				"status":      "EXPIRED",
				"type":        "IMPORTED",
				"not_after":   "2025-12-31T23:59:59Z",
				"in_use":      "false",
			},
			RawStruct: acmtypes.CertificateSummary{
				DomainName:     aws.String("legacy.acme-corp.com"),
				CertificateArn: aws.String("arn:aws:acm:us-east-1:123456789012:certificate/d4e5f6a7-8901-23ab-cdef-444444444444"),
				Status:         acmtypes.CertificateStatusExpired,
				Type:           acmtypes.CertificateTypeImported,
				NotAfter:       aws.Time(mustParseTime("2025-12-31T23:59:59+00:00")),
				InUse:          aws.Bool(false),
				ImportedAt:     aws.Time(time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC)),
			},
		},
	}
}

// apiGateways returns demo API Gateway V2 fixtures.
func apiGateways() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "abc123def4",
			Name:   "acme-public-api",
			Status: "",
			Fields: map[string]string{
				"api_id":      "abc123def4",
				"name":        "acme-public-api",
				"protocol":    "HTTP",
				"endpoint":    "https://abc123def4.execute-api.us-east-1.amazonaws.com",
				"description": "Public REST API for Acme Corp mobile and web clients",
			},
			RawStruct: apigwtypes.Api{
				ApiId:                    aws.String("abc123def4"),
				Name:                     aws.String("acme-public-api"),
				ProtocolType:             apigwtypes.ProtocolTypeHttp,
				ApiEndpoint:              aws.String("https://abc123def4.execute-api.us-east-1.amazonaws.com"),
				Description:              aws.String("Public REST API for Acme Corp mobile and web clients"),
				RouteSelectionExpression: aws.String("${request.method} ${request.path}"),
				CreatedDate:              aws.Time(time.Date(2025, 3, 10, 9, 0, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "efg567hij8",
			Name:   "acme-websocket-api",
			Status: "",
			Fields: map[string]string{
				"api_id":      "efg567hij8",
				"name":        "acme-websocket-api",
				"protocol":    "WEBSOCKET",
				"endpoint":    "wss://efg567hij8.execute-api.us-east-1.amazonaws.com",
				"description": "WebSocket API for real-time order notifications",
			},
			RawStruct: apigwtypes.Api{
				ApiId:                    aws.String("efg567hij8"),
				Name:                     aws.String("acme-websocket-api"),
				ProtocolType:             apigwtypes.ProtocolTypeWebsocket,
				ApiEndpoint:              aws.String("wss://efg567hij8.execute-api.us-east-1.amazonaws.com"),
				Description:              aws.String("WebSocket API for real-time order notifications"),
				RouteSelectionExpression: aws.String("$request.body.action"),
				CreatedDate:              aws.Time(time.Date(2025, 7, 5, 14, 30, 0, 0, time.UTC)),
			},
		},
		{
			ID:     "klm901nop2",
			Name:   "internal-service-api",
			Status: "",
			Fields: map[string]string{
				"api_id":      "klm901nop2",
				"name":        "internal-service-api",
				"protocol":    "HTTP",
				"endpoint":    "https://klm901nop2.execute-api.us-east-1.amazonaws.com",
				"description": "Internal microservice-to-microservice API",
			},
			RawStruct: apigwtypes.Api{
				ApiId:                    aws.String("klm901nop2"),
				Name:                     aws.String("internal-service-api"),
				ProtocolType:             apigwtypes.ProtocolTypeHttp,
				ApiEndpoint:              aws.String("https://klm901nop2.execute-api.us-east-1.amazonaws.com"),
				Description:              aws.String("Internal microservice-to-microservice API"),
				RouteSelectionExpression: aws.String("${request.method} ${request.path}"),
				CreatedDate:              aws.Time(time.Date(2025, 10, 12, 11, 0, 0, 0, time.UTC)),
			},
		},
	}
}
