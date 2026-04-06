package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDbiSG reads VpcSecurityGroups from the DBInstance RawStruct and returns their IDs.
// Pattern F — no cache needed.
func checkDbiSG(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
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
func checkDbiKMS(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
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
func checkDbiSubnets(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
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
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}
	return relatedResult("alarm", ids)
}

// checkDbiRDSSnap searches the rds-snap cache for snapshots whose DBInstanceIdentifier
// matches this DB instance's identifier.
// Pattern C — reverse cache lookup.
func checkDbiRDSSnap(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	dbIdentifier := res.ID
	if dbIdentifier == "" {
		return resource.RelatedCheckResult{TargetType: "rds-snap", Count: 0}
	}

	snapList, truncated, err := dbiRelatedResources(ctx, clients, cache, "rds-snap")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "rds-snap", Count: -1, Err: err}
	}
	if snapList == nil {
		return resource.RelatedCheckResult{TargetType: "rds-snap", Count: -1}
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
		return resource.RelatedCheckResult{TargetType: "rds-snap", Count: -1}
	}
	return relatedResult("rds-snap", ids)
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
