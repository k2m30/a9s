# Slice A — related panel / navigation triage (main @ ad18304c)

Types: acm alarm ami apigw asg athena backup cb cf cfn codeartifact ct-events dbc-snap dbc dbi-snap dbi ddb eb-rule

Sources abbreviated: C = reviews/review-claude.md, X = reviews/review-codex.md, BL = reviews/backlog.md, OL = oneliners "rejected" lists. All paths under `core/aws/` unless shown otherwise.

## 1. Findings

| # | type | source | verdict | class | current file:line | evidence |
|---|---|---|---|---|---|---|
| 1 | acm | C acm P2#2 "apigw pivot emits domain names"; X acm P2 acm_related:133; C apigw P2#9; X apigw P2#8 | FIXED | — | acm_related.go:120-127; ref_ids.go:501-520 | InUseBy ARNs go through `apigwRefToID`, which gives no ID for `/domainnames/`; the ref is dropped, so the count is a lower bound and no domain name is emitted as an ID |
| 2 | acm | C acm P3#3 "ELB pivot counts Classic LBs" | LIVE | CONTRACT | acm_related.go:100-104; ref_ids.go:416-431; elb.go:17 | `elbRefToID` accepts a classic `loadbalancer/<name>` (one path part), and `relatedRefs` never checks the list. The elb list is ELBv2 only, so a cert on a CLB listener shows (1) and the drill opens empty |
| 3 | acm | C acm P3#4 "r53 no label boundary"; X acm P2 acm_related:205 | FIXED | — | acm_related.go:193; predicates.go:98-101 | `dnsZoneHosts`: equality or `HasSuffix(name, "."+zone)` |
| 4 | acm | C acm P3#5 "r53 should fall back to DomainName" | DISPROVED | — | docs/resources/acm.md:49 | The spec now defines r53 discovery as the validation record's innermost public zone only. The `ProvenZero` at acm_related.go:166-168 matches the spec |
| 5 | acm | X acm P2 apigw_related:255 "apigw→acm returns UUID segment" | FIXED | — | apigw_related.go:223-230; ref_ids.go:203 | Keeps the full CertificateArn; `acmRefToID` = `arnWholeRef` |
| 6 | acm/apigw | X acm P2 GetDomainNames/GetApiMappings paging; X apigw P2#4; C apigw P2#7 (domain/mapping part) | FIXED | — | apigw_related.go:189-215 | Both calls use `PageAll`, and `!complete` is carried into `relatedResultTrunc` |
| 7 | acm | X acm P2 r53_related:306 "validation record names as ACM IDs" | FIXED | — | r53_related.go:252-285 | Validation CNAMEs map to a domain, which is matched against the loaded ACM summaries; `cert.ID` is emitted |
| 8 | alarm | C alarm P2#1 "CloudTrail pivot lists every alarm's events"; X alarm P2 :182; X ct-events P1#1 | FIXED | — | catalog_monitoring.go:63,102; install.go:18-27 | `ctEventsCheckerFor("alarm")` builds `ResourceName:ID` from CloudTrailKey |
| 9 | alarm | C alarm P2#2 "ECS/EKS misattribution"; X alarm P2 ecs/eks | FIXED | — | alarm_match.go:141-150,230,235 | Namespace match is now exact (`AWS/ECS`, `ECS/ContainerInsights` vs `AWS/EKS`, `ContainerInsights`) |
| 10 | alarm/asg | C alarm P2#3 "ASG pivot ignores scaling-policy actions"; C asg P2#3 | FIXED | — | alarm_match.go:188-200,221 | The asg spec has `ActionService: "autoscaling"`; `alarmActionTarget` takes the name after `autoScalingGroupName/`, in both directions |
| 11 | alarm | C alarm P2#6 "dimension pivots don't check target exists" | FIXED | — | alarm_related_extra.go:14-80; related_common.go:302-330 | `alarmRowsNaming` emits only rows of the loaded target list, unknown with no list |
| 12 | alarm/apigw | C alarm P3#7 (apigw); C apigw P2#8; X alarm P2 ApiName; X apigw P2#6 (alarm→apigw) | FIXED | — | related_common.go:322-330; alarm_match.go:220 | Emits `row.ID`; `ApiName` is matched against the row's Name |
| 13 | apigw | C apigw P2#6 "apigw→alarm misses REST alarms"; X apigw P2#6 (apigw→alarm) | FIXED | — | apigw_related.go:246-248; alarm_match.go:220 | DimensionNames `ApiId`,`ApiName`; the values are ID and Name |
| 14 | alarm | C alarm P3#7 (waf); X alarm P2 waf_related/WebACL | LIVE | MATCH | alarm_match.go:254 | The waf spec has no `Values`, so it matches the `WebACL` dimension against ID/Name. The AWS waf-metrics doc says `WebACL` = "The metric name of the WebACL" (VisibilityConfig.MetricName); an ACL whose metric name differs from its name is never linked |
| 15 | alarm | C alarm P3#8 "metric-math alarms proven zero" | FIXED | — | alarm_match.go:36-51 | `AlarmDimensions` also collects `Metrics[].MetricStat.Metric` dims with their namespace |
| 16 | alarm | X alarm P2 eb_related:185 "eb→alarm substring" | FIXED | — | eb_related.go:156-158; alarm_match.go:227 | Goes through the shared namespace+dimension table |
| 17 | alarm | BL 11 "every alarm renders EKS Clusters (0+)" | LIVE | CARD | related_fetch.go:41,64-65 | `anyDegraded` marks the list truncated whenever one row is degraded, and `FetchIsPartial` folds per-item detail failures into "subset". An ID/Name-only match still renders 0+ |
| 18 | ami | C ami P2#1 "KMS pivot always 0" | LIVE | CALL | ami_related_extra.go:48-60 | Reads `BlockDeviceMappings[].Ebs.KmsKeyId`, which DescribeImages does not return (EbsBlockDevice doc). `refs` is empty, so kmsRelated gives an exact 0 for an encrypted AMI. Needs the snapshot's KmsKeyId |
| 19 | ami | X ami P2 "ami→kms raw ARN/alias IDs" | FIXED | — | ami_related_extra.go:59; ref_ids.go:303-371 | Goes through `kmsRelated`: ARN to key ID, alias resolved or dropped (lower bound) |
| 20 | ami | C ami P2#2 + X ami P1 "ami→asg via running instances" | LIVE | CONTRACT | ami_related.go:67-121 | Still matches ASG `Instances[]` against ec2 `image_id`; an ASG at desired=0 never matches. A nil ec2 list continues with an empty map and returns a resolved count (:85-90) |
| 21 | ami | C ami P3#4 + X ami P1 "disabled AMIs missing, pivots end in not found" | FIXED | — | ami.go:50,83 | `IncludeDisabled: true` on both inputs |
| 22 | ami | C ami P3#5 "cfn pivot proven 0 with no search" | LIVE | CARD | ami_related_extra.go:27-28 | `ProvenZero("cfn")` when the stack-name tag is absent; docs/resources/ami.md:138 says no count |
| 23 | ami/asg | X ami P1 "$Latest instead of $Default"; C asg P3#6 | FIXED | — | asg_related.go:308-309; ng_related.go:196; eks_related_extra.go:278 | One `launchTemplateVersions` helper, defaulting to `$Default` |
| 24 | ami/asg | X ami P2 mixed-instances overrides; C asg P3#7; X asg P2 overrides | LIVE | CONTRACT | asg_related.go:106-109,231-234; asg_related_extra.go:103-106 | Only `MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification` is read; `Overrides[].LaunchTemplateSpecification` is ignored and the result is exact (`false`) |
| 25 | ami | X ami P2 "resolve:ssm: emitted as AMI ID" | LIVE | ID | asg_related.go:117-119; ng_related.go:205-207; eks_related_extra.go:289-291 | Raw `LaunchTemplateData.ImageId` goes to `relatedResultTrunc("ami", …)` with no resolver, so `resolve:ssm:/…` becomes an AMI ID |
| 26 | apigw | C apigw P2#2; X apigw P1#2; BL 1 "REST APIs sent to apigatewayv2" | LIVE | CALL | apigw_related.go:127-143 (used at :31,:153,:296,:414) | No protocol branch: v2 `GetIntegrations`/`GetAuthorizers` run for REST IDs, and the lambda/kms/elb/role pivots error on every REST API |
| 27 | apigw | C apigw P2#5 "elb pivot matches LBs sharing subnet/SG" | LIVE | MATCH | apigw_related.go:365-393 | Still intersects the VPC link's subnets/SGs with every LB; `IntegrationUri` (listener ARN) is not read |
| 28 | apigw | C apigw P2#7 (integrations/authorizers); X apigw P2#3; X apigw P2#5 | FIXED | — | apigw_related.go:136-142,442-448 | `PageAll`, and completeness is carried |
| 29 | apigw | C apigw P2#10; X apigw P2#7 "lambda→apigw by name substring, skips REST" | LIVE | MATCH | lambda_related_extra.go:76-92 | `strings.Contains(api.Name, fnName)` plus a tag keyed by the function name. REST rows fail `assertStruct[apigwtypes.Api]` and are skipped without truncation (:78-81) |
| 30 | apigw | C apigw P2#11; X apigw P2#9 "r53→apigw emits d-xxxx" | FIXED | — | r53_related.go:188 | `listedRefs` keeps only IDs the apigw list holds; `d-…` names no row, so the count is a lower bound and no bogus ID is emitted |
| 31 | apigw | C apigw P3#14 "logs prefix no terminator" | FIXED | — | apigw_related.go:113-119 | Exact `/aws/apigateway/<name>` or the `…/` path boundary |
| 32 | apigw | C apigw P3#15 "stage-variable placeholders / cross-account lambdas" | LIVE | ID | parse.go:125-133; apigw_related.go:161-165 | Cross-account is fixed (`localARN` rejects foreign ARNs). `…:function:${stageVariables.fn}` still parses as a lambda ARN, and `resolveRefs` (no list check) counts `${stageVariables.fn}` as a function ID |
| 33 | asg | C asg P1#1; X asg P1 "asg→elb emits LB ARNs" | FIXED | — | asg_related.go:157-161; ref_ids.go:416-431 | `elbRefToID` maps an ELBv2 ARN to its name; classic names are not read |
| 34 | asg | C asg P2#4; X asg P2 "ecs→asg lists every ECS-managed ASG" | LIVE | MATCH | ecs_related_extra.go:43-47 | Any ASG with the `AmazonECSManaged` tag key is counted under every cluster |
| 35 | asg | C asg P3#8; X asg P2 "instance-profile path" | FIXED | — | ref_ids.go:522-532; asg_related.go:268 | `instanceProfileName` takes the last `/` segment |
| 36 | asg | BL 3 "asg→ng renders (1+)" | LIVE | CARD | asg_related.go:54; related_fetch.go:41 | A degraded ng row makes the list truncated, so the one-to-one pivot shows a lower bound |
| 37 | athena | C athena P1#1; X athena P2 "logs pivot reads metrics flag" | FIXED | — | athena_related.go:67-87 | Reads `MonitoringConfiguration.CloudWatchLoggingConfiguration.{Enabled,LogGroup}` |
| 38 | athena | C athena P2#4; X athena P2 athena.go:64 "s3→athena proven 0 on GetWorkGroup failure" | FIXED | — | s3_related.go:269-272; athena.go:67-68 | An unread config makes the count a lower bound |
| 39 | athena | C athena P3#5; X athena P2 "KMS ignores other keys" | LIVE | CONTRACT | athena_related.go:52-62 | Only `ResultConfiguration.EncryptionConfiguration.KmsKey` is read; CustomerContent and ManagedQueryResults keys are ignored (spec §2 kms) |
| 40 | athena | C athena P3#6; X athena P2 "glue→athena name equality" | FIXED | — | docs/related-resources.md:117; catalog_data.go (glue row) | Pivot removed; glue's contract row has no athena and there is no athena checker in glue_related |
| 41 | athena | X athena P3 "same GetWorkGroup per checker" | LIVE | OTHER | athena_related.go:18-33 (called :38,:52,:70,:94) | 4 GetWorkGroup calls per detail open, with no `RetryOnThrottle`; a throttle turns each pivot into unknown |
| 42 | backup | C backup P1#1,#2; X backup P1#1,#2, P1#4, P2#5 "flattened selections / NotResources / selector-less / exclusions / `=` in tag key" | FIXED | — | backup_match.go:36-70,108-136 | One per-selection evaluator on structured `BackupSelection`: `(Resources∪ListOfTags) ∧ Conditions ∧ ¬NotResources`; empty means all; tags are a map |
| 43 | backup | X backup P2#6; C ddb P2#2; C dbi-snap P2#1; X dbi-snap P2#3,#4; X dbc-snap P2 "tag-only / partial plans give proven 0" | FIXED | — | backup_match.go:108-114,153-171 | Unread tags or a partial plan is "undecided", which gives unknown (nothing matched) or a lower bound, never proven 0 |
| 44 | backup | BL 2 "backup pivots never read resource tags" | LIVE | CARD | ddb_related.go:48; s3_related.go:347; dbi_snap_related.go:124; dbc_snap_related.go:140 | `backupTarget{unread: …}`: any tag-selected plan leaves the answer unknown or a lower bound where one tag read would prove it |
| 45 | backup | X backup P2#7,#8 "ebs ARN selections / ebs-snap exact ARN" | FIXED | — | ebs_related.go:128-132; ebs_snap_related.go:130 | Both go through `backupPivot` |
| 46 | backup | C backup P3#4; X backup P2#9 "backup→role ListBackupSelections one page" | FIXED | — | backup_related.go:30-49 | `PageAll` and `!complete` |
| 47 | cb | C cb P2#1 "SECRETS_MANAGER ARN/json-key refs" | FIXED | — | cb_related.go:198-204; secrets_related.go:~186-196; ref_ids.go:385-414 | Both directions use `secretsRefToID` (cuts `:json-key`, strips `-XXXXXX` / matches the row ARN) |
| 48 | cb | C cb P3#9; X cb P2#7 "ECR pivots ignore registry / substring" | FIXED | — | predicates.go:26-39; cb_related.go:147; ecr_related.go:115 | `imageRefersToRepo`: exact `registry/repo` after stripping tag or digest, in both directions |
| 49 | cb | C cb P3#10 "S3 pivot skips SecondarySources / S3Logs" | LIVE | CONTRACT | cb_related.go:158-170 | Reads Artifacts, SecondaryArtifacts and Source only; docs/resources/cb.md:67 also names `SecondarySources[].Location` and `LogsConfig.S3Logs.Location` |
| 50 | cb | X cb P2#8 "build-log drill with empty stream" | FIXED | — | catalog_cicd.go:386-389 | `DrillCondition` requires both `log_group_name` and `log_stream_name` |
| 51 | cf | C cf P2#2; X cf P2 "Lambda@Edge `return` on duplicate" | FIXED | — | cf_related.go:246-253 | The collector appends every item; `relatedRefs` de-duplicates |
| 52 | cf | C cf P2#3; X cf P2 "Log Groups pivot returns S3 log bucket" | FIXED | — | cf_related.go:35-37; catalog_dns_cdn.go:117-124 | No cf→logs pivot; the log bucket joins the s3 pivot |
| 53 | cf | C cf P2#4 (acm, alarm part) "session region vs us-east-1" | FIXED | — | cf_related.go:157-172; alarm_match.go:223 | acm reads `relatedListIn(certRegion)`; alarm has MetricsRegion us-east-1 |
| 54 | cf | C cf P2#4 (lambda part); OL-claude "us-east-1 Lambda client for CF" | LIVE | CALL | cf_related.go:258; ref_ids.go:129-131 | Lambda@Edge ARNs are us-east-1, but resolution uses the session-region context. Outside us-east-1 `localARN` rejects every ref, so the pivot always shows 0+ with no rows |
| 55 | cf | C cf P3#7; X cf P2 waf_related:141 "ListDistributionsByWebACLId one page" | FIXED | — | waf.go:48-62 | `PageAll` on `NextMarker` |
| 56 | cf | X cf P3 "rows restored without RawStruct give unknown" | DISPROVED | — | cf_related.go:25-28,117-120,146-149; related_common.go:41-46 | Unknown is the declared state for a source row that was not read; no wrong count or ID is produced |
| 57 | cfn | X cfn P3 "NotificationARNs navigation unreachable" | FIXED | — | core/config/defaults_cicd.go:13; core/semantics/projection/generic.go:219-228 | The field is in the default view, and scalar-list ARN entries are navigable without a colon split |
| 58 | codeartifact | C codeartifact P2#1; X codeartifact P1 "row ID is bare repo name" | FIXED | — | codeartifact.go:68 | `codeArtifactRepoID(domainName, repoName)` |
| 59 | codeartifact | C codeartifact P2#2; X codeartifact P2 (IDs) "secrets→codeartifact returns secret names/tags as IDs" | FIXED | — | secrets_related_extra.go:43-57 | Emits `repo.ID` of loaded rows, marked heuristic |
| 60 | codeartifact | X codeartifact P2 (label part) "panel says Domains, targets repos" | LIVE | OTHER | catalog_secrets.go:82 | DisplayName is still "CodeArtifact Domains" for the repository type |
| 61 | codeartifact | C codeartifact P3#3; X codeartifact P3 "pipeline→codeartifact nonexistent provider" | FIXED | — | docs/related-resources.md:131; catalog_cicd.go:140-160 | Pivot removed; the pipeline contract row has no codeartifact |
| 62 | codeartifact | X codeartifact P2 catalog_cicd:297 "ct-events filter by ResourceName misses package events" | LIVE | MATCH | catalog_cicd.go:297-301 | LookupEvents `ResourceName=repo_name`. The AWS CodeArtifact CloudTrail doc sample (ReadFromRepository) carries the repo only in `requestParameters.repositoryName` with `domainName`, while the qualifier path is `requestParameters.domain` |
| 63 | ct-events | C ct-events P2#1 "cross-account role resolves to local role" | FIXED | — | ct_events_related.go:88-117; core/semantics/ctevent/target.go:127-128,276-280; sections.go:110-116 | The issuer ARN is preferred and read through the role resolver (account-checked); TARGET/ACTOR rows of another account are not navigable |
| 64 | ct-events | C ct-events P2#2 "AccessKeyId pivot suppressed for Root" | FIXED | — | ct_events_related.go:561-579 | Gated only on a non-empty `accessKeyId` |
| 65 | ct-events | C ct-events P3#7 "IAM Roles drops the caller when roleArn present" | LIVE | CONTRACT | ct_events_related.go:66-73 | `requestParameters.roleArn` returns first; docs/resources/ct-events.md:41 defines the role as `sessionIssuer.arn` (the caller) |
| 66 | ct-events | C ct-events P3#8 "SharedEventId drill can't answer" | FIXED | — | catalog_monitoring.go:274-276; docs/resources/ct-events.md:114-116 | Facet removed; the spec lists AccessKeyId/Username/EventName |
| 67 | ct-events | X ct-events P2#2 "substring CT matching in ecs/ecs-svc/ecs-task/ecr" | FIXED | — | install.go:18-27 | Every non-ct-events type's ct pivot is `ctEventsCheckerFor` (CloudTrailKey filter); no substring checkers remain |
| 68 | ct-events/dbi/dbc | X ct-events P2#3; C dbi P2#5; C dbc P3#5 "t key vs pivot use different values" | FIXED | — | catalog_databases.go:72,285; core/resource/related.go:726-744 | One CloudTrailKey (`ResourceName:ID`) for both paths, plus the `_altname` ARN retry |
| 69 | ct-events | X ct-events P2#4 "TARGET resource types not navigable" | LIVE | NAV | core/semantics/ctevent/target.go:157-173,186-189 | `AWS::Lambda::Function`, VPC, SG and Subnet are unmapped; every `ec2` ARN is labelled "Instance", so an SG or VPC ARN navigates to ec2 |
| 70 | ct-events | X ct-events P2#5 "S3 Object TARGET uses key as NavID" | LIVE | ID | core/semantics/ctevent/target.go:127-134 | The `resources[]` path: `AWS::S3::Object` value `bucket/key`, and the NavID after the first `/` is `key`. Only the requestParameters path (:344-347) uses the bucket |
| 71 | ct-events | BL 5 "no ct-events→ecr pivot" | LIVE | CONTRACT | catalog_monitoring.go:261-276; docs/related-resources.md:98 | No ecr registration, so `AWS::ECR::Repository` records are unreachable |
| 72 | dbc-snap | C dbc-snap P3#4 "Backup pivot ≠ spec's awsbackup:job- rule" | DISPROVED | — | docs/resources/dbc-snap.md:64-65 | The spec now defines the pivot as plans that select the parent cluster, which matches dbc_snap_related.go:140 |
| 73 | dbc-snap | X dbc-snap P3 "DBClusterIdentifier not navigable" | FIXED | — | catalog_databases.go:803 | `{FieldPath: "DBClusterIdentifier", TargetType: "dbc", Resolve: dbcSnapParentRow}` |
| 74 | dbc | X dbc P3 "dbc-snap→dbc labelled DocumentDB Cluster" | FIXED | — | catalog_databases.go:796 | "DB Cluster" |
| 75 | dbi-snap | X dbi-snap P1#1 "links use mutable names"; BL 12 "parent row navigable, opens nothing" | FIXED | — | dbi_snap_issue_enrichment.go:116-150; catalog_databases.go:703; core/app/detail_body.go:266-270 | Matches by `DbiResourceId`; the navigable field resolves through `dbiSnapParentRow` to the current row ID |
| 76 | dbi-snap | X dbi-snap P2#2; OL-codex "dbi-snap KMS alias" | FIXED | — | dbi_snap_related.go:51; ref_ids.go:300-354 | `kmsRelated`: an alias is resolved through the loaded row or DescribeKey |
| 77 | dbi-snap | C dbi-snap P3#4 "Backup pivot ≠ spec" | DISPROVED | — | docs/resources/dbi-snap.md:45-46 | The spec defines plan coverage of the parent instance |
| 78 | dbi | C dbi P2#4; X dbi P2 "ENI pivot counts other DBs' ENIs" | LIVE | MATCH | dbi_related.go:249-272 | Filter `description=RDSNetworkInterface ∧ group-id∈SGs` still returns every RDS ENI on shared SGs (now marked heuristic) |
| 79 | dbi | X dbi P2 ct_events_related:441 "ResourceType `DBInstance` not accepted" | DISPROVED | — | ct_events_related.go:373-383 | Matches `AWS::RDS::DBInstance` (the form LookupEvents returns; the same `AWS::<svc>::<type>` form is seen live in backlog.md:36) and falls back to `requestParameters.dBInstanceIdentifier` |
| 80 | ddb | C ddb P2#1 "lambda pivot resolves alias name" | FIXED | — | ddb_related.go:99-102; ref_ids.go:195 | `lambdaRefToID` cuts at the first `:` after `function:` |
| 81 | ddb | X ddb P2 "VPCE ignores Interface endpoints" | FIXED | — | ddb_related_extra.go:56 | Matches on the `.dynamodb` service-name suffix, for both endpoint types |
| 82 | ddb | X ddb P2 "logs pivot relates hand-named groups" | LIVE | OTHER | ddb_related_extra.go:18-38; docs/resources/ddb.md:61 | The spec premise is wrong: the DynamoDB Contributor Insights guide (contributorinsights_HowItWorks) says CI creates CloudWatch CI *rules*, not log groups, so only arbitrarily named groups match |
| 83 | ddb | X ddb P2 "alarm pivot namespace / metric-math" | FIXED | — | alarm_match.go:36-51,226 | `AWS/DynamoDB` namespace, and metric-query dims are included |
| 84 | ddb | X ddb P2 lambda_related_extra:162 "ListEventSourceMappings NextMarker" | FIXED | — | lambda_related.go:160-170 | `PageAll` on `NextMarker` |
| 85 | eb-rule | C eb-rule P2#3 "SNS pivot emits topic name" | FIXED | — | eb_rule_related.go:58-67; ref_ids.go:204 | `snsRefToID` keeps the full ARN |
| 86 | eb-rule | C eb-rule P2#4 "SQS pivot omits target DLQs" | LIVE | CONTRACT | eb_rule_related.go:58-66 | Only `Targets[].Arn` is read; docs/resources/eb-rule.md:67 requires `DeadLetterConfig.Arn` too |
| 87 | eb-rule | C eb-rule P3#7; X eb-rule P1 "empty EventBusName" | FIXED | — | eb_rule_targets.go:36-39; eb_rule_related.go:46-48 | Set only when non-empty |
| 88 | eb-rule | X eb-rule P2 "pipeline/sfn/sqs ListRuleNamesByTarget default bus, one page" | FIXED | — | eb_rule_related.go:101-139 | Per bus from `ebRuleBuses`, `PageAll`, bus-qualified row IDs |
| 89 | eb-rule | X eb-rule P2 ecr_related:237 "comparator patterns give proven 0" | LIVE | MATCH | ecr_related.go:266-275 | `[{"prefix":"prod-"}]` fails `[]string` unmarshal but sets `hasRepoFilter`, so the result is false (exact 0) |
| 90 | eb-rule | X eb-rule P2 s3_related:388 "raw substring on pattern" | LIVE | MATCH | s3_related.go:368-379 | `Contains(pattern, "aws.s3") && Contains(pattern, "\"bucket\"")`; any quoted field value equal to the bucket name matches |
| 91 | eb-rule | X eb-rule P2 ecs_svc_related_extra:197 "serviceName ignored; clusterArn substring" | LIVE | MATCH | ecs_svc_related_extra.go:188-217 | The clusterArn part is fixed (exact last segment). `detail.serviceName`/`service` and `resources` are not examined, so such a rule broad-matches every service |

## 2. Counts

- In scope: 91 rows, merged from 144 source findings (62 claude, 75 codex, 1 oneliner-rejected, 6 backlog rows: 1, 2, 3, 5, 11, 12).
- FIXED: 55
- DISPROVED: 5 (#4, #56, #72, #77, #79)
- LIVE: 31
  - ID 3 (#25, #32, #70)
  - MATCH 9 (#14, #27, #29, #34, #62, #78, #89, #90, #91)
  - PAGE 0
  - CALL 3 (#18, #26, #54)
  - CARD 4 (#17, #22, #36, #44)
  - NAV 1 (#69)
  - CONTRACT 8 (#2, #20, #24, #39, #49, #65, #71, #86)
  - OTHER 3 (#41, #60, #82)
- Out of scope: 121. This is 58 claude + 55 codex + 8 oneliner-rejected: list fetch, findings/severity, wave-2 posture, console links, child-list IDs, and list columns such as the ct-events TARGET column.

## 3. Shared shapes

- **Hand-parsed EventBridge patterns, one per source type.** ecr (ecr_related.go:246-292), s3 (s3_related.go:368-379) and ecs-svc (ecs_svc_related_extra.go:163-217) each parse `EventPattern` their own way: literal arrays only, raw substring, and a subset of detail keys. #89, #90 and #91 have one cause. One pattern evaluator (content filters, unknown keys make the result unknown) would close all three.
- **IDs emitted without the target resolver or list.** Launch-template `ImageId` goes raw into `relatedResultTrunc("ami")` at asg_related.go:117, ng_related.go:205 and eks_related_extra.go:289 (#25). `resolveRefs` accepts syntactically valid but impossible refs: the apigw lambda placeholder at apigw_related.go:164 (#32), and the classic ELB name that `elbRefToID` accepts although the elb list is ELBv2-only (#2). The fix is a resolver that rejects what the list can never hold (`ami-` prefix, `${`, classic form), or `listedRefs` at these sites.
- **Degraded row means the list is truncated.** `anyDegraded` (related_fetch.go:41,65) turns per-item detail failures into "subset". This causes #17 (alarm→eks 0+) and #36 (asg→ng 1+, and ng→eks from backlog 3 in another slice).
- **Launch source resolved from the base template only.** The asg ami, role and sg pivots (#24), and ami→asg (#20) matching running instances instead of the same launch-source resolver, are the "same fact computed two ways" pair.
- **Name or heuristic joins where AWS has a real link.** lambda→apigw uses the API name substring (#29), ecs→asg uses tag presence (#34), dbi→eni uses SG overlap (#78) and apigw→elb uses subnet/SG overlap (#27). Each has an authoritative field: integrations or the Lambda policy SourceArn, capacity-provider ASG ARNs, the ENI's attachment, and IntegrationUri.
- **Checker reads fewer source fields than its spec names.** athena kms (#39), cb s3 (#49), eb-rule sqs DLQ (#86) and the ct-events caller role (#65).
- **Region of a global service.** cf→acm and cf→alarm read us-east-1 (#53), but cf→lambda@edge still resolves against the session region (#54).
- **Identity is not the name AWS puts in the metric.** The waf `WebACL` dimension is VisibilityConfig.MetricName (#14). The same `AlarmMatchSpec.Values` hook other types use (ec2, elb, sns) would carry it.
