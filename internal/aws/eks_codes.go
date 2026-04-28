// eks_codes.go — canonical FindingCode constants for the eks resource type.
// Phase 03 PR-03b. The fetcher writes Findings using these codes; the
// EKS Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeEKSStateCreating — cluster is in the "CREATING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeEKSStateCreating domain.FindingCode = "eks.state.creating"

	// CodeEKSStateUpdating — cluster is in the "UPDATING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeEKSStateUpdating domain.FindingCode = "eks.state.updating"

	// CodeEKSStateFailed — cluster is in the "FAILED" lifecycle state.
	// Severity: SevBroken.
	CodeEKSStateFailed domain.FindingCode = "eks.state.failed"
)
