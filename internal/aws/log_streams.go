package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("log_streams", []string{"stream_name", "last_event", "first_event", "stored_bytes"})

	resource.RegisterChildFetcher("log_streams", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLogStreams(ctx, c.CloudWatchLogs, parentCtx["log_group_name"])
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
// log group and converts the response into a slice of generic Resource structs.
// It paginates via NextToken.
func FetchLogStreams(ctx context.Context, api CWLogsDescribeLogStreamsAPI, logGroupName string) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		input := &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: &logGroupName,
			NextToken:    nextToken,
		}

		output, err := api.DescribeLogStreams(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("fetching log streams: %w", err)
		}

		for _, s := range output.LogStreams {
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

			storedBytes := ""
			if s.StoredBytes != nil {
				storedBytes = formatBytes(*s.StoredBytes)
			}

			r := resource.Resource{
				ID:     name,
				Name:   name,
				Status: "",
				Fields: map[string]string{
					"stream_name":  name,
					"last_event":   lastEvent,
					"first_event":  firstEvent,
					"stored_bytes": storedBytes,
				},
				RawStruct: s,
			}

			resources = append(resources, r)
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
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
