// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

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
	kmsList, truncated, err := relatedResourcesFor(ctx, clients, cache, "kms")
	if err != nil {
		return resource.ErrorRelated("kms", err)
	}
	if kmsList == nil {
		return resource.UnknownRelated("kms")
	}
	rc := refContext(clients, cache, "kms")
	rc.Targets = kmsList
	keyID, local := resource.ResolveRef("kms", *param.KeyId, rc)

	var ids []string
	for _, kmsRes := range kmsList {
		if local && kmsRes.ID == keyID {
			ids = append(ids, kmsRes.ID)
		}
	}
	return relatedResultTrunc("kms", ids, truncated || !local)
}
