package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ===========================================================================
// 1. ECS Clusters — ListClusters pagination (NextToken)
//    Current code calls ListClusters once without following NextToken.
// ===========================================================================

// mockPaginatedECSListClustersClient returns multiple pages of ListClusters
// results, controlled by NextToken.
type mockPaginatedECSListClustersClient struct {
	outputs []*ecs.ListClustersOutput
	inputs  []*ecs.ListClustersInput
	callIdx int
}

func (m *mockPaginatedECSListClustersClient) ListClusters(
	ctx context.Context,
	params *ecs.ListClustersInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListClustersOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.callIdx >= len(m.outputs) {
		return &ecs.ListClustersOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

// mockPaginatedECSDescribeClustersClient returns cluster details for
// whichever ARNs are requested.
type mockPaginatedECSDescribeClustersClient struct {
	clustersByARN map[string]ecstypes.Cluster
	callCount     int
}

func (m *mockPaginatedECSDescribeClustersClient) DescribeClusters(
	ctx context.Context,
	params *ecs.DescribeClustersInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeClustersOutput, error) {
	m.callCount++
	var clusters []ecstypes.Cluster
	for _, arn := range params.Clusters {
		if c, ok := m.clustersByARN[arn]; ok {
			clusters = append(clusters, c)
		}
	}
	return &ecs.DescribeClustersOutput{Clusters: clusters}, nil
}

func TestFetchECSClusters_Pagination(t *testing.T) {
	arn1 := "arn:aws:ecs:us-east-1:123456789012:cluster/page1-cluster-1"
	arn2 := "arn:aws:ecs:us-east-1:123456789012:cluster/page1-cluster-2"
	arn3 := "arn:aws:ecs:us-east-1:123456789012:cluster/page2-cluster-1"

	listMock := &mockPaginatedECSListClustersClient{
		outputs: []*ecs.ListClustersOutput{
			{
				NextToken:   aws.String("page2-token"),
				ClusterArns: []string{arn1, arn2},
			},
			{
				ClusterArns: []string{arn3},
			},
		},
	}

	describeMock := &mockPaginatedECSDescribeClustersClient{
		clustersByARN: map[string]ecstypes.Cluster{
			arn1: {ClusterName: aws.String("page1-cluster-1"), Status: aws.String("ACTIVE"), RunningTasksCount: 3, ActiveServicesCount: 2},
			arn2: {ClusterName: aws.String("page1-cluster-2"), Status: aws.String("ACTIVE"), RunningTasksCount: 1, ActiveServicesCount: 1},
			arn3: {ClusterName: aws.String("page2-cluster-1"), Status: aws.String("ACTIVE"), RunningTasksCount: 5, ActiveServicesCount: 3},
		},
	}

	resources, err := awsclient.FetchECSClusters(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 clusters across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-cluster-1" {
			t.Errorf("expected %q, got %q", "page1-cluster-1", resources[0].ID)
		}
	})

	t.Run("page1_second", func(t *testing.T) {
		if len(resources) < 2 {
			t.Skip("not enough resources")
		}
		if resources[1].ID != "page1-cluster-2" {
			t.Errorf("expected %q, got %q", "page1-cluster-2", resources[1].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-cluster-1" {
			t.Errorf("expected %q, got %q", "page2-cluster-1", resources[2].ID)
		}
	})

	t.Run("list_api_called_twice", func(t *testing.T) {
		if listMock.callIdx != 2 {
			t.Errorf("expected ListClusters called 2 times, got %d", listMock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(listMock.inputs) < 2 {
			t.Fatalf("expected at least 2 ListClusters inputs captured, got %d", len(listMock.inputs))
		}
		if listMock.inputs[1].NextToken == nil || *listMock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", listMock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// 2. ECS Services — ListClusters pagination affects service discovery
//    Current code calls ListClusters once; services in clusters from page 2
//    are never discovered.
// ===========================================================================

// mockPaginatedECSSvcListClustersClient returns paginated ListClusters results
// for the ECS Services fetcher.
type mockPaginatedECSSvcListClustersClient struct {
	outputs []*ecs.ListClustersOutput
	inputs  []*ecs.ListClustersInput
	callIdx int
}

func (m *mockPaginatedECSSvcListClustersClient) ListClusters(
	ctx context.Context,
	params *ecs.ListClustersInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListClustersOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.callIdx >= len(m.outputs) {
		return &ecs.ListClustersOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

// mockPaginatedECSSvcListServicesClient returns services per cluster.
type mockPaginatedECSSvcListServicesClient struct {
	servicesByCluster map[string]*ecs.ListServicesOutput
}

func (m *mockPaginatedECSSvcListServicesClient) ListServices(
	ctx context.Context,
	params *ecs.ListServicesInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListServicesOutput, error) {
	if out, ok := m.servicesByCluster[*params.Cluster]; ok {
		return out, nil
	}
	return &ecs.ListServicesOutput{}, nil
}

// mockPaginatedECSSvcDescribeServicesClient returns service details for
// any requested service ARNs.
type mockPaginatedECSSvcDescribeServicesClient struct {
	servicesByARN map[string]ecstypes.Service
	callCount     int
}

func (m *mockPaginatedECSSvcDescribeServicesClient) DescribeServices(
	ctx context.Context,
	params *ecs.DescribeServicesInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeServicesOutput, error) {
	m.callCount++
	var services []ecstypes.Service
	for _, arn := range params.Services {
		if s, ok := m.servicesByARN[arn]; ok {
			services = append(services, s)
		}
	}
	return &ecs.DescribeServicesOutput{Services: services}, nil
}

func TestFetchECSServices_PaginatedListClusters(t *testing.T) {
	cluster1ARN := "arn:aws:ecs:us-east-1:123456789012:cluster/cluster-1"
	cluster2ARN := "arn:aws:ecs:us-east-1:123456789012:cluster/cluster-2"

	svc1ARN := "arn:aws:ecs:us-east-1:123456789012:service/cluster-1/svc-alpha"
	svc2ARN := "arn:aws:ecs:us-east-1:123456789012:service/cluster-2/svc-beta"

	listClustersMock := &mockPaginatedECSSvcListClustersClient{
		outputs: []*ecs.ListClustersOutput{
			{
				NextToken:   aws.String("page2-token"),
				ClusterArns: []string{cluster1ARN},
			},
			{
				ClusterArns: []string{cluster2ARN},
			},
		},
	}

	listServicesMock := &mockPaginatedECSSvcListServicesClient{
		servicesByCluster: map[string]*ecs.ListServicesOutput{
			cluster1ARN: {ServiceArns: []string{svc1ARN}},
			cluster2ARN: {ServiceArns: []string{svc2ARN}},
		},
	}

	describeServicesMock := &mockPaginatedECSSvcDescribeServicesClient{
		servicesByARN: map[string]ecstypes.Service{
			svc1ARN: {
				ServiceName:    aws.String("svc-alpha"),
				ClusterArn:     aws.String(cluster1ARN),
				Status:         aws.String("ACTIVE"),
				DesiredCount:   2,
				RunningCount:   2,
				LaunchType:     ecstypes.LaunchTypeFargate,
				TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web:1"),
			},
			svc2ARN: {
				ServiceName:    aws.String("svc-beta"),
				ClusterArn:     aws.String(cluster2ARN),
				Status:         aws.String("ACTIVE"),
				DesiredCount:   3,
				RunningCount:   3,
				LaunchType:     ecstypes.LaunchTypeEc2,
				TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api:2"),
			},
		},
	}

	resources, err := awsclient.FetchECSServices(
		context.Background(),
		listClustersMock,
		listServicesMock,
		describeServicesMock,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 2 {
			t.Fatalf("expected 2 services across 2 cluster pages, got %d", len(resources))
		}
	})

	t.Run("service_from_page1_cluster", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "svc-alpha" {
			t.Errorf("expected %q, got %q", "svc-alpha", resources[0].ID)
		}
	})

	t.Run("service_from_page2_cluster", func(t *testing.T) {
		if len(resources) < 2 {
			t.Skip("not enough resources")
		}
		if resources[1].ID != "svc-beta" {
			t.Errorf("expected %q, got %q", "svc-beta", resources[1].ID)
		}
	})

	t.Run("list_clusters_called_twice", func(t *testing.T) {
		if listClustersMock.callIdx != 2 {
			t.Errorf("expected ListClusters called 2 times, got %d", listClustersMock.callIdx)
		}
	})

	t.Run("describe_services_called_for_both_clusters", func(t *testing.T) {
		if describeServicesMock.callCount != 2 {
			t.Errorf("expected DescribeServices called 2 times (once per cluster), got %d", describeServicesMock.callCount)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(listClustersMock.inputs) < 2 {
			t.Fatalf("expected at least 2 ListClusters inputs captured, got %d", len(listClustersMock.inputs))
		}
		if listClustersMock.inputs[1].NextToken == nil || *listClustersMock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", listClustersMock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// 3. ECS Tasks — ListClusters pagination affects task discovery
//    Current code calls ListClusters once; tasks in clusters from page 2
//    are never discovered.
// ===========================================================================

// mockPaginatedECSTaskListClustersClient returns paginated ListClusters results
// for the ECS Tasks fetcher.
type mockPaginatedECSTaskListClustersClient struct {
	outputs []*ecs.ListClustersOutput
	inputs  []*ecs.ListClustersInput
	callIdx int
}

func (m *mockPaginatedECSTaskListClustersClient) ListClusters(
	ctx context.Context,
	params *ecs.ListClustersInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListClustersOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.callIdx >= len(m.outputs) {
		return &ecs.ListClustersOutput{}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

// mockPaginatedECSTaskListTasksClient returns tasks per cluster.
type mockPaginatedECSTaskListTasksClient struct {
	tasksByCluster map[string]*ecs.ListTasksOutput
}

func (m *mockPaginatedECSTaskListTasksClient) ListTasks(
	ctx context.Context,
	params *ecs.ListTasksInput,
	optFns ...func(*ecs.Options),
) (*ecs.ListTasksOutput, error) {
	if out, ok := m.tasksByCluster[*params.Cluster]; ok {
		return out, nil
	}
	return &ecs.ListTasksOutput{}, nil
}

// mockPaginatedECSTaskDescribeTasksClient returns task details for
// any requested task ARNs.
type mockPaginatedECSTaskDescribeTasksClient struct {
	tasksByARN map[string]ecstypes.Task
	callCount  int
}

func (m *mockPaginatedECSTaskDescribeTasksClient) DescribeTasks(
	ctx context.Context,
	params *ecs.DescribeTasksInput,
	optFns ...func(*ecs.Options),
) (*ecs.DescribeTasksOutput, error) {
	m.callCount++
	var tasks []ecstypes.Task
	for _, arn := range params.Tasks {
		if task, ok := m.tasksByARN[arn]; ok {
			tasks = append(tasks, task)
		}
	}
	return &ecs.DescribeTasksOutput{Tasks: tasks}, nil
}

func TestFetchECSTasks_PaginatedListClusters(t *testing.T) {
	cluster1ARN := "arn:aws:ecs:us-east-1:123456789012:cluster/cluster-1"
	cluster2ARN := "arn:aws:ecs:us-east-1:123456789012:cluster/cluster-2"

	task1ARN := "arn:aws:ecs:us-east-1:123456789012:task/cluster-1/aaaa1111"
	task2ARN := "arn:aws:ecs:us-east-1:123456789012:task/cluster-2/bbbb2222"

	listClustersMock := &mockPaginatedECSTaskListClustersClient{
		outputs: []*ecs.ListClustersOutput{
			{
				NextToken:   aws.String("page2-token"),
				ClusterArns: []string{cluster1ARN},
			},
			{
				ClusterArns: []string{cluster2ARN},
			},
		},
	}

	listTasksMock := &mockPaginatedECSTaskListTasksClient{
		tasksByCluster: map[string]*ecs.ListTasksOutput{
			cluster1ARN: {TaskArns: []string{task1ARN}},
			cluster2ARN: {TaskArns: []string{task2ARN}},
		},
	}

	describeTasksMock := &mockPaginatedECSTaskDescribeTasksClient{
		tasksByARN: map[string]ecstypes.Task{
			task1ARN: {
				TaskArn:           aws.String(task1ARN),
				ClusterArn:        aws.String(cluster1ARN),
				LastStatus:        aws.String("RUNNING"),
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web:1"),
				LaunchType:        ecstypes.LaunchTypeFargate,
				Cpu:               aws.String("256"),
				Memory:            aws.String("512"),
			},
			task2ARN: {
				TaskArn:           aws.String(task2ARN),
				ClusterArn:        aws.String(cluster2ARN),
				LastStatus:        aws.String("RUNNING"),
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api:2"),
				LaunchType:        ecstypes.LaunchTypeEc2,
				Cpu:               aws.String("512"),
				Memory:            aws.String("1024"),
			},
		},
	}

	resources, err := awsclient.FetchECSTasks(
		context.Background(),
		listClustersMock,
		listTasksMock,
		describeTasksMock,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 2 {
			t.Fatalf("expected 2 tasks across 2 cluster pages, got %d", len(resources))
		}
	})

	t.Run("task_from_page1_cluster", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "aaaa1111" {
			t.Errorf("expected %q, got %q", "aaaa1111", resources[0].ID)
		}
	})

	t.Run("task_from_page2_cluster", func(t *testing.T) {
		if len(resources) < 2 {
			t.Skip("not enough resources")
		}
		if resources[1].ID != "bbbb2222" {
			t.Errorf("expected %q, got %q", "bbbb2222", resources[1].ID)
		}
	})

	t.Run("list_clusters_called_twice", func(t *testing.T) {
		if listClustersMock.callIdx != 2 {
			t.Errorf("expected ListClusters called 2 times, got %d", listClustersMock.callIdx)
		}
	})

	t.Run("describe_tasks_called_for_both_clusters", func(t *testing.T) {
		if describeTasksMock.callCount != 2 {
			t.Errorf("expected DescribeTasks called 2 times (once per cluster), got %d", describeTasksMock.callCount)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(listClustersMock.inputs) < 2 {
			t.Fatalf("expected at least 2 ListClusters inputs captured, got %d", len(listClustersMock.inputs))
		}
		if listClustersMock.inputs[1].NextToken == nil || *listClustersMock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", listClustersMock.inputs[1].NextToken, "page2-token")
		}
	})
}

// ===========================================================================
// 4. Kinesis — ListStreams pagination (HasMoreStreams + NextToken)
//    Current code calls ListStreams once without following pagination.
// ===========================================================================

// mockPaginatedKinesisClient returns multiple pages of ListStreams results.
type mockPaginatedKinesisClient struct {
	outputs []*kinesis.ListStreamsOutput
	inputs  []*kinesis.ListStreamsInput
	callIdx int
}

func (m *mockPaginatedKinesisClient) ListStreams(
	ctx context.Context,
	params *kinesis.ListStreamsInput,
	optFns ...func(*kinesis.Options),
) (*kinesis.ListStreamsOutput, error) {
	m.inputs = append(m.inputs, params)
	if m.callIdx >= len(m.outputs) {
		return &kinesis.ListStreamsOutput{
			HasMoreStreams: aws.Bool(false),
			StreamNames:    []string{},
		}, nil
	}
	out := m.outputs[m.callIdx]
	m.callIdx++
	return out, nil
}

func TestFetchKinesisStreams_Pagination(t *testing.T) {
	mock := &mockPaginatedKinesisClient{
		outputs: []*kinesis.ListStreamsOutput{
			{
				HasMoreStreams: aws.Bool(true),
				NextToken:      aws.String("page2-token"),
				StreamNames:    []string{"page1-stream-1", "page1-stream-2"},
				StreamSummaries: []kinesistypes.StreamSummary{
					{
						StreamName:   aws.String("page1-stream-1"),
						StreamARN:    aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/page1-stream-1"),
						StreamStatus: kinesistypes.StreamStatusActive,
					},
					{
						StreamName:   aws.String("page1-stream-2"),
						StreamARN:    aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/page1-stream-2"),
						StreamStatus: kinesistypes.StreamStatusActive,
					},
				},
			},
			{
				HasMoreStreams: aws.Bool(false),
				StreamNames:    []string{"page2-stream-1"},
				StreamSummaries: []kinesistypes.StreamSummary{
					{
						StreamName:   aws.String("page2-stream-1"),
						StreamARN:    aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/page2-stream-1"),
						StreamStatus: kinesistypes.StreamStatusActive,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchKinesisStreams(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	t.Run("total_count", func(t *testing.T) {
		if len(resources) != 3 {
			t.Fatalf("expected 3 streams across 2 pages, got %d", len(resources))
		}
	})

	t.Run("page1_first", func(t *testing.T) {
		if len(resources) < 1 {
			t.Skip("not enough resources")
		}
		if resources[0].ID != "page1-stream-1" {
			t.Errorf("expected %q, got %q", "page1-stream-1", resources[0].ID)
		}
	})

	t.Run("page1_second", func(t *testing.T) {
		if len(resources) < 2 {
			t.Skip("not enough resources")
		}
		if resources[1].ID != "page1-stream-2" {
			t.Errorf("expected %q, got %q", "page1-stream-2", resources[1].ID)
		}
	})

	t.Run("page2_first", func(t *testing.T) {
		if len(resources) < 3 {
			t.Skip("not enough resources")
		}
		if resources[2].ID != "page2-stream-1" {
			t.Errorf("expected %q, got %q", "page2-stream-1", resources[2].ID)
		}
	})

	t.Run("api_called_twice", func(t *testing.T) {
		if mock.callIdx != 2 {
			t.Errorf("expected ListStreams called 2 times, got %d", mock.callIdx)
		}
	})

	t.Run("page2_received_token", func(t *testing.T) {
		if len(mock.inputs) < 2 {
			t.Fatalf("expected at least 2 inputs captured, got %d", len(mock.inputs))
		}
		if mock.inputs[1].NextToken == nil || *mock.inputs[1].NextToken != "page2-token" {
			t.Errorf("NextToken not forwarded to page 2: got %v, want %q", mock.inputs[1].NextToken, "page2-token")
		}
	})
}
