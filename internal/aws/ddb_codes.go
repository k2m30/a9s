package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeDDBKMSKeyInaccessible domain.FindingCode = "ddb.broken.kms_key_inaccessible"
	CodeDDBArchivedKMSLost    domain.FindingCode = "ddb.broken.archived_kms_lost"
	CodeDDBCreating           domain.FindingCode = "ddb.warn.creating"
	CodeDDBUpdating           domain.FindingCode = "ddb.warn.updating"
	CodeDDBDeleting           domain.FindingCode = "ddb.warn.deleting"
	CodeDDBArchiving          domain.FindingCode = "ddb.warn.archiving"
)
