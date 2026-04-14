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
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// EnricherResult is the typed return value of a Wave 2 enricher.
//
//   - IssueCount is the number of resources the enricher classifies as issue-worthy
//     for the menu badge. Severity "!" findings contribute; severity "~" findings
//     (informational) do NOT contribute to IssueCount.
//   - Truncated is true when the enricher only inspected a subset (e.g., capped at
//     EnrichmentCap) so the count is a lower bound.
//   - Findings is a map from resource.Resource.ID → EnrichmentFinding for every
//     affected resource the enricher observed. For account-wide enrichers (RDS,
//     EC2 status checks, EBS), Findings may contain entries for resources that
//     are NOT in the input `resources` slice — banner derivation uses this
//     information. Enrichers that receive API identifiers in a different form
//     (e.g., ARNs) MUST normalize to Resource.ID; if no match can be determined,
//     the affected resource is skipped silently.
//     MAY be empty when no issues are found. MUST NOT be nil on success — use
//     make(map[string]resource.EnrichmentFinding) for the empty case.
type EnricherResult struct {
	IssueCount int
	Truncated  bool
	Findings   map[string]resource.EnrichmentFinding
}

// EnricherFunc is a pluggable function that makes additional API calls for a
// resource type and returns a typed EnricherResult. The resources slice contains
// retained first-page resources from Wave 1 probes.
type EnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error)

// EnricherRegistry maps resource short names to their Wave 2 enricher functions.
// Ordered by priority: batchable (cheap) first, per-resource (expensive) last.
var EnricherRegistry = map[string]EnricherFunc{
	"rds":  EnrichRDSDocDBMaintenance,
	"dbi":  EnrichRDSDocDBMaintenance,
	"ec2":  EnrichEC2StatusChecks,
	"ebs":  EnrichEBSVolumeStatus,
	"cb":   EnrichCodeBuildStatus,
	"tg":   EnrichTargetGroupHealth,
	"pipe": EnrichCodePipelineStatus,
	"ddb":  EnrichDynamoDBStatus,
	"sfn":  EnrichStepFunctionsStatus,
	"glue": EnrichGlueJobStatus,
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
	// Build a set of probed resource IDs for matching against ARN suffixes.
	probeIDs := make(map[string]bool, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			probeIDs[r.ID] = true
		}
	}
	// Emit a finding for every ARN that has pending maintenance.
	// Use the probed ID if it matches; otherwise fall back to the ARN suffix itself
	// (account-wide enrichers may surface off-page resources).
	for _, action := range out.PendingMaintenanceActions {
		if action.ResourceIdentifier == nil {
			continue
		}
		arn := *action.ResourceIdentifier
		// Collect action descriptions for the summary.
		var actions []string
		for _, pa := range action.PendingMaintenanceActionDetails {
			if pa.Action != nil {
				actions = append(actions, *pa.Action)
			}
		}
		summary := "pending maintenance"
		if len(actions) > 0 {
			summary = "pending maintenance: " + strings.Join(actions, ", ")
		}
		// Determine the key: prefer the matching probeID; fall back to ARN suffix.
		key := ""
		for id := range probeIDs {
			if strings.HasSuffix(arn, ":"+id) {
				key = id
				break
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
		}
	}
	// IssueCount is always 0 for RDS maintenance (severity "~" — excluded from menu badge).
	return EnricherResult{IssueCount: 0, Truncated: out.Marker != nil, Findings: findings}, nil
}

// EnrichEC2StatusChecks calls DescribeInstanceStatus (1 call, all instances)
// and returns a Finding for every instance with impaired system or instance status.
// Severity is "!" (broken/degraded).
func EnrichEC2StatusChecks(ctx context.Context, clients *ServiceClients, _ []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.EC2 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	out, err := clients.EC2.DescribeInstanceStatus(ctx, &ec2.DescribeInstanceStatusInput{
		IncludeAllInstances: aws.Bool(true),
	})
	if err != nil {
		return EnricherResult{}, err
	}
	for _, s := range out.InstanceStatuses {
		// Only count "impaired" — not "not-applicable" (stopped instances),
		// "insufficient-data" (recently launched), or "initializing".
		sysImpaired := s.SystemStatus != nil && s.SystemStatus.Status == ec2types.SummaryStatusImpaired
		instImpaired := s.InstanceStatus != nil && s.InstanceStatus.Status == ec2types.SummaryStatusImpaired
		if !sysImpaired && !instImpaired {
			continue
		}
		if s.InstanceId == nil {
			continue
		}
		// Prefer "system status impaired" when both are impaired.
		summary := "instance status impaired"
		if sysImpaired {
			summary = "system status impaired"
		}
		findings[*s.InstanceId] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  summary,
		}
	}
	// Paginated API — result may be truncated (lower bound).
	return EnricherResult{IssueCount: len(findings), Truncated: out.NextToken != nil, Findings: findings}, nil
}

// EnrichEBSVolumeStatus calls DescribeVolumeStatus (1 call, all volumes)
// and returns a Finding for every volume with non-ok status.
// Severity is "!" (broken/degraded).
func EnrichEBSVolumeStatus(ctx context.Context, clients *ServiceClients, _ []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.EC2 == nil {
		return EnricherResult{Findings: findings}, nil
	}
	out, err := clients.EC2.DescribeVolumeStatus(ctx, &ec2.DescribeVolumeStatusInput{})
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
		findings[*v.VolumeId] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  "volume I/O degraded",
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
	// Collect project names from resources (r.ID is the project name for cb).
	names := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			names = append(names, r.ID)
		}
	}
	if len(names) == 0 {
		return EnricherResult{Findings: findings}, nil
	}
	// ListBuildsForProject for each to get the latest build ID, then batch get.
	// Track the project name for each build ID so we can key the finding by r.ID.
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
		return EnricherResult{Findings: findings}, nil
	}
	builds, err := clients.CodeBuild.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{
		Ids: buildIDs,
	})
	if err != nil {
		return EnricherResult{}, err
	}
	for _, b := range builds.Builds {
		// Only terminal failures produce findings. In-flight (InProgress, Stopping)
		// and success are not user-actionable "issues".
		switch b.BuildStatus {
		case cbtypes.StatusTypeSucceeded, cbtypes.StatusTypeInProgress:
			continue
		}
		if b.Id == nil {
			continue
		}
		projectName := buildIDToProject[*b.Id]
		if projectName == "" {
			continue
		}
		summary := fmt.Sprintf("latest build %s", string(b.BuildStatus))
		if b.EndTime != nil {
			summary = fmt.Sprintf("latest build %s (%s)", string(b.BuildStatus), b.EndTime.Format("2006-01-02"))
		}
		findings[projectName] = resource.EnrichmentFinding{
			Severity: "!",
			Summary:  summary,
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
		for _, t := range out.TargetHealthDescriptions {
			if t.TargetHealth != nil && t.TargetHealth.State != elbtypes.TargetHealthStateEnumHealthy {
				unhealthy++
			}
		}
		if unhealthy > 0 {
			findings[r.ID] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  fmt.Sprintf("unhealthy targets: %d/%d", unhealthy, total),
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
			if stage.LatestExecution != nil && stage.LatestExecution.Status == "Failed" {
				stageName := ""
				if stage.StageName != nil {
					stageName = *stage.StageName
				}
				summary := fmt.Sprintf("stage %s failed", stageName)
				// Pipeline findings are keyed by r.Name (the pipeline name used for the
				// GetPipelineState API call), since that is the natural lookup key for
				// CodePipeline resources. Fall back to r.ID if Name is empty.
				key := r.Name
				if key == "" {
					key = r.ID
				}
				findings[key] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  summary,
				}
				break // first failed stage is sufficient
			}
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}

// EnrichDynamoDBStatus calls DescribeTable for each table (1 per table, cap ~50).
// Returns a Finding for each table with non-ACTIVE status or a non-ACTIVE GSI.
// Severity is "!" (broken/degraded).
// Summary: "table status: <status>" for the table itself, or "GSI <name> status: <status>" for a GSI.
func EnrichDynamoDBStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	if clients.DynamoDB == nil {
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
		out, err := clients.DynamoDB.DescribeTable(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(r.Name),
		})
		if err != nil {
			truncated = true
			continue
		}
		if out.Table == nil {
			continue
		}
		key := r.ID
		if key == "" {
			key = r.Name
		}
		if out.Table.TableStatus != dbtypes.TableStatusActive {
			findings[key] = resource.EnrichmentFinding{
				Severity: "!",
				Summary:  fmt.Sprintf("table status: %s", string(out.Table.TableStatus)),
			}
			continue
		}
		// Check GSIs — first non-ACTIVE GSI wins.
		for _, gsi := range out.Table.GlobalSecondaryIndexes {
			if gsi.IndexStatus != dbtypes.IndexStatusActive {
				gsiName := ""
				if gsi.IndexName != nil {
					gsiName = *gsi.IndexName
				}
				findings[key] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  fmt.Sprintf("GSI %s status: %s", gsiName, string(gsi.IndexStatus)),
				}
				break
			}
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
				findings[r.ID] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  fmt.Sprintf("latest execution %s", string(s)),
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
			s := out.JobRuns[0].JobRunState
			if s == gluetypes.JobRunStateFailed || s == gluetypes.JobRunStateError || s == gluetypes.JobRunStateTimeout {
				key := r.ID
				if key == "" {
					key = r.Name
				}
				findings[key] = resource.EnrichmentFinding{
					Severity: "!",
					Summary:  fmt.Sprintf("latest run %s", string(s)),
				}
			}
		}
	}
	return EnricherResult{IssueCount: len(findings), Truncated: truncated, Findings: findings}, nil
}
