package aws

import (
	"context"
	"strings"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDbiSG reads VpcSecurityGroups from the DBInstance RawStruct and returns their IDs.
// Pattern F — no cache needed.
func checkDbiSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	var ids []string
	for _, sg := range db.VpcSecurityGroups {
		if sg.VpcSecurityGroupId != nil && *sg.VpcSecurityGroupId != "" {
			ids = append(ids, *sg.VpcSecurityGroupId)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	return relatedResult("sg", ids)
}

// checkDbiKMS reads the KmsKeyId ARN from the DBInstance RawStruct and extracts the UUID suffix.
// Pattern F — no cache needed.
func checkDbiKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "kms", Count: -1}
	}
	if db.KmsKeyId == nil || *db.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	arn := *db.KmsKeyId
	idx := strings.LastIndex(arn, "/")
	if idx < 0 || idx == len(arn)-1 {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := arn[idx+1:]
	return relatedResult("kms", []string{keyID})
}

// checkDbiSubnets reads DBSubnetGroup.Subnets from the DBInstance RawStruct and returns their IDs.
// Pattern F — no cache needed.
func checkDbiSubnets(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if db.DBSubnetGroup == nil || len(db.DBSubnetGroup.Subnets) == 0 {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	var ids []string
	for _, subnet := range db.DBSubnetGroup.Subnets {
		if subnet.SubnetIdentifier != nil && *subnet.SubnetIdentifier != "" {
			ids = append(ids, *subnet.SubnetIdentifier)
		}
	}
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	return relatedResult("subnet", ids)
}

// checkDbiAlarm searches the alarm cache for alarms with a "DBInstanceIdentifier" dimension
// matching this DB instance's identifier.
// Pattern D — dimension-based lookup.
func checkDbiAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dbIdentifier := res.ID
	if dbIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := dbiRelatedResources(ctx, clients, cache, "alarm")
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
			if d.Name != nil && *d.Name == "DBInstanceIdentifier" && d.Value != nil && *d.Value == dbIdentifier {
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

// checkDbiDBISnap searches the dbi-snap cache for snapshots whose DBInstanceIdentifier
// matches this DB instance's identifier.
// Pattern C — reverse cache lookup.
func checkDbiDBISnap(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dbIdentifier := res.ID
	if dbIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "dbi-snap", Count: 0}
	}

	snapList, truncated, err := dbiRelatedResources(ctx, clients, cache, "dbi-snap")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "dbi-snap", Count: -1, Err: err}
	}
	if snapList == nil {
		return resource.ApproximateZero("dbi-snap")
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
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("dbi-snap")
	}
	return relatedResult("dbi-snap", ids)
}

// checkDBILogs searches the logs cache for log groups matching the RDS naming convention.
// Pattern N — naming convention: /aws/rds/instance/{db-instance-id}/{log-type}
func checkDBILogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dbID := res.ID
	if dbID == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}

	prefix := "/aws/rds/instance/" + dbID + "/"

	logList, truncated, err := dbiRelatedResources(ctx, clients, cache, "logs")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1, Err: err}
	}
	if logList == nil {
		return resource.ApproximateZero("logs")
	}

	var ids []string
	for _, logRes := range logList {
		if strings.HasPrefix(logRes.ID, prefix) {
			ids = append(ids, logRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("logs")
	}
	return relatedResult("logs", ids)
}

// dbiRelatedResources returns the resource list for target from cache or by fetching the first page.
func dbiRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}

// checkDbiSecrets resolves the Secrets Manager secret managed for this RDS
// instance's master user password. DBInstance.MasterUserSecret.SecretArn holds
// the full secret ARN; we match it against the secrets cache by ARN, ID (secret
// name), or Name.
func checkDbiSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	if db.MasterUserSecret == nil || db.MasterUserSecret.SecretArn == nil || *db.MasterUserSecret.SecretArn == "" {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	secretARN := *db.MasterUserSecret.SecretArn

	secretList, truncated, err := dbiRelatedResources(ctx, clients, cache, "secrets")
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

func checkDbiVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	inst, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok || inst.DBSubnetGroup == nil || inst.DBSubnetGroup.VpcId == nil || *inst.DBSubnetGroup.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*inst.DBSubnetGroup.VpcId})
}

// checkDbiDBC returns the Aurora/RDS cluster this DB instance belongs to, if
// any. DBInstance.DBClusterIdentifier is non-nil only for Aurora/RDS cluster
// members. We match that identifier against the dbc cache by ID/Name.
func checkDbiDBC(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: -1}
	}
	if db.DBClusterIdentifier == nil || *db.DBClusterIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: 0}
	}
	clusterID := *db.DBClusterIdentifier

	dbcList, truncated, err := dbiRelatedResources(ctx, clients, cache, "dbc")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "dbc", Count: -1, Err: err}
	}
	if dbcList == nil {
		return resource.ApproximateZero("dbc")
	}
	var ids []string
	for _, dbcRes := range dbcList {
		if dbcRes.ID == clusterID || dbcRes.Name == clusterID {
			ids = append(ids, dbcRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.ApproximateZero("dbc")
	}
	return relatedResult("dbc", ids)
}

// checkDbiRole extracts IAM role ARNs from the DBInstance's AssociatedRoles
// and MonitoringRoleArn fields. Each DBInstanceRole has a RoleArn; we extract
// the role name (last segment after "/"). MonitoringRoleArn is the enhanced
// monitoring role.
func checkDbiRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	db, ok := assertStruct[rdstypes.DBInstance](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
	}
	var sgIDs []string
	for _, sg := range db.VpcSecurityGroups {
		if sg.VpcSecurityGroupId != nil && *sg.VpcSecurityGroupId != "" {
			sgIDs = append(sgIDs, *sg.VpcSecurityGroupId)
		}
	}
	if len(sgIDs) == 0 {
		return resource.RelatedCheckResult{TargetType: "eni", Count: 0}
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.EC2 == nil {
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "eni", Count: -1, Err: err}
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
// Returns Count=-1 (unknown) when the cache is truncated or a cache miss occurs.
// FetchFilter["ResourceName"] is always set so the caller can do a filtered re-fetch.
func checkDbiCTEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dbID := res.ID
	if dbID == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
	}
	fetchFilter := map[string]string{"ResourceName": dbID}
	eventList, truncated, err := dbiRelatedResources(ctx, clients, cache, "ct-events")
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
	if truncated {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, FetchFilter: fetchFilter}
	}
	result := relatedResult("ct-events", ids)
	result.FetchFilter = fetchFilter
	return result
}
