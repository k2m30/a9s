# Small, obvious findings from reviews/review-codex.md

Each row below was opened on the current tree (main @ 7609bbfa) and confirmed still present.

## Confirmed present

24 findings.

| id | where | what | fix | confidence |
|----|-------|------|-----|------------|
| `acm-list-includes` | `core/aws/acm.go:87` (`FetchACMCertificatesPage`); same in `cmd/snapshot/security.go:433` | `ListCertificatesInput` carries only `MaxItems`, so ACM's restrictive default applies: RSA-3072/4096, ECDSA and ACME-issued certificates never appear in the list or the snapshot. | Set `Includes` with every supported key algorithm and all three key-pair origins on both requests. | stated |
| `ami-include-disabled` | `core/aws/ami.go:78` (`FetchAMIsPage`), `core/aws/ami.go:48` (by-IDs), `cmd/snapshot/ec2_network.go:183` | `DescribeImages` excludes disabled AMIs unless `IncludeDisabled` is set; a disabled self-owned AMI is missing from the list and from every pivot that resolves it. `IncludeDeprecated` is already set on the by-IDs path — `IncludeDisabled` is not, anywhere. | Add `IncludeDisabled: aws.Bool(true)` to all three inputs. | stated |
| `cb-sortorder` | `core/aws/cb_issue_enrichment.go:57` | `SortOrder: DESCENDING` is sent to `ListBuildsForProject`; CodeBuild rejects the request once a project has over 100 builds, so latest-build status disappears for exactly the busy projects. Descending is already the API default. | Drop the `SortOrder` field. | stated |
| `cb-gitlab-source` | `core/aws/cb.go:130` (`cbContributorSourceTypes`) | The contributor-controlled-buildspec finding covers GitHub, GitHub Enterprise, Bitbucket and CodeCommit; GitLab and self-managed GitLab are absent, so a GitLab-backed project never gets the finding. | Add the SDK's `SourceTypeGitlab` and `SourceTypeGitlabSelfManaged` constants to the map. | stated |
| `cb-duplicate-ended` | `core/aws/cb_issue_enrichment.go:123` and `:139` | A failed build with an end time appends the same `Ended` row twice (once untiered, once with tier `!`) in one loop pass, so the failure detail panel prints the completion date twice. | Keep one `Ended` row, with the intended tier. | stated |
| `cb-log-seconds` | `core/aws/cb_build_logs.go:97` | `formatEpochMillisSec` — whose name and doc comment both say "including seconds" — formats with `2006-01-02 15:04`, so build log events in the same minute carry identical timestamps. | Use the `2006-01-02 15:04:05` layout. | stated |
| `cb-log-nav-guard` | `core/aws/catalog_cicd.go:387` | The build-logs child view is navigable when `log_group_name` alone is set; a build that has not reached provisioning has no stream, so selecting logs issues `GetLogEvents` with an empty stream name. | Require `log_stream_name` non-empty as well in the guard. | stated |
| `cb-inprogress-ok` | `core/aws/cb_issue_enrichment.go:107` | `IN_PROGRESS` and `STOPPED` are folded in with `SUCCEEDED` and rendered as `last_build: OK`, so the project list claims success for a build that is still running or was cancelled. | Reserve `OK` for `SUCCEEDED`; write the actual status for the other two. | stated |
| `ebrule-eventbusname` | `core/aws/eb_rule_targets.go:35` | `EventBusName` is always set from `parentCtx`, including when it is `""`. AWS treats an omitted bus as `default` but rejects an explicit empty value, so opening a default-bus rule's targets fails. | Set `EventBusName` only when non-empty. | stated |
| `ddb-vpce-interface` | `core/aws/ddb_related_extra.go:56` (`checkDdbVPCE`) | The VPC-endpoint pivot requires `type == "Gateway"`, so a DynamoDB `Interface` endpoint (PrivateLink) is missing from the table's Related Resources. | Accept `Gateway` and `Interface`. | stated |
| `ddb-billing-blank` | `core/aws/ddb.go:113` | `billingMode` stays `""` when `BillingModeSummary` is nil, which DynamoDB permits for a provisioned table never switched to on-demand — the Billing column renders blank instead of `Provisioned`. | Render `Provisioned` when the summary is absent. | stated |
| `ecs-svc-tags` | `core/aws/ecs_svc.go:48` (`DescribeServices` call) | The request has no `Include`, so ECS returns no tags: the service `Tags` field renders empty and the CloudFormation tag-based relation always misses. | Pass `Include: []ecstypes.ServiceField{ServiceFieldTags}`. | stated |
| `ecs-task-tags` | `core/aws/ecs_task.go:54`, `core/aws/ecs_svc_tasks.go:137` | Neither `DescribeTasks` request asks for `TAGS`, so both task detail views render tags as empty. | Pass `Include: []ecstypes.TaskField{TaskFieldTags}` on both. | stated |
| `ebs-snap-maxresults` | `core/aws/ebs_snap_issue_enrichment.go:78` | The public-share `DescribeSnapshots(RestorableByUserIds=all)` request sets no `MaxResults`, so the first call can return an unbounded page before the walker's page cap applies. | Set `MaxResults: aws.Int32(DefaultPageSize)`. | stated |
| `dbcsnap-label` | `core/aws/catalog_databases.go:772` | The `dbc-snap` related row is labelled "DocumentDB Cluster" although `dbc` is the merged type and the target can be an Aurora / Multi-AZ RDS cluster. | Rename the `DisplayName` to "DB Cluster". | stated |
| `dbcsnap-navigable-cluster` | `core/aws/catalog_databases.go:778` (`Navigable` for `dbc-snap`) | `DBClusterIdentifier` is rendered on the snapshot detail but only `VpcId` and `KmsKeyId` are registered navigable, so the operator cannot open the source cluster from the field. | Add `{FieldPath: "DBClusterIdentifier", TargetType: "dbc"}`. | stated |
| `ecs-task-windows-rootfs` | `core/aws/ecs_task_issue_enrichment.go:251` | The writable-root-filesystem rule fires for every container with no `RuntimePlatform` check; `readonlyRootFilesystem` is unsupported on Windows, so every Windows container is flagged with an impossible remediation. | Skip the rule when `RuntimePlatform.OperatingSystemFamily` is a Windows family. | stated |
| `ebs-volume-insufficient-data` | `core/aws/ebs_issue_enrichment.go:82`, finding set at `:108` | Every `VolumeStatus` other than `ok` raises `ebs.volume-io-degraded`, so a newly attached volume still reporting `insufficient-data` is shown as broken with a corruption warning. AWS documents `insufficient-data` as "checks may still be running". | Raise the degraded finding for `impaired` only; treat `insufficient-data` as pending. | stated |
| `waf-cf-nextmarker` | `core/aws/waf.go:47` (`wafDistributionIDs`), reported exact at `core/aws/waf_related.go:117` | `ListDistributionsByWebACLId` is called once and the result is returned with `truncated=false`, so a Web ACL on more distributions than one page holds reports a partial count as complete. | Follow `DistributionList.NextMarker` to exhaustion, or return the result as truncated. | stated |
| `backup-selections-pagination-snapshot` | `cmd/snapshot/ops.go:807` | `ListBackupSelections` is called without following `NextToken`, so captured backup metadata silently omits later selections. (The UI path at `core/aws/backup_related.go:31` already pages — this is the remaining site.) | Page through `NextToken`. | stated |
| `ctevent-s3-object-navid` | `core/semantics/ctevent/target.go:131-137` (`resourceRefToRow`) | `NavID` is derived by stripping everything before the first `/` or `:`, so an S3 Object ref `bucket/key` navigates to `key`. The s3 list is keyed by bucket, so the Object TARGET link opens nothing. | For the `Object` label, take the segment before the first `/` as `NavID`. | stated |
| `ecr-mutable-with-exclusion` | `core/aws/ecr.go:121` | The mutable-tag finding fires only on exact `MUTABLE`. `MUTABLE_WITH_EXCLUSION` also permits tag overwrites, so a repository in that mode gets no warning. The comment beside it only reasons about `IMMUTABLE_WITH_EXCLUSION`. | Include `ImageTagMutabilityMutableWithExclusion` in the condition. | stated |
| `sfn-arguments-taskdef` | `core/aws/ecs_svc_related_extra.go:399` | The ECS-service → Step Functions check reads only the legacy `Parameters` map of a `ecs:runTask` state; the current JSONata form puts the task definition under `Arguments`, so the relation reports zero. | Read `Arguments` as well as `Parameters`. | stated |
| `ecs-svc-logs-limit` | `core/aws/ecs_svc_logs.go:78` | `FilterLogEvents` is sent with no `Limit`, so a single call can return AWS's default 10,000 events although the view claims a 200-event cap. | Set `Limit` to the view's remaining capacity on the request. | inferred |

## Already fixed (dropped)

16 candidates were verified as fixed on the current tree and dropped.

| id | where the review pointed | why dropped |
|----|--------------------------|-------------|
| acm field keys | `catalog_dns_cdn.go:167` | `certificate_arn` and `key_algorithm` are both in ACM's `FieldKeys` now. |
| acm validation-zone suffix | `acm_related.go:205` | Matching goes through `dnsZoneHosts(zn, recordName)`, which enforces the label boundary. |
| asg `$Latest` default | `asg_related.go:124` | `asg_related.go:309` now uses `cmp.Or(version, "$Default")`. |
| asg instance-profile path | `asg_related.go:315` | `asgInstanceProfileToRoles` parses through `instanceProfileName(profileNameOrARN)`. |
| cf Lambda@Edge early return | `cf_related.go:301` | The collection is a `collect` closure over both behaviour lists; no early `return`. |
| cfn field keys | `catalog_cicd.go:91` | `termination_protection` and `output_secret` are both declared. |
| apigw `stages_count` | `apigw_issue_enrichment.go:268` | Written at `apigw_issue_enrichment.go:173`. |
| apigw integrations pagination | `apigw_related.go:143` | `apigwListIntegrations` uses `PageAll` with a page cap. |
| apigw domains / mappings pagination | `apigw_related.go:217`, `:232` | `checkApigwACM` pages `GetDomainNames` via `PageAll`. |
| backup selections pagination (UI) | `backup_related.go:30` | `checkBackupRole` pages `ListBackupSelections` by `NextToken`. |
| dbc empty RDS status | `dbc_rds.go:98` | `computeDBClusterFindings` returns posture only when `Status == ""`. |
| dbc docdb engine filter | `dbc.go:20` | Replaced deliberately by dedup-by-ID, documented at `dbc.go:250`. |
| ec2 `insufficient-data` severity | `catalog_compute.go:270` | Modelled as its own warning finding `ec2.instance-status.insufficient-data`. |
| iampolicy `PrincipalIsAWSService` | `evaluate.go:121` | Recognised as restrictive at `core/iampolicy/evaluate.go:344`. |
| athena cost cap column | `catalog_data.go:105` | `cost_cap` is populated by the fetcher at `athena.go:90`. |
| ct-events nondeterministic target | `ct_events_target_local.go:111` | That file no longer exists; the local-target logic was reworked. |

## Rejected as not small (evaluated, not listed above)

16: alarm `AlarmTypes` / composite alarms, `ec2_by_ids` 1000-ID batching, EBS Multi-Attach attachment list, CodeArtifact blank `state` column, EB `IncludeDeleted`, CFN secret-output redaction, eb-rule custom event buses, dbi-snap KMS alias resolution, ecs-task bare SSM parameter names, `cb_build_logs` GetLogEvents pagination, EBS `recoverable`/`recovering` snapshot states, alarm no-actions finding, alarm-history ID dedup, apigw `endpoint` vs `endpoint_type`, pipeline `CodeDeployToECS`, ECR Basic Scanning findings. Each needs a new API call, a new finding/field, or a decision about what a9s should show.
