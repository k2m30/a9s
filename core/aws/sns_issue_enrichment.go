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
		TruncatedIDs: make(map[string]bool),
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
		// Walk all pages so subs_count is exact for topics with >100 subscribers.
		var subs []snstypes.Subscription
		var nextToken *string
		var pagedErr error
		for {
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
			nextToken = out.NextToken
		}

		mu.Lock()
		defer mu.Unlock()
		if pagedErr != nil {
			MarkSkipped(&result, r.ID, &failures, pagedErr)
			return
		}
		result.FieldUpdates[r.ID] = map[string]string{
			"subs_count": resource.FormatExact(len(subs)),
		}
		// Posture is read for every topic, including one with no
		// subscribers: an unencrypted topic nobody listens to is still
		// unencrypted.
		snsTopicPosture(ctx, clients, &result, &failures, r.ID, ownAccount)
		if len(subs) == 0 {
			setWave2Finding(&result, r.ID, snsCodeNoSubscribers, "~", "sns", nil)
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
			setWave2Finding(&result, r.ID, snsCodeAllPending, "~", "sns", nil)
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
			setWave2Finding(result, topicARN, snsCodePublicPolicy, "!", "sns", publicPolicyRows(ex))
		}
	}
	// AWS omits the attribute entirely for an unencrypted topic and returns
	// it empty in some responses; both are the same fact.
	if out.Attributes["KmsMasterKeyId"] == "" {
		setWave2Finding(result, topicARN, snsCodeNoKMS, "~", "sns", []domain.DetailRow{{Label: "Encryption key", Value: "none", Tier: "~"}})
	}
}
