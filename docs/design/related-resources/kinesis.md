# Kinesis Data Streams (kinesis) — Related Resources

## Real-World Use Cases

**1. "What consumes data from this stream?"** The stream has producers pushing data in and consumers pulling it out. Consumers include Lambda event source mappings, Kinesis Data Firehose delivery streams, and custom KCL applications. The stream API tells you nothing about consumers.

**2. "Why is the stream throttling?"** The stream config shows shard count and capacity mode, but you need CloudWatch metrics (ReadProvisionedThroughputExceeded, WriteProvisionedThroughputExceeded) to see actual throughput vs limits.

**3. "Which log groups are streaming to this Kinesis stream?"** CloudWatch Log subscription filters can send data to Kinesis for processing or archival.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| Lambda Functions (lambda) | `lambda:ListEventSourceMappings` with `EventSourceArn` matching this stream's ARN. | "Which Lambdas process data from this stream?" The primary consumer pattern. | P0 |
| CloudWatch Log Groups (logs) | Requires iterating log groups and checking `logs:DescribeSubscriptionFilters` for destinations matching this stream's ARN. Expensive. If a9s has log group data cached, scan subscription filter destinations in-memory. | "Which log groups stream to this Kinesis stream?" | P1 |
| CloudWatch Alarms (alarm) | Search alarms with `StreamName` dimension. | "What monitoring watches this stream?" Throttling and iterator age alarms. | P1 |
| CloudFormation Stacks (cfn) | Check tags. | "Which stack manages this stream?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Kinesis Data Firehose (not in a9s) | `firehose:ListDeliveryStreams` then `firehose:DescribeDeliveryStream` for each — check `Source.KinesisStreamSourceDescription.KinesisStreamARN` for matches. | "Does this stream feed into a Firehose delivery stream?" Firehose → S3/Redshift/OpenSearch is a common pattern. | P1 |
| KMS Key (kms) | Stream response has `EncryptionType` and `KeyId` — FORWARD (if server-side encryption is enabled). | "Who can read data in this stream?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteStream | "Who deleted this stream?" All data in the stream is lost. All consumers break. |
| UpdateShardCount | "Who scaled the stream?" Shard count changes affect both throughput and cost. Shard splitting/merging temporarily makes some shards read-only. |
| StartStreamEncryption / StopStreamEncryption | "Who changed encryption settings?" Stopping encryption on a sensitive data stream is a security event. |
