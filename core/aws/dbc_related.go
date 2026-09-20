// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdb_types "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// errDbcNoClusterDetail is the answer when the row holds something that is not
// a DB cluster, or the engine's client is unusable. Distinct from
// errRawStructMissing: there the row was never read, here what we have cannot
// be used.
var errDbcNoClusterDetail = errors.New("the cluster could not be read")

// dbcSubnetGroupInfo is a minimal, engine-agnostic view of a DBSubnetGroup that
// the dbc → subnet and dbc → vpc pivots need. Both dbcDocDBSubnetGroup and
// dbcRDSSubnetGroup return this type so callers are not coupled to a specific SDK
// shape.
type dbcSubnetGroupInfo struct {
	VpcId   *string
	Subnets []dbcSubnetIdentifier
}

// dbcSubnetIdentifier carries the SubnetIdentifier field shared by both
// docdb_types.Subnet and rdstypes.Subnet shapes.
type dbcSubnetIdentifier struct {
	SubnetIdentifier *string
}

// dbcClusterIdentifier extracts DBClusterIdentifier from either
// docdb_types.DBCluster or rdstypes.DBCluster RawStruct shape — the dbc
// fetcher merges results from both engines.
func dbcClusterIdentifier(raw any) string {
	if c, ok := assertStruct[docdb_types.DBCluster](raw); ok && c.DBClusterIdentifier != nil {
		return *c.DBClusterIdentifier
	}
	if c, ok := assertStruct[rdstypes.DBCluster](raw); ok && c.DBClusterIdentifier != nil {
		return *c.DBClusterIdentifier
	}
	return ""
}

// dbcClusterVpcSecurityGroupIDs returns the VPC SG IDs from either shape.
// Returns (ids, true) when the RawStruct is a recognised cluster shape (even if
// no SGs are attached); returns (nil, false) when the type is unrecognised.
func dbcClusterVpcSecurityGroupIDs(raw any) ([]string, bool) {
	if c, ok := assertStruct[docdb_types.DBCluster](raw); ok {
		var ids []string
		for _, sg := range c.VpcSecurityGroups {
			if sg.VpcSecurityGroupId != nil && *sg.VpcSecurityGroupId != "" {
				ids = append(ids, *sg.VpcSecurityGroupId)
			}
		}
		return ids, true
	}
	if c, ok := assertStruct[rdstypes.DBCluster](raw); ok {
		var ids []string
		for _, sg := range c.VpcSecurityGroups {
			if sg.VpcSecurityGroupId != nil && *sg.VpcSecurityGroupId != "" {
				ids = append(ids, *sg.VpcSecurityGroupId)
			}
		}
		return ids, true
	}
	return nil, false
}

// dbcClusterKmsKeyID returns the KmsKeyId from either shape.
func dbcClusterKmsKeyID(raw any) string {
	if c, ok := assertStruct[docdb_types.DBCluster](raw); ok && c.KmsKeyId != nil {
		return *c.KmsKeyId
	}
	if c, ok := assertStruct[rdstypes.DBCluster](raw); ok && c.KmsKeyId != nil {
		return *c.KmsKeyId
	}
	return ""
}

// dbcClusterSubnetGroupName returns the DBSubnetGroup name from either shape.
// Both docdb_types.DBCluster.DBSubnetGroup and rdstypes.DBCluster.DBSubnetGroup
// are *string (the group name), not a struct pointer.
func dbcClusterSubnetGroupName(raw any) string {
	if c, ok := assertStruct[docdb_types.DBCluster](raw); ok && c.DBSubnetGroup != nil {
		return *c.DBSubnetGroup
	}
	if c, ok := assertStruct[rdstypes.DBCluster](raw); ok && c.DBSubnetGroup != nil {
		return *c.DBSubnetGroup
	}
	return ""
}

// dbcClusterMasterSecretARN returns the MasterUserSecret.SecretArn from either shape.
func dbcClusterMasterSecretARN(raw any) string {
	if c, ok := assertStruct[docdb_types.DBCluster](raw); ok {
		if c.MasterUserSecret != nil && c.MasterUserSecret.SecretArn != nil {
			return *c.MasterUserSecret.SecretArn
		}
		return ""
	}
	if c, ok := assertStruct[rdstypes.DBCluster](raw); ok {
		if c.MasterUserSecret != nil && c.MasterUserSecret.SecretArn != nil {
			return *c.MasterUserSecret.SecretArn
		}
		return ""
	}
	return ""
}

// checkDbcSG reads VpcSecurityGroups[] from the DBCluster RawStruct and returns their IDs.
// Handles both docdb_types.DBCluster and rdstypes.DBCluster shapes.
func checkDbcSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	ids, ok := dbcClusterVpcSecurityGroupIDs(res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if len(ids) == 0 {
		return resource.ProvenZero("sg", "ids")
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkDbcAlarm searches the alarm cache for alarms with a "DBClusterIdentifier" dimension
// matching this DocumentDB cluster's identifier.
func checkDbcAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "dbc", res)
}

// checkDbcLogs searches the logs cache for log groups matching the DocumentDB cluster's
// naming convention: /aws/docdb/{clusterID}/audit or /aws/docdb/{clusterID}/profiler.
func checkDbcLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterID := res.ID
	if clusterID == "" {
		return resource.ProvenZero("logs", "clusterID")
	}

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}

	// dbc covers both DocumentDB (/aws/docdb/<cluster>/*) and Aurora
	// (/aws/rds/cluster/<cluster>/*). Match either prefix so the pivot
	// resolves on both engine families.
	docdbPrefix := "/aws/docdb/" + clusterID + "/"
	rdsPrefix := "/aws/rds/cluster/" + clusterID + "/"
	var ids []string
	for _, logRes := range logList {
		if strings.HasPrefix(logRes.ID, docdbPrefix) || strings.HasPrefix(logRes.ID, rdsPrefix) {
			ids = append(ids, logRes.ID)
		}
	}
	return relatedResultTrunc("logs", ids, truncated)
}

// checkDbcDBI does a reverse lookup — scans the dbi cache for DBInstances
// whose DBClusterIdentifier matches this cluster's identifier. Aurora /
// DocumentDB clusters own one or more DBInstance members.
// Handles both docdb_types.DBCluster and rdstypes.DBCluster shapes.
func checkDbcDBI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterID := res.ID
	if id := dbcClusterIdentifier(res.RawStruct); id != "" {
		clusterID = id
	}
	if clusterID == "" {
		return resource.ProvenZero("dbi", "clusterID")
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
		db, ok := assertStruct[rdstypes.DBInstance](dbiRes.RawStruct)
		if !ok {
			continue
		}
		if db.DBClusterIdentifier != nil && *db.DBClusterIdentifier == clusterID {
			ids = append(ids, dbiRes.ID)
		}
	}
	return relatedResultTrunc("dbi", ids, truncated)
}

// checkDbcDbcSnap scans the dbc-snap cache for snapshots taken from this
// cluster (dbcSnapTakenFrom).
func checkDbcDbcSnap(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	if res.ID == "" {
		return resource.ProvenZero("dbc-snap", "res.ID")
	}

	snapList, truncated, err := relatedResourcesFor(ctx, clients, cache, "dbc-snap")
	if err != nil {
		return resource.ErrorRelated("dbc-snap", err)
	}
	if snapList == nil {
		return resource.UnknownRelated("dbc-snap")
	}

	var ids []string
	for _, snapRes := range snapList {
		if dbcSnapTakenFrom(snapRes.RawStruct, res) {
			ids = append(ids, snapRes.ID)
		}
	}
	return relatedResultTrunc("dbc-snap", ids, truncated)
}

// checkDbcSubnet resolves the subnets inside the cluster's DBSubnetGroup via
// a single DescribeDBSubnetGroups call (live API). The DBCluster
// response only carries the subnet-group name; this call resolves it to the
// concrete Subnets slice. For rdstypes.DBCluster (Aurora) shapes the call goes
// to c.RDS; for docdb_types.DBCluster shapes it goes to c.DocDB.
// See docs/resources/dbc.md.
func checkDbcSubnet(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	sng, err := dbcSubnetGroup(ctx, clients, res)
	if err != nil {
		return relatedFromErr("subnet", err)
	}
	if sng == nil {
		return resource.ProvenZero("subnet", "sng")
	}
	var ids []string
	for _, s := range sng.Subnets {
		if s.SubnetIdentifier != nil && *s.SubnetIdentifier != "" {
			ids = append(ids, *s.SubnetIdentifier)
		}
	}
	return relatedResultTrunc("subnet", ids, false)
}

// checkDbcVPC resolves the VPC that hosts the cluster's subnet group via a
// single DescribeDBSubnetGroups call. Engine dispatch mirrors
// checkDbcSubnet — Aurora rows use c.RDS, DocDB rows use c.DocDB.
func checkDbcVPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	sng, err := dbcSubnetGroup(ctx, clients, res)
	if err != nil {
		return relatedFromErr("vpc", err)
	}
	if sng == nil {
		return resource.ProvenZero("vpc", "sng")
	}
	if sng.VpcId == nil || *sng.VpcId == "" {
		return resource.ProvenZero("vpc", "sng.VpcId")
	}
	return relatedResultTrunc("vpc", []string{*sng.VpcId}, false)
}

// dbcSubnetGroup dispatches to the appropriate engine-specific helper based on
// the RawStruct shape:
//   - rdstypes.DBCluster  → dbcRDSSubnetGroup  (Aurora / Multi-AZ; RDS API)
//   - docdb_types.DBCluster → dbcDocDBSubnetGroup (DocumentDB; DocDB API)
//
// A nil group with a nil error means the cluster names no subnet group, or the
// named group does not exist; an error means the describe could not be made
// or failed.
func dbcSubnetGroup(ctx context.Context, clients any, res resource.Resource) (*dbcSubnetGroupInfo, error) {
	if _, ok := assertStruct[rdstypes.DBCluster](res.RawStruct); ok {
		return dbcRDSSubnetGroup(ctx, clients, res)
	}
	if _, ok := assertStruct[docdb_types.DBCluster](res.RawStruct); ok {
		return dbcDocDBSubnetGroup(ctx, clients, res)
	}
	if res.RawStruct == nil {
		return nil, errRawStructMissing
	}
	return nil, errDbcNoClusterDetail
}

// dbcRDSSubnetGroup resolves the subnet group for an Aurora / Multi-AZ DB
// cluster (rdstypes.DBCluster shape) by calling c.RDS.DescribeDBSubnetGroups.
// Aurora subnet groups belong to the RDS API, not the DocDB API.
func dbcRDSSubnetGroup(ctx context.Context, clients any, res resource.Resource) (*dbcSubnetGroupInfo, error) {
	name := dbcClusterSubnetGroupName(res.RawStruct)
	if name == "" {
		return nil, nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.RDS == nil {
		return nil, errDbcNoClusterDetail
	}
	groups, _, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]rdstypes.DBSubnetGroup, *string, error) {
		out, err := c.RDS.DescribeDBSubnetGroups(ctx, &rds.DescribeDBSubnetGroupsInput{
			DBSubnetGroupName: &name,
			Marker:            marker,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.DBSubnetGroups, out.Marker, nil
	})
	if err != nil {
		return nil, fmt.Errorf("describing subnet group %s: %w", name, err)
	}
	if len(groups) == 0 {
		return nil, nil
	}
	sg := groups[0]
	info := &dbcSubnetGroupInfo{VpcId: sg.VpcId}
	for _, s := range sg.Subnets {
		info.Subnets = append(info.Subnets, dbcSubnetIdentifier{SubnetIdentifier: s.SubnetIdentifier})
	}
	return info, nil
}

// dbcDocDBSubnetGroup resolves the subnet group for a DocumentDB cluster
// (docdb_types.DBCluster shape) by calling c.DocDB.DescribeDBSubnetGroups.
func dbcDocDBSubnetGroup(ctx context.Context, clients any, res resource.Resource) (*dbcSubnetGroupInfo, error) {
	name := dbcClusterSubnetGroupName(res.RawStruct)
	if name == "" {
		return nil, nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.DocDB == nil {
		return nil, errDbcNoClusterDetail
	}
	groups, _, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]docdb_types.DBSubnetGroup, *string, error) {
		out, err := c.DocDB.DescribeDBSubnetGroups(ctx, &docdb.DescribeDBSubnetGroupsInput{
			DBSubnetGroupName: &name,
			Marker:            marker,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.DBSubnetGroups, out.Marker, nil
	})
	if err != nil {
		return nil, fmt.Errorf("describing subnet group %s: %w", name, err)
	}
	if len(groups) == 0 {
		return nil, nil
	}
	sg := groups[0]
	info := &dbcSubnetGroupInfo{VpcId: sg.VpcId}
	for _, s := range sg.Subnets {
		info.Subnets = append(info.Subnets, dbcSubnetIdentifier{SubnetIdentifier: s.SubnetIdentifier})
	}
	return info, nil
}

// checkDbcSecrets resolves the Secrets Manager secret managed for this cluster's
// master user password. DBCluster.MasterUserSecret.SecretArn holds the full
// secret ARN; we match it against the secrets cache by ARN.
// Handles both docdb_types.DBCluster and rdstypes.DBCluster shapes.
func checkDbcSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	secretARN := dbcClusterMasterSecretARN(res.RawStruct)
	if secretARN == "" {
		// Parent has no MasterUserSecret — true regardless of whether the
		// RawStruct shape was a recognised cluster. Returning Count=0 is
		// definitive: there is no cluster-managed master secret to associate.
		return unreadZero(res, resource.ProvenZero("secrets", "secretARN"))
	}

	secretList, truncated, err := relatedResourcesFor(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.ErrorRelated("secrets", err)
	}
	if secretList == nil {
		return resource.UnknownRelated("secrets")
	}

	var ids []string
	for _, secretRes := range secretList {
		if secretRes.Fields["arn"] == secretARN {
			ids = append(ids, secretRes.ID)
			continue
		}
		raw, rawOK := assertStruct[smtypes.SecretListEntry](secretRes.RawStruct)
		if rawOK && raw.ARN != nil && *raw.ARN == secretARN {
			ids = append(ids, secretRes.ID)
		}
	}
	return unreadZeroScanned(res, len(secretList), relatedResultTrunc("secrets", ids, truncated))
}

// checkDbcKMS extracts the KMS key from the DBCluster's KmsKeyId field.
// KmsKeyId is a KMS key ARN. Returns the key ID (last segment after "/").
// Handles both docdb_types.DBCluster and rdstypes.DBCluster shapes.
func checkDbcKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	keyID := dbcClusterKmsKeyID(res.RawStruct)
	if keyID == "" {
		return unreadZero(res, resource.ProvenZero("kms", "keyID"))
	}
	keyID = kmsRefFromField(keyID, res.Type)
	return unreadZero(res, kmsRelated(ctx, clients, cache, []string{keyID}))
}

// checkDbcCTEvents looks up cached CloudTrail events for the cluster's
// DBClusterIdentifier. Universal pivot — every registered type gets one;
// see the Policy section of docs/related-resources.md. FetchFilter["ResourceName"] is always
// set so the caller can do a filtered re-fetch; Count is "unknown" (windowed)
// — the panel renders the visible page count rather than a total.
// ResourceType is "AWS::RDS::DBCluster" — both DocDB and Aurora clusters share
// this CloudTrail resource type (docs/resources/dbc.md).
// Built via BuildCTEventsPivotChecker — see ct_events_pivot.go for the shared logic.
var checkDbcCTEvents = BuildCTEventsPivotChecker(CTEventsPivotConfig{
	IDExtractor: func(res resource.Resource) string {
		id := dbcClusterIdentifier(res.RawStruct)
		if id == "" {
			id = res.ID
		}
		return id
	},
})
