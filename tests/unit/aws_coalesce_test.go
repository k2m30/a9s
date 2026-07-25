package unit

// aws_coalesce_test.go — regression coverage for core/aws/coalesce.go's
// singleflight decorators: NewCoalescingSFN/NewCoalescingSNS/NewCoalescingS3
// each wrap a live client, coalescing concurrent identical calls to exactly
// one narrow-interface method (DescribeStateMachine keyed by StateMachineArn
// / GetTopicAttributes keyed by TopicArn / GetBucketPolicy keyed by Bucket)
// into a single underlying call whose result is shared by every concurrent
// caller on that key. This is in-flight dedup only, never a cache: a call
// made after the previous one already completed always re-executes, and
// concurrent calls with a DIFFERENT key never collide. Every other method —
// including S3/SNS's non-aggregate narrow interfaces (e.g.
// S3GetBucketPolicyAPI/S3GetBucketCorsAPI/S3GetBucketLifecycleAPI, not part
// of S3API itself) that enrichS3/s3_related.go and the sns fetchers
// type-assert at their own call sites — must keep passing straight through,
// since SNSFullAPI/S3FullAPI widen the embedded field far enough for Go's
// automatic interface-method promotion to cover them for free.
//
// Axes per decorator: N concurrent identical-key calls share one underlying
// call and one identical result; two sequential calls both re-execute (no
// caching); two concurrent DIFFERENT-key calls both execute independently;
// a non-coalesced aggregate method passes straight through; the
// non-aggregate narrow interfaces the FullAPI widening exists for are
// reachable through the SAME narrowed static type production code actually
// uses (ServiceClients.SNS/S3 are typed SNSAPI/S3API, not the wide FullAPI),
// then re-asserted back to the narrow interface exactly like
// enrichS3/s3_related.go/the sns fetchers do. SFN has no non-aggregate
// narrow interface (coalesce.go's own doc comment: SFNAPI is already the
// complete aggregate), so its interface-transparency axis is skipped —
// nothing beyond SFNAPI itself to pin.
//
// The concurrent-call axes use golang.org/x/sync/singleflight's own
// canonical test idiom (TestDoDup): launch N goroutines, sleep briefly so
// they all pile up inside the shared in-flight call, then release a gate
// channel the fake's target method blocks on. Every shared counter is an
// atomic.Int64 so this passes under -race.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// concurrentGateSleep gives N goroutines launched just before it time to
// reach the shared in-flight singleflight call before the test releases the
// fake's block channel — the same sleep-then-release idiom
// golang.org/x/sync/singleflight's own TestDoDup uses.
const concurrentGateSleep = 20 * time.Millisecond

// ---------------------------------------------------------------------------
// coalesceSfnFake — implements awsclient.SFNAPI. SFNAPI is already the
// complete SFN aggregate (coalesce.go's doc comment), so no wider FullAPI
// exists for SFN.
// ---------------------------------------------------------------------------

type coalesceSfnFake struct {
	describeCalls atomic.Int64
	describeBlock chan struct{} // non-nil: DescribeStateMachine blocks here before returning
	listCalls     atomic.Int64
}

func (f *coalesceSfnFake) DescribeStateMachine(_ context.Context, in *sfn.DescribeStateMachineInput, _ ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error) {
	f.describeCalls.Add(1)
	if f.describeBlock != nil {
		<-f.describeBlock
	}
	return &sfn.DescribeStateMachineOutput{
		Status: sfntypes.StateMachineStatusActive,
		// RoleArn echoes the input key back so a test can verify a shared
		// result carries the right per-key payload without adding a
		// dedicated field.
		RoleArn: in.StateMachineArn,
	}, nil
}

func (f *coalesceSfnFake) ListStateMachines(_ context.Context, _ *sfn.ListStateMachinesInput, _ ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error) {
	f.listCalls.Add(1)
	return &sfn.ListStateMachinesOutput{}, nil
}
func (f *coalesceSfnFake) ListExecutions(_ context.Context, _ *sfn.ListExecutionsInput, _ ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error) {
	return &sfn.ListExecutionsOutput{}, nil
}
func (f *coalesceSfnFake) GetExecutionHistory(_ context.Context, _ *sfn.GetExecutionHistoryInput, _ ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error) {
	return &sfn.GetExecutionHistoryOutput{}, nil
}
func (f *coalesceSfnFake) ListTagsForResource(_ context.Context, _ *sfn.ListTagsForResourceInput, _ ...func(*sfn.Options)) (*sfn.ListTagsForResourceOutput, error) {
	return &sfn.ListTagsForResourceOutput{}, nil
}

var _ awsclient.SFNAPI = (*coalesceSfnFake)(nil)

// ---------------------------------------------------------------------------
// coalesceSnsFake — implements awsclient.SNSFullAPI (SNSAPI plus the two
// fetcher-only narrow operations SNSFullAPI widens for: ListTopics,
// ListSubscriptions).
// ---------------------------------------------------------------------------

type coalesceSnsFake struct {
	getAttrsCalls          atomic.Int64
	getAttrsBlock          chan struct{} // non-nil: GetTopicAttributes blocks here before returning
	listSubsByTopicCalls   atomic.Int64
	listTopicsCalls        atomic.Int64
	listSubscriptionsCalls atomic.Int64
}

func (f *coalesceSnsFake) GetTopicAttributes(_ context.Context, in *sns.GetTopicAttributesInput, _ ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	f.getAttrsCalls.Add(1)
	if f.getAttrsBlock != nil {
		<-f.getAttrsBlock
	}
	return &sns.GetTopicAttributesOutput{
		Attributes: map[string]string{"TopicArn": aws.ToString(in.TopicArn)},
	}, nil
}

func (f *coalesceSnsFake) ListSubscriptionsByTopic(_ context.Context, _ *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	f.listSubsByTopicCalls.Add(1)
	return &sns.ListSubscriptionsByTopicOutput{}, nil
}
func (f *coalesceSnsFake) ListTagsForResource(_ context.Context, _ *sns.ListTagsForResourceInput, _ ...func(*sns.Options)) (*sns.ListTagsForResourceOutput, error) {
	return &sns.ListTagsForResourceOutput{}, nil
}
func (f *coalesceSnsFake) GetSubscriptionAttributes(_ context.Context, _ *sns.GetSubscriptionAttributesInput, _ ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error) {
	return &sns.GetSubscriptionAttributesOutput{}, nil
}
func (f *coalesceSnsFake) ListTopics(_ context.Context, _ *sns.ListTopicsInput, _ ...func(*sns.Options)) (*sns.ListTopicsOutput, error) {
	f.listTopicsCalls.Add(1)
	return &sns.ListTopicsOutput{}, nil
}
func (f *coalesceSnsFake) ListSubscriptions(_ context.Context, _ *sns.ListSubscriptionsInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsOutput, error) {
	f.listSubscriptionsCalls.Add(1)
	return &sns.ListSubscriptionsOutput{}, nil
}

var _ awsclient.SNSFullAPI = (*coalesceSnsFake)(nil)

// ---------------------------------------------------------------------------
// coalesceS3Fake — implements awsclient.S3FullAPI (S3API plus the six
// narrow operations S3FullAPI widens for).
// ---------------------------------------------------------------------------

type coalesceS3Fake struct {
	getPolicyCalls atomic.Int64
	getPolicyBlock chan struct{} // non-nil: GetBucketPolicy blocks here before returning
	// getPolicyFn, when non-nil, fully overrides GetBucketPolicy's default
	// gated/counted/echo behavior (getPolicyCalls is still incremented) —
	// used by the bypass test, which needs per-call control finer than one
	// shared block channel gives.
	getPolicyFn        func(*s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error)
	listBucketsCalls   atomic.Int64
	getCorsCalls       atomic.Int64
	getLifecycleCalls  atomic.Int64
	getTaggingCalls    atomic.Int64
	getEncryptionCalls atomic.Int64
	getLoggingCalls    atomic.Int64
}

func (f *coalesceS3Fake) GetBucketPolicy(_ context.Context, in *s3.GetBucketPolicyInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	f.getPolicyCalls.Add(1)
	if f.getPolicyFn != nil {
		return f.getPolicyFn(in)
	}
	if f.getPolicyBlock != nil {
		<-f.getPolicyBlock
	}
	return &s3.GetBucketPolicyOutput{
		// Policy echoes the input key back so a test can verify a shared
		// result carries the right per-key payload without adding a
		// dedicated field.
		Policy: in.Bucket,
	}, nil
}

func (f *coalesceS3Fake) ListBuckets(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	f.listBucketsCalls.Add(1)
	return &s3.ListBucketsOutput{}, nil
}
func (f *coalesceS3Fake) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{}, nil
}
func (f *coalesceS3Fake) GetBucketNotificationConfiguration(_ context.Context, _ *s3.GetBucketNotificationConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return &s3.GetBucketNotificationConfigurationOutput{}, nil
}
func (f *coalesceS3Fake) GetPublicAccessBlock(_ context.Context, _ *s3.GetPublicAccessBlockInput, _ ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	return &s3.GetPublicAccessBlockOutput{}, nil
}
func (f *coalesceS3Fake) GetBucketCors(_ context.Context, _ *s3.GetBucketCorsInput, _ ...func(*s3.Options)) (*s3.GetBucketCorsOutput, error) {
	f.getCorsCalls.Add(1)
	return &s3.GetBucketCorsOutput{}, nil
}
func (f *coalesceS3Fake) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	f.getLifecycleCalls.Add(1)
	return &s3.GetBucketLifecycleConfigurationOutput{}, nil
}
func (f *coalesceS3Fake) GetBucketTagging(_ context.Context, _ *s3.GetBucketTaggingInput, _ ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error) {
	f.getTaggingCalls.Add(1)
	return &s3.GetBucketTaggingOutput{}, nil
}
func (f *coalesceS3Fake) GetBucketEncryption(_ context.Context, _ *s3.GetBucketEncryptionInput, _ ...func(*s3.Options)) (*s3.GetBucketEncryptionOutput, error) {
	f.getEncryptionCalls.Add(1)
	return &s3.GetBucketEncryptionOutput{}, nil
}
func (f *coalesceS3Fake) GetBucketLogging(_ context.Context, _ *s3.GetBucketLoggingInput, _ ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error) {
	f.getLoggingCalls.Add(1)
	return &s3.GetBucketLoggingOutput{}, nil
}

var _ awsclient.S3FullAPI = (*coalesceS3Fake)(nil)

// ---------------------------------------------------------------------------
// Sequential one-fetch-per-op memoization (boundary-sealing wave).
//
// TestNewCoalescingSFN_SequentialCalls_BothReexecute above pins the
// pre-existing, still-true axis: two sequential calls under a bare
// context.Background() (opID 0) must both reach the inner fake — "opID 0
// never memoized" is not a new carve-out, it is that exact test, unchanged.
// The two tests below add the genuinely new axis: a NON-ZERO op now
// memoizes a completed result across sequential (non-overlapping) calls —
// singleflight alone (coalesce.go's pre-existing mechanism) cannot do this,
// since a call made after the previous one already returned always starts a
// fresh singleflight entry and re-executes.
// ---------------------------------------------------------------------------

// TestNewCoalescingSFN_SequentialCalls_SameNonZeroOp_MemoizesToOneUnderlyingCall
// drives two SEQUENTIAL calls (the first fully returns before the second
// starts — no goroutines, no gate channel) under the SAME non-zero
// WithDetailOp id and the same StateMachineArn. The inner fake must be
// reached exactly once; the second call's result must still carry the
// correct (memoized) content, proving the second call was served from the
// memo rather than merely also happening to succeed independently.
func TestNewCoalescingSFN_SequentialCalls_SameNonZeroOp_MemoizesToOneUnderlyingCall(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	fake := &coalesceSfnFake{}
	decorated := awsclient.NewCoalescingSFN(fake)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(5))

	first, err := decorated.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)})
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	second, err := decorated.DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)})
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if got := fake.describeCalls.Load(); got != 1 {
		t.Errorf("DescribeStateMachine reached the inner fake %d times across two SEQUENTIAL calls under the same non-zero op, want 1 (completed result must be memoized per (op, key))", got)
	}
	if first == nil || first.RoleArn == nil || *first.RoleArn != arn {
		t.Fatalf("first call result = %+v, want RoleArn echoing %q", first, arn)
	}
	if second == nil || second.RoleArn == nil || *second.RoleArn != arn {
		t.Errorf("second call result = %+v, want the memoized result still echoing RoleArn %q", second, arn)
	}
}

// TestNewCoalescingSFN_SequentialCalls_DifferentNonZeroOps_EachFetchesIndependently
// extends TestWithDetailOp_DifferentOperations_BothExecuteIndependently
// (concurrent axis) to the sequential case: two non-zero operations calling
// the identical underlying key, one strictly after the other completes,
// must each still reach the inner fake — a memo is scoped per (op, key), so
// a different op never rides on an earlier op's memoized entry for the same
// key.
func TestNewCoalescingSFN_SequentialCalls_DifferentNonZeroOps_EachFetchesIndependently(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	fake := &coalesceSfnFake{}
	decorated := awsclient.NewCoalescingSFN(fake)

	ctxOp1 := awsclient.WithDetailOp(context.Background(), domain.Gen(1))
	ctxOp2 := awsclient.WithDetailOp(context.Background(), domain.Gen(2))

	if _, err := decorated.DescribeStateMachine(ctxOp1, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)}); err != nil {
		t.Fatalf("operation 1 call error: %v", err)
	}
	if _, err := decorated.DescribeStateMachine(ctxOp2, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)}); err != nil {
		t.Fatalf("operation 2 call error: %v", err)
	}

	if got := fake.describeCalls.Load(); got != 2 {
		t.Errorf("DescribeStateMachine reached the inner fake %d times for two sequential DIFFERENT non-zero operations on the same key, want 2 (each operation gets its own memo entry)", got)
	}
}

// ---------------------------------------------------------------------------
// SFN: NewCoalescingSFN
// ---------------------------------------------------------------------------

func TestNewCoalescingSFN_ConcurrentIdenticalCalls_ShareOneUnderlyingCallAndResult(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	fake := &coalesceSfnFake{describeBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingSFN(fake)

	const n = 8
	results := make([]*sfn.DescribeStateMachineOutput, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = decorated.DescribeStateMachine(context.Background(), &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)})
		}(i)
	}
	time.Sleep(concurrentGateSleep)
	close(fake.describeBlock)
	wg.Wait()

	if got := fake.describeCalls.Load(); got != 1 {
		t.Errorf("DescribeStateMachine reached the inner fake %d times across %d concurrent identical calls, want 1", got, n)
	}
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("call %d: unexpected error: %v", i, errs[i])
		}
		if results[i] == nil || results[i].RoleArn == nil || *results[i].RoleArn != arn {
			t.Errorf("call %d: result = %+v, want RoleArn echoing %q", i, results[i], arn)
		}
		if results[i] != results[0] {
			t.Errorf("call %d: result pointer %p != call 0's %p — all concurrent callers must share the identical singleflight result", i, results[i], results[0])
		}
	}
}

func TestNewCoalescingSFN_SequentialCalls_BothReexecute(t *testing.T) {
	const arn = "arn:aws:states:us-east-1:123456789012:stateMachine:order-processing"
	fake := &coalesceSfnFake{}
	decorated := awsclient.NewCoalescingSFN(fake)

	if _, err := decorated.DescribeStateMachine(context.Background(), &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)}); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if _, err := decorated.DescribeStateMachine(context.Background(), &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arn)}); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if got := fake.describeCalls.Load(); got != 2 {
		t.Errorf("DescribeStateMachine reached the inner fake %d times across two sequential identical calls, want 2 (no caching, only in-flight dedup)", got)
	}
}

func TestNewCoalescingSFN_ConcurrentDifferentKeys_BothExecuteIndependently(t *testing.T) {
	fake := &coalesceSfnFake{describeBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingSFN(fake)

	arns := [2]string{
		"arn:aws:states:us-east-1:123456789012:stateMachine:order-processing",
		"arn:aws:states:us-east-1:123456789012:stateMachine:payment-processing",
	}
	var errs [2]error
	var wg sync.WaitGroup
	wg.Add(2)
	for i := range 2 {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = decorated.DescribeStateMachine(context.Background(), &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(arns[i])})
		}(i)
	}
	time.Sleep(concurrentGateSleep)
	close(fake.describeBlock)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if got := fake.describeCalls.Load(); got != 2 {
		t.Errorf("DescribeStateMachine reached the inner fake %d times for two concurrent DIFFERENT-key calls, want 2 (different keys must not coalesce)", got)
	}
}

func TestNewCoalescingSFN_PassThrough_ListStateMachinesReachesInnerFake(t *testing.T) {
	fake := &coalesceSfnFake{}
	decorated := awsclient.NewCoalescingSFN(fake)

	if _, err := decorated.ListStateMachines(context.Background(), &sfn.ListStateMachinesInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := fake.listCalls.Load(); got != 1 {
		t.Errorf("ListStateMachines reached the inner fake %d times, want 1 (non-coalesced method must pass straight through)", got)
	}
}

// ---------------------------------------------------------------------------
// SNS: NewCoalescingSNS
// ---------------------------------------------------------------------------

func TestNewCoalescingSNS_ConcurrentIdenticalCalls_ShareOneUnderlyingCallAndResult(t *testing.T) {
	const topicArn = "arn:aws:sns:us-east-1:123456789012:order-events"
	fake := &coalesceSnsFake{getAttrsBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingSNS(fake)

	const n = 8
	results := make([]*sns.GetTopicAttributesOutput, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = decorated.GetTopicAttributes(context.Background(), &sns.GetTopicAttributesInput{TopicArn: aws.String(topicArn)})
		}(i)
	}
	time.Sleep(concurrentGateSleep)
	close(fake.getAttrsBlock)
	wg.Wait()

	if got := fake.getAttrsCalls.Load(); got != 1 {
		t.Errorf("GetTopicAttributes reached the inner fake %d times across %d concurrent identical calls, want 1", got, n)
	}
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("call %d: unexpected error: %v", i, errs[i])
		}
		if results[i] == nil || results[i].Attributes["TopicArn"] != topicArn {
			t.Errorf("call %d: result = %+v, want Attributes[TopicArn] = %q", i, results[i], topicArn)
		}
		if results[i] != results[0] {
			t.Errorf("call %d: result pointer %p != call 0's %p — all concurrent callers must share the identical singleflight result", i, results[i], results[0])
		}
	}
}

func TestNewCoalescingSNS_SequentialCalls_BothReexecute(t *testing.T) {
	const topicArn = "arn:aws:sns:us-east-1:123456789012:order-events"
	fake := &coalesceSnsFake{}
	decorated := awsclient.NewCoalescingSNS(fake)

	if _, err := decorated.GetTopicAttributes(context.Background(), &sns.GetTopicAttributesInput{TopicArn: aws.String(topicArn)}); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if _, err := decorated.GetTopicAttributes(context.Background(), &sns.GetTopicAttributesInput{TopicArn: aws.String(topicArn)}); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if got := fake.getAttrsCalls.Load(); got != 2 {
		t.Errorf("GetTopicAttributes reached the inner fake %d times across two sequential identical calls, want 2 (no caching, only in-flight dedup)", got)
	}
}

// TestNewCoalescingSNS_SequentialCalls_SameNonZeroOp_MemoizesToOneUnderlyingCall
// mirrors TestNewCoalescingSFN_SequentialCalls_SameNonZeroOp_MemoizesToOneUnderlyingCall
// for SNS: two sequential calls under the same non-zero op must reach the
// inner fake exactly once.
func TestNewCoalescingSNS_SequentialCalls_SameNonZeroOp_MemoizesToOneUnderlyingCall(t *testing.T) {
	const topicArn = "arn:aws:sns:us-east-1:123456789012:order-events"
	fake := &coalesceSnsFake{}
	decorated := awsclient.NewCoalescingSNS(fake)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(5))

	if _, err := decorated.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: aws.String(topicArn)}); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if _, err := decorated.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: aws.String(topicArn)}); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if got := fake.getAttrsCalls.Load(); got != 1 {
		t.Errorf("GetTopicAttributes reached the inner fake %d times across two SEQUENTIAL calls under the same non-zero op, want 1 (completed result must be memoized per (op, key))", got)
	}
}

func TestNewCoalescingSNS_ConcurrentDifferentKeys_BothExecuteIndependently(t *testing.T) {
	fake := &coalesceSnsFake{getAttrsBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingSNS(fake)

	arns := [2]string{
		"arn:aws:sns:us-east-1:123456789012:order-events",
		"arn:aws:sns:us-east-1:123456789012:payment-events",
	}
	var errs [2]error
	var wg sync.WaitGroup
	wg.Add(2)
	for i := range 2 {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = decorated.GetTopicAttributes(context.Background(), &sns.GetTopicAttributesInput{TopicArn: aws.String(arns[i])})
		}(i)
	}
	time.Sleep(concurrentGateSleep)
	close(fake.getAttrsBlock)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if got := fake.getAttrsCalls.Load(); got != 2 {
		t.Errorf("GetTopicAttributes reached the inner fake %d times for two concurrent DIFFERENT-key calls, want 2 (different keys must not coalesce)", got)
	}
}

func TestNewCoalescingSNS_PassThrough_ListSubscriptionsByTopicReachesInnerFake(t *testing.T) {
	fake := &coalesceSnsFake{}
	decorated := awsclient.NewCoalescingSNS(fake)

	if _, err := decorated.ListSubscriptionsByTopic(context.Background(), &sns.ListSubscriptionsByTopicInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := fake.listSubsByTopicCalls.Load(); got != 1 {
		t.Errorf("ListSubscriptionsByTopic reached the inner fake %d times, want 1 (non-coalesced aggregate method must pass straight through)", got)
	}
}

func TestNewCoalescingSNS_InterfaceTransparency_NonAggregateNarrowInterfaces(t *testing.T) {
	fake := &coalesceSnsFake{}
	decorated := awsclient.NewCoalescingSNS(fake)

	// Narrow to SNSAPI first — exactly ServiceClients.SNS's static field
	// type — then re-assert to the narrow interfaces the way the sns/
	// sns_subscriptions fetchers actually do at their call sites. SNSAPI
	// itself does not embed ListTopics/ListSubscriptions (sns_interfaces.go);
	// only SNSFullAPI's widening keeps this assertion alive through the
	// decorator.
	var snsAPI awsclient.SNSAPI = decorated

	listTopicsAPI, ok := snsAPI.(awsclient.SNSListTopicsAPI)
	if !ok {
		t.Fatal("NewCoalescingSNS's result, narrowed to SNSAPI, does not satisfy SNSListTopicsAPI — the sns fetcher's type-assertion would break")
	}
	if _, err := listTopicsAPI.ListTopics(context.Background(), &sns.ListTopicsInput{}); err != nil {
		t.Fatalf("ListTopics: unexpected error: %v", err)
	}
	if got := fake.listTopicsCalls.Load(); got != 1 {
		t.Errorf("ListTopics reached the inner fake %d times, want 1", got)
	}

	listSubsAPI, ok := snsAPI.(awsclient.SNSListSubscriptionsAPI)
	if !ok {
		t.Fatal("NewCoalescingSNS's result, narrowed to SNSAPI, does not satisfy SNSListSubscriptionsAPI — the sns_subscriptions fetcher's type-assertion would break")
	}
	if _, err := listSubsAPI.ListSubscriptions(context.Background(), &sns.ListSubscriptionsInput{}); err != nil {
		t.Fatalf("ListSubscriptions: unexpected error: %v", err)
	}
	if got := fake.listSubscriptionsCalls.Load(); got != 1 {
		t.Errorf("ListSubscriptions reached the inner fake %d times, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// S3: NewCoalescingS3
// ---------------------------------------------------------------------------

func TestNewCoalescingS3_ConcurrentIdenticalCalls_ShareOneUnderlyingCallAndResult(t *testing.T) {
	const bucket = "acme-app-logs-prod"
	fake := &coalesceS3Fake{getPolicyBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingS3(fake)

	const n = 8
	results := make([]*s3.GetBucketPolicyOutput, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = decorated.GetBucketPolicy(context.Background(), &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
		}(i)
	}
	time.Sleep(concurrentGateSleep)
	close(fake.getPolicyBlock)
	wg.Wait()

	if got := fake.getPolicyCalls.Load(); got != 1 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times across %d concurrent identical calls, want 1", got, n)
	}
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("call %d: unexpected error: %v", i, errs[i])
		}
		if results[i] == nil || results[i].Policy == nil || *results[i].Policy != bucket {
			t.Errorf("call %d: result = %+v, want Policy echoing %q", i, results[i], bucket)
		}
		if results[i] != results[0] {
			t.Errorf("call %d: result pointer %p != call 0's %p — all concurrent callers must share the identical singleflight result", i, results[i], results[0])
		}
	}
}

func TestNewCoalescingS3_SequentialCalls_BothReexecute(t *testing.T) {
	const bucket = "acme-app-logs-prod"
	fake := &coalesceS3Fake{}
	decorated := awsclient.NewCoalescingS3(fake)

	if _, err := decorated.GetBucketPolicy(context.Background(), &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if _, err := decorated.GetBucketPolicy(context.Background(), &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if got := fake.getPolicyCalls.Load(); got != 2 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times across two sequential identical calls, want 2 (no caching, only in-flight dedup)", got)
	}
}

// TestNewCoalescingS3_SequentialCalls_SameNonZeroOp_MemoizesToOneUnderlyingCall
// mirrors TestNewCoalescingSFN_SequentialCalls_SameNonZeroOp_MemoizesToOneUnderlyingCall
// for S3's successful-result axis: two sequential calls under the same
// non-zero op must reach the inner fake exactly once.
func TestNewCoalescingS3_SequentialCalls_SameNonZeroOp_MemoizesToOneUnderlyingCall(t *testing.T) {
	const bucket = "acme-app-logs-prod"
	fake := &coalesceS3Fake{}
	decorated := awsclient.NewCoalescingS3(fake)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(5))

	if _, err := decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if _, err := decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if got := fake.getPolicyCalls.Load(); got != 1 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times across two SEQUENTIAL calls under the same non-zero op, want 1 (completed result must be memoized per (op, key))", got)
	}
}

// TestNewCoalescingS3_SequentialCalls_BenignNoSuchBucketPolicy_MemoizedWithinOp
// pins coalescingS3's benign-absence memoization split (#261 boundary-sealing
// wave, item c): a NoSuchBucketPolicy error — the common, expected "no
// policy attached" case — is a definitive answer for the rest of the
// operation, so a second sequential call under the SAME op must be served
// from the memo, not re-fetched.
func TestNewCoalescingS3_SequentialCalls_BenignNoSuchBucketPolicy_MemoizedWithinOp(t *testing.T) {
	const bucket = "acme-app-logs-prod"
	benignErr := &smithy.GenericAPIError{Code: "NoSuchBucketPolicy", Message: "no policy"}
	fake := &coalesceS3Fake{
		getPolicyFn: func(_ *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
			return nil, benignErr
		},
	}
	decorated := awsclient.NewCoalescingS3(fake)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(5))

	_, err1 := decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	_, err2 := decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})

	if got := fake.getPolicyCalls.Load(); got != 1 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times across two SEQUENTIAL calls under the same op with a benign NoSuchBucketPolicy result, want 1 (memoized)", got)
	}
	if !errors.Is(err1, benignErr) {
		t.Errorf("first call error = %v, want the benign NoSuchBucketPolicy error", err1)
	}
	if !errors.Is(err2, benignErr) {
		t.Errorf("second call error = %v, want the SAME memoized benign NoSuchBucketPolicy error", err2)
	}
}

// TestNewCoalescingS3_SequentialCalls_RetryableError_NotMemoizedWithinOp pins
// the other half of the split: a genuinely retryable error (anything other
// than the benign absence case) must never be memoized — the next call
// within the same operation must retry against the inner fake, not
// permanently pin a transient failure for the operation's remaining
// lifetime.
func TestNewCoalescingS3_SequentialCalls_RetryableError_NotMemoizedWithinOp(t *testing.T) {
	const bucket = "acme-app-logs-prod"
	retryableErr := &smithy.GenericAPIError{Code: "InternalError", Message: "we messed up"}
	fake := &coalesceS3Fake{
		getPolicyFn: func(_ *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
			return nil, retryableErr
		},
	}
	decorated := awsclient.NewCoalescingS3(fake)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(5))

	if _, err := decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)}); err == nil {
		t.Fatal("first call: expected the retryable error, got nil")
	}
	if _, err := decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)}); err == nil {
		t.Fatal("second call: expected the retryable error, got nil")
	}

	if got := fake.getPolicyCalls.Load(); got != 2 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times across two SEQUENTIAL calls under the same op with a retryable error, want 2 (never memoized)", got)
	}
}

func TestNewCoalescingS3_ConcurrentDifferentKeys_BothExecuteIndependently(t *testing.T) {
	fake := &coalesceS3Fake{getPolicyBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingS3(fake)

	buckets := [2]string{"acme-app-logs-prod", "acme-app-assets-prod"}
	var errs [2]error
	var wg sync.WaitGroup
	wg.Add(2)
	for i := range 2 {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = decorated.GetBucketPolicy(context.Background(), &s3.GetBucketPolicyInput{Bucket: aws.String(buckets[i])})
		}(i)
	}
	time.Sleep(concurrentGateSleep)
	close(fake.getPolicyBlock)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if got := fake.getPolicyCalls.Load(); got != 2 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times for two concurrent DIFFERENT-key calls, want 2 (different keys must not coalesce)", got)
	}
}

func TestNewCoalescingS3_PassThrough_ListBucketsReachesInnerFake(t *testing.T) {
	fake := &coalesceS3Fake{}
	decorated := awsclient.NewCoalescingS3(fake)

	if _, err := decorated.ListBuckets(context.Background(), &s3.ListBucketsInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := fake.listBucketsCalls.Load(); got != 1 {
		t.Errorf("ListBuckets reached the inner fake %d times, want 1 (non-coalesced aggregate method must pass straight through)", got)
	}
}

func TestNewCoalescingS3_InterfaceTransparency_NonAggregateNarrowInterfaces(t *testing.T) {
	fake := &coalesceS3Fake{}
	decorated := awsclient.NewCoalescingS3(fake)

	// Narrow to S3API first — exactly ServiceClients.S3's static field
	// type — then re-assert to the narrow interfaces the way
	// enrichS3/s3_related.go actually do at their call sites. S3API itself
	// does not embed GetBucketPolicy/GetBucketCors/GetBucketLifecycleConfiguration
	// (s3_interfaces.go); only S3FullAPI's widening keeps these assertions
	// alive through the decorator.
	var s3API awsclient.S3API = decorated

	policyAPI, ok := s3API.(awsclient.S3GetBucketPolicyAPI)
	if !ok {
		t.Fatal("NewCoalescingS3's result, narrowed to S3API, does not satisfy S3GetBucketPolicyAPI — the s3→role related checker's type-assertion would break")
	}
	if _, err := policyAPI.GetBucketPolicy(context.Background(), &s3.GetBucketPolicyInput{Bucket: aws.String("acme-app-logs-prod")}); err != nil {
		t.Fatalf("GetBucketPolicy via S3GetBucketPolicyAPI: unexpected error: %v", err)
	}

	corsAPI, ok := s3API.(awsclient.S3GetBucketCorsAPI)
	if !ok {
		t.Fatal("NewCoalescingS3's result, narrowed to S3API, does not satisfy S3GetBucketCorsAPI — enrichS3's type-assertion would break")
	}
	if _, err := corsAPI.GetBucketCors(context.Background(), &s3.GetBucketCorsInput{}); err != nil {
		t.Fatalf("GetBucketCors: unexpected error: %v", err)
	}
	if got := fake.getCorsCalls.Load(); got != 1 {
		t.Errorf("GetBucketCors reached the inner fake %d times, want 1", got)
	}

	lifecycleAPI, ok := s3API.(awsclient.S3GetBucketLifecycleAPI)
	if !ok {
		t.Fatal("NewCoalescingS3's result, narrowed to S3API, does not satisfy S3GetBucketLifecycleAPI — enrichS3's type-assertion would break")
	}
	if _, err := lifecycleAPI.GetBucketLifecycleConfiguration(context.Background(), &s3.GetBucketLifecycleConfigurationInput{}); err != nil {
		t.Fatalf("GetBucketLifecycleConfiguration: unexpected error: %v", err)
	}
	if got := fake.getLifecycleCalls.Load(); got != 1 {
		t.Errorf("GetBucketLifecycleConfiguration reached the inner fake %d times, want 1", got)
	}
}

// TestWithDetailOp_SameOperation_ConcurrentIdenticalCalls_ShareOneUnderlyingCall
// pins the coalescing namespace WithDetailOp/DetailOpFromContext establish
// (coalesce.go's coalesceKey): every AWS call made on behalf of the SAME
// core/runtime.DetailOperation shares one singleflight group per underlying
// key, exactly like the plain (no-op-context) axis above — WithDetailOp only
// adds a namespace prefix to the same key, it does not change the dedup
// mechanism itself.
func TestWithDetailOp_SameOperation_ConcurrentIdenticalCalls_ShareOneUnderlyingCall(t *testing.T) {
	const bucket = "acme-app-logs-prod"
	fake := &coalesceS3Fake{getPolicyBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingS3(fake)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(7))

	const n = 8
	results := make([]*s3.GetBucketPolicyOutput, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = decorated.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
		}(i)
	}
	time.Sleep(concurrentGateSleep)
	close(fake.getPolicyBlock)
	wg.Wait()

	if got := fake.getPolicyCalls.Load(); got != 1 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times across %d concurrent calls under the SAME operation, want 1", got, n)
	}
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("call %d: unexpected error: %v", i, errs[i])
		}
		if results[i] != results[0] {
			t.Errorf("call %d: result pointer %p != call 0's %p — every caller under the same operation must share the identical singleflight result", i, results[i], results[0])
		}
	}
}

// TestWithDetailOp_DifferentOperations_BothExecuteIndependently pins that two
// operations calling the identical underlying key never coalesce with each
// other — a fresh detail open or an explicit refresh mints a new
// DetailOperation.ID, and coalesceKey folds that ID into the singleflight key,
// so the two operations occupy disjoint namespaces even though the AWS-level
// key (Bucket) is identical.
func TestWithDetailOp_DifferentOperations_BothExecuteIndependently(t *testing.T) {
	const bucket = "acme-app-logs-prod"
	fake := &coalesceS3Fake{getPolicyBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingS3(fake)
	ctxOp1 := awsclient.WithDetailOp(context.Background(), domain.Gen(1))
	ctxOp2 := awsclient.WithDetailOp(context.Background(), domain.Gen(2))

	var errs [2]error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errs[0] = decorated.GetBucketPolicy(ctxOp1, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = decorated.GetBucketPolicy(ctxOp2, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	}()
	time.Sleep(concurrentGateSleep)
	close(fake.getPolicyBlock)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if got := fake.getPolicyCalls.Load(); got != 2 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times for two DIFFERENT operations calling the same key, want 2 (operations must not coalesce with each other)", got)
	}
}

// TestWithDetailOp_NewOperationNeverJoinsOlderOperationsInFlightCall pins the
// structural guarantee coalesce.go's doc comment describes: a refresh begins
// a brand-new operation — a new ID, a new namespace — so it is structurally
// unable to join whatever pre-refresh call is still in flight under the old
// ID. No Forget call is involved (unlike the deleted bypass mechanism this
// test replaces): the namespaces simply never collide, so the new
// operation's call executes immediately rather than waiting for the older,
// still-blocked one to complete.
func TestWithDetailOp_NewOperationNeverJoinsOlderOperationsInFlightCall(t *testing.T) {
	const bucket = "acme-app-logs-prod"
	flightAEntered := make(chan struct{})
	flightAGate := make(chan struct{})
	var seq atomic.Int64

	fake := &coalesceS3Fake{
		getPolicyFn: func(in *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
			n := seq.Add(1)
			if n == 1 {
				// Flight A (operation 1): the first call in, blocks until the
				// test releases it — operation 2's call below must not wait.
				close(flightAEntered)
				<-flightAGate
			}
			return &s3.GetBucketPolicyOutput{Policy: aws.String(fmt.Sprintf("%s#%d", aws.ToString(in.Bucket), n))}, nil
		},
	}
	decorated := awsclient.NewCoalescingS3(fake)

	var flightAResult *s3.GetBucketPolicyOutput
	var flightAErr error
	var wgA sync.WaitGroup
	wgA.Add(1)
	go func() {
		defer wgA.Done()
		ctxOp1 := awsclient.WithDetailOp(context.Background(), domain.Gen(1))
		flightAResult, flightAErr = decorated.GetBucketPolicy(ctxOp1, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	}()
	<-flightAEntered

	// Operation 2's call: same underlying key, still-blocked flight A must
	// not delay it. Bounded-wait so an accidental cross-operation join
	// manifests as a clean failure, not a hang.
	op2Done := make(chan struct{})
	var op2Result *s3.GetBucketPolicyOutput
	var op2Err error
	go func() {
		defer close(op2Done)
		ctxOp2 := awsclient.WithDetailOp(context.Background(), domain.Gen(2))
		op2Result, op2Err = decorated.GetBucketPolicy(ctxOp2, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	}()
	select {
	case <-op2Done:
	case <-time.After(500 * time.Millisecond):
		close(flightAGate) // release flight A so the leaked goroutine above doesn't outlive the test
		wgA.Wait()
		t.Fatal("operation 2's call appears to have joined operation 1's still-blocked in-flight call instead of starting its own — a later operation must never join an earlier one's namespace")
	}
	if op2Err != nil {
		t.Fatalf("operation 2 call error: %v", op2Err)
	}

	close(flightAGate)
	wgA.Wait()
	if flightAErr != nil {
		t.Fatalf("flight A (operation 1) error: %v", flightAErr)
	}

	if got := fake.getPolicyCalls.Load(); got != 2 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times for operation 1 + operation 2, want 2", got)
	}
	if flightAResult == nil || op2Result == nil || *flightAResult.Policy == *op2Result.Policy {
		t.Errorf("operation 1's result %v and operation 2's result %v must differ — each operation must get its own fresh call", flightAResult, op2Result)
	}
}

// TestWithDetailOp_NoOpContext_CoalescesAsSharedDefaultNamespace pins
// DetailOpFromContext's zero-value fallback: a context that never passed
// through WithDetailOp (e.g. a call site not yet wired into the
// DetailOperation lifecycle) resolves to operation ID 0, and every such
// caller shares that SAME default namespace — they coalesce with each other
// exactly as same-operation callers do, never as if each had its own
// unnamespaced identity.
func TestWithDetailOp_NoOpContext_CoalescesAsSharedDefaultNamespace(t *testing.T) {
	if awsclient.DetailOpFromContext(context.Background()) != domain.Gen(0) {
		t.Fatalf("DetailOpFromContext(context.Background()) = %d, want 0", awsclient.DetailOpFromContext(context.Background()))
	}

	const bucket = "acme-app-logs-prod"
	fake := &coalesceS3Fake{getPolicyBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingS3(fake)

	const n = 4
	results := make([]*s3.GetBucketPolicyOutput, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			// No WithDetailOp — a bare context, exactly like a call site that
			// has not (yet) been wired into the DetailOperation lifecycle.
			results[i], errs[i] = decorated.GetBucketPolicy(context.Background(), &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
		}(i)
	}
	time.Sleep(concurrentGateSleep)
	close(fake.getPolicyBlock)
	wg.Wait()

	if got := fake.getPolicyCalls.Load(); got != 1 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times across %d concurrent no-op-context calls, want 1 (they must share the default namespace)", got, n)
	}
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("call %d: unexpected error: %v", i, errs[i])
		}
		if results[i] != results[0] {
			t.Errorf("call %d: result pointer %p != call 0's %p — no-op-context callers must share the identical singleflight result", i, results[i], results[0])
		}
	}
}

// ---------------------------------------------------------------------------
// Lambda: NewCoalescingLambda (#261 boundary-sealing wave, item d) —
// coalesceLambdaFake implements awsclient.LambdaAPI. LambdaAPI is already
// the complete aggregate of every Lambda operation asserted anywhere in
// core/aws (coalesce.go's own doc comment), so no wider FullAPI type exists
// for Lambda, unlike SNS/S3.
// ---------------------------------------------------------------------------

type coalesceLambdaFake struct {
	getFunctionCalls atomic.Int64
	getFunctionBlock chan struct{} // non-nil: GetFunction blocks here before returning
	listFuncCalls    atomic.Int64
}

func (f *coalesceLambdaFake) GetFunction(_ context.Context, in *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	f.getFunctionCalls.Add(1)
	if f.getFunctionBlock != nil {
		<-f.getFunctionBlock
	}
	return &lambda.GetFunctionOutput{
		Configuration: &lambdatypes.FunctionConfiguration{
			// FunctionName echoes the input key back so a test can verify a
			// shared result carries the right per-key payload without adding
			// a dedicated field.
			FunctionName: in.FunctionName,
		},
		// Code.ImageUri lets checkLambdaECR (core/aws/lambda_related.go)
		// resolve a repo name end to end, for the integration test below
		// that exercises the real checker through this fake.
		Code: &lambdatypes.FunctionCodeLocation{
			ImageUri: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app"),
		},
	}, nil
}

func (f *coalesceLambdaFake) ListFunctions(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	f.listFuncCalls.Add(1)
	return &lambda.ListFunctionsOutput{}, nil
}
func (f *coalesceLambdaFake) ListEventSourceMappings(_ context.Context, _ *lambda.ListEventSourceMappingsInput, _ ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error) {
	return &lambda.ListEventSourceMappingsOutput{}, nil
}
func (f *coalesceLambdaFake) ListTags(_ context.Context, _ *lambda.ListTagsInput, _ ...func(*lambda.Options)) (*lambda.ListTagsOutput, error) {
	return &lambda.ListTagsOutput{}, nil
}

var _ awsclient.LambdaAPI = (*coalesceLambdaFake)(nil)

func TestNewCoalescingLambda_ConcurrentIdenticalCalls_ShareOneUnderlyingCallAndResult(t *testing.T) {
	const fnName = "process-payment"
	fake := &coalesceLambdaFake{getFunctionBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingLambda(fake)

	const n = 8
	results := make([]*lambda.GetFunctionOutput, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = decorated.GetFunction(context.Background(), &lambda.GetFunctionInput{FunctionName: aws.String(fnName)})
		}(i)
	}
	time.Sleep(concurrentGateSleep)
	close(fake.getFunctionBlock)
	wg.Wait()

	if got := fake.getFunctionCalls.Load(); got != 1 {
		t.Errorf("GetFunction reached the inner fake %d times across %d concurrent identical calls, want 1", got, n)
	}
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("call %d: unexpected error: %v", i, errs[i])
		}
		if results[i] == nil || results[i].Configuration == nil || results[i].Configuration.FunctionName == nil || *results[i].Configuration.FunctionName != fnName {
			t.Errorf("call %d: result = %+v, want Configuration.FunctionName echoing %q", i, results[i], fnName)
		}
		if results[i] != results[0] {
			t.Errorf("call %d: result pointer %p != call 0's %p — all concurrent callers must share the identical singleflight result", i, results[i], results[0])
		}
	}
}

func TestNewCoalescingLambda_SequentialCalls_BothReexecute(t *testing.T) {
	const fnName = "process-payment"
	fake := &coalesceLambdaFake{}
	decorated := awsclient.NewCoalescingLambda(fake)

	if _, err := decorated.GetFunction(context.Background(), &lambda.GetFunctionInput{FunctionName: aws.String(fnName)}); err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if _, err := decorated.GetFunction(context.Background(), &lambda.GetFunctionInput{FunctionName: aws.String(fnName)}); err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if got := fake.getFunctionCalls.Load(); got != 2 {
		t.Errorf("GetFunction reached the inner fake %d times across two sequential identical calls under opID 0, want 2 (no caching, only in-flight dedup)", got)
	}
}

// TestNewCoalescingLambda_SequentialCalls_SameNonZeroOp_MemoizesToOneUnderlyingCall
// mirrors the SFN/SNS/S3 sequential-memoization pins for Lambda: two
// sequential calls under the same non-zero op must reach the inner fake
// exactly once.
func TestNewCoalescingLambda_SequentialCalls_SameNonZeroOp_MemoizesToOneUnderlyingCall(t *testing.T) {
	const fnName = "process-payment"
	fake := &coalesceLambdaFake{}
	decorated := awsclient.NewCoalescingLambda(fake)
	ctx := awsclient.WithDetailOp(context.Background(), domain.Gen(5))

	first, err := decorated.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(fnName)})
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	second, err := decorated.GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(fnName)})
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if got := fake.getFunctionCalls.Load(); got != 1 {
		t.Errorf("GetFunction reached the inner fake %d times across two SEQUENTIAL calls under the same non-zero op, want 1 (completed result must be memoized per (op, key))", got)
	}
	if first == nil || second == nil || first.Configuration.FunctionName == nil || second.Configuration.FunctionName == nil || *first.Configuration.FunctionName != *second.Configuration.FunctionName {
		t.Errorf("first result %+v and second (memoized) result %+v must carry the same FunctionName", first, second)
	}
}

func TestNewCoalescingLambda_ConcurrentDifferentKeys_BothExecuteIndependently(t *testing.T) {
	fake := &coalesceLambdaFake{getFunctionBlock: make(chan struct{})}
	decorated := awsclient.NewCoalescingLambda(fake)

	names := [2]string{"process-payment", "process-refund"}
	var errs [2]error
	var wg sync.WaitGroup
	wg.Add(2)
	for i := range 2 {
		go func(i int) {
			defer wg.Done()
			_, errs[i] = decorated.GetFunction(context.Background(), &lambda.GetFunctionInput{FunctionName: aws.String(names[i])})
		}(i)
	}
	time.Sleep(concurrentGateSleep)
	close(fake.getFunctionBlock)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}
	if got := fake.getFunctionCalls.Load(); got != 2 {
		t.Errorf("GetFunction reached the inner fake %d times for two concurrent DIFFERENT-key calls, want 2 (different keys must not coalesce)", got)
	}
}

func TestNewCoalescingLambda_PassThrough_ListFunctionsReachesInnerFake(t *testing.T) {
	fake := &coalesceLambdaFake{}
	decorated := awsclient.NewCoalescingLambda(fake)

	if _, err := decorated.ListFunctions(context.Background(), &lambda.ListFunctionsInput{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := fake.listFuncCalls.Load(); got != 1 {
		t.Errorf("ListFunctions reached the inner fake %d times, want 1 (non-coalesced method must pass straight through)", got)
	}
}

// TestNewCoalescingLambda_SatisfiesEveryNarrowLambdaInterface is a
// compile-level assertion (#261 boundary-sealing wave, item d): the
// decorator NewCoalescingLambda returns must still satisfy every narrow
// Lambda interface asserted anywhere in core/aws (apigw_related.go,
// related_common.go, secrets_related_extra.go, lambda_related.go,
// lambda_detail_enrichment.go each narrow ServiceClients.Lambda to one of
// these) — a future decorator field rename or method-set change that
// silently dropped one would break those call sites' type assertions rather
// than fail to compile here.
func TestNewCoalescingLambda_SatisfiesEveryNarrowLambdaInterface(t *testing.T) {
	decorated := awsclient.NewCoalescingLambda(&coalesceLambdaFake{})

	var _ awsclient.LambdaAPI = decorated
	if _, ok := decorated.(awsclient.LambdaGetFunctionAPI); !ok {
		t.Error("NewCoalescingLambda's result does not satisfy LambdaGetFunctionAPI")
	}
	if _, ok := decorated.(awsclient.LambdaListFunctionsAPI); !ok {
		t.Error("NewCoalescingLambda's result does not satisfy LambdaListFunctionsAPI")
	}
	if _, ok := decorated.(awsclient.LambdaListEventSourceMappingsAPI); !ok {
		t.Error("NewCoalescingLambda's result does not satisfy LambdaListEventSourceMappingsAPI")
	}
	if _, ok := decorated.(awsclient.LambdaListTagsAPI); !ok {
		t.Error("NewCoalescingLambda's result does not satisfy LambdaListTagsAPI")
	}
}

// coalesceLambdaECRDefByTarget returns the "lambda" RelatedDef targeting
// "ecr" (checkLambdaECR, core/aws/lambda_related.go) — the same lookup
// aws_lambda_related_test.go's lambdaCheckerByTarget performs, duplicated
// here rather than shared across the package boundary (that helper lives in
// package unit_test; this file is package unit).
func coalesceLambdaECRDefByTarget(t *testing.T) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("lambda") {
		if def.TargetType == "ecr" {
			if def.Checker == nil {
				t.Fatal("lambda related checker for ecr is nil")
			}
			return def.Checker
		}
	}
	t.Fatal("lambda has no registered related def targeting ecr")
	return nil
}

// TestLambdaGetFunction_ECRCheckerAndEnricher_ShareOneUnderlyingCallPerOperation
// pins item (d)'s integration scenario end to end through the REAL exported
// consumers: opening an Image-package-type Lambda's detail dispatches BOTH
// the ecr related checker (checkLambdaECR) and the detail enricher
// (enrichLambda), and both call GetFunction for the identical function.
// Driven under one shared coalescingLambda-decorated fake and one shared
// non-zero WithDetailOp id — exactly how core/runtime.DetailOperation wires
// every task an operation spawns — the inner fake must be reached exactly
// once.
func TestLambdaGetFunction_ECRCheckerAndEnricher_ShareOneUnderlyingCallPerOperation(t *testing.T) {
	const fnName = "image-fn"
	const op = domain.Gen(9)
	fake := &coalesceLambdaFake{}
	decorated := awsclient.NewCoalescingLambda(fake)
	sc := &awsclient.ServiceClients{Lambda: decorated}

	ctx := awsclient.WithDetailOp(context.Background(), op)
	res := resource.Resource{
		ID:        fnName,
		Fields:    map[string]string{"package_type": "Image"},
		RawStruct: lambdatypes.FunctionConfiguration{FunctionName: aws.String(fnName)},
	}

	ecrChecker := coalesceLambdaECRDefByTarget(t)
	ecrResult := ecrChecker(ctx, sc, res, nil)
	if ecrResult.State == domain.RelatedUnknown {
		t.Fatalf("ecr related check returned Unknown, want a resolved result driving a real GetFunction call: %+v", ecrResult)
	}

	enricher := resource.GetDetailEnricher("lambda")
	if enricher == nil {
		t.Fatal("lambda detail enricher not registered")
	}
	// OpID must match the operation ctx already carries (enrichDetail's
	// engine re-wraps ctx via WithDetailOp(ctx, dctx.OpID) rather than
	// trusting the incoming ctx's own marker — core/aws/detail_enrich_engine.go)
	// so the enricher's GetFunction call lands in the SAME coalescing
	// namespace as the checker's call above.
	dctx := &awsclient.DetailEnrichmentCtx{Clients: sc, OpID: op}
	if _, err := enricher(ctx, dctx, res); err != nil {
		t.Fatalf("enrichLambda: unexpected error: %v", err)
	}

	if got := fake.getFunctionCalls.Load(); got != 1 {
		t.Errorf("GetFunction reached the inner fake %d times across the ecr related checker + the detail enricher under ONE operation, want 1", got)
	}
}
