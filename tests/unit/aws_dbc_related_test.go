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

func TestRelated_DBC_Registered(t *testing.T) {
	defs := resource.GetRelated("dbc")
	if len(defs) == 0 {
		t.Fatal("no related defs registered for dbc")
	}

	type expectation struct {
		displayName string
		hasChecker  bool
	}
	expected := map[string]expectation{
		"sg":      {"Security Groups", true},
		"alarm":   {"CloudWatch Alarms", true},
		"secrets": {"Secrets Manager", false},
		"logs":    {"Log Groups", true},
	}
	for target, want := range expected {
		found := false
		for _, def := range defs {
			if def.TargetType == target {
				found = true
				if want.hasChecker && def.Checker == nil {
					t.Errorf("dbc %q: Checker should not be nil", target)
				}
				if !want.hasChecker && def.Checker != nil {
					t.Errorf("dbc %q: Checker should be nil (stub)", target)
				}
				if def.DisplayName != want.displayName {
					t.Errorf("dbc %q: DisplayName = %q, want %q", target, def.DisplayName, want.displayName)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected related def for target %q not found", target)
		}
	}
}

// dbcCheckerByTarget returns the RelatedChecker for the given target type registered
// under "dbc". It fails the test immediately if the checker is nil or not found.
func dbcCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("dbc") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("dbc related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("dbc related checker for %s not found", target)
	return nil
}

// --- checkDbcAlarm tests (Pattern D — dimension-based) ---

func TestRelated_DBC_Alarm_MatchByDimension(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "docdb-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("docdb-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("DBClusterIdentifier"),
					Value: aws.String("my-cluster"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "my-cluster",
		Fields: map[string]string{},
	}

	checker := dbcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "docdb-cpu-alarm" {
		t.Errorf("ResourceIDs = %v, want [docdb-cpu-alarm]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_DBC_Alarm_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "docdb-other-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("docdb-other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("DBClusterIdentifier"),
					Value: aws.String("different-cluster"),
				},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}

	res := resource.Resource{
		ID:     "my-cluster",
		Fields: map[string]string{},
	}

	checker := dbcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_DBC_Alarm_EmptyID(t *testing.T) {
	alarmRes := resource.Resource{
		ID:     "docdb-cpu-alarm",
		Fields: map[string]string{},
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("docdb-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{
					Name:  aws.String("DBClusterIdentifier"),
					Value: aws.String("my-cluster"),
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

	checker := dbcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_DBC_Alarm_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "my-cluster",
		Fields: map[string]string{},
	}

	checker := dbcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

// --- checkDbcLogs tests (Pattern N — naming convention) ---

func TestRelated_DBC_Logs_MatchByNamingConvention(t *testing.T) {
	auditLog := resource.Resource{
		ID:     "/aws/docdb/my-cluster/audit",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{auditLog}},
	}

	res := resource.Resource{
		ID:     "my-cluster",
		Fields: map[string]string{},
	}

	checker := dbcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/docdb/my-cluster/audit" {
		t.Errorf("ResourceIDs = %v, want [/aws/docdb/my-cluster/audit]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}

	// Two log groups for the same cluster.
	profilerLog := resource.Resource{
		ID:     "/aws/docdb/my-cluster/profiler",
		Fields: map[string]string{},
	}
	cache2 := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{auditLog, profilerLog}},
	}

	result2 := checker(context.Background(), nil, res, cache2)
	if result2.Count != 2 {
		t.Errorf("Count = %d, want 2 (audit + profiler)", result2.Count)
	}
}

func TestRelated_DBC_Logs_NoMatch(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/docdb/different-cluster/audit",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:     "my-cluster",
		Fields: map[string]string{},
	}

	checker := dbcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_DBC_Logs_EmptyID(t *testing.T) {
	logRes := resource.Resource{
		ID:     "/aws/docdb/my-cluster/audit",
		Fields: map[string]string{},
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}

	res := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
	}

	checker := dbcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ID)", result.Count)
	}
}

func TestRelated_DBC_Logs_NilCache(t *testing.T) {
	cache := resource.ResourceCache{}

	res := resource.Resource{
		ID:     "my-cluster",
		Fields: map[string]string{},
	}

	checker := dbcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown — empty cache, no clients)", result.Count)
	}
}

func TestRelatedDemo_DBC_Registered(t *testing.T) {
	_ = demo.GetResources
	checker := resource.GetRelatedDemo("dbc")
	if checker == nil {
		t.Fatal("no demo checker registered for dbc")
	}

	results := checker(resource.Resource{ID: "acme-docdb-prod"})
	if len(results) == 0 {
		t.Fatal("demo checker returned no results")
	}
	for _, r := range results {
		if r.TargetType == "" {
			t.Error("demo result has empty TargetType")
		}
	}
}
