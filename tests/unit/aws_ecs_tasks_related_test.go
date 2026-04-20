// aws_ecs_tasks_related_test.go contains unit tests for ECS Tasks related-resource checkers.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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

// --- Secrets checker (Pattern F — TaskDefinition.ContainerDefinitions[].Secrets) ---

// TestRelated_ECSTask_Secrets_Match verifies that secretsmanager ARNs in
// ContainerDefinitions[].Secrets[].ValueFrom are returned as ResourceIDs.
func TestRelated_ECSTask_Secrets_Match(t *testing.T) {
	const smARN1 = "arn:aws:secretsmanager:us-east-1:123456789012:secret:db-password-AbcXyz"
	const smARN2 = "arn:aws:secretsmanager:us-east-1:123456789012:secret:api-key-XyzAbc"
	td := ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name: aws.String("app"),
				Secrets: []ecstypes.Secret{
					{Name: aws.String("DB_PASSWORD"), ValueFrom: aws.String(smARN1)},
					{Name: aws.String("API_KEY"), ValueFrom: aws.String(smARN2)},
				},
			},
		},
	}
	res := resource.Resource{
		ID:        "my-task-def:5",
		Fields:    map[string]string{},
		RawStruct: td,
	}

	checker := ecsTaskCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 2 {
		t.Fatalf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Fatalf("ResourceIDs length = %d, want 2: %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	for _, want := range []string{smARN1, smARN2} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

// TestRelated_ECSTask_Secrets_Empty verifies that a TaskDefinition with no
// secretsmanager ARNs in Secrets produces Count=0.
func TestRelated_ECSTask_Secrets_Empty(t *testing.T) {
	td := ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name: aws.String("app"),
				Secrets: []ecstypes.Secret{
					// SSM parameter only — not a secretsmanager ARN
					{Name: aws.String("PARAM"), ValueFrom: aws.String("arn:aws:ssm:us-east-1:123456789012:parameter/my-param")},
				},
			},
		},
	}
	res := resource.Resource{
		ID:        "my-task-def:5",
		Fields:    map[string]string{},
		RawStruct: td,
	}

	checker := ecsTaskCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no secretsmanager ARNs)", result.Count)
	}
}

// TestRelated_ECSTask_Secrets_WrongRawStruct verifies that a non-TaskDefinition
// RawStruct returns Count=-1 (wrong type guard).
func TestRelated_ECSTask_Secrets_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "task-abc123",
		RawStruct: "not-a-task-definition",
	}

	checker := ecsTaskCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// --- SSM checker (Pattern F — TaskDefinition.ContainerDefinitions[].Secrets) ---

// TestRelated_ECSTask_SSM_Match verifies that SSM parameter ARNs in
// ContainerDefinitions[].Secrets[].ValueFrom are returned as parameter names.
func TestRelated_ECSTask_SSM_Match(t *testing.T) {
	const ssmARN1 = "arn:aws:ssm:us-east-1:123456789012:parameter/prod/db/host"
	const ssmARN2 = "arn:aws:ssm:us-east-1:123456789012:parameter/prod/api/key"
	td := ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name: aws.String("app"),
				Secrets: []ecstypes.Secret{
					{Name: aws.String("DB_HOST"), ValueFrom: aws.String(ssmARN1)},
					{Name: aws.String("API_KEY"), ValueFrom: aws.String(ssmARN2)},
				},
			},
		},
	}
	res := resource.Resource{
		ID:        "my-task-def:5",
		Fields:    map[string]string{},
		RawStruct: td,
	}

	checker := ecsTaskCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 2 {
		t.Fatalf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Fatalf("ResourceIDs length = %d, want 2: %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	// The checker extracts the parameter name (suffix after "/parameter/").
	for _, want := range []string{"prod/db/host", "prod/api/key"} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

// TestRelated_ECSTask_SSM_Empty verifies that a TaskDefinition with only
// secretsmanager ARNs (no SSM) produces Count=0.
func TestRelated_ECSTask_SSM_Empty(t *testing.T) {
	td := ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Name: aws.String("app"),
				Secrets: []ecstypes.Secret{
					// secretsmanager only — not SSM
					{Name: aws.String("DB_PWD"), ValueFrom: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:db-pwd-AbcXyz")},
				},
			},
		},
	}
	res := resource.Resource{
		ID:        "my-task-def:5",
		Fields:    map[string]string{},
		RawStruct: td,
	}

	checker := ecsTaskCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SSM ARNs)", result.Count)
	}
}

// TestRelated_ECSTask_SSM_WrongRawStruct verifies that a non-TaskDefinition
// RawStruct returns Count=-1 (wrong type guard).
func TestRelated_ECSTask_SSM_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "task-abc123",
		RawStruct: "not-a-task-definition",
	}

	checker := ecsTaskCheckerByTarget(t, "ssm")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSTaskAlarm — cache-based, TaskId or TaskArn dimension contains taskID
// ---------------------------------------------------------------------------

func TestRelated_ECSTask_Alarm_MatchByTaskIdDimension(t *testing.T) {
	taskID := "abc123def456"
	alarmRes := resource.Resource{
		ID: "ecs-task-cpu-alarm",
		RawStruct: cwtypes.MetricAlarm{
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("TaskId"), Value: aws.String("abc123def456")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	res := resource.Resource{ID: taskID, Fields: map[string]string{}}

	checker := ecsTaskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "ecs-task-cpu-alarm" {
		t.Errorf("ResourceIDs = %v, want [ecs-task-cpu-alarm]", result.ResourceIDs)
	}
}

func TestRelated_ECSTask_Alarm_MatchByTaskArnDimension(t *testing.T) {
	taskID := "abc123def456"
	alarmRes := resource.Resource{
		ID: "ecs-task-mem-alarm",
		RawStruct: cwtypes.MetricAlarm{
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("TaskArn"), Value: aws.String("arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123def456")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	res := resource.Resource{ID: taskID, Fields: map[string]string{}}

	checker := ecsTaskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (TaskArn contains taskID)", result.Count)
	}
}

func TestRelated_ECSTask_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "unrelated-alarm",
		RawStruct: cwtypes.MetricAlarm{
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("TaskId"), Value: aws.String("totally-different-task")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	res := resource.Resource{ID: "abc123def456", Fields: map[string]string{}}

	checker := ecsTaskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECSTask_Alarm_EmptyTaskID(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, resource.Resource{ID: ""}, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty task ID)", result.Count)
	}
}

func TestRelated_ECSTask_Alarm_NilCache(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, resource.Resource{ID: "abc123def456"}, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSTaskCTEvents — cache-based, ResourceName contains taskID
// ---------------------------------------------------------------------------

func TestRelated_ECSTask_CTEvents_Match(t *testing.T) {
	taskID := "abc123def456"
	evRes := resource.Resource{
		ID: "ct-event-xyz",
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String("arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123def456")},
			},
		},
	}
	otherEv := resource.Resource{
		ID: "ct-event-other",
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes, otherEv}},
	}
	res := resource.Resource{ID: taskID, Fields: map[string]string{}}

	checker := ecsTaskCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "ct-event-xyz" {
		t.Errorf("ResourceIDs = %v, want [ct-event-xyz]", result.ResourceIDs)
	}
}

func TestRelated_ECSTask_CTEvents_NoMatch(t *testing.T) {
	evRes := resource.Resource{
		ID: "ct-event-other",
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes}},
	}
	res := resource.Resource{ID: "abc123def456", Fields: map[string]string{}}

	checker := ecsTaskCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECSTask_CTEvents_EmptyTaskID(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, resource.Resource{ID: ""}, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty task ID)", result.Count)
	}
}

func TestRelated_ECSTask_CTEvents_NilCache(t *testing.T) {
	checker := ecsTaskCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, resource.Resource{ID: "abc123def456"}, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSTaskEC2 — container-instance UUID from ContainerInstanceArn
// ---------------------------------------------------------------------------

func TestRelated_ECSTask_EC2_MatchFromContainerInstanceArn(t *testing.T) {
	task := ecstypes.Task{
		ContainerInstanceArn: aws.String("arn:aws:ecs:us-east-1:123456789012:container-instance/my-cluster/abcdef1234567890"),
	}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abcdef1234567890" {
		t.Errorf("ResourceIDs = %v, want [abcdef1234567890]", result.ResourceIDs)
	}
}

func TestRelated_ECSTask_EC2_FargateTaskNilArn(t *testing.T) {
	task := ecstypes.Task{}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (Fargate task — no container instance)", result.Count)
	}
}

func TestRelated_ECSTask_EC2_EmptyContainerInstanceArn(t *testing.T) {
	task := ecstypes.Task{
		ContainerInstanceArn: aws.String(""),
	}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ContainerInstanceArn)", result.Count)
	}
}

func TestRelated_ECSTask_EC2_InvalidRawStruct(t *testing.T) {
	res := resource.Resource{ID: "task-abc", RawStruct: "not-a-task"}

	checker := ecsTaskCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (invalid RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSTaskECR — ECR repo names from container image URIs
// ---------------------------------------------------------------------------

func TestRelated_ECSTask_ECR_MatchSingleRepo(t *testing.T) {
	task := ecstypes.Task{
		Containers: []ecstypes.Container{
			{Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:v1.2.3")},
		},
	}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-app" {
		t.Errorf("ResourceIDs = %v, want [my-app]", result.ResourceIDs)
	}
}

func TestRelated_ECSTask_ECR_DeduplicatesMultipleContainersSameRepo(t *testing.T) {
	task := ecstypes.Task{
		Containers: []ecstypes.Container{
			{Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:v1")},
			{Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app:v2")},
		},
	}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (same repo, two tags — deduped)", result.Count)
	}
}

func TestRelated_ECSTask_ECR_SkipsNonECRImages(t *testing.T) {
	task := ecstypes.Task{
		Containers: []ecstypes.Container{
			{Image: aws.String("nginx:latest")},
			{Image: aws.String("docker.io/library/redis:7")},
		},
	}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no ECR images)", result.Count)
	}
}

func TestRelated_ECSTask_ECR_StripsTagFromRepo(t *testing.T) {
	task := ecstypes.Task{
		Containers: []ecstypes.Container{
			{Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/org/my-app:prod")},
		},
	}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "org/my-app" {
		t.Errorf("ResourceIDs = %v, want [org/my-app]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkECSTaskENI — ENI IDs from task attachments (awsvpc mode)
// ---------------------------------------------------------------------------

func TestRelated_ECSTask_ENI_MatchFromAttachment(t *testing.T) {
	task := ecstypes.Task{
		Attachments: []ecstypes.Attachment{
			{
				Type: aws.String("ElasticNetworkInterface"),
				Details: []ecstypes.KeyValuePair{
					{Name: aws.String("networkInterfaceId"), Value: aws.String("eni-0a1b2c3d4e5f67890")},
					{Name: aws.String("subnetId"), Value: aws.String("subnet-12345678")},
				},
			},
		},
	}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "eni-0a1b2c3d4e5f67890" {
		t.Errorf("ResourceIDs = %v, want [eni-0a1b2c3d4e5f67890]", result.ResourceIDs)
	}
}

func TestRelated_ECSTask_ENI_NoAttachments(t *testing.T) {
	task := ecstypes.Task{}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no attachments)", result.Count)
	}
}

func TestRelated_ECSTask_ENI_AttachmentWrongType(t *testing.T) {
	task := ecstypes.Task{
		Attachments: []ecstypes.Attachment{
			{
				Type: aws.String("ServiceConnect"),
				Details: []ecstypes.KeyValuePair{
					{Name: aws.String("networkInterfaceId"), Value: aws.String("eni-0a1b2c3d4e5f67890")},
				},
			},
		},
	}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong attachment type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSTaskSubnet — subnet IDs from task attachments (awsvpc mode)
// ---------------------------------------------------------------------------

func TestRelated_ECSTask_Subnet_MatchFromAttachment(t *testing.T) {
	task := ecstypes.Task{
		Attachments: []ecstypes.Attachment{
			{
				Type: aws.String("ElasticNetworkInterface"),
				Details: []ecstypes.KeyValuePair{
					{Name: aws.String("subnetId"), Value: aws.String("subnet-0a1b2c3d")},
					{Name: aws.String("networkInterfaceId"), Value: aws.String("eni-xyz")},
				},
			},
		},
	}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "subnet-0a1b2c3d" {
		t.Errorf("ResourceIDs = %v, want [subnet-0a1b2c3d]", result.ResourceIDs)
	}
}

func TestRelated_ECSTask_Subnet_NoAttachments(t *testing.T) {
	task := ecstypes.Task{}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no attachments)", result.Count)
	}
}

func TestRelated_ECSTask_Subnet_InvalidRawStruct(t *testing.T) {
	res := resource.Resource{ID: "task-abc", RawStruct: "not-a-task"}

	checker := ecsTaskCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (invalid RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSTaskSG — always Count:0 for Task (SGs not in Task.Attachments)
// ---------------------------------------------------------------------------

// TestRelated_ECSTask_SG_AlwaysZeroForValidTask verifies that the checker
// returns Count:0 even for a task with attachments — SG IDs are on the service,
// not the task.
func TestRelated_ECSTask_SG_AlwaysZeroForValidTask(t *testing.T) {
	task := ecstypes.Task{
		Attachments: []ecstypes.Attachment{
			{
				Type: aws.String("ElasticNetworkInterface"),
				Details: []ecstypes.KeyValuePair{
					{Name: aws.String("subnetId"), Value: aws.String("subnet-0a1b2c3d")},
				},
			},
		},
	}
	res := resource.Resource{ID: "task-abc", RawStruct: task}

	checker := ecsTaskCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (SGs not in Task.Attachments)", result.Count)
	}
}

func TestRelated_ECSTask_SG_InvalidRawStruct(t *testing.T) {
	res := resource.Resource{ID: "task-abc", RawStruct: "not-a-task"}

	checker := ecsTaskCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (invalid RawStruct)", result.Count)
	}
}
