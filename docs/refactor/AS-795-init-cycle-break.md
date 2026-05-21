# AS-795 — init() → catalog cycle-break (prereq for AS-731)

**Parent**: AS-731 (PR-04n: delete `internal/resource/registry.go` and the `Register*` API)
**Sizing**: XL program — split into 15 child PRs (1 scaffold + 12 per-category + 1 Wave 2 cleanup + 1 acceptance gate)
**Stage**: 2 — Spec & Design (this doc)

---

## 1. Problem

AS-731 (PR-04n) asks for `rg '^func init\(\)' internal/aws/ internal/resource/ internal/catalog/` to be zero. Today there are **194 `init()` functions in `internal/aws/`** plus `internal/resource/ct_events_init.go` and `internal/resource/projection_init.go`. Every one of these init() bodies populates a runtime registry in `internal/resource/` (or in `internal/aws/issue_enrichment.go` for Wave 2). The per-category PRs 04b–04m (AS-718..AS-730) populated catalog struct literals with **identity, columns, color, and (partially) children only** — Fetcher, Wave2, Related, Navigable, FieldKeys, FieldAliases, FetchByIDs, FilteredFetcher, child fetchers, Reveal, and DetailEnrich are all still wired at runtime through the `init()` storm.

Verification (read once at spec-publish time):

```text
internal/catalog/types_compute.go:419 — EC2 entry has Color+Augment but no Fetcher/Related/Navigable/Wave2.
internal/aws/ec2.go:14–60 — EC2 init() body calls resource.RegisterFieldKeys/RegisterFieldAliases/RegisterPaginated/RegisterRelated.
internal/aws/ec2_issue_enrichment.go:14–18 — EC2 Wave 2 init() body calls registerIssueEnricher("ec2", EnrichEC2InstanceStatus, 100).
internal/resource/ct_events_init.go:8–14 — calls catalog.RegisterProject("ct-events", ctevent.Project) inside init().
```

The blocker is **a Go import cycle**:

- `internal/catalog` currently imports only `internal/domain`. (Leaf.)
- `internal/resource` imports `internal/catalog` (the wrapper-fallback path).
- `internal/aws` imports `internal/resource` (for `Register*` and type aliases).
- Putting `Fetcher: FetchEC2InstancesPage` directly into the EC2 catalog struct literal would force `internal/catalog` to import `internal/aws` — closing the cycle.

The existing cycle-breaker (`catalog.RegisterProject` called from `internal/resource/ct_events_init.go`'s `init()`) does NOT scale: it still requires an `init()` somewhere, contradicting AS-731's exit criterion.

Deleting `internal/resource/registry.go` (AS-731 step 1) without first migrating the 194 init() bodies into something else immediately breaks the build for ~200 files. AS-731 cannot close without this prereq.

---

## 2. Design — cycle-break pattern

### 2.1 Decision

**Move the catalog data slices (`var <category>Types = []ResourceTypeDef{...}`) out of `internal/catalog/` and into `internal/aws/`. Make `internal/catalog/` a types-only package. Wire the data into `internal/catalog` via a single `aws.Install()` function called once from `cmd/a9s/main.go` and from test bootstrap helpers.**

#### Import graph after the migration

```text
internal/domain/         (leaf — contracts, no a9s imports)
                              ↑
internal/catalog/        (types + registry-state-only — imports domain)
                              ↑
internal/aws/            (transport + per-category catalog data — imports catalog + domain)
                              ↑
internal/resource/       (legacy wrappers, gradually deleted — imports catalog + domain)
                              ↑
internal/tui/, cmd/a9s/  (consumers — call aws.Install() once at startup)
```

The cycle never closes because:

- The forbidden edge `internal/catalog → internal/aws` is never created. `internal/catalog/` stays a pure types package.
- The new edge `internal/aws → internal/catalog` is acyclic. `internal/catalog` does not import `internal/aws`, `internal/resource`, or anything from `internal/aws`.
- `internal/aws/types_compute.go` (the relocated data file) declares `var computeTypes = []catalog.ResourceTypeDef{ {ShortName: "ec2", Fetcher: FetchEC2InstancesPage, ...} }` — the fetcher symbol resolves in the same package, no setter dance per type.

#### Catalog API surface change

`internal/catalog/catalog.go` flips from `var ResourceTypes = allTypes()` to a setter-backed registry:

```go
// internal/catalog/catalog.go (post-AS-795a)
package catalog

var (
    registry      []ResourceTypeDef                  // set by SetTypes
    childRegistry = map[string]ResourceTypeDef{}     // set by SetChildTypes
)

// SetTypes installs the top-level catalog. MUST be called exactly once at
// program start (main() / TestMain) BEFORE any Find/All/ByCategory call.
// Idempotent on identical input; panics on a second call with different data.
func SetTypes(types []ResourceTypeDef) { ... }

// SetChildTypes installs the child-type catalog. Same lifecycle as SetTypes.
func SetChildTypes(children []ResourceTypeDef) { ... }

func Find(name string) *ResourceTypeDef { /* read registry */ }
func All() []ResourceTypeDef            { /* read registry */ }
func ByCategory(cat string) []ResourceTypeDef { /* read registry */ }
func AllShortNames() []string           { /* read registry */ }
func FindChild(name string) *ResourceTypeDef { /* read childRegistry */ }
```

`internal/catalog/RegisterProject` and `RegisterAugment` (today's setter pattern in `catalog.go:80–101`) are **deleted in AS-795a** — replaced by `Project` and `Augment` being explicit fields on the catalog struct literal, populated in `internal/aws/types_<category>.go`. The single existing call site (`internal/resource/ct_events_init.go`) is also deleted (file removed) once the ct-events catalog literal carries `Project: ctevent.Project` directly.

#### aws.Install()

```go
// internal/aws/install.go (new in AS-795a)
package aws

import "github.com/k2m30/a9s/v3/internal/catalog"

// Install loads the AWS resource catalog into internal/catalog. MUST be called
// once at program start (main() / TestMain) before any catalog.Find/All call.
// Replaces every init() that previously populated internal/resource/ registries.
func Install() {
    catalog.SetTypes(allTopLevelTypes())
    catalog.SetChildTypes(allChildTypes())
}

func allTopLevelTypes() []catalog.ResourceTypeDef {
    var all []catalog.ResourceTypeDef
    all = append(all, computeTypes...)
    all = append(all, containersTypes...)
    all = append(all, networkingTypes...)
    all = append(all, databasesTypes...)
    all = append(all, monitoringTypes...)
    all = append(all, messagingTypes...)
    all = append(all, secretsTypes...)
    all = append(all, dnsCdnTypes...)
    all = append(all, securityTypes...)
    all = append(all, cicdTypes...)
    all = append(all, dataTypes...)
    all = append(all, backupTypes...)
    return all
}
```

Test bootstrap (TestMain in each test binary that touches the catalog) gains a one-liner:

```go
func TestMain(m *testing.M) {
    aws.Install()
    os.Exit(m.Run())
}
```

A handful of unit tests already use `tests/unit/testmain_test.go` or equivalent; we add `aws.Install()` to whichever TestMain a given package owns. Where no TestMain exists, we add one in AS-795a. This is mechanical.

### 2.2 ResourceTypeDef field additions

The current struct (`internal/catalog/types.go`) covers Fetcher, Wave2, Project, Related, Navigable, Children, Reveal, DetailEnrich. The fields needed to absorb the rest of the init()-storm:

| New field | Type | Replaces |
|---|---|---|
| `FieldKeys []string` | `[]string` | `resource.fieldKeyRegistry` |
| `FieldAliases map[string]string` | `map[string]string` | `resource.fieldAliasBuiltins` |
| `FetchByIDs domain.FetchByIDsFunc` | function | `resource.fetchByIDsRegistry` |
| `FilteredFetcher domain.FilteredPaginatedFetcher` | function | `resource.filteredPaginatedRegistry` |
| `ChildFetcher domain.PaginatedChildFetcher` (on child type defs only) | function | `resource.paginatedChildRegistry` |
| `IssueEnricherFieldKeys []string` | `[]string` | `resource.issueEnricherFieldKeysRegistry` |

`Wave 2` (`Wave2 any`) stays `any` to avoid a `catalog → aws` import (the concrete type is `*aws.IssueEnricher`). `internal/aws/types_<category>.go` populates with the concrete pointer; consumers in `internal/tui/` type-assert to `*aws.IssueEnricher`.

`Wave 2 priority` (today's `IssueEnricher.Priority`) lives on the `*aws.IssueEnricher` struct already; the catalog stores the struct, not the function. No new field needed.

### 2.3 Why not the alternatives

| Option | Verdict | Why |
|---|---|---|
| **Option A — Data in `internal/aws/`** (chosen) | ✅ | Fewest packages; literal references resolve in-package; "mechanical-resource-implementation" criterion stays "one struct literal + one transport file"; the 04-catalog "aws MUST NOT import catalog" constraint was defensive under the old "data in catalog" model — inverting it is internally consistent. |
| Option B — New `internal/wiring/` package | ❌ | Adds a third package whose only job is concatenating slices. The literals in `internal/wiring/types_<cat>.go` reference symbols from `internal/aws/` across the package boundary, weakening the "definition adjacent to fetcher" property. The 16-files-to-relocate cost is the same as Option A. |
| Option C — Keep `Register*` setters; call them all from `main()` | ❌ | Trades 194 `init()` calls for 194 `Register*` calls in `main()`. Loses compile-time correctness (registration order matters, missed calls silently produce nil fetchers). Does not reach "catalog struct literal = source of truth" — keeps the dual-source-of-truth this refactor exists to remove. |

The 04-catalog plan's risk register (line 413) already anticipated this exact pattern: *"Type definitions referenced from both sides (`PaginatedFetcher`, `IssueEnricher`, `RelatedDef`, `NavigableField`) live in the leaf `internal/domain/` package."* That invariant holds; Option A only changes where the **data slice** lives, not where the **types** live.

### 2.4 Trade-offs accepted

- `internal/aws/` grows from 359 files to 371 (+12 `types_<cat>.go` files). It is already the largest package by file count. Mitigation: rename the relocated files `internal/aws/catalog_<cat>.go` (instead of `types_<cat>.go`) so the file naming makes the layering explicit. The `var <cat>Types` slices stay lowercase / unexported.
- Test binaries that touch `catalog.Find` / `catalog.All` MUST call `aws.Install()` first. Tests that don't reference the catalog stay unaffected. This is enforced by a panic in `catalog.Find` when `registry` is nil (AS-795a) so a missing `aws.Install()` fails loudly the first time the test calls a catalog accessor, not silently with "type not found."
- `aws.Install()` runs in O(N) at startup — N = 194 top-level + child types — to flatten the per-category slices into the shared registry. This is sub-millisecond; not a concern.

---

## 3. PR breakdown

| PR | Title | Scope | Size |
|---|---|---|---|
| **AS-795a** | Cycle-break scaffold | (1) Move all 12 `internal/catalog/types_<cat>.go` files into `internal/aws/catalog_<cat>.go`. (2) Replace `internal/catalog/catalog.go`'s `var ResourceTypes = allTypes()` with the SetTypes/SetChildTypes API. (3) Add `internal/aws/install.go` with `Install()`. (4) Wire `aws.Install()` into `cmd/a9s/main.go` and into all test TestMains that need it. (5) Add new fields (`FieldKeys`, `FieldAliases`, `FetchByIDs`, `FilteredFetcher`, `IssueEnricherFieldKeys`, `ChildFetcher`) to `ResourceTypeDef` — populated as zero values for now. (6) Delete `internal/catalog/catalog.go:RegisterProject/RegisterAugment`; delete `internal/resource/ct_events_init.go` (the ct-events catalog literal in the new compute file gets `Project: ctevent.Project` directly). (7) **No init() in `internal/aws/` migrated yet** — all 194 init() bodies continue to register into `internal/resource/` registries. The catalog stays partially populated (identity/columns/color only). `make ready-to-push` clean. | **L** (~800 LOC: 12 file moves + 200 LOC new wiring + 100 LOC test-bootstrap edits + 12 ResourceTypeDef field additions + zero init() migrations) |
| AS-795b | compute category migration | For each compute type (ec2, ecs, ecs-svc, asg, ami, ami-snap, lambda, ng): migrate Fetcher, Wave2, Related, Navigable, FieldKeys, FieldAliases, FetchByIDs, RevealFetcher, DetailEnrich, FilteredFetcher into the catalog struct literal in `internal/aws/catalog_compute.go`. Delete the init() body in each `internal/aws/<svc>.go`. Delete `internal/aws/<svc>_issue_enrichment.go` files whose only content was init()+NoOp. Verify `rg '^func init\(\)' internal/aws/{ec2,ecs,ecs-svc,asg,ami,lambda,ng}*.go` → zero. | M-L (~800 LOC) |
| AS-795c | containers category | Same pattern for containers (eks, ecr, ecr-images). | M (~400 LOC) |
| AS-795d | networking category | Same pattern for networking (vpc, sg, subnet, eni, igw, nat, rtb, eip, elb, tg, tg-health, tgw, vpce, r53, r53-records). | L (~1200 LOC, large category) |
| AS-795e | databases category | Same pattern (rds, dbi, dbc, dbi-snap, dbc-snap, redis, ddb, redshift, opensearch). | M-L (~900 LOC) |
| AS-795f | monitoring category | Same pattern (alarm, logs, log-streams, log-events, cwlogs, trail, ct-events). | M (~600 LOC) |
| AS-795g | messaging category | Same pattern (sns, sns-sub, sqs, msk, kinesis, eb, eb-rule, eb-rule-targets, sfn). | M-L (~800 LOC) |
| AS-795h | secrets category | Same pattern (secrets, kms, acm, ssm, ses). | M (~500 LOC) |
| AS-795i | dns-cdn category | Same pattern (cf, apigw, waf). | S-M (~300 LOC) |
| AS-795j | security category | Same pattern (iam-roles, iam-policies, iam-users, iam-groups, iam-group-members, iam-role-policies). | M (~600 LOC) |
| AS-795k | cicd category | Same pattern (cb, cb-builds, cb-build-logs, pipeline, pipeline-stages, codeartifact). | M (~500 LOC) |
| AS-795l | data category | Same pattern (s3, athena, glue, glue-runs, ebs, ebs-snap). | M (~500 LOC) |
| AS-795m | backup category | Same pattern (backup, cfn, cfn-events, cfn-resources). | M (~400 LOC) |
| AS-795n | Wave 2 enricher infrastructure cleanup | Replace `internal/aws/issue_enrichment.go:IssueEnricherRegistry` with a derived view: a function `catalog.AllByWave2() []*aws.IssueEnricher` that iterates `catalog.All()` filtering by `Wave2 != nil`. Delete `registerIssueEnricher`. Update consumers: `internal/tui/runtime_adapter_navigate.go:495`, `internal/tui/probe_adapter.go:164`, and `internal/tui/probe_enrichment_cache_test.go:54–58`. Delete all `internal/aws/*_issue_enrichment.go` files whose only remaining content was init()+NoOp (most already gone after 04b–m; this is the final sweep). | M (~400 LOC) |
| **AS-795o** | Acceptance gate | Assert `rg '^func init\(\)' internal/aws/ internal/resource/ internal/catalog/` is zero. Assert `rg 'Fetcher:' internal/aws/catalog_*.go | wc -l` matches the count of top-level resource types. Assert `rg 'Wave2:' internal/aws/catalog_*.go` covers every type (explicit `Wave2: nil` or real `&IssueEnricher{...}`). Run `make ready-to-push`, `./a9s --demo` smoke. Hand off to AS-731 by posting "AS-795 prereq complete" + linking AS-795o's merged PR. | S (~100 LOC + verification) |

**Parallelism.** AS-795b through AS-795m can run in parallel after AS-795a lands (different category files, no shared edits). AS-795n must serialize after the category PR that owns its last init() (typically AS-795l data or AS-795m backup). AS-795o serializes after AS-795n.

**Dependency edges**: AS-795a blocks AS-795b–m; AS-795b–m block AS-795n; AS-795n blocks AS-795o; AS-795o unblocks AS-731.

---

## 4. Exit criteria

Per the issue body (verbatim) plus a few derived gates:

```bash
# Original acceptance from issue body:
rg 'Fetcher:' internal/aws/catalog_*.go      # every top-level entry has a non-nil Fetcher
rg 'Related:' internal/aws/catalog_*.go      # every entry needing related panels has it populated
rg 'Wave2:' internal/aws/catalog_*.go        # every entry has explicit Wave2 (nil or &IssueEnricher{...})
go build ./... && make test                  # clean before AS-731 picks up

# Derived gates:
rg '^func init\(\)' internal/aws/ internal/resource/ internal/catalog/   # zero hits
rg 'resource\.RegisterPaginated|resource\.RegisterRelated|resource\.RegisterNavigableFields|resource\.RegisterDefaultNavFields|resource\.RegisterFetchByIDs|resource\.RegisterPaginatedChild|resource\.RegisterFilteredPaginated|resource\.RegisterRevealFetcher|resource\.RegisterDetailEnricher|resource\.RegisterChildType|resource\.RegisterFieldKeys|resource\.RegisterFieldAliases' internal/aws/    # zero hits (all migration scopes done)
rg 'registerIssueEnricher' internal/aws/    # zero hits (AS-795n done)
rg 'IssueEnricherRegistry\[' internal/    # zero hits — consumers use catalog.AllByWave2()
make ready-to-push                                                       # passes
./a9s --demo                                                             # all 12 categories list+detail correctly
```

The `make test` and `./a9s --demo` gates are the parity proof — behavior unchanged for every resource type before vs after AS-795o.

---

## 5. Scoped handoffs

Per the SCOPE GATE protocol, AS-795a's QA + Coder handoffs are below. The per-category PRs (AS-795b–m) get their own scoped handoffs dispatched after AS-795a lands; their shape is mechanical and follows the AS-795b template once AS-795a's scaffold is in place.

### 5.1 AS-795a Coder dispatch (scaffold)

```text
Task: Cycle-break scaffold for AS-795 — relocate catalog data to internal/aws/ and introduce aws.Install()
Mode: execute
Files to create:
  - internal/aws/install.go        :: aws.Install(), allTopLevelTypes(), allChildTypes()
  - internal/aws/catalog_compute.go    (moved from internal/catalog/types_compute.go)
  - internal/aws/catalog_containers.go (moved from internal/catalog/types_containers.go)
  - internal/aws/catalog_networking.go (moved from internal/catalog/types_networking.go)
  - internal/aws/catalog_databases.go  (moved from internal/catalog/types_databases.go)
  - internal/aws/catalog_monitoring.go (moved from internal/catalog/types_monitoring.go)
  - internal/aws/catalog_messaging.go  (moved from internal/catalog/types_messaging.go)
  - internal/aws/catalog_secrets.go    (moved from internal/catalog/types_secrets.go)
  - internal/aws/catalog_dns_cdn.go    (moved from internal/catalog/types_dns_cdn.go)
  - internal/aws/catalog_security.go   (moved from internal/catalog/types_security.go)
  - internal/aws/catalog_cicd.go       (moved from internal/catalog/types_cicd.go)
  - internal/aws/catalog_data.go       (moved from internal/catalog/types_data.go)
  - internal/aws/catalog_backup.go     (moved from internal/catalog/types_backup.go)
Files to modify:
  - internal/catalog/catalog.go :: replace `var ResourceTypes = allTypes()` + `allTypes()` with `var registry []ResourceTypeDef`, `SetTypes`, `SetChildTypes`, `FindChild`. Delete `RegisterProject` and `RegisterAugment` (no longer needed).
  - internal/catalog/types.go :: add fields `FieldKeys []string`, `FieldAliases map[string]string`, `FetchByIDs domain.FetchByIDsFunc`, `FilteredFetcher domain.FilteredPaginatedFetcher`, `IssueEnricherFieldKeys []string`, `ChildFetcher domain.PaginatedChildFetcher`. Document each as "populated by aws.Install(); zero value if no Wave 1/2 surface."
  - cmd/a9s/main.go :: add `aws.Install()` call as the first non-flag statement in main().
  - tests/unit/*_test.go and tests/unit/testmain_test.go :: in every TestMain (or add one if absent), call `aws.Install()` before `os.Exit(m.Run())`. Audit via `grep -l TestMain tests/unit/` and edit accordingly. Where the test package has no TestMain and a test references `catalog.Find` / `catalog.All`, add a TestMain that calls Install.
Files to delete:
  - internal/resource/ct_events_init.go (ct-events catalog literal in catalog_monitoring.go gets `Project: ctevent.Project` directly — that file becomes the new home)
Files to modify but leave init() bodies intact:
  - internal/aws/*.go (194 files) :: DO NOT migrate init() bodies in this PR. They continue to register into internal/resource/ legacy maps. The wrapper-fallback path in internal/resource/registry.go remains the runtime source for Fetcher/Related/etc. until AS-795b–m.
Files explicitly NOT touched (read-only):
  - internal/resource/registry.go, related.go, navigation.go (legacy wrappers stay; deleted in AS-731 after AS-795o)
  - internal/tui/* (consumers stay on resource.Get* wrappers)
Expected behavior:
  - `make build && make test && make lint && make security && make gofix` pass clean.
  - `rg '^func init\(\)' internal/catalog/` returns zero hits (was already zero; verify).
  - `rg '^func init\(\)' internal/resource/` returns one hit (`projection_init.go`) — staying until AS-795n.
  - `rg '^func init\(\)' internal/aws/` returns 194 hits — unchanged in this PR.
  - `catalog.Find("ec2")` returns the same struct (identity/columns/color) it did before; Fetcher/Wave2/Related/Navigable still nil because not yet migrated.
  - Catalog struct fields FieldKeys / FieldAliases / FetchByIDs / FilteredFetcher / IssueEnricherFieldKeys / ChildFetcher exist but are zero values for every entry; per-category PRs populate them.
  - `aws.Install()` is idempotent on identical input; panics on second call with different data (defensive — catches double-install bugs in tests).
  - `catalog.Find` / `catalog.All` panic with a clear message if `SetTypes` has not yet been called (catches missing aws.Install in a test binary).
Verification:
  - make ready-to-push
  - go run ./cmd/a9s -- --demo  ←  smoke; every list view renders, every detail view opens.
Exit: tests pass; gates green locally; comment on AS-795a with what changed and verification output.
```

### 5.2 AS-795a QA dispatch (scaffold)

```text
Task: Test coverage for AS-795a cycle-break scaffold
Mode: score
Test files to create:
  - tests/unit/catalog_install_test.go :: package external, tests aws.Install + catalog.SetTypes
Test files to modify:
  - tests/unit/testmain_test.go :: add aws.Install() to TestMain (if not present)
What to test:
  - aws.Install() is callable from a TestMain; catalog.Find("ec2") returns a non-nil entry afterwards.
  - aws.Install() is idempotent on identical input (calling twice does not panic, does not duplicate entries).
  - catalog.SetTypes panics or errors on a second call with a *different* slice (defensive against accidental re-install with different data).
  - catalog.Find / catalog.All panic with a clear message ("catalog.SetTypes not called") when invoked before SetTypes — test using a fresh sub-process or a dedicated test binary; OR move this assertion into a build-time test that imports only catalog without aws.Install (so the panic path is exercised).
  - For one type per category (12 types — ec2, eks, vpc, rds, alarm, sns, secrets, cf, iam-roles, cb, s3, backup): catalog.Find(short) returns a non-nil entry with the same Name/ShortName/Category/Columns it had before the refactor (golden table).
  - For every type in catalog.All(): the entry has zero-value Fetcher/Related/Navigable/Wave2 (NOT yet migrated — those land in AS-795b–m).
  - Tests EXPLICITLY NOT in scope for AS-795a: per-category Fetcher/Wave2/Related/Navigable parity (those land per category in AS-795b–m).
Type signatures (so QA can compile without exploring):
  - func aws.Install()
  - func catalog.SetTypes(types []catalog.ResourceTypeDef)
  - func catalog.SetChildTypes(children []catalog.ResourceTypeDef)
  - func catalog.Find(name string) *catalog.ResourceTypeDef
  - func catalog.All() []catalog.ResourceTypeDef
  - func catalog.FindChild(name string) *catalog.ResourceTypeDef
  - struct catalog.ResourceTypeDef { ShortName, Name, Category, Columns, ..., FieldKeys, FieldAliases, FetchByIDs, FilteredFetcher, IssueEnricherFieldKeys, ChildFetcher, ... }
Acceptance: failing tests committed to the AS-795a feature branch; coder makes them pass in execute mode.
```

The per-category PRs (AS-795b–m) get their own scoped handoffs published as a comment on the corresponding child issue after AS-795a lands. Each one follows the template:

```text
Files to modify:
  - internal/aws/catalog_<cat>.go :: populate Fetcher/Wave2/Related/Navigable/FieldKeys/FieldAliases/FetchByIDs/RevealFetcher/DetailEnrich/FilteredFetcher/ChildFetcher for every entry in the slice.
  - internal/aws/<svc>.go (every file in this category): delete the init() body; delete the import of internal/resource if init() was the only consumer.
  - internal/aws/<svc>_issue_enrichment.go: if file only contains init()+NoOp, delete entire file; if it has a real Enrich* function, keep the function and delete the init() body.
Acceptance: rg '^func init\(\)' internal/aws/<cat-services>*.go is zero; behavior parity with main branch verified via ./a9s --demo and snapshot tests.
```

---

## 6. Risks

| Risk | Mitigation |
|---|---|
| `aws.Install()` not called in a test binary → `catalog.Find` panics. | AS-795a adds `aws.Install()` to every TestMain that touches `catalog.Find` / `catalog.All` / `resource.FindResourceType`. CI catches any missed TestMain by panic on first test invocation. Documenting the rule in `CLAUDE.md` for future test authors. |
| Test parallelism: `aws.Install()` is package-level state. | `aws.Install()` is idempotent on identical input. Tests that modify the registry (rare — only `tests/unit/probe_enrichment_cache_test.go:54–58`) use the existing snapshot+restore idiom; same idiom applies post-AS-795n. |
| Per-category PRs (b–m) collide with concurrent edits to `internal/aws/`. | Each category PR touches only its own catalog file + the init()-only files for that category. No cross-category writes. Pre-PR audit: `git diff --stat` confirms ≤ N+1 files (catalog_<cat>.go + N svc files). |
| Wave 2 enricher migration (AS-795n) breaks consumers in `internal/tui/`. | AS-795n's scope explicitly lists the three consumer sites: `runtime_adapter_navigate.go:495`, `probe_adapter.go:164`, `probe_enrichment_cache_test.go:54–58`. The migration replaces the map lookup with `catalog.Find(rt).Wave2.(*aws.IssueEnricher)`. Tests assert identical Wave 2 dispatch order pre/post. |
| Catalog struct grows large (~25 fields). | The 04-catalog risk register (line 412) already accepted this. Field grouping in comments (Identity / Display / Behavior / Cross-cutting / Color&Augmentation / Findings) keeps the struct literal readable. No nested sub-structs in this program. |
| `make ready-to-push` time grows because the binary now does catalog.SetTypes at startup. | Sub-millisecond — 194 entries flattened into a shared slice. Negligible. |

---

## 7. Cross-references

- **`docs/refactor/04-catalog.md`** — Phase 04 plan; this AS-795 program fills in the runtime-wiring migration that PR-04b–m left undone.
- **`docs/architecture.md`** — Single-source-of-truth doc; AS-731 is part of the architecture-intent conformance program. After AS-795o lands, the catalog struct literal IS the single source of truth, matching the architecture intent.
- **AS-731** — Blocked by this program; mechanical after AS-795o.
- **AS-722 / AS-727 / AS-724** — Precedent for "category catalog PR closes done after types-file present + legacy deleted + build/test green"; AS-795b–m apply the same closure rule.

## 8. Out of scope

- Renaming `internal/aws/` → `internal/transport/`. The 04-catalog plan explicitly forbids it (line 395 of `04-catalog.md`); AS-795 inherits the same prohibition.
- Removing `internal/resource/` entirely. AS-731 owns that. AS-795 stops at "every init() is gone; every Register* call site is gone in `internal/aws/`."
- Capability modules (logs, cost, actions). Phase 05 owns those.
- Generated Go code. `cmd/catalogen` (markdown only) is unchanged. No new codegen.
