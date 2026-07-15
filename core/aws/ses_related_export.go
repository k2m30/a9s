package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ses"
)

// SESActiveReceiptRuleSetForTest is an exported test-only wrapper for the
// unexported sesActiveReceiptRuleSet — production code does not call it.
// Lives outside _test.go because tests in tests/unit/ are package unit_test
// and can't see same-package test helpers.
func SESActiveReceiptRuleSetForTest(ctx context.Context, c *ServiceClients) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	return sesActiveReceiptRuleSet(ctx, c)
}
