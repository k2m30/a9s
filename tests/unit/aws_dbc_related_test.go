package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	docdb_types "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// dbcCheckerByTarget returns the RelatedChecker for the given target type
// registered under "dbc". It fails the test immediately if the checker is
// not found or is nil — providing clear diagnostics when registrations drift.
// This helper MUST live at package scope because aws_wave5_related_test.go
// calls it from TestRelated_DBC_Subnet_* tests.
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

// ────────────────────────────────────────────────────────────────────────────
// Registration completeness
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_Registered verifies that all nine expected related defs are
// registered for "dbc" with correct DisplayNames and non-nil Checkers.
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
		"sg":         {"Security Groups", true},
		"alarm":      {"CloudWatch Alarms", true},
		"logs":       {"Log Groups", true},
		"kms":        {"KMS Key", true},
		"secrets":    {"Secrets Manager", true},
		"dbi":        {"RDS Instances", true},
		"dbc-snap": {"DocumentDB Snapshots", true},
		"subnet":     {"Subnets", true},
		"vpc":        {"VPC", true},
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
			t.Errorf("expected related def for target %q not found in dbc registrations", target)
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// checkDbcSG — Pattern F (no cache)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_SG_Found verifies that VpcSecurityGroups on the DBCluster
// RawStruct are returned as ResourceIDs.
func TestRelated_DBC_SG_Found(t *testing.T) {
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
			VpcSecurityGroups: []docdb_types.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String("sg-0aaa111111111111a"), Status: aws.String("active")},
				{VpcSecurityGroupId: aws.String("sg-0bbb222222222222b"), Status: aws.String("active")},
			},
		},
	}

	checker := dbcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	if !seen["sg-0aaa111111111111a"] {
		t.Errorf("ResourceIDs missing sg-0aaa111111111111a; got %v", result.ResourceIDs)
	}
	if !seen["sg-0bbb222222222222b"] {
		t.Errorf("ResourceIDs missing sg-0bbb222222222222b; got %v", result.ResourceIDs)
	}
}

// TestRelated_DBC_SG_Empty verifies that a cluster with no security groups
// returns Count=0.
func TestRelated_DBC_SG_Empty(t *testing.T) {
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
			VpcSecurityGroups:   []docdb_types.VpcSecurityGroupMembership{},
		},
	}

	checker := dbcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no security groups)", result.Count)
	}
}

// TestRelated_DBC_SG_WrongRawStruct verifies that a non-DBCluster RawStruct
// returns Count=-1 (assertStruct fails).
func TestRelated_DBC_SG_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "acme-docdb-prod",
		RawStruct: "not-a-cluster",
	}

	checker := dbcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// checkDbcAlarm — Pattern D (dimension-based cache lookup)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_Alarm_Found verifies that a CloudWatch alarm with dimension
// "DBClusterIdentifier" matching the cluster ID is returned.
func TestRelated_DBC_Alarm_Found(t *testing.T) {
	const clusterID = "acme-docdb-prod"
	alarmRes := resource.Resource{
		ID: "alarm-docdb-prod-cpu",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("alarm-docdb-prod-cpu"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("DBClusterIdentifier"), Value: aws.String(clusterID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	src := resource.Resource{
		ID: clusterID,
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String(clusterID),
		},
	}

	checker := dbcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "alarm-docdb-prod-cpu" {
		t.Errorf("ResourceIDs = %v, want [alarm-docdb-prod-cpu]", result.ResourceIDs)
	}
}

// TestRelated_DBC_Alarm_NotFound verifies that alarms with non-matching
// dimensions produce Count=0.
func TestRelated_DBC_Alarm_NotFound(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "alarm-other-cluster-cpu",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("alarm-other-cluster-cpu"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("DBClusterIdentifier"), Value: aws.String("other-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
		},
	}

	checker := dbcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (dimension mismatch)", result.Count)
	}
}

// TestRelated_DBC_Alarm_EmptyID verifies that a cluster with an empty ID
// short-circuits and returns Count=0.
func TestRelated_DBC_Alarm_EmptyID(t *testing.T) {
	src := resource.Resource{
		ID:        "",
		RawStruct: docdb_types.DBCluster{},
	}

	checker := dbcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cluster ID short-circuits)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// checkDbcLogs — Pattern N (naming convention)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_Logs_Found verifies that log groups matching the
// /aws/docdb/{clusterID}/ naming convention are returned.
func TestRelated_DBC_Logs_Found(t *testing.T) {
	const clusterID = "acme-docdb-prod"
	auditLog := resource.Resource{ID: "/aws/docdb/" + clusterID + "/audit"}
	profilerLog := resource.Resource{ID: "/aws/docdb/" + clusterID + "/profiler"}
	otherLog := resource.Resource{ID: "/aws/docdb/other-cluster/audit"}

	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{auditLog, profilerLog, otherLog}},
	}
	src := resource.Resource{
		ID: clusterID,
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String(clusterID),
		},
	}

	checker := dbcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (audit + profiler log groups)", result.Count)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	if !seen[auditLog.ID] {
		t.Errorf("ResourceIDs missing %q; got %v", auditLog.ID, result.ResourceIDs)
	}
	if !seen[profilerLog.ID] {
		t.Errorf("ResourceIDs missing %q; got %v", profilerLog.ID, result.ResourceIDs)
	}
}

// TestRelated_DBC_Logs_NoMatch verifies Count=0 when no log group has the
// cluster's prefix.
func TestRelated_DBC_Logs_NoMatch(t *testing.T) {
	otherLog := resource.Resource{ID: "/aws/docdb/other-cluster/audit"}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{otherLog}},
	}
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
		},
	}

	checker := dbcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no matching log groups)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// checkDbcDBI — reverse lookup (dbi cache by DBClusterIdentifier)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_DBI_Found verifies that RDS instances with a matching
// DBClusterIdentifier are returned.
func TestRelated_DBC_DBI_Found(t *testing.T) {
	const clusterID = "acme-docdb-prod"
	dbiRes := resource.Resource{
		ID: "acme-docdb-prod-instance-1",
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("acme-docdb-prod-instance-1"),
			DBClusterIdentifier:  aws.String(clusterID),
		},
	}
	otherDbi := resource.Resource{
		ID: "other-cluster-instance",
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("other-cluster-instance"),
			DBClusterIdentifier:  aws.String("other-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{dbiRes, otherDbi}},
	}
	src := resource.Resource{
		ID: clusterID,
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String(clusterID),
		},
	}

	checker := dbcCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "acme-docdb-prod-instance-1" {
		t.Errorf("ResourceIDs = %v, want [acme-docdb-prod-instance-1]", result.ResourceIDs)
	}
}

// TestRelated_DBC_DBI_EmptyID verifies that a cluster with empty ID returns Count=0.
func TestRelated_DBC_DBI_EmptyID(t *testing.T) {
	src := resource.Resource{
		ID:        "",
		RawStruct: docdb_types.DBCluster{},
	}

	checker := dbcCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cluster ID)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// checkDbcDbcSnap — reverse lookup (dbc-snap cache by DBClusterIdentifier)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_DbcSnap_Found verifies that snapshots with matching
// DBClusterIdentifier are returned.
func TestRelated_DBC_DbcSnap_Found(t *testing.T) {
	const clusterID = "acme-docdb-prod"
	snapRes := resource.Resource{
		ID: "dbc-snap-acme-prod-20240101",
		RawStruct: docdb_types.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("dbc-snap-acme-prod-20240101"),
			DBClusterIdentifier:         aws.String(clusterID),
		},
	}
	otherSnap := resource.Resource{
		ID: "dbc-snap-other-cluster",
		RawStruct: docdb_types.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("dbc-snap-other-cluster"),
			DBClusterIdentifier:         aws.String("other-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"dbc-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{snapRes, otherSnap}},
	}
	src := resource.Resource{
		ID: clusterID,
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String(clusterID),
		},
	}

	checker := dbcCheckerByTarget(t, "dbc-snap")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "dbc-snap-acme-prod-20240101" {
		t.Errorf("ResourceIDs = %v, want [dbc-snap-acme-prod-20240101]", result.ResourceIDs)
	}
}

// TestRelated_DBC_DbcSnap_Empty verifies Count=0 when no snapshots match.
func TestRelated_DBC_DbcSnap_Empty(t *testing.T) {
	otherSnap := resource.Resource{
		ID: "dbc-snap-other-cluster",
		RawStruct: docdb_types.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("dbc-snap-other-cluster"),
			DBClusterIdentifier:         aws.String("other-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"dbc-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{otherSnap}},
	}
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
		},
	}

	checker := dbcCheckerByTarget(t, "dbc-snap")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no matching snapshots)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// checkDbcSecrets — ARN match against Secrets Manager cache
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_Secrets_Found verifies that a secret whose ARN matches the
// cluster's MasterUserSecret.SecretArn is returned.
func TestRelated_DBC_Secrets_Found(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/docdb/acme-docdb-prod-XyZaBc"
	secretRes := resource.Resource{
		ID:   secretARN,
		Name: "prod/docdb/acme-docdb-prod",
		Fields: map[string]string{
			"arn": secretARN,
		},
		RawStruct: smtypes.SecretListEntry{
			Name: aws.String("prod/docdb/acme-docdb-prod"),
			ARN:  aws.String(secretARN),
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes}},
	}
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
			MasterUserSecret: &docdb_types.ClusterMasterUserSecret{
				SecretArn: aws.String(secretARN),
			},
		},
	}

	checker := dbcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != secretARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, secretARN)
	}
}

// TestRelated_DBC_Secrets_NoMasterUserSecret verifies Count=0 when the cluster
// has no MasterUserSecret (nil pointer guard).
func TestRelated_DBC_Secrets_NoMasterUserSecret(t *testing.T) {
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
			MasterUserSecret:    nil,
		},
	}

	checker := dbcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no MasterUserSecret)", result.Count)
	}
}

// TestRelated_DBC_Secrets_WrongRawStruct verifies Count=-1 when RawStruct is
// not a DBCluster (assertStruct fails).
func TestRelated_DBC_Secrets_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "acme-docdb-prod",
		RawStruct: "not-a-cluster",
	}

	checker := dbcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// checkDbcKMS — Pattern F (no cache, KmsKeyId ARN suffix)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_KMS_Found verifies that the KMS key ID is extracted from the
// cluster's KmsKeyId ARN (last segment after "/").
func TestRelated_DBC_KMS_Found(t *testing.T) {
	const keyARN = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"
	const keyID = "a1b2c3d4-5678-90ab-cdef-111111111111"

	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
			KmsKeyId:            aws.String(keyARN),
		},
	}

	checker := dbcCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != keyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, keyID)
	}
}

// TestRelated_DBC_KMS_NoKey verifies Count=0 when the cluster has no KmsKeyId.
func TestRelated_DBC_KMS_NoKey(t *testing.T) {
	src := resource.Resource{
		ID: "acme-docdb-unencrypted",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-unencrypted"),
			KmsKeyId:            nil,
		},
	}

	checker := dbcCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key)", result.Count)
	}
}

// TestRelated_DBC_KMS_WrongRawStruct verifies Count=0 for non-DBCluster RawStruct.
func TestRelated_DBC_KMS_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "acme-docdb-prod",
		RawStruct: "not-a-cluster",
	}

	checker := dbcCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct defaults to 0 for Pattern F)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// checkDbcSubnet — Pattern C (live API, DescribeDBSubnetGroups)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_Subnet_NilDocDB verifies Count=-1 when DocDB client is nil.
// (aws_wave5_related_test.go covers TestRelated_DBC_Subnet_NilClientsW5, this
// test covers the ServiceClients != nil but DocDB == nil path.)
func TestRelated_DBC_Subnet_NilDocDB(t *testing.T) {
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
			DBSubnetGroup:       aws.String("acme-docdb-subnet-group"),
		},
	}
	clients := &awsclient.ServiceClients{DocDB: nil}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil DocDB client)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// checkDbcVPC — Pattern C (live API, DescribeDBSubnetGroups)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_VPC_NilDocDB verifies Count=-1 when DocDB client is nil.
func TestRelated_DBC_VPC_NilDocDB(t *testing.T) {
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
			DBSubnetGroup:       aws.String("acme-docdb-subnet-group"),
		},
	}
	clients := &awsclient.ServiceClients{DocDB: nil}
	checker := dbcCheckerByTarget(t, "vpc")
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil DocDB client)", result.Count)
	}
}

// TestRelated_DBC_VPC_NoSubnetGroup verifies Count=-1 when the cluster has no
// DBSubnetGroup — dbcSubnetGroup returns nil, so both subnet and vpc return -1.
func TestRelated_DBC_VPC_NoSubnetGroup(t *testing.T) {
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
			DBSubnetGroup:       nil,
		},
	}
	checker := dbcCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no DBSubnetGroup → dbcSubnetGroup nil)", result.Count)
	}
}

// TestRelated_DBC_VPC_WrongRawStruct verifies Count=-1 when RawStruct is not
// a DBCluster (assertStruct fails → dbcSubnetGroup returns nil).
func TestRelated_DBC_VPC_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "acme-docdb-prod",
		RawStruct: "not-a-cluster",
	}
	checker := dbcCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type → dbcSubnetGroup nil)", result.Count)
	}
}
