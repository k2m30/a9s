// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// kms_issue_enrichment.go — Wave 2 issue enrichment for the kms resource type.
package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// kms canonical FindingCodes.
const (
	kmsCodeRotationDisabled domain.FindingCode = "kms.rotation-disabled"
)

// EnrichKMSRotation calls GetKeyRotationStatus for each customer-managed key (cap EnrichmentCap)
// and returns a Finding when key rotation is not enabled.
// Severity is "~" (informational); IssueCount stays 0 — rotation-disabled is an
// informational finding, not a "!" issue.
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
	n := min(len(resources), EnrichmentCap)
	var mu sync.Mutex
	_ = ForEachParallel(ctx, n, EnrichmentParallelism, func(i int) {
		r := resources[i]
		keyID := r.ID
		if keyID == "" {
			return
		}
		out, err := clients.KMS.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{
			KeyId: aws.String(keyID),
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			code, _, _ := ClassifyAWSError(err)
			if code == "AccessDeniedException" || code == "AccessDenied" {
				// AWS-managed keys: skip silently without marking truncated
				return
			}
			// Any other error: skip this key but signal incomplete data via truncated
			result.TruncatedIDs[r.ID] = true
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
			setWave2Finding(&result, keyID, kmsCodeRotationDisabled, "key rotation disabled", "~", "kms", nil, "")
		}
	})
	result.IssueCount = 0
	// "~"-only enrichment: EnrichmentCap bounds informational coverage, never the issue count — so it never lower-bounds the issue badge (cf. EnrichSESAccount).
	result.Truncated = false
	return result, nil
}
