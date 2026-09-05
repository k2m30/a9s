// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// asg_codes.go — canonical FindingCode constants for the asg resource type.
// The fetcher writes Findings using these codes; the
// ASG Color func reads wave1 Findings (Source == "wave1") to color rows.
package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	// CodeASGStateDeleting — Auto Scaling Group has "Delete in progress" status.
	// Severity: SevWarn (transitional, lifecycle terminal).
	CodeASGStateDeleting domain.FindingCode = "asg.state.deleting"

	// CodeASGUnderprovisioned — fewer instances in service than MinSize.
	// Severity: SevBroken.
	CodeASGUnderprovisioned domain.FindingCode = "asg.instances.underprovisioned"

	// CodeASGUnhealthyInstances — at least one instance reports Unhealthy.
	// Severity: SevWarn.
	CodeASGUnhealthyInstances domain.FindingCode = "asg.instances.unhealthy"

	// CodeASGScalingSuspended — SuspendedProcesses includes a Launch,
	// Terminate, or HealthCheck process, meaning the ASG cannot scale or
	// replace unhealthy instances on its own. Severity: SevWarn.
	CodeASGScalingSuspended domain.FindingCode = "asg.scaling.suspended"

	// CodeASGLegacyLaunchConfig — the group launches from a launch
	// configuration rather than a launch template. Severity: SevWarn.
	CodeASGLegacyLaunchConfig domain.FindingCode = "asg.launch-config.legacy"

	// CodeASGSingleAZ — the group spans fewer than two availability zones.
	// Severity: SevWarn.
	CodeASGSingleAZ domain.FindingCode = "asg.single-az"

	// CodeASGNoELBHealthCheck — the group is attached to a load balancer or
	// target group but still decides health from EC2 status checks alone.
	// Severity: SevWarn.
	CodeASGNoELBHealthCheck domain.FindingCode = "asg.no-elb-health-check"
)

// S5 operator sentences for the codes above.
const (
	asgLegacyLaunchConfigDetail = "The group launches from a launch configuration, an immutable legacy resource AWS no longer develops — it cannot carry IMDSv2 defaults, newer instance types, or versioned edits. Copy it to a launch template and point the group at that."
	asgSingleAZDetail           = "Every instance in this group sits in one availability zone, so a single zone failure takes the whole group down. Add subnets from at least one more zone to the group's VPCZoneIdentifier."
	asgNoELBHealthCheckDetail   = "The group is behind a load balancer but only watches EC2 status checks, so an instance whose application has stopped answering stays in service. Set the group's health check type to the load balancer's."
)
