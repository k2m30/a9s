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

	// CodeEBSOrphanUnattached — volume is "available" (unattached) and older
	// than 7 days — billed hourly with no workload. Severity: SevWarn.
	CodeEBSOrphanUnattached domain.FindingCode = "ebs.orphan-unattached"

	// CodeEBSUnencrypted — volume is not encrypted at rest, violating CIS
	// EC2.7. Severity: SevWarn.
	CodeEBSUnencrypted domain.FindingCode = "ebs.encryption.disabled"

	// CodeEBSSnapStatePending — EBS snapshot is in the "pending" state.
	// Severity: SevWarn (transitional).
	CodeEBSSnapStatePending domain.FindingCode = "ebs-snap.state.pending"

	// CodeEBSSnapStateError — EBS snapshot is in the "error", "recoverable", or
	// "recovering" state. Severity: SevBroken.
	CodeEBSSnapStateError domain.FindingCode = "ebs-snap.state.error"

	// CodeEBSSnapUnencrypted — snapshot is not encrypted at rest, violating
	// CIS EC2.1. Severity: SevWarn.
	CodeEBSSnapUnencrypted domain.FindingCode = "ebs-snap.encryption.disabled"

	// CodeEBSSnapAgedAutomated — automated snapshot older than 365 days —
	// billed indefinitely with no retention policy pruning it.
	// Severity: SevWarn.
	CodeEBSSnapAgedAutomated domain.FindingCode = "ebs-snap.aged-automated"

	// CodeEBSSnapOrphan — snapshot's source volume is no longer present in
	// the loaded ebs cache. Severity: SevWarn.
	CodeEBSSnapOrphan domain.FindingCode = "ebs-snap.orphan"
)
