package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("alarm_history", []string{
		"timestamp", "history_item_type", "history_summary",
	})

	resource.RegisterPaginatedChild("alarm_history", func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAlarmHistory(ctx, c.CloudWatch, parentCtx, continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Alarm History",
		ShortName: "alarm_history",
		Columns:   resource.AlarmHistoryColumns(),
	})
}

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

	return resource.Resource{
		ID:    id,
		Name:  id,
		Fields: map[string]string{
			"timestamp":         timestamp,
			"history_item_type": historyItemType,
			"history_summary":   historySummary,
		},
		RawStruct: item,
	}
}
