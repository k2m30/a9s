# KMS Keys (kms) — Related Resources

## Real-World Use Cases

**1. "What data does this key protect?"** KMS keys are referenced by virtually every AWS service that supports encryption at rest — S3, RDS, EBS, EFS, SQS, SNS, DynamoDB, Secrets Manager, SSM, OpenSearch, Redshift, Kinesis, MSK, CloudWatch Logs, ECR. Before disabling or scheduling deletion of a key, you MUST know the blast radius.

**2. "Who has access to this key?"** The KMS key policy is the primary access control mechanism — it determines who can encrypt, decrypt, and manage the key. IAM policies alone are not sufficient; the key policy must also allow access.

**3. "Someone scheduled this key for deletion — what breaks?"** A 7-30 day countdown begins. Every encrypted resource using this key becomes permanently inaccessible after deletion. Find all dependent resources immediately.

## Reverse Relationships

KMS keys are the most transitively referenced resource in AWS — almost every encrypted resource points to one.

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| EBS Volumes (ebs) | `ec2:DescribeVolumes` with `Filters=[{Name=encrypted, Values=[true]}]`, then filter by `KmsKeyId` matching this key. Or if a9s has EBS data cached, search in-memory. | "Which volumes does this key encrypt?" Volume data becomes inaccessible if the key is deleted. | P0 |
| RDS Instances (dbi) | Search RDS instances where `KmsKeyId` matches. If a9s has RDS data cached, search in-memory. | "Which databases does this key encrypt?" | P0 |
| S3 Buckets (s3) | Search bucket encryption configurations for this key. Requires `s3:GetBucketEncryption` per bucket. Expensive. | "Which buckets use this key for SSE-KMS?" | P0 |
| Secrets Manager (secrets) | Search secrets where `KmsKeyId` matches. If a9s has secrets data cached, search in-memory. | "Which secrets does this key encrypt?" | P1 |
| EBS Snapshots (ebs-snap) | `ec2:DescribeSnapshots` with `Filters=[{Name=encrypted, Values=[true]}]`, filter by `KmsKeyId`. | "Which snapshots depend on this key?" | P1 |
| DynamoDB Tables (ddb) | Search tables where `SSEDescription.KMSMasterKeyArn` matches. | "Which DynamoDB tables use this key?" | P1 |
| SQS Queues (sqs) | Search queue attributes for `KmsMasterKeyId` matching this key. | "Which queues use this key?" | P1 |
| CloudWatch Log Groups (logs) | Search log groups where `kmsKeyId` matches. | "Which log groups use this key?" | P2 |
| CloudFormation Stacks (cfn) | Check tags. | "Which stack manages this key?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Key Policy → IAM Principals | `kms:GetKeyPolicy` returns the policy JSON. Parse `Statement[].Principal` to find IAM roles, users, accounts, and services with access. Key statements: `kms:Encrypt`, `kms:Decrypt`, `kms:GenerateDataKey`, `kms:DescribeKey`, `kms:CreateGrant`. | "Who can use this key?" THE security question. A key with overly broad access undermines all encryption. | P0 |
| Grants → IAM Roles | `kms:ListGrants` returns temporary delegated access. Grants are commonly created by AWS services (RDS, EBS) to perform encryption operations on behalf of a user. Shows `GranteePrincipal` (who gets access) and `Operations` (what they can do). | "Who has delegated access?" Grants are less visible than key policies but equally powerful. | P1 |
| Aliases | `kms:ListAliases` for this key — shows friendly names (e.g., `alias/aws/s3`, `alias/my-app-key`). Not a separate resource but critical context. | "What's the human-readable name for this key?" Key IDs are UUIDs — aliases provide meaning. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| ScheduleKeyDeletion | "Who scheduled this key for deletion?" 7-30 day countdown to permanent data loss for all encrypted resources. THE most critical KMS event — should trigger immediate investigation and key policy review. |
| DisableKey | "Who disabled this key?" Disabled keys can't encrypt or decrypt. All services using this key will start failing immediately. Reversible (re-enable), unlike deletion. |
| PutKeyPolicy | "Who changed the key policy?" Key policy changes can grant or revoke access to all data encrypted by this key. |
