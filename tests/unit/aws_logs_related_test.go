package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func logsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("logs") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("logs related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("logs related checker for %s not found", target)
	return nil
}

// --- Lambda checker (Pattern C — cache, name parsed from /aws/lambda/{name}) ---

func TestRelated_Logs_Lambda_Found(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"
	const functionName = "my-function"

	lambdaRes := resource.Resource{
		ID:   functionName,
		Name: functionName,
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:   logGroupName,
		Name: logGroupName,
		Fields: map[string]string{
			"log_group_name": logGroupName,
		},
	}

	checker := logsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != functionName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, functionName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Logs_Lambda_NotLambdaGroup(t *testing.T) {
	const logGroupName = "/aws/rds/instance/mydb/error"

	lambdaRes := resource.Resource{
		ID:   "mydb",
		Name: "mydb",
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:   logGroupName,
		Name: logGroupName,
		Fields: map[string]string{
			"log_group_name": logGroupName,
		},
	}

	checker := logsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (not a lambda log group)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Logs_Lambda_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "/aws/lambda/my-function",
		Name: "/aws/lambda/my-function",
		Fields: map[string]string{
			"log_group_name": "/aws/lambda/my-function",
		},
	}

	checker := logsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- Alarms checker (Pattern C — cache, LogGroupName dimension) ---

func TestRelated_Logs_Alarms_Found(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"

	alarmRes := resource.Resource{
		ID: "log-group-error-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("log-group-error-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("LogGroupName"), Value: aws.String(logGroupName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   logGroupName,
		Name: logGroupName,
		Fields: map[string]string{
			"log_group_name": logGroupName,
		},
	}

	checker := logsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "log-group-error-alarm" {
		t.Errorf("ResourceIDs = %v, want [log-group-error-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Logs_Alarms_NotFound(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"

	alarmRes := resource.Resource{
		ID: "other-log-group-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-log-group-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("LogGroupName"), Value: aws.String("/aws/lambda/different-function")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   logGroupName,
		Name: logGroupName,
		Fields: map[string]string{
			"log_group_name": logGroupName,
		},
	}

	checker := logsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Logs_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "/aws/lambda/my-function",
		Name: "/aws/lambda/my-function",
		Fields: map[string]string{
			"log_group_name": "/aws/lambda/my-function",
		},
	}

	checker := logsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count)
	}
}

// --- KMS checker (Pattern F — reads KmsKeyId from LogGroup RawStruct) ---

func TestRelated_Logs_KMS_MatchByARN(t *testing.T) {
	source := resource.Resource{
		ID: "/aws/lambda/my-function",
		RawStruct: cloudwatchlogstypes.LogGroup{
			LogGroupName: aws.String("/aws/lambda/my-function"),
			KmsKeyId:     aws.String("arn:aws:kms:us-east-1:123456789012:key/abcd-1234"),
		},
	}

	checker := logsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abcd-1234" {
		t.Errorf("ResourceIDs = %v, want [abcd-1234]", result.ResourceIDs)
	}
}

func TestRelated_Logs_KMS_MatchByPlainKeyID(t *testing.T) {
	source := resource.Resource{
		ID: "/aws/lambda/my-function",
		RawStruct: cloudwatchlogstypes.LogGroup{
			LogGroupName: aws.String("/aws/lambda/my-function"),
			KmsKeyId:     aws.String("mrk-abcd1234"),
		},
	}

	checker := logsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "mrk-abcd1234" {
		t.Errorf("ResourceIDs = %v, want [mrk-abcd1234]", result.ResourceIDs)
	}
}

func TestRelated_Logs_KMS_NoKey(t *testing.T) {
	source := resource.Resource{
		ID: "/aws/lambda/my-function",
		RawStruct: cloudwatchlogstypes.LogGroup{
			LogGroupName: aws.String("/aws/lambda/my-function"),
			KmsKeyId:     nil,
		},
	}

	checker := logsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil KmsKeyId)", result.Count)
	}
}

func TestRelated_Logs_KMS_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "/aws/lambda/my-function",
		RawStruct: "not-a-log-group",
	}

	checker := logsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for invalid RawStruct", result.Count)
	}
}

// --- APIGW checker (Pattern N+C — "API-Gateway-Execution-Logs_{id}/{stage}") ---

func TestRelated_Logs_APIGW_MatchByExecutionLogName(t *testing.T) {
	const apiID = "abc1234567"
	apiRes := resource.Resource{ID: apiID, Name: "my-api"}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{apiRes}},
	}
	source := resource.Resource{
		ID: "API-Gateway-Execution-Logs_" + apiID + "/prod",
	}

	checker := logsCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != apiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, apiID)
	}
}

func TestRelated_Logs_APIGW_NoExecutionLogPrefix(t *testing.T) {
	apiRes := resource.Resource{ID: "abc1234567", Name: "my-api"}
	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{apiRes}},
	}
	source := resource.Resource{
		ID: "/aws/apigateway/my-api",
	}

	checker := logsCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (not an execution log group)", result.Count)
	}
}

func TestRelated_Logs_APIGW_EmptyID(t *testing.T) {
	source := resource.Resource{ID: ""}

	checker := logsCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty ID", result.Count)
	}
}

func TestRelated_Logs_APIGW_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "API-Gateway-Execution-Logs_abc1234567/prod",
	}

	checker := logsCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count)
	}
}

// --- ECSTask checker (Pattern N+C — "/ecs/{family}") ---

func TestRelated_Logs_ECSTask_MatchByFamily(t *testing.T) {
	const family = "web-task"
	taskRes := resource.Resource{ID: "web-task:3", Name: "web-task"}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{taskRes}},
	}
	source := resource.Resource{ID: "/ecs/" + family}

	checker := logsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "web-task:3" {
		t.Errorf("ResourceIDs = %v, want [web-task:3]", result.ResourceIDs)
	}
}

func TestRelated_Logs_ECSTask_NoECSPrefix(t *testing.T) {
	taskRes := resource.Resource{ID: "web-task:3", Name: "web-task"}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{taskRes}},
	}
	source := resource.Resource{ID: "/aws/lambda/web-task"}

	checker := logsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (not an ecs log group)", result.Count)
	}
}

func TestRelated_Logs_ECSTask_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "/ecs/web-task"}

	checker := logsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count)
	}
}

// --- Kinesis checker (Pattern C — nil clients → -1, empty filter list → 0) ---

func TestRelated_Logs_Kinesis_NilClients(t *testing.T) {
	source := resource.Resource{ID: "/aws/lambda/my-function"}

	checker := logsCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- S3 checker (Pattern C — nil clients → -1) ---

func TestRelated_Logs_S3_NilClients(t *testing.T) {
	source := resource.Resource{ID: "/aws/lambda/my-function"}

	checker := logsCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLogsKinesis — live DescribeSubscriptionFilters path
// ---------------------------------------------------------------------------

// TestRelated_Logs_Kinesis_FoundViaSubscriptionFilter verifies that a
// subscription filter whose DestinationArn is a Kinesis stream ARN returns
// the stream name as a resource ID.
func TestRelated_Logs_Kinesis_FoundViaSubscriptionFilter(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"
	const streamName = "acme-audit-stream"
	const kinesisARN = "arn:aws:kinesis:us-east-1:123456789012:stream/" + streamName
	dest := kinesisARN
	filters := []cloudwatchlogstypes.SubscriptionFilter{
		{DestinationArn: &dest},
	}
	clients := &awsclient.ServiceClients{
		CloudWatchLogs: newFakeCWLogsWithSubFilters(filters),
	}
	source := resource.Resource{ID: logGroupName}

	checker := logsCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != streamName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, streamName)
	}
}

// TestRelated_Logs_Kinesis_NonKinesisFilterReturnsZero verifies Count=0 when
// the subscription filter destination is not a Kinesis ARN.
func TestRelated_Logs_Kinesis_NonKinesisFilterReturnsZero(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"
	dest := "arn:aws:firehose:us-east-1:123456789012:deliverystream/acme"
	filters := []cloudwatchlogstypes.SubscriptionFilter{
		{DestinationArn: &dest},
	}
	clients := &awsclient.ServiceClients{
		CloudWatchLogs: newFakeCWLogsWithSubFilters(filters),
	}
	source := resource.Resource{ID: logGroupName}

	checker := logsCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no kinesis filter)", result.Count)
	}
}

// TestRelated_Logs_Kinesis_EmptyFilterListReturnsZero verifies Count=0 when
// the log group has no subscription filters (API succeeds with empty list).
func TestRelated_Logs_Kinesis_EmptyFilterListReturnsZero(t *testing.T) {
	clients := &awsclient.ServiceClients{
		CloudWatchLogs: newFakeCWLogsWithSubFilters([]cloudwatchlogstypes.SubscriptionFilter{}),
	}
	source := resource.Resource{ID: "/aws/lambda/my-function"}

	checker := logsCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty filter list)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkLogsS3 — live DescribeSubscriptionFilters path
// ---------------------------------------------------------------------------

// TestRelated_Logs_S3_FoundViaSubscriptionFilter verifies that a subscription
// filter whose DestinationArn is an S3 bucket ARN returns the bucket name.
func TestRelated_Logs_S3_FoundViaSubscriptionFilter(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"
	const bucketName = "acme-audit-logs-bucket"
	const s3ARN = "arn:aws:s3:::" + bucketName
	dest := s3ARN
	filters := []cloudwatchlogstypes.SubscriptionFilter{
		{DestinationArn: &dest},
	}
	clients := &awsclient.ServiceClients{
		CloudWatchLogs: newFakeCWLogsWithSubFilters(filters),
	}
	source := resource.Resource{ID: logGroupName}

	checker := logsCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != bucketName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, bucketName)
	}
}

// TestRelated_Logs_S3_BucketWithPathPrefixExtractsBucketName verifies that
// an S3 ARN with a path suffix (bucket/prefix) extracts only the bucket name.
func TestRelated_Logs_S3_BucketWithPathPrefixExtractsBucketName(t *testing.T) {
	const bucketName = "acme-audit-logs-bucket"
	dest := "arn:aws:s3:::" + bucketName + "/logs/prefix"
	filters := []cloudwatchlogstypes.SubscriptionFilter{
		{DestinationArn: &dest},
	}
	clients := &awsclient.ServiceClients{
		CloudWatchLogs: newFakeCWLogsWithSubFilters(filters),
	}
	source := resource.Resource{ID: "/aws/lambda/my-function"}

	checker := logsCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != bucketName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, bucketName)
	}
}

// TestRelated_Logs_S3_NonS3FilterReturnsZero verifies Count=0 when the
// subscription filter destination is not an S3 ARN.
func TestRelated_Logs_S3_NonS3FilterReturnsZero(t *testing.T) {
	dest := "arn:aws:kinesis:us-east-1:123456789012:stream/my-stream"
	filters := []cloudwatchlogstypes.SubscriptionFilter{
		{DestinationArn: &dest},
	}
	clients := &awsclient.ServiceClients{
		CloudWatchLogs: newFakeCWLogsWithSubFilters(filters),
	}
	source := resource.Resource{ID: "/aws/lambda/my-function"}

	checker := logsCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no S3 filter)", result.Count)
	}
}
