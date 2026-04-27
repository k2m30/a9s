package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ses"
)

// SESActiveReceiptRuleSetForTest is a test-only export of sesActiveReceiptRuleSet.
// It exists in a non-test file so tests/unit/ (a different package) can call it.
// The production binary cost is negligible — one extra exported symbol.
func SESActiveReceiptRuleSetForTest(ctx context.Context, c *ServiceClients) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	return sesActiveReceiptRuleSet(ctx, c)
}
