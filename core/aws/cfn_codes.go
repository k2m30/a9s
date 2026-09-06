// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeCFNStackFailed     domain.FindingCode = "cfn.stack.failed"
	CodeCFNStackRollback   domain.FindingCode = "cfn.stack.rollback"
	CodeCFNStackInProgress domain.FindingCode = "cfn.stack.in_progress"
	// CodeCFNStackDeleted — stack is DELETE_COMPLETE. DescribeStacks still
	// reports it for a retention window after deletion.
	CodeCFNStackDeleted domain.FindingCode = "cfn.stack.deleted"
)

const (
	// CodeCFNTerminationProtectionOff — a live, top-level stack has
	// termination protection disabled. Nested stacks (ParentId set) inherit
	// the root stack's protection and are not evaluated.
	CodeCFNTerminationProtectionOff domain.FindingCode = "cfn.termination-protection-off"

	// CodeCFNOutputSecret — a stack output value looks like a credential.
	//nolint:gosec // G101 false positive: a finding code, not a credential
	CodeCFNOutputSecret domain.FindingCode = "cfn.output-secret"
)

// S5 operator sentences: what is wrong, what it exposes, what fixing it takes.
const (
	cfnTerminationProtectionOffDetail = "A single delete call removes this stack and every resource it owns, with no second step to stop an accidental or scripted deletion. Turn on termination protection so the stack must be unprotected deliberately before it can be deleted."

	//nolint:gosec // G101 false positive: operator prose about a credential, not one
	cfnOutputSecretDetail = "A stack output holds what looks like a credential, and outputs are readable by anyone who can describe the stack and importable by any other stack in the account. Move the value into Secrets Manager, export only its name, and rotate the exposed credential."
)
