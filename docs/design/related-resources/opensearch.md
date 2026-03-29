# OpenSearch Domains (opensearch) — Related Resources

## Real-World Use Cases

**1. "Why is the OpenSearch cluster red?"** Cluster status green/yellow/red is in CloudWatch metrics, not the API response. You need to check FreeStorageSpace, JVMMemoryPressure, and ClusterStatus metrics. Red usually means a primary shard can't be allocated — often a disk space issue.

**2. "Which log groups are streaming data to this cluster?"** CloudWatch Log Groups can have subscription filters that send logs to OpenSearch. The OpenSearch domain has no idea who's sending data to it.

**3. "Why is Kibana inaccessible?"** If Cognito authentication is enabled, you need to check the Cognito user pool and identity pool configuration. If it's VPC-based, check the security groups on the domain's ENIs.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CloudWatch Log Group Subscriptions (logs) | Search log groups for subscription filters with this OpenSearch domain's ARN as the destination. `logs:DescribeSubscriptionFilters` per log group. Expensive — requires iterating log groups. If a9s has log group data cached, scan in-memory. | "Which log groups send data to this cluster?" Understanding the data ingest pipeline. | P1 |
| CloudWatch Alarms (alarm) | Search alarms with `DomainName` and `ClientId` dimensions. | "What monitoring watches this cluster?" Critical alarms: ClusterStatus.red, FreeStorageSpace, JVMMemoryPressure. | P0 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this domain?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CloudWatch Log Groups (logs) | Domain response has `LogPublishingOptions` — FORWARD. Each log type (SEARCH_SLOW_LOGS, INDEX_SLOW_LOGS, ES_APPLICATION_LOGS, AUDIT_LOGS) maps to a CloudWatch log group ARN. | "Where are the slow query and error logs?" The application logs show indexing errors and cluster issues. | P0 |
| Security Groups (sg) | Domain response has `VPCOptions.SecurityGroupIds` — FORWARD (VPC domains only). | "Why can't the app reach OpenSearch?" Port 443 must be open. | P1 |
| Subnets (subnet) | Domain response has `VPCOptions.SubnetIds` — FORWARD (VPC domains only). | "Which AZs is this domain in?" | P1 |
| KMS Key (kms) | Domain response has `EncryptionAtRestOptions.KmsKeyId` — FORWARD (if enabled). | "Who controls the encryption key?" | P2 |
| Cognito (not in a9s) | Domain response has `CognitoOptions.UserPoolId` and `CognitoOptions.IdentityPoolId` — FORWARD (if Cognito auth enabled). | "Why can't users access Kibana?" Cognito configuration issues block dashboard access. | P2 |
| S3 (s3) | Manual snapshot repositories use S3 for storage. The snapshot repository config (via OpenSearch API, not AWS API) contains the S3 bucket name. | "Where are this domain's snapshots stored?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteDomain | "Who deleted this OpenSearch domain?" All data is lost. |
| UpdateDomainConfig | "Who changed instance types, storage, or access policies?" Configuration changes can cause blue/green deployments that temporarily reduce cluster capacity. |
| CreateDomain | "Who created this domain and with what config?" Cost and security audit. |
