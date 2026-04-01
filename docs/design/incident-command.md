# Incident Command / Cockpit Design Spec

Issue: proposed
Version: 1.0
Target: a9s vNext
Status: Design

---

## 1. Goal

Add a new colon command, `:incident`, that opens a focused incident cockpit for
debugging AWS outages from symptoms instead of from service menus.

The purpose is not to replace the existing resource-browser model. The purpose
is to reduce operator pain during incidents:

- too many AWS Console tabs
- too many service-specific CLIs and flags
- too much manual timeline reconstruction
- too much jumping between logs, alarms, deploys, and CloudTrail

`a9s` should remain terminal-first, keyboard-first, and read-only by default.

---

## 2. TUI Reality Check

This design must respect terminal constraints.

What the cockpit should **not** try to do:

- not a literal node-edge graph renderer
- not a many-pane observability dashboard
- not a dense wall of tiny charts
- not a mouse-driven investigation surface
- not infinite live streams in every panel at once

What works well in a TUI:

- one primary focus area
- one or two supporting sidebars
- sortable event lists
- pinned evidence
- progressive drill-down
- explicit scope and time range
- clear command entry points

So the cockpit should be a **guided investigation workspace**, not a visual NOC dashboard.

---

## 3. Command Model

### 3.1 Entry Command

Primary command:

```text
:incident
```

This opens the Incident Start view.

Aliases:

```text
:inc
:debug
```

`debug` is acceptable because it matches operator intent better than "incident"
in some workflows, but `incident` should remain canonical.

### 3.2 Fast-Start Variants

Minimal syntax only. Do not design a shell parser inside command mode.

Supported forms:

```text
:incident
:incident <target>
:incident <target> <window>
```

Examples:

```text
:incident payments-api
:incident payments-api 30m
:incident tg/payments
:incident sqs/orders-dlq 4h
:incident alarm/high-5xx 1h
```

Rules:

- `<target>` is a single token, no quoted parser in v1
- `<window>` is optional and limited to `15m`, `30m`, `1h`, `4h`, `24h`
- if parsing fails, open the Incident Start view with the raw text prefilled

Do not attempt a Terraform-style CLI grammar in the header input.

---

## 4. Mental Model

` :incident ` opens a temporary **workspace**, not a resource type.

The workspace has 4 operator questions:

1. What is the symptom?
2. What changed?
3. What depends on it?
4. What evidence should I inspect next?

This is distinct from the normal list/detail flow.

---

## 5. View Structure

The cockpit uses a **single frame** with mode changes inside it, consistent with
existing a9s patterns.

Flow:

```text
[1] Incident Start
      |
      v
[2] Incident Overview
      |
      +--> [3] Timeline
      +--> [4] Suspects
      +--> [5] Evidence
      +--> [6] Resource Drill
```

No nested floating windows. No mini-dashboard grid.

### 5.1 Start View

Purpose: capture just enough context to start investigating.

Fields:

- Target
- Time window
- Investigation mode

Modes:

- Service issue
- Alarm fired
- Resource unhealthy
- "Something changed"

Wireframe:

```text
 a9s vX.Y.Z  prod:us-east-1                                            ? for help
┌──────────────────────────── incident ───────────────────────────────────────────┐
│                                                                                 │
│  START AN INCIDENT                                                              │
│                                                                                 │
│  Target:     payments-api__________________________________                     │
│  Window:     [15m] [30m] [ 1h ] [ 4h ] [24h]                                    │
│  Mode:       [service issue] [alarm] [resource] [something changed]             │
│                                                                                 │
│  Examples                                                                       │
│  payments-api       tg/payments       sqs/orders       alarm/high-5xx           │
│                                                                                 │
│  Enter: start incident     Esc: cancel                                          │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Why this shape:

- fast to complete
- easy to understand
- no parser complexity
- works in 80 columns

### 5.2 Overview

This is the home screen of the cockpit.

Layout at comfortable width:

- left: incident summary and suspects
- center: timeline
- right: evidence queue

Layout at narrow width:

- stacked sections in one scrollable view

Do not force a three-column layout under 120 columns.

Overview sections:

- Symptom summary
- Top suspects
- Correlated timeline
- Related resources
- Suggested next pivots

Wireframe, wide terminal:

```text
┌────────────────────── incident -- payments-api -- 30m ─────────────────────────┐
│ Summary               Timeline                              Evidence            │
│ 502s on ALB           12:03 deploy started                  [1] ECS svc logs    │
│ p95 +340%             12:05 new tasks healthy? no           [2] TG health       │
│ target 3/8 healthy    12:06 5xx alarm fired                 [3] CloudTrail      │
│                      >12:07 target failures rise            [4] task stops      │
│ Suspects              12:08 task restarts                   [5] prev revision    │
│ 1. bad deploy         12:09 IAM role updated                                     │
│ 2. health check diff  12:10 stop reason EssentialExited                           │
│ 3. secret/env change                                                           │
└─────────────────────────────────────────────────────────────────────────────────┘
```

The selected section is navigable with `Tab`.

### 5.3 Timeline View

This is the primary debugging surface.

It is a single merged list of incident-relevant events:

- alarms
- deploy events
- ECS or Lambda events
- target group health changes
- CloudTrail writes
- config or secret changes

Rows should have:

- time
- source
- event
- resource
- severity

This is a table, not a chart.

### 5.4 Suspects View

This is a ranked list, not an AI essay.

Each suspect row contains:

- suspect label
- why it is suspected
- supporting evidence count
- shortcut to inspect

Example:

```text
1. ECS deploy revision 128
   errors began 2m after rollout; 5 stopped tasks; previous revision healthy
```

### 5.5 Evidence View

This is a queue of useful pivots, not raw data.

Examples:

- ECS service logs
- target health
- task stop reasons
- recent CloudTrail writes
- last known-good task definition

Selecting evidence opens either:

- a specialized incident subview, or
- an existing a9s resource/detail/child view with incident context preserved

### 5.6 Resource Drill View

This is where the cockpit hands off to a focused operational screen.

Examples:

- ECS drill
- ALB/TG drill
- Lambda drill
- RDS drill
- SQS drill

This should reuse current a9s strengths instead of inventing a whole new UI model.

---

## 6. Navigation and Keys

The cockpit needs only a few new concepts.

### 6.1 Global Keys Inside Incident Mode

| Key | Action |
|---|---|
| `Tab` | Cycle focused section |
| `Enter` | Open selected item |
| `b` | Back to incident overview |
| `t` | Open full timeline |
| `u` | Change time window |
| `p` | Pin/unpin evidence item |
| `f` | Refine target/filter |
| `Esc` | Leave current subview or exit incident mode |

Notes:

- `t` is acceptable here because incident mode is its own context
- avoid uppercase-heavy bindings in the cockpit
- do not overload too many letters; incident mode should feel calmer than the normal browser

### 6.2 Header Right-Side States

Normal:

```text
incident: payments-api  30m  service issue
```

While refining:

```text
:incident payments-api 30m█
```

While filtering timeline:

```text
/deploy█
```

This keeps the header model consistent with existing a9s behavior.

---

## 7. Scope and Time

These are the most important controls in an incident TUI.

### 7.1 Time Window

In v1, support only fixed windows:

- 15m
- 30m
- 1h
- 4h
- 24h

No arbitrary datetime picker in v1. Terminal UX for custom timestamps is too slow
for the primary workflow.

### 7.2 Scope

Because the current product is single profile and single region, the cockpit
must be honest about that.

Header should always show:

```text
incident scope: prod:us-east-1
```

Future support can add:

```text
scope: prod+shared-services / us-east-1+eu-west-1
```

But v1 should not fake multi-account incident awareness if it does not exist.

---

## 8. Data Model for UX

The user should not have to think in AWS service APIs first.

The cockpit organizes data as:

- symptom
- event
- suspect
- evidence
- related resource

This is a UX model, not an internal code model.

### 8.1 Symptom Types

Support only a small set in v1:

- service degraded
- alarm fired
- resource unhealthy
- unknown change

This is enough to reduce operator decision fatigue.

### 8.2 Evidence Types

Evidence rows can point to:

- existing child views
- existing resource lists filtered to a target
- CloudTrail search
- new incident-specific timeline rows

The cockpit should feel like a coordinator over existing views.

---

## 9. How Overview Is Filled

The overview is not hand-written per incident and it is not an open-ended AI
inference engine. It should be assembled by a small, explicit pipeline:

```text
target -> template -> scoped expansion -> evidence fetch -> normalize
       -> heuristic rules -> ranked suspects -> overview sections
```

This keeps the system explainable and realistic for a TUI.

### 9.1 Overview Fill Strategy

The cockpit should populate the overview in phases.

Phase 1:

- resolve the target
- identify the target template
- render summary skeleton

Phase 2:

- fetch the highest-signal evidence providers
- merge incident events into a timeline

Phase 3:

- run heuristic rules
- rank suspects
- populate evidence queue and next pivots

This avoids a blank screen while slower providers are still loading.

### 9.2 Target Templates

Yes, the system is mostly template-driven.

Supported v1 target templates:

- ECS service
- target group / load balancer
- Lambda function
- CloudWatch alarm
- SQS queue

Each template defines:

- how to resolve the target text
- which related resources matter
- which evidence providers to run
- which heuristics are relevant
- which evidence items should be shown first

Example: `ecs service` template

- primary object: ECS service
- related resources: cluster, task definition, target group, load balancer,
  recent tasks, log groups
- evidence providers: service events, task stop reasons, target health,
  CloudTrail writes, recent logs
- heuristics: recent deploy regression, health-check mismatch, secret/env/iam
  regression, capacity or startup failure

The key point: templates are based on resource families, not on trying to
predefine every outage scenario in AWS.

Template count should stay intentionally small.

Recommended scope:

- v1: 5-8 target templates
- mature product: roughly 12-20 templates
- never hundreds, if the model is correct

If the design requires a template per AWS resource type, the design is wrong.
The scaling mechanism should be composition:

- templates cover target families
- evidence providers cover signal families
- heuristic rules cover failure patterns

That allows many real incident combinations to be handled without an explosion
in bespoke templates.

### 9.3 Constrained Scope Expansion

After the target is resolved, expand only a small local scope.

Bad:

- crawl the whole account
- discover every dependency recursively

Good:

- ECS service -> cluster, task definition, TG, ALB, tasks, logs
- Lambda -> function config, recent invocations, logs, DLQ, triggers
- SQS -> queue, DLQ, consumer, recent alarms

This gives enough context for incident triage without turning the cockpit into a
slow graph explorer.

### 9.4 Evidence Providers

Each template invokes a fixed set of evidence providers.

Examples:

- alarm history provider
- deploy/provider-specific event provider
- target health provider
- task stop reason provider
- CloudTrail change provider
- logs summary provider

Providers should return normalized records, not raw API-specific objects.

Conceptually:

```text
IncidentEvent {
  ts
  source
  kind
  resource_ref
  severity
  summary
  raw_ref
}
```

This lets the cockpit merge ECS events, alarms, and CloudTrail changes into one
timeline without the UI caring which AWS API produced them.

### 9.5 Heuristic Rules

After evidence is normalized, run small rule packs.

Examples:

- If a deploy started within 10 minutes before the first error spike, emit a
  `recent deploy` suspect.
- If targets became unhealthy after a new revision was registered, emit a
  `health-check or app-startup regression` suspect.
- If task stop reasons and logs point to missing credentials or access denied,
  emit an `env/secret/iam regression` suspect.
- If queue age rises while consumers are nominal, emit a `consumer throughput or
  handler failure` suspect.

Each rule should output:

- suspect label
- one-line explanation
- supporting evidence references
- next recommended pivot

### 9.6 Suspect Ranking

Suspects do not need ML.

Simple weighted scoring is enough:

- time proximity to symptom start
- number of supporting events
- resource-template relevance
- severity of supporting evidence

No black-box confidence scores in v1. The operator should be able to see why a
suspect ranked high.

### 9.7 Section Population

The overview sections are derived mechanically:

- `Summary`: current symptom facts and strongest correlations
- `Timeline`: normalized event stream sorted by time
- `Suspects`: ranked outputs of heuristic rules
- `Evidence`: supporting pivots attached to suspects and template defaults

This means the overview is neither static nor magical. It is rule-based
correlation over a small, explicit scope.

### 9.8 What Is Predefined vs Dynamic

Predefined:

- target templates
- scope expansion rules
- evidence providers
- heuristic rules
- suspect scoring weights

Dynamic:

- actual resources resolved from the target
- actual AWS events found in the chosen window
- actual correlations between those events
- final suspect ranking for this incident

This is the right level of structure for a TUI incident cockpit.

---

## 10. Is It Worth Building?

Only if the scope is kept narrow.

The full cockpit vision is attractive, but the total AWS surface area is too
large for a small team to support if this turns into a universal incident
platform.

### 10.1 When It Is Worth It

The feature is worth building if:

- the product already has strong read-only infrastructure primitives
- users regularly debug incidents from the terminal
- a few templates cover a large share of operational pain
- the cockpit is implemented as a thin orchestration layer over existing views

The likely high-value v1 problems are repetitive and expensive:

- ECS service broken after deploy
- ALB / target group unhealthy
- Lambda errors or throttling
- SQS backlog and stuck consumers
- CloudWatch alarm fired and "what changed?"

If the cockpit can cut these investigations from tens of minutes to a few
minutes, the feature is likely worth its cost.

### 10.2 When It Is Not Worth It

The feature is not worth building if:

- value appears only after broad AWS coverage
- the design depends on universal dependency discovery
- the team cannot absorb long-tail AWS support burden
- the product starts promising broad causal intelligence
- most users mainly browse infrastructure rather than investigate incidents

The failure mode is a large, expensive system that still only partially
understands real customer estates.

### 10.3 Recommended Product Strategy

Build `:incident` only as a narrow MVP first.

Good approach:

- support 2-3 primary templates first
- reuse existing resource and child views
- merge a few high-signal evidence sources
- keep suspect ranking simple and inspectable

Bad approach:

- attempt broad service coverage up front
- build a universal topology engine
- support arbitrary custom environments before proving baseline value

### 10.4 Go / No-Go Test

The first version should prove one thing:

Can `:incident` clearly outperform the current manual workflow for a few common
incident classes?

Recommended MVP proof target:

- `:incident <ecs-service>`
- `:incident <alarm>`
- merged timeline from a limited set of sources
- short suspects list
- evidence handoff into existing views

If that MVP does not feel materially better than current a9s navigation plus
manual operator knowledge, expansion should stop.

---

## 11. Ranking Logic, UX First

The suspect list should be heuristic and explain itself.

Bad:

- "Probable root cause: deployment issue"

Good:

- "Deploy revision 128 started 2m before error spike"
- "3 of 8 targets became unhealthy after health-check change"
- "Task role policy changed after last healthy deploy"

Each suspect must have:

- one-line explanation
- one-line next action

No black-box confidence scores in v1.

---

## 12. Incident Flows

### 10.1 `:incident payments-api`

1. Start with target prefilled.
2. Press Enter.
3. Land on Overview.
4. Timeline auto-focuses.
5. Enter on a deploy event opens the ECS drill.
6. `b` returns to overview without losing context.

### 10.2 `:incident alarm/high-5xx 1h`

1. Start directly in 1h scope.
2. Overview highlights the alarm, related load balancer, target group, and service.
3. Evidence queue offers alarm history, target health, and service logs first.

### 10.3 `:incident`

1. Open Start view empty.
2. User types a target or picks mode first.
3. App shows examples and minimal friction.

This matters because 3 a.m. operators often know the symptom but not the exact object name.

---

## 13. What The Cockpit Should Reuse

Reuse existing a9s strengths whenever possible:

- detail view
- YAML view
- child views
- command-mode header interaction
- CloudTrail search model
- resource filtering and sorting

The cockpit should coordinate these, not replace them.

---

## 14. What The Cockpit Must Avoid

- Do not open 6 independent polling panes.
- Do not rely on color alone to communicate severity.
- Do not require exact AWS identifiers to get started.
- Do not force the user through a form with 10 fields.
- Do not make the timeline so wide that it is unreadable in 100 columns.
- Do not pretend to know root cause when only correlation exists.
- Do not break the user's existing mental model of `Esc`, `Enter`, `Tab`, `:`, and `/`.

---

## 15. MVP Recommendation

The first useful version of `:incident` should include only:

- Incident Start view
- Incident Overview
- merged timeline for a limited set of event sources
- suspects list with explanations
- evidence queue
- handoff into existing resource/child views

Specifically, v1 should support only these target families:

- ECS service
- target group / load balancer
- Lambda function
- CloudWatch alarm
- SQS queue

This is enough to relieve real operator pain without overbuilding.

For a stricter first release, an even smaller MVP is recommended:

- ECS service
- CloudWatch alarm

Everything else can follow only if these two prove real operator value.

---

## 16. Command Reference Addition

When implemented, the command reference should add:

| Command | Action |
|---|---|
| `:incident` / `:inc` | Open incident start view |
| `:incident <target>` | Open incident cockpit for a target |
| `:incident <target> <window>` | Open cockpit with explicit time window |

---

## 17. Bottom Line

`:incident` should not be "another list view". It should be the one place in
`a9s` where the user can start from pain, get a time-ordered story, see likely
suspects, and jump into the right evidence without stitching the investigation
together manually.
