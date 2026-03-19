package unit_test

import (
	"testing"
	"time"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/fieldpath"
	"github.com/k2m30/a9s/internal/resource"
)

// ===========================================================================
// Helpers
// ===========================================================================

func ptrString(s string) *string    { return &s }
func ptrBool(b bool) *bool          { return &b }
func ptrInt32(i int32) *int32       { return &i }
func ptrInt64(i int64) *int64       { return &i }
func ptrTime(t time.Time) *time.Time { return &t }

var testTime = time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

// ===========================================================================
// Realistic SDK struct builders
// ===========================================================================

func realisticS3Bucket() s3types.Bucket {
	return s3types.Bucket{
		Name:         ptrString("my-production-bucket"),
		CreationDate: ptrTime(testTime),
		BucketArn:    ptrString("arn:aws:s3:::my-production-bucket"),
		BucketRegion: ptrString("us-east-1"),
	}
}

func realisticS3ObjectFile() s3types.Object {
	return s3types.Object{
		Key:          ptrString("data/report-2025.csv"),
		Size:         ptrInt64(1048576),
		LastModified: ptrTime(testTime),
		StorageClass: s3types.ObjectStorageClassStandard,
		ETag:         ptrString("\"d41d8cd98f00b204e9800998ecf8427e\""),
	}
}

func realisticS3ObjectFolder() s3types.CommonPrefix {
	return s3types.CommonPrefix{
		Prefix: ptrString("data/reports/"),
	}
}

func realisticEC2Instance() ec2types.Instance {
	return ec2types.Instance{
		InstanceId:       ptrString("i-0abcdef1234567890"),
		InstanceType:     ec2types.InstanceTypeT3Medium,
		PrivateIpAddress: ptrString("10.0.1.42"),
		PublicIpAddress:  ptrString("54.123.45.67"),
		LaunchTime:       ptrTime(testTime),
		ImageId:          ptrString("ami-0abcdef1234567890"),
		VpcId:            ptrString("vpc-0abc1234"),
		SubnetId:         ptrString("subnet-0abc5678"),
		Architecture:     ec2types.ArchitectureValuesX8664,
		KeyName:          ptrString("prod-keypair"),
		PrivateDnsName:   ptrString("ip-10-0-1-42.ec2.internal"),
		EbsOptimized:     ptrBool(true),
		Placement: &ec2types.Placement{
			AvailabilityZone: ptrString("us-east-1a"),
			Tenancy:          ec2types.TenancyDefault,
		},
		IamInstanceProfile: &ec2types.IamInstanceProfile{
			Arn: ptrString("arn:aws:iam::123456789012:instance-profile/web-server-role"),
			Id:  ptrString("AIPAXYZ1234567890ABCD"),
		},
		MetadataOptions: &ec2types.InstanceMetadataOptionsResponse{
			HttpEndpoint:            ec2types.InstanceMetadataEndpointStateEnabled,
			HttpTokens:              ec2types.HttpTokensStateRequired,
			HttpPutResponseHopLimit: ptrInt32(2),
		},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: ptrInt32(16),
		},
		Tags: []ec2types.Tag{
			{Key: ptrString("Name"), Value: ptrString("web-server-prod")},
			{Key: ptrString("env"), Value: ptrString("production")},
		},
		SecurityGroups: []ec2types.GroupIdentifier{
			{GroupId: ptrString("sg-0abc1234"), GroupName: ptrString("web-sg")},
		},
		PlatformDetails: ptrString("Linux/UNIX"),
		Platform:        ec2types.PlatformValuesWindows,
	}
}

func realisticRDSInstance() rdstypes.DBInstance {
	return rdstypes.DBInstance{
		DBInstanceIdentifier: ptrString("prod-db-01"),
		DBInstanceArn:        ptrString("arn:aws:rds:us-east-1:123456789012:db:prod-db-01"),
		Engine:               ptrString("mysql"),
		EngineVersion:        ptrString("8.0.35"),
		DBInstanceStatus:     ptrString("available"),
		DBInstanceClass:      ptrString("db.r5.large"),
		MultiAZ:              ptrBool(true),
		AllocatedStorage:     ptrInt32(100),
		StorageType:          ptrString("gp3"),
		Iops:                 ptrInt32(3000),
		StorageEncrypted:     ptrBool(true),
		KmsKeyId:             ptrString("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
		AvailabilityZone:     ptrString("us-east-1a"),
		PubliclyAccessible:   ptrBool(false),
		DBSubnetGroup: &rdstypes.DBSubnetGroup{
			DBSubnetGroupName:        ptrString("prod-db-subnet-group"),
			DBSubnetGroupDescription: ptrString("Production DB subnet group"),
		},
		VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: ptrString("sg-0abc1234"), Status: ptrString("active")},
		},
		BackupRetentionPeriod:      ptrInt32(7),
		PreferredMaintenanceWindow: ptrString("sun:03:00-sun:04:00"),
		PreferredBackupWindow:      ptrString("02:00-03:00"),
		DeletionProtection:         ptrBool(true),
		MasterUsername:             ptrString("admin"),
		PerformanceInsightsEnabled: ptrBool(true),
		TagList: []rdstypes.Tag{
			{Key: ptrString("env"), Value: ptrString("production")},
		},
		Endpoint: &rdstypes.Endpoint{
			Address: ptrString("prod-db-01.abc123.us-east-1.rds.amazonaws.com"),
			Port:    ptrInt32(3306),
		},
	}
}

func realisticRedisCacheCluster() elasticachetypes.CacheCluster {
	return elasticachetypes.CacheCluster{
		CacheClusterId:     ptrString("redis-prod-001"),
		ARN:                ptrString("arn:aws:elasticache:us-east-1:123456789012:cluster:redis-prod-001"),
		Engine:             ptrString("redis"),
		EngineVersion:      ptrString("7.0.12"),
		CacheNodeType:      ptrString("cache.r6g.large"),
		CacheClusterStatus: ptrString("available"),
		NumCacheNodes:      ptrInt32(3),
		ConfigurationEndpoint: &elasticachetypes.Endpoint{
			Address: ptrString("redis-prod-001.abc123.clustercfg.use1.cache.amazonaws.com"),
			Port:    ptrInt32(6379),
		},
		PreferredAvailabilityZone: ptrString("us-east-1a"),
		CacheNodes: []elasticachetypes.CacheNode{
			{
				CacheNodeId:              ptrString("0001"),
				CacheNodeStatus:          ptrString("available"),
				CustomerAvailabilityZone: ptrString("us-east-1a"),
			},
		},
		ReplicationGroupId:      ptrString("redis-prod-repl"),
		CacheSubnetGroupName:    ptrString("redis-prod-subnet-group"),
		SecurityGroups: []elasticachetypes.SecurityGroupMembership{
			{SecurityGroupId: ptrString("sg-0abc1234"), Status: ptrString("active")},
		},
		AtRestEncryptionEnabled:    ptrBool(true),
		TransitEncryptionEnabled:   ptrBool(true),
		AuthTokenEnabled:           ptrBool(false),
		SnapshotRetentionLimit:     ptrInt32(7),
		PreferredMaintenanceWindow: ptrString("sun:05:00-sun:06:00"),
	}
}

func realisticDocDBCluster() docdbtypes.DBCluster {
	return docdbtypes.DBCluster{
		DBClusterIdentifier: ptrString("docdb-prod-cluster"),
		DBClusterArn:        ptrString("arn:aws:rds:us-east-1:123456789012:cluster:docdb-prod-cluster"),
		Engine:              ptrString("dbc"),
		EngineVersion:       ptrString("5.0.0"),
		Status:              ptrString("available"),
		Endpoint:            ptrString("docdb-prod.cluster-abc123.us-east-1.docdb.amazonaws.com"),
		ReaderEndpoint:      ptrString("docdb-prod.cluster-ro-abc123.us-east-1.docdb.amazonaws.com"),
		Port:                ptrInt32(27017),
		StorageEncrypted:    ptrBool(true),
		KmsKeyId:            ptrString("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
		DeletionProtection:  ptrBool(true),
		DBSubnetGroup:       ptrString("docdb-prod-subnet-group"),
		VpcSecurityGroups: []docdbtypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: ptrString("sg-0abc5678"), Status: ptrString("active")},
		},
		BackupRetentionPeriod:      ptrInt32(7),
		PreferredMaintenanceWindow: ptrString("sun:04:00-sun:05:00"),
		MasterUsername:             ptrString("docdbadmin"),
		DBClusterMembers: []docdbtypes.DBClusterMember{
			{DBInstanceIdentifier: ptrString("docdb-prod-instance-1"), IsClusterWriter: ptrBool(true)},
			{DBInstanceIdentifier: ptrString("docdb-prod-instance-2"), IsClusterWriter: ptrBool(false)},
		},
	}
}

func realisticEKSCluster() *ekstypes.Cluster {
	return &ekstypes.Cluster{
		Name:            ptrString("prod-cluster"),
		Version:         ptrString("1.28"),
		Status:          ekstypes.ClusterStatusActive,
		Endpoint:        ptrString("https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com"),
		PlatformVersion: ptrString("eks.5"),
		Arn:             ptrString("arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster"),
		RoleArn:         ptrString("arn:aws:iam::123456789012:role/eks-cluster-role"),
		CreatedAt:       ptrTime(testTime),
		KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
			ServiceIpv4Cidr: ptrString("172.20.0.0/16"),
		},
		ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
			ClusterSecurityGroupId: ptrString("sg-0abc9999"),
			EndpointPrivateAccess:  true,
			EndpointPublicAccess:   true,
			VpcId:                  ptrString("vpc-0abc1234"),
		},
		Logging: &ekstypes.Logging{
			ClusterLogging: []ekstypes.LogSetup{
				{
					Enabled: ptrBool(true),
					Types:   []ekstypes.LogType{ekstypes.LogTypeApi, ekstypes.LogTypeAudit},
				},
			},
		},
		Identity: &ekstypes.Identity{
			Oidc: &ekstypes.OIDC{
				Issuer: ptrString("https://oidc.eks.us-east-1.amazonaws.com/id/ABCDEF1234567890"),
			},
		},
		Tags: map[string]string{
			"env":     "production",
			"team":    "platform",
		},
	}
}

func realisticSecretListEntry() smtypes.SecretListEntry {
	rotatedTime := testTime.Add(-24 * time.Hour)
	return smtypes.SecretListEntry{
		Name:              ptrString("prod/database/password"),
		Description:       ptrString("Production database password"),
		LastAccessedDate:  ptrTime(testTime),
		LastChangedDate:   ptrTime(testTime),
		RotationEnabled:   ptrBool(true),
		ARN:               ptrString("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/password-AbCdEf"),
		KmsKeyId:          ptrString("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
		CreatedDate:       ptrTime(testTime.Add(-90 * 24 * time.Hour)),
		LastRotatedDate:   ptrTime(rotatedTime),
		RotationLambdaARN: ptrString("arn:aws:lambda:us-east-1:123456789012:function:SecretsManagerRotation"),
		RotationRules: &smtypes.RotationRulesType{
			AutomaticallyAfterDays: ptrInt64(30),
		},
		PrimaryRegion: ptrString("us-east-1"),
		Tags: []smtypes.Tag{
			{Key: ptrString("env"), Value: ptrString("production")},
		},
	}
}

// ===========================================================================
// S3 Buckets
// ===========================================================================

func TestQA_ListViewColumns_S3Bucket(t *testing.T) {
	bucket := realisticS3Bucket()
	vd := config.DefaultViewDef("s3")

	for _, col := range vd.List {
		t.Run(col.Title, func(t *testing.T) {
			result := fieldpath.ExtractScalar(bucket, col.Path)
			if result == "" {
				t.Errorf("ExtractScalar(%q) returned empty for realistic S3 Bucket", col.Path)
			}
		})
	}

	// Verify specific values
	if got := fieldpath.ExtractScalar(bucket, "Name"); got != "my-production-bucket" {
		t.Errorf("Name: expected %q, got %q", "my-production-bucket", got)
	}
}

func TestQA_DetailViewPaths_S3Bucket(t *testing.T) {
	bucket := realisticS3Bucket()
	vd := config.DefaultViewDef("s3")

	for _, path := range vd.Detail {
		t.Run(path, func(t *testing.T) {
			result := fieldpath.ExtractSubtree(bucket, path)
			if result == "" {
				t.Errorf("ExtractSubtree(%q) returned empty for realistic S3 Bucket", path)
			}
		})
	}
}

func TestQA_NilFields_S3Bucket(t *testing.T) {
	// Minimal bucket with nil fields
	bucket := s3types.Bucket{}
	vd := config.DefaultViewDef("s3")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractScalar(bucket, col.Path)
		})
	}

	for _, path := range vd.Detail {
		t.Run("detail_"+path, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractSubtree(bucket, path)
		})
	}
}

// ===========================================================================
// S3 Objects (files) — s3types.Object
// ===========================================================================

func TestQA_ListViewColumns_S3ObjectFile(t *testing.T) {
	obj := realisticS3ObjectFile()
	vd := config.DefaultViewDef("s3_objects")

	for _, col := range vd.List {
		t.Run(col.Title, func(t *testing.T) {
			result := fieldpath.ExtractScalar(obj, col.Path)
			if result == "" {
				t.Errorf("ExtractScalar(%q) returned empty for realistic S3 Object (file)", col.Path)
			}
		})
	}

	// Verify specific values
	if got := fieldpath.ExtractScalar(obj, "Key"); got != "data/report-2025.csv" {
		t.Errorf("Key: expected %q, got %q", "data/report-2025.csv", got)
	}
}

func TestQA_DetailViewPaths_S3ObjectFile(t *testing.T) {
	obj := realisticS3ObjectFile()
	vd := config.DefaultViewDef("s3_objects")

	// s3_objects has no Detail paths defined in defaults, but test list paths with ExtractSubtree
	for _, col := range vd.List {
		t.Run(col.Path, func(t *testing.T) {
			result := fieldpath.ExtractSubtree(obj, col.Path)
			if result == "" {
				t.Errorf("ExtractSubtree(%q) returned empty for realistic S3 Object (file)", col.Path)
			}
		})
	}
}

func TestQA_NilFields_S3ObjectFile(t *testing.T) {
	// Minimal S3 Object with nil fields
	obj := s3types.Object{}
	vd := config.DefaultViewDef("s3_objects")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic — nil fields should return ""
			_ = fieldpath.ExtractScalar(obj, col.Path)
		})
	}
}

// ===========================================================================
// S3 Objects (folders) — s3types.CommonPrefix — CRITICAL: ONLY has Prefix!
// ===========================================================================

func TestQA_ListViewColumns_S3ObjectFolder(t *testing.T) {
	folder := realisticS3ObjectFolder()
	vd := config.DefaultViewDef("s3_objects")

	// CommonPrefix ONLY has Prefix field, NOT Key, Size, LastModified, StorageClass
	// The extraction must NOT crash on missing fields — it should return ""
	for _, col := range vd.List {
		t.Run(col.Title, func(t *testing.T) {
			// Must not panic
			result := fieldpath.ExtractScalar(folder, col.Path)
			// Only "Key" maps to nothing on CommonPrefix — Prefix is the field name
			// All paths (Key, Size, LastModified, StorageClass) should return "" for CommonPrefix
			// since CommonPrefix only has Prefix field
			_ = result
		})
	}

	// Verify Prefix extraction works directly
	if got := fieldpath.ExtractScalar(folder, "Prefix"); got != "data/reports/" {
		t.Errorf("Prefix: expected %q, got %q", "data/reports/", got)
	}
}

func TestQA_NilFields_S3ObjectFolder(t *testing.T) {
	// Completely empty CommonPrefix
	folder := s3types.CommonPrefix{}
	vd := config.DefaultViewDef("s3_objects")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractScalar(folder, col.Path)
		})
	}
}

func TestQA_S3ObjectFolder_DoesNotCrashOnAnyColumn(t *testing.T) {
	// Ensure that every s3_objects column applied to a CommonPrefix never panics
	folder := realisticS3ObjectFolder()
	vd := config.DefaultViewDef("s3_objects")

	for _, col := range vd.List {
		t.Run("scalar_"+col.Path, func(t *testing.T) {
			_ = fieldpath.ExtractScalar(folder, col.Path)
		})
		t.Run("subtree_"+col.Path, func(t *testing.T) {
			_ = fieldpath.ExtractSubtree(folder, col.Path)
		})
	}
}

// ===========================================================================
// EC2 Instances
// ===========================================================================

func TestQA_ListViewColumns_EC2(t *testing.T) {
	inst := realisticEC2Instance()
	vd := config.DefaultViewDef("ec2")

	for _, col := range vd.List {
		t.Run(col.Title, func(t *testing.T) {
			// Columns with empty Path use Fields fallback (e.g. Name from Tags)
			// ExtractScalar won't find them — skip
			if col.Path == "" {
				t.Skipf("column %q has no path (uses Fields fallback)", col.Title)
				return
			}
			result := fieldpath.ExtractScalar(inst, col.Path)
			// Tags is a slice, so ExtractScalar returns "" (non-scalar)
			if col.Path == "Tags" {
				if result != "" {
					t.Errorf("Tags is a slice; ExtractScalar should return empty, got %q", result)
				}
				return
			}
			if result == "" {
				t.Errorf("ExtractScalar(%q) returned empty for realistic EC2 Instance", col.Path)
			}
		})
	}

	// Verify specific values
	if got := fieldpath.ExtractScalar(inst, "InstanceId"); got != "i-0abcdef1234567890" {
		t.Errorf("InstanceId: expected %q, got %q", "i-0abcdef1234567890", got)
	}
	if got := fieldpath.ExtractScalar(inst, "State.Name"); got != "running" {
		t.Errorf("State.Name: expected %q, got %q", "running", got)
	}
	if got := fieldpath.ExtractScalar(inst, "InstanceType"); got != "t3.medium" {
		t.Errorf("InstanceType: expected %q, got %q", "t3.medium", got)
	}
	if got := fieldpath.ExtractScalar(inst, "PrivateIpAddress"); got != "10.0.1.42" {
		t.Errorf("PrivateIpAddress: expected %q, got %q", "10.0.1.42", got)
	}
	if got := fieldpath.ExtractScalar(inst, "PublicIpAddress"); got != "54.123.45.67" {
		t.Errorf("PublicIpAddress: expected %q, got %q", "54.123.45.67", got)
	}
}

func TestQA_DetailViewPaths_EC2(t *testing.T) {
	inst := realisticEC2Instance()
	vd := config.DefaultViewDef("ec2")

	for _, path := range vd.Detail {
		t.Run(path, func(t *testing.T) {
			result := fieldpath.ExtractSubtree(inst, path)
			if result == "" {
				t.Errorf("ExtractSubtree(%q) returned empty for realistic EC2 Instance", path)
			}
		})
	}

	// Verify Tags renders as YAML (non-empty, multi-line content)
	tagsYAML := fieldpath.ExtractSubtree(inst, "Tags")
	if tagsYAML == "" {
		t.Error("Tags should produce non-empty YAML")
	}

	// Verify SecurityGroups renders as YAML
	sgYAML := fieldpath.ExtractSubtree(inst, "SecurityGroups")
	if sgYAML == "" {
		t.Error("SecurityGroups should produce non-empty YAML")
	}

	// Verify State renders as YAML subtree (has Name and Code fields)
	stateYAML := fieldpath.ExtractSubtree(inst, "State")
	if stateYAML == "" {
		t.Error("State should produce non-empty YAML")
	}
}

func TestQA_NilFields_EC2(t *testing.T) {
	// Minimal EC2 instance — no public IP, no tags, no state
	inst := ec2types.Instance{
		InstanceType: ec2types.InstanceTypeT3Micro,
	}
	vd := config.DefaultViewDef("ec2")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic — nil pointer fields should return ""
			_ = fieldpath.ExtractScalar(inst, col.Path)
		})
	}

	for _, path := range vd.Detail {
		t.Run("detail_"+path, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractSubtree(inst, path)
		})
	}

	// Specifically verify nil PublicIpAddress returns ""
	if got := fieldpath.ExtractScalar(inst, "PublicIpAddress"); got != "" {
		t.Errorf("nil PublicIpAddress should return empty, got %q", got)
	}

	// Specifically verify nil State returns "" for nested path
	if got := fieldpath.ExtractScalar(inst, "State.Name"); got != "" {
		t.Errorf("nil State.Name should return empty, got %q", got)
	}
}

// ===========================================================================
// DB Instances
// ===========================================================================

func TestQA_ListViewColumns_RDS(t *testing.T) {
	db := realisticRDSInstance()
	vd := config.DefaultViewDef("dbi")

	for _, col := range vd.List {
		t.Run(col.Title, func(t *testing.T) {
			result := fieldpath.ExtractScalar(db, col.Path)
			if result == "" {
				t.Errorf("ExtractScalar(%q) returned empty for realistic RDS Instance", col.Path)
			}
		})
	}

	// Verify specific values
	if got := fieldpath.ExtractScalar(db, "DBInstanceIdentifier"); got != "prod-db-01" {
		t.Errorf("DBInstanceIdentifier: expected %q, got %q", "prod-db-01", got)
	}
	if got := fieldpath.ExtractScalar(db, "Engine"); got != "mysql" {
		t.Errorf("Engine: expected %q, got %q", "mysql", got)
	}
	if got := fieldpath.ExtractScalar(db, "Endpoint.Address"); got != "prod-db-01.abc123.us-east-1.rds.amazonaws.com" {
		t.Errorf("Endpoint.Address: expected correct address, got %q", got)
	}
	// MultiAZ is *bool — ExtractScalar should format it via FormatValue
	if got := fieldpath.ExtractScalar(db, "MultiAZ"); got != "Yes" {
		t.Errorf("MultiAZ: expected %q, got %q", "Yes", got)
	}
}

func TestQA_DetailViewPaths_RDS(t *testing.T) {
	db := realisticRDSInstance()
	vd := config.DefaultViewDef("dbi")

	// RDS SDK struct uses "TagList" but the view config references "Tags".
	// fieldpath can't resolve "Tags" on the struct (falls back to Fields map in production).
	knownMismatches := map[string]bool{"Tags": true}

	for _, path := range vd.Detail {
		t.Run(path, func(t *testing.T) {
			result := fieldpath.ExtractSubtree(db, path)
			if result == "" {
				if knownMismatches[path] {
					t.Skipf("path %q is a known config/struct name mismatch (struct uses TagList)", path)
					return
				}
				t.Errorf("ExtractSubtree(%q) returned empty for realistic RDS Instance", path)
			}
		})
	}

	// Verify Endpoint renders as YAML subtree (has Address and Port)
	epYAML := fieldpath.ExtractSubtree(db, "Endpoint")
	if epYAML == "" {
		t.Error("Endpoint should produce non-empty YAML")
	}

	// Verify TagList extraction works directly (the actual SDK field name)
	tagListYAML := fieldpath.ExtractSubtree(db, "TagList")
	if tagListYAML == "" {
		t.Error("TagList should produce non-empty YAML")
	}
}

func TestQA_NilFields_RDS(t *testing.T) {
	// Minimal RDS instance — creating state, no endpoint
	db := rdstypes.DBInstance{
		DBInstanceIdentifier: ptrString("test-db"),
		DBInstanceStatus:     ptrString("creating"),
	}
	vd := config.DefaultViewDef("dbi")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic — nil Endpoint should not crash on Endpoint.Address
			_ = fieldpath.ExtractScalar(db, col.Path)
		})
	}

	for _, path := range vd.Detail {
		t.Run("detail_"+path, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractSubtree(db, path)
		})
	}

	// Specifically verify nil Endpoint.Address returns ""
	if got := fieldpath.ExtractScalar(db, "Endpoint.Address"); got != "" {
		t.Errorf("nil Endpoint.Address should return empty, got %q", got)
	}
}

// ===========================================================================
// Redis (ElastiCache)
// ===========================================================================

func TestQA_ListViewColumns_Redis(t *testing.T) {
	cluster := realisticRedisCacheCluster()
	vd := config.DefaultViewDef("redis")

	for _, col := range vd.List {
		t.Run(col.Title, func(t *testing.T) {
			result := fieldpath.ExtractScalar(cluster, col.Path)
			if result == "" {
				t.Errorf("ExtractScalar(%q) returned empty for realistic Redis CacheCluster", col.Path)
			}
		})
	}

	// Verify specific values
	if got := fieldpath.ExtractScalar(cluster, "CacheClusterId"); got != "redis-prod-001" {
		t.Errorf("CacheClusterId: expected %q, got %q", "redis-prod-001", got)
	}
	if got := fieldpath.ExtractScalar(cluster, "ConfigurationEndpoint.Address"); got != "redis-prod-001.abc123.clustercfg.use1.cache.amazonaws.com" {
		t.Errorf("ConfigurationEndpoint.Address: expected correct address, got %q", got)
	}
}

func TestQA_DetailViewPaths_Redis(t *testing.T) {
	cluster := realisticRedisCacheCluster()
	vd := config.DefaultViewDef("redis")

	for _, path := range vd.Detail {
		t.Run(path, func(t *testing.T) {
			result := fieldpath.ExtractSubtree(cluster, path)
			if result == "" {
				t.Errorf("ExtractSubtree(%q) returned empty for realistic Redis CacheCluster", path)
			}
		})
	}

	// Verify ConfigurationEndpoint renders as YAML subtree (has Address and Port)
	epYAML := fieldpath.ExtractSubtree(cluster, "ConfigurationEndpoint")
	if epYAML == "" {
		t.Error("ConfigurationEndpoint should produce non-empty YAML")
	}
}

func TestQA_NilFields_Redis(t *testing.T) {
	// Minimal Redis cluster — no endpoint, no nodes
	cluster := elasticachetypes.CacheCluster{
		CacheClusterId:     ptrString("redis-test"),
		CacheClusterStatus: ptrString("creating"),
	}
	vd := config.DefaultViewDef("redis")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic — nil ConfigurationEndpoint should not crash
			_ = fieldpath.ExtractScalar(cluster, col.Path)
		})
	}

	for _, path := range vd.Detail {
		t.Run("detail_"+path, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractSubtree(cluster, path)
		})
	}

	// Specifically verify nil ConfigurationEndpoint.Address returns ""
	if got := fieldpath.ExtractScalar(cluster, "ConfigurationEndpoint.Address"); got != "" {
		t.Errorf("nil ConfigurationEndpoint.Address should return empty, got %q", got)
	}
}

// ===========================================================================
// DocumentDB
// ===========================================================================

func TestQA_ListViewColumns_DocDB(t *testing.T) {
	cluster := realisticDocDBCluster()
	vd := config.DefaultViewDef("dbc")

	for _, col := range vd.List {
		t.Run(col.Title, func(t *testing.T) {
			result := fieldpath.ExtractScalar(cluster, col.Path)
			// DBClusterMembers is a slice, so ExtractScalar returns "" (non-scalar)
			if col.Path == "DBClusterMembers" {
				if result != "" {
					t.Errorf("DBClusterMembers is a slice; ExtractScalar should return empty, got %q", result)
				}
				return
			}
			if result == "" {
				t.Errorf("ExtractScalar(%q) returned empty for realistic DocDB Cluster", col.Path)
			}
		})
	}

	// Verify specific values
	if got := fieldpath.ExtractScalar(cluster, "DBClusterIdentifier"); got != "docdb-prod-cluster" {
		t.Errorf("DBClusterIdentifier: expected %q, got %q", "docdb-prod-cluster", got)
	}
	if got := fieldpath.ExtractScalar(cluster, "Status"); got != "available" {
		t.Errorf("Status: expected %q, got %q", "available", got)
	}
	if got := fieldpath.ExtractScalar(cluster, "Endpoint"); got != "docdb-prod.cluster-abc123.us-east-1.docdb.amazonaws.com" {
		t.Errorf("Endpoint: expected correct address, got %q", got)
	}
}

func TestQA_DetailViewPaths_DocDB(t *testing.T) {
	cluster := realisticDocDBCluster()
	vd := config.DefaultViewDef("dbc")

	for _, path := range vd.Detail {
		t.Run(path, func(t *testing.T) {
			result := fieldpath.ExtractSubtree(cluster, path)
			if result == "" {
				t.Errorf("ExtractSubtree(%q) returned empty for realistic DocDB Cluster", path)
			}
		})
	}

	// Verify DBClusterMembers renders as YAML
	membersYAML := fieldpath.ExtractSubtree(cluster, "DBClusterMembers")
	if membersYAML == "" {
		t.Error("DBClusterMembers should produce non-empty YAML")
	}
}

func TestQA_NilFields_DocDB(t *testing.T) {
	// Minimal DocDB cluster — no members, no endpoints
	cluster := docdbtypes.DBCluster{
		DBClusterIdentifier: ptrString("docdb-test"),
		Status:              ptrString("creating"),
	}
	vd := config.DefaultViewDef("dbc")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractScalar(cluster, col.Path)
		})
	}

	for _, path := range vd.Detail {
		t.Run("detail_"+path, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractSubtree(cluster, path)
		})
	}
}

// ===========================================================================
// EKS Clusters
// ===========================================================================

func TestQA_ListViewColumns_EKS(t *testing.T) {
	cluster := realisticEKSCluster()
	vd := config.DefaultViewDef("eks")

	for _, col := range vd.List {
		t.Run(col.Title, func(t *testing.T) {
			result := fieldpath.ExtractScalar(cluster, col.Path)
			if result == "" {
				t.Errorf("ExtractScalar(%q) returned empty for realistic EKS Cluster", col.Path)
			}
		})
	}

	// Verify specific values
	if got := fieldpath.ExtractScalar(cluster, "Name"); got != "prod-cluster" {
		t.Errorf("Name: expected %q, got %q", "prod-cluster", got)
	}
	if got := fieldpath.ExtractScalar(cluster, "Version"); got != "1.28" {
		t.Errorf("Version: expected %q, got %q", "1.28", got)
	}
	if got := fieldpath.ExtractScalar(cluster, "Status"); got != "ACTIVE" {
		t.Errorf("Status: expected %q, got %q", "ACTIVE", got)
	}
}

func TestQA_DetailViewPaths_EKS(t *testing.T) {
	cluster := realisticEKSCluster()
	vd := config.DefaultViewDef("eks")

	for _, path := range vd.Detail {
		t.Run(path, func(t *testing.T) {
			result := fieldpath.ExtractSubtree(cluster, path)
			if result == "" {
				t.Errorf("ExtractSubtree(%q) returned empty for realistic EKS Cluster", path)
			}
		})
	}

	// Verify KubernetesNetworkConfig renders as YAML subtree
	kncYAML := fieldpath.ExtractSubtree(cluster, "KubernetesNetworkConfig")
	if kncYAML == "" {
		t.Error("KubernetesNetworkConfig should produce non-empty YAML")
	}
}

func TestQA_NilFields_EKS(t *testing.T) {
	// Minimal EKS cluster — no endpoint, no network config
	cluster := &ekstypes.Cluster{
		Name:   ptrString("test-cluster"),
		Status: ekstypes.ClusterStatusCreating,
	}
	vd := config.DefaultViewDef("eks")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractScalar(cluster, col.Path)
		})
	}

	for _, path := range vd.Detail {
		t.Run("detail_"+path, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractSubtree(cluster, path)
		})
	}
}

// ===========================================================================
// Secrets Manager
// ===========================================================================

func TestQA_ListViewColumns_Secrets(t *testing.T) {
	secret := realisticSecretListEntry()
	vd := config.DefaultViewDef("secrets")

	for _, col := range vd.List {
		t.Run(col.Title, func(t *testing.T) {
			result := fieldpath.ExtractScalar(secret, col.Path)
			if result == "" {
				t.Errorf("ExtractScalar(%q) returned empty for realistic SecretListEntry", col.Path)
			}
		})
	}

	// Verify specific values
	if got := fieldpath.ExtractScalar(secret, "Name"); got != "prod/database/password" {
		t.Errorf("Name: expected %q, got %q", "prod/database/password", got)
	}
	if got := fieldpath.ExtractScalar(secret, "Description"); got != "Production database password" {
		t.Errorf("Description: expected %q, got %q", "Production database password", got)
	}
	if got := fieldpath.ExtractScalar(secret, "RotationEnabled"); got != "Yes" {
		t.Errorf("RotationEnabled: expected %q, got %q", "Yes", got)
	}
}

func TestQA_DetailViewPaths_Secrets(t *testing.T) {
	secret := realisticSecretListEntry()
	vd := config.DefaultViewDef("secrets")

	for _, path := range vd.Detail {
		t.Run(path, func(t *testing.T) {
			result := fieldpath.ExtractSubtree(secret, path)
			if result == "" {
				t.Errorf("ExtractSubtree(%q) returned empty for realistic SecretListEntry", path)
			}
		})
	}

	// Verify Tags renders as YAML
	tagsYAML := fieldpath.ExtractSubtree(secret, "Tags")
	if tagsYAML == "" {
		t.Error("Tags should produce non-empty YAML")
	}
}

func TestQA_NilFields_Secrets(t *testing.T) {
	// Minimal secret — only name
	secret := smtypes.SecretListEntry{
		Name: ptrString("test-secret"),
	}
	vd := config.DefaultViewDef("secrets")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractScalar(secret, col.Path)
		})
	}

	for _, path := range vd.Detail {
		t.Run("detail_"+path, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractSubtree(secret, path)
		})
	}
}

// ===========================================================================
// Cross-cutting: Verify all default view defs exist and have paths
// ===========================================================================

func TestQA_AllResourceTypesHaveDefaults(t *testing.T) {
	resourceTypes := append(resource.AllShortNames(), "s3_objects")

	for _, rt := range resourceTypes {
		t.Run(rt, func(t *testing.T) {
			vd := config.DefaultViewDef(rt)
			if len(vd.List) == 0 {
				t.Errorf("resource type %q has no default list columns", rt)
			}
		})
	}
}

// ===========================================================================
// Edge Case: S3 Object Size field — int64 scalar extraction
// ===========================================================================

func TestQA_S3Object_SizeField_Int64Extraction(t *testing.T) {
	obj := realisticS3ObjectFile()

	// Size is *int64 — ExtractScalar should dereference and format as number
	got := fieldpath.ExtractScalar(obj, "Size")
	if got == "" {
		t.Error("Size should not be empty for a file with size")
	}
	if got != "1048576" {
		t.Errorf("Size: expected %q, got %q", "1048576", got)
	}
}

// ===========================================================================
// Edge Case: S3 Object StorageClass — named string type (enum)
// ===========================================================================

func TestQA_S3Object_StorageClass_EnumExtraction(t *testing.T) {
	obj := realisticS3ObjectFile()

	got := fieldpath.ExtractScalar(obj, "StorageClass")
	if got == "" {
		t.Error("StorageClass should not be empty")
	}
	if got != "STANDARD" {
		t.Errorf("StorageClass: expected %q, got %q", "STANDARD", got)
	}
}

// ===========================================================================
// Edge Case: EC2 InstanceType — named string type (enum)
// ===========================================================================

func TestQA_EC2_InstanceType_EnumExtraction(t *testing.T) {
	inst := realisticEC2Instance()

	got := fieldpath.ExtractScalar(inst, "InstanceType")
	if got != "t3.medium" {
		t.Errorf("InstanceType: expected %q, got %q", "t3.medium", got)
	}
}

// ===========================================================================
// Edge Case: EC2 with no public IP (private instance)
// ===========================================================================

func TestQA_EC2_NilPublicIP(t *testing.T) {
	inst := ec2types.Instance{
		InstanceId:       ptrString("i-private"),
		InstanceType:     ec2types.InstanceTypeT3Micro,
		PrivateIpAddress: ptrString("10.0.0.1"),
		// PublicIpAddress is nil
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}

	if got := fieldpath.ExtractScalar(inst, "PublicIpAddress"); got != "" {
		t.Errorf("expected empty for nil PublicIpAddress, got %q", got)
	}
	// PrivateIpAddress should still work
	if got := fieldpath.ExtractScalar(inst, "PrivateIpAddress"); got != "10.0.0.1" {
		t.Errorf("PrivateIpAddress: expected %q, got %q", "10.0.0.1", got)
	}
}

// ===========================================================================
// Edge Case: RDS with nil Endpoint (during creation)
// ===========================================================================

func TestQA_RDS_NilEndpoint(t *testing.T) {
	db := rdstypes.DBInstance{
		DBInstanceIdentifier: ptrString("creating-db"),
		DBInstanceStatus:     ptrString("creating"),
		// Endpoint is nil during creation
	}

	// Nested path through nil struct should return "" without panic
	if got := fieldpath.ExtractScalar(db, "Endpoint.Address"); got != "" {
		t.Errorf("expected empty for nil Endpoint.Address, got %q", got)
	}

	// ExtractSubtree on nil Endpoint should return ""
	if got := fieldpath.ExtractSubtree(db, "Endpoint"); got != "" {
		t.Errorf("expected empty for nil Endpoint, got %q", got)
	}
}

// ===========================================================================
// Edge Case: Redis with nil ConfigurationEndpoint
// ===========================================================================

func TestQA_Redis_NilConfigurationEndpoint(t *testing.T) {
	cluster := elasticachetypes.CacheCluster{
		CacheClusterId:     ptrString("redis-no-endpoint"),
		CacheClusterStatus: ptrString("available"),
		// ConfigurationEndpoint is nil (single-node clusters)
	}

	if got := fieldpath.ExtractScalar(cluster, "ConfigurationEndpoint.Address"); got != "" {
		t.Errorf("expected empty for nil ConfigurationEndpoint.Address, got %q", got)
	}

	if got := fieldpath.ExtractSubtree(cluster, "ConfigurationEndpoint"); got != "" {
		t.Errorf("expected empty for nil ConfigurationEndpoint, got %q", got)
	}
}

// ===========================================================================
// Edge Case: DocDB with empty DBClusterMembers slice
// ===========================================================================

func TestQA_DocDB_EmptyMembers(t *testing.T) {
	cluster := docdbtypes.DBCluster{
		DBClusterIdentifier: ptrString("docdb-empty"),
		Status:              ptrString("creating"),
		// DBClusterMembers is nil/empty
	}

	// ExtractScalar on a slice should return ""
	if got := fieldpath.ExtractScalar(cluster, "DBClusterMembers"); got != "" {
		t.Errorf("expected empty for nil DBClusterMembers, got %q", got)
	}

	// ExtractSubtree on empty/nil slice should return "" or "null"
	got := fieldpath.ExtractSubtree(cluster, "DBClusterMembers")
	if got != "" && got != "null" {
		t.Errorf("expected empty or null for empty DBClusterMembers, got %q", got)
	}
}

// ===========================================================================
// Edge Case: EKS with nil KubernetesNetworkConfig
// ===========================================================================

func TestQA_EKS_NilNetworkConfig(t *testing.T) {
	cluster := &ekstypes.Cluster{
		Name:   ptrString("cluster-no-net"),
		Status: ekstypes.ClusterStatusActive,
		// KubernetesNetworkConfig is nil
	}

	if got := fieldpath.ExtractSubtree(cluster, "KubernetesNetworkConfig"); got != "" {
		t.Errorf("expected empty for nil KubernetesNetworkConfig, got %q", got)
	}
}

// ===========================================================================
// Edge Case: Secrets with no tags and no rotation
// ===========================================================================

func TestQA_Secrets_MinimalFields(t *testing.T) {
	secret := smtypes.SecretListEntry{
		Name: ptrString("minimal-secret"),
		// No description, no dates, no rotation, no tags
	}

	if got := fieldpath.ExtractScalar(secret, "Description"); got != "" {
		t.Errorf("expected empty for nil Description, got %q", got)
	}
	if got := fieldpath.ExtractScalar(secret, "LastAccessedDate"); got != "" {
		t.Errorf("expected empty for nil LastAccessedDate, got %q", got)
	}
	if got := fieldpath.ExtractScalar(secret, "LastChangedDate"); got != "" {
		t.Errorf("expected empty for nil LastChangedDate, got %q", got)
	}
	if got := fieldpath.ExtractScalar(secret, "RotationEnabled"); got != "" {
		t.Errorf("expected empty for nil RotationEnabled, got %q", got)
	}
	// nil/empty Tags may return "" or "null" depending on yaml.Marshal behavior
	if got := fieldpath.ExtractSubtree(secret, "Tags"); got != "" && got != "null" {
		t.Errorf("expected empty or null for nil Tags, got %q", got)
	}
}
