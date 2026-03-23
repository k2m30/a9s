package aws

import (
	"context"
	"fmt"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("tg_health", []string{"target_id", "port", "az", "health", "reason", "description"})

	resource.RegisterChildFetcher("tg_health", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchTargetHealth(ctx, c.ELBv2, parentCtx["target_group_arn"])
	})
	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Target Health",
		ShortName: "tg_health",
		Columns:   resource.TargetHealthColumns(),
	})
}

// FetchTargetHealth calls the ELBv2 DescribeTargetHealth API for a given
// target group ARN and converts the response into a slice of generic Resource structs.
// No pagination — a single API call returns all targets.
func FetchTargetHealth(ctx context.Context, api ELBv2DescribeTargetHealthAPI, targetGroupArn string) ([]resource.Resource, error) {
	output, err := api.DescribeTargetHealth(ctx, &elbv2.DescribeTargetHealthInput{
		TargetGroupArn: &targetGroupArn,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching target health: %w", err)
	}

	var resources []resource.Resource

	for _, thd := range output.TargetHealthDescriptions {
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

		r := resource.Resource{
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

		resources = append(resources, r)
	}

	return resources, nil
}
