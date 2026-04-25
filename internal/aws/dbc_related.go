package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdb_types "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDbcSG reads VpcSecurityGroups[] from the DBCluster RawStruct and returns their IDs.
// Pattern F — no cache needed.
func checkDbcSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[docdb_types.DBCluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	var ids []string
	for _, sg := range cluster.VpcSecurityGroups {
		if sg.VpcSecurityGroupId != nil && *sg.VpcSecurityGroupId != "" {
			ids = append(ids, *sg.VpcSecurityGroupId)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	return relatedResult("sg", ids)
}

// checkDbcAlarm searches the alarm cache for alarms with a "DBClusterIdentifier" dimension
// matching this DocumentDB cluster's identifier.
// Pattern D — dimension-based lookup.
func checkDbcAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterID := res.ID
	if clusterID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := dbcRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.ApproximateZero("alarm")
	}

	var ids []string
	for _, alarmRes := range alarmList {
		alarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range alarm.Dimensions {
			if d.Name != nil && *d.Name == "DBClusterIdentifier" && d.Value != nil && *d.Value == clusterID {
				ids = append(ids, alarmRes.ID)
				break
			}
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("alarm")
	}
	return relatedResult("alarm", ids)
}

// checkDbcLogs searches the logs cache for log groups matching the DocumentDB cluster's
// naming convention: /aws/docdb/{clusterID}/audit or /aws/docdb/{clusterID}/profiler.
// Pattern N — naming convention.
func checkDbcLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterID := res.ID
	if clusterID == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	logList, truncated, err := dbcRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.ApproximateZero("logs")
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("logs")
	}
	return relatedResult("logs", ids)
}

// dbcRelatedResources returns the resource list for target from cache or by fetching the first page.
func dbcRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// checkDbcDBI does a reverse lookup — scans the dbi cache for DBInstances
// whose DBClusterIdentifier matches this cluster's identifier. Aurora /
// DocumentDB clusters own one or more DBInstance members.
func checkDbcDBI(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterID := res.ID
	if cluster, ok := assertStruct[docdb_types.DBCluster](res.RawStruct); ok && cluster.DBClusterIdentifier != nil && *cluster.DBClusterIdentifier != "" {
		clusterID = *cluster.DBClusterIdentifier
	}
	if clusterID == "" {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: 0}
	}

	dbiList, truncated, err := dbcRelatedResources(ctx, clients, cache, "dbi")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "dbi", Count: -1, Err: err}
	}
	if dbiList == nil {
		return resource.ApproximateZero("dbi")
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("dbi")
	}
	return relatedResult("dbi", ids)
}

// checkDbcDbcSnap does a reverse lookup — scans the dbc-snap cache for
// snapshots whose DBClusterIdentifier matches this cluster's identifier.
func checkDbcDbcSnap(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterID := res.ID
	if cluster, ok := assertStruct[docdb_types.DBCluster](res.RawStruct); ok && cluster.DBClusterIdentifier != nil && *cluster.DBClusterIdentifier != "" {
		clusterID = *cluster.DBClusterIdentifier
	}
	if clusterID == "" {
		return resource.RelatedCheckResult{TargetType: "dbc-snap", Count: 0}
	}

	snapList, truncated, err := dbcRelatedResources(ctx, clients, cache, "dbc-snap")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "dbc-snap", Count: -1, Err: err}
	}
	if snapList == nil {
		return resource.ApproximateZero("dbc-snap")
	}

	var ids []string
	for _, snapRes := range snapList {
		snap, ok := assertStruct[docdb_types.DBClusterSnapshot](snapRes.RawStruct)
		if !ok {
			continue
		}
		if snap.DBClusterIdentifier != nil && *snap.DBClusterIdentifier == clusterID {
			ids = append(ids, snapRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("dbc-snap")
	}
	return relatedResult("dbc-snap", ids)
}

// checkDbcSubnet resolves the subnets inside the cluster's DBSubnetGroup via
// a single docdb:DescribeDBSubnetGroups call (Pattern C — live API). The
// DBCluster response only carries the subnet-group name; this call resolves
// it to the concrete Subnets slice.
func checkDbcSubnet(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	sng := dbcSubnetGroup(ctx, clients, res)
	if sng == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	var ids []string
	for _, s := range sng.Subnets {
		if s.SubnetIdentifier != nil && *s.SubnetIdentifier != "" {
			ids = append(ids, *s.SubnetIdentifier)
		}
	}
	return relatedResult("subnet", ids)
}

// checkDbcVPC resolves the VPC that hosts the cluster's subnet group via a
// single docdb:DescribeDBSubnetGroups call (Pattern C).
func checkDbcVPC(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	sng := dbcSubnetGroup(ctx, clients, res)
	if sng == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	if sng.VpcId == nil || *sng.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*sng.VpcId})
}

// dbcSubnetGroup makes a single docdb:DescribeDBSubnetGroups call for this
// cluster's DBSubnetGroup name, wrapped in RetryOnThrottle. Returns nil if
// the call cannot be made or the group was not found.
func dbcSubnetGroup(ctx context.Context, clients any, res resource.Resource) *docdb_types.DBSubnetGroup {
	cluster, ok := assertStruct[docdb_types.DBCluster](res.RawStruct)
	if !ok || cluster.DBSubnetGroup == nil || *cluster.DBSubnetGroup == "" {
		return nil
	}
	name := *cluster.DBSubnetGroup

	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.DocDB == nil {
		return nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*docdb.DescribeDBSubnetGroupsOutput, error) {
		return c.DocDB.DescribeDBSubnetGroups(ctx, &docdb.DescribeDBSubnetGroupsInput{
			DBSubnetGroupName: &name,
		})
	})
	if err != nil || out == nil || len(out.DBSubnetGroups) == 0 {
		return nil
	}
	return &out.DBSubnetGroups[0]
}

// checkDbcSecrets resolves the Secrets Manager secret managed for this cluster's
// master user password. DBCluster.MasterUserSecret.SecretArn holds the full
// secret ARN; we match it against the secrets cache by ARN.
func checkDbcSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[docdb_types.DBCluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	if cluster.MasterUserSecret == nil || cluster.MasterUserSecret.SecretArn == nil || *cluster.MasterUserSecret.SecretArn == "" {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	secretARN := *cluster.MasterUserSecret.SecretArn

	secretList, truncated, err := dbcRelatedResources(ctx, clients, cache, "secrets")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1, Err: err}
	}
	if secretList == nil {
		return resource.ApproximateZero("secrets")
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("secrets")
	}
	return relatedResult("secrets", ids)
}

// checkDbcKMS extracts the KMS key from the DocumentDB DBCluster's KmsKeyId field.
// KmsKeyId is a KMS key ARN. Returns the key ID (last segment after "/").
// Pattern F — no cache needed.
func checkDbcKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[docdb_types.DBCluster](res.RawStruct)
	if !ok || cluster.KmsKeyId == nil || *cluster.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *cluster.KmsKeyId
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}
