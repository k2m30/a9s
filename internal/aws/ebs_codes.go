// ebs_codes.go — canonical FindingCode constants for the ebs and ebs-snap resource types.
// The fetcher writes Findings using these codes; the
// EBS Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeEBSStateCreating — EBS volume is in the "creating" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeEBSStateCreating domain.FindingCode = "ebs.state.creating"

	// CodeEBSStateError — EBS volume is in the "error" state.
	// Severity: SevBroken.
	CodeEBSStateError domain.FindingCode = "ebs.state.error"

	// CodeEBSSnapStatePending — EBS snapshot is in the "pending" state.
	// Severity: SevWarn (transitional).
	CodeEBSSnapStatePending domain.FindingCode = "ebs-snap.state.pending"

	// CodeEBSSnapStateError — EBS snapshot is in the "error", "recoverable", or
	// "recovering" state. Severity: SevBroken.
	CodeEBSSnapStateError domain.FindingCode = "ebs-snap.state.error"
)
