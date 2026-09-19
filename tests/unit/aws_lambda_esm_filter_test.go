package unit

// Tests for the LambdaFake.ListEventSourceMappings ARN filter
// (core/demo/fakes/lambda.go). The fixture has exactly one event source
// mapping: process-orders ← arn:aws:sqs:us-east-1:123456789012:order-processing-queue.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/core/demo/fakes"
)

const fixtSQSOrdersARN = "arn:aws:sqs:us-east-1:123456789012:order-processing-queue"

// TestLambdaFake_ListEventSourceMappings_NilARN verifies that passing nil
// EventSourceArn returns all mappings without filtering.
func TestLambdaFake_ListEventSourceMappings_NilARN(t *testing.T) {
	f := fakes.NewLambda()
	out, err := f.ListEventSourceMappings(context.Background(), &lambda.ListEventSourceMappingsInput{
		EventSourceArn: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.EventSourceMappings) == 0 {
		t.Error("expected at least one mapping when EventSourceArn is nil (no filter applied)")
	}
}

// TestLambdaFake_ListEventSourceMappings_MatchingARN verifies that passing the
// SQS ARN for process-orders returns exactly one mapping with the correct UUID.
func TestLambdaFake_ListEventSourceMappings_MatchingARN(t *testing.T) {
	f := fakes.NewLambda()

	allOut, _ := f.ListEventSourceMappings(context.Background(), &lambda.ListEventSourceMappingsInput{})
	total := len(allOut.EventSourceMappings)

	out, err := f.ListEventSourceMappings(context.Background(), &lambda.ListEventSourceMappingsInput{
		EventSourceArn: aws.String(fixtSQSOrdersARN),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.EventSourceMappings) != 1 {
		t.Errorf("len(mappings) = %d, want 1 for SQS ARN %q", len(out.EventSourceMappings), fixtSQSOrdersARN)
	}
	if len(out.EventSourceMappings) >= total && total > 1 {
		t.Errorf("filter returned same count as unfiltered (%d); ARN filter is not applied", total)
	}
	if len(out.EventSourceMappings) == 1 {
		got := aws.ToString(out.EventSourceMappings[0].EventSourceArn)
		if got != fixtSQSOrdersARN {
			t.Errorf("EventSourceArn = %q, want %q", got, fixtSQSOrdersARN)
		}
		if aws.ToString(out.EventSourceMappings[0].UUID) != "esm-process-orders-01" {
			t.Errorf("UUID = %q, want %q", aws.ToString(out.EventSourceMappings[0].UUID), "esm-process-orders-01")
		}
	}
}

// TestLambdaFake_ListEventSourceMappings_UnknownARN verifies that an ARN not
// present in the fixture returns an empty list (not an error).
func TestLambdaFake_ListEventSourceMappings_UnknownARN(t *testing.T) {
	f := fakes.NewLambda()
	out, err := f.ListEventSourceMappings(context.Background(), &lambda.ListEventSourceMappingsInput{
		EventSourceArn: aws.String("arn:aws:sqs:us-east-1:123456789012:nonexistent-queue"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.EventSourceMappings) != 0 {
		t.Errorf("len(mappings) = %d, want 0 for unknown ARN", len(out.EventSourceMappings))
	}
}
