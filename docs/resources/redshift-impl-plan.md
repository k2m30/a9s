---
shortName: redshift
source: docs/resources/redshift.md
generatedBy: a9s-implement-resource skill
---

# redshift — Implementation Plan

Pseudocode test spec, fixture list, and contract-surface gap analysis derived from `docs/resources/redshift.md`. This doc is the handoff contract for phases 6a, 6b, and 7. The spec doc is authoritative; if this plan and the spec disagree, the spec wins and this plan is regenerated.

## §0. Phase-2 TBD resolution

No `TBD` markers in the spec. Every §3 signal, §2 pivot, and §6 citation is resolved. Phase 2 is a no-op.

## §0.1 Universal coverage matrix

| ID  | Invariant | Fixture ID(s) | Test |
|-----|-----------|---------------|------|
| U1  | Healthy blank S4 | `acme-warehouse`, `acme-reporting`, `staging-dwh` (paused — Out of Scope, treated Healthy) | `ExpectRowStatusBlank` |
| U2  | Warning / Broken §4 phrase | one per §3.1 signal (resizing, rebooting, incompatible-network, hardware-failure, storage-full, unavailable, failed, avail-maintenance, avail-modifying, pending-change, maintenance-deferred, publicly-accessible, unencrypted) | `ExpectRowStatusEquals` |
| U3  | `~` glyph on Healthy + ~ finding | N/A — Wave 2 = None | SKIP |
| U4  | `!` glyph on Healthy + ! finding | N/A — Wave 2 = None | SKIP |
| U5  | No glyph on non-green rows | every warning / broken fixture | `ExpectRowNoGlyphPrefix` |
| U6  | S1 badge counts `!`-severity instances | all fixtures | `ExpectMenuIssueCount(redshift, 0)` — Wave 2 = None → no `!` findings |
| U7a | Multi-W1 `<top> (+N-1)` suffix | `warn-redshift-multi` (avail + pending + publicly + unencrypted) | `ExpectRowStatusEquals(..., "pending change queued (+2)")` |
| U7b | Wave-1 + Wave-2 stacking | N/A — Wave 2 = None | SKIP |
| U7c | S5 lists every Wave-2 finding | N/A — Wave 2 = None | SKIP |
| U7d | `!` beats `~` | N/A — Wave 2 = None | SKIP |
| U7e | S5 lists every Wave-1 phrase | `warn-redshift-multi` | `ExpectViewContains` on detail for each `Resource.Issues` entry (capitalize first letter) |
| U7f | `Resource.Issues` populated in §4 precedence | every warn/broken fixture | unit test: `got.Issues` deep-equals expected ordered slice |
| U8  | Broken > Warning > ~ precedence | N/A for redshift (no ! or ~ in Wave 2) — the fetcher must still return the Broken phrase when ClusterAvailabilityStatus=Unavailable overrides a Warning-bucket ClusterStatus | phrase precedence test (see §1 case #5b) |
| U9  | Related pivot counts (count shown: yes) | `acme-warehouse` (CloudWatch-logging graph-root) covers alarm/cfn/kms/logs/role/secrets/sg/subnet/vpc/ct-events; `acme-reporting` (S3-logging graph-root) covers s3 (logs/s3 are mutually exclusive per AWS — see §5.1 note) | `ExpectRelatedRowCountAtLeast(..., 1)` per pivot, across the two graph-roots |
| U10 | No jargon columns | all fixtures | `ExpectViewNotContains("CIS","Flags","Policy","Issues","NOBKP","UNENC","PUB","NOPROT")` |
| U11 | Summary ≠ Rows content | N/A — Wave 2 = None | SKIP |

## §1. Pseudocode test spec

Every row = one case. All GIVEN conditions use AWS field names from spec §3.

### Wave 1 signals — §3.1

```text
TEST: healthy_silent
GIVEN: a Redshift cluster with ClusterStatus=available, ClusterAvailabilityStatus=Available,
       PubliclyAccessible=false, Encrypted=true, no PendingModifiedValues, no active
       DeferredMaintenanceWindows.
WHEN:  the list is fetched and rendered.
THEN:
  - row color green
  - S4 (Status cell) renders blank — no "OK", "ACTIVE", "available", "healthy", or "-"
  - Resource.Status == ""
  - Resource.Issues == nil (or empty slice)
  - no `!` / `~` glyph

TEST: transitional_resizing
GIVEN: ClusterStatus=resizing; all other warning fields clean.
WHEN:  fetched + rendered.
THEN:
  - row color yellow
  - S4 == "resizing"
  - Resource.Issues == ["resizing"]
  - S5 detail contains "Cluster is resizing; queries may be intermittently unavailable."
  - no glyph

TEST: transitional_rebooting
GIVEN: ClusterStatus=rebooting.
THEN: S4 == "rebooting", Issues == ["rebooting"], yellow, no glyph.

TEST: broken_incompatible_network
GIVEN: ClusterStatus=incompatible-network.
THEN:
  - row red
  - S4 == "broken: incompatible-network"
  - Resource.Issues == ["broken: incompatible-network"]
  - S5 detail contains "Cluster cannot start: incompatible-network. Inspect parameter group / HSM / VPC settings."
  - no glyph

TEST: broken_hardware_failure
GIVEN: ClusterStatus=hardware-failure.
THEN: S4 == "broken: hardware-failure", Issues == ["broken: hardware-failure"], red, no glyph.

TEST: broken_storage_full
GIVEN: ClusterStatus=storage-full.
THEN: S4 == "broken: storage-full", Issues == ["broken: storage-full"], red, no glyph.

TEST: avail_unavailable
GIVEN: ClusterStatus=available, ClusterAvailabilityStatus=Unavailable.
THEN: S4 == "unavailable", Issues == ["unavailable"], red, no glyph.

TEST: avail_failed
GIVEN: ClusterStatus=available, ClusterAvailabilityStatus=Failed.
THEN: S4 == "failed", Issues == ["failed"], red, no glyph.

TEST: avail_maintenance
GIVEN: ClusterStatus=available, ClusterAvailabilityStatus=Maintenance.
THEN: S4 == "maintenance", Issues == ["maintenance"], yellow, no glyph.

TEST: avail_modifying
GIVEN: ClusterStatus=available, ClusterAvailabilityStatus=Modifying.
THEN: S4 == "modifying", Issues == ["modifying"], yellow, no glyph.

TEST: pending_change
GIVEN: ClusterStatus=available, PendingModifiedValues.NodeType="ra3.4xlarge".
THEN: S4 == "pending change queued", Issues == ["pending change queued"], yellow, no glyph.

TEST: maintenance_deferred_active
GIVEN: ClusterStatus=available; one DeferredMaintenanceWindow with
       DeferMaintenanceStartTime = (now - 1h), DeferMaintenanceEndTime = (now + 48h).
THEN: S4 == "maintenance deferred", Issues == ["maintenance deferred"], yellow, no glyph.

TEST: maintenance_deferred_expired   (negative case — must NOT trigger)
GIVEN: ClusterStatus=available; one DeferredMaintenanceWindow with
       DeferMaintenanceEndTime in the past.
THEN: S4 blank, Issues empty, green — expired windows are not active.

TEST: publicly_accessible
GIVEN: ClusterStatus=available, PubliclyAccessible=true.
THEN: S4 == "publicly accessible", Issues == ["publicly accessible"], yellow, no glyph.

TEST: unencrypted
GIVEN: ClusterStatus=available, Encrypted=false.
THEN: S4 == "unencrypted at rest", Issues == ["unencrypted at rest"], yellow, no glyph.
```

### Rule-7 multi-finding cases

```text
TEST: multi_w1_pending_plus_public_plus_unencrypted   (covers U7a, U7f)
GIVEN: ClusterStatus=available, ClusterAvailabilityStatus=Available,
       PendingModifiedValues.NodeType="ra3.4xlarge",
       PubliclyAccessible=true, Encrypted=false.
       (Three coexisting §3.1 Warnings.)
WHEN:  fetched.
THEN:
  - Resource.Status == "pending change queued (+2)"
  - Resource.Issues == ["pending change queued", "publicly accessible", "unencrypted at rest"]
    — §4 precedence: pending-change (row 9) > publicly-accessible (row 11) > unencrypted (row 12)
  - row color yellow
  - no glyph

TEST: multi_w1_detail_enumerates_every_phrase   (covers U7e)
GIVEN: same fixture as multi_w1_pending_plus_public_plus_unencrypted.
WHEN:  OpenDetailResource on it.
THEN:
  - rendered detail contains "Pending change queued"
  - rendered detail contains "Publicly accessible"
  - rendered detail contains "Unencrypted at rest"
    — Attention section applies capitalizeFirst at render time; data stays lowercase.

TEST: multi_w1_two_warnings_suffix_plus_1
GIVEN: ClusterStatus=available, PubliclyAccessible=true, Encrypted=false.
THEN: Resource.Status == "publicly accessible (+1)",
      Resource.Issues == ["publicly accessible", "unencrypted at rest"].
```

### Precedence / severity tests — §4 rule 7

```text
TEST: broken_cluster_status_beats_availability_modifying   (covers U8)
GIVEN: ClusterStatus=storage-full, ClusterAvailabilityStatus=Modifying.
THEN:
  - Resource.Status == "broken: storage-full"
  - Resource.Issues == ["broken: storage-full"]
    — Broken severity wins over Warning; only the Broken phrase surfaces.
  - row red (NOT yellow)
  - no glyph

TEST: broken_availability_beats_warning_publicly_accessible
GIVEN: ClusterStatus=available, ClusterAvailabilityStatus=Unavailable, PubliclyAccessible=true.
THEN:
  - Resource.Status == "unavailable"
  - Resource.Issues == ["unavailable"]
    — When Broken is active, Warning phrases are suppressed (severity precedence).
  - row red
```

### Anti-tests — §3.3 Wave 3 OUT OF SCOPE

```text
TEST: anti_cloudwatch_disk_space
GIVEN: any cluster (healthy or otherwise).
WHEN:  fetcher runs (no CloudWatch API call is made by the fetcher).
THEN:
  - No PercentageDiskSpaceUsed-derived phrase appears in Resource.Status or Resource.Issues
    for any fixture in the suite.

TEST: anti_cloudwatch_health_status
GIVEN: any cluster.
THEN:  No HealthStatus-derived phrase surfaces anywhere.
```

### Silence test — universal

```text
TEST: silence_on_all_healthy
GIVEN: any of the graph-root healthy fixtures (acme-warehouse, acme-reporting).
THEN:
  - Resource.Status == ""
  - Resource.Issues empty
  - S4 blank
```

### Related-panel tests — one per §2 pivot

```text
TEST: related_alarm   (CW Alarms)
GIVEN: the graph-root cluster acme-warehouse AND alarm-cache containing 2 MetricAlarms
       whose Dimensions[] has {Name:"ClusterIdentifier", Value:"acme-warehouse"} plus
       other unrelated alarms.
THEN: checkRedshiftAlarms returns Count == 2, IDs == those 2 alarms' IDs.

TEST: related_cfn
GIVEN: cluster Tags include {Key:"aws:cloudformation:stack-name", Value:"acme-warehouse-stack"}
       AND cfn-cache contains a Stack with that name.
THEN: checkRedshiftCFN returns Count == 1, IDs == ["acme-warehouse-stack"].

TEST: related_kms
GIVEN: cluster KmsKeyId == "arn:aws:kms:us-east-1:...:key/kms-redshift-1".
THEN: checkRedshiftKMS returns Count == 1, IDs == ["kms-redshift-1"].

TEST: related_logs_cloudwatch_multi_export
GIVEN: DescribeLoggingStatus returns LoggingEnabled=true, LogDestinationType=cloudwatch,
       LogExports=["connectionlog","userlog","useractivitylog"].
THEN: checkRedshiftLogs returns Count == 3, IDs ==
      ["/aws/redshift/cluster/acme-warehouse/connectionlog",
       "/aws/redshift/cluster/acme-warehouse/userlog",
       "/aws/redshift/cluster/acme-warehouse/useractivitylog"].

TEST: related_logs_s3_mode_returns_zero
GIVEN: DescribeLoggingStatus returns LoggingEnabled=true, LogDestinationType=s3.
THEN: checkRedshiftLogs returns Count == 0.

TEST: related_logs_disabled_returns_zero
GIVEN: DescribeLoggingStatus returns LoggingEnabled=false.
THEN: checkRedshiftLogs returns Count == 0.

TEST: related_role
GIVEN: Cluster.IamRoles = [
         {IamRoleArn: "arn:aws:iam::123456789012:role/redshift-copy-role"},
         {IamRoleArn: "arn:aws:iam::123456789012:role/redshift-unload-role"},
       ].
THEN: checkRedshiftRole returns Count == 2,
      IDs == ["redshift-copy-role","redshift-unload-role"] (bare names, not ARNs).

TEST: related_s3_bucket_when_s3_logging
GIVEN: DescribeLoggingStatus returns LoggingEnabled=true, LogDestinationType=s3,
       BucketName="acme-redshift-audit".
THEN: checkRedshiftS3 returns Count == 1, IDs == ["acme-redshift-audit"].

TEST: related_s3_returns_zero_when_cloudwatch_mode
GIVEN: LogDestinationType=cloudwatch.
THEN: checkRedshiftS3 returns Count == 0.

TEST: related_secrets
GIVEN: Cluster.MasterPasswordSecretArn ==
       "arn:aws:secretsmanager:us-east-1:123456789012:secret:redshift!acme-warehouse-AbCdEf"
       AND secrets-cache contains a Secret whose Fields["arn"] equals that ARN.
THEN: checkRedshiftSecrets returns Count == 1, IDs == [that secret's ID].

TEST: related_sg
GIVEN: Cluster.VpcSecurityGroups = [
         {VpcSecurityGroupId: "sg-warehouse-1"},
         {VpcSecurityGroupId: "sg-warehouse-2"},
       ].
THEN: checkRedshiftSG returns Count == 2, IDs == ["sg-warehouse-1","sg-warehouse-2"].

TEST: related_subnet
GIVEN: Cluster.ClusterSubnetGroupName="redshift-prod-subnet-group"
       AND DescribeClusterSubnetGroups returns one group with Subnets:
         [{SubnetIdentifier:"subnet-prod-a"},{SubnetIdentifier:"subnet-prod-b"}].
THEN: checkRedshiftSubnet returns Count == 2, IDs == ["subnet-prod-a","subnet-prod-b"].

TEST: related_vpc
GIVEN: Cluster.VpcId == "vpc-prod".
THEN: checkRedshiftVPC returns Count == 1, IDs == ["vpc-prod"].

TEST: related_alarm_negative_no_match
GIVEN: cluster X AND cache of alarms, none whose dimension matches X.
THEN: checkRedshiftAlarms returns Count == 0.

TEST: related_s3_log_destination_unset_returns_zero
GIVEN: DescribeLoggingStatus returns LoggingEnabled=true, BucketName=nil/"".
THEN: checkRedshiftS3 returns Count == 0.
```

### Column / view config anti-tests

```text
TEST: no_jargon_columns_in_list_view
GIVEN: redshift list rendered against any fixture.
THEN: rendered view does NOT contain "CIS", "Flags", "Policy", "Issues", "NOBKP",
      "UNENC", "PUB", "NOPROT" anywhere in the column headers.
```

## §2. Fixture list (plain language)

One Cluster per test case. Cross-file fixture entries (alarm, cfn, kms, secrets, sg, subnet) live in sibling fixture files per `a9s-create-demo-fixture` skill's graph plan. The `6a` coder writes `internal/demo/fixtures/redshift.go` AND the targeted sibling additions below.

### Primary (healthy / graph-roots)

```text
FIXTURE: acme-warehouse   (graph-root #1: CloudWatch-logging variant — U9 coverage)
A healthy production warehouse cluster.
ClusterIdentifier = "acme-warehouse".
ClusterStatus = "available".
ClusterAvailabilityStatus = "Available".
NodeType = "ra3.xlplus". NumberOfNodes = 4.
DBName = "analytics". MasterUsername = "admin".
ClusterCreateTime = 2025-03-10T09:00:00Z.
AvailabilityZone = "us-east-1a".
VpcId = "vpc-prod".
ClusterSubnetGroupName = "redshift-prod-subnet-group".
ClusterNamespaceArn = "arn:aws:redshift:us-east-1:123456789012:namespace:acme-warehouse".
Endpoint.Address = "acme-warehouse.c9xyz123.us-east-1.redshift.amazonaws.com". Port = 5439.
KmsKeyId = "arn:aws:kms:us-east-1:123456789012:key/kms-redshift-1".
MasterPasswordSecretArn = "arn:aws:secretsmanager:us-east-1:123456789012:secret:redshift!acme-warehouse-AbCdEf".
PubliclyAccessible = false. Encrypted = true.
VpcSecurityGroups = [
  {VpcSecurityGroupId: "sg-warehouse-1", Status: "active"},
  {VpcSecurityGroupId: "sg-warehouse-2", Status: "active"},
].
IamRoles = [
  {IamRoleArn: "arn:aws:iam::123456789012:role/redshift-copy-role", ApplyStatus: "in-sync"},
  {IamRoleArn: "arn:aws:iam::123456789012:role/redshift-unload-role", ApplyStatus: "in-sync"},
].
Tags = [
  {Key: "aws:cloudformation:stack-name", Value: "acme-warehouse-stack"},
  {Key: "Environment", Value: "prod"},
].
```

```text
FIXTURE: acme-reporting   (graph-root #2: S3-logging variant — closes s3 pivot)
Healthy reporting cluster with S3 audit logging.
Same identity / VPC / SG / role / KMS / secret / tags shape as acme-warehouse (adjust IDs:
role/redshift-reporting-copy-role, sg-reporting-1/2, stack-name acme-reporting-stack,
KmsKeyId kms-redshift-2, MasterPasswordSecretArn redshift!acme-reporting-XxYyZz).
ClusterIdentifier = "acme-reporting".
ClusterStatus = "available".
NodeType = "ra3.xlplus". NumberOfNodes = 2.
DBName = "reporting".
VpcId = "vpc-prod".
ClusterSubnetGroupName = "redshift-prod-subnet-group".
(Logging state lives in the typed-fake DescribeLoggingStatus response, see §2.1 Sibling fixtures.)
```

```text
FIXTURE: staging-dwh   (out-of-scope state — healthy fallback)
ClusterIdentifier = "staging-dwh".
ClusterStatus = "paused".
(Treated as Healthy because `paused` is in §5 Out of Scope — Issues empty, Status blank, row green.)
VpcId = "vpc-staging". ClusterSubnetGroupName = "redshift-staging-subnet-group".
NodeType = "dc2.large". NumberOfNodes = 2.
```

### Wave 1 transitional — Warning bucket (§3.1)

```text
FIXTURE: redshift-resizing         ClusterStatus=resizing.
FIXTURE: redshift-rebooting        ClusterStatus=rebooting.
```

### Wave 1 broken — Broken bucket (§3.1)

```text
FIXTURE: redshift-incompatible-network   ClusterStatus=incompatible-network.
FIXTURE: redshift-hardware-failure       ClusterStatus=hardware-failure.
FIXTURE: redshift-storage-full           ClusterStatus=storage-full.
```

### ClusterAvailabilityStatus-driven (§3.1)

```text
FIXTURE: redshift-avail-unavailable   ClusterStatus=available, ClusterAvailabilityStatus=Unavailable.
FIXTURE: redshift-avail-failed        ClusterStatus=available, ClusterAvailabilityStatus=Failed.
FIXTURE: redshift-avail-maintenance   ClusterStatus=available, ClusterAvailabilityStatus=Maintenance.
FIXTURE: redshift-avail-modifying     ClusterStatus=available, ClusterAvailabilityStatus=Modifying.
```

### PendingModifiedValues / DeferredMaintenanceWindows / Publicly accessible / Unencrypted (§3.1)

```text
FIXTURE: redshift-pending-change
ClusterStatus=available; PendingModifiedValues.NodeType="ra3.4xlarge".

FIXTURE: redshift-maintenance-deferred
ClusterStatus=available; DeferredMaintenanceWindows = [
  {DeferMaintenanceStartTime: now-1h, DeferMaintenanceEndTime: now+48h,
   DeferMaintenanceIdentifier:"dmw-active"},
].

FIXTURE: redshift-maintenance-deferred-expired   (negative / anti-case)
ClusterStatus=available; DeferredMaintenanceWindows = [
  {DeferMaintenanceStartTime: now-72h, DeferMaintenanceEndTime: now-24h,
   DeferMaintenanceIdentifier:"dmw-expired"},
].
Expected: Healthy (window is past).

FIXTURE: redshift-publicly-accessible
ClusterStatus=available; PubliclyAccessible=true.

FIXTURE: redshift-unencrypted
ClusterStatus=available; Encrypted=false.
```

### Multi-finding (rule 7)

```text
FIXTURE: warn-redshift-multi   (U7a / U7e / U7f vehicle)
ClusterStatus=available; PendingModifiedValues.NodeType="ra3.4xlarge";
PubliclyAccessible=true; Encrypted=false.
Expected Resource.Status = "pending change queued (+2)".
Expected Resource.Issues = ["pending change queued", "publicly accessible", "unencrypted at rest"].

FIXTURE: warn-redshift-two     (intermediate suffix case — +1)
ClusterStatus=available; PubliclyAccessible=true; Encrypted=false.
Expected Resource.Status = "publicly accessible (+1)".
Expected Resource.Issues = ["publicly accessible", "unencrypted at rest"].
```

### Severity-precedence vehicles (U8)

```text
FIXTURE: redshift-broken-with-warning-hidden
ClusterStatus=storage-full; ClusterAvailabilityStatus=Modifying; PubliclyAccessible=true;
Encrypted=false.
Expected: Resource.Status = "broken: storage-full",
          Resource.Issues = ["broken: storage-full"] (Broken suppresses the Warnings).
          Row red.

FIXTURE: redshift-avail-unavailable-with-warning-hidden   (same shape, availability-driven)
ClusterStatus=available; ClusterAvailabilityStatus=Unavailable; PubliclyAccessible=true.
Expected: Resource.Status = "unavailable", Issues = ["unavailable"].
```

### §2.1 Sibling fixtures (cross-file — 6a graph plan)

For every graph-root pivot in U9, the referenced target type MUST have a matching fixture entry. `a9s-create-demo-fixture` phase 2 produces this plan automatically; below is the required list so phase 7.5 scope knows which sibling files may appear in the coder's diff.

| Target file | Additions |
|-------------|-----------|
| `internal/demo/fixtures/cloudwatch.go` | 2 alarms with `Dimensions=[ClusterIdentifier=acme-warehouse]` + 1 alarm for `acme-reporting` (covers alarm pivot ≥2 on graph-root #1, ≥1 on graph-root #2) |
| `internal/demo/fixtures/cfn.go` | Stack `acme-warehouse-stack` + Stack `acme-reporting-stack` |
| `internal/demo/fixtures/kms.go` | Keys `kms-redshift-1`, `kms-redshift-2` |
| `internal/demo/fixtures/secrets.go` | Secrets with ARNs matching MasterPasswordSecretArn on both graph-roots |
| `internal/demo/fixtures/ec2.go` | Security Groups `sg-warehouse-1`, `sg-warehouse-2`, `sg-reporting-1`, `sg-reporting-2`; Subnets `subnet-prod-a`, `subnet-prod-b`, `subnet-staging-a`, `subnet-staging-b`; VPCs `vpc-prod`, `vpc-staging` (if any of these missing) |
| `internal/demo/fixtures/iam.go` | Roles `redshift-copy-role`, `redshift-unload-role`, `redshift-reporting-copy-role` |
| `internal/demo/fixtures/s3.go` | Bucket `acme-redshift-audit` (receives acme-reporting's S3 audit logs) |
| `internal/demo/fixtures/cloudtrail.go` | ≥2 events with `ResourceName=acme-warehouse` for ct-events pivot ≥2 |
| `internal/demo/fixtures/cwlogs.go` | Log groups `/aws/redshift/cluster/acme-warehouse/connectionlog`, `/userlog`, `/useractivitylog` |

Adversarial fixtures (nil-pointer Cluster, malformed Tags) stay inline in test files per the 6a rule — the demo never carries them.

### §2.2 Typed-fake DescribeLoggingStatus / DescribeClusterSubnetGroups responses

The Redshift typed fake under `internal/demo/fakes/` (or equivalent handlers in `internal/demo/handlers.go`) must respond to:

- `DescribeLoggingStatus(ClusterIdentifier=acme-warehouse)` →
  `{LoggingEnabled: true, LogDestinationType: cloudwatch, LogExports: ["connectionlog","userlog","useractivitylog"]}`.
- `DescribeLoggingStatus(ClusterIdentifier=acme-reporting)` →
  `{LoggingEnabled: true, LogDestinationType: s3, BucketName: "acme-redshift-audit"}`.
- `DescribeLoggingStatus(ClusterIdentifier=*)` (all others) →
  `{LoggingEnabled: false}` (silence).
- `DescribeClusterSubnetGroups(ClusterSubnetGroupName=redshift-prod-subnet-group)` →
  one group with `Subnets = [{SubnetIdentifier:"subnet-prod-a"},{SubnetIdentifier:"subnet-prod-b"}]`.
- `DescribeClusterSubnetGroups(ClusterSubnetGroupName=redshift-staging-subnet-group)` →
  one group with `Subnets = [{SubnetIdentifier:"subnet-staging-a"},{SubnetIdentifier:"subnet-staging-b"}]`.

## §3. Contract-surface gap analysis

### §3.1 Fetcher (`internal/aws/redshift.go`)

- **Gap**: `Resource.Status` is set to raw `ClusterStatus` (`"available"`, `"resizing"`, `"storage-full"`, …). Spec §4 demands derived phrases (`""`, `"resizing"`, `"broken: storage-full"`, `"publicly accessible"`, etc.) with `(+N)` suffix on multi-warning.
- **Gap**: `Resource.Issues` never populated — rule-7 U7e/U7f regression class.
- **Gap**: No Wave-1 precedence logic (ClusterAvailabilityStatus overriding ClusterStatus for Unavailable/Failed; Broken suppressing Warnings).
- **Gap**: No active-window check for `DeferredMaintenanceWindows[]` (must test now ∈ [start, end]).
- **Gap**: No `PendingModifiedValues` non-empty detection.
- **Keep**: Raw fields (`publicly_accessible`, `encrypted`, `cluster_availability_status`) — Color function reads these.
- **Add**: Raw field `cluster_status` so Color function reads it directly (avoids parsing the derived phrase in `status`). The existing Color func in `internal/resource/types_databases.go` already reads these raw fields.
- **Update Color func** in `internal/resource/types_databases.go`: switch on `cluster_status` raw field instead of `status` (since `status` now carries the phrase). Strip `(+N)` suffix via `resource.StripFindingSuffix` is NOT needed since the raw `cluster_status` carries no suffix — but keep the pattern future-proof by reading both.

### §3.2 Interfaces (`internal/aws/redshift_interfaces.go`)

- **No gap.** All three narrow interfaces (`DescribeClusters`, `DescribeLoggingStatus`, `DescribeClusterSubnetGroups`) plus aggregate `RedshiftAPI` already present. No addition needed.

### §3.3 Related (`internal/aws/redshift_related.go`)

- **Matches spec §2**: alarm, cfn, kms, logs, role, s3, secrets, sg, subnet, vpc all registered. ct-events universal.
- **Gap — logs checker**: currently returns single path `/aws/redshift/cluster/<ID>`. Spec §2 demands one per enabled `LogExports[]` entry (`/connectionlog`, `/userlog`, `/useractivitylog`). Fix the checker to read `LogExports` and emit one ID per export (when `LogDestinationType==cloudwatch`).
- **NavigableFields**: only `VpcId → vpc` registered. Spec §2 exposes other navigable fields (KmsKeyId, MasterPasswordSecretArn, ClusterSubnetGroupName), but these route through the pivot checker rather than direct NavigableField registration — navigable-field route is reserved for single-value, structurally-valid RawStruct paths. VpcId is the only one that fits. No change needed.

### §3.4 Issue enrichment (`internal/aws/redshift_issue_enrichment.go`)

- Wave 2 = None. File exists as NoOp registration (170 B) — this is the codebase convention for Wave-2-None resources (see `rds_snap_issue_enrichment.go`, `secrets_issue_enrichment.go`, many others). Keep as-is. NOT in phase 7 / phase 7.5 scope.

### §3.5 Detail enrichment

- No `redshift_detail_enrichment.go` exists. Spec §3.2 has no Wave 2 signals → no detail enrichment needed. NOT in scope.

### §3.6 View config (`internal/config/defaults_databases.go` + `.a9s/views/redshift.yaml`)

- Current columns: Cluster ID, Status, Pending (NodeType), Node Type, Nodes, Database, Endpoint.
- **Pending column** (Path: `PendingModifiedValues.NodeType`) is an identity/metadata column, not jargon. It shows AWS data. Keep it.
- **Status column** backing key = `status`. Keep — fetcher will now write the spec §4 phrase here.
- **No jargon columns.** No `CIS` / `Flags` / `Policy` / `Issues` columns.
- **No change** to `internal/config/defaults_databases.go` column list. Regenerate `.a9s/views/redshift.yaml` via `go run ./cmd/viewsgen/` if any change happens (no change expected).

### §3.7 Color function (`internal/resource/types_databases.go`)

- **Gap**: Current Color func reads `r.Fields["status"]` and matches on raw states (`"available"`, `"storage-full"`). After the fetcher change, `status` will carry the phrase (`""`, `"broken: storage-full"`, `"publicly accessible"`, `"pending change queued (+2)"`).
- **Fix**: Switch the primary match to `r.Fields["cluster_status"]` (new raw field added by fetcher) — keeps logic crisp. Continue reading `cluster_availability_status`, `publicly_accessible`, `encrypted` raw fields as today.
- Alternative acceptable: keep reading `status`, strip suffix via `resource.StripFindingSuffix`, and match phrase prefix. But phrase-match is brittle; raw-field-match is the pattern in `rds`/`dbi` and is preferred.

## §4. Summary of phase-7 coder file list

| File | Action |
|------|--------|
| `internal/aws/redshift.go` | Rewrite fetcher: compute S4 phrase + Issues slice with precedence + `(+N)` suffix. Add raw `cluster_status` field to Fields. |
| `internal/aws/redshift_interfaces.go` | No change |
| `internal/aws/redshift_related.go` | Fix `checkRedshiftLogs` to emit one ID per enabled `LogExports[]` entry when `LogDestinationType==cloudwatch`. All other checkers unchanged. |
| `internal/aws/redshift_issue_enrichment.go` | No change (NoOp — Wave 2 = None). **NOT in approved scope.** |
| `internal/aws/redshift_detail_enrichment.go` | Not created (Wave 2 = None). **NOT in approved scope.** |
| `internal/resource/types_databases.go` | Update Redshift `Color` func: switch on `cluster_status` raw field; keep availability / publicly / encrypted upgrades. |
| `internal/config/defaults_databases.go` | No change expected (columns already spec-compliant) |
| `.a9s/views/redshift.yaml` | Regenerate from defaults.go only if columns change (no change expected) |
| `internal/demo/fixtures/redshift.go` | **Phase 6a only** — rebuild per §2 fixtures |
| `internal/demo/fixtures/{cloudwatch,cfn,kms,secrets,ec2,iam,s3,cloudtrail,cwlogs}.go` | **Phase 6a only** — sibling additions per §2.1 |

## §5. Known exemptions for phase 9 report

### §5.1 logs / s3 mutual exclusivity

Redshift audit logging destinations are mutually exclusive per the AWS API contract (`LogDestinationType` is one of `s3`, `cloudwatch`). A single cluster cannot simultaneously populate both the `logs` pivot (CloudWatch) AND the `s3` pivot (S3 bucket). Phase 9.3 therefore reports across TWO graph-root fixtures:

- `acme-warehouse` — CloudWatch logging variant — covers 10/11 pivots (s3=0 by design).
- `acme-reporting` — S3 logging variant — covers 10/11 pivots (logs=0 by design).

Union covers 11/11 `count shown: yes` pivots ≥ 1. The `drillThroughFixtures` table in `tests/integration/scenario_related_drill_through_test.go` has two entries for `redshift`.

### §5.2 `~` / `!` glyphs unreachable

Wave 2 = None for redshift. Universal rules U3, U4, U7b, U7c, U7d are structurally unreachable. Phase 9 report marks them `N/A (Wave 2 = None)`.

## §6. Citations

- This plan derives from `docs/resources/redshift.md` (the golden doc).
- Rule 7 semantics and `(+N)` suffix construction: `docs/resources/dbi.md` §4 + `internal/aws/rds.go:computeDBIStatusAndIssues`.
- Sibling fixture pattern: `a9s-create-demo-fixture` skill phase 2 (graph-plan).
- logs/s3 exclusivity: AWS `redshift:DescribeLoggingStatus` API doc — `LogDestinationType` is a single-value enum.
