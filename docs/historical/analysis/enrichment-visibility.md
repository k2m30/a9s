# Enrichment Visibility Spec (v4)

**Parent**: #196 (issue counts and attention filter)
**Problem**: Wave 2 enrichment discovers hidden issues but the resource list shows no visual indicator. The menu says `issues:N` but all rows are green.

> **Historical spec.** This document captures the design as implemented. Since then: `EnricherResult` was renamed to `IssueEnricherResult`, `EnricherFunc` to `IssueEnricherFunc`, and `NoOpEnricher` to `NoOpIssueEnricher`. The monolithic `internal/aws/enrichment.go` was split across `internal/aws/issue_enrichment.go` (infrastructure) + one `*_issue_enrichment.go` per resource short name. For the current architecture, see [`docs/architecture.md`](architecture.md).

## Approach: Derived Banner + Finding Snapshots + Row Marker

### 1. EnrichmentFinding type (neutral package)

Lives in `internal/resource/` (not `internal/tui/` or `internal/aws/`) to avoid import cycles:

```go
// internal/resource/enrichment.go
type EnrichmentFinding struct {
    Severity string // "!" (broken) or "~" (informational)
    Summary  string // "pending maintenance: system-update (New OS patch)"
}
```

Both `internal/aws/issue_enrichment.go` (enricher return type) and `internal/tui/` (storage, display) import from `internal/resource/`.

### 2. Enricher return type

```go
// internal/aws/issue_enrichment.go
type IssueEnricherResult struct {
    IssueCount   int
    Truncated    bool
    TruncatedIDs map[string]bool
    Findings     map[string]resource.EnrichmentFinding // resourceID → finding
    FieldUpdates map[string]map[string]string
}
```

**Finding key normalization**: The `Findings` map is keyed by `resource.Resource.ID` — the same identifier used in list rows. Enrichers that receive identifiers from AWS APIs in a different form (e.g., ARNs from `DescribePendingMaintenanceActions`) MUST normalize to `Resource.ID` before storing the finding. The existing RDS enricher already does ARN-suffix matching; the same pattern applies to all enrichers. The canonical rule: **extract the resource identifier that matches `Resource.ID` as populated by the type's fetcher**. If the enricher cannot determine the matching ID, it skips that resource (no finding stored).

```go
// Example: RDS maintenance ARN → Resource.ID
// ARN: arn:aws:rds:eu-west-2:123456789012:db:docdb-docdb-dev
// Resource.ID: docdb-docdb-dev
// Enricher extracts "docdb-docdb-dev" from the ARN suffix

type IssueEnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error)
```

Enrichers return findings for every affected resource they inspect. For account-wide enrichers (RDS maintenance, EC2 status), findings cover all resources the API returns — not just those in the `resources` slice. This means findings can exist for resources that are off-page in `probeResources`.

### 3. Findings storage on root Model

```go
// On root Model:
enrichmentFindings map[string]map[string]resource.EnrichmentFinding // shortName → resourceID → finding
enrichmentRan      map[string]bool                                  // shortName → true if Wave 2 completed this session
enrichmentTypeGen  map[string]int                                   // shortName → per-type generation; bumped on every rerun
```

`enrichmentRan` is the "enrichment completed this session" signal — separate from `issueKnown` (which reflects cached counts). This drives banner visibility.

`enrichmentTypeGen` is a per-type stale-guard/token counter. It is bumped whenever a rerun for that type starts (startup Wave 2 dispatch, menu Ctrl+R, or top-level list Ctrl+R). `probeEnrichment` captures the current value; `handleEnrichmentChecked` drops messages with stale TypeGen. The wrapped Ctrl+R fetch also captures it as a token so overlapping Ctrl+R presses dispatch only the newest rerun.

**Populated by**: `handleEnrichmentChecked` — only on `msg.Err == nil`. Replaces (not merges) the per-type findings map from `msg.Findings`, sets `enrichmentRan[shortName] = true`. On error (`msg.Err != nil`), neither `enrichmentFindings` nor `enrichmentRan` is updated — the type keeps whatever state was set when the rerun started (cleared — see below).

**Invalidated per-type when a rerun starts**: When `startEnrichment()` fires a probe for a type, immediately clear that type's entry from `enrichmentFindings` and `enrichmentRan`. This ensures:
- If the rerun succeeds: findings are replaced with fresh data.
- If the rerun fails: the type has no findings (honest unknown state). Banner, markers, and detail section disappear for that type until the next successful enrichment.

This avoids stale findings surviving a failed rerun. The brief window between "rerun started" and "rerun completed" shows no findings for that type — this is correct since the old data is being replaced.

**Cleared entirely on**: profile switch and region switch. These clear ALL types' findings and `enrichmentRan` maps.

**Refresh semantics by path**:

| Refresh path | Wave 2 rerun? | Findings behavior |
|-------------|--------------|-------------------|
| Main menu `Ctrl+R` | Yes — restarts Wave 1 → Wave 2 | Per-type cleared as each probe starts, replaced on success |
| Top-level list `Ctrl+R` | **Yes — rerun Wave 2 for this type only** | This type's findings cleared on rerun start, replaced on success |
| Detail `Ctrl+R` | No | Findings unchanged (detail refresh is for related/detail enrichment, not Wave 2) |
| Profile/region switch | Yes — full restart | All findings cleared |

**New behavior for top-level list refresh**: When `Ctrl+R` is pressed on a top-level resource list, in addition to re-fetching the list data, also rerun Wave 2 enrichment for that specific resource type (if it has a registered enricher and the list is not in demo mode). This ensures the background-check banner/markers refresh alongside the list data.

**Implementation — token stamped on `ResourcesLoadedMsg`** (do NOT call `probeEnrichment` immediately with the current stale `probeResources`; that slice is cleared once the initial Wave 2 completes, so an immediate dispatch would enrich against an empty input and wipe valid findings):

1. Bump `m.enrichmentTypeGen[shortName]`; capture the new value as a token `tok`.
2. Clear `m.enrichmentFindings[shortName]` and `m.enrichmentRan[shortName]`.
3. Dispatch a **wrapped fetch command** that runs the normal refresh fetch and inspects the inner result:
   - On `ResourcesLoadedMsg`: stamp `msg.TypeGen = tok` and return the same message type.
   - On `APIErrorMsg`: pass through unchanged (no rerun).
   - On any other message: pass through unchanged.
4. The existing `ResourcesLoadedMsg` handler at `internal/tui/app.go:428` does its full list-update work unconditionally (unchanged). After its existing write-through block, add a small tail branch:

   ```go
   if msg.TypeGen != 0 && msg.TypeGen == m.enrichmentTypeGen[msg.ResourceType] {
       m.probeResources[msg.ResourceType] = msg.Resources
       cmd = tea.Batch(cmd, m.probeEnrichment(msg.ResourceType, msg.TypeGen))
   }
   ```

   Normal fetches carry `TypeGen=0` and the tail is a no-op. Stale Ctrl+R fetches (overlap) carry a `TypeGen` that no longer matches and the tail is skipped — but the list still updated because the existing handler body runs first. Fresh Ctrl+R fetches match and dispatch the rerun.

**Why a field instead of a new message type**: The existing handler already does the unconditional list update we need; stamping a token onto the existing `ResourcesLoadedMsg` lets us add the rerun as a conditional tail without duplicating handler logic, introducing a new message type, or extracting a helper. Earlier drafts of this design used an `EnrichmentRerunReadyMsg` composite envelope — that was more complex and strictly less maintainable because the envelope's handler had to re-implement (or call into) the existing list-update path.

This handles overlapping refreshes and failed fetches correctly: a failed refresh emits `APIErrorMsg` (existing error banner applies) and leaves findings cleared (honest unknown) with no latent state. No shared flags, no new message types.

### 4. List-level enrichment banner (derived from list state)

The banner is **derived in `applySortAndFilter()`** after every data change.

**Derivation logic**:

```text
findingCount       = len(enrichmentFindings[shortName])           // severity-agnostic (! AND ~)
visibleIssueCount  = count of IsIssueRowColor(r.Status) in filteredResources
visibleFindingCount = count of r in filteredResources where findingsByID[r.ID] exists
showEnrichmentBanner =
    findingCount > 0               // any finding exists (NOT tied to menu-badge IssueCount)
    AND enrichmentRanThisSession   // Wave 2 actually completed for this type (not just cached)
    AND visibleIssueCount == 0     // no issue-colored rows in the VISIBLE (filtered) set
```

**Important**: The banner keys off `findingCount = len(Findings)`, NOT the menu-badge `IssueCount`. This is so that `~`-severity findings (e.g., RDS/DocDB pending maintenance) still trigger the banner — they don't bump the menu badge but they are legitimate hidden issues worth surfacing. The menu-badge rule (FR-015, decision 13: `!`-only) is independent and preserved.

Note: `visibleIssueCount` and `visibleFindingCount` both count from `filteredResources`, not `allResources`. This means a text filter that hides all issue-status rows will show the banner (correct — the visible list is all green), and the short-form banner correctly reflects whether any marked row is actually visible after filtering.

**Fields on ResourceListModel**:

```go
enrichmentIssueCount      int  // from menu, updated when enrichment completes
enrichmentTruncated       bool // true if the enrichment count is a lower bound (issues:N+)
enrichmentRanThisSession  bool // true only after Wave 2 ran for this type this session
```

Updated by the root Model via setters when enrichment completes or when the list is created from the menu.

**Banner text** — no row-specific guidance since rows have no affordance:

```text
⚠ N issues detected by background checks — not visible on this page
```

When truncated (`enrichmentTruncated == true`):

```text
⚠ N+ issues detected by background checks — not visible on this page
```

When at least one **visible (post-filter)** resource has a finding (checked against `findingsByID` against `filteredResources`, not `allResources`):

```text
⚠ N issues detected by background checks
```

(Or `N+` if truncated.) No "press d" or "load more" hints. The banner states a fact. The user can press `d` on any row to see if that specific resource has a finding — the detail view will show it if one exists.

**Why visible-only**: The suffix "— not visible on this page" is a factual claim about the current filtered view. If a text filter hides all marked rows, they are genuinely not visible — the long-form wording is accurate. Using `loaded` (`allResources`) instead would display "no suffix" while the user sees zero marked rows, which is misleading.

**Banner lifecycle**:
- Derived after every `applySortAndFilter()` call
- Disappears when `visibleIssueCount > 0` (issue-colored rows become visible)
- **On top-level list `Ctrl+R`**: findings for that type are cleared immediately (banner disappears), then restored on wrapped fetch success → stamped `ResourcesLoadedMsg` → tail-branch token match → successful `probeEnrichment` completion. A failed refresh (`APIErrorMsg`) leaves findings cleared (banner stays gone — honest unknown).
- **On detail `Ctrl+R`**: findings unchanged; banner state unchanged.
- Disappears on profile/region switch (findings cleared, Wave 2 reruns).

### 5. Row marker for loaded resources with findings

For loaded resources that have a finding in `enrichmentFindings`, add a minimal row marker: a colored `·` (middle dot) prepended to the Name/ID column value. This gives the user a truthful per-row affordance without the full prefix system.

**Column targeting**: The marker is attached to the **identity column** — resolved semantically, not by position. The resolver finds the identity column using a cascade that accounts for the current view config format where many columns have `Key` empty and only `Path` or `Title` set:

1. Match `c.key == typeDef.IdentityKey` (new optional field on `ResourceTypeDef`)
2. Match `c.key == "name"` (common convention)
3. Match `c.path` contains "Name" or "Identifier" (covers path-only configs like `Path: "DBInstanceIdentifier"`)
4. Match `strings.EqualFold(c.title, "Name")` or `strings.EqualFold(c.title, typeDef.Name)` (title-based fallback)
5. Fall back to index 0 of the resolved column list

The marker stays with the identity column even if the user scrolls horizontally and the column moves off-screen.

**Implementation**: In `table_render.go`, resolve the marker target column once per render pass:

```go
markerColIdx := resolveIdentityColumn(resolvedColumns, m.typeDef)

func resolveIdentityColumn(cols []resolvedColumn, td resource.ResourceTypeDef) int {
    for i, c := range cols {
        if td.IdentityKey != "" && c.key == td.IdentityKey { return i }
    }
    for i, c := range cols {
        if c.key == "name" { return i }
    }
    for i, c := range cols {
        if strings.Contains(c.path, "Name") || strings.Contains(c.path, "Identifier") { return i }
    }
    for i, c := range cols {
        if strings.EqualFold(c.title, "Name") { return i }
    }
    return 0
}
```

`IdentityKey` is a new optional field on `ResourceTypeDef`. If unset, the cascade handles it. Most current types will resolve via the `path` or `title` match without needing `IdentityKey` set.

The `enrichmentFindingsForType` reference is passed to the list model at creation/update (the subset of `m.enrichmentFindings[shortName]` for this type). The dot renders in ColStopped for `!` severity, ColPending for `~`.

This is:
- Truthful: only marks rows that actually have findings
- Minimal: 2 chars, no column width impact on healthy rows
- Not a prefix on Status: avoids the v1 complexity of `! available`
- NO_COLOR safe: `·` is a visible character

### 6. Detail view shows finding

Finding snapshot passed to `DetailModel` at construction:

```go
// On DetailModel:
enrichmentFinding *resource.EnrichmentFinding
```

Set in `handleNavigate`:

```go
if findings, ok := m.enrichmentFindings[resourceType]; ok {
    if f, ok := findings[res.ID]; ok {
        detail.SetEnrichmentFinding(&f)
    }
}
```

**Stale snapshot handling**: Current detail refresh (`Ctrl+R` in detail view) re-enriches the resource via `EnrichDetailMsg` but does NOT reconstruct the `DetailModel`. To handle the case where enrichment completes while a detail view is open, add a message-based update path:

When `EnrichmentCheckedMsg` arrives and the active view is a `DetailModel` for a resource of the enriched type:

```go
if detail, ok := m.activeView().(*views.DetailModel); ok {
    if detail.ResourceType() == msg.ResourceType {
        if f, ok := msg.Findings[detail.ResourceID()]; ok {
            detail.SetEnrichmentFinding(&f)
        } else {
            // Resource recovered — clear stale finding
            detail.SetEnrichmentFinding(nil)
        }
    }
}
```

This handles both cases: enrichment discovers a new finding (set it), and enrichment reruns and the resource is no longer affected (clear it).

This handles: detail opened before enrichment → enrichment completes → detail updates live.

YAML and JSON views do NOT show findings.

**Rendered in detail view**: When finding is non-nil, render at the top:

```text
⚠ Background Check
  pending maintenance: system-update (New OS patch)
```

ColStopped for `!`, ColPending for `~`. Absent when finding is nil.

### Tier Classification

| Severity | Meaning | Menu badge? | Row marker | Detail section |
|----------|---------|-------------|-----------|---------------|
| `!` | Actively broken/degraded | Yes | Red `·` | Red "Background Check" |
| `~` | Scheduled/informational | No | Yellow `·` | Yellow "Background Check" |

| Type | Enricher | Severity | Summary format |
|------|----------|----------|---------------|
| EC2 | DescribeInstanceStatus | `!` | `"system status impaired"` |
| EBS | DescribeVolumeStatus | `!` | `"volume I/O degraded"` |
| Target Groups | DescribeTargetHealth | `!` | `"unhealthy targets: 2/5"` |
| CodeBuild | BatchGetBuilds | `!` | `"latest build FAILED (2026-04-13)"` |
| CodePipeline | GetPipelineState | `!` | `"stage Deploy failed"` |
| SFN | ListExecutions | `!` | `"latest execution FAILED"` |
| Glue | GetJobRuns | `!` | `"latest run FAILED"` |
| DynamoDB | DescribeContinuousBackups | `!` | `"PITR disabled"` |
| RDS/DocDB | DescribePendingMaintenanceActions | `~` | `"pending maintenance: system-update, os-upgrade"` |
| ASG | DescribeScalingActivities | `!` | `"scaling failure: insufficient capacity"` |
| Backup | ListBackupJobs | `!` | `"backup job FAILED (2026-04-13)"` |
| SES | GetAccount | `!` | `"sending PAUSED — enforcement status"` |
| KMS | GetKeyRotationStatus | `!` | `"automatic rotation disabled"` |
| EFS | DescribeMountTargets | `!` | `"no mount targets"` |
| TGW | DescribeTransitGatewayAttachments | `!` | `"attachment state: failing"` |
| VPC | DescribeFlowLogs | `!` | `"no flow logs (CIS EC2.6)"` |
| S3 | GetPublicAccessBlock | `!` | `"public access block incomplete"` |
| ECS Services | DescribeServices | `!` | `"deployment FAILED: reason"` |
| ECS Clusters | DescribeClusters | `!` | `"active services: 5, running tasks: 12"` |
| ECS Tasks | DescribeTasks | `!` | `"task stopped: EssentialContainerExited"` |
| EB Rules | ListTargetsByRule | `!` | `"no targets attached"` |
| EB Env | DescribeEnvironmentHealth | `!` | `"health: Red — cause"` |
| ELB | DescribeLoadBalancerAttributes | `!` | `"access logs disabled, PAB off"` |
| SQS | GetQueueAttributes | `!` | `"no DLQ configured, no KMS encryption"` |
| SNS | ListSubscriptionsByTopic | `!` | `"no subscriptions"` |
| MSK | DescribeClusterV2 | `!` | `"TLS disabled, outdated Kafka version"` |
| ACM | DescribeCertificate | `!` | `"expires in 7 days"` |
| CloudFront | GetDistributionConfig | `!` | `"TLSv1.0, no custom error response"` |
| API Gateway | GetStages | `!` | `"no throttling, no access logging"` |
| CloudFormation | DescribeStackEvents | `!` | `"stack event: UPDATE_FAILED"` |
| ECR | DescribeImageScanFindings | `!` | `"HIGH: 3, CRITICAL: 1"` |
| CodeArtifact | GetRepositoryPermissionsPolicy | `!` | `"no permissions policy"` |
| Athena | GetWorkGroup | `!` | `"query enforcement disabled, no encryption"` |
| Route 53 | GetHostedZone | `!` | `"private zone, orphan (0 records)"` |
| WAF | GetLoggingConfiguration | `!` | `"no logging, no associated resources"` |
| IAM Roles | GetRole | `!` | `"role unused >90 days"` |
| IAM Policies | GetPolicyVersion | `!` | `"admin-star policy (CIS IAM.16)"` |
| IAM Users | GetLoginProfile + ListMFADevices | `!` | `"console access, no MFA"` |
| IAM Groups | GetGroup + ListAttachedGroupPolicies | `!` | `"inline policies attached, no members"` |
| CW Logs | DescribeMetricFilters | `!` | `"no metric filters (audit gap)"` |

Types with Wave 2 = "None" (24 total) are registered as `NoOpIssueEnricher` — returns zero findings, zero issues. Some types use in-fetcher Wave 2 (their fetcher already performs per-resource Describe calls and populates health fields; the `NoOpIssueEnricher` entry exists for contract conformance).

### What This Does NOT Change

- `resource.Resource.Fields` — no mutation
- `IsIssueRowColor` predicate — unchanged
- `issueStatusSet` — unchanged
- `AttentionFilter` semantics — unchanged (keys off Status via `IsDimRowColor`)
- Disk cache format — unchanged
- `resourceCacheEntry` — unchanged
- EC2 fetcher-level `! running`/`~ running` — unchanged

### Acceptance Criteria

- [ ] `EnrichmentFinding` lives in `internal/resource/`, no import cycles
- [ ] `IssueEnricherFunc` returns `IssueEnricherResult` with typed `Findings` map
- [ ] Findings keyed by `resource.Resource.ID` — enrichers normalize API identifiers (ARNs) to match
- [ ] Findings cover all affected resources from the API, including off-page
- [ ] `enrichmentRan` is separate from `issueKnown` — banner does NOT show on cold start
- [ ] Banner derived from `filteredResources` (not `allResources`) — text filter hiding issue rows triggers banner
- [ ] Banner text does not promise row-level d affordance
- [ ] Banner disappears when visible issue-colored rows appear
- [ ] Banner text shows `N+` when enrichment count is truncated
- [ ] Row marker `·` shown on the identity column (resolved semantically via cascade, index 0 fallback) for loaded resources with findings
- [ ] Row marker stable across custom view configs and horizontal scroll
- [ ] Row marker colored by severity (red `!`, yellow `~`)
- [ ] Detail view shows "Background Check" section for resources with findings
- [ ] Detail section updates live when enrichment completes while detail is open
- [ ] Detail section clears when enrichment reruns and the resource is no longer affected (recovery)
- [ ] YAML/JSON views do NOT show findings
- [ ] Findings session-scoped, cleared entirely on profile/region switch
- [ ] Per-type findings invalidated (cleared) when a Wave 2 rerun starts for that type
- [ ] Failed Wave 2 rerun leaves the type with no findings (honest unknown, no stale data)
- [ ] Top-level list Ctrl+R reruns Wave 2 for that specific type (alongside list re-fetch), seeded from the freshly fetched resources (not stale `probeResources`)
- [ ] Per-type findings replaced on enrichment rerun
- [ ] Overlapping Ctrl+R presses on the same list are safe: older fetches still update the list (existing handler behavior), but their tail-branch token check fails so no rerun dispatches; only the newest press has a token that matches `enrichmentTypeGen[T]` and dispatches enrichment
- [ ] Stale Ctrl+R fetches still apply the list update via the existing handler; only the rerun tail-branch is skipped — FR-014 requires the list to refresh on every Ctrl+R success
- [ ] Failed refresh leaves no latent rerun state; findings remain cleared until a later successful refresh reruns enrichment
- [ ] Banner keys off `len(Findings)` (severity-agnostic), NOT the menu-badge `IssueCount` — RDS/DocDB with `~` findings still triggers the banner
- [ ] Short-form banner ("no suffix") fires only when a finding exists in the VISIBLE (filtered) row set
- [ ] RDS/DocDB: severity `~`, NOT counted in menu badge

### Files Affected

| File | Change |
|------|--------|
| `internal/resource/enrichment.go` | NEW — `EnrichmentFinding` type |
| `internal/aws/issue_enrichment.go` | `IssueEnricherResult` type; enrichers return findings (formerly `internal/aws/enrichment.go`, split per short name) |
| `internal/tui/app.go` | Adds `enrichmentFindings`, `enrichmentRan`, `enrichmentTypeGen` maps. The existing `ResourcesLoadedMsg` case at `app.go:428` gets a small tail branch after its existing write-through block: `if msg.TypeGen != 0 && msg.TypeGen == m.enrichmentTypeGen[T] { seed probeResources; dispatch probeEnrichment }`. No helper extraction — the existing handler body already performs the unconditional list update. |
| `internal/tui/app_handlers.go` | Clear all findings + `enrichmentTypeGen` on profile/region switch; on list Ctrl+R: bump `enrichmentTypeGen[T]`, clear `enrichmentFindings[T]`/`enrichmentRan[T]`, dispatch wrapped fetch command capturing the new gen as a token |
| `internal/tui/app_handlers_navigate.go` | Store findings (validating BOTH session-wide `Gen` and per-type `TypeGen`); invalidate per-type on rerun start; update detail live; pass enrichment state to list |
| `internal/tui/app_fetchers.go` | `probeEnrichment` captures and carries `TypeGen` (per-type gen); returns `Findings`; NEW wrapped fetch command for the Ctrl+R-for-rerun path. Shape: runs inner `refreshResourceList`; on `ResourcesLoadedMsg` result stamps `msg.TypeGen = tok` and returns the same message type; `APIErrorMsg` and any other inner message pass through unchanged |
| `internal/tui/messages/messages.go` | `ResourcesLoadedMsg` gets new `TypeGen int` field (0 on normal fetches; non-zero only on Ctrl+R-for-rerun path); `EnrichmentCheckedMsg` carries `Findings` map + `TypeGen` field. No new message types. |
| `internal/tui/views/resourcelist.go` | `enrichmentIssueCount`, `enrichmentRanThisSession`; banner derivation |
| `internal/tui/views/resourcelist_helpers.go` | Render banner; `visibleIssueCount` from filteredResources |
| `internal/tui/views/table_render.go` | `·` row marker on identity column for resources with findings |
| `internal/resource/types.go` | Optional `IdentityKey` field on `ResourceTypeDef` |
| `internal/tui/views/detail.go` | `enrichmentFinding` field; `SetEnrichmentFinding()` |
| `internal/tui/views/detail_fields.go` | Render "Background Check" section |

### Implementation Order

1. `EnrichmentFinding` in `internal/resource/enrichment.go`
2. `IssueEnricherResult` + update `IssueEnricherFunc` signature in `internal/aws/issue_enrichment.go`
3. Update all enrichers to return `IssueEnricherResult` with findings (43 real + 24 `NoOpIssueEnricher`)
4. `enrichmentFindings` + `enrichmentRan` + `enrichmentTypeGen` maps on Model; add `TypeGen` field to `EnrichmentCheckedMsg`; store findings from `EnrichmentCheckedMsg` only when BOTH session-wide `Gen` and per-type `TypeGen` match (drop stale)
5. Clear all findings and `enrichmentTypeGen` on profile/region switch; invalidate per-type (bump `TypeGen`, clear findings/ran) when a rerun starts
6. Add `TypeGen int` field to `ResourcesLoadedMsg`; build wrapped fetch command in `app_fetchers.go` that stamps the captured token onto the `ResourcesLoadedMsg` it forwards (passes `APIErrorMsg` through unchanged).
6a. Wire top-level list Ctrl+R path: bump `enrichmentTypeGen[T]`, clear findings/ran, dispatch wrapped fetch. In the existing `ResourcesLoadedMsg` case at `app.go:428`, add a tail branch that checks the stamped token and seeds `probeResources` + dispatches `probeEnrichment` only on match. Overlap-safe: older fetches still update the list but their tail branch fails the token check. Error-safe: `APIErrorMsg` path unchanged; findings stay cleared (honest unknown).
7. `enrichmentIssueCount` + `enrichmentRanThisSession` + `findingsByID` on ResourceListModel; derive banner with severity-agnostic `findingCount = len(Findings)` and visible-only short-form check
8. Render banner in View()
9. `·` row marker in table_render.go
10. Pass finding to DetailModel; live update on enrichment completion
11. Render "Background Check" in detail view
12. Tests — including explicit cases for overlapping Ctrl+R (stale ready-msg dropped), failed refresh (no latent state), `~`-only finding triggers banner (RDS case), and visible-only short banner under text filter
