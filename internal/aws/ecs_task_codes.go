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

// ecsTaskWave1Findings returns the wave1 Finding slice for an ECS task's
// last_status. Returns nil for terminal/healthy states (RUNNING, STOPPED).
// Shared by ecs_task.go and ecs_svc_tasks.go::convertEcsTask to keep their
// lifecycle classification in lockstep.
func ecsTaskWave1Findings(status string) []domain.Finding {
	switch status {
	case "PROVISIONING":
		return []domain.Finding{{Code: CodeECSTaskStateProvisioning, Phrase: "provisioning", Severity: domain.SevWarn, Source: "wave1"}}
	case "PENDING":
		return []domain.Finding{{Code: CodeECSTaskStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"}}
	case "ACTIVATING":
		return []domain.Finding{{Code: CodeECSTaskStateActivating, Phrase: "activating", Severity: domain.SevWarn, Source: "wave1"}}
	case "DEACTIVATING":
		return []domain.Finding{{Code: CodeECSTaskStateDeactivating, Phrase: "deactivating", Severity: domain.SevWarn, Source: "wave1"}}
	case "STOPPING":
		return []domain.Finding{{Code: CodeECSTaskStateStopping, Phrase: "stopping", Severity: domain.SevWarn, Source: "wave1"}}
	case "DEPROVISIONING":
		return []domain.Finding{{Code: CodeECSTaskStateDeprovisioning, Phrase: "deprovisioning", Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}
