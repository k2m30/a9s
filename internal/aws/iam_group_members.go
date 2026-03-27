package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("iam_group_members", []string{
		"user_name", "user_id", "arn", "path", "create_date", "password_last_used",
	})

	resource.RegisterPaginatedChild("iam_group_members", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchIAMGroupMembers(ctx, c.IAM, parentCtx, continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Group Members",
		ShortName: "iam_group_members",
		Columns:   resource.IAMGroupMemberColumns(),
		CopyField: "user_name",
	})
}

// FetchIAMGroupMembers calls the IAM GetGroup API and converts the response
// into a FetchResult of generic Resource structs representing group members.
func FetchIAMGroupMembers(
	ctx context.Context,
	api IAMGetGroupAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	groupName := parentCtx["group_name"]

	var resources []resource.Resource
	var marker *string
	if continuationToken != "" {
		marker = &continuationToken
	}

	for {
		input := &iam.GetGroupInput{
			GroupName: &groupName,
			Marker:   marker,
		}

		output, err := api.GetGroup(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("getting group %s: %w", groupName, err)
		}

		for _, user := range output.Users {
			userName := ""
			if user.UserName != nil {
				userName = *user.UserName
			}

			userID := ""
			if user.UserId != nil {
				userID = *user.UserId
			}

			arn := ""
			if user.Arn != nil {
				arn = *user.Arn
			}

			path := ""
			if user.Path != nil {
				path = *user.Path
			}

			createDate := ""
			if user.CreateDate != nil {
				createDate = user.CreateDate.UTC().Format("2006-01-02 15:04")
			}

			resources = append(resources, resource.Resource{
				ID:     userName,
				Name:   userName,
				Status: "",
				Fields: map[string]string{ //nolint:gosec // "password_last_used" is a display field key, not a credential
					"user_name":          userName,
					"user_id":            userID,
					"arn":                arn,
					"path":               path,
					"create_date":        createDate,
					"password_last_used": "N/A (not in API)",
				},
				RawStruct: user,
			})
		}

		if !output.IsTruncated {
			break
		}
		marker = output.Marker
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			TotalHint:   len(resources),
			PageSize:    len(resources),
		},
	}, nil
}
