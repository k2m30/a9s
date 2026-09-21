package unit

// b1_request_completeness_test.go — the request parameters each list and
// describe call must carry for the view above it to render every row and
// field it claims.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestB1FetchAMIsPage_AsksForDisabledImages(t *testing.T) {
	mock := &capturingDescribeImagesClient{output: &ec2.DescribeImagesOutput{}}

	if _, err := awsclient.FetchAMIsPage(context.Background(), mock, ""); err != nil {
		t.Fatalf("FetchAMIsPage: %v", err)
	}
	if len(mock.inputs) != 1 {
		t.Fatalf("DescribeImages called %d times, want 1", len(mock.inputs))
	}
	if !aws.ToBool(mock.inputs[0].IncludeDisabled) {
		t.Error("IncludeDisabled is not set: an AMI the account disabled is missing from the list")
	}
}

func TestB1FetchAMIsByIDs_AsksForDisabledImages(t *testing.T) {
	mock := &capturingDescribeImagesClient{output: &ec2.DescribeImagesOutput{
		Images: []ec2types.Image{{ImageId: aws.String("ami-0abc111222333444a")}},
	}}

	if _, err := awsclient.FetchAMIsByIDs(context.Background(), mock, []string{"ami-0abc111222333444a"}); err != nil {
		t.Fatalf("FetchAMIsByIDs: %v", err)
	}
	if len(mock.inputs) != 1 {
		t.Fatalf("DescribeImages called %d times, want 1", len(mock.inputs))
	}
	if !aws.ToBool(mock.inputs[0].IncludeDisabled) {
		t.Error("IncludeDisabled is not set: a pivot onto a disabled AMI resolves to nothing")
	}
}

func TestB1FetchNetworkInterfaces_AsksForManagedResources(t *testing.T) {
	mock := &mockENIPaginatedClient{}

	if _, err := awsclient.FetchNetworkInterfacesPage(context.Background(), mock, ""); err != nil {
		t.Fatalf("FetchNetworkInterfacesPage: %v", err)
	}
	if len(mock.inputs) != 1 {
		t.Fatalf("DescribeNetworkInterfaces called %d times, want 1", len(mock.inputs))
	}
	if !aws.ToBool(mock.inputs[0].IncludeManagedResources) {
		t.Error("IncludeManagedResources is not set: an interface a service owns is missing, and the security group it holds reads as unused")
	}
}

// b1DescribeServicesCapture records the Include list of every DescribeServices
// request.
type b1DescribeServicesCapture struct {
	output *ecs.DescribeServicesOutput
	inputs []*ecs.DescribeServicesInput
}

func (m *b1DescribeServicesCapture) DescribeServices(_ context.Context, params *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	m.inputs = append(m.inputs, params)
	return m.output, nil
}

func TestB1FetchECSServices_AsksForTags(t *testing.T) {
	clusterARN := "arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"
	listClusters := &mockECSListClustersClient{output: &ecs.ListClustersOutput{ClusterArns: []string{clusterARN}}}
	listServices := &mockECSListServicesClient{outputs: map[string]*ecs.ListServicesOutput{
		clusterARN: {ServiceArns: []string{"arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/web"}},
	}}
	describe := &b1DescribeServicesCapture{output: &ecs.DescribeServicesOutput{}}

	if _, err := awsclient.FetchECSServicesPage(context.Background(), listClusters, listServices, describe, ""); err != nil {
		t.Fatalf("FetchECSServicesPage: %v", err)
	}
	if len(describe.inputs) != 1 {
		t.Fatalf("DescribeServices called %d times, want 1", len(describe.inputs))
	}
	if !b1HasServiceField(describe.inputs[0].Include, ecstypes.ServiceFieldTags) {
		t.Errorf("Include = %v, want it to name TAGS: without them the service detail shows no tags and the CloudFormation pivot resolves nothing", describe.inputs[0].Include)
	}
}

func b1HasServiceField(fields []ecstypes.ServiceField, want ecstypes.ServiceField) bool {
	for _, f := range fields {
		if f == want {
			return true
		}
	}
	return false
}

func b1HasTaskField(fields []ecstypes.TaskField, want ecstypes.TaskField) bool {
	for _, f := range fields {
		if f == want {
			return true
		}
	}
	return false
}

func TestB1FetchECSTasks_AsksForTags(t *testing.T) {
	var got []*ecs.DescribeTasksInput
	mock := buildECSTaskMockAPI(nil)
	mock.describeTasksFn = func(in *ecs.DescribeTasksInput) (*ecs.DescribeTasksOutput, error) {
		got = append(got, in)
		return &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{{
			TaskArn:           aws.String(testTaskARN),
			ClusterArn:        aws.String(testClusterARN),
			LastStatus:        aws.String("RUNNING"),
			TaskDefinitionArn: aws.String(testTaskDefARN),
		}}}, nil
	}

	fetcher := ecsTaskPaginatedFetcher(t)
	if _, err := fetcher(context.Background(), &awsclient.ServiceClients{ECS: mock}, ""); err != nil {
		t.Fatalf("ecs-task fetcher: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("DescribeTasks called %d times, want 1", len(got))
	}
	if !b1HasTaskField(got[0].Include, ecstypes.TaskFieldTags) {
		t.Errorf("Include = %v, want it to name TAGS: without them the task detail shows no tags", got[0].Include)
	}
}

// b1SvcTasksAPI answers the two calls FetchEcsSvcTasks makes and records the
// describe requests.
type b1SvcTasksAPI struct {
	awsclient.ECSAPI
	describes []*ecs.DescribeTasksInput
}

func (f *b1SvcTasksAPI) ListTasks(_ context.Context, in *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	if in.DesiredStatus == ecstypes.DesiredStatusRunning {
		return &ecs.ListTasksOutput{TaskArns: []string{testTaskARN}}, nil
	}
	return &ecs.ListTasksOutput{}, nil
}

func (f *b1SvcTasksAPI) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	f.describes = append(f.describes, in)
	return &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{{
		TaskArn:           aws.String(testTaskARN),
		ClusterArn:        aws.String(testClusterARN),
		LastStatus:        aws.String("RUNNING"),
		TaskDefinitionArn: aws.String(testTaskDefARN),
	}}}, nil
}

func TestB1FetchEcsSvcTasks_AsksForTags(t *testing.T) {
	fake := &b1SvcTasksAPI{}

	if _, err := awsclient.FetchEcsSvcTasks(context.Background(), fake, fake, testClusterARN, "web", ""); err != nil {
		t.Fatalf("FetchEcsSvcTasks: %v", err)
	}
	if len(fake.describes) != 1 {
		t.Fatalf("DescribeTasks called %d times, want 1", len(fake.describes))
	}
	if !b1HasTaskField(fake.describes[0].Include, ecstypes.TaskFieldTags) {
		t.Errorf("Include = %v, want it to name TAGS", fake.describes[0].Include)
	}
}

// b1SnapshotsCapture records every DescribeSnapshots request the ebs-snap
// enricher issues.
type b1SnapshotsCapture struct {
	awsclient.EC2API
	inputs []*ec2.DescribeSnapshotsInput
}

func (f *b1SnapshotsCapture) DescribeSnapshots(_ context.Context, in *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	f.inputs = append(f.inputs, in)
	return &ec2.DescribeSnapshotsOutput{}, nil
}

func TestB1EBSSnapPublicShares_BoundsThePage(t *testing.T) {
	fake := &b1SnapshotsCapture{}
	rs := []resource.Resource{{
		ID:        "snap-0aaaa1111bbbb2222",
		RawStruct: ec2types.Snapshot{SnapshotId: aws.String("snap-0aaaa1111bbbb2222"), VolumeId: aws.String("vol-0aaaa1111bbbb2222")},
		Fields:    map[string]string{"snapshot_id": "snap-0aaaa1111bbbb2222", "volume_id": "vol-0aaaa1111bbbb2222"},
	}}

	if _, err := pw1EBSSnapEnricher(t)(context.Background(),
		&awsclient.ServiceClients{EC2: fake}, rs, pw1EBSCache("vol-0aaaa1111bbbb2222")); err != nil {
		t.Fatalf("ebs-snap enricher: %v", err)
	}
	if len(fake.inputs) == 0 {
		t.Fatal("DescribeSnapshots was never called")
	}
	if fake.inputs[0].MaxResults == nil {
		t.Error("MaxResults is not set: the first page is whatever size AWS chooses, ahead of the walker's page cap")
	}
}
