// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// logs_related.go contains CloudWatch Log Group related-resource checker functions.
package aws

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
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
		return foundNone("lambda", "functionName")
	}

	lambdaList, truncated, err := relatedResourcesFor(ctx, clients, cache, "lambda")
	if err != nil {
		return ReadFailed("lambda", err)
	}
	if lambdaList == nil {
		return NotRead("lambda")
	}

	var ids []string
	for _, lambdaRes := range lambdaList {
		if lambdaRes.ID == functionName || lambdaRes.Name == functionName {
			ids = append(ids, lambdaRes.ID)
		}
	}
	return relatedResultTrunc("lambda", ids, truncated)
}

// checkLogsAlarms reports the alarms watching this log group: the ones
// carrying it as a dimension, and the ones over a metric its metric filters
// emit. Nothing in an alarm names a log group when a filter is what bridges
// them, so the filters are read once and the loaded alarms matched on the
// metric they publish.
func checkLogsAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	emitted, partial := logGroupFilterMetrics(ctx, clients, res.ID)
	result := alarmIDsByDimension(ctx, clients, cache, "logs", res, func(a cwtypes.MetricAlarm) bool {
		m, ok := AlarmMetricWatched(a)
		return ok && emitted[m]
	})
	return alsoPartial(result, partial)
}

// logGroupFilterMetrics returns the metrics the group's metric filters emit.
// partial is true when the filters could not be read in full, which leaves
// whatever matched a lower bound rather than a proven count.
func logGroupFilterMetrics(ctx context.Context, clients any, logGroupName string) (emitted map[AlarmMetric]bool, partial bool) {
	api := metricFiltersAPI(clients)
	if api == nil || logGroupName == "" {
		return nil, false
	}
	filters, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]cloudwatchlogstypes.MetricFilter, *string, error) {
		out, err := api.DescribeMetricFilters(ctx, &cloudwatchlogs.DescribeMetricFiltersInput{
			LogGroupName: aws.String(logGroupName),
			NextToken:    token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.MetricFilters, out.NextToken, nil
	})
	if err != nil {
		return nil, true
	}
	emitted = map[AlarmMetric]bool{}
	for _, f := range filters {
		for _, t := range f.MetricTransformations {
			m := AlarmMetric{Namespace: aws.ToString(t.MetricNamespace), Name: aws.ToString(t.MetricName)}
			if m.Namespace != "" && m.Name != "" {
				emitted[m] = true
			}
		}
	}
	return emitted, !complete
}

// alarmMetricLogGroups returns the log groups whose metric filters emit the
// metric, which is how a metric-filter alarm names the group it watches.
func alarmMetricLogGroups(ctx context.Context, clients any, m AlarmMetric) (groups map[string]bool, partial bool) {
	api := metricFiltersAPI(clients)
	if api == nil {
		return nil, false
	}
	filters, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]cloudwatchlogstypes.MetricFilter, *string, error) {
		out, err := api.DescribeMetricFilters(ctx, &cloudwatchlogs.DescribeMetricFiltersInput{
			MetricName:      aws.String(m.Name),
			MetricNamespace: aws.String(m.Namespace),
			NextToken:       token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.MetricFilters, out.NextToken, nil
	})
	if err != nil {
		return nil, true
	}
	groups = map[string]bool{}
	for _, f := range filters {
		if name := aws.ToString(f.LogGroupName); name != "" {
			groups[name] = true
		}
	}
	return groups, !complete
}

// metricFiltersAPI is the session's CloudWatch Logs client, or nil for a
// session that has none.
func metricFiltersAPI(clients any) CWLogsDescribeMetricFiltersAPI {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CloudWatchLogs == nil {
		return nil
	}
	return c.CloudWatchLogs
}

// checkLogsKMS extracts the KMS key ID from the CloudWatch Log Group's KmsKeyId
// field. The value may be a full ARN (arn:aws:kms:…/key-id) or a plain key ID.
func checkLogsKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// The key is on the row's fields too, which a disk-restored row keeps.
	keyID, read := res.Fields["kms_key_id"]
	if lg, ok := assertStruct[cloudwatchlogstypes.LogGroup](res.RawStruct); ok {
		keyID, read = aws.ToString(lg.KmsKeyId), true
	}
	if !read {
		return NotRead("kms")
	}
	if keyID == "" {
		return foundNone("kms", "lg.KmsKeyId")
	}
	return kmsRelated(ctx, clients, cache, []string{kmsRefFromField(keyID, res.Type)})
}

// checkLogsAPIGW matches log groups whose name indicates API Gateway execution
// logs (API-Gateway-Execution-Logs_{rest-api-id}/{stage}) and resolves the
// referenced REST API from the apigw cache.
func checkLogsAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	logGroupName := res.ID
	if logGroupName == "" {
		return foundNone("apigw", "logGroupName")
	}
	apiID := logGroupOwner(logGroupName, "API-Gateway-Execution-Logs_")
	if apiID == "" {
		return foundNone("apigw", "apiID")
	}

	apiList, truncated, err := relatedResourcesFor(ctx, clients, cache, "apigw")
	if err != nil {
		return ReadFailed("apigw", err)
	}
	if apiList == nil {
		return NotRead("apigw")
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
		return foundNone("ecs-task", "logGroupName")
	}
	family := logGroupOwner(logGroupName, "/ecs/")
	if family == "" {
		return foundNone("ecs-task", "family")
	}

	taskList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-task")
	if err != nil {
		return ReadFailed("ecs-task", err)
	}
	if taskList == nil {
		return NotRead("ecs-task")
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
			return NotRead("kinesis")
		}
		return ReadFailed("kinesis", err)
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
			return NotRead("s3")
		}
		return ReadFailed("s3", err)
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
