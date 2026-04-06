package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func alarmCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("alarm") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("alarm related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("alarm related checker for %s not found", target)
	return nil
}

// --- SNS Checker Tests ---

func TestRelated_Alarm_SNS_Found(t *testing.T) {
	snsARN := "arn:aws:sns:us-east-1:123456789012:my-topic"
	raw := cwtypes.MetricAlarm{
		AlarmActions: []string{snsARN},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != snsARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, snsARN)
	}
}

func TestRelated_Alarm_SNS_OKActions(t *testing.T) {
	snsARN := "arn:aws:sns:us-east-1:123456789012:ok-topic"
	raw := cwtypes.MetricAlarm{
		OKActions: []string{snsARN},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_Alarm_SNS_InsufficientDataActions(t *testing.T) {
	snsARN := "arn:aws:sns:us-east-1:123456789012:insufficient-topic"
	raw := cwtypes.MetricAlarm{
		InsufficientDataActions: []string{snsARN},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_Alarm_SNS_FiltersNonSNS(t *testing.T) {
	// Lambda ARN in actions should not be counted
	lambdaARN := "arn:aws:lambda:us-east-1:123456789012:function:my-func"
	raw := cwtypes.MetricAlarm{
		AlarmActions: []string{lambdaARN},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (Lambda ARN should not match SNS)", result.Count)
	}
}

func TestRelated_Alarm_SNS_Deduplicates(t *testing.T) {
	snsARN := "arn:aws:sns:us-east-1:123456789012:shared-topic"
	raw := cwtypes.MetricAlarm{
		AlarmActions:            []string{snsARN},
		OKActions:               []string{snsARN},
		InsufficientDataActions: []string{snsARN},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (same ARN in all actions should deduplicate)", result.Count)
	}
}

func TestRelated_Alarm_SNS_NoActions(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Alarm_SNS_InvalidRawStruct(t *testing.T) {
	res := resource.Resource{ID: "test-alarm", RawStruct: "not-a-metric-alarm"}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 for invalid RawStruct", result.Count)
	}
}

// --- ASG Checker Tests ---

func TestRelated_Alarm_ASG_MatchByDimension(t *testing.T) {
	// Alarm has AutoScalingGroupName dimension pointing to "my-asg"
	// ASG cache has a resource with ID "my-asg"
	// → Count: 1
	raw := cwtypes.MetricAlarm{
		Dimensions: []cwtypes.Dimension{
			{
				Name:  aws.String("AutoScalingGroupName"),
				Value: aws.String("my-asg"),
			},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-asg"},
		}},
	}

	checker := alarmCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

func TestRelated_Alarm_ASG_NoMatch(t *testing.T) {
	// Alarm has dimension, but ASG cache has different name
	// → Count: 0
	raw := cwtypes.MetricAlarm{
		Dimensions: []cwtypes.Dimension{
			{
				Name:  aws.String("AutoScalingGroupName"),
				Value: aws.String("my-asg"),
			},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "other-asg"},
		}},
	}

	checker := alarmCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_Alarm_ASG_NoDimension(t *testing.T) {
	// Alarm has no AutoScalingGroupName dimension
	// → Count: 0
	raw := cwtypes.MetricAlarm{
		Dimensions: []cwtypes.Dimension{
			{
				Name:  aws.String("FunctionName"),
				Value: aws.String("my-lambda"),
			},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-asg"},
		}},
	}

	checker := alarmCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no AutoScalingGroupName dimension)", result.Count)
	}
}

func TestRelated_Alarm_ASG_NilCache(t *testing.T) {
	// Empty cache
	// → Count: -1
	raw := cwtypes.MetricAlarm{
		Dimensions: []cwtypes.Dimension{
			{
				Name:  aws.String("AutoScalingGroupName"),
				Value: aws.String("my-asg"),
			},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}
	cache := resource.ResourceCache{}

	checker := alarmCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count)
	}
}

// --- Demo Checker Test ---

func TestRelatedDemo_Alarm_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("alarm")
	if checker == nil {
		t.Fatal("no demo checker registered for alarm")
	}
	results := checker(resource.Resource{ID: "demo-alarm"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
