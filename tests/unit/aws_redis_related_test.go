package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
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

func TestNavigableFields_Redis_None(t *testing.T) {
	nav := resource.IsFieldNavigable("redis", "CacheClusterId")
	if nav != nil {
		t.Errorf("expected no navigable fields for redis, but CacheClusterId resolved to %v", nav)
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

// --- CloudFormation checker (stub) ---

func TestRelated_Redis_CFN_IsStub(t *testing.T) {
	defs := resource.GetRelated("redis")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for redis")
	}
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker != nil {
				t.Errorf("redis cfn Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target cfn not found for redis")
}

// --- Demo Checker ---

func TestRelatedDemo_Redis_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("redis")
	if checker == nil {
		t.Fatal("no demo checker registered for redis")
	}

	results := checker(resource.Resource{ID: "acme-prod-sessions"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"alarm": false, "cfn": false}
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
}
