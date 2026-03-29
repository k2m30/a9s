# CloudTrail Trails (trail) — Related Resources

## Real-World Use Cases

**1. "Where do audit logs go and are they being delivered?"** The trail configuration tells you the S3 bucket, CloudWatch log group, and SNS topic. But is delivery actually working? Check the trail status and the S3 bucket for recent objects.

**2. "Is this trail covering all regions and all event types?"** Security audit — a trail that only covers one region or excludes management events has blind spots.

**3. "Who can tamper with this trail?"** If someone can stop logging or delete the trail, they can cover their tracks. Check the trail's S3 bucket policy and the IAM policies that allow CloudTrail modifications.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal) | Trails are infrastructure-level resources. Other resources don't reference them. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| S3 Bucket (s3) | Trail response has `S3BucketName` — FORWARD. Navigate to the bucket to verify it exists, check its policy, and confirm objects are being delivered. | "Where are audit logs stored?" The S3 bucket is the primary audit data store. | P0 |
| CloudWatch Log Group (logs) | Trail response has `CloudWatchLogsLogGroupArn` — FORWARD (if configured). Navigate to the log group for near-real-time event searching. | "Where can I search events in near-real-time?" CloudWatch Logs delivery enables faster event searching than S3. | P1 |
| SNS Topic (sns) | Trail response has `SnsTopicARN` — FORWARD (if configured). Navigate to the topic to see who gets notified of new log delivery. | "Who is notified when logs are delivered?" | P1 |
| KMS Key (kms) | Trail response has `KmsKeyId` — FORWARD (if encrypted). | "Who can decrypt the audit logs?" Security-sensitive — the KMS key policy controls audit log access. | P1 |
| Event Selectors | `cloudtrail:GetEventSelectors` or `cloudtrail:GetAdvancedEventSelectors` — shows which events are logged (management, data, insights) and any resource type filters. Not a resource but critical for understanding coverage. | "What does this trail actually capture?" A trail that excludes S3 data events won't show who accessed your files. | P1 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| StopLogging | "Who stopped logging?" THE most security-critical CloudTrail event. Stopping a trail is the first step in covering tracks. This event should trigger an immediate security alert. |
| DeleteTrail | "Who deleted the audit trail?" Destruction of audit infrastructure. |
| UpdateTrail | "Who changed the trail configuration?" Changing the S3 bucket or disabling log file validation could redirect or compromise audit data. |
