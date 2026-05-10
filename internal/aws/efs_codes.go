package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeEFSError          domain.FindingCode = "efs.broken.error"
	CodeEFSNoMountTargets domain.FindingCode = "efs.broken.no_mount_targets"
	CodeEFSCreating       domain.FindingCode = "efs.warn.creating"
	CodeEFSUpdating       domain.FindingCode = "efs.warn.updating"
	CodeEFSDeleting       domain.FindingCode = "efs.warn.deleting"
)
