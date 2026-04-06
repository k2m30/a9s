package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("eni", []string{"eni_id", "name", "status", "type", "vpc_id", "private_ip"})

	resource.RegisterPaginated("eni", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchNetworkInterfacesPage(ctx, c.EC2, continuationToken)
	})
}

// FetchNetworkInterfaces calls the EC2 DescribeNetworkInterfaces API and converts the
// response into a slice of generic Resource structs.
func FetchNetworkInterfaces(ctx context.Context, api EC2DescribeNetworkInterfacesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchNetworkInterfacesPage(ctx, api, token)
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

// FetchNetworkInterfacesPage fetches a single page of network interfaces.
func FetchNetworkInterfacesPage(ctx context.Context, api EC2DescribeNetworkInterfacesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeNetworkInterfacesInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeNetworkInterfaces(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching network interfaces: %w", err)
	}

	var resources []resource.Resource

	for _, eni := range output.NetworkInterfaces {
		eniID := ""
		if eni.NetworkInterfaceId != nil {
			eniID = *eni.NetworkInterfaceId
		}

		// Extract Name from TagSet (NetworkInterface uses TagSet, not Tags)
		name := ""
		for _, tag := range eni.TagSet {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		status := string(eni.Status)
		interfaceType := string(eni.InterfaceType)

		vpcID := ""
		if eni.VpcId != nil {
			vpcID = *eni.VpcId
		}

		privateIP := ""
		if eni.PrivateIpAddress != nil {
			privateIP = *eni.PrivateIpAddress
		}

		r := resource.Resource{
			ID:     eniID,
			Name:   name,
			Status: status,
			Fields: map[string]string{
				"eni_id":     eniID,
				"name":       name,
				"status":     status,
				"type":       interfaceType,
				"vpc_id":     vpcID,
				"private_ip": privateIP,
			},
			RawStruct: eni,
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
