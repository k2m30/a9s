// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// alarm_related_extra.go contains CloudWatch alarm related-resource checkers
// that resolve alarm dimension values back to source resources.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/core/resource"
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
		return resource.UnknownRelated("apigw")
	}
	if v := alarmDimension(alarm, "ApiName"); v != "" {
		return relatedResult("apigw", []string{v})
	}
	if v := alarmDimension(alarm, "ApiId"); v != "" {
		return relatedResult("apigw", []string{v})
	}
	return resource.KnownRelated("apigw", nil, false)
}

func checkAlarmCB(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("cb")
	}
	if v := alarmDimension(alarm, "ProjectName"); v != "" {
		return relatedResult("cb", []string{v})
	}
	return resource.KnownRelated("cb", nil, false)
}

func checkAlarmDBI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("dbi")
	}
	if v := alarmDimension(alarm, "DBInstanceIdentifier"); v != "" {
		return relatedResult("dbi", []string{v})
	}
	return resource.KnownRelated("dbi", nil, false)
}

func checkAlarmEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ec2")
	}
	if v := alarmDimension(alarm, "InstanceId"); v != "" {
		return relatedResult("ec2", []string{v})
	}
	return resource.KnownRelated("ec2", nil, false)
}

func checkAlarmECS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("ecs")
	}
	if v := alarmDimension(alarm, "ClusterName"); v != "" {
		return relatedResult("ecs", []string{v})
	}
	return resource.KnownRelated("ecs", nil, false)
}

func checkAlarmEKS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("eks")
	}
	if v := alarmDimension(alarm, "ClusterName"); v != "" {
		// AWS/EKS namespace — differentiate from AWS/ECS via namespace check.
		if alarm.Namespace != nil && (strings.Contains(*alarm.Namespace, "EKS") || strings.Contains(*alarm.Namespace, "ContainerInsights")) {
			return relatedResult("eks", []string{v})
		}
	}
	return resource.KnownRelated("eks", nil, false)
}

func checkAlarmKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	if v := alarmDimension(alarm, "KeyId"); v != "" {
		return relatedResult("kms", []string{v})
	}
	return resource.KnownRelated("kms", nil, false)
}

func checkAlarmLambda(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("lambda")
	}
	if v := alarmDimension(alarm, "FunctionName"); v != "" {
		return relatedResult("lambda", []string{v})
	}
	return resource.KnownRelated("lambda", nil, false)
}

func checkAlarmLogs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("logs")
	}
	if v := alarmDimension(alarm, "LogGroupName"); v != "" {
		return relatedResult("logs", []string{v})
	}
	return resource.KnownRelated("logs", nil, false)
}

func checkAlarmS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("s3")
	}
	if v := alarmDimension(alarm, "BucketName"); v != "" {
		return relatedResult("s3", []string{v})
	}
	return resource.KnownRelated("s3", nil, false)
}

func checkAlarmSFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sfn")
	}
	if v := alarmDimension(alarm, "StateMachineArn"); v != "" {
		if idx := strings.LastIndex(v, ":"); idx >= 0 && idx < len(v)-1 {
			return relatedResult("sfn", []string{v[idx+1:]})
		}
		return relatedResult("sfn", []string{v})
	}
	return resource.KnownRelated("sfn", nil, false)
}

func checkAlarmWAF(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("waf")
	}
	if v := alarmDimension(alarm, "WebACL"); v != "" {
		return relatedResult("waf", []string{v})
	}
	return resource.KnownRelated("waf", nil, false)
}

// checkAlarmCTEvents scans the ct-events cache for events that reference this
// alarm (DescribeAlarms / PutMetricAlarm etc).
func checkAlarmCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	name := res.ID
	if name == "" {
		return resource.KnownRelated("ct-events", nil, false)
	}
	evList, truncated, err := alarmRelatedResources(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.ErrorRelated("ct-events", err)
	}
	if evList == nil {
		return resource.UnknownRelated("ct-events")
	}
	var ids []string
	for _, evRes := range evList {
		if strings.Contains(evRes.Fields["source"], "monitoring.amazonaws.com") &&
			strings.Contains(evRes.Fields["event_name"], "Alarm") {
			ids = append(ids, evRes.ID)
		}
	}
	return relatedResultTrunc("ct-events", ids, truncated)
}
