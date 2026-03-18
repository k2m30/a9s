package unit_test

import (
	"strings"
	"testing"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/views"
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

	cfg, err := config.LoadFrom([]string{"/Users/k2m30/projects/a9s/views.yaml"})
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

