# Slice C — related panel and navigation triage

Types: kms lambda logs lt msk mwaa nat ng opensearch pipeline policy r53 redis redshift role rtb s3. Tree: main @ ad18304c.

`reviews/review-codex.md` has no sections for these types (it ends at `glue`). `reviews/oneliners-codex.md` "Rejected as not small" names one of them, pipeline `CodeDeployToECS`, which is merged into row 35. `reviews/oneliners-claude.md` "Notes on what was rejected" names only classes, not rows, for these types; its "GetCrawlers for the S3↔Glue join" and "per-bucket S3 region routing" match rows 57 and 54. From `reviews/backlog.md`, row 2 (s3 part) and row 3 (ng part) are in this slice. Rows 1, 5, 11 and 12 concern other types.

## Findings

| # | type | source | verdict | class | current file:line | evidence |
|---|------|--------|---------|-------|-------------------|----------|
| 1 | kms | claude P2 §1 CloudTrail pivot filters on bare key UUID | LIVE | ID | core/aws/catalog_secrets.go:151; core/aws/kms.go:98-109 | `CloudTrailKey: "ResourceName:ID"` sends the UUID. The row has no `Fields["arn"]`, so `ctAltName` (core/resource/related.go:757-770) has no ARN spelling to retry. Trigger: any key whose events carry only the key ARN. |
| 2 | kms | claude P2 §2 grants read one page | FIXED | — | core/aws/kms_related.go:198-218 | `PageAll` over `ListGrants` with `NextMarker`; `relatedResultTrunc(..., dropped \|\| !complete)`. |
| 3 | lambda | claude P2 §5 MSK pivot emits cluster UUID | FIXED | — | core/aws/lambda_related_extra.go:165-176; core/aws/ref_ids.go:200 | ARNs go through `mskRefToID = arnNameRef("kafka","cluster/","/")`, which gives the cluster name. |
| 4 | lambda | claude P2 §6 EFS pivot emits access-point id | FIXED | — | core/aws/lambda_related_extra.go:45-56; core/aws/ref_ids.go:434-457; core/aws/efs.go:128 | `efsRefToID` maps `fsap-` to the file system whose `access_point_ids` holds it. When the fs is not loaded, the ref is dropped and the count reads as a lower bound. |
| 5 | lambda | claude P2 §7 apigw pivot matches name substring / tag key | LIVE | CARD | core/aws/lambda_related_extra.go:64-94; core/aws/related_common.go:156-158 | The row is now labelled "candidates", as the contract allows. But `heuristicResult` returns `ProvenZero` when no name or tag matches, so an API that integrates the function under an unrelated name reads as a proven `(0)`. Integrations are never read. |
| 6 | lambda | claude P2 §8 SQS/SNS pivots ignore the DLQ target | LIVE | CONTRACT | core/aws/lambda_related.go:130-146; core/aws/lambda_related_extra.go:270-303 | `checkLambdaSQS` reads only event-source mappings, and `checkLambdaSNS` reads only `sns-sub`. `Fields["dlq_target_arn"]` (lambda.go:100) is never consulted. The contract says "SQS queues invoking the function or used as DLQ" (related-resources.md lambda `sqs`), and the spec's §2 sns (a) says the same for SNS. |
| 7 | lambda | claude P3 §10 Kinesis EFO consumer ARN gives consumer name | FIXED | — | core/aws/ref_ids.go:198 | `kinesisRefToID = arnNameRef("kinesis","stream/","/")` takes the segment after `stream/` up to the next `/`. |
| 8 | logs | claude P2 §3 alarm pivot misses metric-filter alarms | FIXED | — | core/aws/logs_related.go:53-97 | `logGroupFilterMetrics` pages `DescribeMetricFilters` and matches alarms on the (namespace, metric) pair. A partial read gives `PartialScan`. |
| 9 | logs | claude P2 §4 S3 exports pivot can never return a bucket | LIVE | CALL | core/aws/logs_related.go:260-271 | The code still looks for `s3` ARNs in subscription-filter destinations, which never hold S3. The contract says "Export tasks to S3", which is `DescribeExportTasks`. Every group shows a proven `(0)`. |
| 10 | logs | claude P3 §6 lambda pivot only parses `/aws/lambda/<name>` | LIVE | CONTRACT | core/aws/logs_related.go:20-45 | No `LoggingConfig.LogGroup` match and no subscription-filter Lambda consumers. The contract says "Lambdas whose logs land here OR subscription-filter consumers". |
| 11 | logs | claude P3 §7 KMS pivot unknown on cache-seeded row | LIVE | CARD | core/aws/logs_related.go:140-146; core/aws/cwlogs.go:121-123 | With `RawStruct == nil` the pivot returns `UnknownRelated`, although `Fields["kms_key_id"]` and `Fields["encryption"]` hold the answer. |
| 12 | msk | claude P2 §1 Secrets pivot keeps the random suffix | FIXED | — | core/aws/msk_related.go:220; core/aws/ref_ids.go:385-413 | `secretsRefToID` matches a loaded ID or ARN first, then strips the `-XXXXXX` suffix. |
| 13 | msk | claude P2 §2 KMS pivot passes the raw ARN | FIXED | — | core/aws/msk_related.go:238; core/aws/ref_ids.go:304-322 | `kmsRelated` → `kmsRefToID` gives the key ID. |
| 14 | msk | claude P2 §3 serverless clusters report 0 sg/subnet/vpc | LIVE | CARD | core/aws/msk_related.go:30, 98, 112 | `Provisioned == nil` returns `ProvenZero`. `Serverless.VpcConfigs[]` is never read. Trigger: any SERVERLESS cluster. |
| 15 | msk | claude P3 §5 ListScramSecrets called without SCRAM enabled | LIVE | CARD | core/aws/msk_related.go:191-221 | No `Sasl.Scram.Enabled` check. If the principal lacks `kafka:ListScramSecrets`, a non-SCRAM cluster shows an error where the answer is a proven 0. |
| 16 | msk | claude P3 §6 ListScramSecrets one page | FIXED | — | core/aws/msk_related.go:207-221 | `PageAll` on `NextToken`; `!complete` becomes a lower bound. |
| 17 | msk | claude P3 §7 ListEventSourceMappings one page | FIXED | — | core/aws/related_common.go:377-398; core/aws/lambda_related.go:160-171 | `listEventSourceMappings` uses `PageAll` on `NextMarker`. |
| 18 | msk | claude P3 §8 alarm pivot has no namespace | FIXED | — | core/aws/alarm_match.go:246; core/aws/msk_related.go:20 | spec `{Namespaces: AWS/Kafka, DimensionNames: "Cluster Name"}`. |
| 19 | mwaa | claude P2 §1 alarm pivot uses a dimension MWAA never publishes | FIXED | — | core/aws/alarm_match.go:247; core/aws/mwaa_related.go:24 | Namespaces `AmazonMWAA`, `AWS/MWAA`; dimensions `Environment`, `EnvironmentName`. The contract text (related-resources.md mwaa `alarm`) still names only `EnvironmentName`: a doc drift, not a code defect. |
| 20 | nat | claude P3 §2 alarm pivot misses metric-math alarms | FIXED | — | core/aws/alarm_match.go:36-52 | `AlarmDimensions` walks `Metrics[].MetricStat.Metric.Dimensions` with each query's namespace. Shared by every alarm pivot. |
| 21 | ng | claude P2 §1 sg ignores `RemoteAccess.SourceSecurityGroups` | LIVE | CONTRACT | core/aws/ng_related.go:167-177 | Reads only `Resources.RemoteAccessSecurityGroup`. The contract `ng` → `sg` is "RemoteAccess.SourceSecurityGroups". A bastion-SG-only node group shows a proven `(0)`. |
| 22 | ng | claude P2 §2 ami reports proven 0 for default node groups | LIVE | CARD | core/aws/ng_related.go:188, 210 | With no launch template, or a template without `ImageId`, the pivot returns `ProvenZero`. The group still runs the EKS-optimised AMI (ng.go:37-38 says so). |
| 23 | ng | claude P3 §3 ng→ami ($Latest) vs fetcher ($Default) | FIXED | — | core/aws/asg_related.go:308-309; core/aws/ng_related.go:195; core/aws/ng.go:29 | Both call `launchTemplateVersions`, which defaults to `$Default`. |
| 24 | ng | claude P3 §4 CloudTrail pivot cannot tell clusters apart | FIXED | — | core/aws/catalog_containers.go:116-120 | `CloudTrailQualifier{ParentField: "cluster_name", EventPaths: requestParameters.clusterName}` drops other clusters' events. |
| 25 | ng | claude P3 §6 ami→ng drops groups whose image was not resolved | LIVE | CARD | core/aws/ami_related_extra.go:67-86; core/aws/catalog_containers.go:228-233 | Degraded rows now truncate through `anyDegraded` (related_fetch.go:41/65): that part is fixed. A failed `DescribeLaunchTemplateVersions` still writes `image_id=""` with no row marker. Once the list is cached (related_fetch.go:41 reads `entry.IsTruncated \|\| anyDegraded`), `checkAMING` answers exact. I did not verify that the cached entry never carries the page's partial failure as `IsTruncated`. |
| 26 | ng | backlog row 3 ng→eks renders `(1+)` | LIVE | CARD | core/aws/ng_related.go:42; core/aws/related_fetch.go:41, 65 | `relatedResultTrunc("eks", ids, truncated)`, where `truncated` includes `anyDegraded(eksList)`. A one-to-one parent renders `1+` when any eks row is degraded. |
| 27 | opensearch | claude P2 §2 logs pivot counts disabled log-publishing options | LIVE | CONTRACT | core/aws/opensearch_related.go:33-37 | The loop never checks `opt.Enabled`, so a group the domain no longer publishes to is counted. |
| 28 | opensearch | claude P2 §3 VPCOptions navigable fields never render | FIXED | — | core/config/defaults_databases.go:61; core/aws/catalog_databases.go:531-533 | `{Path: "VPCOptions"}` is in the detail list. |
| 29 | opensearch | claude P3 §4 alarm pivot has no namespace | FIXED | — | core/aws/alarm_match.go:249; core/aws/opensearch_related.go:20 | `AWS/ES` + `DomainName`. |
| 30 | opensearch | claude P3 §5 ACM pivot re-fetches with DescribeDomainConfig, ignores CustomEndpointEnabled | LIVE | CALL | core/aws/opensearch_related.go:177-205 | Still one `DescribeDomainConfig` per open, with no `CustomEndpointEnabled` check. `DomainStatus.DomainEndpointOptions` in RawStruct already carries both fields. A role without `es:DescribeDomainConfig` gets an error row. |
| 31 | pipeline | claude P2 §1 KMS pivot counts alias ARNs and cross-region keys | FIXED | — | core/aws/pipeline_related.go:206-222; core/aws/ref_ids.go:124-135, 332-362 | `kmsRelated` resolves aliases through `DescribeKey`. `localARN` drops keys in other regions or accounts, and a dropped key makes the count a lower bound. |
| 32 | pipeline | claude P2 §2 S3 pivot misses source bucket | FIXED | — | core/aws/pipeline_related.go:266-274 | Reads both `BucketName` and `S3Bucket`. |
| 33 | pipeline | claude P2 §3 cross-region/account action targets match local same-named resources | LIVE | CALL | core/aws/pipeline_related.go:108-124 (cb), 146-162 (cfn), 165-181 (ecr), 185-203 (ecs-svc), 226-243 (lambda); core/aws/codebuild_related.go:79-88; core/aws/ecr_related_extra.go:105-114 | `ActionDeclaration.Region` is never read, so bare names are matched against the session region. The role part is fixed: `relatedRefs` + `localARN` drop foreign-account role ARNs (:127-143). |
| 34 | pipeline | claude P3 §4 ecs-svc ignores cluster | FIXED | — | core/aws/pipeline_related.go:196-202; core/aws/ecs_svc.go:126-128 | Emits `ecsSvcID(ClusterName, ServiceName)` through `resolveRefs`. |
| 35 | pipeline | claude P3 §5 blue/green ECS reports proven 0 + codex oneliners-rejected "pipeline CodeDeployToECS" | LIVE | CONTRACT | core/aws/pipeline_related.go:193 | The filter is still `ECS`/`ECSBlueGreen`. `CodeDeployToECS` falls through to a proven `(0)`. |
| 36 | pipeline | claude P3 §6 eb-rule proven 0 without ARN / one page | LIVE | CARD | core/aws/pipeline_related.go:304-305 | (a) is live: an empty `Fields["arn"]` (STS unresolved) returns `ProvenZero`. (b) is fixed: `ebRulesTargeting` pages every bus (eb_rule_related.go:110-130). |
| 37 | policy | claude P1 §1 ListEntitiesForPolicy one page | FIXED | — | core/aws/iam_policies_related.go:44-60, 96 | `PageAll` on `Marker`; `IsTruncated = !complete` becomes a lower bound. |
| 38 | policy | claude P2 §2 inline group policies keyed by bare name | FIXED | — | core/aws/iam_policies.go:407-413, 453-465; core/aws/iam_policies_related.go:131-133 | The ID is `inline/<group>/<name>`. Managed rows are keyed by ARN (iam_policies.go:130). |
| 39 | policy | claude P2 §3 ListGroups / ListGroupPolicies one page | LIVE | PAGE | core/aws/iam_policies.go:355-357, 395-397 | Single calls, no `Marker` loop. Inline policies of groups past page 1 are missing, and a group → policy drill for them fails. |
| 40 | policy | claude P2 §4 CloudTrail filters by name, not ARN | FIXED | — | core/aws/catalog_security.go:124; core/resource/related.go:741-770; core/aws/ct_events.go:94-111 | Sends `policy_name`. An empty first page retries under `Fields["arn"]` (the alt spelling). When both spellings occur in one account, the list is the documented subset (related-resources.md Policy rule 4). |
| 41 | policy | claude P2 §5 drill from a CloudTrail `policyArn` to an AWS-managed policy fails | FIXED | — | core/aws/iam_policies.go:163-177, 287-296 | `getAWSManagedPolicy` calls `GetPolicy` on an ARN directly. |
| 42 | policy | claude P3 §8 process-global ListEntitiesForPolicy cache outlives rotation | LIVE | OTHER (cache not session-scoped) | core/aws/iam_policies_related.go:19-24, 35-66 | It is still a package `sync.Map` keyed only by ARN. It stores `err` too, including context-cancel. An AWS-managed ARN opened within 5 s of a profile switch shows the previous account's entities. |
| 43 | r53 | claude P2 §1 elb pivot misses NLB | FIXED | — | core/aws/predicates.go:145-147; core/aws/r53_related.go:93 | `maybeELBDNS` checks for `.elb.`. The join is on `dns_name`. |
| 44 | r53 | claude P2 §2 apigw pivot extracts `d-xxxx` domain id | LIVE | ID | core/aws/parse.go:228-234; core/aws/r53_related.go:169-189 | The label before `.execute-api.` is a custom-domain ID and never an API ID. `listedRefs` now marks this a lower bound, so the row renders `(0+)` instead of a proven 0, but it can never match. |
| 45 | r53 | claude P2 §3 acm pivot emits validation record names | FIXED | — | core/aws/r53_related.go:240-289 | Validation CNAME domains are matched against the cert `DomainName`/SANs in the acm list, and the pivot emits `cert.ID`. |
| 46 | r53 | claude P2 §4 logs pivot reports 0 outside us-east-1 | FIXED | — | core/aws/r53_related.go:326-341 | `relatedListIn` reads the config ARN's region. `inRegion` carries that region to the drill. |
| 47 | r53 | claude P2 §5 ct-events queries the session region | FIXED | — | core/aws/catalog_dns_cdn.go:42-43 | `CloudTrailRegion: ctRegionUSEast1`. The reviewer's side question, whether CloudTrail records `/hostedzone/Z…` or `Z…`, is still unverified: `CloudTrailKey: ResourceName:ID` sends `/hostedzone/Z…`, and there is no `Fields["arn"]` alt. |
| 48 | r53 | claude P2 §6 failed record scan stored as "no aliases, not truncated" | LIVE | PAGE | core/aws/r53.go:88-96, 162-170; core/aws/s3_related.go:408 | When `aliasErr` is set, the row keeps `records_truncated=""`, so `checkS3R53` treats the zone as a confident non-match. `ListResourceRecordSets` in `enumerateR53AliasTargets` is not wrapped in `RetryOnThrottle`. |
| 49 | r53 | claude P3 §8 vpc pivot counts associations in other regions | LIVE | CALL | core/aws/r53_related.go:367-375 | `VPCRegion` is dropped. Every association is emitted as exact and drilled in the session region. |
| 50 | role | claude P2 §2 trust users/groups include Deny and foreign-account principals | FIXED | — | core/aws/iam_roles_related.go:69-84; core/aws/ref_ids.go:45-68 | `iampolicy.Parse` + `AllowedPrincipals` (Deny applied). `policyRefContext` + `localARN` drop other accounts. |
| 51 | rtb | claude P2 §1 every `GatewayId` links to igw | FIXED | — | core/aws/ref_ids.go:492-494; core/app/detail_body.go:271-276 | `igwRefToID` accepts only `igw-`, and `resolveNavIDs` clears navigability when the ID is empty. `local`, `vgw-` and `vpce-` render as plain text. |
| 52 | rtb | claude P2 §2 `VpcPeeringConnectionId` links to vpc | FIXED | — | core/aws/catalog_networking.go:391 | `TargetType: "vpc-peer"`. |
| 53 | rtb | claude P3 §3 blackhole route targets still navigable | LIVE | NAV | core/aws/catalog_networking.go:387-390; core/semantics/projection/generic.go:255-265 | The nav entries have no `Resolve` and no route-state guard. nat, eni and tgw have no `RefToID`, so a blackholed `nat-…` stays a link to a deleted target. The related checkers skip blackhole routes, so the two surfaces still disagree. |
| 54 | s3 | claude P1 §1 buckets outside the session region cannot be browsed or checked + claude oneliners-rejected "per-bucket S3 region routing" | FIXED | — | core/aws/s3_cross_region.go:41-43, 65-85; core/aws/catalog_data.go:276; core/aws/s3_related.go:100, 165, 439 | `s3For` / `s3PerBucket` route each call to `BucketRegion` (learned from ListBuckets, else `GetBucketLocation`). |
| 55 | s3 | claude P2 §3 role/SQS/Lambda pivots ignore the ARN's account | FIXED | — | core/aws/s3_related.go:55-86, 464-470; core/aws/ref_ids.go:124-135 | Notifications go through `relatedRefs` with `localARN` (account and region). The role pivot goes through `grantedPrincipalRefs` + `refContext` account. |
| 56 | s3 | claude P3 §4 KMS pivot shows 0 for the `aws/s3` managed key | LIVE | CARD | core/aws/s3_related.go:194-201 | `aws:kms` with an empty `KMSMasterKeyID` still hits `continue`, and `kmsRelated(nil)` gives a proven `(0)` for a KMS-encrypted bucket. |
| 57 | s3 | claude P3 §5 Glue pivot joins job script location, not crawler targets + claude oneliners-rejected "GetCrawlers for the S3↔Glue join" | LIVE | CONTRACT | core/aws/s3_related.go:288-311 | Matches only `Command.ScriptLocation`. The contract `s3` → `glue` is "Glue crawlers over S3 data". |
| 58 | s3 | backlog row 2 backup pivot ignores resource tags | LIVE | OTHER (tag selection never read) | core/aws/s3_related.go:316-347 | `backupPivot(..., backupTarget{unread: "GetBucketTagging"})` renders a tag-selected plan as a lower bound, honestly. The tag read the backlog asks for does not exist. |

## Counts

- In scope: 58 (56 review-claude findings, 2 backlog rows; 3 oneliners-rejected mentions merged into rows 35, 54 and 57).
- FIXED: 32
- DISPROVED: 0
- LIVE: 26
  - ID 2 (1, 44)
  - MATCH 0
  - PAGE 2 (39, 48)
  - CALL 4 (9, 30, 33, 49)
  - CARD 9 (5, 11, 14, 15, 22, 25, 26, 36, 56)
  - NAV 1 (53)
  - CONTRACT 6 (6, 10, 21, 27, 35, 57)
  - OTHER 2 (42, 58)
- Out of scope: 42. Per type: kms 4, lambda 6, logs 7, lt 3, msk 3, mwaa 1, nat 1, ng 1, opensearch 1, pipeline 1, policy 3, r53 1, redis 4, redshift 3, role 2, rtb 0, s3 1.

## Shared shapes

1. **Proven zero when the checker read a narrower shape than the relation.** A checker looks in one field. When that field is empty it returns `ProvenZero`, although the relation lives in a field it never read:
   - ng → ami: default AMI (ng_related.go:188, 210)
   - msk → sg/subnet/vpc: `Serverless.VpcConfigs` (msk_related.go:30, 98, 112)
   - s3 → kms: `aws/s3` (s3_related.go:198)
   - pipeline → ecs-svc: `CodeDeployToECS` (pipeline_related.go:193)
   - pipeline → eb-rule: missing ARN (pipeline_related.go:305)
   - logs → s3: wrong API (logs_related.go:260-271)
   - lambda → apigw: `heuristicResult` zero (related_common.go:157-158)

   These are rows 5, 9, 14, 22, 35, 36 and 56. They all rely on "absent in what I read" meaning "absent". The fix belongs in how these checkers decide a zero, not in each one separately.
2. **Region carried by a bare name or a side field is ignored.** `localARN` gates region and account only for refs that are ARNs. A pivot whose source gives a bare name plus a separate region field passes through with no region check:
   - pipeline actions: `ActionDeclaration.Region` in the cb/cfn/ecr/ecs-svc/lambda checkers and the reverse `cbPipelineHasProject` / `ecrPipelineHasRepo`
   - r53 private-zone VPCs: `VPC.VPCRegion` (r53_related.go:367-375)

   These are rows 33 and 49.
3. **The checker reads fewer sources than the contract bullet names.** The contract names a union and the checker reads one part:
   - lambda → sqs/sns: DLQ (lambda_related.go:130-146, lambda_related_extra.go:270-303)
   - logs → lambda: `LoggingConfig.LogGroup` and subscription consumers (logs_related.go:20-45)
   - ng → sg: `RemoteAccess.SourceSecurityGroups` (ng_related.go:167-177)
   - s3 → glue: crawler targets (s3_related.go:288-311)
   - opensearch → logs: `Enabled` filter (opensearch_related.go:33-37)

   These are rows 6, 10, 21, 27 and 57.
4. **The degraded-row flag doubles as list truncation.** `anyDegraded` is folded into `truncated` in related_fetch.go:41 and :65. It makes a one-to-one parent render `(1+)` (row 26, ng → eks). The same flag is also what gives ami → ng its only protection for degraded rows (row 25). A row whose sub-read failed without degrading, such as ng `image_id` after a failed LT read, gets no marker at all. This is the same shape as backlog row 11 (alarm → eks `0+`).
5. **CloudTrail lookup with no ARN spelling.** Types keyed on a non-ARN ID that store no `Fields["arn"]` get no `ctAltName` retry: kms (catalog_secrets.go:151, kms.go:102-108; row 1) and r53 (catalog_dns_cdn.go:42; row 47 side question). The alt-spelling mechanism exists (related.go:741-770), but these rows never feed it.
