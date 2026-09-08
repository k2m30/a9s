package unit

// misc4_r2_sns_deleted_id_test.go — misc4 row 2.
//
// AWS reports a deleted subscription by putting the word "Deleted" where the
// SubscriptionArn goes, so every deleted subscription under one topic arrives
// carrying the same string. Taking that string as the row ID makes them one
// identity: the store's append-page dedup drops the second, and the cursor
// sitting on it is left pointing at the first. The pending case already keys
// on what the row does have — its protocol and endpoint — and the deleted
// case is the same problem.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

const misc4SNSTopicARN = "arn:aws:sns:us-east-1:123456789012:acme-orders"

func misc4DeletedSubs(t *testing.T, endpoints ...string) []resource.Resource {
	t.Helper()
	subs := make([]snstypes.Subscription, 0, len(endpoints))
	for _, e := range endpoints {
		subs = append(subs, snstypes.Subscription{
			SubscriptionArn: aws.String("Deleted"),
			TopicArn:        aws.String(misc4SNSTopicARN),
			Protocol:        aws.String("email"),
			Endpoint:        aws.String(e),
			Owner:           aws.String("123456789012"),
		})
	}
	mock := &mockSNSListSubscriptionsByTopicClient{
		outputs: []*sns.ListSubscriptionsByTopicOutput{{Subscriptions: subs}},
	}
	out, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, misc4SNSTopicARN, "")
	if err != nil {
		t.Fatalf("FetchSNSTopicSubscriptions: %v", err)
	}
	return out.Resources
}

// TestSNSSubByTopicDeletedRowsHaveDistinctIDs pins that two deleted
// subscriptions under one topic are two identities.
func TestSNSSubByTopicDeletedRowsHaveDistinctIDs(t *testing.T) {
	rows := misc4DeletedSubs(t, "ops@example.com", "oncall@example.com")
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].ID == rows[1].ID {
		t.Fatalf("both deleted subscriptions share the row ID %q", rows[0].ID)
	}
	for _, r := range rows {
		if r.ID == "Deleted" {
			t.Errorf("row %q takes the literal state word as its ID", r.Name)
		}
		if r.Fields["confirmation_status"] != "Deleted" {
			t.Errorf("row %q: confirmation_status = %q, want \"Deleted\"",
				r.Name, r.Fields["confirmation_status"])
		}
	}
}

// TestSNSSubByTopicDeletedSecondPageSurvivesTheAppend pins the damage the
// collision does: the child view pages, and the store's append-page dedup
// drops any row whose ID it has already seen. A cursor left on the second
// deleted subscription must still find it there.
func TestSNSSubByTopicDeletedSecondPageSurvivesTheAppend(t *testing.T) {
	page1 := misc4DeletedSubs(t, "ops@example.com")
	page2 := misc4DeletedSubs(t, "oncall@example.com")

	store := session.NewRowStore()
	store.Observe("sns-sub-by-topic", page1, nil, session.OriginFetch, false)
	rows, _ := store.Observe("sns-sub-by-topic", page2, nil, session.OriginFetch, true)

	if len(rows) != 2 {
		t.Fatalf("after appending the second page the store holds %d rows, want 2", len(rows))
	}
	if rows[1].Name != "oncall@example.com" {
		t.Errorf("cursor row 2 is %q, want \"oncall@example.com\"", rows[1].Name)
	}
}

// TestSNSSubByTopicUnknownRowsHaveDistinctIDs pins the sibling the deleted
// case's probe turned up: a subscription that comes back with no
// SubscriptionArn at all takes the empty string as its ID, so two of them
// under one topic collide exactly as two deleted ones did.
func TestSNSSubByTopicUnknownRowsHaveDistinctIDs(t *testing.T) {
	mock := &mockSNSListSubscriptionsByTopicClient{
		outputs: []*sns.ListSubscriptionsByTopicOutput{{
			Subscriptions: []snstypes.Subscription{
				{TopicArn: aws.String(misc4SNSTopicARN), Protocol: aws.String("sqs"),
					Endpoint: aws.String("arn:aws:sqs:us-east-1:123456789012:acme-one")},
				{TopicArn: aws.String(misc4SNSTopicARN), Protocol: aws.String("sqs"),
					Endpoint: aws.String("arn:aws:sqs:us-east-1:123456789012:acme-two")},
			},
		}},
	}
	out, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, misc4SNSTopicARN, "")
	if err != nil {
		t.Fatalf("FetchSNSTopicSubscriptions: %v", err)
	}
	if len(out.Resources) != 2 {
		t.Fatalf("got %d rows, want 2", len(out.Resources))
	}
	for _, r := range out.Resources {
		if r.ID == "" {
			t.Errorf("row %q has no ID at all", r.Name)
		}
	}
	if out.Resources[0].ID == out.Resources[1].ID {
		t.Errorf("both rows share the ID %q", out.Resources[0].ID)
	}
}
