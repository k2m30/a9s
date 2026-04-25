# DB Cluster Snapshots (dbc-snap) — Related Resources

## Real-World Use Cases

**1. "When was this cluster last backed up?"** Navigate from the snapshot to its source cluster to verify backup recency. Check if automated backups are sufficient or if manual snapshots are needed before schema changes.

**2. "Was this snapshot shared?"** Security audit — same concern as RDS snapshots.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal) | Snapshots are leaf resources. No other resources reference them by ARN. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| DocumentDB Cluster (dbc) | Snapshot has `DBClusterIdentifier` — FORWARD. Navigate to the source cluster to verify it exists. | "Is the source cluster still running?" | P0 |
| KMS Key (kms) | Snapshot has `KmsKeyId` — FORWARD (if encrypted). Must be accessible for restore. | "Can this snapshot be restored?" | P1 |
| Shared Accounts (not a resource) | `docdb:DescribeDBClusterSnapshotAttributes` — same pattern as RDS. | "Who can access this snapshot?" | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteDBClusterSnapshot | "Who deleted this snapshot?" |
| ModifyDBClusterSnapshotAttribute | "Who shared this snapshot?" |
| CopyDBClusterSnapshot | "Who copied this snapshot cross-region?" |
