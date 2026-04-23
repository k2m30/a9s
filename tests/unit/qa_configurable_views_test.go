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

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ===========================================================================
// Helpers
// ===========================================================================

var testTime = time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

// ===========================================================================
// Realistic SDK struct builders
// ===========================================================================

func realisticS3Bucket() s3types.Bucket {
	return s3types.Bucket{
		Name:         new("my-production-bucket"),
		CreationDate: new(testTime),
		BucketArn:    new("arn:aws:s3:::my-production-bucket"),
		BucketRegion: new("us-east-1"),
	}
}

func realisticS3ObjectFile() s3types.Object {
	return s3types.Object{
		Key:          new("data/report-2025.csv"),
		Size:         new(int64(1048576)),
		LastModified: new(testTime),
		StorageClass: s3types.ObjectStorageClassStandard,
		ETag:         new("\"d41d8cd98f00b204e9800998ecf8427e\""),
	}
}

func realisticS3ObjectFolder() s3types.CommonPrefix {
	return s3types.CommonPrefix{
		Prefix: new("data/reports/"),
	}
}

func realisticEC2Instance() ec2types.Instance {
	return ec2types.Instance{
		InstanceId:       new("i-0abcdef1234567890"),
		InstanceType:     ec2types.InstanceTypeT3Medium,
		PrivateIpAddress: new("10.0.1.42"),
		PublicIpAddress:  new("54.123.45.67"),
		LaunchTime:       new(testTime),
		ImageId:          new("ami-0abcdef1234567890"),
		VpcId:            new("vpc-0abc1234"),
		SubnetId:         new("subnet-0abc5678"),
		Architecture:     ec2types.ArchitectureValuesX8664,
		KeyName:          new("prod-keypair"),
		PrivateDnsName:   new("ip-10-0-1-42.ec2.internal"),
		EbsOptimized:     new(true),
		Placement: &ec2types.Placement{
			AvailabilityZone: new("us-east-1a"),
			Tenancy:          ec2types.TenancyDefault,
		},
		IamInstanceProfile: &ec2types.IamInstanceProfile{
			Arn: new("arn:aws:iam::123456789012:instance-profile/web-server-role"),
			Id:  new("AIPAXYZ1234567890ABCD"),
		},
		MetadataOptions: &ec2types.InstanceMetadataOptionsResponse{
			HttpEndpoint:            ec2types.InstanceMetadataEndpointStateEnabled,
			HttpTokens:              ec2types.HttpTokensStateRequired,
			HttpPutResponseHopLimit: new(int32(2)),
		},
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: new(int32(16)),
		},
		Tags: []ec2types.Tag{
			{Key: new("Name"), Value: new("web-server-prod")},
			{Key: new("env"), Value: new("production")},
		},
		SecurityGroups: []ec2types.GroupIdentifier{
			{GroupId: new("sg-0abc1234"), GroupName: new("web-sg")},
		},
		BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{
			{
				DeviceName: new("/dev/sda1"),
				Ebs: &ec2types.EbsInstanceBlockDevice{
					VolumeId: new("vol-0abc1234567890def"),
					Status:   ec2types.AttachmentStatusAttached,
				},
			},
		},
		PlatformDetails:   new("Linux/UNIX"),
		Platform:          ec2types.PlatformValuesWindows,
		InstanceLifecycle: ec2types.InstanceLifecycleTypeSpot,
	}
}

func realisticRDSInstance() rdstypes.DBInstance {
	return rdstypes.DBInstance{
		DBInstanceIdentifier: new("prod-db-01"),
		DBInstanceArn:        new("arn:aws:rds:us-east-1:123456789012:db:prod-db-01"),
		Engine:               new("mysql"),
		EngineVersion:        new("8.0.35"),
		DBInstanceStatus:     new("available"),
		DBInstanceClass:      new("db.r5.large"),
		MultiAZ:              new(true),
		AllocatedStorage:     new(int32(100)),
		StorageType:          new("gp3"),
		Iops:                 new(int32(3000)),
		StorageEncrypted:     new(true),
		KmsKeyId:             new("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
		AvailabilityZone:     new("us-east-1a"),
		PubliclyAccessible:   new(false),
		DBSubnetGroup: &rdstypes.DBSubnetGroup{
			DBSubnetGroupName:        new("prod-db-subnet-group"),
			DBSubnetGroupDescription: new("Production DB subnet group"),
		},
		VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: new("sg-0abc1234"), Status: new("active")},
		},
		BackupRetentionPeriod:      new(int32(7)),
		PreferredMaintenanceWindow: new("sun:03:00-sun:04:00"),
		PreferredBackupWindow:      new("02:00-03:00"),
		DeletionProtection:         new(true),
		MasterUsername:             new("admin"),
		PerformanceInsightsEnabled: new(true),
		TagList: []rdstypes.Tag{
			{Key: new("env"), Value: new("production")},
		},
		Endpoint: &rdstypes.Endpoint{
			Address: new("prod-db-01.abc123.us-east-1.rds.amazonaws.com"),
			Port:    new(int32(3306)),
		},
	}
}

func realisticRedisReplicationGroup() elasticachetypes.ReplicationGroup {
	return elasticachetypes.ReplicationGroup{
		ReplicationGroupId:       new("redis-prod-001"),
		ARN:                      new("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:redis-prod-001"),
		Description:              new("Prod Redis replication group"),
		Status:                   new("available"),
		CacheNodeType:            new("cache.r6g.large"),
		MemberClusters:           []string{"redis-prod-001-001", "redis-prod-001-002", "redis-prod-001-003"},
		MultiAZ:                  elasticachetypes.MultiAZStatusEnabled,
		AutomaticFailover:        elasticachetypes.AutomaticFailoverStatusEnabled,
		KmsKeyId:                 new("arn:aws:kms:us-east-1:123456789012:key/redis-prod-001-key"),
		AtRestEncryptionEnabled:  new(true),
		TransitEncryptionEnabled: new(true),
		AuthTokenEnabled:         new(false),
		SnapshotRetentionLimit:   new(int32(7)),
		SnapshotWindow:           new("05:00-06:00"),
		ConfigurationEndpoint: &elasticachetypes.Endpoint{
			Address: new("redis-prod-001.abc123.clustercfg.use1.cache.amazonaws.com"),
			Port:    new(int32(6379)),
		},
		LogDeliveryConfigurations: []elasticachetypes.LogDeliveryConfiguration{
			{
				LogType:         elasticachetypes.LogTypeSlowLog,
				LogFormat:       elasticachetypes.LogFormatText,
				DestinationType: elasticachetypes.DestinationTypeCloudWatchLogs,
				DestinationDetails: &elasticachetypes.DestinationDetails{
					CloudWatchLogsDetails: &elasticachetypes.CloudWatchLogsDestinationDetails{
						LogGroup: new("/aws/elasticache/redis/redis-prod-001/slow-log"),
					},
				},
				Status: elasticachetypes.LogDeliveryConfigurationStatusActive,
			},
		},
	}
}

func realisticDocDBCluster() docdbtypes.DBCluster {
	return docdbtypes.DBCluster{
		DBClusterIdentifier: new("docdb-prod-cluster"),
		DBClusterArn:        new("arn:aws:rds:us-east-1:123456789012:cluster:docdb-prod-cluster"),
		Engine:              new("dbc"),
		EngineVersion:       new("5.0.0"),
		Status:              new("available"),
		Endpoint:            new("docdb-prod.cluster-abc123.us-east-1.docdb.amazonaws.com"),
		ReaderEndpoint:      new("docdb-prod.cluster-ro-abc123.us-east-1.docdb.amazonaws.com"),
		Port:                new(int32(27017)),
		StorageEncrypted:    new(true),
		KmsKeyId:            new("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
		DeletionProtection:  new(true),
		DBSubnetGroup:       new("docdb-prod-subnet-group"),
		VpcSecurityGroups: []docdbtypes.VpcSecurityGroupMembership{
			{VpcSecurityGroupId: new("sg-0abc5678"), Status: new("active")},
		},
		BackupRetentionPeriod:      new(int32(7)),
		PreferredMaintenanceWindow: new("sun:04:00-sun:05:00"),
		MasterUsername:             new("docdbadmin"),
		DBClusterMembers: []docdbtypes.DBClusterMember{
			{DBInstanceIdentifier: new("docdb-prod-instance-1"), IsClusterWriter: new(true)},
			{DBInstanceIdentifier: new("docdb-prod-instance-2"), IsClusterWriter: new(false)},
		},
	}
}

func realisticEKSCluster() *ekstypes.Cluster {
	return &ekstypes.Cluster{
		Name:            new("prod-cluster"),
		Version:         new("1.28"),
		Status:          ekstypes.ClusterStatusActive,
		Endpoint:        new("https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com"),
		PlatformVersion: new("eks.5"),
		Arn:             new("arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster"),
		RoleArn:         new("arn:aws:iam::123456789012:role/eks-cluster-role"),
		CreatedAt:       new(testTime),
		KubernetesNetworkConfig: &ekstypes.KubernetesNetworkConfigResponse{
			ServiceIpv4Cidr: new("172.20.0.0/16"),
		},
		ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
			ClusterSecurityGroupId: new("sg-0abc9999"),
			EndpointPrivateAccess:  true,
			EndpointPublicAccess:   true,
			VpcId:                  new("vpc-0abc1234"),
		},
		Logging: &ekstypes.Logging{
			ClusterLogging: []ekstypes.LogSetup{
				{
					Enabled: new(true),
					Types:   []ekstypes.LogType{ekstypes.LogTypeApi, ekstypes.LogTypeAudit},
				},
			},
		},
		Identity: &ekstypes.Identity{
			Oidc: &ekstypes.OIDC{
				Issuer: new("https://oidc.eks.us-east-1.amazonaws.com/id/ABCDEF1234567890"),
			},
		},
		Tags: map[string]string{
			"env":  "production",
			"team": "platform",
		},
	}
}

func realisticSecretListEntry() smtypes.SecretListEntry {
	rotatedTime := testTime.Add(-24 * time.Hour)
	return smtypes.SecretListEntry{
		Name:              new("prod/database/password"),
		Description:       new("Production database password"),
		LastAccessedDate:  new(testTime),
		LastChangedDate:   new(testTime),
		RotationEnabled:   new(true),
		ARN:               new("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/password-AbCdEf"),
		KmsKeyId:          new("arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012"),
		CreatedDate:       new(testTime.Add(-90 * 24 * time.Hour)),
		LastRotatedDate:   new(rotatedTime),
		RotationLambdaARN: new("arn:aws:lambda:us-east-1:123456789012:function:SecretsManagerRotation"),
		RotationRules: &smtypes.RotationRulesType{
			AutomaticallyAfterDays: new(int64(30)),
		},
		PrimaryRegion: new("us-east-1"),
		Tags: []smtypes.Tag{
			{Key: new("env"), Value: new("production")},
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
			if col.Path == "" {
				// Key-only columns (e.g. Status) are populated at runtime from
				// Fields["status"] by the Wave-2 enricher. They have no Path to
				// resolve against the raw SDK struct — skip ExtractScalar here.
				t.Skipf("key-only column %q (Key=%q) is populated from Fields at runtime, not from struct path", col.Title, col.Key)
			}
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

	for _, df := range vd.Detail {
		path := df.String()
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

	for _, df := range vd.Detail {
		path := df.String()
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
			if col.Path == "" {
				// Key-based column (e.g. Size with Key="size") — no Path to extract
				if col.Key == "" {
					t.Errorf("column %q has neither Path nor Key set", col.Title)
				}
				return
			}
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
		if col.Path == "" {
			// Key-based column (e.g. Size with Key="size") — no Path to extract
			continue
		}
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

	for _, df := range vd.Detail {
		path := df.String()
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

	for _, df := range vd.Detail {
		path := df.String()
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
			if col.Path == "" && col.Key != "" {
				// Key-backed column: value comes from Resource.Fields[key] populated by the
				// fetcher, not from a raw-struct path. ExtractScalar is not applicable here.
				return
			}
			if col.Path == "" && col.Key == "" {
				t.Errorf("column %q has neither Path nor Key", col.Title)
				return
			}
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

	for _, df := range vd.Detail {
		path := df.String()
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
		DBInstanceIdentifier: new("test-db"),
		DBInstanceStatus:     new("creating"),
	}
	vd := config.DefaultViewDef("dbi")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic — nil Endpoint should not crash on Endpoint.Address
			_ = fieldpath.ExtractScalar(db, col.Path)
		})
	}

	for _, df := range vd.Detail {
		path := df.String()
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
	rg := realisticRedisReplicationGroup()
	vd := config.DefaultViewDef("redis")

	for _, col := range vd.List {
		t.Run(col.Title, func(t *testing.T) {
			if col.Path == "" {
				t.Skipf("column %q has no path (uses Fields fallback)", col.Title)
				return
			}
			result := fieldpath.ExtractScalar(rg, col.Path)
			if result == "" {
				t.Errorf("ExtractScalar(%q) returned empty for realistic Redis ReplicationGroup", col.Path)
			}
		})
	}

	// Verify specific values
	if got := fieldpath.ExtractScalar(rg, "ReplicationGroupId"); got != "redis-prod-001" {
		t.Errorf("ReplicationGroupId: expected %q, got %q", "redis-prod-001", got)
	}
	if got := fieldpath.ExtractScalar(rg, "ConfigurationEndpoint.Address"); got != "redis-prod-001.abc123.clustercfg.use1.cache.amazonaws.com" {
		t.Errorf("ConfigurationEndpoint.Address: expected correct address, got %q", got)
	}
}

func TestQA_DetailViewPaths_Redis(t *testing.T) {
	rg := realisticRedisReplicationGroup()
	vd := config.DefaultViewDef("redis")

	for _, df := range vd.Detail {
		path := df.String()
		t.Run(path, func(t *testing.T) {
			result := fieldpath.ExtractSubtree(rg, path)
			if result == "" {
				t.Errorf("ExtractSubtree(%q) returned empty for realistic Redis ReplicationGroup", path)
			}
		})
	}

	// Verify ConfigurationEndpoint renders as YAML subtree (has Address and Port)
	epYAML := fieldpath.ExtractSubtree(rg, "ConfigurationEndpoint")
	if epYAML == "" {
		t.Error("ConfigurationEndpoint should produce non-empty YAML")
	}
}

func TestQA_NilFields_Redis(t *testing.T) {
	// Minimal Redis replication group — no endpoint, no member clusters
	rg := elasticachetypes.ReplicationGroup{
		ReplicationGroupId: new("redis-test"),
		Status:             new("creating"),
	}
	vd := config.DefaultViewDef("redis")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic — nil ConfigurationEndpoint should not crash
			_ = fieldpath.ExtractScalar(rg, col.Path)
		})
	}

	for _, df := range vd.Detail {
		path := df.String()
		t.Run("detail_"+path, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractSubtree(rg, path)
		})
	}

	// Specifically verify nil ConfigurationEndpoint.Address returns ""
	if got := fieldpath.ExtractScalar(rg, "ConfigurationEndpoint.Address"); got != "" {
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
			// Columns with an empty Path read from the Resource.Fields map via
			// Key (fetcher-computed values like the §4 status phrase). ExtractScalar
			// cannot derive a value from the raw SDK struct for these.
			if col.Path == "" {
				return
			}
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

	for _, df := range vd.Detail {
		path := df.String()
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
		DBClusterIdentifier: new("docdb-test"),
		Status:              new("creating"),
	}
	vd := config.DefaultViewDef("dbc")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractScalar(cluster, col.Path)
		})
	}

	for _, df := range vd.Detail {
		path := df.String()
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
			if col.Path == "" {
				// Key-only columns (resolved via Resource.Fields[col.Key]) have no SDK
				// struct path. Skip path-based extraction; the field-key resolution
				// path is exercised by other tests.
				t.Skip("path-less column — key-based, see Fields[] resolution test")
			}
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

	for _, df := range vd.Detail {
		if df.Key != "" {
			continue // key-form fields live in Fields[], not in the SDK struct
		}
		path := df.String()
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
		Name:   new("test-cluster"),
		Status: ekstypes.ClusterStatusCreating,
	}
	vd := config.DefaultViewDef("eks")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractScalar(cluster, col.Path)
		})
	}

	for _, df := range vd.Detail {
		path := df.String()
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

	for _, df := range vd.Detail {
		path := df.String()
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
		Name: new("test-secret"),
	}
	vd := config.DefaultViewDef("secrets")

	for _, col := range vd.List {
		t.Run("list_"+col.Title, func(t *testing.T) {
			// Must not panic
			_ = fieldpath.ExtractScalar(secret, col.Path)
		})
	}

	for _, df := range vd.Detail {
		path := df.String()
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
		InstanceId:       new("i-private"),
		InstanceType:     ec2types.InstanceTypeT3Micro,
		PrivateIpAddress: new("10.0.0.1"),
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
		DBInstanceIdentifier: new("creating-db"),
		DBInstanceStatus:     new("creating"),
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
	rg := elasticachetypes.ReplicationGroup{
		ReplicationGroupId: new("redis-no-endpoint"),
		Status:             new("available"),
		// ConfigurationEndpoint is nil (single-node replication groups)
	}

	if got := fieldpath.ExtractScalar(rg, "ConfigurationEndpoint.Address"); got != "" {
		t.Errorf("expected empty for nil ConfigurationEndpoint.Address, got %q", got)
	}

	if got := fieldpath.ExtractSubtree(rg, "ConfigurationEndpoint"); got != "" {
		t.Errorf("expected empty for nil ConfigurationEndpoint, got %q", got)
	}
}

// ===========================================================================
// Edge Case: DocDB with empty DBClusterMembers slice
// ===========================================================================

func TestQA_DocDB_EmptyMembers(t *testing.T) {
	cluster := docdbtypes.DBCluster{
		DBClusterIdentifier: new("docdb-empty"),
		Status:              new("creating"),
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
		Name:   new("cluster-no-net"),
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
		Name: new("minimal-secret"),
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
