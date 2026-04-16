// msk_related.go contains MSK cluster related-resource checker functions.
package aws

import (
	"context"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("msk", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkMSKAlarms, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkMSKSG, NeedsTargetCache: false},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkMSKKMS},
	})

	// kafkatypes.Cluster: Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId → kms
	resource.RegisterNavigableFields("msk", []resource.NavigableField{
		{FieldPath: "Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId", TargetType: "kms"},
	})
}



// checkMSKAlarms checks the cache for CloudWatch alarms with "Cluster Name" dimension matching this cluster.
func checkMSKAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := mskRelatedResources(ctx, clients, cache, "alarm")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1, Err: err}
	}
	if alarmList == nil {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
	}

	var ids []string
	for _, alarmRes := range alarmList {
		rawAlarm, ok := assertStruct[cwtypes.MetricAlarm](alarmRes.RawStruct)
		if !ok {
			continue
		}
		for _, d := range rawAlarm.Dimensions {
			if d.Name != nil && *d.Name == "Cluster Name" && d.Value != nil && *d.Value == clusterName {
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

// checkMSKSG returns the security groups associated with the MSK cluster's broker nodes.
// It reads the SecurityGroups field from the Provisioned.BrokerNodeGroupInfo struct (Pattern F).
func checkMSKSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	if cluster.Provisioned == nil || cluster.Provisioned.BrokerNodeGroupInfo == nil {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	ids := cluster.Provisioned.BrokerNodeGroupInfo.SecurityGroups
	if len(ids) == 0 {
		return resource.RelatedCheckResult{TargetType: "sg", Count: 0}
	}
	return relatedResult("sg", ids)
}

// mskRelatedResources returns the resource list for target from cache or by fetching the first page.
func mskRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}


// checkMSKKMS extracts the KMS key ID from the MSK cluster's
// Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId field.
// Pattern F — no cache needed.
func checkMSKKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok || cluster.Provisioned == nil ||
		cluster.Provisioned.EncryptionInfo == nil ||
		cluster.Provisioned.EncryptionInfo.EncryptionAtRest == nil ||
		cluster.Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId == nil ||
		*cluster.Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	return relatedResult("kms", []string{*cluster.Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId})
}




