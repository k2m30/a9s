// aws_ecs_tasks_related_test.go contains unit tests for ECS Tasks related-resource checkers.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func ecsTaskCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ecs-task") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ecs-task related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ecs-task related checker for %s not found", target)
	return nil
}

// --- ECS Service checker (Pattern F — Group field) ---

func TestRelated_ECSTask_Service_FromGroup(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "ecs-svc")
	task := ecstypes.Task{
		Group: aws.String("service:api-gateway"),
	}
	res := resource.Resource{
		ID:        "abc123",
		Fields:    map[string]string{},
		RawStruct: task,
	}

	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Fatalf("expected Count=1, got %d", result.Count)
	}
	if len(result.ResourceIDs) != 1 {
		t.Fatalf("expected 1 ResourceID, got %d", len(result.ResourceIDs))
	}
	if result.ResourceIDs[0] != "api-gateway" {
		t.Errorf("expected ResourceIDs[0]=%q, got %q", "api-gateway", result.ResourceIDs[0])
	}
}

func TestRelated_ECSTask_Service_NoGroup(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "ecs-svc")
	task := ecstypes.Task{
		Group: aws.String(""),
	}
	res := resource.Resource{
		ID:        "abc123",
		Fields:    map[string]string{},
		RawStruct: task,
	}

	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("expected Count=0 for empty group, got %d", result.Count)
	}
}

func TestRelated_ECSTask_Service_NonServiceGroup(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "ecs-svc")
	task := ecstypes.Task{
		Group: aws.String("family:batch-job"),
	}
	res := resource.Resource{
		ID:        "abc123",
		Fields:    map[string]string{},
		RawStruct: task,
	}

	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("expected Count=0 for non-service group, got %d", result.Count)
	}
}

func TestRelated_ECSTask_Service_InvalidRawStruct(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "ecs-svc")
	res := resource.Resource{
		ID:        "abc123",
		Fields:    map[string]string{},
		RawStruct: "not-a-task",
	}

	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("expected Count=-1 for invalid RawStruct, got %d", result.Count)
	}
}

// --- ECS Cluster checker (Pattern F — ClusterArn field with ARN fallback) ---

func TestRelated_ECSTask_Cluster_FromArn(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "ecs")
	task := ecstypes.Task{
		ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/acme-services"),
	}
	res := resource.Resource{
		ID:        "abc123",
		Fields:    map[string]string{},
		RawStruct: task,
	}

	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Fatalf("expected Count=1, got %d", result.Count)
	}
	if len(result.ResourceIDs) != 1 {
		t.Fatalf("expected 1 ResourceID, got %d", len(result.ResourceIDs))
	}
	if result.ResourceIDs[0] != "acme-services" {
		t.Errorf("expected ResourceIDs[0]=%q, got %q", "acme-services", result.ResourceIDs[0])
	}
}

func TestRelated_ECSTask_Cluster_NilArn(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "ecs")
	// No ClusterArn on the struct — fall back to Fields["cluster"]
	task := ecstypes.Task{}
	res := resource.Resource{
		ID: "abc123",
		Fields: map[string]string{
			"cluster": "arn:aws:ecs:us-east-1:123456789012:cluster/acme-services",
		},
		RawStruct: task,
	}

	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Fatalf("expected Count=1, got %d", result.Count)
	}
	if len(result.ResourceIDs) != 1 {
		t.Fatalf("expected 1 ResourceID, got %d", len(result.ResourceIDs))
	}
	if result.ResourceIDs[0] != "acme-services" {
		t.Errorf("expected ResourceIDs[0]=%q, got %q", "acme-services", result.ResourceIDs[0])
	}
}

func TestRelated_ECSTask_Cluster_NoCluster(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "ecs")
	task := ecstypes.Task{}
	res := resource.Resource{
		ID:        "abc123",
		Fields:    map[string]string{},
		RawStruct: task,
	}

	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("expected Count=0 when no cluster info, got %d", result.Count)
	}
}

// --- Demo Checker ---

func TestRelatedDemo_ECSTask_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("ecs-task")
	if checker == nil {
		t.Fatal("no demo checker registered for ecs-task")
	}

	results := checker(resource.Resource{ID: "abc123"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}

	targets := make(map[string]bool)
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
		targets[r.TargetType] = true
	}

	for _, expected := range []string{"ecs-svc", "ecs"} {
		if !targets[expected] {
			t.Errorf("demo checker returned no result for target %q", expected)
		}
	}
}
