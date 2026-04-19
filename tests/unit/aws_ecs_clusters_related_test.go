package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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

// ---------------------------------------------------------------------------
// checkECSASG — ASG with AmazonECSManaged or ClusterName tag
// ---------------------------------------------------------------------------

func TestRelated_ECS_ASG_MatchByAmazonECSManagedTag(t *testing.T) {
	tagKey := "AmazonECSManaged"
	tagVal := "true"
	asgRes := resource.Resource{
		ID: "ecs-asg-managed",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("ecs-asg-managed"),
			Tags: []asgtypes.TagDescription{
				{Key: &tagKey, Value: &tagVal},
			},
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "ecs-asg-managed" {
		t.Errorf("ResourceIDs = %v, want [ecs-asg-managed]", result.ResourceIDs)
	}
}

func TestRelated_ECS_ASG_MatchByClusterNameTag(t *testing.T) {
	tagKey := "ClusterName"
	tagVal := "my-cluster"
	asgRes := resource.Resource{
		ID: "ecs-asg-cluster",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("ecs-asg-cluster"),
			Tags: []asgtypes.TagDescription{
				{Key: &tagKey, Value: &tagVal},
			},
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_ECS_ASG_NoMatchWhenDifferentCluster(t *testing.T) {
	tagKey := "ClusterName"
	tagVal := "other-cluster"
	asgRes := resource.Resource{
		ID: "other-asg",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("other-asg"),
			Tags: []asgtypes.TagDescription{
				{Key: &tagKey, Value: &tagVal},
			},
		},
	}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{asgRes}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (different cluster)", result.Count)
	}
}

func TestRelated_ECS_ASG_EmptySourceID(t *testing.T) {
	checker := ecsCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, resource.Resource{ID: ""}, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cluster ID)", result.Count)
	}
}

func TestRelated_ECS_ASG_NilCache(t *testing.T) {
	checker := ecsCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, resource.Resource{ID: "my-cluster"}, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSEC2 — EC2 instances tagged "aws:ecs:cluster-name" or "ClusterName"
// ---------------------------------------------------------------------------

func TestRelated_ECS_EC2_MatchByECSClusterNameTag(t *testing.T) {
	ec2Res := resource.Resource{
		ID: "i-0a1b2c3d4e5f67890",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0a1b2c3d4e5f67890"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:ecs:cluster-name"), Value: aws.String("my-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "i-0a1b2c3d4e5f67890" {
		t.Errorf("ResourceIDs = %v, want [i-0a1b2c3d4e5f67890]", result.ResourceIDs)
	}
}

func TestRelated_ECS_EC2_MatchByClusterNameTag(t *testing.T) {
	ec2Res := resource.Resource{
		ID: "i-0a1b2c3d4e5f67890",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0a1b2c3d4e5f67890"),
			Tags: []ec2types.Tag{
				{Key: aws.String("ClusterName"), Value: aws.String("my-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_ECS_EC2_NoMatchDifferentCluster(t *testing.T) {
	ec2Res := resource.Resource{
		ID: "i-0a1b2c3d4e5f67890",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0a1b2c3d4e5f67890"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:ecs:cluster-name"), Value: aws.String("other-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (different cluster)", result.Count)
	}
}

func TestRelated_ECS_EC2_EmptySourceID(t *testing.T) {
	checker := ecsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, resource.Resource{ID: ""}, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cluster ID)", result.Count)
	}
}

func TestRelated_ECS_EC2_NilCache(t *testing.T) {
	checker := ecsCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, resource.Resource{ID: "my-cluster"}, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSCTEvents — CloudTrail events that reference this cluster
// ---------------------------------------------------------------------------

func TestRelated_ECS_CTEvents_Match(t *testing.T) {
	clusterName := "my-cluster"
	evRes := resource.Resource{
		ID: "event-abc123",
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster")},
			},
		},
	}
	otherEv := resource.Resource{
		ID: "event-def456",
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/other-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes, otherEv}},
	}
	source := resource.Resource{ID: clusterName, Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "event-abc123" {
		t.Errorf("ResourceIDs = %v, want [event-abc123]", result.ResourceIDs)
	}
}

func TestRelated_ECS_CTEvents_NoMatch(t *testing.T) {
	evRes := resource.Resource{
		ID: "event-abc123",
		RawStruct: cloudtrailtypes.Event{
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/other-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECS_CTEvents_EmptySourceID(t *testing.T) {
	checker := ecsCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, resource.Resource{ID: ""}, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cluster ID)", result.Count)
	}
}

func TestRelated_ECS_CTEvents_NilCache(t *testing.T) {
	checker := ecsCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, resource.Resource{ID: "my-cluster"}, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSTasks — ECS tasks whose ClusterArn refers to this cluster
// ---------------------------------------------------------------------------

func TestRelated_ECS_Tasks_MatchByExactClusterName(t *testing.T) {
	clusterName := "my-cluster"
	taskRes := resource.Resource{
		ID: "task-abc123def456",
		RawStruct: ecstypes.Task{
			ClusterArn: aws.String("my-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{taskRes}},
	}
	source := resource.Resource{ID: clusterName, Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_ECS_Tasks_MatchByARNSuffix(t *testing.T) {
	clusterName := "my-cluster"
	taskRes := resource.Resource{
		ID: "task-abc123def456",
		RawStruct: ecstypes.Task{
			ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{taskRes}},
	}
	source := resource.Resource{ID: clusterName, Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (ARN suffix match)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "task-abc123def456" {
		t.Errorf("ResourceIDs = %v, want [task-abc123def456]", result.ResourceIDs)
	}
}

func TestRelated_ECS_Tasks_NoMatchDifferentCluster(t *testing.T) {
	taskRes := resource.Resource{
		ID: "task-abc123def456",
		RawStruct: ecstypes.Task{
			ClusterArn: aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/other-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{taskRes}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECS_Tasks_EmptySourceID(t *testing.T) {
	checker := ecsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, resource.Resource{ID: ""}, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cluster ID)", result.Count)
	}
}

func TestRelated_ECS_Tasks_NilCache(t *testing.T) {
	checker := ecsCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, resource.Resource{ID: "my-cluster"}, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSLogs — log groups whose ID contains the cluster name as substring
// ---------------------------------------------------------------------------

func TestRelated_ECS_Logs_Match(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/ecs/my-cluster/app",
		Fields: map[string]string{},
	}
	otherLog := resource.Resource{
		ID:     "/ecs/other-service/worker",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes, otherLog}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/ecs/my-cluster/app" {
		t.Errorf("ResourceIDs = %v, want [/ecs/my-cluster/app]", result.ResourceIDs)
	}
}

func TestRelated_ECS_Logs_NoMatch(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/ecs/other-service/worker",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}
	source := resource.Resource{ID: "my-cluster", Fields: map[string]string{}}

	checker := ecsCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECS_Logs_EmptySourceID(t *testing.T) {
	checker := ecsCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, resource.Resource{ID: ""}, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cluster ID)", result.Count)
	}
}

func TestRelated_ECS_Logs_NilCache(t *testing.T) {
	checker := ecsCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, resource.Resource{ID: "my-cluster"}, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkECSKMS — KmsKeyId from ExecuteCommandConfiguration (Pattern F)
// ---------------------------------------------------------------------------

// TestRelated_ECS_KMS_FoundFullARN verifies that a cluster with a full KMS ARN in
// Configuration.ExecuteCommandConfiguration.KmsKeyId returns the last segment as ID.
func TestRelated_ECS_KMS_FoundFullARN(t *testing.T) {
	source := resource.Resource{
		ID:   "my-cluster",
		Name: "my-cluster",
		RawStruct: ecstypes.Cluster{
			ClusterName: aws.String("my-cluster"),
			Configuration: &ecstypes.ClusterConfiguration{
				ExecuteCommandConfiguration: &ecstypes.ExecuteCommandConfiguration{
					KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/abcd1234-5678-90ab-cdef-111111111111"),
				},
			},
		},
	}

	checker := ecsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abcd1234-5678-90ab-cdef-111111111111" {
		t.Errorf("ResourceIDs = %v, want [abcd1234-5678-90ab-cdef-111111111111]", result.ResourceIDs)
	}
}

// TestRelated_ECS_KMS_PlainKeyID verifies that a plain key ID (no "/" separator) is
// returned unchanged.
func TestRelated_ECS_KMS_PlainKeyID(t *testing.T) {
	source := resource.Resource{
		ID:   "my-cluster",
		Name: "my-cluster",
		RawStruct: ecstypes.Cluster{
			ClusterName: aws.String("my-cluster"),
			Configuration: &ecstypes.ClusterConfiguration{
				ExecuteCommandConfiguration: &ecstypes.ExecuteCommandConfiguration{
					KmsKeyId: aws.String("abcd1234-5678-90ab-cdef-111111111111"),
				},
			},
		},
	}

	checker := ecsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abcd1234-5678-90ab-cdef-111111111111" {
		t.Errorf("ResourceIDs = %v, want [abcd1234-5678-90ab-cdef-111111111111]", result.ResourceIDs)
	}
}

// TestRelated_ECS_KMS_NilConfiguration verifies Count=0 when Configuration is nil.
func TestRelated_ECS_KMS_NilConfiguration(t *testing.T) {
	source := resource.Resource{
		ID:   "my-cluster",
		Name: "my-cluster",
		RawStruct: ecstypes.Cluster{
			ClusterName:   aws.String("my-cluster"),
			Configuration: nil,
		},
	}

	checker := ecsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Configuration)", result.Count)
	}
}

// TestRelated_ECS_KMS_EmptyKeyID verifies Count=0 when KmsKeyId is an empty string.
func TestRelated_ECS_KMS_EmptyKeyID(t *testing.T) {
	source := resource.Resource{
		ID:   "my-cluster",
		Name: "my-cluster",
		RawStruct: ecstypes.Cluster{
			ClusterName: aws.String("my-cluster"),
			Configuration: &ecstypes.ClusterConfiguration{
				ExecuteCommandConfiguration: &ecstypes.ExecuteCommandConfiguration{
					KmsKeyId: aws.String(""),
				},
			},
		},
	}

	checker := ecsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty KmsKeyId)", result.Count)
	}
}

// TestRelated_ECS_KMS_WrongRawStruct verifies Count=0 when RawStruct is not an ECS Cluster.
func TestRelated_ECS_KMS_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "my-cluster",
		RawStruct: "not-a-cluster",
	}

	checker := ecsCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct)", result.Count)
	}
}
