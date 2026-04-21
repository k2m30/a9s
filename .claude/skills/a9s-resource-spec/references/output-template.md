# Output Template — `docs/resources/<shortName>.md`

Use this template verbatim. Every generated spec must have these seven sections in this order.

---

````markdown
---
shortName: <shortName>
name: <Display Name from attention-signals.md>
awsApiRef: <URL from related-resources.md>
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# <shortName> — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `<shortName>`
- **Display name**: <name>
- **AWS API reference**: <url>
- **List API**: <list operation as named in attention-signals.md "Source" column, e.g. `DescribeInstances`>
- **Describe API (if any)**: <per-resource Describe used in Wave 2, or "not used">

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `<t1>`, `<t2>`, … .

### `<target-shortName>`

- **Why related**: <one-sentence DevOps pivot or AWS API field citation from related-resources.md>.
- **How discovered**: <"read field X on the resource" | "cross-reference the already-loaded `<type>` list by <field>" | "call <AWS API>"> — or `TBD — not specified in related-resources.md` if the doc is silent.
- **Count shown**: yes | unknown.

<Repeat the subsection for every target in the contract row. Put `ct-events` last and flag it as:
"universal pivot — applies to every registered type; see related-resources.md §Policy.">

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `<condition from Wave 1 cell>`.
  - **State bucket**: Healthy | Warning | Broken | Dim.
  - **How obtained**: <which list-response field, or which cross-referenced sibling list>.

If the Wave 1 cell is `None`, write a single line: `No Wave 1 signals — the list API does not return fields usable for attention.`

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `<condition from Wave 2 cell>`.
  - **State bucket**: Healthy | Warning | Broken | Dim.
  - **API call**: <exact API name + scope: "one account-wide call" | "one per resource" | "one per N resources">.
  - **Cost shape**: account-wide | per-resource | hybrid.

If Wave 2 is `None`, write: `No Wave 2 signals.`

### 3.3 Wave 3 — OUT OF SCOPE

Copy the Wave 3 cell verbatim. Prefix every bullet with `OUT OF SCOPE:`. These are documented so the reader knows what is intentionally excluded from a9s; they are not to be implemented.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `stopping: Server.SpotInstanceShutdown`, `expires in 7d`). **Healthy rows render blank** — no `OK` / `available` / `ACTIVE` / `running`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4 (short cause), S5 (full sentence). No S1.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `<condition>` | 1 \| 2 | Healthy \| Warning \| Broken \| Dim | `!` \| `~` \| n/a | `<exact short cause for the status column, e.g. "stopped: Server.SpotInstanceShutdown" — never a bare state keyword>` | `<full one-line operator-readable sentence, e.g. "Instance stopped by AWS spot reclamation on 2026-04-12.">` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? If any §4 row fails that test, name the gap here and the fix (e.g. "add `age` to the Status column", "show `LastAccessedDate` on the list"). Otherwise write: "All problem rows are self-explanatory in the list — operator can triage without opening detail."

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

One bullet per claim in §§2–4.1. Citation sources, in order of authority:

- a9s golden doc — `<short claim>` — `docs/<file>.md` § `<heading or table row>`.
- AWS Go SDK v2 — `<short claim>` — `AWS SDK Go v2 — <package>/types.<Shape> § <Field>`.
- AWS API Reference (fallback) — `<short claim>` — `AWS API Reference: <page>` § `<field>` (`<URL>`).
- a9s-devops consultation — `<short claim>` — `a9s-devops (<YYYY-MM-DD>): possible=yes|no, worth=yes|no. <rationale>`.
- user decision — `<short claim>` — `user (<YYYY-MM-DD>): <answer>. <rationale if given>`.

Golden docs describe a9s *behavior*. The SDK and AWS docs describe the *response shape* — prefer the SDK, it's what the code sees. `a9s-devops` fills gaps the golden docs leave silent using real operator workflow knowledge. `user` resolves material UX/UI calls. If a claim cannot be cited from any of these, replace it with `TBD — a9s-devops confirmed not available in AWS surface, <reason>`.
````
