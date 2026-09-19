// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// msk_related.go contains MSK cluster related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkMSKAlarms checks the cache for CloudWatch alarms with "Cluster Name" dimension matching this cluster.
func checkMSKAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "", "Cluster Name", res.ID)
}

// checkMSKSG returns the security groups associated with the MSK cluster's broker nodes.
// It reads the SecurityGroups field from the Provisioned.BrokerNodeGroupInfo struct.
func checkMSKSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if cluster.Provisioned == nil || cluster.Provisioned.BrokerNodeGroupInfo == nil {
		return resource.ProvenZero("sg", "cluster.Provisioned.BrokerNodeGroupInfo")
	}
	ids := cluster.Provisioned.BrokerNodeGroupInfo.SecurityGroups
	if len(ids) == 0 {
		return resource.ProvenZero("sg", "ids")
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkMSKLambda calls lambda:ListEventSourceMappings with the EventSourceArn
// filter set to this cluster's ARN (one call per open cluster — the per-open
// call budget in docs/related-resources.md) and maps the returned FunctionArn entries
// against the lambda cache. MSK → Lambda triggers use the cluster ARN as the
// event source.
func checkMSKLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok || cluster.ClusterArn == nil || *cluster.ClusterArn == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("lambda")
		}
		return resource.ProvenZero("lambda", "ClusterArn")
	}
	return lambdaEventSourceMappingLambdaCheck(ctx, clients, *cluster.ClusterArn, cache)
}

// checkMSKCFN matches the MSK cluster's aws:cloudformation:stack-name tag to
// a CFN stack. kafkatypes.Cluster.Tags is a map[string]string
// populated at list time (ListClustersV2). Returns Count: 0 when no tag is set.
func checkMSKCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("cfn")
	}
	stackName := cluster.Tags["aws:cloudformation:stack-name"]
	if stackName == "" {
		return resource.ProvenZero("cfn", "stackName")
	}

	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return resource.ErrorRelated("cfn", err)
	}
	if cfnList == nil {
		return resource.UnknownRelated("cfn")
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
	return relatedResultTrunc("cfn", ids, truncated)
}

// checkMSKSubnet returns the subnets the cluster's broker nodes run in
// (Provisioned.BrokerNodeGroupInfo.ClientSubnets).
func checkMSKSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if cluster.Provisioned == nil || cluster.Provisioned.BrokerNodeGroupInfo == nil {
		return resource.ProvenZero("subnet", "cluster.Provisioned.BrokerNodeGroupInfo")
	}
	ids := cluster.Provisioned.BrokerNodeGroupInfo.ClientSubnets
	return relatedResultTrunc("subnet", ids, false)
}

// checkMSKVPC returns the VPC that hosts the cluster's broker subnets by
// looking up the first ClientSubnet in the subnet cache and reading its VpcId.
func checkMSKVPC(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("vpc")
	}
	if cluster.Provisioned == nil || cluster.Provisioned.BrokerNodeGroupInfo == nil {
		return resource.ProvenZero("vpc", "cluster.Provisioned.BrokerNodeGroupInfo")
	}
	subnets := cluster.Provisioned.BrokerNodeGroupInfo.ClientSubnets
	if len(subnets) == 0 {
		return resource.ProvenZero("vpc", "subnets")
	}

	subnetList, truncated, err := relatedResourcesFor(ctx, clients, cache, "subnet")
	if err != nil {
		return resource.ErrorRelated("vpc", err)
	}
	if subnetList == nil {
		return resource.UnknownRelated("vpc")
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
		return relatedResultTrunc("vpc", []string{*sn.VpcId}, false)
	}
	if truncated {
		// Subnet cache is truncated — the cluster's client subnet may be on a
		// dropped page; answer is unknown rather than a definitive non-match.
		return resource.UnknownRelated("vpc")
	}
	return resource.ProvenZero("vpc", "the complete subnet list")
}

// checkMSKLogs resolves the CloudWatch log group configured for broker
// logs in LoggingInfo.BrokerLogs.CloudWatchLogs.LogGroup. ListClustersV2
// returns kafkatypes.Cluster that carries LoggingInfo when set on the cluster;
// when unset, Count: 0. This is a forward lookup.
func checkMSKLogs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("logs")
	}
	if cluster.Provisioned == nil ||
		cluster.Provisioned.LoggingInfo == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs.CloudWatchLogs == nil {
		return resource.ProvenZero("logs", "BrokerLogs.CloudWatchLogs")
	}
	cw := cluster.Provisioned.LoggingInfo.BrokerLogs.CloudWatchLogs
	if cw.Enabled == nil || !*cw.Enabled || cw.LogGroup == nil || *cw.LogGroup == "" {
		return resource.ProvenZero("logs", "cw.LogGroup")
	}
	return relatedResultTrunc("logs", []string{*cw.LogGroup}, false)
}

// checkMSKS3 extracts the S3 bucket configured for broker log delivery in
// LoggingInfo.BrokerLogs.S3.Bucket. Forward lookup from kafkatypes.Cluster.
func checkMSKS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("s3")
	}
	if cluster.Provisioned == nil ||
		cluster.Provisioned.LoggingInfo == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs.S3 == nil {
		return resource.ProvenZero("s3", "BrokerLogs.S3")
	}
	s3Log := cluster.Provisioned.LoggingInfo.BrokerLogs.S3
	if s3Log.Enabled == nil || !*s3Log.Enabled || s3Log.Bucket == nil || *s3Log.Bucket == "" {
		return resource.ProvenZero("s3", "s3Log.Bucket")
	}
	return relatedResultTrunc("s3", []string{*s3Log.Bucket}, false)
}

// checkMSKSecrets calls kafka:ListScramSecrets(clusterArn) and returns the
// Secrets Manager secret names associated with this cluster's SCRAM auth.
func checkMSKSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok || cluster.ClusterArn == nil || *cluster.ClusterArn == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("secrets")
		}
		return resource.ProvenZero("secrets", "ClusterArn")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.MSK == nil {
		return resource.UnknownRelated("secrets")
	}
	scramAPI, ok := c.MSK.(MSKListScramSecretsAPI)
	if !ok {
		return resource.UnknownRelated("secrets")
	}
	arns, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]string, *string, error) {
		out, err := scramAPI.ListScramSecrets(ctx, &kafka.ListScramSecretsInput{
			ClusterArn: aws.String(*cluster.ClusterArn),
			NextToken:  token,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.SecretArnList, out.NextToken, nil
	})
	if err != nil {
		return resource.ErrorRelated("secrets", err)
	}
	ids, dropped := resolveRefs("secrets", arns, refContext(clients, cache, "secrets"))
	return relatedResultTrunc("secrets", ids, dropped || !complete)
}

// checkMSKKMS extracts the KMS key ID from the MSK cluster's
// Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId field.
func checkMSKKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok || cluster.Provisioned == nil ||
		cluster.Provisioned.EncryptionInfo == nil ||
		cluster.Provisioned.EncryptionInfo.EncryptionAtRest == nil ||
		cluster.Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId == nil ||
		*cluster.Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("kms")
		}
		return resource.ProvenZero("kms", "EncryptionAtRest.DataVolumeKMSKeyId")
	}
	return kmsRelated(ctx, clients, cache, []string{*cluster.Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId})
}
