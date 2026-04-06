// glue_related.go contains Glue Job related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkGlueRole extracts the Role from the Glue Job RawStruct. The value may be
// a full ARN (arn:aws:iam::…:role/name) or a plain role name. The role name is
// extracted from the last path segment of an ARN, or used directly if no "/" is present.
// The role cache is then searched by name.
func checkGlueRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	job, ok := assertStruct[gluetypes.Job](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if job.Role == nil || *job.Role == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleVal := *job.Role
	roleName := roleVal
	if idx := strings.LastIndex(roleVal, "/"); idx >= 0 && idx < len(roleVal)-1 {
		roleName = roleVal[idx+1:]
	}

	roleList, _, err := glueRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.Fields["role_name"] == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	return relatedResult("role", ids)
}

// checkGlueAlarms searches the alarm cache for alarms with a "JobName" dimension
// matching this Glue job's name.
func checkGlueAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	jobName := res.Name
	if jobName == "" {
		jobName = res.ID
	}
	if jobName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := glueRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "JobName" && d.Value != nil && *d.Value == jobName {
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

// glueRelatedResources returns the resource list for target from cache or by fetching the first page.
func glueRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
