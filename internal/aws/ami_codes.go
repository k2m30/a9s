// ami_codes.go — canonical FindingCode constants for the ami resource type.
// Phase 03 PR-03b. The fetcher writes Findings using these codes; the
// AMI Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeAMIStatePending — AMI is in the "pending" or "transient" state.
	// Severity: SevWarn (transitional).
	CodeAMIStatePending domain.FindingCode = "ami.state.pending"

	// CodeAMIStateFailed — AMI is in the "failed", "error", or "invalid" state.
	// Severity: SevBroken.
	CodeAMIStateFailed domain.FindingCode = "ami.state.failed"
)
