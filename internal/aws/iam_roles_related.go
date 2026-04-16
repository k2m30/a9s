// iam_roles_related.go contains IAM Role related-resource checker functions.
// Role is a reverse-lookup hub: other resources (Lambda, Glue, Node Groups) store
// role ARNs in their RawStruct. The checkers here search those target caches.
package aws

import (
	"context"
	"encoding/json"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("role", []resource.RelatedDef{
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkRoleLambda, NeedsTargetCache: true},
		{TargetType: "glue", DisplayName: "Glue Jobs", Checker: checkRoleGlue, NeedsTargetCache: true},
		{TargetType: "ng", DisplayName: "Node Groups", Checker: checkRoleNG, NeedsTargetCache: true},
		{TargetType: "policy", DisplayName: "IAM Policies", Checker: checkRolePolicy, NeedsTargetCache: false},
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkRoleEC2, NeedsTargetCache: true},
		{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkRoleEKS, NeedsTargetCache: true},
		{TargetType: "iam-group", DisplayName: "IAM Groups (via AssumeRolePolicy)", Checker: checkRoleIamGroup, NeedsTargetCache: false},
		{TargetType: "iam-user", DisplayName: "IAM Users (via AssumeRolePolicy)", Checker: checkRoleIamUser, NeedsTargetCache: false},
		{TargetType: "kms", DisplayName: "KMS Keys (policies grant)", Checker: checkRoleKMS, NeedsTargetCache: false},
	})
}

// checkRoleEKS scans the eks cluster cache for clusters whose RoleArn matches this
// role's ARN or name. Pattern C.
func checkRoleEKS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := res.ID
	roleARN := ""
	if raw, ok := assertStruct[iamtypes.Role](res.RawStruct); ok && raw.Arn != nil {
		roleARN = *raw.Arn
	}

	eksList, truncated, err := roleRelatedResources(ctx, clients, cache, "eks")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1, Err: err}
	}
	if eksList == nil {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1}
	}

	var ids []string
	for _, e := range eksList {
		cluster, ok := assertStruct[ekstypes.Cluster](e.RawStruct)
		if !ok || cluster.RoleArn == nil {
			continue
		}
		arn := *cluster.RoleArn
		if (roleARN != "" && arn == roleARN) || (roleName != "" && roleNameFromARN(arn) == roleName) {
			ids = append(ids, e.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "eks", Count: -1}
	}
	return relatedResult("eks", ids)
}

// checkRoleIamGroup extracts IAM group names from this role's AssumeRolePolicy (trust
// policy) by scanning Principal.AWS ARN entries matching ":group/". The trust
// document is already fetched + URL-decoded by the role fetcher and lives in
// Fields["assume_role_policy_document"] — 0 API calls, offline parse.
func checkRoleIamGroup(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	doc := res.Fields["assume_role_policy_document"]
	if doc == "" {
		return resource.RelatedCheckResult{TargetType: "iam-group", Count: 0}
	}
	seen := map[string]struct{}{}
	extractPrincipalsByKind([]byte(doc), ":group/", seen)
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	return relatedResult("iam-group", names)
}

// checkRoleIamUser extracts IAM user names from this role's AssumeRolePolicy trust
// policy by scanning Principal.AWS ARN entries matching ":user/". 0-call path.
func checkRoleIamUser(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	doc := res.Fields["assume_role_policy_document"]
	if doc == "" {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: 0}
	}
	seen := map[string]struct{}{}
	extractPrincipalsByKind([]byte(doc), ":user/", seen)
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	return relatedResult("iam-user", names)
}

// extractPrincipalsByKind walks a JSON IAM policy document and records the
// last-segment name (after the final "/") from Principal.AWS ARN entries whose
// string contains the given kindMarker (e.g. ":group/", ":user/", ":role/").
func extractPrincipalsByKind(doc []byte, kindMarker string, seen map[string]struct{}) {
	var raw any
	if err := json.Unmarshal(doc, &raw); err != nil {
		return
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, val := range x {
				if k == "AWS" {
					addKindedPrincipal(val, kindMarker, seen)
				}
				walk(val)
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(raw)
}

func addKindedPrincipal(v any, kindMarker string, seen map[string]struct{}) {
	switch x := v.(type) {
	case string:
		if strings.Contains(x, kindMarker) {
			seen[arnRoleName(x)] = struct{}{}
		}
	case []any:
		for _, it := range x {
			if s, ok := it.(string); ok && strings.Contains(s, kindMarker) {
				seen[arnRoleName(s)] = struct{}{}
			}
		}
	}
}

// checkRoleKMS returns Count: -1. KMS keys whose key policy or grants reference this
// role are only discoverable by enumerating KMS keys and inspecting GetKeyPolicy /
// ListGrants per key (N+1 per key).
func checkRoleKMS(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
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

// checkRolePolicy uses the IAM ListAttachedRolePolicies API to return the
// managed policies attached to this IAM role.
func checkRolePolicy(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "policy", Count: -1}
	}
	roleName := res.ID
	if roleName == "" {
		raw, ok2 := assertStruct[iamtypes.Role](res.RawStruct)
		if ok2 && raw.RoleName != nil {
			roleName = *raw.RoleName
		}
	}
	if roleName == "" {
		return resource.RelatedCheckResult{TargetType: "policy", Count: 0}
	}
	out, err := c.IAM.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: &roleName,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "policy", Count: -1, Err: err}
	}
	ids := customerManagedAttachedPolicyNames(out.AttachedPolicies)
	return relatedResult("policy", ids)
}

// checkRoleEC2 scans the EC2 instance cache for instances whose IamInstanceProfile
// ARN contains this role's name. Instance profiles often share the role name.
// Pattern C: cache scan with ARN-contains approximation.
func checkRoleEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := res.ID
	if roleName == "" {
		if raw, ok := assertStruct[iamtypes.Role](res.RawStruct); ok && raw.RoleName != nil {
			roleName = *raw.RoleName
		}
	}
	if roleName == "" {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: 0}
	}

	ec2List, truncated, err := roleRelatedResources(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1, Err: err}
	}
	if ec2List == nil {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}

	var ids []string
	for _, ec2Res := range ec2List {
		inst, ok := assertStruct[ec2types.Instance](ec2Res.RawStruct)
		if !ok {
			continue
		}
		if inst.IamInstanceProfile == nil || inst.IamInstanceProfile.Arn == nil {
			continue
		}
		profileARN := *inst.IamInstanceProfile.Arn
		// Instance profile ARN contains the profile name, which commonly matches
		// the role name (e.g. arn:aws:iam::123:instance-profile/my-role).
		if strings.Contains(profileARN, "/"+roleName) {
			ids = append(ids, ec2Res.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ec2", Count: -1}
	}
	return relatedResult("ec2", ids)
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



