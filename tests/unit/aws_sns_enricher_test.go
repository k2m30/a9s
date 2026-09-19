package unit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type snsListSubscriptionsByTopicFake struct {
	awsclient.SNSAPI
	results  map[string][]snstypes.Subscription
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

var _ awsclient.SNSAPI = (*snsListSubscriptionsByTopicFake)(nil)

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

func snsARNFor(name string) string {
	return "arn:aws:sns:us-east-1:123456789012:" + name
}

func confirmedSub(topicARN, subARN, protocol, endpoint string) snstypes.Subscription {
	return snstypes.Subscription{
		TopicArn:        aws.String(topicARN),
		SubscriptionArn: aws.String(subARN),
		Protocol:        aws.String(protocol),
		Endpoint:        aws.String(endpoint),
	}
}

func pendingSub(topicARN, protocol, endpoint string) snstypes.Subscription {
	return snstypes.Subscription{
		TopicArn:        aws.String(topicARN),
		SubscriptionArn: aws.String("PendingConfirmation"),
		Protocol:        aws.String(protocol),
		Endpoint:        aws.String(endpoint),
	}
}

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
}

func TestEnrichSNSSubscriptions_OrphanTopicProducesFindingSevTilde(t *testing.T) {
	topic1 := snsARNFor("my-topic-1")
	topic2 := snsARNFor("my-topic-2")
	fake := &snsListSubscriptionsByTopicFake{
		results: map[string][]snstypes.Subscription{
			topic1: {},
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
	fs, ok := result.Findings[topic1]
	if !ok {
		t.Fatalf("expected finding keyed by %q", topic1)
	}
	f := fs[0]
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
	fs, ok := result.Findings[topic1]
	if !ok {
		t.Fatalf("expected finding keyed by %q", topic1)
	}
	f := fs[0]
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

// sns is a "~"-only enricher (IssueCount is always 0), so a coverage gap
// never lower-bounds the issue badge: Truncated stays false.
func TestEnrichSNSSubscriptions_APIErrorMarksRowTruncatedIDNotBadge(t *testing.T) {
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
	// The call that failed is recorded, so the row never renders "?" with
	// nothing in the error log to say what refused.
	if err == nil {
		t.Fatal("a failed per-resource call returned no error — the reason never reaches the log")
	}
	if _, ok := result.Findings[topic1]; ok {
		t.Error("my-topic-1 must NOT have a finding when the API call fails")
	}
	if result.Truncated {
		t.Error("Truncated must stay false: sns is a \"~\"-only enricher, so an API error marks the row via TruncatedIDs, never the aggregate issue badge")
	}
	if _, marked := result.TruncatedIDs[topic1]; !marked {
		t.Errorf("TruncatedIDs[%q] must be true — the ListSubscriptionsByTopic error must mark that topic's row with a \"?\" coverage gap", topic1)
	}
}

// The fake returns a healthy attribute set: an empty map is not neutral — a
// missing KmsMasterKeyId means the topic is unencrypted and a missing Policy is
// read as unknown — and would add an encryption finding to every scenario in
// this file.
func (f *snsListSubscriptionsByTopicFake) GetTopicAttributes(_ context.Context, in *sns.GetTopicAttributesInput, _ ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	arn := ""
	if in != nil && in.TopicArn != nil {
		arn = *in.TopicArn
	}
	return &sns.GetTopicAttributesOutput{Attributes: map[string]string{
		"TopicArn":       arn,
		"KmsMasterKeyId": "alias/aws/sns",
		"Policy":         `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"SNS:Publish","Resource":"` + arn + `"}]}`,
	}}, nil
}
