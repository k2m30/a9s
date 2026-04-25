# Detail View Test Coverage Guide

How to properly cover a resource's detail view + all related resources with tests. Distilled from the ct-events v2.1 test-suite work where shortcuts cost multiple rework rounds.

Target audience: architect agents scoping test tasks, QA agents writing them, humans reviewing coverage claims.

---

## 1. Scope — what "properly covered" actually means

"Detail view covered" ≠ "the tests pass." The user's definition is: **every visible interaction lands on a real resource and you can prove it**.

Six distinct layers must be tested independently. Each layer catches a class of bugs the other layers miss.

| Layer | What it catches | What it misses |
|-------|-----------------|----------------|
| **1. Golden snapshot** — pixel-exact rendered `View()` per case | Layout drift, section ordering, spacing, typography | Navigation, dispatch, resolution |
| **2. Section structure** — assertions about which sections appear, their order, which rows they contain | Missing/extra sections, wrong ERROR hoist position, drop-boring-defaults violations | Rendering bugs, navigation |
| **3. Navigation dispatch** — cursor to a navigable row + enter captures a `RelatedNavigateMsg{TargetType, TargetID}` | Wrong TargetType, wrong TargetID | Whether the TargetID actually resolves to a real resource |
| **4. Navigation resolution** — the dispatched TargetID must exist in `demo.GetResources(TargetType)` by `r.ID == TargetID` | Fixture drift, ARN→name extraction bugs, missing fixtures | Whether pushing the target view actually produces the right UI state |
| **5. End-to-end navigation** — run the dispatched `tea.Cmd` through the root `Model.Update` and inspect the resulting view stack | Bugs in `handleRelatedNavigate`, child-type resolution, filter propagation | — (this is the deepest layer) |
| **6. Related column coverage** — right-panel typed groups + pivot rows | All right-column behavior | — |

**Every layer is non-negotiable for full coverage.** Skipping layer 4 is how "links lead to random resources" bugs ship. Skipping layer 6 is how the right panel breaks silently.

---

## 2. The fixture alignment invariant

The load-bearing rule that makes everything else work:

> **Every ID referenced inside a demo fixture's event payload (ARNs, keys, IDs) must exist as a real `Resource.ID` in the corresponding target type's fixture set.**

Concrete examples from ct-events:
- Event JSON contains `arn:aws:iam::333:user/bob` → the `iam-user` fixture set must have an entry with `ID: "bob"` (the extracted bare name, NOT the full ARN)
- Event JSON contains `requestParameters.bucketName: "prod-logs"` → the `s3` fixture set must contain `ID: "prod-logs"`
- Event JSON contains `keyId: "2f7e9a5b-..."` → the `kms` fixture set must contain that exact UUID

If the extractor produces a different form than the fixture IDs store (e.g. extractor → `bob`, fixture → `bob.smith`), navigation silently falls through to random-list mode. The app "works" in the sense that nothing crashes — it just lies to the user.

### Enforce the invariant with a test

Every navigation test that captures a `RelatedNavigateMsg` MUST also assert:

```go
targets, err := demo.GetResources(nav.TargetType)
if err != nil || len(targets) == 0 {
    t.Fatalf("demo.GetResources(%q) unavailable — fixture missing", nav.TargetType)
}
found := false
for _, r := range targets {
    if r.ID == nav.TargetID {
        found = true
        break
    }
}
if !found {
    ids := collectIDs(targets)
    t.Errorf("TargetID %q not found in demo.GetResources(%q). Available: %v",
        nav.TargetID, nav.TargetType, ids)
}
```

An empty fixture set MUST fail (`t.Fatalf`), not skip. An empty set is a bug: either the fixture is missing or `GetResources` took the wrong argument.

---

## 3. Coverage matrix

For resource type **X**, build this matrix before writing any tests:

### 3.1 Case corpus

Every wireframe case / demo fixture of X. One row per fixture. For ct-events this was 9 cases (A–I). For a simpler resource type it might be 1–3 cases.

### 3.2 Per-case navigable field inventory

For each case, enumerate:
- **Left-column navigable fields**: rows in the detail body where `IsNavigable == true` (Principal, Bucket, Object, Instance, Key, Role, User, etc.)
- **Negative coverage**: fields that should NOT be navigable (e.g. Root principal, AWSService row)

### 3.3 Per-case expected related groups

For each case, enumerate:
- **Typed groups**: each entry in `RegisterRelated("X", ...)` with the expected `Count` for that specific event
- **Pivot rows**: each entry with `Count: -1` and a `FetchFilter`
- **Negative coverage**: groups expected to be `Count: 0` (visible but dim)

### 3.4 Expected target IDs

For every navigable pair (case, field), the exact `TargetID` string that navigation will dispatch. This is the source of truth the test asserts against — no inference, no "whatever the current code produces."

Example ct-events Case C:

| Case | Field | TargetType | Expected TargetID | Resolves to fixture? |
|------|-------|------------|-------------------|----------------------|
| C | Principal (IAMUser bob) | `iam-user` | `bob` | `bob` ∈ iam-user fixtures |
| C | Bucket | `s3` | `prod-logs` | `prod-logs` ∈ s3 fixtures |
| C | Object | `s3` | `prod-logs` (bucket, not object path) | same |
| C | [related] IAM Users | `iam-user` | `bob` | same |
| C | [related] S3 Buckets | `s3` | `prod-logs` | same |

Build this matrix for every case. **No shortcuts.** The matrix IS the test's expected values.

---

## 4. TDD protocol — non-negotiable

Per the project constitution Principle I and hard lessons from this session:

### 4.1 QA first, always

Every production code change driven by tests. No exceptions. Even for "obvious" bug fixes — write the failing test first. The failing test is the contract that defines what "fixed" means.

### 4.2 QA value-score handshake before execute

Before any QA dispatch writes a file, the architect runs the score handshake:

1. Dispatch in `Mode: score` — QA returns `SCORE: <N> — <rationale>` without writing anything
2. Architect reads the score + rationale, decides to accept/rework/drop
3. If accepted, re-dispatch in `Mode: execute` with `Confirmed score: <N>`

**Drop tests scoring ≤30 unless there's a specific reason not to.** Tautological checks, duplication of existing coverage, and "assertion exists" tautologies are busywork.

**Override low scores when the existing coverage has demonstrable holes.** If your coverage judgment has been wrong recently (bugs shipped despite "covered" tests), treat near-duplicates as cheap insurance.

### 4.3 Never weaken assertions to make tests pass

If a test fails with a buggy expected value → the test is correct, the production code is wrong. FIX the production code.

If a coder reports "test fails, I updated the expectation to match the actual behavior" → the coder made the bug permanent. Reject and re-dispatch.

Example from this session: 4 Principal nav tests (A, C, F, G) silently updated their expected TargetID from `bare_name` to `arn:aws:sts::...` because the production code dispatched the full ARN. Those "passing" tests LOCKED THE BUG. When the user ran the app, navigation landed on random resources because no role ID matched the ARN. The fix was to restore the original assertions (bare name) and fix the production code.

### 4.4 Never ship a "things to verify manually" list

After the test suite passes, do NOT hand the user a checklist of "run this, press that, confirm the thing." If something is worth verifying, it's worth a test. Handing a manual-verification list is outsourcing QA to the user and guarantees bugs ship.

The correct end of a test dispatch batch: "N tests passing, 0 failing, here's the delta." Not "please run `./a9s --demo` and verify X, Y, Z."

### 4.5 Rebuild the binary after production code changes

Per `CLAUDE.md`: `go build -o a9s ./cmd/a9s/` after every production change. Otherwise the `--demo` run uses a stale binary and the user sees the old buggy behavior. Multiple fix dispatches during this session got blamed for "not fixing anything" because the coder didn't rebuild at the end.

---

## 5. Test dispatch checklist

For each resource type X being covered, dispatch atomic QA tasks in this order. Each is independent and parallel-safe unless noted.

### Pre-flight (architect reads these before dispatching)

- [ ] Demo fixtures for X exist and render a usable detail view
- [ ] `RegisterRelated("X", ...)` is called and the expected TypedGroup set is documented in a design doc
- [ ] The expected pivot rows are documented
- [ ] Every ID referenced in X's demo event payloads exists in the corresponding target type's fixture set (fixture alignment invariant — §2)

### Layer 1: Golden snapshots (one dispatch per case)

- [ ] For each case, a `TestXDemoGolden_CaseK` function that loads the demo fixture, renders `View()` at a wide terminal (e.g. 180×40), strips ANSI, compares against a committed file under `tests/testdata/golden/x_demo_case_k.txt`
- [ ] First run: `UPDATE_GOLDEN=1 go test` to seed, then commit the golden file
- [ ] Golden files are inspected visually at commit time — don't rubber-stamp

### Layer 2: Section structure (one or two dispatches)

- [ ] Section ordering: expected order per design doc (e.g. ACTOR → ACTION → TARGET → CONTEXT → ERROR → REQUEST → RESPONSE)
- [ ] Conditional sections (e.g. ERROR only when error present) appear in the right position
- [ ] Empty-section omission rule: sections with zero rows are dropped entirely
- [ ] Drop-boring-defaults: specific rows that should never appear regardless of input

### Layer 3: Left-column navigation dispatch (one dispatch per case)

- [ ] For every navigable field in every case: cursor walk + enter → captures a `RelatedNavigateMsg`
- [ ] Assert exact `TargetType`, `TargetID`, `SourceType` — the matrix from §3.4 is the source of truth
- [ ] Negative coverage: non-navigable rows (Root, Service, etc.) dispatch nil or a non-navigate message

### Layer 4: Navigation resolution (extend Layer 3 subtests)

- [ ] For every navigable subtest, after capturing the dispatch, call `assertTargetResolves(t, nav)` which looks up `nav.TargetID` in `demo.GetResources(nav.TargetType)`
- [ ] Empty fixture set is a hard fail, not skip
- [ ] Failure message shows the expected TargetID + available fixture IDs

### Layer 5: End-to-end navigation follow-through

- [ ] For each navigable pair, run the dispatched `tea.Cmd` through the root `Model.Update`
- [ ] Assert the view stack has a new view pushed
- [ ] Assert the new view is the expected type (resource list, detail, filtered list)
- [ ] Assert the new view shows the expected resource (cursor on it, or it's the only item, or title matches)
- [ ] **Limitation**: if `Model` has all unexported fields and no `export_test.go` test helpers, this layer requires a refactor to extract a testable helper (e.g. `resource.ResolveNavigationTarget` was extracted during this session for this reason). Document the refactor as a separate dispatch.

### Layer 6: Right-column related groups

- [ ] Registration test: `TestXRelatedGroups_AllTypedRegistered` asserts every expected TargetType from the design doc is in `GetRelated("X")`
- [ ] Per-case count test: for each case, for each registered group, assert the expected count (many will be zero — that's fine, it's negative coverage)
- [ ] Per-case dispatch test: cursor to each non-zero related row + enter → captures the correct `RelatedNavigateMsg`
- [ ] Resolution test: every right-column navigation's TargetID resolves via §2 invariant
- [ ] Pivot rows: `Count: -1` with `FetchFilter` — separate dispatch test because the flow differs

---

## 6. Pitfalls (learned the hard way)

### 6.1 Stale gopls diagnostics

The editor's LSP cache lags behind the actual file state during rapid agent iterations. 15+ times in this session a diagnostic said "undefined X" or "X imported not used" while `go build` was clean. **Always verify with `rtk go build` before acting on a diagnostic.** Never dispatch a "fix" based on a diagnostic alone.

### 6.2 ARN vs bare-name TargetID

CloudTrail ARNs look like `arn:aws:sts::123:assumed-role/RoleName/session`. If the extractor stores the full ARN in `Row.Value` and the navigation dispatch uses `item.Value` as TargetID, navigation will never match because a9s's role registry keys by bare name.

Fix pattern: add a separate `NavID` field to `Row` / `FieldItem` for the navigation identifier, keeping `Value` as the display string (per wireframe).

### 6.3 Child types vs top-level types

`resource.FindResourceType(name)` only searches top-level types. Child types (e.g. `s3_objects`) registered via `RegisterChildType` are invisible to it. `handleRelatedNavigate` must fall back to `GetChildType` or use a unified `ResolveNavigationTarget` helper, otherwise pressing enter on a child-type related row produces "unknown resource type: s3_objects".

### 6.4 `fmt.Sprintf("%v", mapValue)` in a generic walk

Go's default formatting of `map[string]any` produces `map[k1:v1 k2:v2]` — ugly, unstable, unfit for UI. Use explicit type switches in summarizers and render slices/maps with readable formatting (JSON-like compact form or one row per key).

### 6.5 `assertTargetResolves` is not the same as actual resolution

Looking up `TargetID` in `demo.GetResources(TargetType)` is Layer 4 — it verifies the ID matches a fixture. It does NOT verify that `handleRelatedNavigate` actually pushes the right view (Layer 5). Both layers catch different bugs. Don't conflate them.

### 6.6 "Nothing changed" = you didn't rebuild the binary

If the user runs `--demo` and says "nothing changed," 90% of the time it's because no one ran `go build -o a9s ./cmd/a9s/` after the last fix. Put the rebuild in the verification checklist of every coder dispatch.

### 6.7 Async agent file collisions

Parallel agents editing the same file (e.g. 7 golden-test dispatches all appending to `ctdetail_demo_golden_test.go`) will collide on stale reads and silently drop each other's edits. If collisions are possible, serialize them or split into different files from the start.

### 6.8 Right-column checker counts in demo mode

`RegisterRelatedDemo` overrides the production `Checker` with a demo-mode flat-return. If the override returns `Count=0` for every group regardless of event, the right column renders stubs forever. Each case needs a per-event switch in the demo checker to produce the right counts.

### 6.9 Design doc section references can be stale

Design docs referencing "§7b.10 registers 14 typed groups" may not match the current code's 2 registered groups. When in doubt, grep the actual registrations and treat the doc as advisory until verified.

### 6.10 "Already covered by existing test" is a dangerous claim

When QA scores a test as low because an existing test "already covers it," verify the overlap claim explicitly. This session had multiple cases where "already covered" meant "a different test touches the same function" — not actually the same contract. If your coverage judgment has been wrong recently, override the low score and write the duplicate test as cheap insurance.

---

## 7. Minimum test count estimate

For a resource X with:
- C cases
- F average navigable fields per case
- G typed related groups
- P pivot rows

Full coverage is approximately:
- Layer 1: **C** golden tests
- Layer 2: **~5–10** structure assertions
- Layer 3 + 4: **C × F** navigation tests (each with dispatch + resolve assertions)
- Layer 5: **C × F** end-to-end tests (if Model is testable; otherwise fewer via a resolver helper)
- Layer 6 registration: **1**
- Layer 6 counts: **C × G** subtests
- Layer 6 dispatch + resolve: **~C × G / 3** (non-zero counts only)
- Layer 6 pivots: **P**

For ct-events (C=9, F≈2.5, G=13, P=4): ~180 tests minimum. This session landed ~170 and still had gaps; 180–200 is the realistic target.

**If the total test count looks like "~20 tests for a detail view with 9 cases and a right column" — coverage is incomplete.** Ask "what bug would I miss?" for every skipped layer.

---

## 8. Dispatch-order template

A concrete dispatch sequence that respects TDD and produces working coverage:

1. **Reconnaissance** (architect reads design docs + current code state)
2. **Fixture alignment audit** (architect manually cross-references event-payload IDs against target fixtures — §2 invariant)
3. **Layer 6 registration test** (QA score → execute; will fail if any group is unregistered)
4. **Layer 6 registration fix** (coder — add missing `RegisterRelated` entries + checkers)
5. **Layer 6 per-case count tests** (QA score → execute; will fail against demo stubs)
6. **Layer 6 demo checker fix** (coder — per-event switch in `RegisterRelatedDemo`)
7. **Layer 1 golden snapshots** (QA execute, one dispatch per case; parallelize cautiously — file collision risk)
8. **Layer 3 + 4 navigation matrix tests** (QA score → execute, one dispatch per case)
9. **Layer 3 + 4 failures drive fixture alignment** (coder — add missing target fixtures OR fix extraction)
10. **Layer 5 helper refactor if needed** (coder — e.g. `ResolveNavigationTarget`)
11. **Layer 5 helper test** (QA score → execute)
12. **Rebuild binary** (coder OR explicit verification step)
13. **User runs `--demo` for visual acceptance** (optional, as a sanity check — NOT as the primary coverage mechanism)

---

## 9. Anti-checklist — things that feel like coverage but aren't

- ✗ "The test suite is at N,NNN passing" (counts without a coverage model mean nothing)
- ✗ "Here are things to verify manually" (outsourced QA)
- ✗ "I captured the dispatched message and it looks right" (proxy for navigation, not real navigation)
- ✗ "The golden file matches" (catches drift, not semantics)
- ✗ "The test passes" (passing tests can lock bugs if assertions are weak)
- ✗ "Already covered by TestXxx" (verify overlap or write the duplicate as cheap insurance)
- ✗ "This is a rigid pattern, skipping the score handshake" (protocol exists for a reason)
- ✗ "The design doc says N groups are registered" (verify with grep)
- ✗ "The build succeeded, diagnostics are probably stale" (right 80% of the time, catastrophic 20%)

---

## 10. TL;DR for future architects

If you're scoping a detail view coverage task:

1. **Build the fixture alignment matrix FIRST.** Every ID in every event payload maps to a fixture ID in the target set. No matrix → no coverage.
2. **QA writes tests for every layer.** Golden, structure, dispatch, resolve, end-to-end, related column (typed + pivots). Every layer catches different bugs.
3. **Score handshake before every QA execute.** Drop tautologies, override "already covered" when coverage has recently been wrong.
4. **Never weaken assertions.** If the test fails, the code is wrong. Re-dispatch the coder with the failing test as the contract.
5. **Never ship a manual-verification checklist.** If it matters, it's a test.
6. **Rebuild the binary at the end.** Or the user will tell you "nothing changed."
7. **Verify stale diagnostics with `rtk go build`.** Don't dispatch fixes for hallucinated compile errors.
