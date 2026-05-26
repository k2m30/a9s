package aws

import "github.com/k2m30/a9s/v3/internal/domain"

// IAM role-policy findings emitted by FetchRolePolicies. Over-privileged
// managed policies (AdministratorAccess, PowerUserAccess) classify as broken
// to highlight the security risk; inline policies classify as dim to
// distinguish them from managed-policy rows.
const (
	CodeRolePolicyOverPrivileged domain.FindingCode = "role-policy.broken.over_privileged"
	CodeRolePolicyInline         domain.FindingCode = "role-policy.dim.inline"
)
