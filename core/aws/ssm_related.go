// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strings"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkSSMKMS checks the KMS cache for the key used to encrypt this SecureString parameter.
// Pattern C: extracts KeyId from RawStruct, then scans the kms cache for a match.
func checkSSMKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	param, ok := assertStruct[ssmtypes.ParameterMetadata](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("kms")
		}
		return resource.KnownRelated("kms", nil, false)
	}

	if param.Type != ssmtypes.ParameterTypeSecureString {
		return resource.KnownRelated("kms", nil, false)
	}

	if param.KeyId == nil || *param.KeyId == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	keyRef := *param.KeyId

	kmsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "kms")
	if err != nil {
		return resource.ErrorRelated("kms", err)
	}
	if kmsList == nil {
		return resource.UnknownRelated("kms")
	}

	var ids []string
	for _, kmsRes := range kmsList {
		if matchesKMSKeyRef(kmsRes, keyRef) {
			ids = append(ids, kmsRes.ID)
		}
	}
	return relatedResultTrunc("kms", ids, truncated)
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
