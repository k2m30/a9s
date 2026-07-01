// ecs_svc_codes.go — canonical FindingCode constants for the ecs-svc resource type.
// The fetcher writes Findings using these codes; the
// ecs-svc Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeECSSvcStateInactive — service is inactive (terminal; non-recoverable without recreation).
	// Severity: SevBroken.
	CodeECSSvcStateInactive domain.FindingCode = "ecs-svc.state.inactive"

	// CodeECSSvcStateDraining — service is draining connections.
	// Severity: SevWarn (transitional).
	CodeECSSvcStateDraining domain.FindingCode = "ecs-svc.state.draining"
)
