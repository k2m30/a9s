// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// TopicEnriched wraps snstypes.Topic with the topic's attributes from
// GetTopicAttributes — the list call (ListTopics) only carries TopicArn.
type TopicEnriched struct {
	snstypes.Topic
	Attributes map[string]any `json:"Attributes,omitempty" yaml:"Attributes,omitempty"`
}

// enrichSns fetches topic attributes via GetTopicAttributes, parsing JSON
// values (Policy, DeliveryPolicy, EffectiveDeliveryPolicy) into structured
// data via parseJSONOrRaw for readable YAML/JSON rendering. Uncached: a
// single cheap call, and SubscriptionsConfirmed/SubscriptionsPending drift
// as subscribers (un)confirm during a session.
func enrichSns(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	return enrichDetail(ctx, clients, res, detailEnrichSpec[snstypes.Topic, map[string]any]{
		unwrap: unwrapEnriched(func(w TopicEnriched) snstypes.Topic { return w.Topic }),
		id: func(topic snstypes.Topic, _ resource.Resource) (string, error) {
			if topic.TopicArn == nil || *topic.TopicArn == "" {
				return "", fmt.Errorf("topic has no ARN")
			}
			return *topic.TopicArn, nil
		},
		fetch: func(ctx context.Context, c *ServiceClients, id string, _ snstypes.Topic, _ resource.Resource) (map[string]any, error) {
			api, ok := c.SNS.(SNSGetTopicAttributesAPI)
			if !ok {
				return nil, fmt.Errorf("SNS client does not support GetTopicAttributes")
			}
			out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sns.GetTopicAttributesOutput, error) {
				return api.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: &id})
			})
			if err != nil {
				return nil, err
			}
			attrs := make(map[string]any, len(out.Attributes))
			for k, v := range out.Attributes {
				if len(v) > 0 && (v[0] == '{' || v[0] == '[') {
					attrs[k] = parseJSONOrRaw(v)
					continue
				}
				attrs[k] = v
			}
			return attrs, nil
		},
		wrap: func(topic snstypes.Topic, attrs map[string]any) any {
			return TopicEnriched{Topic: topic, Attributes: attrs}
		},
	})
}
