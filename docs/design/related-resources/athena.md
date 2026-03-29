# Athena Workgroups (athena) — Related Resources

## Real-World Use Cases

**1. "Where do query results go?"** The workgroup configuration specifies an S3 output location. Navigate to the bucket to see query result files and their sizes (cost indicator — Athena charges per TB scanned).

**2. "Is this workgroup enforcing cost controls?"** Check the workgroup's byte scan limit and requester-pays settings. Without limits, a single bad query can scan petabytes.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| (Minimal) | Workgroups are organizational entities. Other resources don't reference them by ARN. | — | — |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| S3 Bucket (s3) — Query Results | Workgroup config `ResultConfiguration.OutputLocation` — FORWARD. Navigate to the S3 bucket to see result sizes and manage storage. | "Where do query results go? How much storage are they using?" Query results accumulate and can become expensive. | P0 |
| KMS Key (kms) | If result encryption is enabled, `ResultConfiguration.EncryptionConfiguration.KmsKey` — FORWARD. | "Who can read query results?" | P2 |
| Glue Data Catalog (not in a9s) | Athena queries reference Glue databases and tables. The workgroup itself doesn't specify which catalog, but queries do. | "What data does this workgroup query?" Requires looking at named queries or query history. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteWorkGroup | "Who deleted this workgroup?" Named queries and settings are lost. |
| UpdateWorkGroup | "Who changed query result location or byte scan limits?" Removing scan limits can lead to huge bills. |
| StartQueryExecution | "Who ran queries and how much data was scanned?" For cost attribution — Athena charges by bytes scanned. |
