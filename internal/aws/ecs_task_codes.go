// ecs_task_codes.go — canonical FindingCode constants for the ecs-task resource type.
// Phase 03 PR-03c. The fetcher writes Findings using these codes; the
// ecs-task Color func reads wave1 Findings (Source == "wave1") to color rows.
//
// NOTE: RUNNING and STOPPED are lifecycle states with no Finding emitted.
// STOPPED's meaningful information is carried by stop_code and handled
// structurally in the Color func.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeECSTaskStateProvisioning — task is in the "PROVISIONING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeECSTaskStateProvisioning domain.FindingCode = "ecs-task.state.provisioning"

	// CodeECSTaskStatePending — task is in the "PENDING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeECSTaskStatePending domain.FindingCode = "ecs-task.state.pending"

	// CodeECSTaskStateActivating — task is in the "ACTIVATING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeECSTaskStateActivating domain.FindingCode = "ecs-task.state.activating"

	// CodeECSTaskStateDeactivating — task is in the "DEACTIVATING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeECSTaskStateDeactivating domain.FindingCode = "ecs-task.state.deactivating"

	// CodeECSTaskStateStopping — task is in the "STOPPING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeECSTaskStateStopping domain.FindingCode = "ecs-task.state.stopping"

	// CodeECSTaskStateDeprovisioning — task is in the "DEPROVISIONING" lifecycle state.
	// Severity: SevWarn (transitional).
	CodeECSTaskStateDeprovisioning domain.FindingCode = "ecs-task.state.deprovisioning"
)
