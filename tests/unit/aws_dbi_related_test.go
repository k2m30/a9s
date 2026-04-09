package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
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

// --- checkDbiKMS tests (Pattern F — reads KmsKeyId from RawStruct) ---

func TestRelated_DBI_KMS_ExtractsKeyIDFromARN(t *testing.T) {
	res := resource.Resource{
		ID:     "my-db-instance",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			KmsKeyId: aws.String("arn:aws:kms:us-east-1:111122223333:key/mrk-abc12345-1234-1234-1234-abc123456789"),
		},
	}

	checker := dbiCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "kms")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "mrk-abc12345-1234-1234-1234-abc123456789" {
		t.Errorf("ResourceIDs = %v, want [mrk-abc12345-1234-1234-1234-abc123456789]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_DBI_KMS_ReturnsZeroWhenNoKey(t *testing.T) {
	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{KmsKeyId: nil},
	}

	checker := dbiCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key)", result.Count)
	}
	if result.TargetType != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "kms")
	}
}

func TestRelated_DBI_KMS_ReturnsZeroWhenEmptyARN(t *testing.T) {
	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{KmsKeyId: aws.String("")},
	}

	checker := dbiCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ARN)", result.Count)
	}
}

func TestRelated_DBI_KMS_ReturnsZeroWhenARNHasNoSlash(t *testing.T) {
	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{KmsKeyId: aws.String("not-an-arn-with-slash")},
	}

	checker := dbiCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (ARN without slash yields empty key ID)", result.Count)
	}
}

func TestRelated_DBI_KMS_ReturnsNegOneOnBadRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: "not-a-db-instance",
	}

	checker := dbiCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad RawStruct type)", result.Count)
	}
	if result.TargetType != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "kms")
	}
}

func TestRelated_DBI_KMS_KeyIDOnlyARN(t *testing.T) {
	// ARN where the part after "/" is a plain UUID (not multi-region key).
	res := resource.Resource{
		ID:     "my-db-instance",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			KmsKeyId: aws.String("arn:aws:kms:us-west-2:123456789012:key/plain-uuid-1234"),
		},
	}

	checker := dbiCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "plain-uuid-1234" {
		t.Errorf("ResourceIDs = %v, want [plain-uuid-1234]", result.ResourceIDs)
	}
}

// --- checkDbiSubnets tests (Pattern F — reads DBSubnetGroup from RawStruct) ---

func TestRelated_DBI_Subnets_ReturnsSubnetIDs(t *testing.T) {
	res := resource.Resource{
		ID:     "my-db-instance",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				Subnets: []rdstypes.Subnet{
					{SubnetIdentifier: aws.String("subnet-aaa111")},
					{SubnetIdentifier: aws.String("subnet-bbb222")},
				},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.TargetType != "subnet" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "subnet")
	}
	wantIDs := map[string]bool{"subnet-aaa111": false, "subnet-bbb222": false}
	for _, id := range result.ResourceIDs {
		wantIDs[id] = true
	}
	for id, found := range wantIDs {
		if !found {
			t.Errorf("ResourceIDs missing %q; got %v", id, result.ResourceIDs)
		}
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_DBI_Subnets_ReturnsZeroWhenNoSubnetGroup(t *testing.T) {
	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{DBSubnetGroup: nil},
	}

	checker := dbiCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no subnet group)", result.Count)
	}
	if result.TargetType != "subnet" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "subnet")
	}
}

func TestRelated_DBI_Subnets_ReturnsZeroWhenEmptySubnets(t *testing.T) {
	res := resource.Resource{
		ID:     "my-db-instance",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				Subnets: []rdstypes.Subnet{},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty subnets slice)", result.Count)
	}
}

func TestRelated_DBI_Subnets_SkipsNilSubnetIdentifier(t *testing.T) {
	res := resource.Resource{
		ID:     "my-db-instance",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				Subnets: []rdstypes.Subnet{
					{SubnetIdentifier: nil},
					{SubnetIdentifier: aws.String("")},
					{SubnetIdentifier: aws.String("subnet-valid")},
				},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (only non-empty subnet IDs counted)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "subnet-valid" {
		t.Errorf("ResourceIDs = %v, want [subnet-valid]", result.ResourceIDs)
	}
}

func TestRelated_DBI_Subnets_ReturnsNegOneOnBadRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: "not-a-db-instance",
	}

	checker := dbiCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad RawStruct type)", result.Count)
	}
	if result.TargetType != "subnet" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "subnet")
	}
}

func TestRelated_DBI_Subnets_SingleSubnet(t *testing.T) {
	res := resource.Resource{
		ID:     "my-db-instance",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				Subnets: []rdstypes.Subnet{
					{SubnetIdentifier: aws.String("subnet-only-one")},
				},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "subnet-only-one" {
		t.Errorf("ResourceIDs = %v, want [subnet-only-one]", result.ResourceIDs)
	}
}
