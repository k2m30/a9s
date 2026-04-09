package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("policy", []string{"policy_name", "policy_type", "attachment_count", "path", "create_date"})

	resource.RegisterPaginated("policy", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		result, err := FetchIAMPoliciesPage(ctx, c.IAM, continuationToken)
		if err != nil {
			return result, err
		}
		inlines := fetchInlineGroupPolicies(ctx, c.IAM)
		result.Resources = append(result.Resources, inlines...)
		if result.Pagination != nil {
			result.Pagination.PageSize = len(result.Resources)
		}
		return result, nil
	})

	resource.RegisterRelated("policy", []resource.RelatedDef{
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkPolicyRole, NeedsTargetCache: false},
		{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkPolicyUser, NeedsTargetCache: false},
		{TargetType: "iam-group", DisplayName: "IAM Groups", Checker: checkPolicyGroup, NeedsTargetCache: false},
	})
}

// FetchIAMPolicies calls the IAM ListPolicies API and returns all pages of
// customer-managed policies. Used by existing tests and the legacy fetcher.
func FetchIAMPolicies(ctx context.Context, api IAMListPoliciesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchIAMPoliciesPage(ctx, api, token)
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

// FetchIAMPoliciesPage calls the IAM ListPolicies API with Scope=Local
// and returns a single page of customer-managed policies.
// Pass an empty continuationToken for the first page.
func FetchIAMPoliciesPage(ctx context.Context, api IAMListPoliciesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &iam.ListPoliciesInput{
		Scope:    iamtypes.PolicyScopeTypeLocal,
		MaxItems: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.ListPolicies(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching IAM policies: %w", err)
	}

	var resources []resource.Resource
	for _, policy := range output.Policies {
		policyName := ""
		if policy.PolicyName != nil {
			policyName = *policy.PolicyName
		}

		attachmentCount := "0"
		if policy.AttachmentCount != nil {
			attachmentCount = fmt.Sprintf("%d", *policy.AttachmentCount)
		}

		path := ""
		if policy.Path != nil {
			path = *policy.Path
		}

		createDate := ""
		if policy.CreateDate != nil {
			createDate = policy.CreateDate.Format("2006-01-02 15:04")
		}

		r := resource.Resource{
			ID:     policyName,
			Name:   policyName,
			Status: "",
			Fields: map[string]string{
				"policy_name":      policyName,
				"policy_type":      "managed",
				"attachment_count": attachmentCount,
				"path":             path,
				"create_date":      createDate,
			},
			RawStruct: policy,
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

func fetchInlineGroupPolicies(ctx context.Context, api IAMAPI) []resource.Resource {
	var resources []resource.Resource
	groupsOut, err := api.ListGroups(ctx, &iam.ListGroupsInput{})
	if err != nil {
		return nil
	}
	for _, group := range groupsOut.Groups {
		if group.GroupName == nil {
			continue
		}
		out, err := api.ListGroupPolicies(ctx, &iam.ListGroupPoliciesInput{GroupName: group.GroupName})
		if err != nil {
			continue
		}
		for _, name := range out.PolicyNames {
			resources = append(resources, resource.Resource{
				ID:   name,
				Name: name,
				Fields: map[string]string{
					"policy_name":      name,
					"policy_type":      "inline",
					"attachment_count": "",
					"path":             "inline/" + *group.GroupName,
					"create_date":      "",
				},
			})
		}
	}
	return resources
}
