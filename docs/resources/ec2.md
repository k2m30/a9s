---
shortName: ec2
name: EC2 Instances
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# ec2 — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ec2`
- **Display name**: EC2 Instances
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html>
- **List API**: `DescribeInstances`
- **Describe API (if any)**: `DescribeInstanceStatus(IncludeAllInstances=true)` — one account-wide call used by the Wave 2 issue enricher; no per-instance `DescribeInstances` fan-out.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `ami`, `asg`, `backup`, `cfn`, `ebs`, `ebs-snap`, `eip`, `eni`, `kms`, `logs`, `ng`, `role`, `sg`, `ssm`, `subnet`, `tg`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: CloudWatch alarms watching this instance — first signal of impact.
- **How discovered**: cross-reference the already-loaded `alarm` list by `MetricAlarm.Dimensions[]` containing `{Name: "InstanceId", Value: <instance-id>}` — a9s-devops: alarms pin their target via `Dimensions`, so a cache-scan against loaded alarms is the standard pivot and requires no extra API call.
- **Count shown**: yes.

### `ami`

- **Why related**: Provenance of the running image; compare against latest approved AMI.
- **How discovered**: read field `Instance.ImageId` on the resource; cross-reference the already-loaded `ami` list by `Image.ImageId`.
- **Count shown**: yes.

### `asg`

- **Why related**: ASG that owns the instance (if any) — lifecycle context.
- **How discovered**: read field `Instance.Tags[]` for key `aws:autoscaling:groupName`; cross-reference the already-loaded `asg` list by `AutoScalingGroupName`. Fallback: scan loaded `asg` list where `Instances[].InstanceId` matches — a9s-devops: ASG-launched instances carry the reserved AWS tag, so tag lookup is the cheap-and-reliable pivot; the `Instances[]` fallback covers instances launched via the ASG API at run-time.
- **Count shown**: yes.

### `backup`

- **Why related**: Backup plans that protect this instance.
- **How discovered**: cross-reference the already-loaded `backup` list; match by backup-plan selection tags present on `Instance.Tags[]` or by ARN via `backup:ListProtectedResources` — a9s-devops: AWS Backup couples plans to resources through tag-based selections; scanning the loaded plan list for matching selection tags surfaces coverage without an extra API call. When live-lookup is required, `ListProtectedResources` is the authoritative API.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created the instance — infra-as-code linkage.
- **How discovered**: read field `Instance.Tags[]` for key `aws:cloudformation:stack-name` or `aws:cloudformation:stack-id`; cross-reference the already-loaded `cfn` list by `StackName` — a9s-devops: CloudFormation stamps these reserved tags on every stack-managed resource, so tag lookup is the correct pivot; no API call needed when `cfn` is already loaded.
- **Count shown**: yes.

### `ebs`

- **Why related**: Attached storage — capacity/IOPS troubleshooting.
- **How discovered**: read field `Instance.BlockDeviceMappings[].Ebs.VolumeId`; cross-reference the already-loaded `ebs` list by `Volume.VolumeId`.
- **Count shown**: yes.

### `ebs-snap`

- **Why related**: Instance's AMI snapshots for rollback/forensic workflows.
- **How discovered**: derive via `Instance.ImageId` → AMI → `Image.BlockDeviceMappings[].Ebs.SnapshotId`, plus `Instance.BlockDeviceMappings[].Ebs.VolumeId` → volume → snapshots from that volume; cross-reference the already-loaded `ebs-snap` list by `Snapshot.SnapshotId` / `Snapshot.VolumeId` — a9s-devops: rollback workflows hinge on the AMI-root snapshot and the attached-volume snapshots, so the pivot must union both sources. No extra API call when `ami`, `ebs`, `ebs-snap` are already loaded.
- **Count shown**: yes.

### `eip`

- **Why related**: Addresses associated with the instance; traffic attribution.
- **How discovered**: cross-reference the already-loaded `eip` list by `Address.InstanceId == <instance-id>` OR `Address.NetworkInterfaceId` in `Instance.NetworkInterfaces[].NetworkInterfaceId` — a9s-devops: EIPs associate to either an instance directly or to an ENI on the instance, so the pivot must check both fields to catch secondary-interface attachments.
- **Count shown**: yes.

### `eni`

- **Why related**: ENIs for multi-homed or secondary interfaces.
- **How discovered**: read field `Instance.NetworkInterfaces[].NetworkInterfaceId`; cross-reference the already-loaded `eni` list by `NetworkInterface.NetworkInterfaceId`.
- **Count shown**: yes.

### `kms`

- **Why related**: Instance-attached volume encryption keys.
- **How discovered**: derive via `Instance.BlockDeviceMappings[].Ebs.VolumeId` → loaded `ebs` list → `Volume.KmsKeyId`; cross-reference the already-loaded `kms` list by `KeyMetadata.Arn` — a9s-devops: encryption keys are carried on volumes, not on the instance itself; a two-hop walk through the volume list is the only path, and no extra API call is needed when `ebs` and `kms` are loaded.
- **Count shown**: yes.

### `logs`

- **Why related**: CloudWatch Log Groups fed by CloudWatch agent on this instance.
- **How discovered**: cross-reference the already-loaded `logs` list by log-group name convention — the CloudWatch agent writes streams keyed on `<instance-id>`, and common groups follow `/aws/ec2/<id>` or carry an `InstanceId` tag — a9s-devops: there is no authoritative EC2→log-group link in the AWS surface, so this is a best-effort naming/tag-convention match; operators still benefit because agent-managed groups are the top place they look during an incident.
- **Count shown**: yes.

### `ng`

- **Why related**: Nodegroup owning this instance.
- **How discovered**: read field `Instance.Tags[]` for key `eks:nodegroup-name` (plus `aws:eks:cluster-name`); cross-reference the already-loaded `ng` list by `Nodegroup.NodegroupName` — a9s-devops: EKS Managed Node Groups tag every launched instance with these reserved keys, so tag lookup is the clean pivot with no extra API call.
- **Count shown**: yes.

### `role`

- **Why related**: Permissions the instance operates with.
- **How discovered**: read field `Instance.IamInstanceProfile.Arn` (or `.Id`), strip the profile, resolve to role name; cross-reference the already-loaded `role` list by `Role.RoleName` — a9s-devops: instance-profile ARNs embed the profile name, not the role; when profile name and role name differ, a `GetInstanceProfile` call is needed. In practice the two match 99% of the time so the cache-scan is sufficient; fall back to the API only on miss.
- **Count shown**: yes.

### `sg`

- **Why related**: Ingress/egress rules; first stop for connectivity issues.
- **How discovered**: read field `Instance.SecurityGroups[].GroupId`; cross-reference the already-loaded `sg` list by `SecurityGroup.GroupId`.
- **Count shown**: yes.

### `ssm`

- **Why related**: SSM Managed Instance / Session Manager presence on this instance.
- **How discovered**: call `ssm:DescribeInstanceInformation` filtered by `InstanceIds=[<id>]` — a9s-devops: `ssm` in the contract here is the SSM Managed Instance view (registration/ping status, last-seen, agent version), not Parameter Store. The cheap pivot is a single filtered `DescribeInstanceInformation` call per open; any match means the instance is SSM-enrolled and reachable via Session Manager. Worth=yes for the daily operator because "can I shell in via SSM?" is a common triage question.
- **Count shown**: yes.

### `subnet`

- **Why related**: Primary ENI's subnet; used when diagnosing placement/routing.
- **How discovered**: read field `Instance.SubnetId`; cross-reference the already-loaded `subnet` list by `Subnet.SubnetId`.
- **Count shown**: yes.

### `tg`

- **Why related**: Target groups this instance is registered with — traffic routing.
- **How discovered**: cross-reference the already-loaded `tg` list by calling `DescribeTargetHealth` per TG and matching `TargetHealthDescriptions[].Target.Id == <instance-id>` — a9s-devops: target-group membership is not on `Instance`; it lives on the TG side via `DescribeTargetHealth`, which a9s already fans out for the `tg` Wave 2 enrichment. The same result set answers the ec2→tg pivot, so no net-new API call is required when `tg` is already loaded.
- **Count shown**: yes.

### `vpc`

- **Why related**: Network parent; pivoted to for VPC-wide troubleshooting.
- **How discovered**: read field `Instance.VpcId`; cross-reference the already-loaded `vpc` list by `Vpc.VpcId`.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for all API calls touching this instance.
- **How discovered**: call `cloudtrail:LookupEvents` with `LookupAttributes=[{AttributeKey=ResourceName, AttributeValue=<instance-id>}]`.
- **Count shown**: yes.
- **Note**: universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeInstances](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html)

Transcribed from `docs/attention-signals.md § Signals § COMPUTE` row `ec2`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `State.Name == shutting-down` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Instance.State.Name` on the `DescribeInstances` response.

- **Signal**: `State.Name == stopped` AND `StateReason.Code` begins with `Server.*` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Instance.StateReason.Code` + `Instance.StateReason.Message` on the `DescribeInstances` response.

- **Signal**: `State.Name == stopped` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Instance.State.Name` on the `DescribeInstances` response.

- **Signal**: `State.Name == terminated` → Dim.
  - **State bucket**: Dim.
  - **How obtained**: `Instance.State.Name` on the `DescribeInstances` response.

- **Signal**: instance metadata answers without a session token.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `PublicIpAddress` set.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `State.Name == pending`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `State.Name == stopping`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `SystemStatus.Status == impaired` or `InstanceStatus.Status == impaired` → Broken.
  - **State bucket**: Broken.
  - **API call**: `DescribeInstanceStatus(IncludeAllInstances=true)` — one account-wide call.
  - **Cost shape**: account-wide.

- **Signal**: `SystemStatus.Status == initializing` or `InstanceStatus.Status == initializing` → Warning (checks have not yet passed since start).
  - **State bucket**: Warning.
  - **API call**: `DescribeInstanceStatus(IncludeAllInstances=true)` — one account-wide call.
  - **Cost shape**: account-wide.

- **Signal**: `SystemStatus.Status == insufficient-data` or `InstanceStatus.Status == insufficient-data` → Warning (AWS cannot determine).
  - **State bucket**: Warning.
  - **API call**: `DescribeInstanceStatus(IncludeAllInstances=true)` — one account-wide call.
  - **Cost shape**: account-wide.

- **Signal**: `Events[]` containing a scheduled retirement or reboot with `NotBefore` within 7 days → Warning.
  - **State bucket**: Warning.
  - **API call**: `DescribeInstanceStatus(IncludeAllInstances=true)` — one account-wide call; inspects `InstanceStatus.Events[].Code` in `instance-retirement`/`system-reboot`/`instance-reboot`/`system-maintenance`/`instance-stop` with `NotBefore <= now + 7d`.
  - **Cost shape**: account-wide.

- **Signal**: public address behind a security group open on a sensitive port (`sg` cache cross-ref).
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: public address behind a security group admitting every protocol from `0.0.0.0/0` (`sg` cache cross-ref, `wide_open`).
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type. A group that opens every port says everything a port list would, so it is reported instead of the list, not beside it.

- **Signal**: credential in `DescribeInstanceAttribute(userData)`.
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `StatusCheckFailed` metric-based detection.
- OUT OF SCOPE: IMDSv1 detection.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `ec2`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `State.Name == shutting-down` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `shutting down` |
| `stopped` + `StateReason.Code` begins `Server.*` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `stopped by AWS` |
| `State.Name == stopped` with no `Server.*` state reason | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `stopped` |
| `terminated` | 1 | Dim | n/a | S2, S4 | `terminated` |
| instance metadata answers without a session token | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `IMDSv1 allowed` |
| `PublicIpAddress` set | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `public address` |
| `State.Name == pending` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `pending` |
| `State.Name == stopping` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `stopping` |
| `SystemStatus.Status == impaired` (or `InstanceStatus.Status == impaired`) | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `impaired: system checks failing` |
| `SystemStatus.Status == initializing` | 2 | Warning | `~` | S2, S3, S4, S5 | `initializing: checks in progress` |
| `SystemStatus.Status == insufficient-data` | 2 | Warning | `~` | S2, S3, S4, S5 | `status unknown: AWS insufficient-data` |
| `Events[]` scheduled retirement/reboot within 7 days | 2 | Warning | `~` | S2, S3, S4, S5 | `scheduled event` |
| public address behind a security group open on a sensitive port (`sg` cache cross-ref) | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `<port(s) LIST> reachable from the internet` |
| public address behind a security group admitting every protocol from `0.0.0.0/0` (`sg` cache cross-ref, `wide_open`) | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `every port reachable from the internet` |
| credential in `DescribeInstanceAttribute(userData)` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `credential in user data` |

Notes on list-text construction:

- The `Server.*` case shows the AWS reason code directly (e.g., `Server.SpotInstanceShutdown`, `Server.InsufficientInstanceCapacity`) — it is already short and meaningful to operators.
- The long-stopped case requires computing age from `StateTransitionReason` (format `"User initiated (YYYY-MM-DD HH:MM:SS GMT)"`) and rendering it in days.
- The scheduled-event case renders the event's `NotBefore` countdown in human form (`in 3d`, `in 12h`); on passed `NotBefore` switch to `retirement overdue`.
- `running` (Healthy) intentionally has no row here — S4 blank, no glyph. Silence is the UX.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for every row above — the Status column always carries either a human cause (`impaired: system checks failing`, `scheduled event`, `status unknown: AWS insufficient-data`) or the state keyword itself (`stopped`, `pending`, `terminated`). The only residual concern is the `stopping` transitional case, which is inherently short-lived and does not need a cause beyond the verb.

## 4.2 On-Demand Detail Enrichment

Opening the detail, YAML, or JSON view triggers one extra read-only call whose result is attached to the resource's raw structure for the stacked views. List views are never affected.

- AWS API: `DescribeInstanceAttribute` (`Attribute=userData`)
- Payload: the instance user data, base64-decoded (and gunzipped when gzip-compressed, as cloud-init payloads commonly are) — the bootstrap script, critical when debugging instance launch/config; non-text payloads stay in their original base64 form. Renders both as the last detail-view field and in the YAML/JSON views
- Cache: none — a single cheap call, and user data can be edited while an instance is stopped
- Failure: a flash message; the view still renders the un-enriched resource

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch `StatusCheckFailed`, IMDSv1 detection).
- `not-applicable` value on `SystemStatus.Status` / `InstanceStatus.Status` — Healthy, informational only: `docs/attention-signals.md § Signals § COMPUTE` row `ec2` carries no finding for it.
- Any UI element not listed in §4 — no new columns, no new icons, no new views, no new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- a9s golden doc — EC2 Wave 1 state mapping (`running`, `pending`, `stopping`, `stopped`, `terminated`) — `docs/attention-signals.md § Signals § COMPUTE` row `ec2`.
- a9s golden doc — `StateReason.Code` `Server.*` → Broken on stopped instance — `docs/attention-signals.md § Signals § COMPUTE` row `ec2`.
- a9s golden doc — `StateTransitionReason` user-initiated >30d → Warning — `docs/attention-signals.md § Signals § COMPUTE` row `ec2`.
- a9s golden doc — Wave 2 source `DescribeInstanceStatus(IncludeAllInstances=true)` with `impaired`/`initializing`/`insufficient-data`/`not-applicable` bucketing — `docs/attention-signals.md § Signals § COMPUTE` row `ec2`.
- a9s golden doc — scheduled retirement/reboot `Events[]` within 7 days → Warning — `docs/attention-signals.md § Signals § COMPUTE` row `ec2`.
- a9s golden doc — Wave 3 `StatusCheckFailed` + IMDSv1 OUT OF SCOPE — `docs/attention-signals.md § Not yet implemented`.
- a9s golden doc — related targets contract for `ec2` — `docs/related-resources.md` § Per-type contract / `ec2` row.
- a9s golden doc — per-target reasoning (`alarm`, `ami`, `asg`, `backup`, `cfn`, `ct-events`, `ebs`, `ebs-snap`, `eip`, `eni`, `kms`, `logs`, `ng`, `role`, `sg`, `ssm`, `subnet`, `tg`, `vpc`) — `docs/related-resources.md` § Per-target reasoning / `ec2`.
- a9s golden doc — `ct-events` as universal pivot — `docs/related-resources.md` § Policy (item 4).
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- AWS API Reference — `State.Name`, `StateReason.Code`, `StateReason.Message`, `StateTransitionReason`, `ImageId`, `BlockDeviceMappings`, `NetworkInterfaces`, `SecurityGroups`, `SubnetId`, `VpcId`, `IamInstanceProfile`, `Tags` — `AWS API Reference: API_Instance` (<https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Instance.html>).
- AWS API Reference — `InstanceStatusEvent.Code` values (`instance-reboot`, `system-reboot`, `system-maintenance`, `instance-retirement`, `instance-stop`), `NotBefore`, `NotAfter`, `NotBeforeDeadline` — `AWS API Reference: API_InstanceStatusEvent` (<https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InstanceStatusEvent.html>).
- AWS API Reference — `Volume.KmsKeyId` (kms pivot via volumes) — `AWS API Reference: API_Volume` (<https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Volume.html>).
- AWS API Reference — `Address.InstanceId` and `Address.NetworkInterfaceId` (eip pivot) — `AWS API Reference: API_Address` (<https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html>).
- AWS API Reference — `MetricAlarm.Dimensions[]` (alarm pivot) — `AWS API Reference: API_MetricAlarm` (<https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricAlarm.html>).
- AWS API Reference — `TargetHealthDescription.Target.Id` (tg pivot) — `AWS API Reference: API_DescribeTargetHealth` (<https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeTargetHealth.html>).
- AWS API Reference — `ssm:DescribeInstanceInformation` filtered by `InstanceIds` (ssm Managed-Instance pivot) — `AWS API Reference: API_DescribeInstanceInformation` (<https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_DescribeInstanceInformation.html>).
- a9s-devops consultation — `alarm` discovery via `Dimensions{InstanceId,...}` on loaded alarm list — a9s-devops (2026-04-20): possible=yes, worth=yes. Dimensions are the canonical target pointer for CloudWatch alarms; cache-scan is zero-cost when alarms are loaded.
- a9s-devops consultation — `asg` discovery via `aws:autoscaling:groupName` tag — a9s-devops (2026-04-20): possible=yes, worth=yes. AWS sets the reserved tag on every ASG-launched instance; tag lookup is the cheap pivot.
- a9s-devops consultation — `backup` discovery via plan-selection tag match / `ListProtectedResources` — a9s-devops (2026-04-20): possible=yes, worth=yes. AWS Backup associates resources by tag-based selections; tag scan is zero-cost, API fallback covers ARN-based selections.
- a9s-devops consultation — `cfn` discovery via `aws:cloudformation:stack-name` tag — a9s-devops (2026-04-20): possible=yes, worth=yes. Reserved CFN tag is present on every stack-managed resource.
- a9s-devops consultation — `ebs-snap` discovery via AMI-root and volume-sourced snapshots — a9s-devops (2026-04-20): possible=yes, worth=yes. Rollback/forensics hinges on both sources; zero-cost when `ami`, `ebs`, `ebs-snap` are loaded.
- a9s-devops consultation — `eip` discovery via `Address.InstanceId` + `Address.NetworkInterfaceId` — a9s-devops (2026-04-20): possible=yes, worth=yes. EIPs attach directly or via ENI; both paths must be checked.
- a9s-devops consultation — `kms` discovery via two-hop through `ebs` to `Volume.KmsKeyId` — a9s-devops (2026-04-20): possible=yes, worth=yes. Instance-level encryption keys only exist on attached volumes; no extra API call needed.
- a9s-devops consultation — `logs` discovery via CloudWatch-agent naming / tag convention — a9s-devops (2026-04-20): possible=yes (best-effort), worth=yes. No authoritative AWS link exists; agent-managed groups are the top incident-triage destination so a convention match is still useful.
- a9s-devops consultation — `ng` discovery via `eks:nodegroup-name` tag — a9s-devops (2026-04-20): possible=yes, worth=yes. EKS stamps the reserved tag on every managed nodegroup instance.
- a9s-devops consultation — `role` discovery via `Instance.IamInstanceProfile.Arn` with profile-name ≈ role-name assumption and `GetInstanceProfile` fallback — a9s-devops (2026-04-20): possible=yes, worth=yes. Profile vs role name usually match; fallback is cheap.
- a9s-devops consultation — `ssm` target means Managed-Instance view, not Parameter Store — a9s-devops (2026-04-20): possible=yes, worth=yes. `DescribeInstanceInformation(InstanceIds=[id])` is the correct, single-call pivot; "can I shell in via SSM?" is a daily triage question.
- a9s-devops consultation — `tg` discovery via `DescribeTargetHealth` fan-out already performed for the TG list — a9s-devops (2026-04-20): possible=yes, worth=yes. No net-new API call when `tg` Wave 2 has run; avoids duplicating fan-out.
- a9s-devops consultation — Count shown is `yes` for every target — a9s-devops (2026-04-20): possible=yes, worth=yes. Every pivot above returns a concrete matched set; rendering the count is standard a9s behavior.
- a9s-devops consultation — Status column wording for `stopped: Server.*` surfaces the raw AWS code — a9s-devops (2026-04-20): possible=yes, worth=yes. `Server.SpotInstanceShutdown`, `Server.InsufficientInstanceCapacity` etc. are already short and industry-known to operators; translating them would lose precision.
- a9s-devops consultation — long-stopped wording shows age (`stopped 42d ago`) rather than just `stopped` — a9s-devops (2026-04-20): possible=yes, worth=yes. Age is the whole reason the row is flagged; without it the row violates the "no bare state keyword" rule.

<!-- BEGIN GENERATED: header -->
ec2 — COMPUTE. Status key: `state` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| ec2.state.pending | pending | warn | wave1 | The instance is still booting, so it is not serving yet and its status checks have not run. Give it a minute; if it stays here, check the launch's status reason and whether the instance type has capacity in that Availability Zone. |
| ec2.state.shutting-down | shutting down | warn | wave1 | The instance is being terminated and will disappear shortly, taking anything on its instance-store volumes with it. If this was not intended, stop whatever issued the terminate call now — a terminated instance cannot be recovered. |
| ec2.state.stopping | stopping | warn | wave1 | The instance is on its way down and is no longer accepting traffic, so anything routed to it is failing now. Wait for it to reach a stopped state before starting it again or detaching its volumes. |
| ec2.state.stopped | stopped | warn | wave1 | Nothing this instance hosts is answering, while its EBS volumes and any Elastic IP attached to it keep costing money. Start it if it should be serving, or terminate it and release its volumes if it is genuinely finished with. |
| ec2.state.stopped.server | stopped by AWS | broken | wave1 | AWS stopped this instance itself rather than an operator doing it, which points at degraded underlying hardware or a billing or compliance action on the account. Start it again so it comes up on different hardware, and check the instance's status events and your account notifications for the reason. |
| ec2.state.terminated | terminated | dim | wave1 | — |
| ec2.instance-status-impaired | impaired: system checks failing | broken | wave2 | AWS's own checks of the host or the instance are failing, which means the problem is below your software: the hypervisor, the network path, or the instance's ability to boot. Stop and start the instance so it moves to different hardware, and read the system log first if you need the cause on record. |
| ec2.instance-status.initializing | initializing: checks in progress | warn | wave2 | The instance is up but its checks have not passed yet, so a load balancer will not send it traffic and an alarm on it has nothing to judge. Give it a few minutes; a check still initializing well past boot usually means the instance is not finishing its startup. |
| ec2.instance-status.insufficient-data | status unknown: AWS insufficient-data | warn | wave2 | AWS cannot reach the instance to check it, so its health is unknown rather than good, and a monitor reading this as healthy is reading nothing. Wait for the next check, and treat a run of these as a possible host problem worth a stop and start. |
| ec2.scheduled-event | scheduled event | warn | wave2 | AWS has scheduled maintenance, a retirement or a reboot for this instance, and it will happen whether or not anyone is ready. Read the event window, then stop and start the instance at a time you choose so it moves to healthy hardware on your schedule. |
| ec2.imdsv1-allowed | IMDSv1 allowed | warn | wave1 | Instance metadata answers requests without a session token, so an SSRF bug on this host can read the attached IAM role's credentials. Require session tokens for instance metadata. |
| ec2.public-ip | public address | warn | wave1 | The instance holds a routable public address, so every port its security groups leave open is reachable from the internet. Put it behind a NAT gateway or load balancer unless it must be addressed directly. |
| ec2.internet-exposed | <port(s) LIST> reachable from the internet | broken | wave2 | Sensitive ports on this instance answer from any address on the internet, so the services behind them are exposed to untargeted scanning. Narrow the security group's ingress rules to known CIDRs or reach the host through a bastion. |
| ec2.internet-exposed-all | every port reachable from the internet | broken | wave2 | A security group on this instance admits every protocol and port from any address on the internet, so nothing the host listens on is shielded. Replace the all-protocols rule with the ports the service actually needs, from the addresses that need them. |
| ec2.user-data-secret | credential in user data | broken | wave2 | A credential is stored in this instance's user data, which every principal holding ec2:DescribeInstanceAttribute can read. Move the value into Secrets Manager or Systems Manager Parameter Store and rotate it. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| tg | Target Groups | yes |
| asg | Auto Scaling Groups | yes |
| alarm | CloudWatch Alarms | yes |
| ng | EKS Node Groups | yes |
| cfn | CloudFormation Stacks | yes |
| eip | Elastic IPs | yes |
| ebs | EBS Volumes | no |
| ebs-snap | EBS Snapshots | yes |
| ct-events | CloudTrail Events | no |
| sg | Security Groups | no |
| vpc | VPC | no |
| role | IAM Role | yes |
| ami | AMI | no |
| eni | Network Interfaces | no |
| subnet | Subnet | no |
| kms | KMS Keys | yes |
| logs | Log Groups | yes |
| ssm | SSM Parameters | no |
| backup | Backup Plans | yes |
<!-- END GENERATED: related -->
