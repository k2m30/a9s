# Backup Plans (backup) — Related Resources

## Real-World Use Cases

**1. "What resources does this backup plan protect?"** The plan defines WHEN and WHERE to back up, but the selection criteria (which resources) are a separate concept. You need to see the backup selections to know which EC2, RDS, EBS, EFS, and DynamoDB resources are protected.

**2. "Are backups actually succeeding?"** The plan exists, but are recovery points being created on schedule? Check the backup vault for recent recovery points from this plan.

**3. "Can I restore from this plan's backups?"** Navigate to the backup vault to see recovery points and their status.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal) | Backup plans reference resources for protection, not the other way. Protected resources don't know they're in a backup plan. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Protected Resources (various) | `backup:ListBackupSelections` with `BackupPlanId` → `backup:GetBackupSelection` for each. Selections contain resource ARNs or tag-based selection criteria (e.g., "back up all resources tagged `backup=true`"). Parse ARNs for a9s resource types. | "What does this plan back up?" Navigate to the specific EC2 instances, RDS databases, EBS volumes, etc. that are protected. | P0 |
| Backup Vault (not in a9s) | Plan's backup rules reference `TargetBackupVaultName`. `backup:DescribeBackupVault` shows the vault's encryption key, access policy, and lock configuration. | "Where are backups stored?" The vault is the container for all recovery points. | P1 |
| Recovery Points (not in a9s) | `backup:ListRecoveryPointsByBackupVault` with the target vault, filtered by `BackupPlanId`. Shows actual backup snapshots with creation date, status, and size. | "Are backups actually being created? When was the last one?" | P1 |
| IAM Role (role) | Plan has `IamRoleArn` via backup selections — FORWARD. The role needs permissions to access the resources being backed up. | "Why are backups failing?" Insufficient permissions on the backup role. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteBackupPlan | "Who deleted this backup plan?" Resources are no longer being backed up on this schedule. |
| UpdateBackupPlan | "Who changed the backup schedule or retention?" Reducing retention or frequency weakens DR posture. |
| StartBackupJob | "Who triggered a manual backup?" Outside the normal schedule — usually pre-maintenance or pre-migration safety. |
