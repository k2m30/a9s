# QA User Stories: CloudTrail Event Detail View Redesign (Issue #245)

Covers the redesigned CloudTrail event detail view — the sectioned
WHO/WHAT/WHERE/WHEN/REQUEST/RESPONSE rendering of a single CloudTrail
event, replacing the previous flat key/value list.

All stories are black-box: derived from `docs/design/ct-event-detail.md`
and `docs/design/ct-taxonomy.md` only. No source code is referenced.

Synthetic account IDs throughout: `111111111111`, `222222222222`,
`333333333333`, `444444444444`, `555555555555`, `666666666666`,
`777777777777`, `888888888888`, `999999999999`.

AWS CLI reference command for any case below:

```
aws cloudtrail lookup-events \
    --lookup-attributes AttributeKey=EventId,AttributeValue=<event-id>
```

Every field cited in a story resolves out of the JSON string returned in
the `CloudTrailEvent` attribute of that command.

---

## A. Section structure and ordering

### A.1 Section headers render in fixed order

| ID | Given | When | Then |
|----|-------|------|------|
| A.1.1 | I am on the CloudTrail events list with at least one `AwsApiCall` write event (e.g., `PutBucketPolicy`) visible. | I press `enter` on that row. | The detail view opens. Section headers appear in this order from top to bottom: `WHO`, `WHAT`, `WHERE`, `WHEN`, `REQUEST`, `RESPONSE` (or `ERROR` when the event failed). Each header is on its own line, bold, blue accent. |
| A.1.2 | I am viewing any CT event detail. | I scroll the viewport. | Section headers remain as non-selectable separator rows. The field cursor skips over them when I press `j`/`k`. |
| A.1.3 | A read-only event has no `responseElements` and no `errorCode`. | I open its detail. | The `RESPONSE` section is omitted entirely — no empty header is rendered. |
| A.1.4 | An `AwsServiceEvent` has no `requestParameters`. | I open its detail. | The `REQUEST` header is replaced by `SERVICE EVENT DETAILS` (purple accent) and `requestParameters` is not shown. |
| A.1.5 | I am viewing any CT event detail. | I look anywhere in the viewport. | No section labeled `RAW` is rendered inline. The bottom hint bar shows `R raw` meaning RAW is reached only via the `R` key toggle. |

---

## B. Wireframe case coverage (design §4.1 – §4.9)

Each story names the wireframe case and asserts the distinctive rendered
values. All inputs are synthetic.

### B.1 Case A — Karpenter service-role DescribeInstances read

| ID | Given | When | Then |
|----|-------|------|------|
| B.1.1 | Event `e-a1b2c3d4`: `eventName=DescribeInstances`, `eventSource=ec2.amazonaws.com`, `userIdentity.type=AssumedRole`, `sessionIssuer.userName=KarpenterNodeRole`, principalId session suffix `karpenter-1759`, `sessionContext.ec2RoleDelivery="2.0"`, no error. | I open its detail. | Header: verb glyph `R` (dim read), eventName `DescribeInstances`, actor `KarpenterNodeRole → karpenter-1759` with badges `[SERVICE]` and `[IMDSv2]`, target `(no resource)` dim, outcome `OK` green, time and region on the right. WHO shows `Actor`, `Account 111111111111`, `Principal arn:aws:iam::111111111111:role/KarpenterNodeRole`, `Issuer role KarpenterNodeRole`, `Session karpenter-1759 (started ..., ... ago)`, `MFA no`, `Access key ASIAY44QH8DCKARPEXMP`, `User agent Go SDK v2`. WHAT shows `Read only true`, `Resources (no resource)`. WHERE shows `Region us-east-1`, `Source IP 10.0.14.221`, `TLS TLSv1.3 TLS_AES_128_GCM_SHA256`. REQUEST shows `filters` and `maxResults 1000`. |

AWS CLI comparison:
```
aws cloudtrail lookup-events --lookup-attributes \
    AttributeKey=EventName,AttributeValue=DescribeInstances
```

### B.2 Case B — SSO IdentityCenter TerminateInstances with MFA

| ID | Given | When | Then |
|----|-------|------|------|
| B.2.1 | Event `e-b2c3d4e5`: `eventName=TerminateInstances`, `userIdentity.type=AssumedRole`, `sessionIssuer.arn` contains `AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d`, session name `alice@corp`, `mfaAuthenticated="true"`, `sourceIPAddress="AWS Internal"`, user agent is the console, `resources[]` contains one `AWS::EC2::Instance` `i-0f1e2d3c4b5a69788`. | I open its detail. | Header verb glyph `D` (red destructive), actor `sso:alice@corp (via AdminAccess)` with badges `[CONSOLE] [MFA]`. WHO shows `Actor` with both badges, `MFA yes`, `Source ident alice@corp`, `User agent Console (AWS Internal)`. WHAT `Read only false` and a `Resources` block with the instance ARN. WHERE shows `Source IP AWS Internal` plus `[AWS-INTERNAL]` dim badge. REQUEST shows `instancesSet` with two items `[0] i-0f1e2d3c4b5a69788` and `[1] i-0f1e2d3c4b5a69789`. RESPONSE shows `terminatingInstances [ i-0f1e2d3c4b5a69788: shutting-down ← running ]`. |

AWS CLI comparison:
```
aws cloudtrail lookup-events --lookup-attributes \
    AttributeKey=EventName,AttributeValue=TerminateInstances
```

### B.3 Case C — IAMUser PutObject AccessDenied (long-lived key)

| ID | Given | When | Then |
|----|-------|------|------|
| B.3.1 | Event `e-c3d4e5f6`: `eventName=PutObject`, `userIdentity.type=IAMUser`, `userName=bob`, `accessKeyId=AKIAIOSFODNN7BOB1XMP`, no `sessionContext`, `errorCode=AccessDenied`, `errorMessage="User: arn:aws:iam::333333333333:user/bob is not authorized to perform: s3:PutObject on resource: arn:aws:s3:::prod-logs/2026/04/07/app.log because no identity-based policy allows the s3:PutObject action"`. | I open its detail. | Header verb glyph `W` orange (mutating), actor `bob` with `[LONG-LIVED-KEY]` yellow badge, outcome `FAILED (AccessDenied)` bold red. WHO: `Actor bob [LONG-LIVED-KEY]`, `Account 333333333333`, `Principal arn:aws:iam::333333333333:user/bob`, `MFA no`, `Access key AKIAIOSFODNN7BOB1XMP`, `User agent AWS CLI v2`. WHAT `Category Data`, `Type AwsApiCall`, `Resources` block with bucket and object rows. There is NO `RESPONSE` header; instead an `ERROR` header (red) is shown with `AccessDenied` and the wrapped error message verbatim. |

AWS CLI comparison:
```
aws cloudtrail lookup-events --lookup-attributes \
    AttributeKey=EventName,AttributeValue=PutObject
```

### B.4 Case D — AwsServiceEvent KMS RotateKey

| ID | Given | When | Then |
|----|-------|------|------|
| B.4.1 | Event `e-d4e5f6a7`: `eventType=AwsServiceEvent`, `eventName=RotateKey`, `userIdentity.type=AWSService`, `invokedBy=kms.amazonaws.com`, `resources[]` has `AWS::KMS::Key` entry, `serviceEventDetails={keyId,rotationType:AUTOMATIC,backingKeyGenerated:true}`, `sourceIPAddress="AWS Internal"`. | I open its detail. | Header verb glyph `S` blue (service), actor `kms.amazonaws.com` with `[SERVICE]` blue badge, outcome `OK`. WHO shows `Actor kms.amazonaws.com [SERVICE]`, `Account 444444444444`, `Invoked by kms.amazonaws.com`. There is no `Session`, no `MFA`, no `Access key` row. WHAT `Type AwsServiceEvent`. WHERE shows `Source IP AWS Internal [AWS-INTERNAL]`. The `REQUEST` header is replaced by `SERVICE EVENT DETAILS` (purple) showing `keyId`, `rotationType AUTOMATIC`, `backingKeyGenerated true`. There is no `RESPONSE` section. |

AWS CLI comparison:
```
aws cloudtrail lookup-events --lookup-attributes \
    AttributeKey=EventName,AttributeValue=RotateKey
```

### B.5 Case E — Root user PutBucketPolicy (red banner)

| ID | Given | When | Then |
|----|-------|------|------|
| B.5.1 | Event `e-e5f6a7b8`: `userIdentity.type=Root`, `accountId=555555555555`, `eventName=PutBucketPolicy`, bucket `prod-artifacts`, console user agent. | I open its detail. | A full-width bright-red banner block appears at the top, spanning the width of the card, reading `ROOT USER ACTION — account 555555555555` on one line and `W PutBucketPolicy prod-artifacts OK` on the next. WHO shows `Actor ROOT (account 555555555555) [ROOT]` with bright-red `[ROOT]` badge, `Principal arn:aws:iam::555555555555:root`, `Access key (signed with root credentials)` dim placeholder, `User agent Console`. WHAT shows `Resources AWS::S3::Bucket arn:aws:s3:::prod-artifacts`. REQUEST shows `bucketName prod-artifacts` and a collapsed `policy` JSON value. |
| B.5.2 | Terminal width is under 60 columns and I open the Root event. | The Root banner renders. | The banner degrades to a single red line containing `ROOT USER ACTION — account 555555555555` (per §7 responsive spec). |

AWS CLI comparison:
```
aws cloudtrail lookup-events --lookup-attributes \
    AttributeKey=Username,AttributeValue=Root
```

### B.6 Case F — WebIdentityUser / IRSA GetObject

| ID | Given | When | Then |
|----|-------|------|------|
| B.6.1 | Event `e-f6a7b8c9`: `eventName=GetObject`, `userIdentity.type=AssumedRole`, `sessionIssuer.userName=eks-checkout-svc-sa` displayed as `checkout-svc-sa`, session `1717156821993453824`, `webIdFederationData.federatedProvider="arn:aws:iam::666666666666:oidc-provider/oidc.eks.eu-west-1.amazonaws.com/id/EXAMPLE0D8C"`, `vpcEndpointId=vpce-0abc123def456`, success. | I open its detail. | Actor `checkout-svc-sa → 1717156821...` with badges `[SERVICE] [IRSA]`. WHO includes a `Web federation` row showing the OIDC provider ARN with an explicit dim suffix `(not navigable — OIDC providers not listed in a9s)`. WHERE shows `VPC endpoint vpce-0abc123def456 (acct 666666666666) [VPCE]` blue badge. REQUEST shows `bucketName checkout-config` and `key prod/config.json`. |

AWS CLI comparison:
```
aws cloudtrail lookup-events --lookup-attributes \
    AttributeKey=EventName,AttributeValue=GetObject
```

### B.7 Case G — Cross-account PutObject

| ID | Given | When | Then |
|----|-------|------|------|
| B.7.1 | Event `e-a7b8c9d0`: `eventName=PutObject`, `userIdentity.type=AssumedRole`, `sessionIssuer.userName=CiBuildRole`, session `build-4821`, `userIdentity.accountId=777777777777` (role), `recipientAccountId=777777777777`, `sharedEventID=f1e2d3c4-b5a6-7890-1234-567890abcdef`, current a9s profile account is `888888888888` (caller context). | I open its detail. | Header shows actor `CiBuildRole → build-4821 (from 888888888888)` with yellow `[X-ACCT]` badge. WHO shows `Account 888888888888 (caller)`, `Recipient 777777777777` decorative, and `Principal arn:aws:iam::777777777777:role/CiBuildRole (cross-acct, dim)`. WHERE shows `Recipient 777777777777 [X-ACCT]` plus `Shared event f1e2d3c4-b5a6-7890-1234-567890abcdef`. |
| B.7.2 | The cross-account event in B.7.1 is displayed. | I compare `userIdentity.accountId` to `recipientAccountId`. | When they differ, the `[X-ACCT]` badge is shown on the header line and a `Recipient` row appears in WHERE. When they match, no `[X-ACCT]` badge or Recipient row is shown. |

AWS CLI comparison:
```
aws cloudtrail lookup-events --lookup-attributes \
    AttributeKey=ResourceType,AttributeValue=AWS::S3::Object
```

### B.8 Case H — Insight event (AwsCloudTrailInsight)

| ID | Given | When | Then |
|----|-------|------|------|
| B.8.1 | Event `e-b8c9d0e1`: `eventCategory=Insight`, `eventType=AwsCloudTrailInsight`, `insightDetails.insightType=ApiCallRateInsight`, `state=Start`, `eventSource=ec2.amazonaws.com`, `eventName=RunInstances`, `insightContext.statistics.baseline.average=0.24`, `insight.average=18.70`, three attributions (`userIdentityArn`, `userAgent`, `errorCode`). | I open its detail. | Header verb glyph `I` purple (insight) with the label `INSIGHT ApiCallRateInsight Start` and target `(statistical)` dim. The usual WHO/WHAT/WHERE headers are replaced by purple-accent `INSIGHT`, `STATISTICS`, and `ATTRIBUTIONS` sections. STATISTICS shows `Baseline average 0.24 calls/min (7d window)` and `Insight average 18.70 calls/min (during anomaly)`. ATTRIBUTIONS lists each attribute with its `insight` and `baseline` values, including a `(none)` placeholder for empty `errorCode`. WHEN header is still present with `Event time`. No REQUEST or RESPONSE section is shown. |

AWS CLI comparison:
```
aws cloudtrail lookup-events --lookup-attributes \
    AttributeKey=EventCategory,AttributeValue=Insight
```

### B.9 Case I — NetworkActivity VPCE deny

| ID | Given | When | Then |
|----|-------|------|------|
| B.9.1 | Event `e-c9d0e1f2`: `eventCategory=NetworkActivity`, `eventType=AwsVpceEvent`, `eventName=PutObject`, `vpcEndpointId=vpce-0ff11223344556677`, `errorCode=VpceAccessDenied`, `errorMessage="The VPC endpoint policy denies the s3:PutObject action on arn:aws:s3:::prod-lake/landing/2026/04/07/batch-0719.parquet"`, actor `DataPipelineRole → dp-0719`. | I open its detail. | Header outcome `FAILED (VpceAccessDenied)` red, actor badged `[VPCE]`. WHAT shows `Category NetworkActivity`, `Type AwsVpceEvent`. WHERE shows `VPC endpoint vpce-0ff11223344556677 (acct 111111111111) [VPCE]`. There is no `RESPONSE`; instead an `ERROR` header with `VpceAccessDenied` and the wrapped error message. |

AWS CLI comparison:
```
aws cloudtrail lookup-events --lookup-attributes \
    AttributeKey=EventCategory,AttributeValue=NetworkActivity
```

---

## C. Actor rendering by userIdentity type (§2.1)

### C.1 All ten variants

| ID | Given `userIdentity.type` and supporting fields | When | Then actor string and badges |
|----|--------------------------------------------------|------|------------------------------|
| C.1.1 | `Root`, `accountId=555555555555`, no alias. | I open the event detail. | Actor reads `ROOT (account 555555555555)` bright red, badge `[ROOT]`. A red banner block is drawn above the header. |
| C.1.2 | `Root`, `accountId=555555555555`, account has alias `acme-prod`. | I open the detail. | Actor reads `ROOT (acme-prod)` bright red with `[ROOT]` badge. |
| C.1.3 | `IAMUser`, `userName=bob`, `accessKeyId=AKIAIOSFODNN7BOB1XMP`, no sessionContext. | I open the detail. | Actor reads `bob`, badge `[LONG-LIVED-KEY]` yellow. |
| C.1.4 | `AssumedRole` non-SSO, `sessionIssuer.userName=DeployRole`, session `ci-41`, `mfaAuthenticated=false`. | I open the detail. | Actor reads `DeployRole → ci-41`, no badges except those that apply. |
| C.1.5 | `AssumedRole` non-SSO with `mfaAuthenticated="true"`. | I open the detail. | Actor shows `[MFA]` green badge. |
| C.1.6 | `AssumedRole` non-SSO with `sessionContext.ec2RoleDelivery="2.0"`. | I open the detail. | Actor shows `[IMDSv2]` blue badge. |
| C.1.7 | `AssumedRole` non-SSO with `webIdFederationData.federatedProvider` non-empty. | I open the detail. | Actor shows `[IRSA]` blue badge. |
| C.1.8 | `AssumedRole` SSO: `sessionIssuer.arn` contains `AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d`, session name `alice@corp`. | I open the detail. | Actor reads `sso:alice@corp (via AdminAccess)`. The `AdminAccess` label is parsed out of `AWSReservedSSO_<PermissionSet>_<hash>`. |
| C.1.9 | `AssumedRole` service-linked: `sessionIssuer.userName=AWSServiceRoleForAutoScaling`, `invokedBy=autoscaling.amazonaws.com`. | I open the detail. | Actor reads `AWSServiceRoleForAutoScaling (via autoscaling.amazonaws.com)` with blue `[SERVICE]` badge. |
| C.1.10 | `FederatedUser`, sessionIssuer userName `AssumeRoleUser`, principalId suffix `123:Alice`. | I open the detail. | Actor reads `federated:AssumeRoleUser/Alice`. |
| C.1.11 | `AWSService`, `invokedBy=kms.amazonaws.com`, no accessKeyId. | I open the detail. | Actor reads `kms.amazonaws.com` with blue `[SERVICE]` badge. No MFA, no session, no access-key rows shown. |
| C.1.12 | `AWSAccount`, `accountId=222222222222`. | I open the detail. | Actor reads `account:222222222222` with yellow `[X-ACCT]` badge. |
| C.1.13 | `WebIdentityUser`, `userName=sub-1234`, `identityProvider=accounts.google.com`. | I open the detail. | Actor reads `webid:sub-1234@accounts.google.com`. |
| C.1.14 | `IdentityCenterUser` with `onBehalfOf.userId=94487408-1234-...`. | I open the detail. | Actor reads `idc:94487408-1234-...`. |
| C.1.15 | `Unknown` or blank type. | I open the detail. | Actor reads `unknown` dim. |

---

## D. Outcome and error surfacing (§2.3, §3.7)

| ID | Given | When | Then |
|----|-------|------|------|
| D.1 | Event with no `errorCode`, `readOnly=false`, `responseElements` non-null containing `bucket` and `versionId`. | I open the detail. | Header outcome is a subtle green `OK`. A `RESPONSE` section (blue accent) shows the identifier-ish responseElements keys (e.g. `bucket`, `versionId`). No `ERROR` header is rendered. |
| D.2 | Event with `errorCode=AccessDenied` and an `errorMessage`. | I open the detail. | Header outcome is bold red `FAILED (AccessDenied)`. There is no `RESPONSE` section. An `ERROR` header (red accent, §3.7) shows `AccessDenied` bold on the first line and the wrapped `errorMessage` verbatim below. No `responseElements` rows are shown for errors. |
| D.3 | Read-only success (`readOnly=true`, `responseElements=null`). | I open the detail. | `RESPONSE` section is omitted entirely. Outcome in the header is `OK` green. |
| D.4 | Event with `errorCode` set but `errorMessage` missing. | I open the detail. | The `ERROR` header still renders with just the error code; the message row is omitted (empty rows collapse). |

---

## E. Right-column related panel (§4b, §7b.10)

### E.1 Group rows

| ID | Given | When | Then |
|----|-------|------|------|
| E.1.1 | Terminal width ≥ 100 and I open any CT event detail. | The view renders. | A 32-column right column titled `RELATED` appears to the right of the detail card. It lists typed resource groups (e.g., `IAM Roles (n)`, `IAM Users (n)`, `EC2 Instances (n)`, `S3 Buckets (n)`, `S3 Objects (n)`, `Lambda Functions (n)`, `RDS Instances (n)`, `KMS Keys (n)`, `Secrets (n)`, `VPC Endpoints (n)`, `Security Groups (n)`, `DynamoDB Tables (n)`, `CloudFormation Stacks (n)`) each followed by a parenthesized integer count. |
| E.1.2 | Event B.2 (SSO TerminateInstances of two instances). | I look at the right column. | `IAM Roles (1)`, `EC2 Instances (2)`, `IAM Users (0)`, `S3 Buckets (0)` rows appear. Zero-count rows remain visible and dim. |
| E.1.3 | Event B.3 (IAMUser PutObject). | I look at the right column. | `IAM Users (1)`, `S3 Buckets (1)`, `S3 Objects (1)`, `IAM Roles (0)` rows are visible. |
| E.1.4 | Any CT event is open. | I look at the pivot separator. | Below the typed-resource groups there is a dim `─── pivots ───` separator, followed by pivot rows without counts: `CT events by AccessKeyId`, `CT events by Username`, `CT events by EventName`. |
| E.1.5 | Event B.1 (root user) has no `accessKeyId`. | I look at pivot rows. | The `CT events by AccessKeyId` pivot row is NOT rendered for a root event. `CT events by Username` and `CT events by EventName` are still rendered. |
| E.1.6 | Event B.7 (cross-account) has `sharedEventID` set. | I look at pivot rows. | An additional pivot row `CT events by SharedEventId` is rendered below `CT events by EventName`. |
| E.1.7 | A zero-count row is rendered dim. | I press `enter` or navigate onto it in the right column. | The row is not actionable — no navigation occurs. |

### E.2 Navigation from the right column

| ID | Given | When | Then |
|----|-------|------|------|
| E.2.1 | I am viewing the CT event detail with the left card focused. | I press `tab`. | Focus moves to the right column. A `►` cursor appears next to the first actionable (non-zero) row. |
| E.2.2 | The right column is focused with `►` on `EC2 Instances (2)`. | I press `enter`. | The app navigates to the EC2 instances list filtered to the two instance IDs extracted from the event. The detail view is left. |
| E.2.3 | The right column is focused on `CT events by Username`. | I press `enter`. | The app navigates back to the CT events list filtered by the `Username` of the current event. The count column in the right column header showed no number (navigate-style row, not a count row). |
| E.2.4 | The right column is focused. | I press `esc` or `tab` again. | Focus returns to the left detail card. |

---

## F. Per-row in-place navigation (§7b.1)

| ID | Given | When | Then |
|----|-------|------|------|
| F.1 | Event B.2 REQUEST section: `instancesSet [0] i-0f1e2d3c4b5a69788`, `[1] i-0f1e2d3c4b5a69789`. | I move the field cursor onto the `[1] i-0f1e2d3c4b5a69789` row. | The value `i-0f1e2d3c4b5a69789` is rendered underlined in accent color (navigable style). |
| F.2 | The cursor is on the `[1]` row of F.1. | I press `enter`. | The app navigates directly to the EC2 detail view for instance `i-0f1e2d3c4b5a69789`, NOT to a filtered list. This is distinct from pressing the right-column `EC2 Instances (2)` row, which opens a filtered list of both instances. |
| F.3 | Event B.1 (Karpenter) WHO section. | I move the cursor onto the `Principal` row containing `arn:aws:iam::111111111111:role/KarpenterNodeRole`. | The value is navigable (underlined, accent). Pressing `enter` opens the IAM role detail for `KarpenterNodeRole`. |
| F.4 | Event B.3 (bob) WHO section. | I move the cursor onto the `Principal` row `arn:aws:iam::333333333333:user/bob`. | The value is navigable and pressing `enter` opens the IAM user detail for `bob`. |
| F.5 | Event B.7 cross-account WHO section. | I move the cursor onto the `Principal arn:aws:iam::777777777777:role/CiBuildRole (cross-acct, dim)` row. | The value is rendered dim and is NOT navigable — the role lives in another account not listed in a9s. Pressing `enter` does nothing. |
| F.6 | Event B.6 (IRSA) WHO section has a `Web federation` row with OIDC provider ARN. | I move the cursor onto that row. | The value is dim and not navigable. |
| F.7 | Any event WHAT section shows a `Resources` block with multiple entries (e.g., bucket + object). | I move the cursor onto any individual resource entry. | Each resource ARN row is independently navigable. Pressing `enter` opens the corresponding a9s resource type detail (bucket opens S3 bucket, object opens the object drill-down). |

---

## G. Per-service REQUEST handling (§3.6, §6)

| ID | Given | When | Then |
|----|-------|------|------|
| G.1 | `eventType=AwsApiCall`, EC2 `RunInstances` with nested `requestParameters`. | I open the detail. | The REQUEST section renders summarized identifier-ish fields (not raw JSON). Remaining nested structure is collapsed to depth 2 by the generic pretty-printer. |
| G.2 | `eventType=AwsServiceEvent`. | I open the detail. | There is no `requestParameters` row. `SERVICE EVENT DETAILS` (purple) replaces REQUEST and renders the contents of `serviceEventDetails`. |
| G.3 | `eventType=AwsConsoleSignIn`, `requestParameters=null`, `additionalEventData={MFAUsed:"Yes",LoginTo:"https://console...",MobileVersion:"No"}`. | I open the detail. | REQUEST shows the `additionalEventData` fields (`MFAUsed`, `MFAIdentifier`, `LoginTo`, `MobileVersion`) instead of raw null requestParameters. |
| G.4 | `eventCategory=Insight`. | I open the detail. | REQUEST is not rendered; instead `INSIGHT`, `STATISTICS`, and `ATTRIBUTIONS` purple-accent sections render as in B.8. |
| G.5 | `eventType=AwsVpceEvent`. | I open the detail. | REQUEST shows the underlying API's `requestParameters` normally, and `vpcEndpointId` is shown prominently in WHERE (not buried in REQUEST). |

---

## H. WHERE section — special flags

| ID | Given | When | Then |
|----|-------|------|------|
| H.1 | `sourceIPAddress="AWS Internal"`. | I open the detail. | WHERE `Source IP` shows `AWS Internal` with dim `[AWS-INTERNAL]` badge. |
| H.2 | `sourceIPAddress="ec2.amazonaws.com"` (service DNS). | I open the detail. | WHERE `Source IP` shows the service DNS string with blue `[SERVICE]` badge. |
| H.3 | `vpcEndpointId` set. | I open the detail. | WHERE shows `VPC endpoint <id> (acct <vpcEndpointAccountId>)` with blue `[VPCE]` badge. |
| H.4 | `tlsDetails.tlsVersion=TLSv1.3`, `cipherSuite=TLS_AES_128_GCM_SHA256`, `clientProvidedHostHeader=ec2.us-east-1.amazonaws.com`. | I open the detail. | WHERE shows `TLS TLSv1.3 TLS_AES_128_GCM_SHA256` and `Host header ec2.us-east-1.amazonaws.com`. |
| H.5 | `edgeDeviceDetails` object present (S3 on Outposts). | I open the detail. | WHERE shows an `Edge device` row with a one-line summary. |

---

## I. WHO section — user agent parsing (§5.5)

| ID | Given `userAgent` | When | Then |
|----|-------------------|------|------|
| I.1 | `aws-cli/2.17.9 Python/3.12.4 Darwin/24.1.0 exe/x86_64 prompt/off command/s3.cp` | I open the detail. | `User agent` row reads `AWS CLI v2 (aws-cli/2.17.9 Python/3.12.4 Darwin/24.1.0)`. |
| I.2 | `aws-sdk-go-v2/1.30.3 os/linux lang/go/1.22.1` | I open the detail. | `User agent` row reads `Go SDK v2 (aws-sdk-go-v2/1.30.3)`. |
| I.3 | `Boto3/1.34.92 md/Botocore#1.34.92 ua/2.0 os/linux/5.15.0 lang/python/3.11.6` | I open the detail. | `User agent` is parsed as `Boto3`. |
| I.4 | `Terraform/1.8.5 (+https://www.terraform.io) terraform-provider-aws/5.65.0` | I open the detail. | `User agent` is parsed as `Terraform`. |
| I.5 | `Mozilla/5.0 ... Safari/605.1.15` | I open the detail. | `User agent` is parsed as `Console` (with the browser string in parentheses). |
| I.6 | `signin.amazonaws.com` as `sourceIPAddress`, or `AWS Internal` source IP on a console-action event. | I open the detail. | `User agent` row shows `Console (AWS Internal)`. |

---

## J. Key bindings and bottom hint bar (§6, §8)

The bottom border hint bar follows the frame hint format used across a9s
(key glyph + short description, separated by `──`).

| ID | Given | When | Then |
|----|-------|------|------|
| J.1 | Any CT event detail is open. | I look at the bottom border of the detail frame. | The bottom border contains, in order, the hints: `R raw`, `y copy`, `/ search`, `tab cols`, `esc back`. Each hint is separated from the next by `──`. |
| J.2 | RAW is collapsed (default). | I press shift-`R`. | A `RAW` section header (purple) appears at the bottom of the viewport. The full `CloudTrailEvent` JSON is rendered with YAML-style coloring. If the blob exceeds 60 KB, the body is truncated with the literal string `... [truncated, press y to copy full]`. |
| J.3 | RAW is expanded. | I press shift-`R` again. | The RAW section collapses and is no longer rendered. |
| J.4 | Any CT event detail is open. | I press `y`. | The full `CloudTrailEvent` JSON string is copied to the clipboard. This overrides the generic "copy value under cursor" behavior for CT events only. |
| J.5 | Any CT event detail is open with multiple sections. | I press `/`, type `AccessDenied`, and press enter. | The rendered sections are searched (including RAW if currently expanded). Matching text is highlighted and the viewport scrolls to the first match. |
| J.6 | Any CT event detail is open. | I press `w`. | Soft wrap toggles. Long ARNs and error messages wrap onto multiple lines instead of being cut at the right margin. |
| J.7 | The right column is shown. | I press `r`. | The right column toggles off. Pressing `r` again toggles it back on. |
| J.8 | The left detail card is focused. | I press `tab`. | Focus moves to the right column and a `►` cursor appears next to the first actionable row. |
| J.9 | Any CT event detail is open. | I press `esc`. | The detail view closes and the CT events list reappears with the same cursor position. |
| J.10 | I am on the field cursor sitting on a navigable row (e.g., `Principal`). | I press `enter`. | The app navigates to the target resource detail; see §F. |

---

## K. Verb classification and outcome colors (§2.2, §5)

| ID | Given `eventName` | When | Then |
|----|-------------------|------|------|
| K.1 | `DescribeInstances`, `ListBuckets`, `GetObject`, `HeadObject`. | I open each detail. | Header verb glyph is `R` dim, eventName is dim. |
| K.2 | `TerminateInstances`, `DeleteBucket`, `RevokeSecurityGroupIngress`, `DetachRolePolicy`. | I open each detail. | Header verb glyph is `D` bold red. |
| K.3 | `CreateRole`, `PutBucketPolicy`, `UpdateFunctionCode`, `ModifyDBInstance`. | I open each detail. | Header verb glyph is `W` bold orange. |
| K.4 | `AssumeRole`, `Decrypt`, `GenerateDataKey`. | I open each detail. | Header verb glyph uses the default ambiguous style (neither red nor orange nor dim). |
| K.5 | `eventType=AwsServiceEvent` (any name). | I open the detail. | Header verb glyph is `S` blue. |
| K.6 | `eventType=AwsCloudTrailInsight` (any name). | I open the detail. | Header verb glyph is `I` purple. |

---

## L. Responsive layout (§7)

| ID | Given terminal size | When | Then |
|----|----------------------|------|------|
| L.1 | Width ≥ 100. | I open a CT event. | Full layout. 32-col right column shown automatically. |
| L.2 | Width 80–99. | I open a CT event. | Right column is narrower (roughly width/3). Long ARNs truncate with `…`. Soft-wrap via `w` is recommended. |
| L.3 | Width 60–79. | I open a CT event. | Right column is hidden unless I press `r` to toggle it on. Badges collapse to single-letter glyphs such as `[R]`, `[M]`, `[S]`. |
| L.4 | Width < 60. | I open a CT event. | Right column cannot be shown. Section headers remain. Row content wraps aggressively. Root banner degrades to a single red line. |
| L.5 | Height < 24. | I open a CT event. | RAW auto-collapses regardless of previous state. WHEN is merged into the HEADER line. |

---

## M. Session metadata and MFA (§3.2)

| ID | Given | When | Then |
|----|-------|------|------|
| M.1 | `sessionContext.attributes.creationDate="2026-04-07T13:44:02Z"`, event time `14:02:11Z`. | I open the detail. | WHO `Session` row shows the session name and `(started 13:44:02Z, 18m ago)` computed session age. |
| M.2 | `sessionContext.attributes.mfaAuthenticated="true"`. | I open the detail. | WHO `MFA` row shows `yes` and actor has a green `[MFA]` badge. |
| M.3 | `eventType=AwsConsoleSignIn` with `additionalEventData.MFAUsed="Yes"`. | I open the detail. | WHO `MFA` row shows `yes` sourced from `additionalEventData`. |
| M.4 | `userIdentity.sessionContext.sourceIdentity="alice@corp"`. | I open the detail. | WHO `Source identity` row shows `alice@corp`. |
| M.5 | `userIdentity.accessKeyId` is empty string. | I open the detail. | WHO `Access key` row is dim and empty (or omitted when the WHO block would otherwise have no data at all). |

---

## N. WHEN section

| ID | Given | When | Then |
|----|-------|------|------|
| N.1 | Event time `2026-04-07T14:02:11Z`, sessionContext creationDate `2026-04-07T13:44:02Z`. | I open the detail. | WHEN shows `Event time 2026-04-07T14:02:11Z`, `Session started 2026-04-07T13:44:02Z`, `Session age 00:18:09`. |
| N.2 | IAMUser event with no sessionContext. | I open the detail. | WHEN shows only `Event time` — no session-started / session-age rows are rendered. |

---

## O. Target (WHOM) resolution (§2.4)

| ID | Given | When | Then |
|----|-------|------|------|
| O.1 | `resources[]` is non-empty with one entry `{type:AWS::EC2::Instance, ARN:...}`. | I open the detail. | Header target and WHAT `Resources` block both render from `resources[]`. |
| O.2 | `resources[]` is empty but `requestParameters.bucketName=prod-logs`. | I open the detail. | Header target falls back to the per-service extractor and shows `prod-logs`. WHAT `Resources` shows the same. |
| O.3 | Neither `resources[]` nor any fallback path resolves. | I open the detail. | Header target reads `(no resource)` dim. WHAT `Resources` reads `(no resource)` dim. |

---

## P. Search (/) interaction

| ID | Given | When | Then |
|----|-------|------|------|
| P.1 | I am on a CT event detail that has WHO/WHAT/WHERE/WHEN/REQUEST/RESPONSE sections. | I press `/` and type `bob`. | Search matches the `bob` substring wherever it appears — WHO actor, Principal ARN, error message — and highlights all matches. Pressing `N` cycles through matches. |
| P.2 | RAW is collapsed. I press `/` and type a term only present inside the raw JSON body. | I press enter. | No match is found, because RAW is not rendered. I press `R` to expand RAW, then `/` again with the same term. | The match is now found and highlighted inside RAW. |

---

Coverage summary: 9 wireframe cases (§B), section ordering + no-inline-RAW
(§A), 15 actor variants (§C), error/outcome rules (§D), right-column
groups + pivots + navigation (§E), per-row in-place navigation (§F), per
-service REQUEST handling (§G), WHERE flags (§H), user agent parsing
(§I), all key bindings from the bottom hint bar (§J), verb classification
(§K), responsive rules (§L), session/MFA (§M), WHEN (§N), target
fallback (§O), search (§P).
