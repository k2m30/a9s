# Enrichment Visibility Spec

**Parent**: #196 (issue counts and attention filter)
**Problem**: Wave 2 enrichment discovers hidden issues but the resource list shows no visual indicator. The menu says `issues:N` but all rows are green. This is confusing and misleading.

## Design

Extend the existing EC2 `! running` / `~ running` prefix pattern to all enriched resource types. Enrichers mark affected resources with a prefix field. The table renderer displays the prefix. Issue counting uses the marks. No new columns, no new UI concepts.

### Severity Tiers

| Prefix | Meaning | Row Color | Badge counted? |
|--------|---------|-----------|---------------|
| `!` | Actively broken/degraded NOW | ColStopped (red) | Yes |
| `~` | Needs attention, not urgent | ColPending (yellow) | No — detail-view only |

### Per-Type Mapping

| Type | Short | Enricher API | What it finds | Prefix | On Column | Tier |
|------|-------|-------------|---------------|--------|-----------|------|
| EC2 | ec2 | DescribeInstanceStatus | Impaired system/instance checks | `!` | State cell (existing) | A |
| EBS | ebs | DescribeVolumeStatus | I/O impaired | `!` | Status cell | A |
| Target Groups | tg | DescribeTargetHealth | Unhealthy targets | `!` | Name cell | A |
| CodeBuild | cb | BatchGetBuilds | Latest build failed | `!` | Name cell | B |
| CodePipeline | pipe | GetPipelineState | Latest stage failed | `!` | Name cell | B |
| Step Functions | sfn | ListExecutions(max:1) | Latest execution failed | `!` | Name cell | B |
| Glue | glue | GetJobRuns(max:1) | Latest job run failed | `!` | Name cell | B |
| RDS/DocDB | dbi | DescribePendingMaintenanceActions | Pending maintenance | `~` | Status cell | C |
| DynamoDB | ddb | DescribeTable | Table not ACTIVE | `!` | Name cell | A |

**Tier A** (actively broken): counted in menu badge `issues:N`
**Tier B** (execution failures): counted in menu badge `issues:N`
**Tier C** (informational/scheduled): NOT counted in menu badge — prefix visible in list only

### How It Works

#### 1. Enrichers mark resources (not just count)

Change `EnricherFunc` to mutate the resource slice:

```go
type EnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (issueCount int, truncated bool, err error)
```

The enricher sets two fields on affected resources:
- `resources[i].Fields["_issue"] = "!"` or `"~"` — the prefix for the list view
- `resources[i].Fields["_issue_detail"]` — human-readable explanation for the detail view

Example for RDS pending maintenance:
```
Fields["_issue"] = "~"
Fields["_issue_detail"] = "pending maintenance: system-update (New OS patch), db-upgrade (aurora-postgresql 16.11, auto-apply 2026-05-07)"
```

Example for Target Groups:
```
Fields["_issue"] = "!"
Fields["_issue_detail"] = "unhealthy targets: 2/5 (i-abc123: unhealthy, i-def456: draining)"
```

Example for CodeBuild:
```
Fields["_issue"] = "!"
Fields["_issue_detail"] = "latest build FAILED: deploy-api-prod#142 (2026-04-13)"
```

The returned `issueCount` counts only Tier A + B marks (the `!` ones). Tier C marks (`~`) are set on resources but not counted.

Note on mutability: `[]resource.Resource` is a slice of structs, not pointers. However, mutations to `resources[i].Fields` via index ARE visible to the caller because the Fields map is a shared reference. Use Fields only — do not mutate Status or other struct fields.

#### 2. Probe retains resources; enricher marks them; marks flow back

Current flow:
```
Wave 1 probe → AvailabilityCheckedMsg{Resources: [...]} → m.probeResources["ec2"] = [...]
Wave 2 enricher → count issues → EnrichmentCheckedMsg{Issues: N}
```

New flow:
```
Wave 1 probe → AvailabilityCheckedMsg{Resources: [...]} → m.probeResources["ec2"] = [...]
Wave 2 enricher → mark resources[i].Fields["_issue"] = "!" → count marks → EnrichmentCheckedMsg{Issues: N}
(probeResources now contains marked resources)
```

The marks live on the retained resources in `m.probeResources`. When the user enters a resource list, the marks need to be visible. Two options:

**Option A**: The list fetcher re-fetches and re-enriches (expensive, duplicates work).
**Option B**: The table renderer checks `m.probeResources` at render time (coupling between view and app state).
**Option C**: When the enricher marks resources, propagate the marks to the main menu as a per-type map. When the user opens a resource list and the fetcher returns resources, merge marks from the stored map into the fresh resources by ID match.

Option C is cleanest:

```go
// On Model:
issueMarks map[string]map[string]string  // shortName → resourceID → prefix ("!" or "~")
```

After enrichment: `m.issueMarks["dbi"]["docdb-docdb-dev"] = "~"`

When `ResourcesLoadedMsg` arrives for the list:
```go
if marks, ok := m.issueMarks[resourceType]; ok {
    for i, r := range resources {
        if prefix, ok := marks[r.ID]; ok {
            resources[i].Fields["_issue"] = prefix
        }
    }
}
```

#### 3. Detail view shows the explanation

When the user opens a detail view (`d` or `Enter`) for a resource with `Fields["_issue_detail"]` set, the detail view renders it as a visible field. The existing detail view already renders all `Fields` entries — `_issue_detail` will appear as a field with key `issue_detail` (strip the underscore prefix for display).

The field should render near the top of the detail view (after ID/Name/Status) so the user sees it immediately. It uses the issue color: ColStopped for `!` marks, ColPending for `~` marks.

For YAML and JSON views: `_issue_detail` appears in the Fields section alongside other fields. No special rendering needed — the content is already human-readable.

User flow:
1. Main menu shows `DB Instances (2) issues:2` → user enters
2. List shows `~ available` on both rows (yellow) → user sees something is flagged
3. User presses `d` on `docdb-docdb-dev` → detail view shows:
   ```
   DB Identifier:  docdb-docdb-dev
   Status:         available
   Issue:          pending maintenance: system-update (New OS patch)
   Engine:         docdb
   ...
   ```
4. User now understands WHY the `~` prefix was there

For target groups (Tier A, no detail enrichment today):
1. List shows `! api-prod-tg` (red) → user sees something is broken
2. Detail shows: `Issue: unhealthy targets: 2/5 (i-abc123: unhealthy, i-def456: draining)`

#### 4. Table renderer displays the prefix

Generalize the existing EC2-specific check at `table_render.go:197-204`:

```go
// Current (EC2-only):
if (c.key == "state" || c.path == "State.Name") && val == "running" {
    sysStatus := r.Fields["system_status"]
    instStatus := r.Fields["instance_status"]
    if sysStatus == "impaired" || instStatus == "impaired" {
        val = "! " + val
    } else if sysStatus == "initializing" || instStatus == "initializing" {
        val = "~ " + val
    }
}

// New (generic, runs AFTER the EC2-specific check):
if prefix := r.Fields["_issue"]; prefix != "" {
    // Apply prefix to the target column:
    // - Status/State column if it exists in the configured columns
    // - Otherwise the first column (Name)
    if isTargetColumn(c, r) {
        val = prefix + " " + val
    }
}
```

The `isTargetColumn` logic:
1. If the column is the Status/State column (key contains "status" or "state") → yes
2. If no Status column was found in this render pass and this is the first column → yes
3. Otherwise → no

For EC2: the existing specific check at line 197 still runs first (it handles `!` vs `~` based on system vs instance status). The generic `_issue` field check is a fallback for types without specific logic.

#### 4. Row color override

When `r.Fields["_issue"] == "!"`, the row should render in ColStopped (red) regardless of the Status value. When `"~"`, render in ColPending (yellow).

In the row color resolution (`RowColorStyle` or the render loop):
```go
if issue := r.Fields["_issue"]; issue == "!" {
    rowStyle = lipgloss.NewStyle().Foreground(ColStopped)
} else if issue == "~" {
    rowStyle = lipgloss.NewStyle().Foreground(ColPending)
}
```

This overrides the normal Status-based coloring for marked resources.

#### 5. Issue counting uses marks

The `IsIssueRowColor(status)` predicate still works for Wave 1 (Status-based counting). For Wave 2, the enricher returns a count of `!`-marked resources. The `~`-marked resources are not counted.

In `probeResourceAvailability()` (Wave 1): count by `IsIssueRowColor(r.Status)` — unchanged.
In enrichers (Wave 2): count by `resources[i].Fields["_issue"] == "!"` — only Tier A+B.
In `ResourceListModel.issueCount`: also check `r.Fields["_issue"] == "!"` in addition to `IsIssueRowColor(r.Status)`.

### Acceptance Criteria

- [ ] RDS/DocDB instances with pending maintenance show `~ available` (yellow) in the list — NOT counted in menu badge
- [ ] EC2 instances with impaired status checks show `! running` (red) in the list AND counted in menu badge (existing behavior preserved)
- [ ] EBS volumes with impaired I/O show `! in-use` (red) in the list AND counted in menu badge
- [ ] Target groups with unhealthy targets show `! tg-name` (red, on Name column) AND counted in menu badge
- [ ] CodeBuild projects with failed latest build show `! project-name` (red, on Name column) AND counted in menu badge
- [ ] CodePipeline with failed stage shows `! pipeline-name` (red, on Name column) AND counted in menu badge
- [ ] SFN with failed latest execution shows `! sfn-name` (red, on Name column) AND counted in menu badge
- [ ] Glue with failed latest run shows `! job-name` (red, on Name column) AND counted in menu badge
- [ ] DynamoDB tables not ACTIVE show `! table-name` (red, on Name column) AND counted in menu badge
- [ ] Row color overrides Status-based color when `_issue` prefix is set
- [ ] `ctrl+z` on resource list shows `!`-marked rows (they have issue colors)
- [ ] `~`-marked rows are visible under `ctrl+z` (ColPending is non-dim) but not counted as issues
- [ ] Menu badge `issues:N` counts ONLY `!`-marked resources (Tier A+B), not `~`-marked (Tier C)
- [ ] NO_COLOR mode: prefix is ASCII `!`/`~`, works without color
- [ ] Demo mode: no enrichment runs, no prefixes (no real AWS)
- [ ] Existing EC2 `! running`/`~ running` behavior preserved — the generic system is additive, not a replacement
- [ ] Detail view shows `_issue_detail` field with human-readable explanation for marked resources
- [ ] Detail view renders the issue field near the top, colored by severity (red for `!`, yellow for `~`)
- [ ] YAML/JSON views include `_issue_detail` in the Fields section

### Files Affected

| File | Change |
|------|--------|
| `internal/aws/enrichment.go` | Enrichers set `Fields["_issue"]` on affected resources |
| `internal/tui/app.go` | Add `issueMarks map[string]map[string]string` field |
| `internal/tui/app_fetchers.go` | After enrichment, extract marks from `probeResources` |
| `internal/tui/app_handlers_navigate.go` | Merge marks into resources on `ResourcesLoadedMsg` |
| `internal/tui/views/table_render.go` | Generic `_issue` prefix check + row color override |
| `internal/tui/views/resourcelist_helpers.go` | `issueCount` also checks `Fields["_issue"]` |
| `internal/tui/views/detail_fields.go` | Render `_issue_detail` field near top with severity color |
| `internal/tui/styles/styles.go` | No change — uses existing ColStopped/ColPending |

### What This Does NOT Change

- `IsIssueRowColor` predicate — unchanged, still used for Wave 1 Status-based counting
- `issueStatusSet` — unchanged
- `AttentionFilter` toggle behavior — unchanged
- Cache format — unchanged (issue counts in cache are numeric, not per-resource marks)
- `EnricherFunc` return signature — unchanged (count + truncated + error)
- Main menu badge logic — unchanged (uses counts from enrichers)
- Demo fixtures — no change needed (demo skips Wave 2)

### Implementation Order

1. Add `issueMarks` map to `Model`, populate from enricher results
2. Merge marks into resources when `ResourcesLoadedMsg` arrives
3. Generalize table renderer to check `Fields["_issue"]` prefix
4. Add row color override for `_issue`-marked resources
5. Update `issueCount` on ResourceListModel to include `_issue` marks
6. Update enrichers to set `Fields["_issue"]` on affected resources (9 enrichers)
7. RDS/DocDB enricher: set `"~"` (not counted in badge), others set `"!"` (counted)
8. Tests for each enricher's marking behavior
9. Tests for renderer prefix display
10. Tests for row color override
