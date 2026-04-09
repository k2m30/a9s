# ct-events Fixture Cleanup Spec

**Audit date:** 2026-04-08
**File:** `internal/demo/fixtures_monitoring.go` (ct-events slice ~L1118–1661)
**Total events:** 18 — all 18 have at least one internal inconsistency.

## Consistency rules

Each event has 5 data sources that must agree:

1. `CloudTrailEvent` JSON `userIdentity.type` — Root / IAMUser / AssumedRole / AWSService
2. `userIdentity.userName` — present iff type=IAMUser
3. `userIdentity.sessionContext.sessionIssuer.userName` — present iff type=AssumedRole
4. Top-level `Username` field — matches principal name
5. `Fields.user` / `Fields.role_name` — drive left-column navigable fields

**Derivation rules:**
- `Fields.user` = userName iff type=IAMUser, else empty
- `Fields.role_name` = sessionIssuer.userName iff type=AssumedRole, else empty
- Top-level `Username` = userName (IAMUser) / sessionIssuer.userName (AssumedRole) / "Root" or nil (Root) / nil (AWSService)
- `resources[]` ARNs must correspond to real demo fixtures
- `requestParameters.bucketName` (and similar) must match `resources[]` and exist as fixtures

## Fixture cross-reference status

All referenced fixtures EXIST in demo data:
- **iam-user**: `alice.johnson`, `bob`, `bob.smith`, `ci-service-account`, `charlie`
- **role**: `acme-eks-node-role`, `deploy-bot`, `acme-ci-deploy-role`, `monitoring-agent`, `acme-rds-monitoring`, `ci-runner`, `KarpenterNodeRole`, `AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d`, `eks-checkout-svc-sa`, `CiBuildRole`, `DataPipelineRole`
- **s3**: `webapp-assets-prod`, `data-pipeline-logs`, `ml-training-data`, `cloudtrail-audit-logs`, `prod-logs`, `prod-artifacts`, `checkout-config`, `shared-artifacts`, `prod-lake`
- **ec2**: `i-0a1b2c3d4e5f60001`, `i-0a1b2c3d4e5f60002`
- **iam-policy**: `arn:aws:iam::aws:policy/AdministratorAccess`

No missing fixtures. All fixes are edits to existing event data.

---

## Cleanup actions by identity type

### Root identity (2 events)

**`evt-0a1b2c3d4e5f60001` CreateBucket**
- Current: `Username="alice.johnson"`, `Fields.user="alice.johnson"`, `Fields.role_name="deploy-bot"`
- Fix: set `Username=nil`, `Fields.user=""`, `Fields.role_name=""`
- Keep: `resources[]=[webapp-assets-prod]`, `requestParameters.bucketName=webapp-assets-prod`

**`e-e5f6a7b8` PutBucketPolicy (Case E)**
- Current: `Fields.user="ci-service-account"`, `Fields.role_name="deploy-bot"`, `requestParameters.bucketName="prod-artifacts"` but `resources[0]="webapp-assets-prod"`
- Fix: set `Fields.user=""`, `Fields.role_name=""`; update `resources[0]` to `prod-artifacts` to match requestParameters

### AWSService identity (3 events)

**`evt-0a1b2c3d4e5f60004` TerminateInstanceInAutoScalingGroup**
- Current: `Fields.user="alice.johnson"`, `Fields.role_name="monitoring-agent"`
- Fix: set both to empty

**`e-d4e5f6a7` RotateKey (Case D)**
- Current: `Fields.user="ci-service-account"`, `Fields.role_name="monitoring-agent"`
- Fix: set both to empty

**`e-b8c9d0e1` RunInstances (Case H)** — Insight event, no userIdentity
- Current: `Fields.user="bob.smith"`, `Fields.role_name="acme-eks-node-role"`
- Fix: set both to empty

### IAMUser identity (6 events)

**`evt-0a1b2c3d4e5f60002` DeleteBucket**
- Current: `Fields.role_name="acme-ci-deploy-role"` (IAMUser has no assumed role)
- Fix: set `Fields.role_name=""`

**`evt-0a1b2c3d4e5f60005` ApiCallRateInsight**
- Current: `Fields.role_name="acme-rds-monitoring"`
- Fix: set `Fields.role_name=""`

**`e-c3d4e5f6` PutObject (Case C)**
- Current: `Fields.role_name="acme-ci-deploy-role"`; `requestParameters.bucketName="prod-logs"` but `resources[0]="webapp-assets-prod"`
- Fix: set `Fields.role_name=""`; update `requestParameters.bucketName` to `webapp-assets-prod` (or update `resources[0]` to `prod-logs` — pick whichever matches scenario intent)
- Note: userName `bob` IS a valid fixture (audit agent was wrong). No user fix needed.

**`e-d0e1f2a3` CreateUser (Case J)**
- Current: `Fields.role_name="acme-ci-deploy-role"`
- Fix: set `Fields.role_name=""`

**`e-e1f2a3b4` AttachUserPolicy (Case K)**
- Current: `Fields.role_name="acme-ci-deploy-role"`; `resources[0]="bob"`
- Fix: set `Fields.role_name=""`. Keep `resources[0]="bob"` (fixture exists).

**`e-f2a3b4c5` CreateAccessKey (Case L)**
- Current: `Fields.role_name="acme-ci-deploy-role"`; `resources[0]="bob"`
- Fix: set `Fields.role_name=""`. Keep `resources[0]="bob"`.

### AssumedRole identity (7 events)

**`evt-0a1b2c3d4e5f60003` DescribeInstances**
- Current: `Fields.user="alice.johnson"` (AssumedRole has role only, no underlying user)
- Fix: set `Fields.user=""`

**`evt-0a1b2c3d4e5f60006` VpcEndpointAccess**
- Current: `Fields.user="ci-service-account"`
- Fix: set `Fields.user=""`

**`e-a1b2c3d4` DescribeInstances (Case A)**
- Current: `Fields.user="alice.johnson"`
- Fix: set `Fields.user=""`

**`e-b2c3d4e5` TerminateInstances (Case B)**
- Current: `Fields.user="alice.johnson"` (JSON sessionName is `alice@corp`, not `alice.johnson`)
- Fix: set `Fields.user=""`

**`e-f6a7b8c9` GetObject (Case F)**
- Current: `Fields.user="alice.johnson"`; `requestParameters.bucketName="checkout-config"` but `resources[0]="data-pipeline-logs"`
- Fix: set `Fields.user=""`; update `resources[0]` to `checkout-config`

**`e-a7b8c9d0` PutObject (Case G)**
- Current: `Fields.user="bob.smith"`; `requestParameters.bucketName="shared-artifacts"` but `resources[0]="ml-training-data"`
- Fix: set `Fields.user=""`; update `resources[0]` to `shared-artifacts`

**`e-c9d0e1f2` PutObject (Case I)**
- Current: `Fields.user="alice.johnson"`; `requestParameters.bucketName="prod-lake"` but `resources[0]="cloudtrail-audit-logs"`
- Fix: set `Fields.user=""`; update `resources[0]` to `prod-lake`

---

## Impact on test coverage

Once fixtures are clean:

1. **Left-column navigable fields** (`user`, `role_name`) will be empty for Root / AWSService / AssumedRole (user) / IAMUser (role_name) events. Existing tests that assert these resolve for such events will need to stop asserting non-empty expectations.

2. **Right-column related groups** will derive cleanly from the event JSON:
   - `role` group → only from `sessionIssuer.userName` for AssumedRole events
   - `iam-user` group → only from `userIdentity.userName` for IAMUser events
   - `s3` group → from `resources[]` + `requestParameters.bucketName`
   - etc.

3. **Cases A–L golden snapshots** may need regeneration (`UPDATE_GOLDEN=1`).

4. **Related count matrices** in `ctdetail_demo_related_test.go` may need updates where the cross-linked Fields map was inflating counts.

## Suggested execution order

1. Apply fixture fixes (one atomic coder task, 18 event edits).
2. Run `rtk go test ./tests/unit/ -count=1` — expect failures in the 4 test matrices that reference the cross-linked values.
3. Regenerate golden snapshots: `UPDATE_GOLDEN=1 rtk go test ./tests/unit/ -run TestCtDetailDemoGolden -count=1`.
4. Update nav/rightcol/related matrices to match clean data.
5. Rebuild binary, visually verify `./a9s --demo` against wireframes.
6. Regenerate the coverage checklist (`docs/ct-events-test-coverage-checklist.md`) from clean ground truth.

## Open question for the user

For the 4 bucket mismatches (Cases C, E, F, G, I), there are two valid fixes:
- **A**: update `resources[0]` to match `requestParameters.bucketName` (trust requestParameters)
- **B**: update `requestParameters.bucketName` to match `resources[0]` (trust resources)

Recommendation: **Option A** — requestParameters is what the caller actually sent; resources[] is metadata. Also the design wireframes in `docs/design/ct-event-detail-v2.md` show distinct bucket names per case (prod-artifacts, prod-logs, checkout-config, shared-artifacts, prod-lake) which match the requestParameters, suggesting that was the authored intent.
