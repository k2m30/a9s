# SNS Topics (sns) — Related Resources

## Real-World Use Cases

**1. "What publishes to this topic?"** An SNS topic receives messages from many sources — CloudWatch Alarms, S3 notifications, CloudFormation events, application code. The topic has no publisher list. The access policy hints at allowed publishers but doesn't enumerate active ones.

**2. "Who receives notifications from this topic?"** Subscriptions are the delivery mechanism. Navigate to subscriptions to see email addresses, SQS queues, Lambda functions, and HTTPS endpoints.

**3. "Why isn't the alarm notification reaching Slack?"** The alarm fires and sends to this SNS topic, but Slack doesn't get the message. Check subscriptions: is the HTTPS endpoint (Slack webhook) confirmed? Is it healthy?

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CloudWatch Alarms (alarm) | Search alarms where `AlarmActions[]`, `OKActions[]`, or `InsufficientDataActions[]` contain this topic's ARN. If a9s has alarm data cached, search in-memory. | "Which alarms publish to this topic?" THE most common reverse lookup for SNS. Understanding the alarm → notification chain. | P0 |
| S3 Buckets (s3) | Search S3 bucket notification configurations for `TopicConfigurations[].TopicArn` matching this topic. Expensive — requires iterating buckets. | "Which S3 buckets publish events to this topic?" | P1 |
| EventBridge Rules (eb-rule) | Search EventBridge rules for targets with this topic's ARN. | "Which EventBridge rules send events to this topic?" | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. Also: CFN can send stack event notifications to SNS topics. Check stacks' `NotificationARNs` field. | "Which CFN stacks notify this topic on deploy events?" | P2 |
| Budgets (not in a9s) | AWS Budgets can send alerts to SNS topics. `budgets:DescribeBudgets` and check notification configurations. | "Does this topic receive cost alerts?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| SNS Subscriptions (sns-sub) | `sns:ListSubscriptionsByTopic` — returns all subscriptions with protocol (email, sqs, lambda, https) and endpoint. This is also a child view. | "Who receives notifications?" Navigate to each subscription to see confirmation status and delivery details. | P0 |
| Access Policy → Publishers | `sns:GetTopicAttributes` returns `Policy` JSON. Parse `Condition` and `Principal` to identify which services and accounts are allowed to publish. `aws:SourceArn` conditions reveal specific resources (alarms, S3 buckets) authorized to publish. | "Who is ALLOWED to publish here?" Broader than who actually publishes — shows the full trust model. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteTopic | "Who deleted this topic?" Breaks all alarm notifications, S3 event notifications, and any other publisher-subscriber chain. Catastrophic if this was the incident notification topic. |
| SetTopicAttributes | "Who changed the access policy or delivery settings?" Policy changes can block or allow new publishers. |
| Subscribe / Unsubscribe | "Who added or removed a subscriber?" Adding an unauthorized subscriber can leak notifications. Removing a subscriber breaks notification delivery. |
