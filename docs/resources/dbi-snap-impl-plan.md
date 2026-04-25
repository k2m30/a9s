---
shortName: dbi-snap
spec: docs/resources/dbi-snap.md
generatedBy: a9s-implement-resource skill
date: 2026-04-25
---

# dbi-snap — Implementation Plan

Working contract for the implementation of `dbi-snap` against `docs/resources/dbi-snap.md`. Tests assert against §1; fixtures live in §2; coder works against §3 (gap analysis) and §4 (architectural extension).

## §0 Spec snapshot

- **Identity**: `DescribeDBSnapshots` only — no per-snapshot describe call.
- **Wave 1 signals (6)**:
  1. `Status == "available"` → Healthy
  2. `Status == "creating"` → Warning (S4 = `creating: <pct>%`)
  3. `Status == "failed"` → Broken (S4 = `failed`)
  4. `Status` matches `incompatible-*` → Broken (S4 = the literal AWS keyword, e.g. `incompatible-restore`)
  5. `Encrypted == false` → Warning (S4 = `unencrypted`)
  6. cross-ref `dbi`: source DB no longer present → Warning (S4 = `orphan: source DB deleted`)
  7. cross-ref `dbi`: automated snapshot older than parent `BackupRetentionPeriod` → Warning (S4 = `automated, <N>d past retention`)
- **Wave 2**: None (per spec §3.2).
- **Wave 3 (out of scope)**: per-snapshot `DescribeDBSnapshotAttributes` (public-snapshot detection).
- **Related panel**: `dbi` (1, count yes), `kms` (1, count yes), `dbc` (1, count yes), `backup` (count yes — via `ListRecoveryPointsByResource`), `ct-events` (count unknown).

## §0.1 §4 precedence ladder (pinned)

The spec §4 doesn't restate severity precedence, so this plan pins it. Severity-first, then table order within severity:

1. **Broken**:
   1. `failed`
   2. `incompatible-<keyword>` (keyword preserved verbatim, e.g. `incompatible-restore`, `incompatible-parameters`)
2. **Warning**:
   1. `creating: <pct>%` (transitional — only present when `Status == creating`; rarely coexists with the others)
   2. `unencrypted`
   3. `orphan: source DB deleted`
   4. `automated, <N>d past retention`

For the multi-W1 fixture, the `(+N)` suffix is appended after the top phrase per universal rule 7. Wave-2 = None, so U7b/U7c/U7d/U8 cases collapse to N/A and are recorded as such in §1.

## §1 Pseudocode test spec

One pseudocode case per §3 signal plus the universal coverage matrix rows.

### §1.1 Per-signal cases

```
TEST: healthy_available_blank_S4
GIVEN: DBSnapshot{Status:"available", Encrypted:true, parent dbi present in cache}
WHEN:  list is fetched
THEN:
  - row color = green
  - Resource.Status = ""             (S4 blank)
  - Resource.Issues = [] / nil
  - no `!` / `~` glyph
  - Menu issues:N does NOT bump

TEST: warning_creating_carries_pct
GIVEN: DBSnapshot{Status:"creating", PercentProgress:42, Encrypted:true}
WHEN:  list is fetched
THEN:
  - row color = yellow
  - Resource.Status = "creating: 42%"
  - Resource.Issues = ["creating: 42%"]
  - no glyph (non-green row)

TEST: broken_failed
GIVEN: DBSnapshot{Status:"failed", Encrypted:true}
WHEN:  list is fetched
THEN:
  - row color = red
  - Resource.Status = "failed"
  - Resource.Issues = ["failed"]

TEST: broken_incompatible_keyword_preserved
GIVEN: DBSnapshot{Status:"incompatible-restore", Encrypted:true}
WHEN:  list is fetched
THEN:
  - row color = red
  - Resource.Status = "incompatible-restore"
  - Resource.Issues = ["incompatible-restore"]

TEST: warning_unencrypted
GIVEN: DBSnapshot{Status:"available", Encrypted:false}
WHEN:  list is fetched
THEN:
  - row color = yellow
  - Resource.Status = "unencrypted"
  - Resource.Issues = ["unencrypted"]

TEST: warning_orphan_source_dbi_deleted
GIVEN: DBSnapshot{Status:"available", DBInstanceIdentifier:"deleted-db",
        Encrypted:true},  dbi cache loaded but does NOT contain "deleted-db"
WHEN:  list is fetched AND issue enricher (with dbi cache) runs
THEN:
  - row color = yellow
  - Resource.Status (after enrichment) = "orphan: source DB deleted"
  - Resource.Issues = ["orphan: source DB deleted"]
  - no glyph

TEST: warning_automated_past_retention
GIVEN: parent dbi.BackupRetentionPeriod = 7,
        DBSnapshot{Status:"available", Encrypted:true,
                   SnapshotType:"automated",
                   SnapshotCreateTime: now - 14 days,
                   DBInstanceIdentifier: <parent>}
WHEN:  list is fetched AND enricher runs
THEN:
  - row color = yellow
  - Resource.Status (after enrichment) = "automated, 7d past retention"
  - Resource.Issues = ["automated, 7d past retention"]

TEST: skip_orphan_when_dbi_cache_not_loaded
GIVEN: DBSnapshot{Status:"available", DBInstanceIdentifier:"x"},
        dbi cache NOT in ResourceCache
WHEN:  enricher runs
THEN:
  - no orphan finding emitted (rule skipped per spec §3.1)
  - Resource.Status unchanged from fetcher value

TEST: skip_automated_retention_when_parent_dbi_missing
GIVEN: DBSnapshot{Status:"available", SnapshotType:"automated",
                   DBInstanceIdentifier:"missing-from-dbi"},
        dbi cache loaded but parent NOT present
WHEN:  enricher runs
THEN:
  - parent missing => orphan finding fires (covered by orphan test)
  - past-retention rule does NOT also fire (skipped per spec §3.1
    "Skip the rule when the parent DB is not in the loaded sibling list")
  - Resource.Status = "orphan: source DB deleted"  (orphan wins; no double-emit)

TEST: arn_field_populated_for_backup_pivot
GIVEN: DBSnapshot{DBSnapshotArn:"arn:aws:rds:us-east-1:123:snapshot:rds:foo"}
WHEN:  list is fetched
THEN:  Resource.Fields["arn"] == "arn:aws:rds:us-east-1:123:snapshot:rds:foo"
       (consumed by checkDBISnapBackup)
```

### §1.2 Multi-finding cases (rule 7)

```
TEST: multi_w1_unencrypted_plus_orphan_suffix     (covers U7a)
GIVEN: DBSnapshot{Status:"available", Encrypted:false,
                   DBInstanceIdentifier:"deleted-db"},
        dbi cache loaded, parent NOT present
WHEN:  list fetched + enricher runs
THEN:
  - Resource.Status = "unencrypted (+1)"  (top precedence: unencrypted < orphan)
  - Resource.Issues = ["unencrypted", "orphan: source DB deleted"]
  - row color = yellow

TEST: multi_w1_unencrypted_plus_past_retention    (additional U7a coverage)
GIVEN: parent dbi.BackupRetentionPeriod = 7,
        DBSnapshot{Status:"available", Encrypted:false,
                   SnapshotType:"automated",
                   SnapshotCreateTime: now - 21 days,
                   DBInstanceIdentifier: <parent>}
WHEN:  list fetched + enricher runs
THEN:
  - Resource.Status = "unencrypted (+1)"  (unencrypted < past-retention)
  - Resource.Issues = ["unencrypted", "automated, 14d past retention"]

TEST: detail_view_surfaces_every_wave1_phrase     (covers U7e)
GIVEN: the multi_w1_unencrypted_plus_orphan fixture
WHEN:  OpenDetailResource on it
THEN:
  - rendered detail contains "Unencrypted"
  - rendered detail contains "Orphan: source DB deleted"
  - bare "unencrypted" appears (without the (+1) suffix; detail enumerates)

TEST: fetcher_populates_resource_issues           (covers U7f)
GIVEN: each §3.1 fixture
WHEN:  FetchDBISnapshotsPage runs
THEN:  got.Issues deep-equals expected slice in precedence order
       (Healthy → empty; single warning → single phrase; multi → top first)

# U7b/U7c/U7d/U8: N/A — Wave 2 = None for dbi-snap, no Wave-1+Wave-2 stack possible.
```

### §1.3 Universal coverage matrix mapping

| ID | Covered by test | Notes |
|---|---|---|
| U1 healthy blank S4 | healthy_available_blank_S4 | |
| U2 §4 phrase per signal | warning_*, broken_* | |
| U3 `~` on Healthy+~ | N/A — no Wave 2 `~` signal | |
| U4 `!` on Healthy+! | N/A — no Wave 2 `!` signal | |
| U5 no glyph on non-green | every warning_* / broken_* test asserts | |
| U6 S1 badge counts `!` instances | N/A — no `!` signals → expected N=0 | |
| U7a multi-W1 (+N) | multi_w1_unencrypted_plus_orphan | |
| U7b W1+W2 stack | N/A — Wave 2 = None | |
| U7c S5 every Wave-2 finding | N/A — Wave 2 = None | |
| U7d `!` beats `~` | N/A — no Wave 2 | |
| U7e S5 every Wave-1 phrase | detail_view_surfaces_every_wave1_phrase | |
| U7f Resource.Issues populated | fetcher_populates_resource_issues + enricher_populates_resource_issues | |
| U8 Broken > Warning | covered by single-signal severity tests + a multi-test combining Encrypted=false + Status=failed asserts `failed` wins (no suffix because Encrypted=false is suppressed when Status≠available) | |
| U9 related pivot counts | scenario test in phase 8 | |
| U10 no jargon columns | scenario test in phase 8 | |
| U11 Summary ≠ Rows | N/A — Wave 2 = None | |
| U12 partial AWS failure → FlashMsg | partial-failure scenario test | inject DescribeDBSnapshots error |
| U13 every SDK call wrapped | static grep audit | |
| U14 ARN-as-ID guard | N/A — fetcher emits `ID = DBSnapshotIdentifier` (bare); checker reads ARN from `Fields["arn"]` per Rule E7 | |
| U15 strict fakes reject non-ARN | enforced by demo fake | |
| U16 AssertNoEnrichmentErrors | scenario test calls it | |

### §1.4 Severity edge case

```
TEST: severity_broken_beats_warning
GIVEN: DBSnapshot{Status:"failed", Encrypted:false}
WHEN:  list fetched
THEN:
  - Resource.Status = "failed"          (Broken severity wins; no suffix because
                                         Encrypted=false signal is suppressed
                                         when Status is non-available — the
                                         snapshot is failed-end-state, encryption
                                         flag is informational at that point)
  - Resource.Issues = ["failed"]
  - row color = red
```

## §2 Fixtures

Single-source fixture file: `internal/demo/fixtures/dbi-snap.go` — exports a `DBISnapFixtures` struct + `NewDBISnapFixtures()` constructor + per-fixture exported ID/ARN consts. The existing inline `buildDBISnapshots()` block in `internal/demo/fixtures/rds.go` (lines 446–~600) is folded into this new file and removed from `rds.go` (the RDS fake's reference is updated to call into the new package symbol).

Sibling cross-references required for graph-connected non-zero pivots:
- `kms.go` — graph-root snapshot's `KmsKeyId` must match an existing `KMSFixtures` key (already covered by `dbiKMSKeyID`).
- `dbi.go` — parent DB instance must exist (already covered by `ProdDbiID`).
- `backup.go` — must contain a Backup Recovery Point pointing to the graph-root snapshot's ARN. **NEW SIBLING UPDATE** required.
- `ct_events.go` — must contain CloudTrail entries with `ResourceName = <graph-root snapshot identifier>`. **NEW SIBLING UPDATE** required.

### §2.1 Fixture list

| ID const | Identifier | State | Purpose |
|---|---|---|---|
| `ProdDBISnapID` (graph-root) | `rds:prod-dbi-1-2026-04-15` | Healthy + Encrypted=true + automated, parent `ProdDbiID` present, `KmsKeyId=dbiKMSKeyID` | Graph-root for §9.3 — drives `dbi`, `kms`, `backup` pivots non-zero. `dbc` pivot is intentionally absent (Aurora cluster snapshots live in `dbc-snap`, not `dbi-snap`). |
| `WarnDBISnapCreatingID` | `dev-feature-branch-snap` | Status=creating, PercentProgress=42, Encrypted=true | Covers Wave-1 `creating: 42%` signal |
| `BrokenDBISnapFailedID` | `prod-dbi-1-failed-snap` | Status=failed, Encrypted=true, parent ProdDbiID | Covers Broken `failed` |
| `BrokenDBISnapIncompatibleID` | `legacy-mysql-snap-incompatible` | Status=incompatible-restore, Encrypted=true | Covers Broken `incompatible-restore` |
| `WarnDBISnapUnencryptedID` | `unenc-pre-migration-snap` | Status=available, Encrypted=false, parent ProdDbiID | Covers Warning `unencrypted` |
| `WarnDBISnapOrphanID` | `orphan-deleted-db-snap` | Status=available, Encrypted=true, DBInstanceIdentifier="deleted-legacy-db" (NOT in dbi list) | Covers Warning `orphan: source DB deleted` |
| `WarnDBISnapPastRetentionID` | `rds:past-retention-db-2026-03-15` | Status=available, automated, Encrypted=true, parent set to a NEW `dbi.go` fixture with `BackupRetentionPeriod=7`, SnapshotCreateTime = NOW - 30 days | Covers Warning `automated, 23d past retention` |
| `MultiW1DBISnapID` (multi-W1) | `multi-orphan-unenc-snap` | Status=available, Encrypted=false, DBInstanceIdentifier="deleted-legacy-db" | U7a/U7e — `unencrypted (+1)` |
| `BackupCoveredDBISnapID` | `awsbackup:job-deadbeef-snap` | Status=available, Encrypted=true, parent ProdDbiID | Realism: AWS Backup-created identifier prefix; backup pivot already drives non-zero on graph-root |
| `SeverityBrokenWarnDBISnapID` | `failed-with-unenc-snap` | Status=failed, Encrypted=false | Covers severity_broken_beats_warning (U8) |

Adversarial fixtures (NOT in this file — stay inline in `tests/unit/aws_rds_snap_test.go`):
- nil `DBSnapshotIdentifier`
- nil `Status`
- malformed ARN
- nil `SnapshotCreateTime` (past-retention rule must skip cleanly)

### §2.2 Graph-root structural exemption (§9.3 ≥50% Count ≥ 2)

`ProdDBISnapID` is the graph-root. The pivots resolve as follows:

- `dbi` — Count = 1 (a snapshot has exactly one source instance; capped at 1 by AWS).
- `kms` — Count = 1 (a snapshot has exactly one encryption key; capped at 1).
- `backup` — Count = 1 on the graph-root (one recovery point pointing at `ProdDBISnapARN`).
- `ct-events` — count "unknown" (windowed; exempt per universal rule).

**Structural exemption**: `dbi-snap` pivots are 1:1 by AWS data model. The dbc
pivot is intentionally NOT registered (Aurora cluster snapshots live in
`dbc-snap`, never in `dbi-snap` — real AWS rejects `CreateDBSnapshot` on
Aurora cluster members). The universal §9.3 rule "≥50% Count ≥ 2" is
unsatisfiable for this resource type and is documented as an exemption in
the phase 9.3 report. The `BackupCoveredDBISnapID` fixture independently
exercises Count ≥ 2 on the `backup` pivot via two recovery points so that
code path is still tested.

### §2.3 Sibling-fixture updates required (graph plan)

These are scoped for phase 6a in addition to writing `dbi-snap.go`:

1. `dbi.go` — add `ProdDbiRetentionParentID` fixture (BackupRetentionPeriod=7) so the past-retention test has a parent. Stable ID + ARN constants.
2. `backup.go` — add 1 `RecoveryPoint` entry with `ResourceArn` matching `ProdDBISnapID`'s DBSnapshotArn (graph-root pivot ≥ 1) and 2 entries for `BackupCoveredDBISnapARN` (independent ≥ 2 coverage).
3. `ct_events.go` — add 3 CloudTrail event entries with `ResourceName = ProdDBISnapID` (event names: `CreateDBSnapshot`, `ModifyDBSnapshotAttribute`, `CopyDBSnapshot`).

## §3 Contract surface gap analysis

### §3.1 Fetcher (`internal/aws/dbi_snap.go`)

| Current | Required | Action |
|---|---|---|
| Status = raw AWS keyword | Status = §4 phrase per precedence; healthy = `""` | Replace with `computeDBISnapStatusAndIssues(snap)` |
| `Resource.Issues` not populated | Populated in §4 precedence order | Add ordered slice |
| `Fields["arn"]` not populated | Populated from `DBSnapshotArn` | Add field |
| `DescribeDBSnapshots` not throttle-wrapped | wrapped in `RetryOnThrottle` | E1/U13 fix |
| Errors propagate via raw `fmt.Errorf` | Use `AggregateFailures` if multiple snapshots fail individually | Page-level errors are already top-level returns; per-row failures don't apply (the SDK returns the whole page or errors) |
| `RegisterPaginated` returns top-level err verbatim | unchanged | Keep |

### §3.2 Related panel (`internal/aws/rds_snap_related.go`)

| Current | Required | Action |
|---|---|---|
| 4 pivots: dbi, kms, dbc, backup | + `ct-events` (universal pivot) | Register `ct-events` |
| `checkDBISnapBackup` reads `Fields["arn"]` then falls back to RawStruct | Same; verify fetcher populates `Fields["arn"]` | Fetcher fix above closes the gap |
| `checkDBISnapBackup` calls `ListRecoveryPointsByResource` directly with `RetryOnThrottle` | OK per E1 | No change |
| All checkers wrap any per-item AWS calls | OK | No change |

### §3.3 Issue enricher (`internal/aws/rds_snap_issue_enrichment.go`)

| Current | Required | Action |
|---|---|---|
| `NoOpIssueEnricher` | Real cross-ref enricher (orphan + past-retention) | Replace |

The new enricher:
- reads `cache["dbi"].Resources` (zero API calls — pure cross-ref)
- per snapshot: if `DBInstanceIdentifier` is non-empty AND not in dbi list → emit orphan finding
- per snapshot: if parent dbi present + SnapshotType=="automated" + age > parent's `BackupRetentionPeriod` → emit past-retention finding
- skips the rule when dbi cache entry is missing or empty (per spec "skip when dbi list not loaded")
- emits `FieldUpdates[id]["status"]` with the merged §4 phrase (using `BumpFindingSuffix` if a fetcher Status already exists)
- emits `IssueAppends[id]` with phrases to append to `Resource.Issues`

### §3.4 Detail enricher

Not needed (spec §2 has no extra-API field requirements beyond what `DescribeDBSnapshots` returns).

### §3.5 View config (`internal/config/defaults.go` + `.a9s/views/dbi-snap.yaml`)

| Current | Required | Action |
|---|---|---|
| `Status` column reads `path: Status` (raw AWS field) | Reads `path: status` (computed Fields key) | Change path |
| `Encrypted` column (true/false) | DELETE — folded into Status as `unencrypted` per §4 | Remove from defaults.go entry; regenerate yaml |
| No `dbi-snap` entry in `defaultViews` | Add full entry | New code |
| Identity columns: Snapshot ID, DB Instance, Engine, Type, Created | Keep (universal rule allows identity/metadata columns) | No change |

Final column set (in order): Snapshot ID, DB Instance, **Status** (computed), Engine, Type, Created.

### §3.6 Interfaces (`internal/aws/rds_interfaces.go`)

`RDSDescribeDBSnapshotsAPI` already declared. No new interface needed for the cross-ref enricher (it makes zero API calls — cache only). `BackupListRecoveryPointsByResourceAPI` already exists.

## §4 Architectural extension — `IssueEnricherFunc` cache parameter

The cross-ref Wave-1 signals require the enricher to read the `dbi` cache. The current signature does not pass cache.

### §4.1 Signature change

```go
// BEFORE:
type IssueEnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error)

// AFTER:
type IssueEnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource, cache resource.ResourceCache) (IssueEnricherResult, error)
```

### §4.2 New `IssueEnricherResult` field

```go
// IssueAppends carries per-resource phrases to append to Resource.Issues
// at dispatch-merge time. Used by cross-ref Wave-1 enrichers (e.g. dbi-snap)
// to land Wave-1 phrases that require sibling-cache access. Phrases must
// match the §4 spec text verbatim. The dispatcher appends these AFTER
// fetcher-populated phrases preserving §4 precedence (caller's responsibility
// to compute the merged status string and emit it via FieldUpdates["status"]).
IssueAppends map[string][]string
```

### §4.3 Files touched (mechanical changes)

- `internal/aws/issue_enrichment.go` — type def + `NoOpIssueEnricher` signature + `IssueEnricherResult` struct.
- `internal/aws/issue_enrichment_test.go` — update mock signatures.
- `internal/aws/*_issue_enrichment.go` (43 files with custom enrichers) — add `_ resource.ResourceCache` param to each enricher func.
- `internal/aws/rds_snap_issue_enrichment.go` — replace NoOp registration with the real cross-ref enricher.
- `internal/tui/messages/messages.go` — add `IssueAppends` field on `EnrichmentCheckedMsg`.
- `internal/tui/app_probes.go` — capture `m.buildResourceCacheSnapshot()` at dispatch and pass into the enricher closure; populate `EnrichmentCheckedMsg.IssueAppends`.
- `internal/tui/app_handlers_availability.go` — merge `IssueAppends` into both `probeResources[shortName]` and `resourceCache[shortName].resources` Issues slices (parallel to the existing FieldUpdates merge loop).
- `internal/tui/views/resourcelist.go` (or wherever `ApplyFieldUpdates` lives) — add `ApplyIssueAppends` mirror.
- Test scaffolds in `tests/unit/` that construct mock enrichers — update call sites.

### §4.4 Backward compatibility

Every non-dbi-snap enricher receives the `cache` param and ignores it via `_`. Behavior is identical. NoOp is identical. Tests for non-dbi-snap enrichers are not touched beyond signature.

## §5 Implementation phases (post-impl-plan approval)

1. **Phase 6a (this plan's §2)** — fixture file + sibling updates.
2. **Phase 4-arch** — IssueEnricherFunc signature refactor (one focused coder dispatch, mechanical only).
3. **Phase 6b** (parallel with 7) — QA tests in `tests/unit/aws_rds_snap_*_test.go`.
4. **Phase 7** — coder implements: fetcher rewrite, view config, real cross-ref enricher, ct-events related registration.
5. **Phase 7.5** — scope-diff gate.
6. **Phase 8** — scenario-harness visual render gate (dbi-snap visual test + drill-through row + partial-failure scenario).
7. **Phase 9** — final report checklist (with §9.3 structural-cap exemption recorded).
