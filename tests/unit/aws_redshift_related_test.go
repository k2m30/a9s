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

func redshiftCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("redshift") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("redshift related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("redshift related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_Redshift_None(t *testing.T) {
	nav := resource.IsFieldNavigable("redshift", "ClusterIdentifier")
	if nav != nil {
		t.Errorf("expected no navigable fields for redshift, but ClusterIdentifier resolved to %v", nav)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, ClusterIdentifier dimension) ---

func TestRelated_Redshift_Alarms_Found(t *testing.T) {
	const clusterID = "analytics-prod"

	alarmRes := resource.Resource{
		ID: "redshift-cpu-high",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("redshift-cpu-high"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ClusterIdentifier"), Value: aws.String(clusterID)},
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
	}

	checker := redshiftCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "redshift-cpu-high" {
		t.Errorf("ResourceIDs = %v, want [redshift-cpu-high]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_Redshift_Alarms_NotFound(t *testing.T) {
	const clusterID = "analytics-prod"

	alarmRes := resource.Resource{
		ID: "other-cluster-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-cluster-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ClusterIdentifier"), Value: aws.String("different-cluster")},
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
	}

	checker := redshiftCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Redshift_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		Fields: map[string]string{
			"cluster_id": "analytics-prod",
		},
	}

	checker := redshiftCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- CloudFormation checker (stub) ---

func TestRelated_Redshift_CFN_IsStub(t *testing.T) {
	defs := resource.GetRelated("redshift")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for redshift")
	}
	for _, def := range defs {
		if def.TargetType == "cfn" {
			if def.Checker != nil {
				t.Errorf("redshift cfn Checker should be nil (stub)")
			}
			return
		}
	}
	t.Error("expected related def for target cfn not found for redshift")
}

// --- Demo Checker ---

func TestRelatedDemo_Redshift_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("redshift")
	if checker == nil {
		t.Fatal("no demo checker registered for redshift")
	}

	results := checker(resource.Resource{ID: "analytics-prod"})
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
