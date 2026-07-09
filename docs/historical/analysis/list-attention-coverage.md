# List Attention Coverage — Verification Table

Audit of `.a9s/views/<type>.yaml` list columns against
`docs/attention-signals.md` (the Wave 1 / Wave 2 golden contract).

For each resource type, this table answers: **"Can a user looking at the list
view tell *what* is wrong, or only *that* something is wrong (color)?"**

Coloring alone forces every issue into a binary. A red row tells the user
"go look at this" but not "what to look at." This audit identifies where
the column set fails to discriminate the cause.

## Tiers (preferred → fallback)

| Tier | Action | Cost | When to use |
|---|---|---|---|
| **A** | Add a column from an existing AWS SDK field via `.a9s/views/*.yaml` only | Lowest — view edit, no Go code | The signal already lives on the list response (Wave 1) or in a populated `Fields[]` entry |
| **B** | Invent a computed column (`CellDecorator` or formatter) | Medium — Go code in fetcher or registry | The signal requires synthesis (cross-ref, Wave 2, threshold check, derived label) |
| **C** | Surface only in detail view via `FindingRow` (current `Background Check` section) | Highest — user must drill in | Long-form text (multi-line error message, list of action descriptions); too wide for a table cell |

## Symbols

- ✓ Status visible at a glance (column shows a status-like field)
- ~ Status partially visible (column exists but the issue cause is hidden)
- ✗ No status-like column — issue is invisible without color decoding

---

## Compute

| shortName | Today | Visible columns | Hidden signals (per golden) | Tier | Recommended change |
|---|---|---|---|---|---|
| `ec2` | ~ | Name, **State**, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time | `system_status` / `instance_status` (Wave 2) — a "running" instance with failed status checks shows green status. Long-stopped (>30d) hidden. `StateReason.Code` Server.* hidden | **A** | Add `Health` column reading `Fields["system_status"]` / `Fields["instance_status"]` — already populated by EC2 fetcher post-enrichment loop |
| `ecs-svc` | ✓ | Service Name, Cluster, **Status**, Desired, Running, Launch Type | `events[]` "unable to place" / "ELB checks failed" (Wave 2). Deployment circuit-breaker. | **C** | Detail-only — error text is multi-line |
| `ecs` | ✓ | Cluster Name, **Status**, Running, Pending, Services | `pendingTasksCount` sustained → already partially visible via Pending col | None | Adequate |
| `ecs-task` | ~ | Task ID, Cluster, **Status**, Task Definition, Launch, CPU, Memory | `StopCode` (UserInitiated vs TaskFailedToStart vs EssentialContainerExited). `healthStatus==UNHEALTHY` | **A** | Add `Stop Code` and `Health Status` columns from SDK |
| `lambda` | ~ | Function Name, **Runtime**, Memory, Timeout, **State**, Last Modified | `Runtime` is shown but user can't tell if it's deprecated. `LastUpdateStatus==Failed` hidden. `DeadLetterConfig==nil` hidden. | **B** | Decorate Runtime cell to mark deprecated (`python3.7 ⚠`). Add `DLQ` column (Yes/No). |
| `asg` | ✓ | ASG Name, Min, Max, Desired, Instances, **Status** | `SuspendedProcesses` hidden. Latest scaling activity Failed (Wave 2) hidden. InService<MinSize hidden. | **B** | Add `Suspended` column (count of suspended processes) and `Last Activity` column from Wave 2 enrichment |
| `eb` | ✓ | Environment, Application, **Status**, **Health**, Version | `Causes[]` non-empty (Wave 2) | **C** | Detail-only — Causes are multi-line |
| `ebs` | ✓ | Name, Volume ID, **State**, Size, Type, IOPS, **Encrypted**, Attached To, AZ, Created | `available` with CreateTime>7d → orphan (computed). VolumeStatus impaired (Wave 2) hidden in list. | **B** | Add computed `Orphan` indicator (state=available && age>7d). Wave-2 impaired surfaced via row color is fine for this one. |
| `ebs-snap` | ✓ | Name, Snapshot ID, **State**, Volume ID, Size, **Encrypted**, Description, Started, Progress | Age>365d auto = cost warn. Source volume deleted → orphan. | **B** | Computed `Age` column with auto-warn over 365d; `Orphan` indicator from cross-ref `ebs` |
| `ami` | ~ | Name, Image ID, **State**, Arch, Platform, Root Device, Created, Public | `DeprecationTime < now()` hidden. Backing snapshot missing (cross-ref) hidden. | **A** | Add `Deprecated` column from `DeprecationTime` (compare to now) |

## Containers

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `eks` | ✓ | Cluster Name, Version, **Status**, Endpoint, Platform Version | `health.issues[]` codes — what's wrong with the cluster | **B** | Add `Issues` column (count of health.issues entries from Wave 2 in-fetcher data) |
| `ng` | ✓ | Node Group, Cluster, **Status**, Instance Types, Desired | Same — `health.issues[]` codes (InsufficientFreeAddresses, AccessDenied, etc.) | **B** | Same — `Issues` column showing first issue code or count |

## Networking

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `elb` | ~ | Name, Type, Scheme, **State**, DNS Name, VPC ID | `State.Reason` for failed/impaired (text). | **A** | Add `State Reason` column from `State.Reason` SDK field |
| `tg` | ✗ | Target Group, Port, Protocol, VPC ID, Target Type, Health Check | Health (Wave 2) — healthy/unhealthy target counts. Orphan (LoadBalancerArns==[]). | **B** | Add computed `Health` column ("3/4 healthy" or "ORPHAN") from Wave 2 |
| `vpc` | ✓ | Name, VPC ID, CIDR Block, **State**, Default | Flow logs missing (Wave 2). No subnets (cross-ref). | **B** | Add `Flow Logs` column (Yes/No from Wave 2) and `Subnets` count |
| `subnet` | ~ | Name, Subnet ID, VPC ID, CIDR Block, AZ, **State**, **Available IPs** | `MapPublicIpOnLaunch=true` without IGW route → misconfigured public (cross-ref `rtb`). | **A** | Add `Public` column from `MapPublicIpOnLaunch` (Yes/No) — color cell red if true & RTB has no IGW |
| `rtb` | ✗ | Name, Route Table ID, VPC ID, Routes, Assoc. | `Routes[].State==blackhole` (dead targets). Unassociated (orphan). | **B** | Add computed `Issues` column ("1 blackhole" or "ORPHAN") |
| `nat` | ~ | Name, NAT Gateway ID, VPC ID, Subnet ID, **State**, Public IP | `FailureCode` / `FailureMessage` text on `failed` state | **A** | Add `Failure` column from `FailureCode` SDK field |
| `igw` | ~ | Name, IGW ID, VPC ID, **State** | Detached/orphan (Attachments==[]). IGW attached to VPC with no IGW route (cross-ref). | **A** | The State column already reflects `Attachments[].State`; rename header to "Attachment" for clarity. Add cross-ref via row color (already in fetcher) |
| `eip` | ✗ | Name, Allocation ID, Public IP, Association, Instance, Domain | **Unattached** (cost!). **Zombie** (attached to stopped EC2). | **B** | Add computed `Status` column: "unattached" / "zombie:i-xxx" / "OK". Critical — billed hourly when unattached. |
| `vpce` | ~ | Service Name, Endpoint ID, Type, **State**, VPC ID | `LastError` text. Interface endpoint NetworkInterfaceIds==[]. | **A** | Add `Last Error` column from `LastError.Message` SDK field |
| `tgw` | ~ | Name, TGW ID, **State**, Owner, Description | Failed/pendingAcceptance attachments (Wave 2). | **B** | Add `Att. Issues` column (count of failed/pending) from Wave 2 |
| `eni` | ✓ | Name, ENI ID, **Status**, Type, VPC ID, Private IP | Status `available` already signals orphan. Adequate. | None | Adequate (color signals orphan via Wave 1) |
| `sg` | ✗✗✗ | Group Name, Group ID, VPC ID, Description | **Open dangerous ports (0.0.0.0/0 → 22/3389/3306/etc)** — critical security risk INVISIBLE. Orphan SG (no ENI ref) hidden. | **B** | **Highest priority.** Add computed `Risk` column: "OPEN:22,3306" or "OPEN_ADMIN" or "ORPHAN" or "OK". Computed in fetcher from IpPermissions and dangerous_open_count Field. |

## Databases & Storage

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `dbi` | ~ | DB Identifier, Engine, Version, **Status**, Class, Endpoint, Multi-AZ | `PubliclyAccessible`, `StorageEncrypted=false`, `BackupRetentionPeriod=0`, `DeletionProtection=false` — all CIS controls hidden. Pending maintenance (Wave 2) hidden. | **B** | Add `CIS` column showing flags ("PUB|UNENC|NOPROT|NOBKP" or "OK"). Maintenance dates surface via row color + Background Check detail (already done). |
| `dbc` | ~ | Cluster ID, Version, **Status**, Instances, Endpoint | No writer (DBClusterMembers without IsClusterWriter==true) → critical. Same CIS flags as dbi. | **A** | Add `Writer` column (Yes/No) and **B** `CIS` column |
| `redis` | ~ | Cluster ID, Version, Node Type, **Status**, Nodes, Endpoint | `AutomaticFailover != enabled` on multi-AZ → warning | **A** | Add `Failover` column from `AutomaticFailover` SDK field |
| `ddb` | ~ | Table Name, **Status**, Items, Size, Billing | PITR disabled (Wave 2) hidden. | **A** | Add `PITR` column from Wave 2 enrichment data (already computed) |
| `opensearch` | ✗ | Domain Name, Engine Version, Instance Type, Instances, Endpoint | NO status column at all. `Processing`, `UpgradeProcessing`, `DomainProcessingStatus==Isolated`, software updates available. | **A** | Add `Status` column (DomainProcessingStatus or computed "Processing"/"Isolated"/"Available") |
| `redshift` | ~ | Cluster ID, **Status**, Node Type, Nodes, Database, Endpoint | `PendingModifiedValues`, `DeferredMaintenanceWindows[]`, PubliclyAccessible, Encrypted=false hidden | **A**+**B** | Add `Pending` column (count) and `CIS` flags column |
| `efs` | ✓ | Name, File System ID, **State**, Perf Mode, **Encrypted**, **Mounts** | NumberOfMountTargets visible. Mount target lifecycle (Wave 2) hidden. | None | Adequate; row color from Wave 2 sufficient |
| `s3` | ✗ | Bucket Name, Region, Creation Date | **Public access risk** (Wave 2 GetPublicAccessBlock) hidden — critical security signal | **A** | Add `Public Access` column from Wave 2 enrichment ("BLOCKED" / "RISK" / "?") |
| `dbi-snap` | ~ | Snapshot ID, DB Instance, **Status**, Engine, Type, Created | `Encrypted=false` (CIS RDS.4) hidden. Orphan (source DB deleted) hidden. | **A** | Add `Encrypted` column from SDK |
| `dbc-snap` | ✓ | Snapshot ID, Cluster ID, **Status**, Engine, Type, Created, Storage | Manual snapshot age >365d (cost) | **B** | Add `Age` computed column with cost-warn over 365d |

## Messaging

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `sqs` | ~ | Queue Name, **Messages**, In Flight, Delay, Queue URL | Backlog vs threshold (Wave 2). Age of oldest message > VisibilityTimeout×5. Missing redrive policy. | **A**+**B** | Add `DLQ` column from Wave 2 RedrivePolicy presence; Messages col already visible — add color decorator |
| `sns` | ✗ | Topic Name, Topic ARN | Subscriptions count (Wave 2). Orphan (no subs). | **B** | Add computed `Subs` column from Wave 2 |
| `sns-sub` | ✗ | Topic ARN, Protocol, Endpoint, Subscription ARN | `SubscriptionArn == "PendingConfirmation"` hidden | **A** | Add `Confirmed` column (Yes/No from SubscriptionArn check) |
| `eb-rule` | ~ | Rule Name, **State**, Event Bus, Schedule, Description | Targets-empty on enabled rule → broken (Wave 2). Disabled rule with targets → warn. | **A** | Add `Targets` column (count from Wave 2 ListTargetsByRule) |
| `kinesis` | ✓ | Stream Name, **Status**, Mode, Created | Adequate | None | — |
| `msk` | ✓ | Cluster Name, Type, **State**, Version | Adequate | None | — |
| `sfn` | ✗ | Name, Type, ARN, Created | Latest execution failure (Wave 2). Consecutive failures. | **B** | Add `Last Run` computed column from Wave 2 ("FAILED 2h ago" / "OK") |

## Secrets & Config

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `secrets` | ~ | Secret Name, Description, **Last Accessed**, Last Changed, **Rotation** | Overdue rotation (now > NextRotationDate). Dormant (LastAccessed > 180d). | **B** | Add `Status` computed column ("OVERDUE" / "DORMANT" / "OK") |
| `ssm` | ✗ | Name, Type, Version, **Last Modified**, Description | SecureString stale > 365d. Suspicious String name (-secret/-password). Advanced unused. | **B** | Add `Risk` computed column ("STALE" / "SHOULD_ENCRYPT" / "COSTS") |
| `kms` | ~ | Alias, Key ID, **Status**, Description | `KeyRotationEnabled==false` (Wave 2) hidden. | **A** | Add `Rotation` column from Wave 2 GetKeyRotationStatus |

## Security & IAM

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `role` | ~ | Role Name, **Last Used**, Path, Created, Description | Wildcard principal in trust policy (Wave 1!) hidden. RoleLastUsed >90d already visible via Last Used. | **A** | Add `Trust` column flagging "WILDCARD" if `Principal:{"AWS":"*"}` without external-id |
| `policy` | ~ | Policy Name, Type, **Attached**, Path, Created | Wildcard admin policy doc (Wave 2 GetPolicyVersion) hidden. Orphan visible via Attached==0. | **B** | Add `Risk` column ("ADMIN_*" or "ORPHAN" or "OK") |
| `iam-user` | ~ | User Name, User ID, Path, Created, **Password Last Used** | MFA status, access key age, never-used keys (Wave 2) hidden. Console-login-without-MFA → broken. | **B** | Add `Risk` column ("NO_MFA" / "OLD_KEY" / "DORMANT" / "OK") |
| `iam-group` | ✗ | Group Name, Group ID, Path, Created, ARN | Empty group (Wave 2) hidden. | **A** | Add `Members` column (count from Wave 2 GetGroup) |
| `waf` | ✗ | Name, ID, Description | `Rules==[]` (no-op ACL). `DefaultAction==Allow` with zero rules (broken). | **B** | Add `Rules` column from Wave 2 (count) and color when 0 |

## DNS, CDN, Certs

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `r53` | ✓ | Name, Zone ID, **Records**, Private, Comment | Records count visible — adequate for "<=2 = unused" inference | None | — |
| `cf` | ~ | Domain Name, Distribution ID, **Status**, Enabled, Aliases, Price Class | TLS version, WAF presence, Logging.Enabled hidden. | **A** | Add `WAF` (Yes/No from `WebACLId != ""`) and `TLS` (`MinimumProtocolVersion`) columns |
| `acm` | ✓ | Domain Name, **Status**, Type, **Expires**, **In Use** | Days-to-expiry needs user math | **B** | Add `Days Left` computed column (NotAfter - now()) — color <30d warn, <7d broken |
| `apigw` | ✗ | Name, API ID, Protocol, Endpoint, Description | Stages count (Wave 2). Orphan API (no stages) hidden. | **B** | Add `Stages` column from Wave 2 |

## Monitoring

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `alarm` | ✓ | Alarm Name, **State**, Metric, Namespace, Threshold | `ActionsEnabled==false` (muted) hidden. Zombie alarm (sibling missing) needs cross-ref. | **A** | Add `Actions` column (Yes/No from ActionsEnabled and len(AlarmActions)>0) |
| `logs` | ✓ | Log Group Name, Size, **Retention**, Metric Filters, Created | Stale log group (no events recent) hidden. KMS-pending-delete cross-ref hidden. | **A** | Add `Last Event` column from Wave 2 DescribeLogStreams (most recent timestamp or "stale") |
| `trail` | ✓ | Trail Name, S3 Bucket, Home Region, Multi-Region | `LatestDeliveryTime >1h` on logging trail hidden. | **B** | Fetcher already reads `GetTrailStatus` and colors on `is_logging==false`, `latest_delivery_error!=""`, `log_file_validation_enabled==false`. Remaining: add `Logging` and `Last Delivery` columns (presentation-only). |
| `ct-events` | n/a | V, TIME, ACTOR, ORIGIN, EVENT, TARGET, OUTCOME | OUTCOME column already shows pass/fail | None | Adequate |

## CI/CD

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `cfn` | ✓ | Stack Name, **Status**, Created, Updated, Description | Drift status (Wave 2) hidden. Stuck >1h needs Updated col interpretation. | **A** | Add `Drift` column from Wave 2 (DRIFTED / IN_SYNC / NOT_CHECKED) |
| `pipeline` | ✗ | Pipeline Name, Type, Version, Created, Updated | Latest stage failure (Wave 2). Stuck stage. | **B** | Add `Last Status` computed column from Wave 2 GetPipelineState |
| `cb` | ✗ | Project Name, Source Type, Description, Last Modified | Latest build status (Wave 2). | **B** | Add `Last Build` computed column from Wave 2 |
| `ecr` | ~ | Repository, URI, Tag Mutability, **Scan**, Created | Vuln counts (Wave 2 — currently disabled) hidden. | **B** | Add `Vulns` column ("CRIT:3" / "HIGH:5" / "OK") once ECR enricher is reimplemented (currently TODO'd) |
| `codeartifact` | ✗ | Repository, Domain, Description, Owner | Empty repo + age >30d (Wave 2). | **B** | Add `Packages` column from Wave 2 ListPackages |

## Data & Analytics

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `glue` | ✗ | Job Name, Version, Worker Type, Workers, Last Modified | Latest run failure (Wave 2). | **B** | Add `Last Run` computed column from Wave 2 GetJobRuns |
| `athena` | ✓ | Workgroup, **State**, Description, Engine | EnforceWorkGroupConfiguration hidden. Cost-cutoff unset hidden. | **A** | Add `Cost Cap` column (Yes/No from BytesScannedCutoffPerQuery) |

## Backup & Email

| shortName | Today | Visible columns | Hidden signals | Tier | Recommended change |
|---|---|---|---|---|---|
| `backup` | ~ | Plan Name, Plan ID, Created, **Last Execution** | Last execution status hidden — Last Execution is timestamp only. | **B** | Add `Last Status` column from Wave 2 (FAILED / PARTIAL / OK) |
| `ses` | ✓ | Identity, Type, **Verification**, **Sending** | Account-wide EnforcementStatus PROBATION/SHUTDOWN hidden. SendQuota near limit hidden. | **B** | Add account-wide banner (not per-row) for PROBATION/SHUTDOWN; per-row adequate |

---

## Priority gaps (ranked by user impact)

1. **`sg`** — Open dangerous ports invisible. Critical security signal. Tier B, computed Risk column.
2. **`s3`** — Public access risk invisible (only color). Tier A, Public Access column.
3. **`trail`** — now colored on `IsLogging==false` / delivery error / no log-file validation; remaining gap is presentation (Logging + Last Delivery columns). Tier B.
4. **`eip`** — Unattached (billed hourly) invisible. Tier B, Status column.
5. **`opensearch`** — No status column at all. Tier A, add Status.
6. **`tg`** — No health column. Tier B, computed Health column.
7. **`dbi`** / **`dbc`** — CIS flags (public/unencrypted/no-backup/no-protection) all invisible. Tier B, CIS column.
8. **`iam-user`** — MFA / old keys / dormant invisible. Tier B, Risk column.
9. **`pipeline`** / **`cb`** / **`glue`** — Latest run status invisible. Tier B, Last Run/Build column.
10. **`cfn`** — Drift status invisible. Tier A, Drift column from Wave 2.

## Implementation pattern

**Tier A** changes are pure `.a9s/views/<type>.yaml` edits — add an entry under
`list:` with `path: <SDK field path>` (or `key: <Fields map key>` for
fetcher-populated fields). No Go code, no test changes beyond column-count
assertions.

**Tier B** changes need:
- A computed value, either: (a) populated by the fetcher into `Fields[]`, or
  (b) emitted by a `CellDecorator` registered in the resource type.
- The column entry in `.a9s/views/<type>.yaml` referencing the new key.
- A unit test asserting the computed value for representative inputs.

**Tier C** changes need:
- A new per-type case in `injectEnrichmentSection` (or rely on the generic
  "Background Check" fallback added in PR #273).

## Wave-2 dependency

Many Tier-A/B recommendations require data that already lives in
`Fields[]` after the Wave-2 enricher runs (e.g. `health_issues_count`,
`dangerous_open_count`). Adding a column for these is a YAML-only change.
Where the data is *not* yet populated (e.g. ECR vuln counts — enricher
disabled per the `EnrichECRRepository` TODO), the YAML change must wait
for the fetcher/enricher to land.
