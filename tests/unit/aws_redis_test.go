package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type mockRedisRGClient struct {
	output *elasticache.DescribeReplicationGroupsOutput
	err    error
}

func (m *mockRedisRGClient) DescribeReplicationGroups(
	_ context.Context,
	_ *elasticache.DescribeReplicationGroupsInput,
	_ ...func(*elasticache.Options),
) (*elasticache.DescribeReplicationGroupsOutput, error) {
	return m.output, m.err
}

// rgOutput defaults Engine to "redis" when unset; engine-filter tests build
// DescribeReplicationGroupsOutput directly.
func rgOutput(rg elasticachetypes.ReplicationGroup) *elasticache.DescribeReplicationGroupsOutput {
	if rg.Engine == nil {
		rg.Engine = aws.String("redis")
	}
	return &elasticache.DescribeReplicationGroupsOutput{
		ReplicationGroups: []elasticachetypes.ReplicationGroup{w2RedisHealthyPosture(rg)},
	}
}

// w2RedisHealthyPosture fills the posture fields a fixture leaves unset, so
// lifecycle and shard-status tests do not also assert posture findings.
func w2RedisHealthyPosture(rg elasticachetypes.ReplicationGroup) elasticachetypes.ReplicationGroup {
	if rg.AtRestEncryptionEnabled == nil {
		rg.AtRestEncryptionEnabled = aws.Bool(true)
	}
	if rg.TransitEncryptionEnabled == nil {
		rg.TransitEncryptionEnabled = aws.Bool(true)
	}
	if rg.AuthTokenEnabled == nil {
		rg.AuthTokenEnabled = aws.Bool(true)
	}
	if rg.SnapshotRetentionLimit == nil {
		rg.SnapshotRetentionLimit = aws.Int32(7)
	}
	return rg
}

func TestRedis_Fetch_HealthyAvailable(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("prod-redis-sessions"),
			Status:             aws.String("available"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			CacheNodeType:      aws.String("cache.r6g.large"),
			MemberClusters:     []string{"prod-redis-sessions-001"},
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions"),
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]
	if r.ID != "prod-redis-sessions" {
		t.Errorf("ID = %q, want %q", r.ID, "prod-redis-sessions")
	}
	if r.Fields["status"] != "" {
		t.Errorf("Fields[\"status\"] = %q, want %q (Healthy silence)", r.Fields["status"], "")
	}
	if len(r.Findings) != 0 {
		t.Errorf("Findings = %v, want empty (Healthy)", r.Findings)
	}
}

func TestRedis_Fetch_StatusCreating(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("dev-feature-redis"),
			Status:             aws.String("creating"),
			MultiAZ:            elasticachetypes.MultiAZStatusDisabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusDisabling,
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	const wantPhrase = "creating \u2014 new group"
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, wantPhrase)
	}
}

func TestRedis_Fetch_StatusModifying(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("prod-redis-cache"),
			Status:             aws.String("modifying"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	const wantPhrase = "modifying \u2014 config change"
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, wantPhrase)
	}
}

func TestRedis_Fetch_StatusSnapshotting(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("prod-redis-analytics"),
			Status:             aws.String("snapshotting"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	const wantPhrase = "snapshotting \u2014 backup running"
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, wantPhrase)
	}
}

func TestRedis_Fetch_StatusDeleting(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("old-redis-unused"),
			Status:             aws.String("deleting"),
			MultiAZ:            elasticachetypes.MultiAZStatusDisabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusDisabled,
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	const wantPhrase = "deleting \u2014 teardown"
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, wantPhrase)
	}
}

func TestRedis_Fetch_StatusCreateFailed(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("bad-config-redis"),
			Status:             aws.String("create-failed"),
			MultiAZ:            elasticachetypes.MultiAZStatusDisabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusDisabled,
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	const wantPhrase = "create failed \u2014 see events"
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, wantPhrase)
	}
}

func TestRedis_Fetch_MultiAZWithoutFailover(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("legacy-redis-analytics"),
			Status:             aws.String("available"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusDisabled,
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	const wantPhrase = "multi-AZ without auto-failover"
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want one finding with Phrase %q", r.Findings, wantPhrase)
	}
}

func TestRedis_Fetch_MultiAZDisabled_NoFinding(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("staging-redis"),
			Status:             aws.String("available"),
			MultiAZ:            elasticachetypes.MultiAZStatusDisabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusDisabled,
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	if r.Fields["status"] != "" {
		t.Errorf("Fields[\"status\"] = %q, want %q (single-AZ groups do not trigger the signal)", r.Fields["status"], "")
	}
	if len(r.Findings) != 0 {
		t.Errorf("Findings = %v, want empty (no signal for single-AZ)", r.Findings)
	}
}

func TestRedis_Fetch_MultiW1_ModifyingPlusNoFailover(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("legacy-redis-billing"),
			Status:             aws.String("modifying"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusDisabled,
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	const wantStatus = "modifying \u2014 config change (+1)"
	if r.Fields["status"] != wantStatus {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantStatus)
	}
	if len(r.Findings) != 2 {
		t.Fatalf("Findings len = %d, want 2; Findings = %v", len(r.Findings), r.Findings)
	}
	// Warnings sort alphabetically by phrase.
	if r.Findings[0].Phrase != "modifying \u2014 config change" {
		t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "modifying \u2014 config change")
	}
	if r.Findings[1].Phrase != "multi-AZ without auto-failover" {
		t.Errorf("Findings[1].Phrase = %q, want %q", r.Findings[1].Phrase, "multi-AZ without auto-failover")
	}
}

func TestRedis_Fetch_PopulatesColumns(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("prod-redis-sessions"),
			Status:             aws.String("available"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			CacheNodeType:      aws.String("cache.r6g.large"),
			MemberClusters:     []string{"prod-redis-sessions-001", "prod-redis-sessions-002", "prod-redis-sessions-003"},
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("prod-redis-sessions.cfg.use1.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			ARN: aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions"),
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	if r.Fields["cluster_id"] != "prod-redis-sessions" {
		t.Errorf("Fields[\"cluster_id\"] = %q, want %q", r.Fields["cluster_id"], "prod-redis-sessions")
	}
	if r.Fields["node_type"] != "cache.r6g.large" {
		t.Errorf("Fields[\"node_type\"] = %q, want %q", r.Fields["node_type"], "cache.r6g.large")
	}
	if r.Fields["nodes"] != "3" {
		t.Errorf("Fields[\"nodes\"] = %q, want %q", r.Fields["nodes"], "3")
	}
	if r.Fields["endpoint"] != "prod-redis-sessions.cfg.use1.cache.amazonaws.com" {
		t.Errorf("Fields[\"endpoint\"] = %q, want %q", r.Fields["endpoint"], "prod-redis-sessions.cfg.use1.cache.amazonaws.com")
	}
}

func TestRedis_Wave3_NoMetricFieldsInvented(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("prod-redis-sessions"),
			Status:             aws.String("available"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	forbiddenKeys := []string{"memory_pressure", "evictions", "replication_lag", "engine_cpu"}
	for _, key := range forbiddenKeys {
		if val, ok := r.Fields[key]; ok {
			t.Errorf("Fields[%q] = %q should not exist — Wave 3 metrics are out of scope for Wave 1 fetcher", key, val)
		}
	}
	if r.Fields["status"] != "" {
		t.Errorf("Fields[\"status\"] = %q, want %q (healthy group)", r.Fields["status"], "")
	}
}

func rgOutputMulti(rgs ...elasticachetypes.ReplicationGroup) *elasticache.DescribeReplicationGroupsOutput {
	out := make([]elasticachetypes.ReplicationGroup, 0, len(rgs))
	for _, rg := range rgs {
		out = append(out, w2RedisHealthyPosture(rg))
	}
	return &elasticache.DescribeReplicationGroupsOutput{
		ReplicationGroups: out,
	}
}

func TestRedis_Fetch_SkipsNonRedisEngines(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutputMulti(
			elasticachetypes.ReplicationGroup{
				ReplicationGroupId: aws.String("prod-redis"),
				Status:             aws.String("available"),
				Engine:             aws.String("redis"),
			},
			elasticachetypes.ReplicationGroup{
				ReplicationGroupId: aws.String("prod-valkey"),
				Status:             aws.String("available"),
				Engine:             aws.String("valkey"),
			},
		),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource (redis only), got %d: %v",
			len(result.Resources), resourceIDs(result.Resources))
	}
	if result.Resources[0].ID != "prod-redis" {
		t.Errorf("ID = %q, want %q", result.Resources[0].ID, "prod-redis")
	}
}

func TestRedis_Fetch_MemcachedEngineFiltered(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutputMulti(
			elasticachetypes.ReplicationGroup{
				ReplicationGroupId: aws.String("prod-redis"),
				Status:             aws.String("available"),
				Engine:             aws.String("redis"),
			},
			elasticachetypes.ReplicationGroup{
				ReplicationGroupId: aws.String("prod-memcached"),
				Status:             aws.String("available"),
				Engine:             aws.String("memcached"),
			},
		),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource (redis only), got %d: %v",
			len(result.Resources), resourceIDs(result.Resources))
	}
	if result.Resources[0].ID != "prod-redis" {
		t.Errorf("ID = %q, want %q", result.Resources[0].ID, "prod-redis")
	}
}

func TestRedis_Fetch_NilEngineFiltered(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutputMulti(
			elasticachetypes.ReplicationGroup{
				ReplicationGroupId: aws.String("prod-redis"),
				Status:             aws.String("available"),
				Engine:             aws.String("redis"),
			},
			elasticachetypes.ReplicationGroup{
				ReplicationGroupId: aws.String("rg-no-engine"),
				Status:             aws.String("available"),
				Engine:             nil,
			},
		),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource (redis only), got %d: %v",
			len(result.Resources), resourceIDs(result.Resources))
	}
	if result.Resources[0].ID != "prod-redis" {
		t.Errorf("ID = %q, want %q", result.Resources[0].ID, "prod-redis")
	}
}

func resourceIDs(rs []resource.Resource) []string {
	ids := make([]string, len(rs))
	for i, r := range rs {
		ids[i] = r.ID
	}
	return ids
}

func nodeGroup(id, status string) elasticachetypes.NodeGroup {
	return elasticachetypes.NodeGroup{
		NodeGroupId: aws.String(id),
		Status:      aws.String(status),
	}
}

func TestRedis_Fetch_SingleShardModifying_UsesRGPhrase(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("single-shard-modifying"),
			Status:             aws.String("modifying"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			NodeGroups: []elasticachetypes.NodeGroup{
				nodeGroup("0001", "modifying"),
			},
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	const wantPhrase = "modifying \u2014 config change"
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q (single-shard preserves RG phrase)", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want [%q]", r.Findings, wantPhrase)
	}
}

func TestRedis_Fetch_MultiShard_OneShardModifying(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("multi-shard-modifying"),
			Status:             aws.String("modifying"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			NodeGroups: []elasticachetypes.NodeGroup{
				nodeGroup("0001", "modifying"),
				nodeGroup("0002", "available"),
				nodeGroup("0003", "available"),
			},
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	const wantPhrase = "shard 0001: modifying"
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want [%q]", r.Findings, wantPhrase)
	}
}

func TestRedis_Fetch_MultiShard_TwoShardsTransitioning(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("multi-shard-two-transitioning"),
			Status:             aws.String("modifying"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			NodeGroups: []elasticachetypes.NodeGroup{
				nodeGroup("0001", "modifying"),
				nodeGroup("0002", "snapshotting"),
				nodeGroup("0003", "available"),
			},
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	const wantStatus = "shard 0001: modifying (+1)"
	if r.Fields["status"] != wantStatus {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantStatus)
	}
	if len(r.Findings) != 2 {
		t.Fatalf("Findings len = %d, want 2; Findings = %v", len(r.Findings), r.Findings)
	}
	// Alphabetical by phrase: "shard 0001: modifying" < "shard 0002: snapshotting".
	if r.Findings[0].Phrase != "shard 0001: modifying" {
		t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "shard 0001: modifying")
	}
	if r.Findings[1].Phrase != "shard 0002: snapshotting" {
		t.Errorf("Findings[1].Phrase = %q, want %q", r.Findings[1].Phrase, "shard 0002: snapshotting")
	}
}

func TestRedis_Fetch_MultiShard_AllShardAvailableButRGModifying(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("multi-shard-rg-modifying"),
			Status:             aws.String("modifying"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			NodeGroups: []elasticachetypes.NodeGroup{
				nodeGroup("0001", "available"),
				nodeGroup("0002", "available"),
				nodeGroup("0003", "available"),
			},
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	// Transient: RG is modifying but no specific shard is — fall back to RG phrase.
	const wantPhrase = "modifying \u2014 config change"
	if r.Fields["status"] != wantPhrase {
		t.Errorf("Fields[\"status\"] = %q, want %q (fallback to RG phrase when no shard is transitioning)", r.Fields["status"], wantPhrase)
	}
	if len(r.Findings) != 1 || r.Findings[0].Phrase != wantPhrase {
		t.Errorf("Findings = %v, want [%q]", r.Findings, wantPhrase)
	}
}

func TestRedis_Fetch_MultiShard_ShardPlusMultiAZNoFailover(t *testing.T) {
	mock := &mockRedisRGClient{
		output: rgOutput(elasticachetypes.ReplicationGroup{
			ReplicationGroupId: aws.String("multi-shard-no-failover"),
			Status:             aws.String("modifying"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusDisabled,
			NodeGroups: []elasticachetypes.NodeGroup{
				nodeGroup("0001", "modifying"),
				nodeGroup("0002", "available"),
				nodeGroup("0003", "available"),
			},
		}),
	}
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	r := result.Resources[0]

	// Alphabetical: "multi-AZ without auto-failover" < "shard 0001: modifying".
	const wantStatus = "multi-AZ without auto-failover (+1)"
	if r.Fields["status"] != wantStatus {
		t.Errorf("Fields[\"status\"] = %q, want %q", r.Fields["status"], wantStatus)
	}
	if len(r.Findings) != 2 {
		t.Fatalf("Findings len = %d, want 2; Findings = %v", len(r.Findings), r.Findings)
	}
	if r.Findings[0].Phrase != "multi-AZ without auto-failover" {
		t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "multi-AZ without auto-failover")
	}
	if r.Findings[1].Phrase != "shard 0001: modifying" {
		t.Errorf("Findings[1].Phrase = %q, want %q", r.Findings[1].Phrase, "shard 0001: modifying")
	}
}
