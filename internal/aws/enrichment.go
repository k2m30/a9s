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
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	smithy "github.com/aws/smithy-go"

	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/efs"

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
