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
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// TestQA_ListRawStruct_LogStreams: verify list rendering with LogStream RawStruct
// ===========================================================================

func TestQA_ListRawStruct_LogStreams(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("log_streams")
	if typeDef == nil {
		t.Fatal("log_streams child resource type not registered")
	}

	cfg := configForType("log_streams")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ls := cwlogstypes.LogStream{
		LogStreamName:       ptrString("2024/03/22/[$LATEST]abcdef1234567890"),
		FirstEventTimestamp: ptrInt64(1711065600000),
		LastEventTimestamp:  ptrInt64(1711152000000),
	}

	resources := []resource.Resource{
		{
			ID:     "2024/03/22/[$LATEST]abcdef1234567890",
			Name:   "2024/03/22/[$LATEST]abcdef1234567890",
			Status: "",
			Fields: map[string]string{
				"stream_name": "2024/03/22/[$LATEST]abcdef1234567890",
				"last_event":  "2024-03-23 00:00",
				"first_event": "2024-03-22 00:00",
			},
			RawStruct: ls,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "abcdef1234567890") {
		t.Errorf("log_streams list should contain stream name, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_LogEvents: verify list rendering with OutputLogEvent RawStruct
// ===========================================================================

func TestQA_ListRawStruct_LogEvents(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("log_events")
	if typeDef == nil {
		t.Fatal("log_events child resource type not registered")
	}

	cfg := configForType("log_events")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ev := cwlogstypes.OutputLogEvent{
		Timestamp:     ptrInt64(1711065600000),
		Message:       ptrString("ERROR Failed to connect to database"),
		IngestionTime: ptrInt64(1711065601000),
	}

	resources := []resource.Resource{
		{
			ID:     "evt-1711065600000-0",
			Name:   "ERROR Failed to connect to database",
			Status: "ERROR",
			Fields: map[string]string{
				"timestamp":      "2024-03-22 00:00",
				"message":        "ERROR Failed to connect to database",
				"ingestion_time": "2024-03-22 00:00",
			},
			RawStruct: ev,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "Failed to connect") {
		t.Errorf("log_events list should contain event message, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_TargetHealth: verify list rendering with TargetHealthDescription RawStruct
// ===========================================================================

func TestQA_ListRawStruct_TargetHealth(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("tg_health")
	if typeDef == nil {
		t.Fatal("tg_health child resource type not registered")
	}

	cfg := configForType("tg_health")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

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
	}

	resources := []resource.Resource{
		{
			ID:     "i-0abc1234def56789a",
			Name:   "i-0abc1234def56789a",
			Status: "unhealthy",
			Fields: map[string]string{
				"target_id":   "i-0abc1234def56789a",
				"port":        "8080",
				"az":          "us-east-1a",
				"health":      "unhealthy",
				"reason":      "Target.FailedHealthChecks",
				"description": "Health checks failed with 503",
			},
			RawStruct: thd,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "i-0abc1234def56789a") {
		t.Errorf("tg_health list should contain target ID, got:\n%s", view)
	}
}

// ===========================================================================
// TestLambdaInvocationsListRawStruct: verify list rendering with FilteredLogEvent RawStruct
// ===========================================================================

func TestLambdaInvocationsListRawStruct(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("lambda_invocations")
	if typeDef == nil {
		t.Fatal("lambda_invocations child resource type not registered")
	}

	cfg := configForType("lambda_invocations")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ev := cwlogstypes.FilteredLogEvent{
		Timestamp:     ptrInt64(1711065600000),
		Message:       ptrString("REPORT RequestId: 12345678-1234-1234-1234-123456789012\tDuration: 2103.45 ms\tBilled Duration: 2200 ms\tMemory Size: 256 MB\tMax Memory Used: 128 MB\t"),
		IngestionTime: ptrInt64(1711065601000),
		EventId:       ptrString("evt-001"),
	}

	resources := []resource.Resource{
		{
			ID:     "12345678-1234-1234-1234-123456789012",
			Name:   "12345678-1234-1234-1234-123456789012",
			Status: "OK",
			Fields: map[string]string{
				"request_id":  "12345678-1234-1234-1234-123456789012",
				"timestamp":   "2024-03-22 00:00",
				"status":      "OK",
				"duration_ms": "2103 ms",
				"memory_used": "128/256 MB",
				"cold_start":  "no",
			},
			RawStruct: ev,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "12345678") {
		t.Errorf("lambda_invocations list should contain request ID, got:\n%s", view)
	}
}

// ===========================================================================
// TestLambdaInvocationLogsListRawStruct: verify list rendering with FilteredLogEvent RawStruct
// ===========================================================================

func TestLambdaInvocationLogsListRawStruct(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("lambda_invocation_logs")
	if typeDef == nil {
		t.Fatal("lambda_invocation_logs child resource type not registered")
	}

	cfg := configForType("lambda_invocation_logs")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ev := cwlogstypes.FilteredLogEvent{
		Timestamp:     ptrInt64(1711065600000),
		Message:       ptrString("INFO Processing request for user abc-123"),
		IngestionTime: ptrInt64(1711065600500),
		EventId:       ptrString("log-002"),
	}

	resources := []resource.Resource{
		{
			ID:     "log-002",
			Name:   "INFO Processing request for user abc-123",
			Status: "",
			Fields: map[string]string{
				"timestamp": "2024-03-22 00:00",
				"message":   "INFO Processing request for user abc-123",
			},
			RawStruct: ev,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "Processing request") {
		t.Errorf("lambda_invocation_logs list should contain log message, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_EcsSvcEvents: verify list rendering with ServiceEvent RawStruct
// ===========================================================================

func TestQA_ListRawStruct_EcsSvcEvents(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("ecs_svc_events")
	if typeDef == nil {
		t.Fatal("ecs_svc_events child resource type not registered")
	}

	cfg := configForType("ecs_svc_events")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	ev := ecstypes.ServiceEvent{
		Id:        ptrString("evt-list-001"),
		CreatedAt: &ts,
		Message:   ptrString("(service web-service) has reached a steady state."),
	}

	resources := []resource.Resource{
		{
			ID:     "evt-list-001",
			Name:   "(service web-service) has reached a steady state.",
			Status: "",
			Fields: map[string]string{
				"timestamp": "2024-03-22 10:00",
				"message":   "(service web-service) has reached a steady state.",
			},
			RawStruct: ev,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "steady state") {
		t.Errorf("ecs_svc_events list should contain event message, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_EcsSvcTasks: verify list rendering with Task RawStruct
// ===========================================================================

func TestQA_ListRawStruct_EcsSvcTasks(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("ecs_tasks")
	if typeDef == nil {
		t.Fatal("ecs_tasks child resource type not registered")
	}

	cfg := configForType("ecs_tasks")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	startedAt := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	task := ecstypes.Task{
		TaskArn:           ptrString("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456"),
		LastStatus:        ptrString("RUNNING"),
		DesiredStatus:     ptrString("RUNNING"),
		HealthStatus:      ecstypes.HealthStatusHealthy,
		TaskDefinitionArn: ptrString("arn:aws:ecs:us-east-1:123456789012:task-definition/web-app:5"),
		StartedAt:         &startedAt,
	}

	resources := []resource.Resource{
		{
			ID:     "abc123def456",
			Name:   "abc123def456",
			Status: "RUNNING",
			Fields: map[string]string{
				"task_id_short":  "abc123def456",
				"status":         "RUNNING",
				"health":         "HEALTHY",
				"task_def_short": "web-app:5",
				"started_at":     "2024-03-22 10:00",
				"stopped_reason": "",
			},
			RawStruct: task,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "abc123def456") {
		t.Errorf("ecs_tasks list should contain task ID, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_EcsSvcLogs: verify list rendering with FilteredLogEvent RawStruct
// ===========================================================================

func TestQA_ListRawStruct_EcsSvcLogs(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("ecs_svc_logs")
	if typeDef == nil {
		t.Fatal("ecs_svc_logs child resource type not registered")
	}

	cfg := configForType("ecs_svc_logs")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ev := cwlogstypes.FilteredLogEvent{
		Timestamp:     ptrInt64(1711036800000),
		Message:       ptrString("INFO Starting application server on port 8080"),
		IngestionTime: ptrInt64(1711036801000),
		LogStreamName: ptrString("ecs/web/abc123def456"),
		EventId:       ptrString("evt-svc-log-list"),
	}

	resources := []resource.Resource{
		{
			ID:     "evt-svc-log-list",
			Name:   "INFO Starting application server on port 8080",
			Status: "",
			Fields: map[string]string{
				"timestamp":    "2024-03-21 16:00",
				"stream_short": "web/abc123de",
				"message":      "INFO Starting application server on port 8080",
			},
			RawStruct: ev,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "Starting application") {
		t.Errorf("ecs_svc_logs list should contain log message, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_CfnEvents: verify list rendering with StackEvent RawStruct
// ===========================================================================

func TestQA_ListRawStruct_CfnEvents(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("cfn_events")
	if typeDef == nil {
		t.Fatal("cfn_events child resource type not registered")
	}

	cfg := configForType("cfn_events")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	ev := cfntypes.StackEvent{
		EventId:              ptrString("evt-list-cfn-001"),
		StackId:              ptrString("arn:aws:cloudformation:us-east-1:123456789012:stack/my-stack/guid1"),
		StackName:            ptrString("my-stack"),
		Timestamp:            &ts,
		LogicalResourceId:    ptrString("MyBucket"),
		ResourceType:         ptrString("AWS::S3::Bucket"),
		ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
		ResourceStatusReason: ptrString("Resource creation complete"),
	}

	resources := []resource.Resource{
		{
			ID:     "evt-list-cfn-001",
			Name:   "2024-03-22 10:00:00",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"timestamp":              "2024-03-22 10:00:00",
				"logical_resource_id":    "MyBucket",
				"resource_type":          "AWS::S3::Bucket",
				"resource_status":        "CREATE_COMPLETE",
				"resource_status_reason": "Resource creation complete",
			},
			RawStruct: ev,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "MyBucket") {
		t.Errorf("cfn_events list should contain logical resource id, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_CfnResources: verify list rendering with StackResourceSummary RawStruct
// ===========================================================================

func TestQA_ListRawStruct_CfnResources(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("cfn_resources")
	if typeDef == nil {
		t.Fatal("cfn_resources child resource type not registered")
	}

	cfg := configForType("cfn_resources")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	summary := cfntypes.StackResourceSummary{
		LogicalResourceId:    ptrString("MyBucket"),
		PhysicalResourceId:   ptrString("my-stack-mybucket-abc123"),
		ResourceType:         ptrString("AWS::S3::Bucket"),
		ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
		LastUpdatedTimestamp: &ts,
		DriftInformation: &cfntypes.StackResourceDriftInformationSummary{
			StackResourceDriftStatus: cfntypes.StackResourceDriftStatusInSync,
		},
	}

	resources := []resource.Resource{
		{
			ID:     "MyBucket",
			Name:   "MyBucket",
			Status: "CREATE_COMPLETE",
			Fields: map[string]string{
				"logical_resource_id":  "MyBucket",
				"physical_resource_id": "my-stack-mybucket-abc123",
				"resource_type":        "AWS::S3::Bucket",
				"resource_status":      "CREATE_COMPLETE",
				"drift_status":         "IN_SYNC",
				"last_updated":         "2024-03-22 10:00:00",
			},
			RawStruct: summary,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "MyBucket") {
		t.Errorf("cfn_resources list should contain logical resource id, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_AsgActivities: verify list rendering with Activity RawStruct
// ===========================================================================

func TestQA_ListRawStruct_AsgActivities(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("asg_activities")
	if typeDef == nil {
		t.Fatal("asg_activities child resource type not registered")
	}

	cfg := configForType("asg_activities")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	activity := asgtypes.Activity{
		ActivityId:           ptrString("act-list-001"),
		AutoScalingGroupName: ptrString("my-asg"),
		Cause:                ptrString("At 2024-03-22T10:00:00Z an instance was started"),
		Description:          ptrString("Launching a new EC2 instance: i-0abc1234"),
		StartTime:            &ts,
		StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
	}

	resources := []resource.Resource{
		{
			ID:     "act-list-001",
			Name:   "2024-03-22 10:00:00",
			Status: "Successful",
			Fields: map[string]string{
				"start_time":  "2024-03-22 10:00:00",
				"status_code": "Successful",
				"description": "Launching a new EC2 instance: i-0abc1234",
				"cause":       "At 2024-03-22T10:00:00Z an instance was started",
			},
			RawStruct: activity,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "Successful") {
		t.Errorf("asg_activities list should contain status code, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_AlarmHistory: verify list rendering with AlarmHistoryItem RawStruct
// ===========================================================================

func TestQA_ListRawStruct_AlarmHistory(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("alarm_history")
	if typeDef == nil {
		t.Fatal("alarm_history child resource type not registered")
	}

	cfg := configForType("alarm_history")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	item := cwtypes.AlarmHistoryItem{
		AlarmName:       ptrString("HighCPUAlarm"),
		AlarmType:       cwtypes.AlarmTypeMetricAlarm,
		HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
		HistorySummary:  ptrString("Alarm updated from OK to ALARM"),
		Timestamp:       &ts,
	}

	resources := []resource.Resource{
		{
			ID:     "2024-03-22 10:00:00",
			Name:   "2024-03-22 10:00:00",
			Status: "StateUpdate",
			Fields: map[string]string{
				"timestamp":         "2024-03-22 10:00:00",
				"history_item_type": "StateUpdate",
				"history_summary":   "Alarm updated from OK to ALARM",
			},
			RawStruct: item,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "StateUpdate") {
		t.Errorf("alarm_history list should contain history item type, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_ELBListeners: verify list rendering with Listener RawStruct
// ===========================================================================

func TestQA_ListRawStruct_ELBListeners(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("elb_listeners")
	if typeDef == nil {
		t.Fatal("elb_listeners child resource type not registered")
	}

	cfg := configForType("elb_listeners")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

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

	resources := []resource.Resource{
		{
			ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456",
			Name:   "443",
			Status: "",
			Fields: map[string]string{
				"port":                  "443",
				"protocol":              "HTTPS",
				"default_action_type":   "forward",
				"default_action_target": "api-prod-tg",
				"ssl_policy":            "ELBSecurityPolicy-TLS13-1-2-2021-06",
				"certificate_short":     "abc-def-123",
			},
			RawStruct: listener,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "443") {
		t.Errorf("elb_listeners list should contain port 443, got:\n%s", view)
	}
	if !strings.Contains(view, "HTTPS") {
		t.Errorf("elb_listeners list should contain protocol HTTPS, got:\n%s", view)
	}
	if !strings.Contains(view, "forward") {
		t.Errorf("elb_listeners list should contain action type 'forward', got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_CBBuilds: verify list rendering with Build RawStruct
// ===========================================================================

func TestQA_ListRawStruct_CBBuilds(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("cb_builds")
	if typeDef == nil {
		t.Fatal("cb_builds child resource type not registered")
	}

	cfg := configForType("cb_builds")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	startTs := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	endTs := time.Date(2024, 6, 15, 10, 4, 12, 0, time.UTC)
	build := cbtypes.Build{
		Id:            ptrString("my-project:build-id-001"),
		Arn:           ptrString("arn:aws:codebuild:us-east-1:123456789012:build/my-project:build-id-001"),
		BuildNumber:   ptrInt64(142),
		BuildStatus:   cbtypes.StatusTypeSucceeded,
		StartTime:     &startTs,
		EndTime:       &endTs,
		CurrentPhase:  ptrString("COMPLETED"),
		SourceVersion: ptrString("abc123de"),
		Initiator:     ptrString("codepipeline/my-pipeline"),
		Logs: &cbtypes.LogsLocation{
			GroupName:  ptrString("/aws/codebuild/my-project"),
			StreamName: ptrString("build-id-001"),
		},
	}

	resources := []resource.Resource{
		{
			ID:     "my-project:build-id-001",
			Name:   "#142",
			Status: "SUCCEEDED",
			Fields: map[string]string{
				"build_number":         "142",
				"build_status":         "SUCCEEDED",
				"start_time":           "2024-06-15 10:00:00",
				"duration":             "4m 12s",
				"source_version_short": "abc123de",
				"initiator":            "codepipeline/my-pipeline",
			},
			RawStruct: build,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "SUCCEEDED") {
		t.Errorf("cb_builds list should contain build status, got:\n%s", view)
	}
	if !strings.Contains(view, "142") {
		t.Errorf("cb_builds list should contain build number, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_CBBuildLogs: verify list rendering with OutputLogEvent RawStruct
// ===========================================================================

func TestQA_ListRawStruct_CBBuildLogs(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("cb_build_logs")
	if typeDef == nil {
		t.Fatal("cb_build_logs child resource type not registered")
	}

	cfg := configForType("cb_build_logs")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ev := cwlogstypes.OutputLogEvent{
		Timestamp:     ptrInt64(1718445600000),
		Message:       ptrString("[Container] Running command echo hello"),
		IngestionTime: ptrInt64(1718445601000),
	}

	resources := []resource.Resource{
		{
			ID:     "evt-1718445600000-0",
			Name:   "[Container] Running command echo hello",
			Status: "IN_PROGRESS",
			Fields: map[string]string{
				"timestamp":      "2024-06-15 10:00:00",
				"message":        "[Container] Running command echo hello",
				"ingestion_time": "2024-06-15 10:00:01",
				"event_id":       "evt-1718445600000-0",
			},
			RawStruct: ev,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "Running command") {
		t.Errorf("cb_build_logs list should contain message text, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_ECRImages: verify list rendering with ImageDetail RawStruct
// ===========================================================================

func TestQA_ListRawStruct_ECRImages(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("ecr_images")
	if typeDef == nil {
		t.Fatal("ecr_images child resource type not registered")
	}

	cfg := configForType("ecr_images")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

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

	resources := []resource.Resource{
		{
			ID:     "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			Name:   "latest, v1.0.0",
			Status: "",
			Fields: map[string]string{
				"image_tags":     "latest, v1.0.0",
				"digest_short":   "abcdef123456",
				"pushed_at":      "2024-06-15 10:00:00",
				"image_size":     "50.0 MB",
				"scan_status":    "COMPLETE",
				"finding_counts": "3H 5M",
			},
			RawStruct: img,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "latest") {
		t.Errorf("ecr_images list should contain image tag, got:\n%s", view)
	}
	if !strings.Contains(view, "abcdef123456") {
		t.Errorf("ecr_images list should contain digest short, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_PipelineStages: verify list rendering with PipelineStageRow RawStruct
// ===========================================================================

func TestQA_ListRawStruct_PipelineStages(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("pipeline_stages")
	if typeDef == nil {
		t.Fatal("pipeline_stages child resource type not registered")
	}

	cfg := configForType("pipeline_stages")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	row := awsclient.PipelineStageRow{
		StageName:    "Source",
		StageStatus:  "Succeeded",
		ActionName:   "GitHub",
		ActionStatus: "Succeeded",
		ExternalURL:  "https://github.com/org/repo/commit/abc123",
	}

	resources := []resource.Resource{
		{
			ID:     "Source/GitHub",
			Name:   "GitHub",
			Status: "running",
			Fields: map[string]string{
				"stage_name":       "Source",
				"stage_status":     "Succeeded",
				"action_name":      "GitHub",
				"action_status":    "Succeeded",
				"last_change_time": "2024-06-15 10:00:00",
				"external_url":     "https://github.com/org/repo/commit/abc123",
			},
			RawStruct: row,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "Source") {
		t.Errorf("pipeline_stages list should contain stage name, got:\n%s", view)
	}
	if !strings.Contains(view, "GitHub") {
		t.Errorf("pipeline_stages list should contain action name, got:\n%s", view)
	}
	if !strings.Contains(view, "Succeeded") {
		t.Errorf("pipeline_stages list should contain action status, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_RolePolicies: verify list rendering with RolePolicyRow RawStruct
// ===========================================================================

func TestQA_ListRawStruct_RolePolicies(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("role_policies")
	if typeDef == nil {
		t.Fatal("role_policies child resource type not registered")
	}

	cfg := configForType("role_policies")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	row := awsclient.RolePolicyRow{
		PolicyName: "ReadOnlyAccess",
		PolicyArn:  "arn:aws:iam::aws:policy/ReadOnlyAccess",
		PolicyType: "Managed",
	}

	resources := []resource.Resource{
		{
			ID:     "arn:aws:iam::aws:policy/ReadOnlyAccess",
			Name:   "ReadOnlyAccess",
			Status: "",
			Fields: map[string]string{
				"policy_name": "ReadOnlyAccess",
				"policy_arn":  "arn:aws:iam::aws:policy/ReadOnlyAccess",
				"policy_type": "Managed",
			},
			RawStruct: row,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "ReadOnlyAccess") {
		t.Errorf("role_policies list should contain policy name, got:\n%s", view)
	}
	if !strings.Contains(view, "Managed") {
		t.Errorf("role_policies list should contain policy type, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_ELBListenerRules: verify list rendering with Rule RawStruct
// ===========================================================================

func TestQA_ListRawStruct_ELBListenerRules(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("elb_listener_rules")
	if typeDef == nil {
		t.Fatal("elb_listener_rules child resource type not registered")
	}

	cfg := configForType("elb_listener_rules")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	rule := elbtypes.Rule{
		RuleArn:  ptrString("arn:rule/1"),
		Priority: ptrString("100"),
	}

	resources := []resource.Resource{
		{
			ID:     "arn:rule/1",
			Name:   "100",
			Status: "",
			Fields: map[string]string{
				"priority":           "100",
				"conditions_summary": "path: /api/*",
				"action_type":        "forward",
				"action_target":      "api-tg",
			},
			RawStruct: rule,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "100") {
		t.Errorf("elb_listener_rules list should contain priority, got:\n%s", view)
	}
	if !strings.Contains(view, "forward") {
		t.Errorf("elb_listener_rules list should contain action type, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_DbiEvents: verify list rendering with rdstypes.Event RawStruct
// ===========================================================================

func TestQA_ListRawStruct_DbiEvents(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("dbi_events")
	if typeDef == nil {
		t.Fatal("dbi_events child resource type not registered")
	}

	cfg := configForType("dbi_events")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	ev := rdstypes.Event{
		Date:             &ts,
		EventCategories:  []string{"maintenance"},
		Message:          ptrString("Applying offline patches to DB instance"),
		SourceIdentifier: ptrString("my-db-instance"),
		SourceType:       rdstypes.SourceTypeDbInstance,
		SourceArn:        ptrString("arn:aws:rds:us-east-1:123456789012:db:my-db-instance"),
	}

	resources := []resource.Resource{
		{
			ID:   "2024-06-15 10:00:00/my-db-instance",
			Name: "2024-06-15 10:00:00",
			Fields: map[string]string{
				"timestamp":         "2024-06-15 10:00:00",
				"event_categories":  "maintenance",
				"message":           "Applying offline patches to DB instance",
				"source_identifier": "my-db-instance",
				"source_type":       "db-instance",
				"source_arn":        "arn:aws:rds:us-east-1:123456789012:db:my-db-instance",
			},
			RawStruct: ev,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "maintenance") {
		t.Errorf("dbi_events list should contain event category, got:\n%s", view)
	}
	if !strings.Contains(view, "Applying offline patches") {
		t.Errorf("dbi_events list should contain message, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_EbRuleTargets: verify list rendering with Target RawStruct
// ===========================================================================

func TestQA_ListRawStruct_EbRuleTargets(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("eb_rule_targets")
	if typeDef == nil {
		t.Fatal("eb_rule_targets child resource type not registered")
	}

	cfg := configForType("eb_rule_targets")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	target := ebtypes.Target{
		Id:      ptrString("lambda-target-1"),
		Arn:     ptrString("arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-daily"),
		RoleArn: ptrString("arn:aws:iam::123456789012:role/EventBridgeLambdaRole"),
	}

	resources := []resource.Resource{
		{
			ID:     "lambda-target-1",
			Name:   "lambda-target-1",
			Status: "",
			Fields: map[string]string{
				"target_id":          "lambda-target-1",
				"target_arn":         "arn:aws:lambda:us-east-1:123456789012:function:data-pipeline-daily",
				"resource_type_name": "Lambda: data-pipeline-daily",
				"input_summary":      "\u2014",
			},
			RawStruct: target,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "lambda-target-1") {
		t.Errorf("eb_rule_targets list should contain target ID, got:\n%s", view)
	}
	if !strings.Contains(view, "Lambda") {
		t.Errorf("eb_rule_targets list should contain resource type name, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_GlueRuns: verify list rendering with JobRun RawStruct
// ===========================================================================

func TestQA_ListRawStruct_GlueRuns(t *testing.T) {
	ensureNoColor(t)

	typeDef := resource.GetChildType("glue_runs")
	if typeDef == nil {
		t.Fatal("glue_runs child resource type not registered")
	}

	cfg := configForType("glue_runs")
	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	startTs := time.Date(2024, 8, 10, 14, 30, 0, 0, time.UTC)
	dpuSec := 45000.0

	run := gluetypes.JobRun{
		Id:            ptrString("jr_abc12345-6789-0abc-def0-123456789012"),
		JobName:       ptrString("etl-daily-load"),
		JobRunState:   gluetypes.JobRunStateSucceeded,
		StartedOn:     &startTs,
		ExecutionTime: 2843,
		DPUSeconds:    &dpuSec,
	}

	resources := []resource.Resource{
		{
			ID:     "jr_abc12345-6789-0abc-def0-123456789012",
			Name:   "2024-08-10 14:30:00",
			Status: "SUCCEEDED",
			Fields: map[string]string{
				"run_id_short":         "jr_abc12",
				"job_run_state":        "SUCCEEDED",
				"started_on":           "2024-08-10 14:30:00",
				"execution_time_human": "47m 23s",
				"error_message":        "",
				"dpu_hours":            "12.5",
				"run_id":               "jr_abc12345-6789-0abc-def0-123456789012",
				"job_name":             "etl-daily-load",
			},
			RawStruct: run,
		},
	}

	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})
	view := stripAnsi(m.View())

	if !strings.Contains(view, "jr_abc12") {
		t.Errorf("glue_runs list should contain run_id_short, got:\n%s", view)
	}
	if !strings.Contains(view, "SUCCEEDED") {
		t.Errorf("glue_runs list should contain state, got:\n%s", view)
	}
}

// Compile-time type assertion for the new child view types
var (
	_ asgtypes.Activity                = asgtypes.Activity{}
	_ cwtypes.AlarmHistoryItem         = cwtypes.AlarmHistoryItem{}
	_ cwlogstypes.LogStream            = cwlogstypes.LogStream{}
	_ cwlogstypes.OutputLogEvent       = cwlogstypes.OutputLogEvent{}
	_ cwlogstypes.FilteredLogEvent     = cwlogstypes.FilteredLogEvent{}
	_ elbtypes.TargetHealthDescription = elbtypes.TargetHealthDescription{}
	_ ecstypes.ServiceEvent            = ecstypes.ServiceEvent{}
	_ ecstypes.Task                    = ecstypes.Task{}
	_ cfntypes.StackEvent              = cfntypes.StackEvent{}
	_ cfntypes.StackResourceSummary    = cfntypes.StackResourceSummary{}
	_ elbtypes.Listener                = elbtypes.Listener{}
	_ cbtypes.Build                    = cbtypes.Build{}
	_ ecrtypes.ImageDetail             = ecrtypes.ImageDetail{}
	_ awsclient.PipelineStageRow       = awsclient.PipelineStageRow{}
	_ awsclient.RolePolicyRow          = awsclient.RolePolicyRow{}
	_ elbtypes.Rule                    = elbtypes.Rule{}
	_ iamtypes.User                    = iamtypes.User{}
	_ rdstypes.Event                   = rdstypes.Event{}
	_ ebtypes.Target                   = ebtypes.Target{}
	_ gluetypes.JobRun                 = gluetypes.JobRun{}
)
