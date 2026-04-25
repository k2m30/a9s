# Related-Panel DevOps Consensus

Merged from 5 independent blind DevOps reviews. 115 parent→related pairs.

## Full table (all 5 API sequences per row)

### `--------` → `---------` — ----------- (5/5 agree)

- **Agent 1** (-----------): --------------
- **Agent 2** (-----------): --------------
- **Agent 3** (-----------): --------------
- **Agent 4** (-----------): --------------
- **Agent 5** (-----------): --------------

### `alarm` → `role` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `cloudwatch:DescribeAlarms` → inspect `AlarmActions`/`OKActions`/`InsufficientDataActions` for ARNs; if any action is `arn:aws:ssm:...:opsitem` or `arn:aws:swf:...action/actions/AWS_EC2.InstanceId.*`, the documented action uses AWSServiceRoleForCloudWatch* SLRs — no direct role ARN is stored. Mostly `no` in practice.
- **Agent 2** (sometimes): `cloudwatch:DescribeAlarms` to read `AlarmActions`/`OKActions`/`InsufficientDataActions` ARNs; if actions include `arn:aws:automate:...` or `ssm:...`, those imply a service-linked role. Generally no direct alarm→role link — would need to follow action ARNs (e.g. SNS topic → subscribed Lambda → `lambda:GetFunctionConfiguration.Role`)
- **Agent 3** (sometimes): `cloudwatch:DescribeAlarms` → inspect `AlarmActions`/`OKActions`/`InsufficientDataActions` ARNs; for `arn:aws:ssm:*:*:automation-definition/*` actions, any attached role arrives via the SSM document's AssumeRole. Generally weak — no direct `role` ARN on alarm itself
- **Agent 4** (sometimes): `cloudwatch:DescribeAlarms` on the alarm name; inspect `AlarmActions`/`OKActions`/`InsufficientDataActions` ARNs; if an action is `arn:aws:ssm:...:automation-definition/...` call `ssm:GetAutomationExecution`/`DescribeDocument` to read the `AssumeRole` parameter. No direct role field exists on an alarm itself.
- **Agent 5** (sometimes): `cloudwatch:DescribeAlarms` → inspect `AlarmActions`, `OKActions`, `InsufficientDataActions` for any `arn:aws:iam::...:role/...` ARNs. SSM Incident Manager and some Systems Manager OpsItem actions may reference roles indirectly. Most commonly no role is attached.

### `apigw` → `kms` — sometimes (3/5) — split: sometimes:3, no:2

- **Agent 1** (no): (API Gateway REST/HTTP APIs do not accept customer KMS keys for any at-rest feature; cache encryption uses AWS-owned keys)
- **Agent 2** (sometimes): `apigateway:GetRestApi` + `apigateway:GetStages` → `CacheClusterEnabled` (AWS-owned key only); for integrations: `apigateway:GetResources` → `apigateway:GetIntegration` → follow integration URI to backend, then check that backend's KMS usage. No direct API field returns a KMS ARN
- **Agent 3** (sometimes): `apigateway:GetStage` / `apigatewayv2:GetStages` → inspect `CacheClusterEnabled` + cache encryption; `apigateway:GetRestApis` → no direct KMS field. Generally no direct KMS attachment — unsure whether the checker meant CloudWatch log group KMS (`logs:DescribeLogGroups` → `KmsKeyId`)
- **Agent 4** (sometimes): For REST: `apigateway:GetStages` → each stage's `MethodSettings` does not expose a KMS key (AWS-managed). For HTTP/WebSocket (v2): `apigatewayv2:GetStages` likewise has no KMS field. Indirect: `apigateway:GetRestApi`/`GetIntegration` → if integration URI is a Lambda → `lambda:GetFunctionConfiguration` → `KMSKeyArn`. Otherwise no direct linkage.
- **Agent 5** (no): — (API Gateway REST/HTTP APIs do not support customer-managed KMS keys for at-rest encryption of cached data or config; cache encryption uses AWS-managed keys only)

### `asg` → `ami` — yes (5/5 agree)

- **Agent 1** (yes): `autoscaling:DescribeAutoScalingGroups` → get `LaunchConfigurationName` or `LaunchTemplate`/`MixedInstancesPolicy`. If LC: `autoscaling:DescribeLaunchConfigurations` → `ImageId`. If LT: `ec2:DescribeLaunchTemplateVersions` (version `$Latest` or `$Default` or the pinned one) → `LaunchTemplateData.ImageId`.
- **Agent 2** (yes): `autoscaling:DescribeAutoScalingGroups` → read `LaunchConfigurationName` or `LaunchTemplate`/`MixedInstancesPolicy`; then `autoscaling:DescribeLaunchConfigurations` → `ImageId`, OR `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`
- **Agent 3** (yes): `autoscaling:DescribeAutoScalingGroups` → get `LaunchConfigurationName` or `LaunchTemplate`/`MixedInstancesPolicy`; then `autoscaling:DescribeLaunchConfigurations` (→ `ImageId`) or `ec2:DescribeLaunchTemplateVersions` (→ `LaunchTemplateData.ImageId`)
- **Agent 4** (yes): `autoscaling:DescribeAutoScalingGroups` with the ASG name → read `LaunchTemplate.LaunchTemplateId`/`Version` or `LaunchConfigurationName` or `MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification`. Then: for launch template `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`; for launch configuration `autoscaling:DescribeLaunchConfigurations` → `ImageId`.
- **Agent 5** (yes): `autoscaling:DescribeAutoScalingGroups` → get `LaunchConfigurationName` or `LaunchTemplate`/`MixedInstancesPolicy`. If LaunchConfiguration: `autoscaling:DescribeLaunchConfigurations` → `ImageId`. If LaunchTemplate: `ec2:DescribeLaunchTemplateVersions` with the template ID/version → `LaunchTemplateData.ImageId`.

### `asg` → `cfn` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `autoscaling:DescribeAutoScalingGroups` → read tags; look for `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id`. Alternatively `resourcegroupstaggingapi:GetResources` filtered on the ASG ARN.
- **Agent 2** (sometimes): `autoscaling:DescribeAutoScalingGroups` → read tag `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id`; OR `cloudformation:DescribeStackResources` with `PhysicalResourceId=<asg-name>`
- **Agent 3** (sometimes): `autoscaling:DescribeAutoScalingGroups` → inspect `Tags` for `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id`; or `resourcegroupstaggingapi:GetResources` filtered on that tag
- **Agent 4** (sometimes): `autoscaling:DescribeAutoScalingGroups` → inspect `Tags` for `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id`. Then `cloudformation:DescribeStacks` with that stack name/ID.
- **Agent 5** (sometimes): `autoscaling:DescribeAutoScalingGroups` → inspect `Tags` for `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id`, then `cloudformation:DescribeStacks` with that stack name.

### `asg` → `elb` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (yes): `autoscaling:DescribeAutoScalingGroups` → `LoadBalancerNames` (classic ELB names). For ALB/NLB use `LoadBalancerTargetGroups` → `elbv2:DescribeTargetGroups(TargetGroupArns)` → `LoadBalancerArns`.
- **Agent 2** (yes): `autoscaling:DescribeAutoScalingGroups` → `LoadBalancerNames` (classic ELB) and `LoadBalancerTargetGroups` would need mapping via `elbv2:DescribeTargetGroups` → `LoadBalancerArns`; also `autoscaling:DescribeLoadBalancers` with `AutoScalingGroupName`
- **Agent 3** (yes): `autoscaling:DescribeAutoScalingGroups` → `LoadBalancerNames` (classic ELB) and `TargetGroupARNs` (ALB/NLB). For classic: `elb:DescribeLoadBalancers` by name. For ALB/NLB the link is via target groups, not `elb` directly
- **Agent 4** (yes): `autoscaling:DescribeAutoScalingGroups` → `LoadBalancerNames` (classic ELB). For ALB/NLB use `autoscaling:DescribeLoadBalancerTargetGroups` → `TargetGroupARNs`, then `elbv2:DescribeTargetGroups` → `LoadBalancerArns`, then `elbv2:DescribeLoadBalancers`.
- **Agent 5** (sometimes): `autoscaling:DescribeAutoScalingGroups` → `LoadBalancerNames` field lists classic ELB names. Then `elasticloadbalancing:DescribeLoadBalancers` with those names.

### `asg` → `role` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (sometimes): `autoscaling:DescribeAutoScalingGroups` → LaunchConfig/LaunchTemplate → `IamInstanceProfile` → `iam:GetInstanceProfile` → `Roles[].RoleName`. Also the ASG may use a service-linked role `AWSServiceRoleForAutoScaling`.
- **Agent 2** (yes): `autoscaling:DescribeAutoScalingGroups` → `LaunchConfigurationName` or `LaunchTemplate`; then `autoscaling:DescribeLaunchConfigurations` → `IamInstanceProfile`, OR `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.IamInstanceProfile`; then `iam:GetInstanceProfile` → `Roles[].RoleName`; also `autoscaling:DescribeAutoScalingGroups.ServiceLinkedRoleARN`
- **Agent 3** (yes): `autoscaling:DescribeAutoScalingGroups` → `LaunchConfigurationName` or `LaunchTemplate`; then `autoscaling:DescribeLaunchConfigurations` → `IamInstanceProfile`; or `ec2:DescribeLaunchTemplateVersions` → `IamInstanceProfile.Arn`/`Name`; then `iam:GetInstanceProfile` → `Roles[].Arn`
- **Agent 4** (yes): `autoscaling:DescribeAutoScalingGroups` → get `LaunchTemplate` or `LaunchConfigurationName`. Launch template: `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.IamInstanceProfile.{Arn,Name}`, then `iam:GetInstanceProfile` → `Roles[].Arn`. Launch config: `autoscaling:DescribeLaunchConfigurations` → `IamInstanceProfile`, same `iam:GetInstanceProfile` chain. Also check `ServiceLinkedRoleARN` on the ASG itself.
- **Agent 5** (yes): `autoscaling:DescribeAutoScalingGroups` → `ServiceLinkedRoleARN` gives service-linked role. Also launch config/template → `IamInstanceProfile` → `iam:GetInstanceProfile` → `Roles[].Arn`.

### `asg` → `sg` — yes (5/5 agree)

- **Agent 1** (yes): `autoscaling:DescribeAutoScalingGroups` → LaunchConfig → `SecurityGroups`; or LaunchTemplate → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.SecurityGroupIds` / `NetworkInterfaces[].Groups`.
- **Agent 2** (yes): `autoscaling:DescribeAutoScalingGroups` → `LaunchConfigurationName` or `LaunchTemplate`; then `autoscaling:DescribeLaunchConfigurations.SecurityGroups`, OR `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.SecurityGroupIds` / `NetworkInterfaces[].Groups`
- **Agent 3** (yes): `autoscaling:DescribeAutoScalingGroups` → LaunchConfig/LaunchTemplate; then `autoscaling:DescribeLaunchConfigurations` → `SecurityGroups`; or `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.SecurityGroupIds` / `NetworkInterfaces[].Groups`
- **Agent 4** (yes): `autoscaling:DescribeAutoScalingGroups` → resolve launch template/config as above; `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.SecurityGroupIds` and/or `NetworkInterfaces[].Groups`; or `DescribeLaunchConfigurations` → `SecurityGroups`.
- **Agent 5** (yes): `autoscaling:DescribeAutoScalingGroups` → `LaunchConfigurationName` or `LaunchTemplate`. `autoscaling:DescribeLaunchConfigurations` → `SecurityGroups`; or `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.SecurityGroupIds` / `NetworkInterfaces[].Groups`.

### `asg` → `sns` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (yes): `autoscaling:DescribeNotificationConfigurations(AutoScalingGroupNames=[name])` → `TopicARN` list.
- **Agent 2** (yes): `autoscaling:DescribeNotificationConfigurations` with `AutoScalingGroupNames=[name]` → `TopicARN`
- **Agent 3** (yes): `autoscaling:DescribeNotificationConfigurations` filtered by `AutoScalingGroupNames` → `TopicARN`
- **Agent 4** (yes): `autoscaling:DescribeNotificationConfigurations` with the ASG name → `TopicARN` list.
- **Agent 5** (sometimes): `autoscaling:DescribeNotificationConfigurations` with ASG name → `TopicARN` for each event. Also `autoscaling:DescribeLifecycleHooks` → `NotificationTargetARN` may be SNS.

### `asg` → `vpc` — yes (5/5 agree)

- **Agent 1** (yes): `autoscaling:DescribeAutoScalingGroups` → `VPCZoneIdentifier` (comma-separated subnet IDs) → `ec2:DescribeSubnets(SubnetIds)` → `VpcId`.
- **Agent 2** (yes): `autoscaling:DescribeAutoScalingGroups` → `VPCZoneIdentifier` (subnet IDs); `ec2:DescribeSubnets` with those IDs → `VpcId`
- **Agent 3** (yes): `autoscaling:DescribeAutoScalingGroups` → `VPCZoneIdentifier` (subnet-id list); then `ec2:DescribeSubnets` with those ids → `VpcId`
- **Agent 4** (yes): `autoscaling:DescribeAutoScalingGroups` → `VPCZoneIdentifier` (comma-separated subnet IDs); `ec2:DescribeSubnets` with those subnet IDs → `VpcId`.
- **Agent 5** (yes): `autoscaling:DescribeAutoScalingGroups` → `VPCZoneIdentifier` contains comma-separated subnet IDs. `ec2:DescribeSubnets` with those subnet IDs → `VpcId`.

### `backup` → `eb-rule` — sometimes (5/5 agree)

- **Agent 1** (sometimes): List via `events:ListRuleNamesByTarget` is reverse; correct direction: `events:ListRules` → for each rule `events:DescribeRule` → inspect `EventPattern` for `"source":["aws.backup"]` and `detail.backupVaultName`/`detail.backupPlanId` matching the parent. Expensive iteration.
- **Agent 2** (sometimes): `events:ListRules` then `events:ListTargetsByRule` and inspect EventPattern for `source: aws.backup` and references to the backup vault/plan ARN; also `backup:ListBackupVaultNotifications` (SNS only, not EB)
- **Agent 3** (sometimes): `events:ListRules` then for each `events:ListTargetsByRule` → inspect `Arn` for `arn:aws:backup:...`; or use `events:ListRuleNamesByTarget` with the backup vault/plan ARN
- **Agent 4** (sometimes): `backup:ListBackupVaults` / `backup:GetBackupVaultNotifications` does not expose EB rules; instead use `events:ListRuleNamesByTarget` with the vault ARN or plan ARN as target, or iterate `events:ListRules` → `events:ListTargetsByRule` and match by backup resource ARN.
- **Agent 5** (sometimes): List-all approach: `events:ListRules` → for each, `events:ListTargetsByRule` and inspect `EventPattern` for `source: aws.backup` or target ARN matching vault/plan.

### `backup` → `logs` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Iterate `events:ListRules` filtered by `aws.backup` source, then `events:ListTargetsByRule` → target ARN with `arn:aws:logs:...` prefix. No direct API from Backup vault/plan to a log group.
- **Agent 2** (sometimes): `backup:DescribeReportPlan` → `ReportDeliveryChannel.S3BucketName`; for CloudWatch Logs directly — Backup doesn't natively stream to CWL. Check `logs:DescribeLogGroups` with prefix `/aws/backup` (none standard); indirect via CloudTrail `logs:DescribeLogGroups` subscribed to trail
- **Agent 3** (sometimes): `events:ListRules` filtered on Backup events → targets of type `logs:*:*:log-group:*`; otherwise no direct Backup→Logs link
- **Agent 4** (sometimes): `logs:DescribeLogGroups` with prefix `/aws/backup`, or use `resourcegroupstaggingapi:GetResources` filtering by tags on log groups tied to the vault. Otherwise there is no direct `LogGroup` field on a BackupVault/BackupPlan.
- **Agent 5** (sometimes): `backup:DescribeReportPlan` (for audit reports) may indicate delivery to S3, not Logs. Generally inspect via CloudTrail/EventBridge → Logs routing; no direct backup→log-group API.

### `cb` → `pipeline` — yes (5/5 agree)

- **Agent 1** (yes): `codepipeline:ListPipelines` → for each `codepipeline:GetPipeline` → iterate `Stages[].Actions[]` where `ActionTypeId.Provider == "CodeBuild"` and `Configuration.ProjectName == <cb project>`. (Reverse search — no direct CodeBuild→pipeline API.)
- **Agent 2** (yes): `codepipeline:ListPipelines` → for each, `codepipeline:GetPipeline` → inspect `Stages[].Actions[]` where `ActionTypeId.Provider=CodeBuild` and `Configuration.ProjectName=<cb-name>`
- **Agent 3** (yes): `codepipeline:ListPipelines` then for each `codepipeline:GetPipeline` → iterate `stages[].actions[]` where `actionTypeId.category=Build` and `configuration.ProjectName` matches the CodeBuild project name
- **Agent 4** (yes): `codepipeline:ListPipelines` → for each pipeline `codepipeline:GetPipeline` → scan `Stages[].Actions[]` where `ActionTypeId.Provider == "CodeBuild"` and `Configuration.ProjectName` == the CodeBuild project name. (Reverse lookup from a project requires iterating pipelines; no API returns pipelines by project.)
- **Agent 5** (yes): `codebuild:BatchGetProjects` → project ARN. Then `codepipeline:ListPipelines` → for each pipeline `codepipeline:GetPipeline` → scan `Stages[].Actions[]` for `ActionTypeId.Provider == "CodeBuild"` and `Configuration.ProjectName` matching.

### `codeartifact` → `acm` — no (5/5 agree)

- **Agent 1** (no): (CodeArtifact does not integrate with ACM — domains/repositories are reached via AWS-managed TLS only)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (CodeArtifact domains/repositories have no direct ACM certificate association)

### `codeartifact` → `cb` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Iterate `codebuild:ListProjects` → `codebuild:BatchGetProjects` → inspect `ServiceRole`'s policies or `Source.Buildspec`/env vars referencing the CodeArtifact domain/repo. Expensive, heuristic.
- **Agent 2** (sometimes): `codebuild:ListProjects` + `codebuild:BatchGetProjects` → inspect `Environment.EnvironmentVariables` or buildspec for CodeArtifact domain/repo references (grep-based, no first-class API)
- **Agent 3** (sometimes): No direct API link. `codebuild:ListProjects` + `codebuild:BatchGetProjects` → scan `environment.environmentVariables` and `source.buildspec` for the CodeArtifact domain/repo name (heuristic)
- **Agent 4** (sometimes): No direct API. Iterate `codebuild:ListProjects` → `codebuild:BatchGetProjects` → scan `Source.Buildspec`/`Environment.EnvironmentVariables` for the CodeArtifact domain/repo ARN.
- **Agent 5** (sometimes): List-all approach: `codebuild:ListProjects` → for each `codebuild:BatchGetProjects` → inspect `Source.Buildspec`/environment variables for the repository endpoint; or check role policy for `codeartifact:*` scoped to the repo ARN.

### `codeartifact` → `kinesis` — no (5/5 agree)

- **Agent 1** (no): (CodeArtifact has no Kinesis integration)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (CodeArtifact has no direct Kinesis integration)

### `codeartifact` → `kms` — yes (5/5 agree)

- **Agent 1** (yes): `codeartifact:DescribeDomain(domain)` → `EncryptionKey` (KMS key ARN).
- **Agent 2** (yes): `codeartifact:DescribeDomain` with `domain=<name>` → `domain.encryptionKey` (KMS key ARN)
- **Agent 3** (yes): `codeartifact:DescribeDomain` → `encryptionKey` (KMS key ARN). Repositories inherit the domain's key
- **Agent 4** (yes): `codeartifact:DescribeDomain` with domain name → `domain.encryptionKey` (KMS key ARN). Repositories inherit the domain's key.
- **Agent 5** (yes): `codeartifact:DescribeDomain` with domain name → `encryptionKey` field returns the KMS key ARN.

### `codeartifact` → `lambda` — no (5/5 agree)

- **Agent 1** (no): (CodeArtifact does not directly invoke or reference Lambda functions)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (no direct Lambda integration with CodeArtifact domain/repository)

### `codeartifact` → `logs` — no (5/5 agree)

- **Agent 1** (no): (CodeArtifact does not write to a customer CloudWatch Logs group; only CloudTrail captures API events)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (CodeArtifact has no native CloudWatch Logs integration; only CloudTrail data events optionally to Logs)

### `codeartifact` → `r53` — no (5/5 agree)

- **Agent 1** (no): (CodeArtifact endpoints are AWS-managed; no Route 53 relationship)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (CodeArtifact endpoints are AWS-managed; no Route 53 records required)

### `codeartifact` → `role` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `codeartifact:GetDomainPermissionsPolicy(domain)` and `codeartifact:GetRepositoryPermissionsPolicy(domain, repository)` → parse `Document.Statement[].Principal.AWS` for role ARNs.
- **Agent 2** (sometimes): `codeartifact:GetDomainPermissionsPolicy` or `codeartifact:GetRepositoryPermissionsPolicy` → parse policy document `Statement[].Principal.AWS` for role ARNs
- **Agent 3** (sometimes): `codeartifact:GetDomainPermissionsPolicy` and `codeartifact:GetRepositoryPermissionsPolicy` → parse `Statement[].Principal.AWS` for role ARNs
- **Agent 4** (sometimes): `codeartifact:GetDomainPermissionsPolicy` and `codeartifact:GetRepositoryPermissionsPolicy` → parse `document` JSON `Statement[].Principal.AWS` for role ARNs.
- **Agent 5** (sometimes): `codeartifact:GetDomainPermissionsPolicy` and `codeartifact:GetRepositoryPermissionsPolicy` → parse policy JSON `Statement[].Principal.AWS` for IAM role ARNs.

### `codeartifact` → `waf` — no (5/5 agree)

- **Agent 1** (no): (CodeArtifact endpoints are not protected by customer-managed WAF)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (CodeArtifact endpoints are not WAF-protectable)

### `ddb` → `backup` — yes (5/5 agree)

- **Agent 1** (yes): `dynamodb:DescribeContinuousBackups(TableName)` for PITR; for on-demand/AWS-Backup: `backup:ListRecoveryPointsByResource(ResourceArn=<tableArn>)` → `RecoveryPoints[].BackupVaultName`/`RecoveryPointArn`.
- **Agent 2** (yes): `dynamodb:DescribeContinuousBackups` → point-in-time recovery status; `backup:ListProtectedResources` filter by `ResourceArn=<table-arn>` → backup plans/recovery points; `backup:ListRecoveryPointsByResource` with table ARN
- **Agent 3** (yes): `backup:ListProtectedResources` filter `ResourceType=DynamoDB`; or `backup:ListRecoveryPointsByResource` with the DDB table ARN; also `dynamodb:DescribeContinuousBackups` tells PITR status (not AWS Backup plans)
- **Agent 4** (yes): `dynamodb:DescribeContinuousBackups` with table name → `PointInTimeRecoveryDescription`. For on-demand/AWS Backup: `backup:ListRecoveryPointsByResource` with the table ARN → recovery points; or `backup:ListProtectedResources` filtered to `ResourceType == DynamoDB`.
- **Agent 5** (yes): `dynamodb:DescribeContinuousBackups` → PITR status. For AWS Backup integration: `backup:ListProtectedResources` and filter by `ResourceArn` matching the DynamoDB table ARN; or tag-based: AWS Backup selections reference tables by ARN or tag via `backup:ListBackupSelections`.

### `ddb` → `kinesis` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (yes): `dynamodb:DescribeKinesisStreamingDestination(TableName)` → `KinesisDataStreamDestinations[].StreamArn`.
- **Agent 2** (yes): `dynamodb:DescribeKinesisStreamingDestination` with `TableName` → `KinesisDataStreamDestinations[].StreamArn`
- **Agent 3** (yes): `dynamodb:DescribeKinesisStreamingDestination` with table name → `KinesisDataStreamDestinations[].StreamArn`
- **Agent 4** (yes): `dynamodb:DescribeKinesisStreamingDestination` with table name → `KinesisDataStreamDestinations[].StreamArn`.
- **Agent 5** (sometimes): `dynamodb:DescribeKinesisStreamingDestination` with table name → `KinesisDataStreamDestinations[].StreamArn`.

### `ddb` → `secrets` — no (5/5 agree)

- **Agent 1** (no): (DynamoDB has no direct Secrets Manager integration; at-rest uses KMS, not Secrets Manager)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (DynamoDB has no direct Secrets Manager association; any usage would be app-level)

### `ddb` → `sns` — no (4/5) — split: no:4, sometimes:1

- **Agent 1** (sometimes): Indirect. `cloudwatch:DescribeAlarmsForMetric(Namespace=AWS/DynamoDB, Dimensions=TableName)` → `AlarmActions` with SNS topic ARNs. No direct DDB→SNS API.
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (DynamoDB tables don't directly integrate with SNS; event notifications require Streams + Lambda or EventBridge Pipes)

### `docdb-snap` → `backup` — sometimes (4/5) — split: sometimes:4, yes:1

- **Agent 1** (sometimes): `backup:ListRecoveryPointsByResource(ResourceArn=<clusterArn>)` — AWS Backup tracks DocDB cluster resources; snapshot→backup mapping is via the parent cluster. No direct snapshot-ARN-to-backup-vault API.
- **Agent 2** (sometimes): `backup:ListRecoveryPointsByResource` with `ResourceArn=<docdb-cluster-arn>`; OR `backup:ListProtectedResources` filter by resource type `Aurora`/`DocumentDB`
- **Agent 3** (yes): `backup:ListProtectedResources` filter by DocumentDB cluster ARN; or `backup:ListRecoveryPointsByResource` with the cluster/snapshot ARN — AWS Backup manages DocDB snapshots when a plan targets DocDB
- **Agent 4** (sometimes): `backup:ListRecoveryPointsByResource` with the DocumentDB cluster ARN (or snapshot ARN where supported) → recovery points; or inspect the snapshot's ARN tag `aws:backup:source-resource`. DocDB-native snapshots (`rds:DescribeDBClusterSnapshots` with `--engine docdb`) are not Backup recovery points.
- **Agent 5** (sometimes): `backup:ListRecoveryPointsByResource` with the DocumentDB cluster ARN; or `backup:DescribeRecoveryPoint` if the snapshot ARN is a recovery-point ARN. Alternatively match tags `aws:backup:source-resource`.

### `eb` → `elb` — yes (5/5 agree)

- **Agent 1** (yes): `elasticbeanstalk:DescribeEnvironmentResources(EnvironmentName)` → `EnvironmentResources.LoadBalancers[].Name` (classic) or `elasticbeanstalk:DescribeEnvironments` → env resources then `elbv2:DescribeLoadBalancers` by tag `elasticbeanstalk:environment-name`.
- **Agent 2** (yes): `elasticbeanstalk:DescribeEnvironmentResources` with `EnvironmentName` → `EnvironmentResources.LoadBalancers[].Name` (classic) or `elasticbeanstalk:DescribeConfigurationSettings` → namespace `aws:elbv2:loadbalancer` for ALB/NLB ARNs
- **Agent 3** (yes): `elasticbeanstalk:DescribeEnvironmentResources` with `EnvironmentName` → `EnvironmentResources.LoadBalancers[].Name` (classic ELB) and also `Instances`, `AutoScalingGroups`
- **Agent 4** (yes): `elasticbeanstalk:DescribeEnvironmentResources` with `EnvironmentName` → `EnvironmentResources.LoadBalancers[].Name` (classic ELB by default). For ALB: `DescribeEnvironments`/`DescribeConfigurationSettings` option `aws:elasticbeanstalk:environment:LoadBalancerType`, then `elbv2:DescribeLoadBalancers` filtering by tag `elasticbeanstalk:environment-name`.
- **Agent 5** (yes): `elasticbeanstalk:DescribeEnvironmentResources` with env name → `EnvironmentResources.LoadBalancers[].Name` (classic ELB). For ALB/NLB: inspect the same response's `LoadBalancers` or the associated CloudFormation stack resources.

### `eb` → `role` — yes (5/5 agree)

- **Agent 1** (yes): `elasticbeanstalk:DescribeConfigurationSettings(ApplicationName, EnvironmentName)` → `OptionSettings` where `Namespace=aws:autoscaling:launchconfiguration` and `OptionName=IamInstanceProfile`; also `aws:elasticbeanstalk:environment` / `ServiceRole`. Then `iam:GetInstanceProfile` to resolve roles.
- **Agent 2** (yes): `elasticbeanstalk:DescribeConfigurationSettings` → namespace `aws:autoscaling:launchconfiguration` option `IamInstanceProfile`; also `aws:elasticbeanstalk:environment` option `ServiceRole`
- **Agent 3** (yes): `elasticbeanstalk:DescribeConfigurationSettings` → option settings `aws:autoscaling:launchconfiguration:IamInstanceProfile` (instance profile) and `aws:elasticbeanstalk:environment:ServiceRole`
- **Agent 4** (yes): `elasticbeanstalk:DescribeConfigurationSettings` → `OptionSettings` with namespace `aws:autoscaling:launchconfiguration` key `IamInstanceProfile` and namespace `aws:elasticbeanstalk:environment` key `ServiceRole`. Resolve instance profile via `iam:GetInstanceProfile`.
- **Agent 5** (yes): `elasticbeanstalk:DescribeEnvironments` → `EnvironmentId`. `elasticbeanstalk:DescribeConfigurationSettings` → OptionSettings for `aws:autoscaling:launchconfiguration:IamInstanceProfile` and `aws:elasticbeanstalk:environment:ServiceRole`.

### `eb` → `s3` — yes (5/5 agree)

- **Agent 1** (yes): `elasticbeanstalk:DescribeApplicationVersions(ApplicationName)` → `SourceBundle.S3Bucket`/`S3Key`. Also EB stores logs in `elasticbeanstalk-<region>-<account>` bucket (derivable) → `s3:GetBucketLocation` optional.
- **Agent 2** (yes): `elasticbeanstalk:DescribeApplicationVersions` → `SourceBundle.S3Bucket`/`S3Key`; OR `elasticbeanstalk:CreateStorageLocation` (returns bucket); S3 bucket naming convention `elasticbeanstalk-<region>-<account-id>`
- **Agent 3** (yes): Application versions live in S3: `elasticbeanstalk:DescribeApplicationVersions` → `SourceBundle.S3Bucket`; also the EB-managed bucket `elasticbeanstalk-<region>-<account>` is used for logs/configs
- **Agent 4** (yes): Application versions live in an EB-owned bucket: `elasticbeanstalk:CreateStorageLocation` returns the bucket name, or `elasticbeanstalk:DescribeApplicationVersions` → `SourceBundle.S3Bucket`. ELB access logs and EB logs may go to a customer bucket, discoverable via `DescribeConfigurationSettings` option `aws:elasticbeanstalk:hostmanager:LogPublicationControl`.
- **Agent 5** (yes): `elasticbeanstalk:CreateStorageLocation` returns the EB bucket name (`elasticbeanstalk-<region>-<account>`); application versions live in that bucket. Also `elasticbeanstalk:DescribeApplicationVersions` → `SourceBundle.S3Bucket`.

### `eb` → `sg` — yes (5/5 agree)

- **Agent 1** (yes): `elasticbeanstalk:DescribeEnvironmentResources` → `Instances[].Id` then `ec2:DescribeInstances` → `SecurityGroups[]`; or `DescribeConfigurationSettings` → `OptionSettings` `aws:autoscaling:launchconfiguration/SecurityGroups`.
- **Agent 2** (yes): `elasticbeanstalk:DescribeEnvironmentResources` → `EnvironmentResources.Instances[].Id`; then `ec2:DescribeInstances` → `SecurityGroups`; OR `elasticbeanstalk:DescribeConfigurationSettings` → namespace `aws:autoscaling:launchconfiguration` option `SecurityGroups`
- **Agent 3** (yes): `elasticbeanstalk:DescribeConfigurationSettings` → option `aws:autoscaling:launchconfiguration:SecurityGroups` and `aws:elbv2:loadbalancer:SecurityGroups`; or `elasticbeanstalk:DescribeEnvironmentResources` → `Instances` → `ec2:DescribeInstances` → `SecurityGroups`
- **Agent 4** (yes): `elasticbeanstalk:DescribeEnvironmentResources` → `Instances[].Id`; `ec2:DescribeInstances` → `SecurityGroups[]`. Also `DescribeConfigurationSettings` options `aws:autoscaling:launchconfiguration:SecurityGroups` and `aws:elbv2:loadbalancer:SecurityGroups`.
- **Agent 5** (yes): `elasticbeanstalk:DescribeEnvironmentResources` → inspect `Instances`; or `DescribeConfigurationSettings` OptionSettings for `aws:autoscaling:launchconfiguration:SecurityGroups` and `aws:elbv2:loadbalancer:SecurityGroups`.

### `eb` → `tg` — yes (3/5) — split: yes:3, sometimes:2

- **Agent 1** (yes): `elasticbeanstalk:DescribeEnvironmentResources` → `LoadBalancers[]` → `elbv2:DescribeListeners(LoadBalancerArn)` → `DefaultActions[].TargetGroupArn`.
- **Agent 2** (sometimes): `elasticbeanstalk:DescribeEnvironmentResources` → `EnvironmentResources.LoadBalancers[].Name`; then `elbv2:DescribeListeners` → `elbv2:DescribeRules` → target groups; OR `elbv2:DescribeTargetGroups` and filter by tag `elasticbeanstalk:environment-name`
- **Agent 3** (yes): `elasticbeanstalk:DescribeEnvironmentResources` → `LoadBalancers` (for ALB environments); then `elbv2:DescribeTargetGroups` with `LoadBalancerArn` of the EB load balancer
- **Agent 4** (yes): `elasticbeanstalk:DescribeEnvironmentResources` → `LoadBalancers[].Name` (ARN for ALB); then `elbv2:DescribeListeners` → `DefaultActions[].TargetGroupArn`, or `elbv2:DescribeTargetGroups --load-balancer-arn`.
- **Agent 5** (sometimes): `elasticbeanstalk:DescribeEnvironmentResources` → get LoadBalancer ARN; `elasticloadbalancingv2:DescribeListeners` → listener ARNs; `elasticloadbalancingv2:DescribeRules` / `DescribeTargetGroups` filtering by LB ARN.

### `ec2` → `ssm` — yes (3/5) — split: yes:3, sometimes:2

- **Agent 1** (yes): `ssm:DescribeInstanceInformation(Filters=[{Key:"InstanceIds",Values:[<id>]}])` → returns SSM-managed instance info (PingStatus, PlatformName). Also `ssm:ListAssociations(AssociationFilterList=[{Key:"InstanceId",...}])` for associations.
- **Agent 2** (yes): `ssm:DescribeInstanceInformation` with `Filters=[{Key:InstanceIds,Values:[<id>]}]`; also `ssm:ListAssociations` filtered by instance; `ssm:GetParameters` referenced in user-data requires scanning instance user-data
- **Agent 3** (sometimes): `ssm:DescribeInstanceInformation` with `InstanceInformationFilterList=[{key=InstanceIds,valueSet=[i-...]}]` → presence means it's an SSM managed instance; also `ssm:ListAssociations` per instance → associations
- **Agent 4** (yes): `ssm:DescribeInstanceInformation` filter by `InstanceIds` → `PingStatus`, `AssociationStatus`; `ssm:ListAssociations` / `ssm:DescribeAssociationExecutionTargets` filter by instance; `ssm:GetInventory` / `ssm:DescribeInstancePatchStates` with the instance ID.
- **Agent 5** (sometimes): `ssm:DescribeInstanceInformation` with filter `InstanceIds=<id>`. Presence of a matching entry means the instance is SSM-managed. Also `ssm:ListAssociations` filtering on the instance ID.

### `ecr` → `eb-rule` — sometimes (3/5) — split: sometimes:3, yes:2

- **Agent 1** (yes): `events:ListRules` → for each `events:DescribeRule` with `EventPattern` source `aws.ecr` and `detail.repository-name == <repoName>`; or `events:ListRuleNamesByTarget` if a Lambda/SNS target has the ECR repo in its environment. Iterative.
- **Agent 2** (yes): `events:ListRules` → for each `events:ListTargetsByRule` then inspect `EventPattern` for `source: aws.ecr` and `resources` matching repository ARN
- **Agent 3** (sometimes): `events:ListRuleNamesByTarget` won't work (rules don't target ECR); instead `events:ListRules` → for each `events:DescribeRule` → inspect `EventPattern` for `"source":["aws.ecr"]` and `detail.repository-name` matching the repo
- **Agent 4** (sometimes): Iterate `events:ListRules` → `events:DescribeRule` and parse `EventPattern` for `source:["aws.ecr"]` and `resources` containing the repository ARN. No reverse API exists.
- **Agent 5** (sometimes): List-all approach: `events:ListRules` → `events:DescribeRule` → inspect `EventPattern` for `source: aws.ecr` and `resources:` matching the repository ARN.

### `ecr` → `ecs` — yes (3/5) — split: yes:3, sometimes:2

- **Agent 1** (yes): Iterate `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Image` whose registry host matches `<account>.dkr.ecr.<region>.amazonaws.com/<repoName>`. Expensive reverse scan.
- **Agent 2** (yes): `ecs:ListTaskDefinitions` → for each `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Image`; filter where image URI starts with `<account>.dkr.ecr.<region>.amazonaws.com/<repo>`
- **Agent 3** (yes): `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `containerDefinitions[].image` — match against the ECR repository URI (`<acct>.dkr.ecr.<region>.amazonaws.com/<repo>`); running services: `ecs:ListClusters` → `ecs:ListServices` → `ecs:DescribeServices` → `taskDefinition` → same
- **Agent 4** (sometimes): Iterate `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → scan `ContainerDefinitions[].Image` for the ECR repository URI, then `ecs:ListServices`/`DescribeServices` to find services using that task def.
- **Agent 5** (sometimes): List-all approach: `ecs:ListTaskDefinitions` → for each `ecs:DescribeTaskDefinition` → scan `ContainerDefinitions[].Image` for the repo URI. Then `ecs:ListServices`/`ListClusters` to map to services.

### `ecr` → `eks` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Not directly queryable at AWS API level (image pulls are inside the k8s cluster). Best effort: check `eks:ListClusters` and match by tag, or inspect Kubernetes deployments via `kubectl` — out of scope for AWS-only APIs. Mostly `no` from AWS alone.
- **Agent 2** (sometimes): `eks:ListClusters` → then must inspect pods/deployments inside the cluster (kubectl) for image pulls; OR scan node-group instance profiles for ECR permissions. No direct AWS API returns ECR↔EKS mapping
- **Agent 3** (sometimes): No direct AWS API. Heuristic: `eks:ListClusters` → workloads use whatever image. Proper sequence requires `kubectl get pods -A -o json` on each cluster and matching image refs to the ECR repo URI
- **Agent 4** (sometimes): No AWS API direct link. Workload-level linkage lives in the Kubernetes cluster (pod specs, not AWS API). At AWS level you can check the repository policy (`ecr:GetRepositoryPolicy`) for a principal that is an EKS node role.
- **Agent 5** (sometimes): No AWS API directly returns this. Would require Kubernetes API access per cluster (`kubectl get pods -o jsonpath` for image refs). Via IAM: check EKS node role/Pod-Identity/IRSA roles with `ecr:*` scoped to the repo ARN.

### `ecr` → `pipeline` — sometimes (3/5) — split: sometimes:3, yes:2

- **Agent 1** (yes): `codepipeline:ListPipelines` → `codepipeline:GetPipeline` → iterate actions; source actions with `ActionTypeId.Provider == "ECR"` and `Configuration.RepositoryName == <repo>`. Reverse search.
- **Agent 2** (sometimes): `codepipeline:ListPipelines` → `codepipeline:GetPipeline` → `Stages[].Actions[]` where `ActionTypeId.Provider=ECR` and `Configuration.RepositoryName=<repo>`
- **Agent 3** (yes): `codepipeline:ListPipelines` → `codepipeline:GetPipeline` → iterate `stages[].actions[]`; ECR source actions: `actionTypeId.provider=ECR` with `configuration.RepositoryName`; also scan Build actions' env vars for ECR URIs
- **Agent 4** (sometimes): Iterate `codepipeline:ListPipelines` → `GetPipeline` → scan stages for actions with `ActionTypeId.Provider == "ECR"` and `Configuration.RepositoryName` matching the repo; also inspect CodeBuild actions' projects for ECR usage.
- **Agent 5** (sometimes): List-all approach: `codepipeline:ListPipelines` → `codepipeline:GetPipeline` → scan `Stages[].Actions[]` for `ActionTypeId.Provider == "ECR"` and `Configuration.RepositoryName`.

### `ecr` → `role` — sometimes (3/5) — split: sometimes:3, yes:2

- **Agent 1** (sometimes): `ecr:GetRepositoryPolicy(repositoryName)` → parse `Statement[].Principal.AWS` for IAM role ARNs.
- **Agent 2** (sometimes): `ecr:GetRepositoryPolicy` with `repositoryName` → parse JSON `Statement[].Principal.AWS` for role ARNs
- **Agent 3** (yes): `ecr:GetRepositoryPolicy` on the repo → parse policy `Statement[].Principal.AWS` for role ARNs
- **Agent 4** (sometimes): `ecr:GetRepositoryPolicy` → parse `policyText` `Statement[].Principal.AWS` for role ARNs. Also `ecr:GetRegistryPolicy` at account level.
- **Agent 5** (yes): `ecr:GetRepositoryPolicy` with repository name → parse policy JSON `Statement[].Principal.AWS` for role ARNs. Registry policy via `ecr:GetRegistryPolicy` may also reference roles.

### `ecs-svc` → `cf` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Reverse scan: `cloudfront:ListDistributions` → for each, inspect `Origins.Items[].DomainName`; match against ALB DNS from `ecs:DescribeServices` → `LoadBalancers[].TargetGroupArn` → `elbv2:DescribeTargetGroups` → `LoadBalancerArns` → `elbv2:DescribeLoadBalancers` → `DNSName`.
- **Agent 2** (sometimes): `ecs:DescribeServices` → `LoadBalancers[].TargetGroupArn`; `elbv2:DescribeTargetGroups` → `LoadBalancerArns`; `elbv2:DescribeLoadBalancers` → `DNSName`; `cloudfront:ListDistributions` → filter `Origins[].DomainName` matching that DNS name
- **Agent 3** (sometimes): `ecs:DescribeServices` → `loadBalancers[].targetGroupArn`; `elbv2:DescribeTargetGroups` → `LoadBalancerArns`; `elbv2:DescribeLoadBalancers` → `DNSName`; then `cloudfront:ListDistributions` → find `Origins.Items[].DomainName` matching the ALB DNS
- **Agent 4** (sometimes): `ecs:DescribeServices` → `LoadBalancers[].TargetGroupArn` → `elbv2:DescribeTargetGroups` → `LoadBalancerArns`; then iterate `cloudfront:ListDistributions` → search `Origins[].DomainName` for the LB DNS name.
- **Agent 5** (sometimes): `ecs:DescribeServices` → `LoadBalancers[].TargetGroupArn` → `elbv2:DescribeTargetGroups`/`DescribeLoadBalancers` to get LB DNS. Then `cloudfront:ListDistributions` and inspect `Origins.Items[].DomainName` for the LB DNS match.

### `ecs-svc` → `eb-rule` — sometimes (4/5) — split: sometimes:4, yes:1

- **Agent 1** (yes): `events:ListRuleNamesByTarget(TargetArn=<ecsServiceArn>)` returns rules whose target is this ECS service (ECS task via RunTask target uses `EcsParameters.TaskDefinitionArn`). Also iterate `events:ListRules` and match targets.
- **Agent 2** (sometimes): `events:ListRuleNamesByTarget` with `TargetArn=<ecs-cluster-or-service-arn>`; OR `events:ListRules` + `events:ListTargetsByRule` where Target `EcsParameters.TaskDefinitionArn` refers to the service's task family
- **Agent 3** (sometimes): `events:ListRules` → for each `events:ListTargetsByRule` → target with `EcsParameters.TaskDefinitionArn` matching the service's task def, or `Arn` being an ECS cluster ARN with `EcsParameters`
- **Agent 4** (sometimes): `events:ListRuleNamesByTarget` with the cluster ARN as the target, or iterate `events:ListRules` → `ListTargetsByRule` and match `EcsParameters.TaskDefinitionArn` and `Arn` (cluster) against the service's cluster.
- **Agent 5** (sometimes): `events:ListRuleNamesByTarget` with the service ARN or task-def ARN; or `events:ListRules` → `events:ListTargetsByRule` → filter targets with `EcsParameters.TaskDefinitionArn` matching.

### `ecs-svc` → `ecr` — yes (3/5) — split: yes:3, sometimes:2

- **Agent 1** (yes): `ecs:DescribeServices` → `TaskDefinition` → `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Image`; parse `<account>.dkr.ecr.<region>.amazonaws.com/<repo>` to derive ECR repo names.
- **Agent 2** (yes): `ecs:DescribeServices` → `TaskDefinition`; then `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Image`; filter for ECR URIs
- **Agent 3** (yes): `ecs:DescribeServices` → `taskDefinition` → `ecs:DescribeTaskDefinition` → `containerDefinitions[].image` — parse ECR URIs
- **Agent 4** (sometimes): `ecs:DescribeServices` → `TaskDefinition` → `ecs:DescribeTaskDefinition` → scan `ContainerDefinitions[].Image` for ECR URIs; extract repo name, then `ecr:DescribeRepositories`.
- **Agent 5** (sometimes): `ecs:DescribeServices` → `TaskDefinition` ARN. `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Image`. Parse image URI for `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>`.

### `ecs-svc` → `r53` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `ecs:DescribeServices` → `ServiceRegistries[].RegistryArn` (Cloud Map service) → `servicediscovery:GetService` → `DnsConfig.NamespaceId` → `servicediscovery:GetNamespace` → `HostedZoneId` (if public) → `route53:ListResourceRecordSets(HostedZoneId)`.
- **Agent 2** (sometimes): `ecs:DescribeServices` → `ServiceRegistries[].RegistryArn` (Cloud Map, uses R53 private hosted zones); also `route53:ListHostedZones` + `route53:ListResourceRecordSets` filter aliases pointing to the service's ALB DNS
- **Agent 3** (sometimes): `ecs:DescribeServices` → `serviceRegistries[].registryArn` (Cloud Map service); `servicediscovery:GetService` → `NamespaceId`; `servicediscovery:GetNamespace` → `Properties.DnsProperties.HostedZoneId`. For ALB-alias: route53:ListHostedZones → `ListResourceRecordSets` → alias targeting ALB DNS
- **Agent 4** (sometimes): `ecs:DescribeServices` → `ServiceRegistries[].RegistryArn` (Cloud Map service ARN) → `servicediscovery:GetService` → `DnsConfig.NamespaceId` → `servicediscovery:GetNamespace` → `Properties.DnsProperties.HostedZoneId`, then `route53:ListResourceRecordSets`.
- **Agent 5** (sometimes): `ecs:DescribeServices` → `ServiceRegistries[].RegistryArn` (Cloud Map service). `servicediscovery:GetService` → `NamespaceId` → `servicediscovery:GetNamespace` → `Properties.DnsProperties.HostedZoneId`.

### `ecs-svc` → `secrets` — yes (3/5) — split: yes:3, sometimes:2

- **Agent 1** (yes): `ecs:DescribeServices` → `TaskDefinition` → `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` (Secrets Manager ARN) and `ContainerDefinitions[].Environment` is not secrets — only `Secrets` array or `secretOptions`.
- **Agent 2** (yes): `ecs:DescribeServices.TaskDefinition` → `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` (Secrets Manager ARN) and `RepositoryCredentials.CredentialsParameter`
- **Agent 3** (yes): `ecs:DescribeServices` → `taskDefinition`; `ecs:DescribeTaskDefinition` → `containerDefinitions[].secrets[].valueFrom` (ARNs of secrets manager or ssm params)
- **Agent 4** (sometimes): `ecs:DescribeServices` → `TaskDefinition`; `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` (ARN of secret) or `Environment` with secretsmanager ARNs.
- **Agent 5** (sometimes): `ecs:DescribeServices` → TaskDefinition ARN. `ecs:DescribeTaskDefinition` → scan `ContainerDefinitions[].Secrets[].ValueFrom` and `ContainerDefinitions[].Environment` for `arn:aws:secretsmanager:...`.

### `ecs-svc` → `sfn` — sometimes (4/5) — split: sometimes:4, no:1

- **Agent 1** (sometimes): Reverse scan: `stepfunctions:ListStateMachines` → `stepfunctions:DescribeStateMachine` → parse `definition` JSON for `"Resource":"arn:aws:states:::ecs:runTask"` with `TaskDefinition` matching. No direct API.
- **Agent 2** (sometimes): `stepfunctions:ListStateMachines` → `stepfunctions:DescribeStateMachine` → parse `definition` JSON for `"Resource": "arn:aws:states:::ecs:runTask"` and `Parameters.TaskDefinition` matching service's family
- **Agent 3** (sometimes): `states:ListStateMachines` → for each `states:DescribeStateMachine` → parse `definition` JSON for `Resource: "arn:aws:states:::ecs:runTask*"` and `Parameters.TaskDefinition` matching the service's task def
- **Agent 4** (sometimes): Iterate `stepfunctions:ListStateMachines` → `DescribeStateMachine` → parse `definition` JSON `Resource` fields of type `arn:aws:states:::ecs:runTask*` and match cluster/taskDefinition.
- **Agent 5** (no): — (ECS services do not directly reference Step Functions; SFN may invoke ECS tasks but not services)

### `ecs-task` → `kms` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `ecs:DescribeTaskDefinition(taskDefinition)` → container `Secrets` point at Secrets Manager ARNs; the secrets themselves are encrypted with KMS. At task def level: `ecs:DescribeClusters(Clusters,include=[CONFIGURATIONS])` → `Configuration.ExecuteCommandConfiguration.KmsKeyId`.
- **Agent 2** (sometimes): `ecs:DescribeTaskDefinition` → `EphemeralStorage` (no KMS exposed); `ContainerDefinitions[].Secrets[].ValueFrom` then `secretsmanager:DescribeSecret.KmsKeyId` or `ssm:DescribeParameters`/`kms` referenced by parameter. No direct KMS field on task def
- **Agent 3** (sometimes): `ecs:DescribeTaskDefinition` → `volumes[].efsVolumeConfiguration.fileSystemId` → `efs:DescribeFileSystems` → `KmsKeyId`; also `containerDefinitions[].secrets[].valueFrom` → `secretsmanager:DescribeSecret` → `KmsKeyId`
- **Agent 4** (sometimes): `ecs:DescribeTaskDefinition` → inspect `ContainerDefinitions[].Secrets[].ValueFrom` (resolve secret → `secretsmanager:DescribeSecret` → `KmsKeyId`) and `LogConfiguration.Options["awslogs-group"]` (resolve → `logs:DescribeLogGroups` → `KmsKeyId`). Task itself has no direct KMS field.
- **Agent 5** (sometimes): `ecs:DescribeTaskDefinition` → inspect `EphemeralStorage` (Fargate), `ContainerDefinitions[].Secrets` (follow each ARN to `secretsmanager:DescribeSecret`.`KmsKeyId` or `ssm:DescribeParameters`).

### `ecs-task` → `secrets` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (yes): `ecs:DescribeTaskDefinition(taskDefinition)` → `ContainerDefinitions[].Secrets[].ValueFrom` that start with `arn:aws:secretsmanager:`.
- **Agent 2** (yes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` (Secrets Manager ARNs)
- **Agent 3** (yes): `ecs:DescribeTaskDefinition` → `containerDefinitions[].secrets[].valueFrom` where ARN prefix is `arn:aws:secretsmanager:`
- **Agent 4** (yes): `ecs:DescribeTaskDefinition` → for each container, `Secrets[].ValueFrom` where ARN prefix is `arn:aws:secretsmanager:`. Also scan `Environment` for secret ARNs (non-standard).
- **Agent 5** (sometimes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` starting with `arn:aws:secretsmanager:...`.

### `ecs-task` → `ssm` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (yes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` that start with `arn:aws:ssm:...:parameter/...` — SSM Parameter Store parameters.
- **Agent 2** (yes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` entries with `arn:aws:ssm:*:parameter/*`; also `ExecutionRoleArn` may have SSM perms (not a direct SSM resource link)
- **Agent 3** (yes): `ecs:DescribeTaskDefinition` → `containerDefinitions[].secrets[].valueFrom` where ARN prefix is `arn:aws:ssm:*:*:parameter/...` (SSM Parameter Store)
- **Agent 4** (yes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` where ARN prefix is `arn:aws:ssm:...:parameter/...`.
- **Agent 5** (sometimes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` starting with `arn:aws:ssm:...:parameter/...`.

### `efs` → `backup` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (yes): `backup:ListRecoveryPointsByResource(ResourceArn=<efsFileSystemArn>)` → `RecoveryPoints[].BackupVaultName`. Or `backup:ListProtectedResources` filtered by `ResourceType=EFS`.
- **Agent 2** (yes): `backup:ListProtectedResources` filter `ResourceType=EFS` and `ResourceArn=<fs-arn>`; `backup:ListRecoveryPointsByResource` with EFS filesystem ARN
- **Agent 3** (yes): `backup:ListProtectedResources` filter by EFS filesystem ARN; or `backup:ListRecoveryPointsByResource` with `ResourceArn=arn:aws:elasticfilesystem:<region>:<acct>:file-system/fs-...`
- **Agent 4** (yes): `backup:ListRecoveryPointsByResource` with the EFS file system ARN → recovery points and associated backup plans.
- **Agent 5** (sometimes): `backup:ListProtectedResources` filter `ResourceArn == <efs-arn>`; also `backup:ListRecoveryPointsByResource` with the EFS file system ARN.

### `efs` → `ecs-task` — yes (3/5) — split: yes:3, sometimes:2

- **Agent 1** (yes): Reverse: `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `Volumes[].EfsVolumeConfiguration.FileSystemId == <fsId>`. Expensive iteration.
- **Agent 2** (yes): `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `Volumes[].EfsVolumeConfiguration.FileSystemId` matching the EFS id
- **Agent 3** (yes): `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `volumes[].efsVolumeConfiguration.fileSystemId` — match to the EFS id
- **Agent 4** (sometimes): Iterate `ecs:ListTaskDefinitions` → `DescribeTaskDefinition` → scan `Volumes[].EfsVolumeConfiguration.FileSystemId` for the EFS ID.
- **Agent 5** (sometimes): List-all approach: `ecs:ListTaskDefinitions` → for each `ecs:DescribeTaskDefinition` → scan `Volumes[].EfsVolumeConfiguration.FileSystemId` matching.

### `eip` → `kms` — no (5/5 agree)

- **Agent 1** (no): (Elastic IPs are not encrypted and have no KMS relationship)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (Elastic IPs are not encrypted; no KMS relationship)

### `eks` → `acm` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Indirect. Cluster control plane endpoint uses AWS-managed certs. Workload-level certs live on ALBs — discover via tag `elbv2.k8s.aws/cluster=<clusterName>` on ALBs: `elbv2:DescribeLoadBalancers` → filter by tag → `elbv2:DescribeListeners` → `Certificates[].CertificateArn`.
- **Agent 2** (sometimes): `eks:ListClusters` → requires inspecting Kubernetes Ingress resources (kubectl) for `alb.ingress.kubernetes.io/certificate-arn` annotation; `elbv2:DescribeListeners` on cluster's ALBs → `Certificates[].CertificateArn`. No direct EKS API returns ACM
- **Agent 3** (sometimes): No direct EKS API field. Heuristic: discover ingress ALBs via cluster workloads, then `elbv2:DescribeListeners` on those LBs → `Certificates[].CertificateArn`. Pure AWS-API chain starting only from cluster name is not possible without kubectl access
- **Agent 4** (sometimes): No direct AWS API field on the cluster. Indirect: check `elbv2:DescribeListeners` for LBs tagged `elbv2.k8s.aws/cluster=<cluster-name>` → `Certificates[].CertificateArn`.
- **Agent 5** (sometimes): No direct EKS→ACM API. Would require Kubernetes API access to inspect Ingress annotations (`alb.ingress.kubernetes.io/certificate-arn`) per cluster.

### `eks` → `ami` — yes (5/5 agree)

- **Agent 1** (yes): `eks:ListNodegroups(clusterName)` → `eks:DescribeNodegroup` → `ReleaseVersion` or `LaunchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`. For managed nodegroups without LT: derive AMI from `AmiType`+`ReleaseVersion` via EKS-optimized SSM parameter `/aws/service/eks/optimized-ami/...`.
- **Agent 2** (yes): `eks:ListNodegroups` with `clusterName`; `eks:DescribeNodegroup` → `ReleaseVersion`/`AmiType` (AMI ID derivable via SSM parameter `/aws/service/eks/optimized-ami/...`) or `LaunchTemplate` → `ec2:DescribeLaunchTemplateVersions.LaunchTemplateData.ImageId`; for self-managed nodes: ASG path
- **Agent 3** (yes): `eks:ListNodegroups` → `eks:DescribeNodegroup` → `releaseVersion`+`amiType` (managed) or `launchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `ImageId`; for self-managed: scan ASGs in the cluster's VPC
- **Agent 4** (yes): `eks:ListNodegroups` → `DescribeNodegroup` → `ReleaseVersion`/`AmiType`; if a launch template is used, read `LaunchTemplate.Id/Version` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`. Self-managed node groups: ASG/launch template chain (see `asg`→`ami`).
- **Agent 5** (yes): `eks:DescribeCluster` then `eks:ListNodegroups` → for each `eks:DescribeNodegroup` → `ReleaseVersion`/`AmiType` (managed node group). For self-managed, inspect the launch template → AMI ID.

### `eks` → `ec2` — yes (5/5 agree)

- **Agent 1** (yes): `ec2:DescribeInstances(Filters=[{Name:"tag:aws:eks:cluster-name",Values:[<cluster>]}])`. Also `eks:ListNodegroups` → `DescribeNodegroup` → `Resources.AutoScalingGroups[]` → `autoscaling:DescribeAutoScalingGroups` → `Instances[]`.
- **Agent 2** (yes): `eks:ListNodegroups` → `eks:DescribeNodegroup` → `Resources.AutoScalingGroups[].Name`; then `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`; OR `ec2:DescribeInstances` filter tag `aws:eks:cluster-name=<cluster>` or `kubernetes.io/cluster/<cluster>=owned`
- **Agent 3** (yes): `eks:ListNodegroups` → `eks:DescribeNodegroup` → `resources.autoScalingGroups[].name`; `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`; then `ec2:DescribeInstances`
- **Agent 4** (yes): `eks:ListNodegroups` → `DescribeNodegroup` → `Resources.AutoScalingGroups[].Name`; `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`; or directly `ec2:DescribeInstances` filter `tag:eks:cluster-name=<name>`.
- **Agent 5** (yes): `eks:DescribeCluster` → VPC/subnets. For node instances: `eks:ListNodegroups` → `eks:DescribeNodegroup` → `Resources.AutoScalingGroups[].Name` → `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`.

### `eks` → `ecr` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Not directly queryable via AWS EKS APIs. Heuristic: cluster node IAM role (from `eks:DescribeNodegroup → NodeRole`) typically has `AmazonEC2ContainerRegistryReadOnly`; image pull source is inside k8s (`kubectl get pods -o jsonpath=...`) — outside AWS-only scope. Mostly `no` from pure AWS APIs.
- **Agent 2** (sometimes): `eks:DescribeCluster` — no direct link; must inspect pod specs via kubectl, OR infer from nodegroup IAM role having `AmazonEC2ContainerRegistryReadOnly`. No first-class AWS API
- **Agent 3** (sometimes): No direct AWS API; requires `kubectl` on the cluster. Alternative: `eks:DescribeCluster` then via cluster access, enumerate pod images and parse ECR URIs
- **Agent 4** (sometimes): No direct AWS API. Workload-level (inside the cluster). At AWS level you can enumerate node-role permissions that allow pulling from ECR (`iam:GetRolePolicy` on node role) or check `ecr:GetRepositoryPolicy` for the node role principal.
- **Agent 5** (sometimes): No AWS API returns this. Requires `kubectl get pods -o jsonpath` to extract image refs. Indirect: node-role/IRSA policies referencing `ecr:*` on specific repos.

### `eks` → `iam-user` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `eks:ListAccessEntries(clusterName)` → for each `eks:DescribeAccessEntry` → `PrincipalArn` may be an IAM user ARN (`arn:aws:iam::<acct>:user/...`). Legacy: read `aws-auth` ConfigMap in `kube-system` (k8s, not AWS).
- **Agent 2** (sometimes): `eks:ListAccessEntries` with `clusterName` → `eks:DescribeAccessEntry` → `principalArn` (may be IAM user); legacy aws-auth ConfigMap requires kubectl
- **Agent 3** (sometimes): `eks:ListAccessEntries` → for each, `eks:DescribeAccessEntry` → `principalArn` that starts with `arn:aws:iam::*:user/...`. Legacy: read `aws-auth` configmap (requires kubectl)
- **Agent 4** (sometimes): `eks:ListAccessEntries --cluster-name` → `DescribeAccessEntry` → `principalArn` (if it's `arn:aws:iam::...:user/...`). The legacy aws-auth path requires reading the Kubernetes ConfigMap, not an AWS API call.
- **Agent 5** (sometimes): `eks:ListAccessEntries` with cluster name → `accessEntries[]` → for each `eks:DescribeAccessEntry` → `principalArn` filter for `arn:aws:iam::...:user/...`. Legacy: inspect `aws-auth` ConfigMap via Kubernetes API.

### `elb` → `logs` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `elbv2:DescribeLoadBalancerAttributes(LoadBalancerArn)` → attributes keyed `access_logs.s3.enabled`/`connection_logs.s3.enabled` (S3, not CWL). True CWL linkage: `elbv2:DescribeLoadBalancerAttributes` does not expose a CWL log group directly. Classic ELB: `elb:DescribeLoadBalancerAttributes` → `AccessLog.S3BucketName` (S3 only). So `no` for direct CWL log group from the ELB API.
- **Agent 2** (sometimes): `elbv2:DescribeLoadBalancerAttributes` → attribute `access_logs.s3.enabled`, `access_logs.s3.bucket`, `access_logs.s3.prefix`; for classic: `elb:DescribeLoadBalancerAttributes` → `AccessLog`. For CloudWatch Logs, no native — only via log-delivery to a Firehose/S3-to-Logs pipeline
- **Agent 3** (sometimes): Classic ELB access logs only support S3 (`elb:DescribeLoadBalancerAttributes` → `AccessLog.S3BucketName`). For ALB/NLB: `elbv2:DescribeLoadBalancerAttributes` → `access_logs.s3.bucket`. No direct CloudWatch Logs destination for ELB access logs; may be no-op
- **Agent 4** (sometimes): For ALB/NLB: `elbv2:DescribeLoadBalancerAttributes` → `access_logs.s3.bucket` — note this is S3, not CloudWatch Logs. No direct CW Log group. If target is Lambda: `elbv2:DescribeTargetGroups` → target Lambda → its `/aws/lambda/<name>` log group.
- **Agent 5** (sometimes): Access logs go to S3 via `elbv2:DescribeLoadBalancerAttributes` (`access_logs.s3.*`). ALB connection logs also S3. CloudWatch Logs integration is not native; no direct log-group linkage.

### `iam-user` → `kms` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Reverse scan: `kms:ListKeys` → for each `kms:GetKeyPolicy(PolicyName=default)` → parse `Principal.AWS` for user ARN; and `kms:ListGrants(KeyId)` → `GranteePrincipal`. Expensive.
- **Agent 2** (sometimes): `iam:ListAttachedUserPolicies` + `iam:ListUserPolicies` → fetch each policy document; OR iterate `kms:ListKeys` → `kms:GetKeyPolicy` → parse for `Principal.AWS` referencing user ARN (expensive fanout)
- **Agent 3** (sometimes): Iterate all keys: `kms:ListKeys` → `kms:GetKeyPolicy` → parse `Statement[].Principal.AWS` for the user ARN. Expensive but legitimate
- **Agent 4** (sometimes): Iterate `kms:ListKeys` → for each key `kms:GetKeyPolicy` → parse statements for `Principal.AWS` containing the user ARN. (Expensive cross-account scan; legitimate.)
- **Agent 5** (sometimes): `iam:ListAttachedUserPolicies` + `iam:ListUserPolicies` → `iam:GetPolicyVersion`/`GetUserPolicy` → scan statements for `kms:*` resources. Conversely: list-all `kms:ListKeys` → `kms:GetKeyPolicy` → inspect policy `Principal.AWS` for user ARN.

### `iam-user` → `role` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `iam:ListRoles` → for each `iam:GetRole` → `AssumeRolePolicyDocument` → parse `Statement[].Principal.AWS` for user ARN. Or `iam:ListAttachedUserPolicies`/`iam:ListUserPolicies` → `iam:GetPolicy`/`GetPolicyVersion` and inspect `Resource` for role ARNs.
- **Agent 2** (sometimes): `iam:ListRoles` → for each `iam:GetRole.AssumeRolePolicyDocument`; parse `Statement[].Principal.AWS` for user ARN (expensive fanout)
- **Agent 3** (sometimes): `iam:ListRoles` → for each `iam:GetRole` → `AssumeRolePolicyDocument` → parse `Statement[].Principal.AWS` for the user ARN. Also `iam:ListAttachedUserPolicies`+`iam:ListUserPolicies` → inspect for `sts:AssumeRole` resources
- **Agent 4** (sometimes): `iam:ListAttachedUserPolicies` + `iam:ListUserPolicies` → `GetPolicyVersion`/`GetUserPolicy` → parse for `sts:AssumeRole` with resource ARNs. Reverse: iterate `iam:ListRoles` → `GetRole` → parse `AssumeRolePolicyDocument.Statement[].Principal.AWS` for the user ARN.
- **Agent 5** (sometimes): `iam:ListAttachedUserPolicies` + `iam:SimulatePrincipalPolicy`. Or list-all `iam:ListRoles` → `iam:GetRole` → inspect `AssumeRolePolicyDocument.Statement[].Principal.AWS` for user ARN.

### `kinesis` → `ddb` — sometimes (4/5) — split: sometimes:4, yes:1

- **Agent 1** (sometimes): `dynamodb:ListTables` → `dynamodb:DescribeKinesisStreamingDestination(TableName)` → match `StreamArn == <kinesisArn>`. Expensive reverse iteration.
- **Agent 2** (sometimes): `dynamodb:ListTables` → for each `dynamodb:DescribeKinesisStreamingDestination` → match `KinesisDataStreamDestinations[].StreamArn` with this stream's ARN (expensive fanout); cheaper: `kinesis:DescribeStreamConsumer` doesn't expose DDB source
- **Agent 3** (yes): `dynamodb:ListTables` → for each `dynamodb:DescribeKinesisStreamingDestination` → `KinesisDataStreamDestinations[].StreamArn` matches the stream ARN
- **Agent 4** (sometimes): `kinesis:DescribeStreamConsumer` does not expose this. Reverse: iterate `dynamodb:ListTables` → `DescribeKinesisStreamingDestination` and match `StreamArn`.
- **Agent 5** (sometimes): List-all approach: `dynamodb:ListTables` → for each `dynamodb:DescribeKinesisStreamingDestination` → filter `StreamArn` matching.

### `kinesis` → `eb-rule` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `events:ListRuleNamesByTarget(TargetArn=<kinesisStreamArn>)` → rule names, then `events:DescribeRule` per rule.
- **Agent 2** (sometimes): `events:ListRuleNamesByTarget` with `TargetArn=<stream-arn>` (if supported for Kinesis targets); OR `events:ListRules` + `events:ListTargetsByRule` and filter Target `Arn`
- **Agent 3** (sometimes): `events:ListRuleNamesByTarget` with the Kinesis stream ARN → rule names; then `events:DescribeRule`
- **Agent 4** (sometimes): `events:ListRuleNamesByTarget` with the stream ARN → rule names; then `events:DescribeRule`.
- **Agent 5** (sometimes): `events:ListRuleNamesByTarget` with the stream ARN.

### `kinesis` → `logs` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Reverse scan: `logs:DescribeLogGroups` → for each `logs:DescribeSubscriptionFilters(logGroupName)` → `DestinationArn == <kinesisStreamArn>`. Expensive.
- **Agent 2** (sometimes): `logs:DescribeLogGroups` → for each `logs:DescribeSubscriptionFilters` → match `DestinationArn` (expensive fanout)
- **Agent 3** (sometimes): `logs:DescribeLogGroups` → for each `logs:DescribeSubscriptionFilters` → `DestinationArn` matching the stream ARN. Expensive but legitimate
- **Agent 4** (sometimes): Iterate `logs:DescribeLogGroups` → `logs:DescribeSubscriptionFilters` → match `DestinationArn` == stream ARN. Also `logs:DescribeDestinations` for cross-account Logs-destinations, whose `TargetArn` may be a Kinesis stream.
- **Agent 5** (sometimes): List-all approach: `logs:DescribeLogGroups` → for each `logs:DescribeSubscriptionFilters` → filter `DestinationArn` matching the stream ARN.

### `kms` → `role` — sometimes (4/5) — split: sometimes:4, yes:1

- **Agent 1** (sometimes): `kms:GetKeyPolicy(KeyId,PolicyName="default")` → parse `Statement[].Principal.AWS` for role ARNs; `kms:ListGrants(KeyId)` → `GranteePrincipal` / `RetiringPrincipal` for role ARNs.
- **Agent 2** (sometimes): `kms:GetKeyPolicy` with `KeyId` → parse `Statement[].Principal.AWS` for role ARNs; also `kms:ListGrants` → `GranteePrincipal` and `RetiringPrincipal`
- **Agent 3** (yes): `kms:GetKeyPolicy` → parse `Statement[].Principal.AWS` for role ARNs; also `kms:ListGrants` → `GranteePrincipal` / `RetiringPrincipal` as role ARNs
- **Agent 4** (sometimes): `kms:GetKeyPolicy` (parse `Principal.AWS` for role ARNs) and `kms:ListGrants` (→ `GranteePrincipal`, `RetiringPrincipal`).
- **Agent 5** (sometimes): `kms:GetKeyPolicy` → parse `Statement[].Principal.AWS` for role ARNs. Also `kms:ListGrants` → `GranteePrincipal` and `RetiringPrincipal` for role ARNs.

### `lambda` → `asg` — no (5/5 agree)

- **Agent 1** (no): (Lambda functions have no direct ASG relationship; Lambda is serverless)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (Lambda functions have no direct Auto Scaling Group relationship; Lambda scaling is managed by the service)

### `lambda` → `ec2` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `lambda:GetFunctionConfiguration(FunctionName)` → `VpcConfig.SubnetIds`/`SecurityGroupIds`. Not instances. For ENIs: `ec2:DescribeNetworkInterfaces(Filters=[{Name:"requester-id",Values:["*:AWSLambda*"]}])` matched by description `"AWS Lambda VPC ENI-<functionName>..."`.
- **Agent 2** (sometimes): `lambda:GetFunctionConfiguration` → `VpcConfig.SubnetIds`/`SecurityGroupIds`; to find EC2 instances in same subnets: `ec2:DescribeInstances` with `Filters=[subnet-id]`. No direct Lambda→EC2 API
- **Agent 3** (sometimes): `lambda:GetFunctionConfiguration` → `VpcConfig.SubnetIds`+`SecurityGroupIds` (subnets/SGs, not EC2 instances). No direct Lambda→EC2-instance link
- **Agent 4** (sometimes): `lambda:GetFunctionConfiguration` → `VpcConfig.SubnetIds`/`SecurityGroupIds`; `ec2:DescribeNetworkInterfaces --filter description="AWS Lambda VPC ENI-<functionname>-<uuid>"` to find the ENIs. No EC2 instances are involved.
- **Agent 5** (sometimes): `lambda:GetFunctionConfiguration` → `VpcConfig.SubnetIds` and `VpcConfig.SecurityGroupIds`. For ENIs: `ec2:DescribeNetworkInterfaces` filter `requester-id` / description `AWS Lambda VPC ENI-<function-name>`. No direct EC2 instance link.

### `lambda` → `ecr` — sometimes (4/5) — split: sometimes:4, yes:1

- **Agent 1** (yes): `lambda:GetFunction(FunctionName)` → `Code.ImageUri` (container-image Lambdas) — parse ECR repo name from URI `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>@sha256:...`.
- **Agent 2** (sometimes): `lambda:GetFunction` → `Code.ImageUri` (present when `PackageType=Image`); parse repo name from URI `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>:<tag>`
- **Agent 3** (sometimes): `lambda:GetFunction` → `Code.ImageUri` (for `PackageType=Image`) — parse ECR repo URI
- **Agent 4** (sometimes): `lambda:GetFunction` → `Code.ImageUri` (ECR image URI) when `PackageType == "Image"`. Parse to get the ECR repo name.
- **Agent 5** (sometimes): `lambda:GetFunction` → `Code.ImageUri`. Parse URI for `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>:<tag>`.

### `lambda` → `elb` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Reverse: `elbv2:DescribeTargetGroups` → filter `TargetType == "lambda"` → `elbv2:DescribeTargetHealth(TargetGroupArn)` → `TargetHealthDescriptions[].Target.Id == <lambdaArn>`. Then TG→LB via `LoadBalancerArns`.
- **Agent 2** (sometimes): `elbv2:DescribeTargetGroups` → filter `TargetType=lambda`; `elbv2:DescribeTargetHealth` → `Target.Id` == Lambda ARN; expensive fanout unless starting from TG side
- **Agent 3** (sometimes): `elbv2:DescribeTargetGroups` → filter `TargetType=lambda`; then `elbv2:DescribeTargetHealth` → targets' `Id` matches the function ARN; then `elbv2:DescribeListeners`/`DescribeLoadBalancers` for the LB
- **Agent 4** (sometimes): No direct API on the function. Reverse: iterate `elbv2:DescribeTargetGroups` where `TargetType == "lambda"` → `elbv2:DescribeTargetHealth` → target ID is the Lambda ARN.
- **Agent 5** (sometimes): `lambda:GetPolicy` → parse function's resource-based policy for `Principal.Service: elasticloadbalancing.amazonaws.com` and `SourceArn` of the target group. Then `elbv2:DescribeTargetGroups`/`DescribeLoadBalancers`.

### `lambda` → `r53` — no (3/5) — split: no:3, sometimes:2

- **Agent 1** (no): (Lambda has no native Route 53 linkage; custom domains go via API Gateway/CloudFront)
- **Agent 2** (no):
- **Agent 3** (sometimes): No direct API. Heuristic: find the function's API Gateway/ALB integration, then `route53:ListResourceRecordSets` across zones looking for alias targeting that domain
- **Agent 4** (sometimes): No direct link. Function URL: `lambda:GetFunctionUrlConfig` gives an AWS-owned domain; to see if Route 53 points at it, iterate hosted zones (`route53:ListHostedZones` → `ListResourceRecordSets`) and match CNAMEs.
- **Agent 5** (no): — (Lambda has no direct Route 53 relationship; would only be indirect via API Gateway/CloudFront custom domains)

### `lambda` → `sfn` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Reverse: `stepfunctions:ListStateMachines` → `stepfunctions:DescribeStateMachine` → parse `definition` JSON for states with `Resource == <lambdaArn>` or `Resource == "arn:aws:states:::lambda:invoke"` + `Parameters.FunctionName == <lambdaArn>`.
- **Agent 2** (sometimes): `stepfunctions:ListStateMachines` → `stepfunctions:DescribeStateMachine` → parse `definition` for `"Resource": "<lambda-arn>"` or `arn:aws:states:::lambda:invoke` with `FunctionName`; expensive fanout
- **Agent 3** (sometimes): `states:ListStateMachines` → for each `states:DescribeStateMachine` → parse `definition` JSON for `"Resource": "<lambda-arn>"` or `"Resource": "arn:aws:states:::lambda:invoke"` with `Parameters.FunctionName`
- **Agent 4** (sometimes): Iterate `stepfunctions:ListStateMachines` → `DescribeStateMachine` → parse `definition` JSON for `Resource` == function ARN or `"arn:aws:states:::lambda:invoke"` with `FunctionName` == function ARN.
- **Agent 5** (sometimes): List-all approach: `stepfunctions:ListStateMachines` → for each `stepfunctions:DescribeStateMachine` → parse `definition` JSON and scan `States.*.Resource` for `arn:aws:lambda:...:function:<name>` or `arn:aws:states:::lambda:invoke`.

### `ng` → `ami` — yes (5/5 agree)

- **Agent 1** (yes): `eks:DescribeNodegroup(clusterName,nodegroupName)` → `LaunchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`. Managed NGs without LT: derive AMI from `AmiType`+`ReleaseVersion` via `/aws/service/eks/optimized-ami/...` SSM parameter.
- **Agent 2** (yes): `eks:DescribeNodegroup` with `clusterName`,`nodegroupName` → `AmiType`+`ReleaseVersion` (resolve via SSM param `/aws/service/eks/optimized-ami/<k8s>/<ami-type>/recommended/image_id`); OR `LaunchTemplate.Id`/`Version` → `ec2:DescribeLaunchTemplateVersions.LaunchTemplateData.ImageId`
- **Agent 3** (yes): `eks:DescribeNodegroup` → if `launchTemplate` set: `ec2:DescribeLaunchTemplateVersions` → `ImageId`; else `amiType`+`releaseVersion` determines the EKS-optimized AMI (resolvable via `ssm:GetParameter` for `/aws/service/eks/optimized-ami/...`)
- **Agent 4** (yes): `eks:DescribeNodegroup` → `AmiType` and `ReleaseVersion`; if `LaunchTemplate` is set, `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`. Managed node groups without a custom LT use the EKS-optimized AMI (resolvable via SSM parameter `/aws/service/eks/optimized-ami/...`).
- **Agent 5** (yes): `eks:DescribeNodegroup` with cluster/nodegroup → `ReleaseVersion` + `AmiType` (managed); for custom, `LaunchTemplate.Id`/`Version` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`.

### `ng` → `ebs` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (yes): `eks:DescribeNodegroup` → LT or `DiskSize`; instance-level EBS via `Resources.AutoScalingGroups[]` → ASG → `ec2:DescribeInstances` → `BlockDeviceMappings[].Ebs.VolumeId`.
- **Agent 2** (sometimes): `eks:DescribeNodegroup` → `Resources.AutoScalingGroups[].Name`; `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`; `ec2:DescribeVolumes` with `Filters=[attachment.instance-id]`; also `LaunchTemplate` → `BlockDeviceMappings[].Ebs`
- **Agent 3** (yes): `eks:DescribeNodegroup` → `resources.autoScalingGroups[].name`; `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`; `ec2:DescribeInstances` → `BlockDeviceMappings[].Ebs.VolumeId`; or via launch template `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.BlockDeviceMappings`
- **Agent 4** (yes): `eks:DescribeNodegroup` → `DiskSize` (if no LT) implies a root EBS volume; or `LaunchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.BlockDeviceMappings[].Ebs`. For live volumes: `DescribeNodegroup` → `Resources.AutoScalingGroups` → instances → `ec2:DescribeInstances` → `BlockDeviceMappings[].Ebs.VolumeId`.
- **Agent 5** (yes): `eks:DescribeNodegroup` → `DiskSize` / launch template → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.BlockDeviceMappings[].Ebs`. Live volumes: list ASG → instances → `ec2:DescribeInstances` → `BlockDeviceMappings[].Ebs.VolumeId`.

### `ng` → `kms` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `eks:DescribeNodegroup` → `LaunchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.BlockDeviceMappings[].Ebs.KmsKeyId`. Also node instance role may appear in a KMS key policy.
- **Agent 2** (sometimes): `eks:DescribeNodegroup` → `LaunchTemplate.Id`/`Version`; `ec2:DescribeLaunchTemplateVersions.LaunchTemplateData.BlockDeviceMappings[].Ebs.KmsKeyId`. No KMS directly on nodegroup
- **Agent 3** (sometimes): `eks:DescribeNodegroup` → `launchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `BlockDeviceMappings[].Ebs.KmsKeyId`; also `eks:DescribeCluster` → `encryptionConfig[].provider.keyArn` applies to nodes' secrets
- **Agent 4** (sometimes): `eks:DescribeNodegroup` → `LaunchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.BlockDeviceMappings[].Ebs.KmsKeyId`. Otherwise EBS defaults may involve an AWS-managed key.
- **Agent 5** (sometimes): `eks:DescribeNodegroup` → launch template → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.BlockDeviceMappings[].Ebs.KmsKeyId`. Cluster secrets encryption: `eks:DescribeCluster` → `EncryptionConfig[].Provider.KeyArn`.

### `ng` → `subnet` — yes (5/5 agree)

- **Agent 1** (yes): `eks:DescribeNodegroup` → `Subnets[]`.
- **Agent 2** (yes): `eks:DescribeNodegroup` → `Subnets[]`
- **Agent 3** (yes): `eks:DescribeNodegroup` → `subnets[]` (subnet IDs directly)
- **Agent 4** (yes): `eks:DescribeNodegroup` → `Subnets[]`.
- **Agent 5** (yes): `eks:DescribeNodegroup` → `Subnets[]` field lists subnet IDs.

### `opensearch` → `role` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `opensearch:DescribeDomain(DomainName)` → `DomainStatus.AccessPolicies` (parse `Principal.AWS` for role ARNs); also `AdvancedSecurityOptions.MasterUserOptions.MasterUserARN`.
- **Agent 2** (sometimes): `opensearch:DescribeDomain` → `AccessPolicies` JSON; parse `Statement[].Principal.AWS` for role ARNs; also `AdvancedSecurityOptions.MasterUserOptions.MasterUserARN`
- **Agent 3** (sometimes): `opensearch:DescribeDomainConfig` / `opensearch:DescribeDomain` → `AccessPolicies` JSON → parse `Statement[].Principal.AWS` for role ARNs; the `AWSServiceRoleForAmazonOpenSearchService` SLR is implicit
- **Agent 4** (sometimes): `opensearch:DescribeDomain` → `AccessPolicies` (parse `Principal.AWS` for role ARNs), `CognitoOptions.RoleArn`, `AdvancedSecurityOptions.MasterUserOptions.MasterUserARN` if it's a role.
- **Agent 5** (sometimes): `opensearch:DescribeDomain` → `AccessPolicies` (JSON policy) → parse `Statement[].Principal.AWS` for role ARNs. Also `AdvancedSecurityOptions.MasterUserOptions.MasterUserARN`.

### `pipeline` → `eb-rule` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (yes): `events:ListRuleNamesByTarget(TargetArn=<pipelineArn>)` → rules that trigger this pipeline. Also `codepipeline:GetPipeline` → `Triggers[]` (v2 pipelines) references connections, not EB rules directly.
- **Agent 2** (yes): `codepipeline:GetPipeline` → determine triggers; `events:ListRules` with prefix `codepipeline-` + `events:ListTargetsByRule` where Target `Arn` matches pipeline ARN; OR `events:ListRuleNamesByTarget` with `TargetArn=<pipeline-arn>`
- **Agent 3** (yes): `events:ListRuleNamesByTarget` with pipeline ARN → rule names (CodePipeline is commonly triggered by EventBridge rules on CodeCommit/ECR/S3 events); also `codepipeline:GetPipeline` triggers
- **Agent 4** (sometimes): `events:ListRuleNamesByTarget` with the pipeline ARN → rules; then `events:DescribeRule`. Also `codepipeline:ListWebhooks` for GitHub-style triggers.
- **Agent 5** (yes): `codepipeline:GetPipeline` typically has an EventBridge rule created for CodeCommit/S3/ECR sources. `events:ListRuleNamesByTarget` with the pipeline ARN `arn:aws:codepipeline:...:<pipeline>`.

### `pipeline` → `logs` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `codepipeline:GetPipeline` → for each CodeBuild action, `codebuild:BatchGetProjects([projectName])` → `LogsConfig.CloudWatchLogs.GroupName`. Aggregates log groups of embedded CB projects.
- **Agent 2** (sometimes): `codepipeline:GetPipeline` → `Stages[].Actions[]` with `ActionTypeId.Provider=CodeBuild`; then `codebuild:BatchGetProjects` → `LogsConfig.CloudWatchLogs.GroupName`. Pipeline itself → CloudTrail only
- **Agent 3** (sometimes): `codepipeline:GetPipeline` → for each Build action `configuration.ProjectName`; `codebuild:BatchGetProjects` → `logsConfig.cloudWatchLogs.groupName`
- **Agent 4** (sometimes): `codepipeline:GetPipeline` → for each CodeBuild action `Configuration.ProjectName` → `codebuild:BatchGetProjects` → `LogsConfig.CloudWatchLogs.GroupName`.
- **Agent 5** (sometimes): `codepipeline:GetPipeline` → extract CodeBuild project names from actions → `codebuild:BatchGetProjects` → `LogsConfig.CloudWatchLogs.GroupName`. No pipeline-level log group.

### `rds-snap` → `backup` — sometimes (4/5) — split: sometimes:4, yes:1

- **Agent 1** (sometimes): `backup:ListRecoveryPointsByResource(ResourceArn=<rdsSnapshotArn or dbInstanceArn>)`. Note: Backup tracks the DB instance/cluster, not each manual snapshot individually.
- **Agent 2** (sometimes): `backup:ListProtectedResources` filter `ResourceType=RDS`; `backup:ListRecoveryPointsByResource` with `ResourceArn=<db-cluster-or-instance-arn>`; the snapshot may appear as a recovery point
- **Agent 3** (yes): `backup:ListRecoveryPointsByResource` with the RDS DB instance/cluster ARN; or `backup:ListProtectedResources` filter `ResourceType=RDS`/`Aurora`
- **Agent 4** (sometimes): `backup:ListRecoveryPointsByResource` with the DB instance/cluster ARN, or inspect snapshot tags `aws:backup:source-resource`; also `backup:DescribeRecoveryPoint` with the backup-vault-level recovery point ARN.
- **Agent 5** (sometimes): `backup:ListRecoveryPointsByResource` with the DB cluster/instance ARN; match `CreatedBy.BackupVaultName` / `RecoveryPointArn`. Or check snapshot tags for `aws:backup:source-resource`.

### `role` → `kms` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Reverse scan: `kms:ListKeys` → `kms:GetKeyPolicy(KeyId,PolicyName="default")` → parse principals for this role ARN; and `kms:ListGrants(KeyId)` → `GranteePrincipal == <roleArn>`. Expensive. Also `iam:ListAttachedRolePolicies`+`iam:GetPolicyVersion` — search `Resource` for KMS ARNs.
- **Agent 2** (sometimes): `iam:ListAttachedRolePolicies` + `iam:ListRolePolicies` → parse for `kms:*` Resource ARNs; OR iterate `kms:ListKeys` + `kms:GetKeyPolicy` + `kms:ListGrants` looking for role ARN (expensive fanout)
- **Agent 3** (sometimes): Primary: iterate `kms:ListKeys` → `kms:GetKeyPolicy` → parse principals for this role; also `kms:ListGrants` per key → `GranteePrincipal` is role ARN. Secondary: `iam:ListAttachedRolePolicies`+`iam:ListRolePolicies` → `iam:GetPolicyVersion`/`GetRolePolicy` → parse statements' `Resource` for `arn:aws:kms:...`
- **Agent 4** (sometimes): `iam:ListAttachedRolePolicies` + `iam:ListRolePolicies` → `GetPolicyVersion`/`GetRolePolicy` → parse for `kms:*` actions with key ARN/alias resources. Reverse: iterate `kms:ListKeys` → `GetKeyPolicy` → `Principal.AWS` == role ARN.
- **Agent 5** (sometimes): `iam:ListAttachedRolePolicies` + `iam:ListRolePolicies` → `iam:GetPolicyVersion`/`GetRolePolicy` → scan statements for `Resource: arn:aws:kms:...`. Converse: list-all `kms:ListKeys` → `kms:GetKeyPolicy` → scan `Principal.AWS` for the role ARN; also `kms:ListGrants` → `GranteePrincipal`.

### `secrets` → `codeartifact` — sometimes (3/5) — split: sometimes:3, no:2

- **Agent 1** (no): (Secrets Manager does not natively integrate with CodeArtifact; any coupling would be via user buildspec/code)
- **Agent 2** (sometimes): `secretsmanager:DescribeSecret` → tags/description; no first-class API link. `unsure — would need to verify AWS docs on CodeArtifact token storage convention`
- **Agent 3** (sometimes): No direct AWS API link. `secretsmanager:DescribeSecret` has no CodeArtifact field. Heuristic: name-match `Name` containing "codeartifact", or parse `GetResourcePolicy`. Unsure — would need to verify AWS docs on CodeArtifact secret integration
- **Agent 4** (sometimes): `codeartifact:DescribeDomain`/`DescribeRepository` has no secret field directly. No reliable direct API link. `resourcegroupstaggingapi:GetResources` filtered to both types may find matching tags.
- **Agent 5** (no): — (Secrets Manager and CodeArtifact have no direct relationship)

### `secrets` → `eb` — sometimes (4/5) — split: sometimes:4, no:1

- **Agent 1** (sometimes): Reverse scan: `elasticbeanstalk:DescribeConfigurationSettings` across all envs → `OptionSettings` with values containing the secret ARN. Expensive.
- **Agent 2** (no):
- **Agent 3** (sometimes): No direct API. `elasticbeanstalk:DescribeConfigurationSettings` across envs → scan option settings for the secret ARN/name. Heuristic
- **Agent 4** (sometimes): Iterate `elasticbeanstalk:DescribeConfigurationSettings` for all environments → scan `OptionSettings[].Value` for `{{resolve:secretsmanager:<arn>}}` matching the secret.
- **Agent 5** (sometimes): List-all approach: `elasticbeanstalk:DescribeEnvironments` → `DescribeConfigurationSettings` → scan `OptionSettings` for env vars referencing the secret ARN. No native EB→secret API.

### `secrets` → `ecr` — no (5/5 agree)

- **Agent 1** (no): (Secrets Manager has no ECR linkage)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (no direct relationship; ECR pull creds may use secrets for non-AWS registries, not ECR itself)

### `secrets` → `ecs-task` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (yes): Reverse scan: `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom == <secretArn>`.
- **Agent 2** (yes): `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → match `ContainerDefinitions[].Secrets[].ValueFrom` or `RepositoryCredentials.CredentialsParameter` against `<secret-arn>` (expensive fanout)
- **Agent 3** (yes): `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `containerDefinitions[].secrets[].valueFrom` matches the secret ARN; also `repositoryCredentials.credentialsParameter`
- **Agent 4** (yes): Iterate `ecs:ListTaskDefinitions` → `DescribeTaskDefinition` → scan `ContainerDefinitions[].Secrets[].ValueFrom` and `RepositoryCredentials.CredentialsParameter` for the secret ARN.
- **Agent 5** (sometimes): List-all approach: `ecs:ListTaskDefinitions` → for each `ecs:DescribeTaskDefinition` → scan `ContainerDefinitions[].Secrets[].ValueFrom` for the secret ARN.

### `secrets` → `logs` — sometimes (3/5) — split: sometimes:3, no:2

- **Agent 1** (no): (Secrets Manager does not log to a customer-owned CWL group directly; only via CloudTrail)
- **Agent 2** (sometimes): `secretsmanager:DescribeSecret` → `RotationLambdaARN`; then `lambda:GetFunctionConfiguration` — Lambda logs go to `/aws/lambda/<function-name>` in CWL
- **Agent 3** (sometimes): `secretsmanager:DescribeSecret` → `RotationLambdaARN`; then `logs:DescribeLogGroups` with prefix `/aws/lambda/<function-name>`
- **Agent 4** (sometimes): `secretsmanager:DescribeSecret` → `RotationLambdaARN` → `lambda:GetFunctionConfiguration` → `LoggingConfig.LogGroup` (or default `/aws/lambda/<name>`).
- **Agent 5** (no): — (Secrets Manager audit is via CloudTrail, not CloudWatch Logs directly)

### `secrets` → `pipeline` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Reverse scan: `codepipeline:ListPipelines` → `GetPipeline` → action `Configuration` values containing `arn:aws:secretsmanager:...` or inspect CB projects (via `codebuild:BatchGetProjects` → `Environment.EnvironmentVariables[].Type == "SECRETS_MANAGER"`).
- **Agent 2** (sometimes): `codepipeline:GetPipeline` + `codebuild:BatchGetProjects` → `Environment.EnvironmentVariables[]` where `Type=SECRETS_MANAGER` and `Value=<secret-arn>` (expensive fanout)
- **Agent 3** (sometimes): No direct API. `codepipeline:ListPipelines` → `GetPipeline` → inspect action `configuration` env vars; more commonly `codebuild:BatchGetProjects` → `environment.environmentVariables[].type=SECRETS_MANAGER` → `value` is the secret name
- **Agent 4** (sometimes): Iterate `codepipeline:ListPipelines` → `GetPipeline` → scan action `Configuration` JSON values and associated CodeBuild projects' env vars for the secret ARN.
- **Agent 5** (sometimes): List-all approach: `codepipeline:ListPipelines` → `codepipeline:GetPipeline` → scan `Stages[].Actions[].Configuration` values for the secret ARN / `{{resolve:secretsmanager:...}}`.

### `secrets` → `role` — sometimes (4/5) — split: sometimes:4, yes:1

- **Agent 1** (sometimes): `secretsmanager:GetResourcePolicy(SecretId)` → parse `Principal.AWS` for role ARNs. Also `DescribeSecret` → `RotationLambdaARN` → `lambda:GetFunctionConfiguration` → `Role`.
- **Agent 2** (sometimes): `secretsmanager:GetResourcePolicy` with `SecretId` → parse `Statement[].Principal.AWS` for role ARNs; also `secretsmanager:DescribeSecret.RotationLambdaARN` → Lambda's execution role
- **Agent 3** (yes): `secretsmanager:GetResourcePolicy` → parse `Statement[].Principal.AWS` for role ARNs; also rotation role: `secretsmanager:DescribeSecret` → `RotationLambdaARN` → `lambda:GetFunctionConfiguration` → `Role`
- **Agent 4** (sometimes): `secretsmanager:GetResourcePolicy` → parse `Principal.AWS`; plus `DescribeSecret` → `RotationLambdaARN` → `lambda:GetFunctionConfiguration` → `Role`.
- **Agent 5** (sometimes): `secretsmanager:GetResourcePolicy` → parse `ResourcePolicy.Statement[].Principal.AWS`. Also `secretsmanager:DescribeSecret` → rotation Lambda's execution role.

### `secrets` → `s3` — no (5/5 agree)

- **Agent 1** (no): (Secrets Manager has no direct S3 relationship; secret value is not stored in a customer bucket)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (no direct relationship)

### `secrets` → `sns` — sometimes (3/5) — split: sometimes:3, no:2

- **Agent 1** (sometimes): No direct SecretsManager→SNS API. Via CW alarms: `cloudwatch:DescribeAlarms` on `AWS/SecretsManager` namespace and secret dimensions → `AlarmActions` SNS ARNs.
- **Agent 2** (sometimes): `events:ListRules` + `events:ListTargetsByRule` where `EventPattern.source=aws.secretsmanager` and `Target.Arn` is SNS ARN; no direct Secrets Manager field
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (sometimes): `secretsmanager:DescribeSecret` → `RotationLambdaARN`. No direct SNS topic field on the secret. Check via EventBridge rules filtering `source: aws.secretsmanager`.

### `ses` → `acm` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `ses:GetIdentityVerificationAttributes` / `ses:GetEmailIdentity` (SES v2) → not exposed. No direct ACM linkage. Mostly `no`.
- **Agent 2** (sometimes): `unsure — would need to verify AWS docs on SES custom TLS cert usage`
- **Agent 3** (sometimes): No direct API link. Unsure — would need to verify AWS docs on SES identity + ACM. Heuristic: cross-check identity domain against ACM certs issued for that domain
- **Agent 4** (sometimes): No direct SES API. Match by domain: `sesv2:GetEmailIdentity` → identity (domain) → `acm:ListCertificates` → find certs whose `DomainName`/`SubjectAlternativeNames` match.
- **Agent 5** (sometimes): Not a direct API relationship. `ses:GetIdentityDkimAttributes` / `ses:GetIdentityMailFromDomainAttributes`. No ACM ARN returned.

### `ses` → `alarm` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Reverse: `cloudwatch:DescribeAlarmsForMetric(Namespace=AWS/SES, MetricName=Reputation.BounceRate or Reputation.ComplaintRate)` with identity dimension.
- **Agent 2** (sometimes): `cloudwatch:DescribeAlarms` → filter `Namespace=AWS/SES` and `Dimensions` (if any); OR search alarm metric for `AWS/SES`
- **Agent 3** (sometimes): `cloudwatch:DescribeAlarms` → filter `Namespace=AWS/SES` and `Dimensions` containing configuration-set or identity matching the SES resource
- **Agent 4** (sometimes): Iterate `cloudwatch:DescribeAlarms` filtered by `Namespace == "AWS/SES"` and `Dimensions[].Name == "Identity"` or `ConfigurationSet`.
- **Agent 5** (sometimes): List-all approach: `cloudwatch:DescribeAlarms` filter `Namespace: AWS/SES` or `AWS/SES/ConfigurationSet` and match dimensions (e.g., `ConfigurationSet`).

### `ses` → `cfn` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Tag-based: `resourcegroupstaggingapi:GetResources` on the SES resource ARN → tags `aws:cloudformation:stack-id`.
- **Agent 2** (sometimes): `ses:GetEmailIdentity`/`ses:ListConfigurationSets` → tags (`aws:cloudformation:stack-name`); `cloudformation:DescribeStackResources` filter `ResourceType=AWS::SES::*`
- **Agent 3** (sometimes): `resourcegroupstaggingapi:GetResources` with `ResourceTypeFilters=["ses:identity","ses:configuration-set"]` → tags `aws:cloudformation:stack-name`
- **Agent 4** (sometimes): `resourcegroupstaggingapi:GetResources` on the SES identity ARN → tags `aws:cloudformation:stack-id`; then `cloudformation:DescribeStacks`.
- **Agent 5** (sometimes): SES identities/config sets do not expose tags in all APIs; `resourcegroupstaggingapi:GetResources` with `ses:identity`/`ses:configuration-set` → filter for `aws:cloudformation:stack-id` tag, then `cloudformation:DescribeStacks`.

### `ses` → `eb-rule` — sometimes (4/5) — split: sometimes:4, yes:1

- **Agent 1** (yes): `sesv2:GetEmailIdentity(EmailIdentity)` → nothing about events; events go through a configuration set → `sesv2:GetConfigurationSetEventDestinations(ConfigurationSetName)` → destinations include `EventBridgeDestination.EventBusArn`. Match EB rules on that bus.
- **Agent 2** (sometimes): `ses:GetConfigurationSetEventDestinations`/`sesv2:GetConfigurationSetEventDestinations` → `EventDestinations[]` where `EventBridgeDestination` or legacy `SNSDestination` present; `events:ListRules` filtered by `source=aws.ses`
- **Agent 3** (sometimes): `events:ListRules` → `events:DescribeRule` → parse `EventPattern` for `"source":["aws.ses"]` and dimension filters matching identity/config-set
- **Agent 4** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → destinations with `EventBridgeDestination.EventBusArn`. Then `events:ListRules --event-bus-name` → rules on that bus.
- **Agent 5** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → `EventDestinations[].EventBridgeDestination.EventBusArn`. Then `events:ListRules --event-bus-name <bus>`.

### `ses` → `kinesis` — yes (3/5) — split: yes:3, sometimes:2

- **Agent 1** (yes): `sesv2:GetConfigurationSetEventDestinations(ConfigurationSetName)` → `EventDestinations[].KinesisFirehoseDestination.DeliveryStreamArn` (Firehose, not Kinesis Data Streams directly — but Firehose ARNs are in `firehose:`). For raw Kinesis: none natively.
- **Agent 2** (yes): `sesv2:GetConfigurationSetEventDestinations` → `EventDestinations[].KinesisFirehoseDestination.IAMRoleARN` and `DeliveryStreamARN`
- **Agent 3** (sometimes): `ses:ListConfigurationSets` → `sesv2:GetConfigurationSetEventDestinations` (or v1: `ses:DescribeConfigurationSet` with `EventDestinations`) → destinations of type `KinesisFirehoseDestination` have `DeliveryStreamArn`. For native Kinesis Data Streams, no direct SES destination
- **Agent 4** (yes): `sesv2:GetConfigurationSetEventDestinations` → event destinations with `KinesisFirehoseDestination.DeliveryStreamArn` (Firehose) or `KinesisFirehoseDestination.IamRoleArn`. SES writes events to Firehose, not directly to Kinesis Data Streams.
- **Agent 5** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → `EventDestinations[].KinesisFirehoseDestination.DeliveryStreamArn`.

### `ses` → `kms` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `ses:DescribeActiveReceiptRuleSet` → rules with `S3Action.KmsKeyArn`. For sending: `sesv2:GetEmailIdentity` → `DkimAttributes` (no KMS).
- **Agent 2** (sometimes): `ses:DescribeActiveReceiptRuleSet` → rule `S3Action.KmsKeyArn`; for SMTP credentials IAM user, indirect only
- **Agent 3** (sometimes): `ses:DescribeReceiptRule` → `Actions[].S3Action.KmsKeyArn`; also SESv2 identity DKIM has no KMS
- **Agent 4** (sometimes): Receiving: `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].S3Action.KmsKeyArn`. Sending: no direct field.
- **Agent 5** (sometimes): `ses:DescribeReceiptRule` → `S3Action.KmsKeyArn`. Otherwise no direct KMS link.

### `ses` → `lambda` — sometimes (4/5) — split: sometimes:4, yes:1

- **Agent 1** (sometimes): `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].LambdaAction.FunctionArn`.
- **Agent 2** (yes): `ses:DescribeActiveReceiptRuleSet` → rule `LambdaAction.FunctionArn`; also `sesv2:GetConfigurationSetEventDestinations` — Lambda is not a direct destination (must go via SNS/EventBridge)
- **Agent 3** (sometimes): `ses:DescribeActiveReceiptRuleSet` → rules[].actions[].LambdaAction.FunctionArn`
- **Agent 4** (sometimes): `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].LambdaAction.FunctionArn`. Also Firehose transforms: Firehose stream ARN → `firehose:DescribeDeliveryStream` → `ProcessingConfiguration.Processors[].Parameters.LambdaArn`.
- **Agent 5** (sometimes): `ses:DescribeActiveReceiptRuleSet` + `ses:DescribeReceiptRule` → `Actions[].LambdaAction.FunctionArn`.

### `ses` → `logs` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → CloudWatch destinations exist (`CloudWatchDestination.DimensionConfigurations` — metrics only, not logs). For logs: event→Firehose→CWL chain. Mostly `no` for a direct SES→CWL group.
- **Agent 2** (sometimes): No direct API; requires chasing event destinations (`sesv2:GetConfigurationSetEventDestinations`) and following Lambda/Firehose targets
- **Agent 3** (sometimes): No direct SES→Logs destination. SES configuration-set event destinations: CloudWatch, Kinesis Firehose, SNS, EventBridge — not Logs. Likely no
- **Agent 4** (sometimes): No direct SES→CW Logs destination. Chain: `sesv2:GetConfigurationSetEventDestinations` → CloudWatch dimension destination is metrics only; for logs follow Firehose → CW Logs destination.
- **Agent 5** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → `EventDestinations[].CloudWatchDestination`. Note: CloudWatchDestination is CloudWatch metrics, not Logs. No direct Logs destination exists; only via Kinesis Firehose→Logs.

### `ses` → `role` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `ses:ListIdentityPolicies(Identity)` → `ses:GetIdentityPolicies` → parse `Principal.AWS`; and `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].S3Action.IamRoleArn`/`SNSAction`/`BounceAction` fields.
- **Agent 2** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → `KinesisFirehoseDestination.IAMRoleARN`; `ses:DescribeActiveReceiptRuleSet` → S3 action `KmsKeyArn` (not role); SES sending requires no role
- **Agent 3** (sometimes): No direct API link. Most SES→role edges are indirect through actions (Lambda exec role, Kinesis Firehose delivery role). Heuristic only
- **Agent 4** (sometimes): `sesv2:GetEmailIdentityPolicies` / `ses:ListIdentityPolicies` → parse `Principal.AWS`; `GetConfigurationSetEventDestinations` → `KinesisFirehoseDestination.IamRoleArn`, `SnsDestination` is role-less but receiving rules' `S3Action`/`BounceAction` use the SES service role.
- **Agent 5** (sometimes): `ses:DescribeReceiptRule` → `S3Action.IamRoleArn`, `BounceAction.TopicArn` trust, etc. `sesv2:GetConfigurationSetEventDestinations` → Firehose destinations need a role (not returned directly; check Firehose's role).

### `ses` → `s3` — yes (3/5) — split: yes:3, sometimes:2

- **Agent 1** (yes): `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].S3Action.BucketName`. Also `sesv2:GetConfigurationSetEventDestinations` has no S3 destination natively (only via Firehose→S3).
- **Agent 2** (yes): `ses:DescribeActiveReceiptRuleSet` → rule `S3Action.BucketName`; also config-set Firehose destinations deliver to S3 (via Firehose's own bucket config)
- **Agent 3** (yes): `ses:DescribeActiveReceiptRuleSet` → rules[].actions[].S3Action.BucketName` (and `ObjectKeyPrefix`). SES storing received emails in S3
- **Agent 4** (sometimes): `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].S3Action.BucketName`.
- **Agent 5** (sometimes): `ses:DescribeActiveReceiptRuleSet` + `ses:DescribeReceiptRule` → `Actions[].S3Action.BucketName`.

### `ses` → `sns` — yes (3/5) — split: yes:3, sometimes:2

- **Agent 1** (yes): `sesv2:GetConfigurationSetEventDestinations(ConfigurationSetName)` → `EventDestinations[].SnsDestination.TopicArn`. Also `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].SNSAction.TopicArn`/`BounceAction.Sender` etc. And identity notifications: `ses:GetIdentityNotificationAttributes` → `BounceTopic`/`ComplaintTopic`/`DeliveryTopic`.
- **Agent 2** (yes): `ses:DescribeActiveReceiptRuleSet` → rule `SNSAction.TopicArn`; `sesv2:GetConfigurationSetEventDestinations` → `SnsDestination.TopicArn`; also `ses:GetIdentityNotificationAttributes` → `BounceTopic`/`ComplaintTopic`/`DeliveryTopic`
- **Agent 3** (yes): `ses:DescribeActiveReceiptRuleSet` → rules[].actions[].SNSAction.TopicArn`; also configuration-set event destinations:`sesv2:GetConfigurationSetEventDestinations` → `SnsDestination.TopicArn`
- **Agent 4** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → `SnsDestination.TopicArn`. Legacy identity-level: `ses:GetIdentityNotificationAttributes` → `BounceTopic`/`ComplaintTopic`/`DeliveryTopic`.
- **Agent 5** (sometimes): `ses:GetIdentityNotificationAttributes` → `BounceTopic`/`ComplaintTopic`/`DeliveryTopic`. Also `sesv2:GetConfigurationSetEventDestinations` → `SnsDestination.TopicArn`.

### `ses` → `trail` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `cloudtrail:LookupEvents(LookupAttributes=[{AttributeKey:"EventSource",AttributeValue:"ses.amazonaws.com"}])` — trails that log mgmt events. Or `cloudtrail:DescribeTrails` + `GetEventSelectors`.
- **Agent 2** (sometimes): `cloudtrail:DescribeTrails` + `cloudtrail:GetEventSelectors` — SES management events are captured by any trail logging management events. No SES-specific trail field
- **Agent 3** (sometimes): `cloudtrail:DescribeTrails` → trails are account-wide; one could filter events by `EventSource=ses.amazonaws.com`. No per-identity trail link
- **Agent 4** (sometimes): No direct API. Check `cloudtrail:DescribeTrails` → all trails implicitly capture SES control-plane events; data events are not defined for SES.
- **Agent 5** (sometimes): List-all approach: `cloudtrail:DescribeTrails` → for each `cloudtrail:GetEventSelectors` and `GetTrailStatus`; data events for SES are limited. Usually SES API calls are in management events by default.

### `sfn` → `cfn` — sometimes (5/5 agree)

- **Agent 1** (sometimes): Tag: `stepfunctions:ListTagsForResource(ResourceArn)` → `aws:cloudformation:stack-id`. Or `resourcegroupstaggingapi:GetResources` on the SFN ARN.
- **Agent 2** (sometimes): `stepfunctions:DescribeStateMachine` → tags; OR `stepfunctions:ListTagsForResource` → look for `aws:cloudformation:stack-name`; `cloudformation:DescribeStackResources` filter `ResourceType=AWS::StepFunctions::StateMachine`
- **Agent 3** (sometimes): `states:DescribeStateMachine` → `tags` include `aws:cloudformation:stack-name`; or `cloudformation:DescribeStackResources` filter `ResourceType=AWS::StepFunctions::StateMachine`
- **Agent 4** (sometimes): `stepfunctions:DescribeStateMachine` → `Tags` include `aws:cloudformation:stack-id` (if tagged); then `cloudformation:DescribeStacks`. Also check `cloudformation:DescribeStackResources` filtered by type `AWS::StepFunctions::StateMachine`.
- **Agent 5** (sometimes): `stepfunctions:DescribeStateMachine` plus `stepfunctions:ListTagsForResource` → filter for `aws:cloudformation:stack-id`, then `cloudformation:DescribeStacks`.

### `sfn` → `eb-rule` — sometimes (3/5) — split: sometimes:3, yes:2

- **Agent 1** (yes): `events:ListRuleNamesByTarget(TargetArn=<stateMachineArn>)` → rules that start this state machine.
- **Agent 2** (yes): `events:ListRuleNamesByTarget` with `TargetArn=<state-machine-arn>`
- **Agent 3** (sometimes): `events:ListRuleNamesByTarget` with the state-machine ARN → rule names
- **Agent 4** (sometimes): `events:ListRuleNamesByTarget` with the state machine ARN → rules; `events:DescribeRule`.
- **Agent 5** (sometimes): `events:ListRuleNamesByTarget` with the state machine ARN.

### `sns` → `cfn` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `sns:ListTagsForResource(ResourceArn)` → `aws:cloudformation:stack-id` tag. Or `resourcegroupstaggingapi:GetResources`.
- **Agent 2** (sometimes): `sns:GetTopicAttributes` → no direct stack link; `sns:ListTagsForResource` with `ResourceArn=<topic>` → look for `aws:cloudformation:stack-name`; OR `cloudformation:DescribeStackResources` filter `ResourceType=AWS::SNS::Topic`
- **Agent 3** (sometimes): `sns:ListTagsForResource` with topic ARN → tag `aws:cloudformation:stack-name`; or `resourcegroupstaggingapi:GetResources` filter `ResourceTypeFilters=["sns:topic"]`
- **Agent 4** (sometimes): `sns:ListTagsForResource` on the topic ARN → tag `aws:cloudformation:stack-id`; then `cloudformation:DescribeStacks`.
- **Agent 5** (sometimes): `sns:ListTagsForResource` on the topic ARN → filter for `aws:cloudformation:stack-id`, then `cloudformation:DescribeStacks`.

### `sns-sub` → `ecs` — no (4/5) — split: no:4, sometimes:1

- **Agent 1** (sometimes): No direct API. `sns:GetSubscriptionAttributes(SubscriptionArn)` → `Endpoint` field. If `Protocol=="sqs"`, trace SQS queue consumers via CloudWatch or inspect ECS task definitions referencing that queue URL in env — reverse iteration. Mostly `no`.
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (SNS subscriptions don't target ECS clusters/services directly; only via EventBridge or Lambda shim)

### `sns-sub` → `kms` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `sns:GetTopicAttributes(TopicArn)` (topic is derived from subscription's `TopicArn`) → `Attributes.KmsMasterKeyId`.
- **Agent 2** (sometimes): `sns:GetTopicAttributes` with the subscription's `TopicArn` → `KmsMasterKeyId`; subscription has no KMS field
- **Agent 3** (sometimes): `sns:GetSubscriptionAttributes` → `Endpoint` (e.g., SQS ARN); then `sqs:GetQueueAttributes` → `KmsMasterKeyId`; for Firehose: `firehose:DescribeDeliveryStream` → `DeliveryStreamEncryptionConfiguration.KeyARN`
- **Agent 4** (sometimes): `sns:GetSubscriptionAttributes` → `Endpoint`. Based on endpoint type: SQS → `sqs:GetQueueAttributes` → `KmsMasterKeyId`; Firehose → `firehose:DescribeDeliveryStream` → server-side encryption config; Lambda → `lambda:GetFunctionConfiguration` → `KMSKeyArn`.
- **Agent 5** (sometimes): `sns:GetSubscriptionAttributes` with subscription ARN does not return a KMS ARN. Must inspect the parent topic `sns:GetTopicAttributes` → `KmsMasterKeyId`.

### `sns-sub` → `policy` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `sns:GetSubscriptionAttributes(SubscriptionArn)` → `Attributes.FilterPolicy`, `DeliveryPolicy`, `RedrivePolicy`. These are the "policies" on a subscription.
- **Agent 2** (sometimes): `sns:GetSubscriptionAttributes` with `SubscriptionArn` → `FilterPolicy`, `DeliveryPolicy`, `RedrivePolicy`, `EffectiveDeliveryPolicy`
- **Agent 3** (sometimes): `sns:GetSubscriptionAttributes` → `FilterPolicy`, `DeliveryPolicy`, `RedrivePolicy`; parent `sns:GetTopicAttributes` → `Policy`
- **Agent 4** (sometimes): `sns:GetSubscriptionAttributes` → attributes `DeliveryPolicy`, `FilterPolicy`, `RedrivePolicy`, `EffectiveDeliveryPolicy`. No separate resource.
- **Agent 5** (sometimes): `sns:GetSubscriptionAttributes` → `FilterPolicy`, `RedrivePolicy`, `DeliveryPolicy`.

### `sqs` → `eb-rule` — yes (3/5) — split: yes:3, sometimes:2

- **Agent 1** (yes): `events:ListRuleNamesByTarget(TargetArn=<queueArn>)` → rules that target this SQS queue.
- **Agent 2** (yes): `events:ListRuleNamesByTarget` with `TargetArn=<queue-arn>`
- **Agent 3** (yes): `events:ListRuleNamesByTarget` with SQS queue ARN → rules targeting this queue
- **Agent 4** (sometimes): `events:ListRuleNamesByTarget` with the queue ARN → rules; `events:DescribeRule`.
- **Agent 5** (sometimes): `events:ListRuleNamesByTarget` with the queue ARN.

### `sqs` → `role` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `sqs:GetQueueAttributes(QueueUrl,AttributeNames=["Policy"])` → parse `Statement[].Principal.AWS` for role ARNs.
- **Agent 2** (sometimes): `sqs:GetQueueAttributes` with `AttributeNames=[Policy]` → parse `Statement[].Principal.AWS` for role ARNs; also `lambda:ListEventSourceMappings` with `EventSourceArn=<queue-arn>` → function role
- **Agent 3** (sometimes): `sqs:GetQueueAttributes` with `AttributeName=Policy` → parse `Statement[].Principal.AWS` for role ARNs
- **Agent 4** (sometimes): `sqs:GetQueueAttributes` → `Policy` → parse `Statement[].Principal.AWS` for role ARNs.
- **Agent 5** (sometimes): `sqs:GetQueueAttributes` with `AttributeNames=[Policy]` → parse policy JSON `Statement[].Principal.AWS` for role ARNs.

### `tg` → `kms` — no (5/5 agree)

- **Agent 1** (no): (ALB/NLB target groups have no KMS attribute; encryption is on the listener's ACM cert)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (ELBv2 target groups have no direct KMS encryption relationship)

### `tg` → `role` — no (3/5) — split: no:3, sometimes:2

- **Agent 1** (no): (Target groups have no IAM role attribute)
- **Agent 2** (sometimes): `elbv2:DescribeTargetGroups` → `TargetType`; if `lambda` → `elbv2:DescribeTargetHealth` → `Target.Id` (Lambda ARN) → `lambda:GetFunctionConfiguration.Role`
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (sometimes): `elbv2:DescribeTargetGroups` → `TargetType == "lambda"` → `elbv2:DescribeTargetHealth` → `Target.Id` = Lambda ARN → `lambda:GetFunctionConfiguration` → `Role`.

### `tg` → `secrets` — no (5/5 agree)

- **Agent 1** (no): (Target groups have no Secrets Manager relationship)
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (target groups have no secret association)

### `tgw` → `cfn` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `ec2:DescribeTransitGateways(TransitGatewayIds=[id])` → `Tags[]` → `aws:cloudformation:stack-id`. Or `resourcegroupstaggingapi:GetResources` on TGW ARN.
- **Agent 2** (sometimes): `ec2:DescribeTransitGateways` → `Tags` filter `aws:cloudformation:stack-name`; OR `cloudformation:DescribeStackResources` filter `ResourceType=AWS::EC2::TransitGateway`
- **Agent 3** (sometimes): `ec2:DescribeTransitGateways` → `Tags` with `aws:cloudformation:stack-name`; or `resourcegroupstaggingapi:GetResources` filter `ResourceTypeFilters=["ec2:transit-gateway"]`
- **Agent 4** (sometimes): `ec2:DescribeTransitGateways` → `Tags` `aws:cloudformation:stack-id`; then `cloudformation:DescribeStacks`.
- **Agent 5** (sometimes): `ec2:DescribeTransitGateways` → `Tags` → filter `aws:cloudformation:stack-id`, then `cloudformation:DescribeStacks`.

### `tgw` → `role` — sometimes (3/5) — split: sometimes:3, no:2

- **Agent 1** (sometimes): No per-TGW role attribute. SLR `AWSServiceRoleForVPCTransitGateway` via `iam:GetRole`. For RAM-shared TGWs: `ram:ListResources(resourceArns=[tgwArn])` → principals (accounts, not roles typically). Mostly `no` for a direct per-TGW role.
- **Agent 2** (sometimes): `ram:GetResourceShares` + `ram:ListResources` filtered by TGW ARN → `ram:ListPrincipals` (returns accounts/OUs/roles); TGW itself has no role field
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (sometimes): `iam:GetRole` with name `AWSServiceRoleForVPCTransitGateway`. No TGW-ARN-scoped API returns it; relationship is implicit.

### `tgw` → `rtb` — yes (4/5) — split: yes:4, sometimes:1

- **Agent 1** (yes): `ec2:DescribeTransitGatewayRouteTables(Filters=[{Name:"transit-gateway-id",Values:[<tgwId>]}])` → TGW route tables (these are TGW RTBs, not VPC RTBs). For VPC RTBs with TGW routes: `ec2:DescribeRouteTables(Filters=[{Name:"route.transit-gateway-id",Values:[<tgwId>]}])`.
- **Agent 2** (yes): `ec2:DescribeTransitGatewayRouteTables` with `Filters=[transit-gateway-id]`; VPC route tables that route to the TGW: `ec2:DescribeRouteTables` filter `Routes.TransitGatewayId=<tgw-id>`
- **Agent 3** (sometimes): `ec2:DescribeTransitGatewayAttachments` filter by `transit-gateway-id` → attachment IDs; for VPC attachments: `ec2:DescribeRouteTables` filter `route.transit-gateway-id=<tgw-id>` → VPC route tables with a route to this TGW
- **Agent 4** (yes): `ec2:DescribeTransitGatewayRouteTables` with filter `transit-gateway-id` → TGW route tables. Note: these are TGW route tables, not VPC route tables. VPC-level RTBs are linked via `ec2:DescribeRouteTables` filter `route.transit-gateway-id`.
- **Agent 5** (yes): `ec2:DescribeTransitGatewayRouteTables` with filter `transit-gateway-id=<id>`. Returns TGW route tables (not VPC route tables — note: TGW RT is a distinct resource). For VPC route tables that route through the TGW: `ec2:DescribeRouteTables` → filter `route.transit-gateway-id=<id>`.

### `tgw` → `subnet` — yes (5/5 agree)

- **Agent 1** (yes): `ec2:DescribeTransitGatewayAttachments(Filters=[{Name:"transit-gateway-id",Values:[<tgwId>]}])` → for VPC attachments: `ec2:DescribeTransitGatewayVpcAttachments` → `SubnetIds[]`.
- **Agent 2** (yes): `ec2:DescribeTransitGatewayVpcAttachments` with `Filters=[transit-gateway-id]` → `SubnetIds[]`; also `ec2:DescribeTransitGatewayAttachments`
- **Agent 3** (yes): `ec2:DescribeTransitGatewayVpcAttachments` filter by `transit-gateway-id` → `SubnetIds[]`
- **Agent 4** (yes): `ec2:DescribeTransitGatewayAttachments` with filter `transit-gateway-id` and `resource-type=vpc` → attachment IDs; `ec2:DescribeTransitGatewayVpcAttachments` → `SubnetIds[]`.
- **Agent 5** (yes): `ec2:DescribeTransitGatewayAttachments` with filter `transit-gateway-id=<id>` and `resource-type=vpc` → `TransitGatewayVpcAttachmentId`. Then `ec2:DescribeTransitGatewayVpcAttachments` → `SubnetIds[]`.

### `tgw` → `vpc` — yes (5/5 agree)

- **Agent 1** (yes): `ec2:DescribeTransitGatewayVpcAttachments(Filters=[{Name:"transit-gateway-id",Values:[<tgwId>]}])` → `VpcId` for each attachment.
- **Agent 2** (yes): `ec2:DescribeTransitGatewayVpcAttachments` with `Filters=[transit-gateway-id]` → `VpcId`
- **Agent 3** (yes): `ec2:DescribeTransitGatewayVpcAttachments` filter by `transit-gateway-id` → `VpcId` per attachment
- **Agent 4** (yes): `ec2:DescribeTransitGatewayVpcAttachments` with filter `transit-gateway-id` → `VpcId` for each attachment.
- **Agent 5** (yes): `ec2:DescribeTransitGatewayAttachments` with filter `transit-gateway-id=<id>` and `resource-type=vpc` → `ResourceId` is the VPC ID.

### `waf` → `role` — sometimes (5/5 agree)

- **Agent 1** (sometimes): `wafv2:GetLoggingConfiguration(ResourceArn=<webAclArn>)` → `LogDestinationConfigs[]` (Firehose ARNs). Firehose then has an IAM role: `firehose:DescribeDeliveryStream(DeliveryStreamName)` → `DeliveryStreamDescription.Destinations[].ExtendedS3DestinationDescription.RoleARN`.
- **Agent 2** (sometimes): `wafv2:GetLoggingConfiguration` with `ResourceArn=<webacl-arn>` → `LogDestinationConfigs[]` (Firehose/S3/CWL ARN); for Firehose: `firehose:DescribeDeliveryStream` → `ExtendedS3DestinationDescription.RoleARN`
- **Agent 3** (sometimes): `wafv2:GetLoggingConfiguration` with WebACL ARN → `LogDestinationConfigs` (Firehose ARN); the SLR `AWSServiceRoleForWAFLogging` is implicit. No direct role ARN on the WebACL
- **Agent 4** (sometimes): `wafv2:GetLoggingConfiguration` → `LogDestinationConfigs` (Firehose/S3/CW Logs ARNs); Firehose → `firehose:DescribeDeliveryStream` → `*Configuration.RoleARN`. Also `wafv2:GetPermissionPolicy` on Firewall Manager-managed ACLs for principals.
- **Agent 5** (sometimes): `wafv2:GetLoggingConfiguration` with WebACL ARN → `LogDestinationConfigs[]` (Firehose/S3/Logs ARNs). The Firehose delivery stream's role is not in the WAF API response — fetch separately via `firehose:DescribeDeliveryStream`. No direct WAF→role field.

## Unanimous `no` (drop from golden contract?)

| Parent | Related |
|--------|---------|
| `codeartifact` | `acm` |
| `codeartifact` | `kinesis` |
| `codeartifact` | `lambda` |
| `codeartifact` | `logs` |
| `codeartifact` | `r53` |
| `codeartifact` | `waf` |
| `ddb` | `secrets` |
| `eip` | `kms` |
| `lambda` | `asg` |
| `secrets` | `ecr` |
| `secrets` | `s3` |
| `tg` | `kms` |
| `tg` | `secrets` |

## Unanimous `yes` (implementable, prioritize)

### `asg` → `ami`

- **Agent 1**: `autoscaling:DescribeAutoScalingGroups` → get `LaunchConfigurationName` or `LaunchTemplate`/`MixedInstancesPolicy`. If LC: `autoscaling:DescribeLaunchConfigurations` → `ImageId`. If LT: `ec2:DescribeLaunchTemplateVersions` (version `$Latest` or `$Default` or the pinned one) → `LaunchTemplateData.ImageId`.
- **Agent 2**: `autoscaling:DescribeAutoScalingGroups` → read `LaunchConfigurationName` or `LaunchTemplate`/`MixedInstancesPolicy`; then `autoscaling:DescribeLaunchConfigurations` → `ImageId`, OR `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`
- **Agent 3**: `autoscaling:DescribeAutoScalingGroups` → get `LaunchConfigurationName` or `LaunchTemplate`/`MixedInstancesPolicy`; then `autoscaling:DescribeLaunchConfigurations` (→ `ImageId`) or `ec2:DescribeLaunchTemplateVersions` (→ `LaunchTemplateData.ImageId`)
- **Agent 4**: `autoscaling:DescribeAutoScalingGroups` with the ASG name → read `LaunchTemplate.LaunchTemplateId`/`Version` or `LaunchConfigurationName` or `MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification`. Then: for launch template `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`; for launch configuration `autoscaling:DescribeLaunchConfigurations` → `ImageId`.
- **Agent 5**: `autoscaling:DescribeAutoScalingGroups` → get `LaunchConfigurationName` or `LaunchTemplate`/`MixedInstancesPolicy`. If LaunchConfiguration: `autoscaling:DescribeLaunchConfigurations` → `ImageId`. If LaunchTemplate: `ec2:DescribeLaunchTemplateVersions` with the template ID/version → `LaunchTemplateData.ImageId`.

### `asg` → `sg`

- **Agent 1**: `autoscaling:DescribeAutoScalingGroups` → LaunchConfig → `SecurityGroups`; or LaunchTemplate → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.SecurityGroupIds` / `NetworkInterfaces[].Groups`.
- **Agent 2**: `autoscaling:DescribeAutoScalingGroups` → `LaunchConfigurationName` or `LaunchTemplate`; then `autoscaling:DescribeLaunchConfigurations.SecurityGroups`, OR `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.SecurityGroupIds` / `NetworkInterfaces[].Groups`
- **Agent 3**: `autoscaling:DescribeAutoScalingGroups` → LaunchConfig/LaunchTemplate; then `autoscaling:DescribeLaunchConfigurations` → `SecurityGroups`; or `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.SecurityGroupIds` / `NetworkInterfaces[].Groups`
- **Agent 4**: `autoscaling:DescribeAutoScalingGroups` → resolve launch template/config as above; `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.SecurityGroupIds` and/or `NetworkInterfaces[].Groups`; or `DescribeLaunchConfigurations` → `SecurityGroups`.
- **Agent 5**: `autoscaling:DescribeAutoScalingGroups` → `LaunchConfigurationName` or `LaunchTemplate`. `autoscaling:DescribeLaunchConfigurations` → `SecurityGroups`; or `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.SecurityGroupIds` / `NetworkInterfaces[].Groups`.

### `asg` → `vpc`

- **Agent 1**: `autoscaling:DescribeAutoScalingGroups` → `VPCZoneIdentifier` (comma-separated subnet IDs) → `ec2:DescribeSubnets(SubnetIds)` → `VpcId`.
- **Agent 2**: `autoscaling:DescribeAutoScalingGroups` → `VPCZoneIdentifier` (subnet IDs); `ec2:DescribeSubnets` with those IDs → `VpcId`
- **Agent 3**: `autoscaling:DescribeAutoScalingGroups` → `VPCZoneIdentifier` (subnet-id list); then `ec2:DescribeSubnets` with those ids → `VpcId`
- **Agent 4**: `autoscaling:DescribeAutoScalingGroups` → `VPCZoneIdentifier` (comma-separated subnet IDs); `ec2:DescribeSubnets` with those subnet IDs → `VpcId`.
- **Agent 5**: `autoscaling:DescribeAutoScalingGroups` → `VPCZoneIdentifier` contains comma-separated subnet IDs. `ec2:DescribeSubnets` with those subnet IDs → `VpcId`.

### `cb` → `pipeline`

- **Agent 1**: `codepipeline:ListPipelines` → for each `codepipeline:GetPipeline` → iterate `Stages[].Actions[]` where `ActionTypeId.Provider == "CodeBuild"` and `Configuration.ProjectName == <cb project>`. (Reverse search — no direct CodeBuild→pipeline API.)
- **Agent 2**: `codepipeline:ListPipelines` → for each, `codepipeline:GetPipeline` → inspect `Stages[].Actions[]` where `ActionTypeId.Provider=CodeBuild` and `Configuration.ProjectName=<cb-name>`
- **Agent 3**: `codepipeline:ListPipelines` then for each `codepipeline:GetPipeline` → iterate `stages[].actions[]` where `actionTypeId.category=Build` and `configuration.ProjectName` matches the CodeBuild project name
- **Agent 4**: `codepipeline:ListPipelines` → for each pipeline `codepipeline:GetPipeline` → scan `Stages[].Actions[]` where `ActionTypeId.Provider == "CodeBuild"` and `Configuration.ProjectName` == the CodeBuild project name. (Reverse lookup from a project requires iterating pipelines; no API returns pipelines by project.)
- **Agent 5**: `codebuild:BatchGetProjects` → project ARN. Then `codepipeline:ListPipelines` → for each pipeline `codepipeline:GetPipeline` → scan `Stages[].Actions[]` for `ActionTypeId.Provider == "CodeBuild"` and `Configuration.ProjectName` matching.

### `codeartifact` → `kms`

- **Agent 1**: `codeartifact:DescribeDomain(domain)` → `EncryptionKey` (KMS key ARN).
- **Agent 2**: `codeartifact:DescribeDomain` with `domain=<name>` → `domain.encryptionKey` (KMS key ARN)
- **Agent 3**: `codeartifact:DescribeDomain` → `encryptionKey` (KMS key ARN). Repositories inherit the domain's key
- **Agent 4**: `codeartifact:DescribeDomain` with domain name → `domain.encryptionKey` (KMS key ARN). Repositories inherit the domain's key.
- **Agent 5**: `codeartifact:DescribeDomain` with domain name → `encryptionKey` field returns the KMS key ARN.

### `ddb` → `backup`

- **Agent 1**: `dynamodb:DescribeContinuousBackups(TableName)` for PITR; for on-demand/AWS-Backup: `backup:ListRecoveryPointsByResource(ResourceArn=<tableArn>)` → `RecoveryPoints[].BackupVaultName`/`RecoveryPointArn`.
- **Agent 2**: `dynamodb:DescribeContinuousBackups` → point-in-time recovery status; `backup:ListProtectedResources` filter by `ResourceArn=<table-arn>` → backup plans/recovery points; `backup:ListRecoveryPointsByResource` with table ARN
- **Agent 3**: `backup:ListProtectedResources` filter `ResourceType=DynamoDB`; or `backup:ListRecoveryPointsByResource` with the DDB table ARN; also `dynamodb:DescribeContinuousBackups` tells PITR status (not AWS Backup plans)
- **Agent 4**: `dynamodb:DescribeContinuousBackups` with table name → `PointInTimeRecoveryDescription`. For on-demand/AWS Backup: `backup:ListRecoveryPointsByResource` with the table ARN → recovery points; or `backup:ListProtectedResources` filtered to `ResourceType == DynamoDB`.
- **Agent 5**: `dynamodb:DescribeContinuousBackups` → PITR status. For AWS Backup integration: `backup:ListProtectedResources` and filter by `ResourceArn` matching the DynamoDB table ARN; or tag-based: AWS Backup selections reference tables by ARN or tag via `backup:ListBackupSelections`.

### `eb` → `elb`

- **Agent 1**: `elasticbeanstalk:DescribeEnvironmentResources(EnvironmentName)` → `EnvironmentResources.LoadBalancers[].Name` (classic) or `elasticbeanstalk:DescribeEnvironments` → env resources then `elbv2:DescribeLoadBalancers` by tag `elasticbeanstalk:environment-name`.
- **Agent 2**: `elasticbeanstalk:DescribeEnvironmentResources` with `EnvironmentName` → `EnvironmentResources.LoadBalancers[].Name` (classic) or `elasticbeanstalk:DescribeConfigurationSettings` → namespace `aws:elbv2:loadbalancer` for ALB/NLB ARNs
- **Agent 3**: `elasticbeanstalk:DescribeEnvironmentResources` with `EnvironmentName` → `EnvironmentResources.LoadBalancers[].Name` (classic ELB) and also `Instances`, `AutoScalingGroups`
- **Agent 4**: `elasticbeanstalk:DescribeEnvironmentResources` with `EnvironmentName` → `EnvironmentResources.LoadBalancers[].Name` (classic ELB by default). For ALB: `DescribeEnvironments`/`DescribeConfigurationSettings` option `aws:elasticbeanstalk:environment:LoadBalancerType`, then `elbv2:DescribeLoadBalancers` filtering by tag `elasticbeanstalk:environment-name`.
- **Agent 5**: `elasticbeanstalk:DescribeEnvironmentResources` with env name → `EnvironmentResources.LoadBalancers[].Name` (classic ELB). For ALB/NLB: inspect the same response's `LoadBalancers` or the associated CloudFormation stack resources.

### `eb` → `role`

- **Agent 1**: `elasticbeanstalk:DescribeConfigurationSettings(ApplicationName, EnvironmentName)` → `OptionSettings` where `Namespace=aws:autoscaling:launchconfiguration` and `OptionName=IamInstanceProfile`; also `aws:elasticbeanstalk:environment` / `ServiceRole`. Then `iam:GetInstanceProfile` to resolve roles.
- **Agent 2**: `elasticbeanstalk:DescribeConfigurationSettings` → namespace `aws:autoscaling:launchconfiguration` option `IamInstanceProfile`; also `aws:elasticbeanstalk:environment` option `ServiceRole`
- **Agent 3**: `elasticbeanstalk:DescribeConfigurationSettings` → option settings `aws:autoscaling:launchconfiguration:IamInstanceProfile` (instance profile) and `aws:elasticbeanstalk:environment:ServiceRole`
- **Agent 4**: `elasticbeanstalk:DescribeConfigurationSettings` → `OptionSettings` with namespace `aws:autoscaling:launchconfiguration` key `IamInstanceProfile` and namespace `aws:elasticbeanstalk:environment` key `ServiceRole`. Resolve instance profile via `iam:GetInstanceProfile`.
- **Agent 5**: `elasticbeanstalk:DescribeEnvironments` → `EnvironmentId`. `elasticbeanstalk:DescribeConfigurationSettings` → OptionSettings for `aws:autoscaling:launchconfiguration:IamInstanceProfile` and `aws:elasticbeanstalk:environment:ServiceRole`.

### `eb` → `s3`

- **Agent 1**: `elasticbeanstalk:DescribeApplicationVersions(ApplicationName)` → `SourceBundle.S3Bucket`/`S3Key`. Also EB stores logs in `elasticbeanstalk-<region>-<account>` bucket (derivable) → `s3:GetBucketLocation` optional.
- **Agent 2**: `elasticbeanstalk:DescribeApplicationVersions` → `SourceBundle.S3Bucket`/`S3Key`; OR `elasticbeanstalk:CreateStorageLocation` (returns bucket); S3 bucket naming convention `elasticbeanstalk-<region>-<account-id>`
- **Agent 3**: Application versions live in S3: `elasticbeanstalk:DescribeApplicationVersions` → `SourceBundle.S3Bucket`; also the EB-managed bucket `elasticbeanstalk-<region>-<account>` is used for logs/configs
- **Agent 4**: Application versions live in an EB-owned bucket: `elasticbeanstalk:CreateStorageLocation` returns the bucket name, or `elasticbeanstalk:DescribeApplicationVersions` → `SourceBundle.S3Bucket`. ELB access logs and EB logs may go to a customer bucket, discoverable via `DescribeConfigurationSettings` option `aws:elasticbeanstalk:hostmanager:LogPublicationControl`.
- **Agent 5**: `elasticbeanstalk:CreateStorageLocation` returns the EB bucket name (`elasticbeanstalk-<region>-<account>`); application versions live in that bucket. Also `elasticbeanstalk:DescribeApplicationVersions` → `SourceBundle.S3Bucket`.

### `eb` → `sg`

- **Agent 1**: `elasticbeanstalk:DescribeEnvironmentResources` → `Instances[].Id` then `ec2:DescribeInstances` → `SecurityGroups[]`; or `DescribeConfigurationSettings` → `OptionSettings` `aws:autoscaling:launchconfiguration/SecurityGroups`.
- **Agent 2**: `elasticbeanstalk:DescribeEnvironmentResources` → `EnvironmentResources.Instances[].Id`; then `ec2:DescribeInstances` → `SecurityGroups`; OR `elasticbeanstalk:DescribeConfigurationSettings` → namespace `aws:autoscaling:launchconfiguration` option `SecurityGroups`
- **Agent 3**: `elasticbeanstalk:DescribeConfigurationSettings` → option `aws:autoscaling:launchconfiguration:SecurityGroups` and `aws:elbv2:loadbalancer:SecurityGroups`; or `elasticbeanstalk:DescribeEnvironmentResources` → `Instances` → `ec2:DescribeInstances` → `SecurityGroups`
- **Agent 4**: `elasticbeanstalk:DescribeEnvironmentResources` → `Instances[].Id`; `ec2:DescribeInstances` → `SecurityGroups[]`. Also `DescribeConfigurationSettings` options `aws:autoscaling:launchconfiguration:SecurityGroups` and `aws:elbv2:loadbalancer:SecurityGroups`.
- **Agent 5**: `elasticbeanstalk:DescribeEnvironmentResources` → inspect `Instances`; or `DescribeConfigurationSettings` OptionSettings for `aws:autoscaling:launchconfiguration:SecurityGroups` and `aws:elbv2:loadbalancer:SecurityGroups`.

### `eks` → `ami`

- **Agent 1**: `eks:ListNodegroups(clusterName)` → `eks:DescribeNodegroup` → `ReleaseVersion` or `LaunchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`. For managed nodegroups without LT: derive AMI from `AmiType`+`ReleaseVersion` via EKS-optimized SSM parameter `/aws/service/eks/optimized-ami/...`.
- **Agent 2**: `eks:ListNodegroups` with `clusterName`; `eks:DescribeNodegroup` → `ReleaseVersion`/`AmiType` (AMI ID derivable via SSM parameter `/aws/service/eks/optimized-ami/...`) or `LaunchTemplate` → `ec2:DescribeLaunchTemplateVersions.LaunchTemplateData.ImageId`; for self-managed nodes: ASG path
- **Agent 3**: `eks:ListNodegroups` → `eks:DescribeNodegroup` → `releaseVersion`+`amiType` (managed) or `launchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `ImageId`; for self-managed: scan ASGs in the cluster's VPC
- **Agent 4**: `eks:ListNodegroups` → `DescribeNodegroup` → `ReleaseVersion`/`AmiType`; if a launch template is used, read `LaunchTemplate.Id/Version` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`. Self-managed node groups: ASG/launch template chain (see `asg`→`ami`).
- **Agent 5**: `eks:DescribeCluster` then `eks:ListNodegroups` → for each `eks:DescribeNodegroup` → `ReleaseVersion`/`AmiType` (managed node group). For self-managed, inspect the launch template → AMI ID.

### `eks` → `ec2`

- **Agent 1**: `ec2:DescribeInstances(Filters=[{Name:"tag:aws:eks:cluster-name",Values:[<cluster>]}])`. Also `eks:ListNodegroups` → `DescribeNodegroup` → `Resources.AutoScalingGroups[]` → `autoscaling:DescribeAutoScalingGroups` → `Instances[]`.
- **Agent 2**: `eks:ListNodegroups` → `eks:DescribeNodegroup` → `Resources.AutoScalingGroups[].Name`; then `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`; OR `ec2:DescribeInstances` filter tag `aws:eks:cluster-name=<cluster>` or `kubernetes.io/cluster/<cluster>=owned`
- **Agent 3**: `eks:ListNodegroups` → `eks:DescribeNodegroup` → `resources.autoScalingGroups[].name`; `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`; then `ec2:DescribeInstances`
- **Agent 4**: `eks:ListNodegroups` → `DescribeNodegroup` → `Resources.AutoScalingGroups[].Name`; `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`; or directly `ec2:DescribeInstances` filter `tag:eks:cluster-name=<name>`.
- **Agent 5**: `eks:DescribeCluster` → VPC/subnets. For node instances: `eks:ListNodegroups` → `eks:DescribeNodegroup` → `Resources.AutoScalingGroups[].Name` → `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`.

### `ng` → `ami`

- **Agent 1**: `eks:DescribeNodegroup(clusterName,nodegroupName)` → `LaunchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`. Managed NGs without LT: derive AMI from `AmiType`+`ReleaseVersion` via `/aws/service/eks/optimized-ami/...` SSM parameter.
- **Agent 2**: `eks:DescribeNodegroup` with `clusterName`,`nodegroupName` → `AmiType`+`ReleaseVersion` (resolve via SSM param `/aws/service/eks/optimized-ami/<k8s>/<ami-type>/recommended/image_id`); OR `LaunchTemplate.Id`/`Version` → `ec2:DescribeLaunchTemplateVersions.LaunchTemplateData.ImageId`
- **Agent 3**: `eks:DescribeNodegroup` → if `launchTemplate` set: `ec2:DescribeLaunchTemplateVersions` → `ImageId`; else `amiType`+`releaseVersion` determines the EKS-optimized AMI (resolvable via `ssm:GetParameter` for `/aws/service/eks/optimized-ami/...`)
- **Agent 4**: `eks:DescribeNodegroup` → `AmiType` and `ReleaseVersion`; if `LaunchTemplate` is set, `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`. Managed node groups without a custom LT use the EKS-optimized AMI (resolvable via SSM parameter `/aws/service/eks/optimized-ami/...`).
- **Agent 5**: `eks:DescribeNodegroup` with cluster/nodegroup → `ReleaseVersion` + `AmiType` (managed); for custom, `LaunchTemplate.Id`/`Version` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.ImageId`.

### `ng` → `subnet`

- **Agent 1**: `eks:DescribeNodegroup` → `Subnets[]`.
- **Agent 2**: `eks:DescribeNodegroup` → `Subnets[]`
- **Agent 3**: `eks:DescribeNodegroup` → `subnets[]` (subnet IDs directly)
- **Agent 4**: `eks:DescribeNodegroup` → `Subnets[]`.
- **Agent 5**: `eks:DescribeNodegroup` → `Subnets[]` field lists subnet IDs.

### `tgw` → `subnet`

- **Agent 1**: `ec2:DescribeTransitGatewayAttachments(Filters=[{Name:"transit-gateway-id",Values:[<tgwId>]}])` → for VPC attachments: `ec2:DescribeTransitGatewayVpcAttachments` → `SubnetIds[]`.
- **Agent 2**: `ec2:DescribeTransitGatewayVpcAttachments` with `Filters=[transit-gateway-id]` → `SubnetIds[]`; also `ec2:DescribeTransitGatewayAttachments`
- **Agent 3**: `ec2:DescribeTransitGatewayVpcAttachments` filter by `transit-gateway-id` → `SubnetIds[]`
- **Agent 4**: `ec2:DescribeTransitGatewayAttachments` with filter `transit-gateway-id` and `resource-type=vpc` → attachment IDs; `ec2:DescribeTransitGatewayVpcAttachments` → `SubnetIds[]`.
- **Agent 5**: `ec2:DescribeTransitGatewayAttachments` with filter `transit-gateway-id=<id>` and `resource-type=vpc` → `TransitGatewayVpcAttachmentId`. Then `ec2:DescribeTransitGatewayVpcAttachments` → `SubnetIds[]`.

### `tgw` → `vpc`

- **Agent 1**: `ec2:DescribeTransitGatewayVpcAttachments(Filters=[{Name:"transit-gateway-id",Values:[<tgwId>]}])` → `VpcId` for each attachment.
- **Agent 2**: `ec2:DescribeTransitGatewayVpcAttachments` with `Filters=[transit-gateway-id]` → `VpcId`
- **Agent 3**: `ec2:DescribeTransitGatewayVpcAttachments` filter by `transit-gateway-id` → `VpcId` per attachment
- **Agent 4**: `ec2:DescribeTransitGatewayVpcAttachments` with filter `transit-gateway-id` → `VpcId` for each attachment.
- **Agent 5**: `ec2:DescribeTransitGatewayAttachments` with filter `transit-gateway-id=<id>` and `resource-type=vpc` → `ResourceId` is the VPC ID.

## Disagreement (needs human review)

### `apigw` → `kms` — votes: a1=no, a2=sometimes, a3=sometimes, a4=sometimes, a5=no

- **Agent 1** (no): (API Gateway REST/HTTP APIs do not accept customer KMS keys for any at-rest feature; cache encryption uses AWS-owned keys)
- **Agent 2** (sometimes): `apigateway:GetRestApi` + `apigateway:GetStages` → `CacheClusterEnabled` (AWS-owned key only); for integrations: `apigateway:GetResources` → `apigateway:GetIntegration` → follow integration URI to backend, then check that backend's KMS usage. No direct API field returns a KMS ARN
- **Agent 3** (sometimes): `apigateway:GetStage` / `apigatewayv2:GetStages` → inspect `CacheClusterEnabled` + cache encryption; `apigateway:GetRestApis` → no direct KMS field. Generally no direct KMS attachment — unsure whether the checker meant CloudWatch log group KMS (`logs:DescribeLogGroups` → `KmsKeyId`)
- **Agent 4** (sometimes): For REST: `apigateway:GetStages` → each stage's `MethodSettings` does not expose a KMS key (AWS-managed). For HTTP/WebSocket (v2): `apigatewayv2:GetStages` likewise has no KMS field. Indirect: `apigateway:GetRestApi`/`GetIntegration` → if integration URI is a Lambda → `lambda:GetFunctionConfiguration` → `KMSKeyArn`. Otherwise no direct linkage.
- **Agent 5** (no): — (API Gateway REST/HTTP APIs do not support customer-managed KMS keys for at-rest encryption of cached data or config; cache encryption uses AWS-managed keys only)

### `asg` → `elb` — votes: a1=yes, a2=yes, a3=yes, a4=yes, a5=sometimes

- **Agent 1** (yes): `autoscaling:DescribeAutoScalingGroups` → `LoadBalancerNames` (classic ELB names). For ALB/NLB use `LoadBalancerTargetGroups` → `elbv2:DescribeTargetGroups(TargetGroupArns)` → `LoadBalancerArns`.
- **Agent 2** (yes): `autoscaling:DescribeAutoScalingGroups` → `LoadBalancerNames` (classic ELB) and `LoadBalancerTargetGroups` would need mapping via `elbv2:DescribeTargetGroups` → `LoadBalancerArns`; also `autoscaling:DescribeLoadBalancers` with `AutoScalingGroupName`
- **Agent 3** (yes): `autoscaling:DescribeAutoScalingGroups` → `LoadBalancerNames` (classic ELB) and `TargetGroupARNs` (ALB/NLB). For classic: `elb:DescribeLoadBalancers` by name. For ALB/NLB the link is via target groups, not `elb` directly
- **Agent 4** (yes): `autoscaling:DescribeAutoScalingGroups` → `LoadBalancerNames` (classic ELB). For ALB/NLB use `autoscaling:DescribeLoadBalancerTargetGroups` → `TargetGroupARNs`, then `elbv2:DescribeTargetGroups` → `LoadBalancerArns`, then `elbv2:DescribeLoadBalancers`.
- **Agent 5** (sometimes): `autoscaling:DescribeAutoScalingGroups` → `LoadBalancerNames` field lists classic ELB names. Then `elasticloadbalancing:DescribeLoadBalancers` with those names.

### `asg` → `role` — votes: a1=sometimes, a2=yes, a3=yes, a4=yes, a5=yes

- **Agent 1** (sometimes): `autoscaling:DescribeAutoScalingGroups` → LaunchConfig/LaunchTemplate → `IamInstanceProfile` → `iam:GetInstanceProfile` → `Roles[].RoleName`. Also the ASG may use a service-linked role `AWSServiceRoleForAutoScaling`.
- **Agent 2** (yes): `autoscaling:DescribeAutoScalingGroups` → `LaunchConfigurationName` or `LaunchTemplate`; then `autoscaling:DescribeLaunchConfigurations` → `IamInstanceProfile`, OR `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.IamInstanceProfile`; then `iam:GetInstanceProfile` → `Roles[].RoleName`; also `autoscaling:DescribeAutoScalingGroups.ServiceLinkedRoleARN`
- **Agent 3** (yes): `autoscaling:DescribeAutoScalingGroups` → `LaunchConfigurationName` or `LaunchTemplate`; then `autoscaling:DescribeLaunchConfigurations` → `IamInstanceProfile`; or `ec2:DescribeLaunchTemplateVersions` → `IamInstanceProfile.Arn`/`Name`; then `iam:GetInstanceProfile` → `Roles[].Arn`
- **Agent 4** (yes): `autoscaling:DescribeAutoScalingGroups` → get `LaunchTemplate` or `LaunchConfigurationName`. Launch template: `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.IamInstanceProfile.{Arn,Name}`, then `iam:GetInstanceProfile` → `Roles[].Arn`. Launch config: `autoscaling:DescribeLaunchConfigurations` → `IamInstanceProfile`, same `iam:GetInstanceProfile` chain. Also check `ServiceLinkedRoleARN` on the ASG itself.
- **Agent 5** (yes): `autoscaling:DescribeAutoScalingGroups` → `ServiceLinkedRoleARN` gives service-linked role. Also launch config/template → `IamInstanceProfile` → `iam:GetInstanceProfile` → `Roles[].Arn`.

### `asg` → `sns` — votes: a1=yes, a2=yes, a3=yes, a4=yes, a5=sometimes

- **Agent 1** (yes): `autoscaling:DescribeNotificationConfigurations(AutoScalingGroupNames=[name])` → `TopicARN` list.
- **Agent 2** (yes): `autoscaling:DescribeNotificationConfigurations` with `AutoScalingGroupNames=[name]` → `TopicARN`
- **Agent 3** (yes): `autoscaling:DescribeNotificationConfigurations` filtered by `AutoScalingGroupNames` → `TopicARN`
- **Agent 4** (yes): `autoscaling:DescribeNotificationConfigurations` with the ASG name → `TopicARN` list.
- **Agent 5** (sometimes): `autoscaling:DescribeNotificationConfigurations` with ASG name → `TopicARN` for each event. Also `autoscaling:DescribeLifecycleHooks` → `NotificationTargetARN` may be SNS.

### `ddb` → `kinesis` — votes: a1=yes, a2=yes, a3=yes, a4=yes, a5=sometimes

- **Agent 1** (yes): `dynamodb:DescribeKinesisStreamingDestination(TableName)` → `KinesisDataStreamDestinations[].StreamArn`.
- **Agent 2** (yes): `dynamodb:DescribeKinesisStreamingDestination` with `TableName` → `KinesisDataStreamDestinations[].StreamArn`
- **Agent 3** (yes): `dynamodb:DescribeKinesisStreamingDestination` with table name → `KinesisDataStreamDestinations[].StreamArn`
- **Agent 4** (yes): `dynamodb:DescribeKinesisStreamingDestination` with table name → `KinesisDataStreamDestinations[].StreamArn`.
- **Agent 5** (sometimes): `dynamodb:DescribeKinesisStreamingDestination` with table name → `KinesisDataStreamDestinations[].StreamArn`.

### `ddb` → `sns` — votes: a1=sometimes, a2=no, a3=no, a4=no, a5=no

- **Agent 1** (sometimes): Indirect. `cloudwatch:DescribeAlarmsForMetric(Namespace=AWS/DynamoDB, Dimensions=TableName)` → `AlarmActions` with SNS topic ARNs. No direct DDB→SNS API.
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (DynamoDB tables don't directly integrate with SNS; event notifications require Streams + Lambda or EventBridge Pipes)

### `docdb-snap` → `backup` — votes: a1=sometimes, a2=sometimes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (sometimes): `backup:ListRecoveryPointsByResource(ResourceArn=<clusterArn>)` — AWS Backup tracks DocDB cluster resources; snapshot→backup mapping is via the parent cluster. No direct snapshot-ARN-to-backup-vault API.
- **Agent 2** (sometimes): `backup:ListRecoveryPointsByResource` with `ResourceArn=<docdb-cluster-arn>`; OR `backup:ListProtectedResources` filter by resource type `Aurora`/`DocumentDB`
- **Agent 3** (yes): `backup:ListProtectedResources` filter by DocumentDB cluster ARN; or `backup:ListRecoveryPointsByResource` with the cluster/snapshot ARN — AWS Backup manages DocDB snapshots when a plan targets DocDB
- **Agent 4** (sometimes): `backup:ListRecoveryPointsByResource` with the DocumentDB cluster ARN (or snapshot ARN where supported) → recovery points; or inspect the snapshot's ARN tag `aws:backup:source-resource`. DocDB-native snapshots (`rds:DescribeDBClusterSnapshots` with `--engine docdb`) are not Backup recovery points.
- **Agent 5** (sometimes): `backup:ListRecoveryPointsByResource` with the DocumentDB cluster ARN; or `backup:DescribeRecoveryPoint` if the snapshot ARN is a recovery-point ARN. Alternatively match tags `aws:backup:source-resource`.

### `eb` → `tg` — votes: a1=yes, a2=sometimes, a3=yes, a4=yes, a5=sometimes

- **Agent 1** (yes): `elasticbeanstalk:DescribeEnvironmentResources` → `LoadBalancers[]` → `elbv2:DescribeListeners(LoadBalancerArn)` → `DefaultActions[].TargetGroupArn`.
- **Agent 2** (sometimes): `elasticbeanstalk:DescribeEnvironmentResources` → `EnvironmentResources.LoadBalancers[].Name`; then `elbv2:DescribeListeners` → `elbv2:DescribeRules` → target groups; OR `elbv2:DescribeTargetGroups` and filter by tag `elasticbeanstalk:environment-name`
- **Agent 3** (yes): `elasticbeanstalk:DescribeEnvironmentResources` → `LoadBalancers` (for ALB environments); then `elbv2:DescribeTargetGroups` with `LoadBalancerArn` of the EB load balancer
- **Agent 4** (yes): `elasticbeanstalk:DescribeEnvironmentResources` → `LoadBalancers[].Name` (ARN for ALB); then `elbv2:DescribeListeners` → `DefaultActions[].TargetGroupArn`, or `elbv2:DescribeTargetGroups --load-balancer-arn`.
- **Agent 5** (sometimes): `elasticbeanstalk:DescribeEnvironmentResources` → get LoadBalancer ARN; `elasticloadbalancingv2:DescribeListeners` → listener ARNs; `elasticloadbalancingv2:DescribeRules` / `DescribeTargetGroups` filtering by LB ARN.

### `ec2` → `ssm` — votes: a1=yes, a2=yes, a3=sometimes, a4=yes, a5=sometimes

- **Agent 1** (yes): `ssm:DescribeInstanceInformation(Filters=[{Key:"InstanceIds",Values:[<id>]}])` → returns SSM-managed instance info (PingStatus, PlatformName). Also `ssm:ListAssociations(AssociationFilterList=[{Key:"InstanceId",...}])` for associations.
- **Agent 2** (yes): `ssm:DescribeInstanceInformation` with `Filters=[{Key:InstanceIds,Values:[<id>]}]`; also `ssm:ListAssociations` filtered by instance; `ssm:GetParameters` referenced in user-data requires scanning instance user-data
- **Agent 3** (sometimes): `ssm:DescribeInstanceInformation` with `InstanceInformationFilterList=[{key=InstanceIds,valueSet=[i-...]}]` → presence means it's an SSM managed instance; also `ssm:ListAssociations` per instance → associations
- **Agent 4** (yes): `ssm:DescribeInstanceInformation` filter by `InstanceIds` → `PingStatus`, `AssociationStatus`; `ssm:ListAssociations` / `ssm:DescribeAssociationExecutionTargets` filter by instance; `ssm:GetInventory` / `ssm:DescribeInstancePatchStates` with the instance ID.
- **Agent 5** (sometimes): `ssm:DescribeInstanceInformation` with filter `InstanceIds=<id>`. Presence of a matching entry means the instance is SSM-managed. Also `ssm:ListAssociations` filtering on the instance ID.

### `ecr` → `eb-rule` — votes: a1=yes, a2=yes, a3=sometimes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `events:ListRules` → for each `events:DescribeRule` with `EventPattern` source `aws.ecr` and `detail.repository-name == <repoName>`; or `events:ListRuleNamesByTarget` if a Lambda/SNS target has the ECR repo in its environment. Iterative.
- **Agent 2** (yes): `events:ListRules` → for each `events:ListTargetsByRule` then inspect `EventPattern` for `source: aws.ecr` and `resources` matching repository ARN
- **Agent 3** (sometimes): `events:ListRuleNamesByTarget` won't work (rules don't target ECR); instead `events:ListRules` → for each `events:DescribeRule` → inspect `EventPattern` for `"source":["aws.ecr"]` and `detail.repository-name` matching the repo
- **Agent 4** (sometimes): Iterate `events:ListRules` → `events:DescribeRule` and parse `EventPattern` for `source:["aws.ecr"]` and `resources` containing the repository ARN. No reverse API exists.
- **Agent 5** (sometimes): List-all approach: `events:ListRules` → `events:DescribeRule` → inspect `EventPattern` for `source: aws.ecr` and `resources:` matching the repository ARN.

### `ecr` → `ecs` — votes: a1=yes, a2=yes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): Iterate `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Image` whose registry host matches `<account>.dkr.ecr.<region>.amazonaws.com/<repoName>`. Expensive reverse scan.
- **Agent 2** (yes): `ecs:ListTaskDefinitions` → for each `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Image`; filter where image URI starts with `<account>.dkr.ecr.<region>.amazonaws.com/<repo>`
- **Agent 3** (yes): `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `containerDefinitions[].image` — match against the ECR repository URI (`<acct>.dkr.ecr.<region>.amazonaws.com/<repo>`); running services: `ecs:ListClusters` → `ecs:ListServices` → `ecs:DescribeServices` → `taskDefinition` → same
- **Agent 4** (sometimes): Iterate `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → scan `ContainerDefinitions[].Image` for the ECR repository URI, then `ecs:ListServices`/`DescribeServices` to find services using that task def.
- **Agent 5** (sometimes): List-all approach: `ecs:ListTaskDefinitions` → for each `ecs:DescribeTaskDefinition` → scan `ContainerDefinitions[].Image` for the repo URI. Then `ecs:ListServices`/`ListClusters` to map to services.

### `ecr` → `pipeline` — votes: a1=yes, a2=sometimes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `codepipeline:ListPipelines` → `codepipeline:GetPipeline` → iterate actions; source actions with `ActionTypeId.Provider == "ECR"` and `Configuration.RepositoryName == <repo>`. Reverse search.
- **Agent 2** (sometimes): `codepipeline:ListPipelines` → `codepipeline:GetPipeline` → `Stages[].Actions[]` where `ActionTypeId.Provider=ECR` and `Configuration.RepositoryName=<repo>`
- **Agent 3** (yes): `codepipeline:ListPipelines` → `codepipeline:GetPipeline` → iterate `stages[].actions[]`; ECR source actions: `actionTypeId.provider=ECR` with `configuration.RepositoryName`; also scan Build actions' env vars for ECR URIs
- **Agent 4** (sometimes): Iterate `codepipeline:ListPipelines` → `GetPipeline` → scan stages for actions with `ActionTypeId.Provider == "ECR"` and `Configuration.RepositoryName` matching the repo; also inspect CodeBuild actions' projects for ECR usage.
- **Agent 5** (sometimes): List-all approach: `codepipeline:ListPipelines` → `codepipeline:GetPipeline` → scan `Stages[].Actions[]` for `ActionTypeId.Provider == "ECR"` and `Configuration.RepositoryName`.

### `ecr` → `role` — votes: a1=sometimes, a2=sometimes, a3=yes, a4=sometimes, a5=yes

- **Agent 1** (sometimes): `ecr:GetRepositoryPolicy(repositoryName)` → parse `Statement[].Principal.AWS` for IAM role ARNs.
- **Agent 2** (sometimes): `ecr:GetRepositoryPolicy` with `repositoryName` → parse JSON `Statement[].Principal.AWS` for role ARNs
- **Agent 3** (yes): `ecr:GetRepositoryPolicy` on the repo → parse policy `Statement[].Principal.AWS` for role ARNs
- **Agent 4** (sometimes): `ecr:GetRepositoryPolicy` → parse `policyText` `Statement[].Principal.AWS` for role ARNs. Also `ecr:GetRegistryPolicy` at account level.
- **Agent 5** (yes): `ecr:GetRepositoryPolicy` with repository name → parse policy JSON `Statement[].Principal.AWS` for role ARNs. Registry policy via `ecr:GetRegistryPolicy` may also reference roles.

### `ecs-svc` → `eb-rule` — votes: a1=yes, a2=sometimes, a3=sometimes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `events:ListRuleNamesByTarget(TargetArn=<ecsServiceArn>)` returns rules whose target is this ECS service (ECS task via RunTask target uses `EcsParameters.TaskDefinitionArn`). Also iterate `events:ListRules` and match targets.
- **Agent 2** (sometimes): `events:ListRuleNamesByTarget` with `TargetArn=<ecs-cluster-or-service-arn>`; OR `events:ListRules` + `events:ListTargetsByRule` where Target `EcsParameters.TaskDefinitionArn` refers to the service's task family
- **Agent 3** (sometimes): `events:ListRules` → for each `events:ListTargetsByRule` → target with `EcsParameters.TaskDefinitionArn` matching the service's task def, or `Arn` being an ECS cluster ARN with `EcsParameters`
- **Agent 4** (sometimes): `events:ListRuleNamesByTarget` with the cluster ARN as the target, or iterate `events:ListRules` → `ListTargetsByRule` and match `EcsParameters.TaskDefinitionArn` and `Arn` (cluster) against the service's cluster.
- **Agent 5** (sometimes): `events:ListRuleNamesByTarget` with the service ARN or task-def ARN; or `events:ListRules` → `events:ListTargetsByRule` → filter targets with `EcsParameters.TaskDefinitionArn` matching.

### `ecs-svc` → `ecr` — votes: a1=yes, a2=yes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `ecs:DescribeServices` → `TaskDefinition` → `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Image`; parse `<account>.dkr.ecr.<region>.amazonaws.com/<repo>` to derive ECR repo names.
- **Agent 2** (yes): `ecs:DescribeServices` → `TaskDefinition`; then `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Image`; filter for ECR URIs
- **Agent 3** (yes): `ecs:DescribeServices` → `taskDefinition` → `ecs:DescribeTaskDefinition` → `containerDefinitions[].image` — parse ECR URIs
- **Agent 4** (sometimes): `ecs:DescribeServices` → `TaskDefinition` → `ecs:DescribeTaskDefinition` → scan `ContainerDefinitions[].Image` for ECR URIs; extract repo name, then `ecr:DescribeRepositories`.
- **Agent 5** (sometimes): `ecs:DescribeServices` → `TaskDefinition` ARN. `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Image`. Parse image URI for `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>`.

### `ecs-svc` → `secrets` — votes: a1=yes, a2=yes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `ecs:DescribeServices` → `TaskDefinition` → `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` (Secrets Manager ARN) and `ContainerDefinitions[].Environment` is not secrets — only `Secrets` array or `secretOptions`.
- **Agent 2** (yes): `ecs:DescribeServices.TaskDefinition` → `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` (Secrets Manager ARN) and `RepositoryCredentials.CredentialsParameter`
- **Agent 3** (yes): `ecs:DescribeServices` → `taskDefinition`; `ecs:DescribeTaskDefinition` → `containerDefinitions[].secrets[].valueFrom` (ARNs of secrets manager or ssm params)
- **Agent 4** (sometimes): `ecs:DescribeServices` → `TaskDefinition`; `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` (ARN of secret) or `Environment` with secretsmanager ARNs.
- **Agent 5** (sometimes): `ecs:DescribeServices` → TaskDefinition ARN. `ecs:DescribeTaskDefinition` → scan `ContainerDefinitions[].Secrets[].ValueFrom` and `ContainerDefinitions[].Environment` for `arn:aws:secretsmanager:...`.

### `ecs-svc` → `sfn` — votes: a1=sometimes, a2=sometimes, a3=sometimes, a4=sometimes, a5=no

- **Agent 1** (sometimes): Reverse scan: `stepfunctions:ListStateMachines` → `stepfunctions:DescribeStateMachine` → parse `definition` JSON for `"Resource":"arn:aws:states:::ecs:runTask"` with `TaskDefinition` matching. No direct API.
- **Agent 2** (sometimes): `stepfunctions:ListStateMachines` → `stepfunctions:DescribeStateMachine` → parse `definition` JSON for `"Resource": "arn:aws:states:::ecs:runTask"` and `Parameters.TaskDefinition` matching service's family
- **Agent 3** (sometimes): `states:ListStateMachines` → for each `states:DescribeStateMachine` → parse `definition` JSON for `Resource: "arn:aws:states:::ecs:runTask*"` and `Parameters.TaskDefinition` matching the service's task def
- **Agent 4** (sometimes): Iterate `stepfunctions:ListStateMachines` → `DescribeStateMachine` → parse `definition` JSON `Resource` fields of type `arn:aws:states:::ecs:runTask*` and match cluster/taskDefinition.
- **Agent 5** (no): — (ECS services do not directly reference Step Functions; SFN may invoke ECS tasks but not services)

### `ecs-task` → `secrets` — votes: a1=yes, a2=yes, a3=yes, a4=yes, a5=sometimes

- **Agent 1** (yes): `ecs:DescribeTaskDefinition(taskDefinition)` → `ContainerDefinitions[].Secrets[].ValueFrom` that start with `arn:aws:secretsmanager:`.
- **Agent 2** (yes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` (Secrets Manager ARNs)
- **Agent 3** (yes): `ecs:DescribeTaskDefinition` → `containerDefinitions[].secrets[].valueFrom` where ARN prefix is `arn:aws:secretsmanager:`
- **Agent 4** (yes): `ecs:DescribeTaskDefinition` → for each container, `Secrets[].ValueFrom` where ARN prefix is `arn:aws:secretsmanager:`. Also scan `Environment` for secret ARNs (non-standard).
- **Agent 5** (sometimes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` starting with `arn:aws:secretsmanager:...`.

### `ecs-task` → `ssm` — votes: a1=yes, a2=yes, a3=yes, a4=yes, a5=sometimes

- **Agent 1** (yes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` that start with `arn:aws:ssm:...:parameter/...` — SSM Parameter Store parameters.
- **Agent 2** (yes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` entries with `arn:aws:ssm:*:parameter/*`; also `ExecutionRoleArn` may have SSM perms (not a direct SSM resource link)
- **Agent 3** (yes): `ecs:DescribeTaskDefinition` → `containerDefinitions[].secrets[].valueFrom` where ARN prefix is `arn:aws:ssm:*:*:parameter/...` (SSM Parameter Store)
- **Agent 4** (yes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` where ARN prefix is `arn:aws:ssm:...:parameter/...`.
- **Agent 5** (sometimes): `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom` starting with `arn:aws:ssm:...:parameter/...`.

### `efs` → `backup` — votes: a1=yes, a2=yes, a3=yes, a4=yes, a5=sometimes

- **Agent 1** (yes): `backup:ListRecoveryPointsByResource(ResourceArn=<efsFileSystemArn>)` → `RecoveryPoints[].BackupVaultName`. Or `backup:ListProtectedResources` filtered by `ResourceType=EFS`.
- **Agent 2** (yes): `backup:ListProtectedResources` filter `ResourceType=EFS` and `ResourceArn=<fs-arn>`; `backup:ListRecoveryPointsByResource` with EFS filesystem ARN
- **Agent 3** (yes): `backup:ListProtectedResources` filter by EFS filesystem ARN; or `backup:ListRecoveryPointsByResource` with `ResourceArn=arn:aws:elasticfilesystem:<region>:<acct>:file-system/fs-...`
- **Agent 4** (yes): `backup:ListRecoveryPointsByResource` with the EFS file system ARN → recovery points and associated backup plans.
- **Agent 5** (sometimes): `backup:ListProtectedResources` filter `ResourceArn == <efs-arn>`; also `backup:ListRecoveryPointsByResource` with the EFS file system ARN.

### `efs` → `ecs-task` — votes: a1=yes, a2=yes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): Reverse: `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `Volumes[].EfsVolumeConfiguration.FileSystemId == <fsId>`. Expensive iteration.
- **Agent 2** (yes): `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `Volumes[].EfsVolumeConfiguration.FileSystemId` matching the EFS id
- **Agent 3** (yes): `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `volumes[].efsVolumeConfiguration.fileSystemId` — match to the EFS id
- **Agent 4** (sometimes): Iterate `ecs:ListTaskDefinitions` → `DescribeTaskDefinition` → scan `Volumes[].EfsVolumeConfiguration.FileSystemId` for the EFS ID.
- **Agent 5** (sometimes): List-all approach: `ecs:ListTaskDefinitions` → for each `ecs:DescribeTaskDefinition` → scan `Volumes[].EfsVolumeConfiguration.FileSystemId` matching.

### `kinesis` → `ddb` — votes: a1=sometimes, a2=sometimes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (sometimes): `dynamodb:ListTables` → `dynamodb:DescribeKinesisStreamingDestination(TableName)` → match `StreamArn == <kinesisArn>`. Expensive reverse iteration.
- **Agent 2** (sometimes): `dynamodb:ListTables` → for each `dynamodb:DescribeKinesisStreamingDestination` → match `KinesisDataStreamDestinations[].StreamArn` with this stream's ARN (expensive fanout); cheaper: `kinesis:DescribeStreamConsumer` doesn't expose DDB source
- **Agent 3** (yes): `dynamodb:ListTables` → for each `dynamodb:DescribeKinesisStreamingDestination` → `KinesisDataStreamDestinations[].StreamArn` matches the stream ARN
- **Agent 4** (sometimes): `kinesis:DescribeStreamConsumer` does not expose this. Reverse: iterate `dynamodb:ListTables` → `DescribeKinesisStreamingDestination` and match `StreamArn`.
- **Agent 5** (sometimes): List-all approach: `dynamodb:ListTables` → for each `dynamodb:DescribeKinesisStreamingDestination` → filter `StreamArn` matching.

### `kms` → `role` — votes: a1=sometimes, a2=sometimes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (sometimes): `kms:GetKeyPolicy(KeyId,PolicyName="default")` → parse `Statement[].Principal.AWS` for role ARNs; `kms:ListGrants(KeyId)` → `GranteePrincipal` / `RetiringPrincipal` for role ARNs.
- **Agent 2** (sometimes): `kms:GetKeyPolicy` with `KeyId` → parse `Statement[].Principal.AWS` for role ARNs; also `kms:ListGrants` → `GranteePrincipal` and `RetiringPrincipal`
- **Agent 3** (yes): `kms:GetKeyPolicy` → parse `Statement[].Principal.AWS` for role ARNs; also `kms:ListGrants` → `GranteePrincipal` / `RetiringPrincipal` as role ARNs
- **Agent 4** (sometimes): `kms:GetKeyPolicy` (parse `Principal.AWS` for role ARNs) and `kms:ListGrants` (→ `GranteePrincipal`, `RetiringPrincipal`).
- **Agent 5** (sometimes): `kms:GetKeyPolicy` → parse `Statement[].Principal.AWS` for role ARNs. Also `kms:ListGrants` → `GranteePrincipal` and `RetiringPrincipal` for role ARNs.

### `lambda` → `ecr` — votes: a1=yes, a2=sometimes, a3=sometimes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `lambda:GetFunction(FunctionName)` → `Code.ImageUri` (container-image Lambdas) — parse ECR repo name from URI `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>@sha256:...`.
- **Agent 2** (sometimes): `lambda:GetFunction` → `Code.ImageUri` (present when `PackageType=Image`); parse repo name from URI `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>:<tag>`
- **Agent 3** (sometimes): `lambda:GetFunction` → `Code.ImageUri` (for `PackageType=Image`) — parse ECR repo URI
- **Agent 4** (sometimes): `lambda:GetFunction` → `Code.ImageUri` (ECR image URI) when `PackageType == "Image"`. Parse to get the ECR repo name.
- **Agent 5** (sometimes): `lambda:GetFunction` → `Code.ImageUri`. Parse URI for `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>:<tag>`.

### `lambda` → `r53` — votes: a1=no, a2=no, a3=sometimes, a4=sometimes, a5=no

- **Agent 1** (no): (Lambda has no native Route 53 linkage; custom domains go via API Gateway/CloudFront)
- **Agent 2** (no):
- **Agent 3** (sometimes): No direct API. Heuristic: find the function's API Gateway/ALB integration, then `route53:ListResourceRecordSets` across zones looking for alias targeting that domain
- **Agent 4** (sometimes): No direct link. Function URL: `lambda:GetFunctionUrlConfig` gives an AWS-owned domain; to see if Route 53 points at it, iterate hosted zones (`route53:ListHostedZones` → `ListResourceRecordSets`) and match CNAMEs.
- **Agent 5** (no): — (Lambda has no direct Route 53 relationship; would only be indirect via API Gateway/CloudFront custom domains)

### `ng` → `ebs` — votes: a1=yes, a2=sometimes, a3=yes, a4=yes, a5=yes

- **Agent 1** (yes): `eks:DescribeNodegroup` → LT or `DiskSize`; instance-level EBS via `Resources.AutoScalingGroups[]` → ASG → `ec2:DescribeInstances` → `BlockDeviceMappings[].Ebs.VolumeId`.
- **Agent 2** (sometimes): `eks:DescribeNodegroup` → `Resources.AutoScalingGroups[].Name`; `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`; `ec2:DescribeVolumes` with `Filters=[attachment.instance-id]`; also `LaunchTemplate` → `BlockDeviceMappings[].Ebs`
- **Agent 3** (yes): `eks:DescribeNodegroup` → `resources.autoScalingGroups[].name`; `autoscaling:DescribeAutoScalingGroups` → `Instances[].InstanceId`; `ec2:DescribeInstances` → `BlockDeviceMappings[].Ebs.VolumeId`; or via launch template `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.BlockDeviceMappings`
- **Agent 4** (yes): `eks:DescribeNodegroup` → `DiskSize` (if no LT) implies a root EBS volume; or `LaunchTemplate` → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.BlockDeviceMappings[].Ebs`. For live volumes: `DescribeNodegroup` → `Resources.AutoScalingGroups` → instances → `ec2:DescribeInstances` → `BlockDeviceMappings[].Ebs.VolumeId`.
- **Agent 5** (yes): `eks:DescribeNodegroup` → `DiskSize` / launch template → `ec2:DescribeLaunchTemplateVersions` → `LaunchTemplateData.BlockDeviceMappings[].Ebs`. Live volumes: list ASG → instances → `ec2:DescribeInstances` → `BlockDeviceMappings[].Ebs.VolumeId`.

### `pipeline` → `eb-rule` — votes: a1=yes, a2=yes, a3=yes, a4=sometimes, a5=yes

- **Agent 1** (yes): `events:ListRuleNamesByTarget(TargetArn=<pipelineArn>)` → rules that trigger this pipeline. Also `codepipeline:GetPipeline` → `Triggers[]` (v2 pipelines) references connections, not EB rules directly.
- **Agent 2** (yes): `codepipeline:GetPipeline` → determine triggers; `events:ListRules` with prefix `codepipeline-` + `events:ListTargetsByRule` where Target `Arn` matches pipeline ARN; OR `events:ListRuleNamesByTarget` with `TargetArn=<pipeline-arn>`
- **Agent 3** (yes): `events:ListRuleNamesByTarget` with pipeline ARN → rule names (CodePipeline is commonly triggered by EventBridge rules on CodeCommit/ECR/S3 events); also `codepipeline:GetPipeline` triggers
- **Agent 4** (sometimes): `events:ListRuleNamesByTarget` with the pipeline ARN → rules; then `events:DescribeRule`. Also `codepipeline:ListWebhooks` for GitHub-style triggers.
- **Agent 5** (yes): `codepipeline:GetPipeline` typically has an EventBridge rule created for CodeCommit/S3/ECR sources. `events:ListRuleNamesByTarget` with the pipeline ARN `arn:aws:codepipeline:...:<pipeline>`.

### `rds-snap` → `backup` — votes: a1=sometimes, a2=sometimes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (sometimes): `backup:ListRecoveryPointsByResource(ResourceArn=<rdsSnapshotArn or dbInstanceArn>)`. Note: Backup tracks the DB instance/cluster, not each manual snapshot individually.
- **Agent 2** (sometimes): `backup:ListProtectedResources` filter `ResourceType=RDS`; `backup:ListRecoveryPointsByResource` with `ResourceArn=<db-cluster-or-instance-arn>`; the snapshot may appear as a recovery point
- **Agent 3** (yes): `backup:ListRecoveryPointsByResource` with the RDS DB instance/cluster ARN; or `backup:ListProtectedResources` filter `ResourceType=RDS`/`Aurora`
- **Agent 4** (sometimes): `backup:ListRecoveryPointsByResource` with the DB instance/cluster ARN, or inspect snapshot tags `aws:backup:source-resource`; also `backup:DescribeRecoveryPoint` with the backup-vault-level recovery point ARN.
- **Agent 5** (sometimes): `backup:ListRecoveryPointsByResource` with the DB cluster/instance ARN; match `CreatedBy.BackupVaultName` / `RecoveryPointArn`. Or check snapshot tags for `aws:backup:source-resource`.

### `secrets` → `codeartifact` — votes: a1=no, a2=sometimes, a3=sometimes, a4=sometimes, a5=no

- **Agent 1** (no): (Secrets Manager does not natively integrate with CodeArtifact; any coupling would be via user buildspec/code)
- **Agent 2** (sometimes): `secretsmanager:DescribeSecret` → tags/description; no first-class API link. `unsure — would need to verify AWS docs on CodeArtifact token storage convention`
- **Agent 3** (sometimes): No direct AWS API link. `secretsmanager:DescribeSecret` has no CodeArtifact field. Heuristic: name-match `Name` containing "codeartifact", or parse `GetResourcePolicy`. Unsure — would need to verify AWS docs on CodeArtifact secret integration
- **Agent 4** (sometimes): `codeartifact:DescribeDomain`/`DescribeRepository` has no secret field directly. No reliable direct API link. `resourcegroupstaggingapi:GetResources` filtered to both types may find matching tags.
- **Agent 5** (no): — (Secrets Manager and CodeArtifact have no direct relationship)

### `secrets` → `eb` — votes: a1=sometimes, a2=no, a3=sometimes, a4=sometimes, a5=sometimes

- **Agent 1** (sometimes): Reverse scan: `elasticbeanstalk:DescribeConfigurationSettings` across all envs → `OptionSettings` with values containing the secret ARN. Expensive.
- **Agent 2** (no):
- **Agent 3** (sometimes): No direct API. `elasticbeanstalk:DescribeConfigurationSettings` across envs → scan option settings for the secret ARN/name. Heuristic
- **Agent 4** (sometimes): Iterate `elasticbeanstalk:DescribeConfigurationSettings` for all environments → scan `OptionSettings[].Value` for `{{resolve:secretsmanager:<arn>}}` matching the secret.
- **Agent 5** (sometimes): List-all approach: `elasticbeanstalk:DescribeEnvironments` → `DescribeConfigurationSettings` → scan `OptionSettings` for env vars referencing the secret ARN. No native EB→secret API.

### `secrets` → `ecs-task` — votes: a1=yes, a2=yes, a3=yes, a4=yes, a5=sometimes

- **Agent 1** (yes): Reverse scan: `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `ContainerDefinitions[].Secrets[].ValueFrom == <secretArn>`.
- **Agent 2** (yes): `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → match `ContainerDefinitions[].Secrets[].ValueFrom` or `RepositoryCredentials.CredentialsParameter` against `<secret-arn>` (expensive fanout)
- **Agent 3** (yes): `ecs:ListTaskDefinitions` → `ecs:DescribeTaskDefinition` → `containerDefinitions[].secrets[].valueFrom` matches the secret ARN; also `repositoryCredentials.credentialsParameter`
- **Agent 4** (yes): Iterate `ecs:ListTaskDefinitions` → `DescribeTaskDefinition` → scan `ContainerDefinitions[].Secrets[].ValueFrom` and `RepositoryCredentials.CredentialsParameter` for the secret ARN.
- **Agent 5** (sometimes): List-all approach: `ecs:ListTaskDefinitions` → for each `ecs:DescribeTaskDefinition` → scan `ContainerDefinitions[].Secrets[].ValueFrom` for the secret ARN.

### `secrets` → `logs` — votes: a1=no, a2=sometimes, a3=sometimes, a4=sometimes, a5=no

- **Agent 1** (no): (Secrets Manager does not log to a customer-owned CWL group directly; only via CloudTrail)
- **Agent 2** (sometimes): `secretsmanager:DescribeSecret` → `RotationLambdaARN`; then `lambda:GetFunctionConfiguration` — Lambda logs go to `/aws/lambda/<function-name>` in CWL
- **Agent 3** (sometimes): `secretsmanager:DescribeSecret` → `RotationLambdaARN`; then `logs:DescribeLogGroups` with prefix `/aws/lambda/<function-name>`
- **Agent 4** (sometimes): `secretsmanager:DescribeSecret` → `RotationLambdaARN` → `lambda:GetFunctionConfiguration` → `LoggingConfig.LogGroup` (or default `/aws/lambda/<name>`).
- **Agent 5** (no): — (Secrets Manager audit is via CloudTrail, not CloudWatch Logs directly)

### `secrets` → `role` — votes: a1=sometimes, a2=sometimes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (sometimes): `secretsmanager:GetResourcePolicy(SecretId)` → parse `Principal.AWS` for role ARNs. Also `DescribeSecret` → `RotationLambdaARN` → `lambda:GetFunctionConfiguration` → `Role`.
- **Agent 2** (sometimes): `secretsmanager:GetResourcePolicy` with `SecretId` → parse `Statement[].Principal.AWS` for role ARNs; also `secretsmanager:DescribeSecret.RotationLambdaARN` → Lambda's execution role
- **Agent 3** (yes): `secretsmanager:GetResourcePolicy` → parse `Statement[].Principal.AWS` for role ARNs; also rotation role: `secretsmanager:DescribeSecret` → `RotationLambdaARN` → `lambda:GetFunctionConfiguration` → `Role`
- **Agent 4** (sometimes): `secretsmanager:GetResourcePolicy` → parse `Principal.AWS`; plus `DescribeSecret` → `RotationLambdaARN` → `lambda:GetFunctionConfiguration` → `Role`.
- **Agent 5** (sometimes): `secretsmanager:GetResourcePolicy` → parse `ResourcePolicy.Statement[].Principal.AWS`. Also `secretsmanager:DescribeSecret` → rotation Lambda's execution role.

### `secrets` → `sns` — votes: a1=sometimes, a2=sometimes, a3=no, a4=no, a5=sometimes

- **Agent 1** (sometimes): No direct SecretsManager→SNS API. Via CW alarms: `cloudwatch:DescribeAlarms` on `AWS/SecretsManager` namespace and secret dimensions → `AlarmActions` SNS ARNs.
- **Agent 2** (sometimes): `events:ListRules` + `events:ListTargetsByRule` where `EventPattern.source=aws.secretsmanager` and `Target.Arn` is SNS ARN; no direct Secrets Manager field
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (sometimes): `secretsmanager:DescribeSecret` → `RotationLambdaARN`. No direct SNS topic field on the secret. Check via EventBridge rules filtering `source: aws.secretsmanager`.

### `ses` → `eb-rule` — votes: a1=yes, a2=sometimes, a3=sometimes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `sesv2:GetEmailIdentity(EmailIdentity)` → nothing about events; events go through a configuration set → `sesv2:GetConfigurationSetEventDestinations(ConfigurationSetName)` → destinations include `EventBridgeDestination.EventBusArn`. Match EB rules on that bus.
- **Agent 2** (sometimes): `ses:GetConfigurationSetEventDestinations`/`sesv2:GetConfigurationSetEventDestinations` → `EventDestinations[]` where `EventBridgeDestination` or legacy `SNSDestination` present; `events:ListRules` filtered by `source=aws.ses`
- **Agent 3** (sometimes): `events:ListRules` → `events:DescribeRule` → parse `EventPattern` for `"source":["aws.ses"]` and dimension filters matching identity/config-set
- **Agent 4** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → destinations with `EventBridgeDestination.EventBusArn`. Then `events:ListRules --event-bus-name` → rules on that bus.
- **Agent 5** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → `EventDestinations[].EventBridgeDestination.EventBusArn`. Then `events:ListRules --event-bus-name <bus>`.

### `ses` → `kinesis` — votes: a1=yes, a2=yes, a3=sometimes, a4=yes, a5=sometimes

- **Agent 1** (yes): `sesv2:GetConfigurationSetEventDestinations(ConfigurationSetName)` → `EventDestinations[].KinesisFirehoseDestination.DeliveryStreamArn` (Firehose, not Kinesis Data Streams directly — but Firehose ARNs are in `firehose:`). For raw Kinesis: none natively.
- **Agent 2** (yes): `sesv2:GetConfigurationSetEventDestinations` → `EventDestinations[].KinesisFirehoseDestination.IAMRoleARN` and `DeliveryStreamARN`
- **Agent 3** (sometimes): `ses:ListConfigurationSets` → `sesv2:GetConfigurationSetEventDestinations` (or v1: `ses:DescribeConfigurationSet` with `EventDestinations`) → destinations of type `KinesisFirehoseDestination` have `DeliveryStreamArn`. For native Kinesis Data Streams, no direct SES destination
- **Agent 4** (yes): `sesv2:GetConfigurationSetEventDestinations` → event destinations with `KinesisFirehoseDestination.DeliveryStreamArn` (Firehose) or `KinesisFirehoseDestination.IamRoleArn`. SES writes events to Firehose, not directly to Kinesis Data Streams.
- **Agent 5** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → `EventDestinations[].KinesisFirehoseDestination.DeliveryStreamArn`.

### `ses` → `lambda` — votes: a1=sometimes, a2=yes, a3=sometimes, a4=sometimes, a5=sometimes

- **Agent 1** (sometimes): `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].LambdaAction.FunctionArn`.
- **Agent 2** (yes): `ses:DescribeActiveReceiptRuleSet` → rule `LambdaAction.FunctionArn`; also `sesv2:GetConfigurationSetEventDestinations` — Lambda is not a direct destination (must go via SNS/EventBridge)
- **Agent 3** (sometimes): `ses:DescribeActiveReceiptRuleSet` → rules[].actions[].LambdaAction.FunctionArn`
- **Agent 4** (sometimes): `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].LambdaAction.FunctionArn`. Also Firehose transforms: Firehose stream ARN → `firehose:DescribeDeliveryStream` → `ProcessingConfiguration.Processors[].Parameters.LambdaArn`.
- **Agent 5** (sometimes): `ses:DescribeActiveReceiptRuleSet` + `ses:DescribeReceiptRule` → `Actions[].LambdaAction.FunctionArn`.

### `ses` → `s3` — votes: a1=yes, a2=yes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].S3Action.BucketName`. Also `sesv2:GetConfigurationSetEventDestinations` has no S3 destination natively (only via Firehose→S3).
- **Agent 2** (yes): `ses:DescribeActiveReceiptRuleSet` → rule `S3Action.BucketName`; also config-set Firehose destinations deliver to S3 (via Firehose's own bucket config)
- **Agent 3** (yes): `ses:DescribeActiveReceiptRuleSet` → rules[].actions[].S3Action.BucketName` (and `ObjectKeyPrefix`). SES storing received emails in S3
- **Agent 4** (sometimes): `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].S3Action.BucketName`.
- **Agent 5** (sometimes): `ses:DescribeActiveReceiptRuleSet` + `ses:DescribeReceiptRule` → `Actions[].S3Action.BucketName`.

### `ses` → `sns` — votes: a1=yes, a2=yes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `sesv2:GetConfigurationSetEventDestinations(ConfigurationSetName)` → `EventDestinations[].SnsDestination.TopicArn`. Also `ses:DescribeActiveReceiptRuleSet` → `Rules[].Actions[].SNSAction.TopicArn`/`BounceAction.Sender` etc. And identity notifications: `ses:GetIdentityNotificationAttributes` → `BounceTopic`/`ComplaintTopic`/`DeliveryTopic`.
- **Agent 2** (yes): `ses:DescribeActiveReceiptRuleSet` → rule `SNSAction.TopicArn`; `sesv2:GetConfigurationSetEventDestinations` → `SnsDestination.TopicArn`; also `ses:GetIdentityNotificationAttributes` → `BounceTopic`/`ComplaintTopic`/`DeliveryTopic`
- **Agent 3** (yes): `ses:DescribeActiveReceiptRuleSet` → rules[].actions[].SNSAction.TopicArn`; also configuration-set event destinations:`sesv2:GetConfigurationSetEventDestinations` → `SnsDestination.TopicArn`
- **Agent 4** (sometimes): `sesv2:GetConfigurationSetEventDestinations` → `SnsDestination.TopicArn`. Legacy identity-level: `ses:GetIdentityNotificationAttributes` → `BounceTopic`/`ComplaintTopic`/`DeliveryTopic`.
- **Agent 5** (sometimes): `ses:GetIdentityNotificationAttributes` → `BounceTopic`/`ComplaintTopic`/`DeliveryTopic`. Also `sesv2:GetConfigurationSetEventDestinations` → `SnsDestination.TopicArn`.

### `sfn` → `eb-rule` — votes: a1=yes, a2=yes, a3=sometimes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `events:ListRuleNamesByTarget(TargetArn=<stateMachineArn>)` → rules that start this state machine.
- **Agent 2** (yes): `events:ListRuleNamesByTarget` with `TargetArn=<state-machine-arn>`
- **Agent 3** (sometimes): `events:ListRuleNamesByTarget` with the state-machine ARN → rule names
- **Agent 4** (sometimes): `events:ListRuleNamesByTarget` with the state machine ARN → rules; `events:DescribeRule`.
- **Agent 5** (sometimes): `events:ListRuleNamesByTarget` with the state machine ARN.

### `sns-sub` → `ecs` — votes: a1=sometimes, a2=no, a3=no, a4=no, a5=no

- **Agent 1** (sometimes): No direct API. `sns:GetSubscriptionAttributes(SubscriptionArn)` → `Endpoint` field. If `Protocol=="sqs"`, trace SQS queue consumers via CloudWatch or inspect ECS task definitions referencing that queue URL in env — reverse iteration. Mostly `no`.
- **Agent 2** (no):
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (no): — (SNS subscriptions don't target ECS clusters/services directly; only via EventBridge or Lambda shim)

### `sqs` → `eb-rule` — votes: a1=yes, a2=yes, a3=yes, a4=sometimes, a5=sometimes

- **Agent 1** (yes): `events:ListRuleNamesByTarget(TargetArn=<queueArn>)` → rules that target this SQS queue.
- **Agent 2** (yes): `events:ListRuleNamesByTarget` with `TargetArn=<queue-arn>`
- **Agent 3** (yes): `events:ListRuleNamesByTarget` with SQS queue ARN → rules targeting this queue
- **Agent 4** (sometimes): `events:ListRuleNamesByTarget` with the queue ARN → rules; `events:DescribeRule`.
- **Agent 5** (sometimes): `events:ListRuleNamesByTarget` with the queue ARN.

### `tg` → `role` — votes: a1=no, a2=sometimes, a3=no, a4=no, a5=sometimes

- **Agent 1** (no): (Target groups have no IAM role attribute)
- **Agent 2** (sometimes): `elbv2:DescribeTargetGroups` → `TargetType`; if `lambda` → `elbv2:DescribeTargetHealth` → `Target.Id` (Lambda ARN) → `lambda:GetFunctionConfiguration.Role`
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (sometimes): `elbv2:DescribeTargetGroups` → `TargetType == "lambda"` → `elbv2:DescribeTargetHealth` → `Target.Id` = Lambda ARN → `lambda:GetFunctionConfiguration` → `Role`.

### `tgw` → `role` — votes: a1=sometimes, a2=sometimes, a3=no, a4=no, a5=sometimes

- **Agent 1** (sometimes): No per-TGW role attribute. SLR `AWSServiceRoleForVPCTransitGateway` via `iam:GetRole`. For RAM-shared TGWs: `ram:ListResources(resourceArns=[tgwArn])` → principals (accounts, not roles typically). Mostly `no` for a direct per-TGW role.
- **Agent 2** (sometimes): `ram:GetResourceShares` + `ram:ListResources` filtered by TGW ARN → `ram:ListPrincipals` (returns accounts/OUs/roles); TGW itself has no role field
- **Agent 3** (no):
- **Agent 4** (no):
- **Agent 5** (sometimes): `iam:GetRole` with name `AWSServiceRoleForVPCTransitGateway`. No TGW-ARN-scoped API returns it; relationship is implicit.

### `tgw` → `rtb` — votes: a1=yes, a2=yes, a3=sometimes, a4=yes, a5=yes

- **Agent 1** (yes): `ec2:DescribeTransitGatewayRouteTables(Filters=[{Name:"transit-gateway-id",Values:[<tgwId>]}])` → TGW route tables (these are TGW RTBs, not VPC RTBs). For VPC RTBs with TGW routes: `ec2:DescribeRouteTables(Filters=[{Name:"route.transit-gateway-id",Values:[<tgwId>]}])`.
- **Agent 2** (yes): `ec2:DescribeTransitGatewayRouteTables` with `Filters=[transit-gateway-id]`; VPC route tables that route to the TGW: `ec2:DescribeRouteTables` filter `Routes.TransitGatewayId=<tgw-id>`
- **Agent 3** (sometimes): `ec2:DescribeTransitGatewayAttachments` filter by `transit-gateway-id` → attachment IDs; for VPC attachments: `ec2:DescribeRouteTables` filter `route.transit-gateway-id=<tgw-id>` → VPC route tables with a route to this TGW
- **Agent 4** (yes): `ec2:DescribeTransitGatewayRouteTables` with filter `transit-gateway-id` → TGW route tables. Note: these are TGW route tables, not VPC route tables. VPC-level RTBs are linked via `ec2:DescribeRouteTables` filter `route.transit-gateway-id`.
- **Agent 5** (yes): `ec2:DescribeTransitGatewayRouteTables` with filter `transit-gateway-id=<id>`. Returns TGW route tables (not VPC route tables — note: TGW RT is a distinct resource). For VPC route tables that route through the TGW: `ec2:DescribeRouteTables` → filter `route.transit-gateway-id=<id>`.

## Summary

- Total pairs: 115
- Unanimous `yes`: 16
- Unanimous `no`: 13
- Split/mixed: 86
