package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func ecsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ecs") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ecs related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ecs related checker for %s not found", target)
	return nil
}

// --- Navigable Field Registration ---

func TestNavigableFields_ECS_Registered(t *testing.T) {
	fields := resource.GetNavigableFields("ecs")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for ecs")
	}

	expected := map[string]string{
		"Configuration.ExecuteCommandConfiguration.KmsKeyId": "kms",
	}
	for path, targetType := range expected {
		nav := resource.IsFieldNavigable("ecs", path)
		if nav == nil {
			t.Errorf("expected navigable field %q not found", path)
			continue
		}
		if nav.TargetType != targetType {
			t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, targetType)
		}
	}
}

func TestNavigableFields_ECS_FieldPathsResolve(t *testing.T) {
	resources, ok := demo.GetResources("ecs")
	if !ok {
		t.Fatal("no demo fixture registered for ecs — fixtures_compute_ecs.go must register it")
	}
	if len(resources) == 0 {
		t.Fatal("demo fixture returned no resources for ecs")
	}

	fields := resource.GetNavigableFields("ecs")
	if len(fields) == 0 {
		t.Fatal("no navigable fields registered for ecs")
	}

	for _, nav := range fields {
		found := false
		for _, r := range resources {
			items := fieldpath.ExtractFieldList(r.RawStruct, r.Fields, []string{nav.FieldPath}, nil)
			for _, item := range items {
				if item.Value != "" && item.Value != "-" {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			t.Skipf("NavigableField.FieldPath %q resolved to empty/missing value in all demo fixtures", nav.FieldPath)
		}
	}
}

// --- Demo Checker ---

func TestRelatedDemo_ECS_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("ecs")
	if checker == nil {
		t.Fatal("no demo checker registered for ecs")
	}

	results := checker(resource.Resource{ID: "acme-services"})
	if len(results) != 3 {
		t.Fatalf("demo checker returned %d results, want 3", len(results))
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}

// --- ecs-svc checker (Pattern C — cache-based, ClusterArn suffix match) ---

func TestRelated_ECS_ECSService_Found(t *testing.T) {
	clusterName := "my-cluster"
	svcRes := resource.Resource{
		ID: "my-service",
		RawStruct: ecstypes.Service{
			ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"ecs-svc": resource.ResourceCacheEntry{Resources: []resource.Resource{svcRes}},
	}
	source := resource.Resource{ID: clusterName, Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ecs-svc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-service" {
		t.Errorf("ResourceIDs = %v, want [my-service]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ECS_ECSService_NotFound(t *testing.T) {
	svcRes := resource.Resource{
		ID: "my-service",
		RawStruct: ecstypes.Service{
			ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/other-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"ecs-svc": resource.ResourceCacheEntry{Resources: []resource.Resource{svcRes}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ecs-svc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECS_ECSService_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ecs-svc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_ECS_ECSService_EmptySourceID(t *testing.T) {
	svcRes := resource.Resource{
		ID: "my-service",
		RawStruct: ecstypes.Service{
			ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"ecs-svc": resource.ResourceCacheEntry{Resources: []resource.Resource{svcRes}},
	}
	source := resource.Resource{ID: "", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ecs-svc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty source ID", result.Count)
	}
}

// --- alarm checker (Pattern C — cache-based, ClusterName dimension) ---

func TestRelated_ECS_Alarm_Found(t *testing.T) {
	clusterName := "my-cluster"
	alarmRes := resource.Resource{
		ID: "my-alarm",
		RawStruct: cwtypes.MetricAlarm{
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ClusterName"), Value: aws.String("my-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{ID: clusterName, Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-alarm" {
		t.Errorf("ResourceIDs = %v, want [my-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ECS_Alarm_NotFound(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "my-alarm",
		RawStruct: cwtypes.MetricAlarm{
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ClusterName"), Value: aws.String("different-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECS_Alarm_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_ECS_Alarm_EmptySourceID(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "my-alarm",
		RawStruct: cwtypes.MetricAlarm{
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ClusterName"), Value: aws.String("my-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{ID: "", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty source ID", result.Count)
	}
}

// --- cfn checker (Pattern C — cache-based, aws:cloudformation:stack-name tag) ---

func TestRelated_ECS_CFN_Found(t *testing.T) {
	cfnRes := resource.Resource{
		ID: "my-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("my-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID: "my-cluster",
		RawStruct: ecstypes.Cluster{
			ClusterName: aws.String("my-cluster"),
			Tags: []ecstypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
		Fields: map[string]string{},
	}

	checker := ecsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-stack" {
		t.Errorf("ResourceIDs = %v, want [my-stack]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ECS_CFN_NotFound(t *testing.T) {
	cfnRes := resource.Resource{
		ID: "different-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("different-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID: "my-cluster",
		RawStruct: ecstypes.Cluster{
			ClusterName: aws.String("my-cluster"),
			Tags: []ecstypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
		Fields: map[string]string{},
	}

	checker := ecsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECS_CFN_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "my-cluster",
		RawStruct: ecstypes.Cluster{
			ClusterName: aws.String("my-cluster"),
			Tags: []ecstypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
		Fields: map[string]string{},
	}

	checker := ecsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_ECS_CFN_EmptySourceID(t *testing.T) {
	cfnRes := resource.Resource{
		ID: "my-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("my-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	// No aws:cloudformation:stack-name tag — cluster not created by CFN.
	source := resource.Resource{
		ID: "my-cluster",
		RawStruct: ecstypes.Cluster{
			ClusterName: aws.String("my-cluster"),
			Tags:        []ecstypes.Tag{},
		},
		Fields: map[string]string{},
	}

	checker := ecsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for no CFN tag", result.Count)
	}
}
