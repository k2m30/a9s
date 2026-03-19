package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("igw", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchInternetGateways(ctx, c.EC2)
	})
	resource.RegisterFieldKeys("igw", []string{"igw_id", "name", "vpc_id", "state"})
}

// FetchInternetGateways calls the EC2 DescribeInternetGateways API and converts the
// response into a slice of generic Resource structs.
func FetchInternetGateways(ctx context.Context, api EC2DescribeInternetGatewaysAPI) ([]resource.Resource, error) {
	output, err := api.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, igw := range output.InternetGateways {
		igwID := ""
		if igw.InternetGatewayId != nil {
			igwID = *igw.InternetGatewayId
		}

		name := ""
		for _, tag := range igw.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		// Extract VPC ID and state from attachments
		vpcID := ""
		state := "detached"
		if len(igw.Attachments) > 0 {
			if igw.Attachments[0].VpcId != nil {
				vpcID = *igw.Attachments[0].VpcId
			}
			state = string(igw.Attachments[0].State)
		}

		detail := map[string]string{
			"Internet Gateway ID": igwID,
			"Name":                name,
			"VPC ID":              vpcID,
			"State":               state,
		}

		if igw.OwnerId != nil {
			detail["Owner ID"] = *igw.OwnerId
		}

		for _, tag := range igw.Tags {
			if tag.Key != nil && tag.Value != nil {
				detail[fmt.Sprintf("Tag: %s", *tag.Key)] = *tag.Value
			}
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(igw, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     igwID,
			Name:   name,
			Status: state,
			Fields: map[string]string{
				"igw_id": igwID,
				"name":   name,
				"vpc_id": vpcID,
				"state":  state,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  igw,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
