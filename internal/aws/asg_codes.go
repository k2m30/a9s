// asg_codes.go — canonical FindingCode constants for the asg resource type.
// Phase 03 PR-03b. The fetcher writes Findings using these codes; the
// ASG Color func reads Findings[0].Severity to color rows.
package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	// CodeASGStateDeleting — Auto Scaling Group has "Delete in progress" status.
	// Severity: SevWarn (transitional, lifecycle terminal).
	CodeASGStateDeleting domain.FindingCode = "asg.state.deleting"
)
