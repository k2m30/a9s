// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// roleCodeWildcardTrust is the canonical FindingCode for a role whose trust
// policy allows any AWS principal ("*") to assume it without a mitigating
// sts:ExternalId condition.
const roleCodeWildcardTrust domain.FindingCode = "role.trust.wildcard-principal"

// roleWildcardTrustFindings mirrors colorRole's own wildcard-principal
// detection (catalog_security.go) so the Findings list and the row color
// never disagree. Checks both the nested-object form ({"AWS":"*"}, caught by
// trustWildcard=="true" from parseTrustWildcard) and the bare-string form
// ("Principal":"*", which parseTrustWildcard's typed struct cannot
// unmarshal since Principal is not an object in that shape) — the same two
// shapes colorRole's substring check covers.
func roleWildcardTrustFindings(trustWildcard, assumeRolePolicyDoc string) []domain.Finding {
	if trustWildcard == "true" ||
		strings.Contains(assumeRolePolicyDoc, `"Principal":"*"`) ||
		strings.Contains(assumeRolePolicyDoc, `"Principal": "*"`) {
		return []domain.Finding{{
			Code: roleCodeWildcardTrust, Phrase: "anyone can assume this role",
			Severity: domain.SevBroken, Source: "wave1",
		}}
	}
	return nil
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
			decoded, err := url.QueryUnescape(*role.AssumeRolePolicyDocument)
			if err == nil {
				assumeRolePolicyDoc = decoded
			} else {
				assumeRolePolicyDoc = *role.AssumeRolePolicyDocument
			}
		}

		// Detect wildcard principal in trust policy.
		trustWildcard, trustSummary := parseTrustWildcard(assumeRolePolicyDoc)

		policyResources := ""
		if listPoliciesAPI != nil && getPolicyAPI != nil && roleName != "" {
			policyResources = enumerateRoleInlinePolicyResources(ctx, listPoliciesAPI, getPolicyAPI, roleName)
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
				"trust_wildcard":              trustWildcard,
				"trust_summary":               trustSummary,
				"policy_resources":            policyResources,
			},
			Findings:  roleWildcardTrustFindings(trustWildcard, assumeRolePolicyDoc),
			RawStruct: role,
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
// failures (e.g. NoSuchEntity) are collected and returned as a composite
// error via AggregateFailures, while the resources that did resolve are
// still returned so the caller gets partial success rather than an
// all-or-nothing failure.
func FetchRolesByIDs(ctx context.Context, api IAMGetRoleAPI, ids []string) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var failures []string
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
		if err != nil || output == nil || output.Role == nil {
			if err == nil {
				err = fmt.Errorf("empty response")
			}
			failures = append(failures, fmt.Sprintf("%s: %s", id, err.Error()))
			continue
		}
		resources = append(resources, roleToResource(*output.Role))
	}
	return resources, AggregateFailures("role FetchByIDs", failures, len(ids))
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
		decoded, err := url.QueryUnescape(*role.AssumeRolePolicyDocument)
		if err == nil {
			assumeRolePolicyDoc = decoded
		} else {
			assumeRolePolicyDoc = *role.AssumeRolePolicyDocument
		}
	}

	trustWildcard, trustSummary := parseTrustWildcard(assumeRolePolicyDoc)

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
			"trust_wildcard":              trustWildcard,
			"trust_summary":               trustSummary,
			// GetRole does not return inline-policy resources; leaving this
			// empty on a lazily-fetched role (vs. a ListRoles page fetch)
			// mirrors how other by-ID fetchers omit list-only enrichment
			// fields rather than pay for it on every single-ID drill.
			"policy_resources": "",
		},
		Findings:  roleWildcardTrustFindings(trustWildcard, assumeRolePolicyDoc),
		RawStruct: role,
	}
}

// enumerateRoleInlinePolicyResources walks a role's inline policies and
// returns a comma-separated list of every Statement[].Resource entry across
// all documents. Emitted as Fields["policy_resources"] so sibling pivots
// (s3, kms, secrets, …) can scan the list and match by ARN substring.
// Cost: 1 ListRolePolicies + N GetRolePolicy per role.
// Attached (managed) policies require a separate walk via
// ListAttachedRolePolicies + GetPolicyVersion and are not enumerated here
// yet; inline policies cover the s3-access-role case and are the minimum
// needed to make the s3→role pivot resolve.
func enumerateRoleInlinePolicyResources(
	ctx context.Context,
	listAPI IAMListRolePoliciesAPI,
	getAPI IAMGetRolePolicyAPI,
	roleName string,
) string {
	listOut, err := listAPI.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	if err != nil || listOut == nil {
		return ""
	}
	var allResources []string
	for _, policyName := range listOut.PolicyNames {
		getOut, getErr := getAPI.GetRolePolicy(ctx, &iam.GetRolePolicyInput{
			RoleName:   aws.String(roleName),
			PolicyName: aws.String(policyName),
		})
		if getErr != nil || getOut == nil || getOut.PolicyDocument == nil {
			continue
		}
		doc := *getOut.PolicyDocument
		if decoded, decErr := url.QueryUnescape(doc); decErr == nil {
			doc = decoded
		}
		allResources = append(allResources, extractPolicyResources(doc)...)
	}
	return strings.Join(allResources, ",")
}

// extractPolicyResources parses a policy-document JSON string and returns
// every Resource entry across all Statements, flattening string and []string
// forms.
func extractPolicyResources(doc string) []string {
	if doc == "" {
		return nil
	}
	var parsed struct {
		Statement []struct {
			Resource any `json:"Resource"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(doc), &parsed); err != nil {
		return nil
	}
	var out []string
	for _, stmt := range parsed.Statement {
		switch v := stmt.Resource.(type) {
		case string:
			if v != "" {
				out = append(out, v)
			}
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// parseTrustWildcard examines a decoded AssumeRolePolicyDocument JSON string
// and returns ("true"/"false", "WILDCARD"/"") indicating whether the policy
// has a Statement with Principal.AWS == "*" and no Condition.StringEquals.sts:ExternalId.
func parseTrustWildcard(doc string) (trustWildcard, trustSummary string) {
	if doc == "" {
		return "false", ""
	}
	var policy struct {
		Statement []struct {
			Principal struct {
				AWS any `json:"AWS"`
			} `json:"Principal"`
			Condition map[string]any `json:"Condition"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(doc), &policy); err != nil {
		return "false", ""
	}
	for _, stmt := range policy.Statement {
		hasWildcard := false
		switch v := stmt.Principal.AWS.(type) {
		case string:
			if v == "*" {
				hasWildcard = true
			}
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok && s == "*" {
					hasWildcard = true
					break
				}
			}
		}
		if !hasWildcard {
			continue
		}
		// Check for mitigating condition.
		hasExternalID := false
		if cond, ok := stmt.Condition["StringEquals"]; ok {
			if condMap, ok := cond.(map[string]any); ok {
				for k := range condMap {
					if strings.EqualFold(k, "sts:externalid") {
						hasExternalID = true
						break
					}
				}
			}
		}
		if !hasExternalID {
			return "true", "WILDCARD"
		}
	}
	return "false", ""
}
