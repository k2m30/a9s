---
shortName: redis
sourceSpec: docs/resources/redis.md
---

# redis — Implementation Plan

Derived from `docs/resources/redis.md`. Working doc for QA + coder handoff.

## 0b. Review-driven amendments (2026-04-23) — bug fixes + shard signals

After phase 9 landed, review surfaced three bugs (P2-1, P2-2, P3) and the user asked to expand Wave 1 with shard-level visibility (moving CloudTrail-failover to Wave 3 per §3.3 of the spec). This section lists the deltas from the first cycle; §§ 1-6 below have been amended accordingly.

### 0b.1 Bug — Engine filter missing in fetcher

Symptom: Valkey / Memcached replication groups (shared with Redis through `DescribeReplicationGroups`) leak into the redis list, get classified as ElastiCache Redis, and flow through Redis-specific detail/related logic.

Fix: in `FetchRedisPage`, skip any RG whose `Engine` is not `"redis"` (case-insensitive). The old `DescribeCacheClusters` path filtered on Engine; parity requires the same filter. Add before the rgID extraction:

```go
if rg.Engine == nil || !strings.EqualFold(aws.ToString(rg.Engine), "redis") {
    continue
}
```

Required unit test: `TestRedis_Fetch_SkipsNonRedisEngines` — a fixture with one `"redis"` RG and one `"valkey"` RG → fetcher returns only the redis one.

### 0b.2 Bug — ct-events checker overmatches

Symptoms:
- `strings.Contains(*r.ResourceName, rgID)` matches `prod-redis-sessions` when `rgID=prod-redis` (substring overmatch).
- `EventSource == "elasticache.amazonaws.com"` fallback matches ElastiCache events for every RG on the account — a `ModifyReplicationGroup` event on a different RG shows up on this row's related panel.

Fixes (both in `checkRedisCtEvents`):
- Change the `ResourceName` match from `strings.Contains` to exact equality: `if r.ResourceName != nil && *r.ResourceName == rgID { matched = true; break }`.
- DELETE the EventSource fallback block (line 205-207 in the current file). If no `Resources[]` entry on the event names this RG, it is not related to this row.
- Additionally accept RG-ARN equality: if `r.ResourceName == rg.ARN` or `r.ARN == rg.ARN` the event is related. The existing code only checks ResourceName.

Required unit tests:
- `TestRelated_Redis_CtEvents_ExactIDMatch` — event `Resources[0].ResourceName == rgID` → Count = 1.
- `TestRelated_Redis_CtEvents_ARNMatch` — event `Resources[0].ResourceName == rg.ARN` → Count = 1.
- `TestRelated_Redis_CtEvents_SubstringDoesNotOvermatch` — two RGs named `prod-redis` and `prod-redis-sessions`. Event naming `prod-redis-sessions` → Count is 0 on the `prod-redis` row, 1 on the `prod-redis-sessions` row.
- `TestRelated_Redis_CtEvents_ElastiCacheSourceDoesNotOvermatch` — event with EventSource=elasticache.amazonaws.com but Resources[] naming a different RG → Count = 0 on this row.

### 0b.3 Bug — NeedsTargetCache on field-only checkers

Symptom: opening a cold redis detail view makes unnecessary KMS and VPC list calls, burning two of the four probe slots without changing the result.

Fix: in `redis_related.go` `RegisterRelated(...)`, set `NeedsTargetCache: false` for the `kms` and `vpc` entries. Reason: `checkRedisKMS` returns `[KmsKeyId]` extracted from RawStruct with no cache lookup; `checkRedisVPC` returns `[sng.VpcId]` extracted from the subnet-group chain with no cache lookup. Neither uses the `cache` parameter.

Required tests: the existing `TestRelated_Redis_KMS` and `TestRelated_Redis_VPC` stay green (they construct a cache but don't depend on prefetch behavior).

### 0b.4 Spec expansion — shard-level Wave 1 signals

See updated spec §3.1 and §4. Summary:

- New signal: `any NodeGroup.Status != "available"` on multi-shard RGs → Warning, phrase `shard <ng-id>: <state>`. One distinct §4 phrase per transitioning shard. Rule 7 `(+N-1)` suffix applies across multiple shards.
- Single-shard RGs preserve the existing `modifying — config change` / `snapshotting — backup running` phrases.
- Detail view Attention section includes per-node AZ + role rows for every non-available NodeGroup.

New fetcher logic in `FetchRedisPage` (replaces the current `modifying`/`snapshotting` case in `computeRedisIssues`):

```go
// After checking create-failed, creating, deleting:
if state == "modifying" || state == "snapshotting" {
    if len(rg.NodeGroups) <= 1 {
        // Single-shard: preserve existing phrase.
        issues = append(issues, existingRGPhrase(state))
    } else {
        // Multi-shard: emit one phrase per transitioning shard.
        // (Note: it's possible the RG.Status is "modifying" while every NodeGroup
        // is "available" — a transient state. Fall back to the RG-level phrase
        // in that case.)
        anyShard := false
        for _, ng := range rg.NodeGroups {
            ngStatus := strings.ToLower(aws.ToString(ng.Status))
            if ngStatus != "" && ngStatus != "available" {
                phrase := fmt.Sprintf("shard %s: %s",
                    aws.ToString(ng.NodeGroupId), ngStatus)
                issues = append(issues, phrase)
                anyShard = true
            }
        }
        if !anyShard {
            issues = append(issues, existingRGPhrase(state))
        }
    }
}
```

Required unit tests:
- `TestRedis_Fetch_SingleShardModifying_UsesRGPhrase` — 1 NodeGroup, Status=modifying → phrase `modifying — config change`.
- `TestRedis_Fetch_MultiShard_OneShardModifying` — 3 NodeGroups, `0001` modifying, `0002` + `0003` available → phrase `shard 0001: modifying`, `Resource.Issues == ["shard 0001: modifying"]`.
- `TestRedis_Fetch_MultiShard_TwoShardsModifying` — 3 NodeGroups, `0001` modifying, `0002` snapshotting, `0003` available → phrase `shard 0001: modifying (+1)`, `Resource.Issues == ["shard 0001: modifying", "shard 0002: snapshotting"]` (alphabetical by phrase).
- `TestRedis_Fetch_MultiShard_AllAvailableButRGModifying` — 3 NodeGroups, all available, `RG.Status == modifying` → fallback phrase `modifying — config change`, `Resource.Issues == ["modifying — config change"]`.

### 0b.5 Detail-view AZ / role visibility

Render per-non-available-NodeGroup block in detail. Example:

```
Attention (2)
  ~ Shard 0001: modifying
    Primary AZ: us-east-1a
    Replicas:   us-east-1b, us-east-1c
  ~ Shard 0002: snapshotting
    Primary AZ: us-east-1b
    Replicas:   us-east-1a
```

Implementation: populate `Resource.EnrichmentFinding` (or use `Resource.Issues`-backed detail rendering — check which path the unified `injectAttentionSection` already supports). The `Rows` slice for each finding carries `Primary AZ: <az>` and `Replicas: <comma-joined AZs>`. No Wave-2 enricher needed — the data comes from the fetcher.

**Caveat**: spec §3.1 notes `CurrentRole` may be nil for cluster-mode-enabled Redis. Fetcher code must tolerate nil `CurrentRole` by falling back to endpoint match (NodeGroupMember.CacheClusterId == NodeGroup.PrimaryEndpoint owner) or by dropping the role label when ambiguous. Do NOT emit "primary=unknown" strings.

### 0b.6 Fixture updates

Required new fixtures in `internal/demo/fixtures/redis.go`:
- `multi-shard-healthy`: ClusterEnabled=true, 3 NodeGroups all available, for graph-connectivity coverage. Each NodeGroup has 1 primary + 1 replica in different AZs.
- `multi-shard-one-modifying`: 3 NodeGroups, `0001` status=modifying, others available. Asserts `shard 0001: modifying` phrase + U7f `Resource.Issues` ordering.
- `multi-shard-two-transitioning`: 3 NodeGroups, `0001` modifying + `0002` snapshotting + `0003` available. Asserts `shard 0001: modifying (+1)` + multi-shard rule 7.
- Existing `valkey` fixture (new): one `Engine=valkey` RG included in `ReplicationGroups` slice. Phase-8 render gate asserts it does NOT appear in the redis list. Serves as regression pin for P2-1.

### 0b.7 Scope-diff expectations for this round

Changed files (per phase 7.5 gate):
- `internal/aws/redis.go` — fetcher changes (engine filter, shard signals, per-node detail population).
- `internal/aws/redis_related.go` — ct-events tightening; NeedsTargetCache flags.
- `internal/demo/fixtures/redis.go` — new fixtures.
- `internal/config/defaults_databases.go` — unchanged (same columns).
- `internal/resource/types_databases.go` — Color function MAY need to accept new shard phrases (`shard <id>: modifying` etc). Add a prefix match rather than exhaustive enumeration.
- `.a9s/views/redis.yaml` — regenerate if defaults change (no change expected).
- `tests/unit/aws_redis_test.go`, `aws_redis_related_test.go` — new unit tests per §0b.1/0b.2/0b.4.
- `tests/integration/scenario_redis_visual_test.go` — new scenario assertions for shard phrases + AZ visibility.
- `docs/resources/redis.md`, `docs/resources/redis-impl-plan.md` — spec + plan updates.

No new files should appear outside this list. Wave-3 CT-failover is documented as out-of-scope, NOT added as code.

---

## 0. Summary of deltas found in phase 5

Spec says one thing; the current code does another. Listed here so the coder knows every delta to close.

### 0.1 Fetcher semantics (BIGGEST DELTA)

- **Spec §1**: List API = `DescribeReplicationGroups`; the a9s row = one `elasticachetypes.ReplicationGroup`.
- **Current code**: `redis_related.go` casts `res.RawStruct` to `elasticachetypes.CacheCluster` (search: `assertStruct[elasticachetypes.CacheCluster]`). The fetcher lists cache clusters, not replication groups.
- **Required change**: coder rewrites `internal/aws/redis.go` to `RegisterPaginated("redis", ...)` using `DescribeReplicationGroups`. `Resource.RawStruct` becomes `elasticachetypes.ReplicationGroup`. Every related checker is rewritten to read from the RG or resolve via one `DescribeCacheClusters(MemberClusters[0])` call for SG / SNS / subnet-group.

### 0.2 `ct-events` pivot missing from registration

- **Spec §2**: ct-events is one of the 10 targets.
- **Current code**: `redis_related.go` `init()` registers 9 pivots — alarm, sg, cfn, kms, logs, secrets, sns, subnet, vpc — and NO ct-events.
- **Required change**: add `{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkRedisCtEvents, NeedsTargetCache: true}` to the `RegisterRelated` call. Implement the checker (reverse-lookup on the loaded `ct-events` cache by `ResourceName == ReplicationGroupId` or `EventSource == elasticache.amazonaws.com`). Pattern already used in `iam_users_related.go:60-95` and `ecr_related_extra.go`.

### 0.3 `redis_issue_enrichment.go` must be deleted

- **Spec §3.2**: `No Wave 2 signals`.
- **Current code**: `redis_issue_enrichment.go` exists and registers a Wave 2 enricher that re-fetches `DescribeReplicationGroups` to back-fill `automatic_failover` / `multi_az` onto `Fields` of cache-cluster rows. This is a workaround for the fetcher listing the wrong thing (see 0.1). Once the fetcher lists replication groups directly, those fields come from the list response — no Wave 2 needed.
- **Required change**: delete the file in its entirety. Delete the corresponding test file. The color function in `internal/resource/types_databases.go:137-163` already reads `automatic_failover` / `multi_az` from `Fields` — those fields are now populated by the fetcher instead of the Wave 2 enricher. (The color function itself is kept; only its input source changes from enricher to fetcher.)

### 0.4 Status column carries bare keywords, not §4 phrases

- **Spec §4**: Status column for a `modifying` group reads `"modifying — config change"`, not `"modifying"`. Healthy rows render blank.
- **Current code**:
  - `.a9s/views/redis.yaml` column `Status` uses `path: CacheClusterStatus` (raw AWS field). After the migration the path doesn't exist on `ReplicationGroup` (field is `Status`, not `CacheClusterStatus`).
  - `internal/resource/types_databases.go:139-148` matches `r.Fields["status"]` against bare keywords (`"available"`, `"creating"`, `"modifying"`, `"snapshotting"`, `"deleting"`, `"create-failed"`). That stays keyword-based — but the DISPLAYED `status` field must be the §4 phrase, and the color function must strip the `(+N)` suffix and match on the phrase prefix (or on an internal `status_bucket` field). See §4 "Color function wiring" below.
- **Required change**: fetcher populates `Fields["status"]` with the §4 phrase (Healthy → empty; each §3.1 signal → its §4 "List text" column). Color function in `types_databases.go` rewritten to strip `(+N)` via `resource.StripFindingSuffix`, then match on the resulting phrase (or its prefix) to derive the color bucket. The `automatic_failover != enabled on multi-AZ` signal is now encoded in `Fields["status"]` as `"multi-AZ without auto-failover"` — the color function no longer needs to read `automatic_failover` / `multi_az` separately, and those fields can be dropped from the resource's `Fields` map.

### 0.5 View YAML has a jargon-flavoured "Failover" column

- **Spec §4**: there is exactly ONE Status column; no parallel "Failover" / "Flags" / "Policy" column.
- **Current code**: `.a9s/views/redis.yaml` lines 14-16 declare a `Failover` column backed by `automatic_failover`.
- **Required change**: delete the `Failover` column from `.a9s/views/redis.yaml` — it's a parallel-to-Status column invented to surface the `multi-AZ without auto-failover` signal before the Status column could carry it. Post-migration, the signal lands in Status per §4. Also regenerate the yaml from `defaults.go` via `go run ./cmd/viewsgen/` so the yaml and defaults match (the yaml currently lists 7 columns; defaults.go lists 6 — the yaml is stale).

### 0.6 `checkRedisSecrets` returns hard-coded 0

- **Spec §2 secrets**: discovery via tag (`elasticache:replication-group-id=<id>`) or naming (`<id>/auth-token`). Best-effort; may be zero when no convention used.
- **Current code**: `redis_related.go:219-229` returns `Count: 0` unconditionally after fetching the RG.
- **Required change**: implement tag-based + name-based cross-reference on the loaded `secrets` cache. Match secret whose `Tags[elasticache:replication-group-id] == <rgID>` OR whose `Name == <rgID>/auth-token`. Returns count of matches (0 if none — that's allowed for non-root fixtures; the graph-root fixture must have a matching secret so U9 passes).

### 0.7 `internal/demo/fixtures/elasticache.go` is the old single-file store

- **Current**: fixtures live at `internal/demo/fixtures/elasticache.go` keyed on `ElastiCacheFixtures.CacheClusters`. The fake `internal/demo/fakes/elasticache.go:30-32` returns an empty list for `DescribeReplicationGroups`.
- **Required**: phase 6a folds this file into `internal/demo/fixtures/redis.go` with the new `RedisFixtures` struct (per the `a9s-create-demo-fixture` skill). The struct exposes both `ReplicationGroups []elasticachetypes.ReplicationGroup` AND `CacheClusters []elasticachetypes.CacheCluster` (related checkers still need one `DescribeCacheClusters` call per RG to reach SG/SNS/subnet-group fields). `counts.go:46` and `fakes/elasticache.go:15-41` are updated to the new symbol. The old file is deleted.

### 0.8 Navigable fields

- **Current `RegisterNavigableFields`**: `SecurityGroups.SecurityGroupId → sg`, `KmsKeyId → kms`. First reads from the member CacheCluster (won't work on a ReplicationGroup raw struct).
- **Required change**: drop `SecurityGroups.SecurityGroupId` (not on ReplicationGroup). Keep `KmsKeyId → kms` (is on ReplicationGroup). Optionally add `MemberClusters → cluster` if a cluster-member child view exists (out of scope here if not registered).

## 1. Pseudocode test spec

One test case per §3 / §4 signal, plus universal-rule tests per the coverage matrix (§3 below). Lowercase phrases — the renderer applies `capitalizeFirst` to Attention entries at display time.

### Fetcher-level (Wave 1) tests — `aws_redis_test.go`

```text
TEST: redis_fetch_healthy_available
GIVEN: a ReplicationGroup with Status=available, AutomaticFailover=enabled, MultiAZ=enabled
WHEN:  FetchRedisPage runs on the fixture
THEN:
  - Resource.ID == ReplicationGroupId
  - Resource.Name == ReplicationGroupId (or Description if present)
  - Resource.Fields["status"] == ""                 (Healthy silence per §4)
  - Resource.Issues is empty/nil
  - Resource.RawStruct is the ReplicationGroup

TEST: redis_fetch_status_creating
GIVEN: a ReplicationGroup with Status=creating
WHEN:  FetchRedisPage runs
THEN:
  - Resource.Fields["status"] == "creating — new group"
  - Resource.Issues == ["creating — new group"]

TEST: redis_fetch_status_modifying
GIVEN: Status=modifying
WHEN:  FetchRedisPage runs
THEN:
  - Resource.Fields["status"] == "modifying — config change"
  - Resource.Issues == ["modifying — config change"]

TEST: redis_fetch_status_snapshotting
GIVEN: Status=snapshotting
WHEN:  FetchRedisPage runs
THEN:
  - Resource.Fields["status"] == "snapshotting — backup running"
  - Resource.Issues == ["snapshotting — backup running"]

TEST: redis_fetch_status_deleting
GIVEN: Status=deleting
WHEN:  FetchRedisPage runs
THEN:
  - Resource.Fields["status"] == "deleting — teardown"
  - Resource.Issues == ["deleting — teardown"]

TEST: redis_fetch_status_create_failed
GIVEN: Status=create-failed
WHEN:  FetchRedisPage runs
THEN:
  - Resource.Fields["status"] == "create failed — see events"
  - Resource.Issues == ["create failed — see events"]

TEST: redis_fetch_multiaz_no_auto_failover
GIVEN: Status=available, MultiAZ=enabled, AutomaticFailover=disabled
WHEN:  FetchRedisPage runs
THEN:
  - Resource.Fields["status"] == "multi-AZ without auto-failover"
  - Resource.Issues == ["multi-AZ without auto-failover"]

TEST: redis_fetch_multiaz_disabled_no_finding
GIVEN: Status=available, MultiAZ=disabled, AutomaticFailover=disabled
WHEN:  FetchRedisPage runs
THEN:
  - Resource.Fields["status"] == ""                 (single-AZ groups don't trigger the signal; §4 note)
  - Resource.Issues is empty

TEST: redis_fetch_multi_w1_modifying_plus_no_failover   (covers U7a)
GIVEN: Status=modifying, MultiAZ=enabled, AutomaticFailover=disabled
WHEN:  FetchRedisPage runs
THEN:
  - Resource.Fields["status"] == "modifying — config change (+1)"
  - Resource.Issues == ["modifying — config change", "multi-AZ without auto-failover"]
     (top phrase first, hidden phrases in §4 precedence order)

TEST: redis_fetch_populates_columns
GIVEN: any ReplicationGroup with ConfigurationEndpoint and CacheNodeType set
WHEN:  FetchRedisPage runs
THEN:
  - Resource.Fields["cluster_id"] == ReplicationGroupId
  - Resource.Fields["node_type"] == CacheNodeType
  - Resource.Fields["nodes"] == "<count of MemberClusters>"
  - Resource.Fields["endpoint"] contains ConfigurationEndpoint.Address

ANTI-TESTS (§3.3 Wave 3 out of scope — no surface rendered)

TEST: redis_wave3_memory_pressure_no_surface
GIVEN: a ReplicationGroup with status=available (no metric call simulated)
WHEN:  FetchRedisPage runs
THEN:  Resource.Fields["status"] == ""     (CloudWatch DatabaseMemoryUsagePercentage is OUT OF SCOPE;
                                            the fetcher never asks, so no finding is produced — this
                                            test merely pins that no field is invented)

TEST: redis_wave3_replication_lag_no_surface
GIVEN: a ReplicationGroup with status=available
WHEN:  FetchRedisPage runs
THEN:  same as above — no ReplicationLag field produced

(Similar stubs for Evictions and EngineCPUUtilization.)
```

### Related-panel tests — `aws_redis_related_test.go`

One test per §2 pivot. All against the graph-root fixture (prod-redis-sessions) which carries a matching sibling entry for every target.

```text
TEST: redis_related_alarm
GIVEN: graph-root RG with MemberClusters=[prod-redis-sessions-001, 002, 003];
       alarm cache contains MetricAlarm with Dimensions.CacheClusterId = prod-redis-sessions-001
WHEN:  checkRedisAlarms runs
THEN:  returns {TargetType: "alarm", Count: >=1}

TEST: redis_related_cfn
GIVEN: graph-root RG, ListTagsForResource returns aws:cloudformation:stack-name = acme-prod-redis;
       cfn cache has a Stack named acme-prod-redis
WHEN:  checkRedisCFN runs
THEN:  returns {TargetType: "cfn", Count: 1}

TEST: redis_related_ct_events                    (new — fills the 0.2 delta)
GIVEN: graph-root RG (ReplicationGroupId=prod-redis-sessions);
       ct-events cache has at least one event with ResourceName matching that id
WHEN:  checkRedisCtEvents runs
THEN:  returns {TargetType: "ct-events", Count: >=1}

TEST: redis_related_kms
GIVEN: graph-root RG with KmsKeyId set; kms cache has a matching key
WHEN:  checkRedisKMS runs
THEN:  returns {TargetType: "kms", Count: 1}

TEST: redis_related_logs
GIVEN: graph-root RG with LogDeliveryConfigurations → CloudWatchLogsDetails.LogGroup = /aws/elasticache/redis/prod;
       logs cache contains a LogGroup of that name
WHEN:  checkRedisLogs runs
THEN:  returns {TargetType: "logs", Count: 1}

TEST: redis_related_secrets_tag_match             (tightens 0.6 delta)
GIVEN: graph-root RG (ReplicationGroupId=prod-redis-sessions);
       secrets cache contains a secret with Tag[elasticache:replication-group-id]=prod-redis-sessions
WHEN:  checkRedisSecrets runs
THEN:  returns {TargetType: "secrets", Count: 1}

TEST: redis_related_secrets_name_match
GIVEN: RG, secrets cache contains a secret named prod-redis-sessions/auth-token
WHEN:  checkRedisSecrets runs
THEN:  returns {TargetType: "secrets", Count: 1}

TEST: redis_related_secrets_no_match
GIVEN: RG, no tag/name convention in the secrets cache
WHEN:  checkRedisSecrets runs
THEN:  returns {TargetType: "secrets", Count: 0}     (best-effort zero — allowed)

TEST: redis_related_sg
GIVEN: RG with MemberClusters=[prod-redis-sessions-001];
       DescribeCacheClusters on the member returns SecurityGroups=[sg-abc];
       sg cache contains sg-abc
WHEN:  checkRedisSG runs
THEN:  returns {TargetType: "sg", Count: 1}

TEST: redis_related_sns
GIVEN: RG with MemberClusters=[prod-redis-sessions-001];
       DescribeCacheClusters returns NotificationConfiguration.TopicArn = arn:aws:sns:...:ops-pager;
       sns cache contains topic name ops-pager
WHEN:  checkRedisSNS runs
THEN:  returns {TargetType: "sns", Count: 1}

TEST: redis_related_subnet
GIVEN: RG with MemberClusters=[prod-redis-sessions-001];
       DescribeCacheClusters returns CacheSubnetGroupName = prod-subnet-grp;
       DescribeCacheSubnetGroups(prod-subnet-grp) returns Subnets=[subnet-a, subnet-b];
       subnet cache contains both
WHEN:  checkRedisSubnet runs
THEN:  returns {TargetType: "subnet", Count: 2}

TEST: redis_related_vpc
GIVEN: same chain as subnet; CacheSubnetGroup.VpcId = vpc-prod;
       vpc cache contains vpc-prod
WHEN:  checkRedisVPC runs
THEN:  returns {TargetType: "vpc", Count: 1}
```

## 2. Fixture list (plain language)

One canonical `RedisFixtures` file at `internal/demo/fixtures/redis.go` that the `a9s-create-demo-fixture` skill produces. The exported struct holds BOTH raw `ReplicationGroups` (for the list API) AND raw `CacheClusters` (for related checkers' N+1 `DescribeCacheClusters` call). Plus a `SubnetGroups` slice for the subnet/vpc chain, and a `Tags` map keyed on ARN (for `ListTagsForResource` → cfn pivot).

### FIXTURE: prod-redis-sessions (GRAPH ROOT — every §2 pivot resolves non-zero here)

- ReplicationGroupId: `prod-redis-sessions`
- Description: `Prod sessions Redis`
- Status: `available`
- AutomaticFailover: `enabled`
- MultiAZ: `enabled`
- MemberClusters: `[prod-redis-sessions-001, prod-redis-sessions-002, prod-redis-sessions-003]`
- ConfigurationEndpoint.Address: `prod-redis-sessions.cfg.use1.cache.amazonaws.com`, Port 6379
- CacheNodeType: `cache.r6g.large`
- KmsKeyId: `arn:aws:kms:us-east-1:123456789012:key/11111111-1111-1111-1111-111111111111`
- LogDeliveryConfigurations: one entry with DestinationType=cloudwatch-logs, LogGroup=`/aws/elasticache/redis/prod-redis-sessions/slow-log`
- ARN: `arn:aws:elasticache:us-east-1:123456789012:replicationgroup:prod-redis-sessions`
- Tags (via ListTagsForResource on the ARN above): `aws:cloudformation:stack-name = acme-prod-redis`

**Member cluster for related discovery** — `prod-redis-sessions-001`:
- CacheClusterId: `prod-redis-sessions-001`
- ReplicationGroupId: `prod-redis-sessions`
- SecurityGroups: `[sg-redis-prod-a]` (exists in `sg.go` fixture)
- NotificationConfiguration.TopicArn: `arn:aws:sns:us-east-1:123456789012:redis-ops-pager` (matches `ops-pager` topic in `sns.go` — or add)
- CacheSubnetGroupName: `prod-redis-subnet-group`

**Subnet group** — `prod-redis-subnet-group`:
- Subnets: `[{SubnetIdentifier: subnet-prod-a}, {SubnetIdentifier: subnet-prod-b}]`
- VpcId: `vpc-prod-main`

**Sibling fixture entries the graph-plan must add or confirm**:
- `alarm.go`: MetricAlarm with `Dimensions.CacheClusterId = prod-redis-sessions-001` (add if not present).
- `cfn.go`: Stack named `acme-prod-redis` (add).
- `ct-events.go`: CloudTrail event with `ResourceName = prod-redis-sessions` (or `EventSource = elasticache.amazonaws.com`).
- `kms.go`: Key ID `11111111-1111-1111-1111-111111111111` (confirm or add).
- `logs.go`: LogGroup `/aws/elasticache/redis/prod-redis-sessions/slow-log` (add).
- `secrets.go`: Secret named `prod-redis-sessions/auth-token` OR tagged `elasticache:replication-group-id=prod-redis-sessions` (add).
- `sg.go`: SG `sg-redis-prod-a` (confirm or add).
- `sns.go`: Topic `redis-ops-pager` (add).
- `subnet.go`: Subnets `subnet-prod-a`, `subnet-prod-b` (confirm or add).
- `vpc.go`: VPC `vpc-prod-main` (confirm or add).

### FIXTURE: staging-redis-healthy (Healthy, single-AZ)

- ReplicationGroupId: `staging-redis`
- Status: `available`
- MultiAZ: `disabled`, AutomaticFailover: `disabled`
- MemberClusters: `[staging-redis-001]`
- **Expected S4**: blank. Single-AZ groups do not produce the `multi-AZ without auto-failover` finding (§4 note).

### FIXTURE: redis-creating (Warning)

- ReplicationGroupId: `dev-feature-redis`, Status: `creating`, MultiAZ: disabled, MemberClusters: `[]` (still being created)
- **Expected S4**: `creating — new group`

### FIXTURE: redis-modifying (Warning)

- ReplicationGroupId: `prod-redis-cache`, Status: `modifying`, MultiAZ: `enabled`, AutomaticFailover: `enabled`
- **Expected S4**: `modifying — config change`

### FIXTURE: redis-snapshotting (Warning)

- ReplicationGroupId: `prod-redis-analytics`, Status: `snapshotting`, MultiAZ: `enabled`, AutomaticFailover: `enabled`
- **Expected S4**: `snapshotting — backup running`

### FIXTURE: redis-deleting (Warning)

- ReplicationGroupId: `old-redis-unused`, Status: `deleting`, MultiAZ: `disabled`
- **Expected S4**: `deleting — teardown`

### FIXTURE: redis-create-failed (Broken)

- ReplicationGroupId: `bad-config-redis`, Status: `create-failed`, MemberClusters: `[]`
- **Expected S4**: `create failed — see events`

### FIXTURE: redis-multiaz-no-failover (Warning, single signal)

- ReplicationGroupId: `legacy-redis-analytics`, Status: `available`, MultiAZ: `enabled`, AutomaticFailover: `disabled`
- **Expected S4**: `multi-AZ without auto-failover`

### FIXTURE: warn-redis-multi (Mandatory multi-W1 per coverage row U7a)

- ReplicationGroupId: `legacy-redis-billing`, Status: `modifying`, MultiAZ: `enabled`, AutomaticFailover: `disabled`
- Two coexisting §3.1 Warnings: `modifying — config change` AND `multi-AZ without auto-failover`.
- **Expected S4**: `modifying — config change (+1)`. Top phrase chosen by alphabetical ordering (§6 below); hidden count = 1.
- **Expected Resource.Issues**: `["modifying — config change", "multi-AZ without auto-failover"]` in that order.

**Skipped fixtures with justification** (universal coverage matrix, column ID on the right):

- U7b (Wave-1 + Wave-2 stacking): SKIPPED — spec §3.2 = "No Wave 2 signals" → no Wave-2 enricher exists, suffix-bump path has no source of bumps. Confirmed with user in phase 2 (no TBDs).
- U7c (S5 all Wave-2 findings visible): SKIPPED — same reason; no Wave-2 findings to render.
- U7d (! beats ~): SKIPPED — spec has no `!` Wave-2 signals and no `~` Wave-2 signals.
- U11 (Summary ≠ Rows): SKIPPED — no Wave-2 enricher emits `EnrichmentFinding`.

## 3. Contract surface gap analysis

Summarized from phase 5. Per-file delta:

### 3.1 `internal/aws/redis_interfaces.go`

- Current: aggregate `ElastiCacheAPI` already embeds 4 narrow APIs covering `DescribeCacheClusters`, `DescribeReplicationGroups`, `DescribeCacheSubnetGroups`, `ListTagsForResource`. Sufficient for the new design.
- Delta: no change needed. Coder may add a comment that `DescribeReplicationGroups` is now the primary list API.

### 3.2 `internal/aws/redis_related.go`

- Current: 9 pivots, all casting `res.RawStruct` to `CacheCluster`. `ct-events` missing. `secrets` stubs to 0. `kms`/`logs`/`sg`/`sns`/`subnet`/`vpc` make an extra API call per row to reach fields that — post-migration — live on the parent RG or can be cheaply resolved via one `DescribeCacheClusters(MemberClusters[0])` call.
- Delta:
  - Add `{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkRedisCtEvents, NeedsTargetCache: true}`.
  - Rewrite `checkRedisSG` / `checkRedisSNS` / `checkRedisSubnet` / `checkRedisVPC` to cast `RawStruct` to `ReplicationGroup` and use `redisMemberCluster(ctx, clients, res)` helper that `DescribeCacheClusters(MemberClusters[0])` once and caches the result on the context (or returns it inline for all four checkers if they share a single call).
  - Rewrite `checkRedisKMS` to read `RG.KmsKeyId` from `RawStruct` directly (no extra API call).
  - Rewrite `checkRedisLogs` to read `RG.LogDeliveryConfigurations` from `RawStruct` directly.
  - Rewrite `checkRedisCFN` to call `ListTagsForResource(ResourceName=RG.ARN)` — the tag API takes an ARN and works on replication-group ARNs too.
  - Rewrite `checkRedisSecrets` to do the tag-based + name-based cross-reference on the loaded `secrets` cache per the spec's §2 discovery mechanism. Returns 0 when no tag/name match (this is allowed per spec; the graph-root fixture carries a matching secret so U9 passes on the showroom).
  - Update `RegisterNavigableFields`: drop `SecurityGroups.SecurityGroupId` (not on RG), keep `KmsKeyId → kms`.

### 3.3 `internal/aws/redis_issue_enrichment.go`

- Current: exists; registers a Wave 2 enricher that back-fills `automatic_failover` / `multi_az` fields.
- Delta: **DELETE** (and its test file). Those fields are now on the list response per the §3.1 signals. The logic moves into `FetchRedisPage` which emits `Fields["status"]` directly as the §4 phrase (including the `multi-AZ without auto-failover` case).

### 3.4 `internal/aws/redis_detail_enrichment.go`

- Current: does not exist.
- Delta: remains absent. Spec §2 does not demand detail-view enrichment beyond the list shape. The unified Attention section (`internal/tui/views/detail_fields.go:injectAttentionSection`) renders `Resource.Issues` without per-type code.

### 3.5 `internal/config/defaults.go` + `.a9s/views/redis.yaml`

- Current defaults.go columns: `cluster_id`, `engine_version`, `node_type`, `status`, `nodes`, `endpoint` — all identity-class except `status`. One Status column (correct per phase-5 audit). No jargon columns. Matches universal rules.
- Current yaml: has an extra `Failover` column backed by `automatic_failover` (stale).
- Delta:
  - Regenerate yaml via `go run ./cmd/viewsgen/` so it matches defaults.go and drops the stale `Failover` column.
  - Audit the `engine_version` column: `ReplicationGroup` does NOT carry an `EngineVersion` field in the SDK. Options:
    1. Drop the column.
    2. Leave it and have the fetcher fill `Fields["engine_version"]` by reading it from the first member cluster via the same `DescribeCacheClusters` call used by SG/SNS/subnet-group.
    3. Leave it blank.
  - Coder decides based on what's minimal; dropping the column is preferred (less API traffic, spec §1 doesn't mandate engine_version).

### 3.6 `internal/resource/types_databases.go` (color function)

- Current: matches bare keywords (`"available"`, `"creating"`, `"modifying"`, `"snapshotting"`, `"deleting"`, `"create-failed"`, `"rebooting cluster nodes"`, `"restore-failed"`, `"incompatible-network"`, `"deleted"`) against `r.Fields["status"]`, and separately reads `multi_az` / `automatic_failover` for the multi-AZ-no-failover bump.
- Delta: rewrite to mirror `dbc`'s pattern (types_databases.go:178-205). Strip `(+N)` suffix via `resource.StripFindingSuffix`, match on the §4 phrase (or its prefix), remove the `multi_az`/`automatic_failover` branch (that signal now lives in the Status phrase).

  ```go
  Color: func(r Resource) Color {
      phrase := StripFindingSuffix(r.Fields["status"])
      switch phrase {
      case "":
          return ColorHealthy
      case "create failed — see events":
          return ColorBroken
      case
          "creating — new group",
          "modifying — config change",
          "snapshotting — backup running",
          "deleting — teardown",
          "multi-AZ without auto-failover":
          return ColorWarning
      }
      return ColorHealthy
  }
  ```

### 3.7 `internal/demo/fixtures/elasticache.go` + `internal/demo/fakes/elasticache.go` + `internal/demo/fixtures/counts.go`

- Current: fixture struct is `ElastiCacheFixtures{CacheClusters}`. Fake returns empty for `DescribeReplicationGroups` / `DescribeCacheSubnetGroups` / `ListTagsForResource`. `counts.go:46` references `NewElastiCacheFixtures().CacheClusters`.
- Delta (driven by phase 6a `a9s-create-demo-fixture`):
  - Create `internal/demo/fixtures/redis.go` with `RedisFixtures` struct exposing `ReplicationGroups`, `CacheClusters`, `SubnetGroups`, `TagLists map[string][]Tag`. Constructor `NewRedisFixtures()`.
  - Delete `internal/demo/fixtures/elasticache.go`.
  - Update `internal/demo/fakes/elasticache.go` to back all four methods with the new `RedisFixtures` (DescribeReplicationGroups returns the new slice; DescribeCacheClusters filters by input CacheClusterId; DescribeCacheSubnetGroups looks up by name; ListTagsForResource looks up by ResourceName=ARN).
  - Update `counts.go:46` to `"redis": len(NewRedisFixtures().ReplicationGroups)`.

## 4. Coverage matrix (mandatory before phase 6)

| ID | Invariant | Required fixture | Required test | Status |
|----|-----------|-----------------|---------------|--------|
| U1 | Healthy blank S4 | prod-redis-sessions, staging-redis-healthy | scenario `ExpectRowStatusBlank` | planned |
| U2 | Warning/Broken §4 phrase | redis-creating / -modifying / -snapshotting / -deleting / -create-failed / -multiaz-no-failover | scenario `ExpectRowStatusEquals` per | planned |
| U3 | `~` glyph on Healthy+~ | — | — | N/A (no Wave-2 `~` signals) |
| U4 | `!` glyph on Healthy+! | — | — | N/A (no Wave-2 `!` signals) |
| U5 | No glyph on non-green rows | every warning/broken fixture | scenario `ExpectRowNoGlyphPrefix` | planned |
| U6 | S1 badge counts `!` instances | — | scenario `ExpectMenuIssueCount("redis", 0)` (no `!` signals) | planned |
| U7a | Multi-W1 `(+N-1)` suffix | warn-redis-multi | scenario `ExpectRowStatusEquals("legacy-redis-billing", "modifying — config change (+1)")` | planned |
| U7b | W1+W2 stack bumps suffix | — | — | N/A (no Wave-2) |
| U7c | S5 lists every Wave-2 finding | — | — | N/A |
| U7d | `!` beats `~` | — | — | N/A |
| U7e | S5 lists every Wave-1 phrase | warn-redis-multi | scenario `ExpectViewContains(capitalizeFirst(phrase))` per entry | planned |
| U7f | `Resource.Issues` in §4 order | every §3 fixture | unit: `got.Issues` deep-equals expected slice | planned |
| U8 | Broken > Warning > ~ | — | — | N/A (no Wave-2) |
| U9 | Related pivot counts (`count shown: yes`) | prod-redis-sessions | scenario `ExpectRelatedRowCountAtLeast` for each of the 10 pivots | planned |
| U10 | No jargon columns | all | scenario `ExpectViewNotContains("Failover", "Flags", "Policy", "CIS")` | planned |
| U11 | Summary ≠ Rows content | — | — | N/A (no Wave-2 enricher) |

## 5. §4 Precedence (tie-breaker) — declared

§4 does not pin an explicit Warning ordering. The fetcher uses **alphabetical by phrase** among same-severity finds, matching the skill's documented default. Resulting order for §3.1 Warnings (lowercase compare):

1. `creating — new group`
2. `deleting — teardown`
3. `modifying — config change`
4. `multi-AZ without auto-failover`   (lexicographic: `modi` < `mult`)
5. `snapshotting — backup running`

Broken (`create failed — see events`) beats all of the above regardless. `warn-redis-multi` demonstrates: two active Warnings → top = `modifying — config change`, hidden = `multi-AZ without auto-failover`, so Status = `modifying — config change (+1)`.

## 6. Wave 2 = None — confirmation

Confirmed in spec §3.2: `No Wave 2 signals`. Enricher file, enricher test file, and enricher-related columns are out of scope for this run. Phase 7 deletes `redis_issue_enrichment.go`. Phase 7.5's approved scope union does NOT include a Wave-2 enricher file.
