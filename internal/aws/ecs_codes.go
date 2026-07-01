// ecs_codes.go — canonical FindingCode constants for the ecs resource type.
// The fetcher writes Findings using these codes; the
// ecs Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeECSStateProvisioning — cluster is in the "PROVISIONING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeECSStateProvisioning domain.FindingCode = "ecs.state.provisioning"

	// CodeECSStateDeprovisioning — cluster is in the "DEPROVISIONING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeECSStateDeprovisioning domain.FindingCode = "ecs.state.deprovisioning"

	// CodeECSStateFailed — cluster has failed.
	// Severity: SevBroken.
	CodeECSStateFailed domain.FindingCode = "ecs.state.failed"

	// CodeECSStateInactive — cluster is inactive (non-recoverable without recreation).
	// Severity: SevBroken.
	CodeECSStateInactive domain.FindingCode = "ecs.state.inactive"
)
