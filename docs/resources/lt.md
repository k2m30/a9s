---
shortName: lt
name: Launch Templates
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ResponseLaunchTemplateData.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# lt — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `lt` (aliases: `launch-template`, `launchtemplate`, `launch-templates`, `lts`)
- **Display name**: Launch Templates
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ResponseLaunchTemplateData.html>
- **List API**: `DescribeLaunchTemplates` (paginated; carries NO `LaunchTemplateData` — identity/version-number/creator facts only)
- **Describe API (if any)**: `DescribeLaunchTemplateVersions(Versions=["$Default"])`, one call per template — the single extra call that funds ALL §2 data pivots AND all §3.2 signals. `$Default` (not `$Latest`) is what `asg`/`ng`/`ec2` actually resolve at launch; `$Latest` is staging.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ami`, `asg`, `ec2`, `kms`, `ng`, `sg`, `subnet`, `ct-events`.

### `ami`

- **Why related**: `LaunchTemplateData.ImageId` — the image new instances boot from; "what exactly does this template launch".
- **How discovered**: read field on the `$Default` version; pivot ONLY when the value matches `ami-` (a `resolve:ssm:` reference is a display fact, not a pivot).
- **Count shown**: yes.

### `asg`

- **Why related**: "which fleets launch from this template" — the blast radius of a bad default-version bump.
- **How discovered**: cross-reference the already-loaded `asg` list by `AutoScalingGroup.LaunchTemplate.LaunchTemplateId`, `MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification`, and per-`Overrides[]` specifications. Zero extra API calls.
- **Count shown**: yes.

### `ec2`

- **Why related**: instances actually running from this template — direct, ASG-launched, and NG-launched alike.
- **How discovered**: cross-reference the already-loaded `ec2` list by the AWS auto-tag `aws:ec2launchtemplate:id`. Degrades to unknown (`?`) when the ec2 cache is truncated — never a fake 0. No dedicated `DescribeInstances` call.
- **Count shown**: yes (unknown when the sibling cache is truncated).

### `kms`

- **Why related**: `BlockDeviceMappings[].Ebs.KmsKeyId` — which key encrypts the volumes this template provisions.
- **How discovered**: read field on the `$Default` version; key-id/ARN forms only (alias forms are detail-only). Zero counts are the normal case.
- **Count shown**: yes.

### `ng`

- **Why related**: EKS node groups pinning this template — the other fleet consumer.
- **How discovered**: cross-reference the already-loaded `ng` list by `Nodegroup.LaunchTemplate.Id`/`Name`. Zero extra API calls.
- **Count shown**: yes.

### `sg`

- **Why related**: the security groups every launched instance lands in — first stop for "will this instance be reachable".
- **How discovered**: union of `LaunchTemplateData.SecurityGroupIds` ∪ `NetworkInterfaces[].Groups` (mutually exclusive by API design). `SecurityGroups` (names, EC2-Classic legacy) are detail-only.
- **Count shown**: yes.

### `subnet`

- **Why related**: `NetworkInterfaces[].SubnetId` — a template that pins a subnet pins instance placement.
- **How discovered**: read field on the `$Default` version; usually empty by design (the subnet normally comes from the ASG/NG side) — rendered only when non-empty.
- **Count shown**: yes.

### `ct-events`

- **Why related**: audit trail — "who bumped the default version" is the first incident question for a bad rollout. Universal pivot — applies to every registered type; see related-resources.md §Policy.
- **How discovered**: CloudTrail LookupEvents by LaunchTemplateId.
- **Count shown**: yes.

Explicitly excluded (per `docs/related-resources.md` §`lt`): `role` (`IamInstanceProfile` is a PROFILE, not a role; profile→role resolution needs `iam:GetInstanceProfile` — a second call — and a name-equality heuristic is dishonest; detail field only), `eks` (the cluster reference lives on the node group; pivot via `ng`).

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention.

Deliberately not Wave-1 signals (a9s-devops 2026-07-14): `DefaultVersionNumber != LatestVersionNumber` is DISPLAY ONLY (live-witnessed as the healthy steady state — a pending-rollout latest version is normal working practice; flagging it is alarm fatigue). "Who references this template" is answered by the related panel (`asg`/`ng`/`ec2` pivot counts, one detail keypress) — NOT a list column: a cross-cache computed column has no existing mechanism and an unreferenced template is not a problem worth flagging (templates are free and inert). `CreatedBy`/`CreateTime` are plain columns.

### 3.2 Wave 2 — bounded extra API calls

All three signals ride the same single call: `DescribeLaunchTemplateVersions(Versions=["$Default"])`, one per template.

- **Signal**: `MetadataOptions.HttpTokens != "required"` — IMDSv1 allowed. Unset defaults to `optional` (SDK-confirmed), so absence of MetadataOptions IS the signal, not its negation. `HttpEndpoint == disabled` is NOT a risk and produces no signal.
  - **State bucket**: Warning.
  - **API call**: `DescribeLaunchTemplateVersions`, one per template.
  - **Cost shape**: per-resource.
- **Signal**: any `BlockDeviceMappings[].Ebs.Encrypted == false` — explicitly disabled EBS encryption. `nil` is UNKNOWN, not unencrypted (accounts with default-encryption make nil legitimate) — never flag nil.
  - **State bucket**: Warning.
  - **API call**: same call.
  - **Cost shape**: per-resource.
- **Signal**: `ImageId` present in the already-loaded `ami` cache AND that AMI's `DeprecationTime` is past — new launches use a deprecated image. NOT-in-cache ≠ deregistered (public/marketplace AMIs are legitimately absent from the owner-scoped cache) — no signal then. `resolve:ssm:` references are skipped.
  - **State bucket**: Warning.
  - **API call**: none beyond the same call (cross-ref of the loaded `ami` list).
  - **Cost shape**: per-resource.
- **Signal**: per-template `DescribeLaunchTemplateVersions` denied — the row is KEPT with all its list fields (`details denied`, shared rich-degradation contract).
  - **State bucket**: Warning.
  - **API call**: the denial IS the response.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: version diffing (`$Default` vs `$Latest` — N calls; "read the diff" drill-down).
- OUT OF SCOPE: UserData decode / secret-scan.
- OUT OF SCOPE: `IamInstanceProfile`→role resolution (`iam:GetInstanceProfile`).
- OUT OF SCOPE: `ssm:GetParameter` AMI-reference resolution.
- OUT OF SCOPE: explicit `DescribeImages` existence/deprecation checks.
- OUT OF SCOPE: dedicated `DescribeInstances`-by-tag sweep.
- OUT OF SCOPE: spot/CPU/placement deep-config analysis.

## 4. Issue Visualization

Surfaces S1–S5 per `docs/attention-signals.md` §Visualization Surfaces; wave→surface mapping as standard. Every signal is color-bearing — the fleet color invariant ("color derives from findings", the conformance-gate owner rule) applies to the enricher-borne deprecated-AMI check too: its row renders Warning-colored like any other finding. Its `~` class affects only the S1 aggregation (no badge bump — the badge counts state rows and `!`-class checks) and S5 ordering.

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| IMDSv1 allowed | 2 | Warning | n/a | S2, S4, S5 | `IMDSv1 allowed` | `Instance metadata does not require session tokens; IMDSv1 credentials are exposed to SSRF.` |
| EBS encryption off | 2 | Warning | n/a | S2, S4, S5 | `EBS encryption disabled` | `A block device explicitly sets Encrypted=false; launched instances get unencrypted volumes.` |
| deprecated AMI | 2 | Warning (background `~` class: no S1 bump) | n/a | S2, S4, S5 | `deprecated AMI` | `The default version references an AMI past its deprecation time.` |
| `DescribeLaunchTemplateVersions` denied | 2 | Warning | n/a | S2, S4, S5 | `details denied` | `Access to the default version was denied; only the listed fields are visible.` |
| credential in the `$Default` version's `UserData` | 2 | Broken | `!` | S1, S3, S4, S5 | `credential in user data` | `A credential is pasted into the default version's user data, so it is readable by anyone who can call ec2:DescribeLaunchTemplateVersions and lands on every instance launched from this template. Move the value to Secrets Manager or Systems Manager Parameter Store and rotate it.` |

Notes:

- No raw AWS enum reaches a rendered surface.
- Multiple findings stack with the framework `(+N)` suffix; S5 enumerates each.
- AccessDenied on `ec2:DescribeLaunchTemplates`: menu row shows the error state, never `0`.
- Healthy templates (IMDSv2 required, no explicit-off encryption, current AMI) render green with a blank Status — the normal case for a well-run account is a silent list.
- The deprecated-AMI check does not bump the `issues:N` badge (S1 counts urgent findings only) — the template still launches; the Warning-colored row plus the Status phrase flag it for the next maintenance window.
- `DescribeLaunchTemplateVersions` denied: an authorization failure renders `details denied`; a non-authorization describe failure (nil body, transient error, no `$Default` version in the response) renders the neutral `details unavailable`.

## 4.1 UX review (two sentences)

Every problem row names its cause in the Status column (`IMDSv1 allowed`, `EBS encryption disabled`, `deprecated AMI`), so the 3am operator triages without opening detail. The blast-radius question — which fleets and instances launch from this template — is answered by the `asg`/`ng`/`ec2` pivot counts one detail keypress away.

## 5. Out of Scope

- All §3.3 Wave 3 items.
- `role`/`eks` pivots (§2 exclusions with citations).
- Flagging `DefaultVersionNumber != LatestVersionNumber` or unreferenced templates (§3.1 — deliberate non-signals).
- Any UI element not in S1–S5; any write operation (`architecture.md` §"What is a9s?").

## 6. Citations

- Pivot set and exclusions — `docs/related-resources.md` § `lt`; one-call `$Default` budget noted there.
- List shape carries no data fields — `AWS SDK Go v2 — ec2/types.LaunchTemplate` (identity, `DefaultVersionNumber`/`LatestVersionNumber`, `CreatedBy`, `CreateTime`, `Tags` only).
- IMDSv1 signal — `AWS SDK Go v2 — ec2/types.LaunchTemplateInstanceMetadataOptions § HttpTokens` ("optional — … you receive the IMDSv1 role credentials"); unset-defaults-to-optional per the same doc; `a9s-devops (2026-07-14): possible=yes, worth=yes. dbi-PubliclyAccessible class; SSRF/credential-theft precedent.`
- Encryption signal — `AWS SDK Go v2 — ec2/types.LaunchTemplateEbsBlockDevice § Encrypted, § KmsKeyId`; nil-is-unknown rule — `a9s-devops (2026-07-14): possible=yes, worth=yes. Default-encryption accounts make nil legitimate; flagging nil is a false positive.`
- Deprecated-AMI signal — `docs/attention-signals.md` § Compute row `ami` (`DeprecationTime < now()` → Warning) cross-referenced from the loaded cache; `a9s-devops (2026-07-14): possible=yes, worth=yes. Not-in-cache ≠ deregistered — public/marketplace AMIs legitimately absent.`
- `$Default`-not-`$Latest` read — `a9s-devops (2026-07-14): $Default is what asg/ng/ec2 resolve at launch; $Latest is staging.`
- default≠latest display-only, no unused-flag — `a9s-devops (2026-07-14): possible=yes, worth=no. Healthy steady state; alarm fatigue; templates free+inert.`
- references answered by the related panel, not a list column — `user (2026-07-14): reuse common techniques fit to current infrastructure, no resource-specific machinery. A cross-cache computed list column has no house mechanism; the asg/ng/ec2 pivots already carry the counts.`
- sg union discovery — `AWS SDK Go v2 — ec2/types.ResponseLaunchTemplateData § SecurityGroupIds, § SecurityGroups, § NetworkInterfaces` (ids vs legacy names vs per-ENI groups).
- ec2 cross-ref by auto-tag — `a9s-devops (2026-07-14): aws:ec2launchtemplate:id auto-tag catches direct+ASG+NG launches; degrade to unknown on truncated cache, never fake 0.`
- role exclusion — `a9s-devops (2026-07-14): IamInstanceProfile is a profile, not a role; second call or dishonest heuristic required.`
- Degraded-row contract — shared rich degradation (fleet contract since v3.50.0, rich form since transfer).
- Read-only invariant — `docs/architecture.md` § "What is a9s?".

<!-- BEGIN GENERATED: header -->
lt — COMPUTE. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| lt.warn.imdsv1 | IMDSv1 allowed | warn | wave1 | — |
| lt.warn.unencrypted | EBS encryption disabled | warn | wave1 | — |
| lt.warn.deprecated\_ami | deprecated AMI | warn | wave2 | — |
| lt.user-data-secret | credential in user data | broken | wave2 | — |
| lt.warn.details\_denied | details denied | warn | wave1 | — |
| lt.warn.details\_unavailable | details unavailable | warn | wave1 | — |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| ami | AMI | no |
| asg | Auto Scaling Groups | yes |
| ec2 | EC2 Instances | yes |
| kms | KMS Key | no |
| ng | EKS Node Groups | yes |
| sg | Security Groups | no |
| subnet | Subnets | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
