// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// msk_related.go contains MSK cluster related-resource checker functions.
package aws

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkMSKAlarms checks the cache for CloudWatch alarms with "Cluster Name" dimension matching this cluster.
func checkMSKAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "msk", res)
}

// checkMSKSG returns the security groups of the cluster's network: the
// broker nodes' of a provisioned cluster, the VPC configurations' of a
// serverless one.
func checkMSKSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}
	_, sgs := mskNetwork(cluster)
	return relatedResultTrunc("sg", sgs, false)
}

// mskNetwork returns the subnets and security groups a cluster's network is
// made of: Provisioned.BrokerNodeGroupInfo for a provisioned cluster,
// Serverless.VpcConfigs for a serverless one.
func mskNetwork(cluster kafkatypes.Cluster) (subnets, sgs []string) {
	if p := cluster.Provisioned; p != nil && p.BrokerNodeGroupInfo != nil {
		subnets = append(subnets, p.BrokerNodeGroupInfo.ClientSubnets...)
		sgs = append(sgs, p.BrokerNodeGroupInfo.SecurityGroups...)
	}
	if cluster.Serverless != nil {
		for _, vc := range cluster.Serverless.VpcConfigs {
			subnets = append(subnets, vc.SubnetIds...)
			sgs = append(sgs, vc.SecurityGroupIds...)
		}
	}
	return subnets, sgs
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
			return NotRead("lambda")
		}
		return foundNone("lambda", "ClusterArn")
	}
	return lambdaEventSourceMappingLambdaCheck(ctx, clients, *cluster.ClusterArn, cache)
}

// checkMSKCFN matches the MSK cluster's aws:cloudformation:stack-name tag to
// a CFN stack. kafkatypes.Cluster.Tags is a map[string]string
// populated at list time (ListClustersV2). Returns Count: 0 when no tag is set.
func checkMSKCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("cfn")
	}
	stackName := cluster.Tags["aws:cloudformation:stack-name"]
	if stackName == "" {
		return foundNone("cfn", "stackName")
	}

	cfnList, truncated, err := relatedResourcesFor(ctx, clients, cache, "cfn")
	if err != nil {
		return ReadFailed("cfn", err)
	}
	if cfnList == nil {
		return NotRead("cfn")
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

// checkMSKSubnet returns the subnets of the cluster's network (mskNetwork).
func checkMSKSubnet(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("subnet")
	}
	subnets, _ := mskNetwork(cluster)
	return relatedResultTrunc("subnet", subnets, false)
}

// checkMSKVPC returns the VPCs that host the cluster's subnets, read off each
// subnet's VpcId in the subnet list. A subnet the list may hold on a page
// nobody read is a VPC unread.
func checkMSKVPC(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("vpc")
	}
	subnets, _ := mskNetwork(cluster)
	if len(subnets) == 0 {
		return foundNone("vpc", "the cluster's subnets")
	}

	subnetList, truncated, err := relatedResourcesFor(ctx, clients, cache, "subnet")
	if err != nil {
		return ReadFailed("vpc", err)
	}
	if subnetList == nil {
		return NotRead("vpc")
	}

	var vpcs []string
	unread := false
	for _, want := range subnets {
		i := slices.IndexFunc(subnetList, func(r resource.Resource) bool { return r.ID == want })
		if i < 0 {
			unread = unread || truncated
			continue
		}
		sn, snOk := assertStruct[ec2types.Subnet](subnetList[i].RawStruct)
		if !snOk || aws.ToString(sn.VpcId) == "" {
			unread = true
			continue
		}
		vpcs = append(vpcs, *sn.VpcId)
	}
	return relatedAnswer("vpc", relatedRead{ids: vpcs, unread: unread})
}

// checkMSKLogs resolves the CloudWatch log group configured for broker
// logs in LoggingInfo.BrokerLogs.CloudWatchLogs.LogGroup. ListClustersV2
// returns kafkatypes.Cluster that carries LoggingInfo when set on the cluster;
// when unset, Count: 0. This is a forward lookup.
func checkMSKLogs(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("logs")
	}
	if cluster.Provisioned == nil ||
		cluster.Provisioned.LoggingInfo == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs.CloudWatchLogs == nil {
		return foundNone("logs", "BrokerLogs.CloudWatchLogs")
	}
	cw := cluster.Provisioned.LoggingInfo.BrokerLogs.CloudWatchLogs
	if cw.Enabled == nil || !*cw.Enabled || cw.LogGroup == nil || *cw.LogGroup == "" {
		return foundNone("logs", "cw.LogGroup")
	}
	return relatedResultTrunc("logs", []string{*cw.LogGroup}, false)
}

// checkMSKS3 extracts the S3 bucket configured for broker log delivery in
// LoggingInfo.BrokerLogs.S3.Bucket. Forward lookup from kafkatypes.Cluster.
func checkMSKS3(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("s3")
	}
	if cluster.Provisioned == nil ||
		cluster.Provisioned.LoggingInfo == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs == nil ||
		cluster.Provisioned.LoggingInfo.BrokerLogs.S3 == nil {
		return foundNone("s3", "BrokerLogs.S3")
	}
	s3Log := cluster.Provisioned.LoggingInfo.BrokerLogs.S3
	if s3Log.Enabled == nil || !*s3Log.Enabled || s3Log.Bucket == nil || *s3Log.Bucket == "" {
		return foundNone("s3", "s3Log.Bucket")
	}
	return relatedResultTrunc("s3", []string{*s3Log.Bucket}, false)
}

// checkMSKSecrets returns the Secrets Manager secrets associated with the
// cluster for SASL/SCRAM, read with kafka:ListScramSecrets. MSK associates
// secrets only with a provisioned cluster that has SCRAM enabled
// (ServerlessSasl carries IAM alone), so any other cluster has none and is
// not asked. A refused call is a place not read; Kafka answers a caller it
// denies with 401 UnauthorizedException or 403 ForbiddenException
// (docs.aws.amazon.com/msk/1.0/apireference/clusters-clusterarn-scram-secrets.html).
func checkMSKSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[kafkatypes.Cluster](res.RawStruct)
	if !ok || cluster.ClusterArn == nil || *cluster.ClusterArn == "" {
		if res.RawStruct == nil {
			return NotRead("secrets")
		}
		return foundNone("secrets", "ClusterArn")
	}
	if !mskSCRAMEnabled(cluster) {
		return foundNone("secrets", "ClientAuthentication.Sasl.Scram")
	}
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil || c.MSK == nil {
		return NotRead("secrets")
	}
	scramAPI, ok := c.MSK.(MSKListScramSecretsAPI)
	if !ok {
		return NotRead("secrets")
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
	switch {
	case err != nil && isAWSRefusal(err):
		return relatedAnswer("secrets", relatedRead{unread: true})
	case err != nil:
		return ReadFailed("secrets", err)
	}
	return listedRelated(ctx, clients, cache, "secrets", arns, !complete)
}

// mskSCRAMEnabled reports whether a provisioned cluster accepts SASL/SCRAM.
func mskSCRAMEnabled(cluster kafkatypes.Cluster) bool {
	p := cluster.Provisioned
	return p != nil && p.ClientAuthentication != nil && p.ClientAuthentication.Sasl != nil &&
		p.ClientAuthentication.Sasl.Scram != nil && aws.ToBool(p.ClientAuthentication.Sasl.Scram.Enabled)
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
			return NotRead("kms")
		}
		return foundNone("kms", "EncryptionAtRest.DataVolumeKMSKeyId")
	}
	return kmsRelated(ctx, clients, cache, []string{*cluster.Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId})
}
