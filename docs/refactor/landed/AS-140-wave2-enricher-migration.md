# AS-140 — Wave-2 Enricher Migration to Findings + AttentionDetails

**Parent**: AS-71 (Wave-1 fetcher migration, PR #335, merged)
**Sizing**: M (~1500–2000 LOC across enrichers, tests, renderer)
**Stage**: 2 — Spec & Design

---

## 1. Problem

Four Wave-2 enrichers currently write their status display phrase into
`IssueEnricherResult.FieldUpdates["status"]`, which gets merged into
`resource.Resource.Fields["status"]` via `ApplyFieldUpdates`. This was
correct before AS-71 (the fetchers had not yet migrated to emitting
`domain.Finding` directly), but now represents a layering violation:

- The enrichers already write to `IssueEnricherResult.Findings`
  (the `resource.EnrichmentFinding` map), which is converted into
  `domain.Finding` + `domain.AttentionDetail` by `DeriveFindings` /
  `applyEnrichment`.
- The `FieldUpdates["status"]` path is a second, redundant status-phrase
  channel that now only exists to inform `extractCellValue` — a 3-layer
  priority that can be collapsed to 2.

Additionally, `snapshot_cross_ref.go` contains two private helpers
(`wave1StatusAndIssues`, `computeMergedStatus`) whose sole job is
computing the merged Wave-1 + Wave-2 status phrase for the
`FieldUpdates["status"]` write. Once that write is removed, both
helpers become dead code.

---

## 2. Scope

### 2a. Enrichers to change (remove `FieldUpdates["status"]` writes)

| File | Enricher func | What to remove |
|------|--------------|----------------|
| `internal/aws/snapshot_cross_ref.go` | `EnrichSnapshotCrossRef` | `result.FieldUpdates[res.ID]` write; `computeMergedStatus` call; also delete `computeMergedStatus` and `wave1StatusAndIssues` functions |
| `internal/aws/dbi_issue_enrichment.go` | `EnrichDBIMaintenance` | `fieldUpdates` map + `fieldUpdates[key]` write; `statusByID` map (no longer needed for status overlay — `probeIDs` stays for ARN matching) |
| `internal/aws/dbc_issue_enrichment.go` | `EnrichDBCMaintenance` | `fieldUpdates` map + `fieldUpdates[key]` write; `statusByID` map |
| `internal/aws/efs_issue_enrichment.go` | `EnrichEFSMountTargets` | `fieldUpdates` map + `fieldUpdates[fsID]` write; `newStatus` / `r.Issues`-based status computation |

**Keep unchanged**: `IssueEnricherResult.FieldUpdates` field (other enrichers — WAF `rules_summary`, SES status, CFN merging — still use it).

### 2b. Renderer collapse (`internal/tui/views/table_render.go`)

`extractCellValue` status-column priority (lines ~349–370) currently reads:
1. `r.Fields["status"]` (wave-1 phrase OR wave-2 overlay from `ApplyFieldUpdates`)
2. `r.Findings[0].Phrase` (fallback)
3. `r.Fields[lifecycleKey]`

After this migration, no wave-2 enricher overlays `r.Fields["status"]` for
the four types above. The correct two-layer priority is:

1. `phraseFromFindings(r.Findings)` — aggregates ALL `domain.Finding` entries
   (wave-1 from fetcher, wave-2 from `applyEnrichment`) with a `(+N)` suffix
   when multiple findings are present. Returns `""` for a healthy resource.
2. `r.Fields[lifecycleKey]` — lifecycle steady-state ("running", "available",
   etc.) for resources with no active findings.

The `r.Fields["status"]` intermediate read is removed because:
- Migrated wave-1 fetchers (AS-71) already emit `domain.Finding`; their
  phrases live in `r.Findings`, not just `r.Fields["status"]`.
- `DeriveFindings` / `DeriveWave1Only` is called on every resource before
  it is rendered, ensuring `r.Findings` is always populated.
- SES and WAF enrichers that still write `FieldUpdates["status"]` /
  `FieldUpdates["rules_summary"]` also write to `IssueEnricherResult.Findings`,
  so their wave-2 finding reaches `r.Findings` via `applyEnrichment`.

### 2c. `phraseFromFindings` — shared location

`phraseFromFindings([]domain.Finding) string` currently lives as a private
function in `internal/aws/rds.go`. `table_render.go` is in `internal/tui/views`
and cannot call it directly.

**Decision**: inline the three-line helper as a package-private function in
`table_render.go` under the name `phraseFromFindings`. Do NOT export it or
move it to a shared package in this PR — that refactor is orthogonal and can
be tracked separately.

```go
func phraseFromFindings(findings []domain.Finding) string {
    if len(findings) == 0 {
        return ""
    }
    if len(findings) == 1 {
        return findings[0].Phrase
    }
    return fmt.Sprintf("%s (+%d)", findings[0].Phrase, len(findings)-1)
}
```

`internal/aws/rds.go` retains its own private copy unchanged.

---

## 3. Invariants preserved

- **Idempotency**: enrichers still read from `IssueEnricherResult.Findings`
  (EnrichmentFinding), not previously-merged fields. The `FieldUpdates`
  write was the only site that read `r.Fields["status"]` or `r.Issues` —
  both removed. DeriveFindings is deterministic and re-derives on each run.
- **IssueCount**: unchanged. `dbc` increments by 1 for each overdue finding
  (severity `!`); `dbi` always returns 0 (severity `~`); `efs` returns
  `len(findings)`; `snapshot_cross_ref` always returns 0 (severity `!` for
  orphan/retention but IssueCount is not modified).
  **Wait** — `snapshot_cross_ref` returns `IssueEnricherResult{}` with no
  IssueCount field; `EnrichSnapshotCrossRef` sets `result.Findings` and
  `result.FieldUpdates` but leaves `result.IssueCount = 0`. Unchanged.
- **Stacked wave-1 + wave-2 display**: the `(+N)` suffix is now computed at
  render time by `phraseFromFindings(r.Findings)` rather than eagerly during
  enrichment. `r.Findings` contains both wave-1 and wave-2 entries after
  `applyEnrichment` runs, so the suffix is correct.
- **Attention section**: `r.AttentionDetails` is populated by `DeriveFindings`
  from `IssueEnricherResult.Findings` (EnrichmentFinding.Rows). This path is
  unchanged — removing `FieldUpdates` writes has zero effect on it.

---

## 4. Out of scope

- `ses_issue_enrichment.go` — still writes `FieldUpdates["status"]` AND
  `Findings`. Migrating SES is a separate follow-up.
- Exporting `phraseFromFindings` or deduplicating between packages.
- `waf_issue_enrichment.go` — writes `FieldUpdates["rules_summary"]`
  (non-status key); unaffected.
- `computeMergedStatus` test coverage — the function is deleted, so tests
  testing it directly must be removed or repurposed.

---

## 5. Files to create/modify

### Coder

| File | Action | Key change |
|------|--------|-----------|
| `internal/aws/snapshot_cross_ref.go` | Modify | Remove `result.FieldUpdates` map init and all writes; remove `FieldUpdates` from result literal; delete `computeMergedStatus`; delete `wave1StatusAndIssues`; update doc comment |
| `internal/aws/dbi_issue_enrichment.go` | Modify | Remove `fieldUpdates` map; remove `statusByID` map; remove `existing`/`newStatus` locals; remove `fieldUpdates` from return; update doc comment |
| `internal/aws/dbc_issue_enrichment.go` | Modify | Remove `fieldUpdates` map; remove `statusByID` map; remove `existing`/`newStatus` locals; remove `fieldUpdates` from return; update doc comment |
| `internal/aws/efs_issue_enrichment.go` | Modify | Remove `fieldUpdates` map; remove `newStatus` / `r.Issues` logic; remove `fieldUpdates` from return; update doc comment |
| `internal/tui/views/table_render.go` | Modify | Add `phraseFromFindings` helper; update `extractCellValue` status priority from 3-layer to 2-layer; update inline comment |

### QA

| File | Action | Key change |
|------|--------|-----------|
| `tests/unit/aws_snapshot_cross_ref_test.go` | Modify | Assert `FieldUpdates` is nil or empty for orphan/past-retention findings; remove any test of `computeMergedStatus` directly; assert `Findings` still populated |
| `tests/unit/aws_dbi_issue_enrichment_test.go` | Modify | Assert `FieldUpdates` is nil or empty; existing `Findings` + `Summary` assertions stay |
| `tests/unit/aws_dbc_issue_enrichment_test.go` | Modify | Same pattern as dbi |
| `tests/unit/aws_efs_issue_enrichment_test.go` | Modify | Assert `FieldUpdates` is nil or empty; existing `Findings` assertions stay |
| `tests/unit/enrichment_rds_findings_test.go` | Modify (if needed) | Update any assertions about `FieldUpdates["status"]` for dbi/dbc stacked cases |
| `tests/unit/qa_enrichment_detail_test.go` | Modify (if needed) | Update/add test for `extractCellValue` with wave-1+wave-2 stacked resources |

---

## 6. Acceptance criteria

- `FieldUpdates["status"]` is empty (nil or len=0) for all four enrichers.
- `Findings` (EnrichmentFinding) populated exactly as before for each enricher.
- `r.AttentionDetails` populated via `DeriveFindings` path (no change required).
- `extractCellValue` returns the wave-2 phrase for a healthy resource
  (no wave-1 findings), via `phraseFromFindings(r.Findings)`.
- `extractCellValue` returns `"stopped (+1)"` for a resource with one
  wave-1 finding "stopped" and one wave-2 finding stacked on it.
- `wave1StatusAndIssues` and `computeMergedStatus` are deleted.
- `make ready-to-push` green; AS-132 regression tests pass (or are updated
  to assert via Findings rather than FieldUpdates).

---

## 7. Key code pointers for Coder

- `IssueEnricherResult` struct: `internal/aws/issue_enrichment.go:135`
- `applyEnrichment`: `internal/tui/app_enrich_fold.go:39`
- `DeriveFindings`: `internal/semantics/attention/derive.go:39`
- Current `extractCellValue` status block: `internal/tui/views/table_render.go:330–370`
- `phraseFromFindings` (private, aws package): `internal/aws/rds.go:205`
- `ApplyFieldUpdates` (still needed by other enrichers): `internal/tui/views/resourcelist.go:547`
