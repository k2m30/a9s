// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkSSMKMS returns the key that encrypts this SecureString parameter, from
// its KeyId (a key ID or an alias).
func checkSSMKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	param, ok := assertStruct[ssmtypes.ParameterMetadata](res.RawStruct)
	if !ok {
		return NotRead("kms")
	}

	if param.Type != ssmtypes.ParameterTypeSecureString {
		return foundNone("kms", "param.Type")
	}

	if param.KeyId == nil || *param.KeyId == "" {
		return foundNone("kms", "param.KeyId")
	}
	return kmsRelated(ctx, clients, cache, []string{*param.KeyId})
}
