package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("asg", []string{"asg_name", "min_size", "max_size", "desired", "instances", "status"})

	resource.RegisterPaginated("asg", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAutoScalingGroupsPage(ctx, c.AutoScaling, continuationToken)
	})
}

// FetchAutoScalingGroups calls the AutoScaling DescribeAutoScalingGroups API and converts the
// response into a slice of generic Resource structs.
func FetchAutoScalingGroups(ctx context.Context, api ASGDescribeAutoScalingGroupsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchAutoScalingGroupsPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchAutoScalingGroupsPage fetches a single page of Auto Scaling groups.
func FetchAutoScalingGroupsPage(ctx context.Context, api ASGDescribeAutoScalingGroupsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &autoscaling.DescribeAutoScalingGroupsInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeAutoScalingGroups(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Auto Scaling groups: %w", err)
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
			RawStruct: asg,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
