package unit

import (
	"context"
	"fmt"
	"testing"

	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// SNS Topics fetcher tests
// ---------------------------------------------------------------------------

func TestFetchSNSTopics_ParsesMultipleTopics(t *testing.T) {
	mock := &mockSNSListTopicsClient{
		output: &sns.ListTopicsOutput{
			Topics: []snstypes.Topic{
				{
					TopicArn: strPtr("arn:aws:sns:us-east-1:123456789012:my-alerts-topic"),
				},
				{
					TopicArn: strPtr("arn:aws:sns:us-east-1:123456789012:prod-notifications"),
				},
			},
		},
	}

	resources, err := awsclient.FetchSNSTopics(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"topic_arn", "display_name"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first topic
	r0 := resources[0]
	if r0.ID != "arn:aws:sns:us-east-1:123456789012:my-alerts-topic" {
		t.Errorf("resource[0].ID: expected %q, got %q", "arn:aws:sns:us-east-1:123456789012:my-alerts-topic", r0.ID)
	}
	if r0.Name != "my-alerts-topic" {
		t.Errorf("resource[0].Name: expected %q, got %q", "my-alerts-topic", r0.Name)
	}
	if r0.Fields["topic_arn"] != "arn:aws:sns:us-east-1:123456789012:my-alerts-topic" {
		t.Errorf("resource[0].Fields[\"topic_arn\"]: expected %q, got %q",
			"arn:aws:sns:us-east-1:123456789012:my-alerts-topic", r0.Fields["topic_arn"])
	}
	if r0.Fields["display_name"] != "my-alerts-topic" {
		t.Errorf("resource[0].Fields[\"display_name\"]: expected %q, got %q", "my-alerts-topic", r0.Fields["display_name"])
	}

	// Verify second topic
	r1 := resources[1]
	if r1.ID != "arn:aws:sns:us-east-1:123456789012:prod-notifications" {
		t.Errorf("resource[1].ID: expected %q, got %q", "arn:aws:sns:us-east-1:123456789012:prod-notifications", r1.ID)
	}
	if r1.Name != "prod-notifications" {
		t.Errorf("resource[1].Name: expected %q, got %q", "prod-notifications", r1.Name)
	}
	if r1.Fields["display_name"] != "prod-notifications" {
		t.Errorf("resource[1].Fields[\"display_name\"]: expected %q, got %q", "prod-notifications", r1.Fields["display_name"])
	}
}

func TestFetchSNSTopics_ErrorResponse(t *testing.T) {
	mock := &mockSNSListTopicsClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}

	resources, err := awsclient.FetchSNSTopics(context.Background(), mock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchSNSTopics_EmptyResponse(t *testing.T) {
	mock := &mockSNSListTopicsClient{
		output: &sns.ListTopicsOutput{
			Topics: []snstypes.Topic{},
		},
	}

	resources, err := awsclient.FetchSNSTopics(context.Background(), mock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
