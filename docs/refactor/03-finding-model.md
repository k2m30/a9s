# Phase 03 — Canonical finding model

**14 PRs. Mandatory. Depends on Phase 01 (`Severity` enum) and Phase 02 (`Session` owner).**

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

14 PRs total: 1 setup + 12 per-category cutover + 1 cleanup.

### PR-03a — Types, shim, view-side conversion

**Goal.** Introduce all new types. Add a one-way derive shim that synthesizes `Findings` and `AttentionDetails` from existing `Status` + `Issues` + `EnrichmentFinding` at fetch boundary. Convert every view-side `r.Status` read to the new model. After this PR, no view code reads `Resource.Status`.

This is the largest and riskiest single PR in the program. The shim is the safety net that makes the per-category cutover (PR-03b through PR-03m) safe.

**Files added**

- `internal/domain/finding.go` — `FindingCode`, `Finding`
- `internal/semantics/attention/types.go` — `AttentionDetail`, `DetailRow`
- `internal/semantics/attention/derive.go` — **the one-way shim.** `func DeriveFindings(r Resource, td ResourceTypeDef) ([]Finding, map[FindingCode]AttentionDetail)`. Reads `r.Status`, `r.Issues`, and any per-resource `EnrichmentFinding`. Synthesizes a `FindingCode` from the source phrase using a deterministic hash or registry. **Never writes back.**

**Files modified**

- `internal/resource/resource.go` — add `Findings`, `AttentionDetails`. Keep `Status`, `Issues` for now; they become legacy fields written by unmigrated fetchers.
- `internal/resource/types.go` — add `LifecycleKey` to `ResourceTypeDef`.
- All 12 `internal/resource/types_*.go` files — set `LifecycleKey` on every type def. Default empty means "state". Audit which types use a non-default key (e.g. some types may use `"phase"` or `"status"`).
- `internal/tui/views/resourcelist.go`, `resourcelist_helpers.go`, `table_render.go` — list-view Status column and color logic now reads `Findings[0].Phrase` / `Findings[0].Severity`, falling back to `Fields[td.LifecycleKey]`. NEVER reads `r.Status`.
- `internal/tui/views/detail.go`, `detail_fields.go`, `detail_helpers.go`, `rightcolumn.go` — Attention section reads `r.AttentionDetails[code]` for each `r.Findings[i]`. NEVER reads `EnrichmentFinding` directly.
- `internal/tui/views/mainmenu.go` — issue-count badges count `len(filter(r.Findings, IsIssue))` per resource, not `r.Status`-string interpretation.
- `internal/tui/views/attention.go` — filter predicate reads `Findings`.
- `internal/tui/app_handlers_availability.go`, `app_probes.go`, `app_enrich.go` — anywhere a `Resource` is constructed or mutated post-fetch, ensure `Findings` is populated via the shim before the resource flows into views. Concretely: `app_fetchers.go` wraps fetcher results and runs `attention.DeriveFindings` before publishing.

**Exit criteria**

```bash
# View code does not read Resource.Status:
rg '\br?\.Status\b|\bres\.Status\b|resource\.Resource\{[^}]*Status:' internal/tui/
# expected: zero hits

# Fetchers and enrichers still write Status (legacy path) — that's fine for now:
rg 'Status:\s*\w' internal/aws/ | wc -l
# expected: many hits — these are migrated in 03b through 03m

# View code does not read EnrichmentFinding directly (everything goes through AttentionDetail):
rg 'EnrichmentFinding|\.Summary\b|\.Rows\b' internal/tui/
# expected: only the legacy compatibility shim references; zero direct field reads
```

Behavior verification: `./a9s --demo` produces visually identical list and detail output to pre-PR. The shim's job is to make this PR a no-op visually.

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
4. Declares `FindingCode` constants in `internal/transport/<svc>/codes.go` (or sibling file). Per-enricher namespacing: `ec2.CodeImpaired = "ec2.impaired"`, `rds.CodeMaintPending = "rds.maint.pending"`, etc. Phase 04 graduates these to a declarative table; in Phase 03 they are typed constants per enricher.
5. The shim in `attention/derive.go` no longer fires for migrated types (it has a per-type early-return: "if `r.Findings` already populated by fetcher, skip"). Verify the early-return covers each migrated category before merging.

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
ls internal/transport/<svc>/codes.go
# expected: present for every service in the category that has a Wave 2 enricher
```

Behavior verification:
- `./a9s --demo`: every resource type in the migrated category renders list and detail correctly.
- Per-resource integration test (where one exists): findings present, codes stable, attention details populated.

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
- `FindingDef` declarative table on `ResourceTypeDef`. Phase 04 graduates `FindingCode` constants into this.
- Markdown spec generation. Phase 04.
- Removing the `sessionRuntime` embed. Phase 5a-extract.
- Generation type unification. Phase 5a-gens.

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
