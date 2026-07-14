# Deterministic Issue Signals Catalog

Status: design inventory
Purpose: enumerate deterministic ways a9s could find and highlight suspicious
or problematic resources without relying on vague heuristics or free-form AI
interpretation
Related docs:

- [#196](https://github.com/k2m30/a9s/issues/196)
- [`docs/design/resource-issues-overlay.md`](~/projects/a9s/docs/design/resource-issues-overlay.md)
- [`docs/design/resources-groupping.md`](~/projects/a9s/docs/design/resources-groupping.md)
- [`docs/issues/resource-issues-overlay-issue.md`](~/projects/a9s/docs/issues/resource-issues-overlay-issue.md)

---

## 1. Goal

This document collects deterministic issue-detection patterns that can be used
across AWS resource types.

"Deterministic" means:

- explicit input
- explicit rule
- explainable output
- no probabilistic guesswork

The output may still be noisy, but it should always be explainable as:

```text
signal X on resource Y matched rule Z
```

This document is intentionally broader than the current rollout plan. It is an
inventory of what is possible, not a commitment to implement everything.

---

## 2. Core Principle

The system should prefer:

- explicit states
- explicit counters
- explicit timestamps
- explicit mismatches

Over:

- vague pattern detection
- hidden scoring
- "something looks off" logic

Every issue flag should have a machine-readable reason and a user-facing reason.

Example:

```text
reason_code: unhealthy_targets
reason_text: 3 unhealthy targets in target group
```

---

## 3. Deterministic Signal Families

## 3.1 State / Status Signals

The simplest and usually strongest category.

Definition:

- inspect a resource state/status field
- compare against a known bad or degraded set

Examples:

- EC2 instance state != `running`
- RDS status != `available`
- CloudWatch alarm state = `ALARM`
- ACM cert state = `FAILED_VALIDATION`
- CloudFormation stack state in rollback/failure family

General rule form:

```text
if status in {failed, error, degraded, alarm, unhealthy, stopped, rejected}
then flag issue
```

Useful for:

- list-level scans
- cheap background checks
- highly explainable UI labels

---

## 3.2 Transitional-State Timeout Signals

A state may be acceptable briefly but suspicious if it lasts too long.

Definition:

- inspect lifecycle state
- compare elapsed time in state to a threshold

Examples:

- EBS snapshot still `pending` after threshold
- CloudFront distribution still `in progress` too long
- ACM cert still `PENDING_VALIDATION` too long
- NAT gateway still `deleting` or `pending` too long
- stack update still in progress too long

General rule form:

```text
if state in transitional_set and time_in_state > threshold
then flag issue
```

This is deterministic if:

- the current state is explicit
- the last transition timestamp is available or inferable

---

## 3.3 Health-Check Aggregate Signals

Definition:

- inspect child/member health under a parent resource
- count unhealthy/degraded members

Examples:

- target group unhealthy targets > 0
- load balancer with zero healthy targets
- ECS service with unhealthy tasks/targets
- ASG with in-service < desired
- EKS node group with unhealthy nodes

General rule form:

```text
if bad_child_count > 0
then flag issue
```

Or:

```text
if healthy_child_count == 0 and total_child_count > 0
then flag critical issue
```

This is one of the highest-value deterministic signal families.

---

## 3.4 Desired vs Actual Mismatch Signals

Definition:

- compare configured target state to observed live state

Examples:

- ECS desired count > running count
- ASG desired > in-service
- replica count != healthy replicas
- expected target registrations > healthy targets

General rule form:

```text
if desired_value != actual_value
then flag issue
```

Severity may be proportional to the size of the mismatch.

---

## 3.5 Recent Operation Failure Signals

Definition:

- inspect the most recent execution, deployment, run, or activity
- flag if the latest outcome is failure-like

Examples:

- latest CodePipeline execution failed
- latest CodeBuild build failed
- latest Glue job run failed
- latest Step Functions execution failed
- latest scaling activity failed
- latest deployment failed

General rule form:

```text
if latest_operation_status in failure_set
then flag issue
```

This is deterministic and often high-signal.

---

## 3.6 Repeated Failure Count Signals

Definition:

- count failures in a bounded recent window
- compare to threshold

Examples:

- Lambda failed invocations in last 15m > threshold
- Step Functions failed executions in last hour > threshold
- ECS task stop events in last 15m > threshold
- CodeBuild failures in last N runs > threshold

General rule form:

```text
if failure_count(window) >= threshold
then flag issue
```

This is deterministic, but it requires careful window and threshold defaults.

---

## 3.7 Error-Rate Signals

Definition:

- compute error ratio from explicit counters
- compare against threshold

Examples:

- Lambda errors / invocations > threshold
- API 5xx / requests > threshold if integrated through alarms or metrics
- DLQ receives / total processed > threshold

General rule form:

```text
if error_count / total_count >= threshold
then flag issue
```

Deterministic, but depends on trustworthy counters and bounded windows.

---

## 3.8 Throughput / Backlog / Queueing Signals

Definition:

- use explicit backlog or age indicators
- compare against threshold

Examples:

- SQS ApproximateAgeOfOldestMessage > threshold
- SQS visible messages > threshold
- Kinesis iterator age > threshold
- consumer lag > threshold

General rule form:

```text
if backlog_metric > threshold
then flag issue
```

This category is especially useful for "system is slow" and "stuck pipeline"
reports.

---

## 3.9 Saturation / Capacity Signals

Definition:

- inspect a utilization metric or hard-limit indicator
- compare against threshold

Examples:

- DB connections near max
- storage usage near full
- Lambda concurrency near account or function limit
- subnet IP availability below threshold
- disk burst balance or CPU credit exhaustion if exposed

General rule form:

```text
if utilization >= threshold
then flag issue
```

This is deterministic, but many saturation signals require extra metric reads.

---

## 3.10 Throttling / Rejection Signals

Definition:

- detect explicit service throttles or rejected operations

Examples:

- Lambda throttles > 0 in recent window
- API Gateway throttles > threshold
- VPC endpoint state = rejected
- pending acceptance too long for share/endpoint/tgw workflows

General rule form:

```text
if throttle_count > 0 or state == rejected
then flag issue
```

---

## 3.11 Event / History Failure Signals

Definition:

- inspect recent event streams for explicit failure markers

Examples:

- ECS service event contains placement failure
- RDS event indicates failover/problem
- CloudFormation event contains failure state
- alarm history contains repeated alarm-action failures

General rule form:

```text
if recent_events contain failure_pattern
then flag issue
```

This is deterministic if matching is based on structured status fields or a
curated explicit phrase set.

---

## 3.12 Structured Log Error Count Signals

Definition:

- count explicit error/severity markers in bounded recent logs

Examples:

- recent Lambda invocation logs contain `ERROR` events
- ECS service logs contain N explicit error lines in last 10m
- CodeBuild logs contain failed phase markers

General rule form:

```text
if explicit_error_log_count(window) >= threshold
then flag issue
```

This is only deterministic if:

- the matching rules are explicit
- the time window is bounded
- the signal source is well-defined

Free-form semantic log understanding is out of scope.

---

## 3.13 Stop-Reason / Exit-Code Signals

Definition:

- inspect explicit failure reasons from stopped work units

Examples:

- ECS task stop reason populated
- essential container exited
- non-zero container exit code
- failed Lambda invocation result type
- failed step execution cause

General rule form:

```text
if last_stop_reason in failure_set or exit_code != 0
then flag issue
```

This category is one of the most operator-friendly because it often answers not
just "is broken" but "how did it fail."

---

## 3.14 Configuration Validity / Expiry Signals

Definition:

- inspect explicit validity windows or known invalid states

Examples:

- ACM certificate expired
- secret or certificate rotation overdue
- identity unverified
- key pending deletion
- pending deletion resource where deletion is operationally significant

General rule form:

```text
if validity_end < now or state in invalidity_set
then flag issue
```

This is deterministic, but semantics vary by resource family.

---

## 3.15 Association / Attachment Integrity Signals

Definition:

- detect missing or broken required associations

Examples:

- target group with no registered targets
- service without load balancer attachment when one is expected
- route or listener referencing missing downstream resource
- endpoint with broken association

General rule form:

```text
if required_association_count == 0 or association_state invalid
then flag issue
```

This can be high-value, but only when the requirement is explicit and not a
guess.

---

## 3.16 Consistency / Cross-Field Mismatch Signals

Definition:

- compare two explicit fields that should agree

Examples:

- task desired vs running
- replicas configured vs replicas available
- logging enabled flag vs missing destination details
- alarm action enabled vs action target invalid

General rule form:

```text
if field_a and field_b are logically inconsistent
then flag issue
```

This is deterministic and often cheap.

---

## 3.17 Last-Seen / Freshness Signals

Definition:

- inspect how old the last successful signal or activity is

Examples:

- no successful recent invocations where activity is expected
- replication heartbeat stale
- last successful backup too old
- last delivery or execution timestamp too old

General rule form:

```text
if now - last_success_ts > threshold
then flag issue
```

Useful, but dangerous if "expected activity" is not truly known.

---

## 3.18 Slow-Operation / Latency Signals

Definition:

- compare explicit latency or duration against threshold

Examples:

- query latency > threshold
- invocation duration > threshold
- build duration anomaly relative to configured threshold
- API p95 latency > threshold

General rule form:

```text
if latency_metric(window) > threshold
then flag issue
```

Deterministic, but requires metric access and careful threshold selection.

---

## 3.19 CloudTrail Change Signals

Definition:

- detect explicit recent write/change events on a resource
- usually not enough alone to call it broken
- but can raise a "recent change" risk flag

Examples:

- IAM role updated in the last 15m
- target group health-check config changed
- Lambda configuration updated
- security group modified

General rule form:

```text
if recent_write_event_count(window) > 0
then flag recent-change marker
```

This should usually be a separate annotation, not a generic issue by itself.

---

## 3.20 Security / Exposure Signals

Definition:

- detect explicit policy or exposure conditions that are always risky

Examples:

- public exposure where policy forbids it
- wildcard IAM permission if policy mode is enabled
- disabled key on actively referenced resource

General rule form:

```text
if posture_rule violated
then flag issue
```

This is deterministic only if the policy baseline is explicit. Without a known
policy baseline, generic security risk detection should be conservative.

---

## 4. Signal Output Shape

Every signal should be representable as structured output:

```text
IssueSignal {
  resource_type
  resource_id
  signal_family
  reason_code
  reason_text
  severity
  observed_value
  expected_value
  observed_at
}
```

This allows one resource to have multiple reasons without collapsing them too
early.

Example:

```text
resource_type: tg
resource_id: tg-payments
signal_family: health_aggregate
reason_code: unhealthy_targets
reason_text: 3 unhealthy targets
severity: issue
observed_value: 3
expected_value: 0
```

Suggested normalized severities:

- `issue`: hard failure or unhealthy condition
- `warning`: degraded or threshold-based problem
- `risk`: recent change or suspicious state worth surfacing at the highest verbosity

These map naturally to a possible a9s UI model:

- `!` -> show `issue`
- `!!` -> show `issue` + `warning`
- `!!!` -> show `issue` + `warning` + `risk`

This should be treated as a precision/recall ladder:

- `!` favors precision
- `!!` trades some precision for broader coverage
- `!!!` favors recall and therefore tolerates more noise

---

## 5. Resource-Type Mapping Guidance

When deciding which signal families apply to a resource type:

- prefer native state/status first
- then prefer explicit child-health aggregates
- then prefer recent operation results
- then bounded counters and metrics
- use logs only when matching rules are explicit and cheap enough

Do not start from logs if a structured signal already exists.

---

## 6. Determinism Rules

To count as deterministic in a9s, a signal should satisfy most of these:

- source is explicit and bounded
- rule is explicit
- threshold is explicit
- reason can be shown in one line
- repeated runs produce the same result for the same input
- the user can inspect where the signal came from

Signals that fail these tests should not be part of the default issues overlay.

---

## 7. Possible UI Verbosity Mapping

If a9s uses repeated `!` presses similarly to CLI verbosity flags, the signal
catalog can be surfaced progressively:

### `!` Hard Issues

Show only:

- explicit failed states
- explicit unhealthy states
- explicit alarm/rejected states
- critical desired vs actual mismatches

### `!!` Warnings

Add:

- repeated recent failures
- backlog/age thresholds
- saturation/capacity thresholds
- transitional-state timeouts
- degraded but not failed statuses

### `!!!` Risks

Add:

- recent change markers
- pending deletion markers
- expiry-soon markers
- posture/config validity flags that are suspicious but not necessarily broken

This keeps the mental model simple while allowing richer deterministic signals to
coexist without overwhelming the default view.
It also makes the noise increase explicit and intentional.

---

## 8. What This Catalog Is For

This inventory is useful for:

- deciding which signals belong in baseline scans
- deciding which signals require enrichment
- designing per-resource issue rules
- avoiding hidden or ad hoc issue logic

It is not a commitment to build every signal family.

---

## 9. Bottom Line

There are many deterministic ways to surface suspicious AWS resources, but they
fall into a manageable set of signal families. The important design job is not
inventing more signal types. It is choosing the right ones per resource type and
keeping the resulting issue flags explicit, bounded, and explainable.
