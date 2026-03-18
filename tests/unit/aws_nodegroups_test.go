package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/tests/testdata"
)

// ---------------------------------------------------------------------------
// T-NG01 - Test Node Groups three-step fetch (ListClusters -> ListNodegroups -> DescribeNodegroup)
// ---------------------------------------------------------------------------

func TestFetchNodeGroups_ParsesMultipleClustersAndGroups(t *testing.T) {
	listClustersMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{"cluster-a", "cluster-b"},
		},
	}

	listNGMock := &mockEKSListNodegroupsClient{
		outputs: map[string]*eks.ListNodegroupsOutput{
			"cluster-a": {Nodegroups: []string{"ng-web", "ng-worker"}},
			"cluster-b": {Nodegroups: []string{"ng-api"}},
		},
	}

	desiredSize := int32(3)
	minSize := int32(1)
	maxSize := int32(10)

	desiredSize2 := int32(5)
	minSize2 := int32(2)
	maxSize2 := int32(8)

	desiredSize3 := int32(2)
	minSize3 := int32(1)
	maxSize3 := int32(4)

	describeNGMock := &mockEKSDescribeNodegroupClient{
		outputs: map[string]*eks.DescribeNodegroupOutput{
			"cluster-a/ng-web": {
				Nodegroup: &ekstypes.Nodegroup{
					NodegroupName: aws.String("ng-web"),
					ClusterName:   aws.String("cluster-a"),
					Status:        ekstypes.NodegroupStatusActive,
					InstanceTypes: []string{"t3.medium"},
					ScalingConfig: &ekstypes.NodegroupScalingConfig{
						DesiredSize: &desiredSize,
						MinSize:     &minSize,
						MaxSize:     &maxSize,
					},
					AmiType:      ekstypes.AMITypesAl2X8664,
					CapacityType: ekstypes.CapacityTypesOnDemand,
				},
			},
			"cluster-a/ng-worker": {
				Nodegroup: &ekstypes.Nodegroup{
					NodegroupName: aws.String("ng-worker"),
					ClusterName:   aws.String("cluster-a"),
					Status:        ekstypes.NodegroupStatusActive,
					InstanceTypes: []string{"m5.large", "m5.xlarge"},
					ScalingConfig: &ekstypes.NodegroupScalingConfig{
						DesiredSize: &desiredSize2,
						MinSize:     &minSize2,
						MaxSize:     &maxSize2,
					},
					AmiType:      ekstypes.AMITypesAl2X8664,
					CapacityType: ekstypes.CapacityTypesSpot,
				},
			},
			"cluster-b/ng-api": {
				Nodegroup: &ekstypes.Nodegroup{
					NodegroupName: aws.String("ng-api"),
					ClusterName:   aws.String("cluster-b"),
					Status:        ekstypes.NodegroupStatusCreating,
					InstanceTypes: []string{"c5.large"},
					ScalingConfig: &ekstypes.NodegroupScalingConfig{
						DesiredSize: &desiredSize3,
						MinSize:     &minSize3,
						MaxSize:     &maxSize3,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	// Verify required fields exist on all resources
	requiredFields := []string{"nodegroup_name", "cluster_name", "status", "instance_types", "desired_size"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first node group (ng-web from cluster-a)
	r0 := resources[0]
	if r0.ID != "ng-web" {
		t.Errorf("resource[0].ID: expected %q, got %q", "ng-web", r0.ID)
	}
	if r0.Name != "ng-web" {
		t.Errorf("resource[0].Name: expected %q, got %q", "ng-web", r0.Name)
	}
	if r0.Status != "ACTIVE" {
		t.Errorf("resource[0].Status: expected %q, got %q", "ACTIVE", r0.Status)
	}
	if r0.Fields["nodegroup_name"] != "ng-web" {
		t.Errorf("resource[0].Fields[\"nodegroup_name\"]: expected %q, got %q", "ng-web", r0.Fields["nodegroup_name"])
	}
	if r0.Fields["cluster_name"] != "cluster-a" {
		t.Errorf("resource[0].Fields[\"cluster_name\"]: expected %q, got %q", "cluster-a", r0.Fields["cluster_name"])
	}
	if r0.Fields["status"] != "ACTIVE" {
		t.Errorf("resource[0].Fields[\"status\"]: expected %q, got %q", "ACTIVE", r0.Fields["status"])
	}
	if r0.Fields["instance_types"] != "t3.medium" {
		t.Errorf("resource[0].Fields[\"instance_types\"]: expected %q, got %q", "t3.medium", r0.Fields["instance_types"])
	}
	if r0.Fields["desired_size"] != "3" {
		t.Errorf("resource[0].Fields[\"desired_size\"]: expected %q, got %q", "3", r0.Fields["desired_size"])
	}

	// Verify second node group (ng-worker from cluster-a) - multiple instance types
	r1 := resources[1]
	if r1.Fields["nodegroup_name"] != "ng-worker" {
		t.Errorf("resource[1].Fields[\"nodegroup_name\"]: expected %q, got %q", "ng-worker", r1.Fields["nodegroup_name"])
	}
	if r1.Fields["cluster_name"] != "cluster-a" {
		t.Errorf("resource[1].Fields[\"cluster_name\"]: expected %q, got %q", "cluster-a", r1.Fields["cluster_name"])
	}
	if r1.Fields["instance_types"] != "m5.large, m5.xlarge" {
		t.Errorf("resource[1].Fields[\"instance_types\"]: expected %q, got %q", "m5.large, m5.xlarge", r1.Fields["instance_types"])
	}
	if r1.Fields["desired_size"] != "5" {
		t.Errorf("resource[1].Fields[\"desired_size\"]: expected %q, got %q", "5", r1.Fields["desired_size"])
	}

	// Verify third node group (ng-api from cluster-b) - different cluster, creating status
	r2 := resources[2]
	if r2.Fields["nodegroup_name"] != "ng-api" {
		t.Errorf("resource[2].Fields[\"nodegroup_name\"]: expected %q, got %q", "ng-api", r2.Fields["nodegroup_name"])
	}
	if r2.Fields["cluster_name"] != "cluster-b" {
		t.Errorf("resource[2].Fields[\"cluster_name\"]: expected %q, got %q", "cluster-b", r2.Fields["cluster_name"])
	}
	if r2.Status != "CREATING" {
		t.Errorf("resource[2].Status: expected %q, got %q", "CREATING", r2.Status)
	}
}

func TestFetchNodeGroups_DetailDataPopulated(t *testing.T) {
	listClustersMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{"prod-cluster"},
		},
	}

	listNGMock := &mockEKSListNodegroupsClient{
		outputs: map[string]*eks.ListNodegroupsOutput{
			"prod-cluster": {Nodegroups: []string{"ng-main"}},
		},
	}

	desiredSize := int32(4)
	minSize := int32(2)
	maxSize := int32(10)
	diskSize := int32(50)

	describeNGMock := &mockEKSDescribeNodegroupClient{
		outputs: map[string]*eks.DescribeNodegroupOutput{
			"prod-cluster/ng-main": {
				Nodegroup: &ekstypes.Nodegroup{
					NodegroupName: aws.String("ng-main"),
					ClusterName:   aws.String("prod-cluster"),
					Status:        ekstypes.NodegroupStatusActive,
					InstanceTypes: []string{"m5.large"},
					ScalingConfig: &ekstypes.NodegroupScalingConfig{
						DesiredSize: &desiredSize,
						MinSize:     &minSize,
						MaxSize:     &maxSize,
					},
					AmiType:        ekstypes.AMITypesAl2X8664,
					CapacityType:   ekstypes.CapacityTypesOnDemand,
					DiskSize:       &diskSize,
					NodeRole:       aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
					NodegroupArn:   aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/prod-cluster/ng-main/abc123"),
					ReleaseVersion: aws.String("1.28.3-20231106"),
					Version:        aws.String("1.28"),
					Subnets:        []string{"subnet-aaa", "subnet-bbb"},
					Labels:         map[string]string{"env": "production", "team": "platform"},
					Tags:           map[string]string{"Name": "ng-main", "Environment": "prod"},
				},
			},
		},
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// Verify DetailData keys
	expectedDetailKeys := []string{
		"Node Group Name",
		"Cluster Name",
		"Status",
		"Instance Types",
		"AMI Type",
		"Capacity Type",
		"Disk Size",
		"Desired Size",
		"Min Size",
		"Max Size",
		"Node Role",
		"Node Group ARN",
		"Release Version",
		"Kubernetes Version",
		"Subnets",
	}

	for _, key := range expectedDetailKeys {
		if _, ok := r.DetailData[key]; !ok {
			t.Errorf("DetailData missing key %q", key)
		}
	}

	// Verify specific detail values
	if r.DetailData["Node Group Name"] != "ng-main" {
		t.Errorf("DetailData[\"Node Group Name\"]: expected %q, got %q", "ng-main", r.DetailData["Node Group Name"])
	}
	if r.DetailData["Cluster Name"] != "prod-cluster" {
		t.Errorf("DetailData[\"Cluster Name\"]: expected %q, got %q", "prod-cluster", r.DetailData["Cluster Name"])
	}
	if r.DetailData["Status"] != "ACTIVE" {
		t.Errorf("DetailData[\"Status\"]: expected %q, got %q", "ACTIVE", r.DetailData["Status"])
	}
	if r.DetailData["AMI Type"] != "AL2_x86_64" {
		t.Errorf("DetailData[\"AMI Type\"]: expected %q, got %q", "AL2_x86_64", r.DetailData["AMI Type"])
	}
	if r.DetailData["Capacity Type"] != "ON_DEMAND" {
		t.Errorf("DetailData[\"Capacity Type\"]: expected %q, got %q", "ON_DEMAND", r.DetailData["Capacity Type"])
	}
	if r.DetailData["Disk Size"] != "50" {
		t.Errorf("DetailData[\"Disk Size\"]: expected %q, got %q", "50", r.DetailData["Disk Size"])
	}
	if r.DetailData["Desired Size"] != "4" {
		t.Errorf("DetailData[\"Desired Size\"]: expected %q, got %q", "4", r.DetailData["Desired Size"])
	}
	if r.DetailData["Min Size"] != "2" {
		t.Errorf("DetailData[\"Min Size\"]: expected %q, got %q", "2", r.DetailData["Min Size"])
	}
	if r.DetailData["Max Size"] != "10" {
		t.Errorf("DetailData[\"Max Size\"]: expected %q, got %q", "10", r.DetailData["Max Size"])
	}
	if r.DetailData["Node Role"] != "arn:aws:iam::123456789012:role/eks-node-role" {
		t.Errorf("DetailData[\"Node Role\"]: expected %q, got %q", "arn:aws:iam::123456789012:role/eks-node-role", r.DetailData["Node Role"])
	}
	if r.DetailData["Subnets"] != "subnet-aaa, subnet-bbb" {
		t.Errorf("DetailData[\"Subnets\"]: expected %q, got %q", "subnet-aaa, subnet-bbb", r.DetailData["Subnets"])
	}
	if r.DetailData["Kubernetes Version"] != "1.28" {
		t.Errorf("DetailData[\"Kubernetes Version\"]: expected %q, got %q", "1.28", r.DetailData["Kubernetes Version"])
	}
	if r.DetailData["Release Version"] != "1.28.3-20231106" {
		t.Errorf("DetailData[\"Release Version\"]: expected %q, got %q", "1.28.3-20231106", r.DetailData["Release Version"])
	}

	// Verify labels in DetailData
	if r.DetailData["Label: env"] != "production" {
		t.Errorf("DetailData[\"Label: env\"]: expected %q, got %q", "production", r.DetailData["Label: env"])
	}
	if r.DetailData["Label: team"] != "platform" {
		t.Errorf("DetailData[\"Label: team\"]: expected %q, got %q", "platform", r.DetailData["Label: team"])
	}

	// Verify tags in DetailData
	if r.DetailData["Tag: Name"] != "ng-main" {
		t.Errorf("DetailData[\"Tag: Name\"]: expected %q, got %q", "ng-main", r.DetailData["Tag: Name"])
	}
	if r.DetailData["Tag: Environment"] != "prod" {
		t.Errorf("DetailData[\"Tag: Environment\"]: expected %q, got %q", "prod", r.DetailData["Tag: Environment"])
	}
}

func TestFetchNodeGroups_ListClustersError(t *testing.T) {
	listClustersMock := &mockEKSListClustersClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}
	listNGMock := &mockEKSListNodegroupsClient{}
	describeNGMock := &mockEKSDescribeNodegroupClient{}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchNodeGroups_ListNodegroupsError(t *testing.T) {
	listClustersMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{"cluster-a"},
		},
	}
	listNGMock := &mockEKSListNodegroupsClient{
		err: fmt.Errorf("AWS API error: list nodegroups failed"),
	}
	describeNGMock := &mockEKSDescribeNodegroupClient{}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchNodeGroups_DescribeNodegroupError(t *testing.T) {
	listClustersMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{"cluster-a"},
		},
	}
	listNGMock := &mockEKSListNodegroupsClient{
		outputs: map[string]*eks.ListNodegroupsOutput{
			"cluster-a": {Nodegroups: []string{"ng-web"}},
		},
	}
	describeNGMock := &mockEKSDescribeNodegroupClient{
		err: fmt.Errorf("AWS API error: describe nodegroup failed"),
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchNodeGroups_EmptyClusters(t *testing.T) {
	listClustersMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{},
		},
	}
	listNGMock := &mockEKSListNodegroupsClient{}
	describeNGMock := &mockEKSDescribeNodegroupClient{}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchNodeGroups_ClustersButNoNodeGroups(t *testing.T) {
	listClustersMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{"cluster-a", "cluster-b"},
		},
	}
	listNGMock := &mockEKSListNodegroupsClient{
		outputs: map[string]*eks.ListNodegroupsOutput{
			"cluster-a": {Nodegroups: []string{}},
			"cluster-b": {Nodegroups: []string{}},
		},
	}
	describeNGMock := &mockEKSDescribeNodegroupClient{}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchNodeGroups_RawStructPopulated(t *testing.T) {
	listClustersMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{"cluster-x"},
		},
	}

	listNGMock := &mockEKSListNodegroupsClient{
		outputs: map[string]*eks.ListNodegroupsOutput{
			"cluster-x": {Nodegroups: []string{"ng-test"}},
		},
	}

	desiredSize := int32(1)
	minSize := int32(1)
	maxSize := int32(3)

	describeNGMock := &mockEKSDescribeNodegroupClient{
		outputs: map[string]*eks.DescribeNodegroupOutput{
			"cluster-x/ng-test": {
				Nodegroup: &ekstypes.Nodegroup{
					NodegroupName: aws.String("ng-test"),
					ClusterName:   aws.String("cluster-x"),
					Status:        ekstypes.NodegroupStatusActive,
					InstanceTypes: []string{"t3.small"},
					ScalingConfig: &ekstypes.NodegroupScalingConfig{
						DesiredSize: &desiredSize,
						MinSize:     &minSize,
						MaxSize:     &maxSize,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// Verify RawStruct is set and is the correct type
	if r.RawStruct == nil {
		t.Fatal("expected RawStruct to be non-nil")
	}

	ng, ok := r.RawStruct.(*ekstypes.Nodegroup)
	if !ok {
		t.Fatalf("expected RawStruct to be *ekstypes.Nodegroup, got %T", r.RawStruct)
	}

	if ng.NodegroupName == nil || *ng.NodegroupName != "ng-test" {
		t.Errorf("RawStruct.NodegroupName: expected %q, got %v", "ng-test", ng.NodegroupName)
	}

	// Verify RawJSON is populated
	if r.RawJSON == "" {
		t.Error("expected RawJSON to be non-empty")
	}
}

func TestFetchNodeGroups_NilScalingConfig(t *testing.T) {
	listClustersMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{"cluster-z"},
		},
	}

	listNGMock := &mockEKSListNodegroupsClient{
		outputs: map[string]*eks.ListNodegroupsOutput{
			"cluster-z": {Nodegroups: []string{"ng-noscale"}},
		},
	}

	describeNGMock := &mockEKSDescribeNodegroupClient{
		outputs: map[string]*eks.DescribeNodegroupOutput{
			"cluster-z/ng-noscale": {
				Nodegroup: &ekstypes.Nodegroup{
					NodegroupName: aws.String("ng-noscale"),
					ClusterName:   aws.String("cluster-z"),
					Status:        ekstypes.NodegroupStatusActive,
					InstanceTypes: []string{"t3.micro"},
					ScalingConfig: nil, // nil ScalingConfig
				},
			},
		},
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// When ScalingConfig is nil, desired_size should be empty
	if r.Fields["desired_size"] != "" {
		t.Errorf("expected empty desired_size with nil ScalingConfig, got %q", r.Fields["desired_size"])
	}
}

// ---------------------------------------------------------------------------
// T-NG-REAL - Test node groups fetcher with sanitized fixture data
// (3 node groups from test-cluster-1: gpu CREATE_FAILED, kafka ACTIVE, system ACTIVE)
// ---------------------------------------------------------------------------

func TestFetchNodeGroups_RealAWSData(t *testing.T) {
	realNGs := testdata.RealNodeGroups()

	// Build the three-step mock using sanitized data
	// All 3 node groups belong to the single cluster "test-cluster-1"
	clusterName := "test-cluster-1"

	listClustersMock := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{
			Clusters: []string{clusterName},
		},
	}

	ngNames := make([]string, len(realNGs))
	for i, ng := range realNGs {
		ngNames[i] = *ng.NodegroupName
	}

	listNGMock := &mockEKSListNodegroupsClient{
		outputs: map[string]*eks.ListNodegroupsOutput{
			clusterName: {Nodegroups: ngNames},
		},
	}

	describeOutputs := make(map[string]*eks.DescribeNodegroupOutput)
	for i := range realNGs {
		ng := realNGs[i]
		key := clusterName + "/" + *ng.NodegroupName
		describeOutputs[key] = &eks.DescribeNodegroupOutput{
			Nodegroup: &ng,
		}
	}

	describeNGMock := &mockEKSDescribeNodegroupClient{
		outputs: describeOutputs,
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClustersMock, listNGMock, describeNGMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Sanitized data has exactly 3 node groups
	if len(resources) != 3 {
		t.Fatalf("expected 3 resources from real data, got %d", len(resources))
	}

	// Build lookup by resource ID (nodegroup name)
	byID := make(map[string]int)
	for i, r := range resources {
		byID[r.ID] = i
	}

	// --- Node Group 1: GPU (CREATE_FAILED) ---
	gpuIdx, ok := byID["gpu-20250101120000000000000001"]
	if !ok {
		t.Fatal("missing gpu node group in results")
	}
	gpu := resources[gpuIdx]
	if gpu.Status != "CREATE_FAILED" {
		t.Errorf("gpu node group Status: expected %q, got %q", "CREATE_FAILED", gpu.Status)
	}
	if gpu.Fields["cluster_name"] != "test-cluster-1" {
		t.Errorf("gpu node group cluster_name: expected %q, got %q", "test-cluster-1", gpu.Fields["cluster_name"])
	}
	if gpu.Fields["instance_types"] != "g4dn.xlarge" {
		t.Errorf("gpu node group instance_types: expected %q, got %q", "g4dn.xlarge", gpu.Fields["instance_types"])
	}
	if gpu.Fields["desired_size"] != "2" {
		t.Errorf("gpu node group desired_size: expected %q, got %q", "2", gpu.Fields["desired_size"])
	}

	// DetailData checks for GPU node group
	if gpu.DetailData["AMI Type"] != "AL2_x86_64_GPU" {
		t.Errorf("gpu DetailData[AMI Type]: expected %q, got %q", "AL2_x86_64_GPU", gpu.DetailData["AMI Type"])
	}
	if gpu.DetailData["Capacity Type"] != "ON_DEMAND" {
		t.Errorf("gpu DetailData[Capacity Type]: expected %q, got %q", "ON_DEMAND", gpu.DetailData["Capacity Type"])
	}
	if gpu.DetailData["Min Size"] != "1" {
		t.Errorf("gpu DetailData[Min Size]: expected %q, got %q", "1", gpu.DetailData["Min Size"])
	}
	if gpu.DetailData["Max Size"] != "3" {
		t.Errorf("gpu DetailData[Max Size]: expected %q, got %q", "3", gpu.DetailData["Max Size"])
	}
	if gpu.DetailData["Kubernetes Version"] != "1.31" {
		t.Errorf("gpu DetailData[Kubernetes Version]: expected %q, got %q", "1.31", gpu.DetailData["Kubernetes Version"])
	}
	if gpu.DetailData["Release Version"] != "1.31.7-20250519" {
		t.Errorf("gpu DetailData[Release Version]: expected %q, got %q", "1.31.7-20250519", gpu.DetailData["Release Version"])
	}
	if gpu.DetailData["Node Group ARN"] != "arn:aws:eks:us-east-1:123456789012:nodegroup/test-cluster-1/gpu-20250101120000000000000001/78cb8e0e-6400-fea1-9939-803bc27e4134" {
		t.Errorf("gpu DetailData[Node Group ARN] mismatch: got %q", gpu.DetailData["Node Group ARN"])
	}
	if gpu.DetailData["Label: group"] != "gpu" {
		t.Errorf("gpu DetailData[Label: group]: expected %q, got %q", "gpu", gpu.DetailData["Label: group"])
	}
	if gpu.DetailData["Tag: Name"] != "gpu" {
		t.Errorf("gpu DetailData[Tag: Name]: expected %q, got %q", "gpu", gpu.DetailData["Tag: Name"])
	}
	// DiskSize should be empty (uses launch template)
	if gpu.DetailData["Disk Size"] != "" {
		t.Errorf("gpu DetailData[Disk Size]: expected empty (launch template), got %q", gpu.DetailData["Disk Size"])
	}
	// Subnets
	expectedSubnets := "subnet-0aaa111111111111a, subnet-0bbb222222222222b, subnet-0ccc333333333333c"
	if gpu.DetailData["Subnets"] != expectedSubnets {
		t.Errorf("gpu DetailData[Subnets]: expected %q, got %q", expectedSubnets, gpu.DetailData["Subnets"])
	}
	// Created At should be populated
	if gpu.DetailData["Created At"] == "" {
		t.Error("gpu DetailData[Created At] must not be empty")
	}
	// Verify RawStruct contains health issues (real CREATE_FAILED data)
	gpuRaw, ok := gpu.RawStruct.(*ekstypes.Nodegroup)
	if !ok {
		t.Fatalf("gpu RawStruct should be *ekstypes.Nodegroup, got %T", gpu.RawStruct)
	}
	if gpuRaw.Health == nil || len(gpuRaw.Health.Issues) != 1 {
		t.Fatalf("gpu RawStruct should have 1 health issue, got %v", gpuRaw.Health)
	}
	if gpuRaw.Health.Issues[0].Code != ekstypes.NodegroupIssueCodeAsgInstanceLaunchFailures {
		t.Errorf("gpu health issue code: expected AsgInstanceLaunchFailures, got %v", gpuRaw.Health.Issues[0].Code)
	}
	if gpuRaw.Health.Issues[0].Message == nil || !strings.Contains(*gpuRaw.Health.Issues[0].Message, "VcpuLimitExceeded") {
		t.Errorf("gpu health issue message should contain VcpuLimitExceeded")
	}

	// --- Node Group 2: Kafka (ACTIVE, fixed 3/3/3 scaling, NO_SCHEDULE taint) ---
	kafkaIdx, ok := byID["kafka-20250101120000000000000002"]
	if !ok {
		t.Fatal("missing kafka node group in results")
	}
	kafka := resources[kafkaIdx]
	if kafka.Status != "ACTIVE" {
		t.Errorf("kafka node group Status: expected %q, got %q", "ACTIVE", kafka.Status)
	}
	if kafka.Fields["instance_types"] != "t3.large" {
		t.Errorf("kafka node group instance_types: expected %q, got %q", "t3.large", kafka.Fields["instance_types"])
	}
	if kafka.Fields["desired_size"] != "3" {
		t.Errorf("kafka node group desired_size: expected %q, got %q", "3", kafka.Fields["desired_size"])
	}
	// Fixed-size cluster: min=max=desired=3
	if kafka.DetailData["Min Size"] != "3" {
		t.Errorf("kafka DetailData[Min Size]: expected %q, got %q", "3", kafka.DetailData["Min Size"])
	}
	if kafka.DetailData["Max Size"] != "3" {
		t.Errorf("kafka DetailData[Max Size]: expected %q, got %q", "3", kafka.DetailData["Max Size"])
	}
	if kafka.DetailData["Label: group"] != "kafka" {
		t.Errorf("kafka DetailData[Label: group]: expected %q, got %q", "kafka", kafka.DetailData["Label: group"])
	}
	if kafka.DetailData["Tag: Name"] != "kafka" {
		t.Errorf("kafka DetailData[Tag: Name]: expected %q, got %q", "kafka", kafka.DetailData["Tag: Name"])
	}
	// Verify taint is preserved in RawStruct
	kafkaRaw, ok := kafka.RawStruct.(*ekstypes.Nodegroup)
	if !ok {
		t.Fatalf("kafka RawStruct should be *ekstypes.Nodegroup, got %T", kafka.RawStruct)
	}
	if len(kafkaRaw.Taints) != 1 {
		t.Fatalf("kafka RawStruct should have 1 taint, got %d", len(kafkaRaw.Taints))
	}
	if kafkaRaw.Taints[0].Key == nil || *kafkaRaw.Taints[0].Key != "kafka" {
		t.Errorf("kafka taint key: expected %q, got %v", "kafka", kafkaRaw.Taints[0].Key)
	}
	if kafkaRaw.Taints[0].Effect != ekstypes.TaintEffectNoSchedule {
		t.Errorf("kafka taint effect: expected NO_SCHEDULE, got %v", kafkaRaw.Taints[0].Effect)
	}
	// Kafka health should be clean (empty issues)
	if kafkaRaw.Health == nil || len(kafkaRaw.Health.Issues) != 0 {
		t.Errorf("kafka health should have 0 issues, got %v", kafkaRaw.Health)
	}

	// --- Node Group 3: system (ACTIVE, 2-3x t3.large, karpenter label) ---
	systemIdx, ok := byID["system-20250101120000000000000003"]
	if !ok {
		t.Fatal("missing system node group in results")
	}
	system := resources[systemIdx]
	if system.Status != "ACTIVE" {
		t.Errorf("system node group Status: expected %q, got %q", "ACTIVE", system.Status)
	}
	if system.Fields["instance_types"] != "t3.large" {
		t.Errorf("system instance_types: expected %q, got %q", "t3.large", system.Fields["instance_types"])
	}
	if system.Fields["desired_size"] != "2" {
		t.Errorf("system desired_size: expected %q, got %q", "2", system.Fields["desired_size"])
	}
	if system.DetailData["Min Size"] != "2" {
		t.Errorf("system DetailData[Min Size]: expected %q, got %q", "2", system.DetailData["Min Size"])
	}
	if system.DetailData["Max Size"] != "3" {
		t.Errorf("system DetailData[Max Size]: expected %q, got %q", "3", system.DetailData["Max Size"])
	}
	if system.DetailData["AMI Type"] != "AL2023_x86_64_STANDARD" {
		t.Errorf("system DetailData[AMI Type]: expected %q, got %q", "AL2023_x86_64_STANDARD", system.DetailData["AMI Type"])
	}
	if system.DetailData["Label: karpenter.sh/controller"] != "true" {
		t.Errorf("system missing karpenter label, got %q", system.DetailData["Label: karpenter.sh/controller"])
	}
	if system.DetailData["Label: group"] != "system" {
		t.Errorf("system DetailData[Label: group]: expected %q, got %q", "system", system.DetailData["Label: group"])
	}
	if system.DetailData["Tag: Name"] != "system" {
		t.Errorf("system DetailData[Tag: Name]: expected %q, got %q", "system", system.DetailData["Tag: Name"])
	}
	// system has no taints
	systemRaw, ok := system.RawStruct.(*ekstypes.Nodegroup)
	if !ok {
		t.Fatalf("system RawStruct should be *ekstypes.Nodegroup, got %T", system.RawStruct)
	}
	if len(systemRaw.Taints) != 0 {
		t.Errorf("system should have 0 taints, got %d", len(systemRaw.Taints))
	}

	// --- Cross-cutting assertions for all 3 node groups ---
	for i, r := range resources {
		// All belong to the same cluster
		if r.Fields["cluster_name"] != "test-cluster-1" {
			t.Errorf("resource[%d].Fields[cluster_name]: expected %q, got %q", i, "test-cluster-1", r.Fields["cluster_name"])
		}
		// ID should equal Name (nodegroup name used for both)
		if r.ID != r.Name {
			t.Errorf("resource[%d]: ID (%q) should equal Name (%q)", i, r.ID, r.Name)
		}
		// Required fields present
		requiredFields := []string{"nodegroup_name", "cluster_name", "status", "instance_types", "desired_size"}
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
		// RawJSON must be populated
		if r.RawJSON == "" {
			t.Errorf("resource[%d].RawJSON must not be empty", i)
		}
		// RawStruct must be non-nil
		if r.RawStruct == nil {
			t.Errorf("resource[%d].RawStruct must not be nil", i)
		}
		// All share same Kubernetes version 1.31
		if r.DetailData["Kubernetes Version"] != "1.31" {
			t.Errorf("resource[%d].DetailData[Kubernetes Version]: expected %q, got %q", i, "1.31", r.DetailData["Kubernetes Version"])
		}
		// All share same release version
		if r.DetailData["Release Version"] != "1.31.7-20250519" {
			t.Errorf("resource[%d].DetailData[Release Version]: expected %q, got %q", i, "1.31.7-20250519", r.DetailData["Release Version"])
		}
		// All are ON_DEMAND capacity type
		if r.DetailData["Capacity Type"] != "ON_DEMAND" {
			t.Errorf("resource[%d].DetailData[Capacity Type]: expected %q, got %q", i, "ON_DEMAND", r.DetailData["Capacity Type"])
		}
		// All share the same 3 subnets
		if r.DetailData["Subnets"] != expectedSubnets {
			t.Errorf("resource[%d].DetailData[Subnets]: expected %q, got %q", i, expectedSubnets, r.DetailData["Subnets"])
		}
		// All have Tag: name = test-cluster-1
		if r.DetailData["Tag: name"] != "test-cluster-1" {
			t.Errorf("resource[%d].DetailData[Tag: name]: expected %q, got %q", i, "test-cluster-1", r.DetailData["Tag: name"])
		}
	}
}
