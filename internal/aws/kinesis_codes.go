package aws

import "github.com/k2m30/a9s/v3/internal/domain"

const (
	CodeKinesisCreating domain.FindingCode = "kinesis.warn.creating"
	CodeKinesisUpdating domain.FindingCode = "kinesis.warn.updating"
	CodeKinesisDeleting domain.FindingCode = "kinesis.warn.deleting"
)
