# CloudWatch Log Groups (logs) — Related Resources

## Real-World Use Cases

**1. "Which service writes to this log group?"** Log groups receive data from many sources — Lambda, ECS, RDS, EKS, API Gateway, CodeBuild, custom applications. The log group itself doesn't store who creates it. You need to parse the log group name or check tags to identify the source service.

**2. "Where do these logs go after CloudWatch?"** Log groups can have subscription filters that stream data to Lambda, Kinesis, or OpenSearch for further processing. These are the data pipelines built on top of logs.

**3. "Why is our CloudWatch bill so high?"** Log group ingestion and storage costs add up. You need to check retention settings (are we keeping logs forever?) and identify which log groups ingest the most data.

**4. "Can I search across this log group for a specific error?"** This is a child view concern (log streams → log events), but knowing which log group to search requires understanding the source resource first.

## Reverse Relationships

Log groups are the recipients of data from many services. Most services write TO log groups, but the log group has no pointer back.

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Lambda Functions (lambda) | Lambda writes to `/aws/lambda/{function-name}` by default. Parse the log group name. Or search Lambda functions whose `LoggingConfig.LogGroup` matches this group. | "Which Lambda writes here?" The most common log group → source navigation. | P0 |
| ECS Services (ecs-svc) | ECS tasks write to log groups specified in task definition `logConfiguration`. Search task definitions for `awslogs-group` matching this log group name. | "Which ECS service writes here?" | P1 |
| RDS Instances (dbi) | RDS writes to `/aws/rds/instance/{db-id}/{log-type}`. Parse the log group name to extract the DB instance ID. | "Which database writes here?" | P1 |
| EKS Clusters (eks) | EKS control plane writes to `/aws/eks/{cluster-name}/cluster`. Parse the log group name. | "Which EKS cluster writes here?" | P1 |
| CodeBuild Projects (cb) | CodeBuild writes to log groups specified in project config `logsConfig.cloudWatchLogs.groupName`. Parse or search CodeBuild projects. | "Which CodeBuild project writes here?" | P1 |
| API Gateway (apigw) | API GW access logs go to a configured log group. Parse the log group ARN from API stage settings. | "Which API writes here?" | P2 |
| VPC Flow Logs (not in a9s) | `ec2:DescribeFlowLogs` with `Filters=[{Name=log-destination, Values=[arn matching this log group]}]`. | "Which VPC sends flow logs here?" | P2 |
| CloudTrail Trails (trail) | Search trails whose `CloudWatchLogsLogGroupArn` matches this log group. | "Does CloudTrail deliver events here?" | P1 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Source Resource (parse name) | Parse the log group name using known AWS naming conventions: `/aws/lambda/{name}` → Lambda, `/aws/rds/instance/{id}/{type}` → RDS, `/aws/rds/cluster/{id}/{type}` → DocDB/Aurora, `/aws/eks/{cluster}/cluster` → EKS, `/aws/codebuild/{project}` → CodeBuild, `/aws/apigateway/{id}` → API GW, `/aws/docdb/{cluster}/{type}` → DocumentDB, `/aws-glue/jobs/{type}` → Glue, `/aws/elasticache/{group}/{type}` → ElastiCache. For non-standard names, fall back to tags (`aws:resource:name`) or heuristics. | "What writes to this log group?" The log group name is the primary navigation clue. | P0 |
| Subscription Filters → Lambda/Kinesis/OpenSearch | `logs:DescribeSubscriptionFilters` with `logGroupName`. Returns filter pattern + destination ARN (Lambda, Kinesis stream, or OpenSearch domain). | "Where do these logs flow to?" Subscription filters are the data pipeline — logs → Lambda for alerting, logs → Kinesis → S3 for archival, logs → OpenSearch for search. | P1 |
| Metric Filters → CloudWatch Metrics | `logs:DescribeMetricFilters` with `logGroupName`. Returns filter patterns and the CloudWatch metrics they publish. | "Are any custom metrics derived from these logs?" Metric filters create CloudWatch metrics from log patterns (e.g., count of "ERROR" strings → custom alarm). | P1 |
| S3 Export | `logs:DescribeExportTasks` — shows if log data was exported to S3. | "Has this log data been archived?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteLogGroup | "Who deleted this log group and all its data?" Immediate, irreversible data loss. Often millions of log lines gone. |
| PutRetentionPolicy / DeleteRetentionPolicy | "Who changed log retention?" Setting retention to 1 day deletes most historical data. Removing the policy means logs are kept forever (cost explosion). |
| PutSubscriptionFilter / DeleteSubscriptionFilter | "Who changed the log streaming pipeline?" Removing a subscription filter breaks downstream log processing (alerting, archival, search). |
