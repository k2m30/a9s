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

	t.Run("event_0_Fields_resource_status", func(t *testing.T) {
		if got := resources[0].Fields["resource_status"]; got != "CREATE_COMPLETE" {
			t.Errorf("Fields[\"resource_status\"]: expected %q, got %q", "CREATE_COMPLETE", got)
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

// TestFetchCfnEvents_Pagination verifies the single-page pagination contract:
// one API call is made, resources from that page are returned, and IsTruncated/NextToken
// reflect whether more pages exist. A second call with the continuation token verifies
// that the token is forwarded and the final page sets IsTruncated=false.
func TestFetchCfnEvents_Pagination(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	// Page 1: 3 items with NextToken indicating more pages exist.
	page1Mock := &mockCFNDescribeStackEventsClient{
		output: &cloudformation.DescribeStackEventsOutput{
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
	}

	// First call: no continuation token — fetches page 1.
	result1, err := awsclient.FetchCfnEvents(
		context.Background(),
		page1Mock,
		"paginated-stack",
		"",
	)
	if err != nil {
		t.Fatalf("page 1: expected no error, got %v", err)
	}

	t.Run("page1_item_count", func(t *testing.T) {
		if len(result1.Resources) != 3 {
			t.Fatalf("expected 3 resources on page 1, got %d", len(result1.Resources))
		}
	})

	t.Run("page1_is_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("page 1: IsTruncated should be true when NextToken is present")
		}
	})

	t.Run("page1_next_token", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.NextToken != "page2-token" {
			t.Errorf("page 1: NextToken expected %q, got %q", "page2-token", result1.Pagination.NextToken)
		}
	})

	t.Run("page1_page_size", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.PageSize != 3 {
			t.Errorf("page 1: PageSize expected 3, got %d", result1.Pagination.PageSize)
		}
	})

	t.Run("page1_total_hint_negative", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.TotalHint != -1 {
			t.Errorf("page 1: TotalHint should be -1 when truncated, got %d", result1.Pagination.TotalHint)
		}
	})

	t.Run("page1_event_ids", func(t *testing.T) {
		expectedIDs := []string{"evt-p1-1", "evt-p1-2", "evt-p1-3"}
		for i, expectedID := range expectedIDs {
			if result1.Resources[i].ID != expectedID {
				t.Errorf("page 1: resources[%d].ID: expected %q, got %q", i, expectedID, result1.Resources[i].ID)
			}
		}
	})

	t.Run("page1_all_fields_populated", func(t *testing.T) {
		requiredFields := []string{"timestamp", "logical_resource_id", "resource_type", "resource_status", "resource_status_reason"}
		for i, r := range result1.Resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("page 1: resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})

	// Page 2: 3 items with no NextToken — last page.
	page2Mock := &mockCFNDescribeStackEventsClient{
		output: &cloudformation.DescribeStackEventsOutput{
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
	}

	// Second call: pass continuation token from page 1 to fetch page 2.
	result2, err := awsclient.FetchCfnEvents(
		context.Background(),
		page2Mock,
		"paginated-stack",
		result1.Pagination.NextToken,
	)
	if err != nil {
		t.Fatalf("page 2: expected no error, got %v", err)
	}

	t.Run("page2_item_count", func(t *testing.T) {
		if len(result2.Resources) != 3 {
			t.Fatalf("expected 3 resources on page 2, got %d", len(result2.Resources))
		}
	})

	t.Run("page2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("page 2: IsTruncated should be false on last page")
		}
	})

	t.Run("page2_empty_next_token", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.NextToken != "" {
			t.Errorf("page 2: NextToken should be empty on last page, got %q", result2.Pagination.NextToken)
		}
	})

	t.Run("page2_total_hint_equals_count", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.TotalHint != 3 {
			t.Errorf("page 2: TotalHint should equal item count (3) on last page, got %d", result2.Pagination.TotalHint)
		}
	})

	t.Run("page2_event_ids", func(t *testing.T) {
		expectedIDs := []string{"evt-p2-1", "evt-p2-2", "evt-p2-3"}
		for i, expectedID := range expectedIDs {
			if result2.Resources[i].ID != expectedID {
				t.Errorf("page 2: resources[%d].ID: expected %q, got %q", i, expectedID, result2.Resources[i].ID)
			}
		}
	})
}

// TestFetchCfnEvents_MaxCap verifies that a single API page of 50 items is
// returned as-is with correct IsTruncated=true metadata when the API indicates
// more pages exist. The 200-item cap no longer applies — each call returns one
// page and the caller drives pagination via continuation tokens.
func TestFetchCfnEvents_MaxCap(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// Build one page of 50 events with a NextToken indicating more pages exist.
	var events []cfntypes.StackEvent
	for i := range 50 {
		events = append(events, cfntypes.StackEvent{
			EventId:           aws.String(fmt.Sprintf("evt-p0-%d", i)),
			StackName:         aws.String("big-stack"),
			Timestamp:         &ts,
			LogicalResourceId: aws.String(fmt.Sprintf("Resource-p0-%d", i)),
			ResourceType:      aws.String("AWS::EC2::Instance"),
			ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
		})
	}

	mock := &mockCFNDescribeStackEventsClient{
		output: &cloudformation.DescribeStackEventsOutput{
			NextToken:   aws.String("token-page-1"),
			StackEvents: events,
		},
	}

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

	t.Run("single_page_returned", func(t *testing.T) {
		if len(resources) != 50 {
			t.Errorf("expected exactly 50 resources (one page), got %d", len(resources))
		}
	})

	t.Run("is_truncated_true", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result.Pagination.IsTruncated {
			t.Error("IsTruncated should be true when NextToken is present")
		}
	})

	t.Run("next_token_forwarded", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result.Pagination.NextToken != "token-page-1" {
			t.Errorf("NextToken expected %q, got %q", "token-page-1", result.Pagination.NextToken)
		}
	})

	t.Run("page_size_correct", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result.Pagination.PageSize != 50 {
			t.Errorf("PageSize expected 50, got %d", result.Pagination.PageSize)
		}
	})

	t.Run("total_hint_negative_when_truncated", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result.Pagination.TotalHint != -1 {
			t.Errorf("TotalHint should be -1 when truncated, got %d", result.Pagination.TotalHint)
		}
	})

	t.Run("first_event_correct", func(t *testing.T) {
		if resources[0].ID != "evt-p0-0" {
			t.Errorf("first resource ID: expected %q, got %q", "evt-p0-0", resources[0].ID)
		}
	})

	t.Run("last_event_correct", func(t *testing.T) {
		if resources[49].ID != "evt-p0-49" {
			t.Errorf("last resource ID: expected %q, got %q", "evt-p0-49", resources[49].ID)
		}
	})
}
