# MSK Clusters (msk) — Related Resources

## Real-World Use Cases

**1. "What consumes from this Kafka cluster?"** MSK is a Kafka cluster. Consumers include Lambda event source mappings, Kafka Connect connectors, and custom consumer applications. The MSK API shows cluster infrastructure but not consumer group state (that's Kafka protocol, not AWS API).

**2. "Why can't my consumer connect?"** Check the cluster's security groups for port access (9092 for plaintext, 9094 for TLS, 9096 for SASL), subnet reachability, and IAM authentication configuration.

**3. "Where do broker logs go?"** MSK can send logs to CloudWatch Logs, S3, or Kinesis Data Firehose. The configuration is in the cluster's logging settings.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Lambda Functions (lambda) | `lambda:ListEventSourceMappings` with `EventSourceArn` matching this MSK cluster's ARN. | "Which Lambdas consume from this cluster?" Lambda can poll MSK topics directly. | P0 |
| CloudWatch Alarms (alarm) | Search alarms with `Cluster Name` dimension. | "What monitoring watches this cluster?" | P1 |
| CloudFormation Stacks (cfn) | Check tags. | "Which stack manages this cluster?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CloudWatch Log Groups (logs) | Cluster config `LoggingInfo.BrokerLogs.CloudWatchLogs.LogGroup` — FORWARD (if enabled). | "Where are broker logs?" Debug Kafka broker issues. | P1 |
| S3 Bucket (s3) | Cluster config `LoggingInfo.BrokerLogs.S3.Bucket` — FORWARD (if S3 logging enabled). | "Where are archived broker logs?" | P1 |
| Security Groups (sg) | Cluster response has security group IDs — FORWARD. Navigate to SGs to verify Kafka ports (9092/9094/9096) are open. | "Why can't producers/consumers connect?" | P0 |
| Subnets (subnet) | Cluster response has client subnets — FORWARD. | "Which AZs is this cluster in?" | P1 |
| KMS Key (kms) | If encryption is configured, cluster has KMS key ID — FORWARD. | "Who controls the encryption key?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteCluster | "Who deleted this Kafka cluster?" All topics and data are lost. |
| UpdateBrokerCount | "Who scaled the cluster?" Adding brokers triggers partition rebalancing. |
| UpdateClusterConfiguration | "Who changed broker configuration?" Config changes like `auto.create.topics.enable` or `log.retention.hours` affect cluster behavior. |
