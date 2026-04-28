// subnet_codes.go — canonical FindingCode constants for the subnet resource type.
// The fetcher writes Findings using these codes; the Subnet Color func reads
// wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeSubnetStatePending — subnet is in the "pending" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeSubnetStatePending domain.FindingCode = "subnet.state.pending"

	// CodeSubnetStateUnavailable — subnet is unavailable.
	// Severity: SevBroken.
	CodeSubnetStateUnavailable domain.FindingCode = "subnet.state.unavailable"

	// CodeSubnetStateFailed — subnet has failed.
	// Severity: SevBroken.
	CodeSubnetStateFailed domain.FindingCode = "subnet.state.failed"

	// CodeSubnetStateFailedInsufficientCapacity — subnet failed due to
	// insufficient capacity.
	// Severity: SevBroken.
	CodeSubnetStateFailedInsufficientCapacity domain.FindingCode = "subnet.state.failed-insufficient-capacity"
)
