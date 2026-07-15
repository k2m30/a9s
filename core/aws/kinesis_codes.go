// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeKinesisCreating domain.FindingCode = "kinesis.warn.creating"
	CodeKinesisUpdating domain.FindingCode = "kinesis.warn.updating"
	CodeKinesisDeleting domain.FindingCode = "kinesis.warn.deleting"
)
