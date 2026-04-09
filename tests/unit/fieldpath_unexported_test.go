package unit_test

import (
	"strings"
	"testing"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
)

// TestExtractSubtree_AllAWSResourceTypes verifies that ExtractSubtree does NOT panic
// on any AWS SDK struct, including those with unexported fields.
// Every supported resource type is tested with both scalar and nested/slice paths.
func TestExtractSubtree_AllAWSResourceTypes(t *testing.T) {
	t.Run("EC2 Instance", func(t *testing.T) {
		inst := ec2types.Instance{
			InstanceId:       new("i-12345"),
			InstanceType:     ec2types.InstanceTypeT2Micro,
			PrivateIpAddress: new("10.0.0.1"),
			PublicIpAddress:  new("54.1.2.3"),
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameRunning,
				Code: new(int32(16)),
			},
			Placement: &ec2types.Placement{
				AvailabilityZone: new("eu-central-1a"),
				Tenancy:          ec2types.TenancyDefault,
			},
			SecurityGroups: []ec2types.GroupIdentifier{
				{GroupId: new("sg-111"), GroupName: new("web")},
				{GroupId: new("sg-222"), GroupName: new("db")},
			},
			Tags: []ec2types.Tag{
				{Key: new("Name"), Value: new("my-instance")},
				{Key: new("Env"), Value: new("prod")},
			},
			BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
				{DeviceName: new("/dev/sda1")},
			},
		}

		assertNoSubtreePanic(t, inst, "InstanceId", "i-12345")
		assertNoSubtreePanic(t, inst, "State", "running")
		assertNoSubtreePanic(t, inst, "Placement", "eu-central-1a")
		assertNoSubtreePanic(t, inst, "SecurityGroups", "sg-111")
		assertNoSubtreePanic(t, inst, "Tags", "Name")
		assertNoSubtreePanic(t, inst, "BlockDeviceMappings", "/dev/sda1")
		assertNoScalarPanic(t, inst, "InstanceId", "i-12345")
		assertNoScalarPanic(t, inst, "State.Name", "running")
		assertNoScalarPanic(t, inst, "InstanceType", "t2.micro")
	})

	t.Run("S3 Bucket", func(t *testing.T) {
		bucket := s3types.Bucket{
			Name:      new("my-bucket"),
			BucketArn: new("arn:aws:s3:::my-bucket"),
		}

		assertNoSubtreePanic(t, bucket, "Name", "my-bucket")
		assertNoScalarPanic(t, bucket, "Name", "my-bucket")
	})

	t.Run("S3 Object", func(t *testing.T) {
		obj := s3types.Object{
			Key:          new("path/to/file.txt"),
			Size:         new(int64(1024)),
			StorageClass: s3types.ObjectStorageClassStandard,
		}

		assertNoSubtreePanic(t, obj, "Key", "path/to/file.txt")
		assertNoScalarPanic(t, obj, "Key", "path/to/file.txt")
		assertNoScalarPanic(t, obj, "StorageClass", "STANDARD")
	})

	t.Run("RDS Instance", func(t *testing.T) {
		db := rdstypes.DBInstance{
			DBInstanceIdentifier: new("mydb"),
			Engine:               new("postgres"),
			EngineVersion:        new("14.9"),
			DBInstanceStatus:     new("available"),
			DBInstanceClass:      new("db.t3.micro"),
			MultiAZ:              new(true),
			AllocatedStorage:     new(int32(20)),
			Endpoint: &rdstypes.Endpoint{
				Address: new("mydb.abc.rds.amazonaws.com"),
				Port:    new(int32(5432)),
			},
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				DBSubnetGroupName: new("default"),
			},
			VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: new("sg-rds-1"), Status: new("active")},
			},
		}

		assertNoSubtreePanic(t, db, "DBInstanceIdentifier", "mydb")
		assertNoSubtreePanic(t, db, "Endpoint", "mydb.abc.rds.amazonaws.com")
		assertNoSubtreePanic(t, db, "DBSubnetGroup", "default")
		assertNoSubtreePanic(t, db, "VpcSecurityGroups", "sg-rds-1")
		assertNoScalarPanic(t, db, "Engine", "postgres")
		assertNoScalarPanic(t, db, "Endpoint.Address", "mydb.abc.rds.amazonaws.com")
		assertNoScalarPanic(t, db, "MultiAZ", "Yes")
	})

	t.Run("ElastiCache Redis", func(t *testing.T) {
		cluster := elasticachetypes.CacheCluster{
			CacheClusterId:     new("redis-001"),
			Engine:             new("redis"),
			EngineVersion:      new("7.0"),
			CacheClusterStatus: new("available"),
			CacheNodeType:      new("cache.t3.micro"),
			NumCacheNodes:      new(int32(1)),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: new("redis-001.abc.cache.amazonaws.com"),
				Port:    new(int32(6379)),
			},
			CacheNodes: []elasticachetypes.CacheNode{
				{CacheNodeId: new("0001")},
			},
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{
				{SecurityGroupId: new("sg-redis-1"), Status: new("active")},
			},
		}

		assertNoSubtreePanic(t, cluster, "CacheClusterId", "redis-001")
		assertNoSubtreePanic(t, cluster, "ConfigurationEndpoint", "redis-001.abc.cache.amazonaws.com")
		assertNoSubtreePanic(t, cluster, "CacheNodes", "0001")
		assertNoSubtreePanic(t, cluster, "SecurityGroups", "sg-redis-1")
		assertNoScalarPanic(t, cluster, "Engine", "redis")
		assertNoScalarPanic(t, cluster, "NumCacheNodes", "1")
	})

	t.Run("DocumentDB Cluster", func(t *testing.T) {
		cluster := docdbtypes.DBCluster{
			DBClusterIdentifier: new("docdb-001"),
			Engine:              new("dbc"),
			EngineVersion:       new("5.0"),
			Status:              new("available"),
			Endpoint:            new("docdb-001.abc.docdb.amazonaws.com"),
			ReaderEndpoint:      new("docdb-001-ro.abc.docdb.amazonaws.com"),
			Port:                new(int32(27017)),
			StorageEncrypted:    new(true),
			DBClusterMembers: []docdbtypes.DBClusterMember{
				{DBInstanceIdentifier: new("docdb-001-instance-1"), IsClusterWriter: new(true)},
			},
			AssociatedRoles: []docdbtypes.DBClusterRole{
				{RoleArn: new("arn:aws:iam::123:role/docdb-role")},
			},
		}

		assertNoSubtreePanic(t, cluster, "DBClusterIdentifier", "docdb-001")
		assertNoSubtreePanic(t, cluster, "DBClusterMembers", "docdb-001-instance-1")
		assertNoSubtreePanic(t, cluster, "AssociatedRoles", "docdb-role")
		assertNoScalarPanic(t, cluster, "Endpoint", "docdb-001.abc.docdb.amazonaws.com")
		assertNoScalarPanic(t, cluster, "StorageEncrypted", "Yes")
		assertNoScalarPanic(t, cluster, "Port", "27017")
	})

	t.Run("EKS Cluster", func(t *testing.T) {
		cluster := ekstypes.Cluster{
			Name:            new("eks-001"),
			Version:         new("1.28"),
			Status:          ekstypes.ClusterStatusActive,
			Endpoint:        new("https://abc.eks.amazonaws.com"),
			Arn:             new("arn:aws:eks:eu-central-1:123:cluster/eks-001"),
			PlatformVersion: new("eks.5"),
			RoleArn:         new("arn:aws:iam::123:role/eks-role"),
			KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
				IpFamily:        ekstypes.IpFamilyIpv4,
				ServiceIpv4Cidr: new("10.100.0.0/16"),
			},
			EncryptionConfig: []ekstypes.EncryptionConfig{
				{Resources: []string{"secrets"}},
			},
		}

		assertNoSubtreePanic(t, cluster, "Name", "eks-001")
		assertNoSubtreePanic(t, cluster, "KubernetesNetworkConfig", "10.100.0.0/16")
		assertNoSubtreePanic(t, cluster, "EncryptionConfig", "secrets")
		assertNoScalarPanic(t, cluster, "Version", "1.28")
		assertNoScalarPanic(t, cluster, "Status", "ACTIVE")
		assertNoScalarPanic(t, cluster, "Arn", "arn:aws:eks:eu-central-1:123:cluster/eks-001")
	})

	t.Run("Secrets Manager", func(t *testing.T) {
		secret := smtypes.SecretListEntry{
			Name:            new("my-secret"),
			ARN:             new("arn:aws:secretsmanager:eu-central-1:123:secret:my-secret"),
			Description:     new("A test secret"),
			KmsKeyId:        new("alias/aws/secretsmanager"),
			RotationEnabled: new(false),
			Tags: []smtypes.Tag{
				{Key: new("Env"), Value: new("prod")},
			},
			RotationRules: &smtypes.RotationRulesType{
				AutomaticallyAfterDays: new(int64(30)),
			},
		}

		assertNoSubtreePanic(t, secret, "Name", "my-secret")
		assertNoSubtreePanic(t, secret, "Tags", "Env")
		assertNoSubtreePanic(t, secret, "RotationRules", "30")
		assertNoScalarPanic(t, secret, "ARN", "arn:aws:secretsmanager")
		assertNoScalarPanic(t, secret, "Description", "A test secret")
		assertNoScalarPanic(t, secret, "RotationEnabled", "No")
	})
}

// assertNoSubtreePanic calls ExtractSubtree and fails if it panics or result doesn't contain expected.
func assertNoSubtreePanic(t *testing.T, obj any, path, expectedContains string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ExtractSubtree(%T, %q) panicked: %v", obj, path, r)
		}
	}()
	result := fieldpath.ExtractSubtree(obj, path)
	if !strings.Contains(result, expectedContains) {
		t.Errorf("ExtractSubtree(%T, %q) = %q, want to contain %q", obj, path, result, expectedContains)
	}
}

// assertNoScalarPanic calls ExtractScalar and fails if it panics or result doesn't contain expected.
func assertNoScalarPanic(t *testing.T, obj any, path, expectedContains string) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ExtractScalar(%T, %q) panicked: %v", obj, path, r)
		}
	}()
	result := fieldpath.ExtractScalar(obj, path)
	if !strings.Contains(result, expectedContains) {
		t.Errorf("ExtractScalar(%T, %q) = %q, want to contain %q", obj, path, result, expectedContains)
	}
}

//go:fix inline
func int64Ptr(i int64) *int64 { return new(i) }
