// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// redshift_related.go contains Redshift Cluster related-resource checker functions.
package aws

import (
	"context"

	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkRedshiftAlarms checks the cache for CloudWatch alarms with ClusterIdentifier dimension matching this cluster.
func checkRedshiftAlarms(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	return alarmIDsByDimension(ctx, clients, cache, "redshift", res)
}

// checkRedshiftSG extracts security group IDs from the Redshift Cluster's
// VpcSecurityGroups slice.
func checkRedshiftSG(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("sg")
	}
	var ids []string
	for _, vsg := range cluster.VpcSecurityGroups {
		if vsg.VpcSecurityGroupId != nil && *vsg.VpcSecurityGroupId != "" {
			ids = append(ids, *vsg.VpcSecurityGroupId)
		}
	}
	return relatedResultTrunc("sg", ids, false)
}

// checkRedshiftVPC returns the VPC this Redshift cluster runs in.
// Reads Cluster.VpcId from the RawStruct.
func checkRedshiftVPC(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("vpc")
	}
	if cluster.VpcId == nil || *cluster.VpcId == "" {
		return foundNone("vpc", "cluster.VpcId")
	}
	return relatedResultTrunc("vpc", []string{*cluster.VpcId}, false)
}

// checkRedshiftRole extracts IAM role ARNs from the Redshift Cluster's IamRoles slice.
func checkRedshiftRole(_ context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("role")
	}
	if len(cluster.IamRoles) == 0 {
		return foundNone("role", "cluster.IamRoles")
	}
	var refs []string
	for _, r := range cluster.IamRoles {
		if r.IamRoleArn != nil {
			refs = append(refs, *r.IamRoleArn)
		}
	}
	return relatedRefs("role", refs, refContext(clients, cache, "role"))
}

// checkRedshiftKMS extracts the KMS key ID from the Redshift Cluster's KmsKeyId
// field.
func checkRedshiftKMS(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok || cluster.KmsKeyId == nil || *cluster.KmsKeyId == "" {
		if res.RawStruct == nil {
			return NotRead("kms")
		}
		return foundNone("kms", "KmsKeyId")
	}
	keyID := kmsRefFromField(*cluster.KmsKeyId, res.Type)
	return kmsRelated(ctx, clients, cache, []string{keyID})
}

// checkRedshiftCFN checks the Cluster's Tags for aws:cloudformation:stack-name
// and matches against the CFN stack cache.
func checkRedshiftCFN(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("cfn")
	}
	stackName := ""
	for _, tag := range cluster.Tags {
		if tag.Key != nil && *tag.Key == "aws:cloudformation:stack-name" && tag.Value != nil {
			stackName = *tag.Value
			break
		}
	}
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
		raw, rawOK := assertStruct[cfntypes.Stack](cfnRes.RawStruct)
		if rawOK && raw.StackName != nil && *raw.StackName == stackName {
			ids = append(ids, cfnRes.ID)
		}
	}
	return relatedResultTrunc("cfn", ids, truncated)
}

// checkRedshiftSecrets resolves the admin-credentials secret managed for this
// Redshift cluster. Cluster.MasterPasswordSecretArn holds the full secret
// ARN; we match it against the secrets cache by ARN.
func checkRedshiftSecrets(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok {
		return NotRead("secrets")
	}
	if cluster.MasterPasswordSecretArn == nil || *cluster.MasterPasswordSecretArn == "" {
		return foundNone("secrets", "cluster.MasterPasswordSecretArn")
	}
	secretARN := *cluster.MasterPasswordSecretArn

	secretList, truncated, err := relatedResourcesFor(ctx, clients, cache, "secrets")
	if err != nil {
		return ReadFailed("secrets", err)
	}
	if secretList == nil {
		return NotRead("secrets")
	}

	var ids []string
	for _, secretRes := range secretList {
		if secretRes.Fields["arn"] == secretARN {
			ids = append(ids, secretRes.ID)
		}
	}
	return relatedResultTrunc("secrets", ids, truncated)
}

// checkRedshiftLogs resolves the cluster's audit-log target via a single
// redshift:DescribeLoggingStatus call. When LogDestinationType
// is cloudwatch, one log-group ID is emitted per enabled LogExports[] entry
// following the /aws/redshift/cluster/{clusterID}/{logExport} naming convention.
func checkRedshiftLogs(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	status, err := redshiftLoggingStatus(ctx, clients, res)
	if err != nil {
		return ReadFailed("logs", err)
	}
	if status == nil {
		return NotRead("logs")
	}
	if status.LoggingEnabled == nil || !*status.LoggingEnabled {
		return foundNone("logs", "status.LoggingEnabled")
	}
	if status.LogDestinationType != redshifttypes.LogDestinationTypeCloudwatch {
		// S3-only audit logging — no log group association.
		return foundNone("logs", "status.LogDestinationType")
	}
	clusterID := res.ID
	if clusterID == "" {
		return keyMissing("logs", "clusterID")
	}
	if len(status.LogExports) == 0 {
		// CloudWatch logging enabled but no specific exports configured.
		return foundNone("logs", "status.LogExports")
	}
	// Emit one log-group ID per enabled export:
	// /aws/redshift/cluster/{clusterID}/{logExport}
	var ids []string
	for _, export := range status.LogExports {
		ids = append(ids, "/aws/redshift/cluster/"+clusterID+"/"+export)
	}
	return relatedResultTrunc("logs", ids, false)
}

// checkRedshiftS3 resolves the audit-log S3 bucket via a single
// redshift:DescribeLoggingStatus call. BucketName is set only
// when the cluster logs to S3 (not CloudWatch).
func checkRedshiftS3(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	status, err := redshiftLoggingStatus(ctx, clients, res)
	if err != nil {
		return ReadFailed("s3", err)
	}
	if status == nil {
		return NotRead("s3")
	}
	if status.LoggingEnabled == nil || !*status.LoggingEnabled {
		return foundNone("s3", "status.LoggingEnabled")
	}
	if status.BucketName == nil || *status.BucketName == "" {
		return foundNone("s3", "status.BucketName")
	}
	return relatedResultTrunc("s3", []string{*status.BucketName}, false)
}

// checkRedshiftSubnet resolves the cluster's subnet-group members via a
// single redshift:DescribeClusterSubnetGroups call.
func checkRedshiftSubnet(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	cluster, ok := assertStruct[redshifttypes.Cluster](res.RawStruct)
	if !ok || cluster.ClusterSubnetGroupName == nil || *cluster.ClusterSubnetGroupName == "" {
		return NotRead("subnet")
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Redshift == nil {
		return NotRead("subnet")
	}
	name := *cluster.ClusterSubnetGroupName
	groups, _, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]redshifttypes.ClusterSubnetGroup, *string, error) {
		out, err := c.Redshift.DescribeClusterSubnetGroups(ctx, &redshift.DescribeClusterSubnetGroupsInput{
			ClusterSubnetGroupName: &name,
			Marker:                 marker,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.ClusterSubnetGroups, out.Marker, nil
	})
	if err != nil {
		return ReadFailed("subnet", err)
	}
	if len(groups) == 0 {
		return foundNone("subnet", "ClusterSubnetGroups")
	}
	var ids []string
	for _, sn := range groups[0].Subnets {
		if sn.SubnetIdentifier != nil && *sn.SubnetIdentifier != "" {
			ids = append(ids, *sn.SubnetIdentifier)
		}
	}
	return relatedResultTrunc("subnet", ids, false)
}

// redshiftLoggingStatus performs a single DescribeLoggingStatus call for this
// cluster's identifier, wrapped in RetryOnThrottle. Returns (nil, nil) when
// the client is unavailable or the cluster ID is empty (no API call
// attempted — callers render an UnknownRelated result without a FlashMsg).
// Returns (nil, err) on API failure so callers can surface the underlying
// error via Result.Err → FlashMsg → error log.
func redshiftLoggingStatus(ctx context.Context, clients any, res resource.Resource) (*redshift.DescribeLoggingStatusOutput, error) {
	clusterID := res.ID
	if clusterID == "" {
		return nil, nil
	}
	c, cok := clients.(*ServiceClients)
	if !cok || c == nil || c.Redshift == nil {
		return nil, nil
	}
	return RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*redshift.DescribeLoggingStatusOutput, error) {
		return c.Redshift.DescribeLoggingStatus(ctx, &redshift.DescribeLoggingStatusInput{
			ClusterIdentifier: &clusterID,
		})
	})
}
