# Phase 04 — Declarative catalog; generated markdown

**14 PRs. Mandatory. Depends on Phase 03 (finding model).**

## Goal

Replace the `Register*` API with a static, declarative `internal/catalog` package. After this phase:

- `var ResourceTypes = []ResourceTypeDef{...}` is the single source of truth for every resource type.
- Each `ResourceTypeDef` literal carries direct function references for `Fetcher`, `Wave2`, `Project`, `Related`, `Navigable`, plus declarative tables for `Findings []FindingDef`, `Columns`, `Children`, `LifecycleKey`, `CloudTrail`, etc.
- `internal/catalog` remains **resource-shaped metadata only**. Cross-cutting capabilities such as logs, investigation/search workflows, cost views, and any future action system do not accrete as behavior handlers on `ResourceTypeDef`; the only catalog-side hook is declarative support metadata (`Capabilities`), while implementations bind separately by resource type.
- `cmd/catalogen` is a `go generate`–driven binary that emits **markdown only** — no generated Go code:
  - `docs/attention-signals.md`, `docs/related-resources.md`, `docs/resources/<short>.md` — generated specs.
  - Catalog-backed wrappers (PR-04a) handle legacy-registry compatibility at runtime; there is no generated `internal/aws/registry_generated.go`.
- CI runs the generator and `git diff --exit-code`; drift fails the build.

After this phase, the **mechanical-resource-implementation acceptance test** from `00-overview.md` is satisfiable: a new resource is one catalog struct literal, one transport file, optional Wave 2 / related files, demo fixtures, tests. No `init()`. No `Register*`. No markdown edits.

## Why one direction only — and no generated `init()`

The previous draft of this phase had additive fields on `ResourceTypeDef` *alongside* the existing `Register*` registry, with each per-category PR migrating consumers. That's dual-authoring: a new resource added during the migration window has to be declared in two places. Two sources of truth are exactly the lazy compromise this refactor exists to remove.

A second iteration proposed generating a `registry_generated.go` whose `init()` populated the legacy registry maps. That contradicts cross-phase invariant #5 ("compile-time codegen, not runtime `init()`") even if the generated file is checked in. Generated `init()` is still `init()`.

**The correct shape:**

- **From PR-04a onward, the catalog is authoritative.** Nothing is hand-edited in the legacy registry.
- The legacy registry accessors (`resource.FindResourceType`, `resource.AllResourceTypes`, `resource.AllShortNames`) become **thin Go wrappers around `catalog`** in PR-04a. No generated init() ever. The wrappers iterate `catalog.ResourceTypes` directly. Hand-written, one-line each.
- Per-category PRs (04b through 04m) move type definitions from `internal/resource/types_<category>.go` into `internal/catalog/types_<category>.go` AND remove the corresponding `categoryResourceTypes()` call from `buildResourceTypes()`. After PR-04m, `buildResourceTypes()` body is empty.
- For Wave 1 fetcher, Wave 2 enricher, related-def, navigable-field registration: today these live in `init()` calls in `internal/aws/*.go`. Per-category PRs migrate these *into the catalog struct literals* directly, replacing the legacy `init()` calls. After each per-category PR, that category has zero `init()` in `internal/aws/`.
- During 04a–04m, unmigrated categories' `init()` calls in `internal/aws/` continue to populate legacy registries. The catalog accessors fall back to legacy lookups for any short name not yet in `catalog.ResourceTypes`. After PR-04m, the fallback is unreachable and is deleted by 04n.
- 04n cuts the rope: delete `internal/resource/registry.go`, delete the fallback shim, delete the empty `buildResourceTypes()`, delete the legacy `Register*` symbols. `cmd/catalogen` ships, but only emits markdown — no Go code generation.

## Catalog shape

```go
// internal/catalog/types.go
package catalog

type ResourceTypeDef struct {
    // Identity
    Name      string
    ShortName string
    Aliases   []string
    Category  string
    ListTitle string

    // Display
    Columns        []Column
    LifecycleKey   string  // Phase 03 introduced; defaults "state"
    IdentityKey    string
    CellDecorators map[string]func(domain.Resource, string) string
    CopyField      string

    // Behavior
    Fetcher    PaginatedFetcher
    Wave2      IssueEnricher  // nil = no Wave 2 signal — replaces NoOpIssueEnricher
    Project    DetailProjector  // nil = generic projection
    Related    []RelatedDef
    Navigable  []NavigableField
    Children   []ChildViewDef
    Reveal     RevealFetcher
    DetailEnrich DetailEnricher  // optional; e.g. policy-doc fetch

    // Cross-cutting
    Capabilities         []domain.CapabilityID  // opt-in to logs, ct-investigate, cost, actions; handlers live outside catalog
    CloudTrailKey         string
    ExcludeFromIssueBadge bool
    StubCreator           func(string) domain.Resource
    RelatedContextFromIDs func([]string) map[string]string

    // Findings declarative table — graduated from Phase 03's per-enricher constants.
    Findings []FindingDef
}

type FindingDef struct {
    Code     domain.FindingCode
    Phrase   string             // §4 phrase
    Severity domain.Severity
    Source   string             // "wave1" | "wave2" — provenance class
}

// Static. No init(). No Register*.
var ResourceTypes = []ResourceTypeDef{
    // ... populated per-category in PR-04b onward
}
```

Boundary rule: if behavior is intrinsic to a resource type's list/detail/reveal/related/navigation metadata, it belongs in `ResourceTypeDef`. If it is a cross-cutting capability with its own screens, queries, or task model (logs, CloudTrail investigation, cost analysis, future actions), it gets its own module keyed by resource type, not another optional behavior field on the catalog struct. The one catalog-side hook is `Capabilities []domain.CapabilityID`: declarative support metadata only. Runtime uses that opt-in list when dispatching to capability modules in Phase 05.

## PR breakdown

14 PRs total: 1 skeleton + 12 per-category + 1 cleanup.

### PR-04a — Catalog skeleton + accessor wrappers + markdown generator

**Goal.** Create `internal/catalog/`, redirect legacy accessors to wrap `catalog`, and ship `cmd/catalogen` as a markdown-only generator. This PR introduces the machinery; the catalog is empty (no type entries yet) and the legacy accessors fall back to legacy registries for every short name.

**Files added**

- `internal/catalog/types.go` — `ResourceTypeDef`, `FindingDef`, helper types.
- `internal/catalog/catalog.go` — `var ResourceTypes []ResourceTypeDef` (initially empty), accessor functions (`Find(shortName) *ResourceTypeDef`, `All() []ResourceTypeDef`, `AllShortNames() []string`, `ByCategory(cat) []ResourceTypeDef`).
- `cmd/catalogen/main.go` — reads `catalog.ResourceTypes`, emits markdown only:
  - `docs/attention-signals.md` — markdown table generated from `Findings` × `Severity` data.
  - `docs/related-resources.md` — generated from `Related` defs.
  - `docs/resources/<short>.md` — per-resource markdown using **section markers**: `<!-- BEGIN GENERATED: <section> -->` / `<!-- END GENERATED: <section> -->`. Generated sections (Findings table, Related table, Columns table, Children) are replaced in place; prose between markers stays untouched. **No whole-file overwrite, ever.** (See "Per-resource markdown contract" below.)

**Wrapper coverage — every legacy registry surface, not just type defs.**

The per-category PRs delete `init()` registrations not only for `RegisterResourceType` but for every other registry the consumers depend on (fetchers, related, navigable fields, Wave 2 enrichers). If only `FindResourceType` / `AllResourceTypes` are wrapped, then 04b deleting compute-category `RegisterPaginated` / `RegisterRelated` / etc. breaks every compute consumer. **All registry surfaces below get catalog-backed wrappers in PR-04a, with legacy fallback active during 04b–m.** The fallback branches close in PR-04n.

The full wrapper set added to `internal/resource/registry.go` (or moved into `internal/catalog/wrappers.go` if cleaner — pick one in the PR):

| Wrapper | Replaces | Live consumers (verify and migrate in 04a) |
|---|---|---|
| `FindResourceType(name) *ResourceTypeDef` | iteration of `resourceTypes` slice | `internal/tui/app_probes.go:112`, `internal/resource/navigation.go:27` |
| `AllResourceTypes() []ResourceTypeDef` | direct read of `resourceTypes` | `tests/unit/architecture_conformance_test.go`, projector coverage test |
| `AllShortNames() []string` | derived from `resourceTypes` | `internal/tui/app_probes.go:214` |
| `GetPaginatedFetcher(short) PaginatedFetcher` | `paginatedFetchers` registry map | `internal/tui/app_fetchers.go:21`, `internal/tui/app_probes.go:227`, `internal/resource/navigation.go` (fetcher-only types path) |
| `GetFilteredPaginatedFetcher(short) FilteredPaginatedFetcher` | `filteredPaginatedFetchers` registry map | `internal/tui/app_fetchers.go:61` |
| `GetPaginatedChildFetcher(short) PaginatedChildFetcher` | `childFetchers` registry map | `internal/tui/app_fetchers.go:122` |
| `GetRevealFetcher(short) RevealFetcher` | `revealFetchers` registry map | `internal/tui/app_fetchers.go:263` |
| `HasRevealFetcher(short) bool` | `revealFetchers` registry presence | wherever the UI predicates on reveal availability (audit pre-PR) |
| `GetFetchByIDs(short) FetchByIDsFn` | `fetchByIDsRegistry` map | `internal/tui/app_related.go:144` |
| `GetDetailEnricher(short) DetailEnricher` | `detailEnrichers` registry map | `internal/tui/app_enrich.go:23` |
| `HasDetailEnricher(short) bool` | `detailEnrichers` registry presence | wherever the UI predicates on enrichment availability (audit pre-PR) |
| `GetChildType(short) *ResourceTypeDef` | `childTypes` registry map | `internal/tui/app_handlers.go:451`, `internal/tui/app_handlers_navigate.go:454`, `internal/resource/navigation.go:18` |
| `GetFieldKeys(short) []string` | `fieldKeysRegistry` | `internal/tui/views/detail_fields.go:284` |
| `ApplyFieldAliases(short, fields) map[string]string` | `fieldAliases` registry | `internal/tui/views/detail_fields.go:284` |
| `GetRelated(short) []RelatedDef` | `relatedRegistry` map | `internal/tui/app_related.go:26` |
| `GetNavigableFields(short) []NavigableField` | `navigableFieldsRegistry` map | `internal/tui/views/detail_fields.go:272` |
| `GetIssueEnricher(short) IssueEnricher` | `IssueEnricherRegistry` map | wherever `internal/tui/` invokes Wave 2 enrichers (audit pre-PR) |

The wrapper set must close every consumer the per-category PRs (04b–m) will strand by deleting `init()` calls. If any registry surface is missing from this list, the migrated category breaks the moment its `init()` is removed. Audit pre-PR with:

```bash
rg '^func\s+(Get|Find|All|Apply|Has)\w+\(' internal/resource/ internal/aws/issue_enrichment.go
```

Every public `Get*` / `Find*` / `All*` / `Apply*` / `Has*` accessor in those files must have a corresponding wrapper. The grep above is the audit gate; do not skip it.

**Wrapper template:**

```go
func FindResourceType(name string) *ResourceTypeDef {
    if t := catalog.Find(name); t != nil { return adaptCatalog(t) }
    // fallback for short names not yet in catalog (only meaningful 04a–04m)
    return findInLegacyRegistry(name)
}

func GetPaginatedFetcher(short string) PaginatedFetcher {
    if t := catalog.Find(short); t != nil && t.Fetcher != nil { return t.Fetcher }
    return legacyPaginatedFetchers[short]
}
// ... etc., one per wrapper above
```

After PR-04m, the fallback branch in every wrapper is unreachable; PR-04n deletes the fallbacks along with the legacy registry maps.

**Files modified**

- `internal/resource/registry.go` (or new `internal/catalog/wrappers.go`) — adds every wrapper above.
- **Consumer migration in this PR**: `internal/tui/app_fetchers.go`, `internal/tui/app_probes.go`, `internal/tui/app_related.go`, `internal/tui/views/detail_fields.go`, `internal/resource/navigation.go`, plus any other site found by:
  ```bash
  rg 'resource\.(GetPaginatedFetcher|FindResourceType|GetRelated|GetNavigableFields|AllShortNames|GetTypeByShortName|AllResourceTypes|GetIssueEnricher)\b' internal/
  ```
  Every match either keeps using `resource.<API>` (which is now a wrapper) — acceptable, since the wrapper handles routing — or switches to `catalog.<API>` if the wrappers live in `internal/catalog/`. Pick one location and stick with it; do not split wrappers across two packages.
- **Import-cycle audit**: `internal/catalog` imports `internal/domain` for the type aliases. `internal/resource` imports `internal/catalog` for wrappers. `internal/catalog` MUST NOT import `internal/resource` — verify with `go list -f '{{.Imports}}' github.com/k2m30/a9s/v3/internal/catalog | grep internal/resource` (expected: zero hits).
- `Makefile` — `make generate` runs `go generate ./...` invoking `cmd/catalogen`; CI runs `make generate` then `git diff --exit-code`.
- `internal/catalog/doc.go` — `//go:generate go run ../../cmd/catalogen` directive (catalog drives the generator, since catalog is the input).

**Exit criteria**

```bash
ls internal/catalog/types.go internal/catalog/catalog.go
# expected: both present

ls cmd/catalogen/main.go
# expected: present

make generate
# expected: no errors (writes only to docs/, between BEGIN/END markers)
git diff --exit-code
# expected: no diff (regenerated markdown matches committed)

# No generated Go file in this PR:
ls internal/aws/registry_generated.go 2>&1
# expected: "No such file or directory" — there is NO Go codegen, only markdown

go build ./...
# expected: clean compile

make test
# expected: passes — catalog is empty, accessors fall back to legacy registries, behavior unchanged
```

**Stabilization checkpoint**: PR-04n. PR-04a installs the catalog skeleton and the wrapper layer that downstream per-category PRs (04b–m) depend on. Once any per-category PR has landed, reverting PR-04a alone breaks every migrated category; the unit of revert is the phase. Per `00-overview.md` "Migration discipline", do not preserve dual-iteration paths (legacy `resourceTypes` slice AND `catalog.ResourceTypes`) beyond what the migration window strictly needs — the wrappers are one-way compat for the duration of the phase, and PR-04n removes them.

---

### PR-04b through PR-04m — Per-category population

12 PRs, one per category, mirroring Phase 03's split:

| PR | Category |
|---|---|
| 04b | compute |
| 04c | containers |
| 04d | networking |
| 04e | databases |
| 04f | security |
| 04g | secrets |
| 04h | monitoring |
| 04i | messaging |
| 04j | cicd |
| 04k | dns_cdn |
| 04l | data |
| 04m | backup |

**Per-PR scope (template).** Each per-category PR does the migration in lockstep so the build never breaks:

1. Creates `internal/catalog/types_<category>.go` with a slice of `ResourceTypeDef` literals — one per resource type in the category. Direct function references for `Fetcher`, `Wave2`, `Project`, `Related`, `Navigable`. Direct table for `Findings`.
2. Appends `types_<category>ResourceTypes...` (or equivalent) to `var ResourceTypes` in `internal/catalog/catalog.go`.
3. Removes the corresponding `<category>ResourceTypes()` call from `buildResourceTypes()` in `internal/resource/types.go` in the *same* PR. **The category function and its file are deleted only when the catalog has the full type set for that category.** This keeps `go build ./...` clean (the hard-safety #1 from `00-overview.md`'s migration discipline). It does NOT mean the legacy `buildResourceTypes()` and the catalog must agree on every test or render at every PR — only that the tree compiles. Test/render parity for migrated categories is asserted at the phase-exit checkpoint (PR-04n).
4. Deletes `internal/resource/types_<category>.go` once the legacy `buildResourceTypes()` no longer references its function.
5. For Wave 1 fetcher / Wave 2 enricher / related-def / navigable-field registrations: removes the `init()` calls in `internal/aws/<svc>*.go` for every type in this category. Replaces them with direct field assignments in the catalog struct literal. Each per-category PR shrinks `init()` count in `internal/aws/` for that category to zero.
6. Deletes any `internal/aws/<svc>_issue_enrichment.go` file whose entire content was a `NoOpIssueEnricher` registration. (Files that contain real enricher functions stay; the function is now referenced from the catalog `Wave2` field.)
7. **Color → severity inline.** The new catalog `ResourceTypeDef` has no `Color` field. For every type in this category, delete the per-type `Color` helper (`func ec2Color`, `func rdsColor`, etc.) and replace `td.Color(r)` call sites in `internal/tui/views/` with `styles.SeverityStyle(rowSeverity(r, td))` — `internal/ui/styles/severity.go` is added in PR-04a alongside the catalog skeleton. The first per-category PR (04b) creates `internal/ui/styles/severity.go`; subsequent PRs reuse it.
8. Runs `make generate`. Verifies the generated markdown for this category is updated and committed.
9. Updates the corresponding tests in `tests/unit/`. Most tests will be unaffected (they go through public APIs that now route via catalog wrappers); the ones that directly construct `ResourceTypeDef` literals or call legacy registration functions need migration. Estimated per-category test diff: 5–15 files, 100–300 lines.

**Per-PR exit criteria.** For category `<X>` in PR-04<X>:

```bash
# Catalog declares this category's types:
ls internal/catalog/types_<x>.go
# expected: present

# Old registration source is gone:
ls internal/resource/types_<x>.go 2>&1
# expected: "No such file or directory"

# buildResourceTypes() no longer references this category:
rg '<x>ResourceTypes\(\)' internal/resource/types.go
# expected: zero hits

# No init() functions remain in the category's transport files:
rg '^func init\(\)' internal/aws/<x-services>*.go
# expected: zero hits

# Markdown is regenerated and committed:
make generate
git diff --exit-code
# expected: no diff

# Generated markdown reflects the new category:
rg '<one-of-x-types>' docs/attention-signals.md
# expected: present

# go build still works (this is the build-break guard):
go build ./...
# expected: clean
```

Behavior verification:
- `./a9s --demo`: every type in the migrated category lists and details correctly.
- Findings tables in the catalog match what the runtime emits (Phase 03's shim is gone by now; Findings come straight from the enricher).
- Snapshot tests for every type in this category (added in Phase 03) match.

**Stabilization checkpoint**: PR-04n. Per-category PRs are *scopeable* in parallel (different developers can author 04b through 04m concurrently without file collisions) but they are NOT independently revertable. Each per-category PR deletes the legacy `init()` registrations for its category; reverting one PR after subsequent PRs have built on the empty `buildResourceTypes()` body breaks the build. The fallback in catalog wrappers is migration-window scaffolding that PR-04n removes — do not preserve it as a permanent escape hatch (forbidden by `00-overview.md` hard-safety #3, no permanent dual-source-of-truth).

---

## Per-resource markdown contract

Existing `docs/resources/<short>.md` files (66 non-impl-plan files at time of writing — verify with `ls docs/resources/ | grep -v impl-plan | wc -l` before sealing the exit criterion) carry DevOps prose: "why this matters", workflow notes, real-world examples. **This prose is NOT trivially derivable from the catalog struct and is preserved across regenerations.**

The contract: every per-resource markdown file is annotated with section markers. The generator only writes between markers. Prose between markers is preserved verbatim. New markers can be added; old markers stay even if their content is empty (so `cmd/catalogen` knows where to inject).

```markdown
# {{Resource Name}}

<!-- BEGIN GENERATED: header -->
{{ShortName}} — {{Category}}. Lifecycle key: `{{LifecycleKey}}`.
<!-- END GENERATED: header -->

## Why this matters

(prose, hand-edited, never overwritten)

## Findings

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source |
| --- | --- | --- | --- |
| ec2.impaired | impaired | broken | wave1 |
| ...
<!-- END GENERATED: findings -->

## Workflow

(prose, hand-edited, never overwritten)

## Related Resources

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| ...
<!-- END GENERATED: related -->
```

**Generator algorithm.** For each `<short>.md`:
1. If file does not exist: emit a stub from a template with all expected `BEGIN/END GENERATED` blocks; prose-only sections marked with `(TODO: write narrative)`.
2. If file exists: read it; for each `BEGIN GENERATED: <section>` … `END GENERATED: <section>` pair, replace the block content. Outside blocks, byte-preserve.
3. If file is missing a generated block the generator wants to write (e.g. a new `findings` section): append a new block at the end with a comment indicating it was auto-added.

**Human edits**: anywhere outside `BEGIN GENERATED` / `END GENERATED` markers. The generator never touches that prose.

**No GENERATED blocks at all on a file**: the generator treats the file as fully hand-edited (legacy mode) and only verifies that the catalog's `ShortName` matches a file in `docs/resources/`. Migration of legacy files into marker form happens incrementally, one PR at a time, until every file uses the contract. Phase 04 does not require all 66 files to be marker-converted in one go — that's a separate cleanup that can run alongside the per-category PRs.

---

### PR-04n — Delete legacy registry; finalize zero-init() state

**Goal.** Cut the rope. Legacy `Register*` API is deleted. The legacy `resourceTypes` slice is deleted. Consumers read directly from `catalog.ResourceTypes`. The catalog accessor wrappers' fallback branch is deleted (it was only meaningful during 04b–04m).

**Files modified**

- `internal/resource/registry.go` (or wherever the wrappers live after 04a) — delete the legacy fallback branch. `FindResourceType` etc. are pure passthroughs to `catalog.Find` etc.
- Every direct consumer of `resource.FindResourceType`, `resource.AllResourceTypes`, `resource.AllShortNames` — optionally switch to calling `catalog.*` directly. (Keeping the resource-package wrappers as aliases is fine for one more PR cycle if it reduces churn; the wrappers are now zero-cost.)
- `tests/unit/architecture_conformance_test.go` — iterate `catalog.ResourceTypes` directly. Drop any markdown-parsing scaffolding.

**Files deleted**

- `internal/resource/registry.go` `Register*` exported functions — entire surface
- `internal/resource/types.go` `var resourceTypes = buildResourceTypes()`, `buildResourceTypes()`, the per-category builder slice — all gone (or `buildResourceTypes()` becomes `return nil` and is then deleted in a follow-up if linter complains)
- `internal/aws/issue_enrichment.go` `IssueEnricherRegistry` map and `registerIssueEnricher` helper — Wave 2 is now declarative on each type def
- All `NoOpIssueEnricher` registrations (most already deleted in 04b–04m; this is the final sweep)
- Any `*_issue_enrichment.go` file whose only remaining content was `init()` registration code

**Exit criteria**

```bash
# Zero init() functions in feature wiring:
rg '^func init\(\)' internal/aws/ internal/resource/ internal/catalog/
# expected: zero hits

# No Register* functions remain:
rg '^func Register[A-Z]\w+' internal/resource/ internal/aws/
# expected: zero hits

# No NoOpIssueEnricher references:
rg 'NoOpIssueEnricher' internal/
# expected: zero hits

# Legacy registry file is gone:
ls internal/resource/registry.go 2>&1
# expected: "No such file or directory"

# Generated markdown is current:
make generate
git diff --exit-code
# expected: no diff

# Per-resource specs exist for every catalog entry (verify count via catalog, not hard-coded):
go run ./cmd/catalogen -verify
# expected: every entry in catalog.ResourceTypes has a corresponding docs/resources/<short>.md

# Conformance tests iterate catalog directly:
rg 'parseMarkdownTable|attention-signals\.md' tests/unit/
# expected: zero hits — tests read the catalog, not parsed markdown
```

Mechanical-resource-implementation acceptance test passes (overview's program-wide criterion):
- Cherry-pick a hypothetical CloudHSM resource type addition. Confirm the file set is exactly:
  - `internal/catalog/types_security.go` (one struct literal added to slice)
  - `internal/aws/cloudhsm.go` (new file: the paginated fetcher and any related/Wave2 functions referenced by the catalog literal)
  - `internal/demo/fixtures/cloudhsm.go`
  - `tests/unit/cloudhsm_*.go`
- No `init()`, no `Register*`, no markdown edits, no `app.go` touches. `make generate` produces the markdown.

## Out of scope

- Renaming `internal/aws/` → `internal/transport/`. **Decision: no rename, ever, in this program.** Earlier drafts deferred this to "post-refactor mechanical PR"; the deferral was never going to be cheap (323 files in `internal/aws/` today, all import paths in `internal/catalog/` and `internal/tui/` reference `internal/aws/...`) and the rename adds zero structural value beyond aesthetics. The catalog references functions at their `internal/aws/<svc>.go` paths permanently. If `internal/aws/` becomes a misleading name (because half the files are no longer "AWS clients" but generic transport), rename it then. Until then, leave it.
- Logs/investigation/cost/action capability modules. Phase 04 defines the resource catalog boundary; it does not model capability handlers/screens inside the catalog beyond declarative support metadata (`Capabilities`).
- `internal/runtime` extraction. Phase 5a-extract.
- Gen type unification. Phase 5a-gens.
- Command/event message split. Phase 5b.

## Cross-references

- **Depends on Phase 03**: catalog `Findings []FindingDef` table only makes sense once `Finding`/`FindingCode` exist canonically.
- **Depends on Phase 02**: capability interfaces in `internal/session` are referenced by the `Fetcher` and `Wave2` function signatures stored in the catalog.
- **Enables Phase 5a-extract**: by then, `tui.Model` reads from `catalog`, not from a registered type API; un-embedding `sessionRuntime` no longer has any registry-coupling concerns.

## Risk register

| Risk | Mitigation |
|---|---|
| Generator output changes during a PR but contributor forgets `make generate` | CI gate: `make generate && git diff --exit-code`. Pre-commit hook in `.git/hooks/pre-commit` invokes the same gate locally. |
| Catalog struct gets unwieldy as fields accumulate | First ask whether the proposed field belongs in the catalog at all; cross-cutting capability contracts live outside `ResourceTypeDef`. If the remaining resource metadata still exceeds ~30 fields, split into nested sub-structs (`Display`, `Behavior`, `Pivots`). The struct literal stays readable; field grouping mirrors the documentation sections. |
| Static `var ResourceTypes` requires `internal/catalog` to import `internal/aws/<svc>` for function references — risk of import cycle if `internal/aws/` ever imports `internal/catalog/` | Constraint: `internal/aws/` MUST NOT import `internal/catalog/`. Type definitions referenced from both sides (`PaginatedFetcher`, `IssueEnricher`, `RelatedDef`, `NavigableField`) live in the leaf `internal/domain/` package. Verify with `go list -f '{{.Imports}}' github.com/k2m30/a9s/v3/internal/aws | grep internal/catalog` (expected: zero hits). |
| 22 NoOp enricher files contain non-trivial init logic that's not just registration | Audit before deletion: `cat internal/aws/<svc>_issue_enrichment.go` for each NoOp file. If the file has anything beyond a `registerIssueEnricher(short, NoOpIssueEnricher, prio)` call, defer that file's deletion to a separate cleanup. |
| `cmd/catalogen` runs slowly because it imports the whole `catalog` package | This is fine — the generator runs at build time, not runtime. Slow generators are acceptable; runtime `init()` storms are not. |
