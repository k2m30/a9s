# Enrichment Visibility Spec (v3)

**Parent**: #196 (issue counts and attention filter)
**Problem**: Wave 2 enrichment discovers hidden issues but the resource list shows no visual indicator. The menu says `issues:N` but all rows are green.

## Approach: Derived Banner + Finding Snapshots

### 1. List-level enrichment banner (derived, not set-once)

The banner is **derived after every list data change** — not set at construction time. It is recomputed in `applySortAndFilter()` (which already runs after `ResourcesLoadedMsg`, load-more, refresh, and filter changes).

**Derivation logic** (in `ResourceListModel`):

```
showEnrichmentBanner =
    enrichmentIssueCount > 0           // menu-level enrichment found issues for this type
    AND enrichmentFindingsAvailable     // findings were populated THIS session (not just cached counts)
    AND issueCount == 0                 // no IsIssueRowColor rows on the currently loaded page
```

The two inputs (`enrichmentIssueCount` and `enrichmentFindingsAvailable`) are passed to the list model at creation and updated when enrichment completes. They are NOT derived from `issueKnown` or cached counts — they are a separate signal:

```go
// On ResourceListModel:
enrichmentIssueCount        int  // from menu.GetIssueCounts(), set at creation + updated on enrichment
enrichmentFindingsAvailable bool // true only after Wave 2 findings were populated THIS session
```

**Why `enrichmentFindingsAvailable` is separate from `issueKnown`**: On cold start, cached issue counts restore `issueKnown=true` and `issueCounts[type]=N`, but no session findings exist yet. The banner must NOT show until Wave 2 actually runs and populates findings. `enrichmentFindingsAvailable` is set to true only when `EnrichmentCheckedMsg` arrives with findings for this type.

**Banner text** adapts based on whether any loaded row has a finding:

- If at least one loaded row has a finding in the session findings map:
  `"⚠ N issues found by background checks (press d on highlighted rows)"`
- If no loaded row has a finding (all affected resources are off-page):
  `"⚠ N issues found by background checks (load more pages with m)"`

This eliminates the false "d for details" hint when findings are off-page.

**Banner lifecycle**:
- Appears: after `applySortAndFilter()` when derivation condition is true
- Updates: on load-more (may disappear if loaded page now has issue rows, or text changes)
- Disappears: when user loads a page containing issue-status rows (`issueCount > 0`), or on refresh, or when the filter changes the visible set

### 2. Typed enrichment findings (session-scoped, passed to views)

```go
// On root Model:
type EnrichmentFinding struct {
    Severity string // "!" (broken) or "~" (informational)
    Summary  string // "pending maintenance: system-update (New OS patch)"
}

// shortName → resourceID → finding
enrichmentFindings map[string]map[string]EnrichmentFinding
```

**Populated by**: enrichers, after each `EnrichmentCheckedMsg`. The enricher iterates over `probeResources[shortName]`, checks each resource, and stores a finding for affected ones.

**Cleared on**: profile switch, region switch, AND manual refresh (Ctrl+R). Per-type entries are replaced (not merged) when enrichment reruns for that type, so stale findings from a previous enrichment cycle don't linger.

**NOT stored in**: `resource.Resource.Fields`, disk cache, or `resourceCacheEntry`.

**Passed to DetailModel**: When `handleNavigate` constructs a `DetailModel`, it looks up `m.enrichmentFindings[resourceType][resource.ID]` and passes the finding as an explicit field on `DetailModel`:

```go
// On DetailModel (new field):
enrichmentFinding *EnrichmentFinding // nil = no finding for this resource
```

Set at construction in `handleNavigate`:

```go
detail := views.NewDetailModel(resource, viewConfig, keys)
if f, ok := m.enrichmentFindings[resourceType][resource.ID]; ok {
    detail.SetEnrichmentFinding(&f)
}
```

No message path needed. No root-Model access from DetailModel. The finding is a snapshot passed at construction time — if the user refreshes detail (Ctrl+R), the detail is reconstructed from the root Model which has the current findings.

**Rendered in detail view**: When `enrichmentFinding != nil`, render a section at the top of the detail view (after the title, before the first field group):

```
⚠ Background Check
  pending maintenance: system-update (New OS patch)
```

Styled with ColPending (yellow) for `~` severity, ColStopped (red) for `!`. If the finding is nil, this section is absent — no change to normal detail rendering.

YAML and JSON views do NOT show findings (findings are operational metadata, not resource state).

### 3. Enricher changes

Enrichers currently return `(issueCount int, truncated bool, err error)`. The signature stays the same. The new behavior: after calling the AWS API, the enricher also populates `m.enrichmentFindings[shortName][resourceID]` for each affected resource.

This requires the enricher to have access to the findings map. Two options:

**Option A**: Pass findings map to the enricher (change `EnricherFunc` signature).
**Option B**: Return findings alongside counts; handler stores them.

Option B is cleaner — enrichers stay pure functions:

```go
type EnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error)

type EnricherResult struct {
    IssueCount int
    Truncated  bool
    Findings   map[string]EnrichmentFinding // resourceID → finding
}
```

The handler extracts `Findings` from the result and stores them in `m.enrichmentFindings[shortName]`.

### Tier Classification

| Severity | Meaning | Menu badge? | Examples |
|----------|---------|-------------|---------|
| `!` | Actively broken/degraded | Yes | Impaired checks, unhealthy targets, failed builds |
| `~` | Scheduled/informational | No | Pending maintenance |

| Type | Enricher | Severity | Summary format |
|------|----------|----------|---------------|
| EC2 | DescribeInstanceStatus | `!` | `"impaired: system-status impaired"` |
| EBS | DescribeVolumeStatus | `!` | `"impaired: volume I/O degraded"` |
| Target Groups | DescribeTargetHealth | `!` | `"unhealthy targets: 2/5"` |
| CodeBuild | BatchGetBuilds | `!` | `"latest build FAILED (2026-04-13)"` |
| CodePipeline | GetPipelineState | `!` | `"stage Deploy failed"` |
| SFN | ListExecutions | `!` | `"latest execution FAILED"` |
| Glue | GetJobRuns | `!` | `"latest run FAILED"` |
| DynamoDB | DescribeTable | `!` | `"table status: UPDATING"` |
| RDS/DocDB | DescribePendingMaintenanceActions | `~` | `"pending maintenance: system-update, os-upgrade"` |

### What This Does NOT Change

- `resource.Resource.Fields` — no mutation, no `_issue` keys
- `IsIssueRowColor` predicate — unchanged
- `issueStatusSet` — unchanged
- `AttentionFilter` semantics — unchanged (still keys off Status via `IsDimRowColor`)
- Disk cache format — unchanged
- `resourceCacheEntry` — unchanged
- EC2 fetcher-level `! running`/`~ running` — unchanged (that's fetcher enrichment, not Wave 2)

### Acceptance Criteria

- [ ] Banner appears when enrichment found issues but loaded page has zero issue-status rows
- [ ] Banner recomputes on every list data change (load-more, refresh, filter)
- [ ] Banner does NOT appear on cold start with cached counts before Wave 2 runs
- [ ] Banner text says "press d on highlighted rows" only when at least one loaded row has a finding
- [ ] Banner text says "load more pages with m" when all findings are off-page
- [ ] Banner disappears when loaded page gains issue-status rows
- [ ] Detail view shows "Background Check" section for resources with findings
- [ ] Detail section is absent for resources without findings
- [ ] Detail section uses severity-appropriate color (red for `!`, yellow for `~`)
- [ ] YAML/JSON views do NOT show findings
- [ ] Findings are session-scoped, not cached to disk
- [ ] Findings cleared on profile/region switch AND manual refresh
- [ ] Per-type findings replaced (not merged) on enrichment rerun
- [ ] RDS/DocDB findings: severity `~`, NOT counted in menu badge
- [ ] `EnricherFunc` returns `EnricherResult` with `Findings` map
- [ ] EC2 fetcher-level `! running`/`~ running` unchanged

### Files Affected

| File | Change |
|------|--------|
| `internal/aws/enrichment.go` | `EnricherResult` struct; enrichers return findings per resource |
| `internal/tui/app.go` | `enrichmentFindings` map; `EnrichmentFinding` struct |
| `internal/tui/app_handlers.go` | Clear findings on profile/region switch + refresh |
| `internal/tui/app_handlers_navigate.go` | Store findings from enricher results; pass finding to DetailModel; pass enrichment state to list model |
| `internal/tui/app_fetchers.go` | Update `probeEnrichment` to carry findings in message |
| `internal/tui/messages/messages.go` | `EnrichmentCheckedMsg` carries `Findings` map |
| `internal/tui/views/resourcelist.go` | `enrichmentIssueCount`, `enrichmentFindingsAvailable` fields; banner derivation in `applySortAndFilter()` |
| `internal/tui/views/resourcelist_helpers.go` | Render banner in `View()` |
| `internal/tui/views/detail.go` | `enrichmentFinding *EnrichmentFinding` field; `SetEnrichmentFinding()` |
| `internal/tui/views/detail_fields.go` | Render "Background Check" section |

### Implementation Order

1. Define `EnrichmentFinding` and `EnricherResult` types
2. Change `EnricherFunc` signature to return `EnricherResult`
3. Update 9 enrichers to populate `Findings` in results
4. Add `enrichmentFindings` map to Model; store findings from `EnrichmentCheckedMsg`
5. Clear findings on profile/region switch + refresh; replace per-type on rerun
6. Add `enrichmentIssueCount`, `enrichmentFindingsAvailable` to ResourceListModel
7. Derive banner in `applySortAndFilter()`; render in View()
8. Pass finding snapshot to DetailModel at construction
9. Render "Background Check" section in detail view
10. Tests for banner derivation, finding lifecycle, detail rendering
