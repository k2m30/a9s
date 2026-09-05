// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// policy_findings.go owns the vocabulary every policy-derived finding shares:
// the one definition of "an AWS-managed policy that hands out
// administrator-equivalent power" (so the role, user and group
// ListAttached*Policies sweeps can never disagree), and the supporting rows
// a public or cross-account resource policy renders.
package aws

import (
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
)

// adminManagedPolicyARNs are the AWS-managed policies whose attachment makes
// the principal effectively an account administrator.
var adminManagedPolicyARNs = []string{ //nolint:gochecknoglobals // static AWS-managed ARN set
	"arn:aws:iam::aws:policy/AdministratorAccess",
	"arn:aws:iam::aws:policy/PowerUserAccess",
}

// adminAttachedPolicyName returns the name of the first admin-equivalent
// managed policy in the attachment list, or "" when none is attached.
func adminAttachedPolicyName(attached []iamtypes.AttachedPolicy) string {
	for _, p := range attached {
		if slices.Contains(adminManagedPolicyARNs, aws.ToString(p.PolicyArn)) {
			name := aws.ToString(p.PolicyName)
			if name == "" {
				name = aws.ToString(p.PolicyArn)
			}
			return name
		}
	}
	return ""
}

// adminAttachedDetail is the S5 sentence stamped on every admin-attached
// finding, whatever principal carries it.
const adminAttachedDetail = "This principal is attached to an AWS-managed policy that grants " +
	"administrator-equivalent access, so anything it can be used for it can be used for everything. " +
	"Replace the managed policy with a scoped policy covering only the actions this principal needs."

// adminAttachedRows is the supporting Attention row shared by the role, user
// and group admin-attached findings.
func adminAttachedRows(policyName string) []domain.DetailRow {
	return []domain.DetailRow{{Label: "Policy", Value: policyName, Tier: "~"}}
}

// publicPolicyRows renders the supporting rows for a "policy is open to
// anyone" finding, shared by every resource-policy check so the KMS key, the
// secret and the next one all describe the exposure identically.
func publicPolicyRows(ex iampolicy.Exposure) []domain.DetailRow {
	rows := []domain.DetailRow{{Label: "Principal", Value: "*", Tier: "!"}}
	if len(ex.PublicActions) > 0 {
		rows = append(rows, domain.DetailRow{
			Label: "Actions", Value: strings.Join(ex.PublicActions, ", "), Tier: "!",
		})
	}
	return rows
}

// crossAccountPolicyRows renders the supporting rows for a "policy grants
// another account" finding.
func crossAccountPolicyRows(ex iampolicy.Exposure) []domain.DetailRow {
	return []domain.DetailRow{{Label: "Accounts", Value: strings.Join(ex.CrossAccount, ", "), Tier: "~"}}
}
