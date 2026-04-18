# a9s Intended Architecture

> **NORMATIVE CONTRACT.** This is the authoritative architecture specification for a9s.
> When the implementation diverges from this document, the implementation is wrong
> (or this document must be amended via PR with justification).
> For current-state descriptive details, see [architecture.md](./architecture.md).

This document is the normative architecture spec for a9s.

Use it to answer: "How should the system be designed?" and "Is the current implementation conforming?"

Use [`architecture.md`](./architecture.md) for the descriptive/current guide. Use this file as the intended-design contract.

## Scope

This spec covers:

- runtime ownership and layer boundaries
- the resource registry model
- fetch, related, navigation, and enrichment pipelines
- cache and invalidation rules
- extension rules for adding new resource types

This spec does not try to document every current implementation detail or every historical compromise.

## Product Shape

a9s is a read-only terminal UI for AWS.

Core product constraints:

- The app MUST remain read-only at runtime. Production code may call AWS `List*`, `Describe*`, `Get*`, or equivalent read APIs only.
- The app MUST feel resource-centric, not service-client-centric. Users navigate resources, relationships, and investigations.
- The app MUST prefer correctness and explainability over clever implicit behavior.
- The app MUST degrade honestly: unknown must stay unknown; partial results must stay marked partial.

## Architectural Principles

1. One root application model owns session state and orchestration.
2. Views render state and emit typed messages; they do not perform AWS I/O.
3. Registries are the declarative source of truth for supported resource types and behaviors.
4. AWS adapter code translates AWS SDK data into `resource.Resource`; it does not own navigation or UI policy.
5. Every async result must carry enough identity to reject stale, cross-resource, cross-profile, or cross-region updates.
6. Background systems must be registry-complete. Hand-maintained allowlists for supported types are architectural debt and should not exist.

## Layer Boundaries

### `cmd/a9s`

Bootstrap only:

- parse flags
- load config/theme
- resolve startup profile/region/demo mode
- create root TUI model
- start Bubble Tea

It MUST NOT contain feature logic.

### `internal/tui`

Owns runtime orchestration:

- view stack
- global key handling
- message routing
- async command dispatch
- session-scoped in-memory caches
- stale-result protection
- refresh/profile/region invalidation

It MAY know about resource-type metadata and registry contracts.
It MUST NOT embed AWS service-call logic in views.

### `internal/resource`

Owns declarative contracts:

- `Resource` shape
- resource type definitions
- fetcher/reveal/enricher registrations
- related definitions
- navigable field definitions
- child-view definitions
- color classification policy

It is the schema and registry layer, not the execution layer.

### `internal/aws`

Owns AWS adapter logic:

- create AWS clients
- call SDK APIs
- map SDK responses into `resource.Resource`
- implement related checkers
- implement Wave 2 enrichers
- implement on-demand detail enrichers

It MUST NOT know about Bubble Tea views, stack state, or terminal rendering.

### `internal/cache`

Persistence only:

- disk-backed availability cache
- TTL and atomic file replacement

It MUST NOT decide UI behavior beyond the data it stores.

### `internal/config`

Owns view and theme configuration loading/merging.

### `internal/demo`

Owns fake transports and synthetic data.

Demo mode is an injected runtime mode, not a parallel architecture.

## Root Runtime Model

The root `tui.Model` is the session owner.

It MUST own:

- active profile and region
- current `ServiceClients`
- app-scoped context/cancellation
- the full view stack
- session caches
- async generation counters/tokens
- background progress state

Views MAY own local UI state such as cursor position, filter text, viewport offsets, and right-column focus.
Views MUST NOT own shared session state or background workers.

## Message-Driven Runtime

a9s follows the Elm Architecture via Bubble Tea:

```text
input -> Update(msg) -> state + Cmd -> async result msg -> Update(msg)
```

Rules:

- `Update()` MUST stay non-blocking.
- AWS calls MUST run in `tea.Cmd` closures.
- Views communicate upward via typed messages only.
- The root model decides navigation, cache writes, and async result routing.

## View Stack

Navigation is stack-based, not route-table-based.

Intended behavior:

- `MainMenu -> ResourceList -> Detail/YAML/JSON/Reveal/...`
- `Esc` is the back/dismiss key.
- `q` is the quit key in normal mode; it is not the navigation primitive.
- sibling switches such as `detail <-> yaml <-> json` SHOULD replace in place rather than grow the stack

The stack should model investigation flow directly and remain easy to reason about.

## Resource Model

The generic runtime resource is:

```go
type Resource struct {
    ID        string
    Name      string
    Status    string
    Fields    map[string]string
    RawStruct any
}
```

Intended meanings:

- `ID` is the canonical navigation and matching identifier for that resource type.
- `Name` is the primary human label when present.
- `Status` is a coarse display/status hint only.
- `Fields` contains flat values used by tables, filters, and lightweight detail rendering.
- `RawStruct` preserves the typed AWS response object for richer detail/YAML/JSON views.

Architectural invariants:

- Cross-view and cross-pipeline matching MUST use the same canonical identity the fetcher emits in `Resource.ID`.
- Any enrichment or related checker that returns per-resource data MUST key it by the target type's `Resource.ID`, not by an adjacent ARN/name/task ID unless that is the target type's canonical ID.

## Registry Model

The registry layer defines what the app supports.

For a top-level resource type, intended support normally means:

- `ResourceTypeDef`
- paginated fetcher
- field key registration
- color classification
- optional aliases
- optional related definitions
- optional navigable fields
- optional child views
- optional reveal fetcher
- explicit Wave 2 enricher registration, including explicit no-op when Wave 2 is intentionally absent

Registry invariants:

- Registry keys MUST be consistent across type definition, fetcher, related target, and enricher registration.
- Registry absence MUST mean unsupported, not "supported but forgotten".
- Any system that operates "for all supported types" MUST derive from the relevant registry rather than a duplicated list.

## Fetch Pipeline

There are four distinct fetch modes:

1. top-level paginated fetch
2. child-resource paginated fetch
3. server-side filtered paginated fetch
4. exact cache-hit navigation with no fetch

The root model MUST choose one of these explicitly for every navigation path.

First-page fetch results are important session primitives. They are used for:

- list rendering
- main-menu counts
- resource-cache warmup
- related cache warmup
- retained Wave 2 inputs

`Load more` MUST continue the same fetch mode the list was opened with.

## Availability And Wave 1

Wave 1 is the first-page probe and visible-issue count pass.

Intended behavior:

- one top-level probe per registered top-level resource type
- count first-page resources
- note truncation explicitly
- count visible issues via the type's `Color` function
- retain the first page for downstream enrichment

Unknown vs zero is important:

- fetch failure or unavailable data => unknown
- zero from a complete probe => confirmed zero
- zero from a truncated probe => lower bound, not confirmed zero

The UI must preserve that distinction in menu badges, attention filtering, and refresh behavior.

## Wave 2 Enrichment

Wave 2 discovers hidden issues that are not visible from Wave 1 row coloring alone.

Architecture:

- the authoritative set is `aws.EnricherRegistry`
- every documented resource type has an explicit entry
- real enrichers and `NoOpEnricher` entries are both valid registry members

Queueing invariants:

- the background enrichment queue MUST be built from the full registry, not from a manual shortlist
- startup and main-menu refresh MUST consider every registered enricher automatically
- a type with a registered enricher MUST NOT require the user to manually open that list before Wave 2 can run, except when Wave 2 intentionally depends on first-page probe data not yet collected

Execution invariants:

- Wave 2 consumes retained first-page resources from Wave 1
- account-wide enrichers may discover findings beyond the retained page
- per-resource enrichers MAY cap fan-out for safety, but truncation must be surfaced honestly
- findings MUST be keyed by the affected resource's canonical `Resource.ID`
- reruns MUST replace current findings for that type, not merge stale generations

Freshness invariants:

- profile switch, region switch, and explicit reruns MUST invalidate stale in-flight results
- there MUST be both session-wide and per-type stale guards where overlapping reruns are possible
- top-level list refresh MUST rerun Wave 2 for that resource type when the type participates in Wave 2

Presentation invariants:

- menu issue badges represent resource-health issues, not every informational finding
- list rows may show per-resource markers for findings
- detail views may show a background-check section for the selected resource
- lack of a visible row marker MUST NOT be interpreted as proof that no hidden issues exist off-page

## Related Panel

The detail right column is a related-resource investigation panel.

`RelatedDef` is declarative and defines:

- target type
- display name
- checker
- whether the checker requires target-cache access

Intended checker semantics:

- `Count == -1` means unknown
- `Count == 0` means confirmed zero
- `Count > 0` means confirmed matches
- `Approximate == true` means the count is a lower bound derived from partial cached data

Architectural invariants:

- `TargetType` identifies the actual type the drill-in opens
- `ResourceIDs` MUST be IDs valid for that `TargetType`
- a checker for `TargetType: "ecs"` must return ECS cluster IDs, not ECS task IDs or task-definition IDs
- when a checker needs cached target pages, the dispatcher may prefetch the target type's first page on cache miss
- related results MUST be generation-guarded so stale checks cannot update a later detail view

## Related Navigation

Related navigation should prefer the most exact drill-in available.

Intended precedence:

1. exact cache hit for a single target => open detail directly
2. exact target IDs / related ID set => open a filtered list scoped to those IDs
3. server-side filtered fetch => use only when the target type explicitly supports it
4. otherwise open the unfiltered target list

Critical invariants:

- `FetchFilter` is not a generic alternative to `ResourceIDs`
- a non-empty `FetchFilter` MUST only be attached when the target type has a matching `FilteredPaginatedFetcher` and that filter is the intended canonical drill-in mechanism
- if exact `ResourceIDs` are already known for a target type without filtered-fetch support, navigation MUST preserve and use those IDs
- server-side filters are appropriate for types like CloudTrail event pivots; they are not a blanket navigation mechanism for arbitrary target types

## Navigable Fields

Navigable fields are field-level drill-ins from detail/YAML/JSON views.

Intended rules:

- the field path registration lives in `internal/resource`
- the navigation target is still resolved by the root model
- field navigation and right-column navigation must converge on the same related-navigation contract

## On-Demand Detail Enrichment

This is separate from Wave 2.

Purpose:

- fetch richer detail data when a user opens detail/YAML/JSON for one resource

Rules:

- it is opt-in per resource type
- it runs asynchronously after the view opens
- errors are surfaced as feedback, not fatal view corruption
- session-scoped caches are allowed when the fetched data is expensive and unlikely to change within the session
- package-global mutable caches are not allowed

## Caches

There are four intended cache classes:

1. disk availability cache
2. in-memory resource cache
3. in-memory related-result cache
4. feature-specific session caches for enrichers/detail fetches

Cache rules:

- disk cache is for availability/count summaries, not authoritative drill-in data
- resource cache stores loaded rows plus enough list state to resume a view cheaply
- related cache stores derived related-check results keyed to the source resource
- feature-specific caches belong to the current session/client set and disappear when clients are replaced

## Invalidation Rules

Profile switch and region switch are hard session-boundary events.

They MUST:

- replace the app context / AWS clients
- invalidate resource cache
- invalidate related cache
- invalidate enrichment state and findings
- invalidate in-flight async results via generation counters

Refresh MUST be narrower:

- main-menu refresh reruns availability and Wave 2 globally
- top-level list refresh reruns that list's fetch and its Wave 2 path when applicable
- detail refresh reruns detail/related work for that resource, not unrelated global work

## Config And Schema Ownership

View config controls presentation, not runtime semantics.

Config MAY choose:

- columns
- ordering
- field visibility
- labels

Config MUST NOT redefine:

- canonical resource identity
- fetcher kind
- related target type
- refresh invalidation rules
- the message/state ownership model

Those are code-level contracts.

## Extension Rules

When adding a new top-level resource type, intended completeness is:

1. add the `ResourceTypeDef`
2. add the top-level fetcher
3. register field keys and aliases
4. define `Color`
5. decide related defs and navigable fields
6. decide CloudTrail pivot support
7. decide Wave 2 status and register either a real enricher or explicit no-op
8. update golden docs/tests that define coverage

When adding a related checker:

1. return IDs for the actual declared `TargetType`
2. use `FetchFilter` only if the target has filtered-fetch support
3. preserve unknown vs zero vs approximate semantics
4. ensure drill-in opens the resource(s) the checker counted

## Golden Sources Of Truth

These documents are intended architectural contracts, not optional notes:

- [`attention-signals.md`](./attention-signals.md): which types participate in Wave 1 and Wave 2
- `related-resources.md`: which cross-resource relationships should exist
- [`architecture.md`](./architecture.md): descriptive/current runtime guide
- this file: intended/normative design

If implementation, tests, and docs disagree, this file plus the feature-specific golden docs should be used to drive reconciliation.

## Practical Review Checklist

When checking implementation against intended architecture, ask:

- Does every supported type derive behavior from registries rather than hard-coded lists?
- Does every async pipeline carry enough identity/generation to reject stale results?
- Do related drill-ins open exactly what the checker counted?
- Are `FetchFilter` and `ResourceIDs` used for the right reasons?
- Are Wave 2 findings keyed to canonical resource IDs?
- Do refresh/profile/region transitions invalidate the correct state?
- Is unknown preserved honestly instead of being collapsed into zero?
