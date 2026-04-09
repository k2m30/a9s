# ct-events Test Coverage Checklist

**Generated:** 2026-04-08
**Post-fix state:** reflects Cases J/K/L invariant fixes (alice.admin→alice.johnson, role_name→acme-ci-deploy-role, charlie+AdministratorAccess fixtures added, LookupEvents count 15→18).

## Section 1: Summary

| Metric | Count |
|--------|-------|
| Total ct-events fixtures | 18 |
| Fully covered (left + right + related count) | 9 (Cases A–I) |
| Partial / no coverage | 9 (initial 6 + J/K/L) |
| Unresolvable navigable-field references | 1 (`acme-rds-monitoring` in initial Case 5) |
| Unresolvable right-column targets | 1 (KMS key ARN in Case D) |
| "Description-only" roles referenced but not registered as fixtures | 5 (see §3) |

The 18 events split into two groups:
- **Initial 6** — `evt-0a1b2c3d4e5f60001..6` (pre-v2 fixtures in `fixtures_monitoring.go` ~L1118–1297). Present in demo mode, **not in any ctdetail test matrix**.
- **Cases A–L** — `e-a1b2c3d4..f2a3b4c5` (v2 wireframe-aligned fixtures ~L1298–1660). Cases A–I covered by 3 test matrices; J/K/L not in any test matrix.

Navigable fields registered for ct-events (`RegisterNavigableFields("ct-events", …)`):
- `user` → `iam-user`
- `role_name` → `role`

Related groups registered for ct-events (`RegisterRelated("ct-events", …)`): 13 typed + 4 self-pivot = **17**:
- Typed: `role`, `iam-user`, `ec2`, `s3`, `s3_objects`, `lambda`, `rds`, `kms`, `secrets`, `vpce`, `sg`, `ddb`, `cfn`
- Pivots (ct-events → ct-events): `AccessKeyId`, `Username`, `EventName`, `SharedEventId`

## Section 2: Coverage Table

Legend: **Resolves?** ✅ = target fixture exists / ❌ = missing. **Covered?** ✅ = asserted in named test / ❌ = no assertion exists. One row per (event, navigable field) or (event, related group).

### Initial 6 events (no test coverage)

| Event ID | EventName | Col | Field/Group | Expected Target | Resolves? | Covered? |
|---|---|---|---|---|---|---|
| evt-…60001 | CreateBucket | left | user | alice.johnson | ✅ | ❌ |
| evt-…60001 | CreateBucket | left | role_name | deploy-bot | ✅ | ❌ |
| evt-…60001 | CreateBucket | right | s3 | webapp-assets-prod | ✅ | ❌ |
| evt-…60001 | CreateBucket | right | pivot EventName | self (CreateBucket) | ✅ | ❌ |
| evt-…60002 | DeleteBucket | left | user | bob.smith | ✅ | ❌ |
| evt-…60002 | DeleteBucket | left | role_name | acme-ci-deploy-role | ✅ | ❌ |
| evt-…60002 | DeleteBucket | right | s3 | webapp-assets-prod | ✅ | ❌ |
| evt-…60003 | DescribeInstances | left | user | alice.johnson | ✅ | ❌ |
| evt-…60003 | DescribeInstances | left | role_name | acme-eks-node-role | ✅ | ❌ |
| evt-…60004 | TerminateInstanceInAutoScalingGroup | left | user | alice.johnson | ✅ | ❌ |
| evt-…60004 | TerminateInstanceInAutoScalingGroup | left | role_name | monitoring-agent | ✅ | ❌ |
| evt-…60004 | TerminateInstanceInAutoScalingGroup | right | ec2 | i-…60001 | ✅ | ❌ |
| evt-…60005 | ApiCallRateInsight | left | user | bob.smith | ✅ | ❌ |
| evt-…60005 | ApiCallRateInsight | left | role_name | **acme-rds-monitoring** | ❌ | ❌ |
| evt-…60006 | VpcEndpointAccess | left | user | ci-service-account | ✅ | ❌ |
| evt-…60006 | VpcEndpointAccess | left | role_name | ci-runner | ✅ | ❌ |

Non-listed right-column groups for initial events are all expected-empty and currently untested.

### Cases A–I (fully covered)

All (event, field) and (event, related group) pairs for Cases A–I are asserted in:
- `tests/unit/ctdetail_demo_nav_test.go` (28 left-column subtests)
- `tests/unit/ctdetail_demo_rightcol_nav_test.go` (28 right-column subtests, uses `assertRelatedIDsResolve`)
- `tests/unit/ctdetail_demo_related_test.go` (117 per-case count subtests)
- `tests/unit/ctdetail_demo_golden_test.go` (9 golden snapshots)

| Event ID | EventName | Highlights | Covered? |
|---|---|---|---|
| e-a1b2c3d4 | DescribeInstances | role=KarpenterNodeRole | ✅ A |
| e-b2c3d4e5 | TerminateInstances | ec2=2 instances, role=AWSReservedSSO_… | ✅ B |
| e-c3d4e5f6 | PutObject | s3+s3_objects=webapp-assets-prod, iam-user=bob.smith | ✅ C |
| e-d4e5f6a7 | RotateKey | kms key ARN (⚠️ key UUID resolves via stripKMSKeyID) | ✅ D |
| e-e5f6a7b8 | PutBucketPolicy | s3=webapp-assets-prod | ✅ E |
| e-f6a7b8c9 | GetObject | s3+s3_objects=data-pipeline-logs, vpce=vpce-0abc123def456 | ✅ F |
| e-a7b8c9d0 | PutObject | s3+s3_objects=ml-training-data, role=CiBuildRole | ✅ G |
| e-b8c9d0e1 | RunInstances | no actionable related | ✅ H |
| e-c9d0e1f2 | PutObject | s3+s3_objects=cloudtrail-audit-logs, vpce=vpce-0ff11223344556677 | ✅ I |

### Cases J/K/L (IAM — Phase 1 summarizer coverage, NO test coverage)

| Event ID | EventName | Col | Field/Group | Expected Target | Resolves? | Covered? |
|---|---|---|---|---|---|---|
| e-d0e1f2a3 | CreateUser | left | user | alice.johnson | ✅ | ❌ |
| e-d0e1f2a3 | CreateUser | left | role_name | acme-ci-deploy-role | ✅ | ❌ |
| e-d0e1f2a3 | CreateUser | right | iam-user | charlie (new fixture) | ✅ | ❌ |
| e-d0e1f2a3 | CreateUser | right | pivot Username | self (alice.johnson) | ✅ | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | left | user | alice.johnson | ✅ | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | left | role_name | acme-ci-deploy-role | ✅ | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | iam-user | bob | ✅ | ❌ |
| e-e1f2a3b4 | AttachUserPolicy | right | (policy nav) | AdministratorAccess | ✅ | ❌ |
| e-f2a3b4c5 | CreateAccessKey | left | user | alice.johnson | ✅ | ❌ |
| e-f2a3b4c5 | CreateAccessKey | left | role_name | acme-ci-deploy-role | ✅ | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | iam-user | bob | ✅ | ❌ |
| e-f2a3b4c5 | CreateAccessKey | right | pivot AccessKeyId | self (AKIA…EXAMPLE) | ✅ | ❌ |

## Section 3: Gaps Requiring Action

### 3.1 Unresolvable references
1. **`acme-rds-monitoring` role** — referenced by initial Case 5 `role_name`, no role fixture. Fix: add role fixture OR change `role_name` to an existing role.
2. **Cases A/B/F/G/I "role" group checkers** — the group count tests pass, but per-group resolution goes through `checkRoleByCtEventsSessionIssuer` which matches by name. The following names appear as inline strings in Case comments but need verification that a matching `Name` exists in `iamRoleFixtures()`:
   - `KarpenterNodeRole` (Case A)
   - `AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d` (Case B)
   - `eks-checkout-svc-sa` (Case F)
   - `CiBuildRole` (Case G)
   - `DataPipelineRole` (Case I)
   
   Cases A–I pass `assertRelatedIDsResolve` in `ctdetail_demo_rightcol_nav_test.go`, so these resolve *today* — flagged only because the original fixture-listing agent noted them as "description-only comments." Re-verify next time the `iamRoleFixtures()` list changes.

### 3.2 Missing test matrix entries
- **Initial 6 events** not in any of the 3 ct-events test matrices. Zero left-column, zero right-column, zero golden snapshot, zero related-count coverage.
- **Cases J/K/L** (IAM Phase 1) not in any test matrix. Phase 1 was declared done after unit-level tests of `ctdetail.SummarizeIAM`, but no demo-mode end-to-end test exercises the IAM summarizer.

### 3.3 Action items to close the gaps

**Priority 1 — Cases J/K/L** (Phase 1 completion):
1. Add J/K/L to `ctdetail_demo_nav_test.go` left-column matrix with `assertTargetResolves`.
2. Add J/K/L to `ctdetail_demo_rightcol_nav_test.go` right-column matrix with `assertRelatedIDsResolve`.
3. Add J/K/L to `ctdetail_demo_related_test.go` count matrix.
4. Add J/K/L golden snapshots to `ctdetail_demo_golden_test.go` (use `UPDATE_GOLDEN=1` to generate).

**Priority 2 — Initial 6 events**:
1. Either decide they're legacy and move to an archive/skip list (document why), OR
2. Extend all 3 matrices to cover them the same way Cases A–L are covered.
3. Resolve `acme-rds-monitoring` reference in Case 5.

**Priority 3 — Structural**:
- Consider refactoring the test matrices to auto-iterate `demo.GetResources("ct-events")` so new fixtures can't be silently left out of tests.

## Section 4: Protocol for future fixture additions

To avoid the J/K/L invariant-violation incident recurring:

1. Before adding a ct-events fixture, verify in `fixtures_security.go` / `fixtures_compute.go` / etc. that every referenced `userName`, `role_name`, `bucketName`, `keyId`, `vpceId`, resource ARN, etc. already exists.
2. Add the fixture to **all 4** test matrices in the same PR:
   - `ctdetail_demo_nav_test.go` (left column)
   - `ctdetail_demo_rightcol_nav_test.go` (right column)
   - `ctdetail_demo_related_test.go` (counts)
   - `ctdetail_demo_golden_test.go` (snapshot)
3. Bump `TestDemoTransport_LookupEvents` expected count.
4. Run full suite (`rtk go test ./tests/unit/ -count=1`) and confirm 0 failures before reporting done.

See also: `docs/testing-detail-view-coverage.md` — the full TDD playbook for detail-view coverage.
