// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_task_codes.go — canonical FindingCode constants for the ecs-task resource type.
// The fetcher writes Findings using these codes; the
// ecs-task Color func reads them (any Finding, wave1 or wave2) to color rows.
package aws

import (
	"strings"

	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/domain"
)

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

	// CodeECSTaskStateStopped — task is in the "STOPPED" lifecycle state with a
	// user-initiated or empty stop code (a normal, non-error stop).
	// Severity: SevDim.
	CodeECSTaskStateStopped domain.FindingCode = "ecs-task.state.stopped"

	// CodeECSTaskStopCodeFailed — task is STOPPED with a stop code other than
	// UserInitiated (AWS-initiated stop, e.g. task failed to start or its
	// essential container exited).
	// Severity: SevBroken.
	CodeECSTaskStopCodeFailed domain.FindingCode = "ecs-task.stop-code.failed"

	// CodeECSTaskHealthUnhealthy — task's container health check reports
	// UNHEALTHY while the task itself is still RUNNING.
	// Severity: SevBroken.
	CodeECSTaskHealthUnhealthy domain.FindingCode = "ecs-task.health.unhealthy"
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

// ecsTaskStructuralFindings returns the wave1 Finding slice for the ecs-task
// top-level resource type, mirroring colorECSTask's own precedence: a
// RUNNING task with an UNHEALTHY container health check is broken outright;
// a STOPPED task with a non-UserInitiated stop code is broken; any other
// STOPPED task is a normal, dim lifecycle stop; everything else falls
// through to the shared transitional-state findings.
// ecsTaskHealthWords renders a container health status as the word an operator
// says. HEALTHY is the SDK's spelling of the enum; a task AWS reports no
// health for at all stays empty, which is not the same as reporting unknown.
//
// Both task fetchers route through here and so does the predicate below, so
// the column and the finding can never disagree about what the string is.
func ecsTaskHealthWords(status ecstypes.HealthStatus) string {
	return strings.ToLower(string(status))
}

func ecsTaskStructuralFindings(status, stopCode, healthStatus string) []domain.Finding {
	// Compared case-insensitively because this predicate also runs over rows
	// rebuilt from the on-disk type cache, and a row cached before the health
	// column moved to words still holds the SDK's uppercase spelling. Reading
	// it strictly would retire the finding on exactly those rows: the cell
	// would say the task is unhealthy while the row coloured green.
	if strings.EqualFold(healthStatus, string(ecstypes.HealthStatusUnhealthy)) {
		return []domain.Finding{{Code: CodeECSTaskHealthUnhealthy, Phrase: "unhealthy", Severity: domain.SevBroken, Source: "wave1"}}
	}
	// Not ecsTaskGone: that predicate answers "has teardown started", which
	// is true for the transitional states below and would swallow their own
	// lifecycle findings. This branch asks the narrower question of whether
	// the task has actually stopped, which is what picks stop-code over dim.
	if status == "STOPPED" {
		if stopCode != "" && stopCode != "UserInitiated" {
			return []domain.Finding{{Code: CodeECSTaskStopCodeFailed, Phrase: "stopped: " + stopCode, Severity: domain.SevBroken, Source: "wave1"}}
		}
		return []domain.Finding{{Code: CodeECSTaskStateStopped, Phrase: "stopped", Severity: domain.SevDim, Source: "wave1"}}
	}
	return ecsTaskWave1Findings(status)
}
