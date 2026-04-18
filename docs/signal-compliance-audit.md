# Attention Signals Wiring Audit

## Compute

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| ec2 | running → Healthy | YES | Color func State.Name switch | |
| ec2 | system_status != ok → Broken | YES | EnrichEC2InstanceStatus populates Fields["system_status"], Color func checks | |
| ec2 | Events[] retirement ≤7d → Warning | YES | EnrichEC2InstanceStatus checks NotBeforeDeadline, emits Finding | |
| ec2 | StateReason.Code begins Server.* on stopped → Broken | PARTIAL | Color func checks (line 72 types_compute.go) but StateReason NOT fetched from API — always missing | Bug: StateReason not extracted from DescribeInstances response |
| ec2 | StateTransitionReason >30d → Warning | NO | Not fetched or enriched | Wave-1 gap |
| ecs-svc | status: ACTIVE → Healthy; DRAINING → Dim | YES | Color func Field["status"] switch | |
| ecs-svc | running_count < desired_count → Warning | YES | Color func logic (line 132 types_compute.go) | |
| ecs-svc | deployments[].rolloutState==FAILED → Broken | YES | EnrichECSServices parses deployments field, emits findings | |
| ecs-svc | unable to place / ELB health failed in events → Broken | YES | EnrichECSServices checks events array (150+ line scan) | |
| ecs-svc | circuit-breaker triggered → Broken | PARTIAL | CircuitBreaker enum in ECS API but not parsed by enricher | Implementation gap |
| ecs | status: ACTIVE → Healthy | YES | Color func State.Name switch (type_compute.go line 173) | |
| ecs | PROVISIONING/DEPROVISIONING → Warning | YES | Color func switch arm | |
| ecs | FAILED/INACTIVE → Broken | YES | Color func switch arm | |
| ecs | pending_tasks > 0 sustained → Warning | NO | No Wave-2 enricher for task counts | Wave-2 gap |
| ecs | running_tasks == 0 && registered > 0 → Warning | NO | No Wave-2 enricher for container instance check | Wave-2 gap |
| ecs-task | lastStatus: RUNNING → Healthy | YES | Color func (types_compute.go line 204) | |
| ecs-task | healthStatus == UNHEALTHY → Broken | YES | Color func override (line 200) | |
| ecs-task | STOPPED with StopCode != UserInitiated → Broken | YES | Color func logic (line 209) | |
| ecs-task | StopCode in TaskFailedToStart / EssentialContainerExited → Broken | YES | Color func checks (line 209) | |
| lambda | State: Active → Healthy | YES | Color func (types_compute.go line 235) | |
| lambda | State: Pending → Warning | YES | Color func (line 238) | |
| lambda | State: Failed → Broken | YES | Color func (line 242) | |
| lambda | LastUpdateStatus == Failed → Broken | YES | Color func override (line 249) | |
| lambda | Runtime in deprecated list → Broken | YES | Color func checks deprecatedLambdaRuntimes (line 253) | |
| lambda | DeadLetterConfig == nil → Warning | YES | Color func if dlq_target_arn == "" (line 261) | |
| asg | Status == "" → Healthy | YES | Color func (types_compute.go line 289) | |
| asg | Delete in progress → Warning | YES | Color func (line 292) | |
| asg | Instances[].HealthStatus == Unhealthy → Warning | NO | Field NOT registered; Field never populated | Wave-1 gap |
| asg | InService < MinSize → Broken | NO | Field NOT registered; Field never populated | Wave-1 gap |
| asg | SuspendedProcesses containing Launch/Terminate → Warning | NO | Field NOT registered; Field never populated | Wave-1 gap |
| asg | Latest ScalingActivity StatusCode == Failed → Broken | YES | EnrichASGScalingActivities per resource (100 cap) | |
| eb | Health: Green → Healthy | YES | Color func (types_compute.go line 243+) | |
| eb | Yellow/Grey → Warning | YES | Color func | |
| eb | Red → Broken | YES | Color func | |
| eb | Status == Terminated → Dim | YES | Color func | |
| eb | DescribeEnvironmentHealth Causes[] non-empty → Warning | YES | EnrichEBEnvironmentHealth | |
| ebs | State: in-use → Healthy | YES | Color func | |
| ebs | creating/deleting → Warning | YES | Color func | |
| ebs | error → Broken | YES | Color func | |
| ebs | available >7d → Warning (orphan) | NO | CreateTime field fetched (ebs.go line 141) but not checked | Wave-1 gap |
| ebs | Encrypted == false → Warning (CIS) | PARTIAL | Field "encrypted" fetched (ebs.go) but NO column/color rule | BUGGY: Data collected, not surfaced |
| ebs | VolumeStatus.Status != ok → Broken | YES | EnrichEBSVolumeStatus (Wave-2) | |
| ebs | VolumeStatus == warning → Warning | YES | EnrichEBSVolumeStatus | |
| ebs | Events[] non-empty → Warning | YES | EnrichEBSVolumeStatus (line 336) | |
| ebs-snap | State: completed → Healthy | YES | Color func | |
| ebs-snap | pending → Warning | YES | Color func | |
| ebs-snap | error/recoverable → Broken | YES | Color func | |
| ebs-snap | Age > 365d + automated → Warning | NO | Not computed; no age check enricher | Wave-2 gap |
| ebs-snap | Encrypted == false → Warning (CIS) | PARTIAL | Field present but not surfaced in color/column | BUGGY |
| ebs-snap | Cross-ref ebs: source deleted → Warning | NO | No enricher compares snapshot.VolumeId against loaded EBS list | Wave-2 gap |
| ami | State: available → Healthy | YES | Color func | |
| ami | pending/transient → Warning | YES | Color func | |
| ami | failed/error/invalid → Broken | YES | Color func | |
| ami | deregistered/disabled → Dim | YES | Color func | |
| ami | DeprecationTime < now() → Warning | NO | DeprecatedTime field not extracted | Wave-1 gap |
| ami | Cross-ref snapshot owner deleted → Warning | NO | No enricher cross-checks snapshot delete | Wave-2 gap |

## Containers

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| eks | status: ACTIVE → Healthy (Wave-2) | YES | Fetcher DescribeCluster per cluster populates health_issues at fetch time; Color func reads it | Pragmatic in-fetcher Wave-2 |
| eks | CREATING/UPDATING/DELETING → Warning | YES | Color func (types_containers.go) | |
| eks | FAILED → Broken | YES | Color func | |
| eks | health.issues[] non-empty → Broken | YES | Color func override | |
| ng | status: ACTIVE → Healthy (Wave-2) | YES | Fetcher DescribeNodegroup per group; Color func reads health_issues | Pragmatic in-fetcher Wave-2 |
| ng | CREATING/UPDATING/DELETING → Warning | YES | Color func | |
| ng | CREATE_FAILED/DELETE_FAILED/DEGRADED → Broken | YES | Color func | |
| ng | health.issues[] codes → Broken | YES | Color func override | |

## Networking

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| elb | State.Code: active → Healthy | YES | Color func (types_networking.go) | |
| elb | provisioning/active_impaired → Warning | YES | Color func | |
| elb | failed → Broken | YES | Color func | |
| elb | State.Reason surface as detail | PARTIAL | State.Reason fetched but only used in Findings, not in column | |
| tg | LoadBalancerArns == [] → Warning (orphan) | YES | Color func checks (types_networking.go) | |
| tg | DescribeTargetHealth: any == unhealthy → Warning | YES | EnrichTargetGroupHealth (Wave-2) | |
| tg | all unhealthy → Broken | YES | EnrichTargetGroupHealth counts | |
| vpc | State: available → Healthy | YES | Color func | |
| vpc | pending → Warning | YES | Color func | |
| vpc | Cross-ref subnet: no subnets → Warning | NO | No enricher cross-checks subnet count | Wave-2 gap |
| vpc | DescribeFlowLogs: none → Warning | YES | EnrichVPCFlowLogs | |
| subnet | State: available → Healthy | YES | Color func | |
| subnet | pending → Warning | YES | Color func | |
| subnet | unavailable/failed/failed-insufficient-capacity → Broken | YES | Color func | |
| subnet | AvailableIpAddressCount / CIDR < 0.1 → Warning | NO | Field NOT populated; no enricher checks IP availability | Wave-1 gap |
| subnet | AvailableIpAddressCount / CIDR < 0.02 → Broken | NO | Field NOT populated | Wave-1 gap |
| subnet | MapPublicIpOnLaunch without IGW route → Warning | NO | Field NOT checked; no enricher validates RTB | Wave-2 gap |
| rtb | Routes[].State == blackhole → Broken | NO | Field NOT extracted; State not checked | Wave-1 gap |
| rtb | No Associations AND not VPC main → Warning | NO | Field NOT extracted | Wave-1 gap |
| nat | State: available → Healthy | YES | Color func | |
| nat | pending/deleting → Warning | YES | Color func | |
| nat | failed → Broken | YES | Color func | |
| nat | FailureCode non-empty → Broken detail | NO | FailureCode/FailureMessage not extracted | Wave-1 gap |
| igw | Attachments[].State: attached → Healthy | YES | Color func | |
| igw | attaching/detaching → Warning | YES | Color func | |
| igw | detached / len(Attachments) == 0 → Warning (orphan) | YES | Color func | |
| igw | Attached to VPC with no IGW route → Warning (unused) | NO | No enricher cross-checks RTB for route | Wave-2 gap |
| eip | AssociationId && InstanceId && NetworkInterfaceId absent → Warning (unattached) | YES | Color func (types_networking.go) | |
| eip | Attached to stopped instance → Warning (zombie) | NO | No enricher cross-checks EC2 state | Wave-2 gap |
| vpce | State: Available → Healthy | YES | Color func | |
| vpce | PendingAcceptance/Pending/Deleting → Warning | YES | Color func | |
| vpce | Failed/Rejected/Expired → Broken | YES | Color func | |
| vpce | Partial (interface ENI failed) → Broken | YES | Color func | |
| vpce | Deleted → Dim | YES | Color func | |
| vpce | LastError non-empty → Broken detail | PARTIAL | LastError field populated but not forced to Broken | BUGGY |
| vpce | Interface NetworkInterfaceIds == [] → Broken | NO | Field NOT checked in Color func | Wave-1 gap |
| vpce | Gateway RouteTableIds == [] → Warning (orphan) | NO | Field NOT checked | Wave-1 gap |
| tgw | State: available → Healthy | YES | Color func | |
| tgw | pending/modifying/deleting → Warning | YES | Color func | |
| tgw | deleted → Dim | YES | Color func | |
| tgw | DescribeTransitGatewayAttachments: failed/failing/rejected → Broken | YES | EnrichTGWAttachments | |
| tgw | pendingAcceptance > 24h → Warning | NO | EnrichTGWAttachments time calc NOT implemented | Wave-2 gap |
| eni | Status: in-use/associated → Healthy | YES | Color func | |
| eni | attaching/detaching → Warning | YES | Color func | |
| eni | available → Warning (orphan) | YES | Color func | |
| eni | Requester-managed with zombie desc → Warning | NO | Field NOT extracted | Wave-1 gap |
| sg | IpPermissions[]: 0.0.0.0/0 on admin/db ports → Broken | YES | Color func checks dangerous_open_count (set in sg.go fetcher) | |
| sg | Not referenced by any ENI → Warning (orphan) | NO | No enricher cross-checks ENI references | Wave-2 gap |

## Databases & Storage

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| dbi | DBInstanceStatus: available → Healthy | YES | Color func | |
| dbi | transitional → Warning | YES | Color func | |
| dbi | failed/storage-full/incompatible-*/restore-error → Broken | YES | Color func | |
| dbi | BackupRetentionPeriod == 0 → Warning | NO | Field not extracted/checked | Wave-1 gap |
| dbi | PubliclyAccessible == true → Warning (CIS) | NO | Field not extracted/checked | Wave-1 gap |
| dbi | StorageEncrypted == false → Warning (CIS) | NO | Field not extracted/checked | Wave-1 gap |
| dbi | DeletionProtection == false → Warning | NO | Field not extracted/checked | Wave-1 gap |
| dbi | DescribePendingMaintenanceActions: ForcedApplyDate/AutoAppliedAfterDate in past → Warning | YES | EnrichRDSDocDBMaintenance (batchable, 1 call) | |
| dbc | Status: available → Healthy | YES | Color func | |
| dbc | transitional → Warning | YES | Color func | |
| dbc | failed/inaccessible-encryption/incompatible → Broken | YES | Color func | |
| dbc | No DBClusterMembers with IsClusterWriter == true → Broken | NO | Field not extracted | Wave-1 gap |
| dbc | DeletionProtection == false → Warning | NO | Field not extracted | Wave-1 gap |
| dbc | StorageEncrypted == false → Warning | NO | Field not extracted | Wave-1 gap |
| dbc | BackupRetentionPeriod == 0 → Warning | NO | Field not extracted | Wave-1 gap |
| dbc | DescribePendingMaintenanceActions (shared with dbi) | YES | EnrichRDSDocDBMaintenance | |
| redis | Status: available → Healthy | YES | Color func | |
| redis | creating/modifying/deleting/snapshotting → Warning | YES | Color func | |
| redis | create-failed → Broken | YES | Color func | |
| redis | AutomaticFailover != enabled on multi-AZ → Warning | PARTIAL | Field extracted (redis.go) but NOT checked in Color func | BUGGY |
| ddb | TableStatus: ACTIVE → Healthy (Wave-2 via DescribeTable) | YES | Fetcher already does per-table DescribeTable call; Color func reads status | Pragmatic in-fetcher Wave-2 |
| ddb | CREATING/UPDATING/DELETING/ARCHIVING → Warning | YES | Color func | |
| ddb | INACCESSIBLE_ENCRYPTION_CREDENTIALS/ARCHIVED → Broken | YES | Color func | |
| ddb | PITR disabled (DescribeContinuousBackups) → Warning | NO | EnrichDynamoDBPITR registered but check not implemented | Partial: enricher called but finding logic missing |
| opensearch | Deleted == true → Dim (Wave-2 via DescribeDomains) | YES | Fetcher already calls DescribeDomains; Color func reads Deleted | Pragmatic in-fetcher Wave-2 |
| opensearch | Processing/UpgradeProcessing → Warning | YES | Color func | |
| opensearch | DomainProcessingStatus == Isolated → Broken | YES | Color func | |
| opensearch | ServiceSoftwareOptions.UpdateAvailable with date in past → Warning | PARTIAL | UpdateAvailable field extracted but date not checked in Color func | BUGGY |
| opensearch | EncryptionAtRestOptions.Enabled == false → Warning | NO | Field not checked | Wave-1 gap |
| redshift | ClusterStatus: available → Healthy | YES | Color func | |
| redshift | creating/modifying/resizing/rebooting → Warning | YES | Color func | |
| redshift | incompatible-*/hardware-failure/storage-full → Broken | YES | Color func | |
| redshift | ClusterAvailabilityStatus: Unavailable/Failed → Broken | NO | Field not extracted | Wave-1 gap |
| redshift | Maintenance/Modifying → Warning | NO | Field not extracted | Wave-1 gap |
| redshift | PendingModifiedValues non-empty → Warning | NO | Field not extracted | Wave-1 gap |
| redshift | DeferredMaintenanceWindows[] active → Warning | NO | Field not extracted | Wave-1 gap |
| redshift | PubliclyAccessible == true → Warning | NO | Field not extracted | Wave-1 gap |
| redshift | Encrypted == false → Warning | NO | Field not extracted | Wave-1 gap |
| efs | LifeCycleState: available → Healthy | YES | Color func | |
| efs | creating/updating/deleting → Warning | YES | Color func | |
| efs | error → Broken | YES | Color func | |
| efs | NumberOfMountTargets == 0 → Broken | NO | Field not extracted/checked | Wave-1 gap |
| efs | DescribeMountTargets: any LifeCycleState != available → Broken | YES | EnrichEFSMountTargets | |
| s3 | GetPublicAccessBlock: NoSuchConfig or any flag false → Warning | YES | EnrichS3PublicAccessBlock (Wave-2) | |
| rds-snap | Status: available → Healthy | YES | Color func | |
| rds-snap | creating → Warning | YES | Color func | |
| rds-snap | failed/incompatible-* → Broken | YES | Color func | |
| rds-snap | Encrypted == false → Warning (CIS) | NO | Field not extracted | Wave-1 gap |
| rds-snap | Cross-ref dbi: source deleted → Warning | NO | No enricher compares SourceDBInstanceIdentifier | Wave-2 gap |
| rds-snap | Age > BackupRetentionPeriod && automated → Warning | NO | No enricher computes age vs retention | Wave-2 gap |
| docdb-snap | Status: available → Healthy | YES | Color func | |
| docdb-snap | creating → Warning | YES | Color func | |
| docdb-snap | failed → Broken | YES | Color func | |
| docdb-snap | Manual age > 365d → Warning | NO | Field not extracted/computed | Wave-1 gap |
| docdb-snap | Age > cluster retention × 1.5 && automated → Warning | NO | No enricher cross-checks cluster age | Wave-2 gap |

## Messaging

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| sqs | GetQueueAttributes(All): ApproximateNumberOfMessages > threshold → Warning | YES | EnrichSQSAttributes (Wave-2) | |
| sqs | Rising unbounded → Broken | PARTIAL | Logic to detect trend NOT implemented | Wave-2 gap |
| sqs | ApproximateAgeOfOldestMessage > VisibilityTimeout×5 → Warning | YES | EnrichSQSAttributes | |
| sqs | Is-DLQ with messages → Warning | YES | EnrichSQSAttributes | |
| sqs | RedrivePolicy unset on main → Warning | YES | EnrichSQSAttributes | |
| sns | GetTopicAttributes: SubscriptionsConfirmed == 0 AND Pending == 0 → Warning | YES | EnrichSNSSubscriptions (Wave-2) | |
| sns | KmsMasterKeyId absent on sensitive topic → Warning | NO | No enricher checks encryption | Wave-2 gap |
| sns-sub | SubscriptionArn == "PendingConfirmation" → Warning | YES | Color func checks status | |
| eb-rule | State: ENABLED → Healthy | YES | Color func | |
| eb-rule | ENABLED_WITH_ALL_CLOUDTRAIL → Healthy | YES | Color func | |
| eb-rule | DISABLED → Dim | YES | Color func | |
| eb-rule | ListTargetsByRule: ENABLED AND len(Targets) == 0 → Broken | YES | EnrichEventBridgeRuleTargets (Wave-2) | |
| eb-rule | DISABLED AND len(Targets) > 0 → Warning | YES | EnrichEventBridgeRuleTargets | |
| eb-rule | Any target without DeadLetterConfig → Warning | YES | EnrichEventBridgeRuleTargets | |
| kinesis | StreamStatus: ACTIVE → Healthy | YES | Color func | |
| kinesis | CREATING/UPDATING/DELETING → Warning | YES | Color func | |
| msk | State: ACTIVE → Healthy | YES | Color func | |
| msk | CREATING/UPDATING/MAINTENANCE/REBOOTING_BROKER → Warning | YES | Color func | |
| msk | DELETING → Dim | YES | Color func | |
| msk | FAILED → Broken | YES | Color func | |
| sfn | ListExecutions(statusFilter=FAILED, maxRecords=1): any recent → Warning | YES | EnrichStepFunctionsStatus (Wave-2) | |
| sfn | Consecutive failures → Broken | PARTIAL | Logic implemented but threshold not clear | Wave-2 gap |

## Secrets & Config

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| secrets | RotationEnabled && now > NextRotationDate → Warning | NO | Fields not extracted | Wave-1 gap |
| secrets | RotationEnabled && (now - LastRotatedDate) > AutomaticallyAfterDays × 2 → Broken | NO | Fields not extracted | Wave-1 gap |
| secrets | LastAccessedDate > 180d → Warning | NO | Field not extracted | Wave-1 gap |
| secrets | DeletedDate set → Warning | NO | Field not extracted | Wave-1 gap |
| ssm | Type == SecureString && LastModifiedDate > 365d → Warning | NO | Field not extracted | Wave-1 gap |
| ssm | Type == String && name suffix -password/-secret/-token → Warning | NO | Field not extracted | Wave-1 gap |
| ssm | Tier == Advanced unused > 90d → Warning | NO | Field not extracted | Wave-1 gap |
| kms | DescribeKey: KeyState == Enabled → Healthy (Wave-2) | YES | Fetcher already calls DescribeKey; Color func reads KeyState | Pragmatic in-fetcher Wave-2 |
| kms | Creating/Updating → Warning | YES | Color func | |
| kms | Disabled → Warning | YES | Color func | |
| kms | PendingDeletion/PendingImport/PendingReplicaDeletion → Broken | YES | Color func | |
| kms | Unavailable → Broken | YES | Color func | |
| kms | GetKeyRotationStatus: Enabled == false on CMK → Warning | YES | EnrichKMSRotation (Wave-2) | |

## Security & IAM

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| role | AssumeRolePolicyDocument: Principal:{"AWS":"*"} without external-id → Broken | YES | Color func (types_security.go) checks TrustRelationshipBroken | |
| role | GetRole: RoleLastUsed > 90d missing → Warning | YES | EnrichIAMRoleLastUsed (Wave-2) | |
| policy | AttachmentCount == 0 AND not AWS-managed → Warning | YES | Color func checks orphan_policy | |
| policy | GetPolicyVersion: wildcard admin → Broken | YES | EnrichIAMPolicy (Wave-2) | |
| iam-user | PasswordLastUsed absent AND CreateDate > 90d → Warning | YES | Color func (types_security.go) | |
| iam-user | ListAccessKeys: Active with LastUsedDate > 90d → Warning | YES | EnrichIAMUserMFA (Wave-2) | |
| iam-user | Active with LastUsedDate == N/A AND CreateDate > 90d → Warning | YES | EnrichIAMUserMFA | |
| iam-user | Console enabled AND no MFA → Broken | YES | EnrichIAMUserMFA checks mfa_device | |
| iam-group | GetGroup: Users == [] AND age > 30d → Warning | YES | EnrichIAMGroup (Wave-2) | |
| waf | GetWebACL: Rules == [] → Warning | YES | EnrichWAFLogging (Wave-2) | |
| waf | DefaultAction == Allow with zero rules → Broken | YES | EnrichWAFLogging | |

## DNS, CDN, Certs

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| r53 | ResourceRecordSetCount <= 2 on non-new → Warning | YES | EnrichRoute53Zone checks record count | |
| r53 | GetDNSSEC: SIGNING with KSK inactive → Broken | NO | No enricher calls GetDNSSEC | Wave-2 gap |
| cf | Status: Deployed → Healthy | YES | Color func | |
| cf | InProgress → Warning | YES | Color func | |
| cf | Enabled == false → Dim | YES | Color func | |
| cf | MinimumProtocolVersion in SSLv3/TLSv1/TLSv1_2016/TLSv1.1_2016 → Warning | YES | Color func checks old TLS | |
| cf | WebACLId == "" → Warning (no WAF) | YES | Color func checks waf_enabled | |
| cf | GetDistributionConfig: Logging.Enabled == false → Warning | YES | EnrichCloudFrontDistribution (Wave-2) | |
| acm | Status: ISSUED → Healthy | YES | Color func | |
| acm | PENDING_VALIDATION → Warning | YES | Color func | |
| acm | EXPIRED/REVOKED/FAILED/VALIDATION_TIMED_OUT → Broken | YES | Color func | |
| acm | INACTIVE → Dim | YES | Color func | |
| acm | NotAfter - now() < 30d → Warning | YES | Color func checks days_to_expiry | |
| acm | NotAfter - now() < 7d → Broken | YES | Color func checks days_to_expiry | |
| acm | InUse == false on non-expired → Warning (orphan) | PARTIAL | Field extracted but not forced to Warning in Color func | BUGGY |
| acm | DescribeCertificate: RenewalStatus == FAILED → Broken | YES | EnrichACMCertificate (Wave-2) | |
| acm | DomainValidationOptions[].ValidationStatus == FAILED → Broken | YES | EnrichACMCertificate | |
| apigw | GetStages: no deployed stage → Warning (orphan) | YES | EnrichAPIGatewayStage (Wave-2) | |

## Monitoring

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| alarm | StateValue: OK → Healthy | YES | Color func | |
| alarm | INSUFFICIENT_DATA → Warning | YES | Color func | |
| alarm | ALARM → Broken | YES | Color func | |
| alarm | ActionsEnabled == false → Warning (muted) | YES | Color func checks actions_enabled | |
| alarm | AlarmActions == [] → Warning (nowhere) | YES | Color func checks has_actions | |
| alarm | INSUFFICIENT_DATA > 2×Period old → Broken (dead metric) | YES | Color func time check | |
| alarm | Cross-ref siblings: resource absent → Warning (zombie) | PARTIAL | Logic implemented but only when sibling list loaded | Wave-2 gap |
| logs | retentionInDays == nil → Warning (cost) | YES | Color func checks retention_days | |
| logs | storedBytes == 0 && creationTime < now()-90d → Warning (orphan) | YES | Color func checks storage_bytes && age | |
| logs | Cross-ref kms: referenced key in PendingDeletion → Broken | NO | No enricher cross-checks KMS key deletion | Wave-2 gap |
| logs | DescribeLogStreams: lastEventTimestamp stale → Warning | YES | EnrichLogsMetricFilters (Wave-2) | |
| trail | LogFileValidationEnabled == false → Warning | YES | Color func (in fetcher) | |
| trail | GetTrailStatus: IsLogging == false → Broken | YES | Trail fetcher calls GetTrailStatus; Color func checks is_logging | Pragmatic in-fetcher Wave-2 |
| trail | LatestDeliveryError non-empty → Broken | YES | Color func checks delivery_error | |
| trail | LatestDeliveryTime > 1h ago on IsLogging == true → Broken | YES | Color func time check | |
| ct-events | errorCode present in parsed Event → Warning | NO | ct-events is read-only event listing; parsing not in scope | Out of scope (Wave-3 adjacent) |

## CI/CD

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| cfn | StackStatus: CREATE_COMPLETE/UPDATE_COMPLETE → Healthy | YES | Color func | |
| cfn | *_IN_PROGRESS/REVIEW_IN_PROGRESS → Warning | YES | Color func | |
| cfn | ROLLBACK_COMPLETE → Warning (failed create) | YES | Color func | |
| cfn | UPDATE_ROLLBACK_COMPLETE/IMPORT_ROLLBACK_COMPLETE → Warning | YES | Color func | |
| cfn | *_FAILED → Broken | YES | Color func | |
| cfn | IN_PROGRESS > 1h → Broken (stuck) | YES | Color func time check | |
| cfn | DriftInformation.StackDriftStatus == DRIFTED → Warning | YES | Color func | |
| cfn | DescribeStackEvents: recent *_FAILED → Broken | YES | EnrichCFNCombined (Wave-2) | |
| pipeline | GetPipelineState: any stage in Failed/Stopped/Cancelled → Broken | YES | EnrichCodePipelineStatus (Wave-2) | |
| pipeline | Stage InProgress > 2h → Warning | YES | EnrichCodePipelineStatus | |
| cb | Latest build status FAILED/FAULT/TIMED_OUT → Broken | YES | EnrichCodeBuildStatus (Wave-2) | |
| ecr | imageScanningConfiguration.scanOnPush == false → Warning | YES | Color func | |
| ecr | DescribeImages latest: findingSeverityCounts.CRITICAL > 0 → Broken | YES | EnrichECRRepository (Wave-2) | |
| ecr | HIGH > 0 → Warning | YES | EnrichECRRepository | |
| codeartifact | ListPackages(maxRecords=1): empty repo age > 30d → Warning | YES | EnrichCodeArtifactRepository (Wave-2) | |

## Data & Analytics

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| glue | GetJobRuns(maxRecords=1): latest in FAILED/TIMEOUT/ERROR/EXPIRED → Broken | YES | EnrichGlueJobStatus (Wave-2, batchable) | |
| athena | State: ENABLED → Healthy | YES | Color func | |
| athena | DISABLED → Warning (admin off) | YES | Color func | |
| athena | GetWorkGroup: EnforceWorkGroupConfiguration == false AND EncryptionConfiguration == nil → Warning | YES | EnrichAthenaWorkGroup (Wave-2) | |
| athena | BytesScannedCutoffPerQuery unset → Warning (cost) | YES | EnrichAthenaWorkGroup | |

## Backup & Email

| shortName | Signal (golden contract) | Wired? | Where | Notes |
|---|---|---|---|---|
| backup | ListBackupJobs(ByCreatedAfter=now-24h): any FAILED/EXPIRED/ABORTED → Broken | YES | EnrichBackupJobs (Wave-2) | |
| backup | PARTIAL → Warning | YES | EnrichBackupJobs | |
| ses | VerificationStatus: Success → Healthy | YES | Color func | |
| ses | Pending → Warning | YES | Color func | |
| ses | Failed/TemporaryFailure/NotStarted → Broken | YES | Color func | |
| ses | SendingEnabled == false → Warning | YES | Color func | |
| ses | GetAccount (SESv2): EnforcementStatus in PROBATION/SHUTDOWN → Broken | YES | EnrichSESAccount (Wave-2) | |
| ses | SentLast24Hours > 0.8 × Max24HourSend → Warning | YES | EnrichSESAccount | |

## Top Gaps (by user impact)

### Critical (security/data loss risk)

1. **EBS: Encrypted == false (CIS EC2.7)** — Field collected in fetcher but NOT exposed in list column or color; users have encrypted disk state but cannot see it. HIGH-VALUE fix: 1 line Color check + column def.

2. **RDS/Aurora: PubliclyAccessible, StorageEncrypted, DeletionProtection == false (CIS)** — Zero wiring for 3 major security controls. Fields not extracted from API response. REQUIRES: Fetcher extraction + Wave-1 color logic.

3. **Redshift: PubliclyAccessible, Encrypted, ClusterAvailabilityStatus** — 5 Wave-1 signals completely unwired. REQUIRES: Fetcher extraction + Color logic.

4. **ASG: Instances[].HealthStatus, SuspendedProcesses, InService < MinSize** — Wave-1 gaps prevent visibility into broken ASGs. Users see "active" status when ASG is degraded (unhealthy instances, launch suspended). REQUIRES: Fetcher field extraction + Color enhancements.

5. **VPC/Subnet: AvailableIpAddressCount < threshold** — IP exhaustion visibility completely missing. Users cannot see subnets at risk of allocation failure. REQUIRES: Fetcher extraction + Wave-1 color check.

6. **NAT Gateway, Route Table: Blackhole routes, orphan state** — Dead routing not surfaced. REQUIRES: Fetcher extraction + Color logic.

### High (operational/cost visibility)

7. **EBS Snapshot: Age > 365d (cost drift)** — Manual snapshots accumulating indefinitely. No age computation enricher. REQUIRES: EnrichEBSSnapshotAge (Wave-2) with 1-call DescribeSnapshots (already have VolumeIds).

8. **S3/EFS: No cross-ref enrichers** — Orphan buckets, orphan file systems, orphan volumes undetected when source deleted. REQUIRES: 3× Wave-2 enrichers (1 call each, batch-compare against target list).

9. **DynamoDB/OpenSearch: UpdateAvailable field extracted but not forced to Warning/Broken** — Updates pending but color not applied. BUGGY wiring. REQUIRES: 2× Color func additions.

10. **SecretsManager + SSM Parameter: Rotation/encryption state** — 4 Wave-1 secrets controls and 3× SSM controls completely unimplemented. REQUIRES: Fetcher extraction + Wave-1 color logic for each.

