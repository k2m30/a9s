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
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
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

// TestNewCoalescingS3_Bypass_ForgetGivesFreshResultAndPreventsLaterJoin pins
// F3: enrichDetail (core/aws/detail_enrich_engine.go) marks ctx with an
// unexported coalesce-bypass marker when DetailEnrichmentCtx.SkipCache is
// set (an explicit detail refresh); on that marker the coalescing
// decorator's gated method forgets any in-flight entry for the key
// (singleflight.Group.Forget) and calls the inner client directly instead
// of joining. The marker itself has no exported constructor (coalesce.go's
// withCoalesceBypass/coalesceBypassed are package-private), so the only way
// to exercise it from this external package is the same way production
// does: through enrichS3 with SkipCache: true, sharing one coalescing
// client instance with a plain concurrent caller — exactly how
// checkS3Role and enrichS3 share one ServiceClients.S3 in production
// (coalesce.go's doc comment).
func TestNewCoalescingS3_Bypass_ForgetGivesFreshResultAndPreventsLaterJoin(t *testing.T) {
	const bucket = "acme-app-logs-prod"
	flightAEntered := make(chan struct{})
	flightAGate := make(chan struct{})
	var seq atomic.Int64

	fake := &coalesceS3Fake{
		getPolicyFn: func(in *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
			n := seq.Add(1)
			if n == 1 {
				// Flight A: the first call in, blocks until the test
				// releases it — everything else (bypass, and a later
				// plain caller) must not wait on this.
				close(flightAEntered)
				<-flightAGate
			}
			return &s3.GetBucketPolicyOutput{Policy: aws.String(fmt.Sprintf("%s#%d", aws.ToString(in.Bucket), n))}, nil
		},
	}
	coalesced := awsclient.NewCoalescingS3(fake)

	// Flight A: a plain call directly on the decorator.
	var flightAResult *s3.GetBucketPolicyOutput
	var flightAErr error
	var wgA sync.WaitGroup
	wgA.Add(1)
	go func() {
		defer wgA.Done()
		flightAResult, flightAErr = coalesced.GetBucketPolicy(context.Background(), &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	}()
	<-flightAEntered

	// The bypass call: drive it through the real enrichS3 pipeline with
	// SkipCache: true, sharing the SAME coalescing client instance as
	// flight A.
	enricher := s3Enricher(t)
	dctx := &awsclient.DetailEnrichmentCtx{
		Clients:    &awsclient.ServiceClients{S3: coalesced},
		PolicyDocs: &awsclient.PolicyDocumentCache{},
		DetailDocs: &awsclient.DetailDocCache{},
		SkipCache:  true,
	}
	bypassGot, err := enricher(context.Background(), dctx, makeS3Res(bucket))
	if err != nil {
		t.Fatalf("bypass (SkipCache) enrichS3 call error: %v", err)
	}
	if got := fake.getPolicyCalls.Load(); got != 2 {
		t.Fatalf("GetBucketPolicy reached the inner fake %d times for flight A + the bypass call, want 2 (bypass must not wait for flight A)", got)
	}
	bypassEnriched, ok := bypassGot.RawStruct.(awsclient.BucketEnriched)
	if !ok {
		t.Fatalf("bypass RawStruct = %T, want BucketEnriched", bypassGot.RawStruct)
	}
	bypassPolicy, ok := bypassEnriched.Policy.(string)
	if !ok {
		t.Fatalf("bypass enriched.Policy = %T (%v), want string", bypassEnriched.Policy, bypassEnriched.Policy)
	}

	// A caller issued right after the bypass (which must have already
	// Forgotten the key) must not join flight A, which is STILL blocked at
	// this point — it must start its own call. Bounded-wait so a wrongly
	// preserved join manifests as a clean failure, not a hang.
	thirdDone := make(chan struct{})
	var thirdResult *s3.GetBucketPolicyOutput
	var thirdErr error
	go func() {
		defer close(thirdDone)
		thirdResult, thirdErr = coalesced.GetBucketPolicy(context.Background(), &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	}()
	select {
	case <-thirdDone:
	case <-time.After(500 * time.Millisecond):
		close(flightAGate) // release flight A so the leaked goroutines above don't outlive the test
		wgA.Wait()
		t.Fatal("a caller issued right after the bypass appears to have joined flight A's still-blocked in-flight call instead of starting its own — Forget must evict the key immediately, not defer to flight A's completion")
	}
	if thirdErr != nil {
		t.Fatalf("third call error: %v", thirdErr)
	}

	close(flightAGate)
	wgA.Wait()
	if flightAErr != nil {
		t.Fatalf("flight A error: %v", flightAErr)
	}

	if got := fake.getPolicyCalls.Load(); got != 3 {
		t.Errorf("GetBucketPolicy reached the inner fake %d times for flight A + bypass + the post-Forget caller, want 3", got)
	}
	if flightAResult == nil || *flightAResult.Policy == bypassPolicy {
		t.Errorf("flight A's result %v must differ from the bypass result %q — the bypass caller must get its own fresh call, not flight A's shared one", flightAResult, bypassPolicy)
	}
	if thirdResult == nil || *thirdResult.Policy == bypassPolicy {
		t.Errorf("the post-Forget caller's result %v must differ from the bypass result %q — it must be its own fresh call, not a join of a forgotten entry", thirdResult, bypassPolicy)
	}
}

// TestCoalesceBypass_ExportedPair pins the exported WithCoalesceBypass/
// CoalesceBypassed pair (renames of the former unexported
// withCoalesceBypass/coalesceBypassed) — runtime.RunRelatedDef (the new
// single-sourced related-check executor shared by both the TUI and neutral
// lanes) constructs the bypass marker directly via this pair instead of
// going through enrichDetail/SkipCache, so the pair itself must now be
// externally constructible and observable. Unlike
// TestNewCoalescingS3_Bypass_ForgetGivesFreshResultAndPreventsLaterJoin
// above (which had to route through enrichS3 because the marker had no
// exported constructor), this test builds the bypassed ctx directly.
func TestCoalesceBypass_ExportedPair(t *testing.T) {
	if awsclient.CoalesceBypassed(context.Background()) {
		t.Error("CoalesceBypassed(context.Background()) = true, want false for a bare context")
	}
	bypassed := awsclient.WithCoalesceBypass(context.Background())
	if !awsclient.CoalesceBypassed(bypassed) {
		t.Error("CoalesceBypassed(WithCoalesceBypass(ctx)) = false, want true")
	}

	const bucket = "acme-app-logs-prod"
	flightAEntered := make(chan struct{})
	flightAGate := make(chan struct{})
	var seq atomic.Int64

	fake := &coalesceS3Fake{
		getPolicyFn: func(in *s3.GetBucketPolicyInput) (*s3.GetBucketPolicyOutput, error) {
			n := seq.Add(1)
			if n == 1 {
				// Flight A: the first call in, blocks until the test releases
				// it — the bypassed call below must not wait on this.
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
		flightAResult, flightAErr = decorated.GetBucketPolicy(context.Background(), &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	}()
	<-flightAEntered

	// The bypassed call: same key, ctx built directly via the exported pair —
	// must not join flight A's still-blocked in-flight call.
	bypassCtx := awsclient.WithCoalesceBypass(context.Background())
	bypassResult, err := decorated.GetBucketPolicy(bypassCtx, &s3.GetBucketPolicyInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("bypass call error: %v", err)
	}
	if got := fake.getPolicyCalls.Load(); got != 2 {
		t.Fatalf("GetBucketPolicy reached the inner fake %d times for flight A + the bypass call, want 2 (bypass must not wait for flight A)", got)
	}

	close(flightAGate)
	wgA.Wait()
	if flightAErr != nil {
		t.Fatalf("flight A error: %v", flightAErr)
	}
	if flightAResult == nil || bypassResult == nil || *flightAResult.Policy == *bypassResult.Policy {
		t.Errorf("flight A's result %v and the bypass result %v must differ — the bypass call must not join flight A's shared in-flight call", flightAResult, bypassResult)
	}
}
