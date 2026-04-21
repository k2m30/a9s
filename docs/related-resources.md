# Related Resources — Golden Contract

> ⚠️ **SINGLE SOURCE OF TRUTH — DO NOT EDIT AD-HOC**
> 
> This document defines, for every registered a9s resource type, the AWS
> resources that MUST appear in the detail-view RELATED panel (right column).
> 
> The contract is anchored to:
> - The **AWS API Reference** for each resource type (URL cited per section).
> - **DevOps operational pivots** — resources an engineer reaches for during
> incident response, audit, capacity review, or infra debugging.
> 
> **How this document was built.** Six independent senior-DevOps audits ran
> blind (no access to existing a9s code or tests), each producing a complete
> expected-related-panel table from AWS API docs + operational knowledge.
> Results were merged: every pivot listed by ≥1 of the 6 audits was included
> unless manual AWS-API verification confirmed it was resource-local (e.g.
> bucket policy, queue policy) or an otherwise niche path.
> 
> **Drift has already happened once — don't let it happen again.** Features
> previously registered were removed during refactors and never restored.
> This doc is the backstop.

## Policy

1. **Addition** — anyone adding a row or row-entry MUST cite the AWS API field
   or a concrete DevOps workflow (one line). Put the citation in the reasoning
   column; the policy-review PR MUST NOT merge without it.
2. **Removal** — anyone removing a row-entry MUST cite why the AWS API or
   workflow reference no longer applies. Evidence > opinion.
3. **New resource type** — adding a type to the registry requires adding a
   section here in the same PR. The test suite
   (`tests/unit/qa_related_panel_contract_test.go`) enforces this.
4. **Universal pivots** — `ct-events` (CloudTrail audit trail) is implicitly
   relevant for every registered type; its presence is verified by the test
   suite directly against `resource.GetRelated`, not by per-type rows here.
5. **Never bypass** — do NOT "temporarily" remove a row to unblock a refactor.
   Previous drift happened exactly this way. If the registration is blocking
   you, fix the registration, not the contract.

## Per-type contract

| Type | AWS API | Expected related targets |
|------|---------|--------------------------|
| `acm` | [API_CertificateDetail](https://docs.aws.amazon.com/acm/latest/APIReference/API_CertificateDetail.html) | `apigw`, `cf`, `ct-events`, `elb`, `r53` |
| `alarm` | [API_MetricAlarm](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricAlarm.html) | `apigw`, `asg`, `cb`, `ct-events`, `dbi`, `ec2`, `ecs`, `eks`, `kms`, `lambda`, `logs`, `s3`, `sfn`, `sns`, `waf` |
| `ami` | [API_Image](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Image.html) | `asg`, `cfn`, `ct-events`, `ebs-snap`, `ec2`, `kms`, `ng` |
| `apigw` | [apis](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis.html) | `acm`, `alarm`, `cf`, `ct-events`, `elb`, `kms`, `lambda`, `logs`, `r53`, `role`, `sfn`, `sns`, `vpce`, `waf` |
| `asg` | [API_AutoScalingGroup](https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_AutoScalingGroup.html) | `alarm`, `ami`, `ct-events`, `ec2`, `elb`, `ng`, `role`, `sg`, `sns`, `subnet`, `tg`, `vpc` |
| `athena` | [API_WorkGroup](https://docs.aws.amazon.com/athena/latest/APIReference/API_WorkGroup.html) | `ct-events`, `glue`, `kms`, `logs`, `role`, `s3` |
| `backup` | [API_BackupPlan](https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupPlan.html) | `ct-events`, `kms`, `role`, `sns` |
| `cb` | [API_Project](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_Project.html) | `alarm`, `ct-events`, `ecr`, `kms`, `logs`, `pipeline`, `role`, `s3`, `secrets`, `sg`, `ssm`, `subnet`, `vpc` |
| `cf` | [API_Distribution](https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_Distribution.html) | `acm`, `alarm`, `ct-events`, `elb`, `lambda`, `logs`, `r53`, `s3`, `waf` |
| `cfn` | [API_Stack](https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html) | `cfn`, `ct-events`, `eb-rule`, `role`, `s3`, `sns` |
| `codeartifact` | [API_Repository](https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_Repository.html) | `ct-events`, `kms` |
| `ct-events` | [API_LookupEvents](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html) | `cfn`, `ct-events`, `dbi`, `ddb`, `ec2`, `iam-user`, `kms`, `lambda`, `role`, `s3`, `secrets`, `sg`, `trail`, `vpce` |
| `dbc` | [API_DBCluster](https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBCluster.html) | `alarm`, `ct-events`, `dbi`, `docdb-snap`, `kms`, `logs`, `secrets`, `sg`, `subnet`, `vpc` |
| `dbi` | [API_DBInstance](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html) | `alarm`, `ct-events`, `dbc`, `eni`, `kms`, `logs`, `rds-snap`, `role`, `secrets`, `sg`, `subnet`, `vpc` |
| `ddb` | [API_TableDescription](https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_TableDescription.html) | `alarm`, `backup`, `ct-events`, `kinesis`, `kms`, `lambda`, `logs`, `vpce` |
| `docdb-snap` | [API_DBClusterSnapshot](https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBClusterSnapshot.html) | `backup`, `ct-events`, `dbc`, `kms`, `vpc` |
| `eb` | [API_EnvironmentDescription](https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html) | `alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `elb`, `logs`, `role`, `s3`, `sg`, `tg` |
| `eb-rule` | [API_Rule](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_Rule.html) | `ct-events`, `kinesis`, `lambda`, `logs`, `role`, `sfn`, `sns`, `sqs` |
| `ebs` | [API_Volume](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Volume.html) | `alarm`, `backup`, `cfn`, `ct-events`, `ebs-snap`, `ec2`, `kms` |
| `ebs-snap` | [API_Snapshot](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Snapshot.html) | `ami`, `backup`, `ct-events`, `ebs`, `ec2`, `kms` |
| `ec2` | [API_Instance](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html) | `alarm`, `ami`, `asg`, `backup`, `cfn`, `ct-events`, `ebs`, `ebs-snap`, `eip`, `eni`, `kms`, `logs`, `ng`, `role`, `sg`, `ssm`, `subnet`, `tg`, `vpc` |
| `ecr` | [API_Repository](https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_Repository.html) | `cb`, `cfn`, `ct-events`, `eb-rule`, `ecs-task`, `kms`, `lambda`, `pipeline`, `role` |
| `ecs` | [API_Cluster](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Cluster.html) | `alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `ecs-svc`, `ecs-task`, `kms`, `logs` |
| `ecs-svc` | [API_Service](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html) | `alarm`, `cfn`, `ct-events`, `eb-rule`, `ecr`, `ecs`, `ecs-task`, `elb`, `logs`, `role`, `secrets`, `sfn`, `sg`, `subnet`, `tg`, `vpc` |
| `ecs-task` | [API_Task](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Task.html) | `alarm`, `ct-events`, `ec2`, `ecr`, `ecs`, `ecs-svc`, `eni`, `logs`, `role`, `secrets`, `sg`, `ssm`, `subnet` |
| `efs` | [API_FileSystemDescription](https://docs.aws.amazon.com/efs/latest/ug/API_FileSystemDescription.html) | `alarm`, `backup`, `cfn`, `ct-events`, `ec2`, `ecs-task`, `eni`, `kms`, `lambda`, `sg`, `subnet`, `vpc` |
| `eip` | [API_Address](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html) | `alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `ecs`, `ecs-svc`, `ecs-task`, `eni`, `logs`, `nat` |
| `eks` | [API_Cluster](https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html) | `alarm`, `ami`, `asg`, `cfn`, `ct-events`, `ec2`, `kms`, `logs`, `ng`, `role`, `sg`, `subnet`, `vpc` |
| `elb` | [API_LoadBalancer](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancer.html) | `acm`, `alarm`, `cf`, `cfn`, `ct-events`, `eni`, `r53`, `s3`, `sg`, `subnet`, `tg`, `vpc`, `waf` |
| `eni` | [API_NetworkInterface](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NetworkInterface.html) | `ct-events`, `ec2`, `eip`, `elb`, `lambda`, `nat`, `sg`, `subnet`, `vpc`, `vpce` |
| `glue` | [API_Job](https://docs.aws.amazon.com/glue/latest/webapi/API_Job.html) | `alarm`, `athena`, `cfn`, `ct-events`, `kms`, `logs`, `role`, `s3`, `secrets` |
| `iam-group` | [API_Group](https://docs.aws.amazon.com/IAM/latest/APIReference/API_Group.html) | `ct-events`, `iam-user`, `policy` |
| `iam-user` | [API_User](https://docs.aws.amazon.com/IAM/latest/APIReference/API_User.html) | `ct-events`, `iam-group`, `policy` |
| `igw` | [API_InternetGateway](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InternetGateway.html) | `ct-events`, `rtb`, `vpc` |
| `kinesis` | [API_StreamDescription](https://docs.aws.amazon.com/kinesis/latest/APIReference/API_StreamDescription.html) | `alarm`, `cfn`, `ct-events`, `ddb`, `kms`, `lambda` |
| `kms` | [API_KeyMetadata](https://docs.aws.amazon.com/kms/latest/APIReference/API_KeyMetadata.html) | `ct-events`, `dbi`, `ebs`, `role`, `s3`, `secrets` |
| `lambda` | [API_FunctionConfiguration](https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html) | `alarm`, `apigw`, `cf`, `cfn`, `ct-events`, `ddb`, `eb-rule`, `ecr`, `efs`, `eni`, `kinesis`, `kms`, `logs`, `msk`, `role`, `s3`, `secrets`, `sg`, `sns`, `sns-sub`, `sqs`, `ssm`, `subnet`, `tg`, `vpc` |
| `logs` | [API_LogGroup](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_LogGroup.html) | `alarm`, `apigw`, `ct-events`, `ecs-task`, `kinesis`, `kms`, `lambda`, `s3` |
| `msk` | [v1-clusters](https://docs.aws.amazon.com/msk/1.0/apireference/v1-clusters.html) | `alarm`, `cfn`, `ct-events`, `kms`, `lambda`, `logs`, `s3`, `secrets`, `sg`, `subnet`, `vpc` |
| `nat` | [API_NatGateway](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGateway.html) | `alarm`, `ct-events`, `eip`, `eni`, `rtb`, `subnet`, `vpc` |
| `ng` | [API_Nodegroup](https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html) | `ami`, `asg`, `ct-events`, `ebs`, `ec2`, `eks`, `role`, `sg`, `subnet` |
| `opensearch` | [API_DomainStatus](https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DomainStatus.html) | `acm`, `alarm`, `cfn`, `ct-events`, `kms`, `logs`, `sg`, `subnet`, `vpc` |
| `pipeline` | [API_PipelineDeclaration](https://docs.aws.amazon.com/codepipeline/latest/APIReference/API_PipelineDeclaration.html) | `cb`, `cfn`, `codeartifact`, `ct-events`, `eb-rule`, `ecr`, `ecs-svc`, `kms`, `lambda`, `role`, `s3`, `sns` |
| `policy` | [API_Policy](https://docs.aws.amazon.com/IAM/latest/APIReference/API_Policy.html) | `ct-events`, `iam-group`, `iam-user`, `role` |
| `r53` | [API_HostedZone](https://docs.aws.amazon.com/Route53/latest/APIReference/API_HostedZone.html) | `acm`, `apigw`, `cf`, `ct-events`, `elb`, `logs`, `s3`, `vpc` |
| `rds-snap` | [API_DBSnapshot](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html) | `backup`, `ct-events`, `dbc`, `dbi`, `kms` |
| `redis` | [API_ReplicationGroup](https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_ReplicationGroup.html) | `alarm`, `cfn`, `ct-events`, `kms`, `logs`, `secrets`, `sg`, `sns`, `subnet`, `vpc` |
| `redshift` | [API_Cluster](https://docs.aws.amazon.com/redshift/latest/APIReference/API_Cluster.html) | `alarm`, `cfn`, `ct-events`, `kms`, `logs`, `role`, `s3`, `secrets`, `sg`, `subnet`, `vpc` |
| `role` | [API_Role](https://docs.aws.amazon.com/IAM/latest/APIReference/API_Role.html) | `ct-events`, `ec2`, `eks`, `glue`, `iam-group`, `iam-user`, `lambda`, `ng`, `policy` |
| `rtb` | [API_RouteTable](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RouteTable.html) | `cfn`, `ct-events`, `eni`, `igw`, `nat`, `subnet`, `tgw`, `vpc`, `vpce` |
| `s3` | [API_ListBuckets](https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListBuckets.html) | `athena`, `backup`, `cf`, `cfn`, `ct-events`, `eb-rule`, `glue`, `iam-user`, `kms`, `lambda`, `logs`, `r53`, `role`, `sns`, `sqs`, `trail`, `waf` |
| `secrets` | [API_SecretListEntry](https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_SecretListEntry.html) | `cb`, `cfn`, `codeartifact`, `ct-events`, `dbi`, `eb`, `ecs-task`, `kms`, `lambda`, `logs`, `role`, `sns` |
| `ses` | [API_IdentityInfo](https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_IdentityInfo.html) | `ct-events`, `eb-rule`, `kinesis`, `lambda`, `r53`, `s3`, `sns` |
| `sfn` | [API_StateMachineListItem](https://docs.aws.amazon.com/step-functions/latest/apireference/API_StateMachineListItem.html) | `alarm`, `ct-events`, `eb-rule`, `kms`, `lambda`, `logs`, `role` |
| `sg` | [API_SecurityGroup](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_SecurityGroup.html) | `cfn`, `ct-events`, `ec2`, `elb`, `eni`, `lambda`, `sg`, `vpc` |
| `sns` | [API_Topic](https://docs.aws.amazon.com/sns/latest/api/API_Topic.html) | `alarm`, `ct-events`, `kms`, `role`, `sns-sub` |
| `sns-sub` | [API_Subscription](https://docs.aws.amazon.com/sns/latest/api/API_Subscription.html) | `ct-events`, `lambda`, `sns`, `sqs` |
| `sqs` | [API_GetQueueAttributes](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html) | `alarm`, `ct-events`, `eb-rule`, `kms`, `lambda`, `sns`, `sns-sub`, `sqs` |
| `ssm` | [API_ParameterMetadata](https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_ParameterMetadata.html) | `ct-events`, `kms` |
| `subnet` | [API_Subnet](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Subnet.html) | `asg`, `cfn`, `ct-events`, `ec2`, `efs`, `eks`, `elb`, `eni`, `nat`, `rtb`, `vpc`, `vpce` |
| `tg` | [API_TargetGroup](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_TargetGroup.html) | `alarm`, `asg`, `backup`, `cfn`, `ct-events`, `dbc`, `dbi`, `ec2`, `ecs-svc`, `elb`, `lambda`, `logs`, `rds-snap`, `sg`, `subnet`, `vpc` |
| `tgw` | [API_TransitGateway](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TransitGateway.html) | `ct-events`, `role`, `rtb`, `subnet`, `vpc` |
| `trail` | [API_Trail](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html) | `ct-events`, `kms`, `logs`, `role`, `s3`, `sns` |
| `vpc` | [API_Vpc](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Vpc.html) | `cfn`, `ct-events`, `ec2`, `elb`, `eni`, `igw`, `nat`, `rtb`, `sg`, `subnet`, `tgw`, `vpce` |
| `vpce` | [API_VpcEndpoint](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcEndpoint.html) | `acm`, `alarm`, `cf`, `ct-events`, `eni`, `logs`, `r53`, `rtb`, `s3`, `sg`, `subnet`, `tg`, `vpc`, `waf` |
| `waf` | [API_WebACL](https://docs.aws.amazon.com/waf/latest/APIReference/API_WebACL.html) | `alarm`, `apigw`, `cf`, `ct-events`, `elb`, `logs` |

## Per-target reasoning

One entry per `(type, target)` pair. Reasoning is one line anchored to an AWS
API field (preferred) or a concrete DevOps workflow.

### `acm`

AWS API: https://docs.aws.amazon.com/acm/latest/APIReference/API_CertificateDetail.html

- **`apigw`** — API Gateway custom domains using this cert.
- **`cf`** — CloudFront distributions using this cert.
- **`ct-events`** — Audit trail for cert issuance/renewal.
- **`elb`** — Load balancer listeners using this cert.
- **`r53`** — Route 53 hosted zones used for DNS validation.

### `alarm`

AWS API: https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricAlarm.html

- **`apigw`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`asg`** — MetricAlarm.AlarmActions pointing at ASG scaling policies.
- **`cb`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ct-events`** — Audit trail for alarm config changes.
- **`dbi`** — Common alarm dimension: RDS instance metrics.
- **`ec2`** — Common alarm dimension: EC2 CPU / Status Checks.
- **`ecs`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`eks`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`kms`** — Alarms on KMS key usage.
- **`lambda`** — Common alarm dimension: Lambda Errors/Throttles/Duration.
- **`logs`** — Metric-filter-driven alarms point at log groups.
- **`s3`** — S3 request metrics alarm dimension.
- **`sfn`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sns`** — MetricAlarm.AlarmActions / OKActions — SNS topics notified.
- **`waf`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.

### `ami`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Image.html

- **`asg`** — Mentioned by 3/6 independent DevOps audits as an AWS-API or operational pivot.
- **`cfn`** — AMIs often consumed by CloudFormation templates.
- **`ct-events`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ebs-snap`** — AMI block devices reference EBS snapshots.
- **`ec2`** — Reverse lookup: instances using this AMI.
- **`kms`** — AMI BlockDeviceMappings[].Ebs.KmsKeyId.
- **`ng`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.

### `apigw`

AWS API: https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis.html

- **`acm`** — Custom-domain TLS certificate.
- **`alarm`** — Stage latency/error alarms.
- **`cf`** — APIGW often fronted by CloudFront.
- **`ct-events`** — Audit trail for API changes.
- **`elb`** — VpcLink NLB backend.
- **`kms`** — KMS key referenced by Lambda integrations (weak pair: no direct API GW KMS field; follows Lambda integration FunctionConfiguration.KMSKeyArn).
- **`lambda`** — Lambda integrations.
- **`logs`** — API access log destination.
- **`r53`** — R53 alias records for custom domains.
- **`role`** — Invocation/authorizer role.
- **`sfn`** — Step Functions integration target.
- **`sns`** — APIGW -> SNS via integration.
- **`vpce`** — Private APIs expose via VPC endpoint.
- **`waf`** — WebACL attached to the API stage.

### `asg`

AWS API: https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_AutoScalingGroup.html

- **`alarm`** — Alarms that trigger scaling policies.
- **`ami`** — LaunchConfiguration.ImageId or LaunchTemplate.LaunchTemplateData.ImageId — AMI used by instances.
- **`ct-events`** — Audit trail for scaling events / config changes.
- **`ec2`** — Instances the ASG currently manages.
- **`elb`** — AutoScalingGroup.LoadBalancerNames (classic ELBs) + TargetGroupARNs → DescribeTargetGroups.LoadBalancerArns (ALB/NLB).
- **`ng`** — EKS node groups wrap ASGs; shown when parent node group exists.
- **`role`** — AutoScalingGroup.ServiceLinkedRoleARN + LaunchConfiguration/Template IamInstanceProfile → GetInstanceProfile roles.
- **`sg`** — LaunchConfiguration.SecurityGroups or LaunchTemplate.SecurityGroupIds / NetworkInterfaces[].Groups.
- **`sns`** — DescribeNotificationConfigurations.TopicARN + DescribeLifecycleHooks.NotificationTargetARN (SNS-only).
- **`subnet`** — AutoScalingGroup.VPCZoneIdentifier — subnets the ASG launches into.
- **`tg`** — AutoScalingGroup.TargetGroupARNs — TGs the ASG registers instances with.
- **`vpc`** — AutoScalingGroup.VPCZoneIdentifier → DescribeSubnets.VpcId — VPC(s) the ASG operates in.

### `athena`

AWS API: https://docs.aws.amazon.com/athena/latest/APIReference/API_WorkGroup.html

- **`ct-events`** — Audit trail for workgroup changes.
- **`glue`** — Glue Data Catalog backing Athena.
- **`kms`** — Result-encryption key.
- **`logs`** — Workgroup query logs.
- **`role`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`s3`** — Query result output bucket.

### `backup`

AWS API: https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupPlan.html

- **`ct-events`** — Audit trail for plan/selection/job events.
- **`kms`** — Recovery-point encryption key.
- **`role`** — Backup service role used for restore jobs.
- **`sns`** — Vault notifications.

### `cb`

AWS API: https://docs.aws.amazon.com/codebuild/latest/APIReference/API_Project.html

- **`alarm`** — Build-failure alarms.
- **`ct-events`** — Audit trail for build events.
- **`ecr`** — ECR repos the project pushes to.
- **`kms`** — EncryptionKey on artifacts.
- **`logs`** — Build log group.
- **`pipeline`** — Pipelines consuming this project.
- **`role`** — Project.ServiceRole.
- **`s3`** — Source/artifact buckets.
- **`secrets`** — Secrets as build env variables.
- **`sg`** — VpcConfig.SecurityGroupIds.
- **`ssm`** — SSM parameters as build env.
- **`subnet`** — VpcConfig.Subnets.
- **`vpc`** — VpcConfig.VpcId.

### `cf`

AWS API: https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_Distribution.html

- **`acm`** — Distribution.ViewerCertificate.AcmCertificateArn.
- **`alarm`** — Distribution error-rate alarms.
- **`ct-events`** — Audit trail for distribution changes.
- **`elb`** — ALB origins.
- **`lambda`** — Lambda@Edge associations.
- **`logs`** — Realtime / access logs.
- **`r53`** — Route 53 alias records pointing here.
- **`s3`** — S3 origins.
- **`waf`** — Distribution.WebACLId.

### `cfn`

AWS API: https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html

- **`cfn`** — Nested stacks.
- **`ct-events`** — Audit trail for stack events.
- **`eb-rule`** — Stack-event publishing via EventBridge.
- **`role`** — Stack.RoleARN — stack service role.
- **`s3`** — TemplateURL S3 location.
- **`sns`** — Stack.NotificationARNs — event topics.

### `codeartifact`

AWS API: https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_Repository.html

- **`ct-events`** — Audit trail for repo policy/package events.
- **`kms`** — Domain `EncryptionKey` (resolved via `DescribeDomain` using the repo's `DomainName` + `DomainOwner`); CodeArtifact encryption is a domain-level, not repository-level, property. <!-- amended by a9s-resource-spec during codeartifact gen: AWS SDK Go v2 shows EncryptionKey lives on DomainDescription/DomainSummary, not RepositoryDescription/RepositorySummary -->

### `ct-events`

AWS API: https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html

- **`iam-user`** — `userIdentity.userName` (Type=IAMUser) — events performed by this IAM user.
- **`role`** — `userIdentity.sessionContext.sessionIssuer.arn` (Type=AssumedRole) — events performed under this role.
- **`ec2`** — `resources[].ARN` matching EC2 instance ARNs — EC2-targeted CloudTrail events.
- **`s3`** — `resources[].ARN` matching S3 bucket ARNs — data-plane and management events on S3.
- **`lambda`** — `resources[].ARN` matching Lambda function ARNs — invocation and config events.
- **`dbi`** — `resources[].ARN` matching RDS instance ARNs — RDS management events.
- **`kms`** — `resources[].ARN` matching KMS key ARNs — key usage and policy events.
- **`secrets`** — `resources[].ARN` matching Secrets Manager ARNs — secret access and rotation events.
- **`vpce`** — `resources[].ARN` matching VPC endpoint ARNs — endpoint policy and lifecycle events.
- **`sg`** — `resources[].ARN` matching security group ARNs — rule change and association events.
- **`ddb`** — `resources[].ARN` matching DynamoDB table ARNs — table management events.
- **`cfn`** — `resources[].ARN` matching CloudFormation stack ARNs — stack lifecycle events.
- **`trail`** — `resources[].ARN` matching CloudTrail trail ARNs — trail config and status events.
- **`ct-events` (by AccessKeyId)** — Self-pivot: convenience filter within ct-events by `userIdentity.accessKeyId`.
- **`ct-events` (by Username)** — Self-pivot: convenience filter within ct-events by `userIdentity.userName`.
- **`ct-events` (by EventName)** — Self-pivot: convenience filter within ct-events by `eventName`.
- **`ct-events` (by SharedEventId)** — Self-pivot: convenience filter within ct-events by `sharedEventId`.

### `dbc`

AWS API: https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBCluster.html

- **`alarm`** — Cluster CW alarms.
- **`ct-events`** — Audit trail for cluster changes.
- **`dbi`** — Cluster member instances.
- **`docdb-snap`** — Cluster snapshots.
- **`kms`** — Cluster encryption key.
- **`logs`** — Cluster log exports.
- **`secrets`** — Master credentials in Secrets Manager.
- **`sg`** — VpcSecurityGroups — cluster SGs.
- **`subnet`** — DBSubnetGroup subnets.
- **`vpc`** — DBSubnetGroup VPC.

### `dbi`

AWS API: https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html

- **`alarm`** — CloudWatch alarms on CPU/Storage/Connections.
- **`ct-events`** — Audit trail for DB config / modifyDBInstance.
- **`dbc`** — Aurora instance → cluster.
- **`eni`** — DB instances back onto ENIs.
- **`kms`** — KmsKeyId — storage encryption key.
- **`logs`** — DB engine log exports (e.g. /aws/rds/instance/<id>/error).
- **`rds-snap`** — Snapshots of this instance.
- **`role`** — MonitoringRoleArn / S3-integration role.
- **`secrets`** — Secrets Manager entries holding master credentials.
- **`sg`** — VpcSecurityGroups — SGs attached to the instance.
- **`subnet`** — DBSubnetGroup.Subnets — subnets the instance spans.
- **`vpc`** — DBSubnetGroup.VpcId.

### `ddb`

AWS API: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_TableDescription.html

- **`alarm`** — Throttle/error/ReadCapacity alarms.
- **`backup`** — AWS Backup recovery points.
- **`ct-events`** — Audit trail for table schema/capacity changes.
- **`kinesis`** — Kinesis Data Streams for DDB.
- **`kms`** — SSEDescription.KMSMasterKeyArn — table encryption key.
- **`lambda`** — Lambdas consuming DDB Streams from this table.
- **`logs`** — ContributorInsights / Streams logs.
- **`vpce`** — Gateway endpoint for DynamoDB.

### `docdb-snap`

AWS API: https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBClusterSnapshot.html

- **`backup`** — Snapshots covered by Backup vaults.
- **`ct-events`** — Audit trail for snapshot events.
- **`dbc`** — Source cluster.
- **`kms`** — Encryption key.
- **`vpc`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.

### `eb`

AWS API: https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html

- **`alarm`** — Health alarms.
- **`asg`** — Environment's backing ASG (elasticbeanstalk:environment-name tag on ASG).
- **`cfn`** — Beanstalk creates a CloudFormation stack per environment (awseb-{envId}-stack prefix).
- **`ct-events`** — Audit trail for environment config changes.
- **`ec2`** — Instances running the environment (elasticbeanstalk:environment-name tag on EC2 instances).
- **`elb`** — DescribeEnvironmentResources.EnvironmentResources.LoadBalancers[].Name — ELB(s) fronting this environment.
- **`logs`** — Log groups prefixed /aws/elasticbeanstalk/{envName}/.
- **`role`** — DescribeConfigurationSettings OptionSettings: aws:autoscaling:launchconfiguration/IamInstanceProfile → GetInstanceProfile roles; aws:elasticbeanstalk:environment/ServiceRole.
- **`s3`** — DescribeApplicationVersions.ApplicationVersions[].SourceBundle.S3Bucket — buckets holding application version bundles.
- **`sg`** — DescribeConfigurationSettings OptionSettings: aws:autoscaling:launchconfiguration/SecurityGroups and aws:elbv2:loadbalancer/SecurityGroups.
- **`tg`** — DescribeEnvironmentResources.LoadBalancers[].Name → elbv2:DescribeListeners → DefaultActions/ForwardConfig TargetGroupArn.

### `eb-rule`

AWS API: https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_Rule.html

- **`ct-events`** — Audit trail for rule changes.
- **`kinesis`** — Rule → Kinesis target.
- **`lambda`** — Lambda targets of this rule.
- **`logs`** — Rule → CW Logs target.
- **`role`** — Rule.RoleArn — IAM role used for target invocation.
- **`sfn`** — Step Functions state-machine targets.
- **`sns`** — SNS targets of this rule.
- **`sqs`** — SQS targets of this rule.

### `ebs`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Volume.html

- **`alarm`** — Volume CW alarms (throughput/IOPS).
- **`backup`** — Volumes covered by AWS Backup.
- **`cfn`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ct-events`** — Audit trail for volume changes.
- **`ebs-snap`** — Snapshots of this volume.
- **`ec2`** — Volume.Attachments[].InstanceId.
- **`kms`** — Volume.KmsKeyId — at-rest encryption key.

### `ebs-snap`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Snapshot.html

- **`ami`** — AMIs derived from this snapshot.
- **`backup`** — Snapshots covered by AWS Backup.
- **`ct-events`** — Audit trail for snapshot events.
- **`ebs`** — Source volume.
- **`ec2`** — Instances that could be restored from this snapshot.
- **`kms`** — Snapshot encryption key.

### `ec2`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html

- **`alarm`** — CloudWatch alarms watching this instance — first signal of impact.
- **`ami`** — Instance.ImageId — provenance of the running image; compare against latest approved AMI.
- **`asg`** — ASG that owns the instance (if any) — lifecycle context.
- **`backup`** — Instances covered by AWS Backup.
- **`cfn`** — CloudFormation stack that created it — infra-as-code linkage.
- **`ct-events`** — Audit trail for all API calls touching this instance.
- **`ebs`** — Instance.BlockDeviceMappings[].Ebs.VolumeId — attached storage; capacity/IOPS troubleshooting.
- **`ebs-snap`** — Instance's AMI snapshots for rollback/forensic workflows.
- **`eip`** — Addresses associated with the instance; traffic attribution.
- **`eni`** — Instance.NetworkInterfaces[] — ENIs for multi-homed or secondary interfaces.
- **`kms`** — Instance-attached volume encryption keys.
- **`logs`** — CloudWatch Log Groups fed by CloudWatch agent on this instance.
- **`ng`** — Nodegroup owning this instance.
- **`role`** — IamInstanceProfile → role — permissions the instance operates with.
- **`sg`** — Instance.SecurityGroups[] — ingress/egress rules; first stop for connectivity issues.
- **`ssm`** — SSM Managed Instances / Session Manager.
- **`subnet`** — Instance.SubnetId — primary ENI's subnet; used when diagnosing placement/routing.
- **`tg`** — Target groups this instance is registered with — traffic routing.
- **`vpc`** — Instance.VpcId — network parent; pivoted to for VPC-wide troubleshooting.

### `ecr`

AWS API: https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_Repository.html

- **`cb`** — CodeBuild projects that push images.
- **`cfn`** — CloudFormation stack that created the repo.
- **`ct-events`** — Audit trail for image push/pull, policy changes.
- **`eb-rule`** — Image-scan EventBridge events.
<!-- amended by a9s-resource-spec during ecr gen: removed stale `ecs` bullet — contradicts the per-type contract row (line 66) and the explicit non-match at line 1092 (`ecr → ecs` has no first-class API; use `ecr → ecs-task`). -->
- **`ecs-task`** — Task defs pull from repo.
- **`kms`** — EncryptionConfiguration.KmsKey.
- **`lambda`** — Lambda functions using container image from this repo.
- **`pipeline`** — Pipelines pushing to this repo.
- **`role`** — Pull/push IAM roles.

### `ecs`

AWS API: https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Cluster.html

- **`alarm`** — Cluster-level alarms on resource utilization.
- **`asg`** — Container-instance ASG.
- **`cfn`** — CloudFormation stack that created the cluster.
- **`ct-events`** — Audit trail for cluster config changes.
- **`ec2`** — Container instances (if EC2 launch type).
- **`ecs-svc`** — Services running on this cluster.
- **`ecs-task`** — Tasks running in this cluster.
- **`kms`** — ExecuteCommandConfiguration.KmsKeyId.
- **`logs`** — awslogs driver log groups.

### `ecs-svc`

AWS API: https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html

- **`alarm`** — Service alarms (CPU/Memory/PendingTasks).
- **`cfn`** — CloudFormation stack that created the service.
- **`ct-events`** — Audit trail for service changes.
- **`eb-rule`** — Scheduled tasks are EB-driven.
- **`ecr`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ecs`** — Parent cluster.
- **`ecs-task`** — Running tasks for this service.
- **`elb`** — Load balancer fronting the service (via TG).
- **`logs`** — Task container logs.
- **`role`** — Service.RoleArn / task-level roles.
- **`secrets`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sfn`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sg`** — AwsvpcConfiguration.SecurityGroups.
- **`subnet`** — AwsvpcConfiguration.Subnets.
- **`tg`** — Service.LoadBalancers[].TargetGroupArn — target groups.
- **`vpc`** — AwsvpcConfiguration subnets imply VPC parent.

### `ecs-task`

AWS API: https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Task.html

- **`alarm`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ct-events`** — Audit trail for task start/stop events.
- **`ec2`** — Container-instance EC2.
- **`ecr`** — Containers pulled from ECR.
- **`ecs`** — Parent cluster.
- **`ecs-svc`** — Owning service (Task.Group = 'service:<name>').
- **`eni`** — Task ENI (awsvpc mode).
- **`logs`** — CloudWatch Log Groups receiving container logs.
- **`role`** — Task / execution role.
- **`secrets`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sg`** — Task ENI SGs.
- **`ssm`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`subnet`** — Task ENI subnet.

### `efs`

AWS API: https://docs.aws.amazon.com/efs/latest/ug/API_FileSystemDescription.html

- **`alarm`** — BurstCreditBalance / PercentIOLimit alarms.
- **`backup`** — AWS Backup recovery points.
- **`cfn`** — CloudFormation stack that created the FS.
- **`ct-events`** — Audit trail for FS changes.
- **`ec2`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ecs-task`** — ECS tasks mounting EFS.
- **`eni`** — Mount-target ENIs.
- **`kms`** — FileSystemDescription.KmsKeyId.
- **`lambda`** — Lambdas mounting this file system.
- **`sg`** — MountTarget security groups.
- **`subnet`** — MountTarget subnets.
- **`vpc`** — Mount targets live in a VPC.

### `eip`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html

- **`alarm`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`asg`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`cfn`** — CFN stack that created the EIP.
- **`ct-events`** — Audit trail for allocation/association.
- **`ec2`** — Associated instance.
- **`ecs`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ecs-svc`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ecs-task`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`eni`** — Associated ENI.
- **`logs`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`nat`** — NAT gateway consuming this EIP.

### `eks`

AWS API: https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html

- **`alarm`** — CloudWatch alarms on cluster/control-plane metrics.
- **`ami`** — AMIs applied to worker nodes.
- **`asg`** — Backing ASG.
- **`cfn`** — CloudFormation stack that created the cluster.
- **`ct-events`** — Audit trail for cluster config changes.
- **`ec2`** — Worker-node instances.
- **`kms`** — EncryptionConfig.Provider.KeyArn.
- **`logs`** — Control-plane log groups /aws/eks/<cluster>/cluster.
- **`ng`** — Node groups attached to the cluster.
- **`role`** — Cluster.RoleArn — EKS service role.
- **`sg`** — Cluster.ResourcesVpcConfig.ClusterSecurityGroupId + additional SGs.
- **`subnet`** — Cluster.ResourcesVpcConfig.SubnetIds — cluster subnets.
- **`vpc`** — Cluster.ResourcesVpcConfig.VpcId — cluster's VPC.

### `elb`

AWS API: https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancer.html

- **`acm`** — HTTPS listener certificate.
- **`alarm`** — CloudWatch alarms on LB metrics (4xx/5xx/latency).
- **`cf`** — ALB as CloudFront origin.
- **`cfn`** — CloudFormation stack that created the LB.
- **`ct-events`** — Audit trail for LB config changes.
- **`eni`** — LB creates ENIs per AZ.
- **`r53`** — Route 53 alias/records pointing at this LB.
- **`s3`** — Access-log S3 destination.
- **`sg`** — Attached security groups (ALB only).
- **`subnet`** — AZ subnets the LB listens in.
- **`tg`** — Target groups attached to this LB.
- **`vpc`** — LoadBalancer.VpcId.
- **`waf`** — WebACL associated with ALB.

### `eni`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NetworkInterface.html

- **`ct-events`** — Audit trail for ENI attach/detach.
- **`ec2`** — Attached instance (if any).
- **`eip`** — Associated EIP (if any).
- **`elb`** — ELB creates ENIs.
- **`lambda`** — Lambda-in-VPC creates ENIs.
- **`nat`** — NAT gateway backing ENI.
- **`sg`** — Attached security groups.
- **`subnet`** — ENI's subnet.
- **`vpc`** — Parent VPC.
- **`vpce`** — Interface endpoint ENIs.

### `glue`

AWS API: https://docs.aws.amazon.com/glue/latest/webapi/API_Job.html

- **`alarm`** — Job-run failure alarms.
- **`athena`** — Athena queries Glue Catalog.
- **`cfn`** — CloudFormation stack that created the job.
- **`ct-events`** — Audit trail for job events.
- **`kms`** — Data + bookmark encryption key.
- **`logs`** — Job log destination.
- **`role`** — Job.Role.
- **`s3`** — Sources/sinks in S3.
- **`secrets`** — Glue connections → Secrets Manager.

### `iam-group`

AWS API: https://docs.aws.amazon.com/IAM/latest/APIReference/API_Group.html

- **`ct-events`** — Audit trail for group membership changes.
- **`iam-user`** — Members of this group.
- **`policy`** — Attached managed policies.

### `iam-user`

AWS API: https://docs.aws.amazon.com/IAM/latest/APIReference/API_User.html

- **`ct-events`** — Audit trail for user actions and credential changes.
- **`iam-group`** — Groups the user belongs to.
- **`policy`** — Attached managed policies.

### `igw`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InternetGateway.html

- **`ct-events`** — Audit trail for attach/detach events.
- **`rtb`** — Route tables with 0.0.0.0/0 → igw default routes.
- **`vpc`** — Attached VPC.

### `kinesis`

AWS API: https://docs.aws.amazon.com/kinesis/latest/APIReference/API_StreamDescription.html

- **`alarm`** — IteratorAge / IncomingRecords alarms.
- **`cfn`** — CloudFormation stack that created the stream.
- **`ct-events`** — Audit trail for stream changes.
- **`ddb`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`kms`** — StreamDescription.KeyId — stream-encryption key.
- **`lambda`** — Lambda consumers of the stream.

### `kms`

AWS API: https://docs.aws.amazon.com/kms/latest/APIReference/API_KeyMetadata.html

- **`ct-events`** — Audit trail for key usage (Encrypt/Decrypt calls).
- **`dbi`** — RDS instances using this key.
- **`ebs`** — EBS volumes using this key.
- **`role`** — Key policy trusts roles.
- **`s3`** — S3 buckets using this key for SSE-KMS.
- **`secrets`** — Secrets encrypted with this key.

### `lambda`

AWS API: https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html

- **`alarm`** — Errors/Throttles/Duration alarms watching the function.
- **`apigw`** — API Gateway integrations.
- **`cf`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`cfn`** — CloudFormation stack that created the function.
- **`ct-events`** — Audit trail for function config changes.
- **`ddb`** — DDB Streams triggers.
- **`eb-rule`** — EventBridge rules with this function as a target.
- **`ecr`** — Container-image Lambda.
- **`efs`** — FileSystemConfigs.
- **`eni`** — Lambda-in-VPC ENIs.
- **`kinesis`** — Kinesis event-source mapping.
- **`kms`** — Env-var encryption key.
- **`logs`** — CloudWatch Log Groups /aws/lambda/<name> where function logs land.
- **`msk`** — MSK event-source mapping.
- **`role`** — FunctionConfiguration.Role — execution permissions.
- **`s3`** — S3 event-source mapping.
- **`secrets`** — Secrets accessed at runtime.
- **`sg`** — FunctionConfiguration.VpcConfig.SecurityGroupIds — function ENI SGs.
- **`sns`** — SNS event source mapping.
- **`sns-sub`** — SNS subscriptions delivering to the function.
- **`sqs`** — SQS queues invoking the function or used as DLQ.
- **`ssm`** — Parameters as config.
- **`subnet`** — FunctionConfiguration.VpcConfig.SubnetIds — function ENI subnets.
- **`tg`** — TargetGroup registration.
- **`vpc`** — FunctionConfiguration.VpcConfig.VpcId — VPC the function runs in.

### `logs`

AWS API: https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_LogGroup.html

- **`alarm`** — Metric-filter-driven alarms.
- **`apigw`** — APIGW access logs.
- **`ct-events`** — Audit trail for log group changes.
- **`ecs-task`** — awslogs driver log groups.
- **`kinesis`** — Subscription filter → Kinesis/Firehose.
- **`kms`** — LogGroup.KmsKeyId.
- **`lambda`** — Lambdas whose logs land here OR subscription-filter consumers.
- **`s3`** — Export tasks to S3.

### `msk`

AWS API: https://docs.aws.amazon.com/msk/1.0/apireference/v1-clusters.html

- **`alarm`** — MSK broker CW alarms.
- **`cfn`** — CloudFormation stack that created the cluster.
- **`ct-events`** — Audit trail for cluster changes.
- **`kms`** — EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId.
- **`lambda`** — Lambdas consuming from MSK topics (event source mapping).
- **`logs`** — LoggingInfo BrokerLogs.CloudWatchLogs.
- **`s3`** — LoggingInfo BrokerLogs.S3.
<!-- amended by a9s-resource-spec during msk gen: SDK Sasl.Scram carries only an `Enabled` bool, not secret ARNs; the attached SCRAM-secret ARNs are returned by ListScramSecrets(ClusterArn=...). -->
- **`secrets`** — ClientAuthentication.Sasl.Scram (enabled flag) + `ListScramSecrets(ClusterArn)` returning `SecretArnList[]` — the Secrets Manager secrets attached for SASL/SCRAM auth.
- **`sg`** — BrokerNodeGroupInfo.SecurityGroups — broker SGs.
- **`subnet`** — BrokerNodeGroupInfo.ClientSubnets — broker subnets.
<!-- amended by a9s-resource-spec during msk gen: SDK BrokerNodeGroupInfo has no `ClientVpcIpAddresses` field; the VPC is derived from the ClientSubnets by cross-referencing the loaded `subnet` list (Subnet.VpcId). -->
- **`vpc`** — derived from BrokerNodeGroupInfo.ClientSubnets → cross-reference subnet list → Subnet.VpcId.

### `nat`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGateway.html

- **`alarm`** — NAT bandwidth/error alarms.
- **`ct-events`** — Audit trail for NAT changes.
- **`eip`** — NatGatewayAddresses[].AllocationId — attached EIPs.
- **`eni`** — NAT backing ENI.
- **`rtb`** — Route tables with default routes pointing at this NAT.
- **`subnet`** — Subnet the NAT lives in (must be public).
- **`vpc`** — Parent VPC.

### `ng`

AWS API: https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html

- **`ami`** — Nodegroup LaunchTemplate ImageId via ec2:DescribeLaunchTemplateVersions.
- **`asg`** — Nodegroup.Resources.AutoScalingGroups — backing ASG.
- **`ct-events`** — Audit trail for nodegroup changes.
- **`ebs`** — ASG → instances → ec2:DescribeInstances BlockDeviceMappings.Ebs.VolumeId.
- **`ec2`** — Worker-node instances.
- **`eks`** — Parent EKS cluster.
- **`role`** — Nodegroup.NodeRole — IAM role nodes assume.
- **`sg`** — RemoteAccess.SourceSecurityGroups.
- **`subnet`** — Nodegroup.Subnets[] (direct field).

### `opensearch`

AWS API: https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DomainStatus.html

- **`acm`** — Custom endpoint TLS cert.
- **`alarm`** — Cluster health alarms.
- **`cfn`** — CloudFormation stack that created the domain.
- **`ct-events`** — Audit trail for domain config changes.
- **`kms`** — EncryptionAtRestOptions.KmsKeyId.
- **`logs`** — Slow/index/audit log destinations.
- **`sg`** — VPCOptions.SecurityGroupIds — domain ENI SGs.
- **`subnet`** — VPCOptions.SubnetIds — domain ENI subnets.
- **`vpc`** — VPCOptions.VPCId — attached VPC (if any).

### `pipeline`

AWS API: https://docs.aws.amazon.com/codepipeline/latest/APIReference/API_PipelineDeclaration.html

- **`cb`** — CodeBuild projects used as pipeline actions.
- **`cfn`** — Deploy CFN action.
- **`codeartifact`** — CodeArtifact as source.
- **`ct-events`** — Audit trail for pipeline state changes.
- **`eb-rule`** — Triggered by EventBridge.
- **`ecr`** — Push/pull images.
- **`ecs-svc`** — Deploy to ECS.
- **`kms`** — Artifact-store encryption key.
- **`lambda`** — Invoke Lambda action.
- **`role`** — Pipeline service role.
- **`s3`** — Artifact store bucket.
- **`sns`** — Approval SNS topic.

### `policy`

AWS API: https://docs.aws.amazon.com/IAM/latest/APIReference/API_Policy.html

- **`ct-events`** — Audit trail for policy version / attach events.
- **`iam-group`** — Groups with this policy attached.
- **`iam-user`** — Users with this policy attached.
- **`role`** — Roles with this policy attached.

### `r53`

AWS API: https://docs.aws.amazon.com/Route53/latest/APIReference/API_HostedZone.html

- **`acm`** — DNS-validated certs reference this zone.
- **`apigw`** — APIGW custom domain aliases.
- **`cf`** — CloudFront distributions aliased from records in this zone.
- **`ct-events`** — Audit trail for zone record changes.
- **`elb`** — Load balancers aliased from records in this zone.
- **`logs`** — Query logs → CW Logs.
- **`s3`** — Alias to S3 website endpoint.
- **`vpc`** — Private hosted zones VPC association.

### `rds-snap`

AWS API: https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html

- **`backup`** — Snapshots covered by AWS Backup.
- **`ct-events`** — Audit trail for snapshot create/restore/copy.
- **`dbc`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`dbi`** — Source DB instance.
- **`kms`** — Encryption key.

### `redis`

AWS API: https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_ReplicationGroup.html

- **`alarm`** — Replication-group CW alarms.
- **`cfn`** — CloudFormation stack that created the group.
- **`ct-events`** — Audit trail for group changes.
- **`kms`** — At-rest encryption key.
- **`logs`** — LogDeliveryConfigurations.
- **`secrets`** — AuthTokenSecret.
- **`sg`** — Attached security groups.
- **`sns`** — NotificationTopicArn.
- **`subnet`** — CacheSubnetGroup.Subnets.
- **`vpc`** — CacheSubnetGroup.VpcId.

### `redshift`

AWS API: https://docs.aws.amazon.com/redshift/latest/APIReference/API_Cluster.html

- **`alarm`** — Cluster CW alarms (CPU/DiskSpaceUsed).
- **`cfn`** — CloudFormation stack that created the cluster.
- **`ct-events`** — Audit trail for cluster changes.
- **`kms`** — Cluster.KmsKeyId — storage encryption.
- **`logs`** — LoggingProperties destination.
- **`role`** — IamRoles associated.
- **`s3`** — COPY/UNLOAD / audit-log bucket.
- **`secrets`** — Master credentials in Secrets Manager.
- **`sg`** — Cluster.VpcSecurityGroups — attached SGs.
- **`subnet`** — Cluster.ClusterSubnetGroupName → subnets.
- **`vpc`** — Cluster.VpcId — cluster VPC.

### `role`

AWS API: https://docs.aws.amazon.com/IAM/latest/APIReference/API_Role.html

- **`ct-events`** — Audit trail for role AssumeRole / policy attach events.
- **`ec2`** — EC2 instances assuming this role via instance profile.
- **`eks`** — EKS service role.
- **`glue`** — Glue jobs assuming this role.
- **`iam-group`** — Trust relationships may reference groups.
- **`iam-user`** — Trust may include user principals.
- **`lambda`** — Lambdas executing as this role.
- **`ng`** — EKS node groups assuming this role.
- **`policy`** — Attached managed policies.

### `rtb`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RouteTable.html

- **`cfn`** — CloudFormation stack that created the route table.
- **`ct-events`** — Audit trail for route changes.
- **`eni`** — ENI route targets (e.g. firewall appliances).
- **`igw`** — IGW route targets.
- **`nat`** — NAT gateway route targets.
- **`subnet`** — Explicitly-associated subnets.
- **`tgw`** — Transit gateway route targets.
- **`vpc`** — Parent VPC.
- **`vpce`** — Gateway-endpoint routes.

### `s3`

AWS API: https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListBuckets.html

- **`athena`** — Athena queries over S3 data.
- **`backup`** — S3 covered by AWS Backup.
- **`cf`** — CloudFront distributions with this bucket as origin.
- **`cfn`** — CloudFormation stack that created the bucket.
- **`ct-events`** — Audit trail for bucket-level events.
- **`eb-rule`** — EB rules on S3 object events.
- **`glue`** — Glue crawlers over S3 data.
- **`iam-user`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`kms`** — Bucket SSE-KMS key.
- **`lambda`** — Lambdas with this bucket as event source.
- **`logs`** — Server access-log target bucket.
- **`r53`** — R53 alias to S3 website endpoint.
- **`role`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sns`** — BucketNotification SNS target.
- **`sqs`** — BucketNotification SQS target.
- **`trail`** — CloudTrails writing to this bucket.
- **`waf`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.

### `secrets`

AWS API: https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_SecretListEntry.html

- **`cb`** — Reverse-scan: CodeBuild Project.Environment.EnvironmentVariables where Type=SECRETS_MANAGER and Value==ARN or name prefix.
- **`cfn`** — SecretListEntry.Tags["aws:cloudformation:stack-name"] matched against CFN stack cache.
- **`codeartifact`** — Heuristic: secret Name or Tags contain "codeartifact" (no direct AWS API).
- **`ct-events`** — Audit trail for secret rotation/access.
- **`dbi`** — Reverse-scan: DBInstance.MasterUserSecret.SecretArn == this secret's ARN.
- **`eb`** — Reverse-scan: elasticbeanstalk:DescribeConfigurationSettings OptionSettings[].Value contains `{{resolve:secretsmanager:<ARN>`.
- **`ecs-task`** — Reverse-scan: TaskDefinition.ContainerDefinitions[].Secrets[].ValueFrom==ARN or RepositoryCredentials.CredentialsParameter==ARN.
- **`kms`** — SecretListEntry.KmsKeyId — UUID suffix matched against KMS key cache.
- **`lambda`** — SecretListEntry.RotationLambdaARN — function name suffix matched against Lambda cache.
- **`logs`** — RotationLambdaARN → lambda:GetFunction → FunctionConfiguration.LoggingConfig.LogGroup (or default /aws/lambda/<name>).
- **`role`** — secretsmanager:GetResourcePolicy → Statement[].Principal.AWS role ARNs; RotationLambdaARN → lambda:GetFunction → FunctionConfiguration.Role.
- **`sns`** — RotationLambdaARN → lambda:GetFunction → FunctionConfiguration.DeadLetterConfig.TargetArn if SNS ARN.

### `ses`

AWS API: https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_IdentityInfo.html

- **`ct-events`** — Audit trail for identity changes.
- **`eb-rule`** — sesv2:GetEmailIdentity → ConfigurationSetName → sesv2:GetConfigurationSetEventDestinations → EventBridgeDestination.EventBusArn.
- **`kinesis`** — sesv2:GetEmailIdentity → ConfigurationSetName → sesv2:GetConfigurationSetEventDestinations → KinesisFirehoseDestination.DeliveryStreamArn (Firehose, not Kinesis Data Streams).
- **`lambda`** — ses:DescribeActiveReceiptRuleSet → LambdaAction.FunctionArn (SES v1 only; not available via SESv2 SDK — returns 0).
- **`r53`** — Identity domain (or domain portion of email address) matched against Route 53 hosted zone names.
- **`s3`** — ses:DescribeActiveReceiptRuleSet → S3Action.BucketName (SES v1 only; not available via SESv2 SDK — returns 0).
- **`sns`** — sesv2:GetEmailIdentity → ConfigurationSetName → sesv2:GetConfigurationSetEventDestinations → SnsDestination.TopicArn.

### `sfn`

AWS API: https://docs.aws.amazon.com/step-functions/latest/apireference/API_StateMachineListItem.html

- **`alarm`** — Execution-failure alarms.
- **`ct-events`** — Audit trail for state-machine changes.
- **`eb-rule`** — EventBridge rules with this state machine as target.
- **`kms`** — Execution-data encryption.
- **`lambda`** — Lambda integrations invoked by the state machine.
- **`logs`** — Execution log groups.
- **`role`** — StateMachine.RoleArn — execution role.

### `sg`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_SecurityGroup.html

- **`cfn`** — CloudFormation stack that created the SG.
- **`ct-events`** — Audit trail for rule changes.
- **`ec2`** — Instances that have this SG attached.
- **`elb`** — Load balancers with this SG attached (ALBs only).
- **`eni`** — ENIs with this SG attached (covers Lambda, RDS, etc.).
- **`lambda`** — Lambda VPC ENIs reference SGs.
- **`sg`** — Other SGs referenced in this SG's ingress/egress rules.
- **`vpc`** — Parent VPC.

### `sns`

AWS API: https://docs.aws.amazon.com/sns/latest/api/API_Topic.html

- **`alarm`** — Topic delivery/failure alarms.
- **`ct-events`** — Audit trail for topic changes.
- **`kms`** — KmsMasterKeyId (SSE-KMS).
- **`role`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sns-sub`** — Subscriptions on this topic.

### `sns-sub`

AWS API: https://docs.aws.amazon.com/sns/latest/api/API_Subscription.html

- **`ct-events`** — Audit trail for subscription changes.
- **`lambda`** — Lambda endpoint subscriber.
- **`sns`** — Parent topic.
- **`sqs`** — SQS endpoint subscriber.

### `sqs`

AWS API: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html

- **`alarm`** — ApproximateAgeOfOldestMessage / MessagesVisible alarms.
- **`ct-events`** — Audit trail for queue attribute changes.
- **`eb-rule`** — EB-rule target queue.
- **`kms`** — KmsMasterKeyId (SSE-KMS).
- **`lambda`** — Lambda event-source mappings consuming this queue.
- **`sns`** — SQS subscribed to SNS topic.
- **`sns-sub`** — SNS subscriptions delivering to this queue.
- **`sqs`** — DLQ reference / RedriveTarget.

### `ssm`

AWS API: https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_ParameterMetadata.html

- **`ct-events`** — Audit trail for parameter reads/writes.
- **`kms`** — KeyId — KMS key for SecureString.

### `subnet`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Subnet.html

- **`asg`** — ASGs referencing this subnet.
- **`cfn`** — CloudFormation stack that created the subnet.
- **`ct-events`** — Audit trail for subnet changes.
- **`ec2`** — Instances in this subnet.
- **`efs`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`eks`** — EKS clusters declaring subnet.
- **`elb`** — Load balancer AZ-subnet mappings.
- **`eni`** — ENIs in this subnet.
- **`nat`** — NAT gateways in this subnet.
- **`rtb`** — Route tables associated with this subnet.
- **`vpc`** — Parent VPC.
- **`vpce`** — Interface endpoints in subnet.

### `tg`

AWS API: https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_TargetGroup.html

- **`alarm`** — TG health/unhealthy-host count alarms.
- **`asg`** — ASGs registering into this TG.
- **`backup`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.
- **`cfn`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ct-events`** — Audit trail for TG changes.
- **`dbc`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`dbi`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ec2`** — Instance targets.
- **`ecs-svc`** — ECS services routing to this TG.
- **`elb`** — Load balancers using this TG.
- **`lambda`** — Lambda targets.
- **`logs`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.
- **`rds-snap`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sg`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`subnet`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`vpc`** — TargetGroup.VpcId.

### `tgw`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TransitGateway.html

- **`ct-events`** — Audit trail for attachment changes.
- **`role`** — Cross-account RAM share roles.
- **`rtb`** — VPC route tables with routes targeting this TGW.
- **`subnet`** — VPC attachment subnets.
- **`vpc`** — VPCs attached to this TGW.

### `trail`

AWS API: https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html

- **`ct-events`** — Audit trail for trail config changes (meta!).
- **`kms`** — Trail.KmsKeyId — log-file encryption key.
- **`logs`** — Trail.CloudWatchLogsLogGroupArn — associated log group.
- **`role`** — CloudWatchLogsRoleArn / org-trail role.
- **`s3`** — Trail.S3BucketName — destination bucket.
- **`sns`** — Trail.SnsTopicARN — delivery notifications.

### `vpc`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Vpc.html

- **`cfn`** — CloudFormation stack that created the VPC.
- **`ct-events`** — Audit trail for VPC-level changes.
- **`ec2`** — EC2 instances in this VPC.
- **`elb`** — Load balancers in this VPC.
- **`eni`** — ENIs in VPC.
- **`igw`** — Internet gateways attached to this VPC.
- **`nat`** — NAT gateways in this VPC.
- **`rtb`** — Route tables in this VPC.
- **`sg`** — Security groups scoped to this VPC.
- **`subnet`** — Subnets in this VPC.
- **`tgw`** — VPC attachments to TGWs.
- **`vpce`** — VPC endpoints in this VPC.

### `vpce`

AWS API: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcEndpoint.html

- **`acm`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.
- **`alarm`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.
- **`cf`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ct-events`** — Audit trail for endpoint changes.
- **`eni`** — ENIs backing interface endpoints.
- **`logs`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.
- **`r53`** — Private DNS → R53 private zones.
- **`rtb`** — Route tables for gateway endpoints.
- **`s3`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sg`** — SGs attached to interface endpoints.
- **`subnet`** — Interface endpoint subnets.
- **`tg`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.
- **`vpc`** — Parent VPC.
- **`waf`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.

### `waf`

AWS API: https://docs.aws.amazon.com/waf/latest/APIReference/API_WebACL.html

- **`alarm`** — Blocked-request alarms.
- **`apigw`** — API Gateways with this WebACL attached.
- **`cf`** — CloudFront distributions with this WebACL attached.
- **`ct-events`** — Audit trail for ACL rule changes.
- **`elb`** — ALBs with this WebACL attached.
- **`logs`** — Logging configuration → CW Logs.

## Explicitly excluded

> **Do not re-add.** These 58 parent→related pairs were audited by five
> independent DevOps reviewers and found to have no implementable linkage in
> the AWS API surface (beyond heuristic reverse-scans that would lie to users
> with false positives or silent zeros). See
> [related-panel-devops-consensus.md](./historical/019-related-panel/related-panel-devops-consensus.md)
> for the evidence trail. Re-adding any of these pairs requires new AWS API
> evidence cited per the Policy section at the top of this file.

### Unanimous `no` (13)

- `codeartifact` → `acm` — no ACM integration for CodeArtifact domains/repositories.
- `codeartifact` → `kinesis` — no Kinesis integration.
- `codeartifact` → `lambda` — no direct Lambda integration with CodeArtifact.
- `codeartifact` → `logs` — no native CloudWatch Logs integration; CloudTrail data events only.
- `codeartifact` → `r53` — endpoints are AWS-managed; no Route 53 records required.
- `codeartifact` → `waf` — endpoints are not WAF-protectable.
- `ddb` → `secrets` — DynamoDB has no direct Secrets Manager association; any usage is app-level.
- `eip` → `kms` — Elastic IPs have no KMS association.
- `lambda` → `asg` — Lambda functions don't reference Auto Scaling Groups.
- `secrets` → `ecr` — no direct linkage between a secret and an ECR repository.
- `secrets` → `s3` — no direct linkage between a secret and an S3 bucket.
- `tg` → `kms` — target groups have no IAM-key or KMS attribute.
- `tg` → `secrets` — target groups have no secret attribute.

### Unanimous `sometimes` — no first-class AWS field (41)

- `alarm` → `role` — no direct role field on alarms; any linkage is indirect via action ARNs (SSM automation, SNS-subscribed Lambda).
- `asg` → `cfn` — no direct CFN field; recovery is tag-heuristic only (`aws:cloudformation:stack-name`).
- `backup` → `eb-rule` — only reverse scan: iterate EventBridge rules for `source: aws.backup` pattern.
- `backup` → `logs` — no direct Backup→Logs API; CloudTrail-mediated at best.
- `codeartifact` → `cb` — no direct API; requires scanning CodeBuild projects' buildspecs/env vars for domain references.
- `codeartifact` → `role` — only via `GetDomain/RepositoryPermissionsPolicy` parse (indirect).
- `ecr` → `ecs` — no first-class ECR→ECS-cluster API; use `ecr` → `ecs-task` for the actual linkage via task definitions.
- `ecr` → `eks` — no AWS API; requires Kubernetes API access per cluster.
- `ecs-svc` → `cf` — no direct ECS service → CloudFront link.
- `ecs-svc` → `r53` — service discovery registries are indirect.
- `ecs-task` → `kms` — no direct KMS reference on a task; indirect via execution role + log group encryption.
- `eks` → `acm` — no direct cert attachment on an EKS cluster.
- `eks` → `ecr` — image resolution lives in Kubernetes, not the EKS API.
- `eks` → `iam-user` — `aws-auth` ConfigMap resolution lives in the cluster, not the EKS API.
- `elb` → `logs` — access logs go to S3 by default; CW Logs linkage is via attributes, not first-class.
- `iam-user` → `kms` — no direct key-user attribute on a user.
- `iam-user` → `role` — indirect via trust policies across all roles; reverse scan only.
- `kinesis` → `eb-rule` — reverse scan of EventBridge rules.
- `kinesis` → `logs` — indirect via subscription filters / Firehose.
- `lambda` → `ec2` — no direct EC2 link on a Lambda function.
- `lambda` → `elb` — only via ALB target group (`tg`), not a direct ELB attribute.
- `lambda` → `sfn` — no direct SFN attribute on a function.
- `ng` → `kms` — no direct KMS field on a node group.
- `opensearch` → `role` — advanced-security master user is a policy pivot, not a role field.
- `pipeline` → `logs` — execution logs go to CloudTrail events, not a first-class log group.
- `role` → `kms` — no direct KMS attribute on a role; indirect via attached policies.
- `secrets` → `pipeline` — no direct CodePipeline linkage.
- `ses` → `acm` — SES uses DKIM, not ACM, for domain identities.
- `ses` → `alarm` — alarms are general reverse-scan of CloudWatch alarms with SES dimensions.
- `ses` → `cfn` — tag-heuristic only.
- `ses` → `kms` — configuration set / identity encryption is AWS-managed by default.
- `ses` → `logs` — event destinations go to Firehose/SNS/EventBridge, not CW Logs directly.
- `ses` → `role` — role usage is embedded in receipt-rule actions / Firehose destinations.
- `ses` → `trail` — CloudTrail data events link is indirect.
- `sfn` → `cfn` — tag-heuristic only.
- `sns` → `cfn` — tag-heuristic only.
- `sns-sub` → `kms` — subscription-level encryption is topic-level, not subscription-level.
- `sns-sub` → `policy` — subscription policies are attributes, not standalone policies.
- `sqs` → `role` — no direct role on a queue; indirect via queue policy.
- `tgw` → `cfn` — tag-heuristic only.
- `waf` → `role` — WAF logging role is embedded in Firehose destination.

### Majority `no` / 0 yes (4)

- `ddb` → `sns` — no direct DDB→SNS API; event notifications go via Streams + Lambda or EventBridge Pipes.
- `lambda` → `r53` — no native linkage; custom domains go via API Gateway / CloudFront.
- `sns-sub` → `ecs` — SNS subscriptions don't target ECS clusters/services directly.
- `tg` → `role` — no IAM role attribute on target groups.
