// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fixtures provides typed AWS SDK fixture data for demo mode.
package fixtures

import (
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

// WAFNoRules is the witness Web ACL for waf.no-rules: it exists and is
// associated, but contains no rules at all. Every other demo ACL has at
// least one rule.
const WAFNoRules = "acme-empty-waf"

// WAFOrphan is the witness Web ACL for waf.orphan: it has rules and a logging
// configuration, and is attached to nothing. Every other demo ACL appears in
// ResourcesByWebACL with at least one associated resource.
const WAFOrphan = "acme-unattached-waf"

// WAFFixtures holds typed fixture data for WAFv2.
type WAFFixtures struct {
	// WebACLSummaries holds REGIONAL-scope Web ACLs (served by ListWebACLs
	// when Scope=REGIONAL).
	WebACLSummaries []wafv2types.WebACLSummary
	// CloudFrontWebACLSummaries holds CLOUDFRONT-scope Web ACLs (served by
	// ListWebACLs when Scope=CLOUDFRONT — a us-east-1-only global listing).
	CloudFrontWebACLSummaries []wafv2types.WebACLSummary
	// ResourcesByWebACL maps WebACL ARN to associated resource ARNs.
	ResourcesByWebACL map[string][]string
}

// NewWAFFixtures constructs WAFFixtures from the canonical demo data.
var sharedWAFFixtures = sync.OnceValue(func() *WAFFixtures {
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
				Id:          aws.String("a1b2c3d4-5678-90ab-cdef-444444444444"),
				Name:        aws.String(WAFNoRules),
				ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/" + WAFNoRules + "/a1b2c3d4-5678-90ab-cdef-444444444444"),
				Description: aws.String("Web ACL attached to the internal ALB with no rules configured"),
				LockToken:   aws.String("lock-token-444"),
			},
			{
				Id:          aws.String("a1b2c3d4-5678-90ab-cdef-555555555555"),
				Name:        aws.String(WAFOrphan),
				ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/" + WAFOrphan + "/a1b2c3d4-5678-90ab-cdef-555555555555"),
				Description: aws.String("Web ACL left behind after the ALB it protected was deleted"),
				LockToken:   aws.String("lock-token-555"),
			},
			{
				Id:          aws.String("a1b2c3d4-5678-90ab-cdef-333333333333"),
				Name:        aws.String("acme-staging-waf"),
				ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-staging-waf/a1b2c3d4-5678-90ab-cdef-333333333333"),
				Description: aws.String("WAF for staging environment ALB"),
				LockToken:   aws.String("lock-token-333"),
			},
		},
		// acme-cloudfront-waf is CLOUDFRONT-scope — CloudFront can only bind
		// Web ACLs created with Scope=CLOUDFRONT (AWS SDK Go v2 —
		// wafv2/types.Scope § CLOUDFRONT), so this ACL is served under
		// ListWebACLs(Scope=CLOUDFRONT), not REGIONAL.
		CloudFrontWebACLSummaries: []wafv2types.WebACLSummary{
			{
				Id:          aws.String("a1b2c3d4-5678-90ab-cdef-222222222222"),
				Name:        aws.String("acme-cloudfront-waf"),
				ARN:         aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-cloudfront-waf/a1b2c3d4-5678-90ab-cdef-222222222222"),
				Description: aws.String("WAF for CloudFront distributions"),
				LockToken:   aws.String("lock-token-222"),
			},
		},
		ResourcesByWebACL: map[string][]string{
			"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-prod-api-waf/a1b2c3d4-5678-90ab-cdef-111111111111": {
				"arn:aws:apigateway:us-east-1::/restapis/abc123def4/stages/prod",
			},
			"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-cloudfront-waf/a1b2c3d4-5678-90ab-cdef-222222222222": {
				"arn:aws:cloudfront::123456789012:distribution/E1A2B3C4D5E6F7",
			},
			// staging-web-alb — matches the real elb.go fixture ARN exactly
			// so checkELBWAF's GetWebACLForResource reverse lookup resolves.
			"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-staging-waf/a1b2c3d4-5678-90ab-cdef-333333333333": {
				"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/staging-web-alb/5555555555aaaaaa",
			},
			// Associated, so the empty ACL carries the no-rules finding
			// alone rather than also reading as an orphan.
			"arn:aws:wafv2:us-east-1:123456789012:regional/webacl/" + WAFNoRules + "/a1b2c3d4-5678-90ab-cdef-444444444444": {
				"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/internal-api-alb/6666666666bbbbbb",
			},
		},
	}
})

func NewWAFFixtures() *WAFFixtures {
	return sharedWAFFixtures()
}

func init() {
	Register(Pin{ShortName: "waf", Rows: 5, Issues: 0, CoverageGaps: []string{"broken", "dim"}})
}
