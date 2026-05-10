package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Mock — ElastiCacheDescribeReplicationGroupsAPI
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// rgOutput wraps a single ReplicationGroup into a DescribeReplicationGroupsOutput.
// Engine defaults to "redis" when unset so pre-existing tests don't need to
// restate it. Tests that exercise the engine-filter path (valkey, memcached,
// nil Engine) construct DescribeReplicationGroupsOutput directly.
func rgOutput(rg elasticachetypes.ReplicationGroup) *elasticache.DescribeReplicationGroupsOutput {
	if rg.Engine == nil {
		rg.Engine = aws.String("redis")
	}
	return &elasticache.DescribeReplicationGroupsOutput{
		ReplicationGroups: []elasticachetypes.ReplicationGroup{rg},
	}
}

// fetchOnePage calls FetchRedisPage with the given mock and returns the first
// resource, failing if the result count does not equal 1.
func fetchOnePage(t *testing.T, mock *mockRedisRGClient) (interface{ GetFields() map[string]string }, interface{}) {
	t.Helper()
	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	return nil, result.Resources[0]
}

// ---------------------------------------------------------------------------
// T001 — Healthy available: Fields["status"] == "" (Healthy silence)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// T002 — Status=creating
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// T003 — Status=modifying (with AutomaticFailover enabled — single warning)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// T004 — Status=snapshotting
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// T005 — Status=deleting
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// T006 — Status=create-failed (Broken)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// T007 — MultiAZ=enabled, AutomaticFailover=disabled (single warning)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// T008 — MultiAZ=disabled, AutomaticFailover=disabled (single-AZ — no finding)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// T009 — Multi-W1 (U7a): Status=modifying + MultiAZ without auto-failover
// ---------------------------------------------------------------------------

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
	// Findings must be in §4 precedence order: alphabetical among warnings.
	// "modifying — config change" < "multi-AZ without auto-failover" alphabetically.
	if r.Findings[0].Phrase != "modifying \u2014 config change" {
		t.Errorf("Findings[0].Phrase = %q, want %q", r.Findings[0].Phrase, "modifying \u2014 config change")
	}
	if r.Findings[1].Phrase != "multi-AZ without auto-failover" {
		t.Errorf("Findings[1].Phrase = %q, want %q", r.Findings[1].Phrase, "multi-AZ without auto-failover")
	}
}

// ---------------------------------------------------------------------------
// T010 — Column population: cluster_id, node_type, nodes, endpoint
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// T011 — Anti-test: no CloudWatch metric fields invented (Wave 3 out of scope)
// ---------------------------------------------------------------------------

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

	// Wave 3 metric fields must NOT be populated by the Wave 1 fetcher.
	forbiddenKeys := []string{"memory_pressure", "evictions", "replication_lag", "engine_cpu"}
	for _, key := range forbiddenKeys {
		if val, ok := r.Fields[key]; ok {
			t.Errorf("Fields[%q] = %q should not exist — Wave 3 metrics are out of scope for Wave 1 fetcher", key, val)
		}
	}
	// Verify status is still healthy-silence (no metric override).
	if r.Fields["status"] != "" {
		t.Errorf("Fields[\"status\"] = %q, want %q (healthy group)", r.Fields["status"], "")
	}
}

// ---------------------------------------------------------------------------
// §0b.1 — Engine filter: Valkey / Memcached / nil Engine RGs must be dropped
// ---------------------------------------------------------------------------

// rgOutputMulti wraps multiple ReplicationGroups into a DescribeReplicationGroupsOutput.
func rgOutputMulti(rgs ...elasticachetypes.ReplicationGroup) *elasticache.DescribeReplicationGroupsOutput {
	return &elasticache.DescribeReplicationGroupsOutput{
		ReplicationGroups: rgs,
	}
}

// TestRedis_Fetch_SkipsNonRedisEngines verifies that a Valkey RG is excluded
// and only the Redis RG is returned. This is the regression pin for §0b.1.
// EXPECTED FAIL until coder adds engine filter in FetchRedisPage.
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

// TestRedis_Fetch_MemcachedEngineFiltered verifies that a Memcached RG is
// excluded and only the Redis RG is returned.
// EXPECTED FAIL until coder adds engine filter in FetchRedisPage.
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

// TestRedis_Fetch_NilEngineFiltered verifies that a RG with Engine==nil is
// treated as non-redis and dropped (defensive nil guard per §0b.1).
// EXPECTED FAIL until coder adds engine filter in FetchRedisPage.
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

// resourceIDs returns the IDs of a slice of resources for error messages.
func resourceIDs(rs []resource.Resource) []string {
	ids := make([]string, len(rs))
	for i, r := range rs {
		ids[i] = r.ID
	}
	return ids
}

// ---------------------------------------------------------------------------
// §0b.4 — Shard-level signals for multi-shard RGs
// ---------------------------------------------------------------------------

// nodeGroup is a convenience constructor for an elasticachetypes.NodeGroup.
func nodeGroup(id, status string) elasticachetypes.NodeGroup {
	return elasticachetypes.NodeGroup{
		NodeGroupId: aws.String(id),
		Status:      aws.String(status),
	}
}

// TestRedis_Fetch_SingleShardModifying_UsesRGPhrase verifies that a single-shard
// RG (NodeGroups==1) with Status=modifying uses the old RG-level phrase, not a
// shard-scoped phrase. Single-shard behavior must be preserved.
// NOTE: this test passes with the CURRENT code (no shard logic yet) because the
// current code already emits the RG-level phrase for all modifying groups.
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

// TestRedis_Fetch_MultiShard_OneShardModifying verifies that a 3-shard RG with
// only shard 0001 modifying emits a shard-scoped phrase.
// EXPECTED FAIL until coder adds per-NodeGroup logic in computeRedisIssues.
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

// TestRedis_Fetch_MultiShard_TwoShardsTransitioning verifies that a 3-shard RG
// with 0001 modifying and 0002 snapshotting emits the leading phrase with (+1).
// Rule 7: top phrase is alphabetically first; hidden count is 1.
// Alphabetical: "shard 0001: modifying" < "shard 0002: snapshotting".
// EXPECTED FAIL until coder adds per-NodeGroup logic in computeRedisIssues.
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

// TestRedis_Fetch_MultiShard_AllShardAvailableButRGModifying verifies that when
// all NodeGroups are available but the RG itself reports Status=modifying, the
// fetcher falls back to the RG-level phrase (transient state).
// EXPECTED FAIL until coder adds the anyShard fallback in computeRedisIssues.
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

// TestRedis_Fetch_MultiShard_ShardPlusMultiAZNoFailover verifies Rule 7 when
// a shard-level phrase coexists with the multi-AZ without auto-failover warning.
// Alphabetical: "multi-AZ without auto-failover" < "shard 0001: modifying"
// ("multi" < "shard") → multi-AZ phrase is the top phrase; shard is hidden.
// EXPECTED FAIL until coder adds per-NodeGroup logic in computeRedisIssues.
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
	// Top phrase: "multi-AZ without auto-failover"; hidden count: 1.
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
