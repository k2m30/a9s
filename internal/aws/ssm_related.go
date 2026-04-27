package aws

import (
	"context"
	"strings"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("ssm", []resource.RelatedDef{
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkSSMKMS, NeedsTargetCache: true},
	})

	// ssmtypes.ParameterMetadata: KeyId (present for SecureString parameters)
	resource.RegisterDefaultNavFields("ssm", []resource.NavigableField{
		{FieldPath: "KeyId", TargetType: "kms"},
	})
}

// checkSSMKMS checks the KMS cache for the key used to encrypt this SecureString parameter.
// Pattern C: extracts KeyId from RawStruct, then scans the kms cache for a match.
func checkSSMKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	param, ok := assertStruct[ssmtypes.ParameterMetadata](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}

	if param.Type != ssmtypes.ParameterTypeSecureString {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}

	if param.KeyId == nil || *param.KeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyRef := *param.KeyId

	kmsList, truncated, err := ssmRelatedResources(ctx, clients, cache, "kms")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
	}
	if kmsList == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}

	var ids []string
	for _, kmsRes := range kmsList {
		if matchesKMSKeyRef(kmsRes, keyRef) {
			ids = append(ids, kmsRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("kms")
	}
	return relatedResult("kms", ids)
}

// matchesKMSKeyRef returns true if the given KMS resource matches the key reference.
// keyRef may be a key ID, key ARN, alias name (e.g. "alias/aws/ssm"), or alias ARN.
func matchesKMSKeyRef(kmsRes resource.Resource, keyRef string) bool {
	if kmsRes.ID == keyRef {
		return true
	}
	if kmsRes.Fields["key_id"] == keyRef {
		return true
	}
	if arn, ok := kmsRes.Fields["arn"]; ok && arn != "" {
		if arn == keyRef || strings.HasSuffix(arn, "/"+keyRef) {
			return true
		}
	}
	if alias, ok := kmsRes.Fields["alias"]; ok && alias != "" {
		if alias == keyRef {
			return true
		}
	}
	// keyRef is an ARN containing the key ID (e.g. "arn:aws:kms:...:key/<id>")
	if strings.Contains(keyRef, "/") && strings.HasSuffix(keyRef, "/"+kmsRes.ID) {
		return true
	}
	return false
}

// ssmRelatedResources returns the cached resource list for the given target type,
// or fetches the first page via the registered paginated fetcher.
func ssmRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
