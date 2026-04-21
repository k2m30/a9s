---
name: a9s-create-demo-fixture
description: Create or update the single-source fixture file for a resource type under `internal/demo/fixtures/<shortName>.go`. Use when the coder is given a fixture-creation task during phase 6 of `a9s-implement-resource`, or any time a resource type needs new demo/test data (new states, new edge cases, new spec coverage). Takes a plain-language fixture list (typically from `docs/resources/<shortName>-impl-plan.md` §2) and produces realistic, graph-connected AWS SDK typed fakes that both `./a9s --demo` and the unit test suite import. Cross-references sibling fixture files so every related-panel pivot renders a non-zero count. Never creates orphan fixtures — a DB instance fixture without matching KMS keys, security groups, alarms, and CloudTrail events is a bug, not a feature. Adversarial / malformed fixtures (nil pointers, error cases) stay inline in tests and are explicitly out of scope for this skill.
argument-hint: <shortName>
allowed-tools:
  - Read
  - Glob
  - Grep
  - Bash(go doc *)
  - Bash(ls *)
  - Bash(rg *)
  - Bash(./a9s --demo *)
  - Bash(make build)
  - Write
  - Edit
---

# a9s Demo Fixture Skill

Take a plain-language fixture list and produce a **realistic, graph-connected** demo fixture file under `internal/demo/fixtures/<shortName>.go` that both `./a9s --demo` and the unit test suite import from. One file per service. No duplication between demo and test data.

## Why this skill exists

A fixture that compiles and passes its own unit tests is not enough. The demo is the product's showroom — an operator opens `./a9s --demo`, navigates to the resource list, opens the detail view, and expects every pivot in the related panel to render a sensible count. If the dbi fixture references `kms-key/abcd-1234` but no KMS fixture has that key, the `kms` related row shows `0` and the demo looks broken — even though the fetcher, the related checker, and the unit tests are all correct. The graph must be stitched.

Writing the fixture blind is not the hard part. The hard part is:

1. Knowing what IDs/ARNs every related-panel checker actually reads, so the fixture references values that exist in sibling fixture files.
2. Covering the state matrix the spec describes (one fixture per Wave 1/Wave 2 state in §3–§4).
3. Appending matching entries to sibling fixture files (alarm dimensions, CloudTrail events, snapshots) so the cross-resource graph lights up.
4. Knowing when NOT to add a fixture to the demo file (adversarial nil-pointer / error-path fixtures corrupt the demo and belong inline in tests).

## Inputs

- `<shortName>` — the resource type, e.g. `dbi`, `ec2`, `lambda`.
- **Required prerequisite**: `docs/resources/<shortName>-impl-plan.md` §2 exists with a plain-language fixture list. If missing, stop and tell the user to run phases 0–4 of `a9s-implement-resource` first.

## Files the skill is allowed to read

**Required:**
- `docs/resources/<shortName>.md` — spec, for state coverage and §2 related-panel targets.
- `docs/resources/<shortName>-impl-plan.md` — fixture list is authoritative for what fixtures to produce.
- `internal/aws/<shortName>_related.go` — related checkers, to know exactly which fields each pivot reads.
- `internal/demo/fixtures/<shortName>.go` — the existing fixture file for this service, if any (read it so we extend rather than duplicate).
- `internal/demo/fixtures/<peer>.go` for every related-panel target — to check whether the IDs/ARNs this fixture references already exist and to plan sibling updates.
- `internal/demo/handlers.go` — to confirm whether a new handler branch is needed or the typed-fake path covers this service.

**Forbidden to read:**
- `tests/**` — test files never influence demo fixture design.

## Canonical fixture shape (single-source rule)

Follow the convention already in the tree:

```go
package fixtures

import (
    "github.com/aws/aws-sdk-go-v2/aws"
    rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

// <Service>Fixtures holds typed fixture data for <service>.
type <Service>Fixtures struct {
    Instances []rdstypes.DBInstance // or the appropriate raw SDK slice
}

const (
    // stable IDs / ARNs referenced from this and sibling fixture files
    ProdDbiID  = "prod-db"
    ProdDbiARN = "arn:aws:rds:us-east-1:123456789012:db:prod-db"
)

func New<Service>Fixtures() *<Service>Fixtures {
    return &<Service>Fixtures{
        Instances: []rdstypes.DBInstance{
            baselineHealthy(),
            withStatus(baselineHealthy(), "failed"),
            withStatus(baselineHealthy(), "storage-full"),
            // … one per spec state
        },
    }
}
```

Rules:

- **One file per service**, at `internal/demo/fixtures/<shortName>.go`. No `_fixtures` suffix. Fold any existing `<shortName>_fixtures.go` into it.
- **Raw SDK types only** (e.g. `rdstypes.DBInstance`, not `resource.Resource`). The demo app runs the real fetcher over these; tests feed them into fetchers directly. Single source of truth.
- **Exported stable IDs/ARNs as `const`** so sibling fixture files reference them by symbol, not by string literal. When the ID needs to change, one rename ripples through.
- **Each state variant is its own element** in the slice — one row per state the spec demos. Prefer a `baseline*()` helper + mutator functions (`withStatus`, `withPendingModifiedClass`, etc.) so shared fields stay in one place.

## Adversarial fixtures — out of scope

These stay inline in the test file with a comment:

- Nil pointers on fields the spec marks required (the demo would crash or render "(nil)").
- Malformed/unparseable AWS responses (bad ARN format, empty identifier).
- API error paths (timeouts, throttling, permission denied). These are test-only.
- Resources the spec explicitly marks Wave 3 / out-of-scope. The demo should not display what the spec says not to display.

If the impl-plan §2 includes any of these, leave them as inline constructions in the QA test file and note the exclusion in §3 of the plan's "Demo fixture coverage" subsection (created below).

## Phases

### Phase 0 — Intake

Confirm `docs/resources/<shortName>-impl-plan.md` §2 exists. Confirm `docs/resources/<shortName>.md` §2 (related targets) is readable.

List the contents of `internal/demo/fixtures/` to see which sibling files exist. Note any missing peers the related panel needs.

### Phase 1 — Build the coverage matrix

From the impl-plan §2, list each fixture with:

- Its purpose (which test / which demo state).
- The state bucket it represents (Healthy / Warning / Broken / Transitional).
- Whether it is adversarial (excluded from this skill's scope).

From the spec §2, list every related-panel target for this resource. For each, read the corresponding checker in `internal/aws/<shortName>_related.go` and record the exact AWS field the checker reads (e.g. `DBInstance.VpcSecurityGroups[].VpcSecurityGroupId` for `sg`).

### Phase 2 — Graph plan (cross-file references)

For each non-adversarial fixture, walk its related-panel targets. For each target:

- Read the sibling `internal/demo/fixtures/<target>.go`.
- Check whether the ID/ARN the fixture references exists in the sibling's `New<Target>Fixtures()` output.
- If not, plan a sibling update: append one realistic entry that matches this fixture's reference.

Example for a dbi fixture with `VpcSecurityGroups=[{VpcSecurityGroupId="sg-rds-prod"}]`:

- Read `internal/demo/fixtures/ec2.go` (or wherever SGs live).
- If `sg-rds-prod` is absent, plan to append `SecurityGroup{GroupId:"sg-rds-prod", ...}` with a matching VPC reference.

Do this for **every** pivot in the spec §2, not just the ones the checker currently passes. Missing a pivot = demo renders 0 = silent breakage.

Record the plan in a new §"Demo fixture coverage" subsection appended to `docs/resources/<shortName>-impl-plan.md`, listing:

- One line per fixture: state, related-panel IDs it carries, sibling updates required.
- Per-pivot table: target type → sibling file → existing-or-new ID → expected count.

### Phase 3 — Write the fixture file

Write or rewrite `internal/demo/fixtures/<shortName>.go`:

- Package `fixtures`, exported struct `<Service>Fixtures`, constructor `New<Service>Fixtures()`.
- Exported `const` block for every ID/ARN this fixture defines (for sibling files to reference by symbol).
- Baseline constructor + mutator helpers per the impl-plan §2 grouping.
- One entry per non-adversarial fixture in the coverage matrix.
- NO adversarial fixtures (they belong inline in tests).
- Fold any existing `internal/demo/fixtures/<shortName>_fixtures.go` into this file and delete the old file.

### Phase 4 — Sibling file updates

Apply the phase-2 plan. For each sibling file:

- Append the new entries to the existing `New<Target>Fixtures()` constructor.
- Reference IDs by the `const` symbols exported from this shortName's fixture file (import-sibling pattern — yes, fixture files may depend on each other; this is intentional, it is how the graph connects).
- Alarms: add MetricAlarm entries with `Dimensions=[{Name=<field>, Value=<fixture const>}]` matching this resource type's CloudWatch dimension name.
- CloudTrail events: add LookupEvents entries with `Resources=[{ResourceName=<fixture const>}]` for the write APIs the spec §6 calls out (e.g. `ModifyDBInstance`, `RebootDBInstance`).
- Parent / child / peer graphs: if the resource has a parent (e.g. dbc for aurora members), add a parent fixture that references this one via its `Members[]` or equivalent field.

### Phase 5 — Handler routing check

If the service already relies on typed fakes (the common case), no `handlers.go` change is needed — `registerAllHandlers` only wires STS.

If the service needs a new route (rare — only when the SDK path doesn't use the typed-fake transport), add the handler to `internal/demo/handlers.go` and register it from `registerAllHandlers`. Document the reason in a comment.

### Phase 6 — Sanity check (cheap, local)

This skill does not run the final visual render gate — that is phase 8 of `a9s-implement-resource`, which uses the scripted scenario harness against the rendered list view. Duplicating that gate here would be wasteful.

Phase 6 is a cheap local sanity check so the skill doesn't hand off a broken fixture graph:

1. `make build` — the fixture file and every sibling update must compile.
2. `go test -count=1 ./internal/demo/...` — the demo package's own tests (fixture loaders, handler shape) pass.
3. Self-audit — print the graph plan from phase 2 with each pivot's expected count vs the sibling file's actual entries. If any pivot's sibling fixture is missing or has the wrong ID, loop back to phase 4 and fix.

The authoritative gate (rendered output matches spec §4 + §7 + §8) runs in `a9s-implement-resource` phase 8 via `tests/integration/scenario_<shortName>_visual_test.go`. This skill is only responsible for making sure the data this test will see is coherent.

### Phase 7 — Report

Emit one block:

```text
<shortName> fixtures: <N> written (<K> non-adversarial in demo, <A> adversarial left inline in tests).
Sibling updates: <alarm:N, ct-events:N, kms:N, sg:N, subnet:N, vpc:N, ...>.
Demo render verified: resource list rows=<N>, related-panel pivots non-zero=<M>/<total>.
File: internal/demo/fixtures/<shortName>.go
```

## What this skill never does

- Does not write under `tests/` — QA owns tests.
- Does not edit the fetcher, related checker, or enricher. Those are the outer skill's phase-7 scope.
- Does not invent states not in the spec. One fixture per spec §3/§4 row; if the spec doesn't describe the state, the fixture doesn't exist.
- Does not duplicate an existing sibling entry. If `vpc-abc` already exists in `vpc.go`, the new dbi fixture references it by the sibling's const; it does not add a second VPC with the same ID.
- Does not commit, push, or open a PR. That is the user's call after verification.
