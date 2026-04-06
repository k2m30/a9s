package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("iam-group", []string{"group_name", "group_id", "path", "create_date", "arn"})

	resource.RegisterPaginated("iam-group", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchIAMGroupsPage(ctx, c.IAM, continuationToken)
	})

	resource.RegisterRelated("iam-group", []resource.RelatedDef{
		{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkGroupUser, NeedsTargetCache: false},
		{TargetType: "policy", DisplayName: "IAM Policies", Checker: checkGroupPolicy, NeedsTargetCache: false},
	})
}

// FetchIAMGroups calls the IAM ListGroups API and returns all pages of groups.
// Used by existing tests and the legacy fetcher.
func FetchIAMGroups(ctx context.Context, api IAMListGroupsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchIAMGroupsPage(ctx, api, token)
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

// FetchIAMGroupsPage calls the IAM ListGroups API and returns a single page
// of groups. Pass an empty continuationToken for the first page.
func FetchIAMGroupsPage(ctx context.Context, api IAMListGroupsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &iam.ListGroupsInput{}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.ListGroups(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching IAM groups: %w", err)
	}

	var resources []resource.Resource
	for _, group := range output.Groups {
		groupName := ""
		if group.GroupName != nil {
			groupName = *group.GroupName
		}

		groupID := ""
		if group.GroupId != nil {
			groupID = *group.GroupId
		}

		path := ""
		if group.Path != nil {
			path = *group.Path
		}

		createDate := ""
		if group.CreateDate != nil {
			createDate = group.CreateDate.Format("2006-01-02 15:04")
		}

		arn := ""
		if group.Arn != nil {
			arn = *group.Arn
		}

		r := resource.Resource{
			ID:     groupName,
			Name:   groupName,
			Status: "",
			Fields: map[string]string{
				"group_name":  groupName,
				"group_id":    groupID,
				"path":        path,
				"create_date": createDate,
				"arn":         arn,
			},
			RawStruct: group,
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
