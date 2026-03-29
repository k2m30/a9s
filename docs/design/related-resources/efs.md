# EFS File Systems (efs) — Related Resources

## Real-World Use Cases

**1. "Which instances or containers mount this file system?"** EFS is shared storage — multiple EC2 instances, ECS tasks, and Lambda functions can mount the same EFS. But EFS has no API that lists its consumers. You need to find mount targets and then trace which resources connect to them.

**2. "Why can't my container mount EFS?"** Check mount targets exist in the right subnets, and that the mount target security groups allow NFS traffic (port 2049) from the container's security group.

**3. "Is this EFS backed up?"** Check both EFS automatic backups and AWS Backup plans.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Lambda Functions (lambda) | `lambda:ListFunctions` and check `FileSystemConfigs[].Arn` for this EFS ARN. If a9s has Lambda data cached, search in-memory. | "Which Lambdas mount this EFS?" Lambda-EFS is used for large ML models, shared libraries, or persistent Lambda storage. | P1 |
| ECS Task Definitions (not in a9s) | Search task definitions for `volumes[].efsVolumeConfiguration.fileSystemId` matching this EFS. | "Which ECS tasks mount this EFS?" | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this EFS?" | P2 |
| Backup Plans (backup) | `backup:ListProtectedResources` and check for this EFS ARN. | "Is this EFS in a backup plan?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Mount Targets / Subnets (subnet) | `efs:DescribeMountTargets` with `FileSystemId`. Each mount target is in a specific subnet and AZ. Navigate to subnets to verify connectivity. | "Which AZs can mount this EFS?" EFS needs a mount target in the same AZ as the consumer. | P0 |
| Security Groups (sg) | Each mount target has security groups: `efs:DescribeMountTargetSecurityGroups` with `MountTargetId`. Navigate to SGs to verify NFS port 2049 is open. | "Why can't the instance mount EFS?" SG must allow inbound NFS from the client's SG. | P0 |
| Access Points (not in a9s) | `efs:DescribeAccessPoints` with `FileSystemId`. Access points provide POSIX-enforced paths and user/group IDs for multi-tenant access. | "How is access partitioned?" ECS tasks and Lambdas often use access points for isolation. | P1 |
| KMS Key (kms) | EFS response has `KmsKeyId` — FORWARD (if encrypted). | "Who controls the encryption key?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteFileSystem | "Who deleted this EFS?" All data is lost. Mount targets must be deleted first, so this is deliberate. |
| DeleteMountTarget | "Who removed a mount point?" Resources in that AZ can no longer mount the EFS. |
| PutFileSystemPolicy | "Who changed the resource policy?" Policy controls cross-account and root access. |
