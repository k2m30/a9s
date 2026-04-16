package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// WAFFake implements aws.WAFv2API against fixture data loaded at construction time.
type WAFFake struct {
	fix *fixtures.WAFFixtures
}

// NewWAF constructs a WAFFake backed by fixture data from the fixtures package.
func NewWAF() *WAFFake {
	return &WAFFake{fix: fixtures.NewWAFFixtures()}
}

func (f *WAFFake) ListWebACLs(_ context.Context, _ *wafv2.ListWebACLsInput, _ ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error) {
	return &wafv2.ListWebACLsOutput{WebACLs: f.fix.WebACLSummaries}, nil
}

func (f *WAFFake) ListResourcesForWebACL(_ context.Context, input *wafv2.ListResourcesForWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.ListResourcesForWebACLOutput, error) {
	if input.WebACLArn == nil {
		return nil, fmt.Errorf("ListResourcesForWebACL: WebACL ARN is required")
	}
	arns := f.fix.ResourcesByWebACL[*input.WebACLArn]
	return &wafv2.ListResourcesForWebACLOutput{ResourceArns: arns}, nil
}

// GetLoggingConfiguration returns a stub logging configuration.
// In demo mode the staging WAF ACL has no logging configured
// (returns WAFNonexistentItemException), triggering the finding.
// Other ACLs return a dummy LoggingConfiguration.
func (f *WAFFake) GetLoggingConfiguration(_ context.Context, input *wafv2.GetLoggingConfigurationInput, _ ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error) {
	if input.ResourceArn == nil {
		return nil, fmt.Errorf("GetLoggingConfiguration: ResourceArn is required")
	}
	const stagingARN = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-staging-waf/a1b2c3d4-5678-90ab-cdef-333333333333"
	if *input.ResourceArn == stagingARN {
		return nil, &wafv2types.WAFNonexistentItemException{
			Message: aws.String("The referenced item doesn't exist"),
		}
	}
	return &wafv2.GetLoggingConfigurationOutput{
		LoggingConfiguration: &wafv2types.LoggingConfiguration{
			ResourceArn:         input.ResourceArn,
			LogDestinationConfigs: []string{"arn:aws:firehose:us-east-1:123456789012:deliverystream/aws-waf-logs-acme"},
		},
	}, nil
}
