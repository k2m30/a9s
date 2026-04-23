---
shortName: s3
derivedFrom: docs/resources/s3.md
---

# s3 — Implementation Plan

This doc is the execution contract between QA (tests) and coder (implementation). It derives from `docs/resources/s3.md` (the golden spec) — if the two disagree, the spec wins and this plan is regenerated.

## 0. TBD resolution (phase 2)

The spec contains two `TBD` markers — both were resolved by the spec itself in §5 Out of Scope and §6 Citations (already carried as a9s-devops decisions dated 2026-04-20). No new user input required this run.

| TBD | Disposition | Citation |
|---|---|---|
| `iam-user` discovery | **out of scope** — principal attribution requires CloudTrail data-plane parsing (Wave 3) | spec §5, §6 |
| `waf` discovery (direct S3→WAF) | **out of scope** — WAF attaches via CloudFront only; already reachable via `cf` panel entry | spec §5, §6 |

Cleanup implication: phase 7 deletes `checkS3IAMUser` and `checkS3WAF` plus their `RegisterRelated` entries.

## 1. Universal coverage matrix (mandatory)

The s3 spec has **zero Wave 1 signals** and **one Wave 2 signal** (`!` severity on Healthy rows). Rule-7 suffix cases (U7a, U7b, U7c, U7d, U7e, U7f) require stacked findings that cannot occur for this resource. Each N/A is justified inline.

| ID | Invariant | Fixture | Test case |
|----|-----------|---------|-----------|
| U1 | Healthy blank S4 | `healthy-bucket` (PAB all-blocked) | `ExpectRowStatusBlank("healthy-bucket")` |
| U2 | Warning/Broken §4 phrase | N/A — no §3.1 signals | skipped (documented) |
| U3 | `~` glyph on Healthy+~ | N/A — no `~` signals | skipped |
| U4 | `!` glyph on Healthy+! | `bucket-no-pab`, `bucket-partial-pab`, `bucket-nil-pab-cfg` | `ExpectRowNamePrefix("bucket-no-pab", "! ")` for each |
| U5 | No glyph on non-green rows | N/A — no non-green rows possible | skipped |
| U6 | S1 badge counts `!`-severity instances | 3 `!` fixtures, 1 healthy, 1 OOS | `ExpectMenuIssueCount("s3", 3)` |
| U7a | Multi Wave-1 `(+N)` suffix | N/A — zero Wave-1 signals | skipped |
| U7b | W1+W2 suffix bump | N/A — zero Wave-1 signals | skipped |
| U7c | S5 lists every Wave-2 finding in full | `bucket-no-pab` | `ExpectViewContains("no public access block configuration")` on detail |
| U7d | `!` beats `~` | N/A — no `~` signals in spec | skipped |
| U7e | S5 every Wave-1 phrase | N/A — zero Wave-1 signals | skipped |
| U7f | `Resource.Issues` populated in §4 order | every `!` fixture | unit: `got.Issues` deep-equals `[]string{}` (spec has no W1 signals, so always empty) |
| U8 | Broken > Warning > `~` precedence | N/A — single severity class | skipped |
| U9 | Related pivot counts | `healthy-bucket` (graph root) | `ExpectRelatedRowCountAtLeast` for every `count shown: yes` pivot |
| U10 | No jargon columns | all fixtures | `ExpectViewNotContains("Public Access", "CIS", "Flags", "Policy", "Issues", "NOBKP", "UNENC", "PUB", "NOPROT")` |
| U11 | Summary stable, Rows carry detail | every `!` fixture | unit: `finding.Summary == "public access block incomplete"` AND `!strings.Contains(finding.Summary, rowValue)` for every Row |

## 2. Pseudocode test spec (phase 3)

One test per §4 row + the universal silence/anti-tests + the coverage-matrix demands above.

```text
TEST: healthy_bucket_silence   (U1, U6)
GIVEN: an S3 bucket with GetPublicAccessBlock returning all four flags = true
WHEN:  the list is fetched and rendered; enrichment runs
THEN:
  - row color = green (Healthy)
  - S4 Status cell = "" (blank — not "OK", "available", "-", "healthy")
  - no `!` / `~` glyph prefix on the bucket name
  - S1 menu issues:N does NOT bump for this bucket
  - Resource.Issues is empty
  - no EnrichmentFinding emitted for this bucket

TEST: bucket_no_pab_configuration   (U4, U11, spec §4 primary row)
GIVEN: GetPublicAccessBlock returns NoSuchPublicAccessBlockConfiguration API error
WHEN:  the enricher runs
THEN:
  - EnrichmentFinding.Severity == "!"
  - EnrichmentFinding.Summary  == "public access block incomplete"
  - EnrichmentFinding.Rows contains a row {Label:"Status", Value:"no public access block configuration"}
  - EnrichmentFinding.Rows contains a row {Label:"Account-level PAB", Value:"may still apply"}
  - Summary does NOT contain the substring "no public access block configuration"
  - FieldUpdates[bucket]["status"] == "public access block incomplete"
  - row stays green (Healthy — this is a background-check annotation)
  - `!` glyph prefixes the bucket name in the list

TEST: bucket_partial_pab_single_flag_false
GIVEN: GetPublicAccessBlock returns a config where BlockPublicAcls = false (others true)
WHEN:  the enricher runs
THEN:
  - EnrichmentFinding.Severity == "!"
  - EnrichmentFinding.Summary  == "public access block incomplete"
  - EnrichmentFinding.Rows contains {Label:"BlockPublicAcls", Value:"false"}
  - EnrichmentFinding.Rows contains {Label:"Account-level PAB", Value:"may still apply"}
  - Summary does NOT contain "BlockPublicAcls" or "false"
  - FieldUpdates[bucket]["status"] == "public access block incomplete"
  - row stays green, `!` glyph renders

TEST: bucket_partial_pab_multiple_flags_false
GIVEN: GetPublicAccessBlock returns BlockPublicAcls=false AND BlockPublicPolicy=false (others true)
WHEN:  the enricher runs
THEN:
  - EnrichmentFinding.Severity == "!"
  - EnrichmentFinding.Summary  == "public access block incomplete"   (same phrase — stable across instances)
  - EnrichmentFinding.Rows contains BOTH {Label:"BlockPublicAcls", Value:"false"} AND {Label:"BlockPublicPolicy", Value:"false"}
  - Summary contains neither the flag name nor "false"
  - row stays green, `!` glyph renders

TEST: bucket_nil_pab_configuration_treated_as_no_pab
GIVEN: GetPublicAccessBlock returns out.PublicAccessBlockConfiguration == nil (no API error)
WHEN:  the enricher runs
THEN:
  - same finding shape as bucket_no_pab_configuration
  - Severity `!`, Summary "public access block incomplete"
  - Rows surface the "no configuration returned" detail

TEST: enricher_handles_unknown_api_error_gracefully
GIVEN: GetPublicAccessBlock returns a non-NoSuchPublicAccessBlockConfiguration error (e.g. AccessDenied)
WHEN:  the enricher runs
THEN:
  - no EnrichmentFinding emitted for this bucket (data is incomplete, cannot claim "incomplete PAB")
  - TruncatedIDs[bucket] == true (signals enrichment incomplete for this row)
  - row stays green with blank Status (no lie)

TEST: s1_menu_badge_counts_distinct_instances   (U6)
GIVEN: 3 buckets each with a PAB issue (one no-PAB, one single-flag-false, one multi-flag-false),
       1 healthy bucket with full PAB, and 1 bucket whose GetPublicAccessBlock errored
WHEN:  the list page is rendered after enrichment
THEN:
  - menu badge text is "issues:3"
  - the 3 `!` rows each render the `!` glyph prefix on the name cell
  - the healthy row has no glyph, blank Status
  - the errored row has no glyph, blank Status (silent on incomplete data)

TEST: detail_view_attention_section_renders_all_rows   (U7c, U11)
GIVEN: the bucket_partial_pab_multiple_flags_false fixture
WHEN:  OpenDetailResource on it
THEN:
  - the detail view contains the single Attention section header "Attention (1)"
  - the primary entry line reads "! Public access block incomplete"
       (capitalizeFirst applied at render time — data stays lowercase)
  - the Attention section contains the row "BlockPublicAcls: false"
  - the Attention section contains the row "BlockPublicPolicy: false"
  - the Attention section contains the row "Account-level PAB: may still apply"

TEST: related_panel_lists_only_in_scope_pivots   (§5 out-of-scope enforcement)
GIVEN: any bucket fixture
WHEN:  the related panel renders
THEN:
  - NO "IAM Users" row (iam-user is §5 OOS)
  - NO "WAF" row (waf is §5 OOS)
  - ALL of these rows are present: athena, backup, cf, cfn, eb-rule, glue, kms, lambda, logs, r53, role, sns, sqs, trail, ct-events

TEST: related_panel_counts_non_zero_for_graph_root   (U9)
GIVEN: the healthy-bucket fixture wired as the graph root (sibling fixtures reference it)
WHEN:  OpenDetailResource on it
THEN:
  - related pivots with `count shown: yes` in spec §2 each show Count >= 1:
    athena, backup, cf, cfn, eb-rule, glue, kms, lambda, logs, r53, role, sns, sqs, trail

TEST: no_jargon_column_in_view   (U10)
GIVEN: any bucket fixture
WHEN:  the list view renders
THEN:
  - rendered frame does NOT contain the strings "Public Access", "CIS", "Flags", "Policy", "Issues", "NOBKP", "UNENC", "PUB", "NOPROT"
  - rendered frame DOES contain the column header "Status"

TEST: anti_test_wave3_oos_encryption   (§3.3 OOS guard)
GIVEN: a bucket with SSE-KMS encryption absent (in fixture)
WHEN:  the list renders
THEN:
  - no encryption-related signal surfaces anywhere (no S4 text, no S5 line, no row-color change, no finding)
  - operator is shown no encryption opinion

TEST: fetcher_resource_issues_empty   (U7f)
GIVEN: any bucket fixture
WHEN:  FetchS3Page runs
THEN:
  - got.Issues is nil or empty slice (spec has no Wave 1 signals)
```

## 3. Fixture list (phase 4)

Single-source fixture file at `internal/demo/fixtures/s3.go`. The existing file is overwritten by phase 6a. Adversarial fixtures (error-path, nil configs that come from API error paths) stay inline in the test file — not in this fixtures file.

### Fixtures

```text
FIXTURE: healthy-bucket  (graph root — every other fixture references this one for counts)
An S3 bucket fully locked down. ListBuckets returns:
  Name = "a9s-demo-healthy"
  BucketArn = "arn:aws:s3:::a9s-demo-healthy"
  CreationDate = 2024-03-10T09:00:00Z
  BucketRegion = "us-east-1"
Typed-fake GetPublicAccessBlock returns all four flags = true.
Notification config: LambdaFunctionConfigurations → arn:aws:lambda:us-east-1:<acct>:function:a9s-demo-s3-notifier ;
                    TopicConfigurations → arn:aws:sns:us-east-1:<acct>:a9s-demo-s3-events ;
                    QueueConfigurations → arn:aws:sqs:us-east-1:<acct>:a9s-demo-s3-dlq
Encryption: SSE-KMS with KMSMasterKeyID = arn:aws:kms:us-east-1:<acct>:key/a9s-demo-s3-key
Logging: LoggingEnabled.TargetBucket = "a9s-demo-logs"
Tagging: aws:cloudformation:stack-name = "a9s-demo-stack"

FIXTURE: bucket-no-pab
A bucket with no PublicAccessBlock configuration set.
  Name = "a9s-demo-nopab"
  BucketArn = "arn:aws:s3:::a9s-demo-nopab"
  CreationDate = 2024-06-01T12:00:00Z
  BucketRegion = "us-east-1"
Typed-fake GetPublicAccessBlock returns NoSuchPublicAccessBlockConfiguration smithy API error.
No notification / encryption / logging / tagging (bare bucket).

FIXTURE: bucket-partial-pab
A bucket with BlockPublicAcls = false, the other three flags = true.
  Name = "a9s-demo-partial-pab"
  BucketArn = "arn:aws:s3:::a9s-demo-partial-pab"
  CreationDate = 2024-09-15T11:30:00Z
  BucketRegion = "us-east-1"

FIXTURE: bucket-multi-false-pab
A bucket with BlockPublicAcls = false AND BlockPublicPolicy = false, other two = true.
  Name = "a9s-demo-multifail-pab"
  BucketArn = "arn:aws:s3:::a9s-demo-multifail-pab"
  CreationDate = 2025-02-20T08:15:00Z
  BucketRegion = "us-east-1"

FIXTURE: bucket-nil-pab-cfg
A bucket whose GetPublicAccessBlock returns out.PublicAccessBlockConfiguration == nil (no error).
  Name = "a9s-demo-nilcfg"
  BucketArn = "arn:aws:s3:::a9s-demo-nilcfg"
  CreationDate = 2025-05-01T14:00:00Z
  BucketRegion = "us-east-1"
```

**Coverage-matrix counts:**
- Healthy: 1 (`healthy-bucket`)
- `!` findings: 4 (`bucket-no-pab`, `bucket-partial-pab`, `bucket-multi-false-pab`, `bucket-nil-pab-cfg`)
- → **expected S1 badge = `issues:4`**

The U6 pseudocode test above states 3 `!` fixtures. Reconcile: 4 is correct for the demo fixture file. The test must assert N = 4 against this fixture set. (The earlier "3" text in U6 is illustrative from the matrix table template — the authoritative number is the fixture count.)

### Sibling-fixture graph additions (phase 6a cross-file work)

The healthy-bucket's related panel must show non-zero counts. The coder extends these sibling fixtures (does not replace them):

- `internal/demo/fixtures/lambda.go` — ensure a function named `a9s-demo-s3-notifier` exists (or add one).
- `internal/demo/fixtures/sns.go` — ensure a topic named `a9s-demo-s3-events` exists.
- `internal/demo/fixtures/sqs.go` — ensure a queue named `a9s-demo-s3-dlq` exists.
- `internal/demo/fixtures/kms.go` — ensure a key with ID `a9s-demo-s3-key` exists.
- `internal/demo/fixtures/cloudformation.go` (or wherever cfn fixtures live) — ensure a stack named `a9s-demo-stack` exists.
- `internal/demo/fixtures/cloudtrail.go` — ensure a trail with `S3BucketName = "a9s-demo-healthy"` exists.
- `internal/demo/fixtures/cloudfront.go` — ensure a distribution with an origin `a9s-demo-healthy.s3.us-east-1.amazonaws.com` exists.
- `internal/demo/fixtures/route53.go` — ensure a hosted zone with `AliasTarget.DNSName` matching `s3-website-us-east-1.amazonaws.com` AND record name containing `a9s-demo-healthy`.
- `internal/demo/fixtures/iam.go` or `role.go` — ensure a role whose policy_resources field contains `arn:aws:s3:::a9s-demo-healthy`.
- `internal/demo/fixtures/athena.go` — ensure a workgroup with `ResultConfiguration.OutputLocation = s3://a9s-demo-healthy/athena/`.
- `internal/demo/fixtures/glue.go` — ensure a Glue job with `Command.ScriptLocation = s3://a9s-demo-healthy/scripts/etl.py`.
- `internal/demo/fixtures/backup.go` — ensure a selection/recovery-point referencing `arn:aws:s3:::a9s-demo-healthy`.
- `internal/demo/fixtures/eventbridge.go` or `eb_rule.go` — ensure a rule whose `target_arns` field contains `arn:aws:s3:::a9s-demo-healthy`.
- `internal/demo/fixtures/cloudwatch_logs.go` or wherever logs fixtures live — the S3-access-log destination is ANOTHER S3 bucket (`a9s-demo-logs`); ensure that bucket exists in this same `s3.go` fixture file.

The `a9s-create-demo-fixture` skill's phase-2 graph plan owns the exact file list — coder follows the skill.

## 4. Contract-surface gap analysis (phase 5)

| Surface | Current state | Spec demand | Gap / coder action |
|---|---|---|---|
| `internal/aws/s3_interfaces.go` | All APIs already declared (ListBuckets, ListObjectsV2, GetBucketLocation, GetBucketNotificationConfiguration, GetPublicAccessBlock, GetBucketEncryption, GetBucketLogging, GetBucketTagging). Aggregate `S3API`. | Same — spec §3.2 uses `GetPublicAccessBlock` (already present). | **No change**. |
| `internal/aws/s3_related.go` | 16 registrations: athena, backup, cf, cfn, eb-rule, glue, **iam-user**, kms, lambda, logs, r53, role, sns, sqs, trail, **waf**. | Spec §2 + §5: 15 active pivots (no iam-user, no waf). ct-events registered globally via `zzz_ct_events_all_related.go` — no per-resource work. | **Remove** `checkS3IAMUser` + `checkS3WAF` function bodies AND their two `RegisterRelated` entries. Leave the other 14 checkers untouched. |
| `internal/aws/s3_issue_enrichment.go` | Emits severity `~`, summary varies per case ("no public access block (account-level may apply)" or `"public-access block partial: <flag>=false"`). Writes `FieldUpdates[name]["public_access"]` = `"?" / "BLOCKED" / "RISK"`. Registers `IssueEnricherFieldKeys("s3", []string{"public_access"})`. | severity `!`; summary stable `"public access block incomplete"` (EnrichmentFinding contract U11); per-case detail in `Rows`; `FieldUpdates[name]["status"] = "public access block incomplete"`; register `[]string{"status"}`. | **Rewrite** enricher body per rules above. Delete all references to `public_access` field. |
| `internal/aws/s3_detail_enrichment.go` | Does not exist. | Not required — §2 pivots already use per-pivot S3 API calls at pivot-time (CFN/KMS/Logs) or read fetcher-prepopulated fields (Lambda/SNS/SQS notifications). | **No new file**. |
| `internal/aws/s3.go` (fetcher) | Unknown (not read per skill rules); phase 7 coder reads and rewrites as needed. | Spec §1 Identity: list API returns `Name`, `CreationDate`, `BucketRegion`, `BucketArn`. Fetcher must also populate `Fields["notification_lambda" / "notification_sns" / "notification_sqs"]` via GetBucketNotificationConfiguration to support the related-panel checkers. `Resource.Issues` must be empty (no Wave 1 signals). | Coder reads current, aligns with spec. No Wave 1 classification. |
| `.a9s/views/s3.yaml` + `internal/config/defaults_databases.go` "s3" block | Columns: Bucket Name, Region, Creation Date, **Public Access** (Key `public_access`, jargon). | Columns: Bucket Name, Region, Creation Date, **Status** (Key `status`, width 32, carries `public access block incomplete` phrase per §4). | In `defaults_databases.go`: replace `Public Access` ListColumn with `{Title: "Status", Key: "status", Path: "Name", Width: 32}`. Regenerate `.a9s/views/s3.yaml` via `go run ./cmd/viewsgen/`. |
| Universal ct-events pivot | Auto-registered by `zzz_ct_events_all_related.go`. | Spec §2: ct-events with `count shown: yes`. | **No per-resource work**. |

## 5. Execution order (skill phases 6a / 6b / 7)

1. Delete stale test files (skill runner does this before 6a).
2. Phase 6a: coder via `a9s-create-demo-fixture` skill writes `internal/demo/fixtures/s3.go` + sibling additions per §3 above. Blocks 6b and 7.
3. Phase 6b: QA writes four test files (`aws_s3_test.go`, `aws_s3_related_test.go`, `aws_s3_issue_enrichment_test.go`; `aws_s3_detail_enrichment_test.go` is NOT in scope — no detail enricher exists). Parallel with 7.
4. Phase 7: coder edits `s3.go`, `s3_interfaces.go`, `s3_related.go`, `s3_issue_enrichment.go`, `internal/config/defaults_databases.go` s3 block; regenerates `.a9s/views/s3.yaml`. Parallel with 6b.
5. Phase 7.5: scope-diff gate. Approved union is exactly: `internal/aws/s3.go`, `internal/aws/s3_interfaces.go`, `internal/aws/s3_related.go`, `internal/aws/s3_issue_enrichment.go`, `internal/config/defaults_databases.go`, `.a9s/views/s3.yaml`, `internal/demo/fixtures/s3.go` plus the sibling files listed in §3 graph additions, plus the four test files (minus the detail-enricher test which is out-of-scope this run).
6. Phase 8: scenario-harness visual render gate at `tests/integration/scenario_s3_visual_test.go`.
