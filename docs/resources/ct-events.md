---
shortName: ct-events
name: CloudTrail Events
awsApiRef: https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# ct-events — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

Note on resource shape: a ct-events row is one CloudTrail **event** (a point-in-time log entry returned by `LookupEvents`), not a long-lived AWS resource. Each row represents a single API call recorded by CloudTrail. Identity is `EventId`; "state" is "did this call succeed, or did it fail, and how?". This is also the **universal pivot** — every other a9s resource type surfaces `ct-events` as a related target to reach its audit trail.

## 1. Identity

- **shortName**: `ct-events`
- **Display name**: CloudTrail Events
- **AWS API reference**: <https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html>
- **List API**: `LookupEvents`
- **Describe API (if any)**: not used — all event data arrives on the list response. The richest fields (`errorCode`, `errorMessage`, `userIdentity`, `requestParameters`) are embedded as a JSON string in `Event.CloudTrailEvent` and must be parsed client-side.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `cfn`, `ct-events` (self-pivot — 4 facets), `dbi`, `ddb`, `ec2`, `iam-user`, `kms`, `lambda`, `role`, `s3`, `secrets`, `sg`, `trail`, `vpce`.

The detail view of a ct-events row is "who did what to which AWS resource?" — so the related panel is the set of principals and target resources extracted from the event payload.

### `iam-user`

- **Why related**: the human or machine identity that made the API call — the "who" of the event.
- **How discovered**: parse `Event.CloudTrailEvent` JSON and read `userIdentity.userName` when `userIdentity.type == "IAMUser"`; cross-reference the already-loaded `iam-user` list by name.
- **Count shown**: yes.

### `role`

- **Why related**: the assumed-role identity that made the API call — the "who" for STS-derived sessions.
- **How discovered**: parse `Event.CloudTrailEvent` JSON and read `userIdentity.sessionContext.sessionIssuer.arn` when `userIdentity.type == "AssumedRole"`; cross-reference the already-loaded `role` list by ARN.
- **Count shown**: yes.

### `ec2`

- **Why related**: an EC2 instance targeted by this event — the "what" the operator wants to jump to after seeing a suspicious call.
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::EC2::Instance"` (or whose `ResourceName` matches an `i-*` pattern); cross-reference the already-loaded `ec2` list.
- **Count shown**: yes.

### `s3`

- **Why related**: an S3 bucket referenced by the event — management-plane (policy, ACL, encryption) and data-plane (if data-events are enabled) calls.
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::S3::Bucket"`; cross-reference the already-loaded `s3` list by bucket name.
- **Count shown**: yes.

### `lambda`

- **Why related**: a Lambda function invoked or reconfigured by the event.
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::Lambda::Function"`; cross-reference the already-loaded `lambda` list.
- **Count shown**: yes.

### `dbi`

- **Why related**: an RDS DB instance touched by an RDS management call (e.g. `ModifyDBInstance`, `RebootDBInstance`).
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::RDS::DBInstance"`; cross-reference the already-loaded `dbi` list.
- **Count shown**: yes.

### `kms`

- **Why related**: a KMS key whose usage (Encrypt/Decrypt/GenerateDataKey) or policy was touched — core for key-access forensics.
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::KMS::Key"`; cross-reference the already-loaded `kms` list by key ID/ARN.
- **Count shown**: yes.

### `secrets`

- **Why related**: a Secrets Manager secret whose value was accessed or which was rotated/modified.
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::SecretsManager::Secret"`; cross-reference the already-loaded `secrets` list by ARN.
- **Count shown**: yes.

### `vpce`

- **Why related**: a VPC endpoint whose policy or lifecycle was changed — reachable from events like `ModifyVpcEndpoint`.
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::EC2::VPCEndpoint"`; cross-reference the already-loaded `vpce` list.
- **Count shown**: yes.

### `sg`

- **Why related**: a security group whose rules were changed (e.g. `AuthorizeSecurityGroupIngress`) — the "who opened port X?" pivot.
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::EC2::SecurityGroup"`; cross-reference the already-loaded `sg` list.
- **Count shown**: yes.

### `ddb`

- **Why related**: a DynamoDB table whose schema or capacity was changed.
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::DynamoDB::Table"`; cross-reference the already-loaded `ddb` list.
- **Count shown**: yes.

### `cfn`

- **Why related**: a CloudFormation stack whose lifecycle or state was changed (`CreateStack`, `UpdateStack`, `DeleteStack`).
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::CloudFormation::Stack"`; cross-reference the already-loaded `cfn` list.
- **Count shown**: yes.

### `trail`

- **Why related**: meta-audit — a CloudTrail trail whose config was changed (`StopLogging`, `UpdateTrail`, `DeleteTrail`). A trail being altered is itself a detection signal.
- **How discovered**: iterate `Event.Resources[]` and keep entries whose `ResourceType == "AWS::CloudTrail::Trail"`; cross-reference the already-loaded `trail` list.
- **Count shown**: yes.

### `ct-events` (self-pivot — four facets)

The ct-events self-pivots are convenience filters that re-launch `LookupEvents` with a lookup attribute derived from the current event, letting the operator broaden the query without leaving the panel.

- **By AccessKeyId** — filter by `userIdentity.accessKeyId` to see every call made by the same credential (key-compromise forensics).
- **By Username** — filter by `userIdentity.userName` to see every call made by the same IAM user across services.
- **By EventName** — filter by `eventName` to see every occurrence of the same API call across the account (e.g. every `ConsoleLogin`, every `DeleteObject`).
- **By SharedEventId** — filter by `sharedEventId` to group events that share a cross-service request (CloudTrail assigns a common id when one customer action produces multiple events).
- **Count shown**: yes for each facet.
- **Discovery**: parse the four fields out of `Event.CloudTrailEvent` JSON on the currently-selected event; no extra AWS call is made until the operator picks a facet.

### Universal pivot note

ct-events is the **universal pivot** referenced by every other registered type (see `related-resources.md` §Policy, rule 4: "`ct-events` is implicitly relevant for every registered type"). The panel on those other types carries a single `ct-events` entry pre-scoped to that resource's ARN; the rich self-pivot structure above only appears when the operator is already *on* a ct-events row.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md` row `ct-events`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `Event.ReadOnly == "false"` (field is a string on the SDK `Event` shape) — isolates write-attempts vs read-only calls. On its own this is not a state-bucket verdict, it is a **filter facet** used by the other signals and by the operator's query; for a write-attempt whose `errorCode` is empty the event is Healthy.
  - **State bucket**: Healthy (write succeeded).
  - **How obtained**: `Event.ReadOnly` field on the `LookupEvents` response.

- **Signal**: parsed `errorCode` present in `Event.CloudTrailEvent` JSON → a single failed API call.
  - **State bucket**: Warning.
  - **How obtained**: parse `Event.CloudTrailEvent` (raw JSON string) on the list response and read the top-level `errorCode` key. No extra API call.

- **Signal**: count of events with `errorCode == "AccessDenied"` AND `ReadOnly == "false"` in the last hour, grouped by principal (`userIdentity.arn`) > N → a credential being brute-force-probed against the write surface.
  - **State bucket**: Broken.
  - **How obtained**: client-side aggregation over the already-loaded `LookupEvents` page. No extra API call. Threshold `N` — see §6 `a9s-devops consultation`.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Absence-of-expected-events alerting.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing." `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `AccessDenied: iam:DeleteUser`). **Healthy rows render blank** — no `OK` / `Success`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping applied here:

- Write success (Healthy, `ReadOnly=="false"` + no `errorCode`) → no §4 row. S2 renders green, S4 renders blank. Silence is the UX.
- Read success (Healthy, `ReadOnly=="true"` + no `errorCode`) → no §4 row. Same as above.
- Wave 1 Warning (single `errorCode` on an event) → S2 (yellow) + S4 (cause text). No S1, S3, S5.
- Wave 1 Broken (AccessDenied-write storm by principal > N/h) → S2 (red) + S4 (cause text). No S1, S3, S5 — consistent with the "Wave 1 Warning/Broken/Dim" rule.

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `errorCode` present on event | 1 | Warning | n/a | S2 + S4 | `<errorCode>: <eventSource>:<eventName>` | — (Wave 1 Warning does not reach S5) |
| AccessDenied-write storm by principal > N/h | 1 | Broken | n/a | S2 + S4 | `AccessDenied storm: <principal> Nx in 1h` | — (Wave 1 Broken does not reach S5) |

Rules for filling list and detail text:

- Banned words: `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`. None appear above.
- No bare state keyword: `AccessDenied` alone is a state keyword; the S4 text always pairs it with the event source and name (`AccessDenied: iam:DeleteUser`) so the operator can see *what* was denied.
- The storm row shows `principal` as the short form of `userIdentity.arn` (e.g. last ARN segment or IAM username), `N` as the actual count.

## 4.1 UX review (two sentences)

At 3am, glancing at a ct-events list filtered by an anxious operator, can they tell what's wrong with a problem row without opening detail? Yes — a yellow row reads e.g. `AccessDenied: iam:DeleteUser` and a red row reads e.g. `AccessDenied storm: alice 23x in 1h`, both of which name the principal or the call by the time the eye crosses the Status column; operator can triage without opening detail. The storm row in particular is the highest-signal log-forensics line a9s produces, and it is self-explanatory on the list.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Cross-referencing events against identity providers outside AWS (Okta, Azure AD) — a9s-devops: not worth it, the CloudTrail `userIdentity.federatedProvider` string is enough for in-UI filtering; full IdP correlation belongs in a SIEM.

## 6. Citations

- a9s golden doc — ct-events has an attention-signals row — `docs/attention-signals.md` § `Monitoring` table, row `ct-events`.
- a9s golden doc — per-type contract lists `cfn, ct-events, dbi, ddb, ec2, iam-user, kms, lambda, role, s3, secrets, sg, trail, vpce` — `docs/related-resources.md` § `Per-type contract`, row `ct-events`.
- a9s golden doc — four self-pivot facets (AccessKeyId / Username / EventName / SharedEventId) — `docs/related-resources.md` § `Per-target reasoning` → `### ct-events`.
- a9s golden doc — universal-pivot rule (ct-events applies to every registered type) — `docs/related-resources.md` § `Policy`, rule 4.
- a9s golden doc — read-only invariant — `docs/architecture.md` § `What is a9s?`.
- AWS Go SDK v2 — `Event.ReadOnly` is `*string` (confirms the `=="false"` string comparison in attention-signals) — `AWS SDK Go v2 — cloudtrail/types.Event § ReadOnly`.
- AWS Go SDK v2 — `Event.CloudTrailEvent` is a JSON string carrying the full event body — `AWS SDK Go v2 — cloudtrail/types.Event § CloudTrailEvent`.
- AWS Go SDK v2 — `Event.Resources []Resource` with `Resource.ResourceType` and `Resource.ResourceName` — basis for all non-self related pivots — `AWS SDK Go v2 — cloudtrail/types.Event § Resources`, `AWS SDK Go v2 — cloudtrail/types.Resource § ResourceType`, `AWS SDK Go v2 — cloudtrail/types.Resource § ResourceName`.
- AWS Go SDK v2 — `Event.Username`, `Event.AccessKeyId`, `Event.EventName`, `Event.EventId`, `Event.EventTime`, `Event.EventSource` present directly on the SDK struct; they are echoed from the JSON but also exposed top-level for cheap access — `AWS SDK Go v2 — cloudtrail/types.Event`.
- AWS API Reference — `LookupEvents` reference page — `AWS API Reference: LookupEvents` (`https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html`).
- a9s-devops consultation — threshold `N` for the AccessDenied-write-storm Broken signal — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. A practitioner SOC rule of thumb is N=10 AccessDenied writes from the same principal in a 1h window — that's above normal fat-finger (1–3 denies on a misremembered API) and below noisy automation that is usually exempted by tag. N should be configurable per profile; the default in this spec is 10 and is a product call pending user confirmation.`
- a9s-devops consultation — event rows are not long-lived resources; Healthy rows omit §4 entirely — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. Operators filter ct-events down to a time window and then scan for red/yellow rows; adding "Success" text on every green row would quadruple noise. Blank S4 on Healthy is the correct answer.`
- a9s-devops consultation — Wave 1 Broken does not reach S5 in this spec — `a9s-devops persona (2026-04-20): possible=yes, worth=no. The storm signal is aggregation over a page, not a property of one event row; a per-row detail sentence would either repeat the list text or claim an aggregate truth on the wrong scope. The aggregate belongs in S4 or in a dedicated "storms" view, both of which are covered by S2+S4 here.`
- a9s-devops consultation — no Wave 2 even though CloudTrail has richer APIs — `a9s-devops persona (2026-04-20): possible=yes (GetEventSelectors, GetInsightSelectors exist), worth=no for the ct-events row itself. Those APIs describe *trails*, not events, and their signals are already owned by the trail spec. Adding them here would duplicate work and conflate two mental models.`
- a9s-devops consultation — `ct-events` self-pivot is exposed as four distinct menu items rather than one generic "filter" — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. Four facets = four common forensics questions ("what else did this key do?", "every ConsoleLogin this week?", etc.) each worth a one-keystroke pivot; collapsing into a single generic filter would add a form prompt and cost time during an incident.`

<!-- BEGIN GENERATED: header -->
ct-events — MONITORING. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| role | IAM Roles | yes |
| iam-user | IAM Users | yes |
| ec2 | EC2 Instances | yes |
| s3 | S3 Buckets | yes |
| lambda | Lambda Functions | yes |
| dbi | RDS Instances | yes |
| kms | KMS Keys | yes |
| secrets | Secrets | yes |
| vpce | VPC Endpoints | yes |
| sg | Security Groups | yes |
| ddb | DynamoDB Tables | yes |
| cfn | CloudFormation Stacks | yes |
| trail | CloudTrail Trails | yes |
| ct-events | CT events by AccessKeyId | no |
| ct-events | CT events by Username | no |
| ct-events | CT events by EventName | no |
| ct-events | CT events by SharedEventId | no |
<!-- END GENERATED: related -->
