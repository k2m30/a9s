// ng_related.go contains EKS Node Group related-resource checker functions.
package aws

import (
	"context"
	"strings"

	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkNGEKS extracts ClusterName from the Node Group RawStruct and searches
// the eks cache for a matching cluster by name.
func checkNGEKS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1}
	}
	clusterName := res.Fields["cluster_name"]
	if ng.ClusterName != nil && *ng.ClusterName != "" {
		clusterName = *ng.ClusterName
	}
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "eks", Count: 0}
	}

	eksList, _, err := ngRelatedResources(ctx, clients, cache, "eks")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1, Err: err}
	}
	if eksList == nil {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1}
	}

	var ids []string
	for _, eksRes := range eksList {
		if eksRes.Name == clusterName || eksRes.Fields["cluster_name"] == clusterName {
			ids = append(ids, eksRes.ID)
		}
	}
	return relatedResult("eks", ids)
}

// checkNGRole extracts the NodeRole ARN from the Node Group RawStruct, derives
// the role name from the last "/" segment, and searches the role cache by name.
func checkNGRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if ng.NodeRole == nil || *ng.NodeRole == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleVal := *ng.NodeRole
	roleName := roleVal
	if idx := strings.LastIndex(roleVal, "/"); idx >= 0 && idx < len(roleVal)-1 {
		roleName = roleVal[idx+1:]
	}

	roleList, _, err := ngRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.Fields["role_name"] == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	return relatedResult("role", ids)
}

// checkNGASG extracts Resources.AutoScalingGroups from the Node Group RawStruct
// and searches the asg cache for matching ASGs by name.
func checkNGASG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	ng, ok := assertStruct[ekstypes.Nodegroup](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	if ng.Resources == nil || len(ng.Resources.AutoScalingGroups) == 0 {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}

	asgNames := make(map[string]struct{}, len(ng.Resources.AutoScalingGroups))
	for _, asg := range ng.Resources.AutoScalingGroups {
		if asg.Name != nil && *asg.Name != "" {
			asgNames[*asg.Name] = struct{}{}
		}
	}
	if len(asgNames) == 0 {
		return resource.RelatedCheckResult{TargetType: "asg", Count: 0}
	}

	asgList, truncated, err := ngRelatedResources(ctx, clients, cache, "asg")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1, Err: err}
	}
	if asgList == nil {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}

	var ids []string
	for _, asgRes := range asgList {
		if _, found := asgNames[asgRes.ID]; found {
			ids = append(ids, asgRes.ID)
			continue
		}
		if _, found := asgNames[asgRes.Name]; found {
			ids = append(ids, asgRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "asg", Count: -1}
	}
	return relatedResult("asg", ids)
}

// ngRelatedResources returns the resource list for target from cache or by fetching the first page.
func ngRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
