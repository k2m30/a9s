// aws_dbi_related_wave2_test.go — coverage wave 2 for dbi_related.go checkers.
// Covers: checkDBILogs, checkDbiVPC, checkDbiDBC, checkDbiRole, checkDbiENI.
// checkDbiENI requires a live EC2 client — only nil-client path is tested here.
package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// checkDBILogs — Pattern N: prefix "/aws/rds/instance/{dbID}/"
// ---------------------------------------------------------------------------

func TestRelated_DBI_Logs_MatchByPrefix(t *testing.T) {
	const dbID = "my-postgres-db"
	logGroup1 := "/aws/rds/instance/" + dbID + "/postgresql"
	logGroup2 := "/aws/rds/instance/" + dbID + "/upgrade"

	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: logGroup1},
				{ID: logGroup2},
				{ID: "/aws/rds/instance/other-db/postgresql"},
			},
		},
	}

	src := resource.Resource{
		ID:        dbID,
		RawStruct: rdstypes.DBInstance{},
	}

	checker := dbiCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	for _, want := range []string{logGroup1, logGroup2} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

func TestRelated_DBI_Logs_NoMatchDifferentDB(t *testing.T) {
	const dbID = "my-postgres-db"

	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "/aws/rds/instance/other-db/postgresql"},
			},
		},
	}

	src := resource.Resource{
		ID:        dbID,
		RawStruct: rdstypes.DBInstance{},
	}

	checker := dbiCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// Edge: empty dbID → Count=0 (guard).
func TestRelated_DBI_Logs_EmptyIDReturnsZero(t *testing.T) {
	src := resource.Resource{ID: "", RawStruct: rdstypes.DBInstance{}}

	checker := dbiCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty dbID)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkDbiVPC — Pattern F: VpcId from DBSubnetGroup
// ---------------------------------------------------------------------------

func TestRelated_DBI_VPC_ReturnsVpcID(t *testing.T) {
	const vpcID = "vpc-0abc123def456789a"

	src := resource.Resource{
		ID: "my-postgres-db",
		RawStruct: rdstypes.DBInstance{
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId: aws.String(vpcID),
			},
		},
	}

	checker := dbiCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != vpcID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, vpcID)
	}
}

func TestRelated_DBI_VPC_ReturnsZeroWhenNoSubnetGroup(t *testing.T) {
	src := resource.Resource{
		ID:        "my-postgres-db",
		RawStruct: rdstypes.DBInstance{DBSubnetGroup: nil},
	}

	checker := dbiCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// Edge: DBSubnetGroup present but VpcId empty — returns 0.
func TestRelated_DBI_VPC_ReturnsZeroWhenEmptyVpcID(t *testing.T) {
	src := resource.Resource{
		ID: "my-postgres-db",
		RawStruct: rdstypes.DBInstance{
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId: aws.String(""),
			},
		},
	}

	checker := dbiCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty VpcId)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkDbiDBC — Pattern C: reverse cache lookup by DBClusterIdentifier
// ---------------------------------------------------------------------------

func TestRelated_DBI_DBC_MatchByClusterID(t *testing.T) {
	const clusterID = "my-aurora-cluster"

	dbcRes := resource.Resource{
		ID:   clusterID,
		Name: clusterID,
	}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{Resources: []resource.Resource{dbcRes}},
	}

	src := resource.Resource{
		ID: "my-aurora-instance-1",
		RawStruct: rdstypes.DBInstance{
			DBClusterIdentifier: aws.String(clusterID),
		},
	}

	checker := dbiCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != clusterID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, clusterID)
	}
}

func TestRelated_DBI_DBC_ReturnsZeroWhenNoClusterID(t *testing.T) {
	src := resource.Resource{
		ID: "my-standalone-db",
		RawStruct: rdstypes.DBInstance{
			DBClusterIdentifier: nil,
		},
	}

	checker := dbiCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no cluster)", result.Count)
	}
}

// Edge: cluster not found in cache — Count=0.
func TestRelated_DBI_DBC_ClusterNotInCache(t *testing.T) {
	const clusterID = "my-aurora-cluster"

	otherClusterRes := resource.Resource{ID: "other-cluster", Name: "other-cluster"}
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{Resources: []resource.Resource{otherClusterRes}},
	}

	src := resource.Resource{
		ID: "my-aurora-instance-1",
		RawStruct: rdstypes.DBInstance{
			DBClusterIdentifier: aws.String(clusterID),
		},
	}

	checker := dbiCheckerByTarget(t, "dbc")
	result := checker(context.Background(), nil, src, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (cluster not in cache)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkDbiRole — Pattern F: AssociatedRoles + MonitoringRoleArn
// ---------------------------------------------------------------------------

func TestRelated_DBI_Role_ExtractsFromAssociatedRoles(t *testing.T) {
	const roleARN = "arn:aws:iam::123456789012:role/rds-monitoring-role"
	const roleName = "rds-monitoring-role"

	src := resource.Resource{
		ID: "my-postgres-db",
		RawStruct: rdstypes.DBInstance{
			AssociatedRoles: []rdstypes.DBInstanceRole{
				{RoleArn: aws.String(roleARN)},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != roleName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, roleName)
	}
}

func TestRelated_DBI_Role_ReturnsZeroWhenNoRoles(t *testing.T) {
	src := resource.Resource{
		ID: "my-postgres-db",
		RawStruct: rdstypes.DBInstance{
			AssociatedRoles:    nil,
			MonitoringRoleArn:  nil,
		},
	}

	checker := dbiCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no roles)", result.Count)
	}
}

// Edge: both AssociatedRoles and MonitoringRoleArn present — both returned.
func TestRelated_DBI_Role_CombinesAssociatedAndMonitoringRoles(t *testing.T) {
	const assocRoleARN = "arn:aws:iam::123456789012:role/rds-s3-integration-role"
	const monitorRoleARN = "arn:aws:iam::123456789012:role/rds-enhanced-monitoring-role"

	src := resource.Resource{
		ID: "my-postgres-db",
		RawStruct: rdstypes.DBInstance{
			AssociatedRoles: []rdstypes.DBInstanceRole{
				{RoleArn: aws.String(assocRoleARN)},
			},
			MonitoringRoleArn: aws.String(monitorRoleARN),
		},
	}

	checker := dbiCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (assoc + monitoring role)", result.Count)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	if !seen["rds-s3-integration-role"] {
		t.Errorf("ResourceIDs missing rds-s3-integration-role; got %v", result.ResourceIDs)
	}
	if !seen["rds-enhanced-monitoring-role"] {
		t.Errorf("ResourceIDs missing rds-enhanced-monitoring-role; got %v", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// checkDbiENI — Pattern A: requires live EC2 client; nil-client path only.
// ---------------------------------------------------------------------------

// TestRelated_DBI_ENI_NilClientReturnsNegOne verifies that without a live EC2
// client the checker returns Count=-1 (unavailable, not an error).
func TestRelated_DBI_ENI_NilClientReturnsNegOne(t *testing.T) {
	src := resource.Resource{
		ID: "my-postgres-db",
		RawStruct: rdstypes.DBInstance{
			// DBSubnetGroup.VpcId must be non-empty so the nil-client guard is
			// reached (the checker returns Count=0 early when VpcId is absent).
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId: aws.String("vpc-0a1b2c3d4e5f60001"),
			},
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String("sg-0a1b2c3d4e5f60001")},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no EC2 client)", result.Count)
	}
}
