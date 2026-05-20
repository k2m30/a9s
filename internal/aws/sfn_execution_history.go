package aws

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

var camelSplitter = regexp.MustCompile("([a-z])([A-Z])")

func init() {
	catalog.RegisterChildView(catalog.ResourceTypeDef{
		Name:      "SFN Execution History",
		ShortName: "sfn_execution_history",
		Columns:   resource.SFNExecutionHistoryColumns(),
		CopyField: "event_detail",
		FieldKeys: []string{
			"timestamp", "event_type", "event_type_short",
			"state_name", "event_detail", "event_id", "previous_event_id",
		},
		ChildFetcher: func(ctx context.Context, clients any, parentCtx domain.ParentContext, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchSFNExecutionHistory(ctx, c.SFN, parentCtx, continuationToken)
		},
	})
}

// FetchSFNExecutionHistory calls the SFN GetExecutionHistory API and converts
// the response into a FetchResult with pagination support. A single API call is
// made per invocation; IsTruncated and NextToken are forwarded as pagination
// metadata for the caller to request the next page.
func FetchSFNExecutionHistory(
	ctx context.Context,
	api SFNGetExecutionHistoryAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	executionArn := parentCtx["execution_arn"]

	input := &sfn.GetExecutionHistoryInput{
		ExecutionArn: &executionArn,
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.GetExecutionHistory(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("getting execution history for %s: %w", executionArn, err)
	}

	var resources []resource.Resource
	var lastStateName string
	for _, event := range output.Events {
		resources = append(resources, ConvertHistoryEvent(event, &lastStateName))
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

// ConvertHistoryEvent converts a single SFN HistoryEvent into a generic Resource.
// The lastStateName pointer tracks state context across sequential events so that
// task-level events can inherit the state name from a preceding StateEntered event.
func ConvertHistoryEvent(event sfntypes.HistoryEvent, lastStateName *string) resource.Resource {
	eventType := string(event.Type)

	timestamp := ""
	if event.Timestamp != nil {
		timestamp = event.Timestamp.UTC().Format("2006-01-02 15:04")
	}

	stateName := resolveStateName(event, lastStateName)

	return resource.Resource{
		ID:     fmt.Sprintf("%d", event.Id),
		Name:   HumanizeEventType(eventType),
		Status: ClassifyEventStatus(eventType),
		Fields: map[string]string{
			"timestamp":         timestamp,
			"event_type":        eventType,
			"event_type_short":  HumanizeEventType(eventType),
			"state_name":        stateName,
			"event_detail":      ExtractEventDetail(event),
			"event_id":          fmt.Sprintf("%d", event.Id),
			"previous_event_id": fmt.Sprintf("%d", event.PreviousEventId),
		},
		RawStruct: event,
	}
}

// resolveStateName determines the state name for a history event.
// 1. If event has StateEnteredEventDetails.Name: use it, update *lastStateName
// 2. If event has StateExitedEventDetails.Name: use it, update *lastStateName
// 3. If event type starts with "Execution": return em-dash (execution-level)
// 4. If *lastStateName != "": use inherited state name
// 5. Otherwise: em-dash
func resolveStateName(event sfntypes.HistoryEvent, lastStateName *string) string {
	if event.StateEnteredEventDetails != nil && event.StateEnteredEventDetails.Name != nil {
		*lastStateName = *event.StateEnteredEventDetails.Name
		return *lastStateName
	}
	if event.StateExitedEventDetails != nil && event.StateExitedEventDetails.Name != nil {
		*lastStateName = *event.StateExitedEventDetails.Name
		return *lastStateName
	}
	if strings.HasPrefix(string(event.Type), "Execution") {
		return "\u2014"
	}
	if *lastStateName != "" {
		return *lastStateName
	}
	return "\u2014"
}

// HumanizeEventType converts a CamelCase event type string to a space-separated
// form. For example, "TaskFailed" becomes "Task Failed".
func HumanizeEventType(eventType string) string {
	return camelSplitter.ReplaceAllString(eventType, "${1} ${2}")
}

// ClassifyEventStatus maps an SFN history event type string to a synthetic
// status string used for row coloring in the TUI.
func ClassifyEventStatus(eventType string) string {
	// ExecutionStarted must be checked before generic suffix matching
	if eventType == "ExecutionStarted" {
		return "active"
	}
	if strings.HasSuffix(eventType, "Succeeded") || strings.HasSuffix(eventType, "Exited") {
		return "succeeded"
	}
	if strings.HasSuffix(eventType, "Failed") || strings.HasSuffix(eventType, "TimedOut") || eventType == "ExecutionAborted" {
		return "failed"
	}
	if strings.HasSuffix(eventType, "Scheduled") || strings.HasSuffix(eventType, "Started") || strings.HasSuffix(eventType, "Entered") {
		return "pending"
	}
	return "active"
}

// ExtractEventDetail extracts a human-readable detail string from a history event.
// It checks failure details, state I/O, execution I/O, and task details.
// Newlines are stripped from the result.
func ExtractEventDetail(event sfntypes.HistoryEvent) string {
	// Check all *FailedEventDetails for Error/Cause
	if detail := extractFailedDetail(event); detail != "" {
		return sanitizeDetail(detail)
	}

	// Check state entered/exited I/O
	if event.StateEnteredEventDetails != nil && event.StateEnteredEventDetails.Input != nil {
		return sanitizeDetail(*event.StateEnteredEventDetails.Input)
	}
	if event.StateExitedEventDetails != nil && event.StateExitedEventDetails.Output != nil {
		return sanitizeDetail(*event.StateExitedEventDetails.Output)
	}

	// Check execution started/succeeded I/O
	if event.ExecutionStartedEventDetails != nil && event.ExecutionStartedEventDetails.Input != nil {
		return sanitizeDetail(*event.ExecutionStartedEventDetails.Input)
	}
	if event.ExecutionSucceededEventDetails != nil && event.ExecutionSucceededEventDetails.Output != nil {
		return sanitizeDetail(*event.ExecutionSucceededEventDetails.Output)
	}

	// Check task scheduled/submitted details
	if event.TaskScheduledEventDetails != nil && event.TaskScheduledEventDetails.Resource != nil {
		return sanitizeDetail(*event.TaskScheduledEventDetails.Resource)
	}
	if event.TaskSubmittedEventDetails != nil && event.TaskSubmittedEventDetails.Output != nil {
		return sanitizeDetail(*event.TaskSubmittedEventDetails.Output)
	}

	// Check task succeeded output
	if event.TaskSucceededEventDetails != nil && event.TaskSucceededEventDetails.Output != nil {
		return sanitizeDetail(*event.TaskSucceededEventDetails.Output)
	}

	return "\u2014"
}

// extractFailedDetail checks all failure detail fields and returns Error: Cause
// or just Error if present.
func extractFailedDetail(event sfntypes.HistoryEvent) string {
	type failDetail struct {
		Error *string
		Cause *string
	}

	var fd *failDetail

	switch {
	case event.TaskFailedEventDetails != nil:
		fd = &failDetail{event.TaskFailedEventDetails.Error, event.TaskFailedEventDetails.Cause}
	case event.ExecutionFailedEventDetails != nil:
		fd = &failDetail{event.ExecutionFailedEventDetails.Error, event.ExecutionFailedEventDetails.Cause}
	case event.ActivityFailedEventDetails != nil:
		fd = &failDetail{event.ActivityFailedEventDetails.Error, event.ActivityFailedEventDetails.Cause}
	case event.LambdaFunctionFailedEventDetails != nil:
		fd = &failDetail{event.LambdaFunctionFailedEventDetails.Error, event.LambdaFunctionFailedEventDetails.Cause}
	case event.ExecutionTimedOutEventDetails != nil:
		fd = &failDetail{event.ExecutionTimedOutEventDetails.Error, event.ExecutionTimedOutEventDetails.Cause}
	case event.TaskTimedOutEventDetails != nil:
		fd = &failDetail{event.TaskTimedOutEventDetails.Error, event.TaskTimedOutEventDetails.Cause}
	case event.LambdaFunctionTimedOutEventDetails != nil:
		fd = &failDetail{event.LambdaFunctionTimedOutEventDetails.Error, event.LambdaFunctionTimedOutEventDetails.Cause}
	case event.ActivityTimedOutEventDetails != nil:
		fd = &failDetail{event.ActivityTimedOutEventDetails.Error, event.ActivityTimedOutEventDetails.Cause}
	case event.ExecutionAbortedEventDetails != nil:
		fd = &failDetail{event.ExecutionAbortedEventDetails.Error, event.ExecutionAbortedEventDetails.Cause}
	case event.TaskStartFailedEventDetails != nil:
		fd = &failDetail{event.TaskStartFailedEventDetails.Error, event.TaskStartFailedEventDetails.Cause}
	case event.TaskSubmitFailedEventDetails != nil:
		fd = &failDetail{event.TaskSubmitFailedEventDetails.Error, event.TaskSubmitFailedEventDetails.Cause}
	case event.LambdaFunctionScheduleFailedEventDetails != nil:
		fd = &failDetail{event.LambdaFunctionScheduleFailedEventDetails.Error, event.LambdaFunctionScheduleFailedEventDetails.Cause}
	case event.LambdaFunctionStartFailedEventDetails != nil:
		fd = &failDetail{event.LambdaFunctionStartFailedEventDetails.Error, event.LambdaFunctionStartFailedEventDetails.Cause}
	case event.ActivityScheduleFailedEventDetails != nil:
		fd = &failDetail{event.ActivityScheduleFailedEventDetails.Error, event.ActivityScheduleFailedEventDetails.Cause}
	case event.MapRunFailedEventDetails != nil:
		fd = &failDetail{event.MapRunFailedEventDetails.Error, event.MapRunFailedEventDetails.Cause}
	}

	if fd == nil {
		return ""
	}

	errStr := ""
	if fd.Error != nil {
		errStr = *fd.Error
	}
	causeStr := ""
	if fd.Cause != nil {
		causeStr = *fd.Cause
	}

	if errStr != "" && causeStr != "" {
		return errStr + ": " + causeStr
	}
	if errStr != "" {
		return errStr
	}
	if causeStr != "" {
		return causeStr
	}
	return ""
}

// sanitizeDetail strips newlines from detail strings.
func sanitizeDetail(s string) string {
	return strings.ReplaceAll(s, "\n", " ")
}
