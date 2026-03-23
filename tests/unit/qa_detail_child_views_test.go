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
