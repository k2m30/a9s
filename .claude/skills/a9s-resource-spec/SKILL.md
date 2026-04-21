---
name: a9s-resource-spec
description: Generate a uniform, implementation-blind specification for a single a9s resource type. Use whenever the user asks to "spec", "document", or "describe" a resource by its shortName (e.g. "spec ec2", "write the resource doc for dbi", "generate the lambda spec", "create docs/resources/s3.md"), or asks for the contract, related panel, issues algorithm, or Wave 1/Wave 2 behavior for a named AWS resource type in the a9s codebase. Reads only the four golden docs in docs/ — never source code — and writes docs/resources/<shortName>.md following a strict template so all resource specs are diff-able and can drive test and fixture generation. Trigger this skill for any request that names an a9s resource shortName and asks for a doc, spec, contract, or behavior summary.
argument-hint: <shortName>
allowed-tools:
  - Read
  - Glob
  - Grep
  - Write
  - Update
  - Edit
  - Bash
  - Bash(go doc *)
  - Bash(mkdir -p *)
  - Agent(a9s-devops)
  - Agent(general-purpose)
  - WebFetch(domain:docs.aws.amazon.com)
---

# a9s Resource Spec Generator

Generate `docs/resources/<shortName>.md` — the **golden UX/UI document for one a9s resource type**, written from the operator's perspective. It describes *how the resource should appear and behave to a human using a9s*: what the row looks like in the list, what color it is, what glyph if any, what the Status column says, what the detail view reveals, what related resources are one key press away. Not how the code is organized — how the UX is.

## What this doc is for

This is the **should-be**, not the is. It is the contract; implementation conforms to it. When code and this doc disagree, code is wrong.

Concretely, each generated doc drives three downstream activities:

1. **UX/UI review** — a reader (operator, designer, reviewer) can read it and immediately agree or disagree that this is how the resource should look and feel. Disagreement is a design conversation, not an engineering bug.
2. **Implementation verification** — tests, consistency checkers, and code reviews compare production behavior to this doc.
3. **Fixture and test generation** — the exact Status column strings and detail sentences in the doc become the assertions tests run.

## WHAT vs HOW — where each source fits

This split matters and anchors everything the skill does:

- **WHAT** — which resources exist, which related pivots they have, which issues to watch for, which AWS APIs return what. Lives in the four a9s golden docs and the AWS API Reference. Changes slowly. Answers "does this resource have an X issue?"
- **HOW** — how each WHAT is delivered to the operator: row color, glyph, Status column wording, detail-view sentence, menu count rule. **This is what the generated `docs/resources/<shortName>.md` establishes.** HOW is a UX/UI decision and is expected to evolve. Regenerating a spec captures the current HOW.

The four a9s golden docs are **not** the HOW. Passages in `docs/enrichment-visibility.md` that look like HOW decisions (row middle-dot `·`, `⚠ Background Check` header, derived list-level banner) are stale HOW that this skill now supersedes. The current HOW lives in this skill's surface rules (S1–S5) and in the generated per-resource doc.

Ground each generated spec in:

- **WHAT sources** — the four a9s golden docs (resources to show, issues to watch, API references to use) and the AWS API Reference (field names and response shapes).
- **HOW sources** — this skill's S1–S5 surface rules, applied to the WHAT.

## Why implementation-blind

The skill never reads a9s source. If it did, the code would silently become the spec and drift would stop being detectable — the whole point is catching drift between should-be (this doc) and is (the code).

When a WHAT is not specified in the golden docs or the AWS Reference, the spec writes `TBD — not specified`. A `TBD` is valuable — it surfaces an unmade decision so the team can make it deliberately.

## Handling gaps — escalate, don't TBD

Raw `TBD` is a last resort. Before writing one, gaps go through two escalation steps:

### Step 1 — Ask the `a9s-devops` agent (mandatory for every would-be TBD)

The skill **must** dispatch `a9s-devops` (`.claude/agents/a9s-devops.md`) whenever it would otherwise write a `TBD`. The agent is a senior AWS practitioner who knows real workflows — it fills gaps the golden docs don't cover with operator-grounded answers.

For each gap, ask two questions:

1. **Possible?** — Does AWS expose what we need (a field, an API, a cross-reference) to answer this gap? Cite the field or API if yes.
2. **Worth it?** — Does a daily-driver operator actually benefit from this being in the spec? Explain the workflow or explicitly say no.

Prompt shape to `a9s-devops`:

> Context: generating the a9s UX/UI spec for resource `<shortName>`. Gap: `<describe what is silent in the golden docs>`. Please answer: (a) is it possible to fill this from AWS — which field or API? (b) is it worth filling for a daily-driver operator — what's the workflow? Return both answers in 3-5 sentences, including any field/API citation.

Interpret the agent's response:

- **possible=yes, worth=yes** → write the answer into the relevant spec section (§2 related target, §3 signal, §4 table cell), followed by `— a9s-devops: <one-line rationale>`. The TBD is resolved.
- **possible=yes, worth=no** → record in §5 Out of Scope with the rationale: `<what> — a9s-devops: not worth it, <reason>.` No TBD.
- **possible=no** → record as `TBD — a9s-devops: not available in AWS surface, <reason>.` This is the rare case where a real `TBD` remains, now documented.

Batch related gaps into one agent dispatch where sensible (e.g. "how are `alarm`, `asg`, `backup`, `logs`, `ng`, `tg` discovered as related targets for ec2?" is one dispatch, not six).

### Step 2 — Ask the user (only for material HOW choices)

Use sparingly. Ask the user directly only when the call is a **UX/UI** choice, not an AWS-practitioner question — the devops agent can't decide `!` vs `~` severity by itself because that's a product decision. Examples that go to the user, not devops:

- Severity choice (`!` vs `~`) for a borderline finding when it will establish a rule for future resources.
- Which AWS field, from several equally valid candidates, to surface in the Status column when the visible text matters.

How to ask: one focused question at a time, phrased from the operator's perspective. If the user answers "decide", the skill decides and notes the call in §6 Citations.

### Decision record

Every devops consultation and every user answer goes into §6 Citations on its own bullet, with this format:

- `<claim>` — `a9s-devops agent (<date>): possible=yes|no, worth=yes|no. <rationale>.`
- `<claim>` — `user decision (<date>): <short answer>. <rationale if given>.`

Remember decisions within a session so the skill doesn't re-ask.

## Operator perspective — keep asking

Every sentence you write should answer one of these, in order:

- What does the operator **see** on the list row for this resource? (color, glyph, identity, status text)
- When they see something concerning, what does it **mean** — in their words, not AWS jargon?
- What's their **next step** — stay on the list, or press detail for the full story?
- Which **other resources** do they care about from here, and what does each pivot tell them?

If a sentence in the generated spec doesn't serve one of those questions, cut it.

## Inputs

- `<shortName>` — the a9s resource short name, e.g. `ec2`, `dbi`, `lambda`, `s3`, `ng`, `sg`, `elb`.

## Sources of truth

**Primary — the four golden docs:**

1. `docs/architecture.md` — layering, read-only invariant, allowed UI surfaces.
2. `docs/related-resources.md` — per-type contract of related targets. The AWS API Reference URL for each type lives here.
3. `docs/attention-signals.md` — Wave 1 / Wave 2 / Wave 3 signals per type. Each row cites its AWS API.
4. `docs/enrichment-visibility.md` — historical record of which surface categories exist (menu count, row color, glyph, status text, detail line). Treat it as WHAT only; its specific HOW mechanics are superseded by this skill's S1–S5 rules (see "Superseded HOW" below).

**Secondary — the AWS Go SDK v2 types.** a9s vendors `github.com/aws/aws-sdk-go-v2`. The skill consults the SDK directly with `go doc` — authoritative, local, instant. Use it to:

- Confirm an AWS field name is spelled correctly and actually exists on the response shape. `go doc` shows the Go struct with field names as returned by the SDK (same as the wire format for AWS APIs).
- Find the exact field that carries a human cause for S4 Status text — e.g. for a stopped EC2 instance the cause lives in `StateReason.Message` and `StateTransitionReason`; for a failed NAT gateway it is `FailureMessage`; for a certificate expiry the value is `NotAfter`. These are the fields that let the list row read `stopping: Server.SpotInstanceShutdown` instead of a bare state word.
- Decide whether a signal is list-response only (Wave 1) or genuinely needs a Describe call (Wave 2). Compare the List* output type against the Describe* output type — if the field only exists on the Describe shape, the signal is Wave 2.

Typical commands — **must be run from the a9s project root** (where `go.mod` lives) so module resolution works. Running from `$HOME` will fail with `cannot find package ... in any of: ($GOROOT not set)`.

```bash
# cwd = /Users/k2m30/projects/a9s
go doc github.com/aws/aws-sdk-go-v2/service/<svc>/types.<Shape>
go doc github.com/aws/aws-sdk-go-v2/service/ec2/types.Instance
go doc github.com/aws/aws-sdk-go-v2/service/acm/types.CertificateSummary
go doc github.com/aws/aws-sdk-go-v2/service/acm/types.CertificateDetail
```

If the Bash call is not already running in the project root, prefix with `cd /Users/k2m30/projects/a9s &&` or invoke via the Bash tool where the working directory is the project.

**Tertiary — web docs (fallback only).** When the SDK comment is sparse or the semantics of a field aren't clear from the struct, fetch the AWS API Reference HTML (`docs.aws.amazon.com`). Prefer the SDK; fall back to web only when you need the narrative explanation.

Citation format follows the source you used:

- SDK: `AWS SDK Go v2 — <package>/types.<Shape> § <Field>`
- Web (fallback): `AWS API Reference: <API page name> § <field> (<URL>)`

When the SDK and the golden doc contradict each other on a field name, prefer the SDK — it is what the code actually sees on the wire.

**Never read a9s source.** Do not open `internal/**`, `cmd/**`, `tests/**`, `.a9s/**`. If the question is about a9s *behavior* (how it fetches, how it displays), the answer is in the golden docs or it is `TBD`. If the question is about *AWS surface* (what field names exist, what shape a response has), the answer is in the AWS API Reference.

## Procedure

1. **Validate the shortName.** Confirm the shortName appears in both:
   - the "Per-type contract" table in `docs/related-resources.md`, and
   - one of the signal tables in `docs/attention-signals.md`.

   If either is missing, stop and report exactly which doc lacks the row. Do not proceed, do not fabricate. This is the single hard-stop in the procedure.

2. **Extract the raw material.** From each source, pull what you need for the shortName:
   - `related-resources.md`: the contract row (AWS API URL + expected related targets), plus any reasoning-column notes nearby.
   - `attention-signals.md`: `Name`, `Wave 1`, `Wave 2`, `Wave 3`, `Source` cells for the row.
   - `enrichment-visibility.md`: only the high-level fact that surface categories exist. Do not copy specific mechanics — use the S1–S5 rules in this skill instead.
   - `architecture.md`: the read-only invariant — cited once in Out of Scope.
   - AWS Go SDK v2 (`go doc github.com/aws/aws-sdk-go-v2/service/<svc>/types.<Shape>`): the exact field name(s) that carry the cause text for S4 / S5, and which fields are on the List* vs Describe* shape.

3. **Fill the template** in `references/output-template.md` exactly. Keep every section heading and ordering. Keep AWS field names verbatim (`State.Name`, `health.issues[]`, `StorageEncrypted`, etc.) — paraphrasing loses the precision tests rely on.

4. **Handle silence.** Every would-be `TBD` escalates. Order: (a) for material UX/UI choices (severity, which field to surface), ask the user directly; (b) for anything else — discovery mechanisms, workflow rationale, cost/benefit — dispatch `a9s-devops` for a possible/worth verdict and record it inline with a citation (see "Handling gaps — escalate, don't TBD" above); (c) only write a raw `TBD` when devops confirms the gap is not fillable from AWS surface. Never infer from a9s source, never invent a mechanism.

5. **Write the file.** `docs/resources/<shortName>.md`. Create the directory if it is missing. Overwrite an existing file — uniformity wins over preservation.

6. **Print a one-line summary** to the conversation (not into the file):

   ```text
   <shortName>: related=<N> wave1=<N> wave2=<N> wave3=<N> tbd=<N> devops=<N> superseded=<N>[ (<short list>)]
   ```

   `devops` = number of gaps filled by `a9s-devops` consultation (possible=yes answers written into the spec). `tbd` = remaining unfillable gaps after escalation. `superseded` = passages in the golden docs that describe now-superseded HOW (see "Superseded HOW" below). If zero, omit the parenthetical. This one line is the user's quick eyeball check that the extraction ran.

## Output shape

See `references/output-template.md` for the full template. Sections, always in this order:

1. Frontmatter — `shortName`, `name`, `awsApiRef`, `generatedFrom`.
2. Identity — shortName, display name, AWS API URL, list API, describe API (or "not used").
3. Related Resources Panel — one subsection per related target (why / how discovered / count shown). `ct-events` goes last, flagged as the universal pivot.
4. Attention Algorithm — 3.1 Wave 1, 3.2 Wave 2, 3.3 Wave 3 (copied verbatim, each bullet prefixed `OUT OF SCOPE:`).
5. Issue Visualization — per-signal table mapping every Wave 1/2 signal to surfaces reached, exact S4 list text, and S5 detail text. Includes §4.1 "UX review" paragraph.
6. Out of Scope — Wave 3 signals, any UI element not in S1–S5, any write operation.
7. Citations — one bullet per claim, citing either `docs/<file>.md § <heading>` or `AWS API Reference: <page> § <field> (<URL>)`.

## Allowed visualization surfaces (exactly five)

These are the only surfaces the spec may reference. Any other "surface" the golden docs describe is superseded HOW — the skill flags it for removal (see "Superseded HOW in the golden docs" below) rather than including it.

- **S1 — Menu `issues:N` count** on the main menu view. Counts `!` findings only. `~` findings are deliberately excluded — they are informational annotations, not issues the operator needs to chase.
- **S2 — Row color** in the list view. The row is colored by its state bucket: Healthy → green (`ColRunning` `#9ece6a`), Warning → yellow (`ColPending` `#e0af68`), Broken → red (`ColStopped` `#f7768e`), Dim → terminated/gray. Yellow/red/dim are themselves the "something is off" signal — they need no further annotation to get the operator's attention.
- **S3 — `!` / `~` glyph** prefixed before the name. Glyphs attach **only to green (Healthy) rows** as background-check annotations meaning "no immediate action needed, but worth knowing." Examples: RDS `available` + `~` → maintenance scheduled in 12 days; ACM `ISSUED` + `!` → certificate expires in 7 days. Glyphs **never** appear on yellow, red, or dim rows — those colors are already sufficient indicators.
- **S4 — Status / description column text** in the list view, carrying the condition as short human-readable text (e.g. `stopping: Server.SpotInstanceShutdown`, `expires in 7d`). **Healthy rows render this column blank** — no `OK`, no `available`, no `ACTIVE`, no `running`. Empty-ness is the signal "nothing to see here", which drastically reduces list noise. Non-healthy rows always carry cause text here (state keyword alone is not enough; pair it with the reason).
- **S5 — Detail view enrichment line** — a short operator-readable sentence shown in the detail view for a resource that has a finding. No ceremonial header, no banner ornament; just the line rendered inline with the detail fields.

Mapping rules for §4 of the generated spec:

- A **Wave 1 Healthy** row is not worth a §4 row at all. S4 renders blank, no glyph, no finding. Omit from the table.
- A **Wave 1 Warning / Broken / Dim** signal drives **S2** (color — already the attention signal) and **S4** (cause text, never a bare state keyword). It does not reach S1, S3, or S5 because Wave 1 does not produce a finding object, and S3 is forbidden on non-green rows.
- A **Wave 2 Broken-style background finding on a Healthy resource** gets `!` → **S1, S3, S4, S5**. The row stays green; the `!` glyph and a short S4 cause-line flag the concern; operator can press detail for the full S5 sentence. Examples: ACM certificate nearing expiry, EC2 scheduled retirement <7d.
- A **Wave 2 Warning/informational background finding on a Healthy resource** gets `~` → **S3, S4, S5**. Does **not** bump S1. Examples: RDS maintenance scheduled, EBS snapshot approaching cost-threshold age.
- A **Wave 2 finding that lands on a resource whose row is already yellow/red/dim** is redundant visually — the color is already the signal. The finding is still worth recording in S5 for the detail view, and it may still bump S1 (if `!`), but S3 is suppressed (no glyph on non-green rows) and S4 should deduplicate with the existing cause.

A signal that cannot land on at least one of these five surfaces is a gap — flag it in the generated spec's §5 Out of Scope, do not invent a new element.

## UX rules the spec must enforce

The spec drives implementation, so the spec must make bad UX impossible. A real failure mode this skill is designed to prevent: a list row showing `DORMANT` with no reason the user can read, and a jargon-banner `count is a lower bound (truncated)` above it. The user walks away knowing something is wrong but not what.

Rules every generated spec must follow:

- **No internal jargon in any user-visible text.** In every Status column value, glyph tooltip, and detail summary the spec writes, these words are banned: `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`. If a phrase from the golden docs uses these words, rewrite it in plain operator language for S4/S5, and note the rewrite.
- **State keywords are not explanations.** `DORMANT`, `available`, `stopped`, `failed`, `ACTIVE` on their own tell the user nothing. In S4 (status/description column) or S5 (detail summary), the spec must pair the state with a cause the user cares about — e.g. `stopped: Server.SpotInstanceShutdown`, `dormant: not accessed in 400d`, `failed: IAM role missing permissions`. The spec's Summary column shows the exact text an operator would read.
- **Every §3 signal has a readable reason.** For Wave 1 signals, fill the Summary column with the exact S4 / S5 wording a user would read. `n/a (row color only)` is not acceptable — a red row with no words is the screenshot bug. Even a Healthy row, when its spec row appears in §4, gets a short descriptor like `running` or omits S4/S5 entirely by leaving the row out of §4.
- **S3 glyph never appears alone.** When `!` or `~` is prefixed to a name, either the same row carries the cause in S4 (preferred — no navigation needed), or the cause is in S5 and the operator reaches it via the standard detail keypress. S5 is always the fallback; the spec does not pick a different route.
- **S1 is the only terse surface.** The menu `issues:N` count is a number; it carries no reason, and that's fine because the user drills in to the list.
- **Write for the operator, not the architect.** Read each Summary back aloud as if you were the on-call engineer glancing at the list. If the answer to "what do I do next?" is not obvious from that line, rewrite it.

Add a brief §4.1 "UX review" block to every generated spec — a two-sentence paragraph answering, for this resource: "At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail?" If the answer is no for any §3 signal, call it out as a UX gap the implementation must fix (e.g. add the cause to the Status column).

## Superseded HOW in the golden docs

The golden docs contain passages that look like HOW decisions — because HOW used to live there. Known superseded passages: row middle-dot `·` marker, `⚠ Background Check` detail header, derived list-level banner (`⚠ N issues detected by background checks`). These are not drift in the usual sense — they are earlier UX calls, now replaced by the S1–S5 rules above and the per-resource specs this skill generates.

When the skill encounters such a passage:

1. Ignore it for generation purposes — do not cite it, do not reuse its mechanics.
2. In the one-line summary printed after writing the file, append `superseded=<count>` with a short list — e.g. `superseded=2 (row dot, banner)`.
3. Ask the user: "Found N superseded HOW passages in `<file>`. Want me to propose edits that remove them so the golden docs stay WHAT-only? (y/n)". On `y`, produce a diff patch for user review and apply only after explicit approval. On `n`, leave the golden doc untouched; the spec still ignores those passages.

The skill has permission to edit golden docs **only** to remove superseded HOW and **only** after the user approves each diff. It never adds new content to golden docs — HOW content belongs in the per-resource spec, not in the WHAT docs.

## Generation quality rules

- **Every claim carries a citation.** If you cannot cite it to a golden doc or the AWS API Reference, you cannot write it. Better to emit a `TBD` than an unsourced sentence.
- **One bullet per distinct signal.** Do not collapse "status checks" or "encryption checks" into a single line — tests want to target each condition separately.
- **Uniformity beats eloquence.** The docs are consumed by diff tools and test generators. A boring, mechanical extraction that matches the template in every resource is the right output.

## What this skill does not do

- Does not edit source code.
- Does not read source code.
- Does not run tests or verify anything. It produces the contract that other tools verify against.
- Does not invent behavior to fill gaps — gaps become `TBD` markers so the golden docs can be patched.
