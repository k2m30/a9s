package unit_test

import (
	"strings"
	"testing"
	"time"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ===========================================================================
// Log Stream detail view tests
// ===========================================================================

func TestQA_Detail_LogStream_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ls := cwlogstypes.LogStream{
		LogStreamName:       ptrString("2024/03/22/[$LATEST]abcdef1234567890"),
		Arn:                 ptrString("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-func:log-stream:2024/03/22/[$LATEST]abcdef1234567890"),
		FirstEventTimestamp: ptrInt64(1711065600000),
		LastEventTimestamp:  ptrInt64(1711152000000),
		StoredBytes:         ptrInt64(14336),
		CreationTime:        ptrInt64(1711060000000),
	}
	res := buildResource(
		"2024/03/22/[$LATEST]abcdef1234567890",
		"2024/03/22/[$LATEST]abcdef1234567890",
		ls,
	)
	cfg := detailConfigForType("log_streams")
	m := newDetailModel(res, "log_streams", cfg)

	view := m.View()
	for _, expected := range []string{
		"LogStreamName", "abcdef1234567890",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("LogStream detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_LogStream_NilFields(t *testing.T) {
	ensureNoColor(t)
	ls := cwlogstypes.LogStream{}
	res := buildResource("empty-stream", "empty-stream", ls)
	cfg := detailConfigForType("log_streams")
	m := newDetailModel(res, "log_streams", cfg)

	view := m.View()
	if view == "" {
		t.Error("LogStream detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// Log Event detail view tests
// ===========================================================================

func TestQA_Detail_LogEvent_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.OutputLogEvent{
		Timestamp:     ptrInt64(1711065600000),
		Message:       ptrString("ERROR NullPointerException in com.example.App.main"),
		IngestionTime: ptrInt64(1711065601000),
	}
	res := buildResource(
		"evt-1711065600000",
		"ERROR NullPointerException in com.example.App.main",
		ev,
	)
	cfg := detailConfigForType("log_events")
	m := newDetailModel(res, "log_events", cfg)

	view := m.View()
	for _, expected := range []string{
		"Message", "NullPointerException",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("LogEvent detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_LogEvent_NilFields(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.OutputLogEvent{}
	res := buildResource("empty-event", "empty-event", ev)
	cfg := detailConfigForType("log_events")
	m := newDetailModel(res, "log_events", cfg)

	view := m.View()
	if view == "" {
		t.Error("LogEvent detail should not be empty even with nil fields")
	}
}

// TestQA_Detail_LogEvent_FormattedTimestamps verifies that timestamps in the
// detail view show human-readable format, not raw epoch milliseconds.
// The detail renderer should prefer Fields values (which are pre-formatted)
// over raw SDK struct extraction for fields that match Fields keys.
func TestQA_Detail_LogEvent_FormattedTimestamps(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.OutputLogEvent{
		Timestamp:     ptrInt64(1711065600000),
		Message:       ptrString("test message"),
		IngestionTime: ptrInt64(1711065601000),
	}
	res := buildResource("evt-001", "test message", ev)
	// Fetcher populates Fields with formatted values
	res.Fields = map[string]string{
		"timestamp":      "2024-03-22 00:00",
		"message":        "test message",
		"ingestion_time": "2024-03-22 00:00",
	}
	cfg := detailConfigForType("log_events")
	m := newDetailModel(res, "log_events", cfg)

	view := m.View()
	// Should show formatted timestamp, NOT raw epoch ms
	if strings.Contains(view, "1711065600000") {
		t.Errorf("detail view should show formatted timestamp, not raw epoch ms:\n%s", view)
	}
	if !strings.Contains(view, "2024-03-22") {
		t.Errorf("detail view should contain formatted date '2024-03-22':\n%s", view)
	}
}

// ===========================================================================
// Target Health detail view tests (child of Target Groups)
// ===========================================================================

func TestQA_Detail_TargetHealth_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	port := int32(8080)
	thd := elbtypes.TargetHealthDescription{
		Target: &elbtypes.TargetDescription{
			Id:               ptrString("i-0abc1234def56789a"),
			Port:             &port,
			AvailabilityZone: ptrString("us-east-1a"),
		},
		TargetHealth: &elbtypes.TargetHealth{
			State:       elbtypes.TargetHealthStateEnumUnhealthy,
			Reason:      elbtypes.TargetHealthReasonEnumFailedHealthChecks,
			Description: ptrString("Health checks failed with 503"),
		},
		HealthCheckPort: ptrString("8080"),
	}
	res := buildResource(
		"i-0abc1234def56789a",
		"i-0abc1234def56789a",
		thd,
	)
	cfg := detailConfigForType("tg_health")
	m := newDetailModel(res, "tg_health", cfg)

	view := m.View()
	for _, expected := range []string{
		"i-0abc1234def56789a",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("TargetHealth detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_TargetHealth_NilFields(t *testing.T) {
	ensureNoColor(t)
	thd := elbtypes.TargetHealthDescription{}
	res := buildResource("empty-target", "empty-target", thd)
	cfg := detailConfigForType("tg_health")
	m := newDetailModel(res, "tg_health", cfg)

	view := m.View()
	if view == "" {
		t.Error("TargetHealth detail should not be empty even with nil fields")
	}
}

// TestQA_Detail_LongFieldNames verifies that detail view field names are not
// truncated. Field names like "Target.AvailabilityZone" and
// "HealthCheckIntervalSeconds" must be fully visible.
func TestQA_Detail_LongFieldNames(t *testing.T) {
	ensureNoColor(t)
	port := int32(80)
	thd := elbtypes.TargetHealthDescription{
		Target: &elbtypes.TargetDescription{
			Id:               ptrString("10.10.19.75"),
			Port:             &port,
			AvailabilityZone: ptrString("eu-west-2b"),
		},
		TargetHealth: &elbtypes.TargetHealth{
			State:       elbtypes.TargetHealthStateEnumHealthy,
			Description: ptrString("Target is healthy"),
		},
	}
	res := buildResource("10.10.19.75", "10.10.19.75", thd)
	cfg := detailConfigForType("tg_health")
	m := newDetailModel(res, "tg_health", cfg)

	view := m.View()
	// Long field names must NOT be truncated with ellipsis
	longNames := []string{
		"Target.AvailabilityZone:",
		"TargetHealth.State:",
		"TargetHealth.Description:",
	}
	for _, name := range longNames {
		if !strings.Contains(view, name) {
			t.Errorf("detail view should show full field name %q without truncation:\n%s", name, view)
		}
	}
}

// ===========================================================================
// Lambda Invocation detail view tests (child of Lambda)
// ===========================================================================

func TestLambdaInvocationDetailViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.FilteredLogEvent{
		Timestamp:     ptrInt64(1711065600000),
		Message:       ptrString("REPORT RequestId: 12345678-1234-1234-1234-123456789012\tDuration: 2103.45 ms\tBilled Duration: 2200 ms\tMemory Size: 256 MB\tMax Memory Used: 128 MB\t"),
		IngestionTime: ptrInt64(1711065601000),
		LogStreamName: ptrString("2024/03/22/[$LATEST]abcdef"),
		EventId:       ptrString("evt-001"),
	}
	res := buildResource(
		"12345678-1234-1234-1234-123456789012",
		"12345678-1234-1234-1234-123456789012",
		ev,
	)
	res.Fields = map[string]string{
		"request_id":         "12345678-1234-1234-1234-123456789012",
		"timestamp":          "2024-03-22 00:00",
		"status":             "OK",
		"duration_ms":        "2103 ms",
		"billed_duration_ms": "2200 ms",
		"memory_size_mb":     "256",
		"memory_used_mb":     "128",
		"cold_start":         "no",
		"memory_used":        "128/256 MB",
	}
	cfg := detailConfigForType("lambda_invocations")
	m := newDetailModel(res, "lambda_invocations", cfg)

	view := m.View()
	for _, expected := range []string{
		"12345678-1234-1234-1234-123456789012",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("LambdaInvocation detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestLambdaInvocationDetailViewNilFields(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.FilteredLogEvent{}
	res := buildResource("empty-invocation", "empty-invocation", ev)
	cfg := detailConfigForType("lambda_invocations")
	m := newDetailModel(res, "lambda_invocations", cfg)

	view := m.View()
	if view == "" {
		t.Error("LambdaInvocation detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// Lambda Invocation Log detail view tests (level-2 child of Invocations)
// ===========================================================================

func TestLambdaInvocationLogDetailViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.FilteredLogEvent{
		Timestamp:     ptrInt64(1711065600000),
		Message:       ptrString("INFO Processing request for user abc-123"),
		IngestionTime: ptrInt64(1711065600500),
		EventId:       ptrString("log-002"),
	}
	res := buildResource(
		"log-002",
		"INFO Processing request for user abc-123",
		ev,
	)
	res.Fields = map[string]string{
		"timestamp": "2024-03-22 00:00",
		"message":   "INFO Processing request for user abc-123",
	}
	cfg := detailConfigForType("lambda_invocation_logs")
	m := newDetailModel(res, "lambda_invocation_logs", cfg)

	view := m.View()
	for _, expected := range []string{
		"Processing request",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("LambdaInvocationLog detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestLambdaInvocationLogDetailViewNilFields(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.FilteredLogEvent{}
	res := buildResource("empty-log", "empty-log", ev)
	cfg := detailConfigForType("lambda_invocation_logs")
	m := newDetailModel(res, "lambda_invocation_logs", cfg)

	view := m.View()
	if view == "" {
		t.Error("LambdaInvocationLog detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// ECS Service Events detail view tests (child of ECS Services)
// ===========================================================================

func TestQA_Detail_EcsSvcEvents_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	ev := ecstypes.ServiceEvent{
		Id:        ptrString("evt-detail-001"),
		CreatedAt: &ts,
		Message:   ptrString("(service web-service) has reached a steady state."),
	}
	res := buildResource(
		"evt-detail-001",
		"(service web-service) has reached a steady state.",
		ev,
	)
	res.Fields = map[string]string{
		"timestamp": "2024-03-22 10:00",
		"message":   "(service web-service) has reached a steady state.",
	}
	cfg := detailConfigForType("ecs_svc_events")
	m := newDetailModel(res, "ecs_svc_events", cfg)

	view := m.View()
	for _, expected := range []string{
		"steady state",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("EcsSvcEvents detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_EcsSvcEvents_NilFields(t *testing.T) {
	ensureNoColor(t)
	ev := ecstypes.ServiceEvent{}
	res := buildResource("empty-event", "empty-event", ev)
	cfg := detailConfigForType("ecs_svc_events")
	m := newDetailModel(res, "ecs_svc_events", cfg)

	view := m.View()
	if view == "" {
		t.Error("EcsSvcEvents detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// ECS Service Tasks detail view tests (child of ECS Services)
// ===========================================================================

func TestQA_Detail_EcsSvcTasks_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	startedAt := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	task := ecstypes.Task{
		TaskArn:           ptrString("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456"),
		LastStatus:        ptrString("RUNNING"),
		DesiredStatus:     ptrString("RUNNING"),
		HealthStatus:      ecstypes.HealthStatusHealthy,
		TaskDefinitionArn: ptrString("arn:aws:ecs:us-east-1:123456789012:task-definition/web-app:5"),
		StartedAt:         &startedAt,
		Cpu:               ptrString("256"),
		Memory:            ptrString("512"),
		LaunchType:        ecstypes.LaunchTypeFargate,
		PlatformVersion:   ptrString("1.4.0"),
		Group:             ptrString("service:web-service"),
	}
	res := buildResource(
		"abc123def456",
		"abc123def456",
		task,
	)
	res.Fields = map[string]string{
		"task_id_short":  "abc123def456",
		"status":         "RUNNING",
		"health":         "HEALTHY",
		"task_def_short": "web-app:5",
		"started_at":     "2024-03-22 10:00",
		"stopped_reason": "",
	}
	cfg := detailConfigForType("ecs_tasks")
	m := newDetailModel(res, "ecs_tasks", cfg)

	view := m.View()
	for _, expected := range []string{
		"abc123def456",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("EcsSvcTasks detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_EcsSvcTasks_NilFields(t *testing.T) {
	ensureNoColor(t)
	task := ecstypes.Task{}
	res := buildResource("empty-task", "empty-task", task)
	cfg := detailConfigForType("ecs_tasks")
	m := newDetailModel(res, "ecs_tasks", cfg)

	view := m.View()
	if view == "" {
		t.Error("EcsSvcTasks detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// ECS Service Logs detail view tests (child of ECS Services)
// ===========================================================================

func TestQA_Detail_EcsSvcLogs_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.FilteredLogEvent{
		Timestamp:     ptrInt64(1711036800000),
		Message:       ptrString("INFO Starting application server on port 8080"),
		IngestionTime: ptrInt64(1711036801000),
		LogStreamName: ptrString("ecs/web/abc123def456"),
		EventId:       ptrString("evt-svc-log-001"),
	}
	res := buildResource(
		"evt-svc-log-001",
		"INFO Starting application server on port 8080",
		ev,
	)
	res.Fields = map[string]string{
		"timestamp":    "2024-03-21 16:00",
		"stream_short": "web/abc123de",
		"message":      "INFO Starting application server on port 8080",
	}
	cfg := detailConfigForType("ecs_svc_logs")
	m := newDetailModel(res, "ecs_svc_logs", cfg)

	view := m.View()
	for _, expected := range []string{
		"Starting application",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("EcsSvcLogs detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_EcsSvcLogs_NilFields(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.FilteredLogEvent{}
	res := buildResource("empty-svc-log", "empty-svc-log", ev)
	cfg := detailConfigForType("ecs_svc_logs")
	m := newDetailModel(res, "ecs_svc_logs", cfg)

	view := m.View()
	if view == "" {
		t.Error("EcsSvcLogs detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// CFN Stack Events detail view tests
// ===========================================================================

func TestCfnEventsDetailViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	ev := cfntypes.StackEvent{
		EventId:              ptrString("evt-detail-001"),
		StackId:              ptrString("arn:aws:cloudformation:us-east-1:123456789012:stack/my-stack/guid1"),
		StackName:            ptrString("my-stack"),
		Timestamp:            &ts,
		LogicalResourceId:    ptrString("MyBucket"),
		PhysicalResourceId:   ptrString("my-stack-mybucket-abc123"),
		ResourceType:         ptrString("AWS::S3::Bucket"),
		ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
		ResourceStatusReason: ptrString("Resource creation complete"),
		ClientRequestToken:   ptrString("console-token-12345"),
	}
	res := buildResource(
		"evt-detail-001",
		"2024-03-22 10:00:00",
		ev,
	)
	res.Fields = map[string]string{
		"timestamp":              "2024-03-22 10:00:00",
		"logical_resource_id":    "MyBucket",
		"resource_type":          "AWS::S3::Bucket",
		"resource_status":        "CREATE_COMPLETE",
		"resource_status_reason": "Resource creation complete",
	}
	cfg := detailConfigForType("cfn_events")
	m := newDetailModel(res, "cfn_events", cfg)

	view := m.View()
	for _, expected := range []string{
		"MyBucket",
		"evt-detail-001",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("CfnEvents detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestCfnEventsDetailViewNilFields(t *testing.T) {
	ensureNoColor(t)
	ev := cfntypes.StackEvent{}
	res := buildResource("empty-cfn-event", "empty-cfn-event", ev)
	cfg := detailConfigForType("cfn_events")
	m := newDetailModel(res, "cfn_events", cfg)

	view := m.View()
	if view == "" {
		t.Error("CfnEvents detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// CFN Stack Resources detail view tests
// ===========================================================================

func TestCfnResourcesDetailViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	res_summary := cfntypes.StackResourceSummary{
		LogicalResourceId:    ptrString("MyBucket"),
		PhysicalResourceId:   ptrString("my-stack-mybucket-abc123"),
		ResourceType:         ptrString("AWS::S3::Bucket"),
		ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
		ResourceStatusReason: ptrString(""),
		LastUpdatedTimestamp: &ts,
		DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
			StackResourceDriftStatus: cfntypes.StackResourceDriftStatusInSync,
		},
	}
	res := buildResource(
		"MyBucket",
		"MyBucket",
		res_summary,
	)
	res.Fields = map[string]string{
		"logical_resource_id":  "MyBucket",
		"physical_resource_id": "my-stack-mybucket-abc123",
		"resource_type":        "AWS::S3::Bucket",
		"resource_status":      "CREATE_COMPLETE",
		"drift_status":         "IN_SYNC",
		"last_updated":         "2024-03-22 10:00:00",
	}
	cfg := detailConfigForType("cfn_resources")
	m := newDetailModel(res, "cfn_resources", cfg)

	view := m.View()
	for _, expected := range []string{
		"MyBucket",
		"my-stack-mybucket-abc123",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("CfnResources detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestCfnResourcesDetailViewNilFields(t *testing.T) {
	ensureNoColor(t)
	res_summary := cfntypes.StackResourceSummary{}
	res := buildResource("empty-cfn-resource", "empty-cfn-resource", res_summary)
	cfg := detailConfigForType("cfn_resources")
	m := newDetailModel(res, "cfn_resources", cfg)

	view := m.View()
	if view == "" {
		t.Error("CfnResources detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// ASG Scaling Activities detail view tests
// ===========================================================================

func TestAsgActivityDetailViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	endTs := time.Date(2024, 3, 22, 10, 5, 0, 0, time.UTC)
	progress := int32(100)
	activity := asgtypes.Activity{
		ActivityId:            ptrString("act-detail-001"),
		AutoScalingGroupName:  ptrString("my-asg"),
		AutoScalingGroupARN:   ptrString("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:guid:autoScalingGroupName/my-asg"),
		AutoScalingGroupState: ptrString("InService"),
		Cause:                 ptrString("At 2024-03-22T10:00:00Z an instance was started"),
		Description:           ptrString("Launching a new EC2 instance: i-0abc1234"),
		Details:               ptrString("{\"Subnet ID\":\"subnet-12345\"}"),
		StartTime:             &ts,
		EndTime:               &endTs,
		StatusCode:            asgtypes.ScalingActivityStatusCodeSuccessful,
		StatusMessage:         ptrString(""),
		Progress:              &progress,
	}
	res := buildResource(
		"act-detail-001",
		"2024-03-22 10:00:00",
		activity,
	)
	res.Fields = map[string]string{
		"start_time":  "2024-03-22 10:00:00",
		"status_code": "Successful",
		"description": "Launching a new EC2 instance: i-0abc1234",
		"cause":       "At 2024-03-22T10:00:00Z an instance was started",
	}
	cfg := detailConfigForType("asg_activities")
	m := newDetailModel(res, "asg_activities", cfg)

	view := m.View()
	for _, expected := range []string{
		"ActivityId",
		"StatusCode",
		"Description",
		"Cause",
		"Progress",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("AsgActivity detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestAsgActivityDetailViewNilFields(t *testing.T) {
	ensureNoColor(t)
	activity := asgtypes.Activity{}
	res := buildResource("empty-asg-activity", "empty-asg-activity", activity)
	cfg := detailConfigForType("asg_activities")
	m := newDetailModel(res, "asg_activities", cfg)

	view := m.View()
	if view == "" {
		t.Error("AsgActivity detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// Alarm History detail view tests
// ===========================================================================

func TestAlarmHistoryDetailViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	item := cwtypes.AlarmHistoryItem{
		AlarmName:       ptrString("HighCPUAlarm"),
		AlarmType:       cwtypes.AlarmTypeMetricAlarm,
		HistoryData:     ptrString(`{"version":"1.0","oldState":{"stateValue":"OK"}}`),
		HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
		HistorySummary:  ptrString("Alarm updated from OK to ALARM"),
		Timestamp:       &ts,
	}
	res := buildResource("2024-03-22 10:00:00", "2024-03-22 10:00:00", item)
	res.Status = "StateUpdate"
	res.Fields = map[string]string{
		"timestamp":         "2024-03-22 10:00:00",
		"history_item_type": "StateUpdate",
		"history_summary":   "Alarm updated from OK to ALARM",
	}
	cfg := detailConfigForType("alarm_history")
	m := newDetailModel(res, "alarm_history", cfg)

	view := m.View()
	for _, expected := range []string{
		"HistoryItemType",
		"HistorySummary",
		"AlarmName",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("AlarmHistory detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestAlarmHistoryDetailViewNilFields(t *testing.T) {
	ensureNoColor(t)
	item := cwtypes.AlarmHistoryItem{}
	res := buildResource("empty-alarm-history", "empty-alarm-history", item)
	cfg := detailConfigForType("alarm_history")
	m := newDetailModel(res, "alarm_history", cfg)

	view := m.View()
	if view == "" {
		t.Error("AlarmHistory detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// ELB Listeners detail view tests (child of Load Balancers)
// ===========================================================================

func TestQA_Detail_ELBListeners_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	listener := elbtypes.Listener{
		ListenerArn: ptrString("arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456"),
		Port:        ptrInt32(443),
		Protocol:    elbtypes.ProtocolEnumHttps,
		SslPolicy:   ptrString("ELBSecurityPolicy-TLS13-1-2-2021-06"),
		Certificates: []elbtypes.Certificate{{
			CertificateArn: ptrString("arn:aws:acm:us-east-1:123456789012:certificate/abc-def-123"),
		}},
		DefaultActions: []elbtypes.Action{{
			Type:           elbtypes.ActionTypeEnumForward,
			TargetGroupArn: ptrString("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-prod-tg/abc123"),
		}},
	}
	res := buildResource(
		"arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456",
		"443",
		listener,
	)
	cfg := detailConfigForType("elb_listeners")
	m := newDetailModel(res, "elb_listeners", cfg)

	view := m.View()
	for _, expected := range []string{
		"ListenerArn",
		"Port",
		"Protocol",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("ELBListeners detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_ELBListeners_NilFields(t *testing.T) {
	ensureNoColor(t)
	listener := elbtypes.Listener{}
	res := buildResource("empty-listener", "empty-listener", listener)
	cfg := detailConfigForType("elb_listeners")
	m := newDetailModel(res, "elb_listeners", cfg)

	view := m.View()
	if view == "" {
		t.Error("ELBListeners detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// SFN Executions detail view tests (child of Step Functions)
// ===========================================================================

func TestQA_Detail_SFNExecutions_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	stopTs := time.Date(2024, 6, 15, 10, 2, 47, 0, time.UTC)
	itemCount := int32(42)
	redriveCount := int32(1)
	redriveTs := time.Date(2024, 6, 15, 11, 0, 0, 0, time.UTC)

	item := sfntypes.ExecutionListItem{
		ExecutionArn:           ptrString("arn:aws:states:us-east-1:123456789012:execution:my-state-machine:exec-001"),
		Name:                   ptrString("exec-001"),
		StartDate:              &startTs,
		StopDate:               &stopTs,
		StateMachineArn:        ptrString("arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine"),
		Status:                 sfntypes.ExecutionStatusSucceeded,
		ItemCount:              &itemCount,
		MapRunArn:              ptrString("arn:aws:states:us-east-1:123456789012:mapRun:my-state-machine/exec-001:map-run-id"),
		RedriveCount:           &redriveCount,
		RedriveDate:            &redriveTs,
		StateMachineAliasArn:   ptrString("arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine:prod"),
		StateMachineVersionArn: ptrString("arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine:1"),
	}
	res := buildResource("exec-001", "exec-001", item)
	res.Status = "SUCCEEDED"
	res.Fields = map[string]string{
		"execution_arn":             "arn:aws:states:us-east-1:123456789012:execution:my-state-machine:exec-001",
		"name":                      "exec-001",
		"status":                    "SUCCEEDED",
		"start_date":                "2024-06-15 10:00:00",
		"stop_date":                 "2024-06-15 10:02:47",
		"duration":                  "2m 47s",
		"state_machine_arn":         "arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine",
		"state_machine_alias_arn":   "arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine:prod",
		"state_machine_version_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:my-state-machine:1",
		"map_run_arn":               "arn:aws:states:us-east-1:123456789012:mapRun:my-state-machine/exec-001:map-run-id",
		"item_count":                "42",
		"redrive_count":             "1",
		"redrive_date":              "2024-06-15 11:00:00",
	}
	cfg := detailConfigForType("sfn_executions")
	m := newDetailModel(res, "sfn_executions", cfg)

	view := m.View()
	for _, expected := range []string{
		"ExecutionArn",
		"Name",
		"Status",
		"StartDate",
		"StopDate",
		"StateMachineArn",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("SFNExecutions detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_SFNExecutions_NilFields(t *testing.T) {
	ensureNoColor(t)
	item := sfntypes.ExecutionListItem{}
	res := buildResource("empty-sfn-execution", "empty-sfn-execution", item)
	cfg := detailConfigForType("sfn_executions")
	m := newDetailModel(res, "sfn_executions", cfg)

	view := m.View()
	if view == "" {
		t.Error("SFNExecutions detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// SFN Execution History detail view tests (Level 2 child of SFN Executions)
// ===========================================================================

func TestQA_Detail_SFNExecutionHistory_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	event := sfntypes.HistoryEvent{
		Id:        1,
		Timestamp: &ts,
		Type:      sfntypes.HistoryEventTypeTaskStateEntered,
		StateEnteredEventDetails: &sfntypes.StateEnteredEventDetails{
			Name:  ptrString("ProcessOrder"),
			Input: ptrString(`{"orderId":"12345"}`),
		},
	}
	res := buildResource("1", "Task State Entered", event)
	res.Status = "pending"
	res.Fields = map[string]string{
		"timestamp":         "2024-06-15 10:00:00",
		"event_type":        "TaskStateEntered",
		"event_type_short":  "Task State Entered",
		"state_name":        "ProcessOrder",
		"event_detail":      `{"orderId":"12345"}`,
		"event_id":          "1",
		"previous_event_id": "0",
	}
	cfg := detailConfigForType("sfn_execution_history")
	m := newDetailModel(res, "sfn_execution_history", cfg)

	view := m.View()
	for _, expected := range []string{
		"Timestamp",
		"Type",
		"Id",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("SFNExecutionHistory detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_SFNExecutionHistory_FailedEvent(t *testing.T) {
	ensureNoColor(t)
	ts := time.Date(2024, 6, 15, 10, 0, 5, 0, time.UTC)

	event := sfntypes.HistoryEvent{
		Id:              5,
		PreviousEventId: 4,
		Timestamp:       &ts,
		Type:            sfntypes.HistoryEventTypeTaskFailed,
		TaskFailedEventDetails: &sfntypes.TaskFailedEventDetails{
			Resource:     ptrString("lambda:invoke"),
			ResourceType: ptrString("lambda"),
			Error:        ptrString("States.TaskFailed"),
			Cause:        ptrString("Lambda function returned error"),
		},
	}
	res := buildResource("5", "Task Failed", event)
	res.Status = "failed"
	res.Fields = map[string]string{
		"timestamp":         "2024-06-15 10:00:05",
		"event_type":        "TaskFailed",
		"event_type_short":  "Task Failed",
		"state_name":        "ProcessOrder",
		"event_detail":      "States.TaskFailed: Lambda function returned error",
		"event_id":          "5",
		"previous_event_id": "4",
	}
	cfg := detailConfigForType("sfn_execution_history")
	m := newDetailModel(res, "sfn_execution_history", cfg)

	view := m.View()
	// The detail view should render key fields from the TaskFailedEventDetails struct
	for _, expected := range []string{
		"TaskFailedEventDetails",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("SFNExecutionHistory failed event detail should contain %q, got:\n%s",
				expected, view)
		}
	}
}

func TestQA_Detail_SFNExecutionHistory_NilFields(t *testing.T) {
	ensureNoColor(t)
	event := sfntypes.HistoryEvent{}
	res := buildResource("empty-history", "empty-history", event)
	cfg := detailConfigForType("sfn_execution_history")
	m := newDetailModel(res, "sfn_execution_history", cfg)

	view := m.View()
	if view == "" {
		t.Error("SFNExecutionHistory detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// CodeBuild Builds detail view tests (child of CodeBuild Projects)
// ===========================================================================

func TestQA_Detail_CBBuilds_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	endTs := time.Date(2024, 6, 15, 10, 4, 12, 0, time.UTC)

	build := cbtypes.Build{
		Id:                    ptrString("my-project:build-id-001"),
		Arn:                   ptrString("arn:aws:codebuild:us-east-1:123456789012:build/my-project:build-id-001"),
		BuildNumber:           ptrInt64(142),
		BuildStatus:           cbtypes.StatusTypeSucceeded,
		StartTime:             &startTs,
		EndTime:               &endTs,
		CurrentPhase:          ptrString("COMPLETED"),
		SourceVersion:         ptrString("abc123def456789012345678901234567890abcd"),
		ResolvedSourceVersion: ptrString("abc123def456789012345678901234567890abcd"),
		Initiator:             ptrString("codepipeline/my-pipeline"),
		ProjectName:           ptrString("my-project"),
		Logs: &cbtypes.LogsLocation{
			GroupName:  ptrString("/aws/codebuild/my-project"),
			StreamName: ptrString("build-id-001"),
		},
	}
	res := buildResource("my-project:build-id-001", "#142", build)
	res.Status = "SUCCEEDED"
	res.Fields = map[string]string{
		"build_number":            "142",
		"build_status":            "SUCCEEDED",
		"start_time":              "2024-06-15 10:00:00",
		"end_time":                "2024-06-15 10:04:12",
		"duration":                "4m 12s",
		"source_version_short":    "abc123de",
		"initiator":               "codepipeline/my-pipeline",
		"build_id":                "my-project:build-id-001",
		"build_arn":               "arn:aws:codebuild:us-east-1:123456789012:build/my-project:build-id-001",
		"current_phase":           "COMPLETED",
		"source_version":          "abc123def456789012345678901234567890abcd",
		"resolved_source_version": "abc123def456789012345678901234567890abcd",
		"log_group_name":          "/aws/codebuild/my-project",
		"log_stream_name":         "build-id-001",
	}
	cfg := detailConfigForType("cb_builds")
	m := newDetailModel(res, "cb_builds", cfg)

	view := m.View()
	for _, expected := range []string{
		"BuildStatus",
		"StartTime",
		"Arn",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("CBBuilds detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_CBBuilds_NilFields(t *testing.T) {
	ensureNoColor(t)
	build := cbtypes.Build{}
	res := buildResource("empty-cb-build", "empty-cb-build", build)
	cfg := detailConfigForType("cb_builds")
	m := newDetailModel(res, "cb_builds", cfg)

	view := m.View()
	if view == "" {
		t.Error("CBBuilds detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// CodeBuild Build Logs detail view tests (Level 2 child of CodeBuild Builds)
// ===========================================================================

func TestQA_Detail_CBBuildLogs_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.OutputLogEvent{
		Timestamp:     ptrInt64(1718445600000),
		Message:       ptrString("[Container] Running command echo hello"),
		IngestionTime: ptrInt64(1718445601000),
	}
	res := buildResource(
		"evt-1718445600000-0",
		"[Container] Running command echo hello",
		ev,
	)
	res.Status = "IN_PROGRESS"
	res.Fields = map[string]string{
		"timestamp":      "2024-06-15 10:00:00",
		"message":        "[Container] Running command echo hello",
		"ingestion_time": "2024-06-15 10:00:01",
		"event_id":       "evt-1718445600000-0",
	}
	cfg := detailConfigForType("cb_build_logs")
	m := newDetailModel(res, "cb_build_logs", cfg)

	view := m.View()
	for _, expected := range []string{
		"Timestamp",
		"Message",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("CBBuildLogs detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_CBBuildLogs_NilFields(t *testing.T) {
	ensureNoColor(t)
	ev := cwlogstypes.OutputLogEvent{}
	res := buildResource("empty-cb-log", "empty-cb-log", ev)
	cfg := detailConfigForType("cb_build_logs")
	m := newDetailModel(res, "cb_build_logs", cfg)

	view := m.View()
	if view == "" {
		t.Error("CBBuildLogs detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// ECR Images detail view tests
// ===========================================================================

func TestQA_Detail_ECRImages_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	pushedAt := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

	img := ecrtypes.ImageDetail{
		ImageDigest:      ptrString("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"),
		ImageTags:        []string{"latest", "v1.0.0"},
		ImagePushedAt:    &pushedAt,
		ImageSizeInBytes: ptrInt64(52428800),
		ImageScanStatus: &ecrtypes.ImageScanStatus{
			Status: ecrtypes.ScanStatusComplete,
		},
		ImageScanFindingsSummary: &ecrtypes.ImageScanFindingsSummary{
			FindingSeverityCounts: map[string]int32{
				"HIGH":   3,
				"MEDIUM": 5,
			},
		},
	}
	res := buildResource("sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", "latest, v1.0.0", img)
	res.Status = ""
	res.Fields = map[string]string{
		"image_tags":     "latest, v1.0.0",
		"digest_short":   "abcdef123456",
		"pushed_at":      "2024-06-15 10:00:00",
		"image_size":     "50.0 MB",
		"scan_status":    "COMPLETE",
		"finding_counts": "3H 5M",
		"image_uri":      "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo:latest",
	}
	cfg := detailConfigForType("ecr_images")
	m := newDetailModel(res, "ecr_images", cfg)

	view := m.View()
	for _, expected := range []string{
		"ImageDigest",
		"ImageTags",
		"ImagePushedAt",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("ECRImages detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_ECRImages_NilFields(t *testing.T) {
	ensureNoColor(t)
	img := ecrtypes.ImageDetail{}
	res := buildResource("empty-ecr-image", "empty-ecr-image", img)
	cfg := detailConfigForType("ecr_images")
	m := newDetailModel(res, "ecr_images", cfg)

	view := m.View()
	if view == "" {
		t.Error("ECRImages detail should not be empty even with nil fields")
	}
}

// ===========================================================================
// Pipeline Stages detail view tests (child of CodePipelines)
// ===========================================================================

func TestQA_Detail_PipelineStages_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)

	row := awsclient.PipelineStageRow{
		StageName:       "Source",
		StageStatus:     "Succeeded",
		ActionName:      "GitHub",
		ActionStatus:    "Succeeded",
		LastStatusChange: ptrTime(time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)),
		ExternalURL:     "https://github.com/org/repo/commit/abc123",
		Token:           "approval-token-xyz",
		ErrorCode:       "",
		ErrorMessage:    "",
		RevisionId:      "abc123def456",
		RevisionSummary: "commit-sha-abc",
	}
	res := buildResource("Source/GitHub", "GitHub", row)
	res.Status = "running"
	res.Fields = map[string]string{
		"stage_name":           "Source",
		"stage_status":         "Succeeded",
		"action_name":          "GitHub",
		"action_status":        "Succeeded",
		"last_change_time":     "2024-06-15 10:00:00",
		"external_url":         "https://github.com/org/repo/commit/abc123",
		"action_token":         "approval-token-xyz",
		"action_error_details": "",
		"revision_id":          "abc123def456",
		"revision_summary":     "commit-sha-abc",
	}
	cfg := detailConfigForType("pipeline_stages")
	m := newDetailModel(res, "pipeline_stages", cfg)

	view := m.View()
	for _, expected := range []string{
		"StageName",
		"ActionName",
		"ActionStatus",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("PipelineStages detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_PipelineStages_NilFields(t *testing.T) {
	ensureNoColor(t)
	row := awsclient.PipelineStageRow{}
	res := buildResource("empty-pipeline-stage", "empty-pipeline-stage", row)
	cfg := detailConfigForType("pipeline_stages")
	m := newDetailModel(res, "pipeline_stages", cfg)

	view := m.View()
	if view == "" {
		t.Error("PipelineStages detail should not be empty even with nil fields")
	}
}
