# EC2 Instances (ec2) — Related Resources

## Real-World Use Cases

**1. "Why can't this instance receive traffic?"** You're looking at an EC2 instance that's running but unreachable. The instance's forward links show its security groups and subnet, but you need the REVERSE: which target groups have this instance registered, and is it healthy in those TGs? If it's unhealthy in the target group, the ALB isn't sending traffic to it — even though the instance itself is fine.

**2. "This instance is running but we didn't launch it."** During a security review or cost cleanup, you find an unfamiliar instance. You need to know: is it managed by an ASG? Is it part of an EKS node group? Was it created by a CloudFormation stack? The instance's own API response has none of this information — you need to cross-reference ASGs, node groups, and CFN stacks that claim this instance.

**3. "Which alarms will fire if this instance dies?"** You're about to terminate an instance during maintenance. Before you do, you need to know what monitoring is attached to it. CloudWatch alarms that use `InstanceId` as a dimension are invisible from the EC2 side — the instance has no idea it's being watched.

**4. "What was this instance doing before it crashed?"** An instance terminated unexpectedly. You need CloudTrail to see WHO terminated it (was it an ASG scale-in? manual action? spot reclamation?), and you need the SSM Session Manager history to see if someone was SSH'd in making changes.

**5. "Is this instance managed or orphaned?"** During infrastructure cleanup, you need to determine if an instance is managed by any automation (ASG, EKS, Elastic Beanstalk, OpsWorks) or if it's a hand-launched orphan. This requires checking multiple reverse relationships — if nothing claims it, it's likely orphaned.

## Reverse Relationships

Resources that reference this EC2 instance, but the instance has no pointer back.

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Target Groups (tg) | `elbv2:DescribeTargetHealth` for each TG, match on instance ID in `Target.Id`. Expensive at scale — requires iterating all TGs. Alternative: if a9s already has TG data loaded, search in-memory for the instance ID across all TG target lists. | "Why isn't this instance receiving ALB traffic?" Instance is registered but showing `unhealthy` in the TG. Or instance was deregistered and nobody told the app team. | P0 |
| Auto Scaling Groups (asg) | `autoscaling:DescribeAutoScalingInstances` with `InstanceIds` filter — returns the ASG name directly. Single API call, no iteration needed. | "Is this instance managed by an ASG?" Determines whether the instance will be replaced if terminated, and explains why it appeared or disappeared. | P0 |
| CloudWatch Alarms (alarm) | `cloudwatch:DescribeAlarms` then filter where `Dimensions` contains `Name=InstanceId, Value={instance-id}`. No server-side filter for alarm dimensions — must iterate alarms and filter client-side. If a9s has alarms cached, search in-memory. | "What monitoring is attached to this instance?" Before termination or during incident response, you need to know which alarms are watching this instance and whether any are in ALARM state. | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` and `aws:cloudformation:stack-id` tags on the instance. These tags are automatically applied by CFN when it creates the resource. If tags are absent, the instance wasn't created by CFN. | "Which IaC stack manages this instance?" Needed to understand if you can safely modify or terminate it, and to find the template that defines its configuration. | P1 |
| EKS Node Groups (ng) | Check for `eks:nodegroup-name` and `eks:cluster-name` tags. EKS-managed node groups apply these tags automatically. Alternative: `eks:DescribeNodegroup` for each NG and check if the instance's ASG matches the NG's ASG. | "Is this instance an EKS worker node?" Determines if the instance is part of a Kubernetes cluster and whether terminating it will affect pod scheduling. | P1 |
| ECS Container Instances (not in a9s) | `ecs:ListContainerInstances` + `ecs:DescribeContainerInstances` with filter on `ec2InstanceId`. Must iterate across ECS clusters. | "Is this instance running ECS tasks?" The EC2 instance is the host for an ECS container instance, which runs tasks. Terminating it would kill those tasks. | P1 |
| Elastic Beanstalk Environments (eb) | Check for `elasticbeanstalk:environment-name` tag. EB applies this tag automatically. Alternative: `elasticbeanstalk:DescribeEnvironmentResources` returns instance IDs per environment. | "Is this instance part of a Beanstalk environment?" Similar to ASG — determines if the instance is managed and will be replaced. | P2 |
| SSM Managed Instances (not in a9s) | `ssm:DescribeInstanceInformation` with `InstanceInformationFilterList` filtering on `InstanceIds`. Returns SSM agent status, platform, last ping time. | "Does this instance have SSM agent running? Can I Session Manager into it?" Also reveals the instance's OS and patch compliance status without needing SSH. | P2 |

## Algorithmic Relationships

Connections that require resource-specific logic, naming conventions, or multi-hop lookups.

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| EBS Snapshots (ebs-snap) | EC2 API response includes `BlockDeviceMappings[].Ebs.VolumeId`. For each attached volume, query `ec2:DescribeSnapshots` with `Filters=[{Name=volume-id, Values=[vol-xxx]}]` to find snapshots of those volumes. | "When was the last backup of this instance's volumes?" During incident recovery, you need to find the most recent snapshot to restore from. Also useful for cost analysis — old snapshots accumulate. | P1 |
| Elastic IP (eip) | EC2 API response includes `PublicIpAddress`, but does NOT indicate whether it's an Elastic IP or an auto-assigned public IP. Check `ec2:DescribeAddresses` with `Filters=[{Name=instance-id, Values=[i-xxx]}]` to find any EIP association. | "Will this instance keep its public IP if I stop/start it?" If it has an EIP, yes. If it has an auto-assigned IP, no. Critical for instances that need stable public addressing. | P1 |
| CloudWatch Log Groups (logs) | No reliable naming convention like Lambda. However: (1) check for CloudWatch agent config in SSM Parameter Store at `/AmazonCloudWatch-{instance-id}` or common paths, (2) search log groups for streams containing the instance ID in the stream name (common pattern: `{instance-id}/...`), (3) check SSM inventory for installed CloudWatch agent. This is heuristic — not guaranteed. | "Where are this instance's application logs?" Unlike Lambda, EC2 log destinations are manually configured. Best-effort lookup that works for instances using the standard CloudWatch agent setup. | P2 |
| Route 53 Records (r53) | Search across all hosted zones for A records pointing to the instance's private or public IP. Requires iterating `route53:ListResourceRecordSets` for each zone. Expensive, but answerable if a9s has R53 data cached. | "What DNS names point to this instance?" Useful when decommissioning — you need to update or remove DNS records that reference the instance's IP. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| TerminateInstances | "Who killed my instance?" The #1 most searched EC2 event. During incidents, you need to know: was it an ASG scale-in, a human, a Lambda, or a Spot interruption? The `userIdentity` and `requestParameters.instancesSet` tell the full story. |
| StopInstances | "Who stopped my instance and when?" Unexpected stops cause outages. The event shows the actor — often a cost-optimization Lambda or a well-meaning colleague. |
| RunInstances | "Who launched this instance and with what config?" During security reviews: was it launched with the right AMI, role, and security groups? The `requestParameters` contain the full launch configuration. |
