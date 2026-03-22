package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("sg", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSecurityGroups(ctx, c.EC2)
	})
	resource.RegisterFieldKeys("sg", []string{"group_id", "group_name", "vpc_id", "description"})
}

// FetchSecurityGroups calls the EC2 DescribeSecurityGroups API and converts
// the response into a slice of generic Resource structs.
func FetchSecurityGroups(ctx context.Context, api EC2DescribeSecurityGroupsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching security groups: %w", err)
	}

	var resources []resource.Resource

	for _, sg := range output.SecurityGroups {
		// Extract GroupId
		groupID := ""
		if sg.GroupId != nil {
			groupID = *sg.GroupId
		}

		// Extract GroupName
		groupName := ""
		if sg.GroupName != nil {
			groupName = *sg.GroupName
		}

		// Extract VpcId
		vpcID := ""
		if sg.VpcId != nil {
			vpcID = *sg.VpcId
		}

		// Extract Description
		description := ""
		if sg.Description != nil {
			description = *sg.Description
		}

		r := resource.Resource{
			ID:     groupID,
			Name:   groupName,
			Status: "", // SGs have no status field
			Fields: map[string]string{
				"group_id":    groupID,
				"group_name":  groupName,
				"vpc_id":      vpcID,
				"description": description,
			},
			RawStruct:  sg,
		}

		resources = append(resources, r)
	}

	return resources, nil
}

