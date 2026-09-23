// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_roles_related.go contains IAM Role related-resource checker functions.
// Role is a reverse-lookup hub: other resources (Lambda, Glue, Node Groups) store
// role ARNs in their RawStruct. The checkers here search those target caches.
package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
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
// role's ARN or name.
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

// checkRoleIamGroup lists the IAM groups this role's trust policy grants.
// The trust document is already fetched and URL-decoded by the role fetcher
// and lives in Fields["assume_role_policy_document"]: 0 API calls.
func checkRoleIamGroup(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return roleTrustPrincipals(clients, res, cache, "iam-group", "group/")
}

// checkRoleIamUser lists the IAM users this role's trust policy grants.
// 0-call path.
func checkRoleIamUser(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return roleTrustPrincipals(clients, res, cache, "iam-user", "user/")
}

// roleTrustPrincipals resolves the principals of kind the role's trust
// policy grants to target rows. A trust policy that does not parse leaves
// the pivot unknown.
func roleTrustPrincipals(clients any, res resource.Resource, cache resource.ResourceCache, target, kind string) resource.RelatedCheckResult {
	doc := res.Fields["assume_role_policy_document"]
	if doc == "" {
		return resource.ProvenZero(target, "assume_role_policy_document")
	}
	roleARN := ""
	if raw, ok := assertStruct[iamtypes.Role](res.RawStruct); ok {
		roleARN = aws.ToString(raw.Arn)
	}
	rc := policyRefContext(clients, cache, target, roleARN)
	refs, ok := grantedPrincipalRefs(doc, kind)
	if !ok {
		return resource.UnknownRelated(target)
	}
	return relatedRefs(target, refs, rc)
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
		return resource.ProvenZero("policy", "roleName")
	}
	attached, complete, err := listAttachedRolePolicies(ctx, c.IAM, roleName)
	if err != nil {
		return resource.ErrorRelated("policy", err)
	}
	return relatedResultTrunc("policy", attachedPolicyIDs(attached), !complete)
}

// checkRoleEC2 lists the instance profiles that hold this role
// (iam:ListInstanceProfilesForRole) and counts the instances in the ec2 list
// running under one of them. A profile's name says nothing about its role.
func checkRoleEC2(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	roleName := res.ID
	if roleName == "" {
		if raw, ok := assertStruct[iamtypes.Role](res.RawStruct); ok && raw.RoleName != nil {
			roleName = *raw.RoleName
		}
	}
	if roleName == "" {
		return resource.ProvenZero("ec2", "roleName")
	}

	ec2List, truncated, err := relatedResourcesFor(ctx, clients, cache, "ec2")
	if err != nil {
		return resource.ErrorRelated("ec2", err)
	}
	if ec2List == nil {
		return resource.UnknownRelated("ec2")
	}
	profileOf := make(map[string]string, len(ec2List))
	for _, ec2Res := range ec2List {
		if inst, ok := assertStruct[ec2types.Instance](ec2Res.RawStruct); ok && inst.IamInstanceProfile != nil && aws.ToString(inst.IamInstanceProfile.Arn) != "" {
			profileOf[ec2Res.ID] = *inst.IamInstanceProfile.Arn
		}
	}
	if len(profileOf) == 0 {
		return relatedResultTrunc("ec2", nil, truncated)
	}

	c, err := svcClients(clients)
	// no finding: without the IAM client nothing was read.
	if err != nil || c.IAM == nil {
		return resource.UnknownRelated("ec2")
	}
	api, ok := c.IAM.(IAMListInstanceProfilesForRoleAPI)
	if !ok {
		return resource.UnknownRelated("ec2")
	}
	profiles, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.InstanceProfile, *string, error) {
		out, callErr := api.ListInstanceProfilesForRole(ctx, &iam.ListInstanceProfilesForRoleInput{RoleName: aws.String(roleName), Marker: marker})
		if callErr != nil {
			return nil, nil, callErr
		}
		return out.InstanceProfiles, out.Marker, nil
	})
	if err != nil {
		return resource.ErrorRelated("ec2", err)
	}
	var ids []string
	for _, ec2Res := range ec2List {
		if slices.ContainsFunc(profiles, func(p iamtypes.InstanceProfile) bool { return aws.ToString(p.Arn) == profileOf[ec2Res.ID] }) {
			ids = append(ids, ec2Res.ID)
		}
	}
	return relatedResultTrunc("ec2", ids, truncated || !complete)
}
