// aws_ses_invalidation_test.go — Cache-invalidation regression tests for SES.
//
// Verifies that InvalidateSESRuleSetCache(clients) correctly clears the per-client
// receipt-rule-set cache so that the next checker call retries the API.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestInvalidateSESRuleSetCache verifies that after a successful
// DescribeActiveReceiptRuleSet response is cached, calling
// InvalidateSESRuleSetCache(clients) forces the next request to
// retry the API call.
//
// Behaviour pinned:
//   1. First checker call: API called (counter = 1), valid rule set returned.
//   2. Second checker call: cache hit, no new API call (counter still 1).
//   3. Call InvalidateSESRuleSetCache(clients).
//   4. Third checker call: cache miss, API called again (counter = 2).
//   5. Returns same rule-set output (fake always returns the same fixture).
func TestInvalidateSESRuleSetCache(t *testing.T) {
	// Build a simple rule set with one global LambdaAction so the checker walk
	// succeeds and returns Count=1 each time.
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			{
				Name:       aws.String("global-rule"),
				Recipients: nil, // applies to all identities
				Actions: []sestypes.ReceiptAction{
					{LambdaAction: &sestypes.LambdaAction{
						FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:ses-inbound"),
					}},
				},
			},
		},
	}

	v1Mock := &fakeSESV1{
		responses: []sesV1Response{
			// Repeated success — always returns the same rule set.
			{output: ruleSetOutput, err: nil},
		},
	}

	// Use a single *ServiceClients pointer — sesRuleSetCaches uses the pointer as key.
	clients := &awsclient.ServiceClients{SES: v1Mock}

	src := resource.Resource{
		ID:     "any@example.com",
		Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
	}

	checker := sesCheckerByTarget(t, "lambda")

	// ---- Call 1: first call; must hit the API. ----
	result1 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result1.Err != nil {
		t.Fatalf("call 1: unexpected error: %v", result1.Err)
	}
	if result1.Count != 1 {
		t.Errorf("call 1: Count = %d, want 1", result1.Count)
	}
	if v1Mock.calls != 1 {
		t.Errorf("after call 1: mock.calls = %d, want 1", v1Mock.calls)
	}

	// ---- Call 2: cache hit; must NOT call the API again. ----
	result2 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result2.Err != nil {
		t.Fatalf("call 2: unexpected error: %v", result2.Err)
	}
	if result2.Count != 1 {
		t.Errorf("call 2: Count = %d, want 1 (cached)", result2.Count)
	}
	if v1Mock.calls != 1 {
		t.Errorf("after call 2: mock.calls = %d, want 1 (cache must absorb call 2)", v1Mock.calls)
	}

	// ---- Invalidate the cache. ----
	awsclient.InvalidateSESRuleSetCache(clients)

	// ---- Call 3: cache miss after invalidation; API must be called again. ----
	result3 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result3.Err != nil {
		t.Fatalf("call 3: unexpected error: %v", result3.Err)
	}
	if result3.Count != 1 {
		t.Errorf("call 3: Count = %d, want 1 (fresh fetch after invalidation)", result3.Count)
	}
	if v1Mock.calls != 2 {
		t.Errorf("after call 3: mock.calls = %d, want 2 (invalidation must force a new API call)", v1Mock.calls)
	}
}
