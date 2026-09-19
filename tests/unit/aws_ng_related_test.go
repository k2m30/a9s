package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func ngCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ng") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ng related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ng related checker for %s not found", target)
	return nil
}

func TestNavigableFields_NG_Registered(t *testing.T) {
	expected := map[string]string{
		"ClusterName": "eks",
		"NodeRole":    "role",
	}
	for path, wantTarget := range expected {
		nav := resource.IsFieldNavigableForTest("ng", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found for ng", path)
			continue
		}
		if nav.TargetType != wantTarget {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, wantTarget)
		}
	}
}

func TestRelated_NG_EKS_Found(t *testing.T) {
	eksRes := resource.Resource{
		ID:   "my-cluster",
		Name: "my-cluster",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("my-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"eks": resource.ResourceCacheEntry{Resources: []resource.Resource{eksRes}},
	}
	source := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		Fields: map[string]string{
			"cluster_name": "my-cluster",
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("my-cluster"),
			NodeRole:      aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
		},
	}

	checker := ngCheckerByTarget(t, "eks")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-cluster" {
		t.Errorf("ResourceIDs = %v, want [my-cluster]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_NG_EKS_NotFound(t *testing.T) {
	eksRes := resource.Resource{
		ID:   "different-cluster",
		Name: "different-cluster",
		RawStruct: &ekstypes.Cluster{
			Name: aws.String("different-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"eks": resource.ResourceCacheEntry{Resources: []resource.Resource{eksRes}},
	}
	source := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		Fields: map[string]string{
			"cluster_name": "my-cluster",
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("my-cluster"),
		},
	}

	checker := ngCheckerByTarget(t, "eks")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_NG_EKS_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		Fields: map[string]string{
			"cluster_name": "my-cluster",
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("my-cluster"),
		},
	}

	checker := ngCheckerByTarget(t, "eks")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count())
	}
}

func TestRelated_NG_Role_Found(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/eks-node-role"
	const roleName = "eks-node-role"

	roleRes := resource.Resource{
		ID:   roleName,
		Name: roleName,
		RawStruct: iamtypes.Role{
			RoleName: aws.String(roleName),
			Arn:      aws.String(roleARN),
		},
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}
	source := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		Fields: map[string]string{
			"node_role": roleARN,
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("my-cluster"),
			NodeRole:      aws.String(roleARN),
		},
	}

	checker := ngCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != roleName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), roleName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_NG_Role_NotFound(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/eks-node-role"

	roleRes := resource.Resource{
		ID:   "DifferentRole",
		Name: "DifferentRole",
	}
	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{Resources: []resource.Resource{roleRes}},
	}
	source := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		Fields: map[string]string{
			"node_role": roleARN,
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("my-cluster"),
			NodeRole:      aws.String(roleARN),
		},
	}

	checker := ngCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (event-derived: role reference resolves by identity)", result.Count())
	}
}

func TestRelated_NG_Role_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		Fields: map[string]string{
			"node_role": "arn:aws:iam::123456789012:role/eks-node-role",
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("my-cluster"),
			NodeRole:      aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
		},
	}

	checker := ngCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count())
	}
}

func TestRelated_NG_ASG_Found(t *testing.T) {
	const asgName = "eks-acme-prod-ng-general"

	asgRes := resource.Resource{
		ID:   asgName,
		Name: asgName,
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String(asgName),
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}
	source := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		Fields: map[string]string{
			"cluster_name": "acme-prod",
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("acme-prod"),
			NodeRole:      aws.String("arn:aws:iam::123456789012:role/eks-node-role"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String(asgName)},
				},
			},
		},
	}

	checker := ngCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != asgName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), asgName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_NG_ASG_NotFound(t *testing.T) {
	asgRes := resource.Resource{
		ID:   "different-asg",
		Name: "different-asg",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("different-asg"),
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}
	source := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("acme-prod"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("eks-acme-prod-ng-general")},
				},
			},
		},
	}

	checker := ngCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_NG_ASG_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("acme-prod"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("eks-acme-prod-ng-general")},
				},
			},
		},
	}

	checker := ngCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count())
	}
}

func ngSrcResourceWithLaunchTemplate(ltID, ltVersion string) resource.Resource {
	return resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		Fields: map[string]string{
			"cluster_name": "acme-prod",
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("acme-prod"),
			LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
				Id:      aws.String(ltID),
				Version: aws.String(ltVersion),
			},
		},
	}
}

func TestRelated_NG_AMI_Match(t *testing.T) {
	const ltID = "lt-abc12345"
	const amiID = "ami-0a1b2c3d4e5f60001"

	fakeEC2 := newFakeEC2WithLaunchTemplateVersions([]ec2types.LaunchTemplateVersion{
		{
			LaunchTemplateId: aws.String(ltID),
			LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
				ImageId: aws.String(amiID),
			},
		},
	})
	clients := &awsclient.ServiceClients{EC2: fakeEC2}
	res := ngSrcResourceWithLaunchTemplate(ltID, "1")

	checker := ngCheckerByTarget(t, "ami")
	result := checker(context.Background(), clients, res, nil)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != amiID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), amiID)
	}
	if result.Err() != nil {
		t.Errorf("unexpected Err: %v", result.Err())
	}
}

func TestRelated_NG_AMI_Empty(t *testing.T) {
	res := resource.Resource{
		ID:     "general-pool",
		Name:   "general-pool",
		Fields: map[string]string{},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName:  aws.String("general-pool"),
			ClusterName:    aws.String("acme-prod"),
			LaunchTemplate: nil, // managed NG — no custom LT
		},
	}
	fakeEC2 := &fakeEC2ForASG{}
	clients := &awsclient.ServiceClients{EC2: fakeEC2}

	checker := ngCheckerByTarget(t, "ami")
	result := checker(context.Background(), clients, res, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (managed NG, no launch template)", result.Count())
	}
}

func TestRelated_NG_AMI_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "general-pool",
		RawStruct: "not-a-nodegroup",
	}
	checker := ngCheckerByTarget(t, "ami")
	result := checker(context.Background(), nil, res, nil)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

func TestRelated_NG_EBS_Empty(t *testing.T) {
	res := resource.Resource{
		ID:     "general-pool",
		Name:   "general-pool",
		Fields: map[string]string{},
		RawStruct: ekstypes.Nodegroup{
			ClusterName: aws.String("acme-prod"),
		},
	}

	checker := ngCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (nodegroup name unresolved)", result.Count())
	}
}

// The nodegroup name comes from Fields["nodegroup_name"], with RawStruct only
// as an override, so an unrelated RawStruct type yields Count=0 rather than -1.
func TestRelated_NG_EBS_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "general-pool",
		RawStruct: "not-a-nodegroup",
	}
	checker := ngCheckerByTarget(t, "ebs")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct type, no nodegroup_name field)", result.Count())
	}
}

func TestRelated_NG_Subnet_Match(t *testing.T) {
	const sub1 = "subnet-0a1b2c3d4e5f60001"
	const sub2 = "subnet-0a1b2c3d4e5f60002"

	res := resource.Resource{
		ID:     "general-pool",
		Name:   "general-pool",
		Fields: map[string]string{},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("acme-prod"),
			Subnets:       []string{sub1, sub2},
		},
	}

	checker := ngCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, nil)

	if result.Count() != 2 {
		t.Fatalf("Count = %d, want 2", result.Count())
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		seen[id] = true
	}
	for _, want := range []string{sub1, sub2} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs())
		}
	}
	if result.Err() != nil {
		t.Errorf("unexpected Err: %v", result.Err())
	}
}

func TestRelated_NG_Subnet_Empty(t *testing.T) {
	res := resource.Resource{
		ID:     "general-pool",
		Name:   "general-pool",
		Fields: map[string]string{},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			ClusterName:   aws.String("acme-prod"),
			Subnets:       []string{},
		},
	}

	checker := ngCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty Subnets)", result.Count())
	}
}

func TestRelated_NG_Subnet_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "general-pool",
		RawStruct: "not-a-nodegroup",
	}
	checker := ngCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, nil)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

func TestRelated_NG_SG_Found(t *testing.T) {
	res := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			Resources: &ekstypes.NodegroupResources{
				RemoteAccessSecurityGroup: aws.String("sg-remote12345"),
			},
		},
	}

	checker := ngCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "sg-remote12345" {
		t.Errorf("ResourceIDs = %v, want [sg-remote12345]", result.ResourceIDs())
	}
}

func TestRelated_NG_SG_NilResources(t *testing.T) {
	res := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			Resources:     nil,
		},
	}

	checker := ngCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (nil Resources)", result.Count())
	}
}

func TestRelated_NG_SG_EmptyGroupID(t *testing.T) {
	res := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
			Resources: &ekstypes.NodegroupResources{
				RemoteAccessSecurityGroup: aws.String(""),
			},
		},
	}

	checker := ngCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty RemoteAccessSecurityGroup)", result.Count())
	}
}

func TestRelated_NG_SG_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "general-pool",
		RawStruct: "not-a-nodegroup",
	}

	checker := ngCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

func TestRelated_NG_EC2_MatchByNodegroupTag(t *testing.T) {
	const ngName = "general-pool"
	const clusterName = "prod-cluster"

	ec2Res := resource.Resource{
		ID:   "i-abcdef1234567890",
		Name: "i-abcdef1234567890",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-abcdef1234567890"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String(ngName)},
				{Key: aws.String("eks:cluster-name"), Value: aws.String(clusterName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}
	source := resource.Resource{
		ID:   ngName,
		Name: ngName,
		Fields: map[string]string{
			"nodegroup_name": ngName,
			"cluster_name":   clusterName,
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String(ngName),
			ClusterName:   aws.String(clusterName),
		},
	}

	checker := ngCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "i-abcdef1234567890" {
		t.Errorf("ResourceIDs = %v, want [i-abcdef1234567890]", result.ResourceIDs())
	}
}

func TestRelated_NG_EC2_NoMatchDifferentCluster(t *testing.T) {
	const ngName = "general-pool"

	ec2Res := resource.Resource{
		ID:   "i-abcdef1234567890",
		Name: "i-abcdef1234567890",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-abcdef1234567890"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String(ngName)},
				{Key: aws.String("eks:cluster-name"), Value: aws.String("other-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}
	source := resource.Resource{
		ID:   ngName,
		Name: ngName,
		Fields: map[string]string{
			"nodegroup_name": ngName,
			"cluster_name":   "prod-cluster",
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String(ngName),
			ClusterName:   aws.String("prod-cluster"),
		},
	}

	checker := ngCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (different cluster)", result.Count())
	}
}

func TestRelated_NG_EC2_EmptyNodegroupName(t *testing.T) {
	source := resource.Resource{
		ID:   "",
		Name: "",
		Fields: map[string]string{
			"nodegroup_name": "",
		},
		RawStruct: ekstypes.Nodegroup{},
	}

	checker := ngCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty nodegroup name)", result.Count())
	}
}

func TestRelated_NG_EC2_NilCache(t *testing.T) {
	source := resource.Resource{
		ID:   "general-pool",
		Name: "general-pool",
		Fields: map[string]string{
			"nodegroup_name": "general-pool",
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("general-pool"),
		},
	}

	checker := ngCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil cache, nil clients)", result.Count())
	}
}

func TestRelated_NG_EC2_TruncatedCacheNoMatch(t *testing.T) {
	const ngName = "general-pool"

	ec2Res := resource.Resource{
		ID:   "i-xxxxxxxxxxxxxxxx",
		Name: "i-xxxxxxxxxxxxxxxx",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-xxxxxxxxxxxxxxxx"),
			Tags: []ec2types.Tag{
				{Key: aws.String("eks:nodegroup-name"), Value: aws.String("other-pool")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{ec2Res},
			IsTruncated: true,
		},
	}
	source := resource.Resource{
		ID:   ngName,
		Name: ngName,
		Fields: map[string]string{
			"nodegroup_name": ngName,
		},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String(ngName),
		},
	}

	checker := ngCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (truncated cache, no match)")
	}
}
