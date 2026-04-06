package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("vpce", []string{"vpce_id", "service_name", "type", "state", "vpc_id"})

	resource.RegisterPaginated("vpce", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchVPCEndpointsPage(ctx, c.EC2, continuationToken)
	})
}

// FetchVPCEndpoints calls the EC2 DescribeVpcEndpoints API and converts the
// response into a slice of generic Resource structs.
func FetchVPCEndpoints(ctx context.Context, api EC2DescribeVpcEndpointsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchVPCEndpointsPage(ctx, api, token)
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

// FetchVPCEndpointsPage fetches a single page of VPC endpoints.
func FetchVPCEndpointsPage(ctx context.Context, api EC2DescribeVpcEndpointsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeVpcEndpointsInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeVpcEndpoints(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching VPC endpoints: %w", err)
	}

	var resources []resource.Resource

	for _, vpce := range output.VpcEndpoints {
		vpceID := ""
		if vpce.VpcEndpointId != nil {
			vpceID = *vpce.VpcEndpointId
		}

		serviceName := ""
		if vpce.ServiceName != nil {
			serviceName = *vpce.ServiceName
		}

		endpointType := string(vpce.VpcEndpointType)
		state := string(vpce.State)

		vpcID := ""
		if vpce.VpcId != nil {
			vpcID = *vpce.VpcId
		}

		r := resource.Resource{
			ID:     vpceID,
			Name:   serviceName,
			Status: state,
			Fields: map[string]string{
				"vpce_id":      vpceID,
				"service_name": serviceName,
				"type":         endpointType,
				"state":        state,
				"vpc_id":       vpcID,
			},
			RawStruct: vpce,
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
