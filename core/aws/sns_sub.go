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
			ID:   snsSubRowID(subscriptionArn, topicArn, protocol, endpoint),
			Name: topicName,
			Fields: map[string]string{
				"topic_arn":        topicArn,
				"protocol":         protocol,
				"endpoint":         endpoint,
				"subscription_arn": snsSubARNOrEmpty(subscriptionArn),
				"confirmed":        snsSubConfirmedWord(subscriptionArn),
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
	if snsSubConfirmation(subscriptionArn) == snsSubDeleted || protocol != "http" {
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

// AWS returns these two words in place of the SubscriptionArn for a
// subscription in either state — they are the state, not an ARN. Named so the
// finding and the Confirmed column read the same two literals.
const (
	snsSubArnPending = "PendingConfirmation"
	snsSubArnDeleted = "Deleted"
)

// snsSubStateFindings mirrors colorSNSSub's own precedence.
func snsSubStateFindings(subscriptionArn string) []domain.Finding {
	switch subscriptionArn {
	case snsSubArnPending:
		return []domain.Finding{wave1Finding(CodeSNSSubPendingConfirmation)}
	case snsSubArnDeleted:
		return []domain.Finding{wave1Finding(CodeSNSSubDeleted)}
	}
	return nil
}

// The four states a subscription can be in as far as confirmation goes.
const (
	snsSubConfirmed = "confirmed"
	snsSubPending   = "pending"
	snsSubDeleted   = "deleted"
	snsSubUnknown   = "unknown"
)

// snsSubConfirmation is the one reading of whether an endpoint confirmed. The
// SubscriptionArn is the only evidence either surface has — AWS puts the state
// there instead of an ARN — and a subscription that came back with neither has
// confirmed nothing. Both the subscription list and the by-topic child call
// this and then word the answer for their own column; neither reads the ARN
// for itself, so the two lists cannot disagree about the same subscription.
func snsSubConfirmation(subscriptionArn string) string {
	switch subscriptionArn {
	case snsSubArnPending:
		return snsSubPending
	case snsSubArnDeleted:
		return snsSubDeleted
	case "":
		return snsSubUnknown
	}
	return snsSubConfirmed
}

// snsSubRowID is the row identity both subscription surfaces use. A confirmed
// subscription is its ARN. For the other three states AWS puts the state word
// where the ARN goes, or sends nothing at all, so the row is keyed on what it
// does have — and taking the state word itself would make every subscription
// in that state one identity, which the row store's page dedup then collapses
// to a single row.
//
// The topic is part of the key because the account-wide list spans every
// topic: the same address unsubscribed from two topics is two subscriptions,
// and protocol plus endpoint alone would merge them. The by-topic child gets
// the same key for the same subscription, so a row's identity does not depend
// on which list it was reached from.
func snsSubRowID(subscriptionArn, topicArn, protocol, endpoint string) string {
	state := snsSubConfirmation(subscriptionArn)
	if state == snsSubConfirmed {
		return subscriptionArn
	}
	return strings.Join([]string{state, topicArn, protocol, endpoint}, "/")
}

// snsSubARNOrEmpty is the subscription's ARN, or empty when there is none.
// AWS puts the state word in that field instead, and a column headed
// "Subscription ARN" showing the word "PendingConfirmation" is telling the
// reader an SDK enum where it promised an identifier. The state has its own
// column, which snsSubConfirmedWord words.
func snsSubARNOrEmpty(subscriptionArn string) string {
	if snsSubConfirmation(subscriptionArn) == snsSubConfirmed {
		return subscriptionArn
	}
	return ""
}

// snsSubConfirmedWord is the subscription list's Confirmed column: under that
// heading the confirmed state is a plain "yes".
func snsSubConfirmedWord(subscriptionArn string) string {
	if state := snsSubConfirmation(subscriptionArn); state != snsSubConfirmed {
		return state
	}
	return "yes"
}
