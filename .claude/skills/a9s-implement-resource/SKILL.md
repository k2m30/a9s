---
name: a9s-implement-resource
description: Implement (or re-implement) an a9s resource type from its golden UX/UI spec at `docs/resources/<shortName>.md`. Use whenever the user asks to "implement", "wire up", "finish", "fix", or "rebuild" a resource that already has a spec doc — including cases where partial, stubbed, or buggy code exists and must be replaced. Treats the spec doc as the contract and the existing implementation as disposable. Reads ONLY the spec doc and four contract-surface files (`<shortName>_interfaces.go`, `<shortName>_related.go`, `<shortName>_issue_enrichment.go`, `<shortName>_detail_enrichment.go`); never reads existing tests or fetchers. Dispatches `a9s-qa` and `a9s-coder` with scoped file lists. Cleans up stubs and "pretend to work" code tied to TBDs. Trigger this for any request that names a resource shortName and asks for implementation, tests, fixtures, or cleanup — even if the user doesn't explicitly mention the spec doc.
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
  - Bash(rm *)
  - Bash(git status *)
  - Bash(make *)
  - Bash(go test *)
  - AskUserQuestion
  - Agent(a9s-qa)
  - Agent(a9s-coder)
---

# a9s Resource Implementation Skill

Take a resource that already has a golden UX/UI spec at `docs/resources/<shortName>.md` and make the code match. Assume the current code is partial, stubbed, or bug-ridden. Assume the existing tests are disposable. The spec is the contract.

This skill must be invoked from the main Claude Code session, because it dispatches `a9s-qa` and `a9s-coder` subagents (Claude Code does not allow subagents to spawn other subagents). If the skill discovers at phase 6 that `Agent` dispatch is unavailable, stop and report:

```text
phase 6 blocked: Agent dispatch unavailable. Re-invoke the skill from the main Claude Code session.
```

Do not roleplay QA or coder inline — the split exists so QA tests are derived from the pseudocode spec, not from the working notes this skill accumulated in phases 0–5.

## Why it's structured this way

Three beliefs anchor everything below:

- **The doc is the should-be. The code is the is.** If they disagree, the code is wrong — unless the doc is stale, in which case we regenerate it first. Reading existing tests to "preserve coverage" bakes the old bugs into the new implementation. That's why this skill never reads old tests.
- **TBDs are visible product debt.** The spec makes missing decisions observable. They must be resolved with the user before any code lands, and any code that currently *pretends* to handle a TBD (silent fallback, stub value, hard-coded default) must be removed, not refactored.
- **Four files are enough to understand the contract surface.** The fetcher body, the existing tests, the mocks file, the fixtures — none of that needs reading. The interface file, the related-defs file, and the two enrichment files tell you what's plugged in and what the signatures look like. Everything else is implementation detail that the coder will rewrite anyway.

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

You may skim other files (message definitions, registry shape) to ground type signatures, but only the spec and the four surface files are load-bearing.

## Phases

Run in order. Phases 0–5 are analysis and planning done by the skill runner. Phases 6a/6b/7 dispatch agents. Phase 7.5 is the scope-diff gate. Phase 8 is the scenario-harness visual render gate. Phase 9 is the final four-point report checklist — implementation is not "done" until 9 emits PASS on every item.

### Phase 0 — Intake

**Spec check.** Confirm the spec doc exists. If not: "No spec at `docs/resources/<shortName>.md`. Run the `a9s-resource-spec` skill first." Stop.

**Contract-surface files.** List which of the four contract-surface files (`<shortName>_interfaces.go`, `<shortName>_related.go`, `<shortName>_issue_enrichment.go`, `<shortName>_detail_enrichment.go`) exist. Note which are missing; the coder will create them in phase 7.

**Wave 2 = None check.** If spec §3.2 says `No Wave 2 signals`, then the issue-enrichment file and the enricher test file are NOT part of the approved scope for this run. Note this in the impl-plan so phases 6b, 7, and 7.5 skip them.

**Fetcher file location.** The fetcher may live at `internal/aws/<shortName>.go` OR in a shared `internal/aws/<service>.go` when several resource types share an AWS service (e.g. `redis` lives in `elasticache.go`; `dbi` in `rds.go`; `ng` in `eks.go`). Locate it using the Grep tool (pattern `RegisterPaginated\("<shortName>"`, glob `internal/aws/*.go`, output `files_with_matches`) — do NOT invoke `grep` or `rg` as a Bash command per project shell rules.

Record the actual path. Phase 7 scope will reference THIS file, not a hypothetical `<shortName>.go`. If the fetcher lives in a shared service file, phase 7 extends (not creates) that file; phase 7.5's approved union includes `internal/aws/<service>.go` in place of `internal/aws/<shortName>.go`.

The renderer already wires all five UI surfaces (S1 in `mainmenu.go`, S2 via `typeDef.ResolveColor`, S3 in `table_render.go`, S4 via the fetcher's `Status` field, S5 via `EnrichmentFinding.Summary`). The universal rules below govern every per-resource run — they are the same for every resource type and asserted by phase 8's scenario test.

## Universal UI rules (apply to every resource)

These rules are invariant across all resource types. They are enforced by phase 5 (config audit) and phase 8 (scenario-harness visual render gate). Spec files do NOT restate them per-resource.

**Column layout**

- The list view carries ONE Status column (S4). Always one. Never split across two or more columns.
- No parallel "flags" / "policy" / "CIS" / "Issues" / jargon-code columns. All Wave 1 warnings ride in the Status column per spec §4 precedence.
- Column definitions live in `internal/config/defaults.go` → generated into `.a9s/views/<shortName>.yaml` via `go run ./cmd/viewsgen/`. That is the single authority; the spec does not restate columns.
- Identity / metadata columns (name, engine, version, region, etc.) are per-resource and declared in defaults.go.

**Visual rendering**

1. **Healthy rows render a blank Status cell.** Banned strings: `OK`, `ACTIVE`, `available`, `running`, `healthy`, `-`.
2. **Warning / Broken / Dim rows render the exact §4 "List text" phrase** from the resource's spec. No bare state keyword unless §4 explicitly approves it.
3. **`!` / `~` glyph prefix appears only on Healthy (green) rows with a Wave 2 finding.** Non-green rows render no glyph regardless of any finding — the color is the signal. This is the LIST view (S3) rule only. In the DETAIL view's Attention section (S5), every entry is prefixed with `!` or `~` regardless of row color — there glyph is the per-entry severity marker, not a row-level flag.
4. **Menu `issues:N` badge counts INSTANCES, not findings.** N = count of distinct resources that have at least one `!` severity finding attached. An instance with 3 `!` findings contributes 1. An instance with only `~` findings contributes 0. Therefore `N ≤ total instances in the list`. `~` never bumps the badge.
5. **Related panel counts are non-zero for every §2 pivot with `count shown: yes`** on the graph-root fixture. No exceptions, no "deferred pending sibling enrichment", no "follow-up ticket" excuses. A registered pivot that always returns `0` is a bug, not a deferred feature. If surfacing the pivot requires N+1 calls in the sibling fetcher (GetWorkGroup per workgroup, ListTargetsByRule per rule, ListResourceRecordSets per zone, etc.), do it — that cost is the cost of a working pivot, and it's the only version of the contract that renders a real operator graph. Pivots where §2 explicitly says `count shown: unknown` (e.g. windowed `ct-events`) are the only exemption, and even those must be documented in §2 with the reason.

   **"Implement resource" means implement the whole contract.** No deferrals, no stubs that return 0, no "Phase 7 will wire this" comments. If a pivot can't be wired end-to-end (fetcher populates the field, fixture tags an entry at the graph-root bucket, checker resolves non-zero), the skill is not done. User guidance 2026-04-23: "related resources MUST work. if they don't it's a bug. simple."
6. **Detail view (S5) renders findings through the unified `Attention (N)` section** — one section, at the top of the detail view, with a count in the header. There are NO per-type section names (`Pending Maintenance`, `Latest Build`, `Target Health`, etc. — those existed before 2026-04-22 and were collapsed into the single Attention section). Every Wave-1 phrase from `Resource.Issues` and the Wave-2 `EnrichmentFinding` render as entries inside it. The renderer (`injectAttentionSection` in `internal/tui/views/detail_fields.go`) handles this universally — no per-resource code required. Entry presentation: each primary entry is `<glyph> <phrase>` with the first letter capitalized for readability (data stays canonical lowercase; the capitalization is purely visual via `capitalizeFirst`). Rows render indented beneath the primary entry as `Label: Value` pairs.
7. **Multiple findings on the same instance remain individually visible across S2–S5.** S1 and S3 aggregate to one per instance (one count, one glyph — `!` beats `~`, color picks worst severity). But no finding may silently disappear. When an instance carries more than one finding:
   - **S4** renders the highest-precedence phrase plus a `(+N)` suffix when others exist on the same row — e.g. `storage-full (+2)`. The operator sees there is more to open for.
   - **S5** Attention section lists every finding — one entry per Wave-1 phrase from `Resource.Issues`, one entry for the Wave-2 `EnrichmentFinding` (with its `Rows` as context lines), sorted `!` first then `~`. No entry is collapsed or summarised away.
   **Precedence is severity-first, wave-second.** Order for picking the top finding on a row:
   1. **Severity bucket** — Broken > Warning > Dim > Healthy-with-finding. The worst severity wins S2 (color), S3 (glyph, Healthy-only), S4 (phrase).
   2. **Within the same severity** — Wave 2 beats Wave 1 (Wave 2 findings carry richer cause text). Within Wave 2, `!` beats `~`.
   3. **Within the same severity + wave** — the resource spec's §4 table pins a tie-breaker order (e.g. `no automated backups` > `publicly accessible` > …). If the spec doesn't, alphabetical by phrase is the safe default.

   This matters for resources like ECR: a repo with `scanOnPush=false` (Wave 1 Warning) AND `CRITICAL>0` (Wave 2 Broken) renders **red** with the Wave 2 Broken phrase in S4, because Broken severity beats Warning severity regardless of wave origin.

## Wave 2 enricher output contract

Every Wave 2 enricher emits `resource.EnrichmentFinding` objects whose shape must obey this contract. The contract is codified in `internal/resource/enrichment.go`; the skill restates it here because violating it produces visible duplication in the Attention section.

- **Summary is the short S5 phrase.** Lowercase, ≈1–4 words, matching a §4-style phrase (e.g. `"pending maintenance"`, `"unhealthy targets 2/5"`, `"latest build failed"`). This is what renders beside the glyph on the Attention primary entry row, and it is also what may be promoted verbatim into the S4 Status column via `FieldUpdates["status"]` (so it has to fit there).
- **Rows are the structured facts that support Summary.** Concrete values — the specific Action, Description, Earliest Target, failing-target names, failure timestamp — go in Rows as `Label: Value` pairs. Rows render beneath the primary entry as indented context.
- **Never embed Row content in Summary.** Every fact lives in exactly ONE place. If the enricher sets `Summary = fmt.Sprintf("pending maintenance: %s (%s)", action, description)` while also emitting `Action` and `Description` as Rows, that's the anti-pattern — the Attention section will render both and the duplication is visible to the user. The test `TestDBI_Enrich_MaintenancePending_HealthyRow` asserts Summary does not contain any Row value as a substring; copy that shape for every new enricher.
- **Summary is stable across instances of the same finding type.** It should not mutate based on the specific facts of one instance (that's what Rows are for). dbi's "pending maintenance" is the same phrase for every instance regardless of which action is pending; the Action row carries the per-instance detail.

Phase 6b QA must include a test per enricher that asserts both `Summary == <short-phrase>` AND `!strings.Contains(Summary, rowValue)` for every Row value the enricher emits. Phase 7 coder rejects tasks that build Summary from concatenation of Row fields.

## Universal coverage matrix (mandatory before phase 3)

Phase 3's "one case per signal" is NECESSARY but NOT SUFFICIENT. Rule 7 (`(+N)` suffix + stacking) is a cross-signal invariant and will be missed if each signal is tested in isolation. Before writing the phase 3 pseudocode, produce this coverage table in the impl-plan doc. Every row MUST point to at least one fixture ID and one test case. If the table can't be filled, phase 3 is blocked.

| ID | Invariant | Required fixture shape | Required test |
|----|-----------|-----------------------|---------------|
| U1 | Healthy blank S4 | 1 Healthy fixture | `ExpectRowStatusBlank` |
| U2 | Warning/Broken §4 phrase | ≥1 fixture per §3.1 signal | `ExpectRowStatusEquals` per signal |
| U3 | `~` glyph on Healthy+~ finding | 1 Healthy + Wave-2 ~ fixture | `ExpectRowNamePrefix("~ ")` |
| U4 | `!` glyph on Healthy+! finding | 1 Healthy + Wave-2 ! fixture (skip if spec has no `!`) | `ExpectRowNamePrefix("! ")` |
| U5 | No glyph on non-green rows | any Warning / Broken fixture | `ExpectRowNoGlyphPrefix` |
| U6 | S1 badge counts `!`-severity instances | all fixtures | `ExpectMenuIssueCount` |
| **U7a** | **Multi Wave-1: `<top> (+N-1)` suffix** | **1 fixture with ≥2 coexisting Wave-1 warnings** | **`ExpectRowStatusEquals(id, "<top> (+N)")`** |
| **U7b** | **Wave-1 + Wave-2 stack: bumps suffix** | **1 fixture with Wave-1 Warning AND a Wave-2 finding** | **`ExpectRowStatusEquals(id, "<w1phrase> (+1)")`** |
| **U7c** | **S5 lists every Wave-2 finding in full** | **same fixture as U7b** | **`ExpectViewContains(<w2 cause>)` on detail** |
| U7d | `!` beats `~` in multi-finding precedence | 1 fixture with both ! and ~ (if spec has both) | `ExpectRowNamePrefix("! ")` on Healthy stack |
| **U7e** | **S5 lists every Wave-1 phrase as well — `Resource.Issues` surfaces in detail, not just in the Status column** | **multi-W1 fixture (`warn-<short>-multi`)** | **`ExpectViewContains(<phrase>)` on detail for each entry in `Resource.Issues` (with first letter capitalized — Attention applies `capitalizeFirst` at render time); the bare `<top phrase>` must appear without its `(+N)` suffix because the detail enumerates the entries, not the rolled-up Status string** |
| **U7f** | **`Resource.Issues` is populated in §4 precedence order by the fetcher — one entry per active signal** | **every `warn-*` and `broken-*` fixture** | **unit test: `got.Issues` deep-equals the expected ordered slice** |
| U8 | Broken severity > Warning > `~` | fixture with Wave-1 Warning + Wave-2 Broken (if spec has it) | phrase precedence test |
| U9 | Related pivot counts (`count shown: yes`) | 1 graph-root fixture | `ExpectRelatedRowCountAtLeast` per pivot |
| U10 | No jargon columns | all fixtures | `ExpectViewNotContains("CIS", "Flags", …)` |
| **U11** | **Summary ≠ Rows content (EnrichmentFinding contract)** | **every fixture with a Wave-2 finding that has Rows** | **unit test: `finding.Summary == <short-phrase>` AND `!strings.Contains(finding.Summary, row.Value)` for every Row** |

Demo mode runs Wave 2 enrichment end-to-end against typed fakes (the `!m.isDemo` guard was removed 2026-04-22 after it was caught hiding the `(+N)` / `~` / "maintenance scheduled" rendering in the actual demo). Every row in this table is therefore reachable via the scripted scenario harness without message injection, provided the harness drains the `AvailabilityPrefetchedMsg` → `EnrichmentCheckedMsg` chain (it does as of 2026-04-22).

### Phase 1 — Read the spec doc

Load `docs/resources/<shortName>.md` end to end. Extract:

- Identity: list API, describe API (if any).
- Related-panel targets with discovery mechanism and count-shown policy.
- Wave 1 signals with state bucket, field, severity implications.
- Wave 2 signals with API call, cost shape, severity.
- Wave 3 signals (explicitly out of scope — record but do not implement).
- Issue-visualization table: per-signal S4 list text, S5 detail text, surfaces reached.
- Out-of-Scope list.
- §6 citations — note any `a9s-devops` decisions and `user decision` entries so you know which calls were already made.

Do **not** read existing code. Do **not** read existing tests.

### Phase 2 — Resolve TBDs with the user

Scan the loaded spec for remaining `TBD` markers and for signals explicitly deferred ("§5 Out of Scope per devops" etc).

For each, ask the user **one question at a time** via `AskUserQuestion` with three clearly labelled options:

- **resolve** — user provides the answer; update the spec doc inline and add a §6 citation `user decision (<date>): <answer>`.
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
- **Mandatory multi-finding cases (rule 7)** — one test per coverage-matrix row U7a/U7b/U7c. These never emerge naturally from enumerating §3 signals; the skill runner inserts them explicitly:

```text
TEST: multi_w1_<top>_plus_<hidden>_suffix   (covers U7a)
GIVEN: a fixture with §3.1 warning <top> AND §3.1 warning <hidden> both present
WHEN:  the list is fetched
THEN:  Resource.Status == "<top> (+1)"
       — suffix increments by 1 per hidden warning
       — the top phrase is the §4 precedence winner, not the later one

TEST: w1_plus_w2_bumps_suffix   (covers U7b)
GIVEN: a fixture with §3.1 warning <w1> AND a Wave-2 finding
WHEN:  the enricher runs over a resource slice whose Status is already "<w1>"
THEN:  FieldUpdates[id]["status"] == "<w1> (+1)"
       — NOT "<w2cause>" (Wave-1 wins severity precedence)
       — NOT "<w1> (+1) (+1)" (double-suffix must not accumulate across runs)

TEST: w1_multi_plus_w2_bumps_existing_suffix
GIVEN: a fixture with N≥2 Wave-1 warnings (Status = "<top> (+N-1)") AND a Wave-2 finding
WHEN:  the enricher runs
THEN:  FieldUpdates[id]["status"] == "<top> (+N)"
       — the existing suffix increments; the phrase does not change

TEST: detail_view_shows_all_findings   (covers U7c)
GIVEN: the same fixture as w1_plus_w2_bumps_suffix
WHEN:  OpenDetailResource on it
THEN:  rendered detail contains the Wave-2 finding's Action/Description rows
       — "no finding silently disappears" per rule 7

TEST: detail_view_surfaces_every_wave1_phrase   (covers U7e)
GIVEN: the multi-W1 fixture (`warn-<short>-multi`) with N≥2 coexisting §3.1 warnings
WHEN:  OpenDetailResource on it
THEN:  rendered detail contains EACH entry of `Resource.Issues` verbatim
       — the list Status shows "<top> (+N-1)", the detail enumerates
         every phrase so the operator never has to infer hidden warnings.

TEST: fetcher_populates_resource_issues   (covers U7f)
GIVEN: each §3 fixture (Healthy, single warning, multi warning, transitional, broken)
WHEN:  FetchXxxPage runs on the fixture
THEN:  got.Issues deep-equals the expected ordered slice
       — Healthy → nil / empty
       — N warnings → N phrases in §4 precedence order
       — broken / transitional → single-entry slice matching the §4 phrase
```

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

**Mandatory multi-finding fixtures (rule 7):** the per-signal list above covers §3 but not rule 7. Append these to §2 before dispatching phase 6a:

- **Multi-W1 fixture** — an instance with ≥2 coexisting §3.1 warnings (e.g. `warn-<short>-multi`). Concrete values for every field the §3.1 signals read. Expected `Resource.Status` = `"<top §4 phrase> (+N-1)"` where N is the warning count.
- **W1 + W2 stacking fixture** — an instance with one §3.1 Warning AND a Wave-2 finding (e.g. `warn-<short>-<w1>-plus-<w2>`). Both the matching §3.2 API response entry AND the fetcher-side fields that trigger the §3.1 Warning. Expected `FieldUpdates[id]["status"]` after enrichment = `"<w1 phrase> (+1)"`.
- **Wave-3-exclusive fixture** (when applicable) — when the resource has both `!` and `~` Wave-2 signals, add one fixture with both. Verifies U7d precedence (`!` beats `~`).

Skip any fixture whose preconditions are unreachable for this resource (e.g. U7d is skipped when the resource has only `~` Wave-2 signals). Record the skip in the impl-plan with a one-line justification — do NOT silently omit.

### Phase 5 — Contract surface read + view-config audit

Read the four files listed above. From each, extract only:

- **`<shortName>_interfaces.go`**: the aggregate service interface name, every narrow `*API` interface already declared, every AWS SDK method signature wired in. The coder needs to extend this; the QA agent needs these signatures to write mocks.
- **`<shortName>_related.go` (+ `_extra.go`)**: existing `RegisterRelated` calls — which targets are already wired, which fields they read. Compare to §2 of the spec. Mark deltas.
- **`<shortName>_issue_enrichment.go`**: the registered enricher function signature, the AWS API it calls, whether it's registered at package init. Compare to §3.2 of the spec.
- **`<shortName>_detail_enrichment.go`** (if present): same shape as issue enrichment but for detail-view fields. If absent, note that.

Also read **`.a9s/views/<shortName>.yaml`** and **`internal/config/defaults.go`** (the `defaultViews` section for this shortName) and audit each declared column against the universal column rules (see "Universal UI rules" in phase 0):

- **Exactly one Status column (S4)** — backing key `status`, carrying phrases derived per spec §4. If absent, coder must add. If present twice or split, coder must merge.
- **No jargon columns** — any column whose name or backing key looks like an encoded flag set (`CIS`, `Flags`, `Policy`, `Issues`, `NOBKP`, `UNENC`, etc.) is invented UI. It goes on the coder's delete list. Its data belongs in the Status column per §4 precedence.
- **Identity / metadata columns** — name, engine, version, region, and similar pure-data columns are allowed and per-resource. No authorization list: if the column is a plain identifier the operator would want at 3am glance, it stays.
- **Unsourced columns** — any column whose value does not trace to an AWS SDK field or a spec §4 derivation is invented. Delete.

Record any column delta in the impl-plan's "Contract surface gap analysis" section alongside the related/enricher deltas. If the only finding is "Status column is correct, no jargon columns, identity columns match AWS fields" — that is the normal case, record it and move on. The same section lists: what the spec §2/§3/§4 demand, what the four surface files currently provide, and the delta the coder must close.

### Phase 6 — Fixtures-first, then QA + coder in parallel

Three sub-steps: **6a** (fixtures), then **6b** (QA tests) and **7** (coder implementation) in parallel.

Rationale for the sequencing:
- **Fixtures are a single asset**, not two. `internal/demo/fixtures/<shortName>.go` feeds BOTH `./a9s --demo` (showcase) AND the unit test suite (6 test files in the tree currently import from here, and counting). Tests import raw SDK-shape fixtures from this file; inline construction in tests is the anti-pattern we are retiring.
- **Exception: adversarial fixtures** (nil pointers, malformed AWS responses, API error paths, anything the spec marks out of scope) corrupt the demo and stay inline in the QA test file. The `a9s-create-demo-fixture` skill enforces this boundary.
- **6a blocks 6b**: tests reference fixture symbols; without the file on disk QA's tests don't compile (QA cannot write under `internal/`).
- **6a blocks 7**: if the coder rewrote the fixture file after 6b wrote tests against it, every test would break. Fixtures are written once in 6a and never rewritten inside this skill invocation.
- **6b and 7 do NOT block each other**: QA writes only `tests/unit/*`; coder in phase 7 writes only `internal/aws/<shortName>*.go`. No file overlap, no runtime dependency.

Before dispatching 6a, delete the exact stale test files that 6b will rewrite:

```bash
rm -f tests/unit/aws_<shortName>_test.go \
      tests/unit/aws_<shortName>_related_test.go \
      tests/unit/aws_<shortName>_issue_enrichment_test.go \
      tests/unit/aws_<shortName>_detail_enrichment_test.go
```

Do NOT use a trailing-glob (`aws_<shortName>*.go`) — some resources have child-view tests (`aws_<shortName>_events_test.go`, etc.) or unrelated neighbours that would match and vanish. Stale legacy versions of the above four files produce duplicate `Test*` declarations once 6b lands; that's a compile error, not a warning.

**Precondition:** verify `Agent(a9s-qa)` and `Agent(a9s-coder)` are callable. If not:

```text
phase 6 blocked: Agent dispatch unavailable. Re-invoke the skill from the main Claude Code session.
```

#### 6a. Coder — fixtures only (blocks 6b and 7)

Dispatch `Agent(a9s-coder)` with a narrow, fixture-only task. The coder uses the `a9s-create-demo-fixture` skill to build a graph-connected fixture file at `internal/demo/fixtures/<shortName>.go` (single file per service — no `_fixtures` suffix; fold any existing `<shortName>_fixtures.go`).

```text
## CODER TASK: <shortName> demo fixtures (phase 6a)
Parallelization: sequential (blocks 6b QA and phase 7 coder implementation)

### Invoke this skill:
Skill: a9s-create-demo-fixture with argument <shortName>. Follow the skill end-to-end.

### Files to create or overwrite:
- internal/demo/fixtures/<shortName>.go — single source for demo + tests, raw SDK types
- internal/demo/fixtures/<peer>.go — targeted sibling updates per the skill's phase 2 graph plan (alarm, kms, sg, subnet, vpc, rds-snap, role, secrets, ct-events, logs, and any other §2 pivot target that needs a matching entry for this fixture's references)

### Files to delete:
- internal/demo/fixtures/<shortName>_fixtures.go (if present — folded into <shortName>.go)

### Expected exports (QA will import these by exact name):
- `<ShortNameCamel>Fixtures` struct + `New<ShortNameCamel>Fixtures()` constructor. The camel-cased SHORTNAME, not the AWS service name — `redis` gets `RedisFixtures`, not `ElastiCacheFixtures`; `dbi` gets `DBIFixtures`, not `RDSFixtures`; `ng` gets `NGFixtures`, not `EKSFixtures`.
- Exported ID/ARN `const` (e.g. `ProdDbiID`, `ProdDbiARN`, `ProdRedisID`) for sibling-file cross-reference.
- One slice element per non-adversarial fixture in docs/resources/<shortName>-impl-plan.md §2.

### Forbidden inputs:
- Do not read tests/**.
- Do not write under tests/.
- Do not include adversarial fixtures (nil pointers, malformed responses, error paths) — those stay inline in the test files.

### Context files (read-only):
- docs/resources/<shortName>.md (§2 related targets)
- docs/resources/<shortName>-impl-plan.md (§2 fixture list — authoritative)
- internal/aws/<shortName>_related.go (which fields each pivot reads)
- internal/demo/fixtures/<peer>.go for every §2 target
- internal/demo/handlers.go (confirm typed-fake path suffices)

### Verify before reporting complete:
- `make build` succeeds.
- `rg -n '^func New|^const ' internal/demo/fixtures/<shortName>.go` lists every exported symbol.
- The skill's phase 6 "graph renders" checks pass (row count, pivot non-zero counts — reported as a single block).
```

Record the exact exported symbol list the coder emits — 6b needs it.

#### 6b. QA — test files (parallel with phase 7, after 6a)

Dispatch `Agent(a9s-qa)` with the scored handshake (`Mode: score` → accept/rework → `Mode: execute` with `Confirmed score: <N>`).

QA task shape:

```text
## QA TASK: Tests for <shortName> from spec
Mode: score
Parallelization: parallel-safe with phase 7 (both run after 6a)

### Test files to create (or overwrite):
- tests/unit/aws_<shortName>_test.go — fetcher tests per §3.1 Wave 1 signals
- tests/unit/aws_<shortName>_issue_enrichment_test.go — Wave 2 enricher tests per §3.2
- tests/unit/aws_<shortName>_detail_enrichment_test.go — detail enricher tests (if §2 demands one)
- tests/unit/aws_<shortName>_related_test.go — related-target discovery tests per §2

### Fixture usage rule:
Tests import from internal/demo/fixtures/<shortName>.go (single source of truth).
The phase 6a output exports these exact symbols — call them by name:
<paste the symbol list from 6a>

Only adversarial cases (nil pointers, malformed responses, error paths) may be constructed inline
in the test file — the demo fixture never carries these because they corrupt the showroom.

### What to test:
- Every signal row in spec §4 becomes one test case.
- Every related target in spec §2 becomes one discovery test.
- Silence test: Healthy fixture → row green, S4 blank, no finding.
- Out-of-Scope anti-tests for every §3.3 Wave 3 entry.

### Forbidden inputs:
- Do not read existing tests/unit/*<shortName>*.go — rewrite from the pseudocode spec.
- Do not read internal/aws/<shortName>.go — you are testing the contract.
- Do not write under internal/.

### Context files (read-only):
- docs/resources/<shortName>.md
- docs/resources/<shortName>-impl-plan.md
- internal/demo/fixtures/<shortName>.go (symbols and state coverage)
- internal/aws/<shortName>_interfaces.go (mock signatures)
- internal/resource/resource.go, internal/resource/enrichment.go
```

QA replies `SCORE: <N> — <rationale>`. Accept or rework. On accept, re-dispatch same scope with `Mode: execute` and `Confirmed score: <N>`.

### Phase 7 — Coder handoff (full implementation, parallel with 6b)

Runs after 6a, in parallel with 6b.

The fixture file written in 6a is NOT in this file list and MUST NOT be rewritten — QA's tests reference its symbols.

Coder task shape:

```text
## CODER TASK: Implement <shortName> against the spec (phase 7 — non-fixture implementation)
Parallelization: parallel-safe with 6b QA (both run after 6a)

### Files to create or overwrite (closed set — adding any file not in this list is a scope violation caught in phase 7.5):
- **Fetcher** — either `internal/aws/<shortName>.go` OR `internal/aws/<service>.go` if the fetcher lives in a shared service file (phase 0 located it). Exactly one of the two.
- internal/aws/<shortName>_interfaces.go — add any missing narrow interface
- internal/aws/<shortName>_related.go — RegisterRelated for every target in §2
- internal/aws/<shortName>_issue_enrichment.go — Wave 2 enricher per §3.2. ONLY include this file when §3.2 has signals; if the spec says `No Wave 2 signals`, omit this file entirely. All enricher body lives here — do NOT spin off `<shortName>_maintenance.go` / `<shortName>_overdue.go` / similar helper files.
- internal/aws/<shortName>_detail_enrichment.go — only if §2 of the spec requires detail fields beyond the list shape
- .a9s/views/<shortName>.yaml — regenerate via `go run ./cmd/viewsgen/` AFTER amending `internal/config/defaults.go` (the yaml is generated, never hand-edited)
- internal/config/defaults.go — update the `defaultViews` entry for this shortName per the universal column rules in phase 0 (exactly one Status column, no jargon columns, identity/metadata per-resource)

### Do NOT touch:
- internal/demo/fixtures/<shortName>.go — written in 6a; rewriting breaks QA's test compile.
- Any file not in the list above. If you think you need a new helper file, reply REJECT with scope expansion request to the skill runner.

### Expected behavior:
- Fetcher maps AWS fields to resource.Resource per §2 Identity section.
- Status/S4 column carries the exact text in the §4 "List text" column, never a bare state keyword.
- Row color follows state bucket from §3.1.
- Wave 2 enricher populates resource.EnrichmentFinding with the exact Summary from §4 "Detail text" column.
- No invented UI. No row `·` dot. No `⚠ Background Check` header. No derived banner. See spec §"Allowed visualization surfaces".

### Rule-7 implementation (multi-finding `(+N)` suffix) — MANDATORY:

Rule 7 is NOT wired by any universal infrastructure; each resource emits the `(+N)` suffix itself through its fetcher + enricher pair. Missing this is the most common implementation bug — verified against dbi on 2026-04-22.

1. **Fetcher — multi-W1 suffix**: when the resource has multiple §3.1 warnings that can coexist (e.g. `available` + `BackupRetentionPeriod=0` + `PubliclyAccessible=true`), collect ALL applicable warnings in §4 precedence order, then emit:
   - 0 warnings → `""` (Healthy silence).
   - 1 warning → `"<phrase>"` (single, no suffix).
   - N≥2 → `"<top phrase> (+N-1)"`.
   The top phrase is the first-in-precedence (per §4), the suffix is `(+<count of hidden warnings>)`. Never collapse silently to just the top phrase when >1 warning exists.

2. **Enricher — W1+W2 suffix bumping**: when a Wave-2 finding lands on a row whose fetcher-produced `Resource.Status` is non-empty, the enricher MUST bump the `(+N)` suffix in `FieldUpdates[id]["status"]` — NOT overwrite with the Wave-2 phrase (Wave-1 wins severity precedence), NOT leave Status alone (the operator loses the finding-count signal). Use the shared package helper — do NOT reinvent:
   ```go
   import "github.com/k2m30/a9s/v3/internal/resource"
   // "publicly accessible"        → "publicly accessible (+1)"
   // "publicly accessible (+1)"   → "publicly accessible (+2)"
   newStatus := resource.BumpFindingSuffix(existing)
   ```
   On Healthy rows (Status == ""), set the spec's Wave-2 short cause verbatim (e.g. `"maintenance scheduled"`) — no suffix, single finding.

3. **Color function (`internal/resource/types_<service>.go`) — MUST strip `(+N)` before matching**: use the shared `resource.StripFindingSuffix` (defined in `internal/resource/finding_suffix.go`) — do NOT reinvent. Every per-type Color func that pattern-matches on `Resource.Fields["status"]` MUST strip the suffix first, OR match phrase prefix. Failing this, `"publicly accessible (+1)"` falls through all Warning switches and color-buckets Healthy — a spec-§4 violation.

4. **Wave-2 short-cause phrase must NOT be in the Warning color switch**: a Wave-2 `~` finding renders on a HEALTHY (green) row. If the Color func lists `"maintenance scheduled"` among Warning phrases, the row turns yellow and the glyph is suppressed — spec rule 3 violation. The enricher sets the Status phrase; the Color func must treat it as Healthy (the row color is driven by the Wave-1 bucket, not by the Wave-2 phrase).

5. **Fetcher MUST populate `Resource.Issues`** — an ordered slice of every active WAVE-1 §4 phrase for this row (in precedence order). The common pattern is to split `computeXxxStatus` into `computeXxxStatusAndIssues(raw) (topPhrase string, allIssues []string)`, then:
   - Healthy → empty slice.
   - Single warning → one entry matching the top phrase.
   - N≥2 warnings → N entries; the first is the top, the rest are the ones hidden behind the `(+N-1)` suffix.
   - Broken / transitional → a single entry with the §4 phrase.

   **Issues is Wave-1 ONLY.** Wave-2 cause lives in `EnrichmentFinding.Summary` + `Rows`. The Attention renderer merges both at display time. An enricher that appends to `Resource.Issues` produces double entries — that's the anti-pattern.

   Dropping this populates `Resource.Issues == nil`; the detail view silently hides every hidden warning; universal rule U7e fails. This is the bug that leaked through on 2026-04-22 — spec rule 7 violation revealed when the user opened a row that listed `(+3)` on the list but showed no individual issues in detail. The integration test `TestScenario_DBIVisual_DetailSurfacesAllIssues` is the regression pin.

6. **Detail view universally renders via `injectAttentionSection`** (in `internal/tui/views/detail_fields.go` — no per-resource work needed). The Attention section appears at the top of the detail view whenever `len(Resource.Issues) > 0 OR EnrichmentFinding != nil`. A future resource that forgets to populate `Resource.Issues` will see its detail view silently hide every Wave-1 phrase — the U7e/U7f QA unit tests are the only upstream check. **Do NOT add a per-type `injectXxxSection` function**; that pattern was collapsed on 2026-04-22 into the single unified section and re-introducing it is a regression.

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

### Phase 7.5 — Scope-diff gate (runs immediately after coder claims done, BEFORE phase 8)

The coder agent cannot be fully trusted to stay inside the approved file list — past runs have silently added helper files (`<shortName>_maintenance.go`, `<shortName>_helpers.go`, etc.) that were never in scope. Gate on the diff, not the claim.

Run:

```bash
git status --porcelain -- internal/aws/<shortName>*.go internal/demo/fixtures/<shortName>*.go .a9s/views/<shortName>.yaml internal/config/defaults.go tests/unit/aws_<shortName>*.go
```

Parse the output. Build the set of files touched. Diff against the approved-scope union:

- Phase 6a approved: `internal/demo/fixtures/<shortName>.go` + any sibling files listed in the 6a graph plan.
- Phase 6b approved: `tests/unit/aws_<shortName>_test.go`, `tests/unit/aws_<shortName>_related_test.go`, `tests/unit/aws_<shortName>_issue_enrichment_test.go` (ONLY if §3.2 has Wave 2 signals), `tests/unit/aws_<shortName>_detail_enrichment_test.go` (ONLY if §2 demands a detail enricher).
- Phase 7 approved: the fetcher (either `internal/aws/<shortName>.go` OR the shared `internal/aws/<service>.go` located in phase 0 — not both), `internal/aws/<shortName>_interfaces.go`, `internal/aws/<shortName>_related.go`, `internal/aws/<shortName>_issue_enrichment.go` (ONLY if §3.2 has Wave 2 signals), `internal/aws/<shortName>_detail_enrichment.go` (ONLY if §2 demands a detail enricher), `.a9s/views/<shortName>.yaml`, `internal/config/defaults.go`.

Any file in the `git status` output that is NOT in this union = SCOPE VIOLATION. Do not run phase 8. Return the list to the coder agent with:

```text
Phase 7.5 REJECTED. Out-of-scope files detected:
- <path>
- <path>
Either remove these (and fold their content into an approved file) or reply with a scope-expansion request explaining why a new file is necessary. If justified, update docs/resources/<shortName>-impl-plan.md §3 deltas and the skill runner will re-dispatch.
```

Coder must clean up and report back. Only proceed to phase 8 after the diff is clean.

### Phase 8 — Scenario-harness visual render gate

Unit tests assert on `Resource.Status` and `EnrichmentFinding`. They do NOT verify that the rendered list view matches the spec — a fetcher can return the right `Status` while the view misreads it, or a `.a9s/views/<shortName>.yaml` can declare a jargon column. Phase 8 closes that gap by asserting on the actual rendered output via the scripted scenario harness (`tests/integration/SCENARIO_HARNESS.md`).

#### 8.1 Scenario-harness test file

The skill runner writes `tests/integration/scenario_<shortName>_visual_test.go`. (This is one of the few test files the skill runner authors directly — it is not within QA's scope because it is a render-gate artifact, not a behavioral unit test.)

The test drives the real `tui.Model.Update()` loop via `fullIntegrationNewDemoScenario(t)`, navigates to the resource list and each fixture's detail view, and asserts the **universal UI rules** (see phase 0) with resource-specific fixture IDs and §4 phrases plugged in:

1. **No jargon column**: assert the rendered frame contains no known-jargon column title (`CIS`, `Flags`, `Policy`, `Issues`, `NOBKP`, `UNENC`, `PUB`, `NOPROT`) via `scenario.ExpectViewNotContains(...)` for each.
2. **Healthy rows blank S4**: for each fixture whose spec §3 state bucket = Healthy, `scenario.ExpectRowStatusBlank(<fixture ID>)`. Banned render strings (defined in the harness helper): `OK`, `ACTIVE`, `available`, `running`, `healthy`, `-`.
3. **Warning/Broken rows show §4 phrase**: for each fixture whose bucket ≠ Healthy, `scenario.ExpectRowStatusEquals(<fixture ID>, <exact §4 "List text" phrase>)`.
4. **Glyph presence/absence**: for each fixture whose Wave 2 finding severity is `~` on a Healthy row, `scenario.ExpectRowNamePrefix(<fixture ID>, "~ ")`. For `!`, `"! "`. For any non-Healthy row, `scenario.ExpectRowNoGlyphPrefix(<fixture ID>)` regardless of finding presence.
5. **S1 menu count**: `scenario.ExpectMenuIssueCount(<shortName>, <expected N>)` where N = count of distinct fixtures that have at least one `!` severity finding (NOT total finding count — a fixture with 3 `!` findings counts as 1). Must satisfy `N ≤ total fixture count for this type`. When the spec §3.2 has no Wave 2 `!` signals at all, N = 0 and the helper treats that as "badge absent" (no `issues:` string in the menu entry for this type).
6. **Related pivot counts**: for each fixture, `scenario.OpenDetailResource(<shortName>, <fixture ID>)` then for each pivot in spec §2 whose "count shown" is `yes`, `scenario.ExpectRelatedRowCountAtLeast(<pivot display name>, 1)`. Pivots where §2 says `count shown: unknown` are skipped.
7. **Multi-W1 suffix (U7a)**: for the `warn-<short>-multi` fixture, `scenario.ExpectRowStatusEquals(<id>, "<top> (+N-1)")`.
8. **W1+W2 suffix (U7b)**: for the `warn-<short>-<w1>-plus-<w2>` fixture, `scenario.ExpectRowStatusEquals(<id>, "<w1> (+1)")`.
9. **Healthy + ~ glyph (U3)**: `scenario.ExpectRowNamePrefix(<healthy+w2 id>, "~ ")`.
10. **Non-green rows no glyph (rule 3)**: `scenario.ExpectRowNoGlyphPrefix(<w1+w2 id>)` — the row has a Wave-2 finding but is Warning-colored; the color is the signal, no glyph.
11. **S5 Wave-2 finding-row visibility (U7c)**: `scenario.OpenDetailResource(<shortName>, <w1+w2 fixture>)`; then `scenario.ExpectViewContains(<w2 finding Action or Description string>)`. Verifies no Wave-2 finding silently disappears when Status shows the Wave-1 phrase.
12. **S5 every-Wave-1-phrase visibility (U7e)**: on the multi-W1 fixture, open detail and assert every entry of `Resource.Issues` appears — BUT with first letter capitalized, because `injectAttentionSection` applies `capitalizeFirst` to every entry at render time. Data (`Resource.Issues`) stays canonical lowercase (`"publicly accessible"`); the rendered frame has `"Publicly accessible"`. Use a helper or pin the expected strings explicitly:
    ```go
    multi := selectXxxByID(t, scenario, <multiW1 id>)
    scenario.OpenDetailResource("<short>", multi)
    for _, phrase := range multi.Issues {
        // Attention section capitalizes first rune for presentation.
        rendered := strings.ToUpper(phrase[:1]) + phrase[1:]
        scenario.ExpectViewContains(rendered)
    }
    ```
    The `(+N)` suffix itself is NOT expected in the detail — the detail enumerates phrases, one per row. This is the U7e regression pin that caught the 2026-04-22 bug.

**Wave-2 in demo mode is native** — no injection required. The phase-8 scenario test drives `fullIntegrationNewDemoScenario`, calls `OpenList`, and asserts every rule above directly. The harness's `shouldDrainFollowups` includes `AvailabilityPrefetchedMsg` / `AvailabilityCheckedMsg` / `EnrichmentCheckedMsg` so the enrichment chain cascades end-to-end.

If a new resource's enricher does not produce expected findings in demo:
1. Verify the resource's typed fake implements every AWS API the enricher calls (e.g. `DescribePendingMaintenanceActions`, `GetStages`, `GetPolicyVersion`). Missing fake methods cause the enricher to error and findings never land.
2. Verify the fake's response matches what the enricher expects — empty slices vs error vs missing field all cascade differently.
3. Verify `handleAvailabilityPrefetched` seeds `resourceCache` from `msg.Resources` so `FieldUpdates` merged by `handleEnrichmentChecked` survive the first `OpenList`.

The scenario test no longer needs a Part-A-then-Part-B structure — all assertions run after `OpenList` returns, with the enrichment having already merged.

The test file runs inside the existing integration test target:

```bash
go test -tags integration ./tests/integration -run TestScenario_<ShortName>Visual -count=1 -v
```

If `scenario.ExpectRowStatusBlank` / `ExpectRowNamePrefix` / `ExpectMenuIssueCount` / `ExpectRelatedRowCountAtLeast` are missing from the scenario harness, stop phase 8 and file a global harness-extension task against `tests/integration/scripted_scenario_helpers_test.go` — do NOT roll one's own render assertions per-resource.

#### 8.2 Supporting checks (must pass along with 8.1)

- `make test` — no red in the full unit test suite.
- `make lint` — no issues.
- `rg -n 'TODO|FIXME|stub|fake' internal/aws/<shortName>*.go` — nothing except legitimate comments.

#### 8.3 Report

```text
<shortName>: render-gate PASS.
- columns: <N> declared in defaults.go, <N> in rendered view (match: yes); jargon columns: 0
- healthy-blank-S4: <N> fixtures checked, <N> violations
- warning/broken phrases: <N> fixtures checked, <N> violations
- glyphs: ~<N> / !<N> prefixes verified; <N> non-green rows glyph-free
- S1 menu badge: expected issues:<N>, got issues:<N>
- related pivots non-zero: <M>/<total> (skipped: <N> unknown-count)
- rule-7 U7a (multi-W1 +N suffix): <fixture-id> → "<expected>" OK
- rule-7 U7b (W1+W2 suffix bump): <fixture-id> → "<expected>" OK
- rule-7 U7c (S5 all findings): <fixture-id> detail contains <w2 cause> OK
- rule-7 U7d (! beats ~):        <skipped because no ! signals in spec | OK>
- rule-7 U7e (S5 every Wave-1 phrase): <fixture> → all <N> entries verbatim in detail OK
- rule-7 U7f (Resource.Issues population): <N> fixtures, <N> deep-equals OK
- Wave-2 native in demo:         yes (enrichment chain drains end-to-end)
- unit tests: <N> passing, 0 failing
- stubs: 0 / TBDs resolved: <N> / deferred: <N> / out-of-scope: <N>
Implementation approved — ready for review at internal/aws/<shortName>*.go and tests/integration/scenario_<shortName>_visual_test.go.
```

Rule-7 lines are **required** — "N/A" is only valid when the spec has at most one §3.1 signal AND no Wave-2 signals (extremely rare). Omitting them = render-gate FAIL.

If any rule fails, report the exact failure and the fixture that triggered it. Do NOT summarize as "mostly passing" — a single failed render-gate rule is a blocking defect.

#### 8.4 User-observable visual sanity — MANDATORY before claiming done

Run the scenario test with `-v` and paste the rendered output of ONE multi-warning detail view in the final report. The scenario harness captures the rendered frame via `scenario.currentView()`; emit it to the test log before the assertions so the reader can see exactly what an `./a9s --demo` user would see on that row:

```go
t.Log("\n" + scenario.currentView())  // just before the ExpectViewContains assertions
```

This is the regression pin for the 2026-04-22 class of bugs where unit tests green, render-gate green, and the actual user screenshot shows that phrases are silently missing. Ship the rendered detail alongside the PASS report. If the rendered frame lacks any phrase asserted above, the gate fails regardless of what the `Expect*` calls returned.

### Phase 9 — Final report checklist (runs AFTER phase 8 passes)

This phase does not change any code. It is the closeout that proves the four user-facing invariants that have been paid for in blood across multiple shipped resources. Skip any of these four and the skill is NOT done — regardless of how green phases 6–8 look.

Emit the checklist verbatim in the final report, with each item marked `PASS`, `FAIL`, or `N/A (<reason>)`. If any item is FAIL, keep the skill running until it is PASS. "N/A" is only valid when the resource spec genuinely has no instance of the thing being checked (e.g. no Wave-1 signals at all → no multi-issue fixture possible), and the reason must be cited.

#### 9.1 No illegal UI elements — only S1–S5

Walk the scenario-harness rendered frame from 8.4 and audit for anything outside the five approved surfaces:

- S1 — main-menu `issues:N` badge.
- S2 — row color (green / yellow / red / dim).
- S3 — `!` / `~` glyph prefix on Healthy rows only.
- S4 — Status column phrase (spec §4 text + optional `(+N)` suffix).
- S5 — unified `Attention (N)` section in the detail view.

Banned (any appearance = FAIL):
- Banners, toasts, floating overlays summarizing findings.
- `?` glyph (ambiguity glyph — never approved).
- Parallel "flags", "policy", "CIS", "issues" columns.
- `+` on the S1 badge (`issues:N+`). The `+` is reserved for operational count truncation on pagination, NOT on the finding count.
- Any string where the Status cell of a Healthy row is non-blank (`OK`, `-`, `available`, etc.).

Report format:

```text
9.1 illegal UI elements: PASS — S1-S5 only, no banners/?-glyph/jargon columns/S4-on-healthy
```

#### 9.2 Fixture coverage — every signal + one multi-issue instance

- Every §3.1 Wave-1 signal has at least one dedicated fixture. Healthy baseline is a separate fixture.
- Every §3.2 Wave-2 signal has at least one dedicated fixture.
- **At least one fixture carries ≥2 active findings simultaneously** (Wave-1 multi, or Wave-1 + Wave-2). This is the rule-7 (+N) / stacking test vehicle. Without it, the multi-issue code paths never run against realistic demo data.

Report format:

```text
9.2 fixture coverage:
    Wave-1 signals: <N> required, <N> fixtures present — PASS
    Wave-2 signals: <N> required, <N> fixtures present — PASS (or N/A — no Wave-2)
    multi-issue fixture: <fixture-id> carries <finding-1>, <finding-2>, ... — PASS
```

#### 9.3 "All related resources non-zero" graph-root

At least ONE fixture must resolve every registered `count shown: yes` pivot to ≥ 1. This is the showroom instance — an operator opens its detail view and sees every pivot populated, proving each registered panel entry actually works end-to-end. Document which fixture and list every pivot count.

If the resource type has engine-split registrations (e.g. dbc covers both DocDB and Aurora, where some pivots are engine-specific), identify the graph-root that covers the union — add fixture entries as needed so one fixture carries the full set.

Report format:

```text
9.3 graph-root all-pivots-non-zero: <fixture-id>
    - <Pivot Display Name 1>: <count>
    - <Pivot Display Name 2>: <count>
    - ...
    <N>/<total> registered `count shown: yes` pivots ≥ 1 — PASS
    (ct-events and other `count shown: unknown` pivots exempt)
```

FAIL if any registered `count shown: yes` pivot is 0 on the nominated graph-root. Fix the fixture (or the sibling fetcher enrichment) until it's non-zero — do NOT accept a "best attempt" graph-root that covers 9 of 10 pivots.

#### 9.4 Detail view surfaces every finding — with a test

The unified `Attention (N)` section in the detail view must render every finding carried by the selected resource:

- Every Wave-1 phrase from `Resource.Issues`, first letter capitalized, `!`/`~` prefixed per severity.
- Every Wave-2 `EnrichmentFinding` — Summary line plus Rows beneath as indented `Label: Value` context.

This must be asserted by a phase-8 scenario test that opens the multi-issue fixture's detail view and calls `ExpectViewContains(...)` for every expected phrase / row value. The test must target a fixture with ≥2 findings so the test proves no finding silently disappears (the 2026-04-22 regression class).

Report format:

```text
9.4 detail-view completeness test: TestScenario_<Short>Visual#multi_issue_detail
    Fixture: <multi-issue fixture-id>
    Findings asserted in detail (via ExpectViewContains):
      - <Wave-1 phrase A>
      - <Wave-1 phrase B>
      - <Wave-2 Summary>
      - <Wave-2 Row: Label=Value>
      ...
    Rendered frame pasted into test log (t.Log(scenario.currentView())): YES
    PASS
```

If any expected phrase is missing from the rendered detail frame (even with green unit tests) — FAIL. The rendered frame from 8.4 is the authority.

#### 9.5 Final report format

Aggregate the four items plus any skipped ones under a dedicated header in the PR-ready report:

```text
## Phase 9 Report Checklist

9.1 illegal UI elements: PASS
9.2 fixture coverage: PASS (Wave-1 N/N, Wave-2 N/N, multi-issue: <id>)
9.3 graph-root all-pivots-non-zero: PASS (<fixture>, N/N pivots)
9.4 detail-view completeness: PASS (test: <name>, rendered frame logged)

Implementation: DONE.
```

If any item is FAIL, do NOT emit "DONE" — loop back to the phase that owns the gap (fixture gaps → phase 6a; test gaps → phase 6b; rendering gaps → phase 7 or the scenario test).

## What this skill never does

- Does not commit or push. The user does that.
- Does not touch unrelated resources. Scope is one shortName per invocation.
- Does not skip phase 2. If there are TBDs, they get resolved before any code moves.
- Does not "preserve" existing test expectations. The pseudocode spec in phase 3 is authoritative; anything in `tests/**` that contradicts it is wrong by definition.

## Handling a spec change mid-flight

If phase 2 produces a TBD answer that materially changes §2 or §4 of the spec doc, update the spec first, then restart phases 3 and 4 from the amended spec. The impl-plan doc always reflects the current spec. Cheaper than discovering the contradiction in phase 7.

## What to do when the spec is wrong

Occasionally the spec has an actual factual error — e.g. an AWS field that doesn't exist on the list response. Stop the skill and regenerate the spec (`a9s-resource-spec <shortName>`) first. Do not patch around the error at the impl-plan level; that just moves the drift.
