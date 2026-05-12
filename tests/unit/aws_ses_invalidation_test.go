// aws_ses_invalidation_test.go — Cache-invalidation regression tests for SES.
//
// Verifies that InvalidateSESRuleSetCache(clients) correctly clears the per-client
// receipt-rule-set cache so that the next checker call retries the API.
//
// Also contains Pin 2: regression pin verifying that Ctrl+R on a detail view for
// a ses resource type calls InvalidateSESRuleSetCache, so the next related-panel
// check re-fetches the receipt-rule-set instead of serving stale cached data.
//
// Also contains Pin 3: singleflight coalescing regression pin verifying that N
// concurrent callers of SESActiveReceiptRuleSetForTest (which delegates to the
// unexported sesActiveReceiptRuleSet) result in exactly 1 upstream API call when
// singleflight is in place.
//
// NOTE TO CODER: This test requires the following exported test helper to be added
// to internal/aws/ses_related.go (or a new ses_related_export_test.go file):
//
//	// SESActiveReceiptRuleSetForTest is a test-only export of sesActiveReceiptRuleSet.
//	func SESActiveReceiptRuleSetForTest(ctx context.Context, c *ServiceClients) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
//	    return sesActiveReceiptRuleSet(ctx, c)
//	}
//
// The test will fail to compile until this export exists.
package unit_test

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// TestInvalidateSESRuleSetCache verifies that after a successful
// DescribeActiveReceiptRuleSet response is cached, calling
// InvalidateSESRuleSetCache(clients) forces the next request to
// retry the API call.
//
// Behaviour pinned:
//  1. First checker call: API called (counter = 1), valid rule set returned.
//  2. Second checker call: cache hit, no new API call (counter still 1).
//  3. Call InvalidateSESRuleSetCache(clients).
//  4. Third checker call: cache miss, API called again (counter = 2).
//  5. Returns same rule-set output (fake always returns the same fixture).
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
	// rule set cache lives on c.RuleSets() rather than a process-global map
	// keyed by *ServiceClients pointer). Use SetRuleSets because the field
	// is unexported (CR-flagged race fix).
	clients := &awsclient.ServiceClients{SES: v1Mock}
	clients.SetRuleSets(session.NewRuleSetStore())

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

	// ---- Invalidate the cache by swapping the store. ----
	// Post-PR-02d (P2 fix): swap rather than Clear() so in-flight blocked
	// fetchers can't re-poison the active store. Production code does this
	// on Ctrl+R for SES detail/list views; the test mirrors that pattern.
	clients.SetRuleSets(session.NewRuleSetStore())

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
	// rule set cache lives on c.RuleSets()). Reuse one *ServiceClients pointer
	// so that the TUI model and the checker share the same store reference.
	clients := &awsclient.ServiceClients{SES: v1Mock}
	clients.SetRuleSets(session.NewRuleSetStore())

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
	m = applyMsg(m, messages.ClientsReady{Clients: clients})

	// Push an SES detail view onto the stack.
	m = applyMsg(m, messages.Navigate{
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
			"(handleRefresh must swap the RuleSets store for ses detail views)", v1Mock.calls)
	}
}

// blockingSESV1 implements SESV1API but blocks the call inside
// DescribeActiveReceiptRuleSet until releaseCh is closed. Used to
// simulate a slow upstream API that doesn't return until after a
// concurrent refresh has invalidated the cache.
type blockingSESV1 struct {
	releaseCh chan struct{}
	enteredCh chan struct{} // closed once the call is in-flight
	output    *ses.DescribeActiveReceiptRuleSetOutput
	calls     int
}

func (b *blockingSESV1) DescribeActiveReceiptRuleSet(
	_ context.Context,
	_ *ses.DescribeActiveReceiptRuleSetInput,
	_ ...func(*ses.Options),
) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	b.calls++
	select {
	case <-b.enteredCh:
		// already closed
	default:
		close(b.enteredCh)
	}
	<-b.releaseCh
	return b.output, nil
}

// TestSESRuleSetSwap_LateWriterDoesNotPoisonNewStore pins the P2 fix:
// when an in-flight DescribeActiveReceiptRuleSet call is blocked, and the
// caller swaps `c.RuleSets` for a fresh store (the production
// invalidation pattern from Ctrl+R), the late writer's Set must land on
// the orphaned old store — NOT on the new active slot. Otherwise the
// next checker run would see stale Lambda/S3 relationships even after
// the user explicitly refreshed.
//
// Pre-fix behaviour (in-place Clear()): Set lands on the same store and
// repopulates the active cache → next checker sees stale data.
//
// Post-fix behaviour (swap + capture): Set lands on the captured (now
// orphaned) store → new active store stays empty → next checker fetches
// fresh.
func TestSESRuleSetSwap_LateWriterDoesNotPoisonNewStore(t *testing.T) {
	staleOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{{
			Name:       aws.String("stale-rule"),
			Recipients: nil,
			Actions: []sestypes.ReceiptAction{
				{LambdaAction: &sestypes.LambdaAction{
					FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:stale"),
				}},
			},
		}},
	}

	v1Mock := &blockingSESV1{
		releaseCh: make(chan struct{}),
		enteredCh: make(chan struct{}),
		output:    staleOutput,
	}

	clients := &awsclient.ServiceClients{SES: v1Mock}
	clients.SetRuleSets(session.NewRuleSetStore())

	src := resource.Resource{
		ID:     "any@example.com",
		Fields: map[string]string{"identity_type": "EMAIL_ADDRESS"},
	}
	checker := sesCheckerByTarget(t, "lambda")

	// Goroutine A: blocked checker.
	checkerDone := make(chan struct{})
	go func() {
		defer close(checkerDone)
		_ = checker(context.Background(), clients, src, resource.ResourceCache{})
	}()

	// Wait for the blocked call to enter the SES API stub.
	<-v1Mock.enteredCh

	// Capture the OLD store reference so we can verify the late writer
	// targeted it (not the new one).
	oldStore := clients.RuleSets()

	// Refresh: swap to a fresh store (production Ctrl+R pattern).
	clients.SetRuleSets(session.NewRuleSetStore())

	// Release the blocked call. Goroutine A returns and writes its result.
	close(v1Mock.releaseCh)
	<-checkerDone

	// Pin: the new active store is empty. The late writer pollute the
	// orphaned old store, not the new one.
	if _, ok := clients.RuleSets().Get(); ok {
		t.Errorf("new RuleSets store has cached entry — late writer poisoned the active slot")
	}
	// Confirm the orphaned store DID receive the write (so we know the
	// fake actually executed Set; otherwise the test could be vacuously
	// passing).
	if _, ok := oldStore.Get(); !ok {
		t.Errorf("orphaned old store has NO cached entry — late writer didn't fire; test is vacuous")
	}
}

// ---------------------------------------------------------------------------
// PIN 3 — singleflight coalescing of concurrent sesActiveReceiptRuleSet callers
// ---------------------------------------------------------------------------
// Pre-fix: two concurrent callers both observe a cache miss and both invoke
// DescribeActiveReceiptRuleSet. When one succeeds and the sibling transiently
// fails (throttle / 5xx), the failing checker returns Count:-1 even though the
// cache now holds the successful result.
//
// Post-fix: singleflight ensures exactly one upstream call is issued; all
// concurrent waiters share the single result.
//
// This test calls the unexported sesActiveReceiptRuleSet via the exported
// test wrapper SESActiveReceiptRuleSetForTest. The coder MUST add that
// wrapper to internal/aws/ses_related.go (see the package-level NOTE at the
// top of this file). Until the wrapper exists the test fails to compile —
// that IS the intended red light for Plan B.

// atomicBlockingSESV1 is a goroutine-safe SES v1 mock that:
//   - counts calls atomically
//   - blocks every DescribeActiveReceiptRuleSet call on releaseCh
//   - signals inFlightCh when at least one call is in progress
type atomicBlockingSESV1 struct {
	calls      atomic.Int32
	releaseCh  chan struct{}
	inFlightCh chan struct{} // closed once the first call enters the stub
	closeOnce  sync.Once
	output     *ses.DescribeActiveReceiptRuleSetOutput
}

func (a *atomicBlockingSESV1) DescribeActiveReceiptRuleSet(
	_ context.Context,
	_ *ses.DescribeActiveReceiptRuleSetInput,
	_ ...func(*ses.Options),
) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	a.calls.Add(1)
	a.closeOnce.Do(func() { close(a.inFlightCh) })
	<-a.releaseCh
	return a.output, nil
}

// Compile-time check: atomicBlockingSESV1 satisfies SESV1API.
var _ awsclient.SESV1API = (*atomicBlockingSESV1)(nil)

// TestSESActiveReceiptRuleSet_Singleflight_CoalescesConcurrentMisses verifies
// that N concurrent callers of sesActiveReceiptRuleSet result in exactly 1
// upstream API invocation when singleflight coalescing is in place.
//
// Expected red light (pre-fix): a.calls == 5 (one per goroutine) — the assertion
// `a.calls.Load() == 1` fires, failing the test.
//
// Expected green light (post-fix): a.calls == 1; all goroutines received the
// same successful output.
func TestSESActiveReceiptRuleSet_Singleflight_CoalescesConcurrentMisses(t *testing.T) {
	const N = 5

	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			{
				Name:       aws.String("coalesced-rule"),
				Recipients: nil, // global — applies to all identities
				Actions: []sestypes.ReceiptAction{
					{LambdaAction: &sestypes.LambdaAction{
						FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:coalesced"),
					}},
				},
			},
		},
	}

	mock := &atomicBlockingSESV1{
		releaseCh:  make(chan struct{}),
		inFlightCh: make(chan struct{}),
		output:     ruleSetOutput,
	}

	clients := &awsclient.ServiceClients{SES: mock}
	clients.SetRuleSets(session.NewRuleSetStore())

	type result struct {
		out *ses.DescribeActiveReceiptRuleSetOutput
		err error
	}
	results := make([]result, N)
	var wg sync.WaitGroup
	wg.Add(N)

	// started is a hard barrier: each goroutine signals it BEFORE entering
	// the SES helper, giving us a strong guarantee that all N goroutines have
	// been scheduled before we release the stub. This eliminates the flaky
	// 50ms sleep while still being bounded (100 Gosched iterations cap the
	// wait so a stuck test fails fast instead of hanging the suite).
	started := make(chan struct{}, N)
	for i := range N {
		go func(idx int) {
			defer wg.Done()
			started <- struct{}{} // signal: "I'm about to call"
			out, err := awsclient.SESActiveReceiptRuleSetForTest(context.Background(), clients)
			results[idx] = result{out: out, err: err}
		}(i)
	}

	// Wait for all N goroutines to signal they are about to call.
	for range N {
		<-started
	}
	// Wait for the leader to enter the blocking stub.
	<-mock.inFlightCh
	// Yield repeatedly to give the remaining N-1 goroutines a chance to reach
	// the singleflight wait point before we release. Bounded at 100 iterations
	// so a stuck test fails fast rather than hanging the suite indefinitely.
	for i := 0; i < 100; i++ {
		runtime.Gosched()
	}

	// Release all blocked callers.
	close(mock.releaseCh)
	wg.Wait()

	// With singleflight, the API must have been called exactly once.
	if got := mock.calls.Load(); got != 1 {
		t.Errorf("DescribeActiveReceiptRuleSet called %d times, want 1 — singleflight not coalescing concurrent misses", got)
	}

	// Every goroutine must have received a non-nil, non-error result with the
	// expected rule-set name (no caller should see a failure while another succeeded).
	for i, r := range results {
		if r.err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, r.err)
			continue
		}
		if r.out == nil {
			t.Errorf("goroutine %d: got nil output, want non-nil", i)
			continue
		}
		if len(r.out.Rules) == 0 {
			t.Errorf("goroutine %d: got empty Rules, want at least 1", i)
			continue
		}
		if got, want := aws.ToString(r.out.Rules[0].Name), "coalesced-rule"; got != want {
			t.Errorf("goroutine %d: rule name = %q, want %q", i, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// PIN 4 — singleflight ctx-coupling: leader cancellation must NOT poison followers
// ---------------------------------------------------------------------------
// Bug (post-PR-307): sesActiveReceiptRuleSet passes the caller's ctx directly
// into the singleflight.Group.Do closure. When the leader's ctx is canceled,
// the singleflight fetcher aborts with context.Canceled and propagates that
// error to every follower — even followers whose own ctx was not canceled.
//
// Pre-fix behaviour: follower receives errB == context.Canceled (leader's error).
// Post-fix behaviour: follower receives a successful result despite leader cancellation.

// ctxAwareSESV1 is a ctx-respecting SES v1 mock used only for PIN 4.
// It blocks each call on releaseCh OR the call's ctx.Done — whichever fires first.
// When ctx fires, it returns ctx.Err() so the caller observes cancellation.
// calls is atomic so concurrent goroutines can increment it safely.
type ctxAwareSESV1 struct {
	calls      atomic.Int32
	releaseCh  chan struct{}
	inFlightCh chan struct{} // closed once the first call enters
	closeOnce  sync.Once
	output     *ses.DescribeActiveReceiptRuleSetOutput
}

func (c *ctxAwareSESV1) DescribeActiveReceiptRuleSet(
	ctx context.Context,
	_ *ses.DescribeActiveReceiptRuleSetInput,
	_ ...func(*ses.Options),
) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	c.calls.Add(1)
	c.closeOnce.Do(func() { close(c.inFlightCh) })
	select {
	case <-c.releaseCh:
		return c.output, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Compile-time check: ctxAwareSESV1 satisfies SESV1API.
var _ awsclient.SESV1API = (*ctxAwareSESV1)(nil)

// TestSESActiveReceiptRuleSet_Singleflight_LeaderCancelDoesNotPoisonFollower
// is the regression pin for the ctx-coupling bug introduced in PR #307.
//
// The invariant pinned here is: a follower goroutine with a long-lived ctx
// must succeed even when the singleflight leader's ctx is canceled before
// the upstream API call completes.
//
// Regarding call count: we accept 1 or 2 upstream calls. A correct fix may
// either (a) detach the fetcher from the leader's ctx so the single in-flight
// call completes for everyone (1 call), or (b) retry with a context-independent
// ctx when the leader's ctx fires (2 calls). Both are valid — we pin only the
// follower-success invariant, not the exact implementation strategy.
//
// Leader outcome (errA): NOT asserted strictly. Depending on the fix, the leader
// may receive context.Canceled (if it cancelled before the API responded) or a
// successful result (if the fix retries on a background ctx and shares the result).
// Leader's outcome is an impl-detail; follower's success is the regression pin.
func TestSESActiveReceiptRuleSet_Singleflight_LeaderCancelDoesNotPoisonFollower(t *testing.T) {
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			{
				Name:       aws.String("follower-rule"),
				Recipients: nil, // global — applies to all identities
				Actions: []sestypes.ReceiptAction{
					{LambdaAction: &sestypes.LambdaAction{
						FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:follower"),
					}},
				},
			},
		},
	}

	mock := &ctxAwareSESV1{
		releaseCh:  make(chan struct{}),
		inFlightCh: make(chan struct{}),
		output:     ruleSetOutput,
	}

	clients := &awsclient.ServiceClients{SES: mock}
	clients.SetRuleSets(session.NewRuleSetStore())

	type result struct {
		out *ses.DescribeActiveReceiptRuleSetOutput
		err error
	}
	var (
		resA, resB result
		wg         sync.WaitGroup
	)
	wg.Add(2)

	// Goroutine A — the leader. Its ctx will be canceled while the API is blocking.
	ctxA, cancelA := context.WithCancel(context.Background())
	go func() {
		defer wg.Done()
		out, err := awsclient.SESActiveReceiptRuleSetForTest(ctxA, clients)
		resA = result{out: out, err: err}
	}()

	// Wait for goroutine A to enter the blocking stub.
	<-mock.inFlightCh
	// A is now inside the fetcher closure inside singleflight.Do. A few yields
	// for paranoia (singleflight bookkeeping after fetcher invocation).
	for range 10 {
		runtime.Gosched()
	}

	// Goroutine B — the follower. Uses a long-lived context (never canceled).
	// bStarted is closed immediately before B calls the helper, giving us a
	// deterministic signal that B has been scheduled.
	bStarted := make(chan struct{})
	go func() {
		defer wg.Done()
		close(bStarted)
		out, err := awsclient.SESActiveReceiptRuleSetForTest(context.Background(), clients)
		resB = result{out: out, err: err}
	}()

	// Wait for B to start, then yield aggressively to let it reach the
	// singleflight DoChan wait point before we cancel A.
	// 100 yields matches PIN 3's barrier idiom; bounded so a stuck test
	// fails fast rather than hanging the suite indefinitely.
	<-bStarted
	for range 100 {
		runtime.Gosched()
	}

	// Cancel the leader's ctx. This is the trigger that exposes the bug:
	// a naive singleflight implementation propagates ctxA's error to B.
	cancelA()

	// Release the mock — any continuing detached fetch (Fix 1: WithoutCancel)
	// finishes the work for B. wg.Wait() below ensures both goroutines finish
	// before we check results, so no sleep is needed after cancelA.
	close(mock.releaseCh)

	// Wait for both goroutines to finish.
	wg.Wait()

	// ---- REGRESSION PIN (follower must succeed) ----

	// errB must be nil: the follower's context was never canceled.
	// Pre-fix failure: errB == context.Canceled (poisoned by leader).
	if resB.err != nil {
		t.Errorf("follower errB = %v, want nil — follower's ctx was not canceled; leader cancellation must not propagate to follower", resB.err)
	}

	// outB must be non-nil when errB is nil.
	if resB.out == nil {
		t.Errorf("follower outB = nil, want non-nil successful result")
	}

	// outB must carry the expected fixture data (not a zero-value struct).
	if resB.out != nil {
		if len(resB.out.Rules) == 0 {
			t.Errorf("follower outB.Rules is empty, want at least 1 rule")
		} else if got, want := aws.ToString(resB.out.Rules[0].Name), "follower-rule"; got != want {
			t.Errorf("follower outB.Rules[0].Name = %q, want %q", got, want)
		}
	}

	// call count must be 1 or 2 — see comment above for rationale.
	// Fewer than 1 is impossible; more than 2 suggests an unbounded retry loop.
	if got := mock.calls.Load(); got < 1 || got > 2 {
		t.Errorf("DescribeActiveReceiptRuleSet called %d times, want 1 or 2 (coalesced or detached re-fetch)", got)
	}

	// leader outcome intentionally not asserted — see package-level comment.
	_ = resA
}

// ---------------------------------------------------------------------------
// PIN 5 — nil result must NOT be cached as a successful entry
// ---------------------------------------------------------------------------
// Regression introduced in the GetOrFetch refactor (PR-307): GetOrFetch calls
// s.Set(v) whenever err == nil, even when v == nil. This stores (ruleSet=nil,
// ok=true). Subsequent Get() calls return (nil, true) and the fetcher is
// never invoked again — the nil result is sticky.
//
// AWS SES DescribeActiveReceiptRuleSet legitimately returns (nil, nil) when
// no rule set is active, so this regression is reachable in production.
//
// Pre-fix expected failure mode: mock.calls == 1 after the second call
// (the nil was cached on the first call; the fetcher is never invoked again).
//
// Post-fix expected pass mode: mock.calls == 2 (the nil result was not cached;
// the fetcher is invoked on the second call too).

// nilReturnSESV1 is a minimal SESV1API mock that always returns (nil, nil)
// — emulating an AWS account with no active SES receipt rule set.
// calls is an atomic counter so the test can assert exact invocation count.
type nilReturnSESV1 struct {
	calls atomic.Int32
}

func (n *nilReturnSESV1) DescribeActiveReceiptRuleSet(
	_ context.Context,
	_ *ses.DescribeActiveReceiptRuleSetInput,
	_ ...func(*ses.Options),
) (*ses.DescribeActiveReceiptRuleSetOutput, error) {
	n.calls.Add(1)
	return nil, nil
}

// Compile-time check: nilReturnSESV1 satisfies SESV1API.
var _ awsclient.SESV1API = (*nilReturnSESV1)(nil)

// TestSESActiveReceiptRuleSet_NilResultIsNotCached is the regression pin for the
// nil-caching bug in GetOrFetch.
//
// The invariant pinned: when the AWS API returns (nil, nil), the store must NOT
// record a successful cache entry. The next call must invoke the fetcher again.
func TestSESActiveReceiptRuleSet_NilResultIsNotCached(t *testing.T) {
	mock := &nilReturnSESV1{}

	clients := &awsclient.ServiceClients{SES: mock}
	clients.SetRuleSets(session.NewRuleSetStore())

	ctx := context.Background()

	// ---- Call 1: fetcher returns (nil, nil). ----
	out1, err1 := awsclient.SESActiveReceiptRuleSetForTest(ctx, clients)
	if err1 != nil {
		t.Fatalf("call 1: unexpected error: %v", err1)
	}
	if out1 != nil {
		t.Fatalf("call 1: expected nil output, got %v", out1)
	}
	if got := mock.calls.Load(); got != 1 {
		t.Fatalf("after call 1: mock.calls = %d, want 1", got)
	}

	// ---- Call 2: nil must NOT have been cached — fetcher must be called again. ----
	out2, err2 := awsclient.SESActiveReceiptRuleSetForTest(ctx, clients)
	if err2 != nil {
		t.Fatalf("call 2: unexpected error: %v", err2)
	}
	if out2 != nil {
		t.Fatalf("call 2: expected nil output, got %v", out2)
	}
	// REGRESSION PIN: pre-fix mock.calls == 1 (sticky nil); post-fix mock.calls == 2.
	if got := mock.calls.Load(); got != 2 {
		t.Errorf("mock.calls = %d, want 2 — nil result was cached as a success (sticky nil regression)", got)
	}
}
