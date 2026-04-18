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

func init() {
	resource.RegisterFieldKeys("role", []string{"role_name", "role_id", "path", "create_date", "description", "assume_role_policy_document", "trust_wildcard", "trust_summary"})

	resource.RegisterPaginated("role", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchIAMRolesPage(ctx, c.IAM, continuationToken)
	})
}

// FetchIAMRoles calls the IAM ListRoles API and returns all pages of roles.
// Used by existing tests and the legacy fetcher.
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

		r := resource.Resource{
			ID:     roleName,
			Name:   roleName,
			Status: "",
			Fields: map[string]string{
				"role_name":                   roleName,
				"role_id":                     roleID,
				"path":                        path,
				"create_date":                 createDate,
				"description":                 description,
				"assume_role_policy_document": assumeRolePolicyDoc,
				"trust_wildcard":              trustWildcard,
				"trust_summary":               trustSummary,
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
