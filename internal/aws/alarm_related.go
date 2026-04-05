package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkAlarmSNS checks AlarmActions, OKActions, and InsufficientDataActions for
// SNS topic ARNs. Pattern F — reads directly from RawStruct, no cache needed.
func checkAlarmSNS(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
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
