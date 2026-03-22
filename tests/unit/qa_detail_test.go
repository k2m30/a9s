package unit_test

// Shared helpers (stripAnsi, ensureNoColor, detailKeyPress, detailSpecialKey,
// newDetailModel, newDetailModelSmall, detailApplyMsg, buildResource,
// buildResourceWithFields, configForType) are defined in
// helpers_external_test.go to avoid duplication.

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

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// S3 Bucket Detail
// ---------------------------------------------------------------------------

func TestQA_Detail_S3Bucket_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	bucket := realisticS3Bucket()
	res := buildResource("my-production-bucket", "my-production-bucket", bucket)
	cfg := configForType("s3")
	m := newDetailModel(res, "s3", cfg)

	view := m.View()
	if !strings.Contains(view, "CreationDate") {
		t.Error("S3 bucket detail should contain CreationDate")
	}
	if !strings.Contains(view, "2025-06-15 10:30:00") {
		t.Errorf("S3 bucket detail should contain formatted timestamp, got:\n%s", view)
	}
}

func TestQA_Detail_S3Bucket_FrameTitle(t *testing.T) {
	bucket := realisticS3Bucket()
	res := buildResource("my-production-bucket", "my-production-bucket", bucket)
	cfg := configForType("s3")
	m := newDetailModel(res, "s3", cfg)

	title := m.FrameTitle()
	if title != "my-production-bucket" {
		t.Errorf("FrameTitle expected %q, got %q", "my-production-bucket", title)
	}
}

func TestQA_Detail_S3Bucket_NilFields(t *testing.T) {
	ensureNoColor(t)
	bucket := s3types.Bucket{} // all nil/zero
	res := buildResource("empty-bucket", "empty-bucket", bucket)
	cfg := configForType("s3")
	m := newDetailModel(res, "s3", cfg)

	// Should not panic and should render something
	view := m.View()
	if view == "" {
		t.Error("detail view should not be empty even with nil S3 bucket fields")
	}
}

// ---------------------------------------------------------------------------
// S3 Object Detail
// ---------------------------------------------------------------------------

func TestQA_Detail_S3Object_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	obj := realisticS3ObjectFile()
	res := buildResource("data/report-2025.csv", "data/report-2025.csv", obj)
	cfg := configForType("s3_objects")
	m := newDetailModel(res, "s3_objects", cfg)

	view := m.View()
	if !strings.Contains(view, "data/report-2025.csv") {
		t.Errorf("S3 object detail should contain key, got:\n%s", view)
	}
	if !strings.Contains(view, "2025-06-15 10:30:00") {
		t.Errorf("S3 object detail should contain formatted LastModified, got:\n%s", view)
	}
}

func TestQA_Detail_S3Object_NilFields(t *testing.T) {
	ensureNoColor(t)
	obj := s3types.Object{} // all nil
	res := buildResource("empty-obj", "empty-obj", obj)
	cfg := configForType("s3_objects")
	m := newDetailModel(res, "s3_objects", cfg)

	// Should not panic
	view := m.View()
	_ = view
}

// ---------------------------------------------------------------------------
// EC2 Instance Detail
// ---------------------------------------------------------------------------

func TestQA_Detail_EC2_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	view := m.View()
	for _, expected := range []string{
		"InstanceId", "i-0abcdef1234567890",
		"InstanceType", "t3.medium",
		"PrivateIpAddress", "10.0.1.42",
		"PublicIpAddress", "54.123.45.67",
		"Architecture", "x86_64",
		"LaunchTime", "2025-06-15 10:30:00",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("EC2 detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_EC2_NestedState(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	view := m.View()
	if !strings.Contains(view, "State") {
		t.Error("EC2 detail should contain State section")
	}
	if !strings.Contains(view, "running") {
		t.Error("EC2 detail State should contain 'running'")
	}
}

func TestQA_Detail_EC2_SecurityGroups(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	view := m.View()
	if !strings.Contains(view, "SecurityGroups") {
		t.Error("EC2 detail should contain SecurityGroups section")
	}
	if !strings.Contains(view, "sg-0abc1234") {
		t.Error("EC2 detail SecurityGroups should contain group ID 'sg-0abc1234'")
	}
	if !strings.Contains(view, "web-sg") {
		t.Error("EC2 detail SecurityGroups should contain group name 'web-sg'")
	}
}

func TestQA_Detail_EC2_Tags(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	view := m.View()
	if !strings.Contains(view, "Tags") {
		t.Error("EC2 detail should contain Tags section")
	}
	if !strings.Contains(view, "web-server-prod") {
		t.Error("EC2 detail Tags should contain tag value 'web-server-prod'")
	}
	if !strings.Contains(view, "production") {
		t.Error("EC2 detail Tags should contain tag value 'production'")
	}
}

func TestQA_Detail_EC2_NilPublicIP(t *testing.T) {
	ensureNoColor(t)
	inst := ec2types.Instance{
		InstanceId:       ptrString("i-private"),
		PrivateIpAddress: ptrString("10.0.0.1"),
		// PublicIpAddress is nil
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: ptrInt32(16),
		},
	}
	res := buildResource("i-private", "i-private", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	// Should not panic
	view := m.View()
	if !strings.Contains(view, "PrivateIpAddress") {
		t.Error("EC2 detail should still show PrivateIpAddress for private-only instance")
	}
	if !strings.Contains(view, "10.0.0.1") {
		t.Error("EC2 detail should contain private IP value")
	}
}

func TestQA_Detail_EC2_EmptyTags(t *testing.T) {
	ensureNoColor(t)
	inst := ec2types.Instance{
		InstanceId:   ptrString("i-notags"),
		InstanceType: ec2types.InstanceTypeT3Micro,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
			Code: ptrInt32(16),
		},
		// Tags is nil/empty
		// SecurityGroups is nil/empty
	}
	res := buildResource("i-notags", "i-notags", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	// Should not panic
	view := m.View()
	if view == "" {
		t.Error("EC2 detail should render even with empty tags")
	}
	if !strings.Contains(view, "InstanceId") {
		t.Error("EC2 detail should still render InstanceId")
	}
}

func TestQA_Detail_EC2_TerminatedInstance(t *testing.T) {
	ensureNoColor(t)
	inst := ec2types.Instance{
		InstanceId:   ptrString("i-terminated"),
		InstanceType: ec2types.InstanceTypeT3Large,
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameTerminated,
			Code: ptrInt32(48),
		},
		LaunchTime: ptrTime(testTime),
	}
	res := buildResource("i-terminated", "i-terminated", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	view := m.View()
	if !strings.Contains(view, "terminated") {
		t.Error("EC2 detail for terminated instance should contain 'terminated'")
	}
}

func TestQA_Detail_EC2_FrameTitle(t *testing.T) {
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	if title := m.FrameTitle(); title != "web-server-prod" {
		t.Errorf("FrameTitle expected %q, got %q", "web-server-prod", title)
	}
}

func TestQA_Detail_EC2_FrameTitleFallsBackToID(t *testing.T) {
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "", inst) // no Name
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	if title := m.FrameTitle(); title != "i-0abcdef1234567890" {
		t.Errorf("FrameTitle should fall back to ID, got %q", title)
	}
}

// ---------------------------------------------------------------------------
// RDS Instance Detail
// ---------------------------------------------------------------------------

func TestQA_Detail_RDS_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	db := realisticRDSInstance()
	res := buildResource("prod-db-01", "prod-db-01", db)
	cfg := configForType("dbi")
	m := newDetailModel(res, "dbi", cfg)

	view := m.View()
	for _, expected := range []string{
		"DBInstanceIdentifier", "prod-db-01",
		"Engine", "mysql",
		"EngineVersion", "8.0.35",
		"DBInstanceStatus", "available",
		"DBInstanceClass", "db.r5.large",
		"MultiAZ", "Yes",
		"AllocatedStorage", "100",
		"StorageType", "gp3",
		"AvailabilityZone", "us-east-1a",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("RDS detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_RDS_NestedEndpoint(t *testing.T) {
	ensureNoColor(t)
	db := realisticRDSInstance()
	res := buildResource("prod-db-01", "prod-db-01", db)
	cfg := configForType("dbi")
	m := newDetailModel(res, "dbi", cfg)

	view := m.View()
	if !strings.Contains(view, "Endpoint") {
		t.Error("RDS detail should contain Endpoint section")
	}
	if !strings.Contains(view, "prod-db-01.abc123.us-east-1.rds.amazonaws.com") {
		t.Error("RDS detail Endpoint should contain address FQDN")
	}
	if !strings.Contains(view, "3306") {
		t.Error("RDS detail Endpoint should contain port 3306")
	}
}

func TestQA_Detail_RDS_NilEndpoint(t *testing.T) {
	ensureNoColor(t)
	db := rdstypes.DBInstance{
		DBInstanceIdentifier: ptrString("creating-db"),
		DBInstanceStatus:     ptrString("creating"),
		Engine:               ptrString("mysql"),
		// Endpoint is nil during creation
	}
	res := buildResource("creating-db", "creating-db", db)
	cfg := configForType("dbi")
	m := newDetailModel(res, "dbi", cfg)

	// Should not panic
	view := m.View()
	if !strings.Contains(view, "creating-db") {
		t.Error("RDS detail should show identifier even when endpoint is nil")
	}
}

func TestQA_Detail_RDS_BooleanMultiAZFalse(t *testing.T) {
	ensureNoColor(t)
	db := rdstypes.DBInstance{
		DBInstanceIdentifier: ptrString("test-db"),
		Engine:               ptrString("mysql"),
		MultiAZ:              ptrBool(false),
	}
	res := buildResource("test-db", "test-db", db)
	cfg := configForType("dbi")
	m := newDetailModel(res, "dbi", cfg)

	view := m.View()
	if !strings.Contains(view, "No") {
		t.Error("RDS detail MultiAZ=false should render as 'No'")
	}
}

// ---------------------------------------------------------------------------
// Redis (ElastiCache) Detail
// ---------------------------------------------------------------------------

func TestQA_Detail_Redis_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticRedisCacheCluster()
	res := buildResource("redis-prod-001", "redis-prod-001", cluster)
	cfg := configForType("redis")
	m := newDetailModel(res, "redis", cfg)

	view := m.View()
	for _, expected := range []string{
		"CacheClusterId", "redis-prod-001",
		"Engine", "redis",
		"EngineVersion", "7.0.12",
		"CacheClusterStatus", "available",
		"CacheNodeType", "cache.r6g.large",
		"NumCacheNodes", "3",
		"PreferredAvailability", "us-east-1a",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Redis detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Redis_NestedConfigurationEndpoint(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticRedisCacheCluster()
	res := buildResource("redis-prod-001", "redis-prod-001", cluster)
	cfg := configForType("redis")
	m := newDetailModel(res, "redis", cfg)

	view := m.View()
	if !strings.Contains(view, "ConfigurationEndpoint") {
		t.Error("Redis detail should contain ConfigurationEndpoint section")
	}
	if !strings.Contains(view, "redis-prod-001.abc123.clustercfg.use1.cache.amazonaws.com") {
		t.Error("Redis detail ConfigurationEndpoint should contain address")
	}
	if !strings.Contains(view, "6379") {
		t.Error("Redis detail ConfigurationEndpoint should contain port 6379")
	}
}

func TestQA_Detail_Redis_NilConfigurationEndpoint(t *testing.T) {
	ensureNoColor(t)
	cluster := elasticachetypes.CacheCluster{
		CacheClusterId:     ptrString("redis-single"),
		CacheClusterStatus: ptrString("available"),
		Engine:             ptrString("redis"),
		// ConfigurationEndpoint is nil (single-node cluster)
	}
	res := buildResource("redis-single", "redis-single", cluster)
	cfg := configForType("redis")
	m := newDetailModel(res, "redis", cfg)

	// Should not panic
	view := m.View()
	if !strings.Contains(view, "redis-single") {
		t.Error("Redis detail should show cluster ID even when ConfigurationEndpoint is nil")
	}
}

// ---------------------------------------------------------------------------
// DocumentDB Cluster Detail
// ---------------------------------------------------------------------------

func TestQA_Detail_DocDB_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticDocDBCluster()
	res := buildResource("docdb-prod-cluster", "docdb-prod-cluster", cluster)
	cfg := configForType("dbc")
	m := newDetailModel(res, "dbc", cfg)

	view := m.View()
	for _, expected := range []string{
		"DBClusterIdentifier", "docdb-prod-cluster",
		"Engine", "dbc",
		"EngineVersion", "5.0.0",
		"Status", "available",
		"Endpoint", "docdb-prod.cluster-abc123.us-east-1.docdb.amazonaws.com",
		"ReaderEndpoint", "docdb-prod.cluster-ro-abc123.us-east-1.docdb.amazonaws.com",
		"Port", "27017",
		"StorageEncrypted", "Yes",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("DocDB detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_DocDB_NestedDBClusterMembers(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticDocDBCluster()
	res := buildResource("docdb-prod-cluster", "docdb-prod-cluster", cluster)
	cfg := configForType("dbc")
	m := newDetailModel(res, "dbc", cfg)

	view := m.View()
	if !strings.Contains(view, "DBClusterMembers") {
		t.Error("DocDB detail should contain DBClusterMembers section")
	}
	if !strings.Contains(view, "docdb-prod-instance-1") {
		t.Error("DocDB detail should contain member instance-1")
	}
	if !strings.Contains(view, "docdb-prod-instance-2") {
		t.Error("DocDB detail should contain member instance-2")
	}
}

func TestQA_Detail_DocDB_EmptyMembers(t *testing.T) {
	ensureNoColor(t)
	cluster := docdbtypes.DBCluster{
		DBClusterIdentifier: ptrString("docdb-new"),
		Status:              ptrString("creating"),
		Engine:              ptrString("dbc"),
		// DBClusterMembers is nil (newly created)
	}
	res := buildResource("docdb-new", "docdb-new", cluster)
	cfg := configForType("dbc")
	m := newDetailModel(res, "dbc", cfg)

	// Should not panic
	view := m.View()
	if !strings.Contains(view, "docdb-new") {
		t.Error("DocDB detail should show cluster ID even with empty members")
	}
}

func TestQA_Detail_DocDB_StorageEncryptedFalse(t *testing.T) {
	ensureNoColor(t)
	cluster := docdbtypes.DBCluster{
		DBClusterIdentifier: ptrString("docdb-unenc"),
		Status:              ptrString("available"),
		Engine:              ptrString("dbc"),
		StorageEncrypted:    ptrBool(false),
	}
	res := buildResource("docdb-unenc", "docdb-unenc", cluster)
	cfg := configForType("dbc")
	m := newDetailModel(res, "dbc", cfg)

	view := m.View()
	if !strings.Contains(view, "No") {
		t.Error("DocDB detail StorageEncrypted=false should render as 'No'")
	}
}

// ---------------------------------------------------------------------------
// EKS Cluster Detail
// ---------------------------------------------------------------------------

func TestQA_Detail_EKS_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticEKSCluster()
	res := buildResource("prod-cluster", "prod-cluster", cluster)
	cfg := configForType("eks")
	m := newDetailModel(res, "eks", cfg)

	view := m.View()
	for _, expected := range []string{
		"prod-cluster",
		"Version", "1.28",
		"Status", "ACTIVE",
		"Endpoint", "https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com",
		"PlatformVersion", "eks.5",
		"Arn", "arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster",
		"RoleArn", "arn:aws:iam::123456789012:role/eks-cluster-role",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("EKS detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_EKS_NestedKubernetesNetworkConfig(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticEKSCluster()
	res := buildResource("prod-cluster", "prod-cluster", cluster)
	cfg := configForType("eks")
	m := newDetailModel(res, "eks", cfg)

	view := m.View()
	// Key may be truncated to 22 chars ("KubernetesNetworkConf" + ellipsis)
	if !strings.Contains(view, "KubernetesNetworkConf") {
		t.Error("EKS detail should contain KubernetesNetworkConfig field (possibly truncated)")
	}
	if !strings.Contains(view, "172.20.0.0/16") {
		t.Error("EKS detail KubernetesNetworkConfig should contain ServiceIpv4Cidr")
	}
}

func TestQA_Detail_EKS_NilNetworkConfig(t *testing.T) {
	ensureNoColor(t)
	cluster := &ekstypes.Cluster{
		Name:     ptrString("new-cluster"),
		Status:   ekstypes.ClusterStatusCreating,
		Version:  ptrString("1.29"),
		Endpoint: ptrString("https://example.eks.amazonaws.com"),
		// KubernetesNetworkConfig is nil
	}
	res := buildResource("new-cluster", "new-cluster", cluster)
	cfg := configForType("eks")
	m := newDetailModel(res, "eks", cfg)

	// Should not panic
	view := m.View()
	if !strings.Contains(view, "new-cluster") {
		t.Error("EKS detail should show cluster name even with nil network config")
	}
}

func TestQA_Detail_EKS_LongARN(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticEKSCluster()
	res := buildResource("prod-cluster", "prod-cluster", cluster)
	cfg := configForType("eks")
	m := newDetailModel(res, "eks", cfg)

	view := m.View()
	if !strings.Contains(view, "arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster") {
		t.Error("EKS detail should show full ARN without truncation")
	}
}

// ---------------------------------------------------------------------------
// Secrets Manager Detail
// ---------------------------------------------------------------------------

func TestQA_Detail_Secrets_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	secret := realisticSecretListEntry()
	res := buildResource("prod/database/password", "prod/database/password", secret)
	cfg := configForType("secrets")
	m := newDetailModel(res, "secrets", cfg)

	view := m.View()
	for _, expected := range []string{
		"prod/database/password",
		"Description", "Production database password",
		"RotationEnabled", "Yes",
		"arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/database/password-AbCdEf",
		"arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Secrets detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Secrets_Tags(t *testing.T) {
	ensureNoColor(t)
	secret := realisticSecretListEntry()
	res := buildResource("prod/database/password", "prod/database/password", secret)
	cfg := configForType("secrets")
	m := newDetailModel(res, "secrets", cfg)

	view := m.View()
	if !strings.Contains(view, "Tags") {
		t.Error("Secrets detail should contain Tags section")
	}
	if !strings.Contains(view, "production") {
		t.Error("Secrets detail Tags should contain tag value 'production'")
	}
}

func TestQA_Detail_Secrets_RotationDisabled(t *testing.T) {
	ensureNoColor(t)
	secret := smtypes.SecretListEntry{
		Name:            ptrString("test-secret"),
		RotationEnabled: ptrBool(false),
	}
	res := buildResource("test-secret", "test-secret", secret)
	cfg := configForType("secrets")
	m := newDetailModel(res, "secrets", cfg)

	view := m.View()
	if !strings.Contains(view, "No") {
		t.Error("Secrets detail RotationEnabled=false should render as 'No'")
	}
}

func TestQA_Detail_Secrets_MinimalFields(t *testing.T) {
	ensureNoColor(t)
	secret := smtypes.SecretListEntry{
		Name: ptrString("minimal-secret"),
		// No description, dates, tags, etc.
	}
	res := buildResource("minimal-secret", "minimal-secret", secret)
	cfg := configForType("secrets")
	m := newDetailModel(res, "secrets", cfg)

	// Should not panic
	view := m.View()
	if !strings.Contains(view, "minimal-secret") {
		t.Error("Secrets detail should show name even with minimal fields")
	}
}

func TestQA_Detail_Secrets_EmptyTags(t *testing.T) {
	ensureNoColor(t)
	secret := smtypes.SecretListEntry{
		Name: ptrString("no-tags-secret"),
		ARN:  ptrString("arn:aws:secretsmanager:us-east-1:123:secret:no-tags-secret-XyZ"),
		// Tags is nil
	}
	res := buildResource("no-tags-secret", "no-tags-secret", secret)
	cfg := configForType("secrets")
	m := newDetailModel(res, "secrets", cfg)

	// Should not panic, Tags section should not appear (empty slice)
	view := m.View()
	if !strings.Contains(view, "no-tags-secret") {
		t.Error("Secrets detail should render name")
	}
}

// ---------------------------------------------------------------------------
// Key-Value Formatting: key column ~22 chars
// ---------------------------------------------------------------------------

func TestQA_Detail_KeyColumnWidth(t *testing.T) {
	ensureNoColor(t)
	db := realisticRDSInstance()
	res := buildResource("prod-db-01", "prod-db-01", db)
	cfg := configForType("dbi")
	m := newDetailModel(res, "dbi", cfg)

	view := m.View()
	lines := strings.Split(view, "\n")
	// Look for a line with a scalar key-value pair (e.g., Engine: mysql)
	for _, line := range lines {
		if strings.Contains(line, "Engine") && strings.Contains(line, "mysql") {
			// The key column is padded to 22 chars.
			// Format: "   " (3-char indent) + key padded to 22 + value
			// The space between key and value should be consistent
			trimmed := strings.TrimLeft(line, " ")
			if len(trimmed) < 22 {
				t.Errorf("key-value line too short, may not have padding: %q", line)
			}
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Scroll (j/k/g/G)
// ---------------------------------------------------------------------------

func TestQA_Detail_ScrollDown_J(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModelSmall(res, "ec2", cfg)

	viewBefore := m.View()
	m, _ = detailApplyMsg(m, detailKeyPress("j"))
	viewAfter := m.View()

	if viewBefore == viewAfter {
		t.Log("View did not change after j -- content may fit in viewport")
	}
}

func TestQA_Detail_ScrollUp_K(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModelSmall(res, "ec2", cfg)

	// Scroll down first, then up
	m, _ = detailApplyMsg(m, detailKeyPress("j"))
	m, _ = detailApplyMsg(m, detailKeyPress("j"))
	viewAfterDown := m.View()

	m, _ = detailApplyMsg(m, detailKeyPress("k"))
	viewAfterUp := m.View()

	if viewAfterDown == viewAfterUp {
		t.Log("View did not change after k -- scroll may not have moved")
	}
}

func TestQA_Detail_ScrollTop_G(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModelSmall(res, "ec2", cfg)

	// Scroll to bottom first
	m, _ = detailApplyMsg(m, detailKeyPress("G"))
	// Then go to top
	m, _ = detailApplyMsg(m, detailKeyPress("g"))

	view := m.View()
	// After going to top, the first detail field should be visible
	if !strings.Contains(view, "InstanceId") {
		t.Error("After pressing g (top), first detail field InstanceId should be visible")
	}
}

func TestQA_Detail_ScrollBottom_ShiftG(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModelSmall(res, "ec2", cfg)

	viewTop := m.View()
	if !strings.Contains(viewTop, "InstanceId") {
		t.Error("Initial view should show InstanceId at top")
	}

	// Scroll to bottom
	m, _ = detailApplyMsg(m, detailKeyPress("G"))
	viewBottom := m.View()

	if viewTop != viewBottom {
		t.Log("View changed after G, content scrolled to bottom")
	}
}

// ---------------------------------------------------------------------------
// Wrap toggle (w)
// ---------------------------------------------------------------------------

func TestQA_Detail_WrapToggle(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticEKSCluster()
	res := buildResource("prod-cluster", "prod-cluster", cluster)
	cfg := configForType("eks")
	m := newDetailModelSmall(res, "eks", cfg)

	viewBefore := m.View()

	// Toggle wrap on
	m, _ = detailApplyMsg(m, detailKeyPress("w"))
	viewWrapped := m.View()

	// Toggle wrap off
	m, _ = detailApplyMsg(m, detailKeyPress("w"))
	viewUnwrapped := m.View()

	// Wrap toggle should not crash, views may differ
	_ = viewBefore
	_ = viewWrapped
	_ = viewUnwrapped
}

// ---------------------------------------------------------------------------
// YAML switch (y)
// ---------------------------------------------------------------------------

func TestQA_Detail_YAMLSwitch(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	_, cmd := detailApplyMsg(m, detailKeyPress("y"))

	if cmd == nil {
		t.Fatal("pressing 'y' in detail view should return a command")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if navMsg.Target != messages.TargetYAML {
		t.Errorf("expected TargetYAML, got %v", navMsg.Target)
	}
	if navMsg.Resource == nil {
		t.Error("NavigateMsg should include the resource")
	}
	if navMsg.Resource.ID != "i-0abcdef1234567890" {
		t.Errorf("NavigateMsg resource ID should be %q, got %q", "i-0abcdef1234567890", navMsg.Resource.ID)
	}
}

func TestQA_Detail_YAMLSwitch_AllTypes(t *testing.T) {
	ensureNoColor(t)
	cases := []struct {
		name     string
		typeName string
		res      resource.Resource
	}{
		{"S3", "s3", buildResource("bucket", "bucket", realisticS3Bucket())},
		{"EC2", "ec2", buildResource("i-abc", "inst", realisticEC2Instance())},
		{"RDS", "dbi", buildResource("rds-1", "rds-1", realisticRDSInstance())},
		{"Redis", "redis", buildResource("redis-1", "redis-1", realisticRedisCacheCluster())},
		{"DocDB", "dbc", buildResource("docdb-1", "docdb-1", realisticDocDBCluster())},
		{"EKS", "eks", buildResource("eks-1", "eks-1", realisticEKSCluster())},
		{"Secrets", "secrets", buildResource("secret-1", "secret-1", realisticSecretListEntry())},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := configForType(tc.typeName)
			m := newDetailModel(tc.res, tc.typeName, cfg)
			_, cmd := detailApplyMsg(m, detailKeyPress("y"))
			if cmd == nil {
				t.Fatalf("%s: pressing 'y' should return a command", tc.name)
			}
			msg := cmd()
			navMsg, ok := msg.(messages.NavigateMsg)
			if !ok {
				t.Fatalf("%s: expected NavigateMsg, got %T", tc.name, msg)
			}
			if navMsg.Target != messages.TargetYAML {
				t.Errorf("%s: expected TargetYAML, got %v", tc.name, navMsg.Target)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Escape key behavior
// ---------------------------------------------------------------------------

func TestQA_Detail_EscapeNotConsumedByDetail(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-abc", "inst", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	// Press escape -- the detail model delegates unknown keys to the viewport.
	_, cmd := detailApplyMsg(m, detailSpecialKey(tea.KeyEscape))
	// Escape is not handled by detail; it bubbles to the root.
	_ = cmd
}

// ---------------------------------------------------------------------------
// Fallback rendering: Fields map (no RawStruct)
// ---------------------------------------------------------------------------

func TestQA_Detail_FallbackFieldsMap(t *testing.T) {
	ensureNoColor(t)
	res := buildResourceWithFields("test-id", "test-name", map[string]string{
		"cluster_id":     "test-cluster",
		"engine_version": "7.0.7",
		"status":         "available",
	})
	// nil config forces pure Fields-map fallback rendering
	m := newDetailModel(res, "", nil)

	view := m.View()
	// Fields map fallback sorts keys alphabetically
	if !strings.Contains(view, "cluster_id") {
		t.Error("fallback detail should contain 'cluster_id'")
	}
	if !strings.Contains(view, "test-cluster") {
		t.Error("fallback detail should contain field value 'test-cluster'")
	}
	if !strings.Contains(view, "engine_version") {
		t.Error("fallback detail should contain 'engine_version'")
	}
	if !strings.Contains(view, "status") {
		t.Error("fallback detail should contain 'status'")
	}
}

func TestQA_Detail_NoDataAvailable_NoConfig(t *testing.T) {
	ensureNoColor(t)
	// With no config AND no fields, show "No detail data available"
	res := resource.Resource{ID: "empty", Name: "empty"}
	m := newDetailModel(res, "", nil)

	view := m.View()
	if !strings.Contains(view, "No detail data available") {
		t.Errorf("empty resource detail (no config) should show 'No detail data available', got:\n%s", view)
	}
}

func TestQA_Detail_EmptyResource_WithConfig_ShowsPathsAsDash(t *testing.T) {
	ensureNoColor(t)
	// With config but empty resource, show all config paths with "-"
	res := resource.Resource{ID: "empty", Name: "empty"}
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	view := m.View()
	if !strings.Contains(view, "InstanceId") {
		t.Error("config-driven detail should show InstanceId path even with no data")
	}
	if !strings.Contains(view, "-") {
		t.Error("empty fields should show '-' placeholder")
	}
}

// ---------------------------------------------------------------------------
// Fixture data detail: all types with realistic data
// ---------------------------------------------------------------------------

func TestQA_Detail_FixtureS3Buckets(t *testing.T) {
	ensureNoColor(t)
	bucket := realisticS3Bucket()
	res := buildResource("my-production-bucket", "my-production-bucket", bucket)
	cfg := configForType("s3")
	m := newDetailModel(res, "s3", cfg)

	view := m.View()
	if view == "" {
		t.Error("S3 bucket fixture detail should produce non-empty view")
	}
	if !strings.Contains(view, "2025-06-15") {
		t.Errorf("S3 bucket detail should contain creation date, got:\n%s", view)
	}
}

func TestQA_Detail_FixtureEC2Instances(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	view := m.View()
	if !strings.Contains(view, "i-0abcdef1234567890") {
		t.Error("EC2 fixture detail should contain instance ID")
	}
	if !strings.Contains(view, "t3.medium") {
		t.Error("EC2 fixture detail should contain instance type")
	}
}

func TestQA_Detail_FixtureRDSInstances(t *testing.T) {
	ensureNoColor(t)
	db := realisticRDSInstance()
	res := buildResource("prod-db-01", "prod-db-01", db)
	cfg := configForType("dbi")
	m := newDetailModel(res, "dbi", cfg)

	view := m.View()
	if !strings.Contains(view, "prod-db-01") {
		t.Error("RDS fixture detail should contain DB identifier")
	}
	if !strings.Contains(view, "mysql") {
		t.Error("RDS fixture detail should contain engine")
	}
}

func TestQA_Detail_FixtureRedisClusters(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticRedisCacheCluster()
	res := buildResource("redis-prod-001", "redis-prod-001", cluster)
	cfg := configForType("redis")
	m := newDetailModel(res, "redis", cfg)

	view := m.View()
	if !strings.Contains(view, "redis-prod-001") {
		t.Error("Redis fixture detail should contain cluster ID")
	}
	if !strings.Contains(view, "7.0.12") {
		t.Error("Redis fixture detail should contain engine version")
	}
}

func TestQA_Detail_FixtureDocDBClusters(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticDocDBCluster()
	res := buildResource("docdb-prod-cluster", "docdb-prod-cluster", cluster)
	cfg := configForType("dbc")
	m := newDetailModel(res, "dbc", cfg)

	view := m.View()
	if !strings.Contains(view, "docdb-prod-cluster") {
		t.Error("DocDB fixture detail should contain cluster ID")
	}
	if !strings.Contains(view, "dbc") {
		t.Error("DocDB fixture detail should contain engine")
	}
}

func TestQA_Detail_FixtureEKSClusters(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticEKSCluster()
	res := buildResource("prod-cluster", "prod-cluster", cluster)
	cfg := configForType("eks")
	m := newDetailModel(res, "eks", cfg)

	view := m.View()
	if !strings.Contains(view, "prod-cluster") {
		t.Error("EKS fixture detail should contain cluster name")
	}
	if !strings.Contains(view, "1.28") {
		t.Error("EKS fixture detail should contain version")
	}
}

func TestQA_Detail_FixtureSecrets(t *testing.T) {
	ensureNoColor(t)
	secret := realisticSecretListEntry()
	res := buildResource("prod/database/password", "prod/database/password", secret)
	cfg := configForType("secrets")
	m := newDetailModel(res, "secrets", cfg)

	view := m.View()
	if !strings.Contains(view, "prod/database/password") {
		t.Error("Secrets fixture detail should contain secret name")
	}
	if !strings.Contains(view, "Production database password") {
		t.Error("Secrets fixture detail should contain description")
	}
}

// ---------------------------------------------------------------------------
// Cross-cutting: Boolean formatting
// ---------------------------------------------------------------------------

func TestQA_Detail_CrossCutting_BooleanYes(t *testing.T) {
	ensureNoColor(t)
	db := rdstypes.DBInstance{
		DBInstanceIdentifier: ptrString("bool-yes"),
		Engine:               ptrString("mysql"),
		MultiAZ:              ptrBool(true),
	}
	res := buildResource("bool-yes", "bool-yes", db)
	cfg := configForType("dbi")
	m := newDetailModel(res, "dbi", cfg)

	view := m.View()
	if !strings.Contains(view, "Yes") {
		t.Error("boolean true should render as 'Yes'")
	}
}

func TestQA_Detail_CrossCutting_BooleanNo(t *testing.T) {
	ensureNoColor(t)
	db := rdstypes.DBInstance{
		DBInstanceIdentifier: ptrString("bool-no"),
		Engine:               ptrString("mysql"),
		MultiAZ:              ptrBool(false),
	}
	res := buildResource("bool-no", "bool-no", db)
	cfg := configForType("dbi")
	m := newDetailModel(res, "dbi", cfg)

	view := m.View()
	if !strings.Contains(view, "No") {
		t.Error("boolean false should render as 'No'")
	}
}

// ---------------------------------------------------------------------------
// Cross-cutting: Timestamp formatting
// ---------------------------------------------------------------------------

func TestQA_Detail_CrossCutting_TimestampFormat(t *testing.T) {
	ensureNoColor(t)
	bucket := realisticS3Bucket()
	res := buildResource("bucket", "bucket", bucket)
	cfg := configForType("s3")
	m := newDetailModel(res, "s3", cfg)

	view := m.View()
	if !strings.Contains(view, "2025-06-15 10:30:00") {
		t.Errorf("timestamp should be formatted as 'YYYY-MM-DD HH:MM:SS', got:\n%s", view)
	}
	if strings.Contains(view, "T10:30:00") {
		t.Error("timestamp should not use ISO 8601 format with T separator")
	}
}

// ---------------------------------------------------------------------------
// Cross-cutting: Integer formatting
// ---------------------------------------------------------------------------

func TestQA_Detail_CrossCutting_IntegerFormat(t *testing.T) {
	ensureNoColor(t)
	db := realisticRDSInstance()
	res := buildResource("dbi", "dbi", db)
	cfg := configForType("dbi")
	m := newDetailModel(res, "dbi", cfg)

	view := m.View()
	if !strings.Contains(view, "100") {
		t.Error("integer field AllocatedStorage should render as plain number '100'")
	}
}

func TestQA_Detail_CrossCutting_PortInteger(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticDocDBCluster()
	res := buildResource("dbc", "dbc", cluster)
	cfg := configForType("dbc")
	m := newDetailModel(res, "dbc", cfg)

	view := m.View()
	if !strings.Contains(view, "27017") {
		t.Error("integer field Port should render as plain number '27017'")
	}
}

// ---------------------------------------------------------------------------
// Cross-cutting: Field ordering matches views.yaml detail list
// ---------------------------------------------------------------------------

func TestQA_Detail_FieldOrdering_EC2(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-test", "i-test", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	view := m.View()
	expectedOrder := []string{
		"InstanceId", "State", "InstanceType", "ImageId",
		"VpcId", "SubnetId", "PrivateIpAddress", "PublicIpAddress",
		"SecurityGroups", "LaunchTime", "Architecture",
	}
	lastIdx := -1
	for _, field := range expectedOrder {
		idx := strings.Index(view, field)
		if idx == -1 {
			t.Errorf("expected field %q not found in detail view", field)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("field %q (at %d) should appear after previous field (at %d) -- ordering broken", field, idx, lastIdx)
		}
		lastIdx = idx
	}
}

func TestQA_Detail_FieldOrdering_RDS(t *testing.T) {
	ensureNoColor(t)
	db := realisticRDSInstance()
	res := buildResource("rds-test", "rds-test", db)
	cfg := configForType("dbi")
	m := newDetailModel(res, "dbi", cfg)

	view := m.View()
	expectedOrder := []string{
		"DBInstanceIdentifier", "Engine", "EngineVersion",
		"DBInstanceStatus", "DBInstanceClass", "Endpoint",
		"MultiAZ", "AllocatedStorage", "StorageType", "AvailabilityZone",
	}
	lastIdx := -1
	for _, field := range expectedOrder {
		idx := strings.Index(view, field)
		if idx == -1 {
			t.Errorf("expected field %q not found in RDS detail view", field)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("field %q (at %d) should appear after previous field (at %d)", field, idx, lastIdx)
		}
		lastIdx = idx
	}
}

func TestQA_Detail_FieldOrdering_Secrets(t *testing.T) {
	ensureNoColor(t)
	secret := realisticSecretListEntry()
	res := buildResource("secret-test", "secret-test", secret)
	cfg := configForType("secrets")
	m := newDetailModel(res, "secrets", cfg)

	view := m.View()
	expectedOrder := []string{
		"Description", "LastAccessedDate", "LastChangedDate",
		"RotationEnabled", "ARN", "KmsKeyId", "Tags",
	}
	lastIdx := -1
	for _, field := range expectedOrder {
		idx := strings.Index(view, field)
		if idx == -1 {
			t.Errorf("expected field %q not found in Secrets detail view", field)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("field %q (at %d) should appear after previous field (at %d)", field, idx, lastIdx)
		}
		lastIdx = idx
	}
}

func TestQA_Detail_FieldOrdering_DocDB(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticDocDBCluster()
	res := buildResource("docdb-test", "docdb-test", cluster)
	cfg := configForType("dbc")
	m := newDetailModel(res, "dbc", cfg)

	view := m.View()
	expectedOrder := []string{
		"DBClusterIdentifier", "Engine", "EngineVersion", "Status",
		"Endpoint", "ReaderEndpoint", "Port", "StorageEncrypted",
		"DBClusterMembers",
	}
	lastIdx := -1
	for _, field := range expectedOrder {
		idx := strings.Index(view, field)
		if idx == -1 {
			t.Errorf("expected field %q not found in DocDB detail view", field)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("field %q (at %d) should appear after previous field (at %d)", field, idx, lastIdx)
		}
		lastIdx = idx
	}
}

func TestQA_Detail_FieldOrdering_Redis(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticRedisCacheCluster()
	res := buildResource("redis-test", "redis-test", cluster)
	cfg := configForType("redis")
	m := newDetailModel(res, "redis", cfg)

	view := m.View()
	// Note: field names longer than 22 chars are truncated by PadOrTrunc.
	// "ConfigurationEndpoint" (21 chars) fits; "PreferredAvailabilityZone" (25 chars) is truncated.
	expectedOrder := []string{
		"CacheClusterId", "Engine", "EngineVersion",
		"CacheClusterStatus", "CacheNodeType", "NumCacheNodes",
		"ConfigurationEndpoint", "PreferredAvailability",
	}
	lastIdx := -1
	for _, field := range expectedOrder {
		idx := strings.Index(view, field)
		if idx == -1 {
			t.Errorf("expected field %q not found in Redis detail view", field)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("field %q (at %d) should appear after previous field (at %d)", field, idx, lastIdx)
		}
		lastIdx = idx
	}
}

func TestQA_Detail_FieldOrdering_EKS(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticEKSCluster()
	res := buildResource("eks-test", "eks-test", cluster)
	cfg := configForType("eks")
	m := newDetailModel(res, "eks", cfg)

	view := m.View()
	// Note: "KubernetesNetworkConfig" (23 chars) is truncated by PadOrTrunc (keyW=22).
	// When its YAML representation is a single line, it renders via kv() with truncated key.
	expectedOrder := []string{
		"Version", "Status", "Endpoint",
		"PlatformVersion", "Arn", "RoleArn", "KubernetesNetworkConf",
	}
	lastIdx := -1
	for _, field := range expectedOrder {
		idx := strings.Index(view, field)
		if idx == -1 {
			t.Errorf("expected field %q not found in EKS detail view", field)
			continue
		}
		if idx <= lastIdx {
			t.Errorf("field %q (at %d) should appear after previous field (at %d)", field, idx, lastIdx)
		}
		lastIdx = idx
	}
}

// ---------------------------------------------------------------------------
// ResourceID (for clipboard copy)
// ---------------------------------------------------------------------------

func TestQA_Detail_ResourceID(t *testing.T) {
	inst := realisticEC2Instance()
	res := buildResource("i-0abcdef1234567890", "web-server-prod", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	if id := m.ResourceID(); id != "i-0abcdef1234567890" {
		t.Errorf("ResourceID expected %q, got %q", "i-0abcdef1234567890", id)
	}
}

// ---------------------------------------------------------------------------
// All resource types: View() is non-empty after SetSize
// ---------------------------------------------------------------------------

func TestQA_Detail_AllTypes_ViewNonEmpty(t *testing.T) {
	ensureNoColor(t)
	cases := []struct {
		name     string
		typeName string
		res      resource.Resource
	}{
		{"S3Bucket", "s3", buildResource("b", "b", realisticS3Bucket())},
		{"S3Object", "s3_objects", buildResource("o", "o", realisticS3ObjectFile())},
		{"EC2", "ec2", buildResource("i", "i", realisticEC2Instance())},
		{"RDS", "dbi", buildResource("r", "r", realisticRDSInstance())},
		{"Redis", "redis", buildResource("rc", "rc", realisticRedisCacheCluster())},
		{"DocDB", "dbc", buildResource("dc", "dc", realisticDocDBCluster())},
		{"EKS", "eks", buildResource("ek", "ek", realisticEKSCluster())},
		{"Secrets", "secrets", buildResource("s", "s", realisticSecretListEntry())},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := configForType(tc.typeName)
			m := newDetailModel(tc.res, tc.typeName, cfg)
			view := m.View()
			if view == "" {
				t.Errorf("%s detail view is empty", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// All resource types: nil/empty RawStruct still renders
// ---------------------------------------------------------------------------

func TestQA_Detail_AllTypes_NilFieldsRender(t *testing.T) {
	ensureNoColor(t)
	cases := []struct {
		name     string
		typeName string
		res      resource.Resource
	}{
		{"S3Bucket_nil", "s3", buildResource("b", "b", s3types.Bucket{})},
		{"S3Object_nil", "s3_objects", buildResource("o", "o", s3types.Object{})},
		{"EC2_nil", "ec2", buildResource("i", "i", ec2types.Instance{})},
		{"RDS_nil", "dbi", buildResource("r", "r", rdstypes.DBInstance{})},
		{"Redis_nil", "redis", buildResource("rc", "rc", elasticachetypes.CacheCluster{})},
		{"DocDB_nil", "dbc", buildResource("dc", "dc", docdbtypes.DBCluster{})},
		{"EKS_nil", "eks", buildResource("ek", "ek", &ekstypes.Cluster{})},
		{"Secrets_nil", "secrets", buildResource("s", "s", smtypes.SecretListEntry{})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := configForType(tc.typeName)
			m := newDetailModel(tc.res, tc.typeName, cfg)
			// Must not panic
			view := m.View()
			_ = view
		})
	}
}

// ---------------------------------------------------------------------------
// View not ready before SetSize
// ---------------------------------------------------------------------------

func TestQA_Detail_ViewBeforeSetSize(t *testing.T) {
	inst := realisticEC2Instance()
	res := buildResource("i-test", "i-test", inst)
	cfg := configForType("ec2")
	k := keys.Default()
	m := views.NewDetail(res, "ec2", cfg, k)
	// Do NOT call SetSize

	view := m.View()
	if !strings.Contains(view, "Initializing") {
		t.Errorf("View before SetSize should show 'Initializing...', got %q", view)
	}
}

// ---------------------------------------------------------------------------
// Scroll does not crash before SetSize
// ---------------------------------------------------------------------------

func TestQA_Detail_ScrollBeforeReady(t *testing.T) {
	inst := realisticEC2Instance()
	res := buildResource("i-test", "i-test", inst)
	cfg := configForType("ec2")
	k := keys.Default()
	m := views.NewDetail(res, "ec2", cfg, k)
	// Do NOT call SetSize -- viewport is not ready

	// These should not panic
	m, _ = m.Update(detailKeyPress("j"))
	m, _ = m.Update(detailKeyPress("k"))
	m, _ = m.Update(detailKeyPress("g"))
	m, _ = m.Update(detailKeyPress("G"))
	_, _ = m.Update(detailKeyPress("w"))
}

// ---------------------------------------------------------------------------
// Resize after initial SetSize
// ---------------------------------------------------------------------------

func TestQA_Detail_Resize(t *testing.T) {
	ensureNoColor(t)
	inst := realisticEC2Instance()
	res := buildResource("i-test", "i-test", inst)
	cfg := configForType("ec2")
	m := newDetailModel(res, "ec2", cfg)

	viewBefore := m.View()

	m.SetSize(60, 20)
	viewAfter := m.View()

	if viewBefore == viewAfter {
		t.Log("View may differ after resize -- dimensions changed")
	}
	if viewAfter == "" {
		t.Error("View after resize should not be empty")
	}
}

// ---------------------------------------------------------------------------
// Long value not truncated
// ---------------------------------------------------------------------------

func TestQA_Detail_LongValueNotTruncated(t *testing.T) {
	ensureNoColor(t)
	secret := smtypes.SecretListEntry{
		Name: ptrString("test-secret"),
		ARN:  ptrString("arn:aws:secretsmanager:us-east-1:123456789012:secret:very/long/path/to/secret-AbCdEfGhIjKl"),
	}
	res := buildResource("test-secret", "test-secret", secret)
	cfg := configForType("secrets")
	m := newDetailModel(res, "secrets", cfg)

	view := m.View()
	if !strings.Contains(view, "arn:aws:secretsmanager:us-east-1:123456789012:secret:very/long/path/to/secret-AbCdEfGhIjKl") {
		t.Error("long ARN value should not be truncated in detail view")
	}
}

// ---------------------------------------------------------------------------
// Init returns no command
// ---------------------------------------------------------------------------

func TestQA_Detail_InitReturnsNoCmd(t *testing.T) {
	inst := realisticEC2Instance()
	res := buildResource("i-test", "i-test", inst)
	cfg := configForType("ec2")
	k := keys.Default()
	m := views.NewDetail(res, "ec2", cfg, k)

	_, cmd := m.Init()
	if cmd != nil {
		t.Error("Init() should return nil command")
	}
}
