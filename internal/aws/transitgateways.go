package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("tgw", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchTransitGateways(ctx, c.EC2)
	})
	resource.RegisterFieldKeys("tgw", []string{"tgw_id", "name", "state", "owner_id", "description"})
}

// FetchTransitGateways calls the EC2 DescribeTransitGateways API and converts the
// response into a slice of generic Resource structs.
func FetchTransitGateways(ctx context.Context, api EC2DescribeTransitGatewaysAPI) ([]resource.Resource, error) {
	output, err := api.DescribeTransitGateways(ctx, &ec2.DescribeTransitGatewaysInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching transit gateways: %w", err)
	}

	var resources []resource.Resource

	for _, tgw := range output.TransitGateways {
		tgwID := ""
		if tgw.TransitGatewayId != nil {
			tgwID = *tgw.TransitGatewayId
		}

		// Extract Name from Tags
		name := ""
		for _, tag := range tgw.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		state := string(tgw.State)

		ownerID := ""
		if tgw.OwnerId != nil {
			ownerID = *tgw.OwnerId
		}

		description := ""
		if tgw.Description != nil {
			description = *tgw.Description
		}

		r := resource.Resource{
			ID:     tgwID,
			Name:   name,
			Status: state,
			Fields: map[string]string{
				"tgw_id":      tgwID,
				"name":        name,
				"state":       state,
				"owner_id":    ownerID,
				"description": description,
			},
			RawStruct:  tgw,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
