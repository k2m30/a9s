# ECS Clusters (ecs) — Related Resources

## Real-World Use Cases

**1. "How many services are running on this cluster and are any unhealthy?"** The cluster's own API response (`DescribeClusters`) returns aggregate statistics — `activeServicesCount`, `runningTasksCount` — but not the actual service list. You need the reverse: which services belong to this cluster?

**2. "Is this cluster running out of capacity?"** For EC2 launch type clusters, you need to see container instances (the EC2 hosts) and their remaining CPU/memory. The cluster knows how many container instances it has, but not their resource utilization — that requires `DescribeContainerInstances`.

**3. "Which capacity providers are actually placing tasks?"** The cluster has capacity provider strategy, but to see actual placement you need to cross-reference running tasks and their capacity provider assignments.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| ECS Services (ecs-svc) | `ecs:ListServices` with `cluster` filter. Returns service ARNs belonging to this cluster. If a9s has ECS service data loaded, filter in-memory by cluster name. | "What services run on this cluster?" The primary question when investigating a cluster. | P0 |
| CloudWatch Alarms (alarm) | Search alarms with `ClusterName` dimension matching this cluster. Container Insights alarms use this dimension. | "What monitoring is set up for this cluster?" | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag on the cluster. | "Which stack created this cluster?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| EC2 Instances (ec2) | Multi-hop: `ecs:ListContainerInstances` with cluster → `ecs:DescribeContainerInstances` → each has `ec2InstanceId`. | "Which EC2 instances are hosts for this cluster?" For EC2 launch type, these are the actual compute hosts. Needed for capacity analysis and to understand if an instance retirement affects the cluster. | P1 |
| ECS Tasks (ecs-task) | `ecs:ListTasks` with `cluster` filter. Returns all tasks (from all services plus standalone tasks). | "What's actually running on this cluster right now?" Includes both service-managed tasks and standalone tasks (one-off jobs, scheduled tasks). | P1 |
| Auto Scaling Groups (asg) | For EC2 launch type: container instances → EC2 instance IDs → check ASG membership via `autoscaling:DescribeAutoScalingInstances`. Cluster's capacity providers may also directly reference ASGs (`autoScalingGroupProvider.autoScalingGroupArn`). | "What ASG manages this cluster's compute capacity?" Essential for understanding scaling behavior and why new tasks can't be placed. | P1 |
| CloudWatch Log Groups (logs) | Container Insights uses log group `/aws/ecs/containerinsights/{cluster-name}/performance`. Also, each service's task definitions point to application log groups, but those are per-service, not per-cluster. | "Where are this cluster's performance metrics and logs?" Container Insights is the primary observability source for ECS clusters. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteCluster | "Who deleted this cluster?" Cluster deletion fails if services or tasks still exist, so this indicates a deliberate decommission. |
| UpdateCluster | "Who changed cluster settings (capacity providers, Container Insights, execute command)?" |
| PutClusterCapacityProviders | "Who changed the capacity provider strategy?" Affects how tasks are placed across Fargate vs EC2 vs Fargate Spot. |
