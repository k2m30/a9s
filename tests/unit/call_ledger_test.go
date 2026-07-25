package unit

// call_ledger_test.go — regression coverage for core/aws/call_ledger.go: the
// CallLedger makes coalesce.go's "at most one call per (operation, api,
// arguments)" contract mechanically checkable. Reuses aws_coalesce_test.go's
// fakes (coalesceSfnFake, coalesceS3Fake, coalesceLambdaFake) and its
// concurrentGateSleep idiom — same package, same file's block-channel
// pattern, no new fakes needed.
//
// Landed-code note (re-read before trusting the concurrent-callers pin): all
// four decorators (coalescingSFN/SNS/S3/Lambda) capture
// singleflight.Group.Do's `shared` return value and record a CallServed for
// a goroutine that JOINED another's in-flight call without executing it
// itself (`if shared && !executed { ledger.record(..., CallServed) }`) —
// join-recording is symmetric across all four, not SFN-only. A joiner is
// therefore visible in Records() both when it arrives concurrently (joins
// the in-flight singleflight call) and when it arrives sequentially after
// completion (hits completedResultMemo) — TestLambdaGetFunction_ECRCheckerAndEnricher_Ledger_BothAskersVisible
// drives the real checker+enricher SEQUENTIALLY (mirroring
// TestLambdaGetFunction_ECRCheckerAndEnricher_ShareOneUnderlyingCallPerOperation
// and coalesce.go's own doc comment on the real web-drain sequence), and
// TestCallLedger_LambdaConcurrentJoin_BothAskersVisible drives the same
// scenario CONCURRENTLY, both landing on (executed=1, served=1).
import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// clCountOutcomes tallies how many records in recs carry each CallOutcome.
func clCountOutcomes(recs []awsclient.CallRecord) (executed, served int) {
	for _, r := range recs {
		switch r.Outcome {
		case awsclient.CallExecuted:
			executed++
		case awsclient.CallServed:
			served++
		}
	}
	return executed, served
}

// ---------------------------------------------------------------------------
// General ledger contract — driven via SFN (the one decorator with full
// join-recording), so "N concurrent callers" exercises exactly what's landed.
// ---------------------------------------------------------------------------

func TestCallLedger_ConcurrentCallers_OneExecutedNMinusOneServed(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	const n = 8
	fake := &coalesceSfnFake{describeBlock: make(chan struct{})}
	ledger := awsclient.NewCallLedger()
	decorated := awsclient.NewCoalescingSFNWithLedger(fake, ledger)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(5))

	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			_, _ = decorated.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)})
		}()
	}
	time.Sleep(concurrentGateSleep)
	close(fake.describeBlock)
	wg.Wait()

	recs := ledger.Records()
	executed, served := clCountOutcomes(recs)
	if executed != 1 {
		t.Errorf("CallExecuted count = %d, want 1 (exactly one underlying call for %d concurrent identical callers)", executed, n)
	}
	if served != n-1 {
		t.Errorf("CallServed count = %d, want %d (every joiner must be recorded)", served, n-1)
	}
	for _, r := range recs {
		if r.Operation != domain.Gen(5) || r.API != "sfn.DescribeStateMachine" || r.Args != arn {
			t.Errorf("record %+v does not carry the expected (Operation=5, API=sfn.DescribeStateMachine, Args=%q)", r, arn)
		}
	}
}

func TestCallLedger_SequentialSameOp_OneExecutedOneServed(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	fake := &coalesceSfnFake{}
	ledger := awsclient.NewCallLedger()
	decorated := awsclient.NewCoalescingSFNWithLedger(fake, ledger)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(5))

	if _, err := decorated.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)}); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if _, err := decorated.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)}); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	executed, served := clCountOutcomes(ledger.Records())
	if executed != 1 || served != 1 {
		t.Errorf("outcomes = (executed=%d, served=%d), want (1, 1) — the memo path", executed, served)
	}
}

func TestCallLedger_DifferentOperations_EachExecutes(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	fake := &coalesceSfnFake{}
	ledger := awsclient.NewCallLedger()
	decorated := awsclient.NewCoalescingSFNWithLedger(fake, ledger)
	ctxOp1 := awsclient.WithDetailOp(context.Background(), domain.Gen(1))
	ctxOp2 := awsclient.WithDetailOp(context.Background(), domain.Gen(2))

	if _, err := decorated.DescribeStateMachine(ctxOp1, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)}); err != nil {
		t.Fatalf("op1 call error: %v", err)
	}
	if _, err := decorated.DescribeStateMachine(ctxOp2, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)}); err != nil {
		t.Fatalf("op2 call error: %v", err)
	}

	executed, served := clCountOutcomes(ledger.Records())
	if executed != 2 || served != 0 {
		t.Errorf("outcomes = (executed=%d, served=%d), want (2, 0) — two different operations never coalesce with each other", executed, served)
	}
	if dups := ledger.Duplicates(); len(dups) != 0 {
		t.Errorf("Duplicates() = %+v, want empty — each operation has exactly one execution of its own", dups)
	}
}

// TestCallLedger_OperationZero_NeverCountsAsDuplicate pins call_ledger.go's
// own documented carve-out: opID 0 (no active DetailOperation) is never
// memoized by coalesce.go, so repeated fetches under it are ordinary
// behavior — Duplicates() must not misreport them.
func TestCallLedger_OperationZero_NeverCountsAsDuplicate(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	fake := &coalesceSfnFake{}
	ledger := awsclient.NewCallLedger()
	decorated := awsclient.NewCoalescingSFNWithLedger(fake, ledger)

	if _, err := decorated.DescribeStateMachine(context.Background(), &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)}); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if _, err := decorated.DescribeStateMachine(context.Background(), &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)}); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	executed, served := clCountOutcomes(ledger.Records())
	if executed != 2 || served != 0 {
		t.Fatalf("precondition: outcomes = (executed=%d, served=%d), want (2, 0) — opID 0 is never memoized, so both calls must re-execute", executed, served)
	}
	if dups := ledger.Duplicates(); len(dups) != 0 {
		t.Errorf("Duplicates() = %+v, want empty — operation 0 traffic is excluded by design (never coalesced, so repeats are not a defect)", dups)
	}
}

// TestCallLedger_Duplicates_OnlyCountsExecutions_NeverReportsServed drives a
// GENUINE double-execution (S3's retryable-error path is never memoized —
// TestNewCoalescingS3_SequentialCalls_RetryableError_NotMemoizedWithinOp
// already pins this at the coalesce.go level) alongside a normal
// successful-and-memoized key in the SAME operation, and asserts
// Duplicates() reports only the genuinely double-executed key — a
// CallServed entry sharing a key must never inflate the count.
func TestCallLedger_Duplicates_OnlyCountsExecutions_NeverReportsServed(t *testing.T) {
	const retryBucket = "retry-bucket"
	const normalBucket = "normal-bucket"
	retryableErr := &smithy.GenericAPIError{Code: "InternalError", Message: "we messed up"}
	fake := &coalesceS3Fake{
		getPolicyFn: func(in *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
			if aws.ToString(in.Bucket) == retryBucket {
				return nil, retryableErr
			}
			return &s3.GetBucketPolicyOutput{Policy: in.Bucket}, nil
		},
	}
	ledger := awsclient.NewCallLedger()
	decorated := awsclient.NewCoalescingS3WithLedger(fake, ledger)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(5))

	// Retryable error: never memoized, so both sequential calls genuinely
	// re-execute — a real duplicate.
	if _, err := decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(retryBucket)}); err == nil {
		t.Fatal("first retry-bucket call: expected the retryable error, got nil")
	}
	if _, err := decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(retryBucket)}); err == nil {
		t.Fatal("second retry-bucket call: expected the retryable error, got nil")
	}

	// Normal successful key: memoized after the first call — one execution,
	// one serve, NOT a duplicate.
	if _, err := decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(normalBucket)}); err != nil {
		t.Fatalf("first normal-bucket call error: %v", err)
	}
	if _, err := decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(normalBucket)}); err != nil {
		t.Fatalf("second normal-bucket call error: %v", err)
	}

	dups := ledger.Duplicates()
	if len(dups) != 1 {
		t.Fatalf("Duplicates() = %+v, want exactly 1 entry (retry-bucket's genuine double-execution)", dups)
	}
	for rec, count := range dups {
		if rec.Args != retryBucket {
			t.Errorf("Duplicates() reported key %+v, want Args=%q (normal-bucket's served entry must never appear here)", rec, retryBucket)
		}
		if rec.Outcome != awsclient.CallExecuted {
			t.Errorf("Duplicates() key %+v has Outcome=%v, want CallExecuted — Duplicates() must never key on CallServed", rec, rec.Outcome)
		}
		if count != 2 {
			t.Errorf("Duplicates()[%+v] = %d, want 2", rec, count)
		}
	}
}

// ---------------------------------------------------------------------------
// The real regression: checkLambdaECR + enrichLambda for the same function,
// one operation — the exact defect that reached main in the enrichment work.
// Mirrors TestLambdaGetFunction_ECRCheckerAndEnricher_ShareOneUnderlyingCallPerOperation
// (aws_coalesce_test.go) verbatim, swapping in the ledger-enabled
// constructor and asserting on Records() rather than only the fake's raw
// call count.
// ---------------------------------------------------------------------------

func TestLambdaGetFunction_ECRCheckerAndEnricher_Ledger_BothAskersVisible(t *testing.T) {
	const fnName = "image-fn"
	const op = domain.Gen(9)
	fake := &coalesceLambdaFake{}
	ledger := awsclient.NewCallLedger()
	decorated := awsclient.NewCoalescingLambdaWithLedger(fake, ledger)
	sc := &awsclient.ServiceClients{Lambda: decorated}

	ctx := awsclient.WithDetailOp(context.Background(), op)
	res := resource.Resource{
		ID:        fnName,
		Fields:    map[string]string{"package_type": "Image"},
		RawStruct: lambdatypes.FunctionConfiguration{FunctionName: aws.String(fnName)},
	}

	ecrChecker := coalesceLambdaECRDefByTarget(t)
	ecrResult := ecrChecker(ctx, sc, res, nil)
	if ecrResult.State() == domain.RelatedUnknown {
		t.Fatalf("ecr related check returned Unknown, want a resolved result driving a real GetFunction call: %+v", ecrResult)
	}

	enricher := resource.GetDetailEnricher("lambda")
	if enricher == nil {
		t.Fatal("lambda detail enricher not registered")
	}
	dctx := &awsclient.DetailEnrichmentCtx{Clients: sc, OpID: op}
	if _, err := enricher(ctx, dctx, res); err != nil {
		t.Fatalf("enrichLambda: unexpected error: %v", err)
	}

	if got := fake.getFunctionCalls.Load(); got != 1 {
		t.Fatalf("GetFunction reached the inner fake %d times across the ecr related checker + the detail enricher under ONE operation, want 1", got)
	}

	recs := ledger.Records()
	executed, served := clCountOutcomes(recs)
	if executed != 1 || served != 1 {
		t.Fatalf("outcomes = (executed=%d, served=%d), want (1, 1) — BOTH the checker's ask and the enricher's ask must be visible in Records()", executed, served)
	}
	for _, r := range recs {
		if r.Operation != op || r.API != "lambda.GetFunction" || r.Args != fnName {
			t.Errorf("record %+v does not carry the expected (Operation=%d, API=lambda.GetFunction, Args=%q)", r, op, fnName)
		}
	}
}

// TestCallLedger_LambdaConcurrentJoin_BothAskersVisible is the CONCURRENT
// twin of TestLambdaGetFunction_ECRCheckerAndEnricher_Ledger_BothAskersVisible:
// two goroutines racing on the identical FunctionName under one operation —
// one executes, one joins the in-flight singleflight call — and both must be
// visible in Records(), exactly like coalescingSFN's already-covered
// concurrent axis (join-recording is symmetric across all four decorators;
// see this file's header comment).
func TestCallLedger_LambdaConcurrentJoin_BothAskersVisible(t *testing.T) {
	const fnName = "process-payment"
	fake := &coalesceLambdaFake{getFunctionBlock: make(chan struct{})}
	ledger := awsclient.NewCallLedger()
	decorated := awsclient.NewCoalescingLambdaWithLedger(fake, ledger)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(3))

	var wg sync.WaitGroup
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			_, _ = decorated.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(fnName)})
		}()
	}
	time.Sleep(concurrentGateSleep)
	close(fake.getFunctionBlock)
	wg.Wait()

	if got := fake.getFunctionCalls.Load(); got != 1 {
		t.Fatalf("GetFunction reached the inner fake %d times across 2 concurrent identical calls, want 1", got)
	}
	executed, served := clCountOutcomes(ledger.Records())
	if executed != 1 || served != 1 {
		t.Errorf("outcomes = (executed=%d, served=%d), want (1, 1) — the joining goroutine must be recorded too, not just the executing one", executed, served)
	}
}

// ---------------------------------------------------------------------------
// Operation-0 gap (coalesceCall's memoHit/memo.set both gate on opID != 0,
// but g.Do is called unconditionally): no automated pin existed for either
// half before this — an "obviously fine" branch that silently changes shape
// during the next refactor of coalesce.go. The sequential case is pinned
// side by side with the non-zero contrast (TestCallLedger_SequentialSameOp_
// OneExecutedOneServed's own SFN case, deliberately repeated here rather than
// only cross-referenced) so a reader sees both behaviors in one place;
// covers SFN and Lambda so this is a property of the shared coalesceCall
// helper, not one client.
// ---------------------------------------------------------------------------

// TestCallLedger_OpIDZero_SequentialCalls_NeverMemoized_VsNonZeroMemoizes
// pins the two sequential behaviors coalesceCall's opID guard is responsible
// for, per decorator: opID 0 never hits the memo (ordinary list/probe
// traffic must re-execute every time, never be silently cached), while a
// real (non-zero) operation memoizes its second identical call.
func TestCallLedger_OpIDZero_SequentialCalls_NeverMemoized_VsNonZeroMemoizes(t *testing.T) {
	const sfnArn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	const fnName = "process-payment"

	cases := []struct {
		name         string
		opID         domain.Gen
		run          func(ctx context.Context) *awsclient.CallLedger
		wantExecuted int
		wantServed   int
	}{
		{
			name: "sfn/opID-zero-both-calls-execute",
			opID: 0,
			run: func(ctx context.Context) *awsclient.CallLedger {
				fake := &coalesceSfnFake{}
				ledger := awsclient.NewCallLedger()
				decorated := awsclient.NewCoalescingSFNWithLedger(fake, ledger)
				for range 2 {
					_, _ = decorated.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(sfnArn)})
				}
				return ledger
			},
			wantExecuted: 2,
			wantServed:   0,
		},
		{
			name: "sfn/opID-nonzero-second-call-served-from-memo",
			opID: 11,
			run: func(ctx context.Context) *awsclient.CallLedger {
				fake := &coalesceSfnFake{}
				ledger := awsclient.NewCallLedger()
				decorated := awsclient.NewCoalescingSFNWithLedger(fake, ledger)
				for range 2 {
					_, _ = decorated.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(sfnArn)})
				}
				return ledger
			},
			wantExecuted: 1,
			wantServed:   1,
		},
		{
			name: "lambda/opID-zero-both-calls-execute",
			opID: 0,
			run: func(ctx context.Context) *awsclient.CallLedger {
				fake := &coalesceLambdaFake{}
				ledger := awsclient.NewCallLedger()
				decorated := awsclient.NewCoalescingLambdaWithLedger(fake, ledger)
				for range 2 {
					_, _ = decorated.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(fnName)})
				}
				return ledger
			},
			wantExecuted: 2,
			wantServed:   0,
		},
		{
			name: "lambda/opID-nonzero-second-call-served-from-memo",
			opID: 11,
			run: func(ctx context.Context) *awsclient.CallLedger {
				fake := &coalesceLambdaFake{}
				ledger := awsclient.NewCallLedger()
				decorated := awsclient.NewCoalescingLambdaWithLedger(fake, ledger)
				for range 2 {
					_, _ = decorated.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(fnName)})
				}
				return ledger
			},
			wantExecuted: 1,
			wantServed:   1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := awsclient.WithDetailOp(context.Background(), tc.opID)
			ledger := tc.run(ctx)
			executed, served := clCountOutcomes(ledger.Records())
			if executed != tc.wantExecuted || served != tc.wantServed {
				t.Errorf("outcomes = (executed=%d, served=%d), want (%d, %d)", executed, served, tc.wantExecuted, tc.wantServed)
			}
		})
	}
}

// TestCallLedger_OpIDZero_ConcurrentCalls_StillDedupViaSingleflight is the
// CONCURRENT twin of the sequential opID-0 case above: opID 0 bypasses the
// memo entirely, but g.Do is still called unconditionally, so two genuinely
// concurrent identical callers under opID 0 must still coalesce into one
// execution — ordinary (non-operation) traffic gets the same in-flight
// dedup as operation-scoped traffic, just never the memo's "already
// finished" half. Uses the file's existing blocked-fake + concurrentGateSleep
// idiom so the overlap is deterministic rather than hoped for. Covers SFN
// and Lambda.
func TestCallLedger_OpIDZero_ConcurrentCalls_StillDedupViaSingleflight(t *testing.T) {
	const sfnArn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	const fnName = "process-payment"

	t.Run("sfn", func(t *testing.T) {
		fake := &coalesceSfnFake{describeBlock: make(chan struct{})}
		ledger := awsclient.NewCallLedger()
		decorated := awsclient.NewCoalescingSFNWithLedger(fake, ledger)
		ctx := context.Background() // opID 0 — no WithDetailOp

		var wg sync.WaitGroup
		wg.Add(2)
		for range 2 {
			go func() {
				defer wg.Done()
				_, _ = decorated.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(sfnArn)})
			}()
		}
		time.Sleep(concurrentGateSleep)
		close(fake.describeBlock)
		wg.Wait()

		if got := fake.describeCalls.Load(); got != 1 {
			t.Fatalf("DescribeStateMachine reached the inner fake %d times across 2 concurrent opID-0 calls, want 1", got)
		}
		executed, served := clCountOutcomes(ledger.Records())
		if executed != 1 || served != 1 {
			t.Errorf("outcomes = (executed=%d, served=%d), want (1, 1) — opID 0 still dedups CONCURRENT identical callers via singleflight, even though it is never memoized for sequential ones", executed, served)
		}
	})

	t.Run("lambda", func(t *testing.T) {
		fake := &coalesceLambdaFake{getFunctionBlock: make(chan struct{})}
		ledger := awsclient.NewCallLedger()
		decorated := awsclient.NewCoalescingLambdaWithLedger(fake, ledger)
		ctx := context.Background() // opID 0 — no WithDetailOp

		var wg sync.WaitGroup
		wg.Add(2)
		for range 2 {
			go func() {
				defer wg.Done()
				_, _ = decorated.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(fnName)})
			}()
		}
		time.Sleep(concurrentGateSleep)
		close(fake.getFunctionBlock)
		wg.Wait()

		if got := fake.getFunctionCalls.Load(); got != 1 {
			t.Fatalf("GetFunction reached the inner fake %d times across 2 concurrent opID-0 calls, want 1", got)
		}
		executed, served := clCountOutcomes(ledger.Records())
		if executed != 1 || served != 1 {
			t.Errorf("outcomes = (executed=%d, served=%d), want (1, 1) — opID 0 still dedups CONCURRENT identical callers via singleflight", executed, served)
		}
	})
}
