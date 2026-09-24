// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// alarm_related_extra.go contains CloudWatch alarm related-resource checkers
// that resolve alarm dimension values back to source resources.
package aws

import (
	"context"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

func checkAlarmAPIGW(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "apigw", res)
}

func checkAlarmCB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "cb", res)
}

func checkAlarmDBI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "dbi", res)
}

func checkAlarmEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "ec2", res)
}

func checkAlarmECS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "ecs", res)
}

func checkAlarmEKS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "eks", res)
}

func checkAlarmKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "kms", res)
}

func checkAlarmLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "lambda", res)
}

// checkAlarmLogs reports the log groups this alarm watches: the one it names
// in a LogGroupName dimension, and the ones whose metric filters emit the
// metric it is over, read with one DescribeMetricFilters on that metric.
func checkAlarmLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	var groups map[string]bool
	var partial bool
	if alarm, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct); ok {
		if m, named := AlarmMetricWatched(alarm); named {
			groups, partial = alarmMetricLogGroups(ctx, clients, m)
		}
	}
	result := alarmRowsNaming(ctx, clients, cache, "logs", res, func(row resource.Resource) bool {
		return groups[row.ID]
	})
	return alsoPartial(result, partial)
}

func checkAlarmS3(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "s3", res)
}

func checkAlarmSFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "sfn", res)
}

func checkAlarmWAF(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "waf", res)
}
