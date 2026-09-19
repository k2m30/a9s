
## acm

# acm — production-code review

## 1. P1 — ListCertificates default filter hides every non-RSA_2048 certificate; weak-key finding is unreachable

- **File**: `core/aws/acm.go:85-87` (`FetchACMCertificatesPage` builds `acm.ListCertificatesInput` with only `MaxItems`/`NextToken`).
- **Evidence**: AWS API reference for ListCertificates: "Default filtering returns only `RSA_2048` certificates" and "By default, this action does not return certificates with a `CertificateKeyPairOrigin` of `ACME`." No `Includes.KeyTypes` or `CertificateKeyPairOrigins` is set anywhere in the fetcher.
- **Trigger**: An account holding EC_prime256v1 / EC_secp384r1 / RSA_3072 / RSA_4096 / RSA_1024 certificates, or ACME-origin certificates.
- **User impact**: Those certificates never appear in the list, so their expiry / pending-validation / failed / orphan findings and the main-menu issue count are silently missing — an EC cert expiring tomorrow shows nothing. The `acm.weak-key` finding (`acm.go:212-214`, `acm.go:175-179`, catalog `catalog_dns_cdn.go:184`) can never fire, because the only weak key it detects (`RSA 1024`) is excluded by the default filter.
- **Fix direction**: Set `Includes: &types.Filters{KeyTypes: <all types.KeyAlgorithm values>}` and `CertificateKeyPairOrigins` to all origins on every page request.

## 2. P2 — API Gateway pivot emits custom-domain names as apigw IDs; they never match an apigw row

- **File**: `core/aws/acm_related.go:117-121` (`checkACMAPIGW`, `/domainnames/` branch).
- **Evidence**: The `apigw` type's rows are keyed by API ID (`core/aws/apigw.go:180`, `core/aws/apigw.go:245`: `ID: apiID`). The branch appends the segment after the last `/` of `arn:aws:apigateway:<region>::/domainnames/<domain>`, i.e. a DNS name such as `api.example.com`, which is never an API ID.
- **Trigger**: A certificate attached to an API Gateway custom domain (the normal way ACM certificates are used by API Gateway).
- **User impact**: The related panel shows "API Gateways (1)" but drilling in finds no matching API — the count is not backed by a navigable row.
- **Fix direction**: Resolve domain name to API IDs (GetBasePathMappings / apigatewayv2 GetApiMappings) or pivot to a type keyed by domain name; otherwise do not count domainnames entries as apigw rows.

## 3. P3 — ELB pivot counts Classic Load Balancers that the elb list does not contain

- **File**: `core/aws/acm_related.go:98-99` (`checkACMELB`, classic-ARN branch).
- **Evidence**: The branch explicitly extracts names from Classic ELB ARNs (`...:loadbalancer/<name>`), but the `elb` type is fetched from elbv2 `DescribeLoadBalancers` (`core/aws/elb.go:25`, ID = `LoadBalancerName`) and holds only ALB/NLB/GWLB.
- **Trigger**: A certificate used by a Classic Load Balancer listener.
- **User impact**: "Load Balancers (N)" includes a load balancer that drilling into the elb list cannot show.
- **Fix direction**: Drop the classic branch (or route it to a type that lists Classic ELBs).

## 4. P3 — Route 53 zone match has no label boundary

- **File**: `core/aws/acm_related.go:190` (`strings.HasSuffix(recordName, zn)`).
- **Trigger**: Validation record `_x.notexample.com.` while zone `example.com` is loaded and the certificate's own zone (`notexample.com`) is not in this account.
- **User impact**: The panel pivots to an unrelated hosted zone as the one holding the validation record.
- **Fix direction**: Match `recordName == zn || strings.HasSuffix(recordName, "."+zn)`.

## 5. P3 — Route 53 pivot reports 0 for certificates without DNS validation records, contrary to the spec's DomainName match

- **File**: `core/aws/acm_related.go:176-178`.
- **Evidence**: `docs/resources/acm.md:49` defines r53 discovery as a longest-suffix match of `CertificateSummary.DomainName` against loaded zones, with `DomainValidationOptions[].ResourceRecord.Name` only as a more precise hint. The code returns a known zero whenever no DVO carries a `ResourceRecord`.
- **Trigger**: Imported certificates and email-validated certificates (no `ResourceRecord`), whose domain is hosted in a loaded zone.
- **User impact**: "Route 53 Zones (0)" for a certificate whose zone is in the account.
- **Fix direction**: When no validation record names exist, fall back to `DomainName` (wildcard prefix stripped) for the suffix match.

## alarm

# alarm — production-code review

Scope: `core/aws/alarm.go`, `alarm_codes.go`, `alarm_interfaces.go`, `alarm_related.go`, `alarm_related_extra.go`, `alarm_history.go`, `alarm_history_codes.go`, the `alarm` entry in `core/aws/catalog_monitoring.go`, `core/config/defaults_monitoring.go`, plus the related-navigation and list-append paths they feed (`core/runtime/handlers_related.go`, `core/runtime/executor.go`, `core/app/list_body.go`, `core/resource/resource.go`). Spec: `docs/resources/alarm.md`.

## 1. [P2] The CloudTrail pivot lists every alarm's events, not this alarm's

- **File**: `core/aws/alarm_related_extra.go:169-188` (match at `:183-184`)
- **Trigger**: open any alarm's detail while the ct-events cache holds CloudWatch alarm events (PutMetricAlarm, DeleteAlarms, DisableAlarmActions, SetAlarmState, …) for other alarms.
- **Impact**: `name := res.ID` is only checked for emptiness. The loop keeps every event whose `source` contains `monitoring.amazonaws.com` and whose `event_name` contains `Alarm`. Every alarm then shows the same "CloudTrail Events (N)" count. Drilling in opens a list of changes to unrelated alarms. An operator looking for who disabled *this* alarm's actions gets the whole account's alarm audit trail and may blame the wrong change. Sibling checkers (`checkDbiCTEvents`, `core/aws/dbi_related.go:290-320`) scope by `Resources[].ResourceName == id` and set `FetchFilter{"ResourceName": id}`.
- **Fix**: match on the event's `Resources[].ResourceName`, or `Fields["resource_name"]`, equal to the alarm name or ARN. Return `DeferredRelated`/`WithFetchFilter({"ResourceName": name})` the way `checkDbiCTEvents` does.

## 2. [P2] The ECS and EKS pivots misattribute clusters across services

- **File**: `core/aws/alarm_related_extra.go:73-81` (ECS, no namespace guard) and `:84-95` (EKS, `strings.Contains(ns, "ContainerInsights")` at `:91`)
- **Trigger**:
  - An EKS Container Insights alarm (namespace `ContainerInsights`, dimension `ClusterName`) shows "ECS Clusters (1)" pointing at the EKS cluster name.
  - An ECS Container Insights alarm (namespace `ECS/ContainerInsights`) matches `Contains("ContainerInsights")` and shows "EKS Clusters (1)" pointing at the ECS cluster name.
  - Any other namespace that uses a `ClusterName` dimension (for example `AWS/MemoryDB`) produces an ECS pivot.
- **Impact**: the related panel claims a cluster in the wrong service. The drill opens an empty or wrong list. On a same-named ECS and EKS cluster, it silently opens the wrong resource.
- **Fix**: guard on the exact namespace. ECS: `AWS/ECS` or `ECS/ContainerInsights`. EKS: `AWS/EKS` or `ContainerInsights`, with no substring match. This follows spec §2.

## 3. [P2] The ASG pivot ignores scaling-policy actions and only reads the dimension

- **File**: `core/aws/alarm_related.go:43-77` (dimension-only lookup at `:52-60`)
- **Trigger**: a step-scaling or simple-scaling alarm whose metric is not dimensioned by `AutoScalingGroupName` (SQS `ApproximateNumberOfMessagesVisible`, ALB `RequestCountPerTarget`, a custom metric). Its `AlarmActions` hold `arn:aws:autoscaling:…:scalingPolicy:…:autoScalingGroupName/<asg>`.
- **Impact**: the checker returns `KnownRelated("asg", nil, false)`, which is a proven zero, for an alarm that actually scales an ASG. When the alarm goes red, the operator cannot pivot to the group it resizes. Spec §2 `asg` names the scaling-policy ARN scan as the primary discovery path.
- **Fix**: scan `AlarmActions`/`OKActions`/`InsufficientDataActions` for autoscaling `scalingPolicy` ARNs. Extract the `autoScalingGroupName/<name>` segment and union the result with the dimension match before the cache lookup.

## 4. [P2] Alarm-history rows use a minute timestamp as ID, so load-more drops rows

- **File**: `core/aws/alarm_history.go:73-79`, `:95` (`ID: timestamp` formatted `2006-01-02 15:04`)
- **Trigger**: an alarm with more history than one `DescribeAlarmHistory` page. A flapping alarm or any state change emits a StateUpdate and an Action item within the same second. The operator presses "m" to load more.
- **Impact**: child screens append through `core/app/list_body.go:149` → `resource.DedupByID` (`core/resource/resource.go:15-32`). Every incoming item whose minute matches an already-loaded row is silently discarded, and duplicates inside the new page collapse too. History entries, including ALARM transitions, vanish from the list. On the first page, several rows share one ID. Items with a nil `Timestamp` all get ID `""`.
- **Fix**: build a unique ID, for example the full-precision RFC3339Nano timestamp plus `HistoryItemType` plus a hash of `HistorySummary`, or the page offset. Keep the minute-formatted string only as the display `timestamp` field.

## 5. [P2] "no actions" is suppressed exactly when the alarm is firing or blind

- **File**: `core/aws/alarm.go:113-125` (`no_actions` only under `case "OK"`, `:119-122`). Mirrored by the `colorAlarm` fallback at `core/aws/catalog_monitoring.go:17-23`.
- **Trigger**: an alarm in `ALARM` or `INSUFFICIENT_DATA` with `AlarmActions == []`.
- **Impact**: the detail findings (S5) show only "alarm triggered". Nothing says nobody was notified, which is the worst case of the alert-to-nowhere signal. Spec §3.1/§4 define `AlarmActions == []` as an independent Wave 1 signal reaching S1–S5. The sibling `alarm.actions-disabled` finding is correctly emitted independent of state (`alarm.go:78-84`), so the two "silent alarm" signals behave inconsistently.
- **Fix**: emit `CodeAlarmNoActions` whenever `len(AlarmActions) == 0`, regardless of state, and let severity ordering keep the row red for ALARM.

## 6. [P2] Dimension pivots assert a count without checking that the target exists

- **File**: `core/aws/alarm_related_extra.go:32, 35, 46, 57, 68, 79, 92, 104, 115, 126, 137, 149, 151, 162`
- **Trigger**: an alarm left behind after its resource was deleted, the "zombie alarm". For example, `InstanceId=i-…` for a terminated instance or `FunctionName` for a deleted Lambda function.
- **Impact**: each checker returns `relatedResult(target, []string{dimValue})` straight from the dimension. The panel shows "EC2 Instances (1)", "Lambda Functions (1)", and so on, for a resource that does not exist. The drill opens an empty filtered list, or for `ec2`/`kms` (which have `FetchByIDs`) a not-found fetch error at detail open (`core/runtime/executor.go:994-1006`). Spec §2 requires cross-referencing the loaded sibling list. A zombie alarm is exactly the case where the count lies.
- **Fix**: resolve the value against `relatedResourcesFor(ctx, clients, cache, target)` the way `checkAlarmASG` does, and mark those defs `NeedsTargetCache`. Report unknown when the list is unavailable and the truncation flag when it is truncated.

## 7. [P3] The apigw and waf pivots carry names where the target's ID is an API ID or ACL UUID

- **File**: `core/aws/alarm_related_extra.go:31-32` (`ApiName`) and `:161-162` (`WebACL`, which is the ACL *name*). The target IDs are set at `core/aws/apigw.go:180,245` (`ID: apiID`) and `core/aws/waf.go:225` (`ID: id`).
- **Trigger**: open the API Gateways pivot on a REST API latency alarm, or the WAF pivot on a WAFv2 alarm.
- **Impact**: `ResolveRelatedNavigate` (`core/runtime/handlers_related.go:357-373`) never gets a cache hit on the ID. The drill always falls back to a text-filtered list instead of opening the detail, and REST API names are not unique. Any existence check (finding 6) keyed by ID would also always miss.
- **Fix**: map the dimension value to the target's ID through the loaded list by `Name` before returning IDs.

## 8. [P3] Metric-math and anomaly-detection alarms: pivots claim a proven zero, and the detail has no metric definition

- **File**: `core/aws/alarm_related_extra.go:17-24` (`alarmDimension` reads only top-level `MetricAlarm.Dimensions`) together with every `KnownRelated(target, nil, false)` fallback in that file and in `alarm_related.go:58-60`. `core/config/defaults_monitoring.go:8-16` (no `Metrics` / `ThresholdMetricId` detail paths).
- **Trigger**: an alarm defined through `Metrics[]`, such as a Lambda error-rate expression or an anomaly-detection band. `MetricName`, `Namespace` and `Dimensions` are all nil.
- **Impact**: every dimension pivot shows a definitive 0 even though the dimensions exist under `Metrics[].MetricStat.Metric.Dimensions`. The detail view shows empty Metric, Namespace and Threshold with no way to see what the alarm evaluates.
- **Fix**: when `Dimensions` is empty and `Metrics` is set, collect dimensions from `Metrics[].MetricStat.Metric` (and their namespaces), or return unknown instead of a proven zero. Add `Metrics` and `ThresholdMetricId` to the alarm detail view.

## 9. [P3] The Threshold column rounds small thresholds to 0.00

- **File**: `core/aws/alarm.go:52-54` (`fmt.Sprintf("%.2f", *alarm.Threshold)`). The field wins over the column `Path` in `core/app/list_columns.go:54-56`.
- **Trigger**: an alarm with threshold `0.005` (error rate), `0.001`, or any value needing more than two decimals.
- **Impact**: the list shows `0.00`, which reads as "fires on any non-zero value". The operator misjudges the alarm's sensitivity.
- **Fix**: use `strconv.FormatFloat(v, 'g', -1, 64)`, or drop the pre-formatted field and let the `Threshold` path render.

## 10. [P3] The "Configured actions" row under "actions disabled" counts only ALARM-transition actions

- **File**: `core/aws/alarm.go:56`, `:81-83`
- **Trigger**: an alarm with `ActionsEnabled=false` whose actions are wired only on `OKActions` or `InsufficientDataActions`.
- **Impact**: the row reports `Configured actions: 0` while actions are in fact configured behind the switch. This contradicts the finding's purpose of showing how much is muted.
- **Fix**: count `len(AlarmActions)+len(OKActions)+len(InsufficientDataActions)` for this row, or label it "Alarm actions".

## ami

# ami — production-code review

Scope: `core/aws/ami.go`, `core/aws/ami_related.go`, `core/aws/ami_related_extra.go`, `core/aws/ami_codes.go`, the `ami` entry in `core/aws/catalog_compute.go` (lines 806-866, `colorAMI` at 128), `core/config/defaults_compute.go:134-144`, and the dependencies needed to verify them (`related_common.go`, `related_shared.go`, `asg_related.go:checkASGAMI`, `ec2_related_extra.go:checkEC2AMI`, `runtime/executor.go` FetchByIDs call sites, the `ng` fetcher's `image_id` population in `catalog_containers.go:322-328`).

AWS semantics checked against the current API reference: `API_EbsBlockDevice.html` and `API_DescribeImages.html`.

## Findings

### 1. P2: The KMS pivot always reports 0 keys for encrypted AMIs

- **File/line**: `core/aws/ami_related_extra.go:48-67` (`checkAMIKMS`)
- **Defect**: the checker reads `BlockDeviceMappings[].Ebs.KmsKeyId` from the `DescribeImages` response. AWS documents `EbsBlockDevice.KmsKeyId` as "only supported on `BlockDeviceMapping` objects called by RunInstances, RequestSpotFleet, and RequestSpotInstances". `DescribeImages` returns `Encrypted` and `SnapshotId`, but not the key. The loop therefore never finds a key, and line 64-65 returns `KnownRelated("kms", nil, false)`, which is a proven zero.
- **Trigger**: open the detail view of any AMI whose backing snapshots are KMS-encrypted.
- **User impact**: the related panel shows "KMS Keys 0" for an encrypted image. The spec (§2 `kms`) says this pivot exists so operators can check key access and cross-account sharing breakage. The panel states that no key is involved, which is false, so the operator cannot follow the pivot. The raw value would also skip the `kmsKeyIDFromField` normalization that every sibling KMS checker applies (for example `lt_related.go:66`).
- **Fix direction**: for mappings with `Ebs.Encrypted == true`, get the key from the backing snapshot's `KmsKeyId`. Use the `ebs-snap` rows (`NeedsTargetCache`) or `DescribeSnapshots` on the `SnapshotId`s, and normalize through `kmsKeyIDFromField`. When an encrypted mapping's key cannot be resolved, return `UnknownRelated("kms")` instead of a proven zero.

### 2. P2: The ASG pivot uses running instances as a stand-in for the launch source, so ASGs scaled to zero never match

- **File/line**: `core/aws/ami_related.go:67-111` (`checkAMIASG`). The unknown EC2 case also collapses into a proven zero at `ami_related.go:84-85`.
- **Defect**: an ASG is linked to this AMI only if one of its current `Instances[]` appears in the ec2 list with `image_id == amiID`. The spec (§2 `asg`) requires a match on the ASG's launch source, meaning `LaunchConfiguration.ImageId` or `LaunchTemplate` version `ImageId`. The reverse checker `checkASGAMI` (`asg_related.go:85-125`) already resolves that source. The same relationship is therefore computed two different ways, and the two directions disagree. In addition, if the ec2 list comes back nil, the code continues with an empty map (lines 84-85) and returns a resolved zero instead of Unknown.
- **Trigger**: an ASG whose launch template or launch configuration uses this AMI but that currently has 0 instances (desired=0, a scheduled scale-in, or instances still pending). A second trigger is an ASG whose instances fall outside the single ec2 page that was read.
- **User impact**: the AMI's related panel shows no ASG for an image that the ASG will launch on its next scale-out. This is the exact "can I deregister this AMI?" check the spec names. The operator deregisters the image and the next scale-out fails. Opening the ASG's own panel still shows this AMI, so the two views contradict each other.
- **Fix direction**: resolve each ASG's launch-source ImageId using the same launch configuration / launch template logic as `checkASGAMI`, extracted into one shared helper that both directions call, and match on that. When the source cannot be resolved, return Unknown, not zero.

### 3. P2: The "shared with all AWS accounts" (Broken) finding fires on public AMIs the account does not own

- **File/line**: `core/aws/ami.go:214-221`, which is reached through `core/aws/ami.go:45-50` (`FetchAMIsByIDs`)
- **Defect**: the comment at 216-217 says a public image here must be owned by the account because "the fetcher asks for Owners=self". Nothing enforces that. `imageResource` is also used by `FetchAMIsByIDs`, which calls `DescribeImages` with `ImageIds` and no `Owners` filter. The related-panel lazy-add path (`runtime/executor.go:994-1005`) and the drill path (`runtime/executor.go:526`) both fetch AMIs this way.
- **Trigger**: open an EC2 instance launched from an Amazon Linux, Ubuntu, or Marketplace AMI, then follow the `ami` pivot or the `ImageId` navigable field (`checkEC2AMI` at `ec2_related_extra.go:18-27`). Amazon, Canonical, and Marketplace AMIs have `Public=true`.
- **User impact**: a third-party AMI is shown as Broken (red), with a `Public: yes` `!` detail row and the advice "Remove the `all` group from the image's launch permission". The operator cannot take that action on an image they do not own. A normal vendor image looks like a data-exposure incident.
- **Fix direction**: emit `CodeAMIPublic` only for images this account owns. One option is to compare `img.OwnerId` with the caller's account ID. The other is to compute the finding only on the `Owners=self` page path and not in the shared `imageResource`. Either way the ownership condition is enforced in code instead of asserted in a comment.

### 4. P3: Disabled AMIs are missing from both the list and by-ID fetches

- **File/line**: `core/aws/ami.go:77-80` (`FetchAMIsPage` input) and `core/aws/ami.go:46-49` (`FetchAMIsByIDs` input)
- **Defect**: the `DescribeImages` reference says of `IncludeDisabled`: "Default: No disabled AMIs are included in the response." Unlike `IncludeDeprecated`, it has no exception for the owner. Neither call sets `IncludeDisabled`.
- **Trigger**: the account owns an AMI that was disabled with `DisableImage`. A second trigger is an EC2 instance or launch template that references a disabled AMI.
- **User impact**: disabled images the account owns are missing from the AMI list, even though they can be re-enabled and their snapshots are still billed. The spec's Dim row for `State == disabled` (§3.1, §4, `ami.go:204-205`) can never appear. Following a pivot to a disabled AMI ends in "ami … not found" instead of the dim `disabled` row.
- **Fix direction**: set `IncludeDisabled: aws.Bool(true)` on both `DescribeImagesInput`s.

### 5. P3: The CloudFormation pivot claims a proven "0 stacks" when it did not search

- **File/line**: `core/aws/ami_related_extra.go:27-28` (`checkAMICFN`)
- **Defect**: when the AMI has no `aws:cloudformation:stack-name` tag, the checker returns `KnownRelated("cfn", nil, false)`. With a present RawStruct, `unreadZero` passes that through unchanged, so a resolved zero reaches the panel. The spec (§2 `cfn`: "Count shown: unknown"; §5: "The row remains in the related panel but shows no count") says stack references to an AMI have no discovery path, because they live in template parameters and resources. The tag check does not cover that, and almost no AMI carries the tag.
- **Trigger**: open the detail view of practically any AMI, including one whose `ImageId` is hardcoded or passed as a parameter in a stack.
- **User impact**: the panel shows "CloudFormation Stacks 0". Before deregistering, the operator reads this as "no stack depends on this image", which the code never checked.
- **Fix direction**: return `UnknownRelated("cfn")` when no stack-name tag is present, so the row shows no count, as the spec requires. Keep the tag match only as a positive hit.

## apigw

# apigw — production-code review

Scope: `core/aws/apigw.go`, `core/aws/apigw_related.go`, `core/aws/apigw_issue_enrichment.go`, `core/aws/apigw_interfaces.go`, the `apigw` catalog entry in `core/aws/catalog_dns_cdn.go`, the `apigw` view defaults in `core/config/defaults_dns_cdn.go`, and the pivots from other types into apigw (`alarm_related_extra.go`, `acm_related.go`, `lambda_related_extra.go`, `r53_related.go`, `logs_related.go`, `waf_related.go`).

16 findings (P0:0 P1:1 P2:10 P3:5)

---

## 1. P1 — The REST resource policy is never parsed, so a REST API protected by a scoped policy is flagged "internet-facing with no authorizer" (Broken)

- **Location**: `core/aws/apigw_issue_enrichment.go:242` (reads it through `apigwRESTPolicy`, `:289-295`)
- **Trigger**: A REGIONAL or EDGE REST API with no authorizer, protected by a resource policy scoped with `aws:SourceIp` or `aws:SourceVpce`, or limited to specific principals.
- **Cause**: `GetRestApis` / `GetRestApi` return `RestApi.Policy` as a JSON string with escaped quotes, `{\"Version\":\"2012-10-17\",...}`. The Terraform AWS provider has to unescape it with `strconv.Unquote` in `flattenAPIPolicy` (`internal/service/apigateway/rest_api.go`). `iampolicy.Parse` only URL-decodes (`core/iampolicy/policy.go:59-66`), so `json.Unmarshal` fails. `perr != nil` then leaves `scoped = false`, and the code falls through to `CodeAPIGWNoAuthorizerPublic`.
- **User impact**: Every REST API guarded only by a resource policy shows as a red, Broken row that raises the menu issue count. The protection that exists is reported as missing.
- **Fix**: Unescape the policy before `iampolicy.Parse`: wrap it in quotes and `strconv.Unquote`, after normalising `\/`. If the policy is present but cannot be parsed, treat the result as unknown (no finding) instead of unscoped.

## 2. P2 — The related panel sends apigatewayv2 calls for REST APIs, so the lambda, kms, elb and role pivots error on every REST API

- **Location**: `core/aws/apigw_related.go:133-149` (`apigwListIntegrations`), called from `:30` (kms), `:159` (lambda), `:326` (elb) and `:453` (role). `checkApigwRole` also calls v2 `GetAuthorizers` at `:481-483`.
- **Trigger**: Opening the detail view of any row with `protocol == "REST"`. The fetcher merges v1 REST APIs into the list (`apigw.go:88-95`).
- **Cause**: apigatewayv2 serves only HTTP and WebSocket APIs. For a REST API ID, `GetIntegrations` and `GetAuthorizers` return `NotFoundException: Invalid API identifier specified`. The checkers turn that into `ErrorRelated`.
- **User impact**: For every REST API (the most common kind), the Lambda Functions, KMS Keys, Load Balancers and IAM Role rows are dimmed error dead ends. Lambda targets and invocation roles cannot be reached from REST APIs at all.
- **Fix**: Branch on `res.Fields["protocol"] == "REST"` and use the v1 client. Walk `GetResources`, then read the integration `uri`/`credentials` from `GetIntegration` or from resources returned with `embed=methods`. Use v1 `GetAuthorizers` for `authorizerCredentials`. Until that exists, return `UnknownRelated` for REST instead of issuing a v2 call that is certain to fail.

## 3. P2 — "No authorizer" findings ignore IAM authorization, so IAM-protected APIs are flagged, and REST ones as Broken

- **Location**: `core/aws/apigw_issue_enrichment.go:237-260` (REST), `:299-318` (HTTP/WebSocket)
- **Trigger**:
  - A REST API whose methods use `authorizationType: AWS_IAM` (or require API keys), with no Lambda or Cognito authorizer.
  - An HTTP or WebSocket API whose routes use `AuthorizationType: AWS_IAM`.
- **Cause**: The verdict is based only on whether `GetAuthorizers` returns an empty list. IAM authorization is set per method or route and creates no authorizer object.
- **User impact**:
  - IAM-signed REST APIs show red: "internet-facing with no authorizer", with the detail "Anyone on the internet can call every route".
  - IAM-protected HTTP APIs show "no authorizer".
  - Both claims are false for APIs that require SigV4.
- **Fix**: Before emitting a finding, check the route or method authorization type (v2 `GetRoutes` `AuthorizationType`; v1 `GetResources` with `embed=methods` `authorizationType`). Emit a finding only when some route or method has `NONE`. Otherwise, narrow the phrase and detail so they do not claim that nothing checks identity.

## 4. P2 — A throttle of 0 is reported as "no throttling configured (DoS risk)", but it actually blocks all traffic

- **Location**: `core/aws/apigw_issue_enrichment.go:147-161`
- **Trigger**: An HTTP API stage whose `DefaultRouteSettings.ThrottlingBurstLimit` or `ThrottlingRateLimit` is `0`. This is common: Terraform and Pulumi write 0 when default route settings are touched (terraform-provider-aws #30373, pulumi-aws #2363).
- **Cause**: A burst or rate limit of 0 means every request gets `429 Too Many Requests`. The code treats 0 as "unthrottled". A stage with no throttle settings (nil fields) is not flagged at all.
- **User impact**: A stage that rejects all traffic shows as a low-severity `~` "stage configuration issues" row whose text points to the opposite cause. That is an outage presented as a hardening hint.
- **Fix**: Treat an explicit 0 as "all requests throttled (429)" and consider raising its severity. Do not describe it as a DoS risk.

## 5. P2 — The apigw→elb pivot matches any load balancer that shares a subnet or security group with the VPC link, not the one the integration targets

- **Location**: `core/aws/apigw_related.go:333-436`
- **Trigger**: An HTTP API with a `VPC_LINK` integration, in a VPC where other ALBs or NLBs sit in the same private subnets (the usual layout).
- **Cause**: A v2 VPC link carries only `SubnetIds` and `SecurityGroupIds`, and any load balancer with an AZ subnet in that set is counted. The actual target is in `Integration.IntegrationUri`, which for a `VPC_LINK` integration holds the ALB or NLB listener ARN. The spec (`docs/resources/apigw.md` §2 elb) also says to read from a targets field, not to intersect subnets.
- **User impact**: The Load Balancers count is inflated and lists unrelated load balancers.
- **Fix**: For `ConnectionType == VPC_LINK` integrations, take the load balancer from `IntegrationUri`: parse the listener ARN (`.../listener/app|net/<name>/<lbid>/<listenerid>`) into the load balancer ARN or ID and match it against the elb cache. Drop the subnet/security-group intersection.

## 6. P2 — The apigw→alarm pivot never matches REST API alarms

- **Location**: `core/aws/apigw_related.go:275-277`
- **Trigger**: A REST API with CloudWatch alarms on `AWS/ApiGateway` metrics.
- **Cause**: REST API metrics use the dimension `ApiName` (the API name, plus `Stage`). Only HTTP and WebSocket metrics use `ApiId`. The checker matches only `ApiId == res.ID`.
- **User impact**: Every REST API shows CloudWatch Alarms as a proven zero dead end, even when alarms exist.
- **Fix**: For REST rows, match `Namespace == "AWS/ApiGateway"` with `ApiName == res.Name`; keep `ApiId` for v2 rows.

## 7. P2 — Related-panel calls ignore pagination and report partial results as exact counts

- **Location**: `core/aws/apigw_related.go`
  - `:142-144`: `GetIntegrations` with no `NextToken` loop
  - `:217-219`: `GetDomainNames` has no loop
  - `:232-234`: `GetApiMappings` has no loop
  - `:481-483`: `GetAuthorizers` in `checkApigwRole` has no loop
- **Trigger**: An API with more integrations or authorizers than one page, or an account with more custom domains or mappings than one page.
- **Cause**: `out.NextToken` is never read, so only the first page is scanned. The results go out through `relatedResult` / `relatedResultTrunc(..., false)` as exact counts.
- **User impact**:
  - Lambda, KMS, role and ELB targets beyond page 1 disappear.
  - ACM certificates on later domains are missed.
  - Undercounts render as exact, and a zero renders as a proven-zero dead end.
- **Fix**: Page each call until `NextToken` is nil. The enricher's `apigwHTTPNoAuthorizer` already does this. Alternatively, mark the result truncated when a token is left unread.

## 8. P2 — alarm→apigw returns the `ApiName` dimension value as an API ID

- **Location**: `core/aws/alarm_related_extra.go:32`
- **Trigger**: Opening a REST API alarm, which has dimension `ApiName=<name>`.
- **Cause**: The alarm's `ApiName` value, a name, is returned as the apigw ID. apigw rows are keyed by API ID (`apigw.go:179-181`).
- **User impact**: The API Gateways row shows (1), but selecting it lands on an empty or not-found list.
- **Fix**: Resolve the name against the apigw cache (`Name == v`, `protocol == "REST"`) and return the matching IDs. Report Unknown when the cache is not available.

## 9. P2 — acm→apigw returns custom domain names as API IDs

- **Location**: `core/aws/acm_related.go:137-140`
- **Trigger**: Opening an ACM certificate used by an API Gateway custom domain. `InUseBy` then contains `arn:aws:apigateway:<region>::/domainnames/api.example.com`.
- **Cause**: The last path segment, the domain name, is appended as an apigw ID. No apigw row has a domain name as its ID.
- **User impact**: The API Gateways row counts domains, and selecting it leads nowhere.
- **Fix**: Resolve each domain to its API IDs with `apigatewayv2:GetApiMappings(DomainName)` (v1 `GetBasePathMappings` for edge domains). Otherwise report Unknown instead of returning domain names.

## 10. P2 — lambda→apigw matches APIs by substring of the API name and skips every REST API

- **Location**: `core/aws/lambda_related_extra.go:93-106`
- **Trigger**:
  - A function named, for example, `api` or `auth`.
  - Any REST API that fronts the function.
- **Cause**:
  - `strings.Contains(*api.Name, fnName)` reports every API whose name contains the function name, whether or not it integrates with the function.
  - `assertStruct[apigatewayv2types.Api]` fails on REST rows, whose RawStruct is `apigateway/types.RestApi`, so REST APIs are silently skipped.
  - `api.Tags[fnName]` reads a tag keyed by the function name, which is not an AWS convention.
- **User impact**: False-positive API Gateway counts on Lambda detail views, and no match for REST APIs that actually invoke the function.
- **Fix**: Derive the relation from integrations, either with a reverse lookup through the Lambda resource policy (`lambda:GetPolicy` statements with `SourceArn` `arn:aws:execute-api:...:<apiId>/...`) or with per-API `GetIntegrations`. Remove the name and tag heuristic.

## 11. P2 — r53→apigw extracts the regional domain label (`d-xxxx`), which never equals an API ID

- **Location**: `core/aws/r53_related.go:197-203`
- **Trigger**: A hosted zone with an alias record to an API Gateway custom domain.
- **Cause**: Route 53 alias targets for API Gateway are the custom domain's regional or edge target (`d-abc123.execute-api.<region>.amazonaws.com` or a CloudFront name). They are never `<apiId>.execute-api...`, so `d[:idx]` is `d-abc123` and matches no apigw row.
- **User impact**: The zone's API Gateways row is always a proven zero dead end, even when the zone fronts APIs.
- **Fix**: Map the `d-xxxx` regional domain name through `GetDomainNames` (`DomainNameConfigurations[].ApiGatewayDomainName`) and `GetApiMappings` to API IDs.

## 12. P3 — Missing access logs on an HTTP stage is reported under "stage configuration issues" instead of "no access logs"

- **Location**: `core/aws/apigw_issue_enrichment.go:164-176` (compare the code comment at `:33-35`)
- **Trigger**: An HTTP or WebSocket stage with `AccessLogSettings == nil`.
- **Cause**: The HTTP lane adds the gap to the `apigwCodeStageConfigIssues` rows. `CodeAPIGWNoAccessLogs` is documented as the one code for both lanes but is emitted only by the REST lane. The spec defines stage-config-issues as a gap "with no code of its own".
- **User impact**: The same gap shows as "no access logs" on REST rows and as "stage configuration issues" on HTTP rows, so filtering and badges by code miss the HTTP ones.
- **Fix**: Emit `CodeAPIGWNoAccessLogs` with a `Stage` row from `apigwHTTPRow`, and keep stage-config-issues for throttling only.

## 13. P3 — "No deployed stages" counts stages, not deployments

- **Location**: `core/aws/apigw_issue_enrichment.go:192-193`
- **Trigger**: An HTTP or WebSocket API with a stage that has never been deployed (`AutoDeploy` false, `DeploymentId` nil).
- **Cause**: The finding fires only when `len(stages) == 0`. The spec (§3.2) defines the signal as "no stage with a `DeploymentId`".
- **User impact**: An API that answers nothing, because its only stage has no deployment, shows as clean.
- **Fix**: Count stages with a non-empty `DeploymentId` and emit the finding when that count is 0.

## 14. P3 — apigw→logs matches access-log groups by name prefix, so other APIs' log groups are counted

- **Location**: `core/aws/apigw_related.go:119`, `:124`
- **Trigger**: APIs named `orders` and `orders-v2`: the `orders` API also matches `/aws/apigateway/orders-v2`.
- **Cause**: `strings.HasPrefix(logRes.ID, "/aws/apigateway/"+apiName)` has no terminator.
- **User impact**: Inflated Log Groups count that includes other APIs' groups.
- **Fix**: Match `== prefix` or `HasPrefix(prefix + "/")`. The spec's source is `Stage.AccessLogSettings.DestinationArn`, which avoids guessing.

## 15. P3 — Lambda target parsing treats stage-variable placeholders and cross-account functions as local function names

- **Location**: `core/aws/apigw_related.go:49-64` (kms), `:175-190` (lambda)
- **Trigger**:
  - An integration URI such as `...:function:${stageVariables.fn}/invocations`.
  - A Lambda function in another account or region.
- **Cause**: Everything after `:function:` is taken as a bare name. The placeholder becomes the ID `${stageVariables.fn}`, and the account and region of the ARN are discarded.
- **User impact**:
  - The Lambda Functions row counts a function that does not exist.
  - The KMS pivot calls `GetFunction` with the placeholder, which fails and sets truncated or error.
  - Cross-account targets resolve to a same-named local function, or to nothing.
- **Fix**: Skip names containing `${`, and pass the full function ARN, not the bare name, to `GetFunction` and to the related IDs.

## 16. P3 — The HTTP-lane `GetStages` call is not wrapped in `RetryOnThrottle`

- **Location**: `core/aws/apigw_issue_enrichment.go:118-121`
- **Trigger**: A Wave 2 sweep over many HTTP APIs at `EnrichmentParallelism`, which exceeds the API Gateway control-plane rate limit.
- **Cause**: The REST lane and the authorizer walk use `RetryOnThrottle`, but this call does not, so a single `TooManyRequestsException` goes straight to `MarkSkipped`.
- **User impact**: Rows are skipped at random during sweeps and their stage findings are missing.
- **Fix**: Wrap the call in `RetryOnThrottle(ctx, DefaultRetryConfig(), ...)`, as the sibling calls are.

## asg

# asg — production-code review

Scope: `core/aws/asg*.go`, the `asg` / `asg_activities` catalog entries in `core/aws/catalog_compute.go`, `core/config/defaults_compute.go`, and the reverse pivots into `asg` (`ec2`, `ami`, `lt`, `tg`, `subnet`, `eip`, `eks`, `ng`, `ecs`, `eb`, `alarm`).

Totals: 8 findings (P0:0 P1:1 P2:3 P3:4)

---

## 1. [P1] asg → elb pivot emits IDs that no `elb` row carries: the drill-down is empty

- **File:** `core/aws/asg_related.go:160` (classic names) and `core/aws/asg_related.go:186` (`ids = append(ids, tg.LoadBalancerArns...)`)
- **Evidence:** the `elb` fetcher keys every row by load-balancer **name** (`core/aws/elb.go:64`, `ID: lbName`) and is ELBv2-only (`catalog_networking.go:117-118`, `FetchLoadBalancersPage(ctx, c.ELBv2, …)`). `elb` registers no `FetchByIDs`, and the related drill keeps only rows whose `r.ID` is in the result's ID set (`core/app/list_filter.go:19-29`). Every other pivot into `elb` returns `elbRes.ID` (for example `tg_related.go:72`, `sg_related.go:114`, `cf_related.go:99`).
- **Trigger:** an ASG attached to an ALB/NLB target group, or to a classic ELB, opens its detail view.
- **User impact:** the Load Balancers row reads "(1)" (or more). Drilling into it opens an empty list, because no row ID matches an ARN. Classic ELB names are also counted, but the `elb` list can never show them. This breaks the "is traffic reaching new instances?" pivot for every group behind a load balancer.
- **Fix direction:** map `LoadBalancerArns` to `elb` row IDs by matching `Fields["load_balancer_arn"]` in the loaded `elb` list, as the other pivots do. Either drop classic names or report them as a count the drill can show.

## 2. [P2] "no load balancer health check" fires on groups that already use ELB health checks

- **File:** `core/aws/asg.go:139`
- **Evidence:** `aws.ToString(asg.HealthCheckType) != "ELB"`. AWS documents `HealthCheckType` as "One or more comma-separated health check types" (valid values `EC2`, `EBS`, `ELB`, `VPC_LATTICE`), so a value such as `ELB,EBS` is valid.
- **Trigger:** a group behind a target group with ELB and EBS health checks both enabled (`HealthCheckType = "ELB,EBS"`).
- **User impact:** the row turns warning-yellow and shows "no load balancer health check" even though ELB health checks are on. The detail row reads `elb,ebs` next to advice to enable the load balancer's check.
- **Fix direction:** split on `,` and fire only when no element equals `ELB` (or `VPC_LATTICE`).

## 3. [P2] asg ↔ alarm pivot matches the metric dimension, not the alarms that drive the group's scaling policies

- **Files:** `core/aws/asg_related.go:43` (`alarmIDsByDimension(…, "AutoScalingGroupName", res.ID)`) and the reverse pivot `core/aws/alarm_related.go:54`
- **Evidence:** `docs/resources/asg.md` §2 and `docs/related-resources.md` (`alarm`→`asg`: "MetricAlarm.AlarmActions pointing at ASG scaling policies") define the link as the scaling-policy ARN in `AlarmActions`, which contains `autoScalingGroupName/<ASG>`. The code only looks at `Dimensions`.
- **Trigger:** a step-scaling policy driven by an SQS queue-depth alarm (dimension `QueueName`), or target tracking on `ALBRequestCountPerTarget`, whose alarms carry the `LoadBalancer`/`TargetGroup` dimensions.
- **User impact:** the "why is this group scaling?" pivot shows "(0)" or leaves out the alarms that actually scale the group. From those alarms, the reverse `asg` pivot reads zero. At the same time, alarms that only watch the group's metrics are counted as if they trigger scaling.
- **Fix direction:** match `AlarmActions` entries of the form `arn:…:autoscaling:…:scalingPolicy:…:autoScalingGroupName/<name>:…` in both directions. Keep the dimension match only if the contract is changed to allow it.

## 4. [P2] ECS cluster → asg lists every ECS-managed ASG in the region under every cluster

- **File:** `core/aws/ecs_related_extra.go:44-47`
- **Evidence:** a group qualifies as soon as it carries the `AmazonECSManaged` tag key. `clusterName` is never compared on that branch.
- **Trigger:** an account with two or more ECS clusters that each have a managed-scaling capacity provider backed by an ASG.
- **User impact:** each cluster's Auto Scaling Groups pivot shows the other clusters' container-instance ASGs, both in the count and in the drill.
- **Fix direction:** resolve the cluster's `CapacityProviders` to `AutoScalingGroupProvider.AutoScalingGroupArn` (DescribeCapacityProviders), or at minimum require a cluster-specific match rather than the tag's presence alone.

## 5. [P3] "launch configuration allows IMDSv1" fires when the metadata endpoint is disabled

- **File:** `core/aws/asg_issue_enrichment.go:205`
- **Evidence:** the rule looks only at `HttpTokens`. The sibling launch-template rule treats `HttpEndpoint == disabled` as "metadata unreachable" and does not flag it (`core/aws/lt.go:163-166`).
- **Trigger:** a launch configuration with `MetadataOptions{HttpEndpoint: disabled, HttpTokens: optional}`.
- **User impact:** a false IMDSv1/SSRF warning on a group whose instances cannot reach IMDS at all. It also contradicts what the same configuration produces on the `lt` type.
- **Fix direction:** skip the finding when `lc.MetadataOptions.HttpEndpoint == asgtypes.InstanceMetadataEndpointStateDisabled`.

## 6. [P3] Launch-template resolution falls back to `$Latest`, but AWS's default is `$Default`

- **Files:** `core/aws/asg_related.go:124` (ami), `core/aws/asg_related.go:278` (role), `core/aws/asg_related_extra.go:118` (sg)
- **Evidence:** the AWS `LaunchTemplateSpecification.Version` reference says "The default value is `$Default`". `docs/related-resources.md` (lt section) also says "`$Default` (not `$Latest`) is what `asg`/`ng`/`ec2` actually resolve at launch".
- **Trigger:** a group whose `LaunchTemplate` spec has no `Version`, where the template's latest version differs from its default.
- **User impact:** the AMI, IAM Roles and Security Groups pivots describe a staged template version the group does not launch from, so the rollback AMI or the SGs shown are wrong.
- **Fix direction:** fall back to `$Default` in all three places, ideally through one shared resolver.

## 7. [P3] ami/sg/role pivots ignore per-override launch templates in a MixedInstancesPolicy

- **Files:** `core/aws/asg_related.go:116-119`, `core/aws/asg_related.go:268-271`, `core/aws/asg_related_extra.go:110-113`
- **Evidence:** only `MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification` is read. `Overrides[].LaunchTemplateSpecification` is never read, yet `checkLTASG` (`core/aws/lt_related.go:139-143`) counts those overrides as the group's templates. Each result is returned as exact (`relatedResult`, not truncated).
- **Trigger:** a mixed-architecture group, for example x86 base template plus an arm64 override template with a different AMI, instance profile or SGs.
- **User impact:** the AMI, SG and role pivots show an exact count that leaves out the override templates' AMIs, SGs and roles.
- **Fix direction:** resolve every distinct override specification alongside the base one, or mark the result truncated when overrides carry their own template.

## 8. [P3] Instance-profile ARNs with a path are sent to GetInstanceProfile as "path/name"

- **File:** `core/aws/asg_related.go:317-321`
- **Evidence:** everything after `:instance-profile/` is kept, so `arn:aws:iam::<acct>:instance-profile/app/web` becomes `app/web`. `GetInstanceProfile` takes a bare name.
- **Trigger:** a launch template or launch configuration that names an instance profile created under an IAM path.
- **User impact:** the call fails, and the IAM Roles pivot shows unknown or a lower bound instead of the instance role.
- **Fix direction:** use the last `/` segment of the ARN's resource part as `InstanceProfileName`.

## athena

# athena — production code review

Scope: `core/aws/athena.go`, `athena_codes.go`, `athena_interfaces.go`, `athena_issue_enrichment.go`, `athena_related.go`, the `athena` catalog entry in `core/aws/catalog_data.go`, `core/config/defaults_data.go` (athena detail), plus the inbound pivots `checkS3Athena` (`core/aws/s3_related.go`) and `checkGlueAthena` (`core/aws/glue_related.go`). SDK checked: `github.com/aws/aws-sdk-go-v2/service/athena v1.66.0`.

## 1. P1 — Logs pivot reads the metrics flag and shows a made-up log group

- **File**: `core/aws/athena_related.go:82-88` (`checkAthenaLogs`)
- **Trigger**: any workgroup with `Configuration.PublishCloudWatchMetricsEnabled == true`. The console turns this on by default.
- **Code**: the pivot tests `PublishCloudWatchMetricsEnabled`, which only controls CloudWatch **metrics**, and then builds a log-group name `"/aws/athena/" + res.ID`. AWS does not create a log group with that name. The field that actually names a log group is `Configuration.MonitoringConfiguration.CloudWatchLoggingConfiguration.LogGroup` (when `Enabled == true`). It exists on `WorkGroupConfiguration` in SDK v1.66.0 and is the field `docs/resources/athena.md` §2 `logs` specifies. The code never reads it.
- **User impact**: most workgroups show "Log Groups: 1". Drilling in points at a log group that does not exist. Spark workgroups that do have a configured log group are shown as 0 unless metrics happen to be on, and even then they show the made-up name instead of the real group.
- **Fix**: read `MonitoringConfiguration.CloudWatchLoggingConfiguration`. When `Enabled` is true and `LogGroup` is not empty, return that group. Otherwise return a count of 0. Drop both the `PublishCloudWatchMetricsEnabled` check and the synthesized name.

## 2. P2 — "query results stored unencrypted" fires on workgroups that use managed query results

- **File**: `core/aws/athena_issue_enrichment.go:84-91`
- **Trigger**: a workgroup with `Configuration.ManagedQueryResultsConfiguration.Enabled == true`. AWS rejects `ResultConfiguration.OutputLocation` on such a workgroup, so `ResultConfiguration` or its `EncryptionConfiguration` is typically nil.
- **Code**: the finding fires whenever `ResultConfiguration == nil || ResultConfiguration.EncryptionConfiguration == nil`. It never looks at `ManagedQueryResultsConfiguration`. AWS docs state that managed query results are encrypted by default with an AWS owned key, or with a customer managed key when `ManagedQueryResultsConfiguration.EncryptionConfiguration` is set.
- **User impact**: a yellow row says "query results stored unencrypted" (and "written to S3 … readable by anyone who can read the results bucket") for a workgroup whose results are encrypted and are not in any customer bucket. This is a false security warning.
- **Fix**: skip the finding when `ManagedQueryResultsConfiguration != nil && Enabled`.

## 3. P2 — The "Cost Cap" list column is always blank

- **File**: `core/aws/catalog_data.go:105` (`{Title: "Cost Cap", Path: "Configuration.BytesScannedCutoffPerQuery"}`)
- **Trigger**: every athena list render.
- **Code**: list columns are resolved with `fieldpath.ExtractScalar(r.RawStruct, col.Path)` (`core/app/list_columns.go:57`). For athena, `RawStruct` is the `types.WorkGroupSummary` returned by `ListWorkGroups` (`core/aws/athena.go:85`). That struct has only `CreationTime`, `Description`, `EngineVersion`, `IdentityCenterApplicationArn`, `Name` and `State`, with no `Configuration`. The fetcher also sets no field key for this value.
- **User impact**: the column takes up 12 characters and is empty on every row, so a workgroup with a per-query scan limit looks the same as one without.
- **Fix**: the fetcher already calls `GetWorkGroup` (see #4). Copy `Configuration.BytesScannedCutoffPerQuery` into a `Fields` key and give the column that `Key`. The other option is to remove the column.

## 4. P2 — Fetcher runs GetWorkGroup serially and silently drops errors; the S3 → Athena pivot then reports a confirmed 0

- **File**: `core/aws/athena.go:35,62-72`. Consumer: `core/aws/s3_related.go:288-294` (`checkS3Athena`).
- **Trigger**: a list page of up to 50 workgroups, or any `GetWorkGroup` failure (AccessDenied, throttling) during the list fetch.
- **Code**: every list page makes up to 50 `GetWorkGroup` calls in sequence, one after another, before it returns. On any error, `result_output_location` is left as `""`. That value cannot be told apart from "no output location configured", and the row carries no truncated or unknown marker. `checkS3Athena` matches buckets against `Fields["result_output_location"]` and returns a known count (`relatedResultTrunc`).
- **User impact**:
  - The athena list waits for up to 50 sequential round-trips per page.
  - When `athena:GetWorkGroup` is denied or throttled, the S3 bucket detail shows "Athena WorkGroups: 0" as a known answer even when workgroups write results to that bucket.
  - The same workgroup configuration is fetched a third time by the Wave 2 enricher and again by every related checker.
- **Fix**: run the per-workgroup calls with `ForEachParallel`/`EnrichmentParallelism`. Record a lookup failure on the resource, for example with a marker field, and have `checkS3Athena` return `UnknownRelated` when any cached workgroup lacks a successfully read location.

## 5. P3 — KMS pivot ignores the other encryption keys

- **File**: `core/aws/athena_related.go:63-71` (`checkAthenaKMS`)
- **Trigger**: a workgroup encrypted through `Configuration.CustomerContentEncryptionConfiguration.KmsKey` (Spark), or through `Configuration.ManagedQueryResultsConfiguration.EncryptionConfiguration.KmsKey` (managed results with a CMK), and without `ResultConfiguration.EncryptionConfiguration.KmsKey`.
- **Code**: only `ResultConfiguration.EncryptionConfiguration.KmsKey` is read. `docs/resources/athena.md` §2 `kms` also requires `CustomerContentEncryptionConfiguration.KmsKey`.
- **User impact**: "KMS Keys: 0" appears for a workgroup that is encrypted with a customer key, so there is no pivot to the key that protects its data.
- **Fix**: collect the key from all three fields, remove duplicates, and pass the result to `relatedResult`.

## 6. P3 — Glue → Athena pivot links a job to a workgroup only because the names match

- **File**: `core/aws/glue_related.go:215` (`checkGlueAthena`)
- **Trigger**: a Glue job whose name equals an Athena workgroup name, for example a job named `primary`.
- **Code**: `wg.Fields["glue_job"] == jobName || wg.ID == jobName`. The athena fetcher never sets `glue_job`, so the only way to match is a name collision. No AWS field links a Glue job to a workgroup.
- **User impact**: the Glue job detail shows "Athena WorkGroups: 1" and pivots to an unrelated workgroup. For every other job it shows a known 0 that nothing actually checked.
- **Fix**: remove the name-equality match. Without a real linking field, drop the pivot from the glue catalog entry, or return `UnknownRelated`.

Sources: [Managed query results](https://docs.aws.amazon.com/athena/latest/ug/managed-results.html), [Encrypt managed query results](https://docs.aws.amazon.com/athena/latest/ug/encrypting-managed-results.html), [ManagedQueryResultsConfiguration](https://docs.aws.amazon.com/athena/latest/APIReference/API_ManagedQueryResultsConfiguration.html)

## backup

# backup — production-code review

Scope: `core/aws/backup.go`, `backup_interfaces.go`, `backup_match.go`, `backup_coverage.go`, `backup_issue_enrichment.go`, `backup_related.go`, `catalog_backup.go`, `core/config/defaults_backup.go`, plus the helpers they call (`walkAccountPages`, `setWave2Finding`, `fillPhrase`, `backupSelectionTagsMatch`, `selector.MatchARN`) and the callers of the backup plan fields.

AWS semantics verified against the AWS Backup developer guide ("Assign resources with AWS CLI"): within one selection, the `Resources` entries are ORed with each other, `Conditions` entries are ANDed with each other, and **`Resources` is ANDed with `Conditions`**. For example, `Resources:["*"]` plus `StringEquals backup=true` means "all resources tagged backup=true". `ListOfTags` is ORed with `Resources`. `NotResources` excludes only from its own selection. AWS's own example, "all resources tagged backup=true except EBS volumes tagged stage=test", uses two selections: one with `NotResources: volume/*`, and one that selects `volume/*`.

## Findings

### 1. P1: Conditions-based tag selections are read as OR with `Resources`, so a `"*"` + tag plan covers every resource

- **File/line:** `core/aws/backup.go:188-191` (flattening), with `backup.go:238-240` (a single positive `Conditions` entry is accepted as representable), used at `core/aws/backup_coverage.go:168` and `core/aws/backup_match.go:17-38`.
- **Trigger:** A plan has a selection built the recommended way, from the console ("include all resource types" plus "refine selection using tags") or from the CLI: `Resources:["*"]` (or `arn:aws:ec2:*:*:volume/*`, `arn:aws:rds:*:*:db:*`) with `Conditions.StringEquals aws:ResourceTag/backup=true`. The fetcher stores `resources="*"` and `selection_tags="backup=true"` separately on the plan row. `backupPlanCovers` returns true as soon as the ARN list matches (`backup_coverage.go:168`), without checking the tag condition that AWS ANDs with it.
- **User impact:** Every DynamoDB table, DB instance, DB cluster and EBS volume in the account counts as covered, including untagged ones AWS Backup never backs up. The "not in a backup plan" finding (ddb/dbi/dbc/ebs) is silenced account-wide, which is the operator's main signal for unprotected data. The related-panel `backup` pivots use the same flattened fields through `BackupPlanCoversARN` (ddb, efs, dbi-snap, dbc-snap, ec2, s3), so they also list the plan against resources it does not protect.
- **Fix direction:** Model coverage per selection instead of through three plan-wide CSVs. Store each selection's Resources, NotResources, ListOfTags and Conditions as structured data. Evaluate each selection as `(Resources match OR ListOfTags match) AND all Conditions hold AND NOT NotResources`, with an empty Resources list meaning "all", and consider the plan covered if any selection covers the resource. Replace `BackupPlanCoversARN`, `backupPlanCovers` and `backupSelectionTagsMatch` with this one evaluator so the finding and the pivots cannot diverge.

### 2. P1: `NotResources` from one selection is applied to every selection of the plan

- **File/line:** `core/aws/backup.go:189` (all selections' `NotResources` merged into one plan-wide `not_resources`), used at `core/aws/backup_coverage.go:165` and `core/aws/backup_match.go:20-28`.
- **Trigger:** A plan uses AWS's documented two-selection pattern. Selection A is `Resources:["*"]`, `NotResources:["arn:aws:ec2:*:*:volume/*"]`, and selection B is `Resources:["arn:aws:ec2:*:*:volume/*"]`, each with tag conditions. Or more simply: selection A includes `volume/vol-1` explicitly, while selection B is `"*"` with `NotResources: vol-1`. The merged `not_resources` excludes the volumes from the whole plan, and exclusion is checked first.
- **User impact:** EBS volumes (or any excluded ARN) that selection B does back up get a false broken-row "not in a backup plan" finding. The ebs/ec2/ddb/dbi/efs related `backup` pivots show 0 plans for them. The operator is told data is unprotected when it is protected.
- **Fix direction:** Same as finding 1: evaluate `NotResources` only against its own selection's includes.

### 3. P2: The partial-job phrase reports job counts as "resources skipped"

- **File/line:** `core/aws/backup_issue_enrichment.go:98` (`totalCount` counts every in-window job of the plan), `:159-160` and `:165` (the counts fill `partial: <N> of <M resource(s)> skipped`).
- **Trigger:** A plan backs up one resource hourly. In 24h it has 24 jobs, one of them `PARTIAL`. The row and status cell read `partial: 1 of 24 resources skipped`.
- **User impact:** The status text gives a wrong number of affected resources: there is one resource, not 24. It also says a resource was "skipped" when a `PARTIAL` job is a composite job whose child jobs partly failed. Any plan that runs more than once per day in the window overstates M. A plan whose jobs are all partial can read `N of N resources skipped`, which suggests nothing was backed up.
- **Fix direction:** Either count distinct `BackupJob.ResourceArn` values (partial resources out of distinct resources in the window), or change the declared phrase to talk about jobs (`<N> of <M job(s)> partial in last 24h`) so the numbers match the words.

### 4. P3: The backup → role pivot reads only the first page of `ListBackupSelections` and presents the count as exact

- **File/line:** `core/aws/backup_related.go:30-34` (one call, `NextToken` ignored), `:53` (`relatedResult` with `truncated=false`).
- **Trigger:** `ListBackupSelections` returns a `NextToken` (the API is paginated, MaxResults 1–1000, and the default page size is not documented) for a plan with many selections. The backup fetcher itself pages this same API (`backup.go:158-195`).
- **User impact:** IAM roles bound to selections past the first page are left out of the related panel, and the count shows as complete instead of `N+`. The operator investigating a permissions failure can miss the role actually in use.
- **Fix direction:** Loop on `NextToken`, bounded by `PerParentPageCap` as the fetcher does. When the cap is hit or a later page fails, return `relatedResultTrunc("role", ids, true)`.

## cb

# cb (CodeBuild Projects): production-code review

Scope: `core/aws/cb.go`, `cb_interfaces.go`, `cb_codes.go`, `cb_builds.go`, `cb_builds_codes.go`, `cb_build_logs.go`, `cb_issue_enrichment.go`, `cb_related.go`, `codebuild_related.go`, the cb / cb_builds / cb_build_logs entries in `catalog_cicd.go`, and the reverse pivots into cb (`secrets_related.go`, `ecr_related.go`, `pipeline_related.go`, `alarm_related_extra.go`). The SDK checked was `service/codebuild v1.78.0`, the version in go.mod.

Summary: 10 findings (P0:0 P1:0 P2:5 P3:5)

---

## 1. P2: SECRETS_MANAGER env vars in ARN form or with a JSON key point the pivot at IDs that do not exist

- **File/line:** `core/aws/cb_related.go:268-280` (forward `checkCbSecrets`); `core/aws/secrets_related.go:234-236` (reverse `checkSecretsCB`)
- **Trigger:** a project has a `SECRETS_MANAGER` env var whose value is in one of the documented `secret-id:json-key:version-stage:version-id` forms:
  - (a) A full ARN such as `arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db-AbCdEf`. The code keeps the text after `:secret:`, which is `prod/db-AbCdEf` and still has the 6-character suffix AWS appends. Secret rows use the plain name as `Resource.ID` (`secrets.go:71`), so this ID never matches.
  - (b) A name with a JSON key, such as `prod/db:password`. Nothing is stripped, so the ID is `prod/db:password`.
  - (c) The reverse direction compares `val == secretARN` or `HasPrefix(val, name+":")`, so an ARN value with a JSON-key suffix (`...:secret:prod/db-AbCdEf:password::`) matches nothing.
- **User impact:** the cb detail panel shows "Secrets Manager 1", but opening it finds no secret. The secret's panel reports 0 CodeBuild projects for a project that injects it. The two directions of one relation disagree.
- **Fix direction:** write one shared parser for a CodeBuild secret reference. It should cut at the first `:` after the name part, and for an ARN it should also drop the `-XXXXXX` suffix (or match on the secret row's `arn` field). Use it in both `checkCbSecrets` and `checkSecretsCB`.

## 2. P2: an S3-hosted buildspec ARN is reported as "buildspec taken from the source repository"

- **File/line:** `core/aws/cb.go:145-155` (`cbBuildspecFromSource`)
- **Trigger:** a GitHub, Bitbucket or CodeCommit project with `Source.Buildspec = "arn:aws:s3:::ops-bucket/buildspec.yml"`. The SDK doc for `ProjectSource.Buildspec` lists this as a valid form. The value ends in `.yml`, so the `HasSuffix` branch returns true. The opposite case also fails: a relative path in the repository that does not end in `.yml`/`.yaml` (for example `ci/buildspec`) hits the final `return "", false`, so no finding is raised.
- **User impact:** a project that follows the finding's own advice (buildspec kept outside the pull-request-controlled repository) still shows a yellow "buildspec taken from the source repository" row. A repository path without a `.yml` suffix is silently treated as healthy.
- **Fix direction:** check for an `arn:aws:s3:::` prefix first and treat it as not from source. Then treat any single-line value that is not an S3 ARN as a repository-relative path, whatever its suffix.

## 3. P2: GitLab and self-managed GitLab sources are never checked for the buildspec risk

- **File/line:** `core/aws/cb.go:130-135` (`cbContributorSourceTypes`)
- **Trigger:** a project whose `Source.Type` is `GITLAB` or `GITLAB_SELF_MANAGED` (both are in the SDK enum, `types/enums.go:1101-1102`) and whose buildspec is read from the repository. The set only lists GITHUB, GITHUB_ENTERPRISE, BITBUCKET and CODECOMMIT.
- **User impact:** GitLab-backed projects never show the warning. Anyone who can open a merge request can change what runs under the build role, and a9s reports the project as clean.
- **Fix direction:** add `SourceTypeGitlab` and `SourceTypeGitlabSelfManaged` to the set.

## 4. P2: a username-only source URL, which is Bitbucket's standard clone URL, is flagged as a leaked credential

- **File/line:** `core/aws/cb.go:174` (`cbSourceLocationCredential`)
- **Trigger:** `Source.Location = "https://jdoe@bitbucket.org/team/repo.git"`, the HTTPS clone URL Bitbucket gives out. The guard only returns healthy when there is no password and the username is empty, so any userinfo with a plain username raises the finding.
- **User impact:** a red Broken row, "credential in the source repository address", with advice to rotate a credential that does not exist. This is a false broken count on the main-menu badge for every Bitbucket project configured in the usual way.
- **Fix direction:** raise the finding when a password is present. When there is only a username, raise it only if the username looks like a token (reuse the secret-scan heuristic that `addSecretScanFinding` already applies).

## 5. P2: the Build Logs view drops everything older than the last GetLogEvents page and still reports itself complete

- **File/line:** `core/aws/cb_build_logs.go:26-30`, `:84-90`; `core/aws/catalog_cicd.go:406-407`
- **Trigger:** a build log larger than one GetLogEvents response (1 MB or 10,000 events). The call uses `StartFromHead=false`, ignores `continuationToken`, discards `NextBackwardToken`, and returns `IsTruncated: false, TotalHint: len(resources)`.
- **User impact:** for long builds the operator sees only the tail. The start of the build (source download, install phase, the first error) is missing, and nothing says so because the count reads as complete.
- **Fix direction:** pass `continuationToken` as `NextToken` and return `NextBackwardToken` with `IsTruncated=true` when it differs from the token that was sent (GetLogEvents signals the end by returning the same token). At a minimum, mark the result as truncated.

## 6. P3: the latest-build finding repeats the "Ended" row

- **File/line:** `core/aws/cb_issue_enrichment.go:106-108` and `:124-126`
- **Trigger:** any project whose latest build is FAILED, FAULT or TIMED_OUT and has an `EndTime`.
- **User impact:** the detail finding block shows "Ended <date>" twice, once untiered and once tiered `!`. The duplicate also uses a slot in the capped row budget (`capRows`).
- **Fix direction:** delete one of the two appends and keep the single row at the tier you want.

## 7. P3: the failed-phase row only appears for FAILED phases, not FAULT or TIMED_OUT

- **File/line:** `core/aws/cb_issue_enrichment.go:116-122`
- **Trigger:** the latest build is TIMED_OUT or FAULT. The breaking phase then has `PhaseStatus` TIMED_OUT or FAULT, but the loop only matches `StatusTypeFailed`.
- **User impact:** for timeouts and platform faults the finding never names the phase that was running or broke. The catalog's detail text for this finding tells the operator to find that phase.
- **Fix direction:** match any non-SUCCEEDED terminal `PhaseStatus` (FAILED, FAULT, TIMED_OUT, STOPPED).

## 8. P3: the Status column shows "OK" for builds that are in progress or were stopped

- **File/line:** `core/aws/cb_issue_enrichment.go:91-92`
- **Trigger:** the latest build is IN_PROGRESS or STOPPED.
- **User impact:** `last_build`, the list's Status column (the LifecycleKey), reads "OK" for a build that has not finished or was cancelled. That claims a success nobody observed; the previous build may have failed.
- **Fix direction:** write "OK" only for SUCCEEDED. Write the humanized status ("in progress", "stopped") for the other two, still without a finding.

## 9. P3: the ECR pivots ignore the registry, and the two directions match differently

- **File/line:** `core/aws/cb_related.go:153`, `:181-195` (forward matches on repository name only); `core/aws/ecr_related.go:78` (reverse uses `strings.Contains(image, repoURI)`)
- **Trigger:**
  - (a) Forward: the build image is `999999999999.dkr.ecr.us-east-1.amazonaws.com/base` from a shared-services account, and the current account also has a repository named `base`. The pivot links to the wrong, local repository.
  - (b) Reverse: repository `app` has URI `.../app`. A project whose image is `.../app-worker:latest` contains that string, so it is listed as using `app`.
- **User impact:** wrong ECR↔CodeBuild links in both panels, and the two directions do not agree with each other.
- **Fix direction:** one helper that parses the image into `registry/repo` (dropping `:tag` or `@digest`) and compares it exactly with the ECR row's `uri` field. Use it from both checkers.

## 10. P3: the S3 pivot skips source and log locations that the spec lists

- **File/line:** `core/aws/cb_related.go:216-227`
- **Trigger:** a project with an S3 bucket only in `SecondarySources[].Location` (type S3) or in `LogsConfig.S3Logs.Location`. The spec (docs/resources/cb.md §2 s3) names both.
- **User impact:** the S3 Buckets pivot shows 0 for a bucket the build reads source from or writes logs to.
- **Fix direction:** also read S3-typed `SecondarySources[].Location` and `LogsConfig.S3Logs.Location`. The latter can be `arn:aws:s3:::bucket/prefix`, so strip the `arn:aws:s3:::` prefix before cutting at the first `/`.

## cf

# cf (CloudFront Distributions): production-code review

Scope: `core/aws/cf.go`, `cf_interfaces.go`, `cf_issue_enrichment.go`, `cf_related.go`, the `cf` entry in `catalog_dns_cdn.go`, and inbound pivots to cf (`checkR53CF`, `checkACMCF`, `checkS3CF`, `checkELBCF`, `checkApigwCF`, `checkLambdaCF`, `checkWAFCF`).

## 1. P2: every default-certificate distribution is flagged "minimum TLS below 1.2"

- **Where:** `core/aws/cf_issue_enrichment.go:132-136`
- **Trigger:** a distribution that serves on its `*.cloudfront.net` name (`ViewerCertificate.CloudFrontDefaultCertificate == true`). AWS says: "If the distribution uses the CloudFront domain name … CloudFront automatically sets the security policy to `TLSv1` regardless of the value that you set here" (API_ViewerCertificate). `GetDistributionConfig` therefore returns `TLSv1`, and `cfTLSBelow12Word` maps that to "TLS 1.0".
- **Impact:** `cf.deprecated-tls` fires on every default-certificate distribution. The operator cannot fix this with the setting the finding tells them to change. The spec (§3.2) limits this signal to `CloudFrontDefaultCertificate == false`, and the §4 table calls it a "Weak TLS policy on aliased distribution".
- **Fix:** run the TLS check only when `!aws.ToBool(vc.CloudFrontDefaultCertificate)`.

## 2. P2: the Lambda@Edge pivot drops functions after a duplicate

- **Where:** `core/aws/cf_related.go:300-301`
- **Trigger:** one cache behavior attaches the same function to two events (for example viewer-request and origin-request), and a different function comes later in the same `LambdaFunctionAssociations.Items`. On the duplicate, `if seen[name] { return }` exits the whole `collect` closure instead of skipping that one item.
- **Impact:** the later functions on that behavior are left out. The Lambda@Edge count is too low, and the operator never sees an edge function that may be causing 5xx errors.
- **Fix:** change `return` to `continue`.

## 3. P2: the "Log Groups" pivot returns the S3 access-log bucket, and the s3 pivot leaves it out

- **Where:** `core/aws/cf_related.go:343-347` (`checkCfLogs`) and `core/aws/cf_related.go:21-58` (`checkCfS3`)
- **Trigger:** a distribution with standard logging enabled (`Logging.Bucket = mylogs.s3.amazonaws.com`).
- **Impact:** `checkCfLogs` returns the bucket name `mylogs` as a related `logs` ID. `logs` is the CloudWatch Log Groups type, so the panel shows "Log Groups (1)" and drilling in finds no log group of that name. The spec (§2 `logs` / `s3`) says standard logging goes to S3 and belongs under the `s3` pivot. `checkCfS3` only matches origins, so the log bucket is missing from "S3 Buckets".
- **Fix:** add the `Logging.Bucket` bucket to the s3 pivot. That bucket is only in `GetDistributionConfig`, not in `DistributionSummary`. Make `logs` return unknown or empty until there is a real log-group mechanism, as the spec says (TBD).

## 4. P2: the acm, alarm and lambda pivots read the session region, but CloudFront keeps these in us-east-1

- **Where:**
  - `core/aws/cf_related.go:144-161` (`checkCfACM`, through the regional `ACM` client at `core/aws/client.go:201`)
  - `core/aws/cf_related.go:253-255` (`checkCfAlarm`, through the regional `CloudWatch` client at `client.go:191`)
  - `core/aws/cf_related.go:260-315` (`checkCfLambda` returns function names for the regional `lambda` list, `client.go:190`)
- **Trigger:** any session region other than us-east-1. CloudFront only accepts ACM certificates from us-east-1 (API_ViewerCertificate, `ACMCertificateArn`). CloudFront metrics, and so the alarms on them, live only in us-east-1. Lambda@Edge functions are created in us-east-1.
- **Impact:** in eu-west-1, for example, "ACM Certificates" and "CloudWatch Alarms" show a definite, non-truncated 0 for distributions that do have a certificate and alarms. "Lambda@Edge" shows a count, but drilling in opens a regional Lambda list that does not contain those functions. `WAFv2CloudFront` (`client.go:125-129`, 214) already solves this problem for WAF.
- **Fix:** add us-east-1-pinned ACM, CloudWatch and Lambda clients or target fetches for cf pivots, following the `WAFv2CloudFront` pattern. If that is not done, return unknown instead of a known 0 when `clients.Region != "us-east-1"`.

## 5. P3: `cf.insecure-protocol` ignores non-default cache behaviors

- **Where:** `core/aws/cf_issue_enrichment.go:221-229`
- **Trigger:** `DefaultCacheBehavior` is `redirect-to-https`, but an ordered cache behavior (for example `/api/*`) in `cfg.CacheBehaviors.Items` has `ViewerProtocolPolicy == allow-all`.
- **Impact:** the distribution still accepts plain HTTP on that path, and no finding is raised. The spec signal is "viewer allows plain HTTP", and the finding's own detail says "The distribution accepts plain HTTP from viewers".
- **Fix:** also loop over `cfg.CacheBehaviors.Items` and add a row with the path pattern for each `allow-all` behavior.

## 6. P3: HeadBucket network calls run under the enricher's shared mutex

- **Where:** `core/aws/cf_issue_enrichment.go:209-210` (`mu.Lock(); defer mu.Unlock()`) held across `cfConfigFindings` at line 261, which calls `bucketGone`, which calls `s3:HeadBucket` (line 76).
- **Trigger:** several distributions whose S3 origins name buckets missing from a complete s3 cache, for example cross-account origins.
- **Impact:** all HeadBucket calls run one after another behind a single lock, and every other worker's result merge waits on each call. That defeats `EnrichmentParallelism` and can make Wave 2 for cf take much longer. This breaks the pattern `mergeRowResult` documents (`core/aws/issue_enrichment.go:229-236`): evaluate with the lock released, and hold it only for the merge.
- **Fix:** evaluate each distribution into a per-row `IssueEnricherResult` without the lock, then merge with `mergeRowResult`.

## 7. P3: the WAF → cf pivot ignores `ListDistributionsByWebACLId` truncation

- **Where:** `core/aws/waf_related.go:141-156`
- **Trigger:** a CLOUDFRONT-scope Web ACL attached to more distributions than one page returns (the default `MaxItems` is 100). The call is made once, with no `Marker` loop, and `DistributionList.IsTruncated` is never read.
- **Impact:** the panel shows an exact-looking count (`relatedResult`, not truncated) that leaves out the remaining distributions.
- **Fix:** page with `Marker`/`NextMarker`, or return `relatedResultTrunc("cf", ids, aws.ToBool(out.DistributionList.IsTruncated))`.

## cfn

# cfn — production-code review

Scope: `core/aws/cfn.go`, `cfn_codes.go`, `cfn_interfaces.go`, `cfn_issue_enrichment.go`, `cfn_detail_enrichment.go`, `cfn_related.go`, `cfn_events.go`, `cfn_resources.go` (+ `_codes`), the `cfn` entry in `core/aws/catalog_cicd.go` (incl. `colorCFN`), the `cfn_events` / `cfn_resources` child entries in `core/aws/catalog_backup.go`, and the shared Wave 2 helpers in `core/aws/issue_enrichment.go`.

## 1. P2 — The combined Wave 2 enricher discards every supporting row

- **File/line:** `core/aws/cfn_issue_enrichment.go:159-164` (`EnrichCFNCombined` return literal)
- **Trigger:** Any stack for which `EnrichCFNStackEvents` raises `cfn.recent-resource-failure` (rows naming `AWS::X::Y/LogicalId` + `Reason`) or `EnrichCFNDrift` raises `cfn.stack-drifted` (row `Drift Status`). Both sub-enrichers write those rows into `result.AttentionDetails` through `setWave2Finding` (`issue_enrichment.go:173-183`), but the combined result only carries `Truncated`, `TruncatedIDs`, `Findings`, `FieldUpdates`. `AttentionDetails` is never merged. `EnrichCFNCombined` is the registered `Wave2.Fn` (`catalog_cicd.go:90`).
- **User impact:** The Attention section shows "recent resource failure" with no indication of which resource failed or why. That resource and reason are the whole diagnostic value of the signal.
- **Fix direction:** Merge `AttentionDetails` from both sub-results per (id, code), for example by folding both results through `mergeRowResult`, which already unions Findings, TruncatedIDs, FieldUpdates and AttentionDetails correctly.

## 2. P2 — A drift finding is dropped when the same stack also has a recent resource failure

- **File/line:** `core/aws/cfn_issue_enrichment.go:140-141`
- **Trigger:** A stack that is `DRIFTED` and also has a `*_FAILED` event on its first events page. `maps.Copy(merged, eventsResult.Findings)` replaces the whole `[]Finding` slice for that ID, so `cfn.stack-drifted` is overwritten, not appended.
- **User impact:** The "stack drifted from template" warning disappears exactly on stacks that are already troubled. The spec (§3.2/§4) defines the two signals as independent rows, and `setWave2Finding`'s own contract is "every independently-evaluated condition survives as its own Finding".
- **Fix direction:** Append per ID with code de-duplication instead of `maps.Copy` (same fix as #1: `mergeRowResult`).

## 3. P2 — "recent resource failure" is raised on healthy stacks from historical events

- **File/line:** `core/aws/cfn_issue_enrichment.go:73-104`
- **Trigger:** A stack whose update failed and rolled back weeks ago and has since had one or more successful updates (current status `UPDATE_COMPLETE`). `DescribeStackEvents` returns the stack's full event history newest-first, and the loop flags any `*_FAILED` event anywhere on the first page. It does not stop at the boundary of the most recent stack operation (the latest stack-level event with `ResourceType == AWS::CloudFormation::Stack` and a terminal status). A small stack emits only a handful of events per operation, so an old failure stays on page 1 for many later operations.
- **User impact:** A false Broken (`!`) finding and a red row on a healthy stack. The detail sentence also claims "what is deployed is not what the template describes", which is false for that stack.
- **Fix direction:** Scan events only back to the start of the latest stack operation (stop at the first stack-level `*_IN_PROGRESS` that starts the operation, or at the previous stack-level terminal event). Alternatively, only consider events newer than `LastUpdatedTime`/`CreationTime` of the listed stack.

## 4. P2 — Credential-in-outputs and termination-protection checks are skipped on live stacks whose status ends in `_FAILED`

- **File/line:** `core/aws/cfn.go:137-138` (`cfnStackIsTearingDown`), used at `cfn.go:161`
- **Trigger:** A stack in `UPDATE_ROLLBACK_FAILED`, `UPDATE_FAILED` (update with rollback disabled) or `IMPORT_ROLLBACK_FAILED`. These are live stacks that own deployed resources and still expose their `Outputs`. The `HasSuffix(status, "_FAILED")` test classes them as "never built / on its way out", so `cfnPostureOf` returns empty words and neither posture finding is evaluated.
- **User impact:** A credential sitting in the outputs of a long-lived stack that is stuck in `UPDATE_ROLLBACK_FAILED` is never reported (Broken signal silently suppressed). The missing termination protection on that live stack is not reported either.
- **Fix direction:** Replace the suffix test with an explicit set of statuses that really have nothing to fix: `DELETE_*`, `CREATE_FAILED`, `ROLLBACK_*` (failed create). Keep `UPDATE_*_FAILED` / `IMPORT_ROLLBACK_FAILED` evaluated. At minimum, never suppress the output-secret scan for a stack whose outputs are readable.

## 5. P3 — Termination-protection warning is raised on a `ROLLBACK_COMPLETE` tombstone

- **File/line:** `core/aws/cfn.go:137-138`
- **Trigger:** A stack in `ROLLBACK_COMPLETE` (its create failed). The status has no `DELETE_` prefix and no `_FAILED` suffix, so `cfnPostureOf` evaluates it and emits `cfn.termination-protection-off`. The function's documented intent is to exempt "one that never built", and `ROLLBACK_COMPLETE` is exactly that case.
- **User impact:** The operator is told to turn on termination protection for a stack whose only valid next action is deletion. This contradicts the stack's own rollback finding and adds a spurious `(+1)` to the row phrase.
- **Fix direction:** Include `ROLLBACK_COMPLETE` (and `ROLLBACK_IN_PROGRESS`) in the exempt set, as part of the explicit set from #4.

## 6. P2 — Drift is fetched with an extra call per stack and capped at 50, although the list response already carries it

- **File/line:** `core/aws/cfn_issue_enrichment.go:182` (cap) and `:197-201` (per-stack `DescribeStacks`); `core/aws/cfn.go:30-79` (list fetcher never reads `stack.DriftInformation`)
- **Trigger:** An account with more than 50 stacks (`EnrichmentCap`). `DescribeStacks` without `StackName` already returns `Stack.DriftInformation` for every stack (same `cfntypes.Stack` struct), but `EnrichCFNDrift` calls `DescribeStacks` again per stack for the first 50 only. It marks every stack past 50 as uninspected (`CheckCap`) and sets `Truncated`.
- **User impact:** For stacks 51 and later, the Drift column stays blank, a `DRIFTED` stack never shows "stack drifted from template", the row renders "?", and the cfn issue count is shown as a lower bound. All of this happens even though the fact was already in memory. It also costs up to 50 redundant API calls per refresh, which compete with the events enricher for throttle budget.
- **Fix direction:** Derive `drift_status` and the drift finding from `stack.DriftInformation` of the list row's `RawStruct` inside `EnrichCFNDrift`, with no API call and no cap. Alternatively, produce it in the Wave 1 fetcher and keep `IssueEnricherFieldKeys` consistent.

## codeartifact

# codeartifact — production-code review

Findings: 4 (P0:0 P1:0 P2:2 P3:2)

## 1. P2 — The row ID is the bare repository name, but repository names are only unique within a domain

- **File/line**: `core/aws/codeartifact.go:68` (`ID: repoName`); the ID is reused as the enrichment key at `core/aws/codeartifact_issue_enrichment.go:70`
- **Trigger**: the account has two domains that each hold a repository with the same name (for example `npm-store` in `dev-domain` and in `prod-domain`). CodeArtifact only requires a repository name to be unique inside its domain, and `ListRepositories` returns repositories from every domain.
- **Impact**: everything downstream keys on `r.ID`:
  - Wave-2 findings: `setWave2Finding` writes into `r.Findings[resourceID]` (`core/aws/issue_enrichment.go:169-170`), and those findings are applied per `r.ID` (`core/runtime/helpers.go:135`). The two findings are also merged. If one repository has a public policy and the other has none, both rows show both findings. A clean repository can show red `public access policy`.
  - Package counts: `FieldUpdates[key]` is overwritten by whichever goroutine finishes last. Both rows then show the same Packages value (`core/runtime/handlers_availability.go:770`).
  - `MarkSkipped`/`TruncatedIDs` for one repository mark the other one too.
  - Related navigation to `codeartifact` by ID matches both rows (`core/runtime/handlers_related.go:250,425`). Lazy/page merges and the filtered list drop the second row as a duplicate (`core/runtime/handlers_resources.go:489`, `core/app/list_state.go:295`).
  - `CloudTrailKey: "ResourceName:ID"` (`core/aws/catalog_cicd.go:297`) also mixes the two repositories' events together.
- **Fix direction**: build a composite ID that is unique across domains, such as the repository ARN or `domainOwner/domain/repo`. Keep the repository name as `Name` / `repo_name`. Change the console URL (`catalog_cicd.go:310`) and any inbound pivots to use `repo_name` or the composite key.

## 2. P2 — secrets → codeartifact pivot returns secret names and tag values as CodeArtifact IDs

- **File/line**: `core/aws/secrets_related_extra.go:41`, `:58`, `:62` (registered at `core/aws/catalog_secrets.go:80`)
- **Trigger**: a secret whose name contains `codeartifact` (for example `ci/codeartifact-token`), or a secret with a tag whose key or value contains `codeartifact`.
- **Impact**: the related panel on the secret shows `CodeArtifact Domains (1)`. The ID it passes is the secret's own name, a tag value, or even a tag key. None of these is a codeartifact row ID, which is the repository name (`codeartifact.go:68`). Drilling in filters the codeartifact list by `r.ID` (`core/runtime/handlers_related.go:250,425`) and finds nothing, so the user gets an empty list behind a non-zero count. The pivot's label also says "Domains", but the target type lists repositories.
- **Fix direction**: the name heuristic cannot produce a repository ID. Either return `UnknownRelated` (blank, drill in) instead of IDs, or parse a real `arn:aws:codeartifact:…:repository/<domain>/<repo>` out of tag values and return the repository ID in the codeartifact type's ID format. Rename the pivot to "CodeArtifact Repos".

## 3. P3 — pipeline → codeartifact pivot matches an action provider that CodePipeline does not have

- **File/line**: `core/aws/pipeline_related.go:178`, result at `:185`
- **Trigger**: open the detail view of any pipeline.
- **Impact**: the CodePipeline action-provider list has no `CodeArtifact` provider in any category (Source/Build/Test/Deploy/Approval/Invoke). See <https://docs.aws.amazon.com/codepipeline/latest/userguide/reference-action-types.html>. Pipelines reach CodeArtifact through CodeBuild. So the checker always returns `KnownRelated(nil)`, and the panel shows a dimmed, non-actionable proven `(0)` for a relationship it never checked. This is a false "no dependency" answer.
- **Fix direction**: drop the pivot, or return `UnknownRelated("codeartifact")` so the row is blank and drillable instead of a proven zero. Update the contract row in `docs/related-resources.md` to match.

## 4. P3 — `ListPackages` failures are swallowed with no failure signal

- **File/line**: `core/aws/codeartifact_issue_enrichment.go:93-95`
- **Trigger**: `ListPackages` fails for a repository, for example `AccessDeniedException` because the read-only role lacks `codeartifact:ListPackages`, or throttling.
- **Impact**: `total = -1` skips the field update. The error is not added to `failures`, `AggregateFailures` never reports it, and no row is marked as unchecked. The Packages column stays blank with no error flash or log entry. The operator cannot tell "denied/failed" from "not loaded yet". Every other failing call in this enricher goes through `MarkSkipped`/`failures` (`:125-126`, `:134-135`).
- **Fix direction**: append the error to `failures` (under the mutex) so `AggregateFailures` surfaces it. The partial-error channel already covers this. Don't mark the row skipped, because the permissions-policy verdict for it is still valid.

## ct-events

# ct-events — production-code review

Scope: `core/aws/ct_events*.go`, `core/aws/catalog_monitoring.go` (ct-events entry, `colorCTEvents`), `core/semantics/ctevent/*`, the related-navigation path (`core/runtime/handlers_related.go`, `core/runtime/fetchers.go`), and `resource.BuildCloudTrailFilter`.
AWS semantics checked against the CloudTrail userIdentity and record-contents reference pages (docs.aws.amazon.com, fetched 2026-09-18).

8 findings: P0:0 P1:0 P2:3 P3:5

---

## 1. [P2] Cross-account role identities resolve to a same-named role in the local account

- **Where**:
  - `core/aws/ct_events_related.go:61-98`: every candidate source (`requestParameters.roleArn` at :71, `Resources[]` at :80, `sessionIssuer.userName` at :93) goes through `ctRoleAlternatives`/`ctIDAlternatives`. These put the bare role name (`"X"`) ahead of the full ARN, and `extractRoleNameFromCTEventJSON` has only the name, with no account.
  - `core/aws/ct_events_related.go:152-183`: `ctEventsMatchTarget` matches on `r.Name`, and the first candidate that matches wins.
  - `core/semantics/ctevent/sections.go:113-121,175-191`: the ACTOR `Principal` row's `NavID` (from `arnNavID`) drops the account.
  - `core/semantics/ctevent/target.go:108-119`: the TARGET `Role` row's `NavID` (from `roleNavID`) also drops the account.
- **Trigger**: an event recorded in account A where the caller is an AssumedRole from account B, for example B's `OrganizationAccountAccessRole`, `AWSReservedSSO_*` or a CI role name that also exists in A. The same happens for a Resources/`roleArn` entry that names a role ARN from another account.
- **Impact**: the "IAM Roles" related count and the Principal/Role drill both land on account A's own role with that name. The event itself is flagged `cross-account access` as a security signal, yet a forensics pivot on it sends the operator to the wrong principal. The spec (§2 `role`) says to cross-reference by ARN.
- **Fix direction**: carry the account through. Build candidates from `sessionIssuer.arn` (or the full ARN) and match on the role row's ARN. When the ARN's account is not the recipient account, return no local match (Known 0 or non-navigable). Apply the same account check to `arnNavID`/`roleNavID` navigation.

## 2. [P2] The "CT events by AccessKeyId" pivot is suppressed for root-user events

- **Where**: `core/aws/ct_events_related.go:622-625`
- **Trigger**: any event with `userIdentity.type == "Root"` that carries `userIdentity.accessKeyId`. Per AWS, `accessKeyId` is "the access key ID that was used to sign the request". That covers root access keys and root console temporary credentials.
- **Impact**: the pivot returns Known 0 based on the false premise "Root has no access key". This is exactly the key-compromise question the facet exists for (spec §2 `ct-events` — "By AccessKeyId … key-compromise forensics"), asked about the highest-privilege principal, and the facet disappears.
- **Fix direction**: remove the Root short-circuit and gate only on a non-empty `accessKeyId`.

## 3. [P2] The detail ACTOR section shows an AWS service as the actor for identities that have no ARN

- **Where**: `core/semantics/ctevent/sections.go:93-106` (`isServiceEvent := … || ui.ARN == ""`)
- **Trigger**: `userIdentity.type` of `AWSAccount` or `IdentityCenterUser`. Neither carries `arn`, per the AWS userIdentity reference. `AWSAccount` is how another account's `AssumeRole` into a role you own is recorded.
- **Impact**: ACTOR renders `Service: sts.amazonaws.com` (the event source) instead of the calling account or principal. The Access key and User agent rows are dropped too. The detail view tells the operator an AWS service made a cross-account call that another account made.
- **Fix direction**: decide "service" only from `EventType == "AwsServiceEvent"` / `ui.Type == "AWSService"`. For ARN-less human or account identities, emit a Principal row from `type` + `accountId`/`principalId` (or `onBehalfOf.userId`) and keep the other ACTOR rows.

## 4. [P3] The list TARGET column is picked by Go map iteration order, so it changes between renders and can disagree with the detail TARGET

- **Where**: `core/aws/ct_events_target_local.go:109-117`. The catch-all returns the first `*Id`/`*Name`/`*Arn` key found by `range req`. The detail counterpart, `core/semantics/ctevent/target.go:380-410` (`catchAllScan`), ranks by suffix and breaks ties by key.
- **Trigger**: a management event with no `resources[]` and several matching request keys, for example `PutRolePolicy` (`roleName`, `policyName`), `CreateLogStream` (`logGroupName`, `logStreamName`) or `AttachRolePolicy` (`roleName`, `policyArn`).
- **Impact**: `_ct.target` (TARGET column, sort key, `/` filter text) can show a different value on each fetch or refresh for the same event, and a different value from the detail's TARGET row. The same fact is computed twice with two different rules.
- **Fix direction**: have the list use the detail's deterministic `ctevent.ExtractTarget`/`catchAllScan` (one extraction table) instead of the duplicate `extractTargetByEventName`.

## 5. [P3] The console-origin override reads `sessionCredentialFromConsole` from the wrong JSON path

- **Where**: `core/aws/ct_events.go:440-455`. The code looks under `userIdentity.sessionContext.sessionCredentialFromConsole`.
- **Trigger**: any console-originated API call. AWS records `sessionCredentialFromConsole` as a top-level record field (record-contents reference, "Since: 1.08").
- **Impact**: the override never fires. Console calls whose `userAgent` doesn't contain "console" (a browser UA, `AWS Internal`, …) show ORIGIN `Browser` or `SDK` instead of `Console`.
- **Fix direction**: read `parsed["sessionCredentialFromConsole"]` at the top level (string `"true"`).

## 6. [P3] The ACTOR section shows an empty `Federation` row on ordinary assumed-role events

- **Where**: `core/semantics/ctevent/sections.go:129-132`
- **Trigger**: an AssumedRole event with `"webIdFederationData": {}`. This is the common shape (see the AWS sessionContext example). AWS states that an empty value "signifies that there is no information about the identity provider."
- **Impact**: most assumed-role events get a blank-valued `Federation:` line in the detail view. That is noise on the actor block and suggests web-identity federation that did not happen.
- **Fix direction**: emit the row only when `FederatedProvider != ""`.

## 7. [P3] The Related "IAM Roles" entry drops the calling role whenever the event also names a target role

- **Where**: `core/aws/ct_events_related.go:61-98`. `ctEventsRoleCandidates` returns only the first source (`roleArn`, then `Resources[]` with a Role type, then Username, then `sessionIssuer`) as a single candidate group.
- **Trigger**: an AssumedRole caller (role A) calls `AssumeRole` on role B, or `UpdateAssumeRolePolicy`/`AttachRolePolicy` on B.
- **Impact**: the panel shows at most one role, B, so the caller A is unreachable from the panel. Spec §2 `role` defines this entry as the caller's role ("the assumed-role identity that made the API call", from `sessionIssuer.arn`). "Who did this" is lost on the event type most used for role-chaining forensics.
- **Fix direction**: return the caller (`sessionIssuer.arn`) as its own candidate group, plus the target role as a separate group, so both can match.

## 8. [P3] The "CT events by SharedEventId" drill can only ever surface the event itself, and only if it is in the newest loaded page

- **Where**:
  - `core/aws/ct_events_related.go:661-685`: a local-field filter on `shared_event_id`, or a fallback `EventId` filter that returns the source event itself.
  - `core/aws/ct_events.go:89-131`: the local filter is applied to an unfiltered 50-event LookupEvents page.
- **Trigger**: open a cross-account event and drill "CT events by SharedEventId". Per AWS, the other events that share a `sharedEventID` are delivered to other accounts, so this account's LookupEvents has at most one match: the current event.
- **Impact**: if the event is older than the newest 50 account events, the drill opens empty and the operator has to page ("m") through the account's history. At best it shows the row the operator came from. The facet is offered as a navigable pivot but never answers its question (spec §2: "group events that share a cross-service request").
- **Fix direction**: offer the facet only where it can find something. For example, show the shared id as a detail value, or scope the lookup server-side (`EventName` + `EventSource` for the same call) before the local match. Stop presenting an EventId self-lookup as the fallback pivot.

## dbc-snap

# dbc-snap production-code review

Scope: `core/aws/dbc_snap.go`, `core/aws/dbc_snap_rds.go`, `core/aws/dbc_snap_related.go`,
`core/aws/dbc_snap_issue_enrichment.go`, `core/aws/dbc_snap_codes.go`, `core/aws/snapshot_cross_ref.go`,
the `dbc-snap` catalog entry in `core/aws/catalog_databases.go`, `dedupResourcesByID` in `core/aws/dbc.go`,
the `dbc-snap` view defaults in `core/config/defaults_databases.go`, and the list-fetch / row-store paths
the fetcher's pages flow through (`core/runtime/executor.go`, `core/session/rowstore.go`, `core/app/list_body.go`).

## Findings

### 1. P2: A copied snapshot is flagged "orphan: source cluster deleted" while its source cluster still exists

- **File/line:** `core/aws/snapshot_cross_ref.go:172-184` (orphan rule), fed by `core/aws/dbc_snap_issue_enrichment.go:114-128` (`dbcSnapParentID`)
- **Trigger:** A cluster snapshot copied from another region, or from another account's shared snapshot (a common DR pattern). AWS documents `DBClusterIdentifier` as "the DB cluster identifier of the DB cluster that this DB cluster snapshot was created from", so a copy keeps the source cluster's name. `SourceDBClusterSnapshotArn` is set on copies. The parent-ID extractor returns `DBClusterIdentifier` and never checks `SourceDBClusterSnapshotArn`. The orphan rule then looks up that name in the current region's `dbc` list, doesn't find it, and emits `dbc-snap.orphan`.
- **User impact:** Every cross-region or cross-account DR copy shows as a red Broken row saying "orphan: source cluster deleted". The detail text tells the operator the cluster is gone and suggests deleting the snapshot, even though the cluster is alive in its own region or account. This is a false Broken finding, and it points the operator toward deleting DR copies.
- **Fix direction:** In `dbcSnapParentID`, return `("", false)` when `SourceDBClusterSnapshotArn` is non-empty and its region or account differs from the current one. At minimum, skip the orphan rule for copied snapshots. Do the same for the `dbi-snap` extractor (`SourceRegion`/`SourceDBSnapshotIdentifier`), which shares the helper.

### 2. P2: Duplicate snapshot rows after a refresh once the DocDB side spans more than one page

- **File/line:** `core/aws/catalog_databases.go:742` (dedup covers only the last DocDB page plus RDS page 1). The duplicates reach the screen through `core/runtime/executor.go:425-445` (the refetch loop appends pages with no ID dedup) and `core/session/rowstore.go:292-293` (the replace path stores rows as given).
- **Trigger:** The spec (§1) and the `dedupResourcesByID` comment both say the DocDB and RDS `DescribeDBClusterSnapshots` endpoints each return snapshots for both engine families. With more than 50 (`DefaultPageSize`) snapshots, DocDB page 1 is returned on its own (line 726-728). Later, the last DocDB page is merged with RDS page 1 and deduped only against each other (line 742). RDS page 1 repeats the rows of DocDB page 1, so they come back again, and so do all later `rds:` pages. On first load, the append path deduplicates against existing rows (`list_body.go:149`, `rowstore.go:291`), so no duplicates show, but the RDS pages add mostly nothing. On a refetch, `executor.go` pages up to `CachedListDepth` (the on-disk population from the last session) with a plain `append` and then stores the result as a replace (`appendPage=false`). No dedup runs on that path.
- **User impact:** After refresh or reopen in an account with more than 50 cluster snapshots, the same snapshot appears two times in the list. Counts, issue badges and the related-panel reverse lookup from `dbc` then count those snapshots twice. The `CachedListDepth` bound also counts duplicates, so the refetch stops early with fewer unique rows than were cached. During "load more", the user can page through whole RDS pages that add no new rows.
- **Fix direction:** Remove the duplicate truth at the source. Keep the fetcher from emitting a row the other endpoint already emitted (for example, filter each side by engine family: `docdb` on the DocDB call, everything else on the RDS call). The alternative is to dedup by ID in the executor's accumulation loop, since that path is the one that skips the store's append dedup. `dbc` (`catalog_databases.go:330`) has the same shape and needs the same fix.

### 3. P3: The console link for a DocumentDB cluster snapshot opens the RDS console

- **File/line:** `core/aws/catalog_databases.go:697-699`
- **Trigger:** Opening the console link on any `dbc-snap` row whose `engine` is `docdb`. The URL is always `rds/home?...#db-snapshot:id=<id>`, whatever the engine. The sibling `dbc` entry (`catalog_databases.go:276-288`) switches on `Fields["engine"]` and sends `docdb*` to `docdb/home?...` because DocumentDB resources are managed in the DocumentDB console.
- **User impact:** For DocumentDB snapshots, "open in console" goes to the RDS console, where the snapshot isn't shown, so the operator has to find it by hand in the DocumentDB console.
- **Fix direction:** Branch on `r.Fields["engine"]` the same way `dbc` does. Send `docdb*` to the DocumentDB console's snapshots page and keep the RDS URL for Aurora and Multi-AZ rows. The RDS hash fragment for cluster snapshots (as opposed to instance snapshots, `#db-snapshot:`) was not verified against live console URLs in this review. Check it when making the change.

### 4. P3: The Backup pivot lists every plan that covers the parent cluster, not whether AWS Backup made this snapshot

- **File/line:** `core/aws/dbc_snap_related.go:113-164` (`checkDbcSnapBackup`)
- **Trigger:** Spec §2 `backup` defines the pivot as "whether a snapshot was created by Backup". Discovery is a prefix match of `awsbackup:job-` on `DBClusterSnapshotIdentifier`, and the count is 0 or 1. The code ignores the snapshot identifier. It resolves the parent cluster ARN from the `dbc` cache and returns every backup plan whose selection covers that cluster.
- **User impact:** Manual and automated snapshots of a Backup-covered cluster show "Backup Plans N", which implies AWS Backup governs their lifecycle when it doesn't. The count can be above 1, which the spec rules out. Orphaned snapshots, whose parent is gone, always show 0, even when AWS Backup did create them.
- **Fix direction:** Match the spec. Relate the snapshot to Backup only when its identifier has the `awsbackup:job-` prefix, with a count of 0 or 1. If plan-level drill-through is still wanted, keep plan resolution behind that gate. Otherwise, update the spec deliberately, but don't leave code and spec disagreeing.

### 5. P3: The DocumentDB-side snapshot list call doesn't retry on throttling

- **File/line:** `core/aws/dbc_snap.go:63`
- **Trigger:** `DescribeDBClusterSnapshots` gets throttled (for example, during a startup probe burst across many types). The RDS-side call (`dbc_snap_rds.go:69-71`) and every sibling list call (`dbc.go:27`, `dbc_rds.go:119`, `dbi_snap.go:76`) go through `RetryOnThrottle`. This call doesn't. A DocDB-side error returns straight from the fetcher (`catalog_databases.go:722-725`), and the RDS side is never tried.
- **User impact:** One throttling error blanks the whole `dbc-snap` list, Aurora snapshots included, and shows an error. The same burst leaves the other database lists intact.
- **Fix direction:** Wrap the call in `RetryOnThrottle(ctx, DefaultRetryConfig(), ...)`, the same way the RDS side does.

## dbc

# dbc — production-code review

Scope: `core/aws/dbc.go`, `core/aws/dbc_rds.go`, `core/aws/dbc_related.go`, `core/aws/dbc_issue_enrichment.go`, `core/aws/dbc_codes.go`, `core/aws/dbc_interfaces.go`, `core/aws/rds_posture.go`, `core/aws/ct_events_pivot.go`, the `dbc` entry in `core/aws/catalog_databases.go`, `core/config/defaults_databases.go` (dbc), inbound pivots `checkDbiDBC` / `checkDbcSnapDBC`, and `core/session/rowstore.go` (page append dedup).

## 1. P1 — Terminal failure statuses are shown as a benign "in progress" warning

- **File/line**: `core/aws/dbc.go:171-175` and `core/aws/dbc.go:220-222`; `core/aws/dbc_rds.go:55-59` and `core/aws/dbc_rds.go:97-99`.
- **Trigger**: the cluster `Status` is one of the documented terminal or failed cluster states that are missing from the Broken map: `cloning-failed`, `migration-failed`, `upgrade-failed`, `inaccessible-encryption-credentials-recoverable`. `stopped` is affected too. (Source: Aurora User Guide, "Viewing DB cluster status".) None of these is in `brokenCode` or in the transitional set, so they reach the "unknown status" fallback. That fallback emits `CodeDBCTransitional`.
- **User impact**: a failed cluster shows `upgrade-failed: in progress` (or `migration-failed: in progress`, and so on) with Warning `~` severity. Its detail text says the cluster "is mid-operation … Wait for it to return to available". AWS says such a cluster will not recover on its own: `upgrade-failed` is not billed and gets a final snapshot. The row is never counted as Broken, so the main-menu `!` badge misses it. `inaccessible-encryption-credentials-recoverable` needs operator action (a restart after the KMS key is fixed) but is shown as a transition. `stopped` is shown as "in progress" even though nothing is running.
- **Fix direction**: map `cloning-failed`, `migration-failed` and `upgrade-failed` to a Broken code, and map `inaccessible-encryption-credentials-recoverable` to `CodeDBCEncryptionKeyUnreachable`. Give `stopped` a non-"in progress" phrase. Keep the transitional fallback only for states that really are transitional. Merge the two status tables (`transitionalDBCStatusSet` / `transitionalRDSDBCStatusSet` and the two `brokenCode` maps) into one table so the lanes cannot drift apart.

## 2. P2 — "no writer: reads only" (Broken) fires on healthy global-database secondary and cross-Region replica clusters

- **File/line**: `core/aws/dbc.go:182-185`; `core/aws/dbc_rds.go:66-69`.
- **Trigger**: an `available` cluster that is a secondary in an Aurora or DocumentDB global cluster (`GlobalClusterIdentifier` set), or a cross-Region read-replica cluster (`ReplicationSourceIdentifier` set). By design its members are all readers (`IsClusterWriter == false`), and a headless secondary has no members at all. No production code reads `GlobalClusterIdentifier` or `ReplicationSourceIdentifier` (grep over `core/aws` returns nothing).
- **User impact**: a correctly configured DR secondary is painted Broken `!` with "every write fails … promote a reader or add an instance". Following that advice on a global secondary breaks replication. The false Broken also inflates the menu badge.
- **Fix direction**: skip the no-writer predicate when `ReplicationSourceIdentifier` is non-empty (both SDK shapes carry it) or when `GlobalClusterIdentifier` is set (RDS shape). Also pass `IsReadReplica` into `rdsPosture` so single-AZ is not reported for these clusters either.

## 3. P2 — Aurora clusters keep the DocumentDB-shaped row, so Aurora-only findings never fire and Neptune clusters leak into the list

- **File/line**: `core/aws/dbc.go:20-22` (unfiltered DocDB `DescribeDBClusters`); `core/aws/catalog_databases.go:330` (dedup keeps the DocDB row first); `core/aws/dbc.go:162-168` (DocDB posture passes nil `AutoMinorVersionUpgrade` / `IAMAuthEnabled`). `core/session/rowstore.go:291` has the same first-wins rule for rows that arrive on later pages.
- **Trigger**: AWS documents that DocDB `DescribeDBClusters` returns RDS and Neptune clusters unless it is called with `filterName=engine,Values=docdb`. The spec (§1) records that this was verified live for `aurora-postgresql`. The DocDB fetcher sends no filter. Each Aurora cluster is therefore built first as a `docdbtypes.DBCluster`, and `dedupResourcesByID` (or the RowStore append dedup, across pages) discards the RDS-shaped row. `docdbtypes.DBCluster` has no `AutoMinorVersionUpgrade` or `IAMDatabaseAuthenticationEnabled` field (verified in docdb@v1.56.0 `types.DBCluster`). Neptune clusters from the DocDB endpoint are kept, although `dbc_rds.go:135` explicitly drops Neptune as "not supported as dbc".
- **User impact**: an Aurora cluster with auto minor version upgrade off, or IAM DB auth off, never shows `auto minor version upgrade off` / `IAM database authentication off`. Two of the spec §4 signals are dead for exactly the engine they exist for. The RDS-shaped detail (and any RDS-only field) is also lost for Aurora rows. Neptune clusters appear in the DB Clusters list, and whether they are included depends on which endpoint returned them.
- **Fix direction**: add `Filters: [{Name: "engine", Values: ["docdb"]}]` to the DocDB call, as the SDK and API docs instruct. This leaves the RDS lane authoritative for Aurora and Multi-AZ clusters and drops Neptune on both lanes. The dedup can stay as a safety net.

## 4. P3 — The RDS lane reports "`: in progress`" for a cluster with no status; the DocDB lane was fixed and this lane was not

- **File/line**: `core/aws/dbc_rds.go:97-99` (compare `core/aws/dbc.go:213-218`).
- **Trigger**: the RDS `DescribeDBClusters` row has a nil or empty `Status`.
- **User impact**: the Aurora row shows the Warning phrase ": in progress" with an empty keyword and claims a transition nobody reported. The same input on the DocDB lane shows no status finding. The two copies of the algorithm disagree.
- **Fix direction**: factor one shared `dbcStatusFindings(status, writers, deletionProtection, storageEncrypted, backupRetention, posture)` that both lanes call, so the empty-status guard and the status tables (finding 1) exist once.

## 5. P3 — The CloudTrail hotkey and the ct-events related pivot filter the same cluster by different values

- **File/line**: `core/aws/catalog_databases.go:274` (`CloudTrailKey: "ResourceName:Fields.arn"`) vs `core/aws/dbc_related.go:421-429` (the pivot uses `DBClusterIdentifier` for both the count and the drill-in `FetchFilter["ResourceName"]`).
- **Trigger**: open CloudTrail events for a dbc row from the list hotkey, then compare with the detail panel's CloudTrail Events pivot and its drill-in.
- **User impact**: LookupEvents `ResourceName` matches one exact string, so the two entry points query CloudTrail with different values (the cluster ARN versus the bare identifier). For the same cluster they return different event sets, and one of them misses the events CloudTrail indexed under the other form. I did not verify which form CloudTrail records for `AWS::RDS::DBCluster`. The inconsistency itself is certain from the code.
- **Fix direction**: derive both from one source. Either set `CloudTrailKey` to the identifier (`ResourceName:ID`, which is what most types use), or make the pivot's `IDExtractor` return `Fields["arn"]`. Choose after confirming against a live `lookup-events` response for a cluster change.

## dbi-snap

# dbi-snap — production code review

Scope: `core/aws/dbi_snap.go`, `dbi_snap_codes.go`, `dbi_snap_related.go`, `dbi_snap_issue_enrichment.go`, `snapshot_cross_ref.go`, the `dbi-snap` catalog entry in `core/aws/catalog_databases.go`, `core/config/defaults_databases.go` (the dbi-snap view), `checkDbiDBISnap` in `core/aws/dbi_related.go`, and the dependencies they call: `backup_match.go`, `backup.go`, `backup_coverage.go`, `ct_events_pivot.go`, `issue_enrichment.go`, and `core/runtime/executor.go` (`RunRelatedDef`).

## Findings

### 1. P2: The Backup Plans pivot ignores tag-based selections and partially read plans, then reports a definite 0

- **File:** `core/aws/dbi_snap_related.go:154`
- **Code:** `if BackupPlanCoversARN(planRes.Fields["resources"], planRes.Fields["not_resources"], parentARN) {`
- **Trigger:** the snapshot's source DB instance is covered by an AWS Backup plan that selects resources by tag (`ListOfTags` or `Conditions`). This is the most common way to set up AWS Backup selections. The same happens when the backup fetcher could not read all of a plan's selections, which it marks with `Fields["selections_partial"]` (`core/aws/backup.go:90-95`, `:126`).
- **Why it happens:** `BackupPlanCoversARN` (`core/aws/backup_match.go:17-41`) looks only at the `resources` and `not_resources` ARN patterns. The codebase already has a coverage join that handles both cases: `backupPlanCovers` checks `selection_tags` against the resource's tags (`core/aws/backup_coverage.go:164-172`), and `backupPlansIncomplete` refuses to answer "not covered" when a plan is partial (`core/aws/backup_coverage.go:125-133`). The dbi-snap checker uses neither, so the same fact is computed two ways and the two can disagree.
- **User impact:** the detail panel shows "Backup Plans: 0", a confirmed zero, for a snapshot whose parent instance is protected by AWS Backup. An operator deciding whether this manual snapshot is the only copy is told there is no backup coverage when there is.
- **Fix direction:** send the check through the shared `backupPlanCovers`/`backupPlansCover` join. Pass the parent `DBInstance.DBInstanceArn` plus its `TagList` as a `k=v` map, and return `UnknownRelated("backup")` when `backupPlansIncomplete(planList)` is true. Better still, delete `BackupPlanCoversARN` so that only one coverage implementation remains.

### 2. P2: Cross-Region or cross-account copies, and snapshots of renamed instances, are flagged Broken as "orphan: source DB deleted"

- **Files:** `core/aws/dbi_snap_issue_enrichment.go:44-50` (`GetParentID` returns `DBInstanceIdentifier` only); `core/aws/snapshot_cross_ref.go:156` (parents are looked up by name only) and `:172-179` (the orphan branch).
- **Trigger:**
  - A snapshot copied into this Region or account with `CopyDBSnapshot`, which is the standard cross-Region or cross-account DR pattern. The AWS `DBSnapshot` reference says `DBInstanceIdentifier` is the instance the snapshot "was created from", and `SourceRegion` or `SourceDBSnapshotIdentifier` are set on copies. The source instance is alive in another Region or account, so it is never in this Region's `dbi` list.
  - An instance renamed with `ModifyDBInstance NewDBInstanceIdentifier`. Its older snapshots keep the old identifier. The stable key, `DbiResourceId`, is on both `DBSnapshot` and `DBInstance` but is never used.
- **User impact:** every DR copy, and every snapshot of a renamed instance, turns red with Broken severity and the text "orphan: source DB deleted". The finding's detail tells the operator to take ownership or delete the snapshot. This pushes a deletion decision onto what may be the only off-Region copy, and it inflates the Broken count on the main menu.
- **Fix direction:**
  - Skip the orphan rule (return no parent) when `SourceDBSnapshotIdentifier` is set, or when `SourceRegion` is set and is not the current Region.
  - Match parents by `DbiResourceId` rather than by name. This also stops the past-retention rule from comparing against an unrelated instance that was later created under the same name.

### 3. P3: The public-share check spends its 50-snapshot budget on automated and AWS Backup snapshots that cannot be shared

- **Files:** `core/aws/snapshot_cross_ref.go:249` (`capAtEnrichmentCap` over all rows) and `:261`; `core/aws/dbi_snap_issue_enrichment.go:81-89` (`dbiSnapShareAttributes` calls the API for any snapshot).
- **Trigger:** an account where `DescribeDBSnapshots` returns more than `EnrichmentCap` (50, `core/aws/issue_enrichment.go:70`) snapshots, most of them `automated`. Automated snapshots are the default, so a few instances with multi-week retention reach this easily. `DescribeDBSnapshotAttributes` is documented for manual DB snapshots only, and only manual snapshots can be made public.
- **User impact:** the 50 attribute reads are spent on snapshots that can never carry the "shared with all AWS accounts" finding. Manual snapshots past the cap are marked uninspected, so a publicly shared manual snapshot, the one data-exposure signal on this type, can go unreported while a lower-bound marker is shown.
- **Fix direction:** filter the rows to `SnapshotType == "manual"` before `capAtEnrichmentCap`. A per-config predicate such as `IsShareable(raw) bool` in `SnapshotCrossRefConfig` would do it, and dbc-snap has the same shape.

### 4. P3: The Backup Plans pivot answers a different question from the one the spec defines

- **File:** `core/aws/dbi_snap_related.go:100-158` (`checkDBISnapBackup`).
- **Trigger:** any automated (`rds:…`) or manual snapshot whose parent instance is covered by a Backup plan, or an AWS Backup-created snapshot (`awsbackup:job-…`) whose parent has since been deleted.
- **Why it happens:** `docs/resources/dbi-snap.md` §2 `backup` defines the pivot as "was this snapshot produced by AWS Backup": 0 or 1, found by the `DBSnapshotIdentifier` prefix `awsbackup:job-`. The code instead counts plans that cover the parent instance, whoever created the snapshot. The spec states that where code and spec disagree, the code is wrong.
- **User impact:**
  - A manual or automated snapshot of a Backup-covered instance shows "Backup Plans: 1", which suggests its lifecycle is governed by AWS Backup when it is not.
  - A Backup-created recovery point whose instance was deleted shows 0, or unknown, which hides the fact that Backup retention governs it.
- **Fix direction:** implement the spec's `awsbackup:` identifier rule. If a plan-level pivot is really wanted, update the spec and `docs/related-resources.md` first, then keep the code as it is but fix finding 1.

## dbi

# dbi — production-code review

Scope: `core/aws/rds.go`, `rds_posture.go`, `dbi_related.go`, `dbi_issue_enrichment.go`, `rds_events.go`, `backup_coverage.go` (dbi call site), `catalog_databases.go` (dbi + dbi_events entries), `core/resource/related.go` (CloudTrail filter), `core/app/list_body.go` / `core/resource/resource.go` (row dedup, for dbi_events).

AWS semantics were checked against the current AWS docs: DB instance status table (`accessing-monitoring.html`), `API_DBInstance`, and `API_CreateDBInstance`.

---

## 1. P1 — Documented broken DB instance statuses show as healthy rows

- **File/line**: `core/aws/rds.go:178-188` (`brokenMap`), `core/aws/rds.go:157-164` (`transitionalStatusSet`), fall-through at `core/aws/rds.go:224-228`
- **Trigger**: an instance whose `DBInstanceStatus` is one of these documented values: `inaccessible-encryption-credentials-recoverable` (the KMS key can't be reached and the DB is down), `insufficient-capacity` (the instance can't be created), `incompatible-create`, or `upgrade-failed`. Three documented in-progress statuses are also missing from the transitional set: `delete-precheck`, `storage-config-upgrade`, and `storage-initialization`.
- **User impact**: none of these statuses is in either map. They fall into the "unknown status" branch, which adds no lifecycle finding. `colorDBI` → `colorAnyFindingOrHealthy` then colours the row from posture findings only. An instance that is down because its KMS key can't be reached, or that failed to upgrade, gets a healthy or yellow row and adds nothing to the broken badge count. Only the raw status text in the Status column (rds.go:101-103) hints at the problem. The four configuration warnings are skipped as well, because they only run when the status is `available`.
- **Fix direction**: add `inaccessible-encryption-credentials-recoverable`, `insufficient-capacity`, `incompatible-create` and `upgrade-failed` to `brokenMap`, each with its own FindingDef in the catalog. Add `delete-precheck`, `storage-config-upgrade` and `storage-initialization` to `transitionalStatusSet`.

## 2. P2 — Cluster-member instances get instance-level findings for settings the cluster owns

- **File/line**: `core/aws/rds.go:219-221` (deletion protection), `core/aws/rds.go:244` (`SkipSingleAZ` looks only at the engine prefix), `core/aws/dbi_issue_enrichment.go:50` (backup coverage)
- **Trigger**: any Aurora instance, that is, any `DBInstance` with `DBClusterIdentifier` set. AWS `CreateDBInstance` says of `DeletionProtection`: "This setting doesn't apply to Amazon Aurora DB instances … DB instances in a DB cluster can be deleted even when deletion protection is enabled for the DB cluster." AWS Backup protects Aurora at the cluster level, so a plan selects the cluster ARN, never the instance ARN. `DescribeDBInstances` also returns cluster-member instances of other engines, such as DocumentDB and Neptune. Their `MultiAZ` is false and their engine does not start with "aurora", so they get the single-AZ finding too.
- **User impact**: every cluster-member instance whose `DeletionProtection` reads false gets `deletion protection off`, whose remedy ("Turn deletion protection on") cannot be carried out on the instance. `addBackupCoverage` matches the instance ARN, so every Aurora instance gets `not covered by a backup plan` even when a plan covers its cluster by ARN. Non-Aurora cluster members also get `single-AZ` with the remedy "Enable Multi-AZ", which does not exist for them. All of these inflate the badge and the issue counts.
- **Fix direction**: use one predicate, `db.DBClusterIdentifier != nil`, and skip the deletion-protection, single-AZ and backup-coverage findings for those rows. The cluster (`dbc`) already reports these settings. `SkipSingleAZ` should use this predicate, not the engine prefix.

## 3. P2 — Configuration warnings disappear whenever the status is not `available`

- **File/line**: `core/aws/rds.go:195-223`
- **Trigger**: a public, unencrypted instance with no backups enters a routine transitional status: `backing-up` (daily), `modifying`, or `storage-optimization`, which AWS documents as able to last "up to and even beyond 24 hours" while the instance stays available.
- **User impact**: the four checks (no automated backups, public endpoint, unencrypted storage, deletion protection off) run only in the `available` branch. The posture pack (single-AZ, IAM auth, minor-upgrade, default user, CA cert) is appended in the transitional and broken branches too, so the two groups disagree. The warnings come and go with the backup window, and the badge count and Attention section change from refresh to refresh with no configuration change. Spec §3.1 lists these four signals without any status condition. §4 only decides which S4 phrase wins, and `domain.StatusPhrase` already stacks extra findings as "(+N)".
- **Fix direction**: evaluate the four configuration checks once, next to `dbiPostureFindings`, and append them after the lifecycle lead finding in every branch except teardown. Keep precedence in the ordering, not in whether the findings are emitted.

## 4. P2 — The ENI pivot counts ENIs of every RDS instance that shares a security group

- **File/line**: `core/aws/dbi_related.go:264-284`
- **Trigger**: two or more RDS instances share a security group, which is common (a shared `db-sg` or the default VPC SG).
- **User impact**: the filter is `description=RDSNetworkInterface` AND `group-id ∈ {this instance's SGs}`. EC2 ORs the values of one filter together, so it returns every RDS-managed ENI attached to any of those SGs. The panel shows an inflated "Network Interfaces" count, and drilling in lists other databases' ENIs as this instance's ENIs.
- **Fix direction**: scope the ENIs to this instance. For example, resolve the instance's endpoint address(es) and match the ENI's private IP. At minimum, add a `vpc-id` filter, only claim ENIs attached to exactly this instance's SG set, and mark the result as not proven (unknown or truncated) instead of a known exact count.

## 5. P2 — Two CloudTrail filters for the same instance: the `t` key uses the ARN, the related panel uses the identifier

- **File/line**: `core/aws/catalog_databases.go:70` (`CloudTrailKey: "ResourceName:Fields.arn"`) and `core/aws/dbi_related.go:295` and `:309` (`FetchFilter{"ResourceName": dbID}`, exact `*rr.ResourceName == dbID`)
- **Trigger**: open the dbi detail and compare the related panel's CloudTrail Events pivot with the CloudTrail jump (`t` / `resource.BuildCloudTrailFilter`) for the same instance.
- **User impact**: the two paths send different `LookupEvents` `ResourceName` values: the full ARN and the bare identifier. The catalog key encodes the project's own claim that CloudTrail indexes RDS instances by ARN (CHANGELOG: "ARN-based CloudTrail filters for … RDS"). If that is true, the related-panel pivot counts and filters on a value CloudTrail never indexes, so it shows 0 events while `t` shows events. The reverse checker (`checkCtEventsRDS`, ct_events_related.go:441) accepts both forms through `ctIDAlternatives`, but this forward checker accepts only the exact identifier. Whichever form AWS actually uses, one of the two paths is wrong, because the same fact is computed two ways. I did not confirm from AWS documentation which form `Event.Resources[].ResourceName` carries for `AWS::RDS::DBInstance`.
- **Fix direction**: derive the pivot's FetchFilter from `resource.BuildCloudTrailFilter(res, "dbi")`, as `ctEventsCheckerFor` does. When matching cached events, accept both the identifier and the `…:db:<id>` ARN form, as `checkLambdaCTEvents` does for functions.

## 6. P3 — `dbi_events` row IDs are not unique, so events are dropped when pages merge

- **File/line**: `core/aws/rds_events.go:70` and `:95` (`id := timestamp + "/" + sourceIdentifier`, with the timestamp at minute precision)
- **Trigger**: an instance emits several events within the same minute (a reboot gives "DB instance shutdown", "DB instance restarted", …; so do backup start/finish and failovers), and the 7-day event list spans more than one page (the `DescribeEvents` default page size is 100).
- **User impact**: `SourceIdentifier` is always the same instance, so events in the same minute share one row ID. When a later page is appended, `dedupAgainstExisting` → `resource.DedupByID` (core/app/list_body.go:149, core/resource/resource.go:25-28) silently drops every incoming event whose ID is already present or repeats within the page. An operator reading the timeline can miss a failure or failover event. Cursor and detail lookups keyed by ID can also land on the wrong event.
- **Fix direction**: build the ID from the full-precision `event.Date` (RFC3339Nano) plus `SourceArn`, `EventCategories`, and the message (or a hash of it). The display timestamp can keep minute precision.

## ddb

# ddb — production code review

Scope: `core/aws/ddb.go`, `ddb_codes.go`, `ddb_interfaces.go`, `ddb_issue_enrichment.go`, `ddb_related.go`, `ddb_related_extra.go`, the `ddb` catalog entry (`core/aws/catalog_databases.go:395-447`), `core/config/defaults_databases.go` (`ddb` detail), plus the helpers they call (`backup_coverage.go`, `backup_match.go`, `related_common.go`, `teardown.go`, `degraded_resource.go`, `resource/related.go`).

Findings: 4 (P0:0 P1:0 P2:2 P3:2)

---

## 1. [P2] The ddb → lambda pivot resolves an alias-qualified mapping to the alias name, not the function

- **File/line**: `core/aws/ddb_related.go:183-185` (`checkDdbLambda`)
- **Code**:

  ```go
  parts := strings.Split(*m.FunctionArn, ":")
  name := parts[len(parts)-1]
  ```

- **Trigger**: an event source mapping on the table's stream was created against a version or alias (`FunctionName = arn:aws:lambda:…:function:orders-consumer:prod`). `ListEventSourceMappings` returns the qualified `FunctionArn` (`…:function:orders-consumer:prod`).
- **User impact**: the Lambda Functions row in the related panel shows the ID `prod` (or `3`) instead of `orders-consumer`. Drilling into it navigates to a Lambda function that does not exist. If one mapping is qualified and another is not, the same function is counted twice.
- **Fix direction**: delete the hand-rolled body and delegate to the shared `lambdaEventSourceMappingLambdaCheck(ctx, clients, streamARN, cache)` (`core/aws/related_common.go:290`). The kinesis and msk pivots already use it. It strips the qualifier through `resource.LambdaNameFromARN`, dedupes by ARN, resolves IDs against the lambda cache and retries on throttling.

## 2. [P2] The ddb → backup pivot ignores tag-based selections and partially read plans, and reports a proven 0

- **File/line**: `core/aws/ddb_related.go:94-106` (`checkDdbBackup`)
- **Trigger**: (a) a backup plan selects tables by tag (`ListOfTags`, or `Conditions.StringEquals` on `aws:ResourceTag/...`, stored in the plan row's `selection_tags` field) and has no ARN in `Resources` that matches this table. (b) A plan's selection walk stopped early, so the row carries `selections_partial`.
- **User impact**: the panel shows "Backup Plans (0)" as a known, untruncated zero for a table that a plan does back up. Meanwhile the Wave 2 coverage join for the same row (`addBackupCoverage`/`backupTagsAccessor`, `core/aws/backup_coverage.go:35-126`) reads the table's tags, sees the plan covers it, and correctly raises no "not covered by a backup plan" finding. The panel and the status column contradict each other. In case (b) the panel states a zero the data cannot support. Separately, the panel matches ARNs with `selector.MatchARN` (`backup_match.go`) while the join uses `backupARNPatternMatches`. That is two matchers for the same fact.
- **Fix direction**: route the panel through the same coverage predicate the enricher uses: `backupPlanCovers(plan, arn, tags)` plus the `backupPlansIncomplete` guard. When any plan selects by tag, read the table's tags with `dynamoDBTagsForARN` (one call; the client already satisfies `DynamoDBListTagsOfResourceAPI`). If the tags cannot be read, or a plan is partial, return a truncated or unknown result instead of a known 0.

## 3. [P3] The Billing column is blank for provisioned tables

- **File/line**: `core/aws/ddb.go:113-116`
- **Code**:

  ```go
  if table.BillingModeSummary != nil {
      billingMode = domain.HumanizeStatusPhrase(string(table.BillingModeSummary.BillingMode))
  }
  ```

- **Trigger**: any table that has always used the default PROVISIONED mode. `DescribeTable` does not return `BillingModeSummary` for such tables. The summary appears only once a billing mode was set explicitly or switched (AWS behaviour; see cloud-custodian issue #6268 and the `BillingModeSummary` API reference).
- **User impact**: the list's "Billing" column is empty for what is usually most of an account's older tables. On-demand tables show "pay per request", so the column is only filled for one mode.
- **Fix direction**: when `BillingModeSummary` is nil, fall back to `PROVISIONED` (the documented default), humanized the same way.

## 4. [P3] The resource-policy pass calls `GetResourcePolicy` with an empty ARN for describe-denied or degraded rows

- **File/line**: `core/aws/ddb_issue_enrichment.go:120-126` (`enrichDDBResourcePolicies`)
- **Trigger**: `DescribeTable` is denied or returns a nil table for a listed table. `FetchDynamoDBTablesPage` (`ddb.go:82-91`) then keeps the row through `DegradedDetails`, whose `Fields` have no `"arn"` and whose `RawStruct` is nil. The guard at line 120 checks only `name` and teardown, so the pass sends `GetResourcePolicy(ResourceArn: "")`.
- **User impact**: every degraded ddb row makes a call that is certain to fail. The call returns a ValidationException, and `MarkSkipped` plus `Finish` turn it into a logged `GetResourcePolicy` failure and a truncated (lower-bound) issue count for the whole type. The operator sees an error that names the wrong cause: a malformed request instead of the denied describe.
- **Fix direction**: skip the call when `r.Fields["arn"] == ""` and mark the row uninspected (`markUninspected`) with no API call, the same way `addBackupCoverage` already skips ARN-less rows. An alternative is to build the ARN from the table name, region and account.

## eb-rule

# eb-rule — production code review

Scope: `core/aws/eb_rule*.go`, the `eb-rule` / `eb_rule_targets` catalog entries in `core/aws/catalog_messaging.go`, `core/config/defaults_messaging.go`, `core/resource/columns.go` (`EbRuleTargetColumns`), and the Wave-2 fold / badge / field-update paths they feed (`core/runtime/helpers.go`, `core/runtime/handlers_availability.go`, `core/app/list_body.go`, `core/app/list_state.go`).

## 1. P1: rules on custom event buses are never listed

- **File:** `core/aws/eb_rule.go:19` (`ListRulesInput` has no `EventBusName`, and nothing enumerates buses. `EventBridgeAPI` in `core/aws/eb_rule_interfaces.go:28` has no `ListEventBuses`).
- **Trigger:** an account has rules on any bus other than `default`, such as a custom application bus or a partner bus. The AWS ListRules reference says: "EventBusName … If you omit this, the default event bus is used."
- **User impact:** those rules never appear in the list, the menu count, or Wave-2 checks. An enabled custom-bus rule with no targets can never be flagged. The `Event Bus` column, which the spec treats as the way to tell buses apart, only ever shows `default`. The dev snapshot collector (`cmd/snapshot/messaging.go:243`) has the same default-only scope.
- **Fix direction:** page `ListEventBuses`, then run `ListRules` per bus with `EventBusName`. The same rule name can exist on several buses, so make the row ID unique per bus (for example the rule ARN, or `bus/name`). Update everything keyed on the bare name to match: `ContextKeys.rule_name: "ID"`, enrichment `FieldUpdates[ruleName]` and findings keys, `ConsoleURL`'s `r.ID`, and `CloudTrailKey`. The related-panel lookup at `core/aws/eb_rule_related.go:52` must also pass `EventBusName`. Today it omits it, so once custom-bus rules are listed every target pivot would show "?" (ResourceNotFound) for them.

## 2. P1: a failed ListTargetsByRule reports "enabled rule has no targets" and `Targets 0`

- **File:** `core/aws/eb_rule_issue_enrichment.go:98` (`noTargets` has no `fetchErr` guard) and `:141` (`target_count` is written even when the call failed).
- **Trigger:** `ListTargetsByRule` fails for an ENABLED rule, from throttling under `EnrichmentParallelism`, AccessDenied on `events:ListTargetsByRule`, or an error on a later page. `targets` is then empty (or partial), `targetsTruncated` is false, and `noTargets` becomes true. `MarkSkipped` records the row as uninspected, but `setWave2Finding(... ebRuleCodeNoTargets ...)` still writes into `result.Findings`, and `FieldUpdates` still gets `"0"`, or a partial count with no `+`.
- **User impact:**
  - The menu and list issue badge count every affected rule as broken. `unifiedIssueCount` (`core/runtime/handlers_availability.go:1043`) reads `findings[r.ID]` whatever `TruncatedIDs` says, so the badge claims broken rules that were never inspected. That is a false red signal at the top level.
  - The field update is applied unconditionally (`core/runtime/handlers_availability.go:766-781`, `core/app/list_body.go:927`), so the `Targets` column shows a confident `0` for a rule whose targets were never read.
- **Fix direction:** when `fetchErr != nil`, call `MarkSkipped` and return before computing `noTargets`, the DLQ and drift rows, or `FieldUpdates`. Leave `target_count` unset, or mark it unknown, so the row shows "?".

## 3. P2: the SNS related pivot drills into an empty list

- **File:** `core/aws/eb_rule_related.go:91-94` (`case "sns", "sqs"` keeps only the text after the last `:`).
- **Trigger:** an eb-rule with an SNS topic target. The user opens detail and follows "SNS (targets)".
- **User impact:** the checker returns the bare topic name, but SNS rows use the full topic ARN as their ID (`core/aws/sns.go:42`). The exact-ID drill (`core/app/list_state.go:281-300`, `set[r.ID]`) matches nothing. The panel shows a count of 1 or more, but the drilled list is empty, or loads without the topic. `checkS3SNS` (`core/aws/s3_related.go:86-89`) documents this exact failure and returns the ARN unchanged.
- **Fix direction:** for `sns`, return the target ARN as-is. Keep the last-segment extraction for `sqs` only, because SQS IDs are queue names (`core/aws/sqs.go:75`).

## 4. P2: the SQS pivot leaves out target dead-letter queues

- **File:** `core/aws/eb_rule_related.go:143` (`checkEbRuleSQS`), through `ebRuleTargetsByService` at `:56-100`, which reads only `Targets[].Arn`.
- **Trigger:** a rule target has `DeadLetterConfig.Arn` pointing at an SQS queue, which is the normal DLQ setup.
- **User impact:** the spec (§2 `sqs`) requires both primary SQS targets and `Targets[].DeadLetterConfig.Arn`, de-duplicated. The DLQ, where failed events actually pile up, is not reachable from the rule, and the SQS count understates what the spec promises.
- **Fix direction:** in the SQS path, also collect `t.DeadLetterConfig.Arn` (queue name after the last `:`) and de-duplicate against primary targets.

## 5. P3: `ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS` rules with no targets are not flagged

- **File:** `core/aws/eb_rule_issue_enrichment.go:98` (`state == "ENABLED"`).
- **Trigger:** a rule in the `ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS` state (a valid `RuleState`, listed in spec §6) that has zero targets.
- **User impact:** the rule is active and matching events but delivers them nowhere, yet no broken finding is raised and the badge misses it.
- **Fix direction:** treat any state other than `DISABLED` as enabled, or explicitly include `ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS`.

## 6. P3: two different "no DLQ" checks disagree

- **File:** `core/aws/eb_rule_issue_enrichment.go:111` (`target.DeadLetterConfig == nil`) and `core/aws/eb_rule_targets.go:102` (`nil || Arn == nil || *Arn == ""`).
- **Trigger:** a target whose `DeadLetterConfig` object is present but has no `Arn` or an empty one. `Arn` is optional on the `DeadLetterConfig` shape.
- **User impact:** the target list flags "no DLQ configured", while the parent rule's Wave-2 check calls the same target fine. The rule row and its drill-down contradict each other.
- **Fix direction:** have the enricher call the one predicate the target list already uses (`ebRuleTargetFindings` or a shared `targetHasDLQ`), so both surfaces make the same decision.

## 7. P3: the targets child fetcher sends an empty `EventBusName`

- **File:** `core/aws/eb_rule_targets.go:33-36` (`EventBusName: &eventBus` is set unconditionally).
- **Trigger:** the parent row's `event_bus` field is empty. `FetchEventBridgeRulesPage` leaves it `""` when `Rule.EventBusName` is nil (`core/aws/eb_rule.go:47-50`), and the AWS ListRules example response has no `EventBusName`. The enricher guards this same case (`eb_rule_issue_enrichment.go:74-76`). The child fetcher does not, so it sends `"EventBusName": ""`, which breaks the parameter's minimum-length-1 constraint.
- **User impact:** pressing Enter on such a rule shows a validation error instead of its targets.
- **Fix direction:** set `EventBusName` only when `eventBus != ""`, as the enricher and `cmd/snapshot/messaging.go:273-275` do.

## Summary

| Priority | Count |
|---|---|
| P0 | 0 |
| P1 | 2 |
| P2 | 2 |
| P3 | 3 |

## eb

# eb — production-code review

Scope: `core/aws/eb.go`, `eb_codes.go`, `eb_interfaces.go`, `eb_issue_enrichment.go`, `eb_related.go`, `eb_related_extra.go`, the `eb` catalog entry (`core/aws/catalog_messaging.go`), `colorEB` (`core/aws/catalog_compute.go`), the `eb` view defaults (`core/config/defaults_compute.go`), the reverse pivot `checkSecretsEB` (`core/aws/secrets_related_extra.go`), and `captureEB` (`cmd/snapshot/compute.go`). I checked the AWS API semantics against the current AWS docs.

Summary: 5 findings (P0:0 P1:0 P2:5 P3:0)

---

## 1. P2 — Wave 2 calls DescribeEnvironmentHealth on every environment, including ones where the call cannot succeed

- **File/line**: `core/aws/eb_issue_enrichment.go:66-74`
- **Evidence**: `EnrichEBEnvironmentHealth` calls `DescribeEnvironmentHealth` for every row that has a name. It has no check for health-reporting mode and no check for lifecycle state. The sibling pass in the same file, `ebConfigurationPosture` at lines 118-120, skips Terminating/Terminated rows with `ebLifecycleEnded`, so the two passes disagree. The AWS API reference for DescribeEnvironmentHealth says: "The DescribeEnvironmentHealth operation is only available with AWS Elastic Beanstalk Enhanced Health." Its only modeled errors are `InvalidRequest` and `ElasticBeanstalkService`. Neither code is in `notFoundCodes` (`core/aws/partial_errors.go:89-139`), so `MarkSkipped` (line 73) marks the row uninspected and records a failure. `AggregateFailures("DescribeEnvironmentHealth", …)` then returns a non-nil error.
- **Trigger**: An account has an environment on basic health reporting. That environment already gets the `enhanced health reporting off` finding from the posture pass. A Terminated environment in the list (the type deliberately models these as `eb.status.terminated`) also triggers it.
- **User impact**: On every list load, each basic-health or terminated environment shows as not inspected, and Wave 2 reports a DescribeEnvironmentHealth failure. This is expected API behavior, not a real failure, so the error signal is noise and trains operators to ignore real enrichment errors. The call is keyed by `EnvironmentName`, not by `EnvironmentId`. If a terminated environment's name has been reused by a live environment, the terminated row gets the live environment's Causes.
- **Fix direction**: Skip rows where `ebLifecycleEnded(status)` is true, as the posture pass does. Treat the enhanced-health-not-enabled answer as "no causes available" instead of a failure; the posture pass already knows `SystemType`, so the result can gate the causes call. Address the call by `EnvironmentId` (`r.ID`).

## 2. P2 — IAM Role pivot passes a ServiceRole ARN through unnormalized, so the pivot opens nothing

- **File/line**: `core/aws/eb_related_extra.go:301-303`
- **Evidence**: The `aws:elasticbeanstalk:environment / ServiceRole` value goes into `ids` verbatim (`ids = append(ids, val)`), and the code comment itself says "ServiceRole may be a role ARN or a role name". `role` rows are keyed by role name (`core/aws/iam_roles.go:157` `ID: roleName`). The instance-profile branch on the same pivot already converts ARNs to names (`asgInstanceProfileToRoles` → `arnRoleName`, `core/aws/asg_related.go:337`).
- **Trigger**: An environment whose ServiceRole option holds an ARN (`arn:aws:iam::123456789012:role/aws-elasticbeanstalk-service-role`).
- **User impact**: The IAM Role row counts the service role, but the drill-down cannot match the ARN to any role row, so the service role never appears. When the instance-profile role and the service role are the same role, the name and the ARN are counted as two roles.
- **Fix direction**: Normalize with `roleNameFromARN(val)` before appending, the same way the instance-profile branch does.

## 3. P2 — Classic-ELB environments break the Target Group pivot and give the Load Balancer pivot an ID the elb list cannot hold

- **File/line**: `core/aws/eb_related_extra.go:105-113` (tg), `:150-152` (tg error), `:51-55` (elb)
- **Evidence**: `checkEbTG` resolves each `EnvironmentResources.LoadBalancers[].Name` through `elbv2:DescribeLoadBalancers(Names=[name])`. A Beanstalk environment with `LoadBalancerType=classic` has a Classic ELB, which the elbv2 API does not return, so the call fails with `LoadBalancerNotFound`. The failure is appended to `failures` (line 112). With no target-group ARNs, `AggregateFailures` turns it into `ErrorRelated("tg", …)` (line 152) instead of a proven zero. `checkEbELB` returns the classic name as an `elb` ID. The `elb` type lists only ELBv2 load balancers (`catalog_networking.go:118` → `FetchLoadBalancersPage(ctx, c.ELBv2, …)`, IDs are ELBv2 names at `core/aws/elb.go:64`).
- **Trigger**: Open the detail view of an environment that uses a Classic Load Balancer.
- **User impact**: The Target Groups row becomes an error dead end, with an error flash and a log entry on every open. The Load Balancers row shows `(1)`, but drilling in shows an empty list.
- **Fix direction**: In `checkEbTG`, treat a not-found answer for a name as "not an ELBv2 load balancer" (a proven zero for that LB) instead of a failure. In `checkEbELB`, return only names the ELBv2 API can resolve (or check them against the `elb` cache), so a classic LB is not counted as an `elb` row.

## 4. P2 — Reverse pivot Secrets Manager secret → Elastic Beanstalk misses Beanstalk's native secret references

- **File/line**: `core/aws/secrets_related_extra.go:98`, `:132`
- **Evidence**: The checker only matches option values that contain `{{resolve:secretsmanager:<ARN>`. Elastic Beanstalk's documented mechanism for secrets in environment variables (platform versions from 2025-03-26 on, "Fetching secrets and parameters to Elastic Beanstalk environment variables") is the `aws:elasticbeanstalk:application:environmentsecrets` namespace, where `Value` is the plain secret ARN. The `{{resolve:` prefix never appears there.
- **Trigger**: A secret is wired into an environment through `environmentsecrets` and the operator opens that secret's detail view.
- **User impact**: The Elastic Beanstalk row shows a proven `(0)` dead end for a secret that an environment actually consumes. The operator concludes the secret is unused, so a rotation or deletion looks safe when it is not.
- **Fix direction**: Also match options where `Namespace == "aws:elasticbeanstalk:application:environmentsecrets"` and `Value` equals the secret ARN or starts with it (e.g. a JSON-key suffix). Keep the `{{resolve:` match as a secondary pattern.

## 5. P2 — CloudWatch Alarms pivot matches unrelated alarms by substring and by any dimension

- **File/line**: `core/aws/eb_related.go:185-193`
- **Evidence**: An alarm counts as related if any dimension, of any name, has a value equal to the environment name. It also counts if the alarm ID contains the environment name as a substring (`strings.Contains(a.ID, envName)`). The spec (`docs/resources/eb.md` §2 `alarm`) calls for a match on `Dimensions=[{Name:EnvironmentName,Value:<env>}]`. Environment names can be as short as 4 characters (`prod`, `test`, `api1`).
- **Trigger**: An environment named `prod`, in an account with alarms like `prod-rds-cpu` or `payments-prod-5xx`, or with an alarm on another resource whose dimension value happens to equal the environment name.
- **User impact**: The alarm count is inflated, and the drill-down lists alarms for databases, queues, and other environments. This defeats the pivot's purpose of showing which alarms fire when this environment degrades.
- **Fix direction**: Match only the `EnvironmentName` dimension (optionally restricted to namespace `AWS/ElasticBeanstalk`) using the existing `alarmIDsByDimension` helper (`core/aws/related_common.go:211`), and drop the substring fallback.


## ebs-snap

# ebs-snap: production code review

Scope: `core/aws/ebs.go` (fetchers, row builder, Wave 1 findings), `core/aws/ebs_snap_issue_enrichment.go` (Wave 2), `core/aws/snapshot_cross_ref.go`, `core/aws/ebs_snap_related.go`, the `ebs-snap` catalog entry in `core/aws/catalog_compute.go`, `core/config/defaults_compute.go`, and the shared helpers these call (`walkAccountPages`, `related_common.go`, `backup_coverage.go`).

Six findings: P0:0, P1:0, P2:3, P3:3.

---

## 1. P2: Copied snapshots are flagged "orphan: source volume deleted", and their EBS pivot and link go to a placeholder volume ID

- **Where**: `core/aws/ebs_snap_issue_enrichment.go:109-114` (`GetParentID` returns `Snapshot.VolumeId` without any check); `core/aws/ebs_snap_related.go:54` (`checkEBSSnapEBS`); `core/aws/catalog_compute.go:794` (`Navigable` `VolumeId` → `ebs`).
- **AWS semantics**: the `Snapshot.VolumeId` docs say: "Snapshots created by a copy snapshot operation have an arbitrary volume ID that you should not use for any purpose." In practice this placeholder is `vol-ffffffff`.
- **Trigger**: the account holds a snapshot made with `CopySnapshot`. This covers cross-region DR copies, cross-account copies, and the re-encrypted copy that the `ebs-snap.encryption.disabled` finding itself tells the operator to make. The `ebs` list is loaded and not truncated.
- **Impact**:
  - Every such copy gets a Warning row that says `orphan: source volume deleted`, with a false "delete it" instruction.
  - The detail view shows "EBS Volume (1)" pointing at a volume that does not exist, and the VolumeId field can be navigated to the same dead target.
  - Following the unencrypted finding's advice therefore produces a new warning.
- **Fix direction**: treat a VolumeId that the source cannot vouch for as "no parent". Examples are the `vol-ffffffff` placeholder, or any snapshot whose `Description` or copy provenance marks it as a copy. `GetParentID` should return `("", false)`, `checkEBSSnapEBS` should return a known zero, and the navigable link should be suppressed. Put this in one predicate that all three sites share.

## 2. P2: The Backup pivot reports a proven "0" for Backup-created snapshots whose plan selects by wildcard or tag

- **Where**: `core/aws/ebs_snap_related.go:133-139` (`checkEBSSnapBackup`, `arn == sourceARN` exact string equality against `Fields["resources"]`).
- **Trigger**: the snapshot carries `aws:backup:source-resource`. The owning plan selects resources the way plans usually do: with an ARN wildcard (`arn:aws:ec2:*:*:volume/*`, `*`), or with a tag condition (`selection_tags`). A plan whose selections were only partly enumerated (`backupSelectionsPartialField`) has the same effect.
- **Impact**: the related panel shows "Backup (0)" as a resolved count for a snapshot that AWS Backup demonstrably created. That is the "safe to delete this orphan" signal the spec says this pivot exists to prevent. The same codebase already has the correct predicate (`backupPlanCovers` / `backupARNListMatches` in `core/aws/backup_coverage.go:167-203`, which handles wildcards, `not_resources` and tags) but does not use it here. This is a second copy of the same fact, and it disagrees with the first.
- **Fix direction**: route the match through `backupPlanCovers` (or `backupARNListMatches` plus the exclude list). Return Unknown instead of a known zero when any plan selects by tag (the source volume's tags are not on the snapshot) or when `backupPlansIncomplete` is true.

## 3. P2: A snapshot in the `error` state never shows AWS's `StateMessage`

- **Where**: `core/aws/ebs.go:303` (`wave1Finding(CodeEBSSnapStateError)` carries no detail row); `core/config/defaults_compute.go:128-131` (the `ebs-snap` detail field list has no `StateMessage`).
- **Spec**: `docs/resources/ebs-snap.md` §3.1 says `StateMessage` "carries AWS's human-readable cause … and is used for S4/S5 text". Its UX citation pairs `error` with `<StateMessage>` so that a red row can be triaged.
- **Trigger**: an encrypted cross-region or cross-account copy fails, for example because a KMS grant is missing. AWS sets `State=error` and a `StateMessage`.
- **Impact**: the list says only `error`. The detail Attention block and field list never show why the snapshot failed, so the operator cannot diagnose it (usually a KMS permission) without leaving a9s.
- **Fix direction**: add a `StateMessage` detail row to the `error` finding when the field is non-empty. At minimum, add `{Path: "StateMessage"}` to the `ebs-snap` detail defaults.

## 4. P3: The EC2 pivot reports a proven "0" for every snapshot not made by `CreateImage`

- **Where**: `core/aws/ebs_snap_related.go:59-65` (`checkEBSSnapEC2`).
- **Trigger**: any snapshot whose Description does not contain `Created by CreateImage(i-…)`. This covers manual snapshots, DLM snapshots and AWS Backup snapshots of a volume that is still attached to a running instance.
- **Impact**: the panel shows "EC2 Instance (0)" as a resolved answer. The source volume's `Attachments[].InstanceId` actually names the instance, which is the restore and rollback relationship the spec describes (path (a): volume, then attachments). A missing description match proves nothing, yet it is reported as known-none.
- **Fix direction**: when the description does not match, resolve through the `ebs` cache (`Volume.VolumeId == Snapshot.VolumeId` → `Attachments[].InstanceId`). Return Unknown when that cache is absent, and keep a truncated result where the cache is truncated.

## 5. P3: `FetchEBSSnapshotsByIDs` loses the whole batch when any ID no longer exists, and reports a generic error instead of "missing"

- **Where**: `core/aws/ebs.go:207-214`.
- **AWS semantics**: the DescribeSnapshots docs say: "If you specify an invalid snapshot ID, an error is returned" (`InvalidSnapshot.NotFound`). AWS does not return a partial `snapshotSet`. The function's own comment (lines 191-193) and the missing-ID aggregation (lines 223-231) assume the partial-list behaviour, so the aggregation never runs for a deleted ID. The sibling `FetchEC2InstancesByIDs` (`core/aws/ec2_by_ids.go:52-80`) handles exactly this whole-batch rejection.
- **Trigger**: a lazy-add or by-ID drill (the `ami` → `ebs-snap` pivot, the `BlockDeviceMappings.Ebs.SnapshotId` navigable field, or any cache-miss exact-ID drill) names one snapshot ID that has been deleted or is malformed.
- **Impact**: the valid snapshots in the same batch are not shown. The operator sees a raw API error rather than a "not found / missing" answer for the one bad ID.
- **Fix direction**: mirror the EC2 path. On `InvalidSnapshot.NotFound` / `InvalidSnapshotID.Malformed`, parse the offending `snap-…` IDs from the message and retry once without them. Report the excluded IDs through `AggregateMissing`, and never retry with an empty ID list, because that would be an unfiltered call.

## 6. P3: The Wave 2 public-share check labels a refused call "stopped at the inspection cap"

- **Where**: `core/aws/issue_enrichment.go:599-619` (`walkAccountPages`). On a page error it breaks out of the loop and falls through to `markUninspected(result, r.ID, CheckCap)` for every unseen row. It is called from `core/aws/ebs_snap_issue_enrichment.go:74`.
- **Trigger**: `DescribeSnapshots(OwnerIds=self, RestorableByUserIds=all)` fails, for example with AccessDenied from a restrictive SCP or a non-throttle error on page 1.
- **Impact**:
  - Every ebs-snap row records the uninspected reason as `CheckCap`. Per its own contract (`issue_enrichment.go:300-305`), `CheckCap` means "nothing refused", and it is "the one mark the runtime acts on" by re-running checks on demand when the detail view opens.
  - So the operator is told a9s stopped at its own bound, when in fact AWS refused the call, and each detail open retries a call that will be denied again.
  - The aggregate `Truncated` flag is also raised through `if cut { SetTruncated(...) }` (`ebs_snap_issue_enrichment.go:98-100`), which presents a failure as a cap.
- **Fix direction**: in `walkAccountPages`, when `err != nil`, mark the unseen rows with the failing check (`checkOf(err)` / `MarkSkipped`) instead of `CheckCap`, and return `cut=false` for an error-terminated walk. Only a walk that exhausts `EnrichmentCap` is a cap.

---

### Considered and disproved

- The public-share query uses `RestorableByUserIds=["all"]` together with `OwnerIds=["self"]`. This is valid: the DescribeSnapshots docs say you can specify "`all` for public snapshots" in the restorable-users list.
- The orphan rule when the `ebs` cache is truncated: it is skipped (`snapshot_cross_ref.go:171-173`), which is correct.
- List pagination (`FetchEBSSnapshotsPage`) carries `NextToken` and `IsTruncated` correctly and uses `MaxResults`.
- The KMS pivot's ARN→key-id split (`ebs_snap_related.go:73-86`) is consistent with the ebs sibling (`checkEBSKMS`).
- The spec's "age > 365d AND `Description` begins with `Created by`" rule differs from `ebs.go:337-338` (`Created by CreateImage` prefix, or `automated` substring). This is not reported: which real-world automated descriptions should count could not be confirmed against AWS documentation.

## ebs

# ebs — production-code review

Scope: `core/aws/ebs.go`, `core/aws/ebs_codes.go`, `core/aws/ebs_issue_enrichment.go`, `core/aws/ebs_related.go`, the `ebs` catalog entry in `core/aws/catalog_compute.go` (lines 705–755, `colorEBS` at 113–119), the `ebs` detail view in `core/config/defaults_compute.go:119-126`, and the shared helpers these call (`core/aws/backup_coverage.go`, `core/aws/related_common.go`, `core/aws/related_fetch.go`, `walkAccountPages` in `core/aws/issue_enrichment.go`).

## 1. P2: `insufficient-data` volume status is reported as broken "volume I/O degraded"

- **File/line**: `core/aws/ebs_issue_enrichment.go:75-103`
- **Code**: the only statuses skipped are nil and `ok` (`v.VolumeStatus.Status == ec2types.VolumeStatusInfoStatusOk`). Every other value gets `ebsCodeVolumeIODegraded` (SevBroken). The SDK enum `VolumeStatusInfoStatus` has four values: `ok`, `impaired`, `insufficient-data`, `warning`.
- **Trigger**: a volume whose status checks are still running, or whose status AWS cannot determine. `DescribeVolumeStatus` then returns `VolumeStatus.Status = "insufficient-data"`.
- **User impact**: the row turns red and says `volume I/O degraded`. The detail text says "reads may return stale or corrupt blocks" and tells the user to snapshot and restore onto a fresh volume. The main-menu broken count goes up for a volume AWS has not reported a problem on. The spec (§3.2, §4) lists only `impaired` and `warning` as Broken.
- **Fix direction**: flag only `impaired` and `warning`. Treat `insufficient-data` (and any status the code does not recognise) as no finding, or as uninspected.

## 2. P2: the Backup related panel and the "not covered by a backup plan" finding use different matching rules

- **File/line**: `core/aws/ebs_related.go:143-149` (`checkEBSBackup`), compared with `core/aws/ebs_issue_enrichment.go:36-39` → `core/aws/backup_coverage.go:152-172` (`backupPlansCover` / `backupPlanCovers`)
- **Code**: `checkEBSBackup` matches plans only with `backupSelectionTagsMatch(planRes.Fields["selection_tags"], tags)`. The Wave 2 enricher uses `backupPlanCovers`, which also checks the plan's `resources` ARN list and its `not_resources` exclusions (`core/aws/backup.go:81-87`). It also ignores plans whose selections were only partly read (`selections_partial`). The same fact is computed two ways.
- **Trigger A**: a plan selects volumes by ARN, for example `Resources: ["arn:aws:ec2:*:*:volume/*"]` or `"*"`, which is the usual "back up every EBS volume" setup.
- **Trigger B**: a plan selects by tag but excludes this volume's ARN in `NotResources`.
- **User impact**:
  - Trigger A: the volume row has no "not covered" warning, but the detail panel shows `Backup (0)`. The user checking "is this volume protected before I delete it?" is told it is not.
  - Trigger B: the panel lists the plan as covering the volume, while the Attention section warns `not covered by a backup plan`.
  - When a plan's selections were only partly read, the panel still shows a definite `0`.
- **Fix direction**: have `checkEBSBackup` call the same `backupPlanCovers(plan, ebsVolumeARN(...), tags)` predicate. Return unknown or truncated when `backupPlansIncomplete` is true, or when tags are unknown and some plan selects by tag. That leaves one predicate for both surfaces.

## 3. P2: volumes protected through an instance-level backup selection are flagged "not covered by a backup plan"

- **File/line**: `core/aws/ebs_issue_enrichment.go:36-39`. The same gap exists in `core/aws/ebs_related.go:121-150`.
- **Code**: the coverage join tries only the volume's own ARN (`ebsVolumeARN` → `...:volume/vol-…`) and the volume's own tags. It never considers the instance in `Volume.Attachments[].InstanceId`.
- **Trigger**: an AWS Backup plan selects EC2 instances, by instance ARN (`arn:aws:ec2:*:*:instance/*`) or by a tag set on the instance and not propagated to its volumes. AWS Backup backs up an EC2 instance as an AMI that includes the attached EBS volumes, so those volumes are protected.
- **User impact**: every attached volume of a backed-up instance gets a Warning with the text "No backup plan selects this volume … a deletion is final". The Backup panel shows 0. In the common instance-level backup setup, this is a false alarm on every in-use volume, and it wrongly tells operators the data is unprotected.
- **Fix direction**: for attached volumes, also test each attached instance's ARN, plus its tags from the `ec2` cache, against the plans. When the plans select instances and the `ec2` cache is missing or cut short, report nothing (unknown) instead of "uncovered". Apply the same rule in `checkEBSBackup`, which should share the predicate after fix 2.

## 4. P3: the EC2 related pivot shows only the first attachment of a Multi-Attach volume

- **File/line**: `core/aws/ebs.go:76-79` (`attached_to` = `Attachments[0].InstanceId` only), consumed by `core/aws/ebs_related.go:16-22` (`checkEBSEC2`)
- **Trigger**: an io1/io2 volume with `MultiAttachEnabled=true` that is attached to 2 to 16 instances.
- **User impact**: the related panel shows `EC2 Instance (1)` and links to one instance, so the other instances are missing. The spec (§2 `ec2`) says the count is "1+ when multi-attach is enabled". An operator checking "whose workload does this carry?" before a change misses instances that use the volume.
- **Fix direction**: in `checkEBSEC2`, read every `Attachments[].InstanceId` from `RawStruct`, de-duplicated. Fall back to `Fields["attached_to"]` (marked truncated, or unknown) only when `RawStruct` is missing. The list column can stay first-attachment only.

## 5. P3: the orphan finding reports the volume's age as its unattached time

- **File/line**: `core/aws/ebs.go:362-368`. The phrase and detail text are in `core/aws/catalog_compute.go:749`.
- **Code**: `<N>` is `time.Since(CreateTime)`. The phrase reads `orphan: unattached <N>d`, and the detail says "The volume has been unattached since it was created".
- **Trigger**: a volume created 400 days ago and detached minutes ago, for example during instance replacement or maintenance.
- **User impact**: the row reads `orphan: unattached 400d` and the detail says it has never been attached. The user is told to delete it, even though it may have been serving a workload an hour ago. The API has no detach timestamp on `Volume`, so the displayed duration is invented.
- **Fix direction**: keep the signal as the spec defines it, but change the wording to say what is known, for example `orphan: unattached, created <N>d ago`. Also change the detail text so it no longer says the volume was never attached.

## 6. P3: a snapshot in `error` state counts as the volume's snapshot

- **File/line**: `core/aws/ebs_issue_enrichment.go:159-164` (`addEBSSnapshotCoverage`)
- **Code**: every cached `ebs-snap` row with a `volume_id` marks that volume as snapshotted, whatever its `state`.
- **Trigger**: an in-use volume whose only snapshots have state `error`. The ebs-snap fetcher itself classes these as broken, "holds no usable copy of the volume" (`core/aws/ebs.go:302-303`, catalog detail text at `catalog_compute.go:799`).
- **User impact**: the `no snapshot exists` warning is suppressed, so the volume appears to have a restore point when it has none.
- **Fix direction**: count only snapshots whose `state` is `completed` (and `pending`, if an in-flight snapshot should satisfy the check). Ignore `error`, `recoverable` and `recovering`.

## ec2

# ec2 — production code review

Scope: `core/aws/ec2.go`, `ec2_by_ids.go`, `ec2_codes.go`, `ec2_interfaces.go`, `ec2_issue_enrichment.go`, `ec2_detail_enrichment.go`, `ec2_related.go`, `ec2_related_extra.go`, the `ec2` entry in `core/aws/catalog_compute.go`, and the helpers they call (`backup_match.go`, `related_common.go`, `core/runtime/handlers_related.go`, and the `ssm` entry in `catalog_secrets.go`).

## 1. P2: the ec2 → tg pivot lists every instance-type target group in the VPC, not the groups the instance is registered in

- **File:** `core/aws/ec2_related.go:54` (`checkEC2TargetGroups`)
- **Code:** any target group with `target_type == "instance"` and `tgVpcID == vpcID` is counted. Registration is never checked: no `DescribeTargetHealth` call, and no cached `Target.Id` is read.
- **Trigger:** a VPC with N instance-type target groups. Open any instance in that VPC, including one registered in no target group.
- **User impact:** the related panel shows "Target Groups (N)" for every instance in the VPC, and following the pivot lists target groups the instance does not belong to. During an incident this points the operator at the wrong load-balancer path. The spec (§2 `tg`) says membership comes from `DescribeTargetHealth` `Target.Id == <instance-id>`. The sibling `checkLambdaTG` (`lambda_related_extra.go:320`) already does this.
- **Fix direction:** follow `checkLambdaTG`. For each instance-type target group in the instance's VPC, call `DescribeTargetHealth` and keep only groups where some `Target.Id` equals the instance ID. A better option is to store the target IDs on the `tg` row (the tg Wave 2 pass already reads them at `tg_issue_enrichment.go:98`) and match against that.

## 2. P2: the ec2 → ssm pivot returns the instance ID as if it were an SSM Parameter

- **Files:** `core/aws/ec2_related.go:366` (`checkEC2SSM`); registration at `core/aws/catalog_compute.go:323` (`TargetType: "ssm"`, `DisplayName: "SSM Parameters"`)
- **Code:** if `ssm:DescribeInstanceInformation` returns a match, the checker reports `relatedResult("ssm", []string{instanceID})`. The `ssm` type is Parameter Store (`catalog_secrets.go:103-107`) and has no `FetchByIDs`.
- **Trigger:** open an instance that is enrolled in SSM.
- **User impact:** the panel shows "SSM Parameters (1)", which claims a parameter exists. Pressing Enter goes through `relatedFetchTasks`/`KindFetchResources` (`core/runtime/handlers_related.go:227-313`) and opens the Parameter Store list filtered to the ID `i-…`. No parameter has that ID, so the list is empty. The count is false and the pivot leads nowhere. The spec (§2 `ssm`) means the SSM Managed-Instance view, which a9s does not have as a type.
- **Fix direction:** stop reporting this result under the Parameter Store type. Either register a managed-instance target type, or show SSM enrollment as a detail field or finding rather than a related-panel count on `ssm`.

## 3. P2: completed scheduled events are still reported as "scheduled event"

- **File:** `core/aws/ec2_issue_enrichment.go:166-183` (the event loop in `ec2InstanceStatusFindings`)
- **Code:** each `is.Events` entry whose `NotBeforeDeadline` or `NotBefore` falls before `now+7d` produces the finding. `ev.Description` is never checked, and a timestamp in the past always satisfies `.Before(cutoff)`.
- **Trigger:** an instance whose retirement, reboot or maintenance event has already been carried out. AWS docs (`API_InstanceStatusEvent`): "After a scheduled event is completed, it can still be described for up to a week. If the event has been completed, this description starts with the following text: [Completed]." A `[Canceled]` event has the same problem.
- **User impact:** for up to a week after the work is done, the instance keeps a Warning "scheduled event" row, a badge count and a detail row naming a date in the past. The finding tells the operator to stop and start or reschedule work that has already happened.
- **Fix direction:** skip events whose `Description` starts with `[Completed]` or `[Canceled]`. If the spec's "overdue" wording is not wanted, also skip events whose deadline has passed.

## 4. P2: ARN-based AWS Backup selections never match an ec2 instance

- **File:** `core/aws/ec2_related_extra.go:156` (`instanceARN := res.Fields["arn"]`)
- **Code:** the ec2 fetcher never writes an `"arn"` field. `ec2InstanceToResource` (`core/aws/ec2.go:143-155`) has no such key and the catalog `FieldKeys` do not list one. `instanceARN` is therefore always `""`, and `BackupPlanCoversARN` returns `false` straight away on an empty ARN (`core/aws/backup_match.go:18-20`).
- **Trigger:** a backup plan whose selection uses `Resources` ARNs or wildcards (for example `arn:aws:ec2:*:*:instance/*`, `*`, or the instance's exact ARN) rather than tags.
- **User impact:** "Backup Plans" shows 0 for instances that such plans do protect. The operator is told the instance has no backup coverage when it does.
- **Fix direction:** build the instance ARN from values the fetcher has: `arn:<partition>:ec2:<region>:<Instance/Reservation OwnerId>:instance/<id>`. Put it in `Fields["arn"]` in `ec2InstanceToResource`, or compute it in the checker.

## 5. P3: "IMDSv1 allowed" is raised for instances whose metadata endpoint is disabled

- **File:** `core/aws/ec2.go:181`
- **Code:** the finding fires on `MetadataOptions.HttpTokens == optional` alone. `MetadataOptions.HttpEndpoint` is never read.
- **Trigger:** an instance with `HttpEndpoint = disabled` and `HttpTokens = optional`, which is a valid combination.
- **User impact:** the finding (Warning, list text "IMDSv1 allowed", detail row "Metadata tokens: optional") says an SSRF bug could read role credentials through metadata. With the endpoint disabled, metadata is not reachable at all, so the posture finding is false.
- **Fix direction:** also require `inst.MetadataOptions.HttpEndpoint != ec2types.InstanceMetadataEndpointStateDisabled`.

## 6. P3: rows fetched by ID have no status-check fields, so Health is blank and the state marker is missing

- **Files:** `core/aws/ec2_by_ids.go:107` (`fetchEC2InstancesByIDsOnce` builds rows with `ec2InstanceToResource` only). The consumers are `core/aws/catalog_compute.go:250` (Health column `instance_status`), `:259-272` (the `!`/`~` state decorator) and `:136-148` (the Status Checks detail section).
- **Code:** `system_status` and `instance_status` are set only by `enrichEC2StatusChecks`, and only the paged fetcher calls it (`core/aws/ec2.go:41-43`). The by-ID path never does, although the comment at `ec2.go:77-81` says both paths produce the same shape.
- **Trigger:** an instance reached through the related-panel lazy-add or through the Cost Explorer RESOURCE_ID drill, which goes through `FetchByIDs`.
- **User impact:** for that row the Health column is empty, a running impaired or initializing instance shows no `!`/`~` marker, and the detail view has no Status Checks section. The same instance looks different depending on how the operator reached it.
- **Fix direction:** call `enrichEC2StatusChecks` from `FetchEC2InstancesByIDs`, or move it into a shared builder that both paths use. This means the by-IDs API interface must also include `DescribeInstanceStatus`.

## 7. P3: the ec2 → ebs-snap pivot leaves out the AMI's root snapshots

- **File:** `core/aws/ec2_related.go:254-279` (`checkEC2EBSSnap`)
- **Code:** it matches only snapshots whose `VolumeId` is one of the instance's attached volumes. The `ImageId` → AMI → `BlockDeviceMappings[].Ebs.SnapshotId` path is never followed.
- **Trigger:** an instance launched from an AMI whose backing snapshots are in the loaded `ebs-snap` list. These snapshots come from the AMI's source volume, not from the instance's volumes.
- **User impact:** the rollback snapshots are left out of the count and the list, and the count reads 0 when no volume snapshots exist. The spec (§2 `ebs-snap`) says "the pivot must union both sources".
- **Fix direction:** also resolve `Instance.ImageId` against the loaded `ami` cache (an `ec2types.Image` RawStruct), collect its `BlockDeviceMappings[].Ebs.SnapshotId`, and union those IDs with the volume-sourced matches.

## ecr

# ecr — production-code review

Scope: `core/aws/ecr.go`, `ecr_codes.go`, `ecr_interfaces.go`, `ecr_issue_enrichment.go`, `ecr_related.go`, `ecr_related_extra.go`, `ecr_images.go`, `ecr_images_codes.go`, the `ecr` entry in `core/aws/catalog_cicd.go`, the `ecr_images` child entry in `core/aws/catalog_containers.go`, and the direct helpers they call. Intended behavior is taken from `docs/resources/ecr.md`.

8 findings (P0:0 P1:1 P2:4 P3:3)

---

## 1. P1: Wave 2 vulnerability counts come from a field that current basic scanning leaves empty

- **File:** `core/aws/ecr_issue_enrichment.go:142-156` (and the same dependency in `core/aws/ecr_images.go:136-139,186-196`)
- **What happens:** `EnrichECRRepository` reads CRITICAL and HIGH counts only from `DescribeImages` → `ImageDetails[].ImageScanFindingsSummary.FindingSeverityCounts`. The AWS `DescribeImages` API reference says: "The new version of Amazon ECR *Basic Scanning* doesn't use the ImageDetail:imageScanFindingsSummary and ImageDetail:imageScanStatus attributes from the API response to return scan results. Use the DescribeImageScanFindings API instead." `ECRDescribeImageScanFindingsAPI` is declared and embedded in `ECRAPI` (`ecr_interfaces.go:43-47,75`), but no production code calls it.
- **Trigger:** A registry that uses the current AWS-native basic scanning has an image with critical CVEs.
- **User impact:** A nil summary is skipped silently (`continue`), so the repository gets no `ecr.vulnerabilities` or `ecr.vulnerabilities-high` finding. It also gets `critical_vulns=0`, `high_vulns=0`, which the Critical and High columns show as a real zero. The row looks clean even though the image is vulnerable. In the `ecr_images` child view, `finding_counts`, the critical and high findings, and the `failed` status stay blank for the same reason.
- **Fix direction:** For the image the finding is about (see #2), fall back to `DescribeImageScanFindings` (its `ImageScanFindings.FindingSeverityCounts`) when `ImageScanFindingsSummary` is nil. Also write `critical_vulns` and `high_vulns` only when a scan result was actually read. Otherwise leave them unset, so the columns do not claim zero.

## 2. P2: Vulnerability finding sums up to 10 arbitrary images instead of using the latest pushed image

- **File:** `core/aws/ecr_issue_enrichment.go:36-43,99-104,140-185`
- **What happens:** The call is `DescribeImages{MaxResults: 10}` with no sort. The comment says "AWS returns the most recent images by default", but the `DescribeImages` API reference makes no ordering guarantee. The code then adds CRITICAL and HIGH counts across every image in that page. The spec (§3.2) asks for the single latest image by `imagePushedAt`, sorted client-side. The repo's own snapshot oracle does exactly that (`cmd/snapshot/compute.go:433-468`, a full paginated walk and then a max over `ImagePushedAt`). So the same fact is computed two different ways.
- **Trigger:** (a) A repository has more than 10 images and the newest push is not in the first page, or (b) several old or superseded images share the same CVEs.
- **User impact:** (a) A critical in the image that is actually deployed is missed, and the row shows healthy. (b) The phrase `<N> critical, <M> high vulnerabilities` counts the same CVE once per image, so N and M are inflated. The row can also go red only because of long-superseded images the current image already fixed.
- **Fix direction:** Paginate `DescribeImages` (or at least read enough pages to find the maximum `ImagePushedAt`), pick the latest image, and take its counts only. Share that selection with the snapshot capture so the two cannot drift.

## 3. P2: ecr → lambda lists every container-image Lambda under every repository

- **File:** `core/aws/ecr_related.go:22-53`
- **What happens:** `checkECRLambda` adds any function with `PackageType == Image` and never compares it with this repository. The `repoURI` it reads is only checked for being empty. The spec (§2 `lambda`) requires matching `Code.ImageUri` against `<RegistryId>.dkr.ecr.<region>.amazonaws.com/<RepositoryName>`. The reverse checker `checkLambdaECR` (`core/aws/lambda_related.go:223-`) already resolves the exact repository through `GetFunction` → `Code.ImageUri`, so the two directions disagree.
- **Trigger:** The account has several image-based Lambdas built from different repositories. The user opens any ECR repository.
- **User impact:** The Lambda count and the drill-down include functions that have nothing to do with this repository, so the "who runs this image?" answer is wrong.
- **Fix direction:** Resolve each image function's `Code.ImageUri` (GetFunction, bounded) and keep only those whose parsed repository and registry match this repo. Use one shared ECR image-URI parser for both directions (see #4).

## 4. P2: ecr → ecs-task matches by substring, pulling in other repositories, accounts and regions

- **File:** `core/aws/ecr_related_extra.go:57-69`
- **What happens:** An image counts as a match when `strings.Contains(image, ".dkr.ecr.") && strings.Contains(image, "/"+repoName)`. Repository `app` therefore matches `…/app-worker:1`, `…/team/app:1`, `…/app/sidecar:1`, and images from any other account or region that has a repository with that name. The reverse checker `checkECSTaskECR` (`core/aws/ecs_task_related_extra.go:104-141`) parses the repository path exactly, so the two directions disagree.
- **Trigger:** Repositories whose names are prefixes or path segments of each other (`app` and `app-worker`), or namespaced repositories (`team/app` and `app`).
- **User impact:** The ECS Tasks count and list for a repository include tasks running a different image. That misleads the operator during incident triage.
- **Fix direction:** Parse `host/path[:tag|@digest]`. Compare the host with `<RegistryId>.dkr.ecr.<region>.amazonaws.com` and the path with `RepositoryName` exactly (for example, take the prefix from `Fields["uri"]` and require the next character to be `:`, `@` or the end of the string). Use this one parser here and in `checkECSTaskECR` and `checkLambdaECR`.

## 5. P2: ecr → cb uses a prefix substring and never reads environment variables

- **File:** `core/aws/ecr_related.go:57-82`
- **What happens:** A project matches only when `strings.Contains(*raw.Environment.Image, repoURI)` is true. Repository URI `…/app` is a substring of `…/app-worker:latest`, so it is a false match. The spec (§2 `cb`) also requires matching `Environment.EnvironmentVariables[].Value` against the repository URI or name. That code is absent. Projects that push to ECR usually reference the target repository through environment variables (for example `IMAGE_REPO_NAME`), while `Environment.Image` is the build image (for example `aws/codebuild/standard:7.0`).
- **Trigger:** (a) Repositories whose URIs share a prefix. (b) A CodeBuild project that builds and pushes to this repository with a stock build image.
- **User impact:** (a) Unrelated projects are counted. (b) The main pivot the spec describes, "the build that produced this image", shows 0.
- **Fix direction:** Match `Environment.Image` with the exact URI parser from #4. Also scan `EnvironmentVariables[].Value` for an exact match on the repository URI or name, as the spec says.

## 6. P3: ecr → ct-events matches any event whose resource name contains the repository name

- **File:** `core/aws/ecr_related_extra.go:19-44`
- **What happens:** `strings.Contains(*r.ResourceName, repoName)` counts events for `app-worker`, `myapp`, a Lambda called `app-api`, and so on under repository `app`. The sibling `checkLambdaCTEvents` (`core/aws/lambda_related_extra.go:292-297`) uses an exact name match or an exact ARN-suffix match.
- **Trigger:** A repository whose name is a substring of other resource names in the region's recent CloudTrail events.
- **User impact:** The CloudTrail Events count and list include activity on unrelated resources, which confuses the audit trail.
- **Fix direction:** Match `ResourceName == repoName` or `== RepositoryArn` (or `HasSuffix(":repository/"+repoName)`), the way the Lambda checker does.

## 7. P3: ecr → eb-rule `resources` filter matches repository ARNs by substring

- **File:** `core/aws/ecr_related.go:245-255`
- **What happens:** `strings.Contains(r, repoARN)` makes a rule filtering on `arn:…:repository/app-worker` match repository `app` (`arn:…:repository/app`).
- **Trigger:** An EventBridge rule with an `aws.ecr` source and a `resources` filter on a repository whose ARN has this repository's ARN as a prefix.
- **User impact:** The EventBridge Rules count includes a rule that never fires for this repository.
- **Fix direction:** Compare with `r == repoARN`. If wildcard or prefix patterns need support, handle them as EventBridge content filters, not as a raw substring.

## 8. P3: ecr_images is newest-first within each page only

- **File:** `core/aws/ecr_images.go:19-23,53-62`
- **What happens:** `FetchECRImages` sorts each `DescribeImages` page (up to 100 images, no `MaxResults`) by `ImagePushedAt`, but pages come back in the API's own unspecified order and are appended as they arrive. The doc comment promises "sorted by push time (newest first)".
- **Trigger:** A repository has more than 100 images and the user opens its image list.
- **User impact:** The first screen can show old images while the most recently pushed image sits on a later page behind "load more". The operator can easily miss the image that is actually current.
- **Fix direction:** Either load all pages before sorting (image lists are bounded by lifecycle policies), or change the comment and the UI claim to say the order holds per page only.

## ecs-svc

# ecs-svc — production-code review

Scope: `core/aws/ecs_svc*.go`, `core/aws/catalog_compute.go` (ecs-svc entry, `colorECSSvc`), `core/aws/catalog_containers.go` (ecs-svc child views), inbound pivots (`ecs_related.go`, `ecs_task_related.go`, `tg_related.go`, `eip_related.go`, `pipeline_related.go`), plus the runtime/app code needed to verify identity and navigation (`core/resource/resource.go`, `core/runtime/helpers.go`, `core/runtime/handlers_related.go`, `core/app/list_filter.go`).

AWS semantics were checked against the current API reference for ListServices, DescribeServices and FilterLogEvents.

Totals: 14 findings (P0:0 P1:3 P2:8 P3:3)

---

## P1-1 — ListServices is never paginated, so a cluster's services past the first 10 are dropped

- **File/line**: `core/aws/ecs_svc.go:40-45` (single `ListServices` call, no `NextToken` loop, no `MaxResults`)
- **Trigger**: any cluster with more than 10 services. Per the API reference, with no `maxResults`, "ListServices returns up to 10 results and a nextToken value if applicable". The fetcher reads `svcListOutput.ServiceArns` once and never looks at `svcListOutput.NextToken`.
- **User impact**: the ecs-svc list silently leaves out every service after the 10th in each cluster. Pagination reports `IsTruncated` only for `ListClusters`, so nothing tells the operator that rows are missing. Issue counts, the cluster→services pivot (`checkECSServices`), tg→ecs-svc, and the task/eip→ecs-svc drills (which open no row) all under-report the same way. The snapshot collector (`cmd/snapshot/compute.go:180`, `listAllServiceArns`) does paginate, so the app and the collector disagree.
- **Fix**: loop `ListServices` until `NextToken` is nil (`MaxResults: 100` is fine). Then call `DescribeServices` in batches of 10 ARNs, because the API caps `services` at 10 per call. Wrap both calls in `RetryOnThrottle`.

## P1-2 — The row ID is the bare service name, which is unique only within one cluster

- **File/line**: `core/aws/ecs_svc.go:100` (`ID: serviceName`). The ID is used at `core/resource/resource.go:25` (`DedupByID`), `core/runtime/helpers.go:135` (`findings[r.ID]`), `core/app/list_filter.go:26` (`RelatedIDSet[r.ID]`), `core/aws/ecs_svc_issue_enrichment.go:95` (`resourceByService[svcName]`) and `:209`/`:227` (`setWave2Finding(&result, svcName, …)`).
- **Trigger**: the same service name in two clusters, for example `api` in both `prod` and `staging`. This is the normal pattern for per-environment clusters.
- **User impact**:
  - Wave-2 findings are keyed by ID. A failed deployment or public-IP finding on `prod/api` also shows on `staging/api`, and `staging/api`'s own findings also show on `prod/api`.
  - When clusters span `ListClusters` pages, `DedupByID` drops the second cluster's `api` row entirely during page append (in both the RowStore and the list).
  - Navigating from the `prod` cluster to ecs-svc filters by `RelatedIDSet`, so it also shows `staging/api`.
  - ecs-task → ecs-svc and eip → ecs-svc return only the bare name. A single-ID cache hit (`handlers_related.go:375`) opens whichever `api` comes first, possibly from the wrong cluster.
- **Fix**: make the ID cluster-qualified. Use `ServiceArn`, or `<cluster>/<service>`, and keep `Name: serviceName`. Update every producer of ecs-svc IDs to match: `checkECSTaskService`, `checkEIPECSSvc` (both have the task's `ClusterArn`), `checkPipelineECSSvc` (`ClusterName` is in the action configuration), and the enricher's `resourceByService` and `setWave2Finding` keys. Checkers that read `res.ID` as the service name (alarms, ct-events, eb-rule, tasks) must switch to `Fields["service_name"]`.

## P1-3 — The service log view (`L`) shows the oldest events in the log group, not the recent ones

- **File/line**: `core/aws/ecs_svc_logs.go:81-84` (`FilterLogEventsInput` has no `StartTime` and no `StartFromHead: false`); docstring at `:28-32` says "retrieve recent log lines".
- **Trigger**: open `L` on any service whose log group holds more than about 200 events, which is any service that has run for a while. The FilterLogEvents reference says "By default, the events are returned in ascending timestamp order (oldest first)". The fetcher stops at `maxLogEvents` (200) or `maxLogScanPages`.
- **User impact**: during an incident the operator sees the first 200 lines of the group's retention, which can be weeks or months old, instead of the current failure. Recent lines are only reachable by paging through the entire history. On a large group, the 100-page scan cap is reached before anything recent appears.
- **Fix**: request newest-first. Set `StartFromHead: aws.Bool(false)` with a `StartTime` on or after 2024-01-01, because the API requires that for descending order; a recent window such as the last 24h also avoids empty pages. Alternatively, bound `StartTime` to a recent window. Pass `Limit` so a single page cannot exceed the 200 cap.

---

## P2-1 — DescribeServices is called without `include=TAGS`, so the CFN pivot always reports zero and the detail view shows no tags

- **File/line**: `core/aws/ecs_svc.go:51-54` (no `Include: []ecstypes.ServiceField{ecstypes.ServiceFieldTags}`). Consumers: `core/aws/ecs_svc_related.go:103-114` (`checkECSSvcCFN` reads `raw.Tags`) and `core/config/defaults_compute.go:34` (detail field `Tags`).
- **Trigger**: any service. The DescribeServices reference says: "If this field is omitted, tags aren't included in the response."
- **User impact**: `raw.Tags` is always empty. `stackName` stays empty, and `unreadZero` returns a resolved `0` because RawStruct is non-nil. The panel therefore says "CloudFormation Stacks (0)" for stack-managed services, which is a false negative stated as fact. The detail view's Tags field is always blank.
- **Fix**: add `Include: []ecstypes.ServiceField{ecstypes.ServiceFieldTags}` to the fetcher's DescribeServices call. The enricher's call needs it too if it replaces RawStruct.

## P2-2 — The `L` log view is not scoped to the service's streams and reads only the first awslogs container

- **File/line**: `core/aws/ecs_svc_logs.go:56-66` (first awslogs container only; `awslogs-stream-prefix` is never read, although the docstring at `:29` says it is) and `:81-84` (no `LogStreamNamePrefix`).
- **Trigger**: several services or task families share one log group (for example `/ecs/prod`), or the service has more than one container, such as an app plus a sidecar that logs to another group.
- **User impact**: the "service logs" view mixes in lines from every other service in the shared group. When the first awslogs container is a sidecar (for example a log router or an envoy proxy), the application container's group is never shown.
- **Fix**: set `LogStreamNamePrefix` to `awslogs-stream-prefix + "/" + containerName` when the prefix is set. Collect every awslogs container, not just the first.

## P2-3 — The secrets pivot returns raw `ValueFrom` ARNs as IDs, but secrets rows are keyed by name

- **File/line**: `core/aws/ecs_svc_related_extra.go:339-364` (IDs are the raw `ValueFrom` / `CredentialsParameter` strings). Target ID is the name: `core/aws/secrets.go:71` (`ID: secretName`). Filter by exact ID: `core/app/list_filter.go:26`.
- **Trigger**: any service whose task definition injects a Secrets Manager secret. This is especially visible with the JSON-key form `arn:…:secret:db-AbCdEf:password::`.
- **User impact**: the panel shows "Secrets (N)", but Enter opens a filtered list with zero rows, because no secrets row has an ARN as its ID. With JSON-key references, one secret used for `username` and `password` counts as 2. The sibling `checkECSTaskSecrets` (`ecs_task_related_extra.go:195-205`) cross-references against the secrets list and matches on name or `Fields["arn"]`, so the two pivots compute the same fact two different ways.
- **Fix**: normalize each `ValueFrom` to the base secret ARN by stripping the `:json-key:version-stage:version-id` suffix. Then cross-reference the loaded `secrets` list by `Fields["arn"]` and return `sRes.ID`, the way the ecs-task checker does. Better still, share one helper between the two checkers.

## P2-4 — The ecs-task pivot matches on the service name only and ignores the cluster

- **File/line**: `core/aws/ecs_svc_related_extra.go:71` (`*task.Group == "service:"+svcName`, with `svcName := res.ID` at `:54`)
- **Trigger**: the same service name in two clusters. Task `Group` holds only `service:<name>`.
- **User impact**: the "ECS Tasks" count and drill for `prod/api` include `staging/api`'s tasks. An operator can then read the wrong environment's stopped reasons.
- **Fix**: also require `task.ClusterArn` to equal the service's `ClusterArn` from RawStruct, or its last segment to equal `Fields["cluster"]`.

## P2-5 — The CloudTrail events pivot uses substring matching on the service name

- **File/line**: `core/aws/ecs_svc_related_extra.go:42` (`strings.Contains(*r.ResourceName, svcName)`)
- **Trigger**: a short or common service name such as `api`, `web` or `worker`. The ct-events cache covers every event source in the region.
- **User impact**: the count and drill include unrelated events, for example on `api-gateway-*`, `web-assets` buckets, `worker-queue` SQS queues, or other services whose name contains this one. The audit trail for the service is unreliable.
- **Fix**: match exact identity. Compare `ResourceName` with the service ARN, or with the ARN's `service/<cluster>/<name>` resource part, or the exact name when the event source is `ecs.amazonaws.com`. Do not use a substring.

## P2-6 — The logs pivot guesses by task family substring instead of reading `awslogs-group`

- **File/line**: `core/aws/ecs_svc_related.go:254-276` (`strings.Contains(logRes.ID, family)`)
- **Trigger**: a log group named without the family (for example `/ecs/prod`, `/aws/ecs/containerinsights/...`), or a short family name that is a substring of other groups (`api` matches `/aws/lambda/rapid-api`, `/aws/apigateway/...`).
- **User impact**: the pivot misses the group the service actually writes to, or lists unrelated groups. It also disagrees with the `L` view (`ecs_svc_logs.go:63`), which reads `Options["awslogs-group"]` from `DescribeTaskDefinition` as the spec (§2 `logs`) requires.
- **Fix**: resolve the task definition once, as `checkECSSvcECR` and `checkECSSvcSecrets` already do. Collect `awslogs-group` from every awslogs container and cross-reference the logs list by exact name. Ideally share one task-definition resolution across the ecr, logs, role, secrets and `L` code paths.

## P2-7 — The role pivot shows only `Service.RoleArn` and omits the task role and execution role

- **File/line**: `core/aws/ecs_svc_related.go:304-316`
- **Trigger**: any service, and Fargate/awsvpc services in particular. For those, `RoleArn` is absent or is the ECS service-linked role.
- **User impact**: spec §2 `role` requires `TaskDefinition.ExecutionRoleArn` and `TaskRoleArn` as well. The operator cannot pivot to the roles that gate image pulls, secret retrieval and in-container AWS access, which are the ones that matter for troubleshooting. The panel shows 0 or only `AWSServiceRoleForECS`.
- **Fix**: add the task definition's `ExecutionRoleArn` and `TaskRoleArn`, taking the last path segment as the role name, to the `RoleArn`-derived ID.

## P2-8 — The EventBridge pivot matches event patterns instead of rule targets, and broad rules match every service

- **File/line**: `core/aws/ecs_svc_related_extra.go:163-167` (rules with no `EventPattern` are skipped), `:223-228` (source `aws.ecs` with no detail filter returns true), `:215` (`strings.Contains(c, clusterName)`)
- **Trigger**:
  - (a) A scheduled-task rule, which has a `ScheduleExpression`, no `EventPattern`, and an ECS target. The code never matches it.
  - (b) Any rule with `{"source":["aws.ecs"]}` and no detail filter, for example an ECS task-state-change notifier. It matches every service in the account.
  - (c) A cluster name that is a substring of another (`prod` vs `prod-eu`).
- **User impact**: spec §2 `eb-rule` defines the relationship as "rules that target this ECS service" via `ListTargetsByRule` / `EcsParameters`. The panel misses exactly the cron-on-ECS rules operators look for, and reports unrelated notifier rules on every service.
- **Fix**: match on targets (`Target.Arn` equal to the cluster ARN, plus `EcsParameters.TaskDefinitionArn` family or group matching) as the spec defines. Drop the broad-match fallback. Compare the cluster ARN or name exactly.

---

## P3-1 — The ECR pivot counts repositories that are not in the account/region list

- **File/line**: `core/aws/ecs_svc_related_extra.go:272-297`
- **Trigger**: images pulled from a shared-services account or another region, such as `<other-acct>.dkr.ecr.<region>.amazonaws.com/base`. This setup is common.
- **User impact**: the pivot shows "ECR Repositories (1)", but Enter lands on an empty filtered list, because the repo is not in this account's ECR list (`ecr.go:56`, ID = repo name). Also, two repos with the same name in different registries collapse into one ID.
- **Fix**: parse the account and region from the image URI and keep only same-account, same-region repositories. Alternatively, cross-reference the loaded `ecr` list as the spec says.

## P3-2 — The Step Functions pivot matches the task family by substring and ignores JSONata `Arguments`

- **File/line**: `core/aws/ecs_svc_related_extra.go:459-472`
- **Trigger**: family `api` against a state machine running `api-worker` or `payments-api`. Separately, a JSONata state machine, which puts `TaskDefinition` under `Arguments` rather than `Parameters`.
- **User impact**: false-positive state machines in the panel for short family names. JSONata-based orchestrations of the service are never found.
- **Fix**: parse the task-definition reference (an ARN or `family[:rev]`) and compare the family exactly. Read `Arguments` as well as `Parameters`.

## P3-3 — The Wave-2 enricher re-issues DescribeServices for data the list load already holds

- **File/line**: `core/aws/ecs_svc_issue_enrichment.go:111-116`
- **Trigger**: every list load of ecs-svc. `RawStruct` from `FetchECSServicesPage` already carries `Deployments`, `Events`, `NetworkConfiguration`, `DesiredCount` and `RunningCount`.
- **User impact**: DescribeServices traffic doubles, which adds throttling pressure on large accounts, and `?` coverage gaps appear when the second call fails, although the first call already answered. Spec §3.2 states the Wave-2 signals need "none beyond list-load".
- **Fix**: evaluate the Wave-2 signals from `assertStruct[ecstypes.Service](r.RawStruct)`. Call DescribeServices only for rows without RawStruct, such as cache-restored rows.

## ecs-task

# ecs-task — production code review

Scope: `core/aws/ecs_task.go`, `ecs_task_codes.go`, `ecs_task_issue_enrichment.go`, `ecs_task_related.go`, `ecs_task_related_extra.go`, the `ecs-task` catalog entry in `catalog_compute.go`, and the reverse-scan checkers that target `ecs-task` (`ecs_svc_related_extra.go`, `secrets_related_extra.go`, `ecr_related_extra.go`, `efs_related.go`, `logs_related.go`, `eip_related.go`, `ecs_related_extra.go`).

12 findings: P0:0 P1:2 P2:8 P3:2

---

## 1. P1: ListTasks reads only the first page for each cluster, so tasks past 100 disappear without any warning

- **File:** `core/aws/ecs_task.go:49`
- **Trigger:** A cluster has more than 100 tasks. `ListTasks` returns at most 100 ARNs by default and sets `NextToken`. The fetcher sends a single `ListTasksInput{Cluster}`, never reads `taskListOutput.NextToken`, and sets `IsTruncated` only from `ListClusters.NextToken`.
- **User impact:** The ecs-task list, the badge and issue counts, and every reverse scan over the `ecs-task` cache (ecs→tasks, ecs-svc→tasks, efs, secrets, logs, ecr, eip) quietly leave out every task after the first 100 in that cluster. Pagination still reports the list as complete, so nothing says tasks are missing and "m: load more" never appears.
- **Fix:** Loop `ListTasks` on `NextToken` for each cluster, or carry the per-cluster task cursor in the continuation token. Send the ARNs to `DescribeTasks` in batches of 100.

## 2. P1: The ec2 pivot returns the container-instance UUID as an EC2 instance ID

- **File:** `core/aws/ecs_task_related_extra.go:99` (`checkECSTaskEC2`)
- **Trigger:** Any EC2-launch-type task with `ContainerInstanceArn` set. The checker takes the last ARN segment, which is the container-instance UUID, and returns it as an `ec2` ID. The ec2 resource ID is the instance ID `i-…` (`core/aws/ec2.go:139`). The code's own comment says the instance ID is not in this ARN.
- **User impact:** The related panel shows "EC2 Instances: 1", but the ID can never match an EC2 row, so the pivot opens nothing. The spec requires the "which host is this task on" answer (§2 `ec2`), and it is never delivered.
- **Fix:** Resolve `Ec2InstanceId` with `ecs:DescribeContainerInstances(cluster, [ContainerInstanceArn])`, as the spec prescribes. Alternatively, join once in the fetcher and store the instance ID in a field. Return the `i-…` ID and cross-reference it against the ec2 cache.

## 3. P2: The top-level list never includes stopped or stopping tasks, so the main failure signals cannot fire

- **File:** `core/aws/ecs_task.go:49`
- **Trigger:** `ListTasksInput` has no `DesiredStatus`. The API then defaults to `RUNNING`, so it returns no task whose desired status is `STOPPED`. That excludes every task in `STOPPING`, `DEPROVISIONING` or `STOPPED`. By contrast, `FetchEcsSvcTasks` (`ecs_svc_tasks.go`) lists `RUNNING` and `STOPPED` explicitly.
- **User impact:** In the ecs-task list, the spec §3.1/§4 signals `stopped: <reason>` (EssentialContainerExited or TaskFailedToStart), `stopped (task exited)`, `stopping` and `deprovisioning` can never appear. The same applies to the Wave 2 `StopCode` rule. A task that crashed and was replaced leaves no red row, which removes the main "why did my task die" signal.
- **Fix:** Also list `DesiredStatus=STOPPED` for each cluster, which covers the roughly one-hour retention window, and merge the results as `FetchEcsSvcTasks` does.

## 4. P2: "task failed" fires on any non-zero container exit, even for non-essential containers in running tasks

- **File:** `core/aws/ecs_task_issue_enrichment.go:143`
- **Trigger:** A running task has a non-essential sidecar or helper container that exited with a non-zero code. Every row reaching the enricher comes from a `RUNNING` desired-status list (finding 3). The loop marks any `Containers[].ExitCode != 0` as broken without checking `essential` on the task definition or whether the task has stopped. The spec §4 row says "`ExitCode != 0` on an essential container of a stopped task".
- **User impact:** A healthy, serving task shows a red "task failed" row with the detail "This task ended in a failure…". That is a false broken count on the badge and the main menu.
- **Fix:** Apply the rule only when `LastStatus == STOPPED`, and only to containers whose task-definition `ContainerDefinitions[].Essential` is true. The definition is already fetched in `ecsTaskDefinitionPosture`, so reuse it there.

## 5. P2: Secrets Manager references that name a JSON key or a version are stored whole, so the secrets pivot misses them

- **Files:** `core/aws/ecs_task.go:291-292`. The same class of defect is in the reverse scan at `core/aws/secrets_related_extra.go:251`.
- **Trigger:** A container secret uses the documented forms `arn:aws:secretsmanager:region:acct:secret:name-AbCdEf:json-key:version-stage:version-id`, for example `…:name-AbCdEf:username::`. `isSecret(v)` accepts the string, and the full `ValueFrom` goes into `secret_arns`. `checkECSTaskSecrets` compares it for equality with the secret's `ID` (its name) or `Fields["arn"]`, so it never matches. `secretsECSTaskRefsSecret` also compares `*s.ValueFrom == secretARN`.
- **User impact:** ecs-task → Secrets shows 0 for secrets that are actually injected. secrets → ECS Tasks also misses those tasks. Per-key secret injection is common.
- **Fix:** Normalise `ValueFrom` to the secret ARN before storing or comparing it: parse the ARN and keep `secret:<name-suffix>`, dropping the trailing `:json-key:stage:id` segments.

## 6. P2: SSM parameter names are built wrongly for flat (non-hierarchical) parameters

- **File:** `core/aws/ecs_task.go:299-305`
- **Trigger:**
  - (a) `ValueFrom` is an SSM ARN for a flat parameter such as `arn:…:parameter/db_password`, whose real name is `db_password`. The code always adds a leading `/` and stores `/db_password`, while the ssm cache ID is the real name (`core/aws/ssm.go:79`).
  - (b) `ValueFrom` is a bare parameter name without a leading `/`, such as `db_password`. AWS allows this for same-region parameters, but the code accepts bare names only when they start with `/`, so this one is dropped.
- **User impact:** ecs-task → SSM Parameters reports a definite 0 for parameters the task actually reads.
- **Fix:** Do not add `/` unconditionally. Match the ARN resource against both `name` and `/name`, or resolve the name from the cache. Treat any `ValueFrom` that is not an ARN as an SSM name, whether or not it starts with `/`.

## 7. P2: When the task-definition lookup fails, the role, secrets and ssm pivots report a definite 0 instead of unknown or truncated

- **Files:** `core/aws/ecs_task_related.go:115-124` (role), `core/aws/ecs_task_related_extra.go:172-175` (secrets), `core/aws/ecs_task_related_extra.go:215-218` (ssm). The contributing lookup cache is at `core/aws/ecs_task.go:222-243`.
- **Trigger:** `DescribeTaskDefinition` fails for a task with AccessDenied, throttling or a transient error. The fetcher sets `Fields["task_def_join_error"]="true"` and leaves `task_role`, `execution_role`, `secret_arns` and `ssm_param_names` empty. All three checkers read only the empty field and return `KnownRelated(nil,false)`. Only `checkEFSECSTask` checks the flag. In addition, the failure is cached as `seenTaskDefs[arn]=nil`, so every later task on the same definition in that page gets `(zero, nil)` and no error flag at all.
- **User impact:** Without the ECS permission, the panel says the task has no IAM role, no secrets and no parameters. That is a confident but false answer in exactly the "AccessDenied → which role?" situation the pivot exists for.
- **Fix:** Cache the error together with the nil definition so that every task on that definition gets `task_def_join_error`. In the role, secrets and ssm checkers, return `UnknownRelated` or a truncated result when the flag is set.

## 8. P2: The logs pivot matches log groups by substring of the task family instead of the definition's `awslogs-group`

- **File:** `core/aws/ecs_task_related.go:97`
- **Trigger:** `strings.Contains(logRes.ID, family)`. A family such as `api` matches `/aws/lambda/api-gateway-authorizer`, `/ecs/api-v2` and so on. The task's real `awslogs-group`, for example `/prod/web`, is missed when its name does not contain the family. The spec §2 `logs` requires `ContainerDefinitions[].LogConfiguration.Options["awslogs-group"]` from `DescribeTaskDefinition`.
- **User impact:** The "tail the log group" pivot shows unrelated groups and can omit the group the containers actually write to.
- **Fix:** Collect the `awslogs-group` values in `ecsJoinTaskDefinition`, which already describes the definition, store them in a field, and match log-group names exactly.

## 9. P2: The alarm pivot matches TaskId/TaskArn dimensions instead of the spec's ClusterName/ServiceName

- **File:** `core/aws/ecs_task_related_extra.go:42`
- **Trigger:** An alarm on `AWS/ECS` or Container Insights metrics with dimensions `ClusterName` and `ServiceName`, which is the standard way ECS alarms are scoped. The checker accepts only `TaskId` or `TaskArn` dimensions. Those exist only on Container Insights enhanced-observability task metrics.
- **User impact:** "CloudWatch Alarms" is almost always 0 for tasks, even when the service's CPU or memory alarm is firing. The spec §2 `alarm` pivot ("is on-call already paged?") is not delivered.
- **Fix:** Match `ClusterName` equal to the last segment of `Task.ClusterArn`. When `Task.Group` starts with `service:`, also require `ServiceName` equal to the stripped group, as the spec describes. Keeping the TaskId match as an extra path is fine.

## 10. P2: The ecs-svc → ECS Tasks reverse scan ignores the cluster, so tasks of a same-named service in another cluster are counted

- **File:** `core/aws/ecs_svc_related_extra.go:71` (`checkECSSvcTasks`)
- **Trigger:** Two clusters, for example `staging` and `prod`, each run a service named `api`. The scan matches only `task.Group == "service:api"` and never compares `task.ClusterArn` with the service's `Fields["cluster"]`.
- **User impact:** The prod service's related panel counts and lists the staging tasks as well. An operator can end up investigating the wrong environment's tasks.
- **Fix:** Also require the last segment of `task.ClusterArn` to equal `res.Fields["cluster"]`, which is the service's cluster name.

## 11. P3: ECR pivots in both directions accept images from other registries and partial repository names

- **Files:** `core/aws/ecs_task_related_extra.go:115-140` (`checkECSTaskECR`) and `core/aws/ecr_related_extra.go:65` (`checkECRECSTask`)
- **Trigger:**
  - (a) A task image from another account's or region's ECR registry, such as a shared-services account. The forward checker keeps any image containing `.dkr.ecr.` and returns the repository path without checking the registry's account or region, or whether the repository is in the ecr cache.
  - (b) The reverse checker uses `strings.Contains(image, "/"+repoName)`, so repository `api` also matches `…/api-gateway:tag`.
- **User impact:** (a) "ECR Repositories: 1" is shown for a repository that does not exist in this account or region, and the pivot opens nothing. (b) ecr → ECS Tasks lists tasks that run a different repository.
- **Fix:** Parse the registry host into account and region and compare them with the current session. Extract the repository exactly (path up to `:` or `@`) and compare for equality in both directions.

## 12. P3: The Cluster column shows the start of the cluster ARN, which is identical on every row

- **File:** `core/aws/catalog_compute.go:502`
- **Trigger:** The column `{Key: "cluster", Path: "ClusterArn", Width: 24}`. Key-first rendering (`core/app/list_columns.go:223`) prints `Fields["cluster"]`, which holds the full ARN (`ecs_task.go:128`). In a 24-character cell that reads `arn:aws:ecs:us-east-1:12…` for every task.
- **User impact:** The cluster name is never visible in the list, so operators cannot tell which cluster a task belongs to without opening the detail view.
- **Fix:** Store the cluster name (last ARN segment) in a separate display field for this column, and keep the ARN in `cluster` for the enricher and the checkers.

## ecs

# ecs — production code review

Scope: `core/aws/ecs.go`, `core/aws/ecs_issue_enrichment.go`, `core/aws/ecs_related.go`, `core/aws/ecs_related_extra.go`, the `ecs` catalog entry in `core/aws/catalog_compute.go`, `colorECSCluster`, and the pivots that navigate to `ecs` (`ecs_svc_related.go`, `ecs_task_related.go`, `alarm_related_extra.go`, `eip_related.go`, `core/resource/related.go` nav extractor), plus `cmd/snapshot/compute.go` `captureECS`.

8 findings (P0:0 P1:1 P2:4 P3:3)

## 1. [P1] The cluster list never asks for tags or configuration, so the CloudFormation and KMS pivots always show zero

- **File/line**: `core/aws/ecs.go:39-41` (`DescribeClustersInput` has no `Include`); consumers `core/aws/ecs_related.go:60-98` (`checkECSCFN`), `core/aws/ecs_related.go:100-113` (`checkECSKMS`), navigable field `core/aws/catalog_compute.go:475`.
- **Trigger**: any cluster created by a CloudFormation stack, or any cluster with `ecs exec` configured with a KMS key. The AWS DescribeClusters reference says `include` controls extra data: "If `CONFIGURATIONS` is specified, the configuration for the cluster is included... If `TAGS` is specified, the metadata tags associated with the cluster are included. If this field is omitted, this information isn't included." No other production code re-describes ECS clusters with `TAGS` or `CONFIGURATIONS` (the Wave 2 enricher asks only for `STATISTICS` and does not replace `RawStruct`).
- **Impact**: `RawStruct.Tags` and `RawStruct.Configuration` are always nil. Because `RawStruct` is non-nil, `unreadZero` does not turn the result into Unknown. The panel reports a confident "CloudFormation Stacks 0" and "KMS Key 0" on every cluster. The navigable `Configuration.ExecuteCommandConfiguration.KmsKeyId` field never appears in detail. The operator cannot reach the owning stack or the exec key, and is told that neither exists.
- **Fix direction**: pass `Include: []ecstypes.ClusterField{ClusterFieldTags, ClusterFieldConfigurations}` (plus `SETTINGS` if detail should show Container Insights) on the list-page DescribeClusters.

## 2. [P2] The ASG pivot lists every ECS-managed Auto Scaling group in the region under every cluster

- **File/line**: `core/aws/ecs_related_extra.go:44-47` (`checkECSASG`).
- **Trigger**: an account with two or more ECS clusters that use EC2 capacity providers with managed scaling or managed termination protection. ECS puts the `AmazonECSManaged` tag on those ASGs, and the tag does not identify a cluster. The code accepts that tag key without checking a value or cluster.
- **Impact**: every cluster's "Auto Scaling Groups" count includes the capacity ASGs of all other clusters. The operator investigating a capacity problem is sent to the wrong ASG.
- **Fix direction**: resolve the cluster's ASGs the way the spec describes: `Cluster.CapacityProviders` → `DescribeCapacityProviders` → `AutoScalingGroupProvider.AutoScalingGroupArn`, then match ASG ARNs. At minimum, drop the unscoped `AmazonECSManaged` match.

## 3. [P2] The EC2 pivot looks for instance tags that ECS does not set, so EC2-backed clusters show zero instances

- **File/line**: `core/aws/ecs_related_extra.go:82` (`checkECSEC2`).
- **Trigger**: an EC2 launch-type cluster whose container instances joined through `ECS_CLUSTER` in the agent config or user data, which is the standard setup. ECS does not tag the EC2 instance with the cluster name when it registers. The code only matches a `ClusterName` tag that the user adds by hand, or an `aws:ecs:cluster-name` tag.
- **Impact**: the panel shows "EC2 Instances 0" as a resolved count for a cluster with registered hosts. The operator cannot pivot to the hosts to check an agent disconnect or an impaired instance.
- **Fix direction**: follow the spec: `ListContainerInstances(cluster)` + `DescribeContainerInstances` → `Ec2InstanceId`, then cross-reference the `ec2` list by instance ID.

## 4. [P2] The CloudTrail pivot matches any event whose resource name contains the cluster name as a substring

- **File/line**: `core/aws/ecs_related_extra.go:115` (`checkECSCTEvents`).
- **Trigger**: a cluster whose name is a substring of other resource names, for example `default` (default VPC security groups, the `default` parameter groups), `prod` next to `prod-api`, or `api` next to `api-gateway-*`. The ct-events list is region-wide (`relatedResourcesFor` → `FetchRelatedTarget`).
- **Impact**: the "CloudTrail Events" count and list include events for unrelated resources and other clusters. The operator's audit trail for the cluster is polluted.
- **Fix direction**: match exactly: `ResourceName == clusterName`, or `ResourceName == clusterArn`, or an ARN ending in `:cluster/<name>`.

## 5. [P2] The log-group pivot is a cluster-name substring match and ignores the log group the cluster actually names

- **File/line**: `core/aws/ecs_related_extra.go:172` (`checkECSLogs`).
- **Trigger 1**: a cluster named `default`, `prod`, `app` or similar matches every log group whose name contains that string.
- **Trigger 2**: a cluster whose `ExecuteCommandConfiguration.LogConfiguration.CloudWatchLogGroupName` does not contain the cluster name is never matched.
- **Impact**: the "Log Groups" count overcounts with unrelated groups and misses the one log group the cluster configuration references (the `ecs exec` session log).
- **Fix direction**: read `Configuration.ExecuteCommandConfiguration.LogConfiguration.CloudWatchLogGroupName` (this depends on finding 1's `CONFIGURATIONS` include) and match the `logs` list by exact name.

## 6. [P3] The alarm pivots match the `ClusterName` dimension in any namespace, so EKS Container Insights alarms attach to same-named ECS clusters

- **File/line**:
  - `core/aws/ecs_related.go:56`: `alarmIDsByDimension(..., "", "ClusterName", ...)`, with an empty namespace.
  - `core/aws/alarm_related_extra.go:78` (`checkAlarmECS`): no namespace check. `checkAlarmEKS` at `:89` keys on the same dimension.
- **Trigger**: an EKS cluster and an ECS cluster share a name, or an alarm on the `ContainerInsights` (EKS) namespace uses `ClusterName`.
- **Impact**: the ECS cluster's alarm count includes EKS alarms. Going from alarm to cluster, every EKS Container Insights alarm offers an ECS cluster pivot, to a same-named cluster or to one that does not exist.
- **Fix direction**: restrict to the `AWS/ECS` and `ECS/ContainerInsights` namespaces in both directions.

## 7. [P3] The Wave 2 finding's detail text describes a condition the code does not test

- **File/line**: `core/aws/catalog_compute.go:482` (Detail of `ecs.cluster-issue`) against `core/aws/ecs_issue_enrichment.go:82-95`.
- **Trigger**: a cluster with registered container instances and zero running tasks, such as idle EC2 capacity with no services. `running == 0 && registered > 0` fires.
- **Impact**: the detail tells the operator the cluster has "fewer running than the services on it asked for" and "has run out of processor, memory or network-interface capacity". The code never compares against service desired counts. The fired condition means idle capacity, the opposite of running out. The operator is pointed at scaling up when the hosts are empty.
- **Fix direction**: reword the detail to cover both actual triggers (tasks pending; instances registered with nothing running), or split them into two finding codes with their own advice.

## 8. [P3] The Wave 2 enricher raises findings on clusters that are not ACTIVE

- **File/line**: `core/aws/ecs_issue_enrichment.go:67-97`. `cluster.Status` is never checked.
- **Trigger**: a cluster in `DEPROVISIONING`/`PROVISIONING` state with registered instances and no running tasks, or with pending tasks. The spec §4 scopes both Wave 2 signals to "(on ACTIVE cluster)".
- **Impact**: a row that already carries its lifecycle finding also gets "tasks pending or not running". The issue counts include a second finding the spec does not assign.
- **Fix direction**: skip clusters whose `Status` is not `ACTIVE` before evaluating the two Wave 2 conditions.

## efs

# efs — production-code review

Scope: `core/aws/efs*.go`, the `efs` catalog entry (`core/aws/catalog_databases.go`), the view defaults (`core/config/defaults_databases.go`), and the pivots that navigate into efs (`core/aws/lambda_related_extra.go`, `core/aws/subnet_related.go`). I also read the shared helpers these paths depend on: `issue_enrichment.go`, `runtime/helpers.go` FoldWave2Rows, `backup_coverage.go`, `backup_match.go`, `teardown.go`, `iampolicy/evaluate.go`, `semantics/projection/generic.go`, and `app/navigate.go`.

## 1. P2: The Lambda to EFS pivot returns access-point IDs, so drilling in never finds a file system

- **Location:** `core/aws/lambda_related_extra.go:52-71` (`checkLambdaEFS`, `ids = append(ids, arn[idx+1:])` at line 64). It is registered at `core/aws/catalog_compute.go:604`.
- **Trigger:** Open the detail of a Lambda function that has `FileSystemConfigs`. AWS only allows an EFS access-point ARN there: `arn:aws:elasticfilesystem:…:access-point/fsap-…`.
- **What happens:** The checker takes the last ARN segment (`fsap-…`) and reports it as an `efs` resource ID. `efs` rows are keyed by `FileSystemId` (`fs-…`), and `efs` has no FetchByIDs. The panel shows "EFS File Systems (1)", but Enter goes through `seedRelatedExactRows`/`RelatedIDSet` (`core/app/navigate.go:626-644`) and lands on an empty list. The pivot never reaches the file system. The function's own comment admits it cannot resolve fsap to fs.
- **Impact:** The operator cannot get from a Lambda function to the file system it mounts. The count promises a row that does not exist.
- **Fix:** Resolve access point to file system. Either call `efs:DescribeAccessPoints(AccessPointId=…)` and read `FileSystemId`, or scan the loaded efs rows with the per-FS access-point set that `checkEFSLambda` already builds. Emit `fs-…` IDs. Mark the result unknown when the resolution cannot run.

## 2. P2: The Subnet to EFS pivot cannot match any mount-target ENI description

- **Location:** `core/aws/subnet_related.go:257-273` (`checkSubnetEFS`), registered at `core/aws/catalog_networking.go:331`.
- **Trigger:** Open a subnet that hosts EFS mount targets.
- **What happens:** The checker needs `Description` to start with `"EFS mount target for "`, and it takes everything after that prefix as the file-system ID.
  - The format documented in the AWS CreateMountTarget reference is `"Mount target fsmt-id for file system fs-id"`, which fails the prefix check.
  - Real ENIs are commonly described as `"EFS mount target for fs-… (fsmt-…)"`. With that format the extracted ID is `"fs-… (fsmt-…)"`, which never equals an efs row ID.
  - The pivot only works if the description is exactly `"EFS mount target for <fs-id>"`, and neither format is.
  - I am not certain which format live accounts emit today, but both known formats break this parser. By contrast, the reverse checkers (`checkEFSSubnet`/`SG`/`ENI`/`VPC`) use `strings.Contains(desc, fsID)` and work with either format.
- **Impact:** The subnet's "EFS File Systems" pivot reports 0 while file systems are mounted in that subnet, which is a false negative on a blast-radius question.
- **Fix:** Extract the ID with a pattern such as `fs-[0-9a-f]+` from the description, or cross-check `strings.Contains(desc, efsRow.ID)` against the loaded efs list. Do not trust a fixed prefix plus the rest of the string.

## 3. P2: A single failed call on a file system throws away every issue already measured for it

- **Location:**
  - `core/aws/efs_issue_enrichment.go:103-116` (mount-target page failure, `MarkSkipped` at line 110).
  - `core/aws/efs_issue_enrichment.go:200`, `:204` and `:223` (policy call, parse or backup-policy failure).
  - The fold that drops the row is `core/runtime/helpers.go:82-96` (`FoldWave2Rows` skips every ID in `TruncatedIDs`).
- **Trigger:** Any one of these on a given file system:
  - `DescribeMountTargets` fails, for example a throttle that outlasts the retries.
  - `DescribeFileSystemPolicy` or `DescribeBackupPolicy` is denied. This is common with least-privilege roles that grant only `DescribeFileSystems`/`DescribeMountTargets`.
  - The file-system policy fails to parse.
- **What happens:** `MarkSkipped` puts the file system in `TruncatedIDs`, and `FoldWave2Rows` then skips that row completely.
  - A measured "file system policy open to anyone" (Broken) is discarded when the mount-target walk fails. The comment at lines 67-69 says the policy checks run first precisely so that "a mount-target pagination failure cannot swallow them".
  - Likewise, a measured "mount target down" (Broken) is discarded when only `DescribeBackupPolicy` was denied.
- **Impact:** The row shows "?" or its previous state instead of a Broken finding a9s actually proved. A real outage or public exposure disappears because an unrelated call failed.
- **Fix:** Keep the uninspected mark for the check that failed, but still fold the findings from the checks that answered. One way is a per-check uninspected mark, as the page-cap branch already reasons about. The other is not to mark the row uninspected when another check on it produced a definite result.

## 4. P2: The EFS to Backup Plans pivot ignores tag-based selections and partial selection reads

- **Location:** `core/aws/efs_related_extra.go:116-139` (`checkEFSBackup`, which calls `BackupPlanCoversARN` at line 135).
- **Trigger:** Two cases:
  - An AWS Backup plan selects file systems by tag (`ListOfTags`/`Conditions`, carried in `Fields["selection_tags"]`). This is the dominant real-world pattern, and EFS returns the tags on `FileSystemDescription.Tags` at no extra cost.
  - A plan's selection list could not be read completely (`Fields["selections_partial"]`).
- **What happens:** The match uses only `resources`/`not_resources` ARNs. A file system protected by a tag-selected plan reports "Backup Plans (0)" as a known, non-truncated zero. A partially read plan is also treated as covering nothing.
  - The shared coverage join in `core/aws/backup_coverage.go:152-172` (`backupPlanCovers`, `backupSelectionTagsMatch`) handles tags.
  - `backupPlansIncomplete` in the same file handles partial reads.
  - `checkEFSBackup` uses neither, so there are two truth sources for "does plan X cover resource Y".
- **Impact:** The operator is told no backup plan protects a file system that is in fact backed up, or the result is presented as certain when it is not.
- **Fix:**
  - Build a tag map from `fs.Tags` and use `backupPlansCover(plans, fsARN, tags)`.
  - Mark the result truncated or unknown when `backupPlansIncomplete(plans)`.
  - Remove the ARN-only path.

## 5. P3: Every file system being created or deleted shows as Broken "no mount targets" instead of its lifecycle state

- **Location:** `core/aws/efs.go:36-43` (`efsW1Findings`).
- **Trigger:** A file system in `creating` or `deleting` state:
  - A new file system cannot have mount targets until it is `available`.
  - AWS rejects `DeleteFileSystem` while mount targets exist, so a `deleting` file system always has `NumberOfMountTargets == 0`.
- **What happens:** The no-mount-targets finding (Broken) is placed first, so the Status column reads "no mount targets (+1)" in red. The spec's §4 list text for these rows, `creating` and `deleting` (Warning), can never be the top phrase.
  - `efsW1Findings` already skips the encryption finding for teardown states and skips no-mount-targets only for `deleted`.
  - The `creating` detail sentence itself says "wait … then add its mount targets".
- **Impact:** Routine creations and intended deletions look like broken file systems at the top of the list, which inflates the issue count and badge.
- **Fix:** Do not emit `CodeEFSNoMountTargets` when `lcs` is `creating`, `deleting` or `deleted`, meaning any transitional state where zero mount targets is expected.

## 6. P3: The EFS to Lambda pivot reads only the first page of access points

- **Location:** `core/aws/efs_related.go:190-195` (`checkEFSLambda`, a single `DescribeAccessPoints` call with no `NextToken` loop).
- **Trigger:** A file system with more than one page of access points. The API returns up to 100 per page by default, and the service quota allows far more per file system.
- **What happens:** Lambda functions that use access points on later pages are not matched. The result is not marked truncated.
- **Impact:** The Lambda count is too low but shown as exact.
- **Fix:** Page through `NextToken`, capped at `PerParentPageCap` pages. When the cap is hit, return `relatedResultTrunc(..., true)`.

## 7. P3: The KmsKeyId navigable field is never shown because the detail view does not render KmsKeyId

- **Location:** `core/aws/catalog_databases.go:630-632` registers `Navigable{FieldPath: "KmsKeyId", TargetType: "kms"}`. The efs detail field list at `core/config/defaults_databases.go:74-79` has no `KmsKeyId`.
- **Trigger:** Open the detail of an encrypted file system.
- **What happens:** `buildItems` (`core/semantics/projection/generic.go:98-164`) only extracts the configured detail paths. `KmsKeyId` is not rendered at all, so the registered navigation to the KMS key is dead.
- **Impact:** The detail body never shows the encrypting key. The operator can only find it through the related panel.
- **Fix:** Add `{Path: "KmsKeyId"}` to the efs detail defaults and regenerate `.a9s/views/efs.yaml`.

## 8. P3: "automatic backups off" says the data is unrecoverable without checking whether any AWS Backup plan covers the file system

- **Location:** `core/aws/efs_issue_enrichment.go:214-231`. The detail text is at `core/aws/catalog_databases.go:642`.
- **Trigger:** A file system whose EFS automatic backup policy is `DISABLED` or absent, but which a user-defined AWS Backup plan selects by ARN or tag.
- **What happens:** The finding is emitted from `DescribeBackupPolicy` alone. Its detail says "AWS Backup is not taking daily backups of this file system, so a deletion or corruption is unrecoverable", which is false when a custom plan backs it up. The shared `addBackupCoverage` join (`core/aws/backup_coverage.go:35-59`), which other types use for exactly this question, is never consulted.
- **Impact:** A false "unrecoverable" warning on file systems that are backed up, which pushes operators toward an unnecessary change.
- **Fix:** Suppress the finding, or reword it, when the cached backup list covers the file-system ARN or tags via `backupPlansCover`. Alternatively, limit the detail text to what was measured: the automatic backup policy is off.

## eip

# eip — production-code review

Scope reviewed: `core/aws/eip.go`, `core/aws/eip_codes.go`, `core/aws/eip_related.go`,
the `eip` catalog entry in `core/aws/catalog_networking.go:474-517`, the `eip` detail
defaults in `core/config/defaults_networking.go:65-71`, inbound pivots
(`core/aws/ec2_related.go:126-150`, `core/aws/eni_related.go:53-68`,
`core/aws/nat_related.go:85-104`, `core/aws/transfer_related.go:95-109`), the Route 53
dangling-record consumer of the eip cache (`core/aws/r53_issue_enrichment.go:60-130`),
the shared related helpers it depends on (`related_common.go`, `related_shared.go`,
`related_fetch.go`), and `cmd/snapshot/ec2_network.go:452-489`.

## Findings

### 1. [P3] Alarm pivot matches the wrong CloudWatch dimension set

- **File/line**: `core/aws/eip_related.go:109-114` (`checkEIPAlarm`)
- **Trigger**: open the detail view of an Elastic IP that is associated with an EC2
  instance which has any instance-level CloudWatch alarm (e.g. a `CPUUtilization` or
  `StatusCheckFailed` alarm dimensioned on `InstanceId`).
- **User impact**: the related panel's "CloudWatch Alarms" row counts and navigates to
  the instance's alarms as if they were alarms on the Elastic IP. The spec
  (`docs/resources/eip.md` §2 `alarm` and §5 "Out of Scope") defines the match as
  `Dimensions[]` with `Name` `AllocationId` or `NetworkInterfaceId`, and explicitly puts
  any other dimension out of scope. The code builds its match set from `InstanceId` and
  `NetworkInterfaceId`: it adds a dimension the contract excludes and drops the
  `AllocationId` dimension the contract requires. An EIP with only an allocation (no
  instance/ENI) returns a proven zero at line 115-117 without scanning alarms at all, even
  though an alarm dimensioned on its `AllocationId` would match under the contract.
- **Fix direction**: build `wanted` from `Address.AllocationId` (as `AllocationId`) and
  `Address.NetworkInterfaceId` (as `NetworkInterfaceId`); drop the `InstanceId` entry.
  Only return the early zero when both are empty.

## Checked and not defective

- `FetchElasticIPs` (`core/aws/eip.go:25-100`): `DescribeAddresses` is not paginated, so
  the single call and `IsTruncated: false` (`catalog_networking.go:500-503`) are correct.
  The unassociated rule (`eip.go:67`) matches the spec's three-field absence test.
- ec2/eni/cfn/asg/nat/ecs-* checkers return Unknown (or `unreadZero`) when the row has no
  RawStruct, carry truncation through, and use target IDs consistent with the target
  types' resource IDs (asg by name `asg.go:96`, cfn by stack name `cfn.go:64`, ecs-svc /
  ecs by the same convention as `ecs_task_related.go:17-29`).
- Route 53 dangling check reads `eipStatusUnattached` from the single source in `eip.go`.

## eks

# eks — production-code review

Scope: `core/aws/eks.go`, `eks_codes.go`, `eks_interfaces.go`, `eks_related.go`, `eks_related_extra.go`, the `eks` entry in `core/aws/catalog_containers.go`, `core/config/defaults_containers.go`, and the inbound pivots that target `eks` (`ng_related.go`, `iam_roles_related.go`, `subnet_related.go`, `alarm_related_extra.go`).

## 1. P2: the version-support catalogue reads only the first page of DescribeClusterVersions

- **Location**: `core/aws/eks.go:181-195` (`eksVersionCatalogue`)
- **Code**: the code makes one `DescribeClusterVersions(IncludeAll: true)` call. `out.NextToken` is never read, and `MaxResults` is never set.
- **API semantics**: DescribeClusterVersions is paginated. It returns `nextToken` and takes `maxResults` (1–100). AWS does not document the default page size (see the AWS EKS API reference, DescribeClusterVersions).
- **Trigger**: AWS returns the catalogue across more than one page, and a cluster runs a Kubernetes minor that is on a later page.
- **Impact**: that minor is missing from the map, so `eksPostureOf` (`eks.go:283`) sets `VersionSupport` to `unknown`. The broken `eks.version-unsupported` finding is then silently lost for a cluster that is past standard support. The row renders without the red signal and without the "Support" detail row.
- **Fix**: loop on `NextToken`, or use `eks.NewDescribeClusterVersionsPaginator`. Treat a mid-walk error as an unread catalogue (nil), as the code already does.

## 2. P2: the eks → ec2 pivot finds only managed-node-group instances and reports an exact count

- **Location**: `core/aws/eks_related_extra.go:241-330` (`checkEKSEC2`), especially `:289-294` and `:329`
- **Code**: instances are found only through `ListNodegroups` → `DescribeNodegroup` → `Resources.AutoScalingGroups` → `DescribeAutoScalingGroups`. When there are no managed node groups, or none has an ASG, the result is `KnownRelated("ec2", nil, false)`: an exact zero.
- **Spec gap**: the spec (`docs/resources/eks.md` §2 `ec2`) says to use the loaded `ec2` list, filtered by the tag `kubernetes.io/cluster/<name>=owned` OR `eks:cluster-name=<name>`.
- **Trigger**: a cluster whose workers are self-managed ASGs, Karpenter-provisioned nodes, or any other nodes outside managed node groups.
- **Impact**: the related panel shows "EC2 Instances 0" as a confirmed answer for a cluster with running worker nodes. In a mixed cluster it shows an undercount, also presented as exact. This misleads the operator during node troubleshooting.
- **Fix**: implement the spec's tag filter over the `ec2` target cache (`NeedsTargetCache`), matching either tag. Mark the result truncated when that cache is truncated. Alternatively, keep the ASG hop as an addition, but never return an exact zero from it.

## 3. P2: the eks → ami pivot reports an exact 0 for node groups whose AMI it did not resolve

- **Location**: `core/aws/eks_related_extra.go:180-183` and `:216-224` (`checkEKSAMI`)
- **Code**: node groups without a custom launch template are skipped (`continue`). So are launch templates that declare no `ImageId` (`:205-208`). With no IDs collected and no failures, the function returns `relatedResultTrunc("ami", nil, false)`: a confirmed zero.
- **Trigger**: the default managed node group, which has no launch template (the node group has an `AmiType` and `ReleaseVersion` but no LT), or an LT that inherits the EKS-optimised AMI.
- **Impact**: most EKS clusters show "AMI 0" as a proven answer, although every worker node runs an AMI. The spec (§2 `ami`) expects AMIs aggregated across the cluster's node groups.
- **Fix**: when any node group was skipped as unresolved, return Unknown if nothing was resolved, or Truncated if some AMIs were. Better: resolve the AMI from `AmiType`/`ReleaseVersion` (the node group's release AMI), or reuse the `ng` rows' resolved image.

## 4. P3: the eks → ami pivot and the ng fetcher disagree on which launch-template version is used

- **Location**: `core/aws/eks_related_extra.go:185-188`, compared with `core/aws/ng.go:30-33` (`resolveNGImageID`)
- **Code**: `checkEKSAMI` falls back to `"$Latest"` when `LaunchTemplate.Version` is empty. `resolveNGImageID`, which fills the ng row's image, falls back to `"$Default"`. The same fact (a node group's AMI) is computed two ways.
- **Trigger**: a node group whose `LaunchTemplate.Version` comes back empty, where the template's latest version differs from its default version.
- **Impact**: the cluster's AMI pivot names a different AMI from the one the node-group row shows. EKS uses the default version when none is specified, so the eks-side answer is the wrong one.
- **Fix**: call `resolveNGImageID` from `checkEKSAMI` instead of the inline LT read (keep the NotFound soft-skip there), so there is one resolver.

## 5. P2: the alarm → eks pivot treats ECS Container Insights alarms as EKS clusters

- **Location**: `core/aws/alarm_related_extra.go:91` (`checkAlarmEKS`)
- **Code**: `strings.Contains(*alarm.Namespace, "EKS") || strings.Contains(*alarm.Namespace, "ContainerInsights")`
- **Trigger**: an alarm in the `ECS/ContainerInsights` namespace (ECS Container Insights) that has a `ClusterName` dimension. `"ECS/ContainerInsights"` contains `"ContainerInsights"`.
- **Impact**: the alarm's related panel shows "EKS Clusters 1", naming an ECS cluster. Drilling in finds a nonexistent EKS cluster, or a different EKS cluster that has the same name. The value is also returned without a check against the `eks` list.
- **Fix**: match the namespaces exactly: `AWS/EKS` and `ContainerInsights`. Excluding `ECS/ContainerInsights` alone is not enough.

## 6. P3: the eks → alarm pivot also counts ECS alarms for a same-named ECS cluster

- **Location**: `core/aws/eks_related.go:50` (`checkEKSAlarms`)
- **Code**: `alarmIDsByDimension(ctx, clients, cache, "", "ClusterName", res.ID)` passes an empty namespace, so an alarm in any namespace with `ClusterName == <name>` matches. That includes `AWS/ECS` and `ECS/ContainerInsights`.
- **Trigger**: an ECS cluster and an EKS cluster that share a name (for example `prod`), with alarms on the ECS one.
- **Impact**: the EKS cluster's alarm count includes ECS alarms, and the pivot lists them as control-plane alarms for this cluster.
- **Fix**: filter to the EKS namespaces (`AWS/EKS`, `ContainerInsights`). `alarmIDsByDimension` takes one namespace, so this needs either a namespace-set variant or two calls merged.

## 7. P3: inbound role → eks and subnet → eks pivots skip unread (degraded) eks rows yet report exact counts

- **Location**: `core/aws/iam_roles_related.go:41-44` (`checkRoleEKS`) and `core/aws/subnet_related.go:316-322` (`checkSubnetEKS`)
- **Code**: an eks row whose `DescribeCluster` was denied or failed is built by `DegradedDetails` (`eks.go:55`, `:60`). It has no `RawStruct` and no `subnet_ids`, and both checkers `continue` past it. The result's truncated flag comes only from the cache entry. The executor seeds that entry from the fetcher's pagination, and a partial per-item failure does not set it (`core/runtime/executor.go:945-951`).
- **Trigger**: the operator lacks `eks:DescribeCluster` on one cluster, or DescribeCluster fails for it, and the operator opens that cluster's service role or one of its subnets.
- **Impact**: the role or subnet shows "EKS Clusters 0" (or an undercount) as a confirmed answer, although an unread cluster may use it.
- **Fix**: in both checkers, count skipped unread rows (no `RawStruct`, or `Fields[DegradedFindingField]` set). Return Truncated when some matched, or Unknown when nothing matched and an unread row existed.

## elb

# elb — production-code review

Scope: `core/aws/elb.go`, `elb_codes.go`, `elb_interfaces.go`, `elb_issue_enrichment.go`, `elb_listeners.go`, `elb_listeners_codes.go`, `elb_listener_rules.go`, `elb_related.go`, the `elb` / `elb_listeners` / `elb_listener_rules` entries in `catalog_networking.go`, the reverse checkers that target `elb` (`tg_related.go`, `sg_related.go`, `vpc_related.go`, `subnet_related.go`, `eni_related.go`, `waf_related.go`, `ecs_svc_related.go`, `asg_related.go`, `r53_related.go`, `cf_related.go`, `acm_related.go`, `eb_related_extra.go`), and the navigation path (`core/runtime/handlers_related.go`, `core/app/list_filter.go`, `core/resource/related.go`, `core/semantics/projection/generic.go`).

Summary: 5 findings (P0:0 P1:0 P2:3 P3:2)

---

## 1. P2 — The "weak TLS policy" check treats `ELBSecurityPolicy-TLS13-1-0-*` and `-TLS13-1-1-*` as modern, but they accept TLS 1.0 and 1.1

- **File**: `core/aws/elb_issue_enrichment.go:41-56` (`modernTLSPolicyPrefixes`, `isWeakTLSPolicy`)
- **Trigger**: An HTTPS or TLS listener uses `ELBSecurityPolicy-TLS13-1-0-2021-06`, `ELBSecurityPolicy-TLS13-1-1-2021-06`, `ELBSecurityPolicy-TLS13-1-0-PQ-2025-09`, `ELBSecurityPolicy-TLS13-1-0-FIPS-2023-04`, `ELBSecurityPolicy-TLS13-1-1-FIPS-2023-04` or `ELBSecurityPolicy-TLS13-1-0-FIPS-PQ-2025-09`. All of them start with `ELBSecurityPolicy-TLS13-`, so `isWeakTLSPolicy` returns false. The AWS ALB security-policy table (docs.aws.amazon.com/elasticloadbalancing/latest/application/describe-ssl-policies.html) marks TLSv1.0 = Yes for the `TLS13-1-0` policies and TLSv1.1 = Yes for the `TLS13-1-1` policies. It also lists non-forward-secret RSA key-exchange ciphers (`AES128-SHA`, `AES256-SHA`, and others) for both. `ELBSecurityPolicy-TLS-1-2-2017-01` also passes the check, even though it includes the non-FS ciphers `AES128-GCM-SHA256` and `AES256-SHA256`.
- **User impact**: A listener that still negotiates TLS 1.0 or 1.1 without forward secrecy shows no `weak TLS policy on <ports>` finding. The code comment at lines 36-40 and the finding's Detail ("require version 1.2 or later") both describe a guarantee that the prefix rule does not enforce, so the operator gets a false all-clear.
- **Fix direction**: Classify policies by the minimum protocol encoded in the name, not by family prefix. For example, treat `TLS13-1-0-*` and `TLS13-1-1-*` as weak. Decide explicitly whether `TLS-1-2-2017-01` and `TLS-1-2-Ext-2018-06` (non-FS) count as weak. Update the spec §3.2 wording to match.

## 2. P2 — Following a target group's `LoadBalancerArns` field to `elb` opens an empty list

- **Files**: `core/aws/catalog_networking.go:205` (`{FieldPath: "LoadBalancerArns", TargetType: "elb"}`); `core/resource/related.go:55-63` (`navIDExtractors` has no `"elb"` entry); `core/runtime/handlers_related.go:392-400`; `core/app/list_filter.go:100-115`
- **Trigger**: Open a target-group detail and press Enter on a `LoadBalancerArns` value. The value is the full ARN (`arn:aws:elasticloadbalancing:…:loadbalancer/app/<name>/<id>`). No `elb` NavID extractor exists, so the ARN is used as `TargetID`. `elb` rows use the bare LB name as `ID` (`elb.go:64`), so `relatedCacheHit` misses. Navigation then falls through to a filtered list with `FilterText = <ARN>`. `listRowMatches` matches only ID, Name, rendered cells and finding phrases. No `elb` column shows the ARN (`catalog_networking.go:102-109`), so every row is filtered out.
- **User impact**: The navigable field goes to an empty Load Balancers list instead of the balancer it names. `elb` has no `FetchByIDs`, so nothing recovers.
- **Fix direction**: Register an `elb` extractor in `navIDExtractors` that returns the `<name>` segment (the second-to-last `/` segment of a `loadbalancer/(app|net|gwy)/<name>/<id>` ARN). Alternatively, key `elb` rows by ARN everywhere.

## 3. P2 — The ASG → Load Balancers pivot returns IDs that no `elb` row has

- **File**: `core/aws/asg_related.go:160` and `:185-187` (`checkASGELB`)
- **Trigger**: An Auto Scaling group has `TargetGroupARNs`. The checker appends each target group's `LoadBalancerArns` (full ARNs) as `elb` IDs (line 186), but `elb` row IDs are LB names (`elb.go:64`). It also appends `asg.LoadBalancerNames` (line 160). Those are Classic ELB names, and a9s never lists Classic load balancers: `FetchLoadBalancersPage` calls only the ELBv2 `DescribeLoadBalancers`.
- **User impact**: The ASG detail's "Load Balancers" row shows a non-zero count. Enter opens a list scoped to `RelatedIDSet` = {ARNs / classic names}, and `relatedIDSubset` (`core/app/list_filter.go:20-31`) matches no row, so the list is empty. The count also includes Classic balancers that cannot be opened. The sibling checkers `checkTGELB` and `checkECSSvcELB` already resolve ARN to row ID through the `elb` cache.
- **Fix direction**: Resolve each ARN against the `elb` cache by `Fields["load_balancer_arn"]` and emit `elbRes.ID`, as `checkTGELB` does. Alternatively, map ARN to name the same way `checkWAFELB` (`waf_related.go:42-43`) and `checkACMELB` do. Do not report Classic `LoadBalancerNames` as navigable `elb` IDs, or report them as unresolvable.

## 4. P3 — The ACM pivot misses SNI certificates and ignores `DescribeListeners` pagination

- **File**: `core/aws/elb_related.go:159-180` (`checkELBACM`)
- **Trigger**: An HTTPS/TLS listener serves extra certificates through SNI, or the balancer's listeners span more than one `DescribeListeners` page. The checker makes one `DescribeListeners` call and never follows `NextMarker`. It reads `Listener.Certificates`, which per the ELBv2 API (`Listener.Certificates`: "The default certificate for the listener") holds only the default certificate. SNI certificates are returned only by `DescribeListenerCertificates`. The same file's enrichment already walks every page (`allELBListeners`, `elb_issue_enrichment.go:255-276`).
- **User impact**: "ACM Certificates" under-counts for multi-certificate listeners. The reverse pivot `acm` → `elb` (`checkACMELB`, driven by ACM `InUseBy`) does list the balancer, so the two directions disagree.
- **Fix direction**: Walk every listener page (reuse `allELBListeners`). For each HTTPS/TLS listener, also page through `DescribeListenerCertificates` so SNI certificate ARNs are included.

## 5. P3 — The CloudFront ↔ ELB pivots compare DNS names as exact strings

- **Files**: `core/aws/elb_related.go:205-209` (`checkELBCF`); `core/aws/cf_related.go:79-99` (`checkCfELB`)
- **Trigger**: A CloudFront origin names the balancer in any form other than the exact `DNSName` string: the `dualstack.` prefix, a trailing dot, or a different letter case. Both checkers use `==` or a map lookup on the raw strings. The sibling `checkR53ELB` (`r53_related.go:112-120`) canonicalizes with `canonicalDNS` and also tries `"dualstack."+dns`.
- **User impact**: Both the CloudFront row on an `elb` detail and the Load Balancers row on a `cf` detail show 0 for a distribution that does front the balancer.
- **Fix direction**: Compare with `canonicalDNS` on both sides and strip a leading `dualstack.`, the same way `checkR53ELB` does.

---

### Checked and not reported

- `FetchLoadBalancersPage`: the Marker/NextMarker loop and `PageSize` 50 (the API maximum is 400) are correct.
- `FetchELBListenerRules`: the compound cursor handles both local capping and AWS pages correctly.
- `EnrichELBAttributes`: the cap, the two parallel passes under `mu`, and failure de-duplication by ID in `AggregateFailures` are correct.
- WAF short-circuit for NLB/GWLB is correct.
- ENI description parsing for `app/`, `net/` and `gwy/` is correct.
- `checkTGELB`, `checkSGELB`, `checkVPCELB`, `checkSubnetELB`, `checkECSSvcELB`, `checkR53ELB`, `checkWAFELB` and `checkACMELB` all emit LB-name IDs that match `elb` rows.
- TCP-listener rule vs spec: spec §3.2 says to flag "NLB `TCP` listener on 443". `ELBListenerIsPlaintext` (`elb_issue_enrichment.go:100-109`) flags TCP on 80/8080 and deliberately treats TCP:443 as TLS passthrough. That is a spec-wording question rather than a demonstrable runtime defect, so it is not counted.
- Not verified, so not reported: the CloudTrail `ResourceName` form for ELBv2 events (the ID is the name; the ARN is available through `Fields.load_balancer_arn`), and whether Elastic Beanstalk's `LoadBalancers[].Name` is an ARN for ALB environments.

## eni

# eni — production-code review

Scope: `core/aws/eni.go`, `core/aws/eni_related.go`, `core/aws/eni_codes.go`, the `eni` entry and `colorENI` in `core/aws/catalog_networking.go`, and every checker or enricher that pivots into `eni` or reads its cache (`checkLambdaENI`, `checkDbiENI`, `EnrichSGUsage`, plus the ec2/ecs-task/efs/elb/sg/vpc/subnet/rtb/nat/eip/vpce reverse checkers, which I read and found nothing to report on).

## 1. P2 — The ENI list silently leaves out AWS-managed interfaces, so the SG "not attached to anything" finding goes wrong

- **File/line**: `core/aws/eni.go:19-21` (the `DescribeNetworkInterfacesInput` does not set `IncludeManagedResources`). The consumer that turns this into a wrong finding is `core/aws/sg_issue_enrichment.go:39-58`.
- **Trigger**: an account where EC2 managed-resource visibility is `hidden`. AWS made that the default from 2026-04-22 for accounts that had no managed resources before, for example new EKS Auto Mode nodes. `DescribeNetworkInterfaces` then leaves out the managed ENIs unless the request sends `IncludeManagedResources=true`. SDK v1.332.0 has the field (`api_op_DescribeNetworkInterfaces.go:166`), and nothing in `core/`, `internal/` or `cmd/` sets it.
- **User impact**: managed ENIs are missing from the `eni` list. The subnet/vpc/sg → eni counts come out too low. Worse, `EnrichSGUsage` treats a complete, non-truncated `eni` cache as proof of absence, so a security group used only by EKS Auto Mode nodes is flagged `sg.unused` ("not attached to anything"). That tells the operator to delete a group that is in use.
- **Fix direction**: set `IncludeManagedResources: aws.Bool(true)` on the list request. This fits the read-only listing model: show what exists.

## 2. P2 — An unattached requester-managed ENI is never flagged, which contradicts the spec's orphan signal

- **File/line**: `core/aws/eni.go:149-151` (`case "available": if requesterManaged != "true" { … }`). `colorENI` in `core/aws/catalog_networking.go:81-86` reuses the same predicate.
- **Trigger**: an ENI with `Status == available` and `RequesterManaged == true`. A typical case is an interface left behind after its Lambda function, VPC endpoint or load balancer was deleted.
- **User impact**: the spec (`docs/resources/eni.md:106`, table row at `:136`) says `Status == available` → Warning, with no requester-managed exemption. The code returns no finding, so the row is uncoloured and missing from the issue badge. This is exactly the "zombie" population the spec says operators need to clean up, and it hides the most common orphan.
- **Fix direction**: follow the spec and emit `CodeENIStateAvailable` for every `available` ENI. If a specific service is known to park its ENIs in `available`, exempt it explicitly with a citation instead of exempting every requester-managed ENI.

## 3. P2 — The dbi → eni pivot counts other DB instances' interfaces

- **File/line**: `core/aws/dbi_related.go:264-273`.
- **Trigger**: two or more RDS, Aurora, DocumentDB or Neptune instances share a VPC security group, which is the normal setup. The filter is `description=RDSNetworkInterface` AND `group-id ∈ {this instance's SGs}`. Every instance gets that same description, and the `group-id` values match with OR.
- **User impact**: the related panel of one DB instance lists and counts the ENIs of every DB instance that shares any of its security groups. The count is inflated and the pivot opens interfaces that do not belong to it.
- **Fix direction**: narrow the match to something that belongs to this instance. At minimum add a `vpc-id` filter, plus a `subnet-id` filter from its `DBSubnetGroup`. If no field on `DBInstance` uniquely identifies its ENIs, mark the result as approximate (truncated/unknown) instead of stating an exact count.

## 4. P2 — The forward and reverse Lambda ENI pivots use two different "is this a Lambda ENI" rules

- **File/line**: forward `core/aws/eni_related.go:143`, which calls `isLambdaENI` at `:247-256` and accepts a RequesterId containing `awslambda`, `lambda.amazonaws.com`, or any Description containing `Lambda`. Reverse `core/aws/lambda_related_extra.go:545`, which requires `Fields["requester_id"] == "AWS Lambda VPC ENI"` exactly.
- **Trigger**: any Lambda ENI whose RequesterId is not the literal `AWS Lambda VPC ENI`, including the `…awslambda…` shapes the forward checker itself expects. A second trigger is a customer-created ENI whose Description happens to contain "Lambda".
- **User impact**:
  - In the first case, eni → lambda shows the function, but that function's lambda → eni shows 0. The two sides of the same edge contradict each other.
  - In the second case, `isLambdaENI` returns true, the description parse returns `""`, and the eni → lambda row reports unknown instead of a proven 0.
- **Fix direction**: use one shared predicate in both directions, keyed on `InterfaceType == lambda` (the documented type field; the spec's `docs/resources/eni.md` lambda section keys on it), plus the `AWS Lambda VPC ENI-` description prefix. Drop the bare `"Lambda"` substring test.

## 5. P3 — Elastic IPs on secondary private IPs are missed

- **File/line**: `core/aws/eni_related.go:63-67`. The navigable field at `core/aws/catalog_networking.go:661` (`Association.AllocationId`) has the same gap.
- **Trigger**: an ENI with Elastic IPs associated to secondary private IPv4 addresses, for example a NAT instance or a multi-IP host. `NetworkInterface.Association` describes only the primary IP; the rest live in `PrivateIpAddresses[].Association` (SDK `types.NetworkInterfacePrivateIpAddress.Association`).
- **User impact**: the eni → eip count shows 0 or 1 when several EIPs are billed on the interface, and the detail view only navigates from the primary one. The reverse eip → eni pivot (`eip_related.go:30-39`) does resolve, so the two directions disagree.
- **Fix direction**: collect `AllocationId` from `PrivateIpAddresses[].Association` (deduplicated, primary included) in `checkENIEIP`, and add the matching `PrivateIpAddresses.Association.AllocationId` navigable path.

## 6. P3 — A shared Lambda Hyperplane ENI resolves to only one function

- **File/line**: `core/aws/lambda_related_extra.go:542-551` (reverse, prefix `AWS Lambda VPC ENI-<fnName>-`) and `core/aws/eni_related.go:147-151` (forward, one name parsed from Description).
- **Trigger**: two or more functions in the same account use the same subnet and security-group pair. AWS documents that such functions reuse the same Hyperplane ENI, but the ENI Description carries a single function name.
- **User impact**: every function other than the one named in the Description shows lambda → eni = 0, although its traffic goes through that ENI. eni → lambda likewise shows one function out of several.
- **Fix direction**: for lambda → eni, match Lambda-typed ENIs (`InterfaceType == lambda`) whose `SubnetId` is in the function's `VpcConfig.SubnetIds` and whose security-group set equals the function's `VpcConfig.SecurityGroupIds`. Build eni → lambda the same way, over the lambda cache.

## glue

# glue: production-code review

Scope: `core/aws/glue.go`, `glue_codes.go`, `glue_interfaces.go`, `glue_issue_enrichment.go`, `glue_related.go`, `glue_runs.go`, `glue_runs_codes.go`, the `glue` / `glue_runs` entries in `core/aws/catalog_data.go`, the reverse checkers `checkS3Glue` (`s3_related.go`) and `checkRoleGlue` (`iam_roles_related.go`), `cmd/snapshot/ops.go` `captureGlue`, and the helpers they call (`relatedResult`, `bucketFromS3URI`, `roleNameFromARN`, `alarmIDsByDimension`, `ARNForService`, `kmsKeyIDFromField`, runtime related-ID resolution in `core/runtime/handlers_related.go` / `related.go`).

## Findings

### 1. P2: Wave 2 ignores `EXPIRED`, so the job row stays green and "Last Run" reads `OK`

- **File:** `core/aws/glue_issue_enrichment.go:63` (and `:76`)
- **Trigger:** the job's latest run from `GetJobRuns(MaxResults=1)` has `JobRunState == EXPIRED`.
- **Impact:** the spec (`docs/resources/glue.md` §3.2 and §4) lists latest-run `EXPIRED` as Broken `!` with the text `latest run expired`. The condition only checks `FAILED || ERROR || TIMEOUT`, so the job gets no finding, renders healthy, and the else branch writes `last_run: "OK"`. The child `glue_runs` view does classify `EXPIRED` as broken (`glue_runs.go` `glueRunFindings`), so the job list and the drill-down disagree about the same run.
- **Fix:** add `gluetypes.JobRunStateExpired` to the broken set. Better: derive the job's verdict from the same state classification the `glue_runs` rows use, so the two views cannot drift.

### 2. P3: the "Last Run" column says `OK` for runs that are not finished

- **File:** `core/aws/glue_issue_enrichment.go:75-76`
- **Trigger:** the latest run is `RUNNING`, `STARTING`, `WAITING`, `STOPPING` or `STOPPED` (and `EXPIRED`, see finding 1).
- **Impact:** every state outside the broken set is flattened to `last_run = "OK"`. An operator sees `OK` for a run that is still in progress or queued, and the job's list row claims a successful outcome that has not happened.
- **Fix:** write the actual state (for example the humanized `JobRunState`) to `last_run`, and keep `OK` for `SUCCEEDED` only.

### 3. P2: `continuous logging off` is a false warning on every Glue 5.0 job

- **File:** `core/aws/glue.go:134`
- **Trigger:** any job with `GlueVersion == "5.0"` (or later) that does not set `--enable-continuous-cloudwatch-log=true`. Glue 5.0 jobs normally do not set it.
- **Impact:** AWS documentation ("Logging for AWS Glue jobs"; "Enabling continuous logging for AWS Glue 4.0 and earlier jobs") says: "with the introduction of AWS Glue 5.0, all jobs have real-time logging capability." The argument is a 4.0-and-earlier feature. Every 5.0 job still gets a Warning `~` row and badge, and its detail text says output "only appears after the run ends", which is untrue for 5.0. That is noise on healthy modern jobs.
- **Fix:** evaluate the flag only when the job's `GlueVersion` is earlier than 5.0.

### 4. P2: the Log Groups pivot misses the groups a job actually writes to and counts groups it does not write to

- **File:** `core/aws/glue_related.go:59`
- **Trigger:** any of the following:
  - a job on Glue 4.0 or earlier with continuous logging on: the default group is `/aws-glue/jobs/logs-v2`;
  - `DefaultArguments["--continuous-log-logGroup"]` is set (the spec, §2 `logs`, requires this source);
  - Glue 5.0 `--custom-logGroup-prefix` is set;
  - a security configuration with CloudWatch encryption is attached: the groups become `/aws-glue/jobs/<secconfig>-role/<role>/{output,error}`, or `<group>-<secconfig>` for continuous logs.
- **Impact:** the checker matches only the fixed pair `/aws-glue/jobs/output` and `/aws-glue/jobs/error`. It returns a dimmed, non-navigable proven `(0)` for jobs whose logs sit in the groups listed above. It also returns the shared pair for jobs that never log there. On a job that just failed, the operator's first pivot, "where are the logs", points to the wrong place or to a dead end.
- **Fix:** build the candidate names from `GlueVersion`, `--enable-continuous-cloudwatch-log`, `--continuous-log-logGroup`, `--custom-logGroup-prefix`, and the `SecurityConfiguration` and `Role` names. Match those candidates against the logs cache.

### 5. P2: the Secrets Manager pivot returns an ID with the random suffix, so the count never resolves to a row

- **File:** `core/aws/glue_related.go:241`
- **Trigger:** a job default argument holds a secret ARN such as `arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db-AbCdEf`.
- **Impact:** `CutPrefix(a.Resource, "secret:")` yields `prod/db-AbCdEf`. The canonical secrets `Resource.ID` is the bare name `prod/db` (`secrets.go:71`), and drill resolution matches IDs exactly (`core/runtime/handlers_related.go:423-430`, `core/runtime/related.go:36-58`; secrets registers no FetchByIDs). The panel shows `Secrets Manager (1)`, but Enter opens a filtered list that contains no matching secret.
- **Fix:** strip the 6-character random suffix (`-XXXXXX`) Secrets Manager appends to the ARN, or match the full ARN against the cached secrets' ARN (see `secretIdentifiers` in `secrets_related.go`) and return the cached row's ID.

### 6. P2: the Secrets Manager pivot ignores the job's Connections, so it reports a false proven zero

- **File:** `core/aws/glue_related.go:229-251` (`checkGlueSecrets`)
- **Trigger:** the job uses a Glue Connection (`Job.Connections.Connections[]`) whose `ConnectionProperties["SECRET_ID"]` names a secret. This is the standard JDBC credential path, and the default arguments carry no secret ARN.
- **Impact:** the spec (§2 `secrets`) says the discovery path is `GetConnection` → `SECRET_ID`, and calls it "the only on-resource path from a Glue job to the consumed secret." The checker scans only `DefaultArguments` and returns `KnownRelated(nil, false)`, a dimmed, non-navigable `(0)`. The operator is told the job uses no secrets, and cannot find the rotated JDBC secret that broke the run.
- **Fix:** resolve each connection name with `glue:GetConnection` and collect `SECRET_ID`, normalized to the secrets `Resource.ID`. Return Error or Unknown, not zero, when a connection cannot be read.

### 7. P2: the Athena WorkGroups pivot is effectively always a proven zero

- **File:** `core/aws/glue_related.go:215`
- **Trigger:** any Glue job, with the athena list loaded.
- **Impact:** the checker matches `wg.Fields["glue_job"]`, but no production code sets that field (the only occurrence in `core/` and `internal/` is this line). It also matches `wg.ID == jobName`, which holds only by coincidence. The spec (§2 `athena`) requires the full account-wide workgroup list ("Count shown: yes (the full account-wide workgroup count)"). Instead every job shows a dimmed, non-navigable `(0)`, even though the related-panel contract lists Athena as a real pivot.
- **Fix:** return every cached athena workgroup ID, keeping the cache truncation flag, as the spec defines. Drop the dead `glue_job` match.

### 8. P3: KMS alias ARNs in the security configuration become IDs that never match a key

- **File:** `core/aws/glue_related.go:175`
- **Trigger:** a security configuration's `S3Encryption[].KmsKeyArn`, `CloudWatchEncryption.KmsKeyArn` or `JobBookmarksEncryption.KmsKeyArn` holds an alias ARN (`arn:aws:kms:<region>:<acct>:alias/etl`). The API pattern `^$|arn:aws[a-z0-9-]*:kms:.*` allows this.
- **Impact:** last-`/`-segment extraction yields `etl`. That is neither a key ID (kms `Resource.ID`) nor the `alias/etl` form that the shared helper `kmsKeyIDFromField` (`related_common.go:150`) produces for sibling types. The panel counts a key the drill cannot find or fetch.
- **Fix:** use `kmsKeyIDFromField` rather than the ad-hoc split, so glue follows the same ARN-to-ID rule as every other KMS pivot.

## iam-group

# iam-group production-code review

Scope: `core/aws/iam_groups.go`, `iam_groups_related.go`, `iam_group_members.go`, `iam_group_issue_enrichment.go`, the `iam-group` / `iam_group_members` entries in `core/aws/catalog_security.go`, and the direct dependencies they use to resolve pivots (`core/aws/iam_policies.go`, `core/session/policy_store.go`, `core/aws/related_common.go`, `core/aws/issue_enrichment.go`, `core/aws/policy_findings.go`).

## 1. P2: the IAM Users related count ignores `GetGroup` pagination, so it disagrees with the Members column

- **File:line**: `core/aws/iam_groups_related.go:24-36`
- **Trigger**: a group has more than 100 members. `GetGroup` returns 100 users by default. AWS also documents that IAM "might return fewer results, even when there are more results available" and sets `IsTruncated=true` in that case. `checkGroupUser` makes one call, never reads `out.IsTruncated` or `out.Marker`, and returns `relatedResult` (exact, `truncated=false`).
- **User impact**: the related panel shows "IAM Users (100)" as an exact count. On the same row, the Wave 2 enricher pages through every member (`iam_group_issue_enrichment.go:70-95`) and the Members column shows, for example, `150`. This is one fact computed two different ways. The count is wrong and the pivot lists only some of the group's members, which undercounts blast radius before someone edits a policy.
- **Fix direction**: use one paginated member walk, the same one the enricher uses (Marker/IsTruncated with PerParentPageCap), in both places. When the walk stops early, return `relatedResultTrunc(..., true)`.

## 2. P2: inline group policies are keyed by bare policy name, so the Policies pivot can land on another group's policy or on a managed policy

- **File:line**: `core/aws/iam_groups_related.go:56-58` emits bare inline names. `core/aws/iam_policies.go:436-446` builds inline rows with `ID: name`. `core/aws/iam_policies.go:229-231` calls `store.Set(r.ID, r)` over entries that were already keyed by customer-managed name. `core/session/policy_store.go:47-54` stores one value per key.
- **Trigger**: inline policy names are unique only within one group. Two cases trigger this:
  - Group `A` and group `B` each have an inline policy named `s3-access`.
  - A group has an inline policy with the same name as a customer-managed policy.
- **User impact**: from group `B`'s detail view, the IAM Policies pivot resolves `s3-access` to whichever entry was stored last. That can be group `A`'s inline policy (path `inline/A`) or it can replace the customer-managed policy. In the policy list, several rows share the same ID, so an ID-filtered drill shows other groups' policies too. The operator ends up inspecting the wrong permissions.
- **Fix direction**: give inline group policies an ID scoped to the group, such as `inline/<group>/<name>`. Emit that ID from `checkGroupPolicy`, and resolve and store under the same key. Another option is to drop inline names from the pivot, since the spec (§2 and §5) says it covers managed policies only.

## 3. P2: inline-policy resolution lists only the first page of groups, so pivots for later groups fail

- **File:line**: `core/aws/iam_policies.go:383-387`. `ListGroups` is called once with no Marker loop and `IsTruncated` is ignored. `FetchIAMPoliciesByIDsFull` then marks the build complete at `core/aws/iam_policies.go:257`.
- **Trigger**: the account has more than 100 IAM groups, or IAM returns a short page with `IsTruncated=true`. A group beyond the first page has an inline policy.
- **User impact**: `checkGroupPolicy` counts that group's inline policy, but no resolver ever stores it. `FetchIAMPoliciesByIDsFull` then tries it as an AWS-managed policy with `GetPolicy` (`iam_policies.go:268-274`), which fails. The drill from the IAM Policies row errors with "not resolvable by name" and never shows the policy. The inline build is still marked complete, so it never retries during the session. The policy list fetcher has the same gap (`catalog_security.go:153`): those inline policies never appear, and nothing indicates the list is incomplete.
- **Fix direction**: loop `ListGroups` on `Marker` while `IsTruncated`. Also page `ListGroupPolicies` (see finding 4).

## 4. P3: `checkGroupPolicy` and the inline sweep ignore `IsTruncated` on `ListAttachedGroupPolicies` / `ListGroupPolicies`

- **File:line**: `core/aws/iam_groups_related.go:51-65` and `core/aws/iam_policies.go:430-432`
- **Trigger**: IAM returns a short page with `IsTruncated=true`. AWS documents this for all IAM list calls and recommends checking `IsTruncated` after every call.
- **User impact**: the IAM Policies count is shown as exact (`truncated=false` unless a call errored) while policies are missing. The enricher pages both calls (`iam_group_issue_enrichment.go:104-163`), so the Attention section and the related panel can disagree about whether the group has policies.
- **Fix direction**: page both calls with Marker, or pass `IsTruncated` through to `relatedResultTrunc`. Better, reuse the enricher's paginated walk so there is one source of truth.

## 5. P3: one denied or failed list call discards findings that were already proven

- **File:line**: `core/aws/iam_group_issue_enrichment.go:175-178`
- **Trigger**: `ListGroupPolicies`, or any one of the three first calls, fails. For example, the operator's role allows `iam:GetGroup` and `iam:ListAttachedGroupPolicies` but denies `iam:ListGroupPolicies`.
- **User impact**: the function returns before it sets `member_count` or evaluates `adminAttachedPolicyName(allAttached)`. A group with `AdministratorAccess` attached therefore shows only `?` and no "has an administrator policy" warning, even though the data needed to prove it was fetched. The Members column is blank even though `GetGroup` succeeded.
- **Fix direction**: gate each result only on the calls it depends on:
  - `member_count` and the orphan condition need the member walk.
  - admin-attached needs the attached-policy walk.
  - no-policies needs both policy walks.

## 6. P3: the orphan finding ignores the spec's 30-day age threshold

- **File:line**: `core/aws/iam_group_issue_enrichment.go:191-197`
- **Trigger**: a group was created minutes ago and has no members yet, which is the normal state right after `CreateGroup`.
- **User impact**: the row turns yellow at once with "no members or no policies". Spec §3.2 requires `Users==[]` AND `now - CreateDate > 30d`. `CreateDate` is available on the row (`RawStruct` is `iamtypes.Group`) but the enricher never reads it. This produces false-positive hygiene warnings during normal provisioning.
- **Fix direction**: raise the no-members row only when the group's `CreateDate` is more than 30 days old. If the no-policies condition is meant to stay age-independent, fix the spec text instead.

## iam-user

# iam-user — production code review

Scope: `core/aws/iam_users.go`, `core/aws/iam_users_related.go`, `core/aws/iam_user_issue_enrichment.go`, the `iam-user` catalog entry in `core/aws/catalog_security.go`, `core/config/defaults_security.go` (iam-user detail), `core/aws/policy_findings.go`, the reverse checkers that land on `iam-user` (`checkGroupUser`, `checkPolicyUser`, `checkRoleIamUser`, `checkCtEventsUser`), and the CloudTrail detail navigation into `iam-user` (`core/semantics/ctevent/sections.go`, `target.go`).

## Findings

### 1. P2: the detail view always shows Permissions Boundary and Tags as empty

- **File:** `core/config/defaults_security.go:28` (detail paths `PermissionsBoundary`, `Tags`), with `core/aws/iam_users.go:76` (`RawStruct: user` comes from `ListUsers`)
- **Trigger:** open any IAM user whose permissions boundary is set or who has tags.
- **Evidence:** the AWS ListUsers API reference says: "This operation does not return the following attributes, even though they are an attribute of the returned object: PermissionsBoundary, Tags. To view all of the information for a user, see GetUser." The iam-user catalog entry has no `DetailEnrich` and no `FetchByIDs`. No production code calls `GetUser` or `ListUserTags`, so `RawStruct` never holds these fields.
- **User impact:** the detail view shows a boundary-constrained user as if no boundary were set, and a tagged user as if it had no tags. The operator reads missing data as a real absence, which matters most for the boundary because it limits what the user can do.
- **Fix direction:** add a `DetailEnrich` (or a `FetchByIDs`) that calls `GetUser(UserName)` and replaces `RawStruct` with `GetUserOutput.User`. The other option is to remove both paths from the iam-user detail list.

### 2. P2: the ct-events count and list include role-session events whose session name matches the user name

- **File:** `core/aws/iam_users_related.go:83` (cache match on `raw.Username`) and `:92` (fetch filter `Username=<name>`)
- **Trigger:** IAM user `alice` exists, and someone assumes a role with session name `alice`. This is common with SSO or `--role-session-name $USER`. For AssumedRole events, CloudTrail's `Event.Username` is the session name.
- **Evidence:** the ct-events fetcher sets `Fields["user"]` only when `userIdentity.type == "IAMUser"` (`core/aws/ct_events.go:233-236`, `:279`), and the reverse pivot `checkCtEventsUser` (`core/aws/ct_events_related.go:19`) reads that field. The forward pivot instead matches the raw `Username` of every event type when the event has a `RawStruct`, and it drills with a `LookupEvents` Username filter that has the same ambiguity. The two directions compute the same relationship in two different ways.
- **User impact:** the user's CloudTrail panel shows events the user never made, which is misleading during credential-compromise triage. Opening one of those events and pivoting back to iam-user shows 0.
- **Fix direction:** decide identity with one predicate: identity type is IAMUser and the user name matches. Use it in the cache scan (for example, match on `Fields["user"]`) and apply it after the Username-filtered fetch so drilled rows are filtered too.

### 3. P2: the group → IAM Users pivot stops after the first 100 members

- **File:** `core/aws/iam_groups_related.go:24` (`checkGroupUser`)
- **Trigger:** an IAM group with more than 100 members.
- **Evidence:** `GetGroup` pages with `MaxItems`/`Marker` (default 100) and returns `IsTruncated`. The checker makes one call, ignores `IsTruncated` and `Marker`, and returns an exact `relatedResult`.
- **User impact:** the count is too low but is shown as exact, and the drilled IAM Users list leaves out members past the first page.
- **Fix direction:** loop over `GetGroup` pages using `Marker` until `IsTruncated` is false, with a page cap as in `listAttachedUserPolicies`. Report the result as truncated if the cap is hit.

### 4. P3: the MFA column shows "true" for users with no MFA device

- **File:** `core/aws/iam_user_issue_enrichment.go:137-140`
- **Trigger:** a programmatic-only user (no login profile) with no MFA device.
- **Evidence:** `mfaVal` is "true" whenever `!hasConsolePassword`. `ListMFADevices` is never called for these users (line 94), so the code never learns whether they have a device.
- **User impact:** the column titled "MFA" says MFA is present for users who have none. An operator scanning the column for MFA coverage gets a false positive.
- **Fix direction:** write a value that says MFA does not apply to these users (for example, "n/a" or "-"), not "true". The other option is to call `ListMFADevices` for every user and report the actual device state.

### 5. P3: the console "never used" and "dormant" findings ignore when the current password was created

- **File:** `core/aws/iam_user_issue_enrichment.go:77` (the `GetLoginProfile` output is discarded) and `:153-168`
- **Trigger:** (a) A user created two years ago gets a console password today, which raises "console password never used" at once. (b) A user whose earlier password was last used 200 days ago gets a new login profile today. `PasswordLastUsed` still holds the old date ("If the user does not currently have a password but had one in the past, this field contains the date and time the most recent password was used"), so "console sign-in unused for 90 days" is raised.
- **Evidence:** both checks use `User.CreateDate` or `User.PasswordLastUsed`. Neither uses `LoginProfile.CreateDate`, which the code already receives and discards. The spec wording (§3.2, §4) names `CreateDate`, but the finding's own detail text says the password is "an unguarded sign-in path nobody is watching", which is false for a password created today.
- **User impact:** false warnings on newly provisioned console users. They push operators toward deleting login profiles that are in active use.
- **Fix direction:** keep `GetLoginProfileOutput.LoginProfile.CreateDate`. Require it to be more than 90 days old for the never-used finding. For the dormant finding, measure from the later of `PasswordLastUsed` and the login-profile `CreateDate`.

### 6. P3: CloudTrail TARGET "User" rows keep the IAM path in the navigation ID

- **File:** `core/semantics/ctevent/target.go:115-118`
- **Trigger:** a CloudTrail event whose `resources[]` envelope contains an `AWS::IAM::User` ARN for a user with a path other than `/`, such as `arn:aws:iam::123456789012:user/eng/alice`. I did not confirm which IAM events carry `AWS::IAM::User` in `resources[]`.
- **Evidence:** `navID` is everything after the first separator of `user/eng/alice`, which is `eng/alice`. Role rows get `roleNavID` to drop the path, but user rows do not. `NavIDFromValue` (`core/resource/related.go:41`) has an iam-user extractor but no production callers, and `core/app/actions_nav.go:578` uses `NavID` as it is. iam-user IDs are bare user names (`core/aws/iam_users.go:62`). The Principal row (`sections.go:175-189`) strips the path correctly, so the two paths disagree.
- **User impact:** pressing Enter on the User target opens a list filtered to `eng/alice` with no match, so the operator cannot reach the user.
- **Fix direction:** strip to the last `/` segment for `iam-user` the same way as for `role`. Better, route both through one ARN-to-ID function (`arnNavID` or `NavIDFromValue`) so the Principal and Target rows cannot disagree.

### 7. P3: the role → IAM Users (trust) pivot counts other accounts' users as local users

- **File:** `core/aws/iam_roles_related.go:80` (`extractPrincipalsByKind(..., ":user/", ...)`, which takes the last ARN segment)
- **Trigger:** a trust policy that trusts `arn:aws:iam::999999999999:user/deploy` from another account.
- **Evidence:** the account segment is dropped, so `deploy` is returned as an iam-user ID in the current account.
- **User impact:** the count includes principals that are not in the listed account. The drill either finds nothing or opens an unrelated local user who happens to have the same name, which misattributes trust.
- **Fix direction:** parse the ARN and keep only principals whose account matches the session account. Show cross-account user principals separately or leave them out of the iam-user pivot.

## Summary

7 findings: P0:0 P1:0 P2:3 P3:4

## igw

# igw — production-code review

Scope: `core/aws/igw.go`, `core/aws/igw_related.go`, `core/aws/igw_codes.go`, the `igw` catalog entry and `colorIGW` in `core/aws/catalog_networking.go`, `core/config/defaults_networking.go` (`igw` detail), inbound pivots `checkVPCIGW` (`core/aws/vpc_related.go`) and `checkRTBIGW` + the rtb `Routes.GatewayId → igw` navigable (`core/aws/rtb_related.go`, `core/aws/catalog_networking.go`), `cmd/snapshot/ec2_network.go` `captureIGW`, plus the helpers they call (`assertStruct`, `relatedResourcesFor`/`FetchRelatedTarget`, projection nav marking, TUI navigable-enter handler).

## Findings

### 1. P2 — Every route-table `GatewayId` is offered as an Internet Gateway link, including `local`, `vgw-…` and `vpce-…`

- **File/line**: `core/aws/catalog_networking.go:385` (`{FieldPath: "Routes.GatewayId", TargetType: "igw"}`); consumed unconditionally by `core/semantics/projection/generic.go:256-264` (marks any non-empty value at the composed path as navigable) and `internal/tui/app_stack.go:499-516` (sends `RelatedNavigate{TargetType: "igw", TargetID: value}`).
- **Trigger**: Open any route table's detail view. Every table has the `local` route (`GatewayId: local`); tables with a VPN route carry `GatewayId: vgw-…`, and tables with an S3/DynamoDB gateway endpoint carry `GatewayId: vpce-…`. Move the cursor onto one of those `GatewayId` lines and press Enter.
- **User impact**: The line renders as a navigable link to Internet Gateways. Enter opens a related Internet Gateways navigation for ID `local` / `vgw-…` / `vpce-…`. `igw` has no by-ID fetcher, so the handler falls back to the filtered-list path (`internal/tui/runtime_adapter_related.go:121-140`: flash "Resource local not in cache; loading igw list" and an igw list pre-filtered to that value), and the list comes up empty. The operator follows a link that looks valid, lands on an empty Internet Gateways list, and may conclude the gateway was deleted. The related panel is correct: `checkRTBIGW` (`core/aws/rtb_related.go:79`) keeps only `igw-` prefixed IDs. So the panel and the navigable field disagree about which `GatewayId` values are IGWs.
- **Fix direction**: Use one rule for "this GatewayId is an IGW" and have both the navigable and `checkRTBIGW` use it. For example, let a navigable field carry a value predicate or ID prefix (`igw-`) that projection checks before setting `IsNavigable`, so `local`, `vgw-` and `vpce-` values render as plain text.

### 2. P3 — The IGW→Route Tables pivot counts blackhole routes; the reverse pivot drops them

- **File/line**: `core/aws/igw_related.go:57-62` (`checkIGWRTB` matches every route with `GatewayId == igwID`, whatever its `State`) vs `core/aws/rtb_related.go:74-77` (`checkRTBIGW` skips `ec2types.RouteStateBlackhole`).
- **Trigger**: Detach an internet gateway from its VPC and leave the `0.0.0.0/0 → igw-…` route in place. AWS marks that route `blackhole`, and the gateway itself still exists.
- **User impact**: The IGW detail shows "Route Tables (1)" next to its `no VPC attachments` warning, which suggests the gateway still carries traffic for that table. Pivot to the route table and its Internet Gateways panel shows 0, so the two ends of the same edge disagree. The spec says this pivot answers "whether this IGW is actually carrying internet traffic" (docs/resources/igw.md §2 `rtb`), and a blackhole route carries none.
- **Fix direction**: Put the edge rule (route targets this IGW and is not blackhole) in one shared predicate and call it from both checkers, so the counts in the two directions always match.

## kinesis

# kinesis — production-code review

Scope: `core/aws/kinesis*.go`, the kinesis entry in `core/aws/catalog_messaging.go`, `core/config/defaults_messaging.go` (kinesis), shared helpers those paths call (`related_common.go`), and the inbound pivots that navigate to kinesis (`lambda_related_extra.go`, `ddb_related.go`, `logs_related.go`, `eb_rule_related.go`).

## 1. P2: The CloudFormation pivot reads only the first page of stream tags

- **File/line**: `core/aws/kinesis_related.go:52-64` (`checkKinesisCFN`)
- **Trigger**: a stream with more tags than `ListTagsForStream` returns in one response. The call sets no `Limit` and ignores `HasMoreTags`/`ExclusiveStartTagKey`. The API allows up to 50 tags per stream and caps `Limit` at 50. Tags are paged by key, so `aws:cloudformation:stack-name` (lowercase `a`) sorts after the capitalised tags such as `Environment`, `Name` and `Owner`, which makes it one of the tags most likely to fall off the first page. I did not verify the server default page size; the defect applies whenever `HasMoreTags=true`.
- **Impact**: a stream that CloudFormation manages shows `CloudFormation (0)` as a proven zero, not as "unknown/truncated". The operator is told the stream has no owning stack.
- **Fix**: loop while `HasMoreTags`, passing `ExclusiveStartTagKey` = the last key and `Limit: 50`. Alternatively, if the loop stops early, return a truncated or unknown result rather than `KnownRelated(nil,false)`.

## 2. P2: The Lambda-consumer pivot misses enhanced fan-out (EFO) consumers

- **File/line**: `core/aws/kinesis_related.go:29-31` → `core/aws/related_common.go:300-303` (`ListEventSourceMappings` with `EventSourceArn` = the stream ARN)
- **Trigger**: a Lambda event-source mapping created against a Kinesis **stream consumer** ARN (`arn:aws:kinesis:<r>:<acct>:stream/<name>/consumer/<consumer>:<ts>`). Lambda documents EventSourceArn for Kinesis as "the ARN of the data stream or a stream consumer". Filtering on the stream ARN alone does not return mappings whose source is a consumer ARN.
- **Impact**: `Lambda Functions` undercounts, and can show a proven `(0)`, for streams whose consumers use EFO. EFO is the standard setup for low-latency or multi-consumer streams, so the check returns a wrong answer to "who is reading this stream?"
- **Fix**: also cover consumer-sourced mappings. Either call `ListStreamConsumers(StreamARN)` and query ESMs per consumer ARN, or list ESMs and match `EventSourceArn == streamARN || HasPrefix(EventSourceArn, streamARN+"/consumer/")`.

## 3. P2: The Lambda → Kinesis pivot builds a bad stream ID from a consumer ARN

- **File/line**: `core/aws/lambda_related_extra.go:218-224` (`checkLambdaKinesis`)
- **Trigger**: a Lambda function whose event-source mapping points at an EFO consumer ARN (`.../stream/orders/consumer/app1:1614000000`). `strings.LastIndex(arn, "/")` takes the text after the last slash, giving `app1:1614000000` rather than `orders`.
- **Impact**: the Lambda detail view shows a Kinesis related row whose ID matches no stream. Drilling into it opens an empty or unresolvable target instead of the source stream.
- **Fix**: take the stream name as the segment after `:stream/` and before the next `/`. For example, `strings.Cut(arn, ":stream/")` followed by `strings.Cut(rest, "/")`, the same parse `logs_related.go`/`eb_rule_related.go` use plus the cut at the first `/`.

## 4. P2: The alarm pivot counts alarms from any namespace that has a `StreamName` dimension

- **File/line**: `core/aws/kinesis_related.go:19` (`alarmIDsByDimension(..., "", "StreamName", res.ID)`)
- **Trigger**: an account with a Kinesis Video stream (namespace `AWS/KinesisVideo`, dimension `StreamName`) or a custom-namespace metric that shares a name with a Kinesis data stream. Passing an empty namespace turns off the namespace guard. Spec §2 `alarm` requires `Namespace == "AWS/Kinesis"`.
- **Impact**: `CW Alarms (N)` on the data stream includes alarms that watch a different resource. The operator drills into alarms unrelated to the stream's health.
- **Fix**: pass `"AWS/Kinesis"` as the namespace argument.

## 5. P3: The DynamoDB ↔ Kinesis pivots treat disabled streaming destinations as live links

- **File/line**: `core/aws/kinesis_related.go:157-161` (`checkKinesisDDB`) and `core/aws/ddb_related.go:132-141` (`checkDdbKinesis`, the inbound pivot to kinesis)
- **Trigger**: a table where Kinesis streaming was enabled and later disabled, or failed to enable. `DescribeKinesisStreamingDestination` still returns the destination with `DestinationStatus` `DISABLED`, `DISABLING` or `ENABLE_FAILED` (valid values: `ENABLING | ACTIVE | DISABLING | DISABLED | ENABLE_FAILED | UPDATING`). Neither checker reads `DestinationStatus`.
- **Impact**: the kinesis detail shows the table as feeding the stream, and the table detail shows the stream as its sink, although no change data flows. This misleads an operator tracing where the records come from.
- **Fix**: skip destinations whose `DestinationStatus` is `DISABLED`, `DISABLING` or `ENABLE_FAILED` in both checkers. One shared helper covers both directions.

## 6. P3: The kinesis → ddb pivot makes one API call per cached table, in sequence, with no cap

- **File/line**: `core/aws/kinesis_related.go:143-163`, registered at `core/aws/catalog_messaging.go:476`
- **Trigger**: opening any stream's detail view with a large `ddb` cache, for example several hundred tables. The checker calls `DescribeKinesisStreamingDestination` once per table, one after another, each wrapped in `RetryOnThrottle`, with no cap and no parallelism. Spec §2 `ddb` and §5 record this pivot as out of scope for exactly this N-table cost.
- **Impact**: the `ddb` related row stays pending for a long time on every stream detail open. In accounts with many tables it can hit throttling or the check timeout and show an error instead of a count.
- **Fix**: cap the scan (and mark the rest truncated) and run it in parallel with `ForEachParallel`, or drop the pivot as the spec directs.

## 7. P3: The related row is labelled "DynamoDB Streams" but lists DynamoDB tables

- **File/line**: `core/aws/catalog_messaging.go:476` (`DisplayName: "DynamoDB Streams"`)
- **Trigger**: any kinesis detail view.
- **Impact**: the row points at `ddb` (tables), and the checker returns table names (`kinesis_related.go:159`). "DynamoDB Streams" is a separate AWS feature, unrelated to Kinesis streaming destinations, so the label misleads the operator about what the drill opens.
- **Fix**: rename the row to "DynamoDB Tables".

## kms

# kms — production-code review

Scope: `core/aws/kms.go`, `kms_related.go`, `kms_issue_enrichment.go`, `kms_codes.go`, `kms_interfaces.go`, the `kms` entry in `core/aws/catalog_secrets.go`, and the direct dependencies each finding relies on (`core/resource/related.go` `buildFilterFromKey`, `core/aws/related_common.go` `relatedResult`, `core/aws/issue_enrichment.go` `MarkSkipped`, `core/iampolicy/evaluate.go`).

## 1. P2: the CloudTrail pivot filters on the bare key UUID, but KMS records the key ARN as the resource name

- **File/line**: `core/aws/catalog_secrets.go:148` (`CloudTrailKey: "ResourceName:ID"`), resolved by `core/resource/related.go:765-766`, which uses `res.ID`. `FetchKMSKeysPage` sets `ID` to the bare `KeyId` at `core/aws/kms.go:95`, and no `arn` field is stored (`kms.go:98-103`).
- **Trigger**: open any KMS key's detail and follow the "CloudTrail Events" related row.
- **User impact**: `LookupEvents` runs with `ResourceName=<uuid>`. The AWS KMS developer guide ("Logging AWS KMS API calls with AWS CloudTrail", attribute table) says the Resource name is the "Key ARN (or key ID and key ARN)". Events that carry only the ARN are therefore missed, and the audit trail for the key can look empty or incomplete. The spec (docs/resources/kms.md §2 `ct-events`) says to filter by the key's `Arn`.
- **Fix direction**: in the fetchers (`kms.go:98`, `kms.go:223`), store `meta.Arn` as a field such as `arn`, and set `CloudTrailKey: "ResourceName:Fields.arn"`. This is the pattern `secrets` already uses at `catalog_secrets.go:53`.

## 2. P2: the "IAM Roles (grants)" count reads only the first ListGrants page but is reported as exact

- **File/line**: `core/aws/kms_related.go:224-252` (a single `ListGrants` call that ignores `Truncated`/`NextMarker`) and `kms_related.go:254` (`relatedResult`, which is `KnownRelated(..., truncated=false)` per `related_common.go:190-192`).
- **Trigger**: a key with more grants than one page holds (the default page is 50). This is common for keys used by EBS, RDS or Auto Scaling, which create a grant per attachment.
- **User impact**: roles granted access on later pages are silently left out. The panel shows an exact, too-low count with no "+" marker, which gives a wrong answer to "who can use this key?" during a permissions audit.
- **Fix direction**: loop `ListGrants` on `NextMarker` while `Truncated` is true, the same way `buildKMSAliasMap` does. If paging has to be capped, return `relatedResultTrunc(..., true)`.

## 3. P2: rotation enrichment treats keys that cannot rotate as failures

- **File/line**: `core/aws/kms_issue_enrichment.go:62-74`. Every non-AccessDenied error from `GetKeyRotationStatus` goes to `MarkSkipped`, which records a `FailedCall` (`issue_enrichment.go:221-227`). That failure is returned through `AggregateFailures` at `kms_issue_enrichment.go:86`.
- **Trigger**: a customer-managed key with imported key material (`Origin=EXTERNAL`) or a key in a custom key store. AWS rejects `GetKeyRotationStatus` for these with `UnsupportedOperationException` ("origin is EXTERNAL which is not valid for this operation"). Automatic rotation is not supported on them, per the GetKeyRotationStatus API reference.
- **User impact**: each such key is marked uninspected and reported as a failed call. The operator sees a partial-failure error for the kms list every time it loads, even though nothing went wrong and nothing can be fixed.
- **Fix direction**: before calling `GetKeyRotationStatus`, check `KeyMetadata.Origin` (`EXTERNAL`, `AWS_CLOUDHSM`, `EXTERNAL_KEY_STORE`) and `KeySpec`/`KeyUsage` from the stashed `RawStruct`. Alternatively, treat `UnsupportedOperationException` as a complete "rotation not applicable" answer rather than a failure.

## 4. P2: a denied GetKeyRotationStatus on a customer key is silently treated as nothing to report

- **File/line**: `core/aws/kms_issue_enrichment.go:67-71` (`if IsAccessDenied(err) { return }`) and the doc comment at `:29-30`.
- **Trigger**: the browsing role lacks `kms:GetKeyRotationStatus`, or the key policy denies it, on customer-managed keys. These are the only keys `FetchKMSKeysPage` lists (`kms.go:77-79`).
- **User impact**: the key gets no rotation finding, an empty Rotation column, no uninspected marker and no failure. It is indistinguishable from a key whose rotation is fine, so "unknown" is shown as "healthy". The stated reason, that AWS-managed keys reject the call with AccessDenied, is wrong: the GetKeyRotationStatus API reference says "The key rotation status for AWS managed KMS keys is always `true`." AWS-managed keys are also filtered out of the list before enrichment runs.
- **Fix direction**: handle AccessDenied like any other error (`MarkSkipped`) so the row is marked uninspected. If AWS-managed keys should be skipped, skip them explicitly by `KeyManager` (see finding 5).

## 5. P3: the AWS-managed and pending-deletion guards never match, because they assert a value type while RawStruct holds a pointer

- **File/line**: `core/aws/kms_issue_enrichment.go:123` and `:135` (`r.RawStruct.(kmstypes.KeyMetadata)`). Both fetchers store the pointer `descOutput.KeyMetadata`/`out.KeyMetadata` (`*kmstypes.KeyMetadata`) at `core/aws/kms.go:104` and `kms.go:229`. Nothing in between dereferences `RawStruct`. `domain.Resource.Sanitized` keeps the type, and the on-disk cache carries no `RawStruct`.
- **Trigger**: any key that is `PendingDeletion` or `PendingReplicaDeletion`, or an AWS-managed key added through `FetchKMSKeysByIDs`, reaches `kmsKeyPolicyIsPublic`.
- **User impact**: `kmsKeyIsGoingAway` and `kmsKeyIsAWSManaged` always return false. Keys that are pending deletion still get a `GetKeyPolicy` call and can get the red "key policy open to anyone" finding, which the guard at `:95` exists to suppress. AWS-managed keys still get a `GetKeyPolicy` call, and any error from it is recorded as a failure.
- **Fix direction**: use `assertStruct[kmstypes.KeyMetadata](r.RawStruct)` (`related_common.go:72`), which accepts both value and pointer.

## 6. P3: keys pending deletion get a spurious "key rotation disabled" warning

- **File/line**: `core/aws/kms_issue_enrichment.go:62-84`. Rotation is checked for every key regardless of `KeyState`.
- **Trigger**: a customer key in `PendingDeletion` that had rotation enabled.
- **User impact**: per the GetKeyRotationStatus API reference, "While a KMS key is pending deletion, its key rotation status is `false`". The row therefore gets a yellow "key rotation disabled" finding and `rotation_enabled=false` next to "pending deletion". This tells the operator to turn on rotation for a key they are deleting, and it misstates the key's real rotation setting, which comes back if the deletion is cancelled.
- **Fix direction**: skip the rotation call, or at least the finding, when `kmsKeyIsGoingAway(r)` is true. This depends on the fix in finding 5.

## lambda

# lambda — production-code review

Scope: `core/aws/lambda*.go`, the `lambda`, `lambda_invocations` and `lambda_invocation_logs` catalog entries in `core/aws/catalog_compute.go`, and the lambda view defaults in `core/config/defaults_compute.go`, plus the direct dependencies needed to check them (`assertStruct`, `kmsKeyIDFromField`, the msk, efs and kinesis resource IDs).

AWS behaviour was checked against the current AWS docs: ListFunctions, the Lambda runtimes page, FilterLogEvents, the log-format and log-group pages, and function URL auth.

## Findings

### 1. P1: The list never sees a function's lifecycle state or update status

- **Where**: `core/aws/lambda.go:91-92`, `core/aws/lambda.go:111-121`
- **Trigger**: any function whose state is `Pending` or `Failed`, or whose `LastUpdateStatus` is `Failed`. The AWS ListFunctions reference says the call "returns a subset of the FunctionConfiguration fields. To get the additional fields (State, StateReasonCode, StateReason, LastUpdateStatus, LastUpdateStatusReason, LastUpdateStatusReasonCode, RuntimeVersionConfig) … use GetFunction." The fetcher reads `fn.State` and `fn.LastUpdateStatus` straight from the ListFunctions page, so both are always empty.
- **User impact**:
  - The `Status` column (`state`, catalog_compute.go:572) is always blank.
  - `lambda.state.pending`, `lambda.state.failed`, `lambda.state.inactive` and `lambda.last-update.failed` can never fire.
  - A function that cannot be invoked, or whose deploy failed, shows as healthy. If it has no DLQ, it shows only the `no dead-letter queue configured` warning.
  - The main-menu badge undercounts broken functions.
- **Fix direction**: get these fields from GetFunction or GetFunctionConfiguration, for example as a bounded Wave-2 per-function enricher that writes `state` and `last_update_status` and emits the lifecycle findings. Remove the Wave-1 lifecycle branches that cannot fire. Update the spec's §1 claim ("all Wave 1 fields are already on the ListFunctions response") to match.

### 2. P1: The deprecated-runtime list is out of date

- **Where**: `core/aws/catalog_compute.go:24-43`
- **Trigger**: a function on any runtime that the AWS "Deprecated runtimes" table lists but the map leaves out:
  - `nodejs16.x`, `nodejs18.x`, `nodejs20.x`
  - `python3.8`, `python3.9`
  - `ruby3.2`
  - `dotnet6`, `dotnet7`, `dotnet5.0`
  - `provided`, `provided.al2`
  - `nodejs4.3-edge`
- **User impact**: functions on end-of-life runtimes that get no patches render green, with no `runtime is end-of-life` finding. These runtimes (python3.8/3.9, nodejs16/18/20) are among the most common in real accounts.
- **Fix direction**: add the missing identifiers from the current AWS table. Consider keeping the list with deprecation dates, so an entry takes effect when its date passes instead of waiting for a code edit.

### 3. P2: The invocations child view shows the oldest invocations of the last 24h, not the newest

- **Where**: `core/aws/lambda_invocations.go:64-77`, with the reversal at `:109-112` and `:130-133`
- **Trigger**: a function with more than 50 REPORT lines in the last 24h. `FilterLogEvents` is called without `StartFromHead=false`. The AWS docs say: "By default, the events are returned in ascending timestamp order (oldest first)." The code collects the first ~50 events, which are the oldest in the window, and then reverses only that slice.
- **User impact**: when you press enter on a busy function, you see invocations from about 24h ago, labelled newest-first. The invocation that just failed is several "load more" pages away, and you can't reach it within the 100-page scan cap.
- **Fix direction**: set `StartFromHead: aws.Bool(false)` so AWS returns the newest events first, and drop the manual reversal. Continuation tokens keep that direction.

### 4. P2: Functions using the JSON log format show zero invocations

- **Where**: `core/aws/lambda_invocations.go:71`, with the text-only parser at `:34-40`
- **Trigger**: a function with `LoggingConfig.LogFormat == JSON`. This is recommended by AWS and mandatory for Lambda Managed Instances. The AWS docs say that with JSON format "each system log item (platform event) is captured as a JSON object" with `type`/`record` keys (`platform.report`). The `"REPORT RequestId"` filter pattern and the plain-text regex never match.
- **User impact**: the invocations list is empty for an actively invoked function, which looks like "no traffic".
- **Fix direction**: branch on the function's `LoggingConfig.LogFormat`, which needs to be passed through the parent context. For JSON, filter on `{ $.type = "platform.report" }` and parse `record.requestId`, `record.metrics.*` and `record.status`.

### 5. P2: The MSK pivot returns the cluster UUID instead of the cluster name

- **Where**: `core/aws/lambda_related_extra.go:259-260`
- **Trigger**: a function with an MSK event-source mapping. The MSK ARN has the form `arn:aws:kafka:<region>:<acct>:cluster/<clusterName>/<uuid>`. `LastIndex("/")` picks `<uuid>`, but msk resources are keyed by cluster name (`core/aws/msk.go:78`, `ID: clusterName`).
- **User impact**: the MSK row shows a count, but drilling in matches no MSK cluster.
- **Fix direction**: take the segment after `cluster/` and before the next `/`, which is the cluster name.

### 6. P2: The EFS pivot returns access-point IDs, but EFS rows are file-system IDs

- **Where**: `core/aws/lambda_related_extra.go:58-66`
- **Trigger**: any function with `FileSystemConfigs`. The ARN is an access point (`…:access-point/fsap-…`), and the checker returns `fsap-…`. EFS rows are keyed by `fsID` (`core/aws/efs.go:102`), and no production code translates fsap to fs. The comment's "downstream routing can translate" has nothing behind it.
- **User impact**: `EFS File Systems (1)` is shown, but drilling in finds nothing. The operator can't reach the mounted file system.
- **Fix direction**: resolve each access point with `efs:DescribeAccessPoints(AccessPointId)`, which is one call per mount and at most 5 per function, and return its `FileSystemId`.

### 7. P2: The API Gateway pivot matches on the API name or a tag key, not on integrations

- **Where**: `core/aws/lambda_related_extra.go:98-106`
- **Trigger**:
  - A function named, for example, `api`, `auth` or `handler` "matches" every API whose name contains that substring, or that has a tag keyed by the function name.
  - An API that actually integrates the function under an unrelated name is never matched.
  - The spec requires matching the integration `Uri` against `/functions/<FunctionArn>/invocations`.
- **User impact**: the related panel shows wrong API Gateways (false positives) and misses the real ones.
- **Fix direction**: match on integration URIs. Either use an integration-URI field that the apigw fetcher already exposes, or make bounded `GetIntegrations` calls per API, marked truncated on failure. Remove the name and tag heuristic.

### 8. P2: The SQS and SNS pivots ignore the function's own dead-letter target

- **Where**: `core/aws/lambda_related.go:135-171` (`checkLambdaSQS`), `core/aws/lambda_related_extra.go:406-443` (`checkLambdaSNS`)
- **Trigger**: a function with `DeadLetterConfig.TargetArn` set to an SQS queue or SNS topic. The spec (§2 `sqs` pivot (a), `sns` pivot (a)) requires including it. The fetcher already reads it into `Fields["dlq_target_arn"]` (lambda.go:79-82), but neither checker looks at it.
- **User impact**: `SQS Queues (0)` or `SNS Topics (0)` is shown for a function whose DLQ is exactly that queue or topic. The operator can't jump to the DLQ during an incident.
- **Fix direction**:
  - In `checkLambdaSQS`, add the queue name from the DLQ ARN when its service is `sqs`.
  - In `checkLambdaSNS`, add the DLQ topic ARN when its service is `sns`.
  - De-duplicate against the event-source and subscription matches.

### 9. P3: The invocations list mixes in other functions that share a custom log group

- **Where**: `core/aws/lambda_invocations.go:69-77`
- **Trigger**: several functions write to one custom `LoggingConfig.LogGroup`. AWS documents this: "You can configure multiple Lambda functions to send logs to the same CloudWatch log group". Stream names in that case are `YYYY/MM/DD/<function_name>[<version>][<guid>]`. The fetch filters only by log group.
- **User impact**: function A's invocations list shows function B's invocations, timeouts and memory figures as if they were A's.
- **Fix direction**: when the log group is not the default `/aws/lambda/<name>`, keep only events whose `LogStreamName` contains `/<function_name>[`.

### 10. P3: A Kinesis enhanced fan-out mapping gives the consumer name instead of the stream name

- **Where**: `core/aws/lambda_related_extra.go:222-224`
- **Trigger**: an event-source mapping on a stream consumer. The ARN has the form `arn:aws:kinesis:<region>:<acct>:stream/<stream>/consumer/<consumer>:<ts>`. `LastIndex("/")` picks `<consumer>:<ts>`, while kinesis rows are keyed by stream name (`core/aws/kinesis.go:74`).
- **User impact**: the Kinesis count includes an ID that no stream matches, so the drill is empty for functions that use enhanced fan-out.
- **Fix direction**: take the segment right after `stream/`, up to the next `/`.

### 11. P3: The "function endpoint open without authentication" finding ignores the resource policy

- **Where**: `core/aws/lambda_issue_enrichment.go:157-166`
- **Trigger**: a function URL with `AuthType == NONE` whose resource policy does not grant `lambda:InvokeFunctionUrl` (and `lambda:InvokeFunction`) to `*`. AWS docs: "If a function's resource-based policy doesn't grant lambda:invokeFunctionUrl and lambda:InvokeFunction permissions, users get a 403 Forbidden error code … even if the function URL uses the NONE auth type."
- **User impact**: a Broken finding says anyone on the internet can invoke the function, when the endpoint actually returns 403. This is a false alarm. The GetPolicy call in the same goroutine already has the data to tell the cases apart.
- **Fix direction**: raise `lambda.function-url-public` only when the URL auth is NONE *and* the evaluated policy publicly allows `lambda:InvokeFunctionUrl`. Otherwise downgrade the finding or drop it.

## Summary

11 findings: P0 0, P1 2, P2 6, P3 3.

## logs

# logs — production-code review

Scope: `core/aws/cwlogs.go`, `logs_interfaces.go`, `logs_related.go`, `logs_issue_enrichment.go`, `log_streams.go`, `log_events.go`, `catalog_monitoring.go` (logs, log_streams, log_events entries), `core/config/defaults_monitoring.go`, plus the shared helpers they call (`alarmIDsByDimension`, `capAtEnrichmentCap`, `markUninspected`, list/detail "not inspected" rendering, `runtime/detail_op.go`, `runtime/wave2_carry.go`). SDK checked: `cloudwatchlogs@v1.88.0`.

## P2

### 1. Audit-group detection never matches a CloudTrail-delivered log group

- **File**: `core/aws/logs_issue_enrichment.go:102`
- **Trigger**: A trail delivers to CloudWatch Logs under any name AWS documents or the console proposes, such as `CloudTrail/logs`, `CloudTrail/DefaultLogGroup`, or `aws-cloudtrail-logs-<acct>-<id>`. The enricher only checks names that start with `/aws/cloudtrail/`. AWS never assigns that prefix: the CloudTrail docs recommend `CloudTrail/logs`.
- **Impact**: The `logs.missing-metric-filters` finding practically never fires, so an audit group with no metric filter shows as healthy. The only Wave 2 signal for the type is dead on real accounts.
- **Fix**: Identify audit groups from the trail list (`Trail.CloudWatchLogsLogGroupArn`). Another option is the stream name the enricher already reads, since CloudTrail names its streams `<account>_CloudTrail_<region>`. Do not rely on a name prefix.

### 2. Rows past the 50-row enrichment cap show "not inspected" for a check that never applies to them

- **File**: `core/aws/logs_issue_enrichment.go:54` (with `issue_enrichment.go:388-399`, `core/app/list_body.go:620-622`)
- **Trigger**: The account has more than 50 log groups loaded. `capAtEnrichmentCap` runs on the whole list before the audit-prefix filter. Every row from index 50 on is marked uninspected (`CheckCap`), including `/aws/lambda/...` and other non-audit groups that the metric-filter check never evaluates. Audit groups that sort after position 50 are also never checked.
- **Impact**: The Status cell of most log groups in a large account reads "not inspected", which is noise on healthy rows. Those rows also never get `Last Event`. A real audit group past the cap is not checked during the sweep.
- **Fix**: Filter to audit groups first and apply the cap to that subset only. Keep the `last_event_at` fetch as a separate bounded pass that does not mark rows uninspected.

### 3. The "CW Alarms" pivot misses metric-filter alarms

- **File**: `core/aws/logs_related.go:55-57`
- **Trigger**: An alarm watches a metric that a metric filter on this log group produces. This is the canonical log→alarm workflow in the spec (§2 `alarm`: `DescribeMetricFilters` → `metricName`/`metricNamespace` → alarm). The checker only matches alarms that carry a `LogGroupName` dimension. Only the `AWS/Logs` per-group metrics (IncomingBytes and similar) have that dimension. Metric-filter metrics are custom metrics without it.
- **Impact**: The panel shows 0 alarms for a log group that alarms do watch. The operator concludes nothing alerts on it.
- **Fix**: Call `DescribeMetricFilters(logGroupName)`. Match loaded alarms on `MetricName` + `Namespace` from each `metricTransformations[]` entry. Keep the `LogGroupName` dimension match as an additional path.

### 4. The "S3 (exports)" pivot can never return a bucket

- **File**: `core/aws/logs_related.go:207-237`
- **Trigger**: Any log group. The checker looks for S3 ARNs among subscription-filter `destinationArn` values. Subscription filters only accept Kinesis Data Streams, Firehose, Lambda, or a logical destination, never S3, so `ARNForService(..., "s3")` never matches. The comment about a "direct S3 destination for newer filter features" describes something that does not exist. The spec requires `DescribeExportTasks` filtered by log group, using each task's `destination` bucket.
- **Impact**: Every log group shows a known 0 for S3 exports, even when export tasks archived it to S3. This is a false negative presented as a definite answer.
- **Fix**: Use `DescribeExportTasks` (add it to `CWLogsAPI`), filter by `logGroupName`, and return the `Destination` bucket names.

### 5. The log events view shows only one page and cannot load older events

- **File**: `core/aws/log_events.go:91-157`
- **Trigger**: A stream holds more than one `GetLogEvents` page (up to 10,000 events or 1 MB). `continuationToken` is accepted but ignored. `NextBackwardToken`/`NextForwardToken` are dropped. The result is hard-coded to `IsTruncated: false` and `TotalHint: len(resources)`.
- **Impact**: The operator sees only the newest page and is told it is complete. Older events in the stream cannot be reached, and nothing indicates they exist.
- **Fix**: Pass `continuationToken` as `NextToken` and return `NextBackwardToken` as the next token when it differs from the token that was sent. Set `IsTruncated` from that.

## P3

### 6. Lambda pivot ignores `LoggingConfig.LogGroup` and subscription-filter consumers

- **File**: `core/aws/logs_related.go:20-51`
- **Trigger**: A function logs to a custom group through `FunctionConfiguration.LoggingConfig.LogGroup`, which `ListFunctions` returns. A Lambda can also be a subscription-filter destination (spec §2 `lambda` (b)). The checker only parses the `/aws/lambda/<name>` naming convention.
- **Impact**: A custom-named group shows 0 Lambdas although a function writes to it. A function that was redirected to another group is still linked to `/aws/lambda/<name>`.
- **Fix**: Match loaded functions on `LoggingConfig.LogGroup`, and on the default name only when `LoggingConfig.LogGroup` is unset. Add Lambda ARNs from `DescribeSubscriptionFilters` (already fetched by `logsSubscriptionFilters`).

### 7. The KMS pivot reports "unknown" for a cache-seeded row whose Fields already hold the key

- **File**: `core/aws/logs_related.go:62-72`
- **Trigger**: Detail view on a row seeded from the disk cache. According to `runtime/detail_op.go:110-112`, such rows carry no RawStruct. The related check still runs. `checkLogsKMS` reads only `RawStruct.KmsKeyId` and returns `UnknownRelated` when it is nil, although the fetcher wrote `Fields["kms_key_id"]`.
- **Impact**: The KMS Key pivot shows unknown instead of the key until a live refetch.
- **Fix**: Fall back to `res.Fields["kms_key_id"]`. The value is a proven empty when `Fields["encryption"] == "none"`.

### 8. `last_event_at` is a relative string frozen at enrichment time, persisted, and sorted as text

- **File**: `core/aws/logs_issue_enrichment.go:78-95`; column at `core/aws/catalog_monitoring.go:127`
- **Trigger**: The enricher writes values like `"5m ago"` or `"3h ago"`. The field is listed in `IssueEnricherFieldKeys`, so `runtime/wave2_carry.go` carries it into the on-disk cache. The column has no `SortKey`.
- **Impact**: A later session, or the same session hours later, still shows "5m ago" for events that are hours or days old. Sorting by Last Event is lexical: "10m ago" sorts before "2h ago", and date strings sort against "Nd ago" strings arbitrarily.
- **Fix**: Store an absolute timestamp, formatted like `creation_time`, plus a raw epoch sort key. Render relative time at display time if it is wanted.

### 9. DescribeLogStreams failures in the enricher are silent and not retried

- **File**: `core/aws/logs_issue_enrichment.go:73-75`, `138-151`
- **Trigger**: `DescribeLogStreams` is throttled or denied. The call does not go through `RetryOnThrottle`, unlike the sibling `DescribeMetricFilters` call at line 106. Any error is dropped.
- **Impact**: Last Event stays blank, which looks the same as a group with no events. The operator gets no signal that the read failed.
- **Fix**: Wrap the call in `RetryOnThrottle`. Record failures so the cell or detail can say the value was not read.

### 10. Infrequent Access log groups: log events drill-down errors and metric-filter advice is impossible

- **File**: `core/aws/catalog_monitoring.go:130-135` and `302-307` (child navigation); `core/aws/logs_issue_enrichment.go:118-125`
- **Trigger**: A log group with `LogGroupClass=INFREQUENT_ACCESS`. AWS does not support `GetLogEvents`/`FilterLogEvents` or metric filters on that class (CloudWatch Logs "Log classes" feature table).
- **Impact**: Enter → stream → Enter produces an API error instead of an explanation. An IA audit group, if it were detected, would get "add metric filters", which cannot be done on that class.
- **Fix**: Read `LogGroupClass` from the row. Block or explain the events drill-down for IA groups, and skip the metric-filter finding for them.

### 11. The log_events detail references a field that does not exist on the SDK type

- **File**: `core/config/defaults_monitoring.go:52`
- **Trigger**: The detail view of any log event. `{Path: "EventId"}` is listed, but `cwlogstypes.OutputLogEvent` (v1.88.0) has only `IngestionTime`, `Message`, and `Timestamp`.
- **Impact**: The Event ID detail row never resolves to a value.
- **Fix**: Remove the path, or use `{Key: "event_id"}` if the synthetic ID is meant to be shown.

## lt

# lt — production-code review

Scope: `core/aws/lt.go`, `core/aws/lt_issue_enrichment.go`, `core/aws/lt_related.go`, the `lt` catalog entry in `core/aws/catalog_compute.go`, the `lt` view defaults in `core/config/defaults_compute.go`, plus the helpers they call (`degradedDetailsFinding`, `cachedTypedRows`, `assertStruct`, `KnownRelated`, `kmsKeyIDFromField`, `decodeUserData`, `setWave2Finding`, `BuildResourceCacheSnapshot`).

## 1. P2: "IMDSv1 allowed" fires when HttpTokens is unset, but AWS does not always resolve unset to optional

- **File/line:** `core/aws/lt.go:170-171` (with the finding text at `core/aws/catalog_compute.go:907`)
- **What the code does:** it raises `lt.warn.imdsv1` whenever `MetadataOptions == nil` or `HttpTokens != required`. It treats an unset value as `optional`.
- **What AWS says:** the SDK in use (`service/ec2` v1.332.0, `types.LaunchTemplateInstanceMetadataOptionsRequest.HttpTokens`) states: "Default: If the value of ImdsSupport for the Amazon Machine Image (AMI) for your instance is v2.0, the default is required." The account-level IMDS defaults set by `ModifyInstanceMetadataDefaults` also apply when a template leaves the field unset. Leaving it unset does not always mean IMDSv1 is allowed. The spec's claim that unset "defaults to optional (SDK-confirmed)" is contradicted by the SDK itself.
- **Trigger:** a template whose `$Default` version omits `MetadataOptions`/`HttpTokens` and boots an AMI registered with `ImdsSupport=v2.0` (for example, the standard AL2023 AMIs). It also fires in an account whose IMDS default is set to require tokens.
- **User impact:** the row turns Warning with `IMDSv1 allowed` and a remediation sentence, but instances launched from the template actually require IMDSv2. This false positive appears on common, healthy templates.
- **Fix direction:** fire only on an explicit `HttpTokens == optional`. For an unset value, resolve through the loaded `ami` cache: fire only when `Image.ImdsSupport` is not `v2.0`. If the AMI is not in the cache, treat the result as unknown, not as a finding. Update the spec's §3.1 premise to match.

## 2. P3: The "EBS encryption disabled" detail wrongly says the volume is unencrypted however the account default is set

- **File/line:** `core/aws/catalog_compute.go:908` (the signal is emitted by `core/aws/lt.go:176-181`)
- **What the text says:** "…so every instance launched from it gets an unencrypted volume however the account default is configured."
- **What AWS says:** the EBS User Guide (encryption-by-default page) says: "If you enable it for a Region, you cannot disable it for individual volumes or snapshots in that Region." An explicit `Encrypted=false` in a template does not produce an unencrypted volume when encryption-by-default is on.
- **Trigger:** a region with EBS encryption-by-default enabled, and a template that has a block device mapping with `Ebs.Encrypted=false`.
- **User impact:** the detail/S5 text tells the operator their volumes are unencrypted when AWS actually encrypts them. The operator gets a false security claim and does remediation work they don't need.
- **Fix direction:** reword the detail so it doesn't claim the result is independent of the account default (for example: "…launches unencrypted volumes unless the Region enforces encryption by default"). Alternatively, qualify the signal with `GetEbsEncryptionByDefault`.

## 3. P3: The deprecated-AMI check silently finds nothing when the `ami` cache entry is disk-seeded

- **File/line:** `core/aws/lt_issue_enrichment.go:74-81`
- **What the code does:** it treats `cache["ami"]` as loaded if the entry exists. It then keeps only rows whose `RawStruct` asserts to `ec2types.Image`. The code ignores `ResourceCacheEntry.FieldsOnly`. `BuildResourceCacheSnapshot` (`core/runtime/probes.go:1101`) sets that flag for disk-seeded entries, which carry no `RawStruct` (`core/cache/cache.go:48`, C6).
- **Trigger:** the `ami` RowStore entry is still the disk-restored one when `lt`'s Wave 2 runs. For example, the ami cache file exists from a previous session and this session's ami probe failed or hasn't replaced it.
- **User impact:** every row is skipped, so no `deprecated AMI` finding is produced. Because Wave 2 replaces the previous Wave 2 findings on each run, a `deprecated AMI` Warning that an earlier pass had shown disappears. The template reads as healthy even though the disk rows still carry `Fields["deprecated"] = "yes (…)"`.
- **Fix direction:** when `amiEntry.FieldsOnly` is set, either read the ami row's `Fields["deprecated"]` (it starts with "yes" when the deprecation time has passed) or skip the enrichment and mark the lt rows truncated/unknown instead of answering clean.

## msk

# msk — production-code review

Scope: `core/aws/msk.go`, `msk_related.go`, `msk_issue_enrichment.go`, `msk_interfaces.go`, `msk_codes.go`, the `msk` entry in `core/aws/catalog_messaging.go`, `core/config/defaults_messaging.go` (`msk`), and the shared helpers they call (`related_common.go`, related-navigation resolution in `core/runtime/handlers_related.go`, `core/app/list_state.go`, `core/app/handle.go`). SDK checked: `github.com/aws/aws-sdk-go-v2/service/kafka v1.65.0`.

## Findings

### 1. P2: The Secrets pivot returns IDs that never match a secrets row

- **File:** `core/aws/msk_related.go:217-223`
- **Trigger:** An MSK cluster with SASL/SCRAM secrets. `ListScramSecrets` returns full secret ARNs, for example `arn:aws:secretsmanager:…:secret:AmazonMSK_app-AbCdEf`. The checker keeps everything after `:secret:`, so the ID becomes `AmazonMSK_app-AbCdEf`, which still carries the random 6-character suffix. The secrets fetcher keys each row by the bare secret name (`core/aws/secrets.go:71`, `ID: secretName`). Navigation matches on exact `r.ID` in three places: `relatedCacheHit` (`core/runtime/handlers_related.go:423-430`), `seedRelatedExactRows` (`core/app/list_state.go:291-299`) and the filter-text fallback.
- **Impact:** The panel shows "Secrets Manager (N)", but pressing Enter opens an empty list or an empty filtered list. The operator cannot reach the SCRAM secrets from the cluster.
- **Fix:** Pass the full secret ARN and resolve it against the secrets cache by `Fields["arn"]` / `SecretListEntry.ARN`, the same way `dbi_related.go:167-177` does. Do not derive a name by string-cutting the ARN.

### 2. P2: The KMS pivot passes the raw key ARN instead of the key ID

- **File:** `core/aws/msk_related.go:242`
- **Trigger:** Any provisioned cluster with encryption at rest. `DataVolumeKMSKeyId` is a key ARN (`arn:aws:kms:…:key/UUID`), and it is returned unchanged as the related ID. KMS rows are keyed by the bare key ID (`core/aws/kms.go:95,220`). The cache-hit path compares `r.ID == ARN` and never matches. The by-ID drill runs `DescribeKey(ARN)`, which returns a row with `ID = UUID`. `autoOpenSingleDetail` then looks for `Rows[i].ID == ARN` (`core/app/handle.go:706-710`), finds nothing, and falls back to a stub detail or the placeholder list. Every other KMS pivot (18 call sites) normalises the value with `kmsKeyIDFromField` or `arnLastSegment`. The navigable detail field is normalised through `NavIDFromValue`, but the related-panel path is not.
- **Impact:** Pressing Enter on "KMS Key (1)" never opens the real cached key detail. The operator does not see the key state (Disabled, PendingDeletion) that the pivot exists to show.
- **Fix:** `return relatedResult("kms", []string{kmsKeyIDFromField(*…DataVolumeKMSKeyId, res.Type)})`.

### 3. P2: Serverless clusters always report 0 security groups, subnets and VPCs

- **File:** `core/aws/msk_related.go:31-33` (sg), `:99-101` (subnet), `:114-116` (vpc)
- **Trigger:** A cluster with `ClusterType == SERVERLESS`. `Provisioned` is nil, so each of the three checkers returns `KnownRelated(…, nil)`, a proven zero. The network placement for these clusters is in `Cluster.Serverless.VpcConfigs[].SubnetIds` / `.SecurityGroupIds` (SDK `types.Serverless.VpcConfigs`, marked required). The spec (§1) says the configuration lives under `Provisioned` or `Serverless`.
- **Impact:** For every serverless cluster, the panel claims "Security Groups (0)", "Subnets (0)" and "VPC (0)". These are false definitive answers on the pivots operators use for "can't connect" triage.
- **Fix:** When `Provisioned == nil && Serverless != nil`, collect the union of `VpcConfigs[].SecurityGroupIds` and `VpcConfigs[].SubnetIds`, and derive the VPC from the first subnet as the provisioned path does.

### 4. P2: The "broker software outdated" cutoff (2.8) misses most end-of-support versions

- **File:** `core/aws/msk_issue_enrichment.go:141-142`
- **Trigger:** A provisioned cluster running Kafka 2.8.0, 2.8.1, 2.8.2.tiered, 3.1.1, 3.2.0, 3.3.x, 3.4.0, 3.5.1, 3.6.0 or 3.7.x. AWS lists all of these as past end of support as of 2026-09-18: 2.8.x and 3.1–3.3 on 2024-09-11, 2.8.2.tiered on 2025-01-14, 3.4.0 on 2025-08-04, 3.5.1 on 2025-10-23, 3.6.0 on 2026-06-01, 3.7.x on 2026-09-01 (source: docs.aws.amazon.com/msk/latest/developerguide/supported-kafka-versions.html). The hard-coded `major < 2 || (major == 2 && minor < 8)` flags only versions below 2.8.
- **Impact:** A cluster on an unsupported Kafka version shows no "broker software outdated" warning. That contradicts the finding's own detail text ("a Kafka version AWS no longer treats as current").
- **Fix:** Take the version status from AWS rather than a constant: `ListKafkaVersions` (`KafkaVersion.Status` ACTIVE/DEPRECATED), fetched once per enrichment pass. At minimum, move the cutoff to the current end-of-support boundary (below 3.8).

### 5. P3: The Secrets pivot calls `ListScramSecrets` for every cluster, even without SCRAM

- **File:** `core/aws/msk_related.go:210-215`
- **Trigger:** A cluster where `Provisioned.ClientAuthentication.Sasl.Scram.Enabled` is false or nil, or a serverless cluster. Spec §2 says the call is made "When `Enabled == true`" only. The code calls it for every cluster and turns any error into `ErrorRelated`.
- **Impact:** For a principal without `kafka:ListScramSecrets`, a cluster that does not use SCRAM at all shows an error on the Secrets row instead of a proven 0. It also costs one extra API call per opened cluster.
- **Fix:** Before the call, check `Provisioned.ClientAuthentication.Sasl.Scram.Enabled`. If it is not true, return `KnownRelated("secrets", nil, false)`.

### 6. P3: The Secrets pivot reads only the first `ListScramSecrets` page

- **File:** `core/aws/msk_related.go:210-224`
- **Trigger:** A cluster with more SCRAM secrets than one page holds. `ListScramSecretsOutput.NextToken` is ignored, and the result is returned as an exact, non-truncated count.
- **Impact:** An undercounted Secrets row presented as exact.
- **Fix:** Loop on `NextToken` (or use `kafka.NewListScramSecretsPaginator`), wrapped in `RetryOnThrottle` like the sibling Lambda call.

### 7. P3: The Lambda pivot reads only the first `ListEventSourceMappings` page

- **File:** `core/aws/related_common.go:297-301` (shared helper called from `core/aws/msk_related.go:54`)
- **Trigger:** A cluster whose event source mappings run past one page (100 by default). `NextMarker` is ignored, and the result is returned as exact (`relatedResult`, not truncated).
- **Impact:** "Lambda Functions (N)" undercounts consumers and presents the number as exact.
- **Fix:** Paginate on `NextMarker` (for example with `lambda.NewListEventSourceMappingsPaginator`).

### 8. P3: The Alarm pivot does not restrict to the `AWS/Kafka` namespace

- **File:** `core/aws/msk_related.go:21`
- **Trigger:** The checker passes `namespace = ""` to `alarmIDsByDimension`, so any alarm in any namespace with a `Cluster Name` dimension equal to the MSK cluster name matches. Custom-namespace metrics commonly use that dimension name. Spec §2 requires `Namespace == "AWS/Kafka"`.
- **Impact:** Alarms that do not watch this MSK cluster are counted and listed under "CW Alarms".
- **Fix:** `alarmIDsByDimension(ctx, clients, cache, "AWS/Kafka", "Cluster Name", res.ID)`.

### 9. P3: A FAILED cluster never shows its failure cause (`StateInfo` is ignored)

- **File:** `core/aws/msk.go:74-88`; `core/config/defaults_messaging.go:44-49`
- **Trigger:** A cluster in state `FAILED` (or `UPDATING` with a `StateInfo.Code`). Spec §4 says S4 shows `StateInfo.Code` and S5 shows `StateInfo.Message` when present, falling back to "failed". The fetcher never reads `cluster.StateInfo`, and the detail field list does not include `StateInfo`, so the cause cannot be seen anywhere in the UI.
- **Impact:** The operator sees only a generic "failed" and must open the AWS console to learn why the cluster failed.
- **Fix:** Carry `StateInfo.Code` / `StateInfo.Message` into the FAILED (and UPDATING) finding as spec §4 describes, and add `{Path: "StateInfo"}` to the `msk` detail fields.

### 10. P3: The "Version" column shows the cluster revision token, not the Kafka version

- **File:** `core/aws/catalog_messaging.go:506` (also `core/aws/msk.go:64-67`)
- **Trigger:** Any cluster. `Cluster.CurrentVersion` is the MSK cluster metadata revision, an opaque optimistic-lock token such as `K3AEGXETSR30VB` that update calls require. It is not the Kafka version, which lives at `Provisioned.CurrentBrokerSoftwareInfo.KafkaVersion`.
- **Impact:** A list column titled "Version" shows an opaque token. Operators scanning for old Kafka versions get nothing useful from it.
- **Fix:** Retitle the column (for example "Revision"), or populate it from `Provisioned.CurrentBrokerSoftwareInfo.KafkaVersion` and leave it blank for serverless clusters.

## mwaa

# mwaa — production-code review

Scope: `core/aws/mwaa.go`, `core/aws/mwaa_interfaces.go`, `core/aws/mwaa_related.go`,
the `mwaa` catalog literal in `core/aws/catalog_data.go`, the `mwaa` view default in
`core/config/defaults_data.go`, and the shared helpers they call
(`alarmIDsByDimension`, `kmsKeyIDFromField`, `ARNForService`, `degradedDetailsFinding`,
`colorAnyFindingOrHealthy`, `NavIDFromValue`, list-cell and detail projection paths).

## 1. The CW Alarms pivot matches a dimension MWAA never publishes, so it is always zero

- **Priority**: P2
- **Location**: `core/aws/mwaa_related.go:25` (`alarmIDsByDimension(ctx, clients, cache, "AWS/MWAA", "EnvironmentName", res.ID)`)
- **Trigger**: open the detail view of any MWAA environment that has CloudWatch alarms on its metrics.
- **Evidence**: AWS publishes MWAA environment metrics with the dimension key `Environment`, not `EnvironmentName`:
  - `AWS/MWAA` container, queue and database metrics use dimension sets such as `Environment` + `Cluster`, `Environment` + `DatabaseRole` (AWS sample dashboard `aws-samples/amazon-mwaa-examples/usecases/mwaa_utilization_cw_metric/mwaa-cw-metric-dashboard.json`: `"CPUUtilization","Environment","$mwaa_env_name","Cluster","AdditionalWorker"`). The documented dimension table is `Cluster`, `Queue` and `Database` (<https://docs.aws.amazon.com/mwaa/latest/userguide/accessing-metrics-cw-container-queue-db.html>).
  - The Airflow metrics, including `SchedulerHeartbeat`, the metric most outage alarms watch, are published in the **`AmazonMWAA`** namespace with `Environment` + `Function` dimensions (<https://docs.amazonaws.cn/en_us/mwaa/latest/userguide/access-metrics-cw.md>; AWS Compute Blog "Automating Amazon CloudWatch dashboards and alarms for MWAA": `["AmazonMWAA", "QueuedTasks", "Function", "Executor", "Environment", "${EnvironmentName}"]`).
  - `alarmIDsByDimension` (`core/aws/related_common.go:211-243`) requires both an exact namespace match and an exact dimension-name match. No real alarm satisfies `EnvironmentName`, and alarms in `AmazonMWAA` are filtered out by the namespace guard.
- **User impact**: the related panel shows a confident "CW Alarms (0)" (or "(0+)") for every environment. During an incident the operator concludes that no alarms exist on the environment and skips the first triage stop. The spec (`docs/resources/mwaa.md` §2 `alarm`) repeats the same wrong dimension name, so fixing the code also means correcting the spec.
- **Fix direction**: match dimension `Environment` == environment name, and accept both the `AWS/MWAA` and `AmazonMWAA` namespaces. `alarmIDsByDimension` takes a single namespace, so it needs a namespace-set variant, or the mwaa checker should call it with `""` and filter the namespace itself. Update the spec's §2 `alarm` entry in the same change.

## 2. Logging configuration is not shown in the detail view, although the spec makes it a detail-view fact

- **Priority**: P3
- **Location**: `core/config/defaults_data.go:29-38` (`"mwaa"` `Detail` path list)
- **Trigger**: open the detail view of any environment and look for whether task, scheduler, DAG-processing, worker or webserver logging is enabled, and at what level.
- **Evidence**: `docs/resources/mwaa.md` §3.1 and §5 turn down "any `LoggingConfiguration` component disabled" as a signal and call it a "detail-view fact only". The detail projection renders only the configured `Detail` paths (`core/semantics/projection/generic.go:93-104`). The `mwaa` list includes neither `LoggingConfiguration` nor `NetworkConfiguration`. The fetcher flattens only the log-group names into `Fields`, and those keys have no `Detail` path either. The per-module `Enabled`/`LogLevel` values therefore never reach any surface.
- **User impact**: an operator asking "why are there no task logs for this DAG run?" cannot see from a9s that `TaskLogs` is disabled or set to `ERROR`. That is exactly the fact the spec says is kept in the detail view in place of a finding.
- **Fix direction**: add `{Path: "LoggingConfiguration"}` to the `mwaa` `Detail` list, and optionally `{Path: "NetworkConfiguration"}`. Then regenerate `.a9s/views/` with `go run ./cmd/viewsgen/`.

## nat

# nat — production-code review

Scope: `core/aws/nat.go`, `core/aws/nat_related.go`, the `nat` entry in `core/aws/catalog_networking.go` (plus `colorNAT`), `core/config/defaults_networking.go` (`nat` detail), reverse pivots into `nat` (`checkVPCNAT`, `checkSubnetNAT`, `checkRTBNAT`, `checkEIPNAT`, `checkENINAT`), `alarmIDsByDimension` (`core/aws/related_common.go`), the humanize path (`core/catalog/types.go`, `core/app/list_columns.go`, `core/app/detail_body.go`, `core/domain/humanize.go`), and `cmd/snapshot/ec2_network.go` `captureNAT`. SDK checked: `service/ec2@v1.332.0` `types.NatGateway` / `types.NatGatewayAddress`.

## 1. P3 — FailureCode is humanized into mangled, unsearchable text

- **File/line**: `core/aws/catalog_networking.go:398` (`HumanizeFields: []string{"failure_code"}`), applied by `core/app/list_columns.go:99` (list "Failure" column, `Path: "FailureCode"`, `catalog_networking.go:412`) and `core/app/detail_body.go:319` (detail `FailureCode` row), through `core/domain/humanize.go:25-62`.
- **Trigger**: any NAT gateway in `failed` state. `FailureCode` values are dotted identifiers (SDK doc: `InsufficientFreeAddressesInSubnet | Gateway.NotAttached | InvalidAllocationID.NotFound | Resource.AlreadyAssociated | InternalError | InvalidSubnetID.NotFound`). `humanizeCamelCase` inserts a space before every uppercase letter whose predecessor is not uppercase, and `.` is not uppercase, so:
  - `Gateway.NotAttached` → `gateway. not attached`
  - `InvalidAllocationID.NotFound` → `invalid allocation id. not found`
  - `InvalidSubnetID.NotFound` → `invalid subnet id. not found`
- **User impact**: the one field that explains why the NAT failed renders as broken prose in both the list and the detail. The operator cannot copy/search the real AWS error code, which contradicts the spec (`docs/resources/nat.md` §4: the FailureCode is "replaced verbatim with the SDK value") and the rule stated on `ResourceTypeDef.HumanizeFields` (`core/catalog/types.go:92-94`: an AWS-assigned name a person types back "must stay verbatim").
- **Fix direction**: remove `"failure_code"` from the `nat` `HumanizeFields` so the code renders verbatim on both surfaces.

## 2. P3 — the nat → alarm pivot misses metric-math alarms

- **File/line**: `core/aws/related_common.go:233-238` (`alarmIDsByDimension`, called from `core/aws/nat_related.go:149`).
- **Trigger**: a CloudWatch alarm on this NAT gateway built as a metric-math or multi-metric alarm (for example, a percentage on `ErrorPortAllocation`, or `PacketsDropCount` divided by `PacketsInFromSource`). For these alarms `MetricAlarm.Dimensions` is empty. The `NatGatewayId` dimension appears only under `MetricAlarm.Metrics[].MetricStat.Metric.Dimensions`, and nothing in `core/aws/` reads `MetricStat`.
- **User impact**: the NAT detail's CloudWatch Alarms pivot shows 0, or omits the alarm, when an alarm does watch this gateway. The operator concludes the NAT is unmonitored. Because the helper is shared, every type that uses `alarmIDsByDimension` has the same gap.
- **Fix direction**: in `alarmIDsByDimension`, also match the dimension against each `alarm.Metrics[i].MetricStat.Metric.Dimensions` (and apply the namespace filter to `MetricStat.Metric.Namespace` when it is set).

## ng

# ng — production-code review

Scope: `core/aws/ng.go`, `core/aws/ng_codes.go`, `core/aws/ng_related.go`, the `ng` catalog entry and `fetchNodeGroupsPage` in `core/aws/catalog_containers.go`, `core/config/defaults_containers.go` (ng view), and the reverse pivots into `ng` (`eks_related.go`, `asg_related.go`, `ec2_related.go`, `iam_roles_related.go`, `lt_related.go`, `ami_related_extra.go`).

Summary: 6 findings (P0:0 P1:0 P2:2 P3:4)

---

## 1. P2 — ng → sg ignores `RemoteAccess.SourceSecurityGroups`

- **File/line**: `core/aws/ng_related.go:179-183` (`checkNGSG`)
- **Defect**: the checker reads only `Resources.RemoteAccessSecurityGroup`. The spec (`docs/resources/ng.md` §2 `sg`) requires the deduplicated union of that field and `RemoteAccess.SourceSecurityGroups[]`, the client SGs allowed to SSH into the nodes. `RemoteAccess` is never read.
- **Trigger**: a node group created with `remoteAccess.sourceSecurityGroups = [sg-bastion]`.
- **User impact**: the related panel leaves out the bastion/client SGs. The spec cites this exact "why can't I SSH?" misdiagnosis as the reason for the pivot. If `RemoteAccessSecurityGroup` is also empty, the panel shows a confident `0`.
- **Fix direction**: collect `ng.RemoteAccess.SourceSecurityGroups` as well, dedupe them with `Resources.RemoteAccessSecurityGroup`, and return the union.

## 2. P2 — ng → ami reports a confident 0 for node groups that do run an AMI

- **File/line**: `core/aws/ng_related.go:193-196` (no launch template) and `core/aws/ng_related.go:222-228` (the launch template declares no `ImageId`)
- **Defect**: every managed node group runs exactly one AMI. For node groups without a custom launch template, or whose template leaves `ImageId` unset, the AMI is the EKS-optimised default. The fetcher's own comment says so (`core/aws/ng.go:44-46`: "the node group inherits the EKS-optimised default"). `checkNGAMI` still returns `KnownRelated("ami", nil, false)`, a proven zero, where it should return Unknown. The spec (§2 `ami`) says the count is "0 or 1 — a node group pins exactly one AMI".
- **Trigger**: any default managed node group (no launch template), which is the most common kind.
- **User impact**: the panel says "AMI 0", so the operator concludes no image is involved and cannot pivot to it for patch-level checks.
- **Fix direction**: take the AMI from the cached EC2 instances already matched by `matchingNGInstances`, using `Instance.ImageId` (zero extra calls, the same join as ng→ec2/ebs). If that cannot resolve it, return `UnknownRelated("ami")`, never a known zero.

## 3. P3 — ng → ami and the fetcher's `image_id` resolve different launch-template versions

- **File/line**: `core/aws/ng_related.go:204` (`$Latest`) vs `core/aws/ng.go:30-33` (`$Default`)
- **Defect**: two places compute the same fact, "which AMI this node group's launch template pins", with different defaults when `LaunchTemplate.Version` is empty. According to AWS (`LaunchTemplateSpecification.version`), a node group without a version uses the template's **default** version, not the latest. `checkNGAMI` also issues its own `DescribeLaunchTemplateVersions` instead of reading the `Fields["image_id"]` the fetcher already resolved.
- **Trigger**: a node group whose `LaunchTemplate.Version` comes back unset, on a template where `$Latest` ≠ `$Default`.
- **User impact**: ng → ami names one AMI, while ami → ng (which matches on `Fields["image_id"]`) links the node group to a different AMI. The two directions of the same relation disagree.
- **Fix direction**: keep one resolver. Use `resolveNGImageID` (with `$Default`) from `checkNGAMI`, or read `Fields["image_id"]` directly.

## 4. P3 — CloudTrail pivot filters on the bare node-group name and cannot tell clusters apart

- **File/line**: `core/aws/catalog_containers.go:116` (`CloudTrailKey: "ResourceName:Fields.nodegroup_name"`); the filter is built by `core/resource/related.go:756-777` as one `{ResourceName: <name>}` attribute.
- **Defect**: `ngRowID` (`core/aws/ng.go:128-135`) states that node-group names are scoped per cluster and that two clusters may each own a `workers`. The ct-events filter carries only that bare name, with no cluster or ARN discriminator.
- **Trigger**: clusters `prod` and `staging` each have a node group named `workers`. Open ct-events from `prod/workers`.
- **User impact**: the pivot lists the other cluster's node-group events as if they belonged to this one, which misleads "who scaled it" during an incident.
- **Fix direction**: filter on the unique `NodegroupArn`, stored in Fields as the eks type stores `arn`, or post-filter the events by cluster name.

## 5. P3 — `ListNodegroups` failures are recorded as `DescribeNodegroup` item failures against a node-group denominator

- **File/line**: `core/aws/catalog_containers.go:380-383` (with `totalAttempted` only incremented at `:294`, and the aggregate labelled at `:363`, `:390`, `:407`)
- **Defect**: a failed `ListNodegroups` for a cluster is appended as `FailedCall(cluster, err)`, but `totalAttempted` counts only described node groups. `AggregateFailures` then prints `"ng: DescribeNodegroup failed for N of M IDs"`, where N counts clusters and M counts node groups.
- **Trigger**: the role lacks `eks:ListNodegroups` on one cluster (or all clusters).
- **User impact**: the error line reads, for example, `ng: DescribeNodegroup failed for 2 of 0 IDs: ... (e.g. prod)`. That is an impossible count under the wrong operation. The failing cluster's node groups are missing from the list, and the message does not say so clearly.
- **Fix direction**: record list-level failures in a separate aggregate (for example `ng: ListNodegroups` over the number of clusters) and join it with `JoinAggregates`, or record them with `FailedOnPage` semantics.

## 6. P3 — ami → ng silently drops node groups whose image could not be resolved

- **File/line**: `core/aws/ami_related_extra.go:87-93`, fed by `core/aws/catalog_containers.go:323-327` and `core/aws/ng.go:142-151`
- **Defect**: `checkAMING` matches only on `Fields["image_id"]`. A node group whose `DescribeLaunchTemplateVersions` failed gets `image_id=""`, and the row carries no marker (the failure goes only into the aggregate). A degraded row (describe denied or unavailable) never gets `image_id` at all. Both are skipped, and the result is not marked truncated or unknown.
- **Trigger**: `ec2:DescribeLaunchTemplateVersions` is denied or throttled for a launch-template node group, or `DescribeNodegroup` is denied for one node group. Then open the AMI it uses.
- **User impact**: the AMI's related panel shows a confident `0`/undercount for node groups, although a9s never learned which image those groups use.
- **Fix direction**: when any scanned ng row has an unresolved image (a failed LT read, or a degraded row), return the matches as truncated, or Unknown when there are none. Mark such rows in Fields so the checker can tell "no image" from "not read".

## opensearch

# opensearch — production-code review

Scope: `core/aws/opensearch.go`, `core/aws/opensearch_related.go`, `core/aws/opensearch_codes.go`, `core/aws/opensearch_interfaces.go`, the `opensearch` literal in `core/aws/catalog_databases.go`, the `opensearch` view default in `core/config/defaults_databases.go`, plus the direct dependencies needed to verify them (`degraded_resource.go`, `related_common.go`, `related_fetch.go`, `iampolicy/evaluate.go`, `core/app/list_columns.go`, `core/semantics/projection/generic.go`, `teardown.go`).

## 1. [P1] DescribeDomains is called with every listed domain name in one request

- **File/line**: `core/aws/opensearch.go:122-133`
- **Trigger**: an account/region with more than 5 OpenSearch/Elasticsearch domains. `ListDomainNames` returns all of them, and the fetcher passes the whole slice as `DescribeDomainsInput.DomainNames` in a single call.
- **Evidence**: the service allows at most 5 names per DescribeDomains request. Over that, it returns a `ValidationException` ("Please provide a maximum of 5 … domain names to describe"; see cloud-custodian issue #5793 for the same limit on the ES API). The repo's own spec (`docs/resources/opensearch.md:22,90,96`) says "up to 5 domain names per call", and the snapshot collector already batches by 5 (`cmd/snapshot/ops.go:1085-1089`). The TUI fetcher does not. `RetryOnThrottle` does not retry a ValidationException, so `describeErr` is set and `descOutput` is nil.
- **User impact**: when the account has 6 or more domains, every domain comes back as a name-only "details unavailable" row (`opensearch.go:268-279`). All posture findings (isolated, public, encryption off, and the rest) disappear, and the Engine, Instance and Endpoint columns are blank. The related panel returns Unknown because RawStruct is nil. This affects the whole type at the accounts where it matters most.
- **Fix direction**: split `domainNames` into chunks of 5 and call DescribeDomains once per chunk. Keep a separate error for each chunk so that only the names in a failed chunk become `DegradedDetails` rows or failures.

## 2. [P2] Log Groups pivot counts disabled log-publishing options

- **File/line**: `core/aws/opensearch_related.go:37-56`
- **Trigger**: a domain where one or more `LogPublishingOptions` entries have `Enabled=false` but still carry a `CloudWatchLogsLogGroupArn`. A log type that was switched off keeps its ARN in the config.
- **Evidence**: the loop reads only `opt.CloudWatchLogsLogGroupArn` and never checks `opt.Enabled`. The spec (`docs/resources/opensearch.md:55`) says: "for each entry with `Enabled==true` take `CloudWatchLogsLogGroupArn`". The AWS `LogPublishingOption.Enabled` field means "Whether the log should be published."
- **User impact**: the Log Groups count includes groups the domain no longer publishes to. An operator chasing slow or error logs opens a group that receives nothing from this domain.
- **Fix direction**: skip entries where `!aws.ToBool(opt.Enabled)`.

## 3. [P2] Default detail view omits `VPCOptions`, so three declared navigable fields never render

- **File/line**: `core/config/defaults_databases.go:57-63`, together with `core/aws/catalog_databases.go:500-502`
- **Trigger**: open the detail view of any VPC-attached domain with the default view config.
- **Evidence**: the catalog declares `VPCOptions.VPCId → vpc`, `VPCOptions.SubnetIds → subnet` and `VPCOptions.SecurityGroupIds → sg` as navigable. The projection only builds items for configured detail paths (`core/semantics/projection/generic.go:162-190`), and the opensearch detail list has no `VPCOptions` path. Those three navigable fields are therefore never produced. The same list also has no `AccessPolicies`, `NodeToNodeEncryptionOptions`, `ServiceSoftwareOptions` or `LogPublishingOptions`. The "reachable outside a VPC", "node-to-node encryption off" and "software update forced soon" findings therefore have no underlying field in the detail body.
- **User impact**: the detail view never shows which VPC, subnets or security groups the domain lives in, and Enter-to-navigate on those fields cannot work. The operator cannot see the raw evidence behind three of the type's findings without opening YAML.
- **Fix direction**: add `{Path: "VPCOptions"}` to the opensearch detail defaults, plus `NodeToNodeEncryptionOptions`, `AccessPolicies`, `ServiceSoftwareOptions` and `LogPublishingOptions`. Then regenerate `.a9s/views` with viewsgen.

## 4. [P3] Alarm pivot matches any namespace carrying a `DomainName` dimension

- **File/line**: `core/aws/opensearch_related.go:21`
- **Trigger**: an alarm in a namespace other than `AWS/ES` that uses a `DomainName` dimension with the same value as the OpenSearch domain name. For example, `AWS/CloudSearch` metrics are dimensioned by `DomainName`.
- **Evidence**: `alarmIDsByDimension(ctx, clients, cache, "", "DomainName", res.ID)` passes an empty namespace, which disables the namespace filter (`core/aws/related_common.go:230`). The spec (`docs/resources/opensearch.md:37`) requires `Namespace=="AWS/ES"` AND `DomainName==<this.DomainName>`.
- **User impact**: the CW Alarms count can include alarms that belong to a different service's resource, and drilling in lands on alarms unrelated to the domain.
- **Fix direction**: pass `"AWS/ES"` as the namespace.

## 5. [P3] ACM pivot re-fetches with DescribeDomainConfig and ignores `CustomEndpointEnabled`

- **File/line**: `core/aws/opensearch_related.go:201-229`
- **Trigger**: opening the related panel of any domain.
- **Evidence**: `DomainStatus.DomainEndpointOptions` already carries `CustomEndpointCertificateArn` and `CustomEndpointEnabled`, and it is held in `res.RawStruct`. The spec (`docs/resources/opensearch.md:31`) says to read it from `DomainStatus`, and only when `CustomEndpointEnabled==true`. The checker instead makes one extra `DescribeDomainConfig` call for each domain and never checks `CustomEndpointEnabled`.
- **User impact**: an extra API call every time the related panel for a domain is resolved. A role that has `es:DescribeDomains` but not `es:DescribeDomainConfig` sees an error on the ACM pivot even though the data is already in hand. A domain whose custom endpoint is disabled can still list the cert as related.
- **Fix direction**: use `assertStruct[opensearchtypes.DomainStatus](res.RawStruct)`, return Unknown when the assertion fails, read `DomainEndpointOptions.CustomEndpointCertificateArn` only when `aws.ToBool(CustomEndpointEnabled)` is true, and drop the DescribeDomainConfig call.

## pipeline

# pipeline — production-code review

Scope: `core/aws/pipeline.go`, `pipeline_interfaces.go`, `pipeline_issue_enrichment.go`, `pipeline_related.go`, `pipeline_stages.go`, `pipeline_stages_codes.go`, the `pipeline` / `pipeline_stages` literals in `core/aws/catalog_cicd.go`, `core/config/defaults_cicd.go`, `core/resource/columns.go` (`PipelineStageColumns`), and the reverse pivots into pipeline (`checkCbPipeline` in `core/aws/codebuild_related.go`, `checkECRPipeline` in `core/aws/ecr_related_extra.go`). Target-ID formats were checked against the target fetchers, and API semantics against the SDK (`codepipeline@v1.55.0`) and the AWS CodePipeline action reference.

## Findings

### 1. P2: The KMS pivot counts keys it cannot resolve (alias ARNs and keys in other regions)

- **Where:** `core/aws/pipeline_related.go:240-247` (`checkPipelineKMS`)
- **What happens:** `seen[arnLastSegment(*st.EncryptionKey.Id)]`
  - The SDK says `EncryptionKey.Id` may be "the key ID, the key ARN, or the alias ARN" (`types.EncryptionKey § Id`).
  - For an alias ARN (`arn:aws:kms:<r>:<acct>:alias/my-key`), `arnLastSegment` returns `my-key`. That is not a key UUID, which is the `kms` Resource.ID (`kms.go:95`). The lazy `FetchKMSKeysByIDs` then calls `DescribeKey(KeyId: "my-key")`, which fails because a bare alias needs the `alias/` prefix.
  - The loop over `p.ArtifactStores` (a cross-region pipeline) also collects each other region's key and reduces it to a bare UUID. The `kms` list and `DescribeKey` both run in the session region, so that UUID never resolves.
- **Trigger:** a pipeline whose artifact store is encrypted with a customer key referenced by alias ARN, or a cross-region pipeline (`ArtifactStores` map).
- **User impact:** "KMS Key (N)" shows a count. Drilling in shows an empty or short list and a FetchByIDs error at detail open. The operator cannot reach the key that the count promised.
- **Fix direction:**
  - Keep the full key/alias ARN.
  - Resolve an alias ARN to its target key ID, for example via `DescribeKey` on the full alias ARN.
  - Drop keys whose ARN region is not the session region, or mark the result truncated/unknown rather than exact.

### 2. P2: The S3 pivot misses the S3 source bucket

- **Where:** `core/aws/pipeline_related.go:290-297` (`checkPipelineS3`)
- **What happens:** For `Provider == "S3"` actions the checker reads only `Configuration["BucketName"]`, which is the S3 *deploy* action's key.
  - The S3 *source* action stores its bucket under `S3Bucket` (AWS action reference, "Amazon S3 source action", configuration `S3Bucket`/`S3ObjectKey`).
  - The spec (§2 `s3`) explicitly requires `Configuration["S3Bucket"]` from Source actions.
- **Trigger:** any pipeline with an S3 source stage, the classic "upload a zip to trigger" pipeline.
- **User impact:** The "S3 Buckets (artifacts)" pivot omits the bucket that triggers the pipeline. It shows only the artifact store, as an exact count, so the operator is told the list is complete.
- **Fix direction:** For `Provider == "S3"`, collect both `Configuration["S3Bucket"]` (source) and `Configuration["BucketName"]` (deploy).

### 3. P2: Action targets in another region or account are matched to same-named resources in the current region/account

- **Where:**
  - `core/aws/pipeline_related.go:115-122` (cb), `:138-144` (role, per-action `RoleArn`), `:156-163` (cfn), `:196-203` (ecr), `:216-225` (ecs-svc), `:259-266` (lambda)
  - Reverse pivots: `core/aws/codebuild_related.go:80-89` (`cbPipelineHasProject`) and `core/aws/ecr_related_extra.go:136-146` (`ecrPipelineHasRepo`)
- **What happens:** Every action is reduced to a bare name (`ProjectName`, `StackName`, `FunctionName`, `ServiceName`, `RepositoryName`, role last segment). The checkers never read `ActionDeclaration.Region` (SDK: "The action declaration's Amazon Web Services Region") or the account in the action's `RoleArn`.
  - Cross-region actions and cross-account deploy roles are exactly the cases the spec calls common (§2 `role`: "per-action roles … typically used for cross-account deploy targets").
  - The name is then filtered against the session-region/account list by `Resource.ID`.
- **Trigger:** a pipeline with a cross-region deploy action (for example a CloudFormation stack `app` in us-west-2 while a9s is on us-east-1), or a cross-account deploy role (for example `arn:aws:iam::<other>:role/DeployRole`).
- **User impact:**
  - The pivot reports an exact count. Drilling in shows either nothing, or a different resource that happens to share the name in the current region/account. A same-named `DeployRole` in both accounts is common with StackSets-provisioned roles.
  - The reverse pivots from cb and ecr likewise claim a pipeline uses a local project or repo when it actually targets another region's.
- **Fix direction:**
  - Skip actions whose `Region` is set and differs from the session region.
  - Skip role ARNs whose account (`arnAccountID`) differs from the session account.
  - Mark the result truncated when anything was skipped, so it is not reported as exact. Apply the same filter in the two reverse helpers.

### 4. P3: The ECS Services pivot ignores the cluster, so services with the same name in different clusters all match

- **Where:** `core/aws/pipeline_related.go:216-225` (`checkPipelineECSSvc`)
- **What happens:** Only `Configuration["ServiceName"]` is collected. `Configuration["ClusterName"]` is discarded, although the spec (§2 `ecs-svc`) says to match by the (cluster, service) tuple.
  - `ecs-svc` Resource.ID is the bare service name (`ecs_svc.go:100`), with the cluster in `Fields["cluster"]`.
  - The drill filters by ID set, so every service named e.g. `api` in every cluster matches.
- **Trigger:** a pipeline deploying service `api` to cluster `prod` in an account that also runs `api` in cluster `staging`.
- **User impact:** "ECS Services (1)" drills into two rows, one of them the wrong cluster. The operator may inspect the wrong service.
- **Fix direction:** Carry the cluster with the ID, using a cluster-qualified ID or a fetch filter on `Fields["cluster"]` (the checker's `FetchFilter` path), so only the (cluster, service) pair is shown.

### 5. P3: Blue/green ECS pipelines report a proven "(0)" ECS Services

- **Where:** `core/aws/pipeline_related.go:217-220`
- **What happens:**
  - The provider filter is `ECS` / `ECSBlueGreen`. `ECSBlueGreen` is not a CodePipeline provider; the ECS blue/green action is `CodeDeployToECS` (AWS action reference, "Amazon ECS and CodeDeploy blue-green deploy action").
  - That action's configuration carries `ApplicationName` / `DeploymentGroupName`, not `ServiceName`. So a `CodeDeployToECS` pipeline falls through to `relatedResult("ecs-svc", nil)`, a known, non-truncated zero.
  - The spec lists `CodeDeployToECS` as an ECS deploy provider that must be covered.
- **Trigger:** any pipeline whose deploy stage uses the CodeDeploy-to-ECS blue/green action.
- **User impact:** The ECS Services row renders as a dimmed, non-actionable "(0)" dead end for a pipeline that does deploy an ECS service.
- **Fix direction:**
  - For `CodeDeployToECS`, resolve the service via `codedeploy:GetDeploymentGroup` (`EcsServices[]`), or at least return `UnknownRelated` rather than a proven zero.
  - Remove the dead `ECSBlueGreen` literal.

### 6. P3: The EventBridge Rules pivot reports a proven zero or an exact count when it does not know

- **Where:** `core/aws/pipeline_related.go:335-338` and `:347-353` (`checkPipelineEbRule`)
- **What happens:** There are two paths.
  - **(a)** When `Fields["arn"]` is empty, the checker returns `KnownRelated("eb-rule", nil, false)`, a proven zero. The field is empty because `accountIDFromClients` could not resolve the account (`pipeline.go:21,71-73`; STS failure or a cached identity error). That is a "could not look" case, not a zero.
  - **(b)** `ListRuleNamesByTarget` is called once and its `NextToken` is ignored. The result is reported as exact (`truncated=false`) even when AWS says more pages exist.
- **Trigger:**
  - (a) STS `GetCallerIdentity` fails or is denied for the session.
  - (b) More matching rule names than one page returns.
- **User impact:**
  - (a) "EventBridge Rules (0)" is dimmed and non-navigable while rules triggering the pipeline may exist.
  - (b) The count is an undercount presented as exact.
- **Fix direction:**
  - (a) Return `UnknownRelated("eb-rule")` when the ARN cannot be built.
  - (b) Either page through `NextToken` or pass `truncated = out.NextToken != nil`.

### 7. P3: Healthy and never-run pipelines show "ok" in the Status column

- **Where:** `core/aws/pipeline_issue_enrichment.go:65,99`
- **What happens:** `lastStatus := "OK"` … `result.FieldUpdates[key] = map[string]string{"last_status": lastStatus}`.
  - `last_status` is the type's status key (`catalog_cicd.go:120,131`).
  - `extractCellText` (`core/app/list_columns.go:208-214`) renders it through `HumanizeStatusPhrase`, so every row without a failed stage reads `ok`.
  - That includes pipelines that have never executed (empty `StageStates`) and pipelines mid-execution.
  - `docs/attention-signals.md` S4 requires "Healthy rows render blank — no `OK` / `available` / `ACTIVE` / `running`". The spec §3.1 likewise says the Status column is blank for healthy rows.
- **Trigger:** open the CodePipelines list and wait for Wave 2 to finish.
- **User impact:** Healthy rows carry a positive "ok" verdict, including rows where nothing was ever run or checked. This is the confident-healthy wording the S4 contract forbids.
- **Fix direction:** Write `last_status = ""` for rows with no failed stage. The finding phrase already covers failed rows, and the uninspected path already has its own `not inspected` wording.

## policy

# policy — production-code review

Scope: `core/aws/iam_policies.go`, `iam_policies_related.go`, `iam_policy_issue_enrichment.go`, `iam_policy_detail_enrichment.go`, `iam_policy_docs.go`, `iam_policy_scope.go`, `policy_findings.go`, the `policy` entry in `core/aws/catalog_security.go`, `core/config/defaults_security.go` (`policy`), `core/iampolicy/`, `core/semantics/ctevent/summarize_iam.go` (policyArn nav), plus the runtime paths they feed (`core/runtime/executor.go` by-ID fetch, `core/runtime/handlers_availability.go` FieldUpdates merge, `core/session/rowstore.go`).

Summary: 9 findings (P0:0 P1:1 P2:5 P3:3)

---

## 1. P1 — ListEntitiesForPolicy is read one page only, so the role, user and group pivots undercount and still claim to be exact

- **File/line**: `core/aws/iam_policies_related.go:44-46` (`listAllPolicyEntities`), results used at `:77-83`, `:101-107`, `:130-136` through `relatedResult` (`core/aws/related_common.go:190`), which marks them not truncated.
- **Trigger**: a policy attached to more entities than one page returns (the default `MaxItems` is 100). This is typical for a shared customer policy, or for an AWS-managed policy such as `ReadOnlyAccess` reached by a drill. `IsTruncated`/`Marker` are never read, and no `Marker` loop exists.
- **User impact**: the policy's blast-radius panel (the whole point of the Roles, Users and Groups pivots, per spec §2) shows an exact-looking count and list that leave out principals. An operator who retires or tightens the policy based on that list misses the ones it left out.
- **Fix direction**: paginate `ListEntitiesForPolicy` with `Marker` until `IsTruncated` is false (bounded by `PerParentPageCap`, as `listAttachedRolePolicies` does), wrap each call in `RetryOnThrottle`, and use `relatedResultTrunc` when the cap is hit.

## 2. P2 — Inline group policies are listed as `policy` rows keyed only by policy name, so rows collide, findings attach to the wrong row, and their detail never loads

- **File/line**: `core/aws/catalog_security.go:153-159` (the list fetcher appends `fetchInlineGroupPolicies`); `core/aws/iam_policies.go:435-445` (inline `ID`/`Name` = bare policy name, no `RawStruct`, no `arn`); `core/aws/iam_policies.go:230-231` (lazy-add `store.Set(r.ID, r)` runs after the managed build).
- **Trigger**: two groups each have an inline policy with the same name (for example `default`), or a group has an inline policy with the same name as a customer-managed policy (or as an AWS-managed name resolved by lazy-add).
- **User impact**:
  - The list holds duplicate IDs. The first page is stored without dedup (`core/session/rowstore.go:292-293`), and Wave-2 `FieldUpdates` are merged by `r.ID` (`core/runtime/handlers_availability.go:770`). A managed policy's `risk` ("admin policy" / "privilege escalation") is therefore also written onto the inline row with the same name.
  - In the lazy-add store, the inline entry overwrites the customer-managed entry with the same name. A role, user or group → policy drill for that managed name then opens the inline stub instead: no ARN, so the Roles and Users pivots are empty (`policyARNFromResource` returns `""`), and the wrong document is shown.
  - Inline rows have `RawStruct == nil`, so `enrichDetail` returns `ErrDetailEnrichSkipped` on every open (`core/aws/detail_enrich_engine.go`, the `res.RawStruct == nil` branch). The inline policy's document is never shown, and the SDK-path detail fields (`PolicyName`, `Arn`, …) render empty.
  - Listing inline policies here also goes against spec §5 ("this resource type is managed policies only").
- **Fix direction**: follow the spec and drop inline rows from the `policy` list. If the group → inline-policy drill must stay, give inline entries a namespaced identity (for example `inline/<group>/<name>`), make `checkGroupPolicy` emit that ID, keep them out of the name-keyed managed lookup, and give them a `RawStruct` plus a document fetch (`GetGroupPolicy`).

## 3. P2 — The inline-group sweep reads only the first page of ListGroups (and of ListGroupPolicies), silently

- **File/line**: `core/aws/iam_policies.go:384-386` (`ListGroups` with an empty input, no `Marker` loop) and `:424-426` (`ListGroupPolicies`, no `Marker` loop).
- **Trigger**: an account with more than 100 IAM groups (the default `MaxItems`). The same applies to a group whose inline policies span more than one page.
- **User impact**: inline policies of groups after the first page are missing from the list, and nothing reports it: no error, and the pagination is not marked truncated. The `FetchIAMPoliciesByIDsFull` lazy-add then fails for those names. They fall through to `getAWSManagedPolicyByName`, which makes 4 GetPolicy calls that return NoSuchEntity, so the group → policy drill reports "not resolvable".
- **Fix direction**: paginate both calls with `Marker`/`IsTruncated` (bounded), or surface the missing pages as a failure or truncation.

## 4. P2 — The CloudTrail Events pivot filters by policy name instead of policy ARN

- **File/line**: `core/aws/catalog_security.go:119` (`CloudTrailKey: "ResourceName:ID"`, where `ID` = `PolicyName`, `iam_policies.go:127`).
- **Trigger**: open the CloudTrail Events pivot on any managed policy.
- **User impact**: spec §2 requires `LookupEvents` with `ResourceName == <policy ARN>`. The IAM policy events the pivot exists for (`CreatePolicyVersion`, `SetDefaultPolicyVersion`, `Attach*Policy`, `DeletePolicy`) identify the policy by `policyArn`. The name-keyed lookup therefore returns an empty or incomplete audit trail for the policy.
- **Fix direction**: use `CloudTrailKey: "ResourceName:Fields.arn"`, which `buildFilterFromKey` already supports.

## 5. P2 — Drilling from a CloudTrail `policyArn` field to an AWS-managed policy always fails

- **File/line**: `core/semantics/ctevent/summarize_iam.go:31-36` makes `policyArn` navigable with ID = the full ARN. `core/aws/iam_policies.go:279` passes that ARN to `getAWSManagedPolicyByName`, which builds `"arn:"+partition+":iam::aws:"+path+name` (`:162`), giving `arn:aws:iam::aws:policy/arn:aws:iam::aws:policy/AdministratorAccess`.
- **Trigger**: a CloudTrail event such as `AttachRolePolicy` with `policyArn = arn:aws:iam::aws:policy/<X>`, where the operator presses Enter on the field. That is the common case: attachments of AWS-managed policies. Customer-managed ARNs resolve only because `buildLocalPolicies` also indexes by ARN.
- **User impact**: the drill fails with "AWS-managed policy … not resolvable by name" (`executor.go` `ByIDFetchFailed`). Before failing, it first runs a full customer-policy list and a group-inline sweep.
- **Fix direction**: in `FetchIAMPoliciesByIDsFull`, when the ID parses as an ARN, call `GetPolicy` on that ARN directly (and cache it by ARN); keep the path-guessing loop only for bare names.

## 6. P2 — The orphan warning ignores `PermissionsBoundaryUsageCount`, so a policy in use as a permissions boundary is reported as unused

- **File/line**: `core/aws/iam_policies.go:25-27` (`orphanUnattachedPolicyFinding` checks only `AttachmentCount == "0"`); the same count also sets the status at `core/aws/iam_policy_issue_enrichment.go:81-84`.
- **Trigger**: a customer-managed policy used only as a permissions boundary on users or roles. For such a policy `AttachmentCount` is 0 and `PermissionsBoundaryUsageCount` is greater than 0, because AWS counts boundary use separately.
- **User impact**: the row turns yellow with "unattached, no roles/users/groups use it", and the detail text advises "Delete it". That is false: the policy is actively capping principals' permissions, and removing it would lift those caps.
- **Fix direction**: require `PermissionsBoundaryUsageCount == 0` (a nil pointer counts as 0) as well before emitting `iam-policy.orphan-unattached` and before setting the `unattached` status. Carry that count into `Fields` from `managedPolicyToResource` and `buildLocalPolicies`.

## 7. P3 — The "unattached" Status value, which comes from list data, is written only by the Wave-2 document fetch

- **File/line**: `core/aws/iam_policy_issue_enrichment.go:71-84, 99-101` (the `risk` field is set only after `FetchManagedPolicyDocument` succeeds); rows past `EnrichmentCap` are cut at `:57`; the Wave-1 builder `core/aws/iam_policies.go:126-139` never sets `risk`.
- **Trigger**: an orphan customer-managed policy beyond the first 50 enriched, or one whose GetPolicy/GetPolicyVersion call fails.
- **User impact**: the row is yellow from the Wave-1 orphan finding, but its Status column (`risk`) stays blank. The list does not say why the row is flagged.
- **Fix direction**: set `risk = "unattached"` in the Wave-1 resource builder (`managedPolicyToResource`) and let Wave 2 only raise it to admin or privilege escalation.

## 8. P3 — The process-global ListEntitiesForPolicy cache outlives profile/region rotation, and AWS-managed ARNs are the same in every account

- **File/line**: `core/aws/iam_policies_related.go:20-24, 36-51` (a package-level `sync.Map` keyed only by the policy ARN, never cleared by `Session.Rotate`).
- **Trigger**: the operator views the related panel of an AWS-managed policy (`arn:aws:iam::aws:policy/...`, the same ARN in every account), switches profile, and opens the same policy within 5 s. A cancelled-context error is also cached and replayed for 5 s.
- **User impact**: the previous account's roles, users and groups are shown as attached in the new account; or a transient or cancelled error keeps showing for 5 s.
- **Fix direction**: move the cache into session-scoped state that `Rotate()` clears (or key it by the session generation), and do not cache context-cancellation errors.

## 9. P3 — Policy document fetches are not retried on throttling, although the Wave-2 pass fans them out in parallel

- **File/line**: `core/aws/iam_policy_docs.go:19` (`GetPolicy`) and `:29` (`GetPolicyVersion`); called in parallel from `core/aws/iam_policy_issue_enrichment.go:71` (under `EnrichmentParallelism`) and from `enrichPolicy`. Neither wraps the calls in `RetryOnThrottle`. The engine's own contract says fetch is RetryOnThrottle-wrapped (`detail_enrich_engine.go`, `detailEnrichSpec.fetch` doc).
- **Trigger**: an account with dozens of customer-managed policies. There are 2 IAM calls per policy, all under IAM's account-wide rate limit.
- **User impact**: throttled policies are marked skipped, so admin-star and privilege-escalation findings are missing for them (only a truncation marker shows), and a detail open can fail with a throttling error.
- **Fix direction**: wrap both calls in `RetryOnThrottle(ctx, DefaultRetryConfig(), …)`.

## r53

# r53 production-code review

Scope: `core/aws/r53.go`, `r53_records.go`, `r53_related.go`, `r53_issue_enrichment.go`, `r53_interfaces.go`, `catalog_dns_cdn.go` (r53 / r53_records entries), `core/config/defaults_dns_cdn.go`, `catalog_color_helpers.go:r53Color`, the direct consumers `s3_related.go:checkS3R53`, `install.go:ctEventsCheckerFor`, and `client.go`.

8 findings (P0:0 P1:0 P2:6 P3:2)

---

## 1. P2: the r53 → elb pivot never matches a Network Load Balancer

- **File**: `core/aws/r53_related.go:93`
- **Code**: `if strings.Contains(d, ".elb.amazonaws.com") {`
- **Trigger**: a hosted zone with an ALIAS record pointing at an NLB. NLB DNS names use the form `name-id.elb.<region>.amazonaws.com` ([AWS NLB docs](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/network-load-balancers.html)). The region sits between `.elb.` and `.amazonaws.com`, so the substring test fails. Only ALB and CLB names (`name-id.<region>.elb.amazonaws.com`) pass.
- **Impact**: the NLB alias is filtered out before the ELB cache is consulted. If the zone has no other ELB alias, line 97 returns a confident known zero. Otherwise the NLB is silently missing from the count and from the navigation list. The spec (§2 `elb`) says to match on `.elb.`.
- **Fix**: select alias names containing `.elb.` under an `amazonaws.com` or `amazonaws.com.cn` host. The exact-match join against `dns_name` (lines 117–123) already handles both shapes.

## 2. P2: the r53 → apigw pivot can never produce a match

- **File**: `core/aws/r53_related.go:197-203` (extract), `219-222` (join)
- **Trigger**: any zone with an ALIAS record to an API Gateway custom domain. Per the [AliasTarget API reference](https://docs.aws.amazon.com/Route53/latest/APIReference/API_AliasTarget.html), the alias `DNSName` for API Gateway must be the custom domain's `regionalDomainName` (`d-xxxxxxxx.execute-api.<region>.amazonaws.com`). For edge-optimized domains it is a `*.cloudfront.net` distribution. The code takes the label before `.execute-api.` (`d-xxxxxxxx`) as an API ID and joins it against `apigwRes.ID`. That field holds the REST/HTTP API ID (`core/aws/apigw.go:180,245`). A `d-…` domain ID never equals an API ID.
- **Impact**: the API Gateways row always shows 0 for zones that front API Gateway custom domains. It is a confident wrong zero, not an unknown.
- **Fix**: resolve through the custom domain. Use GetDomainNames (`regionalDomainName` / `distributionDomainName`) plus the base-path / API mappings to reach API IDs. Until that exists, return Unknown instead of a known zero when execute-api aliases are present.

## 3. P2: the r53 → acm pivot returns validation record names as ACM IDs

- **File**: `core/aws/r53_related.go:306`
- **Code**: `ids = append(ids, strings.TrimSuffix(name, "."))`
- **Trigger**: a zone holding ACM DNS-validation CNAMEs (`_hash.example.com` → `*.acm-validations.aws.`).
- **Impact**: the related IDs are record FQDNs such as `_abc123.example.com`. ACM rows are keyed by certificate ARN (`core/aws/acm.go:139-146`). The by-ID drill filters the ACM list by `RelatedIDSet`, so the operator gets an empty or stranded list, or a stub for a certificate that does not exist. The count also measures the wrong thing:
  - it counts validation records, not certificates;
  - one CNAME validates every cert for that domain, including expired certs and their renewals;
  - stale validation records for deleted certs are counted too.
- **Fix**: follow spec §2. Take the loaded `acm` list (FetchRelatedTarget) and match each cert's `DomainName` / SANs to this zone's `Name` by longest suffix. Emit certificate ARNs.

## 4. P2: the r53 → logs pivot reports 0 whenever the session region is not us-east-1

- **File**: `core/aws/r53_related.go:340-375`
- **Trigger**: a public zone with query logging enabled, viewed from any session region other than us-east-1. Route 53 public DNS query logging requires the log group to be in us-east-1 ([Route 53 query logging docs](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/query-logs.html)). `FetchRelatedTarget(..., "logs")` loads the session region's log groups, so the us-east-1 group is never found.
- **Impact**: `ListQueryLoggingConfigs` proved a log group exists, yet the panel shows a known "Log Groups 0". This also contradicts the Wave 2 enricher, which correctly raises no "query logging off" for the zone.
- **Fix**: when the config's ARN region (parse `arn:aws:logs:<region>:…`) differs from the session region, return Unknown or a cross-region indication instead of a zero. Alternatively, look up the group with a us-east-1 logs client.

## 5. P2: the r53 → ct-events pivot queries the wrong region

- **File**: `core/aws/catalog_dns_cdn.go:41` (`CloudTrailKey: "ResourceName:ID"`), `core/aws/install.go:19-27`, `core/aws/client.go:220` (`cloudtrail.NewFromConfig(cfg)`, session region)
- **Trigger**: open any hosted zone's CloudTrail Events pivot from a session region other than us-east-1. Route 53 is a global service, and its API events appear in CloudTrail event history only in US East (N. Virginia) ([Route 53 CloudTrail docs](https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/logging-using-cloudtrail.html)). `LookupEvents` runs against the regional client.
- **Impact**: record changes, VPC associations and DNSSEC changes never appear. The pivot shows an empty event list, which the operator reads as "nobody touched this zone".
- **Fix**: route LookupEvents for r53 (a global-service type) through a us-east-1 CloudTrail client, like the existing `WAFv2CloudFront` pinned client. At the same time, check that the `ResourceName` value matches CloudTrail's form: `r.ID` is `/hostedzone/Z…`. I did not verify which form CloudTrail stores. AWS examples show the bare zone ID.

## 6. P2: a failed per-zone record scan at list time is stored as "no S3 aliases, not truncated"

- **File**: `core/aws/r53.go:87-97` (and `162-168`: the scan uses no `RetryOnThrottle`, unlike every other Route 53 call in the package)
- **Trigger**: `ListResourceRecordSets` fails for a zone during `FetchHostedZonesPage`. Throttling is likely because the fetcher issues one sequential call per zone, up to 100 per page, against Route 53's per-account rate limit. AccessDenied also triggers it. `aliasErr` becomes a page-level partial failure, but the row keeps `s3website_alias_names=""` and `records_truncated=""`. Those fields persist through FieldKeys and the YAML cache.
- **Impact**: `checkS3R53` (`core/aws/s3_related.go:423-430`) reads these fields and skips the zone as a confident "does not alias this bucket". The S3 → Route 53 pivot shows 0 (not "N+") for buckets that are aliased in the failed zone. The two directions of the relationship disagree.
- **Fix**: on `aliasErr`, set `records_truncated="true"`, or a dedicated "records_unread" flag that `checkS3R53` treats as a lower bound. Wrap the call in `RetryOnThrottle(ctx, DefaultRetryConfig(), …)` as the sibling calls do.

## 7. P3: the dangling-record check is skipped silently when the ec2 or eip inventory is incomplete

- **File**: `core/aws/r53_issue_enrichment.go:224-226`
- **Code**: `if held == nil { return }`
- **Trigger**: the ec2 or eip cache entry is absent or `IsTruncated` (`heldPublicAddresses`, lines 91-94). For example, the account's instances span more than one page.
- **Impact**: every public zone gets no dangling-record verdict, but none is marked in `TruncatedIDs`. The Wave 2 result claims the zone was fully evaluated. A zone with a record pointing at an unattached EIP renders clean, with no hint that the Broken check never ran. The other unreadable paths in this function (lines 229-230, 188-190) call `MarkSkipped`.
- **Fix**: when `held == nil`, mark each public zone truncated (a `TruncatedIDs` entry naming the missing inventory) instead of returning silently.

## 8. P3: the r53 → vpc pivot counts VPC associations in other regions as navigable VPCs

- **File**: `core/aws/r53_related.go:401-408`
- **Trigger**: a private hosted zone associated with VPCs in several regions, which is common for shared private zones. `GetHostedZone` returns every association with its `VPCRegion`. The loop keeps `VPCId` and drops `VPCRegion`.
- **Impact**: the count includes out-of-region VPCs, but the drill filters the session region's VPC list by ID. The operator sees fewer rows than the count promised, or a stranded placeholder or stub when every association is elsewhere.
- **Fix**: keep only associations whose `VPCRegion` equals the session region for the navigable ID list. Surface the others as a separate note or a lower-bound flag.

## redis

# redis — production-code review

Scope: `core/aws/redis.go`, `core/aws/redis_related.go`, `core/aws/redis_codes.go`, `core/aws/redis_interfaces.go`, the `redis` entry in `core/aws/catalog_databases.go`, the `redis` view in `core/config/defaults_databases.go`, and the helpers they call (`related_common.go`, `related_shared.go`, `related_fetch.go`, `teardown.go`).

AWS semantics were checked against the ReplicationGroup API reference (<https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_ReplicationGroup.html>) and against the SDK at `github.com/aws/aws-sdk-go-v2/service/elasticache@v1.61.0/types/types.go`.

## Findings

### 1. P1: Groups that use RBAC are marked Broken with "no authentication token"

- **File/line**: `core/aws/redis.go:222`
- **Code**: `if transitOn && !aws.ToBool(rg.AuthTokenEnabled) { add(CodeRedisNoAuth, domain.SevBroken, ...) }`
- **Trigger**: a Redis 6+ replication group that has in-transit encryption on and uses role-based access control. Access control comes from a user group (`ReplicationGroup.UserGroupIds` is not empty), and `AuthTokenEnabled` is `false` because AWS does not allow an AUTH token and a user group together. The SDK has `ReplicationGroup.UserGroupIds []string` (types.go:1612), but no production code reads it.
- **User impact**: a group that does require authentication gets a red row. Its detail view says "The group accepts any client that can reach it … no authentication token is required", with a Broken badge. This is a false Broken signal on the most secure setup, and it also raises the menu issue count.
- **Fix direction**: raise `CodeRedisNoAuth` only when `AuthTokenEnabled` is not true **and** `len(rg.UserGroupIds) == 0`.

### 2. P2: The "encryption at rest off" check ignores the effective encryption state

- **File/line**: `core/aws/redis.go:211`
- **Code**: `if !aws.ToBool(rg.AtRestEncryptionEnabled) { add(CodeRedisAtRestOff, ...) }`
- **Trigger**: a replication group where `AtRestEncryptionEnabled` is `false` or nil but `StorageEncryptionType` is `sse-elasticache` or `sse-kms`. The AWS API reference and the SDK doc comment (types.go:1435-1437) both say: "In some cases, encryption at-rest may be enabled even when this value is false. Use `StorageEncryptionType` to view the effective encryption state." The field exists on `ReplicationGroup` (types.go:1597) but is never read.
- **User impact**: an encrypted group gets the Warning "encryption at rest off". Its advice, "recreate the replication group with it enabled and migrate", sends the operator into a needless migration.
- **Fix direction**: when `rg.StorageEncryptionType` is set, it decides the answer (`none` means off, anything else means on). Fall back to `AtRestEncryptionEnabled` only when `StorageEncryptionType` is empty.

### 3. P2: The Endpoint column is empty for cluster-mode-disabled groups, and the detail view never shows an endpoint for them

- **File/line**: `core/aws/catalog_databases.go:231` (column `Path: "ConfigurationEndpoint.Address"`), `core/aws/redis.go:85-88` (`Fields["endpoint"]` comes only from `ConfigurationEndpoint`), `core/config/defaults_databases.go:30-35` (the detail fields include `ConfigurationEndpoint` but not `NodeGroups`)
- **Trigger**: any cluster-mode-disabled replication group, which the spec calls the common case (§4 notes, "95% of deployments"). For those groups `ConfigurationEndpoint` is absent. Their connection endpoints are `NodeGroups[0].PrimaryEndpoint` and `NodeGroups[0].ReaderEndpoint`. No production code reads `PrimaryEndpoint` or `ReaderEndpoint`.
- **User impact**: the Endpoint column is blank for most Redis rows, and the detail view has no endpoint either. The operator cannot see or copy the address the application connects to without opening the raw YAML.
- **Fix direction**: fill `endpoint` from `ConfigurationEndpoint.Address` when it is present, otherwise from `NodeGroups[0].PrimaryEndpoint.Address`. Point the column at that field key instead of the `ConfigurationEndpoint.Address` path. Add `NodeGroups` (or its primary and reader endpoints) to the redis detail fields.

### 4. P3: The per-shard AZ and role breakdown required by the spec is never rendered

- **File/line**: `core/aws/redis.go:167-171` (shard findings get no `AttentionDetail`), `core/config/defaults_databases.go:30-35` (`NodeGroups` is not a detail field)
- **Trigger**: a cluster-mode-enabled group with a shard that is not `available`, or any non-Healthy group whose `NodeGroups` is populated. Spec `docs/resources/redis.md` §3.1 ("Detail-only visibility … per-node AZ + role breakdown") and §4 notes ("the detail view's Attention section renders one row per non-available shard with primary AZ + replica AZs") require this block. `computeShardIssues` records only the id and status. No production code reads `NodeGroupMembers`, `CurrentRole` or `PreferredAvailabilityZone`.
- **User impact**: during a shard transition or failover the operator sees `shard 0002: modifying` but cannot tell which AZ holds the primary and which hold the replicas. That is the context the spec says the detail view exists to provide.
- **Fix direction**: in `computeRedisFindings`, attach an `AttentionDetail` to each `CodeRedisShardIssue` (and to the RG-level lifecycle findings when `NodeGroups` is populated). Give it one row per member: `CacheClusterId`, `CurrentRole` when set, and `PreferredAvailabilityZone`.

## redshift

# redshift — production-code review

Scope: `core/aws/redshift.go`, `redshift_codes.go`, `redshift_interfaces.go`, `redshift_related.go`, `redshift_issue_enrichment.go`, the `redshift` entry in `core/aws/catalog_databases.go`, `core/aws/teardown.go`, `core/config/defaults_databases.go`, `cmd/snapshot/databases.go`, and the shared helpers they call (`issue_enrichment.go`, `related_common.go`, `core/app/list_columns.go`, `core/runtime/handlers_availability.go`).

## 1. P2 — Paused clusters and other unmapped ClusterStatus values show as healthy, with an empty Status cell

- **Where**: `core/aws/redshift.go:143-206` (`computeRedshiftFindings`), `core/aws/redshift.go:93-102` (`status` field), `core/aws/teardown.go:30-35`.
- **Trigger**: a cluster whose `ClusterStatus` is `paused`, `final-snapshot`, `rotating-keys`, `updating-hsm`, `cancelling-resize`, `available, prep-for-resize` or `available, resize-cleanup`, and whose `ClusterAvailabilityStatus` is `Available` or `Maintenance` or empty. None of these values appears in `brokenByStatus` or `transitionalByStatus`, so the function returns no finding, or only posture warnings. `domain.StatusPhrase` then returns `""`, and `Fields["status"]` is set to that empty phrase. The status column has no `Path`, so `core/app/list_columns.go:208-218` has nothing to fall back to and the cell renders blank. `colorRedshift` → `colorAnyFindingOrHealthy` colors the row as healthy.
- **Impact**: in the list, a paused cluster looks the same as a running one: blank Status, healthy color. So does one that is being deleted with a final snapshot. The operator cannot tell from the list that the cluster does not serve queries. The spec says `paused` should be Dim and the other values Warning (`docs/resources/redshift.md:247`). Other fetchers, such as `rds.go:95-104`, `mwaa.go:107` and `transfer.go:117`, fall back to the raw status for exactly this case. Also, `final-snapshot` is not in `isTeardownStatus`. A cluster being deleted with a final snapshot still gets the Wave-1 posture warnings (`public endpoint`, `unencrypted at rest`) and the Wave-2 calls (`EnrichRedshiftPosture` at `redshift_issue_enrichment.go:60`), which the teardown contract says should be skipped.
- **Fix**: when no finding is produced and `ClusterStatus` is not `available`, set `Fields["status"]` to the raw `ClusterStatus`, as `rds.go` does. Give `paused` a Dim color in `colorRedshift`. Add `final-snapshot` to the teardown check for `redshifttypes.Cluster`, and keep it out of the posture warnings in `computeRedshiftFindings`.

## 2. P3 — Wave-2 enricher marks the issue count as a lower bound although it emits only `~` findings

- **Where**: `core/aws/redshift_issue_enrichment.go:49` (`capAtEnrichmentCap` → `SetTruncated(true)`) and `:101` (`Finish` → `SetTruncated(len(failures) > 0)`). There is no `MarkInformationalOnly(&result)` call.
- **Trigger**: any `DescribeLoggingStatus` or `DescribeClusterParameters` failure, for example AccessDenied for one of these calls on a read-only role, or more clusters than `EnrichmentCap`. Both Wave-2 codes (`redshift.audit-logging-off`, `redshift.require-ssl-off`) are `SevWarn` (`catalog_databases.go:583-584`).
- **Impact**: the `IssueEnricherResult` contract (`issue_enrichment.go`, `Truncated` doc) says an enricher that emits only `~` findings must leave `Truncated` false. `handlers_availability.go:813-829` carries `msg.Truncated` into the menu and list issue badge. When any Wave-1 issue exists, the redshift badge shows `N+`, claiming hidden issues that this pass cannot produce. `TruncatedIDs` already shows the per-row gaps. Separately, lines 50-53 are dead code: `n := len(resources); if n < len(resources)` can never be true.
- **Fix**: call `MarkInformationalOnly(&result)` after `Finish`, as `ecs`, `elb`, `vpc` and the other `~`-only enrichers do. Delete the dead `n < len(resources)` branch.

## 3. P3 — Snapshot collector records "pending change" for every cluster

- **Where**: `cmd/snapshot/databases.go:703`, where `PendingModifiedValuesSet: c.PendingModifiedValues != nil`.
- **Trigger**: DescribeClusters returns `"PendingModifiedValues": {}` for clusters with nothing pending. The AWS CLI `describe-clusters` example output confirms this. The SDK therefore decodes a non-nil, empty struct.
- **Impact**: the snapshot writes `pending_modified_values_set: true` for every cluster. The app's own rule (`redshift.go:210-248`, `hasPendingRedshiftModifiedValues`) treats an empty struct as nothing pending. So the captured ground truth disagrees with what the app shows (`pending change queued` only when a sub-field is set), and an oracle built on this field expects a warning that the app correctly does not show.
- **Fix**: compute the field with `awsinternal.hasPendingRedshiftModifiedValues`-equivalent logic: export the helper, or duplicate the sub-field checks. Do not use a nil check.

## role

# role — production-code review

Scope: `core/aws/iam_roles.go`, `core/aws/iam_roles_related.go`, `core/aws/iam_role_issue_enrichment.go`, `core/aws/iam_role_policies.go`, `core/aws/iam_role_policies_detail_enrichment.go`, the `role` / `role_policies` entries in `core/aws/catalog_security.go`, `core/config/defaults_security.go` (role views), and the direct dependencies `core/iampolicy/{policy,evaluate,actions}.go` and `core/aws/policy_findings.go`.

## Findings

### 1. P2: the "Last Used" column and the RoleLastUsed / PermissionsBoundary / Tags detail fields are always blank for listed roles

- **Where**: `core/aws/catalog_security.go:65` (column `{Title: "Last Used", Path: "RoleLastUsed.LastUsedDate"}`) and `core/config/defaults_security.go:11-12` (detail paths `RoleLastUsed`, `PermissionsBoundary`, `Tags`). The data source is `core/aws/iam_roles.go:173` (`RawStruct: role` taken from `ListRoles`). The Wave 2 enricher gets the right data and throws it away at `core/aws/iam_role_issue_enrichment.go:68-92`.
- **Trigger**: open the role list or a role's detail view with a real account. The list comes from `FetchIAMRolesPage`, which calls `ListRoles`. AWS documents that `ListRoles` "does not return the following attributes, even though they are an attribute of the returned object: PermissionsBoundary, RoleLastUsed, Tags" (API_ListRoles). The `role` type registers no `DetailEnrich`, so nothing fills these fields in later. `EnrichIAMRoleLastUsed` calls `GetRole` for every role, but it uses `out.Role.RoleLastUsed` only to decide whether to raise the dormant finding.
- **Impact**:
  - The "Last Used" column is empty on every row of a paginated list.
  - The detail view shows an empty RoleLastUsed, PermissionsBoundary and Tags, even for roles that have been used recently or that carry a boundary or tags.
  - Roles resolved through `FetchRolesByIDs` (`GetRole`) do show these values, so the same role renders differently depending on how it was loaded.
  - The operator cannot see the fact the dormant finding is based on.
- **Fix direction**: pick one source of truth for these fields.
  - Option A: register a `DetailEnrich` for `role` that calls `GetRole` and swaps in the full `iamtypes.Role` as RawStruct.
  - Option B: have the Wave 2 pass publish `RoleLastUsed.LastUsedDate` as a field that the column reads.
  - With either option, drop or re-point the `RoleLastUsed.LastUsedDate` column path so it no longer reads a struct that `ListRoles` never fills.

### 2. P2: the "IAM Users (trust)" / "IAM Groups (trust)" pivots count principals from other accounts and from Deny statements as local trusted users/groups

- **Where**: `core/aws/iam_roles_related.go:91-128` (`extractPrincipalsByKind` / `addKindedPrincipal`), called from `checkRoleIamUser` (line 74) and `checkRoleIamGroup` (line 58).
- **Trigger**, either of these:
  - The trust policy names a user in another account, e.g. `"Principal":{"AWS":"arn:aws:iam::999999999999:user/alice"}`. This is the common cross-account trust pattern.
  - The trust policy has a `"Effect":"Deny"` statement whose `Principal.AWS` names a user.

  The walker records the last ARN segment of every `AWS` key anywhere in the document. It does not check the statement's `Effect` or the ARN's account. The role's own ARN (`iamtypes.Role.Arn`, available on RawStruct) is never consulted.
- **Impact**:
  - The related panel shows `IAM Users (trust): 1` for `alice`.
  - The drill then lands on nothing, because there is no local `alice`, or on an unrelated local user who happens to be named `alice`.
  - A user who is explicitly denied is presented as someone who can assume the role.
  - This is the same class of defect that `sameAccountRoleNames` (`iam_roles_related.go:172`) already guards against for role principals.
- **Fix direction**:
  - Parse with `iampolicy.Parse` (the same parse the trust findings use) instead of a second ad-hoc JSON walk.
  - Take only `Effect == "Allow"` statements.
  - Drop principal ARNs whose account (`arnAccountID`) differs from the role ARN's account.

### 3. P3: the dormant finding fires on a newly created role and claims "over 90 days"

- **Where**: `core/aws/iam_role_issue_enrichment.go:84-89`.
- **Trigger**: a role created less than 90 days ago that has not been assumed yet. `GetRole` then returns `RoleLastUsed` with no `LastUsedDate`, and `isDormant` is set to true unconditionally. `out.Role.CreateDate` is not consulted.
- **Impact**: a role created today shows `dormant role (>90d)`. Its detail text reads "Nothing has assumed this role in over 90 days … then delete the role", which is false and invites the operator to delete a role that was just provisioned.
- **Fix direction**: when `LastUsedDate` is missing, raise the finding only if `CreateDate` is more than 90 days old.

## rtb

# rtb — production-code review

Scope: `core/aws/rtb.go`, `core/aws/rtb_related.go`, the `rtb` catalog entry and `colorRTB` in `core/aws/catalog_networking.go`, the `rtb` view defaults in `core/config/defaults_networking.go`, the detail navigability path (`core/semantics/projection/generic.go`, `core/fieldpath/extract.go`, `core/resource/related.go`, `core/app/actions_nav.go`), and `captureRTB` in `cmd/snapshot/ec2_network.go`.

## Findings

### 1. P2: every route's `GatewayId` links to Internet Gateways, including `local`, `vgw-*` and `vpce-*`

- **File**: `core/aws/catalog_networking.go:385` (`{FieldPath: "Routes.GatewayId", TargetType: "igw"}`)
- **Trigger**: open any route table's detail view. Every route table has the VPC `local` route, and AWS returns `GatewayId: local` for it. Routes to a virtual private gateway (`vgw-…`) and gateway-endpoint routes (`vpce-…`) also carry their target in `GatewayId`.
- **Mechanism**: `buildItems` (`core/semantics/projection/generic.go:257-269`) marks a subfield navigable when its composed path is in the nav map and its value is non-empty. It does not check the value's shape. `domain.NavigableField` (`core/domain/contracts.go:62-65`) has no prefix or discriminator. `NavIDFromValue` has no `igw` extractor (`core/resource/related.go:55-63`), so the raw value is used. `actions_nav.go:573-587` then sends a `RelatedNavigateEvent{TargetType: "igw", TargetID: "local" | "vgw-…" | "vpce-…"}`.
- **User impact**: on every route table, the `GatewayId: local` line shows as a link. Pressing Enter on it tries to open an Internet Gateway called `local`, and the same happens for VGW and endpoint routes. The operator gets a failed or empty jump. VPC-endpoint routes never reach `vpce`, even though the spec (§2 `vpce`) names `Routes[].GatewayId` with the `vpce-` prefix as the endpoint reference.
- **Fix direction**: route navigation by the value's ID prefix: `igw-` goes to `igw`, `vpce-` goes to `vpce`, and `local` and other values are not navigable. Either add a value predicate or prefix to `NavigableField` and honour it in `buildItems`, or have the projection skip nav entries whose value lacks the target's ID prefix. Fix it once in the shared path, because any other type that maps a multi-target field has the same shape.

### 2. P2: `Routes.VpcPeeringConnectionId` links to the VPC type instead of VPC Peering

- **File**: `core/aws/catalog_networking.go:388` (`{FieldPath: "Routes.VpcPeeringConnectionId", TargetType: "vpc"}`)
- **Trigger**: a route table with a peering route (`VpcPeeringConnectionId: pcx-…`). Open its detail view and press Enter on that line.
- **Mechanism**: the value `pcx-…` is dispatched as a `vpc` navigation (`core/app/actions_nav.go:581-587`), so the app looks for a VPC whose ID is `pcx-…`. A `vpc-peer` type exists (`core/aws/catalog_networking.go:733-735`, `ShortName: "vpc-peer"`, column `Path: "VpcPeeringConnectionId"`).
- **User impact**: the peering hop cannot be opened from the route table. Enter produces a not-found or failed VPC lookup instead of opening the peering connection.
- **Fix direction**: change `TargetType` to `"vpc-peer"`.

### 3. P3: detail links to a blackhole route's target still point at a deleted resource

- **File**: `core/aws/catalog_networking.go:384-387` (`Routes.NatGatewayId`, `Routes.GatewayId`, `Routes.NetworkInterfaceId`, `Routes.TransitGatewayId`). Compare `core/aws/rtb_related.go:47-51`, `75-78`, `139-142` and `165-168`.
- **Trigger**: a route table that raises `rtb.route.blackhole`, for example a route whose NAT gateway was deleted. Open detail and press Enter on that route's `NatGatewayId` (or its IGW, ENI or TGW id).
- **Mechanism**: the related checkers skip `State == blackhole` routes. Their comments say AWS keeps the stale target id, so counting it "would advertise an unopenable target". The navigable-field path has no such guard, so the same stale id renders as a link and is dispatched.
- **User impact**: the related panel and the detail links disagree. The route the row is flagged red for offers a link that opens nothing.
- **Fix direction**: use one rule for both paths. Either suppress navigability on subfields that belong to a blackhole route, or show the stale id as plain text. The rule belongs where the route list's navigability is decided, not in each checker.

## s3

# s3 — production-code review

Scope: `core/aws/s3.go`, `s3_interfaces.go`, `s3_cross_region.go`, `s3_issue_enrichment.go`, `s3_detail_enrichment.go`, `s3_related.go`, the s3 and `s3_objects` catalog entries (`catalog_databases.go`, `catalog_data.go`), `config/defaults_databases.go`, the S3 client wiring (`client.go`, `coalesce.go`), and the helpers these call (`parse.go`, `related_common.go`, `iam_roles_related.go`, `athena_related.go`, `r53.go`).

Summary: 5 findings (P0:0 P1:2 P2:1 P3:2)

---

## 1. P1: Buckets outside the session region cannot be browsed, and their posture is never checked

- **Files and lines**
  - `core/aws/client.go:184`: one S3 client, pinned to the session region: `NewCoalescingS3(s3.NewFromConfig(cfg))`.
  - `core/aws/s3.go:176-201` (`FetchS3Objects`), called from `core/aws/catalog_data.go:277`: `ListObjectsV2` goes to that client. Nothing in `core/aws` sets a per-call `o.Region`.
  - `core/aws/catalog_databases.go:165-169`: the child context carries only `{"bucket": "ID"}`, so the bucket's region never reaches the child fetcher.
  - `core/aws/s3_issue_enrichment.go:215-218` and the matching arms at 239, 256, 280, 294, 318, 333: a `PermanentRedirect` marks the bucket unreachable and skips every posture check.
- **Trigger:** `ListBuckets` lists buckets from every region, and the list shows a Region column (`catalog_databases.go:158`, filled from `Bucket.BucketRegion`). In an account with buckets in more than one region, the operator presses Enter on a bucket whose region differs from the session region.
- **User impact**
  - The object view fails with `fetching S3 objects: ... PermanentRedirect`. The objects of any bucket outside the session region cannot be browsed at all.
  - The same buckets get only a `?` from the Wave 2 posture pass. A bucket in another region that is public (`s3.public`, Broken), or that has versioning or its access block off, is never flagged.
  - Its detail view silently leaves out the policy, CORS and lifecycle blocks (`s3_detail_enrichment.go:66,81,95`).
  - Its KMS, access-log, CloudFormation and role pivots render `0+`.
  - The region needed to route these calls is already in `RawStruct` (`s3types.Bucket.BucketRegion`) and is not used.
- **Fix direction:** Route every per-bucket call to the bucket's own region by passing `func(o *s3.Options){ o.Region = bucketRegion }` as an optFn. Read the region from `BucketRegion`, carry it into the `s3_objects` child context next to `bucket`, and pass it from the posture, detail and related paths. Keep the cross-region classifier only for a bucket whose region is unknown.

## 2. P1: The coalescing S3 wrapper hides `HeadBucket`, so CloudFront's "origin bucket missing" check never runs

- **Files and lines**
  - `core/aws/coalesce.go:329-337`: `S3FullAPI` does not include `S3HeadBucketAPI`.
  - `core/aws/coalesce.go:350-351`: `coalescingS3` embeds the interface `S3FullAPI`, so its method set has no `HeadBucket`.
  - `core/aws/client.go:184`: production always installs this wrapper.
  - `core/aws/cf_issue_enrichment.go:73-76`: `clients.S3.(S3HeadBucketAPI)` is therefore always false in production. `cfBucketGoneFunc` returns nil, and at line 115 `bucketGone != nil` is false, so the finding never fires.
  - `core/aws/s3_interfaces.go:103-110` documents HeadBucket as "reached by type assertion", which the wrapper makes impossible. The live path never calls `S3HeadBucketSaysMissing` (`core/aws/s3.go:26`).
- **Trigger:** A CloudFront distribution's S3 origin points to a bucket that has been deleted, with a live, untruncated s3 cache.
- **User impact:** `CodeCFOriginBucketMissing` is never raised for a dangling S3 origin. A dangling origin is a bucket-name takeover risk, and the distribution reads as clean.
- **Fix direction:** Add `S3HeadBucketAPI` to `S3FullAPI`, or embed it in `coalescingS3` as a pass-through. That way every interface the wrapped `*s3.Client` satisfies stays reachable by type assertion.

## 3. P2: Role, SQS and Lambda notification pivots match on name only and ignore the account in the ARN

- **Files and lines**
  - `core/aws/s3_related.go:519-528` (`checkS3Role`): the principal ARN is reduced to `roleNameFromARN` and matched against role `ID`/`Name`, and role IDs are bare names (`iam_roles.go:157`). `isIAMRoleARN` (`s3_related.go:555-558`) never compares the account.
  - `core/aws/s3_related.go:76-85` (`checkS3Lambda`) and `103-112` (`checkS3SQS`): the ARN is reduced to `parts[6]` and `parts[5]`. Lambda and SQS IDs are bare names (`lambda.go:85`, `sqs.go:75`).
- **Trigger:**
  - A bucket policy grants `arn:aws:iam::<other-account>:role/deployer`, and this account also has a role named `deployer`.
  - Or the bucket notifies a cross-account SQS queue or Lambda function whose name matches a local one.
- **User impact:** The panel counts and navigates to an unrelated local role, queue or function. This contradicts the checker's own contract (`s3_related.go:443-444`: "cross-account role ARNs ... are ignored") and sends an audit pivot to the wrong resource.
- **Fix direction:** Match only when the ARN's account (and, for SQS and Lambda, its region) equals the session's. Otherwise compare full ARNs against the target's ARN field, as `checkS3SNS` already does.

## 4. P3: The KMS pivot shows 0 for a bucket encrypted with the AWS-managed `aws/s3` key

- **File and line:** `core/aws/s3_related.go:209-229` (`checkS3KMS`)
- **Trigger:** The bucket's default encryption is `SSEAlgorithm: aws:kms` with no `KMSMasterKeyID`, which means the AWS-managed `aws/s3` key.
- **User impact:** The rule is skipped (`keyID == ""` leads to `continue`), so the panel reports a proven zero KMS keys for a KMS-encrypted bucket. Spec §2 `kms` says AWS-managed `aws/s3` keys are shown as the managed default.
- **Fix direction:** When `SSEAlgorithm` is `aws:kms` or `aws:kms:dsse` and `KMSMasterKeyID` is empty, emit the `alias/aws/s3` identifier, which is the managed default, instead of skipping the rule.

## 5. P3: The Glue pivot joins on job script location instead of the crawler S3 targets the spec names

- **Files and lines:** `core/aws/s3_related.go:297-322` (`checkS3Glue`) and `core/aws/catalog_databases.go:199` ("Glue Jobs")
- **Trigger:** A bucket is the S3 target of a Glue crawler, but no Glue job's `Command.ScriptLocation` lives in that bucket.
- **User impact:** The spec (§2 `glue`) defines the relationship as crawler `Targets.S3Targets[].Path` read from `GetCrawlers`. The code only matches job script buckets, so the operator chasing a stale catalog table from the data bucket sees Glue 0. It also sees a job only because its script happens to live in the bucket.
- **Fix direction:** Implement the spec's crawler join (`GetCrawlers`, matching `S3Targets[].Path` against `s3://<bucket>`), or amend the spec contract so it names the job-script join the code performs.

## secrets

# secrets — production-code review

Scope: `core/aws/secrets*.go`, `core/aws/catalog_secrets.go`, and the production paths that relate or navigate to `secrets` (`kms_related.go`, `ecs_task.go`, `ecs_task_related_extra.go`, `ecs_svc_related_extra.go`, `core/semantics/ctevent/target.go`, `core/runtime/handlers_related.go`, `core/app/navigate.go`, `core/app/list_state.go`, `core/app/list_filter.go`, `core/iampolicy/evaluate.go`, `core/aws/identity_cache.go`).

AWS semantics were checked against the current AWS API reference pages for ListSecrets, SecretListEntry and DescribeSecret, the ECS guide page "Pass Secrets Manager secrets through Amazon ECS environment variables", and the Elastic Beanstalk guide page "Fetching secrets and parameters to Elastic Beanstalk environment variables".

Summary: 10 findings (P0:0 P1:0 P2:8 P3:2)

---

## 1. P2: Secrets scheduled for deletion are never listed, so the "scheduled for deletion" finding cannot fire

- **File:** `core/aws/secrets.go:20` (the `DELETED` branch is at `:62`; the `DELETED` skip in `core/aws/secrets_issue_enrichment.go:58` also never runs)
- **Trigger:** A secret has had `DeleteSecret` called on it and is inside its recovery window. `FetchSecretsPage` builds `ListSecretsInput{MaxResults: …}` and does not set `IncludePlannedDeletion`. According to AWS, ListSecrets "lists the secrets … not including secrets that are marked for deletion". IncludePlannedDeletion is described as follows: "By default, secrets scheduled for deletion aren't included."
- **User impact:** The row never appears, so `secret.DeletedDate != nil` never holds for a live fetch. The Broken finding `secrets.state.deleted` ("scheduled for deletion", whose Detail says "Restore it now…") is never shown. Of all the Wave 1 signals, this is the most urgent one, and it is silently missing. The secrets list and the menu count also leave out secrets that applications are already failing to read.
- **Fix direction:** Set `IncludePlannedDeletion: aws.Bool(true)` on the ListSecrets input. Also check that the related checkers that call APIs rejected for deleted secrets (GetResourcePolicy in `checkSecretsRole`) degrade cleanly for those rows.

## 2. P2: If STS identity is unavailable, every same-account principal in a secret policy is reported as "grants another account"

- **File:** `core/aws/secrets_issue_enrichment.go:48` and `:87` (with `core/iampolicy/evaluate.go:59-61`)
- **Trigger:** `accountIDFromClients` returns `""` when GetCallerIdentity fails or the identity store holds an error (`core/aws/identity_cache.go:53-59`). In that case `iampolicy.Evaluate(doc, "")` treats every principal account as foreign (`id != "" && id != ownAccount`). A normal resource policy that grants a role in the secret's own account (for example `arn:aws:iam::<own>:role/app`) then produces `secrets.cross-account-policy`.
- **User impact:** A false Warning, "resource policy grants another account", appears on healthy secrets. This happens exactly when identity lookup is degraded, such as restricted STS or a VPC without an STS endpoint. The Detail tells the operator to remove the grant.
- **Fix direction:** The owning account is already known without STS because it is part of the secret's ARN (`r.Fields["arn"]`). Use `arnAccountID(secretARN)` as the own account for each secret, and fall back to the identity store only when the ARN is missing. If neither is known, skip the cross-account classification instead of treating every account as foreign.

## 3. P2: The secrets→KMS pivot and the KMS→secrets pivot miss secrets whose `KmsKeyId` is an alias ARN

- **Files:** `core/aws/secrets_related.go:31-42` (`checkSecretsKMS`) and `core/aws/kms_related.go:123-131` (`kmsIDMatches`, used by `checkKMSSecrets`)
- **Trigger:** A secret was created or updated with `--kms-key-id alias/app-key`. AWS DescribeSecret documents `KmsKeyId` as "The key ID **or alias ARN** of the AWS KMS key". Given `arn:aws:kms:…:alias/app-key`, both functions take the text after the last `/` (`app-key`) and compare it with the KMS row ID, which is the key UUID.
- **User impact:** The secret's "KMS Keys" row shows 0. On the key's side, "Secrets Manager" also leaves this secret out. An operator who is sizing up the blast radius of disabling or deleting a key is told the secret does not depend on it.
- **Fix direction:** When the reference is an alias (the resource part starts with `alias/`, or the value is a bare `alias/…`), resolve it against the KMS rows' `alias` field, or the alias list the KMS fetcher already holds, before comparing UUIDs. Do this in one shared helper so both directions stay in sync.

## 4. P2: ECS references to a JSON key or version of a secret are never matched, in either direction

- **Files:** `core/aws/secrets_related_extra.go:251` (`secretsECSTaskRefsSecret`: `*s.ValueFrom == secretARN`), plus the reverse lane at `core/aws/ecs_task.go:292` (the raw `ValueFrom` is stored in `secret_arns`) and `core/aws/ecs_task_related_extra.go:196-201` (exact match against the secret's `arn`)
- **Trigger:** A task definition uses the documented ECS form `valueFrom: arn:aws:secretsmanager:<region>:<acct>:secret:db-AbCdEf:password::` (a JSON key, version stage, or version ID). The value never equals the bare secret ARN.
- **User impact:** secrets→"ECS Tasks" shows 0 for tasks that inject the secret, and ecs-task→"Secrets" shows 0 for the same tasks. JSON-key injection of database credentials is the most common ECS pattern, so the rotation-impact pivot misses most real consumers.
- **Fix direction:** Normalize a Secrets Manager `ValueFrom` to the ARN of the secret itself with one shared helper: the first 7 colon-separated fields (`arn:partition:secretsmanager:region:account:secret:name-XXXXXX`). Use it in both `secretsECSTaskRefsSecret` and `ecsJoinTaskDefinition` (and in `checkECSSvcSecrets`, see finding 5).

## 5. P2: ecs-svc→secrets returns raw `ValueFrom` ARNs as secret IDs, so the pivot count is inflated and the drill always opens empty

- **File:** `core/aws/ecs_svc_related_extra.go:343-344`, `:350-351`, `:357-363`
- **Trigger:** An ECS service's task definition references any Secrets Manager secret. `checkECSSvcSecrets` collects the raw `ValueFrom` / `CredentialsParameter` strings and returns them as the related IDs. A `secrets` row's ID is the secret **name** (`core/aws/secrets.go:71`). Navigation filters the target list by `r.ID` (`core/app/list_state.go:291-293`, `core/runtime/handlers_related.go:432-439`).
- **User impact:** Pressing Enter on "Secrets (N)" from a service opens an empty secrets list every time. The count is also wrong: one secret referenced through two JSON keys (`…:user::`, `…:password::`) counts as 2.
- **Fix direction:** Normalize each reference to the secret's ARN (the helper from finding 4), then resolve it against the secrets list (`Fields["arn"]` → `r.ID`), as `checkECSTaskSecrets` does. Alternatively, return the name part without the 6-character suffix only when the resolution is confirmed.

## 6. P2: secrets→Elastic Beanstalk misses the native EB secret-environment-variable mechanism

- **File:** `core/aws/secrets_related_extra.go:98` and `:132`
- **Trigger:** An EB environment injects the secret through the documented `aws:elasticbeanstalk:application:environmentsecrets` namespace. Its option value is the plain secret ARN (or `<ARN>:<json-key>`), not `{{resolve:secretsmanager:<ARN>…}}`. The checker only looks for the substring `{{resolve:secretsmanager:` + ARN.
- **User impact:** "Elastic Beanstalk" shows 0 for environments that consume the secret through the console or CLI secret-env-var feature, which is the supported path on current platforms. An operator who rotates the secret does not learn which environments need `RestartAppServer`/`UpdateEnvironment` to pick it up.
- **Fix direction:** Also match `OptionSettings` where `Namespace == "aws:elasticbeanstalk:application:environmentsecrets"` and the value equals the secret ARN or starts with `ARN + ":"`. Keep the `{{resolve:` match for ebextensions templates.

## 7. P2: secrets→CodeBuild misses ARN references that carry a JSON key or version

- **File:** `core/aws/secrets_related.go:235-236`
- **Trigger:** A CodeBuild `SECRETS_MANAGER` environment variable uses the documented `secret-id:json-key:version-stage:version-id` form with an ARN as `secret-id`, for example `arn:aws:secretsmanager:…:secret:ci-token-AbCdEf:token`. The checker accepts `val == secretARN` or a name prefix `secretName + ":"`. It never accepts `secretARN + ":"` as a prefix.
- **User impact:** "CodeBuild Projects" shows 0 for builds that read a JSON key from the secret by ARN. The operator chasing a leaked or rotated secret misses those builds.
- **Fix direction:** Add `strings.HasPrefix(val, secretARN+":")` next to the name-prefix check, or reuse the shared ARN normalizer from finding 4.

## 8. P2: A CloudTrail event's "Secret" field navigates by a suffixed or prefixed ID that no secrets row carries

- **File:** `core/semantics/ctevent/target.go:113-116` (the `resourceRefToRow` NavID) and `:231-235` (the `GetSecretValue` row, which has no NavID)
- **Trigger:**
  - An event lists the secret by ARN, `arn:…:secret:prod/api-AbCdEf`. The Secret row's NavID becomes `prod/api-AbCdEf`, which includes the random 6-character suffix.
  - A `GetSecretValue` event's `secretId` is an ARN. The row's Value becomes `secret:prod/api-AbCdEf` and it has no NavID.

  In both cases `ResolveRelatedNavigate` finds no cached row with that ID (`handlers_related.go:365`), because secret row IDs are bare names. It then falls back to a text filter (`:389-396`). `listRowMatches` (`core/app/list_filter.go:100-115`) searches the ID, name, visible cells and findings, and the ARN is not among them.
- **User impact:** From a CloudTrail event detail, Enter on the Secret field opens an empty secrets list instead of the secret. This is the main path for "who read this credential". The related-panel pivot (`ctEventsMatchTarget`) does resolve ARNs, so the panel and the field disagree.
- **Fix direction:** Resolve a secrets ARN to the row ID in one place. For example, register a `secrets` entry in `resource.navIDExtractors` that returns the full ARN, and make secrets navigation match `Fields["arn"]`. Alternatively, have the ctevent rows set TargetID to the full ARN and match it against `Fields["arn"]`, as `ctEventsMatchTarget` already does.

## 9. P3: The Log Groups pivot reports a confirmed log group that it could not read

- **File:** `core/aws/secrets_related_extra.go:286-305`
- **Trigger:** `lambda:GetFunction` on the rotation function fails (AccessDenied, throttling that is not recovered, or the function was deleted, which is itself a common cause of "rotation overdue"). The checker is also taken when `clients` is not a `*ServiceClients`. In all of these cases the checker returns `relatedResult("logs", ["/aws/lambda/<name>"])` as a definite count of 1.
- **User impact:** "Log Groups (1)" is shown as fact. If the function sets a custom `LoggingConfig.LogGroup`, this names the wrong group. If the function is gone, the group may not exist. The operator is sent to the wrong logs while debugging a broken rotation.
- **Fix direction:** On a failed or unavailable GetFunction, return `resource.UnknownRelated("logs")`, as `checkSecretsSNS` already does for the same call. Report the default group only when GetFunction succeeded and `LoggingConfig.LogGroup` is unset.

## 10. P3: Revealing a binary secret shows an empty value with no indication

- **File:** `core/aws/secrets.go:143-147`
- **Trigger:** An operator presses reveal (`x`) on a secret stored as `SecretBinary`, where `SecretString` is nil.
- **User impact:** `RevealSecret` returns `"", nil`, and the reveal view renders an empty secret as if the value were blank. The operator may conclude that the secret is empty or broken.
- **Fix direction:** When `SecretBinary` is non-nil, return a base64 rendering or an explicit "binary secret (N bytes)" message instead of an empty string.

## ses

# ses — production-code review

Scope: `core/aws/ses.go`, `ses_codes.go`, `ses_interfaces.go`, `ses_issue_enrichment.go`, `ses_related.go`, the `ses` entry in `core/aws/catalog_messaging.go`, `core/session/rule_set_store.go`, the SES refresh paths (`internal/tui/runtime_adapter_navigate.go`, `core/app/actions_list.go`), and the badge counter (`core/runtime/handlers_availability.go`).

## 1. P2 — An account-wide SHUTDOWN/PROBATION finding adds N to the issue badge instead of 1

- **File/line:** `core/aws/ses_issue_enrichment.go:62-66` copies the account finding onto every identity. `core/runtime/handlers_availability.go:1016-1047` (`unifiedIssueCount`) then counts every resource that has a Wave-2 `SevBroken` finding.
- **Trigger:** `GetAccount` returns `EnforcementStatus=SHUTDOWN` or `PROBATION` in a region with N healthy identities.
- **User impact:** The main-menu badge and the list-title issue count show N issues. The spec (`docs/resources/ses.md` §4) says "S1 counts the account-level finding once, not N times". With a count of 40, the operator sees 40 broken identities when there is one account-level cause.
- **Fix direction:** Mark account-scoped codes (`ses.account-shutdown`, `ses.account-probation`) so that `unifiedIssueCount` and `listIssueCount` count them once per type, not once per row. For example, add a catalog `AccountScoped` flag on the FindingDef and dedupe by code in the shared counter. Keep the per-row S4 decoration.

## 2. P3 — Receipt-rule recipient matching has the wrong semantics for parent-domain and leading-dot recipients

- **File/line:** `core/aws/ses_related.go:297-300` (email-address branch) and `ses_related.go:309-316` (domain branch), in `sesRuleAppliesToIdentity`.
- **Trigger:** The SES recipient rules (AWS SES Developer Guide, "Creating receipt rules": `example.com` matches that domain only, not its subdomains; `.example.com` matches all subdomains, not the parent):
  - (a) Identity `billing@sub.acme.com` with a rule whose recipient is `acme.com`. `strings.HasSuffix(dLower, "."+rLower)` counts it as a match, but AWS does not apply that rule to `sub.acme.com` mail.
  - (b) A rule whose recipient is `.acme.com`, with identity `billing@sub.acme.com` or domain identity `sub.acme.com`. Neither branch matches, because `HasSuffix("sub.acme.com", "..acme.com")` and `HasSuffix(".acme.com", ".sub.acme.com")` are both false. AWS does apply that rule.
- **User impact:** The Lambda and S3 related rows (`checkSESLambda` at `:345`, `checkSESS3` at `:373`) list functions and buckets that never receive this identity's mail (a), and give an authoritative 0 for the ones that do (b). This is the inbound-mail 3am pivot the spec calls load-bearing.
- **Fix direction:** Implement the documented matching rules: bare domain means exact domain only, a leading `.` means any subdomain, and an address means an exact address (plus `+label` variants). Drop the parent-domain suffix match.

## 3. P3 — Disabled receipt rules and disabled event destinations still produce related pivots

- **File/line:** `core/aws/ses_related.go:345-349` and `:373-377` never check `ReceiptRule.Enabled`. `ses_related.go:192-203` (eb-rule) and `:466-474` (sns) never check `EventDestination.Enabled`.
- **Trigger:** An active rule set contains a rule with `Enabled=false` that has a Lambda or S3 action. Or the identity's configuration set has an event destination with `Enabled=false` that has an SNS topic or EventBridge target.
- **User impact:** The panel shows Lambda functions, buckets, topics or EventBridge rules as receiving this identity's mail or events when SES never sends to them. That sends the operator to the wrong target during an incident.
- **Fix direction:** Skip rules with `!rule.Enabled` in the recipient filter, and skip destinations with `!dest.Enabled` in the eb-rule and sns walks.

## 4. P3 — The Route 53 pivot includes private and non-authoritative zones

- **File/line:** `core/aws/ses_related.go:51-55` (`checkSESR53`).
- **Trigger:** (a) Split-horizon DNS: a private hosted zone with the same name as the identity's domain. `r53.go` emits `private_zone=true` rows, and the matcher ignores that field. (b) Both `example.com` and a delegated `mail.example.com` exist, and the identity is `mail.example.com`. The suffix match returns both zones.
- **User impact:** The spec says this pivot exists so the operator can fix a failed verification by editing the zone that owns the records. A private zone, or a parent zone that has delegated the subdomain, is not that zone. The operator can edit records that SES verification never resolves.
- **Fix direction:** Exclude rows with `Fields["private_zone"]=="true"`, and return only the longest-matching public zone. A parent zone is a fallback only when no exact zone exists.

## 5. P3 — Wave-2 DKIM check spends its 50-call cap on email-address identities

- **File/line:** `core/aws/ses_issue_enrichment.go:81` caps the full identity list before the type filter at `:100-102`.
- **Trigger:** More than 50 identities where email addresses sort ahead of domains. This is common, because accounts often verify many individual addresses.
- **User impact:** `GetEmailIdentity` is called for each address, and those results are always discarded. Domain identities past the cap are marked "?" (uninspected), so a domain with DKIM off goes unreported. An address whose call fails is also marked "?" even though it can never carry the finding.
- **Fix direction:** Filter to `Fields["identity_type"] == sesIdentityTypeDomain` before `capAtEnrichmentCap`.

## 6. P3 — A `GetAccount` failure skips the per-identity DKIM pass entirely

- **File/line:** `core/aws/ses_issue_enrichment.go:50-57`. It returns before `sesIdentityDKIM` at `:71`, and `GetAccount` is not wrapped in `RetryOnThrottle` (the DKIM calls are).
- **Trigger:** The principal lacks `ses:GetAccount` but has `ses:GetEmailIdentity`, or `GetAccount` is throttled once.
- **User impact:** Every "DKIM not enabled" finding disappears for the type, although the DKIM check does not depend on the account call. A single throttle also drops the account-level SHUTDOWN/PROBATION signal for that pass.
- **Fix direction:** Wrap `GetAccount` in `RetryOnThrottle`. On error, record the failure and still run `sesIdentityDKIM`, then return the joined errors.

## 7. P3 — Web UI refresh never invalidates the cached SES receipt rule set

- **File/line:** `core/app/actions_list.go:170-230` (`handleActionRefresh`, the shared path the web lane `core/web` reaches through the `refresh` action) never calls `ResetRuleSets`. The swap exists only in the TUI adapter, at `internal/tui/runtime_adapter_navigate.go:586-588` and `:642-645`.
- **Trigger:** In the web UI, the operator changes or activates a receipt rule, then presses `R` on the ses list or detail.
- **User impact:** The Lambda and S3 pivots keep serving the session-lifetime cached `DescribeActiveReceiptRuleSet` answer (`core/session/rule_set_store.go`) until a profile or region switch. The TUI refreshes them and the web UI does not.
- **Fix direction:** Move the `ResetRuleSets()` call for `ses` into `Controller.handleActionRefresh` (detail and list branches). Delete the two TUI-only copies so both lanes share one refresh path.

## sfn

# sfn: production-code review

Scope: the fetcher (`core/aws/sfn.go`), detail enrichment, Wave 2 issue enrichment, related checkers, the executions and execution-history child views, the catalog entry (`core/aws/catalog_messaging.go`), view defaults, inbound pivots to sfn (alarm, eb-rule, ecs-svc), and the DescribeStateMachine coalescing decorator.

Summary: 8 findings (P0:0 P1:0 P2:2 P3:6)

---

## 1. P2: The KMS pivot mangles alias references, so the count shows 1 but navigation finds no key

- **File**: `core/aws/sfn_related.go:135` (`checkSFNKMS`)
- **Code**: `relatedResult("kms", []string{arnLastSegment(*out.EncryptionConfiguration.KmsKeyId)})`
- **Trigger**: a state machine whose `EncryptionConfiguration.KmsKeyId` is an alias name (`alias/sfn-key`) or an alias ARN (`arn:aws:kms:…:alias/sfn-key`). Step Functions accepts both. `arnLastSegment` splits on the last `/`, which gives `sfn-key`. That is neither a key ID nor an alias. The kms list keys on the key ID (`core/aws/kms.go:95`). A multi-segment alias such as `alias/team/sfn` becomes `sfn`.
- **Impact**: the related panel reports "KMS Key (1)" and the pivot lands on an empty or failed list. The operator cannot reach the CMK during a key rotation or a pending-deletion incident, which is the workflow this pivot exists for (spec §2 kms: "cross-reference the loaded `kms` list by key ID / alias / ARN").
- **Fix**: use the shared seam `kmsKeyIDFromField(raw, "sfn")` (`core/aws/related_common.go:150`), which keeps alias names intact. Or scan the loaded kms list with `matchesKMSKeyRef` (`core/aws/ssm_related.go:53`), as `checkSSMKMS` does.

## 2. P2: The logs pivot guesses the log group from a naming convention and ignores the state machine's actual logging destination

- **File**: `core/aws/sfn_related.go:39-61` (`checkSFNLogs`, line 45 `expectedLogGroup := "/aws/vendedlogs/states/" + sfnName`)
- **Trigger**: any state machine that logs to a log group with a different name. Examples are CDK or Terraform defaults, `/aws/states/<name>-Logs`, or one log group shared by several machines. The spec (§2 logs) says the target comes from `DescribeStateMachine.LoggingConfiguration.Destinations[].CloudWatchLogsLogGroup.LogGroupArn`. The checker never reads that field.
- **Impact**:
  - A state machine with logging on shows "Log Groups (0)". The operator concludes there are no logs during an incident, which is when this pivot matters most.
  - A state machine with logging off, but with a leftover log group named by convention, shows a group it does not write to.
  - The pivot also contradicts the Wave 2 `sfn.logging-off` finding, which reads the real `LoggingConfiguration`.
- **Fix**: call `sfnDescribe` (the call is already coalesced with role, kms and lambda under the detail operation). Take each `Destinations[].CloudWatchLogsLogGroup.LogGroupArn`, remove the `arn:…:log-group:` prefix and any trailing `:*`, and match the result against the loaded logs list.

## 3. P3: The Lambda pivot counts FunctionName values that are not function names

- **File**: `core/aws/sfn_related.go:186-190` (the `FunctionName` branch in `sfnCollectLambdaRefs`) and `core/aws/sfn_related.go:204-207` (`lambdaFuncNameFromARN` returns any non-ARN input unchanged)
- **Trigger**: every non-ARN `FunctionName` string is added as a Lambda ID with no validation. Three documented forms pass through wrongly:
  - JSONata state machines (`"QueryLanguage": "JSONata"`, `"Arguments": {"FunctionName": "{% $states.input.fn %}"}`) produce the ID `{% $states.input.fn %}`.
  - Qualified names (`my-fn:prod`) produce `my-fn:prod`.
  - Partial ARNs (`123456789012:function:my-fn`) pass through unchanged.

  Lambda's `Invoke` accepts all three forms.
- **Impact**: the panel overcounts Lambda functions, and the drill shows IDs that match no row in the Lambda list. Real functions referenced by a qualifier or a partial ARN never match.
- **Fix**: normalise `FunctionName` the way the ARN path already does:
  - Skip values that contain `{%`.
  - Take the name segment of a partial ARN (`…:function:NAME`).
  - Remove a trailing `:qualifier` from bare names.

  Optionally, keep only names present in the loaded lambda list, as spec §2 asks ("cross-reference the loaded `lambda` list by function name").

## 4. P3: The Wave 2 enricher does not raise Truncated when the configuration read fails, and raises it when a machine has merely been deleted

- **File**: `core/aws/sfn_issue_enrichment.go:144` (DescribeStateMachine failure, which never sets `truncated`), `core/aws/sfn_issue_enrichment.go:91-92` (ListExecutions failure, which sets `truncated = true` for every error, not-found included) and `core/aws/sfn_issue_enrichment.go:126`
- **Trigger**:
  - (a) DescribeStateMachine is throttled past the retries or denied (`states:DescribeStateMachine` missing from the role) while ListExecutions succeeds. The "!"-severity `sfn.definition-secret` check never runs, but `result.Truncated` stays false. By the `IssueEnricherResult` contract, Truncated is what marks the issue count as a lower bound.
  - (b) A state machine is deleted between the list call and the enrichment. `MarkSkipped` treats not-found as "no failure", but line 92 still sets `truncated = true`.
- **Impact**:
  - (a) The main-menu and list issue counts read as complete when a "!" check was skipped, so a machine with a credential in its definition can hide behind a count that looks exact.
  - (b) The count is marked as a lower bound ("+") when nothing was lost.
- **Fix**: drop the hand-rolled `truncated` flag. Return `Finish(&result, failures, total, "state machine executions and configuration")`, which raises Truncated exactly when a real failure was recorded, on either call.

## 5. P3: The "Last Run" column shows "OK" for executions that have not succeeded

- **File**: `core/aws/sfn_issue_enrichment.go:98` (`lastRunVal := "OK"`, written for every status except FAILED, TIMED_OUT and ABORTED)
- **Trigger**: the newest execution is RUNNING, or PENDING_REDRIVE (a failed execution being redriven).
- **Impact**: the list tells the operator the last run was OK while it is still running or recovering from a failure. For a long-running STANDARD workflow whose previous run failed, the row reads healthy and "OK".
- **Fix**: set "OK" only for `ExecutionStatusSucceeded`. Show the humanised status (for example "running" or "pending redrive") for the other non-failure statuses.

## 6. P3: The execution-history State column loses its state name at every page boundary

- **File**: `core/aws/sfn_execution_history.go:45` (`var lastStateName string`, declared per `FetchSFNExecutionHistory` call)
- **Trigger**: an execution with more events than one GetExecutionHistory page, so the operator presses "m" for more. The first events of page 2 (TaskScheduled, TaskStarted, TaskSucceeded or TaskFailed, LambdaFunction*) inherit their state name from a StateEntered event on page 1. The tracker starts empty on every call.
- **Impact**: the State column shows "—" on the first events of every later page, including the TaskFailed row the operator is looking for.
- **Fix**: seed `lastStateName` from the previous page, either by carrying it in `parentCtx` or by resolving it through `PreviousEventId`. Or scan back to the nearest StateEntered.

## 7. P3: Execution-history events are misclassified, and JSONata evaluation failures show no detail

- **Files**: `core/aws/sfn_execution_history.go:151-165` (`ClassifyEventStatus`), `core/aws/sfn_execution_history.go:211-250` (`extractFailedDetail`), `core/config/defaults_messaging.go:63-78` (`sfn_execution_history` detail paths)
- **Trigger and impact**:
  - `FailStateEntered` matches the `Entered` suffix at line 162 and is shown as "pending", but it is the event where the workflow fails.
  - The abort events `MapRunAborted`, `MapIterationAborted`, `MapStateAborted`, `ParallelStateAborted`, `TaskStateAborted` and `WaitStateAborted` match no rule and fall through to "active". Only `ExecutionAborted` is special-cased at line 159. These rows get no failure color and no `sfn-execution-history.broken.event_failed` finding.
  - `EvaluationFailed` (JSONata) is classified "failed", but `extractFailedDetail` does not read `EvaluationFailedEventDetails` (SDK v1.51.0 `types.HistoryEvent`). The Detail column shows "—" instead of the error, cause and location. The detail-view field list also omits `EvaluationFailedEventDetails`, as well as the redrive details and the Map-iteration and Map-state details.
- **Fix**:
  - Classify `*Aborted` and `FailStateEntered` as "failed".
  - Add `EvaluationFailedEventDetails` (Error/Cause) to `extractFailedDetail`.
  - Add the missing `HistoryEvent` detail paths to the `sfn_execution_history` defaults.

## 8. P3: The ecs-svc → sfn pivot matches task-definition families by substring and ignores JSONata `Arguments`

- **File**: `core/aws/ecs_svc_related_extra.go:461` and `core/aws/ecs_svc_related_extra.go:468` (`strings.Contains(td, taskDefFamily)` in `sfnASLHasECSFamily`)
- **Trigger**:
  - An ECS service whose family is `web` is reported as run by every state machine that references `webhook-processor`, `web-api:3` or `arn:…:task-definition/my-web:1`.
  - A JSONata state machine passes `TaskDefinition` under `Arguments`, not `Parameters`. The walker reads only `m["Parameters"]`, so the match is missed.
- **Impact**: the ecs-svc detail view lists unrelated Step Functions in its "Step Functions" pivot, and the operator follows false leads. Genuine JSONata-based callers are missed.
- **Fix**: parse the reference into its family (after the last `task-definition/` or `:task-definition/`, with `:revision` removed; a bare `family[:rev]` is also accepted) and compare it for equality. Check both `Parameters` and `Arguments`.

## sg

# sg — production-code review

Scope: `core/aws/sg.go`, `core/aws/sg_issue_enrichment.go`, `core/aws/sg_related.go`, the `sg` entry in `core/aws/catalog_networking.go`, the `sg` detail layout in `core/config/defaults_networking.go`, and the direct dependencies needed to verify them (`CIDROpenToEveryone` in `core/aws/parse.go`, the `security_groups` field in `core/aws/eni.go`, `relatedResourcesFor`, `ctEventsCheckerFor`, and status-cell derivation in `core/app/list_columns.go` / `core/app/list_body.go`).

## 1. P2: an untouched default security group in a dual-stack VPC is flagged "default group allows traffic"

- **File/line:** `core/aws/sg.go:210` (`isAWSDefaultEgress`), called from `sgDefaultAllowsTraffic` (`core/aws/sg.go:195`).
- **Code:** `isAWSDefaultEgress` treats the egress as "AWS-created" only when the single permission has exactly `IpRanges == [0.0.0.0/0]` **and** `len(p.Ipv6Ranges) == 0`.
- **AWS semantics (verified):** <https://docs.aws.amazon.com/vpc/latest/userguide/default-security-group.html> lists two default outbound rules for a default security group: `0.0.0.0/0 All` and `::/0 All`. AWS adds the second one "only if your VPC has an associated IPv6 CIDR block". `DescribeSecurityGroups` returns both under one `IpPermission` (`IpProtocol "-1"`, `IpRanges [0.0.0.0/0]`, `Ipv6Ranges [::/0]`).
- **Trigger:** a VPC with an IPv6 CIDR, where the operator follows the finding's own remediation and removes every ingress rule from the `default` group but leaves the AWS-created egress in place.
- **User impact:** the group stays yellow with `default group allows traffic`, adds to the NETWORKING badge, and its Attention block shows `Ingress rules 0 / Egress rules 1`. The operator gets a warning they cannot clear by following the documented fix. The finding detail says "every egress rule other than the AWS-created allow-all", and this group already meets that. The result is a false positive on every hardened default group in a dual-stack VPC.
- **Fix direction:** also accept the AWS-created shape with `Ipv6Ranges == [::/0]`: one `-1` permission whose IPv4 ranges are `[0.0.0.0/0]` and whose IPv6 ranges are empty or exactly `[::/0]`. Nothing else may be present.

## 2. P2: the `sg → sg` related pivot looks up the reverse of what the contract specifies

- **File/line:** `core/aws/sg_related.go:137-164` (`checkSGSG`), registered at `core/aws/catalog_networking.go:242` as `DisplayName: "Referencing SGs"`.
- **Contract:** `docs/related-resources.md:981` says `sg` means "Other SGs referenced in this SG's ingress/egress rules". `docs/resources/sg.md` §2 `sg` says the same: "read `IpPermissions[].UserIdGroupPairs[].GroupId` and `IpPermissionsEgress[].UserIdGroupPairs[].GroupId` on this SG".
- **Code:** `checkSGSG` does not read this SG's own `UserIdGroupPairs`. It scans the loaded `sg` list for **other** groups whose rules reference this one, which is a reverse lookup.
- **Trigger:** open detail on an app-tier SG whose ingress allows `sg-web` (web → app chain), where nothing references the app SG.
- **User impact:** the related panel shows `Referencing SGs 0`, and the operator cannot pivot to `sg-web`, the group this SG actually trusts. The "trace the chain web→app→db" workflow the contract promises only works backwards. The forward edge is not shown anywhere. Also, because the count depends on the `sg` list cache, a truncated list gives a lower-bound count, while the contract's forward read would be complete from this SG's own `RawStruct` with no cache.
- **Fix direction:** implement the contracted forward pivot. Collect the distinct `GroupId`s from this SG's `IpPermissions` and `IpPermissionsEgress` `UserIdGroupPairs`, excluding self, and return them with `relatedResult("sg", ids)` (not truncated, `NeedsTargetCache: false`). Rename the display to match. If the reverse "referencing" view is wanted too, amend `docs/related-resources.md` with a citation, as its governance rule requires.

Everything else in scope checked out against the code and AWS semantics:

- Fetch and pagination: `MaxResults` 50, which is inside the 5–1000 range, and `NextToken` handling.
- IPv4 and IPv6 internet-exposure detection through `CIDROpenToEveryone`.
- The sensitive-port set, the port enumeration, and the ranking of wide-open vs dangerous ports.
- Status-cell derivation from findings.
- The Wave 2 `sg.unused` gating on a loaded, untruncated `eni` list, and the exemption for default groups.
- The `vpc`, `ec2`, `eni`, `elb`, `lambda`, `cfn` and `ct-events` checkers, and `VpcId` navigation.

## sns-sub

# sns-sub — production code review

Scope: `core/aws/sns_sub.go`, `core/aws/sns_sub_by_topic.go`, `core/aws/sns_sub_related.go`, the `sns-sub` and `sns_subscriptions` entries in `core/aws/catalog_messaging.go`, the reverse lookups into the `sns-sub` cache (`core/aws/sqs_related.go`, `core/aws/lambda_related_extra.go`, `core/aws/sns_related.go`), `core/resource/related.go` (`BuildCloudTrailFilter`), `core/config/defaults_messaging.go`, `core/resource/columns.go`, `cmd/snapshot/messaging.go`.

Row identity used below: `sqs` rows have `ID = Name = queue name` (`core/aws/sqs.go:75-76`), `lambda` rows have `ID = Name = function name` (`core/aws/lambda.go:85-86`), and a non-confirmed `sns-sub` row has the made-up ID `<state>/<topicArn>/<protocol>/<endpoint>` (`core/aws/sns_sub.go`, `snsSubRowID`).

## 1. P2: the sns-sub → Lambda pivot matches the wrong functions by substring and ignores qualified ARNs

- **File**: `core/aws/sns_sub_related.go:52-54` and `:67`
- **Trigger**:
  - (a) A subscription with `Protocol=lambda` and an unqualified endpoint such as `arn:aws:lambda:us-east-1:123456789012:function:api`. `funcName` becomes `api`, and the check `strings.Contains(lambdaRes.ID, funcName)` matches every function whose name contains `api` (`billing-api`, `api-worker`, and so on).
  - (b) A subscription to an alias or version, such as `...:function:orders:prod`. SNS accepts qualified Lambda ARNs. Here the last `:` segment is `prod`, so the pivot matches any function containing `prod` and misses `orders`.
- **User impact**: the Related panel's "Lambda Function" pivot opens a list of unrelated functions, and in case (b) the real subscriber is missing. The spec (§2 `lambda`) says the match is by ARN and shows exactly one endpoint.
- **Fix direction**: parse the ARN. Take the segment after `:function:`, drop any `:qualifier`, and compare for equality with the lambda row's ID or `FunctionArn`. Delete the `strings.Contains` branch. Where the cached row has a `FunctionConfiguration`, also compare the ARN's region and account.

## 2. P2: the SQS → sns-sub (and SQS → sns) reverse lookup matches subscriptions of other queues that share a name prefix

- **File**: `core/aws/sqs_related.go:108` (`checkSQSSNSSub`). The same defect is in the sibling at `core/aws/sqs_related.go:48` (`checkSQSSNS`).
- **Trigger**: queue `orders` exists alongside `orders-dlq` or `orders.fifo`, and a topic subscribes `orders-dlq`. `strings.Contains("arn:aws:sqs:r:a:orders-dlq", "arn:aws:sqs:r:a:orders")` returns true.
- **User impact**: the detail view of queue `orders` lists the SNS subscriptions and topics of `orders-dlq` or `orders.fifo`. The operator is told that a topic feeds a queue it does not feed.
- **Fix direction**: compare `endpoint == queueARN` exactly. Use the name fallback only when the ARN is unknown, and require an exact final segment.

## 3. P3: the sns-sub → SQS pivot matches on queue name alone, so a queue in another region or account resolves to a local queue with the same name

- **File**: `core/aws/sns_sub_related.go:88-90` and `:103`. The reverse direction has the same problem: `core/aws/sqs_related.go:109` and `:50` evaluate the `HasSuffix(":"+queueName)` branch even when the queue ARN is known.
- **Trigger**: a topic in us-east-1 subscribes `arn:aws:sqs:us-west-2:111111111111:orders` (SNS supports cross-region and cross-account SQS subscriptions), and the current region also has a queue named `orders`.
- **User impact**: the "SQS Queue" pivot opens a local queue that receives nothing from this topic. The local queue's detail view in turn lists subscriptions that do not deliver to it. The spec (§2 `sqs`) says to match by ARN.
- **Fix direction**: compare the endpoint with the cached queue's `QueueArn` attribute (`SQSQueueAttributesRow.Attributes["QueueArn"]`). Fall back to the name only when the endpoint's region and account equal the queue's.

## 4. P3: the Lambda → sns-sub (and Lambda → sns) reverse lookup misses alias or version subscriptions and matches same-named functions in other accounts or regions

- **File**: `core/aws/lambda_related_extra.go:470` (`checkLambdaSNSSub`). The same defect is in the sibling at `core/aws/lambda_related_extra.go:432` (`checkLambdaSNS`).
- **Trigger**:
  - (a) The subscription endpoint is `...:function:orders:prod`. Neither `endpoint == fnARN` nor `HasSuffix(endpoint, ":function:orders")` holds, so no match is found.
  - (b) The endpoint is `arn:aws:lambda:eu-west-1:222222222222:function:orders`. The name-suffix branch matches the local `orders` function even though its ARN is known and differs.
- **User impact**: a Lambda triggered through an alias shows no SNS subscriptions or topics. A local function can also claim subscriptions that belong to another account's function.
- **Fix direction**: strip the qualifier from the endpoint ARN and compare the unqualified ARN with `FunctionArn`. Use the name suffix only when `FunctionArn` is unavailable.

## 5. P3: "open in console" builds a broken URL for pending, deleted and unknown subscriptions

- **File**: `core/aws/catalog_messaging.go:298`
- **Trigger**: any sns-sub row whose `SubscriptionArn` is `PendingConfirmation`, `Deleted` or empty. `r.ID` for these rows is the made-up key `pending/<topicArn>/<protocol>/<endpoint>`, and it goes into `#/subscription/<ID>`.
- **User impact**: the console link opens a URL that does not point to any subscription, often with the endpoint URL embedded in the fragment. The child view `sns_subscriptions` in the same file guards this case and returns `""` when `subscription_arn` is empty. The top-level type does not.
- **Fix direction**: build the URL from `r.Fields["subscription_arn"]` and return `""` when it is empty, as the child entry does.

## 6. P3: the CloudTrail Events pivot queries CloudTrail with the made-up row ID for non-confirmed subscriptions

- **File**: `core/aws/catalog_messaging.go:296` (`CloudTrailKey: "ResourceName:ID"`), resolved in `core/resource/related.go:748-778`
- **Trigger**: the detail view of a pending, deleted or unknown subscription. `buildFilterFromKey` sends `ResourceName=pending/arn:aws:sns:.../https/https://...` to LookupEvents.
- **User impact**: the pivot runs a CloudTrail lookup that can never match and reports a known count of 0 events. That count looks like "no activity" when the real answer is "no ARN to look up".
- **Fix direction**: use `CloudTrailKey: "ResourceName:Fields.subscription_arn"`. That field is empty for non-confirmed rows, so `buildFilterFromKey` returns nil and the checker reports no pivot instead of a fake lookup.

## 7. P3: `snsSubFindings` claims unconfirmed subscriptions are exempt from the plain-HTTP finding, but the code exempts only Deleted

- **File**: `core/aws/sns_sub.go:124-126`
- **Trigger**: an `http` subscription that is `PendingConfirmation`. The comment says "Deleted or still unconfirmed ... its transport is not a posture problem yet", but the guard tests only `snsSubDeleted`. The row therefore gets both `pending-confirmation` and `plain-http`.
- **User impact**: the two surfaces disagree with their own stated contract. Anyone who relies on the comment will expect a pending http row to show only the pending finding, but the list and the badge count it under plain-HTTP as well. The spec (§3.1) does not exempt pending rows, so the comment is what is wrong.
- **Fix direction**: correct the comment to "Deleted", or, if the exemption is intended, add `|| snsSubConfirmation(subscriptionArn) == snsSubPending`. Pick one so that the code and the comment state the same rule.

## sns

# sns — production-code review

Scope: `core/aws/sns.go`, `sns_detail_enrichment.go`, `sns_interfaces.go`, `sns_issue_enrichment.go`, `sns_related.go`, the `sns` catalog entry and `sns_subscriptions` child in `catalog_messaging.go`, the inbound `* → sns` pivots, and the helpers they call (`kmsKeyIDFromField`, `iampolicy.Evaluate`, `FetchKMSKeysByIDs`, the related-ID drill filter in `core/app`).

## 1. P2: The sns→kms pivot cuts alias key references down to their last path segment, so the drill finds no key

- **File:** `core/aws/sns_related.go:122`: `return relatedResult("kms", []string{arnLastSegment(keyID)})`
- **Trigger:** A topic encrypted with the AWS-managed key. `GetTopicAttributes` returns `KmsMasterKeyId = "alias/aws/sns"`. A customer alias (`alias/my-key`) or an alias ARN (`arn:aws:kms:…:alias/my-key`) does the same. `arnLastSegment` (`ecs_task_related.go:57`) splits on the last `/` and returns `"sns"` / `"my-key"`.
- **User impact:** The related panel shows `KMS Key (1)`. The drill then sends that ID to `FetchKMSKeysByIDs` → `DescribeKey("sns")`, which fails. The user gets an error or an empty list instead of the key. SNS's default encryption key always hits this case. It is the exact defect `kmsKeyIDFromField` (`related_common.go:122-162`) documents and exists to prevent. The drill only works for key ARNs and bare UUIDs.
- **Fix:** Use `kmsKeyIDFromField(keyID, "sns")` instead of `arnLastSegment(keyID)`.

## 2. P2: The sns→alarm reverse lookup matches by substring, so similarly named topics claim each other's alarms

- **File:** `core/aws/sns_related.go:201`, `:206`, `:211` (`snsAlarmReferences`): `strings.Contains(arn, topicARN) || arn == topicARN`
- **Trigger:** Topic `arn:aws:sns:us-east-1:123456789012:alerts` plus an alarm whose `AlarmActions` holds `arn:aws:sns:us-east-1:123456789012:alerts-critical`. The shorter ARN is a prefix of the longer one, so `Contains` is true.
- **User impact:** The `alerts` topic's `CloudWatch Alarms (N)` count includes alarms that notify a different topic, and the drill lists them. The spec calls this the primary incident pivot ("which alarms route to this channel?"), and it gives the wrong answer for any naming scheme with shared prefixes (`ops` / `ops-prod`, `alerts` / `alerts-critical`).
- **Fix:** Compare with exact equality only: `arn == topicARN`. Alarm actions are full topic ARNs.
- **Same defect elsewhere:** `core/aws/sqs_related.go:48` (`checkSQSSNS`, sqs→sns) matches with `strings.Contains(endpoint, queueARN)`. Queue `orders` therefore picks up topics subscribed to `orders-dlq`. The sns-sub pivot at `sqs_related.go:108` has the same problem.

## 3. P2: eb-rule→sns returns bare topic names, but sns rows are keyed by topic ARN, so the drill is always empty

- **File:** `core/aws/eb_rule_related.go:91-94` (`case "sns", "sqs"` keeps only the text after the last `:`), used by `checkEbRuleSNS` at `eb_rule_related.go:135-140`
- **Trigger:** An EventBridge rule targets an SNS topic. The checker returns `["my-topic"]`. The sns fetcher sets `Resource.ID = TopicArn` (`sns.go:41`). The related drill filters rows by exact `r.ID` (`core/app/list_filter.go:26`, `core/app/list_state.go:291-293`).
- **User impact:** The rule's detail shows `SNS Topics (1)`, but opening it gives an empty list. The topic can never be reached from a rule. `checkS3SNS` (`s3_related.go:87-91`) documents this exact breakage ("stripping to the bare topic name breaks drill-through") and avoids it; this sibling does not.
- **Fix:** For `sns`, return the full target ARN, as the alarm, cfn, asg, ses and s3 → sns checkers do. The `sqs` case can keep the queue name.

## 4. P2: Topic enrichment ties the subscription walk and the posture read together; one failed or throttled call hides the other's findings, including the Broken public-policy finding

- **File:** `core/aws/sns_issue_enrichment.go:74-96` (walk and early return), `:111` (posture call), `:151-155` (posture failure marks the row skipped)
- **Trigger:**
  - (a) `ListSubscriptionsByTopic` is the only SNS call here without `RetryOnThrottle` (compare `GetTopicAttributes` at `:147`). The walk runs 8-way parallel across up to 50 topics, several pages each, against a low per-account TPS quota. One throttle, or any failure on page 2 or later, reaches `MarkSkipped` and `return` at `:94-95`, so `snsTopicPosture` never runs for that topic.
  - (b) When `GetTopicAttributes` fails after a successful walk, `MarkSkipped` flags the whole row as uninspected. `FoldWave2Rows` (`core/runtime/helpers.go:90-92`) then skips the row, which throws away the `subs_count` value and the `no-subscribers` / `all-pending` findings already computed.
- **User impact:** A topic whose policy is open to anyone (`sns.public-policy`, Broken, red row) can show as uninspected with no red flag, just because an unrelated subscription listing was throttled. The comment at `:106-110` says the opposite: "Posture is read for every topic… the subscription walk's completeness has no bearing on" it. That promise does not hold on the error path.
- **Fix:** Wrap each `ListSubscriptionsByTopic` page in `RetryOnThrottle`. Run `snsTopicPosture` before, or regardless of, the walk's outcome, so a walk failure leaves the posture findings in place. Where the framework allows it, record walk and posture failures separately so that one failing does not discard the other's result.

## 5. P3: The sns→role pivot lists roles the policy denies or excludes, and resolves other accounts' roles by bare name in the local account

- **File:** `core/aws/sns_related.go:161-172` (`extractRoleNamesFromPolicy` collects every `"AWS"` key anywhere in the document), `:182-193` (`addPolicyPrincipal` reduces each ARN to a bare role name)
- **Trigger:**
  - (a) A topic policy with a `Deny` statement, or an `Allow` with `NotPrincipal: {"AWS": "arn:aws:iam::…:role/X"}`. The walk has no `Effect`/`Principal` awareness, so role `X` is reported as related.
  - (b) A policy that grants `arn:aws:iam::210987654321:role/publisher` from another account. It is reduced to `publisher` and drilled via role FetchByIDs, which looks up roles by name in the caller's account.
- **User impact:** The spec defines this pivot as "who can publish to this topic?" (IAM audit). In case (a), roles that are explicitly denied or excluded appear as grantees. In case (b), the drill fails, or opens a different local role that happens to share the name, and the audit shows the wrong principal.
- **Fix:** Parse with `iampolicy.Parse`. Take `Statement[].Principal.AWS` only from `Effect == "Allow"` statements that have no `NotPrincipal`. Keep only role ARNs whose account matches the own account (use `accountIDFromClients`, as the enricher does), or show foreign roles without a drill target.

## sqs

# sqs — production code review

Scope: `core/aws/sqs.go`, `core/aws/sqs_interfaces.go`, `core/aws/sqs_issue_enrichment.go`, `core/aws/sqs_related.go`, the `sqs` entry in `core/aws/catalog_messaging.go`, and the helpers they call (`related_common.go`, `related_shared.go`, `core/iampolicy/evaluate.go`, `core/resource/related.go`).

Summary: 5 findings (P0:0 P1:0 P2:4 P3:1)

---

## 1. [P2] The SNS and SNS-subscription pivots match the wrong queues

- **File:** `core/aws/sqs_related.go:48-50` (`checkSQSSNS`) and `core/aws/sqs_related.go:108-109` (`checkSQSSNSSub`)
- **Code:** `strings.Contains(endpoint, queueARN)`, followed by an `else if` / `||` fallback `strings.HasSuffix(endpoint, ":"+queueName)` that runs even when the queue ARN is known.
- **Trigger:**
  - (a) Queue `orders` (`arn:aws:sqs:us-east-1:111111111111:orders`) and a subscription whose endpoint is `arn:aws:sqs:us-east-1:111111111111:orders-dlq`. The first ARN is a substring of the second, so the subscription matches.
  - (b) A subscription in this region delivers to a queue with the same name in another region or account, for example `arn:aws:sqs:us-west-2:111111111111:orders`. SNS→SQS subscriptions can cross regions and accounts. The ARN check fails, but the name-suffix fallback still matches.
- **User impact:** The "SNS Topics" and "SNS Subscriptions" counts on the queue detail include topics and subscriptions that deliver to a different queue. An operator tracing "who publishes into this queue" is sent to the wrong producers.
- **Fix direction:** For `protocol == "sqs"` the endpoint is exactly the queue ARN, so compare with `endpoint == queueARN`. Use the name fallback only when `queueARN == ""`, and anchor it on region and account where they are available. Put the predicate in one function that both checkers call.

## 2. [P2] The Lambda pivot counts an alias or version as the function name, and does not retry on throttling

- **File:** `core/aws/sqs_related.go:217-229` (`checkSQSLambda`)
- **Code:** `parts := strings.Split(*m.FunctionArn, ":"); ids = append(ids, parts[len(parts)-1])`
- **Trigger:** An event-source mapping created against a qualified function, such as `arn:aws:lambda:us-east-1:111111111111:function:worker:prod`. `EventSourceMapping.FunctionArn` keeps the qualifier, so the extracted ID is `prod`, not `worker`. The call also goes to `c.Lambda.ListEventSourceMappings` without `RetryOnThrottle`, so a single throttle shows the whole pivot as an error.
- **User impact:** The "Lambda Functions" pivot lists a non-existent function (`prod`), and drilling into it finds nothing. Two aliases of the same function count as two functions. Under throttling the pivot shows an error where every other ESM pivot retries.
- **Fix direction:** Replace the body with the shared `lambdaEventSourceMappingLambdaCheck(ctx, clients, queueARN, cache)` (`core/aws/related_common.go:290`), which Kinesis and MSK already use. It uses `resource.LambdaNameFromARN`, which strips the qualifier, de-duplicates by ARN and wraps the call in `RetryOnThrottle`.

## 3. [P2] The Lambda pivot never counts functions that use this queue as their dead-letter queue

- **File:** `core/aws/sqs_related.go:205-231` (`checkSQSLambda`)
- **Trigger:** A Lambda function has `DeadLetterConfig.TargetArn` set to this queue's ARN and no event-source mapping on the queue. The spec requires this path: `docs/resources/sqs.md` §2 `lambda` defines the DLQ path (zero-cost scan of the loaded lambda list) and a combined count of consumers and DLQ users.
- **User impact:** The queue detail shows `Lambda Functions (0)` for a queue that is a function's DLQ, so "is this the DLQ of some Lambda?" gets a false "no" during an incident.
- **Fix direction:** Union the ESM result with a scan of the `lambda` target list (via `relatedResourcesFor`) for `FunctionConfiguration.DeadLetterConfig.TargetArn == queueARN`. Carry the list's truncation flag through, and set `NeedsTargetCache: true` and `Truncated: true` on the catalog entry at `core/aws/catalog_messaging.go:230`.

## 4. [P2] Every dead-letter queue is flagged "no DLQ configured"

- **File:** `core/aws/sqs_issue_enrichment.go:88-90` (`EnrichSQSAttributes`)
- **Code:** `if !hasDLQ { setWave2Finding(&result, r.ID, sqsCodeMissingDLQ, nil) }`. It fires for every queue without a `RedrivePolicy`.
- **Trigger:** A queue that is the redrive target of another queue, which is the normal setup for a DLQ. A DLQ has no `RedrivePolicy` of its own, so it always gets the warning. The spec limits the signal to the main (non-DLQ) queue, where "main" is the complement of is-DLQ detection (`docs/resources/sqs.md` §3.2: sibling `RedrivePolicy.deadLetterTargetArn` or `RedriveAllowPolicy`).
- **User impact:** Every DLQ in the account shows as a yellow row reading `no DLQ configured`. That inflates the warning count and tells the operator to give a DLQ its own DLQ.
- **Fix direction:** Before the parallel loop, collect the set of `deadLetterTargetArn` values from the rows passed in (each row's `SQSQueueAttributesRow.Attributes["RedrivePolicy"]` via `sqsRedriveTarget`). Skip `sqsCodeMissingDLQ` when the row's `Fields["arn"]` is in that set. A set `RedriveAllowPolicy` can serve as a second is-DLQ signal.

## 5. [P3] The EventBridge and Lambda pivots read only the first page and report the count as exact

- **File:** `core/aws/sqs_related.go:266-271` (`checkSQSEbRule`, `ListRuleNamesByTarget`) and `core/aws/sqs_related.go:217-219` (`checkSQSLambda`, `ListEventSourceMappings`)
- **Trigger:** A queue with more rules or mappings than fit on one page. `ListRuleNamesByTargetOutput.NextToken` and `ListEventSourceMappingsOutput.NextMarker` are ignored, and the result is built with `relatedResult(..., truncated=false)`.
- **User impact:** The count is shown as exact (no `+`) while it is really a lower bound, so the operator is not told that more rules or consumers exist.
- **Fix direction:** Loop over `NextToken` / `Marker` until the list is exhausted, or pass `truncated = next != nil` to `relatedResultTrunc`.

## ssm

# ssm — production-code review

Scope: `core/aws/ssm.go`, `core/aws/ssm_related.go`, `core/aws/ssm_interfaces.go`, the `ssm` literal in `core/aws/catalog_secrets.go`, `core/config/defaults_secrets.go` (ssm view), and the dependencies they route through: the status-cell cascade in `core/app/list_columns.go`, the KMS fetcher `core/aws/kms.go`, `core/aws/related_fetch.go`, the nav-ID extractor in `core/resource/related.go`, and `core/runtime/handlers_related.go`.

## 1. P1: two different "plaintext credential" classifiers disagree, so the Status cell says "plaintext" on a row that is green and not counted

- **File/line:** `core/aws/ssm.go:74-75` (the `risk` field uses `strings.Contains` on `/password`, `/secret`, `/token`) against `core/aws/ssm.go:20-22` and `:126-131` (`ssmColorFindings` uses `HasSuffix` on `_password`, `_secret`, `_token`, ...). The list Status column reads `risk` (`catalog_secrets.go:117`, `LifecycleKey: "risk"`) only when there are no findings (`core/app/list_columns.go`, `extractCellText`: `domain.StatusPhrase(r.Findings)` first, then `r.Fields[col.Key]`). Row colour comes from the findings (`colorSSM` → `colorFromAnyFinding` / `ssmColorFindings`).
- **Trigger:** a `String` parameter named `/prod/db/password` (or `/app/secret/api`, `/svc/token`). `risk` = `plaintext`, but `ssmColorFindings` returns nil because none of the underscore suffixes match.
- **User impact:** the Status cell shows `plaintext`, but the row is painted healthy, the menu or issue badge does not count it, and the detail view shows no Attention block. This is the spec's only Broken signal (a plaintext credential), and the colour and count surfaces miss it. The reverse also happens: `db_password` gets a Broken finding while `risk` stays empty.
- **Fix direction:** keep one classifier. Implement the spec's rule once, in `ssmColorFindings` or a shared predicate: `Type==String` and (name ends in `-password`/`-secret`/`-token`, or name contains `/password`/`/secret`/`/token`). Derive the `risk` value from that same result, or drop the `risk` branch and let the finding phrase fill the status cell.

## 2. P1: names with a hyphen suffix, which the spec requires, are never flagged

- **File/line:** `core/aws/ssm.go:20-22` (suffix list is underscore-only) and `core/aws/ssm.go:74` (contains-check requires a `/` prefix).
- **Trigger:** a `String` parameter named `prod-db-password`, `github-token` or `stripe-secret`. Spec §3.1 names the `-password` / `-secret` / `-token` suffixes explicitly.
- **User impact:** a plaintext credential gets no finding, no `risk` value, a healthy colour and no badge. The operator sees a clean row for exactly the case the Broken signal exists for.
- **Fix direction:** the same single classifier as finding 1, with the hyphen suffixes added. The existing underscore variants can stay as extras.

## 3. P2: the "not modified in over 365 days" Warning fires on every parameter type, not only SecureString

- **File/line:** `core/aws/ssm.go:133`. The stale check in `ssmColorFindings` has no type guard. `core/aws/ssm.go:72` (the `risk` field) does guard on `SecureString`, so the two sources disagree here too.
- **Trigger:** any `String` or `StringList` configuration parameter (feature flags, AMI IDs, endpoints) last written more than a year ago.
- **User impact:** false Warning rows, coloured, counted in the badge and shown with "not modified in over 365 days". Long-lived config parameters are common, so real accounts get a lot of noise that buries the rotation signal. Spec §3.1 limits this signal to `Type==SecureString`.
- **Fix direction:** gate the stale finding on `paramType == "SecureString"` (as in the spec), in the same single classifier as findings 1 and 2.

## 4. P2: the KMS related row shows 0 for SecureStrings under the default `alias/aws/ssm` key

- **File/line:** `core/aws/ssm_related.go:34-47`. It matches only against the `kms` list from `relatedResourcesFor` → `FetchRelatedTarget` → the paginated fetcher. `core/aws/kms.go:77` (`if meta.KeyManager != kmstypes.KeyManagerTypeCustomer { continue }`) drops every AWS-managed key from that list.
- **Trigger:** a SecureString parameter encrypted with the default key. `ParameterMetadata.KeyId` = `alias/aws/ssm`, and `aws/ssm` is an AWS-managed key.
- **User impact:** "KMS Key (0)" on the most common SecureString setup, which suggests the parameter has no encryption key. The spec (§2) names `alias/aws/ssm` as the expected direct field and says the count is 0 or 1.
- **Fix direction:** when `KeyId` is set and no customer key in the list matches, resolve the reference with the existing `FetchKMSKeysByIDs` (DescribeKey accepts alias names and ARNs), which already exists to cover AWS-managed keys. Return the resolved bare KeyId as the related ID.

## 5. P2: Enter on the navigable `KeyId` detail field fails for any alias value

- **File/line:** `core/aws/catalog_secrets.go:135` (`{FieldPath: "KeyId", TargetType: "kms"}`) with `core/resource/related.go:56` (`"kms": arnLastSlashSegment`), applied in `core/semantics/projection/generic.go` (top-level NavID pass). The resulting TargetID goes to `core/runtime/handlers_related.go` `HandleRelatedNavigate` → `KindFetchByIDDetail` → `FetchKMSKeysByIDs` → `DescribeKey(KeyId: "<segment>")`.
- **Trigger:** open an SSM SecureString detail whose `KeyId` is `alias/aws/ssm` (the default) or any customer alias `alias/app-key`, then press Enter on `KeyId`. The NavID becomes `ssm` / `app-key`, which no cached kms row has as its ID (the IDs are UUIDs), and DescribeKey rejects it because an alias must carry the `alias/` prefix.
- **User impact:** the pivot from a parameter to its encryption key errors out or opens nothing in the default case. It only works when `KeyId` happens to be a bare key ID or a key ARN.
- **Fix direction:** make the kms nav-ID extractor keep alias references whole: return `alias/...` as is, and turn an alias ARN into `alias/...`. Then resolve the alias to the key's UUID (DescribeKey returns `KeyMetadata.KeyId`) before the cache lookup, or match cached kms rows on their `alias` field.

## 6. P3: an alias match misses customer keys that have more than one alias

- **File/line:** `core/aws/ssm_related.go:66-70` (the `alias == keyRef` comparison) depends on `core/aws/kms.go:150` (`aliasMap[*alias.TargetKeyId] = *alias.AliasName`). That map keeps only the last alias ListAliases returns for each key.
- **Trigger:** a customer-managed key with two aliases (e.g. `alias/app` and `alias/app-prod`), and a SecureString parameter whose `KeyId` is the alias that did not survive in the map.
- **User impact:** "KMS Key (0)" even though the key is in the loaded list, so an operator rotating the key does not see that this parameter depends on it.
- **Fix direction:** match on the full set of aliases for the key. Keep every alias per KeyId from the ListAliases pass and expose them, or resolve the alias through a map built from ListAliases (`AliasName` → `TargetKeyId`) before comparing IDs.

## subnet

# subnet — production-code review

Scope: `core/aws/subnet.go`, `core/aws/subnet_codes.go`, `core/aws/subnet_related.go`,
the `subnet` entry and `colorSubnet` in `core/aws/catalog_networking.go`, the `subnet`
detail paths in `core/config/defaults_networking.go`, and the helpers they call
(`relatedResourcesFor`, `FetchRelatedTarget`, `relatedResultTrunc`, `assertStruct`).

## Findings

### 1. P2: The EFS pivot parses mount-target ENI descriptions wrongly, so it never finds a file system

- **File/line:** `core/aws/subnet_related.go:257`, `:267`, `:270`
- **Code:** `checkSubnetEFS` keeps only ENIs whose `Description` starts with `"EFS mount target for "`. It then takes the file-system ID as `strings.TrimPrefix(desc, prefix)`, which keeps the whole rest of the string.
- **Trigger:** Open the detail view of any subnet that has an EFS mount target.
  - AWS documents the mount-target ENI description as `Mount target fsmt-id for file system fs-id` ([CreateMountTarget](https://docs.aws.amazon.com/efs/latest/ug/API_CreateMountTarget.html)). That string does not start with the prefix, so the ENI is skipped.
  - The other form seen in accounts, which the spec quotes (`EFS mount target for fs-… (fsmt-…)`), has a trailing ` (fsmt-…)`. `TrimPrefix` then returns `fs-… (fsmt-…)`, which never equals an `efs` resource ID (`FileSystemId`, `core/aws/efs.go:102`).
  - With either format, `fsIDSet` is empty or never matches.
- **User impact:** The "EFS File Systems" row always shows 0 (a claim of none). During an AZ or subnet-exhaustion incident, the operator is told no file system mounts through the subnet when some do. The reverse pivot `checkEFSSubnet` (`core/aws/efs_related.go:157`) uses `strings.Contains(desc, fsID)` and does find them, so the two directions disagree.
- **Fix direction:** Get the file-system ID from the description with a pattern that works for both formats. Use a regex for `fs-[0-9a-f]{8,40}` anywhere in the description, gated on the ENI being requester-managed by EFS or on the description containing `mount target`. Alternatively, match each `efs` row ID with `strings.Contains`, as `checkEFSSubnet` does. Use one shared helper for both directions.

### 2. P3: The EFS pivot drops the ENI list's truncation flag once a mount target is found

- **File/line:** `core/aws/subnet_related.go:276` and `:296`
- **Code:** `eniTruncated` is only honoured when no mount-target ENI matched (line 276). When at least one matched, the result uses only `efsTruncated` (line 296).
- **Trigger:** The `eni` list is truncated (more than one page of ENIs, or a partial cache entry). The first page holds one of this subnet's mount-target ENIs, and another is on an unread page.
- **User impact:** The count shows as exact (for example "1") instead of a lower bound ("1+"). Other file systems mounted in the subnet go unmentioned. This only shows up once finding 1 is fixed.
- **Fix direction:** Return `relatedResultTrunc("efs", ids, eniTruncated || efsTruncated)`.

### 3. P2: The route-table pivot falls back to the VPC main table even when the route-table list is truncated

- **File/line:** `core/aws/subnet_related.go:175`
- **Code:** `if !hasExplicit && mainRTBID != "" { ids = append(ids, mainRTBID) }` runs whatever the value of `truncated` from `relatedResourcesFor(..., "rtb")`.
- **Trigger:** The account or region has more route tables than the loaded `rtb` page (the fetch or cache entry is truncated). The subnet's explicitly associated table is on an unread page, and the VPC main table is on the loaded page.
- **User impact:** The "Route Tables" pivot names the VPC main route table as this subnet's table. AWS applies the main table only when the subnet has no explicit association, so this table does not govern the subnet. The operator drills into the wrong route table while diagnosing routing, egress or IGW reachability, and the "+" suffix does not warn that the table shown may not apply at all.
- **Fix direction:** Apply the main-table fallback only when the `rtb` list is complete (`!truncated`). When the list is truncated and no explicit association was found, return a truncated zero, or fetch the association directly with `DescribeRouteTables` and filter `association.subnet-id`.

## Summary

3 findings: P0 0, P1 0, P2 2, P3 1.

## tg

# tg — production-code review

Scope: `core/aws/tg.go`, `core/aws/tg_related.go`, `core/aws/tg_issue_enrichment.go`, `core/aws/tg_health.go`, the `tg` / `tg_health` catalog entries in `core/aws/catalog_networking.go`, `core/config/defaults_networking.go`, the pivots into `tg` from other types (`elb`, `ec2`, `ecs-svc`, `asg`, `lambda`, `eb`), and the navigation plumbing they depend on (`core/resource/related.go`, `core/runtime/handlers_related.go`, `core/app/list_filter.go`).

Summary: 5 findings (P0:0 P1:0 P2:4 P3:1)

---

## 1. [P2] eb → tg pivot returns target-group ARNs as IDs, and counts a group once per listener

- **File/line**: `core/aws/eb_related_extra.go:136`, `:141`, `:158`
- **Defect**: `checkEbTG` appends raw `TargetGroupArn` values to `tgARNs` and returns them as the related IDs (`relatedResultTrunc("tg", tgARNs, ...)`). But a `tg` row's `Resource.ID` is the target-group **name** (`core/aws/tg.go:62`). The list is not de-duplicated either: an environment whose HTTP:80 and HTTPS:443 listeners both forward to the same group gets that ARN twice.
- **Trigger**: open an Elastic Beanstalk environment that has an ALB/NLB. Look at the Target Groups row, then press Enter on it.
- **User impact**: the count is inflated. On an environment with an HTTP and an HTTPS listener to one group, the panel shows 2 when the answer is 1. The drill-in shows an empty list. With several IDs, `relatedIDSubset` (`core/app/list_filter.go:20-31`) keeps only rows whose `r.ID` is in the ARN set, and no row matches. With exactly one ID, the `TargetID` is the ARN, so `relatedCacheHit` misses (`core/runtime/handlers_related.go:423-430`). The filtered-list fallback then filters by the ARN string, which is not the ID, not the name and not in any `tg` column (`core/app/list_filter.go:98-114`).
- **Fix direction**: map each ARN to the group name (the `targetgroup/<name>/<hash>` segment, as `checkECSSvcTargetGroups` already does at `core/aws/ecs_svc_related.go:45-51`). Put that in a shared helper and de-duplicate before returning.

## 2. [P2] ec2 → tg pivot claims every instance-type target group in the instance's VPC

- **File/line**: `core/aws/ec2_related.go:54` (checker registered at `core/aws/catalog_compute.go:306`)
- **Defect**: `checkEC2TargetGroups` never checks whether the instance is registered in a group. It adds every `target_type == instance` group whose `VpcId` equals the instance's VPC. The contract (`docs/related-resources.md`, ec2 section: "Target groups this instance is registered with") asks for actual registration.
- **Trigger**: a VPC with several instance-type target groups, for example one per service behind different ALBs. Open any EC2 instance detail.
- **User impact**: the related panel shows made-up relationships. Every instance appears to be behind every instance-type group in its VPC, including groups for other services. During an incident the operator pivots to target groups this instance does not serve, and the count is wrong on every instance in that VPC.
- **Fix direction**: confirm registration from `DescribeTargetHealth(TargetGroupArn)` per candidate group, filtering `Target.Id == instanceID`. This is the same call `checkTGEC2` and `checkLambdaTG` already make. Keep the VPC and `instance`-type filter only to limit the fan-out, and on per-group failures return a truncated or errored result, not a guess.

## 3. [P2] lambda → tg pivot misses functions registered by alias or version ARN

- **File/line**: `core/aws/lambda_related_extra.go:383-384`
- **Defect**: A group's target is matched only when `targetID == fnARN` (the unqualified `FunctionArn`) or `strings.HasSuffix(targetID, ":function:"+fnName)`. A qualified registration such as `arn:aws:lambda:...:function:my-fn:live` matches neither test. AWS docs recommend exactly this form ("create a function alias and include the alias in the function ARN when you register the Lambda function with the load balancer", ELB User Guide, *Use Lambda functions as targets of an Application Load Balancer*). The opposite pivot already strips the qualifier (`core/aws/tg_related.go:300-303`), so the two directions disagree.
- **Trigger**: a Lambda function registered in a lambda-type target group by its alias ARN, following the AWS-recommended setup. Open that Lambda function's detail.
- **User impact**: the Target Groups row reads 0 ("no entries"), yet the `tg` detail for the same group lists this function under Lambda Functions. The operator concludes the function is not behind any ALB.
- **Fix direction**: compare the unqualified function name from both sides. `LambdaNameFromARN(targetID) == fnName` (`core/resource/related.go:92`) already strips the `:alias` or `:version` suffix.

## 4. [P2] Navigable ARN fields into `tg` and out of `tg` to `elb` go to an empty list

- **File/line**: `core/resource/related.go:55-63` (`navIDExtractors` has no `tg` or `elb` entry); affected fields: `core/aws/catalog_networking.go:205` (`tg` detail `LoadBalancerArns` → `elb`), `core/aws/catalog_compute.go:688` (`asg` `TargetGroupARNs` → `tg`), `core/aws/catalog_compute.go:428` (`ecs-svc` `LoadBalancers.TargetGroupArn` → `tg`)
- **Defect**: These navigable fields hold full ARNs. `elb` rows are keyed by load-balancer name (`core/aws/elb.go:64`) and `tg` rows by target-group name (`core/aws/tg.go:62`). `NavIDFromValue` has no extractor for `tg` or `elb`, so `NavID` stays the raw ARN (`core/semantics/projection/generic.go:258-285`). `ResolveRelatedNavigate` then misses the cache, which compares `r.ID == ARN`, and falls back to a filtered list with `FilterText = ARN` (`core/runtime/handlers_related.go:365-400`). `elb` has no `FetchByIDs`. The list filter matches only ID, Name, rendered columns and finding phrases (`core/app/list_filter.go:98-114`). Neither the `elb` nor the `tg` columns include the ARN, so no row matches.
- **Trigger**: in a `tg` detail, press Enter on a `LoadBalancerArns` entry. Or in an `asg` detail, on a `TargetGroupARNs` entry. Or in an `ecs-svc` detail, on `LoadBalancers.TargetGroupArn`.
- **User impact**: a field shown as a link opens an empty Load Balancers or Target Groups list filtered by an ARN. The operator cannot follow the link to the resource it names.
- **Fix direction**: add `navIDExtractors` entries. For `tg`, the name is the segment after `targetgroup/`. For `elb`, the name is the segment after `loadbalancer/app|net|gwy/`. Put this in one shared ARN-to-name helper that the related checkers (findings 1 and 5) also use.

## 5. [P3] tg → elb pivot reports a lower bound although the target group lists its exact load balancers

- **File/line**: `core/aws/tg_related.go:43-75` (registered with `NeedsTargetCache: false`, `Truncated: true` at `core/aws/catalog_networking.go:191`)
- **Defect**: `TargetGroup.LoadBalancerArns` is the complete, authoritative set, and each ARN contains the load-balancer name, which is the `elb` row ID. The checker still resolves IDs only by matching against whatever `elb` rows `relatedResourcesFor` returns. Without a full `elb` cache, that is the first `DescribeLoadBalancers` page (`core/aws/related_fetch.go:44-60`). If no `elb` list is available at all, it returns Unknown (`:52-54`).
- **Trigger**: an account with more load balancers than one page (50, `core/resource/accessors.go:18`) and no loaded `elb` list. Open a target group whose load balancer is not on the first page.
- **User impact**: Load Balancers shows `(0+)`, or `?` when the elb fetch returns nothing, for a group that the API says is attached to one load balancer. At a glance the target group looks orphaned or unknown.
- **Fix direction**: build the IDs directly from `LoadBalancerArns` using the ARN-to-name helper from finding 4, and return an exact, non-truncated result. The `elb` cache is then only needed for lazy row materialisation, not for the count.

## tgw

# tgw — production-code review

Scope: `core/aws/tgw.go`, `core/aws/tgw_codes.go`, `core/aws/tgw_issue_enrichment.go`, `core/aws/tgw_related.go`, the `tgw` entry in `core/aws/catalog_networking.go` (catalog, `colorTGW`), `core/config/defaults_networking.go` (`tgw` detail paths), `core/aws/ec2_interfaces.go` (TGW interfaces), and the reverse pivots `checkRTBTGW` / `checkVPCTGW` where they bear on tgw.

Summary: 6 findings (P0:0 P1:0 P2:2 P3:4)

---

## 1. [P2] Rejected and rejecting attachments are never flagged

- **File:** `core/aws/tgw_issue_enrichment.go:113-118`
- **Code:** the state switch maps only `failed`/`failing` to `tgw.attachment-failed` and `modifying`/`pendingAcceptance`/`rollingBack` to `tgw.attachment-transitional`. Everything else hits `default: continue`.
- **Trigger:** A transit gateway has an attachment in `rejected` or `rejecting` state. For example, the TGW owner rejected a cross-account VPC attachment request. These are valid `TransitGatewayAttachmentState` values, and the `DescribeTransitGatewayAttachments` `state` filter lists them.
- **User impact:** The spec (§3.2 and the §4 row "attachment `State==rejected`/`rejecting` → Broken `!`, `attachment failed`") says these rows should be red. In the app the row stays uncoloured, "Att Issues" is blank, the detail view has no finding, and the badge does not count it. The operator never sees that the attachment was refused and carries no traffic.
- **Fix:** Add `"rejected", "rejecting"` to the `tgwCodeAttachmentFailed` case.

## 2. [P2] The VPC and Subnet pivots read only the first page of VPC attachments

- **File:** `core/aws/tgw_related.go:41-49` (`checkTGWVPC`) and `core/aws/tgw_related.go:145-153` (`checkTGWSubnet`)
- **Code:** each checker makes one `DescribeTransitGatewayVpcAttachments` call and never reads `out.NextToken`. The AWS API reference says "To retrieve the remaining results, make another call with the returned nextToken value". The Wave 2 enricher in the same package paginates the sibling `DescribeTransitGatewayAttachments` call for exactly this reason (`tgw_issue_enrichment.go:60-86`).
- **Trigger:** A hub TGW has more VPC attachments than the service returns in one page, so the response carries a `NextToken`.
- **User impact:** The VPC and Subnet counts in the related panel are too low, and drilling in leaves out VPCs and subnets that really are attached. Both pivots are registered `Truncated: false` (`catalog_networking.go:604,607`), so the panel shows the partial count as the complete answer.
- **Fix:** Loop on `NextToken`, with a page cap like `PerParentPageCap`, in one shared helper that both checkers call. Report `relatedResultTrunc(..., true)` when the cap is hit.

## 3. [P3] The VPC and Subnet pivots count attachments that no longer carry traffic

- **File:** `core/aws/tgw_related.go:53-57` (`checkTGWVPC`) and `core/aws/tgw_related.go:158-166` (`checkTGWSubnet`)
- **Code:** every returned `TransitGatewayVpcAttachment` is used whatever its `State`. The request has no `state` filter, and the loops never check `att.State`. The API's `state` filter accepts `deleted | deleting | failed | rejected | ...`, so those attachments do come back in the response.
- **Trigger:** A VPC attachment was deleted recently, or is `failed`/`rejected`. The API keeps returning deleted attachments for some time after deletion.
- **User impact:** "VPCs attached to this TGW" (spec §2) lists VPCs and subnets that cannot reach the gateway. That misleads the operator while tracing connectivity.
- **Fix:** Keep only live attachments. Either pass a `state` filter (`available`, `modifying`, `pending`, …) or skip `deleted`/`deleting`/`failed`/`rejected`/`rejecting` in the loop. Put this in the same shared helper as finding 2.

## 4. [P3] A pending-acceptance attachment is flagged at once, not after 24 hours

- **File:** `core/aws/tgw_issue_enrichment.go:115-116`
- **Code:** `pendingAcceptance` goes straight to `tgwCodeAttachmentTransitional`, and `att.CreationTime` is never read.
- **Trigger:** An attachment request was created minutes ago and is waiting for normal acceptance by the owner.
- **User impact:** The spec (§3.2 and the §4 row "attachment `State==pendingAcceptance` >24h → Warning") flags an unanswered request only after 24 hours. The code turns every fresh request yellow ("attachment between states"), which is a false positive during routine cross-account onboarding.
- **Fix:** For `pendingAcceptance`, emit the finding only when `att.CreationTime` is non-nil and more than 24h old.

## 5. [P3] The Route Tables pivot counts blackhole routes, but the reverse pivot does not

- **File:** `core/aws/tgw_related.go:82-87` (`checkTGWRTB`)
- **Code:** a route table matches on any route with `TransitGatewayId == tgwID`, including routes in `blackhole` state. The reverse pivot `checkRTBTGW` (`core/aws/rtb_related.go:174-176`) explicitly skips `ec2types.RouteStateBlackhole`. The same "rtb routes to tgw" relation is computed two different ways.
- **Trigger:** The TGW's attachment in a VPC was deleted, but a route to the TGW was left in that VPC's route table. The route is now `blackhole`.
- **User impact:** From the TGW, the route table appears under "Route Tables" (spec: "route tables that direct traffic into this TGW"). From that route table, the TGW does not appear under "Transit Gateways". The operator sees a route table that sends nothing through the gateway listed as one that does.
- **Fix:** Skip `RouteStateBlackhole` routes in `checkTGWRTB`, the same way `checkRTBTGW` does. Better still, share one predicate between the two checkers.

## 6. [P3] The IAM Role pivot shows the same account-wide role for every gateway

- **File:** `core/aws/tgw_related.go:111-126` (`checkTGWRole`)
- **Code:** the checker ignores the gateway. It calls `iam:GetRole` for the fixed name `AWSServiceRoleForVPCTransitGateway` and returns a known count of 1 whenever this account has that role.
- **Trigger:** Any transit gateway, including one owned by another account and shared through RAM (`OwnerId` differs from the caller's account).
- **User impact:** Spec §2 `role` says this pivot is for cross-account RAM-share roles, which no AWS API exposes on the gateway, and sets "Count shown: unknown". Instead the panel shows "IAM Role 1" on every TGW, and it links to this account's service-linked role even for gateways the account does not own. This is a figure the data does not support.
- **Fix:** Match the spec by returning `resource.UnknownRelated("role")`. Alternatively, record in `docs/related-resources.md` that the service-linked role is the intended target, and suppress it when the gateway's owner is not the caller's account.

## trail

# trail — production code review

Scope: `core/aws/trail.go`, `trail_codes.go`, `trail_interfaces.go`, `trail_issue_enrichment.go`, `trail_related.go`, the `trail` entry in `core/aws/catalog_monitoring.go`, `core/config/defaults_monitoring.go` (trail detail), and the dependencies they use: `core/resource/related.go` (NavIDFromValue, BuildCloudTrailFilter), `core/semantics/projection/generic.go`, `core/app/actions_nav.go`, `core/aws/s3_issue_enrichment.go`, `core/aws/s3_cross_region.go`, `core/aws/issue_enrichment.go`, and `core/aws/kms.go`.

## 1. P1: the log-bucket "public" check reads only the bucket policy and misses buckets made public by ACL

- **Location**: `core/aws/trail_issue_enrichment.go:56-58, 68` (`public := statusErr == nil && status.PolicyStatus != nil && aws.ToBool(status.PolicyStatus.IsPublic)`)
- **Trigger**: a trail delivers to a bucket that has no public policy but whose ACL grants to `AllUsers` or `AuthenticatedUsers`, and the bucket's own public access block does not set `IgnorePublicAcls`.
- **Impact**: the spec (`docs/resources/trail.md` §3.2, §4) defines this signal as "the bucket carries the `s3.public` finding". `s3.public` is also raised for ACL grants (`core/aws/s3_issue_enrichment.go:228-238`, `s3ACLPublicRows`). The trail's own Detail text also says "Remove the public grant from that bucket's policy and access control list". In this case the trail row shows no Broken `log bucket is publicly accessible` signal, so an audit log that anyone on the internet can read looks clean. The trail and s3 code paths decide "public" in two different ways, so they disagree about the same bucket.
- **Fix**: do not re-derive "public" in the trail enricher. Call the same classifier the s3 enricher uses: `scanS3BucketPosture`, or its policy-status, ACL and `IgnorePublicAcls` pieces factored into one helper. Map its `s3.public` and `s3.access-logging-off` results to the trail codes.

## 2. P2: a failed GetTrailStatus call is swallowed, so the row looks healthy instead of unknown

- **Location**: `core/aws/trail.go:71-72` (`statusOut, statusErr := api.GetTrailStatus(...)`; `if statusErr == nil && statusOut != nil { ... }`, and the error is otherwise dropped)
- **Trigger**: `GetTrailStatus` fails for one trail. Causes include throttling (there is no `RetryOnThrottle`, unlike every other per-resource call in the package), AccessDenied from an SCP or a narrow read-only policy, or a transient error.
- **Impact**: `is_logging`, `latest_delivery_error` and `latest_delivery_time` stay empty, so `trailWave1Wave2Findings` raises none of the three Broken signals (`not logging`, `delivery error`, `delivery stale`). The row carries no truncated or uninspected marker and no error is reported. A stopped trail therefore looks like a healthy one. The code comment says the row degrades to "unknown status", but nothing marks it unknown.
- **Fix**: wrap the call in `RetryOnThrottle`. On failure, record it as a partial failure (`FailedCall`, then `AggregateFailures`) and mark the row as not inspected for the status signals, the same way the enrichers use `MarkSkipped`. Do not leave the row looking clean.

## 3. P2: a log bucket in another region produces a failure on every refresh and is never inspected

- **Location**: `core/aws/trail_issue_enrichment.go:69-74` and `79-81` (every non-`NoSuchBucketPolicy` error goes to `MarkSkipped`, which adds to `failures` because only NotFound is excused, `core/aws/issue_enrichment.go:221-227`); `:86` `AggregateFailures`.
- **Trigger**: the usual setup of a multi-region trail with its bucket in the home region (for example us-east-1), viewed from any other region. `DescribeTrails` there returns the shadow trail. Calling `GetBucketPolicyStatus` or `GetBucketLogging` through the session-region S3 client gets `PermanentRedirect` or `IllegalLocationConstraintException`.
- **Impact**: on every list load in every non-home region, the trail is marked uninspected, both log-bucket signals are lost, and a "log bucket posture" failure error is raised. The sibling s3 enricher treats this exact error pair as expected multi-region topology, not a failure (`core/aws/s3_cross_region.go:32-37`, `s3_issue_enrichment.go:202-204`). The two paths disagree, and the trail path reports noise for a correct setup.
- **Fix**: resolve the bucket's region (`GetBucketLocation` or `HeadBucket`'s `x-amz-bucket-region`) and call it with a region override (`func(o *s3.Options){ o.Region = r }`). At minimum, classify `isS3CrossRegionErr` the way the s3 path does: mark the row uninspected without adding a failure. Folding this into the shared classifier from finding 1 fixes both.

## 4. P2: Enter on the trail's CloudWatch Logs log group field navigates to a log group named `*`

- **Location**: `core/resource/related.go:59` (`"logs": arnLastColonSegment`) and `:79-85`, applied to the trail detail field `CloudWatchLogsLogGroupArn` declared navigable at `core/aws/catalog_monitoring.go:218` and shown by default at `core/config/defaults_monitoring.go:31`.
- **Trigger**: any trail with CloudWatch Logs delivery. AWS returns `CloudWatchLogsLogGroupArn` in the form `arn:aws:logs:REGION:ACCOUNT:log-group:NAME:*`, and the repo documents this format itself at `core/aws/trail_related.go:47`. `NavIDFromValue` takes the text after the last `:`, which is `*`. `projection/generic.go:277-283` sets that as `NavID`, and `actions_nav.go:576-580` and the TUI then navigate to target ID `*`.
- **Impact**: the pivot from a trail to its log group, which spec §2 describes as the main CloudWatch Logs pivot, lands on a not-found result instead of the log group.
- **Fix**: give `logs` an extractor that cuts after `log-group:` and strips a trailing `:*` or `:log-stream:...`. That is what `parseTrailLogGroupName` in `trail_related.go:162-173` already does, so move that function into `resource` and use it for both the navigation and the related checker.

## 5. P3: the log group related checker matches by name only, ignoring the ARN's region and account

- **Location**: `core/aws/trail_related.go:57, 71-75` (`logRes.ID == logGroupName`)
- **Trigger**: a shadow trail of a multi-region trail, or an organization trail seen from a member account. Its `CloudWatchLogsLogGroupArn` points to a log group in the home region or the management account, while the `logs` cache holds the current region and account.
- **Impact**: if a log group with the same name exists locally (for example a common default such as `CloudTrail/DefaultLogGroup`, or the `aws-controltower/CloudTrailLogs` group), the panel shows 1 and navigates to an unrelated log group. Otherwise it shows a definite 0 for a log group that exists elsewhere.
- **Fix**: compare the ARN's region and account with the session's. If they differ, return an out-of-scope, unknown result instead of matching by name. If they match, compare against the cached log group ARN.

## 6. P3: the CloudTrail Events pivot queries the session region, not the trail's HomeRegion

- **Location**: `core/aws/catalog_monitoring.go:168, 212` (`CloudTrailKey: "ResourceName:ID"`, `ctEventsCheckerFor("trail")`), `core/aws/install.go:19-28`, `core/resource/related.go:748-778`. The filter carries only `ResourceName=<name>` and the lookup runs in the session region.
- **Trigger**: opening a shadow trail (a multi-region trail listed in a region other than its `HomeRegion`) and following the CloudTrail Events pivot.
- **Impact**: trail-management calls (`UpdateTrail`, `StopLogging`, `StartLogging`, `PutEventSelectors`) must be made in the trail's home region and are recorded in that region's event history. The pivot searches the current region, so it shows none of them. This is the "who stopped the trail" workflow that spec §2 `ct-events` names, and the spec requires the lookup to run "in the trail's `HomeRegion`".
- **Fix**: for `trail`, route the deferred ct-events lookup to `Fields["home_region"]` when it differs from the session region, or show the pivot as out of region instead of a definite empty result.

## transfer

# transfer — production-code review

Scope: `core/aws/transfer.go`, `core/aws/transfer_children.go`, `core/aws/transfer_related.go`, `core/aws/transfer_interfaces.go`, the `transfer` / `transfer_agreements` entries in `core/aws/catalog_networking.go`, `core/config/defaults_networking.go`, and the direct dependencies needed to verify them (`mwaaLogGroupNameFromARN`, `resource.LambdaNameFromARN`, `resource.NavIDFromValue`, `AggregateFailures`, the detail-enrich fold in `core/app/detail_state.go`, `internal/tui/runtime_adapter_resources.go`, and `core/runtime/handlers_resources.go`).

## 1. P2: a partial agreement enrichment is thrown away whole

- **File/line**: `core/aws/transfer_children.go:186-192`. The fold that drops it is at `core/app/detail_state.go:331-333` and `internal/tui/runtime_adapter_resources.go:210-212`.
- **Code**: `enrichTransferAgreement` returns the enriched resource together with `errors.Join(localErr, partnerErr)`. The comment says "The enriched detail is returned either way — a profile that did not resolve must not blank the agreement." Both fold paths return early when `msg.Err != nil`. They never call `ApplyDetailEnrichmentForResource`, so `EnrichedRes` is discarded. `core/runtime/handlers_resources.go:386-391` only flashes the error.
- **Trigger**: open an agreement detail where one lookup fails and the others succeed. Examples: one of a profile's `DescribeCertificate` calls is denied, throttled past retry, or times out, or the partner `DescribeProfile` fails while the local one resolves.
- **User impact**: the detail keeps the raw profile ids. Every certificate finding that did resolve is lost, including a Broken `expired` on another certificate. The user sees one generic "enrich transfer_agreements" error flash. A resolved expired certificate is not shown because a different certificate's lookup failed.
- **Fix direction**: either the fold applies `EnrichedRes` when the enricher returns partial data with an error, or the enricher reports the failed lookups as findings (a "certificate check did not run" style finding) and returns `nil` error. Fix it in the shared fold so other enrichers that return partial results are covered too.

## 2. P2: agreement rows disappear when `DescribeAgreement` fails

- **File/line**: `core/aws/transfer_children.go:82-88`
- **Code**: when `DescribeAgreement` fails or returns a nil `Agreement`, the loop records a failure and `continue`s, so no row is appended. `ListAgreements` already returned the `ListedAgreement`, which has `AgreementId`, `Description`, `Status`, `LocalProfileId`, `PartnerProfileId` and `ServerId` (SDK `types.ListedAgreement`). The parent server fetcher deliberately keeps such rows (`transfer.go:88-94`, `buildTransferDegradedResource`: "a listed server never vanishes").
- **Trigger**: the browsing role has `transfer:ListAgreements` but not `transfer:DescribeAgreement`. A per-id describe can also fail from throttling past retry or a transient error.
- **User impact**: the affected agreements vanish from the Agreements child list, and so does their `inactive: partner traffic rejected` Warning, even though `ListedAgreement.Status` carried it. If every describe fails, the list is empty apart from the aggregate error. An INACTIVE partner agreement is then invisible.
- **Fix direction**: build a degraded agreement row from the `ListedAgreement`: the list columns, the `Status` → inactive finding, and a `degradedDetailsFinding("transfer_agreements", err)`. This mirrors `buildTransferDegradedResource`. Register the matching details-denied and details-unavailable `FindingDef`s on the `transfer_agreements` catalog entry.

## 3. P2: a certificate that is not active yet is reported as Broken "expired"

- **File/line**: `core/aws/transfer_children.go:254`
- **Code**: `expired := cert.Status == transfertypes.CertificateStatusTypeInactive`. AWS `DescribedCertificate.Status` docs say `ActiveDate` and `InactiveDate` (or, if unset, the X.509 `NotBefore`/`NotAfter`) "are used to determine whether the certificate has a status of ACTIVE or INACTIVE". So a certificate whose `ActiveDate` is still in the future is also `INACTIVE`.
- **Trigger**: a profile holds a staged renewal certificate imported with a future `ActiveDate` or `NotBefore`. This is the normal AS2 rotation setup.
- **User impact**: the agreement detail shows the Broken finding `expired` ("partner connections … now fail") for a certificate that has not started yet. The operator is sent to replace a certificate that is fine.
- **Fix direction**: report expired only when `InactiveDate` (falling back to `NotAfterDate`) is in the past. Treat `INACTIVE` with a future `ActiveDate` as not-yet-active, which is not a Broken expiry.

## 4. P3: a certificate expiring within 24 hours shows "expires in 0d"

- **File/line**: `core/aws/transfer_children.go:265`
- **Code**: `days := int(remaining.Hours() / 24)` rounds down, and the finding is emitted for any `0 < remaining <= 30d`.
- **Trigger**: a certificate's `InactiveDate` is less than 24 hours away.
- **User impact**: the Warning reads `expires in 0d`, which looks like it has already expired or like a rendering glitch at the most urgent moment. Every other value also understates the time left by up to a day.
- **Fix direction**: round up, for example `int(math.Ceil(remaining.Hours() / 24))`, so the minimum shown is 1d.

## vpc-peer

# vpc-peer — production-code review

Scope reviewed: `core/aws/vpcpeer.go`, `core/aws/vpcpeer_related.go`,
`core/aws/vpcpeer_issue_enrichment.go`, the `vpc-peer` catalog entry in
`core/aws/catalog_networking.go` (lines 732-776), `core/aws/ec2_interfaces.go`
(EC2DescribeVpcPeeringConnectionsAPI), `core/config/defaults_networking.go`
(vpc-peer detail view), and the helpers they call (`cachedTypedRows`,
`relatedResultTrunc`, `assertStruct`, `wave1Finding`, `setWave2Finding`,
`ltFormatTime`, `colorAnyFindingOrHealthy`, `rtb.go` RawStruct shape).

AWS semantics checked against:

- <https://docs.aws.amazon.com/vpc/latest/peering/vpc-peering-basics.html> (lifecycle)
- <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DeleteVpcPeeringConnection.html>

## Finding 1 — P2: the rejected, failed and expired findings tell the operator to delete the connection, and AWS does not allow that

- **File/line**: `core/aws/catalog_networking.go:767` (expired), `:768` (rejected), `:769` (failed). Every S5 detail surface renders these through `catalog.Detail(code)`.
- **Trigger**: any peering connection whose `Status.Code` is `expired`, `rejected` or `failed`. `computeVpcPeerFindings` (`core/aws/vpcpeer.go:147-155`) emits these codes, and their Detail text says "Delete it and raise a new request…", "Delete it and request again…" and "Delete it and request a new one."
- **AWS facts**: DeleteVpcPeeringConnection says "You cannot delete a VPC peering connection that's in the `failed` or `rejected` state." The lifecycle doc says of Expired: "no action can be taken on it by either VPC owner". All three are dropped by AWS on its own schedule: failed after 2 h, rejected and expired after 2 days.
- **User impact**: during an incident, the operator gets a remediation step that AWS rejects. They try it in the console or CLI, get an error, and lose time. Meanwhile the step that would help, creating a new request, is presented as blocked behind the impossible delete.
- **Fix direction**: rewrite the three Detail strings. The connection can't be deleted and AWS removes it after 2 h (failed) or 2 days (rejected/expired). The action is to create a new peering request once the cause (the message shown below) is resolved. Then regenerate the findings table in `docs/resources/vpc-peer.md`.

## Finding 2 — P3: the initiating finding tells the accepter to approve a request that cannot be accepted yet

- **File/line**: `core/aws/catalog_networking.go:765` (`vpcPeerCodeInitiating` Detail).
- **Trigger**: a connection in `initiating-request`, which `core/aws/vpcpeer.go:142-143` maps to `vpc-peer.warn.initiating`.
- **AWS facts**: the lifecycle doc says of Initiating-request: "A request for a VPC peering connection has been initiated. At this stage, the peering connection can fail, or can go to `pending-acceptance`." Only a `pending-acceptance` connection can be accepted. The Detail text says the owner "has not accepted it yet" and to "Have the accepter approve it", which describes the `pending-acceptance` state instead.
- **User impact**: the operator chases the other account's owner to accept a request that the Accept API and the console will refuse. The real meaning is a transient AWS-side state: wait, and if it moves to `failed`, check the status message.
- **Fix direction**: rewrite the Detail as "AWS is still creating the request. It moves to pending acceptance or failed on its own, so wait for it", then regenerate the findings table in the spec.

## vpc

# vpc — production-code review

Scope: `core/aws/vpc.go`, `core/aws/vpc_codes.go`, `core/aws/vpc_related.go`, `core/aws/vpc_issue_enrichment.go`, the `vpc` entry in `core/aws/catalog_networking.go` (lines 255-297), plus the helpers they call (`issue_enrichment.go`, `related_shared.go`, `igw.go`/`subnet.go`/... `vpc_id` fields).

Summary: 5 findings (P0:0 P1:0 P2:2 P3:3)

---

## 1. P2 — A shared VPC is always reported as having "no active VPC flow logs"

- **File/line**: `core/aws/vpc_issue_enrichment.go:94-106` (finding emitted at line 105). Also `core/aws/vpc.go:79-129`, which never reads `Vpc.OwnerId`.
- **Trigger**: The account is a participant in VPC sharing (AWS RAM, the usual AWS Organizations "network account" setup). `DescribeVpcs` returns the shared VPC (with `OwnerId` set to the owner account). The owner has an ACTIVE flow log on the VPC or its subnets.
- **Why it happens**: AWS documents that a participant "can't describe flow logs for a VPC or subnet that was shared with you". Only flow logs on the participant's own ENIs are visible to it (VPC User Guide, "Responsibilities and permissions for owners and participants" and "Troubleshoot VPC Flow Logs"). `DescribeFlowLogs` therefore returns nothing for the shared VPC/subnet ids, `hasActive` stays false, and the enricher emits `vpc.no-flow-logs` and sets `flow_logs=no`.
- **User impact**: Every shared VPC shows a yellow `no active VPC flow logs` row, a Warning in the badge, and a remediation sentence telling the operator to enable a flow log they cannot see and are not allowed to create. In organizations that use shared VPCs this is a false positive on every participant account.
- **Fix direction**: Keep `OwnerId` on the row (a field or `RawStruct`) and compare it with the caller's account id. A VPC owned by another account cannot be inspected for flow logs, so record it through `markUninspected` with a cause that says so, or skip it. Do not raise the finding.

## 2. P2 — If the subnet lookup fails, a VPC whose subnets all have flow logs is still reported as uncovered

- **File/line**: `core/aws/vpc.go:45-47` (`if err != nil { return }` in `stampVPCSubnetIDs`), used by `core/aws/vpc_issue_enrichment.go:53`.
- **Trigger**: The page-wide `DescribeSubnets` call fails after `RetryOnThrottle`. Causes include AccessDenied on `ec2:DescribeSubnets`, persistent throttling, or a transient error. The VPC's flow logs are attached at subnet level only, which the spec (§3.2) explicitly counts as coverage.
- **Why it happens**: On error the function returns and stamps no `subnet_ids`. The enricher then asks `DescribeFlowLogs` about the VPC id alone and finds nothing ACTIVE. It cannot tell "this VPC has no subnets" apart from "the subnet list was never read", so it emits `vpc.no-flow-logs`. Nothing records that the input was incomplete.
- **User impact**: A false Warning, `no active VPC flow logs`, on VPCs that are fully logged, with no `?` or other marker to show the verdict was reached without the subnet data it depends on.
- **Fix direction**: Keep the failure. For example, stamp a marker field on the page's rows when the subnet read failed or stopped at `PerParentPageCap`. In the enricher, mark those rows uninspected (naming `DescribeSubnets`) instead of judging them on the VPC id alone.

## 3. P3 — A VPC with 200 or more subnets exceeds the EC2 filter-value limit and is never inspected

- **File/line**: `core/aws/vpc_issue_enrichment.go:53` (builds `scopes` = VPC id + every subnet id) and `:66-69` (sends them all as one `resource-id` filter).
- **Trigger**: A VPC with at least 200 subnets. The default quota is 200 subnets per VPC, and it can be raised. That makes at least 201 values in one filter, and EC2 accepts at most 200 filter values per request.
- **Why it happens**: `DescribeFlowLogs` rejects the request, and the row goes through `MarkSkipped` (line 88).
- **User impact**: The largest VPCs, where missing flow logs matter most, always show `?` for the flow-log check and add a failure to the aggregate error. They are never actually checked.
- **Fix direction**: Split `scopes` into chunks of at most 200 values, and stop as soon as one chunk returns an ACTIVE log.

## 4. P3 — The Transit Gateways pivot counts attachments that are deleted, rejected or failed

- **File/line**: `core/aws/vpc_related.go:278-300` (`checkVPCTGW`).
- **Trigger**: A VPC whose TGW attachment was recently deleted, or was rejected or failed. `DescribeTransitGatewayAttachments` returns attachments in all of these states (`deleted`, `deleting`, `failed`, `failing`, `rejected`, `rejecting`, ... are all valid values of its `state` filter). The request filters only on `resource-id` and `resource-type`, and the loop never reads `att.State`.
- **User impact**: The related panel shows "Transit Gateways: 1" and offers a pivot to a TGW the VPC is no longer attached to. That misleads an operator who is tracing inter-VPC connectivity.
- **Fix direction**: Add a `state` filter that keeps only live states (`available`, `modifying`, `pending`, `pendingAcceptance`, `initiatingRequest`), or skip attachments whose `State` is deleted, deleting, failed, failing, rejected or rejecting.

## 5. P3 — The flow-log check calls AWS once per VPC with a cap, where the spec asks for one account-wide call

- **File/line**: `core/aws/vpc_issue_enrichment.go:39` (`capAtEnrichmentCap`) and `:43-110` (one `DescribeFlowLogs` per VPC, inside `ForEachParallel`).
- **Trigger**: Any VPC list. Spec `docs/resources/vpc.md` §3.2 says: "`DescribeFlowLogs` — one account-wide call … Cost shape: account-wide (one call covers every VPC in the region)".
- **Why it happens**: The enricher makes up to 50 calls, one per row. Every row past `EnrichmentCap` (50) that it receives is marked uninspected (`CheckCap`) instead of being checked.
- **User impact**: Up to 50 API calls against a shared 10-second enrichment budget (`core/runtime/probes.go:1037`), where one call would do. Any rows beyond the cap show `?` although a single account-wide `DescribeFlowLogs` would have answered for them.
- **Fix direction**: Make one paginated `DescribeFlowLogs` call per enrichment pass, filtered to the scopes of all rows and chunked at 200 filter values (see finding 3). Group ACTIVE logs by `ResourceId`, then decide each VPC from its own id and its `subnet_ids`. Drop the per-row `EnrichmentCap`.

## vpce

# vpce — production-code review

Scope: `core/aws/vpce.go`, `core/aws/vpce_codes.go`, `core/aws/vpce_related.go`, the `vpce` catalog entry and `colorVPCE` in `core/aws/catalog_networking.go`, the `vpce` detail defaults in `core/config/defaults_networking.go`, `cmd/snapshot/ec2_network.go` (`captureVPCE`), and the direct dependencies each path needs (`iampolicy.Evaluate`, `alarmIDsByDimension`, the related-list ID filter in `core/app/list_filter.go`, and the `r53` fetcher's ID format).

## Findings

### 1. P1: state findings compare a case-varying API value with exact-case literals

- **File:line:** `core/aws/vpce.go:174` (`switch state`), where the value comes from `core/aws/vpce.go:100` (`state := string(vpce.State)`). The same comparison is at `core/aws/vpce.go:192` (`state != "Deleting" && state != "Deleted"`).
- **Trigger:** A live `DescribeVpcEndpoints` returns `State` in lower camel case. The AWS CLI reference example shows `"State": "available"`. terraform-provider-aws notes in `internal/service/ec2/consts.go` that "the State values returned from the service may be in varied case e.g. `Pending` and `pending`" and lowercases the value before comparing. The switch only matches `PendingAcceptance`, `Pending`, `Failed`, `Rejected`, `Expired`, `Partial`, `Deleting` and `Deleted`. Values such as `pendingAcceptance`, `failed` or `rejected` match no case.
- **Impact:** On real accounts, endpoints that are failed, rejected, expired, partial, pending acceptance or deleting get no finding. The row is not colored, the menu badge does not count it, and the Status column shows no phrase. The Broken signals the spec requires do not appear. The suppression at line 192 fails the same way: a `deleted` or `deleting` endpoint that still has the default policy is still flagged `endpoint policy open to anyone`. `colorVPCE` (`core/aws/catalog_networking.go:71`) sends `Fields["state"]` through the same function, so the fallback path has the same fault.
- **Fix direction:** Normalize `State` in one place before any comparison (for example `strings.ToLower`, then compare against lowercase constants, or a case-insensitive map to the finding code). Use that normalization in both `vpceFindings` and the policy-suppression check.

### 2. P2: the alarm pivot scans a dimension name that PrivateLink never publishes

- **File:line:** `core/aws/vpce_related.go:90` (`alarmIDsByDimension(ctx, clients, cache, "", "VpcEndpointId", res.ID)`).
- **Trigger:** An alarm on an `AWS/PrivateLinkEndpoints` metric such as `PacketsDropped` or `ActiveConnections` for this endpoint. AWS publishes these metrics under the dimension `VPC Endpoint Id`, with spaces (PrivateLink CloudWatch metrics documentation). No `VpcEndpointId` dimension exists. `alarmIDsByDimension` compares names exactly (`core/aws/related_common.go:234`).
- **Impact:** The CloudWatch Alarms row on the related panel shows 0 for every endpoint, even when alarms watch it. The operator concludes the endpoint has no monitoring.
- **Fix direction:** Match `VPC Endpoint Id`, optionally keeping `VpcEndpointId` as the legacy name the spec mentions, and restrict to namespace `AWS/PrivateLinkEndpoints`.

### 3. P2: the logs pivot queries flow logs by the endpoint ID, which cannot own a flow log

- **File:line:** `core/aws/vpce_related.go:106-110` (filter `resource-id` with `Values: []string{vpceID}`).
- **Trigger:** Any endpoint. Flow logs attach only to `VPC | Subnet | NetworkInterface | TransitGateway | TransitGatewayAttachment | RegionalNatGateway` (the `CreateFlowLogs` `ResourceType` valid values). A `vpce-…` ID is never a flow log's `ResourceId`, so the call always returns nothing.
- **Impact:** Log Groups always shows 0, even when flow logs cover the endpoint's VPC, subnets or ENIs. The "is traffic reaching the endpoint" pivot never works on live accounts.
- **Secondary faults on the same lines:**
  - `DescribeFlowLogs` pagination (`NextToken`) is ignored.
  - At `core/aws/vpce_related.go:123-133`, the fallback that reads `LogDestination` does not check `LogDestinationType`. An `s3` or `kinesis-data-firehose` destination ARN (for example `arn:aws:s3:::bucket/prefix`) is returned verbatim as a "log group" ID. The count is then wrong and the ID matches nothing in the logs list.
- **Fix direction:** Filter `resource-id` by the endpoint's `VpcId`, `SubnetIds[]` and `NetworkInterfaceIds[]`, as the spec says. Keep only `LogDestinationType == cloud-watch-logs`. Page through `NextToken`.

### 4. P2: the r53 pivot returns bare zone IDs that never match r53 list rows

- **File:line:** `core/aws/vpce_related.go:184` (`ids = append(ids, *z.HostedZoneId)`).
- **Trigger:** An endpoint whose VPC is associated with private hosted zones.
  - `ListHostedZonesByVPC` returns `HostedZoneSummary.HostedZoneId` in bare form (`Z111111QQQQQQQ`, per the API reference example).
  - The r53 list rows are keyed on `HostedZone.Id` from `ListHostedZones` (`core/aws/r53.go:58-60,100`), which has the `/hostedzone/Z…` form.
  - The drill-down keeps only rows whose ID is in the related ID set, using exact string matching (`core/app/list_filter.go:26`).
  - The sibling r53 checkers (`acm_related.go`, `ses_related.go`) return the cached `zoneRes.ID`, which is the prefixed form.
- **Impact:** The related panel shows "Route 53 Zones (N)", but pressing Enter opens an empty list. The count and the drill disagree.
- **Fix direction:** Convert to the r53 row ID form (prefix `/hostedzone/`) before returning, or map through one shared zone-ID normalizer used by both the fetcher and the checker.

### 5. P3: the r53 pivot ignores `ListHostedZonesByVPC` pagination and still reports a complete count

- **File:line:** `core/aws/vpce_related.go:168-187`. There is one call, `NextToken` is never read, and the result goes through `relatedResult` with `truncated=false`.
- **Trigger:** A VPC associated with more zones than one page returns (`MaxItems` default). This is common in shared-services VPCs that carry many private zones and Cloud Map/EFS service-owned zones.
- **Impact:** The count shows only the first page as an exact number. It does not render as "N+", so zones beyond the first page are silently missing.
- **Fix direction:** Loop on `NextToken`. At minimum, return `relatedResultTrunc(..., true)` when `NextToken` is non-empty.

## waf

# waf — production-code review

Scope: `core/aws/waf.go`, `core/aws/waf_interfaces.go`, `core/aws/waf_issue_enrichment.go`, `core/aws/waf_related.go`, the `waf` entry in `core/aws/catalog_security.go`, client wiring in `core/aws/client.go`, the inbound pivot `checkAlarmWAF` (`core/aws/alarm_related_extra.go`), and the related-navigation ID matching (`core/app/list_state.go`, `core/app/list_filter.go`, `core/runtime/handlers_related.go`).

AWS API semantics were checked against the current AWS API reference pages for `ListResourcesForWebACL`, `GetLoggingConfiguration`, `GetWebACL` (WAFv2), and `ListDistributionsByWebACLId` (CloudFront).

---

## 1. P1: The orphan check only looks at ALB associations, so ACLs attached to other resources are flagged "not associated with any resource"

- **File/line**: `core/aws/waf_issue_enrichment.go:89-106`
- **Code**: `ListResourcesForWebACL(ctx, &ListResourcesForWebACLInput{WebACLArn: aws.String(arn)})` is called without `ResourceType`, and `len(assocOut.ResourceArns) == 0` then raises `waf.orphan`.
- **AWS semantics**: "If you don't provide a resource type, the call uses the resource type `APPLICATION_LOAD_BALANCER`." Other valid types are `API_GATEWAY | APPSYNC | COGNITO_USER_POOL | APP_RUNNER_SERVICE | VERIFIED_ACCESS_INSTANCE | AMPLIFY | AGENTCORE_GATEWAY`.
- **Trigger**: A REGIONAL web ACL is attached only to an API Gateway REST stage, an AppSync API, a Cognito user pool, and so on, with no ALB.
- **User impact**: The row turns warning-coloured with the text "not associated with any resource", and the detail text tells the operator to "associate it ... or delete it". That is false advice about an ACL that is protecting production traffic. The related panel for the same ACL correctly shows API Gateways (1), so the list and the detail view contradict each other.
- **Fix direction**: Call `ListResourcesForWebACL` once for each supported regional `ResourceType` and stop at the first non-empty result. Raise the orphan finding only when every type comes back empty. Share this lookup with `checkWAFELB` and `checkWAFAPIGW` so the finding and the panel come from one source.

## 2. P1: CLOUDFRONT-scope ACLs are enriched and related through the session-region WAFv2 client

- **Files/lines**:
  - `core/aws/waf_issue_enrichment.go:67-71` (`GetLoggingConfiguration`), `:89-93` (`ListResourcesForWebACL`), `:113-124` (`GetWebACL` with `Scope: CLOUDFRONT`). All three use `clients.WAFv2`.
  - `core/aws/waf_related.go:74-76` (`checkWAFLogs`), `:32-35` (`checkWAFELB`), `:170-173` (`checkWAFAPIGW`). All three use `c.WAFv2` whatever `res.Fields["scope"]` is.
  - `core/aws/client.go:213-214`: only `WAFv2` is bound to the session region. `WAFv2CloudFront`, pinned to us-east-1, is used only by the fetcher (`catalog_security.go:329`).
- **AWS semantics**: For `Scope=CLOUDFRONT`, "API and SDKs - For all calls, use the Region endpoint us-east-1" (GetWebACL). For `ListResourcesForWebACL`: "For Amazon CloudFront, don't use this call. Instead, use the CloudFront call `ListDistributionsByWebACLId`."
- **Trigger**: The session region is anything other than us-east-1. The fetcher lists CLOUDFRONT ACLs in every region (`waf.go:113-132`), so each one then goes through Wave 2 and the related panel on the regional client. In us-east-1 the endpoint is correct, but `ListResourcesForWebACL` is still invalid for a CloudFront ACL.
- **User impact**:
  - Outside us-east-1, `GetWebACL(Scope=CLOUDFRONT)` fails on every CloudFront ACL, so `MarkSkipped` drops all its findings and each Wave 2 pass reports a partial failure.
  - If the wrong-region `GetLoggingConfiguration` comes back as `WAFNonexistentItemException`, the ACL is recorded as "Logging off". In the related panel, `checkWAFLogs` then shows a definitive "Log Groups (0)" even when logging is configured.
  - `checkWAFELB` and `checkWAFAPIGW` either error or return 0 by accident. They should short-circuit to a known 0, because CLOUDFRONT ACLs cannot bind ALBs or API Gateway stages.
  - In us-east-1, `ListResourcesForWebACL` on a CloudFront ACL gives an empty result or an error. Either the ACL is falsely flagged as an orphan even though distributions use it, or it is skipped.
- **Fix direction**: Add one helper that picks the WAFv2 client from `Fields["scope"]` (`WAFv2CloudFront` for CLOUDFRONT), and route every per-ACL call through it. For CLOUDFRONT ACLs, work out the orphan status from `cloudfront:ListDistributionsByWebACLId`, the same source `checkWAFCF` uses, and return known 0 for elb and apigw.

## 3. P2: The "Log Groups" panel counts Firehose and S3 destinations as log groups

- **File/line**: `core/aws/waf_related.go:107-109`
- **Code**: For any destination that is not a CloudWatch Logs ARN, `ids = append(ids, d) // Firehose / S3 destinations: pass through full ARN`.
- **Spec**: `docs/resources/waf.md` §2 `logs`: "filter for ARNs beginning with `arn:aws:logs:` ... others are not the target's scope."
- **Trigger**: A web ACL logs to a Kinesis Firehose stream (`aws-waf-logs-*`) or to an S3 bucket.
- **User impact**: The panel shows "Log Groups (1)". Log-group resource IDs are group names (`cwlogs.go:114`), and related navigation matches on `r.ID` only (`list_state.go:292`, `list_filter.go:26`). Enter therefore opens an empty filtered log-group list, and the count claims a log group that does not exist.
- **Fix direction**: Keep only destinations with the `arn:aws:logs:` prefix and `:log-group:`, and drop everything else.

## 4. P2: The alarm → WAF pivot passes the ACL name, but WAF rows are keyed by Id

- **File/line**: `core/aws/alarm_related_extra.go:161-162`
- **Code**: `if v := alarmDimension(alarm, "WebACL"); v != "" { return relatedResult("waf", []string{v}) }`. The `WebACL` dimension value is the ACL **name**.
- **Mismatch**: WAF resources set `ID: id` (the WebACL UUID) at `core/aws/waf.go:225`. Related navigation matches IDs exactly against `r.ID` (`core/runtime/handlers_related.go:423-429`, `core/app/list_state.go:291-293`, `core/app/list_filter.go:26`).
- **Trigger**: An operator opens a CloudWatch alarm on an `AWS/WAFV2` metric and presses Enter on "WAF Web ACLs (1)".
- **User impact**: The panel promises one ACL, but navigation never gets a cache hit and opens an empty filtered WAF list. The pivot is broken for every WAF alarm.
- **Fix direction**: Resolve the name against the cached `waf` rows (match `Fields["name"]`, and the `Region` dimension or scope where it is present) and return the matching row IDs. Use `relatedResultTrunc` when the WAF list is incomplete.

## 5. P3: `checkWAFCF` ignores CloudFront pagination and reports a truncated count as exact

- **File/line**: `core/aws/waf_related.go:141-156`
- **AWS semantics**: `ListDistributionsByWebACLId`: "MaxItems ... The maximum and default values are both 100". The response carries `IsTruncated` and `NextMarker`.
- **Trigger**: A CLOUDFRONT web ACL is attached to more than 100 distributions.
- **User impact**: The panel shows "CloudFront (100)" as an exact number with no truncation marker, and the remaining distributions cannot be reached from the pivot.
- **Fix direction**: Loop on `Marker`/`NextMarker` while `IsTruncated` is true, or return `relatedResultTrunc` when `IsTruncated` is set.

## 6. P3: Logging that Security Lake or a CloudWatch telemetry rule manages is reported as "no logging configuration"

- **Files/lines**: `core/aws/waf_issue_enrichment.go:67-79`, `core/aws/waf_related.go:74-81`
- **AWS semantics**: For `GetLoggingConfiguration`, `LogScope` defaults to `CUSTOMER`. Configurations managed through Security Lake (`SECURITY_LAKE`) or CloudWatch Logs telemetry rules (`CLOUDWATCH_TELEMETRY_RULE_MANAGED`) are separate log scopes.
- **Trigger**: An ACL's WAF logs are collected only through Security Lake or a CloudWatch telemetry rule.
- **User impact**: The `CUSTOMER`-scope call returns `WAFNonexistentItemException`. The row is flagged "no logging configuration", with detail text saying requests "leave no trace", even though the logs exist.
- **Fix direction**: On `WAFNonexistentItemException` for `CUSTOMER`, also query the `SECURITY_LAKE` and `CLOUDWATCH_TELEMETRY_RULE_MANAGED` log scopes, and raise `waf.no-logging` only when every scope is absent.

## 7. P3: The `waf.no-rules` detail text is false for ACLs whose default action is Block

- **Files/lines**: `core/aws/waf_issue_enrichment.go:138-142` raises `waf.no-rules` from `len(Rules)==0` alone. The catalog detail text at `core/aws/catalog_security.go:347` says "every request reaches the protected resource and the ACL provides no protection at all."
- **Trigger**: A web ACL has zero rules and `DefaultAction.Block` set. This is a deliberate deny-all ACL, for example on a maintenance or internal-only endpoint.
- **User impact**: The detail view tells the operator that all traffic gets through, when the ACL actually blocks every request. That is a wrong statement about the resource's security posture.
- **Fix direction**: `getOut.WebACL.DefaultAction` is already fetched. Either raise the finding only when `DefaultAction.Allow != nil`, or word the detail so it holds for both defaults.
