# AMIs (ami) — Related Resources

## Real-World Use Cases

**1. "How many instances are running this AMI?"** Before deregistering an old AMI, you need to know if any instances or launch templates still reference it. Deregistering an AMI doesn't affect running instances, but it prevents new launches.

**2. "Is this AMI still the latest golden image?"** Your team publishes monthly hardened AMIs. You need to find which ASGs and launch templates reference this AMI versus the newer version.

**3. "This AMI was shared with another account — should it be?"** Security audit: check the AMI's launch permissions for unexpected account access.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| EC2 Instances (ec2) | `ec2:DescribeInstances` with `Filters=[{Name=image-id, Values=[ami-xxx]}]`. Returns all running/stopped instances launched from this AMI. | "Is anyone still using this AMI?" Before deregistration, verify no instances depend on it for stop/start cycles (terminated instances don't matter). | P0 |
| Launch Templates (not in a9s) | `ec2:DescribelaunchTemplateVersions` and filter for `ImageId` matching this AMI. Requires iterating launch templates. | "Which launch templates reference this AMI?" A launch template using a deregistered AMI will fail to launch instances. | P1 |
| Auto Scaling Groups (asg) | Multi-hop: ASG → launch template → AMI. Requires checking each ASG's launch template version for this AMI's ID. If a9s has ASG data cached, cross-reference. | "Which ASGs will be affected if I deregister this AMI?" ASGs with this AMI in their launch template will fail to scale out. | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| EBS Snapshots (ebs-snap) | AMI response has `BlockDeviceMappings[].Ebs.SnapshotId` — FORWARD. These snapshots must exist for the AMI to function. | "Which snapshots back this AMI?" Deleting a backing snapshot corrupts the AMI. | P1 |
| Source EC2 Instance (ec2) | AMI name or description often contains the source instance ID. Also, `ec2:DescribeImages` returns `SourceInstanceId` for AMIs created via `CreateImage`. | "Which instance was this AMI created from?" Trace the lineage — useful for understanding what software and config is baked in. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeregisterImage | "Who deregistered this AMI?" If an AMI disappears, find out who removed it and whether it was intentional. |
| ModifyImageAttribute | "Who shared this AMI with another account?" Security event — AMI sharing can expose proprietary software or sensitive data baked into the image. |
| CreateImage | "Who created this AMI and from which instance?" Audit trail for golden image pipeline. |
