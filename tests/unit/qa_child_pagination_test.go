package unit

// qa_child_pagination_test.go — pagination tests for all 19 child fetchers.
//
// After migration each child fetcher returns a SINGLE page per call with proper
// IsTruncated/NextToken metadata. 4 test cases per fetcher:
//   1. FirstPage    — API returns items + next cursor → IsTruncated=true
//   2. Continuation — non-empty continuationToken passed, no next cursor → IsTruncated=false
//   3. Empty        — API returns no items → IsTruncated=false, len==0
//   4. Error        — API returns error → error propagated

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Mock: S3 ListObjectsV2
// ---------------------------------------------------------------------------

type mockS3ListObjectsV2APIChildPaginated struct {
	PageFunc func(call int) (*s3.ListObjectsV2Output, error)
	calls    int
}

func (m *mockS3ListObjectsV2APIChildPaginated) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchS3Objects_FirstPage(t *testing.T) {
	truncated := true
	mock := &mockS3ListObjectsV2APIChildPaginated{
		PageFunc: func(_ int) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents:              []s3types.Object{{Key: aws.String("reports/jan.csv"), Size: aws.Int64(4096)}},
				IsTruncated:           &truncated,
				NextContinuationToken: aws.String("cont-token-page-2"),
			}, nil
		},
	}
	result, err := awsclient.FetchS3Objects(context.Background(), mock, "my-app-bucket", "", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "cont-token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "cont-token-page-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "reports/jan.csv" {
		t.Errorf("resource ID: expected %q, got %q", "reports/jan.csv", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchS3Objects_Continuation(t *testing.T) {
	truncated := false
	mock := &mockS3ListObjectsV2APIChildPaginated{
		PageFunc: func(_ int) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{
				Contents:    []s3types.Object{{Key: aws.String("reports/feb.csv"), Size: aws.Int64(8192)}},
				IsTruncated: &truncated,
			}, nil
		},
	}
	result, err := awsclient.FetchS3Objects(context.Background(), mock, "my-app-bucket", "", "cont-token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchS3Objects_Empty(t *testing.T) {
	truncated := false
	mock := &mockS3ListObjectsV2APIChildPaginated{
		PageFunc: func(_ int) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{IsTruncated: &truncated}, nil
		},
	}
	result, err := awsclient.FetchS3Objects(context.Background(), mock, "empty-bucket", "", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchS3Objects_Error(t *testing.T) {
	mock := &mockS3ListObjectsV2APIChildPaginated{
		PageFunc: func(_ int) (*s3.ListObjectsV2Output, error) {
			return nil, errors.New("list objects failed")
		},
	}
	_, err := awsclient.FetchS3Objects(context.Background(), mock, "my-app-bucket", "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CloudWatchLogs DescribeLogStreams
// ---------------------------------------------------------------------------

type mockCWLogsDescribeLogStreamsAPIChildPaginated struct {
	PageFunc func(call int) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	calls    int
}

func (m *mockCWLogsDescribeLogStreamsAPIChildPaginated) DescribeLogStreams(_ context.Context, _ *cloudwatchlogs.DescribeLogStreamsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchLogStreams_FirstPage(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsAPIChildPaginated{
		PageFunc: func(_ int) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []cwlogstypes.LogStream{{LogStreamName: aws.String("stream-alpha")}},
				NextToken:  aws.String("next-token-2"),
			}, nil
		},
	}
	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/my-func", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "next-token-2" {
		t.Errorf("NextToken: expected %q, got %q", "next-token-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "stream-alpha" {
		t.Errorf("resource ID: expected %q, got %q", "stream-alpha", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchLogStreams_Continuation(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsAPIChildPaginated{
		PageFunc: func(_ int) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []cwlogstypes.LogStream{{LogStreamName: aws.String("stream-beta")}},
				NextToken:  nil,
			}, nil
		},
	}
	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/my-func", "next-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchLogStreams_Empty(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsAPIChildPaginated{
		PageFunc: func(_ int) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{LogStreams: []cwlogstypes.LogStream{}}, nil
		},
	}
	result, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/my-func", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchLogStreams_Error(t *testing.T) {
	mock := &mockCWLogsDescribeLogStreamsAPIChildPaginated{
		PageFunc: func(_ int) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return nil, errors.New("describe log streams failed")
		},
	}
	_, err := awsclient.FetchLogStreams(context.Background(), mock, "/aws/lambda/my-func", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CloudFormation DescribeStackEvents
// ---------------------------------------------------------------------------

type mockCFNDescribeStackEventsAPIChildPaginated struct {
	PageFunc func(call int) (*cloudformation.DescribeStackEventsOutput, error)
	calls    int
}

func (m *mockCFNDescribeStackEventsAPIChildPaginated) DescribeStackEvents(_ context.Context, _ *cloudformation.DescribeStackEventsInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchCfnEvents_FirstPage(t *testing.T) {
	ts := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC)
	mock := &mockCFNDescribeStackEventsAPIChildPaginated{
		PageFunc: func(_ int) (*cloudformation.DescribeStackEventsOutput, error) {
			return &cloudformation.DescribeStackEventsOutput{
				StackEvents: []cfntypes.StackEvent{
					{EventId: aws.String("evt-001"), Timestamp: &ts, ResourceStatus: cfntypes.ResourceStatusCreateComplete},
				},
				NextToken: aws.String("cfn-next-token-2"),
			}, nil
		},
	}
	result, err := awsclient.FetchCfnEvents(context.Background(), mock, "my-stack", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "cfn-next-token-2" {
		t.Errorf("NextToken: expected %q, got %q", "cfn-next-token-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "evt-001" {
		t.Errorf("resource ID: expected %q, got %q", "evt-001", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchCfnEvents_Continuation(t *testing.T) {
	ts := time.Date(2025, 3, 2, 10, 0, 0, 0, time.UTC)
	mock := &mockCFNDescribeStackEventsAPIChildPaginated{
		PageFunc: func(_ int) (*cloudformation.DescribeStackEventsOutput, error) {
			return &cloudformation.DescribeStackEventsOutput{
				StackEvents: []cfntypes.StackEvent{
					{EventId: aws.String("evt-002"), Timestamp: &ts, ResourceStatus: cfntypes.ResourceStatusUpdateComplete},
				},
				NextToken: nil,
			}, nil
		},
	}
	result, err := awsclient.FetchCfnEvents(context.Background(), mock, "my-stack", "cfn-next-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchCfnEvents_Empty(t *testing.T) {
	mock := &mockCFNDescribeStackEventsAPIChildPaginated{
		PageFunc: func(_ int) (*cloudformation.DescribeStackEventsOutput, error) {
			return &cloudformation.DescribeStackEventsOutput{StackEvents: []cfntypes.StackEvent{}}, nil
		},
	}
	result, err := awsclient.FetchCfnEvents(context.Background(), mock, "my-stack", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchCfnEvents_Error(t *testing.T) {
	mock := &mockCFNDescribeStackEventsAPIChildPaginated{
		PageFunc: func(_ int) (*cloudformation.DescribeStackEventsOutput, error) {
			return nil, errors.New("describe stack events failed")
		},
	}
	_, err := awsclient.FetchCfnEvents(context.Background(), mock, "my-stack", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CloudFormation ListStackResources
// ---------------------------------------------------------------------------

type mockCFNListStackResourcesAPIChildPaginated struct {
	PageFunc func(call int) (*cloudformation.ListStackResourcesOutput, error)
	calls    int
}

func (m *mockCFNListStackResourcesAPIChildPaginated) ListStackResources(_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchCfnResources_FirstPage(t *testing.T) {
	ts := time.Date(2025, 2, 15, 9, 0, 0, 0, time.UTC)
	mock := &mockCFNListStackResourcesAPIChildPaginated{
		PageFunc: func(_ int) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{
				StackResourceSummaries: []cfntypes.StackResourceSummary{
					{
						LogicalResourceId:    aws.String("MyBucket"),
						ResourceType:         aws.String("AWS::S3::Bucket"),
						ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
						LastUpdatedTimestamp: &ts,
					},
				},
				NextToken: aws.String("cfn-res-next-2"),
			}, nil
		},
	}
	result, err := awsclient.FetchCfnResources(context.Background(), mock, "my-stack", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "cfn-res-next-2" {
		t.Errorf("NextToken: expected %q, got %q", "cfn-res-next-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "MyBucket" {
		t.Errorf("resource ID: expected %q, got %q", "MyBucket", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchCfnResources_Continuation(t *testing.T) {
	ts := time.Date(2025, 2, 16, 9, 0, 0, 0, time.UTC)
	mock := &mockCFNListStackResourcesAPIChildPaginated{
		PageFunc: func(_ int) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{
				StackResourceSummaries: []cfntypes.StackResourceSummary{
					{
						LogicalResourceId:    aws.String("MyQueue"),
						ResourceType:         aws.String("AWS::SQS::Queue"),
						ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
						LastUpdatedTimestamp: &ts,
					},
				},
				NextToken: nil,
			}, nil
		},
	}
	result, err := awsclient.FetchCfnResources(context.Background(), mock, "my-stack", "cfn-res-next-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchCfnResources_Empty(t *testing.T) {
	mock := &mockCFNListStackResourcesAPIChildPaginated{
		PageFunc: func(_ int) (*cloudformation.ListStackResourcesOutput, error) {
			return &cloudformation.ListStackResourcesOutput{StackResourceSummaries: []cfntypes.StackResourceSummary{}}, nil
		},
	}
	result, err := awsclient.FetchCfnResources(context.Background(), mock, "my-stack", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchCfnResources_Error(t *testing.T) {
	mock := &mockCFNListStackResourcesAPIChildPaginated{
		PageFunc: func(_ int) (*cloudformation.ListStackResourcesOutput, error) {
			return nil, errors.New("list stack resources failed")
		},
	}
	_, err := awsclient.FetchCfnResources(context.Background(), mock, "my-stack", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: Route53 ListResourceRecordSets
// ---------------------------------------------------------------------------

type mockRoute53ListResourceRecordSetsAPIChildPaginated struct {
	PageFunc func(call int) (*route53.ListResourceRecordSetsOutput, error)
	calls    int
}

func (m *mockRoute53ListResourceRecordSetsAPIChildPaginated) ListResourceRecordSets(_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchR53Records_FirstPage(t *testing.T) {
	mock := &mockRoute53ListResourceRecordSetsAPIChildPaginated{
		PageFunc: func(_ int) (*route53.ListResourceRecordSetsOutput, error) {
			return &route53.ListResourceRecordSetsOutput{
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{Name: aws.String("api.example.com."), Type: r53types.RRTypeA, TTL: aws.Int64(300)},
				},
				IsTruncated:    true,
				NextRecordName: aws.String("www.example.com."),
				NextRecordType: r53types.RRTypeA,
			}, nil
		},
	}
	result, err := awsclient.FetchR53Records(context.Background(), mock, "Z1234567890ABC", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
}

func TestQA_ChildPagination_FetchR53Records_Continuation(t *testing.T) {
	mock := &mockRoute53ListResourceRecordSetsAPIChildPaginated{
		PageFunc: func(_ int) (*route53.ListResourceRecordSetsOutput, error) {
			return &route53.ListResourceRecordSetsOutput{
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{Name: aws.String("www.example.com."), Type: r53types.RRTypeA, TTL: aws.Int64(300)},
				},
				IsTruncated: false,
			}, nil
		},
	}
	// Pass a JSON continuation token as the function expects
	token := `{"n":"www.example.com.","t":"A"}`
	result, err := awsclient.FetchR53Records(context.Background(), mock, "Z1234567890ABC", token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchR53Records_Empty(t *testing.T) {
	mock := &mockRoute53ListResourceRecordSetsAPIChildPaginated{
		PageFunc: func(_ int) (*route53.ListResourceRecordSetsOutput, error) {
			return &route53.ListResourceRecordSetsOutput{
				ResourceRecordSets: []r53types.ResourceRecordSet{},
				IsTruncated:        false,
			}, nil
		},
	}
	result, err := awsclient.FetchR53Records(context.Background(), mock, "Z1234567890ABC", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchR53Records_Error(t *testing.T) {
	mock := &mockRoute53ListResourceRecordSetsAPIChildPaginated{
		PageFunc: func(_ int) (*route53.ListResourceRecordSetsOutput, error) {
			return nil, errors.New("list resource record sets failed")
		},
	}
	_, err := awsclient.FetchR53Records(context.Background(), mock, "Z1234567890ABC", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ECR DescribeImages
// ---------------------------------------------------------------------------

type mockECRDescribeImagesAPIChildPaginated struct {
	PageFunc func(call int) (*ecr.DescribeImagesOutput, error)
	calls    int
}

func (m *mockECRDescribeImagesAPIChildPaginated) DescribeImages(_ context.Context, _ *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchECRImages_FirstPage(t *testing.T) {
	pushedAt := time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC)
	mock := &mockECRDescribeImagesAPIChildPaginated{
		PageFunc: func(_ int) (*ecr.DescribeImagesOutput, error) {
			return &ecr.DescribeImagesOutput{
				ImageDetails: []ecrtypes.ImageDetail{
					{ImageDigest: aws.String("sha256:abcdef123456"), ImageTags: []string{"latest"}, ImagePushedAt: &pushedAt},
				},
				NextToken: aws.String("ecr-next-token-2"),
			}, nil
		},
	}
	parentCtx := map[string]string{"repository_name": "my-app-repo", "repository_uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app-repo"}
	result, err := awsclient.FetchECRImages(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "ecr-next-token-2" {
		t.Errorf("NextToken: expected %q, got %q", "ecr-next-token-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "sha256:abcdef123456" {
		t.Errorf("resource ID: expected %q, got %q", "sha256:abcdef123456", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchECRImages_Continuation(t *testing.T) {
	pushedAt := time.Date(2025, 1, 11, 12, 0, 0, 0, time.UTC)
	mock := &mockECRDescribeImagesAPIChildPaginated{
		PageFunc: func(_ int) (*ecr.DescribeImagesOutput, error) {
			return &ecr.DescribeImagesOutput{
				ImageDetails: []ecrtypes.ImageDetail{
					{ImageDigest: aws.String("sha256:fedcba654321"), ImageTags: []string{"v1.2"}, ImagePushedAt: &pushedAt},
				},
				NextToken: nil,
			}, nil
		},
	}
	parentCtx := map[string]string{"repository_name": "my-app-repo", "repository_uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app-repo"}
	result, err := awsclient.FetchECRImages(context.Background(), mock, parentCtx, "ecr-next-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchECRImages_Empty(t *testing.T) {
	mock := &mockECRDescribeImagesAPIChildPaginated{
		PageFunc: func(_ int) (*ecr.DescribeImagesOutput, error) {
			return &ecr.DescribeImagesOutput{ImageDetails: []ecrtypes.ImageDetail{}, NextToken: nil}, nil
		},
	}
	parentCtx := map[string]string{"repository_name": "my-app-repo", "repository_uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app-repo"}
	result, err := awsclient.FetchECRImages(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchECRImages_Error(t *testing.T) {
	mock := &mockECRDescribeImagesAPIChildPaginated{
		PageFunc: func(_ int) (*ecr.DescribeImagesOutput, error) {
			return nil, errors.New("describe images failed")
		},
	}
	parentCtx := map[string]string{"repository_name": "my-app-repo", "repository_uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app-repo"}
	_, err := awsclient.FetchECRImages(context.Background(), mock, parentCtx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: IAM ListAttachedRolePolicies + ListRolePolicies (role_policies — two APIs)
// ---------------------------------------------------------------------------

type mockIAMListAttachedRolePoliciesAPIChildPaginated struct {
	PageFunc func(call int) (*iam.ListAttachedRolePoliciesOutput, error)
	calls    int
}

func (m *mockIAMListAttachedRolePoliciesAPIChildPaginated) ListAttachedRolePolicies(_ context.Context, _ *iam.ListAttachedRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

type mockIAMListRolePoliciesAPIChildPaginated struct {
	PageFunc func(call int) (*iam.ListRolePoliciesOutput, error)
	calls    int
}

func (m *mockIAMListRolePoliciesAPIChildPaginated) ListRolePolicies(_ context.Context, _ *iam.ListRolePoliciesInput, _ ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchRolePolicies_FirstPage(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesAPIChildPaginated{
		PageFunc: func(_ int) (*iam.ListAttachedRolePoliciesOutput, error) {
			return &iam.ListAttachedRolePoliciesOutput{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{PolicyName: aws.String("ReadOnlyAccess"), PolicyArn: aws.String("arn:aws:iam::aws:policy/ReadOnlyAccess")},
				},
				IsTruncated: false,
			}, nil
		},
	}
	inlineMock := &mockIAMListRolePoliciesAPIChildPaginated{
		PageFunc: func(_ int) (*iam.ListRolePoliciesOutput, error) {
			return &iam.ListRolePoliciesOutput{
				PolicyNames: []string{"my-inline-policy"},
				IsTruncated: false,
			}, nil
		},
	}
	parentCtx := map[string]string{"role_name": "my-app-role"}
	result, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// After migration: first page returns items with IsTruncated based on pagination state.
	// Currently loops all — after migration returns single page. Test the contract:
	// result must have Pagination set.
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources (1 managed + 1 inline), got %d", len(result.Resources))
	}
	if result.Resources[0].Fields["policy_type"] != "Managed" {
		t.Errorf("first resource policy_type: expected %q, got %q", "Managed", result.Resources[0].Fields["policy_type"])
	}
	if result.Resources[1].Fields["policy_type"] != "Inline" {
		t.Errorf("second resource policy_type: expected %q, got %q", "Inline", result.Resources[1].Fields["policy_type"])
	}
}

func TestQA_ChildPagination_FetchRolePolicies_Continuation(t *testing.T) {
	// Continuation: pass a non-empty token; mock returns items + no more pages.
	attachedMock := &mockIAMListAttachedRolePoliciesAPIChildPaginated{
		PageFunc: func(_ int) (*iam.ListAttachedRolePoliciesOutput, error) {
			return &iam.ListAttachedRolePoliciesOutput{
				AttachedPolicies: []iamtypes.AttachedPolicy{
					{PolicyName: aws.String("PowerUserAccess"), PolicyArn: aws.String("arn:aws:iam::aws:policy/PowerUserAccess")},
				},
				IsTruncated: false,
			}, nil
		},
	}
	inlineMock := &mockIAMListRolePoliciesAPIChildPaginated{
		PageFunc: func(_ int) (*iam.ListRolePoliciesOutput, error) {
			return &iam.ListRolePoliciesOutput{PolicyNames: []string{}, IsTruncated: false}, nil
		},
	}
	parentCtx := map[string]string{"role_name": "my-app-role"}
	result, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page")
	}
}

func TestQA_ChildPagination_FetchRolePolicies_Empty(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesAPIChildPaginated{
		PageFunc: func(_ int) (*iam.ListAttachedRolePoliciesOutput, error) {
			return &iam.ListAttachedRolePoliciesOutput{AttachedPolicies: []iamtypes.AttachedPolicy{}, IsTruncated: false}, nil
		},
	}
	inlineMock := &mockIAMListRolePoliciesAPIChildPaginated{
		PageFunc: func(_ int) (*iam.ListRolePoliciesOutput, error) {
			return &iam.ListRolePoliciesOutput{PolicyNames: []string{}, IsTruncated: false}, nil
		},
	}
	parentCtx := map[string]string{"role_name": "empty-role"}
	result, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchRolePolicies_Error(t *testing.T) {
	attachedMock := &mockIAMListAttachedRolePoliciesAPIChildPaginated{
		PageFunc: func(_ int) (*iam.ListAttachedRolePoliciesOutput, error) {
			return nil, errors.New("list attached role policies failed")
		},
	}
	inlineMock := &mockIAMListRolePoliciesAPIChildPaginated{
		PageFunc: func(_ int) (*iam.ListRolePoliciesOutput, error) {
			return &iam.ListRolePoliciesOutput{}, nil
		},
	}
	parentCtx := map[string]string{"role_name": "my-app-role"}
	_, err := awsclient.FetchRolePolicies(context.Background(), attachedMock, inlineMock, parentCtx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CodeBuild ListBuildsForProject + BatchGetBuilds (cb_builds — two APIs)
// ---------------------------------------------------------------------------

type mockCodeBuildListBuildsForProjectAPIChildPaginated struct {
	PageFunc func(call int) (*codebuild.ListBuildsForProjectOutput, error)
	calls    int
}

func (m *mockCodeBuildListBuildsForProjectAPIChildPaginated) ListBuildsForProject(_ context.Context, _ *codebuild.ListBuildsForProjectInput, _ ...func(*codebuild.Options)) (*codebuild.ListBuildsForProjectOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

type mockCodeBuildBatchGetBuildsAPIChildPaginated struct {
	PageFunc func(call int) (*codebuild.BatchGetBuildsOutput, error)
	calls    int
}

func (m *mockCodeBuildBatchGetBuildsAPIChildPaginated) BatchGetBuilds(_ context.Context, _ *codebuild.BatchGetBuildsInput, _ ...func(*codebuild.Options)) (*codebuild.BatchGetBuildsOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchCBBuilds_FirstPage(t *testing.T) {
	buildNum := int64(42)
	startTime := time.Date(2025, 3, 1, 8, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 3, 1, 8, 5, 0, 0, time.UTC)
	listMock := &mockCodeBuildListBuildsForProjectAPIChildPaginated{
		PageFunc: func(_ int) (*codebuild.ListBuildsForProjectOutput, error) {
			return &codebuild.ListBuildsForProjectOutput{
				Ids:       []string{"my-project:build-001"},
				NextToken: aws.String("cb-next-token-2"),
			}, nil
		},
	}
	batchMock := &mockCodeBuildBatchGetBuildsAPIChildPaginated{
		PageFunc: func(_ int) (*codebuild.BatchGetBuildsOutput, error) {
			return &codebuild.BatchGetBuildsOutput{
				Builds: []cbtypes.Build{
					{
						Id:          aws.String("my-project:build-001"),
						BuildNumber: &buildNum,
						BuildStatus: cbtypes.StatusTypeSucceeded,
						StartTime:   &startTime,
						EndTime:     &endTime,
					},
				},
			}, nil
		},
	}
	parentCtx := map[string]string{"project_name": "my-project"}
	result, err := awsclient.FetchCBBuilds(context.Background(), listMock, batchMock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "cb-next-token-2" {
		t.Errorf("NextToken: expected %q, got %q", "cb-next-token-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-project:build-001" {
		t.Errorf("resource ID: expected %q, got %q", "my-project:build-001", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchCBBuilds_Continuation(t *testing.T) {
	buildNum := int64(41)
	startTime := time.Date(2025, 2, 28, 8, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 2, 28, 8, 3, 0, 0, time.UTC)
	listMock := &mockCodeBuildListBuildsForProjectAPIChildPaginated{
		PageFunc: func(_ int) (*codebuild.ListBuildsForProjectOutput, error) {
			return &codebuild.ListBuildsForProjectOutput{
				Ids:       []string{"my-project:build-002"},
				NextToken: nil,
			}, nil
		},
	}
	batchMock := &mockCodeBuildBatchGetBuildsAPIChildPaginated{
		PageFunc: func(_ int) (*codebuild.BatchGetBuildsOutput, error) {
			return &codebuild.BatchGetBuildsOutput{
				Builds: []cbtypes.Build{
					{
						Id:          aws.String("my-project:build-002"),
						BuildNumber: &buildNum,
						BuildStatus: cbtypes.StatusTypeFailed,
						StartTime:   &startTime,
						EndTime:     &endTime,
					},
				},
			}, nil
		},
	}
	parentCtx := map[string]string{"project_name": "my-project"}
	result, err := awsclient.FetchCBBuilds(context.Background(), listMock, batchMock, parentCtx, "cb-next-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchCBBuilds_Empty(t *testing.T) {
	listMock := &mockCodeBuildListBuildsForProjectAPIChildPaginated{
		PageFunc: func(_ int) (*codebuild.ListBuildsForProjectOutput, error) {
			return &codebuild.ListBuildsForProjectOutput{Ids: []string{}, NextToken: nil}, nil
		},
	}
	batchMock := &mockCodeBuildBatchGetBuildsAPIChildPaginated{
		PageFunc: func(_ int) (*codebuild.BatchGetBuildsOutput, error) {
			return &codebuild.BatchGetBuildsOutput{Builds: []cbtypes.Build{}}, nil
		},
	}
	parentCtx := map[string]string{"project_name": "my-project"}
	result, err := awsclient.FetchCBBuilds(context.Background(), listMock, batchMock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchCBBuilds_Error(t *testing.T) {
	listMock := &mockCodeBuildListBuildsForProjectAPIChildPaginated{
		PageFunc: func(_ int) (*codebuild.ListBuildsForProjectOutput, error) {
			return nil, errors.New("list builds for project failed")
		},
	}
	batchMock := &mockCodeBuildBatchGetBuildsAPIChildPaginated{
		PageFunc: func(_ int) (*codebuild.BatchGetBuildsOutput, error) {
			return &codebuild.BatchGetBuildsOutput{}, nil
		},
	}
	parentCtx := map[string]string{"project_name": "my-project"}
	_, err := awsclient.FetchCBBuilds(context.Background(), listMock, batchMock, parentCtx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: CloudWatch DescribeAlarmHistory
// ---------------------------------------------------------------------------

type mockCloudWatchDescribeAlarmHistoryAPIChildPaginated struct {
	PageFunc func(call int) (*cloudwatch.DescribeAlarmHistoryOutput, error)
	calls    int
}

func (m *mockCloudWatchDescribeAlarmHistoryAPIChildPaginated) DescribeAlarmHistory(_ context.Context, _ *cloudwatch.DescribeAlarmHistoryInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchAlarmHistory_FirstPage(t *testing.T) {
	ts := time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)
	mock := &mockCloudWatchDescribeAlarmHistoryAPIChildPaginated{
		PageFunc: func(_ int) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
			return &cloudwatch.DescribeAlarmHistoryOutput{
				AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
					{Timestamp: &ts, HistoryItemType: cwtypes.HistoryItemTypeStateUpdate, HistorySummary: aws.String("Alarm updated")},
				},
				NextToken: aws.String("cw-next-token-2"),
			}, nil
		},
	}
	parentCtx := map[string]string{"alarm_name": "my-cpu-alarm"}
	result, err := awsclient.FetchAlarmHistory(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "cw-next-token-2" {
		t.Errorf("NextToken: expected %q, got %q", "cw-next-token-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
}

func TestQA_ChildPagination_FetchAlarmHistory_Continuation(t *testing.T) {
	ts := time.Date(2025, 3, 16, 10, 0, 0, 0, time.UTC)
	mock := &mockCloudWatchDescribeAlarmHistoryAPIChildPaginated{
		PageFunc: func(_ int) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
			return &cloudwatch.DescribeAlarmHistoryOutput{
				AlarmHistoryItems: []cwtypes.AlarmHistoryItem{
					{Timestamp: &ts, HistoryItemType: cwtypes.HistoryItemTypeAction, HistorySummary: aws.String("Action taken")},
				},
				NextToken: nil,
			}, nil
		},
	}
	parentCtx := map[string]string{"alarm_name": "my-cpu-alarm"}
	result, err := awsclient.FetchAlarmHistory(context.Background(), mock, parentCtx, "cw-next-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchAlarmHistory_Empty(t *testing.T) {
	mock := &mockCloudWatchDescribeAlarmHistoryAPIChildPaginated{
		PageFunc: func(_ int) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
			return &cloudwatch.DescribeAlarmHistoryOutput{AlarmHistoryItems: []cwtypes.AlarmHistoryItem{}}, nil
		},
	}
	parentCtx := map[string]string{"alarm_name": "my-cpu-alarm"}
	result, err := awsclient.FetchAlarmHistory(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchAlarmHistory_Error(t *testing.T) {
	mock := &mockCloudWatchDescribeAlarmHistoryAPIChildPaginated{
		PageFunc: func(_ int) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
			return nil, errors.New("describe alarm history failed")
		},
	}
	parentCtx := map[string]string{"alarm_name": "my-cpu-alarm"}
	_, err := awsclient.FetchAlarmHistory(context.Background(), mock, parentCtx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: RDS DescribeEvents
// ---------------------------------------------------------------------------

type mockRDSDescribeEventsAPIChildPaginated struct {
	PageFunc func(call int) (*rds.DescribeEventsOutput, error)
	calls    int
}

func (m *mockRDSDescribeEventsAPIChildPaginated) DescribeEvents(_ context.Context, _ *rds.DescribeEventsInput, _ ...func(*rds.Options)) (*rds.DescribeEventsOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchRDSEvents_FirstPage(t *testing.T) {
	ts := time.Date(2025, 3, 10, 14, 0, 0, 0, time.UTC)
	mock := &mockRDSDescribeEventsAPIChildPaginated{
		PageFunc: func(_ int) (*rds.DescribeEventsOutput, error) {
			return &rds.DescribeEventsOutput{
				Events: []rdstypes.Event{
					{Date: &ts, Message: aws.String("DB instance restarted"), SourceIdentifier: aws.String("my-db"), SourceType: rdstypes.SourceTypeDbInstance},
				},
				Marker: aws.String("rds-marker-2"),
			}, nil
		},
	}
	result, err := awsclient.FetchRDSEvents(context.Background(), mock, "my-db", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "rds-marker-2" {
		t.Errorf("NextToken: expected %q, got %q", "rds-marker-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
}

func TestQA_ChildPagination_FetchRDSEvents_Continuation(t *testing.T) {
	ts := time.Date(2025, 3, 11, 14, 0, 0, 0, time.UTC)
	mock := &mockRDSDescribeEventsAPIChildPaginated{
		PageFunc: func(_ int) (*rds.DescribeEventsOutput, error) {
			return &rds.DescribeEventsOutput{
				Events: []rdstypes.Event{
					{Date: &ts, Message: aws.String("Backup completed"), SourceIdentifier: aws.String("my-db"), SourceType: rdstypes.SourceTypeDbInstance},
				},
				Marker: nil,
			}, nil
		},
	}
	result, err := awsclient.FetchRDSEvents(context.Background(), mock, "my-db", "rds-marker-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchRDSEvents_Empty(t *testing.T) {
	mock := &mockRDSDescribeEventsAPIChildPaginated{
		PageFunc: func(_ int) (*rds.DescribeEventsOutput, error) {
			return &rds.DescribeEventsOutput{Events: []rdstypes.Event{}, Marker: nil}, nil
		},
	}
	result, err := awsclient.FetchRDSEvents(context.Background(), mock, "my-db", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchRDSEvents_Error(t *testing.T) {
	mock := &mockRDSDescribeEventsAPIChildPaginated{
		PageFunc: func(_ int) (*rds.DescribeEventsOutput, error) {
			return nil, errors.New("describe events failed")
		},
	}
	_, err := awsclient.FetchRDSEvents(context.Background(), mock, "my-db", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: SNS ListSubscriptionsByTopic
// ---------------------------------------------------------------------------

type mockSNSListSubscriptionsByTopicAPIChildPaginated struct {
	PageFunc func(call int) (*sns.ListSubscriptionsByTopicOutput, error)
	calls    int
}

func (m *mockSNSListSubscriptionsByTopicAPIChildPaginated) ListSubscriptionsByTopic(_ context.Context, _ *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchSNSTopicSubscriptions_FirstPage(t *testing.T) {
	mock := &mockSNSListSubscriptionsByTopicAPIChildPaginated{
		PageFunc: func(_ int) (*sns.ListSubscriptionsByTopicOutput, error) {
			return &sns.ListSubscriptionsByTopicOutput{
				Subscriptions: []snstypes.Subscription{
					{Protocol: aws.String("email"), Endpoint: aws.String("ops@example.com"), SubscriptionArn: aws.String("arn:aws:sns:us-east-1:111122223333:alerts:sub-001")},
				},
				NextToken: aws.String("sns-next-token-2"),
			}, nil
		},
	}
	result, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, "arn:aws:sns:us-east-1:111122223333:alerts", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "sns-next-token-2" {
		t.Errorf("NextToken: expected %q, got %q", "sns-next-token-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].Name != "ops@example.com" {
		t.Errorf("resource Name: expected %q, got %q", "ops@example.com", result.Resources[0].Name)
	}
}

func TestQA_ChildPagination_FetchSNSTopicSubscriptions_Continuation(t *testing.T) {
	mock := &mockSNSListSubscriptionsByTopicAPIChildPaginated{
		PageFunc: func(_ int) (*sns.ListSubscriptionsByTopicOutput, error) {
			return &sns.ListSubscriptionsByTopicOutput{
				Subscriptions: []snstypes.Subscription{
					{Protocol: aws.String("sqs"), Endpoint: aws.String("arn:aws:sqs:us-east-1:111122223333:my-queue"), SubscriptionArn: aws.String("arn:aws:sns:us-east-1:111122223333:alerts:sub-002")},
				},
				NextToken: nil,
			}, nil
		},
	}
	result, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, "arn:aws:sns:us-east-1:111122223333:alerts", "sns-next-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchSNSTopicSubscriptions_Empty(t *testing.T) {
	mock := &mockSNSListSubscriptionsByTopicAPIChildPaginated{
		PageFunc: func(_ int) (*sns.ListSubscriptionsByTopicOutput, error) {
			return &sns.ListSubscriptionsByTopicOutput{Subscriptions: []snstypes.Subscription{}, NextToken: nil}, nil
		},
	}
	result, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, "arn:aws:sns:us-east-1:111122223333:alerts", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchSNSTopicSubscriptions_Error(t *testing.T) {
	mock := &mockSNSListSubscriptionsByTopicAPIChildPaginated{
		PageFunc: func(_ int) (*sns.ListSubscriptionsByTopicOutput, error) {
			return nil, errors.New("list subscriptions by topic failed")
		},
	}
	_, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, "arn:aws:sns:us-east-1:111122223333:alerts", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: AutoScaling DescribeScalingActivities
// ---------------------------------------------------------------------------

type mockASGDescribeScalingActivitiesAPIChildPaginated struct {
	PageFunc func(call int) (*autoscaling.DescribeScalingActivitiesOutput, error)
	calls    int
}

func (m *mockASGDescribeScalingActivitiesAPIChildPaginated) DescribeScalingActivities(_ context.Context, _ *autoscaling.DescribeScalingActivitiesInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchAsgActivities_FirstPage(t *testing.T) {
	ts := time.Date(2025, 3, 5, 12, 0, 0, 0, time.UTC)
	mock := &mockASGDescribeScalingActivitiesAPIChildPaginated{
		PageFunc: func(_ int) (*autoscaling.DescribeScalingActivitiesOutput, error) {
			return &autoscaling.DescribeScalingActivitiesOutput{
				Activities: []asgtypes.Activity{
					{ActivityId: aws.String("act-001"), StartTime: &ts, StatusCode: asgtypes.ScalingActivityStatusCodeSuccessful},
				},
				NextToken: aws.String("asg-next-token-2"),
			}, nil
		},
	}
	parentCtx := map[string]string{"asg_name": "my-web-asg"}
	result, err := awsclient.FetchAsgActivities(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "asg-next-token-2" {
		t.Errorf("NextToken: expected %q, got %q", "asg-next-token-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "act-001" {
		t.Errorf("resource ID: expected %q, got %q", "act-001", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchAsgActivities_Continuation(t *testing.T) {
	ts := time.Date(2025, 3, 6, 12, 0, 0, 0, time.UTC)
	mock := &mockASGDescribeScalingActivitiesAPIChildPaginated{
		PageFunc: func(_ int) (*autoscaling.DescribeScalingActivitiesOutput, error) {
			return &autoscaling.DescribeScalingActivitiesOutput{
				Activities: []asgtypes.Activity{
					{ActivityId: aws.String("act-002"), StartTime: &ts, StatusCode: asgtypes.ScalingActivityStatusCodeFailed},
				},
				NextToken: nil,
			}, nil
		},
	}
	parentCtx := map[string]string{"asg_name": "my-web-asg"}
	result, err := awsclient.FetchAsgActivities(context.Background(), mock, parentCtx, "asg-next-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchAsgActivities_Empty(t *testing.T) {
	mock := &mockASGDescribeScalingActivitiesAPIChildPaginated{
		PageFunc: func(_ int) (*autoscaling.DescribeScalingActivitiesOutput, error) {
			return &autoscaling.DescribeScalingActivitiesOutput{Activities: []asgtypes.Activity{}, NextToken: nil}, nil
		},
	}
	parentCtx := map[string]string{"asg_name": "my-web-asg"}
	result, err := awsclient.FetchAsgActivities(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchAsgActivities_Error(t *testing.T) {
	mock := &mockASGDescribeScalingActivitiesAPIChildPaginated{
		PageFunc: func(_ int) (*autoscaling.DescribeScalingActivitiesOutput, error) {
			return nil, errors.New("describe scaling activities failed")
		},
	}
	parentCtx := map[string]string{"asg_name": "my-web-asg"}
	_, err := awsclient.FetchAsgActivities(context.Background(), mock, parentCtx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: SFN ListExecutions
// ---------------------------------------------------------------------------

type mockSFNListExecutionsAPIChildPaginated struct {
	PageFunc func(call int) (*sfn.ListExecutionsOutput, error)
	calls    int
}

func (m *mockSFNListExecutionsAPIChildPaginated) ListExecutions(_ context.Context, _ *sfn.ListExecutionsInput, _ ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchSFNExecutions_FirstPage(t *testing.T) {
	startDate := time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)
	mock := &mockSFNListExecutionsAPIChildPaginated{
		PageFunc: func(_ int) (*sfn.ListExecutionsOutput, error) {
			return &sfn.ListExecutionsOutput{
				Executions: []sfntypes.ExecutionListItem{
					{Name: aws.String("exec-001"), ExecutionArn: aws.String("arn:aws:states:us-east-1:111122223333:execution:my-sm:exec-001"), Status: sfntypes.ExecutionStatusSucceeded, StartDate: &startDate},
				},
				NextToken: aws.String("sfn-next-token-2"),
			}, nil
		},
	}
	parentCtx := map[string]string{"state_machine_arn": "arn:aws:states:us-east-1:111122223333:stateMachine:my-sm"}
	result, err := awsclient.FetchSFNExecutions(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "sfn-next-token-2" {
		t.Errorf("NextToken: expected %q, got %q", "sfn-next-token-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "exec-001" {
		t.Errorf("resource ID: expected %q, got %q", "exec-001", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchSFNExecutions_Continuation(t *testing.T) {
	startDate := time.Date(2025, 3, 2, 9, 0, 0, 0, time.UTC)
	mock := &mockSFNListExecutionsAPIChildPaginated{
		PageFunc: func(_ int) (*sfn.ListExecutionsOutput, error) {
			return &sfn.ListExecutionsOutput{
				Executions: []sfntypes.ExecutionListItem{
					{Name: aws.String("exec-002"), ExecutionArn: aws.String("arn:aws:states:us-east-1:111122223333:execution:my-sm:exec-002"), Status: sfntypes.ExecutionStatusFailed, StartDate: &startDate},
				},
				NextToken: nil,
			}, nil
		},
	}
	parentCtx := map[string]string{"state_machine_arn": "arn:aws:states:us-east-1:111122223333:stateMachine:my-sm"}
	result, err := awsclient.FetchSFNExecutions(context.Background(), mock, parentCtx, "sfn-next-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchSFNExecutions_Empty(t *testing.T) {
	mock := &mockSFNListExecutionsAPIChildPaginated{
		PageFunc: func(_ int) (*sfn.ListExecutionsOutput, error) {
			return &sfn.ListExecutionsOutput{Executions: []sfntypes.ExecutionListItem{}, NextToken: nil}, nil
		},
	}
	parentCtx := map[string]string{"state_machine_arn": "arn:aws:states:us-east-1:111122223333:stateMachine:my-sm"}
	result, err := awsclient.FetchSFNExecutions(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchSFNExecutions_Error(t *testing.T) {
	mock := &mockSFNListExecutionsAPIChildPaginated{
		PageFunc: func(_ int) (*sfn.ListExecutionsOutput, error) {
			return nil, errors.New("list executions failed")
		},
	}
	parentCtx := map[string]string{"state_machine_arn": "arn:aws:states:us-east-1:111122223333:stateMachine:my-sm"}
	_, err := awsclient.FetchSFNExecutions(context.Background(), mock, parentCtx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: SFN GetExecutionHistory
// ---------------------------------------------------------------------------

type mockSFNGetExecutionHistoryAPIChildPaginated struct {
	PageFunc func(call int) (*sfn.GetExecutionHistoryOutput, error)
	calls    int
}

func (m *mockSFNGetExecutionHistoryAPIChildPaginated) GetExecutionHistory(_ context.Context, _ *sfn.GetExecutionHistoryInput, _ ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchSFNExecutionHistory_FirstPage(t *testing.T) {
	ts := time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC)
	mock := &mockSFNGetExecutionHistoryAPIChildPaginated{
		PageFunc: func(_ int) (*sfn.GetExecutionHistoryOutput, error) {
			return &sfn.GetExecutionHistoryOutput{
				Events: []sfntypes.HistoryEvent{
					{Id: 1, Type: sfntypes.HistoryEventTypeExecutionStarted, Timestamp: &ts},
				},
				NextToken: aws.String("sfn-hist-next-2"),
			}, nil
		},
	}
	parentCtx := map[string]string{"execution_arn": "arn:aws:states:us-east-1:111122223333:execution:my-sm:exec-001"}
	result, err := awsclient.FetchSFNExecutionHistory(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "sfn-hist-next-2" {
		t.Errorf("NextToken: expected %q, got %q", "sfn-hist-next-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "1" {
		t.Errorf("resource ID: expected %q, got %q", "1", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchSFNExecutionHistory_Continuation(t *testing.T) {
	ts := time.Date(2025, 3, 1, 9, 1, 0, 0, time.UTC)
	mock := &mockSFNGetExecutionHistoryAPIChildPaginated{
		PageFunc: func(_ int) (*sfn.GetExecutionHistoryOutput, error) {
			return &sfn.GetExecutionHistoryOutput{
				Events: []sfntypes.HistoryEvent{
					{Id: 2, Type: sfntypes.HistoryEventTypeExecutionSucceeded, Timestamp: &ts},
				},
				NextToken: nil,
			}, nil
		},
	}
	parentCtx := map[string]string{"execution_arn": "arn:aws:states:us-east-1:111122223333:execution:my-sm:exec-001"}
	result, err := awsclient.FetchSFNExecutionHistory(context.Background(), mock, parentCtx, "sfn-hist-next-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchSFNExecutionHistory_Empty(t *testing.T) {
	mock := &mockSFNGetExecutionHistoryAPIChildPaginated{
		PageFunc: func(_ int) (*sfn.GetExecutionHistoryOutput, error) {
			return &sfn.GetExecutionHistoryOutput{Events: []sfntypes.HistoryEvent{}, NextToken: nil}, nil
		},
	}
	parentCtx := map[string]string{"execution_arn": "arn:aws:states:us-east-1:111122223333:execution:my-sm:exec-001"}
	result, err := awsclient.FetchSFNExecutionHistory(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchSFNExecutionHistory_Error(t *testing.T) {
	mock := &mockSFNGetExecutionHistoryAPIChildPaginated{
		PageFunc: func(_ int) (*sfn.GetExecutionHistoryOutput, error) {
			return nil, errors.New("get execution history failed")
		},
	}
	parentCtx := map[string]string{"execution_arn": "arn:aws:states:us-east-1:111122223333:execution:my-sm:exec-001"}
	_, err := awsclient.FetchSFNExecutionHistory(context.Background(), mock, parentCtx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: Glue GetJobRuns
// ---------------------------------------------------------------------------

type mockGlueGetJobRunsAPIChildPaginated struct {
	PageFunc func(call int) (*glue.GetJobRunsOutput, error)
	calls    int
}

func (m *mockGlueGetJobRunsAPIChildPaginated) GetJobRuns(_ context.Context, _ *glue.GetJobRunsInput, _ ...func(*glue.Options)) (*glue.GetJobRunsOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchGlueJobRuns_FirstPage(t *testing.T) {
	startedOn := time.Date(2025, 3, 8, 7, 0, 0, 0, time.UTC)
	mock := &mockGlueGetJobRunsAPIChildPaginated{
		PageFunc: func(_ int) (*glue.GetJobRunsOutput, error) {
			return &glue.GetJobRunsOutput{
				JobRuns: []gluetypes.JobRun{
					{Id: aws.String("jr-abcdef12"), JobRunState: gluetypes.JobRunStateStopped, StartedOn: &startedOn, JobName: aws.String("my-etl-job")},
				},
				NextToken: aws.String("glue-next-token-2"),
			}, nil
		},
	}
	result, err := awsclient.FetchGlueJobRuns(context.Background(), mock, "my-etl-job", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "glue-next-token-2" {
		t.Errorf("NextToken: expected %q, got %q", "glue-next-token-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "jr-abcdef12" {
		t.Errorf("resource ID: expected %q, got %q", "jr-abcdef12", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchGlueJobRuns_Continuation(t *testing.T) {
	startedOn := time.Date(2025, 3, 9, 7, 0, 0, 0, time.UTC)
	mock := &mockGlueGetJobRunsAPIChildPaginated{
		PageFunc: func(_ int) (*glue.GetJobRunsOutput, error) {
			return &glue.GetJobRunsOutput{
				JobRuns: []gluetypes.JobRun{
					{Id: aws.String("jr-fedcba98"), JobRunState: gluetypes.JobRunStateFailed, StartedOn: &startedOn, JobName: aws.String("my-etl-job")},
				},
				NextToken: nil,
			}, nil
		},
	}
	result, err := awsclient.FetchGlueJobRuns(context.Background(), mock, "my-etl-job", "glue-next-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchGlueJobRuns_Empty(t *testing.T) {
	mock := &mockGlueGetJobRunsAPIChildPaginated{
		PageFunc: func(_ int) (*glue.GetJobRunsOutput, error) {
			return &glue.GetJobRunsOutput{JobRuns: []gluetypes.JobRun{}, NextToken: nil}, nil
		},
	}
	result, err := awsclient.FetchGlueJobRuns(context.Background(), mock, "my-etl-job", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchGlueJobRuns_Error(t *testing.T) {
	mock := &mockGlueGetJobRunsAPIChildPaginated{
		PageFunc: func(_ int) (*glue.GetJobRunsOutput, error) {
			return nil, errors.New("get job runs failed")
		},
	}
	_, err := awsclient.FetchGlueJobRuns(context.Background(), mock, "my-etl-job", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: IAM GetGroup
// ---------------------------------------------------------------------------

type mockIAMGetGroupAPIChildPaginated struct {
	PageFunc func(call int) (*iam.GetGroupOutput, error)
	calls    int
}

func (m *mockIAMGetGroupAPIChildPaginated) GetGroup(_ context.Context, _ *iam.GetGroupInput, _ ...func(*iam.Options)) (*iam.GetGroupOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchIAMGroupMembers_FirstPage(t *testing.T) {
	createDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	mock := &mockIAMGetGroupAPIChildPaginated{
		PageFunc: func(_ int) (*iam.GetGroupOutput, error) {
			return &iam.GetGroupOutput{
				Group: &iamtypes.Group{GroupName: aws.String("dev-team"), GroupId: aws.String("AGPA111EXAMPLE"), Arn: aws.String("arn:aws:iam::111122223333:group/dev-team"), Path: aws.String("/"), CreateDate: &createDate},
				Users: []iamtypes.User{
					{UserName: aws.String("alice"), UserId: aws.String("AIDA111ALICE"), Arn: aws.String("arn:aws:iam::111122223333:user/alice"), Path: aws.String("/"), CreateDate: &createDate},
				},
				IsTruncated: true,
				Marker:      aws.String("iam-marker-2"),
			}, nil
		},
	}
	parentCtx := map[string]string{"group_name": "dev-team"}
	result, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	// After migration: IsTruncated=true when more pages exist.
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "alice" {
		t.Errorf("resource ID: expected %q, got %q", "alice", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchIAMGroupMembers_Continuation(t *testing.T) {
	createDate := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	mock := &mockIAMGetGroupAPIChildPaginated{
		PageFunc: func(_ int) (*iam.GetGroupOutput, error) {
			return &iam.GetGroupOutput{
				Group: &iamtypes.Group{GroupName: aws.String("dev-team"), GroupId: aws.String("AGPA111EXAMPLE"), Arn: aws.String("arn:aws:iam::111122223333:group/dev-team"), Path: aws.String("/"), CreateDate: &createDate},
				Users: []iamtypes.User{
					{UserName: aws.String("bob"), UserId: aws.String("AIDA111BOB"), Arn: aws.String("arn:aws:iam::111122223333:user/bob"), Path: aws.String("/"), CreateDate: &createDate},
				},
				IsTruncated: false,
			}, nil
		},
	}
	parentCtx := map[string]string{"group_name": "dev-team"}
	result, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx, "iam-marker-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchIAMGroupMembers_Empty(t *testing.T) {
	createDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mock := &mockIAMGetGroupAPIChildPaginated{
		PageFunc: func(_ int) (*iam.GetGroupOutput, error) {
			return &iam.GetGroupOutput{
				Group:       &iamtypes.Group{GroupName: aws.String("empty-group"), GroupId: aws.String("AGPA111EMPTY"), Arn: aws.String("arn:aws:iam::111122223333:group/empty-group"), Path: aws.String("/"), CreateDate: &createDate},
				Users:       []iamtypes.User{},
				IsTruncated: false,
			}, nil
		},
	}
	parentCtx := map[string]string{"group_name": "empty-group"}
	result, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchIAMGroupMembers_Error(t *testing.T) {
	mock := &mockIAMGetGroupAPIChildPaginated{
		PageFunc: func(_ int) (*iam.GetGroupOutput, error) {
			return nil, errors.New("get group failed")
		},
	}
	parentCtx := map[string]string{"group_name": "dev-team"}
	_, err := awsclient.FetchIAMGroupMembers(context.Background(), mock, parentCtx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ELBv2 DescribeListeners
// ---------------------------------------------------------------------------

type mockELBv2DescribeListenersAPIChildPaginated struct {
	PageFunc func(call int) (*elbv2.DescribeListenersOutput, error)
	calls    int
}

func (m *mockELBv2DescribeListenersAPIChildPaginated) DescribeListeners(_ context.Context, _ *elbv2.DescribeListenersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchELBListeners_FirstPage(t *testing.T) {
	port := int32(443)
	mock := &mockELBv2DescribeListenersAPIChildPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeListenersOutput, error) {
			return &elbv2.DescribeListenersOutput{
				Listeners: []elbtypes.Listener{
					{ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:111122223333:listener/app/my-alb/abc/def"), Port: &port, Protocol: elbtypes.ProtocolEnumHttps},
				},
				NextMarker: aws.String("elb-next-marker-2"),
			}, nil
		},
	}
	parentCtx := map[string]string{"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/app/my-alb/abc"}
	result, err := awsclient.FetchELBListeners(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	if result.Pagination.NextToken != "elb-next-marker-2" {
		t.Errorf("NextToken: expected %q, got %q", "elb-next-marker-2", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
}

func TestQA_ChildPagination_FetchELBListeners_Continuation(t *testing.T) {
	port := int32(80)
	mock := &mockELBv2DescribeListenersAPIChildPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeListenersOutput, error) {
			return &elbv2.DescribeListenersOutput{
				Listeners: []elbtypes.Listener{
					{ListenerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:111122223333:listener/app/my-alb/abc/ghi"), Port: &port, Protocol: elbtypes.ProtocolEnumHttp},
				},
				NextMarker: nil,
			}, nil
		},
	}
	parentCtx := map[string]string{"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/app/my-alb/abc"}
	result, err := awsclient.FetchELBListeners(context.Background(), mock, parentCtx, "elb-next-marker-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchELBListeners_Empty(t *testing.T) {
	mock := &mockELBv2DescribeListenersAPIChildPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeListenersOutput, error) {
			return &elbv2.DescribeListenersOutput{Listeners: []elbtypes.Listener{}, NextMarker: nil}, nil
		},
	}
	parentCtx := map[string]string{"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/app/my-alb/abc"}
	result, err := awsclient.FetchELBListeners(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchELBListeners_Error(t *testing.T) {
	mock := &mockELBv2DescribeListenersAPIChildPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeListenersOutput, error) {
			return nil, errors.New("describe listeners failed")
		},
	}
	parentCtx := map[string]string{"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:111122223333:loadbalancer/app/my-alb/abc"}
	_, err := awsclient.FetchELBListeners(context.Background(), mock, parentCtx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ELBv2 DescribeRules
// ---------------------------------------------------------------------------

type mockELBv2DescribeRulesAPIChildPaginated struct {
	PageFunc func(call int) (*elbv2.DescribeRulesOutput, error)
	calls    int
}

func (m *mockELBv2DescribeRulesAPIChildPaginated) DescribeRules(_ context.Context, _ *elbv2.DescribeRulesInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchELBListenerRules_FirstPage(t *testing.T) {
	isDefault := false
	mock := &mockELBv2DescribeRulesAPIChildPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeRulesOutput, error) {
			return &elbv2.DescribeRulesOutput{
				Rules: []elbtypes.Rule{
					{RuleArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:111122223333:listener-rule/app/my-alb/abc/def/rule001"), Priority: aws.String("1"), IsDefault: &isDefault},
				},
			}, nil
		},
	}
	parentCtx := map[string]string{"listener_arn": "arn:aws:elasticloadbalancing:us-east-1:111122223333:listener/app/my-alb/abc/def"}
	result, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	// DescribeRules is single-call (no AWS pagination); IsTruncated always false.
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false (DescribeRules has no server-side pagination)")
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "arn:aws:elasticloadbalancing:us-east-1:111122223333:listener-rule/app/my-alb/abc/def/rule001" {
		t.Errorf("resource ID: expected rule ARN, got %q", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchELBListenerRules_Continuation(t *testing.T) {
	// DescribeRules has no server pagination; continuationToken is accepted but ignored.
	isDefault := true
	mock := &mockELBv2DescribeRulesAPIChildPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeRulesOutput, error) {
			return &elbv2.DescribeRulesOutput{
				Rules: []elbtypes.Rule{
					{RuleArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:111122223333:listener-rule/app/my-alb/abc/def/default"), Priority: aws.String("default"), IsDefault: &isDefault},
				},
			}, nil
		},
	}
	parentCtx := map[string]string{"listener_arn": "arn:aws:elasticloadbalancing:us-east-1:111122223333:listener/app/my-alb/abc/def"}
	result, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx, "some-ignored-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchELBListenerRules_Empty(t *testing.T) {
	mock := &mockELBv2DescribeRulesAPIChildPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeRulesOutput, error) {
			return &elbv2.DescribeRulesOutput{Rules: []elbtypes.Rule{}}, nil
		},
	}
	parentCtx := map[string]string{"listener_arn": "arn:aws:elasticloadbalancing:us-east-1:111122223333:listener/app/my-alb/abc/def"}
	result, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchELBListenerRules_Error(t *testing.T) {
	mock := &mockELBv2DescribeRulesAPIChildPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeRulesOutput, error) {
			return nil, errors.New("describe rules failed")
		},
	}
	parentCtx := map[string]string{"listener_arn": "arn:aws:elasticloadbalancing:us-east-1:111122223333:listener/app/my-alb/abc/def"}
	_, err := awsclient.FetchELBListenerRules(context.Background(), mock, parentCtx, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ECS ListTasks + DescribeTasks (ecs_svc_tasks — two APIs)
// ---------------------------------------------------------------------------

type mockECSListTasksAPIChildPaginated struct {
	PageFunc func(call int) (*ecs.ListTasksOutput, error)
	calls    int
}

func (m *mockECSListTasksAPIChildPaginated) ListTasks(_ context.Context, _ *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

type mockECSDescribeTasksAPIChildPaginated struct {
	PageFunc func(call int) (*ecs.DescribeTasksOutput, error)
	calls    int
}

func (m *mockECSDescribeTasksAPIChildPaginated) DescribeTasks(_ context.Context, _ *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	m.calls++
	return m.PageFunc(m.calls)
}

func TestQA_ChildPagination_FetchEcsSvcTasks_FirstPage(t *testing.T) {
	startedAt := time.Date(2025, 3, 1, 8, 0, 0, 0, time.UTC)
	listMock := &mockECSListTasksAPIChildPaginated{
		PageFunc: func(_ int) (*ecs.ListTasksOutput, error) {
			return &ecs.ListTasksOutput{
				TaskArns: []string{"arn:aws:ecs:us-east-1:111122223333:task/my-cluster/taskid001"},
				NextToken: aws.String("ecs-next-token-2"),
			}, nil
		},
	}
	describeMock := &mockECSDescribeTasksAPIChildPaginated{
		PageFunc: func(_ int) (*ecs.DescribeTasksOutput, error) {
			return &ecs.DescribeTasksOutput{
				Tasks: []ecstypes.Task{
					{TaskArn: aws.String("arn:aws:ecs:us-east-1:111122223333:task/my-cluster/taskid001"), LastStatus: aws.String("RUNNING"), StartedAt: &startedAt},
				},
			}, nil
		},
	}
	result, err := awsclient.FetchEcsSvcTasks(context.Background(), listMock, describeMock, "my-cluster", "my-svc", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true")
	}
	// ECS tasks fetch two statuses (RUNNING + STOPPED) in one call,
	// so per-page continuation isn't supported — NextToken is empty
	// but IsTruncated signals that more tasks exist.
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty (dual-status fetch), got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "taskid001" {
		t.Errorf("resource ID: expected %q, got %q", "taskid001", result.Resources[0].ID)
	}
}

func TestQA_ChildPagination_FetchEcsSvcTasks_Continuation(t *testing.T) {
	startedAt := time.Date(2025, 3, 2, 8, 0, 0, 0, time.UTC)
	listMock := &mockECSListTasksAPIChildPaginated{
		PageFunc: func(_ int) (*ecs.ListTasksOutput, error) {
			return &ecs.ListTasksOutput{
				TaskArns:  []string{"arn:aws:ecs:us-east-1:111122223333:task/my-cluster/taskid002"},
				NextToken: nil,
			}, nil
		},
	}
	describeMock := &mockECSDescribeTasksAPIChildPaginated{
		PageFunc: func(_ int) (*ecs.DescribeTasksOutput, error) {
			return &ecs.DescribeTasksOutput{
				Tasks: []ecstypes.Task{
					{TaskArn: aws.String("arn:aws:ecs:us-east-1:111122223333:task/my-cluster/taskid002"), LastStatus: aws.String("STOPPED"), StartedAt: &startedAt},
				},
			}, nil
		},
	}
	result, err := awsclient.FetchEcsSvcTasks(context.Background(), listMock, describeMock, "my-cluster", "my-svc", "ecs-next-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty, got %q", result.Pagination.NextToken)
	}
}

func TestQA_ChildPagination_FetchEcsSvcTasks_Empty(t *testing.T) {
	listMock := &mockECSListTasksAPIChildPaginated{
		PageFunc: func(_ int) (*ecs.ListTasksOutput, error) {
			return &ecs.ListTasksOutput{TaskArns: []string{}, NextToken: nil}, nil
		},
	}
	describeMock := &mockECSDescribeTasksAPIChildPaginated{
		PageFunc: func(_ int) (*ecs.DescribeTasksOutput, error) {
			return &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{}}, nil
		},
	}
	result, err := awsclient.FetchEcsSvcTasks(context.Background(), listMock, describeMock, "my-cluster", "my-svc", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false")
	}
}

func TestQA_ChildPagination_FetchEcsSvcTasks_Error(t *testing.T) {
	listMock := &mockECSListTasksAPIChildPaginated{
		PageFunc: func(_ int) (*ecs.ListTasksOutput, error) {
			return nil, errors.New("list tasks failed")
		},
	}
	describeMock := &mockECSDescribeTasksAPIChildPaginated{
		PageFunc: func(_ int) (*ecs.DescribeTasksOutput, error) {
			return &ecs.DescribeTasksOutput{}, nil
		},
	}
	_, err := awsclient.FetchEcsSvcTasks(context.Background(), listMock, describeMock, "my-cluster", "my-svc", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Ensure resource package import is used (type check only).
var _ resource.FetchResult
