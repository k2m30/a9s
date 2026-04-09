# CloudTrail Event Detail View — Design (v1, HISTORICAL)

> **Status: superseded by [`ct-event-detail-v2.md`](./ct-event-detail-v2.md)**
> as of 2026-04-08. This document is preserved as a historical artifact —
> it captures the per-field/per-section color approach that was rejected
> after #246 simplified the list view to one-row-one-color. The v2 doc
> drops all per-field color, the ROOT bar, the colored badges, and the
> RAW section. Read v2 for the current truth; read v1 only for historical
> context on what was tried and why it was changed.

Status: design spec, not implemented. No code under `internal/`, `cmd/a9s/`,
`tests/`, or `.a9s/` changes as part of this document. Source of truth for
event shapes is `docs/design/ct-taxonomy.md`; every field path cited below
points back to a numbered section there.

---

## 1. Engine analysis

### 1.1 What the current config engine can express

`internal/config` defines, per resource short name, a `ViewDef` with:

```go
type ViewDef struct {
    List   []ListColumn // table columns, each sourced via fieldpath from RawStruct or @id/@name
    Detail []string     // ordered list of dotted field paths into RawStruct
}
```

`internal/tui/views/detail.go` consumes that by walking `Detail[]`,
extracting each path through `internal/fieldpath`, and rendering a flat
`label: value` list into a viewport. `refreshViewportContent` builds a
`[]fieldpath.FieldItem` (key, value, navigable flag, target type) which
is also the basis for cursor navigation, copy, and `enter`-to-navigate.

Everything it supports is therefore:

1. **Flat key/value list.** No grouped sections, no inline dividers, no
   computed headers. `Detail: []string` has no room for a section marker.
2. **Struct reflection only.** `fieldpath` extracts from `RawStruct`
   (here, `cloudtrailtypes.Event`). The rich payload a CT event actually
   carries lives **inside `Event.CloudTrailEvent`, which is a single
   `*string` holding a ~100 KB JSON blob**. `fieldpath` cannot reach into
   that string. The current `.a9s/views/ct-events.yaml` proves this: it
   can show `EventId`, `EventName`, `EventTime`, `EventSource`,
   `Username`, `ReadOnly`, `AccessKeyId`, `Resources`, and the raw
   `CloudTrailEvent` string — nothing from inside the JSON.
3. **One label per value.** No conditional fields ("show
   `sessionIssuer.userName` only when type == AssumedRole"), no computed
   actor strings, no per-service requestParameters summarizer.
4. **No color semantics per field.** The renderer uses a single
   `ColDetailKey`/`ColDetailVal` pair for all rows. There is no way to
   mark "this is an error, paint it red" or "this is Root, paint it
   bright red".

### 1.2 What a CT event detail view needs

From taxonomy §1–§6:

- Parse `CloudTrailEvent` JSON before rendering. Nothing on the SDK
  envelope besides `EventId` is authoritative.
- Dispatch by `eventCategory` × `eventType` — a `Management/AwsApiCall`
  event, an `Insight` event, and a `NetworkActivity` event have
  different record shapes (taxonomy §1.3, §1.4, §3).
- Render an **actor string** computed from `userIdentity.type` plus
  whichever of 12 variants it is (§4.1–§4.12), including SSO detection
  (`AWSReservedSSO_*` inside `sessionContext.sessionIssuer.arn`),
  `invokedBy` service attribution, and MFA flag.
- Surface **errors as a prominent section** when `errorCode` is set,
  not as row #14 in a flat list (§5.1).
- **Per-service request summarizer** for the top 10 services in §6,
  plus a generic JSON pretty-printer fallback.
- Section headers (WHO / WHAT / WHERE / WHEN / REQUEST / RESPONSE / RAW)
  with color, and section-aware collapse of the RAW blob.
- Special-case coloring: Root is bright red, destructive verbs
  (Delete/Terminate/Put/Revoke) are red-ish, read verbs are dim (§5.6),
  cross-account (recipient ≠ userIdentity.accountId) gets a badge (§5.3),
  `AWS Internal` source IP gets a badge (§5.8).

None of these can be expressed in `Detail: []string`. Extending the
schema to cover them would effectively require: computed fields,
conditional sections, per-service transforms, color metadata, and a
second-level JSON field extractor. At that point the YAML schema
becomes a second (worse) programming language.

### 1.3 Options considered

| Option | Fit | Cost | Risk |
|---|---|---|---|
| **Pure config.** Extend `ViewDef` with sections, conditions, JSON-pointer paths, color hints, per-service transforms. | Possible in principle. | Very high — schema changes, new JSON backend in `fieldpath`, conditional DSL, renderer rewrite. Affects every other resource type through shared config types. | Schema becomes unreadable; we reimplement Go in YAML for one resource. |
| **Hybrid: keep `fieldpath` generic for envelope fields, add a CT-specific renderer branch in `detail.go` keyed by `resourceType == "ct-events"`.** The branch parses `CloudTrailEvent` JSON once, produces a sectioned model, renders through Lipgloss with the existing viewport/search/copy scaffolding. | Natural. `ViewDef` stays a flat list for the 40+ other resources. | Medium — one new file (e.g. `internal/tui/views/detail_ct.go`), one JSON parser, test coverage. | Couples one view to one resource type — which is exactly what CT events warrant. |
| **Custom view.** Dedicated `CTEventDetailModel` replacing `DetailModel` for ct-events. | Cleanest separation. | Higher — must re-implement search, right column, copy, navigation, bottom hints, resize, related panel, field cursor. All already work in `DetailModel`. | Behavior drift vs. other detail views. |

### 1.4 Decision — Hybrid

Keep `DetailModel` as the entry point. When
`m.resourceType == "ct-events"`, `refreshViewportContent` delegates to a
new `renderCTEvent(res)` helper that:

1. Parses `res.RawStruct.(cloudtrailtypes.Event).CloudTrailEvent` once
   into a typed Go struct (`ctRecord` — defined in the implementation
   package, not here).
2. Builds a `[]ctSection`, each with a colored header and a
   `[]fieldpath.FieldItem` body. The existing `fieldList` slice and
   `fieldCursor` mechanism are reused: section headers are inserted as
   non-selectable items, data rows use the same `FieldItem` shape so
   copy / search / navigate-on-enter keep working unchanged.
3. Feeds the rendered string into the existing viewport. Search,
   wrap-toggle, clipboard, escape / back all work without modification.
4. The right-column related panel is populated by a CT-specific
   `RelatedDef` set registered via `RegisterRelated("ct-events", …)`.
   Each checker reads from the same parsed `ctRecord` cached on the
   detail model so the JSON parse runs once per detail open. The
   existing `app_related.go` dispatch and `rightColumnModel` rendering
   are unchanged. See §7b.1 and §7b.10.

The ct-events YAML (`.a9s/views/ct-events.yaml`) remains the source of
truth for the **list view** columns. Its `detail:` block becomes
advisory / unused for the detail view (documented in the file). This is
the smallest change that respects what the taxonomy actually demands.

Uncertainty: this assumes `fieldpath.FieldItem` already carries enough
to represent a section header row without the cursor logic selecting
it. If not, the implementation will need an `IsSectionHeader` bool on
`FieldItem` — a one-line addition, scoped to `fieldpath`. Flagged as an
open question in §8.

---

## 2. Information architecture

Canonical model: **WHO did WHAT to WHOM, WHEN, WHERE, with what OUTCOME**.
Every wireframe in §4 maps onto this.

### 2.1 Actor rendering (WHO)

One computed string per event, derived strictly from `userIdentity`
(taxonomy §4). Fallback chain per type:

| `userIdentity.type` | Primary render | Badges |
|---|---|---|
| `Root` (§4.1) | `ROOT (123456789012)` or `ROOT (<alias>)` | bright-red, `[ROOT]` |
| `IAMUser` (§4.2) | `<userName>` | `[LONG-LIVED-KEY]` when `accessKeyId != ""` and no `sessionContext` |
| `AssumedRole` non-SSO (§4.3) | `<sessionIssuer.userName> → <sessionName>` | `[MFA]` when `sessionContext.attributes.mfaAuthenticated == "true"`; `[IMDSv2]` when `ec2RoleDelivery == "2.0"`; `[IRSA]` when `webIdFederationData.federatedProvider` set |
| `AssumedRole` SSO (§4.3) | `sso:<sessionName> (via <PermissionSet>)` parsed from `AWSReservedSSO_<PermissionSet>_<hash>` | `[MFA]` |
| `AssumedRole` service-linked (§5.4 pattern 2) | `<sessionIssuer.userName> (via <invokedBy>)` | `[SERVICE]` |
| `FederatedUser` (§4.4) | `federated:<sessionIssuer.userName>/<principalId suffix>` | — |
| `AWSService` (§4.5) | `<invokedBy>` e.g. `ec2.amazonaws.com` | `[SERVICE]` |
| `AWSAccount` (§4.6) | `account:<accountId>` | `[X-ACCT]` |
| `SAMLUser` (§4.7) | `saml:<userName>@<identityProvider>` | — |
| `WebIdentityUser` (§4.8) | `webid:<userName>@<identityProvider>` | — |
| `IdentityCenterUser` (§4.9) | `idc:<onBehalfOf.userId>` | — |
| `Directory` (§4.10) | `directory:<userName or accountId>` | — |
| `Unknown` (§4.11) | `unknown` | dim |

All badges are derived, never from taxonomy-absent fields.

### 2.2 Verb classification (WHAT)

Applied to `eventName` per §5.6:

- **Read** (dim): `Describe* List* Get* Head* BatchGet* Query* Scan*
  Lookup* Search* Select* View*`.
- **Write destructive** (red): `Delete* Terminate* Remove* Revoke*
  Detach* Stop* Reset* Cancel*`.
- **Write mutating** (orange): `Create* Put* Post* Update* Modify*
  Attach* Start* Run* Reboot* Restore* Copy* Replace* Set* Enable*
  Disable* Authorize* Tag* Untag* Submit* Send* Publish* Issue* Renew*
  Rotate*`.
- **Ambiguous** (default): `Assume* Decrypt Encrypt Sign Verify
  GenerateDataKey`.
- **Service event** (blue): any `eventType == AwsServiceEvent`.
- **Insight** (purple): any `eventType == AwsCloudTrailInsight`.

### 2.3 Outcome

If `errorCode` is present → `FAILED (<errorCode>)` in red, and
`errorMessage` is shown verbatim in the RESPONSE section. Otherwise →
`OK` in subtle green.

### 2.4 Target (WHOM)

Resolved in this order:

1. `resources[]` from the CloudTrail record when non-empty. Render each
   as `type: ARN`.
2. Per-service fallback paths from §6.
3. `requestParameters.<serviceIdField>` for the top 10 services.
4. `"(no resource)"` dim placeholder when nothing resolves.

---

## 3. Layout sections

Order in the viewport, top to bottom. Each section header is one line,
accent-blue, bold, followed by rows. Empty sections collapse entirely
(no header rendered).

### 3.1 HEADER (one line, not a section)

```
<VERB> <eventName>   <actor>   →   <target>            <outcome>  <time>  <region>
```

`VERB` glyph: `R` read, `W` write, `D` destructive, `S` service, `I` insight.

### 3.2 WHO

| Label | Source |
|---|---|
| Actor | computed §2.1 |
| Account | `userIdentity.accountId` |
| Principal ARN | `userIdentity.arn` (or `sessionContext.sessionIssuer.arn` for AssumedRole) |
| Session | `principalId` suffix for AssumedRole; `sessionContext.attributes.creationDate` + computed age |
| MFA | `sessionContext.attributes.mfaAuthenticated` → `yes`/`no`; `additionalEventData.MFAUsed` on ConsoleLogin |
| Access key | `userIdentity.accessKeyId` (dim when empty) |
| Source identity | `sessionContext.sourceIdentity` when set |
| Web federation | `sessionContext.webIdFederationData.federatedProvider` when set |
| User agent | parsed via §5.5 to `Console / CLI v2 / Go SDK / Boto3 / Terraform / Service: <name> / Browser / <raw prefix>` |

### 3.3 WHAT

| Label | Source |
|---|---|
| Event name | `eventName` |
| Event source | `eventSource` |
| Category | `eventCategory` |
| Type | `eventType` |
| Read only | `readOnly` (falls back to §5.6 heuristic with `(heuristic)` suffix when absent) |
| Resources | `resources[]` with §6 fallback |
| API version | `apiVersion` when present |
| Management | `managementEvent` when true |

### 3.4 WHERE

| Label | Source |
|---|---|
| Region | `awsRegion` |
| Source IP | `sourceIPAddress`; `[AWS-INTERNAL]` badge if `"AWS Internal"`; `[SERVICE]` if `<service>.amazonaws.com` |
| TLS | `tlsDetails.tlsVersion` + `cipherSuite` when present |
| Host header | `tlsDetails.clientProvidedHostHeader` when present |
| VPC endpoint | `vpcEndpointId` + `vpcEndpointAccountId` when present; `[VPCE]` badge |
| Edge device | `edgeDeviceDetails` when present, summarized |
| Recipient account | `recipientAccountId` when `!= userIdentity.accountId`; `[X-ACCT]` badge + `sharedEventID` |

### 3.5 WHEN

| Label | Source |
|---|---|
| Event time | `eventTime` |
| Session started | `sessionContext.attributes.creationDate` |
| Session age | computed |

### 3.6 REQUEST

- `AwsApiCall`: per-service summarizer for the top 10 services (§6),
  fallback to generic pretty-printed JSON collapsed to depth 2.
- `AwsServiceEvent`: render `serviceEventDetails`. `requestParameters`
  is not expected here (§3).
- `AwsConsoleSignIn`: render `additionalEventData` (`MFAUsed`,
  `MFAIdentifier`, `LoginTo`, `MobileVersion`) instead of
  `requestParameters` (typically `null`).
- `AwsCloudTrailInsight`: render `insightDetails.insightType`, `state`,
  `eventSource`, `eventName`, a compact view of
  `insightContext.statistics` (baseline vs. insight) and
  `insightContext.attributions[]`.
- `AwsVpceEvent`: render the underlying API's `requestParameters` plus
  `vpcEndpointId` prominently in WHERE.

### 3.7 RESPONSE / ERROR

- Error path (`errorCode != ""`): red header `ERROR <errorCode>` then
  `errorMessage` wrapped. No `responseElements` (§5.1).
- Success path: top identifier-ish fields from `responseElements` via
  service-specific keys; everything else collapsed.

### 3.8 RAW

Collapsed by default. Press `R` (see §8, q1) to expand; full
`CloudTrailEvent` JSON pretty-printed with YAML-style coloring. Press
`y` to copy the raw JSON. Oversized blobs truncated at 60 KB for
display with `... [truncated, press y to copy full]`.

---

## 4. ASCII wireframes

All values synthetic. Account IDs `111111111111`–`999999999999`.
Rendered at ~96 columns.

**Navigation marker**: navigable rows have their **value** rendered in
`ColAccent` + underlined — exactly the same convention used by every
other a9s detail view (`styles.NavigableField` in
`internal/tui/styles/styles.go:195`). No trailing glyph, no arrow. The
right column lists related resources grouped by type per §7b. Pivot rows
(filter ct-events by a field) use the same underlined-accent value
style — see §7b.5 and §8 q12 for which pivots actually work in v1.

### 4.1 Case A — AssumedRole service role, read, success (Karpenter)

```
╭─ ct-events/e-a1b2c3d4 ─────────────────────────────────────────────────────────╮
│ R  DescribeInstances   KarpenterNodeRole → karpenter-1759 [SERVICE]            │
│    → (no resource)                                OK   14:02:11Z  us-east-1    │
│                                                                                │
│ WHO                                                                            │
│   Actor         KarpenterNodeRole → karpenter-1759       [SERVICE] [IMDSv2]    │
│   Account       111111111111                                                   │
│   Principal     arn:aws:iam::111111111111:role/KarpenterNodeRole               │
│   Issuer role   KarpenterNodeRole                                              │
│   Session       karpenter-1759  (started 13:44:02Z, 18m ago)                   │
│   MFA           no                                                             │
│   Access key    ASIAY44QH8DCKARPEXMP                                           │
│   User agent    Go SDK v2  (aws-sdk-go-v2/1.30.3)                              │
│   — 47 more events from this principal                                         │
│                                                                                │
│ WHAT                                                                           │
│   Event         DescribeInstances                                              │
│   Source        ec2.amazonaws.com                                              │
│   Category      Management      Type   AwsApiCall                              │
│   Read only     true                                                           │
│   Resources     (no resource)                                                  │
│                                                                                │
│ WHERE                                                                          │
│   Region        us-east-1                                                      │
│   Source IP     10.0.14.221                                                    │
│   TLS           TLSv1.3  TLS_AES_128_GCM_SHA256                                │
│                                                                                │
│ WHEN                                                                           │
│   Event time    2026-04-07T14:02:11Z                                           │
│   Session age   00:18:09                                                       │
│                                                                                │
│ REQUEST                                                                        │
│   filters       [ { Name: "instance-state-name", Values: ["running"] } ]       │
│   maxResults    1000                                                           │
│                                                                                │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

### 4.2 Case B — IdentityCenter/SSO AssumedRole console write, MFA, success

```
╭─ ct-events/e-b2c3d4e5 ─────────────────────────────────────────────────────────╮
│ D  TerminateInstances  sso:alice@corp (via AdminAccess) [CONSOLE] [MFA]        │
│    → AWS::EC2::Instance i-0f1e2d3c4b5a69788    OK   14:07:42Z  eu-west-1       │
│                                                                                │
│ WHO                                                                            │
│   Actor         sso:alice@corp (via AdminAccess)        [CONSOLE] [MFA]        │
│   Account       222222222222                                                   │
│   Principal     arn:aws:iam::222222222222:role/aws-reserved/sso.amazonaws.com/ │
│                 AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d                    │
│   Session       alice@corp  (started 13:58:00Z, 9m ago)                        │
│   MFA           yes                                                            │
│   Access key    ASIAZK7L9PQRSSOXEXMP                                           │
│   Source ident  alice@corp                                                     │
│   User agent    Console  (AWS Internal)                                        │
│                                                                                │
│ WHAT                                                                           │
│   Event         TerminateInstances                                             │
│   Source        ec2.amazonaws.com                                              │
│   Category      Management      Type   AwsApiCall                              │
│   Read only     false                                                          │
│   Resources                                                                    │
│     AWS::EC2::Instance  arn:aws:ec2:eu-west-1:222222222222:instance/           │
│                         i-0f1e2d3c4b5a69788                                    │
│                                                                                │
│ WHERE                                                                          │
│   Region        eu-west-1                                                      │
│   Source IP     AWS Internal                                    [AWS-INTERNAL] │
│   TLS           TLSv1.3  TLS_AES_128_GCM_SHA256                                │
│                                                                                │
│ WHEN                                                                           │
│   Event time    2026-04-07T14:07:42Z                                           │
│   Session age   00:09:42                                                       │
│                                                                                │
│ REQUEST                                                                        │
│   instancesSet                                                                 │
│     [0] i-0f1e2d3c4b5a69788                                                    │
│     [1] i-0f1e2d3c4b5a69789                                                    │
│                                                                                │
│ RESPONSE                                                                       │
│   terminatingInstances  [ i-0f1e2d3c4b5a69788: shutting-down ← running ]       │
│                                                                                │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

### 4.3 Case C — IAMUser, long-lived key, AccessDenied on S3 PutObject

```
╭─ ct-events/e-c3d4e5f6 ─────────────────────────────────────────────────────────╮
│ W  PutObject          bob                              FAILED (AccessDenied)   │
│    → AWS::S3::Object  arn:aws:s3:::prod-logs/2026/04/07/app.log   14:11:03Z    │
│    us-east-1                                                                   │
│                                                                                │
│ WHO                                                                            │
│   Actor         bob                                      [LONG-LIVED-KEY]     │
│   Account       333333333333                                                   │
│   Principal     arn:aws:iam::333333333333:user/bob                             │
│   MFA           no                                                             │
│   Access key    AKIAIOSFODNN7BOB1XMP                                           │
│   User agent    AWS CLI v2  (aws-cli/2.17.9 Python/3.12.4 Darwin/24.1.0)       │
│                                                                                │
│ WHAT                                                                           │
│   Event         PutObject                                                      │
│   Source        s3.amazonaws.com                                               │
│   Category      Data            Type   AwsApiCall                              │
│   Read only     false                                                          │
│   Resources                                                                    │
│     AWS::S3::Bucket  arn:aws:s3:::prod-logs                                    │
│     AWS::S3::Object  arn:aws:s3:::prod-logs/2026/04/07/app.log                 │
│                                                                                │
│ WHERE                                                                          │
│   Region        us-east-1                                                      │
│   Source IP     198.51.100.42                                                  │
│   TLS           TLSv1.3                                                        │
│                                                                                │
│ WHEN                                                                           │
│   Event time    2026-04-07T14:11:03Z                                           │
│                                                                                │
│ REQUEST                                                                        │
│   bucketName    prod-logs                                                      │
│   key           2026/04/07/app.log                                             │
│                                                                                │
│ ERROR                                                                          │
│   AccessDenied                                                                 │
│   User: arn:aws:iam::333333333333:user/bob is not authorized to perform:       │
│   s3:PutObject on resource: arn:aws:s3:::prod-logs/2026/04/07/app.log          │
│   because no identity-based policy allows the s3:PutObject action              │
│                                                                                │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

### 4.4 Case D — AWSService (AwsServiceEvent, KMS key rotation)

```
╭─ ct-events/e-d4e5f6a7 ─────────────────────────────────────────────────────────╮
│ S  RotateKey          kms.amazonaws.com           [SERVICE]                    │
│    → AWS::KMS::Key  arn:aws:kms:us-east-1:444444444444:key/...  OK             │
│    02:00:07Z  us-east-1                                                        │
│                                                                                │
│ WHO                                                                            │
│   Actor         kms.amazonaws.com                           [SERVICE]          │
│   Account       444444444444                                                   │
│   Invoked by    kms.amazonaws.com                                              │
│                                                                                │
│ WHAT                                                                           │
│   Event         RotateKey                                                      │
│   Source        kms.amazonaws.com                                              │
│   Category      Management      Type   AwsServiceEvent                         │
│   Resources                                                                    │
│     AWS::KMS::Key  arn:aws:kms:us-east-1:444444444444:key/                     │
│                    2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b                        │
│                                                                                │
│ WHERE                                                                          │
│   Region        us-east-1                                                      │
│   Source IP     AWS Internal                               [AWS-INTERNAL]      │
│                                                                                │
│ WHEN                                                                           │
│   Event time    2026-04-07T02:00:07Z                                           │
│                                                                                │
│ SERVICE EVENT DETAILS                                                          │
│   keyId                  2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b                  │
│   rotationType           AUTOMATIC                                             │
│   backingKeyGenerated    true                                                  │
│                                                                                │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

### 4.5 Case E — Root user action (red banner)

```
╭─ ct-events/e-e5f6a7b8 ─────────────────────────────────────────────────────────╮
│ ██████████████████████████████████████████████████████████████████████████████ │
│ █ ROOT USER ACTION — account 555555555555                                    █ │
│ █ W  PutBucketPolicy  prod-artifacts                           OK            █ │
│ ██████████████████████████████████████████████████████████████████████████████ │
│                                                                                │
│ WHO                                                                            │
│   Actor         ROOT (account 555555555555)                    [ROOT]          │
│   Account       555555555555                                                   │
│   Principal     arn:aws:iam::555555555555:root                                 │
│   MFA           no                                                             │
│   Access key    (signed with root credentials)                                 │
│   User agent    Console  (Mozilla/5.0 ... Safari/605.1.15)                     │
│                                                                                │
│ WHAT                                                                           │
│   Event         PutBucketPolicy                                                │
│   Source        s3.amazonaws.com                                               │
│   Category      Management      Type   AwsApiCall                              │
│   Read only     false                                                          │
│   Resources                                                                    │
│     AWS::S3::Bucket  arn:aws:s3:::prod-artifacts                               │
│                                                                                │
│ WHERE                                                                          │
│   Region        us-east-1                                                      │
│   Source IP     203.0.113.17                                                   │
│                                                                                │
│ WHEN                                                                           │
│   Event time    2026-04-07T03:42:18Z                                           │
│                                                                                │
│ REQUEST                                                                        │
│   bucketName    prod-artifacts                                                 │
│   policy        { "Version": "2012-10-17", "Statement": [ ... ] }              │
│                                                                                │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

### 4.6 Case F — WebIdentityUser / IRSA (EKS pod)

```
╭─ ct-events/e-f6a7b8c9 ─────────────────────────────────────────────────────────╮
│ R  GetObject          checkout-svc-sa → 1717156821... [SERVICE] [IRSA]         │
│    → AWS::S3::Object  arn:aws:s3:::checkout-config/prod/config.json   OK       │
│    14:20:21Z  eu-west-1                                                        │
│                                                                                │
│ WHO                                                                            │
│   Actor         checkout-svc-sa → 1717156821...    [SERVICE] [IRSA]            │
│   Account       666666666666                                                   │
│   Principal     arn:aws:iam::666666666666:role/eks-checkout-svc-sa             │
│   Session       1717156821993453824                                            │
│   MFA           no                                                             │
│   Web federation  arn:aws:iam::666666666666:oidc-provider/                     │
│                   oidc.eks.eu-west-1.amazonaws.com/id/EXAMPLE0D8C              │
│                   (not navigable — OIDC providers not listed in a9s, §8 q11)   │
│   User agent    aws-sdk-go-v2/1.30.3                                           │
│                                                                                │
│ WHAT                                                                           │
│   Event         GetObject                                                      │
│   Source        s3.amazonaws.com                                               │
│   Category      Data            Type   AwsApiCall                              │
│   Read only     true                                                           │
│   Resources                                                                    │
│     AWS::S3::Object  arn:aws:s3:::checkout-config/prod/config.json             │
│                                                                                │
│ WHERE                                                                          │
│   Region        eu-west-1                                                      │
│   Source IP     10.42.3.18                                                     │
│   VPC endpoint  vpce-0abc123def456 (acct 666666666666)      [VPCE]             │
│                                                                                │
│ WHEN                                                                           │
│   Event time    2026-04-07T14:20:21Z                                           │
│                                                                                │
│ REQUEST                                                                        │
│   bucketName    checkout-config                                                │
│   key           prod/config.json                                               │
│                                                                                │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

### 4.7 Case G — Cross-account (recipientAccountId ≠ userIdentity.accountId)

```
╭─ ct-events/e-a7b8c9d0 ─────────────────────────────────────────────────────────╮
│ W  PutObject  CiBuildRole → build-4821 (from 888888888888)  [X-ACCT]           │
│    → AWS::S3::Object  arn:aws:s3:::shared-artifacts/build-4821.tar.gz   OK     │
│    14:31:55Z  us-east-2                                                        │
│                                                                                │
│ WHO                                                                            │
│   Actor         CiBuildRole → build-4821                    [X-ACCT]           │
│   Account       888888888888  (caller)                                         │
│   Recipient     777777777777                              (decorative, §8 q10) │
│   Principal     arn:aws:iam::777777777777:role/CiBuildRole   (cross-acct, dim) │
│   Session       build-4821  (started 14:28:10Z, 3m ago)                        │
│   MFA           no                                                             │
│   Access key    ASIAQF3M2N8KCIB1XMPL                                           │
│   User agent    aws-cli/2.17.9                                                 │
│                                                                                │
│ WHAT                                                                           │
│   Event         PutObject                                                      │
│   Source        s3.amazonaws.com                                               │
│   Category      Data            Type   AwsApiCall                              │
│   Resources                                                                    │
│     AWS::S3::Bucket  arn:aws:s3:::shared-artifacts                             │
│     AWS::S3::Object  arn:aws:s3:::shared-artifacts/build-4821.tar.gz           │
│                                                                                │
│ WHERE                                                                          │
│   Region        us-east-2                                                      │
│   Source IP     52.14.88.201                                                   │
│   Recipient     777777777777                              [X-ACCT]             │
│   Shared event  f1e2d3c4-b5a6-7890-1234-567890abcdef                           │
│                                                                                │
│ WHEN                                                                           │
│   Event time    2026-04-07T14:31:55Z                                           │
│                                                                                │
│ REQUEST                                                                        │
│   bucketName    shared-artifacts                                               │
│   key           build-4821.tar.gz                                              │
│                                                                                │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

### 4.8 Case H — Insight event (AwsCloudTrailInsight)

```
╭─ ct-events/e-b8c9d0e1 ─────────────────────────────────────────────────────────╮
│ I  RunInstances   INSIGHT  ApiCallRateInsight  Start                           │
│    → (statistical)                         OK   09:14:00Z  us-east-1           │
│                                                                                │
│ INSIGHT                                                                        │
│   Type          ApiCallRateInsight                                             │
│   State         Start                                                          │
│   Event source  ec2.amazonaws.com                                              │
│   Event name    RunInstances                                                   │
│                                                                                │
│ STATISTICS                                                                     │
│   Baseline      average  0.24 calls/min  (7d window)                           │
│   Insight       average 18.70 calls/min  (during anomaly)                      │
│                                                                                │
│ ATTRIBUTIONS                                                                   │
│   userIdentityArn                                                              │
│     insight   arn:aws:sts::999999999999:assumed-role/DeployRole/ci-41          │
│     baseline  arn:aws:sts::999999999999:assumed-role/DeployRole/ci-*           │
│   userAgent                                                                    │
│     insight   aws-sdk-go-v2/1.30.3                                             │
│     baseline  Terraform/1.8.5                                                  │
│   errorCode                                                                    │
│     insight   (none)                                                           │
│     baseline  (none)                                                           │
│                                                                                │
│ WHEN                                                                           │
│   Event time    2026-04-07T09:14:00Z                                           │
│                                                                                │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

### 4.9 Case I — NetworkActivity / VPCE deny

```
╭─ ct-events/e-c9d0e1f2 ─────────────────────────────────────────────────────────╮
│ W  PutObject    DataPipelineRole → dp-0719 [VPCE]  FAILED (VpceAccessDenied)   │
│    → AWS::S3::Object  arn:aws:s3:::prod-lake/landing/2026/04/07/batch-0719.par │
│    14:44:17Z  eu-central-1                                                     │
│                                                                                │
│ WHO                                                                            │
│   Actor         DataPipelineRole → dp-0719                                     │
│   Account       111111111111                                                   │
│   Principal     arn:aws:iam::111111111111:role/DataPipelineRole                │
│   User agent    aws-sdk-java/2.25.11                                           │
│                                                                                │
│ WHAT                                                                           │
│   Event         PutObject                                                      │
│   Source        s3.amazonaws.com                                               │
│   Category      NetworkActivity  Type   AwsVpceEvent                           │
│                                                                                │
│ WHERE                                                                          │
│   Region        eu-central-1                                                   │
│   Source IP     10.12.4.77                                                     │
│   VPC endpoint  vpce-0ff11223344556677 (acct 111111111111)  [VPCE]             │
│                                                                                │
│ WHEN                                                                           │
│   Event time    2026-04-07T14:44:17Z                                           │
│                                                                                │
│ REQUEST                                                                        │
│   bucketName    prod-lake                                                      │
│   key           landing/2026/04/07/batch-0719.parquet                          │
│                                                                                │
│ ERROR                                                                          │
│   VpceAccessDenied                                                             │
│   The VPC endpoint policy denies the s3:PutObject action on                    │
│   arn:aws:s3:::prod-lake/landing/2026/04/07/batch-0719.parquet                 │
│                                                                                │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

---

## 4b. Composite wireframes with right column

The §4 wireframes show the left pane only at ~96 columns. Real
detail views render a right-column related panel alongside, populated
by `RelatedDef` checkers. For CT events the related groups are derived
from the parsed event (§7b.10).

Sketches below are at ~118 columns: 84-col left card, 32-col right
column, 2-col gap. Cursor focus highlighted with `►`.

### 4b.1 Case A — Karpenter DescribeInstances (no resources, principal-only related)

```
╭─ ct-events/e-a1b2c3d4 ────────────────────────────────────────────────────────╮  ╭──────────────────────────────╮
│ R  DescribeInstances   KarpenterNodeRole → karpenter-1759 [SERVICE]           │  │           RELATED            │
│    → (no resource)                                OK   14:02:11Z  us-east-1   │  │  IAM Roles (1)               │
│                                                                               │  │  IAM Users (0)               │
│ WHO                                                                           │  │  EC2 Instances (0)           │
│   Actor         KarpenterNodeRole → karpenter-1759       [SERVICE] [IMDSv2]   │  │  S3 Buckets (0)              │
│   Account       111111111111                                                  │  │  Lambda Functions (0)        │
│   Principal     arn:aws:iam::111111111111:role/KarpenterNodeRole              │  │  CT events by AccessKeyId    │
│   Issuer role   KarpenterNodeRole                                             │  │  CT events by Username       │
│   Session       karpenter-1759  (started 13:44:02Z, 18m ago)                  │  │  CT events by EventName      │
│   MFA           no                                                            │  ╰──────────────────────────────╯
│   Access key    ASIAY44QH8DCKARPEXMP                                          │
│   User agent    Go SDK v2  (aws-sdk-go-v2/1.30.3)                             │
│   ...                                                                         │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

### 4b.2 Case B — SSO TerminateInstances (instance + role + user pivots)

```
╭─ ct-events/e-b2c3d4e5 ────────────────────────────────────────────────────────╮  ╭──────────────────────────────╮
│ D  TerminateInstances  sso:alice@corp (via AdminAccess) [CONSOLE] [MFA]       │  │           RELATED            │
│    → AWS::EC2::Instance i-0f1e2d3c4b5a69788    OK   14:07:42Z  eu-west-1      │  │  IAM Roles (1)               │
│                                                                               │  │  EC2 Instances (2)           │
│ WHO                                                                           │  │  IAM Users (0)               │
│   Actor         sso:alice@corp (via AdminAccess)        [CONSOLE] [MFA]       │  │  S3 Buckets (0)              │
│   Principal     arn:aws:iam::222222222222:role/aws-reserved/sso.amazonaws.com/│  │  CT events by AccessKeyId    │
│                 AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d                   │  │  CT events by Username       │
│   ...                                                                         │  │  CT events by EventName      │
│ WHAT                                                                          │  ╰──────────────────────────────╯
│   Resources                                                                   │
│     AWS::EC2::Instance  arn:aws:ec2:eu-west-1:222222222222:instance/          │
│                         i-0f1e2d3c4b5a69788                                   │
│ REQUEST                                                                       │
│   instancesSet                                                                │
│     [0] i-0f1e2d3c4b5a69788                                                   │
│     [1] i-0f1e2d3c4b5a69789                                                   │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

Note: per-row jump still works — cursoring to `[1] i-0f1e2d3c4b5a69789`
and pressing `enter` jumps directly to that instance, while the right
column "EC2 (2)" jumps to a filtered EC2 list of both instances. Both
are intentional, see §7b.1.

### 4b.3 Case C — IAMUser PutObject AccessDenied (bucket + object + user)

```
╭─ ct-events/e-c3d4e5f6 ────────────────────────────────────────────────────────╮  ╭──────────────────────────────╮
│ W  PutObject          bob                              FAILED (AccessDenied)  │  │           RELATED            │
│    → AWS::S3::Object  arn:aws:s3:::prod-logs/2026/04/07/app.log  14:11:03Z    │  │  IAM Users (1)               │
│    us-east-1                                                                  │  │  S3 Buckets (1)              │
│                                                                               │  │  S3 Objects (1)              │
│ WHO                                                                           │  │  IAM Roles (0)               │
│   Actor         bob                                      [LONG-LIVED-KEY]     │  │  CT events by AccessKeyId    │
│   Principal     arn:aws:iam::333333333333:user/bob                            │  │  CT events by Username       │
│   Access key    AKIAIOSFODNN7BOB1XMP                                          │  │  CT events by EventName      │
│ WHAT                                                                          │  ╰──────────────────────────────╯
│   Resources                                                                   │
│     AWS::S3::Bucket  arn:aws:s3:::prod-logs                                   │
│     AWS::S3::Object  arn:aws:s3:::prod-logs/2026/04/07/app.log                │
│ ERROR                                                                         │
│   AccessDenied                                                                │
│   User: arn:aws:iam::333333333333:user/bob is not authorized to perform:      │
│   s3:PutObject on resource: arn:aws:s3:::prod-logs/2026/04/07/app.log         │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

### 4b.4 Case E — Root PutBucketPolicy (root has no role/user related rows)

```
╭─ ct-events/e-e5f6a7b8 ────────────────────────────────────────────────────────╮  ╭──────────────────────────────╮
│ ████████████████████████████████████████████████████████████████████████████  │  │           RELATED            │
│ █ ROOT USER ACTION — account 555555555555                                  █  │  │  IAM Roles (0)               │
│ █ W  PutBucketPolicy  prod-artifacts                           OK          █  │  │  IAM Users (0)               │
│ ████████████████████████████████████████████████████████████████████████████  │  │  S3 Buckets (1)              │
│                                                                               │  │  CT events by Username       │
│ WHO                                                                           │  │  CT events by EventName      │
│   Actor         ROOT (account 555555555555)                    [ROOT]         │  ╰──────────────────────────────╯
│   Principal     arn:aws:iam::555555555555:root                                │
│   Access key    (signed with root credentials)                                │
│ WHAT                                                                          │
│   Resources                                                                   │
│     AWS::S3::Bucket  arn:aws:s3:::prod-artifacts                              │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

(No "CT by AccessKey" pivot row — root events have no AccessKeyId.)

### 4b.5 Case G — Cross-account PutObject (decorative role + S3 bucket/object)

```
╭─ ct-events/e-a7b8c9d0 ────────────────────────────────────────────────────────╮  ╭──────────────────────────────╮
│ W  PutObject  CiBuildRole → build-4821 (from 888888888888)  [X-ACCT]          │  │           RELATED            │
│    → AWS::S3::Object  arn:aws:s3:::shared-artifacts/build-4821.tar.gz   OK    │  │  IAM Roles (0)               │
│    14:31:55Z  us-east-2                                                       │  │  IAM Users (0)               │
│                                                                               │  │  S3 Buckets (1)              │
│ WHO                                                                           │  │  S3 Objects (1)              │
│   Actor         CiBuildRole → build-4821                    [X-ACCT]          │  │  CT events by AccessKeyId    │
│   Account       888888888888  (caller)                                        │  │  CT events by Username       │
│   Recipient     777777777777                                                  │  │  CT events by EventName      │
│   Principal     arn:aws:iam::777777777777:role/CiBuildRole   (cross-acct, dim)│  │  CT events by SharedEventId  │
│   Access key    ASIAQF3M2N8KCIB1XMPL                                          │  ╰──────────────────────────────╯
│ WHAT                                                                          │
│   Resources                                                                   │
│     AWS::S3::Bucket  arn:aws:s3:::shared-artifacts                            │
│     AWS::S3::Object  arn:aws:s3:::shared-artifacts/build-4821.tar.gz          │
╰───────────────────────────────────R raw──y copy──/ search──tab cols──esc back──╯
```

`* IAM Roles (0)` because the role lives in account `777777777777` and
the current a9s profile is `888888888888` — see §8 q10. The row stays
visible (typed) but is not actionable.

---

## 7b.10 right-column related groups (`RelatedDef` set for ct-events)

Registered via `RegisterRelated("ct-events", []RelatedDef{...})`. The
checker is **CT-specific**: it parses `CloudTrailEvent` once per event
and emits one `RelatedCheckResult` per group, with the deduplicated
`ResourceIDs` extracted from §7b.2 (userIdentity), §7b.3 (resources[]),
and §7b.4 (requestParameters/responseElements).

| `TargetType` | `DisplayName` | Source paths | Notes |
|---|---|---|---|
| `iam-role` | IAM Roles | sessionIssuer.arn; resources[].type=AWS::IAM::Role; requestParameters.{roleName,roleArn}; STS responseElements.assumedRoleUser.arn | dedup by role name |
| `iam-user` | IAM Users | userIdentity.arn (when type=IAMUser); resources[].type=AWS::IAM::User; requestParameters.userName | dedup by user name |
| `ec2` | EC2 Instances | resources[].type=AWS::EC2::Instance; requestParameters.instancesSet.items[].instanceId; responseElements.instancesSet.items[].instanceId | one ID per instance |
| `s3` | S3 Buckets | resources[].type=AWS::S3::Bucket; requestParameters.bucketName | dedup by bucket |
| `s3_objects` | S3 Objects | resources[].type=AWS::S3::Object; requestParameters.{bucketName,key} pair | dedup by `bucket\|key` |
| `lambda` | Lambda Functions | resources[].type=AWS::Lambda::Function; requestParameters.functionName | normalized |
| `rds` | RDS Instances | resources[].type=AWS::RDS::DB{Instance,Cluster}; requestParameters.dB{Instance,Cluster}Identifier | |
| `kms` | KMS Keys | resources[].type=AWS::KMS::Key; requestParameters.keyId; serviceEventDetails.keyId | parse alias/ARN |
| `secrets` | Secrets | resources[].type=AWS::SecretsManager::Secret; requestParameters.secretId | |
| `vpce` | VPC Endpoints | top-level vpcEndpointId | only when present |
| `sg` | Security Groups | resources[].type=AWS::EC2::SecurityGroup; requestParameters.groupId | |
| `ddb` | DynamoDB Tables | resources[].type=AWS::DynamoDB::Table; requestParameters.tableName | |
| `cfn` | CloudFormation Stacks | resources[].type=AWS::CloudFormation::Stack; requestParameters.stackName | |
| `ct-events` | CT pivots | special: see below | not a count; uses `FetchFilter` |

The CT pivot rows are emitted as `Count=-1, FetchFilter=…` — the
`rightColumnModel` already understands this shape and renders as
"navigate" without a count (see `internal/tui/views/rightcolumn.go:209`):

| Pivot row label | `FetchFilter` | When emitted |
|---|---|---|
| CT events by AccessKeyId | `{AccessKeyId: <key>}` | when `userIdentity.accessKeyId` non-empty (ASIA/AKIA, not Root) |
| CT events by Username | `{Username: <user>}` | always (Username is always derivable) |
| CT events by EventName | `{EventName: <name>}` | always |
| CT events by SharedEventId | client-side (§8 q12) | only when cross-account |

Pivot rows are grouped under a dim "─── pivots ───" separator in the
right column to distinguish them from typed-resource rows. They are
the v1 short-list confirmed by §8 q12.

Empty groups (count = 0) follow standard right-column convention:
they remain visible but dim and not actionable (see
`rightColumnModel.View` zero-count branch). This makes the panel a
stable visual checklist of "what could be related" rather than
shifting layout per event.

---

## 5. Tokyo Night color mapping

Sourced from `internal/tui/styles/palette.go`.

| Element | Style | Color |
|---|---|---|
| Frame border | normal | `ColBorder` `#414868` |
| Section header (WHO/WHAT/WHERE/WHEN/REQUEST/RESPONSE) | bold | `ColAccent` `#7aa2f7` |
| Section header (SERVICE EVENT DETAILS / INSIGHT / STATISTICS / ATTRIBUTIONS) | bold | `ColYAMLBool` `#bb9af7` (purple) |
| Section header (ERROR) | bold | `ColError` `#f7768e` |
| Label column | dim | `ColDim` `#565f89` |
| Value column | default | `ColDetailVal` `#c0caf5` |
| Actor | bold | `ColAccent` `#7aa2f7` |
| Read verb `R` / dim read eventName | dim | `ColDim` `#565f89` |
| Destructive verb `D` (Delete/Terminate/Revoke) | bold | `ColError` `#f7768e` |
| Mutating verb `W` (Create/Put/Update) | bold | `ColYAMLNum` `#ff9e64` (orange) |
| Service verb `S` | bold | `ColAccent` |
| Insight verb `I` | bold | `ColYAMLBool` `#bb9af7` |
| Outcome `OK` | subtle | `ColSuccess` `#9ece6a` |
| Outcome `FAILED` | bold | `ColError` `#f7768e` |
| Error message body | default on dim bg | `ColDetailVal` / `ColRowAltBg` |
| Root banner block | bold on bright red bg | `ColHeaderFg` on `ColError` |
| `[ROOT]` badge | bold | `ColError` `#f7768e` |
| `[MFA]` / `[CONSOLE]` badges | | `ColSuccess` `#9ece6a` |
| `[SERVICE]` / `[IMDSv2]` / `[IRSA]` / `[VPCE]` badges | | `ColAccent` |
| `[X-ACCT]` / `[LONG-LIVED-KEY]` badges | | `ColWarning` `#e0af68` |
| `[AWS-INTERNAL]` badge | dim | `ColDim` |
| `RAW ▸` | dim | `ColDim` |
| Raw JSON body (expanded) | | `ColYAML*` family |

---

## 6. Key bindings

All existing DetailModel keys preserved. New or overloaded:

| Key | Action | Context |
|---|---|---|
| `R` (shift-r) | Toggle RAW section expand/collapse | ct-events detail only. See §8 q1 for why not `r`. |
| `y` | Copy event JSON (full `CloudTrailEvent` string) | ct-events detail only, overrides generic "copy value under cursor". |
| `enter` | Navigate to related resource under cursor | Unchanged. In ct-events, navigable items include `sessionIssuer.arn` → role, `userIdentity.arn` → iam-user when IAMUser, `resources[].ARN` → typed target. |
| `/` | Search within rendered sections (including RAW if expanded) | Unchanged. |
| `w` | Toggle soft wrap | Unchanged. Especially useful for long ARNs and error messages. |
| `r` | Toggle right column (RELATED) | Unchanged — ct-events has related defs (role, iam-user) registered. |
| `tab` | Focus right column | Unchanged. |
| `esc` | Back | Unchanged. |

---

## 7. Responsive behavior

- **Width ≥ 100**: full layout, 32-col right column auto-shown.
- **Width 80–99**: right column narrower (width/3); long ARNs may
  truncate with `…`; `w` recommended.
- **Width 60–79**: right column auto-hidden unless toggled. Badges
  collapse to single-letter glyphs (`[R]`, `[M]`, `[S]`, …).
- **Width < 60**: no right column. Section headers stay, rows wrap
  aggressively. `ROOT` banner degrades to a single red line.
- **Height < 24**: RAW auto-collapses; WHEN merges into HEADER.

---

## 7b. Navigable fields and pivots

CT events are reference-dense: almost every interesting row is a pointer
to another a9s-managed resource. This section enumerates every navigable
target, picks the engine shape, and annotates the wireframes.

### 7b.1 Engine decision — `RelatedDef` for the right column, plus per-row `FieldItem` annotation for in-place jumps

CT events get **both** halves of the existing detail-view machinery:

1. **Right column (`RelatedDef`)** — a CT-specific
   `RegisterRelated("ct-events", []RelatedDef{...})` contributes one row
   per related resource type that the parsed event references. The
   `Checker` function is a CT-specific synchronous analyzer that opens
   `res.RawStruct.(cloudtrailtypes.Event).CloudTrailEvent`, parses the
   JSON once, walks `userIdentity`, `resources[]`,
   `requestParameters`, `responseElements`, and emits one
   `RelatedCheckResult` per target type with the deduplicated
   `ResourceIDs`. Existing `app_related.go` dispatch, render, and
   `RelatedNavigateMsg` flow are unchanged. This replaces the previous
   per-field `RegisterNavigableFields` plan — CT events deliberately do
   NOT call `RegisterNavigableFields`.
2. **Per-row in-place navigation (`FieldItem.IsNavigable`)** — kept,
   because the right column collapses 47 nav targets into ≤8 rows
   ("IAM Roles (2)", "EC2 Instances (3)", …) and loses the context of
   *which* instance the user wants to jump to from
   `instancesSet[1]`. The CT renderer continues to mark individual
   `FieldItem`s as navigable so cursoring onto a specific row and
   pressing `enter` jumps directly to that one resource. The right
   column is the entry point when the user knows the *type* but not the
   row; the row cursor is the entry point when they know the row.

Why both? Single-source designs lose information either way. Right
column alone forces a second selection step for every multi-target
event. Per-row alone makes the user scan the entire detail to learn
"what other resources does this event touch?" — exactly the question
the right column answers in every other detail view. They're
complementary, not redundant. Cost is small: the JSON parse runs once
per detail open, both consumers read from the same parsed `ctRecord`.

The existing `NavigableField{FieldPath, TargetType}` contract is still
too narrow for the per-row half, for three structural reasons:

1. `FieldPath` is looked up against `Resource.Fields` (flat map) or
   `RawStruct` (the SDK envelope `cloudtrail.types.Event`). CT nav
   targets live inside the parsed `CloudTrailEvent` JSON — they're not
   reachable from either source.
2. Multiple independent targets per logical "row" are common: an
   AssumedRole event's `sessionIssuer.arn` → IAM Role **and**
   `userIdentity.arn` → (historical) assumed-role session are two
   different pointers on the same WHO block. `NavigableField` is 1:1.
3. `resources[]` is an array — each entry resolves to a different
   target (different type for each ARN). `NavigableField` has no array
   shape and no per-entry TargetType resolution.

Three options, considered honestly:

| Option | Verdict |
|---|---|
| **A. Extend `NavigableField` with a `JSONPath` variant.** | Rejected. JSONPath against a 100 KB blob-per-row still can't express "parse ARN → pick target type by service prefix". It leaks CT specifics into the generic registry. |
| **B. Synthesize flat `Resource.Fields["_nav.userIdentity.arn"] = "arn:..."` at fetch time; register NavigableField against the synthetic key.** | Rejected. Fetch-time parsing forces every listed CT event to pay a full JSON parse whether or not the user opens it. Doubles memory per event. Worse: target-type resolution for `resources[].ARN` depends on parsing the ARN, which means the fetcher has to encode 10+ service→short-name mappings. That's a renderer concern, not a fetcher concern. |
| **C. Annotate `FieldItem` directly inside `renderCTEvent`.** | Chosen. The hybrid in §1.4 already constructs `[]fieldpath.FieldItem` by hand for each section. `FieldItem` already carries `IsNavigable` and `TargetType` (see `internal/fieldpath/extract.go:331`). The CT renderer sets those two fields per row as it emits. No registry entry, no schema extension, no precomputation. |

**Corollary**: `ct-events` will NOT call `RegisterNavigableFields`.
Per-row navigation metadata lives only in the `FieldItem`s produced by
`renderCTEvent`. Right-column related metadata lives in a single
`RegisterRelated("ct-events", …)` call whose checker reads from the
same parsed `ctRecord`. The existing `DetailModel.handleEnterNavigation`
path reads `FieldItem.IsNavigable` + `FieldItem.TargetType` and
dispatches a `NavigateMsg`, and the existing `app_related.go` path
dispatches `RelatedNavigateMsg` from right-column rows — neither needs
modification.

**Open question** (added to §8): `NavigateMsg` today carries only a
target-type + a resource ID (or filter map). Some CT targets are ARNs
that need to be parsed into `{region, accountId, serviceId}` before
navigation can scope correctly. This is a renderer-side concern: the
CT renderer produces the "navigation payload" (e.g. `"i-0f1e2d3c"` not
the raw ARN) so that `NavigateMsg` receives the same shape it expects
from other resource types. See §8 q9.

### 7b.2 Nav target table — userIdentity (WHO)

All paths rooted at the parsed `ctRecord`. "a9s type" is the short name
registered in `internal/resource`. "When shown" lists the
`userIdentity.type` values under which the row exists.

| JSON path | a9s type | When shown | Nav payload | Fallback |
|---|---|---|---|---|
| `userIdentity.arn` | `iam-user` | `type == IAMUser` | parse `user/<name>` → userName | plain text, not navigable |
| `userIdentity.arn` | `iam-role` | `type == AssumedRole` (points at assumed-role session, but role is the navigable parent) | parse `assumed-role/<roleName>/…` → roleName | plain text |
| `userIdentity.sessionContext.sessionIssuer.arn` | `iam-role` | `type == AssumedRole`, any non-SSO | parse `role/<roleName>` | plain text |
| `userIdentity.sessionContext.sessionIssuer.arn` | `iam-role` (the SSO permission-set reserved role) | `type == AssumedRole` AND arn matches `AWSReservedSSO_*` | parse `role/aws-reserved/.../AWSReservedSSO_<ps>_<hash>` → full name | plain text; permission-set label stays decorative |
| `userIdentity.accountId` | *none* | any | — | decorative. Pivot-style nav to "switch a9s account" is out of scope (§8 q10) |
| `userIdentity.accessKeyId` | *none* | any signed request | — | decorative. Filter pivot "all CT events with this key" — see §7b.5 |
| `userIdentity.invokedBy` | *none* | `type == AWSService`, or service-linked AssumedRole | — | decorative label — service principals aren't a9s resources |
| `userIdentity.sessionContext.webIdFederationData.federatedProvider` | *none* | IRSA / WebIdentity | — | decorative. OIDC providers aren't listed in a9s today (§8 q11) |
| `userIdentity.onBehalfOf.identityStoreArn` | *none* | `type == IdentityCenterUser` | — | decorative. IdC stores aren't a9s resources |

Actor header line (`header1` in the hybrid renderer) is itself rendered
from the same underlying identity fields — it gets one nav target (the
primary principal, following the chain above).

### 7b.3 Nav target table — resources[]

The generic `resources[]` array ships `{type, ARN}` per entry. Target
mapping is by the `type` field (CloudFormation ResourceType namespace),
with ARN parsing for the nav payload:

| `resources[].type` | a9s type | Nav payload (parsed from ARN) |
|---|---|---|
| `AWS::EC2::Instance` | `ec2` | instance-id |
| `AWS::EC2::Volume` | `ebs` (if registered) — else not navigable | volume-id |
| `AWS::EC2::SecurityGroup` | `sg` | sg-id |
| `AWS::EC2::VPC` | `vpc` | vpc-id |
| `AWS::EC2::Subnet` | `subnet` | subnet-id |
| `AWS::EC2::NetworkInterface` | `eni` | eni-id |
| `AWS::S3::Bucket` | `s3` | bucket name |
| `AWS::S3::Object` | `s3_objects` (child view of `s3`) | `{bucket, key}` — see §7b.6 |
| `AWS::IAM::Role` | `iam-role` | role name |
| `AWS::IAM::User` | `iam-user` | user name |
| `AWS::IAM::Policy` | — (not listed in a9s) | decorative |
| `AWS::Lambda::Function` | `lambda` | function name |
| `AWS::RDS::DBInstance` | `rds` | db identifier |
| `AWS::RDS::DBCluster` | `rds` (cluster scoped) | cluster identifier |
| `AWS::DynamoDB::Table` | `ddb` | table name |
| `AWS::KMS::Key` | `kms` | key id (UUID form) |
| `AWS::SecretsManager::Secret` | `secrets` | secret name |
| `AWS::CloudFormation::Stack` | `cfn` | stack name |
| `AWS::ECS::Cluster` / `::Service` / `::TaskDefinition` | `ecs`, `ecs_svc`, `ecs_taskdef` | parsed from ARN |
| `AWS::EKS::Cluster` / `::Nodegroup` | `eks`, `nodegroups` | name |
| `AWS::SNS::Topic` | `sns` | ARN or suffix |
| `AWS::SQS::Queue` | `sqs` | queue name |
| `AWS::Logs::LogGroup` | `log_groups` | log group name |
| `AWS::CloudTrail::Trail` | (not navigable — self-referential) | decorative |
| other / unknown | — | decorative row, dim `(no a9s target)` hover hint |

One row per `resources[]` entry. Each row gets its own
`FieldItem.IsNavigable` and its own `TargetType`. When the array has
≥2 entries of different types (e.g. Bucket + Object), each is
independently navigable. Cursor traversal walks them in order.

### 7b.4 Nav target table — requestParameters / responseElements (top 10 services)

These are the per-service extraction paths from taxonomy §6. Each one
surfaces as its own row in the REQUEST section. When the same
information already appeared in `resources[]`, the REQUEST row is still
navigable (duplicate cursor target is acceptable and consistent with
how EC2 `BlockDeviceMappings.Ebs.VolumeId` works today).

| Service | JSON path | a9s type | Nav payload |
|---|---|---|---|
| EC2 | `requestParameters.instancesSet.items[].instanceId` | `ec2` | instanceId (one row per item, each navigable) |
| EC2 | `responseElements.instancesSet.items[].instanceId` (RunInstances) | `ec2` | instanceId |
| EC2 | `requestParameters.groupId` (AuthorizeSecurityGroup*) | `sg` | sg-id |
| EC2 | `requestParameters.resourcesSet.items[].resourceId` (CreateTags) | by prefix: `i-`→ec2, `vol-`→ebs, `sg-`→sg, `vpc-`→vpc, `subnet-`→subnet | parsed id |
| EC2 | `responseElements.volumeId` | `ebs` | vol-id |
| S3 | `requestParameters.bucketName` | `s3` | bucket name |
| S3 | `requestParameters.bucketName` + `requestParameters.key` | `s3_objects` (child of the bucket) | `{bucket, key}` — see §7b.6 |
| IAM | `requestParameters.roleName` | `iam-role` | roleName |
| IAM | `requestParameters.userName` | `iam-user` | userName |
| IAM | `requestParameters.groupName` | `iam-group` | groupName |
| IAM | `requestParameters.policyArn` | — (policies not listed) | decorative |
| IAM | `requestParameters.instanceProfileName` | — (not listed) | decorative |
| Lambda | `requestParameters.functionName` | `lambda` | normalized function name |
| RDS | `requestParameters.dBInstanceIdentifier` | `rds` | identifier |
| RDS | `requestParameters.dBClusterIdentifier` | `rds` | identifier |
| DynamoDB | `requestParameters.tableName` | `ddb` | table name |
| KMS | `requestParameters.keyId` | `kms` | key id (parse alias/ARN forms) |
| Secrets | `requestParameters.secretId` | `secrets` | secret name |
| STS | `requestParameters.roleArn` (AssumeRole*) | `iam-role` | role name from ARN |
| STS | `responseElements.assumedRoleUser.arn` | `iam-role` | role name |
| CFN | `requestParameters.stackName` | `cfn` | stack name |
| CFN | `requestParameters.stackSetName` | — (not listed) | decorative |
| Generic | `vpcEndpointId` (WHERE row) | `vpce` (if registered) — else decorative | vpce-id |

### 7b.5 Pivot targets (filter-CT-events, not navigate-to-resource)

These do not jump to a different resource type — they re-open the
`ct-events` list scoped by a filter. The existing
`FilteredPaginatedFetcher` (`internal/resource/registry.go:170`) already
supports filter maps; ct-events already uses it for the Username
filter. Pivot rows set `FieldItem.IsNavigable = true` with
`TargetType = "ct-events"` and the renderer attaches a filter map via a
new `FieldItem.NavFilter map[string]string` field (**only extension we
need** — tiny and additive).

| Row | Filter key | Semantics |
|---|---|---|
| Source IP | `SourceIPAddress` | "other CT events from this IP" — CloudTrail LookupAttributes supports this |
| Access key | `AccessKeyId` | "other CT events with this access key" — supported attribute |
| Request ID | `EventId` / `requestID` | single-event lookup (mostly a degenerate filter, keep it decorative) |
| Shared event ID | `sharedEventID` | "same logical event in all recipient accounts" — only cross-account |
| Principal (actor line) | `Username` | already works via the existing flow |
| Footer row `"N more events from this principal →"` | `Username` | synthetic row at the bottom of WHO |

`NavFilter` is checked by `handleEnterNavigation` after `TargetType`:
if non-nil, dispatch as `NavigateMsg{ResourceType: TargetType, Filter: NavFilter}`
instead of `{ResourceType, ResourceID}`. Both shapes already exist in
`messages/`.

### 7b.6 Multi-field navigation payloads (S3 object child view)

S3 object nav is the only case that needs two keys (`bucket` + `key`).
The CT renderer emits **one** navigable row labelled "key" with
`TargetType = "s3_objects"` and a payload that encodes both into a
single string (`<bucket>|<key>`) — the child-view dispatcher already
accepts a delimited id for s3_objects per `internal/resource/related.go`
ContextKeys resolution. No NavigateMsg change.

If the CT event carries only `bucketName` with no `key`, the row's
target type downgrades to `s3` (bucket list).

### 7b.7 Visual marker

- A navigable row has its **value** rendered in `styles.NavigableField`
  (`ColAccent` `#7aa2f7` + underline) — exactly the convention every
  other a9s detail view uses, defined in
  `internal/tui/styles/styles.go:195`. **No trailing arrow, no other
  glyph.** The underline is load-bearing.
- On cursor focus, the whole row background becomes `ColRowAltBg` as
  today — no CT-specific focus style.
- Rows with multiple navigable sub-items (e.g. `instancesSet` with 3
  instance IDs) render one line per sub-item, each independently
  underlined. `FieldItem.IsSubField = true` already handles the indent.
- Section headers and decorative label rows are never underlined.
- Right-column related panel is rendered alongside, populated by the
  CT-specific `RelatedDef` checkers from §7b.1. Related is a
  "resource-typed summary of relations" (jump by type, navigate to a
  filtered list); per-row navigation is "cursor jump on one row" (jump
  to a specific resource). CT events get both independently.

### 7b.8 Keybinding

No new key. `enter` is already the "navigate under cursor" key for
`DetailModel`. In ct-events, cursor lands only on data rows (not
headers), so `enter` is unambiguous. `/` (search) and `y` (copy) still
work per §6 without overlap.

### 7b.9 Count of navigable targets identified

- WHO: up to **3 distinct targets per event** (principal, sessionIssuer
  role, assumed-role ARN).
- resources[]: up to **20 ResourceType → a9s type mappings** (all
  covered in §7b.3).
- requestParameters/responseElements: **24 JSON paths** across 10
  services (§7b.4).
- WHERE: **1 target** (`vpcEndpointId`).
- Pivots: **6 filter pivots** (§7b.5).

Total distinct nav rules: **~54**. All live inside `renderCTEvent`, not
in any registry.

---

## 8. Open questions

1. **`r` key collision.** `ToggleRelated` is `r`. Raw expand/collapse
   also wants `r`. This design picks `R` (shift-r) for raw toggle to
   avoid the conflict. Alternatives: (a) context-dispatch `r` only when
   right column has no actionable rows, (b) reuse `x` for expand.
   Confirm.
2. **`fieldpath.FieldItem` section headers.** Does the struct already
   carry a non-selectable header marker? If not, add
   `IsSectionHeader bool` and skip it in the cursor logic.
3. **`readOnly` display when absent.** Taxonomy §5.6 says the field is
   unreliable. Proposal: render `true (heuristic)` / `false (heuristic)`
   in dim italic, based on the verb classifier. OK?
4. **Session age formatting.** `HH:MM:SS` when `< 24h`, `<d>d <HH>h`
   when `≥ 24h`. Confirm.
5. **SSO session name.** Rendered verbatim; no email parsing.
6. **Insight Start/End pairing.** Show only the row, leave linkage to a
   followup. OK?
7. **Oversized JSON.** 60 KB display cap; `y` copies full up to the
   100 KB CloudTrail limit. OK?
8. **`ct-events.yaml` detail block.** Keep with a comment "unused for
   detail rendering" vs. remove entirely. Prefer the comment.
9. **`NavigateMsg` payload shape for ARN-derived targets.** Parsing
   `arn:aws:ec2:us-west-2:111111111111:instance/i-abc` into a nav target
   that EC2's list view can consume: we strip to `i-abc` and let the
   existing ID-based filter match. But if the ARN's region/account
   differs from the currently selected a9s profile+region, the target
   list will silently return empty. Proposal: pass region into
   `NavigateMsg`, have the router switch region if it differs; refuse
   and show a toast if account differs (see q10). Confirm.
10. **Cross-account nav.** Should a `resources[].ARN` whose accountId
    differs from the current a9s account be navigable at all? Three
    options: (a) decorative only — dim the row and no `→`; (b)
    navigable but shows a toast "target is in account X, switch
    profile to view"; (c) silently navigate and let the target list
    fetcher fail. Prefer (a) for v1 — least surprising, honest.
11. **OIDC provider / IdC store navigation.** IRSA events carry an
    OIDC provider ARN; IdentityCenterUser events carry an identity
    store ARN. Neither resource type is listed in a9s today. Leave
    decorative; reopen if/when added.
12. **Pivot filter attribute names.** CloudTrail `LookupAttributes`
    accepts a fixed set: `EventId, EventName, ReadOnly, Username,
    ResourceType, ResourceName, EventSource, AccessKeyId,
    EventCategory`. `SourceIPAddress` and `sharedEventID` are **not**
    on that list. The §7b.5 table claims IP and sharedEventID pivots
    work — they don't via LookupEvents. We'd need to filter client-side
    after fetch, which is slow for high-volume trails. Proposal: ship
    only the `AccessKeyId`, `Username`, `EventName` pivots in v1; drop
    IP and sharedEventID rows to decorative until we decide on
    client-side filtering. **This is a correction to §7b.5 — flagged
    honestly as uncertain at design time.**

---

## 9. Deliverables recap

- `docs/design/ct-event-detail.md` (this file).
- `cmd/preview/ct_event/main.go` — runnable Lipgloss v2 static render of
  all 9 wireframe cases in §4. No interactivity, no AWS calls.
- No changes under `internal/`, `cmd/a9s/`, `tests/`, `.a9s/`.
