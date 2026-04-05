package unit_test

import (
	"context"
	"testing"

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

// --- ASG Stub Test ---

func TestRelated_Alarm_ASG_IsStub(t *testing.T) {
	defs := resource.GetRelated("alarm")
	for _, def := range defs {
		if def.TargetType == "asg" {
			if def.Checker != nil {
				t.Error("alarm asg def: expected nil Checker (stub)")
			}
			return
		}
	}
	t.Error("alarm asg related def not found")
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
