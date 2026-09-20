// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkAlarmSNS reports the SNS topics this alarm is about: the one whose
// metrics it watches and the ones its actions notify.
func checkAlarmSNS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "sns", res)
}

// checkAlarmASG reports the Auto Scaling groups this alarm is about: the one
// whose metrics it watches and the one whose scaling policy it runs.
func checkAlarmASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmRowsNaming(ctx, clients, cache, "asg", res)
}
