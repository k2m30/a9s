// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeTGWStatePending   domain.FindingCode = "tgw.state.pending"
	CodeTGWStateModifying domain.FindingCode = "tgw.state.modifying"
	CodeTGWStateDeleting  domain.FindingCode = "tgw.state.deleting"
	CodeTGWStateFailed    domain.FindingCode = "tgw.state.failed"
	CodeTGWStateDeleted   domain.FindingCode = "tgw.state.deleted"
)
