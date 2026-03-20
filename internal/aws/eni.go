package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("eni", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchNetworkInterfaces(ctx, c.EC2)
	})
	resource.RegisterFieldKeys("eni", []string{"eni_id", "name", "status", "type", "vpc_id", "private_ip"})
}

// FetchNetworkInterfaces calls the EC2 DescribeNetworkInterfaces API and converts the
// response into a slice of generic Resource structs.
func FetchNetworkInterfaces(ctx context.Context, api EC2DescribeNetworkInterfacesAPI) ([]resource.Resource, error) {
	output, err := api.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching network interfaces: %w", err)
	}

	var resources []resource.Resource

	for _, eni := range output.NetworkInterfaces {
		eniID := ""
		if eni.NetworkInterfaceId != nil {
			eniID = *eni.NetworkInterfaceId
		}

		// Extract Name from TagSet (NetworkInterface uses TagSet, not Tags)
		name := ""
		for _, tag := range eni.TagSet {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		status := string(eni.Status)
		interfaceType := string(eni.InterfaceType)

		vpcID := ""
		if eni.VpcId != nil {
			vpcID = *eni.VpcId
		}

		privateIP := ""
		if eni.PrivateIpAddress != nil {
			privateIP = *eni.PrivateIpAddress
		}

		r := resource.Resource{
			ID:     eniID,
			Name:   name,
			Status: status,
			Fields: map[string]string{
				"eni_id":     eniID,
				"name":       name,
				"status":     status,
				"type":       interfaceType,
				"vpc_id":     vpcID,
				"private_ip": privateIP,
			},
			RawStruct:  eni,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
