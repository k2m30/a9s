package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

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

		addrDomain := string(addr.Domain)

		// Compute attachment status: UNATTACHED if no association/instance/NIC.
		eipStatus := "ATTACHED"
		unassociated := addr.AssociationId == nil && addr.InstanceId == nil && addr.NetworkInterfaceId == nil
		if unassociated {
			eipStatus = "UNATTACHED"
		}

		r := resource.Resource{
			ID:   allocationID,
			Name: name,
			// Fields["status"] carries the association state (ATTACHED/UNATTACHED);
			// an EIP has no health lifecycle state, so it emits no Finding.
			Fields: map[string]string{
				"allocation_id":  allocationID,
				"name":           name,
				"public_ip":      publicIP,
				"association_id": associationID,
				"instance_id":    instanceID,
				"domain":         addrDomain,
				"status":         eipStatus,
			},
			RawStruct: addr,
		}

		// emit CodeEIPUnassociated Finding when the EIP is
		// allocated but not associated with any instance, ENI, or NAT gateway.
		if unassociated {
			r.Findings = []domain.Finding{{
				Code: CodeEIPUnassociated, Phrase: "unassociated",
				Severity: domain.SevWarn, Source: "wave1",
			}}
		}

		resources = append(resources, r)
	}

	return resources, nil
}
