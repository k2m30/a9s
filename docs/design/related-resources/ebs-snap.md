# EBS Snapshots (ebs-snap) — Related Resources

## Real-World Use Cases

**1. "Which AMIs depend on this snapshot?"** Before deleting an old snapshot to save costs, you need to verify no AMIs reference it — deleting a snapshot that backs an AMI corrupts the AMI.

**2. "Was this snapshot shared with another account?"** During a security audit, you need to check if the snapshot was made public or shared with specific accounts — this could leak data.

**3. "Which instance was this snapshot taken from?"** The snapshot says VolumeId but you need the instance that had that volume attached at snapshot time.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| AMIs (ami) | `ec2:DescribeImages` with `Filters=[{Name=block-device-mapping.snapshot-id, Values=[snap-xxx]}]`. Returns AMIs that use this snapshot in their block device mapping. | "Can I safely delete this snapshot?" If an AMI depends on it, deleting the snapshot breaks the AMI. | P0 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| EBS Volume (ebs) | Snapshot has `VolumeId` — FORWARD. Navigate to the source volume to see if it still exists and is attached. | "Does the source volume still exist?" If yes, this snapshot is a backup of an active volume. If no, this snapshot may be the only copy of the data. | P1 |
| EC2 Instance (ec2) | Parse the snapshot `Description` field. Common pattern for automated snapshots: `"Created by CreateImage(i-xxx) for ami-xxx"`. Also: check if the source volume (`VolumeId`) is currently attached to an instance. | "Which instance did this snapshot come from?" Useful for tracing data lineage — especially when the volume was created from a snapshot (copy chain). | P1 |
| KMS Key (kms) | If encrypted, snapshot has `KmsKeyId` — FORWARD. | "Who can access this snapshot's data?" The KMS key policy controls decryption access. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteSnapshot | "Who deleted this snapshot?" Potential data loss if it was the only backup. |
| ModifySnapshotAttribute | "Who shared this snapshot with another account?" Security-sensitive — could be data exfiltration if unexpected. |
| CopySnapshot | "Who copied this snapshot to another region or account?" Cross-region DR or potential data movement. |
