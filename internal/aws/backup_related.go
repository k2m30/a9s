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
