package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("policy", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchIAMPolicies(ctx, c.IAM)
	})
	resource.RegisterFieldKeys("policy", []string{"policy_name", "policy_id", "attachment_count", "path", "create_date"})
}

// FetchIAMPolicies calls the IAM ListPolicies API with Scope=Local
// to return only customer-managed policies.
func FetchIAMPolicies(ctx context.Context, api IAMListPoliciesAPI) ([]resource.Resource, error) {
	output, err := api.ListPolicies(ctx, &iam.ListPoliciesInput{
		Scope: iamtypes.PolicyScopeTypeLocal,
	})
	if err != nil {
		return nil, err
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
			createDate = policy.CreateDate.Format("2006-01-02T15:04:05Z07:00")
		}

		detail := map[string]string{
			"Policy Name":      policyName,
			"Policy ID":        policyID,
			"Attachment Count": attachmentCount,
			"Path":             path,
			"Created":          createDate,
		}

		if policy.Arn != nil {
			detail["ARN"] = *policy.Arn
		}
		if policy.Description != nil {
			detail["Description"] = *policy.Description
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(policy, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  policy,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
