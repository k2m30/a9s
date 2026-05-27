package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// T-KIN-001 - Test Kinesis Streams response parsing
// ---------------------------------------------------------------------------

func TestFetchKinesisStreams_ParsesMultipleStreams(t *testing.T) {
	now := time.Now()
	mock := &mockKinesisClient{
		output: &kinesis.ListStreamsOutput{
			StreamSummaries: []kinesistypes.StreamSummary{
				{
					StreamName:              aws.String("my-stream-1"),
					StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/my-stream-1"),
					StreamStatus:            kinesistypes.StreamStatusActive,
					StreamCreationTimestamp: &now,
				},
				{
					StreamName:              aws.String("my-stream-2"),
					StreamARN:               aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/my-stream-2"),
					StreamStatus:            kinesistypes.StreamStatusCreating,
					StreamCreationTimestamp: &now,
				},
			},
		},
	}

	resources, err := awsclient.FetchKinesisStreams(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify first stream
	r := resources[0]
	if r.Name != "my-stream-1" {
		t.Errorf("expected Name 'my-stream-1', got %q", r.Name)
	}
	if r.ID != "my-stream-1" {
		t.Errorf("expected ID 'my-stream-1', got %q", r.ID)
	}
	// Healthy streams: Status=="" and Fields["status"]=="" (healthy silence).
	if r.Fields["stream_name"] != "my-stream-1" {
		t.Errorf("expected Fields[stream_name] 'my-stream-1', got %q", r.Fields["stream_name"])
	}
	if r.Fields["status"] != "" {
		t.Errorf("expected Fields[status] %q (healthy silence), got %q", "", r.Fields["status"])
	}
	if r.Fields["stream_arn"] == "" {
		t.Error("expected Fields[stream_arn] to be non-empty")
	}

	// Verify second stream: Status=="" (fetcher does not write Status), phrase is in Fields["status"].
	r2 := resources[1]
	if r2.Fields["status"] != "creating" {
		t.Errorf("expected Fields[status] %q for CREATING stream, got %q", "creating", r2.Fields["status"])
	}

	// Verify RawStruct is set
	if r.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}

}

func TestFetchKinesisStreams_EmptyResponse(t *testing.T) {
	mock := &mockKinesisClient{
		output: &kinesis.ListStreamsOutput{
			StreamSummaries: []kinesistypes.StreamSummary{},
		},
	}

	resources, err := awsclient.FetchKinesisStreams(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchKinesisStreams_APIError(t *testing.T) {
	mock := &mockKinesisClient{
		err: &mockAPIError{code: "AccessDeniedException", message: "access denied"},
	}

	_, err := awsclient.FetchKinesisStreams(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchKinesisStreams_NilFields(t *testing.T) {
	mock := &mockKinesisClient{
		output: &kinesis.ListStreamsOutput{
			StreamSummaries: []kinesistypes.StreamSummary{
				{
					StreamStatus: kinesistypes.StreamStatusActive,
				},
			},
		},
	}

	resources, err := awsclient.FetchKinesisStreams(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.Name != "" {
		t.Errorf("expected empty Name, got %q", r.Name)
	}
}
