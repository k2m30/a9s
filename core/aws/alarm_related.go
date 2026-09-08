// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkAlarmSNS checks AlarmActions, OKActions, and InsufficientDataActions for
// SNS topic ARNs. Pattern F — reads directly from RawStruct, no cache needed.
func checkAlarmSNS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[cwtypes.MetricAlarm](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sns")
	}

	arnSet := map[string]bool{}
	for _, actions := range [][]string{raw.AlarmActions, raw.OKActions, raw.InsufficientDataActions} {
		for _, action := range actions {
			if _, ok := ARNForService(action, "sns"); ok {
				arnSet[action] = true
			}
		}
	}

	if len(arnSet) == 0 {
		return resource.KnownRelated("sns", nil, false)
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
		if res.RawStruct == nil {
			return resource.UnknownRelated("asg")
		}
		return resource.KnownRelated("asg", nil, false)
	}

	var asgName string
	for _, d := range raw.Dimensions {
		if d.Name != nil && *d.Name == "AutoScalingGroupName" && d.Value != nil {
			asgName = *d.Value
			break
		}
	}
	if asgName == "" {
		return resource.KnownRelated("asg", nil, false)
	}

	asgList, truncated, err := relatedResourcesFor(ctx, clients, cache, "asg")
	if err != nil {
		return resource.ErrorRelated("asg", err)
	}
	if asgList == nil {
		return resource.UnknownRelated("asg")
	}

	var ids []string
	for _, asgRes := range asgList {
		if asgRes.ID == asgName || asgRes.Name == asgName {
			ids = append(ids, asgRes.ID)
		}
	}
	return relatedResultTrunc("asg", ids, truncated)
}
