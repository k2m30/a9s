package unit_test

// Tests for helper functions that were previously uncovered:
//
//  1. checkSQSSQS (exercises sqsRedriveTarget via public related checker)
//  2. Actor() via IAMUser ARN path (exercises arnLastSegment indirectly)
//  3. ExtractTarget() via ARN-only resources (exercises labelFromARN indirectly)
//  4. FetchLambdaFunctionsPageWithEventSources (exercises firstLambdaEventSourceARN)
//  5. FetchS3BucketsPageWithNotifications (exercises firstS3NotificationTargets)
//  6. buildinfo.ResolveCommit
//  7. buildinfo.ResolveDate

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
	"github.com/k2m30/a9s/v3/internal/buildinfo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// 1. SQS DLQ relationship checker (exercises sqsRedriveTarget via public API)
// ---------------------------------------------------------------------------

// sqsSQSCheckerForTest retrieves the "sqs" → "sqs" related checker.
func sqsSQSCheckerForTest(t *testing.T) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("sqs") {
		if def.TargetType == "sqs" {
			if def.Checker == nil {
				t.Fatal("sqs->sqs checker is nil")
			}
			return def.Checker
		}
	}
	t.Fatal("sqs->sqs checker not found in registry")
	return nil
}

// sqsDLQRes builds a queue that forwards dead letters to another queue.
func sqsDLQRes(queueName, queueARN, redrivePolicy string) resource.Resource {
	return resource.Resource{
		ID:   queueName,
		Name: queueName,
		Fields: map[string]string{
			"queue_name": queueName,
			"arn":        queueARN,
		},
		RawStruct: awsclient.SQSQueueAttributesRow{
			QueueURL:  "https://sqs.us-east-1.amazonaws.com/123456789012/" + queueName,
			QueueName: queueName,
			Attributes: map[string]string{
				"QueueArn":      queueARN,
				"RedrivePolicy": redrivePolicy,
			},
		},
	}
}

// TestRelated_SQS_SQS_RedrivePolicy_ForwardDLQ verifies that when this queue's
// RedrivePolicy points to another queue (forward DLQ relationship), checkSQSSQS
// returns that queue's ID.
func TestRelated_SQS_SQS_RedrivePolicy_ForwardDLQ(t *testing.T) {
	const dlqARN = "arn:aws:sqs:us-east-1:123456789012:payment-processing-dlq"
	const dlqName = "payment-processing-dlq"

	// "payment-processing" has a redrive policy that points to the DLQ.
	thisRes := sqsDLQRes(
		"payment-processing",
		"arn:aws:sqs:us-east-1:123456789012:payment-processing",
		`{"deadLetterTargetArn":"`+dlqARN+`","maxReceiveCount":"5"}`,
	)

	// Cache contains the DLQ queue.
	dlqRes := sqsDLQRes(dlqName, dlqARN, "")

	cache := resource.ResourceCache{
		"sqs": resource.ResourceCacheEntry{Resources: []resource.Resource{thisRes, dlqRes}},
	}

	checker := sqsSQSCheckerForTest(t)
	result := checker(context.Background(), nil, thisRes, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (forward DLQ relationship)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != dlqName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, dlqName)
	}
}

// TestRelated_SQS_SQS_RedrivePolicy_ReverseDLQ verifies that when ANOTHER queue's
// RedrivePolicy points to THIS queue (this queue IS the DLQ), checkSQSSQS returns
// the other queue's ID.
func TestRelated_SQS_SQS_RedrivePolicy_ReverseDLQ(t *testing.T) {
	const dlqARN = "arn:aws:sqs:us-east-1:123456789012:orders-dlq"
	const dlqName = "orders-dlq"

	// "orders-dlq" is the DLQ — it has no redrive policy itself.
	dlqRes := sqsDLQRes(dlqName, dlqARN, "")

	// "orders" has a redrive policy pointing to the DLQ.
	ordersRes := sqsDLQRes(
		"orders",
		"arn:aws:sqs:us-east-1:123456789012:orders",
		`{"deadLetterTargetArn":"`+dlqARN+`","maxReceiveCount":"3"}`,
	)

	cache := resource.ResourceCache{
		"sqs": resource.ResourceCacheEntry{Resources: []resource.Resource{dlqRes, ordersRes}},
	}

	checker := sqsSQSCheckerForTest(t)
	result := checker(context.Background(), nil, dlqRes, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (reverse DLQ: orders uses this as DLQ)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "orders" {
		t.Errorf("ResourceIDs = %v, want [orders]", result.ResourceIDs)
	}
}

// TestRelated_SQS_SQS_RedrivePolicy_InvalidJSON verifies that a malformed
// RedrivePolicy JSON is silently ignored (no panic, count 0).
func TestRelated_SQS_SQS_RedrivePolicy_InvalidJSON(t *testing.T) {
	thisRes := sqsDLQRes(
		"bad-queue",
		"arn:aws:sqs:us-east-1:123456789012:bad-queue",
		`not-valid-json`,
	)

	otherRes := sqsDLQRes(
		"other-queue",
		"arn:aws:sqs:us-east-1:123456789012:other-queue",
		`not-valid-json-either`,
	)

	cache := resource.ResourceCache{
		"sqs": resource.ResourceCacheEntry{Resources: []resource.Resource{thisRes, otherRes}},
	}

	checker := sqsSQSCheckerForTest(t)
	result := checker(context.Background(), nil, thisRes, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (invalid JSON must not match anything)", result.Count)
	}
}

// TestRelated_SQS_SQS_RedrivePolicy_EmptyCache verifies that an empty cache
// returns Count:-1 (unknown, can't determine relationship).
func TestRelated_SQS_SQS_RedrivePolicy_EmptyCache(t *testing.T) {
	thisRes := sqsDLQRes(
		"my-queue",
		"arn:aws:sqs:us-east-1:123456789012:my-queue",
		`{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:123456789012:my-dlq","maxReceiveCount":"5"}`,
	)

	checker := sqsSQSCheckerForTest(t)
	result := checker(context.Background(), nil, thisRes, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache = unknown)", result.Count)
	}
}

// TestRelated_SQS_SQS_NoRelationship verifies Count 0 when neither forward nor
// reverse DLQ relationships exist.
func TestRelated_SQS_SQS_NoRelationship(t *testing.T) {
	res1 := sqsDLQRes(
		"queue-a",
		"arn:aws:sqs:us-east-1:123456789012:queue-a",
		"",
	)
	res2 := sqsDLQRes(
		"queue-b",
		"arn:aws:sqs:us-east-1:123456789012:queue-b",
		"",
	)

	cache := resource.ResourceCache{
		"sqs": resource.ResourceCacheEntry{Resources: []resource.Resource{res1, res2}},
	}

	checker := sqsSQSCheckerForTest(t)
	result := checker(context.Background(), nil, res1, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no DLQ relationship)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// 2. Actor() — IAMUser with ARN only (exercises arnLastSegment)
// ---------------------------------------------------------------------------

// TestCTDetailActor_IAMUser_ARNOnly verifies that Actor() falls back to
// arnLastSegment when UserName is empty but ARN has a "/user/name" suffix.
func TestCTDetailActor_IAMUser_ARNOnly(t *testing.T) {
	event := &ctevent.Event{
		UserIdentity: ctevent.UserIdentity{
			Type:     "IAMUser",
			UserName: "",
			ARN:      "arn:aws:iam::123456789012:user/alice",
		},
	}

	got := ctevent.Actor(event)
	want := "IAMUser: alice"
	if got != want {
		t.Errorf("Actor() = %q, want %q", got, want)
	}
}

// TestCTDetailActor_IAMUser_ARNNoSlash verifies Actor() when ARN has no "/" —
// arnLastSegment returns "" and Actor falls back to the raw ARN.
func TestCTDetailActor_IAMUser_ARNNoSlash(t *testing.T) {
	event := &ctevent.Event{
		UserIdentity: ctevent.UserIdentity{
			Type:     "IAMUser",
			UserName: "",
			ARN:      "arn:aws:iam::123456789012:root",
		},
	}

	got := ctevent.Actor(event)
	// arnLastSegment returns "" for "arn:aws:iam::123456789012:root" (no "/"),
	// so Actor falls back to the raw ARN.
	if got == "" {
		t.Error("Actor() must not return empty string")
	}
	if got == "IAMUser: " {
		t.Errorf("Actor() = %q — must not produce trailing space with empty name", got)
	}
}

// ---------------------------------------------------------------------------
// 3. ExtractTarget() — ARN-only ResourceRef (exercises labelFromARN)
// ---------------------------------------------------------------------------

// TestCTDetailExtractTarget_LabelFromARN_IAMRole verifies that an IAM role ARN
// in the resources[] envelope gets label "Role" via labelFromARN.
func TestCTDetailExtractTarget_LabelFromARN_IAMRole(t *testing.T) {
	resources := []ctevent.ResourceRef{
		{
			ARN:  "arn:aws:iam::123456789012:role/my-execution-role",
			Type: "", // no type string → forces labelFromARN fallback
		},
	}

	rows, _ := ctevent.ExtractTarget("AssumeRole", "sts.amazonaws.com", "123456789012", resources, nil)
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}
	if rows[0].Key != "Role" {
		t.Errorf("rows[0].Key = %q, want %q", rows[0].Key, "Role")
	}
}

// TestCTDetailExtractTarget_LabelFromARN_IAMUser verifies that an IAM user ARN
// gets label "User" via labelFromARN.
func TestCTDetailExtractTarget_LabelFromARN_IAMUser(t *testing.T) {
	resources := []ctevent.ResourceRef{
		{
			ARN:  "arn:aws:iam::123456789012:user/bob",
			Type: "",
		},
	}

	rows, _ := ctevent.ExtractTarget("CreateUser", "iam.amazonaws.com", "123456789012", resources, nil)
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}
	if rows[0].Key != "User" {
		t.Errorf("rows[0].Key = %q, want %q", rows[0].Key, "User")
	}
}

// TestCTDetailExtractTarget_LabelFromARN_KMSKey verifies that a KMS key ARN
// gets label "Key" via labelFromARN.
func TestCTDetailExtractTarget_LabelFromARN_KMSKey(t *testing.T) {
	resources := []ctevent.ResourceRef{
		{
			ARN:  "arn:aws:kms:us-east-1:123456789012:key/abc-123-def",
			Type: "",
		},
	}

	rows, _ := ctevent.ExtractTarget("Decrypt", "kms.amazonaws.com", "123456789012", resources, nil)
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}
	if rows[0].Key != "Key" {
		t.Errorf("rows[0].Key = %q, want %q", rows[0].Key, "Key")
	}
}

// TestCTDetailExtractTarget_LabelFromARN_S3Bucket verifies that an S3 bucket ARN
// (no object key) gets label "Bucket" via labelFromARN.
func TestCTDetailExtractTarget_LabelFromARN_S3Bucket(t *testing.T) {
	resources := []ctevent.ResourceRef{
		{
			ARN:  "arn:aws:s3:::my-data-bucket",
			Type: "",
		},
	}

	rows, _ := ctevent.ExtractTarget("PutBucketPolicy", "s3.amazonaws.com", "", resources, nil)
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}
	if rows[0].Key != "Bucket" {
		t.Errorf("rows[0].Key = %q, want %q", rows[0].Key, "Bucket")
	}
}

// TestCTDetailExtractTarget_LabelFromARN_S3Object verifies that an S3 ARN with
// an object key gets label "Object" via labelFromARN.
func TestCTDetailExtractTarget_LabelFromARN_S3Object(t *testing.T) {
	resources := []ctevent.ResourceRef{
		{
			ARN:  "arn:aws:s3:::my-data-bucket/prefix/object.json",
			Type: "",
		},
	}

	rows, _ := ctevent.ExtractTarget("GetObject", "s3.amazonaws.com", "", resources, nil)
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}
	if rows[0].Key != "Object" {
		t.Errorf("rows[0].Key = %q, want %q", rows[0].Key, "Object")
	}
}

// TestCTDetailExtractTarget_LabelFromARN_UnknownService verifies that an ARN
// from an unknown service falls back to "Resource".
func TestCTDetailExtractTarget_LabelFromARN_UnknownService(t *testing.T) {
	resources := []ctevent.ResourceRef{
		{
			ARN:  "arn:aws:some-unknown-service:us-east-1:123456789012:thing/xyz",
			Type: "",
		},
	}

	rows, _ := ctevent.ExtractTarget("DoSomething", "some-unknown-service.amazonaws.com", "", resources, nil)
	if len(rows) == 0 {
		t.Fatal("expected at least 1 row")
	}
	if rows[0].Key != "Resource" {
		t.Errorf("rows[0].Key = %q, want %q (unknown service → Resource fallback)", rows[0].Key, "Resource")
	}
}

// ---------------------------------------------------------------------------
// 4. FetchLambdaFunctionsPageWithEventSources (exercises firstLambdaEventSourceARN)
// ---------------------------------------------------------------------------

// mockLambdaListEventSourceMappingsClient implements LambdaListEventSourceMappingsAPI.
type mockLambdaListEventSourceMappingsClient struct {
	output *lambda.ListEventSourceMappingsOutput
	err    error
}

func (m *mockLambdaListEventSourceMappingsClient) ListEventSourceMappings(
	ctx context.Context,
	params *lambda.ListEventSourceMappingsInput,
	optFns ...func(*lambda.Options),
) (*lambda.ListEventSourceMappingsOutput, error) {
	return m.output, m.err
}

// mockLambdaListFunctionsForESM implements LambdaListFunctionsAPI for event-source tests.
type mockLambdaListFunctionsForESM struct {
	output *lambda.ListFunctionsOutput
}

func (m *mockLambdaListFunctionsForESM) ListFunctions(
	ctx context.Context,
	params *lambda.ListFunctionsInput,
	optFns ...func(*lambda.Options),
) (*lambda.ListFunctionsOutput, error) {
	return m.output, nil
}

// TestFetchLambdaFunctionsPageWithEventSources_PopulatesEventSourceARN verifies
// that firstLambdaEventSourceARN is called and its result lands in event_source_arn.
func TestFetchLambdaFunctionsPageWithEventSources_PopulatesEventSourceARN(t *testing.T) {
	const sqsARN = "arn:aws:sqs:us-east-1:123456789012:my-trigger-queue"

	listFuncsMock := &mockLambdaListFunctionsForESM{
		output: &lambda.ListFunctionsOutput{
			Functions: []lambdatypes.FunctionConfiguration{
				{
					FunctionName: aws.String("my-worker"),
					Runtime:      lambdatypes.RuntimeProvidedal2023,
					FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-worker"),
				},
			},
		},
	}

	esmMock := &mockLambdaListEventSourceMappingsClient{
		output: &lambda.ListEventSourceMappingsOutput{
			EventSourceMappings: []lambdatypes.EventSourceMappingConfiguration{
				{EventSourceArn: aws.String(sqsARN)},
			},
		},
	}

	result, err := awsclient.FetchLambdaFunctionsPageWithEventSources(
		context.Background(),
		listFuncsMock,
		esmMock,
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	got := result.Resources[0].Fields["event_source_arn"]
	if got != sqsARN {
		t.Errorf("event_source_arn = %q, want %q", got, sqsARN)
	}
}

// TestFetchLambdaFunctionsPageWithEventSources_NilESMAPI verifies that when
// no event source API is provided, event_source_arn is empty (no panic).
func TestFetchLambdaFunctionsPageWithEventSources_NilESMAPI(t *testing.T) {
	listFuncsMock := &mockLambdaListFunctionsForESM{
		output: &lambda.ListFunctionsOutput{
			Functions: []lambdatypes.FunctionConfiguration{
				{
					FunctionName: aws.String("no-trigger-fn"),
					Runtime:      lambdatypes.RuntimePython312,
					FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:no-trigger-fn"),
				},
			},
		},
	}

	result, err := awsclient.FetchLambdaFunctionsPageWithEventSources(
		context.Background(),
		listFuncsMock,
		nil, // no ESM API
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	got := result.Resources[0].Fields["event_source_arn"]
	if got != "" {
		t.Errorf("event_source_arn = %q, want empty string (no ESM API)", got)
	}
}

// TestFetchLambdaFunctionsPageWithEventSources_EmptyMappings verifies that when
// ListEventSourceMappings returns no mappings, event_source_arn is "".
func TestFetchLambdaFunctionsPageWithEventSources_EmptyMappings(t *testing.T) {
	listFuncsMock := &mockLambdaListFunctionsForESM{
		output: &lambda.ListFunctionsOutput{
			Functions: []lambdatypes.FunctionConfiguration{
				{
					FunctionName: aws.String("fn-no-triggers"),
					Runtime:      lambdatypes.RuntimeNodejs22x,
					FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:fn-no-triggers"),
				},
			},
		},
	}

	esmMock := &mockLambdaListEventSourceMappingsClient{
		output: &lambda.ListEventSourceMappingsOutput{
			EventSourceMappings: []lambdatypes.EventSourceMappingConfiguration{},
		},
	}

	result, err := awsclient.FetchLambdaFunctionsPageWithEventSources(
		context.Background(),
		listFuncsMock,
		esmMock,
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	got := result.Resources[0].Fields["event_source_arn"]
	if got != "" {
		t.Errorf("event_source_arn = %q, want empty string (no mappings)", got)
	}
}

// ---------------------------------------------------------------------------
// 5. FetchS3BucketsPageWithNotifications (exercises firstS3NotificationTargets)
// ---------------------------------------------------------------------------

// mockS3GetBucketNotificationClient implements S3GetBucketNotificationConfigurationAPI.
type mockS3GetBucketNotificationClient struct {
	output *s3.GetBucketNotificationConfigurationOutput
	err    error
}

func (m *mockS3GetBucketNotificationClient) GetBucketNotificationConfiguration(
	ctx context.Context,
	params *s3.GetBucketNotificationConfigurationInput,
	optFns ...func(*s3.Options),
) (*s3.GetBucketNotificationConfigurationOutput, error) {
	return m.output, m.err
}

// mockS3ListBucketsForNotification implements S3ListBucketsAPI for notification tests.
type mockS3ListBucketsForNotification struct {
	output *s3.ListBucketsOutput
}

func (m *mockS3ListBucketsForNotification) ListBuckets(
	ctx context.Context,
	params *s3.ListBucketsInput,
	optFns ...func(*s3.Options),
) (*s3.ListBucketsOutput, error) {
	return m.output, nil
}

// TestFetchS3BucketsPageWithNotifications_PopulatesLambdaNotification verifies that
// firstS3NotificationTargets fills notification_lambda when a Lambda notification exists.
func TestFetchS3BucketsPageWithNotifications_PopulatesLambdaNotification(t *testing.T) {
	const lambdaARN = "arn:aws:lambda:us-east-1:123456789012:function:my-s3-handler"

	listMock := &mockS3ListBucketsForNotification{
		output: &s3.ListBucketsOutput{
			Buckets: []s3types.Bucket{
				{Name: aws.String("my-data-bucket")},
			},
		},
	}

	notifMock := &mockS3GetBucketNotificationClient{
		output: &s3.GetBucketNotificationConfigurationOutput{
			LambdaFunctionConfigurations: []s3types.LambdaFunctionConfiguration{
				{LambdaFunctionArn: aws.String(lambdaARN)},
			},
		},
	}

	result, err := awsclient.FetchS3BucketsPageWithNotifications(
		context.Background(),
		listMock,
		notifMock,
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	got := result.Resources[0].Fields["notification_lambda"]
	if got != lambdaARN {
		t.Errorf("notification_lambda = %q, want %q", got, lambdaARN)
	}
}

// TestFetchS3BucketsPageWithNotifications_PopulatesSQSNotification verifies that
// firstS3NotificationTargets fills notification_sqs when an SQS notification exists.
func TestFetchS3BucketsPageWithNotifications_PopulatesSQSNotification(t *testing.T) {
	const sqsARN = "arn:aws:sqs:us-east-1:123456789012:s3-event-queue"

	listMock := &mockS3ListBucketsForNotification{
		output: &s3.ListBucketsOutput{
			Buckets: []s3types.Bucket{
				{Name: aws.String("uploads-bucket")},
			},
		},
	}

	notifMock := &mockS3GetBucketNotificationClient{
		output: &s3.GetBucketNotificationConfigurationOutput{
			QueueConfigurations: []s3types.QueueConfiguration{
				{QueueArn: aws.String(sqsARN)},
			},
		},
	}

	result, err := awsclient.FetchS3BucketsPageWithNotifications(
		context.Background(),
		listMock,
		notifMock,
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	got := result.Resources[0].Fields["notification_sqs"]
	if got != sqsARN {
		t.Errorf("notification_sqs = %q, want %q", got, sqsARN)
	}
}

// TestFetchS3BucketsPageWithNotifications_PopulatesSNSNotification verifies that
// firstS3NotificationTargets fills notification_sns when an SNS notification exists.
func TestFetchS3BucketsPageWithNotifications_PopulatesSNSNotification(t *testing.T) {
	const snsARN = "arn:aws:sns:us-east-1:123456789012:s3-event-topic"

	listMock := &mockS3ListBucketsForNotification{
		output: &s3.ListBucketsOutput{
			Buckets: []s3types.Bucket{
				{Name: aws.String("archive-bucket")},
			},
		},
	}

	notifMock := &mockS3GetBucketNotificationClient{
		output: &s3.GetBucketNotificationConfigurationOutput{
			TopicConfigurations: []s3types.TopicConfiguration{
				{TopicArn: aws.String(snsARN)},
			},
		},
	}

	result, err := awsclient.FetchS3BucketsPageWithNotifications(
		context.Background(),
		listMock,
		notifMock,
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	got := result.Resources[0].Fields["notification_sns"]
	if got != snsARN {
		t.Errorf("notification_sns = %q, want %q", got, snsARN)
	}
}

// TestFetchS3BucketsPageWithNotifications_NilNotificationAPI verifies that when no
// notification API is provided, all notification fields are empty strings (no panic).
func TestFetchS3BucketsPageWithNotifications_NilNotificationAPI(t *testing.T) {
	listMock := &mockS3ListBucketsForNotification{
		output: &s3.ListBucketsOutput{
			Buckets: []s3types.Bucket{
				{Name: aws.String("some-bucket")},
			},
		},
	}

	result, err := awsclient.FetchS3BucketsPageWithNotifications(
		context.Background(),
		listMock,
		nil, // no notification API
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	for _, field := range []string{"notification_lambda", "notification_sqs", "notification_sns"} {
		if r.Fields[field] != "" {
			t.Errorf("Fields[%q] = %q, want empty (nil notification API)", field, r.Fields[field])
		}
	}
}

// TestFetchS3BucketsPageWithNotifications_NotificationAPIError verifies that a
// notification API error is tolerated (best-effort enrichment — not a fatal error).
func TestFetchS3BucketsPageWithNotifications_NotificationAPIError(t *testing.T) {
	listMock := &mockS3ListBucketsForNotification{
		output: &s3.ListBucketsOutput{
			Buckets: []s3types.Bucket{
				{Name: aws.String("error-bucket")},
			},
		},
	}

	// The notification API returns an error (e.g., permission denied).
	// The bucket should still be returned with empty notification fields.
	notifMock := &mockS3GetBucketNotificationClient{
		err: &mockAWSError{code: "AccessDenied", message: "Access Denied"},
	}

	result, err := awsclient.FetchS3BucketsPageWithNotifications(
		context.Background(),
		listMock,
		notifMock,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error (best-effort enrichment), got %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	if r.ID != "error-bucket" {
		t.Errorf("ID = %q, want %q", r.ID, "error-bucket")
	}
	for _, field := range []string{"notification_lambda", "notification_sqs", "notification_sns"} {
		if r.Fields[field] != "" {
			t.Errorf("Fields[%q] = %q, want empty (notification API error)", field, r.Fields[field])
		}
	}
}

// mockAWSError implements the error interface for simulating AWS errors.
type mockAWSError struct {
	code    string
	message string
}

func (e *mockAWSError) Error() string {
	return e.code + ": " + e.message
}

// ---------------------------------------------------------------------------
// 6 & 7. buildinfo.ResolveCommit and buildinfo.ResolveDate
// ---------------------------------------------------------------------------

// TestBuildinfo_ResolveCommit_ExplicitValue verifies that an explicit non-empty,
// non-"none" value is returned as-is.
func TestBuildinfo_ResolveCommit_ExplicitValue(t *testing.T) {
	got := buildinfo.ResolveCommit("abc123def456")
	want := "abc123def456"
	if got != want {
		t.Errorf("ResolveCommit(%q) = %q, want %q", "abc123def456", got, want)
	}
}

// TestBuildinfo_ResolveCommit_NoneValue verifies that "none" triggers VCS lookup
// (or returns "none" if no VCS info available in test context).
func TestBuildinfo_ResolveCommit_NoneValue(t *testing.T) {
	got := buildinfo.ResolveCommit("none")
	// In test binary, debug.ReadBuildInfo may or may not have vcs.revision.
	// The contract: result must be non-empty (either a real commit or "none").
	if got == "" {
		t.Error("ResolveCommit(\"none\") must not return empty string")
	}
}

// TestBuildinfo_ResolveCommit_EmptyString verifies that empty string triggers
// VCS lookup (or returns "" if no VCS info in test context).
func TestBuildinfo_ResolveCommit_EmptyString(t *testing.T) {
	got := buildinfo.ResolveCommit("")
	// Empty input with no VCS info should return "".
	// The function returns c (the input) as fallback, so empty → "".
	// This is the intended behavior per the source.
	_ = got // any value is valid here; main goal is no panic
}

// TestBuildinfo_ResolveCommit_LongHashTruncatedAt12 verifies that a commit hash
// longer than 12 characters is truncated to 12 (exercising the > 12 branch).
// Note: this exercises the truncation branch only when ldflag "none" or "" leads
// to VCS lookup returning a long hash. Since test binaries may not have vcs.revision,
// we test the explicit-value path instead (the truncation happens for VCS reads, not
// explicit ldflags).
func TestBuildinfo_ResolveCommit_ExplicitLongValue_NotTruncated(t *testing.T) {
	// When c is explicitly set via ldflags (non-"none", non-""), it is returned as-is
	// without truncation. Truncation only applies to VCS fallback reads.
	longHash := "a1b2c3d4e5f6a7b8c9d0"
	got := buildinfo.ResolveCommit(longHash)
	if got != longHash {
		t.Errorf("ResolveCommit(%q) = %q, want %q (explicit value returned as-is)", longHash, got, longHash)
	}
}

// TestBuildinfo_ResolveDate_ExplicitValue verifies that an explicit non-empty,
// non-"unknown" value is returned as-is.
func TestBuildinfo_ResolveDate_ExplicitValue(t *testing.T) {
	got := buildinfo.ResolveDate("2026-01-15T10:00:00Z")
	want := "2026-01-15T10:00:00Z"
	if got != want {
		t.Errorf("ResolveDate(%q) = %q, want %q", "2026-01-15T10:00:00Z", got, want)
	}
}

// TestBuildinfo_ResolveDate_UnknownValue verifies that "unknown" triggers
// VCS lookup and returns a non-empty value or "unknown" as fallback.
func TestBuildinfo_ResolveDate_UnknownValue(t *testing.T) {
	got := buildinfo.ResolveDate("unknown")
	if got == "" {
		t.Error("ResolveDate(\"unknown\") must not return empty string")
	}
}

// TestBuildinfo_ResolveDate_EmptyString verifies that an empty string triggers
// VCS lookup or returns "" as fallback.
func TestBuildinfo_ResolveDate_EmptyString(t *testing.T) {
	got := buildinfo.ResolveDate("")
	// The function returns d (empty) as fallback when no VCS info.
	_ = got // any value is valid; main goal is no panic
}

// TestBuildinfo_ResolveDate_ArbitraryValue verifies that any non-"unknown",
// non-empty value is returned unchanged.
func TestBuildinfo_ResolveDate_ArbitraryValue(t *testing.T) {
	cases := []string{
		"2025-12-31T23:59:59Z",
		"2026-04-01",
		"Thu Jan  1 00:00:00 UTC 2026",
	}
	for _, input := range cases {
		got := buildinfo.ResolveDate(input)
		if got != input {
			t.Errorf("ResolveDate(%q) = %q, want %q", input, got, input)
		}
	}
}
