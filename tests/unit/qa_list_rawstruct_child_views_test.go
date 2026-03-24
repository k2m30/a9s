package unit_test

import (
	"strings"
	"testing"

	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

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

// Compile-time type assertion for the new child view types
var (
	_ cwlogstypes.LogStream        = cwlogstypes.LogStream{}
	_ cwlogstypes.OutputLogEvent   = cwlogstypes.OutputLogEvent{}
	_ cwlogstypes.FilteredLogEvent = cwlogstypes.FilteredLogEvent{}
	_ elbtypes.TargetHealthDescription = elbtypes.TargetHealthDescription{}
)
