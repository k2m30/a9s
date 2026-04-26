# Phase 04 — Declarative catalog; generated markdown

**14 PRs. Mandatory. Depends on Phase 03 (finding model).**

## Goal

Replace the `Register*` API with a static, declarative `internal/catalog` package. After this phase:

- `var ResourceTypes = []ResourceTypeDef{...}` is the single source of truth for every resource type.
- Each `ResourceTypeDef` literal carries direct function references for `Fetcher`, `Wave2`, `Project`, `Related`, `Navigable`, plus declarative tables for `Findings []FindingDef`, `Columns`, `Children`, `LifecycleKey`, `CloudTrail`, etc.
- `cmd/catalogen` is a `go generate`–driven binary that emits:
  - `internal/aws/registry_generated.go` — compatibility scaffolding for any consumer not yet migrated off the legacy registry API.
  - `docs/attention-signals.md`, `docs/related-resources.md`, `docs/resources/<short>.md` — generated specs.
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

**Files modified**

- `internal/resource/registry.go` (or sibling) — `FindResourceType`, `AllResourceTypes`, `AllShortNames` become wrappers:
  ```go
  // before
  func FindResourceType(name string) *ResourceTypeDef { /* iterate resourceTypes */ }
  // after
  func FindResourceType(name string) *ResourceTypeDef {
      if t := catalog.Find(name); t != nil { return adaptCatalog(t) }
      // fallback for short names not yet in catalog (only meaningful 04a–04m)
      return findInLegacyRegistry(name)
  }
  ```
  After PR-04m, the fallback branch is unreachable; PR-04n deletes it along with the legacy registry.
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

**Independently revertable**: yes. Reverting removes the catalog package and the wrappers; legacy accessors return to direct iteration of `resourceTypes`.

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
3. Removes the corresponding `<category>ResourceTypes()` call from `buildResourceTypes()` in `internal/resource/types.go` in the *same* PR. **The category function and its file are deleted only when the catalog has the full type set for that category.** This is what prevents the build break the original draft would have caused: the legacy `buildResourceTypes()` and the catalog are kept consistent at every PR boundary — never both empty, never both populated.
4. Deletes `internal/resource/types_<category>.go` once the legacy `buildResourceTypes()` no longer references its function.
5. For Wave 1 fetcher / Wave 2 enricher / related-def / navigable-field registrations: removes the `init()` calls in `internal/aws/<svc>*.go` for every type in this category. Replaces them with direct field assignments in the catalog struct literal. Each per-category PR shrinks `init()` count in `internal/aws/` for that category to zero.
6. Deletes any `internal/aws/<svc>_issue_enrichment.go` file whose entire content was a `NoOpIssueEnricher` registration. (Files that contain real enricher functions stay; the function is now referenced from the catalog `Wave2` field.)
7. Runs `make generate`. Verifies the generated markdown for this category is updated and committed.
8. Updates the corresponding tests in `tests/unit/`. Most tests will be unaffected (they go through public APIs that now route via catalog wrappers); the ones that directly construct `ResourceTypeDef` literals or call legacy registration functions need migration. Estimated per-category test diff: 5–15 files, 100–300 lines.

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

**Independently revertable**: yes. Each per-category PR is independent. Reverting one puts that category back under the legacy registry path; the catalog's accessor wrappers fall back to legacy lookup for any short name not in `catalog.ResourceTypes`. Other migrated categories are unaffected.

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
  - `internal/transport/cloudhsm/fetch.go` (new; or `internal/aws/cloudhsm.go` until renamed)
  - optionally `wave2.go`, `related.go`
  - `internal/demo/fixtures/cloudhsm.go`
  - `tests/unit/cloudhsm_*.go`
- No `init()`, no `Register*`, no markdown edits, no `app.go` touches. `make generate` produces the markdown.

## Out of scope

- Renaming `internal/aws/` → `internal/transport/`. The catalog references functions in their current location; if a rename is wanted, it's a post-Phase-04 mechanical PR.
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
| Catalog struct gets unwieldy as fields accumulate | If `ResourceTypeDef` exceeds ~30 fields, split into nested sub-structs (`Display`, `Behavior`, `Pivots`). The struct literal stays readable; field grouping mirrors the documentation sections. |
| Static `var ResourceTypes` requires all package imports of `internal/transport/<svc>` for function references — risk of import cycles | Catalog imports `internal/transport`; transport must not import catalog. If it does (e.g. for `ResourceTypeDef`), the type definition itself moves to a leaf package (`internal/domain` or `internal/types`) that both can depend on. |
| 22 NoOp enricher files contain non-trivial init logic that's not just registration | Audit before deletion: `cat internal/aws/<svc>_issue_enrichment.go` for each NoOp file. If the file has anything beyond a `registerIssueEnricher(short, NoOpIssueEnricher, prio)` call, defer that file's deletion to a separate cleanup. |
| `cmd/catalogen` runs slowly because it imports the whole `catalog` package | This is fine — the generator runs at build time, not runtime. Slow generators are acceptable; runtime `init()` storms are not. |
