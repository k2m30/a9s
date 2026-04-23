---
shortName: ses
generatedFrom:
  - docs/resources/ses.md
---

# ses — Implementation Plan

Companion to `docs/resources/ses.md`. Section 1 is the pseudocode test spec (QA's contract). Section 2 is the fixture list (coder's contract). Section 3 is the contract-surface gap analysis (coder's delta).

## 1. Pseudocode test spec (QA authority)

Every case below maps to one row in spec §3 / §4 or to a universal-coverage-matrix invariant. QA turns each into a Go test in `tests/unit/aws_ses_*_test.go`.

### 1.1 Fetcher cases (map AWS → resource.Resource)

```text
TEST: ses_healthy_verified_sending_enabled
GIVEN: a SESv2 IdentityInfo with VerificationStatus=SUCCESS, SendingEnabled=true
WHEN:  FetchSESIdentitiesPage runs on the fixture
THEN:
  - got.Status == ""                        (Healthy silence, §4 rule 1)
  - got.Issues is nil or empty              (Healthy has no Wave-1 phrases)
  - got.Fields["verification_status"] == "SUCCESS"
  - got.Fields["sending_enabled"] == "true"

TEST: ses_pending_verification
GIVEN: VerificationStatus=PENDING, SendingEnabled=true
WHEN:  FetchSESIdentitiesPage runs
THEN:
  - got.Status == "pending verification"    (spec §4)
  - got.Issues == ["pending verification"]

TEST: ses_verification_failed
GIVEN: VerificationStatus=FAILED
WHEN:  FetchSESIdentitiesPage runs
THEN:
  - got.Status == "verification failed"
  - got.Issues == ["verification failed"]

TEST: ses_verification_temp_failure
GIVEN: VerificationStatus=TEMPORARY_FAILURE
THEN:
  - got.Status == "verify: temp failure"
  - got.Issues == ["verify: temp failure"]

TEST: ses_verification_not_started
GIVEN: VerificationStatus=NOT_STARTED
THEN:
  - got.Status == "verification not started"
  - got.Issues == ["verification not started"]

TEST: ses_sending_disabled_on_verified_identity
GIVEN: VerificationStatus=SUCCESS, SendingEnabled=false
THEN:
  - got.Status == "sending disabled"       (Wave 1 Warning — spec §3.1, §6 devops note)
  - got.Issues == ["sending disabled"]

TEST: ses_verification_failed_beats_sending_disabled    (U7a multi-W1)
GIVEN: VerificationStatus=FAILED AND SendingEnabled=false
WHEN:  FetchSESIdentitiesPage runs
THEN:
  - got.Status == "verification failed (+1)"  (Broken bucket wins precedence; (+N-1)=+1 for the hidden warning)
  - got.Issues == ["verification failed", "sending disabled"]   (U7e/U7f: both phrases, broken-first)

TEST: fetcher_populates_resource_issues   (covers U7f over all fetcher fixtures)
GIVEN: each §3.1 fixture case above
THEN:  got.Issues deep-equals the expected ordered slice
```

### 1.2 Wave 2 enricher (GetAccount) cases

Wave 2 findings are account-wide. The enricher returns ONE finding keyed by the synthetic `"account"` id; the coder's enricher implementation replicates that finding onto every identity row via `FieldUpdates` (spec §4 "Account-wide Wave 2 findings apply to the account, not any single identity").

```text
TEST: enricher_account_healthy_nothing
GIVEN: GetAccount returns EnforcementStatus=HEALTHY, SendingEnabled=true,
       SendQuota.Max24HourSend=50000, SendQuota.SentLast24Hours=1000
WHEN:  EnrichSESAccount runs over a slice of Healthy identities
THEN:
  - result.Findings is empty
  - result.IssueCount == 0
  - no FieldUpdates produced

TEST: enricher_account_probation_all_healthy_rows
GIVEN: GetAccount returns EnforcementStatus=PROBATION; identity slice = 3 Healthy rows
WHEN:  EnrichSESAccount runs
THEN:
  - for each identity i in slice: result.Findings[i.ID].Severity == "!"
                                  result.Findings[i.ID].Summary == "account PROBATION"
                                  (spec §4 — short S4 phrase ≈1–4 words)
  - result.Findings[<id>].Rows contains {Label: "Enforcement Status", Value: "PROBATION"}
  - Summary does not contain any Row.Value string (enrichment contract U11)
  - result.IssueCount == 1  (counted once per account per spec §4 "S1 counts the account-level finding once")
  - result.FieldUpdates[id]["status"] == "account PROBATION" for every Healthy id
                                         (prepend account-scope prefix; Healthy rows had blank Status so no suffix bumping)

TEST: enricher_account_shutdown
GIVEN: GetAccount returns EnforcementStatus=SHUTDOWN; slice of 2 Healthy identities
THEN:
  - Findings[i].Severity == "!"
  - Findings[i].Summary == "account SHUTDOWN"
  - FieldUpdates[id]["status"] == "account SHUTDOWN" on Healthy rows
  - IssueCount == 1

TEST: enricher_quota_over_threshold
GIVEN: GetAccount returns EnforcementStatus=HEALTHY, SendQuota.Max24HourSend=10000,
       SendQuota.SentLast24Hours=8500  (85%, > 80%)
WHEN:  EnrichSESAccount runs over 2 Healthy identities
THEN:
  - Findings[i].Severity == "~"
  - Findings[i].Summary == "quota 80%+ used"
  - Rows include {Label: "Sent Last 24h", Value: "8500"} AND {Label: "Max 24h Send", Value: "10000"}
  - IssueCount == 0                         (~ does not bump S1)
  - FieldUpdates[id]["status"] == "quota 80%+ used" on Healthy rows

TEST: enricher_quota_at_exactly_80_percent_does_not_fire
GIVEN: SendQuota.Max24HourSend=10000, SendQuota.SentLast24Hours=8000  (exactly 80%)
THEN:
  - no quota finding produced (strict "> 0.8 ×")

TEST: enricher_probation_wins_over_quota
GIVEN: EnforcementStatus=PROBATION AND SendQuota.SentLast24Hours/Max24HourSend > 0.8
WHEN:  EnrichSESAccount runs
THEN:
  - Findings[i].Summary == "account PROBATION"  (Broken severity beats Warning)
  - IssueCount == 1                             (! bucket)
  - (quota finding is dropped — only one finding per account per spec §4 precedence)

TEST: enricher_w1_plus_w2_bumps_suffix   (U7b)
GIVEN: EnforcementStatus=PROBATION AND the identity slice contains a row whose
       fetcher-produced Status is "verification failed" (non-Healthy)
WHEN:  EnrichSESAccount runs
THEN:
  - FieldUpdates[<verify-failed id>]["status"] == "verification failed (+1)"
         (Wave-1 wins severity; (+N) bumped via resource.BumpFindingSuffix)
  - FieldUpdates[<healthy id>]["status"] == "account PROBATION"
  - Findings[<verify-failed id>].Severity == "!"  (so it surfaces in S5 detail)

TEST: enricher_handles_getaccount_error
GIVEN: GetAccount returns error
THEN:
  - result.Findings is empty (nil map accepted)
  - err != nil so caller sees it
  - no panic

TEST: enricher_handles_nil_client
GIVEN: clients.SESv2 == nil
THEN:
  - result.Findings is empty
  - err == nil  (skip gracefully)

TEST: enricher_summary_not_contain_rows   (U11 — contract)
GIVEN: any finding produced by EnrichSESAccount
THEN:  for every Row in finding.Rows: !strings.Contains(finding.Summary, row.Value)
```

### 1.3 Related-checker cases

```text
TEST: checkSESR53_domain_identity_matches_zone
GIVEN: identity "acme-corp.com" (DOMAIN), r53 cache has zone "acme-corp.com."
WHEN:  checkSESR53 runs
THEN:
  - result.Count == 1

TEST: checkSESR53_email_identity_extracts_domain
GIVEN: identity "noreply@acme-corp.com" (EMAIL_ADDRESS), r53 cache has "acme-corp.com."
THEN:
  - result.Count == 1

TEST: checkSESR53_subdomain_matches_parent_zone
GIVEN: identity "mail.acme-corp.com" (DOMAIN), zone "acme-corp.com."
THEN:
  - result.Count == 1   (suffix match)

TEST: checkSESR53_no_match
GIVEN: identity "unrelated.example.com", zone cache only has acme-corp.com
THEN:
  - result.Count == 0

TEST: checkSESR53_empty_identity
GIVEN: res.ID == ""
THEN:
  - result.Count == 0   (no error)

TEST: checkSESEbRule_eventbridge_destination
GIVEN: GetEmailIdentity returns ConfigurationSetName="es-evt",
       GetConfigurationSetEventDestinations returns one dest with EventBridgeDestination.EventBusArn = "arn:aws:events:...:bus/default"
THEN:
  - result.Count == 1
  - result.IDs contains the EventBusArn

TEST: checkSESEbRule_no_config_set
GIVEN: GetEmailIdentity returns ConfigurationSetName=nil
THEN:
  - result.Count == 0

TEST: checkSESKinesis_firehose_destination
GIVEN: one event destination with KinesisFirehoseDestination.DeliveryStreamArn set
THEN:
  - result.Count == 1

TEST: checkSESSns_topic_destination
GIVEN: one event destination with SnsDestination.TopicArn set
THEN:
  - result.Count == 1

TEST: checkSESLambda_receipt_rule_lambda_action
GIVEN: SES v1 DescribeActiveReceiptRuleSet returns one rule with Actions[].LambdaAction.FunctionArn set
THEN:
  - result.Count == 1

TEST: checkSESLambda_no_active_rule_set
GIVEN: DescribeActiveReceiptRuleSet returns nil rule set (pure outbound account)
THEN:
  - result.Count == 0   (operator-honest absence, no error)

TEST: checkSESS3_receipt_rule_s3_action
GIVEN: SES v1 DescribeActiveReceiptRuleSet returns one rule with Actions[].S3Action.BucketName set
THEN:
  - result.Count == 1

TEST: checkSESS3_no_active_rule_set
THEN:
  - result.Count == 0
```

### 1.4 Anti-tests (§3.3 Wave 3 out-of-scope)

```text
TEST: wave3_dkim_drift_not_surfaced
GIVEN: fixture with DKIM attributes set; fetcher path runs
THEN:
  - resource.Status does not contain "dkim"
  - resource.Issues does not contain "dkim"

TEST: wave3_reputation_bouncerate_not_surfaced
GIVEN: fixture does not call CloudWatch for bounce/complaint rates
THEN:
  - no CloudWatch GetMetricData in fetcher or enricher code path
  - no "bounce", "complaint", "reputation" phrase in Status/Issues
```

### 1.5 Silence test (U1)

```text
TEST: ses_silence_healthy_happy_path
GIVEN: a verified SUCCESS identity, sending enabled, account HEALTHY, quota 5% used
WHEN:  fetcher + enricher run end-to-end
THEN:
  - row color = green, S4 blank, no glyph, no S5 Attention section
```

## 2. Fixture list (coder authority for Phase 6a)

Single file: `internal/demo/fixtures/ses.go`. All fixtures align on `acme-corp.com` so the r53 pivot resolves against the existing `NewR53Fixtures()` zones (which use `acme-corp.com.`, not `acmecorp.com`).

### 2.1 Identity fixtures (one per §3.1 signal + graph-root + multi-W1)

```text
FIXTURE: healthy-domain
A verified domain identity, sending enabled, no glyph expected on Healthy path.
IdentityName = "acme-corp.com"
IdentityType = DOMAIN
VerificationStatus = SUCCESS
SendingEnabled = true

FIXTURE: healthy-email
Verified transactional sender, Healthy.
IdentityName = "noreply@acme-corp.com"
IdentityType = EMAIL_ADDRESS
VerificationStatus = SUCCESS
SendingEnabled = true

FIXTURE: warn-pending-email         (covers §3.1 PENDING)
A pending-verification email identity. Yellow row, S4 "pending verification".
IdentityName = "alerts@acme-corp.com"
IdentityType = EMAIL_ADDRESS
VerificationStatus = PENDING
SendingEnabled = true

FIXTURE: broken-failed-domain       (covers §3.1 FAILED)
Hard DNS failure. Red row, S4 "verification failed".
IdentityName = "ses-failed.acme-corp.com"
IdentityType = DOMAIN
VerificationStatus = FAILED
SendingEnabled = true

FIXTURE: broken-temp-failure-domain  (covers §3.1 TEMPORARY_FAILURE)
Red row, S4 "verify: temp failure".
IdentityName = "temp.acme-corp.com"
IdentityType = DOMAIN
VerificationStatus = TEMPORARY_FAILURE
SendingEnabled = true

FIXTURE: broken-not-started-domain   (covers §3.1 NOT_STARTED)
Red row, S4 "verification not started".
IdentityName = "notstarted.acme-corp.com"
IdentityType = DOMAIN
VerificationStatus = NOT_STARTED
SendingEnabled = true

FIXTURE: warn-sending-disabled      (covers §3.1 SendingEnabled==false on verified)
Verified but sending paused. Yellow, S4 "sending disabled".
IdentityName = "suppressed@acme-corp.com"
IdentityType = EMAIL_ADDRESS
VerificationStatus = SUCCESS
SendingEnabled = false

FIXTURE: warn-ses-multi             (U7a: multi-W1 suffix test vehicle)
A domain that is both FAILED verification AND has sending disabled.
At list render time the fetcher produces "verification failed (+1)"
with Issues = ["verification failed", "sending disabled"].
IdentityName = "broken.acme-corp.com"
IdentityType = DOMAIN
VerificationStatus = FAILED
SendingEnabled = false

FIXTURE: graph-root-mailer          (Phase 9.3 — all §2 pivots non-zero)
A verified domain identity with full event-destination wiring to every §2 target:
  ConfigurationSetName = "es-events-prod" (resolved via GetEmailIdentity)
  EventDestinations (resolved via GetConfigurationSetEventDestinations):
    - EventBridgeDestination { EventBusArn = <eb-rule graph-root default bus ARN> }
    - KinesisFirehoseDestination { DeliveryStreamArn = <kinesis fixture firehose ARN> }
    - SnsDestination { TopicArn = <sns fixture "ses-bounces" topic ARN> }
  SES v1 active receipt rule set contains:
    - LambdaAction { FunctionArn = <lambda fixture "acme-inbound-parser" ARN> }
    - S3Action     { BucketName  = <s3 fixture "acme-inbound-mail" bucket name> }
  r53 hosted zone match: "acme-corp.com." (already present in NewR53Fixtures)
  ct-events: universal pivot, resolved windowed via CloudTrail fixture — already covered by existing ct-events fixture
IdentityName = "acme-corp.com"   (same as healthy-domain — this IS the graph root)
IdentityType = DOMAIN
VerificationStatus = SUCCESS
SendingEnabled = true

NOTE: healthy-domain and graph-root-mailer collapse to the same IdentityInfo entry;
the `graph-root-mailer` role is played by the healthy-domain fixture once sibling
wiring is added. No duplicate row; just one verified Healthy row that happens to
resolve every pivot.
```

### 2.2 Account-level (GetAccount) fixtures

`GetAccount` is a single API call, so the fake returns one of four scripted shapes. The demo fake uses the healthy shape by default; unit tests inject the others inline.

```text
DEFAULT (used by ./a9s --demo): GetAccount returns
  EnforcementStatus = "HEALTHY"
  SendingEnabled    = true
  SendQuota.Max24HourSend    = 50000
  SendQuota.SentLast24Hours  = 1200   (~2.4% — well below 80% threshold)

INLINE-ONLY (test files, not demo):
  - probation shape (EnforcementStatus = PROBATION)
  - shutdown shape  (EnforcementStatus = SHUTDOWN)
  - over-quota shape (SentLast24Hours = 8500, Max = 10000 → 85%)
  - boundary shape  (SentLast24Hours = 8000, Max = 10000 → exactly 80%, must NOT fire)

RATIONALE: account-level distress states are genuinely out-of-band in the showroom
(SHUTDOWN renders every identity as red account-PROBATION/SHUTDOWN which dominates
the demo UX). Demo-default = HEALTHY; the unit/integration tests inject per-case.
```

### 2.3 SES v1 receipt-rule fixtures

SES v1 has a single active rule set per account. The demo fake returns one rule set
that contains both the Lambda and S3 actions needed to satisfy Phase 9.3.

```text
DEFAULT (./a9s --demo): DescribeActiveReceiptRuleSet returns
  Metadata.Name = "acme-inbound-prod"
  Rules[0] = {
    Name = "route-support-to-lambda-and-s3"
    Recipients = ["support@acme-corp.com", "invoices@acme-corp.com"]
    Actions = [
      S3Action     { BucketName = <existing s3 fixture "acme-inbound-mail" bucket name> }
      LambdaAction { FunctionArn = <existing lambda fixture "acme-inbound-parser" ARN> }
    ]
  }

If the referenced Lambda/S3 fixture IDs are missing in sibling files, the fixture
coder adds them during phase 6a graph-wiring (not this file — sibling files).
```

### 2.4 Sibling-fixture updates (phase 6a graph plan)

The fixture coder cross-references the following sibling files and appends entries so every §2 pivot resolves ≥ 1 for the graph-root:

```text
internal/demo/fixtures/lambda.go     — add "acme-inbound-parser" function
internal/demo/fixtures/s3.go         — add "acme-inbound-mail" bucket
internal/demo/fixtures/sns.go        — add "ses-bounces" topic (ARN referenced from ses fixture)
internal/demo/fixtures/kinesis.go    — add firehose delivery stream "ses-event-stream" if not present
internal/demo/fixtures/eventbridge.go — confirm default bus fixture present; if not, add
internal/demo/fixtures/r53.go        — "acme-corp.com." already present, no change
internal/demo/fixtures/ses.go        — this resource's own fixtures
```

All ids / ARNs are produced by the fixture coder using the project's existing
accountID convention (123456789012 for demo).

### 2.5 Adversarial fixtures (inline in tests, NOT demo)

These stay inline in the test files per skill rule:

```text
- GetAccount returns error                           → enricher_handles_getaccount_error
- GetAccount nil client                              → enricher_handles_nil_client
- ListEmailIdentities returns nil Resources slice    → fetcher returns empty
- Identity with IdentityName == nil                  → fetcher skips / treats blank
- GetConfigurationSetEventDestinations returns error → checker returns Count=-1
```

## 3. Contract-surface gap analysis

Delta between what spec §2/§3/§4 require and what the four contract-surface files currently provide. Phase 7 coder closes these.

### 3.1 Views config (.a9s/views/ses.yaml + internal/config/defaults_backup.go)

CURRENT:
```yaml
list:
  Identity: {path: IdentityName, width: 36}
  Type: {path: IdentityType, width: 16}
  Verification: {path: VerificationStatus, width: 16}   ← jargon column
  Sending: {path: SendingEnabled, width: 10}            ← jargon column
```

REQUIRED (universal rule: exactly one Status column, no jargon):
```yaml
list:
  Identity: {path: IdentityName, width: 36}
  Type: {path: IdentityType, width: 12}
  Status: {key: status, width: 36}     ← holds §4 phrases; backed by Resource.Status
```

COLLAPSE: `Verification` + `Sending` → `Status` (`status` key). Both are jargon flags; their data is encoded in §4 phrases produced by the fetcher. `Type` stays (pure identity metadata — DOMAIN vs EMAIL_ADDRESS is operator-useful at 3am).

### 3.2 Fetcher (internal/aws/ses.go)

CURRENT: `Status = verificationStatus` (raw enum "SUCCESS" / "FAILED" etc). No `Issues` populated.

REQUIRED:
- Implement `computeSESStatusAndIssues(identity) (topPhrase string, issues []string)` that maps Wave-1 signals per §4 precedence:
    1. `VerificationStatus==FAILED`           → `"verification failed"` (Broken)
    2. `VerificationStatus==TEMPORARY_FAILURE`→ `"verify: temp failure"` (Broken)
    3. `VerificationStatus==NOT_STARTED`      → `"verification not started"` (Broken)
    4. `VerificationStatus==PENDING`          → `"pending verification"` (Warning)
    5. `SendingEnabled==false` (on verified SUCCESS or on any row already flagged above) → append `"sending disabled"`
    6. Healthy (SUCCESS + enabled)            → `""`, nil slice
- Set `Resource.Status` = top phrase + `(+N-1)` if `len(issues)>=2`.
- Set `Resource.Issues` = full ordered slice (U7f).
- Note: `SendingEnabled==false` combined with `VerificationStatus==PENDING` — the spec §3.1 says the more severe bucket wins. PENDING is Warning, sending-disabled is also Warning → still Warning; either ordering is defensible, pick "pending verification (+1)" with "sending disabled" in Issues (follows precedence table).
- Note: when `VerificationStatus != SUCCESS`, `SendingEnabled==false` is redundant noise (the identity can't send anyway). The spec is explicit that the more-severe bucket wins; the Broken phrase is top; sending-disabled stacks in Issues.

### 3.3 Enricher (internal/aws/ses_issue_enrichment.go)

CURRENT: registers one "account" key; handles SHUTDOWN (!), PROBATION (~), sending-disabled (~). Wrong severities, wrong Summary strings, misses quota, produces one finding keyed "account" instead of replicating per-identity.

REQUIRED:
- Call `GetAccount` once (unchanged).
- Decide single top finding (account-level) using §4 precedence:
    - SHUTDOWN → severity `!`, Summary `"account SHUTDOWN"`.
    - PROBATION → severity `!`, Summary `"account PROBATION"`.
    - quota > 80% → severity `~`, Summary `"quota 80%+ used"`.
    - otherwise: no finding.
- Replicate the one finding onto every identity row in the input slice: for each `res` in `resources`, set `findings[res.ID] = <finding>` with the SAME Summary (Rows may vary; see contract below).
- Set `FieldUpdates[id]["status"]` for every row:
    - If `res.Status == ""` (Healthy): set to `finding.Summary` verbatim (no suffix).
    - If `res.Status != ""` (non-Healthy, already has Wave-1 phrase): bump via `resource.BumpFindingSuffix(res.Status)` — Wave-1 keeps precedence.
- `IssueCount`: 1 if finding severity is `!`, else 0. Counted ONCE for the whole account (spec §4 "S1 counts the account-level finding once, not N times").
- `SendingEnabled==false` is NO LONGER a Wave-2 signal here. Move to Wave 1 in fetcher per spec §3.1. Delete the current `!exists && !out.SendingEnabled` branch.
- Enrichment contract (U11): Summary is the short phrase only; per-identity context rides in Rows (Enforcement Status / Sent Last 24h / Max 24h Send). `strings.Contains(Summary, rowValue)` must be false for every Row.

### 3.4 Related checkers (internal/aws/ses_related.go)

CURRENT: r53, eb-rule, kinesis, sns, lambda (stub), s3 (stub). Most are fine. Gap: lambda and s3 currently return 0 unconditionally.

REQUIRED:
- Replace `checkSESLambda` stub with: call `SES v1 DescribeActiveReceiptRuleSet` → walk `Rules[].Actions[].LambdaAction.FunctionArn` → return set of ARNs.
- Replace `checkSESS3` stub with: call `SES v1 DescribeActiveReceiptRuleSet` → walk `Rules[].Actions[].S3Action.BucketName` → return set of bucket names.
- Both checkers gracefully return `Count: 0` (not -1) when the active rule set is nil or empty (spec: "Accounts with no active receipt rule set render 0 — operator-honest absence").
- Cache the result per (account, region) within the ongoing fetch batch to avoid calling DescribeActiveReceiptRuleSet once per identity when the list has N identities. Implementation detail: package-level `sync.Once` per ServiceClients is acceptable; a per-call cache on a shared context is cleaner if available. Coder's call; must not be N API calls for N identities.

### 3.5 Interfaces (internal/aws/ses_interfaces.go)

REQUIRED ADDITION:
```go
type SESV1DescribeActiveReceiptRuleSetAPI interface {
    DescribeActiveReceiptRuleSet(ctx, *ses.DescribeActiveReceiptRuleSetInput, ...func(*ses.Options))
        (*ses.DescribeActiveReceiptRuleSetOutput, error)
}
type SESV1API interface { SESV1DescribeActiveReceiptRuleSetAPI }
```
Use package path `github.com/aws/aws-sdk-go-v2/service/ses` (v1).

### 3.6 ServiceClients + client factory (internal/aws/client.go)

REQUIRED ADDITION:
- New field: `SES SESV1API` on `ServiceClients`.
- `CreateServiceClients` wires `ses.NewFromConfig(cfg)`.
- Import `github.com/aws/aws-sdk-go-v2/service/ses`.

### 3.7 Demo typed fake (internal/demo/fakes/…)

REQUIRED ADDITION:
- `DescribeActiveReceiptRuleSet` method on the SESv1 fake, returning the fixture from §2.3.
- `GetAccount` method on the SESv2 fake, returning the default HEALTHY shape from §2.2 (already exists? coder confirms; if not, adds).
- `GetEmailIdentity` method returning `ConfigurationSetName = "es-events-prod"` for the graph-root identity.
- `GetConfigurationSetEventDestinations` method returning the event-destinations fixture from §2.1.

If any of these fake methods are missing, enricher/checker calls error out in demo mode; every scenario-harness expect fails.

### 3.8 Detail enrichment

Spec §2 does not demand list-shape-external detail fields. NO `ses_detail_enrichment.go` file needed.

## 4. Universal coverage matrix (mandatory Phase 0 gate)

| ID | Invariant | Fixture ID | Test name |
|----|-----------|-----------|-----------|
| U1 | Healthy blank S4 | `healthy-domain`, `healthy-email` | `ses_silence_healthy_happy_path`, `ses_healthy_verified_sending_enabled` |
| U2 | Warning/Broken §4 phrase | one per §3.1 fixture | `ses_pending_verification`, `ses_verification_failed`, `ses_verification_temp_failure`, `ses_verification_not_started`, `ses_sending_disabled_on_verified_identity` |
| U3 | `~` glyph on Healthy + Wave-2 `~` finding | `healthy-domain` + over-quota account injection | scenario test `ExpectRowNamePrefix("~ ")` |
| U4 | `!` glyph on Healthy + Wave-2 `!` finding | `healthy-domain` + PROBATION account injection | scenario test `ExpectRowNamePrefix("! ")` |
| U5 | No glyph on non-green rows | `broken-failed-domain` + PROBATION injection | scenario test `ExpectRowNoGlyphPrefix` |
| U6 | S1 badge counts `!` instances (ACCOUNT ONCE) | PROBATION/SHUTDOWN account injection over N identities | `enricher_account_probation_all_healthy_rows` (IssueCount==1), scenario test `ExpectMenuIssueCount("ses", 1)` |
| U7a | Multi-W1 `<top> (+N-1)` suffix | `warn-ses-multi` | `ses_verification_failed_beats_sending_disabled` |
| U7b | W1 + W2 stack bumps suffix | `broken-failed-domain` + PROBATION injection | `enricher_w1_plus_w2_bumps_suffix` |
| U7c | S5 lists every Wave-2 finding | same as U7b | scenario test `ExpectViewContains("PROBATION")` on detail |
| U7d | `!` beats `~` precedence | PROBATION injection | `enricher_probation_wins_over_quota` |
| U7e | S5 lists every Wave-1 phrase | `warn-ses-multi` | scenario test `ExpectViewContains("Verification failed")` + `ExpectViewContains("Sending disabled")` |
| U7f | Fetcher populates `Resource.Issues` | all fetcher fixtures | `fetcher_populates_resource_issues` |
| U8 | Broken > Warning > `~` | `warn-ses-multi` + over-quota injection | scenario test asserts Status = "verification failed (+2)" |
| U9 | Related pivots ≥ 1 on graph-root | `graph-root-mailer` (== `healthy-domain`) | scenario test `ExpectRelatedRowCountAtLeast` per pivot |
| U10 | No jargon columns | all fixtures | scenario test `ExpectViewNotContains("Verification", "Sending", "CIS", "Flags", "Policy", "Issues", "NOBKP", "UNENC")` |
| U11 | Summary ≠ Row content | every finding | `enricher_summary_not_contain_rows` |

## 5. Out-of-scope checks (spec §5)

QA writes anti-tests for §3.3 Wave 3 exclusions (§1.4). Coder ensures no code path references DKIM drift detection, CloudWatch reputation metrics, or any write operation.

## 6. Phase 9 acceptance matrix (for final report)

- 9.1 — no illegal UI surfaces beyond S1–S5. Scenario-harness render frame checked against banned-string list.
- 9.2 — fixtures: Wave-1 N=5 signals each covered; Wave-2 3 signals (via account-shape injection); multi-issue fixture = `warn-ses-multi`; plus PROBATION-over-healthy and PROBATION-over-failed for stacking.
- 9.3 — graph-root = `healthy-domain`/`acme-corp.com`. All pivots resolve ≥ 1: r53 (acme-corp.com zone), eb-rule (configured event bus), kinesis (firehose stream), lambda (receipt-rule Lambda), s3 (receipt-rule bucket), sns (SES bounce topic), ct-events (universal, windowed; exempt if `count shown: unknown`).
- 9.4 — scenario test `TestScenario_SESVisual` opens `warn-ses-multi` detail, `ExpectViewContains` every Wave-1 phrase and (with PROBATION injection) the Wave-2 Summary, pastes `scenario.currentView()` to `t.Log`.
