package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("kinesis", []string{"stream_name", "status", "stream_status", "stream_mode", "creation_time"})

	resource.RegisterPaginated("kinesis", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchKinesisStreamsPage(ctx, c.Kinesis, continuationToken)
	})
}

// computeKinesisFindings returns the Wave-1 findings for a Kinesis stream status.
// Returns nil for ACTIVE (healthy) streams.
func computeKinesisFindings(status kinesistypes.StreamStatus) []domain.Finding {
	switch status {
	case kinesistypes.StreamStatusActive:
		return nil
	case kinesistypes.StreamStatusCreating:
		return []domain.Finding{{Code: CodeKinesisCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"}}
	case kinesistypes.StreamStatusUpdating:
		return []domain.Finding{{Code: CodeKinesisUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"}}
	case kinesistypes.StreamStatusDeleting:
		return []domain.Finding{{Code: CodeKinesisDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"}}
	default:
		return nil
	}
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

		status := string(stream.StreamStatus)

		creationTime := ""
		if stream.StreamCreationTimestamp != nil {
			creationTime = stream.StreamCreationTimestamp.Format("2006-01-02 15:04")
		}

		streamMode := ""
		if stream.StreamModeDetails != nil {
			streamMode = string(stream.StreamModeDetails.StreamMode)
		}

		findings := computeKinesisFindings(stream.StreamStatus)
		statusPhrase := ""
		if len(findings) > 0 {
			statusPhrase = findings[0].Phrase
			if len(findings) > 1 {
				statusPhrase = fmt.Sprintf("%s (+%d)", statusPhrase, len(findings)-1)
			}
		}

		r := resource.Resource{
			ID:   streamName,
			Name: streamName,
			Fields: map[string]string{
				"stream_name":   streamName,
				"status":        statusPhrase,
				"stream_status": status,
				"stream_arn":    streamARN,
				"creation_time": creationTime,
				"stream_mode":   streamMode,
			},
			Findings:  findings,
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
