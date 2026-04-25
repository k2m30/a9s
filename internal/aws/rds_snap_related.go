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

// checkRDSSnapBackup resolves AWS Backup PLANS that cover this RDS snapshot
// by reverse-scanning the already-loaded backup PLAN cache (Pattern C —
// cache scan, zero extra API calls). For each cached plan, Fields["resources"]
// holds a comma-separated list of resource ARNs / wildcard patterns from the
// plan's BackupSelection. A plan covers the snapshot iff any Resources entry
// matches the snapshot's ARN AND no NotResources entry matches.
//
// Why plan IDs (not recovery-point ARNs): the backup fetcher's Resource.ID
// space is plan IDs. Returning recovery-point ARNs would resolve Count > 0
// but break drill-through (the target list filter could not match the IDs).
// Recovery points are not first-class a9s resources at present.
//
// As a fallback (no backup-plan cache loaded), return UnknownRelated so the
// UI shows the pivot but can't yet count it — preferable to silently 0.
func checkRDSSnapBackup(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	snapARN := res.Fields["arn"]
	if snapARN == "" {
		// Derive ARN from RawStruct if available.
		snap, ok := assertStruct[rdstypes.DBSnapshot](res.RawStruct)
		if ok && snap.DBSnapshotArn != nil {
			snapARN = *snap.DBSnapshotArn
		}
	}
	if snapARN == "" {
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
		if BackupPlanCoversARN(planRes.Fields["resources"], planRes.Fields["not_resources"], snapARN) {
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
		raw, ok := assertStruct[cloudtrailtypes.Event](eventRes.RawStruct)
		if ok {
			matched := false
			for _, rr := range raw.Resources {
				if rr.ResourceName != nil && *rr.ResourceName == snapID {
					matched = true
					break
				}
			}
			if matched {
				ids = append(ids, eventRes.ID)
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
