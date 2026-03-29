# SQS Queues (sqs) — Related Resources

## Real-World Use Cases

**1. "What processes messages from this queue?"** The queue has messages piling up, but you don't know what consumes them. Is it a Lambda event source mapping? An ECS service polling? A standalone worker? The queue has no consumer field.

**2. "Why is the dead-letter queue filling up?"** You found a DLQ with thousands of messages. You need the REVERSE: which source queue sends failed messages to this DLQ? AWS provides a purpose-built API for this.

**3. "Who sends messages to this queue?"** Understanding producers: SNS subscriptions that deliver here, EventBridge rules that target it, S3 event notifications that push to it. The queue's access policy hints at producers but doesn't enumerate them.

**4. "Is this queue's DLQ monitored?"** If a DLQ exists but has no alarm on `ApproximateNumberOfMessagesVisible`, failed messages accumulate silently.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Lambda Functions (lambda) | `lambda:ListEventSourceMappings` with `EventSourceArn` matching this queue's ARN. Returns Lambda functions that poll this queue. | "What processes messages from this queue?" The most common consumer pattern for SQS. | P0 |
| SNS Subscriptions (sns-sub) | `sns:ListSubscriptions` and filter for `Protocol=sqs` with `Endpoint` matching this queue's ARN. Or if a9s has SNS subscription data cached, filter in-memory. | "Which SNS topics send messages here?" SNS → SQS fan-out is a core messaging pattern. | P0 |
| EventBridge Rules (eb-rule) | Search EventBridge rules for targets with this queue's ARN. `events:ListTargetsByRule` for each rule. If a9s has EventBridge data cached, search in-memory. | "Which EventBridge rules send events to this queue?" | P1 |
| S3 Buckets (s3) | Search S3 bucket notification configurations for `QueueConfigurations[].QueueArn` matching this queue. Expensive — requires iterating buckets. | "Which S3 buckets send notifications to this queue?" | P1 |
| CloudWatch Alarms (alarm) | Search alarms with `QueueName` dimension matching this queue name. | "What monitoring watches this queue?" Critical alarm: ApproximateNumberOfMessagesVisible on DLQs. | P0 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag (requires `sqs:ListQueueTags`). | "Which stack manages this queue?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Dead-Letter Queue (sqs) | Queue attributes have `RedrivePolicy` containing `deadLetterTargetArn` — FORWARD. Navigate to the DLQ to check for accumulated failed messages. | "Where do failed messages go?" If the DLQ is filling up, something is wrong with message processing. | P0 |
| Source Queues (sqs) — if THIS is a DLQ | `sqs:ListDeadLetterSourceQueues` with this queue's URL. Purpose-built API — returns all queues that use this one as their DLQ. | "Which queues send failures here?" THE critical reverse lookup for DLQs. A DLQ without knowing its sources is useless. | P0 |
| KMS Key (kms) | Queue attributes have `KmsMasterKeyId` — FORWARD (if encrypted). | "Who can decrypt messages in this queue?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteQueue | "Who deleted this queue?" Messages in flight are lost. Consumers break. |
| SetQueueAttributes | "Who changed the queue settings?" Visibility timeout, DLQ policy, retention period, or access policy changes. Changing the DLQ policy can silently redirect failed messages. |
| PurgeQueue | "Who purged all messages?" Intentional data destruction — all messages in the queue are deleted. |
