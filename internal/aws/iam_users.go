package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("iam-user", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchIAMUsers(ctx, c.IAM)
	})
	resource.RegisterFieldKeys("iam-user", []string{"user_name", "user_id", "path", "create_date", "password_last_used"})
}

// FetchIAMUsers calls the IAM ListUsers API and converts the
// response into a slice of generic Resource structs.
func FetchIAMUsers(ctx context.Context, api IAMListUsersAPI) ([]resource.Resource, error) {
	output, err := api.ListUsers(ctx, &iam.ListUsersInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching IAM users: %w", err)
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
			createDate = user.CreateDate.Format("2006-01-02T15:04:05Z07:00")
		}

		passwordLastUsed := "Never"
		if user.PasswordLastUsed != nil {
			passwordLastUsed = user.PasswordLastUsed.Format("2006-01-02T15:04:05Z07:00")
		}

		r := resource.Resource{
			ID:     userName,
			Name:   userName,
			Status: "",
			Fields: map[string]string{
				"user_name":          userName,
				"user_id":            userID,
				"path":               path,
				"create_date":        createDate,
				"password_last_used": passwordLastUsed,
			},
			RawStruct:  user,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
