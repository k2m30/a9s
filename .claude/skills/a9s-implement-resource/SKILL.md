---
name: a9s-implement-resource
description: Implement (or re-implement) an a9s resource type from its golden UX/UI spec at `docs/resources/<shortName>.md`. Use whenever the architect is asked to "implement", "wire up", "finish", "fix", or "rebuild" a resource that already has a spec doc — including cases where partial, stubbed, or buggy code exists and must be replaced. Treats the spec doc as the contract and the existing implementation as disposable. Reads ONLY the spec doc and four contract-surface files (`<shortName>_interfaces.go`, `<shortName>_related.go`, `<shortName>_issue_enrichment.go`, `<shortName>_detail_enrichment.go`); never reads existing tests or fetchers. Dispatches `a9s-qa` and `a9s-coder` via the scoped-handoff protocol. Cleans up stubs and "pretend to work" code tied to TBDs. Trigger this for any request that names a resource shortName and asks for implementation, tests, fixtures, or cleanup — even if the user doesn't explicitly mention the spec doc.
argument-hint: <shortName>
allowed-tools:
  - Read
  - Glob
  - Grep
  - Write
  - Edit
  - Bash(go doc *)
  - Bash(mkdir -p *)
  - Bash(ls *)
  - AskUserQuestion
  - Agent(a9s-qa)
  - Agent(a9s-coder)
  - Agent(a9s-devops)
  - Agent(general-purpose)
---

# a9s Resource Implementation Skill

Take a resource that already has a golden UX/UI spec at `docs/resources/<shortName>.md` and make the code match. Assume the current code is partial, stubbed, or bug-ridden. Assume the existing tests are disposable. The spec is the contract.

This skill is run **by `a9s-architect`**. The architect owns the phases and dispatches `a9s-qa` and `a9s-coder` via the scoped-handoff protocol defined in `.claude/agents/a9s-architect.md`. This skill gives the architect a fixed playbook so every resource lands the same way.

## Why it's structured this way

Three beliefs anchor everything below:

- **The doc is the should-be. The code is the is.** If they disagree, the code is wrong — unless the doc is stale, in which case we regenerate it first. Reading existing tests to "preserve coverage" bakes the old bugs into the new implementation. That's why this skill never reads old tests.
- **TBDs are visible product debt.** The spec makes missing decisions observable. The architect must resolve them with the user before any code lands, and any code that currently *pretends* to handle a TBD (silent fallback, stub value, hard-coded default) must be removed, not refactored.
- **Four files are enough to understand the contract surface.** The fetcher body, the existing tests, the mocks file, the fixtures — none of that needs reading. The interface file, the related-defs file, and the two enrichment files tell the architect what's plugged in and what the signatures look like. Everything else is implementation detail that the coder will rewrite anyway.

## Inputs

- `<shortName>` — e.g. `ec2`, `dbi`, `s3`. The spec doc at `docs/resources/<shortName>.md` must exist; if missing, stop and tell the user to run `a9s-resource-spec` first.

## Files the skill is allowed to read

**Required:**
- `docs/resources/<shortName>.md` — the contract.

**Contract surface (read all four if present — some resources don't have all):**
- `internal/aws/<shortName>_interfaces.go` — AWS SDK mock-interface abstractions.
- `internal/aws/<shortName>_related.go` (plus `<shortName>_related_extra.go` if present) — related-target registrations.
- `internal/aws/<shortName>_issue_enrichment.go` — Wave 2 enricher registration and function signature.
- `internal/aws/<shortName>_detail_enrichment.go` — detail-view enricher (may not exist for every resource; that's fine).

**Forbidden to read:**
- `tests/**` — anything under tests. The new tests come from the pseudocode spec (phase 3), not from legacy tests.
- `internal/aws/<shortName>.go` — the fetcher body itself. The interface file and the spec doc define the contract; reading the old fetcher leaks buggy shape into the new design.
- Any demo fixture under `internal/demo/`. The new fixtures come from phase 4, not from what was there before.

The architect may skim other files (message definitions, registry shape) to ground type signatures, but only the spec and the four surface files are load-bearing.

## Phases

The architect runs these in order. Phases 0–5 are the architect's own analysis. Phases 6–8 dispatch agents.

### Phase 0 — Intake

Confirm the spec doc exists. If it does not, stop. Tell the user: "No spec at `docs/resources/<shortName>.md`. Run the `a9s-resource-spec` skill first."

List which contract-surface files exist. Note which are missing; the coder will create them in phase 7.

### Phase 1 — Read the spec doc

Load `docs/resources/<shortName>.md` end to end. Extract:

- Identity: list API, describe API (if any).
- Related-panel targets with discovery mechanism and count-shown policy.
- Wave 1 signals with state bucket, field, severity implications.
- Wave 2 signals with API call, cost shape, severity.
- Wave 3 signals (explicitly out of scope — record but do not implement).
- Issue-visualization table: per-signal S4 list text, S5 detail text, surfaces reached.
- Out-of-Scope list.
- §6 citations — note any `a9s-devops` decisions and `user decision` entries so the architect knows which calls were already made.

Do **not** read existing code. Do **not** read existing tests.

### Phase 2 — Resolve TBDs with the user

Scan the loaded spec for remaining `TBD` markers and for signals explicitly deferred ("§5 Out of Scope per devops" etc).

For each, ask the user **one question at a time** via `AskUserQuestion` with three clearly labelled options:

- **resolve** — user provides the answer; architect updates the spec doc inline and adds a §6 citation `user decision (<date>): <answer>`.
- **defer** — mark the TBD as intentional; cleanup phase (8) deletes any stub code currently covering it, leaving a genuine gap rather than a silent lie.
- **out of scope** — the feature isn't happening at all; move the row from §3/§4 to §5 Out of Scope and delete related stubs.

Batch related TBDs into one question when the answer obviously applies to all. Record every answer in the spec's §6 Citations before moving on so the next phase works from a clean contract.

### Phase 3 — Behavioral test spec (pseudocode)

Write `docs/resources/<shortName>-impl-plan.md`. Section 1 is the pseudocode-test spec, one case per signal from §3 and §4 of the spec:

```text
TEST: <short name>
GIVEN: <AWS fixture state in plain english — e.g. "an EC2 instance with State.Name = stopped, StateReason.Code = Server.SpotInstanceShutdown">
WHEN:  the list is fetched and rendered
THEN:
  - row color = red
  - S4 text contains "stopped: Server.SpotInstanceShutdown"
  - S5 sentence contains "Instance stopped by AWS spot reclamation"
  - S1 issues count does NOT bump (Wave 1 signals don't reach S1)
  - no `!` / `~` glyph (forbidden on non-green rows)
```

One case per row in §4 of the spec. Additionally:

- A **silence test** for the Healthy happy path: row green, S4 blank, no finding, no count.
- One **anti-test** per Wave 3 OUT OF SCOPE item: if a fixture includes this condition, the spec must NOT surface anything.

Keep cases plain. The QA agent turns them into Go tests in phase 6; the pseudocode stays as the human-readable contract in the impl-plan doc.

### Phase 4 — Fixture list (plain language)

Section 2 of the impl-plan doc. One fixture per test case in phase 3, described as a natural-language sentence plus the exact AWS field values needed. Example:

```text
FIXTURE: ec2-stopped-spot-reclaim
A stopped EC2 instance. State.Name = "stopped". StateReason.Code = "Server.SpotInstanceShutdown".
StateReason.Message = "Instance was stopped due to spot reclamation at 2026-04-12T14:00:00Z".
StateTransitionReason = "User initiated (2026-04-12 14:00:00 GMT)".
All other fields use sensible defaults from a typical t3.medium in us-east-1.
```

Group fixtures by reuse — e.g. one baseline "healthy instance" fixture that several tests mutate. The QA agent uses this list to build typed fakes; the coder does not read it directly.

### Phase 5 — Contract surface read

Read the four files listed above. From each, extract only:

- **`<shortName>_interfaces.go`**: the aggregate service interface name, every narrow `*API` interface already declared, every AWS SDK method signature wired in. The coder needs to extend this; the QA agent needs these signatures to write mocks.
- **`<shortName>_related.go` (+ `_extra.go`)**: existing `RegisterRelated` calls — which targets are already wired, which fields they read. Compare to §2 of the spec. Mark deltas.
- **`<shortName>_issue_enrichment.go`**: the registered enricher function signature, the AWS API it calls, whether it's registered at package init. Compare to §3.2 of the spec.
- **`<shortName>_detail_enrichment.go`** (if present): same shape as issue enrichment but for detail-view fields. If absent, note that.

Append a "Contract surface gap analysis" section to the impl-plan doc listing: what the spec demands, what the four files currently provide, the delta the coder must close.

### Phase 6 — QA handoff (Mode: score → execute)

Use the architect's scored QA handshake. The scope rule:

- QA creates brand-new test files from the pseudocode spec (phase 3) and fixtures from the list (phase 4). QA does **not** read `tests/**` for prior art.
- Overwrite existing test files for this resource wholesale. Uniformity beats preservation — matches the rule that drives the golden spec itself.

QA task shape:

```text
## QA TASK: Tests for <shortName> from spec
Mode: score
Parallelization: sequential (before coder)

### Test files to create (or overwrite):
- tests/unit/aws_<shortName>_test.go — fetcher tests per §3.1 Wave 1 signals in docs/resources/<shortName>.md
- tests/unit/aws_<shortName>_issue_enrichment_test.go — Wave 2 enricher tests per §3.2
- tests/unit/aws_<shortName>_detail_enrichment_test.go — detail enricher tests (if §2 demands one)
- tests/unit/aws_<shortName>_related_test.go — related-target discovery tests per §2
- internal/demo/fixtures/<shortName>_fixtures.go — typed fakes derived from the impl-plan fixture list

### What to test:
- Every signal row in §4 of docs/resources/<shortName>.md becomes one test case.
- Every related target in §2 becomes one discovery test.
- Silence test: Healthy fixture → row green, S4 blank, no finding.
- Out-of-Scope anti-tests for every §3.3 Wave 3 entry — if a fixture hits that condition, no surface reaches it.

### Forbidden inputs:
- Do not read existing tests/unit/*<shortName>*.md — rewrite from the pseudocode spec.
- Do not read internal/aws/<shortName>.go — you are testing the contract, not the implementation.

### Context files (read-only):
- docs/resources/<shortName>.md — the behavioral contract
- docs/resources/<shortName>-impl-plan.md — pseudocode + fixture list
- internal/aws/<shortName>_interfaces.go — mock signatures
- internal/resource/resource.go — Resource struct and EnrichmentFinding for assertions
```

QA replies `SCORE: <N> — <rationale>`. Architect accepts or reworks. On accept, re-dispatch same scope with `Mode: execute` and `Confirmed score: <N>`.

### Phase 7 — Coder handoff

Coder task shape:

```text
## CODER TASK: Implement <shortName> against the spec
Parallelization: sequential (after QA execute)

### Files to create or overwrite:
- internal/aws/<shortName>.go — fetcher; one AWS List/Describe per the Identity section
- internal/aws/<shortName>_interfaces.go — add any missing narrow interface
- internal/aws/<shortName>_related.go — RegisterRelated for every target in §2
- internal/aws/<shortName>_issue_enrichment.go — Wave 2 enricher per §3.2
- internal/aws/<shortName>_detail_enrichment.go — only if §2 of the spec requires detail fields beyond the list shape

### Expected behavior:
- Fetcher maps AWS fields to resource.Resource per §2 Identity section.
- Status/S4 column carries the exact text in the §4 "List text" column, never a bare state keyword.
- Row color follows state bucket from §3.1.
- Wave 2 enricher populates resource.EnrichmentFinding with the exact Summary from §4 "Detail text" column.
- No invented UI. No row `·` dot. No `⚠ Background Check` header. No derived banner. See spec §"Allowed visualization surfaces".

### Forbidden inputs:
- Do not read tests/** — you write against contract, not test machinery.

### Cleanup pass (REQUIRED — same PR):
Delete every stub, commented-out block, or "pretend-to-work" fallback related to TBDs now resolved in phase 2:
- Hard-coded defaults that silently cover a deferred signal → remove.
- Empty switch branches that log and return nil → remove.
- Functions named `TODO*` / `FIXME*` / `stub*` / `fake*` in production paths → remove.
- Fields on resource.Resource populated with literal "" or 0 where the spec says "blank" — fine; where the spec demands a real value — remove the lie, fail closed.

### Context files (read-only):
- docs/resources/<shortName>.md — the contract
- docs/resources/<shortName>-impl-plan.md — pseudocode + fixtures + contract-surface gap analysis
- internal/aws/<shortName>_interfaces.go — current mock surface
- internal/resource/resource.go — Resource struct and EnrichmentFinding
- AWS SDK Go v2 types via `go doc github.com/aws/aws-sdk-go-v2/service/<svc>/types.<Shape>`
```

### Phase 8 — Verify and report

After coder finishes, verify:

- `make test` passes for the new test files.
- `make lint` passes.
- No stubs remain — `rg -n 'TODO|FIXME|stub|fake' internal/aws/<shortName>*.go` returns nothing except legitimate comments.
- Every Wave 1/Wave 2 signal in §4 maps to at least one passing test.

Report to the user in one block:

```text
<shortName>: tests=<N> passing; fixtures=<N>; tbds resolved=<N> / deferred=<N> / oos=<N>; stubs removed=<N>.
Implementation ready for review at internal/aws/<shortName>*.go
```

## What this skill never does

- Does not commit or push. The architect may, the skill does not.
- Does not touch unrelated resources. Scope is one shortName per invocation.
- Does not skip phase 2. If there are TBDs, they get resolved before any code moves.
- Does not "preserve" existing test expectations. The pseudocode spec in phase 3 is authoritative; anything in `tests/**` that contradicts it is wrong by definition.

## Handling a spec change mid-flight

If phase 2 produces a TBD answer that materially changes §2 or §4 of the spec doc, update the spec first, then restart phases 3 and 4 from the amended spec. The impl-plan doc always reflects the current spec. Cheaper than discovering the contradiction in phase 7.

## What to do when the spec is wrong

Occasionally the spec has an actual factual error — e.g. an AWS field that doesn't exist on the list response. Stop the skill and regenerate the spec (`a9s-resource-spec <shortName>`) first. Do not patch around the error at the impl-plan level; that just moves the drift.
