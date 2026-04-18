# Related Panel Implementation Plan

**Status**: 61 types with gaps, 358 missing checkers
**Source of truth**: `docs/related-resources.md` Per-type contract table
**Enforcement**: `TestRelatedPanel_ContractMatchesGoldenDoc`

## Preparation (done)

- [x] `zzz_ct_events_all_related.go` auto-registers ct-events for all types (0 manual work)
- [x] `relatedResult()` helper exists in `ec2_related.go` (dedup + count)
- [x] Cache-scan helpers exist per type (e.g., `ec2RelatedResources`, `s3RelatedResources`)

## Implementation Patterns

### P1: Field Extraction (no cache, no API)

Read target ID(s) from `res.Fields["key"]` or `res.RawStruct`. `NeedsTargetCache: false`.
~3 lines per checker. Fastest to implement.

```go
func checkXyzVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
    id := res.Fields["vpc_id"]
    if id == "" { return resource.RelatedCheckResult{TargetType: "vpc", Count: 0} }
    return relatedResult("vpc", []string{id})
}
```

### P2: Cache Scan (scan target cache for references to current resource)

Load target type from cache, scan for matches. `NeedsTargetCache: true`.
~15-25 lines per checker. Needs RawStruct type assertion.

### P3: Reverse Cache Scan (scan current type's cache peers for shared attribute)

Scan another type's cache looking for resources that share a VPC, subnet, SG, etc.
Similar to P2 but the match key is a shared attribute, not a direct reference.

### P4: Stub (relationship exists but data not in list API)

Return `Count: 0`. The relationship is real but can't be resolved from cached data.
Used when the connection requires a separate API call not available on the list response.

## Batches by Target Type

Grouped by the target resource type being added. Each batch adds one target
type across all sources that need it. This maximizes helper reuse.

### Batch 1: `kms` (32 sources) — P1

Every KMS checker follows the same pattern: extract KMS key ARN/ID from
RawStruct field, return the key ID portion after the last `/`.

Sources: alarm, ami, apigw, cb, codeartifact, dbc, ebs (done), ecs-task,
eip, eks, glue, iam-user, kinesis, lambda, logs, msk, ng, opensearch,
pipeline, r53 (stub), redis, redshift, s3, secrets (done), ses, sfn, sg
(stub), sns, sqs, tg, vpce, waf (stub)

*Not all sources have a KMS field on the list response. Stubs for those.*

### Batch 2: `role` (25 sources) — P1/P2

Two sub-patterns:
- **P1 (field)**: Source has a role ARN field → extract role name from ARN
- **P2 (cache-scan)**: Source doesn't have a role field → scan role cache for
  references (rare for this target)

Sources: alarm, apigw, asg, athena, backup, cb (done), cf (stub), dbi, eb,
ecs-svc, ecs-task, eks, kms, lambda (done), logs (stub), opensearch,
pipeline (done), r53 (stub), redshift, s3, secrets, ses, sqs, trail (done),
tgw, waf

### Batch 3: `logs` (23 sources) — P2/P4

Two sub-patterns:
- **P2 (cache-scan)**: Scan log groups for groups named after the source
  (convention: `/aws/{service}/{resource-name}`)
- **P4 (stub)**: When no naming convention exists

Sources: alarm, apigw, athena, backup, cb (done), cf, ddb, eb-rule, ebs,
ecs-task, eip, elb, kinesis, lambda (done), logs (skip), msk, nat (stub),
pipeline, r53, redis, redshift, secrets, ses, vpce, waf

### Batch 4: `vpc` (21 sources) — P1

Most sources have a VPC ID field. Pure field extraction.

Sources: asg, cb, dbc, dbi, docdb-snap, ec2, eks, eni, lambda, msk,
nat (done), opensearch, r53, redis, redshift, rtb, subnet, tg, vpce

### Batch 5: `subnet` (16 sources) — P1/P2

- **P1**: Source has subnet ID(s) → extract
- **P2**: Scan subnet cache for matching VPC

Sources: asg (done), cb, ec2, ecs-svc, ecs-task, eks, eni, lambda,
msk, nat (done), ng, opensearch, redis, redshift, subnet (skip), tg

### Batch 6: `s3` (15 sources) — P2/P4

S3 relationships are mostly operational (log destinations, artifact stores).
Cache-scan S3 looking for bucket name references, or stub when not in list API.

Sources: alarm (stub), cb, cfn, cf (done), elb, glue, lambda, logs,
msk, pipeline, r53, s3 (skip), secrets, ses, vpce

### Batch 7: `sns` (13 sources) — P1/P2

SNS topic references from various services (notifications, alarms).

Sources: alarm (done), asg, backup, cfn, ddb, eb-rule, lambda, pipeline,
redis, s3, secrets, ses, sqs

### Batch 8: `sg` (13 sources) — P1/P2

Security group references. Most sources have a SecurityGroups field.

Sources: asg, cb, eb, ec2, ecs-svc, ecs-task, eks, elb, lambda, msk (done),
ng, opensearch, redis (done), redshift, sg (skip), tg

### Batch 9: `alarm` (12 sources) — P2

Scan CloudWatch alarm cache for alarms with dimensions referencing the source.
Pattern exists in `checkEC2Alarms`.

Sources: cf, ebs, ecs-task, efs, eip, nat, ses, vpce, waf, eb

*alarm→X already done for: ec2, asg, sns. The reverse (X→alarm) is this batch.*

### Batch 10: `secrets` (10 sources) — P2/P4

Scan Secrets Manager cache for secrets tagged/named for the source service.

Sources: cb, codeartifact, ddb, ecs-task, glue, lambda, msk, redis,
redshift, tg

### Batch 11: `lambda` (10 sources) — P2

Scan Lambda cache for functions with VPC, event source, or layer references
to the source.

Sources: apigw (done), cf, ddb, eb-rule, kinesis, lambda (skip), logs,
s3, sfn, sg

### Batch 12: `eni` (9 sources) — P2

Scan ENI cache for interfaces in the source's VPC/subnet, or attached to the
source.

Sources: dbi, ec2, ecs-task, efs, elb, eni (skip), nat, rtb, vpc

### Batch 13: `eb-rule` (9 sources) — P2

Scan EventBridge rules cache for rules with targets referencing the source.

Sources: backup, cfn, eb-rule (skip), ecr, ecs-svc, kinesis, pipeline,
s3, ses, sqs

### Batch 14: `backup` (9 sources) — P2/P4

Scan Backup plan cache for plans protecting the source resource type.
May need stub where backup plan structure doesn't identify targets.

Sources: ddb, docdb-snap, ebs, ebs-snap, ec2, efs, rds-snap, s3, tg

### Batch 15: `ecs-task` (7 sources) — P2

Scan ECS task cache for tasks referencing source resources.

Sources: alarm, ecs, efs, eip, logs, secrets, sg (stub)

### Batch 16: `ecr` (7 sources) — P2

Scan ECR repository cache for references from source services.

Sources: cb (done), ecs-svc, ecs-task, eks, lambda, pipeline, secrets (stub)

### Batch 17: `ec2` (7 sources) — P2

Scan EC2 cache for instances in the source VPC/subnet/SG.

Sources: alarm, ecs, ecs-task, efs, eks, lambda, nat (stub)

### Batch 18: `acm` (7 sources) — P2

Scan ACM cache for certificates used by the source service.

Sources: apigw, codeartifact, eks, opensearch, ses, vpce

### Batch 19: Remaining small batches (< 7 sources each)

| Target | Count | Sources |
|--------|-------|---------|
| `r53` | 6 | apigw, ecs-svc, lambda, r53 (skip), s3, vpce |
| `kinesis` | 6 | codeartifact, ddb, eb-rule, kinesis (skip), logs, ses |
| `cfn` | 6 | ami, asg, ebs, ecr, pipeline, tg |
| `waf` | 5 | apigw (done), codeartifact, s3, vpce, cf (done) |
| `vpce` | 5 | ddb, eni, rtb, subnet, vpce (skip) |
| `sfn` | 5 | apigw, eb-rule, ecs-svc, lambda, sfn (skip) |
| `elb` | 5 | asg, cf, ecs-svc (done), eni, lambda |
| `cf` | 5 | apigw, ecs-svc, lambda, r53, vpce |
| `asg` | 5 | ecs, eip, eks, lambda, subnet |
| `eks` | 4 | ecr, eks (skip), role, subnet (stub) |
| `ecs` | 4 | ecr, eip, ecs (skip), sns-sub |
| `apigw` | 4 | alarm, logs, r53, vpce |
| `ami` | 4 | ec2, eks, ng, ami (skip) |
| `ssm` | 4 | cb, ec2, ecs-task, lambda |
| `tg` | 3 | eb, lambda, vpce |
| `iam-user` | 3 | eks, iam-user (skip), role, s3 |
| `dbi` | 3 | dbc, rds-snap (done), tg |
| `dbc` | 3 | dbi, docdb-snap (done), rds-snap |
| `trail` | 2 | ct-events, ses |
| `tgw` | 2 | rtb, vpc |
| `sqs` | 2 | eb-rule, s3 |
| `pipeline` | 2 | ecr, secrets |
| `glue` | 2 | athena, s3 |
| `efs` | 2 | efs (skip), subnet |
| `ecs-svc` | 2 | ecs-svc (skip), pipeline |
| `ddb` | 2 | kinesis, lambda |
| `codeartifact` | 2 | pipeline, secrets |
| `cb` | 2 | secrets, cb (skip) |
| `athena` | 2 | glue, s3 |
| Singletons | 8 | sns-sub, rds-snap, policy, ng, nat, msk, iam-group, ebs, eb, eip, docdb-snap |

## Recommended Execution Order

1. **Batch 1 (kms)** — 32 checkers, trivial P1 pattern, highest impact
2. **Batch 4 (vpc)** — 21 checkers, trivial P1 pattern
3. **Batch 2 (role)** — 25 checkers, mostly P1 (ARN extraction)
4. **Batch 5 (subnet)** — 16 checkers, mostly P1
5. **Batch 8 (sg)** — 13 checkers, mostly P1
6. **Batch 7 (sns)** — 13 checkers, mix P1/P2
7. **Batch 3 (logs)** — 23 checkers, P2/P4 (naming convention scan)
8. **Batch 9 (alarm)** — 12 checkers, P2 (dimension scan)
9. **Batch 6 (s3)** — 15 checkers, P2/P4
10. **Batches 10-19** — remaining targets

## Notes

- Each batch should be implementable in ~1 agent dispatch (20-40 checkers)
- Batch 1-5 are pure field extraction — can run in parallel
- Batch 3, 9 require cache-scan logic — more complex but patterns exist
- Some checkers will be stubs (Count: 0) when the data isn't on the list response
- Every checker needs: function, RegisterRelated entry, no new tests needed
  (the golden contract test is the test)
