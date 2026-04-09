package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/wafv2"

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
