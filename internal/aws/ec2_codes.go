// ec2_codes.go — canonical FindingCode constants for the ec2 resource type.
// Phase 03 PR-03b. The fetcher writes Findings using these codes; the
// EC2 Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

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
)
