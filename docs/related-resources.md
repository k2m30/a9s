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
   Its checker is `ctEventsCheckerFor(<shortName>)` for every type, and the
   lookup it defers to is built from the type's `CloudTrailKey` and
   `CloudTrailRegion` — the same filter the `t` hotkey sends. A per-type
   CloudTrail checker is a second way to name the same events and must not be
   registered. CloudTrail records a resource under its bare name or under its
   ARN per API call, and an account can hold both spellings for one resource:
   a lookup whose first page is empty is retried once under the other
   spelling, and the list then holds what the answering spelling holds. Where
   both occur in one account the list is a subset. A type whose name is unique
   only under a parent also declares a `CloudTrailQualifier` — the row field
   holding the parent and the event JSON paths that record it — and an event
   whose body names a different parent is dropped; one that names none is
   kept, as `AlarmMatchSpec.QualifierDimension` treats an alarm carrying no
   qualifier dimension.
5. **Never bypass** — do NOT "temporarily" remove a row to unblock a refactor.
   Previous drift happened exactly this way. If the registration is blocking
   you, fix the registration, not the contract.
6. **Error contract** — every checker MUST set `RelatedCheckResult.Err` on API
   failure, using `aws.AggregateFailures(opName, failures, total)` for
   per-item loops. Silent skip (`if err != nil { continue }` without
   aggregation) is banned — see `.claude/skills/a9s-implement-resource/SKILL.md`
   rules **E1–E6**. The app layer converts a non-nil `Result.Err` into a
   `FlashMsg{IsError:true}` so the `!` error log captures the failure;
   without this the pivot renders `?` with no actionable cause. Op-name
   convention: `"<short>-related: <Verb>"` (e.g. `"s3-related: GetBucketPolicy"`).
7. **Call budget** — a checker runs on every detail open, so it gets AT MOST
   one extra AWS API beyond reading the already-loaded sibling caches
   (`resource.ResourceCache`); per-item fan-outs over the OPEN resource's own
   sub-objects are inside the budget, fan-outs over the TARGET type's whole
   population are not — except where AWS offers no reverse read of the
   relation, and then at most `EnrichmentCap` (50) calls, one per target row,
   with the rows past the cap making the count a lower bound `(N+)`. Three
   pivots read that way: `ami` → `asg` (each group's launch sources: no API
   lists the groups that launch an image), `sqs` → `eb-rule` (`ListTargetsByRule`
   per rule: [`ListRuleNamesByTarget`](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_ListRuleNamesByTarget.html)
   matches a target's ARN, not its `DeadLetterConfig`) and `ec2`/`lambda` →
   `tg` ([`DescribeTargetHealth`](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeTargetHealth.html)
   requires one `TargetGroupArn`). A paginated API is walked page by page through
   `aws.PageAll`, up to `PerParentPageCap` pages; a walk the cap stopped is a
   lower bound `(N+)`, never an exact count. A mechanism that cannot resolve within that budget on
   ANY cache state is NOT REGISTERED — it is documented under
   **Explicitly excluded** below instead of shipping a checker that never
   produces a real answer. A permanently-unknowable row (never drillable, never
   countable) is noise, not a decision the operator can act on. How a checker's
   result renders — exact `(N)`, lower-bound `(N+)`/`(0+)`, dimmed proven-zero
   `(0)`, a blank navigable "drill in" row, or a dimmed error dead-end — is
   defined once in
   [`related-resources-engine.md`](related-resources-engine.md); the retired
   `Count: -1` sentinel and the `(?)` badge no longer exist and MUST NOT be
   produced. A blank, navigable transient-unknown row (a registered, computable
   checker whose backing sibling cache is cold) is legitimate; an
   always-unresolvable pivot is a defect. Lifting a pivot back into the registry
   requires the same evidence bar as rule 1.

## Per-type contract

| Type | AWS API | Expected related targets |
|------|---------|--------------------------|
| `acm` | [API_CertificateDetail](https://docs.aws.amazon.com/acm/latest/APIReference/API_CertificateDetail.html) | `apigw`, `cf`, `ct-events`, `elb`, `r53` |
| `alarm` | [API_MetricAlarm](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricAlarm.html) | `apigw`, `asg`, `cb`, `ct-events`, `dbi`, `ec2`, `ecs`, `eks`, `kms`, `lambda`, `logs`, `s3`, `sfn`, `sns`, `waf` |
| `ami` | [API_Image](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Image.html) | `asg`, `cfn`, `ct-events`, `ebs-snap`, `ec2`, `kms`, `ng` |
| `apigw` | [apis](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis.html) | `acm`, `alarm`, `cf`, `ct-events`, `elb`, `kms`, `lambda`, `logs`, `role` |
| `asg` | [API_AutoScalingGroup](https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_AutoScalingGroup.html) | `alarm`, `ami`, `ct-events`, `ec2`, `elb`, `ng`, `role`, `sg`, `sns`, `subnet`, `tg`, `vpc` |
| `athena` | [API_WorkGroup](https://docs.aws.amazon.com/athena/latest/APIReference/API_WorkGroup.html) | `ct-events`, `kms`, `logs`, `role`, `s3` |
| `backup` | [API_BackupPlan](https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupPlan.html) | `ct-events`, `kms`, `role`, `sns` |
| `cb` | [API_Project](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_Project.html) | `alarm`, `ct-events`, `ecr`, `kms`, `logs`, `pipeline`, `role`, `s3`, `secrets`, `sg`, `ssm`, `subnet`, `vpc` |
| `cf` | [API_Distribution](https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_Distribution.html) | `acm`, `alarm`, `ct-events`, `elb`, `lambda`, `r53`, `s3`, `waf` |
| `cfn` | [API_Stack](https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html) | `cfn`, `ct-events`, `eb-rule`, `role`, `s3`, `sns` |
| `codeartifact` | [API_Repository](https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_Repository.html) | `ct-events`, `kms` |
| `ct-events` | [API_LookupEvents](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html) | `cfn`, `ct-events`, `dbi`, `ddb`, `ec2`, `ecr`, `iam-user`, `kms`, `lambda`, `role`, `s3`, `secrets`, `sg`, `trail`, `vpce` |
| `dbc` | [API_DBCluster](https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBCluster.html) | `alarm`, `ct-events`, `dbi`, `dbc-snap`, `kms`, `logs`, `secrets`, `sg`, `subnet`, `vpc` |
| `dbi` | [API_DBInstance](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html) | `alarm`, `ct-events`, `dbc`, `eni`, `kms`, `logs`, `dbi-snap`, `role`, `secrets`, `sg`, `subnet`, `vpc` |
| `ddb` | [API_TableDescription](https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_TableDescription.html) | `alarm`, `backup`, `ct-events`, `kinesis`, `kms`, `lambda`, `vpce` |
| `dbc-snap` | [API_DBClusterSnapshot](https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBClusterSnapshot.html) | `backup`, `ct-events`, `dbc`, `kms`, `vpc` |
| `eb` | [API_EnvironmentDescription](https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html) | `alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `elb`, `logs`, `role`, `s3`, `sg`, `tg` |
| `eb-rule` | [API_Rule](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_Rule.html) | `ct-events`, `kinesis`, `lambda`, `logs`, `role`, `sfn`, `sns`, `sqs` |
| `ebs` | [API_Volume](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Volume.html) | `alarm`, `backup`, `cfn`, `ct-events`, `ebs-snap`, `ec2`, `kms` |
| `ebs-snap` | [API_Snapshot](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Snapshot.html) | `ami`, `backup`, `ct-events`, `ebs`, `ec2`, `kms` |
| `ec2` | [API_Instance](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html) | `alarm`, `ami`, `asg`, `backup`, `cfn`, `ct-events`, `ebs`, `ebs-snap`, `eip`, `eni`, `kms`, `logs`, `ng`, `role`, `sg`, `subnet`, `tg`, `vpc` |
| `ecr` | [API_Repository](https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_Repository.html) | `cb`, `cfn`, `ct-events`, `eb-rule`, `ecs-task`, `kms`, `lambda`, `pipeline`, `role` |
| `ecs` | [API_Cluster](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Cluster.html) | `alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `ecs-svc`, `ecs-task`, `kms`, `logs` |
| `ecs-svc` | [API_Service](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html) | `alarm`, `cfn`, `ct-events`, `eb-rule`, `ecr`, `ecs`, `ecs-task`, `elb`, `logs`, `role`, `secrets`, `sfn`, `sg`, `subnet`, `tg`, `vpc` |
| `ecs-task` | [API_Task](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Task.html) | `alarm`, `ct-events`, `ec2`, `ecr`, `ecs`, `ecs-svc`, `eni`, `logs`, `role`, `secrets`, `sg`, `ssm`, `subnet` |
| `efs` | [API_FileSystemDescription](https://docs.aws.amazon.com/efs/latest/ug/API_FileSystemDescription.html) | `alarm`, `backup`, `cfn`, `ct-events`, `ecs-task`, `eni`, `kms`, `lambda`, `sg`, `subnet`, `vpc` |
| `eip` | [API_Address](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html) | `asg`, `cfn`, `ct-events`, `ec2`, `ecs`, `ecs-svc`, `ecs-task`, `eni`, `nat` |
| `eks` | [API_Cluster](https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html) | `alarm`, `ami`, `asg`, `cfn`, `ct-events`, `ec2`, `kms`, `logs`, `ng`, `role`, `sg`, `subnet`, `vpc` |
| `elb` | [API_LoadBalancer](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancer.html) | `acm`, `alarm`, `cf`, `cfn`, `ct-events`, `eni`, `s3`, `sg`, `subnet`, `tg`, `vpc`, `waf` |
| `eni` | [API_NetworkInterface](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NetworkInterface.html) | `ct-events`, `ec2`, `eip`, `elb`, `lambda`, `nat`, `sg`, `subnet`, `vpc`, `vpce` |
| `glue` | [API_Job](https://docs.aws.amazon.com/glue/latest/webapi/API_Job.html) | `alarm`, `cfn`, `ct-events`, `kms`, `logs`, `role`, `s3`, `secrets` |
| `iam-group` | [API_Group](https://docs.aws.amazon.com/IAM/latest/APIReference/API_Group.html) | `ct-events`, `iam-user`, `policy` |
| `iam-user` | [API_User](https://docs.aws.amazon.com/IAM/latest/APIReference/API_User.html) | `ct-events`, `iam-group`, `policy` |
| `igw` | [API_InternetGateway](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InternetGateway.html) | `ct-events`, `rtb`, `vpc` |
| `kinesis` | [API_StreamDescription](https://docs.aws.amazon.com/kinesis/latest/APIReference/API_StreamDescription.html) | `alarm`, `cfn`, `ct-events`, `ddb`, `kms`, `lambda` |
| `kms` | [API_KeyMetadata](https://docs.aws.amazon.com/kms/latest/APIReference/API_KeyMetadata.html) | `ct-events`, `dbi`, `ebs`, `role`, `secrets` |
| `lambda` | [API_FunctionConfiguration](https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html) | `alarm`, `apigw`, `cf`, `cfn`, `ct-events`, `ddb`, `eb-rule`, `ecr`, `efs`, `eni`, `kinesis`, `kms`, `logs`, `msk`, `role`, `s3`, `secrets`, `sg`, `sns`, `sns-sub`, `sqs`, `ssm`, `subnet`, `tg`, `vpc` |
| `logs` | [API_LogGroup](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_LogGroup.html) | `alarm`, `apigw`, `ct-events`, `ecs-task`, `kinesis`, `kms`, `lambda`, `s3` |
| `lt` | [API_ResponseLaunchTemplateData](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ResponseLaunchTemplateData.html) | `ami`, `asg`, `ct-events`, `ec2`, `kms`, `ng`, `sg`, `subnet` |
| `msk` | [v1-clusters](https://docs.aws.amazon.com/msk/1.0/apireference/v1-clusters.html) | `alarm`, `cfn`, `ct-events`, `kms`, `lambda`, `logs`, `s3`, `secrets`, `sg`, `subnet`, `vpc` |
| `mwaa` | [API_Environment](https://docs.aws.amazon.com/mwaa/latest/API/API_Environment.html) | `alarm`, `ct-events`, `kms`, `logs`, `role`, `s3`, `sg`, `subnet` |
| `nat` | [API_NatGateway](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGateway.html) | `alarm`, `ct-events`, `eip`, `eni`, `rtb`, `subnet`, `vpc` |
| `ng` | [API_Nodegroup](https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html) | `ami`, `asg`, `ct-events`, `ebs`, `ec2`, `eks`, `role`, `sg`, `subnet` |
| `opensearch` | [API_DomainStatus](https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DomainStatus.html) | `acm`, `alarm`, `cfn`, `ct-events`, `kms`, `logs`, `sg`, `subnet`, `vpc` |
| `pipeline` | [API_PipelineDeclaration](https://docs.aws.amazon.com/codepipeline/latest/APIReference/API_PipelineDeclaration.html) | `cb`, `cfn`, `ct-events`, `eb-rule`, `ecr`, `ecs-svc`, `kms`, `lambda`, `role`, `s3`, `sns` |
| `policy` | [API_Policy](https://docs.aws.amazon.com/IAM/latest/APIReference/API_Policy.html) | `ct-events`, `iam-group`, `iam-user`, `role` |
| `r53` | [API_HostedZone](https://docs.aws.amazon.com/Route53/latest/APIReference/API_HostedZone.html) | `acm`, `apigw`, `cf`, `ct-events`, `elb`, `logs`, `s3`, `vpc` |
| `dbi-snap` | [API_DBSnapshot](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html) | `backup`, `ct-events`, `dbi`, `kms` |
| `redis` | [API_ReplicationGroup](https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_ReplicationGroup.html) | `alarm`, `cfn`, `ct-events`, `kms`, `logs`, `secrets`, `sg`, `sns`, `subnet`, `vpc` |
| `redshift` | [API_Cluster](https://docs.aws.amazon.com/redshift/latest/APIReference/API_Cluster.html) | `alarm`, `cfn`, `ct-events`, `kms`, `logs`, `role`, `s3`, `secrets`, `sg`, `subnet`, `vpc` |
| `role` | [API_Role](https://docs.aws.amazon.com/IAM/latest/APIReference/API_Role.html) | `ct-events`, `ec2`, `eks`, `glue`, `iam-group`, `iam-user`, `lambda`, `ng`, `policy` |
| `rtb` | [API_RouteTable](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RouteTable.html) | `cfn`, `ct-events`, `eni`, `igw`, `nat`, `subnet`, `tgw`, `vpc`, `vpce` |
| `s3` | [API_ListBuckets](https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListBuckets.html) | `athena`, `backup`, `cf`, `cfn`, `ct-events`, `eb-rule`, `glue`, `kms`, `lambda`, `r53`, `role`, `s3`, `sns`, `sqs`, `trail` |
| `secrets` | [API_SecretListEntry](https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_SecretListEntry.html) | `cb`, `cfn`, `codeartifact`, `ct-events`, `dbi`, `eb`, `ecs-task`, `kms`, `lambda`, `logs`, `role`, `sns` |
| `ses` | [API_IdentityInfo](https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_IdentityInfo.html) | `ct-events`, `eb-rule`, `lambda`, `r53`, `s3`, `sns` |
| `sfn` | [API_StateMachineListItem](https://docs.aws.amazon.com/step-functions/latest/apireference/API_StateMachineListItem.html) | `alarm`, `ct-events`, `eb-rule`, `kms`, `lambda`, `logs`, `role` |
| `sg` | [API_SecurityGroup](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_SecurityGroup.html) | `cfn`, `ct-events`, `ec2`, `elb`, `eni`, `lambda`, `sg`, `vpc` |
| `sns` | [API_Topic](https://docs.aws.amazon.com/sns/latest/api/API_Topic.html) | `alarm`, `ct-events`, `kms`, `role`, `sns-sub` |
| `sns-sub` | [API_Subscription](https://docs.aws.amazon.com/sns/latest/api/API_Subscription.html) | `ct-events`, `lambda`, `sns`, `sqs` |
| `sqs` | [API_GetQueueAttributes](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html) | `alarm`, `ct-events`, `eb-rule`, `kms`, `lambda`, `sns`, `sns-sub`, `sqs` |
| `ssm` | [API_ParameterMetadata](https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_ParameterMetadata.html) | `ct-events`, `kms` |
| `subnet` | [API_Subnet](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Subnet.html) | `asg`, `cfn`, `ct-events`, `ec2`, `efs`, `eks`, `elb`, `eni`, `nat`, `rtb`, `vpc`, `vpce` |
| `tg` | [API_TargetGroup](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_TargetGroup.html) | `alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `ecs-svc`, `elb`, `lambda`, `vpc` |
| `tgw` | [API_TransitGateway](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TransitGateway.html) | `ct-events`, `role`, `rtb`, `subnet`, `vpc` |
| `trail` | [API_Trail](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html) | `ct-events`, `kms`, `logs`, `role`, `s3`, `sns` |
| `transfer` | [API_DescribedServer](https://docs.aws.amazon.com/transfer/latest/userguide/API_DescribedServer.html) | `acm`, `ct-events`, `eip`, `lambda`, `logs`, `role`, `subnet`, `vpc`, `vpce` |
| `vpc` | [API_Vpc](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Vpc.html) | `cfn`, `ct-events`, `ec2`, `elb`, `eni`, `igw`, `nat`, `rtb`, `sg`, `subnet`, `tgw`, `vpce` |
| `vpc-peer` | [API_VpcPeeringConnection](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcPeeringConnection.html) | `ct-events`, `rtb`, `vpc` |
| `vpce` | [API_VpcEndpoint](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcEndpoint.html) | `alarm`, `ct-events`, `eni`, `logs`, `r53`, `rtb`, `sg`, `subnet`, `vpc` |
| `waf` | [API_WebACL](https://docs.aws.amazon.com/waf/latest/APIReference/API_WebACL.html) | `alarm`, `apigw`, `cf`, `ct-events`, `elb`, `logs` |

## Per-target reasoning

One entry per `(type, target)` pair. Reasoning is one line anchored to an AWS
API field (preferred) or a concrete DevOps workflow.

### `acm`

AWS API: <https://docs.aws.amazon.com/acm/latest/APIReference/API_CertificateDetail.html>

- **`apigw`** — The APIs served with this cert: `InUseBy` names an API Gateway custom domain (`/domainnames/<name>`), and the APIs are the ones that domain's API mappings name, read with `apigatewayv2:GetApiMappings` ([rest-api-mappings](https://docs.aws.amazon.com/apigateway/latest/developerguide/rest-api-mappings.html), [domainnames-domainname-apimappings](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/domainnames-domainname-apimappings.html)). A domain `GetApiMappings` maps nothing on is read again through `apigateway:GetBasePathMappings`, how an edge-optimized domain maps REST APIs ([how-to-edge-optimized-custom-domain-name](https://docs.aws.amazon.com/apigateway/latest/developerguide/how-to-edge-optimized-custom-domain-name.html), [API_GetBasePathMappings](https://docs.aws.amazon.com/apigateway/latest/api/API_GetBasePathMappings.html)). A domain's routing rules send traffic too: each rule's `InvokeApi.ApiId`, read with `apigatewayv2:ListRoutingRules`, counts beside the mappings, and the domain's `routingMode` (`API_MAPPING_ONLY`, `ROUTING_RULE_ONLY`, `ROUTING_RULE_THEN_API_MAPPING`) decides which of the two places are read; an unknown mode reads both ([rest-api-routing-mode](https://docs.aws.amazon.com/apigateway/latest/developerguide/rest-api-routing-mode.html), [domainnames-domainname-routingrules](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/domainnames-domainname-routingrules.html)).
- **`cf`** — CloudFront distributions using this cert.
- **`ct-events`** — Audit trail for cert issuance/renewal.
- **`elb`** — Application, Network and Gateway Load Balancers in the certificate's `InUseBy`; a Classic Load Balancer's ARN names no row of the `elb` list, which `DescribeLoadBalancers` (ELBv2) fills ([API_DescribeLoadBalancers](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeLoadBalancers.html)). ACM's `InUseBy` lags: a load balancer can stay listed for a while after its listener drops the certificate.
- **`r53`** — The hosted zone holding each DNS validation record (`DomainValidationOptions[].ResourceRecord.Name`): the innermost public zone whose name is the record's parent at a label boundary. A private zone never holds it — ACM validates against public DNS.

### `alarm`

AWS API: <https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricAlarm.html>

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
- **`logs`** — Metric-filter-driven alarms point at log groups: the filter that emits the metric the alarm watches names the group, read with `logs:DescribeMetricFilters`.
- **`s3`** — S3 request metrics alarm dimension.
- **`sfn`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sns`** — MetricAlarm.AlarmActions / OKActions — SNS topics notified.
- **`waf`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.

### `ami`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Image.html>

- **`asg`** — Groups whose launch sources (launch template, launch configuration, mixed-instances policy and its overrides) launch this image, and groups running instances of it.
- **`cfn`** — AMIs often consumed by CloudFormation templates.
- **`ct-events`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ebs-snap`** — AMI block devices reference EBS snapshots.
- **`ec2`** — Reverse lookup: instances using this AMI.
- **`kms`** — The keys encrypting the AMI's snapshots, `Snapshot.KmsKeyId` of each snapshot its block devices name; a block device's `Ebs.KmsKeyId` "is only supported on BlockDeviceMapping objects called by RunInstances, RequestSpotFleet, and RequestSpotInstances" ([API_EbsBlockDevice](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_EbsBlockDevice.html)).
- **`ng`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.

### `apigw`

AWS API: <https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis.html>

- **`acm`** — Custom-domain TLS certificate, from `DomainNameConfigurations`, of each domain whose API mappings, base path mappings or routing rules name this API (the `acm` → `apigw` walk).
- **`alarm`** — Stage latency/error alarms.
- **`cf`** — Distributions with an origin whose host is this API's own invoke host, `<api-id>.execute-api.<region>.amazonaws.com`, matched on the leading label.
- **`ct-events`** — Audit trail for API changes.
- **`elb`** — Load balancers behind the API's private integrations: an HTTP API's `VPC_LINK` integration `IntegrationUri` is an ALB or NLB listener ARN, which names its load balancer (a Cloud Map service ARN names none and leaves a lower bound) ([apis-apiid-integrations](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis-apiid-integrations.html)); a REST API's `VPC_LINK` integration reaches the `targetArns` of its VPC link ([API_VpcLink](https://docs.aws.amazon.com/apigateway/latest/api/API_VpcLink.html)).
- **`kms`** — KMS key referenced by Lambda integrations (weak pair: no direct API GW KMS field; follows Lambda integration FunctionConfiguration.KMSKeyArn); a REST API's integrations from `GetResources` ([API_GetResources](https://docs.aws.amazon.com/apigateway/latest/api/API_GetResources.html)).
- **`lambda`** — Lambda integrations: an HTTP or WebSocket API's `GetIntegrations`, a REST API's method integrations from `GetResources` with the `methods` embed ([API_GetResources](https://docs.aws.amazon.com/apigateway/latest/api/API_GetResources.html)).
- **`logs`** — API access log destination.
- **`role`** — Invocation/authorizer roles: integration credentials (`CredentialsArn`, or a REST method integration's `credentials`) and authorizer credentials (`AuthorizerCredentialsArn`, or a REST authorizer's `authorizerCredentials`) matched against the loaded `role` cache ([API_GetResources](https://docs.aws.amazon.com/apigateway/latest/api/API_GetResources.html)).

### `asg`

AWS API: <https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_AutoScalingGroup.html>

- **`alarm`** — Alarms that trigger scaling policies.
- **`ami`** — The image of every launch source: the launch configuration's `ImageId`, `LaunchTemplateData.ImageId` of the launch template, the mixed-instances policy's template and each `Overrides[]` template, and each override's own `ImageId` ([API_LaunchTemplateOverrides](https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_LaunchTemplateOverrides.html)).
- **`ct-events`** — Audit trail for scaling events / config changes.
- **`ec2`** — Instances the ASG currently manages; an instance whose `LifecycleState` is `Terminating*`, `Terminated` or `Detached` is leaving the group ([API_Instance](https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_Instance.html)).
- **`elb`** — TargetGroupARNs → DescribeTargetGroups.LoadBalancerArns (ALB/NLB). Classic `LoadBalancerNames` are not counted: the `elb` type holds ELBv2 load balancers only.
- **`ng`** — EKS node groups wrap ASGs; shown when parent node group exists.
- **`role`** — AutoScalingGroup.ServiceLinkedRoleARN + the instance profile of every launch source (launch configuration, launch template, mixed-instances policy and its `Overrides[]`) → GetInstanceProfile roles ([API_LaunchTemplateOverrides](https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_LaunchTemplateOverrides.html)).
- **`sg`** — The security groups of every launch source: `LaunchConfiguration.SecurityGroups`, and `SecurityGroupIds` / `NetworkInterfaces[].Groups` of each launch template including the mixed-instances policy's and its `Overrides[]` ([API_LaunchTemplateOverrides](https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_LaunchTemplateOverrides.html)).
- **`sns`** — DescribeNotificationConfigurations.TopicARN + DescribeLifecycleHooks.NotificationTargetARN (SNS-only).
- **`subnet`** — AutoScalingGroup.VPCZoneIdentifier — subnets the ASG launches into.
- **`tg`** — AutoScalingGroup.TargetGroupARNs — TGs the ASG registers instances with.
- **`vpc`** — AutoScalingGroup.VPCZoneIdentifier → DescribeSubnets.VpcId — VPC(s) the ASG operates in.

### `athena`

AWS API: <https://docs.aws.amazon.com/athena/latest/APIReference/API_WorkGroup.html>

- **`ct-events`** — Audit trail for workgroup changes.
- **`kms`** — The keys the workgroup configuration names: `ResultConfiguration.EncryptionConfiguration.KmsKey`, `ManagedQueryResultsConfiguration.EncryptionConfiguration.KmsKey` and `CustomerContentEncryptionConfiguration.KmsKey` ([API_WorkGroupConfiguration](https://docs.aws.amazon.com/athena/latest/APIReference/API_WorkGroupConfiguration.html)).
- **`logs`** — Workgroup query logs.
- **`role`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`s3`** — Query result output bucket.

### `backup`

AWS API: <https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupPlan.html>

- **`ct-events`** — Audit trail for plan/selection/job events.
- **`kms`** — Recovery-point encryption key.
- **`role`** — Backup service role used for restore jobs.
- **`sns`** — Vault notifications.

### `cb`

AWS API: <https://docs.aws.amazon.com/codebuild/latest/APIReference/API_Project.html>

- **`alarm`** — Build-failure alarms.
- **`ct-events`** — Audit trail for build events.
- **`ecr`** — The repository the build environment image belongs to (registry and repository path both equal).
- **`kms`** — EncryptionKey on artifacts.
- **`logs`** — Build log group, unless `logsConfig.cloudWatchLogs.status` is `DISABLED` ([API_CloudWatchLogsConfig](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_CloudWatchLogsConfig.html)).
- **`pipeline`** — Pipelines consuming this project.
- **`role`** — Project.ServiceRole.
- **`s3`** — Source, secondary-source and artifact buckets, and the `LogsConfig.S3Logs` bucket when its status is ENABLED ([API_ProjectSource](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_ProjectSource.html), [API_S3LogsConfig](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_S3LogsConfig.html)).
- **`secrets`** — Secrets as build env variables.
- **`sg`** — VpcConfig.SecurityGroupIds.
- **`ssm`** — SSM parameters as build env.
- **`subnet`** — VpcConfig.Subnets.
- **`vpc`** — VpcConfig.VpcId.

### `cf`

AWS API: <https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_Distribution.html>

- **`acm`** — Distribution.ViewerCertificate.AcmCertificateArn.
- **`alarm`** — Distribution error-rate alarms.
- **`ct-events`** — Audit trail for distribution changes.
- **`elb`** — Load balancers named as origins, matched on their DNS name.
- **`lambda`** — Lambda@Edge associations.
- **`r53`** — Hosted zones with an alias record targeting this distribution's domain name (the zone's enumerated `AliasTarget.DNSName` values).
- **`s3`** — S3 origins and the standard-logging bucket (`DistributionConfig.Logging.Bucket`).
- **`waf`** — Distribution.WebACLId.

### `cfn`

AWS API: <https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html>

- **`cfn`** — Nested stacks.
- **`ct-events`** — Audit trail for stack events.
- **`eb-rule`** — Stack-event publishing via EventBridge.
- **`role`** — Stack.RoleARN — stack service role.
- **`s3`** — Buckets this stack manages, from `ListStackResources` (`AWS::S3::Bucket`).
- **`sns`** — Stack.NotificationARNs — event topics.

### `codeartifact`

AWS API: <https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_Repository.html>

- **`ct-events`** — Calls naming the repository, and the package-manager requests recorded as `ReadFromRepository` with the repository only in `requestParameters.domainName` / `repositoryName` ([codeartifact-information-in-cloudtrail](https://docs.aws.amazon.com/codeartifact/latest/ug/codeartifact-information-in-cloudtrail.html)).
- **`kms`** — Domain `EncryptionKey` (resolved via `DescribeDomain` using the repo's `DomainName` + `DomainOwner`); CodeArtifact encryption is a domain-level, not repository-level, property. <!-- amended by a9s-resource-spec during codeartifact gen: AWS SDK Go v2 shows EncryptionKey lives on DomainDescription/DomainSummary, not RepositoryDescription/RepositorySummary -->

### `ct-events`

AWS API: <https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html>

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
- **`ecr`** — the `AWS::ECR::Repository` resource of an ECR record, by repository ARN or by name, else `requestParameters.repositoryName` — push, pull and repository events ([logging-using-cloudtrail](https://docs.aws.amazon.com/AmazonECR/latest/userguide/logging-using-cloudtrail.html)).
- **`cfn`** — `resources[].ARN` matching CloudFormation stack ARNs — stack lifecycle events.
- **`trail`** — `resources[].ARN` matching CloudTrail trail ARNs — trail config and status events.
- **`ct-events` (by AccessKeyId)** — Self-pivot: convenience filter within ct-events by `userIdentity.accessKeyId`.
- **`ct-events` (by Username)** — Self-pivot: convenience filter within ct-events by `userIdentity.userName`.
- **`ct-events` (by EventName)** — Self-pivot: convenience filter within ct-events by `eventName`.

### `dbc`

AWS API: <https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBCluster.html>

- **`alarm`** — Cluster CW alarms.
- **`ct-events`** — Audit trail for cluster changes.
- **`dbi`** — Cluster member instances.
- **`dbc-snap`** — Cluster snapshots.
- **`kms`** — Cluster encryption key.
- **`logs`** — Cluster log exports.
- **`secrets`** — Master credentials in Secrets Manager.
- **`sg`** — VpcSecurityGroups — cluster SGs.
- **`subnet`** — DBSubnetGroup subnets.
- **`vpc`** — DBSubnetGroup VPC.

### `dbi`

AWS API: <https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html>

- **`alarm`** — CloudWatch alarms on CPU/Storage/Connections.
- **`ct-events`** — Audit trail for DB config / modifyDBInstance.
- **`dbc`** — Aurora instance → cluster.
- **`eni`** — DB instances back onto ENIs. Heuristic: RDS-managed ENIs on the instance's security groups; an ENI records no DB instance, and another instance sharing a group shares its ENIs.
- **`kms`** — KmsKeyId — storage encryption key.
- **`logs`** — DB engine log exports (e.g. /aws/rds/instance/<id>/error).
- **`dbi-snap`** — Snapshots of this instance.
- **`role`** — MonitoringRoleArn / S3-integration role.
- **`secrets`** — Secrets Manager entries holding master credentials.
- **`sg`** — VpcSecurityGroups — SGs attached to the instance.
- **`subnet`** — DBSubnetGroup.Subnets — subnets the instance spans.
- **`vpc`** — DBSubnetGroup.VpcId.

### `ddb`

AWS API: <https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_TableDescription.html>

- **`alarm`** — Throttle/error/ReadCapacity alarms.
- **`backup`** — Backup plans whose selections name the table by ARN or select it by its tags ([working-with-supported-services](https://docs.aws.amazon.com/aws-backup/latest/devguide/working-with-supported-services.html)).
- **`ct-events`** — Audit trail for table schema/capacity changes.
- **`kinesis`** — Kinesis Data Streams destinations from `DescribeKinesisStreamingDestination`; a `DISABLED` or `ENABLE_FAILED` destination is not counted ([API_KinesisDataStreamDestination](https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_KinesisDataStreamDestination.html)).
- **`kms`** — SSEDescription.KMSMasterKeyArn — table encryption key.
- **`lambda`** — Lambdas consuming DDB Streams from this table.
- ~~**`logs`**~~ — Removed 2026-09-24. DynamoDB writes no log group: Contributor Insights for DynamoDB delivers through CloudWatch Contributor Insights rules named `DynamoDBContributorInsights-<PKC|PKT|SKC|SKT>-<table>-<timestamp>` (<https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/contributorinsights_HowItWorks.html>), so a log group carrying a table's name is not the table's.
- **`vpce`** — Gateway endpoint for DynamoDB.

### `dbc-snap`

AWS API: <https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBClusterSnapshot.html>

- **`backup`** — Backup plans whose selections name the snapshot's source cluster by ARN or select it by its tags ([working-with-supported-services](https://docs.aws.amazon.com/aws-backup/latest/devguide/working-with-supported-services.html)).
- **`ct-events`** — Audit trail for snapshot events.
- **`dbc`** — Source cluster.
- **`kms`** — Encryption key.
- **`vpc`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.

### `eb`

AWS API: <https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html>

- **`alarm`** — Health alarms.
- **`asg`** — Environment's backing ASG (elasticbeanstalk:environment-name tag on ASG).
- **`cfn`** — Beanstalk creates a CloudFormation stack per environment (awseb-{envId}-stack prefix).
- **`ct-events`** — Audit trail for environment config changes.
- **`ec2`** — Instances running the environment (elasticbeanstalk:environment-name tag on EC2 instances); a `shutting-down` or `terminated` instance runs nothing ([ec2-instance-lifecycle](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-lifecycle.html)).
- **`elb`** — DescribeEnvironmentResources.EnvironmentResources.LoadBalancers[].Name — ELB(s) fronting this environment.
- **`logs`** — Log groups prefixed /aws/elasticbeanstalk/{envName}/.
- **`role`** — DescribeConfigurationSettings OptionSettings: aws:autoscaling:launchconfiguration/IamInstanceProfile → GetInstanceProfile roles; aws:elasticbeanstalk:environment/ServiceRole; and `EnvironmentDescription.OperationsRole` ([API_EnvironmentDescription](https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html)).
- **`s3`** — The bucket of the source bundle this environment runs: `SourceBundle.S3Bucket` of the application version its `VersionLabel` names ([API_EnvironmentDescription](https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html)).
- **`sg`** — DescribeConfigurationSettings OptionSettings: aws:autoscaling:launchconfiguration/SecurityGroups and aws:elbv2:loadbalancer/SecurityGroups.
- **`tg`** — DescribeEnvironmentResources.LoadBalancers[].Name → elbv2:DescribeListeners → DefaultActions/ForwardConfig TargetGroupArn.

### `eb-rule`

AWS API: <https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_Rule.html>

- **`ct-events`** — Audit trail for rule changes.
- **`kinesis`** — Rule → Kinesis target.
- **`lambda`** — Lambda targets of this rule.
- **`logs`** — Rule → CW Logs target.
- **`role`** — Rule.RoleArn — IAM role used for target invocation.
- **`sfn`** — Step Functions state-machine targets.
- **`sns`** — SNS targets of this rule.
- **`sqs`** — SQS targets of this rule, and each target's `DeadLetterConfig.Arn` queue ([API_DeadLetterConfig](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_DeadLetterConfig.html)).

### `ebs`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Volume.html>

- **`alarm`** — Volume CW alarms (throughput/IOPS).
- **`backup`** — Backup plans selecting the volume, or the instance it is attached to, by ARN or by tags: an EC2 backup includes its attached volumes ([working-with-supported-services](https://docs.aws.amazon.com/aws-backup/latest/devguide/working-with-supported-services.html)).
- **`cfn`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ct-events`** — Audit trail for volume changes.
- **`ebs-snap`** — Snapshots of this volume.
- **`ec2`** — Every instance in `Volume.Attachments[].InstanceId` — Multi-Attach puts one io1/io2 volume on up to 16 at once.
- **`kms`** — Volume.KmsKeyId — at-rest encryption key.

### `ebs-snap`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Snapshot.html>

- **`ami`** — AMIs derived from this snapshot.
- **`backup`** — Snapshots covered by AWS Backup.
- **`ct-events`** — Audit trail for snapshot events.
- **`ebs`** — Source volume.
- **`ec2`** — The instance `CreateImage` named in `Snapshot.Description`.
- **`kms`** — Snapshot encryption key.

### `ec2`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html>

- **`alarm`** — CloudWatch alarms watching this instance — first signal of impact.
- **`ami`** — Instance.ImageId — provenance of the running image; compare against latest approved AMI.
- **`asg`** — ASG that owns the instance (if any) — lifecycle context; a group no longer counts an instance whose `LifecycleState` is `Terminating*`, `Terminated` or `Detached` ([API_Instance](https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_Instance.html)).
- **`backup`** — Instances covered by AWS Backup.
- **`cfn`** — CloudFormation stack that created it — infra-as-code linkage.
- **`ct-events`** — Audit trail for all API calls touching this instance.
- **`ebs`** — Instance.BlockDeviceMappings[].Ebs.VolumeId — attached storage; capacity/IOPS troubleshooting.
- **`ebs-snap`** — Snapshots of the volumes this instance has attached, by `Snapshot.VolumeId`, and the snapshots its AMI's block devices name in `Ebs.SnapshotId` ([API_EbsBlockDevice](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_EbsBlockDevice.html)) — rollback/forensic workflows.
- **`eip`** — Addresses associated with the instance; traffic attribution.
- **`eni`** — Instance.NetworkInterfaces[] — ENIs for multi-homed or secondary interfaces.
- **`kms`** — Instance-attached volume encryption keys.
- **`logs`** — Candidates: the log groups whose name carries this instance's id, the CloudWatch agent's convention. No API records which groups an instance writes to, so the row offers candidates and shows no count.
- **`ng`** — Nodegroup owning this instance.
- **`role`** — IamInstanceProfile → role — permissions the instance operates with.
- **`sg`** — Instance.SecurityGroups[] — ingress/egress rules; first stop for connectivity issues.
- **`subnet`** — Instance.SubnetId — primary ENI's subnet; used when diagnosing placement/routing.
- **`tg`** — Instance target groups in this instance's VPC (`TargetGroup.VpcId`) whose `DescribeTargetHealth` lists this instance ([API_DescribeTargetHealth](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeTargetHealth.html)).
- **`vpc`** — Instance.VpcId — network parent; pivoted to for VPC-wide troubleshooting.

### `ecr`

AWS API: <https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_Repository.html>

- **`cb`** — CodeBuild projects whose build environment image belongs to this repository (registry and repository path both equal).
- **`cfn`** — CloudFormation stack that created the repo.
- **`ct-events`** — Audit trail for image push/pull, policy changes.
- **`eb-rule`** — Image-scan EventBridge events.
<!-- amended by a9s-resource-spec during ecr gen: removed stale `ecs` bullet — contradicts the per-type contract row (line 66) and the explicit non-match at line 1092 (`ecr → ecs` has no first-class API; use `ecr → ecs-task`). -->
- **`ecs-task`** — Running tasks whose container images belong to this repository (registry and repository path both equal).
- **`kms`** — EncryptionConfiguration.KmsKey.
- **`lambda`** — Container-packaged functions whose image belongs to this repository, read per function through `lambda:GetFunction` (`Code.ImageUri`).
- **`pipeline`** — Pipelines pushing to this repo.
- **`role`** — Pull/push IAM roles.

### `ecs`

AWS API: <https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Cluster.html>

- **`alarm`** — Cluster-level alarms on resource utilization.
- **`asg`** — The Auto Scaling groups of the cluster's capacity providers: `DescribeCapacityProviders` over `Cluster.CapacityProviders`, each one's `AutoScalingGroupProvider.AutoScalingGroupArn` ([API_AutoScalingGroupProvider](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_AutoScalingGroupProvider.html)).
- **`cfn`** — CloudFormation stack that created the cluster.
- **`ct-events`** — Audit trail for cluster config changes.
- **`ec2`** — The EC2 instances registered as the cluster's container instances: `ListContainerInstances`, then `DescribeContainerInstances` for each one's `ec2InstanceId`, leaving out a `REGISTRATION_FAILED`, `DEREGISTERING` or `INACTIVE` instance ([API_ContainerInstance](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerInstance.html)).
- **`ecs-svc`** — Services running on this cluster.
- **`ecs-task`** — Tasks running in this cluster.
- **`kms`** — ExecuteCommandConfiguration.KmsKeyId.
- **`logs`** — `Cluster.Configuration.ExecuteCommandConfiguration.LogConfiguration.CloudWatchLogGroupName` — the log group receiving this cluster's `ecs exec` session transcripts, the one log group a Cluster names, used when `ExecuteCommandConfiguration.Logging` is `OVERRIDE` ([API_ExecuteCommandConfiguration](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ExecuteCommandConfiguration.html)). Counted 0 or 1.

### `ecs-svc`

AWS API: <https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html>

- **`alarm`** — Service alarms (CPU/Memory/PendingTasks).
- **`cfn`** — CloudFormation stack that created the service.
- **`ct-events`** — Audit trail for service changes.
- **`eb-rule`** — Scheduled tasks are EB-driven.
- **`ecr`** — The repositories the images of the service's task definition belong to (registry and repository path both equal).
- **`ecs`** — Parent cluster.
- **`ecs-task`** — Running tasks for this service.
- **`elb`** — Load balancer fronting the service (via TG).
- **`logs`** — The log groups the containers of the service's task definition write to: `ContainerDefinitions[].LogConfiguration.Options["awslogs-group"]`, and for an `awsfirelens` container the Fluent Bit CloudWatch output's `log_group_name` ([firelens-taskdef](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/firelens-taskdef.html), [Fluent Bit cloudwatch_logs](https://docs.fluentbit.io/manual/data-pipeline/outputs/cloudwatch)), read with one `ecs:DescribeTaskDefinition` per family. A FireLens container routed by a config file or a `log_group_template` leaves a lower bound. A definition that cannot be read leaves the groups whose name carries the family as candidates, without a count.
- **`role`** — `Service.RoleArn`, and the task definition's `taskRoleArn` and `executionRoleArn` ([API_TaskDefinition](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_TaskDefinition.html)).
- **`secrets`** — Secrets Manager secrets the task definition references: container `secrets[].valueFrom` and `logConfiguration.secretOptions[].valueFrom` by ARN, a bare name there being an SSM parameter ([API_Secret](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Secret.html)), and `repositoryCredentials.credentialsParameter` by ARN or, in the task's Region, by name ([API_RepositoryCredentials](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RepositoryCredentials.html)).
- **`sfn`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sg`** — AwsvpcConfiguration.SecurityGroups.
- **`subnet`** — AwsvpcConfiguration.Subnets.
- **`tg`** — Service.LoadBalancers[].TargetGroupArn — target groups.
- **`vpc`** — AwsvpcConfiguration subnets imply VPC parent.

### `ecs-task`

AWS API: <https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Task.html>

- **`alarm`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ct-events`** — Audit trail for task start/stop events.
- **`ec2`** — Container-instance EC2.
- **`ecr`** — The repositories the task's container images belong to (registry and repository path both equal).
- **`ecs`** — Parent cluster.
- **`ecs-svc`** — Owning service (Task.Group = 'service:<name>').
- **`eni`** — Task ENI (awsvpc mode).
- **`logs`** — The log groups the containers of the task's task definition write to: `ContainerDefinitions[].LogConfiguration.Options["awslogs-group"]`, and for an `awsfirelens` container the Fluent Bit CloudWatch output's `log_group_name` ([firelens-taskdef](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/firelens-taskdef.html), [Fluent Bit cloudwatch_logs](https://docs.fluentbit.io/manual/data-pipeline/outputs/cloudwatch)), read with one `ecs:DescribeTaskDefinition` per family. A FireLens container routed by a config file or a `log_group_template` leaves a lower bound. A definition that cannot be read leaves the groups whose name carries the family as candidates, without a count.
- **`role`** — Task / execution role.
- **`secrets`** — Secrets Manager secrets the task definition references: container `secrets[].valueFrom` and `logConfiguration.secretOptions[].valueFrom` by ARN, a bare name there being an SSM parameter ([API_Secret](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Secret.html)), and `repositoryCredentials.credentialsParameter` by ARN or, in the task's Region, by name ([API_RepositoryCredentials](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RepositoryCredentials.html)).
- **`sg`** — Task ENI SGs.
- **`ssm`** — SSM parameters the task definition references in `secrets[].valueFrom`, by ARN or by name, resolved against the loaded parameter list, so a parameter in another region or account is not a local one ([API_Secret](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Secret.html)).
- **`subnet`** — Task ENI subnet.

### `efs`

AWS API: <https://docs.aws.amazon.com/efs/latest/ug/API_FileSystemDescription.html>

- **`alarm`** — BurstCreditBalance / PercentIOLimit alarms.
- **`backup`** — AWS Backup recovery points.
- **`cfn`** — CloudFormation stack that created the FS.
- **`ct-events`** — Audit trail for FS changes.
<!-- `ec2` pivot removed (2026-04-24): AWS exposes no API edge from an EC2
instance to the EFS filesystems it mounts. Mount-target ENIs are
RequesterManaged with no Attachment.InstanceId; mounting happens at the
guest OS layer via DNS. Any registered checker can only return Count=0,
which is a U9 violation. Operators can still correlate via subnet / VPC /
sg pivots, which remain registered. -->
- **`ecs-task`** — ECS tasks mounting EFS via `TaskDefinition.Volumes[].EfsVolumeConfiguration.FileSystemId`. The ecs-task fetcher joins DescribeTaskDefinition and surfaces the joined ids on `Resource.Fields["efs_file_system_ids"]` (comma-separated) so `checkEFSECSTask` can reverse-scan.
- **`eni`** — Mount-target ENIs.
- **`kms`** — FileSystemDescription.KmsKeyId.
- **`lambda`** — Lambdas mounting this file system.
- **`sg`** — MountTarget security groups.
- **`subnet`** — The subnets of this file system's mount-target ENIs, read from each ENI's description.
- **`vpc`** — Mount targets live in a VPC.

### `eip`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html>

- ~~**`alarm`**~~ — Removed 2026-09-24. No CloudWatch metric is keyed by an Elastic IP or its network interface: `AWS/EC2` dimensions its metrics by `AutoScalingGroupName`, `ImageId`, `InstanceId` and `InstanceType` only (<https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/viewing_metrics_with_cloudwatch.html#ec2-cloudwatch-dimensions>; the traffic-mirroring metrics use the same four, <https://docs.aws.amazon.com/vpc/latest/mirroring/traffic-mirror-cloudwatch.html>), so no alarm belongs to an address. An alarm on the instance behind the address is on the instance's own `alarm` pivot.
- **`asg`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`cfn`** — CFN stack that created the EIP.
- **`ct-events`** — Audit trail for allocation/association.
- **`ec2`** — Associated instance.
- **`ecs`** — Cluster of the task whose ENI carries this EIP — zero-call join: `Address.NetworkInterfaceId` matched against task ENI attachments in the already-loaded `ecs-task` cache, then `clusterArn` to the `ecs` cache.
- **`ecs-svc`** — Service owning the task whose ENI carries this EIP — zero-call join via the same `ecs-task` cache match, then the task's `service:` group to the `ecs-svc` cache.
- **`ecs-task`** — Task whose ENI carries this EIP — zero-call join: `Address.NetworkInterfaceId` matched against task ENI attachments in the already-loaded `ecs-task` cache.
- **`eni`** — Associated ENI (`Address.NetworkInterfaceId`).
- **`nat`** — NAT gateway consuming this EIP, through the gateway's live addresses (the `nat` → `eip` rule).

### `eks`

AWS API: <https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html>

- **`alarm`** — CloudWatch alarms on cluster/control-plane metrics.
- **`ami`** — AMIs applied to worker nodes.
- **`asg`** — Backing ASG.
- **`cfn`** — CloudFormation stack that created the cluster.
- **`ct-events`** — Audit trail for cluster config changes.
- **`ec2`** — Worker-node instances; a `shutting-down` or `terminated` instance is no worker ([ec2-instance-lifecycle](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-lifecycle.html)).
- **`kms`** — EncryptionConfig.Provider.KeyArn.
- **`logs`** — Control-plane log groups /aws/eks/<cluster>/cluster.
- **`ng`** — Node groups attached to the cluster.
- **`role`** — Cluster.RoleArn — EKS service role — and an Auto Mode cluster's `ComputeConfig.NodeRoleArn`, the role its nodes run as ([API_ComputeConfigResponse](https://docs.aws.amazon.com/eks/latest/APIReference/API_ComputeConfigResponse.html)).
- **`sg`** — Cluster.ResourcesVpcConfig.ClusterSecurityGroupId + additional SGs.
- **`subnet`** — Cluster.ResourcesVpcConfig.SubnetIds — cluster subnets.
- **`vpc`** — Cluster.ResourcesVpcConfig.VpcId — cluster's VPC.

### `elb`

AWS API: <https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancer.html>

- **`acm`** — Every certificate the HTTPS and TLS listeners serve: the default in `Listener.Certificates` and the SNI certificates `DescribeListenerCertificates` answers with.
- **`alarm`** — CloudWatch alarms on LB metrics (4xx/5xx/latency).
- **`cf`** — Distributions naming this load balancer's DNS name as an origin.
- **`cfn`** — CloudFormation stack that created the LB.
- **`ct-events`** — Audit trail for LB config changes.
- **`eni`** — LB creates ENIs per AZ.
- **`s3`** — Access-log S3 destination, `access_logs.s3.bucket`, when `access_logs.s3.enabled` is `true` ([API_LoadBalancerAttribute](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancerAttribute.html)).
- **`sg`** — Attached security groups (ALB only).
- **`subnet`** — AZ subnets the LB listens in.
- **`tg`** — Target groups attached to this LB.
- **`vpc`** — LoadBalancer.VpcId.
- **`waf`** — WebACL associated with ALB.

### `eni`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NetworkInterface.html>

- **`ct-events`** — Audit trail for ENI attach/detach.
- **`ec2`** — Attached instance, from `Attachment.InstanceId` (if any).
- **`eip`** — Associated Elastic IPs: the `AllocationId` of the association on the primary private address and on each secondary one.
- **`elb`** — ELB creates ENIs.
- **`lambda`** — The functions that use this Hyperplane ENI: `InterfaceType` `lambda`, in one of the function's subnets, with exactly the function's security groups — Lambda shares one ENI among the functions of a subnet and security-group combination ([configuration-vpc](https://docs.aws.amazon.com/lambda/latest/dg/configuration-vpc.html)).
- **`nat`** — NAT gateway backing ENI, through the gateway's live addresses (the `nat` → `eip` rule).
- **`sg`** — Attached security groups.
- **`subnet`** — ENI's subnet.
- **`vpc`** — Parent VPC.
- **`vpce`** — Interface endpoint ENIs.

### `glue`

AWS API: <https://docs.aws.amazon.com/glue/latest/webapi/API_Job.html>

- **`alarm`** — Job-run failure alarms.
- **`cfn`** — CloudFormation stack that created the job.
- **`ct-events`** — Audit trail for job events.
- **`kms`** — Data + bookmark encryption key.
- **`logs`** — The job's output and error groups, and its continuous-logging group: `--continuous-log-logGroup`, `/aws-glue/jobs/logs-v2` when unset ([monitor-continuous-logging-enable](https://docs.aws.amazon.com/glue/latest/dg/monitor-continuous-logging-enable.html)).
- **`role`** — Job.Role.
- **`s3`** — The bucket of the job's script, `Command.ScriptLocation` ([API_JobCommand](https://docs.aws.amazon.com/glue/latest/webapi/API_JobCommand.html)).
- **`secrets`** — The secret each of the job's connections names in `ConnectionProperties.SECRET_ID`, read with `GetConnection` ([API_Connection](https://docs.aws.amazon.com/glue/latest/webapi/API_Connection.html)).

### `iam-group`

AWS API: <https://docs.aws.amazon.com/IAM/latest/APIReference/API_Group.html>

- **`ct-events`** — Audit trail for group membership changes.
- **`iam-user`** — Members of this group, from `GetGroup`.
- **`policy`** — Attached managed policies, from `ListAttachedGroupPolicies`, and the group's own inline policies.

### `iam-user`

AWS API: <https://docs.aws.amazon.com/IAM/latest/APIReference/API_User.html>

- **`ct-events`** — The user's own calls: a `Username` lookup kept to events whose `userIdentity.type` is `IAMUser`, since the lookup also returns the calls of a role session named like the user ([userIdentity](https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-event-reference-user-identity.html)).
- **`iam-group`** — Groups the user belongs to, from `ListGroupsForUser`.
- **`policy`** — Attached managed policies.

### `igw`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InternetGateway.html>

- **`ct-events`** — Audit trail for attach/detach events.
- **`rtb`** — Route tables with a live route to this gateway; a blackhole route names no live gateway.
- **`vpc`** — Attached VPC.

### `kinesis`

AWS API: <https://docs.aws.amazon.com/kinesis/latest/APIReference/API_StreamDescription.html>

- **`alarm`** — IteratorAge / IncomingRecords alarms.
- **`cfn`** — CloudFormation stack that created the stream.
- **`ct-events`** — Audit trail for stream changes.
- **`ddb`** — Tables whose `DescribeKinesisStreamingDestination` names this stream in `KinesisDataStreamDestinations[].StreamArn`; a `DISABLED` or `ENABLE_FAILED` destination is not counted ([API_KinesisDataStreamDestination](https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_KinesisDataStreamDestination.html)).
- **`kms`** — StreamDescription.KeyId — stream-encryption key.
- **`lambda`** — Lambda consumers of the stream: event source mappings on the stream ARN and on each enhanced fan-out consumer's `ConsumerARN` from `ListStreamConsumers` ([with-kinesis](https://docs.aws.amazon.com/lambda/latest/dg/with-kinesis.html)).

### `kms`

AWS API: <https://docs.aws.amazon.com/kms/latest/APIReference/API_KeyMetadata.html>

- **`ct-events`** — Audit trail for key usage (Encrypt/Decrypt calls).
- **`dbi`** — RDS instances using this key.
- **`ebs`** — EBS volumes using this key.
- **`role`** — Key policy trusts roles.
- **`secrets`** — Secrets encrypted with this key.

### `lambda`

AWS API: <https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html>

- **`alarm`** — Errors/Throttles/Duration alarms watching the function.
- **`apigw`** — APIs with an integration invoking this function: an HTTP or WebSocket API's `GetIntegrations` `IntegrationUri`, a REST API's method integrations from `GetResources` with the `methods` embed ([API_GetResources](https://docs.aws.amazon.com/apigateway/latest/api/API_GetResources.html)).
- **`cf`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`cfn`** — CloudFormation stack that created the function.
- **`ct-events`** — Audit trail for function config changes.
- **`ddb`** — DDB Streams triggers.
- **`eb-rule`** — EventBridge rules with this function as a target.
- **`ecr`** — The repository the function's image belongs to (`lambda:GetFunction` `Code.ImageUri`, matched against the repository's URI).
- **`efs`** — FileSystemConfigs.
- **`eni`** — The Hyperplane ENIs this VPC-attached function uses: `InterfaceType` `lambda`, in one of its subnets, with exactly its security groups ([configuration-vpc](https://docs.aws.amazon.com/lambda/latest/dg/configuration-vpc.html)).
- **`kinesis`** — Kinesis event-source mapping.
- **`kms`** — Env-var encryption key.
- **`logs`** — CloudWatch Log Groups /aws/lambda/<name> where function logs land.
- **`msk`** — MSK event-source mapping.
- **`role`** — FunctionConfiguration.Role — execution permissions.
- **`s3`** — S3 event-source mapping.
- **`secrets`** — Secrets accessed at runtime.
- **`sg`** — FunctionConfiguration.VpcConfig.SecurityGroupIds — function ENI SGs.
- **`sns`** — The topic this function's `DeadLetterConfig.TargetArn` names, where failed asynchronous invocations go ([API_DeadLetterConfig](https://docs.aws.amazon.com/lambda/latest/api/API_DeadLetterConfig.html)), and the topics behind the confirmed lambda-protocol subscriptions whose `Endpoint` is this function ([API_Subscribe](https://docs.aws.amazon.com/sns/latest/api/API_Subscribe.html)).
- **`sns-sub`** — SNS subscriptions delivering to the function.
- **`sqs`** — Queues invoking the function through an event source mapping, and the queue its `DeadLetterConfig.TargetArn` names ([API_DeadLetterConfig](https://docs.aws.amazon.com/lambda/latest/api/API_DeadLetterConfig.html)).
- **`ssm`** — Parameters as config.
- **`subnet`** — FunctionConfiguration.VpcConfig.SubnetIds — function ENI subnets.
- **`tg`** — Lambda target groups whose `DescribeTargetHealth` lists this function ([API_DescribeTargetHealth](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeTargetHealth.html)).
- **`vpc`** — FunctionConfiguration.VpcConfig.VpcId — VPC the function runs in.

### `logs`

AWS API: <https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_LogGroup.html>

- **`alarm`** — Metric-filter-driven alarms, found through the group's own metric filters, plus alarms carrying a `LogGroupName` dimension in `AWS/Logs`.
- **`apigw`** — APIGW access logs.
- **`ct-events`** — Audit trail for log group changes.
- **`ecs-task`** — Tasks whose task definition names this group in a container's `awslogs-group` option, the field `ecs-task` → `logs` reads ([using_awslogs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_awslogs.html)).
- **`kinesis`** — Subscription filters whose `destinationArn` is a Kinesis stream.
- **`kms`** — LogGroup.KmsKeyId.
- **`lambda`** — Functions whose `LoggingConfig.LogGroup` is this group, `/aws/lambda/<function name>` when unset ([API_LoggingConfig](https://docs.aws.amazon.com/lambda/latest/api/API_LoggingConfig.html)), and subscription filters whose `destinationArn` is a function.
- **`s3`** — The buckets this group's export tasks wrote to, from `DescribeExportTasks` `ExportTask.destination` ([API_ExportTask](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_ExportTask.html)); a `FAILED`, `CANCELLED` or `PENDING_CANCEL` task wrote nothing ([API_ExportTaskStatus](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_ExportTaskStatus.html)). A subscription filter never delivers to a bucket.

### `lt`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ResponseLaunchTemplateData.html>

One-call budget: `DescribeLaunchTemplates` carries no `LaunchTemplateData` at all — every pivot field below comes from the fetcher's single `DescribeLaunchTemplateVersions(Versions=["$Default"])` call per template. `$Default` (not `$Latest`) is what `asg`/`ng`/`ec2` actually resolve at launch; `$Latest` is staging.

- **`ami`** — `LaunchTemplateData.ImageId` when it matches `ami-` (a `resolve:ssm:` reference is a display fact, not a pivot).
- **`asg`** — loaded-cache cross-ref: `AutoScalingGroup.LaunchTemplate`, `MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification`, and per-`Overrides[]` specifications — "which fleets launch from this template". A specification carries a `LaunchTemplateId` or a `LaunchTemplateName`, and either names the template.
- **`ct-events`** — audit trail: who bumped the default version.
- **`ec2`** — loaded-cache cross-ref by the auto-tag `aws:ec2launchtemplate:id` (catches direct, ASG, and NG launches); degrades to unknown when the ec2 cache has not been loaded at all — never a fake 0. A cache that WAS loaded but is truncated gives a real count so far, rendered `(N+)`.
- **`kms`** — `BlockDeviceMappings[].Ebs.KmsKeyId` in key-id/ARN form only (alias forms are detail-only; zero counts expected on most templates).
- **`ng`** — loaded-cache cross-ref: `Nodegroup.LaunchTemplate.Id`/`Name`.
- **`sg`** — union of `LaunchTemplateData.SecurityGroupIds` ∪ `NetworkInterfaces[].Groups` (mutually exclusive by API design); `SecurityGroups` (names, EC2-Classic legacy) are detail-only.
- **`subnet`** — `NetworkInterfaces[].SubnetId` — usually empty by design (the subnet normally comes from the ASG/NG side); rendered only when non-empty.

Explicitly excluded:

- **`role`** — `IamInstanceProfile` is a PROFILE, not a role; resolving profile→role needs `iam:GetInstanceProfile` (a second API call per template), and a name-equality heuristic is dishonest. Detail field only.
- **`eks`** — the cluster reference lives on the node group; pivot via `ng`.

### `msk`

AWS API: <https://docs.aws.amazon.com/msk/1.0/apireference/v1-clusters.html>

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

### `mwaa`

AWS API: <https://docs.aws.amazon.com/mwaa/latest/API/API_Environment.html>

- **`alarm`** — CloudWatch alarms in the `AWS/MWAA` namespace carry the `EnvironmentName` dimension; first triage stop during an incident (workflow pivot — join key is the environment name, no ARN field).
- **`ct-events`** — Audit trail for environment changes ("who ran UpdateEnvironment").
- **`kms`** — `KmsKey` — encrypts the metadata database, logs, and queue.
- **`logs`** — `LoggingConfiguration.{DagProcessingLogs,SchedulerLogs,WebserverLogs,WorkerLogs,TaskLogs}.CloudWatchLogGroupArn` — five per-component log groups, each counted when its module is `Enabled` ([API_ModuleLoggingConfiguration](https://docs.aws.amazon.com/mwaa/latest/API/API_ModuleLoggingConfiguration.html)); a failed DAG run sends the operator straight to TaskLogs/SchedulerLogs.
- **`role`** — `ExecutionRoleArn` — the role Airflow tasks assume for AWS access ("why can't my DAG write to S3").
- **`s3`** — `SourceBucketArn` — holds DAGs, requirements.txt, and plugins ("why isn't my DAG showing up").
- **`sg`** — `NetworkConfiguration.SecurityGroupIds` ("why can't Airflow reach RDS / my internal API").
- **`subnet`** — `NetworkConfiguration.SubnetIds` — where the environment's ENIs live; AZ and routing debugging.

Explicitly excluded:

- **`vpc`** — no direct field on `Environment`; reachable one hop via the subnet pivot's own related panel.
- **`sqs`** — `CeleryExecutorQueue` names a queue in an AWS-owned service account; a pivot dead-ends in AccessDenied. Detail-view fact only.
- **`vpce`** — `WebserverVpcEndpointService`/`DatabaseVpcEndpointService` are endpoint-service names, not the customer's `vpce-*` IDs, even when `EndpointManagement == CUSTOMER`. Detail text only.

### `nat`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGateway.html>

- **`alarm`** — NAT bandwidth/error alarms.
- **`ct-events`** — Audit trail for NAT changes.
- **`eip`** — NatGatewayAddresses[].AllocationId — attached EIPs. A `failed` or `deleted` gateway holds none, and an address that is `disassociating`, `unassigning` or `failed` is leaving it ([API_NatGateway](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGateway.html), [API_NatGatewayAddress](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGatewayAddress.html)).
- **`eni`** — NAT backing ENI, from the same live addresses as `eip`.
- **`rtb`** — Route tables with a live route to this NAT gateway; a blackhole route names no live gateway.
- **`subnet`** — Subnet the NAT lives in (must be public).
- **`vpc`** — Parent VPC.

### `ng`

AWS API: <https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html>

- **`ami`** — Nodegroup LaunchTemplate ImageId via ec2:DescribeLaunchTemplateVersions.
- **`asg`** — Nodegroup.Resources.AutoScalingGroups — backing ASG.
- **`ct-events`** — Audit trail for nodegroup changes.
- **`ebs`** — ASG → instances → ec2:DescribeInstances BlockDeviceMappings.Ebs.VolumeId.
- **`ec2`** — Worker-node instances; a `shutting-down` or `terminated` instance is no worker ([ec2-instance-lifecycle](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-lifecycle.html)).
- **`eks`** — Parent EKS cluster.
- **`role`** — Nodegroup.NodeRole — IAM role nodes assume.
- **`sg`** — RemoteAccess.SourceSecurityGroups.
- **`subnet`** — Nodegroup.Subnets[] (direct field).

### `opensearch`

AWS API: <https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DomainStatus.html>

- **`acm`** — `DomainEndpointOptions.CustomEndpointCertificateArn`, when `CustomEndpointEnabled` ([API_DomainEndpointOptions](https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DomainEndpointOptions.html)).
- **`alarm`** — Cluster health alarms.
- **`cfn`** — CloudFormation stack that created the domain.
- **`ct-events`** — Audit trail for domain config changes.
- **`kms`** — EncryptionAtRestOptions.KmsKeyId.
- **`logs`** — `LogPublishingOptions` groups whose option is `Enabled` ([API_LogPublishingOption](https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_LogPublishingOption.html)).
- **`sg`** — VPCOptions.SecurityGroupIds — domain ENI SGs.
- **`subnet`** — VPCOptions.SubnetIds — domain ENI subnets.
- **`vpc`** — VPCOptions.VPCId — attached VPC (if any).

### `pipeline`

AWS API: <https://docs.aws.amazon.com/codepipeline/latest/APIReference/API_PipelineDeclaration.html>

- **`cb`** — CodeBuild projects used as pipeline actions.
- **`cfn`** — Deploy CFN action.
- **`ct-events`** — Audit trail for pipeline state changes.
- **`eb-rule`** — Triggered by EventBridge.
- **`ecr`** — Push/pull images.
- **`ecs-svc`** — `ECS` deploy actions' `ClusterName` + `ServiceName`. A `CodeDeployToECS` action names a CodeDeploy application, not a service, and a9s reads no CodeDeploy, so a pipeline holding one reads unknown ([action-reference-ECSbluegreen](https://docs.aws.amazon.com/codepipeline/latest/userguide/action-reference-ECSbluegreen.html)).
- **`kms`** — Artifact-store encryption key.
- **`lambda`** — Invoke Lambda action.
- **`role`** — Pipeline service role.
- **`s3`** — Artifact store bucket.
- **`sns`** — Approval SNS topic.

### `policy`

AWS API: <https://docs.aws.amazon.com/IAM/latest/APIReference/API_Policy.html>

- **`ct-events`** — Audit trail for policy version / attach events.
- **`iam-group`** — Groups with this policy attached, from `ListEntitiesForPolicy`.
- **`iam-user`** — Users with this policy attached.
- **`role`** — Roles with this policy attached, from `ListEntitiesForPolicy`.

### `r53`

AWS API: <https://docs.aws.amazon.com/Route53/latest/APIReference/API_HostedZone.html>

- **`acm`** — DNS-validated certs whose validation CNAME in this zone points at `acm-validations.aws`.
- **`apigw`** — APIGW custom domain aliases.
- **`cf`** — CloudFront distributions this zone's alias records target, matched on the distribution's domain name.
- **`ct-events`** — Audit trail for zone record changes.
- **`elb`** — Load balancers this zone's alias records target, matched on the load balancer's DNS name (application, network, gateway and classic alike).
- **`logs`** — Query logs → CW Logs (`route53:ListQueryLoggingConfigs` per zone; at most one log-group binding).
- **`s3`** — Alias to S3 website endpoint.
- **`vpc`** — Private hosted zones VPC association.

### `dbi-snap`

AWS API: <https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html>

- **`backup`** — Backup plans whose selections name the snapshot's source instance by ARN or select it by its tags ([working-with-supported-services](https://docs.aws.amazon.com/aws-backup/latest/devguide/working-with-supported-services.html)).
- **`ct-events`** — Audit trail for snapshot create/restore/copy.
- **`dbi`** — Source DB instance.
- **`kms`** — Encryption key.

> Note: `dbc` is intentionally absent. Real AWS rejects `CreateDBSnapshot` on
> Aurora cluster members; Aurora cluster snapshots live in `dbc-snap`
> (`DBClusterSnapshot`). A registered `dbi-snap → dbc` pivot would always
> resolve `Count=0`, which is dead UX. See `core/aws/dbi_snap.go` for the
> structural exclusion comment.

### `redis`

AWS API: <https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_ReplicationGroup.html>

- **`alarm`** — Replication-group CW alarms.
- **`cfn`** — CloudFormation stack that created the group.
- **`ct-events`** — Audit trail for group changes.
- **`kms`** — At-rest encryption key.
- **`logs`** — LogDeliveryConfigurations.
- **`secrets`** — AuthTokenSecret.
- **`sg`** — Attached security groups.
- **`sns`** — NotificationTopicArn, when `TopicStatus` is `active`: notifications are sent only then ([API_ModifyCacheCluster](https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_ModifyCacheCluster.html)).
- **`subnet`** — CacheSubnetGroup.Subnets.
- **`vpc`** — CacheSubnetGroup.VpcId.

### `redshift`

AWS API: <https://docs.aws.amazon.com/redshift/latest/APIReference/API_Cluster.html>

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

AWS API: <https://docs.aws.amazon.com/IAM/latest/APIReference/API_Role.html>

- **`ct-events`** — Configuration events *for* this role: who created it, who attached or detached its policies, who changed its trust. The lookup is [`LookupEvents`](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html) on `ResourceName` = the role ARN, in us-east-1 where IAM records its events. The other question — what the role *did* — is not answerable through one lookup attribute: [`LookupAttributeKey`](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupAttribute.html) exposes `{EventId, EventName, ReadOnly, Username, ResourceType, ResourceName, EventSource, AccessKeyId}`, and for `Type=AssumedRole` the role lives only in `userIdentity.sessionContext.sessionIssuer.userName`, which none of them selects.
- **`ec2`** — EC2 instances assuming this role via instance profile.
- **`eks`** — EKS service role.
- **`glue`** — Glue jobs assuming this role.
- **`iam-group`** — Trust relationships may reference groups.
- **`iam-user`** — Trust may include user principals.
- **`lambda`** — Lambdas executing as this role.
- **`ng`** — EKS node groups assuming this role.
- **`policy`** — Attached managed policies, from `ListAttachedRolePolicies`.

### `rtb`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RouteTable.html>

- **`cfn`** — CloudFormation stack that created the route table.
- **`ct-events`** — Audit trail for route changes.
- **`eni`** — ENI route targets (e.g. firewall appliances).
- **`igw`** — Internet gateways this table has a live route to.
- **`nat`** — NAT gateways this table has a live route to.
- **`subnet`** — Subnets this table routes for: those its `Associations` name, plus, for the VPC's main table, the subnets no other table names.
- **`tgw`** — Transit gateways this table has a live route to.
- **`vpc`** — Parent VPC.
- **`vpce`** — Gateway-endpoint routes.

### `s3`

AWS API: <https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListBuckets.html>

- **`athena`** — Athena queries over S3 data.
- **`backup`** — Backup plans whose selections name the bucket by ARN or select it by its tags ([working-with-supported-services](https://docs.aws.amazon.com/aws-backup/latest/devguide/working-with-supported-services.html)).
- **`cf`** — CloudFront distributions whose `Origins.Items` address this bucket.
- **`cfn`** — The stack this bucket's `aws:cloudformation:stack-name` tag names.
- **`ct-events`** — Audit trail for bucket-level events.
- **`eb-rule`** — EB rules on S3 object events.
- **`glue`** — Glue jobs whose script, `Command.ScriptLocation`, is in this bucket ([API_JobCommand](https://docs.aws.amazon.com/glue/latest/webapi/API_JobCommand.html)). A crawler is not a row of the `glue` list, which holds jobs.
- **`kms`** — Bucket SSE-KMS key.
- **`lambda`** — Lambdas with this bucket as event source.
- **`s3`** — Server access-log destination bucket (`GetBucketLogging.LoggingEnabled.TargetBucket`). S3 server-access logs go to another S3 bucket, not CloudWatch Logs — registered as `s3` (DisplayName "Access Log Bucket"), not `logs`.
- **`r53`** — R53 alias to S3 website endpoint.
- **`role`** — Bucket policy may reference IAM role ARNs as principals; advanced audit pivot via `s3:GetBucketPolicy`.
- **`sns`** — BucketNotification SNS target.
- **`sqs`** — BucketNotification SQS target.
- **`trail`** — CloudTrails writing to this bucket.
- ~~**`iam-user`**~~ — Removed 2026-04-22. Principal attribution requires CloudTrail data-plane parsing (Wave 3); `ListBuckets` Owner field is an account ID, not a user ARN. No operator-grade AWS API surface for a direct S3→IAM-user edge. a9s-devops: not worth it (docs/resources/s3.md §5).
- ~~**`waf`**~~ — Removed 2026-04-22. WAF web ACLs attach to CloudFront/ALB/API Gateway/AppSync/Cognito, not S3 directly. The indirect path S3→CloudFront→WAF is already reachable via the `cf` panel entry. a9s-devops: not worth it (docs/resources/s3.md §5).

### `secrets`

AWS API: <https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_SecretListEntry.html>

- **`cb`** — Reverse-scan: CodeBuild Project.Environment.EnvironmentVariables where Type=SECRETS_MANAGER and Value==ARN or name prefix.
- **`cfn`** — SecretListEntry.Tags["aws:cloudformation:stack-name"] matched against CFN stack cache.
- **`codeartifact`** — Heuristic: secret Name or Tags contain "codeartifact" (no direct AWS API). A secret that names none has no candidates, which is a proven zero.
- **`ct-events`** — Audit trail for secret rotation/access.
- **`dbi`** — Reverse-scan: DBInstance.MasterUserSecret.SecretArn == this secret's ARN.
- **`eb`** — Reverse-scan: elasticbeanstalk:DescribeConfigurationSettings OptionSettings[].Value contains `{{resolve:secretsmanager:<ARN>`, or an `aws:elasticbeanstalk:application:environmentsecrets` option names this secret ([AWSHowTo.secrets.env-vars](https://docs.aws.amazon.com/elasticbeanstalk/latest/dg/AWSHowTo.secrets.env-vars.html)).
- **`ecs-task`** — Reverse-scan: task definitions referencing this secret in container `secrets[].valueFrom`, `logConfiguration.secretOptions[].valueFrom` or `repositoryCredentials.credentialsParameter`, by ARN (a JSON-key or version suffix included) or by name ([secrets-envvar-secrets-manager](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/secrets-envvar-secrets-manager.html)).
- **`kms`** — SecretListEntry.KmsKeyId — UUID suffix matched against KMS key cache.
- **`lambda`** — SecretListEntry.RotationLambdaARN — function name suffix matched against Lambda cache.
- **`logs`** — RotationLambdaARN → lambda:GetFunction → FunctionConfiguration.LoggingConfig.LogGroup. When GetFunction does not answer, the default /aws/lambda/<name> is a heuristic candidate: a function with a custom LoggingConfig logs elsewhere.
- **`role`** — secretsmanager:GetResourcePolicy → Statement[].Principal.AWS role ARNs; RotationLambdaARN → lambda:GetFunction → FunctionConfiguration.Role.
- **`sns`** — RotationLambdaARN → lambda:GetFunction → FunctionConfiguration.DeadLetterConfig.TargetArn if SNS ARN.

### `ses`

AWS API: <https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_IdentityInfo.html>

- **`ct-events`** — Audit trail for identity changes.
- **`eb-rule`** — sesv2:GetEmailIdentity → ConfigurationSetName → sesv2:GetConfigurationSetEventDestinations → EventBridgeDestination.EventBusArn; extract bus name and cross-reference the eb-rule cache on `EventBusName`. Returns rule names (not bus ARNs) so drilling filters correctly.
- **`lambda`** — ses:DescribeActiveReceiptRuleSet → LambdaAction.FunctionArn of the enabled rules; a rule with `Enabled` false or unset processes no mail ([API_ReceiptRule](https://docs.aws.amazon.com/ses/latest/APIReference/API_ReceiptRule.html)). Function names extracted from ARNs to match the lambda cache's IDs.
- **`r53`** — Identity domain (or domain portion of email address) matched against Route 53 hosted zone names.
- **`s3`** — ses:DescribeActiveReceiptRuleSet → S3Action.BucketName of the enabled rules ([API_ReceiptRule](https://docs.aws.amazon.com/ses/latest/APIReference/API_ReceiptRule.html)).
- **`sns`** — sesv2:GetEmailIdentity → ConfigurationSetName → sesv2:GetConfigurationSetEventDestinations → SnsDestination.TopicArn of the enabled destinations ([API_EventDestination](https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_EventDestination.html)).

### `sfn`

AWS API: <https://docs.aws.amazon.com/step-functions/latest/apireference/API_StateMachineListItem.html>

- **`alarm`** — Execution-failure alarms.
- **`ct-events`** — Audit trail for state-machine changes.
- **`eb-rule`** — EventBridge rules with this state machine as target.
- **`kms`** — Execution-data encryption.
- **`lambda`** — Lambda integrations invoked by the state machine.
- **`logs`** — The log group `LoggingConfiguration.Destinations[].CloudWatchLogsLogGroup.LogGroupArn` names, unless the level is `OFF` ([API_LoggingConfiguration](https://docs.aws.amazon.com/step-functions/latest/apireference/API_LoggingConfiguration.html)).
- **`role`** — StateMachine.RoleArn — execution role.

### `sg`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_SecurityGroup.html>

- **`cfn`** — CloudFormation stack that created the SG.
- **`ct-events`** — Audit trail for rule changes.
- **`ec2`** — Instances that have this SG attached.
- **`elb`** — Load balancers with this SG attached (ALBs only).
- **`eni`** — ENIs with this SG attached (covers Lambda, RDS, etc.).
- **`lambda`** — Lambda VPC ENIs reference SGs.
- **`sg`** — Security groups this group's own ingress and egress rules reference in `UserIdGroupPairs[].GroupId`; a pair in another account names a group this account cannot list ([API_UserIdGroupPair](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_UserIdGroupPair.html)).
- **`vpc`** — Parent VPC.

### `sns`

AWS API: <https://docs.aws.amazon.com/sns/latest/api/API_Topic.html>

- **`alarm`** — Topic delivery/failure alarms.
- **`ct-events`** — Audit trail for topic changes.
- **`kms`** — KmsMasterKeyId (SSE-KMS).
- **`role`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`sns-sub`** — Subscriptions on this topic.

### `sns-sub`

AWS API: <https://docs.aws.amazon.com/sns/latest/api/API_Subscription.html>

- **`ct-events`** — Audit trail for subscription changes.
- **`lambda`** — Lambda endpoint subscriber.
- **`sns`** — Parent topic.
- **`sqs`** — SQS endpoint subscriber.

### `sqs`

AWS API: <https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html>

- **`alarm`** — ApproximateAgeOfOldestMessage / MessagesVisible alarms.
- **`ct-events`** — Audit trail for queue attribute changes.
- **`eb-rule`** — Rules with a target that is this queue, or whose target's `DeadLetterConfig.Arn` is this queue ([API_DeadLetterConfig](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_DeadLetterConfig.html)).
- **`kms`** — KmsMasterKeyId (SSE-KMS).
- **`lambda`** — Lambda event-source mappings consuming this queue, and functions whose `DeadLetterConfig.TargetArn` is this queue ([API_DeadLetterConfig](https://docs.aws.amazon.com/lambda/latest/api/API_DeadLetterConfig.html)).
- **`sns`** — Topics behind the confirmed sqs-protocol subscriptions whose `Endpoint` is this queue's ARN, whole; a subscription pending confirmation delivers nothing ([API_Subscribe](https://docs.aws.amazon.com/sns/latest/api/API_Subscribe.html)).
- **`sns-sub`** — Subscriptions whose `Endpoint` is this queue's ARN, whole; one queue's ARN is the prefix of another's.
- **`sqs`** — DLQ reference / RedriveTarget.

### `ssm`

AWS API: <https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_ParameterMetadata.html>

- **`ct-events`** — Audit trail for parameter reads/writes.
- **`kms`** — KeyId — KMS key for SecureString.

### `subnet`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Subnet.html>

- **`asg`** — ASGs referencing this subnet.
- **`cfn`** — CloudFormation stack that created the subnet.
- **`ct-events`** — Audit trail for subnet changes.
- **`ec2`** — Instances in this subnet.
- **`efs`** — EFS mount targets project one ENI per AZ into the subnet — zero-call join: scan the already-loaded `eni` cache for the mount-target ENIs in this subnet, read the file system each description names, and match it against the loaded `efs` cache.
- **`eks`** — EKS clusters declaring subnet.
- **`elb`** — Load balancer AZ-subnet mappings.
- **`eni`** — ENIs in this subnet.
- **`nat`** — NAT gateways in this subnet.
- **`rtb`** — The route table this subnet routes through: the one whose `Associations` name it, or the VPC's main table when none does.
- **`vpc`** — Parent VPC (`Subnet.VpcId`).
- **`vpce`** — Interface endpoints in subnet.

### `tg`

AWS API: <https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_TargetGroup.html>

- **`alarm`** — TG health/unhealthy-host count alarms.
- **`asg`** — ASGs registering into this TG.
- **`cfn`** — Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ct-events`** — Audit trail for TG changes.
- **`ec2`** — Instance targets, from `DescribeTargetHealth`.
- **`ecs-svc`** — ECS services routing to this TG.
- **`elb`** — Load balancers using this TG.
- **`lambda`** — Lambda targets.
- **`vpc`** — TargetGroup.VpcId.

### `tgw`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TransitGateway.html>

- **`ct-events`** — Audit trail for attachment changes.
- **`role`** — Cross-account RAM share roles. Heuristic: the account-wide `AWSServiceRoleForVPCTransitGateway` service-linked role, which no one gateway names.
- **`rtb`** — VPC route tables with a live route to this transit gateway; a blackhole route names no live gateway.
- **`subnet`** — VPC attachment subnets.
- **`vpc`** — VPCs attached to this TGW, from its `TransitGateway` VPC attachments; a `deleted`, `failing`, `failed`, `rejecting` or `rejected` attachment connects nothing ([vpc-attachment-lifecycle](https://docs.aws.amazon.com/vpc/latest/tgw/tgw-vpc-attachments.html#vpc-attachment-lifecycle)).

### `trail`

AWS API: <https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html>

- **`ct-events`** — Audit trail for trail config changes (meta!).
- **`kms`** — Trail.KmsKeyId — log-file encryption key.
- **`logs`** — Trail.CloudWatchLogsLogGroupArn — associated log group.
- **`role`** — CloudWatchLogsRoleArn / org-trail role.
- **`s3`** — Trail.S3BucketName — destination bucket.
- **`sns`** — Trail.SnsTopicARN — delivery notifications.

### `transfer`

AWS API: <https://docs.aws.amazon.com/transfer/latest/userguide/API_DescribedServer.html>

- **`acm`** — `DescribedServer.Certificate` (ACM ARN, present for FTPS servers only — the server identity cert; AS2 certificates are transfer-managed, not ACM).
- **`ct-events`** — Audit trail for server changes ("who stopped this server").
- **`eip`** — `EndpointDetails.AddressAllocationIds` (present for internet-facing VPC endpoints only) — the static addresses partners allowlist; live-witnessed ×3 on an internet-facing SFTP server (2026-07-14).
- **`lambda`** — `IdentityProviderDetails.Function` when `IdentityProviderType == AWS_LAMBDA` — "why is auth rejecting this user" jumps to the authorizer.
- **`logs`** — `StructuredLogDestinations` (log-group ARNs) — where a failed-transfer investigation actually goes.
- **`role`** — `LoggingRole` — first stop for "why are there no logs".
- **`subnet`** — `EndpointDetails.SubnetIds` — endpoint ENIs; partner-reachability debugging.
- **`vpc`** — `EndpointDetails.VpcId` (set when `EndpointType == VPC`) — top of the reachability chain.
- **`vpce`** — `EndpointDetails.VpcEndpointId` — the hop that carries the security groups (live-witnessed populated for the auto-created endpoint).

Explicitly excluded:

- **`sg`** — `EndpointDetails.SecurityGroupIds` is documented but NEVER populated in DescribeServer responses (SDK doc: use EC2 DescribeVpcEndpoints with the VpcEndpointId) — a direct pivot would render an empty panel always; reach sg via the `vpce` pivot.
- **`apigw`** — `IdentityProviderDetails.Url` is a free-form URL, not an API id; parsing the execute-api subdomain is fragile. Copyable detail field only.
- **`s3` / `efs`** — `Domain` is an enum only; no bucket/filesystem field exists on the server (the bucket appears as a path inside agreement `BaseDirectory`).

### `vpc`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Vpc.html>

- **`cfn`** — CloudFormation stack that created the VPC.
- **`ct-events`** — Audit trail for VPC-level changes.
- **`ec2`** — EC2 instances in this VPC.
- **`elb`** — Load balancers in this VPC.
- **`eni`** — ENIs in VPC.
- **`igw`** — Internet gateways attached to this VPC.
- **`nat`** — NAT gateways in this VPC.
- **`rtb`** — Route tables in this VPC.
- **`sg`** — Security groups scoped to this VPC.
- **`subnet`** — Subnets in this VPC (`Subnet.VpcId`).
- **`tgw`** — Transit gateways this VPC is attached to, from the same `TransitGateway` attachments and the same liveness rule ([vpc-attachment-lifecycle](https://docs.aws.amazon.com/vpc/latest/tgw/tgw-vpc-attachments.html#vpc-attachment-lifecycle)).
- **`vpce`** — VPC endpoints in this VPC.

### `vpc-peer`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcPeeringConnection.html>

Load-bearing SDK fact: `CidrBlock`/`CidrBlockSet` on `RequesterVpcInfo`/`AccepterVpcInfo` are returned ONLY for `active` connections — nil in every other state (SDK doc comment states this verbatim). Nil-safe rendering is mandatory; the CIDR-overlap check is active-only.

- **`rtb`** — scan the loaded `rtb` cache for live `Routes[].VpcPeeringConnectionId == <pcx-id>` — "who actually routes to this peer"; zero API calls. A blackhole route keeps the stale id after the connection is deleted and is not counted. The pivot that makes the type worth adding.
- **`vpc`** — symmetric CACHE-MEMBERSHIP GATE: pivot only for whichever side's `VpcId` is present in the loaded local `vpc` cache (a cross-account remote side is never in the cache → rendered as a plain fact `VpcId + OwnerId + Region`, not a pivot). Never hardcode requester-is-local.
- **`ct-events`** — audit trail: Create/Accept/Reject/Delete/ModifyVpcPeeringConnection*. Universal pivot.

Explicitly excluded:

- **`sg`** — no declared link exists; a fuzzy CIDR/referenced-SG scan is noise, and cross-region peers cannot reference SGs at all. Wave 3.

### `vpce`

AWS API: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcEndpoint.html>

- **`alarm`** — Mentioned by 2/6 independent DevOps audits as an AWS-API or operational pivot.
- **`ct-events`** — Audit trail for endpoint changes.
- **`eni`** — ENIs backing interface endpoints.
- **`logs`** — Flow logs on the endpoint's VPC or subnets, from `DescribeFlowLogs` by `resource-id`, that deliver to CloudWatch Logs ([API_CreateFlowLogs](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateFlowLogs.html)).
- **`r53`** — Private DNS → R53 private zones (`route53:ListHostedZonesByVPC` per endpoint, keyed by the endpoint's `VpcId`; results matched against the loaded `r53` cache).
- **`rtb`** — Route tables for gateway endpoints.
- **`sg`** — SGs attached to interface endpoints.
- **`subnet`** — Interface endpoint subnets.
- **`vpc`** — Parent VPC.

### `waf`

AWS API: <https://docs.aws.amazon.com/waf/latest/APIReference/API_WebACL.html>

- **`alarm`** — Blocked-request alarms.
- **`apigw`** — API Gateways with this WebACL attached.
- **`cf`** — CloudFront distributions with this WebACL attached.
- **`ct-events`** — Audit trail for ACL rule changes.
- **`elb`** — ALBs with this WebACL attached.
- **`logs`** — `GetLoggingConfiguration` for every `LogScope` (CUSTOMER, SECURITY_LAKE, CLOUDWATCH_TELEMETRY_RULE_MANAGED) → CW Logs destinations ([API_GetLoggingConfiguration](https://docs.aws.amazon.com/waf/latest/APIReference/API_GetLoggingConfiguration.html)).

## Explicitly excluded

> **Do not re-add.** These 79 parent→related pairs (58 from the original
> five-reviewer DevOps audit, 21 budget-excluded pivots deregistered on
> 2026-07-06) have no implementable linkage in the AWS API surface within
> the call budget (beyond heuristic reverse-scans that would lie to users
> with false positives or silent zeros). See
> [related-panel-devops-consensus.md](./historical/019-related-panel/related-panel-devops-consensus.md)
> for the evidence trail. Re-adding any of these pairs requires new AWS API
> evidence cited per the Policy section at the top of this file.

### Budget-excluded — structurally uncomputable on any cache state (25)

> These pivots were REMOVED from the registry (not merely marked
> `budget-excluded` in a per-type row) because their checkers were
> hardcoded to `Count: -1` unconditionally — no cache state, no fixture,
> no AWS account can ever make them resolve a real count or a working
> drill-in. A row that can never show a count and can never be drilled is
> noise, not a decision an operator can act on (Policy rule 7). Each
> citation below is the exact reasoning that previously lived in the
> per-type row/bullet before removal.

- `apigw` → `r53` — alias records live on per-zone `ListResourceRecordSets` and are not cached as joinable record sets (the r53 fetcher summarizes alias targets into one Fields string); resolving custom-domain aliases needs `GetDomainNames` plus per-zone record scans — checker returns Count -1.
- `apigw` → `vpce` — endpoint IDs live on v1 `RestApi.EndpointConfiguration` only; the v2 `GetApis` items carry none, and the v2 path is a brittle resource-policy parse (policy-parse gap) — checker returns Count -1.
- `apigw` → `waf` — v2 APIs carry no Web ACL binding on `GetApis`; WAF-side resolution requires `wafv2:ListResourcesForWebACL` per Web ACL (O(N)) — checker returns Count -1.
- `apigw` → `sfn` — Step Functions integration target: the integration URI only reveals the `:states:action/` service slug — the target state-machine ARN lives in the route REQUEST TEMPLATE, not the IntegrationUri; identifying the state machine requires per-route request-template parsing — checker returns Count -1.
- `apigw` → `sns` — APIGW -> SNS via integration: the integration URI only reveals `:sns:action/Publish` — the topic ARN lives in the route REQUEST TEMPLATE, not the IntegrationUri; identifying the topic requires per-route request-template parsing — checker returns Count -1.
- `athena` → `glue` — every Athena workgroup queries the account's default Glue Data Catalog implicitly; no structured "glue job/catalog" field exists on the WorkGroupConfiguration to name a specific Glue resource, so the checker can only ever return Count 0 or Count -1 — never a real count.
- `cf` → `logs` — [API_Distribution](https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_Distribution.html) has no log-group field: standard logging names an S3 bucket (`DistributionConfig.Logging.Bucket`, counted under `s3`), and real-time logs go to Kinesis Data Streams — no row of the `logs` type is named.
- `glue` → `athena` — a Glue job records no Athena workgroup and a workgroup records no Glue job ([API_Job](https://docs.aws.amazon.com/glue/latest/webapi/API_Job.html), [API_WorkGroup](https://docs.aws.amazon.com/athena/latest/APIReference/API_WorkGroup.html)); Athena reads the account's Data Catalog implicitly, the same reason `athena` → `glue` is excluded — no lookup can discover a link.
- `ec2` → `ssm` — the `ssm` type lists Parameter Store parameters ([API_ParameterMetadata](https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_ParameterMetadata.html)); the SSM managed-instance registration of an instance (`DescribeInstanceInformation`) has no a9s type, and no parameter row stands for an instance.
- `eip` → `logs` — EIPs emit no logs; flow logs on the associated ENI/subnet/VPC are not identifiable from the EIP without per-ENI `DescribeFlowLogs` — checker returns Count -1.
- `elb` → `r53` — record sets live on per-zone `ListResourceRecordSets` and are not cached as joinable structures (the r53 fetcher summarizes alias targets into one Fields string); identifying the aliasing records requires O(N) per-zone record-set queries — checker returns Count -1.
- `kms` → `s3` — S3 bucket resources assembled by `FetchS3BucketsPage` do not store KMS key info in Fields or RawStruct, so the relationship cannot be determined from cache alone — checker returns Count -1.
- `pipeline` → `codeartifact` — CodePipeline has no CodeArtifact action provider ([action structure reference](https://docs.aws.amazon.com/codepipeline/latest/userguide/action-reference.html)), so no `PipelineDeclaration` action names a CodeArtifact repository — no lookup can discover a link.
- `tg` → `backup` — AWS Backup does not support target groups; no AWS field links a TG to a plan or recovery point — checker returns Count -1.
- `tg` → `dbc` — TG target types are instance/ip/lambda/alb; no AWS field references a DocumentDB cluster — checker returns Count -1.
- `tg` → `dbi` — TG target types are instance/ip/lambda/alb; no AWS field references an RDS instance — checker returns Count -1.
- `tg` → `dbi-snap` — no AWS field links a TG to an RDS snapshot — checker returns Count -1.
- `tg` → `logs` — target groups do not emit CloudWatch Logs; ELB access logs go to S3 on the parent LB — checker returns Count -1.
- `tg` → `sg` — `TargetGroup` has no SecurityGroups field; the SG pivot belongs to the parent `elb` — checker returns Count -1.
- `tg` → `subnet` — `TargetGroup` has no subnet field; the subnet pivot belongs to the parent `elb` via `AvailabilityZones[].SubnetId` — checker returns Count -1.
- `vpce` → `acm` — the list response carries no cert reference; resolution requires `PrivateDnsNameConfiguration` lookups per endpoint service — checker returns Count -1.
- `vpce` → `cf` — the CloudFront→VPCE link goes through CloudFront VPC Origins, which are not on `DistributionSummary` — checker returns Count -1.
- `vpce` → `s3` — identifying reachable buckets requires interpreting `VpcEndpoint.PolicyDocument` JSON against bucket policies — no deterministic join within the checker budget; checker returns Count -1.
- `vpce` → `tg` — the TG cache carries no registered targets; matching endpoint IPs requires `DescribeTargetHealth` per TG — checker returns Count -1.
- `vpce` → `waf` — the endpoint list response has no Web ACL binding; WAF associations resolve only from the WAF side via `wafv2:ListResourcesForWebACL` per ACL — checker returns Count -1.

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

<!-- BEGIN GENERATED: related-table -->
| Source Type | Target Type | Display Name | Needs Target Cache |
| --- | --- | --- | --- |
| ec2 | tg | Target Groups | yes |
| ec2 | asg | Auto Scaling Groups | yes |
| ec2 | alarm | CloudWatch Alarms | yes |
| ec2 | ng | EKS Node Groups | yes |
| ec2 | cfn | CloudFormation Stacks | yes |
| ec2 | eip | Elastic IPs | yes |
| ec2 | ebs | EBS Volumes | no |
| ec2 | ebs-snap | EBS Snapshots | yes |
| ec2 | ct-events | CloudTrail Events | no |
| ec2 | sg | Security Groups | no |
| ec2 | vpc | VPC | no |
| ec2 | role | IAM Role | no |
| ec2 | ami | AMI | no |
| ec2 | eni | Network Interfaces | no |
| ec2 | subnet | Subnet | no |
| ec2 | kms | KMS Keys | yes |
| ec2 | logs | Log Groups | yes |
| ec2 | backup | Backup Plans | yes |
| ecs-svc | ecs | ECS Clusters | no |
| ecs-svc | tg | Target Groups | no |
| ecs-svc | alarm | CloudWatch Alarms | yes |
| ecs-svc | elb | Load Balancers | yes |
| ecs-svc | logs | Log Groups | yes |
| ecs-svc | sg | Security Groups | no |
| ecs-svc | role | IAM Role | no |
| ecs-svc | cfn | CloudFormation Stacks | yes |
| ecs-svc | ct-events | CloudTrail Events | no |
| ecs-svc | eb-rule | EventBridge Rules | yes |
| ecs-svc | ecr | ECR Repositories | yes |
| ecs-svc | ecs-task | ECS Tasks | yes |
| ecs-svc | secrets | Secrets | no |
| ecs-svc | sfn | Step Functions | yes |
| ecs-svc | subnet | Subnets | no |
| ecs-svc | vpc | VPC | yes |
| ecs | ecs-svc | ECS Services | yes |
| ecs | alarm | CloudWatch Alarms | yes |
| ecs | cfn | CloudFormation Stacks | yes |
| ecs | kms | KMS Key | no |
| ecs | asg | Auto Scaling Groups | yes |
| ecs | ec2 | EC2 Instances | yes |
| ecs | ct-events | CloudTrail Events | no |
| ecs | ecs-task | ECS Tasks | yes |
| ecs | logs | Log Groups | yes |
| ecs-task | ecs-svc | ECS Services | no |
| ecs-task | ecs | ECS Clusters | no |
| ecs-task | logs | Log Groups | yes |
| ecs-task | role | IAM Role | yes |
| ecs-task | alarm | CloudWatch Alarms | yes |
| ecs-task | ct-events | CloudTrail Events | no |
| ecs-task | ec2 | EC2 Instances | no |
| ecs-task | ecr | ECR Repositories | yes |
| ecs-task | eni | Network Interfaces | no |
| ecs-task | secrets | Secrets | yes |
| ecs-task | sg | Security Groups | yes |
| ecs-task | ssm | SSM Parameters | yes |
| ecs-task | subnet | Subnets | no |
| lambda | role | IAM Roles | no |
| lambda | alarm | CW Alarms | yes |
| lambda | logs | Log Groups | yes |
| lambda | sg | Security Groups | no |
| lambda | vpc | VPC | no |
| lambda | kms | KMS Key | no |
| lambda | sqs | SQS Queues | no |
| lambda | cfn | CloudFormation | no |
| lambda | eb-rule | EventBridge Rules | no |
| lambda | subnet | Subnets | no |
| lambda | efs | EFS File Systems | no |
| lambda | apigw | API Gateways | yes |
| lambda | cf | CloudFront | yes |
| lambda | ddb | DynamoDB Tables | no |
| lambda | kinesis | Kinesis Streams | no |
| lambda | msk | MSK Clusters | no |
| lambda | ct-events | CloudTrail Events | no |
| lambda | tg | Target Groups | yes |
| lambda | sns | SNS Topics | yes |
| lambda | sns-sub | SNS Subscriptions | yes |
| lambda | s3 | S3 Buckets | yes |
| lambda | ecr | ECR Repositories | yes |
| lambda | eni | Network Interfaces | yes |
| lambda | secrets | Secrets | yes |
| lambda | ssm | SSM Parameters | yes |
| asg | ec2 | EC2 Instances | no |
| asg | tg | Target Groups | no |
| asg | subnet | Subnets | no |
| asg | alarm | CloudWatch Alarms | yes |
| asg | ng | EKS Node Groups | yes |
| asg | ami | AMI | no |
| asg | elb | Load Balancers | no |
| asg | role | IAM Roles | no |
| asg | sg | Security Groups | no |
| asg | sns | SNS Topics | no |
| asg | vpc | VPCs | no |
| asg | ct-events | CloudTrail Events | no |
| ebs | ec2 | EC2 Instances | no |
| ebs | ebs-snap | EBS Snapshots | yes |
| ebs | kms | KMS Key | no |
| ebs | alarm | CW Alarms | yes |
| ebs | backup | Backup | yes |
| ebs | cfn | CloudFormation | yes |
| ebs | ct-events | CloudTrail Events | no |
| ebs-snap | ami | AMIs | yes |
| ebs-snap | ebs | EBS Volume | no |
| ebs-snap | ec2 | EC2 Instance | yes |
| ebs-snap | kms | KMS Key | no |
| ebs-snap | backup | Backup | yes |
| ebs-snap | ct-events | CloudTrail Events | no |
| ami | ec2 | EC2 Instances | yes |
| ami | ebs-snap | EBS Snapshots | no |
| ami | asg | Auto Scaling Groups | yes |
| ami | cfn | CloudFormation Stacks | yes |
| ami | kms | KMS Keys | no |
| ami | ng | EKS Node Groups | yes |
| ami | ct-events | CloudTrail Events | no |
| lt | ami | AMI | no |
| lt | asg | Auto Scaling Groups | yes |
| lt | ec2 | EC2 Instances | yes |
| lt | kms | KMS Key | no |
| lt | ng | EKS Node Groups | yes |
| lt | sg | Security Groups | no |
| lt | subnet | Subnets | no |
| lt | ct-events | CloudTrail Events | no |
| eks | ng | Node Groups | yes |
| eks | alarm | CloudWatch Alarms | yes |
| eks | cfn | CloudFormation Stacks | yes |
| eks | logs | Log Groups | yes |
| eks | sg | Security Groups | no |
| eks | vpc | VPC | no |
| eks | role | IAM Role | no |
| eks | kms | KMS Key | no |
| eks | subnet | Subnets | no |
| eks | ami | AMI | no |
| eks | asg | Auto Scaling Groups | no |
| eks | ec2 | EC2 Instances | no |
| eks | ct-events | CloudTrail Events | no |
| ng | eks | EKS Clusters | yes |
| ng | role | IAM Roles | no |
| ng | asg | Auto Scaling Groups | yes |
| ng | ec2 | EC2 Instances | no |
| ng | sg | Security Groups | no |
| ng | ami | AMI | no |
| ng | ebs | EBS Volumes | no |
| ng | subnet | Subnets | no |
| ng | ct-events | CloudTrail Events | no |
| elb | tg | Target Groups | yes |
| elb | alarm | CW Alarms | yes |
| elb | sg | Security Groups | no |
| elb | vpc | VPC | no |
| elb | cfn | CloudFormation | no |
| elb | acm | ACM Certificates | no |
| elb | cf | CloudFront | no |
| elb | eni | Network Interfaces | yes |
| elb | s3 | S3 Buckets | no |
| elb | subnet | Subnets | no |
| elb | waf | WAF Web ACLs | no |
| elb | ct-events | CloudTrail Events | no |
| tg | elb | Load Balancers | no |
| tg | ecs-svc | ECS Services | yes |
| tg | asg | Auto Scaling Groups | yes |
| tg | alarm | CW Alarms | yes |
| tg | vpc | VPC | no |
| tg | cfn | CloudFormation | no |
| tg | ec2 | EC2 Instances | no |
| tg | lambda | Lambda Functions | no |
| tg | ct-events | CloudTrail Events | no |
| sg | vpc | VPC | no |
| sg | ec2 | EC2 Instances | yes |
| sg | eni | Network Interfaces | yes |
| sg | elb | Load Balancers | yes |
| sg | lambda | Lambda Functions | yes |
| sg | cfn | CloudFormation | no |
| sg | sg | Referenced SGs | no |
| sg | ct-events | CloudTrail Events | no |
| vpc | subnet | Subnets | yes |
| vpc | sg | Security Groups | yes |
| vpc | ec2 | EC2 Instances | yes |
| vpc | elb | Load Balancers | yes |
| vpc | nat | NAT Gateways | yes |
| vpc | igw | Internet Gateways | yes |
| vpc | rtb | Route Tables | yes |
| vpc | vpce | VPC Endpoints | yes |
| vpc | cfn | CloudFormation | no |
| vpc | eni | Network Interfaces | yes |
| vpc | tgw | Transit Gateways | no |
| vpc | ct-events | CloudTrail Events | no |
| subnet | ec2 | EC2 Instances | yes |
| subnet | eni | Network Interfaces | yes |
| subnet | nat | NAT Gateways | yes |
| subnet | elb | Load Balancers | yes |
| subnet | rtb | Route Tables | yes |
| subnet | cfn | CloudFormation | no |
| subnet | vpc | VPC | no |
| subnet | asg | Auto Scaling Groups | yes |
| subnet | efs | EFS File Systems | no |
| subnet | eks | EKS Clusters | yes |
| subnet | vpce | VPC Endpoints | yes |
| subnet | ct-events | CloudTrail Events | no |
| rtb | subnet | Subnets | yes |
| rtb | nat | NAT Gateways | no |
| rtb | igw | Internet Gateways | no |
| rtb | cfn | CloudFormation | yes |
| rtb | vpc | VPC | no |
| rtb | eni | Network Interfaces | no |
| rtb | tgw | Transit Gateways | no |
| rtb | vpce | VPC Endpoints | yes |
| rtb | ct-events | CloudTrail Events | no |
| nat | vpc | VPCs | no |
| nat | subnet | Subnets | no |
| nat | rtb | Route Tables | yes |
| nat | alarm | CloudWatch Alarms | yes |
| nat | eip | Elastic IPs | no |
| nat | eni | Network Interfaces | no |
| nat | ct-events | CloudTrail Events | no |
| igw | vpc | VPCs | no |
| igw | rtb | Route Tables | yes |
| igw | ct-events | CloudTrail Events | no |
| eip | ec2 | EC2 Instances | no |
| eip | eni | Network Interfaces | no |
| eip | nat | NAT Gateways | yes |
| eip | asg | Auto Scaling Groups | no |
| eip | cfn | CloudFormation | no |
| eip | ecs | ECS Clusters | no |
| eip | ecs-svc | ECS Services | no |
| eip | ecs-task | ECS Tasks | no |
| eip | ct-events | CloudTrail Events | no |
| vpce | subnet | Subnets | no |
| vpce | sg | Security Groups | no |
| vpce | rtb | Route Tables | no |
| vpce | eni | Network Interfaces | no |
| vpce | vpc | VPC | no |
| vpce | alarm | CloudWatch Alarms | no |
| vpce | logs | Log Groups | no |
| vpce | r53 | Route 53 Zones | no |
| vpce | ct-events | CloudTrail Events | no |
| tgw | vpc | VPCs | no |
| tgw | rtb | Route Tables | yes |
| tgw | role | IAM Role | no |
| tgw | subnet | Subnets | no |
| tgw | ct-events | CloudTrail Events | no |
| eni | ec2 | EC2 Instances | no |
| eni | sg | Security Groups | no |
| eni | eip | Elastic IPs | no |
| eni | vpc | VPC | no |
| eni | subnet | Subnet | no |
| eni | elb | Load Balancers | no |
| eni | lambda | Lambda Functions | yes |
| eni | nat | NAT Gateways | yes |
| eni | vpce | VPC Endpoints | yes |
| eni | ct-events | CloudTrail Events | no |
| transfer | acm | ACM Certificates | no |
| transfer | eip | Elastic IPs | no |
| transfer | lambda | Lambda Functions | no |
| transfer | logs | Log Groups | no |
| transfer | role | IAM Roles | no |
| transfer | subnet | Subnets | no |
| transfer | vpc | VPC | no |
| transfer | vpce | VPC Endpoints | no |
| transfer | ct-events | CloudTrail Events | no |
| vpc-peer | rtb | Route Tables | yes |
| vpc-peer | vpc | VPC | yes |
| vpc-peer | ct-events | CloudTrail Events | no |
| dbi | sg | Security Groups | no |
| dbi | kms | KMS Key | no |
| dbi | subnet | Subnets | no |
| dbi | alarm | CloudWatch Alarms | yes |
| dbi | dbi-snap | DB Instance Snapshots | yes |
| dbi | logs | Log Groups | yes |
| dbi | vpc | VPC | no |
| dbi | secrets | Secrets Manager | yes |
| dbi | dbc | RDS Clusters | no |
| dbi | role | IAM Roles | no |
| dbi | eni | Network Interfaces | no |
| dbi | ct-events | CloudTrail Events | no |
| s3 | trail | CloudTrail Trails | yes |
| s3 | cf | CloudFront | yes |
| s3 | lambda | Lambda (notifications) | no |
| s3 | sns | SNS (notifications) | no |
| s3 | sqs | SQS (notifications) | no |
| s3 | cfn | CloudFormation | no |
| s3 | kms | KMS Key | no |
| s3 | s3 | Access Log Bucket | no |
| s3 | athena | Athena WorkGroups | no |
| s3 | glue | Glue Jobs | no |
| s3 | backup | Backup Plans | no |
| s3 | eb-rule | EventBridge Rules | no |
| s3 | r53 | Route 53 | no |
| s3 | role | IAM Roles | no |
| s3 | ct-events | CloudTrail Events | no |
| redis | alarm | CW Alarms | yes |
| redis | cfn | CloudFormation | yes |
| redis | ct-events | CloudTrail Events | no |
| redis | kms | KMS Key | no |
| redis | logs | Log Groups | yes |
| redis | secrets | Secrets Manager | yes |
| redis | sg | Security Groups | yes |
| redis | sns | SNS Topics | yes |
| redis | subnet | Subnets | yes |
| redis | vpc | VPC | no |
| dbc | sg | Security Groups | no |
| dbc | alarm | CloudWatch Alarms | yes |
| dbc | logs | Log Groups | yes |
| dbc | kms | KMS Key | no |
| dbc | secrets | Secrets Manager | yes |
| dbc | dbi | RDS Instances | yes |
| dbc | dbc-snap | DB Cluster Snapshots | yes |
| dbc | subnet | Subnets | no |
| dbc | vpc | VPC | no |
| dbc | ct-events | CloudTrail Events | no |
| ddb | kms | KMS Key | no |
| ddb | alarm | CloudWatch Alarms | yes |
| ddb | lambda | Lambda Functions | no |
| ddb | kinesis | Kinesis Streams | no |
| ddb | backup | Backup Plans | no |
| ddb | vpce | VPC Endpoints | yes |
| ddb | ct-events | CloudTrail Events | no |
| opensearch | alarm | CW Alarms | yes |
| opensearch | logs | Log Groups | no |
| opensearch | sg | Security Groups | no |
| opensearch | vpc | VPC | no |
| opensearch | kms | KMS Key | no |
| opensearch | cfn | CloudFormation | no |
| opensearch | subnet | Subnets | no |
| opensearch | acm | ACM Certificates | yes |
| opensearch | ct-events | CloudTrail Events | no |
| redshift | alarm | CW Alarms | yes |
| redshift | sg | Security Groups | no |
| redshift | vpc | VPC | no |
| redshift | role | IAM Role | no |
| redshift | kms | KMS Key | no |
| redshift | cfn | CloudFormation | yes |
| redshift | secrets | Secrets Manager | yes |
| redshift | logs | Log Groups | no |
| redshift | s3 | S3 Buckets | no |
| redshift | subnet | Subnets | no |
| redshift | ct-events | CloudTrail Events | no |
| efs | kms | KMS Keys | no |
| efs | cfn | CloudFormation Stacks | yes |
| efs | sg | Security Groups | no |
| efs | subnet | Subnets | no |
| efs | lambda | Lambda Functions | no |
| efs | alarm | CloudWatch Alarms | yes |
| efs | backup | Backup Plans | yes |
| efs | ecs-task | ECS Tasks | yes |
| efs | eni | Network Interfaces | yes |
| efs | vpc | VPC | yes |
| efs | ct-events | CloudTrail Events | no |
| dbi-snap | dbi | DB Instances | yes |
| dbi-snap | kms | KMS Keys | yes |
| dbi-snap | backup | Backup Plans | no |
| dbi-snap | ct-events | CloudTrail Events | no |
| dbc-snap | dbc | DB Cluster | yes |
| dbc-snap | kms | KMS Key | no |
| dbc-snap | vpc | VPC | no |
| dbc-snap | backup | Backup Plans | no |
| dbc-snap | ct-events | CloudTrail Events | no |
| alarm | sns | SNS Topics | yes |
| alarm | asg | Auto Scaling Groups | yes |
| alarm | apigw | API Gateways | yes |
| alarm | cb | CodeBuild Projects | yes |
| alarm | dbi | RDS Instances | yes |
| alarm | ec2 | EC2 Instances | yes |
| alarm | ecs | ECS Clusters | yes |
| alarm | eks | EKS Clusters | yes |
| alarm | kms | KMS Keys | yes |
| alarm | lambda | Lambda Functions | yes |
| alarm | logs | Log Groups | yes |
| alarm | s3 | S3 Buckets | yes |
| alarm | sfn | Step Functions | yes |
| alarm | waf | WAF Web ACLs | yes |
| alarm | ct-events | CloudTrail Events | no |
| logs | lambda | Lambda Functions | yes |
| logs | alarm | CW Alarms | yes |
| logs | kms | KMS Key | no |
| logs | apigw | API Gateway | yes |
| logs | ecs-task | ECS Tasks | yes |
| logs | kinesis | Kinesis Streams | no |
| logs | s3 | S3 (exports) | no |
| logs | ct-events | CloudTrail Events | no |
| trail | s3 | S3 Bucket | yes |
| trail | logs | Log Groups | yes |
| trail | sns | SNS Topic | yes |
| trail | kms | KMS Key | yes |
| trail | role | IAM Role | no |
| trail | ct-events | CloudTrail Events | no |
| ct-events | role | IAM Roles | no |
| ct-events | iam-user | IAM Users | no |
| ct-events | ec2 | EC2 Instances | no |
| ct-events | s3 | S3 Buckets | no |
| ct-events | lambda | Lambda Functions | no |
| ct-events | dbi | RDS Instances | no |
| ct-events | kms | KMS Keys | no |
| ct-events | secrets | Secrets | no |
| ct-events | vpce | VPC Endpoints | no |
| ct-events | sg | Security Groups | no |
| ct-events | ddb | DynamoDB Tables | no |
| ct-events | ecr | ECR Repositories | no |
| ct-events | cfn | CloudFormation Stacks | no |
| ct-events | trail | CloudTrail Trails | no |
| ct-events | ct-events | CT events by AccessKeyId | no |
| ct-events | ct-events | CT events by Username | no |
| ct-events | ct-events | CT events by EventName | no |
| sqs | alarm | CloudWatch Alarms | yes |
| sqs | lambda | Lambda Functions | no |
| sqs | sqs | Dead Letter Queues | yes |
| sqs | sns-sub | SNS Subscriptions | yes |
| sqs | sns | SNS Topics | yes |
| sqs | eb-rule | EventBridge Rules | yes |
| sqs | kms | KMS Key | no |
| sqs | ct-events | CloudTrail Events | no |
| sns | alarm | CloudWatch Alarms | no |
| sns | sns-sub | Subscriptions | yes |
| sns | kms | KMS Key | no |
| sns | role | IAM Role | no |
| sns | ct-events | CloudTrail Events | no |
| sns-sub | sns | SNS Topic | yes |
| sns-sub | lambda | Lambda Function | yes |
| sns-sub | sqs | SQS Queue | yes |
| sns-sub | ct-events | CloudTrail Events | no |
| eb | cfn | CloudFormation Stack | yes |
| eb | logs | Log Groups | yes |
| eb | asg | Auto Scaling Groups | yes |
| eb | ec2 | EC2 Instances | yes |
| eb | alarm | CloudWatch Alarms | yes |
| eb | elb | Load Balancers | no |
| eb | tg | Target Groups | no |
| eb | sg | Security Groups | no |
| eb | role | IAM Role | no |
| eb | s3 | S3 Buckets | no |
| eb | ct-events | CloudTrail Events | no |
| eb-rule | role | IAM Role | no |
| eb-rule | kinesis | Kinesis (targets) | no |
| eb-rule | lambda | Lambda (targets) | no |
| eb-rule | logs | Log Groups (targets) | no |
| eb-rule | sfn | Step Functions (targets) | no |
| eb-rule | sns | SNS (targets) | no |
| eb-rule | sqs | SQS (targets) | no |
| eb-rule | ct-events | CloudTrail Events | no |
| kinesis | alarm | CW Alarms | yes |
| kinesis | lambda | Lambda Functions | yes |
| kinesis | cfn | CloudFormation | no |
| kinesis | ddb | DynamoDB Tables | yes |
| kinesis | kms | KMS Key | no |
| kinesis | ct-events | CloudTrail Events | no |
| msk | alarm | CW Alarms | yes |
| msk | sg | Security Groups | no |
| msk | kms | KMS Key | no |
| msk | lambda | Lambda Functions | yes |
| msk | cfn | CloudFormation | yes |
| msk | subnet | Subnets | no |
| msk | vpc | VPC | yes |
| msk | logs | Log Groups | no |
| msk | s3 | S3 (broker logs) | no |
| msk | secrets | Secrets Manager | no |
| msk | ct-events | CloudTrail Events | no |
| sfn | alarm | CloudWatch Alarms | no |
| sfn | logs | Log Groups | no |
| sfn | role | IAM Role | no |
| sfn | eb-rule | EventBridge Rules | yes |
| sfn | kms | KMS Key | no |
| sfn | lambda | Lambda Functions | no |
| sfn | ct-events | CloudTrail Events | no |
| ses | r53 | Route 53 (DNS) | yes |
| ses | eb-rule | EventBridge Rules | yes |
| ses | lambda | Lambda Functions | no |
| ses | s3 | S3 Buckets | no |
| ses | sns | SNS Topics | no |
| ses | ct-events | CloudTrail Events | no |
| secrets | kms | KMS Keys | yes |
| secrets | lambda | Lambda (rotation) | yes |
| secrets | cfn | CloudFormation | yes |
| secrets | dbi | RDS Instances | yes |
| secrets | cb | CodeBuild Projects | yes |
| secrets | codeartifact | CodeArtifact Repositories | no |
| secrets | eb | Elastic Beanstalk | yes |
| secrets | ecs-task | ECS Tasks | yes |
| secrets | logs | Log Groups | no |
| secrets | role | IAM Roles | no |
| secrets | sns | SNS Topics | no |
| secrets | ct-events | CloudTrail Events | no |
| ssm | kms | KMS Key | yes |
| ssm | ct-events | CloudTrail Events | no |
| kms | ebs | EBS Volumes | yes |
| kms | dbi | RDS Instances | yes |
| kms | secrets | Secrets Manager | yes |
| kms | role | IAM Roles (grants) | no |
| kms | ct-events | CloudTrail Events | no |
| r53 | elb | Load Balancers | yes |
| r53 | cf | CloudFront | yes |
| r53 | acm | ACM Certificates | no |
| r53 | apigw | API Gateways | yes |
| r53 | logs | Log Groups | no |
| r53 | s3 | S3 Buckets | yes |
| r53 | vpc | VPCs | no |
| r53 | ct-events | CloudTrail Events | no |
| cf | s3 | S3 Buckets | yes |
| cf | elb | Load Balancers (origin) | yes |
| cf | waf | WAF Web ACLs | yes |
| cf | acm | ACM Certificates | yes |
| cf | r53 | Route 53 Zones | no |
| cf | alarm | CloudWatch Alarms | yes |
| cf | lambda | Lambda@Edge | no |
| cf | ct-events | CloudTrail Events | no |
| acm | cf | CloudFront Distros | yes |
| acm | elb | Load Balancers | no |
| acm | apigw | API Gateways | no |
| acm | r53 | Route 53 Zones | no |
| acm | ct-events | CloudTrail Events | no |
| apigw | logs | Log Groups | yes |
| apigw | lambda | Lambda Functions | no |
| apigw | acm | ACM Certificates | no |
| apigw | alarm | CloudWatch Alarms | yes |
| apigw | cf | CloudFront | no |
| apigw | elb | Load Balancers | no |
| apigw | kms | KMS Keys | no |
| apigw | role | IAM Role | no |
| apigw | ct-events | CloudTrail Events | no |
| role | lambda | Lambda Functions | yes |
| role | glue | Glue Jobs | yes |
| role | ng | Node Groups | yes |
| role | policy | IAM Policies | no |
| role | ec2 | EC2 Instances | yes |
| role | eks | EKS Clusters | yes |
| role | iam-group | IAM Groups (trust) | no |
| role | iam-user | IAM Users (trust) | no |
| role | ct-events | CloudTrail Events | no |
| policy | role | IAM Roles | no |
| policy | iam-user | IAM Users | no |
| policy | iam-group | IAM Groups | no |
| policy | ct-events | CloudTrail Events | no |
| iam-user | iam-group | IAM Groups | no |
| iam-user | policy | IAM Policies | no |
| iam-user | ct-events | CloudTrail Events | no |
| iam-group | iam-user | IAM Users | no |
| iam-group | policy | IAM Policies | no |
| iam-group | ct-events | CloudTrail Events | no |
| waf | elb | Load Balancers | no |
| waf | apigw | API Gateways | no |
| waf | cf | CloudFront | no |
| waf | alarm | CloudWatch Alarms | no |
| waf | logs | Log Groups | no |
| waf | ct-events | CloudTrail Events | no |
| cfn | role | IAM Roles | no |
| cfn | cfn | Related Stacks | yes |
| cfn | sns | SNS Topics | no |
| cfn | s3 | S3 (stack resources) | no |
| cfn | eb-rule | EventBridge Rules | no |
| cfn | ct-events | CloudTrail Events | no |
| pipeline | cb | CodeBuild Projects | no |
| pipeline | role | IAM Roles | no |
| pipeline | cfn | CloudFormation | no |
| pipeline | eb-rule | EventBridge Rules | no |
| pipeline | ecr | ECR Repositories | no |
| pipeline | ecs-svc | ECS Services | no |
| pipeline | kms | KMS Key | no |
| pipeline | lambda | Lambda Functions | no |
| pipeline | s3 | S3 Buckets (artifacts) | no |
| pipeline | sns | SNS Topics | no |
| pipeline | ct-events | CloudTrail Events | no |
| cb | logs | Log Groups | yes |
| cb | role | IAM Roles | no |
| cb | pipeline | CodePipelines | yes |
| cb | sg | Security Groups | no |
| cb | subnet | Subnets | no |
| cb | vpc | VPC | no |
| cb | kms | KMS Key | no |
| cb | alarm | CloudWatch Alarms | yes |
| cb | ecr | ECR Repositories | yes |
| cb | s3 | S3 Buckets | yes |
| cb | secrets | Secrets Manager | no |
| cb | ssm | SSM Parameters | no |
| cb | ct-events | CloudTrail Events | no |
| ecr | lambda | Lambda Functions | yes |
| ecr | cb | CodeBuild Projects | yes |
| ecr | cfn | CloudFormation Stacks | yes |
| ecr | kms | KMS Key | no |
| ecr | ct-events | CloudTrail Events | no |
| ecr | eb-rule | EventBridge Rules | yes |
| ecr | ecs-task | ECS Tasks | yes |
| ecr | pipeline | CodePipelines | yes |
| ecr | role | IAM Roles | no |
| codeartifact | kms | KMS Key | no |
| codeartifact | ct-events | CloudTrail Events | no |
| glue | role | IAM Roles | no |
| glue | alarm | CW Alarms | yes |
| glue | logs | Log Groups | yes |
| glue | cfn | CloudFormation Stacks | no |
| glue | s3 | S3 (script bucket) | no |
| glue | kms | KMS Key | no |
| glue | secrets | Secrets Manager | no |
| glue | ct-events | CloudTrail Events | no |
| athena | s3 | S3 Buckets (results) | no |
| athena | kms | KMS Keys | no |
| athena | logs | Log Groups | no |
| athena | role | IAM Roles | no |
| athena | ct-events | CloudTrail Events | no |
| mwaa | alarm | CW Alarms | yes |
| mwaa | kms | KMS Key | no |
| mwaa | logs | Log Groups | no |
| mwaa | role | IAM Roles | no |
| mwaa | s3 | S3 Buckets | no |
| mwaa | sg | Security Groups | no |
| mwaa | subnet | Subnets | no |
| mwaa | ct-events | CloudTrail Events | no |
| backup | role | IAM Roles | no |
| backup | kms | KMS Keys | no |
| backup | sns | SNS Topics | no |
| backup | ct-events | CloudTrail Events | no |
<!-- END GENERATED: related-table -->
