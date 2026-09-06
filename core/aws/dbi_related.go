// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkDbiSG reads VpcSecurityGroups from the DBInstance RawStruct and returns their IDs.
// Pattern F — no cache needed.
func checkDbiSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	var ids []string
	for _, sg := range db.VpcSecurityGroups {
		if sg.VpcSecurityGroupId != nil && *sg.VpcSecurityGroupId != "" {
			ids = append(ids, *sg.VpcSecurityGroupId)
		}
	}
	if len(ids) == 0 {
		return resource.KnownRelated("sg", nil, false)
	}
	return relatedResult("sg", ids)
}

// checkDbiKMS reads the KmsKeyId ARN from the DBInstance RawStruct and extracts the UUID suffix.
// Pattern F — no cache needed.
func checkDbiKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("kms")
	}
	if db.KmsKeyId == nil || *db.KmsKeyId == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	keyID := kmsKeyIDFromField(*db.KmsKeyId, res.Type)
	if keyID == "" {
		return resource.KnownRelated("kms", nil, false)
	}
	return relatedResult("kms", []string{keyID})
}

// checkDbiSubnets reads DBSubnetGroup.Subnets from the DBInstance RawStruct and returns their IDs.
// Pattern F — no cache needed.
func checkDbiSubnets(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if db.DBSubnetGroup == nil || len(db.DBSubnetGroup.Subnets) == 0 {
		return resource.KnownRelated("subnet", nil, false)
	}
	var ids []string
	for _, subnet := range db.DBSubnetGroup.Subnets {
		if subnet.SubnetIdentifier != nil && *subnet.SubnetIdentifier != "" {
			ids = append(ids, *subnet.SubnetIdentifier)
		}
	}
	if len(ids) == 0 {
		return resource.KnownRelated("subnet", nil, false)
	}
	return relatedResult("subnet", ids)
}

// checkDbiAlarm searches the alarm cache for alarms with a "DBInstanceIdentifier" dimension
// matching this DB instance's identifier.
// Pattern D — dimension-based lookup.
func checkDbiAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "", "DBInstanceIdentifier", res.ID)
}

// checkDbiDBISnap searches the dbi-snap cache for snapshots whose DBInstanceIdentifier
// matches this DB instance's identifier.
// Pattern C — reverse cache lookup.
func checkDbiDBISnap(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dbIdentifier := res.ID
	if dbIdentifier == "" {
		return resource.KnownRelated("dbi-snap", nil, false)
	}

	snapList, truncated, err := relatedResourcesFor(ctx, clients, cache, "dbi-snap")
	if err != nil {
		return resource.ErrorRelated("dbi-snap", err)
	}
	if snapList == nil {
		return resource.UnknownRelated("dbi-snap")
	}

	var ids []string
	for _, snapRes := range snapList {
		snap, ok := assertStruct[rdstypes.DBSnapshot](snapRes.RawStruct)
		if !ok {
			continue
		}
		if snap.DBInstanceIdentifier != nil && *snap.DBInstanceIdentifier == dbIdentifier {
			ids = append(ids, snapRes.ID)
		}
	}
	return relatedResultTrunc("dbi-snap", ids, truncated)
}

// checkDBILogs searches the logs cache for log groups matching the RDS naming convention.
// Pattern N — naming convention: /aws/rds/instance/{db-instance-id}/{log-type}
func checkDBILogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dbID := res.ID
	if dbID == "" {
		return resource.KnownRelated("logs", nil, false)
	}

	prefix := "/aws/rds/instance/" + dbID + "/"

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}

	var ids []string
	for _, logRes := range logList {
		if strings.HasPrefix(logRes.ID, prefix) {
			ids = append(ids, logRes.ID)
		}
	}
	return relatedResultTrunc("logs", ids, truncated)
}

// checkDbiSecrets resolves the Secrets Manager secret managed for this RDS
// instance's master user password. DBInstance.MasterUserSecret.SecretArn holds
// the full secret ARN; we match it against the secrets cache by ARN, ID (secret
// name), or Name.
func checkDbiSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("secrets")
		}
		// A parent that is not a DBInstance carries no MasterUserSecret, so
		// there is no link to find.
		return resource.KnownRelated("secrets", nil, false)
	}
	if db.MasterUserSecret == nil || db.MasterUserSecret.SecretArn == nil || *db.MasterUserSecret.SecretArn == "" {
		return resource.KnownRelated("secrets", nil, false)
	}
	secretARN := *db.MasterUserSecret.SecretArn

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
	return relatedResultTrunc("secrets", ids, truncated)
}

func checkDbiVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	inst, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok || inst.DBSubnetGroup == nil || inst.DBSubnetGroup.VpcId == nil || *inst.DBSubnetGroup.VpcId == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("vpc")
		}
		return resource.KnownRelated("vpc", nil, false)
	}
	return relatedResult("vpc", []string{*inst.DBSubnetGroup.VpcId})
}

// checkDbiDBC returns the Aurora/RDS cluster this DB instance belongs to, if
// any. DBInstance.DBClusterIdentifier is non-nil only for Aurora/RDS cluster
// members. We match that identifier against the dbc cache by ID/Name.
func checkDbiDBC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		if res.RawStruct == nil {
			return resource.UnknownRelated("dbc")
		}
		return resource.KnownRelated("dbc", nil, false)
	}
	if db.DBClusterIdentifier == nil || *db.DBClusterIdentifier == "" {
		return resource.KnownRelated("dbc", nil, false)
	}
	// In-body: DBClusterIdentifier IS the cluster's resource id (dbc keyed by identifier).
	return relatedResult("dbc", []string{*db.DBClusterIdentifier})
}

// checkDbiRole extracts IAM role ARNs from the DBInstance's AssociatedRoles
// and MonitoringRoleArn fields. Each DBInstanceRole has a RoleArn; we extract
// the role name (last segment after "/"). MonitoringRoleArn is the enhanced
// monitoring role.
func checkDbiRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("role")
	}
	var ids []string
	for _, r := range db.AssociatedRoles {
		if r.RoleArn == nil || *r.RoleArn == "" {
			continue
		}
		arn := *r.RoleArn
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			ids = append(ids, arn[idx+1:])
		} else {
			ids = append(ids, arn)
		}
	}
	if db.MonitoringRoleArn != nil && *db.MonitoringRoleArn != "" {
		arn := *db.MonitoringRoleArn
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			ids = append(ids, arn[idx+1:])
		} else {
			ids = append(ids, arn)
		}
	}
	return relatedResult("role", ids)
}

// checkDbiENI resolves the ENIs that RDS provisions for this DB instance via
// a single ec2:DescribeNetworkInterfaces call (Pattern C). RDS manages its
// ENIs with the description "RDSNetworkInterface" and attaches them to the
// instance's security groups. We filter by description + group-id to scope.
func checkDbiENI(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("eni")
	}
	var sgIDs []string
	for _, sg := range db.VpcSecurityGroups {
		if sg.VpcSecurityGroupId != nil && *sg.VpcSecurityGroupId != "" {
			sgIDs = append(sgIDs, *sg.VpcSecurityGroupId)
		}
	}
	if len(sgIDs) == 0 {
		return resource.KnownRelated("eni", nil, false)
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.EC2 == nil {
		return resource.UnknownRelated("eni")
	}
	descName := "description"
	descVal := "RDSNetworkInterface"
	groupName := "group-id"
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeNetworkInterfacesOutput, error) {
		return c.EC2.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			Filters: []ec2types.Filter{
				{Name: &descName, Values: []string{descVal}},
				{Name: &groupName, Values: sgIDs},
			},
		})
	})
	if err != nil {
		return resource.ErrorRelated("eni", err)
	}
	var ids []string
	for _, ni := range out.NetworkInterfaces {
		if ni.NetworkInterfaceId != nil && *ni.NetworkInterfaceId != "" {
			ids = append(ids, *ni.NetworkInterfaceId)
		}
	}
	return relatedResult("eni", ids)
}

// checkDbiCTEvents checks cached CloudTrail events for references to the DB instance.
// Returns an unknown result when the cache is truncated or a cache miss occurs.
// FetchFilter["ResourceName"] is always set so the caller can do a filtered re-fetch.
func checkDbiCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dbID := res.ID
	if dbID == "" {
		return resource.KnownRelated("ct-events", nil, false)
	}
	fetchFilter := map[string]string{"ResourceName": dbID}
	eventList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.ErrorRelated("ct-events", err).WithFetchFilter(fetchFilter)
	}
	if eventList == nil {
		return resource.DeferredRelated("ct-events", fetchFilter)
	}
	var ids []string
	for _, eventRes := range eventList {
		raw, ok := assertStruct[cloudtrailtypes.Event](eventRes.RawStruct)
		if ok {
			matched := false
			for _, rr := range raw.Resources {
				if rr.ResourceName != nil && *rr.ResourceName == dbID {
					matched = true
					break
				}
			}
			if matched {
				ids = append(ids, eventRes.ID)
			}
			continue
		}
		if eventRes.Fields["resource_name"] == dbID {
			ids = append(ids, eventRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return relatedResultTrunc("ct-events", nil, true).WithFetchFilter(fetchFilter)
	}
	return relatedResultTrunc("ct-events", ids, truncated).WithFetchFilter(fetchFilter)
}
