package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("nat", []string{"nat_gateway_id", "name", "vpc_id", "subnet_id", "state", "public_ip"})

	resource.RegisterPaginated("nat", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchNatGatewaysPage(ctx, c.EC2, continuationToken)
	})

	resource.RegisterNavigableFields("nat", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
		{FieldPath: "NatGatewayAddresses.AllocationId", TargetType: "eip"},
	})

	resource.RegisterRelated("nat", []resource.RelatedDef{
		{TargetType: "vpc", DisplayName: "VPCs", Checker: checkNATVPC, NeedsTargetCache: true},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkNATSubnet, NeedsTargetCache: true},
		{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkNATRTB, NeedsTargetCache: true},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkNATAlarm, NeedsTargetCache: true},
		{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkNATEIP, NeedsTargetCache: true},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkNATENI, NeedsTargetCache: true},
	})
}

// FetchNatGateways calls the EC2 DescribeNatGateways API and converts the
// response into a slice of generic Resource structs.
func FetchNatGateways(ctx context.Context, api EC2DescribeNatGatewaysAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchNatGatewaysPage(ctx, api, token)
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

// FetchNatGatewaysPage fetches a single page of NAT gateways.
func FetchNatGatewaysPage(ctx context.Context, api EC2DescribeNatGatewaysAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeNatGatewaysInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeNatGateways(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching NAT gateways: %w", err)
	}

	var resources []resource.Resource

	for _, nat := range output.NatGateways {
		natID := ""
		if nat.NatGatewayId != nil {
			natID = *nat.NatGatewayId
		}

		name := ""
		for _, tag := range nat.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		vpcID := ""
		if nat.VpcId != nil {
			vpcID = *nat.VpcId
		}

		subnetID := ""
		if nat.SubnetId != nil {
			subnetID = *nat.SubnetId
		}

		state := string(nat.State)

		// Extract public IP from NatGatewayAddresses
		publicIP := ""
		if len(nat.NatGatewayAddresses) > 0 {
			if nat.NatGatewayAddresses[0].PublicIp != nil {
				publicIP = *nat.NatGatewayAddresses[0].PublicIp
			}
		}

		r := resource.Resource{
			ID:     natID,
			Name:   name,
			Status: state,
			Fields: map[string]string{
				"nat_gateway_id": natID,
				"name":           name,
				"vpc_id":         vpcID,
				"subnet_id":      subnetID,
				"state":          state,
				"public_ip":      publicIP,
			},
			RawStruct: nat,
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
