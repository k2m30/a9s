// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ebs_codes.go — canonical FindingCode constants for the ebs and ebs-snap resource types.
// The fetcher writes Findings using these codes; the
// EBS Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

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

// CodeEBSNotInBackupPlan — no backup plan selection matches this volume, so
// nothing is scheduled to copy it. Severity: SevWarn.
const CodeEBSNotInBackupPlan domain.FindingCode = "ebs.not-in-backup-plan"

// ebsNotInBackupPlanDetail is the S5 operator sentence for it.
const ebsNotInBackupPlanDetail = "No backup plan selects this volume, so nothing is scheduled to copy it and a deletion is final. Add it to a plan by ARN, or give it a tag one of your plans already selects on."

// CodeEBSNoSnapshot — an attached volume with no snapshot behind it, so there
// is nothing to restore from. Severity: SevWarn.
const CodeEBSNoSnapshot domain.FindingCode = "ebs.no-snapshot"

// ebsNoSnapshotDetail is the S5 operator sentence for it.
//
//nolint:unused // wired with the check itself in the implementation round.
const ebsNoSnapshotDetail = "This volume is attached and in use, and no snapshot of it exists, so there is no point to restore from. Take one, or put the volume in a backup plan that will."
