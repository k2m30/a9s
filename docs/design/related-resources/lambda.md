# Lambda (lambda) — Related Resources

## Real-World Use Cases

**1. "This Lambda is failing — what's triggering it?"** You see errors in monitoring but the Lambda's own configuration only shows runtime, memory, handler. You need to find the EVENT SOURCES: is it an API Gateway route, an SQS queue, an EventBridge rule, an S3 notification? The Lambda API response has no field listing all triggers — they live on the triggering services' side.

**2. "Who can invoke this function?"** During a security review, you need to know the Lambda's resource policy (who can call it) and its execution role (what it can access). The resource policy is a separate API call, and the execution role's actual permissions require navigating to the IAM role and its attached policies.

**3. "This function is throttling — what's driving the traffic?"** Lambda concurrency is shared across all triggers. You need to find ALL event sources (SQS queues, API GW routes, EventBridge rules, SNS subscriptions) to understand the aggregate invocation pattern. No single API shows you this.

**4. "Why is this Lambda getting AccessDenied on S3?"** You need to jump from the Lambda to its IAM execution role, then see the role's attached policies. This is a three-hop navigation: Lambda → Role ARN → Role detail → attached policies.

**5. "Is this Lambda still needed?"** During cleanup, you check: is anything triggering it? Are there recent CloudTrail invocations? Is it part of a Step Function workflow? Without reverse relationship navigation, answering this requires checking multiple services manually.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| EventBridge Rules (eb-rule) | `events:ListRuleNamesByTarget` with this Lambda's ARN as `TargetArn`. Single API call — purpose-built for this reverse lookup. | "What schedule or event pattern triggers this Lambda?" EventBridge is the most common trigger for scheduled Lambdas. | P0 |
| SNS Subscriptions (sns-sub) | `sns:ListSubscriptions` and filter for `Protocol=lambda` with `Endpoint` matching this function's ARN. Or if a9s has SNS data cached, filter in-memory. | "Which SNS topics trigger this Lambda?" Common for fan-out architectures and alarm-response functions. | P1 |
| API Gateway (apigw) | For HTTP APIs: `apigatewayv2:GetApis` then `apigatewayv2:GetIntegrations` for each, match on `IntegrationUri` containing this Lambda ARN. For REST APIs: requires iterating resources and methods. Expensive. | "Which API endpoint invokes this function?" Critical for understanding the Lambda's role in a web service. | P1 |
| S3 Bucket Notifications (s3) | Requires iterating S3 buckets and calling `s3:GetBucketNotificationConfiguration` for each, checking `LambdaFunctionConfigurations[].LambdaFunctionArn`. Expensive — best done against cached S3 data. | "Which S3 bucket triggers this Lambda on object upload?" Common for image processing, ETL intake pipelines. | P1 |
| Target Groups (tg) | `elbv2:DescribeTargetHealth` for each TG — Lambda can be an ALB target. Check if target type is `lambda` and target ID matches this function ARN. | "Is this Lambda behind an ALB?" Less common than API GW but used for ALB-backed Lambda architectures. | P2 |
| Step Functions (sfn) | Requires parsing state machine definitions (ASL JSON) for `Resource` fields matching this Lambda ARN. `states:ListStateMachines` → `states:DescribeStateMachine` → parse `definition`. Expensive. | "Is this Lambda a step in a workflow?" Important for understanding orchestration dependencies. | P2 |
| CloudWatch Alarms (alarm) | Search alarms with `FunctionName` dimension matching this function name. | "What monitoring watches this Lambda?" Alarms on Errors, Duration, Throttles metrics. | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag, or search CFN stack resources for this Lambda's physical ID. | "Which IaC stack manages this Lambda?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CloudWatch Log Group (logs) | Naming convention: `/aws/lambda/{function-name}`. Override: check `LoggingConfig.LogGroup` field in the Lambda configuration response — if set, it overrides the default. | "Show me this Lambda's logs." THE most important cross-service link for Lambda. Every Lambda debugging session starts here. | P0 |
| SQS Event Sources (sqs) | `lambda:ListEventSourceMappings` with `FunctionName` filter. Returns SQS queue ARNs that this Lambda polls. Note: this is a Lambda API call (forward-ish), but the SQS queue has no pointer to Lambda. | "Which SQS queues does this Lambda consume from?" Event source mappings are the primary trigger mechanism for queue-processing Lambdas. | P0 |
| DynamoDB Stream Triggers (ddb) | Same `lambda:ListEventSourceMappings` — returns DynamoDB stream ARNs. Extract table name from the stream ARN. | "Which DynamoDB table changes trigger this Lambda?" Common in event-driven architectures. | P1 |
| Kinesis Stream Triggers (kinesis) | Same `lambda:ListEventSourceMappings` — returns Kinesis stream ARNs. | "Which Kinesis streams does this Lambda process?" | P1 |
| MSK/Kafka Triggers (msk) | Same `lambda:ListEventSourceMappings` — returns MSK cluster ARN or self-managed Kafka bootstrap servers. | "Which Kafka topics does this Lambda consume?" | P2 |
| IAM Role (role) | Lambda response has `Role` ARN — FORWARD. But the VALUE is navigating to that role's policies to understand permissions. Multi-hop: Lambda → Role ARN → `iam:ListAttachedRolePolicies`. | "What permissions does this Lambda have?" Essential for debugging AccessDenied errors. | P0 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| UpdateFunctionCode20150331v2 | "Who deployed new code to this Lambda?" The most important Lambda audit event. Shows the actor, the deployment package source (S3 or direct upload), and timestamp. Answers "did someone deploy and break it?" |
| UpdateFunctionConfiguration20150331v2 | "Who changed the memory, timeout, environment variables, or VPC config?" Configuration changes cause subtle failures — a reduced timeout causes timeouts, a changed env var points to the wrong database. |
| DeleteFunction20150331 | "Who deleted this Lambda?" Functions can be accidentally deleted, breaking dependent services. The event shows the actor and whether aliases/versions were also removed. |
