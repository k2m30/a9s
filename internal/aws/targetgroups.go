package aws

import (
	"context"
	"fmt"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("tg", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchTargetGroups(ctx, c.ELBv2)
	})
	resource.RegisterFieldKeys("tg", []string{"target_group_name", "port", "protocol", "vpc_id", "target_type", "health_check_path"})
}

// FetchTargetGroups calls the ELBv2 DescribeTargetGroups API and converts the
// response into a slice of generic Resource structs.
func FetchTargetGroups(ctx context.Context, api ELBv2DescribeTargetGroupsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeTargetGroups(ctx, &elbv2.DescribeTargetGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching target groups: %w", err)
	}

	var resources []resource.Resource

	for _, tg := range output.TargetGroups {
		tgName := ""
		if tg.TargetGroupName != nil {
			tgName = *tg.TargetGroupName
		}

		port := ""
		if tg.Port != nil {
			port = fmt.Sprintf("%d", *tg.Port)
		}

		protocol := string(tg.Protocol)

		vpcID := ""
		if tg.VpcId != nil {
			vpcID = *tg.VpcId
		}

		targetType := string(tg.TargetType)

		healthCheckPath := ""
		if tg.HealthCheckPath != nil {
			healthCheckPath = *tg.HealthCheckPath
		}

		r := resource.Resource{
			ID:     tgName,
			Name:   tgName,
			Status: "",
			Fields: map[string]string{
				"target_group_name": tgName,
				"port":              port,
				"protocol":          protocol,
				"vpc_id":            vpcID,
				"target_type":       targetType,
				"health_check_path": healthCheckPath,
			},
			RawStruct:  tg,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
