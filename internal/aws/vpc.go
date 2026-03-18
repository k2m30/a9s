package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("vpc", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchVPCs(ctx, c.EC2)
	})
}

// FetchVPCs calls the EC2 DescribeVpcs API and converts the
// response into a slice of generic Resource structs.
func FetchVPCs(ctx context.Context, api EC2DescribeVpcsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, vpc := range output.Vpcs {
		// Extract VPC ID
		vpcID := ""
		if vpc.VpcId != nil {
			vpcID = *vpc.VpcId
		}

		// Extract Name from Tags
		name := ""
		for _, tag := range vpc.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		// Extract CIDR Block
		cidrBlock := ""
		if vpc.CidrBlock != nil {
			cidrBlock = *vpc.CidrBlock
		}

		// Extract State
		state := string(vpc.State)

		// Extract IsDefault
		isDefault := "false"
		if vpc.IsDefault != nil && *vpc.IsDefault {
			isDefault = "true"
		}

		// Build DetailData
		detail := map[string]string{
			"VPC ID":     vpcID,
			"Name":       name,
			"CIDR Block": cidrBlock,
			"State":      state,
			"Is Default": isDefault,
		}

		// DHCP Options ID
		if vpc.DhcpOptionsId != nil {
			detail["DHCP Options ID"] = *vpc.DhcpOptionsId
		} else {
			detail["DHCP Options ID"] = ""
		}

		// Instance Tenancy
		detail["Instance Tenancy"] = string(vpc.InstanceTenancy)

		// Owner ID
		if vpc.OwnerId != nil {
			detail["Owner ID"] = *vpc.OwnerId
		} else {
			detail["Owner ID"] = ""
		}

		// Tags
		for _, tag := range vpc.Tags {
			if tag.Key != nil && tag.Value != nil {
				detail[fmt.Sprintf("Tag: %s", *tag.Key)] = *tag.Value
			}
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(vpc, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     vpcID,
			Name:   name,
			Status: state,
			Fields: map[string]string{
				"vpc_id":     vpcID,
				"name":       name,
				"cidr_block": cidrBlock,
				"state":      state,
				"is_default": isDefault,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  vpc,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
