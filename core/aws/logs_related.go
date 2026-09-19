// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// logs_related.go contains CloudWatch Log Group related-resource checker functions.
package aws

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkLogsLambda parses the log group name for the /aws/lambda/{name} pattern.
// If matched, it searches the lambda cache for a function with that name.
func checkLogsLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	logGroupName := res.ID
	if logGroupName == "" {
		logGroupName = res.Name
	}

	functionName := logGroupOwner(logGroupName, "/aws/lambda/")
	if functionName == "" {
		return resource.ProvenZero("lambda", "functionName")
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
		if lambdaRes.ID == functionName || lambdaRes.Name == functionName {
			ids = append(ids, lambdaRes.ID)
		}
	}
	return relatedResultTrunc("lambda", ids, truncated)
}

// checkLogsAlarms searches the alarm cache for alarms with a "LogGroupName" dimension
// matching this log group's name (res.ID).
func checkLogsAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "", "LogGroupName", res.ID)
}

// checkLogsKMS extracts the KMS key ID from the CloudWatch Log Group's KmsKeyId
// field. The value may be a full ARN (arn:aws:kms:…/key-id) or a plain key ID.
func checkLogsKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	lg, ok := assertStruct[cloudwatchlogstypes.LogGroup](res.RawStruct)
	if !ok || lg.KmsKeyId == nil || *lg.KmsKeyId == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("kms")
		}
		return resource.ProvenZero("kms", "lg.KmsKeyId")
	}
	keyID := kmsRefFromField(*lg.KmsKeyId, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkLogsAPIGW matches log groups whose name indicates API Gateway execution
// logs (API-Gateway-Execution-Logs_{rest-api-id}/{stage}) and resolves the
// referenced REST API from the apigw cache.
func checkLogsAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	logGroupName := res.ID
	if logGroupName == "" {
		return resource.ProvenZero("apigw", "logGroupName")
	}
	apiID := logGroupOwner(logGroupName, "API-Gateway-Execution-Logs_")
	if apiID == "" {
		return resource.ProvenZero("apigw", "apiID")
	}

	apiList, truncated, err := relatedResourcesFor(ctx, clients, cache, "apigw")
	if err != nil {
		return resource.ErrorRelated("apigw", err)
	}
	if apiList == nil {
		return resource.UnknownRelated("apigw")
	}
	var ids []string
	for _, api := range apiList {
		if api.ID == apiID {
			ids = append(ids, api.ID)
		}
	}
	return relatedResultTrunc("apigw", ids, truncated)
}

// checkLogsECSTask matches log groups named /ecs/{task-family}. The family is
// extracted and searched in the ecs-task cache.
func checkLogsECSTask(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	logGroupName := res.ID
	if logGroupName == "" {
		return resource.ProvenZero("ecs-task", "logGroupName")
	}
	family := logGroupOwner(logGroupName, "/ecs/")
	if family == "" {
		return resource.ProvenZero("ecs-task", "family")
	}

	taskList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-task")
	if err != nil {
		return resource.ErrorRelated("ecs-task", err)
	}
	if taskList == nil {
		return resource.UnknownRelated("ecs-task")
	}
	var ids []string
	for _, taskRes := range taskList {
		// Fields["task_definition"] holds the full task-definition ARN; match
		// its family rather than substring-matching the family against the
		// task's own UUID ID/Name.
		if taskDefFamily(taskRes.Fields["task_definition"]) == family {
			ids = append(ids, taskRes.ID)
		}
	}
	return relatedResultTrunc("ecs-task", ids, truncated)
}

// logsSubscriptionFilters fetches the log group's subscription filters via a
// single DescribeSubscriptionFilters call. Returns (nil, errClientMissing)
// when the client/interface isn't wired, (nil, err) when the call itself
// failed, and (filters, nil) — possibly an empty slice — on success, so
// callers can tell "genuinely no filters" apart from "could not check".
func logsSubscriptionFilters(ctx context.Context, clients any, logGroupName string) (filters []cloudwatchlogstypes.SubscriptionFilter, complete bool, err error) {
	if logGroupName == "" {
		return nil, false, errClientMissing
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CloudWatchLogs == nil {
		return nil, false, errClientMissing
	}
	filterAPI, ok := c.CloudWatchLogs.(CWLogsDescribeSubscriptionFiltersAPI)
	if !ok {
		return nil, false, errClientMissing
	}
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]cloudwatchlogstypes.SubscriptionFilter, *string, error) {
		out, err := filterAPI.DescribeSubscriptionFilters(ctx, &cloudwatchlogs.DescribeSubscriptionFiltersInput{
			LogGroupName: aws.String(logGroupName),
			NextToken:    token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.SubscriptionFilters, out.NextToken, nil
	})
}

// checkLogsKinesis calls cloudwatchlogs:DescribeSubscriptionFilters and
// returns the Kinesis stream names whose ARNs appear as subscription-filter
// destinations on this log group.
func checkLogsKinesis(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	filters, complete, err := logsSubscriptionFilters(ctx, clients, res.ID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.UnknownRelated("kinesis")
		}
		return resource.ErrorRelated("kinesis", err)
	}
	ids, dropped := resolveRefs("kinesis", destinationARNs(filters, "kinesis"), refContext(clients, cache, "kinesis"))
	return relatedResultTrunc("kinesis", ids, dropped || !complete)
}

// checkLogsS3 calls cloudwatchlogs:DescribeSubscriptionFilters and returns S3
// bucket names whose ARNs appear as subscription-filter destinations (via a
// Firehose delivery stream that fans out to S3, or direct S3 destination for
// newer filter features).
func checkLogsS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	filters, complete, err := logsSubscriptionFilters(ctx, clients, res.ID)
	if err != nil {
		if errors.Is(err, errClientMissing) {
			return resource.UnknownRelated("s3")
		}
		return resource.ErrorRelated("s3", err)
	}
	ids, dropped := resolveRefs("s3", destinationARNs(filters, "s3"), refContext(clients, cache, "s3"))
	return relatedResultTrunc("s3", ids, dropped || !complete)
}

// destinationARNs returns the subscription-filter destinations that are ARNs
// of service.
func destinationARNs(filters []cloudwatchlogstypes.SubscriptionFilter, service string) []string {
	var arns []string
	for _, f := range filters {
		if f.DestinationArn == nil {
			continue
		}
		if _, ok := ARNForService(*f.DestinationArn, service); ok {
			arns = append(arns, *f.DestinationArn)
		}
	}
	return arns
}
