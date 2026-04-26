# AWS Incident Cockpit TUI

Related issue: [#196](https://github.com/k2m30/a9s/issues/196)

## Purpose

This document defines the ideal terminal-first AWS debugging, tracing, and
incident response tool for operators who are tired of using the AWS Console as
their primary debugger and tired of stitching together long AWS CLI commands by
hand.

The tool is not a generic dashboard. It is an operator cockpit for answering:

- What is broken?
- Since when?
- What changed?
- What is the blast radius?
- Which dependency failed first?
- What is the safest next action?

## Product Position

The ideal tool sits above `aws cli`, `kubectl`, `terraform`, CloudWatch,
CloudTrail, X-Ray, ECS, EKS, Lambda, Route53, ELB, RDS, SQS, SNS, IAM, and VPC
diagnostics.

It must:

- stay terminal-native
- be keyboard-first
- preserve AWS concepts instead of hiding them
- remain read-only by default
- expose the underlying API/CLI operations behind every view

## Design Principles

- Resource graph first, service silos second.
- Time-correlated evidence beats isolated widgets.
- Read-only by default, explicit escalation for mutations.
- Symptom-first workflows, not service-first workflows.
- Every conclusion must be backed by inspectable evidence.
- Cross-account and cross-region behavior must be first-class.
- The tool must be usable by tired humans during real incidents.

## Core Capabilities

### 1. Incident Workspace

Example:

```sh
awx incident start --service payments-api --since 30m
```

This opens a single incident workspace populated with:

- current alarms
- recent deploys
- service health
- top errors
- upstream and downstream dependencies
- recent infrastructure changes
- a session timeline with pinned evidence

### 2. Unified Timeline

The primary screen is a time-correlated stream across:

- CloudWatch alarms
- deploy events
- autoscaling events
- CloudTrail changes
- ECS and EKS restarts
- Lambda errors and throttles
- RDS failovers and events
- target health changes
- Route53 changes

This is the most important capability. Infra debugging is usually timeline
reconstruction.

### 3. Dependency Graph

The tool must render service relationships such as:

- ALB to target group to ECS service
- queue to consumer
- service to database or cache
- Lambda to trigger to DLQ
- service to VPC and networking edges

It should highlight the most likely first failing edge, not just mark resources
as unhealthy.

### 4. Change Correlation

The tool must answer:

> Show me everything that changed 20 minutes before the error spike.

Correlated changes include:

- deploys
- task definition revisions
- launch template changes
- IAM policy changes
- security group and NACL changes
- route table changes
- parameter and secret changes
- DNS changes
- feature flag changes when integrated

### 5. Cross-Account and Cross-Region Scope

Operators should be able to search once and fan out through configured roles.

Example:

```text
:set scope prod-eu,prod-us,shared-services
```

The active scope must always be obvious.

### 6. Smart Log Querying

The tool must support:

- tailing multiple log groups as one stream
- auto-detecting correlation IDs, request IDs, and trace IDs
- pivoting from ALB 5xx, Lambda request ID, ECS task ID, pod name, or trace ID
- saving reusable log query recipes per service

### 7. Trace Workflows That Start From Symptoms

Tracing must start from symptoms such as:

- 502 from an ALB
- rising p95 latency
- growing SQS age
- pod crash loops

The tool should derive candidate traces and spans rather than forcing the user
to begin inside X-Ray.

### 8. Opinionated Resource Drills

Each major AWS resource type should have a best-practice debug page.

Examples:

- ECS Service: desired vs running, failed deployments, task stop reasons,
  target health, env or secret diff, CPU or memory throttling
- Lambda: concurrency, throttles, cold starts, DLQ, last deploy diff
- RDS: connections, locks, failover history, storage pressure, replication lag,
  recent parameter changes
- VPC pathing: SG, NACL, route checks, flow logs, reachability shortcuts

### 9. Suggested Next Actions

Suggestions must be concrete and evidence-based, for example:

- Targets unhealthy after deploy; compare old and new health-check settings.
- Errors began 3 minutes after task-role IAM update.
- Queue age rising while consumers are idle; inspect visibility timeout and
  handler failures.

No unsupported AI summaries.

### 10. Runbook Integration

The tool should render service-specific runbooks inline and attach safe commands
for common responses such as rollback or restart.

### 11. Safe Remediation Mode

Mutating actions must be separate from normal exploration and require explicit
intent. Before any write operation, the tool should show:

- exact command
- target account and region
- expected blast radius
- approval guard

### 12. Human-Readable Diff Everywhere

The tool should diff:

- task definitions
- Lambda configuration
- environment variables
- IAM policies
- security groups
- listener rules
- Terraform outputs or state-derived snapshots when available

Raw JSON is not enough.

## Ideal Operator Flows

### API Is Returning 502

1. Open service workspace.
2. View ALB health, deploys, ECS task failures, and relevant logs together.
3. Compare current task definition to the previous known-good revision.
4. Inspect env, secret, IAM, and target-health changes.
5. Offer rollback or focused next checks.

### Latency Spike

1. Start from p95 alarm.
2. Rank top endpoints and slow downstream dependencies.
3. Correlate traces, DB pressure, cache behavior, and retries.
4. Identify the likely first bottleneck.

### Queue Backlog Growing

1. Open queue view with visible, in-flight, DLQ, and oldest-age metrics.
2. Jump directly to consumers.
3. Compare producer surge vs consumer degradation timing.

### Something Changed and Nobody Knows What

1. Select time window and scope.
2. Rank likely suspect changes.
3. Jump from each suspect change to impacted resources, logs, and events.

## TUI UX Requirements

- Vim-like keyboard navigation.
- Split-pane layout optimized for fast pivots.
- Command palette for scoped operations.
- Breadcrumbs and context preservation across pivots.
- Strong time-range controls.
- Session bookmarks and pinned evidence.
- Exportable incident bundle for handoff and postmortem.

Suggested commands:

```text
:incident start payments-api since 30m
:logs service/auth since 15m
:changes prod since 1h
:trace req-123
:why unhealthy tg/payments
```

## How It Works Internally

The ideal product should not rely on a vague global intelligence layer. It
should be built from explicit, inspectable parts:

- target templates
- constrained scope expansion
- typed evidence providers
- normalized event records
- heuristic suspect rules
- simple ranking

Conceptually:

```text
target -> template -> scope -> evidence -> normalize -> correlate -> rank
```

This means the cockpit is mostly template-driven, but not rigidly scripted.
Predefined parts choose what to inspect; dynamic evidence determines what
actually appears in the incident workspace.

Examples of target templates:

- ECS service
- target group / load balancer
- Lambda function
- CloudWatch alarm
- SQS queue

Each template defines:

- relevant related resources
- evidence providers to run
- heuristics to evaluate
- default evidence pivots to show in the workspace

The important design choice is to model resource families and reusable evidence
patterns, not to try to encode every possible outage scenario as a separate
"playbook template."

Recommended scale:

- v1 should cover only a small set of high-frequency target families
- mature coverage should likely remain in the low tens of templates
- if the system requires a template per AWS resource type, the model is wrong

The product should scale by composition, not by endlessly adding bespoke
incident templates.

## Product Economics

This vision is only worth building if it is approached as a narrow,
high-leverage workflow, not as a full AWS incident platform.

Worth building when:

- a few target families cover a large share of real incidents
- the workflow materially reduces investigation time
- the cockpit reuses existing primitives instead of replacing them

Not worth building when:

- broad service coverage is required before value appears
- the system depends on universal infra understanding
- support burden grows faster than operator value

The right strategy is to prove the concept with a narrow MVP and stop if that
MVP does not clearly outperform the existing manual workflow.

## Do

- Optimize for incident reconstruction.
- Make account, profile, region, and scope explicit.
- Keep raw AWS concepts visible.
- Show the exact query or command behind every view.
- Cache aggressively, but always show freshness.
- Encode AWS operator knowledge into the workflows.
- Preserve evidence and export sessions.

## Do Not

- Do not become a generic observability dashboard clone.
- Do not hide AWS behind fake abstractions.
- Do not default to mutating actions.
- Do not split logs, metrics, events, and changes into unrelated screens.
- Do not depend on mouse-driven workflows.
- Do not replace evidence with hand-wavy summaries.
- Do not optimize for demos over operator speed.

## MVP

The minimum version that would already be useful in production should include:

- read-only incident workspace
- unified timeline
- CloudTrail and deploy correlation
- cross-resource pivots
- multi-log querying
- service-specific drills for ECS, Lambda, ALB, RDS, and SQS
- incident export

## Definition of Success

The tool is successful when it consistently helps an operator:

- start from a symptom
- narrow the suspect set quickly
- prove or kill hypotheses in minutes
- reconstruct a reliable timeline
- identify the safest next action without leaving the terminal
