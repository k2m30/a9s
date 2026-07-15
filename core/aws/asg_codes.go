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
)
