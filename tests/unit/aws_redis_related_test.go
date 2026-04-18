package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func redisCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("redis") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("redis related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("redis related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_Redis_SecurityGroup(t *testing.T) {
	nav := resource.IsFieldNavigable("redis", "SecurityGroups.SecurityGroupId")
	if nav == nil {
		t.Fatal("expected SecurityGroups.SecurityGroupId to be navigable for redis")
	}
	if nav.TargetType != "sg" {
		t.Errorf("expected TargetType=sg, got %q", nav.TargetType)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, CacheClusterId dimension) ---

func TestRelated_Redis_Alarms_Found(t *testing.T) {
	const clusterID = "acme-prod-sessions"

	alarmRes := resource.Resource{
		ID: "redis-cpu-high",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("redis-cpu-high"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("CacheClusterId"), Value: aws.String(clusterID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   clusterID,
		Name: clusterID,
		Fields: map[string]string{
			"cluster_id": clusterID,
		},
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String(clusterID),
		},
	}

	checker := redisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "redis-cpu-high" {
		t.Errorf("ResourceIDs = %v, want [redis-cpu-high]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Redis_Alarms_NotFound(t *testing.T) {
	const clusterID = "acme-prod-sessions"

	alarmRes := resource.Resource{
		ID: "other-cluster-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-cluster-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("CacheClusterId"), Value: aws.String("different-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   clusterID,
		Name: clusterID,
		Fields: map[string]string{
			"cluster_id": clusterID,
		},
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String(clusterID),
		},
	}

	checker := redisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Redis_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		Fields: map[string]string{
			"cluster_id": "acme-prod-sessions",
		},
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String("acme-prod-sessions"),
		},
	}

	checker := redisCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- redis→cfn: unknown (Count: -1) because DescribeCacheClusters does not
// return tags — ListTagsForResource is required to determine CFN ownership and
// the fetcher does not make that per-cluster call. ---

// TestRelated_Redis_CFN_UnknownWithoutTags verifies the checker returns Count=-1
// (unknown) regardless of whether there are CFN stacks in cache, because the
// CacheCluster RawStruct has no Tags field to read aws:cloudformation:stack-name
// from. Real behavior: we cannot deterministically decide relatedness.
func TestRelated_Redis_CFN_UnknownWithoutTags(t *testing.T) {
	checker := redisCheckerByTarget(t, "cfn")

	source := resource.Resource{
		ID:   "acme-prod-sessions",
		Name: "acme-prod-sessions",
		RawStruct: elasticachetypes.CacheCluster{
			CacheClusterId: aws.String("acme-prod-sessions"),
		},
	}

	// With a populated cfn cache we still cannot answer (no tags available).
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "some-stack", Name: "some-stack", Fields: map[string]string{"stack_name": "some-stack"}},
		}},
	}
	result := checker(context.Background(), nil, source, cache)
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — tags not fetched)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}

	// With an empty cache the answer is still unknown (not zero).
	result2 := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result2.Count != -1 {
		t.Errorf("empty cache: Count = %d, want -1", result2.Count)
	}
}
