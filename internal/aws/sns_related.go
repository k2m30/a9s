// sns_related.go contains SNS topic related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("sns", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkSNSAlarm, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkSNSCFN, NeedsTargetCache: true},
		{TargetType: "sns-sub", DisplayName: "Subscriptions", Checker: checkSNSSub, NeedsTargetCache: true},
	})

	// snstypes topic: detail view renders only TopicArn — no cross-ref fields (KmsMasterKeyId,
	// subscriptions, delivery policies are GetTopicAttributes results, not in the list RawStruct).
}

// checkSNSCFN returns Count: 0 because SNS topic tags are not included in the
// ListTopics response — the CFN relationship cannot be determined from cache alone.
func checkSNSCFN(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
}

// checkSNSAlarm searches the alarm cache for alarms whose AlarmActions, OKActions,
// or InsufficientDataActions reference this SNS topic ARN.
// Pattern C — reverse lookup in alarm cache.
func checkSNSAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	alarmList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "alarm")
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
		}
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
		if snsAlarmReferences(alarm, topicARN) {
			ids = append(ids, alarmRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
}

// checkSNSSub searches the sns-sub cache for subscriptions whose topic_arn
// matches this SNS topic's ARN (Pattern C — reverse lookup in sns-sub cache).
func checkSNSSub(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	topicARN := res.Fields["topic_arn"]
	if topicARN == "" {
		topicARN = res.ID
	}
	if topicARN == "" {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}

	subList, truncated, err := FetchRelatedTarget(ctx, clients, cache, "sns-sub")
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
		}
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1, Err: err}
	}
	if subList == nil {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}

	var ids []string
	for _, subRes := range subList {
		if subRes.Fields["topic_arn"] == topicARN {
			ids = append(ids, subRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "sns-sub", Count: -1}
	}
	return relatedResult("sns-sub", ids)
}

// snsAlarmReferences reports whether any of the alarm's action lists contain
// an ARN that matches or contains the given SNS topic ARN.
func snsAlarmReferences(alarm cwtypes.MetricAlarm, topicARN string) bool {
	for _, arn := range alarm.AlarmActions {
		if strings.Contains(arn, topicARN) || arn == topicARN {
			return true
		}
	}
	for _, arn := range alarm.OKActions {
		if strings.Contains(arn, topicARN) || arn == topicARN {
			return true
		}
	}
	for _, arn := range alarm.InsufficientDataActions {
		if strings.Contains(arn, topicARN) || arn == topicARN {
			return true
		}
	}
	return false
}
