package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
)

// KinesisFixtures holds typed fixture data for Kinesis.
type KinesisFixtures struct {
	Streams []kinesistypes.StreamSummary
	// TagsByStream maps stream name to its tags — backs kinesis:ListTagsForStream
	// for the kinesis:cfn related-panel pivot.
	TagsByStream map[string][]kinesistypes.Tag
	// KeyIDByStream maps stream name to its KMS KeyId — backs
	// kinesis:DescribeStreamSummary for the kinesis:kms related-panel pivot.
	KeyIDByStream map[string]string
}

func mustParseKinesisTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewKinesisFixtures constructs KinesisFixtures from the canonical demo data.
var sharedKinesisFixtures = sync.OnceValue(func() *KinesisFixtures {
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
			// Issue: StreamStatus=UPDATING → wave1 finding (CodeKinesisUpdating,
			// SevWarn) → Warning (shard split/merge or mode change in progress).
			{
				StreamName:              aws.String("payments-ledger-stream"),
				StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/payments-ledger-stream"),
				StreamStatus:            kinesistypes.StreamStatusUpdating,
				StreamCreationTimestamp: aws.Time(mustParseKinesisTime("2025-09-05T11:00:00+00:00")),
				StreamModeDetails: &kinesistypes.StreamModeDetails{
					StreamMode: kinesistypes.StreamModeProvisioned,
				},
			},
		},
		// TagsByStream — required for the kinesis:cfn related-panel pivot
		// (checkKinesisCFN → kinesis:ListTagsForStream). Points at
		// acme-vpc-stack (cfn.go).
		TagsByStream: map[string][]kinesistypes.Tag{
			"clickstream-ingest": {
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("acme-vpc-stack")},
				{Key: aws.String("Environment"), Value: aws.String("production")},
			},
		},
		// KeyIDByStream — required for the kinesis:kms related-panel pivot
		// (checkKinesisKMS → kinesis:DescribeStreamSummary). Reuses the
		// primary production KMS key (kms.go).
		KeyIDByStream: map[string]string{
			"clickstream-ingest": "a1b2c3d4-5678-90ab-cdef-111111111111",
		},
	}
})

func NewKinesisFixtures() *KinesisFixtures {
	return sharedKinesisFixtures()
}
