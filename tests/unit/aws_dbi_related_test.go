package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestRelated_DBI_Registered(t *testing.T) {
	defs := resource.GetRelated("dbi")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for dbi")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"sg":       {"Security Groups", true},
		"kms":      {"KMS Key", true},
		"subnet":   {"Subnets", true},
		"alarm":    {"CloudWatch Alarms", true},
		"rds-snap": {"RDS Snapshots", true},
		"secrets":  {"Secrets Manager", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("dbi %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("dbi %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("dbi %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// dbiCheckerByTarget returns the RelatedChecker for the given target type registered
// under "dbi". It fails the test immediately if the checker is nil or not found.
func dbiCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("dbi") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("dbi related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("dbi related checker for %s not found", target)
	return nil
}

// --- checkDbiAlarm tests (Pattern D — dimension-based) ---

func TestRelated_DBI_Alarm_MatchByDimension(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "rds-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("rds-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("DBInstanceIdentifier"),
					Value: aws.String("my-db-instance"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{},
	}

	checker := dbiCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "rds-cpu-alarm" {
		t.Errorf("ResourceIDs = %v, want [rds-cpu-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_DBI_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "rds-other-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("rds-other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("DBInstanceIdentifier"),
					Value: aws.String("other-db-instance"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{},
	}

	checker := dbiCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_DBI_Alarm_EmptyID(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "rds-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("rds-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("DBInstanceIdentifier"),
					Value: aws.String("my-db-instance"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:        "",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{},
	}

	checker := dbiCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_DBI_Alarm_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{},
	}

	checker := dbiCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- checkDbiRDSSnap tests (Pattern C — target-cache match by DBInstanceIdentifier) ---

func TestRelated_DBI_RDSSnap_MatchByDBIdentifier(t *testing.T) {
	snapRes := resource.Resource{
		ID:     "rds:snapshot:my-db-instance-snap",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBSnapshot{
			DBInstanceIdentifier: aws.String("my-db-instance"),
		},
	}
	cache := resource.ResourceCache{
		"rds-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{snapRes}},
	}

	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{},
	}

	checker := dbiCheckerByTarget(t, "rds-snap")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "rds:snapshot:my-db-instance-snap" {
		t.Errorf("ResourceIDs = %v, want [rds:snapshot:my-db-instance-snap]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_DBI_RDSSnap_NoMatch(t *testing.T) {
	snapRes := resource.Resource{
		ID:     "rds:snapshot:other-db-snap",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBSnapshot{
			DBInstanceIdentifier: aws.String("other-db-instance"),
		},
	}
	cache := resource.ResourceCache{
		"rds-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{snapRes}},
	}

	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{},
	}

	checker := dbiCheckerByTarget(t, "rds-snap")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_DBI_RDSSnap_EmptyID(t *testing.T) {
	snapRes := resource.Resource{
		ID:     "rds:snapshot:my-db-instance-snap",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBSnapshot{
			DBInstanceIdentifier: aws.String("my-db-instance"),
		},
	}
	cache := resource.ResourceCache{
		"rds-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{snapRes}},
	}

	res := resource.Resource{
		ID:        "",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{},
	}

	checker := dbiCheckerByTarget(t, "rds-snap")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_DBI_RDSSnap_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{},
	}

	checker := dbiCheckerByTarget(t, "rds-snap")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- dbi→secrets: undeterminable from cache, returns Count: 0 ---

func TestRelated_DBI_Secrets_ReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:   "my-db-instance",
		Name: "my-db-instance",
	}
	checker := dbiCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (undeterminable from cache)", result.Count)
	}
	if result.TargetType != "secrets" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "secrets")
	}
}

func TestRelatedDemo_DBI_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("dbi")
	if checker == nil {
		t.Fatal("no demo checker registered for dbi")
	}

	results := checker(resource.Resource{ID: "demo-db"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
