# ECS Services (ecs-svc) — Related Resources

## Real-World Use Cases

**1. "Why is this service returning 502s?"** The ECS service shows 3/3 tasks running. But the ALB is returning errors. You need the multi-hop chain: Service → its target group ARN (from `loadBalancers[]`) → the target group → target health. The service knows its TG, but not the load balancer above it. And the TG health is a separate API call entirely.

**2. "What container image is deployed?"** The service references a task definition revision, but you need to drill into the task definition to see the image URI, which points to an ECR repository. Three hops: Service → TaskDefinition → container image → ECR repo and tag.

**3. "Who deployed the last change?"** A bad deployment is causing errors. You need CloudTrail to find the `UpdateService` event — who changed the task definition or desired count? Was it a human, a CodePipeline, or a CodeDeploy blue/green deployment?

**4. "Is auto-scaling fighting with my manual count change?"** You set desired count to 5 but it keeps going back to 3. You need to find the Application Auto Scaling policies targeting this service — they live in a completely different service (`application-autoscaling`), not in ECS.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CodePipeline (pipeline) | Search pipeline definitions for ECS deploy actions targeting this service. Requires `codepipeline:GetPipeline` for each pipeline and parsing action configurations. If a9s has pipeline data cached, search in-memory for this service name or cluster/service combo. | "Which pipeline deploys to this service?" Needed to understand the deployment flow and trace bad deployments back to the pipeline run. | P1 |
| CloudWatch Alarms (alarm) | Search alarms with dimensions `ServiceName` AND `ClusterName` matching this service. | "What monitoring watches this service?" Alarms on CPUUtilization, MemoryUtilization, or custom metrics from Container Insights. | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag on the service, or search CFN stack resources. | "Which IaC stack manages this service?" | P2 |
| Application Auto Scaling (not in a9s) | `application-autoscaling:DescribeScalableTargets` with `ServiceNamespace=ecs`, `ResourceIds=service/{cluster}/{service}`. Returns scaling min/max and whether scaling is enabled. Then `DescribeScalingPolicies` for the actual policies. | "Why does this service keep scaling when I set it to 5 tasks?" Auto scaling overrides manual count changes. | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Load Balancer (elb) | Multi-hop: Service response has `loadBalancers[].targetGroupArn`. Call `elbv2:DescribeTargetGroups` with that ARN → response has `LoadBalancerArns[]` → that's the ELB. Two API calls to bridge Service → TG → ELB. | "Which ALB/NLB fronts this service?" During 502 investigation, you need the load balancer's DNS name, listeners, and access logs. | P0 |
| CloudWatch Log Group (logs) | Multi-hop: Service has `taskDefinition` ARN → `ecs:DescribeTaskDefinition` → `containerDefinitions[].logConfiguration.options["awslogs-group"]`. The log group name is in the task definition, not the service. | "Where are this service's application logs?" THE most important cross-service link for ECS. Every debugging session needs logs. | P0 |
| ECR Repository (ecr) | Multi-hop: Service → TaskDefinition → `containerDefinitions[].image`. Parse the image URI: `{account}.dkr.ecr.{region}.amazonaws.com/{repo}:{tag}`. Extract the repo name. | "What image version is deployed? When was it pushed?" Confirms whether the right code is actually running. | P1 |
| Target Group (tg) | Service response has `loadBalancers[].targetGroupArn` — FORWARD. But the real value is navigating to the TG and seeing target health (are tasks healthy from the LB's perspective?). | "Are my tasks passing ALB health checks?" A task can be RUNNING in ECS but UNHEALTHY in the TG — meaning the ALB isn't sending traffic to it. | P0 |
| IAM Roles (role) | Service's task definition has two role ARNs: `taskRoleArn` (what the container can access) and `executionRoleArn` (what ECS needs to pull images and write logs). Both via task definition — `ecs:DescribeTaskDefinition`. | "Why is this container getting AccessDenied?" or "Why can't ECS pull the image?" Two different roles, two different permission scopes. | P1 |
| Secrets Manager / SSM Parameters | Task definition's `containerDefinitions[].secrets[]` lists secrets injected as env vars, with `valueFrom` containing Secrets Manager ARN or SSM parameter ARN. | "Which secrets does this service use? Has the secret value changed?" Debugging "worked yesterday, broken today" problems caused by rotated secrets. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| UpdateService | "Who deployed to this service or changed the desired count?" The single most important ECS audit event. Shows task definition change (deployment), desired count change (scaling), and force-new-deployment flag. |
| DeleteService | "Who deleted this service?" Services don't disappear on their own — someone or some automation removed it. |
| RegisterTaskDefinition | "Who registered a new task definition revision?" While not directly on the service, this is the precursor to `UpdateService` and shows the exact container image, environment variables, and resource limits in the new revision. |
