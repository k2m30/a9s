package unit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != functionName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), functionName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (not a lambda log group)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
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

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count())
	}
}

func TestRelated_Logs_Alarms_Found(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"

	alarmRes := resource.Resource{
		ID: "log-group-error-alarm",
		RawStruct: cwtypes.MetricAlarm{
			Namespace: aws.String("AWS/Logs"),
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "log-group-error-alarm" {
		t.Errorf("ResourceIDs = %v, want [log-group-error-alarm]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
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

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown/cache miss)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "abcd-1234" {
		t.Errorf("ResourceIDs = %v, want [abcd-1234]", result.ResourceIDs())
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "mrk-abcd1234" {
		t.Errorf("ResourceIDs = %v, want [mrk-abcd1234]", result.ResourceIDs())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (nil KmsKeyId)", result.Count())
	}
}

func TestRelated_Logs_KMS_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "/aws/lambda/my-function",
		RawStruct: "not-a-log-group",
	}

	checker := logsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for invalid RawStruct", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != apiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), apiID)
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (not an execution log group)", result.Count())
	}
}

func TestRelated_Logs_APIGW_EmptyID(t *testing.T) {
	source := resource.Resource{ID: ""}

	checker := logsCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for empty ID", result.Count())
	}
}

func TestRelated_Logs_APIGW_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "API-Gateway-Execution-Logs_abc1234567/prod",
	}

	checker := logsCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count())
	}
}

// TestRelated_Logs_ECSTask_MatchByFamily verifies that checkLogsECSTask
// extracts the family from Fields["task_definition"] (the full task-definition
// ARN, with the trailing :revision stripped after arnLastSegment), not from
// the task's Name/ID.
// A task writes to every log group its containers name in the awslogs
// driver's awslogs-group option, in the task's Region unless awslogs-region
// names another; a group of the same name in another Region is a different
// group. A sidecar on another driver writes to no CloudWatch Logs group.
// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_awslogs.html
func TestRelated_Logs_ECSTask_MatchByFamily(t *testing.T) {
	const (
		webTD    = "arn:aws:ecs:us-east-1:123456789012:task-definition/web-task:3"
		reportTD = "arn:aws:ecs:us-east-1:123456789012:task-definition/report-task:5"
	)
	awslogs := func(name, group, region string) ecstypes.ContainerDefinition {
		return ecstypes.ContainerDefinition{Name: aws.String(name), Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/" + name + ":v3"),
			LogConfiguration: &ecstypes.LogConfiguration{LogDriver: ecstypes.LogDriverAwslogs, Options: map[string]string{
				"awslogs-group": group, "awslogs-region": region, "awslogs-stream-prefix": name}}}
	}
	fake := &t568ECS{taskDefs: map[string]ecstypes.TaskDefinition{
		webTD: {Family: aws.String("web-task"), Revision: 3, ContainerDefinitions: []ecstypes.ContainerDefinition{
			awslogs("web", "/ecs/web-task", "us-east-1"),
			awslogs("envoy", "/ecs/web-task/envoy", "us-east-1"),
			{Name: aws.String("log-router"), Image: aws.String("public.ecr.aws/aws-observability/aws-for-fluent-bit:stable"),
				FirelensConfiguration: &ecstypes.FirelensConfiguration{Type: ecstypes.FirelensConfigurationTypeFluentbit}},
		}},
		reportTD: {Family: aws.String("report-task"), Revision: 5, ContainerDefinitions: []ecstypes.ContainerDefinition{
			awslogs("report", "/ecs/web-task", "us-west-2"),
		}},
	}}
	task := func(id, td string) resource.Resource {
		return resource.Resource{ID: id, Name: id, Type: "ecs-task",
			Fields: map[string]string{"cluster": "prod", "task_definition": td, "last_status": "RUNNING"},
			RawStruct: ecstypes.Task{TaskArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod/" + id),
				TaskDefinitionArn: aws.String(td), ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/prod"), LastStatus: aws.String("RUNNING")}}
	}
	const webTask, reportTask = "3c4d5e6f7a8b4c9d0e1f2a3b4c5d6e7f", "8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e"
	cache := resource.ResourceCache{"ecs-task": {Resources: []resource.Resource{task(webTask, webTD), task(reportTask, reportTD)}}}
	clients := &awsclient.ServiceClients{ECS: fake, Region: "us-east-1"}
	checker := logsCheckerByTarget(t, "ecs-task")

	for _, group := range []string{"/ecs/web-task", "/ecs/web-task/envoy"} {
		source := resource.Resource{ID: group, Name: group, Type: "logs", Fields: map[string]string{"log_group_name": group}}
		t568RequireExact(t, "logs "+group+" → ecs-task", checker(context.Background(), clients, source, cache), webTask)
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (not an ecs log group)", result.Count())
	}
}

func TestRelated_Logs_ECSTask_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "/ecs/web-task"}

	checker := logsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count())
	}
}

func TestRelated_Logs_Kinesis_NilClients(t *testing.T) {
	source := resource.Resource{ID: "/aws/lambda/my-function"}

	checker := logsCheckerByTarget(t, "kinesis")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

func TestRelated_Logs_S3_NilClients(t *testing.T) {
	source := resource.Resource{ID: "/aws/lambda/my-function"}

	checker := logsCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != streamName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), streamName)
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no kinesis filter)", result.Count())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty filter list)", result.Count())
	}
}

// logsExportClients answers DescribeExportTasks with tasks; subscription
// filters, which deliver to Kinesis, Firehose, Lambda or OpenSearch and never
// to S3, are none.
func logsExportClients(tasks ...cloudwatchlogstypes.ExportTask) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{CloudWatchLogs: &t570CWLogs{
		CWLogsAPI: newFakeCWLogsWithSubFilters(nil),
		exports:   tasks,
	}}
}

func logsExportTask(group, bucket, prefix string) cloudwatchlogstypes.ExportTask {
	return cloudwatchlogstypes.ExportTask{
		TaskId:            aws.String("0f1e2d3c-0000-4000-8000-" + fmt.Sprintf("%012d", len(group))),
		TaskName:          aws.String("archive"),
		LogGroupName:      aws.String(group),
		Destination:       aws.String(bucket),
		DestinationPrefix: aws.String(prefix),
		From:              aws.Int64(1785542400000),
		To:                aws.Int64(1788220800000),
		Status:            &cloudwatchlogstypes.ExportTaskStatus{Code: cloudwatchlogstypes.ExportTaskStatusCodeCompleted},
	}
}

// TestRelated_Logs_S3_FoundViaSubscriptionFilter: a log group's S3 archive is
// the bucket its export tasks wrote to (DescribeExportTasks
// ExportTask.Destination); another group's export is not this group's.
func TestRelated_Logs_S3_FoundViaSubscriptionFilter(t *testing.T) {
	const logGroupName = "/aws/lambda/my-function"
	const bucketName = "acme-audit-logs-bucket"
	clients := logsExportClients(
		logsExportTask(logGroupName, bucketName, "exports"),
		logsExportTask("/aws/lambda/other-function", "acme-other-archive", "exports"),
	)
	source := resource.Resource{ID: logGroupName}

	checker := logsCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != bucketName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), bucketName)
	}
}

// TestRelated_Logs_S3_BucketWithPathPrefixExtractsBucketName: an export's key
// prefix (ExportTask.DestinationPrefix) is a path inside the bucket, not part
// of its name.
func TestRelated_Logs_S3_BucketWithPathPrefixExtractsBucketName(t *testing.T) {
	const bucketName = "acme-audit-logs-bucket"
	clients := logsExportClients(logsExportTask("/aws/lambda/my-function", bucketName, "logs/prefix"))
	source := resource.Resource{ID: "/aws/lambda/my-function"}

	checker := logsCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != bucketName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), bucketName)
	}
}

// TestRelated_Logs_S3_NonS3FilterReturnsZero: a group no export task wrote
// has no S3 archive.
func TestRelated_Logs_S3_NonS3FilterReturnsZero(t *testing.T) {
	clients := logsExportClients(logsExportTask("/aws/lambda/other-function", "acme-other-archive", "exports"))
	source := resource.Resource{ID: "/aws/lambda/my-function"}

	checker := logsCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.State() != domain.RelatedResolved || result.Count() != 0 {
		t.Errorf("state = %v, Count = %d, want a resolved 0", result.State(), result.Count())
	}
}
