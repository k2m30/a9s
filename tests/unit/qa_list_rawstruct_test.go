package unit_test

import (
	"strings"
	"testing"

	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	codebuildtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	codepipelinetypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers for list-view RawStruct tests
// ---------------------------------------------------------------------------

// newListModel creates a ResourceListModel with config, loads the given
// resources, sets a wide size so all columns render, and returns View() output.
func newListModel(t *testing.T, shortName string, cfg *config.ViewsConfig, resources []resource.Resource) string {
	t.Helper()

	typeDef := resource.FindResourceType(shortName)
	if typeDef == nil {
		t.Fatalf("unknown resource type %q", shortName)
	}

	k := keys.Default()
	m := views.NewResourceList(*typeDef, cfg, k)
	m.SetSize(400, 50)

	// Simulate resources loaded
	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: resources})

	return stripAnsi(m.View())
}

// ---------------------------------------------------------------------------
// TestQA_ListRawStruct_EC2: State.Name from nested InstanceState struct
// ---------------------------------------------------------------------------

func TestQA_ListRawStruct_EC2(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("ec2")

	inst := realisticEC2Instance()
	res := resource.Resource{
		ID:     "i-0abcdef1234567890",
		Name:   "web-server-prod",
		Status: "running",
		Fields: map[string]string{
			"instance_id": "i-0abcdef1234567890",
			"name":        "web-server-prod",
			"state":       "running",
			"type":        "t3.medium",
			"private_ip":  "10.0.1.42",
			"public_ip":   "54.123.45.67",
		},
		RawStruct: inst,
	}

	view := newListModel(t, "ec2", cfg, []resource.Resource{res})

	// State.Name should be extracted from RawStruct (nested ec2types.InstanceState)
	if !strings.Contains(view, "running") {
		t.Errorf("EC2 list should contain 'running' from State.Name, got:\n%s", view)
	}
	// InstanceId from RawStruct
	if !strings.Contains(view, "i-0abcdef1234567890") {
		t.Errorf("EC2 list should contain instance ID from RawStruct, got:\n%s", view)
	}
	// InstanceType from RawStruct
	if !strings.Contains(view, "t3.medium") {
		t.Errorf("EC2 list should contain instance type from RawStruct, got:\n%s", view)
	}
	// PrivateIpAddress from RawStruct
	if !strings.Contains(view, "10.0.1.42") {
		t.Errorf("EC2 list should contain private IP from RawStruct, got:\n%s", view)
	}
	// PublicIpAddress from RawStruct
	if !strings.Contains(view, "54.123.45.67") {
		t.Errorf("EC2 list should contain public IP from RawStruct, got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestQA_ListRawStruct_RDS: Endpoint.Address from nested rdstypes.Endpoint
// ---------------------------------------------------------------------------

func TestQA_ListRawStruct_RDS(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("dbi")

	db := realisticRDSInstance()
	res := resource.Resource{
		ID:     "prod-db-01",
		Name:   "prod-db-01",
		Status: "available",
		Fields: map[string]string{
			"db_identifier":  "prod-db-01",
			"engine":         "mysql",
			"engine_version": "8.0.35",
			"status":         "available",
			"class":          "db.r5.large",
			"endpoint":       "some-old-endpoint",
			"multi_az":       "Yes",
		},
		RawStruct: db,
	}

	view := newListModel(t, "dbi", cfg, []resource.Resource{res})

	// Endpoint.Address from nested RawStruct (may be truncated by column width)
	if !strings.Contains(view, "prod-db-01.abc123") {
		t.Errorf("RDS list should contain endpoint address prefix from RawStruct, got:\n%s", view)
	}
	// DBInstanceIdentifier from RawStruct
	if !strings.Contains(view, "prod-db-01") {
		t.Errorf("RDS list should contain DB identifier from RawStruct, got:\n%s", view)
	}
	// MultiAZ as "Yes" from RawStruct
	if !strings.Contains(view, "Yes") {
		t.Errorf("RDS list should contain 'Yes' for MultiAZ from RawStruct, got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestQA_ListRawStruct_Redis: ConfigurationEndpoint.Address from nested struct
// ---------------------------------------------------------------------------

func TestQA_ListRawStruct_Redis(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("redis")

	cluster := realisticRedisCacheCluster()
	res := resource.Resource{
		ID:     "redis-prod-001",
		Name:   "redis-prod-001",
		Status: "available",
		Fields: map[string]string{
			"cluster_id":     "redis-prod-001",
			"engine_version": "7.0.12",
			"node_type":      "cache.r6g.large",
			"status":         "available",
			"nodes":          "3",
			"endpoint":       "old-endpoint",
		},
		RawStruct: cluster,
	}

	view := newListModel(t, "redis", cfg, []resource.Resource{res})

	// ConfigurationEndpoint.Address from nested RawStruct (may be truncated)
	if !strings.Contains(view, "redis-prod-001.abc123.clustercfg") {
		t.Errorf("Redis list should contain endpoint prefix from RawStruct, got:\n%s", view)
	}
	// CacheClusterId from RawStruct
	if !strings.Contains(view, "redis-prod-001") {
		t.Errorf("Redis list should contain cluster ID from RawStruct, got:\n%s", view)
	}
	// NumCacheNodes as "3" from RawStruct
	if !strings.Contains(view, "3") {
		t.Errorf("Redis list should contain node count from RawStruct, got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestQA_ListRawStruct_DocDB: Endpoint (plain string) and Status
// ---------------------------------------------------------------------------

func TestQA_ListRawStruct_DocDB(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("dbc")

	cluster := realisticDocDBCluster()
	res := resource.Resource{
		ID:     "docdb-prod-cluster",
		Name:   "docdb-prod-cluster",
		Status: "available",
		Fields: map[string]string{
			"cluster_id":     "docdb-prod-cluster",
			"engine_version": "5.0.0",
			"status":         "available",
			"instances":      "2",
			"endpoint":       "old-endpoint",
		},
		RawStruct: cluster,
	}

	view := newListModel(t, "dbc", cfg, []resource.Resource{res})

	// Endpoint from RawStruct (may be truncated by column width)
	if !strings.Contains(view, "docdb-prod.cluster-abc123") {
		t.Errorf("DocDB list should contain endpoint prefix from RawStruct, got:\n%s", view)
	}
	// DBClusterIdentifier from RawStruct
	if !strings.Contains(view, "docdb-prod-cluster") {
		t.Errorf("DocDB list should contain cluster ID from RawStruct, got:\n%s", view)
	}
	// Status from RawStruct
	if !strings.Contains(view, "available") {
		t.Errorf("DocDB list should contain status from RawStruct, got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestQA_ListRawStruct_EKS: Name, Version, Status, Endpoint
// ---------------------------------------------------------------------------

func TestQA_ListRawStruct_EKS(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("eks")

	cluster := realisticEKSCluster()
	res := resource.Resource{
		ID:     "prod-cluster",
		Name:   "prod-cluster",
		Status: "ACTIVE",
		Fields: map[string]string{
			"cluster_name":     "prod-cluster",
			"version":          "1.28",
			"status":           "ACTIVE",
			"endpoint":         "old-endpoint",
			"platform_version": "eks.5",
		},
		RawStruct: cluster,
	}

	view := newListModel(t, "eks", cfg, []resource.Resource{res})

	// Name from RawStruct
	if !strings.Contains(view, "prod-cluster") {
		t.Errorf("EKS list should contain cluster name from RawStruct, got:\n%s", view)
	}
	// Endpoint from RawStruct
	if !strings.Contains(view, "ABCDEF1234567890") {
		t.Errorf("EKS list should contain endpoint from RawStruct, got:\n%s", view)
	}
	// PlatformVersion from RawStruct
	if !strings.Contains(view, "eks.5") {
		t.Errorf("EKS list should contain platform version from RawStruct, got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestQA_ListRawStruct_Secrets: Name, Description, RotationEnabled
// ---------------------------------------------------------------------------

func TestQA_ListRawStruct_Secrets(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("secrets")

	secret := realisticSecretListEntry()
	res := resource.Resource{
		ID:     "prod/database/password",
		Name:   "prod/database/password",
		Status: "",
		Fields: map[string]string{
			"secret_name":      "prod/database/password",
			"description":      "old-desc",
			"last_accessed":    "old-date",
			"last_changed":     "old-date",
			"rotation_enabled": "old-value",
		},
		RawStruct: secret,
	}

	view := newListModel(t, "secrets", cfg, []resource.Resource{res})

	// Name from RawStruct
	if !strings.Contains(view, "prod/database/password") {
		t.Errorf("Secrets list should contain secret name from RawStruct, got:\n%s", view)
	}
	// Description from RawStruct
	if !strings.Contains(view, "Production database password") {
		t.Errorf("Secrets list should contain description from RawStruct, got:\n%s", view)
	}
	// RotationEnabled as "Yes" from RawStruct
	if !strings.Contains(view, "Yes") {
		t.Errorf("Secrets list should contain 'Yes' for rotation from RawStruct, got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestQA_ListRawStruct_S3: Name and CreationDate
// ---------------------------------------------------------------------------

func TestQA_ListRawStruct_S3(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("s3")

	bucket := realisticS3Bucket()
	res := resource.Resource{
		ID:     "my-production-bucket",
		Name:   "my-production-bucket",
		Status: "",
		Fields: map[string]string{
			"name":          "my-production-bucket",
			"bucket_name":   "my-production-bucket",
			"creation_date": "old-date",
		},
		RawStruct: bucket,
	}

	view := newListModel(t, "s3", cfg, []resource.Resource{res})

	// Name from RawStruct
	if !strings.Contains(view, "my-production-bucket") {
		t.Errorf("S3 list should contain bucket name from RawStruct, got:\n%s", view)
	}
	// CreationDate formatted from RawStruct
	if !strings.Contains(view, "2025-06-15") {
		t.Errorf("S3 list should contain creation date from RawStruct, got:\n%s", view)
	}
}

// ===========================================================================
// CRITICAL TEST: RawStruct values OVERRIDE Fields map values
// ===========================================================================
// This is the core of the bug coverage: if extractCellValue reads from
// Fields instead of RawStruct, these tests will fail because Fields
// contains "WRONG" values while RawStruct contains "CORRECT" values.

func TestQA_ListRawStruct_EC2_RawStructOverridesFields(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("ec2")

	inst := ec2types.Instance{
		InstanceId:       ptrString("i-correct-id"),
		InstanceType:     ec2types.InstanceTypeT3Medium,
		PrivateIpAddress: ptrString("10.0.0.99"),
		State: &ec2types.InstanceState{
			Name: ec2types.InstanceStateNameRunning,
		},
	}

	res := resource.Resource{
		ID:   "i-correct-id",
		Name: "test-instance",
		Fields: map[string]string{
			"instance_id": "WRONG-ID",
			"state":       "WRONG-STATE",
			"type":        "WRONG-TYPE",
			"private_ip":  "WRONG-IP",
		},
		RawStruct: inst,
	}

	view := newListModel(t, "ec2", cfg, []resource.Resource{res})

	// RawStruct values must appear, not Fields values
	if strings.Contains(view, "WRONG-ID") {
		t.Error("EC2 list should NOT contain WRONG-ID from Fields; should use RawStruct")
	}
	if strings.Contains(view, "WRONG-STATE") {
		t.Error("EC2 list should NOT contain WRONG-STATE from Fields; should use RawStruct")
	}
	if strings.Contains(view, "WRONG-TYPE") {
		t.Error("EC2 list should NOT contain WRONG-TYPE from Fields; should use RawStruct")
	}
	if strings.Contains(view, "WRONG-IP") {
		t.Error("EC2 list should NOT contain WRONG-IP from Fields; should use RawStruct")
	}

	// Correct values from RawStruct must appear
	if !strings.Contains(view, "i-correct-id") {
		t.Errorf("EC2 list should contain 'i-correct-id' from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "running") {
		t.Errorf("EC2 list should contain 'running' from RawStruct State.Name, got:\n%s", view)
	}
	if !strings.Contains(view, "t3.medium") {
		t.Errorf("EC2 list should contain 't3.medium' from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "10.0.0.99") {
		t.Errorf("EC2 list should contain '10.0.0.99' from RawStruct, got:\n%s", view)
	}
}

func TestQA_ListRawStruct_RDS_RawStructOverridesFields(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("dbi")

	db := rdstypes.DBInstance{
		DBInstanceIdentifier: ptrString("correct-db"),
		Engine:               ptrString("postgres"),
		EngineVersion:        ptrString("15.4"),
		DBInstanceStatus:     ptrString("available"),
		DBInstanceClass:      ptrString("db.m5.xlarge"),
		MultiAZ:              ptrBool(false),
		Endpoint: &rdstypes.Endpoint{
			Address: ptrString("correct-endpoint.rds.amazonaws.com"),
			Port:    ptrInt32(5432),
		},
	}

	res := resource.Resource{
		ID:   "correct-db",
		Name: "correct-db",
		Fields: map[string]string{
			"db_identifier":  "WRONG-DB",
			"engine":         "WRONG-ENGINE",
			"engine_version": "WRONG-VER",
			"status":         "WRONG-STATUS",
			"class":          "WRONG-CLASS",
			"endpoint":       "WRONG-ENDPOINT",
			"multi_az":       "WRONG-AZ",
		},
		RawStruct: db,
	}

	view := newListModel(t, "dbi", cfg, []resource.Resource{res})

	// RawStruct values must appear
	if !strings.Contains(view, "correct-db") {
		t.Errorf("RDS list should contain 'correct-db' from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "postgres") {
		t.Errorf("RDS list should contain 'postgres' from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "correct-endpoint.rds") {
		t.Errorf("RDS list should contain endpoint prefix from RawStruct, got:\n%s", view)
	}

	// Fields values must NOT appear
	if strings.Contains(view, "WRONG-DB") {
		t.Error("RDS list should NOT contain WRONG-DB from Fields")
	}
	if strings.Contains(view, "WRONG-ENGINE") {
		t.Error("RDS list should NOT contain WRONG-ENGINE from Fields")
	}
	if strings.Contains(view, "WRONG-ENDPOINT") {
		t.Error("RDS list should NOT contain WRONG-ENDPOINT from Fields")
	}
}

func TestQA_ListRawStruct_Redis_RawStructOverridesFields(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("redis")

	cluster := elasticachetypes.CacheCluster{
		CacheClusterId:     ptrString("correct-redis"),
		Engine:             ptrString("redis"),
		EngineVersion:      ptrString("7.0.12"),
		CacheNodeType:      ptrString("cache.r6g.xlarge"),
		CacheClusterStatus: ptrString("available"),
		NumCacheNodes:      ptrInt32(5),
		ConfigurationEndpoint: &elasticachetypes.Endpoint{
			Address: ptrString("correct-redis.cache.amazonaws.com"),
			Port:    ptrInt32(6379),
		},
	}

	res := resource.Resource{
		ID:   "correct-redis",
		Name: "correct-redis",
		Fields: map[string]string{
			"cluster_id":     "WRONG-CLUSTER",
			"engine_version": "WRONG-VER",
			"node_type":      "WRONG-TYPE",
			"status":         "WRONG-STATUS",
			"nodes":          "WRONG-NODES",
			"endpoint":       "WRONG-ENDPOINT",
		},
		RawStruct: cluster,
	}

	view := newListModel(t, "redis", cfg, []resource.Resource{res})

	// RawStruct values
	if !strings.Contains(view, "correct-redis") {
		t.Errorf("Redis list should contain 'correct-redis' from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "correct-redis.cache") {
		t.Errorf("Redis list should contain endpoint prefix from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "cache.r6g.xlarge") {
		t.Errorf("Redis list should contain node type from RawStruct, got:\n%s", view)
	}

	// Fields must not appear
	if strings.Contains(view, "WRONG-CLUSTER") {
		t.Error("Redis list should NOT contain WRONG-CLUSTER from Fields")
	}
	if strings.Contains(view, "WRONG-ENDPOINT") {
		t.Error("Redis list should NOT contain WRONG-ENDPOINT from Fields")
	}
}

func TestQA_ListRawStruct_DocDB_RawStructOverridesFields(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("dbc")

	cluster := docdbtypes.DBCluster{
		DBClusterIdentifier: ptrString("correct-docdb"),
		Engine:              ptrString("dbc"),
		EngineVersion:       ptrString("5.0.0"),
		Status:              ptrString("available"),
		Endpoint:            ptrString("correct-docdb.cluster.us-east-1.docdb.amazonaws.com"),
		DBClusterMembers: []docdbtypes.DBClusterMember{
			{DBInstanceIdentifier: ptrString("inst-1"), IsClusterWriter: ptrBool(true)},
		},
	}

	res := resource.Resource{
		ID:   "correct-docdb",
		Name: "correct-docdb",
		Fields: map[string]string{
			"cluster_id":     "WRONG-CLUSTER",
			"engine_version": "WRONG-VER",
			"status":         "WRONG-STATUS",
			"instances":      "WRONG-COUNT",
			"endpoint":       "WRONG-ENDPOINT",
		},
		RawStruct: cluster,
	}

	view := newListModel(t, "dbc", cfg, []resource.Resource{res})

	// RawStruct values
	if !strings.Contains(view, "correct-docdb") {
		t.Errorf("DocDB list should contain 'correct-docdb' from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "correct-docdb.cluster.us-east-1") {
		t.Errorf("DocDB list should contain endpoint prefix from RawStruct, got:\n%s", view)
	}

	// Fields must not appear for scalar paths
	if strings.Contains(view, "WRONG-CLUSTER") {
		t.Error("DocDB list should NOT contain WRONG-CLUSTER from Fields")
	}
	if strings.Contains(view, "WRONG-ENDPOINT") {
		t.Error("DocDB list should NOT contain WRONG-ENDPOINT from Fields")
	}
	// Note: "Instances" column uses path "DBClusterMembers" which is a slice,
	// so ExtractScalar returns "" and correctly falls back to Fields map.
	// This is expected behavior: non-scalar paths must fall back.
}

func TestQA_ListRawStruct_EKS_RawStructOverridesFields(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("eks")

	cluster := &ekstypes.Cluster{
		Name:            ptrString("correct-eks"),
		Version:         ptrString("1.29"),
		Status:          ekstypes.ClusterStatusActive,
		Endpoint:        ptrString("https://correct-eks.gr7.us-east-1.eks.amazonaws.com"),
		PlatformVersion: ptrString("eks.8"),
	}

	res := resource.Resource{
		ID:   "correct-eks",
		Name: "correct-eks",
		Fields: map[string]string{
			"cluster_name":     "WRONG-NAME",
			"version":          "WRONG-VER",
			"status":           "WRONG-STATUS",
			"endpoint":         "WRONG-ENDPOINT",
			"platform_version": "WRONG-PLAT",
		},
		RawStruct: cluster,
	}

	view := newListModel(t, "eks", cfg, []resource.Resource{res})

	// RawStruct values
	if !strings.Contains(view, "correct-eks") {
		t.Errorf("EKS list should contain 'correct-eks' from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "1.29") {
		t.Errorf("EKS list should contain '1.29' from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "eks.8") {
		t.Errorf("EKS list should contain 'eks.8' from RawStruct, got:\n%s", view)
	}

	// Fields must not appear
	if strings.Contains(view, "WRONG-NAME") {
		t.Error("EKS list should NOT contain WRONG-NAME from Fields")
	}
	if strings.Contains(view, "WRONG-ENDPOINT") {
		t.Error("EKS list should NOT contain WRONG-ENDPOINT from Fields")
	}
}

func TestQA_ListRawStruct_Secrets_RawStructOverridesFields(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("secrets")

	secret := smtypes.SecretListEntry{
		Name:            ptrString("correct/secret/name"),
		Description:     ptrString("Correct description from RawStruct"),
		RotationEnabled: ptrBool(true),
		LastAccessedDate: ptrTime(testTime),
		LastChangedDate:  ptrTime(testTime),
	}

	res := resource.Resource{
		ID:   "correct/secret/name",
		Name: "correct/secret/name",
		Fields: map[string]string{
			"secret_name":      "WRONG-NAME",
			"description":      "WRONG-DESC",
			"last_accessed":    "WRONG-DATE",
			"last_changed":     "WRONG-DATE",
			"rotation_enabled": "WRONG-ROT",
		},
		RawStruct: secret,
	}

	view := newListModel(t, "secrets", cfg, []resource.Resource{res})

	// RawStruct values
	if !strings.Contains(view, "correct/secret/name") {
		t.Errorf("Secrets list should contain name from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "Correct description") {
		t.Errorf("Secrets list should contain description from RawStruct, got:\n%s", view)
	}

	// Fields must not appear
	if strings.Contains(view, "WRONG-NAME") {
		t.Error("Secrets list should NOT contain WRONG-NAME from Fields")
	}
	if strings.Contains(view, "WRONG-DESC") {
		t.Error("Secrets list should NOT contain WRONG-DESC from Fields")
	}
}

func TestQA_ListRawStruct_S3_RawStructOverridesFields(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("s3")

	bucket := s3types.Bucket{
		Name:         ptrString("correct-bucket-name"),
		CreationDate: ptrTime(testTime),
	}

	res := resource.Resource{
		ID:   "correct-bucket-name",
		Name: "correct-bucket-name",
		Fields: map[string]string{
			"name":          "WRONG-BUCKET",
			"bucket_name":   "WRONG-BUCKET",
			"creation_date": "WRONG-DATE",
		},
		RawStruct: bucket,
	}

	view := newListModel(t, "s3", cfg, []resource.Resource{res})

	// RawStruct values
	if !strings.Contains(view, "correct-bucket-name") {
		t.Errorf("S3 list should contain bucket name from RawStruct, got:\n%s", view)
	}
	if !strings.Contains(view, "2025-06-15") {
		t.Errorf("S3 list should contain creation date from RawStruct, got:\n%s", view)
	}

	// Fields must not appear
	if strings.Contains(view, "WRONG-BUCKET") {
		t.Error("S3 list should NOT contain WRONG-BUCKET from Fields")
	}
	if strings.Contains(view, "WRONG-DATE") {
		t.Error("S3 list should NOT contain WRONG-DATE from Fields")
	}
}

// ===========================================================================
// Test with production views.yaml config file
// ===========================================================================

func TestQA_ListRawStruct_WithProductionViewsYAML(t *testing.T) {
	ensureNoColor(t)

	cfg, err := config.LoadFrom([]string{"../../.a9s/views.yaml"})
	if err != nil {
		t.Fatalf("failed to load production views.yaml: %v", err)
	}
	if cfg == nil {
		t.Fatal("production views.yaml not found")
	}

	// Sub-test for each resource type using production config
	t.Run("EC2", func(t *testing.T) {
		inst := ec2types.Instance{
			InstanceId:       ptrString("i-prod-config-test"),
			InstanceType:     ec2types.InstanceTypeM5Large,
			PrivateIpAddress: ptrString("172.16.0.100"),
			State: &ec2types.InstanceState{
				Name: ec2types.InstanceStateNameStopped,
			},
		}
		res := resource.Resource{
			ID:        "i-prod-config-test",
			Name:      "prod-test",
			Fields:    map[string]string{"state": "WRONG"},
			RawStruct: inst,
		}
		view := newListModel(t, "ec2", cfg, []resource.Resource{res})
		if !strings.Contains(view, "stopped") {
			t.Errorf("EC2 with production config should show 'stopped' from State.Name, got:\n%s", view)
		}
		if strings.Contains(view, "WRONG") {
			t.Error("EC2 with production config should NOT show WRONG from Fields")
		}
	})

	t.Run("RDS", func(t *testing.T) {
		db := rdstypes.DBInstance{
			DBInstanceIdentifier: ptrString("prod-rds-test"),
			Engine:               ptrString("aurora-mysql"),
			EngineVersion:        ptrString("3.04.0"),
			DBInstanceStatus:     ptrString("available"),
			DBInstanceClass:      ptrString("db.r6g.2xlarge"),
			MultiAZ:              ptrBool(true),
			Endpoint: &rdstypes.Endpoint{
				Address: ptrString("prod-rds-test.cluster-xyz.us-west-2.rds.amazonaws.com"),
			},
		}
		res := resource.Resource{
			ID:        "prod-rds-test",
			Name:      "prod-rds-test",
			Fields:    map[string]string{"endpoint": "WRONG-EP"},
			RawStruct: db,
		}
		view := newListModel(t, "dbi", cfg, []resource.Resource{res})
		if !strings.Contains(view, "prod-rds-test.cluster-xyz") {
			t.Errorf("RDS with production config should show endpoint prefix from Endpoint.Address, got:\n%s", view)
		}
		if strings.Contains(view, "WRONG-EP") {
			t.Error("RDS with production config should NOT show WRONG-EP from Fields")
		}
	})

	t.Run("Redis", func(t *testing.T) {
		cluster := elasticachetypes.CacheCluster{
			CacheClusterId:     ptrString("prod-redis-test"),
			EngineVersion:      ptrString("7.1.0"),
			CacheNodeType:      ptrString("cache.m7g.large"),
			CacheClusterStatus: ptrString("available"),
			NumCacheNodes:      ptrInt32(2),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: ptrString("prod-redis-test.clustercfg.usw2.cache.amazonaws.com"),
			},
		}
		res := resource.Resource{
			ID:        "prod-redis-test",
			Name:      "prod-redis-test",
			Fields:    map[string]string{"endpoint": "WRONG-EP"},
			RawStruct: cluster,
		}
		view := newListModel(t, "redis", cfg, []resource.Resource{res})
		if !strings.Contains(view, "prod-redis-test.clustercfg") {
			t.Errorf("Redis with production config should show endpoint prefix from ConfigurationEndpoint.Address, got:\n%s", view)
		}
		if strings.Contains(view, "WRONG-EP") {
			t.Error("Redis with production config should NOT show WRONG-EP from Fields")
		}
	})

	t.Run("DocDB", func(t *testing.T) {
		cluster := docdbtypes.DBCluster{
			DBClusterIdentifier: ptrString("prod-docdb-test"),
			EngineVersion:       ptrString("5.0.0"),
			Status:              ptrString("available"),
			Endpoint:            ptrString("prod-docdb-test.cluster-abc.us-west-2.docdb.amazonaws.com"),
		}
		res := resource.Resource{
			ID:        "prod-docdb-test",
			Name:      "prod-docdb-test",
			Fields:    map[string]string{"endpoint": "WRONG-EP"},
			RawStruct: cluster,
		}
		view := newListModel(t, "dbc", cfg, []resource.Resource{res})
		if !strings.Contains(view, "prod-docdb-test.cluster-abc") {
			t.Errorf("DocDB with production config should show endpoint prefix from RawStruct, got:\n%s", view)
		}
	})

	t.Run("EKS", func(t *testing.T) {
		cluster := &ekstypes.Cluster{
			Name:            ptrString("prod-eks-test"),
			Version:         ptrString("1.30"),
			Status:          ekstypes.ClusterStatusActive,
			Endpoint:        ptrString("https://prod-eks-test.gr7.us-west-2.eks.amazonaws.com"),
			PlatformVersion: ptrString("eks.9"),
		}
		res := resource.Resource{
			ID:        "prod-eks-test",
			Name:      "prod-eks-test",
			Fields:    map[string]string{"endpoint": "WRONG-EP"},
			RawStruct: cluster,
		}
		view := newListModel(t, "eks", cfg, []resource.Resource{res})
		if !strings.Contains(view, "prod-eks-test.gr7") {
			t.Errorf("EKS with production config should show endpoint prefix from RawStruct, got:\n%s", view)
		}
	})

	t.Run("Secrets", func(t *testing.T) {
		secret := smtypes.SecretListEntry{
			Name:        ptrString("prod/test/secret"),
			Description: ptrString("Production test secret"),
		}
		res := resource.Resource{
			ID:        "prod/test/secret",
			Name:      "prod/test/secret",
			Fields:    map[string]string{"description": "WRONG-DESC"},
			RawStruct: secret,
		}
		view := newListModel(t, "secrets", cfg, []resource.Resource{res})
		if !strings.Contains(view, "Production test secret") {
			t.Errorf("Secrets with production config should show description from RawStruct, got:\n%s", view)
		}
		if strings.Contains(view, "WRONG-DESC") {
			t.Error("Secrets with production config should NOT show WRONG-DESC from Fields")
		}
	})

	t.Run("S3", func(t *testing.T) {
		bucket := s3types.Bucket{
			Name:         ptrString("prod-config-bucket"),
			CreationDate: ptrTime(testTime),
		}
		res := resource.Resource{
			ID:        "prod-config-bucket",
			Name:      "prod-config-bucket",
			Fields:    map[string]string{"creation_date": "WRONG-DATE"},
			RawStruct: bucket,
		}
		view := newListModel(t, "s3", cfg, []resource.Resource{res})
		if !strings.Contains(view, "2025-06-15") {
			t.Errorf("S3 with production config should show creation date from RawStruct, got:\n%s", view)
		}
		if strings.Contains(view, "WRONG-DATE") {
			t.Error("S3 with production config should NOT show WRONG-DATE from Fields")
		}
	})
}

// ===========================================================================
// Test: Fields-only resource (no RawStruct) should still render via fallback
// ===========================================================================

func TestQA_ListRawStruct_FieldsFallbackWhenNoRawStruct(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("ec2")

	// Resource with Fields only, no RawStruct -- should fall back gracefully
	res := resource.Resource{
		ID:   "i-fallback",
		Name: "fallback-instance",
		Fields: map[string]string{
			"instance_id": "i-fallback",
			"state":       "terminated",
			"type":        "t2.micro",
			"private_ip":  "10.0.0.1",
		},
	}

	// Should NOT panic even without RawStruct
	view := newListModel(t, "ec2", cfg, []resource.Resource{res})

	// View should render without error (fallback to Fields or empty)
	if view == "" {
		t.Error("list view should not be empty when Fields are provided without RawStruct")
	}
}

// ===========================================================================
// TestQA_ListRawStruct_AllTypes: table-driven test covering all resource types
// ===========================================================================

func TestQA_ListRawStruct_AllTypes(t *testing.T) {
	ensureNoColor(t)

	tests := []struct {
		shortName    string
		rawStruct    interface{}
		expectInView []string // values that MUST appear from RawStruct
	}{
		// -- Already covered individually above, included for completeness --
		{"ec2", realisticEC2Instance(), []string{"i-0abcdef1234567890", "running", "t3.medium"}},
		{"dbi", realisticRDSInstance(), []string{"prod-db-01", "mysql", "available"}},
		{"redis", realisticRedisCacheCluster(), []string{"redis-prod-001", "7.0.12", "cache.r6g.large"}},
		{"dbc", realisticDocDBCluster(), []string{"docdb-prod-cluster", "5.0.0", "available"}},
		{"eks", realisticEKSCluster(), []string{"prod-cluster", "1.28"}},
		{"secrets", realisticSecretListEntry(), []string{"prod/database/password", "Production database password"}},
		{"s3", realisticS3Bucket(), []string{"my-production-bucket", "2025-06-15"}},

		// -- New types --
		{"lambda", realisticLambdaFunction(), []string{"my-api-handler", "python3.12"}},
		{"alarm", realisticAlarm(), []string{"HighCPUAlarm", "ALARM", "CPUUtilization"}},
		{"sns", realisticSNSTopic(), []string{"arn:aws:sns:us-east-1:123456789012:my-notifications"}},
		{"elb", realisticELB(), []string{"my-app-alb", "application", "internet-faci"}},
		{"tg", realisticTargetGroup(), []string{"my-app-tg", "8080", "HTTP", "/health"}},
		{"ecs", realisticECSClusterStruct(), []string{"prod-cluster", "ACTIVE"}},
		{"ecs-svc", realisticECSService(), []string{"api-service", "ACTIVE", "FARGATE"}},
		{"ecs-task", realisticECSTask(), []string{"RUNNING", "256", "512"}},
		{"cfn", realisticCFNStack(), []string{"my-app-stack", "CREATE_COMPLETE"}},
		{"role", realisticIAMRole(), []string{"lambda-exec-role", "/"}},
		{"logs", realisticLogGroup(), []string{"/aws/lambda/my-api-handler"}},
		{"ssm", realisticSSMParameter(), []string{"/app/config/db-host", "String"}},
		{"ddb", realisticDDBTable(), []string{"users-table", "ACTIVE"}},
		{"acm", realisticACMCertificate(), []string{"example.com", "ISSUED"}},
		{"asg", realisticASG(), []string{"my-app-asg"}},
		{"vpc", realisticVPC(), []string{"vpc-0abc1234def56789a", "10.0.0.0/16", "available"}},
		{"sg", realisticSecurityGroup(), []string{"sg-0abc1234def56789a", "web-sg", "vpc-0abc1234"}},
		{"ng", realisticNodeGroup(), []string{"prod-ng-01", "prod-cluster", "ACTIVE"}},
		{"subnet", realisticSubnet(), []string{"subnet-0abc1234def56789a", "10.0.1.0/24", "us-east-1a"}},
		{"nat", realisticNATGateway(), []string{"nat-0abc1234def56789a", "available"}},
		{"igw", realisticInternetGateway(), []string{"igw-0abc1234def56789a"}},
		{"eip", realisticEIP(), []string{"eipalloc-0abc1234def56789a", "54.123.45.67"}},
		{"tgw", realisticTransitGateway(), []string{"tgw-0abc1234def56789a", "available"}},
		{"vpce", realisticVPCEndpoint(), []string{"vpce-0abc1234def56789a", "com.amazonaws.us-east-1.s3"}},
		{"eni", realisticENI(), []string{"eni-0abc1234def56789a", "in-use", "10.0.1.42"}},
		{"rds-snap", realisticRDSSnapshot(), []string{"rds-snap-prod-20250615", "prod-db-01", "available"}},
		{"docdb-snap", realisticDocDBSnapshot(), []string{"docdb-snap-prod-20250615", "available"}},
		{"sns-sub", realisticSNSSubscription(), []string{"email", "user@example.com"}},
		{"policy", realisticIAMPolicy(), []string{"ReadOnlyAccess", "ANPAI1234567890EXAMPLE"}},
		{"iam-user", realisticIAMUser(), []string{"deploy-user", "AIDAEXAMPLEUSERID"}},
		{"iam-group", realisticIAMGroup(), []string{"developers", "AGPAEXAMPLEGROUPID"}},
		{"cf", realisticCFDistribution(), []string{"E1A2B3C4D5E6F7", "d1234abcdef.cloudfront.net", "Deployed"}},
		{"r53", realisticR53Zone(), []string{"/hostedzone/Z1234567890ABC", "example.com."}},
		{"apigw", realisticAPIGW(), []string{"abc123def4", "prod-api", "HTTP"}},
		{"ecr", realisticECR(), []string{"my-app", "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-app"}},
		{"efs", realisticEFS(), []string{"fs-0abc1234def56789a", "available"}},
		{"eb-rule", realisticEBRule(), []string{"daily-backup-rule", "ENABLED"}},
		{"sfn", realisticSFN(), []string{"order-processing", "STANDARD"}},
		{"pipeline", realisticPipeline(), []string{"deploy-pipeline", "V2"}},
		{"kinesis", realisticKinesis(), []string{"events-stream", "ACTIVE"}},
		{"waf", realisticWAF(), []string{"prod-waf-acl", "a1b2c3d4-5678-90ab-cdef-EXAMPLE11111"}},
		{"glue", realisticGlueJob(), []string{"etl-daily-job", "4.0", "G.2X"}},
		{"eb", realisticEB(), []string{"prod-api-env", "my-web-app", "Ready"}},
		{"ses", realisticSESIdentity(), []string{"example.com", "DOMAIN"}},
		{"redshift", realisticRedshift(), []string{"analytics-cluster", "available", "dc2.large"}},
		{"trail", realisticTrail(), []string{"org-trail", "cloudtrail-logs-bucket"}},
		{"athena", realisticAthena(), []string{"analytics-wg", "ENABLED"}},
		{"codeartifact", realisticCodeArtifact(), []string{"shared-libs", "my-domain"}},
		{"cb", realisticCodeBuild(), []string{"build-project", "CODECOMMIT"}},
		{"opensearch", realisticOpenSearch(), []string{"search-prod", "OpenSearch_2.11"}},
		{"kms", realisticKMS(), []string{"12345678-1234-1234-1234-123456789012", "Enabled"}},
		{"msk", realisticMSK(), []string{"events-kafka", "PROVISIONED", "ACTIVE"}},
		{"backup", realisticBackup(), []string{"daily-backup-plan", "abc12345-1234-1234-1234-123456789012"}},
	}

	for _, tc := range tests {
		t.Run(tc.shortName, func(t *testing.T) {
			cfg := configForType(tc.shortName)
			res := resource.Resource{
				ID:        "test-id",
				Name:      "test-name",
				RawStruct: tc.rawStruct,
			}
			view := newListModel(t, tc.shortName, cfg, []resource.Resource{res})

			for _, expected := range tc.expectInView {
				if !strings.Contains(view, expected) {
					t.Errorf("%s list should contain %q from RawStruct, got:\n%s",
						tc.shortName, expected, view)
				}
			}
		})
	}
}

// ===========================================================================
// TestQA_ListRawStruct_AllTypes_OverridesFields: RawStruct takes priority
// ===========================================================================

func TestQA_ListRawStruct_AllTypes_OverridesFields(t *testing.T) {
	ensureNoColor(t)

	// Each entry has a RawStruct with correct values and Fields with WRONG values.
	// The test verifies that WRONG values do NOT appear when RawStruct is set.
	// Types whose list columns use "key:" instead of "path:" (e.g., sqs, igw
	// for vpc_id/state, nat for public_ip, rtb for counts) will correctly
	// fall back to Fields for those columns -- we only test path-based columns here.
	tests := []struct {
		shortName    string
		rawStruct    interface{}
		wrongFields  map[string]string
		expectInView []string // values that MUST appear from RawStruct
	}{
		{
			"ec2",
			ec2types.Instance{
				InstanceId:   ptrString("i-correct"),
				InstanceType: ec2types.InstanceTypeT3Medium,
				State:        &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning},
			},
			map[string]string{"instance_id": "WRONG-ID", "state": "WRONG-STATE"},
			[]string{"i-correct", "running"},
		},
		{
			"lambda",
			realisticLambdaFunction(),
			map[string]string{"function_name": "WRONG-FN", "runtime": "WRONG-RT"},
			[]string{"my-api-handler", "python3.12"},
		},
		{
			"alarm",
			realisticAlarm(),
			map[string]string{"alarm_name": "WRONG-ALARM", "state_value": "WRONG-STATE"},
			[]string{"HighCPUAlarm", "ALARM"},
		},
		{
			"vpc",
			realisticVPC(),
			map[string]string{"vpc_id": "WRONG-VPC", "cidr_block": "WRONG-CIDR"},
			[]string{"vpc-0abc1234def56789a", "10.0.0.0/16"},
		},
		{
			"sg",
			realisticSecurityGroup(),
			map[string]string{"group_id": "WRONG-SG", "group_name": "WRONG-NAME"},
			[]string{"sg-0abc1234def56789a", "web-sg"},
		},
		{
			"subnet",
			realisticSubnet(),
			map[string]string{"subnet_id": "WRONG-SUB", "cidr_block": "WRONG-CIDR"},
			[]string{"subnet-0abc1234def56789a", "10.0.1.0/24"},
		},
		{
			"eip",
			realisticEIP(),
			map[string]string{"allocation_id": "WRONG-ALLOC", "public_ip": "WRONG-IP"},
			[]string{"eipalloc-0abc1234def56789a", "54.123.45.67"},
		},
		{
			"ecs",
			realisticECSClusterStruct(),
			map[string]string{"cluster_name": "WRONG-CLS", "status": "WRONG-STATUS"},
			[]string{"prod-cluster", "ACTIVE"},
		},
		{
			"cfn",
			realisticCFNStack(),
			map[string]string{"stack_name": "WRONG-STACK", "stack_status": "WRONG-STATUS"},
			[]string{"my-app-stack", "CREATE_COMPLETE"},
		},
		{
			"role",
			realisticIAMRole(),
			map[string]string{"role_name": "WRONG-ROLE", "path": "WRONG-PATH"},
			[]string{"lambda-exec-role", "/"},
		},
		{
			"policy",
			realisticIAMPolicy(),
			map[string]string{"policy_name": "WRONG-POL", "policy_id": "WRONG-PID"},
			[]string{"ReadOnlyAccess", "ANPAI1234567890EXAMPLE"},
		},
		{
			"cf",
			realisticCFDistribution(),
			map[string]string{"id": "WRONG-ID", "domain_name": "WRONG-DN"},
			[]string{"E1A2B3C4D5E6F7", "d1234abcdef.cloudfront.net"},
		},
		{
			"r53",
			realisticR53Zone(),
			map[string]string{"id": "WRONG-ID", "name": "WRONG-NAME"},
			[]string{"/hostedzone/Z1234567890ABC", "example.com."},
		},
		{
			"apigw",
			realisticAPIGW(),
			map[string]string{"api_id": "WRONG-API", "name": "WRONG-NAME"},
			[]string{"abc123def4", "prod-api"},
		},
		{
			"ses",
			realisticSESIdentity(),
			map[string]string{"identity_name": "WRONG-NAME", "identity_type": "WRONG-TYPE"},
			[]string{"example.com", "DOMAIN"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.shortName, func(t *testing.T) {
			cfg := configForType(tc.shortName)
			res := resource.Resource{
				ID:        "test-id",
				Name:      "test-name",
				Fields:    tc.wrongFields,
				RawStruct: tc.rawStruct,
			}
			view := newListModel(t, tc.shortName, cfg, []resource.Resource{res})

			// RawStruct values must appear
			for _, expected := range tc.expectInView {
				if !strings.Contains(view, expected) {
					t.Errorf("%s list should contain %q from RawStruct, got:\n%s",
						tc.shortName, expected, view)
				}
			}

			// WRONG values from Fields must NOT appear
			for _, wrong := range tc.wrongFields {
				if strings.Contains(view, wrong) {
					t.Errorf("%s list should NOT contain %q from Fields when RawStruct is set",
						tc.shortName, wrong)
				}
			}
		})
	}
}

// ===========================================================================
// TestQA_ListRawStruct_SQS_StringRawStruct: SQS uses string, not struct
// ===========================================================================

func TestQA_ListRawStruct_SQS_StringRawStruct(t *testing.T) {
	ensureNoColor(t)
	cfg := configForType("sqs")

	// SQS fetcher sets RawStruct to fmt.Sprintf("%v", attrs) -- a string.
	// All SQS columns use "key:" not "path:", so fieldpath extraction is N/A.
	// This test verifies it doesn't panic and falls back to Fields.
	res := resource.Resource{
		ID:   "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
		Name: "my-queue",
		Fields: map[string]string{
			"queue_name":          "my-queue",
			"approx_messages":     "42",
			"approx_not_visible":  "3",
			"delay_seconds":       "0",
			"queue_url":           "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
		},
		RawStruct: "map[ApproximateNumberOfMessages:42 ApproximateNumberOfMessagesNotVisible:3]",
	}

	view := newListModel(t, "sqs", cfg, []resource.Resource{res})

	// SQS columns use "key:" so Fields values should appear
	if !strings.Contains(view, "my-queue") {
		t.Errorf("SQS list should contain queue name from Fields, got:\n%s", view)
	}
	if !strings.Contains(view, "42") {
		t.Errorf("SQS list should contain message count from Fields, got:\n%s", view)
	}
}

// ===========================================================================
// TestQA_ListRawStruct_S3Objects: both Object and CommonPrefix types
// ===========================================================================

// NOTE: s3_objects is a sub-resource type not registered via resource.FindResourceType.
// It shares the S3 type def and uses a separate config key ("s3_objects") for its
// column layout. Testing it in isolation requires the full S3 drill-down flow,
// which is beyond the scope of this list-level RawStruct test. The S3 object
// RawStruct rendering is covered by the existing S3 object detail tests.
// The realistic builders (realisticS3ObjectFile, realisticS3ObjectFolder) are
// available for those tests.

// Ensure we actually use all the imported types to avoid unused import errors.
// These type assertions are compile-time checks only.
var (
	_ acmtypes.CertificateSummary
	_ apigatewayv2types.Api
	_ athenatypes.WorkGroupSummary
	_ autoscalingtypes.AutoScalingGroup
	_ backuptypes.BackupPlansListMember
	_ cloudfronttypes.DistributionSummary
	_ cfntypes.Stack
	_ cloudtrailtypes.Trail
	_ cwtypes.MetricAlarm
	_ cwlogstypes.LogGroup
	_ codebuildtypes.Project
	_ codeartifacttypes.RepositorySummary
	_ codepipelinetypes.PipelineSummary
	_ docdbtypes.DBCluster
	_ ddbtypes.TableDescription
	_ ec2types.Instance
	_ ecrtypes.Repository
	_ ecstypes.Cluster
	_ efstypes.FileSystemDescription
	_ ebtypes.EnvironmentDescription
	_ elasticachetypes.CacheCluster
	_ elbv2types.LoadBalancer
	_ ekstypes.Cluster
	_ eventbridgetypes.Rule
	_ gluetypes.Job
	_ iamtypes.Policy
	_ kafkatypes.Cluster
	_ kinesistypes.StreamSummary
	_ kmstypes.KeyMetadata
	_ opensearchtypes.DomainStatus
	_ rdstypes.DBInstance
	_ redshifttypes.Cluster
	_ route53types.HostedZone
	_ s3types.Bucket
	_ smtypes.SecretListEntry
	_ sesv2types.IdentityInfo
	_ sfntypes.StateMachineListItem
	_ snstypes.Topic
	_ ssmtypes.ParameterMetadata
	_ wafv2types.WebACLSummary
)
