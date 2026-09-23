| F# | type | verdict | current file:line | symptom (LIVE only) | class (LIVE only) |
|---|---|---|---|---|---|
| F21 | policy | FIXED | core/aws/iam_policies_related.go:44-61 | | |
| F22 | redis | FIXED | core/aws/redis.go:232 | | |
| F23 | s3 | FIXED | core/aws/s3_cross_region.go:41-119; s3_issue_enrichment.go:94; s3_detail_enrichment.go:57; s3_related.go:100,165,223,439; catalog_data.go:276 | | |
| F24 | s3 / cf | LIVE | core/aws/coalesce.go:326-335, 348-353; client.go:196; cf_issue_enrichment.go:222 | A CloudFront distribution whose S3 origin bucket was deleted shows no "origin bucket missing" finding; it reads clean | decorator hides an optional interface from a type assertion |
| F25 | ssm | FIXED | core/aws/ssm.go:62-75, 112-137 | | |
| F26 | ssm | FIXED | core/aws/ssm.go:112-117 → core/secretscan/scan.go:70,111-113 | | |
| F27 | trail | LIVE | core/aws/trail_issue_enrichment.go:22-25, 58-81 | A trail delivering to a bucket made public by an ACL grant shows no Broken "log bucket is publicly accessible"; the same bucket is red (`s3.public`) in the S3 list | two classifiers for one fact |
| F28 | vpce | LIVE | core/aws/vpce.go:101, 173-196; catalog_networking.go:71 | On a real account, failed/rejected/expired/partial/pending/deleting endpoints get no colour, no status phrase and no badge; a deleted or deleting endpoint that has an open policy is still flagged "open to anyone" | API enum case differs from the wire value |
| F29 | waf | LIVE | core/aws/waf_issue_enrichment.go:89-101, 170-184 | A REGIONAL web ACL attached only to API Gateway, AppSync, Cognito, App Runner or Verified Access is marked "not associated with any resource", while its related panel shows API Gateways (1) | call left on the API's default filter |
| F30 | waf | FIXED | core/aws/waf.go:22-36, 38-60; waf_issue_enrichment.go:63-64, 172-173; waf_related.go:28-29, 64-65, 131-132 | | |
| F31 | acm | LIVE (narrower in core, full in snapshot) | core/aws/acm.go:88-94; cmd/snapshot/security.go:433 | ACME-issued certificates never appear in the ACM list (so no expiry or weak-key signal). The snapshot capture also leaves out every non-RSA_2048 certificate | call left on the API's default filter |
| F32 | cf | LIVE | core/aws/cf_issue_enrichment.go:259-267 | A distribution whose default behaviour is HTTPS-only but has an ordered behaviour set to `allow-all` shows no "traffic allowed without TLS" | only the default element of a list evaluated |
| F33 | dbc | LIVE (narrower: rows are not duplicated) | core/aws/dbc.go:19-22, 254-268; catalog_databases.go:307-335; cmd/snapshot/databases.go:198, 213, 230-259 | Neptune clusters show up in DB Clusters. Aurora and Multi-AZ clusters are kept as the DocumentDB-shaped row, so they lose the RDS-only posture checks (auto-minor-upgrade, IAM auth), and their subnet-group pivot goes through the DocumentDB path | call left on the API's default filter |
| F34 | dbc | DISPROVED | core/aws/dbc_issue_enrichment.go:92-101 | | |
| F35 | ec2 | LIVE | core/aws/ec2.go:108-111, 147; ec2_issue_enrichment.go:228-229, 239-256; sg.go:94-103 | An instance that is reachable over IPv6 (public IPv6 plus a `::/0` rule) and has no public IPv4 gets no internet-exposure finding. An IPv4-only instance whose group opens `::/0` only is reported as exposed | address families collapsed into one verdict |
| F36 | ec2 | LIVE | core/aws/ec2_issue_enrichment.go:213-245 | A public, running instance whose security group is on an sg page that was not read shows clean, not `?` | partial cache read as complete (absent = clean) |
| F37 | ecr | LIVE | core/aws/ecr_images.go:123-131, 173-210; ecr_issue_enrichment.go:36-60, 139; cmd/snapshot/compute.go:463-465 | With the new ECR Basic Scanning, critical/high counts, scan status and images-scanned all read empty or clean | reads a field AWS no longer populates |
| F38 | eip | DISPROVED | core/aws/eip.go:73 | | |
| F39 | eks | LIVE | core/aws/eks_related_extra.go:39-72, 94-179, 190-288 | A cluster that runs only self-managed, Auto Mode or Karpenter nodes shows EC2 Instances (0), AMIs (0) and Auto Scaling Groups (0) as proven answers | one of several sources read as the whole answer |
| F40 | glue (snapshot) | LIVE | cmd/snapshot/ops.go:972, 999; main.go:213-234 | Running `cmd/snapshot` writes every Glue job's `DefaultArguments` values, credentials included, as plaintext into snapshot.json | credential value persisted unredacted |

## F21 — FIXED

`core/aws/iam_policies_related.go:44-53` now walks pages: `PageAll(ctx, PerParentPageCap, ... Marker: marker ... iamNextMarker(page.IsTruncated, page.Marker))`. `:60` sets `out.IsTruncated = !complete`, and the checkers at `:98, :122` return `relatedResultTrunc(..., out.IsTruncated)`. Commit 647694aa ("page every related read and carry coverage").

## F22 — FIXED

`core/aws/redis.go:232`: `if transitOn && !aws.ToBool(rg.AuthTokenEnabled) && len(rg.UserGroupIds) == 0 {`. Commit e0ecd925.

## F23 — FIXED

Every per-bucket call now goes to the bucket's own region. `s3_cross_region.go:41-43` `s3For` → `c.InRegion(c.bucketRegion(ctx, bucket)).S3`. The region comes from ListBuckets `BucketRegion` (`learnBucketRegions`, :48-60), with a fallback to `GetBucketLocation` (:65-98). Callers that use it:

- object list: `catalog_data.go:276` `FetchS3Objects(ctx, s3PerBucket{c}, ...)`, where `s3PerBucket.ListObjectsV2` → `s3For` at :117-119
- posture: `s3_issue_enrichment.go:94` `scanS3BucketPosture(ctx, clients.s3For(ctx, bucketName), ...)`
- detail: `s3_detail_enrichment.go:57`
- related: `s3_related.go:100, 165, 223, 439`

The backup pivot compares `BucketRegion` directly (`s3_related.go:340-345`). Commit 7609bbfa.

## F24 — LIVE

- `coalesce.go:326-335`: `S3FullAPI` = S3API + Policy/Cors/Lifecycle/Tagging/Encryption/Logging/Location. It has no `S3HeadBucketAPI`, and `S3API` (`s3_interfaces.go`) does not include HeadBucket either.
- `coalescingS3` (`:348`) embeds the interface `S3FullAPI` and defines only `GetBucketPolicy` (`:369`). Its method set therefore has no `HeadBucket`, even though the wrapped `*s3.Client` has one.
- `client.go:196` always installs the wrapper. So `cf_issue_enrichment.go:222` `headAPI, _ := clients.S3.(S3HeadBucketAPI)` returns nil, and `cfOriginBucketsGone` returns `nil, nil` at `:87-88`. `InRegion` sets are built by the same `CreateServiceClients`, so they carry the same wrapper.

**Sibling (same shape, LIVE):** `core/aws/lambda_issue_enrichment.go:52` `api, ok := clients.Lambda.(LambdaPostureAPI)`; if not ok, the enricher returns empty.

- `LambdaPostureAPI` (`lambda_interfaces.go:53-57`) needs `GetPolicy` and `ListFunctionUrlConfigs`.
- `coalescingLambda` (`coalesce.go:458`) embeds `LambdaAPI` (`lambda_interfaces.go:61-66`: ListFunctions, ListEventSourceMappings, GetFunction, ListTags) and defines only `GetFunction` (`coalesce.go:480`).
- `client.go:202` always wraps Lambda. So in production the Lambda public-policy and public-function-URL posture pass never runs.
- The doc comment at `coalesce.go:442-447` ("LambdaAPI is already the complete aggregate of every Lambda operation asserted anywhere") is false.

I checked the other wrapped assertions and they are satisfied: SNS (`GetTopicAttributes`, `ListTopics`, `ListSubscriptions` are in `SNSFullAPI`), SFN (`DescribeStateMachine` in `SFNAPI`), ECS (`DescribeTaskDefinition` in `ECSAPI`; container instances go through `coalescingECSContainerInstances`).

## F25 — FIXED

There is one classifier now. `ssm.go:112-117` `ssmColorFindings`: `paramType == "String" && secretscan.KeyNamesCredential(paramName)`. The `risk` field is derived from those findings: `ssm.go:74` `"risk": ssmRiskWord(findings)`, :124-137. The old `strings.Contains`/`HasSuffix` pair is gone. Commit 5809ec64.

## F26 — FIXED

`KeyNamesCredential` (`secretscan/scan.go:111-113`) is `kvKeyRe.MatchString`, and `kvKeyRe` (`:70`) = `(?i)(secret|passw(or)?d|passwd|token|...)`, an unanchored substring match. So `prod-db-password`, `github-token` and `stripe-secret` all match. Commit 5809ec64.

## F27 — LIVE

- `trail_issue_enrichment.go:22-25`: `trailLogBucketAPI` = `GetBucketPolicyStatus` + `GetBucketLogging` only.
- `:71`: `public := statusErr == nil && status.PolicyStatus != nil && aws.ToBool(status.PolicyStatus.IsPublic)`.
- The s3 classifier also raises `s3.public` from ACL grants when `IgnorePublicAcls` is off: `s3_issue_enrichment.go:214-226` `if rows := s3ACLPublicRows(aclOut, ignorePublicACLs); pabRead && rows != nil { add(s3CodePublic, rows) }`.
- The spec (`docs/resources/trail.md:105, 135`) defines the signal as "the bucket carries the `s3.public` finding".

Siblings: the trail's access-logging half (`:77-81`) also re-derives `s3.access-logging-off` separately from the s3 posture scan. It is the same shape, but it reads the same single field, so I found no disagreement today.

## F28 — LIVE

- `vpce.go:101`: `state := string(vpce.State)`, raw.
- `vpce.go:175-196`: switch on `"PendingAcceptance"`, `"Pending"`, ... `"Deleted"`, and `state != "Deleting" && state != "Deleted"`.
- The SDK enum (`ec2@v1.335.0/types/enums.go:11886-11896`) is PascalCase (`StateAvailable State = "Available"`), which is what the code matches.
- Evidence of the wire value: the on-disk list caches under `~/.a9s/cache/<profile>--<region>/vpce.yaml` for 12 live-account profile/region pairs all record `state: available` (lowercase). Only the demo cache holds PascalCase (`Available`, `Failed`, `PendingAcceptance`, ...). So the demo fixtures hide the bug.
- I have only seen `available` live. The other lowercase values (`failed`, `pendingAcceptance`, ...) are inferred from the same casing and I have not observed them.
- `colorVPCE` (`catalog_networking.go:71`) feeds `Fields["state"]` into the same function.

## F29 — LIVE

- `waf_issue_enrichment.go:176-180`: `ListResourcesForWebACL(ctx, &ListResourcesForWebACLInput{WebACLArn: aws.String(arn)})`, with no `ResourceType`.
- `:94-100`: `len(associations) == 0` → `wafCodeOrphan`.
- SDK `wafv2@v1.83.0/api_op_ListResourcesForWebACL.go:55-58`: "If you don't provide a resource type, the call uses the resource type APPLICATION_LOAD_BALANCER. Default: APPLICATION_LOAD_BALANCER".
- Valid types (`types/enums.go:1396-1403`): ALB, API_GATEWAY, APPSYNC, COGNITO_USER_POOL, APP_RUNNER_SERVICE, VERIFIED_ACCESS_INSTANCE, AMPLIFY, AGENTCORE_GATEWAY.
- The related panel queries per type (`waf_related.go:35-37` ALB, `:138-140` API_GATEWAY), which is why the list row and the related panel disagree.
- The CLOUDFRONT half is already correct (`:171-173` → `wafDistributionIDs`).

## F30 — FIXED

- `waf.go:22-36`: `wafRegionOf(CLOUDFRONT)` = `us-east-1`, and `wafIn(scope)` = `c.InRegion(...).WAFv2`.
- The enricher uses `api := clients.wafIn(scope)` for GetLoggingConfiguration and GetWebACL (`waf_issue_enrichment.go:63-64, 67, 104`).
- Associations for CLOUDFRONT go to `wafDistributionIDs` → `cloudfront:ListDistributionsByWebACLId` (`waf.go:38-60`, `waf_issue_enrichment.go:171-173`).
- The related panel: `checkWAFELB` and `checkWAFAPIGW` return `ProvenZero(..., "scope")` for CLOUDFRONT (`waf_related.go:28-29, 131-132`), and `checkWAFLogs` uses `c.wafIn(scope)` (`:64-65`).
- Commit 7609bbfa.

## F31 — LIVE (narrower in core)

- The key-type half is fixed in core: `acm.go:93` `Includes: &acmtypes.Filters{KeyTypes: acmtypes.KeyAlgorithm("").Values()}` (commit d2b7399d).
- The ACME half is still live. SDK `acm@v1.50.0/api_op_ListCertificates.go:20-22` says "By default, this action does not return certificates with a CertificateKeyPairOrigin of ACME. To include ACME certificates, specify ACME in the CertificateKeyPairOrigins filter". `:39-42` `CertificateKeyPairOrigins []types.CertificateKeyPairOrigin` — "Default filtering returns only certificates with key pair origin of AWS_MANAGED and CUSTOMER_PROVIDED". `acm.go:88-94` does not set it.
- `cmd/snapshot/security.go:433` still passes `&acm.ListCertificatesInput{}`, so both halves are live there.

## F32 — LIVE

`cf_issue_enrichment.go:259-267` checks only `cfg.DefaultCacheBehavior.ViewerProtocolPolicy == ViewerProtocolPolicyAllowAll`. `cfg.CacheBehaviors.Items` is never read in the enricher (grep: its only `CacheBehaviors` uses are `cf.go:86` and `cf_related.go:253`, both for Lambda associations). The spec row (`docs/resources/cf.md:102`) says "viewer allows plain HTTP", with no exemption for ordered behaviours.

Siblings: none. The other DefaultCacheBehavior readers (`cf.go:83-87`, `cf_related.go:250-254`) already iterate `CacheBehaviors`.

## F33 — LIVE (narrower)

- `dbc.go:20-22`: `docdb.DescribeDBClustersInput{MaxRecords: ...}`, with no `engine=docdb` filter.
- SDK `docdb@v1.56.0/api_op_DescribeDBClusters.go:14-17`: "…technology that is shared with Amazon RDS and Amazon Neptune. Use the filterName=engine,Values=docdb filter parameter to return only Amazon DocumentDB clusters."
- The code's own comment (`dbc.go:246-249`) says "verified live: the DocDB DescribeDBClusters endpoint returns aurora-postgresql clusters too".
- The merge `dedupResourcesByID(append(docResult.Resources, rdsResult.Resources...))` (`catalog_databases.go:335`) keeps the **first**, docdb-shaped row. Result:
  - An Aurora row is built by `computeDBCFindings(docdbtypes.DBCluster)` (`dbc.go:222-240`), which passes no `AutoMinorVersionUpgrade`/`IAMAuthEnabled`, so those posture findings stay silent.
  - `dbcSubnetGroup` routes that row to `dbcDocDBSubnetGroup` (`dbc_related.go:276-280`).
  - Neptune rows returned by the docdb endpoint are not filtered. Only the RDS side drops them (`dbc_rds.go:77`).
- The finding's "RDS can return DocumentDB/Neptune rows" half is already handled at `dbc_rds.go:77`.
- Snapshot: `cmd/snapshot/databases.go:198` (docdb, unfiltered) and `:213` (rds, no engine skip) feed `combineDBCClusters`/`dedupDBCByID` (`:230-259`) with the same first-wins rule.
- Siblings: `core/aws/dbc_snap.go:56` `docdb.DescribeDBClusterSnapshotsInput{MaxRecords}` is also unfiltered on the same shared backend. The dedup comment (`dbc.go:250-251`) says dbc-snap uses the same first-wins dedup. I did not trace dbc-snap further.

## F34 — DISPROVED

- The docdb client is not a separate backend. `docdb@v1.56.0/endpoints.go:121` sets signing name `"rds"`, and `:375, 410, 429` build `https://rds.<region>…` endpoints.
- `dbc_issue_enrichment.go:95` calls `DescribePendingMaintenanceActions` with no filter. That is the same RDS action on the same endpoint, so it is not scoped to DocumentDB. The same shared backend is why the unfiltered docdb `DescribeDBClusters` returns Aurora clusters (F33, verified live per `dbc.go:248-249`).
- `clusterKeyOf` (`:79-89`) matches any `…:cluster:<id>` ARN whose suffix is a listed row ID, whatever the engine.
- Caveat: this rests on the shared endpoint, not on a live observation of an Aurora pending action returned by the docdb client.

## F35 — LIVE

- `ec2.go:108-111`: only `inst.PublicIpAddress` (IPv4) becomes `Fields["public_ip"]`. No IPv6 field is read (grep `Ipv6Address` in `core/aws/ec2*.go`: 0 hits).
- `ec2_issue_enrichment.go:228-229`: `if publicIP == "" … continue`, so an IPv6-only public instance is never judged.
- `sg.go:94-103` `isInternetFacing` merges `IpRanges` and `Ipv6Ranges` into one verdict, which then becomes the single `wide_open`/`open_ports` fields that `:239-256` joins. So an IPv6-only `::/0` rule is attributed to an IPv4-only instance.

## F36 — LIVE

- `ec2_issue_enrichment.go:214`: `sgEntry, sgLoaded := cache["sg"]`. Only `!sgLoaded` marks the row uninspected (`:232-234`). `sgEntry.IsTruncated` (`domain/contracts.go:113-114`) is never read (grep `IsTruncated` in the file: 0 hits).
- `:241-244`: `ri, known := risk[id]; if !known { continue }`. An attached group absent from a partial list is skipped silently. With no known risky group, `portSet` is empty and the row is left clean (`:257-259`).
- A partial sg entry is a real case: `core/runtime/probes.go:1036-1040` prefetches a declared read "first page only".
- Siblings: `cachedBucketNames` in the CF enricher does it right (it returns nil when the list is incomplete, then `errS3ListIncomplete` at `cf_issue_enrichment.go:95-97`). I did not audit the other 12 `Reads:` enrichers.

## F37 — LIVE

- SDK `ecr@v1.66.0/api_op_DescribeImages.go:21-23`: "The new version of Amazon ECR Basic Scanning doesn't use the ImageDetail$imageScanFindingsSummary and ImageDetail$imageScanStatus attributes from the API response to return scan results. Use the DescribeImageScanFindings API instead."
- Production reads only those two attributes: `ecr_images.go:123-131, 174-203`; `ecr_issue_enrichment.go:139` `summary := img.ImageScanFindingsSummary`; `cmd/snapshot/compute.go:463-465`.
- `ECRDescribeImageScanFindingsAPI` is declared (`ecr_interfaces.go:45, 75`) but never called outside the demo fake.

## F38 — DISPROVED (the trigger cannot occur on current AWS)

- The ID comes from `AllocationId` (`eip.go:73`). SDK `ec2@v1.335.0/types/types.go:399-400` documents `Address.Domain` as "The network ( vpc )". `DomainTypeStandard` survives only as an enum value (`enums.go:2408`).
- `standard` addresses were EC2-Classic addresses. From my own knowledge, not from anything in this repo: AWS retired EC2-Classic in August 2023 and migrated the remaining Classic Elastic IPs to VPC, so no account can still hold one. I have not checked this against a live account.
- If an empty `AllocationId` did occur, the collapse the finding describes would be real, since nothing falls back to `PublicIp`.

## F39 — LIVE

- `checkEKSASG` (`eks_related_extra.go:39-72`) takes ASGs only from the `ng` (managed node group) cache and returns `relatedResultTrunc("asg", ids, truncated)`. With no managed node groups for the cluster, that is a proven 0.
- `checkEKSEC2` (`:190-288`) and `checkEKSAMI` (`:94-179`) walk `ListNodegroups` → `DescribeNodegroup` → ASGs. With zero node groups, `:245` returns `relatedResultTrunc("ec2", nil, false)` when the walk completed, a proven 0.
- The spec already says otherwise. `docs/resources/eks.md:55` says EC2 is found by "tags contain `kubernetes.io/cluster/<Cluster.Name>==owned` OR `eks:cluster-name==<Cluster.Name>`". No production code reads either tag (grep `kubernetes.io/cluster` in `core/aws/*.go`: 0 non-test hits).

## F40 — LIVE

- `cmd/snapshot/ops.go:972` `DefaultArguments map[string]string` is tagged json, and `:999` copies `j.DefaultArguments` verbatim. `main.go:213-234` `writeJSON` marshals and writes it.
- Mitigations: the file is 0600 (CreateTemp + rename, `main.go:210-212`), and the default `--out tests/e2e/testdata/snapshot` is git-ignored (`.gitignore:16`). The plaintext still lands on disk, and a custom `--out` is not covered by the ignore.
- Sibling (same shape): `cmd/snapshot/ops.go:102-106`, where CodeBuild `EnvironmentVariables` are captured with `Value: aws.ToString(ev.Value)` for every type, PLAINTEXT included.
