package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
)

// KinesisFixtures holds typed fixture data for Kinesis.
type KinesisFixtures struct {
	Streams []kinesistypes.StreamSummary
}

func mustParseKinesisTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewKinesisFixtures constructs KinesisFixtures from the canonical demo data.
func NewKinesisFixtures() *KinesisFixtures {
	return &KinesisFixtures{
		Streams: []kinesistypes.StreamSummary{
			{
				StreamName:              aws.String("clickstream-ingest"),
				StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/clickstream-ingest"),
				StreamStatus:            kinesistypes.StreamStatusActive,
				StreamCreationTimestamp: aws.Time(mustParseKinesisTime("2025-06-15T10:30:00+00:00")),
				StreamModeDetails: &kinesistypes.StreamModeDetails{
					StreamMode: kinesistypes.StreamModeOnDemand,
				},
			},
			{
				StreamName:              aws.String("order-events-stream"),
				StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/order-events-stream"),
				StreamStatus:            kinesistypes.StreamStatusActive,
				StreamCreationTimestamp: aws.Time(mustParseKinesisTime("2025-03-01T08:00:00+00:00")),
				StreamModeDetails: &kinesistypes.StreamModeDetails{
					StreamMode: kinesistypes.StreamModeProvisioned,
				},
			},
			{
				StreamName:              aws.String("audit-log-stream"),
				StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/audit-log-stream"),
				StreamStatus:            kinesistypes.StreamStatusCreating,
				StreamCreationTimestamp: aws.Time(mustParseKinesisTime("2026-03-21T09:00:00+00:00")),
				StreamModeDetails: &kinesistypes.StreamModeDetails{
					StreamMode: kinesistypes.StreamModeOnDemand,
				},
			},
			// orders-prod-cdc — DDB→kinesis pivot: matches DescribeKinesisStreamingDestination
			// result for the orders-prod table (KinesisDestinations["orders-prod"]).
			{
				StreamName:              aws.String(OrdersProdKinesisStream),
				StreamARN:               aws.String(OrdersProdKinesisStreamARN),
				StreamStatus:            kinesistypes.StreamStatusActive,
				StreamCreationTimestamp: aws.Time(mustParseKinesisTime("2026-01-01T00:00:00+00:00")),
				StreamModeDetails: &kinesistypes.StreamModeDetails{
					StreamMode: kinesistypes.StreamModeOnDemand,
				},
			},
			// Issue: StreamStatus=DELETING → Warning (stream being torn down)
			{
				StreamName:              aws.String("kinesis-deleting"),
				StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/kinesis-deleting"),
				StreamStatus:            kinesistypes.StreamStatusDeleting,
				StreamCreationTimestamp: aws.Time(mustParseKinesisTime("2025-01-10T10:00:00+00:00")),
				StreamModeDetails: &kinesistypes.StreamModeDetails{
					StreamMode: kinesistypes.StreamModeProvisioned,
				},
			},
		},
	}
}
