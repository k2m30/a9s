package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("sfn_executions", []string{
		"execution_arn", "name", "status", "start_date", "stop_date",
		"duration", "state_machine_arn", "state_machine_alias_arn",
		"state_machine_version_arn", "map_run_arn", "item_count",
		"redrive_count", "redrive_date",
	})

	resource.RegisterPaginatedChild("sfn_executions", func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSFNExecutions(ctx, c.SFN, parentCtx, continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "SFN Executions",
		ShortName: "sfn_executions",
		Columns:   resource.SFNExecutionColumns(),
		CopyField: "execution_arn",
		Children: []resource.ChildViewDef{{
			ChildType:      "sfn_execution_history",
			Key:            "enter",
			ContextKeys:    map[string]string{"execution_arn": "execution_arn", "execution_name": "Name"},
			DisplayNameKey: "execution_name",
		}},
	})
}

// FetchSFNExecutions calls the SFN ListExecutions API and converts the
// response into a FetchResult with pagination support. A single API call is
// made per invocation; IsTruncated and NextToken are forwarded as pagination
// metadata for the caller to request the next page.
func FetchSFNExecutions(
	ctx context.Context,
	api SFNListExecutionsAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	smArn := parentCtx["state_machine_arn"]

	input := &sfn.ListExecutionsInput{
		StateMachineArn: &smArn,
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListExecutions(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing executions for %s: %w", smArn, err)
	}

	var resources []resource.Resource
	for _, item := range output.Executions {
		resources = append(resources, convertSFNExecution(item))
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// convertSFNExecution converts a single SFN ExecutionListItem into a generic Resource.
func convertSFNExecution(item sfntypes.ExecutionListItem) resource.Resource {
	name := ""
	if item.Name != nil {
		name = *item.Name
	}

	executionArn := ""
	if item.ExecutionArn != nil {
		executionArn = *item.ExecutionArn
	}

	status := string(item.Status)

	startDate := ""
	if item.StartDate != nil {
		startDate = item.StartDate.UTC().Format("2006-01-02 15:04")
	}

	stopDate := ""
	if item.StopDate != nil {
		stopDate = item.StopDate.UTC().Format("2006-01-02 15:04")
	}

	duration := ""
	if item.StopDate != nil && item.StartDate != nil {
		duration = FormatHumanDuration(item.StopDate.Sub(*item.StartDate))
	} else if item.StopDate == nil && item.StartDate != nil {
		duration = "~" + FormatHumanDuration(time.Now().UTC().Sub(*item.StartDate))
	}

	stateMachineArn := ""
	if item.StateMachineArn != nil {
		stateMachineArn = *item.StateMachineArn
	}

	stateMachineAliasArn := ""
	if item.StateMachineAliasArn != nil {
		stateMachineAliasArn = *item.StateMachineAliasArn
	}

	stateMachineVersionArn := ""
	if item.StateMachineVersionArn != nil {
		stateMachineVersionArn = *item.StateMachineVersionArn
	}

	mapRunArn := ""
	if item.MapRunArn != nil {
		mapRunArn = *item.MapRunArn
	}

	itemCount := ""
	if item.ItemCount != nil && *item.ItemCount > 0 {
		itemCount = fmt.Sprintf("%d", *item.ItemCount)
	}

	redriveCount := ""
	if item.RedriveCount != nil && *item.RedriveCount > 0 {
		redriveCount = fmt.Sprintf("%d", *item.RedriveCount)
	}

	redriveDate := ""
	if item.RedriveDate != nil {
		redriveDate = item.RedriveDate.UTC().Format("2006-01-02 15:04")
	}

	return resource.Resource{
		ID:     name,
		Name:   name,
		Status: status,
		Fields: map[string]string{
			"execution_arn":             executionArn,
			"name":                      name,
			"status":                    status,
			"start_date":                startDate,
			"stop_date":                 stopDate,
			"duration":                  duration,
			"state_machine_arn":         stateMachineArn,
			"state_machine_alias_arn":   stateMachineAliasArn,
			"state_machine_version_arn": stateMachineVersionArn,
			"map_run_arn":               mapRunArn,
			"item_count":                itemCount,
			"redrive_count":             redriveCount,
			"redrive_date":              redriveDate,
		},
		RawStruct: item,
	}
}

// FormatHumanDuration formats a time.Duration into a human-readable string.
// Examples: "45s", "2m 47s", "2h 30m", "3d 12h".
func FormatHumanDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	totalMinutes := int(d.Minutes())
	totalHours := int(d.Hours())

	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", totalSeconds)
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", totalMinutes, totalSeconds%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", totalHours, totalMinutes%60)
	default:
		return fmt.Sprintf("%dd %dh", totalHours/24, totalHours%24)
	}
}
