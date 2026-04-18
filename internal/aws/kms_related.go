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

	"github.com/k2m30/a9s/v3/internal/resource"
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
		return resource.RelatedCheckResult{TargetType: "ebs", Count: 0}
	}

	ebsList, truncated, err := kmsRelatedResources(ctx, clients, cache, "ebs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: -1, Err: err}
	}
	if ebsList == nil {
		return resource.RelatedCheckResult{TargetType: "ebs", Count: -1}
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
		if kmsIDMatches(*vol.KmsKeyId, keyID) {
			ids = append(ids, ebsRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("ebs")
	}
	return relatedResult("ebs", ids)
}

// checkKMSRDS searches the dbi cache for RDS instances whose KmsKeyId contains this key's ID.
func checkKMSRDS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	keyID := kmsKeyID(res)
	if keyID == "" {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: 0}
	}

	dbiList, truncated, err := kmsRelatedResources(ctx, clients, cache, "dbi")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1, Err: err}
	}
	if dbiList == nil {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1}
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
		if kmsIDMatches(*db.KmsKeyId, keyID) {
			ids = append(ids, dbiRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("dbi")
	}
	return relatedResult("dbi", ids)
}

// checkKMSSecrets searches the secrets cache for Secrets Manager secrets whose KmsKeyId
// contains this key's ID.
func checkKMSSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	keyID := kmsKeyID(res)
	if keyID == "" {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}

	secretsList, truncated, err := kmsRelatedResources(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1, Err: err}
	}
	if secretsList == nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
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
		if kmsIDMatches(*secret.KmsKeyId, keyID) {
			ids = append(ids, secretRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("secrets")
	}
	return relatedResult("secrets", ids)
}

// checkKMSS3 searches the S3 cache for buckets that use this KMS key for encryption.
// S3 bucket resources assembled by FetchS3BucketsPage do not store KMS key info in
// Fields or RawStruct, so this checker returns Count: -1 (unknown).
func checkKMSS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	// S3 resources do not expose KMS key IDs in Fields or RawStruct, so the
	// relationship cannot be determined from cache alone.
	return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
}


// kmsIDMatches reports whether a KMS reference value (full ARN, bare key UUID,
// or alias ARN) contains the given bare key UUID.
// A full ARN has the form arn:aws:kms:region:account:key/{uuid}.
func kmsIDMatches(ref, keyID string) bool {
	if ref == keyID {
		return true
	}
	// Extract the UUID suffix after the last "/" for ARN-style references.
	idx := strings.LastIndex(ref, "/")
	if idx >= 0 && idx < len(ref)-1 {
		return ref[idx+1:] == keyID
	}
	return false
}

// kmsRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func kmsRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// kmsIAMPolicyDoc is a minimal IAM policy document used for parsing Principal.AWS fields.
type kmsIAMPolicyDoc struct {
	Statement []struct {
		Principal struct {
			AWS any `json:"AWS"` // may be string or []string
		} `json:"Principal"`
	} `json:"Statement"`
}

// kmsRoleNamesFromPolicyJSON extracts IAM role names from an IAM policy JSON string.
// Handles Principal.AWS as either a plain string or a JSON array of strings.
// Only entries matching arn:aws:iam::*:role/* are extracted; the role name is
// the last "/" segment.
func kmsRoleNamesFromPolicyJSON(policyJSON string) []string {
	if policyJSON == "" {
		return nil
	}
	var doc kmsIAMPolicyDoc
	if err := json.Unmarshal([]byte(policyJSON), &doc); err != nil {
		return nil
	}
	seen := make(map[string]struct{})
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
				name := arnRoleName(p)
				if name != "" {
					seen[name] = struct{}{}
				}
			}
		}
	}
	return mapKeys(seen)
}

// checkKMSRole resolves IAM roles that have access to this KMS key.
// Pattern C: calls kms:GetKeyPolicy (default policy) to parse Principal.AWS
// role ARNs from the policy JSON, and kms:ListGrants to collect GranteePrincipal
// and RetiringPrincipal role ARNs. Results are deduplicated.
func checkKMSRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	keyID := kmsKeyID(res)
	if keyID == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.KMS == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	policyAPI, ok := c.KMS.(KMSGetKeyPolicyAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	grantsAPI, ok := c.KMS.(KMSListGrantsAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}

	seen := make(map[string]struct{})
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
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	if policyOut != nil && policyOut.Policy != nil {
		for _, name := range kmsRoleNamesFromPolicyJSON(*policyOut.Policy) {
			seen[name] = struct{}{}
		}
	}

	// --- ListGrants: collect role ARNs from GranteePrincipal / RetiringPrincipal ---
	grantsOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kms.ListGrantsOutput, error) {
		return grantsAPI.ListGrants(ctx, &kms.ListGrantsInput{KeyId: &keyID})
	})
	if err != nil {
		// Permission errors, throttling, or any unrecoverable failure must yield -1.
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
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
					seen[arnRoleName(p)] = struct{}{}
				}
			}
		}
	}

	return relatedResult("role", mapKeys(seen))
}
