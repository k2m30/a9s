# EBS Volumes (ebs) — Related Resources

## Real-World Use Cases

**1. "When was this volume last backed up?"** You're reviewing disaster recovery posture. The volume itself has no backup field — you need to find snapshots created from this volume and check the most recent one's timestamp.

**2. "This volume is full — can I see its I/O metrics?"** The volume's API response shows size and type (gp3, io2), but not utilization. CloudWatch metrics for the volume (VolumeQueueLength, VolumeReadOps, BurstBalance) require a separate lookup.

**3. "Who detached this volume from my instance?"** A volume is `available` (unattached) but should be attached. CloudTrail shows the DetachVolume event with the actor.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| EBS Snapshots (ebs-snap) | `ec2:DescribeSnapshots` with `Filters=[{Name=volume-id, Values=[vol-xxx]}]`. Returns all snapshots created from this volume, newest first. | "When was the last backup? How many snapshots exist?" Critical for DR verification and cost analysis (orphaned snapshots accumulate). | P0 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Is this volume managed by IaC?" Determines if it's safe to modify directly. | P2 |
| Backup Recovery Points (not in a9s) | `backup:ListRecoveryPointsByResource` with this volume's ARN. | "Is this volume protected by AWS Backup?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| EC2 Instance (ec2) | Volume response has `Attachments[].InstanceId` — FORWARD. But if unattached (`State=available`), check the most recent DetachVolume CloudTrail event to find the last instance it was attached to. | "Which instance uses this volume?" or "Which instance DID it belong to before detachment?" | P0 |
| KMS Key (kms) | If encrypted, volume has `KmsKeyId` — FORWARD. Navigate to the KMS key to check key policy and rotation status. | "Who has access to this volume's encryption key?" Security audit. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DetachVolume | "Who detached this volume from the instance?" An unexpectedly detached volume causes application failures (data directory disappears). |
| DeleteVolume | "Who deleted this volume?" Data loss event if no snapshots exist. |
| CreateVolume | "Who created this volume and with what specs?" Useful for cost analysis — who created a 1TB io2 volume? |
