package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

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
