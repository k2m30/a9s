package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// PipelineStageRow is the RawStruct for each flattened stage-action row.
// It holds both stage and action data for detail/YAML view rendering.
type PipelineStageRow struct {
	StageName        string
	StageStatus      string
	ActionName       string
	ActionStatus     string
	LastStatusChange *time.Time
	ExternalURL      string
	Token            string
	ErrorCode        string
	ErrorMessage     string
	RevisionId       string
	RevisionSummary  string
}

func init() {
	resource.RegisterFieldKeys("pipeline_stages", []string{
		"stage_name", "stage_status", "action_name", "action_status",
		"last_change_time", "external_url", "action_token",
		"action_error_details", "revision_id", "revision_summary",
	})

	resource.RegisterChildFetcher("pipeline_stages", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchPipelineStages(ctx, c.CodePipeline, parentCtx)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Pipeline Stages",
		ShortName: "pipeline_stages",
		Columns:   resource.PipelineStageColumns(),
		CopyField: "external_url",
	})
}

// FetchPipelineStages calls GetPipelineState and flattens the hierarchical
// stages→actions response into a flat list of stage-action pair Resources.
// A stage with N actions produces N rows. The stage_name field is set only
// on the first row per stage (blank for subsequent actions in the same stage).
func FetchPipelineStages(
	ctx context.Context,
	api CodePipelineGetPipelineStateAPI,
	parentCtx map[string]string,
) ([]resource.Resource, error) {
	pipelineName := parentCtx["pipeline_name"]

	output, err := api.GetPipelineState(ctx, &codepipeline.GetPipelineStateInput{
		Name: &pipelineName,
	})
	if err != nil {
		return nil, fmt.Errorf("getting pipeline state for %s: %w", pipelineName, err)
	}

	var resources []resource.Resource

	for _, stage := range output.StageStates {
		stageName := ""
		if stage.StageName != nil {
			stageName = *stage.StageName
		}

		stageStatus := ""
		if stage.LatestExecution != nil {
			stageStatus = string(stage.LatestExecution.Status)
		}

		for actionIdx, action := range stage.ActionStates {
			r := convertPipelineStageAction(stageName, stageStatus, action, actionIdx == 0)
			resources = append(resources, r)
		}
	}

	return resources, nil
}

// convertPipelineStageAction converts a single stage-action pair into a Resource.
// showStageName controls whether stage_name is populated (true for the first
// action in each stage, false for subsequent actions — visual grouping).
func convertPipelineStageAction(stageName, stageStatus string, action cptypes.ActionState, showStageName bool) resource.Resource {
	displayStageName := ""
	displayStageStatus := ""
	if showStageName {
		displayStageName = stageName
		displayStageStatus = stageStatus
	}

	actionName := ""
	if action.ActionName != nil {
		actionName = *action.ActionName
	}

	actionStatus := ""
	lastChangeTime := ""
	externalURL := ""
	actionToken := ""
	errorDetails := ""
	var lastStatusChange *time.Time
	var errorCode, errorMessage string

	if action.LatestExecution != nil {
		actionStatus = string(action.LatestExecution.Status)
		if action.LatestExecution.LastStatusChange != nil {
			lastStatusChange = action.LatestExecution.LastStatusChange
			lastChangeTime = action.LatestExecution.LastStatusChange.UTC().Format("2006-01-02 15:04:05")
		}
		if action.LatestExecution.ExternalExecutionUrl != nil {
			externalURL = *action.LatestExecution.ExternalExecutionUrl
		}
		if action.LatestExecution.Token != nil {
			actionToken = *action.LatestExecution.Token
		}
		if action.LatestExecution.ErrorDetails != nil {
			if action.LatestExecution.ErrorDetails.Code != nil {
				errorCode = *action.LatestExecution.ErrorDetails.Code
			}
			if action.LatestExecution.ErrorDetails.Message != nil {
				errorMessage = *action.LatestExecution.ErrorDetails.Message
			}
			if errorCode != "" || errorMessage != "" {
				errorDetails = errorCode
				if errorMessage != "" {
					if errorCode != "" {
						errorDetails += ": "
					}
					errorDetails += errorMessage
				}
			}
		}
	}

	revisionID := ""
	revisionSummary := ""
	if action.CurrentRevision != nil {
		if action.CurrentRevision.RevisionId != nil {
			revisionID = *action.CurrentRevision.RevisionId
		}
		if action.CurrentRevision.RevisionChangeId != nil {
			revisionSummary = *action.CurrentRevision.RevisionChangeId
		}
	}

	id := stageName + "/" + actionName
	status := pipelineActionStatus(actionStatus)

	return resource.Resource{
		ID:     id,
		Name:   actionName,
		Status: status,
		Fields: map[string]string{
			"stage_name":           displayStageName,
			"stage_status":         displayStageStatus,
			"action_name":          actionName,
			"action_status":        actionStatus,
			"last_change_time":     lastChangeTime,
			"external_url":         externalURL,
			"action_token":         actionToken,
			"action_error_details": errorDetails,
			"revision_id":          revisionID,
			"revision_summary":     revisionSummary,
		},
		RawStruct: PipelineStageRow{
			StageName:        stageName,
			StageStatus:      stageStatus,
			ActionName:       actionName,
			ActionStatus:     actionStatus,
			LastStatusChange: lastStatusChange,
			ExternalURL:      externalURL,
			Token:            actionToken,
			ErrorCode:        errorCode,
			ErrorMessage:     errorMessage,
			RevisionId:       revisionID,
			RevisionSummary:  revisionSummary,
		},
	}
}

// pipelineActionStatus maps action execution status to the row coloring status.
func pipelineActionStatus(actionStatus string) string {
	switch cptypes.ActionExecutionStatus(actionStatus) {
	case cptypes.ActionExecutionStatusSucceeded:
		return "running"
	case cptypes.ActionExecutionStatusFailed:
		return "failed"
	case cptypes.ActionExecutionStatusInProgress:
		return "pending"
	case cptypes.ActionExecutionStatusAbandoned:
		return "terminated"
	default:
		return "terminated"
	}
}
