---
name: batch-related-views
description: Use when implementing all remaining related-view sub-issues for GitHub issue #119, skipping #140 (EC2, already done). Orchestrates architect scoping then parallel coder+QA dispatch per resource type, sequentially.
---

# Batch Related-View Implementation (#119)

Implements all 65 open sub-issues of #119 sequentially, one resource type at a time.
Per issue: architect reads research doc → scopes CODER+QA tasks → parallel dispatch → verify → commit → close issue.

**Skip:** #140 (EC2 — already closed).

---

## Issues Queue (in order)

| # | ShortName | Title |
|---|-----------|-------|
| 120 | acm | ACM Certificates |
| 121 | alarm | CloudWatch Alarms |
| 122 | ami | AMIs |
| 123 | apigw | API Gateways |
| 124 | asg | Auto Scaling Groups |
| 125 | athena | Athena Workgroups |
| 126 | backup | Backup Plans |
| 127 | cb | CodeBuild Projects |
| 128 | cf | CloudFront Distributions |
| 129 | cfn | CloudFormation Stacks |
| 130 | codeartifact | CodeArtifact Repositories |
| 131 | ct-event | CloudTrail Events |
| 132 | dbc | DocumentDB Clusters |
| 133 | dbi | DB Instances |
| 134 | ddb | DynamoDB Tables |
| 135 | docdb-snap | DocumentDB Snapshots |
| 136 | eb | Elastic Beanstalk |
| 137 | eb-rule | EventBridge Rules |
| 138 | ebs | EBS Volumes |
| 139 | ebs-snap | EBS Snapshots |
| 141 | ecr | ECR Repositories |
| 142 | ecs | ECS Clusters |
| 143 | ecs-svc | ECS Services |
| 144 | ecs-task | ECS Tasks |
| 145 | efs | EFS File Systems |
| 146 | eip | Elastic IPs |
| 147 | eks | EKS Clusters |
| 148 | elb | Load Balancers |
| 149 | eni | Network Interfaces |
| 150 | glue | Glue Jobs |
| 151 | iam-group | IAM Groups |
| 152 | iam-user | IAM Users |
| 153 | igw | Internet Gateways |
| 154 | kinesis | Kinesis Data Streams |
| 155 | kms | KMS Keys |
| 156 | lambda | Lambda Functions |
| 157 | logs | CloudWatch Log Groups |
| 158 | msk | MSK Clusters |
| 159 | nat | NAT Gateways |
| 160 | ng | EKS Node Groups |
| 161 | opensearch | OpenSearch Domains |
| 162 | pipeline | CodePipelines |
| 163 | policy | IAM Policies |
| 164 | r53 | Route 53 Hosted Zones |
| 165 | rds-snap | RDS Snapshots |
| 166 | redis | ElastiCache Redis |
| 167 | redshift | Redshift Clusters |
| 168 | role | IAM Roles |
| 169 | rtb | Route Tables |
| 170 | s3 | S3 Buckets |
| 171 | secrets | Secrets Manager |
| 172 | ses | SES Identities |
| 173 | sfn | Step Functions |
| 174 | sg | Security Groups |
| 175 | sns | SNS Topics |
| 176 | sns-sub | SNS Subscriptions |
| 177 | sqs | SQS Queues |
| 178 | ssm | SSM Parameters |
| 179 | subnet | Subnets |
| 180 | tg | Target Groups |
| 181 | tgw | Transit Gateways |
| 182 | trail | CloudTrail Trails |
| 183 | vpc | VPCs |
| 184 | vpce | VPC Endpoints |
| 185 | waf | WAF Web ACLs |

---

## Per-Issue Loop

Repeat for each row in the table above. The loop body is sequential per issue (no shared-file conflicts between issues). Coder and QA run in parallel within each issue.

### Step 1 — Read Research

Read both files before producing any scope:

```
docs/design/related-resources/{shortname}.md
```

From `.a9s/views_reference.yaml`, extract the `{shortname}` section to find forward field paths.

Also read `internal/aws/{shortname}.go` to confirm which Fields keys are already populated.

Use `internal/aws/ec2_related.go` as the canonical pattern reference (read-only).

### Step 2 — Produce Architect Handoff

Using the `a9s-add-related-view` skill's **Architect Handoff Format**, produce exact CODER TASK and QA TASK. Include:

- Left Column: all `NavigableField` entries with verified FieldPath → TargetType mappings
- Right Column: all `RelatedDef` entries with DisplayName, match strategy (Pattern F or C), and `NeedsTargetCache`
- Exact file paths to create and append points for shared files

### Step 3 — Parallel Dispatch

Dispatch simultaneously:

```
Agent A — a9s-related-coder   → CODER TASK (Steps 1-7)
Agent B — a9s-related-qa      → QA TASK    (Steps 8-12)
```

Both agents auto-load `a9s-add-related-view` and `a9s-common` via their agent definition — do NOT repeat those skill instructions in the dispatch prompt. Pass only the architect scope (handoff table).

### Step 4 — Verify

Run these commands in order (stop on first failure, diagnose before retrying):

```bash
go test ./tests/unit/ -count=1 -timeout 120s -run "Related_{ShortNamePascal}|NavigableFields_{ShortNamePascal}"
make test
make lint
make gofix
make build
```

If any command fails, scope a targeted fix to the appropriate agent (coder or QA) with the exact error and file location. Do NOT proceed to Step 4.5 until all four pass.

### Step 4.5 — Smoke Test (TUI golden test)

Create `tests/unit/aws_{shortname}_related_smoketest_test.go` using the same pattern as `aws_ami_related_smoketest_test.go`. Tests to include:

- **S01**: Right column auto-shows at width=120 with `RELATED` header and `│` separator
- **S02**: Correct display name labels appear in right column
- **S03**: After delivering demo-equivalent results, counts show correctly `(N)` and `(0)`
- **S04**: Tab focuses right column; Enter on a count>0 row emits `RelatedNavigateMsg` with correct `TargetType`
- **S05**: Enter on all-count=0 right column emits no `RelatedNavigateMsg`
- **S06**: Nil-checker (stub) defs have nil Checker; demo checker still returns a result for each stub target

Then run:
```bash
go test ./tests/unit/ -count=1 -timeout 60s -run "TestAMI_Smoke|Test{ShortNamePascal}_Smoke"
```

**If tests fail:** pass bug report to `a9s-architect` for root cause and fix scope. Apply fix, re-run Step 4, then re-run Step 4.5.

**If all pass:** proceed to Step 5. Commit the smoketest file alongside the implementation files.

### Step 5 — Commit

Stage only the files changed for this issue:

```
internal/aws/{shortname}.go
internal/aws/{shortname}_related.go
internal/demo/fixtures_related.go
tests/unit/aws_{shortname}_related_test.go
tests/unit/related_registry_test.go
internal/aws/interfaces.go          (only if new interfaces were added)
tests/unit/mocks_test.go            (only if new mocks were added)
```

Commit message format:
```
feat: related-view for {Title} ({shortname}) (#{N})
```

### Step 6 — Close Issue

```bash
gh issue close {N} --repo k2m30/a9s --comment "Implemented in $(git rev-parse --short HEAD)."
```

Then immediately begin Step 1 for the next issue in the table.

---

## Shared Files — Append Order Rule

These files accumulate one block per resource type:
- `internal/demo/fixtures/<service>.go` — ensure target fixtures contain matching IDs for related navigation
- `tests/unit/related_registry_test.go` — append `TestRelated_{Source}_Registered`

Because agents write to them sequentially (one issue at a time), there are no conflicts. Each agent appends at the last occurrence of the pattern in the file.

---

## Resuming After Interruption

If the batch was interrupted:
1. Run `gh issue list --repo k2m30/a9s --milestone "" --state open --label enhancement` to see which issues remain open
2. Cross-reference with the Issues Queue table above
3. Resume from the first open issue

---

## Completion Gate

After all 65 issues are done:

1. Run full verification:
```bash
make test
make lint
make security
make gofix
make build
```

2. Run pre-push checklist agents:
   - `a9s-consistency-checker`
   - `test-coverage-analyzer`
   - `a9s-architect` with `a9s-arch-review` skill (target: 8.5+/10)

3. Present results to user for release decision.
