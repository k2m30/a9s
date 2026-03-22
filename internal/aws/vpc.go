package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("vpc", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchVPCs(ctx, c.EC2)
	})
	resource.RegisterFieldKeys("vpc", []string{"vpc_id", "name", "cidr_block", "state", "is_default"})
}

// FetchVPCs calls the EC2 DescribeVpcs API and converts the
// response into a slice of generic Resource structs.
func FetchVPCs(ctx context.Context, api EC2DescribeVpcsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching VPCs: %w", err)
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
			RawStruct:  vpc,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
