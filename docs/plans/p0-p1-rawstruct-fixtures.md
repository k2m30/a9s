# Plan: P0/P1 RawStruct Test Coverage

**Author:** a9s-architect
**Date:** 2026-03-21
**Status:** Draft

---

## Correction to Problem Statement

The original request states "34 skipped tests in qa_redis_docdb_test.go." This is inaccurate.
Running `go test ./tests/unit/ -run "TestQA_Redis|TestQA_DocDB" -v` shows all 63 tests
in qa_redis_docdb_test.go **pass**. The `len(fixtures) == 0` guards are safety nets that
never trigger because `fixtureRedisClusters()` and `fixtureDocDBClusters()` always return
non-empty slices.

The actual 59 skipped tests in the suite are subtests of `TestDetailPaths_AllConfiguredFieldsRendered`
in `qa_detail_paths_test.go`, which skip because the `allFixtures` map only contains 8 entries
(s3, s3_objects, ec2, dbi, redis, dbc, eks, secrets) and 54 resource types have no fixture
function mapped.

**The real P0 issue:** The fixtures in `fixtures_test.go` lack `RawStruct`, meaning tests that
USE those fixtures don't exercise the config-driven (RawStruct-based) rendering path. They only
test the Fields-fallback path. This is a silent correctness gap, not a skip problem.

---

## P0: Add RawStruct to Redis/DocDB fixtures in fixtures_test.go

### Goal

Add `RawStruct` fields to `fixtureRedisClusters()` and `fixtureDocDBClusters()` so that
tests using these fixtures exercise the config-driven rendering path (fieldpath extraction
from RawStruct) rather than only the Fields-map fallback.

### SDK Types

From the production fetchers:

| Fixture Function | Fetcher File | SDK Type | Loop Variable |
|-----------------|-------------|----------|---------------|
| `fixtureRedisClusters()` | `internal/aws/redis.go` | `elasticachetypes.CacheCluster` | `cluster` (line 37) |
| `fixtureDocDBClusters()` | `internal/aws/docdb.go` | `docdbtypes.DBCluster` | `cluster` (line 34) |

### Files to Modify

**`tests/unit/fixtures_test.go`** (package `unit`)

1. Add imports:
   ```go
   "github.com/aws/aws-sdk-go-v2/aws"
   elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
   docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
   ```

2. In `fixtureRedisClusters()`, add `RawStruct` to the single resource (line 209-223).
   The struct must match what `FetchRedisClusters` produces -- an `elasticachetypes.CacheCluster`
   value with fields consistent with the existing `Fields` map:

   ```go
   RawStruct: elasticachetypes.CacheCluster{
       CacheClusterId:     aws.String("test-redis-1"),
       Engine:             aws.String("redis"),
       EngineVersion:      aws.String("7.0.7"),
       CacheNodeType:      aws.String("cache.t2.micro"),
       CacheClusterStatus: aws.String("available"),
       NumCacheNodes:      aws.Int32(1),
       // ConfigurationEndpoint is nil (matches empty endpoint in Fields)
   },
   ```

3. In `fixtureDocDBClusters()`, add `RawStruct` to both resources (lines 229-254).

   First resource (test-docdb-cluster):
   ```go
   RawStruct: docdbtypes.DBCluster{
       DBClusterIdentifier: aws.String("test-docdb-cluster"),
       EngineVersion:       aws.String("5.0.0"),
       Status:              aws.String("available"),
       Endpoint:            aws.String("test-docdb-cluster.cluster-abc123def.us-east-1.docdb.amazonaws.com"),
       DBClusterMembers: []docdbtypes.DBClusterMember{
           {DBInstanceIdentifier: aws.String("test-docdb-instance-1"), IsClusterWriter: aws.Bool(true)},
       },
   },
   ```

   Second resource (test-rds-cluster):
   ```go
   RawStruct: docdbtypes.DBCluster{
       DBClusterIdentifier: aws.String("test-rds-cluster"),
       EngineVersion:       aws.String("16.8"),
       Status:              aws.String("available"),
       Endpoint:            aws.String("test-rds-cluster.cluster-abc123def.us-east-1.rds.amazonaws.com"),
       DBClusterMembers: []docdbtypes.DBClusterMember{
           {DBInstanceIdentifier: aws.String("test-rds-instance-1"), IsClusterWriter: aws.Bool(true)},
       },
   },
   ```

### Package Boundary Note

`fixtures_test.go` uses `package unit` (white-box). The `ptrString`, `ptrBool`, `ptrInt32`
helpers only exist in `package unit_test` (in `qa_configurable_views_test.go`), so they
are NOT accessible from `fixtures_test.go`.

**Use `aws.String()`, `aws.Bool()`, `aws.Int32()` from `"github.com/aws/aws-sdk-go-v2/aws"`
instead.** This is the same pattern used by all other `package unit` test files
(e.g., `aws_vpc_test.go`, `aws_secrets_test.go`).

### Verification

After the change, run:
```
go test ./tests/unit/ -run "TestQA_Redis|TestQA_DocDB" -v -count=1
```
All 63 tests should still pass. Additionally, the YAML view tests
(`TestQA_Redis_YAMLView`, `TestQA_DocDB_YAMLView`) will now render from
RawStruct instead of Fields, so the YAML output will contain SDK struct field
names (e.g., `CacheClusterId`) instead of flat map keys (e.g., `cluster_id`).

**This WILL break YAML assertions.** The YAML view (`internal/tui/views/yaml.go` line
140-145) renders from `RawStruct` when present, falling back to `Fields` only when
`RawStruct` is nil. Adding `RawStruct` to fixtures changes the YAML output from
flat field keys (e.g., `cluster_id`, `engine_version`) to SDK struct field names
(e.g., `CacheClusterId`, `EngineVersion`).

Affected tests that WILL break:
- `TestQA_Redis_YAMLView` (line 517-527) -- asserts `res.Fields` keys like "cluster_id"
- `TestQA_Redis_YAMLRawContent` (line 553-573) -- asserts `res.Fields` keys
- `TestQA_DocDB_YAMLView` (line ~1175) -- same pattern
- `TestQA_DocDB_YAMLRawContent` (line ~1226) -- same pattern

The coder MUST update these 4 tests to assert SDK struct field names instead:
- Redis: `CacheClusterId`, `EngineVersion`, `CacheNodeType`, `CacheClusterStatus`, `NumCacheNodes`
- DocDB: `DBClusterIdentifier`, `EngineVersion`, `Status`, `Endpoint`, `DBClusterMembers`

This is actually an improvement -- it means the tests now exercise the real production
rendering path instead of the fallback path.

### Also update `multiStatusRedisFixtures()` and `multiStatusDocDBFixtures()`

These helper fixtures in `qa_redis_docdb_test.go` (package `unit`) also lack RawStruct.
Add minimal `elasticachetypes.CacheCluster` / `docdbtypes.DBCluster` structs for
consistency, but note this is lower priority since these are only used for visual tests.

---

## P1: Extend RawStruct List Tests to Cover All Resource Types

### Current State

`qa_list_rawstruct_test.go` (package `unit_test`) covers 7 types:
EC2, RDS (dbi), Redis, DocDB (dbc), EKS, Secrets, S3.

Each type has 2 test functions:
1. `TestQA_ListRawStruct_{Type}` -- verifies RawStruct values appear in list view
2. `TestQA_ListRawStruct_{Type}_RawStructOverridesFields` -- verifies Fields are NOT used when RawStruct is present

Plus a `TestQA_ListRawStruct_WithProductionViewsYAML` with 7 subtests.

### Target State

All 62 resource types should have RawStruct list coverage. S3 objects (`s3_objects`) should
also be covered as a 63rd entry.

### Approach: Table-Driven Extension

Rather than 55 new individual test functions, extend the existing pattern with a
table-driven test that covers all types. This is the approach the architect recommends.

#### New Test: `TestQA_ListRawStruct_AllTypes`

A single table-driven test in `qa_list_rawstruct_test.go` with this structure:

```go
func TestQA_ListRawStruct_AllTypes(t *testing.T) {
    ensureNoColor(t)

    tests := []struct {
        shortName   string
        rawStruct   interface{}
        expectInView []string  // values that MUST appear from RawStruct
    }{
        // -- Already covered individually, but included for completeness --
        {"ec2", realisticEC2Instance(), []string{"i-0abcdef1234567890", "running", "t3.medium"}},
        {"dbi", realisticRDSInstance(), []string{"prod-db-01", "mysql", "available"}},
        // ... etc for all 7 existing types ...

        // -- New types --
        {"lambda", realisticLambdaFunction(), []string{"my-function", "python3.12"}},
        {"vpc", realisticVPC(), []string{"vpc-0abc1234", "10.0.0.0/16"}},
        // ... one entry per remaining type ...
    }

    for _, tc := range tests {
        t.Run(tc.shortName, func(t *testing.T) {
            cfg := configForType(tc.shortName)
            res := resource.Resource{
                ID:        "test-id",
                Name:      "test-name",
                RawStruct: tc.rawStruct,
            }
            view := newListModel(t, tc.shortName, cfg, []resource.Resource{res})

            for _, expected := range tc.expectInView {
                if !strings.Contains(view, expected) {
                    t.Errorf("%s list should contain %q from RawStruct, got:\n%s",
                        tc.shortName, expected, view)
                }
            }
        })
    }
}
```

#### New Test: `TestQA_ListRawStruct_AllTypes_RawStructOverridesFields`

Same table but with `WRONG-*` Fields values to verify RawStruct takes priority:

```go
func TestQA_ListRawStruct_AllTypes_OverridesFields(t *testing.T) {
    // Similar structure with Fields set to "WRONG-*" values
}
```

### Realistic Fixture Builder Inventory

62 `realistic*()` builders already exist across 4 files (all in `package unit_test`):

| File | Types Covered |
|------|-------------|
| `qa_configurable_views_test.go` | s3 (Bucket), s3 (Object, CommonPrefix), ec2, dbi, redis, dbc, eks, secrets |
| `qa_detail_ec2_family_test.go` | vpc, sg, ng, subnet, rtb, nat, igw, eip, tgw, vpce, eni, rds-snap, docdb-snap, sns-sub, policy (ManagedPolicyDetail -- WRONG TYPE) |
| `qa_detail_services_test.go` | lambda, alarm, sns, elb, tg, ecs, ecs-svc, ecs-task, cfn, role, logs, ssm, ddb, acm, asg, iam-user, iam-group |
| `qa_detail_v220_test.go` | cf, r53, apigw, ecr, efs, eb-rule, sfn, pipeline, kinesis, waf, glue, eb, redshift, trail, athena, codeartifact, cb, opensearch, kms, msk, backup |

### SDK Type Mapping for All 62 Types

| ShortName | Fixture Builder | SDK Type |
|-----------|----------------|----------|
| ec2 | `realisticEC2Instance()` | `ec2types.Instance` |
| ecs-svc | `realisticECSService()` | `ecstypes.Service` |
| ecs | `realisticECSClusterStruct()` | `ecstypes.Cluster` |
| ecs-task | `realisticECSTask()` | `ecstypes.Task` |
| lambda | `realisticLambdaFunction()` | `lambdatypes.FunctionConfiguration` |
| asg | `realisticASG()` | `autoscalingtypes.AutoScalingGroup` |
| eb | `realisticEB()` | `ebtypes.EnvironmentDescription` |
| eks | `realisticEKSCluster()` | `*ekstypes.Cluster` (pointer!) |
| ng | `realisticNodeGroup()` | `ekstypes.Nodegroup` |
| elb | `realisticELB()` | `elbv2types.LoadBalancer` |
| tg | `realisticTargetGroup()` | `elbv2types.TargetGroup` |
| sg | `realisticSecurityGroup()` | `ec2types.SecurityGroup` |
| vpc | `realisticVPC()` | `ec2types.Vpc` |
| subnet | `realisticSubnet()` | `ec2types.Subnet` |
| rtb | `realisticRouteTable()` | `ec2types.RouteTable` |
| nat | `realisticNATGateway()` | `ec2types.NatGateway` |
| igw | `realisticInternetGateway()` | `ec2types.InternetGateway` |
| eip | `realisticEIP()` | `ec2types.Address` |
| vpce | `realisticVPCEndpoint()` | `ec2types.VpcEndpoint` |
| tgw | `realisticTransitGateway()` | `ec2types.TransitGateway` |
| eni | `realisticENI()` | `ec2types.NetworkInterface` |
| dbi | `realisticRDSInstance()` | `rdstypes.DBInstance` |
| s3 | `realisticS3Bucket()` | `s3types.Bucket` |
| redis | `realisticRedisCacheCluster()` | `elasticachetypes.CacheCluster` |
| dbc | `realisticDocDBCluster()` | `docdbtypes.DBCluster` |
| ddb | `realisticDDBTable()` | `ddbtypes.TableDescription` |
| opensearch | `realisticOpenSearch()` | `opensearchtypes.DomainStatus` |
| redshift | `realisticRedshift()` | `redshifttypes.Cluster` |
| efs | `realisticEFS()` | `efstypes.FileSystemDescription` |
| rds-snap | `realisticRDSSnapshot()` | `rdstypes.DBSnapshot` |
| docdb-snap | `realisticDocDBSnapshot()` | `docdbtypes.DBClusterSnapshot` |
| alarm | `realisticAlarm()` | `cwtypes.MetricAlarm` |
| logs | `realisticLogGroup()` | `cwlogstypes.LogGroup` |
| trail | `realisticTrail()` | `cloudtrailtypes.Trail` |
| sqs | N/A -- see note | `fmt.Sprintf("%v", attrs)` (string, NOT a struct) |
| sns | `realisticSNSTopic()` | `snstypes.Topic` |
| sns-sub | `realisticSNSSubscription()` | `snstypes.Subscription` |
| eb-rule | `realisticEBRule()` | `eventbridgetypes.Rule` |
| kinesis | `realisticKinesis()` | `kinesistypes.StreamSummary` |
| msk | `realisticMSK()` | `kafkatypes.Cluster` |
| sfn | `realisticSFN()` | `sfntypes.StateMachineListItem` |
| secrets | `realisticSecretListEntry()` | `smtypes.SecretListEntry` |
| ssm | `realisticSSMParameter()` | `ssmtypes.ParameterMetadata` |
| kms | `realisticKMS()` | `*kmstypes.KeyMetadata` (pointer!) |
| r53 | `realisticR53Zone()` | `route53types.HostedZone` |
| cf | `realisticCFDistribution()` | `cloudfronttypes.DistributionSummary` |
| acm | `realisticACMCertificate()` | `acmtypes.CertificateSummary` |
| apigw | `realisticAPIGW()` | `apigatewayv2types.Api` |
| role | `realisticIAMRole()` | `iamtypes.Role` |
| policy | `realisticManagedPolicyDetail()` | **BUG: uses `iamtypes.ManagedPolicyDetail` but fetcher uses `iamtypes.Policy`** |
| iam-user | `realisticIAMUser()` | `iamtypes.User` |
| iam-group | `realisticIAMGroup()` | `iamtypes.Group` |
| waf | `realisticWAF()` | `wafv2types.WebACLSummary` |
| cfn | `realisticCFNStack()` | `cfntypes.Stack` |
| pipeline | `realisticPipeline()` | `codepipelinetypes.PipelineSummary` |
| cb | `realisticCodeBuild()` | `codebuildtypes.Project` |
| ecr | `realisticECR()` | `ecrtypes.Repository` |
| codeartifact | `realisticCodeArtifact()` | `codeartifacttypes.RepositorySummary` |
| glue | `realisticGlueJob()` | `gluetypes.Job` |
| athena | `realisticAthena()` | `athenatypes.WorkGroupSummary` |
| backup | `realisticBackup()` | `backuptypes.BackupPlansListMember` |
| ses | N/A -- see note | `sesv2types.EmailIdentity` (from `ListEmailIdentitiesOutput`) |

### Known Issues to Fix During P1

1. **policy fixture type mismatch:** `realisticManagedPolicyDetail()` returns
   `iamtypes.ManagedPolicyDetail` but `internal/aws/iam_policies.go` produces
   `iamtypes.Policy`. The fixture must be rewritten to return `iamtypes.Policy`.

2. **sqs has string RawStruct:** The SQS fetcher sets `RawStruct` to
   `fmt.Sprintf("%v", attrs)` (a string), not an SDK struct. Config-driven path
   extraction will not work for SQS. The list test for SQS should either:
   - Skip the RawStruct-overrides-Fields check, or
   - Use a string RawStruct consistent with the fetcher behavior.

3. **ses has no existing fixture builder.** A new `realisticSESIdentity()` function
   must be created returning `sesv2types.EmailIdentity`.

4. **s3_objects has two struct types.** Files use `s3types.Object`, folders use
   `s3types.CommonPrefix`. Both existing builders exist. The test should cover both.

5. **r53 records** are a sub-resource like s3_objects. If a `r53_records` type exists
   in views.yaml, it needs coverage too.

### Files to Create/Modify

| File | Action |
|------|--------|
| `tests/unit/qa_list_rawstruct_test.go` | Add `TestQA_ListRawStruct_AllTypes` and `TestQA_ListRawStruct_AllTypes_OverridesFields` |
| `tests/unit/qa_detail_ec2_family_test.go` | Fix `realisticManagedPolicyDetail()` to return `iamtypes.Policy` (rename to `realisticIAMPolicy()`) |
| `tests/unit/qa_detail_services_test.go` or new file | Add `realisticSESIdentity()` returning `sesv2types.EmailIdentity` |

### Import Requirements for Table-Driven Test

The test file already imports 7 SDK type packages. To cover all 62 types it will need
approximately 30 SDK type imports. This is acceptable for a comprehensive test file.
The imports needed beyond current ones:

```go
import (
    autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
    ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
    ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
    lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
    elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
    cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
    iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
    cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
    cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
    ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
    ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
    acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
    route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
    cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
    apigatewayv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
    wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
    codepipelinetypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
    codebuildtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
    ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
    codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
    gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
    athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
    backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
    cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
    efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
    eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
    kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
    kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
    kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
    opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
    redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
    sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
    snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
    sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)
```

### Also Extend TestDetailPaths_AllConfiguredFieldsRendered (bonus)

`qa_detail_paths_test.go` has an `allFixtures` map with only 8 entries, causing 54 skipped
subtests. This should be extended to map ALL resource types to their realistic fixture
builders. This directly unblocks those 54 skips (plus the alarm subtest skip).

| File | Action |
|------|--------|
| `tests/unit/qa_detail_paths_test.go` | Expand `allFixtures` map from 8 entries to 62 |

The fixture functions are all in `package unit_test` files but `qa_detail_paths_test.go`
is `package unit`. This is a **package boundary conflict** -- the `allFixtures` map
cannot call functions from `unit_test`.

**Resolution options:**
(a) Move `qa_detail_paths_test.go` to `package unit_test`, OR
(b) Create wrapper fixtures in `package unit` that duplicate the realistic structs, OR
(c) Convert `TestDetailPaths_AllConfiguredFieldsRendered` to generate fixtures inline
    rather than calling named functions.

Option (a) is cleanest.

---

## Execution Order

1. **P0 first** -- Modify `fixtures_test.go` to add RawStruct to Redis/DocDB fixtures.
   Verify with existing tests. Fix any YAML assertion breakage.

2. **P1a** -- Fix the `realisticManagedPolicyDetail()` type mismatch (policy). Create
   `realisticSESIdentity()`.

3. **P1b** -- Add `TestQA_ListRawStruct_AllTypes` and `AllTypes_OverridesFields` table-driven
   tests.

4. **P1c** (bonus) -- Extend `TestDetailPaths_AllConfiguredFieldsRendered` to cover all types
   by moving it to `package unit_test` and populating the full fixture map.

---

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Adding RawStruct to fixtures changes YAML view output | Check `views.NewYAML()` rendering logic before committing. Update assertions. |
| Policy fixture type mismatch causes test failures | Fix the fixture builder to use `iamtypes.Policy` before adding to table-driven test. |
| SQS string RawStruct breaks fieldpath extraction | Handle SQS as a special case in the test; verify no production regression. |
| 30+ SDK imports make the test file unwieldy | Acceptable tradeoff for comprehensive coverage. Alternative: split into multiple files by category. |
| Package boundary (unit vs unit_test) | P0 fixtures are in `package unit`; P1 realistic builders are in `package unit_test`. Keep each test file in the package where its dependencies live. |
