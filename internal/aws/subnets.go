package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("subnet", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSubnets(ctx, c.EC2)
	})
	resource.RegisterFieldKeys("subnet", []string{"subnet_id", "name", "vpc_id", "cidr_block", "availability_zone", "state", "available_ips"})
}

// FetchSubnets calls the EC2 DescribeSubnets API and converts the
// response into a slice of generic Resource structs.
func FetchSubnets(ctx context.Context, api EC2DescribeSubnetsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, subnet := range output.Subnets {
		subnetID := ""
		if subnet.SubnetId != nil {
			subnetID = *subnet.SubnetId
		}

		name := ""
		for _, tag := range subnet.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		vpcID := ""
		if subnet.VpcId != nil {
			vpcID = *subnet.VpcId
		}

		cidrBlock := ""
		if subnet.CidrBlock != nil {
			cidrBlock = *subnet.CidrBlock
		}

		az := ""
		if subnet.AvailabilityZone != nil {
			az = *subnet.AvailabilityZone
		}

		state := string(subnet.State)

		availableIPs := ""
		if subnet.AvailableIpAddressCount != nil {
			availableIPs = fmt.Sprintf("%d", *subnet.AvailableIpAddressCount)
		}

		detail := map[string]string{
			"Subnet ID":         subnetID,
			"Name":              name,
			"VPC ID":            vpcID,
			"CIDR Block":        cidrBlock,
			"Availability Zone": az,
			"State":             state,
			"Available IPs":     availableIPs,
		}

		if subnet.MapPublicIpOnLaunch != nil {
			detail["Map Public IP"] = fmt.Sprintf("%t", *subnet.MapPublicIpOnLaunch)
		}

		if subnet.DefaultForAz != nil {
			detail["Default for AZ"] = fmt.Sprintf("%t", *subnet.DefaultForAz)
		}

		if subnet.OwnerId != nil {
			detail["Owner ID"] = *subnet.OwnerId
		}

		if subnet.SubnetArn != nil {
			detail["ARN"] = *subnet.SubnetArn
		}

		for _, tag := range subnet.Tags {
			if tag.Key != nil && tag.Value != nil {
				detail[fmt.Sprintf("Tag: %s", *tag.Key)] = *tag.Value
			}
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(subnet, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     subnetID,
			Name:   name,
			Status: state,
			Fields: map[string]string{
				"subnet_id":         subnetID,
				"name":              name,
				"vpc_id":            vpcID,
				"cidr_block":        cidrBlock,
				"availability_zone": az,
				"state":             state,
				"available_ips":     availableIPs,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  subnet,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
