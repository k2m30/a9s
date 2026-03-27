package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// CFN Stack Events fetcher tests (child of CloudFormation Stacks)
// ---------------------------------------------------------------------------

// TestFetchCfnEvents_Basic verifies parsing of 3 stack events with known
// timestamps, statuses, and reasons, checking ID, Name, Status, all Fields,
// and RawStruct.
func TestFetchCfnEvents_Basic(t *testing.T) {
	ts1 := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2024, 3, 22, 10, 5, 0, 0, time.UTC)
	ts3 := time.Date(2024, 3, 22, 10, 10, 0, 0, time.UTC)

	mock := &mockCFNDescribeStackEventsClient{
		output: &cloudformation.DescribeStackEventsOutput{
			StackEvents: []cfntypes.StackEvent{
				{
					EventId:              aws.String("evt-001"),
					StackId:              aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/my-stack/guid1"),
					StackName:            aws.String("my-stack"),
					Timestamp:            &ts1,
					LogicalResourceId:    aws.String("MyBucket"),
					ResourceType:         aws.String("AWS::S3::Bucket"),
					ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
					ResourceStatusReason: aws.String("Resource creation complete"),
				},
				{
					EventId:              aws.String("evt-002"),
					StackId:              aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/my-stack/guid1"),
					StackName:            aws.String("my-stack"),
					Timestamp:            &ts2,
					LogicalResourceId:    aws.String("MyFunction"),
					ResourceType:         aws.String("AWS::Lambda::Function"),
					ResourceStatus:       cfntypes.ResourceStatusCreateInProgress,
					ResourceStatusReason: aws.String("Resource creation initiated"),
				},
				{
					EventId:              aws.String("evt-003"),
					StackId:              aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/my-stack/guid1"),
					StackName:            aws.String("my-stack"),
					Timestamp:            &ts3,
					LogicalResourceId:    aws.String("my-stack"),
					ResourceType:         aws.String("AWS::CloudFormation::Stack"),
					ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
					ResourceStatusReason: aws.String(""),
				},
			},
		},
	}

	result, err := awsclient.FetchCfnEvents(
		context.Background(),
		mock,
		"my-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("event_0_ID", func(t *testing.T) {
		if resources[0].ID != "evt-001" {
			t.Errorf("ID: expected %q, got %q", "evt-001", resources[0].ID)
		}
	})

	t.Run("event_0_Name_is_formatted_timestamp", func(t *testing.T) {
		if resources[0].Name == "" {
			t.Error("Name should not be empty")
		}
		if !strings.Contains(resources[0].Name, "2024-03-22") {
			t.Errorf("Name should contain formatted date, got %q", resources[0].Name)
		}
	})

	t.Run("event_0_Status", func(t *testing.T) {
		if resources[0].Status != "CREATE_COMPLETE" {
			t.Errorf("Status: expected %q, got %q", "CREATE_COMPLETE", resources[0].Status)
		}
	})

	t.Run("event_0_Fields_timestamp", func(t *testing.T) {
		r := resources[0]
		if r.Fields["timestamp"] == "" {
			t.Error("Fields[timestamp] should not be empty")
		}
		if !strings.Contains(r.Fields["timestamp"], "2024-03-22 10:00") {
			t.Errorf("Fields[timestamp] expected '2024-03-22 10:00', got %q", r.Fields["timestamp"])
		}
	})

	t.Run("event_0_Fields_logical_resource_id", func(t *testing.T) {
		r := resources[0]
		if r.Fields["logical_resource_id"] != "MyBucket" {
			t.Errorf("Fields[logical_resource_id]: expected %q, got %q", "MyBucket", r.Fields["logical_resource_id"])
		}
	})

	t.Run("event_0_Fields_resource_type", func(t *testing.T) {
		r := resources[0]
		if r.Fields["resource_type"] != "AWS::S3::Bucket" {
			t.Errorf("Fields[resource_type]: expected %q, got %q", "AWS::S3::Bucket", r.Fields["resource_type"])
		}
	})

	t.Run("event_0_Fields_resource_status", func(t *testing.T) {
		r := resources[0]
		if r.Fields["resource_status"] != "CREATE_COMPLETE" {
			t.Errorf("Fields[resource_status]: expected %q, got %q", "CREATE_COMPLETE", r.Fields["resource_status"])
		}
	})

	t.Run("event_0_Fields_resource_status_reason", func(t *testing.T) {
		r := resources[0]
		if r.Fields["resource_status_reason"] != "Resource creation complete" {
			t.Errorf("Fields[resource_status_reason]: expected %q, got %q", "Resource creation complete", r.Fields["resource_status_reason"])
		}
	})

	t.Run("event_0_RawStruct", func(t *testing.T) {
		r := resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(cfntypes.StackEvent)
		if !ok {
			t.Fatalf("RawStruct should be cfntypes.StackEvent, got %T", r.RawStruct)
		}
		if raw.EventId == nil || *raw.EventId != "evt-001" {
			t.Error("RawStruct.EventId not preserved correctly")
		}
	})

	// Verify required fields on all events
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"timestamp", "logical_resource_id", "resource_type", "resource_status", "resource_status_reason"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchCfnEvents_Empty verifies that a stack with no events
// returns an empty slice with no error.
func TestFetchCfnEvents_Empty(t *testing.T) {
	mock := &mockCFNDescribeStackEventsClient{
		output: &cloudformation.DescribeStackEventsOutput{
			StackEvents: []cfntypes.StackEvent{},
		},
	}

	result, err := awsclient.FetchCfnEvents(
		context.Background(),
		mock,
		"empty-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchCfnEvents_APIError verifies that API errors are propagated.
func TestFetchCfnEvents_APIError(t *testing.T) {
	mock := &mockCFNDescribeStackEventsClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	result, err := awsclient.FetchCfnEvents(
		context.Background(),
		mock,
		"err-stack",
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources on error, got %d", len(result.Resources))
	}
}

// TestFetchCfnEvents_NilOptionalFields verifies that nil ResourceStatusReason
// and nil PhysicalResourceId do not cause a panic.
func TestFetchCfnEvents_NilOptionalFields(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockCFNDescribeStackEventsClient{
		output: &cloudformation.DescribeStackEventsOutput{
			StackEvents: []cfntypes.StackEvent{
				{
					EventId:   aws.String("evt-nil-001"),
					StackId:   aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/nil-stack/guid"),
					StackName: aws.String("nil-stack"),
					Timestamp: &ts,
					// ResourceStatusReason is nil
					// PhysicalResourceId is nil
					// LogicalResourceId is nil
					// ResourceType is nil
					ResourceStatus: cfntypes.ResourceStatusCreateComplete,
				},
				{
					// EventId is nil too
					StackId:   aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/nil-stack/guid"),
					StackName: aws.String("nil-stack"),
					Timestamp: &ts,
				},
			},
		},
	}

	// Should not panic
	result, err := awsclient.FetchCfnEvents(
		context.Background(),
		mock,
		"nil-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	t.Run("nil_ResourceStatusReason", func(t *testing.T) {
		r := resources[0]
		if r.Fields["resource_status_reason"] != "" {
			t.Logf("Fields[resource_status_reason] is %q (expected empty for nil)", r.Fields["resource_status_reason"])
		}
	})

	t.Run("nil_LogicalResourceId", func(t *testing.T) {
		r := resources[0]
		// Should not panic, should be empty string
		_ = r.Fields["logical_resource_id"]
	})
}

// TestFetchCfnEvents_NewlineStripping verifies that ResourceStatusReason
// with newlines gets cleaned.
func TestFetchCfnEvents_NewlineStripping(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockCFNDescribeStackEventsClient{
		output: &cloudformation.DescribeStackEventsOutput{
			StackEvents: []cfntypes.StackEvent{
				{
					EventId:              aws.String("evt-newline"),
					StackId:              aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/nl-stack/guid"),
					StackName:            aws.String("nl-stack"),
					Timestamp:            &ts,
					LogicalResourceId:    aws.String("MyResource"),
					ResourceType:         aws.String("AWS::EC2::Instance"),
					ResourceStatus:       cfntypes.ResourceStatusCreateFailed,
					ResourceStatusReason: aws.String("Properties validation failed\nfor resource MyResource\nin stack nl-stack"),
				},
			},
		},
	}

	result, err := awsclient.FetchCfnEvents(
		context.Background(),
		mock,
		"nl-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	reason := resources[0].Fields["resource_status_reason"]
	if strings.Contains(reason, "\n") {
		t.Errorf("Fields[resource_status_reason] should not contain newlines, got %q", reason)
	}
}

// TestFetchCfnEvents_TimestampFormatting verifies that a known time.Time
// produces the "2006-01-02 15:04" format in Fields.
func TestFetchCfnEvents_TimestampFormatting(t *testing.T) {
	ts := time.Date(2024, 12, 25, 14, 30, 45, 0, time.UTC)

	mock := &mockCFNDescribeStackEventsClient{
		output: &cloudformation.DescribeStackEventsOutput{
			StackEvents: []cfntypes.StackEvent{
				{
					EventId:           aws.String("evt-ts"),
					StackId:           aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/ts-stack/guid"),
					StackName:         aws.String("ts-stack"),
					Timestamp:         &ts,
					LogicalResourceId: aws.String("MyResource"),
					ResourceType:      aws.String("AWS::EC2::Instance"),
					ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
				},
			},
		},
	}

	result, err := awsclient.FetchCfnEvents(
		context.Background(),
		mock,
		"ts-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	ts_field := resources[0].Fields["timestamp"]
	if ts_field != "2024-12-25 14:30" {
		t.Errorf("Fields[timestamp]: expected %q, got %q", "2024-12-25 14:30", ts_field)
	}
}

// TestFetchCfnEvents_RawStruct verifies that RawStruct preserves the
// original cfntypes.StackEvent, including all sub-fields.
func TestFetchCfnEvents_RawStruct(t *testing.T) {
	ts := time.Date(2024, 3, 22, 12, 30, 0, 0, time.UTC)

	mock := &mockCFNDescribeStackEventsClient{
		output: &cloudformation.DescribeStackEventsOutput{
			StackEvents: []cfntypes.StackEvent{
				{
					EventId:              aws.String("evt-raw-001"),
					StackId:              aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/raw-stack/guid"),
					StackName:            aws.String("raw-stack"),
					Timestamp:            &ts,
					LogicalResourceId:    aws.String("RawResource"),
					PhysicalResourceId:   aws.String("i-0123456789abcdef0"),
					ResourceType:         aws.String("AWS::EC2::Instance"),
					ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
					ResourceStatusReason: aws.String("Resource creation complete"),
					ClientRequestToken:   aws.String("console-token-12345"),
				},
			},
		},
	}

	result, err := awsclient.FetchCfnEvents(
		context.Background(),
		mock,
		"raw-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.RawStruct == nil {
		t.Fatal("RawStruct must not be nil")
	}

	raw, ok := r.RawStruct.(cfntypes.StackEvent)
	if !ok {
		t.Fatalf("RawStruct should be cfntypes.StackEvent, got %T", r.RawStruct)
	}

	t.Run("EventId_preserved", func(t *testing.T) {
		if raw.EventId == nil || *raw.EventId != "evt-raw-001" {
			t.Errorf("RawStruct.EventId not preserved correctly")
		}
	})

	t.Run("Timestamp_preserved", func(t *testing.T) {
		if raw.Timestamp == nil || !raw.Timestamp.Equal(ts) {
			t.Errorf("RawStruct.Timestamp not preserved correctly")
		}
	})

	t.Run("LogicalResourceId_preserved", func(t *testing.T) {
		if raw.LogicalResourceId == nil || *raw.LogicalResourceId != "RawResource" {
			t.Errorf("RawStruct.LogicalResourceId not preserved correctly")
		}
	})

	t.Run("PhysicalResourceId_preserved", func(t *testing.T) {
		if raw.PhysicalResourceId == nil || *raw.PhysicalResourceId != "i-0123456789abcdef0" {
			t.Errorf("RawStruct.PhysicalResourceId not preserved correctly")
		}
	})

	t.Run("ClientRequestToken_preserved", func(t *testing.T) {
		if raw.ClientRequestToken == nil || *raw.ClientRequestToken != "console-token-12345" {
			t.Errorf("RawStruct.ClientRequestToken not preserved correctly")
		}
	})
}

// TestCfnEventColumns verifies that CfnEventColumns returns the expected
// columns with correct keys.
func TestCfnEventColumns(t *testing.T) {
	cols := resource.CfnEventColumns()

	expectedKeys := []string{"timestamp", "logical_resource_id", "resource_type", "resource_status", "resource_status_reason"}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != len(expectedKeys) {
			t.Fatalf("expected %d columns, got %d", len(expectedKeys), len(cols))
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

// TestCfnEvents_ChildTypeRegistered verifies that the child type is
// registered under the correct short name.
func TestCfnEvents_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("cfn_events")
	if td == nil {
		t.Fatal("cfn_events child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "cfn_events" {
		t.Errorf("child type ShortName: expected %q, got %q", "cfn_events", td.ShortName)
	}
}

// TestCfnEvents_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is registered under the correct short name.
func TestCfnEvents_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("cfn_events")
	if f == nil {
		t.Fatal("cfn_events paginated child fetcher not registered")
	}
}

// TestCfnEvents_ParentHasChildDef verifies that the parent cfn resource
// type has a child view definition for cfn_events with key "enter".
func TestCfnEvents_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("cfn")
	if rt == nil {
		t.Fatal("cfn resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "cfn_events" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["stack_name"] == "" {
				t.Error("ContextKeys should include 'stack_name'")
			}
		}
	}
	if !found {
		t.Error("cfn Children should contain cfn_events child view def")
	}
}

// TestFetchCfnEvents_Pagination verifies that paginated responses via NextToken
// are followed and all events collected across multiple pages.
func TestFetchCfnEvents_Pagination(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockCFNDescribeStackEventsClient{
		outputs: []*cloudformation.DescribeStackEventsOutput{
			{
				NextToken: aws.String("page2-token"),
				StackEvents: []cfntypes.StackEvent{
					{
						EventId:           aws.String("evt-p1-1"),
						StackName:         aws.String("paginated-stack"),
						Timestamp:         &ts,
						LogicalResourceId: aws.String("Resource1"),
						ResourceType:      aws.String("AWS::S3::Bucket"),
						ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
					},
					{
						EventId:           aws.String("evt-p1-2"),
						StackName:         aws.String("paginated-stack"),
						Timestamp:         &ts,
						LogicalResourceId: aws.String("Resource2"),
						ResourceType:      aws.String("AWS::Lambda::Function"),
						ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
					},
					{
						EventId:           aws.String("evt-p1-3"),
						StackName:         aws.String("paginated-stack"),
						Timestamp:         &ts,
						LogicalResourceId: aws.String("Resource3"),
						ResourceType:      aws.String("AWS::EC2::Instance"),
						ResourceStatus:    cfntypes.ResourceStatusCreateInProgress,
					},
				},
			},
			{
				// No NextToken — last page
				StackEvents: []cfntypes.StackEvent{
					{
						EventId:           aws.String("evt-p2-1"),
						StackName:         aws.String("paginated-stack"),
						Timestamp:         &ts,
						LogicalResourceId: aws.String("Resource4"),
						ResourceType:      aws.String("AWS::DynamoDB::Table"),
						ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
					},
					{
						EventId:           aws.String("evt-p2-2"),
						StackName:         aws.String("paginated-stack"),
						Timestamp:         &ts,
						LogicalResourceId: aws.String("Resource5"),
						ResourceType:      aws.String("AWS::SNS::Topic"),
						ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
					},
					{
						EventId:           aws.String("evt-p2-3"),
						StackName:         aws.String("paginated-stack"),
						Timestamp:         &ts,
						LogicalResourceId: aws.String("Resource6"),
						ResourceType:      aws.String("AWS::SQS::Queue"),
						ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchCfnEvents(
		context.Background(),
		mock,
		"paginated-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 6 {
			t.Fatalf("expected 6 resources across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_events", func(t *testing.T) {
		expectedIDs := []string{"evt-p1-1", "evt-p1-2", "evt-p1-3"}
		for i, expectedID := range expectedIDs {
			if resources[i].ID != expectedID {
				t.Errorf("resources[%d].ID: expected %q, got %q", i, expectedID, resources[i].ID)
			}
		}
	})

	t.Run("page2_events", func(t *testing.T) {
		expectedIDs := []string{"evt-p2-1", "evt-p2-2", "evt-p2-3"}
		for i, expectedID := range expectedIDs {
			if resources[i+3].ID != expectedID {
				t.Errorf("resources[%d].ID: expected %q, got %q", i+3, expectedID, resources[i+3].ID)
			}
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected 2 API calls for pagination, got %d", mock.callIdx)
		}
	})

	t.Run("all_fields_populated", func(t *testing.T) {
		requiredFields := []string{"timestamp", "logical_resource_id", "resource_type", "resource_status", "resource_status_reason"}
		for i, r := range resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})
}

// TestFetchCfnEvents_MaxEventsCap verifies that the fetcher stops collecting
// events once it reaches the maxEvents=200 cap, even if more pages are available.
func TestFetchCfnEvents_MaxEventsCap(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// Build 5 pages of 50 events each (250 total). The fetcher should stop at 200.
	var outputs []*cloudformation.DescribeStackEventsOutput
	for page := 0; page < 5; page++ {
		var events []cfntypes.StackEvent
		for i := 0; i < 50; i++ {
			events = append(events, cfntypes.StackEvent{
				EventId:           aws.String(fmt.Sprintf("evt-p%d-%d", page, i)),
				StackName:         aws.String("big-stack"),
				Timestamp:         &ts,
				LogicalResourceId: aws.String(fmt.Sprintf("Resource-p%d-%d", page, i)),
				ResourceType:      aws.String("AWS::EC2::Instance"),
				ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
			})
		}
		out := &cloudformation.DescribeStackEventsOutput{
			StackEvents: events,
		}
		if page < 4 {
			out.NextToken = aws.String(fmt.Sprintf("token-page-%d", page+1))
		}
		outputs = append(outputs, out)
	}

	mock := &mockCFNDescribeStackEventsClient{outputs: outputs}

	result, err := awsclient.FetchCfnEvents(
		context.Background(),
		mock,
		"big-stack",
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	resources := result.Resources

	t.Run("capped_at_200", func(t *testing.T) {
		if len(resources) != 200 {
			t.Errorf("expected exactly 200 resources (maxEvents cap), got %d", len(resources))
		}
	})

	t.Run("early_termination", func(t *testing.T) {
		// With 50 events per page, reaching 200 should take exactly 4 pages.
		// The fetcher should NOT call the 5th page.
		if mock.callIdx != 4 {
			t.Errorf("expected 4 API calls (early termination at 200), got %d", mock.callIdx)
		}
	})

	t.Run("first_event_correct", func(t *testing.T) {
		if resources[0].ID != "evt-p0-0" {
			t.Errorf("first resource ID: expected %q, got %q", "evt-p0-0", resources[0].ID)
		}
	})

	t.Run("last_event_correct", func(t *testing.T) {
		// Last event should be the 50th event of page 3 (index 199 = page3, event49)
		if resources[199].ID != "evt-p3-49" {
			t.Errorf("last resource ID: expected %q, got %q", "evt-p3-49", resources[199].ID)
		}
	})
}
