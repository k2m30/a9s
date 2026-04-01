# GitHub Issue Draft: Resource Issues Overlay Across Resource Lists

GitHub issue: [#196](https://github.com/k2m30/a9s/issues/196)

Design spec: [`docs/design/resource-issues-overlay.md`](/Users/k2m30/projects/a9s/docs/design/resource-issues-overlay.md)
Taxonomy source: [`docs/design/resources-groupping.md`](/Users/k2m30/projects/a9s/docs/design/resources-groupping.md)
Signal catalog: [`docs/design/deterministic-issue-signals.md`](/Users/k2m30/projects/a9s/docs/design/deterministic-issue-signals.md)

## Title

Add a cross-resource issues overlay with per-list issue counts and `!` issue-only filtering

## Summary

Add a lightweight issues overlay so a9s can quickly highlight problematic
resources in existing list views without building a full incident cockpit.

Example UX:

```text
ec2(25)  issues:2
ecs-svc(14)  issues:3
alarm(18)  issues:4
```

And:

- `!` / `!!` / `!!!` can progressively widen the flagged set
- title becomes `ec2(2 issues shown of 25)` or similar severity-aware variants

This should be implemented conservatively using explicit per-resource
heuristics. Not every resource type needs support in v1.

Important UX framing:

- `!` favors precision
- `!!` broadens the suspect set
- `!!!` favors recall and will be noisier

The user must be consciously opting into the larger, noisier set.

## Why

This is a high-value, lower-cost alternative to a full incident workspace.

It helps answer:

- what looks broken right now
- how many problematic resources are in this view
- can I filter to just the suspicious ones

## Non-Goals

- No root cause analysis
- No cross-account correlation
- No universal AWS health engine
- No claim that every resource type has meaningful issue semantics

## Proposed Scope

Follow the support levels and rollout plan in the design doc.

### Phase 1

Implement `L1 Strong` issue detection for the highest-signal resource types.

### Phase 2

Implement selected `L2 Enriched` resource types that require one extra targeted
signal or child-view-equivalent fetch.

### Phase 3

Add main-menu issue badges and consider a global issues view if Phase 1 and 2
prove useful.

## Tasks

- [ ] Review and prune the deterministic signal catalog into implementation candidates
- [ ] Finalize UX for title formatting and `!` / `!!` / `!!!` semantics
- [ ] Decide how unsupported resource types render without implying issue support
- [ ] Add shared issue-state model and filtering semantics
- [ ] Implement Phase 1 `L1` resource heuristics
- [ ] Add tests for issue counts, issue-only filtering, and title rendering
- [ ] Add docs/help updates for the new toggle and behavior
- [ ] Implement selected Phase 2 `L2` resource heuristics
- [ ] Add main-menu issue badges
- [ ] Evaluate whether a global "all issues" view is justified

## Suggested Task Split

### Task Group A: Core UX and plumbing

- title rendering
- issue-only toggle
- generic list filtering semantics
- unsupported-type behavior

### Task Group B: Compute / networking / monitoring L1

- EC2
- ECS tasks
- Elastic Beanstalk
- NAT gateways
- VPC endpoints
- Transit gateways
- CloudWatch alarms

### Task Group C: Databases / storage L1

- DB instances
- DB clusters
- Redis
- OpenSearch
- Redshift
- snapshots

### Task Group D: CDN / CI/CD / misc L1

- CloudFront
- ACM
- CloudFormation
- Kinesis
- MSK
- SES identities

### Task Group E: Selected L2 enrichments

- ECS services
- Lambda
- ASG
- EKS
- load balancers
- target groups
- SQS
- Step Functions
- CodePipelines
- CodeBuild
- Glue
- CloudTrail trails

## Acceptance Criteria

- Supported resource lists show `issues:N` when issue semantics exist
- `!` filters the list to issue rows only
- unsupported resource types do not pretend to have issue counts
- issue detection is deterministic and testable
- false positives are minimized

## Notes

This issue should remain the single umbrella tracker. Follow-up implementation
work can be split into narrower issues or PRs by phase and resource family.
