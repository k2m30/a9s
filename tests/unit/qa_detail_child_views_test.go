package unit_test

import (
	"strings"
	"testing"

	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
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
