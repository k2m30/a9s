---
shortName: kinesis
name: Kinesis Streams
awsApiRef: https://docs.aws.amazon.com/kinesis/latest/APIReference/API_StreamDescription.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# kinesis — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `kinesis`
- **Display name**: Kinesis Streams
- **AWS API reference**: <https://docs.aws.amazon.com/kinesis/latest/APIReference/API_StreamDescription.html>
- **List API**: `ListStreams` — returns `StreamSummaries []StreamSummary` plus a parallel `StreamNames []string`. `StreamSummary` carries `StreamARN`, `StreamName`, `StreamStatus`, `StreamCreationTimestamp`, `StreamModeDetails` — enough for the one Wave 1 signal (stream status). The SDK shape is authoritative here; the `attention-signals.md` "Source" column cites `DescribeStreamSummary`, which is the per-stream Wave 2 call (see next field).
- **Describe API (if any)**: `DescribeStreamSummary` per stream — returns `StreamDescriptionSummary`, which is the only shape that carries `KeyId` (the CMK ARN used for server-side encryption) and `EncryptionType`. Not used by any Wave 2 attention signal in `attention-signals.md`, but required for the `kms` related-panel pivot (see §2).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `cfn`, `ct-events`, `ddb`, `kms`, `lambda`.

### `alarm`

- **Why related**: CloudWatch alarms on `IteratorAge` / `IncomingRecords` / `WriteProvisionedThroughputExceeded` — the primary observable health signal for a Kinesis stream, since stream-level health beyond `StreamStatus` lives entirely in CloudWatch.
- **How discovered**: cross-reference the already-loaded `alarm` list; keep alarms whose `Namespace == "AWS/Kinesis"` and whose `Dimensions[]` contains `{Name: "StreamName", Value: <this stream's StreamName>}` — a9s-devops: this is the idiomatic reverse scan; no per-stream API call required when the `alarm` list has been loaded this session.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created the stream — operators routinely pivot from a stream to "what owns this? who deploys changes to it?" when investigating drift, outages, or on-call handoff.
- **How discovered**: call `ListTagsForStream(StreamName=...)` (per stream) and read the tag `aws:cloudformation:stack-name`; cross-reference the already-loaded `cfn` list by stack name — a9s-devops: CloudFormation stamps this tag on every managed resource; Kinesis tags are not on `ListStreams` or `DescribeStreamSummary` so the per-stream tag call is unavoidable but cheap and cached with the detail view.
- **Count shown**: yes (0 or 1).

### `ct-events`

- **Why related**: Universal pivot — who created, updated shard count, changed retention, enabled encryption, or deleted this stream.
- **How discovered**: pre-built CloudTrail query scoped to the stream name / ARN as the resource identifier.
- **Count shown**: unknown (CloudTrail queries are windowed; a reliable total isn't available without a separate count call).
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy.

### `ddb`

- **Why related**: DynamoDB tables that stream change data into this Kinesis stream via `EnableKinesisStreamingDestination` — when a DDB table has a Kinesis destination, this stream is the sink.
- **How discovered**: reverse scan of the already-loaded `ddb` list calling `DescribeKinesisStreamingDestination` per table and matching `KinesisDataStreamDestinations[].StreamArn` — a9s-devops: possible=yes, worth=no for the daily driver. Operators almost always pivot DDB→Kinesis (the DDB detail page naturally shows where its stream goes), not the reverse. Recording the reverse pivot here would add an N-table API cost on every stream detail view for a workflow that fires rarely. See §5 Out of Scope.
- **Count shown**: n/a (pivot not implemented).

### `kms`

- **Why related**: Customer-managed KMS key encrypting server-side data on this stream — operators need this for encryption audits, key-rotation reviews, and access-denied debugging ("why can't this consumer read?").
- **How discovered**: call `DescribeStreamSummary(StreamName=...)`, read `StreamDescriptionSummary.KeyId` (field is absent or `alias/aws/kinesis` when AWS-managed; non-empty customer ARN or `alias/<name>` when CMK-encrypted); cross-reference the already-loaded `kms` list by key ARN or alias — a9s-devops: `KeyId` is on `StreamDescriptionSummary`, not on `StreamSummary`, so the describe call is required; the pivot is worth it because CMK audit is a standard security review step.
- **Count shown**: yes (0 or 1).

### `lambda`

- **Why related**: Lambda functions consuming records from this stream via event-source mappings — the first question an operator asks about a lagging stream is "who's reading this, and are they keeping up?"
- **How discovered**: reverse scan of the already-loaded Lambda event-source-mapping set; keep entries where `EventSourceMappingConfiguration.EventSourceArn == <this stream's StreamARN>` — a9s-devops: ESM is the authoritative link (documented on the SDK shape as `(Kinesis, DynamoDB Streams, Amazon MSK, ...)`); scanning an already-loaded ESM list is zero-cost and is the canonical way to surface consumers.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `StreamStatus == ACTIVE`.
  - **State bucket**: Healthy.
  - **How obtained**: `StreamSummary.StreamStatus` from `ListStreams`.

- **Signal**: `StreamStatus == CREATING`.
  - **State bucket**: Warning.
  - **How obtained**: `StreamSummary.StreamStatus` from `ListStreams`.

- **Signal**: `StreamStatus == UPDATING`.
  - **State bucket**: Warning.
  - **How obtained**: `StreamSummary.StreamStatus` from `ListStreams`.

- **Signal**: `StreamStatus == DELETING`.
  - **State bucket**: Warning.
  - **How obtained**: `StreamSummary.StreamStatus` from `ListStreams`.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

Copy from `attention-signals.md` Wave 3 cell:

- OUT OF SCOPE: CloudWatch `GetRecords.IteratorAgeMilliseconds` (consumer lag).
- OUT OF SCOPE: CloudWatch `WriteProvisionedThroughputExceeded`.
- OUT OF SCOPE: CloudWatch `ReadProvisionedThroughputExceeded`.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank** — no `OK` / `ACTIVE`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4 (short cause), S5 (full sentence). No S1.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `StreamStatus == CREATING` | 1 | Warning | n/a | S2, S4 | `creating` | `Stream is being provisioned; writes and reads are not yet available.` |
| `StreamStatus == UPDATING` | 1 | Warning | n/a | S2, S4 | `updating: resharding` | `Shards are being merged or split; reads and writes continue to work during the update.` |
| `StreamStatus == DELETING` | 1 | Warning | n/a | S2, S4 | `deleting` | `Stream is being deleted; reads and writes will fail.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword alone is not acceptable when a cause helps. For `UPDATING` the cause ("resharding") is the only useful amplification AWS exposes at list time; for `CREATING` / `DELETING` the state word is itself the cause because no richer reason field exists on `StreamSummary`.
- `StreamStatus == ACTIVE` is omitted from this table — healthy rows render blank per the rules.
- Keep both columns short: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, a yellow Kinesis row reads `creating` / `updating: resharding` / `deleting` in the Status column — the operator instantly knows whether the stream is being brought up, reshaped, or torn down without opening detail. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): CloudWatch `IteratorAgeMilliseconds`, `WriteProvisionedThroughputExceeded`, `ReadProvisionedThroughputExceeded`. These are the metrics that actually answer "is this stream healthy in production" — they are excluded from a9s because they exceed the Wave 2 cost budget, not because they are low value.
- **`ddb` reverse pivot** — listing DDB tables that stream into this Kinesis stream via `EnableKinesisStreamingDestination`. Possible via per-table `DescribeKinesisStreamingDestination`, but worth=no: operators pivot DDB→Kinesis in practice, not the reverse; the N-table cost on every stream detail view is not justified by daily workflow. See §2 `ddb`.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

One bullet per claim in §§2–4.1.

- a9s golden doc — per-type contract row for `kinesis` — `docs/related-resources.md` § Per-type contract (row `kinesis` → `alarm, cfn, ct-events, ddb, kms, lambda`).
- a9s golden doc — detailed `kinesis` related list — `docs/related-resources.md` § `kinesis` (AWS API: `API_StreamDescription`).
- a9s golden doc — Wave 1 `StreamStatus` bucketing and absence of Wave 2 signals — `docs/attention-signals.md` § Messaging (row `kinesis`).
- a9s golden doc — Wave 3 CloudWatch metrics explicitly excluded — `docs/attention-signals.md` § Messaging (row `kinesis`, Wave 3 cell).
- a9s golden doc — universal `ct-events` pivot policy — `docs/related-resources.md` § Policy.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- AWS Go SDK v2 — `StreamSummary` carries `StreamStatus` on the list response — `AWS SDK Go v2 — kinesis/types.StreamSummary § StreamStatus`.
- AWS Go SDK v2 — `StreamStatus` enum values `CREATING`, `DELETING`, `ACTIVE`, `UPDATING` — `AWS SDK Go v2 — kinesis/types.StreamStatus`.
- AWS Go SDK v2 — `KeyId` / `EncryptionType` live on `StreamDescriptionSummary` (from `DescribeStreamSummary`), not on `StreamSummary` — `AWS SDK Go v2 — kinesis/types.StreamDescriptionSummary § KeyId, § EncryptionType`.
- AWS Go SDK v2 — `ListStreamsOutput` returns both `StreamSummaries` and `StreamNames` — `AWS SDK Go v2 — kinesis.ListStreamsOutput § StreamSummaries, § StreamNames`.
- AWS Go SDK v2 — Lambda ESM link to Kinesis by stream ARN — `AWS SDK Go v2 — lambda/types.EventSourceMappingConfiguration § EventSourceArn` (comment notes "Kinesis, DynamoDB Streams, Amazon MSK, and self-managed Apache Kafka").
- AWS Go SDK v2 — tag shape on Kinesis — `AWS SDK Go v2 — kinesis/types.Tag § Key, § Value`.
- a9s-devops consultation — `alarm` discovery via reverse scan of loaded `alarm` list on `Namespace==AWS/Kinesis` and `Dimensions.StreamName` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Idiomatic reverse-scan pivot; IteratorAge/throughput alarms are the primary Kinesis health signal an operator looks for.`
- a9s-devops consultation — `cfn` discovery via `ListTagsForStream` + `aws:cloudformation:stack-name` tag — `a9s-devops (2026-04-20): possible=yes, worth=yes. CloudFormation stamps this tag on every managed resource; tags are not on ListStreams/DescribeStreamSummary so a per-stream call is unavoidable but cheap.`
- a9s-devops consultation — `kms` discovery requires Wave-2 `DescribeStreamSummary` to read `KeyId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. KeyId is absent from StreamSummary; the describe call is justified because CMK audit is a standard security review step operators repeat.`
- a9s-devops consultation — `lambda` discovery via reverse scan of ESM by `EventSourceArn` — `a9s-devops (2026-04-20): possible=yes, worth=yes. ESM is the canonical consumer link; scanning an already-loaded ESM set is zero-cost and answers "who is reading this stream?" the first question operators ask about a lagging stream.`
- a9s-devops consultation — `ddb` reverse pivot omitted — `a9s-devops (2026-04-20): possible=yes, worth=no. DescribeKinesisStreamingDestination would be required per DDB table in the loaded list; operators pivot DDB→Kinesis in practice, not the reverse, so the N-table cost is unjustified for a rare workflow. Recorded in §5 Out of Scope.`
