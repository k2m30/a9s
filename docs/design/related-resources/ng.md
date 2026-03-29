# EKS Node Groups (ng) — Related Resources

## Real-World Use Cases

**1. "Why can't new pods schedule on this node group?"** Pods are pending but the node group has available capacity. You need to check the underlying ASG instances — are they healthy? Is the AMI valid? Are there subnet IP exhaustion issues?

**2. "What AMI version are these nodes running?"** After a security advisory, you need to verify nodes are running a patched AMI. The node group has a `releaseVersion` but the actual AMI details are in the launch template.

**3. "Why is this node group upgrade stuck?"** An EKS version upgrade is in progress. You need to see the ASG's instances and their lifecycle states — which nodes have been replaced and which are still on the old version?

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal reverse relationships) | Node groups are leaf resources — few things reference them by ARN. The primary navigation is FROM the node group TO its underlying resources. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Auto Scaling Group (asg) | NG response has `resources.autoScalingGroups[].name` — FORWARD. Navigate to the ASG for scaling activities, instance health, and launch template details. | "What's the actual ASG behind this node group?" The ASG is the real compute manager — its scaling activities explain why nodes appeared or disappeared. | P0 |
| EC2 Instances (ec2) | Multi-hop: NG → ASG name → `autoscaling:DescribeAutoScalingGroups` → `Instances[]`. Alternatively, search EC2 instances for tags `eks:cluster-name={cluster}` AND `eks:nodegroup-name={ng-name}`. | "Which EC2 instances are in this node group?" Needed for diagnosing node-level issues (disk pressure, NotReady status). | P0 |
| Launch Template (not in a9s) | NG response has `launchTemplate.id` and `launchTemplate.version` — FORWARD. The launch template contains AMI, instance type, user data (bootstrap script), and security groups. | "What config do new nodes get?" Essential for debugging nodes joining with wrong settings. | P1 |
| IAM Role (role) | NG response has `nodeRole` ARN — FORWARD. Navigate to the role to see its policies (needs ECR pull, EC2, and EKS permissions). | "Why can't nodes pull images or join the cluster?" Missing IAM permissions on the node role. | P1 |
| Subnets (subnet) | NG response has `subnets[]` — FORWARD. Navigate to subnets to check available IP addresses, especially critical for pod networking. | "Why are pods failing to get IPs?" Subnet IP exhaustion is the #1 EKS networking issue. | P1 |
| EKS Cluster (eks) | NG response has `clusterName` — FORWARD. Navigate to the parent cluster. | "What cluster does this node group belong to?" | P0 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteNodegroup | "Who deleted this node group?" Removing a node group terminates all its instances and evicts all pods. |
| UpdateNodegroupConfig | "Who changed scaling config (min/max/desired) or labels/taints?" Label changes can make pods unschedulable. |
| UpdateNodegroupVersion | "Who triggered an AMI or Kubernetes version update?" Version updates roll through nodes one by one — this event starts the process. |
