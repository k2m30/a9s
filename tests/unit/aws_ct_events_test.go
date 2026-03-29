package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// capturingCloudTrailClient captures the LookupEventsInput for assertion.
type capturingCloudTrailClient struct {
	captured *cloudtrail.LookupEventsInput
	output   *cloudtrail.LookupEventsOutput
	err      error
}

func (m *capturingCloudTrailClient) LookupEvents(ctx context.Context, input *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	m.captured = input
	if m.output == nil {
		return &cloudtrail.LookupEventsOutput{}, m.err
	}
	return m.output, m.err
}

// ---------------------------------------------------------------------------
// CloudTrail Events fetcher tests
// ---------------------------------------------------------------------------

func TestFetchCloudTrailEvents_ParsesMultipleEvents(t *testing.T) {
	eventTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{
				{
					EventId:     aws.String("evt-0001-abcd-1234"),
					EventName:   aws.String("RunInstances"),
					EventTime:   &eventTime,
					EventSource: aws.String("ec2.amazonaws.com"),
					Username:    aws.String("admin"),
					ReadOnly:    aws.String("false"),
					Resources: []cloudtrailtypes.Resource{
						{
							ResourceType: aws.String("AWS::EC2::Instance"),
							ResourceName: aws.String("i-0abc123456def"),
						},
					},
				},
				{
					EventId:     aws.String("evt-0002-efgh-5678"),
					EventName:   aws.String("GetObject"),
					EventTime:   &eventTime,
					EventSource: aws.String("s3.amazonaws.com"),
					Username:    aws.String("readonly-user"),
					ReadOnly:    aws.String("true"),
					Resources:   []cloudtrailtypes.Resource{},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudTrailEvents(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first event
	r0 := resources[0]
	if r0.ID != "evt-0001-abcd-1234" {
		t.Errorf("resource[0].ID: expected %q, got %q", "evt-0001-abcd-1234", r0.ID)
	}
	if r0.Name != "RunInstances" {
		t.Errorf("resource[0].Name: expected %q, got %q", "RunInstances", r0.Name)
	}
	if r0.Status != "false" {
		t.Errorf("resource[0].Status: expected %q, got %q", "false", r0.Status)
	}

	// Verify second event (read-only, empty Resources)
	r1 := resources[1]
	if r1.ID != "evt-0002-efgh-5678" {
		t.Errorf("resource[1].ID: expected %q, got %q", "evt-0002-efgh-5678", r1.ID)
	}
	if r1.Name != "GetObject" {
		t.Errorf("resource[1].Name: expected %q, got %q", "GetObject", r1.Name)
	}
	if r1.Status != "true" {
		t.Errorf("resource[1].Status: expected %q, got %q", "true", r1.Status)
	}
}

func TestFetchCloudTrailEvents_EmptyResponse(t *testing.T) {
	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{},
		},
	}

	resources, err := awsclient.FetchCloudTrailEvents(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchCloudTrailEvents_ErrorResponse(t *testing.T) {
	mock := &mockCloudTrailLookupEventsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchCloudTrailEvents(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchCloudTrailEvents_FieldExtraction(t *testing.T) {
	eventTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{
				{
					EventId:     aws.String("evt-0001-abcd-1234"),
					EventName:   aws.String("RunInstances"),
					EventTime:   &eventTime,
					EventSource: aws.String("ec2.amazonaws.com"),
					Username:    aws.String("admin"),
					ReadOnly:    aws.String("false"),
					Resources: []cloudtrailtypes.Resource{
						{
							ResourceType: aws.String("AWS::EC2::Instance"),
							ResourceName: aws.String("i-0abc123456def"),
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudTrailEvents(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// Verify all FieldKeys are present and have exact values
	if r.Fields["event_name"] != "RunInstances" {
		t.Errorf("Fields[\"event_name\"]: expected %q, got %q", "RunInstances", r.Fields["event_name"])
	}
	if r.Fields["time"] != "2025-03-15 12:00:00" {
		t.Errorf("Fields[\"time\"]: expected %q, got %q", "2025-03-15 12:00:00", r.Fields["time"])
	}
	if r.Fields["user"] != "admin" {
		t.Errorf("Fields[\"user\"]: expected %q, got %q", "admin", r.Fields["user"])
	}
	if r.Fields["source"] != "ec2.amazonaws.com" {
		t.Errorf("Fields[\"source\"]: expected %q, got %q", "ec2.amazonaws.com", r.Fields["source"])
	}
	if r.Fields["resource_type"] != "AWS::EC2::Instance" {
		t.Errorf("Fields[\"resource_type\"]: expected %q, got %q", "AWS::EC2::Instance", r.Fields["resource_type"])
	}
	if r.Fields["resource_name"] != "i-0abc123456def" {
		t.Errorf("Fields[\"resource_name\"]: expected %q, got %q", "i-0abc123456def", r.Fields["resource_name"])
	}
	if r.Fields["read_only"] != "false" {
		t.Errorf("Fields[\"read_only\"]: expected %q, got %q", "false", r.Fields["read_only"])
	}
}

func TestFetchCloudTrailEvents_EmptyResources(t *testing.T) {
	// Event with empty Resources slice — resource_type and resource_name should be empty
	eventTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{
				{
					EventId:     aws.String("evt-no-resources"),
					EventName:   aws.String("ListBuckets"),
					EventTime:   &eventTime,
					EventSource: aws.String("s3.amazonaws.com"),
					Username:    aws.String("admin"),
					ReadOnly:    aws.String("true"),
					Resources:   []cloudtrailtypes.Resource{},
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudTrailEvents(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Fields["resource_type"] != "" {
		t.Errorf("Fields[\"resource_type\"]: expected empty string for event with no resources, got %q", r.Fields["resource_type"])
	}
	if r.Fields["resource_name"] != "" {
		t.Errorf("Fields[\"resource_name\"]: expected empty string for event with no resources, got %q", r.Fields["resource_name"])
	}
}

func TestFetchCloudTrailEvents_ReadOnlyIsString(t *testing.T) {
	// ReadOnly is *string ("true" or "false"), not *bool
	eventTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{
				{
					EventId:   aws.String("evt-readonly-true"),
					EventName: aws.String("DescribeInstances"),
					EventTime: &eventTime,
					ReadOnly:  aws.String("true"),
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudTrailEvents(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Fields["read_only"] != "true" {
		t.Errorf("Fields[\"read_only\"]: expected %q, got %q", "true", r.Fields["read_only"])
	}
	if r.Status != "true" {
		t.Errorf("Status: expected %q (read_only mapped to status), got %q", "true", r.Status)
	}
}

func TestFetchCloudTrailEvents_RawStructIsEvent(t *testing.T) {
	eventTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{
				{
					EventId:   aws.String("evt-rawstruct"),
					EventName: aws.String("CreateBucket"),
					EventTime: &eventTime,
				},
			},
		},
	}

	resources, err := awsclient.FetchCloudTrailEvents(context.Background(), mock)
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

	event, ok := r.RawStruct.(cloudtrailtypes.Event)
	if !ok {
		t.Fatalf("RawStruct should be cloudtrailtypes.Event, got %T", r.RawStruct)
	}
	if event.EventId == nil || *event.EventId != "evt-rawstruct" {
		t.Errorf("RawStruct.EventId: expected %q, got %v", "evt-rawstruct", event.EventId)
	}
}

// ---------------------------------------------------------------------------
// FetchCloudTrailEventsPage pagination tests
// ---------------------------------------------------------------------------

func TestFetchCloudTrailEventsPage_ReturnsPageWithPagination(t *testing.T) {
	eventTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{
				{
					EventId:   aws.String("evt-page1-0001"),
					EventName: aws.String("RunInstances"),
					EventTime: &eventTime,
				},
				{
					EventId:   aws.String("evt-page1-0002"),
					EventName: aws.String("StopInstances"),
					EventTime: &eventTime,
				},
			},
			NextToken: aws.String("token-page-2"),
		},
	}

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination to be non-nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated == true when NextToken is present")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 2 {
		t.Errorf("PageSize: expected 2, got %d", result.Pagination.PageSize)
	}
	if result.Pagination.TotalHint != -1 {
		t.Errorf("TotalHint: expected -1 (unknown), got %d", result.Pagination.TotalHint)
	}
}

func TestFetchCloudTrailEventsPage_LastPage(t *testing.T) {
	eventTime := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)

	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{
				{
					EventId:   aws.String("evt-last-page"),
					EventName: aws.String("DeleteBucket"),
					EventTime: &eventTime,
				},
			},
			NextToken: nil,
		},
	}

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), mock, "some-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination to be non-nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated == false on last page (nil NextToken)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string on last page, got %q", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
}

func TestFetchCloudTrailEventsPage_EmptyPage(t *testing.T) {
	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events:    []cloudtrailtypes.Event{},
			NextToken: nil,
		},
	}

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination to be non-nil even on empty page")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated == false on empty page")
	}
	if result.Pagination.PageSize != 0 {
		t.Errorf("PageSize: expected 0 for empty page, got %d", result.Pagination.PageSize)
	}
}

func TestFetchCloudTrailEventsPage_ContinuationTokenPassedToAPI(t *testing.T) {
	mock := &capturingCloudTrailClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{},
		},
	}

	_, err := awsclient.FetchCloudTrailEventsPage(context.Background(), mock, "my-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if mock.captured == nil {
		t.Fatal("expected LookupEventsInput to be captured by mock")
	}
	if mock.captured.NextToken == nil || *mock.captured.NextToken != "my-token" {
		t.Errorf("NextToken in API input: expected %q, got %v", "my-token", mock.captured.NextToken)
	}
	if mock.captured.MaxResults == nil || *mock.captured.MaxResults != 50 {
		t.Errorf("MaxResults in API input: expected 50, got %v", mock.captured.MaxResults)
	}
}

func TestFetchCloudTrailEventsPage_Error(t *testing.T) {
	mock := &mockCloudTrailLookupEventsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: throttled"),
	}

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected zero Resources on error, got %d", len(result.Resources))
	}
	if result.Pagination != nil {
		t.Errorf("expected nil Pagination on error, got %+v", result.Pagination)
	}
}

func TestFetchCloudTrailEventsPage_FieldExtraction(t *testing.T) {
	eventTime := time.Date(2025, 6, 20, 9, 30, 0, 0, time.UTC)

	mock := &mockCloudTrailLookupEventsClient{
		output: &cloudtrail.LookupEventsOutput{
			Events: []cloudtrailtypes.Event{
				{
					EventId:     aws.String("evt-fields-0001"),
					EventName:   aws.String("PutObject"),
					EventTime:   &eventTime,
					Username:    aws.String("deploy-bot"),
					EventSource: aws.String("s3.amazonaws.com"),
					ReadOnly:    aws.String("false"),
					Resources: []cloudtrailtypes.Resource{
						{
							ResourceType: aws.String("AWS::S3::Object"),
							ResourceName: aws.String("my-bucket/key.json"),
						},
					},
				},
			},
			NextToken: nil,
		},
	}

	result, err := awsclient.FetchCloudTrailEventsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	r := result.Resources[0]

	if r.ID != "evt-fields-0001" {
		t.Errorf("ID: expected %q, got %q", "evt-fields-0001", r.ID)
	}
	if r.Name != "PutObject" {
		t.Errorf("Name: expected %q, got %q", "PutObject", r.Name)
	}
	if r.Fields["event_name"] != "PutObject" {
		t.Errorf("Fields[\"event_name\"]: expected %q, got %q", "PutObject", r.Fields["event_name"])
	}
	if r.Fields["time"] != "2025-06-20 09:30:00" {
		t.Errorf("Fields[\"time\"]: expected %q, got %q", "2025-06-20 09:30:00", r.Fields["time"])
	}
	if r.Fields["user"] != "deploy-bot" {
		t.Errorf("Fields[\"user\"]: expected %q, got %q", "deploy-bot", r.Fields["user"])
	}
	if r.Fields["source"] != "s3.amazonaws.com" {
		t.Errorf("Fields[\"source\"]: expected %q, got %q", "s3.amazonaws.com", r.Fields["source"])
	}
	if r.Fields["resource_type"] != "AWS::S3::Object" {
		t.Errorf("Fields[\"resource_type\"]: expected %q, got %q", "AWS::S3::Object", r.Fields["resource_type"])
	}
	if r.Fields["resource_name"] != "my-bucket/key.json" {
		t.Errorf("Fields[\"resource_name\"]: expected %q, got %q", "my-bucket/key.json", r.Fields["resource_name"])
	}
	if r.Fields["read_only"] != "false" {
		t.Errorf("Fields[\"read_only\"]: expected %q, got %q", "false", r.Fields["read_only"])
	}
}
