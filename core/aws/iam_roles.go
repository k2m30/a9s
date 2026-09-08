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

// role trust-policy FindingCodes and their S5 detail sentences.
const (
	// roleCodeWildcardTrust — the trust policy lets any AWS principal assume
	// the role with nothing scoping the grant.
	roleCodeWildcardTrust domain.FindingCode = "role.trust.wildcard-principal"
	// roleCodeConfusedDeputy — an AWS service can assume the role without an
	// aws:SourceAccount / aws:SourceArn / aws:SourceOrgID condition.
	roleCodeConfusedDeputy domain.FindingCode = "role.trust.confused-deputy"
	// roleCodeInlinePrivEsc — an inline policy grants an action set that adds
	// up to full administrator.
	roleCodeInlinePrivEsc domain.FindingCode = "role.inline-privilege-escalation"

	// uninspectedPolicyResources is what the policy_resources Field holds
	// when the role's inline policies could not be read. Empty would read as
	// "this role's policies name no resources", which is a claim a9s did not
	// earn; "?" is the same unknown mark every other surface uses.
	uninspectedPolicyResources = "?"

	// awsServiceRolePathPrefix marks an AWS service-linked role. AWS owns the
	// trust policy and the attached permissions, so posture findings about
	// either are not actionable by the operator.
	awsServiceRolePathPrefix = "/aws-service-role/"
)

// roleTrustAnalysis is one parse of a role's trust policy: the Findings it
// yields, their supporting Attention rows, and the trust_wildcard /
// trust_summary display fields. The Findings list, the Trust column and the
// row color all read this single verdict.
type roleTrustAnalysis struct {
	findings []domain.Finding
	details  map[domain.FindingCode]domain.AttentionDetail
	wildcard string
	summary  string
}

func analyseRoleTrust(assumeRolePolicyDoc, path string) roleTrustAnalysis {
	out := roleTrustAnalysis{wildcard: "false"}
	doc, err := iampolicy.Parse(assumeRolePolicyDoc)
	if err != nil {
		return out
	}
	if iampolicy.EvaluateTrust(doc, "").Public {
		// The summary is a9s's own verdict on the trust policy, not a value
		// IAM returned, so it is written in the words the Trust column and the
		// detail row render rather than in the shape of an SDK constant.
		out.wildcard, out.summary = "true", "wildcard"
		out.findings = append(out.findings, wave1Finding(roleCodeWildcardTrust))
	}
	if svcs := unscopedServicePrincipals(doc, path); len(svcs) > 0 {
		out.findings = append(out.findings, wave1Finding(roleCodeConfusedDeputy))
		out.details = map[domain.FindingCode]domain.AttentionDetail{
			roleCodeConfusedDeputy: {Rows: []domain.DetailRow{
				{Label: "Services", Value: strings.Join(svcs, ", "), Tier: "~"},
			}},
		}
	}
	return out
}

// unscopedServicePrincipals returns the confused-deputy service principals
// worth reporting on a role's trust policy. AWS service-linked roles are
// excluded (AWS owns the document), and so is a role trusted only by
// ec2.amazonaws.com: an EC2 instance profile is assumed by the instance
// itself, so there is no third-party caller to scope.
func unscopedServicePrincipals(doc iampolicy.Document, path string) []string {
	if strings.HasPrefix(path, awsServiceRolePathPrefix) {
		return nil
	}
	svcs := doc.HasServicePrincipalWithoutSourceScope()
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

	listPoliciesAPI, _ := api.(IAMListRolePoliciesAPI)
	getPolicyAPI, _ := api.(IAMGetRolePolicyAPI)

	var resources []resource.Resource
	var failures []Failure
	for _, role := range output.Roles {
		roleName := ""
		if role.RoleName != nil {
			roleName = *role.RoleName
		}

		roleID := ""
		if role.RoleId != nil {
			roleID = *role.RoleId
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

		trust := analyseRoleTrust(assumeRolePolicyDoc, path)
		findings, details := trust.findings, trust.details

		policyResources := ""
		if listPoliciesAPI != nil && getPolicyAPI != nil && roleName != "" {
			var inline inlinePolicyScan
			policyResources, inline = enumerateRoleInlinePolicies(ctx, listPoliciesAPI, getPolicyAPI, roleName)
			findings, details = addInlinePrivEsc(findings, details, inline)
			failures = append(failures, inline.failures...)
		}

		r := resource.Resource{
			ID:   roleName,
			Name: roleName,
			Type: "role",
			Fields: map[string]string{
				"role_name":                   roleName,
				"role_id":                     roleID,
				"path":                        path,
				"create_date":                 createDate,
				"description":                 description,
				"assume_role_policy_document": assumeRolePolicyDoc,
				"trust_wildcard":              trust.wildcard,
				"trust_summary":               trust.summary,
				"policy_resources":            policyResources,
			},
			Findings:         findings,
			AttentionDetails: details,
			RawStruct:        role,
		}

		resources = append(resources, r)
	}

	// Build pagination metadata — IAM uses IsTruncated bool + Marker *string
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
	}, AggregateFailures("role: inline policy scan", failures, len(output.Roles))
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
				err = fmt.Errorf("empty response")
			}
			failures = append(failures, FailedCall(id, err))
			continue
		}
		r, roleFailures := roleToResource(ctx, api, *output.Role)
		resources = append(resources, r)
		failures = append(failures, roleFailures...)
	}
	return resources, AggregateFailures("role FetchByIDs", failures, len(ids))
}

// roleToResource converts a single iam.Role (as returned by GetRole) into the
// same resource.Resource shape FetchIAMRolesPage produces from ListRoles, so
// a lazily-fetched role is indistinguishable from a paginated one — including
// its inline policies, which GetRole does not return and which api is walked
// for when it serves the two inline-policy operations. The returned failures
// are that walk's own: the caller aggregates them onto its lane, because a
// role whose policies were refused must not be handed back as inspected.
func roleToResource(ctx context.Context, api any, role iamtypes.Role) (resource.Resource, []Failure) {
	roleName := ""
	if role.RoleName != nil {
		roleName = *role.RoleName
	}

	roleID := ""
	if role.RoleId != nil {
		roleID = *role.RoleId
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

	trust := analyseRoleTrust(assumeRolePolicyDoc, path)
	findings, details := trust.findings, trust.details

	listPoliciesAPI, okList := api.(IAMListRolePoliciesAPI)
	getPolicyAPI, okGet := api.(IAMGetRolePolicyAPI)
	policyResources := ""
	var failures []Failure
	if okList && okGet && roleName != "" {
		var inline inlinePolicyScan
		policyResources, inline = enumerateRoleInlinePolicies(ctx, listPoliciesAPI, getPolicyAPI, roleName)
		findings, details = addInlinePrivEsc(findings, details, inline)
		failures = inline.failures
	}

	return resource.Resource{
		ID:   roleName,
		Name: roleName,
		Type: "role",
		Fields: map[string]string{
			"role_name":                   roleName,
			"role_id":                     roleID,
			"path":                        path,
			"create_date":                 createDate,
			"description":                 description,
			"assume_role_policy_document": assumeRolePolicyDoc,
			"trust_wildcard":              trust.wildcard,
			"trust_summary":               trust.summary,
			"policy_resources":            policyResources,
		},
		Findings:         findings,
		AttentionDetails: details,
		RawStruct:        role,
	}, failures
}

// inlinePolicyScan carries the privilege-escalation verdict of a role's
// inline policies: at most one finding, whichever inline policy tripped
// first, with the offending policy name and combination as its rows.
//
// failures is what the scan could not read. A non-empty failures means the
// verdict is not a verdict: the absence of a finding says nothing about the
// role, so the caller must surface the failures on its own lane and render
// the role's policy-derived facts unknown rather than clean.
type inlinePolicyScan struct {
	finding  *domain.Finding
	rows     []domain.DetailRow
	failures []Failure
}

// enumerateRoleInlinePolicies walks a role's inline policies once and
// returns both a comma-separated list of every Statement[].Resource entry
// across all documents and the privilege-escalation verdict.
//
// policy_resources is emitted as a Field so sibling pivots (s3, kms,
// secrets, …) can scan the list and match by ARN substring.
// Cost: 1 ListRolePolicies + N GetRolePolicy per role.
// Attached (managed) policies require a separate walk via
// ListAttachedRolePolicies + GetPolicyVersion and are not enumerated here
// yet; inline policies cover the s3-access-role case and are the minimum
// needed to make the s3→role pivot resolve.
func enumerateRoleInlinePolicies(
	ctx context.Context,
	listAPI IAMListRolePoliciesAPI,
	getAPI IAMGetRolePolicyAPI,
	roleName string,
) (string, inlinePolicyScan) {
	var scan inlinePolicyScan
	listOut, err := listAPI.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	switch {
	case err != nil:
		scan.failures = append(scan.failures, FailedCall(roleName, err))
		return uninspectedPolicyResources, scan
	case listOut == nil:
		scan.failures = append(scan.failures, UnusableAnswer(roleName, "ListRolePolicies returned no answer"))
		return uninspectedPolicyResources, scan
	}
	var allResources []string
	for _, policyName := range listOut.PolicyNames {
		getOut, getErr := getAPI.GetRolePolicy(ctx, &iam.GetRolePolicyInput{
			RoleName:   aws.String(roleName),
			PolicyName: aws.String(policyName),
		})
		switch {
		case getErr != nil:
			scan.failures = append(scan.failures, FailedCall(roleName, getErr))
			continue
		case getOut == nil || getOut.PolicyDocument == nil:
			scan.failures = append(scan.failures,
				UnusableAnswer(roleName, "GetRolePolicy returned no document for "+policyName))
			continue
		}
		parsed, perr := iampolicy.Parse(*getOut.PolicyDocument)
		if perr != nil {
			scan.failures = append(scan.failures, UnusableAnswer(roleName, "unreadable inline policy "+policyName))
			continue
		}
		// Read off the document the escalation check below reads. A second
		// walk of the same JSON is a second answer waiting to disagree — the
		// one this replaced bound Statement to an array and so lost every
		// resource of a policy written with a bare Statement object.
		for _, st := range parsed.Statement {
			allResources = append(allResources, st.Resource...)
		}
		if scan.finding != nil {
			continue
		}
		if combos := parsed.PrivilegeEscalation(); len(combos) > 0 {
			f := wave1Finding(roleCodeInlinePrivEsc)
			scan.finding = &f
			scan.rows = append(
				[]domain.DetailRow{{Label: "Policy", Value: policyName, Tier: "!"}},
				privEscComboRows(combos)...)
		}
	}
	if len(scan.failures) > 0 {
		// A list assembled from the documents that did answer reads as the
		// role's whole policy surface. It is not one.
		return uninspectedPolicyResources, scan
	}
	return strings.Join(allResources, ","), scan
}

// addInlinePrivEsc folds an inline-policy scan into the finding list and
// detail map a role is about to be built from. The rows go through capRows,
// the same bound setWave2Finding and addWave1Rows apply, because a role's
// combo list is as unbounded as any other emitter's: the role is assembled
// before its resource.Resource exists, so this is the sink for that path.
func addInlinePrivEsc(
	findings []domain.Finding,
	details map[domain.FindingCode]domain.AttentionDetail,
	inline inlinePolicyScan,
) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	if inline.finding == nil {
		return findings, details
	}
	findings = append(findings, *inline.finding)
	if details == nil {
		details = map[domain.FindingCode]domain.AttentionDetail{}
	}
	details[roleCodeInlinePrivEsc] = domain.AttentionDetail{Rows: capRows(nil, inline.rows)}
	return findings, details
}
