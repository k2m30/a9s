package unit

// misc4_r2b_sns_toplevel_key_test.go — the account-wide SNS subscription
// list keys a stateless row through the topic.
//
// The list spans every topic, so the word AWS sends in place of an ARN
// would be one identity for every deleted subscription in the account. Two
// topics can each have a deleted subscription to the same address — the
// same on-call mailbox unsubscribed twice is the ordinary case — and
// protocol plus endpoint alone would make those one row.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
)

type misc4ListSubsFake struct {
	subs []snstypes.Subscription
}

func (f *misc4ListSubsFake) ListSubscriptions(_ context.Context, _ *sns.ListSubscriptionsInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsOutput, error) {
	return &sns.ListSubscriptionsOutput{Subscriptions: f.subs}, nil
}

func misc4Sub(arn, topic, protocol, endpoint string) snstypes.Subscription {
	s := snstypes.Subscription{
		TopicArn: aws.String(topic),
		Protocol: aws.String(protocol),
		Endpoint: aws.String(endpoint),
		Owner:    aws.String("123456789012"),
	}
	if arn != "" {
		s.SubscriptionArn = aws.String(arn)
	}
	return s
}

// TestSNSSubListDeletedAcrossTopicsAreDistinctRows pins the account-wide case:
// the same address unsubscribed from two topics is two rows.
func TestSNSSubListDeletedAcrossTopicsAreDistinctRows(t *testing.T) {
	const topicA = "arn:aws:sns:us-east-1:123456789012:acme-orders"
	const topicB = "arn:aws:sns:us-east-1:123456789012:acme-alerts"
	fake := &misc4ListSubsFake{subs: []snstypes.Subscription{
		misc4Sub("Deleted", topicA, "email", "oncall@example.com"),
		misc4Sub("Deleted", topicB, "email", "oncall@example.com"),
		misc4Sub("PendingConfirmation", topicA, "email", "oncall@example.com"),
		misc4Sub("", topicA, "sqs", "arn:aws:sqs:us-east-1:123456789012:acme-one"),
		misc4Sub("", topicB, "sqs", "arn:aws:sqs:us-east-1:123456789012:acme-one"),
	}}

	out, err := awsclient.FetchSNSSubscriptionsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchSNSSubscriptionsPage: %v", err)
	}
	if len(out.Resources) != 5 {
		t.Fatalf("got %d rows, want 5", len(out.Resources))
	}
	seen := map[string]int{}
	for _, r := range out.Resources {
		if r.ID == "" {
			t.Errorf("row on topic %q has no ID at all", r.Fields["topic_arn"])
		}
		if r.ID == "Deleted" || r.ID == "PendingConfirmation" {
			t.Errorf("row takes the literal state word %q as its ID", r.ID)
		}
		seen[r.ID]++
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("%d rows share the ID %q", n, id)
		}
	}
}

// TestSNSSubBothSurfacesKeyStatelessRowsTheSameWay pins that the two fetchers
// key one subscription one way, so a row's identity does not depend on which
// list it was reached from.
func TestSNSSubBothSurfacesKeyStatelessRowsTheSameWay(t *testing.T) {
	const topic = "arn:aws:sns:us-east-1:123456789012:acme-orders"
	sub := misc4Sub("Deleted", topic, "email", "ops@example.com")

	list, err := awsclient.FetchSNSSubscriptionsPage(context.Background(),
		&misc4ListSubsFake{subs: []snstypes.Subscription{sub}}, "")
	if err != nil {
		t.Fatalf("FetchSNSSubscriptionsPage: %v", err)
	}
	child, err := awsclient.FetchSNSTopicSubscriptions(context.Background(),
		&mockSNSListSubscriptionsByTopicClient{
			outputs: []*sns.ListSubscriptionsByTopicOutput{{Subscriptions: []snstypes.Subscription{sub}}},
		}, topic, "")
	if err != nil {
		t.Fatalf("FetchSNSTopicSubscriptions: %v", err)
	}
	if list.Resources[0].ID != child.Resources[0].ID {
		t.Errorf("one subscription, two identities: list %q, child %q",
			list.Resources[0].ID, child.Resources[0].ID)
	}
}

// TestSNSSubDeletedFindingReadsTheNamedConstant pins that the transport check
// reads the state through the same constant the confirmation reading does,
// rather than spelling the word a second time.
func TestSNSSubDeletedFindingReadsTheNamedConstant(t *testing.T) {
	const topic = "arn:aws:sns:us-east-1:123456789012:acme-orders"
	out, err := awsclient.FetchSNSSubscriptionsPage(context.Background(),
		&misc4ListSubsFake{subs: []snstypes.Subscription{
			misc4Sub("Deleted", topic, "http", "http://webhook.example.com/hook"),
		}}, "")
	if err != nil {
		t.Fatalf("FetchSNSSubscriptionsPage: %v", err)
	}
	for _, f := range out.Resources[0].Findings {
		if f.Code == domain.FindingCode("sns-sub.plain-http") {
			t.Errorf("a deleted subscription is not carrying traffic; it must not "+
				"report its transport: %+v", f)
		}
	}
}
