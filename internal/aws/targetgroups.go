package aws

import (
	"context"
	"encoding/json"
	"fmt"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/internal/resource"
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
		return nil, err
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

		detail := map[string]string{
			"Target Group Name": tgName,
			"Port":              port,
			"Protocol":          protocol,
			"VPC ID":            vpcID,
			"Target Type":       targetType,
			"Health Check Path": healthCheckPath,
		}

		if tg.TargetGroupArn != nil {
			detail["ARN"] = *tg.TargetGroupArn
		}

		if tg.HealthCheckPort != nil {
			detail["Health Check Port"] = *tg.HealthCheckPort
		}

		detail["Health Check Protocol"] = string(tg.HealthCheckProtocol)

		if tg.HealthCheckIntervalSeconds != nil {
			detail["Health Check Interval"] = fmt.Sprintf("%d", *tg.HealthCheckIntervalSeconds)
		}

		if tg.HealthyThresholdCount != nil {
			detail["Healthy Threshold"] = fmt.Sprintf("%d", *tg.HealthyThresholdCount)
		}

		if tg.UnhealthyThresholdCount != nil {
			detail["Unhealthy Threshold"] = fmt.Sprintf("%d", *tg.UnhealthyThresholdCount)
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(tg, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  tg,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
