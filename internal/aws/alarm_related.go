package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkAlarmSNS checks AlarmActions, OKActions, and InsufficientDataActions for
// SNS topic ARNs. Pattern F — reads directly from RawStruct, no cache needed.
func checkAlarmSNS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sns", Count: -1}
	}

	arnSet := map[string]bool{}
	for _, arn := range raw.AlarmActions {
		if strings.HasPrefix(arn, "arn:aws:sns:") {
			arnSet[arn] = true
		}
	}
	for _, arn := range raw.OKActions {
		if strings.HasPrefix(arn, "arn:aws:sns:") {
			arnSet[arn] = true
		}
	}
	for _, arn := range raw.InsufficientDataActions {
		if strings.HasPrefix(arn, "arn:aws:sns:") {
			arnSet[arn] = true
		}
	}

	if len(arnSet) == 0 {
		return resource.RelatedCheckResult{TargetType: "sns", Count: 0}
	}
	ids := make([]string, 0, len(arnSet))
	for arn := range arnSet {
		ids = append(ids, arn)
	}
	return relatedResult("sns", ids)
}

// checkAlarmASG checks whether this alarm targets an Auto Scaling Group via its
// "AutoScalingGroupName" dimension. Pattern D reverse — alarm carries the ASG name
// in its dimensions; we look it up in the ASG cache by ID or Name.
func checkAlarmASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}

	var asgName string
	for _, d := range raw.Dimensions {
		if d.Name != nil && *d.Name == "AutoScalingGroupName" && d.Value != nil {
			asgName = *d.Value
			break
		}
	}
	if asgName == "" {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}

	asgList, truncated, err := alarmRelatedResources(ctx, clients, cache, "asg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if asgList == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}

	var ids []string
	for _, asgRes := range asgList {
		if asgRes.ID == asgName || asgRes.Name == asgName {
			ids = append(ids, asgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	return relatedResult("asg", ids)
}

// alarmRelatedResources returns the resource list for target from cache or by fetching the first page.
func alarmRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
