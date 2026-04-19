// aws_eks_related_extra_wave2_test.go — coverage wave 2 for eks_related_extra.go
// Covers: checkEKSSubnet, checkEKSASG, checkEKSCTEvents
// Each has: happy-path, no-match, and one edge case.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// checkEKSSubnet — Pattern F: ResourcesVpcConfig.SubnetIds
// ---------------------------------------------------------------------------

func TestRelated_EKS_Subnet_ReturnsSubnetIDs(t *testing.T) {
	const subnet1 = "subnet-0aaa111111111111a"
	const subnet2 = "subnet-0bbb222222222222b"

	src := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: ekstypes.Cluster{
			Name: aws.String("acme-services"),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				SubnetIds: []string{subnet1, subnet2},
			},
		},
	}

	checker := eksCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	for _, want := range []string{subnet1, subnet2} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

func TestRelated_EKS_Subnet_ReturnsZeroWhenNoVpcConfig(t *testing.T) {
	src := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: ekstypes.Cluster{
			Name:               aws.String("acme-services"),
			ResourcesVpcConfig: nil,
		},
	}

	checker := eksCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil VpcConfig)", result.Count)
	}
}

// Edge: empty SubnetIds slice → Count=0.
func TestRelated_EKS_Subnet_ReturnsZeroWhenEmptySubnetIDs(t *testing.T) {
	src := resource.Resource{
		ID:   "acme-services",
		Name: "acme-services",
		RawStruct: ekstypes.Cluster{
			Name: aws.String("acme-services"),
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				SubnetIds: []string{},
			},
		},
	}

	checker := eksCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty SubnetIds)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkEKSASG — Pattern C: scan ng cache for matching ClusterName + extract ASG names
// ---------------------------------------------------------------------------

func TestRelated_EKS_ASG_MatchByNodeGroupClusterName(t *testing.T) {
	const clusterName = "acme-services"
	const asgName = "eks-acme-services-ng-general-asg"

	ngRes := resource.Resource{
		ID:   "ng-general",
		Name: "ng-general",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("ng-general"),
			ClusterName:   aws.String(clusterName),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String(asgName)},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	src := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		RawStruct: ekstypes.Cluster{
			Name: aws.String(clusterName),
		},
	}

	checker := eksCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != asgName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, asgName)
	}
}

func TestRelated_EKS_ASG_NoMatchDifferentCluster(t *testing.T) {
	const clusterName = "acme-services"

	ngRes := resource.Resource{
		ID:   "ng-other",
		Name: "ng-other",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("ng-other"),
			ClusterName:   aws.String("other-cluster"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("other-asg")},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	src := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		RawStruct: ekstypes.Cluster{
			Name: aws.String(clusterName),
		},
	}

	checker := eksCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (different cluster)", result.Count)
	}
}

// Edge: two node groups in same cluster → both ASGs deduplicated.
func TestRelated_EKS_ASG_DeduplicatesAcrossNodeGroups(t *testing.T) {
	const clusterName = "acme-services"
	const asg1 = "eks-acme-ng1-asg"
	const asg2 = "eks-acme-ng2-asg"

	ngRes1 := resource.Resource{
		ID: "ng-1",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("ng-1"),
			ClusterName:   aws.String(clusterName),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String(asg1)},
				},
			},
		},
	}
	ngRes2 := resource.Resource{
		ID: "ng-2",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("ng-2"),
			ClusterName:   aws.String(clusterName),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String(asg2)},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ngRes1, ngRes2},
		},
	}

	src := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		RawStruct: ekstypes.Cluster{
			Name: aws.String(clusterName),
		},
	}

	checker := eksCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2; ResourceIDs: %v", result.Count, result.ResourceIDs)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	if !seen[asg1] {
		t.Errorf("ResourceIDs missing %q; got %v", asg1, result.ResourceIDs)
	}
	if !seen[asg2] {
		t.Errorf("ResourceIDs missing %q; got %v", asg2, result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkEKSCTEvents — Pattern C: scan ct-events for ResourceName containing clusterName
// ---------------------------------------------------------------------------

func TestRelated_EKS_CTEvents_MatchByResourceName(t *testing.T) {
	const clusterName = "acme-services"
	const eventID = "ct-event-abc123"

	evRes := resource.Resource{
		ID: eventID,
		RawStruct: cloudtrailtypes.Event{
			EventId: aws.String(eventID),
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceName: aws.String("arn:aws:eks:us-east-1:123456789012:cluster/" + clusterName),
					ResourceType: aws.String("AWS::EKS::Cluster"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes}},
	}

	src := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		RawStruct: ekstypes.Cluster{
			Name: aws.String(clusterName),
		},
	}

	checker := eksCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != eventID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, eventID)
	}
}

func TestRelated_EKS_CTEvents_NoMatchDifferentCluster(t *testing.T) {
	const clusterName = "acme-services"

	evRes := resource.Resource{
		ID: "ct-event-xyz",
		RawStruct: cloudtrailtypes.Event{
			EventId: aws.String("ct-event-xyz"),
			Resources: []cloudtrailtypes.Resource{
				{
					ResourceName: aws.String("arn:aws:eks:us-east-1:123456789012:cluster/other-cluster"),
					ResourceType: aws.String("AWS::EKS::Cluster"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes}},
	}

	src := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		RawStruct: ekstypes.Cluster{
			Name: aws.String(clusterName),
		},
	}

	checker := eksCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (different cluster in event)", result.Count)
	}
}

// Edge: event with wrong RawStruct (not cloudtrailtypes.Event) is skipped.
func TestRelated_EKS_CTEvents_SkipsWrongRawStructEvent(t *testing.T) {
	const clusterName = "acme-services"

	// RawStruct is wrong type — assertStruct will fail, event is skipped.
	evRes := resource.Resource{
		ID:        "ct-event-abc123",
		RawStruct: "not-a-cloudtrail-event",
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes}},
	}

	src := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		RawStruct: ekstypes.Cluster{
			Name: aws.String(clusterName),
		},
	}

	checker := eksCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct skipped)", result.Count)
	}
}

// TestRelated_EKS_ASG_NilClientFallsBackToCache verifies that with no live clients
// the checker still uses the ng cache to find ASGs.
func TestRelated_EKS_ASG_NilClientFallsBackToCache(t *testing.T) {
	const clusterName = "acme-services"
	const asgName = "eks-acme-services-ng-spot-asg"

	ngRes := resource.Resource{
		ID: "ng-spot",
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("ng-spot"),
			ClusterName:   aws.String(clusterName),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String(asgName)},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	src := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		RawStruct: ekstypes.Cluster{
			Name: aws.String(clusterName),
		},
	}

	// Pass nil clients — the ng cache path does not require live clients.
	checker := eksCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (nil client, ng cache used)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != asgName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, asgName)
	}
}

