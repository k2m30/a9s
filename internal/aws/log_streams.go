package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("log_streams", []string{"stream_name", "last_event", "first_event"})

	resource.RegisterPaginatedChild("log_streams", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLogStreams(ctx, c.CloudWatchLogs, parentCtx["log_group_name"], continuationToken)
	})
	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Log Streams",
		ShortName: "log_streams",
		Columns:   resource.LogStreamColumns(),
		Children: []resource.ChildViewDef{{
			ChildType:      "log_events",
			Key:            "enter",
			ContextKeys:    map[string]string{"log_group_name": "@parent.log_group_name", "log_stream_name": "Name"},
			DisplayNameKey: "log_stream_name",
		}},
	})
}

// FetchLogStreams calls the CloudWatchLogs DescribeLogStreams API for a given
// log group and converts the response into a FetchResult with pagination
// support. A single API call is made per invocation; IsTruncated and NextToken
// are forwarded as pagination metadata for the caller to request the next page.
func FetchLogStreams(ctx context.Context, api CWLogsDescribeLogStreamsAPI, logGroupName string, continuationToken string) (resource.FetchResult, error) {
	input := &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: &logGroupName,
		OrderBy:      cwlogstypes.OrderByLastEventTime,
		Descending:   aws.Bool(true),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeLogStreams(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching log streams: %w", err)
	}

	var resources []resource.Resource
	for _, s := range output.LogStreams {
		resources = append(resources, convertLogStream(s))
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

// convertLogStream converts a single CloudWatch LogStream into a generic Resource.
func convertLogStream(s cwlogstypes.LogStream) resource.Resource {
	name := ""
	if s.LogStreamName != nil {
		name = *s.LogStreamName
	}

	lastEvent := ""
	if s.LastEventTimestamp != nil {
		lastEvent = formatEpochMillis(*s.LastEventTimestamp)
	}

	firstEvent := ""
	if s.FirstEventTimestamp != nil {
		firstEvent = formatEpochMillis(*s.FirstEventTimestamp)
	}

	return resource.Resource{
		ID:     name,
		Name:   name,
		Status: "",
		Fields: map[string]string{
			"stream_name": name,
			"last_event":  lastEvent,
			"first_event": firstEvent,
		},
		RawStruct: s,
	}
}

// formatEpochMillis converts epoch milliseconds to a human-readable timestamp.
func formatEpochMillis(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02 15:04")
}

// formatBytes converts byte counts to human-readable sizes.
func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}

	const (
		kb = 1024
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
		tb = 1024 * 1024 * 1024 * 1024
	)

	switch {
	case bytes >= tb:
		val := float64(bytes) / float64(tb)
		return formatFloat(val) + " TB"
	case bytes >= gb:
		val := float64(bytes) / float64(gb)
		return formatFloat(val) + " GB"
	case bytes >= mb:
		val := float64(bytes) / float64(mb)
		return formatFloat(val) + " MB"
	case bytes >= kb:
		val := float64(bytes) / float64(kb)
		return formatFloat(val) + " KB"
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// formatFloat formats a float to remove unnecessary decimal places.
func formatFloat(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.1f", v)
}

