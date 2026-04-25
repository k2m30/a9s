package aws

// dbc_related_internal_test.go — internal package tests for shape-agnostic
// dbc cluster helpers.
//
// All helpers are unexported, so tests must live in package aws. The tests
// cover both docdbtypes.DBCluster and rdstypes.DBCluster shapes, plus nil/empty
// edge cases, to pin the dual-dispatch contracts.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestDbcClusterIdentifier_DualShape verifies dbcClusterIdentifier handles both
// docdbtypes.DBCluster and rdstypes.DBCluster, and returns "" for nil/unrecognised input.
func TestDbcClusterIdentifier_DualShape(t *testing.T) {
	cases := []struct {
		name string
		raw  any
		want string
	}{
		{
			name: "docdb_cluster",
			raw:  docdbtypes.DBCluster{DBClusterIdentifier: aws.String("prod-docdb")},
			want: "prod-docdb",
		},
		{
			name: "rds_cluster",
			raw:  rdstypes.DBCluster{DBClusterIdentifier: aws.String("prod-aurora")},
			want: "prod-aurora",
		},
		{
			name: "docdb_nil_identifier",
			raw:  docdbtypes.DBCluster{DBClusterIdentifier: nil},
			want: "",
		},
		{
			name: "rds_nil_identifier",
			raw:  rdstypes.DBCluster{DBClusterIdentifier: nil},
			want: "",
		},
		{
			name: "unrecognised_type",
			raw:  "not-a-cluster",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dbcClusterIdentifier(tc.raw)
			if got != tc.want {
				t.Errorf("dbcClusterIdentifier = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDbcClusterVpcSecurityGroupIDs_DualShape verifies dbcClusterVpcSecurityGroupIDs
// for both SDK shapes and edge cases.
func TestDbcClusterVpcSecurityGroupIDs_DualShape(t *testing.T) {
	sg1 := "sg-aaa111"
	sg2 := "sg-bbb222"

	t.Run("docdb_two_sgs", func(t *testing.T) {
		raw := docdbtypes.DBCluster{
			VpcSecurityGroups: []docdbtypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(sg1)},
				{VpcSecurityGroupId: aws.String(sg2)},
			},
		}
		ids, ok := dbcClusterVpcSecurityGroupIDs(raw)
		if !ok {
			t.Fatal("ok=false for valid docdb shape")
		}
		if len(ids) != 2 || ids[0] != sg1 || ids[1] != sg2 {
			t.Errorf("ids = %v, want [%s %s]", ids, sg1, sg2)
		}
	})

	t.Run("rds_one_sg", func(t *testing.T) {
		raw := rdstypes.DBCluster{
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String(sg1)},
			},
		}
		ids, ok := dbcClusterVpcSecurityGroupIDs(raw)
		if !ok {
			t.Fatal("ok=false for valid rds shape")
		}
		if len(ids) != 1 || ids[0] != sg1 {
			t.Errorf("ids = %v, want [%s]", ids, sg1)
		}
	})

	t.Run("docdb_no_sgs", func(t *testing.T) {
		raw := docdbtypes.DBCluster{VpcSecurityGroups: nil}
		ids, ok := dbcClusterVpcSecurityGroupIDs(raw)
		if !ok {
			t.Fatal("ok=false for valid docdb shape with empty SGs")
		}
		if len(ids) != 0 {
			t.Errorf("ids = %v, want []", ids)
		}
	})

	t.Run("unrecognised_type", func(t *testing.T) {
		_, ok := dbcClusterVpcSecurityGroupIDs("not-a-cluster")
		if ok {
			t.Error("ok=true for unrecognised input type, want false")
		}
	})
}

// TestDbcClusterKmsKeyID_DualShape verifies dbcClusterKmsKeyID for both shapes.
func TestDbcClusterKmsKeyID_DualShape(t *testing.T) {
	const kmsARN = "arn:aws:kms:us-east-1:123456789012:key/abcdef12-0000-0000-0000-000000000000"

	t.Run("docdb", func(t *testing.T) {
		raw := docdbtypes.DBCluster{KmsKeyId: aws.String(kmsARN)}
		got := dbcClusterKmsKeyID(raw)
		if got != kmsARN {
			t.Errorf("got %q, want %q", got, kmsARN)
		}
	})

	t.Run("rds", func(t *testing.T) {
		raw := rdstypes.DBCluster{KmsKeyId: aws.String(kmsARN)}
		got := dbcClusterKmsKeyID(raw)
		if got != kmsARN {
			t.Errorf("got %q, want %q", got, kmsARN)
		}
	})

	t.Run("docdb_nil", func(t *testing.T) {
		raw := docdbtypes.DBCluster{KmsKeyId: nil}
		got := dbcClusterKmsKeyID(raw)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// TestDbcClusterSubnetGroupName_DualShape verifies dbcClusterSubnetGroupName for both shapes.
func TestDbcClusterSubnetGroupName_DualShape(t *testing.T) {
	const sngName = "acme-docdb-subnet-group"

	t.Run("docdb", func(t *testing.T) {
		raw := docdbtypes.DBCluster{DBSubnetGroup: aws.String(sngName)}
		got := dbcClusterSubnetGroupName(raw)
		if got != sngName {
			t.Errorf("got %q, want %q", got, sngName)
		}
	})

	t.Run("rds", func(t *testing.T) {
		raw := rdstypes.DBCluster{DBSubnetGroup: aws.String(sngName)}
		got := dbcClusterSubnetGroupName(raw)
		if got != sngName {
			t.Errorf("got %q, want %q", got, sngName)
		}
	})

	t.Run("docdb_nil", func(t *testing.T) {
		raw := docdbtypes.DBCluster{DBSubnetGroup: nil}
		got := dbcClusterSubnetGroupName(raw)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// TestDbcClusterMasterSecretARN_DualShape verifies dbcClusterMasterSecretARN.
func TestDbcClusterMasterSecretARN_DualShape(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!cluster-abc-xyz"

	t.Run("docdb", func(t *testing.T) {
		raw := docdbtypes.DBCluster{
			MasterUserSecret: &docdbtypes.ClusterMasterUserSecret{
				SecretArn: aws.String(secretARN),
			},
		}
		got := dbcClusterMasterSecretARN(raw)
		if got != secretARN {
			t.Errorf("got %q, want %q", got, secretARN)
		}
	})

	t.Run("rds", func(t *testing.T) {
		raw := rdstypes.DBCluster{
			MasterUserSecret: &rdstypes.MasterUserSecret{
				SecretArn: aws.String(secretARN),
			},
		}
		got := dbcClusterMasterSecretARN(raw)
		if got != secretARN {
			t.Errorf("got %q, want %q", got, secretARN)
		}
	})

	t.Run("docdb_no_secret", func(t *testing.T) {
		raw := docdbtypes.DBCluster{MasterUserSecret: nil}
		got := dbcClusterMasterSecretARN(raw)
		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// TestCheckDbcKMS_DualShape verifies checkDbcKMS extracts the UUID from the
// KMS key ARN for both docdbtypes and rdstypes shapes.
func TestCheckDbcKMS_DualShape(t *testing.T) {
	const kmsARN = "arn:aws:kms:us-east-1:123456789012:key/abcdef12-0000-0000-0000-000000000000"
	const kmsUUID = "abcdef12-0000-0000-0000-000000000000"
	emptyCache := resource.ResourceCache{}

	t.Run("docdb_extracts_uuid", func(t *testing.T) {
		res := resource.Resource{
			ID:        "prod-docdb",
			RawStruct: docdbtypes.DBCluster{KmsKeyId: aws.String(kmsARN)},
		}
		result := checkDbcKMS(context.Background(), nil, res, emptyCache)
		if result.TargetType != "kms" {
			t.Errorf("TargetType = %q, want kms", result.TargetType)
		}
		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}
		if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != kmsUUID {
			t.Errorf("IDs = %v, want [%s]", result.ResourceIDs, kmsUUID)
		}
	})

	t.Run("rds_extracts_uuid", func(t *testing.T) {
		res := resource.Resource{
			ID:        "prod-aurora",
			RawStruct: rdstypes.DBCluster{KmsKeyId: aws.String(kmsARN)},
		}
		result := checkDbcKMS(context.Background(), nil, res, emptyCache)
		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}
		if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != kmsUUID {
			t.Errorf("IDs = %v, want [%s]", result.ResourceIDs, kmsUUID)
		}
	})

	t.Run("docdb_no_key", func(t *testing.T) {
		res := resource.Resource{
			ID:        "prod-docdb-unencrypted",
			RawStruct: docdbtypes.DBCluster{KmsKeyId: nil},
		}
		result := checkDbcKMS(context.Background(), nil, res, emptyCache)
		if result.Count != 0 {
			t.Errorf("Count = %d, want 0 when KmsKeyId nil", result.Count)
		}
	})

	t.Run("rds_no_key", func(t *testing.T) {
		res := resource.Resource{
			ID:        "prod-aurora-unencrypted",
			RawStruct: rdstypes.DBCluster{KmsKeyId: nil},
		}
		result := checkDbcKMS(context.Background(), nil, res, emptyCache)
		if result.Count != 0 {
			t.Errorf("Count = %d, want 0 when KmsKeyId nil", result.Count)
		}
	})
}
