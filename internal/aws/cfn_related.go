// cfn_related.go contains CloudFormation related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkCfnRole extracts the RoleARN from the CloudFormation Stack RawStruct.
// It extracts the role name from the last path segment of the ARN (after the last "/")
// and searches the role cache by name or ID.
// Pattern F — forward field lookup.
func checkCfnRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stack, ok := assertStruct[cfntypes.Stack](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if stack.RoleARN == nil || *stack.RoleARN == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleARN := *stack.RoleARN
	roleName := roleARN
	if idx := strings.LastIndex(roleARN, "/"); idx >= 0 && idx < len(roleARN)-1 {
		roleName = roleARN[idx+1:]
	}

	roleList, _, err := cfnRelatedResources(ctx, clients, cache, "role")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if roleList == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	var ids []string
	for _, roleRes := range roleList {
		if roleRes.Name == roleName || roleRes.ID == roleName {
			ids = append(ids, roleRes.ID)
		}
	}
	return relatedResult("role", ids)
}

// checkCFNCFN finds related CloudFormation stacks — parent and child (nested) stacks.
// Pattern F+C: forward lookup for ParentId (this is a nested stack) and reverse scan
// for stacks whose ParentId matches this stack's StackId (children of this stack).
func checkCFNCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stack, ok := assertStruct[cfntypes.Stack](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}

	cfnList, truncated, err := cfnRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}

	// Collect this stack's StackId for reverse lookup.
	thisStackID := ""
	if stack.StackId != nil {
		thisStackID = *stack.StackId
	}

	// Build a set so we don't emit duplicates.
	seen := make(map[string]struct{})

	// Forward: if this stack has a ParentId, it is a nested stack — add the parent.
	if stack.ParentId != nil && *stack.ParentId != "" {
		parentID := *stack.ParentId
		for _, cfnRes := range cfnList {
			rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
			if !cfnOk {
				continue
			}
			if rawCFN.StackId != nil && *rawCFN.StackId == parentID {
				seen[cfnRes.ID] = struct{}{}
			}
		}
	}

	// Reverse: scan for stacks whose ParentId matches this stack's StackId (child stacks).
	if thisStackID != "" {
		for _, cfnRes := range cfnList {
			rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
			if !cfnOk {
				continue
			}
			if rawCFN.ParentId != nil && *rawCFN.ParentId == thisStackID {
				seen[cfnRes.ID] = struct{}{}
			}
		}
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}

	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
}

// cfnRelatedResources returns the resource list for target from cache or by fetching the first page.
func cfnRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
