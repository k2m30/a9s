// rds_snap_related.go contains related-resource checker functions for RDS snapshots.
package aws

import (
	"context"
	"strings"

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
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
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
	for _, dbiRes := range dbiList {
		if dbiRes.Name != dbName && dbiRes.ID != dbName {
			continue
		}
		db, ok := assertStruct[rdstypes.DBInstance](dbiRes.RawStruct)
		if !ok {
			continue
		}
		if db.DBClusterIdentifier != nil && *db.DBClusterIdentifier != "" {
			clusterID = *db.DBClusterIdentifier
		}
		break
	}
	if clusterID == "" {
		if truncated {
			return resource.RelatedCheckResult{TargetType: "dbc", Count: -1}
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
