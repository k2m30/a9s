// redshift_related.go contains Redshift Cluster related-resource checker functions.
package aws

import (
	"context"
	"strings"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("redshift", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkRedshiftAlarms, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkRedshiftSG},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkRedshiftVPC},
		{TargetType: "role", DisplayName: "IAM Role", Checker: checkRedshiftRole},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkRedshiftKMS},
	})

	// redshifttypes.Cluster: VpcId
	resource.RegisterNavigableFields("redshift", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
	})
}


// checkRedshiftAlarms checks the cache for CloudWatch alarms with ClusterIdentifier dimension matching this cluster.
func checkRedshiftAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterID := res.ID
	if clusterID == "" {
		return resource.RelatedCheckResult{TargetType: "alarm", Count: 0}
	}

	alarmList, truncated, err := redshiftRelatedResources(ctx, clients, cache, "alarm")
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
			if d.Name != nil && *d.Name == "ClusterIdentifier" && d.Value != nil && *d.Value == clusterID {
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

// checkRedshiftSG extracts security group IDs from the Redshift Cluster's
// VpcSecurityGroups slice.
// Pattern F — no cache needed.
func checkRedshiftSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "sg", Count: -1}
	}
	var ids []string
	for _, vsg := range cluster.VpcSecurityGroups {
		if vsg.VpcSecurityGroupId != nil && *vsg.VpcSecurityGroupId != "" {
			ids = append(ids, *vsg.VpcSecurityGroupId)
		}
	}
	return relatedResult("sg", ids)
}

// checkRedshiftVPC returns the VPC this Redshift cluster runs in (Pattern R).
// Reads Cluster.VpcId from the RawStruct.
func checkRedshiftVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	if cluster.VpcId == nil || *cluster.VpcId == "" {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	return relatedResult("vpc", []string{*cluster.VpcId})
}

// checkRedshiftRole extracts IAM role ARNs from the Redshift Cluster's IamRoles slice.
// Each ClusterIamRole has an IamRoleArn field; we extract the role name (last segment after "/").
// Pattern F — no cache needed.
func checkRedshiftRole(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	if len(cluster.IamRoles) == 0 {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	var ids []string
	for _, r := range cluster.IamRoles {
		if r.IamRoleArn == nil || *r.IamRoleArn == "" {
			continue
		}
		arn := *r.IamRoleArn
		if idx := strings.LastIndex(arn, "/"); idx >= 0 && idx < len(arn)-1 {
			ids = append(ids, arn[idx+1:])
		} else {
			ids = append(ids, arn)
		}
	}
	return relatedResult("role", ids)
}

// checkRedshiftKMS extracts the KMS key ID from the Redshift Cluster's KmsKeyId
// field. Pattern F — no cache needed.
func checkRedshiftKMS(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok || cluster.KmsKeyId == nil || *cluster.KmsKeyId == "" {
		return resource.RelatedCheckResult{TargetType: "kms", Count: 0}
	}
	keyID := *cluster.KmsKeyId
	if idx := strings.LastIndex(keyID, "/"); idx >= 0 && idx < len(keyID)-1 {
		keyID = keyID[idx+1:]
	}
	return relatedResult("kms", []string{keyID})
}

// redshiftRelatedResources returns the resource list for target from cache or by fetching the first page.
func redshiftRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}




