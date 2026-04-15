// backup_related.go contains AWS Backup related-resource checker functions.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkBackupRole returns Count: 0 because BackupPlansListMember does not include
// the IAM role ARN used for backup execution — the relationship cannot be
// determined from cache alone.
func checkBackupRole(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	return resource.RelatedCheckResult{TargetType: "role", Count: 0}
}

// No NavigableFields for backup: backup.BackupPlansListMember (list response) carries
// only plan metadata (Name, Id, Arn, CreationDate). IamRoleArn and EncryptionKeyArn
// are on individual backup jobs/rules, not on the plan list struct used as RawStruct.
