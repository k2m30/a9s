package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("eip", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchElasticIPs(ctx, c.EC2)
	})
	resource.RegisterFieldKeys("eip", []string{"allocation_id", "name", "public_ip", "association_id", "instance_id", "domain"})
}

// FetchElasticIPs calls the EC2 DescribeAddresses API and converts the
// response into a slice of generic Resource structs.
func FetchElasticIPs(ctx context.Context, api EC2DescribeAddressesAPI) ([]resource.Resource, error) {
	output, err := api.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, addr := range output.Addresses {
		allocationID := ""
		if addr.AllocationId != nil {
			allocationID = *addr.AllocationId
		}

		publicIP := ""
		if addr.PublicIp != nil {
			publicIP = *addr.PublicIp
		}

		name := ""
		for _, tag := range addr.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		associationID := ""
		if addr.AssociationId != nil {
			associationID = *addr.AssociationId
		}

		instanceID := ""
		if addr.InstanceId != nil {
			instanceID = *addr.InstanceId
		}

		domain := string(addr.Domain)

		detail := map[string]string{
			"Allocation ID":  allocationID,
			"Public IP":      publicIP,
			"Name":           name,
			"Association ID": associationID,
			"Instance ID":    instanceID,
			"Domain":         domain,
		}

		if addr.PrivateIpAddress != nil {
			detail["Private IP"] = *addr.PrivateIpAddress
		}

		if addr.NetworkInterfaceId != nil {
			detail["Network Interface"] = *addr.NetworkInterfaceId
		}

		for _, tag := range addr.Tags {
			if tag.Key != nil && tag.Value != nil {
				detail[fmt.Sprintf("Tag: %s", *tag.Key)] = *tag.Value
			}
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(addr, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     allocationID,
			Name:   name,
			Status: domain,
			Fields: map[string]string{
				"allocation_id":  allocationID,
				"name":           name,
				"public_ip":      publicIP,
				"association_id": associationID,
				"instance_id":    instanceID,
				"domain":         domain,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  addr,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
