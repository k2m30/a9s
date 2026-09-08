// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// computeKinesisFindings returns a []domain.Finding for the given Kinesis stream state.
func computeKinesisFindings(state kinesistypes.StreamStatus) []domain.Finding {
	switch state {
	case kinesistypes.StreamStatusCreating:
		return []domain.Finding{wave1Finding(CodeKinesisCreating)}
	case kinesistypes.StreamStatusUpdating:
		return []domain.Finding{wave1Finding(CodeKinesisUpdating)}
	case kinesistypes.StreamStatusDeleting:
		return []domain.Finding{wave1Finding(CodeKinesisDeleting)}
	default:
		return nil
	}
}

// FetchKinesisStreamsPage fetches a single page of Kinesis streams.
func FetchKinesisStreamsPage(ctx context.Context, api KinesisListStreamsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &kinesis.ListStreamsInput{
		Limit: aws.Int32(DefaultPageSize),
	}
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

		rawStatus := string(stream.StreamStatus)

		creationTime := ""
		if stream.StreamCreationTimestamp != nil {
			creationTime = stream.StreamCreationTimestamp.Format("2006-01-02 15:04")
		}

		streamMode := ""
		if stream.StreamModeDetails != nil {
			streamMode = string(stream.StreamModeDetails.StreamMode)
		}

		findings := computeKinesisFindings(stream.StreamStatus)
		statusPhrase := domain.StatusPhrase(findings)

		r := resource.Resource{
			ID:       streamName,
			Name:     streamName,
			Findings: findings,
			Fields: map[string]string{
				"stream_name":   streamName,
				"status":        statusPhrase,
				"stream_status": rawStatus,
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
