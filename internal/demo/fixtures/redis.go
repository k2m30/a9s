// Package fixtures provides ElastiCache Redis fixture data.
// This is the single-source fixture file for the redis resource type.
// Both ./a9s --demo and the unit test suite import from here.
package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
)

// Exported stable constants referenced from this file and sibling fixture files.
const (
	// ProdRedisID is the ReplicationGroupId of the graph-root redis fixture.
	// Every registered related-panel pivot resolves non-zero against this group.
	ProdRedisID = "prod-redis-sessions"

	// ProdRedisARN is the ARN of the graph-root replication group.
	ProdRedisARN = "arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions"

	// ProdRedisMemberClusterID is the primary member CacheCluster of the graph-root RG.
	// Related checkers that need SG / SNS / subnet data call DescribeCacheClusters on this ID.
	ProdRedisMemberClusterID = "prod-redis-sessions-001"

	// ProdRedisMemberClusterARN is the ARN of the primary member CacheCluster.
	// checkRedisCFN calls ListTagsForResource on this ARN (the CacheCluster ARN, not the RG ARN),
	// so the TagLists map must carry an entry keyed on this value.
	ProdRedisMemberClusterARN = "arn:aws:elasticache:us-east-1:123456789012:cluster:prod-redis-sessions-001"

	// ProdRedisSGID is the security group attached to the primary member cluster.
	// Must exist in ec2.go (SecurityGroups slice).
	ProdRedisSGID = "sg-redis-prod-a"

	// ProdRedisSubnetA and ProdRedisSubnetB are the subnets in the Redis subnet group.
	// Must exist in ec2.go (Subnets slice).
	ProdRedisSubnetA = "subnet-prod-a"
	ProdRedisSubnetB = "subnet-prod-b"

	// ProdRedisVpcID is the VPC that hosts the Redis subnet group.
	// Must exist in ec2.go (Vpcs slice).
	ProdRedisVpcID = "vpc-prod-main"

	// ProdRedisKMSKeyID is the bare KMS key ID used for at-rest encryption.
	// Must exist in kms.go (Keys map).
	ProdRedisKMSKeyID = "11111111-1111-1111-1111-111111111111"

	// ProdRedisKMSKeyARN is the full ARN form of ProdRedisKMSKeyID.
	ProdRedisKMSKeyARN = "arn:aws:kms:us-east-1:123456789012:key/11111111-1111-1111-1111-111111111111"

	// ProdRedisLogGroup is the CloudWatch Logs group receiving slow-log output.
	// Must exist in cwlogs.go (LogGroups slice).
	ProdRedisLogGroup = "/aws/elasticache/redis/prod-redis-sessions/slow-log"

	// ProdRedisSNSTopicARN is the SNS topic ARN for ElastiCache lifecycle events.
	ProdRedisSNSTopicARN = "arn:aws:sns:us-east-1:123456789012:redis-ops-pager"

	// ProdRedisSNSTopicName is the bare topic name extracted from ProdRedisSNSTopicARN.
	// Must exist in sns.go (Topics slice).
	ProdRedisSNSTopicName = "redis-ops-pager"

	// ProdRedisCFNStack is the CloudFormation stack that owns the graph-root RG.
	// Must exist in cfn.go (Stacks slice).
	// The RG's TagLists entry must carry aws:cloudformation:stack-name = this value.
	ProdRedisCFNStack = "acme-prod-redis"

	// ProdRedisSecretName is the Secrets Manager secret holding the AUTH token.
	// Must exist in secrets.go (Secrets slice) by name or tag.
	ProdRedisSecretName = "prod-redis-sessions/auth-token"

	// ProdRedisSubnetGroup is the CacheSubnetGroup name for the graph-root RG.
	ProdRedisSubnetGroup = "prod-redis-subnet-group"

	// WarnRedisMultiID is the ReplicationGroupId of the multi-W1 fixture (U7a).
	WarnRedisMultiID = "legacy-redis-billing"

	// ValkeyEngineID is the ReplicationGroupId of the Valkey fixture.
	// Used as a regression pin to verify the engine filter correctly excludes
	// non-Redis engines from the redis fetcher (P2-1).
	ValkeyEngineID = "prod-valkey"

	// MultiShardHealthyID is the ReplicationGroupId of the multi-shard healthy fixture.
	// ClusterEnabled=true, 3 NodeGroups all available.
	// Carries full graph-connectivity tags matching the prod-redis-sessions fixture
	// so related-pivot scenario tests can exercise multi-shard groups.
	MultiShardHealthyID = "multi-shard-healthy"

	// MultiShardOneModifyingID is the ReplicationGroupId of the multi-shard one-modifying fixture.
	// 3 NodeGroups, shard 0001 modifying, 0002 + 0003 available.
	// Expected Fields["status"] == "shard 0001: modifying".
	MultiShardOneModifyingID = "multi-shard-modifying-0001"

	// MultiShardTwoTransitioningID is the ReplicationGroupId of the multi-shard two-transitioning fixture.
	// 3 NodeGroups, shard 0001 modifying + 0002 snapshotting + 0003 available.
	// Expected Fields["status"] == "shard 0001: modifying (+1)".
	// Value kept short (≤ 27 chars) so the rendered Cluster ID column does not truncate.
	MultiShardTwoTransitioningID = "multi-shard-2-transitioning"
)

// RedisFixtures holds typed fixture data for the ElastiCache Redis resource type.
type RedisFixtures struct {
	// ReplicationGroups is the primary list returned by DescribeReplicationGroups.
	// This is the list API source for the redis resource type.
	ReplicationGroups []elasticachetypes.ReplicationGroup

	// CacheClusters is the member-cluster list returned by DescribeCacheClusters.
	// Related checkers that need SG / SNS / subnet-group data call DescribeCacheClusters
	// on a member cluster ID derived from ReplicationGroup.MemberClusters[].
	CacheClusters []elasticachetypes.CacheCluster

	// SubnetGroups is the list returned by DescribeCacheSubnetGroups.
	// The subnet and vpc related checkers resolve via this slice keyed on CacheSubnetGroupName.
	SubnetGroups []elasticachetypes.CacheSubnetGroup

	// TagLists maps a replication-group ARN to its tag list.
	// The cfn related checker calls ListTagsForResource(ResourceName=RG ARN) and looks up here.
	TagLists map[string][]elasticachetypes.Tag
}

// NewRedisFixtures constructs a fully-populated RedisFixtures instance.
// Fixtures cover every §3.1 signal from docs/resources/redis.md plus the
// multi-W1 case (U7a). The graph-root (prod-redis-sessions) carries matching
// sibling entries for all 10 registered related-panel pivots.
func NewRedisFixtures() *RedisFixtures {
	return &RedisFixtures{
		ReplicationGroups: buildRedisReplicationGroups(),
		CacheClusters:     buildRedisCacheClusters(),
		SubnetGroups:      buildRedisCacheSubnetGroups(),
		TagLists:          buildRedisTagLists(),
	}
}

// ---------------------------------------------------------------------------
// ReplicationGroups
// ---------------------------------------------------------------------------

func buildRedisReplicationGroups() []elasticachetypes.ReplicationGroup {
	return []elasticachetypes.ReplicationGroup{
		// GRAPH ROOT — every §2 related pivot resolves non-zero here.
		// Healthy: Status=available, MultiAZ=enabled, AutomaticFailover=enabled.
		// Expected Fields["status"] == "" (Healthy silence per §4).
		{
			ReplicationGroupId:  aws.String(ProdRedisID),
			Description:         aws.String("Prod sessions Redis"),
			ARN:                 aws.String(ProdRedisARN),
			Status:              aws.String("available"),
			Engine:              aws.String("redis"),
			MultiAZ:             elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:   elasticachetypes.AutomaticFailoverStatusEnabled,
			CacheNodeType:       aws.String("cache.r6g.large"),
			KmsKeyId:            aws.String(ProdRedisKMSKeyARN),
			MemberClusters:      []string{ProdRedisMemberClusterID, "prod-redis-sessions-002", "prod-redis-sessions-003"},
			AtRestEncryptionEnabled: aws.Bool(true),
			TransitEncryptionEnabled: aws.Bool(true),
			AuthTokenEnabled:    aws.Bool(true),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("prod-redis-sessions.cfg.use1.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			LogDeliveryConfigurations: []elasticachetypes.LogDeliveryConfiguration{
				{
					LogType:         elasticachetypes.LogTypeSlowLog,
					LogFormat:       elasticachetypes.LogFormatText,
					DestinationType: elasticachetypes.DestinationTypeCloudWatchLogs,
					DestinationDetails: &elasticachetypes.DestinationDetails{
						CloudWatchLogsDetails: &elasticachetypes.CloudWatchLogsDestinationDetails{
							LogGroup: aws.String(ProdRedisLogGroup),
						},
					},
					Status: elasticachetypes.LogDeliveryConfigurationStatusActive,
				},
			},
			SnapshotRetentionLimit: aws.Int32(1),
			SnapshotWindow:         aws.String("05:00-06:00"),
		},

		// Healthy, single-AZ — Status=available, MultiAZ=disabled, AutomaticFailover=disabled.
		// Single-AZ groups do not trigger multi-AZ-without-auto-failover finding (§4 note).
		// Expected Fields["status"] == "".
		{
			ReplicationGroupId:   aws.String("staging-redis"),
			Description:          aws.String("Staging Redis"),
			ARN:                  aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:staging-redis"),
			Status:               aws.String("available"),
			Engine:               aws.String("redis"),
			MultiAZ:              elasticachetypes.MultiAZStatusDisabled,
			AutomaticFailover:    elasticachetypes.AutomaticFailoverStatusDisabled,
			CacheNodeType:        aws.String("cache.t3.medium"),
			MemberClusters:       []string{"staging-redis-001"},
			AtRestEncryptionEnabled: aws.Bool(false),
			TransitEncryptionEnabled: aws.Bool(false),
			AuthTokenEnabled:     aws.Bool(false),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("staging-redis.cfg.use1.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			SnapshotRetentionLimit: aws.Int32(0),
		},

		// Warning: Status=creating — new group being provisioned.
		// Expected Fields["status"] == "creating — new group".
		{
			ReplicationGroupId:   aws.String("dev-feature-redis"),
			Description:          aws.String("Dev feature Redis (creating)"),
			ARN:                  aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:dev-feature-redis"),
			Status:               aws.String("creating"),
			Engine:               aws.String("redis"),
			MultiAZ:              elasticachetypes.MultiAZStatusDisabled,
			AutomaticFailover:    elasticachetypes.AutomaticFailoverStatusDisabling,
			CacheNodeType:        aws.String("cache.t3.medium"),
			MemberClusters:       []string{},
			AtRestEncryptionEnabled: aws.Bool(true),
			TransitEncryptionEnabled: aws.Bool(true),
			AuthTokenEnabled:     aws.Bool(false),
			SnapshotRetentionLimit: aws.Int32(0),
		},

		// Warning: Status=modifying — config change in progress.
		// MultiAZ=enabled, AutomaticFailover=enabled (no additional warning).
		// Expected Fields["status"] == "modifying — config change".
		{
			ReplicationGroupId:   aws.String("prod-redis-cache"),
			Description:          aws.String("Prod cache Redis (modifying)"),
			ARN:                  aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-cache"),
			Status:               aws.String("modifying"),
			Engine:               aws.String("redis"),
			MultiAZ:              elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:    elasticachetypes.AutomaticFailoverStatusEnabled,
			CacheNodeType:        aws.String("cache.r6g.large"),
			MemberClusters:       []string{"prod-redis-cache-001", "prod-redis-cache-002"},
			AtRestEncryptionEnabled: aws.Bool(true),
			TransitEncryptionEnabled: aws.Bool(true),
			AuthTokenEnabled:     aws.Bool(false),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("prod-redis-cache.cfg.use1.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			SnapshotRetentionLimit: aws.Int32(1),
		},

		// Warning: Status=snapshotting — backup running.
		// Expected Fields["status"] == "snapshotting — backup running".
		{
			ReplicationGroupId:   aws.String("prod-redis-analytics"),
			Description:          aws.String("Prod analytics Redis (snapshotting)"),
			ARN:                  aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-analytics"),
			Status:               aws.String("snapshotting"),
			Engine:               aws.String("redis"),
			MultiAZ:              elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:    elasticachetypes.AutomaticFailoverStatusEnabled,
			CacheNodeType:        aws.String("cache.r6g.large"),
			MemberClusters:       []string{"prod-redis-analytics-001", "prod-redis-analytics-002"},
			AtRestEncryptionEnabled: aws.Bool(true),
			TransitEncryptionEnabled: aws.Bool(true),
			AuthTokenEnabled:     aws.Bool(false),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("prod-redis-analytics.cfg.use1.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			SnapshotRetentionLimit: aws.Int32(7),
		},

		// Warning: Status=deleting — teardown in progress.
		// Expected Fields["status"] == "deleting — teardown".
		{
			ReplicationGroupId:   aws.String("old-redis-unused"),
			Description:          aws.String("Old Redis (deleting)"),
			ARN:                  aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:old-redis-unused"),
			Status:               aws.String("deleting"),
			Engine:               aws.String("redis"),
			MultiAZ:              elasticachetypes.MultiAZStatusDisabled,
			AutomaticFailover:    elasticachetypes.AutomaticFailoverStatusDisabled,
			CacheNodeType:        aws.String("cache.t3.medium"),
			MemberClusters:       []string{"old-redis-unused-001"},
			AtRestEncryptionEnabled: aws.Bool(false),
			TransitEncryptionEnabled: aws.Bool(false),
			AuthTokenEnabled:     aws.Bool(false),
			SnapshotRetentionLimit: aws.Int32(0),
		},

		// Broken: Status=create-failed — provisioning failed.
		// Expected Fields["status"] == "create failed — see events".
		{
			ReplicationGroupId:   aws.String("bad-config-redis"),
			Description:          aws.String("Bad config Redis (create-failed)"),
			ARN:                  aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:bad-config-redis"),
			Status:               aws.String("create-failed"),
			Engine:               aws.String("redis"),
			MultiAZ:              elasticachetypes.MultiAZStatusDisabled,
			AutomaticFailover:    elasticachetypes.AutomaticFailoverStatusDisabled,
			CacheNodeType:        aws.String("cache.t3.medium"),
			MemberClusters:       []string{},
			AtRestEncryptionEnabled: aws.Bool(false),
			TransitEncryptionEnabled: aws.Bool(false),
			AuthTokenEnabled:     aws.Bool(false),
			SnapshotRetentionLimit: aws.Int32(0),
		},

		// Warning: Status=available, MultiAZ=enabled, AutomaticFailover=disabled — single signal.
		// Expected Fields["status"] == "multi-AZ without auto-failover".
		{
			ReplicationGroupId:   aws.String("legacy-redis-analytics"),
			Description:          aws.String("Legacy analytics Redis (multi-AZ no failover)"),
			ARN:                  aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:legacy-redis-analytics"),
			Status:               aws.String("available"),
			Engine:               aws.String("redis"),
			MultiAZ:              elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:    elasticachetypes.AutomaticFailoverStatusDisabled,
			CacheNodeType:        aws.String("cache.r6g.large"),
			MemberClusters:       []string{"legacy-redis-analytics-001", "legacy-redis-analytics-002"},
			AtRestEncryptionEnabled: aws.Bool(true),
			TransitEncryptionEnabled: aws.Bool(true),
			AuthTokenEnabled:     aws.Bool(false),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("legacy-redis-analytics.cfg.use1.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			SnapshotRetentionLimit: aws.Int32(1),
		},

		// Multi-W1 fixture (U7a): Status=modifying + multi-AZ without auto-failover.
		// Two coexisting §3.1 Warnings in the same row.
		// Expected Fields["status"] == "modifying — config change (+1)".
		// Expected Resource.Issues == ["modifying — config change", "multi-AZ without auto-failover"].
		{
			ReplicationGroupId:   aws.String(WarnRedisMultiID),
			Description:          aws.String("Legacy billing Redis (modifying + no failover)"),
			ARN:                  aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:legacy-redis-billing"),
			Status:               aws.String("modifying"),
			Engine:               aws.String("redis"),
			MultiAZ:              elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:    elasticachetypes.AutomaticFailoverStatusDisabled,
			CacheNodeType:        aws.String("cache.r6g.large"),
			MemberClusters:       []string{"legacy-redis-billing-001", "legacy-redis-billing-002"},
			AtRestEncryptionEnabled: aws.Bool(true),
			TransitEncryptionEnabled: aws.Bool(true),
			AuthTokenEnabled:     aws.Bool(false),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("legacy-redis-billing.cfg.use1.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			SnapshotRetentionLimit: aws.Int32(1),
		},

		// Valkey fixture — Engine="valkey", exists solely to prove the engine filter
		// (P2-1) excludes non-Redis engines. Must NOT appear in the redis resource list.
		{
			ReplicationGroupId: aws.String(ValkeyEngineID),
			Description:        aws.String("Prod Valkey (should be filtered out by redis fetcher)"),
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-valkey"),
			Status:             aws.String("available"),
			Engine:             aws.String("valkey"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			CacheNodeType:      aws.String("cache.r7g.large"),
			MemberClusters:     []string{"prod-valkey-001", "prod-valkey-002"},
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("prod-valkey.cfg.use1.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			SnapshotRetentionLimit: aws.Int32(1),
		},

		// Multi-shard healthy — ClusterEnabled=true, 3 NodeGroups all available.
		// Graph-connectivity fixture: carries the same tag set as prod-redis-sessions
		// so related-pivot scenario tests can exercise multi-shard groups.
		// Expected Fields["status"] == "" (Healthy silence).
		{
			ReplicationGroupId: aws.String(MultiShardHealthyID),
			Description:        aws.String("Multi-shard cluster (all shards healthy)"),
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:multi-shard-healthy"),
			Status:             aws.String("available"),
			Engine:             aws.String("redis"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			CacheNodeType:      aws.String("cache.r6g.large"),
			KmsKeyId:           aws.String(ProdRedisKMSKeyARN),
			MemberClusters: []string{
				"multi-shard-healthy-0001-001", "multi-shard-healthy-0001-002",
				"multi-shard-healthy-0002-001", "multi-shard-healthy-0002-002",
				"multi-shard-healthy-0003-001", "multi-shard-healthy-0003-002",
			},
			NodeGroups: []elasticachetypes.NodeGroup{
				{
					NodeGroupId: aws.String("0001"),
					Status:      aws.String("available"),
					NodeGroupMembers: []elasticachetypes.NodeGroupMember{
						{
							CacheClusterId:            aws.String("multi-shard-healthy-0001-001"),
							CurrentRole:               aws.String("primary"),
							PreferredAvailabilityZone: aws.String("us-east-1a"),
						},
						{
							CacheClusterId:            aws.String("multi-shard-healthy-0001-002"),
							CurrentRole:               aws.String("replica"),
							PreferredAvailabilityZone: aws.String("us-east-1b"),
						},
					},
				},
				{
					NodeGroupId: aws.String("0002"),
					Status:      aws.String("available"),
					NodeGroupMembers: []elasticachetypes.NodeGroupMember{
						{
							CacheClusterId:            aws.String("multi-shard-healthy-0002-001"),
							CurrentRole:               aws.String("primary"),
							PreferredAvailabilityZone: aws.String("us-east-1b"),
						},
						{
							CacheClusterId:            aws.String("multi-shard-healthy-0002-002"),
							CurrentRole:               aws.String("replica"),
							PreferredAvailabilityZone: aws.String("us-east-1c"),
						},
					},
				},
				{
					NodeGroupId: aws.String("0003"),
					Status:      aws.String("available"),
					NodeGroupMembers: []elasticachetypes.NodeGroupMember{
						{
							CacheClusterId:            aws.String("multi-shard-healthy-0003-001"),
							CurrentRole:               aws.String("primary"),
							PreferredAvailabilityZone: aws.String("us-east-1c"),
						},
						{
							CacheClusterId:            aws.String("multi-shard-healthy-0003-002"),
							CurrentRole:               aws.String("replica"),
							PreferredAvailabilityZone: aws.String("us-east-1a"),
						},
					},
				},
			},
			LogDeliveryConfigurations: []elasticachetypes.LogDeliveryConfiguration{
				{
					LogType:         elasticachetypes.LogTypeSlowLog,
					LogFormat:       elasticachetypes.LogFormatText,
					DestinationType: elasticachetypes.DestinationTypeCloudWatchLogs,
					DestinationDetails: &elasticachetypes.DestinationDetails{
						CloudWatchLogsDetails: &elasticachetypes.CloudWatchLogsDestinationDetails{
							LogGroup: aws.String(ProdRedisLogGroup),
						},
					},
					Status: elasticachetypes.LogDeliveryConfigurationStatusActive,
				},
			},
			AtRestEncryptionEnabled:  aws.Bool(true),
			TransitEncryptionEnabled: aws.Bool(true),
			AuthTokenEnabled:         aws.Bool(true),
			SnapshotRetentionLimit:   aws.Int32(1),
		},

		// Multi-shard one-modifying — shard 0001 modifying, 0002 + 0003 available.
		// Expected Fields["status"] == "shard 0001: modifying".
		// Expected Resource.Issues == ["shard 0001: modifying"].
		{
			ReplicationGroupId: aws.String(MultiShardOneModifyingID),
			Description:        aws.String("Multi-shard cluster (shard 0001 modifying)"),
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:multi-shard-modifying-0001"),
			Status:             aws.String("modifying"),
			Engine:             aws.String("redis"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			CacheNodeType:      aws.String("cache.r6g.large"),
			MemberClusters: []string{
				"multi-mod-0001-001", "multi-mod-0001-002",
				"multi-mod-0002-001", "multi-mod-0002-002",
				"multi-mod-0003-001", "multi-mod-0003-002",
			},
			NodeGroups: []elasticachetypes.NodeGroup{
				{
					NodeGroupId: aws.String("0001"),
					Status:      aws.String("modifying"),
					NodeGroupMembers: []elasticachetypes.NodeGroupMember{
						{
							CacheClusterId:            aws.String("multi-mod-0001-001"),
							CurrentRole:               aws.String("primary"),
							PreferredAvailabilityZone: aws.String("us-east-1a"),
						},
						{
							CacheClusterId:            aws.String("multi-mod-0001-002"),
							CurrentRole:               aws.String("replica"),
							PreferredAvailabilityZone: aws.String("us-east-1b"),
						},
					},
				},
				{
					NodeGroupId: aws.String("0002"),
					Status:      aws.String("available"),
					NodeGroupMembers: []elasticachetypes.NodeGroupMember{
						{
							CacheClusterId:            aws.String("multi-mod-0002-001"),
							CurrentRole:               aws.String("primary"),
							PreferredAvailabilityZone: aws.String("us-east-1b"),
						},
						{
							CacheClusterId:            aws.String("multi-mod-0002-002"),
							CurrentRole:               aws.String("replica"),
							PreferredAvailabilityZone: aws.String("us-east-1c"),
						},
					},
				},
				{
					NodeGroupId: aws.String("0003"),
					Status:      aws.String("available"),
					NodeGroupMembers: []elasticachetypes.NodeGroupMember{
						{
							CacheClusterId:            aws.String("multi-mod-0003-001"),
							CurrentRole:               aws.String("primary"),
							PreferredAvailabilityZone: aws.String("us-east-1c"),
						},
						{
							CacheClusterId:            aws.String("multi-mod-0003-002"),
							CurrentRole:               aws.String("replica"),
							PreferredAvailabilityZone: aws.String("us-east-1a"),
						},
					},
				},
			},
			AtRestEncryptionEnabled:  aws.Bool(true),
			TransitEncryptionEnabled: aws.Bool(true),
			AuthTokenEnabled:         aws.Bool(false),
			SnapshotRetentionLimit:   aws.Int32(1),
		},

		// Multi-shard two-transitioning — shard 0001 modifying + 0002 snapshotting + 0003 available.
		// Expected Fields["status"] == "shard 0001: modifying (+1)".
		// Expected Resource.Issues == ["shard 0001: modifying", "shard 0002: snapshotting"] (alphabetical).
		{
			ReplicationGroupId: aws.String(MultiShardTwoTransitioningID),
			Description:        aws.String("Multi-shard cluster (two shards transitioning)"),
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:replicationgroup:multi-shard-two-transitioning"),
			Status:             aws.String("modifying"),
			Engine:             aws.String("redis"),
			MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
			AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
			CacheNodeType:      aws.String("cache.r6g.large"),
			MemberClusters: []string{
				"multi-2t-0001-001", "multi-2t-0001-002",
				"multi-2t-0002-001", "multi-2t-0002-002",
				"multi-2t-0003-001", "multi-2t-0003-002",
			},
			NodeGroups: []elasticachetypes.NodeGroup{
				{
					NodeGroupId: aws.String("0001"),
					Status:      aws.String("modifying"),
					NodeGroupMembers: []elasticachetypes.NodeGroupMember{
						{
							CacheClusterId:            aws.String("multi-2t-0001-001"),
							CurrentRole:               aws.String("primary"),
							PreferredAvailabilityZone: aws.String("us-east-1a"),
						},
						{
							CacheClusterId:            aws.String("multi-2t-0001-002"),
							CurrentRole:               aws.String("replica"),
							PreferredAvailabilityZone: aws.String("us-east-1b"),
						},
					},
				},
				{
					NodeGroupId: aws.String("0002"),
					Status:      aws.String("snapshotting"),
					NodeGroupMembers: []elasticachetypes.NodeGroupMember{
						{
							CacheClusterId:            aws.String("multi-2t-0002-001"),
							CurrentRole:               aws.String("primary"),
							PreferredAvailabilityZone: aws.String("us-east-1b"),
						},
						{
							CacheClusterId:            aws.String("multi-2t-0002-002"),
							CurrentRole:               aws.String("replica"),
							PreferredAvailabilityZone: aws.String("us-east-1c"),
						},
					},
				},
				{
					NodeGroupId: aws.String("0003"),
					Status:      aws.String("available"),
					NodeGroupMembers: []elasticachetypes.NodeGroupMember{
						{
							CacheClusterId:            aws.String("multi-2t-0003-001"),
							CurrentRole:               aws.String("primary"),
							PreferredAvailabilityZone: aws.String("us-east-1c"),
						},
						{
							CacheClusterId:            aws.String("multi-2t-0003-002"),
							CurrentRole:               aws.String("replica"),
							PreferredAvailabilityZone: aws.String("us-east-1a"),
						},
					},
				},
			},
			AtRestEncryptionEnabled:  aws.Bool(true),
			TransitEncryptionEnabled: aws.Bool(true),
			AuthTokenEnabled:         aws.Bool(false),
			SnapshotRetentionLimit:   aws.Int32(1),
		},
	}
}

// ---------------------------------------------------------------------------
// CacheClusters
// ---------------------------------------------------------------------------

// buildRedisCacheClusters builds the member-cluster list.
// The graph-root member cluster (ProdRedisMemberClusterID) carries all fields
// needed by the current related checkers: SG, SNS, SubnetGroup, LogDeliveryConfigurations.
// These fields exist on CacheCluster, not ReplicationGroup, and are read by the
// pre-phase-7 related checkers via DescribeCacheClusters on MemberClusters[0].
func buildRedisCacheClusters() []elasticachetypes.CacheCluster {
	return []elasticachetypes.CacheCluster{
		// Graph-root primary member cluster — carries all related-panel pivot fields.
		{
			CacheClusterId:       aws.String(ProdRedisMemberClusterID),
			ReplicationGroupId:   aws.String(ProdRedisID),
			ARN:                  aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:" + ProdRedisMemberClusterID),
			CacheClusterStatus:   aws.String("available"),
			CacheNodeType:        aws.String("cache.r6g.large"),
			Engine:               aws.String("redis"),
			EngineVersion:        aws.String("7.1"),
			NumCacheNodes:        aws.Int32(1),
			CacheSubnetGroupName: aws.String(ProdRedisSubnetGroup),
			PreferredAvailabilityZone: aws.String("us-east-1a"),
			AtRestEncryptionEnabled:   aws.Bool(true),
			TransitEncryptionEnabled:  aws.Bool(true),
			AuthTokenEnabled:          aws.Bool(true),
			SnapshotRetentionLimit:    aws.Int32(1),
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{
				{SecurityGroupId: aws.String(ProdRedisSGID), Status: aws.String("active")},
			},
			NotificationConfiguration: &elasticachetypes.NotificationConfiguration{
				TopicArn:    aws.String(ProdRedisSNSTopicARN),
				TopicStatus: aws.String("active"),
			},
			LogDeliveryConfigurations: []elasticachetypes.LogDeliveryConfiguration{
				{
					LogType:         elasticachetypes.LogTypeSlowLog,
					LogFormat:       elasticachetypes.LogFormatText,
					DestinationType: elasticachetypes.DestinationTypeCloudWatchLogs,
					DestinationDetails: &elasticachetypes.DestinationDetails{
						CloudWatchLogsDetails: &elasticachetypes.CloudWatchLogsDestinationDetails{
							LogGroup: aws.String(ProdRedisLogGroup),
						},
					},
					Status: elasticachetypes.LogDeliveryConfigurationStatusActive,
				},
			},
		},
		// Additional member clusters (no related pivot data needed — not queried).
		{
			CacheClusterId:     aws.String("prod-redis-sessions-002"),
			ReplicationGroupId: aws.String(ProdRedisID),
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:prod-redis-sessions-002"),
			CacheClusterStatus: aws.String("available"),
			CacheNodeType:      aws.String("cache.r6g.large"),
			Engine:             aws.String("redis"),
			EngineVersion:      aws.String("7.1"),
			NumCacheNodes:      aws.Int32(1),
			PreferredAvailabilityZone: aws.String("us-east-1b"),
		},
		{
			CacheClusterId:     aws.String("prod-redis-sessions-003"),
			ReplicationGroupId: aws.String(ProdRedisID),
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:prod-redis-sessions-003"),
			CacheClusterStatus: aws.String("available"),
			CacheNodeType:      aws.String("cache.r6g.large"),
			Engine:             aws.String("redis"),
			EngineVersion:      aws.String("7.1"),
			NumCacheNodes:      aws.Int32(1),
			PreferredAvailabilityZone: aws.String("us-east-1c"),
		},
		// Staging member cluster.
		{
			CacheClusterId:     aws.String("staging-redis-001"),
			ReplicationGroupId: aws.String("staging-redis"),
			ARN:                aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:staging-redis-001"),
			CacheClusterStatus: aws.String("available"),
			CacheNodeType:      aws.String("cache.t3.medium"),
			Engine:             aws.String("redis"),
			EngineVersion:      aws.String("7.0"),
			NumCacheNodes:      aws.Int32(1),
			PreferredAvailabilityZone: aws.String("us-east-1a"),
		},
	}
}

// ---------------------------------------------------------------------------
// CacheSubnetGroups
// ---------------------------------------------------------------------------

func buildRedisCacheSubnetGroups() []elasticachetypes.CacheSubnetGroup {
	return []elasticachetypes.CacheSubnetGroup{
		{
			CacheSubnetGroupName:        aws.String(ProdRedisSubnetGroup),
			CacheSubnetGroupDescription: aws.String("Prod Redis subnet group (multi-AZ)"),
			VpcId:                       aws.String(ProdRedisVpcID),
			ARN:                         aws.String("arn:aws:elasticache:us-east-1:123456789012:subnetgroup:" + ProdRedisSubnetGroup),
			Subnets: []elasticachetypes.Subnet{
				{
					SubnetIdentifier: aws.String(ProdRedisSubnetA),
					SubnetAvailabilityZone: &elasticachetypes.AvailabilityZone{
						Name: aws.String("us-east-1a"),
					},
				},
				{
					SubnetIdentifier: aws.String(ProdRedisSubnetB),
					SubnetAvailabilityZone: &elasticachetypes.AvailabilityZone{
						Name: aws.String("us-east-1b"),
					},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// TagLists
// ---------------------------------------------------------------------------

// buildRedisTagLists returns per-ARN tag lists returned by ListTagsForResource.
// The graph-root ARN carries the aws:cloudformation:stack-name tag so the cfn
// related checker resolves the matching stack in cfn.go.
func buildRedisTagLists() map[string][]elasticachetypes.Tag {
	cfnTag := []elasticachetypes.Tag{
		{
			Key:   aws.String("aws:cloudformation:stack-name"),
			Value: aws.String(ProdRedisCFNStack),
		},
		{
			Key:   aws.String("Environment"),
			Value: aws.String("production"),
		},
		{
			Key:   aws.String("Team"),
			Value: aws.String("platform"),
		},
	}
	return map[string][]elasticachetypes.Tag{
		// RG ARN — kept for reference / future use.
		ProdRedisARN: cfnTag,
		// Member cluster ARN — checkRedisCFN calls ListTagsForResource on cluster.ARN
		// (the CacheCluster ARN), so this entry is what the demo actually resolves.
		ProdRedisMemberClusterARN: cfnTag,
	}
}
