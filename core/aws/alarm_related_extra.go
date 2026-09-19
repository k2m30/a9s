// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// alarm_related_extra.go contains CloudWatch alarm related-resource checkers
// that resolve alarm dimension values back to source resources.
package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
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

// alarmTarget is the result of an alarm pivot whose target the alarm names in
// the first of dims it carries, read through the target's resolver. When
// namespaces is non-empty the dimension names the target only in those metric
// namespaces: "ClusterName" names an ECS cluster in one and an EKS cluster in
// another.
func alarmTarget(clients any, cache resource.ResourceCache, res resource.Resource, target string, namespaces []string, dims ...string) resource.RelatedCheckResult {
	alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated(target)
	}
	if len(namespaces) > 0 && !slices.Contains(namespaces, aws.ToString(alarm.Namespace)) {
		return resource.ProvenZero(target, "the alarm namespace")
	}
	for _, dim := range dims {
		if v := alarmDimension(alarm, dim); v != "" {
			return relatedRefs(target, []string{v}, refContext(clients, cache, target))
		}
	}
	return resource.ProvenZero(target, "the alarm dimensions")
}

func checkAlarmAPIGW(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "apigw", nil, "ApiName", "ApiId")
}

func checkAlarmCB(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "cb", nil, "ProjectName")
}

func checkAlarmDBI(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "dbi", nil, "DBInstanceIdentifier")
}

func checkAlarmEC2(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "ec2", nil, "InstanceId")
}

func checkAlarmECS(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "ecs", []string{"AWS/ECS", "ECS/ContainerInsights"}, "ClusterName")
}

func checkAlarmEKS(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "eks", []string{"AWS/EKS", "ContainerInsights"}, "ClusterName")
}

func checkAlarmKMS(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "kms", nil, "KeyId")
}

func checkAlarmLambda(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "lambda", nil, "FunctionName")
}

func checkAlarmLogs(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "logs", nil, "LogGroupName")
}

func checkAlarmS3(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "s3", nil, "BucketName")
}

func checkAlarmSFN(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "sfn", nil, "StateMachineArn")
}

func checkAlarmWAF(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmTarget(clients, cache, res, "waf", nil, "WebACL")
}

// checkAlarmCTEvents scans the ct-events cache for events that reference this
// alarm (DescribeAlarms / PutMetricAlarm etc).
func checkAlarmCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	name := res.ID
	if name == "" {
		return resource.ProvenZero("ct-events", "name")
	}
	evList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ct-events")
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
