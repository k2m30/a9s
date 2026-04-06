package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/kinesis"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("kinesis", []string{"stream_name", "status", "stream_mode", "creation_time"})

	resource.RegisterPaginated("kinesis", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchKinesisStreamsPage(ctx, c.Kinesis, continuationToken)
	})
}

// FetchKinesisStreams calls the Kinesis ListStreams API and converts the
// response into a slice of generic Resource structs.
// Uses the StreamSummaries field (not the legacy StreamNames).
func FetchKinesisStreams(ctx context.Context, api KinesisListStreamsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchKinesisStreamsPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchKinesisStreamsPage fetches a single page of Kinesis streams.
func FetchKinesisStreamsPage(ctx context.Context, api KinesisListStreamsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &kinesis.ListStreamsInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListStreams(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Kinesis streams: %w", err)
	}

	var resources []resource.Resource

	for _, stream := range output.StreamSummaries {
		streamName := ""
		if stream.StreamName != nil {
			streamName = *stream.StreamName
		}

		streamARN := ""
		if stream.StreamARN != nil {
			streamARN = *stream.StreamARN
		}

		status := string(stream.StreamStatus)

		creationTime := ""
		if stream.StreamCreationTimestamp != nil {
			creationTime = stream.StreamCreationTimestamp.Format("2006-01-02 15:04")
		}

		streamMode := ""
		if stream.StreamModeDetails != nil {
			streamMode = string(stream.StreamModeDetails.StreamMode)
		}

		r := resource.Resource{
			ID:     streamName,
			Name:   streamName,
			Status: status,
			Fields: map[string]string{
				"stream_name":   streamName,
				"status":        status,
				"stream_arn":    streamARN,
				"creation_time": creationTime,
				"stream_mode":   streamMode,
			},
			RawStruct: stream,
		}

		resources = append(resources, r)
	}

	isTruncated := output.HasMoreStreams != nil && *output.HasMoreStreams
	nextToken := ""
	if isTruncated && output.NextToken != nil {
		nextToken = *output.NextToken
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
