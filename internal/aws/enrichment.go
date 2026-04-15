// Package aws provides AWS service clients and resource fetchers.
// This file implements Wave 2 enrichment functions for issue #196.
// Each enricher makes additional API calls to discover hidden issues
// that Wave 1's status-based counting cannot detect.
package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"

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
	"ebs":      EnrichEBSVolumeStatus,
	"cb":       EnrichCodeBuildStatus,
	"tg":       EnrichTargetGroupHealth,
	"pipeline": EnrichCodePipelineStatus,
	"sfn":      EnrichStepFunctionsStatus,
	"glue":     EnrichGlueJobStatus,
	// Wave 2 = None per docs/attention-signals.md — explicit no-op registration
	// makes the empty Wave 2 contract testable.
	"ami":     NoOpEnricher,
	"ebs-snap": NoOpEnricher,
	"lambda":  NoOpEnricher,
	"sns-sub": NoOpEnricher,
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
