// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// sns_related.go contains SNS topic related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sns"

	"github.com/k2m30/a9s/v3/core/resource"
)

// snsGetTopicAttrs wraps GetTopicAttributes in RetryOnThrottle. Returns nil on
// any failure (unsupported client, API error, empty output).
func snsGetTopicAttrs(ctx context.Context, clients any, topicARN string) map[string]string {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.SNS == nil {
		return nil
	}
	api, ok := c.SNS.(SNSGetTopicAttributesAPI)
	if !ok {
		return nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*sns.GetTopicAttributesOutput, error) {
		return api.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: &topicARN})
	})
	// Both callers render this nil as UnknownRelated, so the pivot shows "?"
	// rather than "no key" or "no subscriptions".
	// no finding: nil is this helper's unknown.
	if err != nil || out == nil {
		return nil
	}
	return out.Attributes
}

// checkSNSAlarm reports the CloudWatch alarms on this topic's own metrics
// and the ones that notify it.
func checkSNSAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "sns", res)
}

// checkSNSSub searches the sns-sub cache for subscriptions whose topic_arn
// matches this SNS topic's ARN (Pattern C — reverse lookup in sns-sub cache).
func checkSNSSub(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		topicARN = res.ID
	}
	if topicARN == "" {
		return resource.UnknownRelated("sns-sub")
	}

	subList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "sns-sub")
	if err != nil {
		return resource.ErrorRelated("sns-sub", err)
	}
	if subList == nil {
		return resource.UnknownRelated("sns-sub")
	}

	var ids []string
	for _, subRes := range subList {
		if subRes.Fields["topic_arn"] == topicARN {
			ids = append(ids, subRes.ID)
		}
	}
	return relatedResultTrunc("sns-sub", ids, truncated)
}

// checkSNSKMS resolves the KMS key used for at-rest encryption of this SNS topic
// via GetTopicAttributes (Pattern C: 1 API call, attribute "KmsMasterKeyId").
func checkSNSKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		topicARN = res.ID
	}
	if topicARN == "" {
		return resource.ProvenZero("kms", "topicARN")
	}
	attrs := snsGetTopicAttrs(ctx, clients, topicARN)
	if attrs == nil {
		return resource.UnknownRelated("kms")
	}
	keyID := attrs["KmsMasterKeyId"]
	if keyID == "" {
		return resource.ProvenZero("kms", "keyID")
	}
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkSNSRole lists the IAM roles the SNS topic's access policy
// (GetTopicAttributes "Policy") grants. Pattern C: 1 API call.
func checkSNSRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		topicARN = res.ID
	}
	if topicARN == "" {
		return resource.ProvenZero("role", "topicARN")
	}
	attrs := snsGetTopicAttrs(ctx, clients, topicARN)
	if attrs == nil {
		return resource.UnknownRelated("role")
	}
	policy := attrs["Policy"]
	if policy == "" {
		return resource.ProvenZero("role", "policy")
	}
	rc := policyRefContext(clients, cache, "role", topicARN)
	refs, ok := grantedPrincipalRefs(policy, "role/")
	if !ok {
		return resource.UnknownRelated("role")
	}
	return relatedRefs("role", refs, rc)
}
