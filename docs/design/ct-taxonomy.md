# CloudTrail Event Taxonomy — Reference

Pure research reference for a9s. Captures the shape of CloudTrail events as
they appear in the JSON string returned by `LookupEvents` (the
`event.CloudTrailEvent` field on the typed SDK struct), the actor
(`userIdentity`) variants, and where the acted-upon resource lives for the
top services. No design decisions here — a subsequent pass will consume this.

> Where the docs were ambiguous or a page redirected, the uncertainty is
> called out inline. Treat anything not explicitly cited as best-effort
> derived from the AWS docs and SDK source listed in §7.

## 0. SDK shape vs. raw event shape

The Go SDK type `cloudtrail/types.Event` (v1.55.8) is a thin envelope:

```go
type Event struct {
    AccessKeyId     *string
    CloudTrailEvent *string   // <-- the rich JSON record, as a string
    EventId         *string
    EventName       *string
    EventSource     *string
    EventTime       *time.Time
    ReadOnly        *string   // "true"/"false" string, NOT bool
    Resources       []Resource
    Username        *string
}

type Resource struct {
    ResourceName *string
    ResourceType *string
}
```

`Username` here is best-effort: CloudTrail derives it from `userIdentity` and
it can be empty (e.g. for many `AssumedRole` events) or set to the role
session name. Treat it as a hint, not a source of truth — parse
`CloudTrailEvent` for anything authoritative.

Everything else in this document refers to the JSON inside
`CloudTrailEvent`, which conforms to CloudTrail event record version 1.11
(the current `eventVersion` as of the docs cited in §7).

---

## 1. Event categories

CloudTrail emits four event categories. They are distinguished by the
top-level `eventCategory` string (added in event record v1.07) and, for
network events, by a distinct `eventType` value.

### 1.1 Management events

- `eventCategory`: `"Management"`
- `eventType`: usually `"AwsApiCall"`; also `"AwsServiceEvent"`,
  `"AwsConsoleSignIn"`, `"AwsConsoleAction"`.
- `managementEvent`: `true`.
- **What it captures.** Control-plane operations on AWS resources —
  create/modify/delete, plus all read APIs against the management surface.
  Also covers IAM, STS, console sign-in, security configuration changes,
  and routing/peering registration.
- **Typical eventNames.** `RunInstances`, `StopInstances`,
  `CreateBucket`, `PutBucketPolicy`, `CreateRole`, `AttachRolePolicy`,
  `AssumeRole`, `ConsoleLogin`, `CreateTable`, `UpdateFunctionCode`,
  `DescribeInstances`, `ListBuckets`, `GetCallerIdentity`.
- **`resources[]`.** Usually populated for write APIs; often empty for
  list/describe calls and for service-emitted events. Do not rely on it
  alone.
- **`requestParameters`.** Generally useful and structured. Some services
  redact secrets (e.g. `kms` plaintext, `secretsmanager` values appear as
  `"HIDDEN_DUE_TO_SECURITY_REASONS"`). Size cap 100 KB.
- **Volume.** Low-to-moderate. Default trail is management events only and
  the first copy is free per account.

### 1.2 Data events

- `eventCategory`: `"Data"`
- `eventType`: `"AwsApiCall"` (also `"AwsServiceEvent"` for some service
  emissions).
- `managementEvent`: absent or `false`.
- **What it captures.** Data-plane operations on resources — the actual
  reads/writes against objects, items, keys, functions, etc. Must be
  explicitly enabled via event selectors; not on by default because of
  volume.
- **Typical eventNames.** S3: `GetObject`, `PutObject`, `DeleteObject`,
  `HeadObject`, `CompleteMultipartUpload`. DynamoDB: `GetItem`, `PutItem`,
  `Query`, `Scan`. Lambda: `Invoke`. KMS: `Decrypt`, `Encrypt`,
  `GenerateDataKey` (KMS data events overlap with management — most KMS
  appears under management). SQS: `SendMessage`, `ReceiveMessage`.
- **`resources[]`.** Almost always populated, typically with both the
  parent resource (bucket, table, function) and the child item ARN where
  one exists.
- **`requestParameters`.** Useful but noisy at scale; bodies are not
  logged, only identifiers (e.g. S3 key, DDB primary key). Object data
  itself never appears.
- **Volume.** Very high. Cost-relevant. Often shipped to a dedicated
  trail or event data store with selectors.

### 1.3 Insight events

- `eventCategory`: `"Insight"`
- `eventType`: `"AwsCloudTrailInsight"` (per the field reference; some
  older blog content uses different names).
- **What it captures.** CloudTrail-detected anomalies in the rate of
  write management API calls or in API error rates, computed against a
  per-region baseline. Two flavours: ApiCallRateInsight and
  ApiErrorRateInsight; also data-event variants on trails (not on event
  data stores).
- **Shape difference.** Instead of `requestParameters`/`responseElements`,
  the meaningful payload lives in `insightDetails`:

  ```json
  "insightDetails": {
      "state": "Start",                       // "Start" or "End"
      "eventSource": "ec2.amazonaws.com",
      "eventName": "RunInstances",
      "insightType": "ApiCallRateInsight",     // or ApiErrorRateInsight
      "insightContext": {
          "statistics": { "baseline": {...}, "insight": {...} },
          "attributions": [
              { "attribute": "userIdentityArn", "insight": [...], "baseline": [...] },
              { "attribute": "userAgent",       "insight": [...], "baseline": [...] },
              { "attribute": "errorCode",       "insight": [...], "baseline": [...] }
          ]
      }
  }
  ```

  Each Insight is delivered as a `Start` event and a matching `End` event.
- **`resources[]`.** Not meaningful (the event is statistical).
- **Volume.** Very low — by design only outliers.

### 1.4 Network activity events

- `eventCategory`: `"NetworkActivity"`
- `eventType`: `"AwsVpceEvent"` (the field reference also lists
  `"AwsVpceEvents"`; the example records use `AwsVpceEvent`. I'm not 100%
  certain which form CloudTrail emits in practice — both have appeared in
  AWS material).
- **What it captures.** API requests that traverse a VPC endpoint
  (interface or gateway) into an AWS service, recorded from the
  VPC-endpoint-owner's perspective. GA in 2025; supported services at
  launch are EC2, KMS, S3, and CloudTrail itself. Captures both control
  plane and data plane calls flowing through the endpoint.
- **Distinguishing fields.** `vpcEndpointId` and `vpcEndpointAccountId` are
  populated. Common to see `errorCode: "VpceAccessDenied"` when a VPC
  endpoint policy blocks the request — that's a primary use case.
- **`resources[]`.** Variable; depends on the underlying API.
- **Volume.** Potentially very high; only enabled via advanced event
  selectors with `eventCategory = NetworkActivity`.

---

## 2. Top-level fields in the raw CloudTrailEvent JSON

Source: CloudTrail "record contents for management, data, and network
activity events" (event record v1.11). Optional/Required is from that
table; the "When it appears" column is interpretation.

| Field | Type | Req? | Since | When it appears |
|---|---|---|---|---|
| `eventVersion` | string | Required | 1.0 | Always. `"1.11"` is current. |
| `eventTime` | string (UTC, ISO-8601) | Required | 1.0 | Always. Time the request completed. |
| `userIdentity` | object | Required | 1.0 | Always. See §4. |
| `eventSource` | string | Required | 1.0 | Always. `"<service>.amazonaws.com"`. For console sign-in: `"signin.amazonaws.com"`. |
| `eventName` | string | Required | 1.0 | Always. The API action (mixed case, e.g. `RunInstances`). |
| `awsRegion` | string | Required | 1.0 | Always. Global services (IAM, STS legacy, CloudFront) use `us-east-1`. |
| `sourceIPAddress` | string | Required | 1.0 | Always. Can be a literal IP, an AWS service DNS (`ec2.amazonaws.com`), or `"AWS Internal"` for internal calls. |
| `userAgent` | string | Optional | 1.0 | Almost always. Up to 1 KB. See §5 parsing rules. |
| `errorCode` | string | Optional | 1.0 | Only on failed requests. |
| `errorMessage` | string | Optional | 1.0 | Only on failed requests. |
| `requestParameters` | object | Required | 1.0 | Always present in the schema; value is `null` for some calls and for some console-sign-in events. Up to 100 KB. |
| `responseElements` | object | Required | 1.0 | Present in schema; `null` for read-only APIs. Up to 100 KB. |
| `additionalEventData` | object | Optional | 1.0 | Service-specific extras (S3 SigV2, ConsoleLogin MFA fields, etc.). Up to 28 KB. |
| `requestID` | string | Optional | 1.01 | Almost always for `AwsApiCall`. Absent for service events. |
| `eventID` | string (GUID) | Required | 1.01 | Always. CloudTrail-assigned. |
| `eventType` | string | Required | 1.02 | Always. See §3. |
| `apiVersion` | string | Optional | 1.01 | Only on `AwsApiCall` for services that version their APIs (mostly EC2/older XML APIs). |
| `readOnly` | bool | Optional | 1.01 | Set by service when known. Not 100% reliable — fall back to the eventName heuristic in §5.6. |
| `resources` | array | Optional | 1.01 | Set when CloudTrail can attribute the event to specific resources; see §6. Each entry: `{ accountId?, type, ARN }`. |
| `recipientAccountId` | string | Optional | 1.02 | Account that received this event. Differs from `userIdentity.accountId` on cross-account access. |
| `serviceEventDetails` | object | Optional | 1.05 | Replaces `requestParameters` for `AwsServiceEvent`. Free-form per service. |
| `sharedEventID` | string (GUID) | Optional | 1.03 | Only when CloudTrail delivered the same logical event to multiple accounts (cross-account scenarios). |
| `vpcEndpointId` | string | Optional | 1.04 | When the request entered AWS through a VPC endpoint. |
| `vpcEndpointAccountId` | string | Optional | 1.09 | Owner account of that VPC endpoint. |
| `eventCategory` | string | Required | 1.07 | Always (since 1.07). One of `Management`, `Data`, `Insight`, `NetworkActivity`. |
| `managementEvent` | bool | Optional | 1.06 | `true` for management events; absent/`false` otherwise. |
| `addendum` | object | Optional | 1.08 | Present only on delayed/updated events. Carries `reason`, `updatedFields`, `originalRequestID`, `originalEventID`. |
| `sessionCredentialFromConsole` | string `"true"`/`"false"` | Optional | 1.08 | Marks events whose credentials came from a console session, even when the request was made via SDK/CLI using temporary creds copied out of the console. Note: the docs define this as a string, not a bool. |
| `edgeDeviceDetails` | object | Optional | 1.08 | Outposts/edge requests (e.g. S3 on Outposts). Up to 28 KB. |
| `tlsDetails` | object | Optional | 1.08 | `tlsVersion`, `cipherSuite`, `clientProvidedHostHeader`. Present for many newer services; not all. |
| `eventContext` | object | Optional | 1.11 | Only in event-data-store events when enrichment is configured for tag/IAM-condition keys. Carries `requestContext` and `tagContext`. |
| `insightDetails` | object | Optional | — | Only when `eventCategory = Insight`. See §1.3. (Not listed in the management/data record-contents page; documented separately under "CloudTrail Insights insightDetails element".) |

---

## 3. `eventType` values

| Value | Meaning | Shape changes |
|---|---|---|
| `AwsApiCall` | A customer- or service-issued API call captured in the normal way. | Default shape: `requestParameters` + `responseElements` populated where applicable. `apiVersion` may be present. |
| `AwsServiceEvent` | Emitted by an AWS service itself (e.g. KMS key rotation, EC2 maintenance, certificate rotation). Not a customer call. | `serviceEventDetails` replaces (or supplements) `requestParameters`. `userIdentity.type` is typically `AWSService` with `invokedBy`. `requestID` often absent. |
| `AwsConsoleSignIn` | Console sign-in lifecycle (`ConsoleLogin`, `CheckMfa`, `GetSigninToken`, `SwitchRole`, `ExitRole`, `RenewRole`). | `eventSource` is `signin.amazonaws.com`. `requestParameters` is typically `null`. `responseElements` carries `{"ConsoleLogin": "Success"\|"Failure"}`. `additionalEventData` carries `MFAUsed`, `MFAIdentifier`, `MfaType`, `LoginTo`, `MobileVersion`. `userIdentity` differs by user type — root sign-in does not include `userName` in `ConsoleLogin`. |
| `AwsConsoleAction` | Console-issued actions that don't correspond to a public API (e.g. password change). | Sparse `requestParameters`/`responseElements`. Treated as management. |
| `AwsCloudTrailInsight` | An Insight event. | Carries `insightDetails`; lacks normal `requestParameters`/`responseElements`. Two records per Insight (`Start` then `End`). |
| `AwsVpceEvent` | A network activity event recorded at a VPC endpoint. | Carries `vpcEndpointId` and `vpcEndpointAccountId`. Often carries `errorCode: "VpceAccessDenied"`. (Spelling note: AWS docs also reference `AwsVpceEvents`; uncertain which is canonical.) |

---

## 4. `userIdentity` types

All values below are taken from the CloudTrail userIdentity element
reference. Field-level "absent" means "not set by CloudTrail for this
type"; `accessKeyId` is present whenever the request was signed with an
access key (so most types can carry it).

### 4.1 `Root`

- Present: `type`, `accountId`, optionally `principalId`, `arn`,
  `accessKeyId`, and `userName` (which contains the **account alias**, not
  a user name — only set if the account has an alias).
- Absent: `sessionContext` (except when root has assumed a role; then it
  appears as `AssumedRole` with the root issuer).
- Render: `root@<account>` or `root (<alias>)`.
- Note: For `ConsoleLogin` events, root sign-in does not include
  `userName`.

### 4.2 `IAMUser`

- Present: `type`, `principalId` (`AIDA…`), `arn`, `accountId`, `userName`,
  optionally `accessKeyId`.
- Absent: `sessionContext`.
- Render: `userName` (fall back to `arn`).
- Example:

  ```json
  "userIdentity": {
      "type": "IAMUser",
      "principalId": "AIDAJ45Q7YFFAREXAMPLE",
      "arn": "arn:aws:iam::123456789012:user/Alice",
      "accountId": "123456789012",
      "accessKeyId": "",
      "userName": "Alice"
  }
  ```

### 4.3 `AssumedRole`

- Present: `type`, `principalId` (format `<RoleId>:<SessionName>`), `arn`
  (`arn:aws:sts::<acct>:assumed-role/<RoleName>/<SessionName>`),
  `accountId`, optionally `accessKeyId`, and a `sessionContext` with a
  `sessionIssuer` chain.
- Absent: top-level `userName` (the role name is inside
  `sessionContext.sessionIssuer.userName`; the session name is parsed from
  the ARN or the principalId suffix).
- Optional inside `sessionContext`:
  - `attributes.mfaAuthenticated` — `"true"` / `"false"`.
  - `attributes.creationDate`.
  - `sourceIdentity` — set when the assumer passed `--source-identity`,
    propagated through role chains. Useful as a stable user attribution.
  - `ec2RoleDelivery` — `"1.0"` (IMDSv1) or `"2.0"` (IMDSv2). Present on
    requests signed with an EC2 instance-profile credential.
  - `webIdFederationData` — populated for roles assumed via
    `AssumeRoleWithWebIdentity`; carries `federatedProvider` and
    `attributes`. Empty object when not federated.
- May also include `invokedByDelegate` (cross-account delegated invoker).
- SSO/Identity Center assumed roles look like normal `AssumedRole` events,
  but the `sessionIssuer.arn` will be a role under
  `AWSReservedSSO_<PermissionSet>_<hash>`, and the session name is the SSO
  user identifier. That's how you distinguish "human via SSO" from
  "service via role".
- Example:

  ```json
  "userIdentity": {
      "type": "AssumedRole",
      "principalId": "AROAIDPPEZS35WEXAMPLE:AssumedRoleSessionName",
      "arn": "arn:aws:sts::123456789012:assumed-role/RoleToBeAssumed/MySessionName",
      "accountId": "123456789012",
      "accessKeyId": "",
      "sessionContext": {
          "sessionIssuer": {
              "type": "Role",
              "principalId": "AROAIDPPEZS35WEXAMPLE",
              "arn": "arn:aws:iam::123456789012:role/RoleToBeAssumed",
              "accountId": "123456789012",
              "userName": "RoleToBeAssumed"
          },
          "attributes": {
              "mfaAuthenticated": "false",
              "creationDate": "20131102T010628Z"
          }
      }
  }
  ```

- Render: prefer `sessionContext.sessionIssuer.userName / <sessionName>`.
  When `AWSReservedSSO_*`, render as `sso:<sessionName>` and surface the
  permission set.

### 4.4 `FederatedUser`

- Present: `type`, `principalId`, `accountId`, optional `accessKeyId`,
  and `sessionContext` (with `sessionIssuer` whose `type` is `Root` or
  `IAMUser` — i.e. who called `GetFederationToken`).
- Absent: top-level `userName`.
- Render: `federated:<sessionIssuer.userName>/<principalId suffix>`.

### 4.5 `AWSService`

- Present: `type`, `invokedBy` (service principal,
  e.g. `"autoscaling.amazonaws.com"`), often `principalId` and `accountId`.
- Absent: `userName`, usually `arn`.
- Appears for events emitted by AWS services on the customer's behalf
  (Auto Scaling launching instances, ECS pulling images via a task role,
  Config snapshotting). Often paired with `eventType: AwsServiceEvent`.
- Render: `aws:<invokedBy>`.

### 4.6 `AWSAccount`

- Present: `type`, `principalId`, `accountId`, optional `arn`.
- Absent: `userName`, `sessionContext`.
- Marks cross-account requests where CloudTrail can identify the calling
  account but not the principal within it (typical when a different AWS
  account assumes a role you trust and the event is logged to the trustor
  side).
- Render: `account:<accountId>`.

### 4.7 `SAMLUser`

- Present: `type`, `principalId` (`<saml:namequalifier>:<saml:sub>`),
  `userName` (= `saml:sub`), `identityProvider` (= `saml:namequalifier`),
  optional `accountId`.
- Absent: `sessionContext`, `sessionIssuer`.
- Only appears on the `AssumeRoleWithSAML` request itself; subsequent
  signed requests show as `AssumedRole`.
- Render: `saml:<userName>@<identityProvider>`.

### 4.8 `WebIdentityUser`

- Present: `type`, `principalId` (`<issuer>:<app-id>:<user-id>` shape),
  `userName` (the user-id), `identityProvider`
  (e.g. `accounts.google.com`, `cognito-identity.amazon.com`,
  `www.amazon.com`, `graph.facebook.com`), optional `accountId`.
- Absent: `sessionContext`, `sessionIssuer`.
- Only on the `AssumeRoleWithWebIdentity` call itself.
- Example:

  ```json
  "userIdentity": {
      "type": "WebIdentityUser",
      "principalId": "accounts.google.com:application-id.apps.googleusercontent.com:user-id",
      "userName": "user-id",
      "identityProvider": "accounts.google.com"
  }
  ```

### 4.9 `IdentityCenterUser`

- Present: `type`, `accountId` (the account hosting the Identity Center
  instance), `onBehalfOf` (`{ userId, identityStoreArn }`), `credentialId`
  (bearer token id). `principalId` and `arn` may be present.
- Absent: top-level `userName` (it appears in `additionalEventData` only
  for Identity Center sign-in events).
- This is the *direct* Identity Center actor for bearer-token-based APIs
  (e.g. Q, Identity Center admin actions). The much more common
  "SSO user did something via a permission set" pattern shows up as
  `AssumedRole` with an `AWSReservedSSO_*` issuer — see §4.3.
- Render: `idc:<onBehalfOf.userId>`.
- Example:

  ```json
  "userIdentity": {
      "type": "IdentityCenterUser",
      "accountId": "123456789012",
      "onBehalfOf": {
          "userId": "544894e8-80c1-707f-60e3-3ba6510dfac1",
          "identityStoreArn": "arn:aws:identitystore::123456789012:identitystore/d-9067642ac7"
      },
      "credentialId": "EXAMPLEVHULjJdTUdPJfofVa1sufHDoj7aYcOYcxFVllWR_Whr1fEXAMPLE"
  }
  ```

### 4.10 `Directory`

- Present: `type`, `accountId`, sometimes `userName` (account alias or
  email).
- Used for AWS Directory Service / WorkSpaces-style flows. The doc gives
  minimal field coverage; treat as best-effort.
- Render: `directory:<userName or accountId>`.

### 4.11 `Unknown`

- Present: `type: "Unknown"`, sometimes `accountId`, sometimes `userName`.
- Most other fields absent.
- Appears when CloudTrail cannot resolve the principal (very old events,
  certain anonymous service interactions). Render as `unknown`.

### 4.12 Bare `Role` (sessionIssuer only)

The reference also documents a `Role` `type` value, but this is the
*shape used inside `sessionContext.sessionIssuer`*, not a top-level
`userIdentity.type`. If you ever see it at the top level, treat it as
equivalent to `AssumedRole` without a session.

---

## 5. Special cases worth surfacing

### 5.1 Errors

Top-level `errorCode` and `errorMessage` are populated whenever the
underlying API returned an error. CloudTrail still records the event
(this is how you observe AccessDenied / throttling / validation
failures). The corresponding `responseElements` is usually `null`.
Common high-signal codes:

- `AccessDenied`, `UnauthorizedOperation`, `Client.UnauthorizedOperation`
- `VpceAccessDenied` (network activity events only)
- `Throttling`, `ThrottlingException`, `RequestLimitExceeded`
- `ValidationException`, `InvalidParameterValue`
- `ResourceNotFoundException`, `NoSuchEntity`, `NoSuchBucket`

### 5.2 MFA

`userIdentity.sessionContext.attributes.mfaAuthenticated` is the string
`"true"` or `"false"`. Note: it's a string, not a bool. Only present for
identity types that have a `sessionContext` (`AssumedRole`,
`FederatedUser`). For console sign-in, `additionalEventData.MFAUsed`
("Yes"/"No") and `additionalEventData.MFAIdentifier` are the equivalent.

### 5.3 Cross-account

`recipientAccountId != userIdentity.accountId` means "an actor in another
account caused this event in mine." `sharedEventID` will likely also be
present, linking the same logical request as it was delivered to multiple
accounts. For trust-policy debugging, this combination is the primary
signal.

### 5.4 Service-linked role / service-on-your-behalf

Two patterns:

1. `userIdentity.type == "AWSService"` with `invokedBy` set — the service
   acted entirely under its own credentials.
2. `userIdentity.type == "AssumedRole"` with `invokedBy` *also* set
   (e.g. `"autoscaling.amazonaws.com"`) and the role being a
   service-linked role (`AWSServiceRoleFor*`). This is the more common
   "service used a role in your account to do work."

### 5.5 Console vs SDK vs service (userAgent parsing)

Heuristics that hold up well in practice:

| Pattern in `userAgent` | Likely caller |
|---|---|
| `signin.amazonaws.com` | Console sign-in pipeline |
| `console.amazonaws.com`, `AWS Internal` (also seen as `sourceIPAddress`) | Console-issued API call |
| `aws-cli/<version>` | AWS CLI v1 |
| `aws-cli/2.<version>` | AWS CLI v2 |
| `aws-sdk-go/`, `aws-sdk-go-v2/` | Go SDK |
| `aws-sdk-java/`, `aws-sdk-nodejs/`, `aws-sdk-cpp/`, `aws-sdk-ruby/` | Other SDKs |
| `Boto3/`, `Botocore/` | Python SDK |
| `AWS-Internal/3` (and similar) | Internal AWS service caller |
| `<service>.amazonaws.com` | AWS service on behalf of customer (cross-check `sourceIPAddress`) |
| `Mozilla/5.0 ...` | Browser (console action issued via signed XHR) |
| `Terraform/`, `HashiCorp/go-cleanhttp` | Terraform |
| `Pulumi/` | Pulumi |

`sessionCredentialFromConsole == "true"` is a stronger "this was a
console-derived credential" signal than `userAgent` alone.

### 5.6 Read vs write

Two complementary signals:

1. The top-level `readOnly` field, **when present**. It is optional and
   not every service sets it correctly.
2. The eventName prefix heuristic:
   - **Read**: `Describe*`, `List*`, `Get*`, `Head*`, `BatchGet*`,
     `Query*`, `Scan*`, `Lookup*`, `Search*`, `Select*`, `View*`.
   - **Write**: `Create*`, `Put*`, `Post*`, `Update*`, `Modify*`,
     `Delete*`, `Remove*`, `Detach*`, `Attach*`, `Start*`, `Stop*`,
     `Run*`, `Terminate*`, `Reboot*`, `Restore*`, `Copy*`, `Replace*`,
     `Set*`, `Enable*`, `Disable*`, `Authorize*`, `Revoke*`, `Tag*`,
     `Untag*`, `Reset*`, `Cancel*`, `Submit*`, `Send*`, `Publish*`,
     `Issue*`, `Renew*`, `Rotate*`.
   - **Ambiguous**: `Assume*` (read in form, security-relevant in
     practice), `Decrypt`, `Encrypt`, `Sign`, `Verify`, `GenerateDataKey`
     (KMS data plane).

### 5.7 VPC endpoint access

`vpcEndpointId` (and `vpcEndpointAccountId` since 1.09) is present when
the request entered AWS via a VPC interface or gateway endpoint.
`AwsVpceEvent` (`eventCategory: NetworkActivity`) is the dedicated form;
classic management/data events that happen to traverse a VPCE will also
carry `vpcEndpointId`.

### 5.8 IP vs AWS internal

`sourceIPAddress` can be:

- A literal IPv4/IPv6 address — normal customer call.
- A service DNS name like `ec2.amazonaws.com`, `lambda.amazonaws.com` —
  the call originated from another AWS service in the same trust domain.
- The exact string `"AWS Internal"` — internal AWS plumbing.
- For console actions, an IP plus a `userAgent` containing `console`.

---

## 6. Resource target extraction (top 10 services)

Where the acted-upon resource ID/ARN actually lives. `resources[]` is the
canonical place when CloudTrail can populate it; otherwise dig into
`requestParameters`. Paths below are JSONPath-ish, rooted at the raw
event.

> These paths are derived from the CloudTrail event reference and from
> per-service "Logging API calls with CloudTrail" pages. EC2's
> `instancesSet.items[]` shape and S3's `bucketName`/`key` are well
> attested; the rest match the public API request shapes. When in doubt,
> `resources[].ARN` is always safe if present.

### 6.1 EC2 (`ec2.amazonaws.com`)

- Top-level: `resources[].ARN` is populated for many actions
  (`RunInstances`, `TerminateInstances`, etc.) but **frequently empty**
  for read calls.
- `RunInstances` response: `responseElements.instancesSet.items[].instanceId`.
- `TerminateInstances`/`StartInstances`/`StopInstances`/`RebootInstances`
  request: `requestParameters.instancesSet.items[].instanceId`.
- `AuthorizeSecurityGroupIngress`/`Egress`:
  `requestParameters.groupId` (or `groupName` for EC2-Classic remnants).
- `CreateTags` / `DeleteTags`:
  `requestParameters.resourcesSet.items[].resourceId`.
- `CreateVolume` response: `responseElements.volumeId`.
- `CreateSnapshot` response: `responseElements.snapshotId`.

### 6.2 S3 (`s3.amazonaws.com`)

- Bucket: `requestParameters.bucketName` (always set on bucket-scoped
  ops); also `resources[].ARN = arn:aws:s3:::<bucket>` and `type =
  AWS::S3::Bucket`.
- Object: `requestParameters.key` (data events). For multipart:
  `requestParameters.uploadId`. `resources[]` for object data events
  carries `arn:aws:s3:::<bucket>/<key>` with `type = AWS::S3::Object`.
- `requestParameters.Host` carries the virtual-hosted bucket. SigV4
  region is in `additionalEventData.SignatureVersion` etc.

### 6.3 IAM (`iam.amazonaws.com`)

- `roleName` for role ops (`CreateRole`, `AttachRolePolicy`, `PutRolePolicy`).
- `userName` for user ops.
- `groupName` for group ops.
- `policyArn` for managed-policy ops.
- `policyName` for inline policy ops (paired with `roleName`/`userName`).
- `instanceProfileName` for instance profile ops.
- `resources[]` is populated for most IAM mutating actions with
  `arn:aws:iam::<acct>:role/<roleName>` etc.

### 6.4 Lambda (`lambda.amazonaws.com`)

- `requestParameters.functionName` — may be the bare name, the ARN, or
  partial ARN. Normalize before display.
- For `Invoke` (data event): same field plus
  `requestParameters.qualifier` for version/alias.
- `resources[].ARN`: function ARN, plus the layer ARN for layer ops.

### 6.5 RDS (`rds.amazonaws.com`)

- `requestParameters.dBInstanceIdentifier` (note camel case
  `dBInstanceIdentifier` — that's how the API ships).
- `requestParameters.dBClusterIdentifier` for Aurora.
- `requestParameters.dBSnapshotIdentifier` /
  `dBClusterSnapshotIdentifier`.
- `responseElements.dBInstance.dbiResourceId` is the immutable ID.
- `resources[].ARN` is usually populated for write operations.

### 6.6 DynamoDB (`dynamodb.amazonaws.com`)

- `requestParameters.tableName` for both management and data events.
- For data events: `requestParameters.key` (the primary-key map),
  `requestParameters.indexName` for index queries.
- `resources[].ARN`: `arn:aws:dynamodb:<region>:<acct>:table/<tableName>`.

### 6.7 KMS (`kms.amazonaws.com`)

- `requestParameters.keyId` (alias, key ID, or ARN form — normalize).
- `requestParameters.encryptionContext` is sometimes present and is the
  best way to attribute usage to a logical workload.
- `resources[].ARN` is generally populated and authoritative.
- Key plaintext never appears.

### 6.8 Secrets Manager (`secretsmanager.amazonaws.com`)

- `requestParameters.secretId` (name or ARN).
- For `GetSecretValue`: `requestParameters.versionId` /
  `requestParameters.versionStage` (the value itself is **not** logged;
  the field appears as `"HIDDEN_DUE_TO_SECURITY_REASONS"` if it would
  otherwise leak).
- `resources[].ARN` is the secret ARN.

### 6.9 STS (`sts.amazonaws.com`)

- `AssumeRole`: `requestParameters.roleArn`,
  `requestParameters.roleSessionName`, optional
  `requestParameters.sourceIdentity`,
  `requestParameters.externalId` (not logged in cleartext if marked
  sensitive — depends on the call). Response:
  `responseElements.assumedRoleUser.{arn, assumedRoleId}`.
- `AssumeRoleWithSAML` / `AssumeRoleWithWebIdentity`: same `roleArn`
  field; SAML assertions and OIDC tokens are **not** logged.
- `GetCallerIdentity`: no resource — pure read.
- `resources[]` carries the role ARN for assume-role calls.

### 6.10 CloudFormation (`cloudformation.amazonaws.com`)

- `requestParameters.stackName` (name or ARN).
- For change sets: `requestParameters.changeSetName`.
- For stack-set ops: `requestParameters.stackSetName`.
- Response: `responseElements.stackId` is the canonical ARN.
- `resources[].ARN` carries the stack ARN.

---

## 7. References

- AWS CloudTrail User Guide — *CloudTrail record contents for management,
  data, and network activity events* (event record v1.11):
  <https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-event-reference-record-contents.html>
- AWS CloudTrail User Guide — *CloudTrail userIdentity element*:
  <https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-event-reference-user-identity.html>
- AWS CloudTrail User Guide — *AWS Management Console sign-in events*:
  <https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-event-reference-aws-console-sign-in-events.html>
- AWS CloudTrail User Guide — *CloudTrail Insights insightDetails element*:
  <https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-event-reference-insight-details.html>
- AWS CloudTrail User Guide — *Logging network activity events*:
  <https://docs.aws.amazon.com/awscloudtrail/latest/userguide/logging-network-events-with-cloudtrail.html>
- AWS Cloud Operations Blog — *Announcing AWS CloudTrail network
  activity events for VPC Endpoints*:
  <https://aws.amazon.com/blogs/mt/announcing-aws-cloudtrail-network-activity-events-for-vpc-endpoints/>
- AWS CloudTrail User Guide — *Logging management events* /
  *Logging data events*:
  <https://docs.aws.amazon.com/awscloudtrail/latest/userguide/logging-management-and-data-events-with-cloudtrail.html>
- AWS CloudTrail User Guide — *Logging Insights events*:
  <https://docs.aws.amazon.com/awscloudtrail/latest/userguide/logging-insights-events-with-cloudtrail.html>
- AWS CloudTrail User Guide — *Understanding CloudTrail events*:
  <https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-events.html>
- Go module cache: `github.com/aws/aws-sdk-go-v2/service/cloudtrail@v1.55.8/types/types.go`
  (the `Event` and `Resource` types confirm that the rich JSON record
  lives only in the `CloudTrailEvent` string field).

### Open uncertainties

- **`AwsVpceEvent` vs `AwsVpceEvents`.** The field reference table lists
  the eventType under both spellings in different AWS sources. I have not
  observed enough live events to be certain which one CloudTrail emits.
  Treat both as the same category.
- **`Directory` userIdentity.** The reference page documents this type
  only minimally. The field list above is what's documented; live events
  may carry more.
- **`insightDetails` is not enumerated** in the management/data record
  contents page — it has its own page. The shape in §1.3 is taken from
  that page plus the AWS CLI viewer doc; nested `insightContext` fields
  vary slightly between ApiCallRate and ApiErrorRate variants.
- **`readOnly` reliability.** Documented as optional; in practice some
  services omit it or set it inconsistently. Always combine with the
  eventName-prefix heuristic.
