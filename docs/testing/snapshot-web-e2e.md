# Real-account web-UI checklist testing

Per-resource verification of the **web UI** against expected checklists derived
independently from a real read-only AWS account. The suite answers one question
for every resource type: *given the account's real data, does the rendered web
page show what `docs/resources/<type>.md` says it should?*

The a9s binary is not modified for this in any way — the collector talks to the
AWS API directly, and the test drives the stock `./a9s --web` against the same
live profile.

## The three steps

| Step | Tool | Input | Output |
|------|------|-------|--------|
| 1. Collect | `cmd/snapshot` | read-only AWS profile (AWS SDK only) | `snapshot.json` — ONE big raw-facts file, all 66 types |
| 2. Checklist | `cmd/checklist` | `snapshot.json` + `docs/resources/<type>.md` rules | `checklists/<type>.json` — per-type screen checklist |
| 3. Assert | Playwright, live mode | `checklists/<type>.json` + `./a9s --web -p <profile>` | pass/fail against the rendered DOM |

```text
 AWS API ──(cmd/snapshot)──▶ snapshot.json ──(cmd/checklist)──▶ checklists/<type>.json
                                                                   │
 AWS API ──(./a9s --web -p profile)──▶ real browser ──▶ assertions ◀┘
```

The checklist generator is a *second, independent* implementation of "what should show" — it
never reads a9s UI or enrichment code, so a passing test is a genuine agreement
between two derivations, not a tautology.

Both sides read the same live account, so the baseline can drift between
collection and the test run. The workflow is **refresh, then test**: re-run
steps 1–2, then run step 3 against the same account. A count mismatch first
warrants a collector re-run, not a bug report.

## Quick start (s3 example)

```sh
# 1. Collect raw facts for all 66 types (profile name never enters code; output is gitignored)
go run ./cmd/snapshot --profile my-real-profile --region eu-west-2

# 2. Derive the expected checklist from the facts + the spec rules
go run ./cmd/checklist --snapshot tests/e2e/testdata/snapshot/my-real-profile--eu-west-2 --types s3

# 3. Boot the real web UI on the same profile and assert in a real browser
cd tests/e2e
A9S_E2E_PROFILE=my-real-profile A9S_E2E_REGION=eu-west-2 \
A9S_E2E_CHECKLIST=tests/e2e/testdata/snapshot/my-real-profile--eu-west-2 \
  npx playwright test live-s3.spec.ts
```

## Non-negotiable rules

1. **The documentation is the source of truth.** Validate the renderer against
   `docs/resources/<type>.md` (and its `-impl-plan.md`), never against the other
   renderer — copying the TUI can propagate a TUI bug.
2. **Assert rendered pixels, never CSS classes.** A `dec-error` class that draws
   nothing is not a `!`. Assert visible glyph characters, `toHaveCSS` computed
   colors, rendered cell text, and blank cells — what the operator actually sees.
3. **No real data in committed files.** The profile name is a runtime flag; the
   entire output directory (`tests/e2e/testdata/snapshot/` — real resource
   names) is gitignored via the root `.gitignore`. Only tooling, specs, and
   these docs are committed.
4. **No test plumbing in the product.** The binary gets no flags, env vars, or
   modes for this suite — step 3 runs the same `./a9s --web -p <profile>` a
   user runs.

## Scope

Current scope is the **root menu + resource list** views. Detail view, related
panel, and child views are deferred. Implemented so far: `s3`. To add a type,
extend the collector and checklist generator (see [step1-collector.md](step1-collector.md),
[step2-checklist.md](step2-checklist.md)) and clone the live spec.
