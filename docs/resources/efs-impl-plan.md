---
shortName: efs
derivedFrom: docs/resources/efs.md
---

# efs — Implementation Plan

Derived from `docs/resources/efs.md` + user decisions captured in its §6 Citations.

## 1. Behavioral test spec (pseudocode)

All tests operate on the same renderer pipeline `./a9s --demo` drives. "GIVEN" describes the raw AWS-API state the typed fakes serve; "WHEN" always means `FetchEFSFileSystemsPage` → enricher → `OpenList` / `OpenDetail`.

### Baseline

```text
TEST: efs_available_silence
GIVEN: a FileSystemDescription with LifeCycleState = "available", NumberOfMountTargets = 2, and DescribeMountTargets returning 2 MTs both LifeCycleState="available".
WHEN:  the list is fetched and rendered
THEN:
  - row color = green
  - S4 Status cell is blank (NOT "available", "OK", "-")
  - no `!` / `~` glyph on the name
  - Resource.Issues = []
  - EnrichmentFinding absent for this FS
  - S1 badge does not count this instance
```

### Wave 1 single signals

```text
TEST: efs_lifecycle_creating_warning
GIVEN: FileSystemDescription.LifeCycleState = "creating"; NumberOfMountTargets = 0 is EXPECTED of "creating" but the test isolates the lifecycle signal, so set NumberOfMountTargets = 1 and DescribeMountTargets returns one MT LifeCycleState="creating" as well (does not matter — phase 1 only asserts Wave-1 pre-enrichment fetcher output).
WHEN:  fetcher runs
THEN:
  - Resource.Status = "creating"
  - Resource.Fields["status"] = "creating"
  - Resource.Issues = ["creating"]
  - Color bucket = Warning (yellow)

TEST: efs_lifecycle_updating_warning
GIVEN: LifeCycleState = "updating"; NumberOfMountTargets = 1 (isolate signal)
THEN: Resource.Status = "updating"; Issues = ["updating"]; Warning.

TEST: efs_lifecycle_deleting_warning
GIVEN: LifeCycleState = "deleting"; NumberOfMountTargets = 1
THEN: Resource.Status = "deleting"; Issues = ["deleting"]; Warning.

TEST: efs_lifecycle_error_broken
GIVEN: LifeCycleState = "error"; NumberOfMountTargets = 1
THEN: Resource.Status = "error"; Issues = ["error"]; Broken.

TEST: efs_no_mount_targets_broken
GIVEN: LifeCycleState = "available"; NumberOfMountTargets = 0
THEN: Resource.Status = "no mount targets"; Issues = ["no mount targets"]; Broken.
```

### Wave 2 signal

```text
TEST: efs_mount_target_down_on_healthy_fs
GIVEN:
  - FileSystemDescription.LifeCycleState = "available", NumberOfMountTargets = 2.
  - DescribeMountTargets returns MT-A (LifeCycleState="available", AZ="us-east-1a") and MT-B (LifeCycleState="creating", AZ="us-east-1b").
WHEN:  fetcher then enricher run
THEN:
  - Fetcher: Resource.Status = ""; Issues = [].
  - Enricher produces ONE EnrichmentFinding for this FS with
      Severity = "!"
      Summary  = "mount target down"              (exact §4 "list text" phrase; ≤ 40 chars)
      Rows     = [{Label:"Mount Target", Value:"fsmt-...-B", Tier:"!"},
                  {Label:"AZ",           Value:"us-east-1b"},
                  {Label:"State",        Value:"creating",   Tier:"!"},
                  {Label:"Degraded",     Value:"1/2"}]
      !! Summary MUST NOT contain any Row Value as a substring (U11).
  - FieldUpdates[fs-id]["status"] = "mount target down"
  - Row color = Broken (red) because Fields["status"] post-merge reads "mount target down".
  - NO glyph on the row (non-green; rule 3).
  - S5 Attention section: primary entry "! Mount target down" with Rows indented beneath.
```

### Multi-finding (rule 7, mandatory per Universal coverage matrix)

```text
TEST: efs_multi_w1_no_mounts_plus_deleting_suffix   (U7a — covers S4 suffix AND Issues ordering)
GIVEN: LifeCycleState = "deleting"; NumberOfMountTargets = 0.
WHEN:  fetcher
THEN:
  - active W1 signals (§4 table order): "deleting" (Warning), "no mount targets" (Broken).
  - §4 precedence (severity first, then table order): Broken > Warning → "no mount targets" is top.
  - Resource.Status = "no mount targets (+1)"
  - Resource.Fields["status"] = "no mount targets (+1)"
  - Resource.Issues = ["no mount targets", "deleting"]   (precedence order — Broken first, then hidden Warning)
  - Color bucket = Broken (after StripFindingSuffix).

TEST: efs_w1_plus_w2_bumps_suffix   (U7b — EFS-specific adaptation)
NOTE: EFS has ONE Wave-2 signal and it is Broken severity. So W2 > most W1 Warnings.
      For a W1 Warning (e.g. "updating") + W2 Broken, the Broken W2 phrase WINS S4 per
      severity rule, and "updating" is the hidden entry — opposite of the generic U7b template.
GIVEN: LifeCycleState = "updating"; NumberOfMountTargets = 2; DescribeMountTargets returns MT-A available, MT-B creating.
WHEN:  fetcher (then enricher).
THEN:
  - Fetcher: Resource.Status = "updating", Issues = ["updating"].
  - Enricher: severity W2 = Broken > W1 Warning. Status must become the W2 phrase with +1 suffix
    counting the hidden W1 entry: FieldUpdates[id]["status"] = "mount target down (+1)".
  - Issues (Wave-1-only contract) stays = ["updating"]. The W2 finding lives in EnrichmentFinding.
  - Row color after Color(r) with Fields["status"] merged and suffix stripped → Broken.

TEST: efs_detail_surfaces_all_findings   (U7c + U7e on same fixture)
GIVEN: same fixture as efs_w1_plus_w2_bumps_suffix.
WHEN:  OpenDetailResource.
THEN:  rendered Attention (N) section contains:
  - "Mount target down"  (Wave-2 Summary, capitalized first letter by Attention renderer)
  - "Updating"           (Wave-1 phrase from Resource.Issues)
  - Rows: "Mount Target: fsmt-...-B", "AZ: us-east-1b", "State: creating", "Degraded: 1/2"
  - None of the above strings leak into Summary.

TEST: efs_detail_surfaces_every_wave1_phrase_multi   (U7e on the multi-W1 fixture)
GIVEN: efs_multi_w1_no_mounts_plus_deleting_suffix fixture.
WHEN:  OpenDetailResource.
THEN:  detail contains BOTH "No mount targets" AND "Deleting" as Attention entries.
       The "(+1)" suffix itself is NOT expected — the detail enumerates each phrase.

TEST: efs_fetcher_populates_resource_issues   (U7f — table-driven across every fixture)
FOR every §3 fixture, assert:
  - Healthy baseline           → Issues is empty/nil.
  - Single W1 fixtures          → Issues == [phrase] exactly.
  - Multi-W1 fixture            → Issues == ["no mount targets", "deleting"] in that order.
  - W2-on-Healthy fixture       → Issues is empty/nil (Wave-2 NEVER lands in Issues).
  - W1+W2 stacking fixture      → Issues == ["updating"] (only Wave-1).
```

### Rule-7 precedence — `!` beats `~`: N/A

EFS §3.2 has only one Wave-2 signal and its severity is `!`. No `~` case exists. U7d skipped with justification recorded here.

### Wave-3 anti-tests

```text
TEST: efs_out_of_scope_percentiolimit_unreachable
No CloudWatch metric call is made by the fetcher or the enricher. Unit test: mock CloudWatch client; assert zero Get* calls against CW metrics for EFS in either layer.

TEST: efs_out_of_scope_burstcreditbalance_unreachable
Same shape as above — zero CW-metric calls.
```

## 2. Fixture list (plain language; single source `internal/demo/fixtures/efs.go`)

### Healthy baseline (graph-root instance — satisfies 9.3)

```text
FIXTURE: prod-efs-app-data
A production EFS file system, healthy, serving an app tier.
  FileSystemId       = "fs-0prod1234abcd5678"
  FileSystemArn      = "arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-0prod1234abcd5678"
  Name               = "prod-app-data"
  LifeCycleState     = "available"
  NumberOfMountTargets = 3
  KmsKeyId           = "<ProdEFSKmsKeyARN>"       (must match an entry in fixtures/kms.go)
  Encrypted          = true
  PerformanceMode    = "generalPurpose"
  ThroughputMode     = "bursting"
  SizeInBytes        = { Value: 1073741824, Timestamp: <recent> }
  Tags:
    Name                            = "prod-app-data"
    Environment                     = "production"
    aws:cloudformation:stack-name   = "<ProdCFNStackName>"  (must match fixtures/cfn.go)

DescribeMountTargets(fs-0prod1234abcd5678) returns 3 MTs, all LifeCycleState="available":
  MT-A: MountTargetId="fsmt-prodA...", AZ="us-east-1a", SubnetId="<ProdSubnetA>", VpcId="<ProdVpcID>", NetworkInterfaceId="<ProdEfsEniA>"
  MT-B: ... AZ="us-east-1b", SubnetId="<ProdSubnetB>", NetworkInterfaceId="<ProdEfsEniB>"
  MT-C: ... AZ="us-east-1c", SubnetId="<ProdSubnetC>", NetworkInterfaceId="<ProdEfsEniC>"

DescribeAccessPoints(fs-0prod1234abcd5678) returns TWO access points:
  AP-A: arn:aws:elasticfilesystem:us-east-1:123456789012:access-point/fsap-prod-app-a
  AP-B: arn:aws:elasticfilesystem:us-east-1:123456789012:access-point/fsap-prod-app-b

SIBLING updates required so every §2 pivot renders Count ≥ 1 (AND ≥ 2 for 50% of pivots):
  - fixtures/eni.go: add 3 ENIs whose Description CONTAINS "fs-0prod1234abcd5678" ("EFS mount target for fs-0prod1234abcd5678"), each with:
      - matching SubnetId from the MT list
      - VpcId = "<ProdVpcID>"
      - Groups = [<ProdEFSSecurityGroupID-A>, <ProdEFSSecurityGroupID-B>]   (two SGs → pivot Count ≥ 2)
      - Attachment: NONE (mount-target ENIs are not attached to EC2).
  - fixtures/sg.go: add two security groups used above.
  - fixtures/subnet.go: add three subnets in us-east-1a/b/c, all with VpcId=<ProdVpcID>. Subnet pivot Count = 3.
  - fixtures/vpc.go: add "<ProdVpcID>" (single entry; Count = 1 — within the 50% rule because other pivots cover ≥ 2).
  - fixtures/kms.go: add the KMS key used by this FS. Count = 1.
  - fixtures/cfn.go: add a CFN stack whose Name matches the tag value. Count = 1.
  - fixtures/alarm.go: add TWO CloudWatch alarms with Namespace="AWS/EFS" and Dimensions containing FileSystemId=<fs-id>:
      * "prod-efs-burst-credit-low" (on metric BurstCreditBalance)
      * "prod-efs-percent-io-high"  (on metric PercentIOLimit)
    Alarm pivot Count = 2.
  - fixtures/lambda.go: add TWO Lambda functions each with FileSystemConfigs[].Arn matching one of the two access-point ARNs above. Lambda pivot Count = 2.
  - fixtures/ecs_task.go: add TWO ECS tasks whose (joined) task-definition has volumes[].efsVolumeConfiguration.FileSystemId = "fs-0prod1234abcd5678". ecs-task pivot Count = 2. The fixture file for ecs-task must be reshaped to expose either joined Volumes on the Resource (via a new Fields entry "efs_file_system_ids" — comma-separated) or on a sibling "TaskDefinition" field captured onto Resource.Fields during demo synthesis.
  - fixtures/backup.go (if present — otherwise typed-fake serves): add TWO recovery points whose ResourceArn matches this FS's ARN. Backup pivot Count = 2.
  - fixtures/ec2.go: DO NOT add consumer instances joined to this FS. Per §5, the EC2 pivot is intentionally empty. Count = 0 — this is the one pivot whose non-zero requirement is WAIVED by spec §5 (distinct from "count shown: unknown").
  - ct-events: windowed query — exempt (count shown: unknown).
```

### Wave-1 single-signal fixtures

```text
FIXTURE: warn-efs-creating
  FileSystemId="fs-0warncreating1", Name="provisioning-efs", LifeCycleState="creating",
  NumberOfMountTargets=1, Encrypted=true, PerformanceMode="generalPurpose", KmsKeyId=<any demo KMS key ARN>.
  DescribeMountTargets returns 1 MT in LifeCycleState="creating", AZ="us-east-1a".
  (No sibling fixtures needed — this row is not a graph-root.)

FIXTURE: warn-efs-updating
  Same shape as above with LifeCycleState="updating", NumberOfMountTargets=2, all MTs "available".

FIXTURE: warn-efs-deleting
  LifeCycleState="deleting", NumberOfMountTargets=1, MT in "deleting" state.

FIXTURE: broken-efs-error
  LifeCycleState="error", NumberOfMountTargets=1, MT in "error" state.

FIXTURE: broken-efs-no-mount-targets
  LifeCycleState="available", NumberOfMountTargets=0. No DescribeMountTargets entries.
```

### Multi-finding fixtures (rule 7)

```text
FIXTURE: warn-efs-multi (covers U7a)
  LifeCycleState="deleting", NumberOfMountTargets=0.
  DescribeMountTargets returns empty list.
  Expected Resource.Status = "no mount targets (+1)".
  Expected Resource.Issues = ["no mount targets", "deleting"].

FIXTURE: warn-efs-updating-mt-down (covers U7b, U7c, U7e-partial)
  LifeCycleState="updating", NumberOfMountTargets=2.
  DescribeMountTargets returns MT-A available (AZ us-east-1a), MT-B creating (AZ us-east-1b).
  Expected after fetch+enrich:
    Fields["status"] = "mount target down (+1)"   (W2 Broken wins over W1 Warning)
    Issues           = ["updating"]
    EnrichmentFinding.Summary = "mount target down"
    EnrichmentFinding.Rows    = [Mount Target:…, AZ:us-east-1b, State:creating, Degraded:1/2]
```

### Wave-2 on Healthy fixture

```text
FIXTURE: healthy-efs-with-mt-down  (distinct from warn-efs-updating-mt-down; covers the "~-equivalent" path for ! glyph on Healthy)
  LifeCycleState="available", NumberOfMountTargets=2.
  DescribeMountTargets returns MT-A available, MT-B creating.
  Expected Fetcher: Status="", Issues=[].
  Expected Enricher: Summary="mount target down"; Rows; FieldUpdates[id]["status"]="mount target down" (no suffix — single finding).
  Expected rendering: row is still GREEN (color reads "mount target down" which IS a Broken phrase after strip — see Color-func adaptation below).
  Update: Color function must bucket "mount target down" as BROKEN so the row RENDERS RED.
  → Therefore this fixture's end-state is: Broken row, NO glyph (rule 3), Status "mount target down".
  This fixture tests that W2-on-Healthy correctly ESCALATES the row color, NOT that `!` renders on a
  Healthy-green row. EFS has no W2 severity that lands on a green row while staying green — its
  only W2 signal is Broken.
  So U3 (~-on-Healthy) and U4 (!-on-Healthy) are N/A for EFS.
```

### Universal coverage matrix (resolved for EFS)

| ID   | Invariant | Fixture(s) | Test |
|------|-----------|-----------|------|
| U1   | Healthy blank S4 | `prod-efs-app-data` | `ExpectRowStatusBlank` |
| U2   | Warning/Broken §4 phrase | `warn-efs-creating` / `-updating` / `-deleting` / `broken-efs-error` / `broken-efs-no-mount-targets` / `healthy-efs-with-mt-down` | `ExpectRowStatusEquals` per |
| U3   | `~` on Healthy+~ finding | **N/A — EFS has no `~` Wave-2** | — |
| U4   | `!` on Healthy+! finding | **N/A — EFS's only W2 escalates to Broken, row color changes to red, rule 3 suppresses glyph** | — |
| U5   | No glyph on non-green rows | every `warn-*` / `broken-*` and `healthy-efs-with-mt-down` | `ExpectRowNoGlyphPrefix` |
| U6   | S1 badge counts `!`-instances | all fixtures | `ExpectMenuIssueCount` (expected = 2: `warn-efs-updating-mt-down` + `healthy-efs-with-mt-down`; `broken-efs-error` and `broken-efs-no-mount-targets` are Wave-1 Broken — Wave-1 does NOT bump S1 per §4 table) |
| U7a  | Multi-W1 `(+N-1)` suffix | `warn-efs-multi` | `ExpectRowStatusEquals("warn-efs-multi", "no mount targets (+1)")` |
| U7b  | W1+W2 stack bumps suffix | `warn-efs-updating-mt-down` | `ExpectRowStatusEquals(id, "mount target down (+1)")` |
| U7c  | S5 lists every W2 finding | same | `ExpectViewContains("Mount Target: fsmt-…-B")` etc. |
| U7d  | `!` beats `~` | **N/A — no `~` signals** | — |
| U7e  | S5 every W1 phrase | `warn-efs-multi` | `ExpectViewContains("No mount targets")` + `ExpectViewContains("Deleting")` |
| U7f  | Fetcher populates Issues | every fixture | unit deep-equals |
| U8   | Broken > Warning > ~ | `warn-efs-updating-mt-down` (covers Warning vs Broken) | phrase precedence |
| U9   | Related pivot counts | `prod-efs-app-data` | `ExpectRelatedRowCountAtLeast` per pivot |
| U10  | No jargon columns | all fixtures | `ExpectViewNotContains("CIS","Flags","Policy","Issues","NOBKP","UNENC")` |
| U11  | Summary ≠ Rows content | `healthy-efs-with-mt-down` + `warn-efs-updating-mt-down` | unit: `finding.Summary == "mount target down"` AND `!strings.Contains(finding.Summary, row.Value)` for every Row |

## 3. Contract-surface gap analysis

What spec §2/§3/§4 demand vs. what `internal/aws/efs*.go` + `internal/config/defaults_databases.go` + `internal/resource/types_databases.go` provide today.

### Fetcher (`internal/aws/efs.go`)

- **Gap**: Resource.Status is set to raw `LifeCycleState`. Must be §4 phrase or "" on Healthy.
- **Gap**: Fields["status"] is absent. Must be set.
- **Gap**: No derivation of "no mount targets" phrase from `NumberOfMountTargets == 0`.
- **Gap**: Resource.Issues is never populated. Must be an ordered slice of active W1 phrases in precedence order.
- **Gap**: No multi-W1 `(+N-1)` suffix logic.

### Wave 2 enricher (`internal/aws/efs_issue_enrichment.go`)

- **Gap (U11 violation)**: Current `Summary = "mount target unavailable: %s in %s"` embeds both MountTargetId and AZ (also Row values). Must collapse to `"mount target down"`.
- **Gap**: `FieldUpdates` not set. Must write `FieldUpdates[id]["status"] = <bumped status>` via `resource.BumpFindingSuffix` (or the Wave-2 short-cause on Healthy Status == "").
- **Gap**: Rows do not include "Degraded: N/M" counter. Add it so §4 detail text ("N of M mount targets not available") is realized via Row.
- OK: cap handling, pagination handling, severity="!".
- OK: one finding per FS.

### Related targets (`internal/aws/efs_related.go` + `_extra.go`)

- **OK**: kms, cfn, sg, subnet, lambda, alarm, backup, eni, vpc, ecs-task, ec2 all registered. Count = 11.
- **Gap (pre-existing bug)**: `checkEFSECSTask` assumes `RawStruct == ecstypes.TaskDefinition`. The ecs-task fetcher stores `ecstypes.Task`. Fix per "user decision 2026-04-24" — ecs-task fetcher joins task-defs and exposes EFS file-system IDs on each task via `Fields["efs_file_system_ids"]` (comma-separated). Rewrite `checkEFSECSTask` to split and match that field.
- **OK**: `checkEFSEC2` current impl returns 0 when mount-target ENIs have no Attachment.InstanceId — matches spec §5 "empty panel unless field-backed mechanism". Keep. ec2 pivot Count on graph-root = 0 expected (intentional per spec §5).

### ecs-task fetcher upgrade (approved scope expansion)

- **Add** `ECSDescribeTaskDefinitionAPI` interface method (DescribeTaskDefinition).
- **Add** wire ECS client satisfies new interface.
- **Upgrade** fetcher to: for each unique `TaskDefinitionArn` in this page, call `DescribeTaskDefinition` once (memoized map across the page), extract `td.Volumes[].EfsVolumeConfiguration.FileSystemId`, concatenate the unique file-system IDs, store as `Resource.Fields["efs_file_system_ids"]`. Empty string when no EFS volumes.
- **Keep** Resource.RawStruct as `ecstypes.Task`.
- **Tests**: ecs-task fetcher tests get an additional assertion verifying the joined `efs_file_system_ids` field.

### View config (`internal/config/defaults_databases.go` "efs" block + `.a9s/views/efs.yaml`)

- **Gap**: Column "State" reads SDK field `LifeCycleState`. Universal rule: exactly one Status column backed by the derived `status` key. Replace "State" (Path: "LifeCycleState") with "Status" (Path: "Status" — the generic derived phrase held on Resource.Status).
- No jargon columns present — good.
- Other columns (Name, File System ID, Perf Mode, Encrypted, Mounts) are identity/metadata → keep.
- Regenerate `.a9s/views/efs.yaml` via `go run ./cmd/viewsgen/`.

### Color function (`internal/resource/types_databases.go` efs Color block)

- **Gap**: Current Color func reads `r.Fields["life_cycle_state"]` and `r.Fields["mount_targets"]`. Universal rule: read `r.Fields["status"]` (with fallback to `r.Status`), strip `(+N)` suffix via `resource.StripFindingSuffix`, and match against §4 phrases.
- **Rewrite** to:

  ```go
  status := r.Fields["status"]
  if status == "" { status = r.Status }
  phrase := resource.StripFindingSuffix(status)
  switch phrase {
  case "":
      return ColorHealthy
  case "error", "no mount targets", "mount target down":
      return ColorBroken
  case "creating", "updating", "deleting":
      return ColorWarning
  default:
      return ColorHealthy
  }
  ```

### Interfaces (`internal/aws/efs_interfaces.go`)

- No additions needed for EFS itself (DescribeMountTargets, DescribeAccessPoints, DescribeFileSystems already present).

### Detail enricher

- Not needed (no API beyond list+MT).

### Tests to overwrite (delete stale versions before 6b runs)

- `tests/unit/aws_efs_test.go`
- `tests/unit/aws_efs_related_test.go`
- `tests/unit/aws_efs_issue_enrichment_test.go`
- `tests/unit/aws_efs_detail_enrichment_test.go` (may or may not exist; `rm -f` is safe)

### Post-implementation cleanup (coder, phase 7)

- Remove any code path that silently omits Wave-3 stub fields if any exist (scan for CloudWatch metric calls in efs*.go — none expected; assert).
- Remove any `checkEFSECSTask` logic that relies on `ecstypes.TaskDefinition` RawStruct.
