package unit_test

import (
	"strings"
	"testing"
	"time"

	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
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
		"request_id":       "12345678-1234-1234-1234-123456789012",
		"timestamp":        "2024-03-22 00:00",
		"status":           "OK",
		"duration_ms":      "2103 ms",
		"billed_duration_ms": "2200 ms",
		"memory_size_mb":   "256",
		"memory_used_mb":   "128",
		"cold_start":       "no",
		"memory_used":      "128/256 MB",
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
		"start_time":   "2024-03-22 10:00:00",
		"status_code":  "Successful",
		"description":  "Launching a new EC2 instance: i-0abc1234",
		"cause":        "At 2024-03-22T10:00:00Z an instance was started",
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
