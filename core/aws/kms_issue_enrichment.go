// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// kms_issue_enrichment.go — Wave 2 issue enrichment for the kms resource type.
package aws

import (
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// kms canonical FindingCodes.
const (
	kmsCodeRotationDisabled   domain.FindingCode = "kms.rotation-disabled"
	kmsCodePublicPolicy       domain.FindingCode = "kms.public-policy"
	kmsCodeCrossAccountPolicy domain.FindingCode = "kms.cross-account-policy"
)

// EnrichKMSRotation calls GetKeyRotationStatus for each customer-managed key (cap EnrichmentCap)
// and returns a Finding when key rotation is not enabled.
// Severity is "~" (informational) — rotation-disabled is an informational
// finding, not a "!" issue.
// A key of a kind the findings do not apply to is not read. A per-key error
// marks the key not inspected.
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
		return rowAppliesAny("kms", r, kmsCodeRotationDisabled, kmsCodePublicPolicy, kmsCodeCrossAccountPolicy)
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
		if keyPolicyAPI != nil && rowAppliesAny("kms", r, kmsCodePublicPolicy, kmsCodeCrossAccountPolicy) {
			policy, policyErr := kmsKeyPolicy(ctx, keyPolicyAPI, keyID)
			mu.Lock()
			if policyErr != nil {
				MarkSkipped(&result, keyID, &failures, policyErr)
			} else {
				readPolicy("kms", policy, ownAccount, r.Fields["arn"]).raise(&result, keyID)
			}
			mu.Unlock()
		}
		if !catalog.AppliesToRow("kms", kmsCodeRotationDisabled, r) {
			return
		}
		if kmsKind(r) == kmsKindManualRotation {
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

func kmsKeyPolicy(ctx context.Context, api KMSGetKeyPolicyAPI, keyID string) (string, error) {
	out, err := api.GetKeyPolicy(ctx, &kms.GetKeyPolicyInput{
		KeyId:      aws.String(keyID),
		PolicyName: aws.String("default"),
	})
	if err != nil {
		return "", err
	}
	if out == nil {
		return "", nil
	}
	return aws.ToString(out.Policy), nil
}
