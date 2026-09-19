package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

func TestFetchEcsSvcLogs_Basic(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("web"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         "/ecs/web-service",
								"awslogs-stream-prefix": "ecs",
							},
						},
					},
				},
			},
		},
	}

	cwLogsMock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(1711036800000), // 2024-03-21 16:00:00 UTC
						Message:       aws.String("INFO Starting application server on port 8080"),
						LogStreamName: aws.String("ecs/web/abc123def456"),
						IngestionTime: aws.Int64(1711036801000),
						EventId:       aws.String("evt-svc-001"),
					},
					{
						Timestamp:     aws.Int64(1711036860000), // 2024-03-21 16:01:00 UTC
						Message:       aws.String("INFO Health check passed"),
						LogStreamName: aws.String("ecs/web/abc123def456"),
						IngestionTime: aws.Int64(1711036861000),
						EventId:       aws.String("evt-svc-002"),
					},
					{
						Timestamp:     aws.Int64(1711036920000), // 2024-03-21 16:02:00 UTC
						Message:       aws.String("ERROR Connection refused to database"),
						LogStreamName: aws.String("ecs/web/xyz789uvw012"),
						IngestionTime: aws.Int64(1711036921000),
						EventId:       aws.String("evt-svc-003"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"web-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/web-task:5",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(result.Resources))
	}

	t.Run("event_0_ID_not_empty", func(t *testing.T) {
		if result.Resources[0].ID == "" {
			t.Error("ID should not be empty")
		}
	})

	t.Run("event_0_Fields_timestamp", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["timestamp"] == "" {
			t.Error("Fields[timestamp] should not be empty")
		}
		if r.Fields["timestamp"] == "1711036800000" {
			t.Errorf("timestamp should be formatted, not raw epoch ms: %q", r.Fields["timestamp"])
		}
	})

	t.Run("event_0_Fields_timestamp_formatted", func(t *testing.T) {
		r := result.Resources[0]
		if !strings.Contains(r.Fields["timestamp"], "2024-03-21") {
			t.Errorf("timestamp should contain date '2024-03-21', got %q", r.Fields["timestamp"])
		}
	})

	t.Run("event_0_Fields_stream_short", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["stream_short"] == "" {
			t.Error("Fields[stream_short] should not be empty")
		}
	})

	t.Run("event_0_Fields_message", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["message"] == "" {
			t.Error("Fields[message] should not be empty")
		}
		if !strings.Contains(r.Fields["message"], "Starting application") {
			t.Errorf("Fields[message]: expected to contain 'Starting application', got %q", r.Fields["message"])
		}
	})

	t.Run("event_0_RawStruct", func(t *testing.T) {
		r := result.Resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(cwlogstypes.FilteredLogEvent)
		if !ok {
			t.Fatalf("RawStruct should be cwlogstypes.FilteredLogEvent, got %T", r.RawStruct)
		}
		if raw.Message == nil || !strings.Contains(*raw.Message, "Starting application") {
			t.Error("RawStruct.Message not preserved correctly")
		}
	})

	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"timestamp", "stream_short", "message"}
		for i, r := range result.Resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

func TestFetchEcsSvcLogs_NonAwslogsDriver(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("web"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverFluentd,
							Options:   map[string]string{},
						},
					},
				},
			},
		},
	}

	cwLogsMock := &mockCWLogsFilterLogEventsClient{}

	_, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"fluentd-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/fluentd-task:1",
		"",
	)
	if err == nil {
		t.Fatal("expected an error for non-awslogs driver, got nil")
	}
	if !strings.Contains(err.Error(), "awslogs") {
		t.Errorf("error should mention 'awslogs', got: %v", err)
	}
}

func TestFetchEcsSvcLogs_NoContainers(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{},
			},
		},
	}

	cwLogsMock := &mockCWLogsFilterLogEventsClient{}

	_, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"no-containers-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/empty-task:1",
		"",
	)
	if err == nil {
		t.Fatal("expected an error for empty containers, got nil")
	}
}

func TestFetchEcsSvcLogs_DescribeTaskDefinitionError(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		err: fmt.Errorf("AWS API error: task definition not found"),
	}

	cwLogsMock := &mockCWLogsFilterLogEventsClient{}

	result, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"err-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/missing-task:1",
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if result.Resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(result.Resources))
	}
}

func TestFetchEcsSvcLogs_FilterLogEventsError(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("web"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         "/ecs/web-service",
								"awslogs-stream-prefix": "ecs",
							},
						},
					},
				},
			},
		},
	}

	cwLogsMock := &mockCWLogsFilterLogEventsClient{
		err: fmt.Errorf("AWS API error: throttling exception"),
	}

	result, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"throttled-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/web-task:5",
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if result.Resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(result.Resources))
	}
}

func TestFetchEcsSvcLogs_TimestampFormatting(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("app"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         "/ecs/app",
								"awslogs-stream-prefix": "ecs",
							},
						},
					},
				},
			},
		},
	}

	cwLogsMock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(1711036800000), // 2024-03-21 16:00:00 UTC
						Message:       aws.String("test timestamp formatting"),
						LogStreamName: aws.String("ecs/app/task123"),
						EventId:       aws.String("evt-ts"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/ts-cluster",
		"ts-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/ts-task:1",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	ts := result.Resources[0].Fields["timestamp"]
	if ts == "" {
		t.Fatal("Fields[timestamp] should not be empty")
	}
	if ts == "1711036800000" {
		t.Errorf("timestamp should be formatted, not raw epoch ms: %q", ts)
	}
	if !strings.Contains(ts, "2024-03-21") {
		t.Errorf("timestamp should contain date '2024-03-21', got %q", ts)
	}
}

func TestFetchEcsSvcLogs_NewlineStripping(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("app"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         "/ecs/app",
								"awslogs-stream-prefix": "ecs",
							},
						},
					},
				},
			},
		},
	}

	cwLogsMock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(1711036800000),
						Message:       aws.String("ERROR Connection refused\nRetrying in 5 seconds\n"),
						LogStreamName: aws.String("ecs/app/task123"),
						EventId:       aws.String("evt-nl"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/nl-cluster",
		"nl-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/nl-task:1",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	msg := result.Resources[0].Fields["message"]
	if strings.Contains(msg, "\n") {
		t.Errorf("Fields[message] should not contain newlines, got %q", msg)
	}
}

func TestFetchEcsSvcLogs_NilFields(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("app"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         "/ecs/app",
								"awslogs-stream-prefix": "ecs",
							},
						},
					},
				},
			},
		},
	}

	cwLogsMock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{},
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/nil-cluster",
		"nil-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/nil-task:1",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]

	t.Run("no_panic", func(t *testing.T) {
	})

	t.Run("timestamp_empty", func(t *testing.T) {
		if r.Fields["timestamp"] != "" {
			t.Errorf("Fields[timestamp]: expected empty, got %q", r.Fields["timestamp"])
		}
	})

	t.Run("message_empty", func(t *testing.T) {
		if r.Fields["message"] != "" {
			t.Errorf("Fields[message]: expected empty, got %q", r.Fields["message"])
		}
	})

	t.Run("stream_short_empty", func(t *testing.T) {
		if r.Fields["stream_short"] != "" {
			t.Errorf("Fields[stream_short]: expected empty, got %q", r.Fields["stream_short"])
		}
	})
}

// "ecs/web/abc123def456" shortens to "web/abc123de" (container/short task id).
func TestFetchEcsSvcLogs_StreamShortComputation(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("app"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         "/ecs/app",
								"awslogs-stream-prefix": "ecs",
							},
						},
					},
				},
			},
		},
	}

	cwLogsMock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(1711036800000),
						Message:       aws.String("test stream short"),
						LogStreamName: aws.String("ecs/web/abc123def456789"),
						EventId:       aws.String("evt-ss"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/ss-cluster",
		"ss-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/ss-task:1",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	ss := result.Resources[0].Fields["stream_short"]
	if ss == "" {
		t.Fatal("Fields[stream_short] should not be empty")
	}
	if !strings.Contains(ss, "web") {
		t.Errorf("stream_short should contain container name 'web', got %q", ss)
	}
}

func TestFetchEcsSvcLogs_RawStruct(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("app"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         "/ecs/app",
								"awslogs-stream-prefix": "ecs",
							},
						},
					},
				},
			},
		},
	}

	cwLogsMock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwlogstypes.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(1711036800000),
						Message:       aws.String("raw struct test event"),
						IngestionTime: aws.Int64(1711036801000),
						LogStreamName: aws.String("ecs/app/task-raw"),
						EventId:       aws.String("evt-raw-svc"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/raw-cluster",
		"raw-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/raw-task:1",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	raw, ok := r.RawStruct.(cwlogstypes.FilteredLogEvent)
	if !ok {
		t.Fatalf("RawStruct should be cwlogstypes.FilteredLogEvent, got %T", r.RawStruct)
	}

	t.Run("Timestamp_preserved", func(t *testing.T) {
		if raw.Timestamp == nil || *raw.Timestamp != 1711036800000 {
			t.Errorf("RawStruct.Timestamp not preserved correctly")
		}
	})

	t.Run("Message_preserved", func(t *testing.T) {
		if raw.Message == nil || *raw.Message != "raw struct test event" {
			t.Errorf("RawStruct.Message not preserved correctly")
		}
	})

	t.Run("EventId_preserved", func(t *testing.T) {
		if raw.EventId == nil || *raw.EventId != "evt-raw-svc" {
			t.Errorf("RawStruct.EventId not preserved correctly")
		}
	})

	t.Run("IngestionTime_preserved", func(t *testing.T) {
		if raw.IngestionTime == nil || *raw.IngestionTime != 1711036801000 {
			t.Errorf("RawStruct.IngestionTime not preserved correctly")
		}
	})
}

func TestFetchEcsSvcLogs_Pagination(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("web"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         "/ecs/web-service",
								"awslogs-stream-prefix": "ecs",
							},
						},
					},
				},
			},
		},
	}

	page1Events := make([]cwlogstypes.FilteredLogEvent, 120)
	for i := range page1Events {
		id := fmt.Sprintf("evt-%03d", i)
		ts := int64(1711036800000 + int64(i)*1000)
		msg := fmt.Sprintf("log line %d", i)
		page1Events[i] = cwlogstypes.FilteredLogEvent{
			EventId:   &id,
			Timestamp: &ts,
			Message:   &msg,
		}
	}
	page2Events := make([]cwlogstypes.FilteredLogEvent, 120)
	for i := range page2Events {
		id := fmt.Sprintf("evt-%03d", 120+i)
		ts := int64(1711036800000 + int64(120+i)*1000)
		msg := fmt.Sprintf("log line %d", 120+i)
		page2Events[i] = cwlogstypes.FilteredLogEvent{
			EventId:   &id,
			Timestamp: &ts,
			Message:   &msg,
		}
	}

	nextToken := "page2"
	cwLogsMock := &mockCWLogsFilterLogEventsClient{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{Events: page1Events, NextToken: &nextToken},
			{Events: page2Events},
		},
	}

	results, err := awsclient.FetchEcsSvcLogs(
		context.Background(), taskDefMock, cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod",
		"web-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/web:1",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results.Resources) < 200 {
		t.Errorf("expected at least 200 results (pagination should fetch 2 pages), got %d", len(results.Resources))
	}

	if cwLogsMock.callIdx != 2 {
		t.Errorf("expected 2 FilterLogEvents calls, got %d", cwLogsMock.callIdx)
	}
}

// This scan has no FilterPattern and no start-time bound, so a page with zero
// events plus a NextToken (CloudWatch Logs' normal "keep scanning" signal) is
// expected and maxLogEvents never fires; the scan stops at maxLogScanPages
// (100) calls and reports the result as truncated.
func TestFetchEcsSvcLogs_PageCapStopsScanAndReportsTruncated(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("web"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group":         "/ecs/web-service",
								"awslogs-stream-prefix": "ecs",
							},
						},
					},
				},
			},
		},
	}

	const pageCap = 100
	outputs := make([]*cloudwatchlogs.FilterLogEventsOutput, pageCap+20)
	for i := range outputs {
		outputs[i] = &cloudwatchlogs.FilterLogEventsOutput{NextToken: aws.String(fmt.Sprintf("tok-%d", i+1))}
	}
	cwLogsMock := &mockCWLogsFilterLogEventsClient{outputs: outputs}

	results, err := awsclient.FetchEcsSvcLogs(
		context.Background(), taskDefMock, cwLogsMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod",
		"web-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/web:1",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cwLogsMock.callIdx != pageCap {
		t.Errorf("FilterLogEvents called %d times, want exactly %d (the page cap must fire, not run past it)", cwLogsMock.callIdx, pageCap)
	}
	if !results.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true — a capped scan must not be presented as a complete (empty) answer")
	}
	if len(results.Resources) != 0 {
		t.Errorf("expected 0 resources (no events matched), got %d", len(results.Resources))
	}
}

func TestFetchEcsSvcLogs_ContinuationToken(t *testing.T) {
	taskDefMock := &mockECSDescribeTaskDefinitionClient{
		output: &ecs.DescribeTaskDefinitionOutput{
			TaskDefinition: &ecstypes.TaskDefinition{
				ContainerDefinitions: []ecstypes.ContainerDefinition{
					{
						Name: aws.String("web"),
						LogConfiguration: &ecstypes.LogConfiguration{
							LogDriver: ecstypes.LogDriverAwslogs,
							Options: map[string]string{
								"awslogs-group": "/ecs/web-service",
							},
						},
					},
				},
			},
		},
	}

	wrapper := &tokenCapturingEcsSvcLogsMock{
		inner: &mockCWLogsFilterLogEventsClient{
			outputs: []*cloudwatchlogs.FilterLogEventsOutput{
				{
					Events: []cwlogstypes.FilteredLogEvent{
						{
							Timestamp:     aws.Int64(1711036800000),
							Message:       aws.String("Log from token page"),
							LogStreamName: aws.String("ecs/web/abc123def456"),
							EventId:       aws.String("evt-token-001"),
						},
					},
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcLogs(
		context.Background(),
		taskDefMock,
		wrapper,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod",
		"web-service",
		"arn:aws:ecs:us-east-1:123456789012:task-definition/web:1",
		"my-continuation-token",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	if wrapper.capturedNextToken == nil {
		t.Fatal("expected NextToken to be set in FilterLogEvents call")
	}
	if *wrapper.capturedNextToken != "my-continuation-token" {
		t.Errorf("expected NextToken %q, got %q", "my-continuation-token", *wrapper.capturedNextToken)
	}
}

type tokenCapturingEcsSvcLogsMock struct {
	inner             *mockCWLogsFilterLogEventsClient
	capturedNextToken *string
}

func (m *tokenCapturingEcsSvcLogsMock) FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	m.capturedNextToken = params.NextToken
	return m.inner.FilterLogEvents(ctx, params, optFns...)
}
