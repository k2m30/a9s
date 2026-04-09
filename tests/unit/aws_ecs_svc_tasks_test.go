package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// ECS Service Tasks fetcher tests (child of ECS Services)
// IMPORTANT: This tests the NEW "ecs_tasks" child type, which is DIFFERENT
// from the existing "ecs-task" top-level resource type.
// ---------------------------------------------------------------------------

// TestFetchEcsSvcTasks_Basic verifies parsing of 2 running tasks from a service,
// checking all computed fields: task_id_short, status, health, task_def_short,
// started_at, stopped_reason.
func TestFetchEcsSvcTasks_Basic(t *testing.T) {
	startedAt := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	listTasksMock := &mockECSListTasksClient{
		outputs: map[string]*ecs.ListTasksOutput{
			"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster": {
				TaskArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456",
					"arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/xyz789uvw012",
				},
			},
		},
	}

	describeTasksMock := &mockECSDescribeTasksClient{
		output: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{
				{
					TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456"),
					LastStatus:        aws.String("RUNNING"),
					DesiredStatus:     aws.String("RUNNING"),
					HealthStatus:      ecstypes.HealthStatusHealthy,
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web-app:5"),
					StartedAt:         &startedAt,
					LaunchType:        ecstypes.LaunchTypeFargate,
					Cpu:               aws.String("256"),
					Memory:            aws.String("512"),
				},
				{
					TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/xyz789uvw012"),
					LastStatus:        aws.String("RUNNING"),
					DesiredStatus:     aws.String("RUNNING"),
					HealthStatus:      ecstypes.HealthStatusHealthy,
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/worker:12"),
					StartedAt:         &startedAt,
					LaunchType:        ecstypes.LaunchTypeEc2,
					Cpu:               aws.String("512"),
					Memory:            aws.String("1024"),
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcTasks(
		context.Background(),
		listTasksMock,
		describeTasksMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"web-service",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	t.Run("task_0_task_id_short", func(t *testing.T) {
		r := result.Resources[0]
		// task_id_short should be the last segment of the task ARN
		if r.Fields["task_id_short"] != "abc123def456" {
			t.Errorf("Fields[task_id_short]: expected %q, got %q", "abc123def456", r.Fields["task_id_short"])
		}
	})

	t.Run("task_0_status", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["status"] != "RUNNING" {
			t.Errorf("Fields[status]: expected %q, got %q", "RUNNING", r.Fields["status"])
		}
	})

	t.Run("task_0_health", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["health"] != "HEALTHY" {
			t.Errorf("Fields[health]: expected %q, got %q", "HEALTHY", r.Fields["health"])
		}
	})

	t.Run("task_0_task_def_short", func(t *testing.T) {
		r := result.Resources[0]
		// task_def_short should be "family:revision" extracted from TaskDefinitionArn
		if r.Fields["task_def_short"] != "web-app:5" {
			t.Errorf("Fields[task_def_short]: expected %q, got %q", "web-app:5", r.Fields["task_def_short"])
		}
	})

	t.Run("task_0_started_at", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["started_at"] == "" {
			t.Error("Fields[started_at] should not be empty")
		}
	})

	t.Run("task_0_stopped_reason", func(t *testing.T) {
		r := result.Resources[0]
		// Running task should have empty stopped_reason
		if r.Fields["stopped_reason"] != "" {
			t.Errorf("Fields[stopped_reason]: expected empty for running task, got %q", r.Fields["stopped_reason"])
		}
	})

	t.Run("task_0_ID", func(t *testing.T) {
		if result.Resources[0].ID == "" {
			t.Error("ID should not be empty")
		}
	})

	t.Run("task_0_RawStruct", func(t *testing.T) {
		r := result.Resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(ecstypes.Task)
		if !ok {
			t.Fatalf("RawStruct should be ecstypes.Task, got %T", r.RawStruct)
		}
		if raw.TaskArn == nil || *raw.TaskArn != "arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456" {
			t.Errorf("RawStruct.TaskArn not preserved correctly")
		}
	})

	// Verify required fields on all tasks
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"task_id_short", "status", "health", "task_def_short", "started_at", "stopped_reason"}
		for i, r := range result.Resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchEcsSvcTasks_MixedStatus verifies handling of RUNNING and STOPPED
// tasks together.
func TestFetchEcsSvcTasks_MixedStatus(t *testing.T) {
	startedAt := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	stoppedAt := time.Date(2024, 3, 22, 11, 0, 0, 0, time.UTC)

	listTasksMock := &mockECSListTasksClient{
		outputs: map[string]*ecs.ListTasksOutput{
			"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster": {
				TaskArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/running-task",
					"arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/stopped-task",
				},
			},
		},
	}

	describeTasksMock := &mockECSDescribeTasksClient{
		output: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{
				{
					TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/running-task"),
					LastStatus:        aws.String("RUNNING"),
					DesiredStatus:     aws.String("RUNNING"),
					HealthStatus:      ecstypes.HealthStatusHealthy,
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/app:3"),
					StartedAt:         &startedAt,
					LaunchType:        ecstypes.LaunchTypeFargate,
				},
				{
					TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/stopped-task"),
					LastStatus:        aws.String("STOPPED"),
					DesiredStatus:     aws.String("STOPPED"),
					HealthStatus:      ecstypes.HealthStatusUnknown,
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/app:2"),
					StartedAt:         &startedAt,
					StoppedAt:         &stoppedAt,
					StoppedReason:     aws.String("Essential container in task exited"),
					StopCode:          ecstypes.TaskStopCodeEssentialContainerExited,
					LaunchType:        ecstypes.LaunchTypeFargate,
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcTasks(
		context.Background(),
		listTasksMock,
		describeTasksMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"web-service",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	t.Run("running_task_status", func(t *testing.T) {
		if result.Resources[0].Status != "RUNNING" {
			t.Errorf("Status: expected %q, got %q", "RUNNING", result.Resources[0].Status)
		}
	})

	t.Run("stopped_task_status", func(t *testing.T) {
		if result.Resources[1].Status != "STOPPED" {
			t.Errorf("Status: expected %q, got %q", "STOPPED", result.Resources[1].Status)
		}
	})

	t.Run("stopped_task_reason", func(t *testing.T) {
		if result.Resources[1].Fields["stopped_reason"] != "Essential container in task exited" {
			t.Errorf("Fields[stopped_reason]: expected %q, got %q",
				"Essential container in task exited", result.Resources[1].Fields["stopped_reason"])
		}
	})
}

// TestFetchEcsSvcTasks_Empty verifies that no tasks returns an empty slice.
func TestFetchEcsSvcTasks_Empty(t *testing.T) {
	listTasksMock := &mockECSListTasksClient{
		outputs: map[string]*ecs.ListTasksOutput{
			"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster": {
				TaskArns: []string{},
			},
		},
	}

	describeTasksMock := &mockECSDescribeTasksClient{}

	result, err := awsclient.FetchEcsSvcTasks(
		context.Background(),
		listTasksMock,
		describeTasksMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"empty-service",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchEcsSvcTasks_ListTasksError verifies that ListTasks errors propagate.
func TestFetchEcsSvcTasks_ListTasksError(t *testing.T) {
	listTasksMock := &mockECSListTasksClient{
		err: fmt.Errorf("AWS API error: throttling exception"),
	}

	describeTasksMock := &mockECSDescribeTasksClient{}

	result, err := awsclient.FetchEcsSvcTasks(
		context.Background(),
		listTasksMock,
		describeTasksMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"err-service",
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if result.Resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(result.Resources))
	}
}

// TestFetchEcsSvcTasks_DescribeTasksError verifies that DescribeTasks errors propagate.
func TestFetchEcsSvcTasks_DescribeTasksError(t *testing.T) {
	listTasksMock := &mockECSListTasksClient{
		outputs: map[string]*ecs.ListTasksOutput{
			"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster": {
				TaskArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/some-task",
				},
			},
		},
	}

	describeTasksMock := &mockECSDescribeTasksClient{
		err: fmt.Errorf("AWS API error: internal server error"),
	}

	result, err := awsclient.FetchEcsSvcTasks(
		context.Background(),
		listTasksMock,
		describeTasksMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"err-service",
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if result.Resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(result.Resources))
	}
}

// TestFetchEcsSvcTasks_ComputedFields verifies that computed fields are correct:
// task_id_short from ARN, task_def_short from TaskDefinitionArn.
func TestFetchEcsSvcTasks_ComputedFields(t *testing.T) {
	startedAt := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	listTasksMock := &mockECSListTasksClient{
		outputs: map[string]*ecs.ListTasksOutput{
			"arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster": {
				TaskArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task/my-cluster/a1b2c3d4e5f6",
				},
			},
		},
	}

	describeTasksMock := &mockECSDescribeTasksClient{
		output: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{
				{
					TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/my-cluster/a1b2c3d4e5f6"),
					LastStatus:        aws.String("RUNNING"),
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/my-service:42"),
					StartedAt:         &startedAt,
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcTasks(
		context.Background(),
		listTasksMock,
		describeTasksMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster",
		"my-service",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]

	t.Run("task_id_short_extracts_last_segment", func(t *testing.T) {
		if r.Fields["task_id_short"] != "a1b2c3d4e5f6" {
			t.Errorf("Fields[task_id_short]: expected %q, got %q", "a1b2c3d4e5f6", r.Fields["task_id_short"])
		}
	})

	t.Run("task_def_short_extracts_family_revision", func(t *testing.T) {
		if r.Fields["task_def_short"] != "my-service:42" {
			t.Errorf("Fields[task_def_short]: expected %q, got %q", "my-service:42", r.Fields["task_def_short"])
		}
	})
}

// TestFetchEcsSvcTasks_NilFields verifies that nil StartedAt, nil StoppedReason,
// nil HealthStatus, nil TaskArn, nil TaskDefinitionArn do not cause a panic.
func TestFetchEcsSvcTasks_NilFields(t *testing.T) {
	listTasksMock := &mockECSListTasksClient{
		outputs: map[string]*ecs.ListTasksOutput{
			"arn:aws:ecs:us-east-1:123456789012:cluster/nil-cluster": {
				TaskArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task/nil-cluster/nil-task",
				},
			},
		},
	}

	describeTasksMock := &mockECSDescribeTasksClient{
		output: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{
				{
					// All pointer fields nil, zero-value enums
				},
			},
		},
	}

	// Should not panic
	result, err := awsclient.FetchEcsSvcTasks(
		context.Background(),
		listTasksMock,
		describeTasksMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/nil-cluster",
		"nil-service",
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
		// If we got here, no panic occurred
	})

	t.Run("task_id_short_empty", func(t *testing.T) {
		if r.Fields["task_id_short"] != "" {
			t.Errorf("Fields[task_id_short]: expected empty, got %q", r.Fields["task_id_short"])
		}
	})

	t.Run("started_at_empty", func(t *testing.T) {
		if r.Fields["started_at"] != "" {
			t.Errorf("Fields[started_at]: expected empty, got %q", r.Fields["started_at"])
		}
	})

	t.Run("stopped_reason_empty", func(t *testing.T) {
		if r.Fields["stopped_reason"] != "" {
			t.Errorf("Fields[stopped_reason]: expected empty, got %q", r.Fields["stopped_reason"])
		}
	})
}

// TestFetchEcsSvcTasks_RawStruct verifies that RawStruct preserves the original
// ecstypes.Task, including all SDK fields.
func TestFetchEcsSvcTasks_RawStruct(t *testing.T) {
	startedAt := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	listTasksMock := &mockECSListTasksClient{
		outputs: map[string]*ecs.ListTasksOutput{
			"arn:aws:ecs:us-east-1:123456789012:cluster/raw-cluster": {
				TaskArns: []string{
					"arn:aws:ecs:us-east-1:123456789012:task/raw-cluster/raw-task-id",
				},
			},
		},
	}

	describeTasksMock := &mockECSDescribeTasksClient{
		output: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{
				{
					TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/raw-cluster/raw-task-id"),
					LastStatus:        aws.String("RUNNING"),
					DesiredStatus:     aws.String("RUNNING"),
					HealthStatus:      ecstypes.HealthStatusHealthy,
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/raw-app:1"),
					StartedAt:         &startedAt,
					Cpu:               aws.String("256"),
					Memory:            aws.String("512"),
					LaunchType:        ecstypes.LaunchTypeFargate,
					PlatformVersion:   aws.String("1.4.0"),
					Group:             aws.String("service:web-service"),
					StartedBy:         aws.String("ecs-svc/1234567890"),
				},
			},
		},
	}

	result, err := awsclient.FetchEcsSvcTasks(
		context.Background(),
		listTasksMock,
		describeTasksMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/raw-cluster",
		"web-service",
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

	raw, ok := r.RawStruct.(ecstypes.Task)
	if !ok {
		t.Fatalf("RawStruct should be ecstypes.Task, got %T", r.RawStruct)
	}

	t.Run("TaskArn_preserved", func(t *testing.T) {
		if raw.TaskArn == nil || *raw.TaskArn != "arn:aws:ecs:us-east-1:123456789012:task/raw-cluster/raw-task-id" {
			t.Errorf("RawStruct.TaskArn not preserved correctly")
		}
	})

	t.Run("LastStatus_preserved", func(t *testing.T) {
		if raw.LastStatus == nil || *raw.LastStatus != "RUNNING" {
			t.Errorf("RawStruct.LastStatus not preserved correctly")
		}
	})

	t.Run("StartedAt_preserved", func(t *testing.T) {
		if raw.StartedAt == nil || !raw.StartedAt.Equal(startedAt) {
			t.Errorf("RawStruct.StartedAt not preserved correctly")
		}
	})

	t.Run("PlatformVersion_preserved", func(t *testing.T) {
		if raw.PlatformVersion == nil || *raw.PlatformVersion != "1.4.0" {
			t.Errorf("RawStruct.PlatformVersion not preserved correctly")
		}
	})
}

// TestEcsSvcTaskColumns verifies that EcsSvcTaskColumns returns the expected
// columns with correct keys.
func TestEcsSvcTaskColumns(t *testing.T) {
	cols := resource.EcsSvcTaskColumns()

	expectedKeys := []string{"task_id_short", "status", "health", "task_def_short", "started_at", "stopped_reason"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 6 {
			t.Fatalf("expected 6 columns, got %d", len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, expected := range expectedKeys {
			if cols[i].Key != expected {
				t.Errorf("column[%d].Key: expected %q, got %q", i, expected, cols[i].Key)
			}
		}
	})

	t.Run("columns_have_titles", func(t *testing.T) {
		for i, col := range cols {
			if col.Title == "" {
				t.Errorf("column[%d] (%s) has empty Title", i, col.Key)
			}
		}
	})

	t.Run("columns_have_positive_width", func(t *testing.T) {
		for i, col := range cols {
			if col.Width <= 0 {
				t.Errorf("column[%d] (%s) has non-positive Width: %d", i, col.Key, col.Width)
			}
		}
	})
}

// TestEcsSvcTasks_ChildTypeRegistered verifies that the child type is
// registered under the correct short name.
func TestEcsSvcTasks_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("ecs_tasks")
	if td == nil {
		t.Fatal("ecs_tasks child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "ecs_tasks" {
		t.Errorf("child type ShortName: expected %q, got %q", "ecs_tasks", td.ShortName)
	}
}

// mockPaginatedECSListTasksClient supports pagination via NextToken for testing.
type mockPaginatedECSListTasksClient struct {
	pages []*ecs.ListTasksOutput
	idx   int
}

func (m *mockPaginatedECSListTasksClient) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	if m.idx >= len(m.pages) {
		return &ecs.ListTasksOutput{}, nil
	}
	out := m.pages[m.idx]
	m.idx++
	return out, nil
}

// mockBatchingDescribeTasksClient tracks batch sizes for pagination testing.
type mockBatchingDescribeTasksClient struct {
	allTasks   []ecstypes.Task
	batchSizes []int
}

func (m *mockBatchingDescribeTasksClient) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	m.batchSizes = append(m.batchSizes, len(params.Tasks))
	start := 0
	for _, prev := range m.batchSizes[:len(m.batchSizes)-1] {
		start += prev
	}
	end := min(start+len(params.Tasks), len(m.allTasks))
	return &ecs.DescribeTasksOutput{
		Tasks: m.allTasks[start:end],
	}, nil
}

// TestFetchEcsSvcTasks_Pagination verifies that the fetcher handles pagination
// (NextToken on ListTasks) and batching (DescribeTasks max 100 per call).
func TestFetchEcsSvcTasks_Pagination(t *testing.T) {
	// Build 150 task ARNs across 2 pages of RUNNING tasks
	page1Arns := make([]string, 100)
	for i := range page1Arns {
		page1Arns[i] = fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:task/cluster/task%03d", i)
	}
	page2Arns := make([]string, 50)
	for i := range page2Arns {
		page2Arns[i] = fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:task/cluster/task%03d", 100+i)
	}

	nextToken := "page2"
	listMock := &mockPaginatedECSListTasksClient{
		pages: []*ecs.ListTasksOutput{
			{TaskArns: page1Arns, NextToken: &nextToken},
			{TaskArns: page2Arns},
			{}, // STOPPED status: empty
		},
	}

	// Build matching tasks for DescribeTasks
	allTasks := make([]ecstypes.Task, 150)
	for i := range allTasks {
		arn := fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:task/cluster/task%03d", i)
		allTasks[i] = ecstypes.Task{
			TaskArn:    &arn,
			LastStatus: aws.String("RUNNING"),
		}
	}

	describeMock := &mockBatchingDescribeTasksClient{allTasks: allTasks}

	results, err := awsclient.FetchEcsSvcTasks(context.Background(), listMock, describeMock, "arn:aws:ecs:us-east-1:123456789012:cluster/cluster", "my-service", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results.Resources) != 150 {
		t.Errorf("expected 150 tasks, got %d", len(results.Resources))
	}

	// Should have called DescribeTasks twice: 100 + 50
	if len(describeMock.batchSizes) != 2 {
		t.Fatalf("expected 2 DescribeTasks calls (batching at 100), got %d", len(describeMock.batchSizes))
	}
	if describeMock.batchSizes[0] != 100 {
		t.Errorf("first batch should be 100, got %d", describeMock.batchSizes[0])
	}
	if describeMock.batchSizes[1] != 50 {
		t.Errorf("second batch should be 50, got %d", describeMock.batchSizes[1])
	}
}

// TestEcsSvcTasks_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is
// registered under the correct short name.
func TestEcsSvcTasks_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("ecs_tasks")
	if f == nil {
		t.Fatal("ecs_tasks paginated child fetcher not registered")
	}
}

// TestEcsSvcTasks_ParentHasChildDef verifies that the parent ecs-svc resource
// type has a child view definition for ecs_tasks with key "enter".
func TestEcsSvcTasks_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("ecs-svc")
	if rt == nil {
		t.Fatal("ecs-svc resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "ecs_tasks" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["cluster"] == "" {
				t.Error("ContextKeys should include 'cluster'")
			}
			if child.ContextKeys["service_name"] == "" {
				t.Error("ContextKeys should include 'service_name'")
			}
		}
	}
	if !found {
		t.Error("ecs-svc Children should contain ecs_tasks child view def")
	}
}

// TestFetchEcsSvcTasks_ContinuationToken verifies that a non-empty continuation
// token is accepted without error. FetchEcsSvcTasks fetches RUNNING and STOPPED
// tasks in a single invocation (dual-status pattern); the continuation token
// parameter is accepted for interface compatibility but is not forwarded to the
// ListTasks API — per-status resumption is not supported in this implementation.
func TestFetchEcsSvcTasks_ContinuationToken(t *testing.T) {
	startedAt := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	wrapper := &tokenCapturingECSListTasksMock{
		inner: &mockECSListTasksClient{
			outputs: map[string]*ecs.ListTasksOutput{
				"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster": {
					TaskArns: []string{
						"arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456",
					},
				},
			},
		},
	}

	describeMock := &mockECSDescribeTasksClient{
		output: &ecs.DescribeTasksOutput{
			Tasks: []ecstypes.Task{
				{
					TaskArn:           aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456"),
					LastStatus:        aws.String("RUNNING"),
					HealthStatus:      ecstypes.HealthStatusHealthy,
					TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web:5"),
					StartedAt:         &startedAt,
				},
			},
		},
	}

	// The function must accept a non-empty continuation token without error.
	result, err := awsclient.FetchEcsSvcTasks(
		context.Background(),
		wrapper,
		describeMock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"my-service",
		"my-continuation-token",
	)
	if err != nil {
		t.Fatalf("expected no error with continuation token, got %v", err)
	}

	// The function should still return results regardless of the token.
	if len(result.Resources) == 0 {
		t.Fatal("expected at least 1 resource when continuation token provided")
	}

	// The continuation token is NOT forwarded to ListTasks in this implementation
	// (accepted for interface compatibility only — dual-status pattern fetches
	// one page of RUNNING + one page of STOPPED per invocation).
	if wrapper.capturedNextToken != nil {
		t.Errorf("continuation token should NOT be forwarded to ListTasks API, got %q", *wrapper.capturedNextToken)
	}
}

// tokenCapturingECSListTasksMock wraps the ECS ListTasks mock to capture NextToken
// from the first call only.
type tokenCapturingECSListTasksMock struct {
	inner             *mockECSListTasksClient
	capturedNextToken *string
	callCount         int
}

func (m *tokenCapturingECSListTasksMock) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	if m.callCount == 0 {
		m.capturedNextToken = params.NextToken
	}
	m.callCount++
	return m.inner.ListTasks(ctx, params, optFns...)
}
