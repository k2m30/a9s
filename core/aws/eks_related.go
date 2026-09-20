// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// eks_related.go contains EKS cluster related-resource checker functions.
package aws

import (
	"context"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkEKSNodeGroups checks the cache for node groups belonging to this EKS cluster.
func checkEKSNodeGroups(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		clusterName = res.Fields["cluster_name"]
	}
	if clusterName == "" {
		return resource.ProvenZero("ng", "clusterName")
	}

	ngList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ng")
	if err != nil {
		return resource.ErrorRelated("ng", err)
	}
	if ngList == nil {
		return resource.UnknownRelated("ng")
	}

	var ids []string
	for _, ngRes := range ngList {
		rawNG, ok := assertStruct[ekstypes.Nodegroup](ngRes.RawStruct)
		ngCluster := ngRes.Fields["cluster_name"]
		if ok && rawNG.ClusterName != nil {
			ngCluster = *rawNG.ClusterName
		}
		if ngCluster == clusterName {
			ids = append(ids, ngRes.ID)
		}
	}
	return relatedResultTrunc("ng", ids, truncated)
}

// checkEKSAlarms checks the cache for CloudWatch alarms with ClusterName dimension matching this cluster.
func checkEKSAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "eks", res)
}

// checkEKSCFN checks the EKS cluster's tags for aws:cloudformation:stack-name and finds the matching CFN stack.
// EKS Cluster Tags is map[string]string (not a slice of Tag structs).
func checkEKSCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	stackName := ""
	raw, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if ok {
		stackName = raw.Tags["aws:cloudformation:stack-name"]
	}
	if stackName == "" {
		return unreadZero(res, resource.ProvenZero("cfn", "stackName"))
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
	return unreadZeroScanned(res, len(cfnList), relatedResultTrunc("cfn", ids, truncated))
}

// checkEKSLogs searches the logs cache for the EKS control-plane log group.
// Pattern N — naming convention: /aws/eks/{cluster-name}/cluster
func checkEKSLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	clusterName := res.ID
	if clusterName == "" {
		return resource.ProvenZero("logs", "clusterName")
	}

	expectedLogGroup := "/aws/eks/" + clusterName + "/cluster"

	logList, truncated, err := relatedResourcesFor(ctx, clients, cache, "logs")
	if err != nil {
		return resource.ErrorRelated("logs", err)
	}
	if logList == nil {
		return resource.UnknownRelated("logs")
	}

	var ids []string
	for _, logRes := range logList {
		if logRes.ID == expectedLogGroup {
			ids = append(ids, logRes.ID)
		}
	}
	return relatedResultTrunc("logs", ids, truncated)
}

// checkEKSSG extracts security group IDs from the EKS Cluster's
// ResourcesVpcConfig (ClusterSecurityGroupId + SecurityGroupIds).
// Pattern F — no cache needed.
func checkEKSSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("sg")
	}
	if raw.ResourcesVpcConfig == nil {
		return resource.ProvenZero("sg", "raw.ResourcesVpcConfig")
	}
	var ids []string
	if raw.ResourcesVpcConfig.ClusterSecurityGroupId != nil && *raw.ResourcesVpcConfig.ClusterSecurityGroupId != "" {
		ids = append(ids, *raw.ResourcesVpcConfig.ClusterSecurityGroupId)
	}
	for _, sgID := range raw.ResourcesVpcConfig.SecurityGroupIds {
		if sgID != "" {
			ids = append(ids, sgID)
		}
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkEKSVPC returns the VPC this EKS cluster runs in (Pattern R).
// Reads ResourcesVpcConfig.VpcId from the Cluster RawStruct.
func checkEKSVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok {
		return resource.UnknownRelated("vpc")
	}
	if raw.ResourcesVpcConfig == nil || raw.ResourcesVpcConfig.VpcId == nil || *raw.ResourcesVpcConfig.VpcId == "" {
		return resource.ProvenZero("vpc", "raw.ResourcesVpcConfig.VpcId")
	}
	return relatedResultTrunc("vpc", []string{*raw.ResourcesVpcConfig.VpcId}, false)
}

// checkEKSKMS extracts the KMS key ID from the EKS Cluster's EncryptionConfig.
// The KeyArn has the form arn:aws:kms::ACCOUNT:key/KEY-ID; the key ID is the
// last segment after "/". Pattern F — no cache needed.
func checkEKSKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok || len(raw.EncryptionConfig) == 0 ||
		raw.EncryptionConfig[0].Provider == nil ||
		raw.EncryptionConfig[0].Provider.KeyArn == nil ||
		*raw.EncryptionConfig[0].Provider.KeyArn == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("kms")
		}
		return resource.ProvenZero("kms", "EncryptionConfig.Provider.KeyArn")
	}
	keyID := kmsRefFromField(*raw.EncryptionConfig[0].Provider.KeyArn, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkEKSRole returns the IAM role in the EKS Cluster's RoleArn field.
func checkEKSRole(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	raw, ok := assertStruct[ekstypes.Cluster](res.RawStruct)
	if !ok || raw.RoleArn == nil || *raw.RoleArn == "" {
		if res.RawStruct == nil {
			return resource.UnknownRelated("role")
		}
		return resource.ProvenZero("role", "raw.RoleArn")
	}
	return relatedRefs("role", []string{*raw.RoleArn}, refContext(clients, cache, "role"))
}
