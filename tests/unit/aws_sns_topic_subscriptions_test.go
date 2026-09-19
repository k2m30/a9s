package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestFetchSNSTopicSubscriptions_Basic(t *testing.T) {
	topicArn := "arn:aws:sns:us-east-1:123456789012:my-topic"
	mock := &mockSNSListSubscriptionsByTopicClient{
		outputs: []*sns.ListSubscriptionsByTopicOutput{
			{
				Subscriptions: []snstypes.Subscription{
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:my-topic:a1b2c3d4"),
						TopicArn:        aws.String(topicArn),
						Protocol:        aws.String("email"),
						Endpoint:        aws.String("user@example.com"),
						Owner:           aws.String("123456789012"),
					},
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:my-topic:e5f6g7h8"),
						TopicArn:        aws.String(topicArn),
						Protocol:        aws.String("https"),
						Endpoint:        aws.String("https://api.example.com/webhook"),
						Owner:           aws.String("123456789012"),
					},
					{
						SubscriptionArn: aws.String("PendingConfirmation"),
						TopicArn:        aws.String(topicArn),
						Protocol:        aws.String("sqs"),
						Endpoint:        aws.String("arn:aws:sqs:us-east-1:123456789012:my-queue"),
						Owner:           aws.String("123456789012"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, topicArn, "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	t.Run("email_confirmed_ID", func(t *testing.T) {
		if resources[0].ID != "arn:aws:sns:us-east-1:123456789012:my-topic:a1b2c3d4" {
			t.Errorf("ID: expected confirmed ARN, got %q", resources[0].ID)
		}
	})

	t.Run("email_confirmed_Name", func(t *testing.T) {
		if resources[0].Name != "user@example.com" {
			t.Errorf("Name: expected %q, got %q", "user@example.com", resources[0].Name)
		}
	})

	t.Run("email_confirmed_Fields", func(t *testing.T) {
		r := resources[0]
		if r.Fields["protocol"] != "email" {
			t.Errorf("Fields[protocol]: expected %q, got %q", "email", r.Fields["protocol"])
		}
		if r.Fields["endpoint"] != "user@example.com" {
			t.Errorf("Fields[endpoint]: expected %q, got %q", "user@example.com", r.Fields["endpoint"])
		}
		if r.Fields["owner"] != "123456789012" {
			t.Errorf("Fields[owner]: expected %q, got %q", "123456789012", r.Fields["owner"])
		}
		if r.Fields["topic_arn"] != topicArn {
			t.Errorf("Fields[topic_arn]: expected %q, got %q", topicArn, r.Fields["topic_arn"])
		}
		if r.Fields["subscription_arn"] != "arn:aws:sns:us-east-1:123456789012:my-topic:a1b2c3d4" {
			t.Errorf("Fields[subscription_arn]: expected confirmed ARN, got %q", r.Fields["subscription_arn"])
		}
		if r.Fields["confirmation_status"] != "Confirmed" {
			t.Errorf("Fields[confirmation_status]: expected %q, got %q", "Confirmed", r.Fields["confirmation_status"])
		}
	})

	t.Run("email_confirmed_RawStruct", func(t *testing.T) {
		r := resources[0]
		if r.RawStruct == nil {
			t.Fatal("RawStruct must not be nil")
		}
		raw, ok := r.RawStruct.(snstypes.Subscription)
		if !ok {
			t.Fatalf("RawStruct should be snstypes.Subscription, got %T", r.RawStruct)
		}
		if raw.Protocol == nil || *raw.Protocol != "email" {
			t.Errorf("RawStruct.Protocol: expected %q", "email")
		}
	})

	t.Run("https_confirmed_ID", func(t *testing.T) {
		if resources[1].ID != "arn:aws:sns:us-east-1:123456789012:my-topic:e5f6g7h8" {
			t.Errorf("ID: expected confirmed ARN, got %q", resources[1].ID)
		}
	})

	t.Run("https_confirmed_Fields", func(t *testing.T) {
		r := resources[1]
		if r.Fields["protocol"] != "https" {
			t.Errorf("Fields[protocol]: expected %q, got %q", "https", r.Fields["protocol"])
		}
		if r.Fields["endpoint"] != "https://api.example.com/webhook" {
			t.Errorf("Fields[endpoint]: expected %q, got %q", "https://api.example.com/webhook", r.Fields["endpoint"])
		}
	})

	t.Run("sqs_pending_confirmation_status", func(t *testing.T) {
		r := resources[2]
		if r.Fields["confirmation_status"] != "PendingConfirmation" {
			t.Errorf("Fields[confirmation_status]: expected %q, got %q", "PendingConfirmation", r.Fields["confirmation_status"])
		}
	})
}

func TestFetchSNSTopicSubscriptions_Empty(t *testing.T) {
	mock := &mockSNSListSubscriptionsByTopicClient{
		outputs: []*sns.ListSubscriptionsByTopicOutput{
			{
				Subscriptions: []snstypes.Subscription{},
			},
		},
	}

	result, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, "arn:aws:sns:us-east-1:123456789012:empty-topic", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchSNSTopicSubscriptions_APIError(t *testing.T) {
	mock := &mockSNSListSubscriptionsByTopicClient{
		err: fmt.Errorf("AWS API error: throttling exception"),
	}

	result, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, "arn:aws:sns:us-east-1:123456789012:err-topic", "")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	resources := result.Resources
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d", len(resources))
	}
}

func TestFetchSNSTopicSubscriptions_NilOptionalFields(t *testing.T) {
	mock := &mockSNSListSubscriptionsByTopicClient{
		outputs: []*sns.ListSubscriptionsByTopicOutput{
			{
				Subscriptions: []snstypes.Subscription{
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:topic:sub-nil"),
						TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:topic"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, "arn:aws:sns:us-east-1:123456789012:topic", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	t.Run("protocol_empty", func(t *testing.T) {
		if r.Fields["protocol"] != "" {
			t.Errorf("Fields[protocol]: expected empty, got %q", r.Fields["protocol"])
		}
	})

	t.Run("endpoint_empty", func(t *testing.T) {
		if r.Fields["endpoint"] != "" {
			t.Errorf("Fields[endpoint]: expected empty, got %q", r.Fields["endpoint"])
		}
	})

	t.Run("owner_empty", func(t *testing.T) {
		if r.Fields["owner"] != "" {
			t.Errorf("Fields[owner]: expected empty, got %q", r.Fields["owner"])
		}
	})
}

func TestFetchSNSTopicSubscriptions_ConfirmationStatus(t *testing.T) {
	mock := &mockSNSListSubscriptionsByTopicClient{
		outputs: []*sns.ListSubscriptionsByTopicOutput{
			{
				Subscriptions: []snstypes.Subscription{
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:topic:confirmed-sub"),
						TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:topic"),
						Protocol:        aws.String("email"),
						Endpoint:        aws.String("confirmed@example.com"),
						Owner:           aws.String("123456789012"),
					},
					{
						SubscriptionArn: aws.String("PendingConfirmation"),
						TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:topic"),
						Protocol:        aws.String("email"),
						Endpoint:        aws.String("pending@example.com"),
						Owner:           aws.String("123456789012"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, "arn:aws:sns:us-east-1:123456789012:topic", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	t.Run("confirmed_sub_status", func(t *testing.T) {
		if resources[0].Fields["confirmation_status"] != "Confirmed" {
			t.Errorf("expected %q, got %q", "Confirmed", resources[0].Fields["confirmation_status"])
		}
	})

	t.Run("pending_sub_status", func(t *testing.T) {
		if resources[1].Fields["confirmation_status"] != "PendingConfirmation" {
			t.Errorf("expected %q, got %q", "PendingConfirmation", resources[1].Fields["confirmation_status"])
		}
	})

	t.Run("confirmed_sub_ID_is_ARN", func(t *testing.T) {
		if resources[0].ID != "arn:aws:sns:us-east-1:123456789012:topic:confirmed-sub" {
			t.Errorf("confirmed sub ID should be full ARN, got %q", resources[0].ID)
		}
	})
}

func TestFetchSNSTopicSubscriptions_Pagination(t *testing.T) {
	topicArn := "arn:aws:sns:us-east-1:123456789012:paginated-topic"

	page1Mock := &mockSNSListSubscriptionsByTopicClient{
		outputs: []*sns.ListSubscriptionsByTopicOutput{
			{
				NextToken: aws.String("page2-token"),
				Subscriptions: []snstypes.Subscription{
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:paginated-topic:sub-1"),
						TopicArn:        aws.String(topicArn),
						Protocol:        aws.String("email"),
						Endpoint:        aws.String("page1-user1@example.com"),
						Owner:           aws.String("123456789012"),
					},
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:paginated-topic:sub-2"),
						TopicArn:        aws.String(topicArn),
						Protocol:        aws.String("email"),
						Endpoint:        aws.String("page1-user2@example.com"),
						Owner:           aws.String("123456789012"),
					},
				},
			},
		},
	}

	result1, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), page1Mock, topicArn, "")
	if err != nil {
		t.Fatalf("page 1: expected no error, got %v", err)
	}

	t.Run("page1_item_count", func(t *testing.T) {
		if len(result1.Resources) != 2 {
			t.Fatalf("expected 2 resources on page 1, got %d", len(result1.Resources))
		}
	})

	t.Run("page1_single_api_call", func(t *testing.T) {
		if page1Mock.callIdx != 1 {
			t.Errorf("expected 1 API call for page 1, got %d", page1Mock.callIdx)
		}
	})

	t.Run("page1_is_truncated", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if !result1.Pagination.IsTruncated {
			t.Error("page 1: IsTruncated should be true when NextToken is present")
		}
	})

	t.Run("page1_next_token", func(t *testing.T) {
		if result1.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result1.Pagination.NextToken != "page2-token" {
			t.Errorf("page 1: NextToken expected %q, got %q", "page2-token", result1.Pagination.NextToken)
		}
	})

	t.Run("page1_subscription_names", func(t *testing.T) {
		if result1.Resources[0].Name != "page1-user1@example.com" {
			t.Errorf("resources[0].Name: expected %q, got %q", "page1-user1@example.com", result1.Resources[0].Name)
		}
		if result1.Resources[1].Name != "page1-user2@example.com" {
			t.Errorf("resources[1].Name: expected %q, got %q", "page1-user2@example.com", result1.Resources[1].Name)
		}
	})

	page2Mock := &mockSNSListSubscriptionsByTopicClient{
		outputs: []*sns.ListSubscriptionsByTopicOutput{
			{
				Subscriptions: []snstypes.Subscription{
					{
						SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:paginated-topic:sub-3"),
						TopicArn:        aws.String(topicArn),
						Protocol:        aws.String("https"),
						Endpoint:        aws.String("https://page2.example.com/webhook"),
						Owner:           aws.String("123456789012"),
					},
				},
			},
		},
	}

	result2, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), page2Mock, topicArn, result1.Pagination.NextToken)
	if err != nil {
		t.Fatalf("page 2: expected no error, got %v", err)
	}

	t.Run("page2_item_count", func(t *testing.T) {
		if len(result2.Resources) != 1 {
			t.Fatalf("expected 1 resource on page 2, got %d", len(result2.Resources))
		}
	})

	t.Run("page2_not_truncated", func(t *testing.T) {
		if result2.Pagination == nil {
			t.Fatal("Pagination must not be nil")
		}
		if result2.Pagination.IsTruncated {
			t.Error("page 2: IsTruncated should be false on last page")
		}
	})

	t.Run("page2_subscription_name", func(t *testing.T) {
		if result2.Resources[0].Name != "https://page2.example.com/webhook" {
			t.Errorf("page 2: resource[0].Name expected %q, got %q", "https://page2.example.com/webhook", result2.Resources[0].Name)
		}
	})
}

// A pending subscription's SubscriptionArn is the literal "PendingConfirmation",
// so its ID is composed.
//
// The key carries the topic ARN: both subscription surfaces key a stateless
// row through snsSubRowID, and the account-wide list spans every topic — the
// same address unsubscribed from two topics is two subscriptions, which
// protocol plus endpoint alone would merge into one row.
func TestFetchSNSTopicSubscriptions_PendingIDFormat(t *testing.T) {
	mock := &mockSNSListSubscriptionsByTopicClient{
		outputs: []*sns.ListSubscriptionsByTopicOutput{
			{
				Subscriptions: []snstypes.Subscription{
					{
						SubscriptionArn: aws.String("PendingConfirmation"),
						TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:topic"),
						Protocol:        aws.String("email"),
						Endpoint:        aws.String("user@example.com"),
						Owner:           aws.String("123456789012"),
					},
					{
						SubscriptionArn: aws.String("PendingConfirmation"),
						TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:topic"),
						Protocol:        aws.String("sqs"),
						Endpoint:        aws.String("arn:aws:sqs:us-east-1:123456789012:my-queue"),
						Owner:           aws.String("123456789012"),
					},
				},
			},
		},
	}

	result, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), mock, "arn:aws:sns:us-east-1:123456789012:topic", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	resources := result.Resources

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	t.Run("pending_email_ID", func(t *testing.T) {
		expected := "pending/arn:aws:sns:us-east-1:123456789012:topic/email/user@example.com"
		if resources[0].ID != expected {
			t.Errorf("pending email ID: expected %q, got %q", expected, resources[0].ID)
		}
	})

	t.Run("pending_sqs_ID", func(t *testing.T) {
		expected := "pending/arn:aws:sns:us-east-1:123456789012:topic/sqs/arn:aws:sqs:us-east-1:123456789012:my-queue"
		if resources[1].ID != expected {
			t.Errorf("pending sqs ID: expected %q, got %q", expected, resources[1].ID)
		}
	})
}

func TestSnsSubscriptionColumns(t *testing.T) {
	cols := resource.SnsSubscriptionColumns()

	expectedKeys := []string{"protocol", "endpoint", "confirmation_status", "owner"}
	expectedWidths := []int{10, 48, 18, 14}

	t.Run("column_count", func(t *testing.T) {
		if len(cols) != 4 {
			t.Fatalf("expected 4 columns, got %d", len(cols))
		}
	})

	t.Run("column_keys", func(t *testing.T) {
		for i, expected := range expectedKeys {
			if cols[i].Key != expected {
				t.Errorf("column[%d].Key: expected %q, got %q", i, expected, cols[i].Key)
			}
		}
	})

	t.Run("column_widths", func(t *testing.T) {
		for i, expected := range expectedWidths {
			if cols[i].Width != expected {
				t.Errorf("column[%d].Width: expected %d, got %d", i, expected, cols[i].Width)
			}
		}
	})

	t.Run("columns_have_titles", func(t *testing.T) {
		for i, col := range cols {
			if col.Title == "" {
				t.Errorf("column[%d] (%s) has empty Title", i, col.Key)
			}
		}
	})

	t.Run("columns_have_positive_width", func(t *testing.T) {
		for i, col := range cols {
			if col.Width <= 0 {
				t.Errorf("column[%d] (%s) has non-positive Width: %d", i, col.Key, col.Width)
			}
		}
	})

}

func TestSnsSubscriptions_ChildTypeRegistered(t *testing.T) {
	td := resource.GetChildType("sns_subscriptions")
	if td == nil {
		t.Fatal("sns_subscriptions child resource type not registered")
	}
	if td.Name == "" {
		t.Error("child type Name should not be empty")
	}
	if td.ShortName != "sns_subscriptions" {
		t.Errorf("child type ShortName: expected %q, got %q", "sns_subscriptions", td.ShortName)
	}
}

func TestSnsSubscriptions_PaginatedChildFetcherRegistered(t *testing.T) {
	f := resource.GetPaginatedChildFetcher("sns_subscriptions")
	if f == nil {
		t.Fatal("sns_subscriptions paginated child fetcher not registered")
	}
}

func TestSnsSubscriptions_ParentHasChildDef(t *testing.T) {
	rt := resource.FindResourceType("sns")
	if rt == nil {
		t.Fatal("sns resource type not found")
	}

	found := false
	for _, child := range rt.Children {
		if child.ChildType == "sns_subscriptions" {
			found = true
			if child.Key != "enter" {
				t.Errorf("expected key %q, got %q", "enter", child.Key)
			}
			if child.ContextKeys["topic_arn"] != "ID" {
				t.Errorf("ContextKeys[topic_arn]: expected %q, got %q", "ID", child.ContextKeys["topic_arn"])
			}
			if child.DisplayNameKey != "display_name" {
				t.Errorf("DisplayNameKey: expected %q, got %q", "display_name", child.DisplayNameKey)
			}
		}
	}
	if !found {
		t.Error("sns Children should contain sns_subscriptions child view def")
	}
}

func TestSnsSubscriptions_CopyField(t *testing.T) {
	td := resource.GetChildType("sns_subscriptions")
	if td == nil {
		t.Fatal("sns_subscriptions child type not found")
	}
	if td.CopyField != "endpoint" {
		t.Errorf("CopyField: expected %q, got %q", "endpoint", td.CopyField)
	}
}

func TestFetchSNSTopicSubscriptions_ContinuationToken(t *testing.T) {
	wrapper := &tokenCapturingSNSSubsMock{
		inner: &mockSNSListSubscriptionsByTopicClient{
			outputs: []*sns.ListSubscriptionsByTopicOutput{
				{
					Subscriptions: []snstypes.Subscription{
						{
							Protocol:        aws.String("email"),
							Endpoint:        aws.String("user@example.com"),
							SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:topic:sub-id"),
							TopicArn:        aws.String("arn:aws:sns:us-east-1:123456789012:my-topic"),
						},
					},
				},
			},
		},
	}

	result, err := awsclient.FetchSNSTopicSubscriptions(context.Background(), wrapper, "arn:aws:sns:us-east-1:123456789012:my-topic", "my-continuation-token")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}

	if wrapper.capturedNextToken == nil {
		t.Fatal("expected NextToken to be set in API call")
	}
	if *wrapper.capturedNextToken != "my-continuation-token" {
		t.Errorf("expected NextToken %q, got %q", "my-continuation-token", *wrapper.capturedNextToken)
	}
}

type tokenCapturingSNSSubsMock struct {
	inner             *mockSNSListSubscriptionsByTopicClient
	capturedNextToken *string
}

func (m *tokenCapturingSNSSubsMock) ListSubscriptionsByTopic(ctx context.Context, params *sns.ListSubscriptionsByTopicInput, optFns ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	m.capturedNextToken = params.NextToken
	return m.inner.ListSubscriptionsByTopic(ctx, params, optFns...)
}
