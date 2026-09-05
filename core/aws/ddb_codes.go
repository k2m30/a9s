// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeDDBKMSKeyInaccessible domain.FindingCode = "ddb.broken.kms_key_inaccessible"
	CodeDDBArchivedKMSLost    domain.FindingCode = "ddb.broken.archived_kms_lost"
	CodeDDBCreating           domain.FindingCode = "ddb.warn.creating"
	CodeDDBUpdating           domain.FindingCode = "ddb.warn.updating"
	CodeDDBDeleting           domain.FindingCode = "ddb.warn.deleting"
	CodeDDBArchiving          domain.FindingCode = "ddb.warn.archiving"
)

// CodeDDBDeletionProtectionOff is the wave-1 posture finding read from the
// DescribeTable output the fetcher already holds (Prowler gap closure).
const CodeDDBDeletionProtectionOff domain.FindingCode = "ddb.deletion-protection-off"

// ddbDeletionProtectionOffDetail is the S5 operator sentence for it.
const ddbDeletionProtectionOffDetail = "A single delete call (DeleteTable) destroys this table and its data. Turn on deletion protection so removing it takes a deliberate second step."
