# CloudWatch Alarms (alarm) — Related Resources

## Real-World Use Cases

**1. "What resource is this alarm monitoring?"** The alarm has dimensions (e.g., `InstanceId`, `DBInstanceIdentifier`, `FunctionName`), but these are opaque key-value pairs. You need to map the dimension to the actual resource in a9s and navigate there to see its current state.

**2. "What happens when this alarm fires?"** The alarm's actions send to SNS topics — but who subscribes to those topics? You need to follow: Alarm → SNS Topic ARN → SNS Subscriptions (email, Lambda, PagerDuty webhook). Also: does this alarm trigger auto-scaling?

**3. "Is this alarm part of a composite alarm?"** Composite alarms combine multiple child alarms with AND/OR logic. If this alarm is referenced by a composite, silencing it or changing its threshold affects the composite's behavior.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| ASG Scaling Policies (asg) | `autoscaling:DescribePolicies` and check each policy's `Alarms[].AlarmName` for this alarm. If a9s has ASG data cached, scan policies in-memory. | "Does this alarm drive auto-scaling?" Modifying the alarm threshold directly affects scaling behavior. | P0 |
| Composite Alarms (alarm) | `cloudwatch:DescribeAlarms` with `AlarmTypes=["CompositeAlarm"]`, then parse each composite's `AlarmRule` for this alarm's name. | "Which composite alarms include this one?" Changing this alarm's threshold or state affects the composite. | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Monitored Resource | Parse alarm `Dimensions[]` to identify the resource type and ID. Dimension name → resource type mapping: `InstanceId` → EC2, `DBInstanceIdentifier` → RDS, `FunctionName` → Lambda, `LoadBalancer` → ELB, `TargetGroup` → TG, `TableName` → DynamoDB, `QueueName` → SQS, `CacheClusterId` → Redis, `ClusterName` + `ServiceName` → ECS Service, `ClusterName` alone → EKS or ECS Cluster, `AutoScalingGroupName` → ASG, `FileSystemId` → EFS, `DomainName` → OpenSearch, `BucketName` → S3, `TopicName` → SNS, `StreamName` → Kinesis, `NatGatewayId` → NAT GW. | "What resource is this alarm watching?" THE most important navigation from an alarm. Every alarm investigation starts with "let me see the resource." | P0 |
| SNS Topics (sns) | Alarm response has `AlarmActions[]`, `OKActions[]`, `InsufficientDataActions[]` — FORWARD. These are SNS topic ARNs (or Lambda/EC2 action ARNs). Navigate to the SNS topic to see who receives notifications. | "Who gets paged when this alarm fires?" Critical for incident response — verify the right people are notified. | P0 |
| CloudWatch Dashboard (not in a9s) | Heuristic: `cloudwatch:ListDashboards` then parse dashboard JSON bodies for references to this alarm name. Expensive. | "Which dashboard shows this alarm?" Low priority in a TUI context. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| PutMetricAlarm | "Who created or modified this alarm?" Threshold changes can suppress real incidents or cause false alarms. Shows the full alarm definition including new threshold, period, and evaluation periods. |
| DeleteAlarms | "Who deleted this alarm?" Removing monitoring is a security-sensitive action — was it intentional cleanup or covering tracks? |
| SetAlarmState | "Who manually set the alarm to OK/ALARM?" Manual state changes suppress or force alarms. Usually done during maintenance windows. |
