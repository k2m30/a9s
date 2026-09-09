// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// sns_issue_enrichment.go — Wave 2 issue enrichment for the sns resource type.
package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	snssvc "github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// sns canonical FindingCodes.
const (
	snsCodeNoSubscribers domain.FindingCode = "sns.no-subscribers"
	snsCodeAllPending    domain.FindingCode = "sns.all-pending-confirmation"
	snsCodePublicPolicy  domain.FindingCode = "sns.public-policy"
	snsCodeNoKMS         domain.FindingCode = "sns.no-kms"
)

// S5 operator sentences for the topic codes. The subscriber ones carry no
// supporting row: the phrase is the whole fact and a row repeating it is the
// detail block printing one thing twice.
// EnrichSNSSubscriptions calls ListSubscriptionsByTopic per topic (cap EnrichmentCap)
// to surface orphan topics and topics with all-pending-confirmation subscribers.
// Per-topic errors are treated as truncated (skip silently).
func EnrichSNSSubscriptions(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.SNS == nil {
		return result, nil
	}
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		// fetcher emits ID=ARN
		topicARN := r.ID
		if topicARN == "" {
			return
		}
		// Walk all pages so subs_count is exact for topics with >100 subscribers,
		// up to PerParentPageCap: ListSubscriptionsByTopic has no MaxResults
		// param (page size is a fixed 100), and SNS's own per-topic quota
		// (12.5M subscriptions) is 125,000 pages — far beyond what a single
		// enrichment pass can walk.
		var subs []snstypes.Subscription
		var nextToken *string
		var pagedErr error
		pageCapped := false
		for pages := 0; ; pages++ {
			out, err := clients.SNS.ListSubscriptionsByTopic(ctx, &snssvc.ListSubscriptionsByTopicInput{
				TopicArn:  aws.String(topicARN),
				NextToken: nextToken,
			})
			if err != nil {
				pagedErr = err
				break
			}
			subs = append(subs, out.Subscriptions...)
			if out.NextToken == nil || *out.NextToken == "" {
				break
			}
			if pages+1 >= PerParentPageCap {
				pageCapped = true
				break
			}
			nextToken = out.NextToken
		}

		mu.Lock()
		defer mu.Unlock()
		if pagedErr != nil {
			MarkSkipped(&result, r.ID, &failures, pagedErr)
			return
		}
		count := resource.FormatExact(len(subs))
		if pageCapped {
			// A page cap, not a failed call: there is no error to record, and
			// the row is not uninspected either — the "+" on the count is
			// where the cap is reported. Marking the ID truncated would make
			// FoldWave2Rows skip the row entirely, dropping the posture
			// findings below with it.
			count = resource.FormatTruncated(len(subs))
		}
		result.FieldUpdates[r.ID] = map[string]string{"subs_count": count}
		// Posture is read for every topic, including one with no
		// subscribers and one whose subscription walk was capped: the access
		// policy and the encryption key come from GetTopicAttributes, which
		// the subscription walk's completeness has no bearing on.
		snsTopicPosture(ctx, clients, &result, &failures, r.ID, ownAccount)
		if pageCapped {
			// A capped walk can never rule out a confirmed subscriber sitting
			// beyond the pages inspected — reporting allPending here would be
			// a FALSE "!"-shaped finding (a real confirmed subscriber on page
			// 11 misreported as "all pending"), unlike subs_count, which is
			// an honest lower bound. no-subscribers cannot fire here: subs is
			// always non-empty by the time the walk runs long enough to hit
			// the cap.
			return
		}
		if len(subs) == 0 {
			setWave2Finding(&result, r.ID, snsCodeNoSubscribers, nil)
			return
		}
		allPending := true
		for _, sub := range subs {
			arn := ""
			if sub.SubscriptionArn != nil {
				arn = *sub.SubscriptionArn
			}
			if arn != "PendingConfirmation" {
				allPending = false
				break
			}
		}
		if allPending {
			setWave2Finding(&result, r.ID, snsCodeAllPending, nil)
		}
	})
	return result, AggregateFailures("topic posture and subscriptions", failures, n)
}

// snsTopicPosture reads the topic's own attributes — the access policy and
// the encryption key — and records what they expose. Called with the
// enricher's mutex held.
func snsTopicPosture(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, failures *[]Failure, topicARN, ownAccount string) {
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*snssvc.GetTopicAttributesOutput, error) {
		return clients.SNS.GetTopicAttributes(ctx, &snssvc.GetTopicAttributesInput{
			TopicArn: aws.String(topicARN),
		})
	})
	if err != nil {
		MarkSkipped(result, topicARN, failures, err)
		return
	}
	if doc, parseErr := iampolicy.Parse(out.Attributes["Policy"]); parseErr == nil {
		if ex := iampolicy.Evaluate(doc, ownAccount); ex.Public {
			setWave2Finding(result, topicARN, snsCodePublicPolicy, publicPolicyRows(ex))
		}
	}
	// AWS omits the attribute entirely for an unencrypted topic and returns
	// it empty in some responses; both are the same fact.
	if out.Attributes["KmsMasterKeyId"] == "" {
		setWave2Finding(result, topicARN, snsCodeNoKMS, []domain.DetailRow{{Label: "Encryption key", Value: "none", Tier: tierOf(snsCodeNoKMS)}})
	}
}
