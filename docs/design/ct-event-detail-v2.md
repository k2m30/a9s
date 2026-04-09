# CloudTrail Event Detail View — Design v2

Status: **proposed, awaiting visual approval via `cmd/preview/ct_event/`**.
Supersedes [`ct-event-detail.md`](./ct-event-detail.md).

Author: a9s-architect, 2026-04-08
Driver: v1 (#245) was specced before #246 simplified the list view to
one-row-one-color. The v1 detail design assumed per-field/per-section color
(Root bright red, destructive verbs red-ish, colored badges, ERROR painted
red, ROOT bar). v2 applies the same simplification to the detail view —
**neutral body, single colored anchor on the Event row, structure carries
meaning**.

Spec: [`specs/013-ct-event-detail-v2/spec.md`](../../specs/013-ct-event-detail-v2/spec.md)

---

## 1. Visual rules

### 1.1 Section list and order

| Section | Always present? | Notes |
|---------|------------------|-------|
| **ACTOR**   | Always (except Insight) | Principal, As (when needed), MFA, Access key, User agent, Federation, Service |
| **ACTION**  | Always | `Event:` (always); `Category:` (only when ≠ Management/AwsApiCall); Insight metadata when applicable |
| **TARGET**  | When extraction yields ≥1 row | Per #246 §4 fallback algorithm |
| **CONTEXT** | Always | Region, Source IP, VPC endpoint, Recipient (cross-acct), Time |
| **ERROR**   | Only when `errorCode != ""` | Hoisted directly after CONTEXT |
| **REQUEST** | When non-empty after TARGET extraction removes lifted fields | Per-service summarizer or generic walk |
| **RESPONSE**| When `responseElements` non-empty | Generic walk |

The order is fixed: ACTOR → ACTION → TARGET → CONTEXT → ERROR → REQUEST → RESPONSE.
Empty sections are omitted entirely (header and all).

### 1.2 No color in body — except one anchor

The detail view body uses **only** `ColDetailKey` (label) and `ColDetailVal`
(value), with **one exception**: the **`Event:` row's value** in ACTION is
painted via `RowColorStyle(eventStatus)` using the same severity ladder as
the list view (#246 §1.1–§1.2):

| Status         | Color (palette)         | Trigger                                                                |
|----------------|-------------------------|------------------------------------------------------------------------|
| `ct-info`      | `ColTerminated` (dim)   | Read calls, no error, no flags                                          |
| `ct-attention` | `ColPending` (yellow)   | Verb is W, OR Root, OR cross-account, OR sensitive-reads allowlist     |
| `ct-danger`    | `ColStopped` (red)      | `errorCode != ""`, OR verb is D                                         |

**Why the exception**: the detail view can be opened from contexts other
than the list view (related-resource navigation, direct ID lookup). In
those flows the user has not seen the row's severity color. The Event row
is the natural single-anchor point.

**Why only one row**: any more re-introduces the per-cell rainbow that
#246 tore out. One colored cell in a 12-row body reads as accent, not
chatter.

Every other row in the body is neutral. The frame title is plain
(`╴ ct-events/<eventId> ╶`).

### 1.3 Section headers

Section headers MUST render as **bold uppercase** label on its own line,
flush-left at one space of left padding (matching the data row indent),
with **no color, no horizontal rule, no surrounding blank lines**. Format:

```text
 ACTOR
 Principal:    arn:aws:sts::111111111111:assumed-role/...
 ...
 ACTION
 Event:        ec2:DescribeInstances        ← only this value carries color
 ...
```

Implementation: a single new `IsSection bool` field on `fieldpath.FieldItem`.
The renderer's switch in `renderFromFieldList` adds one case before
`item.IsHeader`:

```go
case item.IsSection:
    line = " " + lipgloss.NewStyle().Bold(true).Render(item.Key)
```

The Event row carries an additional `Severity string` field (or equivalent)
that the renderer reads to apply `RowColorStyle(severity)` to the value
(only the value — not the `Event:` label). All other rows leave `Severity`
empty and render through neutral `ColDetailVal`.

No new style token is added to `internal/tui/styles/`. The bold-only style
is constructed inline; the severity coloring reuses the existing
`RowColorStyle` registry.

### 1.4 No RAW section, no inline raw JSON

The structured view ends after `RESPONSE`. Raw JSON is exclusively reachable
via the existing `y` (YAML view) keybinding, which is unchanged.

### 1.5 Bottom hint border

```text
y yaml   / search   tab cols   esc back
```

The v1 `R raw` hint is dropped — there is no inline raw section. `y` opens
the existing YAML view (the canonical raw-JSON access path).

---

## 2. Per-section content rules

### 2.1 ACTOR

Drop-boring-defaults applies to every row: omit when value is the default
or empty.

| Row             | Source                                         | Always shown? |
|-----------------|------------------------------------------------|---------------|
| `Principal`     | `userIdentity.arn` (navigable)                 | When ARN present |
| `Service`       | `eventSource`                                  | AwsServiceEvent only (no ARN) |
| `As`            | computed: `userName` / `sourceIdentity` / `sessionIssuer.userName` | When the principal ARN does NOT carry the human label (SSO opaque ARNs, SAML, federated) |
| `Federation`    | `webIdFederationData.federatedProvider`        | WebIdentityUser / IRSA only |
| `MFA`           | `sessionContext.attributes.mfaAuthenticated`   | Only when `true` (the `no` case is dropped) |
| `Access key`    | `userIdentity.accessKeyId`                     | When non-empty (navigable) |
| `User agent`    | `userAgent`                                    | When non-empty |

**Removed from prior drafts** (do not reintroduce):
- `Identity type` row — the principal ARN encodes the type (`:role/`, `:user/`, `:root`)
- `Account` row — the principal ARN encodes the 12-digit account inline
- `Actor` computed-summary row — redundant with Principal
- `Issuer role` / `Issuer ARN` separate rows — folded into the Principal ARN

### 2.2 ACTION

| Row             | Source                                          | Always shown? |
|-----------------|-------------------------------------------------|---------------|
| `Event`         | `<service>:<eventName>` where `<service>` = `eventSource` minus `.amazonaws.com` | Always |
| `Category`      | `<eventCategory> / <eventType>`                 | Only when ≠ `Management/AwsApiCall` |
| `Insight type`  | `insightDetails.insightType`                    | Insight events only |
| `State`         | `insightDetails.state`                          | Insight events only |

The `Event:` row's value is the **only colored cell** in the body — see §1.2.

**Removed from prior drafts**: `Source:` row (redundant with the IAM-action
form of `Event:`), `Verb:` row (encoded in eventName), `Read only:` row
(implied from action), `Sensitive:` row (the severity color on the Event row
encodes "elevated above ordinary R" already).

### 2.3 TARGET

Extracted via the **#246 §4 fallback algorithm**. Implementation reuses or
ports `internal/aws/ct_events.go::FormatCTTarget` and the per-event-name
fallback table. Order of precedence:

1. SDK envelope `resources[]` — if non-empty, one row per resource entry
2. Per-event-name lookup in `requestParameters` (the §4 table from #246):
   - `DescribeInstances` → joined `instanceId`s, or `(all)` for empty list
   - `UpdateInstanceInformation` → `instanceId`
   - `GetParameter` / `GetParameters` / `GetParametersByPath` → name / names / path
   - `GetSecretValue` → `secretId`
   - `Decrypt` → `keyId` or `(by alias)`
   - `AssumeRole*` → `roleArn` (then ARN-stripped)
   - `BatchGetImage` → `repositoryName`
   - `BatchGetItem` → joined `requestItems` table names
   - `ListBuckets` → `(none)`
   - S3 PutObject / GetObject / DeleteObject → bucket + object
3. Catch-all: scan top-level `requestParameters` for `*Id` / `*Name` / `*Arn` strings

Each row is rendered with:
- **Label**: resource type derived from the ARN service segment — `Bucket:`,
  `Object:`, `Instance:`, `Key:`, `Role:`, `User:`, `Secret:`, `Repository:`,
  `Function:`, `Cluster:`, `Table:`. Generic `Resource:` only when ambiguous.
- **Value**: ARN-stripped form (collapse `arn:aws:<svc>:<region>:<acct>:` to
  empty), navigable when the resource type is registered with a9s.

**TARGET-vs-REQUEST de-dup rule**: fields lifted into TARGET MUST be removed
from `requestParameters` before the REQUEST section is built. Otherwise
S3 PutObject events would show `bucketName` and `key` in both TARGET and
REQUEST — duplication. After removal, REQUEST is often empty for events
where TARGET captures everything (S3 reads/writes, Secrets Manager GET,
KMS Decrypt, etc.) and then REQUEST is omitted entirely per the empty-section
rule.

### 2.4 CONTEXT

| Row              | Source                                  | Always shown? |
|------------------|-----------------------------------------|---------------|
| `Region`         | `awsRegion`                             | Always (when present) |
| `Source IP`      | `sourceIPAddress` — literal `AWS Internal` text when applicable | Always (when present) |
| `VPC endpoint`   | `vpcEndpointId`                         | When present (NetworkActivity events; navigable to `vpce`) |
| `Recipient`      | `recipientAccountId`, suffixed with ` (cross-account)` | Only when `accountId != recipientAccountId` |
| `Time`           | `eventTime` in RFC3339 form `2026-04-07T14:02:11Z` | Always |

**Cross-account rule**: the principal ARN already encodes the caller account,
so the new `Recipient:` row surfaces the missing recipient half. This adapts
#246's prefix logic (which prefixed the actor cell with the counterparty
account in the constrained list-view cell) to the detail view's row format.

**No display-form timestamp**: forensics > scannability inside the detail
view. The list view already has the scannable `Apr 07 14:02:11` form.

### 2.5 ERROR (conditional)

Hoisted directly after CONTEXT, only when `errorCode != ""`. Two rows:

| Row            | Source         | Notes                                              |
|----------------|----------------|----------------------------------------------------|
| `errorCode`    | `errorCode`    | Plain text, no color (the Event row already carries the danger color) |
| `errorMessage` | `errorMessage` | Plain text, wraps at viewport width                |

### 2.6 REQUEST

Built by the per-service summarizer for `eventSource`. Phase 1 ships
summarizers for `s3.amazonaws.com`, `iam.amazonaws.com`, `ec2.amazonaws.com`.
Phase 2 adds STS, Lambda, RDS, EKS, ECS, Secrets Manager, SSM. Unrecognized
services use `SummarizeGeneric` (flat key/value walk).

Fields lifted into TARGET (per §2.3 de-dup rule) MUST be removed before the
summarizer runs. Empty REQUEST → section omitted.

### 2.7 RESPONSE

`SummarizeGeneric(eventName, responseElements)`. Omitted when empty.

### 2.8 Insight events

Insight events (`eventCategory == "Insight"`) lack `userIdentity` →
**ACTOR section omitted**. The insight metadata folds into ACTION
(`Insight type:` + `State:` rows). The REQUEST section carries the
baseline + insight metric pair plus attribution principals.

### 2.9 NetworkActivity events

NetworkActivity events use the same ACTOR/ACTION/TARGET/CONTEXT/ERROR/REQUEST
structure as AwsApiCall, plus `Category: NetworkActivity / AwsVpceEvent` in
ACTION and `VPC endpoint:` in CONTEXT.

---

## 3. Wireframes — all 9 cases

All wireframes below render the body with **no color except the marked
`Event:` value**. The card border + frame title + bottom hint border are
existing chrome. Card content width: 82 columns. Label column width: 14.

The annotation `[ct-info]` / `[ct-attention]` / `[ct-danger]` next to the
`Event:` row indicates which severity color the value renders in — it is
NOT part of the rendered output, only a wireframe marker for review.

### Case A — Karpenter `ec2:DescribeInstances` (read, success → ct-info dim)

```text
╴ ct-events/e-a1b2c3d4 ╶
╭────────────────────────────────────────────────────────────────────────────────╮
│ ACTOR                                                                          │
│ Principal:    arn:aws:sts::111111111111:assumed-role/KarpenterNodeRole/        │
│               karpenter-1759                                                   │
│ Access key:   ASIAY44QH8DCKARPEXMP                                             │
│ User agent:   aws-sdk-go-v2/1.30.3                                             │
│ ACTION                                                                         │
│ Event:        ec2:DescribeInstances                              [ct-info]     │
│ TARGET                                                                         │
│ Instances:    (all)                                                            │
│ CONTEXT                                                                        │
│ Region:       us-east-1                                                        │
│ Source IP:    10.0.14.221                                                      │
│ Time:         2026-04-07T14:02:11Z                                             │
│ REQUEST                                                                        │
│ filters:      [{Name: instance-state-name, Values: [running]}]                 │
│ maxResults:   1000                                                             │
╰─────────────────────────────────y yaml──/ search──tab cols──esc back──╯
```

### Case B — SSO Console `ec2:TerminateInstances` (D verb, MFA → ct-danger red)

`instancesSet` extracted into TARGET → REQUEST omitted entirely. SSO opaque
principal ARN → `As:` row required.

```text
╴ ct-events/e-b2c3d4e5 ╶
╭────────────────────────────────────────────────────────────────────────────────╮
│ ACTOR                                                                          │
│ Principal:    arn:aws:sts::222222222222:assumed-role/                          │
│               AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d/alice@corp           │
│ As:           alice@corp via AWSReservedSSO_AdminAccess                        │
│ MFA:          yes                                                              │
│ Access key:   ASIAZK7L9PQRSSOXEXMP                                             │
│ User agent:   Console (AWS Internal)                                           │
│ ACTION                                                                         │
│ Event:        ec2:TerminateInstances                            [ct-danger]    │
│ TARGET                                                                         │
│ Instance:     instance/i-0f1e2d3c4b5a69788                                     │
│ Instance:     instance/i-0f1e2d3c4b5a69789                                     │
│ CONTEXT                                                                        │
│ Region:       eu-west-1                                                        │
│ Source IP:    AWS Internal                                                     │
│ Time:         2026-04-07T14:07:42Z                                             │
│ RESPONSE                                                                       │
│ terminating:  [{i-0f1e2d3c4b5a69788: shutting-down ← running},                 │
│                {i-0f1e2d3c4b5a69789: shutting-down ← running}]                 │
╰─────────────────────────────────y yaml──/ search──tab cols──esc back──╯
```

### Case C — `s3:PutObject` AccessDenied (errorCode → ct-danger red, ERROR hoisted)

`bucketName` + `key` extracted into TARGET → REQUEST omitted. ERROR section
sits between CONTEXT and (omitted) REQUEST.

```text
╴ ct-events/e-c3d4e5f6 ╶
╭────────────────────────────────────────────────────────────────────────────────╮
│ ACTOR                                                                          │
│ Principal:    arn:aws:iam::333333333333:user/bob                               │
│ Access key:   AKIAIOSFODNN7BOB1XMP                                             │
│ User agent:   aws-cli/2.17.9 Python/3.12.4 Darwin/24.1.0                       │
│ ACTION                                                                         │
│ Event:        s3:PutObject                                      [ct-danger]    │
│ TARGET                                                                         │
│ Bucket:       prod-logs                                                        │
│ Object:       prod-logs/2026/04/07/app.log                                     │
│ CONTEXT                                                                        │
│ Region:       us-east-1                                                        │
│ Source IP:    198.51.100.42                                                    │
│ Time:         2026-04-07T14:11:03Z                                             │
│ ERROR                                                                          │
│ errorCode:    AccessDenied                                                     │
│ errorMessage: User: arn:aws:iam::333333333333:user/bob is not authorized to    │
│               perform: s3:PutObject on resource:                               │
│               arn:aws:s3:::prod-logs/2026/04/07/app.log because no identity-   │
│               based policy allows the s3:PutObject action                      │
╰─────────────────────────────────y yaml──/ search──tab cols──esc back──╯
```

### Case D — KMS `kms:RotateKey` (AwsServiceEvent → ct-attention yellow)

No `userIdentity` ARN → `Service:` row instead of `Principal:`. `Category:`
row appears because `AwsServiceEvent` ≠ default. `keyId` extracted into
TARGET, ARN-stripped to `key/<uuid>`.

```text
╴ ct-events/e-d4e5f6a7 ╶
╭────────────────────────────────────────────────────────────────────────────────╮
│ ACTOR                                                                          │
│ Service:      kms.amazonaws.com                                                │
│ ACTION                                                                         │
│ Event:        kms:RotateKey                                  [ct-attention]    │
│ Category:     Management / AwsServiceEvent                                     │
│ TARGET                                                                         │
│ Key:          key/2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b                         │
│ CONTEXT                                                                        │
│ Region:       us-east-1                                                        │
│ Source IP:    AWS Internal                                                     │
│ Time:         2026-04-07T02:00:07Z                                             │
│ REQUEST                                                                        │
│ rotationType: AUTOMATIC                                                        │
│ backingKey:   true                                                             │
╰─────────────────────────────────y yaml──/ search──tab cols──esc back──╯
```

### Case E — Root `s3:PutBucketPolicy` (Root + W → ct-attention yellow)

Root principal ARN (`:root` suffix) is the marker. No banner. `bucketName`
extracted into TARGET; the inline `policy` JSON stays in REQUEST.

```text
╴ ct-events/e-e5f6a7b8 ╶
╭────────────────────────────────────────────────────────────────────────────────╮
│ ACTOR                                                                          │
│ Principal:    arn:aws:iam::555555555555:root                                   │
│ User agent:   Console (Mozilla/5.0 ... Safari/605.1.15)                        │
│ ACTION                                                                         │
│ Event:        s3:PutBucketPolicy                             [ct-attention]    │
│ TARGET                                                                         │
│ Bucket:       prod-artifacts                                                   │
│ CONTEXT                                                                        │
│ Region:       us-east-1                                                        │
│ Source IP:    203.0.113.17                                                     │
│ Time:         2026-04-07T03:42:18Z                                             │
│ REQUEST                                                                        │
│ policy:       {"Version": "2012-10-17", "Statement": [...]}                    │
╰─────────────────────────────────y yaml──/ search──tab cols──esc back──╯
```

### Case F — IRSA `s3:GetObject` (WebIdentityUser, R → ct-info dim)

`Federation:` row distinguishes IRSA from a regular AssumedRole. VPC endpoint
moves into CONTEXT. `bucketName` + `key` extracted into TARGET → REQUEST
omitted.

```text
╴ ct-events/e-f6a7b8c9 ╶
╭────────────────────────────────────────────────────────────────────────────────╮
│ ACTOR                                                                          │
│ Principal:    arn:aws:sts::666666666666:assumed-role/eks-checkout-svc-sa/      │
│               1717156821993453824                                              │
│ Federation:   oidc.eks.eu-west-1.amazonaws.com/id/EXAMPLE0D8C                  │
│ User agent:   aws-sdk-go-v2/1.30.3                                             │
│ ACTION                                                                         │
│ Event:        s3:GetObject                                       [ct-info]     │
│ TARGET                                                                         │
│ Bucket:       checkout-config                                                  │
│ Object:       checkout-config/prod/config.json                                 │
│ CONTEXT                                                                        │
│ Region:       eu-west-1                                                        │
│ Source IP:    10.42.3.18                                                       │
│ VPC endpoint: vpce-0abc123def456                                               │
│ Time:         2026-04-07T14:20:21Z                                             │
╰─────────────────────────────────y yaml──/ search──tab cols──esc back──╯
```

### Case G — Cross-account `s3:PutObject` (W + cross-acct → ct-attention yellow)

Caller is in `888888888888` (visible inside the principal ARN). Recipient
`777777777777` (where the bucket lives) → `Recipient:` row in CONTEXT with
`(cross-account)` marker.

```text
╴ ct-events/e-a7b8c9d0 ╶
╭────────────────────────────────────────────────────────────────────────────────╮
│ ACTOR                                                                          │
│ Principal:    arn:aws:sts::888888888888:assumed-role/CiBuildRole/build-4821    │
│ Access key:   ASIAQF3M2N8KCIB1XMPL                                             │
│ User agent:   aws-cli/2.17.9                                                   │
│ ACTION                                                                         │
│ Event:        s3:PutObject                                   [ct-attention]    │
│ TARGET                                                                         │
│ Bucket:       shared-artifacts                                                 │
│ Object:       shared-artifacts/build-4821.tar.gz                               │
│ CONTEXT                                                                        │
│ Region:       us-east-2                                                        │
│ Source IP:    52.14.88.201                                                     │
│ Recipient:    777777777777 (cross-account)                                     │
│ Time:         2026-04-07T14:31:55Z                                             │
╰─────────────────────────────────y yaml──/ search──tab cols──esc back──╯
```

> **Fixture note**: the v1 wireframe had the principal ARN's account as
> `777777777777` while listing `Account: 888888888888 (caller)` —
> internally inconsistent. CloudTrail's `userIdentity.accountId` matches
> the assumed-role's home account. v2 corrects this: caller's role lives
> in `888888888888`, the bucket lives in the recipient `777777777777`.

### Case H — Insight `ApiCallRateInsight` (no ACTOR → starts at ACTION)

No `userIdentity` → no ACTOR section. INSIGHT metadata folds into ACTION.
The frame title is the standard `╴ ct-events/<eventId> ╶` form — the
`Category: Insight / AwsApiCall` row in ACTION carries the disambiguation.

```text
╴ ct-events/e-b8c9d0e1 ╶
╭────────────────────────────────────────────────────────────────────────────────╮
│ ACTION                                                                         │
│ Event:        ec2:RunInstances                                   [ct-info]     │
│ Category:     Insight / AwsApiCall                                             │
│ Insight type: ApiCallRateInsight                                               │
│ State:        Start                                                            │
│ CONTEXT                                                                        │
│ Region:       us-east-1                                                        │
│ Time:         2026-04-07T09:14:00Z                                             │
│ REQUEST                                                                        │
│ baseline:     0.24 calls/min  (7d window)                                      │
│ insight:      18.70 calls/min (during anomaly)                                 │
│ insight prin: arn:aws:sts::999999999999:assumed-role/DeployRole/ci-41          │
│ baseline prin:arn:aws:sts::999999999999:assumed-role/DeployRole/ci-*           │
╰─────────────────────────────────y yaml──/ search──tab cols──esc back──╯
```

### Case I — NetworkActivity VPCE deny (errorCode → ct-danger red)

`bucketName` + `key` extracted into TARGET → REQUEST omitted. `Category:
NetworkActivity / AwsVpceEvent` in ACTION. `VPC endpoint:` row in CONTEXT.

```text
╴ ct-events/e-c9d0e1f2 ╶
╭────────────────────────────────────────────────────────────────────────────────╮
│ ACTOR                                                                          │
│ Principal:    arn:aws:sts::111111111111:assumed-role/DataPipelineRole/dp-0719  │
│ User agent:   aws-sdk-java/2.25.11                                             │
│ ACTION                                                                         │
│ Event:        s3:PutObject                                      [ct-danger]    │
│ Category:     NetworkActivity / AwsVpceEvent                                   │
│ TARGET                                                                         │
│ Bucket:       prod-lake                                                        │
│ Object:       prod-lake/landing/2026/04/07/batch-0719.parquet                  │
│ CONTEXT                                                                        │
│ Region:       eu-central-1                                                     │
│ Source IP:    10.12.4.77                                                       │
│ VPC endpoint: vpce-0ff11223344556677                                           │
│ Time:         2026-04-07T14:44:17Z                                             │
│ ERROR                                                                          │
│ errorCode:    VpceAccessDenied                                                 │
│ errorMessage: The VPC endpoint policy denies the s3:PutObject action on        │
│               arn:aws:s3:::prod-lake/landing/2026/04/07/batch-0719.parquet     │
╰─────────────────────────────────y yaml──/ search──tab cols──esc back──╯
```

---

## 4. Composite layouts (left card + RELATED right column)

The right column is **chrome**, not body, and is rendered exactly as today
(border + centered `RELATED` header + count rows + selection highlight).
Three composite cases mirror v1 §4b — they re-use Cases B / C / E above
without modification, joined horizontally via `renderEventWithRight`.

The right-column count rows continue to use the existing dim/highlight
styles. The left card body's only colored cell remains the `Event:` value.

---

## 5. Severity color matrix

| Case | Event | Conditions | Tier | Color |
|------|-------|------------|------|-------|
| A | `ec2:DescribeInstances` | R, no flags | `ct-info` | dim |
| B | `ec2:TerminateInstances` | D | `ct-danger` | red |
| C | `s3:PutObject` | errorCode | `ct-danger` | red |
| D | `kms:RotateKey` | W (AwsServiceEvent) | `ct-attention` | yellow |
| E | `s3:PutBucketPolicy` | W + Root | `ct-attention` | yellow |
| F | `s3:GetObject` | R, no flags | `ct-info` | dim |
| G | `s3:PutObject` | W + cross-account | `ct-attention` | yellow |
| H | `ec2:RunInstances` | I (Insight) | `ct-info` | dim |
| I | `s3:PutObject` | errorCode (VpceAccessDenied) | `ct-danger` | red |

3 dim / 3 yellow / 3 red — covers the full ladder for the preview.

---

## 6. Body row count comparison

| Case | v2.0 (this redesign's prior draft) | v2.1 (current) | Saving |
|------|-----------------------------------|----------------|--------|
| A | 24 | 14 | -42% |
| B | 31 | 18 | -42% |
| C | 26 | 15 | -42% |
| D | 21 | 13 | -38% |
| E | 21 | 13 | -38% |
| F | 22 | 14 | -36% |
| G | 26 | 14 | -46% |
| H | 18 | 13 | -28% |
| I | 22 | 16 | -27% |
| **Total** | **211** | **130** | **-38%** |

---

## 7. Mapping to spec requirements

| Spec FR  | Where it lives in this doc |
|----------|----------------------------|
| FR-001 (no per-cell color)               | §1.2 — except the single Event-row exception |
| FR-002 (severity color exception)        | §1.2 — Event row only, ladder from #246      |
| FR-003 (no colored badges)               | §3 wireframes — no badges anywhere           |
| FR-004 (section list + order)            | §1.1                                          |
| FR-005 (ERROR hoist)                     | §1.1; §3 Cases C and I show the hoist        |
| FR-006 (header style)                    | §1.3                                          |
| FR-007 (parse + dispatch)                | §2.8 (Insight), §2.9 (NetworkActivity)       |
| FR-008 (ACTOR rules)                     | §2.1                                          |
| FR-009 (MFA when true only)              | §2.1                                          |
| FR-010 (ACTION rules)                    | §2.2                                          |
| FR-011 (CONTEXT rules)                   | §2.4                                          |
| FR-012 (TARGET extraction)               | §2.3                                          |
| FR-013 (REQUEST per-service summarizers) | §2.6                                          |
| FR-014 (RESPONSE generic)                | §2.7                                          |
| FR-015 (per-row navigation)              | §2.1, §2.3 (navigable rows)                  |
| FR-016 (right column unchanged)          | §4                                            |
| FR-017 (YAML view untouched)             | §1.4; §1.5                                    |
| FR-018 (DetailModel reuse)               | §1.3 implementation note                      |
| FR-019 (no new style tokens)             | §1.3 — bold inline + reuse RowColorStyle     |
| FR-020 (this doc + v1 superseded)        | this file + ct-event-detail.md banner        |
| FR-021 (preview update)                  | §3 wireframes are the source of truth        |

---

## 8. Implementation notes for the coder

Transcription rules for `cmd/preview/ct_event/main.go`:

1. **Section list**: replace WHO/WHAT/WHERE/WHEN with ACTOR/ACTION/TARGET/CONTEXT. Add ACTION as a new section between ACTOR and TARGET.
2. **Event row coloring**: the `Event:` row's value renders with a severity-tier color. The preview may hard-code each case's tier (Case A → dim, Case B → red, etc.) using the existing palette tokens (`colDim`, `colWarning`, `colError`), or it may compute via a small helper that mirrors `RowColorStyle`. Either is acceptable for the preview — what matters is that the value shows the right color per §5.
3. **Frame title**: `╴ ct-events/<eventId> ╶` format (back to v1 form).
4. **No badges, no ROOT bar, no colored verb glyph, no colored section headers, no `OK`/`FAILED` outcome line, no inline RAW**.
5. **All 9 case fixtures** must be re-authored to match §3 wireframes verbatim.
6. **TARGET-vs-REQUEST de-dup**: when transcribing fixtures, lift the appropriate fields out of REQUEST into TARGET per §2.3, and omit REQUEST when nothing remains.
7. **Hint border**: drop `R raw`. New hint set: `y yaml`, `/ search`, `tab cols`, `esc back`.
8. **Composite layouts**: the 3 §4b cases re-use Cases B/C/E exactly; right column rendering unchanged.
