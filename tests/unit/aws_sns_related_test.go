package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func snsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("sns") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("sns related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("sns related checker for %s not found", target)
	return nil
}

const (
	snsTopicARN     = "arn:aws:sns:us-east-1:123456789012:alarm-notifications"
	snsTopicARNOther = "arn:aws:sns:us-east-1:123456789012:other-topic"
)

func snsSrcResource() resource.Resource {
	return resource.Resource{
		ID:   "alarm-notifications",
		Fields: map[string]string{
			"topic_arn": snsTopicARN,
		},
	}
}

// --- Alarm checker tests (Pattern C — reverse lookup in alarm cache) ---

func TestRelated_SNS_Alarm_Found(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "test-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:    aws.String("test-alarm"),
			AlarmActions: []string{snsTopicARN},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := snsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, snsSrcResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "test-alarm" {
		t.Errorf("ResourceIDs = %v, want [test-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_SNS_Alarm_MultipleActions(t *testing.T) {
	// Alarm has the same topic ARN in both AlarmActions and OKActions — should still count=1 (same alarm).
	alarmRes := resource.Resource{
		ID:     "multi-action-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:    aws.String("multi-action-alarm"),
			AlarmActions: []string{snsTopicARN},
			OKActions:    []string{snsTopicARN},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := snsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, snsSrcResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (same alarm with topic in multiple action lists)", result.Count)
	}
}

func TestRelated_SNS_Alarm_MultipleAlarms(t *testing.T) {
	alarm1 := resource.Resource{
		ID:     "alarm-one",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:    aws.String("alarm-one"),
			AlarmActions: []string{snsTopicARN},
		},
	}
	alarm2 := resource.Resource{
		ID:     "alarm-two",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("alarm-two"),
			OKActions: []string{snsTopicARN},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarm1, alarm2}},
	}

	checker := snsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, snsSrcResource(), cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs len = %d, want 2: %v", len(result.ResourceIDs), result.ResourceIDs)
	}
}

func TestRelated_SNS_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "unrelated-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:    aws.String("unrelated-alarm"),
			AlarmActions: []string{snsTopicARNOther},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := snsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, snsSrcResource(), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_SNS_Alarm_EmptyARN(t *testing.T) {
	src := resource.Resource{
		ID:     "no-arn-topic",
		Fields: map[string]string{"topic_arn": ""},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:        "some-alarm",
				RawStruct: cwtypes.MetricAlarm{AlarmName: aws.String("some-alarm"), AlarmActions: []string{"arn:aws:sns:us-east-1:123456789012:some-topic"}},
			},
		}},
	}

	checker := snsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty topic_arn)", result.Count)
	}
}

func TestRelated_SNS_Alarm_EmptyCache(t *testing.T) {
	cache := resource.ResourceCache{}

	checker := snsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, snsSrcResource(), cache)

	// No clients, cache miss → -1 (unknown).
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- sns→cfn: undeterminable from cache, returns Count: 0 ---

func TestRelated_SNS_CFN_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "arn:aws:sns:us-east-1:111122223333:alarm-notifications",
		Name: "alarm-notifications",
	}
	checker := snsCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}

// --- InsufficientDataActions coverage ---

func TestRelated_SNS_Alarm_InsufficientDataActions(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "insufficient-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:               aws.String("insufficient-alarm"),
			InsufficientDataActions: []string{snsTopicARN},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := snsCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, snsSrcResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (InsufficientDataActions match)", result.Count)
	}
}

// --- Demo checker test ---

func TestRelatedDemo_SNS_Registered(t *testing.T) {
	_ = demo.GetResources // ensure demo package is initialized
	checker := resource.GetRelatedDemo("sns")
	if checker == nil {
		t.Fatal("no demo checker registered for sns")
	}

	src := resource.Resource{ID: "arn:aws:sns:us-east-1:123456789012:alarm-notifications"}
	results := checker(src)
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}

	// Verify all expected target types are present.
	wantTargets := map[string]bool{"alarm": false, "cfn": false}
	for _, r := range results {
		if _, ok := wantTargets[r.TargetType]; ok {
			wantTargets[r.TargetType] = true
		}
	}
	for target, found := range wantTargets {
		if !found {
			t.Errorf("demo checker missing result for target %q", target)
		}
	}

	// At least one result should have Count > 0.
	hasPositive := false
	for _, r := range results {
		if r.Count > 0 {
			hasPositive = true
			break
		}
	}
	if !hasPositive {
		t.Error("demo checker returned no result with Count > 0")
	}
}
