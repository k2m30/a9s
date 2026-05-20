# AS-726 — PR-04i — Catalog population, messaging category

Stage 2 spec. Sole owner: Architect. Status when written: in_progress.
Parent program: [AS-651](../../AS/issues/AS-651). Prereq: AS-718 (PR-04a) — done.
Sibling refactor docs: `04-catalog.md` (Phase 04 master plan), `AS-660-session-store-relocation.md` (per-PR spec precedent).

## Problem statement

Phase 04 makes `internal/catalog` the single source of truth for resource-type metadata
(`var ResourceTypes`) and the registry world (`Register*` in `internal/resource`,
`registerIssueEnricher` in `internal/aws/issue_enrichment.go`) is supposed to disappear.
Commit `6afe866` (phase-04-pr04f-m bundle) populated `internal/catalog/types_messaging.go`
with identity, columns, children, color, CloudTrailKey, and lifecycle metadata for the
nine messaging top-level types — but did NOT migrate the behavior wiring (Fetcher, Wave 2
enricher, Related defs, Navigable fields, FieldKeys). Those 24 `init()` calls still run
in `internal/aws/*.go` and still register into the legacy `internal/resource` maps. The
runtime then hits the catalog first via wrappers, falls through to the legacy map for
every messaging type, and works — but the legacy registrations remain authoritative
and the catalog rows are partial.

AS-726 finishes the cutover for the messaging category: messaging types' behavior wiring
lives on the catalog row, the legacy registrations for those types vanish, NoOp-only
enrichment files (`sns_sub_issue_enrichment.go`, `kinesis_issue_enrichment.go`) are deleted,
and the demo path + tests still pass.

## Scope (in)

Top-level messaging types (9 entries already in `messagingTypes`):

`sqs`, `sns`, `sns-sub`, `eb-rule`, `kinesis`, `msk`, `sfn`, `ses`, plus
the child-view stubs implied by their `Children:` slices:
`sns_subscriptions`, `eb_rule_targets`, `sfn_executions`, `sfn_execution_history`.

Service files touched (29 in `internal/aws/`):

```
internal/aws/sqs.go
internal/aws/sqs_related.go
internal/aws/sqs_issue_enrichment.go
internal/aws/sns.go
internal/aws/sns_related.go
internal/aws/sns_issue_enrichment.go
internal/aws/sns_sub.go
internal/aws/sns_sub_by_topic.go
internal/aws/sns_sub_related.go
internal/aws/sns_sub_issue_enrichment.go     [DELETE]
internal/aws/eb_rule.go
internal/aws/eb_rule_related.go
internal/aws/eb_rule_targets.go
internal/aws/eb_rule_issue_enrichment.go
internal/aws/kinesis.go
internal/aws/kinesis_related.go
internal/aws/kinesis_issue_enrichment.go     [DELETE]
internal/aws/msk.go
internal/aws/msk_related.go
internal/aws/msk_issue_enrichment.go
internal/aws/sfn.go
internal/aws/sfn_executions.go
internal/aws/sfn_execution_history.go
internal/aws/sfn_issue_enrichment.go
internal/aws/ses.go
internal/aws/ses_issue_enrichment.go
```

Out of scope for messaging (NOT touched by this PR):

- `internal/aws/eb_related.go`, `internal/aws/eb_issue_enrichment.go` — these wire the
  Elastic Beanstalk (`eb`) top-level type, which lives in the COMPUTE category. They will
  be migrated by the compute follow-up PR.
- Compute / containers / databases / networking / monitoring / secrets / dns_cdn /
  security / cicd / data / backup categories' init()s. Those are sibling cutovers under
  AS-651, not AS-726.
- `internal/ui/styles/severity.go` and the `Color` field on `ResourceTypeDef`. AS-718
  was supposed to add `internal/ui/styles/severity.go` (CodexReviewer NEEDS CHANGES on
  AS-718 still records this as a deferred fix) but did not. Color→severity inline
  migration is therefore impossible to land in AS-726 — the destination infra does not
  exist. The messaging `Color` helpers stay inline in `internal/catalog/types_messaging.go`
  (where commit `6afe866` already placed them). This is a deviation from the issue
  description's step 7; see "Acceptance deviations" below.

## Scope (out — deferred to follow-ups)

- **Import-direction flip (`internal/catalog` → `internal/aws`).** The literal exit
  criterion 3 ("Zero `init()` in messaging service files") implies catalog struct literals
  reference fetcher/enricher function values defined in `internal/aws/*.go` directly,
  which requires `internal/catalog` to import `internal/aws`. Today, `internal/aws/issue_enrichment.go`
  imports `internal/catalog`, so the reverse import would cycle. Breaking the
  `aws→catalog` edge (move `IssueEnricher` type + `GetIssueEnricher` + `IssueEnricherRegistry`
  to a leaf package, then point `internal/catalog` at it) is a cross-cutting structural
  change that affects every category, not just messaging. AS-726 keeps the existing
  direction and uses **mutator registration** (mirroring the existing
  `catalog.RegisterProject` / `catalog.RegisterAugment` pattern) so messaging can finish
  without holding up the program. Recommended follow-up: file AS-N28 (after AS-727 lands)
  to flip direction and eliminate the mutator stubs.

- **Child-type promotion.** `sns_subscriptions`, `eb_rule_targets`, `sfn_executions`,
  `sfn_execution_history` are not first-class `catalog.ResourceTypes` entries today;
  they live in `internal/resource/childTypes` and are wired by
  `resource.RegisterChildType` + `resource.RegisterPaginatedChild`. The catalog has no
  child slot. Promoting child types is a cross-cutting catalog-shape decision that should
  be set once for the program. AS-726 keeps the four child-view init()s using a new
  catalog-side child registrar (`catalog.RegisterChildView`) so the legacy
  `internal/resource/childTypes` map can become empty for messaging-derived child
  types, and the catalog accessor (`catalog.FindChild`) becomes authoritative for
  messaging children. Compute's child views (e.g. `ecs_tasks`, `lambda_invocations`)
  continue to use the legacy path; they get the same treatment in their own PRs.

## Acceptance deviations from the issue description

The issue description's exit criterion 3 reads "Zero `init()` in messaging service files".
That bar is unreachable in AS-726 without the import-direction flip discussed above.
The achievable bar in this PR — which still completes the messaging cutover — is:

> Every `init()` in `internal/aws/<messaging-services>*.go` calls only `catalog.Register*`
> (the new mutator API in `internal/catalog/catalog.go`), never `resource.Register*` or
> the legacy `registerIssueEnricher`. Legacy registrations for messaging types are removed
> from `internal/aws/issue_enrichment.go`'s `IssueEnricherRegistry` map by virtue of
> those calls no longer executing. After AS-726 the messaging category is **catalog-authoritative**
> for Fetcher / Wave 2 / Related / Navigable / FieldKeys / IssueEnricherFieldKeys /
> Children, with no legacy fallback consulted.

This deviation is necessary, narrow, and documented. The board/CTO can override by
expanding scope to include the import-direction flip — but that turns AS-726 from M
into XL and should be filed separately.

## Design

### 1. New catalog fields on `ResourceTypeDef`

Add two fields to `internal/catalog/types.go`'s `ResourceTypeDef` struct, below the
existing `Findings` section:

```go
// FieldKeys lists the Resource.Fields keys produced by the fetcher for this
// resource type. Used by detail-field rendering and column-key assertions.
// Migrated from internal/resource fieldKeyRegistry.
FieldKeys []string

// IssueEnricherFieldKeys lists additional Resource.Fields keys produced by
// the Wave 2 enricher via IssueEnricherResult.FieldUpdates. Unioned with
// FieldKeys by GetAllFieldKeys consumers. Migrated from
// internal/resource issueEnricherFieldKeysRegistry.
IssueEnricherFieldKeys []string
```

These are simple `[]string` slices populated either in the static
`messagingTypes` literal (preferred when the values are short and obvious — e.g.
`sqs` has six fetcher keys) or via the new mutator API (when wiring is needed because
the legacy `init()` already constructs them, and minimal diff is desired).

### 2. New catalog mutator API (`internal/catalog/catalog.go`)

Mirror the existing `RegisterProject` / `RegisterAugment` pattern. Each mutator
finds the catalog row by `ShortName` and sets one field. No-op when the row is missing.
Idempotent. Panics only on duplicate non-nil overwrite (safety against double-register).

```go
// RegisterFetcher sets the Wave 1 paginated fetcher on the named catalog row.
// Panics if the row already has a non-nil Fetcher (duplicate registration).
func RegisterFetcher(shortName string, fn domain.PaginatedFetcher)

// RegisterWave2 sets the Wave 2 enricher on the named catalog row. The value is
// a concrete aws.IssueEnricher (stored as any to avoid an import cycle).
// Panics if the row already has a non-nil Wave2.
func RegisterWave2(shortName string, enr any)

// RegisterRelated sets the related-resource defs on the named catalog row.
// Panics if the row already has non-empty Related.
func RegisterRelated(shortName string, defs []domain.RelatedDef)

// RegisterNavigable sets the navigable-field defs on the named catalog row.
// Panics if the row already has non-empty Navigable.
func RegisterNavigable(shortName string, fields []domain.NavigableField)

// RegisterFieldKeys sets the fetcher-produced FieldKeys on the named catalog row.
// Panics on duplicate.
func RegisterFieldKeys(shortName string, keys []string)

// RegisterIssueEnricherFieldKeys appends Wave 2 field keys on the named catalog row.
// Idempotent — duplicates are deduplicated. Multiple enrichers may target the same
// type and union their keys here.
func RegisterIssueEnricherFieldKeys(shortName string, keys []string)

// RegisterChildView registers a child-view ResourceTypeDef into the catalog's
// child registry, keyed by ShortName. Child views are NOT in the top-level
// ResourceTypes slice — they live in a separate childTypes map. Panics on duplicate.
func RegisterChildView(child ResourceTypeDef)

// FindChild returns the child-view ResourceTypeDef registered under the given
// ShortName, or nil if not in the catalog's child registry. Case-insensitive.
func FindChild(shortName string) *ResourceTypeDef
```

Storage for child views: a new package-level map in `internal/catalog/catalog.go`:

```go
var childTypes = map[string]ResourceTypeDef{} //nolint:gochecknoglobals // catalog child registry
```

### 3. Catalog-backed accessor wrappers in `internal/resource/registry.go`

Update three legacy accessors to consult catalog first, fall back to legacy registry:

- `GetFieldKeys(shortName) []string` — read `catalog.Find(shortName).FieldKeys` first.
- `GetIssueEnricherFieldKeys(shortName) []string` — read `catalog.Find(shortName).IssueEnricherFieldKeys` first.
- `GetPaginatedChildFetcher(shortName)` — already a wrapper in PR-04a; ensure the catalog
  child-fetcher path is wired. If `catalog.FindChild(shortName) != nil`, return its
  `Fetcher` (it's a `domain.PaginatedFetcher`, callable with `parentCtx` via a thin
  adapter — see implementation note below).
- `GetChildType(shortName) *ResourceTypeDef` (currently legacy-only per CXR NEEDS CHANGES
  on AS-718) — route to `catalog.FindChild(shortName)` first.

The signature for child fetchers differs (parent context). Two options:

- **A.** Add a separate `domain.PaginatedChildFetcher` field on the catalog's child
  `ResourceTypeDef` (a child entry uses it; top-level entries leave it nil). This is the
  cleanest if a single `ResourceTypeDef` struct must do double-duty for top-level and
  child rows.
- **B.** Define a new minimal struct `catalog.ChildTypeDef` with only the fields children
  use (Name, ShortName, Columns, CopyField, Children, FieldKeys, ChildFetcher), and have
  `RegisterChildView` / `FindChild` take that smaller struct.

**Choose A.** The current `internal/resource/ResourceTypeDef` (where children are
registered today) already double-duties. Mirroring that minimizes diff and lets a future
PR fold child rows into the top-level `ResourceTypes` slice if desired.

So: add a `ChildFetcher domain.PaginatedChildFetcher` field to `catalog.ResourceTypeDef`
(child-only). For child rows, `Fetcher` stays nil. For top-level rows, `ChildFetcher`
stays nil.

### 4. Messaging service file rewiring

For each of the 24 init()-bearing messaging files in scope, replace every
`resource.Register*` / `registerIssueEnricher` call with the equivalent `catalog.Register*`
call. Do not change function signatures or behavior — only the registration target.

Example transformation for `internal/aws/sqs.go`:

```go
// Before
func init() {
    resource.RegisterFieldKeys("sqs", []string{"queue_name", "queue_url", "arn", "approx_messages", "approx_not_visible", "delay_seconds"})
    resource.RegisterPaginated("sqs", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
        // ... unchanged body
    })
}

// After
func init() {
    catalog.RegisterFieldKeys("sqs", []string{"queue_name", "queue_url", "arn", "approx_messages", "approx_not_visible", "delay_seconds"})
    catalog.RegisterFetcher("sqs", func(ctx context.Context, clients any, continuationToken string) (domain.FetchResult, error) {
        // ... unchanged body
    })
}
```

`resource.FetchResult` and `domain.FetchResult` are the same type (`resource.FetchResult = domain.FetchResult`
alias — verify pre-flight; if not aliased, use the type the catalog mutator signature names).

For combined init()s (sfn.go has fetcher + related + navigable in one init; ses.go has
fetcher + related; sns_sub_by_topic.go has child fetcher + child type):

```go
// sfn.go after
func init() {
    catalog.RegisterFieldKeys("sfn", []string{"name", "type", "arn", "creation_date"})
    catalog.RegisterFetcher("sfn", func(...) (domain.FetchResult, error) { ... })
    catalog.RegisterRelated("sfn", []domain.RelatedDef{ ...same six entries... })
    catalog.RegisterNavigable("sfn", []domain.NavigableField{{FieldPath: "RoleArn", TargetType: "role"}})
}
```

For Wave 2 enrichers (`*_issue_enrichment.go`):

```go
// sqs_issue_enrichment.go after
func init() {
    catalog.RegisterWave2("sqs", IssueEnricher{Fn: EnrichSQSAttributes, Priority: 100})
    catalog.RegisterIssueEnricherFieldKeys("sqs", []string{"dlq"})
}
```

The `IssueEnricher` literal here is the existing `aws.IssueEnricher` type (defined in
`internal/aws/issue_enrichment.go`). Stored on the catalog row as `any` because
`internal/catalog` can't reference `aws.IssueEnricher`. `aws.GetIssueEnricher` already
casts `any → IssueEnricher`; that path is unchanged.

For NoOp-only files:

```go
// sns_sub_issue_enrichment.go — DELETE entire file
// kinesis_issue_enrichment.go — DELETE entire file
```

Rationale: with catalog-routed lookup, `ct.Wave2 == nil` IS the "no Wave 2" signal.
`aws.GetIssueEnricher` correctly returns `(IssueEnricher{}, false)` when the catalog
row exists and `Wave2` is nil. Verify with a unit test (`TestNoOpWave2ReturnsAbsent`).

For child views (`sns_sub_by_topic.go`, `eb_rule_targets.go`, `sfn_executions.go`,
`sfn_execution_history.go`):

```go
// sns_sub_by_topic.go after
func init() {
    catalog.RegisterChildView(catalog.ResourceTypeDef{
        Name:         "SNS Subscriptions",
        ShortName:    "sns_subscriptions",
        Columns:      resource.SnsSubscriptionColumns(),
        CopyField:    "endpoint",
        FieldKeys:    []string{"protocol", "endpoint", "confirmation_status", "owner", "subscription_arn", "topic_arn"},
        ChildFetcher: func(ctx context.Context, clients any, parentCtx domain.ParentContext, continuationToken string) (domain.FetchResult, error) {
            c, ok := clients.(*ServiceClients)
            if !ok || c == nil { return domain.FetchResult{}, fmt.Errorf("AWS clients not initialized") }
            return FetchSNSTopicSubscriptions(ctx, c.SNS, parentCtx["topic_arn"], continuationToken)
        },
    })
}
```

`resource.SnsSubscriptionColumns()` is an existing helper returning `[]resource.Column`.
If `resource.Column = domain.Column` aliases (verify), keep the call site as-is.
Otherwise inline the column literal.

### 5. Tests

The catalog-backed accessor wrappers already exist; messaging-type lookups now go
through them. Most tests are unaffected. The tests that must change:

- `tests/unit/architecture_conformance_test.go` — already iterates `catalog.ResourceTypes`
  (per PR-04a). Add assertions that for every messaging row: `Fetcher != nil`,
  `Related != nil` (where the legacy init() registered any), `FieldKeys` matches the
  pre-cutover legacy values.
- Tests that directly poke `resource.RegisterPaginated` / `resource.RegisterRelated` /
  `registerIssueEnricher` for messaging types and assert state — these need to switch
  to `catalog.RegisterFetcher` / `catalog.RegisterRelated` / `catalog.RegisterWave2`. QA
  must grep for these patterns and migrate.
- `tests/unit/qa_*messaging*.go` (if any) and `tests/unit/qa_sqs_*.go`, `qa_sns_*.go`,
  `qa_eb_rule_*.go`, `qa_kinesis_*.go`, `qa_msk_*.go`, `qa_sfn_*.go`, `qa_ses_*.go` —
  any setup helpers that re-register types for isolation need the catalog mutator.

A new test: `TestMessagingCatalogIsAuthoritative` in `tests/unit/architecture_conformance_test.go`:

```go
// Asserts each messaging type's behavior wiring is on the catalog row, not in the
// legacy resource registry. Fails if a legacy fallback would be required.
func TestMessagingCatalogIsAuthoritative(t *testing.T) {
    messagingShortNames := []string{"sqs", "sns", "sns-sub", "eb-rule", "kinesis", "msk", "sfn", "ses"}
    for _, sn := range messagingShortNames {
        ct := catalog.Find(sn)
        if ct == nil { t.Fatalf("catalog.Find(%q) returned nil", sn); continue }
        if ct.Fetcher == nil { t.Errorf("%s: Fetcher missing on catalog row", sn) }
        if len(ct.FieldKeys) == 0 { t.Errorf("%s: FieldKeys missing on catalog row", sn) }
        // sqs, sns, sns-sub, eb-rule, kinesis, msk, sfn — all have Related
        // ses — has Related
    }
}
```

### 6. `make generate` artifact refresh

Run `make generate` after the rewiring. Expected diff: none. The generator reads
`Findings` / `Related` / `Columns` / `Children` from `catalog.ResourceTypes`, which were
already populated by `6afe866`. The new `FieldKeys` and `IssueEnricherFieldKeys` fields
are not emitted to markdown (they're internal). Commit any diff; CI runs
`make generate && git diff --exit-code`.

If the generator's `Related` table for messaging types currently reads from a
catalog-backed lookup that consults `ct.Related`, AND `ct.Related` is currently empty
(because related was only in legacy), then BEFORE AS-726 the generated markdown for
messaging Related tables was empty / fallback; AFTER AS-726 it is populated. Verify by
running `make generate` on a pre-rewiring branch and comparing to post-rewiring. If a
diff appears, commit it as part of this PR (markdown is `docs/related-resources.md`
+ per-resource files `docs/resources/<short>.md`).

## Implementation order (Coder)

1. Catalog field additions (`ResourceTypeDef.FieldKeys`, `IssueEnricherFieldKeys`,
   `ChildFetcher`) — `internal/catalog/types.go`.
2. Catalog mutator API (`RegisterFetcher`, `RegisterWave2`, `RegisterRelated`,
   `RegisterNavigable`, `RegisterFieldKeys`, `RegisterIssueEnricherFieldKeys`,
   `RegisterChildView`, `FindChild`) — `internal/catalog/catalog.go`.
3. Catalog-backed accessor wiring (`resource.GetFieldKeys`,
   `resource.GetIssueEnricherFieldKeys`, `resource.GetChildType`,
   `resource.GetPaginatedChildFetcher`) — `internal/resource/registry.go`. Read catalog
   first, legacy fallback retained for non-messaging types.
4. Rewire 24 messaging service init()s to call `catalog.Register*` — list above.
5. Delete `sns_sub_issue_enrichment.go` and `kinesis_issue_enrichment.go`.
6. `make build`, `make test`, `make lint`, `make security`, `make gofix`, `make generate`,
   verify clean.
7. Self-check: `grep -n 'resource\.\(RegisterFieldKeys\|RegisterPaginated\|RegisterRelated\|RegisterDefaultNavFields\|RegisterPaginatedChild\|RegisterChildType\|RegisterIssueEnricherFieldKeys\)' internal/aws/sqs*.go internal/aws/sns*.go internal/aws/eb_rule*.go internal/aws/kinesis*.go internal/aws/msk*.go internal/aws/sfn*.go internal/aws/ses*.go` → zero hits.
8. Self-check: `grep -n 'registerIssueEnricher(' internal/aws/sqs*.go internal/aws/sns*.go internal/aws/eb_rule*.go internal/aws/kinesis*.go internal/aws/msk*.go internal/aws/sfn*.go internal/aws/ses*.go` → zero hits.

## Acceptance (concrete, machine-checkable)

```bash
# Catalog has the new mutators
grep -E '^func RegisterFetcher\(|^func RegisterWave2\(|^func RegisterRelated\(|^func RegisterNavigable\(|^func RegisterFieldKeys\(|^func RegisterIssueEnricherFieldKeys\(|^func RegisterChildView\(|^func FindChild\(' internal/catalog/catalog.go
# expected: 8 hits

# No messaging-service file references the legacy registrar:
grep -nE 'resource\.(RegisterFieldKeys|RegisterPaginated|RegisterRelated|RegisterDefaultNavFields|RegisterPaginatedChild|RegisterChildType|RegisterIssueEnricherFieldKeys)|registerIssueEnricher\(' \
  internal/aws/sqs*.go internal/aws/sns*.go internal/aws/eb_rule*.go internal/aws/kinesis*.go internal/aws/msk*.go internal/aws/sfn*.go internal/aws/ses*.go
# expected: zero hits

# NoOp-only files are gone:
ls internal/aws/sns_sub_issue_enrichment.go internal/aws/kinesis_issue_enrichment.go 2>&1
# expected: No such file or directory (both)

# Catalog and legacy registry stay in agreement on messaging behavior:
make test
# expected: pass — including the new TestMessagingCatalogIsAuthoritative

# Demo path renders all messaging types correctly:
./a9s --demo
# expected: :sqs / :sns / :sns-sub / :eb-rule / :kinesis / :msk / :sfn / :ses each list and detail-render

# Build / lint / vuln gates:
make build && make lint && make security && make gofix
# expected: clean

# Generator artifact is current:
make generate && git diff --exit-code
# expected: no diff (or commit the generated diff in this PR)
```

## Sizing

**M.** ≈ 600 LOC across ~30 files: catalog mutator API ~150 LOC, 24 init() rewrites
~250 LOC mechanical, file deletions -10 LOC, accessor wrapper updates ~50 LOC,
tests ~150 LOC. Borderline M/L, but mechanical surgery — no novel design beyond the
mutator additions, which mirror existing `RegisterProject`/`RegisterAugment`.

## Hand-offs

Per CAE-1 the spec publication IS the dispatch trigger. Two child issues are filed in
the same heartbeat as this spec:

- **QA child** — failing tests on the feature branch that prove messaging is catalog-authoritative.
- **Coder child** — implementation that makes those tests pass.

QA tests are committed first to the feature branch (so the branch is red at commit
time), Coder lands the implementation that turns it green.

Reviewer chain (Stage 5): always-runs CodeReviewer + CodexReviewer, plus Architect
re-read (size M-borderline-L), plus CTO final sign-off. No further confirmation gate.

## Risk notes for reviewers

- **Mutator panic on duplicate register.** During AS-726, if the same `init()` is
  imported from two places (test transitive imports or a stray duplicate), the new
  `catalog.Register*` will panic on duplicate non-nil overwrite. The legacy `Register*`
  silently overwrote. This is a deliberate tightening; if it surfaces a real duplicate
  (e.g., a test imports both `internal/aws` and a mock that also init()s), fix the test,
  do not loosen the panic.
- **Order of init() between catalog mutator definition and messaging service init()s.**
  Go runs package-level var init before any same-package init() and runs imported
  packages' init() before importer's init(). Messaging files in `internal/aws` import
  `internal/catalog` (added by this PR). `internal/catalog/catalog.go`'s `ResourceTypes`
  is a package-level var initialized via `allTypes()`, which is invoked at package init
  time — BEFORE any `internal/aws/*.go` init() runs. Therefore `catalog.Find(shortName)`
  in `catalog.Register*` will already see the messaging row when the mutator fires. Good.
- **The `eb_*` (Elastic Beanstalk, compute) vs `eb-rule*` (EventBridge, messaging) split.**
  Coder must not touch `eb_related.go` / `eb_issue_enrichment.go` — those are compute.
  Files in scope are explicitly `eb_rule.go`, `eb_rule_related.go`, `eb_rule_targets.go`,
  `eb_rule_issue_enrichment.go`. Self-check via the grep above which lists files explicitly.
- **`internal/aws/issue_enrichment.go`'s `IssueEnricherRegistry` keeps non-messaging
  entries.** Do not touch the registry map or `GetIssueEnricher`'s fallback branch.
  Those serve the 11 other categories whose cutover follows in sibling PRs. PR-04n
  deletes the fallback after every category is migrated.
