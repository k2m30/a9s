// eip_related.go contains Elastic IP related-resource checker functions.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterNavigableFields("eip", []resource.NavigableField{
		{FieldPath: "InstanceId", TargetType: "ec2"},
		{FieldPath: "NetworkInterfaceId", TargetType: "eni"},
	})

	resource.RegisterRelated("eip", []resource.RelatedDef{
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkEIPEC2},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkEIPENI},
		{TargetType: "nat", DisplayName: "NAT Gateways", Checker: checkEIPNAT, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEIPKMS},
	})
}

// checkEIPEC2 returns the EC2 instance associated with this Elastic IP (Pattern F).
func checkEIPEC2(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	if raw.InstanceId == nil || *raw.InstanceId == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	return relatedResult("ec2", []string{*raw.InstanceId})
}

// checkEIPENI returns the network interface associated with this Elastic IP (Pattern F).
func checkEIPENI(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	if raw.NetworkInterfaceId == nil || *raw.NetworkInterfaceId == "" {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	return relatedResult("eni", []string{*raw.NetworkInterfaceId})
}

// checkEIPNAT checks the NAT gateway cache for NAT gateways using this Elastic IP
// allocation (Pattern C — search target cache by AllocationId).
func checkEIPNAT(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	// Resolve the allocation ID from the RawStruct or from res.ID.
	allocationID := res.ID
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if ok && raw.AllocationId != nil && *raw.AllocationId != "" {
		allocationID = *raw.AllocationId
	}
	if allocationID == "" {
		return resource.RelatedCheckResult{TargetType: "nat", Count: 0}
	}

	natList, truncated, err := eipRelatedResources(ctx, clients, cache, "nat")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1, Err: err}
	}
	if natList == nil {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1}
	}

	var ids []string
	for _, natRes := range natList {
		natRaw, natOk := assertStruct[ec2types.NatGateway](natRes.RawStruct)
		if natOk {
			for _, addr := range natRaw.NatGatewayAddresses {
				if addr.AllocationId != nil && *addr.AllocationId == allocationID {
					ids = append(ids, natRes.ID)
					break
				}
			}
			continue
		}
		// Fallback: check Fields keys for allocation ID values.
		for _, v := range natRes.Fields {
			if v == allocationID {
				ids = append(ids, natRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "nat", Count: -1}
	}
	return relatedResult("nat", ids)
}

// checkEIPKMS is a stub. Elastic IP addresses are not KMS-encrypted resources
// and do not carry a KMS key reference.
func checkEIPKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
}

// eipRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func eipRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
