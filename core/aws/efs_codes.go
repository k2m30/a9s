// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import "github.com/k2m30/a9s/v3/core/domain"

const (
	CodeEFSError          domain.FindingCode = "efs.broken.error"
	CodeEFSNoMountTargets domain.FindingCode = "efs.broken.no_mount_targets"
	CodeEFSCreating       domain.FindingCode = "efs.warn.creating"
	CodeEFSUpdating       domain.FindingCode = "efs.warn.updating"
	CodeEFSDeleting       domain.FindingCode = "efs.warn.deleting"
)

// CodeEFSUnencrypted is the wave-1 posture finding read from the
// DescribeFileSystems output the fetcher already holds (Prowler gap closure).
const CodeEFSUnencrypted domain.FindingCode = "efs.unencrypted"

// efsUnencryptedDetail is the S5 operator sentence for it.
const efsUnencryptedDetail = "File data is stored unencrypted at rest. Encryption can only be set when the file system is created — create an encrypted file system and copy the data across."
