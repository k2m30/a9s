package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/resource"
)

func TestQA_SNSSubscriptions_FetchSuccess(t *testing.T) {
	mock := &mockSNSListSubscriptionsClient{
		output: &sns.ListSubscriptionsOutput{
			Subscriptions: []snstypes.Subscription{
				{
					SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:my-topic:sub-001"),
					TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:my-topic"),
					Protocol:        aws.String("email"),
					Endpoint:        aws.String("user@example.com"),
					Owner:           aws.String("123456789012"),
				},
				{
					SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:alerts:sub-002"),
					TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:alerts"),
					Protocol:        aws.String("https"),
					Endpoint:        aws.String("https://example.com/webhook"),
					Owner:           aws.String("123456789012"),
				},
			},
		},
	}

	resources, err := awsclient.FetchSNSSubscriptions(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	r := resources[0]
	if r.ID != "arn:aws:sns:us-east-1:123456789012:my-topic:sub-001" {
		t.Errorf("expected subscription ARN as ID, got %q", r.ID)
	}
	if r.Name != "my-topic" {
		t.Errorf("expected Name 'my-topic' (extracted from topic ARN), got %q", r.Name)
	}
	if r.Fields["topic_arn"] != "arn:aws:sns:us-east-1:123456789012:my-topic" {
		t.Errorf("expected correct topic_arn, got %q", r.Fields["topic_arn"])
	}
	if r.Fields["protocol"] != "email" {
		t.Errorf("expected protocol 'email', got %q", r.Fields["protocol"])
	}
	if r.Fields["endpoint"] != "user@example.com" {
		t.Errorf("expected endpoint 'user@example.com', got %q", r.Fields["endpoint"])
	}

	r2 := resources[1]
	if r2.Fields["protocol"] != "https" {
		t.Errorf("expected protocol 'https', got %q", r2.Fields["protocol"])
	}
	if r2.RawStruct == nil {
		t.Error("expected RawStruct to be set")
	}
}

func TestQA_SNSSubscriptions_FetchEmpty(t *testing.T) {
	mock := &mockSNSListSubscriptionsClient{
		output: &sns.ListSubscriptionsOutput{
			Subscriptions: []snstypes.Subscription{},
		},
	}

	resources, err := awsclient.FetchSNSSubscriptions(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestQA_SNSSubscriptions_FetchError(t *testing.T) {
	mock := &mockSNSListSubscriptionsClient{
		err: fmt.Errorf("access denied"),
	}

	_, err := awsclient.FetchSNSSubscriptions(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestQA_SNSSubscriptions_TypeDef(t *testing.T) {
	rt := resource.FindResourceType("sns-sub")
	if rt == nil {
		t.Fatal("resource type 'sns-sub' not found")
	}
	if rt.Name != "SNS Subscriptions" {
		t.Errorf("expected Name 'SNS Subscriptions', got %q", rt.Name)
	}
	expected := []struct {
		key   string
		title string
	}{
		{"topic_arn", "Topic ARN"},
		{"protocol", "Protocol"},
		{"endpoint", "Endpoint"},
		{"subscription_arn", "Subscription ARN"},
	}
	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}
	for i, want := range expected {
		if rt.Columns[i].Key != want.key {
			t.Errorf("column %d: expected key %q, got %q", i, want.key, rt.Columns[i].Key)
		}
	}
}
