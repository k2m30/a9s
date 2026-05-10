# Phase 01 — Domain bootstrap + typed projection hook

**1 PR. Mandatory. No prerequisites.**

## Goal

Replace the `m.resourceType == "ct-events"` branches in `internal/tui/views/detail_fields.go` with a declarative `DetailProjector` hook on `ResourceTypeDef`. Detail rendering becomes a uniform pipeline: `Resource → []Section → rendered cells`. ct-events is just one type that registers a non-default projector.

This phase also introduces the `internal/domain` leaf package — the foundation every subsequent phase builds on. It carries `Severity`, `Resource` (moved from `internal/resource/resource.go`), and the `Section` / `Item` / `DetailProjector` type declarations.

The motivation for bundling these into one PR is structural: if `DetailProjector` is declared in `internal/semantics/projection/` and typed against `resource.Resource`, then setting `ResourceTypeDef.Project = ctevent.Project` creates an `internal/resource → internal/semantics/ctevent → internal/semantics/projection → internal/resource` import cycle. Putting the type *declarations* in `internal/domain` (a leaf package with no internal imports) and keeping the *implementations* in `internal/semantics/*` breaks the cycle structurally — `internal/semantics/*` and `internal/resource/` both import `internal/domain`, and neither imports the other for types.

## What this phase delivers

### Domain layer (new)

- `internal/domain/severity.go` — `Severity` enum (`SevOK | SevWarn | SevBroken | SevDim`), `IsIssue()` method. No presentation imports.
- `internal/domain/resource.go` — `Resource` struct, **moved** from `internal/resource/resource.go`. Same fields (`ID`, `Name`, `Status`, `Issues`, `Fields`, `RawStruct`) — the migration of `Status` / `Issues` to `Findings` is Phase 03's concern, not this phase's.
- `internal/domain/projection.go` — `Section`, `Item`, and the `DetailProjector func(domain.Resource) []domain.Section` type declaration. Type declarations only.
- `internal/domain/contracts.go` — type declarations for every shared contract the catalog and runtime reference in later phases. Function signatures: `PaginatedFetcher`, `FilteredPaginatedFetcher`, `PaginatedChildFetcher`, `RevealFetcher`, `FetchByIDsFn`, `IssueEnricher` / `IssueEnricherFunc`, `DetailEnricher`, `RelatedChecker`. Struct/types: `Column`, `ChildViewDef`, `RelatedDef`, `NavigableField`, `FetchResult`, `ParentContext`, `ResourceCache`, `CapabilityID`, and the shared query-contract declarations (`QueryFilter`, `TimeRange`, `Cursor`, `QueryLimit`, `QuerySpec`). Currently scattered across `internal/resource/types.go` (`Column`, `ChildViewDef`), `internal/resource/registry.go` (`ParentContext` and the function types), `internal/resource/related.go` (`RelatedDef`, `RelatedChecker`, `ResourceCache`), `internal/resource/enricher.go` (`DetailEnricher`), `internal/resource/pagination.go` (`FetchResult`), and `internal/aws/issue_enrichment.go` (`IssueEnricherFunc`). Move the declarations into `internal/domain/contracts.go`; implementations, registry maps, and registration functions stay where they are. Each original file keeps a `type X = domain.X` re-export alias so existing call sites compile without churn. The aliases die with the legacy registry in PR-04n.

  Without this move, `internal/catalog` would have to import `internal/resource` and `internal/aws` for the type names, recreating the cycle Phase 01 exists to break.

### Semantics layer (new)

- `internal/semantics/projection/generic.go` — the default projector implementation; reads `Resource.Fields` and reflects over `RawStruct` per the existing detail-view logic. Returns `[]domain.Section`. Imports `internal/domain` only.
- `internal/semantics/ctevent/` — entire `internal/aws/ctdetail/` directory **moved** here. Exposes `Project(domain.Resource) []domain.Section`. Imports `internal/domain` only.
- `internal/semantics/selector/` — shared matcher home for non-trivial resource-selection semantics. Minimum contract: `type Matcher interface { Matches(domain.Resource) bool }`. Initial constructors cover wildcard ARN matching and tag-condition matching so future related/checker work does not re-invent string matching in place.

### Existing layers (modified)

- `internal/resource/resource.go` — collapses to a single line: `type Resource = domain.Resource` (Go type alias). Every existing import of `internal/resource` continues to compile. The alias is deleted in PR-04n alongside `internal/resource/registry.go`.
- `ResourceTypeDef.Project domain.DetailProjector` field — optional; nil means "use generic projection from `Fields` + `RawStruct`".
- `internal/tui/views/detail_fields.go` and `detail_render.go` — consume `[]domain.Section` uniformly. ct-events branches deleted.

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

### PR-01 — Domain bootstrap + introduce projection hook

#### Files added

- `internal/domain/severity.go` — `Severity` enum + `IsIssue()` (~30 LOC)
- `internal/domain/resource.go` — `Resource` struct moved from `internal/resource/resource.go` (~25 LOC)
- `internal/domain/projection.go` — `Section`, `Item`, `DetailProjector` type declarations (~40 LOC)
- `internal/domain/contracts.go` — shared catalog/runtime contract declarations (including `CapabilityID` and query-spec types) (~60 LOC)
- `internal/semantics/projection/generic.go` — default projector implementation, returns `[]domain.Section` (extracted from current `detail_fields.go` Fields/RawStruct logic) (~150 LOC)
- `internal/semantics/selector/` — matcher home (`Matcher` interface plus initial wildcard/tag matchers) (~80 LOC)
- `internal/semantics/ctevent/` — entire `internal/aws/ctdetail/` directory, moved
- `internal/semantics/ctevent/projector.go` — wraps existing `BuildSections` to return `[]domain.Section`

#### Files modified

- `internal/resource/resource.go` — replace struct definition with single line: `type Resource = domain.Resource` (alias). Every consumer of `resource.Resource` continues to compile.
- `internal/resource/types.go` — add `Project domain.DetailProjector` field to `ResourceTypeDef`
- `internal/resource/types_monitoring.go` — set `Project: ctevent.Project` on the ct-events type def
- `internal/tui/views/detail_fields.go` — replace ct-events branch with single uniform call: `sections := td.Project(r); if td.Project == nil { sections = projection.Generic(r) }`
- `internal/tui/views/detail_render.go` — consumes `[]domain.Section` (already does morally, just typed now)
- Every `internal/tui/views/detail_*_test.go` that asserts on rendered detail content — verify still passing

#### Files deleted

- The shortName branch in `detail_fields.go` lines 234–253 (ct-events special case)
- `sectionsToFieldItems` shim at `detail_fields.go:470` (no longer needed; everyone returns `[]domain.Section` now)
- `internal/aws/ctdetail/` directory (moved to `internal/semantics/ctevent/`, not deleted, but the old path goes away)

#### What this PR explicitly does NOT do

- Does NOT touch `Resource.Status` / `Resource.Issues` / `(+N)` algebra. That's Phase 03.
- Does NOT introduce `Finding`. Only `Severity`. `Item.Severity` is set by the projector from existing `Resource.Status` / `Fields["state"]` interpretation; no canonical finding model yet.
- Does NOT move `internal/resource/types.go` to `internal/catalog/`. That's Phase 04.
- Does NOT change `EnrichmentFinding` shape — `Item.Severity` may map from `EnrichmentFinding.Severity` strings (`"!"`, `"~"`) until Phase 03 normalizes.

## Exit criteria

A PR is mergeable only when all of these are true. Verification commands run from repo root:

1. **No shortName dispatch in detail rendering.**

   ```bash
   rg '== "ct-events"|"ct-events" ==' internal/tui/views/
   # expected: zero hits
   rg '\b(resType|resourceType|m\.resourceType)\s*==\s*"' internal/tui/views/detail_*.go
   # expected: zero hits

   ```

   (The previous draft used `resType == "` — the live code uses `m.resourceType ==`. Match both, plus the simpler "literal-against-shortname" pattern.)

2. **`internal/aws/ctdetail/` is deleted.**

   ```bash
   ls internal/aws/ctdetail/ 2>&1
   # expected: "No such file or directory"

   ```

3. **`internal/tui/views/` does not import `internal/semantics/ctevent`.**

   ```bash
   rg 'semantics/ctevent' internal/tui/
   # expected: zero hits — only the type def in internal/resource/ may reference ctevent.Project

   ```

4. **`internal/domain` is presentation-free and imports nothing internal.**

   ```bash
   rg 'lipgloss|tcell|color\.' internal/domain/
   # expected: zero hits

   rg 'github\.com/k2m30/a9s' internal/domain/
   # expected: zero hits — domain is a leaf package

   ```

5. **No import cycle from `internal/resource` to `internal/semantics/*`.**

   ```bash
   go list -f '{{.Imports}}' github.com/k2m30/a9s/v3/internal/semantics/ctevent | tr ' ' '\n' | grep 'internal/resource'
   # expected: zero hits
   go list -f '{{.Imports}}' github.com/k2m30/a9s/v3/internal/semantics/projection | tr ' ' '\n' | grep 'internal/resource'
   # expected: zero hits

   ```

6. **`Resource` lives in `internal/domain`; `internal/resource` re-exports.**

   ```bash
   rg '^type Resource struct' internal/domain/resource.go
   # expected: present
   rg '^type Resource =' internal/resource/resource.go
   # expected: present (single-line type alias)
   rg '^type Resource struct' internal/resource/resource.go
   # expected: zero hits

   ```

7. **Generic projector covers every type — structural test, not a smoke list.**

   ```go
   // tests/unit/projection_coverage_test.go
   func TestProjectorCoverageAllTypes(t *testing.T) {
       for _, td := range resource.AllResourceTypes() {
           fix := demo.Fixture(td.ShortName) // first fixture per type
           if fix == nil { continue }
           sections := td.Project
           if sections == nil { sections = projection.Generic }
           got := sections(fix)
           if len(got) == 0 {
               t.Errorf("%s: projector returned zero sections", td.ShortName)
           }
       }
   }

   ```

   This loop replaces the previous "verify ec2/s3/rds/..." smoke list. `make test` runs it; any regression in any of the 66 types fails the gate.

8. **ct-events detail unchanged from user perspective.**
   - Run `./a9s --demo`, navigate to ct-events, drill into an event detail. Sections (Identity, Action, Target, Context, Raw) render identically to before. This is the most visible regression risk.

## Out of scope

- `Finding`, `FindingCode`, `AttentionDetail` types. Phase 03.
- `Resource.Status` removal or `(+N)` algebra. Phase 03.
- Catalog migration or generator binaries. Phase 04.
- Query/evidence screens (logs, CloudTrail search/debug, cost views). `DetailProjector` is for rendering one resource's detail payload, not for time-bounded searches or multi-step investigation UIs. Those land on the runtime/screen contracts in Phase 05.
- `internal/transport/<svc>/` package layout. The `internal/aws/ctdetail/` directory MOVES to `internal/semantics/ctevent/` in this phase (because it stops being a transport concern and becomes a semantics-layer concern), but `internal/aws/<svc>.go` fetcher files stay where they are. **Decision: `internal/aws/` is not renamed in this program** (see `04-catalog.md` "Out of scope").
- Removing package globals (IAM policies, identity cache, SES rule sets). Phase 02 (was previously listed as Phase 03 — Phase 02 is the session-owner phase that closes the global-cache boundary).
- Deleting per-category `Color` functions. They co-exist with `Severity` through this phase. Phase 04 per-category PRs collapse them inline (the new catalog struct has no `Color` field).

## Cross-references

- **Enables every later phase**: `internal/domain` is the leaf package every later phase imports. `Severity`, `Resource`, and the projection types declared here are referenced by Phase 02 (session caches keyed by `domain.Resource`), Phase 03 (`Finding.Severity`, `AttentionDetail` carries `domain.DetailRow`), Phase 04 (catalog struct fields), and Phase 05 (`domain.Gen`).
- **Independent of Phase 02**: Phase 02 (session owner) can land in parallel with no conflicts. Phase 02 imports `internal/domain` for `Resource` but does not depend on the projection types.

## Risk register

| Risk | Mitigation |
|---|---|
| Generic projector loses fidelity for some resource type that currently has implicit special-casing in `detail_fields.go` | The audit table above is the explicit list. Each row is a feature the projector path must preserve. PR-01 verifies each by running detail-view tests; any feature failing the test gates the PR. |
| FieldItem behaviors don't all map cleanly to `Section`/`Item` (e.g. JSON expansion produces a tree where `Item` is flat) | If any row in the audit fails to fit, defer that row's migration to a follow-up PR within Phase 01. The audit is the truth; the abstraction adapts to it, not vice versa. |
| ct-events `Section` shape doesn't perfectly match `projection.Section` | Compare `internal/aws/ctdetail/types.go` `Section` definition with the new `projection.Section`; if they differ, adjust the new type to be a strict superset and provide a one-line adapter in `ctevent/projector.go`. |
| `Severity` enum collides with existing `Color` symbols at call sites | Land `Severity` and `Color` side by side in this phase. `Color` stays in `internal/resource/`. Phase 03 promotes `Severity` to canonical; this phase is additive. |
