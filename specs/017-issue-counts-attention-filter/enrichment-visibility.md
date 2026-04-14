# Enrichment Visibility Spec (v2)

**Parent**: #196 (issue counts and attention filter)
**Problem**: Wave 2 enrichment discovers hidden issues but the resource list shows no visual indicator. The menu says `issues:N` but all rows are green.

## Previous Approach — Rejected

v1 tried to mark individual resources with `Fields["_issue"]` prefixes and propagate marks through `issueMarks` maps. This was rejected because:
- Off-page resources can still have issues the loaded page doesn't show
- `probeResources` is cleared after enrichment, losing detail data
- `Fields` is the wrong place for typed metadata (leaks into search, detail extraction)
- Cache/session reopen paths don't get marks
- Nine enrichers × mark propagation × detail rendering = high effort, still incomplete

## New Approach: List-Level Banner + Typed Attention Metadata

Two changes, both cheap, both solve the actual UX problem:

### 1. List-level enrichment banner

When the user enters a resource list where the menu badge shows enrichment-discovered issues but the loaded page has zero `IsIssueRowColor` rows, show a one-line banner below the column headers:

```
┌──────────────────────── dbi(2) ─────────────────────────┐
│ 1:DB Identifier     2:Engine     3:Version  4:Status    │
│ ⚠ 2 issues found by background checks (d for details)  │
│ docdb-docdb-dev     docdb        5.0.0      available   │
│ rds-eu-west-2-dev…  aurora-post  16.8       available   │
└─────────────────────────────────────────────────────────┘
```

The banner appears when:
- `menu.GetIssueCounts()[shortName] > 0` (enrichment found issues)
- AND the loaded page has zero `IsIssueRowColor` rows (no visible issue-status rows)
- AND enrichment has completed for this type (`menu.GetIssueKnown()[shortName]`)

The banner disappears when:
- The user loads more pages that happen to contain issue-status rows
- Or the user dismisses it (press any navigation key — it's informational, not blocking)

**Why this works**: It solves the exact problem ("badge says issues, list looks green") without touching individual rows, without propagating marks, without modifying Fields, without cache changes. The banner reads from the same issue counts the menu already has.

**Implementation**: A single `enrichmentBanner string` field on `ResourceListModel`, set when the list is created from the main menu if the condition is met. Rendered as a styled line in `View()` above the first data row.

### 2. Typed attention metadata on the model (not Fields)

For detail-view explanations, store enrichment findings as typed metadata on the root Model, not in `resource.Resource.Fields`:

```go
// On Model:
type EnrichmentFinding struct {
    Severity string // "!" (broken) or "~" (informational)
    Summary  string // human-readable: "pending maintenance: system-update, os-upgrade"
}

enrichmentFindings map[string]map[string]EnrichmentFinding // shortName → resourceID → finding
```

This map is:
- Populated by enrichers (they already receive and iterate over resources)
- Session-scoped (cleared on profile/region switch alongside `probeResources`)
- NOT persisted to disk cache (enrichment data is volatile)
- NOT stored in `resource.Resource.Fields` (no search/detail/YAML leakage)

When the user opens a detail view for a resource with a finding, the detail view reads from `m.enrichmentFindings[resourceType][resourceID]` and renders a dedicated section:

```
┌─ docdb-docdb-dev ──────────────────────────────────────┐
│ ⚠ Background Check                                     │
│ pending maintenance: system-update (New OS patch)       │
│                                                         │
│ DB Identifier:  docdb-docdb-dev                         │
│ Status:         available                               │
│ Engine:         docdb                                   │
│ ...                                                     │
└─────────────────────────────────────────────────────────┘
```

For resources without findings, this section is absent. No change to normal detail rendering.

### Tier Classification (unchanged from v1)

| Prefix | Meaning | Menu badge counted? |
|--------|---------|-------------------|
| `!` | Actively broken/degraded NOW | Yes |
| `~` | Scheduled/informational | No |

| Type | Enricher | Finding | Severity |
|------|----------|---------|----------|
| EC2 | DescribeInstanceStatus | Impaired status checks | `!` |
| EBS | DescribeVolumeStatus | I/O impaired | `!` |
| Target Groups | DescribeTargetHealth | Unhealthy targets | `!` |
| CodeBuild | BatchGetBuilds | Latest build failed | `!` |
| CodePipeline | GetPipelineState | Latest stage failed | `!` |
| SFN | ListExecutions | Latest execution failed | `!` |
| Glue | GetJobRuns | Latest job run failed | `!` |
| DynamoDB | DescribeTable | Table not ACTIVE | `!` |
| RDS/DocDB | DescribePendingMaintenanceActions | Pending maintenance | `~` |

### What This Does NOT Do

- No per-row prefixes (no `! running`, `~ available` on individual rows)
- No `Fields` mutation (no `_issue`, `_issue_detail` keys)
- No cache format changes
- No filter semantics changes (`AttentionFilter` unchanged)
- No column additions
- EC2's existing `! running`/`~ running` from the FETCHER-level enrichment stays as-is (it's not Wave 2 — it runs during the fetch itself)

### Acceptance Criteria

- [ ] When menu shows `issues:N` for a type but the loaded list page has all green rows, a banner appears explaining enrichment found issues
- [ ] Banner text includes the issue count and a hint to use detail view
- [ ] Banner disappears on navigation (not sticky/blocking)
- [ ] Detail view for a resource with an enrichment finding shows a dedicated section with severity + summary
- [ ] Detail section is absent for resources without findings
- [ ] `enrichmentFindings` map is session-scoped, cleared on profile/region switch
- [ ] `enrichmentFindings` is NOT stored in Fields, NOT persisted to cache, NOT visible in YAML/JSON dump
- [ ] RDS/DocDB pending maintenance: severity `~`, NOT counted in menu badge
- [ ] Tier A/B enrichers: severity `!`, counted in menu badge
- [ ] EC2 fetcher-level `! running`/`~ running` behavior unchanged

### Files Affected

| File | Change |
|------|--------|
| `internal/tui/app.go` | Add `enrichmentFindings` map field |
| `internal/tui/app_handlers.go` | Clear `enrichmentFindings` on profile/region switch |
| `internal/tui/app_handlers_navigate.go` | Populate findings from enricher results; pass banner state to list on creation |
| `internal/tui/app_fetchers.go` | Enrichers populate findings map |
| `internal/aws/enrichment.go` | Enrichers return findings alongside counts (change return type or use output parameter) |
| `internal/tui/views/resourcelist.go` | Add `enrichmentBanner` field, render in View() |
| `internal/tui/views/detail_fields.go` | Render enrichment finding section when present |

### Implementation Order

1. Add `EnrichmentFinding` struct and `enrichmentFindings` map to Model
2. Change enrichers to populate findings (severity + summary per resource)
3. Store findings on Model after `EnrichmentCheckedMsg`
4. Clear findings on profile/region switch
5. Set `enrichmentBanner` on ResourceListModel when creating from menu (if enrichment issues exist but page is all green)
6. Render banner in `ResourceListModel.View()`
7. Pass findings to detail view, render dedicated section
8. Tests
