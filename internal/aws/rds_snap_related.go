// rds_snap_related.go contains related-resource checker functions for RDS snapshots.
package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkRDSSnapDBI extracts DBInstanceIdentifier from the DBSnapshot RawStruct
// and searches the dbi cache for a matching instance name.
// Pattern C — needs target cache.
func checkRDSSnapDBI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[rdstypes.DBSnapshot](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1}
	}
	if snap.DBInstanceIdentifier == nil || *snap.DBInstanceIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: 0}
	}
	dbName := *snap.DBInstanceIdentifier

	dbiList, truncated, err := rdsSnapRelatedResources(ctx, clients, cache, "dbi")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1, Err: err}
	}
	if dbiList == nil {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1}
	}

	var ids []string
	for _, dbiRes := range dbiList {
		if dbiRes.Name == dbName || dbiRes.ID == dbName {
			ids = append(ids, dbiRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("dbi")
	}
	return relatedResult("dbi", ids)
}

// checkRDSSnapKMS extracts KmsKeyId from the DBSnapshot RawStruct and matches
// it against the kms cache. Handles full ARN format (arn:aws:kms:…/key-id).
// Pattern C — needs target cache.
func checkRDSSnapKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[rdstypes.DBSnapshot](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if snap.KmsKeyId == nil || *snap.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	val := *snap.KmsKeyId
	keyID := val
	if idx := strings.LastIndex(val, "/"); idx >= 0 && idx < len(val)-1 {
		keyID = val[idx+1:]
	}
	if keyID == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}

	kmsList, truncated, err := rdsSnapRelatedResources(ctx, clients, cache, "kms")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1, Err: err}
	}
	if kmsList == nil {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}

	var ids []string
	for _, kmsRes := range kmsList {
		if kmsRes.ID == keyID {
			ids = append(ids, kmsRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("kms")
	}
	return relatedResult("kms", ids)
}

// checkRDSSnapDBC resolves the owning Aurora/RDS cluster by two-hop lookup
// through the dbi cache (no extra API call — reuses the dbi list):
// snap.DBInstanceIdentifier → dbi entry → dbi.DBClusterIdentifier → dbc.
// Returns Count=0 when the source instance is standalone (no cluster).
func checkRDSSnapDBC(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[rdstypes.DBSnapshot](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: -1}
	}
	if snap.DBInstanceIdentifier == nil || *snap.DBInstanceIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: 0}
	}
	dbName := *snap.DBInstanceIdentifier

	dbiList, truncated, err := rdsSnapRelatedResources(ctx, clients, cache, "dbi")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: -1, Err: err}
	}
	if dbiList == nil {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: -1}
	}

	var clusterID string
	dbiFound := false
	for _, dbiRes := range dbiList {
		if dbiRes.Name != dbName && dbiRes.ID != dbName {
			continue
		}
		dbiFound = true
		db, ok := assertStruct[rdstypes.DBInstance](dbiRes.RawStruct)
		if !ok {
			break
		}
		if db.DBClusterIdentifier != nil && *db.DBClusterIdentifier != "" {
			clusterID = *db.DBClusterIdentifier
		}
		break
	}
	if clusterID == "" {
		// UnknownRelated ONLY when the DBI cache was truncated AND we didn't
		// find the source DB instance in the visible window — in that case
		// we never reached the dbc lookup at all. If the source DBI IS in the
		// cache (dbiFound) but has no DBClusterIdentifier, the snapshot is
		// confirmed standalone → Count:0 (definitive), regardless of whether
		// OTHER dbi entries may exist off-page.
		if !dbiFound && truncated {
			return resource.UnknownRelated("dbc")
		}
		return resource.RelatedCheckResult{TargetType: "dbc", Count: 0}
	}
	return relatedResult("dbc", []string{clusterID})
}

// rdsSnapRelatedResources returns cached resources for the target type, or fetches the first page.
func rdsSnapRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// checkRDSSnapBackup resolves AWS Backup PLANS that cover this RDS snapshot's
// PARENT DB INSTANCE by reverse-scanning the already-loaded backup PLAN cache
// (Pattern C — cache scan, zero extra API calls).
//
// AWS Backup tracks the parent DB instance, not individual snapshots — a
// BackupSelection.Resources entry matches an `arn:aws:rds:…:db:<name>` ARN,
// not a snapshot ARN. For each cached plan we test whether its Fields[resources]
// patterns cover the snapshot's parent DBInstanceArn.
//
// The parent DB ARN is resolved via the dbi cache (snap.DBInstanceIdentifier
// → DBInstance.DBInstanceArn). When the parent has been deleted (orphan) or
// the dbi cache is not loaded yet, we cannot identify the parent and return
// UnknownRelated rather than a misleading Count=0.
//
// Why plan IDs (not recovery-point ARNs): the backup fetcher's Resource.ID
// space is plan IDs. Returning recovery-point ARNs would resolve Count > 0
// but break drill-through (the target list filter could not match the IDs).
// Recovery points are not first-class a9s resources at present.
func checkRDSSnapBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snap, ok := assertStruct[rdstypes.DBSnapshot](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1}
	}
	parentName := ""
	if snap.DBInstanceIdentifier != nil {
		parentName = *snap.DBInstanceIdentifier
	}
	if parentName == "" {
		// No parent reference on the snapshot — can't pivot.
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}

	// Resolve parent DBInstanceArn via the dbi cache. If the cache isn't loaded,
	// the parent ARN is unavailable and the answer is genuinely unknown.
	dbiList, _, err := rdsSnapRelatedResources(ctx, clients, cache, "dbi")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "backup", Count: -1, Err: err}
	}
	if dbiList == nil {
		return resource.UnknownRelated("backup")
	}
	parentARN := ""
	for _, dbiRes := range dbiList {
		if dbiRes.ID != parentName && dbiRes.Name != parentName {
			continue
		}
		if db, ok := assertStruct[rdstypes.DBInstance](dbiRes.RawStruct); ok && db.DBInstanceArn != nil {
			parentARN = *db.DBInstanceArn
		}
		break
	}
	if parentARN == "" {
		// Parent DB not in cache (orphan) — Backup tracks the parent so this
		// pivot has no answer for orphan snapshots. Definitive Count=0 rather
		// than UnknownRelated: we DID load the dbi cache and the parent is
		// gone; further coverage by AWS Backup is impossible.
		return resource.RelatedCheckResult{TargetType: "backup", Count: 0}
	}

	planList, truncated, err := rdsSnapRelatedResources(ctx, clients, cache, "backup")
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


// checkRDSSnapCTEvents looks up cached CloudTrail events for the snapshot's
// DBSnapshotIdentifier. Universal pivot — every registered type gets one;
// see docs/related-resources.md §Policy. FetchFilter["ResourceName"] is always
// set so the caller can do a filtered re-fetch; Count is "unknown" (windowed)
// per the spec — the panel renders the visible page count rather than a total.
func checkRDSSnapCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snapID := res.ID
	if snapID == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	fetchFilter := map[string]string{"ResourceName": snapID}
	eventList, truncated, err := rdsSnapRelatedResources(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, Err: err, FetchFilter: fetchFilter}
	}
	if eventList == nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, FetchFilter: fetchFilter}
	}
	var ids []string
	for _, eventRes := range eventList {
		// When a typed cloudtrail Event is present, its Resources slice is
		// authoritative — the Fields["resource_name"] fallback below is only
		// for resources without a typed RawStruct (test helpers, demo
		// shortcuts). If the typed slice exists and contains no match for
		// snapID, the event genuinely doesn't reference this snapshot;
		// don't second-guess that via the text fallback.
		if raw, ok := assertStruct[cloudtrailtypes.Event](eventRes.RawStruct); ok {
			for _, rr := range raw.Resources {
				if rr.ResourceName != nil && *rr.ResourceName == snapID {
					ids = append(ids, eventRes.ID)
					break
				}
			}
			continue
		}
		if eventRes.Fields["resource_name"] == snapID {
			ids = append(ids, eventRes.ID)
		}
	}
	if truncated {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, FetchFilter: fetchFilter}
	}
	result := relatedResult("ct-events", ids)
	result.FetchFilter = fetchFilter
	return result
}
