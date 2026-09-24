// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// kms_related.go contains KMS key related-resource checker functions.
package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
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
		return foundNone("ebs", "keyID")
	}

	ebsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ebs")
	if err != nil {
		return ReadFailed("ebs", err)
	}
	if ebsList == nil {
		return NotRead("ebs")
	}

	rc := kmsViewedKeyContext(clients, cache, res)
	var ids []string
	for _, ebsRes := range ebsList {
		vol, ok := assertStruct[ec2types.Volume](ebsRes.RawStruct)
		if !ok {
			continue
		}
		if vol.KmsKeyId == nil || *vol.KmsKeyId == "" {
			continue
		}
		match, unknown := kmsRefNames(*vol.KmsKeyId, keyID, rc)
		if match {
			ids = append(ids, ebsRes.ID)
		}
		truncated = truncated || unknown
	}
	return relatedResultTrunc("ebs", ids, truncated)
}

// checkKMSRDS searches the dbi cache for RDS instances whose KmsKeyId contains this key's ID.
func checkKMSRDS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	keyID := kmsKeyID(res)
	if keyID == "" {
		return foundNone("dbi", "keyID")
	}

	dbiList, truncated, err := relatedResourcesFor(ctx, clients, cache, "dbi")
	if err != nil {
		return ReadFailed("dbi", err)
	}
	if dbiList == nil {
		return NotRead("dbi")
	}

	rc := kmsViewedKeyContext(clients, cache, res)
	var ids []string
	for _, dbiRes := range dbiList {
		db, ok := assertStruct[rdstypes.DBInstance](dbiRes.RawStruct)
		if !ok {
			continue
		}
		if db.KmsKeyId == nil || *db.KmsKeyId == "" {
			continue
		}
		match, unknown := kmsRefNames(*db.KmsKeyId, keyID, rc)
		if match {
			ids = append(ids, dbiRes.ID)
		}
		truncated = truncated || unknown
	}
	return relatedResultTrunc("dbi", ids, truncated)
}

// checkKMSSecrets searches the secrets cache for Secrets Manager secrets whose KmsKeyId
// contains this key's ID.
func checkKMSSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	keyID := kmsKeyID(res)
	if keyID == "" {
		return foundNone("secrets", "keyID")
	}

	secretsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "secrets")
	if err != nil {
		return ReadFailed("secrets", err)
	}
	if secretsList == nil {
		return NotRead("secrets")
	}

	rc := kmsViewedKeyContext(clients, cache, res)
	var ids []string
	for _, secretRes := range secretsList {
		secret, ok := assertStruct[smtypes.SecretListEntry](secretRes.RawStruct)
		if !ok {
			continue
		}
		if secret.KmsKeyId == nil || *secret.KmsKeyId == "" {
			continue
		}
		match, unknown := kmsRefNames(*secret.KmsKeyId, keyID, rc)
		if match {
			ids = append(ids, secretRes.ID)
		}
		truncated = truncated || unknown
	}
	return relatedResultTrunc("secrets", ids, truncated)
}

// kmsViewedKeyContext is refContext for the kms target with the viewed key
// among the Targets, so its aliases resolve whether or not the kms list is
// loaded.
func kmsViewedKeyContext(clients any, cache resource.ResourceCache, key resource.Resource) domain.RefContext {
	rc := refContext(clients, cache, "kms")
	rc.Targets = append(slices.Clone(rc.Targets), key)
	return rc
}

// kmsRefNames reports whether a KMS reference value (key ARN, bare key ID,
// alias or alias ARN) names the key keyID, and whether it is a local alias no
// target carries, which may name it.
func kmsRefNames(ref, keyID string, rc domain.RefContext) (match, unknown bool) {
	if id, ok := resource.ResolveRef("kms", ref, rc); ok {
		return id == keyID, false
	}
	res, _, local := localARN(ref, rc, "kms")
	return false, local && strings.HasPrefix(res, "alias/")
}

// checkKMSRole resolves IAM roles that have access to this KMS key: the
// roles its default key policy (kms:GetKeyPolicy) grants, and the
// GranteePrincipal and RetiringPrincipal roles of its grants
// (kms:ListGrants). Results are deduplicated.
func checkKMSRole(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	keyID := kmsKeyID(res)
	if keyID == "" {
		return foundNone("role", "keyID")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.KMS == nil {
		return NotRead("role")
	}
	policyAPI, ok := c.KMS.(KMSGetKeyPolicyAPI)
	if !ok {
		return NotRead("role")
	}
	grantsAPI, ok := c.KMS.(KMSListGrantsAPI)
	if !ok {
		return NotRead("role")
	}

	keyARN := ""
	if meta, isMeta := assertStruct[kmstypes.KeyMetadata](res.RawStruct); isMeta {
		keyARN = aws.ToString(meta.Arn)
	}
	rc := policyRefContext(clients, cache, "role", keyARN)
	var refs []string
	policyName := "default"

	policyOut, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kms.GetKeyPolicyOutput, error) {
		return policyAPI.GetKeyPolicy(ctx, &kms.GetKeyPolicyInput{
			KeyId:      &keyID,
			PolicyName: &policyName,
		})
	})
	if err != nil {
		// Permission errors, throttling, or any unrecoverable failure must yield -1.
		return ReadFailed("role", err)
	}
	if policyOut != nil && policyOut.Policy != nil {
		if refs, ok = grantedPrincipalRefs(*policyOut.Policy, "role/"); !ok {
			return NotRead("role")
		}
	}

	grants, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]kmstypes.GrantListEntry, *string, error) {
		out, callErr := grantsAPI.ListGrants(ctx, &kms.ListGrantsInput{KeyId: &keyID, Marker: marker})
		if callErr != nil {
			return nil, nil, callErr
		}
		return out.Grants, out.NextMarker, nil
	})
	ids, dropped := resolveRefs("role", refs, rc)
	// A grant principal is an ARN (a role, an assumed-role session, a user)
	// or an AWS service principal; the role resolver reads the ones that
	// name this account's roles, and every other principal names no row.
	var principals []string
	for _, g := range grants {
		principals = append(principals, aws.ToString(g.GranteePrincipal), aws.ToString(g.RetiringPrincipal))
	}
	for _, p := range arnsOnly(principals) {
		if id, ok := resource.ResolveRef("role", p, rc); ok && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return alsoRead(relatedResultTrunc("role", ids, dropped), pagedRead(complete, err))
}
