// Package fixtures provides ElastiCache fixture data for the ElastiCache fake.
package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
)

// ElastiCacheFixtures holds all ElastiCache domain objects served by the fake.
type ElastiCacheFixtures struct {
	// CacheClusters is the full list returned by DescribeCacheClusters.
	CacheClusters []elasticachetypes.CacheCluster
}

// NewElastiCacheFixtures builds and returns a fully-populated ElastiCacheFixtures struct.
func NewElastiCacheFixtures() *ElastiCacheFixtures {
	return &ElastiCacheFixtures{
		CacheClusters: buildElastiCacheClusters(),
	}
}

const elastiCacheSGID = "sg-0ccc333333333333c"

func buildElastiCacheClusters() []elasticachetypes.CacheCluster {
	return []elasticachetypes.CacheCluster{
		{
			ARN:                       aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:acme-prod-sessions"),
			CacheClusterId:            aws.String("acme-prod-sessions"),
			CacheClusterStatus:        aws.String("available"),
			CacheNodeType:             aws.String("cache.r6g.large"),
			Engine:                    aws.String("redis"),
			EngineVersion:             aws.String("7.1"),
			NumCacheNodes:             aws.Int32(3),
			ReplicationGroupId:        aws.String("acme-prod-sessions-rg"),
			PreferredAvailabilityZone: aws.String("us-east-1a"),
			CacheSubnetGroupName:      aws.String("acme-elasticache-subnet-group"),
			AtRestEncryptionEnabled:   aws.Bool(true),
			TransitEncryptionEnabled:  aws.Bool(true),
			AuthTokenEnabled:          aws.Bool(true),
			SnapshotRetentionLimit:    aws.Int32(1),
			PreferredMaintenanceWindow: aws.String("sun:05:00-sun:06:00"),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("acme-prod-sessions.cfg.usw2.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{
				{SecurityGroupId: aws.String(elastiCacheSGID), Status: aws.String("active")},
			},
			CacheNodes: []elasticachetypes.CacheNode{
				{CacheNodeId: aws.String("0001"), CacheNodeStatus: aws.String("available")},
				{CacheNodeId: aws.String("0002"), CacheNodeStatus: aws.String("available")},
				{CacheNodeId: aws.String("0003"), CacheNodeStatus: aws.String("available")},
			},
			CacheClusterCreateTime: aws.Time(mustTime("2025-06-10T14:30:00Z")),
		},
		{
			ARN:                       aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:acme-prod-cache"),
			CacheClusterId:            aws.String("acme-prod-cache"),
			CacheClusterStatus:        aws.String("available"),
			CacheNodeType:             aws.String("cache.m6g.xlarge"),
			Engine:                    aws.String("redis"),
			EngineVersion:             aws.String("7.1"),
			NumCacheNodes:             aws.Int32(2),
			ReplicationGroupId:        aws.String("acme-prod-cache-rg"),
			PreferredAvailabilityZone: aws.String("us-east-1b"),
			CacheSubnetGroupName:      aws.String("acme-elasticache-subnet-group"),
			AtRestEncryptionEnabled:   aws.Bool(true),
			TransitEncryptionEnabled:  aws.Bool(true),
			AuthTokenEnabled:          aws.Bool(false),
			SnapshotRetentionLimit:    aws.Int32(1),
			PreferredMaintenanceWindow: aws.String("mon:05:00-mon:06:00"),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("acme-prod-cache.cfg.usw2.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{
				{SecurityGroupId: aws.String(elastiCacheSGID), Status: aws.String("active")},
			},
			CacheNodes: []elasticachetypes.CacheNode{
				{CacheNodeId: aws.String("0001"), CacheNodeStatus: aws.String("available")},
				{CacheNodeId: aws.String("0002"), CacheNodeStatus: aws.String("available")},
			},
			CacheClusterCreateTime: aws.Time(mustTime("2025-03-22T09:15:00Z")),
		},
		{
			ARN:                       aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:staging-redis"),
			CacheClusterId:            aws.String("staging-redis"),
			CacheClusterStatus:        aws.String("available"),
			CacheNodeType:             aws.String("cache.t3.medium"),
			Engine:                    aws.String("redis"),
			EngineVersion:             aws.String("7.0"),
			NumCacheNodes:             aws.Int32(1),
			ReplicationGroupId:        aws.String("staging-redis-rg"),
			PreferredAvailabilityZone: aws.String("us-east-1a"),
			CacheSubnetGroupName:      aws.String("acme-staging-elasticache-subnet-group"),
			AtRestEncryptionEnabled:   aws.Bool(false),
			TransitEncryptionEnabled:  aws.Bool(false),
			AuthTokenEnabled:          aws.Bool(false),
			SnapshotRetentionLimit:    aws.Int32(0),
			PreferredMaintenanceWindow: aws.String("tue:06:00-tue:07:00"),
			ConfigurationEndpoint: &elasticachetypes.Endpoint{
				Address: aws.String("staging-redis.cfg.usw2.cache.amazonaws.com"),
				Port:    aws.Int32(6379),
			},
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{
				{SecurityGroupId: aws.String(elastiCacheSGID), Status: aws.String("active")},
			},
			CacheNodes: []elasticachetypes.CacheNode{
				{CacheNodeId: aws.String("0001"), CacheNodeStatus: aws.String("available")},
			},
			CacheClusterCreateTime: aws.Time(mustTime("2025-09-01T11:00:00Z")),
		},
		{
			ARN:                       aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:dev-feature-redis"),
			CacheClusterId:            aws.String("dev-feature-redis"),
			CacheClusterStatus:        aws.String("creating"),
			CacheNodeType:             aws.String("cache.t3.small"),
			Engine:                    aws.String("redis"),
			EngineVersion:             aws.String("7.1"),
			NumCacheNodes:             aws.Int32(1),
			ReplicationGroupId:        aws.String("dev-feature-redis-rg"),
			PreferredAvailabilityZone: aws.String("us-east-1a"),
			CacheSubnetGroupName:      aws.String("acme-elasticache-subnet-group"),
			AtRestEncryptionEnabled:   aws.Bool(true),
			TransitEncryptionEnabled:  aws.Bool(true),
			AuthTokenEnabled:          aws.Bool(false),
			SnapshotRetentionLimit:    aws.Int32(0),
			PreferredMaintenanceWindow: aws.String("wed:06:00-wed:07:00"),
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{
				{SecurityGroupId: aws.String(elastiCacheSGID), Status: aws.String("active")},
			},
			CacheNodes:             []elasticachetypes.CacheNode{},
			CacheClusterCreateTime: aws.Time(mustTime("2026-03-21T08:00:00Z")),
		},
		// Issue: Status=modifying → Warning (configuration change in progress)
		{
			ARN:                       aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:redis-modifying"),
			CacheClusterId:            aws.String("redis-modifying"),
			CacheClusterStatus:        aws.String("modifying"),
			CacheNodeType:             aws.String("cache.r6g.large"),
			Engine:                    aws.String("redis"),
			EngineVersion:             aws.String("7.1"),
			NumCacheNodes:             aws.Int32(2),
			ReplicationGroupId:        aws.String("redis-modifying-rg"),
			PreferredAvailabilityZone: aws.String("us-east-1b"),
			CacheSubnetGroupName:      aws.String("acme-elasticache-subnet-group"),
			AtRestEncryptionEnabled:   aws.Bool(true),
			TransitEncryptionEnabled:  aws.Bool(true),
			AuthTokenEnabled:          aws.Bool(false),
			SnapshotRetentionLimit:    aws.Int32(1),
			PreferredMaintenanceWindow: aws.String("thu:05:00-thu:06:00"),
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{
				{SecurityGroupId: aws.String(elastiCacheSGID), Status: aws.String("active")},
			},
			CacheNodes: []elasticachetypes.CacheNode{
				{CacheNodeId: aws.String("0001"), CacheNodeStatus: aws.String("available")},
				{CacheNodeId: aws.String("0002"), CacheNodeStatus: aws.String("modifying")},
			},
			CacheClusterCreateTime: aws.Time(mustTime("2025-07-15T09:00:00Z")),
		},
		// Issue: Status=create-failed → Broken (cluster provisioning failed)
		{
			ARN:                       aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:redis-create-failed"),
			CacheClusterId:            aws.String("redis-create-failed"),
			CacheClusterStatus:        aws.String("create-failed"),
			CacheNodeType:             aws.String("cache.t3.medium"),
			Engine:                    aws.String("redis"),
			EngineVersion:             aws.String("7.1"),
			NumCacheNodes:             aws.Int32(1),
			PreferredAvailabilityZone: aws.String("us-east-1a"),
			CacheSubnetGroupName:      aws.String("acme-elasticache-subnet-group"),
			AtRestEncryptionEnabled:   aws.Bool(true),
			TransitEncryptionEnabled:  aws.Bool(true),
			AuthTokenEnabled:          aws.Bool(false),
			SnapshotRetentionLimit:    aws.Int32(0),
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{
				{SecurityGroupId: aws.String(elastiCacheSGID), Status: aws.String("active")},
			},
			CacheNodes:             []elasticachetypes.CacheNode{},
			CacheClusterCreateTime: aws.Time(mustTime("2026-04-15T14:00:00Z")),
		},
		// Issue: SnapshotRetentionLimit=0 → Warning (no backups enabled)
		{
			ARN:                        aws.String("arn:aws:elasticache:us-east-1:123456789012:cluster:redis-no-backups"),
			CacheClusterId:             aws.String("redis-no-backups"),
			CacheClusterStatus:         aws.String("available"),
			CacheNodeType:              aws.String("cache.r6g.large"),
			Engine:                     aws.String("redis"),
			EngineVersion:              aws.String("7.1"),
			NumCacheNodes:              aws.Int32(1),
			ReplicationGroupId:         aws.String("redis-no-backups-rg"),
			PreferredAvailabilityZone:  aws.String("us-east-1a"),
			CacheSubnetGroupName:       aws.String("acme-elasticache-subnet-group"),
			AtRestEncryptionEnabled:    aws.Bool(true),
			TransitEncryptionEnabled:   aws.Bool(false),
			AuthTokenEnabled:           aws.Bool(false),
			SnapshotRetentionLimit:     aws.Int32(0),
			PreferredMaintenanceWindow: aws.String("sat:06:00-sat:07:00"),
			SecurityGroups: []elasticachetypes.SecurityGroupMembership{
				{SecurityGroupId: aws.String(elastiCacheSGID), Status: aws.String("active")},
			},
			CacheNodes: []elasticachetypes.CacheNode{
				{CacheNodeId: aws.String("0001"), CacheNodeStatus: aws.String("available")},
			},
			CacheClusterCreateTime: aws.Time(mustTime("2025-05-01T10:00:00Z")),
		},
	}
}
