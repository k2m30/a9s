package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"

	awsclient "github.com/k2m30/a9s/internal/aws"
)

// ---------------------------------------------------------------------------
// SQS Queues fetcher tests (Pattern C: list + describe)
// ---------------------------------------------------------------------------

func TestFetchSQSQueues_ParsesMultipleQueues(t *testing.T) {
	listMock := &mockSQSListQueuesClient{
		output: &sqs.ListQueuesOutput{
			QueueUrls: []string{
				"https://sqs.us-east-1.amazonaws.com/123456789012/my-orders-queue",
				"https://sqs.us-east-1.amazonaws.com/123456789012/my-dlq",
			},
		},
	}
	attrMock := &mockSQSGetQueueAttributesClient{
		outputs: map[string]*sqs.GetQueueAttributesOutput{
			"https://sqs.us-east-1.amazonaws.com/123456789012/my-orders-queue": {
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "42",
					"ApproximateNumberOfMessagesNotVisible": "5",
					"DelaySeconds":                          "0",
					"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:my-orders-queue",
				},
			},
			"https://sqs.us-east-1.amazonaws.com/123456789012/my-dlq": {
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "3",
					"ApproximateNumberOfMessagesNotVisible": "0",
					"DelaySeconds":                          "10",
					"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:my-dlq",
				},
			},
		},
	}

	resources, err := awsclient.FetchSQSQueues(context.Background(), listMock, attrMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields exist
	requiredFields := []string{"queue_name", "queue_url", "approx_messages", "approx_not_visible", "delay_seconds"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first queue
	r0 := resources[0]
	if r0.ID != "my-orders-queue" {
		t.Errorf("resource[0].ID: expected %q, got %q", "my-orders-queue", r0.ID)
	}
	if r0.Name != "my-orders-queue" {
		t.Errorf("resource[0].Name: expected %q, got %q", "my-orders-queue", r0.Name)
	}
	if r0.Fields["queue_name"] != "my-orders-queue" {
		t.Errorf("resource[0].Fields[\"queue_name\"]: expected %q, got %q", "my-orders-queue", r0.Fields["queue_name"])
	}
	if r0.Fields["queue_url"] != "https://sqs.us-east-1.amazonaws.com/123456789012/my-orders-queue" {
		t.Errorf("resource[0].Fields[\"queue_url\"]: expected %q, got %q",
			"https://sqs.us-east-1.amazonaws.com/123456789012/my-orders-queue", r0.Fields["queue_url"])
	}
	if r0.Fields["approx_messages"] != "42" {
		t.Errorf("resource[0].Fields[\"approx_messages\"]: expected %q, got %q", "42", r0.Fields["approx_messages"])
	}
	if r0.Fields["approx_not_visible"] != "5" {
		t.Errorf("resource[0].Fields[\"approx_not_visible\"]: expected %q, got %q", "5", r0.Fields["approx_not_visible"])
	}
	if r0.Fields["delay_seconds"] != "0" {
		t.Errorf("resource[0].Fields[\"delay_seconds\"]: expected %q, got %q", "0", r0.Fields["delay_seconds"])
	}

	// Verify second queue
	r1 := resources[1]
	if r1.ID != "my-dlq" {
		t.Errorf("resource[1].ID: expected %q, got %q", "my-dlq", r1.ID)
	}
	if r1.Fields["approx_messages"] != "3" {
		t.Errorf("resource[1].Fields[\"approx_messages\"]: expected %q, got %q", "3", r1.Fields["approx_messages"])
	}
	if r1.Fields["delay_seconds"] != "10" {
		t.Errorf("resource[1].Fields[\"delay_seconds\"]: expected %q, got %q", "10", r1.Fields["delay_seconds"])
	}
}

func TestFetchSQSQueues_DetailDataPopulated(t *testing.T) {
	listMock := &mockSQSListQueuesClient{
		output: &sqs.ListQueuesOutput{
			QueueUrls: []string{
				"https://sqs.us-east-1.amazonaws.com/123456789012/detail-test-queue",
			},
		},
	}
	attrMock := &mockSQSGetQueueAttributesClient{
		outputs: map[string]*sqs.GetQueueAttributesOutput{
			"https://sqs.us-east-1.amazonaws.com/123456789012/detail-test-queue": {
				Attributes: map[string]string{
					"ApproximateNumberOfMessages":           "7",
					"ApproximateNumberOfMessagesNotVisible": "1",
					"DelaySeconds":                          "5",
					"QueueArn":                              "arn:aws:sqs:us-east-1:123456789012:detail-test-queue",
					"VisibilityTimeout":                     "30",
					"MaximumMessageSize":                    "262144",
					"MessageRetentionPeriod":                "345600",
				},
			},
		},
	}

	resources, err := awsclient.FetchSQSQueues(context.Background(), listMock, attrMock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]
	if r.DetailData == nil {
		t.Fatal("DetailData must not be nil")
	}
	if len(r.DetailData) == 0 {
		t.Fatal("DetailData must not be empty")
	}
	if r.DetailData["Queue Name"] != "detail-test-queue" {
		t.Errorf("DetailData[\"Queue Name\"] = %q, want %q", r.DetailData["Queue Name"], "detail-test-queue")
	}
	if r.DetailData["ARN"] != "arn:aws:sqs:us-east-1:123456789012:detail-test-queue" {
		t.Errorf("DetailData[\"ARN\"] = %q, want %q",
			r.DetailData["ARN"], "arn:aws:sqs:us-east-1:123456789012:detail-test-queue")
	}
	if r.DetailData["Visibility Timeout"] != "30" {
		t.Errorf("DetailData[\"Visibility Timeout\"] = %q, want %q", r.DetailData["Visibility Timeout"], "30")
	}
	if r.DetailData["Retention Period"] != "345600" {
		t.Errorf("DetailData[\"Retention Period\"] = %q, want %q", r.DetailData["Retention Period"], "345600")
	}
}

func TestFetchSQSQueues_ErrorOnList(t *testing.T) {
	listMock := &mockSQSListQueuesClient{
		output: nil,
		err:    fmt.Errorf("AWS API error: access denied"),
	}
	attrMock := &mockSQSGetQueueAttributesClient{
		outputs: map[string]*sqs.GetQueueAttributesOutput{},
	}

	resources, err := awsclient.FetchSQSQueues(context.Background(), listMock, attrMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchSQSQueues_EmptyResponse(t *testing.T) {
	listMock := &mockSQSListQueuesClient{
		output: &sqs.ListQueuesOutput{
			QueueUrls: []string{},
		},
	}
	attrMock := &mockSQSGetQueueAttributesClient{
		outputs: map[string]*sqs.GetQueueAttributesOutput{},
	}

	resources, err := awsclient.FetchSQSQueues(context.Background(), listMock, attrMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}
