package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
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

// --- Demo Checker ---

func TestRelatedDemo_EKS_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("eks")
	if checker == nil {
		t.Fatal("no demo checker registered for eks")
	}

	results := checker(resource.Resource{ID: "acme-services"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all three expected target types are present.
	wantTargets := map[string]bool{"ng": false, "alarm": false, "cfn": false}
	for _, r := range results {
		if _, ok := wantTargets[r.TargetType]; ok {
			wantTargets[r.TargetType] = true
		}
	}
	for target, found := range wantTargets {
		if !found {
			t.Errorf("demo checker missing result for target %q", target)
		}
	}

	// At least one result must have Count > 0 (ng or alarm).
	hasPositive := false
	for _, r := range results {
		if r.Count > 0 {
			hasPositive = true
			break
		}
	}
	if !hasPositive {
		t.Error("demo checker returned no result with Count > 0")
	}
}
