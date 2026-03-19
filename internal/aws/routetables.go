package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("rtb", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRouteTables(ctx, c.EC2)
	})
}

// FetchRouteTables calls the EC2 DescribeRouteTables API and converts the
// response into a slice of generic Resource structs.
func FetchRouteTables(ctx context.Context, api EC2DescribeRouteTablesAPI) ([]resource.Resource, error) {
	output, err := api.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, rtb := range output.RouteTables {
		rtbID := ""
		if rtb.RouteTableId != nil {
			rtbID = *rtb.RouteTableId
		}

		name := ""
		for _, tag := range rtb.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		vpcID := ""
		if rtb.VpcId != nil {
			vpcID = *rtb.VpcId
		}

		routesCount := fmt.Sprintf("%d", len(rtb.Routes))
		associationsCount := fmt.Sprintf("%d", len(rtb.Associations))

		// Determine if this is the main route table
		isMain := "false"
		for _, assoc := range rtb.Associations {
			if assoc.Main != nil && *assoc.Main {
				isMain = "true"
				break
			}
		}

		detail := map[string]string{
			"Route Table ID": rtbID,
			"Name":           name,
			"VPC ID":         vpcID,
			"Routes":         routesCount,
			"Associations":   associationsCount,
			"Main":           isMain,
		}

		if rtb.OwnerId != nil {
			detail["Owner ID"] = *rtb.OwnerId
		}

		for _, tag := range rtb.Tags {
			if tag.Key != nil && tag.Value != nil {
				detail[fmt.Sprintf("Tag: %s", *tag.Key)] = *tag.Value
			}
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(rtb, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     rtbID,
			Name:   name,
			Status: isMain,
			Fields: map[string]string{
				"route_table_id":    rtbID,
				"name":              name,
				"vpc_id":            vpcID,
				"routes_count":      routesCount,
				"associations_count": associationsCount,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  rtb,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
