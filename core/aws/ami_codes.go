// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ami_codes.go — canonical FindingCode constants for the ami resource type.
// The fetcher writes Findings using these codes; the
// AMI Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeAMIStatePending — AMI is in the "pending" or "transient" state.
	// Severity: SevWarn (transitional).
	CodeAMIStatePending domain.FindingCode = "ami.state.pending"

	// CodeAMIStateFailed — AMI is in the "failed", "error", or "invalid" state.
	// Severity: SevBroken.
	CodeAMIStateFailed domain.FindingCode = "ami.state.failed"

	// CodeAMIStateDim — AMI is in the "deregistered" or "disabled" state
	// (no longer launchable). Severity: SevDim.
	CodeAMIStateDim domain.FindingCode = "ami.state.dim"

	// CodeAMIDeprecated — AMI's DeprecationTime has passed; AWS Console no
	// longer recommends it for new launches. Severity: SevWarn.
	CodeAMIDeprecated domain.FindingCode = "ami.deprecated"

	// CodeAMIPublic — the AMI's launch permission includes every AWS
	// account. Severity: SevBroken.
	CodeAMIPublic domain.FindingCode = "ami.public"
)
