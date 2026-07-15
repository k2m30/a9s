// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eks_codes.go — canonical FindingCode constants for the eks resource type.
// The fetcher writes Findings using these codes; the
// EKS Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

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

	// CodeEKSHealthIssue — Health.Issues[] is non-empty on an otherwise-healthy
	// (non-FAILED/CREATING/UPDATING) cluster (docs/resources/eks.md §3.2).
	// Health is tracked independently of lifecycle state. Severity: SevWarn —
	// colorEKSCluster (catalog_containers.go) ranks a bare health issue below
	// the FAILED/CREATING/UPDATING lifecycle states, matching
	// qa_eks_color_test.go's active_with_issues -> ColorWarning contract.
	CodeEKSHealthIssue domain.FindingCode = "eks.health-issue"
)
