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
	CodeECSTaskStateProvisioning domain.FindingCode = "ecs-task.state.provisioning"

	// CodeECSTaskStatePending — task is in the "PENDING" lifecycle state.
	CodeECSTaskStatePending domain.FindingCode = "ecs-task.state.pending"

	// CodeECSTaskStateActivating — task is in the "ACTIVATING" lifecycle state.
	CodeECSTaskStateActivating domain.FindingCode = "ecs-task.state.activating"

	// CodeECSTaskStateDeactivating — task is in the "DEACTIVATING" lifecycle state.
	CodeECSTaskStateDeactivating domain.FindingCode = "ecs-task.state.deactivating"

	// CodeECSTaskStateStopping — task is in the "STOPPING" lifecycle state.
	CodeECSTaskStateStopping domain.FindingCode = "ecs-task.state.stopping"

	// CodeECSTaskStateDeprovisioning — task is in the "DEPROVISIONING" lifecycle state.
	CodeECSTaskStateDeprovisioning domain.FindingCode = "ecs-task.state.deprovisioning"

	// CodeECSTaskStateStopped — task is in the "STOPPED" lifecycle state with a
	// a routine or empty stop code (a normal, non-error stop).
	CodeECSTaskStateStopped domain.FindingCode = "ecs-task.state.stopped"

	// CodeECSTaskStopCodeFailed — task is STOPPED with a stop code that names
	// a failure rather than the platform doing its job: it failed to start,
	// or its essential container exited.
	CodeECSTaskStopCodeFailed domain.FindingCode = "ecs-task.stop-code.failed"

	// CodeECSTaskHealthUnhealthy — task's container health check reports
	// UNHEALTHY while the task itself is still RUNNING.
	CodeECSTaskHealthUnhealthy domain.FindingCode = "ecs-task.health.unhealthy"

	CodeECSTaskStateDeleted      domain.FindingCode = "ecs-task.state.deleted"
	CodeECSTaskStateUnrecognised domain.FindingCode = "ecs-task.state.unrecognised"
)

// ecsTaskStatuses is the task lastStatus domain of the Amazon ECS Developer
// Guide, "Amazon ECS task lifecycle"; the SDK types it as a plain string.
// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-lifecycle-explanation.html
var ecsTaskStatuses = []string{ //nolint:gochecknoglobals // static AWS status domain
	"PROVISIONING", "PENDING", "ACTIVATING", "RUNNING", "DEACTIVATING",
	"STOPPING", "DEPROVISIONING", "STOPPED", "DELETED",
}

// ecsTaskFindings is a task's state finding beside its container health and
// its stop code, a second axis AWS sets once a stop starts; the stop-code
// FindingDef declares every class, a stopped task's included.
func ecsTaskFindings(status, stopCode, healthStatus string) []domain.Finding {
	var posture []domain.Finding
	if strings.EqualFold(healthStatus, string(ecstypes.HealthStatusUnhealthy)) {
		posture = append(posture, wave1Finding(CodeECSTaskHealthUnhealthy))
	}
	if stopCode != "" && !ecsStopCodeIsRoutine(stopCode) {
		posture = append(posture, wave1Finding(CodeECSTaskStopCodeFailed, domain.HumanizeStatusPhrase(stopCode)))
	}
	return withState("ecs-task", status, posture)
}

// ecsStopCodeIsRoutine reports whether a StopCode names the platform doing its
// job rather than the task failing. The scheduler stops tasks on every
// deployment and scale-in, Spot reclaims capacity it warned about, and a
// termination notice is the instance going away — none of them says anything
// about the workload, and colouring them broken puts a red row on a normal
// deployment.
func ecsStopCodeIsRoutine(stopCode string) bool {
	switch stopCode {
	case string(ecstypes.TaskStopCodeUserInitiated),
		string(ecstypes.TaskStopCodeServiceSchedulerInitiated),
		string(ecstypes.TaskStopCodeSpotInterruption),
		string(ecstypes.TaskStopCodeTerminationNotice):
		return true
	}
	return false
}

func ecsTaskWave1Findings(status string) []domain.Finding {
	switch status {
	case "PROVISIONING":
		return []domain.Finding{wave1Finding(CodeECSTaskStateProvisioning)}
	case "PENDING":
		return []domain.Finding{wave1Finding(CodeECSTaskStatePending)}
	case "ACTIVATING":
		return []domain.Finding{wave1Finding(CodeECSTaskStateActivating)}
	case "DEACTIVATING":
		return []domain.Finding{wave1Finding(CodeECSTaskStateDeactivating)}
	case "STOPPING":
		return []domain.Finding{wave1Finding(CodeECSTaskStateStopping)}
	case "DEPROVISIONING":
		return []domain.Finding{wave1Finding(CodeECSTaskStateDeprovisioning)}
	}
	return nil
}

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
	// rebuilt from the on-disk type cache, which may hold the SDK's uppercase
	// spelling. Reading it strictly would drop the finding on those rows: the
	// cell would say the task is unhealthy while the row coloured green.
	if strings.EqualFold(healthStatus, string(ecstypes.HealthStatusUnhealthy)) {
		return []domain.Finding{wave1Finding(CodeECSTaskHealthUnhealthy)}
	}
	if status == "STOPPED" {
		if stopCode != "" && !ecsStopCodeIsRoutine(stopCode) {
			return []domain.Finding{wave1Finding(CodeECSTaskStopCodeFailed, domain.HumanizeStatusPhrase(stopCode))}
		}
		return []domain.Finding{wave1Finding(CodeECSTaskStateStopped)}
	}
	return ecsTaskWave1Findings(status)
}
