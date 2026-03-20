package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("nat", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchNatGateways(ctx, c.EC2)
	})
	resource.RegisterFieldKeys("nat", []string{"nat_gateway_id", "name", "vpc_id", "subnet_id", "state", "public_ip"})
}

// FetchNatGateways calls the EC2 DescribeNatGateways API and converts the
// response into a slice of generic Resource structs.
func FetchNatGateways(ctx context.Context, api EC2DescribeNatGatewaysAPI) ([]resource.Resource, error) {
	output, err := api.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching NAT gateways: %w", err)
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
			RawStruct:  nat,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
