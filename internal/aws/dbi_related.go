package aws

import (
	"context"
	"strings"

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
