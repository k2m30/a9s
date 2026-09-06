// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ec2_codes.go — canonical FindingCode constants for the ec2 resource type.
// The fetcher writes Findings using these codes; the
// EC2 Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeEC2StatePending — instance is in the "pending" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeEC2StatePending domain.FindingCode = "ec2.state.pending"

	// CodeEC2StateStopping — instance is in the "stopping" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeEC2StateStopping domain.FindingCode = "ec2.state.stopping"

	// CodeEC2StateStopped — instance is "stopped" via user-initiated shutdown
	// or default reason. Severity: SevWarn (intentional, recoverable).
	CodeEC2StateStopped domain.FindingCode = "ec2.state.stopped"

	// CodeEC2StateStoppedServer — instance is "stopped" via a Server.* reason
	// code (AWS-initiated, e.g. capacity, spot interruption).
	// Severity: SevBroken.
	CodeEC2StateStoppedServer domain.FindingCode = "ec2.state.stopped.server"

	// CodeEC2StateShuttingDown — instance is in the "shutting-down" lifecycle
	// state (transitional, en route to terminated). Severity: SevWarn.
	CodeEC2StateShuttingDown domain.FindingCode = "ec2.state.shutting-down"

	// CodeEC2StateTerminated — instance is in the "terminated" lifecycle
	// state (permanent, no further action possible). Severity: SevDim.
	CodeEC2StateTerminated domain.FindingCode = "ec2.state.terminated"

	// CodeEC2IMDSv1Allowed — instance metadata is reachable without a
	// session token (MetadataOptions.HttpTokens == optional).
	// Severity: SevWarn.
	CodeEC2IMDSv1Allowed domain.FindingCode = "ec2.imdsv1-allowed"

	// CodeEC2PublicIP — instance holds a routable public IPv4 address.
	// Severity: SevWarn.
	CodeEC2PublicIP domain.FindingCode = "ec2.public-ip"
)

// S5 operator sentences stamped onto Finding.Detail for the posture codes above.
