package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchAlarmHistory calls the CloudWatch DescribeAlarmHistory API and
// converts the response into a FetchResult with pagination support. A single
// API call is made per invocation; IsTruncated and NextToken are forwarded as
// pagination metadata for the caller to request the next page.
func FetchAlarmHistory(
	ctx context.Context,
	api CloudWatchDescribeAlarmHistoryAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	alarmName := parentCtx["alarm_name"]

	input := &cloudwatch.DescribeAlarmHistoryInput{
		AlarmName: &alarmName,
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeAlarmHistory(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("describing alarm history for %s: %w", alarmName, err)
	}

	var resources []resource.Resource
	for _, item := range output.AlarmHistoryItems {
		resources = append(resources, convertAlarmHistoryItem(item))
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

// convertAlarmHistoryItem converts a single CloudWatch AlarmHistoryItem into a generic Resource.
func convertAlarmHistoryItem(item cwtypes.AlarmHistoryItem) resource.Resource {
	timestamp := ""
	id := ""
	if item.Timestamp != nil {
		timestamp = item.Timestamp.UTC().Format("2006-01-02 15:04")
		id = timestamp
	}

	historyItemType := string(item.HistoryItemType)

	historySummary := ""
	if item.HistorySummary != nil {
		historySummary = strings.ReplaceAll(*item.HistorySummary, "\n", " ")
		historySummary = strings.ReplaceAll(historySummary, "\r", " ")
	}

	historyData := ""
	if item.HistoryData != nil {
		historyData = *item.HistoryData
	}

	return resource.Resource{
		ID:   id,
		Name: id,
		Fields: map[string]string{
			"timestamp":         timestamp,
			"history_item_type": historyItemType,
			"history_summary":   historySummary,
		},
		Findings:  alarmHistoryFindings(historyItemType, historyData),
		RawStruct: item,
	}
}

// alarmHistoryStateData is the shape of the JSON payload CloudWatch's
// DescribeAlarmHistory returns in AlarmHistoryItem.HistoryData for
// StateUpdate items — {"version":"1.0","oldState":{...},"newState":{...}}.
// Only the transitioned-to state's value is needed to classify severity.
type alarmHistoryStateData struct {
	NewState struct {
		StateValue string `json:"stateValue"`
	} `json:"newState"`
}

// alarmHistoryFindings returns a wave1 finding for a StateUpdate history item
// whose newState.StateValue is ALARM or INSUFFICIENT_DATA — the same two
// non-healthy states colorAlarm classifies for the live alarm resource
// (internal/aws/catalog_monitoring.go). ConfigurationUpdate/Action items and
// transitions to OK carry no finding (healthy/informational).
func alarmHistoryFindings(historyItemType, historyData string) []domain.Finding {
	if historyItemType != string(cwtypes.HistoryItemTypeStateUpdate) || historyData == "" {
		return nil
	}
	var parsed alarmHistoryStateData
	if err := json.Unmarshal([]byte(historyData), &parsed); err != nil {
		return nil
	}
	switch parsed.NewState.StateValue {
	case "ALARM":
		return []domain.Finding{{Code: CodeAlarmHistoryStateAlarm, Phrase: "alarm", Severity: domain.SevBroken, Source: "wave1"}}
	case "INSUFFICIENT_DATA":
		return []domain.Finding{{Code: CodeAlarmHistoryStateInsufficientData, Phrase: "insufficient data", Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}
