// alarm_related_extra.go contains CloudWatch alarm related-resource checkers
// that resolve alarm dimension values back to source resources.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// alarmDimension returns the first dimension value for the given dimension name.
func alarmDimension(alarm cwtypes.MetricAlarm, name string) string {
	for _, d := range alarm.Dimensions {
		if d.Name != nil && *d.Name == name && d.Value != nil {
			return *d.Value
		}
	}
	return ""
}

func checkAlarmAPIGW(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "apigw", Count: -1}
	}
	if v := alarmDimension(alarm, "ApiName"); v != "" {
		return relatedResult("apigw", []string{v})
	}
	if v := alarmDimension(alarm, "ApiId"); v != "" {
		return relatedResult("apigw", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "apigw", Count: 0}
}

func checkAlarmCB(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cb", Count: -1}
	}
	if v := alarmDimension(alarm, "ProjectName"); v != "" {
		return relatedResult("cb", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "cb", Count: 0}
}

func checkAlarmDBI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1}
	}
	if v := alarmDimension(alarm, "DBInstanceIdentifier"); v != "" {
		return relatedResult("dbi", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "dbi", Count: 0}
}

func checkAlarmEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	if v := alarmDimension(alarm, "InstanceId"); v != "" {
		return relatedResult("ec2", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
}

func checkAlarmECS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ecs", Count: -1}
	}
	if v := alarmDimension(alarm, "ClusterName"); v != "" {
		return relatedResult("ecs", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "ecs", Count: 0}
}

func checkAlarmEKS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1}
	}
	if v := alarmDimension(alarm, "ClusterName"); v != "" {
		// AWS/EKS namespace — differentiate from AWS/ECS via namespace check.
		if alarm.Namespace != nil && (strings.Contains(*alarm.Namespace, "EKS") || strings.Contains(*alarm.Namespace, "ContainerInsights")) {
			return relatedResult("eks", []string{v})
		}
	}
	return resource.RelatedCheckResult{TargetType: "eks", Count: 0}
}

func checkAlarmKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if v := alarmDimension(alarm, "KeyId"); v != "" {
		return relatedResult("kms", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
}

func checkAlarmLambda(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	if v := alarmDimension(alarm, "FunctionName"); v != "" {
		return relatedResult("lambda", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
}

func checkAlarmLogs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	if v := alarmDimension(alarm, "LogGroupName"); v != "" {
		return relatedResult("logs", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
}

func checkAlarmS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	if v := alarmDimension(alarm, "BucketName"); v != "" {
		return relatedResult("s3", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
}

func checkAlarmSFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sfn", Count: -1}
	}
	if v := alarmDimension(alarm, "StateMachineArn"); v != "" {
		if idx := strings.LastIndex(v, ":"); idx >= 0 && idx < len(v)-1 {
			return relatedResult("sfn", []string{v[idx+1:]})
		}
		return relatedResult("sfn", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "sfn", Count: 0}
}

func checkAlarmWAF(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "waf", Count: -1}
	}
	if v := alarmDimension(alarm, "WebACL"); v != "" {
		return relatedResult("waf", []string{v})
	}
	return resource.RelatedCheckResult{TargetType: "waf", Count: 0}
}

// checkAlarmCTEvents scans the ct-events cache for events that reference this
// alarm (DescribeAlarms / PutMetricAlarm etc).
func checkAlarmCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	name := res.ID
	if name == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	evList, truncated, err := alarmRelatedResources(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, Err: err}
	}
	if evList == nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	var ids []string
	for _, evRes := range evList {
		if strings.Contains(evRes.Fields["event_source"], "monitoring.amazonaws.com") &&
			strings.Contains(evRes.Fields["event_name"], "Alarm") {
			ids = append(ids, evRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ct-events")
	}
	return relatedResult("ct-events", ids)
}
