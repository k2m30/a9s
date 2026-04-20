// iam_policy_docs.go owns IAM policy document fetch/decode helpers used by both iam_policy_detail_enrichment.go and iam_role_policies_detail_enrichment.go.
package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
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
	// IAM returns percent-encoded documents per RFC 3986.
	// PathUnescape preserves literal '+' in policy documents (e.g. regex patterns);
	// QueryUnescape treats '+' as space (used in some SDK mock/test encodings).
	// Try PathUnescape first; fall back to QueryUnescape when the result is not valid JSON.
	decoded, err := url.PathUnescape(encoded)
	if err != nil {
		return nil, fmt.Errorf("URL decode: %w", err)
	}
	var doc any
	if err := json.Unmarshal([]byte(decoded), &doc); err != nil {
		// PathUnescape left '+' as literal '+', making the JSON invalid.
		// Retry with QueryUnescape which converts '+' to space.
		if decoded2, err2 := url.QueryUnescape(encoded); err2 == nil {
			if err3 := json.Unmarshal([]byte(decoded2), &doc); err3 == nil {
				return doc, nil
			}
		}
		return nil, fmt.Errorf("JSON parse: %w", err)
	}
	return doc, nil
}
