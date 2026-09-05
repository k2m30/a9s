package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// d1RedisBaseline is a healthy single-shard replication group: available,
// multi-AZ with automatic failover on. rgOutput fills the four encryption and
// backup posture fields, so no case here carries a posture finding it did not
// ask for.
func d1RedisBaseline() elasticachetypes.ReplicationGroup {
	return elasticachetypes.ReplicationGroup{
		ReplicationGroupId: aws.String("prod-redis-sessions"),
		ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions"),
		Engine:             aws.String("redis"),
		Status:             aws.String("available"),
		CacheNodeType:      aws.String("cache.r6g.large"),
		MemberClusters:     []string{"prod-redis-sessions-001"},
		MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
		AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
	}
}

// d1RedisShards returns two node groups, the first in state, the second
// available — the shape that makes computeShardIssues report per shard rather
// than falling back to the group-level phrase.
func d1RedisShards(state string) []elasticachetypes.NodeGroup {
	return []elasticachetypes.NodeGroup{
		{NodeGroupId: aws.String("0001"), Status: aws.String(state)},
		{NodeGroupId: aws.String("0002"), Status: aws.String("available")},
	}
}

// TestRedisColor pins the status → colour mapping for ElastiCache replication
// groups. Each case drives an SDK ReplicationGroup through the redis fetcher and
// compares the colour its findings imply.
//
// The "(+N)" rows are gone: the suffix belongs to the rendered phrase, and the
// severity comparison that decides colour never sees it. The shard rows stay,
// because a per-shard phrase is a real finding on a real multi-shard group.
func TestRedisColor(t *testing.T) {
	cases := []struct {
		name   string
		status string
		mutate func(*elasticachetypes.ReplicationGroup)
		want   resource.Color
	}{
		{name: "available", status: "available", want: resource.ColorHealthy},
		{name: "empty_status", status: "", want: resource.ColorHealthy},

		{name: "creating", status: "creating", want: resource.ColorWarning},
		{name: "modifying", status: "modifying", want: resource.ColorWarning},
		{name: "snapshotting", status: "snapshotting", want: resource.ColorWarning},
		{name: "deleting", status: "deleting", want: resource.ColorWarning},

		{
			name:   "multiaz_without_auto_failover",
			status: "available",
			mutate: func(rg *elasticachetypes.ReplicationGroup) {
				rg.AutomaticFailover = elasticachetypes.AutomaticFailoverStatusDisabled
			},
			want: resource.ColorWarning,
		},

		{name: "create_failed", status: "create-failed", want: resource.ColorBroken},

		{
			name:   "shard_modifying",
			status: "modifying",
			mutate: func(rg *elasticachetypes.ReplicationGroup) { rg.NodeGroups = d1RedisShards("modifying") },
			want:   resource.ColorWarning,
		},
		{
			name:   "shard_snapshotting",
			status: "snapshotting",
			mutate: func(rg *elasticachetypes.ReplicationGroup) { rg.NodeGroups = d1RedisShards("snapshotting") },
			want:   resource.ColorWarning,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rg := d1RedisBaseline()
			rg.Status = aws.String(tc.status)
			if tc.mutate != nil {
				tc.mutate(&rg)
			}
			mock := &mockRedisRGClient{output: rgOutput(rg)}
			page, err := awsclient.FetchRedisPage(context.Background(), mock, "")
			if err != nil {
				t.Fatalf("FetchRedisPage: %v", err)
			}
			if len(page.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(page.Resources))
			}
			d1AssertColor(t, page.Resources[0], tc.want)
		})
	}
}
