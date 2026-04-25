# DocumentDB Clusters (dbc) — Related Resources

## Real-World Use Cases

**1. "Why can't the application connect to DocumentDB?"** Same pattern as RDS — check security groups, subnet reachability, and whether the application is in the same VPC or has cross-VPC access.

**2. "Was there a failover?"** DocumentDB is Multi-AZ. You need to check cluster events for failover notifications and see which instance is currently the writer.

**3. "Where are the credentials?"** DocumentDB credentials are often in Secrets Manager, following RDS-like patterns.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CloudWatch Alarms (alarm) | Search alarms with `DBClusterIdentifier` dimension. | "What monitoring watches this cluster?" Common alarms: CPUUtilization, FreeableMemory, DatabaseConnections. | P0 |
| DB Cluster Snapshots (dbc-snap) | `docdb:DescribeDBClusterSnapshots` with `DBClusterIdentifier` filter. | "When was the last backup?" | P1 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this cluster?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| CloudWatch Log Groups (logs) | DocumentDB publishes audit and profiler logs to `/aws/docdb/{cluster-id}/audit` and `/aws/docdb/{cluster-id}/profiler`. Only if `EnabledCloudwatchLogsExports` includes `audit` or `profiler`. | "Show me slow operations or audit trail." The profiler log shows operations exceeding the threshold — critical for performance debugging. | P0 |
| Secrets Manager (secrets) | Heuristic: Search secrets tagged with `aws:docdb:cluster` or by naming convention (`docdb-{cluster}`, `{app}/docdb`). Or search secret values for the cluster endpoint. | "What are the credentials?" | P1 |
| Security Groups (sg) | Cluster response has `VpcSecurityGroups[].VpcSecurityGroupId` — FORWARD. Navigate to SGs to verify port 27017 is open from the application. | "Why can't the app connect?" | P1 |
| Subnets (subnet) | Cluster response has `DBSubnetGroup` — FORWARD. Navigate to subnets for network topology. | "Which AZs can this cluster use?" | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteDBCluster | "Who deleted this cluster?" Potential data loss. |
| FailoverDBCluster | "Who triggered a manual failover?" Failovers cause brief connection drops. |
| ModifyDBCluster | "Who changed cluster settings?" Parameter or engine version changes. |
