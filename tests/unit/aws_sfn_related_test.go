package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sfnCheckerByTarget returns the RelatedChecker for the given target type registered
// under "sfn". It fails the test immediately if the checker is nil or not found.
func sfnCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("sfn") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("sfn related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("sfn related checker for %s not found", target)
	return nil
}

// --- checkSFNLogs tests (Pattern N — naming convention) ---

func TestRelated_SFN_Logs_Found(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/vendedlogs/states/order-fulfillment-workflow",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	src := resource.Resource{ID: "order-fulfillment-workflow"}
	checker := sfnCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/vendedlogs/states/order-fulfillment-workflow" {
		t.Errorf("ResourceIDs = %v, want [/aws/vendedlogs/states/order-fulfillment-workflow]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_SFN_Logs_NoMatch(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/vendedlogs/states/other-workflow",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	src := resource.Resource{ID: "order-fulfillment-workflow"}
	checker := sfnCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_SFN_Logs_EmptyID(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/vendedlogs/states/order-fulfillment-workflow",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	src := resource.Resource{ID: ""}
	checker := sfnCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_SFN_Logs_CacheMissNoClients(t *testing.T) {
	cache := resource.ResourceCache{}

	src := resource.Resource{ID: "order-fulfillment-workflow"}
	checker := sfnCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- checkSFNAlarm tests (Pattern D — dimension-based) ---

func sfnSrcResource() resource.Resource {
	return resource.Resource{
		ID: "order-fulfillment-workflow",
		Fields: map[string]string{
			"arn": "arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow",
		},
	}
}

func TestRelated_SFN_Alarm_Found(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "sfn-failures",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("sfn-failures"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("StateMachineArn"),
					Value: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := sfnCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, sfnSrcResource(), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "sfn-failures" {
		t.Errorf("ResourceIDs = %v, want [sfn-failures]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_SFN_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "sfn-other-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("sfn-other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("StateMachineArn"),
					Value: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:other-workflow"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := sfnCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, sfnSrcResource(), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_SFN_Alarm_NoDimensions(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "sfn-nodim-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName:  aws.String("sfn-nodim-alarm"),
			Dimensions: []cwtypes.Dimension{},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	checker := sfnCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, sfnSrcResource(), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no dimensions)", result.Count)
	}
}

func TestRelated_SFN_Alarm_EmptyARN(t *testing.T) {
	src := resource.Resource{
		ID:     "order-fulfillment-workflow",
		Fields: map[string]string{"arn": ""},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "sfn-failures",
				RawStruct: cwtypes.MetricAlarm{
					AlarmName: aws.String("sfn-failures"),
					Dimensions: []cwtypes.Dimension{
						{
							Name:  aws.String("StateMachineArn"),
							Value: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow"),
						},
					},
				},
			},
		}},
	}

	checker := sfnCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty arn field)", result.Count)
	}
}

func TestRelated_SFN_Alarm_CacheMissNoClients(t *testing.T) {
	cache := resource.ResourceCache{}

	checker := sfnCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, sfnSrcResource(), cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- sfn→role: undeterminable from cache, returns Count: 0 ---

func TestRelated_SFN_Role_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "order-fulfillment-workflow",
		Name: "order-fulfillment-workflow",
	}
	checker := sfnCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
}

// --- sfn→cfn: undeterminable from cache, returns Count: 0 ---

func TestRelated_SFN_CFN_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "order-fulfillment-workflow",
		Name: "order-fulfillment-workflow",
	}
	checker := sfnCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "cfn" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "cfn")
	}
}
