// cb_related.go contains CodeBuild related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkCbRole extracts the ServiceRole ARN from the CodeBuild Project RawStruct.
// It extracts the role name from the last path segment of the ARN (after the last "/")
// and searches the role cache by name.
// Pattern F — forward field lookup.
func checkCbRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if project.ServiceRole == nil || *project.ServiceRole == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	roleARN := *project.ServiceRole
	roleName := roleARN
	if idx := strings.LastIndex(roleARN, "/"); idx >= 0 && idx < len(roleARN)-1 {
		roleName = roleARN[idx+1:]
	}

	roleList, _, err := cbRelatedResources(ctx, clients, cache, "role")
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

// checkCbLogs searches the logs cache for the CloudWatch log group associated
// with this CodeBuild project.
// Pattern F+N — uses explicit LogsConfig.CloudWatchLogs.GroupName if set,
// otherwise falls back to naming convention: /aws/codebuild/{projectName}.
func checkCbLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	project, ok := assertStruct[cbtypes.Project](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	// Determine expected log group name: explicit config or naming convention.
	expectedLogGroup := "/aws/codebuild/" + res.ID
	if project.LogsConfig != nil &&
		project.LogsConfig.CloudWatchLogs != nil &&
		project.LogsConfig.CloudWatchLogs.GroupName != nil &&
		*project.LogsConfig.CloudWatchLogs.GroupName != "" {
		expectedLogGroup = *project.LogsConfig.CloudWatchLogs.GroupName
	}

	logList, truncated, err := cbRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == expectedLogGroup {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	return relatedResult("logs", ids)
}

// checkCbPipeline returns Count: 0 because CodePipeline PipelineSummary does not
// include stage details — the relationship between a CodeBuild project and its
// containing pipeline cannot be determined from cache alone.
func checkCbPipeline(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "pipeline", Count: 0}
}

// cbRelatedResources returns the resource list for target from cache or by fetching the first page.
func cbRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
