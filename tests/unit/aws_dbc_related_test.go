package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	docdb_types "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// dbcCheckerByTarget returns the RelatedChecker for the given target type
// registered under "dbc". It fails the test immediately if the checker is
// not found or is nil — providing clear diagnostics when registrations drift.
// This helper MUST live at package scope because aws_related_checkers_branch_coverage_test.go
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
		"sg":       {"Security Groups", true},
		"alarm":    {"CloudWatch Alarms", true},
		"logs":     {"Log Groups", true},
		"kms":      {"KMS Key", true},
		"secrets":  {"Secrets Manager", true},
		"dbi":      {"RDS Instances", true},
		"dbc-snap": {"DB Cluster Snapshots", true},
		"subnet":   {"Subnets", true},
		"vpc":      {"VPC", true},
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

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		seen[id] = true
	}
	if !seen["sg-0aaa111111111111a"] {
		t.Errorf("ResourceIDs missing sg-0aaa111111111111a; got %v", result.ResourceIDs())
	}
	if !seen["sg-0bbb222222222222b"] {
		t.Errorf("ResourceIDs missing sg-0bbb222222222222b; got %v", result.ResourceIDs())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no security groups)", result.Count())
	}
}

func TestRelated_DBC_SG_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "acme-docdb-prod",
		RawStruct: "not-a-cluster",
	}

	checker := dbcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

func TestRelated_DBC_Alarm_Found(t *testing.T) {
	const clusterID = "acme-docdb-prod"
	alarmRes := resource.Resource{
		ID: "alarm-docdb-prod-cpu",
		RawStruct: cwtypes.MetricAlarm{
			Namespace: aws.String("AWS/DocDB"),
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "alarm-docdb-prod-cpu" {
		t.Errorf("ResourceIDs = %v, want [alarm-docdb-prod-cpu]", result.ResourceIDs())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (dimension mismatch)", result.Count())
	}
}

// A nil alarm cache is not a proven zero; it resolves to
// UnknownRelated("alarm") (docs/related-resources-engine.md).
func TestRelated_DBC_Alarm_NilCache_ReturnsUnknown(t *testing.T) {
	src := resource.Resource{
		ID: "acme-docdb-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("acme-docdb-prod"),
		},
	}

	checker := dbcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (nil alarm cache is not a proven zero — canonical per docs/related-resources-engine.md §7)", result.State())
	}
}

func TestRelated_DBC_Alarm_EmptyID(t *testing.T) {
	src := resource.Resource{
		ID:        "",
		RawStruct: docdb_types.DBCluster{},
	}

	checker := dbcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty cluster ID short-circuits)", result.Count())
	}
}

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

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (audit + profiler log groups)", result.Count())
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		seen[id] = true
	}
	if !seen[auditLog.ID] {
		t.Errorf("ResourceIDs missing %q; got %v", auditLog.ID, result.ResourceIDs())
	}
	if !seen[profilerLog.ID] {
		t.Errorf("ResourceIDs missing %q; got %v", profilerLog.ID, result.ResourceIDs())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no matching log groups)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "acme-docdb-prod-instance-1" {
		t.Errorf("ResourceIDs = %v, want [acme-docdb-prod-instance-1]", result.ResourceIDs())
	}
}

func TestRelated_DBC_DBI_EmptyID(t *testing.T) {
	src := resource.Resource{
		ID:        "",
		RawStruct: docdb_types.DBCluster{},
	}

	checker := dbcCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty cluster ID)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "dbc-snap-acme-prod-20240101" {
		t.Errorf("ResourceIDs = %v, want [dbc-snap-acme-prod-20240101]", result.ResourceIDs())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no matching snapshots)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != secretARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), secretARN)
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no MasterUserSecret)", result.Count())
	}
}

// dbcClusterMasterSecretARN returns "" for any unrecognised parent, so there
// is no MasterUserSecret link to find. Returning -1 would drop the honest
// lower bound — see TestAllReverseScanCheckers_TruncatedEmptyCacheReturnsTruncated.
func TestRelated_DBC_Secrets_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "acme-docdb-prod",
		RawStruct: "not-a-cluster",
	}

	checker := dbcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no MasterUserSecret on unrecognised parent shape)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != keyID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), keyID)
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key)", result.Count())
	}
}

func TestRelated_DBC_KMS_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "acme-docdb-prod",
		RawStruct: "not-a-cluster",
	}

	checker := dbcCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct defaults to 0 for Pattern F)", result.Count())
	}
}

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

	// With no DocDB client no call was made, so nothing failed: the pivot
	// reads unknown, never an AWS error.
	if result.State() != domain.RelatedUnknown || result.Err() != nil {
		t.Errorf("State = %v Err = %v, want unknown with no error (nil DocDB client)", result.State(), result.Err())
	}
}

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

	// With no DocDB client no call was made, so nothing failed: the pivot
	// reads unknown, never an AWS error.
	if result.State() != domain.RelatedUnknown || result.Err() != nil {
		t.Errorf("State = %v Err = %v, want unknown with no error (nil DocDB client)", result.State(), result.Err())
	}
}

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

	// A cluster naming no subnet group has no subnets, which is a fact about
	// the cluster rather than a gap in what could be read.
	if result.State() != domain.RelatedResolved || result.Count() != 0 {
		t.Errorf("State = %v, Count = %d; want Resolved 0 (the cluster names no DBSubnetGroup)", result.State(), result.Count())
	}
}

func TestRelated_DBC_VPC_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "acme-docdb-prod",
		RawStruct: "not-a-cluster",
	}
	checker := dbcCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	// A client that cannot be called read nothing, so the answer is the error,
	// not a "?" that would stand for three different states.
	if result.State() != domain.RelatedError {
		t.Errorf("State = %v, want RelatedError (RawStruct is not a DBCluster, so nothing was read)", result.State())
	}
}
