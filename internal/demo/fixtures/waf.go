// Package fixtures provides typed AWS SDK fixture data for demo mode.
package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

// WAFFixtures holds typed fixture data for WAFv2.
type WAFFixtures struct {
	WebACLSummaries []wafv2types.WebACLSummary
	// ResourcesByWebACL maps WebACL ARN to associated resource ARNs.
	ResourcesByWebACL map[string][]string
}

// NewWAFFixtures constructs WAFFixtures from the canonical demo data.
func NewWAFFixtures() *WAFFixtures {
	return &WAFFixtures{
		WebACLSummaries: []wafv2types.WebACLSummary{
			{
				Id:          aws.String("a1b2c3d4-5678-90ab-cdef-111111111111"),
				Name:        aws.String("acme-prod-api-waf"),
				ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-prod-api-waf/a1b2c3d4-5678-90ab-cdef-111111111111"),
				Description: aws.String("WAF for production API Gateway"),
				LockToken:   aws.String("lock-token-111"),
			},
			{
				Id:          aws.String("a1b2c3d4-5678-90ab-cdef-222222222222"),
				Name:        aws.String("acme-cloudfront-waf"),
				ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-cloudfront-waf/a1b2c3d4-5678-90ab-cdef-222222222222"),
				Description: aws.String("WAF for CloudFront distributions"),
				LockToken:   aws.String("lock-token-222"),
			},
			{
				Id:          aws.String("a1b2c3d4-5678-90ab-cdef-333333333333"),
				Name:        aws.String("acme-staging-waf"),
				ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-staging-waf/a1b2c3d4-5678-90ab-cdef-333333333333"),
				Description: aws.String("WAF for staging environment ALB"),
				LockToken:   aws.String("lock-token-333"),
			},
		},
		ResourcesByWebACL: map[string][]string{
			"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-prod-api-waf/a1b2c3d4-5678-90ab-cdef-111111111111": {
				"arn:aws:apigateway:us-east-1::/restapis/abc123def4/stages/prod",
			},
			"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-cloudfront-waf/a1b2c3d4-5678-90ab-cdef-222222222222": {
				"arn:aws:cloudfront::123456789012:distribution/E1A2B3C4D5E6F7",
			},
			"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-staging-waf/a1b2c3d4-5678-90ab-cdef-333333333333": {
				"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/staging-alb/5555555555555555",
			},
		},
	}
}
