// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// msk_related.go contains MSK cluster related-resource checker functions.
package aws

import (
	"context"
	"strings"

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
// It reads the SecurityGroups field from the Provisioned.BrokerNodeGroupInfo struct (Pattern F).
func checkMSKSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if cluster.Provisioned == nil || cluster.Provisioned.BrokerNodeGroupInfo == nil {
		return resource.KnownRelated("sg", nil, false)
	}
	ids := cluster.Provisioned.BrokerNodeGroupInfo.SecurityGroups
	if len(ids) == 0 {
		return resource.KnownRelated("sg", nil, false)
	}
	return relatedResult("sg", ids)
}

// checkMSKLambda calls lambda:ListEventSourceMappings with the EventSourceArn
// filter set to this cluster's ARN (one call per open cluster — budget rule 7
// in docs/related-resources.md) and maps the returned FunctionArn entries
// against the lambda cache. MSK → Lambda triggers use the cluster ARN as the
// event source.
func checkMSKLambda(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok || cluster.ClusterArn == nil || *cluster.ClusterArn == "" {
		return resource.KnownRelated("lambda", nil, false)
	}
	return lambdaEventSourceMappingLambdaCheck(ctx, clients, *cluster.ClusterArn, cache)
}

// checkMSKCFN matches the MSK cluster's aws:cloudformation:stack-name tag to
// a CFN stack (Pattern C). kafkatypes.Cluster.Tags is a map[string]string
// populated at list time (ListClustersV2). Returns Count: 0 when no tag is set.
func checkMSKCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("cfn")
	}
	stackName := cluster.Tags["aws:cloudformation:stack-name"]
	if stackName == "" {
		return resource.KnownRelated("cfn", nil, false)
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
// (Provisioned.BrokerNodeGroupInfo.ClientSubnets). Pattern F — no cache needed.
func checkMSKSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("subnet")
	}
	if cluster.Provisioned == nil || cluster.Provisioned.BrokerNodeGroupInfo == nil {
		return resource.KnownRelated("subnet", nil, false)
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
		return resource.UnknownRelated("vpc")
	}
	if cluster.Provisioned == nil || cluster.Provisioned.BrokerNodeGroupInfo == nil {
		return resource.KnownRelated("vpc", nil, false)
	}
	subnets := cluster.Provisioned.BrokerNodeGroupInfo.ClientSubnets
	if len(subnets) == 0 {
		return resource.KnownRelated("vpc", nil, false)
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
		return relatedResult("vpc", []string{*sn.VpcId})
	}
	if truncated {
		// Subnet cache is truncated — the cluster's client subnet may be on a
		// dropped page; answer is unknown rather than a definitive non-match.
		return resource.UnknownRelated("vpc")
	}
	return resource.KnownRelated("vpc", nil, false)
}

// checkMSKLogs would resolve the CloudWatch log group configured for broker
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
		return resource.KnownRelated("logs", nil, false)
	}
	cw := cluster.Provisioned.LoggingInfo.BrokerLogs.CloudWatchLogs
	if cw.Enabled == nil || !*cw.Enabled || cw.LogGroup == nil || *cw.LogGroup == "" {
		return resource.KnownRelated("logs", nil, false)
	}
	return relatedResult("logs", []string{*cw.LogGroup})
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
		return resource.KnownRelated("s3", nil, false)
	}
	s3Log := cluster.Provisioned.LoggingInfo.BrokerLogs.S3
	if s3Log.Enabled == nil || !*s3Log.Enabled || s3Log.Bucket == nil || *s3Log.Bucket == "" {
		return resource.KnownRelated("s3", nil, false)
	}
	return relatedResult("s3", []string{*s3Log.Bucket})
}

// checkMSKSecrets calls kafka:ListScramSecrets(clusterArn) and returns the
// Secrets Manager secret names associated with this cluster's SCRAM auth.
// Pattern C — single API call per checker.
func checkMSKSecrets(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok || cluster.ClusterArn == nil || *cluster.ClusterArn == "" {
		return resource.KnownRelated("secrets", nil, false)
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.MSK == nil {
		return resource.UnknownRelated("secrets")
	}
	scramAPI, ok := c.MSK.(MSKListScramSecretsAPI)
	if !ok {
		return resource.UnknownRelated("secrets")
	}
	out, err := scramAPI.ListScramSecrets(ctx, &kafka.ListScramSecretsInput{
		ClusterArn: aws.String(*cluster.ClusterArn),
	})
	if err != nil {
		return resource.ErrorRelated("secrets", err)
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
		return resource.KnownRelated("kms", nil, false)
	}
	return relatedResult("kms", []string{*cluster.Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId})
}
