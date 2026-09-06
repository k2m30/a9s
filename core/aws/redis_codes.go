// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeRedisCreateFailed               domain.FindingCode = "redis.broken.create_failed"
	CodeRedisCreating                   domain.FindingCode = "redis.warn.creating"
	CodeRedisDeleting                   domain.FindingCode = "redis.warn.deleting"
	CodeRedisModifying                  domain.FindingCode = "redis.warn.modifying"
	CodeRedisSnapshotting               domain.FindingCode = "redis.warn.snapshotting"
	CodeRedisShardIssue                 domain.FindingCode = "redis.warn.shard_issue"
	CodeRedisMultiAZWithoutAutoFailover domain.FindingCode = "redis.warn.multiaz_without_auto_failover"
)

// Security-posture findings (Prowler gap closure), all evaluated in
// computeRedisFindings from the ReplicationGroup the fetcher already holds.
const (
	CodeRedisAtRestOff  domain.FindingCode = "redis.encryption-at-rest-off"
	CodeRedisTransitOff domain.FindingCode = "redis.encryption-in-transit-off"
	CodeRedisNoAuth     domain.FindingCode = "redis.no-auth"
	CodeRedisNoBackup   domain.FindingCode = "redis.no-backup"
)
