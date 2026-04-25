package aws

import (
	"context"
	"strings"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

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

// checkDbcSnapBackup resolves AWS Backup PLANS that cover this DocumentDB or
// Aurora cluster snapshot's PARENT CLUSTER by reverse-scanning the already-
// loaded backup PLAN cache (Pattern C — cache scan, zero extra API calls).
//
// AWS Backup tracks the parent cluster, not individual snapshots — a
// BackupSelection.Resources entry matches an `arn:aws:rds:…:cluster:<name>`
// ARN, not a snapshot ARN. For each cached plan we test whether its
// Fields[resources] patterns cover the snapshot's parent DBClusterArn.
//
// Why plan IDs (not recovery-point ARNs): the backup fetcher's Resource.ID
// space is plan IDs. Returning recovery-point ARNs would resolve Count > 0
// but break drill-through (the target list filter could not match the IDs).
// Recovery points are not first-class a9s resources at present. This mirrors
// the dbi-snap → backup checker pattern.
func checkDbcSnapBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	parentName, parentARN := dbcSnapParentRefs(res.RawStruct)
	if parentName == "" {
		// No parent reference — can't pivot.
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}

	// If the snapshot's RawStruct already exposes the parent cluster ARN we
	// can skip the dbc-cache lookup. Fall back to scanning the dbc cache
	// otherwise (some shapes only carry DBClusterIdentifier).
	if parentARN == "" {
		dbcList, _, err := dbcRelatedResources(ctx, clients, cache, "dbc")
		if err != nil {
			return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
		}
		if dbcList == nil {
			return resource.UnknownRelated("backup")
		}
		for _, dbcRes := range dbcList {
			if dbcRes.ID != parentName && dbcRes.Name != parentName {
				continue
			}
			parentARN = dbcResourceARN(dbcRes.RawStruct)
			break
		}
	}
	if parentARN == "" {
		// Parent cluster not in cache (orphan) or no ARN field on this shape —
		// pivot has no answer; Backup tracks the parent cluster.
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}

	planList, truncated, err := dbcRelatedResources(ctx, clients, cache, "backup")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
	}
	if planList == nil {
		return resource.UnknownRelated("backup")
	}

	var ids []string
	for _, planRes := range planList {
		if BackupPlanCoversARN(planRes.Fields["resources"], planRes.Fields["not_resources"], parentARN) {
			ids = append(ids, planRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("backup")
	}
	return relatedResult("backup", ids)
}

// dbcSnapParentRefs extracts (parentClusterName, parentClusterARN) from
// either docdb or rds DBClusterSnapshot SDK shape. ARN may be "" when the
// shape doesn't carry it (e.g. legacy DocDB responses) — caller then falls
// back to a dbc-cache lookup.
func dbcSnapParentRefs(raw any) (name, arn string) {
	if snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](raw); ok {
		if snap.DBClusterIdentifier != nil {
			name = *snap.DBClusterIdentifier
		}
		return name, ""
	}
	if snap, ok := assertStruct[rdstypes.DBClusterSnapshot](raw); ok {
		if snap.DBClusterIdentifier != nil {
			name = *snap.DBClusterIdentifier
		}
		return name, ""
	}
	return "", ""
}

// dbcResourceARN extracts DBClusterArn from a dbc Resource's RawStruct
// (handles both docdb and rds DBCluster SDK shapes).
func dbcResourceARN(raw any) string {
	if c, ok := assertStruct[docdbtypes.DBCluster](raw); ok {
		if c.DBClusterArn != nil {
			return *c.DBClusterArn
		}
		return ""
	}
	if c, ok := assertStruct[rdstypes.DBCluster](raw); ok {
		if c.DBClusterArn != nil {
			return *c.DBClusterArn
		}
		return ""
	}
	return ""
}
