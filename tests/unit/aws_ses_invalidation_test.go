// aws_ses_invalidation_test.go — Cache-invalidation regression tests for SES.
//
// Verifies that InvalidateSESRuleSetCache(clients) correctly clears the per-client
// receipt-rule-set cache so that the next checker call retries the API.
//
// Also contains Pin 2: regression pin verifying that Ctrl+R on a detail view for
// a ses resource type calls InvalidateSESRuleSetCache, so the next related-panel
// check re-fetches the receipt-rule-set instead of serving stale cached data.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
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

	// Wire a per-test RuleSets store so the cache works (post-PR-02d the
	// rule set cache lives on c.RuleSets rather than a process-global map
	// keyed by *ServiceClients pointer).
	clients := &awsclient.ServiceClients{
		SES:      v1Mock,
		RuleSets: session.NewRuleSetStore(),
	}

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

// ---------------------------------------------------------------------------
// PIN 2 — SES cache invalidation on detail-view Ctrl+R
// ---------------------------------------------------------------------------
// Pre-fix: handleRefresh in app_handlers_navigate.go did NOT call
// InvalidateSESRuleSetCache on the detail-view refresh path (only on the
// resource-list path). This meant Ctrl+R on a detail view for an SES identity
// served stale receipt-rule-set data from the cache.
//
// Post-fix: the detail-view refresh branch also calls InvalidateSESRuleSetCache
// when rt == "ses". This pin verifies that:
//   1. A checker call populates the cache (API call counter = 1).
//   2. A second checker call hits the cache (counter still = 1).
//   3. The TUI model sends Ctrl+R while on a detail view for "ses".
//   4. A third checker call re-fetches from the API (counter = 2).

// TestHandleRefresh_SESDetailViewInvalidatesRuleSetCache verifies that Ctrl+R
// while on a ses-type detail view causes the next ses-lambda checker call to
// re-fetch from the API, proving that the cache was invalidated.
func TestHandleRefresh_SESDetailViewInvalidatesRuleSetCache(t *testing.T) {
	// Build a rule set with one global LambdaAction so the checker returns Count=1.
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			{
				Name:       aws.String("global-rule"),
				Recipients: nil, // global — applies to all identities
				Actions: []sestypes.ReceiptAction{
					{LambdaAction: &sestypes.LambdaAction{
						FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:ses-handler"),
					}},
				},
			},
		},
	}

	v1Mock := &fakeSESV1{
		responses: []sesV1Response{
			{output: ruleSetOutput, err: nil},
		},
	}

	// Wire a per-test RuleSets store so the cache works (post-PR-02d the
	// rule set cache lives on c.RuleSets). Reuse one *ServiceClients pointer
	// so that the TUI model and the checker share the same store reference.
	clients := &awsclient.ServiceClients{
		SES:      v1Mock,
		RuleSets: session.NewRuleSetStore(),
	}

	src := resource.Resource{
		ID:     "any@example.com",
		Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
	}

	checker := sesCheckerByTarget(t, "lambda")

	// ---- Call 1: seed the cache. ----
	r1 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if r1.Count != 1 {
		t.Errorf("call 1: Count = %d, want 1", r1.Count)
	}
	if v1Mock.calls != 1 {
		t.Fatalf("pre-condition: expected 1 API call after seeding cache, got %d", v1Mock.calls)
	}

	// ---- Call 2: cache hit. ----
	r2 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if r2.Count != 1 {
		t.Errorf("call 2: Count = %d, want 1 (cached)", r2.Count)
	}
	if v1Mock.calls != 1 {
		t.Fatalf("pre-condition: cache miss on call 2, mock.calls = %d, want 1", v1Mock.calls)
	}

	// applyMsg applies a message to the TUI model and returns the updated model.
	applyMsg := func(m tui.Model, msg tea.Msg) tui.Model {
		newM, _ := m.Update(msg)
		return newM.(tui.Model)
	}

	// ---- Navigate TUI model to a ses detail view and press Ctrl+R. ----
	sesRes := resource.Resource{
		ID:     "any@example.com",
		Name:   "any@example.com",
		Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
	}

	m := tui.New("demo", "us-east-1",
		tui.WithClients(clients),
		tui.WithNoCache(true),
	)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Wire the pre-supplied clients into m.clients by sending the ClientsReadyMsg
	// that Init() would normally emit as a command (but we don't run the event loop).
	m = applyMsg(m, messages.ClientsReadyMsg{Clients: clients})

	// Push an SES detail view onto the stack.
	m = applyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ses",
		Resource:     &sesRes,
	})

	// Press Ctrl+R while on the ses detail view — must call InvalidateSESRuleSetCache.
	m = applyMsg(m, tea.KeyPressMsg{Code: -1, Text: "\x12"})
	_ = m // model state after refresh is not inspected

	// ---- Call 3: cache must be invalidated. ----
	r3 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if r3.Count != 1 {
		t.Errorf("call 3: Count = %d, want 1 (fresh fetch)", r3.Count)
	}
	if v1Mock.calls != 2 {
		t.Errorf("after Ctrl+R on ses detail view: mock.calls = %d, want 2 "+
			"(handleRefresh must call InvalidateSESRuleSetCache for ses detail views)", v1Mock.calls)
	}
}
