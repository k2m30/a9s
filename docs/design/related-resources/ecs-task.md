# ECS Tasks (ecs-task) — Related Resources

## Real-World Use Cases

**1. "Why did this task stop?"** The task has `StoppedReason` in its API response, but the actual application error is in CloudWatch Logs. You need to follow the task definition's log configuration to the right log group and stream — the stream name follows the pattern `{prefix}/{container-name}/{task-id}`.

**2. "Which EC2 host is this task running on?"** For EC2 launch type, the task has `containerInstanceArn` but not the EC2 instance ID directly. You need a second hop: task → container instance → EC2 instance. For troubleshooting host-level issues (disk full, network saturation).

**3. "Is this task's container actually reachable?"** For `awsvpc` network mode, the task has its own ENI with a private IP. You need to check the security group on that ENI to verify port access.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Target Groups (tg) | If the task belongs to a service with a load balancer, the TG has this task's IP registered. `elbv2:DescribeTargetHealth` for the service's TG, match on this task's private IP. | "Is the ALB sending traffic to this specific task? Is it healthy?" | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CloudWatch Log Group (logs) | Task has `taskDefinitionArn` → `ecs:DescribeTaskDefinition` → `containerDefinitions[].logConfiguration.options["awslogs-group"]`. The log stream name is `{awslogs-stream-prefix}/{container-name}/{task-id}` where task-id is the short ID extracted from the task ARN. | "Show me this task's application logs." The single most important link for debugging a specific task. | P0 |
| EC2 Instance (ec2) | For EC2 launch type: Task has `containerInstanceArn` → `ecs:DescribeContainerInstances` → `ec2InstanceId`. For Fargate: no EC2 instance (runs on AWS-managed compute). | "Which host is this task running on?" Needed when the issue is host-level (memory pressure, disk full, network). | P1 |
| ENI / Security Group (eni, sg) | For `awsvpc` network mode: Task has `attachments[]` with `type=ElasticNetworkInterface` containing `networkInterfaceId`, `privateIPv4Address`, and `subnetId`. The ENI has security groups. | "Why can't this task connect to the database?" Check the ENI's security group rules for the required outbound port. | P1 |
| ECS Service (ecs-svc) | Task has `group` field — for service-managed tasks, format is `service:{service-name}`. Extract the service name. Also `startedBy` may contain the service name. | "Which service manages this task?" Needed to understand if the task will be replaced when stopped, and to navigate to the service's deployment config. | P0 |
| Secrets Manager / SSM (secrets, ssm) | Via task definition: `containerDefinitions[].secrets[].valueFrom` contains Secrets Manager ARNs or SSM parameter ARNs. | "Which secrets are injected into this task?" Debugging "secret not found" or "access denied to secret" errors. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| StopTask | "Who manually stopped this task?" Tasks stopped by the scheduler or service don't generate StopTask — only manual stops. Seeing this event means a human or automation deliberately killed the task. |
| RunTask | "Who launched this standalone task?" For one-off tasks (migrations, batch jobs) not managed by a service. Shows the actor, task definition, and overrides. |
| ExecuteCommand | "Who exec'd into this container?" The ECS Exec equivalent of SSH — shows who opened an interactive session. Security-sensitive event. |
