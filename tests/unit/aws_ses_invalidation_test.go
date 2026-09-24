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

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
)

func TestInvalidateSESRuleSetCache(t *testing.T) {
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			{
				Name:       aws.String("global-rule"),
				Enabled:    true,
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
			{output: ruleSetOutput, err: nil},
		},
	}

	// The rule-set cache lives on c.RuleSets(), so each test wires its own store.
	clients := &awsclient.ServiceClients{SES: v1Mock}
	clients.SetRuleSets(session.NewRuleSetStore())

	src := resource.Resource{
		ID:     "any@example.com",
		Fields: map[string]string{"identity_type": "email address"},
	}

	checker := sesCheckerByTarget(t, "lambda")

	result1 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result1.Err() != nil {
		t.Fatalf("call 1: unexpected error: %v", result1.Err())
	}
	if result1.Count() != 1 {
		t.Errorf("call 1: Count = %d, want 1", result1.Count())
	}
	if v1Mock.calls != 1 {
		t.Errorf("after call 1: mock.calls = %d, want 1", v1Mock.calls)
	}

	result2 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result2.Err() != nil {
		t.Fatalf("call 2: unexpected error: %v", result2.Err())
	}
	if result2.Count() != 1 {
		t.Errorf("call 2: Count = %d, want 1 (cached)", result2.Count())
	}
	if v1Mock.calls != 1 {
		t.Errorf("after call 2: mock.calls = %d, want 1 (cache must absorb call 2)", v1Mock.calls)
	}

	// Ctrl+R swaps the store rather than clearing it, so an in-flight fetcher
	// cannot re-poison the active one.
	clients.SetRuleSets(session.NewRuleSetStore())

	result3 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if result3.Err() != nil {
		t.Fatalf("call 3: unexpected error: %v", result3.Err())
	}
	if result3.Count() != 1 {
		t.Errorf("call 3: Count = %d, want 1 (fresh fetch after invalidation)", result3.Count())
	}
	if v1Mock.calls != 2 {
		t.Errorf("after call 3: mock.calls = %d, want 2 (invalidation must force a new API call)", v1Mock.calls)
	}
}

func TestHandleRefresh_SESDetailViewInvalidatesRuleSetCache(t *testing.T) {
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			{
				Name:       aws.String("global-rule"),
				Enabled:    true,
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

	// The TUI model and the checker share one *ServiceClients, and so one store.
	clients := &awsclient.ServiceClients{SES: v1Mock}
	clients.SetRuleSets(session.NewRuleSetStore())

	src := resource.Resource{
		ID:     "any@example.com",
		Fields: map[string]string{"identity_type": "email address"},
	}

	checker := sesCheckerByTarget(t, "lambda")

	r1 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if r1.Count() != 1 {
		t.Errorf("call 1: Count = %d, want 1", r1.Count())
	}
	if v1Mock.calls != 1 {
		t.Fatalf("pre-condition: expected 1 API call after seeding cache, got %d", v1Mock.calls)
	}

	r2 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if r2.Count() != 1 {
		t.Errorf("call 2: Count = %d, want 1 (cached)", r2.Count())
	}
	if v1Mock.calls != 1 {
		t.Fatalf("pre-condition: cache miss on call 2, mock.calls = %d, want 1", v1Mock.calls)
	}

	applyMsg := func(m tui.Model, msg tea.Msg) tui.Model {
		newM, _ := m.Update(msg)
		return newM.(tui.Model)
	}

	sesRes := resource.Resource{
		ID:     "any@example.com",
		Name:   "any@example.com",
		Fields: map[string]string{"identity_type": "email address"},
	}

	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(clients),
		tui.WithNoCache(true),
	)
	m = applyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Init() emits ClientsReady as a command, and no event loop runs here.
	// ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m = applyMsg(m, messages.ClientsReady{Clients: clients, Gen: 1})

	m = applyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ses",
		Resource:     &sesRes,
	})

	m = applyMsg(m, tea.KeyPressMsg{Code: -1, Text: "\x12"})
	_ = m

	r3 := checker(context.Background(), clients, src, resource.ResourceCache{})
	if r3.Count() != 1 {
		t.Errorf("call 3: Count = %d, want 1 (fresh fetch)", r3.Count())
	}
	if v1Mock.calls != 2 {
		t.Errorf("after Ctrl+R on ses detail view: mock.calls = %d, want 2 "+
			"(handleRefresh must swap the RuleSets store for ses detail views)", v1Mock.calls)
	}
}

// blockingSESV1 blocks DescribeActiveReceiptRuleSet until releaseCh is
// closed: a slow upstream that answers after a concurrent refresh swapped the
// cache.
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

// A DescribeActiveReceiptRuleSet that returns after Ctrl+R swapped
// c.RuleSets writes to the orphaned old store, so the next checker run
// fetches fresh instead of serving stale Lambda/S3 relationships.
func TestSESRuleSetSwap_LateWriterDoesNotPoisonNewStore(t *testing.T) {
	staleOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{{
			Name:       aws.String("stale-rule"),
			Enabled:    true,
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
		Fields: map[string]string{"identity_type": "email address"},
	}
	checker := sesCheckerByTarget(t, "lambda")

	checkerDone := make(chan struct{})
	go func() {
		defer close(checkerDone)
		_ = checker(context.Background(), clients, src, resource.ResourceCache{})
	}()

	<-v1Mock.enteredCh

	oldStore := clients.RuleSets()

	clients.SetRuleSets(session.NewRuleSetStore())

	close(v1Mock.releaseCh)
	<-checkerDone

	if _, ok := clients.RuleSets().Get(); ok {
		t.Errorf("new RuleSets store has cached entry — late writer poisoned the active slot")
	}
	// The write landing on the orphaned store proves the fake ran Set.
	if _, ok := oldStore.Get(); !ok {
		t.Errorf("orphaned old store has NO cached entry — late writer didn't fire; test is vacuous")
	}
}

// Concurrent callers that miss the cache share one upstream call: if one
// succeeded and a sibling transiently failed (throttle / 5xx), the failing
// checker would report RelatedError while the cache holds the answer.

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

var _ awsclient.SESV1API = (*atomicBlockingSESV1)(nil)

func TestSESActiveReceiptRuleSet_Singleflight_CoalescesConcurrentMisses(t *testing.T) {
	const N = 5

	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			{
				Name:       aws.String("coalesced-rule"),
				Enabled:    true,
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

	// started is a barrier: each goroutine signals before entering the SES
	// helper, so all N are scheduled before the stub is released.
	started := make(chan struct{}, N)
	for i := range N {
		go func(idx int) {
			defer wg.Done()
			started <- struct{}{} // signal: "I'm about to call"
			out, err := awsclient.SESActiveReceiptRuleSetForTest(context.Background(), clients)
			results[idx] = result{out: out, err: err}
		}(i)
	}

	for range N {
		<-started
	}
	<-mock.inFlightCh
	// Yield repeatedly to give the remaining N-1 goroutines a chance to reach
	// the singleflight wait point before we release. Bounded at 100 iterations
	// so a stuck test fails fast rather than hanging the suite indefinitely.
	for i := 0; i < 100; i++ {
		runtime.Gosched()
	}

	close(mock.releaseCh)
	wg.Wait()

	if got := mock.calls.Load(); got != 1 {
		t.Errorf("DescribeActiveReceiptRuleSet called %d times, want 1 — singleflight not coalescing concurrent misses", got)
	}

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

// The singleflight fetch is detached from the leader's ctx: a canceled
// leader must not hand context.Canceled to a follower whose own ctx is live.

// ctxAwareSESV1 blocks each call on releaseCh or the call's ctx.Done,
// whichever fires first, and returns ctx.Err() on cancellation.
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

var _ awsclient.SESV1API = (*ctxAwareSESV1)(nil)

// Either 1 or 2 upstream calls is correct: the fetcher may detach from the
// leader's ctx so the one in-flight call completes for everyone, or retry
// with a context-independent ctx when the leader's ctx fires. The leader may
// receive context.Canceled or a result shared from a background retry.
func TestSESActiveReceiptRuleSet_Singleflight_LeaderCancelDoesNotPoisonFollower(t *testing.T) {
	ruleSetOutput := &ses.DescribeActiveReceiptRuleSetOutput{
		Rules: []sestypes.ReceiptRule{
			{
				Name:       aws.String("follower-rule"),
				Enabled:    true,
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

	ctxA, cancelA := context.WithCancel(context.Background())
	go func() {
		defer wg.Done()
		out, err := awsclient.SESActiveReceiptRuleSetForTest(ctxA, clients)
		resA = result{out: out, err: err}
	}()

	<-mock.inFlightCh
	for range 10 {
		runtime.Gosched()
	}

	// bStarted is closed immediately before B calls the helper, giving us a
	// deterministic signal that B has been scheduled.
	bStarted := make(chan struct{})
	go func() {
		defer wg.Done()
		close(bStarted)
		out, err := awsclient.SESActiveReceiptRuleSetForTest(context.Background(), clients)
		resB = result{out: out, err: err}
	}()

	// Bounded yields let B reach the singleflight wait point before A is
	// canceled; a stuck test fails fast instead of hanging the suite.
	<-bStarted
	for range 100 {
		runtime.Gosched()
	}

	// A singleflight coupled to the leader's ctx would hand ctxA's error to B.
	cancelA()

	// Any detached fetch finishes the work for B; wg.Wait() orders both
	// goroutines before the checks.
	close(mock.releaseCh)

	wg.Wait()

	if resB.err != nil {
		t.Errorf("follower errB = %v, want nil — follower's ctx was not canceled; leader cancellation must not propagate to follower", resB.err)
	}

	if resB.out == nil {
		t.Errorf("follower outB = nil, want non-nil successful result")
	}

	if resB.out != nil {
		if len(resB.out.Rules) == 0 {
			t.Errorf("follower outB.Rules is empty, want at least 1 rule")
		} else if got, want := aws.ToString(resB.out.Rules[0].Name), "follower-rule"; got != want {
			t.Errorf("follower outB.Rules[0].Name = %q, want %q", got, want)
		}
	}

	// More than 2 calls means an unbounded retry loop.
	if got := mock.calls.Load(); got < 1 || got > 2 {
		t.Errorf("DescribeActiveReceiptRuleSet called %d times, want 1 or 2 (coalesced or detached re-fetch)", got)
	}

	_ = resA
}

// AWS SES DescribeActiveReceiptRuleSet returns (nil, nil) when no rule set is
// active. That answer is not cached, so the next call fetches again.

// nilReturnSESV1 is a minimal SESV1API mock that always returns (nil, nil)
// — emulating an AWS account with no active SES receipt rule set.
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

var _ awsclient.SESV1API = (*nilReturnSESV1)(nil)

func TestSESActiveReceiptRuleSet_NilResultIsNotCached(t *testing.T) {
	mock := &nilReturnSESV1{}

	clients := &awsclient.ServiceClients{SES: mock}
	clients.SetRuleSets(session.NewRuleSetStore())

	ctx := context.Background()

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

	out2, err2 := awsclient.SESActiveReceiptRuleSetForTest(ctx, clients)
	if err2 != nil {
		t.Fatalf("call 2: unexpected error: %v", err2)
	}
	if out2 != nil {
		t.Fatalf("call 2: expected nil output, got %v", out2)
	}
	if got := mock.calls.Load(); got != 2 {
		t.Errorf("mock.calls = %d, want 2 — nil result was cached as a success (sticky nil regression)", got)
	}
}
