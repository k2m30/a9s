package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("role", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchIAMRoles(ctx, c.IAM)
	})
	resource.RegisterFieldKeys("role", []string{"role_name", "role_id", "path", "create_date", "description"})
}

// FetchIAMRoles calls the IAM ListRoles API and converts the
// response into a slice of generic Resource structs.
func FetchIAMRoles(ctx context.Context, api IAMListRolesAPI) ([]resource.Resource, error) {
	output, err := api.ListRoles(ctx, &iam.ListRolesInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching IAM roles: %w", err)
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
			createDate = role.CreateDate.Format("2006-01-02T15:04:05Z07:00")
		}

		description := ""
		if role.Description != nil {
			description = *role.Description
		}

		r := resource.Resource{
			ID:     roleName,
			Name:   roleName,
			Status: "",
			Fields: map[string]string{
				"role_name":   roleName,
				"role_id":     roleID,
				"path":        path,
				"create_date": createDate,
				"description": description,
			},
			RawStruct:  role,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
