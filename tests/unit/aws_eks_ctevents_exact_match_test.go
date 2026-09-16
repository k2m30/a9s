// aws_eks_ctevents_exact_match_test.go — which CloudTrail events belong to an
// EKS cluster.
//
// Cluster names routinely share a prefix ("prod", "prod-blue", "prod-green"),
// and a CloudTrail event's Resources slice carries entries for every service.
// A substring test over ResourceName with no resource-type check attributes
// one cluster's events to another and mixes in unrelated resources that merely
// happen to be named after the cluster.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/resource"

	_ "github.com/k2m30/a9s/v3/core/aws"
)

// ctEventRow wraps one CloudTrail event resource entry as a cached ct-events row.
func ctEventRow(id, resourceType, resourceName string) resource.Resource {
	entry := cloudtrailtypes.Resource{}
	if resourceType != "" {
		entry.ResourceType = aws.String(resourceType)
	}
	if resourceName != "" {
		entry.ResourceName = aws.String(resourceName)
	}
	return resource.Resource{
		ID: id,
		RawStruct: cloudtrailtypes.Event{
			EventId:   aws.String(id),
			Resources: []cloudtrailtypes.Resource{entry},
		},
	}
}

// TestRelated_EKS_CTEvents_MatchesOnlyTheClusterItself pins the whole
// attribution rule in one cache: the cluster's own events count, a
// prefix-sharing sibling's do not, and a non-EKS resource named after the
// cluster does not.
func TestRelated_EKS_CTEvents_MatchesOnlyTheClusterItself(t *testing.T) {
	const cluster = "prod"
	const clusterARN = "arn:aws:eks:eu-west-2:123456789012:cluster/prod"
	const siblingARN = "arn:aws:eks:eu-west-2:123456789012:cluster/prod-blue"

	events := []resource.Resource{
		ctEventRow("ct-event-by-arn", "AWS::EKS::Cluster", clusterARN),
		ctEventRow("ct-event-by-name", "AWS::EKS::Cluster", cluster),
		ctEventRow("ct-event-sibling-arn", "AWS::EKS::Cluster", siblingARN),
		ctEventRow("ct-event-sibling-name", "AWS::EKS::Cluster", "prod-blue"),
		ctEventRow("ct-event-wrong-type", "AWS::EC2::Instance", cluster),
		ctEventRow("ct-event-no-type", "", cluster),
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: events},
	}

	src := resource.Resource{
		ID:        cluster,
		Name:      cluster,
		RawStruct: ekstypes.Cluster{Name: aws.String(cluster), Arn: aws.String(clusterARN)},
	}

	result := eksCheckerByTarget(t, "ct-events")(context.Background(), nil, src, cache)

	want := map[string]bool{"ct-event-by-arn": true, "ct-event-by-name": true}
	got := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		got[id] = true
	}
	for id := range want {
		if !got[id] {
			t.Errorf("event %q belongs to cluster %q but is not counted; got %v", id, cluster, result.ResourceIDs())
		}
	}
	for id := range got {
		if !want[id] {
			t.Errorf("event %q does not belong to cluster %q but is counted; got %v", id, cluster, result.ResourceIDs())
		}
	}
	if result.Count() != len(want) {
		t.Errorf("Count = %d, want %d", result.Count(), len(want))
	}
}

// TestRelated_EKS_CTEvents_PrefixSiblingSeesOnlyItsOwn is the mirror case:
// the longer-named cluster must not inherit the shorter one's events either.
func TestRelated_EKS_CTEvents_PrefixSiblingSeesOnlyItsOwn(t *testing.T) {
	const cluster = "prod-blue"
	const clusterARN = "arn:aws:eks:eu-west-2:123456789012:cluster/prod-blue"

	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{
			ctEventRow("ct-event-own", "AWS::EKS::Cluster", clusterARN),
			ctEventRow("ct-event-shorter", "AWS::EKS::Cluster", "arn:aws:eks:eu-west-2:123456789012:cluster/prod"),
		}},
	}

	src := resource.Resource{
		ID:        cluster,
		Name:      cluster,
		RawStruct: ekstypes.Cluster{Name: aws.String(cluster), Arn: aws.String(clusterARN)},
	}

	result := eksCheckerByTarget(t, "ct-events")(context.Background(), nil, src, cache)

	if result.Count() != 1 || len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "ct-event-own" {
		t.Errorf("Count/ResourceIDs = %d/%v, want 1/[ct-event-own]", result.Count(), result.ResourceIDs())
	}
}

// TestRelated_EKS_CTEvents_NoEventsIsAProvenZero is the negative half: a
// cluster whose cache holds only other clusters' events resolves to zero
// rather than to a spurious match.
func TestRelated_EKS_CTEvents_NoEventsIsAProvenZero(t *testing.T) {
	const cluster = "prod"

	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{
			ctEventRow("ct-event-other", "AWS::EKS::Cluster", "arn:aws:eks:eu-west-2:123456789012:cluster/staging"),
		}},
	}

	src := resource.Resource{
		ID:        cluster,
		Name:      cluster,
		RawStruct: ekstypes.Cluster{Name: aws.String(cluster)},
	}

	result := eksCheckerByTarget(t, "ct-events")(context.Background(), nil, src, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0; got %v", result.Count(), result.ResourceIDs())
	}
}
