# Review: Current Product vs Incident Cockpit Requirements

Related issue: [#196](https://github.com/k2m30/a9s/issues/196)

## Scope

This review compares the current `a9s` product against
[`docs/aws-incident-cockpit-spec.md`](~/projects/a9s/docs/aws-incident-cockpit-spec.md).

The comparison is based on the current repository state as of 2026-04-01,
primarily:

- [`README.md`](~/projects/a9s/README.md)
- [`ROADMAP.md`](~/projects/a9s/ROADMAP.md)
- [`internal/tui/keys/keys.go`](~/projects/a9s/internal/tui/keys/keys.go)
- [`internal/tui/app_input.go`](~/projects/a9s/internal/tui/app_input.go)
- [`internal/tui/views/detail.go`](~/projects/a9s/internal/tui/views/detail.go)
- design specs under [`docs/design/`](~/projects/a9s/docs/design)

## Executive Summary

`a9s` is already strong as a read-only AWS resource exploration TUI. It is weak
as an incident cockpit.

Current strength:

- broad AWS resource coverage
- safe read-only posture
- fast keyboard-driven navigation
- list, detail, YAML, and child-view drill-down patterns
- profile and region switching
- targeted operational subviews like ECS logs, target health, alarm history,
  CloudTrail events, and RDS events

Current gap:

- no incident workspace
- no unified cross-service timeline
- no dependency graph
- no first-class change-correlation workflow
- no symptom-first investigation flow
- no cross-account incident scope model
- no diff-centric debugging workflow

Net: `a9s` is a good AWS explorer. It is not yet the tool described in the
incident-cockpit spec.

## Scorecard

| Requirement | Status | Notes |
|---|---|---|
| Read-only by default | Strong | Explicitly implemented and documented |
| Keyboard-first TUI | Strong | Core interaction model |
| Broad AWS coverage | Strong | 66 resource types |
| Raw resource inspection | Strong | Detail and YAML views are mature |
| Resource-specific child views | Strong | Good foundation for operational drills |
| Incident workspace | Missing | No session or incident object exists |
| Unified timeline | Missing | Events remain service-specific |
| Dependency graph | Missing | Relationship system is still roadmap/design work |
| Change correlation | Weak | CloudTrail exists, but no correlated change workflow |
| Symptom-first debugging | Missing | Navigation starts from resource types, not incidents |
| Cross-account scope sets | Weak | Profile switching exists; scoped multi-account search does not |
| Cross-region incident view | Weak | Region switch exists; multi-region merged view does not |
| Multi-source logs as one stream | Missing | Logs are child views, not unified streams |
| Trace integration | Missing | No X-Ray or tracing workflow |
| Human-readable diffs | Missing | No before/after diff workflows |
| Suggested next actions | Missing | No evidence-backed recommendation engine |
| Safe remediation mode | Missing | Mutations are intentionally absent |
| Incident export / bundle | Missing | No session export model |

## What a9s Already Does Well

### 1. Safe operational posture

This is already aligned with the target product philosophy.

- README explicitly states the product is read-only.
- The roadmap treats write operations as future gated work.
- This is the correct default for a production incident tool.

### 2. Fast exploration of AWS state

This is the product's strongest current capability.

- main menu plus resource-type lists
- vim-style movement
- filtering and sorting
- YAML inspection
- direct child-view drill-downs

For ad hoc inspection of one resource type at a time, `a9s` is already useful.

### 3. Service-specific operational subviews

This is the best foundation for evolving toward an incident cockpit.

Examples already present in the shipped product:

- ECS service events and container logs
- Lambda invocations and invocation logs
- CloudWatch alarm history
- target group health
- RDS events
- CodeBuild build logs
- CloudTrail events as a resource type

These are good building blocks, but they are not yet composed into an incident
workflow.

### 4. Explicit profile and region controls

Profile and region switching already exist via command mode. That matters for
real operators and is consistent with the target design. The gap is that the
model is still single-profile and single-region at a time.

## Major Gaps

### 1. No incident object or workspace

This is the largest gap.

Current navigation starts from resource categories and resource lists. There is
no way to start from:

- service name
- alarm
- 5xx symptom
- latency spike
- queue backlog
- time window

Without an incident workspace, `a9s` cannot preserve context, timeline, pinned
evidence, or investigation state the way an incident tool should.

### 2. No unified timeline

`a9s` has multiple event-oriented views, but they are isolated:

- ECS service events
- CloudWatch alarm history
- CloudTrail events
- CloudFormation events
- RDS events

There is no merged, time-sorted stream that correlates these together. This
makes causal reconstruction much harder than it needs to be.

### 3. No dependency graph or relationship navigation in the shipped product

The roadmap mentions resource relationships, and the repository contains large
design and QA work for related-resource navigation and resource-to-CloudTrail
pivots. But the current keymap does not bind those flows, and the current root
interaction model is still centered on standard list/detail/child views.

This means the product cannot yet answer:

- what depends on this resource
- what does this resource depend on
- what is the blast radius

That is a hard blocker for incident use.

### 4. CloudTrail is present, but change correlation is still weak

This is an important nuance.

`a9s` does expose CloudTrail data, which is useful. But current usage is still
resource browsing, not change investigation. There is no first-class query like:

- what changed before the outage
- show all write events touching this service and its dependencies
- correlate deploy, IAM, DNS, and target-health changes in one window

CloudTrail data alone is not enough without correlation workflow.

### 5. No symptom-first flow

The ideal tool starts from the symptom. `a9s` starts from the AWS service menu.

That makes it good for exploration but slower for incidents, because operators
must already guess the relevant AWS surface area before the tool helps them.

### 6. No diff workflow

The spec depends heavily on human-readable before/after diffs. `a9s` currently
offers raw detail and YAML inspection, but not comparisons between:

- current vs previous task definition
- before vs after security group
- previous vs current Lambda configuration
- previous vs current IAM policy

This is a major gap because many incidents are introduced by small config
changes.

### 7. No unified log workflow

Current log access is useful but fragmented.

- ECS logs are a child view
- Lambda invocation logs are a child view
- CloudWatch logs are navigated per log group or stream

What is missing:

- merged multi-log tail
- cross-service correlation IDs
- symptom to log pivoting
- saved investigation queries

### 8. No tracing workflow

The target tool needs trace-driven root cause analysis that starts from the
symptom. There is no X-Ray or equivalent trace workflow in the current product.

### 9. No cross-account incident scope

`a9s` supports switching profile and region, which is useful operationally.
But an incident cockpit needs multi-scope investigation, where one query spans:

- prod account
- shared services
- platform account
- multiple regions

That model does not exist yet.

### 10. No guided next actions

The current product presents data. It does not yet help the operator prioritize
the next investigation step based on the evidence already gathered.

## Design Work Already Moving In The Right Direction

The repo contains meaningful design work that supports the direction of the
incident-cockpit spec.

Promising areas:

- related resources architecture
- resource-to-CloudTrail navigation
- cross-view search work
- many resource-specific child views

This means the product is not starting from zero. The issue is composition and
operator workflow, not lack of AWS primitives.

## What To Keep

- Read-only default.
- Keyboard-first navigation.
- Resource-type breadth.
- Detail and YAML views.
- Existing child-view fetcher model.
- Profile and region switching.
- Strong test discipline.

These should remain core primitives even if the product grows more
incident-oriented.

## What To Add Next

### Priority 1

- incident workspace entry point
- unified timeline across alarms, deploys, CloudTrail, and service events
- shipped related-resource pivots
- resource-to-CloudTrail pivot
- service-focused investigation pages for ECS, ALB, Lambda, RDS, and SQS

### Priority 2

- change-correlation queries
- human-readable diff engine
- merged multi-log investigations
- cross-account and cross-region scoped search

### Priority 3

- tracing integration
- evidence-backed next-step suggestions
- incident bundle export
- explicit safe remediation mode

## Bottom Line

`a9s` already solves "browse and inspect AWS resources from the terminal" well.
It does not yet solve "run an incident from the terminal" well.

The shortest path from current product to the incident-cockpit target is not to
add more resource types. It is to compose the existing resource, log, event, and
CloudTrail primitives into a single symptom-first investigation workflow.
