# Auto Scaling Groups (asg) — Related Resources

## Real-World Use Cases

**1. "Why did this ASG scale to 20 instances at 3 AM?"** The ASG response shows current desired/min/max but not WHY it changed. You need to find the scaling policies, and the CloudWatch alarms those policies are wired to. It's a three-hop chain: ASG → Scaling Policies → Alarm names → Alarm detail (was it in ALARM state at 3 AM?).

**2. "Is this ASG managed by EKS?"** During infrastructure review, you find an ASG you don't recognize. Check its tags for `eks:nodegroup-name` and `eks:cluster-name` — if present, it's an EKS-managed node group and shouldn't be modified directly.

**3. "Why can't new instances launch?"** The ASG is trying to scale but instances are failing. You need to check the launch template for the AMI (is it still valid?), the subnet (does it have available IPs?), and the instance type (is there capacity in this AZ?).

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| EKS Node Groups (ng) | Check for `eks:nodegroup-name` and `eks:cluster-name` tags on the ASG. EKS-managed node groups create and manage ASGs with these tags. Alternatively, if a9s has NG data loaded, NG response has `resources.autoScalingGroups[].name`. | "Is this ASG part of a Kubernetes cluster?" Changes to EKS-managed ASGs should go through the node group, not directly. | P0 |
| ECS Capacity Providers (not in a9s) | `ecs:DescribeCapacityProviders` — check `autoScalingGroupProvider.autoScalingGroupArn` for each. | "Is this ASG used as ECS cluster capacity?" The ASG is the compute backend for an ECS capacity provider. | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this ASG?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CloudWatch Alarms (alarm) | Multi-hop: `autoscaling:DescribePolicies` with `AutoScalingGroupName` → each policy has `Alarms[].AlarmName` → these are the CloudWatch alarms that drive scaling. Also search alarms with `AutoScalingGroupName` dimension for monitoring-only alarms. | "What triggers this ASG to scale?" THE critical question for understanding ASG behavior. Scaling policies without their linked alarms are meaningless. | P0 |
| EC2 Instances (ec2) | ASG response has `Instances[]` with instance IDs and lifecycle states — FORWARD. But the reverse is also valuable: given an EC2 instance, does it belong to this ASG? `autoscaling:DescribeAutoScalingInstances`. | "Which instances does this ASG currently manage?" Shows in-service vs terminating vs standby. | P0 |
| Launch Template (not in a9s) | ASG response has `LaunchTemplate.LaunchTemplateId` and `LaunchTemplate.Version` — FORWARD. The launch template contains AMI, instance type, key pair, security groups, user data. | "What configuration do new instances get?" Needed when instances are launching with wrong settings. | P1 |
| Target Groups (tg) | ASG response has `TargetGroupARNs` — FORWARD. Navigate to TGs to see health of ASG-managed instances from the load balancer's perspective. | "Are my ASG instances healthy from the ALB's point of view?" An instance can be InService in the ASG but unhealthy in the TG. | P1 |
| Subnets (subnet) | ASG response has `VPCZoneIdentifier` (comma-separated subnet IDs) — FORWARD. Navigate to subnets to check available IP addresses. | "Why can't the ASG launch new instances?" Often a subnet has run out of IPs, especially in EKS clusters with many pods. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| UpdateAutoScalingGroup | "Who changed desired/min/max capacity or other settings?" The most common ASG change. Shows whether a human, a pipeline, or AWS auto-scaling adjusted the group. |
| SetDesiredCapacity | "Who manually scaled this ASG?" Distinct from policy-driven scaling. If desired count changed but there's no SetDesiredCapacity event, it was a scaling policy. |
| DeleteAutoScalingGroup | "Who deleted this ASG?" Critical for EKS — deleting the node group's ASG directly causes serious problems. |
