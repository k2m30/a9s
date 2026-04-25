# QA User Stories: Negative / Error Case Testing

Scope: comprehensive coverage of error conditions, edge cases, and negative paths across all resource types and views. All stories treat a9s as a black box. Addresses GitHub issue #67.

---

## A. AWS Connection & Network Errors

### Story A.1: Expired AWS credentials show error flash on resource list load

**Given:** The user has expired AWS credentials configured in their profile.
**When:** The user selects any resource type (e.g., EC2 Instances) from the main menu.
**Then:** The loading spinner appears briefly, then the header right side shows a red error flash (e.g., "Error: ExpiredToken") in bold red (`#f7768e`). No rows are displayed in the resource list. The application does not crash.

**AWS comparison:**
aws ec2 describe-instances
Expected error: ExpiredTokenException: The security token included in the request is expired

### Story A.2: Missing AWS credentials show error flash on resource list load

**Given:** The user has no AWS credentials configured (no `~/.aws/credentials`, no environment variables, no IAM role).
**When:** The user selects any resource type from the main menu.
**Then:** The header right side shows a red error flash indicating missing credentials. The resource list frame is displayed but contains no data rows. The application does not crash or panic.

**AWS comparison:**
aws sts get-caller-identity
Expected error: Unable to locate credentials

### Story A.3: STS AssumeRole failure shows error flash

**Given:** The user's AWS profile is configured to assume a role, but the role does not exist or the user is not authorized to assume it.
**When:** The user selects any resource type from the main menu.
**Then:** The header right side shows a red error flash (e.g., "Error: AccessDenied" or "Error: cannot assume role"). The resource list is empty. The application remains responsive and navigable.

**AWS comparison:**
aws sts assume-role --role-arn arn:aws:iam::123456789012:role/nonexistent --role-session-name test
Expected error: AccessDenied or NoSuchEntity

### Story A.4: Network timeout shows error flash

**Given:** The user's network is unreachable or the AWS endpoint is unresolvable (e.g., firewall blocking, DNS failure).
**When:** The user selects any resource type from the main menu.
**Then:** After a timeout period, the header right side shows a red error flash (e.g., "Error: request timeout" or "Error: network unreachable"). The loading spinner stops. The application remains responsive.

**AWS comparison:**
aws ec2 describe-instances --endpoint-url <https://nonexistent.example.com>
Expected error: Could not connect to the endpoint URL

### Story A.5: Invalid region shows error flash

**Given:** The user has configured a non-existent or disabled region (e.g., "us-west-99").
**When:** The user selects any resource type from the main menu.
**Then:** The header right side shows a red error flash indicating the region is not valid. The resource list is empty. The header still shows the invalid region in the profile:region display.

**AWS comparison:**
aws ec2 describe-instances --region us-west-99
Expected error: Could not connect to the endpoint URL

### Story A.6: API throttling shows error flash

**Given:** The user's AWS account is being rate-limited by the service (HTTP 429 / ThrottlingException).
**When:** The user attempts to load a resource list or refresh it with `ctrl+r`.
**Then:** The header right side shows a red error flash (e.g., "Error: Throttling" or "Error: rate exceeded"). The application does not retry indefinitely or become unresponsive.

**AWS comparison:**
aws ec2 describe-instances (when rate limited)
Expected error: RequestLimitExceeded: Request limit exceeded

### Story A.7: Service unavailable (503) shows error flash

**Given:** An AWS service is experiencing an outage or maintenance and returns HTTP 503.
**When:** The user selects the corresponding resource type from the main menu.
**Then:** The header right side shows a red error flash (e.g., "Error: ServiceUnavailable"). The resource list frame is displayed with no rows. The user can navigate back to the main menu with `esc`.

**AWS comparison:**
aws s3 ls (during S3 outage)
Expected error: ServiceUnavailable: Service is not available

### Story A.8: Error flash persists until navigation

**Given:** A resource list has failed to load due to any API error (categories A.1--A.7).
**When:** The user views the error in the header.
**Then:** The error flash message remains visible in the header right side and does not auto-clear after 2 seconds (unlike success flashes). The error persists until the user navigates away with `esc` or performs another action.

### Story A.9: Refresh after error retries the fetch

**Given:** A resource list previously failed to load due to a transient error (e.g., throttling, timeout).
**When:** The user presses `ctrl+r` to refresh.
**Then:** The loading spinner appears again as the application re-fetches from AWS. If the transient condition has cleared, the resources load successfully.

**AWS comparison:**
aws ec2 describe-instances (retry after throttle clears)

### Story A.10: Error on one resource type does not affect others

**Given:** The user loaded EC2 Instances and received an error (e.g., AccessDenied).
**When:** The user presses `esc` to return to the main menu, then selects S3 Buckets.
**Then:** S3 Buckets loads independently. The prior EC2 error does not prevent other resource types from loading.

---

## B. IAM & Permission Errors

### Story B.1: AccessDenied on list call shows error in header

**Given:** The user's IAM identity does not have permission to list a given resource type (e.g., missing `ec2:DescribeInstances`).
**When:** The user selects that resource type from the main menu.
**Then:** The header right side shows a red error flash containing "AccessDenied" or "UnauthorizedOperation". The resource list is empty. The application does not crash.

**AWS comparison:**
aws ec2 describe-instances (without ec2:DescribeInstances permission)
Expected error: UnauthorizedOperation

### Story B.2: AccessDenied on describe call shows error in detail view

**Given:** The user can list resources but cannot describe individual resources (e.g., has `s3:ListAllMyBuckets` but not `s3:GetBucketLocation`).
**When:** The user selects a resource from the list and presses `d` or `Enter` to view details.
**Then:** The detail view shows an error message or the header shows a red error flash. The application does not crash. The user can press `esc` to return to the list.

**AWS comparison:**
aws s3api get-bucket-location --bucket my-bucket (without s3:GetBucketLocation)
Expected error: AccessDenied

### Story B.3: AccessDenied on S3 object listing in child view

**Given:** The user can list S3 buckets but cannot list objects in a specific bucket (missing `s3:ListBucket` on that bucket's policy).
**When:** The user selects an S3 bucket from the bucket list and presses `Enter` to view objects.
**Then:** The S3 objects child view shows an error. The header right side displays a red error flash (e.g., "Error: AccessDenied"). The user can press `esc` to return to the bucket list.

**AWS comparison:**
aws s3 ls s3://restricted-bucket/
Expected error: AccessDenied
Expected fields visible: (none -- error state)

### Story B.4: AccessDenied on child-view fetch for ECS service events

**Given:** The user can list ECS services but lacks permission for the describe call that fetches service events.
**When:** The user selects an ECS service and presses `e` to open Service Events.
**Then:** The child view shows an error. The header right side displays a red error flash. The user can press `esc` to return to the ECS services list.

**AWS comparison:**
aws ecs describe-services --cluster my-cluster --services my-svc (without ecs:DescribeServices)
Expected error: AccessDeniedException

### Story B.5: AccessDenied on child-view fetch for ECS tasks

**Given:** The user can list ECS services but lacks `ecs:ListTasks` or `ecs:DescribeTasks` permissions.
**When:** The user selects an ECS service and presses `Enter` to open Tasks.
**Then:** The child view shows an error in the header. The user can press `esc` to return.

**AWS comparison:**
aws ecs list-tasks --cluster my-cluster --service-name my-svc
Expected error: AccessDeniedException

### Story B.6: AccessDenied on child-view fetch for CloudWatch log streams

**Given:** The user can list log groups but lacks `logs:DescribeLogStreams` permission.
**When:** The user selects a log group and presses `Enter` to view log streams.
**Then:** The log streams child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws logs describe-log-streams --log-group-name /my/group
Expected error: AccessDeniedException

### Story B.7: AccessDenied on child-view fetch for CloudWatch log events

**Given:** The user can list log streams but lacks `logs:GetLogEvents` permission.
**When:** The user selects a log stream and presses `Enter` to view log events.
**Then:** The log events child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws logs get-log-events --log-group-name /my/group --log-stream-name my-stream
Expected error: AccessDeniedException

### Story B.8: KMS decrypt permission denied for Secrets Manager reveal

**Given:** The user can list secrets and even describe them but lacks `kms:Decrypt` permission on the KMS key encrypting the secret value.
**When:** The user selects a secret and presses `x` to reveal the secret value.
**Then:** The reveal view shows an error (e.g., "Error: AccessDenied -- unable to decrypt secret") rather than the secret value. The header shows a red error flash. The user can press `esc` to return.

**AWS comparison:**
aws secretsmanager get-secret-value --secret-id prod/api/key
Expected error: AccessDeniedException: Access to KMS is not allowed

### Story B.9: KMS decrypt permission denied for SSM SecureString reveal

**Given:** The user can list SSM parameters but lacks `kms:Decrypt` permission on the KMS key for a SecureString parameter.
**When:** The user selects an SSM SecureString parameter and presses `x` to reveal its value.
**Then:** The reveal view shows an error rather than the decrypted value. The user can press `esc` to return.

**AWS comparison:**
aws ssm get-parameter --name /prod/db/password --with-decryption
Expected error: AccessDeniedException

### Story B.10: AccessDenied on Lambda invocations child view

**Given:** The user can list Lambda functions but lacks `logs:FilterLogEvents` permission needed to parse invocation data from CloudWatch Logs.
**When:** The user selects a Lambda function and presses `Enter` to view invocations.
**Then:** The invocations child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws logs filter-log-events --log-group-name /aws/lambda/my-function --filter-pattern "REPORT"
Expected error: AccessDeniedException

### Story B.11: AccessDenied on CFN stack events child view

**Given:** The user can list CloudFormation stacks but lacks `cloudformation:DescribeStackEvents` permission.
**When:** The user selects a stack and presses `Enter` to view stack events.
**Then:** The child view shows an error. The user can press `esc` to return.

**AWS comparison:**
aws cloudformation describe-stack-events --stack-name my-stack
Expected error: AccessDeniedException

### Story B.12: AccessDenied on CFN stack resources child view

**Given:** The user can list CloudFormation stacks but lacks `cloudformation:ListStackResources` permission.
**When:** The user selects a stack and presses `r` to view stack resources.
**Then:** The child view shows an error. The user can press `esc` to return.

**AWS comparison:**
aws cloudformation list-stack-resources --stack-name my-stack
Expected error: AccessDeniedException

### Story B.13: AccessDenied on target group health child view

**Given:** The user can list target groups but lacks `elasticloadbalancing:DescribeTargetHealth` permission.
**When:** The user selects a target group and presses `Enter` to view target health.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws elbv2 describe-target-health --target-group-arn arn:aws:...
Expected error: AccessDeniedException

### Story B.14: AccessDenied on ASG scaling activities child view

**Given:** The user can list auto scaling groups but lacks `autoscaling:DescribeScalingActivities` permission.
**When:** The user selects an ASG and presses `Enter` to view scaling activities.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws autoscaling describe-scaling-activities --auto-scaling-group-name my-asg
Expected error: AccessDeniedException

### Story B.15: AccessDenied on ELB listeners child view

**Given:** The user can list load balancers but lacks `elasticloadbalancing:DescribeListeners` permission.
**When:** The user selects an ELB and presses `Enter` to view listeners.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws elbv2 describe-listeners --load-balancer-arn arn:aws:...
Expected error: AccessDeniedException

### Story B.16: AccessDenied on alarm history child view

**Given:** The user can list CloudWatch alarms but lacks `cloudwatch:DescribeAlarmHistory` permission.
**When:** The user selects an alarm and presses `Enter` to view alarm history.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws cloudwatch describe-alarm-history --alarm-name my-alarm
Expected error: AccessDeniedException

### Story B.17: AccessDenied on SFN executions child view

**Given:** The user can list Step Functions state machines but lacks `states:ListExecutions` permission.
**When:** The user selects a state machine and presses `Enter` to view executions.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn arn:aws:...
Expected error: AccessDeniedException

### Story B.18: AccessDenied on CodeBuild builds child view

**Given:** The user can list CodeBuild projects but lacks `codebuild:ListBuildsForProject` or `codebuild:BatchGetBuilds` permission.
**When:** The user selects a CodeBuild project and presses `Enter` to view builds.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws codebuild list-builds-for-project --project-name my-project
Expected error: AccessDeniedException

### Story B.19: AccessDenied on Pipeline stages child view

**Given:** The user can list CodePipeline pipelines but lacks `codepipeline:GetPipelineState` permission.
**When:** The user selects a pipeline and presses `Enter` to view pipeline stages.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws codepipeline get-pipeline-state --name my-pipeline
Expected error: AccessDeniedException

### Story B.20: AccessDenied on ECR images child view

**Given:** The user can list ECR repositories but lacks `ecr:DescribeImages` permission.
**When:** The user selects an ECR repository and presses `Enter` to view images.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws ecr describe-images --repository-name my-repo
Expected error: AccessDeniedException

### Story B.21: AccessDenied on IAM role policies child view

**Given:** The user can list IAM roles but lacks `iam:ListAttachedRolePolicies` and `iam:ListRolePolicies` permissions.
**When:** The user selects a role and presses `Enter` to view attached policies.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws iam list-attached-role-policies --role-name my-role
Expected error: AccessDenied

### Story B.22: AccessDenied on IAM group members child view

**Given:** The user can list IAM groups but lacks `iam:GetGroup` permission.
**When:** The user selects a group and presses `Enter` to view group members.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws iam get-group --group-name my-group
Expected error: AccessDenied

### Story B.23: AccessDenied on R53 records child view

**Given:** The user can list Route 53 hosted zones but lacks `route53:ListResourceRecordSets` permission.
**When:** The user selects a hosted zone and presses `Enter` to view records.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws route53 list-resource-record-sets --hosted-zone-id /hostedzone/Z123
Expected error: AccessDeniedException

### Story B.24: AccessDenied on Glue job runs child view

**Given:** The user can list Glue jobs but lacks `glue:GetJobRuns` permission.
**When:** The user selects a Glue job and presses `Enter` to view job runs.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws glue get-job-runs --job-name my-job
Expected error: AccessDeniedException

### Story B.25: AccessDenied on ECS container logs child view

**Given:** The user can list ECS services but the container log fetcher requires `logs:FilterLogEvents` permission which is missing.
**When:** The user selects an ECS service and presses `L` to open container logs.
**Then:** The child view shows an error flash in the header. The user can press `esc` to return.

**AWS comparison:**
aws logs filter-log-events --log-group-name /ecs/my-service
Expected error: AccessDeniedException

### Story B.26: AccessDenied on nested child views (ELB listeners --> rules)

**Given:** The user can view ELB listeners but lacks `elasticloadbalancing:DescribeRules` permission.
**When:** The user selects a listener and presses `Enter` to view listener rules.
**Then:** The nested child view shows an error flash in the header. The user can press `esc` to return to the listeners list.

**AWS comparison:**
aws elbv2 describe-rules --listener-arn arn:aws:...
Expected error: AccessDeniedException

### Story B.27: AccessDenied on nested child views (SFN executions --> history)

**Given:** The user can list SFN executions but lacks `states:GetExecutionHistory` permission.
**When:** The user selects an execution and presses `Enter` to view execution history.
**Then:** The nested child view shows an error flash in the header. The user can press `esc` to return to the executions list.

**AWS comparison:**
aws stepfunctions get-execution-history --execution-arn arn:aws:...
Expected error: AccessDeniedException

### Story B.28: AccessDenied on nested child views (CodeBuild builds --> build logs)

**Given:** The user can view CodeBuild builds but the build log fetcher requires `logs:GetLogEvents` permission which is missing.
**When:** The user selects a build and presses `Enter` to view build logs.
**Then:** The nested child view shows an error flash in the header. The user can press `esc` to return to the builds list.

**AWS comparison:**
aws logs get-log-events --log-group-name /aws/codebuild/my-project --log-stream-name build-id
Expected error: AccessDeniedException

### Story B.29: AccessDenied across all resource types on list call

**Given:** The user's IAM identity has no permissions for any AWS service.
**When:** The user selects each of the following resource types one at a time from the main menu:
- EC2 Instances, S3 Buckets, RDS Instances, ElastiCache Redis, DocumentDB Clusters, EKS Clusters, Secrets Manager, Lambda Functions, ECS Clusters, ECS Services, ECS Tasks, CloudWatch Alarms, Log Groups, VPCs, Security Groups, Subnets, NAT Gateways, Internet Gateways, Elastic IPs, ENIs, Route Tables, Transit Gateways, VPC Endpoints, ELBs, Target Groups, ASGs, Elastic Beanstalk, CloudFront, Route 53, ACM Certificates, API Gateway, CloudFormation Stacks, CodeBuild Projects, CodePipelines, ECR Repositories, IAM Roles, IAM Policies, IAM Users, IAM Groups, WAF Web ACLs, DynamoDB Tables, OpenSearch Domains, Redshift Clusters, Kinesis Streams, SQS Queues, SNS Topics, SNS Subscriptions, EventBridge Rules, Step Functions, Glue Jobs, Athena Workgroups, MSK Clusters, CloudTrail Trails, KMS Keys, SSM Parameters, Backup Plans, SES Identities, EFS File Systems, Node Groups, RDS Snapshots, DocumentDB Snapshots, CodeArtifact Repositories
**Then:** Each one shows a red error flash in the header. None of them crashes the application.

### Story B.30: AccessDenied on SNS topic subscriptions child view

**Given:** The user can list SNS topics but lacks `sns:ListSubscriptionsByTopic` permission.
**When:** The user selects an SNS topic and presses `Enter` to view subscriptions.
**Then:** The child view shows an error. The header right side displays a red error flash. The user can press `esc` to return to the SNS topics list.

**AWS comparison:**
aws sns list-subscriptions-by-topic --topic-arn arn:aws:sns:us-east-1:123456789012:my-topic
Expected error: AuthorizationError

### Story B.31: AccessDenied on EventBridge rule targets child view

**Given:** The user can list EventBridge rules but lacks `events:ListTargetsByRule` permission.
**When:** The user selects an EventBridge rule and presses `Enter` to view rule targets.
**Then:** The child view shows an error. The header right side displays a red error flash. The user can press `esc` to return to the EventBridge rules list.

**AWS comparison:**
aws events list-targets-by-rule --rule my-rule --event-bus-name default
Expected error: AccessDeniedException

### Story B.32: AccessDenied on RDS instance events child view

**Given:** The user can list RDS instances but lacks `rds:DescribeEvents` permission.
**When:** The user selects an RDS instance and presses `Enter` to view events.
**Then:** The child view shows an error. The header right side displays a red error flash. The user can press `esc` to return to the RDS instances list.

**AWS comparison:**
aws rds describe-events --source-identifier my-db --source-type db-instance
Expected error: AccessDenied

### Story B.33: AccessDenied on nested child views (Lambda invocations --> invocation logs)

**Given:** The user can view Lambda invocations but lacks `logs:FilterLogEvents` permission for the per-invocation log line fetch.
**When:** The user selects a Lambda invocation and presses `Enter` to view its log lines.
**Then:** The nested child view shows an error flash in the header. The user can press `esc` to return to the invocations list.

**AWS comparison:**
aws logs filter-log-events --log-group-name /aws/lambda/my-function --filter-pattern "\"abc-123-request-id\""
Expected error: AccessDeniedException

---

## C. Corrupted / Malformed Data

### Story C.1: Nil fields in AWS SDK response do not cause a panic

**Given:** The AWS API returns a resource with nil/null optional fields (e.g., an EC2 instance with nil PublicIpAddress, nil KeyName, nil IamInstanceProfile).
**When:** The resource list is displayed.
**Then:** Nil fields are shown as empty cells or a dash/placeholder character in the table. The application does not panic or crash. All non-nil fields render correctly.

**AWS comparison:**
aws ec2 describe-instances (instance with no public IP, no key pair)
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP (empty), Instance ID, Launch Time

### Story C.2: Empty string IDs and names render gracefully

**Given:** The AWS API returns a resource where the name or ID field is an empty string (e.g., an EC2 instance with no Name tag).
**When:** The user views the resource list.
**Then:** The row is displayed with an empty cell for the name/ID column. The row is still selectable and navigable. Detail and YAML views work correctly for this resource.

**AWS comparison:**
aws ec2 describe-instances (instance with no Name tag)
Expected fields visible: Name (empty), State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story C.3: Unexpected enum values render as plain text

**Given:** The AWS API returns an instance with a status/state value not in the known set (e.g., an EC2 instance state that is not "running", "stopped", "pending", or "terminated").
**When:** The user views the resource list.
**Then:** The unknown status value is rendered as plain text using the default row color (`#c0caf5`). The row is not colored as green, red, yellow, or dim. The application does not crash.

**AWS comparison:**
aws ec2 describe-instances (hypothetical new state value)
Expected fields visible: Name, State (unknown value in plain color), Type

### Story C.4: Malformed ARNs render as-is without crash

**Given:** The AWS API returns a resource with an ARN that does not conform to the standard format (e.g., missing region segment, extra colons).
**When:** The user views the resource detail or YAML view.
**Then:** The malformed ARN is displayed as a literal string. The application does not crash or truncate it unexpectedly.

**AWS comparison:**
aws iam list-roles (role with unusual ARN format)

### Story C.5: Unicode and special characters in resource names

**Given:** The AWS API returns a resource whose name contains Unicode characters (e.g., emojis, CJK characters, right-to-left text) or special terminal characters (e.g., ANSI escape sequences in tag values).
**When:** The user views the resource list and detail view.
**Then:** The name is displayed without corrupting the table layout. Columns remain aligned. ANSI escape sequences in data are not interpreted as terminal formatting. The application does not crash.

**AWS comparison:**
aws ec2 describe-instances (instance with Name tag containing Unicode)
Expected fields visible: Name (with Unicode intact), State, Type

### Story C.6: Timestamps in unexpected formats render gracefully

**Given:** The AWS API returns a timestamp in an unexpected format (e.g., zero value, epoch 0, or an unusual timezone representation).
**When:** The user views the resource list or detail view.
**Then:** The timestamp is displayed as a formatted string or as a placeholder (e.g., dash or empty) if it represents a zero/null value. The application does not panic on timestamp parsing.

**AWS comparison:**
aws ec2 describe-instances (instance with zero-value LaunchTime, hypothetical)
Expected fields visible: Name, State, Type, Launch Time (placeholder or zero-formatted)

### Story C.7: Nested structs with nil pointers do not cause panics

**Given:** The AWS API returns a resource with a nested struct field that is nil (e.g., an EC2 instance where `Placement` is nil, or an RDS instance where `Endpoint` is nil).
**When:** The user views the resource in the detail view or YAML view.
**Then:** The nil nested struct is displayed as "null", an empty value, or omitted. The application does not panic on nil pointer dereference.

**AWS comparison:**
aws rds describe-db-instances (new instance where Endpoint is not yet assigned)
Expected fields visible: DB Identifier, Engine, Version, Status, Class, Endpoint (empty/null)

### Story C.8: Very long tag values do not break table layout

**Given:** A resource has a tag with a value that is 256 characters long (the AWS maximum).
**When:** The user views the resource in the detail view.
**Then:** The long tag value is displayed, potentially truncated in the table list view but fully visible in the detail or YAML view. The table columns remain properly aligned.

**AWS comparison:**
aws ec2 describe-instances --filters "Name=tag-key,Values=LongTag"

### Story C.9: Resource with all optional fields nil renders in detail view

**Given:** A resource exists where every optional field returned by the AWS API is nil (only required fields have values).
**When:** The user presses `d` to view the detail view for this resource.
**Then:** The detail view renders with the available fields showing values and the nil optional fields showing empty values or dashes. The view is scrollable and not empty. The user can press `y` to switch to YAML view.

### Story C.10: Resource with all optional fields nil renders in YAML view

**Given:** The same resource as C.9 with mostly nil optional fields.
**When:** The user presses `y` to view the YAML view for this resource.
**Then:** The YAML view renders valid YAML. Nil fields appear as `null` (rendered in dim style `#565f89`). The view is scrollable. The user can press `c` to copy the YAML content.

---

## D. Large Data Sets

### Story D.1: Paginated results with 1,000+ resources load completely

**Given:** The user's AWS account has more than 1,000 EC2 instances (exceeding a single API page).
**When:** The user selects EC2 Instances from the main menu.
**Then:** The loading spinner appears. The application fetches all pages of results. The frame title shows the full count (e.g., `ec2-instances(1247)`). All resources are listed and scrollable.

**AWS comparison:**
aws ec2 describe-instances (with > 1000 instances, paginated via NextToken)
Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

### Story D.2: Paginated results for S3 buckets

**Given:** The user's AWS account has more than 1,000 S3 buckets.
**When:** The user selects S3 Buckets from the main menu.
**Then:** All buckets are loaded across multiple API pages. The frame title shows the complete count.

**AWS comparison:**
aws s3api list-buckets (paginated)
Expected fields visible: Bucket Name, Region, Creation Date

### Story D.3: Paginated results for IAM roles

**Given:** The user's AWS account has more than 1,000 IAM roles.
**When:** The user selects IAM Roles from the main menu.
**Then:** All roles are loaded. The frame title reflects the total count.

**AWS comparison:**
aws iam list-roles (paginated via Marker)
Expected fields visible: Role Name, Last Used, Path, Created, Description

### Story D.4: Resource with 50+ tags renders in detail view

**Given:** An EC2 instance has 50 tags (the AWS maximum).
**When:** The user presses `d` to view the detail of this instance.
**Then:** All 50 tags are displayed in the Tags section of the detail view. The view is scrollable with `j`/`k` keys. No tags are silently dropped.

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-xxx (with 50 tags)
Expected fields visible: All EC2 detail fields including Tags with 50 entries

### Story D.5: S3 bucket with 10,000+ objects in child view

**Given:** An S3 bucket contains more than 10,000 objects.
**When:** The user selects the bucket and presses `Enter` to view objects.
**Then:** The S3 objects child view loads all objects across API pages. The frame title shows the total object count. The list is scrollable.

**AWS comparison:**
aws s3api list-objects-v2 --bucket my-large-bucket (paginated via ContinuationToken)
Expected fields visible: Key, Size, Storage Class, Last Modified

### Story D.6: Log group with thousands of log streams

**Given:** A CloudWatch log group has more than 5,000 log streams.
**When:** The user selects the log group and presses `Enter` to view streams.
**Then:** The log streams child view loads the streams. The frame title shows the count. The list is scrollable.

**AWS comparison:**
aws logs describe-log-streams --log-group-name /aws/lambda/busy-function --order-by LastEventTime
Expected fields visible: Stream Name, Last Event, First Event

### Story D.7: Very long resource names truncate in list view but show fully in detail

**Given:** A resource has a name that is 128 characters long.
**When:** The user views the resource list.
**Then:** The name is truncated to fit the configured column width in the list view. The full name is visible in the detail view (press `d`) and the YAML view (press `y`).

### Story D.8: Very wide YAML output supports horizontal scrolling

**Given:** A resource produces YAML output with lines exceeding 200 characters (e.g., long ARNs, base64-encoded data, inline policies).
**When:** The user presses `y` to view the YAML.
**Then:** The YAML view displays the content. The user can scroll vertically with `j`/`k`. Long lines either wrap (if word wrap is toggled on with `w`) or extend beyond the visible area.

### Story D.9: Page up and page down work with large resource lists

**Given:** The resource list contains 500 resources, more than fit on one screen.
**When:** The user presses `pgdn` or `ctrl+d`.
**Then:** The view scrolls down by approximately one page of content. Pressing `pgup` or `ctrl+u` scrolls back up. The selection cursor moves correspondingly.

### Story D.10: Sorting a large list re-renders correctly

**Given:** The resource list contains 500 EC2 instances.
**When:** The user presses `N` to sort by name.
**Then:** All 500 instances are re-sorted. The column header shows the sort indicator (`↑` or `↓`). The selected row moves to reflect its new position. Pressing `N` again reverses the sort order.

**AWS comparison:**
aws ec2 describe-instances --query 'sort_by(Reservations[].Instances[], &Tags[?Key==`Name`].Value | [0])'
Expected fields visible: Name (with sort indicator), State, Lifecycle, Type

### Story D.11: Filtering a large list is responsive

**Given:** The resource list contains 1,000+ resources.
**When:** The user presses `/` and types a filter string.
**Then:** The list filters in real time as the user types, showing only matching rows. The frame title updates to show `(matched/total)`. There is no noticeable delay or lag.

---

## E. Empty / Missing Data

### Story E.1: Zero resources returned shows empty state message

**Given:** The user's AWS account has no EC2 instances in the selected region.
**When:** The user selects EC2 Instances from the main menu.
**Then:** The resource list frame is displayed but contains no data rows. The frame title shows `ec2-instances(0)`. A centered hint message suggests refreshing or changing region. The application does not crash.

**AWS comparison:**
aws ec2 describe-instances (in a region with no instances)
Expected fields visible: (none -- empty state with hint)

### Story E.2: Zero resources for every resource type shows empty state

**Given:** The user's AWS account is brand new with no resources in the selected region.
**When:** The user selects each resource type one at a time from the main menu.
**Then:** Each one shows `resource-type(0)` in the frame title and an empty list with a hint. None of them crashes.

### Story E.3: Empty S3 objects child view

**Given:** An S3 bucket exists but contains no objects.
**When:** The user selects the bucket and presses `Enter` to view objects.
**Then:** The S3 objects child view shows an empty list. The frame title shows a zero count. The user can press `esc` to return to the bucket list.

**AWS comparison:**
aws s3 ls s3://empty-bucket/
Expected fields visible: (none -- empty)

### Story E.4: Empty log streams child view

**Given:** A CloudWatch log group exists but has no log streams.
**When:** The user selects the log group and presses `Enter` to view streams.
**Then:** The log streams child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws logs describe-log-streams --log-group-name /empty/group
Expected fields visible: (none -- empty)

### Story E.5: Empty Lambda invocations child view

**Given:** A Lambda function exists but has never been invoked (no CloudWatch logs).
**When:** The user selects the function and presses `Enter` to view invocations.
**Then:** The invocations child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws logs filter-log-events --log-group-name /aws/lambda/never-invoked --filter-pattern "REPORT"
Expected fields visible: (none -- empty)

### Story E.6: Empty ECS service events child view

**Given:** An ECS service exists but has no recent events.
**When:** The user selects the service and presses `e` to view events.
**Then:** The events child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws ecs describe-services --cluster my-cluster --services new-svc
Expected fields visible: (none -- empty events list)

### Story E.7: Empty ECS tasks child view

**Given:** An ECS service has zero running or stopped tasks.
**When:** The user selects the service and presses `Enter` to view tasks.
**Then:** The tasks child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws ecs list-tasks --cluster my-cluster --service-name no-tasks-svc
Expected fields visible: (none -- empty)

### Story E.8: Empty CFN stack events child view

**Given:** A CloudFormation stack exists but the events API returns an empty list (unusual but possible for deleted stacks).
**When:** The user selects the stack and presses `Enter` to view events.
**Then:** The events child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws cloudformation describe-stack-events --stack-name my-stack
Expected fields visible: (none -- empty)

### Story E.9: Empty CFN stack resources child view

**Given:** A CloudFormation stack exists but has no resources (e.g., empty template).
**When:** The user selects the stack and presses `r` to view resources.
**Then:** The resources child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws cloudformation list-stack-resources --stack-name empty-stack
Expected fields visible: (none -- empty)

### Story E.10: Empty target group health child view

**Given:** A target group exists but has no registered targets.
**When:** The user selects the target group and presses `Enter` to view target health.
**Then:** The target health child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws elbv2 describe-target-health --target-group-arn arn:aws:...
Expected fields visible: (none -- empty)

### Story E.11: Empty ASG scaling activities child view

**Given:** An ASG exists but has no scaling activity history.
**When:** The user selects the ASG and presses `Enter` to view scaling activities.
**Then:** The activities child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws autoscaling describe-scaling-activities --auto-scaling-group-name new-asg
Expected fields visible: (none -- empty)

### Story E.12: Empty alarm history child view

**Given:** A CloudWatch alarm exists but has no history entries.
**When:** The user selects the alarm and presses `Enter` to view alarm history.
**Then:** The history child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws cloudwatch describe-alarm-history --alarm-name new-alarm
Expected fields visible: (none -- empty)

### Story E.13: Empty ELB listeners child view

**Given:** A load balancer exists but has no listeners configured.
**When:** The user selects the ELB and presses `Enter` to view listeners.
**Then:** The listeners child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws elbv2 describe-listeners --load-balancer-arn arn:aws:...
Expected fields visible: (none -- empty)

### Story E.14: Empty SFN executions child view

**Given:** A Step Functions state machine exists but has never been executed.
**When:** The user selects the state machine and presses `Enter` to view executions.
**Then:** The executions child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn arn:aws:...
Expected fields visible: (none -- empty)

### Story E.15: Empty CodeBuild builds child view

**Given:** A CodeBuild project exists but has never been built.
**When:** The user selects the project and presses `Enter` to view builds.
**Then:** The builds child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws codebuild list-builds-for-project --project-name new-project
Expected fields visible: (none -- empty)

### Story E.16: Empty Pipeline stages child view

**Given:** A CodePipeline exists but has no execution state yet.
**When:** The user selects the pipeline and presses `Enter` to view stages.
**Then:** The stages child view shows an empty list or minimal stage structure. The user can press `esc` to return.

**AWS comparison:**
aws codepipeline get-pipeline-state --name new-pipeline
Expected fields visible: (none or minimal stage data)

### Story E.17: Empty ECR images child view

**Given:** An ECR repository exists but contains no images.
**When:** The user selects the repository and presses `Enter` to view images.
**Then:** The images child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws ecr describe-images --repository-name empty-repo
Expected fields visible: (none -- empty)

### Story E.18: Empty IAM role policies child view

**Given:** An IAM role exists but has no attached or inline policies.
**When:** The user selects the role and presses `Enter` to view policies.
**Then:** The policies child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws iam list-attached-role-policies --role-name bare-role
Expected fields visible: (none -- empty)

### Story E.19: Empty IAM group members child view

**Given:** An IAM group exists but has no members.
**When:** The user selects the group and presses `Enter` to view members.
**Then:** The members child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws iam get-group --group-name empty-group
Expected fields visible: (none -- empty)

### Story E.20: Empty R53 records child view

**Given:** A Route 53 hosted zone exists but has only the default NS and SOA records (or none if empty).
**When:** The user selects the zone and presses `Enter` to view records.
**Then:** The records child view shows the minimal record set. The user can press `esc` to return.

**AWS comparison:**
aws route53 list-resource-record-sets --hosted-zone-id /hostedzone/Z123
Expected fields visible: Name, Type, TTL, Values

### Story E.21: Empty Glue job runs child view

**Given:** A Glue job exists but has never been run.
**When:** The user selects the job and presses `Enter` to view job runs.
**Then:** The job runs child view shows an empty list. The user can press `esc` to return.

**AWS comparison:**
aws glue get-job-runs --job-name never-run-job
Expected fields visible: (none -- empty)

### Story E.22: Empty ECS container logs child view

**Given:** An ECS service exists but the container log group is empty or the log configuration is not set.
**When:** The user selects the service and presses `L` to view container logs.
**Then:** The container logs child view shows an empty list or an error message if log configuration is missing. The user can press `esc` to return.

### Story E.23: Empty nested child views (ELB listeners --> rules)

**Given:** An ELB listener exists but has no custom rules (only the default rule).
**When:** The user views the listener and presses `Enter` to view listener rules.
**Then:** The rules child view shows at least the default rule. The user can press `esc` to return.

**AWS comparison:**
aws elbv2 describe-rules --listener-arn arn:aws:...
Expected fields visible: Priority, Conditions, Action, Target

### Story E.24: Empty nested child views (SFN executions --> history)

**Given:** An SFN execution exists and has been started but the history is minimal.
**When:** The user selects the execution and presses `Enter` to view history.
**Then:** The history child view shows whatever events are available. The user can press `esc` to return.

**AWS comparison:**
aws stepfunctions get-execution-history --execution-arn arn:aws:...
Expected fields visible: Timestamp, Event Type, State, Detail

### Story E.25: Empty nested child views (CodeBuild builds --> build logs)

**Given:** A CodeBuild build exists but has no log output (e.g., failed immediately).
**When:** The user selects the build and presses `Enter` to view build logs.
**Then:** The build logs child view shows an empty list or a message indicating no logs. The user can press `esc` to return.

### Story E.26: Empty tags map in detail view

**Given:** A resource exists with an empty tags map (Tags field present but no entries).
**When:** The user presses `d` to view the detail.
**Then:** The Tags section in the detail view is either omitted, shows "none", or displays an empty section. The view does not crash or show a raw empty map.

### Story E.27: Config file missing

**Given:** The user has no `~/.a9s/views.yaml` configuration file.
**When:** The user launches a9s.
**Then:** The application starts normally using built-in default column and detail configurations. No error is shown about the missing config file.

### Story E.28: Config file empty

**Given:** The user has a `~/.a9s/views.yaml` file that exists but is completely empty (0 bytes).
**When:** The user launches a9s.
**Then:** The application starts normally using built-in defaults. No crash occurs.

### Story E.29: Config file with unknown keys

**Given:** The user has a `~/.a9s/views.yaml` file with unrecognized keys (e.g., `unknown_resource: ...` or misspelled field names).
**When:** The user launches a9s.
**Then:** The application starts normally. Unknown keys are ignored. Known resource view configurations still apply correctly.

### Story E.30: Config file with malformed YAML

**Given:** The user has a `~/.a9s/views.yaml` file with invalid YAML syntax (e.g., unclosed quotes, bad indentation).
**When:** The user launches a9s.
**Then:** The application either shows a clear error about the malformed config or falls back to built-in defaults. The application does not crash with a YAML parse stack trace.

### Story E.31: Empty SNS topic subscriptions child view

**Given:** An SNS topic exists but has no subscriptions.
**When:** The user selects the topic and presses `Enter` to view subscriptions.
**Then:** The subscriptions child view shows an empty state message centered in the child frame. The frame title shows a `(0)` count. The user can press `esc` to return to the SNS topics list.

**AWS comparison:**
aws sns list-subscriptions-by-topic --topic-arn arn:aws:sns:us-east-1:123456789012:lonely-topic
Expected fields visible: (none -- empty)

### Story E.32: Empty EventBridge rule targets child view

**Given:** An EventBridge rule exists but has no targets attached.
**When:** The user selects the rule and presses `Enter` to view rule targets.
**Then:** The targets child view shows an empty state message centered in the child frame. The frame title shows a `(0)` count. The user can press `esc` to return to the EventBridge rules list.

**AWS comparison:**
aws events list-targets-by-rule --rule no-targets-rule --event-bus-name default
Expected fields visible: (none -- empty)

### Story E.33: Empty RDS instance events child view

**Given:** An RDS instance exists but has no events in the last 7 days.
**When:** The user selects the instance and presses `Enter` to view events.
**Then:** The events child view shows an empty state message centered in the child frame. The frame title shows a `(0)` count. The user can press `esc` to return to the RDS instances list.

**AWS comparison:**
aws rds describe-events --source-identifier quiet-db --source-type db-instance --duration 10080
Expected fields visible: (none -- empty)

### Story E.34: Empty nested child views (Lambda invocations --> invocation logs)

**Given:** A Lambda invocation exists but the matching log lines have aged out of the CloudWatch retention window.
**When:** The user selects the invocation and presses `Enter` to view its log lines.
**Then:** The nested invocation logs child view shows an empty state message centered in the child frame. The frame title shows a `(0)` count. The user can press `esc` to return to the invocations list.

**AWS comparison:**
aws logs filter-log-events --log-group-name /aws/lambda/my-function --filter-pattern "\"abc-123-request-id\""
Expected fields visible: (none -- empty)

---

## F. Concurrency & Timing

### Story F.1: Profile switch mid-fetch cancels in-flight request

**Given:** The user has navigated to a resource list and the loading spinner is visible (data is being fetched).
**When:** The user presses `:`, types `ctx`, and presses `Enter` to switch to the profile selector, then selects a different profile.
**Then:** The in-flight API request is cancelled or its result is discarded. The application loads data using the new profile's credentials. There is no data from the previous profile displayed.

### Story F.2: Region switch mid-fetch cancels in-flight request

**Given:** The user has navigated to a resource list and the loading spinner is visible.
**When:** The user presses `:`, types `region`, and presses `Enter` to switch regions, then selects a different region.
**Then:** The in-flight API request is cancelled or its result is discarded. The application loads data from the new region. There is no stale data from the old region.

### Story F.3: Resource deleted between list and detail view

**Given:** The user is viewing the EC2 instances list and selects an instance.
**When:** The instance is terminated/deleted in AWS between the time the list was loaded and the user presses `d` to view details.
**Then:** The detail view either shows the last-known data from the list fetch or displays an error if it attempts a fresh describe call. The application does not crash.

**AWS comparison:**
aws ec2 describe-instances --instance-ids i-deleted (after termination)

### Story F.4: Resource deleted between list and child view

**Given:** The user is viewing S3 buckets and selects a bucket.
**When:** The bucket is deleted in AWS between the list load time and pressing `Enter` to view objects.
**Then:** The child view shows an error (e.g., "NoSuchBucket") in the header. The user can press `esc` to return. The application does not crash.

**AWS comparison:**
aws s3 ls s3://deleted-bucket/
Expected error: NoSuchBucket

### Story F.5: Refresh after resource state change shows updated data

**Given:** The user is viewing EC2 instances and an instance that was "running" has been stopped externally.
**When:** The user presses `ctrl+r` to refresh.
**Then:** The list reloads from AWS. The instance now shows "stopped" status. The row color changes from green to red.

**AWS comparison:**
aws ec2 describe-instances (after stopping instance)
Expected fields visible: Name, State (stopped -- red row), Type

### Story F.6: Rapid esc presses during view transitions do not cause crash

**Given:** The user has drilled into a deeply nested view (e.g., ELB --> Listeners --> Rules).
**When:** The user presses `esc` rapidly three times in quick succession.
**Then:** The view stack unwinds correctly, returning to the ELB list or the main menu depending on timing. The application does not crash, leave ghost views, or display corrupted state.

### Story F.7: Navigation during loading state does not corrupt view

**Given:** A resource list is loading (spinner visible).
**When:** The user presses `esc` to go back to the main menu before loading completes.
**Then:** The navigation occurs immediately. The loading is cancelled. The main menu is displayed correctly. If the user re-enters the same resource type, a fresh load begins.

### Story F.8: Profile switch followed by immediate resource navigation

**Given:** The user switches to a new profile via `:ctx`.
**When:** Immediately after the profile switch, the user selects a resource type from the main menu.
**Then:** The resource list loads using the new profile's credentials. There is no residual data from the previous profile.

---

## G. Demo Mode

### Story G.1: Demo mode launches without AWS credentials

**Given:** The user has no AWS credentials configured.
**When:** The user launches a9s in demo mode (e.g., `a9s --demo`).
**Then:** The application starts successfully. The main menu displays all resource types. No AWS API calls are made.

### Story G.2: Demo mode displays fixtures for all resource types

**Given:** The user is running a9s in demo mode.
**When:** The user selects each resource type from the main menu one at a time.
**Then:** Each resource type displays synthetic fixture data with multiple rows. The data includes a mix of statuses (running, stopped, pending) to exercise row coloring. No resource type shows an error or empty list.

### Story G.3: Demo mode detail view shows complete fields

**Given:** The user is in demo mode and viewing a resource list.
**When:** The user selects a resource and presses `d` to view details.
**Then:** The detail view shows synthetic data for all configured detail fields. No field is unexpectedly nil or missing.

### Story G.4: Demo mode YAML view shows complete output

**Given:** The user is in demo mode and viewing a resource list.
**When:** The user selects a resource and presses `y` to view YAML.
**Then:** The YAML view shows valid syntax-highlighted YAML with a representative set of fields and values.

### Story G.5: Demo mode child views display fixture data

**Given:** The user is in demo mode.
**When:** The user navigates into each child view (S3 objects, log streams, ECS tasks, etc.).
**Then:** Each child view displays synthetic fixture data. No child view returns an error or empty state (unless the fixture intentionally represents an empty-state scenario).

### Story G.6: Demo mode covers error display paths

**Given:** The user is in demo mode.
**When:** The user looks at resource lists with fixture data.
**Then:** At least some fixtures include resources in error/failed states (e.g., a stopped EC2 instance displayed in red, a failed CloudFormation stack, a terminated ECS task). This exercises the error/warning row coloring paths.

### Story G.7: Demo mode profile shows as "demo"

**Given:** The user is running in demo mode.
**When:** The user looks at the header bar.
**Then:** The profile portion of the header shows "demo" (or a similar indicator), making it clear this is not live data.

### Story G.8: Demo mode supports all navigation features

**Given:** The user is in demo mode.
**When:** The user uses navigation keys (j/k/g/G), filter (/), sort (N/S/A), horizontal scroll (h/l), command mode (:), and help (?).
**Then:** All features work identically to live mode. Sorting reorders the fixture data. Filtering narrows results. The help screen opens and closes. Commands navigate to the correct views.

---

## H. Terminal Size Edge Cases

### Story H.1: Terminal exactly at minimum width (60 columns)

**Given:** The user's terminal is exactly 60 columns wide.
**When:** The user navigates through resource lists and detail views.
**Then:** The UI renders without errors. Only the first 2 columns (Name, Status) may be visible due to the narrow width. The application is fully functional.

### Story H.2: Terminal exactly at minimum height (7 lines)

**Given:** The user's terminal is exactly 7 lines tall.
**When:** The main menu is displayed.
**Then:** The header uses 1 line, the frame borders use 2 lines, and 4 lines of content are available. The resource list shows up to 3 data rows (after the header row). Navigation works correctly.

### Story H.3: Terminal one pixel below minimum width shows error

**Given:** The user's terminal is 59 columns wide.
**When:** The application renders.
**Then:** The error message "Terminal too narrow. Please resize." is displayed. No other content is rendered.

### Story H.4: Terminal one pixel below minimum height shows error

**Given:** The user's terminal is 6 lines tall.
**When:** The application renders.
**Then:** The error message "Terminal too short. Please resize." is displayed. No other content is rendered.

### Story H.5: Resize from below minimum to above minimum restores UI

**Given:** The terminal is 50 columns wide and the "Terminal too narrow" error is showing.
**When:** The user resizes the terminal to 80 columns wide.
**Then:** The error disappears and the full UI renders correctly at the new size.

### Story H.6: Resize from above minimum to below minimum shows error

**Given:** The application is running normally at 120 columns wide.
**When:** The user resizes the terminal to 50 columns wide.
**Then:** The error message "Terminal too narrow. Please resize." replaces all content. The application does not crash.

### Story H.7: Resize during detail view re-renders correctly

**Given:** The user is viewing the detail view for an EC2 instance.
**When:** The user resizes the terminal from 120 columns to 80 columns.
**Then:** The detail view re-renders at the new width. Key-value pairs reflow. The content is still scrollable and readable.

### Story H.8: Resize during YAML view re-renders correctly

**Given:** The user is viewing the YAML view for a resource.
**When:** The user resizes the terminal.
**Then:** The YAML view re-renders at the new size. Syntax highlighting is preserved. The scroll position is maintained or adjusted appropriately.

### Story H.9: Resize during help screen re-renders correctly

**Given:** The help screen is open.
**When:** The user resizes the terminal.
**Then:** The help screen re-renders at the new size. The four-column layout adjusts. Column widths recalculate.

### Story H.10: Resize during child view re-renders correctly

**Given:** The user is viewing a child view (e.g., S3 objects inside a bucket).
**When:** The user resizes the terminal.
**Then:** The child view re-renders at the new size. Columns adjust. The application does not crash.

### Story H.11: Extremely wide terminal (300+ columns) renders without overflow

**Given:** The user's terminal is 300 columns wide.
**When:** The user views a resource list with all columns visible.
**Then:** All configured columns are displayed. Extra space is handled gracefully (e.g., columns use their defined widths, remaining space is empty). No rendering artifacts.

### Story H.12: Extremely tall terminal (200+ lines) renders without overflow

**Given:** The user's terminal is 200 lines tall.
**When:** The user views a resource list with 50 resources.
**Then:** All 50 rows are visible without scrolling. The frame fills the full terminal height. Empty space below the last row is handled gracefully.

---

## I. View-Specific Error Handling

### Story I.1: Copy to clipboard when clipboard is unavailable

**Given:** The user is on a system where clipboard access is not available (e.g., headless SSH session without xclip/xsel/pbcopy).
**When:** The user selects a resource and presses `c` to copy the resource ID.
**Then:** The header shows a red error flash indicating the copy failed. The application does not crash.

### Story I.2: Reveal secret for a deleted secret

**Given:** The user is viewing the Secrets Manager list. A secret was deleted between the list load and the reveal attempt.
**When:** The user selects the deleted secret and presses `x` to reveal.
**Then:** The reveal view shows an error (e.g., "Error: ResourceNotFoundException"). The user can press `esc` to return. The application does not crash.

**AWS comparison:**
aws secretsmanager get-secret-value --secret-id deleted-secret
Expected error: ResourceNotFoundException

### Story I.3: Reveal secret for a secret with no current version

**Given:** A secret exists in Secrets Manager but has no current secret value (e.g., created but never given a value, or all versions are in staging/pending).
**When:** The user selects the secret and presses `x` to reveal.
**Then:** The reveal view shows an error or an empty value indicator. The user can press `esc` to return.

**AWS comparison:**
aws secretsmanager get-secret-value --secret-id empty-secret
Expected error: ResourceNotFoundException: Secrets Manager can't find the specified secret value

### Story I.4: Reveal view shows red warning in header

**Given:** The user has successfully revealed a secret value.
**When:** The secret is displayed in the reveal view.
**Then:** The header right side shows a persistent red warning: "Secret visible -- press esc to close" in red (`#f7768e`). This replaces the "? for help" hint. The warning does not auto-clear.

### Story I.5: Pressing x on non-secret resource type is a no-op

**Given:** The user is viewing the EC2 instances list (not Secrets Manager or SSM).
**When:** The user presses `x`.
**Then:** Nothing happens. The `x` key binding is only active for Secrets Manager and SSM parameter views.

### Story I.6: Unknown command in command mode shows error flash

**Given:** The user is in command mode (pressed `:`).
**When:** The user types "invalidcommand" and presses `Enter`.
**Then:** The header right side shows a red error flash (e.g., "Error: unknown command"). The error auto-clears after approximately 2 seconds.

### Story I.7: Tab autocomplete with no match in command mode

**Given:** The user is in command mode and has typed "zzz".
**When:** The user presses `Tab` to autocomplete.
**Then:** Nothing happens or the text remains "zzz". No crash or unexpected behavior.

### Story I.8: Sort key on resource type without that sort field

**Given:** The user is viewing a resource list that does not have a status column (e.g., a resource type where Status is not defined in views.yaml).
**When:** The user presses `S` to sort by status.
**Then:** The sort is either a no-op or sorts by the closest available column. The application does not crash.

### Story I.9: Horizontal scroll on resource type with few columns

**Given:** The user is viewing SNS Topics which has only 2 list columns (Topic Name, Topic ARN).
**When:** The user presses `l` to scroll columns right.
**Then:** The scroll is a no-op (all columns fit on screen) or stops at the last column. The application does not crash.

**AWS comparison:**
aws sns list-topics
Expected fields visible: Topic Name, Topic ARN

### Story I.10: Detail view for resource with no configured detail fields

**Given:** A resource type has an empty or missing detail configuration in views.yaml.
**When:** The user selects a resource and presses `d`.
**Then:** The detail view shows whatever data is available (possibly a minimal set of fields from the raw resource) or an appropriate message. The application does not crash.

### Story I.11: YAML view renders valid YAML even with complex nested data

**Given:** A resource has deeply nested fields (e.g., an EC2 instance with BlockDeviceMappings, SecurityGroups, Tags, MetadataOptions).
**When:** The user presses `y` to view YAML.
**Then:** The YAML output is valid and parseable. All nested structures are properly indented. YAML keys are colored in blue (`#7aa2f7`), string values in green (`#9ece6a`), numbers in orange (`#ff9e64`), booleans in purple (`#bb9af7`), and null values in dim (`#565f89`).

### Story I.12: Word wrap toggle in detail view handles long values

**Given:** The user is viewing the detail view for a resource with a very long field value (e.g., a 500-character description).
**When:** The user presses `w` to toggle word wrap.
**Then:** The long value wraps to the next line(s) within the frame width. Pressing `w` again disables wrap and the long value extends as a single line.

---

## J. Profile & Region Selector Edge Cases

### Story J.1: Profile selector with no profiles configured

**Given:** The user has no AWS profiles in `~/.aws/config` or `~/.aws/credentials`.
**When:** The user opens the profile selector via `:ctx`.
**Then:** The profile selector shows an empty list or only a "default" entry. The application does not crash.

### Story J.2: Profile selector shows "(no credentials)" for profiles without credentials

**Given:** The user has AWS profiles configured, but one profile has no credentials (e.g., config entry without matching credentials).
**When:** The user opens the profile selector via `:ctx`.
**Then:** The profile without credentials is shown with a dimmed "(no credentials)" indicator, as shown in the design spec wireframe (section 4.6).

### Story J.3: Switching to a profile with no credentials shows error

**Given:** The user opens the profile selector and selects a profile marked "(no credentials)".
**When:** The user presses `Enter` to switch to that profile.
**Then:** The profile switch occurs but subsequent resource fetches show credential errors. The application does not crash during the switch.

### Story J.4: Current profile shows "(current)" indicator

**Given:** The user is on the "prod" profile.
**When:** The user opens the profile selector.
**Then:** The "prod" profile row shows a "(current)" indicator. The current profile is pre-selected.

### Story J.5: Esc in profile selector cancels without switching

**Given:** The user is on the "prod" profile and opens the profile selector.
**When:** The user navigates to "staging" but presses `Esc` instead of `Enter`.
**Then:** The profile remains "prod". The header continues to show "prod". The profile selector closes and the previous view is restored.

### Story J.6: Region selector with standard regions

**Given:** The user opens the region selector via `:region`.
**When:** The region selector appears.
**Then:** It displays AWS regions (e.g., us-east-1, us-west-2, eu-west-1, etc.) with descriptions. The current region is highlighted with a "(current)" indicator.

### Story J.7: Switching region clears resource cache

**Given:** The user has loaded EC2 instances in us-east-1.
**When:** The user switches to us-west-2 via the region selector.
**Then:** The header updates to show the new region. If the user re-enters EC2 Instances, the data reloads from us-west-2. The previous us-east-1 data is not shown.

### Story J.8: Esc in region selector cancels without switching

**Given:** The user is in us-east-1 and opens the region selector.
**When:** The user navigates to eu-west-1 but presses `Esc`.
**Then:** The region remains us-east-1. The header continues to show us-east-1. The previous view is restored.

### Story J.9: Profile switch reloads main menu

**Given:** The user is on the main menu and switches to a new profile.
**When:** The profile switch completes.
**Then:** The main menu is displayed. The header shows the new profile name. Any previously loaded resource data for the old profile is cleared.

### Story J.10: Region switch from within a resource list

**Given:** The user is viewing the EC2 instances list and switches regions via `:region`.
**When:** The region switch completes.
**Then:** The application either returns to the main menu or reloads the current resource list from the new region. The header shows the new region.

---

## K. Cross-Cutting Error Resilience

### Story K.1: No panic on any input sequence

**Given:** The application is running in any view.
**When:** The user presses any combination of keys in any order (including special keys, function keys, Unicode input).
**Then:** The application does not panic. Unrecognized keys are silently ignored in normal mode. In filter and command modes, printable characters are appended to the input.

### Story K.2: View stack integrity after error

**Given:** The user navigated from Main Menu -> EC2 List -> Detail View, and the detail view encountered an error.
**When:** The user presses `esc`.
**Then:** The user returns to the EC2 List, not to the main menu. The view stack is correctly maintained despite the error.

### Story K.3: Error in one child view does not affect sibling child views

**Given:** The user is viewing ECS services. Opening Events (press `e`) results in an error.
**When:** The user presses `esc` to return to the ECS services list, then presses `Enter` to open Tasks.
**Then:** The Tasks child view loads independently. The prior Events error does not prevent Tasks from loading.

### Story K.4: Rapid ctrl+r does not cause concurrent request corruption

**Given:** The user is viewing a resource list.
**When:** The user presses `ctrl+r` three times rapidly.
**Then:** The application handles the refresh correctly. Only the most recent request's data is displayed. There is no data corruption from concurrent fetches.

### Story K.5: Application handles very slow API responses gracefully

**Given:** An AWS API call takes more than 30 seconds to respond.
**When:** The user is waiting for a resource list to load.
**Then:** The loading spinner continues to animate. The application remains responsive to `esc` (to go back) and `ctrl+c` (to quit). If a timeout is configured, a clear timeout error is shown.

### Story K.6: Error flash does not overlap with filter or command mode

**Given:** An error flash is being displayed in the header right side.
**When:** The user presses `/` to enter filter mode.
**Then:** The filter input replaces the error flash in the header. The modes do not overlap or produce garbled text.

### Story K.7: Frame title shows correct count after error then refresh

**Given:** A resource list failed to load (showing 0 resources due to error).
**When:** The error condition clears and the user presses `ctrl+r` to refresh.
**Then:** The frame title updates from `resource-type(0)` to the correct count (e.g., `ec2-instances(42)`). The data is displayed correctly.

### Story K.8: Empty filter result followed by clear shows all resources

**Given:** The user typed a filter "zzzzz" that matches zero resources.
**When:** The user presses `Esc` to clear the filter.
**Then:** All resources reappear. The frame title returns to showing the full count.

### Story K.9: Copy from empty resource list

**Given:** A resource list is empty (0 resources loaded, either due to error or no resources existing).
**When:** The user presses `c` to copy.
**Then:** The application either shows a warning flash or does nothing. It does not crash.

### Story K.10: Sort on empty resource list

**Given:** A resource list is empty (0 resources).
**When:** The user presses `N`, `S`, or `A` to sort.
**Then:** The sort is a no-op. The application does not crash. The sort indicator may appear on the column header but no rows change.

### Story K.11: Filter on empty resource list

**Given:** A resource list is empty (0 resources).
**When:** The user presses `/` and types a filter string.
**Then:** The filter activates normally. The frame title shows `resource-type(0/0)`. No crash occurs.

### Story K.12: Detail view on empty resource list

**Given:** A resource list is empty (0 resources, no selected row).
**When:** The user presses `d` or `Enter`.
**Then:** The key press is a no-op. The application does not crash or navigate to an empty detail view.

### Story K.13: YAML view on empty resource list

**Given:** A resource list is empty (0 resources, no selected row).
**When:** The user presses `y`.
**Then:** The key press is a no-op. The application does not crash.

### Story K.14: Horizontal scroll on empty resource list

**Given:** A resource list is empty.
**When:** The user presses `h` or `l` to scroll columns.
**Then:** The column headers may scroll but no crash occurs. The application remains functional.

### Story K.15: Multiple concurrent profile and region switches

**Given:** The user is on the main menu.
**When:** The user opens profile selector, selects a profile, immediately opens region selector, and selects a region.
**Then:** Both switches complete correctly. The header shows the new profile and new region. The application does not crash from the rapid configuration changes.
