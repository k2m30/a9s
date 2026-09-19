// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type dupIDSNSFake struct {
	awsclient.SNSAPI
}

func (f *dupIDSNSFake) ListSubscriptionsByTopic(_ context.Context, in *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	arn := aws.ToString(in.TopicArn)
	return &sns.ListSubscriptionsByTopicOutput{Subscriptions: []snstypes.Subscription{
		{SubscriptionArn: aws.String(arn + ":11111111-2222-3333-4444-555555555555"), Protocol: aws.String("sqs")},
	}}, nil
}

func (f *dupIDSNSFake) GetTopicAttributes(_ context.Context, in *sns.GetTopicAttributesInput, _ ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	return &sns.GetTopicAttributesOutput{Attributes: map[string]string{"TopicArn": aws.ToString(in.TopicArn)}}, nil
}

// One resource listed twice states its condition once, and its supporting
// rows accumulate — setWave2Finding's rule for a repeated code. The parallel
// enricher evaluates each listing into a result of its own, so the rule must
// hold across the merge of two results, or the detail view prints the same
// sentence twice and the stacked list phrase counts one topic's missing key as
// two.
func TestEnricherMerge_DuplicateResourceIDRaisesTheFindingOnce(t *testing.T) {
	const topic = "arn:aws:sns:us-east-1:123456789012:orders-events"

	res, err := w2Enricher(t, "sns")(context.Background(), &awsclient.ServiceClients{SNS: &dupIDSNSFake{}},
		[]resource.Resource{w2Res(topic, nil), w2Res(topic, nil)}, nil)
	if err != nil {
		t.Fatalf("EnrichSNSSubscriptions: %v", err)
	}

	w2AssertFindingRaisedOnce(t, res, topic, "sns.no-kms")
	if rows := res.AttentionDetails[topic]["sns.no-kms"].Rows; len(rows) != 2 {
		t.Errorf("sns.no-kms carries %d supporting rows for %s, want 2 (one per listing, the way a repeated code accumulates) — %v", len(rows), topic, rows)
	}
}
