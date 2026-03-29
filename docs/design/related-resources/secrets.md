# Secrets Manager (secrets) — Related Resources

## Real-World Use Cases

**1. "Which database does this secret belong to?"** RDS-managed secrets are automatically created and rotated. The secret's tags and SecretString JSON contain the database endpoint, engine, and port. Navigate to the RDS instance to see its current state.

**2. "What uses this secret?"** ECS task definitions reference secrets by ARN, Lambda functions read them in code, and applications fetch them at startup. Finding all consumers is critical before rotation or deletion.

**3. "Who accessed this secret?"** The most security-sensitive audit question. CloudTrail `GetSecretValue` events show every access — by which principal, from which IP, at what time.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| ECS Task Definitions (not in a9s) | Search task definitions for `containerDefinitions[].secrets[].valueFrom` matching this secret's ARN or name prefix. Requires iterating task definition families. | "Which ECS tasks inject this secret?" If the secret is deleted or rotated, these tasks will fail to start. | P0 |
| Lambda Functions (lambda) | No direct API — Lambda reads secrets in application code, not via AWS resource configuration. Heuristic: check Lambda environment variables for the secret name or ARN (sometimes stored as env vars pointing to the secret). | "Which Lambdas use this secret?" Best-effort lookup. | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| RDS Instance (dbi) | Parse the secret's tags for `aws:rds:primaryCluster` or similar RDS-managed tags. Or parse `SecretString` JSON for `host`, `dbname`, `engine`, `port` fields and match the `host` against RDS instance endpoints in a9s. | "Which database does this secret connect to?" RDS-managed secrets have structured JSON — the endpoint maps directly to a DB instance. | P0 |
| Lambda Rotation Function (lambda) | Secret has `RotationLambdaARN` — FORWARD (if rotation is configured). Navigate to the Lambda to check its health and recent invocations. | "Is rotation working? Why did rotation fail?" If the rotation Lambda fails, the secret becomes stale. | P1 |
| KMS Key (kms) | Secret has `KmsKeyId` — FORWARD. | "Who can decrypt this secret?" The KMS key policy determines who can access the plaintext value, independent of IAM policies on Secrets Manager. | P1 |
| Replica Regions | Secret has `ReplicationStatus` — FORWARD (if replicated). Shows which regions have copies. | "Where are copies of this secret?" Multi-region applications need secrets in each region. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| GetSecretValue | "Who accessed this secret?" THE most important audit event for secrets. During a security investigation, this reveals every principal that read the secret value — potential credential exposure. |
| PutSecretValue / UpdateSecret | "Who rotated or changed the secret?" Unexpected changes can break applications that cache the old value. |
| DeleteSecret | "Who deleted this secret?" Secrets have a recovery window (7-30 days), but once purged, they're gone. |
