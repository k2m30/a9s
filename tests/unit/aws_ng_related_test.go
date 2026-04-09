package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
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

// --- Navigable Field Registration ---

func TestNavigableFields_NG_Registered(t *testing.T) {
	expected := map[string]string{
		"ClusterName": "eks",
		"NodeRole":    "role",
	}
	for path, wantTarget := range expected {
		nav := resource.IsFieldNavigable("ng", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found for ng", path)
			continue
		}
		if nav.TargetType != wantTarget {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, wantTarget)
		}
	}
}

// --- EKS Cluster checker (Pattern C — cache, ClusterName match) ---

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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-cluster" {
		t.Errorf("ResourceIDs = %v, want [my-cluster]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
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

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- IAM Role checker (Pattern C — cache, name extracted from ARN) ---

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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != roleName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, roleName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
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

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- ASG checker (Pattern C — cache, Resources.AutoScalingGroups[].Name match) ---

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

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != asgName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, asgName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
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

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
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

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}
