---
shortName: ct-events
name: CloudTrail Events
awsApiRef: https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
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

Expected targets from `docs/related-resources.md` § Per-type contract: `cfn`, `ct-events` (self-pivot — four facets), `dbi`, `ddb`, `ec2`, `iam-user`, `kms`, `lambda`, `role`, `s3`, `secrets`, `sg`, `trail`, `vpce`.

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

### `ct-events`

The self-pivot carries four facets, all convenience filters that re-launch `LookupEvents` with a lookup attribute derived from the current event, letting the operator broaden the query without leaving the panel.

- **By AccessKeyId** — filter by `userIdentity.accessKeyId` to see every call made by the same credential (key-compromise forensics).
- **By Username** — filter by `userIdentity.userName` to see every call made by the same IAM user across services.
- **By EventName** — filter by `eventName` to see every occurrence of the same API call across the account (e.g. every `ConsoleLogin`, every `DeleteObject`).
- **By SharedEventId** — filter by `sharedEventId` to group events that share a cross-service request (CloudTrail assigns a common id when one customer action produces multiple events).
- **Count shown**: yes for each facet.
- **Discovery**: parse the four fields out of `Event.CloudTrailEvent` JSON on the currently-selected event; no extra AWS call is made until the operator picks a facet.

**Universal pivot note.** ct-events is the **universal pivot** referenced by every other registered type (see `docs/related-resources.md` §Policy, rule 4: "`ct-events` is implicitly relevant for every registered type"). The panel on those other types carries a single `ct-events` entry pre-scoped to that resource's ARN; the rich self-pivot structure above only appears when the operator is already *on* a ct-events row.

## 3. Attention / Issues Algorithm

**Source API**: [LookupEvents](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html)

Transcribed from `docs/attention-signals.md § Signals § MONITORING` row `ct-events`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal, in the order they are tried: the first that
matches is the one the event reports. Every field comes from the
`LookupEvents` response or from the `Event.CloudTrailEvent` JSON parsed on it;
no extra API call is made.

- **Signal**: an `errorCode` on the event → AWS rejected the call.
  - **State bucket**: Broken.
  - **How obtained**: the top-level `errorCode` key of the parsed event; the code is humanized into the cause text.

- **Signal**: the call deletes or tears something down.
  - **State bucket**: Broken.
  - **How obtained**: the event name classifies as destructive.

- **Signal**: the call changed configuration.
  - **State bucket**: Warning.
  - **How obtained**: the event name classifies as a write.

- **Signal**: the account root user made the call.
  - **State bucket**: Warning.
  - **How obtained**: `userIdentity.type == "Root"` on the parsed event.

- **Signal**: the caller's account differs from the account that recorded the event.
  - **State bucket**: Warning.
  - **How obtained**: `userIdentity.accountId` compared with `recipientAccountId`.

- **Signal**: the call reads secret or parameter material.
  - **State bucket**: Warning.
  - **How obtained**: `<service>:<eventName>` matched against the sensitive-read list in `core/aws/ct_events.go`; the event name goes into the cause text.

- **Signal**: anything else.
  - **State bucket**: Dim.
  - **How obtained**: no rule above matched, so the event is routine.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Absence-of-expected-events alerting.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `ct-events`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count.
<!-- END GENERATED: badge -->

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| destructive call (`ct_event.severity.danger`) | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `destructive call` |
| call AWS rejected (`ct_event.danger.failed`) | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `failed: <error>` |
| root user made the call (`ct_event.severity.attention`) | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `root account activity` |
| call changed configuration (`ct_event.attention.write`) | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `modifying call` |
| caller from another account (`ct_event.attention.cross-account`) | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `cross-account access` |
| read of secret or parameter material (`ct_event.attention.sensitive-read`) | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `reads sensitive data (<event>)` |
| every other event (`ct_event.severity.info`) | 1 | Dim | n/a | S2, S4 | `routine event` |

Rules for filling list and detail text:

- Banned words: `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`. None appear above.
- No bare state keyword: a rejected call reads `failed: AccessDenied`, naming the error AWS returned rather than the word `failed` alone.
- `reads sensitive data (<event>)` names the call that read the material, so the operator sees which secret surface was touched without opening the event.

## 4.1 UX review (two sentences)

At 3am, glancing at a ct-events list filtered by an anxious operator, can they tell what's wrong with a problem row without opening detail? Yes — a red row reads `destructive call` or `failed: <error>` and a yellow row reads `root account activity` or `cross-account access`, each naming the kind of call by the time the eye crosses the Status column; operator can triage without opening detail. The event name and the caller sit in their own columns, so the cause text never has to repeat them.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Cross-referencing events against identity providers outside AWS (Okta, Azure AD) — a9s-devops: not worth it, the CloudTrail `userIdentity.federatedProvider` string is enough for in-UI filtering; full IdP correlation belongs in a SIEM.

## 6. Citations

- a9s golden doc — the `ct-events` findings — `docs/attention-signals.md § Signals § MONITORING` row `ct-events`.
- a9s golden doc — per-type contract lists `cfn, ct-events, dbi, ddb, ec2, iam-user, kms, lambda, role, s3, secrets, sg, trail, vpce` — `docs/related-resources.md` § `Per-type contract`, row `ct-events`.
- a9s golden doc — four self-pivot facets (AccessKeyId / Username / EventName / SharedEventId) — `docs/related-resources.md` § `Per-target reasoning` → `### ct-events`.
- a9s golden doc — universal-pivot rule (ct-events applies to every registered type) — `docs/related-resources.md` § `Policy`, rule 4.
- a9s golden doc — read-only invariant — `docs/architecture.md` § `What is a9s?`.
- AWS Go SDK v2 — `Event.ReadOnly` is `*string` (confirms the `=="false"` string comparison the read-only check makes) — `AWS SDK Go v2 — cloudtrail/types.Event § ReadOnly`.
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
ct-events — MONITORING. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| ct\_event.severity.danger | destructive call | broken | wave1 | CloudTrail recorded a call that deletes or tears something down. Verify it was expected and, if not, find out who made it. |
| ct\_event.danger.failed | failed: <error> | broken | wave1 | CloudTrail recorded a call that AWS rejected; the error code is in the phrase. A denied call is either a permission gap or someone probing for one. |
| ct\_event.severity.attention | root account activity | warn | wave1 | CloudTrail recorded a call made by the account root user. Root should not be doing day-to-day work; move the task onto a named principal. |
| ct\_event.attention.write | modifying call | warn | wave1 | CloudTrail recorded a call that changed configuration. Verify the change was expected and that whoever made it meant to. |
| ct\_event.attention.cross-account | cross-account access | warn | wave1 | The caller belongs to a different account than the one that recorded the event, so this is access across an account boundary. Verify the trust it came through is one you meant to grant. |
| ct\_event.attention.sensitive-read | reads sensitive data (<event>) | warn | wave1 | CloudTrail recorded a read of secret or parameter material. Verify the caller was expected: a read leaves the value in the caller's hands with nothing to revoke afterwards. |
| ct\_event.severity.info | routine event | dim | wave1 | — |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| role | IAM Roles | no |
| iam-user | IAM Users | no |
| ec2 | EC2 Instances | no |
| s3 | S3 Buckets | no |
| lambda | Lambda Functions | no |
| dbi | RDS Instances | no |
| kms | KMS Keys | no |
| secrets | Secrets | no |
| vpce | VPC Endpoints | no |
| sg | Security Groups | no |
| ddb | DynamoDB Tables | no |
| cfn | CloudFormation Stacks | no |
| trail | CloudTrail Trails | no |
| ct-events | CT events by AccessKeyId | no |
| ct-events | CT events by Username | no |
| ct-events | CT events by EventName | no |
| ct-events | CT events by SharedEventId | no |
<!-- END GENERATED: related -->
