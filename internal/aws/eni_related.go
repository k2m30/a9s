// eni_related.go contains Network Interface related-resource checker functions.
package aws

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterNavigableFields("eni", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
		{FieldPath: "Groups.GroupId", TargetType: "sg"},
		{FieldPath: "Attachment.InstanceId", TargetType: "ec2"},
		{FieldPath: "Association.AllocationId", TargetType: "eip"},
	})

	resource.RegisterRelated("eni", []resource.RelatedDef{
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkENIEC2, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkENISG, NeedsTargetCache: true},
		{TargetType: "eip", DisplayName: "Elastic IPs", Checker: checkENIEIP, NeedsTargetCache: true},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkENIVPC},
	})
}

// checkENIEC2 extracts Attachment.InstanceId from the ENI RawStruct and searches
// the ec2 cache for a matching instance.
func checkENIEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	if raw.Attachment == nil || raw.Attachment.InstanceId == nil || *raw.Attachment.InstanceId == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}
	instanceID := *raw.Attachment.InstanceId

	ec2List, truncated, err := eniRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if ec2List == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	var ids []string
	for _, ec2Res := range ec2List {
		if ec2Res.ID == instanceID {
			ids = append(ids, ec2Res.ID)
			break
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	return relatedResult("ec2", ids)
}

// checkENISG extracts Groups[].GroupId from the ENI RawStruct and searches
// the sg cache for matching security groups.
func checkENISG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	if len(raw.Groups) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	groupIDs := make(map[string]struct{}, len(raw.Groups))
	for _, g := range raw.Groups {
		if g.GroupId != nil && *g.GroupId != "" {
			groupIDs[*g.GroupId] = struct{}{}
		}
	}
	if len(groupIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	sgList, truncated, err := eniRelatedResources(ctx, clients, cache, "sg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1, Err: err}
	}
	if sgList == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	var ids []string
	for _, sgRes := range sgList {
		if _, found := groupIDs[sgRes.ID]; found {
			ids = append(ids, sgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	return relatedResult("sg", ids)
}

// checkENIEIP extracts Association.AllocationId from the ENI RawStruct and searches
// the eip cache for a matching Elastic IP.
func checkENIEIP(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.NetworkInterface](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1}
	}
	if raw.Association == nil || raw.Association.AllocationId == nil || *raw.Association.AllocationId == "" {
		return resource.RelatedCheckResult{TargetType: "eip", Count: 0}
	}
	allocationID := *raw.Association.AllocationId

	eipList, truncated, err := eniRelatedResources(ctx, clients, cache, "eip")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1, Err: err}
	}
	if eipList == nil {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1}
	}
	var ids []string
	for _, eipRes := range eipList {
		if eipRes.ID == allocationID {
			ids = append(ids, eipRes.ID)
			break
		}
		eipRaw, eipOk := assertStruct[ec2types.Address](eipRes.RawStruct)
		if eipOk && eipRaw.AllocationId != nil && *eipRaw.AllocationId == allocationID {
			ids = append(ids, eipRes.ID)
			break
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "eip", Count: -1}
	}
	return relatedResult("eip", ids)
}

// checkENIVPC returns the VPC this network interface belongs to (Pattern F).
// Reads vpc_id from Fields which is populated by the ENI fetcher.
func checkENIVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{vpcID})
}






// eniRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func eniRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
