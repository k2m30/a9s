# lt — Implementation Plan

Derived from [`docs/resources/lt.md`](lt.md). Spec has zero TBDs. Implementation starts only after transfer (v3.51.0) merges — this branch stacks on `feat/transfer` and the counts below assume its 68/69 baseline.

## 0. Architecture decisions

**In-fetcher N+1 (the transfer/mwaa pattern).** `DescribeLaunchTemplates` carries no `LaunchTemplateData`; every §2 pivot field and all §3.2 signals live on the `$Default` version. The fetcher does `DescribeLaunchTemplates` (paginated) + `DescribeLaunchTemplateVersions(LaunchTemplateId, Versions=["$Default"])` per template — both `RetryOnThrottle`, E3/E5 aggregation, all findings fetcher-written (`Source: "wave1"`). NO enrichment file, NO `Wave2` catalog field.

**RawStruct = composite wrapper.** Neither SDK shape alone carries the whole detail story: the list `LaunchTemplate` has `DefaultVersionNumber`/`LatestVersionNumber`/`Tags` but no data; the `LaunchTemplateVersion` has `LaunchTemplateData`/`CreatedBy`/`CreateTime` but not the latest-version number or tags. RawStruct is an exported wrapper in `internal/aws/lt.go`:

```go
type LTRaw struct {
    Template       transfertypes-style list shape  // ec2types.LaunchTemplate
    DefaultVersion ec2types.LaunchTemplateVersion  // zero-valued on degraded rows
}
```

Navigable/checker field paths run through the wrapper (`DefaultVersion.LaunchTemplateData.ImageId`, …) — `fieldpath` walks nested structs natively, YAML view renders both halves, and the degraded row is the SAME type with a zero `DefaultVersion` (no second RawStruct shape; simpler than transfer's dual-type fallback — checkers just see empty fields).

**Rich degraded rows (fleet contract).** A denied `DescribeLaunchTemplateVersions` keeps the row with all list fields + `DetailsDeniedFindingDef("lt")` finding carrying lt's §4 sentence ("Access to the default version was denied; only the listed fields are visible."). `RawStruct = LTRaw{Template: t}` — zero `DefaultVersion`.

**Cache cross-ref checkers (asg/ng/ec2) — the sg/eni house pattern.** These three checkers read the SIBLING resource cache (`resource.ResourceCache` parameter), not RawStruct: asg by `LaunchTemplate.LaunchTemplateId` + `MixedInstancesPolicy...` + `Overrides[]`, ng by `Nodegroup.LaunchTemplate.Id/Name`, ec2 by tag `aws:ec2launchtemplate:id`. Truncated/absent sibling cache → `resource.UnknownRelated` (renders `?`), never 0. Grep an existing cache-scanning checker (e.g. the alarm cache-scan used by mwaa, or sg's eni cross-ref) and mirror its cache-access idiom exactly.

Findings (codes `lt.*`):

- `lt.warn.imdsv1` "IMDSv1 allowed" SevWarn — `MetadataOptions == nil || HttpTokens != "required"` (unset defaults to optional — SDK-cited)
- `lt.warn.unencrypted` "EBS encryption disabled" SevWarn — any `BlockDeviceMappings[].Ebs.Encrypted == false` explicit; nil never flags
- `lt.warn.deprecated_ami` "deprecated AMI" SevWarn — ImageId in loaded ami cache AND DeprecationTime past; needs the ami cache at fetch time → implemented as a cache cross-ref inside the FETCHER? NO — fetcher has no sibling caches. Decision: this finding is computed by the ec2-style checker layer? Also no. RESOLUTION: the deprecated-ami signal is the ONLY cross-cache finding; implement it exactly like the existing cross-ref findings if a precedent exists (grep `attention-signals.md` "Cross-ref" rows marked IMPLEMENTED); if every cross-ref finding in the fleet is currently NOT IMPLEMENTED (backlog), this signal ships as the same documented backlog state rather than inventing the first cross-cache finding mechanism ad hoc. Phase 0 of implementation MUST resolve this with a grep and record the answer here.
- `DetailsDeniedFindingDef("lt")`

§4 precedence: imdsv1 → unencrypted → deprecated_ami → details_denied.

## 1. Behavioral test spec (pseudocode) — one case per §4 row

```text
TEST: healthy_silence        GIVEN IMDSv2 required + encrypted mappings + current ami  THEN green, S4 blank, 0 findings
TEST: imdsv1_explicit        GIVEN HttpTokens=optional        THEN "IMDSv1 allowed" SevWarn
TEST: imdsv1_default         GIVEN MetadataOptions nil        THEN same finding (unset = optional!)
TEST: unencrypted_explicit   GIVEN Ebs.Encrypted=false        THEN "EBS encryption disabled" SevWarn
TEST: unencrypted_nil_silent GIVEN Ebs.Encrypted nil          THEN no finding (default-encryption accounts)
TEST: multi_stack            GIVEN imdsv1 + unencrypted       THEN "IMDSv1 allowed (+1)"; ordered per §4
TEST: details_denied_rich    GIVEN DLTV denied for one id     THEN row KEPT with list fields + details-denied; composite error names id
TEST: list_denied_is_error   GIVEN DescribeLaunchTemplates denied THEN (FetchResult{}, err)
TEST: partial_describe       GIVEN 5 listed, 2 DLTV fail      THEN 5 rows (2 degraded) + composite (E5)
TEST: related_targets        graph root: ami 1, asg 2, ec2 2, kms 1, sg 2; eks-node root: ng 1, subnet 1, sg 1 (NI union)
TEST: ssm_ami_no_pivot       GIVEN ImageId "resolve:ssm:..."  THEN ami count 0, no finding, id shown as detail fact
TEST: truncated_cache_unknown GIVEN ec2 cache truncated        THEN ec2 pivot renders unknown, never 0
TEST: wave3_anti             default!=latest → no finding; zero references → no finding
```

## 2. Fixture list (`internal/demo/fixtures/lt.go`; synthetic account 123456789012)

```text
prod-web-lt         (GRAPH ROOT) IMDSv2 required, Ebs Encrypted=true + KmsKeyId → kms fixture,
                    ImageId → existing healthy ami fixture, SecurityGroupIds ×2 → sg fixtures,
                    DefaultVersion 4, LatestVersion 4. Referenced by TWO asg fixtures
                    (one plain LaunchTemplate ref, one MixedInstancesPolicy) and TWO ec2
                    fixtures carrying tag aws:ec2launchtemplate:id.
                    Countable pivots on root: ami 1, asg 2, ec2 2, kms 1, sg 2 → ≥2 on 3/5 = 60% ✓ (9.3 gate ≥50%)
eks-node-lt         ng fixture references it (Nodegroup.LaunchTemplate.Id); NetworkInterfaces[]
                    with Groups ×1 + SubnetId → vpc-graph subnet fixture (NI-union sg witness + subnet witness)
ssm-ami-lt          ImageId "resolve:ssm:/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64" —
                    healthy; witnesses the no-pivot/no-finding ssm skip
warn-lt-imdsv1      HttpTokens optional (explicit)
warn-lt-imdsv1-def  MetadataOptions nil (the unset→optional trap witness)
warn-lt-unencrypted one mapping Encrypted=false explicit
warn-lt-multi       imdsv1 + unencrypted → "IMDSv1 allowed (+1)"
warn-lt-denied      listed, DescribeLaunchTemplateVersions denied (fake) → rich degraded row
[warn-lt-deprecated-ami — ONLY if §0's deprecated-ami resolution lands the finding; needs a
                    deprecated ami sibling fixture (DeprecationTime past)]
```

Menu badge: imdsv1, imdsv1-def, unencrypted, multi, denied = **issues:5** (+1 if deprecated lands); rows 8–9. Counts: type 69 (menu 70); docs 68→69; smoke `resource-types(70)`; filter pins 69→70.

## 3. File scope union

| File | Owner |
|---|---|
| internal/demo/fixtures/lt.go (+asg/ec2/ng/ami sibling touches for refs+tags) | 6a coder |
| internal/demo/fakes/lt.go (DescribeLaunchTemplates/DescribeLaunchTemplateVersions; denial for warn-lt-denied) | 6a coder |
| internal/demo/client.go + fixtures/counts.go entry | 6a coder |
| internal/aws/lt.go — fetcher + LTRaw + findings | 7 coder |
| internal/aws/lt_interfaces.go — narrow LT API on the EC2 client (EC2API already in client.go — extend, don't add a client field) | 6a stub / 7 extend |
| internal/aws/lt_related.go — 8 checkers (5 Pattern F via LTRaw, 3 cache cross-ref) | 7 coder |
| internal/aws/catalog_compute.go — ResourceTypeDef (Category COMPUTE, Aliases [lt launch-template launchtemplate launch-templates lts] — uniqueness gate), Navigable (DefaultVersion.LaunchTemplateData.ImageId→ami is NOT navigable-registry-compatible if ami indexes by id — verify; LoggingRole-style role jump N/A) | 7 coder |
| internal/config/defaults_compute.go — columns: Name, Status, Default, Latest, Created By, Created | 7 coder |
| .a9s/views/lt.yaml (viewsgen) | 7 coder |
| tests/unit/aws_lt_test.go + aws_lt_related_test.go | 6b QA |
| tests/integration/scenario_lt_visual_test.go + drillThroughFixtures rows (both roots) + counts pins | runner |
| docs counts 68→69 (README.tmpl ×3 + Compute row, website resources.md +row), README regen | runner |

## 4. Coverage matrix deltas vs transfer

No child views (simpler). No glyph-on-green (all signals color-bearing — fleet standard). U9 graph-root ≥2 ratio: 3/5 = 60% on prod-web-lt (no structural-ceiling deviation needed). U12 partial_describe. Novelty risks concentrated in: (a) cache cross-ref checkers — mirror an existing cache-scanning checker verbatim; (b) LTRaw composite RawStruct — verify fieldpath/YAML/detail render against it in phase 8 before counts work; (c) deprecated-ami cross-cache finding — resolve per §0 BEFORE dispatching QA (test spec depends on it).
