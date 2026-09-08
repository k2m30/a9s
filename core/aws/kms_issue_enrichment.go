// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// kms_issue_enrichment.go — Wave 2 issue enrichment for the kms resource type.
package aws

import (
	"context"
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
// AWS-managed keys reject GetKeyRotationStatus with AccessDeniedException — that error is
// silently skipped without marking Truncated. Other per-key errors set Truncated=true.
func EnrichKMSRotation(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.KMS == nil {
		return result, nil
	}
	keyPolicyAPI, _ := clients.KMS.(KMSGetKeyPolicyAPI)
	ownAccount := accountIDFromClients(ctx, clients, clients.IdentityStore())
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)
	n := len(resources)
	var failures []Failure
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
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
			setWave2Finding(&result, keyID, kmsCodePublicPolicy, "kms", publicPolicyRows(ex))

		}
		mu.Unlock()
		out, err := clients.KMS.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{
			KeyId: aws.String(keyID),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			if IsAccessDenied(err) {
				// AWS-managed keys: skip silently without marking truncated
				return
			}
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
			setWave2Finding(&result, keyID, kmsCodeRotationDisabled, "kms", nil)
		}
	})
	return result, AggregateFailures("key policy and rotation", failures, n)
}

// kmsKeyPolicyIsPublic reads a key's default policy and returns the exposure
// when it grants a wildcard principal. AWS-managed keys are skipped: AWS
// owns their policy and the operator cannot change it. A policy that cannot
// be read or parsed leaves the key unknown, never healthy; the error that made
// it unknown is returned so the caller can record it.
func kmsKeyPolicyIsPublic(ctx context.Context, api KMSGetKeyPolicyAPI, r resource.Resource, ownAccount string) (ex iampolicy.Exposure, public bool, err error) {
	if api == nil || kmsKeyIsAWSManaged(r) || kmsKeyIsGoingAway(r) {
		return ex, false, nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*kms.GetKeyPolicyOutput, error) {
		return api.GetKeyPolicy(ctx, &kms.GetKeyPolicyInput{
			KeyId:      aws.String(r.ID),
			PolicyName: aws.String("default"),
		})
	})
	if err != nil {
		return ex, false, err
	}
	if out == nil || out.Policy == nil {
		// No policy attached is a complete answer, not a gap.
		return ex, false, nil
	}
	doc, perr := iampolicy.Parse(*out.Policy)
	if perr != nil {
		return ex, false, perr
	}
	ex = iampolicy.Evaluate(doc, ownAccount)
	return ex, ex.Public, nil
}

// kmsKeyIsGoingAway reports a key already scheduled for deletion. The
// operator's move on such a key is to wait or cancel the deletion, so a
// posture row about its policy would outlive the key it describes.
func kmsKeyIsGoingAway(r resource.Resource) bool {
	meta, ok := r.RawStruct.(kmstypes.KeyMetadata)
	if !ok {
		return false
	}
	return meta.KeyState == kmstypes.KeyStatePendingDeletion ||
		meta.KeyState == kmstypes.KeyStatePendingReplicaDeletion
}

// kmsKeyIsAWSManaged reads KeyManager off the DescribeKey metadata the
// fetcher stashed. FetchKMSKeysPage filters AWS-managed keys out, but
// FetchKMSKeysByIDs does not, so a drilled-into key can still arrive here.
func kmsKeyIsAWSManaged(r resource.Resource) bool {
	meta, ok := r.RawStruct.(kmstypes.KeyMetadata)
	return ok && meta.KeyManager == kmstypes.KeyManagerTypeAws
}
