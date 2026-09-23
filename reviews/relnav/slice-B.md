# Related-nav triage — slice B

Types: eb ebs-snap ebs ec2 ecr ecs-svc ecs-task ecs efs eip eks elb eni glue iam-group iam-user igw kinesis.
Tree: main @ ad18304c. Paths are relative to `core/aws/` unless they start with another top-level directory.
Codex has no sections for iam-group, iam-user, igw or kinesis. The "Rejected as not small" rows in the oneliner files that name these types (ec2_by_ids batching, EBS Multi-Attach, ecs-task bare SSM names, pipeline CodeDeployToECS, EFS access points, DescribeTargetHealth, DescribeContainerInstances, cluster-qualified ecs-svc IDs, ARN-keyed elb, namespaced inline-policy IDs) duplicate review findings and are merged into those rows. Backlog rows 5 and 11 concern these types. Rows 1, 2, 3 and 12 do not.

## 1. Findings

| # | type | source | verdict | class | current file:line | evidence |
|---|------|--------|---------|-------|-------------------|----------|
| 1 | eb | codex P2 eb→s3 lists every app version's bucket | LIVE | MATCH | eb_related_extra.go:350-364 | Pages DescribeApplicationVersions(ApplicationName) with no VersionLabels filter, so buckets of versions the env does not run are counted. docs/resources/eb.md:73 describes it at application scope, so the spec has the same gap |
| 2 | eb | codex P2 role omits OperationsRole | LIVE | OTHER (contract gap) | eb_related_extra.go:300-319 | Only IamInstanceProfile and ServiceRole are read. docs/resources/eb.md:67 does not list OperationsRole, so the contract needs the AWS field |
| 3 | eb | claude P2 #2 ServiceRole ARN unnormalized | FIXED | — | eb_related_extra.go:314-319 | ServiceRole goes into refs → resolveRefs("role") → roleRefToID (ref_ids.go:278) reduces an ARN to the name |
| 4 | eb | claude P2 #3 Classic-ELB env + codex elb P2 CLB name emitted | LIVE | ID | eb_related_extra.go:52-56, :106-118, :162-167 | checkEbELB appends lb.Name as-is, with no resolveRefs and no list check. The elb list is ELBv2-only (asg_related.go:126-127 says so), so a CLB count opens nothing. checkEbTG: DescribeLoadBalancers(Names=[clb]) errors, and when every LB fails the result is ErrorRelated |
| 5 | eb | claude P2 #5 alarm substring/any-dimension | FIXED | — | eb_related.go:156-157, alarm_match.go:227 | Uses the one match table: AWS/ElasticBeanstalk and ElasticBeanstalk/SQSD, dimension EnvironmentName, exact value |
| 6 | eb | codex P2 secrets→eb skips cache rows without RawStruct | LIVE | CARD | secrets_related_extra.go:80, :93-97, :143 | Reads cache["eb"] directly, ignores FieldsOnly and `continue`s past rows with no struct. A disk-restored eb cache gives relatedResultTrunc(nil,false), a proven 0 |
| 7 | eb | claude P2 #4 secrets→eb misses environmentsecrets namespace | LIVE | MATCH | secrets_related_extra.go:90, :124 | Matches only `{{resolve:secretsmanager:<arn>`. An `aws:elasticbeanstalk:application:environmentsecrets` option holds the plain ARN and is never matched |
| 8 | ebs-snap | claude P2 #1 + codex ebs P2 CopySnapshot volume ID | LIVE (residual) | NAV | pivot fixed at ebs_snap_related.go:50-55 and ebs_snap_issue_enrichment.go:126-129. Residual at catalog_compute.go:848 → ref_ids.go:578-586, :560-562 | The pivot and the orphan finding now skip `vol-ffffffff`. The VolumeId navigable resolves through ebsRefToID, and listHolds returns true when no ebs list is loaded, so `vol-ffffffff` stays navigable and opens nothing. Codex's claim that copies can carry other placeholder IDs is not verified |
| 9 | ebs-snap | claude P2 #2 Backup pivot exact-ARN only | FIXED | — | ebs_snap_related.go:122-130 | backupPivot / backupTarget (the shared per-selection evaluator from #551) |
| 10 | ebs-snap | claude P3 #4 EC2 pivot proven 0 without CreateImage | DISPROVED | — | ebs_snap_related.go:60-74, docs/resources/ebs-snap.md:49 | The current contract defines the pivot as the CreateImage description only, with a known 0 when absent. The code matches the contract |
| 11 | ebs-snap | claude P3 #5 FetchEBSSnapshotsByIDs loses the batch on one bad ID | LIVE | NAV | ebs.go:208-215 | A DescribeSnapshots error returns (nil, err) and nothing retries without the bad ID (the ec2_by_ids pattern). An exact-ID drill with one deleted snapshot shows an error, not the rest |
| 12 | ebs | claude P3 #4 + codex ebs P2 + codex ec2 P2 Multi-Attach first attachment only | FIXED | — | ebs.go:78-83, ebs_related.go:15-21 | attached_to joins every Attachments[].InstanceId. checkEBSEC2 splits the CSV |
| 13 | ebs | claude P2 #2 Backup panel vs finding use different predicates | FIXED | — | ebs_related.go:113-133 | backupPivot with ARN + tags. Tags unread → target.unread |
| 14 | ebs | claude P2 #3 instance-level backup selection ignored (panel part) | LIVE | MATCH | ebs_related.go:118-132 | backupTarget carries only the volume ARN and volume tags. An attached instance's ARN or tags are never tested, so a volume under an instance-selecting plan shows Backup 0 |
| 15 | ec2 | claude P2 #1 ec2→tg lists every instance TG in the VPC | LIVE (candidate-labelled) | CONTRACT | ec2_related.go:21-53, catalog_compute.go:347 | Now heuristicResult with a Distinct declaring "sharing this instance's VPC", but registration is still unread. docs/resources/ec2.md §2 tg requires DescribeTargetHealth membership |
| 16 | ec2 | claude P2 #2 ec2→ssm returns instance ID | FIXED | — | catalog_compute.go:347-365 | The ec2 Related list has no ssm row (unregistered, commit 5709d392) |
| 17 | ec2 | claude P2 #4 Backup ARN field empty | FIXED | — | ec2_related_extra.go:125 | Builds the ARN with sessionEC2ARN(ctx, clients, "instance", id) |
| 18 | ec2 | claude P3 #7 ec2→ebs-snap omits AMI root snapshots | LIVE | CONTRACT | ec2_related.go:195-220, catalog_compute.go:354 | Volume snapshots only (Distinct says so). docs/resources/ec2.md:67 still says "must union both sources" with ImageId → AMI → BlockDeviceMappings |
| 19 | ec2 | codex P2 by-ID fetch sends an unbounded ID list | LIVE | PAGE | ec2_by_ids.go:96-99 | One DescribeInstances(InstanceIds: ids) with no batching. The 1000-ID limit is the reviewer's premise and I have not verified it |
| 20 | ecr | claude P2 #3 ecr→lambda counts every image Lambda | FIXED | — | ecr_related.go:55-91 | GetFunction image URI + imageRefersToRepo (predicates.go:26) |
| 21 | ecr | claude P2 #4 + ecs-task claude P3 #11b ecr→ecs-task substring | FIXED | — | ecr_related_extra.go:36-37, predicates.go:26-39 | Exact registry/repo comparison |
| 22 | ecr | claude P2 #5 ecr→cb prefix and env vars | FIXED | — | ecr_related.go:115, docs/resources/ecr.md:31 | imageRefersToRepo is exact. The contract now names Environment.Image only |
| 23 | ecr | claude P3 #6 ecr→ct-events substring | FIXED | — | install.go:18-27 | Every CT pivot defers to a LookupEvents ResourceName filter built from the type's CloudTrailKey |
| 24 | ecr | claude P3 #7 + codex P2 ecr→eb-rule matching | LIVE (partly fixed) | MATCH | ecr_related.go:270-275, :283-287, :293-294 | The resources ARN is now exact (fixed). `repository-name` is unmarshalled as []string, so `[{"prefix":…}]` fails silently and the rule reads as not matching. A rule with source aws.ecr and no filter matches every repo |
| 25 | ecr | backlog #5 ct-events registers no ecr pivot | LIVE | CONTRACT | catalog_monitoring.go:261-276 | The ct-events Related list has no ecr row, and the docs/related-resources.md ct-events section does not list ecr either |
| 26 | ecs-svc | claude P1-1 ListServices not paginated | FIXED | — | ecs_svc.go:38-46 | Paged with MaxResults 100 and NextToken |
| 27 | ecs-svc | claude P1-2 row ID is the bare service name | FIXED | — | ecs_svc.go:99, :124-127, ref_ids.go:224-252 | ecsSvcID(cluster, name). ecsSvcRefToID / ecsSvcRefFromTask qualify task, eip and pipeline refs |
| 28 | ecs-svc | claude P2-1 + codex P2 DescribeServices without TAGS | FIXED | — | ecs_svc.go:49-54 | Include: ServiceFieldTags |
| 29 | ecs-svc | claude P2-3 secrets pivot emits raw ValueFrom | FIXED | — | ecs_svc_related_extra.go:311, ref_ids.go:385-414 | relatedRefs("secrets") strips `:json-key…` and the random suffix |
| 30 | ecs-svc | codex P2 bare secret names in ValueFrom | LIVE | MATCH | ecs_svc_related_extra.go:298, parse.go:63-66 (same at ecs_task.go:270) | isSecret requires a secretsmanager ARN, so a same-Region bare name is dropped and the result is a proven 0 |
| 31 | ecs-svc, ecs-task | codex ecs-svc P2 + codex ecs-task P2 LogConfiguration.SecretOptions unread | LIVE | MATCH | ecs_svc_related_extra.go:292-309, ecs_task.go:262-289 | Only Secrets[] and RepositoryCredentials are read, so log-driver secrets are missing in both directions (also secrets_related_extra.go secretsECSTaskRefsSecret) |
| 32 | ecs-svc | claude P2-4 + ecs-task claude P2 #10 tasks ignore cluster | FIXED | — | ecs_svc_related_extra.go:50 | Compares ecsSvcID(cluster, svcName) |
| 33 | ecs-svc | claude P2-5 ct-events substring | FIXED | — | install.go:18-27, catalog_compute.go:400-402 | Deferred lookup on service_name with a cluster qualifier |
| 34 | ecs-svc | claude P2-6 logs by family substring | FIXED | — | ecs_svc_related.go:185-197, related_common.go:199-228 | awslogs-group options, exact through listedRefs. Falls back to candidates only when the definition is unread |
| 35 | ecs-svc | claude P2-7 role omits task and execution roles | LIVE | CONTRACT | ecs_svc_related.go:221-230 | Service.RoleArn only. docs/resources/ecs-svc.md:80 requires TaskDefinition.ExecutionRoleArn and TaskRoleArn |
| 36 | ecs-svc | claude P2-8 eb-rule uses patterns, not targets | LIVE | MATCH | ecs_svc_related_extra.go:142-144, :207-210 | Schedule rules (no EventPattern) are skipped, and source aws.ecs with no filter matches every service. The cluster substring is fixed (exact lastSegment, :189-195). docs/resources/ecs-svc.md:43 requires ListTargetsByRule targets |
| 37 | ecs-svc | codex ecs P2 eb-rule rows without RawStruct | LIVE | CARD | ecs_svc_related_extra.go:131-141 | Reads cache["eb-rule"] directly and skips rows with no struct, so a fields-only cache gives a proven 0 |
| 38 | ecs-svc | claude P3-1 + ecs-task claude P3 #11a ECR repos from other registries | FIXED | — | ecr_related.go:31-50 | ecrWorkloadRepos keeps only loaded repos whose URI matches exactly |
| 39 | ecs-svc | claude P3-2 + codex P2 (Arguments) + codex P3 (substring) sfn | LIVE | MATCH | ecs_svc_related_extra.go:399-410 | Only `Parameters` is read, so JSONata `Arguments` is missed. `strings.Contains(td, family)` makes `api` match `api-worker` |
| 40 | ecs-svc | codex P2 pipeline→ecs-svc misses CodeDeployToECS | LIVE | MATCH | pipeline_related.go:192-195 | Accepts provider "ECS" or "ECSBlueGreen". AWS's blue/green provider is CodeDeployToECS (reviewer's AWS citation) |
| 41 | ecs-task | claude P1 #1 ListTasks first page only | FIXED | — | ecs_task.go:43-51 | Paged with MaxResults 100 and NextToken |
| 42 | ecs-task | claude P1 #2 ec2 pivot returns container-instance UUID | FIXED | — | ecs_task_related_extra.go:25-53 | DescribeContainerInstances → Ec2InstanceId |
| 43 | ecs-task | claude P2 #5 JSON-key/version secret refs | FIXED | — | ecs_task_related_extra.go:104-121, ref_ids.go:385-414, secrets_related_extra.go (secretsECSTaskRefsSecret) | Both directions resolve through secretsRefToID |
| 44 | ecs-task | claude P2 #6 + codex P2 SSM parameter names | LIVE | ID | ecs_task.go:274-283 | An ARN always becomes "/"+name, so flat `db_password` becomes `/db_password`. A bare name without a leading "/" is dropped |
| 45 | ecs-task | claude P2 #7 + codex P2 task-def lookup failure → definite 0 | LIVE (partly fixed) | CARD | ecs_task.go:209-211, :228-230 | The first failing task now gets task_def_join_error, and role, secrets and ssm return Unknown (ecs_task.go:317-319). The failure is cached as nil, so every later task on that definition gets empty fields, no flag, and a proven 0 |
| 46 | ecs-task | claude P2 #8 logs by family substring | FIXED | — | ecs_task_related.go:43-53 | ecsTaskDefLogGroups (awslogs-group) |
| 47 | ecs-task | claude P2 #9 alarm dimensions | LIVE (partly fixed) | MATCH | alarm_match.go:229 | ClusterName with a ServiceName qualifier is matched now, but only in namespace ECS/ContainerInsights. An AWS/ECS CPU or memory alarm on ClusterName+ServiceName (docs/resources/ecs-task.md:30-31) never matches |
| 48 | ecs-task | codex P2 cluster-only alarm links every task | DISPROVED | — | docs/resources/ecs-task.md:31, alarm_match.go:147-167 | The contract matches ClusterName, plus ServiceName when the task has one. A cluster-scoped alarm covering every task is the intended relation |
| 49 | ecs-task | codex ecs-task P2 + codex ecs P2 secrets→ecs-task with no RawStruct | LIVE | CARD | secrets_related_extra.go:168, :187-190 | Direct cache read. A row with no struct is `continue`d before the Fields["task_definition"] fallback, so a fields-only cache gives a proven 0 |
| 50 | ecs-task | codex P3 logs→ecs-task by `/ecs/<family>` name | LIVE | MATCH | logs_related.go:183-209 | Log group name → family, returned as an exact count. The forward direction reads awslogs-group (related_common.go:199), so the two ends disagree |
| 51 | ecs | claude P1 #1 DescribeClusters without TAGS or CONFIGURATIONS | FIXED | — | ecs.go:40-45 | Include: Configurations and Tags |
| 52 | ecs | claude P2 #2 + codex P2 ASG by AmazonECSManaged tag | LIVE | MATCH | ecs_related_extra.go:43-48 | The value-less AmazonECSManaged key matches every cluster's capacity ASGs. docs/resources/ecs.md:37 requires CapacityProviders → DescribeCapacityProviders |
| 53 | ecs | claude P2 #3 + codex P2 EC2 by tags ECS does not set | LIVE | MATCH | ecs_related_extra.go:78-86 | Matches `aws:ecs:cluster-name` or `ClusterName`. docs/resources/ecs.md:49 requires ListContainerInstances + DescribeContainerInstances |
| 54 | ecs | claude P2 #4 ct-events substring | FIXED | — | install.go:18-27 | Deferred exact lookup |
| 55 | ecs | claude P2 #5 logs substring | FIXED | — | ecs_related_extra.go:125-145 | Exec-command CloudWatchLogGroupName, exact |
| 56 | ecs, eks | ecs claude P3 #6, eks claude P2 #5 and P3 #6 alarm namespace | FIXED | — | alarm_match.go:229, :234, alarm_related_extra.go:35-37 | Both directions use one table. ecs is AWS/ECS and ECS/ContainerInsights; eks is AWS/EKS and ContainerInsights, exact |
| 57 | efs | claude P2 #1 lambda→efs returns fsap IDs | FIXED | — | lambda_related_extra.go:45-56, ref_ids.go:434-458, efs.go:128 | efsRefToID maps an access point to its file system through access_point_ids |
| 58 | efs | claude P2 #2 subnet→efs description parse | FIXED | — | subnet_related.go:271, predicates.go:117-124 | Token regex `\bfs-[0-9a-z]+\b` handles both formats |
| 59 | efs | claude P2 #4 backup ignores tags and partial selections | FIXED | — | efs_related_extra.go:83-106 | backupPivot with ARN + tags |
| 60 | efs | claude P3 #6 efs→lambda access points first page | FIXED | — | efs_related.go:178-199 | PageAll, truncated when incomplete |
| 61 | efs | claude P3 #7 KmsKeyId navigable not rendered | FIXED | — | core/config/defaults_databases.go:76 | KmsKeyId is in the efs detail paths |
| 62 | eip | codex P1 Domain=standard EIPs get ID "" | DISPROVED (premise) | — | eip.go:35, :73 | Only EC2-Classic had standard-domain addresses, and AWS retired EC2-Classic on 2023-08-15. This comes from my knowledge of AWS; I did not check it against a live account. If any standard address remained, the finding would be live |
| 63 | eip | claude P3 + codex P2 alarm pivot dimension | LIVE | MATCH | alarm_match.go:233, :280-286, eip_related.go:97-99 | InstanceId was dropped (claude's point is fixed). The pivot now matches AWS/EC2 + NetworkInterfaceId, a dimension codex says AWS/EC2 does not publish, so it matches no AWS-published alarm. docs/resources/eip.md:31 has the same premise |
| 64 | eks | codex P1 + claude P2 #2 self-managed / Auto Mode nodes → proven 0 | FIXED | — | eks_related_extra.go:108-160, :46-79, :308-336 | Tagged-node discovery (kubernetes.io/cluster, eks:eks-cluster-name, eks:cluster-name) is merged with the node groups |
| 65 | eks | claude P2 #3 eks→ami exact 0 for default MNG | FIXED | — | eks_related_extra.go:208-217 | The nodes' ImageId is added to the AMI set |
| 66 | eks | claude P3 #4 $Latest vs $Default | FIXED | — | asg_related.go:308-309 | One launchTemplateVersions helper, defaulting to $Default |
| 67 | eks (ng) | codex P2 ng→sg ignores launch-template SGs | LIVE | CARD | ng_related.go:167-176 | Reads RemoteAccessSecurityGroup only. A launch-template node group gets a proven 0 |
| 68 | eks (ng) | codex P2 custom AMI in ReleaseVersion / LT by name | LIVE (eks side fixed) | CARD | ng_related.go:187-188, :210 | The eks pivot gets the AMI through nodes (row 65). ng→ami returns a proven 0 without LaunchTemplate.Id and ignores an `ami-` ReleaseVersion and a template name |
| 69 | eks | codex P2 Auto Mode NodeRoleArn | LIVE | OTHER (contract gap) | eks_related.go:168-176 | Cluster.RoleArn only. docs/resources/eks.md:79 names only RoleArn |
| 70 | eks | claude P3 #7 role/subnet→eks skip degraded rows | FIXED | — | related_fetch.go:40, :64-73 | anyDegraded makes the scan a lower bound |
| 71 | eks | backlog #11 alarm detail "EKS Clusters (0+)" | LIVE | CARD | related_fetch.go:40, :64 | anyDegraded marks a lower bound even for ID/Name-only matches, and the fetch branch folds a per-item DescribeCluster error into truncated |
| 72 | elb | codex P1 apigw→elb by VPC-link subnet/SG overlap | LIVE | MATCH | apigw_related.go:338-396 | Any LB sharing a subnet or SG with the link counts, as an exact count (relatedResultTrunc, not heuristic). The integration's listener ARN is not read |
| 73 | elb | claude P2 #2 tg LoadBalancerArns navigable opens empty | FIXED | — | ref_ids.go:416-431, core/app/detail_body.go:250-275 | Navigable IDs resolve through elbRefToID |
| 74 | elb | claude P2 #3 asg→elb emits ARNs and CLB names | FIXED | — | asg_related.go:125-159 | resolveRefs("elb") on TG LoadBalancerArns. Classic names are dropped |
| 75 | elb | claude P3 #4 elb→acm SNI certificates and paging | FIXED | — | elb_related.go:148-173 | Listeners are paged and DescribeListenerCertificates is used |
| 76 | elb | claude P3 #5 CF↔ELB DNS exact strings | FIXED | — | elb_related.go:232-234, cf_related.go:107 | dnsAliasNames in both directions |
| 77 | eni | codex P1 no FetchByIDs for eni | LIVE | NAV | catalog_networking.go:627-646 | The eni TypeDef has a Fetcher and no FetchByIDs. An exact-ID drill to an ENI past page 1 falls back to the first list page |
| 78 | eni | codex P2 dbi→eni without IncludeManagedResources | LIVE | CALL | dbi_related.go:253-258 | The flag is unset, while eni.go:25 sets it. Whether RDS ENIs are hidden "managed resources" is the reviewer's premise and I have not verified it |
| 79 | eni | claude P2 #1 eni list omits managed ENIs | FIXED | — | eni.go:25 | IncludeManagedResources: true |
| 80 | eni | claude P2 #3 dbi→eni counts other DBs' ENIs | LIVE (candidate-labelled) | MATCH | dbi_related.go:249-272 | Filters are still description + group-id only. The result is now heuristicResult (candidates), so it is no longer an exact count |
| 81 | eni | claude P2 #4 Lambda ENI predicate differs by direction | FIXED | — | predicates.go:107-109, eni_related.go:142, lambda_related_extra.go:388-391 | One isLambdaENI (InterfaceType == lambda) |
| 82 | eni | claude P3 #5 EIPs on secondary private IPs | LIVE (pivot fixed) | NAV | pivot eni_related.go:75-77. Residual catalog_networking.go:665 | checkENIEIP walks PrivateIpAddresses[].Association. The only navigable is Association.AllocationId (primary IP) |
| 83 | eni | claude P3 #6 shared Hyperplane ENI resolves to one function | LIVE | MATCH | lambda_related_extra.go:388-392, eni_related.go:146-150 | Both ends key on the single function name in Description |
| 84 | glue | claude P2 #4 + codex P2 logs pivot fixed pair | LIVE | CONTRACT | glue_related.go:39-56 | Only /aws-glue/jobs/output and /error. docs/resources/glue.md:49 requires `--continuous-log-logGroup` |
| 85 | glue | claude P2 #5 secret ID keeps random suffix | FIXED | — | glue_related.go:192, ref_ids.go:385-414 | relatedRefs("secrets") |
| 86 | glue | claude P2 #6 secrets ignore Connections | LIVE | CONTRACT | glue_related.go:176-192 | DefaultArguments only. docs/resources/glue.md:67 requires GetConnection → SECRET_ID |
| 87 | glue | claude P2 #7 athena pivot always 0 | FIXED | — | catalog_data.go:71-78, docs/resources/glue.md:26 | Pivot removed. The contract excludes it explicitly |
| 88 | glue | claude P3 #8 KMS alias ARN split | FIXED | — | glue_related.go:172 | kmsRelated → kmsRefToID alias handling |
| 89 | iam-group | claude P2 #1 + iam-user claude P2 #3 GetGroup not paged | FIXED | — | iam_groups_related.go:26-34, :63-71 | PageAll with IsTruncated/Marker, truncated when incomplete |
| 90 | iam-group | claude P2 #2 inline policies keyed by bare name | FIXED | — | iam_groups_related.go:55-57, iam_policies.go:453-460 | inlinePolicyID = inline/<group>/<name> |
| 91 | iam-group | claude P2 #3 inline sweep lists the first page of groups | LIVE | PAGE | iam_policies.go:355-357, :395-396 | ListGroups is called once with no Marker loop, and per-group ListGroupPolicies is also unpaged. A group past page 1 has an inline pivot the resolver never stores |
| 92 | iam-group | claude P3 #4 IsTruncated ignored on attached/inline lists (checker) | FIXED | — | iam_groups_related.go:47-58, :73-93 | Both lists are paged, and the result is truncated when incomplete. The sweep side is row 91 |
| 93 | iam-user | claude P2 #2 ct-events counts role sessions named like the user | LIVE | MATCH | catalog_security.go:228 | CloudTrailKey is "Username:ID". A deferred LookupEvents on Username also returns AssumedRole sessions with that session name, and nothing filters on userIdentity.type |
| 94 | iam-user | claude P3 #6 CT TARGET User NavID keeps the IAM path | LIVE | ID | core/semantics/ctevent/target.go:134-140 | NavID is the hand-split text after the first "/". Only role gets roleNavID, so `user/eng/alice` → `eng/alice` |
| 95 | iam-user | claude P3 #7 role→iam-user counts foreign-account users | FIXED | — | iam_roles_related.go:62-83, ref_ids.go:124-135, :289-300 | policyRefContext + localARN drop another account's ARN |
| 96 | igw | claude P2 #1 every rtb GatewayId is an igw link | FIXED | — | ref_ids.go:492-495, core/app/detail_body.go:271-275 | igwRefToID accepts only `igw-`. An unresolvable value is not navigable |
| 97 | igw | claude P3 #2 blackhole routes counted one way | FIXED | — | igw_related.go:58, predicates.go:68 | Shared routeTargetsGateway predicate |
| 98 | kinesis | claude P2 #1 CFN tags first page only | LIVE | PAGE | kinesis_related.go:51-65 | ListTagsForStream has no HasMoreTags/ExclusiveStartTagKey loop, and a missing tag gives a proven 0 |
| 99 | kinesis | claude P2 #2 Lambda pivot misses EFO consumer mappings | LIVE | MATCH | kinesis_related.go:28-34, related_common.go:377-378 | ListEventSourceMappings(EventSourceArn = stream ARN) excludes consumer-ARN mappings |
| 100 | kinesis | claude P2 #3 lambda→kinesis consumer ARN parse | FIXED | — | ref_ids.go:198 | kinesisRefToID = arnNameRef("kinesis","stream/","/") stops at the first "/" |
| 101 | kinesis | claude P2 #4 alarm empty namespace | FIXED | — | alarm_match.go:238 | AWS/Kinesis + StreamName |
| 102 | kinesis | claude P3 #5 disabled streaming destinations count | LIVE | MATCH | kinesis_related.go:158-163 (and ddb_related.go:54 checkDdbKinesis) | DestinationStatus is never read, in either file |
| 103 | kinesis | claude P3 #6 kinesis→ddb one call per table, uncapped | LIVE | OTHER (cost / contract conflict) | kinesis_related.go:135-163, catalog_messaging.go:490 | Sequential DescribeKinesisStreamingDestination per cached table. docs/resources/kinesis.md:51 says "pivot not implemented", while docs/related-resources.md:654 lists ddb |
| 104 | kinesis | claude P3 #7 label "DynamoDB Streams" | FIXED | — | catalog_messaging.go:490 | "DynamoDB Tables" |

## 2. Counts

- **Source findings in scope:** 130 (codex 35, claude 93, backlog 2), merged into 104 rows.
- **Out of scope:** 77 (codex 46, claude 31). These are list columns, findings and severity, Wave-2 posture, the `L` log view, detail rendering and the snapshot collector.
- **Rows:** 104. FIXED 54, DISPROVED 3, LIVE 47.
- **LIVE by class:** ID 3, MATCH 20, PAGE 3, CALL 1, CARD 7, NAV 4, CONTRACT 6, OTHER 3.

## 3. Shared shapes

1. **Direct `cache[target]` reads skip rows without a RawStruct and ignore `entry.FieldsOnly`.** FetchRelatedTarget treats a FieldsOnly entry as a miss (related_fetch.go:33). These checkers bypass it, so a disk-restored target cache gives a proven 0.
   - Rows: 6, 37, 49.
   - Sites: secrets_related_extra.go:80/94 (eb), secrets_related_extra.go:168/188 (ecs-task), ecs_svc_related_extra.go:131/138 (eb-rule), ecr_related.go:226/233 (eb-rule, same shape, not reported), ecs_svc_related_extra.go:337 (sfn, same shape).
2. **EventBridge patterns are matched by hand.** Both matchers unmarshal a filter as []string, so operator objects (`prefix`, `anything-but`) fail silently. A source-only rule matches every resource. Target-based discovery (ListTargetsByRule, schedule rules) is never read.
   - Rows: 24, 36.
   - Sites: ecrEbRuleMatches ecr_related.go:249-295, ecsSvcEbRuleMatches ecs_svc_related_extra.go:154-211.
3. **ECS task-definition secret and parameter references are classified by hand, twice.** The two sites read different field sets: ARN-only isSecret (parse.go:63), no LogConfiguration.SecretOptions, and SSM names rebuilt with a forced "/".
   - Rows: 30, 31, 44.
   - Sites: ecs_svc_related_extra.go:292-309, ecs_task.go:262-289, secretsECSTaskRefsSecret in secrets_related_extra.go.
4. **Membership is approximated by a shared property.** Some sites are now labelled candidate or Distinct. Others still report an exact count.
   - Rows: 15, 50, 52, 53, 72, 80, 83.
   - Candidate or Distinct: ec2→tg (ec2_related.go:52), dbi→eni (dbi_related.go:272).
   - Still an exact count: apigw→elb subnet/SG overlap (apigw_related.go:365-396), ecs→asg and ecs→ec2 tag guesses (ecs_related_extra.go:43-48, :78-86), logs→ecs-task by `/ecs/<family>` (logs_related.go:188-209), lambda↔eni single description name (lambda_related_extra.go:388-392, eni_related.go:146-150).
   - Each of these has an AWS field that states membership: DescribeTargetHealth, the listener ARN in IntegrationUri, CapacityProviders, container instances, awslogs-group, and the ENI's subnet/SG set.
5. **Resource specs and catalog registrations disagree.** Code and contract say different things in both directions.
   - Rows: 15, 18, 35, 84, 86, 103.
   - Contract asks for more than the code reads: ec2 tg and ebs-snap (Distinct vs docs/resources/ec2.md), ecs-svc role (docs/resources/ecs-svc.md:80), glue logs and secrets (docs/resources/glue.md:49, :67).
   - The two contract documents conflict: kinesis→ddb (docs/resources/kinesis.md:51 vs docs/related-resources.md:654).
   - The contract has gaps: eb OperationsRole and eks NodeRoleArn (rows 2, 69).
6. **Hand-split navigation IDs bypass `resource.ResolveRef`.** Only `role` gets a special case. The same split keeps the Secrets Manager random suffix on `secret:` TARGET rows (outside this slice). Two more emitters skip the resolver.
   - Rows: 4, 94.
   - Sites: CloudTrail TARGET rows core/semantics/ctevent/target.go:134-140. Also checkEbELB eb_related_extra.go:52-56, which emits names without resolveRefs or a list check, and ecs_task.go:274-283, which builds SSM IDs outside a resolver.
7. **`anyDegraded` turns every scan over a list with one unread row into a lower bound.** This applies even when the match reads only ID and Name.
   - Row: 71.
   - Site: related_fetch.go:40, :64. It affects every reverse scan into eks and every other type with degraded rows.
8. **Per-parent reads not paged.**
   - Rows: 19, 91, 98.
   - Sites: kinesis ListTagsForStream (kinesis_related.go:51), IAM ListGroups and ListGroupPolicies in the inline sweep (iam_policies.go:355, :395), ec2 by-ID with no batching (ec2_by_ids.go:98).
