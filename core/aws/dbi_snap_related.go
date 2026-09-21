// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// dbi_snap_related.go contains related-resource checker functions for RDS DB instance snapshots.
package aws

import (
	"context"

	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkDBISnapDBI searches the dbi cache for the instance the snapshot was
// taken from.
func checkDBISnapDBI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[rdstypes.DBSnapshot](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("dbi")
	}
	if snap.DBInstanceIdentifier == nil || *snap.DBInstanceIdentifier == "" {
		return resource.ProvenZero("dbi", "snap.DBInstanceIdentifier")
	}

	dbiList, truncated, err := relatedResourcesFor(ctx, clients, cache, "dbi")
	if err != nil {
		return resource.ErrorRelated("dbi", err)
	}
	if dbiList == nil {
		return resource.UnknownRelated("dbi")
	}

	var ids []string
	for _, dbiRes := range dbiList {
		if dbiSnapTakenFrom(snap, dbiRes) {
			ids = append(ids, dbiRes.ID)
		}
	}
	return relatedResultTrunc("dbi", ids, truncated)
}

// checkDBISnapKMS returns the key in the DBSnapshot's KmsKeyId.
func checkDBISnapKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[rdstypes.DBSnapshot](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
		return resource.ProvenZero("kms", "snap.KmsKeyId")
	}
	return kmsRelated(ctx, clients, cache, []string{*snap.KmsKeyId})
}

// checkDBISnapBackup resolves AWS Backup PLANS that cover this RDS snapshot's
// PARENT DB INSTANCE by reverse-scanning the already-loaded backup PLAN cache
// (cache scan, zero extra API calls).
//
// AWS Backup tracks the parent DB instance, not individual snapshots — a
// BackupSelection.Resources entry matches an `arn:aws:rds:…:db:<name>` ARN,
// not a snapshot ARN. For each cached plan we test whether its Fields[resources]
// patterns cover the snapshot's parent DBInstanceArn.
//
// The parent DB ARN is resolved via the dbi cache (dbiSnapTakenFrom
// → DBInstance.DBInstanceArn). When the parent has been deleted (orphan) or
// the dbi cache is not loaded yet, we cannot identify the parent and return
// UnknownRelated rather than a misleading Count=0.
//
// Why plan IDs (not recovery-point ARNs): the backup fetcher's Resource.ID
// space is plan IDs. Returning recovery-point ARNs would resolve Count > 0
// but break drill-through (the target list filter could not match the IDs).
// Recovery points are not first-class a9s resources at present.
func checkDBISnapBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[rdstypes.DBSnapshot](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("backup")
	}
	parentName := ""
	if snap.DBInstanceIdentifier != nil {
		parentName = *snap.DBInstanceIdentifier
	}
	if parentName == "" {
		// No parent reference on the snapshot — can't pivot.
		return resource.ProvenZero("backup", "parentName")
	}

	// Resolve parent DBInstanceArn via the dbi cache. If the cache isn't loaded,
	// the parent ARN is unavailable and the answer is genuinely unknown.
	dbiList, dbiTruncated, err := relatedResourcesFor(ctx, clients, cache, "dbi")
	if err != nil {
		return resource.ErrorRelated("backup", err)
	}
	if dbiList == nil {
		return resource.UnknownRelated("backup")
	}
	parentARN := ""
	for _, dbiRes := range dbiList {
		if !dbiSnapTakenFrom(snap, dbiRes) {
			continue
		}
		if db, ok := assertStruct[rdstypes.DBInstance](dbiRes.RawStruct); ok && db.DBInstanceArn != nil {
			parentARN = *db.DBInstanceArn
		}
		break
	}
	if parentARN == "" {
		// Parent not found in visible window.
		if dbiTruncated {
			// Cache is truncated — parent may be in a later page; answer is unknown.
			return resource.UnknownRelated("backup")
		}
		// Cache is complete — parent is genuinely absent (orphan) — Backup
		// tracks the parent so this pivot has no answer for orphan snapshots.
		return resource.ProvenZero("backup", "parentARN")
	}

	planList, truncated, err := relatedResourcesFor(ctx, clients, cache, "backup")
	if err != nil {
		return resource.ErrorRelated("backup", err)
	}
	if planList == nil {
		return resource.UnknownRelated("backup")
	}

	return backupPivot(planList, truncated, backupTarget{arn: parentARN, unread: "ListTagsForResource"})
}
