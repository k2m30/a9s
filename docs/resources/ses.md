---
shortName: ses
name: SES Identities
awsApiRef: https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_IdentityInfo.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# ses — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ses`
- **Display name**: SES Identities
- **AWS API reference**: https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_IdentityInfo.html
- **List API**: `ListEmailIdentities` (SESv2) — returns `IdentityInfo[]` with `IdentityName`, `IdentityType`, `SendingEnabled`, `VerificationStatus`.
- **Describe API (if any)**: `GetAccount` (SESv2, account-wide, one call — used for Wave 2 enforcement/quota signals). Per-identity enrichment (DKIM drift, etc.) is Wave 3 and not used.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ct-events`, `eb-rule`, `kinesis`, `lambda`, `r53`, `s3`, `sns`.

### `eb-rule`

- **Why related**: EventBridge rules that receive SES event-destination deliveries (bounce/complaint/delivery streams routed through EventBridge).
- **How discovered**: call `sesv2:GetEmailIdentity` for the selected identity → read `ConfigurationSetName` → call `sesv2:GetConfigurationSetEventDestinations` → collect `EventBridgeDestination.EventBusArn` and pivot to those EventBridge buses / rules.
- **Count shown**: yes.

### `kinesis`

- **Why related**: Kinesis Data Firehose delivery streams that receive SES event-destination deliveries (SES publishes to Firehose, not Kinesis Data Streams — the shortName is reused for UX because a9s groups Firehose under `kinesis`).
- **How discovered**: call `sesv2:GetEmailIdentity` → `ConfigurationSetName` → `sesv2:GetConfigurationSetEventDestinations` → `KinesisFirehoseDestination.DeliveryStreamArn`.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda functions invoked by SES **inbound** receipt rules (`LambdaAction`) — the receiver-side workflow for inbound mail. SES v1 feature only; SESv2 does not expose receipt rules. — a9s-devops: in SESv2-only accounts this pivot will always render 0, which is still operator-honest because it documents the absence.
- **How discovered**: call `ses:DescribeActiveReceiptRuleSet` (SES v1) → walk `Rules[].Actions[].LambdaAction.FunctionArn`. When the SDK client is SESv2-only, returns 0.
- **Count shown**: yes.

### `r53`

- **Why related**: the DNS hosted zone that owns this identity's domain — operator needs it to fix a `FAILED` verification (MX/TXT/DKIM records live in the zone).
- **How discovered**: cross-reference the already-loaded `r53` list. For `IdentityType==DOMAIN`, match `IdentityName` against hosted-zone `Name`. For `IdentityType==EMAIL_ADDRESS`, extract the domain portion (after `@`) and match the same way.
- **Count shown**: yes.

### `s3`

- **Why related**: S3 buckets where SES **inbound** receipt rules deposit received mail (`S3Action`). SES v1 feature only; SESv2 does not expose receipt rules.
- **How discovered**: call `ses:DescribeActiveReceiptRuleSet` (SES v1) → walk `Rules[].Actions[].S3Action.BucketName`. When the SDK client is SESv2-only, returns 0.
- **Count shown**: yes.

### `sns`

- **Why related**: SNS topics that receive SES event-destination notifications (bounce/complaint/delivery feedback).
- **How discovered**: call `sesv2:GetEmailIdentity` → `ConfigurationSetName` → `sesv2:GetConfigurationSetEventDestinations` → `SnsDestination.TopicArn`.
- **Count shown**: yes.

### `ct-events`

- **Why related**: universal pivot — applies to every registered type; see related-resources.md §Policy. Lets the operator audit "who changed this identity and when" during an incident.
- **How discovered**: `cloudtrail:LookupEvents` filtered by the identity's resource ARN / name.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `VerificationStatus==SUCCESS`.
  - **State bucket**: Healthy.
  - **How obtained**: `IdentityInfo.VerificationStatus` on the `ListEmailIdentities` response.
- **Signal**: `VerificationStatus==PENDING`.
  - **State bucket**: Warning.
  - **How obtained**: `IdentityInfo.VerificationStatus` on the list response.
- **Signal**: `VerificationStatus==FAILED`.
  - **State bucket**: Broken.
  - **How obtained**: `IdentityInfo.VerificationStatus` on the list response.
- **Signal**: `VerificationStatus==TEMPORARY_FAILURE`.
  - **State bucket**: Broken.
  - **How obtained**: `IdentityInfo.VerificationStatus` on the list response.
- **Signal**: `VerificationStatus==NOT_STARTED`.
  - **State bucket**: Broken.
  - **How obtained**: `IdentityInfo.VerificationStatus` on the list response.
- **Signal**: `SendingEnabled==false`.
  - **State bucket**: Warning.
  - **How obtained**: `IdentityInfo.SendingEnabled` on the list response. If combined with a non-`SUCCESS` verification status, the more severe bucket (Broken) wins.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `EnforcementStatus==PROBATION`.
  - **State bucket**: Broken.
  - **API call**: `sesv2:GetAccount` — one account-wide call.
  - **Cost shape**: account-wide.
- **Signal**: `EnforcementStatus==SHUTDOWN`.
  - **State bucket**: Broken.
  - **API call**: `sesv2:GetAccount` — one account-wide call.
  - **Cost shape**: account-wide.
- **Signal**: `SendQuota.SentLast24Hours > 0.8 × SendQuota.Max24HourSend`.
  - **State bucket**: Warning.
  - **API call**: `sesv2:GetAccount` — one account-wide call (same call as above; no extra cost).
  - **Cost shape**: account-wide.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Per-identity DKIM drift.
- OUT OF SCOPE: Reputation dashboard (`BounceRate`/`ComplaintRate`) via CloudWatch.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank** — no `OK` / `SUCCESS` / `verified`. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4, S5.
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed; S4 deduplicates with the existing per-row cause (prepend account-scope prefix where the finding is account-level); S5 still carries the full sentence; S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `VerificationStatus==PENDING` | 1 | Warning | n/a | S2, S4 | `pending verification` | `Verification in progress — add the DKIM/TXT DNS records and wait for SES to detect them.` |
| `VerificationStatus==FAILED` | 1 | Broken | n/a | S2, S4 | `verification failed` | `SES could not verify this identity — DNS records are missing or incorrect.` |
| `VerificationStatus==TEMPORARY_FAILURE` | 1 | Broken | n/a | S2, S4 | `verify: temp failure` | `Temporary SES-side issue prevented verification — retry from the SES console.` |
| `VerificationStatus==NOT_STARTED` | 1 | Broken | n/a | S2, S4 | `verification not started` | `Identity registered but verification was never initiated — start verification to enable sending.` |
| `SendingEnabled==false` (on verified identity) | 1 | Warning | n/a | S2, S4 | `sending disabled` | `Sending paused on this identity — re-enable to resume outbound mail.` |
| `EnforcementStatus==PROBATION` | 2 | Broken | `!` | S1, S3, S4, S5 | `account PROBATION` | `SES account is on probation — AWS flagged reputation issues; fix bounces/complaints before SES shuts sending down.` |
| `EnforcementStatus==SHUTDOWN` | 2 | Broken | `!` | S1, S3, S4, S5 | `account SHUTDOWN` | `SES account sending is paused by AWS — open a support case after fixing the underlying issue.` |
| `SentLast24Hours > 0.8 × Max24HourSend` | 2 | Warning | `~` | S3, S4, S5 | `quota 80%+ used` | `24h sending quota is over 80% consumed — request a quota increase before throttling begins.` |

Account-wide Wave 2 findings (`PROBATION`, `SHUTDOWN`, quota) apply to the account, not any single identity — a9s-devops: surface the finding on **every** identity row's S4 with the compact `account ...:` prefix so a glance at the list correctly attributes the problem to the account, not the identity; S1 counts the account-level finding once, not N times.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes: verification-failure rows show a red row with a short cause (`verification failed`, `verify: temp failure`, `verification not started`), paused identities show a yellow row with `sending disabled`, and account-level incidents show `account PROBATION` / `account SHUTDOWN` on every row — so the operator immediately distinguishes "this identity is broken" from "the whole SES account is in trouble" without navigating.

## 5. Out of Scope

- All §3.3 Wave 3 signals (per-identity DKIM drift; reputation dashboard via CloudWatch).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Related targets **deliberately not registered** for `ses` (from `docs/related-resources.md` Out-of-scope list): `acm` (SES uses DKIM, not ACM, for domain identities); `alarm` (general reverse-scan of CloudWatch alarms); `cfn` (tag-heuristic only); `kms` (configuration set / identity encryption is AWS-managed by default); `logs` (event destinations go to Firehose/SNS/EventBridge, not CW Logs directly); `role` (role usage is embedded in receipt-rule actions / Firehose destinations); `trail` (CloudTrail data-events link is indirect).

## 6. Citations

- `ses` appears in related-resources.md per-type table — `docs/related-resources.md` § row `| ses | API_IdentityInfo | ct-events, eb-rule, kinesis, lambda, r53, s3, sns |`.
- Per-target discovery mechanics for `eb-rule`, `kinesis`, `lambda`, `r53`, `s3`, `sns` — `docs/related-resources.md` § `### ses`.
- Out-of-scope related targets (`acm`, `alarm`, `cfn`, `kms`, `logs`, `role`, `trail`) — `docs/related-resources.md` § Out-of-scope bullets for `ses`.
- `ct-events` is a universal pivot — `docs/related-resources.md` § Policy.
- Wave 1 and Wave 2 signal list — `docs/attention-signals.md` § Backup & Email, row `ses`.
- `VerificationStatus` enum values (`PENDING`, `SUCCESS`, `FAILED`, `TEMPORARY_FAILURE`, `NOT_STARTED`) — AWS SDK Go v2 — `sesv2/types.VerificationStatus` (enum constants) and `sesv2/types.IdentityInfo` § `VerificationStatus` (field docstring). Matches the row after the amendment note in attention-signals.md.
- `IdentityInfo.SendingEnabled` field — AWS SDK Go v2 — `sesv2/types.IdentityInfo` § `SendingEnabled`.
- `IdentityType` values `EMAIL_ADDRESS` / `DOMAIN` used to decide r53 matching — AWS SDK Go v2 — `sesv2/types.IdentityType`.
- `GetAccount.EnforcementStatus` values (`HEALTHY`, `PROBATION`, `SHUTDOWN`) — AWS SDK Go v2 — `sesv2.GetAccountOutput` § `EnforcementStatus` (field docstring).
- `SendQuota.Max24HourSend` and `SendQuota.SentLast24Hours` — AWS SDK Go v2 — `sesv2/types.SendQuota` § `Max24HourSend`, `SentLast24Hours`.
- Account-wide findings should decorate every identity row's S4 with the `account ...` prefix, and S1 counts once — `a9s-devops (2026-04-20): possible=yes, worth=yes. EnforcementStatus=SHUTDOWN stops all SES sending account-wide; every identity on the list is affected. Honest attribution requires the per-row prefix "account PROBATION:" / "account SHUTDOWN:" so the operator does not misread a single identity as the culprit; counting the finding once in S1 avoids double-counting the same root cause across N rows.`
- `SendingEnabled==false` on a verified identity is a yellow (Warning) row with `sending disabled`, not green + `~` glyph — `a9s-devops (2026-04-20): possible=yes, worth=yes. The ~ annotation is reserved for background checks on fully-healthy rows (e.g. maintenance scheduled). A paused identity cannot send, so it fails the "fully healthy right now" test; bucketing Warning/yellow matches operator intuition that something is operationally off.`
- Read-only invariant (no write operations) — `docs/architecture.md` § "What is a9s?".
