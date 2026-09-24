// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// logs_related.go contains CloudWatch Log Group related-resource checker functions.
package aws

import (
	"cmp"
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkLogsLambda counts the functions that log to this group — each
// function's LoggingConfig.LogGroup, "/aws/lambda/<function name>" by default
// (https://docs.aws.amazon.com/lambda/latest/api/API_LoggingConfig.html), which
// the lambda row carries as log_group — and the functions its subscription
// filters deliver to.
func checkLogsLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	group := cmp.Or(res.ID, res.Name)
	if group == "" {
		return keyMissing("lambda", "group")
	}
	logging := relatedRead{unread: true}
	fns, truncated, err := relatedResourcesFor(ctx, clients, cache, "lambda")
	if err != nil {
		logging.failure = err
	} else if fns != nil {
		logging = relatedRead{partial: truncated}
		for _, fn := range fns {
			if cmp.Or(fn.Fields["log_group"], "/aws/lambda/"+fn.ID) == group {
				logging.ids = append(logging.ids, fn.ID)
			}
		}
	}
	return relatedAnswer("lambda", joinReads(logging, subscribedRead(ctx, clients, cache, group, "lambda", "lambda")))
}

// subscribedRead reads the target rows this group's subscription filters
// deliver to: the destinations that are ARNs of service.
func subscribedRead(ctx context.Context, clients any, cache resource.ResourceCache, group, target, service string) relatedRead {
	filters, complete, err := logsSubscriptionFilters(ctx, clients, group)
	read := pagedRead(complete, err)
	// A session without a CloudWatch Logs client leaves the subscriptions
	// unread, not the relation: the other places it lives still answer.
	read.noClient = false
	ids, dropped := resolveRefs(target, destinationARNs(filters, service), refContext(clients, cache, target))
	read.ids, read.partial = ids, read.partial || dropped
	return read
}

// checkLogsAlarms reports the alarms watching this log group: the ones
// carrying it as a dimension, and the ones over a metric its metric filters
// emit. Nothing in an alarm names a log group when a filter is what bridges
// them, so the filters are read once and the loaded alarms matched on the
// metric they publish.
func checkLogsAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	emitted, filters := logGroupFilterMetrics(ctx, clients, res.ID)
	result := alarmIDsByDimension(ctx, clients, cache, "logs", res, func(a cwtypes.MetricAlarm) bool {
		m, ok := AlarmMetricWatched(a)
		return ok && emitted[m]
	})
	return alsoRead(result, filters)
}

// logGroupFilterMetrics returns the metrics the group's metric filters emit,
// and what reading the filters established.
func logGroupFilterMetrics(ctx context.Context, clients any, logGroupName string) (map[AlarmMetric]bool, relatedRead) {
	api := metricFiltersAPI(clients)
	if api == nil || logGroupName == "" {
		return nil, relatedRead{}
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
	emitted := map[AlarmMetric]bool{}
	for _, f := range filters {
		for _, t := range f.MetricTransformations {
			m := AlarmMetric{Namespace: aws.ToString(t.MetricNamespace), Name: aws.ToString(t.MetricName)}
			if m.Namespace != "" && m.Name != "" {
				emitted[m] = true
			}
		}
	}
	return emitted, pagedRead(complete, err)
}

// alarmMetricLogGroups returns the log groups whose metric filters emit the
// metric, which is how a metric-filter alarm names the group it watches, and
// what reading the filters established.
func alarmMetricLogGroups(ctx context.Context, clients any, m AlarmMetric) (map[string]bool, relatedRead) {
	api := metricFiltersAPI(clients)
	if api == nil {
		return nil, relatedRead{}
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
	groups := map[string]bool{}
	for _, f := range filters {
		if name := aws.ToString(f.LogGroupName); name != "" {
			groups[name] = true
		}
	}
	return groups, pagedRead(complete, err)
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
		return keyMissing("apigw", "logGroupName")
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

// checkLogsECSTask reports the tasks writing to this log group: the tasks
// whose task definition names it in a container's awslogs-group option, the
// field ecs-task → logs reads, as the ecs-task list's own join read it. A
// group named /ecs/<family> belongs to whichever definition names it there.
func checkLogsECSTask(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	group := res.ID
	if group == "" {
		return keyMissing("ecs-task", "logGroupName")
	}
	taskList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ecs-task")
	if err != nil {
		return ReadFailed("ecs-task", err)
	}
	if taskList == nil {
		return NotRead("ecs-task")
	}
	var ids []string
	var reads rowReads
	// A row the list joined answers from its fields; only the rest cost a
	// call each.
	joined, unjoined := splitECSTaskJoin(taskList, "log_groups")
	unjoined, capped := fanOut(unjoined)
	truncated = truncated || capped
	for _, taskRes := range slices.Concat(joined, unjoined) {
		groups, read, err := ecsTaskLogGroups(ctx, clients, taskRes)
		switch {
		case err != nil:
			reads.fail(taskRes.ID, err)
			continue
		case slices.Contains(groups, group):
			reads.read++
			ids = append(ids, taskRes.ID)
		case !read:
			reads.missed()
		default:
			reads.read++
		}
	}
	return reads.answer("ecs-task", "logs-related: DescribeTaskDefinition", ids, truncated)
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
	return relatedAnswer("kinesis", subscribedRead(ctx, clients, cache, res.ID, "kinesis", "kinesis"))
}

// checkLogsS3 counts the buckets this group's export tasks wrote to: each
// ExportTask names its logGroupName and "the name of the S3 bucket to which
// the log data was exported"
// (https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_ExportTask.html).
// DescribeExportTasks filters by task id or status only, so every task is read.
// A subscription filter delivers to Kinesis, Firehose, Lambda or OpenSearch,
// never to a bucket.
func checkLogsS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CloudWatchLogs == nil || res.ID == "" {
		return NotRead("s3")
	}
	api, ok := c.CloudWatchLogs.(CWLogsDescribeExportTasksAPI)
	if !ok {
		return NotRead("s3")
	}
	tasks, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]cloudwatchlogstypes.ExportTask, *string, error) {
		out, err := api.DescribeExportTasks(ctx, &cloudwatchlogs.DescribeExportTasksInput{NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.ExportTasks, out.NextToken, nil
	})
	var buckets []string
	for _, t := range tasks {
		if aws.ToString(t.LogGroupName) == res.ID && exportTaskWrites(t) {
			buckets = append(buckets, aws.ToString(t.Destination))
		}
	}
	ids, dropped := resolveRefs("s3", buckets, refContext(clients, cache, "s3"))
	return alsoRead(relatedResultTrunc("s3", ids, dropped), pagedRead(complete, err))
}

// exportTaskWrites is the one predicate over ExportTaskStatus.code: a
// COMPLETED task wrote to its bucket and a PENDING or RUNNING one is writing;
// a FAILED, CANCELLED or PENDING_CANCEL one wrote nothing to count
// (https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_ExportTaskStatus.html).
func exportTaskWrites(t cloudwatchlogstypes.ExportTask) bool {
	if t.Status == nil {
		return true
	}
	switch t.Status.Code {
	case cloudwatchlogstypes.ExportTaskStatusCodeFailed, cloudwatchlogstypes.ExportTaskStatusCodeCancelled, cloudwatchlogstypes.ExportTaskStatusCodePendingCancel:
		return false
	}
	return true
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
