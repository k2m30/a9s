# ElastiCache Redis (redis) — Related Resources

## Real-World Use Cases

**1. "Why can't the application connect to Redis?"** You need to check the security groups on the Redis cluster — does inbound allow the application's SG on port 6379? Also check if the cluster is in the right subnets reachable from the application's VPC.

**2. "Is this Redis cluster under memory pressure?"** The cluster config shows node type and number of replicas, but you need CloudWatch metrics (DatabaseMemoryUsagePercentage, EngineCPUUtilization, CurrConnections, CacheHitRate) for actual utilization.

**3. "Where are the credentials for this Redis cluster?"** If auth is enabled, the token is often stored in Secrets Manager. You need to find the right secret.

## Reverse Relationships

| Related Resource | How to Find | Scenario | Priority |
|-----------------|-------------|----------|----------|
| CloudWatch Alarms (alarm) | Search alarms with `CacheClusterId` or `ReplicationGroupId` dimension. | "What monitoring watches this Redis?" Common alarms: memory usage, CPU, evictions, replication lag. | P0 |
| CloudFormation Stacks (cfn) | Check for `aws:cloudformation:stack-name` tag. | "Which stack manages this cluster?" | P2 |

## Algorithmic Relationships

| Related Resource | Algorithm | Scenario | Priority |
|-----------------|-----------|----------|----------|
| Security Groups (sg) | Redis response has security group IDs in its configuration — FORWARD. Navigate to SGs to verify inbound rules allow traffic on port 6379 (or 6380 for TLS) from the application's security group. | "Why can't the app connect?" SG misconfiguration is the #1 cause of Redis connectivity failures. | P0 |
| Secrets Manager (secrets) | Heuristic: Search secrets by name pattern (e.g., `redis-{cluster-name}`, `{app}/redis-auth`). Or search secret values for the Redis endpoint. Also check if the cluster has `AuthTokenEnabled=true` — if so, the token must be stored somewhere. | "What's the auth token for this cluster?" Needed for CLI access during debugging. | P1 |
| CloudWatch Log Groups (logs) | If slow log delivery to CloudWatch is configured: `/aws/elasticache/{replication-group-id}/slow-log`. Check `LogDeliveryConfigurations` in the cluster response for the actual destination. | "Are there slow queries hitting this cluster?" Slow log shows commands exceeding the slowlog-log-slower-than threshold. | P1 |
| Subnets (subnet) | Via subnet group: Redis response has `CacheSubnetGroupName` → `elasticache:DescribeCacheSubnetGroups` → `Subnets[].SubnetIdentifier`. | "Which AZs is this cluster in?" For understanding failover behavior and network path. | P2 |

## CloudTrail Events (T key)

| Event Name | Why Engineers Search For It |
|-----------|---------------------------|
| DeleteReplicationGroup / DeleteCacheCluster | "Who deleted this Redis cluster?" Data loss if no snapshot was taken. |
| ModifyReplicationGroup / ModifyCacheCluster | "Who changed node type, number of replicas, or parameters?" Performance changes after modification. |
| CreateSnapshot | "Who took a manual snapshot?" For pre-migration or pre-maintenance safety verification. |
