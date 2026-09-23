# Related-nav triage — slice D

Types: secrets ses sfn sg sns-sub sns sqs ssm subnet tg tgw trail transfer vpc-peer vpc vpce waf. Tree: main @ ad18304c.

Sources read: `reviews/review-claude.md` (sections secrets … waf), `reviews/review-codex.md`, which has no section for any slice-D type because its sections stop at `glue`, the oneliners files, and backlog rows 1/2/3/5/11/12. None of those backlog rows concerns a slice-D type.

Most ID-class findings were closed by the shared resolver layer in `core/aws/ref_ids.go` (catalog `RefToID`, reached through `resource.ResolveRef`/`NavIDFromValue`). KMS alias findings were closed by `kmsRelated`/`kmsRefContext` (DescribeKey fallback for aliases), and pagination findings by `PageAll`.

## Table

| # | type | source | verdict | class | current file:line | evidence |
|---|------|--------|---------|-------|-------------------|----------|
| 1 | secrets | claude P2 #3 secrets↔KMS alias ARN | FIXED | — | core/aws/secrets_related.go:30, core/aws/kms_related.go:122,144 | Both directions go through `kmsRefToID`. An alias resolves via the kms rows' `alias`/`aliases` fields or a DescribeKey lookup (`ref_ids.go:304-363`). |
| 2 | secrets | claude P2 #4 ECS JSON-key/version ValueFrom, both directions | FIXED | — | core/aws/secrets_related_extra.go:245; core/aws/ecs_task_related_extra.go:120 | `secretsRefToID` cuts the `:json-key:…` tail and the `-XXXXXX` suffix (`ref_ids.go:385-411`). |
| 3 | secrets | claude P2 #5 ecs-svc→secrets raw ValueFrom as IDs | FIXED | — | core/aws/ecs_svc_related_extra.go:311 | `relatedRefs("secrets", …)` resolves each ref to the row name and de-duplicates it. |
| 4 | secrets | claude P2 #6 secrets→eb misses `environmentsecrets` namespace | LIVE | OTHER (the contract and the code both miss AWS's native secret env-var mechanism) | core/aws/secrets_related_extra.go:90,121 | It matches only `{{resolve:secretsmanager:<ARN>`. An environment that sets `aws:elasticbeanstalk:application:environmentsecrets` = secret ARN counts 0. `docs/related-resources.md:951` describes the same narrow rule. |
| 5 | secrets | claude P2 #7 secrets→cb ARN with JSON key | FIXED | — | core/aws/secrets_related.go:193 | `ResolveRef("secrets", ev.Value)` strips the `:json-key` tail from an ARN. |
| 6 | secrets | claude P2 #8 ct-events Secret field NavID | LIVE | ID | core/semantics/ctevent/target.go:136, :259 | `resourceRefToRow` hand-splits `secret:prod/api-AbCdEf` into `prod/api-AbCdEf`, keeping the random suffix, and `resolveNavIDs` keeps a projector NavID. The GetSecretValue row has no NavID, so `secretsRefToID("secret:prod/api-AbCdEf")` reads it as bare and cuts at `:`, giving `secret`. Neither ID is a secrets row ID. |
| 7 | secrets | claude P3 #9 logs pivot confirmed after failed GetFunction | FIXED | — | core/aws/secrets_related_extra.go:289,293,301 | Failure and no-client paths now return `HeuristicRelated` (a candidate, not a count). A definite count is returned only after GetFunction succeeds. |
| 8 | ses | claude P3 #2 receipt-rule recipient semantics | LIVE | MATCH | core/aws/ses_related.go:271 (email), :283-290 (domain) | `HasSuffix(dLower, "."+rLower)` still makes a rule for recipient `acme.com` apply to `x@sub.acme.com`. A leading-dot recipient `.acme.com` never matches. |
| 9 | ses | claude P3 #3 disabled rules/event destinations still counted | LIVE | OTHER (the enabled state is not filtered) | core/aws/ses_related.go:315,342 (rules), :184,:420 (destinations) | No `rule.Enabled` or `dest.Enabled` check anywhere in the file. A rule with `Enabled=false` still yields Lambda/S3 rows. |
| 10 | ses | claude P3 #4 r53 pivot includes private and delegated-parent zones | LIVE | MATCH | core/aws/ses_related.go:52 | `dnsZoneHosts` accepts any parent zone. There is no `private_zone` exclusion, unlike acm (`acm_related.go:185`). |
| 11 | ses | claude P3 #7 web refresh never resets the receipt-rule-set cache | LIVE | OTHER (a stale session cache feeds the pivot) | internal/tui/runtime_adapter_navigate.go:577,630 only; core/app/actions_list.go:159 has none | `ResetRuleSets` is called only from the TUI adapter. On web `R`, the lambda/s3 pivots keep serving the old rule set. |
| 12 | sfn | claude P2 #1 KMS alias mangled (+ oneliners-claude sfn-kms-alias) | FIXED | — | core/aws/sfn_related.go:104 | `kmsRelated`. |
| 13 | sfn | claude P2 #2 logs pivot guesses the log group by naming convention | LIVE | MATCH | core/aws/sfn_related.go:43 | It still builds `/aws/vendedlogs/states/<name>` and never reads `LoggingConfiguration.Destinations`, which `docs/resources/sfn.md` §logs requires. |
| 14 | sfn | claude P3 #3 lambda pivot counts non-name FunctionName values | LIVE | ID | core/aws/sfn_related.go:128,142 | Every `FunctionName` goes to `relatedRefs`. `lambdaRefToID` returns a non-ARN value unchanged (`ref_ids.go:159-176`), so `my-fn:prod`, `{% $states.input.fn %}` and `123456789012:function:my-fn` count as rows. Nothing checks them against the lambda list, which the spec asks for. |
| 15 | sfn | claude P3 #8 ecs-svc→sfn substring family match + JSONata `Arguments` (+ codex oneliner sfn-arguments-taskdef) | LIVE | MATCH | core/aws/ecs_svc_related_extra.go:400-412 | `strings.Contains(td, family)` has no boundary, so family `web` matches `webhook-processor`. Only `m["Parameters"]` is read. |
| 16 | sg | claude P2 #2 sg→sg is the reverse of the contract | LIVE | CONTRACT | core/aws/sg_related.go:135-163; core/aws/catalog_networking.go:244 | The checker scans other SGs that reference this one. The contract (`related-resources.md:992`, `resources/sg.md:58-62`) wants this SG's own `UserIdGroupPairs[].GroupId`. |
| 17 | sns-sub | claude P2 #1 sns-sub→lambda substring and qualifier | FIXED | — | core/aws/sns_sub_related.go:57 | `ResolveRef("lambda")` strips the qualifier and checks region/account. Equality only. |
| 18 | sqs / sns-sub / sns | claude sns-sub P2 #2 + sqs P2 #1(a) + sns #2 "same defect elsewhere": `orders` matches `orders-dlq` | FIXED | — | core/aws/predicates.go:79-84 | `endpointIsQueue` compares the ARN with `==`. The `Contains` is gone. |
| 19 | sns-sub | claude P3 #3 (forward) sns-sub→sqs by name alone | FIXED | — | core/aws/sns_sub_related.go:91 | `sqsRefToID` via `localARN` rejects another region's or account's ARN. |
| 20 | sqs / sns-sub | claude sqs P2 #1(b) + sns-sub P3 #3 (reverse): name fallback runs although the queue ARN is known | LIVE | MATCH | core/aws/predicates.go:83 (used at sqs_related.go:44,:98) | When `endpoint != queueARN`, `HasSuffix(endpoint, ":"+queueName)` still matches. A cross-region or cross-account `…:orders` subscription lands on the local `orders`. |
| 21 | sns-sub | claude P3 #4 lambda→sns-sub / lambda→sns qualifier and foreign account | FIXED | — | core/aws/lambda_related_extra.go:291,323; predicates.go:156 | `lambdaRefNamesFunction` goes through `ResolveRef("lambda")`. |
| 22 | sns-sub | claude P3 #6 ct-events pivot queries a made-up row ID | LIVE | ID | core/aws/catalog_messaging.go:305; core/aws/sns_sub.go:205-211; resource/related.go:782 | `CloudTrailKey: "ResourceName:ID"` sends `pending/<topicArn>/<protocol>/<endpoint>` to LookupEvents. The lookup can never match, but it is a live deferred pivot rather than "no pivot". |
| 23 | sns | claude P2 #1 KMS alias (`alias/aws/sns`) | FIXED | — | core/aws/sns_related.go:92 | `kmsRelated`. |
| 24 | sns | claude P2 #2 sns→alarm substring | FIXED | — | core/aws/alarm_match.go:172-183,250 | `actionNames` uses exact `slices.Contains` on the action ARN. |
| 25 | sns | claude P2 #3 eb-rule→sns bare topic name | FIXED | — | core/aws/eb_rule_related.go:68 | `resolveRefs("sns")`. `snsRefToID` keeps the whole ARN (`ref_ids.go:204`). |
| 26 | sns | claude P3 #5 sns→role lists denied and foreign roles | FIXED | — | core/aws/sns_related.go:113-118 | `grantedPrincipalRefs` (Allow after Deny, `ref_ids.go:55-67`). The resolver drops foreign accounts (`policyRefContext`). |
| 27 | sqs | claude P2 #2 lambda ESM qualifier + no retry | FIXED | — | core/aws/sqs_related.go:215; related_common.go:377-409 | Shared `lambdaEventSourceMappingLambdaCheck`, using `resolveRefs` and `PageAll`, which wraps `RetryOnThrottle`. |
| 28 | sqs | claude P2 #3 lambda pivot ignores functions using the queue as DLQ | LIVE | CONTRACT | core/aws/sqs_related.go:206-216 | Only ESM is checked. `docs/resources/sqs.md:54-56` requires the `DeadLetterConfig.TargetArn` scan too, with a combined count. |
| 29 | sqs | claude P3 #5 eb-rule / ESM first page only | FIXED | — | core/aws/eb_rule_related.go:110-137; lambda_related.go:160-170 | Both are paged with `PageAll`, and truncation is carried. |
| 30 | ssm | claude P2 #4 KMS row 0 for `alias/aws/ssm` | FIXED | — | core/aws/ssm_related.go:31; kms.go:233-238 | `kmsRefContext` asks DescribeKey for an unloaded alias, and the returned row carries that alias. |
| 31 | ssm | claude P2 #5 Enter on `KeyId` alias | LIVE | NAV | core/app/detail_body.go:265-275; core/aws/ref_ids.go:318-324 | Navigation resolves against loaded kms rows only, with no DescribeKey fallback. `alias/aws/ssm` is never in the customer-key list, so the field is struck to non-navigable. A customer alias works only while the kms list is loaded. The panel resolves the same value, so the field and the panel disagree. |
| 32 | ssm | claude P3 #6 key with several aliases | FIXED | — | core/aws/kms.go:96-105; ref_ids.go:320 | The `aliases` field holds every alias, and the resolver matches any of them. |
| 33 | subnet | claude P2 #1 EFS mount-target description parse | FIXED | — | core/aws/subnet_related.go:268; predicates.go:117-124 | Regex `\bfs-[0-9a-z]+\b` covers both description forms. |
| 34 | subnet | claude P3 #2 EFS drops ENI truncation (+ oneliners-claude subnet-efs-truncation) | FIXED | — | core/aws/subnet_related.go:289 | `efsTruncated \|\| eniTruncated`. |
| 35 | subnet | claude P2 #3 main-rtb fallback on a truncated list (+ oneliners-claude subnet-main-rtb-truncated) | FIXED | — | core/aws/subnet_related.go:163,187 | The fallback applies only when `rtbComplete`. |
| 36 | tg | claude P2 #1 eb→tg returns ARNs and double-counts | FIXED | — | core/aws/eb_related_extra.go:171 | `resolveRefs("tg")`: the ARN becomes the name, de-duplicated. |
| 37 | tg | claude P2 #2 ec2→tg claims every instance TG in the VPC | FIXED | — | core/aws/ec2_related.go:57; docs/resources/ec2.md:118-122 | The contract was amended to a heuristic (blank, navigable row). The checker returns `heuristicResult`. |
| 38 | tg | claude P2 #3 lambda→tg misses alias-qualified registration | FIXED | — | core/aws/lambda_related_extra.go:248 | `lambdaRefNamesFunction` strips the qualifier. |
| 39 | tg | claude P2 #4 navigable ARN fields into tg and out to elb | FIXED | — | core/aws/ref_ids.go:198 (`tgRefToID`), :416-432 (`elbRefToID`); app/detail_body.go:271 | The catalog `RefToID` for tg/elb turns the ARN into a name for every navigable field. |
| 40 | tg | claude P3 #5 tg→elb lower bound though LoadBalancerArns is complete | LIVE | CARD | core/aws/tg_related.go:44-72 | It matches only against the rows of `relatedResourcesFor` and returns that list's `truncated`. The closed ARN set shows `0+`/`N+`, or Unknown with no elb list, instead of an exact count. `listedRefs` (`ref_ids.go:109`) exists for this case and is not used. |
| 41 | tgw | claude P2 #2 VPC/Subnet pivots first page only | FIXED | — | core/aws/tgw_related.go:55-65 | `PageAll`, with `!complete` carried. |
| 42 | tgw | claude P3 #3 VPC/Subnet pivots count deleted/failed/rejected attachments | LIVE | OTHER (the attachment state is not filtered) | core/aws/tgw_related.go:57-58, :45-49, :159-167 | No `state` filter and no `att.State` check. |
| 42b | vpc | claude P3 #4 vpc→tgw counts deleted/failed/rejected attachments | LIVE | OTHER (the attachment state is not filtered) | core/aws/vpc_related.go:276-297 | The `DescribeTransitGatewayAttachments` filters are `resource-id` and `resource-type` only. The loop appends every `TransitGatewayId` whatever its `att.State`. |
| 43 | tgw | claude P3 #5 tgw→rtb counts blackhole routes (+ oneliners-claude tgw-rtb-blackhole) | FIXED | — | core/aws/tgw_related.go:89; predicates.go:47-50 | The shared `routeGatewayTarget` skips blackhole routes for both ends. |
| 44 | tgw | claude P3 #6 role pivot shows the account-wide SLR as a count | FIXED | — | core/aws/tgw_related.go:130; docs/resources/tgw.md:46-50 | The contract was amended to a heuristic. The checker returns `heuristicResult` (blank row). |
| 45 | trail | claude P2 #4 Enter on CloudWatchLogsLogGroupArn → `*` | FIXED | — | core/aws/ref_ids.go:462-473; catalog_monitoring.go:114,220 | `logsRefToID` cuts after `log-group:` and strips `:*`. |
| 46 | trail | claude P3 #5 logs checker matches by name, ignoring region/account | FIXED | — | core/aws/trail_related.go:60-71 | `relatedListIn` uses the ARN's region. The resolver rejects a foreign account (lower bound). |
| 47 | trail | claude P3 #6 ct-events queries the session region, not HomeRegion | FIXED | — | core/aws/catalog_monitoring.go:170; install.go:38 | `CloudTrailRegion: ctRegionOfTrail` (`home_region`). |
| 48 | vpce | claude P2 #2 alarm dimension `VpcEndpointId` never published | FIXED | — | core/aws/alarm_match.go:253 | Matches `VPC Endpoint Id` and `VpcEndpointId` in `AWS/PrivateLinkEndpoints`. |
| 49 | vpce | claude P2 #3 logs pivot filters flow logs by the endpoint ID | LIVE | CALL | core/aws/vpce_related.go:106-108 | `resource-id` is still `vpceID`, which never owns a flow log, so the answer is always a proven 0. The secondary faults (paging, S3/Firehose destinations) are fixed at :105-128. |
| 50 | vpce | claude P2 #4 r53 bare zone IDs | FIXED | — | core/aws/vpce_related.go:195; ref_ids.go:478-490 | `r53RefToID` adds `/hostedzone/`. |
| 51 | vpce | claude P3 #5 r53 first page only | FIXED | — | core/aws/vpce_related.go:180-190 | `PageAll`, with `!complete` carried. |
| 52 | waf | claude P1 #2 (related part) CLOUDFRONT ACLs read through the regional client | FIXED | — | core/aws/waf_related.go:28-29,63-64,154-156 | elb/apigw short-circuit to a proven 0 for CLOUDFRONT. Logs uses `c.wafIn(scope)` (us-east-1). The Wave-2 part is out of scope. |
| 53 | waf | claude P2 #3 logs pivot counts Firehose/S3 destinations | FIXED | — | core/aws/waf_related.go:80-84 | Only `ARNForService(d, "logs")` destinations are kept. |
| 54 | waf | claude P2 #4 alarm→waf passes the ACL name as ID | FIXED | — | core/aws/alarm_related_extra.go:76; alarm_match.go:128,254 | `alarmRowsNaming` matches the dimension against the row's ID/Name and returns `row.ID`. |
| 55 | waf | claude P3 #5 + codex oneliner `waf-cf-nextmarker`: CF pagination | FIXED | — | core/aws/waf.go:48-63; waf_related.go:117 | `PageAll` over `NextMarker`, with `!complete` carried. |
| 56 | waf | claude P3 #6 (related part) logging in SECURITY_LAKE / telemetry-rule scopes → proven 0 | LIVE | CALL | core/aws/waf_related.go:64-70 | `GetLoggingConfigurationInput` is sent without `LogScope` (the SDK v1.83.0 default is CUSTOMER). `WAFNonexistentItemException` becomes `ProvenZero`, although a `CLOUDWATCH_TELEMETRY_RULE_MANAGED` config can deliver to a log group. |

## Counts

- In-scope findings: 57 rows (merged). Row 18 merges three reports and row 20 merges two, and sqs #1 and sns-sub #3 are each split into a fixed half and a live half.
- FIXED: 38
- DISPROVED: 0
- LIVE: 19. The classes:
  - ID 3: rows 6, 14, 22.
  - MATCH 5: rows 8, 10, 13, 15, 20.
  - PAGE 0.
  - CALL 2: rows 49, 56.
  - CARD 1: row 40.
  - NAV 1: row 31.
  - CONTRACT 2: rows 16, 28.
  - OTHER 5: rows 4, 9, 11, 42, 42b.

Out of scope (list columns, findings/severity, wave-2, rendering, console URL): 36. By type:

| type | out-of-scope findings | count |
|------|-----------------------|-------|
| secrets | #1, #2, #10 | 3 |
| ses | #1, #5, #6 | 3 |
| sfn | #4–#7 | 4 |
| sg | #1 | 1 |
| sns-sub | #5 (console URL), #7 | 2 |
| sns | #4 | 1 |
| sqs | #4 | 1 |
| ssm | #1–#3 | 3 |
| tgw | #1, #4 | 2 |
| trail | #1–#3 | 3 |
| transfer | #1–#4 | 4 |
| vpc-peer | #1, #2 | 2 |
| vpc | #1, #2, #3, #5 | 4 |
| vpce | #1 | 1 |
| waf | #1, #7 (and the wave-2 halves of #2 and #6) | 2 |

Cross-slice notes (not counted: they sit in other types' sections but touch slice-D code):

- codex alarm section: the `waf_related.go:54` WebACL dimension is the metric name, not the ACL name. `alarm_match.go:254` matches ID/Name only, so an ACL whose `VisibilityConfig.MetricName` differs from its name gets no match. This is unverified here and belongs to the alarm slice.
- codex eb-rule section `sfn_related.go:237` / `sqs_related.go:267`: already paged per bus (`eb_rule_related.go:110-137`).
- codex codeartifact and eb sections cite `secrets_related_extra.go:36,58,62,94`.

## Shared shapes

1. **The lifecycle or enabled state is not filtered before counting.** A pivot counts relationships AWS still returns but that carry nothing:
   - tgw VPC/Subnet attachments: `tgw_related.go:57-58` via `tgwVpcAttachments`.
   - vpc→tgw attachments: `vpc_related.go:276-297`.
   - Disabled SES receipt rules and event destinations: `ses_related.go:184,315,342,420`.

   One attachment-state predicate in `tgwVpcAttachments` plus a matching `state` filter at `vpc_related.go:276` covers both attachment sites.
2. **The KMS alias resolution is split between the panel and navigation.** `kmsRelated` falls back to DescribeKey (`ref_ids.go:332-354`). `resolveNavIDs` (`app/detail_body.go:265-275`) resolves against loaded rows only, so every navigable `KmsKeyId`/`KeyId` holding an alias that no loaded row carries is struck non-navigable while its panel row resolves. The sites are:
   - ssm `KeyId` (`catalog_secrets.go:137`)
   - secrets `KmsKeyId` (`catalog_secrets.go:91`)
   - trail `KmsKeyId` (`catalog_monitoring.go:218`)
   - the `KmsKeyId` navigable at `catalog_monitoring.go:155`
   - msk `DataVolumeKMSKeyId` (`catalog_messaging.go:543`)
3. **Hand-parsed IDs that bypass the resolver layer:**
   - ctevent `resourceRefToRow` NavID split (`semantics/ctevent/target.go:130-138`) for every non-role label, including Secret.
   - The GetSecretValue row whose value keeps the `secret:` resource prefix (`target.go:252-259`).
   - The sns-sub `CloudTrailKey: "ResourceName:ID"` on made-up IDs (`catalog_messaging.go:305`).
4. **Bare-value acceptance in `arnNameRef` without a list check.** `lambdaRefToID` returns any non-ARN string as a function name (`ref_ids.go:159-176`), so callers that use `relatedRefs` without `listedRefs` count garbage:
   - sfn FunctionName (`sfn_related.go:128`)
   - ses rule Lambda ARNs (`ses_related.go:320`) are safe only because SES requires ARNs.
5. **A name fallback that fires when the ARN is known and differs.** The sites are:
   - `endpointIsQueue` (`predicates.go:83`)
   - `checkSQSSQS` name fallback (`sqs_related.go:170-175`, ARN-unknown only, so it is safe)
6. **Substring/prefix match without a boundary.** The sites are:
   - `sfnASLHasECSFamily` `strings.Contains(td, family)` (`ecs_svc_related_extra.go:403,409`), although `taskDefFamily` (`ref_ids.go:537`) exists.
   - SES recipient parent-domain suffix (`ses_related.go:271`).
   - SES r53 parent-zone acceptance (`ses_related.go:52`).
