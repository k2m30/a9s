// sns_sub_related.go contains SNS subscription related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkSNSSubTopic checks the sns cache for the topic this subscription belongs to.
// Pattern C: matches res.Fields["topic_arn"] against sns cache IDs (topic ARNs).
func checkSNSSubTopic(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}

	snsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sns")
	if err != nil {
		return resource.ErrorRelated("sns", err)
	}
	if snsList == nil {
		return resource.UnknownRelated("sns")
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
// Pattern C: only relevant when protocol=lambda. Parses the function name from the
// endpoint ARN (last ":" segment) and matches against lambda cache IDs.
func checkSNSSubLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["protocol"] != "lambda" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	endpoint := res.Fields["endpoint"]
	if endpoint == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	// Parse function name from Lambda ARN: arn:aws:lambda:...:function:FunctionName
	funcName := endpoint
	if parts := strings.Split(endpoint, ":"); len(parts) > 0 {
		funcName = parts[len(parts)-1]
	}

	lambdaList, truncated, err := relatedResourcesFor(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.ErrorRelated("lambda", err)
	}
	if lambdaList == nil {
		return resource.UnknownRelated("lambda")
	}

	var ids []string
	for _, lambdaRes := range lambdaList {
		if lambdaRes.ID == funcName || lambdaRes.Name == funcName || strings.Contains(lambdaRes.ID, funcName) {
			ids = append(ids, lambdaRes.ID)
		}
	}
	return relatedResultTrunc("lambda", ids, truncated)
}

// checkSNSSubSQS checks the sqs cache for the queue this subscription delivers to.
// Pattern C: only relevant when protocol=sqs. Parses the queue name from the
// endpoint ARN (last ":" segment) and matches against sqs cache IDs.
func checkSNSSubSQS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.Fields["protocol"] != "sqs" {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: 0}
	}

	endpoint := res.Fields["endpoint"]
	if endpoint == "" {
		return resource.RelatedCheckResult{TargetType: "sqs", Count: 0}
	}

	// Parse queue name from SQS ARN: arn:aws:sqs:...:account:queue-name
	queueName := endpoint
	if parts := strings.Split(endpoint, ":"); len(parts) > 0 {
		queueName = parts[len(parts)-1]
	}

	sqsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "sqs")
	if err != nil {
		return resource.ErrorRelated("sqs", err)
	}
	if sqsList == nil {
		return resource.UnknownRelated("sqs")
	}

	var ids []string
	for _, sqsRes := range sqsList {
		if sqsRes.ID == queueName || sqsRes.Name == queueName {
			ids = append(ids, sqsRes.ID)
		}
	}
	return relatedResultTrunc("sqs", ids, truncated)
}
