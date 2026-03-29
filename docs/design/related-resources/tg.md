# Target Groups (tg) — Related Resources

## Real-World Use Cases

**1. "Why is the ALB returning 502s?"** You found the target group. Now you need to navigate in two directions: DOWN to target health (are backends healthy?), and UP to the load balancer and its listeners (is routing correct?). Target health is a child view, but the ELB and ECS service that OWN this TG are reverse/algorithmic relationships.

**2. "Which ECS service registers targets in this TG?"** A TG has registered instances or IPs, but doesn't know which ECS service manages them. You need the reverse: search ECS services for ones whose `loadBalancers[]` includes this TG ARN.

**3. "Why does this TG keep registering and deregistering targets?"** An ASG or ECS service is cycling targets. You need to find which ASG has this TG in its `TargetGroupARNs`, or which ECS service references it.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| ECS Services (ecs-svc) | Search ECS services for `loadBalancers[].targetGroupArn` matching this TG's ARN. Requires iterating clusters → services. If a9s has ECS service data cached, search in-memory. | "Which ECS service manages targets in this TG?" The most important reverse link for TGs in containerized architectures. | P0 |
| Auto Scaling Groups (asg) | Search ASGs for `TargetGroupARNs` containing this TG's ARN. `autoscaling:DescribeAutoScalingGroups` and filter. If a9s has ASG data cached, search in-memory. | "Which ASG registers instances into this TG?" For EC2-based architectures where ASGs manage target registration. | P0 |
| CloudWatch Alarms (alarm) | Search alarms with `TargetGroup` dimension matching this TG's ARN suffix. Often paired with `LoadBalancer` dimension. | "What monitoring watches this TG?" Alarms on UnHealthyHostCount, TargetResponseTime. | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this TG?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Load Balancer (elb) | TG response has `LoadBalancerArns[]` — FORWARD. Navigate to the ELB to see listeners, DNS name, and routing configuration. | "Which load balancer sends traffic to this TG?" Needed to understand the full traffic path: client → DNS → ELB → TG → targets. | P0 |
| EC2 Instances (ec2) | For instance-type TGs: `elbv2:DescribeTargetHealth` returns target IDs which are EC2 instance IDs. Map these to EC2 instances in a9s. | "Which specific EC2 instances are registered?" Navigate to the instance for OS-level investigation. | P1 |
| Lambda Function (lambda) | For lambda-type TGs: the single registered target is a Lambda function ARN. `elbv2:DescribeTargetHealth` returns it. | "Which Lambda function does this TG invoke?" For ALB-backed Lambda architectures. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeregisterTargets | "Who removed targets from this TG?" Unexpectedly deregistered targets cause traffic drops. Shows the actor — was it an ASG scale-in, ECS, or a human? |
| ModifyTargetGroupAttributes | "Who changed health check settings, deregistration delay, or stickiness?" A changed health check path can mark all targets unhealthy instantly. |
| DeleteTargetGroup | "Who deleted this TG?" Breaks the ELB listener configuration if still referenced. |
