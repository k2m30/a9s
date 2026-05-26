// kms_issue_enrichment.go — Wave 2 issue enrichment for the kms resource type.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// kms canonical FindingCodes.
const (
	kmsCodeRotationDisabled domain.FindingCode = "kms.rotation-disabled"
)

// EnrichKMSRotation calls GetKeyRotationStatus for each customer-managed key (cap EnrichmentCap)
// and returns a Finding when key rotation is not enabled.
// Severity is "~" (informational per CIS KMS.1); IssueCount counts rotation-disabled findings.
// AWS-managed keys reject GetKeyRotationStatus with AccessDeniedException — that error is
// silently skipped without marking Truncated. Other per-key errors set Truncated=true.
func EnrichKMSRotation(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.KMS == nil {
		return result, nil
	}
	truncated := len(resources) > EnrichmentCap
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		keyID := r.ID
		if keyID == "" {
			continue
		}
		out, err := clients.KMS.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{
			KeyId: aws.String(keyID),
		})
		if err != nil {
			code, _, _ := ClassifyAWSError(err)
			if code == "AccessDeniedException" || code == "AccessDenied" {
				// AWS-managed keys: skip silently without marking truncated
				continue
			}
			// Any other error: skip this key but signal incomplete data via truncated
			truncated = true
			result.TruncatedIDs[r.ID] = true
			continue
		}
		rotationVal := "false"
		if out.KeyRotationEnabled {
			rotationVal = "true"
		}
		result.FieldUpdates[keyID] = map[string]string{
			"rotation_enabled": rotationVal,
		}
		if !out.KeyRotationEnabled {
			setWave2Finding(&result, keyID, kmsCodeRotationDisabled, "key rotation disabled (CIS KMS.1)", "~", "kms", nil)
		}
	}
	result.IssueCount = 0
	result.Truncated = truncated
	return result, nil
}
