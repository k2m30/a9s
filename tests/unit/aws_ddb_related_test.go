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

func TestRelated_DDB_Registered(t *testing.T) {
	defs := resource.GetRelated("ddb")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for ddb")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"kms":    {"KMS Key", true},
		"lambda": {"Lambda Functions", true},
		"alarm":  {"CloudWatch Alarms", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("ddb %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("ddb %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("ddb %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// ddbCheckerByTarget returns the RelatedChecker for the given target type registered
// under "ddb". It fails the test immediately if the checker is nil or not found.
func ddbCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ddb") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ddb related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ddb related checker for %s not found", target)
	return nil
}

// --- checkDdbAlarm tests (Pattern D — dimension-based) ---

func TestRelated_DDB_Alarm_MatchByDimension(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "ddb-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("ddb-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("TableName"),
					Value: aws.String("my-table"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "my-table",
		Fields: map[string]string{},
	}

	checker := ddbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "ddb-cpu-alarm" {
		t.Errorf("ResourceIDs = %v, want [ddb-cpu-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_DDB_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "ddb-other-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("ddb-other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("TableName"),
					Value: aws.String("other-table"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "my-table",
		Fields: map[string]string{},
	}

	checker := ddbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_DDB_Alarm_EmptyID(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "ddb-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("ddb-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("TableName"),
					Value: aws.String("my-table"),
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

	checker := ddbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_DDB_Alarm_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "my-table",
		Fields: map[string]string{},
	}

	checker := ddbCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- ddb→lambda: undeterminable from cache, returns Count: 0 ---

func TestRelated_DDB_Lambda_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "acme-orders-table",
		Name: "acme-orders-table",
	}
	checker := ddbCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "lambda" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "lambda")
	}
}

func TestRelatedDemo_DDB_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("ddb")
	if checker == nil {
		t.Fatal("no demo checker registered for ddb")
	}

	results := checker(resource.Resource{ID: "acme-orders-prod"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
