// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// CodeSNSSubPendingConfirmation is the canonical FindingCode for a
// subscription that has not yet confirmed its endpoint.
const CodeSNSSubPendingConfirmation domain.FindingCode = "sns-sub.state.pending-confirmation"

// CodeSNSSubDeleted is the canonical FindingCode for a subscription whose
// endpoint has been deleted.
const CodeSNSSubDeleted domain.FindingCode = "sns-sub.state.deleted"

// CodeSNSSubPlainHTTP is the canonical FindingCode for a subscription that
// delivers over unencrypted HTTP.
const CodeSNSSubPlainHTTP domain.FindingCode = "sns-sub.plain-http"

// FetchSNSSubscriptionsPage fetches a single page of SNS subscriptions.
func FetchSNSSubscriptionsPage(ctx context.Context, api SNSListSubscriptionsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &sns.ListSubscriptionsInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListSubscriptions(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching SNS subscriptions: %w", err)
	}

	var resources []resource.Resource

	for _, sub := range output.Subscriptions {
		subscriptionArn := ""
		if sub.SubscriptionArn != nil {
			subscriptionArn = *sub.SubscriptionArn
		}

		topicArn := ""
		if sub.TopicArn != nil {
			topicArn = *sub.TopicArn
		}

		protocol := ""
		if sub.Protocol != nil {
			protocol = *sub.Protocol
		}

		endpoint := ""
		if sub.Endpoint != nil {
			endpoint = *sub.Endpoint
		}

		// Extract topic name from TopicArn (last segment after ":")
		topicName := topicArn
		if parts := strings.Split(topicArn, ":"); len(parts) > 0 {
			topicName = parts[len(parts)-1]
		}

		findings, details := snsSubFindings(subscriptionArn, protocol, endpoint)

		r := resource.Resource{
			ID:   subscriptionArn,
			Name: topicName,
			Fields: map[string]string{
				"topic_arn":        topicArn,
				"protocol":         protocol,
				"endpoint":         endpoint,
				"subscription_arn": subscriptionArn,
			},
			Findings:         findings,
			AttentionDetails: details,
			RawStruct:        sub,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}
	// SNS ListSubscriptions may return a NextToken even with 0 results (known API quirk).
	if len(resources) == 0 {
		isTruncated = false
		nextToken = ""
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

// snsSubFindings is the single source of every sns-sub wave-1 finding: the
// top-level fetcher, the by-topic child fetcher and colorSNSSub's
// cache-restored fallback all call it. Every argument is a Fields key, never
// an SDK struct, so no caller can see a finding another one cannot.
func snsSubFindings(subscriptionArn, protocol, endpoint string) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	findings := snsSubStateFindings(subscriptionArn)
	// A subscription AWS reports as Deleted or still unconfirmed is not
	// carrying traffic, so its transport is not a posture problem yet.
	if subscriptionArn == "Deleted" || protocol != "http" {
		return findings, nil
	}
	findings = append(findings, wave1Finding(CodeSNSSubPlainHTTP))
	return findings, map[domain.FindingCode]domain.AttentionDetail{
		CodeSNSSubPlainHTTP: {Rows: []domain.DetailRow{
			// Scheme and host only. A webhook path is routinely the shared
			// secret that authenticates the caller (rule 7).
			{Label: "Endpoint", Value: snsSubEndpointOrigin(endpoint), Tier: "~"},
		}},
	}
}

// snsSubEndpointOrigin keeps the scheme and host of a URL endpoint and drops
// the path, query, fragment and any userinfo.
func snsSubEndpointOrigin(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return endpoint
	}
	return u.Scheme + "://" + u.Host
}

// snsSubStateFindings mirrors colorSNSSub's own precedence: AWS returns the
// literal strings "PendingConfirmation" / "Deleted" AS the SubscriptionArn
// value for subscriptions in those states.
func snsSubStateFindings(subscriptionArn string) []domain.Finding {
	switch subscriptionArn {
	case "PendingConfirmation":
		return []domain.Finding{wave1Finding(CodeSNSSubPendingConfirmation)}
	case "Deleted":
		return []domain.Finding{wave1Finding(CodeSNSSubDeleted)}
	}
	return nil
}
