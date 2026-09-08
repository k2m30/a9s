// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchSNSTopicSubscriptions calls the SNS ListSubscriptionsByTopic API and
// converts the response into a FetchResult with pagination support. A single
// API call is made per invocation; IsTruncated and NextToken are forwarded as
// pagination metadata for the caller to request the next page.
func FetchSNSTopicSubscriptions(ctx context.Context, api SNSListSubscriptionsByTopicAPI, topicArn string, continuationToken string) (resource.FetchResult, error) {
	input := &sns.ListSubscriptionsByTopicInput{
		TopicArn: &topicArn,
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListSubscriptionsByTopic(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching SNS topic subscriptions: %w", err)
	}

	var resources []resource.Resource
	for _, sub := range output.Subscriptions {
		resources = append(resources, convertSNSSubscription(sub))
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// convertSNSSubscription converts an SNS Subscription into a generic Resource.
func convertSNSSubscription(sub snstypes.Subscription) resource.Resource {
	protocol := ""
	if sub.Protocol != nil {
		protocol = *sub.Protocol
	}

	endpoint := ""
	if sub.Endpoint != nil {
		endpoint = *sub.Endpoint
	}

	owner := ""
	if sub.Owner != nil {
		owner = *sub.Owner
	}

	subscriptionArn := ""
	if sub.SubscriptionArn != nil {
		subscriptionArn = *sub.SubscriptionArn
	}

	topicArn := ""
	if sub.TopicArn != nil {
		topicArn = *sub.TopicArn
	}

	// The child view has its own words for the confirmation state, but not its
	// own reading of it: snsSubConfirmation is the one place the ARN is
	// interpreted, so this column and the subscription list's Confirmed column
	// cannot disagree about the same subscription.
	confirmationStatus := "Confirmed"
	id := subscriptionArn
	switch snsSubConfirmation(subscriptionArn) {
	case snsSubPending:
		confirmationStatus = snsSubArnPending
		// A pending subscription has no ARN to be identified by, so the row is
		// keyed on what it does have.
		id = fmt.Sprintf("pending/%s/%s", protocol, endpoint)
	case snsSubDeleted:
		confirmationStatus = snsSubArnDeleted
	case snsSubUnknown:
		confirmationStatus = "Unknown"
	}

	findings, details := snsSubFindings(subscriptionArn, protocol, endpoint)

	return resource.Resource{
		ID:   id,
		Name: endpoint,
		Fields: map[string]string{
			"protocol":            protocol,
			"endpoint":            endpoint,
			"confirmation_status": confirmationStatus,
			"owner":               owner,
			"subscription_arn":    subscriptionArn,
			"topic_arn":           topicArn,
		},
		// snsSubFindings (sns_sub.go) is the single source both fetchers
		// and the colour fallback share, so this list can never differ from
		// what the top-level subscription list shows for the same row.
		Findings:         findings,
		AttentionDetails: details,
		RawStruct:        sub,
	}
}
