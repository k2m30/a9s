// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeIGWStateAttaching    domain.FindingCode = "igw.state.attaching"
	CodeIGWUnrecognisedState domain.FindingCode = "igw.state.unrecognised"
	CodeIGWStateDetaching    domain.FindingCode = "igw.state.detaching"
	CodeIGWNoAttachments     domain.FindingCode = "igw.no-attachments"
)
