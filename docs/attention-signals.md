# Attention Signals — Golden Contract

Per registered AWS resource type, this doc defines the read-only signals
that a production SRE cares about: degraded, failing, stopped, stale,
misconfigured-dangerously, cost-bleeding.

Signals are organized by the cost of observing them:

- **Wave 1** — available from the *list* API response (or pure
  computation over it, or cross-reference against already-loaded
  sibling-type lists). Zero extra API calls. **Per-resource `Describe*`
  is Wave 2, not Wave 1.**
- **Wave 2** — requires an extra read-only API call, bounded: one
  account-wide call + a small constant number of calls per resource.
- **Wave 3** — exceeds Wave 2's budget: CloudWatch metrics per resource,
  many calls per resource, async polling, external systems.

State-value buckets: **Healthy**, **Warning**, **Broken**, **Dim**
(terminal/admin-off/informational).

## Visualization Surfaces

Every signal in this contract must land on one or more of these five
existing surfaces. No other UI is allowed. This table is the master
definition; `docs/resources/<shortName>.md` §4 transcribes it verbatim.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count + list frame title `!N` suffix | Aggregated count of `!`-severity findings. `~` findings do not bump. The list frame title appends a space-separated `!N` after the count parentheses when the current list has N > 0 issues (`s3(50+) !5`, `ec2(17) !1`), or `!N+` when N is a truncated lower bound; N uses the same aggregation as the menu badge (Wave 1 issue-colored rows + Wave 2 `!`-severity findings). No suffix when N = 0, and omitted in attention-only mode (`ctrl+z`) — the filtered count already is the issue count, so `name(5 of 50+) [!]` stays as-is. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `stopping: Server.SpotInstanceShutdown`, `expires in 7d`). **Healthy rows render blank** — no `OK` / `available` / `ACTIVE` / `running`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

### S1 — list frame title issue count

S1 has two surfaces: the main-menu `issues:N` badge and the
resource-list frame title. The frame-title rules:

- The title appends a space-separated `!N` after the count parentheses
  when the current list has N > 0 issues: `s3(50+) !5`, `ec2(17) !1`.
- When N is a truncated lower bound, the suffix renders `!N+` —
  mirroring the menu badge's truncated `N+` form.
- N uses the same aggregation as the menu badge: Wave 1 issue-colored
  rows plus Wave 2 `!`-severity findings for the resources in the list.
  `~` findings do not bump.
- Attention-only mode (`ctrl+z`): the existing `[!]` title suffix
  stays and `!N` is omitted — the filtered count already is the issue
  count; `name(5 of 50+) [!]` renders exactly as today.
- Healthy list (N = 0): no suffix — the title is unchanged.

## Signals

### Compute

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `ec2` | EC2 Instances | `State.Name`: `running`→Healthy; `pending`/`shutting-down`/`stopping`→Warning; `stopped`→Warning; `terminated`→Dim. `StateReason.Code` begins `Server.*` on a `stopped` instance → Broken. `StateTransitionReason` carrying a user-initiated date >30d ago → Warning (long-stopped) | `DescribeInstanceStatus(IncludeAllInstances=true)`: `SystemStatus.Status`/`InstanceStatus.Status` == `impaired` → Broken; == `initializing` → Warning (checks have not yet passed since start); == `insufficient-data` → Warning (AWS cannot determine); == `not-applicable` → Healthy (informational only, not surfaced); `Events[]` with scheduled retirement/reboot in ≤7d → Warning | CloudWatch `StatusCheckFailed`; IMDSv1 detection | [DescribeInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html) |
| `ecs-svc` | ECS Services | `status`: `ACTIVE`→Healthy; `DRAINING`→Dim; `INACTIVE`→Broken. `runningCount < desiredCount` → Warning | `DescribeServices`: `deployments[].rolloutState==FAILED` → Broken; `runningCount < desiredCount` AND no active `IN_PROGRESS` deployment → Broken; `events[]` containing `unable to place` / `ELB health checks failed` in last 10m → Broken; deployment circuit-breaker triggered → Broken | CloudWatch `CPUUtilization`/`MemoryUtilization` p99 per service | [DescribeServices](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeServices.html) |
| `ecs` | ECS Clusters | `status`: `ACTIVE`→Healthy; `PROVISIONING`/`DEPROVISIONING`→Warning; `FAILED`/`INACTIVE`→Broken | `DescribeClusters(include=STATISTICS)`: `pendingTasksCount>0` sustained → Warning; `runningTasksCount==0 && registeredContainerInstancesCount>0` → Warning | CloudWatch `CPUReservation`/`MemoryReservation`; `DescribeContainerInstances` per cluster for agent-disconnect | [DescribeClusters](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeClusters.html) |
| `ecs-task` | ECS Tasks | `lastStatus`: `RUNNING`→Healthy; transitional states → Warning; `STOPPED` with `StopCode != UserInitiated`→Broken. `healthStatus==UNHEALTHY`→Broken | `DescribeTasks`: `StopCode` in `TaskFailedToStart`/`EssentialContainerExited` → Broken; container `exitCode` non-zero with `essential=true` → Broken | Cross-cluster outlier detection | [DescribeTasks](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeTasks.html) |
| `lambda` | Lambda Functions | `State`: `Active`→Healthy; `Pending`→Warning; `Inactive`→Dim; `Failed`→Broken. `LastUpdateStatus==Failed`→Broken. `Runtime` in [deprecated-runtimes list](https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtimes.html) → Broken. `DeadLetterConfig==nil` → Warning | None | CloudWatch `Errors/Invocations` ratio; `Throttles`; `Duration` p99 vs `Timeout`; `GetFunctionConcurrency` per function | [GetFunctionConfiguration](https://docs.aws.amazon.com/lambda/latest/api/API_GetFunctionConfiguration.html) |
| `asg` | Auto Scaling Groups | `Status==""`→Healthy; `Delete in progress`→Warning. `Instances[].HealthStatus==Unhealthy` → Warning. InService count < `MinSize` → Broken. `SuspendedProcesses` containing `Launch`/`Terminate`/`HealthCheck` → Warning | `DescribeScalingActivities(MaxRecords=1)`: latest `StatusCode==Failed` → Broken (launch-failure loop) | CloudWatch `GroupDesiredCapacity` vs `GroupInServiceInstances` delta sustained | [DescribeAutoScalingGroups](https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_DescribeAutoScalingGroups.html) |
| `eb` | Elastic Beanstalk | `Health`: `Green`→Healthy; `Yellow`/`Grey`→Warning; `Red`→Broken. `Status==Terminated`→Dim | `DescribeEnvironmentHealth`: `Causes[]` non-empty → Warning detail | `DescribeConfigurationSettings` platform-EOL check; `DescribeEvents` severity filter | [DescribeEnvironments](https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_DescribeEnvironments.html) |
| `ebs` | EBS Volumes | `State`: `in-use`→Healthy; `creating`/`deleting`→Warning; `error`→Broken. `available` with `CreateTime` >7d → Warning (orphan). `Encrypted==false` → Warning (unencrypted at rest) | `DescribeVolumeStatus`: `VolumeStatus.Status==impaired` → Broken; `VolumeStatus.Status==warning` → Warning; `Events[]` non-empty → Warning | CloudWatch `VolumeQueueLength`; `BurstBalance` on gp2 | [DescribeVolumeStatus](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVolumeStatus.html) |
| `ebs-snap` | EBS Snapshots | `State`: `completed`→Healthy; `pending`→Warning; `error`→Broken; `recoverable`/`recovering`→Broken. Age >365d with automated description → Warning (cost). `Encrypted==false` → Warning (unencrypted at rest). Cross-ref `ebs` — source volume deleted → Warning (orphan) | None | `DescribeSnapshotAttribute(createVolumePermission)` per snapshot (public-snapshot detection) | [DescribeSnapshots](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSnapshots.html) |
| `ami` | AMIs | `State`: `available`→Healthy; `pending`/`transient`→Warning; `failed`/`error`/`invalid`→Broken; `deregistered`/`disabled`→Dim. `DeprecationTime < now()` → Warning. Cross-ref `ebs-snap` (owner-scoped only — skip public/marketplace AMIs) — backing snapshot missing → Warning — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | None | `DescribeImageAttribute(launchPermission)` per AMI | [DescribeImages](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeImages.html) |

### Containers

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `eks` | EKS Clusters | None — `ListClusters` returns cluster name strings only | `DescribeCluster` per cluster (N+1): `status`: `ACTIVE`→Healthy; `CREATING`/`UPDATING`/`DELETING`/`PENDING`→Warning; `FAILED`→Broken. `health.issues[]` non-empty → Broken | EKS version EOL calendar vs cluster `version`; addon health | [DescribeCluster](https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeCluster.html) |
| `ng` | EKS Node Groups | None — `ListNodegroups` returns node-group name strings only | `DescribeNodegroup` per node group (N+1): `status`: `ACTIVE`→Healthy; `CREATING`/`UPDATING`/`DELETING`→Warning; `CREATE_FAILED`/`DELETE_FAILED`/`DEGRADED`→Broken. `health.issues[]` codes (`InsufficientFreeAddresses`, `Ec2LaunchTemplateVersionMismatch`, `AutoScalingGroupInvalidConfiguration`, `AccessDenied`, …) → Broken | AMI release drift; `ListUpdates` per node group | [DescribeNodegroup](https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeNodegroup.html) |

### Networking

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `elb` | Load Balancers | ELBv2 only: `State.Code`: `active`→Healthy; `provisioning`/`active_impaired`→Warning; `failed`→Broken. Surface `State.Reason` as Broken detail. Classic (ELBv1) has no `State` field | None (target health lives on `tg`) | CloudWatch `HTTPCode_ELB_5XX_Count`; `DescribeLoadBalancerAttributes` per LB (deletion-protection, access-logs) | [DescribeLoadBalancers](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeLoadBalancers.html) |
| `tg` | Target Groups | `LoadBalancerArns==[]` → Warning (orphan) | `DescribeTargetHealth` per TG: any target `State==unhealthy` → Warning; all unhealthy → Broken | CloudWatch `UnHealthyHostCount` / `HealthyHostCount` ratios | [DescribeTargetHealth](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeTargetHealth.html) |
| `vpc` | VPCs | `State`: `available`→Healthy; `pending`→Warning. Cross-ref `subnet` — no subnets → Warning (empty VPC) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | `DescribeFlowLogs` (filter `ResourceType=VPC` client-side): no flow logs for VPC → Warning (no traffic audit trail) | `DescribeVpcAttribute(EnableDnsSupport)` per VPC | [DescribeVpcs](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVpcs.html) |
| `subnet` | Subnets | `State`: `available`→Healthy; `pending`→Warning; `unavailable`/`failed`→Broken; `failed-insufficient-capacity`→Broken (AZ out of capacity — ENI provisioning here will fail). `AvailableIpAddressCount / CIDR size < 0.1` → Warning, `< 0.02` → Broken — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06). Cross-ref `rtb` — `MapPublicIpOnLaunch=true` without `0.0.0.0/0 → IGW` in associated RTB (including the VPC main RTB when subnet has no explicit association) → Warning (misconfigured public subnet) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | None | None | [DescribeSubnets](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSubnets.html) |
| `rtb` | Route Tables | `Routes[].State==blackhole` → Broken (dead target — gateway/ENI gone). No `Associations[]` AND not VPC main → Warning (orphan) | None | None | [DescribeRouteTables](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeRouteTables.html) |
| `nat` | NAT Gateways | `State`: `available`→Healthy; `pending`/`deleting`→Warning; `failed`→Broken. `FailureCode` non-empty → Broken detail; surface `FailureMessage` | None | CloudWatch `ErrorPortAllocation`, `PacketsDropCount`, `BytesOutToDestination==0` cost-waste | [DescribeNatGateways](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeNatGateways.html) |
| `igw` | Internet Gateways | `Attachments[].State`: `attached`→Healthy; `attaching`/`detaching`→Warning; `detached` → Warning (orphan) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06); `len(Attachments)==0` → Warning (orphan). Cross-ref `rtb` — IGW attached to VPC with no `0.0.0.0/0 → igw` route → Warning (unused) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | None | None | [DescribeInternetGateways](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInternetGateways.html) |
| `eip` | Elastic IPs | `AssociationId` absent AND `InstanceId` absent AND `NetworkInterfaceId` absent → Warning (unattached, billed hourly). Cross-ref `ec2` — attached to instance with `State.Name==stopped` → Warning (zombie billing) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | None | `DescribeAddressesAttribute` per EIP (reverse-DNS) | [DescribeAddresses](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAddresses.html) |
| `vpce` | VPC Endpoints | `State`: `Available`→Healthy; `PendingAcceptance`/`Pending`/`Deleting`→Warning; `Failed`/`Rejected`/`Expired`→Broken; `Partial`→Broken (Interface endpoint with some AZ ENIs failed to provision); `Deleted`→Dim. `LastError` non-empty → Broken detail. Interface endpoint `NetworkInterfaceIds==[]` → Broken. Gateway endpoint `RouteTableIds==[]` → Warning (orphan) | None | Endpoint policy analysis | [DescribeVpcEndpoints](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVpcEndpoints.html) |
| `tgw` | Transit Gateways | `State`: `available`→Healthy; `pending`/`modifying`/`deleting`→Warning; `deleted`→Dim. (Real attachment-level failures surface via Wave 2.) | `DescribeTransitGatewayAttachments`: any attachment `State` in `failed`/`failing`/`rejected`/`rejecting` → Broken; `pendingAcceptance` >24h → Warning | CloudWatch `PacketDropCountBlackhole`/`PacketDropCountNoRoute` | [DescribeTransitGatewayAttachments](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeTransitGatewayAttachments.html) |
| `eni` | Network Interfaces | `Status`: `in-use`/`associated`→Healthy; `attaching`/`detaching`→Warning; `available` → Warning (orphan). Requester-managed with `Description` referencing deleted service → Warning (zombie) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | None | None | [DescribeNetworkInterfaces](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeNetworkInterfaces.html) |
| `sg` | Security Groups | `IpPermissions[]` with `0.0.0.0/0` on ports (22, 23, 21, 3389, 1433, 3306, 5432, 6379, 27017, 11211, 9200) → Broken (exposed admin/db). Cross-ref `eni` — SG not referenced by any ENI → Warning (orphan) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | None | SG-referencing-deleted-SG detection | [DescribeSecurityGroups](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSecurityGroups.html) |

### Databases & Storage

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `dbi` | DB Instances | `DBInstanceStatus`: `available`→Healthy; transitional → Warning; `failed`/`storage-full`/`incompatible-*`/`restore-error`/`inaccessible-encryption-credentials`→Broken. `BackupRetentionPeriod==0` → Warning. `PubliclyAccessible==true` → Warning (reachable from the internet). `StorageEncrypted==false` → Warning (unencrypted at rest). `DeletionProtection==false` → Warning | `DescribePendingMaintenanceActions` (one account-wide call): action with `ForcedApplyDate` or `AutoAppliedAfterDate` in past → Warning | CloudWatch `FreeStorageSpace`, `CPUUtilization`, `ReplicaLag`, `DatabaseConnections` | [DescribeDBInstances](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBInstances.html) |
<!-- amended by a9s-resource-spec during dbc gen: service URL realigned to DocumentDB to match related-resources.md; name disambiguated from RDS Aurora -->
| `dbc` | DocumentDB Clusters | `Status`: `available`→Healthy; transitional → Warning; `failed`/`inaccessible-encryption-credentials`/`incompatible-parameters`→Broken. No `DBClusterMembers[]` entry with `IsClusterWriter==true` → Broken. `DeletionProtection==false` → Warning. `StorageEncrypted==false` → Warning. `BackupRetentionPeriod==0` → Warning | Shared with `dbi`: `DescribePendingMaintenanceActions` — an overdue action emits the `maintenance overdue` finding at Broken severity (unlike `dbi`, which emits Warning) | CloudWatch `DBInstanceReplicaLag`, `DatabaseConnections` | [DescribeDBClusters](https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeDBClusters.html) |
| `redis` | ElastiCache Redis | (Replication-group-scoped; values are `ReplicationGroup.Status`.) `Status`: `available`→Healthy; `creating`/`modifying`/`deleting`/`snapshotting`→Warning; `create-failed`→Broken. `AutomaticFailover` != `enabled` on multi-AZ → Warning | None | CloudWatch `DatabaseMemoryUsagePercentage`, `Evictions`, `ReplicationLag`, `EngineCPUUtilization` | [DescribeReplicationGroups](https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_DescribeReplicationGroups.html) |
| `ddb` | DynamoDB Tables | None — `ListTables` returns table names only | `DescribeTable` per table (N+1): `TableStatus`: `ACTIVE`→Healthy; `CREATING`/`UPDATING`/`DELETING`/`ARCHIVING`→Warning; `INACCESSIBLE_ENCRYPTION_CREDENTIALS`/`ARCHIVED`→Broken. Plus `DescribeContinuousBackups` per table: PITR disabled → Warning | CloudWatch `ReadThrottleEvents`+`WriteThrottleEvents`, `SystemErrors` | [DescribeTable](https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_DescribeTable.html) |
| `opensearch` | OpenSearch Domains | None — `ListDomainNames` returns domain names only | `DescribeDomains` (bounded fan-out): `Deleted==true`→Dim; `Processing==true` or `UpgradeProcessing==true`→Warning; `DomainProcessingStatus==Isolated`→Broken. `ServiceSoftwareOptions.UpdateAvailable==true` with `AutomatedUpdateDate` in past → Warning. `EncryptionAtRestOptions.Enabled==false` → Warning | Cluster-health (Red/Yellow/Green) is CloudWatch-only: `AWS/ES` namespace, `ClusterStatus.red`/`yellow`. Also `FreeStorageSpace`, `JVMMemoryPressure` | [DescribeDomains](https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DescribeDomains.html) |
| `redshift` | Redshift Clusters | `ClusterStatus`: `available`→Healthy; `creating`/`modifying`/`resizing`/`rebooting`/`renaming`/`deleting`→Warning; `incompatible-hsm`/`incompatible-network`/`incompatible-parameters`/`incompatible-restore`/`hardware-failure`/`storage-full`→Broken. `ClusterAvailabilityStatus`: `Unavailable`/`Failed`→Broken, `Maintenance`/`Modifying`→Warning. `PendingModifiedValues` non-empty → Warning. `DeferredMaintenanceWindows[]` active → Warning. `PubliclyAccessible==true` → Warning. `Encrypted==false` → Warning | None | CloudWatch `PercentageDiskSpaceUsed`, `HealthStatus` | [DescribeClusters](https://docs.aws.amazon.com/redshift/latest/APIReference/API_DescribeClusters.html) |
| `efs` | EFS File Systems | `LifeCycleState`: `available`→Healthy; `creating`/`updating`/`deleting`→Warning; `error`→Broken. `NumberOfMountTargets==0` → Broken (unreachable) | `DescribeMountTargets` per FS: any mount target `LifeCycleState` != `available` → Broken | CloudWatch `PercentIOLimit`, `BurstCreditBalance` | [DescribeMountTargets](https://docs.aws.amazon.com/efs/latest/ug/API_DescribeMountTargets.html) |
| `s3` | S3 Buckets | None — `ListBuckets` returns `Name`, `CreationDate`, `BucketRegion`, `BucketArn` only | `GetPublicAccessBlock` per bucket: `NoSuchPublicAccessBlockConfiguration` error or any flag false → Warning (public-exposure risk; account-level PAB may still override) | `GetBucketPolicyStatus`, `GetBucketEncryption`, `GetBucketVersioning`, `GetBucketLogging`, `GetBucketLifecycleConfiguration` (each is per-bucket) | [GetPublicAccessBlock](https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetPublicAccessBlock.html) |
| `dbi-snap` | DB Instance Snapshots | `Status`: `available`→Healthy; `creating`→Warning; `failed`/`incompatible-*`→Broken. `Encrypted==false` → Warning (unencrypted at rest). Cross-ref `dbi` — source DB deleted → Warning (orphan). When parent DB in the loaded list: `SnapshotCreateTime` > parent `BackupRetentionPeriod` AND `SnapshotType==automated` → Warning | None | `DescribeDBSnapshotAttributes` per snapshot (public-snapshot) | [DescribeDBSnapshots](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBSnapshots.html) |
| `dbc-snap` | DB Cluster Snapshots | `Status`: `available`→Healthy; `creating`→Warning; `failed`→Broken. Manual snapshot age >365d → Warning (cost). Cross-ref `dbc`: when parent cluster in loaded list, age > cluster `BackupRetentionPeriod` × 1.5 AND automated → Warning | None | `DescribeDBClusterSnapshotAttributes` per snapshot | [DescribeDBClusterSnapshots](https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeDBClusterSnapshots.html) |

### Messaging

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `sqs` | SQS Queues | None — `ListQueues` returns URLs only | `GetQueueAttributes(All)` per queue: `ApproximateNumberOfMessages` > threshold → Warning — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06), rising unbounded → Broken — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06); `ApproximateAgeOfOldestMessage` > `VisibilityTimeout`×5 → Warning (consumer lag) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06); is-DLQ with messages → Warning — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06); `RedrivePolicy` unset on main queue → Warning | CloudWatch `NumberOfMessagesSent/Received` trend | [GetQueueAttributes](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html) |
| `sns` | SNS Topics | None — `ListTopics` returns ARN only | `GetTopicAttributes` per topic: `SubscriptionsConfirmed==0` AND `SubscriptionsPending==0` → Warning (orphan topic); `KmsMasterKeyId` absent on sensitive topic → Warning — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | CloudWatch `NumberOfNotificationsFailed` | [GetTopicAttributes](https://docs.aws.amazon.com/sns/latest/api/API_GetTopicAttributes.html) |
| `sns-sub` | SNS Subscriptions | `SubscriptionArn == "PendingConfirmation"` → Warning (never confirmed) | None | `GetSubscriptionAttributes` per subscription (DLQ); CloudWatch `NumberOfNotificationsFailed` per endpoint | [ListSubscriptions](https://docs.aws.amazon.com/sns/latest/api/API_ListSubscriptions.html) |
| `eb-rule` | EventBridge Rules | `State`: `ENABLED`→Healthy; `ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS`→Healthy; `DISABLED`→Dim (admin-off) | `ListTargetsByRule` per rule: rule `State==ENABLED` AND `len(Targets)==0` → Broken (rule matches but goes nowhere); rule `State==DISABLED` AND `len(Targets)>0` → Warning (disabled rule with targets — probable oversight); any target without `DeadLetterConfig` → Warning | CloudWatch `FailedInvocations`/`ThrottledRules` per rule | [ListTargetsByRule](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_ListTargetsByRule.html) |
| `kinesis` | Kinesis Streams | `StreamStatus`: `ACTIVE`→Healthy; `CREATING`/`UPDATING`/`DELETING`→Warning | None | CloudWatch `GetRecords.IteratorAgeMilliseconds` (consumer lag), `WriteProvisionedThroughputExceeded`, `ReadProvisionedThroughputExceeded` | [DescribeStreamSummary](https://docs.aws.amazon.com/kinesis/latest/APIReference/API_DescribeStreamSummary.html) |
<!-- amended by a9s-resource-spec during msk gen: SDK kafka/types.ClusterState enum also defines HEALING (auto-broker-replacement); bucketed Warning by operator semantics (capacity degraded during auto-heal). -->
| `msk` | MSK Clusters | `State`: `ACTIVE`→Healthy; `CREATING`/`UPDATING`/`MAINTENANCE`/`REBOOTING_BROKER`/`HEALING`→Warning; `DELETING`→Dim; `FAILED`→Broken | None (per-broker runtime state is not on any read-only AWS action; `ListNodes` returns node metadata but no `RUNNING` enum) | CloudWatch `ActiveControllerCount`, `OfflinePartitionsCount`, `UnderReplicatedPartitions`, `KafkaDataLogsDiskUsed` | [ListClustersV2](https://docs.aws.amazon.com/msk/1.0/apireference/v2-clusters.html) |
| `sfn` | Step Functions | None — `ListStateMachines` is config-only | `ListExecutions(statusFilter=FAILED, maxResults=1)` per state machine: any recent failure → Warning; consecutive failures → Broken | CloudWatch `ExecutionsFailed`/`ExecutionsTimedOut`/`ExecutionThrottled` trend | [ListExecutions](https://docs.aws.amazon.com/step-functions/latest/apireference/API_ListExecutions.html) |
<!-- amended by a9s-resource-spec during ses gen: AWS SDK Go v2 sesv2/types.VerificationStatus enum values are PENDING/SUCCESS/FAILED/TEMPORARY_FAILURE/NOT_STARTED (uppercase, underscored) — realigned from Title-case. -->
| `ses` | SES Identities | `VerificationStatus`: `SUCCESS`→Healthy; `PENDING`→Warning; `FAILED`/`TEMPORARY_FAILURE`/`NOT_STARTED`→Broken. `SendingEnabled==false` → Warning | `GetAccount` (SESv2, account-wide): `EnforcementStatus` in `PROBATION`/`SHUTDOWN` → Broken; `SendQuota.SentLast24Hours > 0.8 × Max24HourSend` → Warning | Per-identity DKIM drift; reputation dashboard (`BounceRate`/`ComplaintRate`) via CloudWatch | [GetAccount (SESv2)](https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_GetAccount.html) |

### Secrets & Config

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `secrets` | Secrets Manager | `RotationEnabled==true && now > NextRotationDate` → Warning (overdue). `RotationEnabled==true && (now - LastRotatedDate) > RotationRules.AutomaticallyAfterDays × 2` → Broken (rotation failing) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06). `LastAccessedDate` >180d → Warning (dormant; note: field is day-truncated and excludes access in the current call). `DeletedDate` set → Broken | None | `DescribeSecret` per secret for `VersionIdsToStages` stuck on `AWSPENDING` | [ListSecrets](https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_ListSecrets.html) |
<!-- amended by a9s-resource-spec during ssm gen: DescribeParameters / ParameterMetadata does not return access timestamps; "unused" cannot be computed at Wave 1. Rephrased to "LastModifiedDate >90d" (age proxy) so the signal is achievable from the list response. True access-age lives in GetParameterHistory and remains Wave 3. -->
| `ssm` | SSM Parameters | `Type==SecureString` AND `LastModifiedDate` >365d → Warning (stale). `Type==String` AND name suffix matches `-password`/`-secret`/`-token` or `/secret`/`/password`/`/token` → Warning (should be SecureString). `Tier==Advanced` AND `LastModifiedDate` >90d → Warning (cost — aged Advanced parameter) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | None | `GetParameterHistory` per parameter (true access-age) | [DescribeParameters](https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_DescribeParameters.html) |
| `kms` | KMS Keys | None — `ListKeys` returns `{KeyId, KeyArn}` only | `DescribeKey` per key: `KeyState`: `Enabled`→Healthy; `Creating`/`Updating`→Warning; `Disabled`→Warning; `PendingDeletion`/`PendingImport`/`PendingReplicaDeletion`→Broken; `Unavailable`→Broken. Plus `GetKeyRotationStatus` per key: `KeyRotationEnabled==false` on CMK → Warning | Key-policy analysis per key (`Principal:*` detection) | [DescribeKey](https://docs.aws.amazon.com/kms/latest/APIReference/API_DescribeKey.html) |

### Security & IAM

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `role` | IAM Roles | `AssumeRolePolicyDocument` (URL-encoded JSON on `ListRoles`) contains `Principal:{"AWS":"*"}` without an external-id condition → Broken | `GetRole` per role: `RoleLastUsed.LastUsedDate` missing or >90d → Warning (dormant; field is region-scoped — may false-warn in multi-region accounts) | `ListAttachedRolePolicies` per role (admin-access detection); `GenerateServiceLastAccessedDetails` async permission-usage audit | [GetRole](https://docs.aws.amazon.com/IAM/latest/APIReference/API_GetRole.html) |
| `policy` | IAM Policies | `AttachmentCount==0` AND not AWS-managed → Warning (orphan) | `GetPolicyVersion` per policy: document contains `"Effect":"Allow","Action":"*","Resource":"*"` → Broken (wildcard admin) | IAM Access Advisor unused-permission analysis | [GetPolicyVersion](https://docs.aws.amazon.com/IAM/latest/APIReference/API_GetPolicyVersion.html) |
| `iam-user` | IAM Users | `PasswordLastUsed` absent AND `CreateDate` >90d → Warning (dormant console user) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | Per user: `ListAccessKeys`, `GetAccessKeyLastUsed` per key, `ListMFADevices`. Key `Status==Active` with `LastUsedDate` >90d → Warning; key Active with `LastUsedDate==N/A` AND `CreateDate` >90d → Warning (never used); console login enabled AND no MFA device → Broken | Credential report (`GenerateCredentialReport` + `GetCredentialReport`) — async/polling, provides all of the above in one account-wide pull when cached | [ListUsers](https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListUsers.html) |
| `iam-group` | IAM Groups | None — `ListGroups` is config-only | `GetGroup` per group: `Users==[]` AND group age >30d → Warning (orphan) | `ListAttachedGroupPolicies` per group (admin-access + blast radius) | [GetGroup](https://docs.aws.amazon.com/IAM/latest/APIReference/API_GetGroup.html) |
| `waf` | WAF Web ACLs | None — `ListWebACLs` is config-only | `GetWebACL` per ACL: `Rules==[]` → Warning (no-op ACL); `DefaultAction==Allow` with zero rules → Broken | `ListResourcesForWebACL` per ACL; CloudWatch `BlockedRequests` spike; managed-rule-group version drift | [GetWebACL](https://docs.aws.amazon.com/waf/latest/APIReference/API_GetWebACL.html) |

### DNS, CDN, Certs

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `r53` | Route 53 Hosted Zones | `ResourceRecordSetCount<=2` on non-new zone → Warning (only SOA+NS, likely unused) | `GetHostedZone` per zone: `Config.PrivateZone==true` with empty `VPCs[]` → Warning (orphan private zone, informational `~` finding). `GetDNSSEC` per zone with `Config.PrivateZone==false` (private zones not supported): status `SIGNING` with KSK inactive → Broken — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | Registrar nameserver lookup (external); health-check status aggregation per zone | [ListHostedZones](https://docs.aws.amazon.com/Route53/latest/APIReference/API_ListHostedZones.html) |
| `cf` | CloudFront Distributions | `Status`: `Deployed`→Healthy; `InProgress`→Warning. `Enabled==false`→Dim. When `ViewerCertificate.CloudFrontDefaultCertificate==false`: `MinimumProtocolVersion` in `SSLv3`/`TLSv1`/`TLSv1_2016`/`TLSv1.1_2016` → Warning — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06). `WebACLId==""` → Warning (no WAF) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | `GetDistributionConfig` per distribution: `DefaultCacheBehavior.ViewerProtocolPolicy==allow-all` or any origin `CustomOriginConfig.OriginProtocolPolicy==http-only` → Warning (insecure protocol, informational `~` finding — distinct from the Wave 1 `MinimumProtocolVersion` weak-TLS signal). `Logging.Enabled==false` → Warning — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | CloudWatch `5xxErrorRate`/`TotalErrorRate`; origin-deleted cross-check | [ListDistributions](https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_ListDistributions.html) |
| `acm` | ACM Certificates | `Status`: `ISSUED`→Healthy; `PENDING_VALIDATION`→Warning; `EXPIRED`/`REVOKED`/`FAILED`/`VALIDATION_TIMED_OUT`→Broken; `INACTIVE`→Dim. `NotAfter - now() < 30d` → Warning, `< 7d` → Broken. `InUse==false` on non-expired cert → Warning (orphan) | `DescribeCertificate` per cert: `RenewalSummary.RenewalStatus==FAILED` → Broken — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06); `DomainValidationOptions[].ValidationStatus==FAILED` → Broken — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | None | [ListCertificates](https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html) |
| `apigw` | API Gateways | None — config-only list | `apigatewayv2:GetStages` per HTTP/WebSocket API: no deployed stage → Warning (orphan API). REST v1 APIs are listed but not currently stage-enriched (v1 `GetStages` path not wired). | CloudWatch `5XXError`/`4XXError`; `GetUsagePlans` quota-breach detection | [GetStages](https://docs.aws.amazon.com/apigateway/latest/api/API_GetStages.html) |

### Monitoring

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `alarm` | CloudWatch Alarms | `StateValue`: `OK`→Healthy; `INSUFFICIENT_DATA`→Warning; `ALARM`→Broken. `ActionsEnabled==false` (muted) and `AlarmActions==[]` (alert-to-nowhere) → Warning — both emitted as the single `alarm.no_actions` finding (phrase `no actions`, keyed on empty `AlarmActions`). `StateValue==INSUFFICIENT_DATA` AND `StateUpdatedTimestamp` older than 2×`Period` → Broken (metric pipeline dead) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06). Cross-ref siblings — `Dimensions` reference a resource ID absent from the already-loaded sibling-type list (skip rule when sibling list wasn't loaded in this sweep) → Warning (zombie alarm) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | None | None | [DescribeAlarms](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_DescribeAlarms.html) |
| `logs` | CloudWatch Log Groups | `retentionInDays` is nil → Warning (Never Expire = cost drift). `storedBytes==0 && creationTime<now()-90d` → Warning (orphan). Cross-ref `kms` — referenced `kmsKeyId` is in `PendingDeletion` → Broken | `DescribeLogStreams(orderBy=LastEventTime, descending=true, limit=1)` per log group: `lastEventTimestamp` stale beyond expected write cadence → Warning (silent service) | Metric-filter-count check | [DescribeLogStreams](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_DescribeLogStreams.html) |
| `trail` | CloudTrail Trails | `LogFileValidationEnabled==false` → Warning (field is on `DescribeTrails` list response) | `GetTrailStatus` per trail: `IsLogging==false` → Broken (trail stopped capturing); `LatestDeliveryError` non-empty → Broken (S3 delivery failing); `LatestDeliveryTime` >1h ago on `IsLogging==true` trail → Broken | `LookupEvents` absence detection | [GetTrailStatus](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_GetTrailStatus.html) |
| `ct-events` | CloudTrail Events | `Event.ReadOnly=="false"` (field is a string) isolates write-attempts. `errorCode` and detailed event info are only inside `Event.CloudTrailEvent` (raw JSON string) — must parse JSON to read. Parsed `errorCode` present → Warning; count `errorCode==AccessDenied` AND `ReadOnly=="false"` in last hour by principal > N → Broken | None | Absence-of-expected-events alerting | [LookupEvents](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html) |

### CI/CD

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `cfn` | CloudFormation Stacks | `StackStatus`: `CREATE_COMPLETE`/`UPDATE_COMPLETE`/`IMPORT_COMPLETE`/`UPDATE_COMPLETE_CLEANUP_IN_PROGRESS`→Healthy; `*_IN_PROGRESS`/`REVIEW_IN_PROGRESS`→Warning; `ROLLBACK_COMPLETE`→Warning (failed-create tombstone — stack operationally dead, delete-and-recreate required, but not actively failing); `UPDATE_ROLLBACK_COMPLETE`/`IMPORT_ROLLBACK_COMPLETE`→Warning (update failed, stack reverted to prior state); `*_FAILED`→Broken. `IN_PROGRESS` >1h → Broken (stuck). `DriftInformation.StackDriftStatus==DRIFTED` → Warning (signal is low-coverage until `DetectStackDrift` has been run) | `DescribeStackEvents` per stack — take the first response page and scan the most recent events client-side: recent event `ResourceStatus==*_FAILED` → Broken | `DetectStackDrift` + `DescribeStackDriftDetectionStatus` (async polling) for fresh drift detection | [DescribeStacks](https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_DescribeStacks.html) |
| `pipeline` | CodePipelines | None — `ListPipelines` is config-only | `GetPipelineState` per pipeline: any `stageStates[].latestExecution.status` in `Failed`/`Stopped`/`Cancelled` → Broken; stage `InProgress` >2h → Warning | `ListPipelineExecutions` trend; dormant-pipeline detection | [GetPipelineState](https://docs.aws.amazon.com/codepipeline/latest/APIReference/API_GetPipelineState.html) |
| `cb` | CodeBuild Projects | None — `ListProjects` is config-only | Latest build status per project (via `ListBuildsForProject(maxResults=1)` + batched `BatchGetBuilds`): latest `buildStatus` in `FAILED`/`FAULT`/`TIMED_OUT` → Broken (excluding user-initiated `STOPPED`) | Stale-project (>90d); cache-config + perf signals | [BatchGetBuilds](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_BatchGetBuilds.html) |
| `ecr` | ECR Repositories | `imageScanningConfiguration.scanOnPush==false` → Warning (no vulnerability scanning) — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | `DescribeImages` per repository, latest image by `imagePushedAt` (client-side sort — the API does not order by time): `imageScanFindingsSummary.findingSeverityCounts.CRITICAL>0` → Broken; `HIGH>0` → Warning (summary present only when scan has run) | `DescribeImageScanFindings` per image; `GetLifecyclePolicy` per repo | [DescribeImages](https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_DescribeImages.html) |
| `codeartifact` | CodeArtifact Repos | None — `ListRepositories` is config-only | `ListPackages(maxResults=1)` per repo: empty repo with age >30d → Warning (unused) | `DescribeRepository` encryption check; `GetRepositoryPermissionsPolicy` analysis | [ListPackages](https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_ListPackages.html) |

### Data & Analytics

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `glue` | Glue Jobs | None — `GetJobs` returns definitions only | `GetJobRuns(maxResults=1)` per job (API returns runs in descending-by-start-time order): latest `JobRunState` in `FAILED`/`TIMEOUT`/`ERROR`/`EXPIRED` → Broken (excluding user-initiated `STOPPED`) | DPU-hours trend; bookmark-stuck detection | [GetJobRuns](https://docs.aws.amazon.com/glue/latest/webapi/API_GetJobRuns.html) |
| `athena` | Athena Workgroups | `State`: `ENABLED`→Healthy; `DISABLED`→Warning (admin off) | `GetWorkGroup` per workgroup: `EnforceWorkGroupConfiguration==false` AND `ResultConfiguration.EncryptionConfiguration==nil` → Warning (governance); `BytesScannedCutoffPerQuery` unset → Warning (cost) | `ListQueryExecutions`+`BatchGetQueryExecution` failure-rate per workgroup | [GetWorkGroup](https://docs.aws.amazon.com/athena/latest/APIReference/API_GetWorkGroup.html) |
| `mwaa` | Managed Airflow | None — `ListEnvironments` returns environment name strings only | `GetEnvironment` per environment (N+1; accounts run 1–5 envs): `Status`: `AVAILABLE`→Healthy; `CREATING`/`CREATING_SNAPSHOT`/`PENDING`/`UPDATING`/`ROLLING_BACK`/`MAINTENANCE`→Warning; `CREATE_FAILED`/`UPDATE_FAILED`/`UNAVAILABLE`→Broken; `DELETING`/`DELETED`→Dim. `LastUpdate.Status==FAILED` on an `AVAILABLE` environment → Warning (last update failed; surface `LastUpdate.Error.ErrorMessage`). `WebserverAccessMode` in `PUBLIC_ONLY`/`PUBLIC_AND_PRIVATE` → Warning (webserver reachable from the internet). AccessDenied on `mwaa:ListEnvironments` → menu row shows the error, never `0` | CloudWatch `AWS/MWAA` metrics: `SchedulerHeartbeat`, DAG-processing `ImportErrors`/`TotalParseTime`, `QueuedTasks`/`RunningTasks`, worker/scheduler CPU+memory. AirflowVersion-EOL check (no stable API source) | [GetEnvironment](https://docs.aws.amazon.com/mwaa/latest/API/API_GetEnvironment.html) |

### Backup

| shortName | Name | Wave 1 | Wave 2 | Wave 3 | Source |
|---|---|---|---|---|---|
| `backup` | Backup Plans | None — `ListBackupPlans` is config-only | `ListBackupJobs(ByCreatedAfter=now-24h)` (account-wide, bucketed by `BackupPlanId`): any job `State` in `FAILED`/`EXPIRED`/`ABORTED` → Broken; `PARTIAL` → Warning (some resources backed up, others failed) | "Newest completed older than rule cadence × 2" (requires `GetBackupPlan` per plan for rule cadence) | [ListBackupJobs](https://docs.aws.amazon.com/aws-backup/latest/devguide/API_ListBackupJobs.html) |

<!-- BEGIN GENERATED: findings-table -->
| Type | Code | Phrase | Severity | Source |
| --- | --- | --- | --- | --- |
| ec2 | ec2.state.pending | pending | warn | wave1 |
| ec2 | ec2.state.shutting-down | shutting down | warn | wave1 |
| ec2 | ec2.state.stopping | stopping | warn | wave1 |
| ec2 | ec2.state.stopped | stopped | warn | wave1 |
| ec2 | ec2.state.stopped.server | stopped | broken | wave1 |
| ec2 | ec2.state.terminated | terminated | dim | wave1 |
| ec2 | ec2.instance-status-impaired | impaired: system checks failing | broken | wave2 |
| ecs-svc | ecs-svc.state.inactive | inactive | broken | wave1 |
| ecs-svc | ecs-svc.state.draining | draining | warn | wave1 |
| ecs-svc | ecs-svc.deployment-failed | deployment failed | broken | wave2 |
| ecs | ecs.state.provisioning | provisioning | warn | wave1 |
| ecs | ecs.state.deprovisioning | deprovisioning | warn | wave1 |
| ecs | ecs.state.failed | failed | broken | wave1 |
| ecs | ecs.state.inactive | inactive | broken | wave1 |
| ecs | ecs.cluster-issue | <N> pending tasks | warn | wave2 |
| ecs-task | ecs-task.state.provisioning | provisioning | warn | wave1 |
| ecs-task | ecs-task.state.pending | pending | warn | wave1 |
| ecs-task | ecs-task.state.activating | activating | warn | wave1 |
| ecs-task | ecs-task.state.deactivating | deactivating | warn | wave1 |
| ecs-task | ecs-task.state.stopping | stopping | warn | wave1 |
| ecs-task | ecs-task.state.deprovisioning | deprovisioning | warn | wave1 |
| ecs-task | ecs-task.state.stopped | stopped | dim | wave1 |
| ecs-task | ecs-task.stop-code.failed | stopped: <stop code> | broken | wave1 |
| ecs-task | ecs-task.health.unhealthy | unhealthy | broken | wave1 |
| ecs-task | ecs-task.task-failed | <stop code or container> failed | broken | wave2 |
| lambda | lambda.last-update.failed | last update failed to apply | broken | wave1 |
| lambda | lambda.runtime.deprecated | runtime is end-of-life | broken | wave1 |
| lambda | lambda.state.pending | pending | warn | wave1 |
| lambda | lambda.state.failed | failed | broken | wave1 |
| lambda | lambda.state.inactive | inactive, evicted after extended idle time | dim | wave1 |
| lambda | lambda.dlq.missing | no dead-letter queue configured | warn | wave1 |
| asg | asg.state.deleting | delete in progress | warn | wave1 |
| asg | asg.instances.underprovisioned | <N> of <M> instances in service | broken | wave1 |
| asg | asg.instances.unhealthy | <N> unhealthy instance(s) | warn | wave1 |
| asg | asg.scaling.suspended | scaling suspended | warn | wave1 |
| asg | asg.scaling-activity-failed | latest scaling activity failed | broken | wave2 |
| ebs | ebs.state.creating | creating | warn | wave1 |
| ebs | ebs.state.error | error | broken | wave1 |
| ebs | ebs.orphan-unattached | orphan: unattached Nd | warn | wave1 |
| ebs | ebs.encryption.disabled | unencrypted | warn | wave1 |
| ebs | ebs.volume-io-degraded | volume I/O degraded | broken | wave2 |
| ebs-snap | ebs-snap.state.pending | pending | warn | wave1 |
| ebs-snap | ebs-snap.state.error | error | broken | wave1 |
| ebs-snap | ebs-snap.encryption.disabled | unencrypted | warn | wave1 |
| ebs-snap | ebs-snap.aged-automated | automated, <N>d old | warn | wave1 |
| ebs-snap | ebs-snap.orphan | orphan: source volume deleted | warn | wave2 |
| ami | ami.state.pending | pending | warn | wave1 |
| ami | ami.state.failed | failed | broken | wave1 |
| ami | ami.state.dim | deregistered | dim | wave1 |
| ami | ami.deprecated | deprecated | warn | wave1 |
| eks | eks.state.creating | creating | warn | wave1 |
| eks | eks.state.updating | updating | warn | wave1 |
| eks | eks.state.failed | failed | broken | wave1 |
| eks | eks.health-issue | issue: <Issue.Code> | warn | wave1 |
| ng | ng.state.creating | creating | warn | wave1 |
| ng | ng.state.updating | updating | warn | wave1 |
| ng | ng.state.deleting | deleting | warn | wave1 |
| ng | ng.state.create-failed | create failed | broken | wave1 |
| ng | ng.state.delete-failed | delete failed | broken | wave1 |
| ng | ng.state.degraded | degraded | broken | wave1 |
| elb | elb.state.provisioning | provisioning | warn | wave1 |
| elb | elb.state.active\_impaired | active impaired | warn | wave1 |
| elb | elb.state.failed | failed | broken | wave1 |
| elb | elb.misconfigured | deletion protection disabled | warn | wave2 |
| tg | tg.unhealthy-targets | unhealthy targets: <N>/<M> | broken | wave2 |
| sg | sg.ingress.wide-open | all ports open to 0.0.0.0/0 | broken | wave1 |
| sg | sg.ingress.dangerous-ports | ports <list> open to 0.0.0.0/0 | broken | wave1 |
| vpc | vpc.state.pending | pending | warn | wave1 |
| vpc | vpc.no-flow-logs | no active VPC flow logs | warn | wave2 |
| subnet | subnet.state.pending | pending | warn | wave1 |
| subnet | subnet.state.unavailable | unavailable | broken | wave1 |
| subnet | subnet.state.failed | failed | broken | wave1 |
| subnet | subnet.state.failed-insufficient-capacity | failed-insufficient-capacity | broken | wave1 |
| rtb | rtb.route.blackhole | blackhole route (target deleted) | broken | wave1 |
| rtb | rtb.orphan-unassociated | no subnet associations | warn | wave1 |
| nat | nat.state.pending | pending | warn | wave1 |
| nat | nat.state.deleting | deleting | warn | wave1 |
| nat | nat.state.failed | failed | broken | wave1 |
| nat | nat.state.deleted | deleted | dim | wave1 |
| igw | igw.state.attaching | attaching | warn | wave1 |
| igw | igw.state.detaching | detaching | warn | wave1 |
| igw | igw.no-attachments | no VPC attachments | warn | wave1 |
| eip | eip.unassociated | unassociated | warn | wave1 |
| vpce | vpce.state.pending\_acceptance | pending acceptance | warn | wave1 |
| vpce | vpce.state.pending | pending | warn | wave1 |
| vpce | vpce.state.deleting | deleting | warn | wave1 |
| vpce | vpce.state.failed | failed | broken | wave1 |
| vpce | vpce.state.rejected | rejected | broken | wave1 |
| vpce | vpce.state.expired | expired | broken | wave1 |
| vpce | vpce.state.partial | partial | broken | wave1 |
| vpce | vpce.state.deleted | deleted | dim | wave1 |
| tgw | tgw.state.pending | pending | warn | wave1 |
| tgw | tgw.state.modifying | modifying | warn | wave1 |
| tgw | tgw.state.deleting | deleting | warn | wave1 |
| tgw | tgw.state.failed | failed | broken | wave1 |
| tgw | tgw.state.deleted | deleted | dim | wave1 |
| tgw | tgw.attachment-failed | attachment <id> failed | broken | wave2 |
| tgw | tgw.attachment-transitional | attachment <id> <state> | warn | wave2 |
| eni | eni.state.attaching | attaching | warn | wave1 |
| eni | eni.state.detaching | detaching | warn | wave1 |
| eni | eni.state.available | available | warn | wave1 |
| dbi | dbi.broken.failed | failed | broken | wave1 |
| dbi | dbi.broken.storage\_full | storage-full | broken | wave1 |
| dbi | dbi.broken.incompatible\_network | incompatible-network | broken | wave1 |
| dbi | dbi.broken.incompatible\_option\_group | incompatible-option-group | broken | wave1 |
| dbi | dbi.broken.incompatible\_parameters | incompatible-parameters | broken | wave1 |
| dbi | dbi.broken.incompatible\_restore | incompatible-restore | broken | wave1 |
| dbi | dbi.broken.restore\_error | restore-error | broken | wave1 |
| dbi | dbi.broken.encryption\_key\_unavailable | encryption key unavailable | broken | wave1 |
| dbi | dbi.broken.stopped | stopped | broken | wave1 |
| dbi | dbi.warn.transitional | <status>: <pending field> | warn | wave1 |
| dbi | dbi.warn.no\_automated\_backups | no automated backups | warn | wave1 |
| dbi | dbi.warn.publicly\_accessible | publicly accessible | warn | wave1 |
| dbi | dbi.warn.unencrypted\_storage | unencrypted storage | warn | wave1 |
| dbi | dbi.warn.deletion\_protection\_off | deletion protection off | warn | wave1 |
| dbi | dbi.pending-maintenance | maintenance scheduled | warn | wave2 |
| s3 | s3.public-access-block-incomplete | public access block incomplete | broken | wave2 |
| redis | redis.broken.create\_failed | create failed — see events | broken | wave1 |
| redis | redis.warn.creating | creating — new group | warn | wave1 |
| redis | redis.warn.deleting | deleting — teardown | warn | wave1 |
| redis | redis.warn.modifying | modifying — config change | warn | wave1 |
| redis | redis.warn.snapshotting | snapshotting — backup running | warn | wave1 |
| redis | redis.warn.shard\_issue | shard <NodeGroupId>: <status> | warn | wave1 |
| redis | redis.warn.multiaz\_without\_auto\_failover | multi-AZ without auto-failover | warn | wave1 |
| dbc | dbc.broken.failed | failed: cluster operation | broken | wave1 |
| dbc | dbc.broken.encryption\_key\_unreachable | encryption key unreachable | broken | wave1 |
| dbc | dbc.broken.incompatible\_parameters | parameter group incompatible | broken | wave1 |
| dbc | dbc.broken.no\_writer | no writer: reads only | broken | wave1 |
| dbc | dbc.warn.transitional | <status>: in progress | warn | wave1 |
| dbc | dbc.warn.deletion\_protection\_off | delete-protection off | warn | wave1 |
| dbc | dbc.warn.not\_encrypted\_at\_rest | not encrypted at rest | warn | wave1 |
| dbc | dbc.warn.no\_automated\_backups | no automated backups | warn | wave1 |
| dbc | dbc.maintenance-overdue | maintenance overdue | broken | wave2 |
| ddb | ddb.broken.kms\_key\_inaccessible | kms key inaccessible | broken | wave1 |
| ddb | ddb.broken.archived\_kms\_lost | archived: kms key lost | broken | wave1 |
| ddb | ddb.warn.creating | creating | warn | wave1 |
| ddb | ddb.warn.updating | updating | warn | wave1 |
| ddb | ddb.warn.deleting | deleting | warn | wave1 |
| ddb | ddb.warn.archiving | archiving | warn | wave1 |
| ddb | ddb.pitr-off | point-in-time recovery disabled | warn | wave2 |
| opensearch | opensearch.dim.deleting | deleting: removal in progress | dim | wave1 |
| opensearch | opensearch.broken.isolated | isolated: quarantined by AWS | broken | wave1 |
| opensearch | opensearch.warn.processing | processing: config change in flight | warn | wave1 |
| opensearch | opensearch.update-forced | software update forced soon | broken | wave2 |
| opensearch | opensearch.encryption-off | encryption at rest off | warn | wave2 |
| redshift | redshift.broken.incompatible\_hsm | incompatible-hsm | broken | wave1 |
| redshift | redshift.broken.incompatible\_network | incompatible-network | broken | wave1 |
| redshift | redshift.broken.incompatible\_parameters | incompatible-parameters | broken | wave1 |
| redshift | redshift.broken.incompatible\_restore | incompatible-restore | broken | wave1 |
| redshift | redshift.broken.hardware\_failure | hardware-failure | broken | wave1 |
| redshift | redshift.broken.storage\_full | storage-full | broken | wave1 |
| redshift | redshift.broken.unavailable | unavailable | broken | wave1 |
| redshift | redshift.broken.failed | failed | broken | wave1 |
| redshift | redshift.warn.creating | creating | warn | wave1 |
| redshift | redshift.warn.modifying | modifying | warn | wave1 |
| redshift | redshift.warn.resizing | resizing | warn | wave1 |
| redshift | redshift.warn.rebooting | rebooting | warn | wave1 |
| redshift | redshift.warn.renaming | renaming | warn | wave1 |
| redshift | redshift.warn.deleting | deleting | warn | wave1 |
| redshift | redshift.warn.maintenance | maintenance | warn | wave1 |
| redshift | redshift.warn.availability\_modifying | modifying | warn | wave1 |
| redshift | redshift.warn.pending\_change | pending change queued | warn | wave1 |
| redshift | redshift.warn.maintenance\_deferred | maintenance deferred | warn | wave1 |
| redshift | redshift.warn.publicly\_accessible | publicly accessible | warn | wave1 |
| redshift | redshift.warn.unencrypted\_at\_rest | unencrypted at rest | warn | wave1 |
| efs | efs.broken.error | error | broken | wave1 |
| efs | efs.broken.no\_mount\_targets | no mount targets | broken | wave1 |
| efs | efs.warn.creating | creating | warn | wave1 |
| efs | efs.warn.updating | updating | warn | wave1 |
| efs | efs.warn.deleting | deleting | warn | wave1 |
| efs | efs.mount-target-down | mount target down | broken | wave2 |
| dbi-snap | dbi-snap.broken.failed | failed | broken | wave1 |
| dbi-snap | dbi-snap.broken.incompatible | <incompatible-\* status> | broken | wave1 |
| dbi-snap | dbi-snap.warn.creating | creating: <pct>% | warn | wave1 |
| dbi-snap | dbi-snap.warn.unencrypted | unencrypted | warn | wave1 |
| dbi-snap | dbi-snap.orphan | orphan: source DB deleted | broken | wave2 |
| dbi-snap | dbi-snap.past-retention | automated, <N>d past retention | broken | wave2 |
| dbc-snap | dbc-snap.broken.failed | failed | broken | wave1 |
| dbc-snap | dbc-snap.broken.incompatible | <incompatible-\* status> | broken | wave1 |
| dbc-snap | dbc-snap.warn.creating | creating | warn | wave1 |
| dbc-snap | dbc-snap.warn.manual\_unused | manual, unused <N>d | warn | wave1 |
| dbc-snap | dbc-snap.warn.unencrypted | unencrypted | warn | wave1 |
| dbc-snap | dbc-snap.orphan | orphan: source cluster deleted | broken | wave2 |
| dbc-snap | dbc-snap.past-retention | automated, <N>d past retention | broken | wave2 |
| alarm | alarm.state.alarm | alarm triggered | broken | wave1 |
| alarm | alarm.state.insufficient\_data | insufficient data | warn | wave1 |
| alarm | alarm.no\_actions | no actions | warn | wave1 |
| logs | logs.retention-never-expire | retention: never expire | warn | wave1 |
| logs | logs.stale-empty | empty, created over 90 days ago | warn | wave1 |
| logs | logs.missing-metric-filters | audit log group missing metric filters | warn | wave2 |
| trail | trail.log-file-validation.disabled | log file validation disabled | warn | wave1 |
| trail | trail.not-logging | not logging | broken | wave2 |
| trail | trail.delivery-error | delivery error: <LatestDeliveryError> | broken | wave2 |
| trail | trail.delivery-stale | delivery stale since <LatestDeliveryTime> | broken | wave2 |
| ct-events | ct\_event.severity.danger | destructive call | broken | wave1 |
| ct-events | ct\_event.severity.attention | root account activity | warn | wave1 |
| ct-events | ct\_event.severity.info | routine event | dim | wave1 |
| sqs | sqs.missing-dlq | no DLQ configured | warn | wave2 |
| sns | sns.no-subscribers | topic has no subscribers | warn | wave2 |
| sns | sns.all-pending-confirmation | all pending confirmation | warn | wave2 |
| sns-sub | sns-sub.state.pending-confirmation | endpoint has not confirmed the subscription | warn | wave1 |
| sns-sub | sns-sub.state.deleted | endpoint deleted | dim | wave1 |
| eb | eb.health.red | health: red | broken | wave1 |
| eb | eb.health.yellow | health: yellow | warn | wave1 |
| eb | eb.health.grey | health: grey | warn | wave1 |
| eb | eb.status.terminated | terminated | dim | wave1 |
| eb | eb.status.launching | launching | warn | wave1 |
| eb | eb.status.terminating | terminating | dim | wave1 |
| eb | eb.environment-causes | EB causes: <first cause> | warn | wave2 |
| eb-rule | eb-rule.state.disabled | disabled | dim | wave1 |
| eb-rule | eb-rule.target-issue | enabled rule has no targets (rule matches but goes nowhere) | broken | wave2 |
| kinesis | kinesis.warn.creating | creating | warn | wave1 |
| kinesis | kinesis.warn.updating | updating | warn | wave1 |
| kinesis | kinesis.warn.deleting | deleting | warn | wave1 |
| msk | msk.warn.creating | creating | warn | wave1 |
| msk | msk.warn.updating | updating | warn | wave1 |
| msk | msk.warn.maintenance | maintenance | warn | wave1 |
| msk | msk.warn.rebooting\_broker | rebooting broker | warn | wave1 |
| msk | msk.warn.healing | healing | warn | wave1 |
| msk | msk.warn.deleting | deleting | warn | wave1 |
| msk | msk.broken.failed | failed | broken | wave1 |
| msk | msk.broker-outdated | broker software outdated | warn | wave2 |
| msk | msk.encryption-not-tls | encryption in transit not enforced | warn | wave2 |
| sfn | sfn.latest-execution-failed | latest execution <STATUS> | broken | wave2 |
| ses | ses.verification.failed | verification failed | broken | wave1 |
| ses | ses.verification.temp\_failure | verify: temp failure | broken | wave1 |
| ses | ses.verification.not\_started | verification not started | broken | wave1 |
| ses | ses.verification.pending | pending verification | warn | wave1 |
| ses | ses.sending.disabled | sending disabled | warn | wave1 |
| ses | ses.account-shutdown | sending paused by AWS (shutdown) | broken | wave2 |
| ses | ses.account-probation | account under review (probation) | broken | wave2 |
| ses | ses.quota-high | quota 80%+ used | warn | wave2 |
| secrets | secrets.state.deleted | deleted | broken | wave1 |
| secrets | secrets.state.rotation\_overdue | rotation overdue | warn | wave1 |
| secrets | secrets.state.dormant | dormant | warn | wave1 |
| secrets | secrets.rotation.disabled | rotation not enabled | warn | wave1 |
| secrets | secrets.value.stale | value unchanged in over 365 days | warn | wave1 |
| ssm | ssm.value.plaintext-sensitive | plaintext value looks like a credential | broken | wave1 |
| ssm | ssm.value.stale | not modified in over 365 days | warn | wave1 |
| kms | kms.state.pending\_deletion | pending deletion | broken | wave1 |
| kms | kms.state.disabled | disabled | warn | wave1 |
| kms | kms.state.unavailable | <key state> | broken | wave1 |
| kms | kms.access-denied | access denied (kms:DescribeKey) | broken | wave1 |
| kms | kms.rotation-disabled | key rotation disabled | warn | wave2 |
| r53 | r53.zone.unused | only default NS/SOA records remain | warn | wave1 |
| r53 | r53.orphan-private-zone | private zone with no VPC associations (orphan) | warn | wave2 |
| cf | cf.insecure-protocol | no HTTPS redirect (insecure); origin without TLS | warn | wave2 |
| acm | acm.expires-soon | expires in <N> days | broken | wave2 |
| acm | acm.orphan | certificate not in use (orphan) | warn | wave2 |
| apigw | apigw.no-deployed-stages | no deployed stages | warn | wave2 |
| apigw | apigw.stage-config-issues | no throttling configured (DoS risk); access logs disabled | warn | wave2 |
| role | role.trust.wildcard-principal | anyone can assume this role | broken | wave1 |
| role | iam-role.dormant | dormant role (>90d) | warn | wave2 |
| policy | iam-policy.orphan-unattached | unattached, no roles/users/groups use it | warn | wave1 |
| policy | iam-policy.admin-star | admin star (allows \* on \*) | broken | wave2 |
| iam-user | iam-user.no-mfa | console user without MFA | broken | wave2 |
| iam-user | iam-user.old-key | key <keyID> >90d (rotation) | warn | wave2 |
| iam-group | iam-group.orphan-or-noop | group has no members (orphan) | warn | wave2 |
| waf | waf.no-logging | no logging configuration | warn | wave2 |
| cfn | cfn.stack.failed | <status, lowercased> | broken | wave1 |
| cfn | cfn.stack.rollback | <status, lowercased> | broken | wave1 |
| cfn | cfn.stack.in\_progress | <status, lowercased> | warn | wave1 |
| cfn | cfn.stack.deleted | delete\_complete | dim | wave1 |
| cfn | cfn.recent-resource-failure | recent resource failure: <ResourceType/LogicalResourceId> | broken | wave2 |
| cfn | cfn.stack-drifted | stack drifted from template | warn | wave2 |
| pipeline | pipeline.stage-failed | stage <stage> failed | broken | wave2 |
| cb | cb.latest-build-failed | latest build <status> (<date>) | broken | wave2 |
| ecr | ecr.vulnerabilities | <N> critical, <M> high vulnerabilities | broken | wave2 |
| codeartifact | codeartifact.no-permissions-policy | no permissions policy | warn | wave2 |
| codeartifact | codeartifact.public-access-policy | public access policy | broken | wave2 |
| glue | glue.latest-run-failed | latest run <STATUS> | broken | wave2 |
| athena | athena.workgroup-disabled | disabled | warn | wave1 |
| athena | athena.governance-misconfigured | EnforceWorkGroupConfiguration (<N> findings) | warn | wave2 |
| backup | backup.job-failed | <N> jobs failed in last 24h | broken | wave2 |
| backup | backup.job-partial | partial: <N> of <M> resources skipped | warn | wave2 |
<!-- END GENERATED: findings-table -->
