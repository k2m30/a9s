package aws

import "github.com/k2m30/a9s/v3/core/domain"

// ASG scaling-activity findings emitted by FetchAsgActivities. Failed is a
// broken terminal state; Cancelled (activity superseded/cancelled by AWS or
// the operator) is a warn — every other StatusCode is normal in-flight or
// successful lifecycle and emits no finding.
const (
	CodeAsgActivityFailed    domain.FindingCode = "asg-activity.broken.failed"
	CodeAsgActivityCancelled domain.FindingCode = "asg-activity.warn.cancelled"
)
