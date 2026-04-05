package aws

import (
	"context"

	docdb_types "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkDbcSG reads VpcSecurityGroups[] from the DBCluster RawStruct and returns their IDs.
// Pattern F — no cache needed.
func checkDbcSG(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
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
