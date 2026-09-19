// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// kms_related.go contains KMS key related-resource checker functions.
package aws

import (
	"context"
	"encoding/json"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// kmsKeyID returns the bare UUID portion of the KMS key ID for this resource.
// res.ID is already the bare key UUID for KMS resources.
func kmsKeyID(res resource.Resource) string {
	return res.ID
}

// checkKMSEBS searches the ebs cache for volumes whose KmsKeyId contains this key's ID.
// EBS volumes store KmsKeyId as a full ARN (arn:aws:kms:region:account:key/{uuid}).
func checkKMSEBS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	keyID := kmsKeyID(res)
	if keyID == "" {
		return resource.KnownRelated("ebs", nil, false)
	}

	ebsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ebs")
	if err != nil {
		return resource.ErrorRelated("ebs", err)
	}
	if ebsList == nil {
		return resource.UnknownRelated("ebs")
	}

	var ids []string
	for _, ebsRes := range ebsList {
		vol, ok := assertStruct[ec2types.Volume](ebsRes.RawStruct)
		if !ok {
			continue
		}
		if vol.KmsKeyId == nil || *vol.KmsKeyId == "" {
			continue
		}
		if kmsIDMatches(*vol.KmsKeyId, keyID, refContext(clients, cache, "kms")) {
			ids = append(ids, ebsRes.ID)
		}
	}
	return relatedResultTrunc("ebs", ids, truncated)
}

// checkKMSRDS searches the dbi cache for RDS instances whose KmsKeyId contains this key's ID.
func checkKMSRDS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	keyID := kmsKeyID(res)
	if keyID == "" {
		return resource.KnownRelated("dbi", nil, false)
	}

	dbiList, truncated, err := relatedResourcesFor(ctx, clients, cache, "dbi")
	if err != nil {
		return resource.ErrorRelated("dbi", err)
	}
	if dbiList == nil {
		return resource.UnknownRelated("dbi")
	}

	var ids []string
	for _, dbiRes := range dbiList {
		db, ok := assertStruct[rdstypes.DBInstance](dbiRes.RawStruct)
		if !ok {
			continue
		}
		if db.KmsKeyId == nil || *db.KmsKeyId == "" {
			continue
		}
		if kmsIDMatches(*db.KmsKeyId, keyID, refContext(clients, cache, "kms")) {
			ids = append(ids, dbiRes.ID)
		}
	}
	return relatedResultTrunc("dbi", ids, truncated)
}

// checkKMSSecrets searches the secrets cache for Secrets Manager secrets whose KmsKeyId
// contains this key's ID.
func checkKMSSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	keyID := kmsKeyID(res)
	if keyID == "" {
		return resource.KnownRelated("secrets", nil, false)
	}

	secretsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.ErrorRelated("secrets", err)
	}
	if secretsList == nil {
		return resource.UnknownRelated("secrets")
	}

	var ids []string
	for _, secretRes := range secretsList {
		secret, ok := assertStruct[smtypes.SecretListEntry](secretRes.RawStruct)
		if !ok {
			continue
		}
		if secret.KmsKeyId == nil || *secret.KmsKeyId == "" {
			continue
		}
		if kmsIDMatches(*secret.KmsKeyId, keyID, refContext(clients, cache, "kms")) {
			ids = append(ids, secretRes.ID)
		}
	}
	return relatedResultTrunc("secrets", ids, truncated)
}

// kmsIDMatches reports whether a KMS reference value (key ARN, bare key ID,
// alias or alias ARN) names the key keyID.
func kmsIDMatches(ref, keyID string, rc domain.RefContext) bool {
	id, ok := resource.ResolveRef("kms", ref, rc)
	return ok && id == keyID
}

// kmsIAMPolicyDoc is a minimal IAM policy document used for parsing Principal.AWS fields.
type kmsIAMPolicyDoc struct {
	Statement []struct {
		Principal struct {
			AWS any `json:"AWS"` // may be string or []string
		} `json:"Principal"`
	} `json:"Statement"`
}

// kmsRoleARNsFromPolicyJSON extracts IAM role ARNs from an IAM policy JSON string.
// Handles Principal.AWS as either a plain string or a JSON array of strings.
// Only entries matching arn:aws:iam::*:role/* are extracted.
func kmsRoleARNsFromPolicyJSON(policyJSON string) []string {
	if policyJSON == "" {
		return nil
	}
	var doc kmsIAMPolicyDoc
	if err := json.Unmarshal([]byte(policyJSON), &doc); err != nil {
		return nil
	}
	var arns []string
	for _, stmt := range doc.Statement {
		var principals []string
		switch v := stmt.Principal.AWS.(type) {
		case string:
			principals = []string{v}
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					principals = append(principals, s)
				}
			}
		}
		for _, p := range principals {
			if strings.Contains(p, ":role/") {
				arns = append(arns, p)
			}
		}
	}
	return arns
}

// checkKMSRole resolves IAM roles that have access to this KMS key.
// Pattern C: calls kms:GetKeyPolicy (default policy) to parse Principal.AWS
// role ARNs from the policy JSON, and kms:ListGrants to collect GranteePrincipal
// and RetiringPrincipal role ARNs. Results are deduplicated.
func checkKMSRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	keyID := kmsKeyID(res)
	if keyID == "" {
		return resource.KnownRelated("role", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.KMS == nil {
		return resource.UnknownRelated("role")
	}
	policyAPI, ok := c.KMS.(KMSGetKeyPolicyAPI)
	if !ok {
		return resource.UnknownRelated("role")
	}
	grantsAPI, ok := c.KMS.(KMSListGrantsAPI)
	if !ok {
		return resource.UnknownRelated("role")
	}

	var refs []string
	policyName := "default"

	// --- GetKeyPolicy: parse Principal.AWS role ARNs ---
	policyOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kms.GetKeyPolicyOutput, error) {
		return policyAPI.GetKeyPolicy(ctx, &kms.GetKeyPolicyInput{
			KeyId:      &keyID,
			PolicyName: &policyName,
		})
	})
	if err != nil {
		// Permission errors, throttling, or any unrecoverable failure must yield -1.
		return resource.ErrorRelated("role", err)
	}
	if policyOut != nil && policyOut.Policy != nil {
		refs = kmsRoleARNsFromPolicyJSON(*policyOut.Policy)
	}

	// --- ListGrants: collect role ARNs from GranteePrincipal / RetiringPrincipal ---
	grantsOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kms.ListGrantsOutput, error) {
		return grantsAPI.ListGrants(ctx, &kms.ListGrantsInput{KeyId: &keyID})
	})
	if err != nil {
		// Permission errors, throttling, or any unrecoverable failure must yield -1.
		return resource.ErrorRelated("role", err)
	}
	if grantsOut != nil {
		for _, g := range grantsOut.Grants {
			for _, p := range []string{
				func() string {
					if g.GranteePrincipal != nil {
						return *g.GranteePrincipal
					}
					return ""
				}(),
				func() string {
					if g.RetiringPrincipal != nil {
						return *g.RetiringPrincipal
					}
					return ""
				}(),
			} {
				if p != "" && strings.Contains(p, ":role/") {
					refs = append(refs, p)
				}
			}
		}
	}

	return relatedRefs("role", refs, refContext(clients, cache, "role"))
}
