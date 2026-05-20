# AS-660 — Move session-scoped stores off `ServiceClients` to `session.Session`

Stage 2 design spec. Owns the contract between QA (Stage 3) and Coder (Stage 4).

Parent: AS-648 P2 #7. Sizing: L. Routing: Architect (this spec) → QA + Coder (split, dispatched in parallel).

## 1. Problem

`internal/aws/ServiceClients` (a9s's AWS transport object) currently carries three session-scoped capability stores as unexported fields guarded by `storesMu`:

```go
// internal/aws/client.go:72-76
type ServiceClients struct {
    storesMu      sync.RWMutex
    iamPolicies   iamPolicyStore
    identityStore identityStore
    ruleSets      ruleSetStore
    // …AWS SDK clients (EC2, S3, RDS, …)
}
```

These are not transport. They are per-(profile, region) **capability state**:

| Store | Purpose | Reader |
| --- | --- | --- |
| `iamPolicies` | session-scoped IAM policy cache, keyed by name + ARN | `iam_policies.go:60` (`FetchIAMPoliciesByIDsFull`) |
| `identityStore` | cached STS caller account ID + sticky failure | `glue_related.go:127`, `ebs_related.go:162` (`accountIDFromClients`) |
| `ruleSets` | single-slot cache for SES v1 `DescribeActiveReceiptRuleSet` | `ses_related.go:133` (`sesActiveReceiptRuleSet`) |

Coupling them to transport is wrong on three axes:

1. **Conceptual** — `docs/architecture.md` already states `ServiceClients` is transport. Session state belongs on `session.Session`.
2. **Operational** — every `Session.Rotate()` / Ctrl+R refresh / connect-failure rollback has to remember to **rewire** the retained transport with the post-rotate stores (`runtime/handlers.go:218-261`, `tui/runtime_adapter_navigate.go:447,495`). The in-code comment at `client.go:266-269` already documents the bug-magnet shape: "any in-flight fetcher that captured a reference before this swap continues to write to the orphaned old store". An entire class of regression (PR #360 "P3 invariant", AS-359, AS-657 generation-stamp) traces back to this duplication.
3. **Concurrency** — `storesMu` exists only because writers (Update goroutine) and readers (fetcher goroutines) race on the three fields. With the fields gone, `ServiceClients` is immutable post-construction and the mutex becomes unnecessary.

The post-PR-03a-fold work already made the *correct* shape exist: `session.Session.IAMPolicies` / `IdentityStore` / `RuleSets` are public fields, with concurrency-safe `session.PolicyStore` / `IdentityStore` / `RuleSetStore` value types and explicit `Session.Rotate()` reset (`session.go:260-269`). What remains is to **drop the bridge** — stop copying these references onto `ServiceClients`, and make the four reader call sites consult `Session` directly via a small carrier that the runtime constructs per dispatch.

## 2. Goals & non-goals

**Goals**
1. `ServiceClients` no longer holds `storesMu`, `iamPolicies`, `identityStore`, `ruleSets` — or the six accessor methods (`IAMPolicies()`, `SetIAMPolicies(...)`, `IdentityStore()`, `SetIdentityStore(...)`, `RuleSets()`, `SetRuleSets(...)`).
2. The four reader sites obtain their store from a session-scoped value, not from transport.
3. `Session.Rotate()` already resets all three; we verify and add a race test.
4. Behavior unchanged: IAM full-policy fetch, identity-store lookups, SES rule-set lookups, Pattern-C related checkers (Glue tags, EBS Backup) continue to work, demo path included.
5. `make ready-to-push` green; `make test-race` green.

**Non-goals**
- AWS API client field changes (EC2/S3/RDS/etc. stay on `ServiceClients`).
- Generation-stamping for fetch results (AS-657 / AS-648 P2 #2).
- Renaming or restructuring `session.Session` beyond the additions for this PR.
- Refactoring fetchers that do not touch the three stores — **the registry's `clients any` carrier stays `*ServiceClients` for those 76+ unchanged fetchers**.

## 3. Threading approach (the design decision)

**Chosen approach: explicit per-dispatch carrier via the registry's `clients any` parameter, but only for the four readers that need it.**

Four call sites read these stores. The other ~76 fetchers do not. The carrier expansion is therefore **surgical**: only the readers' closures and their dispatch points change.

### 3.1 New type: `aws.Scope`

```go
// internal/aws/scope.go
package aws

// Scope bundles AWS transport with the per-session capability stores read by
// fetcher and related-checker code. Constructed by the runtime per dispatch
// (or per ResourceCache wiring) from session.Session; passed as the registry's
// "clients any" carrier for the four resource types whose fetcher closures
// consult session-scoped state:
//
//   - policy            (RegisterFetchByIDs: reads IAMPolicies)
//   - glue              (RegisterRelated → checkGlueCFN: reads IdentityStore)
//   - vol               (RegisterRelated → checkEBSBackup: reads IdentityStore)
//   - ses               (RegisterRelated → checkSESLambda/checkSESS3: reads RuleSets)
//
// All other fetchers continue to receive a bare *ServiceClients via the same
// registry parameter — Scope embeds *ServiceClients via field promotion so the
// transport remains reachable via `s.Clients.EC2`, `s.Clients.IAM`, etc.
//
// Lifetime: a Scope is read-only after construction. The stores it references
// are concurrency-safe internally (session.PolicyStore / IdentityStore /
// RuleSetStore each own their own synchronization). The runtime constructs a
// fresh Scope whenever it dispatches one of the four readers; no mutex needed.
type Scope struct {
    Clients       *ServiceClients
    IAMPolicies   IAMPolicyAccess
    IdentityStore IdentityAccess
    RuleSets      RuleSetAccess
}

// IAMPolicyAccess, IdentityAccess, RuleSetAccess are the exported renames of
// the duck-typed interfaces currently named iamPolicyStore / identityStore /
// ruleSetStore in iam_policies.go / identity_cache.go / ses_related.go. They
// live in internal/aws/ so the package does not import internal/session
// (which would cycle). session.PolicyStore / IdentityStore / RuleSetStore
// satisfy them structurally.
```

### 3.2 The four reader closures change

Each of these four init() closures gets its store from `scope`, not from `c.IAMPolicies() / c.IdentityStore() / c.RuleSets()`:

| Resource | Registry method | Current call site | New shape |
| --- | --- | --- | --- |
| `policy` | `RegisterFetchByIDs` | `iam_policies.go:52-61` | `s, ok := clients.(*Scope); … FetchIAMPoliciesByIDsFull(ctx, s.Clients.IAM, ids, s.IAMPolicies)` |
| `glue` | `RegisterRelated` → `checkGlueCFN` | `glue_related.go:117-127` | `s, ok := clients.(*Scope); … accountIDFromClients(ctx, s.Clients, s.IdentityStore)` |
| `vol` | `RegisterRelated` → `checkEBSBackup` | `ebs_related.go:152-162` | same pattern |
| `ses` | `RegisterRelated` → `checkSESLambda` / `checkSESS3` (both call `sesActiveReceiptRuleSet`) | `ses_related.go:129-153` | `sesActiveReceiptRuleSet(ctx, c, store)` — store passed explicitly from the checker, which receives it via `clients.(*Scope)` |

`accountIDFromClients` already takes an explicit `store identityStore` parameter — no signature change needed there. `FetchIAMPoliciesByIDsFull` already takes an explicit `store iamPolicyStore` parameter — same. `sesActiveReceiptRuleSet(ctx, c *ServiceClients)` **gains** a third `store RuleSetAccess` parameter; its three callers pass `s.RuleSets` from the closure.

### 3.3 Runtime constructs Scope per dispatch

The runtime layer that invokes these four registry slots already has `session.Session` in scope. It constructs a `*Scope` value on the fly. Concretely, the dispatch sites in `internal/runtime/` (and the equivalent demo dispatch in `internal/demo/`) that call into these four registry slots need a one-line helper:

```go
// internal/runtime/scope.go (new, ~25 LOC)
func newScope(s *session.Session) *awsclient.Scope {
    if s == nil || s.Clients == nil {
        return nil
    }
    return &awsclient.Scope{
        Clients:       s.Clients,
        IAMPolicies:   s.IAMPolicies,
        IdentityStore: s.IdentityStore,
        RuleSets:      s.RuleSets,
    }
}
```

The dispatcher selects `clients` per type:

```go
// pseudocode, in the runtime fetch dispatcher
var clients any
switch resType {
case "policy", "glue", "vol", "ses":
    clients = newScope(s)
default:
    clients = s.Clients
}
registry.Fetch(resType).Invoke(ctx, clients, …)
```

The actual dispatch sites are in `internal/runtime/probes.go`, `internal/runtime/tasks*.go`, and `internal/runtime/handlers.go` — Coder grep-finds them; the spec is the four resource types listed above.

### 3.4 Why this and not an alternative

| Alternative | Reason rejected |
| --- | --- |
| **Change `clients any` carrier to `*Scope` for ALL fetchers.** | ~80 type assertions across `internal/aws/*.go` would change. Outside the L size envelope and unnecessary churn — 76 of those fetchers don't read session stores. |
| **`context.WithValue` plus typed scope key.** | Smaller diff (~150 LOC) but uses `context.Value` for cross-cutting first-class data — anti-pattern in Go. Tests would have to remember to attach the scope to every dispatch context; failure mode is silent `nil` deref instead of compile-time error. |
| **Make `ServiceClients` hold a public `*Scope` field.** | Violates acceptance criterion 1 in spirit (`ServiceClients` still holds session state). Re-introduces the bridge: every `Session.Rotate()` would need to swap or replace `Scope`, recreating exactly the bug magnet AS-660 is trying to eliminate. |
| **Embed `*Scope` in `ServiceClients` so all promoted methods work.** | Same problem as above. The point is that transport and session state are **separate values**; embedding glues them back together. |
| **Change registry contract to `func(ctx, clients, scope, …)`.** | The registry is in `internal/resource/`, exported broadly; adding a parameter is a 100+ file edit and changes the published `FetchByIDsFunc` / paginated fetcher types. Massive blast radius, no upside over the per-type carrier approach. |

The chosen design isolates the change to **exactly** the readers that need it. Transport-only fetchers stay untouched. The runtime's per-type branching to build `Scope` for the four resource types is small, explicit, and self-documenting; future readers join the same switch.

### 3.5 Concurrency invariant

After this PR:
- `ServiceClients` has no mutable fields touched from multiple goroutines; the mutex is removed.
- `Scope` is immutable after construction. The stores inside it own their own synchronization (`session.PolicyStore.mu`, `session.IdentityStore.mu`, `session.RuleSetStore.mu`).
- `Session.Rotate()` (already in place) replaces each store reference with a fresh instance, and bumps the relevant generation counters. Any in-flight fetcher that captured a reference to a pre-rotate store continues to write into the orphaned old store on completion — exactly the orphan-store fallout the current `RuleSets()` doc-comment describes, only now uniform across all three stores and free of the rewire dance.
- The "P3 invariant" that `handlers.go:210-220` currently enforces (rewire post-rotate stores into the retained transport on connect-failure rollback) **dissolves**: there is nothing to rewire because the transport never held the stores in the first place. The retained `*ServiceClients` is unaffected by rotation; the fresh stores live on `Session` and the next `newScope()` reads them.

## 4. File-level work plan

### 4.1 Create

- `internal/aws/scope.go` — new file. Defines `Scope`, `IAMPolicyAccess`, `IdentityAccess`, `RuleSetAccess` (the latter three are exported renames of the current unexported duck-typed interfaces). ~60 LOC.
- `internal/runtime/scope.go` — new file. Defines `newScope(*session.Session) *awsclient.Scope`. ~25 LOC.

### 4.2 Modify

**`internal/aws/client.go`** (Coder)
- Delete lines `72-76`: drop `storesMu`, `iamPolicies`, `identityStore`, `ruleSets`.
- Delete lines `193-277`: drop the per-session capability store accessor block (`IAMPolicies()`, `SetIAMPolicies(...)`, `IdentityStore()`, `SetIdentityStore(...)`, `RuleSets()`, `SetRuleSets(...)`), and the doc-comment paragraph at `client.go:57-71` referencing them.
- Drop the `sync` import if no longer used.

**`internal/aws/iam_policies.go`** (Coder)
- Move `iamPolicyStore` interface (lines 17-25) to `internal/aws/scope.go` as exported `IAMPolicyAccess`.
- Change closure at lines `52-61` to type-assert `*Scope` and read `scope.IAMPolicies`.

**`internal/aws/identity_cache.go`** (Coder)
- Move `identityStore` interface (lines 31-35) to `internal/aws/scope.go` as exported `IdentityAccess`.
- `accountIDFromClients` signature is unchanged (already explicit-store).

**`internal/aws/ses_related.go`** (Coder)
- Move `ruleSetStore` interface (lines 19-24) to `internal/aws/scope.go` as exported `RuleSetAccess`.
- Change `sesActiveReceiptRuleSet(ctx context.Context, c *ServiceClients) (...)` → `sesActiveReceiptRuleSet(ctx context.Context, c *ServiceClients, store RuleSetAccess) (...)`. Delete the `store := c.RuleSets()` line at 133.
- Both callers (`checkSESLambda` at 322, `checkSESS3` at 350) currently receive `clients any`; change to `s, ok := clients.(*Scope); … sesActiveReceiptRuleSet(ctx, s.Clients, s.RuleSets)`.

**`internal/aws/glue_related.go`** (Coder)
- `checkGlueCFN` line 122: change type assertion from `clients.(*ServiceClients)` to `clients.(*Scope)`, use `s.Clients` and `s.IdentityStore` (line 127) instead of `c` and `c.IdentityStore()`.

**`internal/aws/ebs_related.go`** (Coder)
- `checkEBSBackup` line 157: same shape as Glue.

**`internal/runtime/handlers.go`** (Coder)
- Delete lines `210-220` (the failure-path rewire — P3 invariant evaporates).
- Delete lines `253-261` (the success-path bridge — `s.Clients` is just installed; nothing more to wire).

**`internal/runtime/probes.go` / `internal/runtime/tasks*.go`** (Coder)
- Wherever a registry fetcher is dispatched, branch on resource type: for `"policy"`, `"glue"`, `"vol"`, `"ses"` pass `newScope(s)`; for everything else pass `s.Clients`. Coder grep-finds the exact sites; Architect spec is the resource-type list above.

**`internal/tui/runtime_adapter_navigate.go`** (Coder)
- Lines `444-449` and `490-496`: drop the `m.core.Session().Clients.SetRuleSets(...)` rewire. Keep the `m.core.Session().RuleSets = session.NewRuleSetStore()` swap (still required for the Ctrl+R semantic — invalidates in-flight blocked DescribeActiveReceiptRuleSet fetchers via orphan-store).

**`internal/demo/client.go`** (Coder)
- Delete lines `65-67` (`SetIAMPolicies`, `SetIdentityStore`, `SetRuleSets` no longer exist). The stores are populated on `session.Session` already by `session.New()`.

**`internal/session/session.go`** (Coder, optional polish)
- Update the comment at `session.go:144-165` so it no longer mentions "Wired into `*ServiceClients.IAMPolicies` on every ClientsReadyMsg" — that wiring is gone.

### 4.3 Delete from tests, replace, or rewrite

**`internal/runtime/handlers_test.go`** (QA)
- Delete `TestHandleClientsReady_Failure_RewiresPostRotateStores` (lines `1061-1108`). The P3 invariant no longer exists; rewriting it as "stores live on Session post-Rotate and the failure path doesn't touch them" is captured in the new `TestSession_Rotate_ResetsCapabilityStores` test below.

**`tests/unit/aws_ses_invalidation_test.go`** (QA)
- Eight uses of `clients.SetRuleSets(...)` and `clients.RuleSets()` (lines 85, 122, 181, 314, 334, 337, 345, 432, 580, 717). Replace each construction of `*ServiceClients` + `SetRuleSets(...)` with `*aws.Scope{ Clients: …, RuleSets: session.NewRuleSetStore() }`. The fetcher-under-test (`sesActiveReceiptRuleSet`) now takes an explicit store, so tests pass it directly without going through a setter.

**`tests/unit/aws_ses_related_test.go`** (QA)
- Line 512: same rewrite as above.

### 4.4 Add new tests (QA)

1. `internal/session/session_test.go` — extend or add `TestSession_Rotate_ResetsCapabilityStores`:
   - Before Rotate: capture references to `s.IAMPolicies`, `s.IdentityStore`, `s.RuleSets`.
   - Call `Rotate()`.
   - Assert each field now holds a **distinct** reference (fresh store).
2. `internal/session/session_test.go` — add race tests under `-race`:
   - `TestSession_Rotate_RaceWithIAMPoliciesReader`: goroutine A reads `s.IAMPolicies.Lookup(...)`; goroutine B calls `Rotate()`. Expect no data race (the field is replaced wholesale, the old store's internal sync handles in-flight method calls).
   - Same for `IdentityStore` and `RuleSets`.
3. `tests/unit/aws_scope_test.go` (new) — verify:
   - `newScope` of a nil session returns nil.
   - `newScope` of a session with `Clients == nil` returns nil.
   - `newScope` returns a Scope whose fields point at the session's stores (reference equality).
4. `tests/unit/aws_scope_integration_test.go` (new) — for each of the four reader closures:
   - Construct `*Scope` with a stub store.
   - Invoke the registered closure (via `resource.GetFetchByIDs("policy")` etc.).
   - Assert the closure read from the scope's store, not from a bare `*ServiceClients`.
   - Confirm that passing a bare `*ServiceClients` to a Scope-expecting closure produces a clean error (Count: -1, or `nil, err`) rather than a panic.

## 5. Acceptance criteria (mirror of issue + spec-specific)

1. `ServiceClients` has **no** `storesMu`, `iamPolicies`, `identityStore`, `ruleSets` fields. Confirmed by `grep`.
2. `ServiceClients` has **no** `IAMPolicies() / SetIAMPolicies(...) / IdentityStore() / SetIdentityStore(...) / RuleSets() / SetRuleSets(...)` methods. Confirmed by `grep`.
3. The four reader sites (`iam_policies.go`, `glue_related.go`, `ebs_related.go`, `ses_related.go`) obtain their store from `*aws.Scope`.
4. `internal/runtime/handlers.go` no longer contains the bridge block (lines 210-220 and 253-261 of the pre-PR file).
5. `internal/tui/runtime_adapter_navigate.go` no longer contains `SetRuleSets` calls (the `RuleSets = session.NewRuleSetStore()` swap stays).
6. `internal/demo/client.go` no longer contains `SetIAMPolicies / SetIdentityStore / SetRuleSets` calls.
7. `Session.Rotate()` replaces each capability store with a fresh instance (already in place; new test pins it).
8. `make ready-to-push` green.
9. `make test-race` green.
10. Demo mode (`./a9s --demo`) still renders IAM policies, Glue tags, EBS Backup, SES rule-set checkers without panics.

## 6. Out of scope (carry forward, do NOT bundle)

- The architecture-doc rewrite that says "ServiceClients is transport-only" — already in place at `docs/architecture.md`. Spec linkage suffices.
- Generation-stamping of fetch results (AS-657 / AS-648 P2 #2).
- Any rename of `*ServiceClients` itself.
- Other AS-648 P2 items (#1, #3, #4, #6, #8, …).

## 7. Risk register

| Risk | Mitigation |
| --- | --- |
| Coder grep misses a `c.IAMPolicies()` / `c.SetIAMPolicies()` call site, build fails on undefined method. | That's the *good* failure mode — compiler catches it. Coder runs `make build` after each file edit; lint catches missed accessors. |
| Test that previously set up state via `clients.SetXxx(...)` instead silently dereferences a nil store. | QA's new `TestSession_Rotate_ResetsCapabilityStores` and `aws_scope_integration_test.go` pin nil-handling. Race test exercises store-mutation under concurrent reads. |
| Demo-mode fetcher hits a reader closure with a bare `*ServiceClients` instead of `*Scope`. | Runtime dispatcher branch on resource type is the single point of construction. Audit the four call sites in `internal/runtime/probes.go` and `internal/runtime/tasks*.go`. |
| Coder agent is currently in `error` (AS-144 platform issue). | Dispatch unblocks the moment the adapter recovers. QA can complete in parallel; Coder's first task is gated on the failing tests already on the branch. AS-660 is **not** blocked-by AS-144 — that platform issue is about Architect orchestration, not about workflow on this issue. |

## 8. Stage gates after this spec

- **Stage 3 (QA)**: writes failing tests per §4.4 and the rewrites in §4.3. Tests land on a feature branch and are committed *failing*. AS-660 child issue assigned to QA.
- **Stage 4 (Coder)**: implements §4.2, deletes the dead bridge code, runs `make ready-to-push` and `make test-race`. AS-660 child issue assigned to Coder, blocked on the QA child.
- **Stage 5 (Review)**: CodeReviewer + CodexReviewer + Architect (this size is L, ≥ M → I review) + CTO final. Architect runs `a9s-arch-review` against the diff and re-reads this spec doc.
- **Stage 6 (pre-push gate)**: `make ready-to-push` enforced; QA's race test gates separately.

## 9. Final disposition for AS-660

This spec is published. QA + Coder dispatched as child issues. AS-660 stays `in_progress` (per AS-630, no `in_review`) until Stage 5 verdicts land. Coder's adapter recovery is a parallel concern owned by CEO via AS-144; this issue is not blocked on it.
