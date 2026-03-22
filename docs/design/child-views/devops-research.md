# Child View Candidates: Operational Workflow Research

**Author:** a9s-devops
**Date:** 2026-03-22
**Status:** Complete -- ready for review before Phase 2

## Methodology

The previous version of this document was organized around AWS API boundaries -- asking "what other APIs does this service offer?" That was wrong. Engineers do not think in API groups. They think in questions:

- **Why did it fail?**
- **Why hasn't it run?**
- **Is it healthy right now?**
- **What changed recently?**

For every one of the 62 implemented resource types, I asked: **after an engineer finds this resource in the list, what is the NEXT thing they need to know?** The answer often lives in a completely different AWS service. A Lambda function's failure history is in CloudWatch Logs, not the Lambda API. An ECS service's crash-loop reason is in its Events list (from the ECS DescribeServices response, which is a different call than the list). An RDS instance's recent troubles show up in RDS Events (a separate API), not in the DescribeDBInstances struct.

A child view is justified when:
1. It answers "what happened?" or "why is this broken?" -- the operational question
2. The data comes from a **different API call** than what populates the parent list
3. The data is a **list of entities** (events, invocations, tasks, streams) not just more fields on the same object
4. Having this in a TUI would save an engineer from opening the Console during an incident

A child view is NOT needed when:
- The parent struct already contains the answer (e.g., EC2 has SecurityGroups[], State, StateReason inline)
- The resource is config-level and rarely inspected during operations (e.g., KMS key metadata)
- The resource is a leaf entity with no meaningful "drill deeper" (e.g., EIP, ENI)

---

## MUST-HAVE: The "What Happened?" Views

These are the child views that answer the single most important operational question for their parent resource. Without these, a9s forces engineers back to the Console at the exact moment speed matters most -- during incidents.

---

### 1. Lambda (lambda) --> Recent Invocations (from CloudWatch Logs)

**The question:** "Why is this Lambda failing? When did it last run? Is it timing out?"

The Lambda parent struct has configuration -- runtime, memory, timeout, handler, layers, VPC config. It tells you everything about HOW the function is set up. It tells you nothing about what actually happened when it ran.

- **Source of truth:** CloudWatch Logs, not the Lambda API
- **How it works:** Every Lambda writes to `/aws/lambda/{function-name}` (or a custom group from `LoggingConfig.LogGroup`). Each invocation creates a log stream entry. The REPORT line at the end of each invocation contains duration, billed duration, memory used, and whether it timed out.
- **Scenario 1 -- "Why is this Lambda failing?":** You see the function in a9s. You press Enter. You see the last 25 invocations with their status (OK/ERROR/TIMEOUT), duration, and timestamp. The one marked ERROR is 3 minutes ago. You drill into it and see the stack trace.
- **Scenario 2 -- "Has this Lambda run today?":** A scheduled Lambda should fire every hour. The invocation list shows the last run was 6 hours ago. Something is wrong with the EventBridge trigger, not the function itself.
- **Scenario 3 -- "Is this Lambda slow?":** The invocation list shows duration trending from 200ms to 8000ms over the last hour. Memory pressure or cold starts.

- **AWS API(s):**
  - `logs:FilterLogEvents` on the log group `/aws/lambda/{FunctionName}` (or `LoggingConfig.LogGroup` from the parent struct)
  - Filter pattern: `"REPORT RequestId"` to extract invocation summaries
  - Each REPORT line contains: RequestId, Duration, Billed Duration, Memory Size, Max Memory Used, Init Duration (cold start)
  - For errors: filter pattern `"ERROR"` or `"Task timed out"`
- **Columns:**
  - Timestamp
  - Request ID (short)
  - Status (OK / ERROR / TIMEOUT -- derived from REPORT + error lines)
  - Duration (ms)
  - Memory Used / Memory Size (e.g., "128/256 MB")
  - Cold Start (yes/no, from Init Duration presence)
- **Nesting:** **Yes -- 2 levels.** Invocation --> Log Lines. Press Enter on an invocation to see the full log output for that RequestId (filtered by `"RequestId: {id}"`). This is where the stack trace, the error message, the actual debugging information lives. This 3-level chain (Lambda --> Invocations --> Log Lines) is the single highest-value drill-down in the entire application.

---

### 2. ECS Services (ecs-svc) --> Service Events + Tasks

**The question:** "Why are tasks in this service failing to start or staying unhealthy?"

The ecs-svc parent struct contains `Events[]` inline (from DescribeServices), but it is buried in the YAML view as a raw array. More importantly, the actual running/stopped tasks require separate API calls and are a completely different entity type than the service.

This resource warrants **two child views**, accessible via different keys or a toggle:

#### 2a. Service Events (from DescribeServices response, Events[] field)

The Events array in the ECS service response is the **first place every engineer looks** when a service is unhealthy. It contains messages like:
- "service my-svc has reached a steady state"
- "service my-svc is unable to consistently start tasks successfully"
- "service my-svc was unable to place a task because no container instance met all of its requirements"

- **Scenario:** Your ECS service shows 0/3 running tasks. You press Enter and see the events timeline: "unable to place a task because no container instance met all of its requirements. The closest matching container instance doesn't have sufficient memory." Now you know it is a capacity problem, not a code problem.
- **AWS API:** Already in the `ecs:DescribeServices` response (`Events[]` field). Requires re-fetching the service with `include=["EVENTS"]` or simply extracting from the existing struct. The most recent 100 events are returned.
- **Columns:**
  - Timestamp (CreatedAt)
  - Message (the full event text -- this IS the data, truncated in list, full in detail)
- **Nesting:** No. Events are the answer.

#### 2b. Service Tasks

- **Scenario 1 -- Deployment verification:** You just deployed v2.3.1. You need to see: are old tasks draining? Are new tasks starting? What task definition revision are they running?
- **Scenario 2 -- Crash loop debugging:** Tasks keep starting and stopping. You need to see StoppedReason for each stopped task: OOM? Health check failure? Essential container exited?
- **AWS API(s):**
  - `ecs:ListTasks` with `serviceName` filter (returns ARNs)
  - `ecs:DescribeTasks` batch call (up to 100 ARNs)
- **Columns:**
  - Task ID (short, from ARN)
  - Last Status (RUNNING / STOPPED / PENDING)
  - Health Status
  - Task Definition (short name:revision)
  - Started At
  - Stopped Reason (if STOPPED -- this is the money column)
- **Nesting:** **Yes.** Task --> Container Details (from `Containers[]` in the DescribeTasks response). And crucially: Task --> Container Logs. The task definition's `logConfiguration` tells you the CloudWatch log group and stream prefix. The log stream name follows the pattern `{prefix}/{container-name}/{task-id}`. Pressing a key (e.g., `L`) on a task should jump to its log stream in CloudWatch Logs. This cross-service link is the difference between "nice TUI" and "indispensable incident tool."

---

### 3. CloudWatch Log Groups (logs) --> Log Streams --> Log Events

**The question:** "What happened? Show me the actual logs."

This is the foundation that makes many other child views possible. Log Groups are the gateway to all operational data in AWS.

- **Scenario 1 -- Incident response:** A Lambda or ECS task is failing. You find the log group. You need the most recent log stream to see what happened.
- **Scenario 2 -- Specific container debugging:** An ECS service has 10 replicas, all writing to the same log group but different streams (by task ID). You need to find the stream for the failing task.
- **Scenario 3 -- Batch job verification:** You need to confirm the nightly ETL job ran. Drill into its log group, find last night's stream.

- **AWS API:** `logs:DescribeLogStreams` with `orderBy=LastEventTime`, `descending=true`
- **Columns:**
  - Log Stream Name (truncated)
  - Last Event Time
  - First Event Time
  - Stored Bytes
- **Nesting:** **Yes -- this is a 3-level chain.** Log Streams --> Log Events via `logs:FilterLogEvents` or `logs:GetLogEvents`. The third level shows actual log lines. This chain (Log Groups --> Streams --> Events) is the read-only equivalent of `kubectl logs` and is the backbone of the entire cross-service log strategy.

---

### 4. CloudFormation Stacks (cfn) --> Stack Events

**The question:** "What is happening with this deployment RIGHT NOW? What failed?"

I am recommending Stack Events as the primary child view over Stack Resources, reversing the order from the previous version. Here is why: during an active deployment (the moment when you most desperately need this view), Stack Events is a live timeline of what is happening. Stack Resources is a static inventory. You need the timeline first.

- **Scenario 1 -- Active deployment monitoring:** A stack update is in progress. You want to watch resources being created/updated in real-time. "CreateComplete... CreateComplete... CREATE_FAILED -- The security group 'sg-abc' does not exist."
- **Scenario 2 -- Post-mortem:** A stack rolled back. You need to find the FIRST failed event to understand the root cause. Scrolling through events in the Console is painful.
- **AWS API:** `cloudformation:DescribeStackEvents` -- paginated, reverse chronological
- **Columns:**
  - Timestamp
  - Logical Resource ID
  - Resource Type (e.g., `AWS::EC2::SecurityGroup`)
  - Status (CREATE_COMPLETE, CREATE_FAILED, UPDATE_IN_PROGRESS, etc.)
  - Status Reason (the error message -- most important column)
- **Nesting:** No. Events are leaf-level. But the Logical Resource ID could cross-link to the actual resource in a9s if it is an implemented type.

---

### 5. CloudFormation Stacks (cfn) --> Stack Resources

**The question:** "What resources does this stack manage? Which one is stuck?"

- **Scenario 1 -- "Which resource is blocking?":** A stack has been in UPDATE_IN_PROGRESS for 40 minutes. You drill into resources and see that the CloudFront distribution is in UPDATE_IN_PROGRESS while everything else is done. That is normal -- CloudFront takes forever. You can stop worrying.
- **Scenario 2 -- Infrastructure inventory:** "What did this CDK stack create?" You see the full list of logical-to-physical resource mappings.
- **Scenario 3 -- Drift detection follow-up:** After running drift detection, you see which specific resources have drifted.
- **AWS API:** `cloudformation:ListStackResources` -- paginated
- **Columns:**
  - Logical Resource ID
  - Physical Resource ID (the actual AWS resource ID)
  - Resource Type
  - Status
  - Drift Status
  - Last Updated
- **Nesting:** Possible. A nested stack resource is itself a CloudFormation stack that could drill into its own events/resources. But this is an edge case.

**Note:** CFN stacks warrant two child views (Events and Resources). They should be accessible via different keys -- e.g., Enter for Events (the "what happened?" view) and a secondary key for Resources (the "what exists?" view).

---

### 6. Target Groups (tg) --> Target Health

**The question:** "Why is this service returning 502s? Which backends are healthy?"

- **Scenario 1 -- 502/503 investigation:** Your ALB is returning errors. The target group parent shows health check configuration (path, port, thresholds). But you need to see the ACTUAL health of each registered target right now.
- **Scenario 2 -- Deployment verification:** After a rolling deploy, confirm all new targets have passed health checks and old ones are draining.
- **Scenario 3 -- Capacity check:** "How many healthy targets are serving traffic?"
- **AWS API:** `elasticloadbalancingv2:DescribeTargetHealth`
- **Columns:**
  - Target ID (instance ID, IP, or Lambda ARN)
  - Port
  - Health State (healthy / unhealthy / draining / initial / unavailable)
  - Health Reason (if not healthy -- e.g., "Health checks failed" or "Target.FailedHealthChecks")
  - Availability Zone
- **Nesting:** No. But Target ID can cross-link to EC2 instance if it is an instance-type target group.

---

### 7. ECS Services (ecs-svc) --> Container Logs (cross-service to CloudWatch)

**The question:** "Show me the application logs for this service."

This is distinct from the Tasks child view (#2b). This view takes the shortcut directly to logs without requiring the user to go Service --> Tasks --> pick a task --> find its log stream. For the common case ("show me what this service is outputting"), you want to go straight from the service to its log group.

- **How it works:** The service's `TaskDefinition` ARN points to a task definition. The task definition's container definitions contain `logConfiguration` with `options["awslogs-group"]` and `options["awslogs-stream-prefix"]`. With these, you can call `logs:FilterLogEvents` on the log group, filtered by the stream prefix, to get recent logs across all containers.
- **AWS API(s):**
  - `ecs:DescribeTaskDefinition` (to get logConfiguration)
  - `logs:FilterLogEvents` on the extracted log group, with `logStreamNamePrefix` set to the awslogs-stream-prefix
- **Scenario:** Service is unhealthy. Instead of navigating Tasks --> pick a task --> find its log stream, you press `L` on the service and immediately see the last 100 log lines from all containers in the service, interleaved by timestamp. This is `kubectl logs -f deployment/my-service` for ECS.
- **Columns:** (This is a log viewer, not a table)
  - Timestamp
  - Stream Name (identifies which task/container)
  - Message (the log line)
- **Nesting:** No. This is already the leaf data.

---

### 8. Auto Scaling Groups (asg) --> Scaling Activities

**The question:** "Why did this ASG scale? Why DIDN'T it scale? What happened to my instances?"

The ASG parent struct has `Instances[]` showing current instances and their lifecycle state. But it does not tell you WHY instances were launched or terminated. Scaling activities are the event log for the ASG.

- **Scenario 1 -- "Why did we scale to 20 instances?":** The scaling activity says "At 2026-03-22T03:15:00Z an alarm triggered policy my-scale-out-policy changing the desired capacity from 5 to 20." Now you know it was an alarm, not manual action.
- **Scenario 2 -- "Why are instances failing to launch?":** Activities show "Launching a new EC2 instance: i-abc123. Status: Failed. Status Message: There is no Spot capacity available that matches your request." Spot capacity exhaustion.
- **Scenario 3 -- "Why hasn't this ASG scaled up despite high CPU?":** No recent scaling activities. The cooldown period or scaling policy configuration is preventing it.
- **AWS API:** `autoscaling:DescribeScalingActivities` with `AutoScalingGroupName` filter -- paginated, newest first
- **Columns:**
  - Start Time
  - Status (Successful / Failed / InProgress / Cancelled)
  - Description (e.g., "Launching a new EC2 instance: i-abc123")
  - Cause (the trigger -- alarm name, scheduled action, manual, etc.)
  - Status Message (error message if failed)
  - End Time
- **Nesting:** No. Activities are leaf-level.

---

### 9. Load Balancers (elb) --> Listeners

**The question:** "How is traffic being routed? Why is HTTPS not working?"

- **Scenario 1 -- Routing debugging:** "Traffic on port 443 should go to target group X but it is going to Y." You need to see listener configurations -- port/protocol/default action.
- **Scenario 2 -- Security audit:** "Does this ALB redirect HTTP to HTTPS?" Check listeners for port 80 redirect action.
- **Scenario 3 -- Certificate troubleshooting:** "Which certificate is the HTTPS listener using?" The listener shows the ACM certificate ARN.
- **AWS API:** `elasticloadbalancingv2:DescribeListeners` with `LoadBalancerArn`
- **Columns:**
  - Port
  - Protocol (HTTP / HTTPS / TCP / TLS)
  - Default Action Type (forward / redirect / fixed-response)
  - Default Action Target (target group name or redirect config)
  - SSL Policy
  - Certificate ARN (truncated)
- **Nesting:** **Yes.** Listener --> Rules via `elasticloadbalancingv2:DescribeRules`. Each listener can have complex path-based or host-based routing rules. Essential for debugging "why is /api/v2 going to the wrong service?"

---

### 10. CloudWatch Alarms (alarm) --> Alarm History

**The question:** "When did this alarm fire? How often? Is it flapping?"

The alarm parent struct shows the CURRENT state (OK / ALARM / INSUFFICIENT_DATA) with StateReason and StateTransitionedTimestamp. But it shows only the most recent transition. During an incident or post-mortem, you need the full history: when did it go to ALARM, when did it recover, and is it flapping?

- **Scenario 1 -- Post-mortem:** "When exactly did the outage start?" The alarm history shows the first OK-->ALARM transition at 02:47 AM.
- **Scenario 2 -- Flapping detection:** The history shows 8 ALARM-->OK-->ALARM transitions in the last hour. The threshold is too tight or the metric is noisy.
- **Scenario 3 -- "Did anyone acknowledge this?":** History shows action executions (SNS notifications sent, Lambda triggered).
- **AWS API:** `cloudwatch:DescribeAlarmHistory` with `AlarmName` -- paginated, preserves 30 days of history
- **Columns:**
  - Timestamp
  - History Item Type (StateUpdate / ConfigurationUpdate / Action)
  - Summary (e.g., "Alarm updated from OK to ALARM")
  - Old State --> New State
- **Nesting:** No. History is leaf-level.

---

## SHOULD-HAVE: The Investigation Views

These child views support common investigation workflows -- not the first thing you check, but the second or third. They answer "what runs here?" and "what is connected?" rather than "what broke?"

---

### 11. Step Functions (sfn) --> Executions

**The question:** "Did the workflow run? Did it succeed? Where did it fail?"

The SFN parent struct is extremely sparse (`StateMachineListItem` -- just name, ARN, type, creation date). Without executions, this resource type is nearly useless in a9s.

- **Scenario 1 -- "Did the nightly ETL run?":** You see the state machine. You press Enter. The most recent execution shows SUCCEEDED at 03:22 AM. Done.
- **Scenario 2 -- "The workflow failed -- which step?":** The execution list shows one FAILED. You drill into it to see the execution history.
- **AWS API:** `states:ListExecutions` with `stateMachineArn` -- paginated, filterable by status
- **Columns:**
  - Execution Name
  - Status (RUNNING / SUCCEEDED / FAILED / TIMED_OUT / ABORTED)
  - Start Date
  - Stop Date
  - Duration (computed)
- **Nesting:** **Yes.** Execution --> Execution History via `states:GetExecutionHistory`. Shows step-by-step state transitions: which state entered, which state exited, which state failed, and the error/cause. This is the equivalent of the Step Functions Console visual execution graph, but as a timeline. Not supported for EXPRESS state machines.

---

### 12. CodeBuild Projects (cb) --> Builds + Build Logs (cross-service)

**The question:** "Is the build passing? What broke?"

- **Scenario 1 -- "Is CI green?":** The parent shows project config (source, environment, service role). You need recent builds with pass/fail status.
- **Scenario 2 -- "What broke the build?":** A build failed. The build detail shows the phase that failed, but the actual error is in the **build logs** which are in CloudWatch Logs at the group/stream specified in the build's `logs.groupName` and `logs.streamName` fields.
- **AWS API(s):**
  - `codebuild:ListBuildsForProject` (returns build IDs, newest first)
  - `codebuild:BatchGetBuilds` (up to 100 IDs -- returns full build details including `logs.groupName`, `logs.streamName`, and `logs.deepLink`)
- **Columns:**
  - Build Number
  - Status (SUCCEEDED / FAILED / IN_PROGRESS / STOPPED)
  - Start Time
  - Duration
  - Source Version (commit SHA, truncated)
  - Initiator (who/what triggered it)
- **Nesting:** **Yes.** Build --> Build Logs. The `logs.groupName` and `logs.streamName` fields in the BatchGetBuilds response point directly to the CloudWatch log stream containing the full build output. Use `logs:GetLogEvents` with that group + stream to show the build log. This is the "why did it fail?" answer. Cross-service link, same pattern as Lambda invocations.

---

### 13. CodePipeline (pipeline) --> Pipeline Stage State

**The question:** "Where is my deploy? What stage is it stuck on?"

- **Scenario 1 -- "Has my change reached production?":** The parent shows pipeline name and last update. You need to see: Source passed, Build passed, Deploy to staging passed, Approval... waiting. Ah, the deploy is waiting for manual approval.
- **Scenario 2 -- "What broke the pipeline?":** The Deploy stage shows FAILED. The action status reveals the CodeDeploy action failed with a specific error.
- **AWS API:** `codepipeline:GetPipelineState` -- returns current state of all stages and actions in a single call (not paginated)
- **Columns:**
  - Stage Name
  - Stage Status (InProgress / Succeeded / Failed)
  - Action Name (within stage)
  - Action Status
  - Last Status Change
  - External Execution URL (link to CodeBuild project, CodeDeploy deployment, etc.)
- **Nesting:** Possible. Stage action --> linked resource (e.g., the CodeBuild build that ran for this action). But GetPipelineState gives enough detail in one call.

---

### 14. ECR Repositories (ecr) --> Images

**The question:** "Which image tags exist? When were they pushed? Is the right version deployed?"

- **Scenario 1 -- "Did CI push the image?":** You just merged to main. Check the ECR repo: is `v2.3.1` there? When was it pushed? 2 minutes ago. Good.
- **Scenario 2 -- "Why is this image so large?":** Drill in and see image sizes. The `:latest` tag is 2.4 GB -- someone added a huge dependency.
- **Scenario 3 -- "Are there vulnerabilities?":** The image list shows scan status and finding severity counts for each image.
- **AWS API:** `ecr:DescribeImages` -- paginated
- **Columns:**
  - Image Tag(s)
  - Image Digest (short prefix)
  - Pushed At
  - Image Size
  - Scan Status
  - Finding Severity Counts (Critical/High/Medium if scanning enabled)
- **Nesting:** No. Images are leaf-level.

---

### 15. RDS Instances (dbi) --> Recent Events (cross-service to RDS Events API)

**The question:** "What happened to this database recently? Was there a failover? A maintenance window? A reboot?"

The RDS parent struct is enormous (150+ fields) and has all current configuration. But it tells you nothing about what HAPPENED -- did it reboot? Was there an automated failover? Did a maintenance window apply a patch? Did storage autoscaling kick in?

- **Scenario 1 -- "Did the database fail over?":** The events show "Multi-AZ instance failover started" and "Failover complete" with timestamps. Now you know the 30-second blip at 3 AM was an RDS failover, not an application bug.
- **Scenario 2 -- "Was there maintenance last night?":** Events show "Applying maintenance, DB instance will be rebooted" -- that explains the brief outage.
- **Scenario 3 -- "Did storage autoscaling happen?":** Events show "Allocating additional storage" -- the 50% I/O drop was storage expansion.
- **AWS API:** `rds:DescribeEvents` with `SourceIdentifier` = DB instance ID, `SourceType` = `db-instance` -- returns events for up to 14 days
- **Columns:**
  - Timestamp (Date)
  - Event Category (availability, failover, maintenance, notification, etc.)
  - Message
  - Source Type
- **Nesting:** No. Events are leaf-level.

---

### 16. IAM Roles (role) --> Attached Policies

**The question:** "What permissions does this role have?"

- **Scenario 1 -- "Why is this Lambda getting AccessDenied?":** Find the execution role, drill in, see attached policies. It has AmazonS3ReadOnlyAccess but not the SQS policy it needs.
- **Scenario 2 -- "Is this role over-permissioned?":** The role has AdministratorAccess attached. During a security audit, that is a finding.
- **AWS API(s):**
  - `iam:ListAttachedRolePolicies` (managed policies)
  - `iam:ListRolePolicies` (inline policy names)
- **Columns:**
  - Policy Name
  - Policy ARN (for managed policies, truncated)
  - Type (Managed / Inline)
- **Nesting:** Possibly. Policy --> Policy Document JSON. But the YAML view of the policy resource itself is likely sufficient for a read-only tool.

---

### 17. SNS Topics (sns) --> Subscriptions

**The question:** "Who receives notifications from this topic?"

The SNS parent struct is `TopicArn` and nothing else. It is the sparsest resource in the entire application. Without subscriptions, the SNS resource type is borderline useless.

- **Scenario 1 -- "Why isn't the alarm reaching Slack?":** Check the topic's subscriptions. The HTTPS endpoint for the Slack webhook is there, but its subscription is "PendingConfirmation." It was never confirmed.
- **Scenario 2 -- "Who gets paged for this?":** The critical-alerts topic has 3 email subscriptions and 1 Lambda. You can see exactly who is in the notification chain.
- **AWS API:** `sns:ListSubscriptionsByTopic` -- paginated
- **Columns:**
  - Protocol (email / https / sqs / lambda / sms)
  - Endpoint (email address, URL, queue ARN, function ARN)
  - Subscription ARN (indicates PendingConfirmation vs confirmed)
  - Owner
- **Nesting:** No. Subscriptions are leaf-level.

---

### 18. EventBridge Rules (eb-rule) --> Targets

**The question:** "What does this rule trigger?"

The eb-rule parent struct has the event pattern or schedule expression, but NOT what happens when the rule matches. The targets are a separate entity.

- **Scenario 1 -- "This scheduled rule should trigger a Lambda, but it isn't. Is the target still attached?":** Drill in and see the target ARN. It points to a Lambda that was deleted. Mystery solved.
- **Scenario 2 -- "What does this cron expression trigger?":** You see `cron(0 8 * * ? *)` on the rule. You press Enter and see: Target = Lambda ARN `data-pipeline-daily`, Input = `{"mode": "full"}`.
- **AWS API:** `events:ListTargetsByRule` -- up to 100 targets per rule
- **Columns:**
  - Target ID
  - Target ARN (with resource type extracted for readability: "Lambda: data-pipeline-daily")
  - Input (JSON override, truncated)
  - Role ARN (truncated)
- **Nesting:** No. Targets are leaf-level.

---

### 19. Glue Jobs (glue) --> Job Runs

**The question:** "Did the ETL job run? How long did it take? Why did it fail?"

Like Step Functions, the Glue parent struct is all about configuration (script location, worker type, connections). The runs are where operational reality lives.

- **Scenario 1 -- "Did last night's ETL complete?":** Most recent run shows SUCCEEDED at 04:15 AM, execution time 47 minutes. Good.
- **Scenario 2 -- "Why did it fail?":** The error message on the failed run says "java.lang.OutOfMemoryError: GC overhead limit exceeded." Need more workers or G.2X instances.
- **AWS API:** `glue:GetJobRuns` with `JobName` -- paginated, newest first
- **Columns:**
  - Run ID (short)
  - State (STARTING / RUNNING / SUCCEEDED / FAILED / TIMEOUT / ERROR / STOPPED)
  - Started On
  - Execution Time (seconds)
  - Error Message (if failed -- the critical column)
  - DPU Hours (cost proxy)
- **Nesting:** No. For detailed logs, the Glue job's `LogUri` or default log group `/aws-glue/jobs/output` can be followed via the Log Groups child view.

---

## SHOULD-HAVE: Structural Drill-Downs

These are child views that reveal structure rather than operational state. They help with "what is connected?" and "how is this configured?" rather than "what happened?" Still valuable, but reach-for-weekly rather than reach-for-daily.

---

### 20. Load Balancers (elb) --> Listener Rules

This is the nested child of Listeners (#9). After seeing the listeners, you often need to see the per-path routing rules.

- **Scenario:** "Why is `/api/v2` returning 404?" You drill from the ALB to its HTTPS listener, then to its rules. Rule 3 forwards `/api/v1/*` to target-group-v1, Rule 4 forwards `/api/v2/*` to target-group-v2. But target-group-v2 has 0 healthy targets. The chain: ELB --> Listeners --> Rules --> (cross-link to TG --> Target Health).
- **AWS API:** `elasticloadbalancingv2:DescribeRules` with `ListenerArn`
- **Columns:**
  - Priority
  - Conditions (host-header, path-pattern)
  - Action Type (forward / redirect / fixed-response)
  - Action Target (target group name)
- **Nesting:** No. Rules are leaf-level. But Action Target can cross-link to Target Group.

---

### 21. IAM Groups (iam-group) --> Group Members

**The question:** "Who is in this group?"

- **Scenario:** Security audit: "Who has admin access?" Find the Admins group, drill in, see the 4 users. One of them left 6 months ago. Finding.
- **AWS API:** `iam:GetGroup` with `GroupName` -- paginated, returns users
- **Columns:**
  - User Name
  - User ID
  - Created Date
  - Password Last Used
- **Nesting:** No.

---

### 22. EC2 Instances (ec2) --> Instance Status Checks (cross-service)

**The question:** "Is this instance reachable? Are there hardware issues?"

The EC2 parent struct has `State.Name` (running/stopped) but NOT the status check results. An instance can be in `running` state but failing system or instance reachability checks.

- **Scenario:** "This instance is running but the application is unreachable." You check instance status: System Reachability = FAILED. It is a host-level issue -- AWS needs to migrate the instance.
- **AWS API:** `ec2:DescribeInstanceStatus` with `InstanceIds` -- returns system status and instance status
- **Implementation note:** This is less of a "list of children" and more of "extra detail data." It might work better as an enrichment to the detail view rather than a child list view. Consider showing status checks in the detail view directly. If not implementable as a detail enrichment, skip this one -- it is borderline.

---

### 23. SFN Executions --> Execution History (nested child of #11)

**The question:** "Which step failed? What was the error?"

- **Scenario:** A Step Function execution shows FAILED. You drill in and see the history: StateEntered "ProcessOrder" --> TaskFailed with error "States.TaskFailed" and cause "Lambda function returned error: OrderNotFound."
- **AWS API:** `states:GetExecutionHistory` with `executionArn` -- paginated, returns events in order
- **Columns:**
  - Timestamp
  - Event Type (TaskStateEntered, TaskSucceeded, TaskFailed, etc.)
  - State Name (which step)
  - Error (if failed)
  - Cause (if failed)
- **Nesting:** No. History events are leaf-level.
- **Note:** Not supported for EXPRESS state machines.

---

## OPTIONAL

Resources where a child view has some value but low frequency of need. Brief justification only.

- **ECS Clusters (ecs) --> Services:** The `ecs-svc` resource already exists at top level. A child view would be a filtered version. Low incremental value unless cross-resource navigation from cluster to its services is needed.
- **EKS Clusters (eks) --> Node Groups:** Same -- `ng` exists at top level. A child view is just filtering.
- **API Gateway (apigw) --> Routes/Stages:** Useful for debugging API routing, but APIGW is infrequent and the Console is adequate. APIs: `apigatewayv2:GetRoutes`, `apigatewayv2:GetStages`.
- **Security Groups (sg) --> Rules (flattened):** The parent struct already has `IpPermissions[]` and `IpPermissionsEgress[]` in the YAML view. A flat table of rules (via `ec2:DescribeSecurityGroupRules`) would be easier to scan during security audits, but the data is already accessible.
- **RDS Instances (dbi) --> DB Log Files:** `rds:DescribeDBLogFiles` lists slow query logs, error logs, etc. High value for database debugging, but the log files themselves require `rds:DownloadDBLogFilePortion` which returns raw text in chunks -- awkward for a TUI. Consider only if the log viewer from the CloudWatch child view (#3) works well.
- **IAM Policies (policy) --> Attached Entities:** "Which roles/users/groups use this policy?" via `iam:ListEntitiesForPolicy`. Useful during security reviews but infrequent.
- **DynamoDB Tables (ddb) --> Stream Records:** If DynamoDB Streams is enabled, `dynamodbstreams:DescribeStream` + `GetShardIterator` + `GetRecords` could show recent change events. But stream consumption is complex and stateful -- not ideal for a TUI.
- **WAF (waf) --> Rules:** `wafv2:GetWebACL` returns the full rule set. Useful for security audits but the YAML detail view is likely sufficient for the full WebACL JSON.
- **CloudFront (cf) --> Invalidations:** `cloudfront:ListInvalidations` shows recent cache invalidation requests and their status. Useful during deploys but infrequent.

---

## NOT NEEDED

Resources where the existing detail/YAML view is sufficient, or where no meaningful operational child entity exists.

### COMPUTE
- **EC2 (ec2):** Parent struct is 170+ fields including SecurityGroups[], NetworkInterfaces[], BlockDeviceMappings[], State, StateReason. The operational data (status checks) is covered in SHOULD-HAVE #22. No separate list entity.
- **Lambda (lambda):** Covered by MUST-HAVE #1 (invocations via CloudWatch Logs). No additional Lambda API child needed.
- **Elastic Beanstalk (eb):** Health, status, platform info are in parent. `elasticbeanstalk:DescribeEvents` exists but EB is legacy. Low priority.
- **ECS Tasks (ecs-task):** Parent struct already has `Containers[]` with detailed status, exit codes, network info, and `StoppedReason`. A containers sub-view is covered as nesting under ecs-svc Tasks (#2b).

### CONTAINERS
- **EKS (eks):** Node groups exist as top-level resource. Cluster add-ons and Fargate profiles are low frequency.
- **Node Groups (ng):** Parent struct has scaling config, instance types, health issues, labels. No operational child entity.

### NETWORKING
- **Security Groups (sg):** See OPTIONAL. Rules already in `IpPermissions[]`.
- **VPC (vpc):** Subnets, route tables, SGs exist as separate resources. No child entity.
- **Subnet (subnet):** Parent has CIDR, AZ, available IPs. No child entity.
- **Route Tables (rtb):** Parent already has `Routes[]` and `Associations[]`. No additional API needed.
- **NAT Gateways (nat):** Parent has addresses, state, failure info. No child entity.
- **Internet Gateways (igw):** Minimal data, no child entity.
- **VPC Endpoints (vpce):** Parent has DNS entries, route tables, subnets, SGs. No child entity.
- **Transit Gateways (tgw):** Attachments could be a child but TGWs are IaC-managed. Low value.
- **ENI (eni):** Parent has IPs, SGs, attachment. No child entity.
- **EIP (eip):** Simple resource. No child entity.

### DATABASES & STORAGE
- **S3 (s3):** Already has child view (S3 Objects). Done.
- **RDS Instances (dbi):** Covered by SHOULD-HAVE #15 (RDS Events). The parent struct handles everything else.
- **Redis (redis):** Parent has cache nodes with endpoints, parameter groups, SGs. No operational child entity. Slow logs would require `elasticache:DescribeSlowLog` but this is very niche.
- **DocumentDB (dbc):** Parent has cluster members with writer status. No child entity.
- **DynamoDB (ddb):** See OPTIONAL for stream records. Scanning items is dangerous in a TUI.
- **OpenSearch (opensearch):** Parent is 100+ fields. No child entity.
- **Redshift (redshift):** Parent has cluster nodes, endpoints, VPC info. No child entity.
- **EFS (efs):** Mount targets could be a child but EFS is infrequently inspected. Low value.
- **RDS Snapshots (rds-snap):** Leaf resource. No child entity.
- **DocumentDB Snapshots (docdb-snap):** Leaf resource. No child entity.

### MONITORING
- **CloudTrail (trail):** Trail configuration is in parent. Trail events (`cloudtrail:LookupEvents`) have enormous volume and require filtering. Better served by CloudWatch Logs Insights or Athena.

### MESSAGING
- **SQS (sqs):** Queue attributes (message count, DLQ config) in parent. `sqs:ReceiveMessage` consumes messages -- not suitable for read-only browsing.
- **SNS Subscriptions (sns-sub):** This IS the child entity type. No further drill-down.
- **Kinesis (kinesis):** Shard details via `kinesis:ListShards` could be useful but shards are rarely inspected. Data records require iterator management unsuitable for TUI.
- **MSK (msk):** Topics are a Kafka protocol operation, not an AWS API. No suitable child view.

### SECRETS & CONFIG
- **Secrets Manager (secrets):** Secret value is fetched via GetSecretValue, already handled by the detail view's "reveal" feature. No child entity.
- **SSM Parameters (ssm):** Value is fetched separately, detail view concern. Parameter history (`ssm:GetParameterHistory`) could be a child but very low frequency.
- **KMS (kms):** Key metadata, grants, policies -- rarely inspected in a TUI.

### DNS & CDN
- **Route 53 (r53):** Already has child view (R53 Records). Done.
- **CloudFront (cf):** See OPTIONAL for invalidations. Parent struct is huge with origins, cache behaviors, etc.
- **ACM (acm):** Certificate details are in parent. No child entity.

### SECURITY & IAM
- **IAM Users (iam-user):** Access keys, MFA devices could be children for security audits, but this is infrequent. Better handled by IAM Access Analyzer.
- **WAF (waf):** See OPTIONAL.

### CI/CD
- **CloudFormation (cfn):** Covered by MUST-HAVE #4 and #5 (Stack Events and Stack Resources).
- **CodePipeline (pipeline):** Covered by SHOULD-HAVE #13 (Pipeline State).
- **CodeBuild (cb):** Covered by SHOULD-HAVE #12 (Builds).
- **ECR (ecr):** Covered by SHOULD-HAVE #14 (Images).
- **CodeArtifact (codeartifact):** Package listing via `codeartifact:ListPackages` is possible but CodeArtifact is rarely inspected interactively.

### DATA & ANALYTICS
- **Glue (glue):** Covered by SHOULD-HAVE #19 (Job Runs).
- **Athena (athena):** Athena is used via query editor, not resource browser. Low value.

### BACKUP & EMAIL
- **Backup (backup):** Backup jobs/recovery points are infrequent. Low value.
- **SES (ses):** Sparse parent, but SES identities are rarely inspected interactively.

---

## Summary

| Tier | Count | Child Views |
|------|-------|-------------|
| MUST-HAVE | 10 | lambda-->invocations(CW), ecs-svc-->events, ecs-svc-->tasks, logs-->streams-->events, cfn-->events, cfn-->resources, tg-->health, ecs-svc-->logs(CW), asg-->activities, elb-->listeners |
| SHOULD-HAVE | 9 | sfn-->executions, cb-->builds+logs(CW), pipeline-->state, ecr-->images, dbi-->events(RDS), role-->policies, sns-->subscriptions, eb-rule-->targets, glue-->runs |
| Structural/Nested | 3 | elb-->listeners-->rules, sfn-->executions-->history, alarm-->history |
| OPTIONAL | 9 | ecs-->services, eks-->ngs, apigw-->routes, sg-->rules, dbi-->log-files, policy-->entities, ddb-->streams, waf-->rules, cf-->invalidations |
| NOT NEEDED | 40 | Everything else |

---

## Recommended Implementation Order

The order is driven by **incident value** -- what would have helped me most at 2 AM:

### Phase 1: "Show me what happened" (highest incident value)

1. **`logs` --> Log Streams --> Log Events** -- Foundation. Other cross-service views depend on the log viewer existing.
2. **`lambda` --> Recent Invocations (from CW Logs)** -- Highest single-resource value. Lambda is the most common serverless compute and the Console log experience is terrible.
3. **`ecs-svc` --> Service Events** -- First thing to check when ECS service is unhealthy. Data is already in the DescribeServices response.
4. **`ecs-svc` --> Tasks** -- Second thing to check. Requires additional API calls.
5. **`tg` --> Target Health** -- "Why is the ALB returning 502?"
6. **`cfn` --> Stack Events** -- "What failed in this deployment?"

### Phase 2: "Show me the timeline" (deployment and debugging workflows)

7. **`cfn` --> Stack Resources** -- "What is stuck?"
8. **`asg` --> Scaling Activities** -- "Why did we scale / not scale?"
9. **`alarm` --> Alarm History** -- "When did this fire? Is it flapping?"
10. **`elb` --> Listeners** -- "How is traffic routed?"
11. **`ecs-svc` --> Container Logs (CW cross-service)** -- "Show me the application output."

### Phase 3: "Show me the context" (CI/CD and investigation)

12. **`sfn` --> Executions** -- "Did the workflow run?"
13. **`cb` --> Builds** -- "Is CI passing?"
14. **`pipeline` --> Pipeline State** -- "Where is my deploy?"
15. **`ecr` --> Images** -- "What tags exist?"
16. **`dbi` --> RDS Events** -- "Was there a failover?"

### Phase 4: "Show me the structure" (security and audit)

17. **`role` --> Policies** -- "What permissions does this role have?"
18. **`sns` --> Subscriptions** -- "Who gets notified?"
19. **`eb-rule` --> Targets** -- "What does this rule trigger?"
20. **`glue` --> Job Runs** -- "Did the ETL run?"
21. **`iam-group` --> Members** -- "Who is in this group?"
22. **`elb` --> Listeners --> Rules** -- "How is path routing configured?"
23. **`sfn` --> Executions --> History** -- "Which step failed?"

---

## Cross-Service Patterns

A key insight from this analysis: **the most valuable child views are cross-service**. They follow the engineer's investigation path, not the AWS API grouping.

| Parent Resource | Cross-Service Child | Source AWS Service | Key Benefit |
|---|---|---|---|
| Lambda | Recent Invocations | CloudWatch Logs | Error + duration history |
| ECS Service | Container Logs | CloudWatch Logs (via task def logConfiguration) | Application output |
| CodeBuild | Build Logs | CloudWatch Logs (via build logs.groupName) | Build failure details |
| RDS Instance | Recent Events | RDS Events API (separate from DescribeDBInstances) | Failover/maintenance history |
| CloudWatch Alarm | State History | CloudWatch Alarm History API (separate from DescribeAlarms) | Flapping detection, timing |
| ASG | Scaling Activities | Auto Scaling Activities API (separate from DescribeASGs) | Scale event reasons |

The implementation pattern for all of these is similar:
1. Extract an identifier from the parent resource (function name, service ARN, DB instance ID, etc.)
2. Call a different AWS API with that identifier
3. Display the results as a child list view

This argues for building the cross-service fetcher pattern as a reusable component rather than special-casing each one.

---

## Nesting Chains (multi-level drill-downs)

These are the highest-value multi-level paths through the data, where each level answers a progressively deeper question:

1. **Log Groups --> Log Streams --> Log Events** (3 levels) -- From "which group?" to "which stream?" to "what happened?" The backbone of all cross-service log viewing.
2. **Lambda --> Invocations --> Log Lines** (3 levels) -- From "which function?" to "which invocation failed?" to "what was the error message?"
3. **ECS Services --> Tasks --> Container Logs** (3 levels + cross-service) -- From "which service?" to "which task?" to "what is it outputting?"
4. **CloudFormation --> Events (timeline)** + **CloudFormation --> Resources (inventory)** (2 parallel children) -- Both children answer different questions from the same parent.
5. **Step Functions --> Executions --> Execution History** (3 levels) -- From "which state machine?" to "which run?" to "which step failed?"
6. **Load Balancers --> Listeners --> Rules** (3 levels) -- From "which ALB?" to "which port?" to "which path goes where?"
7. **CodeBuild --> Builds --> Build Logs** (3 levels + cross-service) -- From "which project?" to "which build?" to "what was the error output?"
