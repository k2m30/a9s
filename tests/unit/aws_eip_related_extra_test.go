package unit_test

// aws_eip_related_extra_test.go — additional coverage for eip_related.go
// Covers: checkEIPCFN, checkEIPAlarm, checkEIPASG, checkEIPECS/ECSSvc/ECSTask/Logs.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- checkEIPCFN (Pattern F — reads aws:cloudformation:stack-name tag) ---

func TestRelated_EIP_CFN_Found(t *testing.T) {
	source := resource.Resource{
		ID: "eipalloc-0a1b2c3d4e5f60001",
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-0a1b2c3d4e5f60001"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("prod-network-stack")},
				{Key: aws.String("Name"), Value: aws.String("nat-eip")},
			},
		},
	}
	checker := eipCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "prod-network-stack" {
		t.Errorf("ResourceIDs[0] = %q, want prod-network-stack", result.ResourceIDs[0])
	}
}

func TestRelated_EIP_CFN_NoStackTag(t *testing.T) {
	source := resource.Resource{
		ID: "eipalloc-0a1b2c3d4e5f60001",
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-0a1b2c3d4e5f60001"),
			Tags: []ec2types.Tag{
				{Key: aws.String("Name"), Value: aws.String("nat-eip")},
			},
		},
	}
	checker := eipCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no CFN tag)", result.Count)
	}
}

func TestRelated_EIP_CFN_NoTags(t *testing.T) {
	source := resource.Resource{
		ID:        "eipalloc-0a1b2c3d4e5f60001",
		RawStruct: ec2types.Address{AllocationId: aws.String("eipalloc-0a1b2c3d4e5f60001"), Tags: nil},
	}
	checker := eipCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no tags)", result.Count)
	}
}

func TestRelated_EIP_CFN_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "eipalloc-0a1b2c3d4e5f60001", RawStruct: "not-an-address"}
	checker := eipCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkEIPAlarm (Pattern C — reverse lookup by InstanceId/NetworkInterfaceId dimension) ---

func TestRelated_EIP_Alarm_MatchByInstanceId(t *testing.T) {
	source := resource.Resource{
		ID: "eipalloc-001",
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-001"),
			InstanceId:   aws.String("i-0abc1234567890def"),
		},
	}
	alarmRes := resource.Resource{
		ID: "instance-cpu-high",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("instance-cpu-high"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("InstanceId"), Value: aws.String("i-0abc1234567890def")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := eipCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "instance-cpu-high" {
		t.Errorf("ResourceIDs[0] = %q, want instance-cpu-high", result.ResourceIDs[0])
	}
}

func TestRelated_EIP_Alarm_MatchByNetworkInterfaceId(t *testing.T) {
	source := resource.Resource{
		ID: "eipalloc-002",
		RawStruct: ec2types.Address{
			AllocationId:       aws.String("eipalloc-002"),
			NetworkInterfaceId: aws.String("eni-0deadbeefcafe0001"),
		},
	}
	alarmRes := resource.Resource{
		ID: "eni-bandwidth-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("eni-bandwidth-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("NetworkInterfaceId"), Value: aws.String("eni-0deadbeefcafe0001")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := eipCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_EIP_Alarm_NoAttachmentReturnsZero(t *testing.T) {
	// EIP with no attached instance or ENI → no relevant alarms.
	source := resource.Resource{
		ID:        "eipalloc-003",
		RawStruct: ec2types.Address{AllocationId: aws.String("eipalloc-003")},
	}
	checker := eipCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (not attached)", result.Count)
	}
}

func TestRelated_EIP_Alarm_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "eipalloc-004", RawStruct: "bad"}
	checker := eipCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkEIPASG (Pattern C — looks up EC2 tag aws:autoscaling:groupName) ---

func TestRelated_EIP_ASG_FoundViaInstanceTag(t *testing.T) {
	source := resource.Resource{
		ID: "eipalloc-005",
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-005"),
			InstanceId:   aws.String("i-0asg1234567890abc"),
		},
	}
	ec2Res := resource.Resource{
		ID: "i-0asg1234567890abc",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-0asg1234567890abc"),
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:autoscaling:groupName"), Value: aws.String("prod-app-asg")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}

	checker := eipCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "prod-app-asg" {
		t.Errorf("ResourceIDs[0] = %q, want prod-app-asg", result.ResourceIDs[0])
	}
}

func TestRelated_EIP_ASG_NoInstanceIDReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:        "eipalloc-006",
		RawStruct: ec2types.Address{AllocationId: aws.String("eipalloc-006")},
	}
	checker := eipCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no instance attached)", result.Count)
	}
}

func TestRelated_EIP_ASG_InstanceNotInASGReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID: "eipalloc-007",
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-007"),
			InstanceId:   aws.String("i-standalone"),
		},
	}
	ec2Res := resource.Resource{
		ID: "i-standalone",
		RawStruct: ec2types.Instance{
			InstanceId: aws.String("i-standalone"),
			Tags:       []ec2types.Tag{{Key: aws.String("Name"), Value: aws.String("standalone")}},
		},
	}
	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{ec2Res}},
	}

	checker := eipCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (instance has no asg tag)", result.Count)
	}
}

func TestRelated_EIP_ASG_NilCacheNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "eipalloc-008",
		RawStruct: ec2types.Address{
			AllocationId: aws.String("eipalloc-008"),
			InstanceId:   aws.String("i-0abc1234567890def"),
		},
	}
	checker := eipCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkEIPECS / checkEIPECSSvc / checkEIPECSTask / checkEIPLogs
// Each returns Count:-1 for a non-empty ID (outside 1-call budget) and
// Count:0 for an empty ID (no EIP → no association possible).
// ---------------------------------------------------------------------------

func TestRelated_EIP_ECS_EmptyIDReturnsZero(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := eipCheckerByTarget(t, "ecs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty EIP ID)", result.Count)
	}
	if result.TargetType != "ecs" {
		t.Errorf("TargetType = %q, want ecs", result.TargetType)
	}
}

func TestRelated_EIP_ECS_NonEmptyIDReturnsMinusOne(t *testing.T) {
	source := resource.Resource{ID: "eipalloc-0a1b2c3d4e5f60001"}
	checker := eipCheckerByTarget(t, "ecs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (outside 1-call budget)", result.Count)
	}
}

func TestRelated_EIP_ECSSvc_EmptyIDReturnsZero(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := eipCheckerByTarget(t, "ecs-svc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty EIP ID)", result.Count)
	}
}

func TestRelated_EIP_ECSTask_EmptyIDReturnsZero(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := eipCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty EIP ID)", result.Count)
	}
}

func TestRelated_EIP_Logs_EmptyIDReturnsZero(t *testing.T) {
	source := resource.Resource{ID: ""}
	checker := eipCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty EIP ID)", result.Count)
	}
}

func TestRelated_EIP_Logs_NonEmptyIDReturnsMinusOne(t *testing.T) {
	source := resource.Resource{ID: "eipalloc-0a1b2c3d4e5f60001"}
	checker := eipCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (outside 1-call budget)", result.Count)
	}
}
