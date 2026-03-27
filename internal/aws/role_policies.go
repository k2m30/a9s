package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// RolePolicyRow is the RawStruct for each role policy row.
// It holds policy data for detail/YAML view rendering.
type RolePolicyRow struct {
	PolicyName string
	PolicyArn  string
	PolicyType string
}

func init() {
	resource.RegisterFieldKeys("role_policies", []string{
		"policy_name", "policy_arn", "policy_type",
	})

	resource.RegisterPaginatedChild("role_policies", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRolePolicies(ctx, c.IAM, c.IAM, parentCtx, continuationToken)
	})

	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Role Policies",
		ShortName: "role_policies",
		Columns:   resource.RolePolicyColumns(),
	})
}

// FetchRolePolicies calls ListAttachedRolePolicies and ListRolePolicies to
// retrieve both managed and inline policies attached to the given IAM role.
// Managed policies appear first, followed by inline policies.
func FetchRolePolicies(
	ctx context.Context,
	attachedAPI IAMListAttachedRolePoliciesAPI,
	inlineAPI IAMListRolePoliciesAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	roleName := parentCtx["role_name"]

	// Fetch managed (attached) policies with pagination
	var managed []resource.Resource
	var attachedMarker *string
	for {
		input := &iam.ListAttachedRolePoliciesInput{
			RoleName: &roleName,
			Marker:   attachedMarker,
		}
		output, err := attachedAPI.ListAttachedRolePolicies(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("listing attached policies for %s: %w", roleName, err)
		}

		for _, p := range output.AttachedPolicies {
			policyName := ""
			if p.PolicyName != nil {
				policyName = *p.PolicyName
			}
			policyArn := ""
			if p.PolicyArn != nil {
				policyArn = *p.PolicyArn
			}

			status := rolePolicyStatus(policyName)

			managed = append(managed, resource.Resource{
				ID:     policyArn,
				Name:   policyName,
				Status: status,
				Fields: map[string]string{
					"policy_name": policyName,
					"policy_arn":  policyArn,
					"policy_type": "Managed",
				},
				RawStruct: RolePolicyRow{
					PolicyName: policyName,
					PolicyArn:  policyArn,
					PolicyType: "Managed",
				},
			})
		}

		if !output.IsTruncated {
			break
		}
		attachedMarker = output.Marker
	}

	// Fetch inline policies with pagination
	var inline []resource.Resource
	var inlineMarker *string
	for {
		input := &iam.ListRolePoliciesInput{
			RoleName: &roleName,
			Marker:   inlineMarker,
		}
		output, err := inlineAPI.ListRolePolicies(ctx, input)
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("listing inline policies for %s: %w", roleName, err)
		}

		for _, name := range output.PolicyNames {
			inline = append(inline, resource.Resource{
				ID:     name,
				Name:   name,
				Status: "terminated",
				Fields: map[string]string{
					"policy_name": name,
					"policy_arn":  "",
					"policy_type": "Inline",
				},
				RawStruct: RolePolicyRow{
					PolicyName: name,
					PolicyArn:  "",
					PolicyType: "Inline",
				},
			})
		}

		if !output.IsTruncated {
			break
		}
		inlineMarker = output.Marker
	}

	// Managed first, then inline
	resources := make([]resource.Resource, 0, len(managed)+len(inline))
	resources = append(resources, managed...)
	resources = append(resources, inline...)

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			TotalHint:   len(resources),
			PageSize:    len(resources),
		},
	}, nil
}

// rolePolicyStatus returns "failed" for high-privilege policies that should
// be visually highlighted (red), empty string otherwise.
func rolePolicyStatus(policyName string) string {
	switch policyName {
	case "AdministratorAccess", "PowerUserAccess":
		return "failed"
	default:
		return ""
	}
}
