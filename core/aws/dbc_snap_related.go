// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkDbcSnapDBC searches the dbc cache for the cluster the snapshot was
// taken from (dbcSnapTakenFrom).
func checkDbcSnapDBC(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	p, ok := dbcSnapParentOf(res.RawStruct)
	if !ok {
		return NotRead("dbc")
	}
	if p.cluster == "" || !p.local() {
		return foundNone("dbc", "p.cluster")
	}

	dbcList, truncated, err := relatedResourcesFor(ctx, clients, cache, "dbc")
	if err != nil {
		return ReadFailed("dbc", err)
	}
	if dbcList == nil {
		return NotRead("dbc")
	}

	var ids []string
	for _, dbcRes := range dbcList {
		if dbcSnapTakenFrom(res.RawStruct, dbcRes) {
			ids = append(ids, dbcRes.ID)
		}
	}
	return relatedResultTrunc("dbc", ids, truncated)
}

// checkDbcSnapKMS reads KmsKeyId from the DBClusterSnapshot RawStruct.
// Extracts UUID after last '/' from the ARN.
// Handles both docdbtypes.DBClusterSnapshot and rdstypes.DBClusterSnapshot shapes.
func checkDbcSnapKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	var keyID string
	if snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct); ok {
		if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
			return foundNone("kms", "snap.KmsKeyId")
		}
		keyID = *snap.KmsKeyId
	} else if snap, ok := assertStruct[rdstypes.DBClusterSnapshot](res.RawStruct); ok {
		if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
			return foundNone("kms", "snap.KmsKeyId")
		}
		keyID = *snap.KmsKeyId
	} else {
		return NotRead("kms")
	}
	keyID = kmsRefFromField(keyID, res.Type)
	if keyID == "" {
		return foundNone("kms", "keyID")
	}
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkDbcSnapVPC reads VpcId from the DBClusterSnapshot RawStruct.
// Handles both docdbtypes.DBClusterSnapshot and rdstypes.DBClusterSnapshot shapes.
func checkDbcSnapVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	if snap, ok := assertStruct[docdbtypes.DBClusterSnapshot](res.RawStruct); ok {
		if snap.VpcId == nil || *snap.VpcId == "" {
			return unreadZero(res, foundNone("vpc", "snap.VpcId"))
		}
		return unreadZero(res, relatedResultTrunc("vpc", []string{*snap.VpcId}, false))
	}
	if snap, ok := assertStruct[rdstypes.DBClusterSnapshot](res.RawStruct); ok {
		if snap.VpcId == nil || *snap.VpcId == "" {
			return unreadZero(res, foundNone("vpc", "snap.VpcId"))
		}
		return unreadZero(res, relatedResultTrunc("vpc", []string{*snap.VpcId}, false))
	}
	return unreadZero(res, foundNone("vpc", "the lookup completed"))
}

// checkDbcSnapBackup resolves AWS Backup PLANS that cover this DocumentDB or
// Aurora cluster snapshot's PARENT CLUSTER by reverse-scanning the already-
// loaded backup PLAN cache (cache scan, zero extra API calls).
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
	p, ok := dbcSnapParentOf(res.RawStruct)
	if !ok || p.cluster == "" || !p.local() {
		return unreadZero(res, foundNone("backup", "p.cluster"))
	}

	// Neither snapshot shape carries the parent cluster ARN, so it is
	// resolved through the dbc cache.
	dbcList, dbcTruncated, err := relatedResourcesFor(ctx, clients, cache, "dbc")
	if err != nil {
		return ReadFailed("backup", err)
	}
	if dbcList == nil {
		return NotRead("backup")
	}
	parentARN := ""
	for _, dbcRes := range dbcList {
		if dbcSnapTakenFrom(res.RawStruct, dbcRes) {
			parentARN = dbcResourceARN(dbcRes.RawStruct)
			break
		}
	}
	if parentARN == "" {
		if dbcTruncated {
			return NotRead("backup")
		}
		return unreadZeroScanned(res, len(dbcList), foundNone("backup", "parentARN"))
	}

	planList, truncated, err := relatedResourcesFor(ctx, clients, cache, "backup")
	if err != nil {
		return ReadFailed("backup", err)
	}
	if planList == nil {
		return NotRead("backup")
	}

	// AWS Backup protects the parent cluster, so a plan selecting it by tag
	// is read against the parent's tags.
	target := backupTarget{arn: parentARN, engine: res.Fields["engine"], unread: "ListTagsForResource"}
	if c, ok := clients.(*ServiceClients); ok && c != nil {
		if api, ok := c.DocDB.(DocDBListTagsForResourceAPI); ok {
			target = target.withTags(docdbTagsForARN(ctx, api, parentARN))
		}
	}
	return unreadZeroScanned(res, len(planList), backupPivot(planList, truncated, target))
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
