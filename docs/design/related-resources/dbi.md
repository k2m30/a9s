# DB Instances (dbi) — Related Resources

## Real-World Use Cases

**1. "Why can't my application connect to this database?"** You check the RDS instance — it's running, endpoint looks right. Now you need to trace the network path: which security group controls inbound access? Is the DB in a private subnet? Does the application's SG allow outbound to the DB's port? Multiple hops through SGs and subnets.

**2. "When was the last backup and can we restore?"** The DB instance shows automated backup retention, but you need to see the actual snapshots — how recent is the latest one? Are there manual snapshots for pre-migration safety?

**3. "Was there a failover last night?"** The DB shows it's Multi-AZ, but the current state tells you nothing about past events. You need RDS Events (a separate API) to see failover notifications, maintenance windows, and storage scaling.

**4. "Which applications use this database?"** You need to find Secrets Manager secrets containing this DB's endpoint, Lambda functions with this endpoint in their environment variables, and ECS task definitions referencing it. There's no single API for this.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CloudWatch Alarms (alarm) | Search alarms with `DBInstanceIdentifier` dimension matching this instance ID. Common alarms: CPUUtilization, FreeableMemory, DatabaseConnections, FreeStorageSpace, ReplicaLag. | "What monitoring watches this DB?" During incidents, check if alarms already fired. | P0 |
| RDS Snapshots (rds-snap) | `rds:DescribeDBSnapshots` with `DBInstanceIdentifier` filter. Returns both automated and manual snapshots. | "When was the last backup? Are there pre-migration safety snapshots?" | P0 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which IaC stack manages this DB?" | P2 |
| Backup Recovery Points (not in a9s) | `backup:ListRecoveryPointsByResource` with this DB instance's ARN. | "Is this DB in an AWS Backup plan?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CloudWatch Log Groups (logs) | RDS publishes logs to `/aws/rds/instance/{db-instance-id}/{log-type}` where log-type is `error`, `general`, `slowquery`, `audit` (MySQL/MariaDB) or `postgresql` (PostgreSQL). Only exists if the corresponding log export is enabled in the DB instance's `EnabledCloudwatchLogsExports`. | "Show me the database error log." Critical for debugging connection errors, deadlocks, slow queries. | P0 |
| Secrets Manager (secrets) | Heuristic search: (1) Check secrets tagged with `aws:rds:primaryCluster` or `aws:rds:db-instance`. (2) Search secrets whose `SecretString` JSON contains this DB's endpoint address. (3) Search by naming convention — secrets often named `rds-{instance-name}` or `{app-name}/database`. If a9s has secrets data cached, search in-memory. | "Where are the credentials for this database?" Engineers always need the password — especially during incident response when connecting via CLI. | P0 |
| Read Replicas (dbi) | DB instance response has `ReadReplicaDBInstanceIdentifiers[]` — FORWARD (for the primary). Read replicas have `ReadReplicaSourceDBInstanceIdentifier` — also FORWARD. Both directions available in the API. | "What replicas exist for this primary?" or "What is this replica's primary?" For understanding replication topology. | P1 |
| Security Groups (sg) | DB response has `VpcSecurityGroups[].VpcSecurityGroupId` — FORWARD. Navigate to SGs to verify inbound rules allow the application's source (SG or CIDR) on the DB port (3306, 5432, etc.). | "Why can't the app connect?" Check if the SG allows inbound from the application's SG. | P1 |
| Subnets (subnet) | DB response has `DBSubnetGroup.Subnets[].SubnetIdentifier` — FORWARD. Shows which AZs the DB can failover to. | "Which AZs is this DB in?" For Multi-AZ, the secondary AZ is where failover goes. | P1 |
| KMS Key (kms) | If encrypted, DB has `KmsKeyId` — FORWARD. | "Who can access this DB's encryption key?" | P2 |
| Parameter Group (not in a9s) | DB response has `DBParameterGroups[].DBParameterGroupName` — FORWARD. Parameter groups control DB engine configuration (max_connections, slow_query_log, etc.). | "What non-default DB parameters are set?" Debugging performance or behavior differences. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteDBInstance | "Who deleted this database?" Catastrophic if no final snapshot was taken. Shows whether `SkipFinalSnapshot` was true. |
| ModifyDBInstance | "Who changed instance class, storage, or parameters?" Performance issues after a modification — was the instance type downgraded? |
| RebootDBInstance | "Who rebooted the database?" A reboot causes a brief outage — was it intentional maintenance or an accident? |
