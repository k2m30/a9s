// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// sns_sub_related.go contains SNS subscription related-resource checker functions.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkSNSSubTopic checks the sns cache for the topic this subscription belongs to.
// Pattern C: matches res.Fields["topic_arn"] against sns cache IDs (topic ARNs).
func checkSNSSubTopic(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		return keyMissing("sns", "topicARN")
	}

	snsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sns")
	if err != nil {
		return ReadFailed("sns", err)
	}
	if snsList == nil {
		return NotRead("sns")
	}

	var ids []string
	for _, snsRes := range snsList {
		if snsRes.ID == topicARN {
			ids = append(ids, snsRes.ID)
		}
	}
	return relatedResultTrunc("sns", ids, truncated)
}

// checkSNSSubLambda checks the lambda cache for the function this subscription invokes.
// Pattern C: only relevant when protocol=lambda. The function the endpoint ARN
// names is matched against lambda cache IDs.
func checkSNSSubLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["protocol"] != "lambda" {
		return foundNone("lambda", "res.Fields[protocol]")
	}

	endpoint := res.Fields["endpoint"]
	if endpoint == "" {
		return foundNone("lambda", "endpoint")
	}

	lambdaList, truncated, err := relatedResourcesFor(ctx, clients, cache, "lambda")
	if err != nil {
		return ReadFailed("lambda", err)
	}
	if lambdaList == nil {
		return NotRead("lambda")
	}
	funcName, local := resource.ResolveRef("lambda", endpoint, refContext(clients, cache, "lambda"))
	if !local {
		return relatedResultTrunc("lambda", nil, true)
	}

	var ids []string
	for _, lambdaRes := range lambdaList {
		if lambdaRes.ID == funcName || lambdaRes.Name == funcName {
			ids = append(ids, lambdaRes.ID)
		}
	}
	return relatedResultTrunc("lambda", ids, truncated)
}

// checkSNSSubSQS checks the sqs cache for the queue this subscription delivers to.
// Pattern C: only relevant when protocol=sqs. The queue the endpoint ARN names
// is matched against sqs cache IDs.
func checkSNSSubSQS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["protocol"] != "sqs" {
		return foundNone("sqs", "res.Fields[protocol]")
	}

	endpoint := res.Fields["endpoint"]
	if endpoint == "" {
		return foundNone("sqs", "endpoint")
	}

	sqsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sqs")
	if err != nil {
		return ReadFailed("sqs", err)
	}
	if sqsList == nil {
		return NotRead("sqs")
	}
	queueName, local := resource.ResolveRef("sqs", endpoint, refContext(clients, cache, "sqs"))
	if !local {
		return relatedResultTrunc("sqs", nil, true)
	}

	var ids []string
	for _, sqsRes := range sqsList {
		if sqsRes.ID == queueName || sqsRes.Name == queueName {
			ids = append(ids, sqsRes.ID)
		}
	}
	return relatedResultTrunc("sqs", ids, truncated)
}
