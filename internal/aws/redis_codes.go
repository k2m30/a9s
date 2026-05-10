package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeRedisCreateFailed               domain.FindingCode = "redis.broken.create_failed"
	CodeRedisCreating                   domain.FindingCode = "redis.warn.creating"
	CodeRedisDeleting                   domain.FindingCode = "redis.warn.deleting"
	CodeRedisModifying                  domain.FindingCode = "redis.warn.modifying"
	CodeRedisSnapshotting               domain.FindingCode = "redis.warn.snapshotting"
	CodeRedisShardIssue                 domain.FindingCode = "redis.warn.shard_issue"
	CodeRedisMultiAZWithoutAutoFailover domain.FindingCode = "redis.warn.multiaz_without_auto_failover"
)
