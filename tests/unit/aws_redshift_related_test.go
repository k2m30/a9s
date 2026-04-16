package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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

// --- redshift→cfn: tag-based aws:cloudformation:stack-name → cfn cache ---

// TestRelated_Redshift_CFN_Found verifies the checker extracts the CFN stack
// name from the Cluster's Tags and matches against the cfn cache.
func TestRelated_Redshift_CFN_Found(t *testing.T) {
	checker := redshiftCheckerByTarget(t, "cfn")

	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String("analytics-prod"),
			Tags: []redshifttypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("analytics-stack")},
			},
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "analytics-stack", Name: "analytics-stack", Fields: map[string]string{"stack_name": "analytics-stack"}},
			{ID: "other-stack", Name: "other-stack", Fields: map[string]string{"stack_name": "other-stack"}},
		}},
	}

	result := checker(context.Background(), nil, source, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "analytics-stack" {
		t.Errorf("ResourceIDs = %v, want [analytics-stack]", result.ResourceIDs)
	}
}

// TestRelated_Redshift_CFN_NoTag verifies Count=0 when the cluster has no
// aws:cloudformation:stack-name tag (not CFN-managed).
func TestRelated_Redshift_CFN_NoTag(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String("analytics-prod"),
			Tags: []redshifttypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("prod")},
			},
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "any-stack", Name: "any-stack"},
		}},
	}
	checker := redshiftCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no CFN tag)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}
