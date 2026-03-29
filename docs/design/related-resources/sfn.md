# Step Functions (sfn) — Related Resources

## Real-World Use Cases

**1. "What services does this workflow orchestrate?"** The state machine definition (ASL JSON) contains `Resource` ARNs for every task state — Lambda functions, ECS tasks, SQS queues, DynamoDB tables, SNS topics, other Step Functions. Understanding the orchestration graph is THE primary question for Step Functions.

**2. "Why isn't this workflow running?"** If triggered by EventBridge, check the rule and its target configuration. If triggered by API Gateway, check the integration. The state machine's own API response doesn't tell you what triggers it.

**3. "Why is this step failing?"** Navigate from the Step Function to the specific resource that failed — usually a Lambda function. The execution history shows the error, but fixing it requires visiting the Lambda's code and logs.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| EventBridge Rules (eb-rule) | Search EventBridge rules for targets with this state machine's ARN. `events:ListRuleNamesByTarget` with this SFN ARN (if the API supports it), or iterate rules and check targets. | "What triggers this workflow?" Schedule-based or event-driven invocation. | P0 |
| API Gateway (apigw) | Search API integrations for Step Functions integration type with this state machine ARN. Requires iterating APIs and routes. | "Is this workflow triggered by an API call?" | P1 |
| CloudFormation Stacks (cfn) | Check tags. | "Which stack manages this state machine?" | P2 |
| CloudWatch Alarms (alarm) | Search alarms with `StateMachineArn` dimension. | "What monitoring watches this workflow?" Alarms on ExecutionsFailed, ExecutionsTimedOut. | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Orchestrated Resources (various) | Parse the state machine `definition` (ASL JSON). For each Task state, extract the `Resource` field: `arn:aws:lambda:...` → Lambda, `arn:aws:states:::ecs:runTask` → ECS, `arn:aws:states:::sqs:sendMessage` → SQS, `arn:aws:states:::dynamodb:putItem` → DynamoDB, `arn:aws:states:::sns:publish` → SNS, `arn:aws:states:::states:startExecution` → nested SFN. The `Parameters` field often contains the specific resource ARN or name. | "What does this workflow touch?" The orchestration graph — which resources are called at each step. Navigating to the failing step's resource is the core debugging path. | P0 |
| IAM Role (role) | State machine has `roleArn` — FORWARD. The role determines what the workflow is ALLOWED to do. Navigate to the role to check permissions. | "Why is this step getting AccessDenied?" The SFN execution role must have permissions for every target service. | P1 |
| CloudWatch Log Group (logs) | State machine `loggingConfiguration.destinations[].cloudWatchLogsLogGroup.logGroupArn` — FORWARD (if logging enabled). Only available for STANDARD type (not EXPRESS with inline logging). | "Where are execution logs?" Useful for debugging complex workflows. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteStateMachine | "Who deleted this state machine?" All ongoing executions are affected. |
| UpdateStateMachine | "Who changed the workflow definition or role?" Definition changes affect step logic; role changes affect permissions. |
| StartExecution | "Who triggered this execution?" Shows the initiator (human, EventBridge, API GW) and any input overrides. |
