// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// role trust-policy FindingCodes and their detail sentences.
const (
	// roleCodeWildcardTrust — the trust policy lets any AWS principal assume
	// the role with nothing scoping the grant.
	roleCodeWildcardTrust domain.FindingCode = "role.trust.wildcard-principal"
	roleCodeTrustUnread   domain.FindingCode = "role.trust.policy-not-inspected"
	// roleCodeConfusedDeputy — an AWS service can assume the role without an
	// aws:SourceAccount / aws:SourceArn / aws:SourceOrgID condition.
	roleCodeConfusedDeputy domain.FindingCode = "role.trust.confused-deputy"
	// roleCodeInlinePrivEsc — an inline policy grants an action set that adds
	// up to full administrator.
	roleCodeInlinePrivEsc domain.FindingCode = "role.inline-privilege-escalation"

	// awsServiceRolePathPrefix marks an AWS service-linked role. AWS owns the
	// trust policy and the attached permissions, so posture findings about
	// either are not actionable by the operator.
	awsServiceRolePathPrefix = "/aws-service-role/"
)

// The two kinds of IAM role: one AWS creates and owns for a service, and
// every other.
const (
	roleKindServiceLinked = "service-linked"
	roleKindStandard      = "standard"
)

var roleKinds = []string{roleKindServiceLinked, roleKindStandard} //nolint:gochecknoglobals // static value set

// roleKind reads the role's Path. A row rebuilt from Fields alone reads the
// path the fetcher stored.
func roleKind(r domain.Resource) string {
	path := r.Fields["path"]
	if role, ok := assertStruct[iamtypes.Role](r.RawStruct); ok {
		path = aws.ToString(role.Path)
	}
	if strings.HasPrefix(path, awsServiceRolePathPrefix) {
		return roleKindServiceLinked
	}
	return roleKindStandard
}

// roleTrustAnalysis is one parse of a role's trust policy: the Findings it
// yields, their supporting Attention rows, and the trust_summary display
// field. The Findings list, the Trust column and the row color all read this
// single verdict.
type roleTrustAnalysis struct {
	findings []domain.Finding
	details  map[domain.FindingCode]domain.AttentionDetail
	summary  string
}

func analyseRoleTrust(assumeRolePolicyDoc string) roleTrustAnalysis {
	trust := readPolicy("role", assumeRolePolicyDoc, "", "")
	// The summary is a9s's own verdict on the trust policy, not a value IAM
	// returned, so it is written in the words the Trust column and the detail
	// row render rather than in the shape of an SDK constant.
	out := roleTrustAnalysis{summary: trust.word("wildcard", "")}
	out.details = map[domain.FindingCode]domain.AttentionDetail{}
	if trust.public() {
		out.findings = append(out.findings, wave1Finding(roleCodeWildcardTrust))
		out.details[roleCodeWildcardTrust] = trust.detail()
	}
	if svcs := unscopedServicePrincipals(trust.servicePrincipalsWithoutSourceScope()); len(svcs) > 0 {
		out.findings = append(out.findings, wave1Finding(roleCodeConfusedDeputy))
		out.details[roleCodeConfusedDeputy] = domain.AttentionDetail{Rows: []domain.DetailRow{
			{Label: "Services", Value: strings.Join(svcs, ", "), Tier: "~"},
		}}
	}
	out.findings = trust.withUnread(out.findings)
	if len(out.details) == 0 {
		out.details = nil
	}
	return out
}

// unscopedServicePrincipals returns the confused-deputy service principals
// worth reporting on a role's trust policy. A role trusted only by
// ec2.amazonaws.com is excluded: an EC2 instance profile is assumed by the
// instance itself, so there is no third-party caller to scope.
func unscopedServicePrincipals(svcs []string) []string {
	if len(svcs) == 1 && svcs[0] == "ec2" {
		return nil
	}
	return svcs
}

// FetchIAMRolesPage calls the IAM ListRoles API and returns a single page
// of roles. Pass an empty continuationToken for the first page.
func FetchIAMRolesPage(ctx context.Context, api IAMListRolesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &iam.ListRolesInput{
		MaxItems: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.ListRoles(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching IAM roles: %w", err)
	}

	var resources []resource.Resource
	for _, role := range output.Roles {
		resources = append(resources, roleToResource(role))
	}

	nextToken := ""
	isTruncated := output.IsTruncated
	if isTruncated && output.Marker != nil {
		nextToken = *output.Marker
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// FetchRolesByIDs resolves a batch of IAM role names to resource.Resource
// values via one GetRole call per id. Each id is the bare RoleName (no path)
// — the same shape FetchIAMRolesPage produces as Resource.ID/Name, and the
// shape callers extract from a role ARN via arnLastSlashSegment even when the
// ARN carries an IAM path such as ".../role/service-role/<name>". GetRole
// itself only ever takes the bare RoleName, so no path-stripping is needed
// here.
//
// Mirrors the resilience contract of FetchIAMPoliciesByIDsFull: per-id
// failures are collected and returned as a composite error via
// AggregateFailures, while the resources that did resolve are still returned
// so the caller gets partial success rather than an all-or-nothing failure.
// A role that no longer exists is not one of those failures — it is the
// deleted-resource race IsNotFoundErr classifies, so it is dropped from the
// answer and kept out of the aggregate.
func FetchRolesByIDs(ctx context.Context, api IAMGetRoleAPI, ids []string) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var failures []Failure
	resources := make([]resource.Resource, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}

		output, err := api.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(id)})
		if IsNotFoundErr(err) {
			// The role went away between the list call and this one: an
			// operational race, not a failure to report. See IsNotFoundErr.
			continue
		}
		if err != nil || output == nil || output.Role == nil {
			if err == nil {
				err = UnusableAnswerErr{Call: "GetRole", Field: "role"}
			}
			failures = append(failures, FailedCall(id, err))
			continue
		}
		resources = append(resources, roleToResource(*output.Role))
	}
	return resources, AggregateFailures("role FetchByIDs", failures, len(seen))
}

// roleToResource converts a single iam.Role (as returned by GetRole) into the
// same resource.Resource shape FetchIAMRolesPage produces from ListRoles, so
// a lazily-fetched role is indistinguishable from a paginated one.
func roleToResource(role iamtypes.Role) resource.Resource {
	roleName := ""
	if role.RoleName != nil {
		roleName = *role.RoleName
	}

	roleID := ""
	if role.RoleId != nil {
		roleID = *role.RoleId
	}

	roleARN := ""
	if role.Arn != nil {
		roleARN = *role.Arn
	}

	path := ""
	if role.Path != nil {
		path = *role.Path
	}

	createDate := ""
	if role.CreateDate != nil {
		createDate = role.CreateDate.Format("2006-01-02 15:04")
	}

	description := ""
	if role.Description != nil {
		description = *role.Description
	}

	assumeRolePolicyDoc := ""
	if role.AssumeRolePolicyDocument != nil {
		assumeRolePolicyDoc = iampolicy.Decode(*role.AssumeRolePolicyDocument)
	}

	trust := analyseRoleTrust(assumeRolePolicyDoc)
	findings, details := trust.findings, trust.details

	return withWave1(resource.Resource{
		ID:   roleName,
		Name: roleName,
		Fields: map[string]string{
			"role_name":                   roleName,
			"role_id":                     roleID,
			"arn":                         roleARN,
			"path":                        path,
			"create_date":                 createDate,
			"description":                 description,
			"assume_role_policy_document": assumeRolePolicyDoc,
			"trust_summary":               trust.summary,
		},
		AttentionDetails: details,
		RawStruct:        role,
	}, findings...)
}
