package unit

import (
	"context"
	"fmt"
	"hash/crc32"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ecsChildPagingFake serves ECS the way the API documents it: ListServices
// returns up to 10 ARNs per page by default (maxResults 1..100), ListTasks up
// to 100 (maxResults 1..100); DescribeServices accepts at most 10 services
// and DescribeTasks at most 100 tasks per call, rejecting more with
// InvalidParameterException.
type ecsChildPagingFake struct {
	awsclient.ECSAPI
	clusters []string
	services map[string]int // cluster ARN -> service count
	tasks    map[string]int // cluster ARN -> task count
}

func ecsPagingClusterARN(name string) string {
	return "arn:aws:ecs:us-east-1:123456789012:cluster/" + name
}

func ecsPagingClusterName(arn string) string { return arn[strings.LastIndex(arn, "/")+1:] }

func ecsPagingServiceName(cluster string, i int) string {
	return fmt.Sprintf("%s-svc-%03d", cluster, i)
}

// ecsPagingServiceID is the row ID of that service: a service name is unique
// inside its cluster, so the row carries the cluster with it.
func ecsPagingServiceID(cluster string, i int) string {
	return cluster + "/" + ecsPagingServiceName(cluster, i)
}

func ecsPagingTaskID(cluster string, i int) string {
	return fmt.Sprintf("%08x%024x", crc32.ChecksumIEEE([]byte(cluster)), i)
}

func ecsPagingPage(total int, token *string, maxResults *int32, def int) (lo, hi int, next *string) {
	size := def
	if maxResults != nil {
		size = int(*maxResults)
	}
	if token != nil {
		_, _ = fmt.Sscanf(*token, "%d", &lo)
	}
	hi = min(lo+size, total)
	if hi < total {
		next = aws.String(fmt.Sprintf("%d", hi))
	}
	return lo, hi, next
}

func (f *ecsChildPagingFake) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{ClusterArns: f.clusters}, nil
}

func (f *ecsChildPagingFake) ListServices(_ context.Context, in *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	cluster := aws.ToString(in.Cluster)
	lo, hi, next := ecsPagingPage(f.services[cluster], in.NextToken, in.MaxResults, 10)
	out := &ecs.ListServicesOutput{NextToken: next}
	name := ecsPagingClusterName(cluster)
	for i := lo; i < hi; i++ {
		out.ServiceArns = append(out.ServiceArns, fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:service/%s/%s", name, ecsPagingServiceName(name, i)))
	}
	return out, nil
}

func (f *ecsChildPagingFake) DescribeServices(_ context.Context, in *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	if len(in.Services) > 10 {
		return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "services can have at most 10 items."}
	}
	out := &ecs.DescribeServicesOutput{}
	for _, arn := range in.Services {
		out.Services = append(out.Services, ecstypes.Service{
			ServiceArn:     aws.String(arn),
			ServiceName:    aws.String(arn[strings.LastIndex(arn, "/")+1:]),
			ClusterArn:     in.Cluster,
			Status:         aws.String("ACTIVE"),
			DesiredCount:   2,
			RunningCount:   2,
			LaunchType:     ecstypes.LaunchTypeFargate,
			TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web:7"),
		})
	}
	return out, nil
}

func (f *ecsChildPagingFake) ListTasks(_ context.Context, in *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	cluster := aws.ToString(in.Cluster)
	lo, hi, next := ecsPagingPage(f.tasks[cluster], in.NextToken, in.MaxResults, 100)
	out := &ecs.ListTasksOutput{NextToken: next}
	name := ecsPagingClusterName(cluster)
	for i := lo; i < hi; i++ {
		out.TaskArns = append(out.TaskArns, fmt.Sprintf("arn:aws:ecs:us-east-1:123456789012:task/%s/%s", name, ecsPagingTaskID(name, i)))
	}
	return out, nil
}

func (f *ecsChildPagingFake) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	if len(in.Tasks) > 100 {
		return nil, &smithy.GenericAPIError{Code: "InvalidParameterException", Message: "tasks can have at most 100 items."}
	}
	out := &ecs.DescribeTasksOutput{}
	for _, arn := range in.Tasks {
		out.Tasks = append(out.Tasks, ecstypes.Task{
			TaskArn:           aws.String(arn),
			ClusterArn:        in.Cluster,
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/web:7"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("256"),
			Memory:            aws.String("512"),
			HealthStatus:      ecstypes.HealthStatusHealthy,
		})
	}
	return out, nil
}

func (f *ecsChildPagingFake) DescribeTaskDefinition(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{
		TaskDefinitionArn: in.TaskDefinition,
		Family:            aws.String("web"),
		Revision:          7,
	}}, nil
}

func ecsPagingCollect(t *testing.T, shortName string, f *ecsChildPagingFake) []string {
	t.Helper()
	fetch := resource.GetPaginatedFetcher(shortName)
	if fetch == nil {
		t.Fatalf("no paginated fetcher registered for %s", shortName)
	}
	var ids []string
	token := ""
	for page := 0; ; page++ {
		if page > 100 {
			t.Fatalf("%s: more than 100 pages, the continuation never ends", shortName)
		}
		res, err := fetch(context.Background(), &awsclient.ServiceClients{ECS: f}, token)
		if err != nil {
			t.Fatalf("%s page %d: %v", shortName, page, err)
		}
		for _, r := range res.Resources {
			ids = append(ids, r.ID)
		}
		if res.Pagination == nil || !res.Pagination.IsTruncated {
			return ids
		}
		token = res.Pagination.NextToken
		if token == "" {
			t.Fatalf("%s page %d: truncated with no continuation token", shortName, page)
		}
	}
}

func assertECSPagingIDs(t *testing.T, got, want []string) {
	t.Helper()
	seen := map[string]int{}
	for _, id := range got {
		seen[id]++
		if seen[id] == 2 {
			t.Errorf("id %q listed twice", id)
		}
	}
	got = slices.Sorted(slices.Values(got))
	want = slices.Sorted(slices.Values(want))
	if !slices.Equal(got, want) {
		var missing []string
		for _, w := range want {
			if seen[w] == 0 {
				missing = append(missing, w)
			}
		}
		t.Errorf("listed %d, want %d; %d missing, first %v", len(got), len(want), len(missing), missing[:min(3, len(missing))])
	}
}

// A cluster with 11 services answers ListServices in two pages (10 + 1);
// the ecs-svc list shows all 11, and every service of the next cluster.
func TestECSServicesList_WalksEveryListServicesPage(t *testing.T) {
	a, b := ecsPagingClusterARN("prod-web"), ecsPagingClusterARN("prod-batch")
	f := &ecsChildPagingFake{
		clusters: []string{a, b},
		services: map[string]int{a: 11, b: 3},
	}
	got := ecsPagingCollect(t, "ecs-svc", f)
	var want []string
	for i := range 11 {
		want = append(want, ecsPagingServiceID("prod-web", i))
	}
	for i := range 3 {
		want = append(want, ecsPagingServiceID("prod-batch", i))
	}
	assertECSPagingIDs(t, got, want)
}

// 120 services in one cluster exceed one list page of the resource list
// (50 rows): the continuation resumes inside the cluster, not after it.
func TestECSServicesList_ResumesInsideALargeCluster(t *testing.T) {
	a, b := ecsPagingClusterARN("prod-web"), ecsPagingClusterARN("prod-batch")
	f := &ecsChildPagingFake{
		clusters: []string{a, b},
		services: map[string]int{a: 120, b: 2},
	}
	got := ecsPagingCollect(t, "ecs-svc", f)
	var want []string
	for i := range 120 {
		want = append(want, ecsPagingServiceID("prod-web", i))
	}
	for i := range 2 {
		want = append(want, ecsPagingServiceID("prod-batch", i))
	}
	assertECSPagingIDs(t, got, want)
}

// A cluster running 130 tasks answers ListTasks in two pages (100 + 30);
// the ecs-task list shows all 130 and the next cluster's tasks.
func TestECSTasksList_WalksEveryListTasksPage(t *testing.T) {
	a, b := ecsPagingClusterARN("prod-web"), ecsPagingClusterARN("prod-batch")
	f := &ecsChildPagingFake{
		clusters: []string{a, b},
		tasks:    map[string]int{a: 130, b: 4},
	}
	got := ecsPagingCollect(t, "ecs-task", f)
	var want []string
	for i := range 130 {
		want = append(want, ecsPagingTaskID("prod-web", i))
	}
	for i := range 4 {
		want = append(want, ecsPagingTaskID("prod-batch", i))
	}
	assertECSPagingIDs(t, got, want)
}
