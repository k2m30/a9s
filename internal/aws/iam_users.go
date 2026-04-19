package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("iam-user", []string{"user_name", "user_id", "path", "create_date", "password_last_used", "has_console_password"})

	resource.RegisterPaginated("iam-user", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchIAMUsersPage(ctx, c.IAM, continuationToken)
	})

	resource.RegisterRelated("iam-user", []resource.RelatedDef{
		{TargetType: "iam-group", DisplayName: "IAM Groups", Checker: checkUserGroup, NeedsTargetCache: false},
		{TargetType: "policy", DisplayName: "IAM Policies", Checker: checkUserPolicy, NeedsTargetCache: false},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkIAMUserCtEvents, NeedsTargetCache: false},
	})

	// iamtypes.User: no navigable cross-ref fields in the rendered detail view.
	// Groups membership is a runtime API relationship (checkUserGroup), not a field on the User struct.
	// PermissionsBoundary.PermissionsBoundaryArn exists but policy ARNs don't match a9s policy IDs.
}

// FetchIAMUsers calls the IAM ListUsers API and returns all pages of users.
// Used by tests; the production path uses the per-page fetcher for pagination.
func FetchIAMUsers(ctx context.Context, api IAMListUsersAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchIAMUsersPage(ctx, api, token)
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

// FetchIAMUsersPage calls the IAM ListUsers API and returns a single page
// of users. Pass an empty continuationToken for the first page.
func FetchIAMUsersPage(ctx context.Context, api IAMListUsersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &iam.ListUsersInput{
		MaxItems: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.ListUsers(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching IAM users: %w", err)
	}

	var resources []resource.Resource
	for _, user := range output.Users {
		userName := ""
		if user.UserName != nil {
			userName = *user.UserName
		}

		userID := ""
		if user.UserId != nil {
			userID = *user.UserId
		}

		path := ""
		if user.Path != nil {
			path = *user.Path
		}

		createDate := ""
		if user.CreateDate != nil {
			createDate = user.CreateDate.Format("2006-01-02 15:04")
		}

		passwordLastUsed := "Never"
		if user.PasswordLastUsed != nil {
			passwordLastUsed = user.PasswordLastUsed.Format("2006-01-02 15:04")
		}

		r := resource.Resource{
			ID:     userName,
			Name:   userName,
			Status: "",
			Fields: map[string]string{
				"user_name":            userName,
				"user_id":              userID,
				"path":                 path,
				"create_date":          createDate,
				"password_last_used":   passwordLastUsed,
				// has_console_password is set to "false" at fetch time; Wave 2
				// EnrichIAMUserMFA calls GetLoginProfile per user to detect
				// console password presence and updates this field via findings.
				// The Color func reads this field directly for row-level coloring.
				"has_console_password": "false", //nolint:gosec // not a credential, display field key
			},
			RawStruct: user,
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
