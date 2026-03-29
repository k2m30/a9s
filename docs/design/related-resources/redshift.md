# Redshift Clusters (redshift) — Related Resources

## Real-World Use Cases

**1. "Who has access to this data warehouse?"** Check the Redshift cluster's IAM roles (for COPY/UNLOAD operations), security groups (network access), and any Secrets Manager secrets storing credentials.

**2. "Is this cluster's audit logging enabled?"** Redshift can log to S3 or CloudWatch Logs. You need to check the logging configuration to find where audit and connection logs go.

**3. "Why are queries slow?"** You need CloudWatch metrics (CPUUtilization, PercentageDiskSpaceUsed, ReadIOPS, QueuedQueries) and the Redshift system tables (accessible only via SQL, not API). For the API-accessible information, check the cluster's parameter group for WLM configuration.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CloudWatch Alarms (alarm) | Search alarms with `ClusterIdentifier` dimension. | "What monitoring watches this cluster?" Alarms on disk usage, CPU, and query queue depth. | P0 |
| Glue Jobs (glue) | Glue jobs that use JDBC connections to Redshift. Requires checking Glue connections for Redshift endpoints. | "Which ETL jobs load data into this cluster?" | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this cluster?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| S3 Bucket (s3) — Audit Logs | `redshift:DescribeLoggingStatus` with `ClusterIdentifier` returns `BucketName` and `S3KeyPrefix` if S3 audit logging is enabled. | "Where are the audit logs?" Compliance and security investigation. | P1 |
| Secrets Manager (secrets) | Heuristic: Search secrets by name pattern (`redshift-{cluster}`, `{app}/redshift`) or search secret values for the cluster endpoint. | "What are the credentials?" | P1 |
| Security Groups (sg) | Cluster response has `VpcSecurityGroups[].VpcSecurityGroupId` — FORWARD. | "Who can connect to this cluster?" Port 5439. | P1 |
| IAM Roles (role) | Cluster response has `IamRoles[].IamRoleArn` — FORWARD. These roles are used for COPY/UNLOAD to S3, Glue catalog access, and Lambda UDFs. | "What AWS resources can this cluster access?" | P1 |
| Snapshots (not in a9s) | `redshift:DescribeClusterSnapshots` with `ClusterIdentifier`. | "When was the last backup?" | P1 |
| Subnets (subnet) | Via subnet group: cluster has `ClusterSubnetGroupName` → `redshift:DescribeClusterSubnetGroups`. | "Which AZs is this cluster in?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteCluster | "Who deleted this cluster?" Data loss if no final snapshot. |
| ModifyCluster | "Who changed node type, count, or other settings?" Resize operations affect availability. |
| ResizeCluster | "Who triggered a resize?" Classic resize takes the cluster offline; elastic resize is faster but still impacts performance. |
