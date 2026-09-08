// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// asg_codes.go — canonical FindingCode constants for the asg resource type.
// The fetcher writes Findings using these codes; the
// ASG Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeASGStateDeleting — Auto Scaling Group has "Delete in progress" status.
	CodeASGStateDeleting domain.FindingCode = "asg.state.deleting"

	// CodeASGUnderprovisioned — fewer instances in service than MinSize.
	CodeASGUnderprovisioned domain.FindingCode = "asg.instances.underprovisioned"

	// CodeASGUnhealthyInstances — at least one instance reports Unhealthy.
	CodeASGUnhealthyInstances domain.FindingCode = "asg.instances.unhealthy"

	// CodeASGScalingSuspended — SuspendedProcesses includes a Launch,
	// Terminate, or HealthCheck process, meaning the ASG cannot scale or
	// replace unhealthy instances on its own. Severity: SevWarn.
	CodeASGScalingSuspended domain.FindingCode = "asg.scaling.suspended"

	// CodeASGLegacyLaunchConfig — the group launches from a launch
	// configuration rather than a launch template. Severity: SevWarn.
	CodeASGLegacyLaunchConfig domain.FindingCode = "asg.launch-config.legacy"

	// CodeASGSingleAZ — the group spans fewer than two availability zones.
	CodeASGSingleAZ domain.FindingCode = "asg.single-az"

	// CodeASGNoELBHealthCheck — the group is attached to a load balancer or
	// target group but still decides health from EC2 status checks alone.
	CodeASGNoELBHealthCheck domain.FindingCode = "asg.no-elb-health-check"
)

// S5 operator sentences for the codes above.
