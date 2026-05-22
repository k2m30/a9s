package aws

import (
	"context"
	"fmt"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchTargetHealth calls the ELBv2 DescribeTargetHealth API for a given
// target group ARN and converts the response into a FetchResult.
// No pagination — a single API call returns all targets.
func FetchTargetHealth(ctx context.Context, api ELBv2DescribeTargetHealthAPI, targetGroupArn string, continuationToken string) (resource.FetchResult, error) {
	output, err := api.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{
		TargetGroupArn: &targetGroupArn,
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching target health: %w", err)
	}

	var resources []resource.Resource

	for _, thd := range output.TargetHealthDescriptions {
		resources = append(resources, convertTargetHealth(thd))
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			TotalHint:   len(resources),
			PageSize:    len(resources),
		},
	}, nil
}

// convertTargetHealth converts a single ELBv2 TargetHealthDescription into a generic Resource.
func convertTargetHealth(thd elbv2types.TargetHealthDescription) resource.Resource {
	targetID := ""
	port := ""
	az := ""

	if thd.Target != nil {
		if thd.Target.Id != nil {
			targetID = *thd.Target.Id
		}
		if thd.Target.Port != nil {
			port = fmt.Sprintf("%d", *thd.Target.Port)
		}
		if thd.Target.AvailabilityZone != nil {
			az = *thd.Target.AvailabilityZone
		}
	}

	health := ""
	reason := ""
	description := ""

	if thd.TargetHealth != nil {
		health = string(thd.TargetHealth.State)
		reason = string(thd.TargetHealth.Reason)
		if thd.TargetHealth.Description != nil {
			description = *thd.TargetHealth.Description
		}
	}

	return resource.Resource{
		ID:     targetID,
		Name:   targetID,
		Status: health,
		Fields: map[string]string{
			"target_id":   targetID,
			"port":        port,
			"az":          az,
			"health":      health,
			"reason":      reason,
			"description": description,
		},
		RawStruct: thd,
	}
}
