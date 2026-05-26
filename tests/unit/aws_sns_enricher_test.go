package unit

// aws_sns_enricher_test.go — Behavioral tests for EnrichSNSSubscriptions.
//
// Contract assertions:
//   - ListSubscriptionsByTopic is called once per SNS resource (keyed by topic ARN).
//   - Both topics have at least one confirmed subscription → 0 findings.
//   - topic-1 has no subscriptions (empty slice) → finding with severity "~", "no subscribers".
//   - topic-1 has subscriptions but all are PendingConfirmation → finding with severity "~", "all pending".
//   - clients.SNS == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for a resource → 0 findings for that resource, Truncated=true, no error returned.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// snsListSubscriptionsByTopicFake implements SNSAPI for enrichment testing.
// It embeds the aggregate interface and overrides only ListSubscriptionsByTopic.
// The results map is keyed by TopicArn (from the input) so the fake can serve
// different responses per resource.
type snsListSubscriptionsByTopicFake struct {
	awsclient.SNSAPI
	// results maps TopicArn → subscriptions. If absent the fake returns errByARN.
	results map[string][]snstypes.Subscription
	// errByARN maps TopicArn → error; overrides results when set.
	errByARN map[string]error
}

func (f *snsListSubscriptionsByTopicFake) ListSubscriptionsByTopic(
	_ context.Context,
	in *sns.ListSubscriptionsByTopicInput,
	_ ...func(*sns.Options),
) (*sns.ListSubscriptionsByTopicOutput, error) {
	arn := ""
	if in != nil && in.TopicArn != nil {
		arn = *in.TopicArn
	}
	if f.errByARN != nil {
		if err, ok := f.errByARN[arn]; ok {
			return nil, err
		}
	}
	subs := f.results[arn]
	return &sns.ListSubscriptionsByTopicOutput{Subscriptions: subs}, nil
}

// Compile-time check: snsListSubscriptionsByTopicFake satisfies SNSAPI.
var _ awsclient.SNSAPI = (*snsListSubscriptionsByTopicFake)(nil)

// snsTopicResources returns a slice of SNS topic Resource stubs with the given names.
// The topic_arn field is set to a realistic ARN derived from the name.
func snsTopicResources(names ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(names))
	for _, name := range names {
		arn := "arn:aws:sns:us-east-1:123456789012:" + name
		res = append(res, resource.Resource{
			ID:   arn,
			Name: name,
			Fields: map[string]string{
				"topic_name": name,
				"topic_arn":  arn,
			},
		})
	}
	return res
}

// snsARNFor returns the topic ARN used by snsTopicResources for the given name.
func snsARNFor(name string) string {
	return "arn:aws:sns:us-east-1:123456789012:" + name
}

// confirmedSub returns a confirmed Subscription for the given topic ARN.
func confirmedSub(topicARN, subARN, protocol, endpoint string) snstypes.Subscription {
	return snstypes.Subscription{
		TopicArn:        aws.String(topicARN),
		SubscriptionArn: aws.String(subARN),
		Protocol:        aws.String(protocol),
		Endpoint:        aws.String(endpoint),
	}
}

// pendingSub returns a PendingConfirmation subscription for the given topic ARN.
func pendingSub(topicARN, protocol, endpoint string) snstypes.Subscription {
	return snstypes.Subscription{
		TopicArn:        aws.String(topicARN),
		SubscriptionArn: aws.String("PendingConfirmation"),
		Protocol:        aws.String(protocol),
		Endpoint:        aws.String(endpoint),
	}
}

// TestEnrichSNSSubscriptions_BothWithSubsProducesNoFindings verifies that when
// both topics have at least one confirmed subscription no findings are produced.
func TestEnrichSNSSubscriptions_BothWithSubsProducesNoFindings(t *testing.T) {
	topic1 := snsARNFor("my-topic-1")
	topic2 := snsARNFor("my-topic-2")
	fake := &snsListSubscriptionsByTopicFake{
		results: map[string][]snstypes.Subscription{
			topic1: {
				confirmedSub(topic1, "arn:aws:sns:us-east-1:123456789012:my-topic-1:sub-aaa", "sqs", "arn:aws:sqs:us-east-1:123456789012:queue-a"),
			},
			topic2: {
				confirmedSub(topic2, "arn:aws:sns:us-east-1:123456789012:my-topic-2:sub-bbb", "email", "ops@example.com"),
			},
		},
	}
	clients := &awsclient.ServiceClients{SNS: fake}
	resources := snsTopicResources("my-topic-1", "my-topic-2")

	result, err := awsclient.EnrichSNSSubscriptions(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichSNSSubscriptions_OrphanTopicProducesFindingSevTilde verifies that
// when topic-1 has an empty subscription list a finding with severity "~" and
// summary containing "no subscribers" is produced for topic-1, and topic-2
// (with a confirmed sub) produces no finding.
func TestEnrichSNSSubscriptions_OrphanTopicProducesFindingSevTilde(t *testing.T) {
	topic1 := snsARNFor("my-topic-1")
	topic2 := snsARNFor("my-topic-2")
	fake := &snsListSubscriptionsByTopicFake{
		results: map[string][]snstypes.Subscription{
			topic1: {}, // empty — orphan topic
			topic2: {
				confirmedSub(topic2, "arn:aws:sns:us-east-1:123456789012:my-topic-2:sub-bbb", "sqs", "arn:aws:sqs:us-east-1:123456789012:queue-b"),
			},
		},
	}
	clients := &awsclient.ServiceClients{SNS: fake}
	resources := snsTopicResources("my-topic-1", "my-topic-2")

	result, err := awsclient.EnrichSNSSubscriptions(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[topic1]
	if !ok {
		t.Fatalf("expected finding keyed by %q", topic1)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if !strings.Contains(strings.ToLower(f.Phrase), "no subscriber") {
		t.Errorf("summary %q must contain \"no subscriber\"", f.Phrase)
	}
	if _, ok := result.Findings[topic2]; ok {
		t.Error("my-topic-2 must NOT appear in Findings — it has a confirmed subscriber")
	}
}

// TestEnrichSNSSubscriptions_AllPendingProducesFindingSevTilde verifies that
// when topic-1 has two subscriptions both in PendingConfirmation state a
// finding with severity "~" and summary containing "all pending" is produced
// for topic-1, and topic-2 (with a confirmed sub) produces no finding.
func TestEnrichSNSSubscriptions_AllPendingProducesFindingSevTilde(t *testing.T) {
	topic1 := snsARNFor("my-topic-1")
	topic2 := snsARNFor("my-topic-2")
	fake := &snsListSubscriptionsByTopicFake{
		results: map[string][]snstypes.Subscription{
			topic1: {
				pendingSub(topic1, "email", "alice@example.com"),
				pendingSub(topic1, "email", "bob@example.com"),
			},
			topic2: {
				confirmedSub(topic2, "arn:aws:sns:us-east-1:123456789012:my-topic-2:sub-ccc", "sqs", "arn:aws:sqs:us-east-1:123456789012:queue-c"),
			},
		},
	}
	clients := &awsclient.ServiceClients{SNS: fake}
	resources := snsTopicResources("my-topic-1", "my-topic-2")

	result, err := awsclient.EnrichSNSSubscriptions(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[topic1]
	if !ok {
		t.Fatalf("expected finding keyed by %q", topic1)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if !strings.Contains(strings.ToLower(f.Phrase), "pending") {
		t.Errorf("summary %q must contain \"pending\"", f.Phrase)
	}
	if _, ok := result.Findings[topic2]; ok {
		t.Error("my-topic-2 must NOT appear in Findings — it has a confirmed subscriber")
	}
}

// TestEnrichSNSSubscriptions_NilClientReturnsEmptyFindingsNoError verifies that
// when clients.SNS is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichSNSSubscriptions_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{SNS: nil}

	result, err := awsclient.EnrichSNSSubscriptions(context.Background(), clients, snsTopicResources("my-topic-1", "my-topic-2"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when SNS client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichSNSSubscriptions_APIErrorSetsTruncatedNoError verifies that when
// the ListSubscriptionsByTopic call for topic-1 returns an error, the enricher
// sets Truncated=true, produces 0 findings for the failed topic, and does not
// propagate the error.
func TestEnrichSNSSubscriptions_APIErrorSetsTruncatedNoError(t *testing.T) {
	apiErr := errors.New("sns: ListSubscriptionsByTopic throttled")
	topic1 := snsARNFor("my-topic-1")
	topic2 := snsARNFor("my-topic-2")
	fake := &snsListSubscriptionsByTopicFake{
		errByARN: map[string]error{
			topic1: apiErr,
		},
		results: map[string][]snstypes.Subscription{
			topic2: {
				confirmedSub(topic2, "arn:aws:sns:us-east-1:123456789012:my-topic-2:sub-ddd", "sqs", "arn:aws:sqs:us-east-1:123456789012:queue-d"),
			},
		},
	}
	clients := &awsclient.ServiceClients{SNS: fake}
	resources := snsTopicResources("my-topic-1", "my-topic-2")

	result, err := awsclient.EnrichSNSSubscriptions(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings[topic1]; ok {
		t.Error("my-topic-1 must NOT have a finding when the API call fails")
	}
	if !result.Truncated {
		t.Error("Truncated must be true when an API call fails")
	}
}
