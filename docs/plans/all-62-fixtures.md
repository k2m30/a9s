# Plan: Shared Fixtures for All 62 Resource Types (Demo + Tests)

**Author:** a9s-architect
**Date:** 2026-03-21
**Status:** Draft
**Supersedes:** Partial overlap with `p0-p1-rawstruct-fixtures.md` (P1 section)

---

## Problem Statement

### Current State

| Location | Types Covered | Has RawStruct | Importable by Tests |
|----------|:------------:|:-------------:|:-------------------:|
| `internal/demo/fixtures.go` + `fixtures_ec2.go` | 5 (ec2, s3, lambda, dbi, s3_objects) | Yes | Yes (package `demo`) |
| `tests/unit/fixtures_test.go` | 8 (s3, s3_objects, ec2, dbi, redis, dbc, eks, secrets) | Partial (redis, dbc only) | No (`package unit`) |
| `tests/unit/qa_*_test.go` (realistic* builders) | 62 | Yes (in-place structs) | No (`package unit_test`) |

Three independent fixture ecosystems with no sharing. Maintaining fixtures in three
places means they drift, and adding a new resource type requires touching at least two
of them.

### Target State

- **Demo mode** shows data for all 62 resource types (currently 57 show empty lists)
- **Unit tests** share the same canonical fixture data with correct RawStruct types
- **One source of truth** per resource type's fixture data
- No circular dependency issues

---

## Design Decision: `internal/demo/` is the Canonical Source

### Rationale

1. **`internal/demo/` is already production code** that ships in the binary. It must have
   correct RawStruct types regardless. Making it THE canonical source eliminates
   duplication rather than creating a third package.

2. **Tests can already import it.** `tests/unit/demo_test.go`, `demo_rawstruct_test.go`,
   and `demo_render_test.go` already import `internal/demo`. No new dependency paths
   needed.

3. **No circular dependency risk.** The import graph is:
   ```
   internal/demo/ --> internal/resource/   (for resource.Resource)
   internal/demo/ --> AWS SDK types         (for RawStruct)
   tests/unit/    --> internal/demo/        (already exists)
   internal/tui/  --> internal/demo/        (already exists)
   ```
   `internal/demo/` does NOT import `internal/tui/`, `internal/aws/`, or `tests/`.
   It only imports `internal/resource/` (for the `Resource` struct) and AWS SDK type
   packages (for RawStruct values). This is a leaf package with minimal inbound coupling.

4. **Alternatives rejected:**
   - `internal/fixtures/` -- Creates a new package with identical dependencies as `demo/`.
     No benefit; just more indirection. Tests already import `demo/`.
   - `internal/testdata/` -- Go convention is for `testdata/` dirs containing static files,
     not Go code. Misleading name.
   - Keep `tests/unit/` as source -- `_test.go` files cannot be imported outside their
     package. Tests that need fixtures would be stuck in `package unit_test` forever.

### What This Means for Existing Test Fixtures

- `tests/unit/fixtures_test.go` (`package unit`) -- **Keep for now.** These Fields-only
  fixtures serve a different purpose: they test the Fields-fallback path. Over time,
  consider replacing them with calls to `demo.GetResources()`, but that is out of scope
  for this plan.

- `tests/unit/qa_*_test.go` `realistic*()` builders (`package unit_test`) -- **Keep as-is.**
  These are already used by ~60+ detail/list tests. They will coexist with demo fixtures.
  The demo fixtures serve demo mode and any NEW tests that need shared data; the realistic
  builders continue to serve their existing test files.

---

## File Organization

### Principle: One File Per Category, Not Per Type

62 individual files would be excessive (each is ~40-100 lines). One monolithic file would
be unreadable. Categories from `resource/types.go` provide a natural grouping of 3-8
types each.

### File Layout

```
internal/demo/
  fixtures.go              # GetResources(), GetS3Objects(), demoData registry, mustParseTime()
  fixtures_compute.go      # ec2, ecs-svc, ecs, ecs-task, lambda, asg, eb
  fixtures_containers.go   # eks, ng
  fixtures_networking.go   # elb, tg, sg, vpc, subnet, rtb, nat, igw, eip, vpce, tgw, eni
  fixtures_databases.go    # dbi, s3, redis, dbc, ddb, opensearch, redshift, efs, rds-snap, docdb-snap
  fixtures_monitoring.go   # alarm, logs, trail
  fixtures_messaging.go    # sqs, sns, sns-sub, eb-rule, kinesis, msk, sfn
  fixtures_secrets.go      # secrets, ssm, kms
  fixtures_dns_cdn.go      # r53, cf, acm, apigw
  fixtures_security.go     # role, policy, iam-user, iam-group, waf
  fixtures_cicd.go         # cfn, pipeline, cb, ecr, codeartifact
  fixtures_data.go         # glue, athena
  fixtures_backup.go       # backup, ses
```

**13 files total** (including the existing `fixtures.go` which stays as the registry).
The existing `fixtures_ec2.go` is renamed/merged into `fixtures_compute.go`.

### Naming Conventions

- **File names:** `fixtures_{category}.go` -- lowercase, snake_case category matching
  the `resource/types.go` categories (normalized to filesystem-safe names).

- **Generator functions:** Private, registered via `init()`. Pattern: `{shortName}Fixtures()`.
  Examples: `ec2Fixtures()`, `rdsInstanceFixtures()`, `sqsQueueFixtures()`.

- **Helper constructors** (for complex types like EC2): `make{Type}(params)`.
  Example: `makeEC2Instance(id, name, state, ...)`. Only used when 3+ instances share
  the same construction pattern.

- **Registration:** Each file's `init()` adds to the `demoData` map:
  ```go
  func init() {
      demoData["ecs-svc"] = ecsServiceFixtures
      demoData["ecs"]     = ecsClusterFixtures
      demoData["ecs-task"] = ecsTaskFixtures
      // ...
  }
  ```

### Sub-Resources

Two resource types are sub-resources accessed through parent drill-down:

| Sub-Resource | Parent | Access Function | Already Exists |
|-------------|--------|----------------|:--------------:|
| s3_objects | s3 (bucket) | `GetS3Objects(bucket, prefix)` | Yes |
| r53_records | r53 (zone) | `GetR53Records(zoneId)` | No -- must add |

`GetR53Records(zoneId string) ([]resource.Resource, bool)` will be added to `fixtures.go`
alongside `GetS3Objects()`. The zone ID will be matched against demo R53 zone fixtures.

---

## SDK Type Mapping (All 62 + 2 Sub-Resources)

Source of truth: `cmd/refgen/main.go` lines 59-122.

### COMPUTE (7 types)

| ShortName | SDK Type | Package Import Alias | Notes |
|-----------|----------|---------------------|-------|
| `ec2` | `ec2types.Instance` | `ec2types "...ec2/types"` | Already done |
| `ecs-svc` | `ecstypes.Service` | `ecstypes "...ecs/types"` | |
| `ecs` | `ecstypes.Cluster` | (same as ecs-svc) | |
| `ecs-task` | `ecstypes.Task` | (same as ecs-svc) | |
| `lambda` | `lambdatypes.FunctionConfiguration` | `lambdatypes "...lambda/types"` | Already done |
| `asg` | `asgtypes.AutoScalingGroup` | `asgtypes "...autoscaling/types"` | |
| `eb` | `ebtypes.EnvironmentDescription` | `ebtypes "...elasticbeanstalk/types"` | |

### CONTAINERS (2 types)

| ShortName | SDK Type | Package Import Alias |
|-----------|----------|---------------------|
| `eks` | `ekstypes.Cluster` | `ekstypes "...eks/types"` |
| `ng` | `ekstypes.Nodegroup` | (same as eks) |

### NETWORKING (12 types)

| ShortName | SDK Type | Package Import Alias |
|-----------|----------|---------------------|
| `elb` | `elbv2types.LoadBalancer` | `elbv2types "...elasticloadbalancingv2/types"` |
| `tg` | `elbv2types.TargetGroup` | (same as elb) |
| `sg` | `ec2types.SecurityGroup` | (same as ec2) |
| `vpc` | `ec2types.Vpc` | (same as ec2) |
| `subnet` | `ec2types.Subnet` | (same as ec2) |
| `rtb` | `ec2types.RouteTable` | (same as ec2) |
| `nat` | `ec2types.NatGateway` | (same as ec2) |
| `igw` | `ec2types.InternetGateway` | (same as ec2) |
| `eip` | `ec2types.Address` | (same as ec2) |
| `vpce` | `ec2types.VpcEndpoint` | (same as ec2) |
| `tgw` | `ec2types.TransitGateway` | (same as ec2) |
| `eni` | `ec2types.NetworkInterface` | (same as ec2) |

### DATABASES & STORAGE (10 types)

| ShortName | SDK Type | Package Import Alias |
|-----------|----------|---------------------|
| `dbi` | `rdstypes.DBInstance` | `rdstypes "...rds/types"` | Already done |
| `s3` | `s3types.Bucket` | `s3types "...s3/types"` | Already done |
| `redis` | `elasticachetypes.CacheCluster` | `elasticachetypes "...elasticache/types"` |
| `dbc` | `docdbtypes.DBCluster` | `docdbtypes "...docdb/types"` |
| `ddb` | `ddbtypes.TableDescription` | `ddbtypes "...dynamodb/types"` |
| `opensearch` | `ostypes.DomainStatus` | `ostypes "...opensearch/types"` |
| `redshift` | `redshifttypes.Cluster` | `redshifttypes "...redshift/types"` |
| `efs` | `efstypes.FileSystemDescription` | `efstypes "...efs/types"` |
| `rds-snap` | `rdstypes.DBSnapshot` | (same as dbi) |
| `docdb-snap` | `docdbtypes.DBClusterSnapshot` | (same as dbc) |

### MONITORING (3 types)

| ShortName | SDK Type | Package Import Alias |
|-----------|----------|---------------------|
| `alarm` | `cwtypes.MetricAlarm` | `cwtypes "...cloudwatch/types"` |
| `logs` | `cwlogstypes.LogGroup` | `cwlogstypes "...cloudwatchlogs/types"` |
| `trail` | `cloudtrailtypes.Trail` | `cloudtrailtypes "...cloudtrail/types"` |

### MESSAGING (7 types)

| ShortName | SDK Type | Package Import Alias | Notes |
|-----------|----------|---------------------|-------|
| `sqs` | `string` (NOT a struct) | N/A | Fetcher uses `fmt.Sprintf("%v", attrs)` -- see Special Cases |
| `sns` | `snstypes.Topic` | `snstypes "...sns/types"` |
| `sns-sub` | `snstypes.Subscription` | (same as sns) |
| `eb-rule` | `eventbridgetypes.Rule` | `eventbridgetypes "...eventbridge/types"` |
| `kinesis` | `kinesistypes.StreamSummary` | `kinesistypes "...kinesis/types"` |
| `msk` | `kafkatypes.Cluster` | `kafkatypes "...kafka/types"` |
| `sfn` | `sfntypes.StateMachineListItem` | `sfntypes "...sfn/types"` |

### SECRETS & CONFIG (3 types)

| ShortName | SDK Type | Package Import Alias |
|-----------|----------|---------------------|
| `secrets` | `smtypes.SecretListEntry` | `smtypes "...secretsmanager/types"` |
| `ssm` | `ssmtypes.ParameterMetadata` | `ssmtypes "...ssm/types"` |
| `kms` | `*kmstypes.KeyMetadata` (pointer) | `kmstypes "...kms/types"` |

### DNS & CDN (4 types)

| ShortName | SDK Type | Package Import Alias |
|-----------|----------|---------------------|
| `r53` | `r53types.HostedZone` | `r53types "...route53/types"` |
| `cf` | `cftypes.DistributionSummary` | `cftypes "...cloudfront/types"` |
| `acm` | `acmtypes.CertificateSummary` | `acmtypes "...acm/types"` |
| `apigw` | `apigwtypes.Api` | `apigwtypes "...apigatewayv2/types"` |

### SECURITY & IAM (5 types)

| ShortName | SDK Type | Package Import Alias |
|-----------|----------|---------------------|
| `role` | `iamtypes.Role` | `iamtypes "...iam/types"` |
| `policy` | `iamtypes.Policy` | (same as role) |
| `iam-user` | `iamtypes.User` | (same as role) |
| `iam-group` | `iamtypes.Group` | (same as role) |
| `waf` | `wafv2types.WebACLSummary` | `wafv2types "...wafv2/types"` |

### CI/CD (5 types)

| ShortName | SDK Type | Package Import Alias |
|-----------|----------|---------------------|
| `cfn` | `cfntypes.Stack` | `cfntypes "...cloudformation/types"` |
| `pipeline` | `cptypes.PipelineSummary` | `cptypes "...codepipeline/types"` |
| `cb` | `cbtypes.Project` | `cbtypes "...codebuild/types"` |
| `ecr` | `ecrtypes.Repository` | `ecrtypes "...ecr/types"` |
| `codeartifact` | `codeartifacttypes.RepositorySummary` | `codeartifacttypes "...codeartifact/types"` |

### DATA & ANALYTICS (2 types)

| ShortName | SDK Type | Package Import Alias |
|-----------|----------|---------------------|
| `glue` | `gluetypes.Job` | `gluetypes "...glue/types"` |
| `athena` | `athenatypes.WorkGroupSummary` | `athenatypes "...athena/types"` |

### BACKUP (2 types)

| ShortName | SDK Type | Package Import Alias |
|-----------|----------|---------------------|
| `backup` | `backuptypes.BackupPlansListMember` | `backuptypes "...backup/types"` |
| `ses` | `sesv2types.IdentityInfo` | `sesv2types "...sesv2/types"` |

### SUB-RESOURCES (2 types)

| ShortName | SDK Type | Package Import Alias | Notes |
|-----------|----------|---------------------|-------|
| `s3_objects` (file) | `s3types.Object` | (same as s3) | Already done |
| `s3_objects` (folder) | `s3types.CommonPrefix` | (same as s3) | Already done |
| `r53_records` | `r53types.ResourceRecordSet` | (same as r53) | New |

---

## Special Cases

### 1. SQS: String RawStruct

The production SQS fetcher (`internal/aws/sqs.go:69`) sets:
```go
RawStruct: fmt.Sprintf("%v", attrs),
```
This is a `string`, not an SDK struct. Config-driven field extraction (fieldpath) does NOT
work on strings. The demo fixture must match production behavior:

```go
RawStruct: "map[ApproximateNumberOfMessages:5 ApproximateNumberOfMessagesNotVisible:2 DelaySeconds:0 ...]",
```

This is a known deficiency tracked separately. The demo should match reality, not paper
over it.

### 2. KMS: Pointer RawStruct

The KMS fetcher (`internal/aws/kms.go:108`) sets `RawStruct: meta` where `meta` is
`*kmstypes.KeyMetadata` (a pointer to the struct from `DescribeKeyOutput.KeyMetadata`).
The demo fixture must also use a pointer:

```go
RawStruct: &kmstypes.KeyMetadata{...},
```

### 3. EKS: Pointer RawStruct

Similarly, `internal/aws/eks.go:77` sets `RawStruct: cluster` where `cluster` is
`*ekstypes.Cluster` (from `DescribeClusterOutput.Cluster`). Demo must use a pointer.

### 4. R53 Records Sub-Resource

R53 records are fetched per hosted zone, like S3 objects per bucket. The demo needs a
new top-level function:

```go
func GetR53Records(zoneId string) ([]resource.Resource, bool)
```

The zone ID must match one of the demo R53 hosted zone fixtures.

### 5. Import Count Per File

The `fixtures_networking.go` file will have the most types (12), but they all use
`ec2types` (plus `elbv2types`), so only 2-3 SDK imports needed. No file requires more
than ~5 SDK type package imports. This is manageable.

---

## Fixture Data Guidelines

Each resource type should provide **3-6 fixtures** that demonstrate:

1. **Status variety** -- At least 2 distinct statuses where the resource type has one
   (e.g., running/stopped for EC2, available/creating for RDS, ACTIVE/DELETING for EKS).
   Resources without meaningful status (S3, SNS, Secrets Manager) get 0-1 status variants.

2. **Realistic naming** -- Names should feel like a real infrastructure environment:
   `prod-api-primary`, `staging-mysql`, `data-pipeline-logs`. Not `test-1`, `test-2`.

3. **Consistent demo account** -- Use account ID `123456789012`, region `us-east-1`,
   VPC `vpc-0abc123def456789a` across all fixtures for cross-type consistency.

4. **Non-trivial RawStruct fields** -- Populate enough fields for the detail view to show
   interesting data. At minimum, every field referenced by `internal/config/defaults.go`
   ViewDef paths for that type. At maximum, fill all fields the SDK struct makes
   convenient.

5. **Fields map must be consistent with RawStruct** -- The Fields map values should
   reflect what the production fetcher would derive from the RawStruct. Since demo mode
   short-circuits the fetcher, the fixture must manually ensure this consistency.

6. **Cross-type references should be consistent** -- EC2 instances should reference VPCs
   and subnets that actually appear in the VPC/subnet fixtures. EKS node groups should
   reference EKS clusters that exist. ECS services should reference ECS clusters that
   exist. This makes the demo experience coherent.

### Demo Scenario: "Acme Corp Production"

All fixtures tell the story of a single AWS account running a mid-size production
workload:

- **2 VPCs**: `prod` (10.0.0.0/16) and `staging` (10.1.0.0/16)
- **1 EKS cluster**: `acme-prod` with 2 node groups
- **2 ECS clusters**: `acme-services` and `acme-batch`
- **6 Lambda functions**: mix of runtimes (Python, Node, Go, Java)
- **5 RDS instances**: Aurora PostgreSQL primary + replica, standalone MySQL, etc.
- **2 ElastiCache clusters**: prod Redis, staging Redis
- **1 DocumentDB cluster**: `acme-docdb-prod`
- Matching security groups, load balancers, target groups, secrets, etc.

---

## How Tests Consume Demo Fixtures

### Pattern 1: Direct Import (Recommended for New Tests)

```go
package unit // or unit_test

import (
    demo "github.com/k2m30/a9s/internal/demo"
    "github.com/k2m30/a9s/internal/resource"
)

func TestSomething(t *testing.T) {
    resources, ok := demo.GetResources("vpc")
    if !ok {
        t.Skip("no demo fixtures for vpc")
    }
    // Use resources[0].RawStruct, resources[0].Fields, etc.
}
```

### Pattern 2: Existing Test Fixtures Stay

Existing `fixtureS3Buckets()`, `fixtureEC2Instances()`, etc. in `fixtures_test.go`
and `realistic*()` builders in `qa_*_test.go` continue to serve their existing tests.
No mass migration. New tests should prefer demo fixtures.

### Pattern 3: Table-Driven Coverage via Demo

The `TestDetailPaths_AllConfiguredFieldsRendered` test (and the proposed
`TestQA_ListRawStruct_AllTypes` from the P1 plan) can iterate over `resource.AllShortNames()`
and call `demo.GetResources()` for each:

```go
func TestDetailPaths_AllTypes(t *testing.T) {
    for _, shortName := range resource.AllShortNames() {
        t.Run(shortName, func(t *testing.T) {
            resources, ok := demo.GetResources(shortName)
            if !ok {
                t.Skipf("no demo fixture for %q", shortName)
            }
            res := resources[0]
            // Test detail path extraction from res.RawStruct...
        })
    }
}
```

This means: once all 62 demo fixtures exist, **all table-driven tests automatically
gain coverage** without any per-type test code.

---

## Migration Plan for fixtures.go

### Step 1: Refactor fixtures.go Registry

Keep `fixtures.go` as the registry (GetResources, GetS3Objects, demoData map,
mustParseTime). Move the existing S3 bucket, S3 object, Lambda, and RDS fixture
functions OUT of `fixtures.go` into their respective category files:

- `s3Buckets()` and `s3Objects()` --> `fixtures_databases.go`
- `lambdaFunctions()` --> `fixtures_compute.go`
- `rdsInstances()` --> `fixtures_databases.go`

### Step 2: Rename fixtures_ec2.go

Rename `fixtures_ec2.go` to `fixtures_compute.go`. Add ECS, Lambda (moved from
fixtures.go), ASG, and EB fixtures to this file.

### Step 3: Create Category Files

Create the remaining 10 category files with `init()` registration and fixture functions.

### Step 4: Add GetR53Records()

Add `GetR53Records(zoneId string) ([]resource.Resource, bool)` to `fixtures.go`
alongside `GetS3Objects()`.

### Step 5: Wire into Demo Mode

`internal/tui/app.go:fetchDemoResources()` already calls `demo.GetResources(canonicalType)`.
Once all 62 types are registered in `demoData`, demo mode automatically works for all
types. No changes to `app.go` needed (the function returns `nil, false` for unknown
types and the UI shows an empty list -- which becomes a populated list once the fixture
is registered).

The R53 records drill-down in `app.go` (line 393, 748) will need a small change to call
`demo.GetR53Records()` when in demo mode, similar to the S3 objects path.

---

## Implementation Order

### Phase 1: Restructure (no new fixtures)

1. Create `fixtures_compute.go` -- move `ec2Instances()` + helpers from `fixtures_ec2.go`,
   move `lambdaFunctions()` from `fixtures.go`
2. Create `fixtures_databases.go` -- move `s3Buckets()`, `s3Objects()`, `rdsInstances()`
   from `fixtures.go`
3. Delete `fixtures_ec2.go` (content moved)
4. Verify: `go test ./tests/unit/ -run TestDemo -count=1` -- all existing demo tests pass
5. Verify: `go build ./cmd/a9s/` -- binary builds

### Phase 2: High-Value Types (15 types, unblock major demo gaps)

Priority order based on typical AWS user workflows:

| Batch | Types | File |
|-------|-------|------|
| 2a | `vpc`, `sg`, `subnet` | `fixtures_networking.go` |
| 2b | `redis`, `dbc`, `eks`, `ng` | `fixtures_databases.go` + `fixtures_containers.go` |
| 2c | `alarm`, `logs`, `secrets` | `fixtures_monitoring.go` + `fixtures_secrets.go` |
| 2d | `role`, `cfn` | `fixtures_security.go` + `fixtures_cicd.go` |
| 2e | `sqs`, `sns` | `fixtures_messaging.go` |

### Phase 3: Remaining Types (42 types)

Fill in the remaining types by category file. Each category file can be implemented as
one unit of work:

| File | Types to Add |
|------|-------------|
| `fixtures_compute.go` | `ecs-svc`, `ecs`, `ecs-task`, `asg`, `eb` |
| `fixtures_networking.go` | `elb`, `tg`, `rtb`, `nat`, `igw`, `eip`, `vpce`, `tgw`, `eni` |
| `fixtures_databases.go` | `ddb`, `opensearch`, `redshift`, `efs`, `rds-snap`, `docdb-snap` |
| `fixtures_monitoring.go` | `trail` |
| `fixtures_messaging.go` | `sns-sub`, `eb-rule`, `kinesis`, `msk`, `sfn` |
| `fixtures_secrets.go` | `ssm`, `kms` |
| `fixtures_dns_cdn.go` | `r53`, `cf`, `acm`, `apigw` |
| `fixtures_security.go` | `policy`, `iam-user`, `iam-group`, `waf` |
| `fixtures_cicd.go` | `pipeline`, `cb`, `ecr`, `codeartifact` |
| `fixtures_data.go` | `glue`, `athena` |
| `fixtures_backup.go` | `backup`, `ses` |

### Phase 4: Sub-Resources and Cross-References

1. Add `r53_records` fixtures to `fixtures_dns_cdn.go`
2. Add `GetR53Records()` to `fixtures.go`
3. Wire R53 records into `app.go` demo path
4. Audit cross-type references for consistency (VPC IDs, cluster names, etc.)

### Phase 5: Test Integration

1. Extend `TestAllDemoResourcesHaveFieldKeys` to cover all 62 types (currently 4)
2. Add table-driven `TestDetailPaths_AllTypes` using demo fixtures (replaces the
   59-skip problem in `qa_detail_paths_test.go`)
3. Add table-driven `TestDemoRawStruct_AllTypes` verifying every fixture has non-nil
   RawStruct of the correct SDK type

---

## Verification Criteria

For each resource type added to demo, these must be true:

1. `demo.GetResources("shortname")` returns `([]resource.Resource, true)` with 3+ items
2. Every resource has non-nil `RawStruct` of the correct SDK type (per the table above)
3. Every resource has all Fields keys matching `resource.GetFieldKeys("shortname")`
4. `RawStruct` field values are consistent with `Fields` map values
5. At least 2 distinct `Status` values (for types that have status)
6. `fieldpath.ToSafeValue(reflect.ValueOf(r.RawStruct))` succeeds (YAML-marshalable)
7. Existing tests do not regress

---

## Import Budget Per File

| File | AWS SDK Imports |
|------|:---------------:|
| `fixtures_compute.go` | 4 (ec2, ecs, lambda, autoscaling + elasticbeanstalk) |
| `fixtures_containers.go` | 1 (eks) |
| `fixtures_networking.go` | 2 (ec2, elbv2) |
| `fixtures_databases.go` | 6 (rds, s3, elasticache, docdb, dynamodb, opensearch + redshift, efs) |
| `fixtures_monitoring.go` | 3 (cloudwatch, cloudwatchlogs, cloudtrail) |
| `fixtures_messaging.go` | 5 (sns, eventbridge, kinesis, kafka, sfn) |
| `fixtures_secrets.go` | 3 (secretsmanager, ssm, kms) |
| `fixtures_dns_cdn.go` | 4 (route53, cloudfront, acm, apigatewayv2) |
| `fixtures_security.go` | 2 (iam, wafv2) |
| `fixtures_cicd.go` | 5 (cloudformation, codepipeline, codebuild, ecr, codeartifact) |
| `fixtures_data.go` | 2 (glue, athena) |
| `fixtures_backup.go` | 2 (backup, sesv2) |

No file exceeds 6 SDK imports. The `aws` core package (`"github.com/aws/aws-sdk-go-v2/aws"`)
is needed in all files for `aws.String()`, `aws.Int32()`, etc.

---

## Estimated LOC Per Category File

| File | Types | Est. LOC |
|------|:-----:|--------:|
| `fixtures_compute.go` | 7 | ~500 |
| `fixtures_containers.go` | 2 | ~150 |
| `fixtures_networking.go` | 12 | ~650 |
| `fixtures_databases.go` | 10 | ~600 |
| `fixtures_monitoring.go` | 3 | ~200 |
| `fixtures_messaging.go` | 7 | ~400 |
| `fixtures_secrets.go` | 3 | ~200 |
| `fixtures_dns_cdn.go` | 4+1 | ~300 |
| `fixtures_security.go` | 5 | ~300 |
| `fixtures_cicd.go` | 5 | ~300 |
| `fixtures_data.go` | 2 | ~150 |
| `fixtures_backup.go` | 2 | ~150 |
| **Total** | **62+2** | **~3900** |

---

## Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| Large PR (3900+ lines of fixture data) | Phase the work: Phase 1 (restructure) is a separate PR. Phase 2 (high-value 15 types) is a second PR. Phase 3 (remaining 42) can be batched by category file. |
| Fixture data drifts from fetcher behavior | Add a test that iterates all 62 types, calls `demo.GetResources()`, and verifies Fields keys match `resource.GetFieldKeys()`. This test will catch drift on every `go test` run. |
| SQS string RawStruct is confusing for tests | Document the special case clearly. Consider fixing the SQS fetcher to use a proper struct in a future PR (out of scope here). |
| Cross-type reference inconsistency | Define shared constants in `fixtures.go`: `demoVPCID`, `demoSubnetID`, `demoEKSClusterName`, etc. All category files import these. |
| go.sum changes from new SDK imports | The demo package already imports `s3/types`, `ec2/types`, `lambda/types`, `rds/types`. Most additional SDK type packages are already in `go.sum` via `internal/aws/` and `cmd/refgen/`. No new top-level `go get` should be needed. |
