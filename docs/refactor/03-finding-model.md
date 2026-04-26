# Phase 03 — Canonical finding model

**16 PRs. Mandatory. Depends on Phase 01 (`Severity` enum) and Phase 02 (`Session` owner).**

## Goal

Replace `Resource.Status` + `Resource.Issues []string` + `(+N)` suffix algebra + `EnrichmentFinding{Severity, Summary, Rows}` with a two-layer canonical model:

- **`Resource.Findings []Finding`** — canonical row/menu/status semantics. Drives row coloring, list-view Status display, menu issue badges, ctrl+z attention filter.
- **`Resource.AttentionDetails map[FindingCode]AttentionDetail`** — supporting structured facts for each finding. Consumed only by the detail view's Attention section.

Lifecycle state (running/stopped/available/deleted) lives in `Resource.Fields[td.LifecycleKey]`, never in a `Resource.Status` field.

After this phase, the contract on the current `EnrichmentFinding` struct (lines 16–35 of `internal/resource/enrichment.go`) — *"Summary must NEVER embed Row content; every fact lives in exactly one place"* — becomes structural rather than reviewer-enforced. `Finding.Phrase` is a single `string` and `AttentionDetail.Rows` is a typed `[]DetailRow`; you can't smuggle one into the other.

## Domain types

```go
// internal/domain/finding.go

// FindingCode is a stable identifier — never displayed.
// Allocation: typed constants per enricher in Phase 03 (option b);
// graduates to a declarative table on ResourceTypeDef in Phase 04 (option c).
type FindingCode string

type Finding struct {
    Code     FindingCode  // stable; keys AttentionDetails
    Phrase   string       // §4 lowercase phrase, display only
    Severity Severity     // domain enum from Phase 01
    Source   string       // provenance: "wave1" | "wave2:<short>" — for tests/audits
}

// internal/semantics/attention/types.go (or similar — placement near projector)

type AttentionDetail struct {
    Rows []DetailRow
}

type DetailRow struct {
    Label string
    Value string
    Tier  string  // "!" | "~" | "" — display tier; empty inherits from Finding.Severity
}
```

`Resource` becomes:

```go
type Resource struct {
    ID               string
    Name             string
    Findings         []Finding
    AttentionDetails map[FindingCode]AttentionDetail
    Fields           map[string]string
    RawStruct        any
}

// Display rule:
//   list view: r.Findings[0].Phrase if non-empty, else r.Fields[td.LifecycleKey]
//   color:     r.Findings[0].Severity if non-empty, else lifecycleSeverity(r.Fields[td.LifecycleKey])
```

`ResourceTypeDef` gains:

```go
type ResourceTypeDef struct {
    // ...
    LifecycleKey string  // "state" by default; types override (e.g. some use "status", "lifecycle", "phase")
}
```

## PR breakdown

16 PRs total: 3 setup PRs (split for reviewability) + 12 per-category cutover + 1 cleanup.

### Resource entry points — the shim must cover every one

Resources enter view-visible state through several distinct paths. PR-03a-shim must invoke `DeriveFindings` at *each* of these boundaries before resources reach views; otherwise the "no view code reads `r.Status`" exit criterion is unsafe for cached/probe/related resources:

| Entry point | Source | Where to apply shim |
|---|---|---|
| Top-level fetcher result | `app_fetchers.go` `LoadResourcesMsg` handler → `ResourcesLoadedMsg` | wrap `[]Resource` before publishing into resource cache |
| Wave 1 probe retention | `app_handlers_availability.go:124` `maps.Copy(m.probeResources, msg.Resources)` | derive across `msg.Resources` before `maps.Copy` |
| Cold-miss related prefetch (`CachedPages`) | `app.go:485` `m.resourceCache[shortName] = &resourceCacheEntry{resources: entry.Resources, ...}` | derive across `entry.Resources` before storing |
| Lazy-add (`LazyAddedResources`) | `app.go:511` writes into `m.lazyResourceCache` | derive across each `extra` slice before storing |
| Child-view fetcher result | `app_handlers_navigate.go` child fetch path | derive before passing to child `ResourceListModel` |
| Detail enricher result (when it returns extra resources) | `EnrichDetailResultMsg` handler | derive before merging into resource state |

Each entry point gets exactly one `attention.DeriveFindings` call. The shim is **idempotent**: calling it on a resource whose `Findings` is already populated is a no-op (early-return). After PR-03a-shim, the only ways a resource can reach view state without `Findings` populated are via direct test construction (acceptable; tests opt in) or via a missed entry point (a bug — covered by PR-03a-views' exit grep).

### PR-03a-types — Domain and projection types only

**Goal.** Add the new types. No consumers, no shim, no view changes. Purely additive; the codebase compiles and behaves identically because nothing reads the new fields.

**Files added**

- `internal/domain/finding.go` — `FindingCode`, `Finding`
- `internal/semantics/attention/types.go` — `AttentionDetail`, `DetailRow`

**Files modified**

- `internal/resource/resource.go` — add `Findings []Finding` and `AttentionDetails map[FindingCode]AttentionDetail` fields. `Status` and `Issues` stay untouched.
- `internal/resource/types.go` — add `LifecycleKey string` field to `ResourceTypeDef`.
- All 12 `internal/resource/types_*.go` files — set `LifecycleKey` on every type def. Empty string defaults to `"state"`. Audit which types use a non-default key.

**Exit criteria**

```bash
# Fields exist on Resource:
rg 'Findings\s+\[\]\w+\.Finding|AttentionDetails\s+map' internal/resource/resource.go
# expected: present

# LifecycleKey is set (or explicitly empty-default) on every type def:
rg 'LifecycleKey:' internal/resource/types_*.go | wc -l
# expected: matches the count of registered top-level types (or every non-default case)

go build ./...
# expected: clean

make test
# expected: passes — types are unread, no behavior change
```

**Independently revertable**: yes. Reverting drops the type fields; nothing else depends on them.

---

### PR-03a-shim — Idempotent derive function, applied at every entry point

**Goal.** Add the one-way derive shim and wire it into every entry point listed above. After this PR, every `Resource` reaching view state has `Findings` and `AttentionDetails` populated — derived from legacy `Status` + `Issues` + `EnrichmentFinding`. View code does not yet read the new fields; consumer migration is PR-03a-views.

**Files added**

- `internal/semantics/attention/derive.go` — `func DeriveFindings(r *Resource, td ResourceTypeDef)`. Reads `r.Status`, `r.Issues`, and any cached `EnrichmentFinding`. Synthesizes a `FindingCode` from a per-type lookup table built before the per-category PR for that type begins. **Never writes back to `Status`/`Issues`. Idempotent: returns immediately if `r.Findings` is already populated.**

**Files modified**

- `internal/tui/app_fetchers.go` — wrap fetcher results: derive across `[]Resource` before publishing.
- `internal/tui/app_handlers_availability.go` (around line 124) — derive across `msg.Resources` before `maps.Copy(m.probeResources, ...)`.
- `internal/tui/app.go` (around line 485) — derive across `entry.Resources` from `CachedPages` before writing to `m.resourceCache`.
- `internal/tui/app.go` (around line 511) — derive across `extra` from `LazyAddedResources` before writing to `m.lazyResourceCache`.
- `internal/tui/app_handlers_navigate.go` — child-view fetcher path: derive before passing resources to the child list.
- `internal/tui/app_enrich.go` — `EnrichDetailResultMsg` path: derive before merging the enriched resource.

**Exit criteria**

```bash
# Every entry point calls DeriveFindings:
rg 'attention\.DeriveFindings\b' internal/tui/
# expected: exactly six call sites (one per entry-point row above; commit message must list them)

# Shim itself is idempotent:
rg 'len\(r\.Findings\) > 0' internal/semantics/attention/derive.go
# expected: present (the early-return guard)

# No code path injects resources into a cache without going through derive:
# Manual audit: grep for assignments into m.resourceCache, m.lazyResourceCache, m.probeResources
# Each must be preceded (or wrapped) by DeriveFindings. Document each in PR description.
```

**Independently revertable**: yes. Reverting removes the shim calls; resources reach views without `Findings`, but views still read `Status`/`Issues` (PR-03a-views hasn't landed yet), so behavior is unchanged.

---

### PR-03a-views — Switch all view-side reads to the new model

**Goal.** Convert every `r.Status` / `r.Issues` / direct `EnrichmentFinding` read in `internal/tui/` to use `Findings` and `AttentionDetails`. After this PR, no view code reads the legacy fields. Fetchers and enrichers still *write* `Status`/`Issues` (the per-category cutover migrates them); the shim from 03a-shim ensures `Findings` is populated by the time views read it.

**Files modified**

- `internal/tui/views/resourcelist.go`, `resourcelist_helpers.go`, `table_render.go` — list-view Status column and color logic read `Findings[0].Phrase` / `Findings[0].Severity`, falling back to `Fields[td.LifecycleKey]`.
- `internal/tui/views/detail.go`, `detail_fields.go`, `detail_helpers.go`, `rightcolumn.go` — Attention section reads `r.AttentionDetails[code]` for each `r.Findings[i]`.
- `internal/tui/views/mainmenu.go` — issue-count badges count `len(filter(r.Findings, IsIssue))`.
- `internal/tui/views/attention.go` — filter predicate reads `Findings`.
- `tests/unit/` — update every test that asserts on `r.Status` content. Concrete files: `resourcelist_*_test.go`, `detail_*_test.go`, `mainmenu_*_test.go`, `attention_*_test.go`, `qa_*_test.go`. **Test migration is in scope for this PR.** Counted budget: ~30 test files, each touching 1–5 assertions. Estimated test diff: 200–400 lines.

**Exit criteria**

```bash
# View code does not read Resource.Status:
rg '\br?\.Status\b|\bres\.Status\b' internal/tui/
# expected: zero hits

# View code does not read Resource.Issues:
rg '\br?\.Issues\b' internal/tui/
# expected: zero hits

# View code does not read EnrichmentFinding directly:
rg 'EnrichmentFinding|\.Summary\b|\.Rows\b' internal/tui/
# expected: zero hits in non-test code

# Tests pass without legacy field reads:
rg 'r\.Status|res\.Status|\.Issues' tests/unit/
# expected: zero hits in updated test files
```

Behavior verification:
- `./a9s --demo` produces visually identical list and detail output to pre-Phase-03 output. **Snapshot-test harness:** before this PR lands, capture canonical-resource detail/list renders for ec2, s3, rds, iam-role, alarm, sg via `cmd/preview-detail` (or similar) into `tests/testdata/snapshots/<short>.txt`. PR-03a-views' exit gate includes: regenerated snapshots match committed snapshots byte-for-byte. The same harness is reused in every PR-03b-m to detect regressions per category.
- `make test` and `make test-race` pass.

**Independently revertable**: yes — but reverting requires reverting tests/unit/ updates too. Cleaner: revert the full PR atomically.

---

### PR-03b through PR-03m — Per-category cutover

12 PRs, one per service-category file in `internal/resource/types_*.go`:

| PR | Category | Resource types affected |
|---|---|---|
| 03b | compute | ec2, lambda, eks, asg, eb (elastic beanstalk), ebs, ami, eip, eni, ebs-snap |
| 03c | containers | ng (eks node groups), ecs, ecs-svc, ecs-task |
| 03d | networking | vpc, subnet, sg, elb, tg, igw, nat, rtb, acm, apigw, waf, eni, vpce, tgw |
| 03e | databases | rds (dbi/dbi-snap/dbc/dbc-snap), s3, redis, opensearch, ddb, redshift, msk, efs, kinesis |
| 03f | security | role, iam-user, policy, iam-group, kms |
| 03g | secrets | secrets, ssm |
| 03h | monitoring | alarm, logs (log groups), trail (cloudtrail), ct-events |
| 03i | messaging | sns, sqs, eb-rule (eventbridge), sfn, sns-sub |
| 03j | cicd | cfn (cloudformation), pipeline (codepipeline), cb (codebuild), ecr, codeartifact |
| 03k | dns_cdn | r53, cf (cloudfront) |
| 03l | data | athena, glue |
| 03m | backup | backup, ses |

**Per-PR scope (template).** Each per-category PR:

1. Updates every fetcher in `internal/aws/<category-services>.go` to populate `Resource.Findings` and `Resource.AttentionDetails` directly. Stops setting `Resource.Status` and `Resource.Issues`.
2. Updates every Wave 2 issue enricher in `internal/aws/<svc>_issue_enrichment.go` to append to `Findings` and write to `AttentionDetails[code]`. Stops calling `BumpFindingSuffix` on `Status`. Stops returning `EnrichmentFinding` — returns `[]Finding` + `map[FindingCode]AttentionDetail` updates.
3. Updates every `Color` function in the corresponding `types_<category>.go` to read `Findings[0].Severity` first, falling back to `lifecycleSeverity(r.Fields[td.LifecycleKey])`. Drops any `r.Status` reads.
4. Declares `FindingCode` constants in `internal/aws/<svc>_codes.go` (a sibling file in the same package as the fetcher). Per-service namespacing: `awsclient.CodeEC2Impaired = "ec2.impaired"`, `awsclient.CodeRDSMaintPending = "rds.maint.pending"`, etc. **The constants stay in `internal/aws/` for the entire program** — the speculative `internal/aws/` → `internal/transport/` rename is out of scope. If that rename ever happens, it's a single `gofmt`-style refactor across the package, post-program.
5. The idempotent shim continues covering any unmigrated path (the early-return condition `len(r.Findings) > 0` is what makes migrated types bypass the shim automatically).
6. Updates the corresponding tests in `tests/unit/` — typically the per-resource fetcher tests (`<svc>_test.go`), Wave 2 enricher tests (`<svc>_issue_enrichment_test.go`), and any view tests scoped to the migrated category. **Test migration is in scope.** Estimated per-category test diff: 5–15 test files modified, 100–300 lines.

**Per-PR exit criteria.** For category `<X>` in PR-03<X>:

```bash
# No fetcher in <X> writes Status:
rg 'Status:\s*\w' internal/aws/<x-services>*.go
# expected: zero hits

# No enricher in <X> uses BumpFindingSuffix:
rg 'BumpFindingSuffix|StripFindingSuffix|SplitFindingSuffix' internal/aws/<x-services>*_issue_enrichment.go
# expected: zero hits

# Color funcs in this category don't read Status:
rg '\br\.Status\b' internal/resource/types_<x>.go
# expected: zero hits

# FindingCode constants exist for every enricher in <X>:
ls internal/aws/<svc>_codes.go
# expected: present for every service in the category that has a Wave 2 enricher

# Snapshot tests for every resource type in this category match (regenerate then diff):
go test ./tests/unit/ -run TestSnapshot_<X> -count=1
# expected: pass
```

Behavior verification:
- `./a9s --demo`: every resource type in the migrated category renders list and detail correctly.
- Per-resource snapshot test (added in PR-03a-views): findings present, phrases stable, attention details rendered correctly.

**Independently revertable**: yes. Each per-category PR is independent of the others. If PR-03f surfaces a finding-derivation bug in security types, revert just that PR; the shim re-covers security resources because they no longer populate `Findings` directly. PR-03b through 03e and 03g–m remain landed.

---

### PR-03n — Cleanup; delete legacy

**Goal.** Delete `Resource.Status`, `Resource.Issues`, the entire `(+N)` suffix algebra, and the shim. Migrate any remaining `EnrichmentFinding` references off the legacy type into `Finding` + `AttentionDetail` directly.

**Files modified**

- `internal/resource/resource.go` — delete `Status`, `Issues` fields.
- `internal/resource/enrichment.go` — either delete entirely (if all consumers now use `Finding` + `AttentionDetail`), or shrink to a deprecation alias if there's a transitional consumer not yet migrated.

**Files deleted**

- `internal/resource/finding_suffix.go` (the entire `(+N)` algebra)
- `internal/semantics/attention/derive.go` (the shim)
- Any helper functions on `EnrichmentFinding` that no longer have callers

**Exit criteria**

```bash
# Status field is gone:
rg '\bStatus\s+string\b' internal/resource/resource.go
# expected: zero hits

# Issues field is gone:
rg '\bIssues\s+\[\]string\b' internal/resource/resource.go
# expected: zero hits

# (+N) algebra deleted:
ls internal/resource/finding_suffix.go 2>&1
# expected: "No such file or directory"
rg 'StripFindingSuffix|BumpFindingSuffix|SplitFindingSuffix' internal/
# expected: zero hits

# Shim deleted:
ls internal/semantics/attention/derive.go 2>&1
# expected: "No such file or directory"

# No fetcher anywhere writes Status:
rg 'resource\.Resource\{[^}]*Status:' internal/aws/
# expected: zero hits
rg '\.Status\s*=' internal/aws/
# expected: zero hits

# View code reads Findings, not legacy:
rg '\.Status\b|\.Issues\b|EnrichmentFinding' internal/tui/
# expected: zero hits
```

Behavior verification:
- `make test` passes.
- `make test-race` passes.
- `./a9s --demo`: every resource type's list and detail visually identical to pre-Phase-03 output.
- Integration: `A9S_CT_PROFILE=<profile> go test -tags integration ./tests/integration/ -run TestFullRelatedViewValidation` passes.

## Out of scope

- Catalog migration (`internal/resource/` → `internal/catalog/`). Phase 04.
- `FindingDef` declarative table on `ResourceTypeDef`. Phase 04 graduates `FindingCode` constants into this — see motivation below.
- Markdown spec generation. Phase 04.
- Removing the `sessionRuntime` embed. Phase 5a-extract.
- Generation type unification. Phase 5a-gens.
- The `internal/aws/` → `internal/transport/` rename. Out of program scope. `FindingCode` constants stay in `internal/aws/<svc>_codes.go` permanently as far as this refactor is concerned.

### Why FindingCode is constants in 03 and a `FindingDef` table in 04

The graduation is motivated, not cosmetic:

- **Phase 03 (constants per enricher).** Each enricher declares its codes inline next to the function that emits them. Compiler-checked, distributed ownership. No registry yet — phase 03 has no machinery to enforce that "every emitted code is declared somewhere central." Adding such machinery in 03 would require a registry, which contradicts the catalog-first direction of phase 04.
- **Phase 04 (declarative `FindingDef` table on `ResourceTypeDef`).** The catalog enumerates every possible finding for a type: code, phrase, severity, source class. This enables three things constants alone cannot: (1) `cmd/catalogen` generates per-resource markdown listing every finding the type can produce; (2) static validation that every `FindingCode` an enricher emits is declared in the type's `Findings []FindingDef` table — drift becomes a build error; (3) optional human-readable description per finding for the markdown.

If those three uses don't materialize in Phase 04, `FindingDef` is wasted indirection and should be dropped in favor of constants-only. The Phase 04 spec (`04-catalog.md`) is the place to commit or drop. Phase 03 ships constants regardless.

## Cross-references

- **Depends on Phase 01**: `Severity` enum used by `Finding.Severity`.
- **Depends on Phase 02**: session-owned caches that hold per-resource finding state get refit. The refit is mechanical: `map[string]EnrichmentFinding` → `map[string][]Finding` + `map[string]map[FindingCode]AttentionDetail`. Easier on a clean session boundary than a porous one.
- **Enables Phase 04**: catalog struct fields reference `Finding`, `AttentionDetail`, `LifecycleKey`. Without Phase 03's domain shape, the catalog would have to model the legacy zoo.

## Risk register

| Risk | Mitigation |
|---|---|
| `(+N)` suffix appears in user-facing strings during the dual-write window if shim and migrated fetcher disagree | The shim has a per-type early-return ("if `Findings` populated, skip"). PR-03b sets the flag for compute, etc. Test: render every demo resource list after each per-category PR; visually verify Status column matches pre-Phase-03 output. |
| `FindingCode` namespacing collisions between services | Convention: codes start with the resource short-name (`ec2.impaired`, `rds.maint.pending`). The Phase 04 catalog validator will enforce this; in Phase 03 it's by convention. |
| Shim derives wrong code for a Wave 2 finding because original `Summary` text drifted | The shim uses a fixed lookup table from known Summary phrases to codes (built per category before the per-category PR begins). New findings in the migrated path bypass the shim entirely. |
| Tests assert on `r.Status` content | All test conversions land in PR-03a alongside view conversions. `tests/unit/` greps for `r.Status` / `res.Status` / `Status:` and updates them all to `Findings[0].Phrase` / `Fields[lifecycle key]`. |
| `EnrichmentFinding.Rows` content gets lost during migration | The shim populates `AttentionDetails` from existing `EnrichmentFinding.Rows` for every type. Each per-category PR confirms detail-view rendering matches before stopping shim coverage. Detail-view tests assert on specific row labels and values, providing per-resource regression guards. |
