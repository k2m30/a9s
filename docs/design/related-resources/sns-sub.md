# SNS Subscriptions (sns-sub) — Related Resources

## Real-World Use Cases

**1. "Is this subscription actually delivering messages?"** The subscription exists and is confirmed, but is the endpoint receiving? For HTTPS endpoints, delivery failures are silent from the SNS side. For SQS endpoints, check the queue for messages.

**2. "What topic does this subscription belong to?"** Navigate from the subscription to its parent topic to understand the full notification chain — what publishes to the topic, and who else subscribes.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (None) | Subscriptions are leaf entities. Nothing references a subscription by ARN. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| SNS Topic (sns) | Subscription has `TopicArn` — FORWARD. Navigate to the topic to see all subscribers and understand the notification source. | "What topic is this subscription on?" The parent context. | P0 |
| Endpoint Resource | Parse `Endpoint` based on `Protocol`: `lambda` → extract Lambda ARN → navigate to Lambda. `sqs` → extract SQS ARN → navigate to SQS queue. `https` → URL (not an a9s resource, but useful for context — Slack webhooks, PagerDuty, etc.). `email` → email address (not navigable). `sms` → phone number (not navigable). | "Where do messages go?" Resolve the endpoint to a navigable resource in a9s. | P0 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| Unsubscribe | "Who removed this subscription?" Broken notification chains — alarms fire but nobody gets paged. |
| ConfirmSubscription | "Was this subscription confirmed?" Unconfirmed subscriptions (especially email and HTTPS) don't deliver messages. |
