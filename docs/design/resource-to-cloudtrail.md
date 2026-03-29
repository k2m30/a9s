# Resource-to-CloudTrail Navigation Design Spec

Issue: #114 (references #112 ct-search)
Version: 1.0
Target: a9s v3.25+
Status: Design

---

## 1. Overview

From ANY resource view in a9s, pressing `T` (uppercase, mnemonic: "Trail") opens
a filtered CloudTrail search view showing ONLY events related to that specific
resource or identity. This is the inverse of ct-search's cross-resource navigation
(Section 7.6 of cloudtrail-search.md): instead of going from a CloudTrail event
TO a resource, the user goes from a resource TO its CloudTrail events.

### Why This Matters

Every AWS resource has a story: who created it, who modified it, who broke it.
Today, answering "what happened to this resource?" requires leaving a9s, opening
the AWS Console, navigating to CloudTrail, and manually constructing a filter.
This feature makes that investigation a single keypress.

### Entry Points

- `T` key from any resource list view (with a resource selected)
- `T` key from any resource detail view
- Both produce the same result: open ct-search pre-populated with filters derived
  from the selected resource

### Relationship to ct-search (#112)

This feature is an **entry point** into ct-search, not a separate view. It reuses
the ct-search three-screen flow (form -> results -> detail) entirely. The only
difference is that the search form arrives pre-populated with filters and the
search executes immediately (like a ct-search preset).

```
RESOURCE LIST/DETAIL       CT-SEARCH (pre-filled)       CT-SEARCH RESULTS
  [user presses T]    -->   form shows filters      -->  events for resource
                            (auto-executes search)
                            user can modify filters
                            from here
```

---

## 2. Key Binding: `T` (uppercase)

### Key Binding Analysis

| Considered | Verdict | Reason |
|------------|---------|--------|
| `R` | Rejected | Used in ct-search event detail for "Copy resource ARN" (Section 8.3 of cloudtrail-search.md) |
| `t` | Rejected | Used in log views for "Toggle timestamp display" (child-views/README.md) |
| `T` | **Chosen** | Completely unbound in all views. Mnemonic: "Trail" (CloudTrail). Uppercase avoids conflict with `t` in log views. |
| `H` | Rejected | Potential future use for "History". Less intuitive mnemonic. |
| `ctrl+t` | Rejected | Some terminals intercept ctrl+t. Less discoverable. |

### Availability by View Context

| View | `T` currently bound? | Can use `T`? |
|------|---------------------|--------------|
| Main menu | No | Yes (no resource selected -- no-op with flash hint) |
| Resource list | No | Yes (primary use case) |
| Detail view | No | Yes (from the detailed resource) |
| YAML view | No | Yes (from the resource being viewed) |
| ct-search form | No | No (already in ct-search; would be confusing) |
| ct-search results | No | No (already in ct-search) |
| ct-search detail | No | No (already in ct-search) |
| Help overlay | No | Dismissed first, then T applies |
| Identity view | No | Yes (show events for the caller identity itself) |

### Key Behavior

- From **resource list**: uses the currently selected (highlighted) resource
- From **detail/YAML view**: uses the resource whose detail/YAML is being shown
- From **main menu**: flash message "Select a resource first" (no navigation)
- From **ct-search**: key is not bound (user is already investigating CloudTrail)

---

## 3. Resource-Type-to-Filter Mapping

The core of this feature: given a resource type and a specific resource, how do
we derive the CloudTrail filter?

### 3.1 Primary Mapping Table

| Resource Type | Short Name | Filter Type | Filter Field | Value Source | Example Filter Value | Common Investigation |
|---------------|------------|-------------|--------------|--------------|---------------------|----------------------|
| EC2 Instances | ec2 | Resource ARN | ResourceARN | Constructed: `arn:aws:ec2:{region}:{account}:instance/{ID}` | `arn:aws:ec2:us-east-1:123456789012:instance/i-0abc123def456` | Who stopped/terminated this instance? |
| S3 Buckets | s3 | Resource ARN | ResourceARN | Constructed: `arn:aws:s3:::{Name}` | `arn:aws:s3:::prod-data-bucket` | Who changed this bucket policy? |
| Lambda Functions | lambda | Resource ARN | ResourceARN | Fields["FunctionArn"] or constructed | `arn:aws:lambda:us-east-1:123456789012:function:api-handler` | Who changed this function's config? |
| Security Groups | sg | Resource ARN | ResourceARN | Constructed: `arn:aws:ec2:{region}:{account}:security-group/{ID}` | `arn:aws:ec2:us-east-1:123456789012:security-group/sg-0abc123` | Who modified these rules? |
| IAM Roles | role | Username | Username | Resource.Name (role name) | `ci-deploy-role` | What has this role been doing? |
| IAM Users | iam-user | Username | Username | Resource.Name (user name) | `admin` | What has this user been doing? |
| IAM Policies | policy | Resource ARN | ResourceARN | Fields["Arn"] | `arn:aws:iam::123456789012:policy/ReadOnlyAccess` | Who attached/detached this policy? |
| IAM Groups | iam-group | Resource ARN | ResourceARN | Constructed: `arn:aws:iam::{account}:group/{Name}` | `arn:aws:iam::123456789012:group/developers` | Who modified group membership? |
| RDS Instances | dbi | Resource ARN | ResourceARN | Fields["DBInstanceArn"] | `arn:aws:rds:us-east-1:123456789012:db:prod-mysql` | Who modified this database? |
| Redis | redis | Resource ARN | ResourceARN | Fields["ARN"] | `arn:aws:elasticache:us-east-1:123456789012:cluster:sessions` | Who modified this cluster? |
| DocumentDB | dbc | Resource ARN | ResourceARN | Fields["DBClusterArn"] | `arn:aws:rds:us-east-1:123456789012:cluster:docdb-prod` | Who modified this cluster? |
| DynamoDB Tables | ddb | Resource ARN | ResourceARN | Fields["TableArn"] | `arn:aws:dynamodb:us-east-1:123456789012:table/users` | Who changed table settings? |
| EKS Clusters | eks | Resource ARN | ResourceARN | Fields["Arn"] | `arn:aws:eks:us-east-1:123456789012:cluster/prod` | Who updated cluster config? |
| ECS Services | ecs-svc | Resource ARN | ResourceARN | Fields["ServiceArn"] | `arn:aws:ecs:us-east-1:123456789012:service/prod/api` | Who changed service definition? |
| ECS Clusters | ecs | Resource ARN | ResourceARN | Fields["ClusterArn"] | `arn:aws:ecs:us-east-1:123456789012:cluster/prod` | Who modified this cluster? |
| Load Balancers | elb | Resource ARN | ResourceARN | Fields["LoadBalancerArn"] | `arn:aws:elasticloadbalancing:...` | Who modified listener rules? |
| Target Groups | tg | Resource ARN | ResourceARN | Fields["TargetGroupArn"] | `arn:aws:elasticloadbalancing:...` | Who changed target health checks? |
| Secrets Manager | secrets | Resource ARN | ResourceARN | Fields["ARN"] | `arn:aws:secretsmanager:us-east-1:...:secret:prod/db-xxx` | Who rotated/accessed this secret? |
| SSM Parameters | ssm | Resource ARN | ResourceARN | Constructed: `arn:aws:ssm:{region}:{account}:parameter/{Name}` | `arn:aws:ssm:us-east-1:...:parameter/prod/api-key` | Who changed this parameter? |
| SQS Queues | sqs | Resource ARN | ResourceARN | Fields["QueueArn"] | `arn:aws:sqs:us-east-1:123456789012:order-processing` | Who modified queue attributes? |
| SNS Topics | sns | Resource ARN | ResourceARN | Fields["TopicArn"] | `arn:aws:sns:us-east-1:123456789012:alerts` | Who changed subscriptions? |
| CloudFormation | cfn | Resource ARN | ResourceARN | Fields["StackId"] | `arn:aws:cloudformation:us-east-1:...:stack/prod-app/...` | Who updated/deleted this stack? |
| VPCs | vpc | Resource ARN | ResourceARN | Constructed: `arn:aws:ec2:{region}:{account}:vpc/{ID}` | `arn:aws:ec2:us-east-1:...:vpc/vpc-0abc123` | Who modified VPC attributes? |
| Auto Scaling | asg | Resource ARN | ResourceARN | Fields["AutoScalingGroupARN"] | `arn:aws:autoscaling:...` | Who changed scaling policies? |
| CW Alarms | alarm | Resource ARN | ResourceARN | Fields["AlarmArn"] | `arn:aws:cloudwatch:...` | Who modified this alarm? |
| CW Log Groups | logs | Resource ARN | ResourceARN | Fields["Arn"] | `arn:aws:logs:...` | Who changed retention policy? |
| KMS Keys | kms | Resource ARN | ResourceARN | Fields["Arn"] | `arn:aws:kms:...` | Who changed key policy? |
| CloudFront | cf | Resource ARN | ResourceARN | Fields["ARN"] | `arn:aws:cloudfront::...:distribution/E123` | Who updated distribution config? |
| ACM Certificates | acm | Resource ARN | ResourceARN | Fields["CertificateArn"] | `arn:aws:acm:...` | Who requested/deleted this cert? |
| Route 53 Zones | r53 | Resource ARN | ResourceARN | Fields["Id"] -> construct ARN | `arn:aws:route53:::hostedzone/Z123` | Who changed DNS records? |
| API Gateway | apigw | Resource ARN | ResourceARN | Constructed from ID | `arn:aws:apigateway:...` | Who deployed/changed this API? |
| WAF | waf | Resource ARN | ResourceARN | Fields["ARN"] | `arn:aws:wafv2:...` | Who modified WAF rules? |
| ECR Repos | ecr | Resource ARN | ResourceARN | Fields["RepositoryArn"] | `arn:aws:ecr:...` | Who changed repo policy? |
| CodeBuild | cb | Resource ARN | ResourceARN | Fields["Arn"] | `arn:aws:codebuild:...` | Who modified build project? |
| CodePipeline | pipeline | Resource ARN | ResourceARN | Constructed from name | `arn:aws:codepipeline:...` | Who modified this pipeline? |
| Step Functions | sfn | Resource ARN | ResourceARN | Fields["StateMachineArn"] | `arn:aws:states:...` | Who updated state machine? |
| Glue Jobs | glue | Event Source | EventSource + client filter | `glue.amazonaws.com` + client-side Name filter | Event Source + resource name | Who modified this Glue job? |
| Athena | athena | Resource ARN | ResourceARN | Fields["WorkGroupArn"] or constructed | `arn:aws:athena:...` | Who changed workgroup config? |
| Backup Plans | backup | Resource ARN | ResourceARN | Fields["BackupPlanArn"] | `arn:aws:backup:...` | Who modified backup schedule? |

### 3.2 IAM Dual-Mode Filter

IAM Roles and IAM Users are special because they can appear in CloudTrail events
in two ways:

1. **As the actor** (who performed the action) -- filter by Username
2. **As the target** (what was acted upon) -- filter by Resource ARN

When pressing `T` on an IAM Role or IAM User, the ct-search form opens with the
**Username filter pre-filled** (actor mode, the more common investigation). The
user can then manually switch to Resource ARN mode by:

1. Clearing the Username field
2. Typing the ARN in the Resource ARN field

To make this easier, the ct-search form shows a hint when opened from an IAM
resource:

```
Showing events BY this role. To see events ON this role, clear Username
and enter the role ARN in Resource ARN.
```

This hint appears in dim italic (`#565f89` italic) below the filter fields,
only when ct-search was triggered from an IAM Role or IAM User.

### 3.2.1 IAM Dual-Mode Scenarios

The following table clarifies when each mode (actor vs. target) matters for
IAM Roles and Users:

| Scenario | Mode | Why |
|----------|------|-----|
| "What is this role doing?" (compromise/audit) | Actor (default) | See all API calls BY the role |
| "Who changed this role's permissions?" | Target (via `f`) | See AttachRolePolicy, PutRolePolicy ON the role |
| "Why can't this role access X?" | Actor + error filter | Look for AccessDenied errors |
| "Is this role being used at all?" | Actor | Zero results = unused, safe to delete |

### 3.3 ARN Construction

Some resources store their ARN in a field; others require construction. The
implementation must handle both cases:

**ARN from field** (preferred -- use when available):
- Lambda: `Fields["FunctionArn"]`
- RDS: `Fields["DBInstanceArn"]`
- EKS: `Fields["Arn"]`
- DynamoDB: `Fields["TableArn"]`
- Secrets: `Fields["ARN"]`
- SQS: `Fields["QueueArn"]`
- SNS: `Fields["TopicArn"]`
- etc.

**ARN constructed** (when no ARN field exists):
- EC2: `arn:aws:ec2:{region}:{account}:instance/{resource.ID}`
- S3: `arn:aws:s3:::{resource.Name}`
- SG: `arn:aws:ec2:{region}:{account}:security-group/{resource.ID}`
- VPC: `arn:aws:ec2:{region}:{account}:vpc/{resource.ID}`
- SSM: `arn:aws:ssm:{region}:{account}:parameter/{resource.Name}`

The region and account are available from the active profile context (already
in the header).

### 3.4 Fallback Behavior

If a resource type has no mapping (e.g., a future resource type added without
updating this mapping), pressing `T` opens ct-search with:
- Time range: 24h
- All filters empty
- Flash hint: "No automatic filter for {resource_type}. Enter filters manually."

This ensures `T` always does something useful, even for unmapped types.

---

## 4. Default Search Parameters

When `T` opens ct-search pre-populated from a resource:

| Parameter | Default Value | Rationale |
|-----------|---------------|-----------|
| Time range | 24h | Balances recency vs. coverage; most operational questions are about recent changes |
| Write-only toggle | ON | "What changed?" is the most common question when investigating a resource |
| Error-only toggle | OFF | Don't filter out successful mutations -- they're often what you're looking for |
| Auto-execute | Yes | Immediately runs the search (like ct-search presets) |

The user lands on the results screen directly. They can press `f` to go back
to the form and modify filters if needed (same as ct-search Section 8.2).

---

## 4.1 CloudTrail Delivery Delay

CloudTrail events can take **up to 15 minutes** to appear after the API call
occurs. This is an AWS-side limitation, not an a9s limitation. If the user
presses `T` immediately after making a change and the most recent event is
missing, it has not yet been delivered.

The results header should include a subtle note when the result count is low
(fewer than 5 events) or when the most recent event is older than 15 minutes:

```
Events may be delayed up to 15 min
```

This text renders in dim (`#565f89`) at the right edge of the frame title or
as a one-line note below the header row, consistent with the "Navigated from"
breadcrumb styling.

---

## 4.2 Data Events Caveat

CloudTrail records two categories of events, and this distinction significantly
affects what users will see when pressing `T`:

**Management events** (always logged by default):
- CreateBucket, PutBucketPolicy, DeleteBucket
- TerminateInstances, RunInstances, StopInstances
- UpdateFunctionConfiguration, UpdateFunctionCode
- CreateTable, DeleteTable, UpdateTable
- CreateKey, DisableKey, ScheduleKeyDeletion
- CreateAccessKey, AttachRolePolicy, CreateRole

**Data events** (NOT logged by default -- require explicit trail configuration):
- S3: GetObject, PutObject, DeleteObject
- Lambda: Invoke
- DynamoDB: GetItem, PutItem, Query, Scan, DeleteItem
- KMS: Encrypt, Decrypt, GenerateDataKey

When a user presses `T`, they will **only see management events** unless data
events have been explicitly enabled in their CloudTrail trail configuration. This
has important implications per resource type:

| Resource | Management Events (always visible) | Data Events (NOT visible by default) | Impact |
|----------|-----------------------------------|--------------------------------------|--------|
| S3 | CreateBucket, PutBucketPolicy, PutBucketEncryption | GetObject, PutObject, DeleteObject | "Who accessed my data?" shows nothing without data events |
| Lambda | UpdateFunctionConfiguration, UpdateFunctionCode, CreateFunction | Invoke | Invocations not visible, only config/code changes |
| DynamoDB | CreateTable, UpdateTable, DeleteTable | GetItem, PutItem, Query, Scan | Item-level operations not visible, only table management |
| KMS | CreateKey, DisableKey, PutKeyPolicy | Encrypt, Decrypt, GenerateDataKey | Key usage not visible, only key management |
| Secrets Manager | CreateSecret, UpdateSecret, DeleteSecret, RotateSecret | -- | `GetSecretValue` IS a management event but is a READ -- hidden by default write-only toggle |

**Special case -- Secrets Manager**: `GetSecretValue` is classified as a
management event (always logged), but it is a read operation. With the default
write-only toggle ON, these reads are hidden. Users investigating secret access
should toggle write-only OFF via the form (`f` key).

### Empty Results Hint

When the result list is empty for resource types known to have significant data
event coverage (S3, Lambda, DynamoDB, KMS), the status area should show a
contextual hint:

```
No events found. S3 data events require explicit CloudTrail trail configuration.
```

The hint text varies by resource type:

| Resource | Hint |
|----------|------|
| S3 | "No events found. S3 data events require explicit CloudTrail trail configuration." |
| Lambda | "No events found. Lambda invocation events require explicit CloudTrail trail configuration." |
| DynamoDB | "No events found. DynamoDB item-level events require explicit CloudTrail trail configuration." |
| KMS | "No events found. KMS encryption/decryption events require explicit CloudTrail trail configuration." |
| Secrets Manager | "No events found. Secret reads are filtered by write-only mode. Press f to toggle." |
| (all others) | "No events found in the last 24h." (standard hint) |

This hint renders in dim italic (`#565f89` italic), consistent with the IAM
dual-mode hint and "Navigated from" breadcrumb.

---

## 4.3 Killer Workflows

These multi-press sequences demonstrate the feature's real-world value. Each
represents a common operational scenario that previously required leaving a9s
and navigating the AWS Console manually.

### Workflow 1: "Who killed my instance?" (3 presses)

```
EC2 list --> select terminated instance --> T
```

Result: See `TerminateInstances` by `intern-deploy-role` at 2:47am. Incident
identified in under 5 seconds.

### Workflow 2: "Is this role compromised?" (3 presses + scroll)

```
IAM Roles --> select suspicious role --> T --> scroll through 24h of events
```

Result: Scan for unusual patterns -- `CreateAccessKey`, `AttachUserPolicy`,
`AssumeRole` calls from unusual source IPs. The actor-mode default shows
everything the role has done.

### Workflow 3: "Who opened the firewall?" (3 presses)

```
SG list --> select SG with 0.0.0.0/0 --> T
```

Result: See `AuthorizeSecurityGroupIngress` by `developer-role`. Identify the
person, the time, and the exact rule that was added.

### Workflow 4: "Why did the deploy break?" (4 presses)

```
Lambda list --> select broken function --> T --> Enter on suspicious event
```

Result: See `UpdateFunctionConfiguration` event detail showing timeout changed
from 30s to 3s. The event detail view (from ct-search) shows the full request
parameters.

### Workflow 5: "Cross-resource chase" (6+ presses)

```
EC2 --> T --> see SG change by platform-role --> Esc --> navigate to SG --> T
```

Result: Follow the trail across resources. See who created the security group,
then who modified it. The view stack makes it easy to jump back and forth.

---

## 5. Wireframes

### 5.1 Triggering from EC2 Instance List

The user is viewing the EC2 instance list with `api-prod-01` selected. They
press `T` to investigate CloudTrail events for that instance.

```
 a9s v3.25.0  prod-admin:us-east-1                                 ? for help
┌──────────────────── ec2-instances(42) ────────────────────────────────────────┐
│ NAME                 STATUS      TYPE       AZ           LAUNCH TIME          │
│ api-prod-01          running     t3.medium  us-east-1a   2026-03-28 09:22    │
│ api-prod-02          running     t3.medium  us-east-1b   2026-03-28 09:22    │
│ worker-01            running     m5.large   us-east-1a   2026-03-15 14:30    │
│ bastion              stopped     t3.micro   us-east-1a   2026-01-10 08:00    │
│ dev-test-03          terminated  t3.small   us-east-1c   2026-03-20 11:15    │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

User presses `T`. The view transitions immediately to ct-search results:

### 5.2 Resulting Filtered ct-search View (EC2)

```
 a9s v3.25.0  prod-admin:us-east-1                                 ? for help
┌── ct-search(8) -- i-0abc123..., write events, last 24h ─────────────────────┐
│ TIME                  EVENT NAME                USER              SOURCE     │
│ 2026-03-29 06:15:42   ModifyInstanceAttribute   ci-deploy         ec2.amaz… │
│ 2026-03-29 04:30:11   StopInstances             admin             ec2.amaz… │
│ 2026-03-29 04:35:22   StartInstances            admin             ec2.amaz… │
│ 2026-03-28 22:10:05   ModifyInstanceAttribute   platform-bot      ec2.amaz… │
│ 2026-03-28 18:45:33   CreateTags                ci-deploy         ec2.amaz… │
│ 2026-03-28 14:20:15   RunInstances              ci-deploy         ec2.amaz… │
│ 2026-03-28 11:05:44   AuthorizeSecurityGrou…    platform-bot      ec2.amaz… │
│ 2026-03-28 09:22:01   RunInstances              ci-deploy         ec2.amaz… │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

Frame title: `ct-search(8) -- i-0abc123..., write events, last 24h`
- The resource ID is truncated with `...` to fit
- "write events" reflects the Write-only toggle being ON
- "last 24h" reflects the default time range

### 5.3 Triggering from IAM Role List (Actor Mode)

```
 a9s v3.25.0  prod-admin:us-east-1                                 ? for help
┌──────────────────── iam-roles(156) ──────────────────────────────────────────┐
│ NAME                              CREATED               LAST USED           │
│ ci-deploy-role                    2025-06-15 10:00      2026-03-29 14:52    │
│ lambda-api-handler-role           2025-09-01 08:30      2026-03-29 14:50    │
│ admin-role                        2024-01-01 00:00      2026-03-29 12:00    │
│ ...                                                                          │
└──────────────────────────────────────────────────────────────────────────────┘
```

User presses `T` on `ci-deploy-role`. Result:

```
 a9s v3.25.0  prod-admin:us-east-1                                 ? for help
┌── ct-search(23) -- ci-deploy-role, write events, last 24h ──────────────────┐
│ TIME                  EVENT NAME                SOURCE             RESOURCE  │
│ 2026-03-29 14:52:44   RunInstances              ec2.amazonaws.com  i-0abc1… │
│ 2026-03-29 14:50:19   UpdateFunctionConfigu…    lambda.amazonaws…  api-han… │
│ 2026-03-29 14:45:01   AssumeRole                sts.amazonaws.com  ci-depl… │
│ 2026-03-29 12:30:00   PutBucketPolicy           s3.amazonaws.com   prod-da… │
│ 2026-03-29 10:15:22   CreateDeployment          codedeploy.amazon… prod-ap… │
│ 2026-03-29 08:00:11   UpdateService             ecs.amazonaws.com  api/pro… │
│ 2026-03-28 22:10:45   UpdateFunctionCode        lambda.amazonaws…  api-han… │
│ 2026-03-28 20:03:11   CreateDeployment          codedeploy.amazon… prod-ap… │
│   . . . (15 more)                                                            │
│-- M: load more --                                                            │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

Note: column layout changes to show SOURCE and RESOURCE instead of USER (since
the user is already filtered -- showing the username again wastes space). This
follows ct-search's adaptive column configuration (Section 6.6).

### 5.4 Triggering from S3 Bucket Detail View

```
 a9s v3.25.0  prod-admin:us-east-1                                 ? for help
┌──────────────── s3 -- prod-data-bucket ──────────────────────────────────────┐
│                                                                              │
│ Bucket:                                                                      │
│  Name:               prod-data-bucket                                        │
│  Region:             us-east-1                                               │
│  CreationDate:       2025-03-15 10:30:00                                     │
│  Versioning:         Enabled                                                 │
│  Encryption:         AES256                                                  │
│  PublicAccess:       All blocked                                             │
│                                                                              │
│ Tags:                                                                        │
│  Environment:        production                                              │
│  Team:               platform                                                │
│  CostCenter:         engineering                                             │
│                                                                              │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

User presses `T`. Result shows events for the bucket ARN:

```
 a9s v3.25.0  prod-admin:us-east-1                                 ? for help
┌── ct-search(5) -- prod-data-bucket, write events, last 24h ────────────────┐
│ TIME                  EVENT NAME             USER              ERROR        │
│ 2026-03-29 14:58:31   PutBucketPolicy        lambda-role       AccessDen…  │
│ 2026-03-29 11:20:05   PutBucketTagging       admin                          │
│ 2026-03-28 22:45:12   PutBucketVersioning    platform-bot                   │
│ 2026-03-28 16:10:33   PutBucketEncryption    ci-deploy                      │
│ 2026-03-28 09:30:00   CreateBucket           ci-deploy                      │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 5.5 Modifying Filters After Navigation

From the results view, user presses `f` to open the form. The form shows the
pre-populated filters with the ability to modify:

```
 a9s v3.25.0  prod-admin:us-east-1                                 ? for help
┌──────────────────────── ct-search ───────────────────────────────────────────┐
│                                                                              │
│  TIME RANGE                                                                  │
│  [15m] [ 1h ] [ 4h ] [24h ] [ 7d ] [30d ] [custom]                          │
│                        ^^^^                                                  │
│  FILTERS                                                    API  local       │
│  Event Name:    ___________________________________         API               │
│  Username:      ___________________________________         API               │
│  Event Source:  ___________________________________         API               │
│  Resource ARN:  arn:aws:s3:::prod-data-bucket______         API               │
│  Access Key:    ___________________________________         API               │
│  Error Code:    ___________________________________              local       │
│  Source IP:     ___________________________________              local       │
│                                                                              │
│  TOGGLES                                                                     │
│  [x] Write events only    [ ] Error events only                              │
│                                                                              │
│  Navigated from: s3 / prod-data-bucket                                       │
│                                                                              │
│                        Enter: search  Esc: back to results                   │
└──────────────────────────────────────────────────────────────────────────────┘
```

The "Navigated from" line is rendered in dim italic (`#565f89` italic) and
serves as a breadcrumb. It appears only when ct-search was triggered via `T`
from a resource.

### 5.6 Triggering from Lambda Function (Error Investigation)

After seeing results, the user toggles Error-only to investigate failures:

```
 a9s v3.25.0  prod-admin:us-east-1                                 ? for help
┌── ct-search(3) -- api-handler, errors, write, last 24h ────────────────────┐
│ TIME                  EVENT NAME                ERROR             USER       │
│ 2026-03-29 14:58:31   UpdateFunctionConfigu…    AccessDenied      intern    │
│ 2026-03-29 10:22:15   InvokeFunction            ResourceNotFou…   ci-depl…  │
│ 2026-03-28 18:00:44   UpdateFunctionCode        InvalidParamet…   ci-depl…  │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 5.7 Triggering from Security Group (Change Audit)

```
 a9s v3.25.0  prod-admin:us-east-1                                 ? for help
┌── ct-search(6) -- sg-0abc123..., write events, last 24h ───────────────────┐
│ TIME                  EVENT NAME                USER              SOURCE    │
│ 2026-03-29 14:55:03   AuthorizeSecurityGrou…    platform-bot      ec2.ama… │
│ 2026-03-29 11:30:22   RevokeSecurityGroupIn…    admin             ec2.ama… │
│ 2026-03-28 22:15:11   AuthorizeSecurityGrou…    ci-deploy         ec2.ama… │
│ 2026-03-28 16:40:33   ModifySecurityGroupRu…    platform-bot      ec2.ama… │
│ 2026-03-28 10:05:44   CreateSecurityGroup       ci-deploy         ec2.ama… │
│ 2026-03-28 08:20:00   AuthorizeSecurityGrou…    platform-bot      ec2.ama… │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## 6. State Transitions

### 6.1 New Message Types

```go
// ResourceTrailMsg is emitted when user presses T on a resource.
// It carries enough context for ct-search to pre-populate filters.
type ResourceTrailMsg struct {
    ResourceType  string            // e.g., "ec2", "s3", "role"
    ResourceID    string            // e.g., "i-0abc123def456"
    ResourceName  string            // e.g., "api-prod-01"
    FilterField   string            // "ResourceARN" or "Username"
    FilterValue   string            // the ARN or username to filter by
    TimeRange     string            // default: "24h"
    WriteOnly     bool              // default: true
    IsIAMIdentity bool              // true for role/iam-user (enables dual-mode hint)
}
```

### 6.2 State Machine Addition

```
                    ┌─────────────────┐
                    │ ANY RESOURCE    │
                    │ list or detail  │
                    └────────┬────────┘
                             │ T key
                             │
                             v
                    ┌─────────────────┐
                    │ ResourceTrailMsg│
                    │ constructed     │
                    └────────┬────────┘
                             │
                             v
                    ┌─────────────────────────┐
                    │ ct-search opens with:   │
                    │  - pre-filled filters   │
                    │  - auto-execute search  │
                    │  - originResource set   │
                    └─────────────────────────┘
                             │
              (continues standard ct-search flow)
                             │
                             v
                    ┌─────────────────┐
                    │ RESULTS LIST    │
                    │ (pre-filtered)  │
                    └────────┬────────┘
                             │
                   f/Esc     │     Enter/d
                     │       │       │
                     v       │       v
              ┌────────────┐ │ ┌────────────────┐
              │ FORM       │ │ │ EVENT DETAIL   │
              │ (editable) │ │ │                │
              └────────────┘ │ └────────────────┘
                             │
                        Esc from form
                             │
                             v
                    ┌─────────────────┐
                    │ ORIGINAL VIEW   │
                    │ (resource list  │
                    │  or detail)     │
                    └─────────────────┘
```

### 6.3 View Stack Behavior

When `T` is pressed, the ct-search view is pushed onto the view stack:

```
Before T:  [main-menu] -> [ec2-list]
After T:   [main-menu] -> [ec2-list] -> [ct-search (pre-filled)]
```

Esc from the ct-search form pops back to the ec2-list. This is standard
view stack behavior -- no special handling needed.

If `T` is pressed from a detail view:

```
Before T:  [main-menu] -> [ec2-list] -> [ec2-detail]
After T:   [main-menu] -> [ec2-list] -> [ec2-detail] -> [ct-search (pre-filled)]
```

---

## 7. Key Bindings (additions to existing)

### 7.1 Resource List (addition to Section 5, Resource List in design.md)

| Key | Action | Notes |
|-----|--------|-------|
| `T` | Open CloudTrail events for selected resource | Opens ct-search pre-filled with resource filter |

### 7.2 Detail View (addition)

| Key | Action | Notes |
|-----|--------|-------|
| `T` | Open CloudTrail events for this resource | Same behavior as from list |

### 7.3 YAML View (addition)

| Key | Action | Notes |
|-----|--------|-------|
| `T` | Open CloudTrail events for this resource | Same behavior as from list |

### 7.4 Help Screen Updates

The help overlay for resource list views gains a new entry:

```
ACTIONS                NAVIGATION           COPY
<d>     Detail         <j>       Down       <c> Resource ID
<y>     YAML           <k>       Up         <T> CloudTrail
<T>     CloudTrail     <g>       Top
<enter> Open           <G>       Bottom
```

The `<T>` entry is rendered with the standard help key style (`#9ece6a` bold)
with description "CloudTrail" in dim.

---

## 8. Responsive Behavior

Same as ct-search (Section 12 of cloudtrail-search.md). The `T` trigger itself
is not width-dependent. The resulting ct-search view handles all width/height
adaptation.

---

## 9. Color Palette

No new colors. All rendering reuses the existing ct-search palette (Section 3
of cloudtrail-search.md) and the standard a9s palette (Section 1 of design.md).

The only new visual elements are hint/breadcrumb lines:

| Element | Foreground | Background | Style |
|---------|------------|------------|-------|
| "Navigated from" breadcrumb | `#565f89` | -- | Italic |
| IAM dual-mode hint | `#565f89` | -- | Italic |
| Data events caveat hint | `#565f89` | -- | Italic |
| Delivery delay note | `#565f89` | -- | Dim |

---

## 10. Implementation Notes

### 10.1 ResourceTypeDef Integration

Each `ResourceTypeDef` gains an optional `TrailFilter` function:

```go
type TrailFilterFunc func(r Resource, region, account string) (field, value string)
```

This function is called when `T` is pressed. It returns:
- `field`: "ResourceARN" or "Username"
- `value`: the actual filter value

If `TrailFilter` is nil, the fallback behavior (Section 3.4) applies.

### 10.2 No Changes to ct-search Model

The ct-search model already supports:
- Pre-filling filters (via `CTSearchPresetMsg`)
- Auto-executing search (presets do this)
- Editing filters after results (`f` key)

The `ResourceTrailMsg` is translated into a `CTSearchPresetMsg`-like flow
internally. The ct-search model gains a single new field:

```go
originResource *struct {
    Type string
    Name string
}
```

This is used only for the "Navigated from" breadcrumb display. It is nil when
ct-search is opened via `:ct-search` command or `s` from ct-events.

### 10.3 Account ID Resolution

Constructing ARNs requires the AWS account ID. This is already available from
the STS `GetCallerIdentity` call used for the identity view (`i` key). The
account ID should be cached at profile load time (it doesn't change within a
session).

### 10.4 Bubbles Components

No new Bubbles components. This feature reuses:
- `bubbles/textinput` (ct-search form)
- `bubbles/spinner` (ct-search loading)
- `bubbles/viewport` (ct-search event detail)

---

## 11. Demo Mode

Demo mode must handle `T` gracefully. When `T` is pressed on a demo resource,
the ct-search opens with pre-filled filters and returns demo fixture events
that appear related to the selected resource.

The demo fixture data from ct-search (Section 14 of cloudtrail-search.md) should
be filtered to match the resource type. For example, pressing `T` on a demo EC2
instance shows only the EC2-related demo events (RunInstances, ModifyInstance,
etc.).

---

## 12. Acceptance Criteria

| # | Criteria | Section |
|---|----------|---------|
| 1 | `T` key from resource list opens ct-search with resource ARN filter pre-populated | S3, S5.1-5.2 |
| 2 | `T` key from detail view opens ct-search for the displayed resource | S5.4 |
| 3 | `T` key from YAML view opens ct-search for the displayed resource | S7.3 |
| 4 | IAM Roles filter by Username (actor mode) by default | S3.2, S5.3 |
| 5 | IAM Users filter by Username (actor mode) by default | S3.2 |
| 6 | IAM dual-mode hint is shown when navigated from IAM resource | S3.2 |
| 7 | Default time range is 24h | S4 |
| 8 | Default write-only toggle is ON | S4 |
| 9 | Search auto-executes (user lands on results, not form) | S4 |
| 10 | User can press `f` to modify pre-filled filters | S5.5 |
| 11 | "Navigated from" breadcrumb appears in the form | S5.5 |
| 12 | Esc from form returns to original resource view | S6.3 |
| 13 | `T` on main menu shows flash hint "Select a resource first" | S2 |
| 14 | `T` on unmapped resource type opens empty ct-search with hint | S3.4 |
| 15 | All resource types in mapping table have correct filter derivation | S3.1 |
| 16 | Demo mode returns contextually appropriate events | S11 |
| 17 | Help overlay includes `T` key for resource list, detail, YAML views | S7.4 |
| 18 | Frame title shows truncated resource identifier | S5.2 |
| 19 | Delivery delay note shown when result count is low or most recent event is >15 min old | S4.1 |
| 20 | Data event caveat hint shown when results are empty for S3, Lambda, DynamoDB, KMS | S4.2 |
| 21 | Secrets Manager empty-results hint mentions write-only toggle | S4.2 |
| 22 | Each resource type in mapping table has a "Common Investigation" scenario | S3.1 |
