# Step 2 — Checklist generator (`cmd/checklist`)

Computes the **expected** root-menu + list-view checklist for each resource
type, purely from the normalized snapshot facts
([step 1](step1-collector.md)) plus the rules transcribed from
`docs/resources/<type>.md`. Part of the
[snapshot web-e2e suite](snapshot-web-e2e.md).

The checklist generator is an **independent second implementation** of "what should show".
It never reads a9s UI or enrichment code and never touches AWS — so when the
Playwright step compares the rendered web UI against `checklists/<type>.json`, a
pass is a genuine agreement between two independent derivations, not a tautology.

## What it produces

```text
tests/e2e/testdata/snapshot/<profile>--<region>/checklists/<type>.json
```

Gitignored (inside the snapshot dir). Regenerate any time from `snapshot.json`
without re-hitting AWS.

## Running

```sh
go run ./cmd/checklist --snapshot tests/e2e/testdata/snapshot/<profile>--<region> --types s3
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--snapshot` | *(required)* | Snapshot dir containing `snapshot.json`; `expected/` is written next to it. |
| `--types` | `s3` | Comma-separated resource short names. |

## Output format

```jsonc
{
  "type": "s3",
  "menu": {
    "display": "S3 Buckets",
    "availability": 50,          // shown lower-bound count
    "avail_truncated": true,     // renders "(50+)" vs "(50)"
    "issues": 5,                 // issue-badge count (S1)
    "issues_truncated": true     // renders "! 5+"
  },
  "list": {
    "columns": ["Bucket Name", "Region", "Creation Date", "Status"],
    "jargon":  ["Public Access", "CIS", "Flags", ...],  // headers that must NEVER appear
    "shown": 50,                 // rows on the first page
    "truncated": true,           // "Results truncated" indicator
    "colors":     { "healthy": 50, "warning": 0, "broken": 0, "dim": 0 },
    "decorators": { "!": 5, "~": 0 },
    "flagged": [ { "name": "...", "decorator": "!", "status": "public access block incomplete" } ]
  }
}
```

Every field maps to an operator-visible signal the step-3 test asserts against
the rendered DOM.

## Where the rules come from

Per resource, the checklist generator applies the **Issue Visualization** surfaces from
`docs/resources/<type>.md` §4 (S1–S5) and the **Attention/Issues Algorithm**
from §3, plus list-level items from `docs/resources/<type>-impl-plan.md`. For s3:

| Surface | Rule (s3.md §4) | Checklist output |
|---------|-----------------|---------------|
| S1 menu `issues:N` | count of `!`-severity findings (`~` does not bump) | `menu.issues` |
| S2 row color | color by state; s3 has no Wave-1 signal → all Healthy=green | `list.colors` all `healthy` |
| S3 `!`/`~` glyph before the name | on a Healthy (green) row only; PAB-incomplete → `!` | `list.flagged[].decorator` |
| S4 Status text | finding cause on flagged rows; **Healthy rows render blank** | `flagged[].status` + blank elsewhere |
| U10 (impl-plan) | exact columns; no jargon headers | `list.columns` / `list.jargon` |

### Framework constants

The checklist generator models framework behavior the list obeys, as documented constants:

- `pageSize = 50` (`resource.DefaultPageSize`) — the list shows the first page;
  `truncated`/`avail_truncated` when total exceeds it.
- `enrichmentCap = 50` (`aws.EnrichmentCap`) — Wave-2 issues are computed over
  the first N; `issues_truncated` when total exceeds it.

These are deliberate contract values. If the framework changes one, that is a
contract change and the constant here must be updated in lockstep.

## Adding a resource type

Add a `checklist<Type>` function in `cmd/checklist/` that unmarshals its raw
section of `snapshot.json` and applies that type's §3/§4 rules to fill an `Expected`,
then register it in the `--types` switch in `main.go`. Redeclare the data shape
locally (the checklist generator depends on no collector internals — reinforcing
independence).

## The golden rule

Expected values describe **operator-visible signals**, and the step-3 test
asserts them as **rendered pixels** — visible glyph characters, `toHaveCSS`
computed colors, cell text, blank cells — never CSS classes. An early s3 spec
asserted the `dec-error` class and reported "5 rows with !" while the page showed
**no `!` at all** (the web template set a class but drew no glyph). Asserting the
class hid a real web bug. Assert what the operator sees.
