package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("asg", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAutoScalingGroups(ctx, c.AutoScaling)
	})
	resource.RegisterFieldKeys("asg", []string{"asg_name", "min_size", "max_size", "desired", "instances", "status"})
}

// FetchAutoScalingGroups calls the AutoScaling DescribeAutoScalingGroups API and converts the
// response into a slice of generic Resource structs.
func FetchAutoScalingGroups(ctx context.Context, api ASGDescribeAutoScalingGroupsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, asg := range output.AutoScalingGroups {
		asgName := ""
		if asg.AutoScalingGroupName != nil {
			asgName = *asg.AutoScalingGroupName
		}

		minSize := ""
		if asg.MinSize != nil {
			minSize = fmt.Sprintf("%d", *asg.MinSize)
		}

		maxSize := ""
		if asg.MaxSize != nil {
			maxSize = fmt.Sprintf("%d", *asg.MaxSize)
		}

		desired := ""
		if asg.DesiredCapacity != nil {
			desired = fmt.Sprintf("%d", *asg.DesiredCapacity)
		}

		instances := fmt.Sprintf("%d", len(asg.Instances))

		status := ""
		if asg.Status != nil {
			status = *asg.Status
		}

		detail := map[string]string{
			"ASG Name":          asgName,
			"Min Size":          minSize,
			"Max Size":          maxSize,
			"Desired Capacity":  desired,
			"Instances":         instances,
			"Status":            status,
			"Availability Zones": strings.Join(asg.AvailabilityZones, ", "),
		}

		if asg.AutoScalingGroupARN != nil {
			detail["ARN"] = *asg.AutoScalingGroupARN
		}

		if asg.LaunchConfigurationName != nil {
			detail["Launch Config"] = *asg.LaunchConfigurationName
		}

		if asg.HealthCheckType != nil {
			detail["Health Check Type"] = *asg.HealthCheckType
		}

		if asg.CreatedTime != nil {
			detail["Created Time"] = asg.CreatedTime.Format("2006-01-02T15:04:05Z07:00")
		}

		if asg.DefaultCooldown != nil {
			detail["Default Cooldown"] = fmt.Sprintf("%d", *asg.DefaultCooldown)
		}

		for _, tag := range asg.Tags {
			if tag.Key != nil && tag.Value != nil {
				detail[fmt.Sprintf("Tag: %s", *tag.Key)] = *tag.Value
			}
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(asg, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     asgName,
			Name:   asgName,
			Status: status,
			Fields: map[string]string{
				"asg_name":  asgName,
				"min_size":  minSize,
				"max_size":  maxSize,
				"desired":   desired,
				"instances": instances,
				"status":    status,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  asg,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
