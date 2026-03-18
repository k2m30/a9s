package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("sg", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSecurityGroups(ctx, c.EC2)
	})
}

// FetchSecurityGroups calls the EC2 DescribeSecurityGroups API and converts
// the response into a slice of generic Resource structs.
func FetchSecurityGroups(ctx context.Context, api EC2DescribeSecurityGroupsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return nil, err
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

		// Build DetailData
		detail := buildSGDetailData(sg, groupID, groupName, vpcID, description)

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(sg, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  sg,
		}

		resources = append(resources, r)
	}

	return resources, nil
}

func buildSGDetailData(sg ec2types.SecurityGroup, groupID, groupName, vpcID, description string) map[string]string {
	detail := map[string]string{
		"Group ID":    groupID,
		"Group Name":  groupName,
		"VPC ID":      vpcID,
		"Description": description,
	}

	// Owner ID
	if sg.OwnerId != nil {
		detail["Owner ID"] = *sg.OwnerId
	} else {
		detail["Owner ID"] = ""
	}

	// Security Group ARN
	if sg.SecurityGroupArn != nil {
		detail["Security Group ARN"] = *sg.SecurityGroupArn
	} else {
		detail["Security Group ARN"] = ""
	}

	// Inbound Rules count
	inboundCount := len(sg.IpPermissions)
	if inboundCount == 1 {
		detail["Inbound Rules"] = "1 rule"
	} else {
		detail["Inbound Rules"] = fmt.Sprintf("%d rules", inboundCount)
	}

	// Outbound Rules count
	outboundCount := len(sg.IpPermissionsEgress)
	if outboundCount == 1 {
		detail["Outbound Rules"] = "1 rule"
	} else {
		detail["Outbound Rules"] = fmt.Sprintf("%d rules", outboundCount)
	}

	// Tags
	for _, tag := range sg.Tags {
		if tag.Key != nil && tag.Value != nil {
			detail["Tag: "+*tag.Key] = *tag.Value
		}
	}

	return detail
}
