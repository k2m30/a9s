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

	resource.RegisterPaginatedChild("alarm_history", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
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
// converts the response into a FetchResult with pagination support. Each call
// returns up to maxAlarmHistoryItems (200) items. When the cap is reached and
// more pages exist, FetchResult.Pagination.IsTruncated is set to true with a
// NextToken for continuation.
func FetchAlarmHistory(
	ctx context.Context,
	api CloudWatchDescribeAlarmHistoryAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	const maxItems = 200

	alarmName := parentCtx["alarm_name"]

	var resources []resource.Resource
	var nextToken *string
	if continuationToken != "" {
		nextToken = &continuationToken
	}

	for {
		input := &cloudwatch.DescribeAlarmHistoryInput{
			AlarmName: &alarmName,
			NextToken: nextToken,
		}

		output, err := api.DescribeAlarmHistory(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("describing alarm history for %s: %w", alarmName, err)
		}

		for _, item := range output.AlarmHistoryItems {
			resources = append(resources, convertAlarmHistoryItem(item))

			if len(resources) >= maxItems {
				apiNextToken := ""
				if output.NextToken != nil {
					apiNextToken = *output.NextToken
				}
				return resource.FetchResult{
					Resources: resources,
					Pagination: &resource.PaginationMeta{
						IsTruncated: apiNextToken != "",
						NextToken:   apiNextToken,
						PageSize:    len(resources),
					},
				}, nil
			}
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			TotalHint:   len(resources),
			PageSize:    len(resources),
		},
	}, nil
}

// convertAlarmHistoryItem converts a single CloudWatch AlarmHistoryItem into a generic Resource.
func convertAlarmHistoryItem(item cwtypes.AlarmHistoryItem) resource.Resource {
	timestamp := ""
	id := ""
	if item.Timestamp != nil {
		timestamp = item.Timestamp.UTC().Format("2006-01-02 15:04:05")
		id = timestamp
	}

	historyItemType := string(item.HistoryItemType)

	historySummary := ""
	if item.HistorySummary != nil {
		historySummary = strings.ReplaceAll(*item.HistorySummary, "\n", " ")
		historySummary = strings.ReplaceAll(historySummary, "\r", " ")
	}

	return resource.Resource{
		ID:     id,
		Name:   id,
		Status: historyItemType,
		Fields: map[string]string{
			"timestamp":         timestamp,
			"history_item_type": historyItemType,
			"history_summary":   historySummary,
		},
		RawStruct: item,
	}
}
