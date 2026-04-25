package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/backup"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDbcSnapDBC reads DBClusterIdentifier from the DBClusterSnapshot RawStruct.
// Pattern F — no cache needed.
func checkDbcSnapDBC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: -1}
	}
	if snap.DBClusterIdentifier == nil || *snap.DBClusterIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: 0}
	}
	return relatedResult("dbc", []string{*snap.DBClusterIdentifier})
}

// checkDbcSnapKMS reads KmsKeyId from the DBClusterSnapshot RawStruct.
// Extracts UUID after last '/' from the ARN.
// Pattern F — no cache needed.
func checkDbcSnapKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *snap.KmsKeyId
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 {
		keyID = keyID[idx+1:]
	}
	if keyID == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{keyID})
}

func checkDbcSnapVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct)
	if !ok || snap.VpcId == nil || *snap.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*snap.VpcId})
}

// checkDbcSnapBackup resolves AWS Backup recovery points for this DocumentDB
// cluster snapshot via backup:ListRecoveryPointsByResource (Pattern A: 1 API call).
// The snapshot ARN is read from DBClusterSnapshotArn in RawStruct.
func checkDbcSnapBackup(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	if snap.DBClusterSnapshotArn == nil || *snap.DBClusterSnapshotArn == "" {
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}
	snapARN := *snap.DBClusterSnapshotArn

	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.Backup == nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	api, ok := c.Backup.(BackupListRecoveryPointsByResourceAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*backup.ListRecoveryPointsByResourceOutput, error) {
		return api.ListRecoveryPointsByResource(ctx, &backup.ListRecoveryPointsByResourceInput{
			ResourceArn: &snapARN,
		})
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
	}
	var ids []string
	for _, rp := range out.RecoveryPoints {
		if rp.RecoveryPointArn != nil && *rp.RecoveryPointArn != "" {
			ids = append(ids, *rp.RecoveryPointArn)
		}
	}
	return relatedResult("backup", ids)
}
