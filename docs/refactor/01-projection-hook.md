# Phase 01 — Typed projection hook

**1 PR. Mandatory. No prerequisites.**

## Goal

Replace the `if resType == "ct-events"` branches in `internal/tui/views/detail_fields.go` with a declarative `DetailProjector` hook on `ResourceTypeDef`. Detail rendering becomes a uniform pipeline: `Resource → []Section → rendered cells`. ct-events is just one type that registers a non-default projector.

This phase also introduces `domain.Severity` — the typed enum used by every subsequent phase. It lands here because Phase 02 needs it for `Item.Severity`, and bringing it forward avoids a second touch later.

## What this phase delivers

- `internal/domain/severity.go` — `Severity` enum (`SevOK | SevWarn | SevBroken | SevDim`), `IsIssue()` method, no presentation imports.
- `internal/semantics/projection/` package — owns `Section`, `Item`, `DetailProjector` type. Imports `internal/domain` (one-way).
- `internal/semantics/ctevent/` package — moved from `internal/aws/ctdetail/`. Exposes `Project(Resource) []projection.Section`.
- `ResourceTypeDef.Project DetailProjector` field — optional; nil means "use generic projection from `Fields` + `RawStruct`".
- `internal/semantics/projection/generic.go` — the default projector; reads `Resource.Fields` and reflects over `RawStruct` per the existing detail-view logic.
- `internal/tui/views/detail_render.go` — refactored to consume `[]projection.Section` regardless of resource type. ct-events branches deleted.

## FieldItem audit — what `Section` and `Item` must carry

Before scoping PR-01, the existing detail-view pipeline (`internal/fieldpath/extract.go` + `internal/tui/views/detail_fields.go`) carries the following metadata on each `FieldItem`. The new `Section`/`Item` abstraction must preserve every one of these:

| Behavior | Where it lives today | New home |
|---|---|---|
| Navigability flag (`Underline + Enter → RelatedNavigateMsg`) | `FieldItem.Navigable` | `Item.Navigable bool` |
| Target type for navigation (`"vpc"`, `"role"`, etc.) | `FieldItem.TargetType` | `Item.TargetType string` |
| Section / sub-section / spacer tagging | `FieldItem.Kind` (Field, Header, Subfield, Spacer) | `Section.Items[].Kind` enum |
| Nav ID overrides via `resource.NavIDFromValue` | applied in `buildFieldList` (`detail_fields.go`) | applied in `projection.Generic` and projector wrappers |
| Tag flattening (each tag becomes its own row) | `flattenTags` in `detail_fields.go` | helper in `projection/generic.go` |
| Embedded JSON expansion (e.g. policy documents) | `expandJSON` branch in `detail_fields.go` | helper in `projection/generic.go` |
| Wave 2 attention injection (leading "Background Check" section) | `injectAttention` in `detail_fields.go` | rendered separately by detail view; projector returns a "main" `[]Section`, attention is layered above (NOT a section returned by `Project`) |
| ct-event color tiers (`"!"`, `"~"`, `"impaired"`, `"initializing"`, `"ct-danger"`, `"ct-attention"`, `"ct-info"`) | `FieldItem.Tier` string | `Item.Tier string` (same string vocabulary; `TierColorStyle` already maps to lipgloss) |
| List-typed scalar extraction (e.g. `Subnets.SubnetId` first element) | `fieldpath.ExtractFirstListScalar` | unchanged; called from `projection.Generic` |
| Per-type field ordering and inclusion (per `~/.a9s/views/<type>.yaml`) | `ViewsConfig` consulted in `buildFieldList` | `projection.Generic` reads same config |

**Out of `Section` / `Item`**: Wave 2 attention rendering remains a separate path. The detail view first renders `r.AttentionDetails` as the "Background Check" section (when present), THEN renders `td.Project(r)` (or `projection.Generic(r)` if nil) below it. The projector is responsible for *core* fields only. This keeps the projector's contract simple — it doesn't need to know about Wave 2 — and matches today's `injectAttention` placement.

Implication: PR-01 scope is larger than the original 10–15 file estimate. Realistic surface is **25–40 files**, including `fieldpath` test updates, `detail_*_test.go` updates that assert on `FieldItem` shape, and the per-feature helpers above. If any audit row turns out to require deep restructuring (e.g. JSON expansion produces sub-trees instead of flat rows), defer that row to a follow-up PR-01-followups in the same phase.

## PR breakdown

This phase ships as **one PR**. The work is tightly coupled (`Severity` must exist before `Item.Severity`, `Section` must exist before the renderer change), and the per-feature helpers in the audit above all touch the same rendering path. Splitting produces dependency confusion without size benefit.

### PR-01 — Introduce projection hook

**Files added**

- `internal/domain/severity.go` (~30 LOC)
- `internal/semantics/projection/types.go` — `Section`, `Item`, `DetailProjector` (~40 LOC)
- `internal/semantics/projection/generic.go` — default projector (extracted from current `detail_fields.go` Fields/RawStruct logic) (~150 LOC)
- `internal/semantics/ctevent/` — entire `internal/aws/ctdetail/` directory, moved
- `internal/semantics/ctevent/projector.go` — wraps existing `BuildSections` to return `[]projection.Section`

**Files modified**

- `internal/resource/types.go` — add `Project DetailProjector` field to `ResourceTypeDef`
- `internal/resource/types_monitoring.go` — set `Project: ctevent.Project` on the ct-events type def
- `internal/tui/views/detail_fields.go` — replace ct-events branch with single uniform call: `sections := td.Project(r); if td.Project == nil { sections = projection.Generic(r) }`
- `internal/tui/views/detail_render.go` — consumes `[]projection.Section` (already does morally, just typed now)
- Every `internal/tui/views/detail_*_test.go` that asserts on rendered detail content — verify still passing

**Files deleted**

- The shortName branch in `detail_fields.go` lines 234–253 (ct-events special case)
- `sectionsToFieldItems` shim in `detail_fields.go:470` (no longer needed; everyone returns `Section` now)
- `internal/aws/ctdetail/` directory (moved, not deleted, but the path goes away)

**What this PR explicitly does NOT do**

- Does NOT touch `Resource.Status` / `Resource.Issues` / `(+N)` algebra. That's Phase 03.
- Does NOT introduce `Finding`. Only `Severity`. `Item.Severity` is set by the projector from existing `Resource.Status` / `Fields["state"]` interpretation; no canonical finding model yet.
- Does NOT move `internal/resource/types.go` to `internal/catalog/`. That's Phase 04.
- Does NOT change `EnrichmentFinding` shape — `Item.Severity` may map from `EnrichmentFinding.Severity` strings (`"!"`, `"~"`) until Phase 03 normalizes.

## Exit criteria

A PR is mergeable only when all of these are true. Verification commands run from repo root:

1. **No shortName dispatch in detail rendering.**
   ```bash
   rg '"ct-events"' internal/tui/views/
   # expected: zero hits
   rg 'resType == "' internal/tui/views/detail_*.go
   # expected: zero hits
   ```

2. **`internal/aws/ctdetail/` is deleted.**
   ```bash
   ls internal/aws/ctdetail/ 2>&1
   # expected: "No such file or directory"
   ```

3. **`internal/ui/` (still `internal/tui/views/` at this phase) does not import `internal/semantics/ctevent`.**
   ```bash
   rg 'semantics/ctevent' internal/tui/
   # expected: zero hits — only the type def in internal/resource/ may reference ctevent.Project
   ```

4. **Severity enum is presentation-free.**
   ```bash
   rg 'lipgloss|tcell|color\.' internal/domain/
   # expected: zero hits
   ```

5. **Generic projector covers every non-ct-events type.**
   - `make test` passes.
   - `./a9s --demo` renders detail views for at least: ec2, s3, rds, iam-role, alarm, sg. Manual verification: pixel-diff against `cmd/preview-detail` output is acceptable evidence; running the demo and visually comparing each detail to a recorded screenshot is the cheap path.

6. **ct-events detail unchanged from user perspective.**
   - Run `./a9s --demo`, navigate to ct-events, drill into an event detail. Sections (Identity, Action, Target, Context, Raw) render identically to before. This is the most visible regression risk.

## Out of scope

- `Finding`, `FindingCode`, `AttentionDetail` types. Phase 03.
- `Resource.Status` removal or `(+N)` algebra. Phase 03.
- Catalog migration or generator binaries. Phase 04.
- `internal/transport` rename or capability interfaces. Phase 03 introduces transport package; this phase still uses `internal/aws/`.
- Removing package globals (IAM policies, identity cache, SES rule sets). Phase 03.

## Cross-references

- **Enables Phase 03**: `Severity` enum is required for `Finding.Severity`.
- **Enables Phase 04**: `DetailProjector` is one of the catalog struct fields. Phase 04 just declares it; Phase 01 makes it work.
- **Independent of Phase 02**: Phase 02 (session owner) can land in parallel with no conflicts.

## Risk register

| Risk | Mitigation |
|---|---|
| Generic projector loses fidelity for some resource type that currently has implicit special-casing in `detail_fields.go` | The audit table above is the explicit list. Each row is a feature the projector path must preserve. PR-01 verifies each by running detail-view tests; any feature failing the test gates the PR. |
| FieldItem behaviors don't all map cleanly to `Section`/`Item` (e.g. JSON expansion produces a tree where `Item` is flat) | If any row in the audit fails to fit, defer that row's migration to a follow-up PR within Phase 01. The audit is the truth; the abstraction adapts to it, not vice versa. |
| ct-events `Section` shape doesn't perfectly match `projection.Section` | Compare `internal/aws/ctdetail/types.go` `Section` definition with the new `projection.Section`; if they differ, adjust the new type to be a strict superset and provide a one-line adapter in `ctevent/projector.go`. |
| `Severity` enum collides with existing `Color` symbols at call sites | Land `Severity` and `Color` side by side in this phase. `Color` stays in `internal/resource/`. Phase 03 promotes `Severity` to canonical; this phase is additive. |
