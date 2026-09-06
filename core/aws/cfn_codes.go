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
