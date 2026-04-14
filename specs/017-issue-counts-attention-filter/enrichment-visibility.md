# Enrichment Visibility Spec (v4)

**Parent**: #196 (issue counts and attention filter)
**Problem**: Wave 2 enrichment discovers hidden issues but the resource list shows no visual indicator. The menu says `issues:N` but all rows are green.

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

Both `internal/aws/enrichment.go` (enricher return type) and `internal/tui/` (storage, display) import from `internal/resource/`.

### 2. Enricher return type

```go
// internal/aws/enrichment.go
type EnricherResult struct {
    IssueCount int
    Truncated  bool
    Findings   map[string]resource.EnrichmentFinding // resourceID → finding
}
```

**Finding key normalization**: The `Findings` map is keyed by `resource.Resource.ID` — the same identifier used in list rows. Enrichers that receive identifiers from AWS APIs in a different form (e.g., ARNs from `DescribePendingMaintenanceActions`) MUST normalize to `Resource.ID` before storing the finding. The existing RDS enricher already does ARN-suffix matching (`enrichment.go:83`); the same pattern applies to all enrichers. The canonical rule: **extract the resource identifier that matches `Resource.ID` as populated by the type's fetcher**. If the enricher cannot determine the matching ID, it skips that resource (no finding stored).

```go
// Example: RDS maintenance ARN → Resource.ID
// ARN: arn:aws:rds:eu-west-2:123456789012:db:docdb-docdb-dev
// Resource.ID: docdb-docdb-dev
// Enricher extracts "docdb-docdb-dev" from the ARN suffix

type EnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error)
```

Enrichers return findings for every affected resource they inspect. For account-wide enrichers (RDS maintenance, EC2 status), findings cover all resources the API returns — not just those in the `resources` slice. This means findings can exist for resources that are off-page in `probeResources`.

### 3. Findings storage on root Model

```go
// On root Model:
enrichmentFindings map[string]map[string]resource.EnrichmentFinding // shortName → resourceID → finding
enrichmentRan      map[string]bool // shortName → true if Wave 2 completed this session
```

`enrichmentRan` is the "enrichment completed this session" signal — separate from `issueKnown` (which reflects cached counts). This drives banner visibility.

**Populated by**: `handleEnrichmentChecked` — only on `msg.Err == nil`. Replaces (not merges) the per-type findings map from `msg.Findings`, sets `enrichmentRan[shortName] = true`. On error (`msg.Err != nil`), neither `enrichmentFindings` nor `enrichmentRan` is updated for that type — a failed enrichment call does not count as "enriched this session."

**Cleared on**: profile switch, region switch, AND manual refresh. Both `enrichmentFindings` and `enrichmentRan` are cleared.

### 4. List-level enrichment banner (derived from list state)

The banner is **derived in `applySortAndFilter()`** after every data change.

**Derivation logic**:

```
visibleIssueCount = count of IsIssueRowColor(r.Status) in filteredResources (not allResources)
showEnrichmentBanner =
    enrichmentIssueCount > 0      // menu-level enrichment found issues for this type
    AND enrichmentRanThisSession   // Wave 2 actually completed for this type (not just cached)
    AND visibleIssueCount == 0     // no issue-colored rows in the VISIBLE (filtered) set
```

Note: `visibleIssueCount` counts from `filteredResources`, not `allResources`. This means a text filter that hides all issue-status rows will show the banner (correct — the visible list is all green).

**Fields on ResourceListModel**:

```go
enrichmentIssueCount      int  // from menu, updated when enrichment completes
enrichmentTruncated       bool // true if the enrichment count is a lower bound (issues:N+)
enrichmentRanThisSession  bool // true only after Wave 2 ran for this type this session
```

Updated by the root Model via setters when enrichment completes or when the list is created from the menu.

**Banner text** — no row-specific guidance since rows have no affordance:

```
⚠ N issues detected by background checks — not visible on this page
```

When truncated (`enrichmentTruncated == true`):

```
⚠ N+ issues detected by background checks — not visible on this page
```

When at least one loaded resource has a finding (checked against `enrichmentFindings` by ID):

```
⚠ N issues detected by background checks
```

(Or `N+` if truncated.) No "press d" or "load more" hints. The banner states a fact. The user can press `d` on any row to see if that specific resource has a finding — the detail view will show it if one exists.

**Banner lifecycle**:
- Derived after every `applySortAndFilter()` call
- Disappears when `visibleIssueCount > 0` (issue-colored rows become visible)
- Disappears on refresh (findings cleared, enrichment reruns)

### 5. Row marker for loaded resources with findings

For loaded resources that have a finding in `enrichmentFindings`, add a minimal row marker: a colored `·` (middle dot) prepended to the Name/ID column value. This gives the user a truthful per-row affordance without the full prefix system.

**Column targeting**: The marker is attached to the **identity column** — resolved semantically, not by position. The resolver checks the resolved columns (from `resolveColumns()` in `table_render.go`) for the first column whose key matches the resource type's identity field (typically `"name"`, `"function_name"`, `"db_identifier"`, `"project_name"`, etc.). If no semantic match is found, fall back to index 0 of the resolved column list. This is stable across custom view configs where users reorder columns.

The marker stays with the identity column even if the user scrolls horizontally and the column moves off-screen. This is consistent with how the cursor highlight works — it highlights the logical row, not the visible viewport.

**Implementation**: In `table_render.go`, resolve the marker target column once per render pass:

```go
markerColIdx := 0 // fallback
for i, c := range resolvedColumns {
    if c.key == m.typeDef.IdentityKey || c.key == "name" {
        markerColIdx = i
        break
    }
}

// Then during row rendering:
if colIdx == markerColIdx {
    if finding, ok := enrichmentFindingsForType[r.ID]; ok {
        val = "· " + val
        // Override cell style to finding severity color
    }
}
```

Note: `IdentityKey` is a new optional field on `ResourceTypeDef` that names the column key for the primary identifier. If unset, falls back to `"name"`. This avoids hardcoding column names per resource type in the renderer.

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

```
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
| DynamoDB | DescribeTable | `!` | `"table status: UPDATING"` |
| RDS/DocDB | DescribePendingMaintenanceActions | `~` | `"pending maintenance: system-update, os-upgrade"` |

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
- [ ] `EnricherFunc` returns `EnricherResult` with typed `Findings` map
- [ ] Findings keyed by `resource.Resource.ID` — enrichers normalize API identifiers (ARNs) to match
- [ ] Findings cover all affected resources from the API, including off-page
- [ ] `enrichmentRan` is separate from `issueKnown` — banner does NOT show on cold start
- [ ] Banner derived from `filteredResources` (not `allResources`) — text filter hiding issue rows triggers banner
- [ ] Banner text does not promise row-level d affordance
- [ ] Banner disappears when visible issue-colored rows appear
- [ ] Banner text shows `N+` when enrichment count is truncated
- [ ] Row marker `·` shown on column index 0 (Name/ID) for loaded resources with findings
- [ ] Row marker attached to logical column, not leftmost visible after horizontal scroll
- [ ] Row marker colored by severity (red `!`, yellow `~`)
- [ ] Detail view shows "Background Check" section for resources with findings
- [ ] Detail section updates live when enrichment completes while detail is open
- [ ] Detail section clears when enrichment reruns and the resource is no longer affected (recovery)
- [ ] YAML/JSON views do NOT show findings
- [ ] Findings session-scoped, cleared on profile/region switch AND refresh
- [ ] Per-type findings replaced on enrichment rerun
- [ ] RDS/DocDB: severity `~`, NOT counted in menu badge

### Files Affected

| File | Change |
|------|--------|
| `internal/resource/enrichment.go` | NEW — `EnrichmentFinding` type |
| `internal/aws/enrichment.go` | `EnricherResult` type; enrichers return findings |
| `internal/tui/app.go` | `enrichmentFindings`, `enrichmentRan` maps |
| `internal/tui/app_handlers.go` | Clear findings + ran on profile/region switch + refresh |
| `internal/tui/app_handlers_navigate.go` | Store findings; update detail live; pass enrichment state to list |
| `internal/tui/app_fetchers.go` | `probeEnrichment` carries findings in message |
| `internal/tui/messages/messages.go` | `EnrichmentCheckedMsg` carries `Findings` map |
| `internal/tui/views/resourcelist.go` | `enrichmentIssueCount`, `enrichmentRanThisSession`; banner derivation |
| `internal/tui/views/resourcelist_helpers.go` | Render banner; `visibleIssueCount` from filteredResources |
| `internal/tui/views/table_render.go` | `·` row marker on identity column for resources with findings |
| `internal/resource/types.go` | Optional `IdentityKey` field on `ResourceTypeDef` |
| `internal/tui/views/detail.go` | `enrichmentFinding` field; `SetEnrichmentFinding()` |
| `internal/tui/views/detail_fields.go` | Render "Background Check" section |

### Implementation Order

1. `EnrichmentFinding` in `internal/resource/enrichment.go`
2. `EnricherResult` + update `EnricherFunc` signature in `internal/aws/enrichment.go`
3. Update 9 enrichers to return `EnricherResult` with findings
4. `enrichmentFindings` + `enrichmentRan` maps on Model; store from `EnrichmentCheckedMsg`
5. Clear on profile/region switch + refresh
6. `enrichmentIssueCount` + `enrichmentRanThisSession` on ResourceListModel; derive banner
7. Render banner in View()
8. `·` row marker in table_render.go
9. Pass finding to DetailModel; live update on enrichment completion
10. Render "Background Check" in detail view
11. Tests
