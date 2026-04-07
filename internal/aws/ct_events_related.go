// ct_events_related.go contains CloudTrail related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkCtEventsUser matches the event username against the iam-user cache.
// Pattern C — cache lookup by name/ID.
func checkCtEventsUser(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	username := res.Fields["user"]
	if username == "" {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: 0}
	}

	userList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "iam-user")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1, Err: err}
	}
	if userList == nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1}
	}

	var ids []string
	for _, userRes := range userList {
		if userRes.Name == username || userRes.ID == username {
			ids = append(ids, userRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1}
	}
	return relatedResult("iam-user", ids)
}

// checkCtEventsRole extracts role information from the CloudTrail event's
// Resources slice (AWS::IAM::Role) and matches against the role cache.
// Pattern C — cache lookup by name extracted from ARN.
func checkCtEventsRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := ctEventsExtractRoleName(res)
	if roleName == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}

	roleList, truncated, err := ctEventsRelatedResources(ctx, clients, cache, "role")
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	return relatedResult("role", ids)
}

// ctEventsExtractRoleName attempts to find a role name from the CloudTrail event.
// It first inspects the event's Resources slice for AWS::IAM::Role entries and
// extracts the name from the ResourceName ARN (last segment after "/"). If no
// role resource is found, it falls back to the Username field — some role-based
// events encode the role as "AWSServiceRole/RoleName".
func ctEventsExtractRoleName(res resource.Resource) string {
	event, ok := assertStruct[cloudtrailtypes.Event](res.RawStruct)
	if ok {
		for _, r := range event.Resources {
			if r.ResourceType != nil && strings.Contains(*r.ResourceType, "Role") {
				if r.ResourceName != nil && *r.ResourceName != "" {
					name := *r.ResourceName
					if idx := strings.LastIndex(name, "/"); idx >= 0 && idx < len(name)-1 {
						return name[idx+1:]
					}
					return name
				}
			}
		}
	}

	// Fallback: check if Username encodes a service role path (e.g. "AWSServiceRole/RoleName").
	username := res.Fields["user"]
	if strings.Contains(username, "/") {
		return username[strings.LastIndex(username, "/")+1:]
	}

	// Third path: AssumedRole events store role info in the CloudTrailEvent JSON string.
	if ok {
		if name := extractRoleNameFromCTEventJSON(event.CloudTrailEvent); name != "" {
			return name
		}
	}

	return ""
}

// ctEventsRelatedResources returns the resource list for target from cache or
// fetches the first page via the registered paginated fetcher.
func ctEventsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
