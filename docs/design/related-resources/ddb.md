# DynamoDB Tables (ddb) — Related Resources

## Real-World Use Cases

**1. "Why is this table throttling?"** The table config shows capacity mode (on-demand vs provisioned) and GSI configuration, but you need CloudWatch metrics to see actual consumed capacity vs provisioned. And you need to check if Application Auto Scaling is configured to handle load spikes.

**2. "What Lambda functions react to changes in this table?"** DynamoDB Streams trigger Lambda functions via event source mappings. The table knows it has a stream enabled, but doesn't know which Lambdas consume it.

**3. "Is this table backed up?"** Check for both DynamoDB's native PITR (point-in-time recovery) and AWS Backup plans that protect this table.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Lambda Functions (lambda) | `lambda:ListEventSourceMappings` with `EventSourceArn` matching this table's `LatestStreamArn`. Returns all Lambda functions triggered by this table's stream. | "What happens when data changes in this table?" DynamoDB Streams + Lambda is a core event-driven pattern. | P0 |
| CloudWatch Alarms (alarm) | Search alarms with `TableName` dimension. Critical alarms: ConsumedReadCapacityUnits, ConsumedWriteCapacityUnits, ThrottledRequests, SystemErrors. | "What monitoring watches this table?" Throttling alarms are especially important for provisioned tables. | P0 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this table?" | P2 |
| Backup Plans (backup) | `backup:ListProtectedResources` and check for this table's ARN. Or search backup selections that include this table's tags or ARN. | "Is this table in an AWS Backup plan?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Application Auto Scaling (not in a9s) | `application-autoscaling:DescribeScalableTargets` with `ServiceNamespace=dynamodb`, `ResourceIds=table/{table-name}`. Also check GSI targets: `table/{table-name}/index/{index-name}`. Returns scaling min/max. Then `DescribeScalingPolicies` for target tracking policies. | "Is auto scaling configured? What are the limits?" If not configured on a provisioned table, throttling during traffic spikes is guaranteed. | P1 |
| DynamoDB Stream (via table response) | Table response has `StreamEnabled` and `LatestStreamArn` — FORWARD. But the value is connecting to the Lambda triggers that consume the stream (see reverse relationships above). | "Is streaming enabled?" The stream itself is just a pipe — the Lambda triggers are what matter. | P1 |
| Global Table Replicas (ddb) | Table response has `Replicas[]` with region names — FORWARD. Each replica is a full DynamoDB table in another region. | "Where are this table's replicas?" DR and multi-region architecture. | P1 |
| KMS Key (kms) | Table response has `SSEDescription.KMSMasterKeyArn` — FORWARD (if using customer-managed key). | "Who controls the encryption key for this table?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteTable | "Who deleted this table and all its data?" Unlike RDS, DynamoDB table deletion is immediate with no final snapshot option. Data is lost unless PITR or backups exist. |
| UpdateTable | "Who changed capacity, added/removed a GSI, or changed billing mode?" GSI additions consume significant write capacity during backfill. |
| UpdateContinuousBackups | "Who enabled or disabled PITR?" Disabling PITR removes the safety net for accidental data deletion. |
