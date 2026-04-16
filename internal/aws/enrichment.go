// Package aws provides AWS service clients and resource fetchers.
// This file implements Wave 2 enrichment functions for issue #196.
// Each enricher makes additional API calls to discover hidden issues
// that Wave 1's status-based counting cannot detect.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	acmsvc "github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	kafkasvc "github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	snssvc "github.com/aws/aws-sdk-go-v2/service/sns"
	sqssvc "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	smithy "github.com/aws/smithy-go"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrsvc "github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// EnricherRegistry maps resource short names to their Wave 2 enricher functions.
// Ordered by priority: batchable (cheap) first, per-resource (expensive) last.
//
// Every registered resource type per docs/attention-signals.md either:
//   - has a real Wave 2 enricher registered here (Wave 2 column non-empty), or
//   - is registered with NoOpEnricher (Wave 2 column is "None" in the doc).
//
// Doc-grounded test TestAttentionSignalsDoc enforces this contract.
var EnricherRegistry = map[string]EnricherFunc{
	"rds":      EnrichRDSDocDBMaintenance,
	"dbi":      EnrichRDSDocDBMaintenance,
	"dbc":      EnrichRDSDocDBMaintenance,
	"ecs-svc":  EnrichECSServices,
	"ecs":      EnrichECSClusters,
	"ecs-task": EnrichECSTasks,
	"eb-rule":  EnrichEventBridgeRuleTargets,
	"ddb":      EnrichDynamoDBPITR,
	"ec2":      EnrichEC2InstanceStatus,
	"asg":      EnrichASGScalingActivities,
	"ebs":      EnrichEBSVolumeStatus,
	"cb":       EnrichCodeBuildStatus,
	"tg":       EnrichTargetGroupHealth,
	"pipeline": EnrichCodePipelineStatus,
	"sfn":      EnrichStepFunctionsStatus,
	"glue":     EnrichGlueJobStatus,
	"backup":   EnrichBackupJobs,
	"ses":      EnrichSESAccount,
	"kms":      EnrichKMSRotation,
	"efs":      EnrichEFSMountTargets,
	"tgw":      EnrichTGWAttachments,
	"eb":       EnrichEBEnvironmentHealth,
	"elb":      EnrichELBAttributes,
	"sqs":      EnrichSQSAttributes,
	"sns":      EnrichSNSSubscriptions,
	"msk":      EnrichMSKCluster,
	"acm":      EnrichACMCertificate,
	"cf":       EnrichCloudFrontDistribution,
	"apigw":    EnrichAPIGatewayStage,
	"cfn":      EnrichCFNStackEvents,
	"ecr":          EnrichECRRepository,
	"codeartifact": EnrichCodeArtifactRepository,
	"athena":       EnrichAthenaWorkGroup,
	// Wave 2 = None per docs/attention-signals.md — explicit no-op registration
	// makes the empty Wave 2 contract testable.
	"alarm":      NoOpEnricher,
	"ami":        NoOpEnricher,
	"ct-events":  NoOpEnricher,
	"docdb-snap": NoOpEnricher,
	"ebs-snap":   NoOpEnricher,
	"eip":        NoOpEnricher,
	"eni":        NoOpEnricher,
	"igw":        NoOpEnricher,
	"kinesis":    NoOpEnricher,
	"lambda":     NoOpEnricher,
	"nat":        NoOpEnricher,
	// eks and ng use NoOpEnricher because their fetchers already perform the
	// per-resource DescribeCluster / DescribeNodegroup calls and populate the
	// health_issues_count and health_issues Wave 2 fields at fetch time. The
	// Color funcs read those fields. This is a pragmatic in-fetcher Wave 2;
	// the registry entry exists for contract conformance
	// (TestAttentionSignalsDoc enforces every documented Wave 2 row has a
	// registry entry).
	"eks": NoOpEnricher,
	"ng":  NoOpEnricher,
	// opensearch and trail use NoOpEnricher because their fetchers already
	// perform the per-resource Describe* calls (DescribeDomains and
	// GetTrailStatus respectively) and populate Wave 2 classification fields
	// at fetch time. The Color funcs read those fields. This is a pragmatic
	// in-fetcher Wave 2; the registry entry exists for contract conformance
	// (TestAttentionSignalsDoc enforces every documented Wave 2 row has a
	// registry entry).
	"opensearch": NoOpEnricher,
	"rds-snap":   NoOpEnricher,
	"redshift":   NoOpEnricher,
	"redis":      NoOpEnricher,
	"rtb":        NoOpEnricher,
	"secrets":    NoOpEnricher,
	"sg":         NoOpEnricher,
	"sns-sub":    NoOpEnricher,
	"ssm":        NoOpEnricher,
	"subnet":     NoOpEnricher,
	"trail":      NoOpEnricher,
	"vpc":        EnrichVPCFlowLogs,
	"vpce":       NoOpEnricher,
	"s3":         EnrichS3PublicAccessBlock,
}

// NoOpEnricher is registered for resource types whose Wave 2 column in
// docs/attention-signals.md is "None". It makes the "no Wave 2 signal"
// classification explicit in the registry rather than implicit-by-absence.
// Returns zero findings, zero issues, not truncated — never fails.
func NoOpEnricher(_ context.Context, _ *ServiceClients, _ []resource.Resource) (EnricherResult, error) {
	return EnricherResult{
		Findings:   map[string]resource.EnrichmentFinding{},
		IssueCount: 0,
		Truncated:  false,
	}, nil
}

// EnrichmentCap is the maximum number of per-resource API calls for non-batchable enrichers.
const EnrichmentCap = 50

// arnSuffix extracts the last colon-delimited segment from an ARN.
// For "arn:aws:rds:region:account:db:instance-id" it returns "instance-id".
func arnSuffix(arn string) string {
	idx := strings.LastIndex(arn, ":")
	if idx < 0 {
		return arn
	}
	return arn[idx+1:]
}

// isInstanceARN returns true when the RDS ARN targets a DB instance
// (resource-type segment = "db"), not a cluster, snapshot, or other resource.
// ARN format: arn:aws:rds:region:account:resource-type:id
func isInstanceARN(arn string) bool {
	parts := strings.Split(arn, ":")
	return len(parts) >= 7 && parts[5] == "db"
}

// formatDate formats a *time.Time as "2006-01-02" or returns "" for nil.
func formatDate(t interface{ Format(string) string }) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// EnrichRDSDocDBMaintenance calls DescribePendingMaintenanceActions (account-wide, 1 call)
// and returns a Finding for every resource with pending maintenance.
// Severity is "~" (informational); IssueCount is always 0 (excluded from menu badge).
// The API returns maintenance actions for all RDS/DocDB resources (clusters AND instances).
func EnrichRDSDocDBMaintenance(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.RDS == nil {
		return EnricherResult{Findings: findings}, nil
	}
	out, err := clients.RDS.DescribePendingMaintenanceActions(ctx, &rds.DescribePendingMaintenanceActionsInput{})
	if err != nil {
		return EnricherResult{}, err
	}
	// Collect probed resource IDs as an ordered slice for deterministic
	// suffix matching below. Using a map's random iteration order would
	// make key selection non-deterministic when two IDs both suffix-match
	// the same ARN (e.g. "foo-db" and "bar-foo-db").
	probeIDs := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			probeIDs = append(probeIDs, r.ID)
		}
	}
	// Emit a finding for every DB instance ARN that has pending maintenance.
	for _, action := range out.PendingMaintenanceActions {
		if action.ResourceIdentifier == nil {
			continue
		}
		arn := *action.ResourceIdentifier
		if !isInstanceARN(arn) {
			continue
		}
		// Collect action descriptions for the summary and rows.
		var actions []string
		var rows []resource.FindingRow
		for _, pa := range action.PendingMaintenanceActionDetails {
			if pa.Action != nil {
				actions = append(actions, *pa.Action)
			}
			// Emit a row per action detail.
			actionVal := ""
			if pa.Action != nil {
				actionVal = *pa.Action
			}
			applyMethod := ""
			if pa.OptInStatus != nil {
				applyMethod = *pa.OptInStatus
			}
			earliestTarget := ""
			if pa.AutoAppliedAfterDate != nil {
				earliestTarget = formatDate(pa.AutoAppliedAfterDate)
			} else if pa.ForcedApplyDate != nil {
				earliestTarget = formatDate(pa.ForcedApplyDate)
			}
			if actionVal != "" {
				rows = append(rows, resource.FindingRow{Label: "Action", Value: actionVal, Tier: "~"})
			}
			if applyMethod != "" {
				rows = append(rows, resource.FindingRow{Label: "Apply Method", Value: applyMethod})
			}
			if earliestTarget != "" {
				rows = append(rows, resource.FindingRow{Label: "Earliest Target", Value: earliestTarget, Tier: "~"})
			}
			if pa.Description != nil && *pa.Description != "" {
				rows = append(rows, resource.FindingRow{Label: "Description", Value: *pa.Description})
			}
		}
		summary := "pending maintenance"
		if len(actions) > 0 {
			summary = "pending maintenance: " + strings.Join(actions, ", ")
		}
		// Determine the key: prefer the longest matching probeID so that
		// when two IDs both suffix-match the same ARN (e.g. "foo-db" and
		// "bar-foo-db" for arn ":bar-foo-db"), the more specific one wins.
		// Iteration is over the ordered probeIDs slice — deterministic.
		key := ""
		for _, id := range probeIDs {
			if strings.HasSuffix(arn, ":"+id) && len(id) > len(key) {
				key = id
			}
		}
		if key == "" {
			key = arnSuffix(arn)
		}
		if key == "" {
			continue
		}
		findings[key] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  summary,
			Rows:     rows,
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: out.Marker != nil, Findings: findings}, nil
}

// EnrichEBSVolumeStatus calls DescribeVolumeStatus (1 call, all volumes)
// and returns a Finding for every volume with non-ok status.
// Severity is "!" (broken/degraded).
func EnrichEBSVolumeStatus(ctx context.Context, clients *ServiceClients, _ []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.EC2 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	out, err := clients.EC2.DescribeVolumeStatus(ctx, &ec2svc.DescribeVolumeStatusInput{})
	if err != nil {
		return EnricherResult{}, err
	}
	for _, v := range out.VolumeStatuses {
		if v.VolumeStatus == nil || v.VolumeStatus.Status == "ok" {
			continue
		}
		if v.VolumeId == nil {
			continue
		}
		ioState := string(v.VolumeStatus.Status)
		rows := []resource.FindingRow{
			{Label: "I/O State", Value: ioState, Tier: "!"},
		}
		// Most recent event (if any).
		if len(v.Events) > 0 {
			ev := v.Events[0]
			eventVal := ""
			if ev.EventType != nil {
				eventVal = *ev.EventType
			}
			if ev.Description != nil && *ev.Description != "" {
				eventVal = *ev.Description
			}
			if eventVal != "" {
				rows = append(rows, resource.FindingRow{Label: "Event", Value: eventVal, Tier: "~"})
			}
		}
		// Most recent action code (if any).
		if len(v.Actions) > 0 {
			ac := v.Actions[0]
			if ac.Code != nil && *ac.Code != "" {
				rows = append(rows, resource.FindingRow{Label: "Action Code", Value: *ac.Code})
			}
		}
		findings[*v.VolumeId] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  "volume I/O degraded",
			Rows:     rows,
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: out.NextToken != nil, Findings: findings}, nil
}

// EnrichCodeBuildStatus calls BatchGetBuilds for the latest build of each project
// and returns a Finding for every project whose latest build is not SUCCEEDED.
// Severity is "!" (broken/degraded). Summary: "latest build FAILED (<date>)".
func EnrichCodeBuildStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.CodeBuild == nil || len(resources) == 0 {
		return EnricherResult{Findings: findings}, nil
	}
	names := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			names = append(names, r.ID)
		}
	}
	if len(names) == 0 {
		return EnricherResult{Findings: findings}, nil
	}
	buildIDToProject := make(map[string]string, len(names))
	var buildIDs []string
	truncated := len(resources) > EnrichmentCap
	for _, name := range names {
		if len(buildIDs) >= EnrichmentCap {
			break
		}
		out, err := clients.CodeBuild.ListBuildsForProject(ctx, &codebuild.ListBuildsForProjectInput{
			ProjectName: aws.String(name),
			SortOrder:   cbtypes.SortOrderTypeDescending,
		})
		if err != nil {
			truncated = true
			continue
		}
		if len(out.Ids) > 0 {
			id := out.Ids[0]
			buildIDs = append(buildIDs, id)
			buildIDToProject[id] = name
		}
	}
	if len(buildIDs) == 0 {
		return EnricherResult{Truncated: truncated, Findings: findings}, nil
	}
	builds, err := clients.CodeBuild.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{
		Ids: buildIDs,
	})
	if err != nil {
		return EnricherResult{}, err
	}
	for _, b := range builds.Builds {
		switch b.BuildStatus {
		case cbtypes.StatusTypeSucceeded, cbtypes.StatusTypeInProgress, cbtypes.StatusTypeStopped:
			continue
		}
		if b.Id == nil {
			continue
		}
		projectName := buildIDToProject[*b.Id]
		if projectName == "" {
			continue
		}
		statusVal := string(b.BuildStatus)
		rows := []resource.FindingRow{
			{Label: "Status", Value: statusVal, Tier: "!"},
		}
		if b.EndTime != nil {
			rows = append(rows, resource.FindingRow{Label: "Ended", Value: b.EndTime.Format("2006-01-02")})
		}
		// Append the latest failed phase if build is not complete.
		if !b.BuildComplete {
			if b.CurrentPhase != nil && *b.CurrentPhase != "" {
				rows = append(rows, resource.FindingRow{Label: "Current Phase", Value: *b.CurrentPhase, Tier: "~"})
			}
		} else {
			// Find the latest failed phase.
			for i := len(b.Phases) - 1; i >= 0; i-- {
				ph := b.Phases[i]
				if ph.PhaseStatus == cbtypes.StatusTypeFailed {
					rows = append(rows, resource.FindingRow{Label: "Phase", Value: string(ph.PhaseType), Tier: "!"})
					break
				}
			}
		}
		summary := fmt.Sprintf("latest build %s", statusVal)
		if b.EndTime != nil {
			summary = fmt.Sprintf("latest build %s (%s)", statusVal, b.EndTime.Format("2006-01-02"))
		}
		findings[projectName] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  summary,
			Rows:     rows,
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichTargetGroupHealth calls DescribeTargetHealth for each target group (1 per TG, cap ~50).
// Returns a Finding for each TG with at least one unhealthy target.
// Severity is "!" (broken/degraded). Summary: "unhealthy targets: X/Y".
func EnrichTargetGroupHealth(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.ELBv2 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		if r.ID == "" {
			continue
		}
		out, err := clients.ELBv2.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
			TargetGroupArn: aws.String(r.ID),
		})
		if err != nil {
			truncated = true
			continue
		}
		total := len(out.TargetHealthDescriptions)
		unhealthy := 0
		var firstReason string
		for _, t := range out.TargetHealthDescriptions {
			if t.TargetHealth != nil && t.TargetHealth.State != elbtypes.TargetHealthStateEnumHealthy {
				unhealthy++
				if firstReason == "" && t.TargetHealth.Reason != "" {
					firstReason = string(t.TargetHealth.Reason)
				}
			}
		}
		if unhealthy > 0 {
			rows := []resource.FindingRow{
				{Label: "Unhealthy Targets", Value: fmt.Sprintf("%d/%d", unhealthy, total), Tier: "!"},
			}
			if firstReason != "" {
				rows = append(rows, resource.FindingRow{Label: "Reason", Value: firstReason, Tier: "~"})
			}
			findings[r.ID] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  fmt.Sprintf("unhealthy targets: %d/%d", unhealthy, total),
				Rows:     rows,
			}
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichCodePipelineStatus calls GetPipelineState for each pipeline (1 per pipeline, cap ~50).
// Returns a Finding for each pipeline with a failed stage.
// Severity is "!" (broken/degraded). Summary: "stage <Name> failed".
func EnrichCodePipelineStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.CodePipeline == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		if r.Name == "" {
			continue
		}
		out, err := clients.CodePipeline.GetPipelineState(ctx, &codepipeline.GetPipelineStateInput{
			Name: aws.String(r.Name),
		})
		if err != nil {
			truncated = true
			continue
		}
		for _, stage := range out.StageStates {
			if stage.LatestExecution == nil || stage.LatestExecution.Status != "Failed" {
				continue
			}
			stageName := ""
			if stage.StageName != nil {
				stageName = *stage.StageName
			}
			rows := []resource.FindingRow{
				{Label: "Failed Stage", Value: stageName, Tier: "!"},
				{Label: "Status", Value: string(stage.LatestExecution.Status)},
			}
			// Collect error details from any failed action in this stage.
			for _, action := range stage.ActionStates {
				if action.LatestExecution == nil {
					continue
				}
				if action.LatestExecution.Status != "Failed" {
					continue
				}
				if action.LatestExecution.ErrorDetails != nil && action.LatestExecution.ErrorDetails.Message != nil {
					msg := *action.LatestExecution.ErrorDetails.Message
					if msg != "" {
						rows = append(rows, resource.FindingRow{Label: "Error", Value: msg, Tier: "!"})
					}
					break
				}
			}
			key := r.ID
			if key == "" {
				key = r.Name
			}
			findings[key] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  fmt.Sprintf("stage %s failed", stageName),
				Rows:     rows,
			}
			break // first failed stage is sufficient
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichStepFunctionsStatus calls ListExecutions(max:1) for each state machine (1 per SFN, cap ~50).
// Returns a Finding for each state machine whose latest execution is FAILED, TIMED_OUT, or ABORTED.
// Severity is "!" (broken/degraded). Summary: "latest execution <STATUS>".
func EnrichStepFunctionsStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.SFN == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		if r.ID == "" {
			continue
		}
		out, err := clients.SFN.ListExecutions(ctx, &sfn.ListExecutionsInput{
			StateMachineArn: aws.String(r.ID),
			MaxResults:      1,
		})
		if err != nil {
			truncated = true
			continue
		}
		if len(out.Executions) > 0 {
			s := out.Executions[0].Status
			if s == sfntypes.ExecutionStatusFailed || s == sfntypes.ExecutionStatusTimedOut || s == sfntypes.ExecutionStatusAborted {
				exec := out.Executions[0]
				rows := []resource.FindingRow{
					{Label: "Latest Status", Value: string(s), Tier: "!"},
				}
				if exec.StopDate != nil {
					rows = append(rows, resource.FindingRow{Label: "Ended", Value: exec.StopDate.Format("2006-01-02")})
				}
				if exec.Name != nil && *exec.Name != "" {
					rows = append(rows, resource.FindingRow{Label: "Execution Name", Value: *exec.Name})
				}
				findings[r.ID] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  fmt.Sprintf("latest execution %s", string(s)),
					Rows:     rows,
				}
			}
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichBackupJobs calls ListBackupJobs once (account-wide) and returns a Finding
// for each BackupPlanId that has a failed/aborted/expired/partial job in the last 24h.
// Severity "!" for FAILED/ABORTED/EXPIRED, "~" for PARTIAL.
// IssueCount counts only "!" findings. First failure per plan wins; no pagination (TODO).
func EnrichBackupJobs(ctx context.Context, clients *ServiceClients, _ []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.Backup == nil {
		return EnricherResult{Findings: findings}, nil
	}
	out, err := clients.Backup.ListBackupJobs(ctx, &backup.ListBackupJobsInput{})
	if err != nil {
		return EnricherResult{}, err
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, job := range out.BackupJobs {
		if job.CreationDate == nil || job.CreationDate.Before(cutoff) {
			continue
		}
		// Determine the key from BackupPlanId (via CreatedBy) or fall back to BackupJobId.
		key := ""
		if job.CreatedBy != nil && job.CreatedBy.BackupPlanId != nil && *job.CreatedBy.BackupPlanId != "" {
			key = *job.CreatedBy.BackupPlanId
		} else if job.BackupJobId != nil {
			key = *job.BackupJobId
		}
		if key == "" {
			continue
		}
		// First failure wins — skip if already recorded.
		if _, exists := findings[key]; exists {
			continue
		}
		switch job.State {
		case backuptypes.BackupJobStateFailed, backuptypes.BackupJobStateAborted, backuptypes.BackupJobStateExpired:
			stateStr := strings.ToLower(string(job.State))
			findings[key] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  fmt.Sprintf("backup %s in last 24h", stateStr),
				Rows: []resource.FindingRow{
					{Label: "State", Value: string(job.State), Tier: "!"},
				},
			}
		case backuptypes.BackupJobStatePartial:
			findings[key] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "backup PARTIAL in last 24h",
				Rows: []resource.FindingRow{
					{Label: "State", Value: string(job.State), Tier: "~"},
				},
			}
		}
	}
	issueCount := 0
	for _, f := range findings {
		if f.Severity == "!" {
			issueCount++
		}
	}
	return EnricherResult{IssueCount: issueCount, Truncated: out.NextToken != nil, Findings: findings}, nil
}

// EnrichSESAccount calls GetAccount once (account-wide) and returns a Finding
// keyed "account" when the account is shut down, on probation, or sending is disabled.
// Severity "!" for SHUTDOWN, "~" for PROBATION or sending disabled.
func EnrichSESAccount(ctx context.Context, clients *ServiceClients, _ []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.SESv2 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	out, err := clients.SESv2.GetAccount(ctx, &sesv2.GetAccountInput{})
	if err != nil {
		return EnricherResult{}, err
	}
	if out.EnforcementStatus != nil {
		switch *out.EnforcementStatus {
		case "SHUTDOWN":
			findings["account"] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  "SES account SHUTDOWN — sending blocked",
				Rows: []resource.FindingRow{
					{Label: "Enforcement Status", Value: "SHUTDOWN", Tier: "!"},
				},
			}
		case "PROBATION":
			findings["account"] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "SES account on PROBATION",
				Rows: []resource.FindingRow{
					{Label: "Enforcement Status", Value: "PROBATION", Tier: "~"},
				},
			}
		}
	}
	// Only check sending-disabled if enforcement status didn't already produce a finding.
	if _, exists := findings["account"]; !exists && !out.SendingEnabled {
		findings["account"] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  "SES account sending disabled",
			Rows: []resource.FindingRow{
				{Label: "Sending Enabled", Value: "false", Tier: "~"},
			},
		}
	}
	issueCount := 0
	for _, f := range findings {
		if f.Severity == "!" {
			issueCount++
		}
	}
	return EnricherResult{IssueCount: issueCount, Truncated: false, Findings: findings}, nil
}

// EnrichEC2InstanceStatus calls DescribeInstanceStatus(IncludeAllInstances=true) once (account-wide)
// and returns a Finding for every instance whose system or instance status is not "ok".
// Scheduled events with NotBeforeDeadline within the next 7 days also produce a Finding.
// Severity "!" for status != ok; "~" for scheduled events. IssueCount counts "!" findings only.
func EnrichEC2InstanceStatus(ctx context.Context, clients *ServiceClients, _ []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.EC2 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	out, err := clients.EC2.DescribeInstanceStatus(ctx, &ec2svc.DescribeInstanceStatusInput{
		IncludeAllInstances: aws.Bool(true),
	})
	if err != nil {
		return EnricherResult{}, err
	}

	now := time.Now()
	cutoff := now.Add(7 * 24 * time.Hour)

	for _, is := range out.InstanceStatuses {
		if is.InstanceId == nil {
			continue
		}
		id := *is.InstanceId

		// Collect rows for this instance.
		var rows []resource.FindingRow
		severity := "~" // start informational; upgrade to "!" on real impairment

		// Check instance status.
		if is.InstanceStatus != nil && is.InstanceStatus.Status != ec2types.SummaryStatusOk {
			statusStr := string(is.InstanceStatus.Status)
			rows = append(rows, resource.FindingRow{Label: "Instance Status", Value: statusStr, Tier: "!"})
			severity = "!"
		}

		// Check system status.
		if is.SystemStatus != nil && is.SystemStatus.Status != ec2types.SummaryStatusOk {
			statusStr := string(is.SystemStatus.Status)
			rows = append(rows, resource.FindingRow{Label: "System Status", Value: statusStr, Tier: "!"})
			severity = "!"
		}

		// Check scheduled events within 7 days.
		// NotBeforeDeadline is the hard deadline (forced retirement/reboot).
		// NotBefore is the earliest scheduled start — also within 7d is actionable.
		for _, ev := range is.Events {
			var eventDate *time.Time
			if ev.NotBeforeDeadline != nil && ev.NotBeforeDeadline.Before(cutoff) {
				eventDate = ev.NotBeforeDeadline
			} else if ev.NotBefore != nil && ev.NotBefore.Before(cutoff) {
				eventDate = ev.NotBefore
			}
			if eventDate == nil {
				continue
			}
			code := string(ev.Code)
			dateStr := eventDate.Format("2006-01-02")
			rows = append(rows, resource.FindingRow{
				Label: "Scheduled Event",
				Value: fmt.Sprintf("%s at %s", code, dateStr),
				Tier:  "~",
			})
		}

		if len(rows) == 0 {
			continue
		}

		// Build summary: prefer the first "!" row's value, fall back to "~".
		summary := ""
		for _, row := range rows {
			if row.Tier == "!" {
				summary = fmt.Sprintf("%s: %s", strings.ToLower(row.Label), row.Value)
				break
			}
		}
		if summary == "" && len(rows) > 0 {
			summary = fmt.Sprintf("scheduled event: %s", rows[0].Value)
		}

		findings[id] = resource.EnrichmentFinding{
			Severity: severity,
			Summary:  summary,
			Rows:     rows,
		}
	}

	issueCount := 0
	for _, f := range findings {
		if f.Severity == "!" {
			issueCount++
		}
	}
	return EnricherResult{IssueCount: issueCount, Truncated: out.NextToken != nil, Findings: findings}, nil
}

// EnrichASGScalingActivities calls DescribeScalingActivities(MaxRecords=1) for each ASG
// (cap EnrichmentCap) and returns a Finding when the latest activity StatusCode == Failed.
// Severity is "!" (broken/degraded). Summary: "latest scaling activity failed: <statusMessage>".
func EnrichASGScalingActivities(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.AutoScaling == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		if r.ID == "" {
			continue
		}
		name := r.ID
		out, err := clients.AutoScaling.DescribeScalingActivities(ctx, &autoscaling.DescribeScalingActivitiesInput{
			AutoScalingGroupName: &name,
			MaxRecords:           aws.Int32(1),
		})
		if err != nil {
			truncated = true
			continue
		}
		if len(out.Activities) == 0 {
			continue
		}
		act := out.Activities[0]
		if act.StatusCode != asgtypes.ScalingActivityStatusCodeFailed {
			continue
		}
		statusMsg := ""
		if act.StatusMessage != nil {
			statusMsg = *act.StatusMessage
		}
		summary := "latest scaling activity failed"
		if statusMsg != "" {
			summary = fmt.Sprintf("latest scaling activity failed: %s", statusMsg)
		}
		rows := []resource.FindingRow{
			{Label: "Status", Value: string(act.StatusCode), Tier: "!"},
		}
		if statusMsg != "" {
			rows = append(rows, resource.FindingRow{Label: "Message", Value: statusMsg, Tier: "!"})
		}
		if act.Cause != nil && *act.Cause != "" {
			rows = append(rows, resource.FindingRow{Label: "Cause", Value: *act.Cause})
		}
		if act.StartTime != nil {
			rows = append(rows, resource.FindingRow{Label: "Started", Value: act.StartTime.Format("2006-01-02")})
		}
		findings[r.ID] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  summary,
			Rows:     rows,
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichGlueJobStatus calls GetJobRuns(max:1) for each job (1 per job, cap ~50).
// Returns a Finding for each job whose latest run is FAILED, ERROR, or TIMEOUT.
// Severity is "!" (broken/degraded). Summary: "latest run <STATUS>".
func EnrichGlueJobStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.Glue == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		if r.Name == "" {
			continue
		}
		out, err := clients.Glue.GetJobRuns(ctx, &glue.GetJobRunsInput{
			JobName:    aws.String(r.Name),
			MaxResults: aws.Int32(1),
		})
		if err != nil {
			truncated = true
			continue
		}
		if len(out.JobRuns) > 0 {
			run := out.JobRuns[0]
			s := run.JobRunState
			if s == gluetypes.JobRunStateFailed || s == gluetypes.JobRunStateError || s == gluetypes.JobRunStateTimeout {
				rows := []resource.FindingRow{
					{Label: "State", Value: string(s), Tier: "!"},
				}
				if run.CompletedOn != nil {
					rows = append(rows, resource.FindingRow{Label: "Ended", Value: run.CompletedOn.Format("2006-01-02")})
				}
				if run.ErrorMessage != nil && *run.ErrorMessage != "" {
					rows = append(rows, resource.FindingRow{Label: "Error", Value: *run.ErrorMessage, Tier: "!"})
				}
				key := r.ID
				if key == "" {
					key = r.Name
				}
				findings[key] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  fmt.Sprintf("latest run %s", string(s)),
					Rows:     rows,
				}
			}
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichDynamoDBPITR calls DescribeContinuousBackups for each table (cap EnrichmentCap)
// and returns a Finding when PITR is not enabled.
// Severity is "~" (informational); IssueCount counts PITR-disabled findings.
func EnrichDynamoDBPITR(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.DynamoDB == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		name := r.Name
		if name == "" {
			name = r.ID
		}
		if name == "" {
			continue
		}
		out, err := clients.DynamoDB.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{
			TableName: aws.String(name),
		})
		if err != nil {
			// sub-call error: skip this table, mark truncated to signal incomplete data
			truncated = true
			continue
		}
		if out.ContinuousBackupsDescription == nil {
			continue
		}
		pitr := out.ContinuousBackupsDescription.PointInTimeRecoveryDescription
		if pitr == nil {
			continue
		}
		if string(pitr.PointInTimeRecoveryStatus) != "ENABLED" {
			findings[name] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "PITR disabled",
			}
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichKMSRotation calls GetKeyRotationStatus for each customer-managed key (cap EnrichmentCap)
// and returns a Finding when key rotation is not enabled.
// Severity is "~" (informational per CIS KMS.1); IssueCount counts rotation-disabled findings.
// AWS-managed keys reject GetKeyRotationStatus with AccessDeniedException — that error is
// silently skipped without marking Truncated. Other per-key errors set Truncated=true.
func EnrichKMSRotation(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.KMS == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		keyID := r.ID
		if keyID == "" {
			continue
		}
		out, err := clients.KMS.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{
			KeyId: aws.String(keyID),
		})
		if err != nil {
			code, _, _ := ClassifyAWSError(err)
			if code == "AccessDeniedException" || code == "AccessDenied" {
				// AWS-managed keys: skip silently without marking truncated
				continue
			}
			// Any other error: skip this key but signal incomplete data via truncated
			truncated = true
			continue
		}
		if !out.KeyRotationEnabled {
			findings[keyID] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "key rotation disabled (CIS KMS.1)",
			}
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichEFSMountTargets calls DescribeMountTargets per file system (cap EnrichmentCap)
// and returns a Finding for any file system with a mount target whose LifeCycleState
// is not "available". Severity is "!" (broken/degraded).
// Summary: "mount target unavailable: <mountTargetID> in <az>".
func EnrichEFSMountTargets(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.EFS == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		fsID := r.ID
		if fsID == "" {
			continue
		}
		out, err := clients.EFS.DescribeMountTargets(ctx, &efs.DescribeMountTargetsInput{
			FileSystemId: aws.String(fsID),
		})
		if err != nil {
			truncated = true
			continue
		}
		for _, mt := range out.MountTargets {
			if mt.LifeCycleState == "available" {
				continue
			}
			mtID := ""
			if mt.MountTargetId != nil {
				mtID = *mt.MountTargetId
			}
			az := ""
			if mt.AvailabilityZoneName != nil {
				az = *mt.AvailabilityZoneName
			}
			findings[fsID] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  fmt.Sprintf("mount target unavailable: %s in %s", mtID, az),
				Rows: []resource.FindingRow{
					{Label: "Mount Target", Value: mtID, Tier: "!"},
					{Label: "AZ", Value: az},
					{Label: "State", Value: string(mt.LifeCycleState), Tier: "!"},
				},
			}
			break // first finding per FS is sufficient
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichTGWAttachments calls DescribeTransitGatewayAttachments per TGW (cap EnrichmentCap)
// and returns a Finding for any TGW with attachments in a failed or transitional state.
// Severity "!" for failed/failing; severity "~" for modifying/pendingAcceptance/rollingBack.
// When multiple issues exist on the same TGW, the worst severity ("!") takes precedence.
func EnrichTGWAttachments(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.EC2 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		tgwID := r.ID
		if tgwID == "" {
			continue
		}
		out, err := clients.EC2.DescribeTransitGatewayAttachments(ctx, &ec2svc.DescribeTransitGatewayAttachmentsInput{
			Filters: []ec2types.Filter{
				{Name: aws.String("transit-gateway-id"), Values: []string{tgwID}},
			},
		})
		if err != nil {
			truncated = true
			continue
		}
		// Collect worst finding across all attachments for this TGW.
		// "!" severity beats "~" severity.
		var worst *resource.EnrichmentFinding
		for _, att := range out.TransitGatewayAttachments {
			attID := ""
			if att.TransitGatewayAttachmentId != nil {
				attID = *att.TransitGatewayAttachmentId
			}
			state := string(att.State)
			var candidate *resource.EnrichmentFinding
			switch state {
			case "failed", "failing":
				candidate = &resource.EnrichmentFinding{
					Severity: "!",
					Summary:  fmt.Sprintf("attachment %s failed", attID),
					Rows: []resource.FindingRow{
						{Label: "Attachment", Value: attID, Tier: "!"},
						{Label: "State", Value: state, Tier: "!"},
					},
				}
			case "modifying", "pendingAcceptance", "rollingBack":
				candidate = &resource.EnrichmentFinding{
					Severity: "~",
					Summary:  fmt.Sprintf("attachment %s %s", attID, state),
					Rows: []resource.FindingRow{
						{Label: "Attachment", Value: attID, Tier: "~"},
						{Label: "State", Value: state, Tier: "~"},
					},
				}
			}
			if candidate == nil {
				continue
			}
			if worst == nil || (worst.Severity != "!" && candidate.Severity == "!") {
				worst = candidate
			}
		}
		if worst != nil {
			findings[tgwID] = *worst
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichVPCFlowLogs calls DescribeFlowLogs per VPC (cap EnrichmentCap)
// and returns a Finding when no active flow logs exist for the VPC.
// Severity is "~" (informational per CIS EC2.6).
// Summary: "no active VPC flow logs (CIS EC2.6)".
func EnrichVPCFlowLogs(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.EC2 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		vpcID := r.ID
		if vpcID == "" {
			continue
		}
		out, err := clients.EC2.DescribeFlowLogs(ctx, &ec2svc.DescribeFlowLogsInput{
			Filter: []ec2types.Filter{
				{Name: aws.String("resource-id"), Values: []string{vpcID}},
			},
		})
		if err != nil {
			truncated = true
			continue
		}
		// No flow logs at all, or none with ACTIVE status → finding.
		hasActive := false
		for _, fl := range out.FlowLogs {
			if fl.FlowLogStatus != nil && *fl.FlowLogStatus == "ACTIVE" {
				hasActive = true
				break
			}
		}
		if !hasActive {
			findings[vpcID] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "no active VPC flow logs (CIS EC2.6)",
			}
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichS3PublicAccessBlock calls GetPublicAccessBlock per bucket (cap EnrichmentCap)
// and returns a Finding when the bucket has no PAB configuration, or when any of the
// four PAB flags is false.
// Severity is "~" (informational).
// Summaries:
//   - "no public access block (account-level may apply)" — NoSuchPublicAccessBlockConfiguration
//   - "public-access block partial: <flag>=false" — one or more flags false
func EnrichS3PublicAccessBlock(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.S3 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		name := r.Name
		if name == "" {
			name = r.ID
		}
		if name == "" {
			continue
		}
		out, err := clients.S3.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{
			Bucket: aws.String(name),
		})
		if err != nil {
			// Check for NoSuchPublicAccessBlockConfiguration (bucket has no PAB config set).
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchPublicAccessBlockConfiguration" {
				findings[name] = resource.EnrichmentFinding{
					Severity: "~",
					Summary:  "no public access block (account-level may apply)",
				}
				continue
			}
			// Other errors: skip but signal incomplete data.
			truncated = true
			continue
		}
		if out.PublicAccessBlockConfiguration == nil {
			findings[name] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "no public access block (account-level may apply)",
			}
			continue
		}
		cfg := out.PublicAccessBlockConfiguration
		// Check each of the four PAB flags; report the first false one.
		type flagCheck struct {
			name  string
			value *bool
		}
		flags := []flagCheck{
			{"BlockPublicAcls", cfg.BlockPublicAcls},
			{"IgnorePublicAcls", cfg.IgnorePublicAcls},
			{"BlockPublicPolicy", cfg.BlockPublicPolicy},
			{"RestrictPublicBuckets", cfg.RestrictPublicBuckets},
		}
		for _, fc := range flags {
			if fc.value == nil || !*fc.value {
				findings[name] = resource.EnrichmentFinding{
					Severity: "~",
					Summary:  fmt.Sprintf("public-access block partial: %s=false", fc.name),
				}
				break
			}
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichECSServices is a Wave 2 enricher for ECS services.
// It groups services by cluster name, batches DescribeServices calls (up to 10 per
// cluster per call — the ECS API maximum), and raises findings for:
//   - Any deployment with RolloutState == FAILED → "!" finding
//   - deployment circuit-breaker triggered → "!" finding
//   - runningCount < desiredCount with no IN_PROGRESS deployment → "!" finding
//   - Recent events (last 10m) containing "unable to place" or "ELB health checks failed" → "!" finding
func EnrichECSServices(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.ECS == nil || len(resources) == 0 {
		return EnricherResult{Findings: findings}, nil
	}

	// Group service names by cluster name. Both fields are populated by FetchECSServicesPage.
	clusterServices := make(map[string][]string)
	for _, r := range resources {
		cluster := r.Fields["cluster"]
		svcName := r.Fields["service_name"]
		if cluster == "" || svcName == "" {
			continue
		}
		clusterServices[cluster] = append(clusterServices[cluster], svcName)
	}

	truncated := len(resources) > EnrichmentCap
	checked := 0

	for clusterName, svcNames := range clusterServices {
		// ECS DescribeServices accepts up to 10 services per call.
		const descBatch = 10
		for i := 0; i < len(svcNames); i += descBatch {
			if checked >= EnrichmentCap {
				truncated = true
				break
			}
			end := min(i+descBatch, len(svcNames))
			batch := svcNames[i:end]
			checked += len(batch)

			out, err := clients.ECS.DescribeServices(ctx, &ecs.DescribeServicesInput{
				Cluster:  aws.String(clusterName),
				Services: batch,
			})
			if err != nil {
				truncated = true
				continue
			}

			now := time.Now()
			for _, svc := range out.Services {
				svcName := ""
				if svc.ServiceName != nil {
					svcName = *svc.ServiceName
				}
				if svcName == "" {
					continue
				}

				// Check deployments for rollout failures and circuit-breaker.
				hasInProgress := false
				var deploymentIssues []string
				for _, dep := range svc.Deployments {
					if dep.RolloutState == ecstypes.DeploymentRolloutStateInProgress {
						hasInProgress = true
					}
					if dep.RolloutState == ecstypes.DeploymentRolloutStateFailed {
						reason := ""
						if dep.RolloutStateReason != nil {
							reason = *dep.RolloutStateReason
						}
						if reason != "" {
							deploymentIssues = append(deploymentIssues, fmt.Sprintf("deployment rollout FAILED: %s", reason))
						} else {
							deploymentIssues = append(deploymentIssues, "deployment rollout FAILED")
						}
						// Detect circuit-breaker in the rollout-state reason.
						if strings.Contains(strings.ToLower(reason), "circuit breaker") {
							deploymentIssues = append(deploymentIssues, "deployment circuit-breaker triggered")
						}
					}
				}

				// runningCount < desiredCount with no IN_PROGRESS deployment → stuck.
				serviceStuck := svc.DesiredCount > 0 &&
					svc.RunningCount < svc.DesiredCount &&
					!hasInProgress

				// Check recent events for placement/ELB failures.
				var eventIssues []string
				for _, ev := range svc.Events {
					if ev.CreatedAt == nil || ev.Message == nil {
						continue
					}
					if now.Sub(*ev.CreatedAt) > 10*time.Minute {
						break // Events are newest-first; stop once outside the 10m window.
					}
					msg := strings.ToLower(*ev.Message)
					if strings.Contains(msg, "unable to place") {
						eventIssues = append(eventIssues, "unable to place task")
					} else if strings.Contains(msg, "elb health checks failed") || strings.Contains(msg, "health checks failed") {
						eventIssues = append(eventIssues, "ELB health checks failed")
					}
				}

				if len(deploymentIssues) == 0 && !serviceStuck && len(eventIssues) == 0 {
					continue
				}

				var rows []resource.FindingRow
				for _, issue := range deploymentIssues {
					rows = append(rows, resource.FindingRow{Label: "Deployment", Value: issue, Tier: "!"})
				}
				if serviceStuck {
					rows = append(rows, resource.FindingRow{
						Label: "Tasks",
						Value: fmt.Sprintf("running %d / desired %d (stuck)", svc.RunningCount, svc.DesiredCount),
						Tier:  "!",
					})
				}
				for _, issue := range eventIssues {
					rows = append(rows, resource.FindingRow{Label: "Event", Value: issue, Tier: "!"})
				}

				summary := "deployment failed"
				if len(deploymentIssues) == 0 && serviceStuck {
					summary = fmt.Sprintf("service stuck: running %d / desired %d", svc.RunningCount, svc.DesiredCount)
				} else if len(deploymentIssues) == 0 && len(eventIssues) > 0 {
					summary = eventIssues[0]
				}

				findings[svcName] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  summary,
					Rows:     rows,
				}
			}
		}
	}

	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichECSClusters is a Wave 2 enricher for ECS clusters.
// It calls DescribeClusters with Include=STATISTICS and raises findings for:
//   - pendingTasksCount > 0 → "~" finding (pending tasks indicate scheduling pressure)
//   - runningTasksCount == 0 && registeredContainerInstancesCount > 0 → "~" finding
//     (instances registered but nothing running — likely stuck deployment or misconfiguration)
//
// Note: IssueCount is 0 for this enricher because all findings are severity "~"
// (informational) and do not contribute to the attention menu badge per the
// EnricherResult contract.
func EnrichECSClusters(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.ECS == nil || len(resources) == 0 {
		return EnricherResult{Findings: findings}, nil
	}

	clusterNames := make([]string, 0, len(resources))
	for _, r := range resources {
		if name := r.Fields["cluster_name"]; name != "" {
			clusterNames = append(clusterNames, name)
		}
	}

	truncated := len(resources) > EnrichmentCap
	checked := 0

	// DescribeClusters accepts up to 100 cluster names per call.
	const descBatch = 100
	for i := 0; i < len(clusterNames); i += descBatch {
		if checked >= EnrichmentCap {
			truncated = true
			break
		}
		end := min(i+descBatch, len(clusterNames))
		batch := clusterNames[i:end]
		checked += len(batch)

		out, err := clients.ECS.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: batch,
			Include:  []ecstypes.ClusterField{ecstypes.ClusterFieldStatistics},
		})
		if err != nil {
			truncated = true
			continue
		}

		for _, cluster := range out.Clusters {
			name := ""
			if cluster.ClusterName != nil {
				name = *cluster.ClusterName
			}
			if name == "" {
				continue
			}

			pending := cluster.PendingTasksCount
			running := cluster.RunningTasksCount
			registered := cluster.RegisteredContainerInstancesCount

			var rows []resource.FindingRow
			var summaries []string

			if pending > 0 {
				rows = append(rows, resource.FindingRow{
					Label: "Pending Tasks",
					Value: fmt.Sprintf("%d tasks pending", pending),
					Tier:  "~",
				})
				summaries = append(summaries, fmt.Sprintf("%d pending tasks", pending))
			}

			if running == 0 && registered > 0 {
				rows = append(rows, resource.FindingRow{
					Label: "Tasks",
					Value: fmt.Sprintf("no running tasks (%d container instances registered)", registered),
					Tier:  "~",
				})
				summaries = append(summaries, "no running tasks but instances registered")
			}

			if len(rows) == 0 {
				continue
			}

			summary := strings.Join(summaries, "; ")
			findings[name] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  summary,
				Rows:     rows,
			}
		}
	}

	// IssueCount is 0: all ECS cluster findings are "~" (informational) and
	// do not contribute to the attention menu badge.
	return EnricherResult{IssueCount: 0, Truncated: truncated, Findings: findings}, nil
}

// EnrichECSTasks is a Wave 2 enricher for ECS tasks.
// It groups tasks by cluster ARN and calls DescribeTasks (up to 100 per call)
// to surface failures that Wave 1 status coloring cannot detect.
//
// Findings raised (severity "!"):
//   - StopCode == TaskFailedToStart → task never launched
//   - StopCode == EssentialContainerExited → essential container died
//   - Any container with a non-zero ExitCode → container crash detected
func EnrichECSTasks(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.ECS == nil || len(resources) == 0 {
		return EnricherResult{Findings: findings}, nil
	}

	// Group task ARNs by cluster ARN.
	clusterTasks := make(map[string][]string)
	taskIDToResource := make(map[string]string) // taskID → resource key (task_id field)
	for _, r := range resources {
		cluster := r.Fields["cluster"]
		taskID := r.Fields["task_id"]
		if cluster == "" || taskID == "" {
			continue
		}
		// Reconstruct task ARN from cluster and task ID (task_id is the last segment).
		// We need to find the full ARN — use the cluster ARN stored in the field.
		// The cluster field stores the full cluster ARN from the fetcher.
		clusterTasks[cluster] = append(clusterTasks[cluster], taskID)
		taskIDToResource[taskID] = taskID
	}

	truncated := len(resources) > EnrichmentCap
	checked := 0

	// DescribeTasks accepts up to 100 task ARNs per call.
	const descBatch = 100
	for clusterARN, taskIDs := range clusterTasks {
		for i := 0; i < len(taskIDs); i += descBatch {
			if checked >= EnrichmentCap {
				truncated = true
				break
			}
			end := min(i+descBatch, len(taskIDs))
			batch := taskIDs[i:end]
			checked += len(batch)

			out, err := clients.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: aws.String(clusterARN),
				Tasks:   batch,
			})
			if err != nil {
				truncated = true
				continue
			}

			for _, task := range out.Tasks {
				// Identify the resource by task ID (last segment of ARN).
				taskID := ""
				if task.TaskArn != nil {
					parts := strings.Split(*task.TaskArn, "/")
					taskID = parts[len(parts)-1]
				}
				if taskID == "" {
					continue
				}

				var rows []resource.FindingRow

				// Check stop code for known failure modes.
				switch task.StopCode {
				case ecstypes.TaskStopCodeTaskFailedToStart:
					rows = append(rows, resource.FindingRow{
						Label: "Stop Code",
						Value: "TaskFailedToStart — task never launched",
						Tier:  "!",
					})
				case ecstypes.TaskStopCodeEssentialContainerExited:
					rows = append(rows, resource.FindingRow{
						Label: "Stop Code",
						Value: "EssentialContainerExited — essential container died",
						Tier:  "!",
					})
				}

				// Check containers for non-zero exit codes.
				for _, container := range task.Containers {
					if container.ExitCode != nil && *container.ExitCode != 0 {
						name := ""
						if container.Name != nil {
							name = *container.Name
						}
						rows = append(rows, resource.FindingRow{
							Label: "Container",
							Value: fmt.Sprintf("%s exited with code %d", name, *container.ExitCode),
							Tier:  "!",
						})
						break // One finding per task is sufficient.
					}
				}

				if len(rows) == 0 {
					continue
				}

				summary := rows[0].Value
				findings[taskID] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  summary,
					Rows:     rows,
				}
			}
		}
	}

	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichEventBridgeRuleTargets is a Wave 2 enricher for EventBridge rules.
// Per rule (cap 50) it calls ListTargetsByRule and raises findings for:
//   - Rule state == ENABLED AND len(Targets) == 0 → "!" finding (rule matches but goes nowhere)
//   - Rule state == DISABLED AND len(Targets) > 0 → "~" finding (disabled rule still has targets — drift)
//   - Any target without DeadLetterConfig → "~" finding (no DLQ on target)
func EnrichEventBridgeRuleTargets(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.EventBridge == nil || len(resources) == 0 {
		return EnricherResult{Findings: findings}, nil
	}

	truncated := len(resources) > EnrichmentCap
	checked := 0

	for _, r := range resources {
		if checked >= EnrichmentCap {
			truncated = true
			break
		}

		ruleName := r.Fields["name"]
		if ruleName == "" {
			ruleName = r.ID
		}
		if ruleName == "" {
			continue
		}

		eventBus := r.Fields["event_bus"]
		state := strings.ToUpper(r.Fields["state"])

		input := &eventbridge.ListTargetsByRuleInput{
			Rule: aws.String(ruleName),
		}
		if eventBus != "" {
			input.EventBusName = aws.String(eventBus)
		}

		out, err := clients.EventBridge.ListTargetsByRule(ctx, input)
		checked++
		if err != nil {
			truncated = true
			continue
		}

		targets := out.Targets
		var rows []resource.FindingRow

		// ENABLED rule with no targets → rule fires but goes nowhere.
		if state == "ENABLED" && len(targets) == 0 {
			rows = append(rows, resource.FindingRow{
				Label: "Targets",
				Value: "enabled rule has no targets (rule matches but goes nowhere)",
				Tier:  "!",
			})
		}

		// DISABLED rule still has targets → probable drift/oversight.
		if state == "DISABLED" && len(targets) > 0 {
			rows = append(rows, resource.FindingRow{
				Label: "Targets",
				Value: fmt.Sprintf("disabled rule still has %d target(s) (drift)", len(targets)),
				Tier:  "~",
			})
		}

		// Targets without DeadLetterConfig → missing DLQ.
		for _, target := range targets {
			if target.DeadLetterConfig == nil {
				targetID := ""
				if target.Id != nil {
					targetID = *target.Id
				}
				rows = append(rows, resource.FindingRow{
					Label: "Target",
					Value: fmt.Sprintf("%s: no dead-letter config", targetID),
					Tier:  "~",
				})
			}
		}

		if len(rows) == 0 {
			continue
		}

		// Determine severity: "!" if any row is "!", otherwise "~".
		severity := "~"
		for _, row := range rows {
			if row.Tier == "!" {
				severity = "!"
				break
			}
		}

		findings[ruleName] = resource.EnrichmentFinding{
			Severity: severity,
			Summary:  rows[0].Value,
			Rows:     rows,
		}
	}

	issueCount := 0
	for _, f := range findings {
		if f.Severity == "!" {
			issueCount++
		}
	}

	return EnricherResult{IssueCount: issueCount, Truncated: truncated, Findings: findings}, nil
}

// EnrichEBEnvironmentHealth calls DescribeEnvironmentHealth for each Elastic
// Beanstalk environment (1 per environment, cap 50). Returns an informational
// "~" finding for each environment with a non-empty Causes slice.
// Summary: "EB causes: <first cause>". IssueCount is always 0 — causes are
// informational signals, not broken-state indicators.
func EnrichEBEnvironmentHealth(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.ElasticBeanstalk == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		name := r.Name
		if name == "" {
			name = r.Fields["environment_name"]
		}
		if name == "" {
			continue
		}
		out, err := clients.ElasticBeanstalk.DescribeEnvironmentHealth(ctx, &elasticbeanstalk.DescribeEnvironmentHealthInput{
			EnvironmentName: aws.String(name),
			AttributeNames:  []ebtypes.EnvironmentHealthAttribute{ebtypes.EnvironmentHealthAttributeCauses},
		})
		if err != nil {
			truncated = true
			continue
		}
		if len(out.Causes) == 0 {
			continue
		}
		firstCause := out.Causes[0]
		rows := []resource.FindingRow{
			{Label: "Cause", Value: firstCause, Tier: "~"},
		}
		// Record additional causes as extra rows.
		for _, cause := range out.Causes[1:] {
			rows = append(rows, resource.FindingRow{Label: "Cause", Value: cause, Tier: "~"})
		}
		// Key on resource ID (environment ID) for registry consistency.
		// Fall back to name if ID is not set.
		key := r.ID
		if key == "" {
			key = name
		}
		findings[key] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  fmt.Sprintf("EB causes: %s", firstCause),
			Rows:     rows,
		}
	}
	return EnricherResult{IssueCount: 0, Truncated: truncated, Findings: findings}, nil
}

// EnrichELBAttributes calls DescribeLoadBalancerAttributes for each load
// balancer (1 per LB, cap 50) and returns an informational "~" finding for
// each LB missing deletion protection or access logging.
// The worst finding per LB is promoted to "!" if both attributes are missing;
// otherwise "~" is used. IssueCount counts findings with Severity "!".
func EnrichELBAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.ELBv2 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		if r.ID == "" {
			continue
		}
		out, err := clients.ELBv2.DescribeLoadBalancerAttributes(ctx, &elasticloadbalancingv2.DescribeLoadBalancerAttributesInput{
			LoadBalancerArn: aws.String(r.ID),
		})
		if err != nil {
			truncated = true
			continue
		}
		var rows []resource.FindingRow
		for _, attr := range out.Attributes {
			if attr.Key == nil || attr.Value == nil {
				continue
			}
			switch *attr.Key {
			case "deletion_protection.enabled":
				if *attr.Value == "false" {
					rows = append(rows, resource.FindingRow{Label: "Deletion Protection", Value: "disabled", Tier: "~"})
				}
			case "access_logs.s3.enabled":
				if *attr.Value == "false" {
					rows = append(rows, resource.FindingRow{Label: "Access Logs", Value: "disabled", Tier: "~"})
				}
			}
		}
		if len(rows) == 0 {
			continue
		}
		// Severity is "~" for each individual finding; promote to "!" only
		// when both misconfiguration flags are present simultaneously.
		severity := "~"
		if len(rows) >= 2 {
			severity = "!"
		}
		findings[r.ID] = resource.EnrichmentFinding{
			Severity: severity,
			Summary:  rows[0].Label + ": " + rows[0].Value,
			Rows:     rows,
		}
	}
	issueCount := 0
	for _, f := range findings {
		if f.Severity == "!" {
			issueCount++
		}
	}
	return EnricherResult{IssueCount: issueCount, Truncated: truncated, Findings: findings}, nil
}

// EnrichSQSAttributes calls GetQueueAttributes per queue (cap EnrichmentCap)
// to surface missing DLQ and missing KMS encryption as Wave 2 findings.
// Per-queue errors are treated as truncated (skip silently).
func EnrichSQSAttributes(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.SQS == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		queueURL := r.Fields["queue_url"]
		if queueURL == "" {
			continue
		}
		out, err := clients.SQS.GetQueueAttributes(ctx, &sqssvc.GetQueueAttributesInput{
			QueueUrl: aws.String(queueURL),
			AttributeNames: []sqstypes.QueueAttributeName{
				sqstypes.QueueAttributeNameRedrivePolicy,
				sqstypes.QueueAttributeNameVisibilityTimeout,
				sqstypes.QueueAttributeNameKmsMasterKeyId,
			},
		})
		if err != nil {
			truncated = true
			continue
		}
		var rows []resource.FindingRow
		if _, ok := out.Attributes["RedrivePolicy"]; !ok {
			rows = append(rows, resource.FindingRow{
				Label: "DLQ",
				Value: "no DLQ configured",
				Tier:  "~",
			})
		}
		if _, ok := out.Attributes["KmsMasterKeyId"]; !ok {
			rows = append(rows, resource.FindingRow{
				Label: "Encryption",
				Value: "no KMS encryption configured",
				Tier:  "~",
			})
		}
		if len(rows) == 0 {
			continue
		}
		findings[r.ID] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  rows[0].Value,
			Rows:     rows,
		}
	}
	return EnricherResult{IssueCount: 0, Truncated: truncated, Findings: findings}, nil
}

// EnrichSNSSubscriptions calls ListSubscriptionsByTopic per topic (cap EnrichmentCap)
// to surface orphan topics and topics with all-pending-confirmation subscribers.
// Per-topic errors are treated as truncated (skip silently).
func EnrichSNSSubscriptions(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.SNS == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		topicARN := r.ID
		if topicARN == "" {
			continue
		}
		out, err := clients.SNS.ListSubscriptionsByTopic(ctx, &snssvc.ListSubscriptionsByTopicInput{
			TopicArn: aws.String(topicARN),
		})
		if err != nil {
			truncated = true
			continue
		}
		subs := out.Subscriptions
		if len(subs) == 0 {
			findings[r.ID] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "topic has no subscribers",
				Rows: []resource.FindingRow{
					{Label: "Subscribers", Value: "topic has no subscribers", Tier: "~"},
				},
			}
			continue
		}
		allPending := true
		for _, sub := range subs {
			arn := ""
			if sub.SubscriptionArn != nil {
				arn = *sub.SubscriptionArn
			}
			if arn != "PendingConfirmation" {
				allPending = false
				break
			}
		}
		if allPending {
			findings[r.ID] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "all pending confirmation",
				Rows: []resource.FindingRow{
					{Label: "Subscribers", Value: "all pending confirmation", Tier: "~"},
				},
			}
		}
	}
	return EnricherResult{IssueCount: 0, Truncated: truncated, Findings: findings}, nil
}

// EnrichMSKCluster calls DescribeClusterV2 per provisioned MSK cluster (cap EnrichmentCap)
// and raises findings for:
//   - Broker software version below 2.8 (major.minor) → "~" "broker software outdated"
//   - EncryptionInTransit.ClientBroker not "TLS" → "~" "encryption in transit not enforced"
//
// Serverless clusters (Provisioned==nil) are skipped.
// Skip if clients.MSK == nil. Per-cluster errors → Truncated.
func EnrichMSKCluster(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.MSK == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		clusterARN := r.ID
		if clusterARN == "" {
			continue
		}
		out, err := clients.MSK.DescribeClusterV2(ctx, &kafkasvc.DescribeClusterV2Input{
			ClusterArn: aws.String(clusterARN),
		})
		if err != nil {
			truncated = true
			continue
		}
		if out.ClusterInfo == nil {
			continue
		}
		prov := out.ClusterInfo.Provisioned
		if prov == nil {
			// Serverless cluster — skip checks.
			continue
		}
		// Check broker software version.
		if prov.CurrentBrokerSoftwareInfo != nil && prov.CurrentBrokerSoftwareInfo.KafkaVersion != nil {
			if isMSKVersionOutdated(*prov.CurrentBrokerSoftwareInfo.KafkaVersion) {
				findings[clusterARN] = resource.EnrichmentFinding{
					Severity: "~",
					Summary:  "broker software outdated",
				}
			}
		}
		// Check encryption in transit (only set finding if not already set).
		if _, alreadyFound := findings[clusterARN]; !alreadyFound {
			if prov.EncryptionInfo != nil &&
				prov.EncryptionInfo.EncryptionInTransit != nil &&
				prov.EncryptionInfo.EncryptionInTransit.ClientBroker != kafkatypes.ClientBrokerTls {
				findings[clusterARN] = resource.EnrichmentFinding{
					Severity: "~",
					Summary:  "encryption in transit not enforced",
				}
			}
		}
	}
	// All MSK findings are severity "~" (informational) and do not contribute to the
	// attention menu badge. IssueCount is always 0 for this enricher.
	return EnricherResult{IssueCount: 0, Truncated: truncated, Findings: findings}, nil
}

// isMSKVersionOutdated returns true when the given Kafka version string is below the
// conservative current cutoff of 2.8 (major.minor). Versions that cannot be parsed
// are treated as up-to-date (safe default — do not produce false-positive findings).
func isMSKVersionOutdated(version string) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}
	major, err := parseVersionPart(parts[0])
	if err != nil {
		return false
	}
	minor, err := parseVersionPart(parts[1])
	if err != nil {
		return false
	}
	// Current cutoff: 2.8. Anything with major < 2 or (major == 2 && minor < 8) is outdated.
	return major < 2 || (major == 2 && minor < 8)
}

// parseVersionPart parses a numeric version component, returning an error for non-numeric input.
func parseVersionPart(s string) (int, error) {
	val := 0
	if len(s) == 0 {
		return 0, fmt.Errorf("empty version part")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-numeric version part: %q", s)
		}
		val = val*10 + int(c-'0')
	}
	return val, nil
}

// EnrichACMCertificate calls DescribeCertificate per ACM certificate (cap EnrichmentCap)
// and raises findings for:
//   - NotAfter within 30 days → "!" finding "expires in <N> days" (or "expired" if past)
//   - ISSUED certificate with no InUseBy entries → "~" finding "certificate not in use (orphan)"
//
// IssueCount counts only "!" severity findings — "~" (informational) are excluded from the badge.
// Skip if clients.ACM == nil. Per-cert errors → Truncated.
func EnrichACMCertificate(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.ACM == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	now := time.Now()
	bangCount := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		certARN := r.ID
		if certARN == "" {
			continue
		}
		out, err := clients.ACM.DescribeCertificate(ctx, &acmsvc.DescribeCertificateInput{
			CertificateArn: aws.String(certARN),
		})
		if err != nil {
			truncated = true
			continue
		}
		if out.Certificate == nil {
			continue
		}
		cert := out.Certificate
		// Expiry check — takes priority over orphan check.
		if cert.NotAfter != nil {
			remaining := cert.NotAfter.Sub(now)
			const expiryWindow = 30 * 24 * time.Hour
			if remaining < expiryWindow {
				var summary string
				if remaining < 0 {
					summary = "expired"
				} else {
					days := int(remaining.Hours() / 24)
					summary = fmt.Sprintf("expires in %d days", days)
				}
				findings[certARN] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  summary,
				}
				bangCount++
				continue
			}
		}
		// Orphan check — only for ISSUED certs not already flagged.
		if cert.Status == acmtypes.CertificateStatusIssued && len(cert.InUseBy) == 0 {
			findings[certARN] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  "certificate not in use (orphan)",
			}
			// "~" is informational — not counted in IssueCount.
		}
	}
	return EnricherResult{IssueCount: bangCount, Truncated: truncated, Findings: findings}, nil
}

// EnrichCloudFrontDistribution calls GetDistributionConfig per distribution (cap EnrichmentCap)
// and returns a Finding for any distribution with insecure viewer or origin protocol settings.
//
// Findings (severity "~" — informational):
//   - DefaultCacheBehavior.ViewerProtocolPolicy == "allow-all" → "no HTTPS redirect (insecure)"
//   - Any Origin with CustomOriginConfig.OriginProtocolPolicy == "http-only" → "origin without TLS"
//
// Skip if clients.CloudFront == nil. Per-distribution errors → truncated.
func EnrichCloudFrontDistribution(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.CloudFront == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		distID := r.ID
		if distID == "" {
			continue
		}
		out, err := clients.CloudFront.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{
			Id: aws.String(distID),
		})
		if err != nil {
			truncated = true
			continue
		}
		if out.DistributionConfig == nil {
			continue
		}
		cfg := out.DistributionConfig
		var rows []resource.FindingRow
		var summaries []string

		// Check viewer protocol policy on default cache behavior.
		if cfg.DefaultCacheBehavior != nil &&
			cfg.DefaultCacheBehavior.ViewerProtocolPolicy == cftypes.ViewerProtocolPolicyAllowAll {
			summaries = append(summaries, "no HTTPS redirect (insecure)")
			rows = append(rows, resource.FindingRow{
				Label: "ViewerProtocolPolicy",
				Value: "allow-all",
				Tier:  "~",
			})
		}

		// Check origin protocol policies.
		if cfg.Origins != nil {
			for _, origin := range cfg.Origins.Items {
				if origin.CustomOriginConfig != nil &&
					origin.CustomOriginConfig.OriginProtocolPolicy == cftypes.OriginProtocolPolicyHttpOnly {
					originID := ""
					if origin.Id != nil {
						originID = *origin.Id
					}
					summaries = append(summaries, "origin without TLS")
					rows = append(rows, resource.FindingRow{
						Label: "Origin",
						Value: originID,
						Tier:  "~",
					})
					rows = append(rows, resource.FindingRow{
						Label: "OriginProtocolPolicy",
						Value: "http-only",
						Tier:  "~",
					})
				}
			}
		}

		if len(summaries) == 0 {
			continue
		}
		summary := strings.Join(summaries, "; ")
		findings[distID] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  summary,
			Rows:     rows,
		}
	}
	// All CloudFront findings are severity "~" (informational).
	// IssueCount counts only "!" severity findings; "~" do not contribute.
	return EnricherResult{IssueCount: 0, Truncated: truncated, Findings: findings}, nil
}

// EnrichAPIGatewayStage calls GetStages per API (cap EnrichmentCap)
// and returns a Finding for any API with stage-level throttling or access-log issues.
//
// Findings (severity "~" — informational):
//   - Any stage with DefaultRouteSettings.ThrottlingBurstLimit == 0 OR ThrottlingRateLimit == 0
//     → "no throttling configured (DoS risk)"
//   - Any stage with AccessLogSettings == nil → "access logs disabled"
//
// Findings are aggregated per API (one finding per API, covering all stages).
// Skip if clients.APIGatewayV2 == nil. Per-API errors → truncated.
func EnrichAPIGatewayStage(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.APIGatewayV2 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		apiID := r.ID
		if apiID == "" {
			continue
		}
		out, err := clients.APIGatewayV2.GetStages(ctx, &apigatewayv2.GetStagesInput{
			ApiId: aws.String(apiID),
		})
		if err != nil {
			truncated = true
			continue
		}
		var summaries []string
		var rows []resource.FindingRow

		for _, stage := range out.Items {
			stageName := stage.StageName
			if stageName == nil {
				stageName = aws.String("(unnamed)")
			}

			// Check throttling on DefaultRouteSettings.
			if drs := stage.DefaultRouteSettings; drs != nil {
				noThrottle := (drs.ThrottlingBurstLimit != nil && *drs.ThrottlingBurstLimit == 0) ||
					(drs.ThrottlingRateLimit != nil && *drs.ThrottlingRateLimit == 0)
				if noThrottle {
					summaries = append(summaries, "no throttling configured (DoS risk)")
					rows = append(rows, resource.FindingRow{
						Label: "Stage",
						Value: *stageName,
						Tier:  "~",
					})
					rows = append(rows, resource.FindingRow{
						Label: "Issue",
						Value: "no throttling configured (DoS risk)",
						Tier:  "~",
					})
				}
			}

			// Check access log settings.
			if stage.AccessLogSettings == nil {
				summaries = append(summaries, "access logs disabled")
				rows = append(rows, resource.FindingRow{
					Label: "Stage",
					Value: *stageName,
					Tier:  "~",
				})
				rows = append(rows, resource.FindingRow{
					Label: "Issue",
					Value: "access logs disabled",
					Tier:  "~",
				})
			}
		}

		if len(summaries) == 0 {
			continue
		}
		// Deduplicate repeated summary messages.
		seen := make(map[string]bool)
		var uniqueSummaries []string
		for _, s := range summaries {
			if !seen[s] {
				seen[s] = true
				uniqueSummaries = append(uniqueSummaries, s)
			}
		}
		findings[apiID] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  strings.Join(uniqueSummaries, "; "),
			Rows:     rows,
		}
	}
	// All API Gateway findings are severity "~" (informational).
	// IssueCount counts only "!" severity findings; "~" do not contribute.
	return EnricherResult{IssueCount: 0, Truncated: truncated, Findings: findings}, nil
}

// EnrichCFNStackEvents calls DescribeStackEvents for each stack (first page only,
// up to EnrichmentCap stacks). It scans the most recent events client-side for any
// resource with ResourceStatus ending in "_FAILED". A failed resource event produces
// a "!" finding: "recent resource failure: <ResourceType>/<LogicalResourceId>".
// This surfaces hidden failures that are not reflected in the top-level StackStatus.
func EnrichCFNStackEvents(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.CloudFormation == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		stackName := r.Fields["stack_name"]
		if stackName == "" {
			stackName = r.ID
		}
		if stackName == "" {
			continue
		}
		out, err := clients.CloudFormation.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
			StackName: aws.String(stackName),
		})
		if err != nil {
			truncated = true
			continue
		}
		// Scan events from the first page for any resource with a _FAILED status.
		// The API returns events in reverse-chronological order; we inspect all
		// events on the first page to catch recent failures.
		var failedRows []resource.FindingRow
		for _, ev := range out.StackEvents {
			status := string(ev.ResourceStatus)
			if !strings.HasSuffix(status, "_FAILED") {
				continue
			}
			logicalID := ""
			if ev.LogicalResourceId != nil {
				logicalID = *ev.LogicalResourceId
			}
			resourceType := ""
			if ev.ResourceType != nil {
				resourceType = *ev.ResourceType
			}
			reason := ""
			if ev.ResourceStatusReason != nil {
				reason = *ev.ResourceStatusReason
			}
			label := resourceType
			if label == "" {
				label = logicalID
			} else if logicalID != "" {
				label = resourceType + "/" + logicalID
			}
			row := resource.FindingRow{Label: label, Value: status, Tier: "!"}
			failedRows = append(failedRows, row)
			if reason != "" {
				failedRows = append(failedRows, resource.FindingRow{Label: "Reason", Value: reason, Tier: "~"})
			}
		}
		if len(failedRows) == 0 {
			continue
		}
		key := r.ID
		if key == "" {
			key = stackName
		}
		findings[key] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  fmt.Sprintf("recent resource failure: %s", failedRows[0].Label),
			Rows:     failedRows,
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichCFNDrift calls DescribeStacks per stack (up to EnrichmentCap stacks) to
// read DriftInformation.StackDriftStatus. A status of DRIFTED produces a "~" finding
// "stack drifted from template". IN_SYNC and NOT_CHECKED stacks produce no finding.
// Severity "~" findings do not contribute to IssueCount.
func EnrichCFNDrift(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.CloudFormation == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		stackName := r.Fields["stack_name"]
		if stackName == "" {
			stackName = r.ID
		}
		if stackName == "" {
			continue
		}
		out, err := clients.CloudFormation.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
			StackName: aws.String(stackName),
		})
		if err != nil {
			truncated = true
			continue
		}
		if len(out.Stacks) == 0 {
			continue
		}
		stack := out.Stacks[0]
		if stack.DriftInformation == nil {
			continue
		}
		if stack.DriftInformation.StackDriftStatus != "DRIFTED" {
			continue
		}
		key := r.ID
		if key == "" {
			key = stackName
		}
		findings[key] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  "stack drifted from template",
			Rows: []resource.FindingRow{
				{Label: "Drift Status", Value: string(stack.DriftInformation.StackDriftStatus), Tier: "~"},
			},
		}
	}
	// "~" findings do not contribute to IssueCount per the EnricherResult contract.
	return EnricherResult{IssueCount: 0, Truncated: truncated, Findings: findings}, nil
}

// EnrichECRRepository calls DescribeImageScanFindings for each ECR repository (up to
// EnrichmentCap repos). It reads ImageScanFindings.FindingSeverityCounts directly:
//   - CRITICAL > 0 → "!" finding "critical vulnerabilities: <count>"
//   - HIGH > 0 (and no CRITICAL) → "~" finding "high vulnerabilities: <count>"
//
// ScanNotFoundException (scan never ran) is silently skipped — Truncated is NOT set.
// Any other API error causes the repo to be skipped with Truncated=true.
func EnrichECRRepository(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.ECR == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	issueCount := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		repoName := r.Fields["repository_name"]
		if repoName == "" {
			repoName = r.ID
		}
		if repoName == "" {
			continue
		}
		out, err := clients.ECR.DescribeImageScanFindings(ctx, &ecrsvc.DescribeImageScanFindingsInput{
			RepositoryName: aws.String(repoName),
		})
		if err != nil {
			// ScanNotFoundException means the scan has never run — normal operational
			// state, silently skip without marking Truncated.
			var scanNotFound *ecrtypes.ScanNotFoundException
			if errors.As(err, &scanNotFound) {
				continue
			}
			truncated = true
			continue
		}
		if out.ImageScanFindings == nil {
			continue
		}
		counts := out.ImageScanFindings.FindingSeverityCounts
		critical := counts["CRITICAL"]
		high := counts["HIGH"]
		if critical == 0 && high == 0 {
			continue
		}
		key := r.ID
		if key == "" {
			key = repoName
		}
		if critical > 0 {
			rows := []resource.FindingRow{
				{Label: "CRITICAL", Value: fmt.Sprintf("%d", critical), Tier: "!"},
			}
			if high > 0 {
				rows = append(rows, resource.FindingRow{Label: "HIGH", Value: fmt.Sprintf("%d", high), Tier: "~"})
			}
			findings[key] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  fmt.Sprintf("critical vulnerabilities: %d", critical),
				Rows:     rows,
			}
			issueCount++
		} else {
			// high > 0, critical == 0
			rows := []resource.FindingRow{
				{Label: "HIGH", Value: fmt.Sprintf("%d", high), Tier: "~"},
			}
			findings[key] = resource.EnrichmentFinding{
				Severity: "~",
				Summary:  fmt.Sprintf("high vulnerabilities: %d", high),
				Rows:     rows,
			}
			// "~" severity does not contribute to IssueCount.
		}
	}
	return EnricherResult{IssueCount: issueCount, Truncated: truncated, Findings: findings}, nil
}

// EnrichCodeArtifactRepository calls GetRepositoryPermissionsPolicy per repository (capped at
// EnrichmentCap) to surface IAM policy findings.
//
// Findings:
//   - ResourceNotFoundException → "~" severity, "no permissions policy" (default open within domain).
//   - Policy.Document contains `"Principal":"*"` → "!" severity, "public access policy".
//
// Per-repo errors other than ResourceNotFoundException mark Truncated=true and are skipped.
// Skip when clients.CodeArtifact == nil.
func EnrichCodeArtifactRepository(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.CodeArtifact == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	issueCount := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		// Support both "repo_name" (fetcher canonical) and "repository_name" (legacy/test alias).
		repoName := r.Fields["repo_name"]
		if repoName == "" {
			repoName = r.Fields["repository_name"]
		}
		if repoName == "" {
			repoName = r.ID
		}
		// Support both "domain_name" (fetcher canonical) and "domain" (legacy/test alias).
		domainName := r.Fields["domain_name"]
		if domainName == "" {
			domainName = r.Fields["domain"]
		}
		domainOwner := r.Fields["domain_owner"]
		if repoName == "" || domainName == "" {
			continue
		}
		input := &codeartifact.GetRepositoryPermissionsPolicyInput{
			Domain:     aws.String(domainName),
			Repository: aws.String(repoName),
		}
		if domainOwner != "" {
			input.DomainOwner = aws.String(domainOwner)
		}
		out, err := clients.CodeArtifact.GetRepositoryPermissionsPolicy(ctx, input)
		key := r.ID
		if key == "" {
			key = repoName
		}
		if err != nil {
			var notFound *codeartifacttypes.ResourceNotFoundException
			if errors.As(err, &notFound) {
				// No policy set — default open within the domain.
				findings[key] = resource.EnrichmentFinding{
					Severity: "~",
					Summary:  "no permissions policy",
				}
				// "~" does not contribute to IssueCount.
				continue
			}
			// Any other error — skip this repo but flag truncation.
			truncated = true
			continue
		}
		if out.Policy == nil || out.Policy.Document == nil {
			continue
		}
		doc := *out.Policy.Document
		if strings.Contains(doc, `"Principal":"*"`) || strings.Contains(doc, `"Principal": "*"`) {
			findings[key] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  "public access policy",
				Rows: []resource.FindingRow{
					{Label: "Principal", Value: "*", Tier: "!"},
				},
			}
			issueCount++
		}
	}
	return EnricherResult{IssueCount: issueCount, Truncated: truncated, Findings: findings}, nil
}

// EnrichAthenaWorkGroup calls GetWorkGroup per workgroup (capped at EnrichmentCap) to
// surface governance and security findings.
//
// Findings:
//   - WorkGroup.Configuration.EnforceWorkGroupConfiguration == false → "~" severity,
//     "EnforceWorkGroupConfiguration disabled (callers can bypass)".
//   - WorkGroup.Configuration.ResultConfiguration.EncryptionConfiguration == nil → "~" severity,
//     "result encryption not configured".
//
// Per-WG errors mark Truncated=true and are skipped.
// Skip when clients.Athena == nil.
func EnrichAthenaWorkGroup(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.Athena == nil {
		return EnricherResult{Findings: findings}, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		wgName := r.Fields["workgroup_name"]
		if wgName == "" {
			wgName = r.ID
		}
		if wgName == "" {
			continue
		}
		out, err := clients.Athena.GetWorkGroup(ctx, &athena.GetWorkGroupInput{
			WorkGroup: aws.String(wgName),
		})
		if err != nil {
			truncated = true
			continue
		}
		if out.WorkGroup == nil || out.WorkGroup.Configuration == nil {
			continue
		}
		cfg := out.WorkGroup.Configuration
		key := r.ID
		if key == "" {
			key = wgName
		}
		var rows []resource.FindingRow
		// EnforceWorkGroupConfiguration defaults to true; false means callers can bypass settings.
		if cfg.EnforceWorkGroupConfiguration != nil && !*cfg.EnforceWorkGroupConfiguration {
			rows = append(rows, resource.FindingRow{
				Label: "EnforceWorkGroupConfiguration",
				Value: "false",
				Tier:  "~",
			})
		}
		// Missing encryption on result configuration is a security concern.
		if cfg.ResultConfiguration == nil || cfg.ResultConfiguration.EncryptionConfiguration == nil {
			rows = append(rows, resource.FindingRow{
				Label: "ResultConfiguration.EncryptionConfiguration",
				Value: "nil",
				Tier:  "~",
			})
		}
		if len(rows) == 0 {
			continue
		}
		summary := rows[0].Label
		if len(rows) > 1 {
			summary = fmt.Sprintf("%s (%d findings)", rows[0].Label, len(rows))
		}
		findings[key] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  summary,
			Rows:     rows,
		}
		// "~" severity does not contribute to IssueCount.
	}
	return EnricherResult{Truncated: truncated, Findings: findings}, nil
}
