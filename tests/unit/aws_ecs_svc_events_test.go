package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// ECS Service Events fetcher tests (child of ECS Services)
// ---------------------------------------------------------------------------

// TestFetchEcsSvcEvents_Basic verifies parsing of 3 service events with known
// timestamps and messages, checking ID, Name, Status, all Fields, and RawStruct.
func TestFetchEcsSvcEvents_Basic(t *testing.T) {
	ts1 := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2024, 3, 22, 10, 5, 0, 0, time.UTC)
	ts3 := time.Date(2024, 3, 22, 10, 10, 0, 0, time.UTC)

	mock := &mockECSDescribeServicesClient{
		output: &ecs.DescribeServicesOutput{
			Services: []ecstypes.Service{
				{
					ServiceName: aws.String("web-service"),
					Events: []ecstypes.ServiceEvent{
						{
							Id:        aws.String("evt-001"),
							CreatedAt: &ts1,
							Message:   aws.String("(service web-service) has reached a steady state."),
						},
						{
							Id:        aws.String("evt-002"),
							CreatedAt: &ts2,
							Message:   aws.String("(service web-service) has started 2 tasks: (task abc123)."),
						},
						{
							Id:        aws.String("evt-003"),
							CreatedAt: &ts3,
							Message:   aws.String("(service web-service) registered 1 targets in (target-group my-tg)."),
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchEcsSvcEvents(
		context.Background(),
		mock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"web-service",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("event_0_ID", func(t *testing.T) {
		if resources[0].ID == "" {
			t.Error("ID should not be empty")
		}
	})

	t.Run("event_0_Name_not_empty", func(t *testing.T) {
		if resources[0].Name == "" {
			t.Error("Name should not be empty")
		}
	})

	t.Run("event_0_Fields_timestamp", func(t *testing.T) {
		r := resources[0]
		if r.Fields["timestamp"] == "" {
			t.Error("Fields[timestamp] should not be empty")
		}
	})

	t.Run("event_0_Fields_message", func(t *testing.T) {
		r := resources[0]
		if r.Fields["message"] == "" {
			t.Error("Fields[message] should not be empty")
		}
		if !strings.Contains(r.Fields["message"], "steady state") {
			t.Errorf("Fields[message] expected to contain 'steady state', got %q", r.Fields["message"])
		}
	})

	t.Run("event_0_RawStruct", func(t *testing.T) {
		r := resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(ecstypes.ServiceEvent)
		if !ok {
			t.Fatalf("RawStruct should be ecstypes.ServiceEvent, got %T", r.RawStruct)
		}
		if raw.Message == nil || !strings.Contains(*raw.Message, "steady state") {
			t.Error("RawStruct.Message not preserved correctly")
		}
	})

	// Verify required fields on all events
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"timestamp", "message"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchEcsSvcEvents_Empty verifies that a service with no events
// returns an empty slice with no error.
func TestFetchEcsSvcEvents_Empty(t *testing.T) {
	mock := &mockECSDescribeServicesClient{
		output: &ecs.DescribeServicesOutput{
			Services: []ecstypes.Service{
				{
					ServiceName: aws.String("empty-service"),
					Events:      []ecstypes.ServiceEvent{},
				},
			},
		},
	}

	resources, err := awsclient.FetchEcsSvcEvents(
		context.Background(),
		mock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"empty-service",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// TestFetchEcsSvcEvents_APIError verifies that API errors are propagated.
func TestFetchEcsSvcEvents_APIError(t *testing.T) {
	mock := &mockECSDescribeServicesClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchEcsSvcEvents(
		context.Background(),
		mock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"err-service",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

// TestFetchEcsSvcEvents_NewlineStripping verifies that messages with
// newlines get cleaned.
func TestFetchEcsSvcEvents_NewlineStripping(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockECSDescribeServicesClient{
		output: &ecs.DescribeServicesOutput{
			Services: []ecstypes.Service{
				{
					ServiceName: aws.String("newline-svc"),
					Events: []ecstypes.ServiceEvent{
						{
							Id:        aws.String("evt-newline"),
							CreatedAt: &ts,
							Message:   aws.String("(service newline-svc) has reached a steady state.\nExtra line here."),
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchEcsSvcEvents(
		context.Background(),
		mock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"newline-svc",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	msg := resources[0].Fields["message"]
	if strings.Contains(msg, "\n") {
		t.Errorf("Fields[message] should not contain newlines, got %q", msg)
	}
}

// TestFetchEcsSvcEvents_NilFields verifies that nil CreatedAt and nil Message
// do not cause a panic.
func TestFetchEcsSvcEvents_NilFields(t *testing.T) {
	mock := &mockECSDescribeServicesClient{
		output: &ecs.DescribeServicesOutput{
			Services: []ecstypes.Service{
				{
					ServiceName: aws.String("nil-svc"),
					Events: []ecstypes.ServiceEvent{
						{
							// All fields nil except Id
							Id: aws.String("evt-nil"),
						},
						{
							// Completely nil
						},
					},
				},
			},
		},
	}

	// Should not panic
	resources, err := awsclient.FetchEcsSvcEvents(
		context.Background(),
		mock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"nil-svc",
	)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	t.Run("nil_CreatedAt", func(t *testing.T) {
		if resources[0].Fields["timestamp"] != "" { //nolint:staticcheck // sentinel check: nil CreatedAt should yield empty timestamp
			t.Logf("Fields[timestamp] is non-empty: %q", resources[0].Fields["timestamp"])
		}
	})

	t.Run("nil_Message", func(t *testing.T) {
		if resources[1].Fields["message"] != "" { //nolint:staticcheck // sentinel check: nil Message should yield empty message
			t.Logf("Fields[message] is non-empty: %q", resources[1].Fields["message"])
		}
	})
}

// TestFetchEcsSvcEvents_RawStruct verifies that RawStruct preserves the
// original ecstypes.ServiceEvent, including all sub-fields.
func TestFetchEcsSvcEvents_RawStruct(t *testing.T) {
	ts := time.Date(2024, 3, 22, 12, 30, 0, 0, time.UTC)

	mock := &mockECSDescribeServicesClient{
		output: &ecs.DescribeServicesOutput{
			Services: []ecstypes.Service{
				{
					ServiceName: aws.String("raw-svc"),
					Events: []ecstypes.ServiceEvent{
						{
							Id:        aws.String("evt-raw-001"),
							CreatedAt: &ts,
							Message:   aws.String("(service raw-svc) has started 1 tasks."),
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchEcsSvcEvents(
		context.Background(),
		mock,
		"arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster",
		"raw-svc",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	raw, ok := r.RawStruct.(ecstypes.ServiceEvent)
	if !ok {
		t.Fatalf("RawStruct should be ecstypes.ServiceEvent, got %T", r.RawStruct)
	}

	t.Run("Id_preserved", func(t *testing.T) {
		if raw.Id == nil || *raw.Id != "evt-raw-001" {
			t.Errorf("RawStruct.Id not preserved correctly")
		}
	})

	t.Run("CreatedAt_preserved", func(t *testing.T) {
		if raw.CreatedAt == nil || !raw.CreatedAt.Equal(ts) {
			t.Errorf("RawStruct.CreatedAt not preserved correctly")
		}
	})

	t.Run("Message_preserved", func(t *testing.T) {
		if raw.Message == nil || !strings.Contains(*raw.Message, "started 1 tasks") {
			t.Errorf("RawStruct.Message not preserved correctly")
		}
	})
}

// TestEcsSvcEventColumns verifies that EcsSvcEventColumns returns the expected
// columns with correct keys.
func TestEcsSvcEventColumns(t *testing.T) {
	cols := resource.EcsSvcEventColumns()

	expectedKeys := []string{"timestamp", "message"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 2 {
			t.Fatalf("expected 2 columns, got %d", len(cols))
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

// TestEcsSvcEvents_ChildTypeRegistered verifies that the child type is
// registered under the correct short name.
func TestEcsSvcEvents_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("ecs_svc_events")
	if td == nil {
		t.Fatal("ecs_svc_events child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "ecs_svc_events" {
		t.Errorf("child type ShortName: expected %q, got %q", "ecs_svc_events", td.ShortName)
	}
}

// TestEcsSvcEvents_ChildFetcherRegistered verifies that the child fetcher is
// registered under the correct short name.
func TestEcsSvcEvents_ChildFetcherRegistered(t *testing.T) {
	f := resource.GetChildFetcher("ecs_svc_events")
	if f == nil {
		t.Fatal("ecs_svc_events child fetcher not registered")
	}
}

// TestEcsSvcEvents_ParentHasChildDef verifies that the parent ecs-svc resource
// type has a child view definition for ecs_svc_events with key "e".
func TestEcsSvcEvents_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("ecs-svc")
	if rt == nil {
		t.Fatal("ecs-svc resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "ecs_svc_events" {
			found = true
			if child.Key != "e" {
				t.Errorf("expected key %q, got %q", "e", child.Key)
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
		t.Error("ecs-svc Children should contain ecs_svc_events child view def")
	}
}
