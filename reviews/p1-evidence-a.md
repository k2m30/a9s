| F# | type | verdict | current file:line | symptom (LIVE only) | class (LIVE only) |
|----|------|---------|-------------------|---------------------|-------------------|
| F01 | acm | LIVE (narrower: key-type half FIXED, ACME half live) | core/aws/acm.go:88-94 | Certificates issued through ACME (CertificateKeyPairOrigin=ACME) never appear in the ACM list, so their expiry and validation findings and their share of the menu count are missing | default server-side list filter left in place |
| F02 | apigw | FIXED | core/iampolicy/policy.go:59-75; core/aws/apigw_issue_enrichment.go:225-236 | — | — |
| F03 | asg | FIXED | core/aws/asg_related.go:154-159 | — | — |
| F04 | athena | LIVE | core/aws/athena_related.go:71-83 | Most workgroups show "Log Groups: 1" and the drill opens a made-up `/aws/athena/<wg>` group. A Spark workgroup with a configured log group shows 0 unless metrics are on, and then it shows the made-up name | pivot reads the wrong API field and synthesises an ID |
| F05 | backup | FIXED | core/aws/backup_match.go:31-70 | — | — |
| F06 | backup | FIXED | core/aws/backup_match.go:36-39 | — | — |
| F07 | dbc | LIVE (narrower: the two status tables are now one; the terminal states are still missing) | core/aws/dbc.go:139-143, 166-175, 216-218 | A cluster in `upgrade-failed`, `migration-failed`, `cloning-failed`, `inaccessible-encryption-credentials-recoverable` or `stopped` shows as a yellow "<status>: in progress" row with "wait for it to return to available" text, and is not counted as broken | incomplete status enumeration; terminal states fall to a default branch |
| F08 | dbi | LIVE | core/aws/rds.go:156-163, 176-186, 222-226 | An instance in `inaccessible-encryption-credentials-recoverable`, `insufficient-capacity` or `incompatible-create` gets no lifecycle finding. Its row is coloured only by posture findings (healthy or yellow) and it is not in the broken count | incomplete status enumeration; terminal states fall to a default branch |
| F09 | eb-rule | FIXED | core/aws/eb_rule.go:109-176, 178-180 | — | — |
| F10 | eb-rule | FIXED | core/aws/eb_rule_issue_enrichment.go:89, 113-129 | — | — |
| F11 | ecr | LIVE | core/aws/ecr_issue_enrichment.go:136-158; core/aws/ecr_images.go:129-130, 178-179, 202-203 | A repository whose images were scanned by the current AWS-native basic scanning shows Critical 0 / High 0 and no vulnerability finding, even when an image has critical CVEs | an unread value shown as a proven zero (the field AWS no longer fills) |
| F12 | ecs-svc | FIXED | core/aws/ecs_svc.go:37-46, 119 | — | — |
| F13 | ecs-svc | FIXED | core/aws/ecs_svc.go:99, 126-128; core/aws/ecs_svc_issue_enrichment.go:96, 217, 235 | — | — |
| F14 | ecs-svc | LIVE | core/aws/ecs_svc_logs.go:78-81 | `L` on a long-running service shows the oldest 200 lines the log group still keeps, which can be weeks old, instead of the current failure | oldest-first log window |
| F15 | ecs-task | FIXED | core/aws/ecs_task.go:42-50, 157 | — | — |
| F16 | ecs-task | FIXED | core/aws/ecs_task_related_extra.go:25-58 | — | — |
| F17 | ecs | FIXED | core/aws/ecs.go:43-46 | — | — |
| F18 | lambda | LIVE (premise from the SDK doc; live AWS not sampled) | core/aws/lambda.go:91-92, 110-121 | The Status column is always blank. A Pending or Failed function, or one whose last update failed, shows as healthy or only "no dead-letter queue". The broken count misses it | unrequested fields: the list call does not return what the code reads |
| F19 | lambda | LIVE | core/aws/catalog_compute.go:24-43 | A function on python3.8/3.9, nodejs16.x/18.x, dotnet6/7 and similar runtimes shows green, with no "runtime is end-of-life" finding | stale hard-coded AWS catalog |
| F20 | opensearch | LIVE | core/aws/opensearch.go:127-131, 258-274 | When the account/region has 6 or more domains, every domain is a name-only "details unavailable" row with no findings and blank Engine/Instance/Endpoint columns | batch size limit of a describe call not respected |

### F01 acm — LIVE (narrower)

- Key types: FIXED. `core/aws/acm.go:93` `Includes: &acmtypes.Filters{KeyTypes: acmtypes.KeyAlgorithm("").Values()}` (commit d2b7399d). `acm.weak-key` can fire now.
- ACME origin: still live. There is no `CertificateKeyPairOrigins` anywhere in core/aws. SDK acm@v1.50.0 `api_op_ListCertificates.go:19-22`: "By default, this action does not return certificates with a CertificateKeyPairOrigin of ACME. To include ACME certificates, specify ACME in the CertificateKeyPairOrigins filter." Also `:41-43`: default filtering returns only AWS_MANAGED and CUSTOMER_PROVIDED.
- Side note: the comment at `acm.go:90-92` says unfiltered returns "only RSA_1024 and RSA_2048". The SDK doc (`:16-17`) says "only RSA_2048".
- Siblings: none on this code path.

### F02 apigw — FIXED

- `core/iampolicy/policy.go:65-68`: `Decode` unquotes the JSON-string-literal form (`json.Unmarshal([]byte(`"`+doc+`"`), &unquoted)`). `json.Unmarshal` handles `\/` as a valid escape. The commit is f52aca8e.
- `apigw_issue_enrichment.go:233-234`: `case policyErr != nil:` now leaves exposure unknown, so no finding.

### F03 asg — FIXED

- `asg_related.go:156-158`: `refs = append(refs, tg.LoadBalancerArns...)` followed by `resolveRefs("elb", refs, refContext(...))`. That goes through the elb `RefToID` (`catalog_networking.go:92`, `elbRefToID`) to the row ID. Classic names are no longer emitted (the comment at `:127-128`). The commit is 647694aa.

### F04 athena — LIVE

- `athena_related.go:76-81`: `if cfg.PublishCloudWatchMetricsEnabled == nil || !*cfg.PublishCloudWatchMetricsEnabled { return resource.ProvenZero(...) }` … `lg := "/aws/athena/" + res.ID`.
- SDK athena@v1.66.0 `types/types.go:4341` `WorkGroupConfiguration.MonitoringConfiguration`. `:712-721` `CloudWatchLoggingConfiguration{Enabled *bool; LogGroup *string}` ("The name of the log group in Amazon CloudWatch Logs where you want to publish your logs"). The code never reads it.
- Siblings: none checked beyond this file.

### F05 backup — FIXED

- `backup_match.go:31-36`: `BackupSelectionCovers` evaluates "(Resources match OR ListOfTags match) AND every Conditions clause AND NOT NotResources match" per selection. `BackupPlanSelections` reads the structured `BackupPlanRow.Selections`. The commit is 7d2ace8d.

### F06 backup — FIXED

- `backup_match.go:37`: `if backupARNsMatch(sel.NotResources, arn, false)` applies to the selection's own NotResources. There is no plan-wide `not_resources` field left. The commit is 7d2ace8d.

### F07 dbc — LIVE (narrower)

- `dbc.go:167-171`: `brokenCode` has only `failed`, `inaccessible-encryption-credentials` and `incompatible-parameters`. `dbc.go:216-218`: "Unknown status — bare keyword passthrough", `wave1Finding(CodeDBCTransitional, c.Status)`. The catalog is at `catalog_databases.go:383`: Phrase "<status>: in progress", SevWarn, "Wait for it to return to available".
- One `computeDBClusterFindings` now serves both DocumentDB and RDS (`dbc_rds.go:21-28`), so the "merge the two tables" half is done.
- None of `upgrade-failed`, `migration-failed`, `cloning-failed`, `inaccessible-encryption-credentials-recoverable` or `stopped` appears anywhere in core/. The SDK types status as a free string. The status list comes from the Aurora User Guide, not from the SDK. I could not confirm `upgrade-failed` as a documented *cluster* status from the local SDK.
- Siblings: `core/aws/rds.go:222-226` (dbi) handles unknown status the opposite way, with no finding at all. The two lanes also disagree on `stopped`: it is Broken on dbi (`rds.go:185`) and "in progress" on dbc.

### F08 dbi — LIVE

- `rds.go:176-186`: `brokenMap` has no `inaccessible-encryption-credentials-recoverable`, `insufficient-capacity` or `incompatible-create`. `rds.go:156-163`: `transitionalStatusSet` has no `delete-precheck`, `storage-config-upgrade` or `storage-initialization`. `rds.go:222-226`: unknown status returns `postureFindings` only.
- The status list comes from the RDS User Guide, from memory, not verified locally. I am not certain that `upgrade-failed` is a documented *instance* status.
- Siblings: dbc (F07), with the same missing-terminal-state shape and the opposite unknown-status fallback.

### F09 eb-rule — FIXED

- `eb_rule.go:114-143` `ebRuleBuses` pages `ListEventBuses`. `eb_rule.go:147-176` runs `ListRules` per bus with `EventBusName`. `eb_rule.go:178-180` `ebRuleID` = `bus/name`. The enricher passes `EventBusName` (`eb_rule_issue_enrichment.go:65`), and the related panel does too (`eb_rule_related.go:47,119`). The commit is 8cf372f9.
- Not fixed, dev tool only: `cmd/snapshot/messaging.go:243` still calls `ListRules` with only `NextToken` (default bus).

### F10 eb-rule — FIXED

- `eb_rule_issue_enrichment.go:89`: `noTargets := fetchErr == nil && ...`. `:113-118`: on `fetchErr != nil`, `targetCountStr = ""` and `MarkSkipped`. The commits are 6a6432a6 and bab2d28f.

### F11 ecr — LIVE

- `ecr_issue_enrichment.go:138-142`: `summary := img.ImageScanFindingsSummary; if summary == nil { continue }`. `:154-157` writes `critical_vulns`/`high_vulns` as `FormatInt(0)` even when no image had a summary.
- SDK ecr@v1.66.0 `api_op_DescribeImages.go:21-22`: "The new version of Amazon ECR Basic Scanning doesn't use the ImageDetail$imageScanFindingsSummary and ImageDetail$imageScanStatus attributes … Use the DescribeImageScanFindingsAPI instead."
- `ECRDescribeImageScanFindingsAPI` is declared (`ecr_interfaces.go:45`) but only in the interface. No production caller exists.
- Siblings: `ecr_images.go:129-130` (`finding_counts`), `:178-179` and `:202-203` (child-view status and findings).

### F12 ecs-svc — FIXED

- `ecs_svc.go:38-46`: `ListServices` with `MaxResults: 100, NextToken: token` through `parentChildWalk`. `ecs_svc.go:119`: `batch: 10` for DescribeServices. The commit is 647694aa.

### F13 ecs-svc — FIXED

- `ecs_svc.go:99` `ID: ecsSvcID(clusterName, serviceName)`, and `:126-128` returns `cluster + "/" + serviceName`. The enricher keys on `ecsSvcID(...)` (`ecs_svc_issue_enrichment.go:96,120,145`) and writes findings on `row.ID` (`:217,235`). The pipeline producer uses `ecsSvcID(a.Configuration["ClusterName"], name)` (`pipeline_related.go:197`). The task and eip producers go through `ecsSvcRefFromTask` and `relatedRefs`. The commit is 8cf372f9.

### F14 ecs-svc — LIVE

- `ecs_svc_logs.go:78-81`: `FilterLogEventsInput{LogGroupName: &logGroup, NextToken: nextToken}`. There is no `StartTime`, no `StartFromHead` and no `Limit`. The `:20-25` comment confirms "no start-time bound".
- SDK cloudwatchlogs@v1.88.0 `api_op_FilterLogEvents.go:48-50`: "By default, the events are returned in ascending timestamp order (oldest first). To return events in descending timestamp order (newest first), set the startFromHead parameter to false." `:148-149`: startFromHead=false requires startTime ≥ 2024-01-01.
- Siblings: `core/aws/lambda_invocations.go:69-75` is a narrower form. It is bounded to the last 24h (`invocationLookbackHours = 24`) with `Limit` 50, but it is still ascending, so a function with more than 50 invocations in 24h shows the oldest 50 of the window. `lambda_invocation_logs.go:44` filters by request ID, which is not the same shape.

### F15 ecs-task — FIXED

- `ecs_task.go:43-47`: `ListTasks` with `MaxResults: 100, NextToken: token` through `parentChildWalk`. `:157`: `batch: 100`. The commit is 647694aa.

### F16 ecs-task — FIXED

- `ecs_task_related_extra.go:41-57`: `DescribeContainerInstances(cluster, [ContainerInstanceArn])` → `ci.Ec2InstanceId` → `resolveRefs("ec2", …)`. The commit is 8cb26722.

### F17 ecs — FIXED

- `ecs.go:43-46`: `DescribeClustersInput{..., Include: []ecstypes.ClusterField{ecstypes.ClusterFieldConfigurations, ecstypes.ClusterFieldTags}}`. The commit is 647694aa.

### F18 lambda — LIVE

- `lambda.go:91-92`: `"state": string(fn.State)` and `"last_update_status": string(fn.LastUpdateStatus)` come from the ListFunctions item. `:110-121`: the lifecycle switch reads `fn.LastUpdateStatus` and `fn.State`.
- SDK lambda@v1.108.0 `api_op_ListFunctions.go:18-21`: "The ListFunctions operation returns a subset of the FunctionConfiguration fields. To get the additional fields (State, StateReasonCode, StateReason, LastUpdateStatus, …) … use GetFunction."
- GetFunction is called only in detail enrichment (`lambda_detail_enrichment.go:58`, detail view only, and it does not write list fields), in the absence probe (`lambda_issue_enrichment.go:200`) and in a related check (`lambda_related.go:234`). None of them writes `state` or `last_update_status`, and none emits lifecycle findings. I did not sample live ListFunctions output.

### F19 lambda — LIVE

- `catalog_compute.go:24-43`: the set stops at nodejs14.x, python3.7, ruby2.7, dotnetcore3.1, java8 and go1.x. None of the named identifiers is present. The SDK enum confirms the identifiers exist (`types/enums.go:832-834,857,860,864,868-869`).
- The SDK carries no deprecation dates. The per-runtime dates come from my knowledge of the AWS "Deprecated runtimes" table, not from a local source. I am confident about nodejs16.x, python3.8, dotnet6, dotnet7, dotnet5.0, provided and nodejs4.3-edge. I am less certain that nodejs20.x, provided.al2, ruby3.2 and python3.9 are past their deprecation date as of 2026-09-23, and I did not verify it. `java8.al2` may also belong on the list.

### F20 opensearch — LIVE

- `opensearch.go:127-131`: one `DescribeDomains(DomainNames: domainNames)` with every listed name. On error, `:264-273` turns every name into `DegradedDetails`.
- The SDK doc comment (`api_op_DescribeDomains.go:31-34`) does not state the 5-name cap. The cap comes from the AWS API reference ("Array Members: Maximum number of 5 items"), which I am citing from memory, not from a local file. The repo encodes the same cap in `cmd/snapshot/ops.go:1085-1089` (`start += 5`) and in `docs/resources/opensearch.md` (per the finding).
- Siblings: none found on this code path. `ecs.go:43` passes one `ListClusters` page to `DescribeClusters`; both default to 100, so it is within the limit.
