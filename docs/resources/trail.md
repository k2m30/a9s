---
shortName: trail
name: CloudTrail Trails
awsApiRef: https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# trail — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `trail`
- **Display name**: CloudTrail Trails
- **AWS API reference**: <https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Trail.html>
- **List API**: `DescribeTrails` (returns `TrailList []types.Trail` with `Name`, `TrailARN`, `HomeRegion`, `S3BucketName`, `S3KeyPrefix`, `SnsTopicARN`, `KmsKeyId`, `CloudWatchLogsLogGroupArn`, `CloudWatchLogsRoleArn`, `LogFileValidationEnabled`, `IsMultiRegionTrail`, `IsOrganizationTrail`, `IncludeGlobalServiceEvents`, `HasCustomEventSelectors`, `HasInsightSelectors`).
- **Describe API (if any)**: `GetTrailStatus` (per trail) — returns `IsLogging`, `LatestDeliveryError`, `LatestDeliveryTime`, `LatestDigestDeliveryError`, `LatestDigestDeliveryTime`, `LatestNotificationError`, `LatestCloudWatchLogsDeliveryError`, `LatestCloudWatchLogsDeliveryTime`, `StartLoggingTime`, `StopLoggingTime`.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ct-events`, `kms`, `logs`, `role`, `s3`, `sns`.

### `kms`

- **Why related**: Log-file encryption key. If the trail is configured to encrypt log files with a customer-managed KMS key, a KMS disable / pending-deletion silently blocks log file write-out — the operator investigating "trail is logging but no files appear" must reach the key one key press away. Cited in `related-resources.md` §`trail` → "Trail.KmsKeyId — log-file encryption key."
- **How discovered**: Read `Trail.KmsKeyId` directly from the list response (`AWS SDK Go v2 — cloudtrail/types.Trail § KmsKeyId`, `*string`, KMS ARN or key id). If non-empty, cross-reference the already-loaded `kms` list by key ARN.
- **Count shown**: yes (single-value field, 0 or 1).

### `logs`

- **Why related**: Associated CloudWatch Logs log group. CloudTrail can stream events into a log group for near-real-time metric filters and alarms — an operator reading a trail expects to pivot to the log group actually receiving events. Cited in `related-resources.md` §`trail` → "Trail.CloudWatchLogsLogGroupArn — associated log group."
- **How discovered**: Read `Trail.CloudWatchLogsLogGroupArn` directly from the list response (`AWS SDK Go v2 — cloudtrail/types.Trail § CloudWatchLogsLogGroupArn`, `*string`, log-group ARN). If non-empty, cross-reference the already-loaded `logs` list by log-group ARN.
- **Count shown**: yes (single-value field, 0 or 1).

### `role`

- **Why related**: IAM role the CloudWatch Logs endpoint assumes to write to the user's log group; for organization trails it also underlies cross-account delivery. When log delivery fails, the role's trust policy and permissions are the second thing the operator checks after the destination. Cited in `related-resources.md` §`trail` → "CloudWatchLogsRoleArn / org-trail role."
- **How discovered**: Read `Trail.CloudWatchLogsRoleArn` directly from the list response (`AWS SDK Go v2 — cloudtrail/types.Trail § CloudWatchLogsRoleArn`, `*string`, IAM role ARN). If non-empty, cross-reference the already-loaded `role` list by role ARN. — a9s-devops: this is the only IAM role surfaced on the Trail shape; organization-trail delivery roles are managed by the org feature but do not appear as a separate field on `Trail`. possible=yes (direct field), worth=yes (CWL delivery role is the canonical failure-point when `LatestCloudWatchLogsDeliveryError` is non-empty).
- **Count shown**: yes (single-value field, 0 or 1).

### `s3`

- **Why related**: Destination bucket for raw log files — the primary evidence store for every CloudTrail. When `LatestDeliveryError` is non-empty, the bucket policy is the single most common culprit; the operator must reach the bucket directly. Cited in `related-resources.md` §`trail` → "Trail.S3BucketName — destination bucket."
- **How discovered**: Read `Trail.S3BucketName` directly from the list response (`AWS SDK Go v2 — cloudtrail/types.Trail § S3BucketName`, `*string`, bucket name — not ARN). Cross-reference the already-loaded `s3` list by bucket name.
- **Count shown**: yes (single-value field, 0 or 1 — a trail always has an S3 destination but the bucket may live in another account).

### `sns`

- **Why related**: Delivery notifications topic. If configured, CloudTrail publishes a notification each time a log file lands in S3 — downstream consumers (lambda, sqs fan-out) subscribe here. An operator debugging "my CloudTrail-driven pipeline stopped" pivots from the trail to the topic. Cited in `related-resources.md` §`trail` → "Trail.SnsTopicARN — delivery notifications."
- **How discovered**: Read `Trail.SnsTopicARN` directly from the list response (`AWS SDK Go v2 — cloudtrail/types.Trail § SnsTopicARN`, `*string`, SNS topic ARN). If non-empty, cross-reference the already-loaded `sns` list by topic ARN.
- **Count shown**: yes (single-value field, 0 or 1).

### `ct-events`

- **Why related**: Universal pivot — applies to every registered type; see `related-resources.md` §Policy. Audit trail for trail config changes (meta!) — `CreateTrail`, `UpdateTrail`, `StartLogging`, `StopLogging`, `DeleteTrail`, `PutEventSelectors`, `PutInsightSelectors`. Particularly valuable here: a trail that "mysteriously stopped" is often a deliberate `StopLogging` call recorded by another trail.
- **How discovered**: `LookupEvents` with `LookupAttributes=[{AttributeKey:ResourceName, AttributeValue:<trail-name>}]` (or `ResourceType=AWS::CloudTrail::Trail`) in the trail's `HomeRegion`.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `LogFileValidationEnabled==false` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `DescribeTrails` list response — field `LogFileValidationEnabled` (`AWS SDK Go v2 — cloudtrail/types.Trail § LogFileValidationEnabled`, `*bool`). When `false` (or nil, which AWS treats as not-enabled), log files are not signed; tamper-evidence is off — a CIS / audit baseline violation.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `IsLogging==false` → Broken (trail stopped capturing).
  - **State bucket**: Broken.
  - **API call**: `GetTrailStatus(Name=<trailARN>)` — one per trail.
  - **Cost shape**: per-resource.

- **Signal**: `LatestDeliveryError` non-empty → Broken (S3 delivery failing).
  - **State bucket**: Broken.
  - **API call**: `GetTrailStatus(Name=<trailARN>)` — one per trail (same call as above — read `LatestDeliveryError` off the same response).
  - **Cost shape**: per-resource.

- **Signal**: `LatestDeliveryTime` >1h ago on `IsLogging==true` trail → Broken (silent delivery).
  - **State bucket**: Broken.
  - **API call**: `GetTrailStatus(Name=<trailARN>)` — one per trail (same call — combine `IsLogging` and `LatestDeliveryTime` from the single response).
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `LookupEvents` absence detection.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing." **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph → S1, S3, S4, S5.
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph → S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed; S4 deduplicates; S5 still carries the full sentence; S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `LogFileValidationEnabled==false` | 1 | Warning | n/a | S2, S4 | `log validation off` | `Log-file integrity validation disabled — files are unsigned; tamper-evidence off.` |
| `IsLogging==false` | 2 | Broken | `!` | S1, S2 (red), S4, S5 (S3 suppressed on red) | `not logging` | `Trail is stopped — no API events are being captured since <StopLoggingTime>.` |
| `LatestDeliveryError` non-empty | 2 | Broken | `!` | S1, S2 (red), S4, S5 (S3 suppressed on red) | `s3 delivery failing` | `S3 delivery failing: <LatestDeliveryError> — fix bucket policy or destination.` |
| `LatestDeliveryTime` >1h stale (on `IsLogging==true`) | 2 | Broken | `!` | S1, S2 (red), S4, S5 (S3 suppressed on red) | `delivery stale >1h` | `Logging is on but no file delivered since <LatestDeliveryTime> — silent delivery failure.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword in the List text column is not acceptable — pair with cause.
- Keep both columns short: List ≤ 40 chars, Detail ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — all four signals put a concrete cause in S4 (`log validation off`, `not logging`, `s3 delivery failing`, `delivery stale >1h`), so the operator can triage trail health from the list alone; the three Wave 2 signals are all Broken (red) and the color + S4 text answer "what?" without a detail keypress, with S5 carrying the exact error string (`LatestDeliveryError`) or timestamp (`StopLoggingTime`, `LatestDeliveryTime`) for the follow-up fix.

## 5. Out of Scope

- All §3.3 Wave 3 signals (`LookupEvents` absence detection — inferring that an expected event type never arrived is an analytics workload, not a per-resource check).
- `LatestDigestDeliveryError` and `LatestNotificationError` from `GetTrailStatus` — a9s-devops: possible=yes, worth=no. Digest failures on a working trail are a niche tamper-evidence concern already covered by `LogFileValidationEnabled`; notification failures only matter to downstream SNS consumers and do not indicate the trail is losing data.
- `LatestCloudWatchLogsDeliveryError` — a9s-devops: possible=yes, worth=no for the default row. CWL delivery is optional; a trail with `CloudWatchLogsLogGroupArn==nil` cannot have this error, and when present the existing `logs` pivot + the `role` pivot already lead the operator to the fix.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- List API and list-response fields — `docs/attention-signals.md` § Monitoring table row `trail`; `AWS SDK Go v2 — cloudtrail/types.Trail § Name, TrailARN, HomeRegion, S3BucketName, SnsTopicARN, KmsKeyId, CloudWatchLogsLogGroupArn, CloudWatchLogsRoleArn, LogFileValidationEnabled, IsMultiRegionTrail, IsOrganizationTrail`.
- Describe API (Wave 2) and its fields — `docs/attention-signals.md` § Monitoring table row `trail`; `AWS SDK Go v2 — cloudtrail.GetTrailStatusOutput § IsLogging, LatestDeliveryError, LatestDeliveryTime, StopLoggingTime`.
- Related targets (full list) — `docs/related-resources.md` § Per-type contract, row `trail`; detailed reasoning in `docs/related-resources.md` § `trail`.
- `kms` via `Trail.KmsKeyId` — `AWS SDK Go v2 — cloudtrail/types.Trail § KmsKeyId`; `docs/related-resources.md` § `trail` ("Trail.KmsKeyId — log-file encryption key").
- `logs` via `Trail.CloudWatchLogsLogGroupArn` — `AWS SDK Go v2 — cloudtrail/types.Trail § CloudWatchLogsLogGroupArn`; `docs/related-resources.md` § `trail` ("Trail.CloudWatchLogsLogGroupArn — associated log group").
- `role` via `Trail.CloudWatchLogsRoleArn` — `AWS SDK Go v2 — cloudtrail/types.Trail § CloudWatchLogsRoleArn`; `docs/related-resources.md` § `trail` ("CloudWatchLogsRoleArn / org-trail role") — a9s-devops (2026-04-20): possible=yes, worth=yes. CWL delivery role is the canonical failure-point when `LatestCloudWatchLogsDeliveryError` is non-empty; org-trail role is managed by the org feature and does not appear as a separate field on `Trail`.
- `s3` via `Trail.S3BucketName` — `AWS SDK Go v2 — cloudtrail/types.Trail § S3BucketName`; `docs/related-resources.md` § `trail` ("Trail.S3BucketName — destination bucket"). Note: the field carries a bucket name, not an ARN; the loaded `s3` list is keyed by bucket name so the match is direct.
- `sns` via `Trail.SnsTopicARN` — `AWS SDK Go v2 — cloudtrail/types.Trail § SnsTopicARN`; `docs/related-resources.md` § `trail` ("Trail.SnsTopicARN — delivery notifications"). The deprecated `SnsTopicName` is ignored in favor of the ARN per the SDK's own deprecation note.
- `ct-events` universal pivot — `docs/related-resources.md` § Policy item 4 ("`ct-events` … is implicitly relevant for every registered type"); per-row note in `docs/related-resources.md` § `trail` ("Audit trail for trail config changes (meta!)").
- Discovery mechanism for every direct-field pivot (read a single `*string` off the `Trail` list-response record) — a9s-devops (2026-04-20): possible=yes, worth=yes. All five non-universal related targets for `trail` are direct fields on the `DescribeTrails` response; no extra API call or enrichment is needed — this is a uniquely cheap related panel.
- Wave 1 `LogFileValidationEnabled==false` → Warning — `docs/attention-signals.md` § Monitoring row `trail`.
- Wave 2 `IsLogging==false` → Broken — `docs/attention-signals.md` § Monitoring row `trail`; `AWS SDK Go v2 — cloudtrail.GetTrailStatusOutput § IsLogging`.
- Wave 2 `LatestDeliveryError` non-empty → Broken — `docs/attention-signals.md` § Monitoring row `trail`; `AWS SDK Go v2 — cloudtrail.GetTrailStatusOutput § LatestDeliveryError`.
- Wave 2 `LatestDeliveryTime` >1h stale while `IsLogging==true` → Broken — `docs/attention-signals.md` § Monitoring row `trail`; `AWS SDK Go v2 — cloudtrail.GetTrailStatusOutput § LatestDeliveryTime, IsLogging`.
- Wave 3 `LookupEvents` absence detection (OUT OF SCOPE) — `docs/attention-signals.md` § Monitoring row `trail`.
- `!` severity for the three Wave 2 Broken signals — a9s-devops (2026-04-20): possible=yes, worth=yes. A stopped trail, a failing S3 delivery, or a silent delivery are all cases where the audit record is actively being lost — this is a compliance/security break and must bump the menu `issues:N` count; `~` would under-sell the risk. S3 is suppressed per the HOW rule because the row is already red.
- Out-of-scope rationale for `LatestDigestDeliveryError`, `LatestNotificationError`, `LatestCloudWatchLogsDeliveryError` — a9s-devops (2026-04-20): possible=yes, worth=no. The golden-doc signals already cover the actively-losing-data cases; these extra fields surface niche sub-failures whose fix path is the existing `kms` / `logs` / `role` pivots.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
