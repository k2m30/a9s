// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_svc_codes.go — canonical FindingCode constants for the ecs-svc resource type.
// The fetcher writes Findings using these codes; the
// ecs-svc Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeECSSvcStateInactive — service is inactive (terminal; non-recoverable without recreation).
	CodeECSSvcStateInactive domain.FindingCode = "ecs-svc.state.inactive"

	// CodeECSSvcStateDraining — service is draining connections.
	CodeECSSvcStateDraining domain.FindingCode = "ecs-svc.state.draining"

	// CodeECSSvcNoTasksRunning — the service wants tasks but none are running.
	CodeECSSvcNoTasksRunning domain.FindingCode = "ecs-svc.tasks.none-running"

	// CodeECSSvcTasksBelowDesired — fewer tasks are running than the service asks for.
	CodeECSSvcTasksBelowDesired domain.FindingCode = "ecs-svc.tasks.below-desired"
)

// S5 operator sentences for the task-count findings.
