package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func eksCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("eks") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("eks related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("eks related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_EKS(t *testing.T) {
	expected := map[string]string{
		"ResourcesVpcConfig.VpcId":                 "vpc",
		"ResourcesVpcConfig.ClusterSecurityGroupId": "sg",
	}
	for path, wantTarget := range expected {
		nav := resource.IsFieldNavigable("eks", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found for eks", path)
			continue
		}
		if nav.TargetType != wantTarget {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, wantTarget)
		}
	}
}

// --- Node Groups checker (Pattern C — cache, ClusterName match) ---

func TestRelated_EKS_NodeGroups_Found(t *testing.T) {
	ngRes1 := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("acme-services"),
		},
	}
	ngRes2 := resource.Resource{
		ID:   "gpu-pool",
		Name: "gpu-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("gpu-pool"),
			ClusterName:   aws.String("acme-services"),
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes1, ngRes2}},
	}
	source := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("acme-services"),
		},
	}

	checker := eksCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs len = %d, want 2; got %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EKS_NodeGroups_NoMatch(t *testing.T) {
	ngRes := resource.Resource{
		ID:   "other-pool",
		Name: "other-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("other-pool"),
			ClusterName:   aws.String("different-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}
	source := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("acme-services"),
		},
	}

	checker := eksCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_EKS_NodeGroups_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("acme-services"),
		},
	}

	checker := eksCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_EKS_NodeGroups_EmptyClusterName(t *testing.T) {
	ngRes := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("acme-services"),
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}
	// Cluster with empty name — should not match any node group.
	source := resource.Resource{
		ID:   "",
		Name: "",
		RawStruct: &ekstypes.Cluster{
			Name: nil,
		},
	}

	checker := eksCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty cluster name", result.Count)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, ClusterName dimension) ---

func TestRelated_EKS_Alarms_MatchClusterName(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "eks-cpu-high",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("eks-cpu-high"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ClusterName"), Value: aws.String("acme-services")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("acme-services"),
		},
	}

	checker := eksCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "eks-cpu-high" {
		t.Errorf("ResourceIDs = %v, want [eks-cpu-high]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EKS_Alarms_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "other-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ClusterName"), Value: aws.String("different-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("acme-services"),
		},
	}

	checker := eksCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_EKS_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("acme-services"),
		},
	}

	checker := eksCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- CloudFormation checker (Pattern C — cache, aws:cloudformation:stack-name tag) ---
// Note: ekstypes.Cluster has Tags map[string]string (not a slice of Tag structs).

func TestRelated_EKS_CFN_FromTags(t *testing.T) {
	cfnRes := resource.Resource{
		ID:   "eks-cluster-stack",
		Name: "eks-cluster-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("eks-cluster-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("acme-services"),
			Tags: map[string]string{
				"aws:cloudformation:stack-name": "eks-cluster-stack",
			},
		},
	}

	checker := eksCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "eks-cluster-stack" {
		t.Errorf("ResourceIDs = %v, want [eks-cluster-stack]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EKS_CFN_NoTag(t *testing.T) {
	cfnRes := resource.Resource{
		ID:   "some-stack",
		Name: "some-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("some-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	// Cluster has no CFN tag — not created by CloudFormation.
	source := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("acme-services"),
			Tags: map[string]string{
				"Environment": "prod",
			},
		},
	}

	checker := eksCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for cluster with no CFN tag", result.Count)
	}
}

func TestRelated_EKS_CFN_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("acme-services"),
			Tags: map[string]string{
				"aws:cloudformation:stack-name": "eks-cluster-stack",
			},
		},
	}

	checker := eksCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- AMI checker tests (Pattern A — EKS.ListNodegroups + EKS.DescribeNodegroup + EC2.DescribeLaunchTemplateVersions) ---

func eksClusterSrcResource() resource.Resource {
	return resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		Fields: map[string]string{},
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("acme-services"),
		},
	}
}

// TestRelated_EKS_AMI_Match verifies that two distinct AMI IDs resolved via
// node group launch templates produce Count=2 with both IDs in ResourceIDs.
func TestRelated_EKS_AMI_Match(t *testing.T) {
	const ltID = "lt-aaa111"
	const ami1 = "ami-0a1b2c3d4e5f60001"
	const ami2 = "ami-0a1b2c3d4e5f60002"

	eksNodegroups := map[string]*ekstypes.Nodegroup{
		"ng-general": {
			NodegroupName: aws.String("ng-general"),
			ClusterName:   aws.String("acme-services"),
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
				Id:      aws.String(ltID),
				Version: aws.String("1"),
			},
		},
		"ng-gpu": {
			NodegroupName: aws.String("ng-gpu"),
			ClusterName:   aws.String("acme-services"),
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
				Id:      aws.String(ltID),
				Version: aws.String("2"),
			},
		},
	}
	fakeEKS := newFakeEKSWithNodegroups([]string{"ng-general", "ng-gpu"}, eksNodegroups)

	// DescribeLaunchTemplateVersions returns a different AMI per call (matched by version).
	callCount := 0
	fakeEC2 := &fakeEC2Batch2{
		describeLaunchTemplateVersionsFn: func(input *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			callCount++
			amiID := ami1
			if len(input.Versions) > 0 && input.Versions[0] == "2" {
				amiID = ami2
			}
			return &ec2.DescribeLaunchTemplateVersionsOutput{
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
					{
						LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
							ImageId: aws.String(amiID),
						},
					},
				},
			}, nil
		},
	}
	clients := &awsclient.ServiceClients{EKS: fakeEKS, EC2: fakeEC2}
	res := eksClusterSrcResource()

	checker := eksCheckerByTarget(t, "ami")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 2 {
		t.Fatalf("Count = %d, want 2; ResourceIDs: %v", result.Count, result.ResourceIDs)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	for _, want := range []string{ami1, ami2} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
	if result.Err != nil {
		t.Errorf("unexpected Err: %v", result.Err)
	}
}

// TestRelated_EKS_AMI_Empty verifies that node groups without a launch template
// (managed NGs) produce Count=0 (SSM-based AMI resolution is deferred).
func TestRelated_EKS_AMI_Empty(t *testing.T) {
	eksNodegroups := map[string]*ekstypes.Nodegroup{
		"ng-managed": {
			NodegroupName:  aws.String("ng-managed"),
			ClusterName:    aws.String("acme-services"),
			LaunchTemplate: nil, // no custom LT — managed NG
		},
	}
	fakeEKS := newFakeEKSWithNodegroups([]string{"ng-managed"}, eksNodegroups)
	fakeEC2 := &fakeEC2Batch2{}
	clients := &awsclient.ServiceClients{EKS: fakeEKS, EC2: fakeEC2}
	res := eksClusterSrcResource()

	checker := eksCheckerByTarget(t, "ami")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (managed NG — no custom LT)", result.Count)
	}
}

// TestRelated_EKS_AMI_WrongRawStruct verifies that a wrong RawStruct type
// returns Count=-1 (defensive guard, assertStruct fails).
func TestRelated_EKS_AMI_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "acme-services",
		RawStruct: "not-a-cluster",
	}
	checker := eksCheckerByTarget(t, "ami")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// --- EC2 checker tests (Pattern A — EKS.ListNodegroups + EKS.DescribeNodegroup + ASG.DescribeAutoScalingGroups) ---

// TestRelated_EKS_EC2_Match verifies that instance IDs gathered via node group
// ASGs produce Count=N with all IDs in ResourceIDs.
func TestRelated_EKS_EC2_Match(t *testing.T) {
	const asgName = "eks-acme-services-ng-general-asg"
	const inst1 = "i-0a1b2c3d4e5f60001"
	const inst2 = "i-0a1b2c3d4e5f60002"

	eksNodegroups := map[string]*ekstypes.Nodegroup{
		"ng-general": {
			NodegroupName: aws.String("ng-general"),
			ClusterName:   aws.String("acme-services"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String(asgName)},
				},
			},
		},
	}
	fakeEKS := newFakeEKSWithNodegroups([]string{"ng-general"}, eksNodegroups)
	fakeASG := newFakeASGWithGroups([]asgtypes.AutoScalingGroup{
		{
			AutoScalingGroupName: aws.String(asgName),
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String(inst1)},
				{InstanceId: aws.String(inst2)},
			},
		},
	})
	clients := &awsclient.ServiceClients{EKS: fakeEKS, AutoScaling: fakeASG}
	res := eksClusterSrcResource()

	checker := eksCheckerByTarget(t, "ec2")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 2 {
		t.Fatalf("Count = %d, want 2; ResourceIDs: %v", result.Count, result.ResourceIDs)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	for _, want := range []string{inst1, inst2} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
	if result.Err != nil {
		t.Errorf("unexpected Err: %v", result.Err)
	}
}

// TestRelated_EKS_EC2_Empty verifies that node groups with no ASG instances
// produce Count=0.
func TestRelated_EKS_EC2_Empty(t *testing.T) {
	const asgName = "eks-acme-services-ng-general-asg"

	eksNodegroups := map[string]*ekstypes.Nodegroup{
		"ng-general": {
			NodegroupName: aws.String("ng-general"),
			ClusterName:   aws.String("acme-services"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String(asgName)},
				},
			},
		},
	}
	fakeEKS := newFakeEKSWithNodegroups([]string{"ng-general"}, eksNodegroups)
	fakeASG := newFakeASGWithGroups([]asgtypes.AutoScalingGroup{
		{
			AutoScalingGroupName: aws.String(asgName),
			Instances:            []asgtypes.Instance{}, // no instances yet (scaling down)
		},
	})
	clients := &awsclient.ServiceClients{EKS: fakeEKS, AutoScaling: fakeASG}
	res := eksClusterSrcResource()

	checker := eksCheckerByTarget(t, "ec2")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no instances)", result.Count)
	}
}

// TestRelated_EKS_EC2_WrongRawStruct verifies that a wrong RawStruct type
// returns Count=-1 (defensive guard, assertStruct fails).
func TestRelated_EKS_EC2_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "acme-services",
		RawStruct: "not-a-cluster",
	}
	checker := eksCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}
