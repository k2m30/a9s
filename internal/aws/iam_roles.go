package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchIAMRoles calls the IAM ListRoles API and returns all pages of roles.
// Used by tests; the production path uses the per-page fetcher for pagination.
func FetchIAMRoles(ctx context.Context, api IAMListRolesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchIAMRolesPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
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
