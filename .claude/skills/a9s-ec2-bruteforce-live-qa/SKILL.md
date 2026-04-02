---
name: a9s-ec2-bruteforce-live-qa
description: Exhaustive live QA workflow for a9s EC2 detail views using a real AWS profile. Walk every EC2 instance, every related-resource row, every navigable field, and compare UI behavior against AWS CLI.
disable-model-invocation: true
---

# a9s EC2 Bruteforce Live QA

Use this skill when the goal is to aggressively brute-force the EC2 experience in `a9s` with real AWS data.

This is not a sampling workflow.

It must iterate through:
- every EC2 instance visible in the target region
- every navigable EC2 field
- every EC2 related-resource row
- every reachable related destination
- every keybinding that is relevant in each EC2 view/column/state

## Non-Negotiable Rules

1. Do **not** trust existing tests as proof of correctness.
2. Do **not** read implementation first to define expected behavior.
3. Do **not** sample one or two instances and generalize.
4. Do **not** stop after the first confirmed bug.
5. Treat blank rows, missing counts, empty destination lists, cache-warmed correctness, and misleading help text as suspect.

## Source-of-Truth Order

Use this precedence:

1. Product/design/QA docs
2. User-visible help text and UI affordances
3. Live app behavior
4. AWS CLI truth
5. Implementation code only after a discrepancy is proven
6. Existing tests only to understand missing coverage

## Required Environment

- Repo root: `/Users/k2m30/projects/a9s`
- AWS profile: `test-profile`
- Region: `eu-west-2`

Launch:

```bash
go run ./cmd/a9s --profile test-profile --region eu-west-2
```

## Phase 1: Read Expectations First

Read these before live testing:

- `AGENTS.md`
- `docs/design/qa-user-stories-related-views-ec2.md`
- `docs/design/related-resources.md`
- `docs/design/child-views`
- `docs/design/resource-to-cloudtrail-preview`
- any EC2-specific QA or design docs directly relevant to related views and child views

Then build an expectation matrix with:

- feature
- documented expected behavior
- source path
- app surface where it should appear
- AWS CLI command needed to verify it

If docs are ambiguous, record that explicitly. Do not fill the gap from code.

## Phase 2: Build the EC2 Inventory

Get the full live EC2 inventory first.

Required command shape:

```bash
aws ec2 describe-instances --profile test-profile --region eu-west-2 ...
```

For each instance capture at minimum:
- InstanceId
- Name tag
- State
- InstanceType
- VpcId
- SubnetId
- ImageId
- SecurityGroups
- attached VolumeIds
- CloudFormation tags if present
- EKS nodegroup tag if present

Create a working table for **every** instance in the region.

No skipping terminated or unusual entries unless docs explicitly exclude them.

## Phase 3: Bruteforce Every Instance

For each EC2 instance:

1. Open its EC2 detail screen in the app.
2. Record the visible detail title and primary fields.
3. Test every navigable field that appears and is supposed to be actionable.
4. Test every related-resource row that appears in the right column.
5. Compare every visible related count against AWS CLI.
6. Compare every destination view against the exact resource(s) expected from AWS CLI.

### Navigable fields

At minimum challenge:
- `VpcId`
- `SubnetId`
- `ImageId`
- every `SecurityGroups.GroupId`

For each navigable field:
- press `Enter`
- verify the destination type is correct
- verify the destination resource is correct
- verify `Esc` returns properly
- verify cache-miss behavior is not misleading or broken

### Related rows

At minimum challenge all rows registered or shown for EC2, including:
- Target Groups
- Auto Scaling Groups
- CloudWatch Alarms
- EKS Node Groups
- CloudFormation Stacks
- Elastic Beanstalk
- Elastic IPs
- EBS Snapshots
- CloudTrail Events

For each row on each instance:
- note whether the row is blank, loading, counted, zero-count, or actionable
- verify whether the count is correct according to AWS CLI
- press `Enter` if the row appears actionable
- confirm destination count and destination resources
- record any mismatch, dead-end, or placeholder behavior

## Phase 4: Bruteforce Keys In Every EC2 State

For each applicable state, press every relevant keybinding and record behavior.

States to cover:
- EC2 list
- EC2 detail left column focused
- EC2 detail right column focused
- EC2 detail right-column filter active
- related destination list
- related destination detail

At minimum challenge:
- `j`, `k`
- `g`, `G`
- `Tab`
- `h`, `l`
- `r`
- `/`
- `Enter`
- `Esc`
- `Ctrl+R`
- `y`
- `?`
- `n`, `N`
- `w`
- `c`
- page movement keys if the view is long enough

If a key is advertised in help, it must be tested.
If a key appears to do nothing, verify whether that is expected, broken, or only conditionally available.

## Phase 5: AWS CLI Verification Rules

Every live claim must be backed by AWS CLI.

Suggested commands:

```bash
aws sts get-caller-identity --profile test-profile
aws configure get region --profile test-profile
aws ec2 describe-instances --profile test-profile --region eu-west-2 ...
aws autoscaling describe-auto-scaling-instances --profile test-profile --region eu-west-2 ...
aws autoscaling describe-auto-scaling-groups --profile test-profile --region eu-west-2 ...
aws elbv2 describe-target-groups --profile test-profile --region eu-west-2 ...
aws ec2 describe-addresses --profile test-profile --region eu-west-2 ...
aws cloudtrail lookup-events --profile test-profile --region eu-west-2 ...
aws cloudwatch describe-alarms --profile test-profile --region eu-west-2 ...
aws eks list-nodegroups --profile test-profile --region eu-west-2 ...
aws ec2 describe-images --profile test-profile --region eu-west-2 ...
aws ec2 describe-volumes --profile test-profile --region eu-west-2 ...
aws ec2 describe-snapshots --profile test-profile --region eu-west-2 ...
aws cloudformation describe-stacks --profile test-profile --region eu-west-2 ...
```

Rules:
- do not invent inferred relationships when CLI can answer directly
- if the app uses an approximation, record both:
  - actual AWS truth
  - app approximation
- if correctness depends on warming a cache by visiting another screen first, classify that as a bug unless docs explicitly permit it

## Phase 6: Only Then Read Code

After a discrepancy is proven, inspect implementation to explain it.

Typical files:
- `internal/aws/ec2.go`
- `internal/aws/ami.go`
- `internal/tui/app_handlers.go`
- `internal/tui/views/detail.go`
- `internal/tui/views/rightcolumn.go`
- `internal/tui/views/help.go`

Use code only for:
- likely cause
- where to add tests
- where to fix behavior

Do not redefine expected behavior from code after the fact.

## Phase 7: Reveal Bugs With Tests

For each confirmed issue:
- add a focused failing or regression test
- append to an existing nearby test file where possible
- prefer a small exact test over a broad imitation of the manual flow

Strong brute-force bug-test categories:
- related counts that return unknown without cache warm
- nil/placeholder related rows that should be actionable
- exact-ID navigation landing in empty lists
- help-advertised keys not working
- bad back-navigation
- inconsistent focus behavior
- wrong destination counts after related navigation

## Required Outputs

### 1. Bruteforce QA log

Write a markdown log under `docs/qa/` containing:
- date
- profile and region
- expectation matrix
- EC2 inventory table
- exact app launch command
- exact AWS CLI commands used
- exact key sequences used
- per-instance findings
- per-related-row findings
- expected vs actual behavior
- documented source references
- confirmed bugs
- remaining ambiguous items

### 2. Coverage summary

Include a matrix with:
- instance id
- navigable fields checked
- related rows checked
- key states checked
- bugs found

### 3. Tests

Add or update tests for confirmed bugs.

### 4. Final summary

Summarize:
- how many EC2 instances were checked
- how many related rows were checked
- how many destination navigations were verified
- bugs found
- tests added
- what remains unverified

## Classification Rules

Every discrepancy must be labeled as one of:
- bug against documented behavior
- bug against visible UI/help affordance
- implementation gap
- test gap
- ambiguous spec

Do not blend them together.

## Success Criteria

This skill is successful only if:
- every EC2 instance in the region was processed
- every EC2 related row was challenged on every applicable instance
- every EC2 navigable field was challenged
- every relevant keybinding in each EC2 view state was exercised
- at least one real bug is confirmed with live evidence
- at least one bug is captured in tests
- the QA log is strong enough for another engineer to replay the brute-force pass
