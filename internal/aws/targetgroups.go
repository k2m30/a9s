package aws

import (
	"context"
	"fmt"

	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("tg", []string{"target_group_name", "port", "protocol", "vpc_id", "target_type", "health_check_path"})

	resource.RegisterPaginated("tg", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchTargetGroupsPage(ctx, c.ELBv2, continuationToken)
	})
}

// FetchTargetGroups calls the ELBv2 DescribeTargetGroups API and converts the
// response into a slice of generic Resource structs.
func FetchTargetGroups(ctx context.Context, api ELBv2DescribeTargetGroupsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchTargetGroupsPage(ctx, api, token)
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

// FetchTargetGroupsPage fetches a single page of target groups.
func FetchTargetGroupsPage(ctx context.Context, api ELBv2DescribeTargetGroupsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &elbv2.DescribeTargetGroupsInput{}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeTargetGroups(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching target groups: %w", err)
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

		tgArn := ""
		if tg.TargetGroupArn != nil {
			tgArn = *tg.TargetGroupArn
		}

		r := resource.Resource{
			ID:     tgName,
			Name:   tgName,
			Status: "",
			Fields: map[string]string{
				"target_group_name": tgName,
				"target_group_arn":  tgArn,
				"port":              port,
				"protocol":          protocol,
				"vpc_id":            vpcID,
				"target_type":       targetType,
				"health_check_path": healthCheckPath,
			},
			RawStruct: tg,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextMarker != nil {
		nextToken = *output.NextMarker
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
