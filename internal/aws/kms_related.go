// kms_related.go contains KMS key related-resource checker functions.
package aws

import (
	"context"
	"strings"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
		return resource.RelatedCheckResult{TargetType: "ebs", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
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

// checkKMSRole returns Count: 0 because KMS keys have key policies (not IAM
// roles) and the key metadata in DescribeKey does not reference a role ARN.
func checkKMSRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
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
