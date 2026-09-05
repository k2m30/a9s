// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeVPCEStatePendingAcceptance domain.FindingCode = "vpce.state.pending_acceptance"
	CodeVPCEStatePending           domain.FindingCode = "vpce.state.pending"
	CodeVPCEStateDeleting          domain.FindingCode = "vpce.state.deleting"
	CodeVPCEStateFailed            domain.FindingCode = "vpce.state.failed"
	CodeVPCEStateRejected          domain.FindingCode = "vpce.state.rejected"
	CodeVPCEStateExpired           domain.FindingCode = "vpce.state.expired"
	CodeVPCEStatePartial           domain.FindingCode = "vpce.state.partial"
	CodeVPCEStateDeleted           domain.FindingCode = "vpce.state.deleted"

	// CodeVPCEPolicyOpen marks an endpoint whose policy grants full access to
	// every principal — the policy AWS attaches when none is supplied.
	CodeVPCEPolicyOpen domain.FindingCode = "vpce.policy-open"
)

// VPCEPolicyOpenPhrase is the S4 status phrase for CodeVPCEPolicyOpen.
const VPCEPolicyOpenPhrase = "endpoint policy open to any principal"

// VPCEPolicyOpenDetail is the S5 operator sentence for CodeVPCEPolicyOpen.
const VPCEPolicyOpenDetail = "The endpoint policy grants every action to every principal, so any identity that " +
	"can reach this endpoint can use it to talk to resources in other accounts. Replace it with a policy naming " +
	"the principals and resources this VPC is allowed to reach."
