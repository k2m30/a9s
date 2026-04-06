package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func mskCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("msk") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("msk related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("msk related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_MSK_None(t *testing.T) {
	nav := resource.IsFieldNavigable("msk", "ClusterName")
	if nav != nil {
		t.Errorf("expected no navigable fields for msk, but ClusterName resolved to %v", nav)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, "Cluster Name" dimension) ---

func TestRelated_MSK_Alarms_Found(t *testing.T) {
	const clusterName = "analytics-kafka-cluster"

	alarmRes := resource.Resource{
		ID: "msk-cpu-utilization",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("msk-cpu-utilization"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("Cluster Name"), Value: aws.String(clusterName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		Fields: map[string]string{
			"cluster_name": clusterName,
		},
	}

	checker := mskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "msk-cpu-utilization" {
		t.Errorf("ResourceIDs = %v, want [msk-cpu-utilization]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_MSK_Alarms_NotFound(t *testing.T) {
	const clusterName = "analytics-kafka-cluster"

	alarmRes := resource.Resource{
		ID: "other-cluster-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-cluster-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("Cluster Name"), Value: aws.String("different-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		Fields: map[string]string{
			"cluster_name": clusterName,
		},
	}

	checker := mskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_MSK_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-kafka-cluster",
		Name: "analytics-kafka-cluster",
		Fields: map[string]string{
			"cluster_name": "analytics-kafka-cluster",
		},
	}

	checker := mskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- msk→lambda: undeterminable from cache, returns Count: 0 ---

func TestRelated_MSK_Lambda_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-kafka-cluster",
		Name: "analytics-kafka-cluster",
	}
	checker := mskCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "lambda" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "lambda")
	}
}

// --- msk→cfn: undeterminable from cache, returns Count: 0 ---

func TestRelated_MSK_CFN_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-kafka-cluster",
		Name: "analytics-kafka-cluster",
	}
	checker := mskCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}

// --- Demo Checker ---

func TestRelatedDemo_MSK_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("msk")
	if checker == nil {
		t.Fatal("no demo checker registered for msk")
	}

	results := checker(resource.Resource{ID: "analytics-kafka-cluster"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"lambda": false, "alarm": false, "cfn": false}
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
