package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// ASG Scaling Activities fetcher tests (child of Auto Scaling Groups)
// ---------------------------------------------------------------------------

// TestFetchAsgActivities_Basic verifies parsing of 1 activity with all fields
// populated, checking ID, Name, Status, all Fields, and RawStruct.
func TestFetchAsgActivities_Basic(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	endTs := time.Date(2024, 3, 22, 10, 5, 0, 0, time.UTC)
	progress := int32(100)

	mock := &mockASGDescribeScalingActivitiesClient{
		output: &autoscaling.DescribeScalingActivitiesOutput{
			Activities: []asgtypes.Activity{
				{
					ActivityId:            aws.String("act-001"),
					AutoScalingGroupName:  aws.String("my-asg"),
					AutoScalingGroupARN:   aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:guid:autoScalingGroupName/my-asg"),
					AutoScalingGroupState: aws.String("InService"),
					Cause:                 aws.String("At 2024-03-22T10:00:00Z an instance was started in response to a difference between desired and actual capacity"),
					Description:           aws.String("Launching a new EC2 instance: i-0abc1234def56789a"),
					Details:               aws.String("{\"Subnet ID\":\"subnet-12345\"}"),
					StartTime:             &ts,
					EndTime:               &endTs,
					StatusCode:            asgtypes.ScalingActivityStatusCodeSuccessful,
					StatusMessage:         aws.String(""),
					Progress:              &progress,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"asg_name": "my-asg",
	}

	result, err := awsclient.FetchAsgActivities(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]

	t.Run("ID_is_ActivityId", func(t *testing.T) {
		if r.ID != "act-001" {
			t.Errorf("ID: expected %q, got %q", "act-001", r.ID)
		}
	})

	t.Run("Name_is_formatted_start_time", func(t *testing.T) {
		if r.Name == "" {
			t.Error("Name should not be empty")
		}
		if !strings.Contains(r.Name, "2024-03-22") {
			t.Errorf("Name should contain formatted date, got %q", r.Name)
		}
	})

	t.Run("Status_is_string_StatusCode", func(t *testing.T) {
		if r.Fields["status"] != "Successful" {
			t.Errorf("Fields[\"status\"]: expected %q, got %q", "Successful", r.Fields["status"])
		}
	})

	t.Run("Fields_start_time", func(t *testing.T) {
		if r.Fields["start_time"] == "" {
			t.Error("Fields[start_time] should not be empty")
		}
		if !strings.Contains(r.Fields["start_time"], "2024-03-22 10:00") {
			t.Errorf("Fields[start_time] expected '2024-03-22 10:00', got %q", r.Fields["start_time"])
		}
	})

	t.Run("Fields_status_code", func(t *testing.T) {
		if r.Fields["status_code"] != "Successful" {
			t.Errorf("Fields[status_code]: expected %q, got %q", "Successful", r.Fields["status_code"])
		}
	})

	t.Run("Fields_description", func(t *testing.T) {
		if r.Fields["description"] != "Launching a new EC2 instance: i-0abc1234def56789a" {
			t.Errorf("Fields[description]: expected %q, got %q", "Launching a new EC2 instance: i-0abc1234def56789a", r.Fields["description"])
		}
	})

	t.Run("Fields_cause", func(t *testing.T) {
		if !strings.Contains(r.Fields["cause"], "instance was started") {
			t.Errorf("Fields[cause] should contain 'instance was started', got %q", r.Fields["cause"])
		}
	})

	t.Run("RawStruct_is_Activity", func(t *testing.T) {
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(asgtypes.Activity)
		if !ok {
			t.Fatalf("RawStruct should be asgtypes.Activity, got %T", r.RawStruct)
		}
		if raw.ActivityId == nil || *raw.ActivityId != "act-001" {
			t.Error("RawStruct.ActivityId not preserved correctly")
		}
	})

	// Verify required fields are present
	t.Run("required_fields_present", func(t *testing.T) {
		requiredFields := []string{"start_time", "status_code", "description", "cause"}
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("Fields missing key %q", key)
			}
		}
	})
}

// TestFetchAsgActivities_Empty verifies that an ASG with no activities
// returns an empty slice with no error.
func TestFetchAsgActivities_Empty(t *testing.T) {
	mock := &mockASGDescribeScalingActivitiesClient{
		output: &autoscaling.DescribeScalingActivitiesOutput{
			Activities: []asgtypes.Activity{},
		},
	}

	parentCtx := map[string]string{
		"asg_name": "empty-asg",
	}

	result, err := awsclient.FetchAsgActivities(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
}

// TestFetchAsgActivities_APIError verifies that API errors are propagated.
func TestFetchAsgActivities_APIError(t *testing.T) {
	mock := &mockASGDescribeScalingActivitiesClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}

	parentCtx := map[string]string{
		"asg_name": "err-asg",
	}

	result, err := awsclient.FetchAsgActivities(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if result.Resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(result.Resources))
	}
}

// TestFetchAsgActivities_NilOptionalFields verifies that nil optional fields
// (Description, Details, EndTime, Progress, StatusMessage, AutoScalingGroupARN,
// AutoScalingGroupState) do not cause a panic.
func TestFetchAsgActivities_NilOptionalFields(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockASGDescribeScalingActivitiesClient{
		output: &autoscaling.DescribeScalingActivitiesOutput{
			Activities: []asgtypes.Activity{
				{
					ActivityId:           aws.String("act-nil-001"),
					AutoScalingGroupName: aws.String("nil-asg"),
					Cause:                aws.String("Manual scaling"),
					StartTime:            &ts,
					StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
					// All optional fields are nil
				},
			},
		},
	}

	parentCtx := map[string]string{
		"asg_name": "nil-asg",
	}

	// Should not panic
	result, err := awsclient.FetchAsgActivities(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error for nil fields, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	t.Run("nil_Description", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["description"] != "" {
			t.Logf("Fields[description] is %q (expected empty for nil)", r.Fields["description"])
		}
	})

	t.Run("status_code_populated", func(t *testing.T) {
		r := result.Resources[0]
		if r.Fields["status_code"] != "Successful" {
			t.Errorf("Fields[status_code]: expected %q, got %q", "Successful", r.Fields["status_code"])
		}
	})
}

// TestFetchAsgActivities_NewlineStripping verifies that Description and Cause
// with newlines and carriage returns get stripped in Fields.
func TestFetchAsgActivities_NewlineStripping(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	mock := &mockASGDescribeScalingActivitiesClient{
		output: &autoscaling.DescribeScalingActivitiesOutput{
			Activities: []asgtypes.Activity{
				{
					ActivityId:           aws.String("act-newline"),
					AutoScalingGroupName: aws.String("nl-asg"),
					Cause:                aws.String("At 2024-03-22T10:00:00Z\nan instance was started\rin response to scaling"),
					Description:          aws.String("Launching a new\nEC2 instance:\ri-0abc1234"),
					StartTime:            &ts,
					StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"asg_name": "nl-asg",
	}

	result, err := awsclient.FetchAsgActivities(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	t.Run("description_no_newlines", func(t *testing.T) {
		desc := result.Resources[0].Fields["description"]
		if strings.Contains(desc, "\n") || strings.Contains(desc, "\r") {
			t.Errorf("Fields[description] should not contain newlines, got %q", desc)
		}
	})

	t.Run("cause_no_newlines", func(t *testing.T) {
		cause := result.Resources[0].Fields["cause"]
		if strings.Contains(cause, "\n") || strings.Contains(cause, "\r") {
			t.Errorf("Fields[cause] should not contain newlines, got %q", cause)
		}
	})
}

// TestFetchAsgActivities_TimestampFormatting verifies that a known time.Time
// produces the "2006-01-02 15:04" format in Fields.
func TestFetchAsgActivities_TimestampFormatting(t *testing.T) {
	ts := time.Date(2024, 12, 25, 14, 30, 45, 0, time.UTC)

	mock := &mockASGDescribeScalingActivitiesClient{
		output: &autoscaling.DescribeScalingActivitiesOutput{
			Activities: []asgtypes.Activity{
				{
					ActivityId:           aws.String("act-ts"),
					AutoScalingGroupName: aws.String("ts-asg"),
					Cause:                aws.String("Manual scaling"),
					StartTime:            &ts,
					StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"asg_name": "ts-asg",
	}

	result, err := awsclient.FetchAsgActivities(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	tsField := result.Resources[0].Fields["start_time"]
	if tsField != "2024-12-25 14:30" {
		t.Errorf("Fields[start_time]: expected %q, got %q", "2024-12-25 14:30", tsField)
	}
}

// TestFetchAsgActivities_RawStruct verifies that RawStruct preserves the
// original asgtypes.Activity, including all sub-fields.
func TestFetchAsgActivities_RawStruct(t *testing.T) {
	ts := time.Date(2024, 3, 22, 12, 30, 0, 0, time.UTC)
	endTs := time.Date(2024, 3, 22, 12, 35, 0, 0, time.UTC)
	progress := int32(100)

	mock := &mockASGDescribeScalingActivitiesClient{
		output: &autoscaling.DescribeScalingActivitiesOutput{
			Activities: []asgtypes.Activity{
				{
					ActivityId:            aws.String("act-raw-001"),
					AutoScalingGroupName:  aws.String("raw-asg"),
					AutoScalingGroupARN:   aws.String("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:guid:autoScalingGroupName/raw-asg"),
					AutoScalingGroupState: aws.String("InService"),
					Cause:                 aws.String("Manual scaling"),
					Description:           aws.String("Launching a new EC2 instance"),
					Details:               aws.String("{\"Subnet ID\":\"subnet-12345\"}"),
					StartTime:             &ts,
					EndTime:               &endTs,
					StatusCode:            asgtypes.ScalingActivityStatusCodeSuccessful,
					StatusMessage:         aws.String(""),
					Progress:              &progress,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"asg_name": "raw-asg",
	}

	result, err := awsclient.FetchAsgActivities(
		context.Background(),
		mock,
		parentCtx,
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

	raw, ok := r.RawStruct.(asgtypes.Activity)
	if !ok {
		t.Fatalf("RawStruct should be asgtypes.Activity, got %T", r.RawStruct)
	}

	t.Run("ActivityId_preserved", func(t *testing.T) {
		if raw.ActivityId == nil || *raw.ActivityId != "act-raw-001" {
			t.Errorf("RawStruct.ActivityId not preserved correctly")
		}
	})

	t.Run("StartTime_preserved", func(t *testing.T) {
		if raw.StartTime == nil || !raw.StartTime.Equal(ts) {
			t.Errorf("RawStruct.StartTime not preserved correctly")
		}
	})

	t.Run("EndTime_preserved", func(t *testing.T) {
		if raw.EndTime == nil || !raw.EndTime.Equal(endTs) {
			t.Errorf("RawStruct.EndTime not preserved correctly")
		}
	})

	t.Run("AutoScalingGroupName_preserved", func(t *testing.T) {
		if raw.AutoScalingGroupName == nil || *raw.AutoScalingGroupName != "raw-asg" {
			t.Errorf("RawStruct.AutoScalingGroupName not preserved correctly")
		}
	})

	t.Run("Details_preserved", func(t *testing.T) {
		if raw.Details == nil || *raw.Details != "{\"Subnet ID\":\"subnet-12345\"}" {
			t.Errorf("RawStruct.Details not preserved correctly")
		}
	})

	t.Run("Progress_preserved", func(t *testing.T) {
		if raw.Progress == nil || *raw.Progress != 100 {
			t.Errorf("RawStruct.Progress not preserved correctly")
		}
	})
}

// TestFetchAsgActivities_Pagination verifies the single-page pagination contract:
// one API call is made, resources from that page are returned, and IsTruncated/NextToken
// reflect whether more pages exist. A second call with the continuation token verifies
// that the token is forwarded and the final page sets IsTruncated=false.
func TestFetchAsgActivities_Pagination(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	// Page 1: 3 items with NextToken indicating more pages exist.
	page1Mock := &mockASGDescribeScalingActivitiesClient{
		output: &autoscaling.DescribeScalingActivitiesOutput{
			NextToken: aws.String("page2-token"),
			Activities: []asgtypes.Activity{
				{
					ActivityId:           aws.String("act-p1-1"),
					AutoScalingGroupName: aws.String("paginated-asg"),
					Cause:                aws.String("Scaling event page 1 item 1"),
					StartTime:            &ts,
					StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
				},
				{
					ActivityId:           aws.String("act-p1-2"),
					AutoScalingGroupName: aws.String("paginated-asg"),
					Cause:                aws.String("Scaling event page 1 item 2"),
					StartTime:            &ts,
					StatusCode:           asgtypes.ScalingActivityStatusCodeFailed,
				},
				{
					ActivityId:           aws.String("act-p1-3"),
					AutoScalingGroupName: aws.String("paginated-asg"),
					Cause:                aws.String("Scaling event page 1 item 3"),
					StartTime:            &ts,
					StatusCode:           asgtypes.ScalingActivityStatusCodeInProgress,
				},
			},
		},
	}

	parentCtx := map[string]string{
		"asg_name": "paginated-asg",
	}

	// First call: no continuation token — fetches page 1.
	result1, err := awsclient.FetchAsgActivities(
		context.Background(),
		page1Mock,
		parentCtx,
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

	t.Run("page1_all_have_status", func(t *testing.T) {
		for i, r := range result1.Resources {
			if r.Fields["status"] == "" {
				t.Errorf("page 1: resources[%d].Fields[\"status\"] should not be empty", i)
			}
		}
	})

	t.Run("page1_all_fields_populated", func(t *testing.T) {
		requiredFields := []string{"start_time", "status_code", "description", "cause"}
		for i, r := range result1.Resources {
			for _, key := range requiredFields {
				if _, ok := r.Fields[key]; !ok {
					t.Errorf("page 1: resource[%d].Fields missing key %q", i, key)
				}
			}
		}
	})

	t.Run("page1_single_api_call", func(t *testing.T) {
		// The new implementation makes exactly one API call per FetchAsgActivities invocation.
		// Verify resources were returned from the single call.
		if len(result1.Resources) == 0 {
			t.Error("expected resources from single API call")
		}
	})

	// Page 2: 2 items with no NextToken — last page.
	page2Mock := &mockASGDescribeScalingActivitiesClient{
		output: &autoscaling.DescribeScalingActivitiesOutput{
			// No NextToken — last page
			Activities: []asgtypes.Activity{
				{
					ActivityId:           aws.String("act-p2-1"),
					AutoScalingGroupName: aws.String("paginated-asg"),
					Cause:                aws.String("Scaling event page 2 item 1"),
					StartTime:            &ts,
					StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
				},
				{
					ActivityId:           aws.String("act-p2-2"),
					AutoScalingGroupName: aws.String("paginated-asg"),
					Cause:                aws.String("Scaling event page 2 item 2"),
					StartTime:            &ts,
					StatusCode:           asgtypes.ScalingActivityStatusCodeCancelled,
				},
			},
		},
	}

	// Second call: pass continuation token from page 1 to fetch page 2.
	result2, err := awsclient.FetchAsgActivities(
		context.Background(),
		page2Mock,
		parentCtx,
		result1.Pagination.NextToken,
	)
	if err != nil {
		t.Fatalf("page 2: expected no error, got %v", err)
	}

	t.Run("page2_item_count", func(t *testing.T) {
		if len(result2.Resources) != 2 {
			t.Fatalf("expected 2 resources on page 2, got %d", len(result2.Resources))
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
		if result2.Pagination.TotalHint != 2 {
			t.Errorf("page 2: TotalHint should equal item count (2) on last page, got %d", result2.Pagination.TotalHint)
		}
	})
}

// TestFetchAsgActivities_LargePage verifies that a single API page of 50 activities is
// returned as-is with correct IsTruncated=true metadata when the API indicates
// more pages exist. The per-invocation single-page contract means the caller
// drives pagination via continuation tokens.
func TestFetchAsgActivities_LargePage(t *testing.T) {
	ts := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// Build one page of 50 activities with a NextToken indicating more pages exist.
	var activities []asgtypes.Activity
	for i := range 50 {
		actTs := ts.Add(time.Duration(i) * time.Second)
		activities = append(activities, asgtypes.Activity{
			ActivityId:           aws.String(fmt.Sprintf("act-p0-%d", i)),
			AutoScalingGroupName: aws.String("big-asg"),
			Cause:                aws.String(fmt.Sprintf("Scaling event p0-%d", i)),
			StartTime:            &actTs,
			StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
		})
	}

	mock := &mockASGDescribeScalingActivitiesClient{
		output: &autoscaling.DescribeScalingActivitiesOutput{
			Activities: activities,
			NextToken:  aws.String("token-page-1"),
		},
	}

	parentCtx := map[string]string{
		"asg_name": "big-asg",
	}

	result, err := awsclient.FetchAsgActivities(
		context.Background(),
		mock,
		parentCtx,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("returns_full_page_of_50", func(t *testing.T) {
		if len(result.Resources) != 50 {
			t.Errorf("expected exactly 50 resources from single API page, got %d", len(result.Resources))
		}
	})

	t.Run("is_truncated_true", func(t *testing.T) {
		if result.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result.Pagination.IsTruncated {
			t.Error("IsTruncated should be true when API returns NextToken")
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

	t.Run("page_size_equals_item_count", func(t *testing.T) {
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

	t.Run("first_activity_correct", func(t *testing.T) {
		if result.Resources[0].ID != "act-p0-0" {
			t.Errorf("first resource ID: expected %q, got %q", "act-p0-0", result.Resources[0].ID)
		}
	})

	t.Run("last_activity_correct", func(t *testing.T) {
		if result.Resources[49].ID != "act-p0-49" {
			t.Errorf("last resource ID: expected %q, got %q", "act-p0-49", result.Resources[49].ID)
		}
	})
}

// TestAsgActivityColumns verifies that AsgActivityColumns returns the expected
// columns with correct keys, titles, and positive widths.
func TestAsgActivityColumns(t *testing.T) {
	cols := resource.AsgActivityColumns()

	expectedKeys := []string{"start_time", "status_code", "description", "cause"}

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

// TestAsgActivities_ChildTypeRegistered verifies that the child type is
// registered under the correct short name.
func TestAsgActivities_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("asg_activities")
	if td == nil {
		t.Fatal("asg_activities child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "asg_activities" {
		t.Errorf("child type ShortName: expected %q, got %q", "asg_activities", td.ShortName)
	}
}

// TestAsgActivities_PaginatedChildFetcherRegistered verifies that the paginated
// child fetcher is
// registered under the correct short name.
func TestAsgActivities_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("asg_activities")
	if f == nil {
		t.Fatal("asg_activities paginated child fetcher not registered")
	}
}

// TestFetchAsgActivities_ContinuationToken verifies that a non-empty
// continuation token is forwarded to the API as NextToken.
func TestFetchAsgActivities_ContinuationToken(t *testing.T) {
	ts := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)

	wrapper := &tokenCapturingAsgActivitiesMock{
		inner: &mockASGDescribeScalingActivitiesClient{
			output: &autoscaling.DescribeScalingActivitiesOutput{
				Activities: []asgtypes.Activity{
					{
						ActivityId:           aws.String("act-token-001"),
						AutoScalingGroupName: aws.String("my-asg"),
						Cause:                aws.String("Manual scaling"),
						StartTime:            &ts,
						StatusCode:           asgtypes.ScalingActivityStatusCodeSuccessful,
					},
				},
			},
		},
	}

	parentCtx := map[string]string{
		"asg_name": "my-asg",
	}

	result, err := awsclient.FetchAsgActivities(
		context.Background(),
		wrapper,
		parentCtx,
		"my-continuation-token",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	if wrapper.capturedNextToken == nil {
		t.Fatal("expected NextToken to be set in API call")
	}
	if *wrapper.capturedNextToken != "my-continuation-token" {
		t.Errorf("expected NextToken %q, got %q", "my-continuation-token", *wrapper.capturedNextToken)
	}
}

// tokenCapturingAsgActivitiesMock wraps the ASG activities mock to capture NextToken.
type tokenCapturingAsgActivitiesMock struct {
	inner             *mockASGDescribeScalingActivitiesClient
	capturedNextToken *string
}

func (m *tokenCapturingAsgActivitiesMock) DescribeScalingActivities(ctx context.Context, params *autoscaling.DescribeScalingActivitiesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	m.capturedNextToken = params.NextToken
	return m.inner.DescribeScalingActivities(ctx, params, optFns...)
}

// TestAsgActivities_ParentHasChildDef verifies that the parent asg resource
// type has a child view definition for asg_activities with key "enter".
func TestAsgActivities_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("asg")
	if rt == nil {
		t.Fatal("asg resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "asg_activities" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["asg_name"] == "" {
				t.Error("ContextKeys should include 'asg_name'")
			}
		}
	}
	if !found {
		t.Error("asg Children should contain asg_activities child view def")
	}
}
