// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// kms_issue_enrichment.go — Wave 2 issue enrichment for the kms resource type.
package aws

import (
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// kms canonical FindingCodes.
const (
	kmsCodeRotationDisabled domain.FindingCode = "kms.rotation-disabled"
	kmsCodePublicPolicy     domain.FindingCode = "kms.public-policy"
)

// EnrichKMSRotation calls GetKeyRotationStatus for each customer-managed key (cap EnrichmentCap)
// and returns a Finding when key rotation is not enabled.
// Severity is "~" (informational) — rotation-disabled is an informational
// finding, not a "!" issue.
// AWS-managed keys are skipped. A per-key error marks the key not inspected.
func EnrichKMSRotation(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.KMS == nil {
		return result, nil
	}
	keyPolicyAPI, _ := clients.KMS.(KMSGetKeyPolicyAPI)
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	resources = capAtEnrichmentCap(&result, resources, func(r resource.Resource) bool {
		// AWS owns an AWS-managed key's policy and rotation; neither is the
		// operator's to change.
		return !kmsKeyIsAWSManaged(r)
	}, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	loopErr := ForEachRow(ctx, &result, resourceIDs(resources), EnrichmentParallelism, func(i int) {
		r := resources[i]
		keyID := r.ID
		if keyID == "" {
			return
		}
		ex, public, policyErr := kmsKeyPolicyIsPublic(ctx, keyPolicyAPI, r, ownAccount)
		mu.Lock()
		switch {
		case policyErr != nil:
			MarkSkipped(&result, keyID, &failures, policyErr)
		case public:
			setWave2Finding(&result, keyID, kmsCodePublicPolicy, publicPolicyRows(ex))

		}
		mu.Unlock()
		if !kmsKeyCanAutoRotate(r) {
			mu.Lock()
			defer mu.Unlock()
			result.FieldUpdates[keyID] = map[string]string{"rotation_enabled": "false"}
			setWave2Finding(&result, keyID, kmsCodeRotationDisabled, nil)
			return
		}
		out, err := clients.KMS.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{
			KeyId: aws.String(keyID),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}
		rotationVal := "false"
		if out.KeyRotationEnabled {
			rotationVal = "true"
		}
		result.FieldUpdates[keyID] = map[string]string{
			"rotation_enabled": rotationVal,
		}
		if !out.KeyRotationEnabled {
			setWave2Finding(&result, keyID, kmsCodeRotationDisabled, nil)
		}
	})
	return result, errors.Join(loopErr, AggregateFailures("key policy and rotation", failures, n))
}

// kmsKeyPolicyIsPublic reads a key's default policy and returns the exposure
// when it grants a wildcard principal. AWS-managed keys are skipped: AWS
// owns their policy and the operator cannot change it. A policy that cannot
// be read or parsed leaves the key unknown, never healthy; the error that made
// it unknown is returned so the caller can record it.
func kmsKeyPolicyIsPublic(ctx context.Context, api KMSGetKeyPolicyAPI, r resource.Resource, ownAccount string) (ex iampolicy.Exposure, public bool, err error) {
	if api == nil || kmsKeyIsAWSManaged(r) {
		return ex, false, nil
	}
	out, err := api.GetKeyPolicy(ctx, &kms.GetKeyPolicyInput{
		KeyId:      aws.String(r.ID),
		PolicyName: aws.String("default"),
	})
	if err != nil {
		return ex, false, err
	}
	if out == nil || out.Policy == nil {
		// Every KMS key has exactly one key policy, so an answer without it
		// has said nothing about who may use the key.
		return ex, false, UnusableAnswerErr{Call: "GetKeyPolicy", Field: "policy"}
	}
	doc, perr := iampolicy.Parse(*out.Policy)
	if perr != nil {
		return ex, false, perr
	}
	ex = iampolicy.Evaluate(doc, ownAccount)
	return ex, ex.Public, nil
}

// kmsKeyCanAutoRotate reports whether GetKeyRotationStatus has a schedule to
// report: "Automatic key rotation is supported only on symmetric encryption
// KMS keys with key material that AWS KMS generates" (AWS KMS Developer Guide,
// "Rotate AWS KMS keys"), which excludes asymmetric and HMAC keys, imported
// key material and keys in a custom key store. A row with no metadata is
// asked.
func kmsKeyCanAutoRotate(r resource.Resource) bool {
	meta, ok := assertStruct[kmstypes.KeyMetadata](r.RawStruct)
	if !ok {
		return true
	}
	return meta.KeySpec == kmstypes.KeySpecSymmetricDefault &&
		meta.Origin == kmstypes.OriginTypeAwsKms &&
		aws.ToString(meta.CustomKeyStoreId) == ""
}

// kmsKeyIsAWSManaged reads KeyManager off the DescribeKey metadata the
// fetcher stashed. FetchKMSKeysPage filters AWS-managed keys out, while
// FetchKMSKeysByIDs keeps them, so a drilled-into key can still arrive here.
func kmsKeyIsAWSManaged(r resource.Resource) bool {
	meta, ok := assertStruct[kmstypes.KeyMetadata](r.RawStruct)
	return ok && meta.KeyManager == kmstypes.KeyManagerTypeAws
}
