// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeTGWStatePending   domain.FindingCode = "tgw.state.pending"
	CodeTGWStateModifying domain.FindingCode = "tgw.state.modifying"
	CodeTGWStateDeleting  domain.FindingCode = "tgw.state.deleting"
	CodeTGWStateFailed    domain.FindingCode = "tgw.state.failed"
	CodeTGWStateDeleted   domain.FindingCode = "tgw.state.deleted"

	// CodeTGWAutoAccept marks a transit gateway that attaches any VPC shared
	// with it without an operator approving the attachment.
	CodeTGWAutoAccept domain.FindingCode = "tgw.auto-accept-attachments"
)

// TGWAutoAcceptPhrase is the S4 status phrase for CodeTGWAutoAccept.
const TGWAutoAcceptPhrase = "auto-accepts shared attachments"

// TGWAutoAcceptDetail is the S5 operator sentence for CodeTGWAutoAccept.
const TGWAutoAcceptDetail = "Any account this gateway is shared with can attach a VPC to it without review, " +
	"putting that VPC on your routed network the moment it asks. Turn auto-accept off and approve each " +
	"attachment explicitly."
