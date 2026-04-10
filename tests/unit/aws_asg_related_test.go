package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestRelated_ASG_Registered verifies all 5 related defs are registered with correct checker presence.
func TestRelated_ASG_Registered(t *testing.T) {
	defs := resource.GetRelated("asg")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for asg")
	}

	checkerExpected := map[string]bool{
		"ec2":    true,  // non-nil
		"tg":     true,  // non-nil
		"subnet": true,  // non-nil
		"alarm":  true,  // non-nil
		"ng":     true,  // non-nil
	}
	for target, wantChecker := range checkerExpected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				hasChecker := def.Checker != nil
				if hasChecker != wantChecker {
					t.Errorf("asg %q: Checker presence = %v, want %v", target, hasChecker, wantChecker)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

func asgCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("asg") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("asg related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("asg related checker for %s not found", target)
	return nil
}

// --- checkAsgAlarm tests (Pattern D — dimension-based) ---

func TestRelated_ASG_Alarm_MatchByDimension(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "asg-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("asg-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("AutoScalingGroupName"),
					Value: aws.String("my-asg"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "asg-cpu-alarm" {
		t.Errorf("ResourceIDs = %v, want [asg-cpu-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ASG_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "asg-other-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("asg-other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("AutoScalingGroupName"),
					Value: aws.String("other-asg"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ASG_Alarm_EmptyID(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "asg-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("asg-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("AutoScalingGroupName"),
					Value: aws.String("my-asg"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_ASG_Alarm_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- checkAsgNG tests (Pattern C — target-cache match by ASG name) ---

func TestRelated_ASG_NG_MatchByASGName(t *testing.T) {
	ngRes := resource.Resource{
		ID:     "my-node-group",
		Fields: map[string]string{},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("my-node-group"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("my-asg")},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-node-group" {
		t.Errorf("ResourceIDs = %v, want [my-node-group]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ASG_NG_NoMatch(t *testing.T) {
	ngRes := resource.Resource{
		ID:     "other-node-group",
		Fields: map[string]string{},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("other-node-group"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("other-asg")},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ASG_NG_EmptyID(t *testing.T) {
	ngRes := resource.Resource{
		ID:     "my-node-group",
		Fields: map[string]string{},
		RawStruct: ekstypes.Nodegroup{
			NodegroupName: aws.String("my-node-group"),
			Resources: &ekstypes.NodegroupResources{
				AutoScalingGroups: []ekstypes.AutoScalingGroup{
					{Name: aws.String("my-asg")},
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"ng": resource.ResourceCacheEntry{Resources: []resource.Resource{ngRes}},
	}

	res := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_ASG_NG_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
	}

	checker := asgCheckerByTarget(t, "ng")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkASGEC2 tests (Pattern F — no cache, reads Instances[] from RawStruct)
// ---------------------------------------------------------------------------

// TestRelated_ASG_EC2_MatchByInstances verifies that checkASGEC2 returns the
// instance IDs from the ASG Instances slice.
func TestRelated_ASG_EC2_MatchByInstances(t *testing.T) {
	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			Instances: []asgtypes.Instance{
				{InstanceId: aws.String("i-0abc111111111111a"), AvailabilityZone: aws.String("us-east-1a"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
				{InstanceId: aws.String("i-0bbb222222222222b"), AvailabilityZone: aws.String("us-east-1b"), HealthStatus: aws.String("Healthy"), LifecycleState: asgtypes.LifecycleStateInService},
			},
		},
	}

	checker := asgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Fatalf("ResourceIDs length = %d, want 2; got %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	if result.ResourceIDs[0] != "i-0abc111111111111a" || result.ResourceIDs[1] != "i-0bbb222222222222b" {
		t.Errorf("ResourceIDs = %v, want [i-0abc111111111111a, i-0bbb222222222222b]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_EC2_NoInstances verifies that checkASGEC2 returns Count=0 when
// the ASG has no instances.
func TestRelated_ASG_EC2_NoInstances(t *testing.T) {
	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			Instances:            []asgtypes.Instance{},
		},
	}

	checker := asgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no instances)", result.Count)
	}
}

// TestRelated_ASG_EC2_NoRawStruct verifies that checkASGEC2 returns Count=-1 when
// the resource has no RawStruct (cannot extract instance data).
func TestRelated_ASG_EC2_NoRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		Fields:    map[string]string{},
		RawStruct: nil,
	}

	checker := asgCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no RawStruct)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkASGTG tests (Pattern C — cache match by TargetGroupARN)
// ---------------------------------------------------------------------------

// TestRelated_ASG_TG_MatchByARN verifies that checkASGTG returns Count=1 when the
// ASG's TargetGroupARNs contains an ARN matching a target group in the cache.
func TestRelated_ASG_TG_MatchByARN(t *testing.T) {
	tgARN := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-tg/abcdef123456"
	tgRes := resource.Resource{
		ID:     "my-tg",
		Fields: map[string]string{},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn:  aws.String(tgARN),
			TargetGroupName: aws.String("my-tg"),
		},
	}
	cache := resource.ResourceCache{
		"tg": resource.ResourceCacheEntry{Resources: []resource.Resource{tgRes}},
	}

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			TargetGroupARNs:      []string{tgARN},
		},
	}

	checker := asgCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (TG matched by ARN)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-tg" {
		t.Errorf("ResourceIDs = %v, want [my-tg]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_TG_NoTargetGroups verifies that checkASGTG returns Count=0 when
// the ASG has no TargetGroupARNs.
func TestRelated_ASG_TG_NoTargetGroups(t *testing.T) {
	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			TargetGroupARNs:      []string{},
		},
	}

	checker := asgCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no TargetGroupARNs)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkASGSubnets tests (Pattern F — no cache, parses VPCZoneIdentifier)
// ---------------------------------------------------------------------------

// TestRelated_ASG_Subnets_ParsesMultiple verifies that checkASGSubnets correctly
// parses a comma-separated VPCZoneIdentifier into individual subnet IDs.
func TestRelated_ASG_Subnets_ParsesMultiple(t *testing.T) {
	subnetA := "subnet-0aaa111111111111a"
	subnetB := "subnet-0bbb222222222222b"
	subnetC := "subnet-0ccc333333333333c"

	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			VPCZoneIdentifier:    aws.String(subnetA + "," + subnetB + "," + subnetC),
		},
	}

	checker := asgCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 3 {
		t.Errorf("Count = %d, want 3 (3 subnets)", result.Count)
	}
	if len(result.ResourceIDs) != 3 {
		t.Fatalf("ResourceIDs length = %d, want 3; got %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ASG_Subnets_EmptyIdentifier verifies that checkASGSubnets returns
// Count=0 when VPCZoneIdentifier is empty.
func TestRelated_ASG_Subnets_EmptyIdentifier(t *testing.T) {
	res := resource.Resource{
		ID:     "my-asg",
		Fields: map[string]string{},
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			VPCZoneIdentifier:    aws.String(""),
		},
	}

	checker := asgCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty VPCZoneIdentifier)", result.Count)
	}
}

// TestRelated_ASG_Subnets_NoRawStruct verifies that checkASGSubnets returns
// Count=-1 when the resource has no RawStruct.
func TestRelated_ASG_Subnets_NoRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		Fields:    map[string]string{},
		RawStruct: nil,
	}

	checker := asgCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no RawStruct)", result.Count)
	}
}
