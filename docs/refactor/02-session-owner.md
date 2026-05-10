# Phase 02 — Session owner; package globals deleted

**5 PRs. Mandatory. No prerequisites.**

## Goal

Every session-scoped cache becomes a typed field on a single `Session` struct. `internal/aws/` (still its current name; rename happens here) gives up its package-globals: the IAM policies cache, the identity cache, and the SES rule-set cache all move to `Session`-owned types accessed through narrow capability interfaces.

`Session` is also the only valid owner for future capability-scoped caches and long-running task state. This phase migrates today's three concrete globals; later features (for example log-source discovery caches or query cursors/progress) must reuse `Session`, not invent new package-level state.

After this phase, **`Session.Rotate()` is the only place that bumps generation counters and clears caches.** Profile/region switch invokes one method; nothing else needs to remember to call cleanup helpers.

## Why this phase is second

The current session boundary is porous:

- `internal/aws/iam_policies.go:162-177` — process-wide `allPoliciesMu` cache, reset via `awsclient.ResetIAMPoliciesCache()` called from `sessionRuntime.resetForSessionSwitch:128`.
- `internal/aws/identity_cache.go:21` — `identityCacheMu` global with explicit `//nolint:gochecknoglobals // process-scoped Pattern C cache` comment.
- `internal/aws/ses_related.go:120-125` — `sesRuleSetCacheMu` + `sesRuleSetCaches map[*ServiceClients]*sesReceiptRuleSetCache`. Reset via `ClearAllSESRuleSetCaches()` called inline from `internal/tui/app_handlers.go:243`, *outside* `resetForSessionSwitch`. Any new session-rotation site that forgets to call it leaks across accounts.

Closing this boundary before any larger phase adds to it means later phases never have to negotiate around session-state hidden in transport.

## What this phase delivers

- `internal/session/` package — owns `Session` struct with typed cache fields, `Rotate()` method, and the architectural ownership point for future capability/task-scoped state.
- Capability interfaces in `internal/session/` (the package that defines them; `internal/aws/` consumes them via function signatures). `PolicyStore`, `IdentityStore`, `RuleSetStore`. Transport functions in `internal/aws/` take capabilities, not `*Session`. Note: `internal/aws/` is not renamed in this program — see `04-catalog.md` "Out of scope".
- Three deletions: `iam_policies.go` global, `identity_cache.go` global, `ses_related.go` rule-set globals.
- One canonical reset path: `Session.Rotate()` replaces every `Reset*Cache` / `Clear*Cache` / `Invalidate*Cache` symbol.
- A standing rule for later phases: if a feature needs session-lifetime memoization or in-flight task bookkeeping, it lives on `Session` (or a typed store owned by `Session`), not in `internal/aws/` package globals.

## PR breakdown

### PR-02a — Session skeleton + capability interfaces + cross-package promoted-selector compat

**Goal.** Define `Session` and the three capability interfaces, and mechanically migrate every existing access site from the embedded `sessionRuntime` (with unexported fields) to an embedded `*session.Session` (with exported fields). This PR is structurally additive at the type level but touches every site that currently uses promoted selectors like `m.resourceCache`, `m.probeResources`, `m.relatedGen`. It is one PR specifically because half-migrating breaks compilation across `internal/tui/`.

**The promoted-selector problem (concrete).** Today, `internal/tui/session_runtime.go:13` defines `sessionRuntime` with unexported fields like `resourceCache`, `probeResources`, `enrichmentFindings`, `relatedGen`. `tui.Model` embeds `sessionRuntime` *by value*, so call sites read `m.resourceCache`, `m.probeResources` via Go's field promotion — those selectors work because the field is unexported but the access site is in the same package (`internal/tui`). Once the type body moves to `internal/session/Session`, the same fields become unexported in a *foreign* package; promoted selectors stop compiling.

**Compatibility approach (chosen).** Capitalize the formerly-unexported fields when moving to `internal/session/Session`, and rename every call site mechanically in the same PR. `m.resourceCache` becomes `m.ResourceCache`; `m.probeResources` becomes `m.ProbeResources`; `m.relatedGen` becomes `m.RelatedGen`. This is one mechanical rename across `internal/tui/`. Three alternatives were rejected:

- **(a) Re-export `sessionRuntime` from `internal/tui` as a wrapper around `*session.Session`.** Either the wrapper still has the unexported fields (dual state, violates invariant #1) or it forwards to exported fields on `*Session` — in which case we still capitalize at call sites, just with more layers. Rejected.
- **(b) Move fields to `internal/session/Session` but keep them unexported, accessing via methods.** Every `m.resourceCache[k] = v` becomes `m.Session().ResourceCacheSet(k, v)`. Order of magnitude more churn than capitalization, with worse readability. Rejected.
- **(c) Keep `sessionRuntime` in `internal/tui/` indefinitely.** Defeats the phase. Rejected.

#### Files added

- `internal/session/session.go` — `Session` struct with exported fields (capitalized from `sessionRuntime`'s former unexported fields). `New()`, `Rotate()` constructors and methods.
- `internal/session/policy_store.go` — `PolicyStore` interface + thread-safe map-backed implementation.
- `internal/session/identity_store.go` — `IdentityStore` interface + impl.
- `internal/session/rule_set_store.go` — `RuleSetStore` interface + impl.

#### Files modified

- `internal/tui/session_runtime.go` — type body deleted; the file may itself be removed entirely, with `tui.Model` updated in `internal/tui/app.go` to embed `*session.Session` directly. The embed pattern preserves promoted-selector access (now to capitalized field names).
- `internal/tui/app.go`, `internal/tui/app_handlers*.go`, `internal/tui/app_fetchers.go`, `internal/tui/app_probes.go`, `internal/tui/app_related.go`, and any other file in `internal/tui/` that reads or writes promoted fields — mechanical capitalization. Sample concrete renames:

  | Before | After |
  |---|---|
  | `m.resourceCache` | `m.ResourceCache` |
  | `m.probeResources` | `m.ProbeResources` |
  | `m.enrichmentFindings` | `m.EnrichmentFindings` |
  | `m.enrichmentRan` | `m.EnrichmentRan` |
  | `m.enrichmentTypeGen` | `m.EnrichmentTypeGen` |
  | `m.relatedGen` | `m.RelatedGen` |
  | `m.availabilityGen` | `m.AvailabilityGen` |
  | `m.enrichGen` | `m.EnrichGen` |

  The full list is whatever the current `sessionRuntime` type body enumerates as fields. Audit the type body before the mechanical rename so nothing is missed.

- `tests/unit/*.go` — any test that constructs `tui.Model{...}` literals with formerly-lowercase field initializers updates to capitalized.

#### Exit criteria

```bash
ls internal/session/
# expected: session.go, policy_store.go, identity_store.go, rule_set_store.go (+ tests)

# Type body fully migrated:
rg '^type sessionRuntime struct' internal/tui/
# expected: zero hits

# Promoted selectors still work via embed:
go build ./...
# expected: clean compile

# No lingering lowercase references to former-private fields outside internal/session:
rg '\bm\.resourceCache\b|\bm\.probeResources\b|\bm\.relatedGen\b|\bm\.enrichmentFindings\b' internal/
# expected: zero hits

# Capability interfaces compile but are not yet load-bearing:
rg 'PolicyStore|IdentityStore|RuleSetStore' internal/aws/
# expected: zero hits — wired up in PR-02b/c/d
```

The capability interfaces are unused at this point. Subsequent PRs in this phase make them load-bearing.

---

### PR-02b — IAM policies: replace global

**Goal.** Delete `internal/aws/iam_policies.go`'s `allPoliciesMu` + cache. The IAM-policies-listing code path takes a `PolicyStore` capability instead.

#### Files modified

- `internal/aws/iam_policies.go` — function signatures change from `func ListAllIAMPolicies(ctx, clients)` to `func ListAllIAMPolicies(ctx, clients, store PolicyStore)`. The cached lookup logic moves into `PolicyStore` impl; `ListAllIAMPolicies` becomes a pure transport function.
- All call sites of `ListAllIAMPolicies` and any sibling functions that read the cache — pass `Session.iamPolicies` (or a runtime-scoped wrapper) at the call site.
- `internal/tui/session_runtime.go:128` — delete the `awsclient.ResetIAMPoliciesCache()` call. `Session.Rotate()` clears `iamPolicies` directly.

#### Files deleted (symbols)

- `var allPoliciesMu sync.Mutex` and the underlying map in `iam_policies.go`
- `func ResetIAMPoliciesCache()` — exported helper no longer needed

#### Exit criteria

```bash
rg 'allPoliciesMu|ResetIAMPoliciesCache' internal/
# expected: zero hits

rg 'sync\.(R?W?)Mutex' internal/aws/iam_policies.go
# expected: zero hits
```

Behavior verification:

- Demo mode passes existing tests.
- Manual: profile-switch in `./a9s --demo` (after seeding two demo profiles) — IAM Roles list re-fetches cleanly.

---

### PR-02c — Identity cache: replace global

**Goal.** Same shape as 02b, applied to `internal/aws/identity_cache.go`.

#### Files modified

- `internal/aws/identity_cache.go` — convert `func GetCallerIdentity(ctx, clients)` to take an `IdentityStore` capability. The `//nolint:gochecknoglobals` comment is what we're earning the right to delete.
- All call sites — pass `Session.identity`.

#### Files deleted (symbols)

- `var identityCacheMu sync.Mutex` and the cached identity value
- The associated `//nolint:gochecknoglobals` comment

#### Exit criteria

```bash
rg 'identityCacheMu' internal/
# expected: zero hits

rg 'gochecknoglobals' internal/aws/
# expected: zero hits — if any remain, they belong to a different PR's deletion target
```

---

### PR-02d — SES rule sets: replace global

**Goal.** Delete `sesRuleSetCacheMu`, `sesRuleSetCaches`, `InvalidateSESRuleSetCache(*ServiceClients)`, `ClearAllSESRuleSetCaches()`. SES related-checkers take a `RuleSetStore` capability.

#### Files modified

- `internal/aws/ses_related.go` — checker functions take `RuleSetStore`. The pattern `cache, ok := sesRuleSetCaches[c]` (lines 141–147) is replaced with `store.Get(...)`.
- `internal/tui/app_handlers.go:243` — delete the inline `awsclient.ClearAllSESRuleSetCaches()` call. `Session.Rotate()` clears `sesRuleSets` directly.

#### Files deleted (symbols)

- `var sesRuleSetCacheMu`, `var sesRuleSetCaches`, `type sesReceiptRuleSetCache` (or move it into `internal/session/rule_set_store.go` as the impl)
- `func InvalidateSESRuleSetCache(*ServiceClients)`, `func ClearAllSESRuleSetCaches()`

#### Exit criteria

```bash
rg 'sesRuleSetCache|ClearAllSESRuleSetCaches|InvalidateSESRuleSetCache' internal/
# expected: zero hits

rg 'sesRuleSetCacheMu|sesReceiptRuleSetCache' internal/aws/
# expected: zero hits (the type may exist in internal/session/ as the impl)
```

---

### PR-02e — Single rotation path; cleanup

**Goal.** `Session.Rotate()` is the sole reset entry point. All `Reset*Cache` / `Clear*Cache` / `Invalidate*Cache` exported symbols in `internal/aws/` are deleted (02b/c/d removed three; this PR finds any stragglers, e.g. `iam_policy_doc_cache.go` if it has any).

Note: PR-02a already moved `sessionRuntime`'s body to `internal/session/Session` and capitalized every promoted-selector access site. PR-02e does NOT re-do that work; it finishes the cache-reset story and finalizes any obsolete test-only helpers that were targeting the deleted `Reset*Cache` symbols.

#### Files modified

- `internal/tui/app_handlers.go` — any remaining inline cache resets are replaced with `m.Session.Rotate()`.
- `internal/aws/ses_cache_test_accessor_test.go` and `internal/aws/ses_clear_cache_test.go` — these tests existed solely to verify `ClearAllSESRuleSetCaches`, deleted in PR-02d. **Migrate them**: re-target the same invariants ("rotate clears all entries"; "rotate on empty store is a no-op") at the new `RuleSetStore` capability, OR delete them if equivalent coverage exists in the new `internal/session/rule_set_store_test.go`. Do not leave them dangling.

#### Files deleted

- Whatever `Reset*Cache` / `Clear*Cache` / `Invalidate*Cache` exported symbols remain in `internal/aws/`.
- `internal/aws/ses_cache_test_accessor_test.go` and `internal/aws/ses_clear_cache_test.go` (assuming the migration above lands their replacements in `internal/session/`).

#### Exit criteria

```bash
rg 'func (Reset|Clear|Invalidate)\w*Cache' internal/aws/
# expected: zero hits

rg '^var \w+\s+(=|sync\.|map\[)' internal/aws/
# expected: zero mutable globals — only allowed: const, type, and embedded data
# (the regex catches mutable-looking var declarations; manual review for false positives)

rg '\.Rotate\(\)' internal/
# expected: exactly one definition (in internal/session/) and call sites only in handleProfileSelected, handleRegionSelected, and any test helpers that simulate session switch
```

Behavior verification:

- Profile-switch in `./a9s` (real AWS): IAM Roles list, STS identity, SES rule-set checkers all re-fetch cleanly with no stale data from the prior account.
- `make test` and `make test-race` pass.
- Integration test against `A9S_CT_PROFILE=<profile>` passes.

## Out of scope

- The `*session.Session` embed in `tui.Model` STAYS in this phase. Phase 5a-extract un-embeds it. Phase 02 relocates the content from `internal/tui/session_runtime.go` into `internal/session/Session` and capitalizes the field names so embedded promotion still works.
- Renaming `internal/aws/` → `internal/transport/`. **Decision: no rename, ever** (see `04-catalog.md`). Capability interfaces live in `internal/session/` permanently.
- Generation type unification (`int` vs `uint64`). Phase 5a-gens.
- Any change to `Resource.Status` / `Findings`. Phase 03.
- Any catalog or markdown work. Phase 04.

## Cross-references

- **Independent of Phase 01**: can land in parallel.
- **Enables Phase 03**: `EnrichmentFinding`-keyed caches on `Session` get refit when the finding model changes; that refit is much smaller than rewriting the global-cache structure simultaneously with the domain shape.
- **Enables Phase 5a**: the un-embedding step in Phase 5a is mechanical because by then `Session` is its own clean type.

## Risk register

| Risk | Mitigation |
|---|---|
| `*ServiceClients` keyed in `sesRuleSetCaches` map encodes important identity that capability-based caches lose | **`RuleSetStore` is keyed per-Session, not per-clients and not per-process.** Today's `map[*ServiceClients]*sesReceiptRuleSetCache` exists because `*ServiceClients` is the only handle the package globals could lock onto for isolation. After Phase 02, each `Session` owns one `RuleSetStore` instance; the map keying disappears entirely (a `Session`'s store has one slot for the active rule-set cache). On profile/region switch, `Session.Rotate()` replaces the store with a fresh one. The SES test suite must be updated to construct test-local `RuleSetStore` instances instead of asserting on per-clients map contents. |
| Any third party (test helper, integration test, demo wiring) calls `ResetIAMPoliciesCache` etc. directly | `rg` for the symbol names before deletion in each PR. Replace with `Session.Rotate()` or test-scoped capability swap. |
| `internal/tui/session_runtime.go` deletion breaks tests that import the old type name | Provide a one-line re-export for the duration of Phase 02; delete the re-export in Phase 5a-extract. |
