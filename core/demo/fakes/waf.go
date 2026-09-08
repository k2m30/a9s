// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"fmt"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// WAFFake implements aws.WAFv2API against fixture data loaded at construction time.
type WAFFake struct {
	fix *fixtures.WAFFixtures
}

// NewWAF constructs a WAFFake backed by fixture data from the fixtures package.
func NewWAF() *WAFFake {
	return &WAFFake{fix: fixtures.NewWAFFixtures()}
}

// ListWebACLs respects the Scope input: Scope=CLOUDFRONT returns only the
// global CLOUDFRONT-scope fixtures; anything else (including the zero value)
// returns the REGIONAL fixtures, matching the real ListWebACLs default.
func (f *WAFFake) ListWebACLs(_ context.Context, input *wafv2.ListWebACLsInput, _ ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error) {
	if input != nil && input.Scope == wafv2types.ScopeCloudfront {
		return &wafv2.ListWebACLsOutput{WebACLs: f.fix.CloudFrontWebACLSummaries}, nil
	}
	return &wafv2.ListWebACLsOutput{WebACLs: f.fix.WebACLSummaries}, nil
}

func (f *WAFFake) ListResourcesForWebACL(_ context.Context, input *wafv2.ListResourcesForWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.ListResourcesForWebACLOutput, error) {
	if input.WebACLArn == nil {
		return nil, fmt.Errorf("ListResourcesForWebACL: WebACL ARN is required")
	}
	if err := validateARN(*input.WebACLArn); err != nil {
		return nil, err
	}
	arns := f.fix.ResourcesByWebACL[*input.WebACLArn]
	return &wafv2.ListResourcesForWebACLOutput{ResourceArns: arns}, nil
}

// GetWebACL returns a stub WebACL with a rules summary for demo mode.
// The prod and cloudfront ACLs have 3 rules; the staging ACL has 1.
func (f *WAFFake) GetWebACL(_ context.Context, input *wafv2.GetWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.GetWebACLOutput, error) {
	if input.Name == nil {
		return nil, fmt.Errorf("GetWebACL: Name is required")
	}
	if !f.hasWebACL(aws.ToString(input.Id), *input.Name) {
		return nil, &wafv2types.WAFNonexistentItemException{
			Message: notFoundMessage("WebACL", *input.Name),
		}
	}
	ruleCount := 3
	switch *input.Name {
	case "acme-staging-waf":
		ruleCount = 1
	case fixtures.WAFNoRules:
		ruleCount = 0
	}
	rules := make([]wafv2types.Rule, ruleCount)
	for i := range rules {
		rules[i] = wafv2types.Rule{Name: aws.String(fmt.Sprintf("rule-%d", i+1))}
	}
	return &wafv2.GetWebACLOutput{
		WebACL: &wafv2types.WebACL{
			Id:    input.Id,
			Name:  input.Name,
			Rules: rules,
		},
	}, nil
}

// GetWebACLForResource performs a reverse lookup over ResourcesByWebACL to
// find which WebACL (if any) protects the given resource ARN, backing the
// elb→waf related-panel pivot (checkELBWAF).
func (f *WAFFake) GetWebACLForResource(_ context.Context, input *wafv2.GetWebACLForResourceInput, _ ...func(*wafv2.Options)) (*wafv2.GetWebACLForResourceOutput, error) {
	if input == nil || input.ResourceArn == nil {
		return nil, fmt.Errorf("GetWebACLForResource: ResourceArn is required")
	}
	if err := validateARN(*input.ResourceArn); err != nil {
		return nil, err
	}
	for webACLArn, resourceArns := range f.fix.ResourcesByWebACL {
		for _, arn := range resourceArns {
			if arn != *input.ResourceArn {
				continue
			}
			for i := range f.fix.WebACLSummaries {
				summary := f.fix.WebACLSummaries[i]
				if summary.ARN == nil || *summary.ARN != webACLArn {
					continue
				}
				return &wafv2.GetWebACLForResourceOutput{
					WebACL: &wafv2types.WebACL{
						Id:   summary.Id,
						Name: summary.Name,
						ARN:  summary.ARN,
					},
				}, nil
			}
		}
	}
	return &wafv2.GetWebACLForResourceOutput{}, nil
}

// GetLoggingConfiguration returns a stub logging configuration.
// In demo mode the staging WAF ACL has no logging configured
// (returns WAFNonexistentItemException), triggering the finding.
// Other ACLs return a dummy LoggingConfiguration.
func (f *WAFFake) GetLoggingConfiguration(_ context.Context, input *wafv2.GetLoggingConfigurationInput, _ ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error) {
	if input.ResourceArn == nil {
		return nil, fmt.Errorf("GetLoggingConfiguration: ResourceArn is required")
	}
	if err := validateARN(*input.ResourceArn); err != nil {
		return nil, err
	}
	const stagingARN = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-staging-waf/a1b2c3d4-5678-90ab-cdef-333333333333"
	if *input.ResourceArn == stagingARN {
		return nil, &wafv2types.WAFNonexistentItemException{
			Message: aws.String("The referenced item doesn't exist"),
		}
	}
	return &wafv2.GetLoggingConfigurationOutput{
		LoggingConfiguration: &wafv2types.LoggingConfiguration{
			ResourceArn:           input.ResourceArn,
			LogDestinationConfigs: []string{"arn:aws:firehose:us-east-1:123456789012:deliverystream/aws-waf-logs-acme"},
		},
	}, nil
}

// hasWebACL reports whether the fixtures register this web ACL, in either
// scope. GetWebACL is keyed by id AND name, so both must match one summary.
func (f *WAFFake) hasWebACL(id, name string) bool {
	match := func(s wafv2types.WebACLSummary) bool {
		return aws.ToString(s.Id) == id && aws.ToString(s.Name) == name
	}
	return slices.ContainsFunc(f.fix.WebACLSummaries, match) ||
		slices.ContainsFunc(f.fix.CloudFrontWebACLSummaries, match)
}
