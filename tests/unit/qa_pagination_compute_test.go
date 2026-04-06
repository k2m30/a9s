package unit

// qa_pagination_compute_test.go — pagination tests for compute/storage fetchers:
// ec2, lambda, s3, ebs, ebs-snap, ami

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeInstances (paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeInstancesAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*ec2.DescribeInstancesOutput, error)
}

func (m *mockEC2DescribeInstancesAPIPaginated) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

func (m *mockEC2DescribeInstancesAPIPaginated) DescribeInstanceStatus(
	_ context.Context,
	_ *ec2.DescribeInstanceStatusInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchEC2InstancesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchEC2InstancesPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeInstancesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId:   aws.String("i-0abc111222333444a"),
								State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
								InstanceType: ec2types.InstanceTypeT3Micro,
							},
						},
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchEC2InstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "i-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "i-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchEC2InstancesPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeInstancesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{
					{
						Instances: []ec2types.Instance{
							{
								InstanceId:   aws.String("i-0xyz999888777666b"),
								State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameStopped},
								InstanceType: ec2types.InstanceTypeT3Small,
							},
						},
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEC2InstancesPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
}

func TestQA_Pagination_FetchEC2InstancesPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeInstancesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []ec2types.Reservation{},
				NextToken:    nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEC2InstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchEC2InstancesPage_Error(t *testing.T) {
	mock := &mockEC2DescribeInstancesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeInstancesOutput, error) {
			return nil, errors.New("describe instances failed")
		},
	}

	_, err := awsclient.FetchEC2InstancesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: Lambda ListFunctions (paginated)
// ---------------------------------------------------------------------------

type mockLambdaListFunctionsAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*lambda.ListFunctionsOutput, error)
}

func (m *mockLambdaListFunctionsAPIPaginated) ListFunctions(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchLambdaFunctionsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchLambdaFunctionsPage_FirstPage(t *testing.T) {
	mock := &mockLambdaListFunctionsAPIPaginated{
		PageFunc: func(_ int) (*lambda.ListFunctionsOutput, error) {
			return &lambda.ListFunctionsOutput{
				Functions: []lambdatypes.FunctionConfiguration{
					{
						FunctionName: aws.String("my-handler"),
						Runtime:      lambdatypes.RuntimeProvidedal2023,
						MemorySize:   aws.Int32(128),
						Timeout:      aws.Int32(30),
					},
				},
				NextMarker: aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchLambdaFunctionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextMarker")
	}
	if result.Pagination.NextToken != "marker-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "marker-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-handler" {
		t.Errorf("resource ID: expected %q, got %q", "my-handler", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchLambdaFunctionsPage_Continuation(t *testing.T) {
	mock := &mockLambdaListFunctionsAPIPaginated{
		PageFunc: func(_ int) (*lambda.ListFunctionsOutput, error) {
			return &lambda.ListFunctionsOutput{
				Functions: []lambdatypes.FunctionConfiguration{
					{
						FunctionName: aws.String("another-handler"),
						Runtime:      lambdatypes.RuntimeNodejs22x,
						MemorySize:   aws.Int32(256),
						Timeout:      aws.Int32(60),
					},
				},
				NextMarker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchLambdaFunctionsPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextMarker=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchLambdaFunctionsPage_Empty(t *testing.T) {
	mock := &mockLambdaListFunctionsAPIPaginated{
		PageFunc: func(_ int) (*lambda.ListFunctionsOutput, error) {
			return &lambda.ListFunctionsOutput{
				Functions:  []lambdatypes.FunctionConfiguration{},
				NextMarker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchLambdaFunctionsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchLambdaFunctionsPage_Error(t *testing.T) {
	mock := &mockLambdaListFunctionsAPIPaginated{
		PageFunc: func(_ int) (*lambda.ListFunctionsOutput, error) {
			return nil, errors.New("list functions failed")
		},
	}

	_, err := awsclient.FetchLambdaFunctionsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: S3 ListBuckets (paginated)
// ---------------------------------------------------------------------------

type mockS3ListBucketsAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*s3.ListBucketsOutput, error)
}

func (m *mockS3ListBucketsAPIPaginated) ListBuckets(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchS3BucketsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchS3BucketsPage_FirstPage(t *testing.T) {
	mock := &mockS3ListBucketsAPIPaginated{
		PageFunc: func(_ int) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: aws.String("my-app-bucket")},
				},
				ContinuationToken: aws.String("cont-token-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchS3BucketsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with ContinuationToken")
	}
	if result.Pagination.NextToken != "cont-token-2" {
		t.Errorf("NextToken: expected %q, got %q", "cont-token-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-app-bucket" {
		t.Errorf("resource ID: expected %q, got %q", "my-app-bucket", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchS3BucketsPage_Continuation(t *testing.T) {
	mock := &mockS3ListBucketsAPIPaginated{
		PageFunc: func(_ int) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: aws.String("my-logs-bucket")},
				},
				ContinuationToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchS3BucketsPage(context.Background(), mock, "cont-token-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (ContinuationToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchS3BucketsPage_Empty(t *testing.T) {
	mock := &mockS3ListBucketsAPIPaginated{
		PageFunc: func(_ int) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets:           []s3types.Bucket{},
				ContinuationToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchS3BucketsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchS3BucketsPage_Error(t *testing.T) {
	mock := &mockS3ListBucketsAPIPaginated{
		PageFunc: func(_ int) (*s3.ListBucketsOutput, error) {
			return nil, errors.New("list buckets failed")
		},
	}

	_, err := awsclient.FetchS3BucketsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeVolumes (EBS paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeVolumesAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*ec2.DescribeVolumesOutput, error)
}

func (m *mockEC2DescribeVolumesAPIPaginated) DescribeVolumes(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchEBSVolumesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchEBSVolumesPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeVolumesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVolumesOutput, error) {
			return &ec2.DescribeVolumesOutput{
				Volumes: []ec2types.Volume{
					{
						VolumeId:   aws.String("vol-0abc111222333444a"),
						State:      ec2types.VolumeStateAvailable,
						VolumeType: ec2types.VolumeTypeGp3,
						Size:       aws.Int32(20),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchEBSVolumesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "vol-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "vol-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchEBSVolumesPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeVolumesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVolumesOutput, error) {
			return &ec2.DescribeVolumesOutput{
				Volumes: []ec2types.Volume{
					{
						VolumeId:   aws.String("vol-0xyz999888777666b"),
						State:      ec2types.VolumeStateInUse,
						VolumeType: ec2types.VolumeTypeGp2,
						Size:       aws.Int32(100),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEBSVolumesPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchEBSVolumesPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeVolumesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVolumesOutput, error) {
			return &ec2.DescribeVolumesOutput{
				Volumes:   []ec2types.Volume{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEBSVolumesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchEBSVolumesPage_Error(t *testing.T) {
	mock := &mockEC2DescribeVolumesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVolumesOutput, error) {
			return nil, errors.New("describe volumes failed")
		},
	}

	_, err := awsclient.FetchEBSVolumesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeSnapshots (EBS snapshots paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeSnapshotsAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*ec2.DescribeSnapshotsOutput, error)
}

func (m *mockEC2DescribeSnapshotsAPIPaginated) DescribeSnapshots(_ context.Context, _ *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchEBSSnapshotsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchEBSSnapshotsPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSnapshotsOutput, error) {
			return &ec2.DescribeSnapshotsOutput{
				Snapshots: []ec2types.Snapshot{
					{
						SnapshotId:  aws.String("snap-0abc111222333444a"),
						State:       ec2types.SnapshotStateCompleted,
						VolumeId:    aws.String("vol-0abc111222333444a"),
						VolumeSize:  aws.Int32(20),
						Encrypted:   aws.Bool(false),
						Description: aws.String("test snapshot"),
						Progress:    aws.String("100%"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchEBSSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "snap-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "snap-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchEBSSnapshotsPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSnapshotsOutput, error) {
			return &ec2.DescribeSnapshotsOutput{
				Snapshots: []ec2types.Snapshot{
					{
						SnapshotId:  aws.String("snap-0xyz999888777666b"),
						State:       ec2types.SnapshotStateCompleted,
						VolumeId:    aws.String("vol-0xyz999888777666b"),
						VolumeSize:  aws.Int32(50),
						Encrypted:   aws.Bool(true),
						Description: aws.String("encrypted snapshot"),
						Progress:    aws.String("100%"),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEBSSnapshotsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchEBSSnapshotsPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSnapshotsOutput, error) {
			return &ec2.DescribeSnapshotsOutput{
				Snapshots: []ec2types.Snapshot{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEBSSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchEBSSnapshotsPage_Error(t *testing.T) {
	mock := &mockEC2DescribeSnapshotsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSnapshotsOutput, error) {
			return nil, errors.New("describe snapshots failed")
		},
	}

	_, err := awsclient.FetchEBSSnapshotsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeImages (AMI paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeImagesAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*ec2.DescribeImagesOutput, error)
}

func (m *mockEC2DescribeImagesAPIPaginated) DescribeImages(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchAMIsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchAMIsPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeImagesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						ImageId:        aws.String("ami-0abc111222333444a"),
						Name:           aws.String("my-golden-ami"),
						State:          ec2types.ImageStateAvailable,
						Architecture:   ec2types.ArchitectureValuesX8664,
						PlatformDetails: aws.String("Linux/UNIX"),
						RootDeviceType: ec2types.DeviceTypeEbs,
						CreationDate:   aws.String("2025-01-15T10:30:00.000Z"),
						Public:         aws.Bool(false),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchAMIsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination == nil {
		t.Fatal("expected Pagination metadata, got nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=true for first page with NextToken")
	}
	if result.Pagination.NextToken != "token-page-2" {
		t.Errorf("NextToken: expected %q, got %q", "token-page-2", result.Pagination.NextToken)
	}
	if result.Pagination.PageSize != 1 {
		t.Errorf("PageSize: expected 1, got %d", result.Pagination.PageSize)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "ami-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "ami-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchAMIsPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeImagesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images: []ec2types.Image{
					{
						ImageId:        aws.String("ami-0xyz999888777666b"),
						Name:           aws.String("my-arm64-ami"),
						State:          ec2types.ImageStateAvailable,
						Architecture:   ec2types.ArchitectureValuesArm64,
						PlatformDetails: aws.String("Linux/UNIX"),
						RootDeviceType: ec2types.DeviceTypeEbs,
						CreationDate:   aws.String("2025-02-01T08:00:00.000Z"),
						Public:         aws.Bool(true),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchAMIsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
}

func TestQA_Pagination_FetchAMIsPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeImagesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeImagesOutput, error) {
			return &ec2.DescribeImagesOutput{
				Images:    []ec2types.Image{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchAMIsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result.Resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(result.Resources))
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for empty result")
	}
}

func TestQA_Pagination_FetchAMIsPage_Error(t *testing.T) {
	mock := &mockEC2DescribeImagesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeImagesOutput, error) {
			return nil, errors.New("describe images failed")
		},
	}

	_, err := awsclient.FetchAMIsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
