package unit_test

// aws_docdb_related_extra_test.go — additional coverage for dbc_related.go
// Covers: checkDbcSG, checkDbcDBI, checkDbcDocdbSnap, checkDbcKMS.
// checkDbcSubnet/VPC require a live docdb:DescribeDBSubnetGroups API call and
// return -1 without a client — those nil-client branches are covered below.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdb_types "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- checkDbcSG (Pattern F — VpcSecurityGroups[].VpcSecurityGroupId) ---

func TestRelated_DBC_SG_Found(t *testing.T) {
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("docdb-cluster-prod"),
			VpcSecurityGroups: []docdb_types.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String("sg-0abc1234")},
				{VpcSecurityGroupId: aws.String("sg-0def5678")},
			},
		},
	}
	checker := dbcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.ResourceIDs[0] != "sg-0abc1234" {
		t.Errorf("ResourceIDs[0] = %q, want sg-0abc1234", result.ResourceIDs[0])
	}
}

func TestRelated_DBC_SG_NoSecurityGroups(t *testing.T) {
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("docdb-cluster-prod"),
			VpcSecurityGroups:   nil,
		},
	}
	checker := dbcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SGs)", result.Count)
	}
}

func TestRelated_DBC_SG_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "docdb-cluster-prod", RawStruct: "not-a-cluster"}
	checker := dbcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkDbcDBI (Pattern C — reverse scan of dbi cache by DBClusterIdentifier) ---

func TestRelated_DBC_DBI_MatchByClusterID(t *testing.T) {
	const clusterID = "docdb-cluster-prod"
	source := resource.Resource{
		ID: clusterID,
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String(clusterID),
		},
	}
	dbi1 := resource.Resource{
		ID: "docdb-cluster-prod-instance-1",
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("docdb-cluster-prod-instance-1"),
			DBClusterIdentifier:  aws.String(clusterID),
		},
	}
	dbi2 := resource.Resource{
		ID: "other-cluster-instance",
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("other-cluster-instance"),
			DBClusterIdentifier:  aws.String("other-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{dbi1, dbi2}},
	}

	checker := dbcCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "docdb-cluster-prod-instance-1" {
		t.Errorf("ResourceIDs[0] = %q, want docdb-cluster-prod-instance-1", result.ResourceIDs[0])
	}
}

func TestRelated_DBC_DBI_NoMatch(t *testing.T) {
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("docdb-cluster-prod"),
		},
	}
	dbi := resource.Resource{
		ID: "other-instance",
		RawStruct: rdstypes.DBInstance{
			DBInstanceIdentifier: aws.String("other-instance"),
			DBClusterIdentifier:  aws.String("other-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{dbi}},
	}

	checker := dbcCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_DBC_DBI_EmptyIDReturnsZero(t *testing.T) {
	source := resource.Resource{ID: "", RawStruct: docdb_types.DBCluster{}}
	checker := dbcCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty cluster ID)", result.Count)
	}
}

func TestRelated_DBC_DBI_NilCacheNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("docdb-cluster-prod"),
		},
	}
	checker := dbcCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}

// --- checkDbcDocdbSnap (Pattern C — reverse scan of docdb-snap cache) ---

func TestRelated_DBC_DocdbSnap_MatchByClusterID(t *testing.T) {
	const clusterID = "docdb-cluster-prod"
	source := resource.Resource{
		ID: clusterID,
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String(clusterID),
		},
	}
	snap1 := resource.Resource{
		ID: "docdb-cluster-prod-snap-20240101",
		RawStruct: docdb_types.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("docdb-cluster-prod-snap-20240101"),
			DBClusterIdentifier:         aws.String(clusterID),
		},
	}
	snap2 := resource.Resource{
		ID: "other-cluster-snap",
		RawStruct: docdb_types.DBClusterSnapshot{
			DBClusterSnapshotIdentifier: aws.String("other-cluster-snap"),
			DBClusterIdentifier:         aws.String("other-cluster"),
		},
	}
	cache := resource.ResourceCache{
		"docdb-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{snap1, snap2}},
	}

	checker := dbcCheckerByTarget(t, "docdb-snap")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "docdb-cluster-prod-snap-20240101" {
		t.Errorf("ResourceIDs[0] = %q, want docdb-cluster-prod-snap-20240101", result.ResourceIDs[0])
	}
}

func TestRelated_DBC_DocdbSnap_NoMatch(t *testing.T) {
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{DBClusterIdentifier: aws.String("docdb-cluster-prod")},
	}
	cache := resource.ResourceCache{
		"docdb-snap": resource.ResourceCacheEntry{Resources: []resource.Resource{}},
	}
	checker := dbcCheckerByTarget(t, "docdb-snap")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_DBC_DocdbSnap_NilCacheNoClients(t *testing.T) {
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{DBClusterIdentifier: aws.String("docdb-cluster-prod")},
	}
	checker := dbcCheckerByTarget(t, "docdb-snap")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}

// --- checkDbcKMS (Pattern F — reads KmsKeyId from DBCluster) ---

func TestRelated_DBC_KMS_ARNExtractedToID(t *testing.T) {
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("docdb-cluster-prod"),
			KmsKeyId:            aws.String("arn:aws:kms:us-east-1:123456789012:key/docdb-key-001"),
		},
	}
	checker := dbcCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "docdb-key-001" {
		t.Errorf("ResourceIDs[0] = %q, want docdb-key-001 (last ARN segment)", result.ResourceIDs[0])
	}
}

func TestRelated_DBC_KMS_NoEncryption(t *testing.T) {
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("docdb-cluster-prod"),
			KmsKeyId:            nil,
		},
	}
	checker := dbcCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (not encrypted)", result.Count)
	}
}

func TestRelated_DBC_KMS_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "docdb-cluster-prod", RawStruct: "not-a-cluster"}
	checker := dbcCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct short-circuits to 0)", result.Count)
	}
}

// --- checkDbcSubnet / checkDbcVPC — nil client path ---

func TestRelated_DBC_Subnet_NilClients(t *testing.T) {
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("docdb-cluster-prod"),
			DBSubnetGroup:       aws.String("docdb-subnet-group"),
		},
	}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients — DescribeDBSubnetGroups unavailable)", result.Count)
	}
}

func TestRelated_DBC_VPC_NilClients(t *testing.T) {
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("docdb-cluster-prod"),
			DBSubnetGroup:       aws.String("docdb-subnet-group"),
		},
	}
	checker := dbcCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- checkDbcSecrets — additional: RawStruct match path ---

func TestRelated_DBC_Secrets_MatchesByRawStructARN(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:docdb-master-abcdef"
	source := resource.Resource{
		ID: "docdb-cluster-prod",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("docdb-cluster-prod"),
			MasterUserSecret: &docdb_types.ClusterMasterUserSecret{
				SecretArn: aws.String(secretARN),
			},
		},
	}
	// Cache entry has no "arn" field but has the ARN in RawStruct.
	secretRes := resource.Resource{
		ID: "docdb-master-abcdef",
		RawStruct: smtypes.SecretListEntry{
			ARN: aws.String(secretARN),
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes}},
	}

	checker := dbcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (matched via RawStruct ARN)", result.Count)
	}
}
