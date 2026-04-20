// logs_related.go contains CloudWatch Log Group related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cloudwatchlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkLogsLambda parses the log group name for the /aws/lambda/{name} pattern.
// If matched, it searches the lambda cache for a function with that name.
func checkLogsLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	logGroupName := res.ID
	if logGroupName == "" {
		logGroupName = res.Name
	}

	const prefix = "/aws/lambda/"
	if !strings.HasPrefix(logGroupName, prefix) {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	functionName := strings.TrimPrefix(logGroupName, prefix)
	if functionName == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	lambdaList, truncated, err := logsRelatedResources(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if lambdaList == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}

	var ids []string
	for _, lambdaRes := range lambdaList {
		if lambdaRes.ID == functionName || lambdaRes.Name == functionName {
			ids = append(ids, lambdaRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("lambda")
	}
	return relatedResult("lambda", ids)
}

// checkLogsAlarms searches the alarm cache for alarms with a "LogGroupName" dimension
// matching this log group's name (res.ID).
func checkLogsAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	logGroupName := res.ID
	if logGroupName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := logsRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "LogGroupName" && d.Value != nil && *d.Value == logGroupName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkLogsKMS extracts the KMS key ID from the CloudWatch Log Group's KmsKeyId
// field. The value may be a full ARN (arn:aws:kms:…/key-id) or a plain key ID.
// Pattern F — no cache needed.
func checkLogsKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	lg, ok := assertStruct[cloudwatchlogstypes.LogGroup](res.RawStruct)
	if !ok || lg.KmsKeyId == nil || *lg.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *lg.KmsKeyId
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// checkLogsAPIGW matches log groups whose name indicates API Gateway execution
// logs (API-Gateway-Execution-Logs_{rest-api-id}/{stage}) and resolves the
// referenced REST API from the apigw cache. Pattern N+C.
func checkLogsAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	logGroupName := res.ID
	if logGroupName == "" {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: 0}
	}
	const prefix = "API-Gateway-Execution-Logs_"
	if !strings.HasPrefix(logGroupName, prefix) {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: 0}
	}
	rest := strings.TrimPrefix(logGroupName, prefix)
	apiID, _, _ := strings.Cut(rest, "/")
	if apiID == "" {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: 0}
	}

	apiList, truncated, err := logsRelatedResources(ctx, clients, cache, "apigw")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1, Err: err}
	}
	if apiList == nil {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1}
	}
	var ids []string
	for _, api := range apiList {
		if api.ID == apiID {
			ids = append(ids, api.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("apigw")
	}
	return relatedResult("apigw", ids)
}

// checkLogsECSTask matches log groups named /ecs/{task-family}. The family is
// extracted and searched in the ecs-task cache. Pattern N+C.
func checkLogsECSTask(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	logGroupName := res.ID
	if logGroupName == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}
	const prefix = "/ecs/"
	if !strings.HasPrefix(logGroupName, prefix) {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}
	family := strings.TrimPrefix(logGroupName, prefix)
	if idx := strings.Index(family, "/"); idx >= 0 {
		family = family[:idx]
	}
	if family == "" {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: 0}
	}

	taskList, truncated, err := logsRelatedResources(ctx, clients, cache, "ecs-task")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1, Err: err}
	}
	if taskList == nil {
		return resource.RelatedCheckResult{TargetType: "ecs-task", Count: -1}
	}
	var ids []string
	for _, taskRes := range taskList {
		if strings.Contains(taskRes.ID, family) || strings.Contains(taskRes.Name, family) {
			ids = append(ids, taskRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ecs-task")
	}
	return relatedResult("ecs-task", ids)
}

// logsSubscriptionFilters fetches the log group's subscription filters via a
// single DescribeSubscriptionFilters call. Returns nil on any failure.
func logsSubscriptionFilters(ctx context.Context, clients any, logGroupName string) []cloudwatchlogstypes.SubscriptionFilter {
	if logGroupName == "" {
		return nil
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.CloudWatchLogs == nil {
		return nil
	}
	filterAPI, ok := c.CloudWatchLogs.(CWLogsDescribeSubscriptionFiltersAPI)
	if !ok {
		return nil
	}
	out, err := filterAPI.DescribeSubscriptionFilters(ctx, &cloudwatchlogs.DescribeSubscriptionFiltersInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err != nil || out == nil {
		return nil
	}
	return out.SubscriptionFilters
}

// checkLogsKinesis calls cloudwatchlogs:DescribeSubscriptionFilters and
// returns the Kinesis stream names whose ARNs appear as subscription-filter
// destinations on this log group. Pattern C — single API call.
func checkLogsKinesis(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	filters := logsSubscriptionFilters(ctx, clients, res.ID)
	if filters == nil {
		// Distinguish "no filters" from "API failed / cannot call".
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil || c.CloudWatchLogs == nil {
			return resource.RelatedCheckResult{TargetType: "kinesis", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "kinesis", Count: 0}
	}
	var ids []string
	for _, f := range filters {
		if f.DestinationArn == nil {
			continue
		}
		arn := *f.DestinationArn
		// Kinesis stream ARN: arn:aws:kinesis:REGION:ACCOUNT:stream/NAME
		if _, name, ok := strings.Cut(arn, ":stream/"); ok && name != "" {
			ids = append(ids, name)
		}
	}
	return relatedResult("kinesis", ids)
}

// checkLogsS3 calls cloudwatchlogs:DescribeSubscriptionFilters and returns S3
// bucket names whose ARNs appear as subscription-filter destinations (via a
// Firehose delivery stream that fans out to S3, or direct S3 destination for
// newer filter features). Pattern C — single API call.
func checkLogsS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	filters := logsSubscriptionFilters(ctx, clients, res.ID)
	if filters == nil {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil || c.CloudWatchLogs == nil {
			return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	var ids []string
	for _, f := range filters {
		if f.DestinationArn == nil {
			continue
		}
		arn := *f.DestinationArn
		// S3 bucket ARN: arn:aws:s3:::bucket-name
		if name, ok := strings.CutPrefix(arn, "arn:aws:s3:::"); ok {
			if before, _, hasSep := strings.Cut(name, "/"); hasSep {
				name = before
			}
			if name != "" {
				ids = append(ids, name)
			}
		}
	}
	return relatedResult("s3", ids)
}

// logsRelatedResources returns the resource list for target from cache or by fetching the first page.
func logsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
