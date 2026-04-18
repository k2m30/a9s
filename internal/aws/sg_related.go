// sg_related.go contains Security Group related-resource checker functions.
package aws

import (
	"context"
	"slices"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("sg", []resource.RelatedDef{
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkSGVPC, NeedsTargetCache: false},
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkSGEC2, NeedsTargetCache: true},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkSGENI, NeedsTargetCache: true},
		{TargetType: "elb", DisplayName: "Load Balancers", Checker: checkSGELB, NeedsTargetCache: true},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkSGLambda, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkSGCFN, NeedsTargetCache: false},
		{TargetType: "sg", DisplayName: "Referencing SGs", Checker: checkSGSG, NeedsTargetCache: true},
	})

	resource.RegisterNavigableFields("sg", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})
}

// checkSGVPC reads the vpc_id field directly from the SG resource.
// No cache access needed — the field is populated by the SG fetcher.
func checkSGVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	vpcID := res.Fields["vpc_id"]
	if vpcID == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{vpcID})
}

// checkSGEC2 scans the EC2 cache for instances whose SecurityGroups slice
// contains a GroupId matching the security group's ID.
func checkSGEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sgID := res.ID
	if sgID == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	list, truncated, err := sgRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	var ids []string
	for _, r := range list {
		inst, ok := assertStruct[ec2types.Instance](r.RawStruct)
		if !ok {
			continue
		}
		for _, sg := range inst.SecurityGroups {
			if sg.GroupId != nil && *sg.GroupId == sgID {
				ids = append(ids, r.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ec2")
	}
	return relatedResult("ec2", ids)
}

// checkSGENI scans the ENI cache for network interfaces whose Groups slice
// contains a GroupId matching the security group's ID.
func checkSGENI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sgID := res.ID
	if sgID == "" {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}

	list, truncated, err := sgRelatedResources(ctx, clients, cache, "eni")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}

	var ids []string
	for _, r := range list {
		eni, ok := assertStruct[ec2types.NetworkInterface](r.RawStruct)
		if !ok {
			continue
		}
		for _, sg := range eni.Groups {
			if sg.GroupId != nil && *sg.GroupId == sgID {
				ids = append(ids, r.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("eni")
	}
	return relatedResult("eni", ids)
}

// checkSGELB scans the ELB cache for load balancers whose SecurityGroups slice
// contains a value matching the security group's ID.
func checkSGELB(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sgID := res.ID
	if sgID == "" {
		return resource.RelatedCheckResult{TargetType: "elb", Count: 0}
	}

	list, truncated, err := sgRelatedResources(ctx, clients, cache, "elb")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "elb", Count: -1}
	}

	var ids []string
	for _, r := range list {
		lb, ok := assertStruct[elbv2types.LoadBalancer](r.RawStruct)
		if !ok {
			continue
		}
		if slices.Contains(lb.SecurityGroups, sgID) {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("elb")
	}
	return relatedResult("elb", ids)
}

// checkSGCFN checks the SG's tags for aws:cloudformation:stack-name.
// No cache access needed — the tag carries the stack name directly.
func checkSGCFN(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ec2types.SecurityGroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	stackName := tagValue(raw.Tags, "aws:cloudformation:stack-name")
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}
	return relatedResult("cfn", []string{stackName})
}

// checkSGSG scans the SG cache for other security groups whose IpPermissions or
// IpPermissionsEgress contain a UserIdGroupPair referencing the source SG's ID.
// This answers "which other SGs reference this one?" — critical for blast-radius analysis.
func checkSGSG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sgID := res.ID
	if sgID == "" {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}

	list, truncated, err := sgRelatedResources(ctx, clients, cache, "sg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}

	var ids []string
	for _, r := range list {
		if r.ID == sgID {
			continue // skip self
		}
		candidate, ok := assertStruct[ec2types.SecurityGroup](r.RawStruct)
		if !ok {
			continue
		}
		if sgReferencedInPermissions(candidate.IpPermissions, sgID) || sgReferencedInPermissions(candidate.IpPermissionsEgress, sgID) {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("sg")
	}
	return relatedResult("sg", ids)
}

// checkSGLambda scans the Lambda cache for functions whose VpcConfig.SecurityGroupIds
// slice contains this security group's ID. Pattern C — reverse lookup.
func checkSGLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	sgID := res.ID
	if sgID == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}

	list, truncated, err := sgRelatedResources(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if list == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}

	var ids []string
	for _, r := range list {
		fn, ok := assertStruct[lambdatypes.FunctionConfiguration](r.RawStruct)
		if !ok {
			continue
		}
		if fn.VpcConfig == nil {
			continue
		}
		if slices.Contains(fn.VpcConfig.SecurityGroupIds, sgID) {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("lambda")
	}
	return relatedResult("lambda", ids)
}

// sgReferencedInPermissions returns true if any IpPermission in the slice contains
// a UserIdGroupPair whose GroupId matches the given sgID.
func sgReferencedInPermissions(perms []ec2types.IpPermission, sgID string) bool {
	for _, perm := range perms {
		for _, pair := range perm.UserIdGroupPairs {
			if pair.GroupId != nil && *pair.GroupId == sgID {
				return true
			}
		}
	}
	return false
}

// sgRelatedResources returns the resource list for target from cache or fetches
// the first page via the registered paginated fetcher.
func sgRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

