package unit

// qa_pagination_infra_test.go — pagination tests for infra fetchers:
// ecs, ecs-svc, ecs-task, asg, eb, vpc, subnet, rtb, nat, igw, eni, vpce, tgw, elb, tg

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Mock: ECS ListClusters (paginated)
// ---------------------------------------------------------------------------

type mockECSListClustersAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*ecs.ListClustersOutput, error)
	lastInput *ecs.ListClustersInput
}

func (m *mockECSListClustersAPIPaginated) ListClusters(_ context.Context, in *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// Mock: ECS DescribeClusters
type mockECSDescribeClustersAPIPaginated struct {
	Calls        int
	DescribeFunc func(call int, arns []string) (*ecs.DescribeClustersOutput, error)
}

func (m *mockECSDescribeClustersAPIPaginated) DescribeClusters(_ context.Context, input *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	m.Calls++
	return m.DescribeFunc(m.Calls, input.Clusters)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchECSClustersPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchECSClustersPage_FirstPage(t *testing.T) {
	listMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{"arn:aws:ecs:us-east-1:111111111111:cluster/my-cluster"},
				NextToken:   aws.String("token-page-2"),
			}, nil
		},
	}
	describeMock := &mockECSDescribeClustersAPIPaginated{
		DescribeFunc: func(_ int, arns []string) (*ecs.DescribeClustersOutput, error) {
			clusters := make([]ecstypes.Cluster, 0, len(arns))
			for _, arn := range arns {
				clusterName := "my-cluster"
				_ = arn
				clusters = append(clusters, ecstypes.Cluster{
					ClusterName:         aws.String(clusterName),
					Status:              aws.String("ACTIVE"),
					RunningTasksCount:   3,
					PendingTasksCount:   0,
					ActiveServicesCount: 2,
				})
			}
			return &ecs.DescribeClustersOutput{Clusters: clusters}, nil
		},
	}

	result, err := awsclient.FetchECSClustersPage(context.Background(), listMock, describeMock, "")
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
	if result.Resources[0].ID != "my-cluster" {
		t.Errorf("resource ID: expected %q, got %q", "my-cluster", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchECSClustersPage_Continuation(t *testing.T) {
	listMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{"arn:aws:ecs:us-east-1:111111111111:cluster/other-cluster"},
				NextToken:   nil,
			}, nil
		},
	}
	describeMock := &mockECSDescribeClustersAPIPaginated{
		DescribeFunc: func(_ int, _ []string) (*ecs.DescribeClustersOutput, error) {
			return &ecs.DescribeClustersOutput{
				Clusters: []ecstypes.Cluster{
					{
						ClusterName: aws.String("other-cluster"),
						Status:      aws.String("ACTIVE"),
					},
				},
			}, nil
		},
	}

	result, err := awsclient.FetchECSClustersPage(context.Background(), listMock, describeMock, "token-page-2")
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
	if listMock.lastInput == nil {
		t.Fatal("list mock was not called")
	}
	if listMock.lastInput.NextToken == nil || *listMock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", listMock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchECSClustersPage_Empty(t *testing.T) {
	listMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{},
				NextToken:   nil,
			}, nil
		},
	}
	describeMock := &mockECSDescribeClustersAPIPaginated{
		DescribeFunc: func(_ int, _ []string) (*ecs.DescribeClustersOutput, error) {
			return &ecs.DescribeClustersOutput{Clusters: []ecstypes.Cluster{}}, nil
		},
	}

	result, err := awsclient.FetchECSClustersPage(context.Background(), listMock, describeMock, "")
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

func TestQA_Pagination_FetchECSClustersPage_Error(t *testing.T) {
	listMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return nil, errors.New("list clusters failed")
		},
	}
	describeMock := &mockECSDescribeClustersAPIPaginated{
		DescribeFunc: func(_ int, _ []string) (*ecs.DescribeClustersOutput, error) {
			return &ecs.DescribeClustersOutput{}, nil
		},
	}

	_, err := awsclient.FetchECSClustersPage(context.Background(), listMock, describeMock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ECS ListServices + DescribeServices (paginated)
// ---------------------------------------------------------------------------

type mockECSListServicesAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*ecs.ListServicesOutput, error)
}

func (m *mockECSListServicesAPIPaginated) ListServices(_ context.Context, _ *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

type mockECSDescribeServicesAPIPaginated struct {
	Calls        int
	DescribeFunc func(call int) (*ecs.DescribeServicesOutput, error)
}

func (m *mockECSDescribeServicesAPIPaginated) DescribeServices(_ context.Context, _ *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	m.Calls++
	return m.DescribeFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchECSServicesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchECSServicesPage_FirstPage(t *testing.T) {
	// For FetchECSServicesPage, the list mock returns cluster arns with NextToken
	// The list mock is used for ListClusters in the page function
	listClustersMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{"arn:aws:ecs:us-east-1:111111111111:cluster/prod"},
				NextToken:   aws.String("token-page-2"),
			}, nil
		},
	}
	listServicesMock := &mockECSListServicesAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListServicesOutput, error) {
			return &ecs.ListServicesOutput{
				ServiceArns: []string{"arn:aws:ecs:us-east-1:111111111111:service/prod/my-service"},
				NextToken:   nil,
			}, nil
		},
	}
	describeServicesMock := &mockECSDescribeServicesAPIPaginated{
		DescribeFunc: func(_ int) (*ecs.DescribeServicesOutput, error) {
			return &ecs.DescribeServicesOutput{
				Services: []ecstypes.Service{
					{
						ServiceName:    aws.String("my-service"),
						ClusterArn:     aws.String("arn:aws:ecs:us-east-1:111111111111:cluster/prod"),
						Status:         aws.String("ACTIVE"),
						DesiredCount:   3,
						RunningCount:   3,
						LaunchType:     ecstypes.LaunchTypeFargate,
						TaskDefinition: aws.String("my-task-def:1"),
					},
				},
			}, nil
		},
	}

	result, err := awsclient.FetchECSServicesPage(context.Background(), listClustersMock, listServicesMock, describeServicesMock, "")
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
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "my-service" {
		t.Errorf("resource ID: expected %q, got %q", "my-service", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchECSServicesPage_Continuation(t *testing.T) {
	listClustersMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{"arn:aws:ecs:us-east-1:111111111111:cluster/staging"},
				NextToken:   nil,
			}, nil
		},
	}
	listServicesMock := &mockECSListServicesAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListServicesOutput, error) {
			return &ecs.ListServicesOutput{
				ServiceArns: []string{"arn:aws:ecs:us-east-1:111111111111:service/staging/another-svc"},
				NextToken:   nil,
			}, nil
		},
	}
	describeServicesMock := &mockECSDescribeServicesAPIPaginated{
		DescribeFunc: func(_ int) (*ecs.DescribeServicesOutput, error) {
			return &ecs.DescribeServicesOutput{
				Services: []ecstypes.Service{
					{
						ServiceName:  aws.String("another-svc"),
						ClusterArn:   aws.String("arn:aws:ecs:us-east-1:111111111111:cluster/staging"),
						Status:       aws.String("ACTIVE"),
						DesiredCount: 1,
						RunningCount: 1,
					},
				},
			}, nil
		},
	}

	result, err := awsclient.FetchECSServicesPage(context.Background(), listClustersMock, listServicesMock, describeServicesMock, "token-page-2")
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
	if listClustersMock.lastInput == nil {
		t.Fatal("list clusters mock was not called")
	}
	if listClustersMock.lastInput.NextToken == nil || *listClustersMock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", listClustersMock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchECSServicesPage_Empty(t *testing.T) {
	listClustersMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{},
				NextToken:   nil,
			}, nil
		},
	}
	listServicesMock := &mockECSListServicesAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListServicesOutput, error) {
			return &ecs.ListServicesOutput{ServiceArns: []string{}, NextToken: nil}, nil
		},
	}
	describeServicesMock := &mockECSDescribeServicesAPIPaginated{
		DescribeFunc: func(_ int) (*ecs.DescribeServicesOutput, error) {
			return &ecs.DescribeServicesOutput{Services: []ecstypes.Service{}}, nil
		},
	}

	result, err := awsclient.FetchECSServicesPage(context.Background(), listClustersMock, listServicesMock, describeServicesMock, "")
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

func TestQA_Pagination_FetchECSServicesPage_Error(t *testing.T) {
	listClustersMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return nil, errors.New("list clusters failed")
		},
	}
	listServicesMock := &mockECSListServicesAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListServicesOutput, error) {
			return &ecs.ListServicesOutput{}, nil
		},
	}
	describeServicesMock := &mockECSDescribeServicesAPIPaginated{
		DescribeFunc: func(_ int) (*ecs.DescribeServicesOutput, error) {
			return &ecs.DescribeServicesOutput{}, nil
		},
	}

	_, err := awsclient.FetchECSServicesPage(context.Background(), listClustersMock, listServicesMock, describeServicesMock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ECS ListTasks + DescribeTasks (paginated)
// ---------------------------------------------------------------------------

type mockECSListTasksAPIPaginated struct {
	Calls    int
	PageFunc func(call int) (*ecs.ListTasksOutput, error)
}

func (m *mockECSListTasksAPIPaginated) ListTasks(_ context.Context, _ *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	m.Calls++
	return m.PageFunc(m.Calls)
}

type mockECSDescribeTasksAPIPaginated struct {
	Calls        int
	DescribeFunc func(call int) (*ecs.DescribeTasksOutput, error)
}

func (m *mockECSDescribeTasksAPIPaginated) DescribeTasks(_ context.Context, _ *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	m.Calls++
	return m.DescribeFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchECSTasksPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchECSTasksPage_FirstPage(t *testing.T) {
	listClustersMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{"arn:aws:ecs:us-east-1:111111111111:cluster/prod"},
				NextToken:   aws.String("token-page-2"),
			}, nil
		},
	}
	listTasksMock := &mockECSListTasksAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListTasksOutput, error) {
			return &ecs.ListTasksOutput{
				TaskArns:  []string{"arn:aws:ecs:us-east-1:111111111111:task/prod/abcdef123456"},
				NextToken: nil,
			}, nil
		},
	}
	describeTasksMock := &mockECSDescribeTasksAPIPaginated{
		DescribeFunc: func(_ int) (*ecs.DescribeTasksOutput, error) {
			return &ecs.DescribeTasksOutput{
				Tasks: []ecstypes.Task{
					{
						TaskArn:           aws.String("arn:aws:ecs:us-east-1:111111111111:task/prod/abcdef123456"),
						ClusterArn:        aws.String("arn:aws:ecs:us-east-1:111111111111:cluster/prod"),
						LastStatus:        aws.String("RUNNING"),
						TaskDefinitionArn: aws.String("my-task-def:1"),
						LaunchType:        ecstypes.LaunchTypeFargate,
						Cpu:               aws.String("256"),
						Memory:            aws.String("512"),
					},
				},
			}, nil
		},
	}

	result, err := awsclient.FetchECSTasksPage(context.Background(), listClustersMock, listTasksMock, describeTasksMock, "")
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
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "abcdef123456" {
		t.Errorf("resource ID: expected %q, got %q", "abcdef123456", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchECSTasksPage_Continuation(t *testing.T) {
	listClustersMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{"arn:aws:ecs:us-east-1:111111111111:cluster/staging"},
				NextToken:   nil,
			}, nil
		},
	}
	listTasksMock := &mockECSListTasksAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListTasksOutput, error) {
			return &ecs.ListTasksOutput{
				TaskArns:  []string{"arn:aws:ecs:us-east-1:111111111111:task/staging/xyz999888777"},
				NextToken: nil,
			}, nil
		},
	}
	describeTasksMock := &mockECSDescribeTasksAPIPaginated{
		DescribeFunc: func(_ int) (*ecs.DescribeTasksOutput, error) {
			return &ecs.DescribeTasksOutput{
				Tasks: []ecstypes.Task{
					{
						TaskArn:    aws.String("arn:aws:ecs:us-east-1:111111111111:task/staging/xyz999888777"),
						ClusterArn: aws.String("arn:aws:ecs:us-east-1:111111111111:cluster/staging"),
						LastStatus: aws.String("STOPPED"),
					},
				},
			}, nil
		},
	}

	result, err := awsclient.FetchECSTasksPage(context.Background(), listClustersMock, listTasksMock, describeTasksMock, "token-page-2")
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
	if listClustersMock.lastInput == nil {
		t.Fatal("list clusters mock was not called")
	}
	if listClustersMock.lastInput.NextToken == nil || *listClustersMock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", listClustersMock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchECSTasksPage_Empty(t *testing.T) {
	listClustersMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return &ecs.ListClustersOutput{
				ClusterArns: []string{},
				NextToken:   nil,
			}, nil
		},
	}
	listTasksMock := &mockECSListTasksAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListTasksOutput, error) {
			return &ecs.ListTasksOutput{TaskArns: []string{}, NextToken: nil}, nil
		},
	}
	describeTasksMock := &mockECSDescribeTasksAPIPaginated{
		DescribeFunc: func(_ int) (*ecs.DescribeTasksOutput, error) {
			return &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{}}, nil
		},
	}

	result, err := awsclient.FetchECSTasksPage(context.Background(), listClustersMock, listTasksMock, describeTasksMock, "")
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

func TestQA_Pagination_FetchECSTasksPage_Error(t *testing.T) {
	listClustersMock := &mockECSListClustersAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListClustersOutput, error) {
			return nil, errors.New("list clusters failed")
		},
	}
	listTasksMock := &mockECSListTasksAPIPaginated{
		PageFunc: func(_ int) (*ecs.ListTasksOutput, error) {
			return &ecs.ListTasksOutput{}, nil
		},
	}
	describeTasksMock := &mockECSDescribeTasksAPIPaginated{
		DescribeFunc: func(_ int) (*ecs.DescribeTasksOutput, error) {
			return &ecs.DescribeTasksOutput{}, nil
		},
	}

	_, err := awsclient.FetchECSTasksPage(context.Background(), listClustersMock, listTasksMock, describeTasksMock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: AutoScaling DescribeAutoScalingGroups (paginated)
// ---------------------------------------------------------------------------

type mockASGDescribeAutoScalingGroupsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
	lastInput *autoscaling.DescribeAutoScalingGroupsInput
}

func (m *mockASGDescribeAutoScalingGroupsAPIPaginated) DescribeAutoScalingGroups(_ context.Context, in *autoscaling.DescribeAutoScalingGroupsInput, _ ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchAutoScalingGroupsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchAutoScalingGroupsPage_FirstPage(t *testing.T) {
	minSize := int32(1)
	maxSize := int32(10)
	desired := int32(3)
	mock := &mockASGDescribeAutoScalingGroupsAPIPaginated{
		PageFunc: func(_ int) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			return &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []asgtypes.AutoScalingGroup{
					{
						AutoScalingGroupName: aws.String("my-asg"),
						MinSize:              &minSize,
						MaxSize:              &maxSize,
						DesiredCapacity:      &desired,
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "my-asg" {
		t.Errorf("resource ID: expected %q, got %q", "my-asg", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchAutoScalingGroupsPage_Continuation(t *testing.T) {
	minSize := int32(2)
	maxSize := int32(20)
	desired := int32(5)
	mock := &mockASGDescribeAutoScalingGroupsAPIPaginated{
		PageFunc: func(_ int) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			return &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []asgtypes.AutoScalingGroup{
					{
						AutoScalingGroupName: aws.String("other-asg"),
						MinSize:              &minSize,
						MaxSize:              &maxSize,
						DesiredCapacity:      &desired,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "token-page-2")
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
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchAutoScalingGroupsPage_Empty(t *testing.T) {
	mock := &mockASGDescribeAutoScalingGroupsAPIPaginated{
		PageFunc: func(_ int) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			return &autoscaling.DescribeAutoScalingGroupsOutput{
				AutoScalingGroups: []asgtypes.AutoScalingGroup{},
				NextToken:         nil,
			}, nil
		},
	}

	result, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchAutoScalingGroupsPage_Error(t *testing.T) {
	mock := &mockASGDescribeAutoScalingGroupsAPIPaginated{
		PageFunc: func(_ int) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
			return nil, errors.New("describe asg failed")
		},
	}

	_, err := awsclient.FetchAutoScalingGroupsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ElasticBeanstalk DescribeEnvironments (paginated)
// ---------------------------------------------------------------------------

type mockEBDescribeEnvironmentsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*elasticbeanstalk.DescribeEnvironmentsOutput, error)
	lastInput *elasticbeanstalk.DescribeEnvironmentsInput
}

func (m *mockEBDescribeEnvironmentsAPIPaginated) DescribeEnvironments(_ context.Context, in *elasticbeanstalk.DescribeEnvironmentsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchEBEnvironmentsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchEBEnvironmentsPage_FirstPage(t *testing.T) {
	mock := &mockEBDescribeEnvironmentsAPIPaginated{
		PageFunc: func(_ int) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
			return &elasticbeanstalk.DescribeEnvironmentsOutput{
				Environments: []ebtypes.EnvironmentDescription{
					{
						EnvironmentName: aws.String("prod-env"),
						EnvironmentId:   aws.String("e-abc111222333"),
						ApplicationName: aws.String("my-app"),
						Status:          ebtypes.EnvironmentStatusReady,
						Health:          ebtypes.EnvironmentHealthGreen,
						VersionLabel:    aws.String("v1.0.0"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchEBEnvironmentsPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "e-abc111222333" {
		t.Errorf("resource ID: expected %q, got %q", "e-abc111222333", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchEBEnvironmentsPage_Continuation(t *testing.T) {
	mock := &mockEBDescribeEnvironmentsAPIPaginated{
		PageFunc: func(_ int) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
			return &elasticbeanstalk.DescribeEnvironmentsOutput{
				Environments: []ebtypes.EnvironmentDescription{
					{
						EnvironmentName: aws.String("staging-env"),
						EnvironmentId:   aws.String("e-xyz999888777"),
						ApplicationName: aws.String("my-app"),
						Status:          ebtypes.EnvironmentStatusReady,
						Health:          ebtypes.EnvironmentHealthYellow,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEBEnvironmentsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchEBEnvironmentsPage_Empty(t *testing.T) {
	mock := &mockEBDescribeEnvironmentsAPIPaginated{
		PageFunc: func(_ int) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
			return &elasticbeanstalk.DescribeEnvironmentsOutput{
				Environments: []ebtypes.EnvironmentDescription{},
				NextToken:    nil,
			}, nil
		},
	}

	result, err := awsclient.FetchEBEnvironmentsPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchEBEnvironmentsPage_Error(t *testing.T) {
	mock := &mockEBDescribeEnvironmentsAPIPaginated{
		PageFunc: func(_ int) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
			return nil, errors.New("describe environments failed")
		},
	}

	_, err := awsclient.FetchEBEnvironmentsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeVpcs (paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeVpcsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*ec2.DescribeVpcsOutput, error)
	lastInput *ec2.DescribeVpcsInput
}

func (m *mockEC2DescribeVpcsAPIPaginated) DescribeVpcs(_ context.Context, in *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchVPCsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchVPCsPage_FirstPage(t *testing.T) {
	isDefault := false
	mock := &mockEC2DescribeVpcsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVpcsOutput, error) {
			return &ec2.DescribeVpcsOutput{
				Vpcs: []ec2types.Vpc{
					{
						VpcId:     aws.String("vpc-0abc111222333444a"),
						CidrBlock: aws.String("10.0.0.0/16"),
						State:     ec2types.VpcStateAvailable,
						IsDefault: &isDefault,
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchVPCsPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "vpc-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "vpc-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchVPCsPage_Continuation(t *testing.T) {
	isDefault := true
	mock := &mockEC2DescribeVpcsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVpcsOutput, error) {
			return &ec2.DescribeVpcsOutput{
				Vpcs: []ec2types.Vpc{
					{
						VpcId:     aws.String("vpc-0xyz999888777666b"),
						CidrBlock: aws.String("172.16.0.0/12"),
						State:     ec2types.VpcStateAvailable,
						IsDefault: &isDefault,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchVPCsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchVPCsPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeVpcsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVpcsOutput, error) {
			return &ec2.DescribeVpcsOutput{
				Vpcs:      []ec2types.Vpc{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchVPCsPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchVPCsPage_Error(t *testing.T) {
	mock := &mockEC2DescribeVpcsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVpcsOutput, error) {
			return nil, errors.New("describe vpcs failed")
		},
	}

	_, err := awsclient.FetchVPCsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeSubnets (paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeSubnetsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*ec2.DescribeSubnetsOutput, error)
	lastInput *ec2.DescribeSubnetsInput
}

func (m *mockEC2DescribeSubnetsAPIPaginated) DescribeSubnets(_ context.Context, in *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchSubnetsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchSubnetsPage_FirstPage(t *testing.T) {
	availableIPs := int32(251)
	mock := &mockEC2DescribeSubnetsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSubnetsOutput, error) {
			return &ec2.DescribeSubnetsOutput{
				Subnets: []ec2types.Subnet{
					{
						SubnetId:                aws.String("subnet-0abc111222333444a"),
						VpcId:                   aws.String("vpc-0abc111222333444a"),
						CidrBlock:               aws.String("10.0.1.0/24"),
						AvailabilityZone:        aws.String("us-east-1a"),
						State:                   ec2types.SubnetStateAvailable,
						AvailableIpAddressCount: &availableIPs,
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchSubnetsPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "subnet-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "subnet-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchSubnetsPage_Continuation(t *testing.T) {
	availableIPs := int32(100)
	mock := &mockEC2DescribeSubnetsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSubnetsOutput, error) {
			return &ec2.DescribeSubnetsOutput{
				Subnets: []ec2types.Subnet{
					{
						SubnetId:                aws.String("subnet-0xyz999888777666b"),
						VpcId:                   aws.String("vpc-0abc111222333444a"),
						CidrBlock:               aws.String("10.0.2.0/24"),
						AvailabilityZone:        aws.String("us-east-1b"),
						State:                   ec2types.SubnetStateAvailable,
						AvailableIpAddressCount: &availableIPs,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSubnetsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchSubnetsPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeSubnetsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSubnetsOutput, error) {
			return &ec2.DescribeSubnetsOutput{
				Subnets:   []ec2types.Subnet{},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchSubnetsPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchSubnetsPage_Error(t *testing.T) {
	mock := &mockEC2DescribeSubnetsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeSubnetsOutput, error) {
			return nil, errors.New("describe subnets failed")
		},
	}

	_, err := awsclient.FetchSubnetsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeRouteTables (paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeRouteTablesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*ec2.DescribeRouteTablesOutput, error)
	lastInput *ec2.DescribeRouteTablesInput
}

func (m *mockEC2DescribeRouteTablesAPIPaginated) DescribeRouteTables(_ context.Context, in *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchRouteTablesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchRouteTablesPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeRouteTablesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeRouteTablesOutput, error) {
			return &ec2.DescribeRouteTablesOutput{
				RouteTables: []ec2types.RouteTable{
					{
						RouteTableId: aws.String("rtb-0abc111222333444a"),
						VpcId:        aws.String("vpc-0abc111222333444a"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchRouteTablesPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "rtb-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "rtb-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchRouteTablesPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeRouteTablesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeRouteTablesOutput, error) {
			return &ec2.DescribeRouteTablesOutput{
				RouteTables: []ec2types.RouteTable{
					{
						RouteTableId: aws.String("rtb-0xyz999888777666b"),
						VpcId:        aws.String("vpc-0abc111222333444a"),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchRouteTablesPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchRouteTablesPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeRouteTablesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeRouteTablesOutput, error) {
			return &ec2.DescribeRouteTablesOutput{
				RouteTables: []ec2types.RouteTable{},
				NextToken:   nil,
			}, nil
		},
	}

	result, err := awsclient.FetchRouteTablesPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchRouteTablesPage_Error(t *testing.T) {
	mock := &mockEC2DescribeRouteTablesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeRouteTablesOutput, error) {
			return nil, errors.New("describe route tables failed")
		},
	}

	_, err := awsclient.FetchRouteTablesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeNatGateways (paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeNatGatewaysAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*ec2.DescribeNatGatewaysOutput, error)
	lastInput *ec2.DescribeNatGatewaysInput
}

func (m *mockEC2DescribeNatGatewaysAPIPaginated) DescribeNatGateways(_ context.Context, in *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchNatGatewaysPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchNatGatewaysPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeNatGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeNatGatewaysOutput, error) {
			return &ec2.DescribeNatGatewaysOutput{
				NatGateways: []ec2types.NatGateway{
					{
						NatGatewayId: aws.String("nat-0abc111222333444a"),
						VpcId:        aws.String("vpc-0abc111222333444a"),
						SubnetId:     aws.String("subnet-0abc111222333444a"),
						State:        ec2types.NatGatewayStateAvailable,
						NatGatewayAddresses: []ec2types.NatGatewayAddress{
							{PublicIp: aws.String("54.1.2.3")},
						},
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchNatGatewaysPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "nat-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "nat-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchNatGatewaysPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeNatGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeNatGatewaysOutput, error) {
			return &ec2.DescribeNatGatewaysOutput{
				NatGateways: []ec2types.NatGateway{
					{
						NatGatewayId: aws.String("nat-0xyz999888777666b"),
						State:        ec2types.NatGatewayStateDeleted,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchNatGatewaysPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchNatGatewaysPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeNatGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeNatGatewaysOutput, error) {
			return &ec2.DescribeNatGatewaysOutput{
				NatGateways: []ec2types.NatGateway{},
				NextToken:   nil,
			}, nil
		},
	}

	result, err := awsclient.FetchNatGatewaysPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchNatGatewaysPage_Error(t *testing.T) {
	mock := &mockEC2DescribeNatGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeNatGatewaysOutput, error) {
			return nil, errors.New("describe nat gateways failed")
		},
	}

	_, err := awsclient.FetchNatGatewaysPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeInternetGateways (paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeInternetGatewaysAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*ec2.DescribeInternetGatewaysOutput, error)
	lastInput *ec2.DescribeInternetGatewaysInput
}

func (m *mockEC2DescribeInternetGatewaysAPIPaginated) DescribeInternetGateways(_ context.Context, in *ec2.DescribeInternetGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchInternetGatewaysPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchInternetGatewaysPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeInternetGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeInternetGatewaysOutput, error) {
			return &ec2.DescribeInternetGatewaysOutput{
				InternetGateways: []ec2types.InternetGateway{
					{
						InternetGatewayId: aws.String("igw-0abc111222333444a"),
						Attachments: []ec2types.InternetGatewayAttachment{
							{
								VpcId: aws.String("vpc-0abc111222333444a"),
								State: ec2types.AttachmentStatusAttached,
							},
						},
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchInternetGatewaysPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "igw-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "igw-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchInternetGatewaysPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeInternetGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeInternetGatewaysOutput, error) {
			return &ec2.DescribeInternetGatewaysOutput{
				InternetGateways: []ec2types.InternetGateway{
					{
						InternetGatewayId: aws.String("igw-0xyz999888777666b"),
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchInternetGatewaysPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchInternetGatewaysPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeInternetGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeInternetGatewaysOutput, error) {
			return &ec2.DescribeInternetGatewaysOutput{
				InternetGateways: []ec2types.InternetGateway{},
				NextToken:        nil,
			}, nil
		},
	}

	result, err := awsclient.FetchInternetGatewaysPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchInternetGatewaysPage_Error(t *testing.T) {
	mock := &mockEC2DescribeInternetGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeInternetGatewaysOutput, error) {
			return nil, errors.New("describe internet gateways failed")
		},
	}

	_, err := awsclient.FetchInternetGatewaysPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeNetworkInterfaces (paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeNetworkInterfacesAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*ec2.DescribeNetworkInterfacesOutput, error)
	lastInput *ec2.DescribeNetworkInterfacesInput
}

func (m *mockEC2DescribeNetworkInterfacesAPIPaginated) DescribeNetworkInterfaces(_ context.Context, in *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchNetworkInterfacesPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchNetworkInterfacesPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeNetworkInterfacesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []ec2types.NetworkInterface{
					{
						NetworkInterfaceId: aws.String("eni-0abc111222333444a"),
						Status:             ec2types.NetworkInterfaceStatusInUse,
						InterfaceType:      ec2types.NetworkInterfaceTypeInterface,
						VpcId:              aws.String("vpc-0abc111222333444a"),
						PrivateIpAddress:   aws.String("10.0.1.5"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchNetworkInterfacesPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "eni-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "eni-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchNetworkInterfacesPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeNetworkInterfacesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []ec2types.NetworkInterface{
					{
						NetworkInterfaceId: aws.String("eni-0xyz999888777666b"),
						Status:             ec2types.NetworkInterfaceStatusAvailable,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchNetworkInterfacesPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchNetworkInterfacesPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeNetworkInterfacesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []ec2types.NetworkInterface{},
				NextToken:         nil,
			}, nil
		},
	}

	result, err := awsclient.FetchNetworkInterfacesPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchNetworkInterfacesPage_Error(t *testing.T) {
	mock := &mockEC2DescribeNetworkInterfacesAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return nil, errors.New("describe network interfaces failed")
		},
	}

	_, err := awsclient.FetchNetworkInterfacesPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeVpcEndpoints (paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeVpcEndpointsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*ec2.DescribeVpcEndpointsOutput, error)
	lastInput *ec2.DescribeVpcEndpointsInput
}

func (m *mockEC2DescribeVpcEndpointsAPIPaginated) DescribeVpcEndpoints(_ context.Context, in *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchVPCEndpointsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchVPCEndpointsPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeVpcEndpointsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVpcEndpointsOutput, error) {
			return &ec2.DescribeVpcEndpointsOutput{
				VpcEndpoints: []ec2types.VpcEndpoint{
					{
						VpcEndpointId:   aws.String("vpce-0abc111222333444a"),
						ServiceName:     aws.String("com.amazonaws.us-east-1.s3"),
						VpcEndpointType: ec2types.VpcEndpointTypeGateway,
						State:           ec2types.StatePending,
						VpcId:           aws.String("vpc-0abc111222333444a"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchVPCEndpointsPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "vpce-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "vpce-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchVPCEndpointsPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeVpcEndpointsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVpcEndpointsOutput, error) {
			return &ec2.DescribeVpcEndpointsOutput{
				VpcEndpoints: []ec2types.VpcEndpoint{
					{
						VpcEndpointId:   aws.String("vpce-0xyz999888777666b"),
						ServiceName:     aws.String("com.amazonaws.us-east-1.dynamodb"),
						VpcEndpointType: ec2types.VpcEndpointTypeGateway,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchVPCEndpointsPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchVPCEndpointsPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeVpcEndpointsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVpcEndpointsOutput, error) {
			return &ec2.DescribeVpcEndpointsOutput{
				VpcEndpoints: []ec2types.VpcEndpoint{},
				NextToken:    nil,
			}, nil
		},
	}

	result, err := awsclient.FetchVPCEndpointsPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchVPCEndpointsPage_Error(t *testing.T) {
	mock := &mockEC2DescribeVpcEndpointsAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeVpcEndpointsOutput, error) {
			return nil, errors.New("describe vpc endpoints failed")
		},
	}

	_, err := awsclient.FetchVPCEndpointsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: EC2 DescribeTransitGateways (paginated)
// ---------------------------------------------------------------------------

type mockEC2DescribeTransitGatewaysAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*ec2.DescribeTransitGatewaysOutput, error)
	lastInput *ec2.DescribeTransitGatewaysInput
}

func (m *mockEC2DescribeTransitGatewaysAPIPaginated) DescribeTransitGateways(_ context.Context, in *ec2.DescribeTransitGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchTransitGatewaysPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchTransitGatewaysPage_FirstPage(t *testing.T) {
	mock := &mockEC2DescribeTransitGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeTransitGatewaysOutput, error) {
			return &ec2.DescribeTransitGatewaysOutput{
				TransitGateways: []ec2types.TransitGateway{
					{
						TransitGatewayId: aws.String("tgw-0abc111222333444a"),
						State:            ec2types.TransitGatewayStateAvailable,
						OwnerId:          aws.String("111111111111"),
						Description:      aws.String("Main transit gateway"),
					},
				},
				NextToken: aws.String("token-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchTransitGatewaysPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "tgw-0abc111222333444a" {
		t.Errorf("resource ID: expected %q, got %q", "tgw-0abc111222333444a", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchTransitGatewaysPage_Continuation(t *testing.T) {
	mock := &mockEC2DescribeTransitGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeTransitGatewaysOutput, error) {
			return &ec2.DescribeTransitGatewaysOutput{
				TransitGateways: []ec2types.TransitGateway{
					{
						TransitGatewayId: aws.String("tgw-0xyz999888777666b"),
						State:            ec2types.TransitGatewayStateDeleted,
					},
				},
				NextToken: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchTransitGatewaysPage(context.Background(), mock, "token-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextToken=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.NextToken == nil || *mock.lastInput.NextToken != "token-page-2" {
		t.Errorf("NextToken not forwarded: got %v, want %q", mock.lastInput.NextToken, "token-page-2")
	}
}

func TestQA_Pagination_FetchTransitGatewaysPage_Empty(t *testing.T) {
	mock := &mockEC2DescribeTransitGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeTransitGatewaysOutput, error) {
			return &ec2.DescribeTransitGatewaysOutput{
				TransitGateways: []ec2types.TransitGateway{},
				NextToken:       nil,
			}, nil
		},
	}

	result, err := awsclient.FetchTransitGatewaysPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchTransitGatewaysPage_Error(t *testing.T) {
	mock := &mockEC2DescribeTransitGatewaysAPIPaginated{
		PageFunc: func(_ int) (*ec2.DescribeTransitGatewaysOutput, error) {
			return nil, errors.New("describe transit gateways failed")
		},
	}

	_, err := awsclient.FetchTransitGatewaysPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ELBv2 DescribeLoadBalancers (paginated, uses NextMarker)
// ---------------------------------------------------------------------------

type mockELBv2DescribeLoadBalancersAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*elbv2.DescribeLoadBalancersOutput, error)
	lastInput *elbv2.DescribeLoadBalancersInput
}

func (m *mockELBv2DescribeLoadBalancersAPIPaginated) DescribeLoadBalancers(_ context.Context, in *elbv2.DescribeLoadBalancersInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchLoadBalancersPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchLoadBalancersPage_FirstPage(t *testing.T) {
	mock := &mockELBv2DescribeLoadBalancersAPIPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeLoadBalancersOutput, error) {
			return &elbv2.DescribeLoadBalancersOutput{
				LoadBalancers: []elbv2types.LoadBalancer{
					{
						LoadBalancerName: aws.String("my-alb"),
						DNSName:          aws.String("my-alb-111.us-east-1.elb.amazonaws.com"),
						Type:             elbv2types.LoadBalancerTypeEnumApplication,
						Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
						State:            &elbv2types.LoadBalancerState{Code: elbv2types.LoadBalancerStateEnumActive},
						VpcId:            aws.String("vpc-0abc111222333444a"),
						LoadBalancerArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:111111111111:loadbalancer/app/my-alb/abc123"),
					},
				},
				NextMarker: aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchLoadBalancersPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "my-alb" {
		t.Errorf("resource ID: expected %q, got %q", "my-alb", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchLoadBalancersPage_Continuation(t *testing.T) {
	mock := &mockELBv2DescribeLoadBalancersAPIPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeLoadBalancersOutput, error) {
			return &elbv2.DescribeLoadBalancersOutput{
				LoadBalancers: []elbv2types.LoadBalancer{
					{
						LoadBalancerName: aws.String("my-nlb"),
						Type:             elbv2types.LoadBalancerTypeEnumNetwork,
						State:            &elbv2types.LoadBalancerState{Code: elbv2types.LoadBalancerStateEnumActive},
					},
				},
				NextMarker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchLoadBalancersPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextMarker=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-page-2")
	}
}

func TestQA_Pagination_FetchLoadBalancersPage_Empty(t *testing.T) {
	mock := &mockELBv2DescribeLoadBalancersAPIPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeLoadBalancersOutput, error) {
			return &elbv2.DescribeLoadBalancersOutput{
				LoadBalancers: []elbv2types.LoadBalancer{},
				NextMarker:    nil,
			}, nil
		},
	}

	result, err := awsclient.FetchLoadBalancersPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchLoadBalancersPage_Error(t *testing.T) {
	mock := &mockELBv2DescribeLoadBalancersAPIPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeLoadBalancersOutput, error) {
			return nil, errors.New("describe load balancers failed")
		},
	}

	_, err := awsclient.FetchLoadBalancersPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mock: ELBv2 DescribeTargetGroups (paginated, uses NextMarker)
// ---------------------------------------------------------------------------

type mockELBv2DescribeTargetGroupsAPIPaginated struct {
	Calls     int
	PageFunc  func(call int) (*elbv2.DescribeTargetGroupsOutput, error)
	lastInput *elbv2.DescribeTargetGroupsInput
}

func (m *mockELBv2DescribeTargetGroupsAPIPaginated) DescribeTargetGroups(_ context.Context, in *elbv2.DescribeTargetGroupsInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error) {
	m.Calls++
	m.lastInput = in
	return m.PageFunc(m.Calls)
}

// ---------------------------------------------------------------------------
// TestQA_Pagination_FetchTargetGroupsPage
// ---------------------------------------------------------------------------

func TestQA_Pagination_FetchTargetGroupsPage_FirstPage(t *testing.T) {
	port := int32(80)
	mock := &mockELBv2DescribeTargetGroupsAPIPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeTargetGroupsOutput, error) {
			return &elbv2.DescribeTargetGroupsOutput{
				TargetGroups: []elbv2types.TargetGroup{
					{
						TargetGroupName: aws.String("my-tg"),
						Port:            &port,
						Protocol:        elbv2types.ProtocolEnumHttp,
						VpcId:           aws.String("vpc-0abc111222333444a"),
						TargetType:      elbv2types.TargetTypeEnumInstance,
						HealthCheckPath: aws.String("/health"),
						TargetGroupArn:  aws.String("arn:aws:elasticloadbalancing:us-east-1:111111111111:targetgroup/my-tg/abc123"),
					},
				},
				NextMarker: aws.String("marker-page-2"),
			}, nil
		},
	}

	result, err := awsclient.FetchTargetGroupsPage(context.Background(), mock, "")
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
	if result.Resources[0].ID != "my-tg" {
		t.Errorf("resource ID: expected %q, got %q", "my-tg", result.Resources[0].ID)
	}
}

func TestQA_Pagination_FetchTargetGroupsPage_Continuation(t *testing.T) {
	port := int32(443)
	mock := &mockELBv2DescribeTargetGroupsAPIPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeTargetGroupsOutput, error) {
			return &elbv2.DescribeTargetGroupsOutput{
				TargetGroups: []elbv2types.TargetGroup{
					{
						TargetGroupName: aws.String("my-tg-https"),
						Port:            &port,
						Protocol:        elbv2types.ProtocolEnumHttps,
						TargetType:      elbv2types.TargetTypeEnumIp,
					},
				},
				NextMarker: nil,
			}, nil
		},
	}

	result, err := awsclient.FetchTargetGroupsPage(context.Background(), mock, "marker-page-2")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Pagination.IsTruncated {
		t.Error("expected IsTruncated=false for last page (NextMarker=nil)")
	}
	if result.Pagination.NextToken != "" {
		t.Errorf("NextToken: expected empty string, got %q", result.Pagination.NextToken)
	}
	if mock.lastInput == nil {
		t.Fatal("mock was not called")
	}
	if mock.lastInput.Marker == nil || *mock.lastInput.Marker != "marker-page-2" {
		t.Errorf("Marker not forwarded: got %v, want %q", mock.lastInput.Marker, "marker-page-2")
	}
}

func TestQA_Pagination_FetchTargetGroupsPage_Empty(t *testing.T) {
	mock := &mockELBv2DescribeTargetGroupsAPIPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeTargetGroupsOutput, error) {
			return &elbv2.DescribeTargetGroupsOutput{
				TargetGroups: []elbv2types.TargetGroup{},
				NextMarker:   nil,
			}, nil
		},
	}

	result, err := awsclient.FetchTargetGroupsPage(context.Background(), mock, "")
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

func TestQA_Pagination_FetchTargetGroupsPage_Error(t *testing.T) {
	mock := &mockELBv2DescribeTargetGroupsAPIPaginated{
		PageFunc: func(_ int) (*elbv2.DescribeTargetGroupsOutput, error) {
			return nil, errors.New("describe target groups failed")
		},
	}

	_, err := awsclient.FetchTargetGroupsPage(context.Background(), mock, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
