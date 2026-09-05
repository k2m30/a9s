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

// S5 operator sentences for the redis posture findings.
const (
	redisAtRestOffDetail  = "Cached data is written to disk and to backups unencrypted. Encryption at rest can only be turned on at creation time — recreate the replication group with it enabled and migrate."
	redisTransitOffDetail = "Client traffic to this group crosses the network in cleartext, so anyone with VPC access can read the cached data. Enable in-transit encryption on the replication group."
	redisNoAuthDetail     = "The group accepts any client that can reach it — encryption in transit is on but no authentication token is required. Set one, so a network-level reachability mistake is not immediately a data breach."
	redisNoBackupDetail   = "Automatic backups are off, so a failed replication group takes its data with it. Set a snapshot retention limit of at least one day."
)
