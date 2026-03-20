package aws

import (
	"context"
	"fmt"

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
		return nil, fmt.Errorf("fetching Auto Scaling groups: %w", err)
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
			RawStruct:  asg,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
