# CloudTrail Events List View — QA Stories (#246)

Scope: redesign of the `ct-events` resource list view per
`docs/design/ct-event-list.md` and the event taxonomy in
`docs/design/ct-taxonomy.md`. Detail-view behavior is covered by
`docs/qa/issue-59-cloudtrail-events.md` and the `#245` design;
this file covers only the **list** view.

All synthetic data uses account IDs `111111111111`,
`222222222222`, `333333333333`. Event names, principals, and ARN
fragments are illustrative only.

---

## 1. Column set, order, and widths

### Story: Default column set in order

**Given:** the user opens `:ct-events` in a 132-column terminal
**When:** the list view loads its first page
**Then:** the columns appear left-to-right as: verb-glyph (1ch),
TIME (19ch), ACTOR (26ch), ORIGIN (7ch), EVENT (22ch), TARGET
(28ch), OUTCOME (14ch), with single-space gutters between cells.
The verb-glyph column has a blank header label; every other
column header is bold accent text.

### Story: Time column uses canonical absolute format

**Given:** an event recorded at 14:31:12 UTC on 2026-04-07
**When:** the row is rendered
**Then:** the TIME cell shows `2026-04-07 14:31:12` exactly,
matching the canonical a9s timestamp format used by RDS events,
ASG activities, and SFN execution history. No relative time, no
timezone suffix.

### Story: TARGET truncates middle-elide

**Given:** a row whose TARGET is `s3/prod-logs/2026/04/07/long/path/file.json`
and the column width is 28
**When:** the row is rendered
**Then:** TARGET shows `s3/prod-logs/2026/04/07/…` with a single
ellipsis where the middle of the string was removed.

### Story: OUTCOME truncates end-elide on long error codes

**Given:** an event with `errorCode = AccessDeniedException` and
column width 14
**When:** the row is rendered
**Then:** OUTCOME shows `FAILED AccessD…` (end-elide).

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 25
```

Expected fields visible: verb-glyph, TIME, ACTOR, ORIGIN, EVENT,
TARGET, OUTCOME.

---

## 2. Verb glyphs (§8a)

### Story: R glyph for read events

**Given:** an event with `eventName = DescribeInstances`
**When:** the row is rendered
**Then:** column 1 shows `R` in dim foreground (`ColDim`).

### Story: R glyph also matches Get*/List*/Head*

**Given:** events `GetCallerIdentity`, `ListBuckets`, `HeadObject`
**When:** rendered
**Then:** all three rows show `R` in dim style.

### Story: W glyph for mutating writes

**Given:** an event with `eventName = PutObject`
**When:** rendered
**Then:** column 1 shows `W` in bold orange (`ColYAMLNum`).

### Story: W glyph matches Create*/Update*/Attach*/Modify*

**Given:** events `CreateFleet`, `UpdateFunctionCode`,
`AttachRolePolicy`, `ModifyInstanceAttribute`
**When:** rendered
**Then:** each row shows `W` in bold orange.

### Story: D glyph for destructive events

**Given:** an event with `eventName = TerminateInstances`
**When:** rendered
**Then:** column 1 shows `D` in bold red (`ColError`).

### Story: D glyph matches Delete*/Revoke*/Detach*

**Given:** events `DeleteLogGroup`, `RevokeSecurityGroupIngress`,
`DetachRolePolicy`
**When:** rendered
**Then:** each row shows `D` in bold red.

### Story: S glyph for service-emitted events

**Given:** an event with `eventType = AwsServiceEvent`, e.g.
`TerminateInstanceInAutoScalingGroup` from `ec2.amazonaws.com`
**When:** rendered
**Then:** column 1 shows `S` in bold accent (`ColAccent`).

### Story: I glyph for Insight events

**Given:** an event with `eventCategory = Insight` and
`eventName = ApiCallRateInsight`
**When:** rendered
**Then:** column 1 shows `I` in bold purple (`ColYAMLBool`).

### Story: N glyph for NetworkActivity events

**Given:** an event with `eventCategory = NetworkActivity`
emitted via a VPC endpoint
**When:** rendered
**Then:** column 1 shows `N` in bold accent (`ColAccent`).

### Story: ? glyph for ambiguous classifications

**Given:** an event whose name does not match any verb pattern
(e.g. `ConsoleLogin`) and is not a service / insight / network
event
**When:** rendered
**Then:** column 1 shows `?` in plain `ColHeaderFg`.

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=EventName,AttributeValue=DescribeInstances
```

Expected fields visible: verb-glyph (R), TIME, ACTOR, EVENT.

---

## 3. Row coloring precedence

### Story: Root user dominates all other tints

**Given:** an event whose `userIdentity.type = Root` and whose
`errorCode` is also non-empty
**When:** the row is rendered
**Then:** the row appears with the `ct-root` tint (red background,
bold light foreground), NOT the plain red foreground used for
generic errors.

### Story: Error tint applies when not root

**Given:** an event from IAM user `bob` with
`errorCode = AccessDenied`
**When:** rendered
**Then:** the entire row is painted in red foreground (`error`
status), no background highlight.

### Story: Cross-account pending tint

**Given:** an event where `recipientAccountId = 111111111111` and
`userIdentity.accountId = 222222222222`, with no error
**When:** rendered
**Then:** the row is painted in yellow foreground (`pending`
status).

### Story: Service event terminated tint

**Given:** an event with `eventType = AwsServiceEvent` from
`ec2.amazonaws.com`, no error, same account
**When:** rendered
**Then:** the row is painted in dim foreground (`terminated`
status).

### Story: Default running tint for plain success

**Given:** a successful read event from `bob` in account
`111111111111` against account `111111111111`
**When:** rendered
**Then:** the row is painted in green foreground (`running`
status).

### Story: Cross-account beats service event

**Given:** an event with `eventType = AwsServiceEvent` AND
`recipientAccountId != userIdentity.accountId`
**When:** rendered
**Then:** the row uses the `pending` (yellow) tint, not
`terminated`.

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=Username,AttributeValue=Root
```

Expected fields visible: row tint = red background, ACTOR =
`ROOT` bold red.

---

## 4. ACTOR rendering for each userIdentity type

### Story: IAM user actor

**Given:** `userIdentity.type = IAMUser`, `userName = bob`
**When:** rendered
**Then:** ACTOR shows `bob`.

### Story: Root actor

**Given:** `userIdentity.type = Root`
**When:** rendered
**Then:** ACTOR shows `ROOT` in bold red, regardless of any row
tint.

### Story: Assumed service role actor

**Given:** `userIdentity.type = AssumedRole`,
`sessionContext.sessionIssuer.userName = KarpenterNodeRole`,
`principalId` ending `:k-1759`
**When:** rendered
**Then:** ACTOR shows `KarpenterNodeRole/k-1759`.

### Story: SSO user actor

**Given:** `userIdentity.type = AssumedRole` from an
AWS-SSO-issued role with session name `alice@corp` and a
permission set name `AdminAccess`
**When:** rendered
**Then:** ACTOR shows `sso:alice@corp (AdminAcc)`, possibly
middle-elided when the column width is exceeded.

### Story: AWS service principal actor

**Given:** `userIdentity.type = AWSService`,
`invokedBy = ec2.amazonaws.com`
**When:** rendered
**Then:** ACTOR shows `ec2.amazonaws.com` in dim foreground.

### Story: Cross-account principal actor

**Given:** `userIdentity.accountId = 222222222222` while the
trail belongs to `111111111111`
**When:** rendered
**Then:** ACTOR is painted yellow and includes the foreign
account ID, e.g. `vpce-0fab12/account-222222`.

### Story: Federated SAML actor

**Given:** `userIdentity.type = FederatedUser` with
`userName = saml/dave`
**When:** rendered
**Then:** ACTOR shows `federated:saml/dave`.

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=Username,AttributeValue=bob
```

Expected fields visible: ACTOR.

---

## 5. ORIGIN parsing from user-agent

### Story: Console origin

**Given:** `userAgent` starts with
`AWS Internal` or `signin.amazonaws.com`
**When:** rendered
**Then:** ORIGIN shows `Console`, painted with the accent color.

### Story: CLI origin

**Given:** `userAgent` starts with `aws-cli/`
**When:** rendered
**Then:** ORIGIN shows `CLI`.

### Story: SDK origin

**Given:** `userAgent` is `aws-sdk-go/1.44.0` or
`Boto3/1.28.0 Python/3.11`
**When:** rendered
**Then:** ORIGIN shows `SDK` (Boto/Python collapses into the same
SDK bucket unless the design's `Boto` token applies).

### Story: Service origin

**Given:** `userIdentity.type = AWSService`
**When:** rendered
**Then:** ORIGIN shows `Service` in dim foreground.

**AWS comparison:**

```
aws cloudtrail lookup-events
```

Expected fields visible: ORIGIN derived from each event's
`userAgent` field.

---

## 6. TARGET extraction (never blank)

### Story: EC2 instance target from resources[]

**Given:** an event with `resources = [{type: AWS::EC2::Instance,
ARN: arn:aws:ec2:us-east-1:111111111111:instance/i-0f1e2d3c4b5a69788}]`
**When:** rendered
**Then:** TARGET shows `ec2/i-0f1e2d3c4b5a69788`.

### Story: S3 bucket target

**Given:** a `PutObject` event with bucket `prod-logs`
**When:** rendered
**Then:** TARGET shows `s3/prod-logs/2026/04/07/…` (object key
appended where present, middle-elided to fit).

### Story: IAM role target

**Given:** an `AttachRolePolicy` event with role
`ops-runner`
**When:** rendered
**Then:** TARGET shows `iam/role/ops-runner`.

### Story: Lambda function target

**Given:** an `UpdateFunctionCode` event with function
`api-prod`
**When:** rendered
**Then:** TARGET shows `lambda/api-prod`.

### Story: RDS instance target

**Given:** a `ModifyDBInstance` event with DB instance
`billing-primary` in account `111111111111`
**When:** rendered
**Then:** TARGET shows `rds/billing-primary`.

### Story: KMS key target

**Given:** a `Decrypt` event referencing key
`arn:aws:kms:us-east-1:111111111111:key/abcd-1234`
**When:** rendered
**Then:** TARGET shows `kms/abcd-1234`.

### Story: Secrets Manager secret target

**Given:** a `GetSecretValue` event for secret `prod/db/password`
**When:** rendered
**Then:** TARGET shows `secrets/prod/db/password`.

### Story: CloudFormation stack target

**Given:** a `DescribeStackResources` event for stack
`billing-pipeline`
**When:** rendered
**Then:** TARGET shows `cfn/billing-pipeline`.

### Story: VPC security group target

**Given:** a `DescribeSecurityGroups` event for
`sg-08feab23`
**When:** rendered
**Then:** TARGET shows `vpc/sg-08feab23`.

### Story: Insight target fallback

**Given:** an Insight event `ApiCallRateInsight` with baseline
average 10 and insight average 42
**When:** rendered
**Then:** TARGET shows `ApiCallRateInsight ×4.2`.

### Story: NetworkActivity target fallback

**Given:** a NetworkActivity event with
`vpcEndpointId = vpce-0fab12` and `eventSource = s3.amazonaws.com`
**When:** rendered
**Then:** TARGET shows `vpce-0fab12 → s3`.

### Story: AwsServiceEvent target fallback

**Given:** a service-emitted event from
`kms.amazonaws.com`
**When:** rendered
**Then:** TARGET shows `kms.amazonaws.com`.

### Story: Management event without resources renders (none)

**Given:** a `ListBuckets` event with no `resources[]` and no
fallback applies
**When:** rendered
**Then:** TARGET shows `(none)`. TARGET is never the empty
string.

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceName,AttributeValue=i-0f1e2d3c4b5a69788
```

Expected fields visible: TARGET.

---

## 7. OUTCOME column

### Story: Successful event shows OK in green

**Given:** an event with no `errorCode`
**When:** rendered
**Then:** OUTCOME shows `OK` in dim green.

### Story: Failed event shows error code in bold red

**Given:** an event with `errorCode = AccessDenied`
**When:** rendered
**Then:** OUTCOME shows `FAILED AccessDenied` (or end-elided
`FAILED AccessD…` at narrow widths) in bold red.

### Story: Throttling errors render with their code

**Given:** an event with `errorCode = Throttling`
**When:** rendered
**Then:** OUTCOME shows `FAILED Throttling` in bold red.

### Story: UnauthorizedOperation renders in bold red

**Given:** a `CreateFleet` event with
`errorCode = UnauthorizedOperation`
**When:** rendered
**Then:** OUTCOME shows `FAILED Unautho…` in bold red and the
row tint is `error`.

### Story: Insight transitions render in yellow

**Given:** an Insight event whose state is `Start`
**When:** rendered
**Then:** OUTCOME shows `START` in yellow.

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=ReadOnly,AttributeValue=false
```

Expected fields visible: OUTCOME.

---

## 8. Default sort

### Story: Newest first by default

**Given:** the user opens `:ct-events` for the first time in a
session
**When:** the page loads
**Then:** rows are ordered TIME descending — the newest event is
at row 1, the oldest is at the bottom.

### Story: Sort overrides the generic name-asc default

**Given:** the user has just navigated from EC2 (which sorts
name-asc) to ct-events
**When:** ct-events opens
**Then:** the active sort is TIME desc, NOT name asc, despite
ct-events being a regular paginated list.

### Story: `s` cycles through TIME, EVENT, ACTOR, OUTCOME, TARGET

**Given:** the cursor is on the list and the current sort is
TIME desc
**When:** the user presses `s` repeatedly
**Then:** the sort cycles in order: TIME desc → EVENT asc →
ACTOR asc → OUTCOME asc → TARGET asc → back to TIME desc.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 25
```

The CloudTrail API returns events in reverse chronological
order; a9s preserves this ordering for the default view.

---

## 9. Filter (`/`)

### Story: Filter is a plain substring matcher

**Given:** the user presses `/` and types `FAILED`
**When:** the filter is applied
**Then:** only rows whose OUTCOME contains `FAILED` remain
visible. The frame title shows `[N of 114, filter: FAILED]`.

### Story: Filter matches across visible columns

**Given:** the user types `/i-0f1e2d3c`
**When:** filter applies
**Then:** rows whose TARGET contains the EC2 instance ID are
shown, regardless of column.

### Story: Filter matches actor substring

**Given:** the user types `/bob`
**When:** filter applies
**Then:** all rows whose ACTOR contains `bob` are shown.

### Story: No sigils, no shortcuts

**Given:** the user types `/root` or `/error`
**When:** filter applies
**Then:** the matcher treats these as literal substrings — they
match rows whose ACTOR contains `root` or whose OUTCOME contains
`error` text. There is NO `/root` or `/error` shortcut for status
tints.

### Story: Esc clears the filter

**Given:** an active filter `bob`
**When:** the user presses `esc`
**Then:** the filter is cleared and all loaded events are
visible again.

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=Username,AttributeValue=bob
```

Note: a9s filter is client-side over the loaded page; for
authoritative principal pivots use the detail view's right
column.

---

## 10. Navigation — enter opens detail

### Story: Enter opens the detail view for the highlighted row

**Given:** the cursor is on a row whose event is
`TerminateInstances` against `i-0f1e2d3c4b5a69788`
**When:** the user presses `enter`
**Then:** the CloudTrail event detail view opens for that
specific event (per `#245` design), with the same time, actor,
and event name shown in the list row.

### Story: Esc returns to the list with cursor preserved

**Given:** the user is in the detail view of an event opened
from row 7
**When:** the user presses `esc`
**Then:** the list view is restored with the cursor still on
row 7.

**AWS comparison:**

```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=EventName,AttributeValue=TerminateInstances
```

---

## 11. Help screen additions

### Story: Help shows the verb-glyph legend

**Given:** the user is on the ct-events list view
**When:** the user presses `?`
**Then:** the help view opens and includes a "CloudTrail Events
legend" section listing each glyph (`R`, `W`, `D`, `S`, `I`, `N`,
`?`) with its meaning, style, and color, exactly as in §8a of
the design.

### Story: Help shows the row status colors

**Given:** the user opens help from the ct-events list
**When:** the legend renders
**Then:** the row-status table is shown: red bg = root, red fg =
error, yellow fg = cross-account, dim fg = service, green fg =
default success.

### Story: Help shows the actor and outcome cell colors

**Given:** the user opens help from ct-events
**When:** the legend renders
**Then:** the actor/outcome subsection lists ROOT (bold red),
cross-account (yellow), service (dim), OK (green), FAILED (bold
red), START/END (yellow).

### Story: Legend is scoped to ct-events

**Given:** the user opens help from any non-ct-events list view
(e.g. EC2)
**When:** help renders
**Then:** the CloudTrail legend section is NOT shown.

### Story: Any key closes help

**Given:** the help view is open with the legend visible
**When:** the user presses any key
**Then:** help closes and the ct-events list is restored.

---

## 12. Key bindings and bottom border

### Story: Bottom border lists the standard keys

**Given:** the ct-events list view is open and unfiltered
**When:** the frame is rendered
**Then:** the bottom border shows
`/filter  s sort  tab cols  enter detail  r related  esc back  ? help`
in the standard a9s hint format.

### Story: No duplicated helper bars

**Given:** the list is rendered
**When:** inspected
**Then:** the key hint line appears exactly once — embedded in
the bottom border — and there is no separate footer bar.

### Story: Esc exits the list view

**Given:** the user is on the ct-events list with no active
filter
**When:** the user presses `esc`
**Then:** the view returns to the main menu.

### Story: Filter mode replaces hints with filter prompt

**Given:** the user has typed `/FAILED`
**When:** the filter is active
**Then:** the bottom border shows
`filter: FAILED ── enter to clear` instead of the key-hint line,
mirroring the standard a9s filter rendering.

---

## 13. Narrow terminal column drop priority

### Story: SRC IP and REGION drop first

**Given:** the user resizes the terminal to 145 columns
**When:** the list re-renders
**Then:** SRC IP is hidden (was off by default anyway) and
REGION is hidden when the page is single-region.

### Story: ORIGIN drops below 115 columns

**Given:** the terminal is resized to 110 columns
**When:** the list re-renders
**Then:** the ORIGIN column is dropped from the layout. The
verb-glyph, TIME, ACTOR, EVENT, TARGET, OUTCOME columns remain.

### Story: TARGET and ACTOR truncate before EVENT

**Given:** the terminal is resized to 95 columns
**When:** the list re-renders
**Then:** TARGET truncates to ~16 chars and ACTOR to ~14 chars,
but EVENT remains at least 14 chars wide.

### Story: TIME collapses to HH:MM:SS at 80 columns

**Given:** the terminal is 80 columns wide
**When:** the list re-renders
**Then:** TIME shows just `14:31:12` (no date), and the visible
columns are verb-glyph, TIME, ACTOR (truncated), EVENT
(truncated), TARGET (truncated), OUTCOME.

### Story: At 60 columns only verb, TIME, EVENT, OUTCOME survive

**Given:** the terminal is resized below 60 columns
**When:** the list re-renders
**Then:** only the verb glyph, TIME (HH:MM:SS), EVENT, and
OUTCOME columns remain visible. ACTOR, ORIGIN, TARGET, REGION,
SRC IP are all hidden.

### Story: Verb glyph and OUTCOME never drop

**Given:** any terminal width above 60 columns
**When:** the list renders
**Then:** the verb glyph (column 1) and OUTCOME (rightmost) are
always visible.

---

## 14. Empty state

### Story: No events in the selected time range

**Given:** the configured CloudTrail lookup window contains zero
events
**When:** the list view loads
**Then:** the frame title shows `ct-events [0]`, the column
header row is rendered, and the body shows a single centered
message such as `no events in selected range` (matching the
existing a9s empty-list convention). The bottom border still
shows the standard key hints.

### Story: Empty state still allows esc to exit

**Given:** the empty-state message is visible
**When:** the user presses `esc`
**Then:** the view returns to the main menu without error.

**AWS comparison:**

```
aws cloudtrail lookup-events --start-time 2030-01-01T00:00:00Z --end-time 2030-01-01T00:01:00Z
```

Expected: empty `Events: []` response.
