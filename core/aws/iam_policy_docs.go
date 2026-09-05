// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_policy_docs.go owns IAM policy document fetch/decode helpers used by both iam_policy_detail_enrichment.go and iam_role_policies_detail_enrichment.go.
package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/k2m30/a9s/v3/core/iampolicy"
)

// FetchManagedPolicyDocument fetches and decodes a managed policy document.
// Two API calls: GetPolicy (for DefaultVersionId) then GetPolicyVersion.
func FetchManagedPolicyDocument(ctx context.Context, getPolAPI IAMGetPolicyAPI, getVerAPI IAMGetPolicyVersionAPI, policyArn string) (any, error) {
	polOut, err := getPolAPI.GetPolicy(ctx, &iam.GetPolicyInput{
		PolicyArn: aws.String(policyArn),
	})
	if err != nil {
		return nil, fmt.Errorf("GetPolicy: %w", err)
	}
	if polOut.Policy == nil || polOut.Policy.DefaultVersionId == nil {
		return nil, fmt.Errorf("GetPolicy returned nil policy or version ID")
	}

	verOut, err := getVerAPI.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
		PolicyArn: aws.String(policyArn),
		VersionId: polOut.Policy.DefaultVersionId,
	})
	if err != nil {
		return nil, fmt.Errorf("GetPolicyVersion: %w", err)
	}
	if verOut.PolicyVersion == nil || verOut.PolicyVersion.Document == nil {
		return nil, fmt.Errorf("GetPolicyVersion returned nil document")
	}

	return decodePolicyDocument(*verOut.PolicyVersion.Document)
}

// FetchInlinePolicyDocument fetches and decodes an inline role policy document.
func FetchInlinePolicyDocument(ctx context.Context, api IAMGetRolePolicyAPI, roleName, policyName string) (any, error) {
	out, err := api.GetRolePolicy(ctx, &iam.GetRolePolicyInput{
		RoleName:   aws.String(roleName),
		PolicyName: aws.String(policyName),
	})
	if err != nil {
		return nil, fmt.Errorf("GetRolePolicy: %w", err)
	}
	if out.PolicyDocument == nil {
		return nil, fmt.Errorf("GetRolePolicy returned nil document")
	}
	return decodePolicyDocument(*out.PolicyDocument)
}

func decodePolicyDocument(encoded string) (any, error) {
	var doc any
	if err := json.Unmarshal([]byte(iampolicy.Decode(encoded)), &doc); err != nil {
		return nil, fmt.Errorf("JSON parse: %w", err)
	}
	return doc, nil
}
