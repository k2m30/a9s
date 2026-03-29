# EventBridge Rules (eb-rule) — Related Resources

## Real-World Use Cases

**1. "What does this rule trigger?"** The rule has an event pattern or schedule, but the TARGETS are a separate API call. You need to see which Lambda functions, SQS queues, ECS tasks, or Step Functions this rule invokes.

**2. "Why didn't this scheduled rule fire?"** The rule shows `cron(0 8 * * ? *)` but the target Lambda has no recent invocations. Is the rule disabled? Is the target still valid (Lambda not deleted)?

**3. "What generates the events this rule catches?"** Parse the `EventPattern` to identify the source service (e.g., `aws.ec2`, `aws.ecs`, `aws.s3`) and event type. This tells you which resource changes trigger this rule.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal) | Rules are configuration entities. Other resources don't reference rules by ARN. The relationship flows from rule → targets. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Target Resources (various) | `events:ListTargetsByRule` returns targets with ARNs. Parse each ARN to identify the a9s resource type: `arn:aws:lambda:...` → Lambda, `arn:aws:sqs:...` → SQS, `arn:aws:states:...` → Step Functions, `arn:aws:ecs:...` → ECS (RunTask action), `arn:aws:sns:...` → SNS. Also a child view. | "What does this rule trigger?" The fundamental question. Navigate to the target to check its health and recent activity. | P0 |
| Source Service (from EventPattern) | Parse `EventPattern` JSON. The `source` field identifies the AWS service (e.g., `["aws.ec2"]`, `["aws.ecs"]`, `["aws.s3"]`). The `detail-type` field narrows the event type (e.g., `"EC2 Instance State-change Notification"`, `"ECS Task State Change"`). | "What triggers this rule?" Understanding the event source helps debug why the rule isn't firing — the source service may not be generating the expected events. | P1 |
| IAM Role (role) | Rule targets may have `RoleArn` — FORWARD. The role is used by EventBridge to invoke the target (e.g., to call `ecs:RunTask` or `states:StartExecution`). | "Why is the rule failing to invoke the target?" AccessDenied in the target invocation usually means the rule's IAM role lacks permissions. | P1 |
| Dead-Letter Queue (sqs) | Some targets have `DeadLetterConfig.Arn` — a DLQ for events that fail to be delivered to the target. | "Are events being dropped?" If the DLQ has messages, the target is rejecting or failing. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteRule | "Who deleted this rule?" Breaks the event-driven chain. Scheduled tasks stop running. |
| DisableRule | "Who disabled this rule?" Same effect as deletion but reversible. Scheduled Lambdas and event-driven workflows stop. |
| PutTargets / RemoveTargets | "Who changed what this rule triggers?" Adding or removing targets changes the downstream behavior. |
