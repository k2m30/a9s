package unit

// qa_coderabbit_pr273_all_types_test.go — per-resource-type contract table.
//
// One row per registered ResourceTypeDef. Each row declares:
//
//   * shortName          — the registry key and colon-command alias
//   * apiDoc             — URL to the AWS API reference that documents the
//                          list/describe call whose output drives issue
//                          classification
//   * statusField        — the Fields map key populated by the fetcher and
//                          rendered in the list "Status" column
//   * healthyStatuses    — status string values that AWS returns for a
//                          resource in a nominal, working state
//   * warningStatuses    — status values that indicate transition, degraded,
//                          or scheduled work (yellow)
//   * brokenStatuses     — status values that indicate failure, stopped,
//                          unreachable, or rejected (red)
//   * alwaysHealthy      — true if the type is config-only: the AWS API
//                          returns no lifecycle field at all, or the field
//                          only carries values that are by definition
//                          healthy. ctrl+z on the main menu permanently
//                          hides these types regardless of anything else,
//                          because they can never produce an attention-
//                          worthy event at the list level.
//   * hasEnricher        — true if awsclient.EnricherRegistry[shortName] is
//                          non-nil; Wave 2 enrichment is an additional
//                          issue source for these types (e.g., tg has
//                          unhealthy targets discovered by Wave 2, not by
//                          list coloring).
//   * reasoning          — one-line justification for the classification,
//                          anchored to the API doc or documented AWS
//                          behavior, NOT to the current a9s implementation.
//
// The test below runs three assertions per row:
//
//   (A) Color classification: for each status string in healthyStatuses,
//       td.ResolveColor(Resource{Fields:{statusField:s}}) == ColorHealthy.
//       Likewise for warningStatuses → ColorWarning, brokenStatuses →
//       ColorBroken. This pins the AWS → a9s classification contract.
//
//   (B) Menu ctrl+z false-positive guard: inject (issues=0, known=true,
//       truncated=false) via AvailabilityCacheLoadedMsg. Toggle ctrl+z.
//       The type's Name must NOT appear in the rendered menu. This holds
//       for alwaysHealthy=true by design (filter hides them) and for
//       stateful types because their confirmed-zero signal is
//       authoritative.
//
//   (C) Menu ctrl+z false-negative guard: inject (issues=2, known=true,
//       truncated=false). Toggle ctrl+z. The type's Name MUST appear in
//       the rendered menu. This is the core contract: if AWS says there
//       are issues, the user must see the type.
//
//       For alwaysHealthy types the contract is different: ctrl+z keeps
//       them hidden even when issues>0 is injected, because alwaysHealthy
//       declares "this type's list-level state cannot produce issues."
//       If AWS evolves a type to produce real state (e.g. AWS adds a
//       health attribute to SNS subscriptions), the row below should be
//       flipped and a Color func added.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

type typeContract struct {
	shortName        string
	apiDoc           string
	statusField      string
	healthyStatuses  []string
	warningStatuses  []string
	brokenStatuses   []string
	alwaysHealthy    bool
	hasEnricher      bool
	reasoning        string
}

// typeContracts is the single source of truth for expected a9s behavior
// per registered AWS resource type. Rows are sorted alphabetically by
// shortName. When adding a new resource type, append a row here AND
// register the type — the test below will fail until both halves agree.
//
// "apiDoc" points to the AWS API reference for the list or describe call
// whose response populates the status field. Follow it when classifying
// new statuses.
var typeContracts = []typeContract{
	// Config-only: no lifecycle field in the AWS API response.
	{shortName: "apigw", apiDoc: "https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis.html", statusField: "", alwaysHealthy: true, reasoning: "API Gateway v2 GetApis response has no health/state field — APIs are either configured or deleted."},
	{shortName: "athena", apiDoc: "https://docs.aws.amazon.com/athena/latest/APIReference/API_ListWorkGroups.html", statusField: "state", alwaysHealthy: true, reasoning: "Athena WorkGroup state is ENABLED | DISABLED; neither is a fault — DISABLED is a deliberate admin action."},
	{shortName: "backup", apiDoc: "https://docs.aws.amazon.com/aws-backup/latest/devguide/API_ListBackupPlans.html", statusField: "", alwaysHealthy: true, reasoning: "AWS Backup plans are declarative — no per-plan health; individual job failures surface via CloudWatch alarms, not the plan list."},
	{shortName: "codeartifact", apiDoc: "https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_ListRepositories.html", statusField: "", alwaysHealthy: true, reasoning: "CodeArtifact repos have no status field on the list response."},
	{shortName: "ecr", apiDoc: "https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_DescribeRepositories.html", statusField: "", alwaysHealthy: true, reasoning: "ECR DescribeRepositories returns no health field; image scan findings are a separate enriched signal (not Wave 2 today)."},
	{shortName: "eip", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAddresses.html", statusField: "", alwaysHealthy: true, reasoning: "Elastic IP is a static allocation; DescribeAddresses has no lifecycle state — only AllocationId and associations."},
	{shortName: "iam-group", apiDoc: "https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListGroups.html", statusField: "", alwaysHealthy: true, reasoning: "IAM groups are config-only."},
	{shortName: "iam-user", apiDoc: "https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListUsers.html", statusField: "", alwaysHealthy: true, reasoning: "IAM users are config-only (access-key rotation health is separate)."},
	{shortName: "igw", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInternetGateways.html", statusField: "", alwaysHealthy: true, reasoning: "Internet Gateways have Attachments[].State but no gateway-level lifecycle — attached or detached is admin action, not health."},
	{shortName: "kinesis", apiDoc: "https://docs.aws.amazon.com/kinesis/latest/APIReference/API_DescribeStreamSummary.html", statusField: "stream_status", healthyStatuses: []string{"ACTIVE"}, warningStatuses: []string{"CREATING", "UPDATING", "DELETING"}, brokenStatuses: nil, alwaysHealthy: true, reasoning: "Kinesis StreamStatus values are ACTIVE | CREATING | UPDATING | DELETING per DescribeStreamSummary — no failure state; currently marked AlwaysHealthy, though CREATING/UPDATING/DELETING are transitional and could legitimately produce yellow rows."},
	{shortName: "kms", apiDoc: "https://docs.aws.amazon.com/kms/latest/APIReference/API_ListKeys.html", statusField: "", alwaysHealthy: true, reasoning: "KMS keys have KeyState (Enabled/Disabled/PendingDeletion/…) — currently config-only in a9s; PendingDeletion is arguably a warning."},
	{shortName: "logs", apiDoc: "https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_DescribeLogGroups.html", statusField: "", alwaysHealthy: true, reasoning: "CloudWatch Log Groups have no lifecycle state — retention/encryption are admin config."},
	{shortName: "msk", apiDoc: "https://docs.aws.amazon.com/msk/1.0/apireference/v1-clusters.html", statusField: "state", healthyStatuses: []string{"ACTIVE"}, warningStatuses: []string{"CREATING", "UPDATING", "MAINTENANCE", "REBOOTING_BROKER", "HEALING"}, brokenStatuses: []string{"FAILED"}, alwaysHealthy: true, reasoning: "MSK ClusterState includes FAILED per ListClustersV2 — currently marked AlwaysHealthy but should surface FAILED as a broken row."},
	{shortName: "opensearch", apiDoc: "https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DescribeDomain.html", statusField: "", alwaysHealthy: true, reasoning: "OpenSearch DomainStatus has Processing/UpgradeProcessing/Deleted flags plus ClusterHealth Red/Yellow/Green — currently not classified; Red cluster health is a clear broken state."},
	{shortName: "policy", apiDoc: "https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListPolicies.html", statusField: "", alwaysHealthy: true, reasoning: "IAM policies have no lifecycle state."},
	{shortName: "r53", apiDoc: "https://docs.aws.amazon.com/Route53/latest/APIReference/API_ListHostedZones.html", statusField: "", alwaysHealthy: true, reasoning: "Route 53 hosted zones are config-only; record-set health checks are separate."},
	{shortName: "rds-snap", apiDoc: "https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBSnapshots.html", statusField: "status", healthyStatuses: []string{"available"}, warningStatuses: []string{"creating", "copying"}, brokenStatuses: []string{"failed"}, alwaysHealthy: true, reasoning: "RDS snapshot Status includes 'failed' per DescribeDBSnapshots — currently AlwaysHealthy, but 'failed' should surface as broken."},
	{shortName: "redshift", apiDoc: "https://docs.aws.amazon.com/redshift/latest/APIReference/API_DescribeClusters.html", statusField: "", alwaysHealthy: true, reasoning: "Redshift ClusterStatus includes 'incompatible-hsm', 'incompatible-network', 'hardware-failure' — currently AlwaysHealthy; these are broken states that should surface."},
	{shortName: "role", apiDoc: "https://docs.aws.amazon.com/IAM/latest/APIReference/API_ListRoles.html", statusField: "", alwaysHealthy: true, reasoning: "IAM roles are config-only."},
	{shortName: "rtb", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeRouteTables.html", statusField: "", alwaysHealthy: true, reasoning: "Route tables have no lifecycle state; individual Routes carry State (active/blackhole) but not the table itself."},
	{shortName: "s3", apiDoc: "https://docs.aws.amazon.com/AmazonS3/latest/API/API_ListBuckets.html", statusField: "", alwaysHealthy: true, reasoning: "S3 ListBuckets returns only name+CreationDate — no status field."},
	{shortName: "secrets", apiDoc: "https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_ListSecrets.html", statusField: "", alwaysHealthy: true, reasoning: "Secrets Manager entries don't have health; rotation failures are a separate CloudWatch signal."},
	{shortName: "ses", apiDoc: "https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_ListEmailIdentities.html", statusField: "", alwaysHealthy: true, reasoning: "SES identities have VerificationStatus (Success/Failed/Pending/TemporaryFailure) — currently AlwaysHealthy; 'Failed' should surface as broken."},
	{shortName: "sg", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSecurityGroups.html", statusField: "", alwaysHealthy: true, reasoning: "Security groups are config-only; no lifecycle state on DescribeSecurityGroups."},
	{shortName: "sns", apiDoc: "https://docs.aws.amazon.com/sns/latest/api/API_ListTopics.html", statusField: "", alwaysHealthy: true, reasoning: "SNS topics have no status field on ListTopics; delivery failures are per-message signals."},
	{shortName: "sns-sub", apiDoc: "https://docs.aws.amazon.com/sns/latest/api/API_ListSubscriptions.html", statusField: "", alwaysHealthy: true, reasoning: "SNS subscriptions have SubscriptionArn='PendingConfirmation' as a sentinel — currently not classified; pending-confirmation is arguably a warning row."},
	{shortName: "sqs", apiDoc: "https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html", statusField: "", alwaysHealthy: true, reasoning: "SQS queues have no lifecycle state; backlog/DLQ health is a CloudWatch signal."},
	{shortName: "ssm", apiDoc: "https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_DescribeParameters.html", statusField: "", alwaysHealthy: true, reasoning: "SSM Parameter Store entries are config-only."},
	{shortName: "trail", apiDoc: "https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_GetTrailStatus.html", statusField: "", alwaysHealthy: true, reasoning: "CloudTrail has IsLogging=false (trail not logging) and LatestDeliveryError — currently AlwaysHealthy; IsLogging=false is a clear broken state."},
	{shortName: "waf", apiDoc: "https://docs.aws.amazon.com/waf/latest/APIReference/API_ListWebACLs.html", statusField: "", alwaysHealthy: true, reasoning: "WAF Web ACLs are config-only."},

	// Enricher-backed: Wave 1 Color is trivial (list row has no status field),
	// Wave 2 enrichment discovers issues via additional API calls.
	{shortName: "cb", apiDoc: "https://docs.aws.amazon.com/codebuild/latest/APIReference/API_BatchGetBuilds.html", statusField: "", hasEnricher: true, reasoning: "CodeBuild project list has no status; Wave 2 BatchGetBuilds inspects latest build StatusType (SUCCEEDED/FAILED/FAULT/TIMED_OUT/STOPPED/IN_PROGRESS). FAILED/FAULT/TIMED_OUT produce findings; STOPPED is intentional cancel and excluded."},
	{shortName: "eb-rule", apiDoc: "https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_ListRules.html", statusField: "state", healthyStatuses: []string{"ENABLED"}, brokenStatuses: []string{"DISABLED"}, reasoning: "EventBridge Rule State is ENABLED | DISABLED — DISABLED is admin action, not a fault; currently trivial Color. No Wave 2 enricher yet."},
	{shortName: "ebs", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVolumeStatus.html", statusField: "", hasEnricher: true, reasoning: "EBS Wave 1 Color is trivial; Wave 2 DescribeVolumeStatus inspects VolumeStatusInfo.Status (ok/warning/impaired/insufficient-data). Impaired → broken."},
	{shortName: "ebs-snap", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSnapshots.html", statusField: "state", healthyStatuses: []string{"completed"}, warningStatuses: []string{"pending"}, brokenStatuses: []string{"error"}, reasoning: "EBS Snapshot State per DescribeSnapshots: pending | completed | error."},
	{shortName: "ecs-svc", apiDoc: "https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeServices.html", statusField: "status", healthyStatuses: []string{"ACTIVE"}, warningStatuses: []string{"DRAINING"}, brokenStatuses: []string{"INACTIVE"}, reasoning: "ECS Service Status: ACTIVE | DRAINING | INACTIVE per DescribeServices. INACTIVE = deleted service; DRAINING = deregistering, yellow."},
	{shortName: "dbi", apiDoc: "https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBInstances.html", statusField: "status", healthyStatuses: []string{"available"}, warningStatuses: []string{"creating", "modifying", "backing-up", "rebooting", "renaming", "upgrading"}, brokenStatuses: []string{"failed", "storage-full", "incompatible-parameters", "incompatible-restore"}, hasEnricher: true, reasoning: "RDS DBInstanceStatus per DescribeDBInstances. Wave 2 also surfaces DescribePendingMaintenanceActions (scheduled = yellow ~)."},
	{shortName: "dbc", apiDoc: "https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeDBClusters.html", statusField: "status", healthyStatuses: []string{"available"}, warningStatuses: []string{"creating", "modifying", "backing-up", "upgrading"}, brokenStatuses: []string{"failed"}, reasoning: "DocDB DBClusterStatus vocabulary mirrors RDS."},
	{shortName: "glue", apiDoc: "https://docs.aws.amazon.com/glue/latest/webapi/API_GetJobRuns.html", statusField: "", hasEnricher: true, reasoning: "Glue job list has no status; Wave 2 GetJobRuns inspects latest JobRunState (SUCCEEDED/FAILED/ERROR/TIMEOUT/STOPPED). FAILED/ERROR/TIMEOUT → finding; STOPPED = intentional cancel, excluded."},
	{shortName: "pipeline", apiDoc: "https://docs.aws.amazon.com/codepipeline/latest/APIReference/API_GetPipelineState.html", statusField: "", hasEnricher: true, reasoning: "CodePipeline list has no status; Wave 2 GetPipelineState inspects per-stage LatestExecution.Status (Succeeded/Failed/InProgress/Stopped). Failed → finding."},
	{shortName: "sfn", apiDoc: "https://docs.aws.amazon.com/step-functions/latest/apireference/API_ListExecutions.html", statusField: "", hasEnricher: true, reasoning: "Step Functions list has no status; Wave 2 ListExecutions inspects ExecutionStatus (RUNNING/SUCCEEDED/FAILED/TIMED_OUT/ABORTED). FAILED/TIMED_OUT/ABORTED → finding; RUNNING is in-flight, excluded."},
	{shortName: "tg", apiDoc: "https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeTargetHealth.html", statusField: "", hasEnricher: true, reasoning: "Target Group list has no status; Wave 2 DescribeTargetHealth per-TG inspects TargetHealth.State. Any non-healthy state → finding."},
	{shortName: "ddb", apiDoc: "https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_DescribeTable.html", statusField: "status", healthyStatuses: []string{"ACTIVE"}, warningStatuses: []string{"CREATING", "UPDATING", "DELETING", "ARCHIVING"}, brokenStatuses: []string{"INACCESSIBLE_ENCRYPTION_CREDENTIALS", "ARCHIVED"}, reasoning: "DynamoDB TableStatus per DescribeTable. INACCESSIBLE_ENCRYPTION_CREDENTIALS = broken (lost KMS key); ARCHIVED = read-only terminal state, broken for operational purposes."},

	// Health-state types (Wave 1 Color func driven by a real status field).
	{shortName: "acm", apiDoc: "https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html", statusField: "status", healthyStatuses: []string{"ISSUED"}, warningStatuses: []string{"PENDING_VALIDATION"}, brokenStatuses: []string{"EXPIRED", "REVOKED", "FAILED", "VALIDATION_TIMED_OUT"}, reasoning: "ACM CertificateStatus per DescribeCertificate: ISSUED | PENDING_VALIDATION | EXPIRED | REVOKED | FAILED | VALIDATION_TIMED_OUT | INACTIVE. INACTIVE is admin action."},
	{shortName: "alarm", apiDoc: "https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_DescribeAlarms.html", statusField: "state", healthyStatuses: []string{"OK"}, warningStatuses: []string{"INSUFFICIENT_DATA"}, brokenStatuses: []string{"ALARM"}, reasoning: "CloudWatch MetricAlarm StateValue: OK | ALARM | INSUFFICIENT_DATA."},
	{shortName: "ami", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeImages.html", statusField: "state", healthyStatuses: []string{"available"}, warningStatuses: []string{"pending", "transient"}, brokenStatuses: []string{"failed", "error", "invalid"}, reasoning: "AMI State per DescribeImages: available | pending | transient | failed | error | invalid | deregistered."},
	{shortName: "asg", apiDoc: "https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_DescribeAutoScalingGroups.html", statusField: "status", healthyStatuses: []string{""}, warningStatuses: []string{"Delete in progress"}, reasoning: "ASG Status is either empty (active) or 'Delete in progress' per DescribeAutoScalingGroups. No broken state."},
	{shortName: "cf", apiDoc: "https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_ListDistributions.html", statusField: "status", healthyStatuses: []string{"Deployed"}, warningStatuses: []string{"InProgress"}, reasoning: "CloudFront Distribution Status per ListDistributions: InProgress | Deployed. No broken state."},
	{shortName: "cfn", apiDoc: "https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_DescribeStacks.html", statusField: "status", healthyStatuses: []string{"CREATE_COMPLETE", "UPDATE_COMPLETE", "IMPORT_COMPLETE"}, warningStatuses: []string{"CREATE_IN_PROGRESS", "UPDATE_IN_PROGRESS", "ROLLBACK_IN_PROGRESS", "UPDATE_ROLLBACK_IN_PROGRESS", "IMPORT_IN_PROGRESS", "IMPORT_ROLLBACK_IN_PROGRESS", "DELETE_IN_PROGRESS"}, brokenStatuses: []string{"CREATE_FAILED", "UPDATE_FAILED", "ROLLBACK_FAILED", "UPDATE_ROLLBACK_FAILED", "IMPORT_ROLLBACK_FAILED", "DELETE_FAILED", "ROLLBACK_COMPLETE", "UPDATE_ROLLBACK_COMPLETE", "IMPORT_ROLLBACK_COMPLETE"}, reasoning: "CloudFormation StackStatus per DescribeStacks. *_ROLLBACK_COMPLETE is a terminal rolled-back state and qualifies as broken (user intervention required)."},
	{shortName: "ct-events", apiDoc: "https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html", statusField: "", reasoning: "CloudTrail events don't participate in menu ctrl+z — ExcludeFromIssueBadge=true. Severity is event-level, not resource-health."},
	{shortName: "docdb-snap", apiDoc: "https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeDBClusterSnapshots.html", statusField: "status", healthyStatuses: []string{"available"}, warningStatuses: []string{"creating"}, brokenStatuses: []string{"failed"}, reasoning: "DocDB DBClusterSnapshot Status mirrors RDS snapshot vocabulary."},
	{shortName: "eb", apiDoc: "https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_DescribeEnvironments.html", statusField: "status", healthyStatuses: []string{"Ready"}, warningStatuses: []string{"Launching", "Updating", "Terminating", "Aborting"}, brokenStatuses: []string{"Terminated"}, reasoning: "Elastic Beanstalk Environment Status per DescribeEnvironments: Launching | Updating | Ready | Terminating | Terminated | Aborting. EB also has Health (Green/Yellow/Red/Grey) but a9s uses Status today."},
	{shortName: "ec2", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html", statusField: "state", healthyStatuses: []string{"running"}, warningStatuses: []string{"pending", "shutting-down", "stopping"}, brokenStatuses: []string{"stopped", "terminated"}, reasoning: "EC2 Instance State per DescribeInstances: pending | running | shutting-down | stopped | stopping | terminated. Stopped/terminated are explicit failure-or-deliberate-off states classified as broken. Wave 2 also overlays status checks via impaired/insufficient-data."},
	{shortName: "ecs", apiDoc: "https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeClusters.html", statusField: "status", healthyStatuses: []string{"ACTIVE"}, warningStatuses: []string{"PROVISIONING", "DEPROVISIONING"}, brokenStatuses: []string{"FAILED", "INACTIVE"}, reasoning: "ECS ClusterStatus per DescribeClusters: ACTIVE | PROVISIONING | DEPROVISIONING | FAILED | INACTIVE."},
	{shortName: "ecs-task", apiDoc: "https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeTasks.html", statusField: "last_status", healthyStatuses: []string{"RUNNING"}, warningStatuses: []string{"PROVISIONING", "PENDING", "ACTIVATING", "DEACTIVATING", "STOPPING", "DEPROVISIONING"}, brokenStatuses: []string{"STOPPED"}, reasoning: "ECS Task LastStatus per DescribeTasks. STOPPED tasks are terminal — often due to failure."},
	{shortName: "efs", apiDoc: "https://docs.aws.amazon.com/efs/latest/ug/API_DescribeFileSystems.html", statusField: "lifecycle_state", healthyStatuses: []string{"available"}, warningStatuses: []string{"creating", "updating", "deleting"}, brokenStatuses: []string{"error"}, reasoning: "EFS FileSystemDescription.LifeCycleState: creating | available | updating | deleting | deleted | error."},
	{shortName: "eks", apiDoc: "https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeCluster.html", statusField: "status", healthyStatuses: []string{"ACTIVE"}, warningStatuses: []string{"CREATING", "UPDATING", "DELETING"}, brokenStatuses: []string{"FAILED"}, reasoning: "EKS ClusterStatus per DescribeCluster."},
	{shortName: "elb", apiDoc: "https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeLoadBalancers.html", statusField: "state", healthyStatuses: []string{"active"}, warningStatuses: []string{"provisioning", "active_impaired"}, brokenStatuses: []string{"failed"}, reasoning: "ELBv2 State.Code per DescribeLoadBalancers: active | provisioning | active_impaired | failed."},
	{shortName: "eni", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeNetworkInterfaces.html", statusField: "status", healthyStatuses: []string{"in-use", "available"}, warningStatuses: []string{"attaching", "detaching"}, reasoning: "ENI Status per DescribeNetworkInterfaces: available | associated | attaching | in-use | detaching."},
	{shortName: "lambda", apiDoc: "https://docs.aws.amazon.com/lambda/latest/api/API_GetFunctionConfiguration.html", statusField: "state", healthyStatuses: []string{"Active"}, warningStatuses: []string{"Pending"}, brokenStatuses: []string{"Failed", "Inactive"}, reasoning: "Lambda FunctionConfiguration.State per GetFunctionConfiguration: Pending | Active | Inactive | Failed."},
	{shortName: "nat", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeNatGateways.html", statusField: "state", healthyStatuses: []string{"available"}, warningStatuses: []string{"pending", "deleting"}, brokenStatuses: []string{"failed"}, reasoning: "NAT Gateway State per DescribeNatGateways: pending | failed | available | deleting | deleted."},
	{shortName: "ng", apiDoc: "https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeNodegroup.html", statusField: "status", healthyStatuses: []string{"ACTIVE"}, warningStatuses: []string{"CREATING", "UPDATING", "DELETING"}, brokenStatuses: []string{"CREATE_FAILED", "DELETE_FAILED", "DEGRADED"}, reasoning: "EKS Nodegroup Status per DescribeNodegroup: CREATING | ACTIVE | UPDATING | DELETING | CREATE_FAILED | DELETE_FAILED | DEGRADED."},
	{shortName: "redis", apiDoc: "https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_DescribeReplicationGroups.html", statusField: "status", healthyStatuses: []string{"available"}, warningStatuses: []string{"creating", "modifying", "deleting", "snapshotting", "camouflage-error"}, brokenStatuses: []string{"create-failed"}, reasoning: "ElastiCache ReplicationGroup.Status per DescribeReplicationGroups."},
	{shortName: "subnet", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSubnets.html", statusField: "state", healthyStatuses: []string{"available"}, warningStatuses: []string{"pending"}, reasoning: "Subnet State per DescribeSubnets: pending | available."},
	{shortName: "tgw", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeTransitGateways.html", statusField: "state", healthyStatuses: []string{"available"}, warningStatuses: []string{"pending", "modifying", "deleting"}, brokenStatuses: []string{"failed"}, reasoning: "Transit Gateway State per DescribeTransitGateways: pending | available | modifying | deleting | deleted | failed."},
	{shortName: "vpc", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVpcs.html", statusField: "state", healthyStatuses: []string{"available"}, warningStatuses: []string{"pending"}, reasoning: "VPC State per DescribeVpcs: pending | available."},
	{shortName: "vpce", apiDoc: "https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVpcEndpoints.html", statusField: "state", healthyStatuses: []string{"available", "accepted"}, warningStatuses: []string{"pendingAcceptance", "pending", "deleting", "partial"}, brokenStatuses: []string{"failed", "rejected", "expired"}, reasoning: "VPC Endpoint State per DescribeVpcEndpoints: PendingAcceptance | Pending | Available | Deleting | Deleted | Rejected | Failed | Expired | Partial."},
}

// TestCR273_AllTypes_ContractRegistryCoverage guards the table: every
// registered resource type must have a corresponding row above. When a
// new type is added to the registry, the developer must also add a row
// here (or explicitly document the omission).
func TestCR273_AllTypes_ContractRegistryCoverage(t *testing.T) {
	declared := make(map[string]bool, len(typeContracts))
	for _, c := range typeContracts {
		if declared[c.shortName] {
			t.Errorf("duplicate typeContracts row for %q", c.shortName)
		}
		declared[c.shortName] = true
	}
	var missing []string
	for _, td := range resource.AllResourceTypes() {
		if !declared[td.ShortName] {
			missing = append(missing, td.ShortName)
		}
	}
	if len(missing) > 0 {
		t.Errorf("typeContracts table is missing rows for these registered types: %v\n\n"+
			"Add a row in typeContracts with: shortName, apiDoc URL, statusField (Fields key the "+
			"fetcher populates), healthy/warning/broken status strings from the AWS API reference, "+
			"and a one-line reasoning anchored to the API doc.", missing)
	}
	// Also: rows must not reference unregistered types.
	registered := make(map[string]bool, len(resource.AllResourceTypes()))
	for _, td := range resource.AllResourceTypes() {
		registered[td.ShortName] = true
	}
	for _, c := range typeContracts {
		if !registered[c.shortName] {
			t.Errorf("typeContracts row references unregistered shortName %q", c.shortName)
		}
	}
}

// TestCR273_AllTypes_ColorClassification asserts, for every registered
// type, that the Color func returns:
//   - ColorHealthy for every status in healthyStatuses
//   - ColorWarning for every status in warningStatuses
//   - ColorBroken for every status in brokenStatuses
//
// The status string is injected through r.Fields[statusField]. When the
// statusField is empty, the type has no lifecycle string on the list
// response and this subtest is skipped — those types rely on enricher
// findings or are purely config-only.
func TestCR273_AllTypes_ColorClassification(t *testing.T) {
	for _, c := range typeContracts {
		t.Run(c.shortName, func(t *testing.T) {
			td := resource.FindResourceType(c.shortName)
			if td == nil {
				t.Fatalf("type %q not registered", c.shortName)
			}
			if c.statusField == "" {
				t.Skipf("type %q has no statusField — list-level classification N/A", c.shortName)
			}
			check := func(status string, want resource.Color, bucket string) {
				t.Helper()
				r := resource.Resource{
					ID:     c.shortName + "-test",
					Status: status,
					Fields: map[string]string{c.statusField: status},
				}
				got := td.ResolveColor(r)
				if got != want {
					t.Errorf("AWS API status %q (%s bucket per %s) classified as %v, want %v\n\nReasoning: %s",
						status, bucket, c.apiDoc, got, want, c.reasoning)
				}
			}
			for _, s := range c.healthyStatuses {
				check(s, resource.ColorHealthy, "healthy")
			}
			for _, s := range c.warningStatuses {
				check(s, resource.ColorWarning, "warning")
			}
			for _, s := range c.brokenStatuses {
				check(s, resource.ColorBroken, "broken")
			}
		})
	}
}

// TestCR273_AllTypes_MenuCtrlZ_NoFalseNegatives asserts, for every
// registered type, that the main-menu ctrl+z filter surfaces the type
// when AWS reports issues for it.
//
// Scenarios:
//  - alwaysHealthy=true: AWS cannot produce a list-level issue for this
//    type. The filter MUST keep it hidden. We inject issues=2 anyway to
//    guard that the AlwaysHealthy flag always wins.
//  - alwaysHealthy=false: AWS reports issues=2; filter MUST show it.
func TestCR273_AllTypes_MenuCtrlZ_NoFalseNegatives(t *testing.T) {
	for _, c := range typeContracts {
		t.Run(c.shortName, func(t *testing.T) {
			td := resource.FindResourceType(c.shortName)
			if td == nil {
				t.Fatalf("type %q not registered", c.shortName)
			}
			if td.ExcludeFromIssueBadge {
				t.Skipf("type %q is ExcludeFromIssueBadge — never visible under ctrl+z", c.shortName)
			}

			m := newRootSizedModel()
			m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoadedMsg{
				Entries:        map[string]int{c.shortName: 3},
				Truncated:      map[string]bool{},
				IssueCounts:    map[string]int{c.shortName: 2},
				IssueTruncated: map[string]bool{c.shortName: false},
				IssueKnown:     map[string]bool{c.shortName: true},
				Expired:        false,
			})
			m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
			plain := stripANSI(rootViewContent(m))
			visible := strings.Contains(plain, td.Name)

			if c.alwaysHealthy {
				if visible {
					t.Errorf("%s marked alwaysHealthy but became visible under ctrl+z with injected issues=2 — AlwaysHealthy flag must always win.\n\nReasoning: %s",
						c.shortName, c.reasoning)
				}
				return
			}
			if !visible {
				t.Errorf("%s reported 2 issues but is NOT visible under ctrl+z — false negative.\n\n"+
					"AWS API: %s\nReasoning: %s\n\nRendered menu:\n%s",
					c.shortName, c.apiDoc, c.reasoning, plain)
			}
		})
	}
}

// TestCR273_AllTypes_MenuCtrlZ_NoFalsePositives asserts, for every
// registered type, that the main-menu ctrl+z filter hides the type when
// AWS reports zero issues authoritatively.
//
// "Authoritatively" means: issueCounts=0, issueKnown=true,
// issueTruncated=false, and (for enricher-backed types) Wave 2 also
// returned Issues=0 with Truncated=false and empty Findings.
//
// Every type — stateful OR alwaysHealthy — must be hidden under these
// conditions.
func TestCR273_AllTypes_MenuCtrlZ_NoFalsePositives(t *testing.T) {
	for _, c := range typeContracts {
		t.Run(c.shortName, func(t *testing.T) {
			td := resource.FindResourceType(c.shortName)
			if td == nil {
				t.Fatalf("type %q not registered", c.shortName)
			}
			if td.ExcludeFromIssueBadge {
				t.Skipf("type %q is ExcludeFromIssueBadge — never visible under ctrl+z", c.shortName)
			}

			m := newRootSizedModel()
			m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoadedMsg{
				Entries:        map[string]int{c.shortName: 3},
				Truncated:      map[string]bool{},
				IssueCounts:    map[string]int{c.shortName: 0},
				IssueTruncated: map[string]bool{c.shortName: false},
				IssueKnown:     map[string]bool{c.shortName: true},
				Expired:        false,
			})
			// Wave 2 clean, if applicable.
			if c.hasEnricher {
				if _, ok := awsclient.EnricherRegistry[c.shortName]; ok {
					m, _ = rootApplyMsg(m, messages.EnrichmentCheckedMsg{
						ResourceType: c.shortName,
						Issues:       0,
						Truncated:    false,
						Findings:     map[string]resource.EnrichmentFinding{},
						Err:          nil,
						Gen:          0,
						TypeGen:      0,
					})
				}
			}
			m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
			plain := stripANSI(rootViewContent(m))
			if strings.Contains(plain, td.Name) {
				t.Errorf("%s reported zero issues authoritatively but appears under ctrl+z — false positive.\n\n"+
					"AWS API: %s\nReasoning: %s\n\nRendered menu:\n%s",
					c.shortName, c.apiDoc, c.reasoning, plain)
			}
		})
	}
}

// TestCR273_AllTypes_MenuCtrlZ_Wave2ErroredSubCall_NoFalsePositives
// asserts the partial-error contract for enricher-backed types: if one
// sub-call errored (Wave 2 sets Truncated=true) but no actual issues were
// found (Issues=0, Findings empty), the type must NOT appear under
// ctrl+z. Truncation is about count completeness, not hidden issues.
//
// Non-enricher types skip this subtest.
func TestCR273_AllTypes_MenuCtrlZ_Wave2ErroredSubCall_NoFalsePositives(t *testing.T) {
	for _, c := range typeContracts {
		t.Run(c.shortName, func(t *testing.T) {
			td := resource.FindResourceType(c.shortName)
			if td == nil {
				t.Fatalf("type %q not registered", c.shortName)
			}
			if !c.hasEnricher {
				t.Skipf("%s has no Wave 2 enricher — partial-error scenario N/A", c.shortName)
			}
			if _, ok := awsclient.EnricherRegistry[c.shortName]; !ok {
				t.Skipf("%s declared hasEnricher=true but is not in EnricherRegistry", c.shortName)
			}

			m := newRootSizedModel()
			m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoadedMsg{
				Entries:        map[string]int{c.shortName: 3},
				Truncated:      map[string]bool{},
				IssueCounts:    map[string]int{c.shortName: 0},
				IssueTruncated: map[string]bool{c.shortName: false},
				IssueKnown:     map[string]bool{c.shortName: true},
				Expired:        false,
			})
			// Wave 2: one sub-call errored → Truncated=true, but Issues=0 and Findings={}.
			m, _ = rootApplyMsg(m, messages.EnrichmentCheckedMsg{
				ResourceType: c.shortName,
				Issues:       0,
				Truncated:    true,
				Findings:     map[string]resource.EnrichmentFinding{},
				Err:          nil,
				Gen:          0,
				TypeGen:      0,
			})
			m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl})
			plain := stripANSI(rootViewContent(m))
			if strings.Contains(plain, td.Name) {
				t.Errorf("%s Wave 2 errored on one sub-call (Truncated=true, Issues=0, Findings={}) "+
					"but appears under ctrl+z — false positive. Truncation signals count completeness, "+
					"not hidden issues.\n\nAWS API: %s\nReasoning: %s",
					c.shortName, c.apiDoc, c.reasoning)
			}
		})
	}
}

// _ silences unused-import warnings when some imports are conditionally used.
var _ = tui.Version
