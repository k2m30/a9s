// ng_codes.go — canonical FindingCode constants for the ng resource type.
// Phase 03 PR-03c. The fetcher writes Findings using these codes; the
// ng Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeNGStateCreating — node group is in the "CREATING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeNGStateCreating domain.FindingCode = "ng.state.creating"

	// CodeNGStateUpdating — node group is in the "UPDATING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeNGStateUpdating domain.FindingCode = "ng.state.updating"

	// CodeNGStateDeleting — node group is in the "DELETING" lifecycle state.
	// Severity: SevWarn (transitional; non-terminal from operator's perspective).
	CodeNGStateDeleting domain.FindingCode = "ng.state.deleting"

	// CodeNGStateCreateFailed — node group creation failed.
	// Severity: SevBroken.
	CodeNGStateCreateFailed domain.FindingCode = "ng.state.create-failed"

	// CodeNGStateDeleteFailed — node group deletion failed.
	// Severity: SevBroken.
	CodeNGStateDeleteFailed domain.FindingCode = "ng.state.delete-failed"

	// CodeNGStateDegraded — node group is degraded.
	// Severity: SevBroken.
	CodeNGStateDegraded domain.FindingCode = "ng.state.degraded"
)
