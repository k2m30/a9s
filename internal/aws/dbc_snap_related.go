package aws

import (
	"context"
	"strings"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDbcSnapDBC reads DBClusterIdentifier from the DBClusterSnapshot RawStruct.
// Handles both docdbtypes.DBClusterSnapshot and rdstypes.DBClusterSnapshot shapes.
// Pattern F — no cache needed.
func checkDbcSnapDBC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct); ok {
		if snap.DBClusterIdentifier == nil || *snap.DBClusterIdentifier == "" {
			return resource.RelatedCheckResult{TargetType: "dbc", Count: 0}
		}
		return relatedResult("dbc", []string{*snap.DBClusterIdentifier})
	}
	if snap, ok := assertStruct[rdstypes.DBClusterSnapshot](res.RawStruct); ok {
		if snap.DBClusterIdentifier == nil || *snap.DBClusterIdentifier == "" {
			return resource.RelatedCheckResult{TargetType: "dbc", Count: 0}
		}
		return relatedResult("dbc", []string{*snap.DBClusterIdentifier})
	}
	return resource.RelatedCheckResult{TargetType: "dbc", Count: -1}
}

// checkDbcSnapKMS reads KmsKeyId from the DBClusterSnapshot RawStruct.
// Extracts UUID after last '/' from the ARN.
// Handles both docdbtypes.DBClusterSnapshot and rdstypes.DBClusterSnapshot shapes.
// Pattern F — no cache needed.
func checkDbcSnapKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	var keyID string
	if snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct); ok {
		if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
			return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
		}
		keyID = *snap.KmsKeyId
	} else if snap, ok := assertStruct[rdstypes.DBClusterSnapshot](res.RawStruct); ok {
		if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
			return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
		}
		keyID = *snap.KmsKeyId
	} else {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 {
		keyID = keyID[idx+1:]
	}
	if keyID == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{keyID})
}

// checkDbcSnapVPC reads VpcId from the DBClusterSnapshot RawStruct.
// Handles both docdbtypes.DBClusterSnapshot and rdstypes.DBClusterSnapshot shapes.
func checkDbcSnapVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct); ok {
		if snap.VpcId == nil || *snap.VpcId == "" {
			return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
		}
		return relatedResult("vpc", []string{*snap.VpcId})
	}
	if snap, ok := assertStruct[rdstypes.DBClusterSnapshot](res.RawStruct); ok {
		if snap.VpcId == nil || *snap.VpcId == "" {
			return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
		}
		return relatedResult("vpc", []string{*snap.VpcId})
	}
	return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
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
//
// Truncated-cache rule: when the dbc cache is truncated AND the parent ARN
// cannot be resolved from the visible window, we cannot determine whether
// the parent is in a later page — return UnknownRelated rather than Count:0.
func checkDbcSnapBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	parentName, parentARN := dbcSnapParentRefs(res.RawStruct)
	if parentName == "" {
		// No parent reference — can't pivot.
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}

	// If the snapshot's RawStruct already exposes the parent cluster ARN we
	// can skip the dbc-cache lookup. rdstypes.DBClusterSnapshot carries
	// DBClusterArn directly; docdbtypes does not.
	if parentARN == "" {
		dbcList, dbcTruncated, err := dbcRelatedResources(ctx, clients, cache, "dbc")
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
		if parentARN == "" {
			// Parent not found in visible window.
			if dbcTruncated {
				// Cache is truncated — parent may be in a later page; answer is unknown.
				return resource.UnknownRelated("backup")
			}
			// Cache is complete — parent is genuinely absent (orphan or no ARN field).
			return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
		}
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

// dbcSnapParentRefs extracts (parentClusterName, parentClusterARN) from a
// DBClusterSnapshot. Neither docdbtypes.DBClusterSnapshot nor
// rdstypes.DBClusterSnapshot carries the parent cluster ARN on the snapshot
// shape — callers fall back to a dbc-cache lookup in both cases.
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

// dbcResourceARN extracts DBClusterArn from a dbc Resource's RawStruct.
// Handles both docdb_types.DBCluster and rdstypes.DBCluster shapes.
func dbcResourceARN(raw any) string {
	if c, ok := assertStruct[docdbtypes.DBCluster](raw); ok && c.DBClusterArn != nil {
		return *c.DBClusterArn
	}
	if c, ok := assertStruct[rdstypes.DBCluster](raw); ok && c.DBClusterArn != nil {
		return *c.DBClusterArn
	}
	return ""
}
