package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/kinesis"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("kinesis", []string{"stream_name", "status", "stream_mode", "creation_time"})
	resource.Register("kinesis", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchKinesisStreams(ctx, c.Kinesis)
	})
}

// FetchKinesisStreams calls the Kinesis ListStreams API and converts the
// response into a slice of generic Resource structs.
// Uses the StreamSummaries field (not the legacy StreamNames).
func FetchKinesisStreams(ctx context.Context, api KinesisListStreamsAPI) ([]resource.Resource, error) {
	output, err := api.ListStreams(ctx, &kinesis.ListStreamsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching Kinesis streams: %w", err)
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
			creationTime = stream.StreamCreationTimestamp.Format("2006-01-02 15:04:05")
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

	return resources, nil
}
