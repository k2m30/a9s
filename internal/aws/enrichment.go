// Package aws provides AWS service clients and resource fetchers.
// This file implements Wave 2 enrichment functions for issue #196.
// Each enricher makes additional API calls to discover hidden issues
// that Wave 1's status-based counting cannot detect.
package aws

import (
	"context"
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

// EnricherFunc is a pluggable function that makes additional API calls for a
// resource type and returns the count of resources with issues plus a truncated
// flag indicating whether the count is a lower bound (e.g., capped at EnrichmentCap).
// The resources slice contains retained first-page resources from Wave 1 probes.
type EnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (issueCount int, truncated bool, err error)

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

// EnrichRDSDocDBMaintenance calls DescribePendingMaintenanceActions (account-wide, 1 call)
// and counts only probed resources that have pending maintenance. The API returns
// maintenance actions for all RDS/DocDB resources (clusters AND instances), so we
// filter to only count resources matching the probed resource IDs.
func EnrichRDSDocDBMaintenance(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (int, bool, error) {
	if clients.RDS == nil {
		return 0, false, nil
	}
	out, err := clients.RDS.DescribePendingMaintenanceActions(ctx, &rds.DescribePendingMaintenanceActionsInput{})
	if err != nil {
		return 0, false, err
	}
	// Build a set of probed resource IDs for matching against ARN suffixes.
	probeIDs := make(map[string]bool, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			probeIDs[r.ID] = true
		}
	}
	// Count only maintenance actions whose resource ARN contains a probed ID.
	// ARN format: arn:aws:rds:region:account:db:instance-id (or :cluster:cluster-id)
	issues := 0
	for _, action := range out.PendingMaintenanceActions {
		if action.ResourceIdentifier == nil {
			continue
		}
		arn := *action.ResourceIdentifier
		// Extract the resource ID from the ARN (last segment after the last colon-delimited type)
		matched := false
		for id := range probeIDs {
			if strings.HasSuffix(arn, ":"+id) {
				matched = true
				break
			}
		}
		if matched {
			issues++
		}
	}
	truncated := out.Marker != nil
	return issues, truncated, nil
}

// EnrichEC2StatusChecks calls DescribeInstanceStatus (1 call, all instances)
// and counts instances with impaired system or instance status.
func EnrichEC2StatusChecks(ctx context.Context, clients *ServiceClients, _ []resource.Resource) (int, bool, error) {
	if clients.EC2 == nil {
		return 0, false, nil
	}
	out, err := clients.EC2.DescribeInstanceStatus(ctx, &ec2.DescribeInstanceStatusInput{
		IncludeAllInstances: aws.Bool(true),
	})
	if err != nil {
		return 0, false, err
	}
	issues := 0
	for _, s := range out.InstanceStatuses {
		// Only count "impaired" — not "not-applicable" (stopped instances),
		// "insufficient-data" (recently launched), or "initializing".
		sysImpaired := s.SystemStatus != nil && s.SystemStatus.Status == ec2types.SummaryStatusImpaired
		instImpaired := s.InstanceStatus != nil && s.InstanceStatus.Status == ec2types.SummaryStatusImpaired
		if sysImpaired || instImpaired {
			issues++
		}
	}
	// Paginated API — result may be truncated (lower bound).
	truncated := out.NextToken != nil
	return issues, truncated, nil
}

// EnrichEBSVolumeStatus calls DescribeVolumeStatus (1 call, all volumes)
// and counts volumes with impaired status.
func EnrichEBSVolumeStatus(ctx context.Context, clients *ServiceClients, _ []resource.Resource) (int, bool, error) {
	if clients.EC2 == nil {
		return 0, false, nil
	}
	out, err := clients.EC2.DescribeVolumeStatus(ctx, &ec2.DescribeVolumeStatusInput{})
	if err != nil {
		return 0, false, err
	}
	issues := 0
	for _, v := range out.VolumeStatuses {
		if v.VolumeStatus != nil && v.VolumeStatus.Status != "ok" {
			issues++
		}
	}
	truncated := out.NextToken != nil
	return issues, truncated, nil
}

// EnrichCodeBuildStatus calls BatchGetBuilds for the latest build of each project
// and counts builds that are not SUCCEEDED.
func EnrichCodeBuildStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (int, bool, error) {
	if clients.CodeBuild == nil || len(resources) == 0 {
		return 0, false, nil
	}
	// Collect project names from resources.
	names := make([]string, 0, len(resources))
	for _, r := range resources {
		if r.ID != "" {
			names = append(names, r.ID)
		}
	}
	if len(names) == 0 {
		return 0, false, nil
	}
	// ListBuildsForProject for each to get latest build ID, then batch get.
	buildIDs := make([]string, 0, len(names))
	for _, name := range names {
		if len(buildIDs) >= EnrichmentCap {
			break
		}
		out, err := clients.CodeBuild.ListBuildsForProject(ctx, &codebuild.ListBuildsForProjectInput{
			ProjectName: aws.String(name),
			SortOrder:   cbtypes.SortOrderTypeDescending,
		})
		if err != nil {
			continue
		}
		if len(out.Ids) > 0 {
			buildIDs = append(buildIDs, out.Ids[0])
		}
	}
	if len(buildIDs) == 0 {
		return 0, false, nil
	}
	builds, err := clients.CodeBuild.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{
		Ids: buildIDs,
	})
	if err != nil {
		return 0, false, err
	}
	issues := 0
	for _, b := range builds.Builds {
		if b.BuildStatus != cbtypes.StatusTypeSucceeded {
			issues++
		}
	}
	return issues, len(resources) > EnrichmentCap, nil
}

// EnrichTargetGroupHealth calls DescribeTargetHealth for each target group (1 per TG, cap ~50).
func EnrichTargetGroupHealth(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (int, bool, error) {
	if clients.ELBv2 == nil {
		return 0, false, nil
	}
	issues := 0
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
			continue
		}
		for _, t := range out.TargetHealthDescriptions {
			if t.TargetHealth != nil && t.TargetHealth.State != elbtypes.TargetHealthStateEnumHealthy {
				issues++
				break // one unhealthy target is enough to flag the TG
			}
		}
	}
	return issues, len(resources) > EnrichmentCap, nil
}

// EnrichCodePipelineStatus calls GetPipelineState for each pipeline (1 per pipeline, cap ~50).
func EnrichCodePipelineStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (int, bool, error) {
	if clients.CodePipeline == nil {
		return 0, false, nil
	}
	issues := 0
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
			continue
		}
		for _, stage := range out.StageStates {
			if stage.LatestExecution != nil && stage.LatestExecution.Status == "Failed" {
				issues++
				break
			}
		}
	}
	return issues, len(resources) > EnrichmentCap, nil
}

// EnrichDynamoDBStatus calls DescribeTable for each table (1 per table, cap ~50).
func EnrichDynamoDBStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (int, bool, error) {
	if clients.DynamoDB == nil {
		return 0, false, nil
	}
	issues := 0
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
			continue
		}
		if out.Table != nil && out.Table.TableStatus != dbtypes.TableStatusActive {
			issues++
			continue
		}
		// Check GSIs
		if out.Table != nil {
			for _, gsi := range out.Table.GlobalSecondaryIndexes {
				if gsi.IndexStatus != dbtypes.IndexStatusActive {
					issues++
					break
				}
			}
		}
	}
	return issues, len(resources) > EnrichmentCap, nil
}

// EnrichStepFunctionsStatus calls ListExecutions(max:1) for each state machine (1 per SFN, cap ~50).
func EnrichStepFunctionsStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (int, bool, error) {
	if clients.SFN == nil {
		return 0, false, nil
	}
	issues := 0
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
			continue
		}
		if len(out.Executions) > 0 {
			s := out.Executions[0].Status
			if s == sfntypes.ExecutionStatusFailed || s == sfntypes.ExecutionStatusTimedOut || s == sfntypes.ExecutionStatusAborted {
				issues++
			}
		}
	}
	return issues, len(resources) > EnrichmentCap, nil
}

// EnrichGlueJobStatus calls GetJobRuns(max:1) for each job (1 per job, cap ~50).
func EnrichGlueJobStatus(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (int, bool, error) {
	if clients.Glue == nil {
		return 0, false, nil
	}
	issues := 0
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
			continue
		}
		if len(out.JobRuns) > 0 {
			s := out.JobRuns[0].JobRunState
			if s == gluetypes.JobRunStateFailed || s == gluetypes.JobRunStateError || s == gluetypes.JobRunStateTimeout {
				issues++
			}
		}
	}
	return issues, len(resources) > EnrichmentCap, nil
}
