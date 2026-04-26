# QA User Stories: Human-Readable Timestamp and Size Formatting (Issue #57)

Covers the conversion of all raw timestamp and size values to human-readable formats
across every resource type. Timestamps should display as relative time (e.g., "2h ago",
"3d ago") or formatted dates (e.g., "Mar 23, 2026") instead of raw ISO 8601 / Unix
timestamps. Sizes should display as KB / MB / GB instead of raw byte counts.

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration files.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. Timestamp Formatting -- List View Columns

All timestamp columns across every resource type should display human-readable values
instead of raw ISO 8601 strings like `2026-01-15T09:22:45Z`.

### A.1 EC2 Instances

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I open the EC2 instance list. An instance was launched 2 hours ago. | The "Launch Time" column shows a relative time such as "2h ago" instead of a raw ISO 8601 timestamp like "2026-03-27T08:00:00Z". |
| A.1.2 | I open the EC2 instance list. An instance was launched 45 days ago. | The "Launch Time" column shows either a relative time like "45d ago" or a formatted date like "Feb 10, 2026" -- not a raw ISO timestamp. |
| A.1.3 | I verify the Launch Time column data against `aws ec2 describe-instances`. | The "Launch Time" column maps to `.Reservations[].Instances[].LaunchTime`. The displayed human-readable value represents the same point in time as the raw API response. |

**AWS comparison:**

```
aws ec2 describe-instances --query 'Reservations[].Instances[].[Tags[?Key==`Name`].Value|[0],State.Name,InstanceType,LaunchTime]' --output table
```

Expected field visible: Launch Time (path: LaunchTime)

### A.2 S3 Buckets

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | I open the S3 bucket list. A bucket was created 6 months ago. | The "Creation Date" column shows a human-readable value such as "6mo ago" or "Sep 27, 2025" instead of a raw ISO timestamp. |
| A.2.2 | I verify the Creation Date against `aws s3api list-buckets`. | The "Creation Date" column maps to `.Buckets[].CreationDate`. The displayed value corresponds to the same moment in time. |

**AWS comparison:**

```
aws s3api list-buckets --query 'Buckets[].[Name,CreationDate]' --output table
```

Expected field visible: Creation Date (path: CreationDate)

### A.3 RDS Instances (dbi)

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | I open the RDS instance list. No timestamp column is configured in the list view. | The RDS list shows DB Identifier, Engine, Version, Status, Class, Endpoint, Multi-AZ. No raw timestamps are visible in the list view. Timestamp fields appear only in the detail view. |

**AWS comparison:**

```
aws rds describe-db-instances --query 'DBInstances[].[DBInstanceIdentifier,Engine,EngineVersion,DBInstanceStatus]' --output table
```

### A.4 CloudFormation Stacks (cfn)

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | I open the CloudFormation stacks list. A stack was created 3 days ago and updated 1 hour ago. | The "Created" column shows "3d ago" or "Mar 24, 2026". The "Updated" column shows "1h ago" or a similar human-readable form. Neither shows raw ISO 8601. |
| A.4.2 | A stack has never been updated (LastUpdatedTime is null). | The "Updated" column shows a dash, "n/a", or is empty. It does not show "null" or crash. |
| A.4.3 | I verify timestamps against `aws cloudformation describe-stacks`. | "Created" maps to `.Stacks[].CreationTime`. "Updated" maps to `.Stacks[].LastUpdatedTime`. |

**AWS comparison:**

```
aws cloudformation describe-stacks --query 'Stacks[].[StackName,StackStatus,CreationTime,LastUpdatedTime]' --output table
```

Expected fields visible: Created (path: CreationTime), Updated (path: LastUpdatedTime)

### A.5 Lambda Functions

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | I open the Lambda function list. A function was last modified 5 minutes ago. | The "Last Modified" column shows "5m ago" or similar relative time. |
| A.5.2 | I verify the Last Modified value against `aws lambda list-functions`. | "Last Modified" maps to `.Functions[].LastModified`. The displayed value represents the same moment. |

**AWS comparison:**

```
aws lambda list-functions --query 'Functions[].[FunctionName,Runtime,MemorySize,State,LastModified]' --output table
```

Expected field visible: Last Modified (path: LastModified)

### A.6 CodeBuild Projects (cb)

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | I open the CodeBuild project list. A project was last modified 12 hours ago. | The "Last Modified" column shows "12h ago" or similar human-readable format. |

**AWS comparison:**

```
aws codebuild list-projects
aws codebuild batch-get-projects --names PROJECT --query 'projects[].[name,source.type,lastModified]'
```

Expected field visible: Last Modified (path: LastModified)

### A.7 Secrets Manager

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | I open the Secrets Manager list. A secret was last accessed yesterday and last changed 2 weeks ago. | The "Last Accessed" column shows "1d ago" or "Mar 26, 2026". The "Last Changed" column shows "2w ago" or "Mar 13, 2026". |
| A.7.2 | A secret has never been accessed (LastAccessedDate is null). | The "Last Accessed" column shows a dash, "n/a", or is empty. No crash or "null" text. |

**AWS comparison:**

```
aws secretsmanager list-secrets --query 'SecretList[].[Name,LastAccessedDate,LastChangedDate]' --output table
```

Expected fields visible: Last Accessed (path: LastAccessedDate), Last Changed (path: LastChangedDate)

### A.8 IAM Roles

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | I open the IAM roles list. A role was created 1 year ago and last used 3 hours ago. | The "Created" column shows "1y ago" or "Mar 27, 2025". The "Last Used" column shows "3h ago". |
| A.8.2 | A role has never been used (RoleLastUsed.LastUsedDate is null). | The "Last Used" column shows a dash, "n/a", or is empty. |

**AWS comparison:**

```
aws iam list-roles --query 'Roles[].[RoleName,CreateDate,RoleLastUsed.LastUsedDate]' --output table
```

Expected fields visible: Created (path: CreateDate), Last Used (path: RoleLastUsed.LastUsedDate)

### A.9 IAM Policies

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | I open the IAM policies list. A policy was created 6 months ago. | The "Created" column shows "6mo ago" or "Sep 2025". |

**AWS comparison:**

```
aws iam list-policies --scope Local --query 'Policies[].[PolicyName,CreateDate]' --output table
```

Expected field visible: Created (path: CreateDate)

### A.10 IAM Users

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | I open the IAM users list. A user was created 2 years ago and last used their password 1 day ago. | The "Created" column shows "2y ago" or "Mar 2024". The "Password Last Used" column shows "1d ago". |
| A.10.2 | A user has never used a password (PasswordLastUsed is null). | The "Password Last Used" column shows a dash, "n/a", or is empty. |

**AWS comparison:**

```
aws iam list-users --query 'Users[].[UserName,CreateDate,PasswordLastUsed]' --output table
```

Expected fields visible: Created (path: CreateDate), Password Last Used (path: PasswordLastUsed)

### A.11 IAM Groups

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | I open the IAM groups list. A group was created 3 months ago. | The "Created" column shows "3mo ago" or "Dec 2025". |

**AWS comparison:**

```
aws iam list-groups --query 'Groups[].[GroupName,CreateDate]' --output table
```

Expected field visible: Created (path: CreateDate)

### A.12 EKS Clusters

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | I open the EKS cluster list. No timestamp columns are configured in the list view. | EKS list shows Cluster Name, Version, Status, Endpoint, Platform Version. CreatedAt appears only in the detail view. |

### A.13 ECR Repositories

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | I open the ECR repository list. A repository was created 8 months ago. | The "Created" column shows "8mo ago" or "Jul 2025". |

**AWS comparison:**

```
aws ecr describe-repositories --query 'repositories[].[repositoryName,createdAt]' --output table
```

Expected field visible: Created (path: CreatedAt)

### A.14 ELB / Load Balancers

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I open the ELB list. No timestamp columns are configured in the list view. | ELB list shows Name, Type, Scheme, State, DNS Name, VPC ID. CreatedTime appears only in the detail view. |

### A.15 Backup Plans

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | I open the Backup plans list. A plan was created 30 days ago and last executed 2 hours ago. | The "Created" column shows "30d ago" or "Feb 25, 2026". The "Last Execution" column shows "2h ago". |
| A.15.2 | A backup plan has never been executed (LastExecutionDate is null). | The "Last Execution" column shows a dash, "n/a", or is empty. |

**AWS comparison:**

```
aws backup list-backup-plans --query 'BackupPlansList[].[BackupPlanName,CreationDate,LastExecutionDate]' --output table
```

Expected fields visible: Created (path: CreationDate), Last Execution (path: LastExecutionDate)

### A.16 SSM Parameters

| ID | Story | Expected |
|----|-------|----------|
| A.16.1 | I open the SSM parameter list. A parameter was last modified 10 minutes ago. | The "Last Modified" column shows "10m ago" or similar. |

**AWS comparison:**

```
aws ssm describe-parameters --query 'Parameters[].[Name,Type,LastModifiedDate]' --output table
```

Expected field visible: Last Modified (path: LastModifiedDate)

### A.17 Step Functions (sfn)

| ID | Story | Expected |
|----|-------|----------|
| A.17.1 | I open the Step Functions list. A state machine was created 2 weeks ago. | The "Created" column shows "2w ago" or "Mar 13, 2026". |

**AWS comparison:**

```
aws stepfunctions list-state-machines --query 'stateMachines[].[name,creationDate]' --output table
```

Expected field visible: Created (path: CreationDate)

### A.18 Glue Jobs

| ID | Story | Expected |
|----|-------|----------|
| A.18.1 | I open the Glue jobs list. A job was last modified 4 days ago. | The "Last Modified" column shows "4d ago" or "Mar 23, 2026". |

**AWS comparison:**

```
aws glue get-jobs --query 'Jobs[].[Name,LastModifiedOn]' --output table
```

Expected field visible: Last Modified (path: LastModifiedOn)

### A.19 Pipelines (CodePipeline)

| ID | Story | Expected |
|----|-------|----------|
| A.19.1 | I open the CodePipeline list. A pipeline was created 60 days ago and updated 1 hour ago. | The "Created" column shows "60d ago" or "Jan 26, 2026". The "Updated" column shows "1h ago". |

**AWS comparison:**

```
aws codepipeline list-pipelines --query 'pipelines[].[name,created,updated]' --output table
```

Expected fields visible: Created (path: Created), Updated (path: Updated)

### A.20 Kinesis Streams

| ID | Story | Expected |
|----|-------|----------|
| A.20.1 | I open the Kinesis streams list. A stream was created 100 days ago. | The "Created" column shows "100d ago" or "Dec 17, 2025". |

**AWS comparison:**

```
aws kinesis list-streams
```

Expected field visible: Created (path: StreamCreationTimestamp)

### A.21 ACM Certificates

| ID | Story | Expected |
|----|-------|----------|
| A.21.1 | I open the ACM certificates list. A certificate expires in 30 days. | The "Expires" column shows a human-readable date like "Apr 26, 2026" or "in 30d". It does not show a raw ISO timestamp. |

**AWS comparison:**

```
aws acm list-certificates --query 'CertificateSummaryList[].[DomainName,Status,NotAfter]' --output table
```

Expected field visible: Expires (path: NotAfter)

### A.22 CloudWatch Log Groups (logs)

| ID | Story | Expected |
|----|-------|----------|
| A.22.1 | I open the CloudWatch Log Groups list. A log group was created 90 days ago. | The "Created" column shows "90d ago" or "Dec 27, 2025". The creation_time is rendered as human-readable. |

**AWS comparison:**

```
aws logs describe-log-groups --query 'logGroups[].[logGroupName,storedBytes,creationTime]' --output table
```

Expected field visible: Created (key: creation_time)

### A.23 DocDB Snapshots and RDS Snapshots

| ID | Story | Expected |
|----|-------|----------|
| A.23.1 | I open the RDS snapshots list. A snapshot was created 7 days ago. | The "Created" column shows "7d ago" or "Mar 20, 2026". |
| A.23.2 | I open the DocumentDB snapshots list. A snapshot was created 14 days ago. | The "Created" column shows "14d ago" or "Mar 13, 2026". |

**AWS comparison:**

```
aws rds describe-db-snapshots --query 'DBSnapshots[].[DBSnapshotIdentifier,Status,SnapshotCreateTime]' --output table
aws docdb describe-db-cluster-snapshots --query 'DBClusterSnapshots[].[DBClusterSnapshotIdentifier,Status,SnapshotCreateTime]' --output table
```

Expected field visible: Created (path: SnapshotCreateTime)

---

## B. Timestamp Formatting -- Child View Columns

### B.1 CloudTrail Events (ec2_cloudtrail and child views with timestamps)

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I open CloudTrail events for an EC2 instance. An event occurred 15 minutes ago. | The "Time" column shows "15m ago" or a formatted time like "14:08" -- not a raw ISO 8601 timestamp. |

### B.2 Alarm History

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | I drill into alarm history. A state change occurred 2 hours ago. | The "Timestamp" column shows "2h ago" or a formatted time -- not a raw ISO timestamp. |

**AWS comparison:**

```
aws cloudwatch describe-alarm-history --alarm-name ALARM --query 'AlarmHistoryItems[].[Timestamp,HistoryItemType,HistorySummary]'
```

Expected field visible: Timestamp (key: timestamp)

### B.3 ASG Scaling Activities

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | I drill into ASG scaling activities. An activity started 30 minutes ago. | The "Start Time" column shows "30m ago" or a formatted time. |

**AWS comparison:**

```
aws autoscaling describe-scaling-activities --auto-scaling-group-name ASG --query 'Activities[].[StartTime,StatusCode,Description]'
```

Expected field visible: Start Time (key: start_time)

### B.4 CFN Stack Events

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | I drill into CloudFormation stack events. An event occurred 5 minutes ago. | The "Timestamp" column shows "5m ago" or a formatted time. |

**AWS comparison:**

```
aws cloudformation describe-stack-events --stack-name STACK --query 'StackEvents[].[Timestamp,LogicalResourceId,ResourceStatus]'
```

Expected field visible: Timestamp (key: timestamp)

### B.5 CFN Stack Resources

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | I drill into CloudFormation stack resources. A resource was last updated 1 hour ago. | The "Updated" column shows "1h ago" or a formatted time. |

**AWS comparison:**

```
aws cloudformation list-stack-resources --stack-name STACK --query 'StackResourceSummaries[].[LogicalResourceId,ResourceStatus,LastUpdatedTimestamp]'
```

Expected field visible: Updated (key: last_updated)

### B.6 CodeBuild Builds

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | I drill into CodeBuild builds for a project. A build started 45 minutes ago. | The "Start Time" column shows "45m ago" or a formatted time. |

**AWS comparison:**

```
aws codebuild batch-get-builds --ids BUILD_ID --query 'builds[].[buildNumber,buildStatus,startTime]'
```

Expected field visible: Start Time (key: start_time)

### B.7 ECS Service Events

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | I drill into ECS service events. An event occurred 10 seconds ago. | The "Timestamp" column shows "10s ago" or a formatted time. |

**AWS comparison:**

```
aws ecs describe-services --cluster CLUSTER --services SERVICE --query 'services[].events[].[createdAt,message]'
```

Expected field visible: Timestamp (key: timestamp)

### B.8 ECS Tasks (ecs_tasks child view)

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | I drill into ECS service tasks. A task started 3 hours ago. | The "Started At" column shows "3h ago" or a formatted time. |

**AWS comparison:**

```
aws ecs describe-tasks --cluster CLUSTER --tasks TASK_ARN --query 'tasks[].[taskArn,lastStatus,startedAt]'
```

Expected field visible: Started At (key: started_at)

### B.9 ECR Images

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | I drill into ECR images for a repository. An image was pushed 2 days ago. | The "Pushed At" column shows "2d ago" or "Mar 25, 2026". |

**AWS comparison:**

```
aws ecr describe-images --repository-name REPO --query 'imageDetails[].[imageTags,imagePushedAt,imageSizeInBytes]'
```

Expected field visible: Pushed At (key: pushed_at)

### B.10 SFN Executions

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | I drill into Step Function executions. An execution started 20 minutes ago and stopped 18 minutes ago. | The "Start Date" column shows "20m ago" or a formatted time. The "Stop Date" column shows "18m ago" or a formatted time. |

**AWS comparison:**

```
aws stepfunctions list-executions --state-machine-arn ARN --query 'executions[].[name,status,startDate,stopDate]'
```

Expected fields visible: Start Date (path: StartDate), Stop Date (path: StopDate)

### B.11 SFN Execution History

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | I drill into SFN execution history. A history event occurred 19 minutes ago. | The "Timestamp" column shows "19m ago" or a formatted time. |

**AWS comparison:**

```
aws stepfunctions get-execution-history --execution-arn ARN --query 'events[].[timestamp,type,id]'
```

Expected field visible: Timestamp (path: Timestamp)

### B.12 Pipeline Stages

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I drill into pipeline stages. A stage last changed 4 hours ago. | The "Last Changed" column shows "4h ago" or a formatted time. |

**AWS comparison:**

```
aws codepipeline get-pipeline-state --name PIPELINE --query 'stageStates[].actionStates[].[actionName,latestExecution.lastStatusChange]'
```

Expected field visible: Last Changed (key: last_change_time)

### B.13 Log Streams

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I drill into log streams for a log group. A stream's last event was 5 minutes ago. | The "Last Event" column shows "5m ago" or a formatted time. The "First Event" column shows a human-readable time. |

**AWS comparison:**

```
aws logs describe-log-streams --log-group-name GROUP --query 'logStreams[].[logStreamName,lastEventTimestamp,firstEventTimestamp]'
```

Expected fields visible: Last Event (key: last_event), First Event (key: first_event)

### B.14 Log Events, Lambda Invocations, Build Logs, Container Logs

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | I view log events in any log-type child view (log_events, lambda_invocation_logs, cb_build_logs, ecs_svc_logs). A log event occurred 30 seconds ago. | The "Timestamp" column shows "30s ago" or a formatted time like "14:22:30". |
| B.14.2 | I view Lambda invocations. An invocation occurred 1 hour ago. | The "Timestamp" column shows "1h ago" or a formatted time. |

**AWS comparison:**

```
aws logs filter-log-events --log-group-name GROUP --query 'events[].[timestamp,message]'
```

Expected field visible: Timestamp (key: timestamp)

### B.15 RDS / DocumentDB Instance Events

| ID | Story | Expected |
|----|-------|----------|
| B.15.1 | I drill into RDS instance events. An event occurred 6 hours ago. | The "Timestamp" column shows "6h ago" or a formatted time. |

**AWS comparison:**

```
aws rds describe-events --source-identifier DB_INSTANCE --source-type db-instance --query 'Events[].[Date,Message]'
```

Expected field visible: Timestamp (path: Date)

### B.16 IAM Group Members

| ID | Story | Expected |
|----|-------|----------|
| B.16.1 | I drill into IAM group members. A user was created 1 year ago and last used their password 2 days ago. | The "Created" column shows "1y ago". The "Password Last Used" column shows "2d ago". |

**AWS comparison:**

```
aws iam get-group --group-name GROUP --query 'Users[].[UserName,CreateDate,PasswordLastUsed]'
```

Expected fields visible: Created (key: create_date), Password Last Used (key: password_last_used)

### B.17 S3 Objects

| ID | Story | Expected |
|----|-------|----------|
| B.17.1 | I drill into an S3 bucket. An object was last modified 3 days ago. | The "Last Modified" column shows "3d ago" or "Mar 24, 2026". |

**AWS comparison:**

```
aws s3api list-objects-v2 --bucket BUCKET --query 'Contents[].[Key,Size,LastModified]'
```

Expected field visible: Last Modified (path: LastModified)

---

## C. Size Formatting -- List View Columns

### C.1 S3 Objects -- Size Column

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I drill into an S3 bucket. An object is 1,048,576 bytes. | The "Size" column shows "1.0 MB" instead of "1048576". |
| C.1.2 | An object is 512 bytes. | The "Size" column shows "512 B" or "0.5 KB". |
| C.1.3 | An object is 5,368,709,120 bytes (5 GB). | The "Size" column shows "5.0 GB". |
| C.1.4 | An object is 0 bytes (empty file). | The "Size" column shows "0 B" or "0". |
| C.1.5 | I verify sizes against `aws s3api list-objects-v2 --bucket BUCKET`. | The "Size" column maps to `.Contents[].Size`. The human-readable value accurately represents the byte count. |

**AWS comparison:**

```
aws s3api list-objects-v2 --bucket BUCKET --query 'Contents[].[Key,Size]' --output table
```

Expected field visible: Size (path: Size)

### C.2 ECR Images -- Size Column

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | I drill into ECR images. An image is 157,286,400 bytes (~150 MB). | The "Size" column shows "150.0 MB" or "150 MB" instead of "157286400". |
| C.2.2 | An image is 1,073,741,824 bytes (~1 GB). | The "Size" column shows "1.0 GB". |
| C.2.3 | I verify sizes against `aws ecr describe-images`. | The "Size" column maps to `.imageDetails[].imageSizeInBytes`. |

**AWS comparison:**

```
aws ecr describe-images --repository-name REPO --query 'imageDetails[].[imageTags,imageSizeInBytes]' --output table
```

Expected field visible: Size (key: image_size)

### C.3 DynamoDB Tables -- Size Column

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | I open the DynamoDB table list. A table is 2,684,354,560 bytes (~2.5 GB). | The "Size" column shows "2.5 GB" instead of the raw byte count. |
| C.3.2 | A table is 0 bytes (empty table). | The "Size" column shows "0 B" or "0". |
| C.3.3 | I verify sizes against `aws dynamodb describe-table`. | The "Size" column maps to `.Table.TableSizeBytes`. |

**AWS comparison:**

```
aws dynamodb list-tables
aws dynamodb describe-table --table-name TABLE --query 'Table.[TableName,TableSizeBytes,ItemCount]'
```

Expected field visible: Size (key: size_bytes)

### C.4 CloudWatch Log Groups -- Size Column

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | I open the CloudWatch Log Groups list. A log group has 10,737,418,240 stored bytes (~10 GB). | The "Size" column shows "10.0 GB" instead of the raw byte count. |
| C.4.2 | A log group has 0 stored bytes. | The "Size" column shows "0 B" or "0". |
| C.4.3 | I verify sizes against `aws logs describe-log-groups`. | The "Size" column maps to `.logGroups[].storedBytes`. |

**AWS comparison:**

```
aws logs describe-log-groups --query 'logGroups[].[logGroupName,storedBytes]' --output table
```

Expected field visible: Size (key: stored_bytes)

### C.5 Lambda Functions -- CodeSize in Detail View

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | I press `d` on a Lambda function. The function's code is 52,428,800 bytes (~50 MB). | The "CodeSize" field in the detail view shows "50.0 MB" or "50 MB" instead of "52428800". |

**AWS comparison:**

```
aws lambda get-function --function-name FUNC --query 'Configuration.CodeSize'
```

Expected field visible: CodeSize (in detail view)

### C.6 EFS -- SizeInBytes in Detail View

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | I press `d` on an EFS file system. The file system size is 1,073,741,824 bytes (~1 GB). | The "SizeInBytes" field in the detail view shows a human-readable value like "1.0 GB". |

**AWS comparison:**

```
aws efs describe-file-systems --query 'FileSystems[].[FileSystemId,SizeInBytes]'
```

Expected field visible: SizeInBytes (in detail view)

---

## D. Timestamp Formatting -- Detail View Fields

### D.1 Timestamps in Detail Views Show Human-Readable Format

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I press `d` on an EC2 instance. The LaunchTime field is present. | The "LaunchTime" value shows a human-readable format (e.g., "Mar 15, 2026, 09:22 AM" or "2026-03-15 09:22 (12d ago)") rather than raw "2026-03-15T09:22:45Z". |
| D.1.2 | I press `d` on an S3 bucket. CreationDate is present. | The "CreationDate" value shows a human-readable format. |
| D.1.3 | I press `d` on a CloudFormation stack. CreationTime and LastUpdatedTime are present. | Both show human-readable formats. |
| D.1.4 | I press `d` on a Secrets Manager secret. LastAccessedDate, LastChangedDate, CreatedDate, LastRotatedDate are present. | All timestamp fields show human-readable formats. |
| D.1.5 | I press `d` on an EKS cluster. CreatedAt is present. | The "CreatedAt" value shows a human-readable format. |
| D.1.6 | I press `d` on an IAM role. CreateDate is present. RoleLastUsed.LastUsedDate is present. | Both timestamp fields show human-readable formats. |
| D.1.7 | I press `d` on an RDS instance. No timestamp fields are null. | All timestamp fields (if present) show human-readable formats in the detail view. |
| D.1.8 | I press `d` on a CloudWatch alarm. StateUpdatedTimestamp and StateTransitionedTimestamp are present. | Both show human-readable formats. |
| D.1.9 | I press `d` on a KMS key. CreationDate is present. | The "CreationDate" value shows a human-readable format. |
| D.1.10 | I press `d` on a NAT gateway. CreateTime is present. | The "CreateTime" value shows a human-readable format. |

---

## E. YAML View -- Timestamp and Size Rendering

### E.1 YAML View Preserves Original Values

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | I press `y` on an EC2 instance with a LaunchTime field. | The YAML view shows timestamps in their original API format (ISO 8601 such as "2026-03-15T09:22:45Z") since YAML views display raw resource data for copy-paste and debugging. |
| E.1.2 | I press `y` on an S3 object with a Size field. | The YAML view shows the raw byte count (e.g., "1048576") rather than humanized "1.0 MB", preserving the exact API response data. |
| E.1.3 | I copy the YAML content and use it in an automation script. | The raw values from the YAML view can be parsed programmatically without needing to reverse human-readable formatting. |

---

## F. Formatting Consistency and Edge Cases

### F.1 Relative Time Boundaries

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | A timestamp is from 30 seconds ago. | The formatted value shows "30s ago" or "just now". |
| F.1.2 | A timestamp is from 59 minutes ago. | The formatted value shows "59m ago". |
| F.1.3 | A timestamp is from 23 hours ago. | The formatted value shows "23h ago". |
| F.1.4 | A timestamp is from 6 days ago. | The formatted value shows "6d ago". |
| F.1.5 | A timestamp is from 29 days ago. | The formatted value shows "29d ago". |
| F.1.6 | A timestamp is from 90 days ago. | The formatted value shows "90d ago" or "3mo ago" or a date like "Dec 27, 2025". |
| F.1.7 | A timestamp is from 400 days ago. | The formatted value shows "1y ago" or a date like "Feb 2025". |
| F.1.8 | A timestamp is in the future (e.g., ACM NotAfter expiry). | The formatted value shows "in 30d" or the formatted date "Apr 26, 2026". Future timestamps are not shown as negative relative times. |

### F.2 Size Formatting Boundaries

| ID | Story | Expected |
|----|-------|----------|
| F.2.1 | A size value is 0 bytes. | The display shows "0 B" or "0". |
| F.2.2 | A size value is 999 bytes. | The display shows "999 B". |
| F.2.3 | A size value is 1,024 bytes. | The display shows "1.0 KB". |
| F.2.4 | A size value is 1,048,576 bytes. | The display shows "1.0 MB". |
| F.2.5 | A size value is 1,073,741,824 bytes. | The display shows "1.0 GB". |
| F.2.6 | A size value is 1,099,511,627,776 bytes. | The display shows "1.0 TB". |
| F.2.7 | A size value is 1,500 bytes. | The display shows "1.5 KB" (one decimal place of precision). |

### F.3 Null and Missing Values

| ID | Story | Expected |
|----|-------|----------|
| F.3.1 | A timestamp field is null or absent in the API response (e.g., CloudFormation LastUpdatedTime on a never-updated stack). | The column or detail field shows a dash "-", "n/a", or is empty. No crash, no "null" text, no "0s ago". |
| F.3.2 | A size field is null or absent in the API response. | The column or detail field shows a dash "-", "n/a", or is empty. No crash. |
| F.3.3 | A timestamp field contains an unexpected format (non-ISO 8601, non-Unix). | The value is displayed as-is (passthrough) rather than crashing. The application degrades gracefully. |

### F.4 Sorting Preserves Correctness

| ID | Story | Expected |
|----|-------|----------|
| F.4.1 | I press `A` (sort by age) on the EC2 instance list where Launch Times show "2h ago", "5d ago", "30d ago". | Rows are sorted by the underlying timestamp value, not by the displayed string. "2h ago" is newest, "30d ago" is oldest. Ascending sort places "30d ago" first. |
| F.4.2 | I press `N` (sort by name) on the S3 objects list where sizes show "1.0 KB", "500 B", "2.0 GB". | Sort by name is alphabetical on the key column and is unaffected by size formatting. |
| F.4.3 | I filter by typing "ago" in any view with relative timestamps. | Filtering matches the displayed text. If timestamps show "2h ago", typing "ago" should match those rows. |

---

## G. Cross-Cutting: Format Consistency Across All Views

### G.1 All Resource Types with Timestamp Columns

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | I systematically open every resource type that has a timestamp column in views.yaml and verify formatting. | Every timestamp column across all resource types (EC2 Launch Time, S3 Creation Date, CFN Created/Updated, Lambda Last Modified, Secrets Last Accessed/Last Changed, IAM Created/Last Used, ECR Created, Backup Created/Last Execution, SSM Last Modified, SFN Created, Glue Last Modified, Pipeline Created/Updated, Kinesis Created, ACM Expires, Logs Created, RDS Snapshots Created, DocDB Snapshots Created) displays a human-readable value. No raw ISO 8601 or Unix timestamps are visible in any list view. |

### G.2 All Child Views with Timestamp Columns

| ID | Story | Expected |
|----|-------|----------|
| G.2.1 | I systematically drill into every child view that has a timestamp column and verify formatting. | Every timestamp column in child views (alarm_history Timestamp, asg_activities Start Time, cfn_events Timestamp, cfn_resources Updated, cb_builds Start Time, ecs_svc_events Timestamp, ecs_tasks Started At, ecr_images Pushed At, sfn_executions Start Date/Stop Date, sfn_execution_history Timestamp, pipeline_stages Last Changed, log_streams Last Event/First Event, log_events Timestamp, lambda_invocations Timestamp, dbi_events Timestamp, iam_group_members Created/Password Last Used, s3_objects Last Modified) displays a human-readable value. |

### G.3 All Views with Size Columns

| ID | Story | Expected |
|----|-------|----------|
| G.3.1 | I systematically open every view that has a size column and verify formatting. | Every size column (S3 Objects Size, ECR Images Size, DynamoDB Size, CloudWatch Logs Size, Lambda CodeSize in detail, EFS SizeInBytes in detail) displays a human-readable value with appropriate units (B, KB, MB, GB, TB). No raw byte counts are visible. |
