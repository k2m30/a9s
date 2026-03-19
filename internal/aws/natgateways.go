package aws

import (
	"context"
	"encoding/json"
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
}

// FetchNatGateways calls the EC2 DescribeNatGateways API and converts the
// response into a slice of generic Resource structs.
func FetchNatGateways(ctx context.Context, api EC2DescribeNatGatewaysAPI) ([]resource.Resource, error) {
	output, err := api.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{})
	if err != nil {
		return nil, err
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

		connectivityType := string(nat.ConnectivityType)

		detail := map[string]string{
			"NAT Gateway ID":    natID,
			"Name":              name,
			"VPC ID":            vpcID,
			"Subnet ID":         subnetID,
			"State":             state,
			"Public IP":         publicIP,
			"Connectivity Type": connectivityType,
		}

		if nat.CreateTime != nil {
			detail["Create Time"] = nat.CreateTime.Format("2006-01-02T15:04:05Z07:00")
		}

		for _, tag := range nat.Tags {
			if tag.Key != nil && tag.Value != nil {
				detail[fmt.Sprintf("Tag: %s", *tag.Key)] = *tag.Value
			}
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(nat, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  nat,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
