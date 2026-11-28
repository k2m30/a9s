// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// enrichRole fills the members ListRoles leaves out (the role catalog
// entry's ListOmits) from GetRole's answer.
func enrichRole(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	return enrichDetail(ctx, clients, res, detailEnrichSpec[iamtypes.Role, struct{}]{
		unwrap: func(raw any) (iamtypes.Role, bool) { r, ok := raw.(iamtypes.Role); return r, ok },
		id: func(role iamtypes.Role, _ resource.Resource) (string, error) {
			if aws.ToString(role.RoleName) == "" {
				return "", fmt.Errorf("role has no name")
			}
			return *role.RoleName, nil
		},
		answer: func(ctx context.Context, c *ServiceClients, id string) (*iamtypes.Role, error) {
			api, ok := c.IAM.(IAMGetRoleAPI)
			if !ok {
				return nil, fmt.Errorf("IAM client does not support GetRole")
			}
			out, err := api.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(id)})
			if err != nil {
				return nil, err
			}
			return out.Role, nil
		},
		listType: "role",
		cache:    detailDocsCache,
		cacheKey: func(id string, _ iamtypes.Role, _ resource.Resource) string { return "role:" + id },
		wrap:     func(role iamtypes.Role, _ struct{}) any { return role },
	})
}

// enrichIAMUser fills the members ListUsers leaves out (the iam-user catalog
// entry's ListOmits) from GetUser's answer. A group member is the same struct
// read through GetGroup, and is answered the same way.
func enrichIAMUser(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	return enrichDetail(ctx, clients, res, detailEnrichSpec[iamtypes.User, struct{}]{
		unwrap: func(raw any) (iamtypes.User, bool) { u, ok := raw.(iamtypes.User); return u, ok },
		id: func(user iamtypes.User, _ resource.Resource) (string, error) {
			if aws.ToString(user.UserName) == "" {
				return "", fmt.Errorf("user has no name")
			}
			return *user.UserName, nil
		},
		answer: func(ctx context.Context, c *ServiceClients, id string) (*iamtypes.User, error) {
			api, ok := c.IAM.(IAMGetUserAPI)
			if !ok {
				return nil, fmt.Errorf("IAM client does not support GetUser")
			}
			out, err := api.GetUser(ctx, &iam.GetUserInput{UserName: aws.String(id)})
			if err != nil {
				return nil, err
			}
			return out.User, nil
		},
		listType: "iam-user",
		cache:    detailDocsCache,
		cacheKey: func(id string, _ iamtypes.User, _ resource.Resource) string { return "iam-user:" + id },
		wrap:     func(user iamtypes.User, _ struct{}) any { return user },
	})
}
