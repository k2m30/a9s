// sqs_related.go contains SQS queue related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("sqs", []resource.RelatedDef{
		{TargetType: "sns-sub", DisplayName: "SNS Subscriptions", Checker: checkSQSSNSSub, NeedsTargetCache: true},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSQSAlarm, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: nil, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: nil, NeedsTargetCache: true},
	})
}

// checkSQSSNSSub searches the sns-sub cache for subscriptions where protocol=sqs
// and the endpoint ARN contains this queue's ARN.
// Pattern C — reverse lookup in sns-sub cache.
func checkSQSSNSSub(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// Attempt to retrieve the queue ARN from the raw struct attributes first.
	queueARN := ""
	if raw, ok := assertStruct[SQSQueueAttributesRow](res.RawStruct); ok {
		queueARN = raw.Attributes["QueueArn"]
	}
	// Fall back to constructing a partial match from the queue name.
	queueName := res.ID
	if queueARN == "" && queueName == "" {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}

	subList, truncated, err := sqsRelatedResources(ctx, clients, cache, "sns-sub")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1, Err: err}
	}
	if subList == nil {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}

	var ids []string
	for _, subRes := range subList {
		if subRes.Fields["protocol"] != "sqs" {
			continue
		}
		endpoint := subRes.Fields["endpoint"]
		if endpoint == "" {
			continue
		}
		// Match by full ARN or queue name as a suffix.
		if (queueARN != "" && strings.Contains(endpoint, queueARN)) ||
			(queueName != "" && strings.HasSuffix(endpoint, ":"+queueName)) {
			ids = append(ids, subRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}
	return relatedResult("sns-sub", ids)
}

// checkSQSAlarm searches the alarm cache for CloudWatch alarms in the AWS/SQS
// namespace with a QueueName dimension matching this queue's name.
// Pattern C — reverse lookup in alarm cache.
func checkSQSAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	queueName := res.ID
	if queueName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := sqsRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		if alarm.Namespace == nil || *alarm.Namespace != "AWS/SQS" {
			continue
		}
		for _, dim := range alarm.Dimensions {
			if dim.Name != nil && *dim.Name == "QueueName" &&
				dim.Value != nil && *dim.Value == queueName {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
}

// sqsRelatedResources returns the cached resource list for the given target type,
// or fetches the first page via the registered paginated fetcher.
func sqsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
