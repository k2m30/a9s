// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// checkRoleEKS scans the eks cluster cache for clusters whose RoleArn matches this
// role's ARN or name. Pattern C.
func checkRoleEKS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := res.ID
	roleARN := ""
	if raw, ok := assertStruct[iamtypes.Role](res.RawStruct); ok && raw.Arn != nil {
		roleARN = *raw.Arn
	}

	eksList, truncated, err := relatedResourcesFor(ctx, clients, cache, "eks")
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
func checkRoleIamGroup(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	doc := res.Fields["assume_role_policy_document"]
	if doc == "" {
		return resource.KnownRelated("iam-group", nil, false)
	}
	var arns []string
	extractPrincipalsByKind([]byte(doc), ":group/", &arns)
	return relatedRefs("iam-group", arns, refContext(clients, cache, "iam-group"))
}

// checkRoleIamUser extracts IAM user names from this role's AssumeRolePolicy trust
// policy by scanning Principal.AWS ARN entries matching ":user/". 0-call path.
func checkRoleIamUser(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	doc := res.Fields["assume_role_policy_document"]
	if doc == "" {
		return resource.KnownRelated("iam-user", nil, false)
	}
	var arns []string
	extractPrincipalsByKind([]byte(doc), ":user/", &arns)
	return relatedRefs("iam-user", arns, refContext(clients, cache, "iam-user"))
}

// extractPrincipalsByKind walks a JSON IAM policy document and appends to arns
// the Principal.AWS ARN entries whose string contains the given kindMarker
// (e.g. ":group/", ":user/", ":role/").
func extractPrincipalsByKind(doc []byte, kindMarker string, arns *[]string) {
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
					addKindedPrincipal(val, kindMarker, arns)
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

func addKindedPrincipal(v any, kindMarker string, arns *[]string) {
	switch x := v.(type) {
	case string:
		if strings.Contains(x, kindMarker) {
			*arns = append(*arns, x)
		}
	case []any:
		for _, it := range x {
			if s, ok := it.(string); ok && strings.Contains(s, kindMarker) {
				*arns = append(*arns, s)
			}
		}
	}
}

// roleNameFromARN is the role a role reference names (role.ID), or the
// reference itself when it names none.
func roleNameFromARN(s string) string {
	if id, ok := resource.ResolveRef("role", s, domain.RefContext{}); ok {
		return id
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

	lambdaList, truncated, err := relatedResourcesFor(ctx, clients, cache, "lambda")
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

	glueList, truncated, err := relatedResourcesFor(ctx, clients, cache, "glue")
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

	ngList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ng")
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
		return resource.KnownRelated("policy", nil, false)
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
		return resource.KnownRelated("ec2", nil, false)
	}

	ec2List, truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
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
		if instanceProfileName(*inst.IamInstanceProfile.Arn) == roleName {
			ids = append(ids, ec2Res.ID)
		}
	}
	return relatedResultTrunc("ec2", ids, truncated)
}
