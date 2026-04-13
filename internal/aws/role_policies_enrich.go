package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterEnricher("role_policies", enrichRolePolicy)
}

// enrichRolePolicy fetches the policy document for a role_policies resource
// and returns an enriched copy with Document set on RawStruct.
// Uses the session-scoped PolicyDocCache on ServiceClients.
func enrichRolePolicy(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return res, fmt.Errorf("invalid clients")
	}

	row, ok := res.RawStruct.(RolePolicyRow)
	if !ok {
		return res, fmt.Errorf("unexpected RawStruct type: %T", res.RawStruct)
	}

	var (
		doc any
		err error
	)

	if row.PolicyType == "Inline" {
		roleName := res.Fields["role_name"]
		if roleName == "" {
			return res, fmt.Errorf("missing role_name for inline policy")
		}
		cacheKey := InlineKey(roleName, row.PolicyName)

		if cached := c.PolicyDocCache.Get(cacheKey); cached != nil {
			doc = cached
		} else {
			getRolePolicyAPI, ok := c.IAM.(IAMGetRolePolicyAPI)
			if !ok {
				return res, fmt.Errorf("IAM client does not support GetRolePolicy")
			}
			doc, err = FetchInlinePolicyDocument(ctx, getRolePolicyAPI, roleName, row.PolicyName)
			if err != nil {
				return res, err
			}
			c.PolicyDocCache.Set(cacheKey, doc)
		}
	} else {
		if row.PolicyArn == "" {
			return res, fmt.Errorf("missing policy ARN for managed policy")
		}
		cacheKey := ManagedKey(row.PolicyArn)

		if cached := c.PolicyDocCache.Get(cacheKey); cached != nil {
			doc = cached
		} else {
			getPolicyAPI, ok1 := c.IAM.(IAMGetPolicyAPI)
			getPolicyVersionAPI, ok2 := c.IAM.(IAMGetPolicyVersionAPI)
			if !ok1 || !ok2 {
				return res, fmt.Errorf("IAM client does not support GetPolicy/GetPolicyVersion")
			}
			doc, err = FetchManagedPolicyDocument(ctx, getPolicyAPI, getPolicyVersionAPI, row.PolicyArn)
			if err != nil {
				return res, err
			}
			c.PolicyDocCache.Set(cacheKey, doc)
		}
	}

	row.Document = doc
	res.RawStruct = row
	return res, nil
}

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
	// Use PathUnescape (not QueryUnescape) to avoid treating + as space.
	decoded, err := url.PathUnescape(encoded)
	if err != nil {
		return nil, fmt.Errorf("URL decode: %w", err)
	}
	var doc any
	if err := json.Unmarshal([]byte(decoded), &doc); err != nil {
		return nil, fmt.Errorf("JSON parse: %w", err)
	}
	return doc, nil
}
