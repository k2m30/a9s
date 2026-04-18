// msk_related.go contains MSK cluster related-resource checker functions.
package aws

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterRelated("msk", []resource.RelatedDef{
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkMSKAlarms, NeedsTargetCache: true},
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkMSKSG, NeedsTargetCache: false},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkMSKKMS},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkMSKLambda, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkMSKCFN, NeedsTargetCache: true},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkMSKSubnet},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkMSKVPC, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkMSKLogs},
		{TargetType: "s3", DisplayName: "S3 (broker logs)", Checker: checkMSKS3},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkMSKSecrets},
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

// checkMSKLambda scans the Lambda cache for functions whose first
// EventSourceArn (captured in Fields["event_source_arn"] at fetch time) matches
// this cluster's ARN. MSK → Lambda triggers use the cluster ARN as the event
// source. Secondary event sources are not captured in the field (only the first
// is stored), so this check may under-count.
func checkMSKLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok || cluster.ClusterArn == nil || *cluster.ClusterArn == "" {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: 0}
	}
	clusterARN := *cluster.ClusterArn

	lambdaList, truncated, err := mskRelatedResources(ctx, clients, cache, "lambda")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1, Err: err}
	}
	if lambdaList == nil {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}

	var ids []string
	for _, fn := range lambdaList {
		if fn.Fields["event_source_arn"] == clusterARN {
			ids = append(ids, fn.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "lambda", Count: -1}
	}
	return relatedResult("lambda", ids)
}

// checkMSKCFN matches the MSK cluster's aws:cloudformation:stack-name tag to
// a CFN stack (Pattern C). kafkatypes.Cluster.Tags is a map[string]string
// populated at list time (ListClustersV2). Returns Count: 0 when no tag is set.
func checkMSKCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	stackName := cluster.Tags["aws:cloudformation:stack-name"]
	if stackName == "" {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: 0}
	}

	cfnList, truncated, err := mskRelatedResources(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1, Err: err}
	}
	if cfnList == nil {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}

	var ids []string
	for _, cfnRes := range cfnList {
		if cfnRes.ID == stackName || cfnRes.Name == stackName || cfnRes.Fields["stack_name"] == stackName {
			ids = append(ids, cfnRes.ID)
			continue
		}
		rawCFN, cfnOk := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if cfnOk && rawCFN.StackName != nil && *rawCFN.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "cfn", Count: -1}
	}
	return relatedResult("cfn", ids)
}

// checkMSKSubnet returns the subnets the cluster's broker nodes run in
// (Provisioned.BrokerNodeGroupInfo.ClientSubnets). Pattern F — no cache needed.
func checkMSKSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: -1}
	}
	if cluster.Provisioned == nil || cluster.Provisioned.BrokerNodeGroupInfo == nil {
		return resource.RelatedCheckResult{TargetType: "subnet", Count: 0}
	}
	ids := cluster.Provisioned.BrokerNodeGroupInfo.ClientSubnets
	return relatedResult("subnet", ids)
}

// checkMSKVPC returns the VPC that hosts the cluster's broker subnets by
// looking up the first ClientSubnet in the subnet cache and reading its VpcId.
// Pattern F + C.
func checkMSKVPC(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}
	if cluster.Provisioned == nil || cluster.Provisioned.BrokerNodeGroupInfo == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}
	subnets := cluster.Provisioned.BrokerNodeGroupInfo.ClientSubnets
	if len(subnets) == 0 {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: 0}
	}

	subnetList, _, err := mskRelatedResources(ctx, clients, cache, "subnet")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1, Err: err}
	}
	if subnetList == nil {
		return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
	}

	want := subnets[0]
	for _, subnetRes := range subnetList {
		if subnetRes.ID != want {
			continue
		}
		sn, snOk := assertStruct[ec2types.Subnet](subnetRes.RawStruct)
		if !snOk || sn.VpcId == nil || *sn.VpcId == "" {
			continue
		}
		return relatedResult("vpc", []string{*sn.VpcId})
	}
	return resource.RelatedCheckResult{TargetType: "vpc", Count: -1}
}

// checkMSKLogs would resolve the CloudWatch log group configured for broker
// logs in LoggingInfo.BrokerLogs.CloudWatchLogs.LogGroup. ListClustersV2
// returns kafkatypes.Cluster that carries LoggingInfo when set on the cluster;
// when unset, Count: 0. This is a forward lookup.
func checkMSKLogs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "logs", Count: -1}
	}
	if cluster.Provisioned == nil ||
		cluster.Provisioned.LoggingInfo == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs.CloudWatchLogs == nil {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	cw := cluster.Provisioned.LoggingInfo.BrokerLogs.CloudWatchLogs
	if cw.Enabled == nil || !*cw.Enabled || cw.LogGroup == nil || *cw.LogGroup == "" {
		return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
	}
	return relatedResult("logs", []string{*cw.LogGroup})
}

// checkMSKS3 extracts the S3 bucket configured for broker log delivery in
// LoggingInfo.BrokerLogs.S3.Bucket. Forward lookup from kafkatypes.Cluster.
func checkMSKS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "s3", Count: -1}
	}
	if cluster.Provisioned == nil ||
		cluster.Provisioned.LoggingInfo == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs.S3 == nil {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	s3Log := cluster.Provisioned.LoggingInfo.BrokerLogs.S3
	if s3Log.Enabled == nil || !*s3Log.Enabled || s3Log.Bucket == nil || *s3Log.Bucket == "" {
		return resource.RelatedCheckResult{TargetType: "s3", Count: 0}
	}
	return relatedResult("s3", []string{*s3Log.Bucket})
}

// checkMSKSecrets calls kafka:ListScramSecrets(clusterArn) and returns the
// Secrets Manager secret names associated with this cluster's SCRAM auth.
// Pattern C — single API call per checker.
func checkMSKSecrets(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok || cluster.ClusterArn == nil || *cluster.ClusterArn == "" {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: 0}
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.MSK == nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	scramAPI, ok := c.MSK.(MSKListScramSecretsAPI)
	if !ok {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1}
	}
	out, err := scramAPI.ListScramSecrets(ctx, &kafka.ListScramSecretsInput{
		ClusterArn: aws.String(*cluster.ClusterArn),
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "secrets", Count: -1, Err: err}
	}
	var ids []string
	for _, arn := range out.SecretArnList {
		// Secret ARN: arn:aws:secretsmanager:REGION:ACCOUNT:secret:NAME-suffix
		// The cache key is the secret name (last segment after ":secret:").
		if _, name, ok := strings.Cut(arn, ":secret:"); ok && name != "" {
			ids = append(ids, name)
		}
	}
	return relatedResult("secrets", ids)
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




