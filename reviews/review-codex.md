
## acm

- P1 — [acm.go:87](/Users/k2m30/projects/a9s/core/aws/acm.go:87), [security.go:435](/Users/k2m30/projects/a9s/cmd/snapshot/security.go:435)  
  Trigger: an account has RSA-3072/4096, ECDSA, or ACME-issued certificates. Both empty `ListCertificatesInput` requests accept ACM’s restrictive defaults.  
  Impact: those certificates—and their expiry, weak-key, and relationship signals—are absent from both ACM inventories. ACM documents that default listing returns only RSA-2048 and excludes ACME certificates. [AWS ListCertificates API](https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html)  
  Fix: explicitly include every supported key type and all three certificate key-pair origins.

- P2 — [apigw_related.go:255](/Users/k2m30/projects/a9s/core/aws/apigw_related.go:255)  
  Trigger: an API Gateway custom domain has an ACM certificate. The checker returns only the certificate ARN’s final UUID segment.  
  Impact: the related ACM drill-down cannot match the ACM resource, whose ID is the complete certificate ARN ([acm.go:145](/Users/k2m30/projects/a9s/core/aws/acm.go:145)); it yields an empty/unresolvable result.  
  Fix: retain and return the full certificate ARN.

- P2 — [acm_related.go:133](/Users/k2m30/projects/a9s/core/aws/acm_related.go:133)  
  Trigger: `DescribeCertificate.InUseBy` contains an API Gateway `/domainnames/<name>` ARN.  
  Impact: ACM reports the domain name as an `apigw` resource ID, but the API inventory is keyed by API ID, so navigation cannot reach the APIs using the certificate.  
  Fix: resolve each custom domain through its API mappings and return the mapped API IDs.

- P2 — [apigw_related.go:217](/Users/k2m30/projects/a9s/core/aws/apigw_related.go:217), [apigw_related.go:232](/Users/k2m30/projects/a9s/core/aws/apigw_related.go:232)  
  Trigger: API Gateway has more custom domains or mappings than one response page.  
  Impact: ACM relationships are silently incomplete but reported as exact. Both calls are paginated; mappings expose a continuation token. [AWS API mappings reference](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/domainnames-domainname-apimappings.html)  
  Fix: paginate `GetDomainNames` and each `GetApiMappings` call, or mark incomplete results as truncated.

- P2 — [r53_related.go:306](/Users/k2m30/projects/a9s/core/aws/r53_related.go:306)  
  Trigger: a hosted zone contains ACM DNS-validation CNAME records, especially for a certificate with multiple validated names.  
  Impact: the relation count represents validation records, not certificates, and emits record names such as `_token.example.com` as ACM IDs. ACM resources use certificate ARNs, so drill-down is wrong or empty.  
  Fix: map validation records to certificate ARNs before reporting ACM resources; otherwise expose these as validation-record data rather than ACM resources.

- P2 — [acm_related.go:205](/Users/k2m30/projects/a9s/core/aws/acm_related.go:205)  
  Trigger: a validation name such as `_x.notexample.com` is evaluated with a hosted zone named `example.com`.  
  Impact: the bare suffix check falsely attributes the validation record to that zone, directing remediation to the wrong Route 53 zone.  
  Fix: require a DNS-label boundary: exact equality or `strings.HasSuffix(recordName, "."+zoneName)`.

- P3 — [catalog_dns_cdn.go:167](/Users/k2m30/projects/a9s/core/aws/catalog_dns_cdn.go:167)  
  Trigger: a user configures a key-based ACM column for `certificate_arn` or `key_algorithm`.  
  Impact: catalog metadata rejects them as unproduced even though the fetcher emits both fields ([acm.go:158](/Users/k2m30/projects/a9s/core/aws/acm.go:158), [acm.go:164](/Users/k2m30/projects/a9s/core/aws/acm.go:164)).  
  Fix: add both keys to ACM’s `FieldKeys`.

## alarm

Findings:

- **P1** — [core/aws/alarm.go:20](/Users/k2m30/projects/a9s/core/aws/alarm.go:20), [core/aws/alarm.go:33](/Users/k2m30/projects/a9s/core/aws/alarm.go:33), [cmd/snapshot/ops.go:517](/Users/k2m30/projects/a9s/cmd/snapshot/ops.go:517), [cmd/snapshot/ops.go:523](/Users/k2m30/projects/a9s/cmd/snapshot/ops.go:523)  
  Trigger: an account has composite or log alarms.  
  Impact: they are absent from both the application and alarm capture output; AWS defaults `DescribeAlarms` to metric alarms when `AlarmTypes` is omitted. [AWS API](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_DescribeAlarms.html)  
  Fix: fetch, model, render, and relate all supported alarm types; preserve type-specific fields and pagination.

- **P2** — [core/aws/alarm.go:113](/Users/k2m30/projects/a9s/core/aws/alarm.go:113)  
  Trigger: a metric alarm is `ALARM` or `INSUFFICIENT_DATA` and has no `AlarmActions`.  
  Impact: the UI reports only state, concealing that the alarm cannot invoke an action when it fires.  
  Fix: emit the no-actions finding independently of the state finding.

- **P2** — [core/aws/alarm_history.go:76](/Users/k2m30/projects/a9s/core/aws/alarm_history.go:76), [core/aws/alarm_history.go:95](/Users/k2m30/projects/a9s/core/aws/alarm_history.go:95), [core/resource/resource.go:15](/Users/k2m30/projects/a9s/core/resource/resource.go:15)  
  Trigger: multiple history records share a UTC minute and one arrives on a later page.  
  Impact: pagination deduplicates by the minute-derived ID and silently drops history entries.  
  Fix: derive a stable unique ID from the full event identity while retaining the formatted timestamp solely for display.

- **P2** — [core/aws/alarm_related_extra.go:31](/Users/k2m30/projects/a9s/core/aws/alarm_related_extra.go:31), [core/aws/catalog_monitoring.go:90](/Users/k2m30/projects/a9s/core/aws/catalog_monitoring.go:90)  
  Trigger: a REST API Gateway alarm has the normal `ApiName` dimension.  
  Impact: the relation returns an API name where navigation requires the API ID, producing an empty drill-in. API Gateway publishes `ApiName` for REST API metrics. [AWS documentation](https://docs.aws.amazon.com/en_en/apigateway/latest/developerguide/api-gateway-metrics-and-dimensions.html)  
  Fix: resolve the reported API name to the cached canonical API ID and require the target cache for this relation.

- **P2** — [core/aws/waf_related.go:54](/Users/k2m30/projects/a9s/core/aws/waf_related.go:54), [core/aws/alarm_related_extra.go:161](/Users/k2m30/projects/a9s/core/aws/alarm_related_extra.go:161), [core/aws/waf.go:224](/Users/k2m30/projects/a9s/core/aws/waf.go:224)  
  Trigger: a Web ACL uses a CloudWatch metric name different from its Web ACL name.  
  Impact: WAF↔alarm relations are missed or navigate to an invalid ID: the `WebACL` dimension is the configured metric name, while WAF resources use the ACL ID. [AWS documentation](https://docs.aws.amazon.com/waf/latest/developerguide/waf-metrics.html)  
  Fix: obtain each ACL’s metric name (or use an ARN dimension where available), then resolve it to the canonical WAF ID before returning the relation.

- **P2** — [core/aws/alarm_related_extra.go:182](/Users/k2m30/projects/a9s/core/aws/alarm_related_extra.go:182)  
  Trigger: any CloudWatch alarm operation exists in the CloudTrail cache.  
  Impact: every alarm shows every monitoring event whose operation name contains `Alarm`, regardless of which alarm it affected.  
  Fix: filter by this alarm’s identity, preferably by using the existing `ResourceName:ID` CloudTrail pivot.

- **P2** — [core/aws/ecs_related.go:56](/Users/k2m30/projects/a9s/core/aws/ecs_related.go:56), [core/aws/eks_related.go:50](/Users/k2m30/projects/a9s/core/aws/eks_related.go:50), [core/aws/alarm_related_extra.go:78](/Users/k2m30/projects/a9s/core/aws/alarm_related_extra.go:78)  
  Trigger: ECS and EKS clusters share a name, especially with Container Insights alarms.  
  Impact: alarms are cross-linked to the wrong cluster type; `ECS/ContainerInsights` also passes the EKS `Contains("ContainerInsights")` check. ECS and EKS use distinct Container Insights namespaces. [ECS](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-metrics-ECS.html), [EKS](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/container-insights-eks-classic-metrics.html)  
  Fix: use strict, service-specific namespace predicates in both source-to-alarm and alarm-to-source checkers.

- **P2** — [core/aws/eb_related.go:185](/Users/k2m30/projects/a9s/core/aws/eb_related.go:185), [core/aws/eb_related.go:191](/Users/k2m30/projects/a9s/core/aws/eb_related.go:191)  
  Trigger: an Elastic Beanstalk environment name matches any unrelated alarm dimension value or appears in an unrelated alarm name.  
  Impact: unrelated alarms appear in the environment’s related panel.  
  Fix: match only documented Elastic Beanstalk dimensions and namespaces; remove the unqualified alarm-name substring fallback.

## ami

- **P1** — Disabled AMIs are omitted from every AMI fetch path.  
  Location: [core/aws/ami.go:46](/Users/k2m30/projects/a9s/core/aws/ami.go:46), [core/aws/ami.go:77](/Users/k2m30/projects/a9s/core/aws/ami.go:77), [cmd/snapshot/ec2_network.go:183](/Users/k2m30/projects/a9s/cmd/snapshot/ec2_network.go:183)  
  Trigger: A self-owned AMI is disabled. `DescribeImages` excludes disabled AMIs unless `IncludeDisabled` is set.  
  Impact: The AMI list/snapshot omits it, and AMI pivots from resources that still reference it fail to resolve it.  
  Fix: Set `IncludeDisabled: aws.Bool(true)` in both normal and by-ID fetches, and in the snapshot paginator. [AWS documentation](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/disable-an-ami.html)

- **P1** — ASG, node-group, and EKS-cluster AMI resolution selects `$Latest` when no launch-template version is specified.  
  Location: [core/aws/asg_related.go:124](/Users/k2m30/projects/a9s/core/aws/asg_related.go:124), [core/aws/ng_related.go:204](/Users/k2m30/projects/a9s/core/aws/ng_related.go:204), [core/aws/eks_related_extra.go:185](/Users/k2m30/projects/a9s/core/aws/eks_related_extra.go:185)  
  Trigger: A consumer omits its template version, and the template’s default version differs from its latest version.  
  Impact: The UI reports and navigates to the wrong AMI; node-group surfaces can disagree with the fetch-time AMI enrichment.  
  Fix: Default to `$Default` and share the version-resolution helper. [Auto Scaling documentation](https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_LaunchTemplateSpecification.html), [EKS API reference](https://docs.aws.amazon.com/eks/latest/APIReference/eks-api.pdf)

- **P1** — AMI → Auto Scaling Group relations only inspect currently recorded EC2 instances, not group launch configuration/template references.  
  Location: [core/aws/ami_related.go:101](/Users/k2m30/projects/a9s/core/aws/ami_related.go:101)  
  Trigger: An ASG references an AMI but has zero instances, or has existing instances from an older AMI after its launch template changed.  
  Impact: The AMI detail shows zero related ASGs even though the group will launch, or is configured to launch, that AMI.  
  Fix: Resolve each ASG’s launch configuration/template AMI (including partial-error handling) and compare that configured AMI to the source AMI.

- **P2** — ASG AMI resolution ignores mixed-instances launch-template overrides.  
  Location: [core/aws/asg_related.go:115](/Users/k2m30/projects/a9s/core/aws/asg_related.go:115)  
  Trigger: A mixed-instances ASG uses an override launch template with a different AMI.  
  Impact: The ASG reports only its base-template AMI; affected override AMIs have no forward ASG relationship.  
  Fix: Resolve the base template plus every override template and return the distinct AMI IDs. [AWS documentation](https://docs.aws.amazon.com/cli/latest/reference/autoscaling/describe-auto-scaling-groups.html)

- **P2** — Systems Manager `resolve:ssm:` image references are emitted as literal AMI IDs.  
  Location: [core/aws/asg_related.go:139](/Users/k2m30/projects/a9s/core/aws/asg_related.go:139), [core/aws/ng_related.go:225](/Users/k2m30/projects/a9s/core/aws/ng_related.go:225), [core/aws/eks_related_extra.go:206](/Users/k2m30/projects/a9s/core/aws/eks_related_extra.go:206), [core/semantics/ctevent/summarize_ec2.go:58](/Users/k2m30/projects/a9s/core/semantics/ctevent/summarize_ec2.go:58)  
  Trigger: A launch template or EC2 CloudTrail request uses an SSM parameter instead of an `ami-` ID.  
  Impact: The AMI relation/link invokes `DescribeImages` with an invalid parameter reference and fails.  
  Fix: Request `ResolveAlias` for launch-template reads and emit only resolved `ami-` IDs; make unresolved SSM references non-navigable. Defensively reject non-AMI IDs in `FetchAMIsByIDs`. [AWS documentation](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ResponseLaunchTemplateData.html)

- **P2** — AMI → KMS relations use raw KMS identifiers while KMS resources are keyed by canonical bare key ID.  
  Location: [core/aws/ami_related_extra.go:58](/Users/k2m30/projects/a9s/core/aws/ami_related_extra.go:58)  
  Trigger: An AMI block-device mapping supplies a key ARN or alias.  
  Impact: The KMS count can be nonzero but its related drill filters on the ARN/alias and renders no matching KMS row.  
  Fix: Canonicalize key ARNs to key IDs and resolve or omit aliases before placing them in `ResourceIDs`. [AWS documentation](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_EbsBlockDeviceResponse.html)

## apigw

1. P1 — [core/aws/apigw_issue_enrichment.go:237](/Users/k2m30/projects/a9s/core/aws/apigw_issue_enrichment.go:237) and [:310](/Users/k2m30/projects/a9s/core/aws/apigw_issue_enrichment.go:310) treat an empty authorizer collection as no authentication.  
   Trigger: REST methods or v2 routes use `AWS_IAM`; or a REST resource policy grants only named principals.  
   Impact: protected APIs are reported as public/no-authorizer, producing false broken-severity security findings. Route/method authorization is independent of the authorizer collection. [REST methods](https://docs.aws.amazon.com/apigateway/latest/api/API_GetResources.html), [v2 routes](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis-apiid-routes-routeid.html), [resource policies](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-resource-policies.html)  
   Fix: inspect paginated REST methods and v2 routes, and evaluate explicit-principal policies before emitting the finding.

2. P1 — [core/aws/apigw_related.go:143](/Users/k2m30/projects/a9s/core/aws/apigw_related.go:143) sends REST API IDs to the v2 `/v2/apis/{apiId}/integrations` endpoint.  
   Trigger: opening Lambda, KMS, load-balancer, or IAM-role relations for a REST API.  
   Impact: those relation rows error instead of resolving.  
   Fix: branch on `protocol == REST` and traverse REST resources/method integrations and REST authorizers; use v2 integrations only for HTTP/WebSocket APIs.

3. P2 — [core/aws/apigw_related.go:143](/Users/k2m30/projects/a9s/core/aws/apigw_related.go:143) reads only the first `GetIntegrations` response page.  
   Trigger: HTTP/WebSocket API with integrations beyond the first page.  
   Impact: Lambda, KMS, load-balancer, and role relations silently omit targets while reported as exact. [GetIntegrations](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis-apiid-integrations.html)  
   Fix: consume `NextToken` through completion, or return a truncated result when capped.

4. P2 — [core/aws/apigw_related.go:218](/Users/k2m30/projects/a9s/core/aws/apigw_related.go:218) and [:233](/Users/k2m30/projects/a9s/core/aws/apigw_related.go:233) do not paginate domain names or API mappings.  
   Trigger: the matching custom domain or API mapping is on a later page.  
   Impact: attached ACM certificates are omitted and the result is incorrectly exact. [Domain names](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/domainnames.html), [API mappings](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/domainnames-domainname-apimappings.html)  
   Fix: paginate both collections.

5. P2 — [core/aws/apigw_related.go:482](/Users/k2m30/projects/a9s/core/aws/apigw_related.go:482) reads only the first page of v2 authorizers.  
   Trigger: an API’s authorizer credential role appears after page one.  
   Impact: the IAM-role relation omits a real role while reported as complete. [GetAuthorizers](https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis-apiid-authorizers.html)  
   Fix: paginate authorizers before returning the resolved relation.

6. P2 — [core/aws/apigw_related.go:276](/Users/k2m30/projects/a9s/core/aws/apigw_related.go:276) and [core/aws/alarm_related_extra.go:31](/Users/k2m30/projects/a9s/core/aws/alarm_related_extra.go:31) use incompatible identifiers for REST API alarms.  
   Trigger: a normal REST API CloudWatch alarm, whose metric dimension is `ApiName`.  
   Impact: API→alarm misses the alarm; alarm→API returns an API name where navigation requires an API ID. [REST metric dimensions](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-metrics-and-dimensions.html)  
   Fix: branch by protocol; match REST alarms by API name and resolve the matching resource ID before navigation.

7. P2 — [core/aws/lambda_related_extra.go:94](/Users/k2m30/projects/a9s/core/aws/lambda_related_extra.go:94) and [:108](/Users/k2m30/projects/a9s/core/aws/lambda_related_extra.go:108) use API names/tags as a proxy for Lambda integrations but return an authoritative result.  
   Trigger: a Lambda is integrated with an API whose name/tags do not contain the function name.  
   Impact: Lambda→API Gateway reports zero even when a real integration exists.  
   Fix: query integrations, or report the result as unknown/truncated rather than resolved.

8. P2 — [core/aws/acm_related.go:133](/Users/k2m30/projects/a9s/core/aws/acm_related.go:133) converts an API Gateway custom-domain ARN into a domain name and returns it as an `apigw` resource ID.  
   Trigger: an ACM certificate attached to an API Gateway custom domain.  
   Impact: the ACM→API Gateway relation contains a noncanonical ID and cannot navigate to the gateway.  
   Fix: resolve the domain through API mappings to API IDs before returning the relation.

9. P2 — [core/aws/r53_related.go:197](/Users/k2m30/projects/a9s/core/aws/r53_related.go:197) interprets the prefix of every `*.execute-api.*` alias target as an API ID.  
   Trigger: the standard Route 53 alias for a Regional custom domain, whose target is `d-…execute-api.<region>.amazonaws.com`.  
   Impact: Route 53→API Gateway silently reports no relation for custom-domain APIs. [AWS custom-domain DNS example](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-regional-api-custom-domain-create.html)  
   Fix: resolve the record/custom-domain name through domain configurations and API mappings instead of treating the target hostname prefix as an API ID.

10. P3 — [core/aws/apigw_issue_enrichment.go:268](/Users/k2m30/projects/a9s/core/aws/apigw_issue_enrichment.go:268) fetches REST stages but never writes `stages_count`.  
    Trigger: any successfully enriched REST API.  
    Impact: the rendered Stages column is blank for REST APIs.  
    Fix: write an exact stage count to `FieldUpdates` after the REST stage fetch.

11. P3 — [core/aws/apigw.go:176](/Users/k2m30/projects/a9s/core/aws/apigw.go:176) stores the REST endpoint type (`regional`, `private`, or `edge`) in `endpoint`, while [core/aws/catalog_dns_cdn.go:206](/Users/k2m30/projects/a9s/core/aws/catalog_dns_cdn.go:206) renders it as “Endpoint.”  
    Trigger: any REST API.  
    Impact: the mixed API list presents a type value where HTTP/WebSocket rows show an invoke URI.  
    Fix: keep a distinct `endpoint_type` field and label it accordingly.

12. P3 — [core/aws/apigw_issue_enrichment.go:148](/Users/k2m30/projects/a9s/core/aws/apigw_issue_enrichment.go:148) labels explicit zero throttling limits as “no throttling configured.”  
    Trigger: a stage explicitly configured with zero rate or burst.  
    Impact: a restrictive/blocking setting is presented as a DoS exposure; API Gateway also applies account-level throttling by default. [AWS throttling behavior](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-throttling.html)  
    Fix: do not interpret zero as absent configuration; model effective account/stage/route throttling separately.

## asg

- **P1** — [core/aws/asg.go:182](/Users/k2m30/projects/a9s/core/aws/asg.go:182), [catalog_compute.go:95](/Users/k2m30/projects/a9s/core/aws/catalog_compute.go:95)  
  **Trigger:** A mixed-instances ASG uses instance weights or `DesiredCapacityType` of `vcpu`/`memory-mib`; e.g. five 2-vCPU instances satisfy a minimum capacity of 10 vCPU.  
  **Impact:** The group is falsely marked underprovisioned because instance count is compared with a capacity-unit minimum, producing erroneous health findings and colors. AWS defines these values in capacity units in these modes. [AWS documentation](https://docs.aws.amazon.com/autoscaling/ec2/userguide/ec2-auto-scaling-mixed-instances-groups-instance-weighting.html)  
  **Fix:** Base health evaluation on capacity units for weighted/non-instance groups, or suppress count-based underprovisioning in those modes.

- **P1** — [core/aws/asg_related.go:186](/Users/k2m30/projects/a9s/core/aws/asg_related.go:186), [core/aws/elb.go:63](/Users/k2m30/projects/a9s/core/aws/elb.go:63)  
  **Trigger:** An ASG is attached to an ALB/NLB target group.  
  **Impact:** The ASG→ELB relationship returns load-balancer ARNs, while catalogued `elb` resources use load-balancer names as IDs. The related-resource count can be nonzero but navigation resolves no ELB rows. Classic ELB names are likewise returned although the catalog fetches only ELBv2 resources.  
  **Fix:** Resolve target-group load-balancer ARNs to catalog resource IDs before returning them; add Classic ELB modeling or omit that relationship until it is representable.

- **P2** — [core/aws/asg_related.go:115](/Users/k2m30/projects/a9s/core/aws/asg_related.go:115), [core/aws/asg_related.go:267](/Users/k2m30/projects/a9s/core/aws/asg_related.go:267), [core/aws/asg_related_extra.go:109](/Users/k2m30/projects/a9s/core/aws/asg_related_extra.go:109)  
  **Trigger:** A mixed-instances ASG has override launch-template specifications pointing to different templates, such as separate x86 and ARM templates.  
  **Impact:** ASG relationships omit AMIs, IAM roles, and security groups used by override templates, causing incomplete dependency and security views. AWS permits overrides to specify distinct launch templates. [AWS documentation](https://docs.aws.amazon.com/autoscaling/ec2/userguide/ec2-auto-scaling-mixed-instances-groups-launch-template-overrides.html)  
  **Fix:** Resolve and union the base template plus every override template specification, including override image IDs where applicable.

- **P2** — [core/aws/ecs_related_extra.go:43](/Users/k2m30/projects/a9s/core/aws/ecs_related_extra.go:43)  
  **Trigger:** An account has multiple ECS clusters and an ASG tagged `AmazonECSManaged`.  
  **Impact:** The ASG is reported as related to every cluster rather than only its actual capacity-provider association, producing false topology and navigation results. The tag indicates capacity-provider management, not a specific cluster; with managed scaling disabled, a capacity provider can be associated with multiple clusters. [AWS documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/cluster-auto-scaling.html)  
  **Fix:** Resolve the cluster’s capacity providers and compare their actual Auto Scaling group ARNs; do not treat the bare tag as a cluster-specific association.

- **P2** — [core/aws/asg_related.go:315](/Users/k2m30/projects/a9s/core/aws/asg_related.go:315)  
  **Trigger:** A launch template references an IAM instance-profile ARN containing a path, such as `instance-profile/application/web`.  
  **Impact:** The full path is passed as `InstanceProfileName`; IAM rejects it because the API accepts only the final profile name. Related IAM roles become unknown or disappear. [AWS API reference](https://docs.aws.amazon.com/IAM/latest/APIReference/API_GetInstanceProfile.html)  
  **Fix:** Parse the ARN and use its final path segment as the instance-profile name before calling IAM.

- **P2** — [core/aws/asg_issue_enrichment.go:46](/Users/k2m30/projects/a9s/core/aws/asg_issue_enrichment.go:46)  
  **Trigger:** The wave-two enrichment context is cancelled while activity lookups are queued for multiple ASGs.  
  **Impact:** The parallel-work error is discarded; ASGs whose activity lookup never began are left looking inspected and healthy rather than incomplete, hiding recent failed scaling activity.  
  **Fix:** Handle the parallel execution error and mark unprocessed groups as uninspected/truncated, propagating the cancellation state.

## athena

- P2 — [core/aws/athena_issue_enrichment.go:84](/Users/k2m30/projects/a9s/core/aws/athena_issue_enrichment.go:84)  
  Trigger: a workgroup uses Athena-managed query results with a configured KMS key.  
  Impact: it is falsely flagged as storing unencrypted query results.  
  Fix: evaluate `ManagedQueryResultsConfiguration` when enabled instead of treating a missing S3 `ResultConfiguration` as unencrypted.

- P2 — [core/aws/athena_issue_enrichment.go:50](/Users/k2m30/projects/a9s/core/aws/athena_issue_enrichment.go:50)  
  Trigger: the enrichment context expires or is cancelled after only some workgroups start.  
  Impact: unscheduled workgroups are treated as clean, hiding findings.  
  Fix: handle `ForEachParallel`’s error and mark unstarted rows uninspected, returning a partial result/error.

- P2 — [core/aws/athena_related.go:82](/Users/k2m30/projects/a9s/core/aws/athena_related.go:82)  
  Trigger: query metrics are enabled without CloudWatch log delivery, or CloudWatch log delivery is configured independently.  
  Impact: the UI invents `/aws/athena/<workgroup>` log-group links or misses the configured group.  
  Fix: derive the relation from `MonitoringConfiguration.CloudWatchLoggingConfiguration.Enabled` and `LogGroup`, not the metrics flag.

- P2 — [core/aws/athena_related.go:63](/Users/k2m30/projects/a9s/core/aws/athena_related.go:63)  
  Trigger: a workgroup uses KMS for managed query results, Spark customer content, or monitoring/log delivery.  
  Impact: the KMS relationship reports zero keys despite configured KMS dependencies.  
  Fix: collect and deduplicate all applicable KMS references from the workgroup configuration.

- P2 — [core/aws/catalog_data.go:105](/Users/k2m30/projects/a9s/core/aws/catalog_data.go:105)  
  Trigger: a workgroup has `BytesScannedCutoffPerQuery` configured.  
  Impact: the “Cost Cap” list column is always blank: the fetcher retains a `WorkGroupSummary`, which has no `Configuration`.  
  Fix: populate a cost-cap field from `GetWorkGroup` or retain a renderable full-workgroup configuration.

- P2 — [core/aws/athena.go:64](/Users/k2m30/projects/a9s/core/aws/athena.go:64)  
  Trigger: `ListWorkGroups` succeeds but a per-workgroup `GetWorkGroup` fails, such as from a missing permission or throttling.  
  Impact: reciprocal S3 relationships can confidently show zero Athena workgroups even though output locations were not read.  
  Fix: preserve these per-item failures as partial/unknown relationship coverage rather than silently storing an indistinguishable empty location.

- P2 — [core/aws/glue_related.go:215](/Users/k2m30/projects/a9s/core/aws/glue_related.go:215)  
  Trigger: a Glue job and Athena workgroup share a name.  
  Impact: the UI presents an invented Glue–Athena relationship; `glue_job` is never populated on Athena resources.  
  Fix: remove the name-equality heuristic or establish the relation only from a fetched, supported linkage.

- P3 — [core/aws/catalog_data.go:124](/Users/k2m30/projects/a9s/core/aws/catalog_data.go:124)  
  Trigger: client-side overrides are allowed but minimum encryption is enabled.  
  Impact: the finding incorrectly says configured encryption is merely advisory, although Athena enforces the minimum encryption level.  
  Fix: account for `EnableMinimumEncryptionConfiguration` in the finding/detail wording.

- P3 — [core/aws/athena_related.go:27](/Users/k2m30/projects/a9s/core/aws/athena_related.go:27)  
  Trigger: opening an Athena workgroup detail view.  
  Impact: S3, KMS, logs, and role checks each issue the same `GetWorkGroup` request, increasing latency and throttle risk.  
  Fix: memoize the configuration per detail operation or fetch it once and share it across related checkers.

## backup

1. **P1 — Backup selections are flattened across assignment boundaries.**  
   Exact: [backup.go:188](/Users/k2m30/projects/a9s/core/aws/backup.go:188), [backup_coverage.go:165](/Users/k2m30/projects/a9s/core/aws/backup_coverage.go:165)  
   Trigger: one plan has multiple selections where one excludes an ARN that another includes—the documented two-selection pattern for refined resource selection.  
   Impact: valid coverage can be reported as absent; related-plan results can be wrong.  
   Fix: retain and evaluate each `BackupSelection` independently; a plan covers a resource when any individual selection covers it. [AWS assignment semantics](https://docs.aws.amazon.com/aws-backup/latest/devguide/assigning-resources-json.html)

2. **P1 — A selector-less backup selection is treated as selecting nothing.**  
   Exact: [backup.go:188](/Users/k2m30/projects/a9s/core/aws/backup.go:188), [backup_coverage.go:168](/Users/k2m30/projects/a9s/core/aws/backup_coverage.go:168)  
   Trigger: a selection omits `Resources`, `ListOfTags`, and `Conditions`.  
   Impact: resources eligible through AWS Backup’s default “all supported and opted-in resources” behavior are marked “not covered.”  
   Fix: model this as an all-eligible selection, or conservatively return unknown until opt-in eligibility is known. [AWS BackupSelection behavior](https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupSelection.html)

3. **P1 — Recent failed jobs are filtered by creation time, not completion time.**  
   Exact: [backup_issue_enrichment.go:59](/Users/k2m30/projects/a9s/core/aws/backup_issue_enrichment.go:59), [backup_issue_enrichment.go:91](/Users/k2m30/projects/a9s/core/aws/backup_issue_enrichment.go:91)  
   Trigger: a backup job starts more than 24 hours ago and fails or becomes partial within the last 24 hours.  
   Impact: the plan misses its failed-job warning.  
   Fix: filter with `ByCompleteAfter` and evaluate `CompletionDate` for terminal failed/partial states. [ListBackupJobs filters](https://docs.aws.amazon.com/aws-backup/latest/APIReference/API_ListBackupJobs.html)

4. **P1 — EC2 and EBS tag pivots ignore exclusions and unrepresentable conditions.**  
   Exact: [ec2_related_extra.go:168](/Users/k2m30/projects/a9s/core/aws/ec2_related_extra.go:168), [ebs_related.go:145](/Users/k2m30/projects/a9s/core/aws/ebs_related.go:145)  
   Trigger: a tagged resource matches a positive tag condition but is excluded by `NotResources`, `StringNotEquals`, or `StringNotLike`.  
   Impact: the related panel says a plan protects a resource that AWS Backup excludes.  
   Fix: use one per-selection matcher that applies all inclusion and exclusion predicates; return a lower-bound/unknown result for unsupported conditions.

5. **P2 — Tag-condition serialization corrupts tag keys containing `=`.**  
   Exact: [backup.go:225](/Users/k2m30/projects/a9s/core/aws/backup.go:225), [related_common.go:177](/Users/k2m30/projects/a9s/core/aws/related_common.go:177)  
   Trigger: a selection uses a tag key such as `team=blue`.  
   Impact: matching fails, producing a false “not covered” finding or zero related plans.  
   Fix: store conditions structurally (or use an escaped encoding), rather than `key=value` CSV. EFS, for example, permits `=` in tag keys. [EFS tag format](https://docs.aws.amazon.com/efs/latest/APIReference/API_Tag.html)

6. **P2 — Several registered backup pivots ignore tag-only selections and return a definitive zero.**  
   Exact: [s3_related.go:357](/Users/k2m30/projects/a9s/core/aws/s3_related.go:357), [ddb_related.go:96](/Users/k2m30/projects/a9s/core/aws/ddb_related.go:96), [efs_related_extra.go:135](/Users/k2m30/projects/a9s/core/aws/efs_related_extra.go:135), [dbi_snap_related.go:154](/Users/k2m30/projects/a9s/core/aws/dbi_snap_related.go:154), [dbc_snap_related.go:159](/Users/k2m30/projects/a9s/core/aws/dbc_snap_related.go:159)  
   Trigger: the only matching plan selection is based on resource tags.  
   Impact: the related panel omits the protecting plan and drill-through.  
   Fix: evaluate available tags, read tags where necessary, or return unknown when tag selection could change the result.

7. **P2 — The EBS-volume backup pivot ignores ARN-based selections.**  
   Exact: [ebs_related.go:145](/Users/k2m30/projects/a9s/core/aws/ebs_related.go:145)  
   Trigger: an EBS volume is selected explicitly or by ARN wildcard, without a matching tag condition.  
   Impact: the panel reports no backup plan despite coverage.  
   Fix: build the volume ARN and evaluate both ARN and tag predicates through the shared per-selection matcher.

8. **P2 — The EBS-snapshot pivot only accepts exact resource ARNs.**  
   Exact: [ebs_snap_related.go:135](/Users/k2m30/projects/a9s/core/aws/ebs_snap_related.go:135)  
   Trigger: an AWS Backup-created snapshot’s source volume is selected by an ARN wildcard or is excluded by `NotResources`.  
   Impact: plan links are missed or incorrectly shown.  
   Fix: use the full selection matcher rather than string equality.

9. **P2 — Selection pagination is omitted in two production paths.**  
   Exact: [backup_related.go:30](/Users/k2m30/projects/a9s/core/aws/backup_related.go:30), [cmd/snapshot/ops.go:807](/Users/k2m30/projects/a9s/cmd/snapshot/ops.go:807)  
   Trigger: a plan has selections beyond the first `ListBackupSelections` page.  
   Impact: IAM-role relationships and captured backup metadata silently omit later selections.  
   Fix: follow `NextToken` through all pages, or explicitly mark output partial. [ListBackupSelections pagination](https://docs.aws.amazon.com/aws-backup/latest/APIReference/API_ListBackupSelections.html)

10. **P2 — Partial-job status labels job counts as skipped-resource counts.**  
    Exact: [backup_issue_enrichment.go:98](/Users/k2m30/projects/a9s/core/aws/backup_issue_enrichment.go:98), [backup_issue_enrichment.go:160](/Users/k2m30/projects/a9s/core/aws/backup_issue_enrichment.go:160), [catalog_backup.go:61](/Users/k2m30/projects/a9s/core/aws/catalog_backup.go:61)  
    Trigger: backup jobs cover multiple child resources, or partial jobs do not map one-to-one to resources.  
    Impact: “N of M resources skipped” is inaccurate because both values are counts of jobs.  
    Fix: either report jobs or derive resource counts from the relevant child-job data.

## cb

1. **P1** — [cb_issue_enrichment.go](/Users/k2m30/projects/a9s/core/aws/cb_issue_enrichment.go:57) sets `SortOrder=DESCENDING`.  
   Trigger: a project has over 100 builds; CodeBuild rejects `ListBuildsForProject` requests with a sort order in that case, although descending is already the default.  
   Impact: latest-build status is unavailable for affected projects.  
   Fix: omit `SortOrder`. [AWS API](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_ListBuildsForProject.html)

2. **P1** — [cb_issue_enrichment.go](/Users/k2m30/projects/a9s/core/aws/cb_issue_enrichment.go:80) discards the partially built enrichment result when `BatchGetBuilds` fails.  
   Trigger: listing succeeds but `BatchGetBuilds` is denied, throttled, or otherwise fails.  
   Impact: projects whose status was not checked are not marked unknown/truncated.  
   Fix: retain initialized result maps, mark every mapped project uninspected, and preserve truncation before returning the error.

3. **P1** — [cb_build_logs.go](/Users/k2m30/projects/a9s/core/aws/cb_build_logs.go:24) ignores `continuationToken` and [always declares the result complete](/Users/k2m30/projects/a9s/core/aws/cb_build_logs.go:87).  
   Trigger: a build log exceeds one `GetLogEvents` page (up to 1 MB or 10,000 events).  
   Impact: most build logs are silently inaccessible beyond the first page.  
   Fix: send and return the appropriate CloudWatch cursor, setting pagination metadata from the response token. [AWS API](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_GetLogEvents.html)

4. **P1** — [cb.go](/Users/k2m30/projects/a9s/core/aws/cb.go:130) omits GitLab and self-managed GitLab source types from contributor-controlled buildspec detection.  
   Trigger: a GitLab-backed CodeBuild project uses its repository buildspec.  
   Impact: the project misses the build-command control finding.  
   Fix: add the SDK’s GitLab source-type constants. [AWS ProjectSource API](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_ProjectSource.html)

5. **P2** — [cb.go](/Users/k2m30/projects/a9s/core/aws/cb.go:152) identifies source buildspecs solely by `.yml`/`.yaml` suffix.  
   Trigger: a Git-backed project uses an S3 buildspec ARN ending in `.yml`, or a repository buildspec with another filename.  
   Impact: false security findings for S3-hosted buildspecs and missed findings for valid alternate repository paths.  
   Fix: distinguish S3 ARNs and inline YAML from relative source paths; do not use file extension as origin evidence. [AWS ProjectSource API](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_ProjectSource.html)

6. **P2** — [cb_issue_enrichment.go](/Users/k2m30/projects/a9s/core/aws/cb_issue_enrichment.go:91) renders `IN_PROGRESS` and `STOPPED` latest builds as `OK`.  
   Trigger: the project’s newest build is still running or was cancelled.  
   Impact: the project list claims success when its latest build did not succeed.  
   Fix: reserve `OK` for `SUCCEEDED`; render the actual non-success status without necessarily raising a broken finding.

7. **P2** — ECR-to-CodeBuild matching loses registry identity in [cb_related.go](/Users/k2m30/projects/a9s/core/aws/cb_related.go:181), while the reverse direction uses a substring match in [ecr_related.go](/Users/k2m30/projects/a9s/core/aws/ecr_related.go:78).  
   Trigger: a cross-account image shares a repository name with a local repository, or a repository name is a prefix of another.  
   Impact: unrelated CodeBuild projects and ECR repositories appear linked.  
   Fix: parse registry and repository components and compare exact repository URIs. [AWS ProjectEnvironment API](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_ProjectEnvironment.html)

8. **P2** — [catalog_cicd.go](/Users/k2m30/projects/a9s/core/aws/catalog_cicd.go:382) permits log navigation when only the log group is present.  
   Trigger: a build has not reached provisioning, so its stream has not been created.  
   Impact: selecting build logs issues an invalid request with an empty stream name.  
   Fix: require both `log_group_name` and `log_stream_name`. [AWS LogsLocation API](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_LogsLocation.html)

9. **P2** — `BuildsNotFound` is ignored by enrichment at [cb_issue_enrichment.go](/Users/k2m30/projects/a9s/core/aws/cb_issue_enrichment.go:82) and treated as “no builds” by snapshot capture at [ops.go](/Users/k2m30/projects/a9s/cmd/snapshot/ops.go:141).  
   Trigger: a build disappears between listing and `BatchGetBuilds`.  
   Impact: the project’s status is silently unverified, and exported output can falsely report no build history.  
   Fix: handle `BuildsNotFound` explicitly: mark the project unavailable for enrichment and emit a distinct snapshot outcome. [AWS API](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_BatchGetBuilds.html)

10. **P3** — [cb_issue_enrichment.go](/Users/k2m30/projects/a9s/core/aws/cb_issue_enrichment.go:106) and [cb_issue_enrichment.go](/Users/k2m30/projects/a9s/core/aws/cb_issue_enrichment.go:124) append the same `Ended` detail twice.  
    Trigger: a failed latest build has an end time.  
    Impact: the failure detail panel duplicates its completion date.  
    Fix: emit one `Ended` row with the intended tier.

11. **P3** — [cb_build_logs.go](/Users/k2m30/projects/a9s/core/aws/cb_build_logs.go:97) claims to format seconds but uses a minute-only layout.  
    Trigger: multiple build log events occur within one minute.  
    Impact: their timestamps are indistinguishable in the log view.  
    Fix: use `2006-01-02 15:04:05`.

## cf

Findings

- P1 — [cf_issue_enrichment.go](/Users/k2m30/projects/a9s/core/aws/cf_issue_enrichment.go:221)  
  Trigger: the default cache behavior is HTTPS-only, but an ordered behavior uses `allow-all`.  
  Impact: the distribution is missing the `cf.insecure-protocol` security finding despite accepting plaintext HTTP on matching paths.  
  Fix: evaluate `CacheBehaviors.Items` as well as `DefaultCacheBehavior`, including each offending path pattern.

- P2 — [cf_issue_enrichment.go](/Users/k2m30/projects/a9s/core/aws/cf_issue_enrichment.go:147) and [ops.go](/Users/k2m30/projects/a9s/cmd/snapshot/ops.go:1239)  
  Trigger: standard logging v2 is configured, but legacy `DistributionConfig.Logging` is absent or disabled.  
  Impact: the UI falsely reports “access logging off,” and snapshots emit `logging_enabled: false`. CloudFront supports v2 independently of legacy logging. [AWS documentation](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/AccessLogs.html)  
  Fix: discover v2 delivery configuration before declaring logging disabled; otherwise label these fields/findings explicitly as legacy logging.

- P2 — [cf_related.go](/Users/k2m30/projects/a9s/core/aws/cf_related.go:347) and [catalog_dns_cdn.go](/Users/k2m30/projects/a9s/core/aws/catalog_dns_cdn.go:121)  
  Trigger: legacy CloudFront access logging is enabled.  
  Impact: the related panel labels an S3 bucket as a CloudWatch Log Group and navigation targets the `logs` resource type with a bucket name, yielding a dead end. `LoggingConfig.Bucket` is an S3 bucket. [AWS API documentation](https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_LoggingConfig.html)  
  Fix: model the legacy destination as S3 (or fold it into the existing S3 relation); separately discover v2 destinations if a log-group relation is required.

- P2 — [waf_related.go](/Users/k2m30/projects/a9s/core/aws/waf_related.go:141)  
  Trigger: a CloudFront-scope WAF Web ACL is associated with more distributions than one `ListDistributionsByWebACLId` response returns.  
  Impact: later distributions are omitted and the panel reports the partial count as complete. The API provides `NextMarker` for this case. [AWS API documentation](https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_ListDistributionsByWebACLId.html)  
  Fix: follow `NextMarker` until complete, or return a truncated result when only one page is intentionally inspected.

- P2 — [cf_related.go](/Users/k2m30/projects/a9s/core/aws/cf_related.go:301)  
  Trigger: a repeated Lambda@Edge function association precedes another distinct association in the same cache-behavior list.  
  Impact: `return` exits collection early, omitting subsequent Lambda functions from the distribution’s related panel.  
  Fix: replace the early `return` with `continue`.

- P3 — [cf_related.go](/Users/k2m30/projects/a9s/core/aws/cf_related.go:22), [cf_related.go](/Users/k2m30/projects/a9s/core/aws/cf_related.go:65), [cf_related.go](/Users/k2m30/projects/a9s/core/aws/cf_related.go:108), [cf_related.go](/Users/k2m30/projects/a9s/core/aws/cf_related.go:137)  
  Trigger: opening a CloudFront row restored from persisted cache before a live refresh supplies `RawStruct`.  
  Impact: S3-origin, ELB-origin, WAF, and ACM relationships all resolve as unknown, despite cached rows being intended to support related navigation.  
  Fix: persist the required source metadata in `Fields` and add checker fallbacks, or rehydrate the selected CloudFront distribution before running these checks.

## cfn

- **P1 — detected secret outputs are still rendered in cleartext.**
  - Location: [cfn.go:78](/Users/k2m30/projects/a9s/core/aws/cfn.go:78), [defaults_cicd.go:13](/Users/k2m30/projects/a9s/core/config/defaults_cicd.go:13), [json.go:95](/Users/k2m30/projects/a9s/internal/tui/views/json.go:95).
  - Trigger: a stack output is detected as a credential.
  - User impact: the supposedly protected value remains in `RawStruct` and is shown in the default Details view and raw JSON/YAML views.
  - Fix: redact detected `Outputs[].OutputValue` values in the display `RawStruct` before any render path can access it.

- **P2 — a failed-event finding suppresses a simultaneous drift finding.**
  - Location: [cfn_issue_enrichment.go:138](/Users/k2m30/projects/a9s/core/aws/cfn_issue_enrichment.go:138).
  - Trigger: a stack is `DRIFTED` and has a failed stack event.
  - User impact: `cfn.stack-drifted` is omitted because the event result overwrites the drift result for that stack ID.
  - Fix: merge finding slices per resource ID, deduplicating by finding code, rather than copying whole map entries.

- **P2 — Wave 2 supporting evidence is discarded by the combined enricher.**
  - Location: [cfn_issue_enrichment.go:159](/Users/k2m30/projects/a9s/core/aws/cfn_issue_enrichment.go:159).
  - Trigger: a stack has a recent failed resource event or drift finding.
  - User impact: users see the high-level finding but not its resource/status-reason or drift evidence in Attention details.
  - Fix: merge and return both sub-enrichers’ `AttentionDetails`, keyed by stack ID and finding code.

- **P2 — the fetched CloudFormation template is absent from the default Details view.**
  - Location: [cfn_detail_enrichment.go:78](/Users/k2m30/projects/a9s/core/aws/cfn_detail_enrichment.go:78), [defaults_cicd.go:8](/Users/k2m30/projects/a9s/core/config/defaults_cicd.go:8).
  - Trigger: opening a CloudFormation stack’s normal Details view.
  - User impact: `GetTemplate` is performed and cached, but `TemplateBody` is not among the configured detail fields, so users cannot see it there.
  - Fix: add `TemplateBody` to the default cfn detail view, or defer template enrichment to JSON/YAML-only navigation.

- **P3 — the declared SNS navigation from `NotificationARNs` is unreachable.**
  - Location: [catalog_cicd.go:103](/Users/k2m30/projects/a9s/core/aws/catalog_cicd.go:103), [defaults_cicd.go:8](/Users/k2m30/projects/a9s/core/config/defaults_cicd.go:8), [generic.go:225](/Users/k2m30/projects/a9s/core/semantics/projection/generic.go:225).
  - Trigger: a stack has notification topic ARNs.
  - User impact: the default Details view omits `NotificationARNs`; if a user adds it, ARN colons are parsed as YAML key/value separators, so no rendered list item matches the registered navigable path.
  - Fix: include `NotificationARNs` in the default view and handle scalar-list navigation without parsing ARN colons as structure.

- **P3 — cfn posture fields are produced but not declared as valid list fields.**
  - Location: [catalog_cicd.go:91](/Users/k2m30/projects/a9s/core/aws/catalog_cicd.go:91), [cfn.go:75](/Users/k2m30/projects/a9s/core/aws/cfn.go:75).
  - Trigger: a user configures a cfn list column keyed `termination_protection` or `output_secret`.
  - User impact: configuration reports the column as unfillable despite the fetcher populating both values.
  - Fix: add both keys to cfn’s `FieldKeys`.

## codeartifact

8 findings:

- **P1** — [core/aws/codeartifact.go:68](/Users/k2m30/projects/a9s/core/aws/codeartifact.go:68)  
  Trigger: two repositories share a name in different CodeArtifact domains. Repository names are unique only within a domain.  
  Impact: both rows share one identity, so enrichment, cache state, navigation, and findings can overwrite or target the wrong repository.  
  Fix: use the repository ARN (or domain-owner/domain/name tuple) as `Resource.ID`; retain the display name separately and update dependent links/relations. [AWS documentation](https://docs.aws.amazon.com/codeartifact/latest/ug/domain-overview.html)

- **P1** — [core/aws/codeartifact_issue_enrichment.go:129](/Users/k2m30/projects/a9s/core/aws/codeartifact_issue_enrichment.go:129)  
  Trigger: a domain policy grants or restricts repository access while the repository has no policy.  
  Impact: public access granted through a domain policy is missed, while a secure repository can be falsely warned as having no permissions policy.  
  Fix: retrieve and evaluate each domain policy alongside repository policy, using CodeArtifact’s combined-policy semantics. [AWS documentation](https://docs.aws.amazon.com/codeartifact/latest/ug/repo-policies.html)

- **P2** — [core/iampolicy/evaluate.go:45](/Users/k2m30/projects/a9s/core/iampolicy/evaluate.go:45)  
  Trigger: a CodeArtifact policy has a wildcard `Allow` but an applicable explicit `Deny` in the repository or domain policy.  
  Impact: the repository is incorrectly marked with `public access policy`.  
  Fix: account for matching explicit denies before emitting the public-access finding.

- **P2** — [core/aws/codeartifact_issue_enrichment.go:92](/Users/k2m30/projects/a9s/core/aws/codeartifact_issue_enrichment.go:92)  
  Trigger: `ListPackages` fails—for example, due to denied access or throttling.  
  Impact: package count failure is silently discarded; the UI can show a blank or stale exact count.  
  Fix: write an explicit unknown count and surface a non-blocking coverage/error indication without suppressing the independent policy result.

- **P2** — [core/aws/catalog_cicd.go:297](/Users/k2m30/projects/a9s/core/aws/catalog_cicd.go:297)  
  Trigger: a CodeArtifact package-manager/API event such as `ReadFromRepository`.  
  Impact: the CloudTrail-events pivot filters by `ResourceName=repositoryName`, but documented CodeArtifact events identify the repository in request parameters rather than a CloudTrail resource entry, so activity is omitted.  
  Fix: add a CodeArtifact-specific local matcher based on `requestParameters.repositoryName` plus domain identity, with an appropriate service-side prefilter. [AWS documentation](https://docs.aws.amazon.com/codeartifact/latest/ug/codeartifact-information-in-cloudtrail.html)

- **P2** — [core/aws/secrets_related_extra.go:36](/Users/k2m30/projects/a9s/core/aws/secrets_related_extra.go:36), [line 58](/Users/k2m30/projects/a9s/core/aws/secrets_related_extra.go:58), [line 62](/Users/k2m30/projects/a9s/core/aws/secrets_related_extra.go:62)  
  Trigger: a secret name or tag merely contains “codeartifact,” including a tag holding a repository ARN.  
  Impact: the relation returns a secret name or raw ARN as a CodeArtifact repository ID, producing false counts and empty/wrong drills; the panel is also labeled “Domains” although it targets repositories.  
  Fix: only accept and parse an explicit repository ARN into the canonical repository ID; otherwise report no proven relation, and rename the panel.

- **P3** — [core/aws/pipeline_related.go:178](/Users/k2m30/projects/a9s/core/aws/pipeline_related.go:178)  
  Trigger: any native CodePipeline.  
  Impact: the registered CodeArtifact relation has no valid native provider to detect; a custom action named `CodeArtifact` can instead create a false relationship from arbitrary configuration.  
  Fix: remove this relation or replace it with a verified EventBridge-rule-based correlation. [AWS provider list](https://docs.aws.amazon.com/codepipeline/latest/userguide/actions-valid-providers.html)

- **P3** — [core/aws/catalog_cicd.go:314](/Users/k2m30/projects/a9s/core/aws/catalog_cicd.go:314)  
  Trigger: a healthy repository with no finding.  
  Impact: its Status column is blank: the catalog declares `state`, but the CodeArtifact fetcher never supplies one and `RepositorySummary` has no state field.  
  Fix: remove the column or populate an explicit healthy status.

## ct-events

## Findings

1. **P1** — [core/aws/alarm_related_extra.go:183](/Users/k2m30/projects/a9s/core/aws/alarm_related_extra.go:183)  
   Trigger: viewing any alarm while the event cache contains an alarm-related CloudWatch event for a different alarm.  
   Impact: every alarm can show and drill into unrelated CloudTrail events.  
   Fix: match the selected alarm’s exact resource name/ARN in the event resources or request parameters; attach a `ResourceName` fetch filter.

2. **P2** — substring matching in [ecs_related_extra.go:115](/Users/k2m30/projects/a9s/core/aws/ecs_related_extra.go:115), [ecs_svc_related_extra.go:42](/Users/k2m30/projects/a9s/core/aws/ecs_svc_related_extra.go:42), [ecs_task_related_extra.go:71](/Users/k2m30/projects/a9s/core/aws/ecs_task_related_extra.go:71), and [ecr_related_extra.go:38](/Users/k2m30/projects/a9s/core/aws/ecr_related_extra.go:38)  
   Trigger: similarly named resources exist, such as `prod` and `prod-blue`.  
   Impact: the related-events count and drill-in include audit events for sibling resources.  
   Fix: match the appropriate CloudTrail resource type and normalized identifier exactly, rather than using `strings.Contains`.

3. **P2** — [core/aws/dbi_related.go:295](/Users/k2m30/projects/a9s/core/aws/dbi_related.go:295) and [core/aws/dbc_related.go:422](/Users/k2m30/projects/a9s/core/aws/dbc_related.go:422) use database IDs, while the catalog routes both resource types by ARN: [catalog_databases.go:70](/Users/k2m30/projects/a9s/core/aws/catalog_databases.go:70), [catalog_databases.go:274](/Users/k2m30/projects/a9s/core/aws/catalog_databases.go:274).  
   Trigger: an RDS instance or cluster event names the resource by ARN.  
   Impact: the right-column pivot misses matching events and its drill-in issues the wrong `ResourceName` filter.  
   Fix: derive the checker’s match and fetch-filter value from `Fields["arn"]`, consistently with the catalog route.

4. **P2** — [core/semantics/ctevent/target.go:135](/Users/k2m30/projects/a9s/core/semantics/ctevent/target.go:135)  
   Trigger: an event’s `resources` entry is `AWS::Lambda::Function` (similarly VPC, security-group, or subnet resource types).  
   Impact: TARGET renders it as a generic, non-navigable resource even though navigation support exists for those labels at [target.go:83](/Users/k2m30/projects/a9s/core/semantics/ctevent/target.go:83).  
   Fix: add the missing CloudTrail resource-type/ARN mappings, including resource-specific EC2 subtype handling.

5. **P2** — [core/semantics/ctevent/target.go:114](/Users/k2m30/projects/a9s/core/semantics/ctevent/target.go:114)  
   Trigger: an S3 object resource ARN such as `arn:aws:s3:::bucket/key`.  
   Impact: the Object TARGET link uses `key` as its S3 target ID, but the S3 list is keyed by bucket, so navigation cannot open the parent bucket.  
   Fix: for Object rows, derive `NavID` as the bucket segment, as the request-parameter path already does.

6. **P3** — [core/aws/ct_events_target_local.go:111](/Users/k2m30/projects/a9s/core/aws/ct_events_target_local.go:111)  
   Trigger: a fallback-target event has multiple top-level `*Id`, `*Name`, or `*Arn` request parameters.  
   Impact: Go map iteration makes the TARGET column choose a different resource unpredictably across fetches.  
   Fix: rank candidate suffixes and break ties by sorted key, matching the deterministic detail-target logic.

## dbc-snap

Findings:

- **P1** — [snapshot_cross_ref.go](/Users/k2m30/projects/a9s/core/aws/snapshot_cross_ref.go:249). Trigger: more than 50 `dbc-snap` resources where automated snapshots precede manual snapshots. The public-share enrichment caps its candidates before restricting them to manual snapshots, although snapshot attributes apply to manual cluster snapshots. Impact: public manual snapshots beyond the cap are reported as not inspected rather than identified as publicly restorable. Fix: filter to manual snapshots before applying the enrichment cap. [AWS RDS API](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBClusterSnapshotAttributes.html)

- **P2** — [dbc_snap_related.go](/Users/k2m30/projects/a9s/core/aws/dbc_snap_related.go:159). Trigger: the parent DB cluster is covered by an AWS Backup selection based on tags/conditions, or the Backup selection listing is partial. The relationship check considers only explicit ARN inclusions/exclusions and returns an authoritative zero. Impact: the snapshot’s “Backup Plans” relationship falsely reports no protection and prevents navigation to the actual plan. Fix: use the tag-aware coverage logic and return an unknown relationship when backup selections are incomplete. [AWS Backup selections support tag and condition rules](https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupSelection.html).

- **P2** — [catalog_databases.go](/Users/k2m30/projects/a9s/core/aws/catalog_databases.go:698). Trigger: opening a console link for any `dbc-snap`, especially a DocumentDB snapshot. The catalog routes every cluster snapshot to the RDS *DB instance snapshot* route; DocumentDB snapshots also require the DocumentDB console. Impact: Console navigation opens the wrong service/page or cannot locate the snapshot. Fix: select the console route from the underlying RDS versus DocumentDB model and use the RDS cluster-snapshot destination for RDS resources. [DocumentDB console guidance](https://docs.aws.amazon.com/documentdb/latest/devguide/backup_restore-share_cluster_snapshots.html)

- **P3** — [catalog_databases.go](/Users/k2m30/projects/a9s/core/aws/catalog_databases.go:775). Trigger: viewing a `dbc-snap` detail and attempting to navigate through `DBClusterIdentifier` to its source DB cluster. The field is rendered but is not registered as navigable. Impact: users cannot directly open the source `dbc` resource from the snapshot detail. Fix: add a `DBClusterIdentifier` → `dbc` navigable-field mapping.

## dbc

- P1 — [core/aws/dbc.go:20](/Users/k2m30/projects/a9s/core/aws/dbc.go:20), [cmd/snapshot/databases.go:198](/Users/k2m30/projects/a9s/cmd/snapshot/databases.go:198), [cmd/snapshot/databases.go:213](/Users/k2m30/projects/a9s/cmd/snapshot/databases.go:213)  
  Trigger: an account has Aurora, Multi-AZ, DocumentDB, or Neptune clusters. Both APIs can return cross-service rows; the DocumentDB API requires the `engine=docdb` filter for a DocumentDB-only result, and RDS can return DocumentDB/Neptune rows.  
  Impact: `dbc` can show unsupported Neptune clusters; RDS clusters can be retained as DocumentDB-shaped rows, breaking engine-specific relations. Snapshot capture additionally mislabels and mixes sources.  
  Fix: filter the DocumentDB request to `engine=docdb`; make capture apply that filter and reject `docdb`/`neptune` rows from its RDS source. [AWS DocumentDB](https://docs.aws.amazon.com/documentdb/latest/APIReference/API_DescribeDBClusters.html), [AWS RDS](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBClusters.html)

- P1 — [core/aws/dbc_issue_enrichment.go:97](/Users/k2m30/projects/a9s/core/aws/dbc_issue_enrichment.go:97)  
  Trigger: an Aurora or Multi-AZ DB cluster has overdue pending maintenance.  
  Impact: no `maintenance overdue` finding is emitted because enrichment queries only the DocumentDB endpoint.  
  Fix: query and merge both DocumentDB and RDS pending-maintenance APIs, matching results by cluster ARN. [DocumentDB API](https://docs.aws.amazon.com/documentdb/latest/APIReference/API_DescribePendingMaintenanceActions.html), [RDS API](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribePendingMaintenanceActions.html)

- P2 — [core/aws/dbc_issue_enrichment.go:49](/Users/k2m30/projects/a9s/core/aws/dbc_issue_enrichment.go:49), [core/aws/dbc_issue_enrichment.go:169](/Users/k2m30/projects/a9s/core/aws/dbc_issue_enrichment.go:169)  
  Trigger: an Aurora or Multi-AZ cluster’s AWS Backup selection depends on a tag.  
  Impact: tag lookup is sent to the DocumentDB API, so coverage for the RDS cluster is treated as unknown and the backup-plan finding is withheld.  
  Fix: dispatch tag reads by cluster raw type: DocumentDB client for DocumentDB, RDS client for Aurora/Multi-AZ. [DocumentDB tags](https://docs.aws.amazon.com/documentdb/latest/APIReference/API_ListTagsForResource.html), [RDS tags](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/USER_Tagging.html)

- P3 — [core/aws/dbc_rds.go:98](/Users/k2m30/projects/a9s/core/aws/dbc_rds.go:98)  
  Trigger: RDS omits the optional `Status` field.  
  Impact: the cluster receives a false transitional finding, rendered as `: in progress`.  
  Fix: mirror the DocumentDB branch: return posture findings without a lifecycle finding when status is empty.

- P3 — [core/aws/catalog_databases.go:769](/Users/k2m30/projects/a9s/core/aws/catalog_databases.go:769)  
  Trigger: viewing relations for an Aurora or Multi-AZ cluster snapshot.  
  Impact: the relation is labeled “DocumentDB Cluster” although it targets the merged `dbc` resource and can point to RDS clusters.  
  Fix: label it “DB Cluster”.

- P3 — [cmd/refgen/main.go:95](/Users/k2m30/projects/a9s/cmd/refgen/main.go:95)  
  Trigger: regenerating the configurable-field reference.  
  Impact: the reference models `dbc` only as `docdbtypes.DBCluster`, omitting RDS-only dbc fields such as `AutoMinorVersionUpgrade` and `IAMDatabaseAuthenticationEnabled`.  
  Fix: generate a merged/union reference for both DocumentDB and RDS cluster shapes.

## dbi-snap

1. **P1 — Snapshot-to-instance links use mutable names instead of immutable resource IDs.**
   - Location: [dbi_snap_issue_enrichment.go](/Users/k2m30/projects/a9s/core/aws/dbi_snap_issue_enrichment.go:44), [dbi_snap_related.go](/Users/k2m30/projects/a9s/core/aws/dbi_snap_related.go:23), [dbi_related.go](/Users/k2m30/projects/a9s/core/aws/dbi_related.go:101), [catalog_databases.go](/Users/k2m30/projects/a9s/core/aws/catalog_databases.go:676)
   - Trigger: An RDS instance is renamed or replaced under a previous `DBInstanceIdentifier`.
   - User impact: Snapshots can be linked to the replacement DB instead of their actual source; orphan, retention, backup-plan, and navigation results become wrong.
   - Fix: Match `DBSnapshot.DbiResourceId` with `DBInstance.DbiResourceId`, falling back conservatively only when unavailable. Both fields represent the immutable source identity. [DBSnapshot API](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html), [DBInstance API](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html)

2. **P2 — KMS related navigation fails for documented alias-form KMS identifiers.**
   - Location: [dbi_snap_related.go](/Users/k2m30/projects/a9s/core/aws/dbi_snap_related.go:56), [related.go](/Users/k2m30/projects/a9s/core/resource/related.go:56)
   - Trigger: `DBSnapshot.KmsKeyId` is an alias name or alias ARN, such as `alias/backup` or `arn:...:alias/backup`.
   - User impact: An encrypted snapshot incorrectly shows no related KMS key, and field navigation targets the truncated alias suffix rather than the key.
   - Fix: Preserve alias identifiers and resolve them through the KMS fetch-by-ID path or alias mapping; do not strip every final slash segment. AWS explicitly permits key IDs, ARNs, alias names, and alias ARNs. [DBSnapshot API](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html)

3. **P2 — The AWS Backup relation omits tag-selected backup plans.**
   - Location: [dbi_snap_related.go](/Users/k2m30/projects/a9s/core/aws/dbi_snap_related.go:154)
   - Trigger: The source DB instance is selected by an AWS Backup plan solely through tags or conditions.
   - User impact: The snapshot reports zero related backup plans and cannot drill into the plan that actually protects its source.
   - Fix: Read the parent DB’s tags when needed and evaluate `selection_tags` with the existing tag-aware backup coverage logic, preserving unknown state if tags cannot be read. AWS Backup supports tag- and condition-based selection. [AWS Backup selection API](https://docs.aws.amazon.com/aws-backup/latest/APIReference/API_BackupSelection.html)

4. **P2 — Partial Backup selection enumeration is reported as an exact relation count.**
   - Location: [dbi_snap_related.go](/Users/k2m30/projects/a9s/core/aws/dbi_snap_related.go:144)
   - Trigger: Any loaded backup plan has `selections_partial` because its selection list or selection details could not be fully read.
   - User impact: A snapshot can show an authoritative zero or incomplete count even though an unread selection may cover its source DB.
   - Fix: Treat partial plan selections as unknown or a truncated lower bound, rather than returning an exact count.

5. **P2 — Public-snapshot enrichment spends its bounded inspection budget on non-manual snapshots.**
   - Location: [snapshot_cross_ref.go](/Users/k2m30/projects/a9s/core/aws/snapshot_cross_ref.go:249)
   - Trigger: More than 50 retained snapshot rows include automated snapshots before a manual public snapshot.
   - User impact: Calls that only apply to manual snapshots consume the enrichment cap, leaving later manual snapshots—including public ones—uninspected.
   - Fix: Filter to manual snapshots before applying the enrichment cap and calling `DescribeDBSnapshotAttributes`. That API reports attributes for manual DB snapshots. [DescribeDBSnapshotAttributes API](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBSnapshotAttributes.html)

## dbi

Findings:

- **P1** — [`core/aws/dbi_issue_enrichment.go:50`](/Users/k2m30/projects/a9s/core/aws/dbi_issue_enrichment.go:50)  
  Trigger: An Aurora instance belongs to a cluster selected by an AWS Backup plan, or an RDS Custom for Oracle instance is scanned. The code checks the instance `db:` ARN as though every `dbi` were independently backup-eligible. AWS Backup protects Aurora at the cluster ARN and does not support RDS Custom for Oracle. [AWS Backup documentation](https://docs.aws.amazon.com/aws-backup/latest/devguide/assigning-resources-json.html)  
  Impact: False “not covered by a backup plan” findings, inflated issue counts, and impossible remediation advice.  
  Fix: Skip instance-level backup coverage for Aurora members and unsupported RDS Custom resources; retain cluster coverage on `dbc`.

- **P1** — [`core/aws/rds.go:157`](/Users/k2m30/projects/a9s/core/aws/rds.go:157), [`core/aws/rds.go:178`](/Users/k2m30/projects/a9s/core/aws/rds.go:178)  
  Trigger: RDS reports `upgrade-failed`, `incompatible-create`, `insufficient-capacity`, or `inaccessible-encryption-credentials-recoverable`; or reports `storage-config-upgrade`, `storage-initialization`, or `delete-precheck`.  
  Impact: Known service states are rendered as raw text without the appropriate finding, color, issue count, or attention details; `delete-precheck` can also retain irrelevant posture findings. AWS documents these as current DB-instance statuses. [AWS RDS status reference](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/accessing-monitoring.html)  
  Fix: Classify the failure states as Broken, the storage states as transitional warnings, and treat `delete-precheck` as teardown.

- **P2** — [`core/aws/ct_events_related.go:441`](/Users/k2m30/projects/a9s/core/aws/ct_events_related.go:441)  
  Trigger: A CloudTrail `LookupEvents` RDS resource is typed `DBInstance`, with no `dBInstanceIdentifier` in request parameters.  
  Impact: Navigating from that CloudTrail event to its DB instance incorrectly shows no relation. CloudTrail documents `DBInstance` as the RDS resource type. [CloudTrail Resource API](https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_Resource.html)  
  Fix: Accept and normalize `DBInstance` as well as any embedded CloudTrail-form resource type.

- **P2** — [`core/aws/dbi_related.go:269`](/Users/k2m30/projects/a9s/core/aws/dbi_related.go:269)  
  Trigger: Two RDS instances share a VPC security group.  
  Impact: Each instance’s Network Interfaces relation includes the other’s RDS-managed ENIs, leading to incorrect counts and navigation. The filters identify RDS ENIs plus shared groups, not a specific DB instance.  
  Fix: Remove/defer this pivot unless an authoritative per-instance ENI mapping is available.

- **P2** — [`core/aws/rds_events.go:69`](/Users/k2m30/projects/a9s/core/aws/rds_events.go:69), [`core/aws/rds_events.go:95`](/Users/k2m30/projects/a9s/core/aws/rds_events.go:95)  
  Trigger: One DB instance emits two RDS events within the same minute.  
  Impact: Both receive the same resource ID; later pagination can discard one through ID de-duplication, and row state can target the wrong event.  
  Fix: Keep the minute-granularity timestamp for display, but construct the ID from full timestamp precision plus stable event content/source data.

## ddb

- **P1** — [core/aws/ddb.go:28](/Users/k2m30/projects/a9s/core/aws/ddb.go:28)  
  Trigger: a global-table replica enters `REPLICATION_NOT_AUTHORIZED`.  
  Impact: replication stoppage can render as healthy (or only show deletion-protection posture); AWS warns this can become irreversible. [AWS docs](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/globaltables-security.html)  
  Fix: classify this status as a broken finding and register its catalog definition.

- **P2** — [core/aws/ddb.go:113](/Users/k2m30/projects/a9s/core/aws/ddb.go:113)  
  Trigger: a provisioned table has no `BillingModeSummary`—which DynamoDB permits for tables never switched to on-demand.  
  Impact: the Billing column is blank instead of showing provisioned capacity. [AWS docs](https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_BillingModeSummary.html)  
  Fix: infer/render `Provisioned` when the summary is absent, with an explicit unknown fallback if needed.

- **P2** — [core/aws/ddb_related_extra.go:56](/Users/k2m30/projects/a9s/core/aws/ddb_related_extra.go:56)  
  Trigger: the VPC has an `Interface` endpoint for `com.amazonaws.<region>.dynamodb`.  
  Impact: the DynamoDB table omits a real private-connectivity endpoint from Related Resources. DynamoDB supports both gateway and interface endpoints. [AWS docs](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/privatelink-interface-endpoints.html)  
  Fix: accept both `Gateway` and `Interface` endpoint types.

- **P2** — [core/aws/ddb_related_extra.go:30](/Users/k2m30/projects/a9s/core/aws/ddb_related_extra.go:30)  
  Trigger: a log group happens to use `/aws/dynamodb/tables/<table>/...`, or Contributor Insights is enabled.  
  Impact: manually named, unrelated log groups are shown as DynamoDB-related, while actual DynamoDB Contributor Insights creates CloudWatch insight rules—not log groups. [AWS docs](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/contributorinsights_HowItWorks.html)  
  Fix: remove this log-group relation, or model the actual Insight Rule relationship.

- **P2** — [core/aws/ddb_related.go:60](/Users/k2m30/projects/a9s/core/aws/ddb_related.go:60)  
  Trigger: a non-DynamoDB/custom metric alarm has a `TableName` dimension matching the table; conversely, a DynamoDB metric-math alarm stores its dimensions under metric queries.  
  Impact: Related Resources can show unrelated alarms and omit valid DynamoDB alarms.  
  Fix: require the `AWS/DynamoDB` namespace and inspect both direct metrics and metric-data-query metrics.

- **P2** — [core/aws/lambda_related_extra.go:162](/Users/k2m30/projects/a9s/core/aws/lambda_related_extra.go:162)  
  Trigger: `ListEventSourceMappings` filtered by a Lambda function returns `NextMarker`.  
  Impact: the Lambda-to-DynamoDB relationship silently omits DynamoDB streams on later pages and reports a confident incomplete count. [AWS docs](https://docs.aws.amazon.com/lambda/latest/api/API_ListEventSourceMappings.html)  
  Fix: follow `NextMarker` until exhaustion, or surface truncation when applying a bounded walk.

- **P2** — [core/iampolicy/evaluate.go:121](/Users/k2m30/projects/a9s/core/iampolicy/evaluate.go:121)  
  Trigger: a DynamoDB resource policy uses `Principal: "*"` with `Bool: {"aws:PrincipalIsAWSService": "true"}`.  
  Impact: the ddb enricher reports a broken “resource policy open to anyone” finding even though the policy is restricted to AWS services. [AWS docs](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/rbac-bpa-rbp.html)  
  Fix: recognize `aws:PrincipalIsAWSService=true` with the `Bool` operator as a restrictive condition.

- **P2** — [core/iampolicy/evaluate.go:46](/Users/k2m30/projects/a9s/core/iampolicy/evaluate.go:46)  
  Trigger: a table resource policy has a broad Allow but an applicable explicit Deny covering the exposed DynamoDB actions.  
  Impact: ddb can falsely report public or cross-account access despite IAM’s explicit-deny precedence. [AWS docs](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_evaluation-logic_policy-eval-denyallow.html)  
  Fix: evaluate applicable Deny statements against principal, action, resource, and condition before emitting exposure findings.

- **P3** — [core/aws/ddb_issue_enrichment.go:131](/Users/k2m30/projects/a9s/core/aws/ddb_issue_enrichment.go:131)  
  Trigger: a scan immediately follows creating a table with a resource policy, or attaching a policy.  
  Impact: DynamoDB may transiently return `PolicyNotFoundException`; the code treats it as definitively absent and misses the policy finding. [AWS docs](https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_GetResourcePolicy.html)  
  Fix: use a short bounded retry for documented eventual-consistency responses before classifying the policy as absent.

## eb-rule

- **P1** — [core/aws/eb_rule_targets.go:33](/Users/k2m30/projects/a9s/core/aws/eb_rule_targets.go:33)  
  **Trigger:** A default-bus rule has no `EventBusName`; the fetcher sends an explicit empty `EventBusName` rather than omitting it. AWS treats omission as default, but a supplied value must be non-empty. [AWS API](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_ListTargetsByRule.html)  
  **Impact:** Opening that rule’s targets fails; Lambda→EventBridge reverse lookup also fails for those rules.  
  **Fix:** Set `EventBusName` only when non-empty (or normalize it to `default`).

- **P1** — [core/aws/eb_rule.go:19](/Users/k2m30/projects/a9s/core/aws/eb_rule.go:19), [cmd/snapshot/messaging.go:243](/Users/k2m30/projects/a9s/cmd/snapshot/messaging.go:243)  
  **Trigger:** Rules exist on a custom event bus. Both paths call `ListRules` without `EventBusName`, which lists only the default bus.  
  **Impact:** Custom-bus rules are wholly absent from the main view and snapshots; their findings, targets, and cache-backed relationships are unavailable.  
  **Fix:** Page `ListEventBuses`, then page `ListRules` per bus; preserve bus identity with the rule to avoid same-name collisions. [ListRules](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_ListRules.html), [ListEventBuses](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_ListEventBuses.html)

- **P2** — [core/aws/pipeline_related.go:348](/Users/k2m30/projects/a9s/core/aws/pipeline_related.go:348), [core/aws/sfn_related.go:237](/Users/k2m30/projects/a9s/core/aws/sfn_related.go:237), [core/aws/sqs_related.go:267](/Users/k2m30/projects/a9s/core/aws/sqs_related.go:267)  
  **Trigger:** A Pipeline, Step Function, or queue is targeted by rules on a custom bus, or by more than 100 rules on the default bus. These calls omit `EventBusName` and discard `NextToken`.  
  **Impact:** Related-resource counts and navigation silently omit matching rules while presenting the result as complete.  
  **Fix:** Enumerate buses, page `ListRuleNamesByTarget` for each, and report incomplete walks as truncated. [AWS API](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_ListRuleNamesByTarget.html)

- **P2** — [core/aws/ecr_related.go:237](/Users/k2m30/projects/a9s/core/aws/ecr_related.go:237)  
  **Trigger:** An ECR rule uses a valid EventBridge comparator, e.g. `{"repository-name":[{"prefix":"prod-"}]}`, and the repository is `prod-api`. The code only unmarshals literal string arrays and returns no relation.  
  **Impact:** The ECR related panel misses rules that actually match the repository’s events.  
  **Fix:** Parse and evaluate supported EventBridge comparison operators, or return unknown rather than a proven zero for unsupported predicates. [Comparison operators](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-create-pattern-operators.html), [ECR event shape](https://docs.aws.amazon.com/AmazonECR/latest/userguide/ecr-eventbridge.html)

- **P2** — [core/aws/s3_related.go:388](/Users/k2m30/projects/a9s/core/aws/s3_related.go:388)  
  **Trigger:** A rule filters `aws.s3` events on `detail.object.key: ["bucket-a"]`, while an unrelated bucket is named `bucket-a`. Raw substring matching reports the rule as filtering that bucket.  
  **Impact:** The S3 related panel offers incorrect EventBridge-rule links.  
  **Fix:** Parse the pattern and inspect only `source` plus the exact `detail.bucket.name` predicate. [AWS S3 pattern example](https://docs.aws.amazon.com/eventbridge/latest/userguide/event-bus-rule-get-started.html)

- **P2** — [core/aws/ecs_svc_related_extra.go:197](/Users/k2m30/projects/a9s/core/aws/ecs_svc_related_extra.go:197), [core/aws/ecs_svc_related_extra.go:215](/Users/k2m30/projects/a9s/core/aws/ecs_svc_related_extra.go:215)  
  **Trigger:** An ECS service-action rule filters `detail.serviceName` or `detail.service`; neither is examined, so it is treated as related to every ECS service. Separately, a `clusterArn` for `prod-blue` matches services in `prod` through substring comparison.  
  **Impact:** ECS service related panels show unrelated EventBridge rules.  
  **Fix:** Recognize `serviceName` and `service`, and compare canonical cluster ARNs/names exactly. [AWS ECS event fields](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_cwe_events.html)

## eb

Findings:

- P2 — [eb_issue_enrichment.go:52](/Users/k2m30/projects/a9s/core/aws/eb_issue_enrichment.go:52), [89](/Users/k2m30/projects/a9s/core/aws/eb_issue_enrichment.go:89)  
  Trigger: the first 50 listed environments are `Terminating`/`Terminated`, followed by active environments.  
  Impact: the health pass consumes the cap before configuration posture filters lifecycle-ended rows, so no active environment receives managed-updates, enhanced-health, or log-streaming checks.  
  Fix: retain the original resource slice for `ebConfigurationPosture`, allowing it to filter first and then apply its own cap.

- P2 — [eb_related_extra.go:348](/Users/k2m30/projects/a9s/core/aws/eb_related_extra.go:348)  
  Trigger: an application retains versions whose source bundles are in different S3 buckets, while an environment runs only one version.  
  Impact: the environment’s S3 relationship lists buckets for unrelated application versions, producing incorrect counts and navigation. AWS defines `VersionLabel` as the version deployed in the environment, and `DescribeApplicationVersions` supports filtering by it. [AWS documentation](https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html) [API reference](https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_DescribeApplicationVersions.html)  
  Fix: pass the environment’s `VersionLabel` through `VersionLabels` and relate only that version’s source bundle.

- P2 — [eb_related_extra.go:312](/Users/k2m30/projects/a9s/core/aws/eb_related_extra.go:312)  
  Trigger: an environment has an `OperationsRole` configured.  
  Impact: the IAM-role relationship omits the operations role, so users cannot discover or navigate to a role the environment uses. `OperationsRole` is an environment role ARN. [AWS documentation](https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html)  
  Fix: add `EnvironmentDescription.OperationsRole` to the collected role references; persist it in fields if cache-backed resolution is required.

- P2 — [secrets_related_extra.go:94](/Users/k2m30/projects/a9s/core/aws/secrets_related_extra.go:94)  
  Trigger: checking a secret’s Elastic Beanstalk relationship while EB rows are restored from the disk cache.  
  Impact: every EB row is skipped because it has no `RawStruct`, yet the function can return a definitive zero, hiding real secret references.  
  Fix: use persisted `application_name` and `environment_name` fields for cache-backed rows, or return unknown/refetch when the EB cache is fields-only.

- P2 — [eb.go:17](/Users/k2m30/projects/a9s/core/aws/eb.go:17)  
  Trigger: an environment has completed termination.  
  Impact: terminated environments are excluded because `IncludeDeleted` is unset, despite the resource defining terminated-state rendering and findings. AWS specifies that `IncludeDeleted=false` excludes deleted environments. [AWS documentation](https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_DescribeEnvironments.html)  
  Fix: enable `IncludeDeleted` and choose an explicit `IncludedDeletedBackTo` retention window.

## ebs-snap

1. **P1 — Recycle Bin snapshots are reported as failed.**

   - Exact: [core/aws/ebs.go](/Users/k2m30/projects/a9s/core/aws/ebs.go:294), [core/aws/catalog_compute.go](/Users/k2m30/projects/a9s/core/aws/catalog_compute.go:811)
   - Trigger: An EBS snapshot is `recoverable` or `recovering`.
   - Impact: The UI marks a recoverable snapshot broken and advises deletion, though AWS defines these as Recycle Bin recovery states, not creation failures.
   - Fix: Handle `recoverable` and `recovering` separately from `error`, with recovery-specific status/remediation. [AWS snapshot states](https://docs.aws.amazon.com/ebs/latest/userguide/ebs-describing-snapshots.html)

2. **P1 — Loading later pages bypasses ebs-snap Wave 2 findings.**

   - Exact: [core/runtime/handlers_resources.go](/Users/k2m30/projects/a9s/core/runtime/handlers_resources.go:163)
   - Trigger: Open `ebs-snap`, allow the first page to enrich, then load another page.
   - Impact: Newly appended snapshots never receive orphan or public-share findings; a public snapshot on a later page can appear healthy.
   - Fix: Enrich appended `ebs-snap` rows and merge their results, rather than suppressing enrichment solely because the result is an append.

3. **P2 — Free-text descriptions are treated as proof of automated retention.**

   - Exact: [core/aws/ebs.go](/Users/k2m30/projects/a9s/core/aws/ebs.go:329), [core/aws/ebs.go](/Users/k2m30/projects/a9s/core/aws/ebs.go:330)
   - Trigger: A snapshot older than 365 days has a description containing `automated`, or is created by a normal `CreateImage` request.
   - Impact: Manually created recovery or AMI-backing snapshots receive a false “automated” cost warning and deletion advice.
   - Fix: Do not infer automation or retention from description text; emit the finding only from authoritative lifecycle metadata, or omit it when unavailable. [CreateImage creates backing snapshots](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateImage.html), [snapshot descriptions are caller-supplied](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateSnapshot.html)

4. **P2 — Public-share enrichment has no requested page bound.**

   - Exact: [core/aws/ebs_snap_issue_enrichment.go](/Users/k2m30/projects/a9s/core/aws/ebs_snap_issue_enrichment.go:78)
   - Trigger: An account has many public self-owned snapshots.
   - Impact: The first public-share query can return an unbounded response before the walker’s page cap applies, causing latency, throttling, or enrichment timeout.
   - Fix: Set `MaxResults` to `DefaultPageSize` on this `DescribeSnapshots` request. [AWS recommends paginated DescribeSnapshots requests](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSnapshots.html)

## ebs

- **P1 — [core/aws/ebs.go:294](/Users/k2m30/projects/a9s/core/aws/ebs.go:294)**  
  Trigger: a snapshot is `recoverable` (Recycle Bin) or `recovering`. AWS defines these as recoverable/transitional states, not creation failures. [AWS documentation](https://docs.aws.amazon.com/en_en/ebs/latest/userguide/ebs-describing-snapshots.html)  
  User impact: it is shown as a broken failed snapshot with advice to replace and delete it.  
  Fix: handle these states separately with accurate Recycle Bin/recovery guidance; reserve the error finding for `error`.

- **P2 — [core/aws/ebs_issue_enrichment.go:82](/Users/k2m30/projects/a9s/core/aws/ebs_issue_enrichment.go:82)**  
  Trigger: `DescribeVolumeStatus` returns `insufficient-data` for a newly attached or still-checking volume.  
  User impact: a normal transient state becomes a broken “volume I/O degraded” finding, warning of possible corruption. AWS says `insufficient-data` means checks may still be running. [AWS documentation](https://docs.aws.amazon.com/cli/latest/reference/ec2/describe-volume-status.html)  
  Fix: emit the degraded finding only for impaired status; model warning/insufficient-data distinctly or retry/report them as pending.

- **P2 — [core/aws/ebs_snap_issue_enrichment.go:127](/Users/k2m30/projects/a9s/core/aws/ebs_snap_issue_enrichment.go:127)**  
  Trigger: a `CopySnapshot` result has an arbitrary source volume ID such as `vol-ffff`, rather than exactly `vol-ffffffff`.  
  User impact: the copy can be falsely reported as an orphan and linked to a nonexistent EBS volume. AWS explicitly says copied snapshots’ volume IDs are arbitrary and must not be used. [AWS documentation](https://docs.aws.amazon.com/ebs/latest/userguide/ebs-copy-snapshot.html)  
  Fix: identify copies via the SDK’s copy-specific metadata (for example, `TransferType`), persist that fact for cached rows, and suppress source-volume relations/orphan checks for copies.

- **P2 — [core/aws/ebs.go:76](/Users/k2m30/projects/a9s/core/aws/ebs.go:76)**  
  Trigger: a Multi-Attach EBS volume is attached to multiple instances.  
  User impact: the list and EBS→EC2 related panel retain only the first attachment, concealing the other consuming instances. Multi-Attach supports up to 16 attachments. [AWS documentation](https://docs.aws.amazon.com/ebs/latest/userguide/ebs-volumes-multi.html)  
  Fix: preserve all attachment instance IDs for display and relationship resolution.

- **P2 — [core/aws/ebs.go:329](/Users/k2m30/projects/a9s/core/aws/ebs.go:329)**  
  Trigger: an old manual snapshot has “automated” in its user-supplied description, or is an AMI-backing snapshot with AWS’s `Created by CreateImage(...)` description.  
  User impact: it is falsely labeled as an aged automated snapshot and advised for deletion, despite potentially backing a registered AMI. [AWS example](https://docs.aws.amazon.com/ec2/latest/devguide/example_ec2_DescribeSnapshots_section.html)  
  Fix: do not infer lifecycle automation or retention from description text; require authoritative lifecycle provenance and exclude active AMI backing snapshots.

- **P3 — [core/aws/backup_coverage.go:48](/Users/k2m30/projects/a9s/core/aws/backup_coverage.go:48)**  
  Trigger: caller identity cannot be resolved, but the complete Backup plan list is empty.  
  User impact: EBS volumes receive no “not covered by a backup plan” finding, even though an empty plan list conclusively proves no plan covers them.  
  Fix: when the Backup list is complete and empty, emit the uncovered finding without requiring a constructible volume ARN.

## ec2

Findings:

- P1 — IPv6 exposure is not modeled. [`core/aws/ec2.go:108`](/Users/k2m30/projects/a9s/core/aws/ec2.go:108), [`core/aws/ec2_issue_enrichment.go:222`](/Users/k2m30/projects/a9s/core/aws/ec2_issue_enrichment.go:222), [`core/aws/sg.go:94`](/Users/k2m30/projects/a9s/core/aws/sg.go:94). Trigger: an instance has public IPv6 and `::/0` ingress but no public IPv4. Impact: it can be internet-reachable without the public-address or internet-exposure findings; IPv6-only rules can also be attributed to IPv4-only instances. Fix: model reachable IPv6 addresses and preserve address-family-specific SG risk. [AWS IPv6 guidance](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-ip-addressing.html)

- P1 — A truncated SG cache can produce a clean EC2 exposure result. [`core/aws/ec2_issue_enrichment.go:208`](/Users/k2m30/projects/a9s/core/aws/ec2_issue_enrichment.go:208), [`core/aws/ec2_issue_enrichment.go:239`](/Users/k2m30/projects/a9s/core/aws/ec2_issue_enrichment.go:239). Trigger: an instance’s SG is on an unloaded SG page. Impact: public instances exposed through that SG receive neither a finding nor an uninspected marker. Fix: when an attached SG is absent from a partial cache, mark exposure uninspected unless a known SG already proves exposure.

- P2 — EC2 internet-exposure findings ignore subnet routing and IGW availability. [`core/aws/ec2_issue_enrichment.go:222`](/Users/k2m30/projects/a9s/core/aws/ec2_issue_enrichment.go:222). Trigger: an instance has a public IPv4 and open SG port, but its subnet has no route to an internet gateway. Impact: the UI states the port is internet-reachable when inbound internet traffic cannot reach it. Fix: require an applicable IGW route before emitting the reachability finding; retain the separate public-address signal. [AWS internet-gateway requirements](https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Internet_Gateway.html)

- P2 — Multi-Attach EBS volumes expose only their first EC2 attachment. [`core/aws/ebs.go:75`](/Users/k2m30/projects/a9s/core/aws/ebs.go:75), [`core/aws/ebs_related.go:16`](/Users/k2m30/projects/a9s/core/aws/ebs_related.go:16). Trigger: an `io1`/`io2` Multi-Attach volume is attached to multiple instances. Impact: the EBS-to-EC2 relation and drill omit every attachment after the first. Fix: retain all attachment instance IDs and build the relation from `Volume.Attachments`. [AWS Multi-Attach documentation](https://docs.aws.amazon.com/ebs/latest/userguide/ebs-volumes-multi.html)

- P2 — By-ID EC2 retrieval sends an unbounded ID list in one request. [`core/aws/ec2_by_ids.go:96`](/Users/k2m30/projects/a9s/core/aws/ec2_by_ids.go:96), reached from [`core/runtime/executor.go:980`](/Users/k2m30/projects/a9s/core/runtime/executor.go:980). Trigger: a related-resource result, such as a large EKS cluster, yields over 1,000 missing EC2 IDs. Impact: lazy EC2 materialization fails instead of loading the related instances. Fix: split requests into batches of at most 1,000 IDs and aggregate results/errors. [AWS pagination guidance](https://docs.aws.amazon.com/ec2/latest/devguide/ec2-api-pagination.html)

- P3 — `insufficient-data` status checks are rendered as neutral. [`core/aws/catalog_compute.go:270`](/Users/k2m30/projects/a9s/core/aws/catalog_compute.go:270), [`core/aws/catalog_compute.go:214`](/Users/k2m30/projects/a9s/core/aws/catalog_compute.go:214). Trigger: an instance or system check returns `insufficient-data`. Impact: its list-state and detail status-check value lack the warning indicator despite being classified as a warning. Fix: map `insufficient-data` to the warning decorator and detail tier.

## ecr

- P1 — ECR scan findings are invisible with new Basic Scanning. Exact locations: [ecr_images.go:129](/Users/k2m30/projects/a9s/core/aws/ecr_images.go:129), [ecr_images.go:178](/Users/k2m30/projects/a9s/core/aws/ecr_images.go:178), [ecr_issue_enrichment.go:139](/Users/k2m30/projects/a9s/core/aws/ecr_issue_enrichment.go:139), [compute.go:463](/Users/k2m30/projects/a9s/cmd/snapshot/compute.go:463). Trigger: a registry uses the new ECR Basic Scanning, which does not populate `DescribeImages` scan fields. User impact: critical/high findings, scan state, and scanned-image counts render as clean/empty. Fix: obtain findings with `DescribeImageScanFindings` and normalize Basic and enhanced results. [AWS documentation](https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_DescribeImages.html)

- P2 — Registry-level scanning produces false “scan on push off” findings. Exact locations: [ecr.go:114](/Users/k2m30/projects/a9s/core/aws/ecr.go:114), [compute.go:417](/Users/k2m30/projects/a9s/cmd/snapshot/compute.go:417). Trigger: registry-level enhanced scanning continuously scans a repository while its deprecated per-repository setting is absent or false. User impact: scanned repositories are incorrectly flagged and reported as unscanned. Fix: read and evaluate the registry scanning configuration and its matching rules before setting this field/finding. [AWS documentation](https://docs.aws.amazon.com/AmazonECR/latest/userguide/image-scanning-enhanced-enabling.html)

- P2 — Exclusion-based mutable tag modes evade the mutable-tag finding. Exact location: [ecr.go:121](/Users/k2m30/projects/a9s/core/aws/ecr.go:121). Trigger: a repository uses `MUTABLE_WITH_EXCLUSION`, or `IMMUTABLE_WITH_EXCLUSION` with a mutable exception such as `latest`. User impact: movable deployed tags receive no warning. Fix: flag every mode that permits any tag overwrite and expose its exclusion filters. [AWS documentation](https://docs.aws.amazon.com/AmazonECR/latest/userguide/image-tag-mutability.html)

- P2 — ECR-to-EventBridge relationships are matched incorrectly. Exact locations: [ecr_related.go:273](/Users/k2m30/projects/a9s/core/aws/ecr_related.go:273), [ecr_related.go:286](/Users/k2m30/projects/a9s/core/aws/ecr_related.go:286). Trigger: a valid rule uses an operator such as `prefix` for `repository-name`, or a literal resource ARN for `repo-prod` when examining `repo`. User impact: matching rules are omitted, while prefix-sharing repositories can be incorrectly shown as related. Fix: evaluate supported EventBridge pattern operators; use exact equality for literal ARN values rather than substring matching. [AWS documentation](https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-create-pattern-operators.html)

## ecs-svc

- P2 — [core/aws/ecs_svc_logs.go:61](/Users/k2m30/projects/a9s/core/aws/ecs_svc_logs.go:61), [line 78](/Users/k2m30/projects/a9s/core/aws/ecs_svc_logs.go:78)  
  Trigger: another workload shares the log group, or the service has multiple awslogs containers.  
  User impact: “Service Logs” mixes unrelated logs and omits all but the first awslogs container.  
  Fix: enumerate all container log configurations and filter to actual service task streams; do not query an entire log group. AWS stream names include prefix, container, and task ID. [AWS docs](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_LogConfiguration.html)

- P2 — [core/aws/ecs_svc_logs.go:78](/Users/k2m30/projects/a9s/core/aws/ecs_svc_logs.go:78), [line 89](/Users/k2m30/projects/a9s/core/aws/ecs_svc_logs.go:89)  
  Trigger: a retained log group has more than 200 events.  
  User impact: the view retrieves the oldest events and can render up to AWS’s default 10,000-event response, despite claiming a 200-event cap and recent logs.  
  Fix: set `Limit` to remaining capacity and request newest-first with an explicit recent `StartTime`. [AWS docs](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_FilterLogEvents.html)

- P2 — [core/aws/ecs_svc_logs.go:61](/Users/k2m30/projects/a9s/core/aws/ecs_svc_logs.go:61), [core/aws/catalog_containers.go:343](/Users/k2m30/projects/a9s/core/aws/catalog_containers.go:343)  
  Trigger: the task definition’s `awslogs-region` differs from the selected ECS region.  
  User impact: service logs fail to load or show an identically named log group from the wrong region.  
  Fix: read `awslogs-region`, use the regional CloudWatch Logs client, and retain that region for console links. [AWS docs](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_LogConfiguration.html)

- P2 — [core/aws/ecs_svc.go:48](/Users/k2m30/projects/a9s/core/aws/ecs_svc.go:48)  
  Trigger: an ECS service has tags, including CloudFormation tags.  
  User impact: `Tags` renders empty and the CloudFormation related-resource check always misses tag-based links.  
  Fix: pass `Include: []ServiceField{ServiceFieldTags}` to `DescribeServices`. [AWS docs](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeServices.html)

- P2 — [core/aws/ecs_svc_related_extra.go:399](/Users/k2m30/projects/a9s/core/aws/ecs_svc_related_extra.go:399)  
  Trigger: a Step Functions ECS `RunTask` state uses the current `Arguments.TaskDefinition` form.  
  User impact: the ECS service’s Step Functions relationship is reported as zero.  
  Fix: inspect `Arguments` as well as legacy `Parameters`. [AWS docs](https://docs.aws.amazon.com/step-functions/latest/dg/connect-ecs.html)

- P3 — [core/aws/ecs_svc_related_extra.go:401](/Users/k2m30/projects/a9s/core/aws/ecs_svc_related_extra.go:401)  
  Trigger: a service task-definition family is a substring of another state machine task definition, such as `api` and `api-worker`.  
  User impact: unrelated state machines appear related to the ECS service.  
  Fix: parse static task-definition references and compare family names exactly; do not substring-match dynamic expressions.

- P2 — [core/aws/pipeline_related.go:193](/Users/k2m30/projects/a9s/core/aws/pipeline_related.go:193)  
  Trigger: a CodePipeline ECS blue/green deployment action.  
  User impact: the pipeline→ECS-service relationship is absent: AWS uses provider `CodeDeployToECS`, while the code checks `ECSBlueGreen` and fields that action does not contain.  
  Fix: recognize `CodeDeployToECS` and resolve its CodeDeploy deployment group to the ECS service. [AWS docs](https://docs.aws.amazon.com/codepipeline/latest/userguide/action-reference-ECSbluegreen.html)

- P2 — [core/aws/ecs_svc_related_extra.go:298](/Users/k2m30/projects/a9s/core/aws/ecs_svc_related_extra.go:298), [core/aws/parse.go:64](/Users/k2m30/projects/a9s/core/aws/parse.go:64)  
  Trigger: a same-region Secrets Manager secret is referenced by name in `ValueFrom` or `RepositoryCredentials`.  
  User impact: the service’s Secrets relationship incorrectly reports no secret.  
  Fix: resolve supported bare secret names against Secrets Manager rows, retaining ambiguity when the value could instead name SSM. [AWS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_definition_parameters_ec2.html)

- P2 — [core/aws/ecs_svc_related_extra.go:292](/Users/k2m30/projects/a9s/core/aws/ecs_svc_related_extra.go:292)  
  Trigger: a container’s logging driver uses `LogConfiguration.SecretOptions`.  
  User impact: credentials used by the service’s log driver are omitted from the Secrets relationship.  
  Fix: collect and resolve `LogConfiguration.SecretOptions[].ValueFrom` alongside environment and repository-credential secrets. [AWS docs](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/secrets-logconfig.html)

## ecs-task

## Findings

- **P1** — [core/aws/ecs_task.go:43](/Users/k2m30/projects/a9s/core/aws/ecs_task.go:43). **Trigger:** viewing top-level `ecs-task` resources; `ListTasks` omits `DesiredStatus`, whose AWS default is `RUNNING`. **Impact:** stopped—including recently failed—tasks are absent. **Fix:** enumerate and merge `RUNNING` and `STOPPED` with status-aware continuation state. [AWS ListTasks](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListTasks.html)

- **P2** — [core/aws/ecs_task.go:208](/Users/k2m30/projects/a9s/core/aws/ecs_task.go:208), [core/aws/ecs_task.go:227](/Users/k2m30/projects/a9s/core/aws/ecs_task.go:227), [core/aws/ecs_task_related.go:72](/Users/k2m30/projects/a9s/core/aws/ecs_task_related.go:72), [core/aws/ecs_task_related_extra.go:105](/Users/k2m30/projects/a9s/core/aws/ecs_task_related_extra.go:105). **Trigger:** a non-`ClientException` task-definition lookup fails for a revision shared by multiple tasks. The failed revision is cached as `nil`; subsequent tasks get no join-error marker, and role/secret/SSM panels return a definitive zero. **Impact:** missing relationships are presented as confirmed absence. **Fix:** cache an error/unknown state and propagate it to every task using that revision; relation checkers must return partial/unknown for it.

- **P2** — [core/aws/ecs_task_issue_enrichment.go:141](/Users/k2m30/projects/a9s/core/aws/ecs_task_issue_enrichment.go:141). **Trigger:** a non-essential container exits nonzero while the essential workload remains healthy. **Impact:** the task is labeled `task failed`, although an unsuccessful non-essential container does not stop or fail the task. **Fix:** require an essential-container failure before adding the task-level finding, or render this solely as a container-level condition. [AWS ContainerDefinition](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerDefinition.html)

- **P2** — [core/aws/ecs_task.go:54](/Users/k2m30/projects/a9s/core/aws/ecs_task.go:54), [core/aws/ecs_svc_tasks.go:137](/Users/k2m30/projects/a9s/core/aws/ecs_svc_tasks.go:137), [core/config/defaults_compute.go:45](/Users/k2m30/projects/a9s/core/config/defaults_compute.go:45), [core/config/defaults_compute.go:92](/Users/k2m30/projects/a9s/core/config/defaults_compute.go:92). **Trigger:** opening task details containing tags. Neither `DescribeTasks` request asks for `TAGS`. **Impact:** both task detail views render tags as empty. **Fix:** pass `Include: []TaskField{TaskFieldTags}` in both calls. [AWS DescribeTasks](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeTasks.html)

- **P2** — [core/aws/ecs_task.go:54](/Users/k2m30/projects/a9s/core/aws/ecs_task.go:54), [core/aws/ecs_svc_tasks.go:137](/Users/k2m30/projects/a9s/core/aws/ecs_svc_tasks.go:137). **Trigger:** AWS returns a successful `DescribeTasks` response with entries in `Failures`, such as when a listed task disappears before description. **Impact:** those listed tasks are silently omitted from the result. **Fix:** inspect `Failures`; retry appropriate transient cases and surface the result as partial when requested tasks cannot be described. [AWS DescribeTasks](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeTasks.html)

- **P2** — [core/aws/ecs_task.go:266](/Users/k2m30/projects/a9s/core/aws/ecs_task.go:266), [core/aws/ecs_task.go:276](/Users/k2m30/projects/a9s/core/aws/ecs_task.go:276), [core/aws/ssm.go:65](/Users/k2m30/projects/a9s/core/aws/ssm.go:65). **Trigger:** a secret references a same-Region SSM parameter by a bare name such as `MyParameter1`, or by an ARN for that name. Bare names without `/` are ignored; ARN-derived names are incorrectly changed to `/MyParameter1`. **Impact:** ECS-task → SSM relations fail for valid parameter references. **Fix:** retain the exact parameter name from `ValueFrom`/the ARN resource component, without forcing a leading slash. [AWS ECS Secret](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Secret.html), [AWS Parameter Store naming](https://docs.aws.amazon.com/systems-manager/latest/userguide/what-is-a-parameter.html)

- **P2** — [core/aws/ecs_task.go:260](/Users/k2m30/projects/a9s/core/aws/ecs_task.go:260), [core/aws/secrets_related_extra.go:244](/Users/k2m30/projects/a9s/core/aws/secrets_related_extra.go:244). **Trigger:** a container uses `LogConfiguration.SecretOptions`. **Impact:** the task-to-secret and secret-to-task relations omit credentials used by the logging driver. **Fix:** collect and resolve `LogConfiguration.SecretOptions[].ValueFrom` alongside container secrets and repository credentials. [AWS log-configuration secrets](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/secrets-logconfig.html)

- **P2** — [core/aws/secrets_related_extra.go:166](/Users/k2m30/projects/a9s/core/aws/secrets_related_extra.go:166), [core/aws/secrets_related_extra.go:185](/Users/k2m30/projects/a9s/core/aws/secrets_related_extra.go:185). **Trigger:** the `ecs-task` cache entry is marked `FieldsOnly`. The checker skips every task before reaching its field fallback. **Impact:** a Secret’s related ECS Tasks panel can return a false zero. **Fix:** use the generic related-resource loader, or read `Fields["task_definition"]` before requiring `RawStruct`.

- **P2** — [core/aws/ecs_task_issue_enrichment.go:251](/Users/k2m30/projects/a9s/core/aws/ecs_task_issue_enrichment.go:251). **Trigger:** a Windows task definition is enriched. `readonlyRootFilesystem` is unsupported for Windows and therefore cannot satisfy this rule. **Impact:** every Windows container is incorrectly flagged for a writable root filesystem, with an impossible remediation. **Fix:** skip this rule when `RuntimePlatform.OperatingSystemFamily` is Windows. [AWS ContainerDefinition](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerDefinition.html)

- **P2** — [core/aws/alarm_match.go:141](/Users/k2m30/projects/a9s/core/aws/alarm_match.go:141), [core/aws/alarm_match.go:232](/Users/k2m30/projects/a9s/core/aws/alarm_match.go:232). **Trigger:** a Container Insights alarm is scoped only to a cluster, or to a cluster and service. Matching accepts `ClusterName` as task identity. **Impact:** a cluster-wide alarm links to every task in that cluster; a service-wide alarm links to every task in that service. **Fix:** require a `TaskId` dimension for an ECS-task alarm relation. [AWS Container Insights metrics](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/Container-Insights-metrics-ECS.html)

- **P3** — [core/aws/logs_related.go:188](/Users/k2m30/projects/a9s/core/aws/logs_related.go:188), [core/aws/logs_related.go:205](/Users/k2m30/projects/a9s/core/aws/logs_related.go:205). **Trigger:** a log group happens to be named `/ecs/<task-family>` but is not configured as that task definition’s `awslogs-group`. **Impact:** unrelated tasks are reported as related to that log group. **Fix:** resolve the task definition and match its explicit `awslogs-group` option; retain naming only as a clearly non-definitive fallback. [AWS LogConfiguration](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_LogConfiguration.html)

- **P3** — [core/aws/ecs_task.go:45](/Users/k2m30/projects/a9s/core/aws/ecs_task.go:45), [core/aws/ecs_svc_tasks.go:74](/Users/k2m30/projects/a9s/core/aws/ecs_svc_tasks.go:74), [core/resource/accessors.go:14](/Users/k2m30/projects/a9s/core/resource/accessors.go:14). **Trigger:** a cluster has over 50 tasks, or a service has up to 100 running plus 100 stopped tasks. The top-level path requests 100; the service-child path leaves the AWS maximum unspecified and combines both status pages. **Impact:** a nominal 50-row page can contain 100 or 200 rows, making pagination boundaries and load-more behavior inconsistent. **Fix:** request and emit at most `DefaultPageSize`, carrying undisplayed task ARNs in the continuation state.

## ecs

- **P1** — [ecs_svc_logs.go:78](/Users/k2m30/projects/a9s/core/aws/ecs_svc_logs.go:78)  
  **Trigger:** Multiple services or containers write to the same CloudWatch log group.  
  **Impact:** A service’s Logs view includes unrelated workloads’ events; it also ignores every `awslogs` container after the first.  
  **Fix:** Collect all configured container log groups and scope each query to the service’s actual task log streams, rather than querying the entire group. [AWS FilterLogEvents](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_FilterLogEvents.html)

- **P2** — [ecs_svc_logs.go:78](/Users/k2m30/projects/a9s/core/aws/ecs_svc_logs.go:78)  
  **Trigger:** A log group contains more than 200 retained events.  
  **Impact:** “Service Logs” starts with the oldest retained events, so current failures are absent.  
  **Fix:** Request newest-first results (`StartFromHead: false`) with an appropriate bounded start time and preserve cursor semantics. [AWS FilterLogEvents](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_FilterLogEvents.html)

- **P2** — [ecs.go:45](/Users/k2m30/projects/a9s/core/aws/ecs.go:45)  
  **Trigger:** Viewing an ECS cluster’s `Settings` detail.  
  **Impact:** Settings, including Container Insights configuration, are always missing because `SETTINGS` is not requested in `DescribeClusters.Include`.  
  **Fix:** Include `ecstypes.ClusterFieldSettings`. [AWS DescribeClusters](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeClusters.html)

- **P2** — [ecs_task.go:43](/Users/k2m30/projects/a9s/core/aws/ecs_task.go:43)  
  **Trigger:** A task stops and no longer appears in the default `ListTasks` response.  
  **Impact:** The top-level ECS Tasks resource omits stopped/failed tasks and their stop diagnostics, despite modeling them.  
  **Fix:** Enumerate both `RUNNING` and `STOPPED` task statuses with a composite cursor, as the service-task path does. [AWS ListTasks](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListTasks.html)

- **P2** — [ecs_issue_enrichment.go:108](/Users/k2m30/projects/a9s/core/aws/ecs_issue_enrichment.go:108)  
  **Trigger:** An EC2-backed cluster has registered capacity but is intentionally idle.  
  **Impact:** A healthy empty or pre-warmed cluster is marked with a cluster issue.  
  **Fix:** Only flag this when there is evidence of unmet workload demand, such as desired service tasks exceeding running tasks.

- **P2** — [ecs_related_extra.go:44](/Users/k2m30/projects/a9s/core/aws/ecs_related_extra.go:44)  
  **Trigger:** An account has multiple ECS clusters using managed-scaling capacity providers.  
  **Impact:** Every ASG tagged `AmazonECSManaged` is related to every cluster, producing incorrect navigation and capacity attribution.  
  **Fix:** Resolve the cluster’s capacity providers and compare their configured ASG ARNs; do not use the generic ECS-managed tag as a cluster selector. [AWS ECS managed scaling](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/cluster-auto-scaling.html)

- **P2** — [ecs_related_extra.go:82](/Users/k2m30/projects/a9s/core/aws/ecs_related_extra.go:82)  
  **Trigger:** An ECS Managed Instances capacity provider creates EC2 instances.  
  **Impact:** Cluster-to-instance relations are absent because the code checks `aws:ecs:cluster-name` instead of AWS’s `aws:ecs:clusterName` tag.  
  **Fix:** Match `aws:ecs:clusterName`, retaining legacy/custom aliases only if needed. [AWS ECS Managed Instances tags](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/instance-details-tags-managed-instances.html)

- **P2** — [secrets_related_extra.go:188](/Users/k2m30/projects/a9s/core/aws/secrets_related_extra.go:188)  
  **Trigger:** ECS task entries are present only as persisted fields rather than live `RawStruct` values.  
  **Impact:** A secret can incorrectly report zero related ECS tasks even when task definitions reference it.  
  **Fix:** Match persisted `secret_arns` fields when raw task data is unavailable, or fetch the missing target before returning a definitive empty result.

- **P2** — [ecs_svc_related_extra.go:138](/Users/k2m30/projects/a9s/core/aws/ecs_svc_related_extra.go:138)  
  **Trigger:** EventBridge rule entries are persisted without live `RawStruct` values.  
  **Impact:** ECS services incorrectly report no related EventBridge rules despite having stored event patterns that match them.  
  **Fix:** Parse the persisted `event_pattern` field as a fallback, or fetch unavailable rule targets before returning zero relations.

## efs

- **P1** — [core/aws/efs_issue_enrichment.go:164](/Users/k2m30/projects/a9s/core/aws/efs_issue_enrichment.go:164)  
  Trigger: an EFS file system has no user-defined file-system policy, so `DescribeFileSystemPolicy` returns `PolicyNotFound`.  
  Impact: the code reports it clean, although EFS’s default policy grants full access to anonymous clients that can reach a mount target.  
  Fix: classify the normal no-policy response as public-default exposure (while preserving unknown/error handling for inaccessible policies). [AWS documentation](https://docs.aws.amazon.com/efs/latest/ug/iam-access-control-nfs-efs.html)

- **P1** — [core/iampolicy/evaluate.go:336](/Users/k2m30/projects/a9s/core/iampolicy/evaluate.go:336)  
  Trigger: a wildcard-principal EFS policy is constrained only by a fixed `elasticfilesystem:AccessPointArn`.  
  Impact: it is treated as non-public and the critical finding is omitted, but AWS does not consider that condition sufficient to make an EFS policy non-public.  
  Fix: use an EFS-specific public-policy evaluator based on EFS’s documented non-public criteria rather than the generic condition list. [AWS documentation](https://docs.aws.amazon.com/efs/latest/ug/access-control-block-public-access.html)

- **P2** — [core/iampolicy/evaluate.go:332](/Users/k2m30/projects/a9s/core/iampolicy/evaluate.go:332)  
  Trigger: a wildcard-principal EFS policy limits access with `elasticfilesystem:AccessedViaMountTarget: true`.  
  Impact: the application raises a critical public-policy finding even though AWS explicitly classifies this policy shape as non-public.  
  Fix: have the EFS evaluator recognize that condition as a non-public constraint. [AWS documentation](https://docs.aws.amazon.com/efs/latest/ug/access-control-block-public-access.html)

- **P2** — [core/aws/efs.go:37](/Users/k2m30/projects/a9s/core/aws/efs.go:37)  
  Trigger: a newly created file system is in `creating` with zero mount targets.  
  Impact: it is marked broken for having no mount targets, despite AWS requiring the file system to become `available` before mount targets can be created; the suggested remediation is impossible at that point.  
  Fix: suppress the no-mount-target finding while the file system is creating. [AWS documentation](https://docs.aws.amazon.com/efs/latest/APIReference/API_CreateFileSystem.html)

- **P3** — [core/aws/catalog_databases.go:644](/Users/k2m30/projects/a9s/core/aws/catalog_databases.go:644)  
  Trigger: a public policy grants only `elasticfilesystem:ClientMount`.  
  Impact: the finding says clients can “read and write,” although `ClientMount` grants read-only access.  
  Fix: make the detail action-aware, or describe the exposure neutrally as client access. [AWS documentation](https://docs.aws.amazon.com/efs/latest/ug/security_iam_resource-based-policy-examples.html)

## eip

- P1 — [core/aws/eip.go:73](/Users/k2m30/projects/a9s/core/aws/eip.go:73)  
  Trigger: an account has two or more `Domain=standard` Elastic IPs. AWS documents these addresses without an `AllocationId`.  
  Impact: every such row gets `ID == ""`, so downstream ID-keyed handling retains only one ([resource.go:25](/Users/k2m30/projects/a9s/core/resource/resource.go:25)); inventory, findings, navigation, console links, and CloudTrail lookup are incorrect or unavailable.  
  Fix: use `AllocationId` when present, otherwise use `PublicIp` as the canonical ID; branch ID-dependent console/CloudTrail handling for standard addresses. [AWS DescribeAddresses documentation](https://docs.aws.amazon.com/ec2/latest/devguide/example_ec2_DescribeAddresses_section.html)

- P2 — [core/aws/alarm_match.go:234](/Users/k2m30/projects/a9s/core/aws/alarm_match.go:234)  
  Trigger: an EIP is associated with an EC2 instance that has a normal `AWS/EC2` CloudWatch alarm, such as `NetworkIn`, dimensioned by `InstanceId`.  
  Impact: the EIP’s CloudWatch Alarms relation always misses that alarm because it looks for an unsupported `NetworkInterfaceId` dimension and ignores `Address.InstanceId`.  
  Fix: match EC2 alarms using the EIP’s associated instance ID and the `InstanceId` dimension; resolve NAT/NLB ownership separately if their native alarms should also appear. [AWS EC2 metric dimensions](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/viewing_metrics_with_cloudwatch.html)
