# RDS Snapshots (rds-snap) — Related Resources

## Real-World Use Cases

**1. "Can I restore from this snapshot?"** You need to verify the snapshot is complete and accessible. If encrypted, check that the KMS key is available. If shared from another account, check the sharing permissions.

**2. "Was this snapshot shared outside the organization?"** During a security audit, check snapshot attributes for cross-account sharing — shared snapshots can leak database contents.

**3. "Which DB instance was this snapshot taken from?"** The snapshot has `DBInstanceIdentifier`, but does that instance still exist? Navigate to the source to verify.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal) | Snapshots are leaf resources — very few things reference them. A restored DB instance doesn't maintain a link to its source snapshot. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| DB Instance (dbi) | Snapshot has `DBInstanceIdentifier` — FORWARD. Navigate to the source DB to verify it still exists and compare current config with snapshot time. | "Is the source DB still running?" If the DB was deleted and this is the only snapshot, it's critical data. | P0 |
| KMS Key (kms) | Snapshot has `KmsKeyId` — FORWARD (if encrypted). The KMS key must be accessible to restore the snapshot. If the key is disabled or deleted, the snapshot is unrecoverable. | "Can this snapshot be restored?" KMS key availability determines restorability. | P1 |
| Shared Accounts (not a resource) | `rds:DescribeDBSnapshotAttributes` returns `DBSnapshotAttributesResult.DBSnapshotAttributes` where attribute name `restore` lists account IDs the snapshot is shared with. `all` means public. | "Who can access this snapshot?" Security audit for data exposure. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteDBSnapshot | "Who deleted this snapshot?" Potential loss of the only backup. |
| ModifyDBSnapshotAttribute | "Who shared this snapshot with another account?" Data exfiltration risk if shared with an untrusted account. |
| CopyDBSnapshot | "Who copied this snapshot to another region or account?" Cross-region copies are for DR; cross-account copies need scrutiny. |
