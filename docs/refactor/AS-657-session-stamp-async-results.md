# AS-657 — Session-stamp ResourcesLoaded / IdentityLoaded / ValueRevealed and reject stale

**Parent**: AS-648 P1 (#2, #3, #4) · **Sizing**: M · **Owner**: Architect (spec), QA (tests), Coder (impl)

## Problem

Three async-result message types are dispatched from `internal/tui/fetch_adapter.go` without a generation stamp. When the user switches profile or region mid-flight, `Session.Rotate()` bumps every counter, but a pre-rotation result still arrives at the post-rotation `Update` loop and overwrites current state:

| Message | Stale-arrival damage |
|---|---|
| `ResourcesLoaded` | `m.allResources` mutated, write-through `ResourceCache` poisoned with old-account data |
| `IdentityLoaded` | Header renders the **previous account ID** after switching |
| `ValueRevealed` | Reveal view pushed with a **secret value from the previous profile** — direct leak |

`AvailabilityChecked` and `ClientsReady` already implement `messages.GenStamped` and are dropped at the central guard in `Core.HandleEvent` (`internal/runtime/orchestrator.go:51-54`) and at the inline `messages.IsStale` checks in `internal/tui/app.go` (lines 406, 425) for adapter-side handlers. The three vulnerable messages are routed adapter-side and bypass the central guard entirely.

## Non-goals

- Removing `iamPolicies/identityStore/ruleSets` from `ServiceClients` — AS-648-h5.
- `probeEnrichment` demo-mode guard — AS-648-h3.
- `AvailabilityPrefetched` zero-gen handling — AS-648-h4.
- Resource-type guard in `ResourceListModel.Update` — AS-648-h1.

## Design

### 1. Message contract (`internal/runtime/messages/event.go`)

Add `Gen domain.Gen` plus the three-method `GenStamped` implementation to each message. Use the precedent of `ClientsReady` (lines 75-86) and `AvailabilityChecked` (163-177).

| Message | Aspect | AcceptZeroGen |
|---|---|---|
| `ResourcesLoaded` | `AspectAvailability` | `true` |
| `APIError`        | `AspectAvailability` | `true` |
| `IdentityLoaded`  | `AspectConnect`      | `true` |
| `IdentityError`   | `AspectConnect`      | `true` |
| `ValueRevealed`   | `AspectConnect`      | `true` |

Rationale for aspects:

- `ResourcesLoaded` is gated by the same generation that gates probes against this account/region (`AvailabilityGen`). This is the counter that `probe_adapter.go` already consults for the rerun-token path (`TypeGen` is a separate per-type counter; `Gen` here is the session-wide AvailabilityGen).
- `IdentityLoaded` / `IdentityError` / `ValueRevealed` are tied to the active AWS connection — `ConnectGen` is bumped on every profile/region switch (`session.go:230`), so a result fetched against an old `Clients` is correctly rejected.
- `APIError` and `IdentityError` mirror the success branch's aspect so a stale error does not flash "this region failed" for a region the user already left.

`AcceptZeroGen: true` matches every other GenStamped event except `AvailabilityChecked` — zero is the test/demo sentinel that always passes. AvailabilityGen and ConnectGen are bumped by `Rotate()` away from zero on the first switch, so a real production stamp will never be zero on the failing path.

### 2. Dispatch sites (`internal/tui/fetch_adapter.go`)

Mirror `connectAWS` (line 160-171): the caller threads the captured generation in as a parameter, the closure stamps it onto both the success and error branches.

| Function | New signature | Gen source at call site |
|---|---|---|
| `fetchResources(resourceType string, gen domain.Gen)` | + `gen` | `m.core.Session().AvailabilityGen` |
| `fetchResourcesFiltered(resourceType, filter, gen)` | + `gen` | `m.core.Session().AvailabilityGen` |
| `fetchChildResources(childType, parentCtx, gen)`     | + `gen` | `m.core.Session().AvailabilityGen` |
| `fetchMoreResources(msg, gen)`                        | + `gen` | `m.core.Session().AvailabilityGen` |
| `fetchIdentity(gen)`                                   | + `gen` | `m.core.Session().ConnectGen` |
| `fetchRevealValue(resourceType, resourceID, gen)`     | + `gen` | `m.core.Session().ConnectGen` |

The capture must happen at the call site (the synchronous handler that builds the `tea.Cmd`), not inside the closure. This is the same pattern as `connectAWS`: capturing inside the closure would read whatever `Session.ConnectGen` is at the time the goroutine runs, which is exactly the post-rotation value we are trying to reject. `fetchAMIDetail` is out of scope — its handler routes through `Flash`/`Navigate`, not a GenStamped event.

Update every branch in each `tea.Cmd` to stamp `Gen: gen`:

```go
return messages.ResourcesLoaded{
    ResourceType: resourceType,
    Resources:    res.Resources,
    Pagination:   res.Pagination,
    Gen:          gen,
    Err:          err,
}
// and
return messages.APIError{
    ResourceType: resourceType,
    Err:          err,
    Gen:          gen,
}
```

### 3. Caller updates

All callers of the six functions above must thread the gen from session. Call sites confirmed:

- `internal/tui/app.go:302, 304, 308` — `fetchChildResources` / `fetchResources` / `fetchMoreResources`
- `internal/tui/app_screens.go:48` — `fetchChildResources`
- `internal/tui/app_input.go:71` — `fetchIdentity`
- `internal/tui/runtime_adapter.go:139` — `fetchIdentity`
- `internal/tui/runtime_adapter_navigate.go:347, 352, 532, 537, 540, 558` — mix of `fetchResources`, `fetchResourcesFiltered`, `fetchChildResources`, `fetchRevealValue`
- `internal/tui/runtime_adapter_related.go:98, 120, 434, 436, 448` — same set
- `internal/tui/probe_adapter.go:139-151` — the `probeEnrichment` wrapper reads back the produced `ResourcesLoaded`; **the wrapper itself does not need to stamp**, but it must pass `gen` through when re-emitting (verify by reading lines 130-160).

### 4. Stale-rejection guard (`internal/tui/app.go`)

Add an inline `messages.IsStale` check at the top of each of the five case branches, mirroring the precedent at app.go:406 (`EnrichDetailResult`) and 425 (`RelatedCheckResult`). Drop on stale:

```go
case messages.ResourcesLoaded:
    if messages.IsStale(msg, m.core.Session()) {
        return m, nil
    }
    m.flash.active = false
    // ...existing body unchanged
```

Same shape for `messages.APIError` (line 310), `messages.IdentityLoaded` (391), `messages.IdentityError` (393), `messages.ValueRevealed` (295). The check must run **before** `deriveFindingsForType`, `updateActiveView`, write-through cache assignment, and view push — otherwise the stale message still mutates state.

Do **not** route these messages through `coreUpdate`/`Core.HandleEvent`. The central guard would catch them once they implement `GenStamped`, but the existing handlers (`handleValueRevealed`, `handleIdentityLoaded`, the inline `ResourcesLoaded` body) live adapter-side and would need to move to core. That is a separate refactor.

### 5. The `--demo` and `--no-cache` happy path

These code paths use `AvailabilityPrefetched` (which is already stamped) and a synchronous bootstrap; they do not call `Rotate()` before the first `ResourcesLoaded`. Acceptance criterion #4 covers this — `AvailabilityGen` is at its initial value when bootstrap fetches dispatch, the captured `gen` matches, and `IsStale` returns false.

## Acceptance criteria

1. A `ResourcesLoaded` carrying `Gen = N-1` after `Rotate()` has bumped `AvailabilityGen` to `N` is dropped: `m.allResources` unchanged, no write-through `ResourceCache` write.
2. An `IdentityLoaded` carrying a stale `ConnectGen` is dropped: `Session.Identity` and the header are unchanged. (Closes "old account in header after switch".)
3. A `ValueRevealed` carrying a stale `ConnectGen` is dropped: reveal view not pushed, secret value not rendered. (Closes the secret-leak path.)
4. Legitimate happy path with matching gen is unaffected; `--demo` and `--no-cache` startup still populate lists; existing tests continue to pass.
5. A stale `APIError` does not flash "fetch <type>: ..." for a region the user already left; same for `IdentityError`.

## Files in scope (whitelist)

**Production code (Coder):**

- `internal/runtime/messages/event.go` — add `Gen` field + 3-method impl on 5 messages.
- `internal/tui/fetch_adapter.go` — 6 function signatures + 9 stamp sites (4 success + 4 error `APIError` branches + identity success/error + reveal success/error).
- `internal/tui/app.go` — 5 `IsStale` guard insertions, plus update 3 fetch call sites (lines 302, 304, 308) to pass gen.
- `internal/tui/app_screens.go` — update `fetchChildResources` call site (line 48).
- `internal/tui/app_input.go` — update `fetchIdentity` call site (line 71).
- `internal/tui/runtime_adapter.go` — update `fetchIdentity` call site (line 139).
- `internal/tui/runtime_adapter_navigate.go` — update 6 call sites.
- `internal/tui/runtime_adapter_related.go` — update 5 call sites.
- `internal/tui/probe_adapter.go` — pass-through verification (read lines 130-160; stamp may be unchanged or must propagate).

**Out of scope for production code:**

- `internal/runtime/orchestrator.go` — the central guard already works through the `GenStamped` interface; no change needed.

**Test code (QA):**

- New file `tests/unit/generation_stamping_fetch_test.go` — covers AC #1–#5 with one test per criterion. Use the `qa_clients_ready_flash_gen_test.go` precedent for harness shape (real Session, Rotate(), synthesized messages, assert state after `coreUpdate`/`Update`).
- Existing files that may grow with one extra subtest:
  - `tests/unit/app_fetchers_dispatchers_test.go` — confirm the 6 fetch functions capture gen at dispatch (not in the closure).
  - `tests/unit/qa_clients_ready_flash_gen_test.go` — the existing harness for the flash-on-stale pattern.

## Verification

- `make build` — type-checks the new fields and method receivers.
- `make test` — runs the new generation-stamping suite plus the existing suite.
- `make lint && make security && make gofix` — clean.
- `make ready-to-push` — full Stage 6 gate.

## Out of scope (recap, for the reviewer)

This PR does **not** touch:

- AS-648-h1 resource-type guard in `ResourceListModel.Update`.
- AS-648-h3 `probeEnrichment` demo-mode guard.
- AS-648-h4 `AvailabilityPrefetched` zero-gen handling.
- AS-648-h5 `ServiceClients` slimming.
