// iam_roles_related.go contains IAM Role related-resource checker functions.
// Role is a reverse-lookup hub: other resources (Lambda, Glue, Node Groups) store
// role ARNs in their RawStruct. The checkers here search those target caches.
package aws

import (
	"context"
	"strings"

	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("role", []resource.RelatedDef{
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkRoleLambda, NeedsTargetCache: true},
		{TargetType: "glue", DisplayName: "Glue Jobs", Checker: checkRoleGlue, NeedsTargetCache: true},
		{TargetType: "ng", DisplayName: "Node Groups", Checker: checkRoleNG, NeedsTargetCache: true},
		{TargetType: "policy", DisplayName: "IAM Policies", Checker: nil, NeedsTargetCache: true},
	})
}

// roleNameFromARN extracts the role name from a role ARN or returns the input as-is
// if it's not an ARN. Works for both:
//
//	"arn:aws:iam::123456789012:role/service-role/my-role" → "my-role"
//	"arn:aws:iam::123456789012:role/my-role" → "my-role"
//	"my-role" → "my-role"
func roleNameFromARN(s string) string {
	if idx := strings.LastIndex(s, "/"); idx >= 0 && idx < len(s)-1 {
		return s[idx+1:]
	}
	return s
}

// checkRoleLambda searches the lambda cache for functions whose Role ARN references
// this IAM role. It resolves both name-segment and full-ARN matches.
func checkRoleLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := res.ID
	roleARN := ""
	if raw, ok := assertStruct[iamtypes.Role](res.RawStruct); ok && raw.Arn != nil {
		roleARN = *raw.Arn
	}

	lambdaList, truncated, err := roleRelatedResources(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if lambdaList == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}

	var ids []string
	for _, lambdaRes := range lambdaList {
		fn, ok := assertStruct[lambdatypes.FunctionConfiguration](lambdaRes.RawStruct)
		if !ok || fn.Role == nil || *fn.Role == "" {
			continue
		}
		fnRoleRef := *fn.Role
		if fnRoleRef == roleName || roleNameFromARN(fnRoleRef) == roleName {
			ids = append(ids, lambdaRes.ID)
			continue
		}
		if roleARN != "" && fnRoleRef == roleARN {
			ids = append(ids, lambdaRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	return relatedResult("lambda", ids)
}

// checkRoleGlue searches the glue cache for jobs whose Role references this IAM role.
// Glue fixtures store a plain name or a full ARN; both are handled via roleNameFromARN.
func checkRoleGlue(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := res.ID

	glueList, truncated, err := roleRelatedResources(ctx, clients, cache, "glue")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "glue", Count: -1, Err: err}
	}
	if glueList == nil {
		return resource.RelatedCheckResult{TargetType: "glue", Count: -1}
	}

	var ids []string
	for _, glueRes := range glueList {
		job, ok := assertStruct[gluetypes.Job](glueRes.RawStruct)
		if !ok || job.Role == nil || *job.Role == "" {
			continue
		}
		jobRole := *job.Role
		if jobRole == roleName || roleNameFromARN(jobRole) == roleName {
			ids = append(ids, glueRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "glue", Count: -1}
	}
	return relatedResult("glue", ids)
}

// checkRoleNG searches the ng (node group) cache for node groups whose NodeRole ARN
// references this IAM role.
func checkRoleNG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := res.ID

	ngList, truncated, err := roleRelatedResources(ctx, clients, cache, "ng")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1, Err: err}
	}
	if ngList == nil {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1}
	}

	var ids []string
	for _, ngRes := range ngList {
		ng, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct)
		if !ok || ng.NodeRole == nil || *ng.NodeRole == "" {
			continue
		}
		nodeRole := *ng.NodeRole
		if nodeRole == roleName || roleNameFromARN(nodeRole) == roleName {
			ids = append(ids, ngRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ng", Count: -1}
	}
	return relatedResult("ng", ids)
}

// roleRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func roleRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
