package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ses"
)

// SESActiveReceiptRuleSetForTest is an exported test-only wrapper for the
// unexported sesActiveReceiptRuleSet — production code does not call it.
// Lives outside _test.go because tests in tests/unit/ are package unit_test
// and can't see same-package test helpers.
//
// Post-AS-660: takes *Scope so the test exercises the same store-acquisition
// path the production checker uses.
func SESActiveReceiptRuleSetForTest(ctx context.Context, s *Scope) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	if s == nil {
		return sesActiveReceiptRuleSet(ctx, nil, nil)
	}
	return sesActiveReceiptRuleSet(ctx, s.Clients, s.RuleSets)
}
