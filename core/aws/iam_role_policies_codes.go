// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

// IAM role-policy findings emitted by FetchRolePolicies. An attached
// AWS-managed administrator policy and an attached broad-power policy are
// two findings, so the Status cell says which of the two the row carries;
// inline policies classify as dim to distinguish them from managed-policy
// rows.
const (
	CodeRolePolicyAdministrator domain.FindingCode = "role-policy.broken.administrator"
	CodeRolePolicyBroadPower    domain.FindingCode = "role-policy.broken.broad_power"
	CodeRolePolicyInline        domain.FindingCode = "role-policy.dim.inline"
)
