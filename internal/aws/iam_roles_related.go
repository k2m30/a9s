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
		return resource.ErrorRelated("eks", err)
	}
	if eksList == nil {
		return resource.UnknownRelated("eks")
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
	return relatedResultTrunc("eks", ids, truncated)
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

// roleNameFromARN extracts the role name from a role ARN or returns the input as-is
// if it's not an ARN. Works for role ARNs (last path segment) and STS
// assumed-role ARNs (the role segment, NOT the trailing session name):
//
//	"arn:aws:iam::123456789012:role/service-role/my-role" → "my-role"
//	"arn:aws:iam::123456789012:role/my-role" → "my-role"
//	"arn:aws:sts::123456789012:assumed-role/my-role/session-abc" → "my-role"
//	"my-role" → "my-role"
func roleNameFromARN(s string) string {
	// STS assumed-role ARN: ".../assumed-role/<role>/<session>". CloudTrail
	// records this form on AssumeRole* events; a plain last-segment trim would
	// yield the session name, which is not a GetRole-resolvable role.
	if _, rest, ok := strings.Cut(s, ":assumed-role/"); ok {
		if role, _, ok := strings.Cut(rest, "/"); ok {
			return role
		}
		return rest
	}
	if idx := strings.LastIndex(s, "/"); idx >= 0 && idx < len(s)-1 {
		return s[idx+1:]
	}
	return s
}

// arnAccountID returns the account-id segment (index 4) of an ARN, or "" when
// the string is not a well-formed ARN. ARN layout:
// arn:partition:service:region:account-id:resource.
func arnAccountID(arn string) string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 5 {
		return ""
	}
	return parts[4]
}

// sameAccountRoleNames normalizes IAM role ARNs to role names (== role.ID) and
// keeps only roles owned by ownerAccount. Cross-account role ARNs are dropped:
// iam:GetRole resolves only within the caller's account, so a foreign-account
// role is not fetchable here — counting it would dead-end the drill or
// false-match a same-named local role. A blank ownerAccount keeps every role
// (best effort when the owner cannot be determined). Results are de-duplicated
// by name in first-seen order; relatedResult sorts for final stability.
func sameAccountRoleNames(arns []string, ownerAccount string) []string {
	seen := make(map[string]struct{}, len(arns))
	names := make([]string, 0, len(arns))
	for _, arn := range arns {
		if ownerAccount != "" {
			if acct := arnAccountID(arn); acct != "" && acct != ownerAccount {
				continue
			}
		}
		name := roleNameFromARN(arn)
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
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
		return resource.ErrorRelated("lambda", err)
	}
	if lambdaList == nil {
		return resource.UnknownRelated("lambda")
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
	return relatedResultTrunc("lambda", ids, truncated)
}

// checkRoleGlue searches the glue cache for jobs whose Role references this IAM role.
// Glue fixtures store a plain name or a full ARN; both are handled via roleNameFromARN.
func checkRoleGlue(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := res.ID

	glueList, truncated, err := roleRelatedResources(ctx, clients, cache, "glue")
	if err != nil {
		return resource.ErrorRelated("glue", err)
	}
	if glueList == nil {
		return resource.UnknownRelated("glue")
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
	return relatedResultTrunc("glue", ids, truncated)
}

// checkRoleNG searches the ng (node group) cache for node groups whose NodeRole ARN
// references this IAM role.
func checkRoleNG(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := res.ID

	ngList, truncated, err := roleRelatedResources(ctx, clients, cache, "ng")
	if err != nil {
		return resource.ErrorRelated("ng", err)
	}
	if ngList == nil {
		return resource.UnknownRelated("ng")
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
	return relatedResultTrunc("ng", ids, truncated)
}

// checkRolePolicy uses the IAM ListAttachedRolePolicies API to return the
// managed policies attached to this IAM role.
func checkRolePolicy(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("policy")
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
		return resource.ErrorRelated("policy", err)
	}
	ids := attachedPolicyNames(out.AttachedPolicies)
	return relatedResult("policy", ids)
}

// checkRoleEC2 scans the EC2 instance cache for instances whose IamInstanceProfile
// ARN last segment (the profile name) equals this role's name — the common
// one-profile-per-role convention (docs/resources/role.md §2 ec2). This is an
// exact-boundary match, not a substring match: "my-role" must not match a
// profile named "my-role-2". Profiles whose name differs from the role name
// (EKS/ASG-generated profiles) are not resolved here — that would require a
// per-profile iam:GetInstanceProfile fan-out across all distinct profile ARNs
// in the ec2 cache, which is out of the zero/one-call budget this checker is
// scoped to; see checkEC2Role (ec2_related.go) for the reverse direction,
// which resolves the same ambiguity per-instance with a bounded single call.
// Pattern C: cache scan with exact-name approximation.
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
		return resource.ErrorRelated("ec2", err)
	}
	if ec2List == nil {
		return resource.UnknownRelated("ec2")
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
		if roleNameFromARN(profileARN) == roleName {
			ids = append(ids, ec2Res.ID)
		}
	}
	return relatedResultTrunc("ec2", ids, truncated)
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
