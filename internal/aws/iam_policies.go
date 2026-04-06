package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("policy", []string{"policy_name", "policy_id", "attachment_count", "path", "create_date"})

	resource.RegisterPaginated("policy", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchIAMPoliciesPage(ctx, c.IAM, continuationToken)
	})

	resource.RegisterRelated("policy", []resource.RelatedDef{
		{TargetType: "role", DisplayName: "IAM Roles", Checker: nil, NeedsTargetCache: false},
		{TargetType: "iam-user", DisplayName: "IAM Users", Checker: nil, NeedsTargetCache: false},
		{TargetType: "iam-group", DisplayName: "IAM Groups", Checker: nil, NeedsTargetCache: false},
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
		Scope: iamtypes.PolicyScopeTypeLocal,
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

		policyID := ""
		if policy.PolicyId != nil {
			policyID = *policy.PolicyId
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
				"policy_id":        policyID,
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
