package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
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
		return nil, fmt.Errorf("fetching Elastic IPs: %w", err)
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
			RawStruct:  addr,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
