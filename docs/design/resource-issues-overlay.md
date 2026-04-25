# Resource Issues Overlay Design Spec

Issue: [#196](https://github.com/k2m30/a9s/issues/196)
Version: 1.0
Target: a9s vNext
Status: Design
Source taxonomy: [`docs/design/resources-groupping.md`](~/projects/a9s/docs/design/resources-groupping.md)
Tracking issue: [#196](https://github.com/k2m30/a9s/issues/196)
Local issue draft: [`docs/issues/resource-issues-overlay-issue.md`](~/projects/a9s/docs/issues/resource-issues-overlay-issue.md)
Signal catalog: [`docs/design/deterministic-issue-signals.md`](~/projects/a9s/docs/design/deterministic-issue-signals.md)

---

## 1. Goal

Add a lightweight, read-only health/issues overlay across all resource lists so
operators can quickly answer:

- which resources look unhealthy or risky right now
- how many problematic resources are in the current list
- can I filter the list down to only problematic items

This is intentionally much smaller and cheaper than a full incident cockpit.

---

## 2. UX Summary

### 2.1 Frame Title

Preferred title format:

```text
ec2(25)  issues:2
ecs-svc(14)  issues:3
alarm(18)  issues:4
```

Avoid compressed forms like `ec2(25/2 issues)` because they are harder to scan.

### 2.2 List Toggle

Add a dedicated issue-mode toggle:

- `!` cycles issue visibility/intensity

Recommended progression:

- normal: all rows, issue counts visible
- `!`: show only strong, obvious issues
- `!!`: show issues plus warnings
- `!!!`: show issues, warnings, and recent-change/risk markers

This is closer to the familiar CLI mental model of `-v`, `-vv`, `-vvv`.
It should also be understood as an explicit precision/recall tradeoff:

- `!` = highest precision, lowest noise
- `!!` = broader suspect set, moderate noise
- `!!!` = highest recall, highest noise

More `!` means a wider flagged set and a greater chance that some rows are only
potentially problematic rather than clearly broken.

Example title states:

```text
ec2(2 issues shown of 25)
acm(4 warnings shown of 18)
lambda(7 flagged shown of 22)
```

`Esc` or another `!` cycle step returns to the normal list state.

### 2.2.1 Why This Is Better Than A Binary Toggle

A binary `only issues` mode works well for hard failures but becomes too blunt
once the product starts surfacing softer deterministic signals.

Examples:

- `ALARM` state is a hard issue
- expired ACM certificate is a hard issue
- recent failed Lambda invocations may be a warning
- recent CloudTrail write on a sensitive resource is a risk marker, not a hard issue

The multi-level `!` model allows a9s to grow richer signals without collapsing
everything into one noisy bucket.

It also makes the noise increase explicit and user-controlled, instead of
quietly broadening the meaning of "issue" over time.

### 2.3 Main Menu

Longer-term, the main menu can show issue badges per resource type:

```text
EC2 Instances              2 issues
Target Groups              5 issues
CloudWatch Alarms          4 issues
```

This should come after per-list support is proven.

---

## 3. Product Boundaries

This feature is an **issues overlay**, not a universal health engine.

It should:

- rely on explicit, per-resource heuristics
- be conservative when evidence is weak
- prefer undercounting to false certainty

It should not:

- claim root cause
- require cross-account correlation
- require incident timeline reconstruction
- show issue counts based on expensive account-wide graph traversal

The broader inventory of deterministic signal types is tracked separately in
[`docs/design/deterministic-issue-signals.md`](~/projects/a9s/docs/design/deterministic-issue-signals.md).
This doc focuses on product behavior and rollout, not on exhaustively listing
every possible signal family.

---

## 4. Detection Levels

Every resource type should be assigned one of four support levels.

| Level | Meaning |
|---|---|
| `L1 Strong` | Issues can be inferred from fields already present in the list/detail shape |
| `L2 Enriched` | Issues require one extra targeted read call or child-view-equivalent signal |
| `L3 Contextual` | Issues are meaningful only with neighboring resource context; defer or keep conservative |
| `L4 None` | No reliable generic issue semantics; do not show an issue count in v1 |

Rules:

- v1 should focus on `L1 Strong`
- v2 can add selected `L2 Enriched`
- `L3` and `L4` should not block rollout

---

## 5. Issue Semantics

An "issue" means one of:

- failed state
- unhealthy state
- degraded state
- stuck transitional state
- active alarm / error / rejection state
- materially suspicious security or config posture

It does **not** mean:

- merely not used
- merely old
- merely expensive
- merely empty

Severity classes:

- `Issue`: hard failure, unhealthy, alarm, rejection, critical mismatch
- `Warning`: degraded, repeated recent failures, growing backlog, high saturation
- `Risk`: recent change, pending deletion, expiring validity, suspicious but not broken

These classes support the `!` / `!!` / `!!!` interaction model.
They should not be rendered as if they have equal certainty.

---

## 6. Resource Coverage Matrix

## 6.1 Compute

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| EC2 Instances | L1 | `stopped`, `stopping`, `terminated`, instance status or system status impaired |
| ECS Services | L2 | running < desired, failed deployment, service events indicating placement or steady-state failure, unhealthy targets |
| ECS Clusters | L2 | active services with failed tasks or capacity/provider issues |
| ECS Tasks | L1 | `STOPPED`, unhealthy, non-zero stop reason, essential container exited |
| Lambda Functions | L2 | recent invocation errors/throttles, failed state, inactive or pending update failure |
| Auto Scaling Groups | L2 | desired != in-service, failed scaling activity, instance refresh failure |
| Elastic Beanstalk | L1 | environment status degraded, severe, updating for too long, non-ready health |
| EBS Volumes | L1 | error state, impaired state, detached when expected attached is known later, creating for too long |
| EBS Snapshots | L1 | error state, pending too long |
| AMIs | L4 | no generic issue semantics; do not count in v1 |

## 6.2 Containers

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| EKS Clusters | L2 | cluster not active, failed update, control plane degraded |
| EKS Node Groups | L2 | not active, degraded, scaling/update failure |

## 6.3 Networking

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| Load Balancers | L2 | failed state, active_impaired, no healthy targets through linked target groups |
| Target Groups | L2 | unhealthy targets > 0, all targets unhealthy, draining only |
| Security Groups | L4 | no generic issue semantics; unsafe posture is policy work, not v1 issues overlay |
| VPCs | L4 | no generic issue semantics |
| Subnets | L3 | availability/IP exhaustion is useful but often needs enrichment and context |
| Route Tables | L4 | no generic issue semantics |
| NAT Gateways | L1 | failed, deleting unexpectedly, not available |
| Internet Gateways | L4 | no generic issue semantics |
| Elastic IPs | L3 | unattached may be wasteful, not necessarily an operational issue |
| VPC Endpoints | L1 | failed or rejected state |
| Transit Gateways | L1 | failed, pending-acceptance too long, modifying issues |
| Network Interfaces | L3 | detached/available can be normal; generic issue detection is weak |

## 6.4 Databases & Storage

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| DB Instances | L1 | not `available`, storage full-ish if surfaced later, failed, incompatible, backing-up too long |
| S3 Buckets | L4 | no generic issue semantics in v1 |
| ElastiCache Redis | L1 | not available, modifying/failing, snapshot/replication failure state if present |
| DB Clusters | L1 | not available, failover/degraded, incompatible states |
| DynamoDB Tables | L3 | status not active is useful; throttling/backlog needs metrics enrichment |
| OpenSearch Domains | L1 | processing/degraded/failed, cluster not active |
| Redshift Clusters | L1 | not available, resizing/restoring too long, maintenance/problem states |
| EFS File Systems | L4 | no strong generic issue semantics in v1 |
| DB Instance Snapshots | L1 | failed, creating too long |
| DB Cluster Snapshots | L1 | failed, creating too long |

## 6.5 Monitoring

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| CloudWatch Alarms | L1 | state = `ALARM` |
| CloudWatch Log Groups | L4 | no generic issue semantics |
| CloudTrail Trails | L2 | logging disabled, not multi-region when expected is policy-level and should not count generically |
| CloudTrail Events | L4 | events themselves are evidence, not resources with issue counts |

## 6.6 Messaging

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| SQS Queues | L2 | oldest message age/backlog over threshold, DLQ growth, no consumers if detectable later |
| SNS Topics | L4 | no generic issue semantics |
| SNS Subscriptions | L1 | pending confirmation, delivery disabled if surfaced |
| EventBridge Rules | L3 | disabled may be intentional; failed invocations need metrics/context |
| Kinesis Streams | L1 | not active, updating/creating too long |
| MSK Clusters | L1 | not active, failed, maintenance/problem state |
| Step Functions | L2 | recent failed executions, state machine with active failures |

## 6.7 Secrets & Config

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| Secrets Manager | L3 | pending deletion is notable but not always an issue; no generic runtime issue semantics |
| SSM Parameters | L4 | no generic issue semantics |
| KMS Keys | L3 | disabled or pending deletion can be significant but often intentional; show carefully later |

## 6.8 DNS & CDN

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| Route 53 Hosted Zones | L4 | no generic issue semantics |
| CloudFront Distributions | L1 | not deployed, failed/in-progress too long |
| ACM Certificates | L1 | expired, failed validation, pending validation too long |
| API Gateways | L3 | generic issue detection weak without deployment/error metrics |

## 6.9 Security & IAM

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| IAM Roles | L4 | no generic issue semantics |
| IAM Policies | L4 | no generic issue semantics |
| IAM Users | L4 | no generic issue semantics |
| IAM Groups | L4 | no generic issue semantics |
| WAF Web ACLs | L3 | unavailable/failed association is useful later; generic v1 issue semantics weak |

## 6.10 CI/CD

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| CloudFormation Stacks | L1 | rollback, failed, delete_failed, update_rollback_failed |
| CodePipelines | L2 | latest execution failed, pipeline stuck |
| CodeBuild Projects | L2 | recent build failures |
| ECR Repositories | L4 | no generic issue semantics |
| CodeArtifact Repositories | L4 | no generic issue semantics |

## 6.11 Data & Analytics

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| Glue Jobs | L2 | recent failed runs |
| Athena Workgroups | L4 | no generic issue semantics in v1 |

## 6.12 Backup

| Resource Type | Level | Candidate Issue Heuristics |
|---|---|---|
| Backup Plans | L4 | plans themselves have no generic issue semantics |
| SES Identities | L1 | unverified, failed verification, sending disabled if surfaced |

---

## 7. Rollout Plan by Value

### Phase 1: High-Signal L1 Types

Ship issue counts and the first `!` mode for:

- EC2 Instances
- ECS Tasks
- Elastic Beanstalk
- NAT Gateways
- VPC Endpoints
- Transit Gateways
- DB Instances
- Redis
- DB Clusters
- OpenSearch
- Redshift
- DB Instance Snapshots
- DB Cluster Snapshots
- CloudWatch Alarms
- Kinesis Streams
- MSK Clusters
- CloudFront Distributions
- ACM Certificates
- CloudFormation Stacks
- SES Identities

### Phase 2: Selected L2 Types

Add enriched detection and support for `!!` warning-level signals for:

- ECS Services
- Lambda Functions
- Auto Scaling Groups
- EKS Clusters
- EKS Node Groups
- Load Balancers
- Target Groups
- SQS Queues
- Step Functions
- CodePipelines
- CodeBuild Projects
- Glue Jobs
- CloudTrail Trails

### Phase 3: Main Menu Aggregates

- issue badges on main menu
- command to jump to "issues only" for a resource type
- maybe a global "all issues" menu view later
- evaluate whether `!!!` risk markers belong in list mode, menu mode, or only in detail views

---

## 8. Development Principles

- Keep issue logic data-driven where possible.
- Keep issue heuristics per resource type explicit and testable.
- When semantics are weak, do not guess.
- Prefer false negatives over noisy false positives.
- Keep titles and toggles readable in 80-column terminals.
- Do not require cross-resource graph expansion for v1 counts.
- Do not promote warnings or risks to hard issues just to inflate counts.
- Make the precision/recall tradeoff explicit in the UI and docs.

---

## 9. QA Expectations

Every supported resource type should have stories for:

- list title without issues
- list title with issues/warnings when supported
- `!` toggled on
- `!!` toggled on when warning signals exist
- `!!!` toggled on when risk markers exist
- row-level issue marking consistency
- refresh behavior
- empty list behavior
- access-denied / missing-enrichment behavior for `L2`

Unsupported types should have explicit behavior:

- no `issues:N` suffix, or
- suffix omitted until support exists

Do not silently imply issue coverage where there is none.

---

## 10. Bottom Line

This feature is worth doing because it gives operators a fast, cheap, and
credible answer to:

- what is clearly broken
- what is degraded
- what changed recently enough to deserve suspicion

without taking on the cost of a full incident platform.

The `!` progression is valuable because it lets the user choose how much noise
they want to admit into the suspect set instead of forcing one global notion of
"issue" across all workflows.
