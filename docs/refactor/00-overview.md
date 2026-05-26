# a9s Architecture Refactor — Overview

This folder is the execution plan for bringing the a9s codebase in line with a no-legacy / no-lazy-compromise architecture. It is a working artifact, not a description of the current state.

For *what is true today*, see [`../architecture.md`](../architecture.md). For *what should be true after this refactor lands*, see this folder.

## Design north star

After this refactor lands, **adding a new resource type is mechanical**:

1. One catalog entry in `internal/catalog/types_<category>.go` — a `ResourceTypeDef` struct literal with direct function references for `Fetcher`, `Wave2`, `Project`, `Related`.
2. One transport file `internal/aws/<svc>.go` — pure function consuming capabilities, returning `[]Resource`. (`internal/aws/` is not renamed in this program — see `04-catalog.md` "Out of scope".)
3. One demo file `internal/demo/fixtures/<svc>.go`.
4. Tests in `tests/unit/`.

If a contributor needs to remember more than four files to add a resource, the refactor failed.

Specifically, after this refactor:

- Zero `init()` calls in feature wiring. Catalog is static data; codegen produces what registries used to.
- Zero `Register*` function calls. The catalog struct *is* the registration.
- Zero `Resource.Status` field initializers. Status display is derived from `Findings` and `Fields[LifecycleKey]`.
- Zero hand-editable structured sections in markdown. `attention-signals.md`, `related-resources.md`, and the structured tables (Findings, Related, Columns) inside per-resource specs are generated between `BEGIN GENERATED` / `END GENERATED` markers; prose between markers is preserved verbatim.
- Zero package-globals in `internal/aws/`. Caches are session-owned; transport receives narrow capability interfaces.
- Zero `if shortName == "x"` branches in dispatch code (per-type default UI state e.g. `resourcelist.go`'s ct-events default-sort is the only allowed shape).
- Zero per-category `Color` functions. A single `styles.SeverityStyle(domain.Severity) lipgloss.Style` mapping is the only severity-to-presentation translator. `Color` is migrated inline by Phase 04 per-category PRs — per-category PRs replace `td.Color(r)` call sites with `styles.SeverityStyle(...)` in the same PR. The `Color` field remains on `ResourceTypeDef` as a per-category hook; the invariant is that no standalone 5a-color PR exists, not that the field disappears.
- A future desktop renderer can be added as an adapter on top of the same app core; it does not require rewriting orchestration, selectors, queries, tasks, or session logic.

## Issue-driven target amendments

As of **2026-04-26**, the open issue set shows architectural pressure that goes beyond cleanup of today's legacy wiring. These are amendments to the target architecture, not new standalone phases in this program:

- **Cross-cutting capabilities are separate from the resource catalog.** `internal/catalog` remains resource-shaped metadata. Logs, CloudTrail investigation, cost views, and any future action system do not accrete as ad-hoc behavior fields on `ResourceTypeDef`; catalog entries declare support via `Capabilities []domain.CapabilityID`, while implementations live in explicit capability modules. Pressure: #18, #20, #21, #60, #63, #73, #112.
- **Query/evidence views are first-class contracts.** Time-bounded searches, event/debug views, log streams, and cost breakdowns are not disguised as plain resource lists or child views. They use explicit query specs (`Filter`, `TimeRange`, `Cursor`, `Limit`) with their own pagination and cache semantics; the shared type declarations live in `internal/domain`. Pressure: #18, #20, #60, #112.
- **Selector/matcher logic is shared semantics, not checker-local string code.** Wildcard ARN matching, tag-condition matching, and future resource-selection predicates are modeled in `internal/semantics/selector/` and reused by related-checkers and coverage logic. Pressure: #296, #297.
- **Slow work uses one background-task contract.** Enrichment, related resolution, log-source discovery, and future query scans report progress, support cancellation, define cache policy, and surface partial completion the same way. Pressure: #21, #261, #292, #293.
- **Screen registration is declarative.** New deep views register screen descriptors with runtime; they do not require growing central type switches in the UI shell. Pressure: #18, #60, #112, #206.
- **Feature paths must be test-bounded.** Every new capability must admit unit/demo coverage and a bounded live smoke path; no feature may require an unbounded full-account crawl to be considered validated. Pressure: #253, #293.
- **Shared app core is renderer-agnostic.** Bubble Tea is a TUI adapter, not the architectural center. Future desktop delivery must be able to drive the same core through plain Go/domain contracts without importing Bubble Tea into shared packages. Candidate adapter paths include a Go-first shell such as Wails, a Tauri shell with a Go sidecar, or an Electron-like shell. This program chooses the seam, not the product.
- **Mutating actions, if ever accepted, live behind an explicit action layer.** They do not leak into the read-only browsing core or silently reuse read-path contracts. Pressure: #63.

## Phase summary

| # | Phase | PRs | Mandatory | Closes |
|---|---|---|---|---|
| 2 | Domain bootstrap + projection hook | 1 | yes | shortName branching in detail rendering; `internal/resource ↔ internal/semantics` cycle risk |
| 3 | Session owner | 5 | yes | package-globals in `internal/aws/`; ad-hoc cache resets; promoted-selector cross-package access |
| 1' | Finding model | 17 | yes | `Status`/`Issues`/`(+N)` triple; parallel Wave 2 enrichment map (`m.enrichmentFindings`); severity-as-color conflation |
| 4 | Catalog | 14 | yes | `Register*` API; markdown-as-source-of-truth |
| 5a-extract | Runtime extraction | 1 | yes | sessionRuntime embed in UI shell |
| 5a-gens | Gen unification | 1 | yes | `int` vs `uint64` gen-counter salad |
| 5b | Message taxonomy | 1 | yes | command/event ambiguity; type-level gen-stamping |

Total: **40 PRs, all mandatory.**

Phase 1' is 17 PRs (was 14): the original PR-03a is split into 03a-types, 03a-shim, 03a-views, and 03a-fold (the Wave 2 row-mutation path that replaces `m.enrichmentFindings` — without this PR, post-enrichment findings disappear from list/detail rendering between 03a-views and 03b). See `03-finding-model.md`. Phase 5b is mandatory (was previously listed as optional); type-level gen-stamping prevents an entire bug class, and "optional structural correctness" is itself the kind of compromise this refactor exists to remove. The Color → severity collapse — invariant #4's structural completion — is handled inline by Phase 04 per-category PRs (per-category PRs replace `td.Color(r)` call sites with `styles.SeverityStyle(...)` in the same PR; the `Color` field stays on `ResourceTypeDef` as the per-category hook). No standalone 5a-color PR.

Phase numbering reflects original priority labels (Phase 2 was the second item in the original priority order; Phase 1' is the redesign of Phase 1 around the lifecycle correction). Filename numbering reflects execution order.

## Execution order and dependencies

```text
01-projection-hook  ─┐
                     ├─→  03-finding-model  ─→  04-catalog  ─→  05-boundary (5a-extract → 5a-gens → 5b)
02-session-owner   ──┘
```

Justification for this order:

- **`01-projection-hook` first**, because it's small, validates the semantics-layer direction, and introduces the `internal/domain` leaf package that every subsequent phase imports — `Severity`, `Resource` (moved here from `internal/resource/`), and the `Section` / `Item` / `DetailProjector` type declarations. Putting the type declarations in `internal/domain` (impls in `internal/semantics/`) is what prevents the `internal/resource ↔ internal/semantics/*` import cycle that would otherwise arise once `ResourceTypeDef.Project` is set to `ctevent.Project`. Discovery is cheap; if `Section`/`Item` shape is wrong, we revert ten files.
- **`02-session-owner` second**, because it closes the ownership boundary before any larger phase adds to it. Doing this before `03-finding-model` means the largest domain-shape change lands on top of a clean session boundary, not a porous one.
- **`03-finding-model` third**, after both infrastructure pieces. Depends on `01-projection-hook`'s `Severity` enum. Carries the largest surface (~150 files) and ships across 17 PRs. Acknowledged refit cost: `02-session-owner`'s session-owned cache types may need shape adjustments when `EnrichmentFinding` splits — accepted because the alternative (do `03` first) means landing the biggest domain change on the porous session boundary.
- **`04-catalog` fourth**, after the domain shape is canonical. Catalog entries reference `Severity`, `Finding`, `LifecycleKey`, `DetailProjector`, all introduced in earlier phases.
- **`05-boundary` last**, because by then everything else is stable. `5a-extract` removes the `sessionRuntime` embed that earlier phases have been working around; `5a-gens` unifies what's left of the gen-counter zoo; `5b-msg-taxonomy` lands the type-level command/event split.

## Migration discipline — stabilization checkpoints, not per-PR atomicity

This program executes as 40 PRs across 5 phases. **Intermediate PRs are NOT required to keep `make test`, `make test-race`, snapshot suites, integration goldens, or `./a9s --demo` rendering fully green.** Test and render regressions during a migration window are accepted IF they are (a) bounded in surface, (b) expected at the time the PR lands, and (c) resolved by the next planned stabilization checkpoint listed below. **Compile failures are NOT relaxed** — see hard-safety #1 below. Reviewers should not require extra compatibility scaffolding — extra shims, dual-write paths, test-only forwarders, or per-PR golden regenerations — whose only purpose is to preserve mid-migration test or UX parity. Forward progress toward the target architecture is preferred over polishing intermediate states.

### Stabilization checkpoints (where correctness IS enforced)

Every phase ends with a checkpoint where the full gate suite (`make test`, `make test-race`, `make lint`, `make security`, `make gofix`, snapshot suite, demo integration suite, real-AWS integration test, pre-push agents) MUST pass. Phase 01 is a single-PR phase, so its checkpoint coincides with PR-01 itself.

| Checkpoint | What is verifiably true at this commit |
|---|---|
| End of Phase 01 (PR-01) | `internal/domain` is the leaf type-declaration package; `internal/semantics/{projection,ctevent,selector}/` exist with stub or real implementations; `ResourceTypeDef.Project` field wired; ct-events shortName branch deleted from `internal/tui/views/`; full test/render parity preserved. Phase 01 ships as a single PR per [`landed/01-projection-hook.md`](./landed/01-projection-hook.md), so this checkpoint coincides with PR-01 itself. |
| End of Phase 02 (PR-02e) | `Session.Rotate()` is the single reset entry point; `internal/aws/` carries no mutable globals; live profile-switch test passes against a real AWS profile; `make test` green. |
| End of Phase 03 (PR-03n) | Canonical `Findings` model is the only model; `Resource.Status`/`Resource.Issues` deleted; the on-disk cache YAML format break is migrated crash-free per the PR-03n risk-register decision; `make test-race` clean. |
| End of Phase 04 (PR-04n) | Catalog is authoritative; zero `init()` and zero `Register*` in feature wiring; `make generate && git diff --exit-code` clean; mechanical-resource-implementation acceptance test passes. |
| End of Phase 05 (PR-05b) | Shared core compiles without Bubble Tea / lipgloss imports; generation counters unified; cmd/event message taxonomy with type-level gen-stamping. |
| Program exit | All of the above hold simultaneously plus the four-file CloudHSM acceptance test from "Mechanical-resource-implementation acceptance test" below. |

### Hard safety guarantees (these hold AT EVERY PR, regardless of checkpoint)

These are non-negotiable even in mid-migration commits. Violating any of these is a blocker no matter how partial the PR is:

1. **HEAD compiles.** `go build ./...` is clean on every committed state. A non-compiling tree blocks all parallel work in the program; the program assumes contributors can branch off any landed commit.
2. **No irreversible data loss or cache corruption.** The on-disk cache YAML format break is *migrated* (clear-on-upgrade with a one-line user notice, or one-way deprecated-field write per PR-03n's choice), never silently dropped.
3. **No permanent dual-source-of-truth contract.** One-way derive shims are acceptable as transitional compatibility; bidirectional mirror-back is not. A shim that writes both directions is a permanent second source of truth wearing a costume — reject it regardless of which PR introduces it.
4. **No misleading permanent API.** If a PR introduces a new exported API, function signature, or struct field whose intended lifetime is permanent, that API MUST be the intended end-state shape — not a transitional shape that a later PR will rename. Renaming carries a churn cost the program cannot afford to pay twice. **Carve-out**: short-lived, explicitly documented forwarding wrappers (e.g. `internal/aws/ct_verb_format.go` — three-line delegations to `internal/semantics/ctevent` introduced in PR-01 to keep test imports compiling without a same-PR test-import migration) are NOT permanent APIs and do not violate this rule. Such wrappers MUST carry a godoc line explicitly stating "transitional forwarding wrapper for <reason>" and MUST be removed in a stabilization-checkpoint PR within the same phase or the next phase.

### What this means in practice

- "Independently revertable: yes" assertions previously attached to intermediate PRs are downgraded to "**stabilization-checkpoint commit**": the PR's intermediate state may not be revertable in isolation. To revert mid-phase, revert the entire phase or wait for the next checkpoint.
- "Every PR exits with `make test` green" is downgraded to "**every stabilization checkpoint exits with `make test` green**; intermediate PRs may carry expected, bounded, documented test failures whose closure is owned by a downstream PR within the same phase."
- "Per-category PRs are independent and parallelizable" is **preserved as a scoping property** — per-category migrations remain independently scopeable so multiple developers can work in parallel — but parallelism does not imply each per-category PR must be test-green when landed in isolation. The phase-end stabilization PR (PR-03n, PR-04n) is where the category fleet snaps green together.

## Cross-phase invariants

These are non-negotiable. A PR that violates any of these is wrong, regardless of which phase it belongs to.

1. **No dual-authoring.** Every kind of metadata has exactly one writable source. Catalog is authoritative; legacy registry is generated. Findings is authoritative; legacy `Status` is derived. Markdown specs are generated; never hand-edited.

2. **One-way migration shims only.** Compatibility shims derive new fields from old state. They NEVER mirror back. A shim that writes both directions is a second source of truth wearing a costume.

3. **Stable IDs, not display strings, for keys.** `FindingCode` keys `AttentionDetails`, never `Finding.Phrase`. Phrase is display text and may change without the underlying finding changing.

4. **Severity is domain, color is presentation.** `domain.Severity` is the typed enum (`SevOK | SevWarn | SevBroken | SevDim`). Each renderer adapter owns its own severity-to-presentation mapping; the TUI adapter's `domain.Severity -> lipgloss.Style` mapping lives in `internal/tui/styles/`. `IsIssue()` is a method on `Severity`, not on `Color`.

5. **Compile-time codegen, not runtime `init()`.** Generated files (markdown specs) are checked in, diffable, reviewable. CI runs the generator and fails if output drifts from the committed file. **End state**: zero `init()` in feature wiring. **Migration window**: during Phase 04, legacy `init()`-based registrations in unmigrated categories continue to fire — this is in-flight migration, not a permanent state. PR-04n's exit criterion is the moment "zero `init()`" becomes structurally true.

6. **No package-globals in `internal/aws/`.** Capability interfaces flow from runtime. `internal/aws/` (the transport package, name unchanged in this program) never imports `internal/session`.

7. **Views stay passive.** Views render state and emit messages. They do not consume use-case interfaces. The runtime/UI boundary is between the message handlers and the view layer, not between the views and a service API.

8. **Features are test-bounded.** Every new capability lands with (a) unit coverage, (b) demo-mode coverage, and (c) a bounded live smoke path that completes in under 60s on a representative account. A feature that requires an unbounded full-account crawl to be considered "validated" is architecturally wrong.

9. **Shared core contracts are platform-agnostic.** `internal/domain`, `internal/runtime`, `internal/session`, `internal/aws`, `internal/catalog`, and `internal/semantics/*` must not depend on Bubble Tea, browser/DOM APIs, Electron, or renderer-specific IPC types. `internal/tui` is the Bubble Tea adapter and may translate those concerns at the boundary.

## How to use this folder

Each phase file (`01-` through `05-`) is structured as:

- **Goal** — one paragraph
- **PR breakdown** — every PR in the phase, with scope
- **Per-PR specs** — for each PR: file list, additions, deletions, exit criteria
- **Out of scope** — what this phase deliberately does not touch
- **Cross-references** — what other phases this depends on or enables

The phase file is the source of truth for the PRs in that phase. The PR description for any given PR may copy-paste from its per-PR spec; the spec is intentionally written to be PR-description-shaped.

When a PR merges, mark its checkbox in the phase file (a one-line edit, committed alongside any follow-up). When all PRs in a phase merge, mark the phase complete in this overview's phase summary table.

## Mechanical-resource-implementation acceptance test

This is the program-wide exit criterion. After all mandatory phases land, adding a hypothetical new resource type — say, `CloudHSM` — must be possible without touching any of these:

- `internal/tui/app.go` or any `internal/tui/app_handlers*.go` file
- `internal/aws/issue_enrichment.go` or any registry-style file
- `internal/resource/registry.go` or `internal/resource/types.go`
- `docs/attention-signals.md` or `docs/related-resources.md` or `docs/resources/<short>.md` (these are generated)
- Any `init()` function anywhere

The full file set required to add `CloudHSM` must be:

```text
internal/catalog/types_security.go    (add one struct literal to the existing slice)
internal/aws/cloudhsm.go              (new file: paginated fetcher + any Wave 2 enricher and related-checker functions referenced by the catalog literal)
internal/demo/fixtures/cloudhsm.go    (new file: demo fixtures)
tests/unit/cloudhsm_*.go              (test files)
```

Four files. **A reviewer reading those four files alone, without prior context, can verify correctness.** No init-order surprises, no dual sources to keep in sync, no markdown to remember to update. Wave 2 / related logic — if any — lives in the same `internal/aws/cloudhsm.go` file, or in a sibling `internal/aws/cloudhsm_wave2.go` if size warrants splitting.

If at the end of the program this test fails — if adding a resource still requires touching `app.go`, or running `go run ./cmd/readmegen`, or remembering to call `RegisterFieldKeys` — the refactor has not landed. The phase exit criteria are calibrated to this acceptance test.

## Status tracking

| Phase | Status | Notes |
|---|---|---|
| 01-projection-hook | LANDED | `internal/domain` bootstrap + Section/Item/DetailProjector type decls landed |
| 02-session-owner | LANDED | session-owned caches; `internal/aws/` package-globals removed |
| 03-finding-model | in progress (PR-03n cleanup pending) | 17 PRs landed; PR-03n cleanup tracked under [AS-1390](../../specs/) umbrella + W1.* sibling issues |
| 04-catalog | LANDED | `aws.Install()` + `catalog.SetTypes(...)` two-step model in production; see `04-catalog.md` |
| 05a-extract | LANDED | `runtime.Core` extracted from `internal/tui/app.go` (PR-05a-h4 series) |
| 05a-gens | LANDED | gen-counter unification merged into 05a sequence |
| 05b-msg-taxonomy | LANDED | message taxonomy + type-level gen-stamping landed alongside 05a |

Update this table as phases complete.

## Notes on counts and verification

- Per-resource markdown count: at the time of writing, `ls docs/resources/ | grep -v impl-plan | wc -l` returns 66; total file count is 77 (including 9 `*-impl-plan.md` files and any other narrative docs). The Phase 04 exit criterion is "every catalog entry has a corresponding `docs/resources/<short>.md`" — verified by enumerating the catalog, not by hard-coding 66. **Verify the live count in the migration PR; the number above is informational.**
- Per-resource markdown content: existing `docs/resources/<short>.md` files contain DevOps prose ("why this matters", workflow notes) that is NOT trivially derivable from a struct. Phase 04 preserves prose between `<!-- BEGIN GENERATED -->` / `<!-- END GENERATED -->` markers; only structured sections (findings table, related table, columns) are regenerated. See `04-catalog.md` for the marker convention.
