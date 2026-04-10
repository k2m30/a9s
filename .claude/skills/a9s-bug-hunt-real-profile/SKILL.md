---
name: a9s-bug-hunt-real-profile
description: Bug-hunting workflow for a9s using real AWS data, grounded in docs first, then live behavior, then code. Use for EC2-detail and related-view QA where existing tests may be misleading.
disable-model-invocation: true
---

# a9s Real-Profile Bug Hunt

Use this skill when the goal is to find real user-visible bugs in `a9s` by comparing documented behavior against the live app and actual AWS CLI data.

## Core Rule

Do **not** read implementation files first to decide what correct behavior should be.

That creates bias and normalizes broken behavior.

Use this source-of-truth order:

1. Product/design/QA docs
2. User-visible help text and UI affordances
3. Live app behavior
4. AWS CLI ground truth
5. Implementation code only after a mismatch is found
6. Existing tests last, only to measure coverage gaps

## Scope

Primary focus:
- first-screen journeys
- EC2 list/detail flows
- related-resource behavior
- navigable field behavior
- keybindings
- layout behavior
- live count accuracy
- cache-dependent or misleading UI

## Required Inputs

- Repo root: `~/projects/a9s`
- AWS profile: **ask the user** for their profile name before starting
- Region: **ask the user** for their target region before starting

App launch (substitute `{PROFILE}` and `{REGION}` with user-provided values):

```bash
go run ./cmd/a9s --profile {PROFILE} --region {REGION}
```

## Phase 1: Build Expectations From Docs Only

Read these first:

- `AGENTS.md`
- `docs/design/qa-user-stories-related-views-ec2.md`
- `docs/design/related-resources.md`
- `docs/design/child-views`
- `docs/design/resource-to-cloudtrail-preview`
- any other directly relevant design or QA docs for the flow under test

Also inspect user-visible help behavior by checking:
- `?` help overlay in the app
- only if needed later, `internal/tui/views/help.go` for confirmation of displayed bindings

Before touching implementation, build an expectation matrix with:

- feature or flow
- expected behavior
- source document
- how to verify it live
- AWS CLI command if applicable

If docs are ambiguous, write that down explicitly. Do not silently fill gaps from code.

## Phase 2: Run Live QA

Launch the app with the real profile and test like an impatient user.

Start with:
1. first screen
2. EC2 list
3. EC2 detail
4. right-column related views
5. navigable fields
6. destination detail/list behavior

For each screen, challenge:
- counts
- labels
- keybinds
- focus behavior
- enter behavior
- escape behavior
- filter behavior
- refresh behavior
- wide vs stacked layout behavior
- whether rows that look actionable are actually actionable

Treat these as suspicious until proven:
- blank related rows
- non-zero counts that appear only after warming cache
- rows shown in help but not actually working
- rows shown in UI that never resolve to a destination
- exact-ID navigation that drops into empty lists

## Phase 3: Verify With AWS CLI

Use AWS CLI to validate every claim that depends on real data.

Examples:
```bash
aws sts get-caller-identity --profile {PROFILE}
aws configure get region --profile {PROFILE}
aws ec2 describe-instances --profile {PROFILE} --region {REGION} ...
aws autoscaling describe-auto-scaling-instances --profile {PROFILE} --region {REGION} ...
aws elbv2 describe-target-groups --profile {PROFILE} --region {REGION} ...
aws ec2 describe-addresses --profile {PROFILE} --region {REGION} ...
aws cloudtrail lookup-events --profile {PROFILE} --region {REGION} ...
aws cloudwatch describe-alarms --profile {PROFILE} --region {REGION} ...
aws eks list-nodegroups --profile {PROFILE} --region {REGION} ...
aws ec2 describe-images --profile {PROFILE} --region {REGION} ...
```

Rules:
- never invent AWS state
- compare UI numbers to CLI numbers
- if app logic intentionally uses an approximation, note both:
  - true AWS result
  - app’s implemented approximation
- if a UI count is only correct after preloading another resource list, treat that as a bug unless docs explicitly allow it

## Phase 4: Only Now Read Implementation

After finding a mismatch, inspect relevant code to explain it.

Typical files:
- `internal/aws/ec2.go`
- `internal/aws/ami.go`
- `internal/tui/app_handlers.go`
- `internal/tui/views/detail.go`
- `internal/tui/views/rightcolumn.go`
- `internal/tui/views/help.go`

Use implementation only for:
- likely root cause
- identifying the right test file
- identifying the right fix location

Do not retroactively redefine expected behavior from code.

## Phase 5: Add Bug-Revealing Tests

When a real bug is confirmed:
- add focused tests that fail for the current behavior
- use existing nearby test files where possible
- prefer small, sharp regression tests over giant scenario bundles

Good test targets:
- cache-dependent related counts
- nil or placeholder related checkers
- broken exact-ID navigation
- help-advertised keybinds that do not work
- incorrect escape/back behavior
- misleading empty-state flows

## Required Deliverables

### 1. QA log file

Write a markdown log under `docs/qa/` with:
- date
- profile and region
- expectation matrix
- app launch command
- exact AWS CLI commands used
- exact key sequences used
- expected vs actual behavior
- source references for expectations
- confirmed bugs
- follow-up recommendations

### 2. Tests

Add or update focused tests revealing the confirmed bug.

### 3. Final summary

Summarize:
- documented expectations checked
- bugs confirmed
- tests added
- commands run
- what remains unverified

## Classification Rules

Every discrepancy must be labeled as one of:
- bug against documented behavior
- bug against visible help/UI affordance
- implementation gap
- test gap
- ambiguous spec

Do not blur these together.

## Success Criteria

The run is successful only if:
- it is grounded in docs first
- at least one real bug is confirmed with live evidence
- at least one bug is captured in a reproducible test
- the QA log is strong enough for another engineer to replay without guessing
- the final output clearly separates:
  - expected behavior
  - actual behavior
  - likely root cause
  - current test-suite blind spots
